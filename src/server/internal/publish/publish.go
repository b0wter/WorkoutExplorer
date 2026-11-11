// Package publish is the only code that creates, writes or removes anything
// under <data>/public (I-3; F-15 design P-3, ADR-028). It implements
// lifecycle.ChangeHook (the public build of one workout) and
// lifecycle.PublicFiles (remove one folder, sweep the area).
//
// Every change is a whole build in memory from original.fit: parse, the
// public transform (F-16 start and end hiding), the artefacts (data file and its
// brotli encoding, the preview image of F-23; F-24 adds its own), then the
// page from the data file just encoded. Only artefacts whose content-hashed name is not on disk
// are written; index.html is always written and is the commit point. A change
// that could hide more removes the folder first (P-7).
//
// Every change to public/ holds an exclusive flock on <data>/public.lock, in
// every process (ADR-031).
//
// No error returned from here contains a public_id or a path under public/
// (P-14): lifecycle logs err.Error().
package publish

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/page"
	"workout-explorer/internal/places"
	"workout-explorer/internal/precompress"
	"workout-explorer/internal/preview"
	"workout-explorer/internal/workout"
)

// Originals reads a workout's stored original.fit (internal/privatedir).
type Originals interface {
	Original(id string) ([]byte, error)
}

// Maps reads a workout's stored basemap for a frame key (internal/privatedir,
// F-36 MC-9). An absent map is an error wrapping fs.ErrNotExist.
type Maps interface {
	PreviewMap(id, key string) ([]byte, error)
}

// Parser is the process's single *ingest.Parser, so its slots, budgets and
// time limit apply to the build (P-3).
type Parser interface {
	Parse(ctx context.Context, data []byte) (workout.Workout, error)
}

// Deps are the collaborators of an Area.
type Deps struct {
	Template  *template.Template // LoadTemplate
	Originals Originals
	Maps      Maps // nil: always the server-made card (F-23)
	Parser    Parser
	Storage   *lifecycle.Storage // nil: no total (tests)
	Logger    *slog.Logger       // nil: no log lines (tests)
	BaseURL   string             // origin of absolute links, checked by cmd (F-23 PC-8)
	Places    *places.Index      // nil: no suggested title (F-45 ST-5)
	// Rows reads the row again inside the lock before a write phase (F-30
	// RG-5); nil: no re-read (tests).
	Rows Rows
}

// Rows reads one workout row (internal/store).
type Rows interface {
	WorkoutByPublicID(ctx context.Context, publicID string) (lifecycle.Record, bool, error)
}

// Area is the public area of one data directory.
type Area struct {
	root string
	lock string // {data}/public.lock (ADR-031)
	d    Deps
	// transform is the public transform (P-4): start and end hiding (F-16).
	// Artefacts and the page see only its output; routeHidden is the fully
	// hidden route of F-16 H-5.
	transform func(workout.Workout, lifecycle.Record) (w workout.Workout, routeHidden bool)
	// step is called before every file-system change with its name; a test
	// seam for failure and kill injection. nil in production.
	step func(name string) error
}

// Modes of P-6. F-31 sets the group so that Caddy can read them.
const (
	dirMode  = 0o750
	fileMode = 0o640
)

const indexName = "index.html"

// Open creates <data>/public (mode 0750) and <data>/public.lock (mode 0600)
// if missing.
func Open(dataDir string, d Deps) (*Area, error) {
	root := filepath.Join(dataDir, "public")
	if err := os.MkdirAll(root, dirMode); err != nil {
		return nil, fail("create", "public area", err)
	}
	a := &Area{root: root, lock: filepath.Join(dataDir, "public.lock"), d: d, transform: publicTransform}
	unlock, err := a.acquire()
	if err != nil {
		return nil, err
	}
	unlock()
	return a, nil
}

// acquire takes the exclusive flock on public.lock that every change to
// public/ holds, in every process (ADR-031). Each call opens the file anew,
// so two callers in one process also exclude each other; the kernel drops
// the lock when the holder dies. Callers outside this package never take it.
func (a *Area) acquire() (unlock func(), err error) {
	f, err := os.OpenFile(a.lock, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fail("open", "public lock", err)
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, fail("lock", "public lock", err)
	}
	return func() { f.Close() }, nil // closing the last descriptor releases the flock
}

// locked runs f under the public lock; a panic in f releases it too.
func (a *Area) locked(f func() error) error {
	unlock, err := a.acquire()
	if err != nil {
		return err
	}
	defer unlock()
	return f()
}

// publicTransform applies the record's settings that remove data before
// anything public exists (I-3): start and end hiding (F-16 H-4). Nothing
// about the cut is stored or logged; every build computes it again.
func publicTransform(w workout.Workout, r lifecycle.Record) (workout.Workout, bool) {
	if !r.HideEnds {
		return w, false
	}
	return workout.HideEnds(w)
}

// validPublicID: 22 base62 characters (ADR-020); checked before any path is built.
func validPublicID(s string) bool {
	if len(s) != 22 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

var errInvalidID = errors.New("publish: invalid public ID")

// fail reduces a file-system error to the operation, the file's role and the
// system error, so no path (and so no public_id) reaches a log line (P-14).
func fail(op, role string, err error) error {
	var pe *fs.PathError
	var le *os.LinkError
	var se *os.SyscallError
	switch {
	case errors.As(err, &pe):
		err = pe.Err
	case errors.As(err, &le):
		err = le.Err
	case errors.As(err, &se):
		err = se.Err
	}
	return fmt.Errorf("publish: %s %s: %w", op, role, err)
}

// role names a file of a public folder for errors.
func role(name string) string {
	switch {
	case name == indexName:
		return "page"
	case strings.HasSuffix(name, ".bin"):
		return "data file"
	case strings.HasSuffix(name, ".bin.br"):
		return "compressed data file"
	case strings.HasSuffix(name, ".png"):
		return "preview"
	case strings.HasPrefix(name, "."):
		return "temporary file"
	}
	return "file"
}

func (a *Area) at(name string) error {
	if a.step == nil {
		return nil
	}
	return a.step(name)
}

// WorkoutChanged is the F-12 hook (O-11) and publish's build (P-1, P-2):
// after holds the committed row. A workout that is not unlisted (a draft)
// only loses its folder. A change that could hide more removes the folder first.
func (a *Area) WorkoutChanged(ctx context.Context, before, after lifecycle.Record) error {
	if !validPublicID(after.PublicID) {
		return errInvalidID
	}
	dir := filepath.Join(a.root, after.PublicID)
	if after.Visibility != lifecycle.Unlisted || HidesMore(before, after) {
		err := a.locked(func() error { _, err := a.removeFolder(dir, true); return err })
		if err != nil || after.Visibility != lifecycle.Unlisted {
			return err
		}
	}
	// Parsing and encoding run outside the lock (ADR-031 item 2).
	b, err := a.build(ctx, after)
	if err != nil {
		return err
	}
	return a.locked(func() error {
		// RG-5: a row changed since the build was written by another process
		// (serve after regenerate read it), which builds from it itself; a
		// stale page must never overwrite a newer, possibly more hidden one.
		if a.d.Rows != nil {
			cur, found, err := a.d.Rows.WorkoutByPublicID(ctx, after.PublicID)
			if err != nil {
				return fmt.Errorf("publish: recheck: %w", err)
			}
			if !found || !cur.Live(time.Now()) || !sameRecord(cur, after) {
				return nil
			}
		}
		return a.write(dir, b)
	})
}

// sameRecord reports whether every field of a and b is equal, times by
// instant.
func sameRecord(a, b lifecycle.Record) bool {
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	for i := 0; i < av.NumField(); i++ {
		x, y := av.Field(i).Interface(), bv.Field(i).Interface()
		switch x := x.(type) {
		case time.Time:
			if !x.Equal(y.(time.Time)) {
				return false
			}
		case *time.Time:
			y := y.(*time.Time)
			if (x == nil) != (y == nil) || x != nil && !x.Equal(*y) {
				return false
			}
		default:
			if !reflect.DeepEqual(x, y) {
				return false
			}
		}
	}
	return true
}

// HidesMore is the pure rule of P-7: a change may be written in place only
// if every field that differs is on the allow-list of changes that cannot
// remove public content. Any other difference, including a field this rule
// does not know, counts as hiding more.
func HidesMore(before, after lifecycle.Record) bool {
	bv, av := reflect.ValueOf(before), reflect.ValueOf(after)
	t := bv.Type()
	for i := 0; i < t.NumField(); i++ {
		if reflect.DeepEqual(bv.Field(i).Interface(), av.Field(i).Interface()) {
			continue
		}
		switch t.Field(i).Name {
		case "Title", "Description", "ExpiresAt", "PublishedAt":
		// Derived from the original, never read by the public build (F-30
		// RG-6); a build that starts reading one must take it off this list.
		case "Sport", "StartTime", "TimeZoneOffsetS", "SummaryJSON", "DataEndS", "HiddenRanges":
		case "Visibility":
			if before.Visibility != lifecycle.Draft || after.Visibility != lifecycle.Unlisted { // only draft → unlisted
				return true
			}
		case "HideEnds":
			if after.HideEnds { // off → on hides more
				return true
			}
		case "AllowDownload":
			if !after.AllowDownload { // on → off hides more
				return true
			}
		default:
			return true
		}
	}
	return false
}

// artefact is one content-hashed file of the build; data is produced only
// when the name is not on disk yet (P-5: brotli runs only for a new name).
type artefact struct {
	name string
	data func() ([]byte, error)
}

type built struct {
	artefacts []artefact
	index     []byte
}

// front is the build's front half (P-4): parse the original, apply the
// public transform, encode and decode the data file. It writes nothing.
func (a *Area) front(ctx context.Context, r lifecycle.Record) (bin []byte, f datafile.File, routeHidden bool, err error) {
	orig, err := a.d.Originals.Original(r.ID)
	if err != nil {
		return nil, f, false, fail("read", "original", err)
	}
	w, err := a.d.Parser.Parse(ctx, orig)
	if err != nil {
		// A stored original the running binary rejects is a server error,
		// never a delete (P-14; F-30 owns deletes on rejection).
		return nil, f, false, fmt.Errorf("publish: parse original: %w", err)
	}
	orig = nil
	w, routeHidden = a.transform(w, r)
	if bin, err = datafile.Encode(w); err != nil {
		return nil, f, false, fmt.Errorf("publish: encode: %w", err)
	}
	// The page comes from the metadata as written (F-13 hand-off, ADR-026 item 3).
	if f, err = datafile.Decode(bin); err != nil {
		return nil, f, false, fmt.Errorf("publish: decode: %w", err)
	}
	return bin, f, routeHidden, nil
}

// Frame is the preview frame of the workout's public data file (F-36 MC-2,
// MC-4): the build's front half, nothing written. ok is false without
// public positions.
func (a *Area) Frame(ctx context.Context, r lifecycle.Record) (fr preview.Frame, ok bool, err error) {
	_, f, _, err := a.front(ctx, r)
	if err != nil {
		return fr, false, err
	}
	fr, ok = preview.FrameOf(f)
	return fr, ok, nil
}

// card is the preview of F-23 PC-1, over the stored basemap when one exists
// for the key of the frame computed now from f (F-36 MC-3); any failure on
// the map side draws the server-made card.
func (a *Area) card(r lifecycle.Record, f datafile.File, vm page.ViewModel) ([]byte, error) {
	if fr, ok := preview.FrameOf(f); ok && a.d.Maps != nil {
		m, err := a.d.Maps.PreviewMap(r.ID, fr.Key)
		if err == nil {
			img, rerr := preview.RenderOnMap(f, vm, fr, m)
			if rerr == nil {
				return img, nil
			}
			err = rerr
		}
		if !errors.Is(err, fs.ErrNotExist) && a.d.Logger != nil {
			// Never the frame, key or path (I-5); errors.Is keeps the reason short.
			reason := "read"
			if errors.Is(err, preview.ErrTooLarge) {
				reason = "size"
			} else if m != nil {
				reason = "render"
			}
			a.d.Logger.Info("preview map not used", "id", r.ID, "reason", reason)
		}
	}
	return preview.Render(f, vm)
}

// build is the public build of P-4, in memory. It is deterministic: the same
// original, settings and stored basemap give the same names and bytes.
func (a *Area) build(ctx context.Context, r lifecycle.Record) (built, error) {
	bin, f, routeHidden, err := a.front(ctx, r)
	if err != nil {
		return built{}, err
	}
	sum := sha256.Sum256(bin)
	name := "data." + hex.EncodeToString(sum[:8]) + ".bin"
	s := page.Settings{
		Title: r.Title, Description: r.Description, ExpiresAt: r.ExpiresAt,
		PublicID: r.PublicID, DataFile: name, RouteHidden: routeHidden, BaseURL: a.d.BaseURL,
	}
	// The town comes from the public file only, after the transform (F-45
	// ST-4); the page uses it only while the title is empty. Never logged.
	if a.d.Places != nil {
		town := ""
		if lat, lon, ok := f.FirstPosition(); ok {
			town, _ = a.d.Places.Nearest(lat, lon)
		}
		s.Suggested = page.Suggest(f.Meta, town)
	}
	// The preview is drawn from the decoded public file and the card text
	// (F-23 PC-1); its name then goes into the page's image URL.
	img, err := a.card(r, f, page.Build(f, s))
	if err != nil {
		return built{}, fmt.Errorf("publish: preview: %w", err)
	}
	isum := sha256.Sum256(img)
	s.Preview = "preview." + hex.EncodeToString(isum[:8]) + ".png"
	vm := page.Build(f, s)
	var buf bytes.Buffer
	if err := a.d.Template.Execute(&buf, vm); err != nil {
		return built{}, fmt.Errorf("publish: page: %w", err)
	}
	return built{
		artefacts: []artefact{
			{name, func() ([]byte, error) { return bin, nil }},
			{name + ".br", func() ([]byte, error) { return precompress.Brotli(bin) }},
			{s.Preview, func() ([]byte, error) { return img, nil }},
		},
		index: buf.Bytes(),
	}, nil
}

// write puts a build into dir (P-6): new artefacts first, then index.html by
// rename over the old one, then the folder is synced; only then is every
// other entry removed. A stop after any step leaves the old complete page or
// the new one (ADR-028 item 3).
func (a *Area) write(dir string, b built) error {
	if err := a.at("create folder"); err != nil {
		return fail("create", "folder", err)
	}
	switch err := os.Mkdir(dir, dirMode); {
	case err == nil:
		if err := syncDir(a.root); err != nil {
			return fail("sync", "public area", err)
		}
	case !errors.Is(err, fs.ErrExist):
		return fail("create", "folder", err)
	}
	keep := map[string]bool{indexName: true}
	for _, art := range b.artefacts {
		keep[art.name] = true
		if info, err := os.Lstat(filepath.Join(dir, art.name)); err == nil && info.Mode().IsRegular() {
			continue
		}
		data, err := art.data()
		if err != nil {
			return fmt.Errorf("publish: %s: %w", role(art.name), err)
		}
		if err := a.place(dir, art.name, data); err != nil {
			return err
		}
	}
	if err := a.place(dir, indexName, b.index); err != nil {
		return err
	}
	if err := a.at("sync folder"); err != nil {
		return fail("sync", "folder", err)
	}
	if err := syncDir(dir); err != nil {
		return fail("sync", "folder", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fail("list", "folder", err)
	}
	for _, e := range entries {
		if keep[e.Name()] {
			continue
		}
		p := filepath.Join(dir, e.Name())
		size := a.measure(p)
		if err := a.at("remove " + e.Name()); err != nil {
			return fail("remove", role(e.Name()), err)
		}
		if err := os.RemoveAll(p); err != nil {
			return fail("remove", role(e.Name()), err)
		}
		a.d.Storage.Release(size)
	}
	return nil
}

// place writes one file under a unique dot-prefixed temporary name in dir
// (so that another process building the same workout cannot share it), syncs
// it and renames it to name (P-6). Every byte goes through the storage total
// (P-9). A failure removes the temporary file; a kill leaves it for the sweep.
func (a *Area) place(dir, name string, data []byte) error {
	rl := role(name)
	final := filepath.Join(dir, name)
	if err := a.at("write " + name); err != nil {
		return fail("write", rl, err)
	}
	a.d.Storage.Add(int64(len(data)))
	f, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		a.d.Storage.Release(int64(len(data)))
		return fail("write", rl, err)
	}
	tmp := f.Name()
	discard := func(op string, err error) error {
		if rerr := os.Remove(tmp); rerr == nil || errors.Is(rerr, fs.ErrNotExist) {
			a.d.Storage.Release(int64(len(data)))
		}
		return fail(op, rl, err)
	}
	if err := f.Chmod(fileMode); err != nil {
		f.Close()
		return discard("write", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return discard("write", err)
	}
	if err := a.at("sync " + name); err != nil {
		f.Close()
		return discard("sync", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return discard("sync", err)
	}
	if err := f.Close(); err != nil {
		return discard("write", err)
	}
	var old int64
	if info, err := os.Lstat(final); err == nil {
		old = info.Size()
	}
	if err := a.at("rename " + name); err != nil {
		return discard("rename", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return discard("rename", err)
	}
	a.d.Storage.Release(old)
	return nil
}

// removeFolder removes dir, index.html first so that Caddy shows the generic
// page at once (P-7, P-13). freed is the sizes of its files; release says
// whether this call releases them from the storage total.
func (a *Area) removeFolder(dir string, release bool) (freed int64, err error) {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	size, err := usage(dir)
	if err != nil {
		return 0, fail("measure", "folder", err)
	}
	var pageSize int64
	if info, err := os.Lstat(filepath.Join(dir, indexName)); err == nil && info.Mode().IsRegular() {
		pageSize = info.Size()
	}
	if err := a.at("remove " + indexName); err != nil {
		return 0, fail("remove", "page", err)
	}
	if err := os.Remove(filepath.Join(dir, indexName)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, fail("remove", "page", err)
	}
	if release {
		a.d.Storage.Release(pageSize)
	}
	if err := syncDir(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, fail("sync", "folder", err)
	}
	if err := a.at("remove folder"); err != nil {
		return 0, fail("remove", "folder", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return 0, fail("remove", "folder", err)
	}
	if err := syncDir(a.root); err != nil {
		return 0, fail("sync", "public area", err)
	}
	if release {
		a.d.Storage.Release(size - pageSize)
	}
	return size, nil
}

// Remove is lifecycle.PublicFiles.Remove: public/{publicID}, index.html
// first. The caller releases freed (F-10 S-6).
func (a *Area) Remove(publicID string) (freed int64, err error) {
	if !validPublicID(publicID) {
		return 0, errInvalidID
	}
	err = a.locked(func() error { freed, err = a.removeFolder(filepath.Join(a.root, publicID), false); return err })
	return freed, err
}

// Sweep is lifecycle.PublicFiles.Sweep (P-8): every entry of public/ that is
// not a kept folder is removed, whatever its name or kind. A kept folder
// without index.html is removed whole; in one with index.html every entry
// the page does not reference is removed, temporary files included
// (QA-15-r1-1: a build stopped between its remove steps leaves no older data
// file behind for Caddy to serve).
func (a *Area) Sweep(keep map[string]struct{}) (removed int, freed int64, err error) {
	unlock, err := a.acquire()
	if err != nil {
		return 0, 0, err
	}
	defer unlock()
	entries, err := os.ReadDir(a.root)
	if err != nil {
		return 0, 0, fail("list", "public area", err)
	}
	var errs []error
	for _, e := range entries {
		p := filepath.Join(a.root, e.Name())
		if _, ok := keep[e.Name()]; ok && e.IsDir() {
			page, err := os.ReadFile(filepath.Join(p, indexName))
			if err == nil {
				n, f, err := a.sweepFolder(p, page)
				removed += n
				freed += f
				if err != nil {
					errs = append(errs, err)
				}
				continue
			}
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fail("read", "page", err))
				continue
			}
			// No page: Caddy shows the generic page, so nothing here may stay.
		}
		if e.IsDir() {
			n, err := a.removeFolder(p, false)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			freed += n
		} else {
			size := a.measure(p)
			if err := os.Remove(p); err != nil {
				errs = append(errs, fail("remove", "entry", err))
				continue
			}
			freed += size
		}
		removed++
	}
	return removed, freed, errors.Join(errs...)
}

// sweepFolder removes every entry of a kept folder that page does not
// reference (see referenced).
func (a *Area) sweepFolder(dir string, page []byte) (removed int, freed int64, err error) {
	inner, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, fail("list", "folder", err)
	}
	var errs []error
	for _, f := range inner {
		if f.Name() == indexName || f.Type().IsRegular() && referenced(page, f.Name()) {
			continue
		}
		q := filepath.Join(dir, f.Name())
		size := a.measure(q)
		if err := os.RemoveAll(q); err != nil {
			errs = append(errs, fail("remove", role(f.Name()), err))
			continue
		}
		removed++
		freed += size
	}
	return removed, freed, errors.Join(errs...)
}

// referenced reports whether page links to the file name: "/" + name with
// no file-name character or "/" after it (pages link by path, /w/{id}/name), or,
// for a pre-compressed "X.br", links to X (Caddy serves X.br in place of X).
// Dot-prefixed (temporary) names are never referenced.
func referenced(page []byte, name string) bool {
	if name == "" || name[0] == '.' {
		return false
	}
	if linked(page, name) {
		return true
	}
	base, ok := strings.CutSuffix(name, ".br")
	return ok && base != "" && linked(page, base)
}

func linked(page []byte, name string) bool {
	t := []byte("/" + name)
	for i := 0; ; {
		j := bytes.Index(page[i:], t)
		if j < 0 {
			return false
		}
		end := i + j + len(t)
		if end == len(page) || !nameChar(page[end]) && page[end] != '/' {
			return true
		}
		i += j + 1
	}
}

func nameChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c == '.' || c == '-' || c == '_'
}

// measure is usage for the bytes a removal frees. A measuring error counts
// the entry as 0 bytes (the total then drifts upwards, towards refusing
// uploads) and is logged without the path (P-14, QA-15-r1-6).
func (a *Area) measure(p string) int64 {
	size, err := usage(p)
	if err != nil && a.d.Logger != nil {
		a.d.Logger.Warn("publish: measure removed entry", "error", fail("measure", "entry", err).Error())
	}
	return size
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// usage is the sum of the sizes of the regular files under root (F-10's
// measure); links are not followed.
func usage(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.Type().IsRegular() {
			return nil
		}
		info, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// Assets returns every name that a page under public/ links as
// /assets/{name}, sorted and without repeats (F-30 RG-9). It takes no lock:
// a page written meanwhile names only the current bundle, so the list can
// only keep too much. A page removed while it is read is skipped.
func (a *Area) Assets() ([]string, error) {
	entries, err := os.ReadDir(a.root)
	if err != nil {
		return nil, fail("list", "public area", err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		page, err := os.ReadFile(filepath.Join(a.root, e.Name(), indexName))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fail("read", "page", err)
		}
		for rest := page; ; {
			i := bytes.Index(rest, []byte("/assets/"))
			if i < 0 {
				break
			}
			rest = rest[i+len("/assets/"):]
			n := 0
			for n < len(rest) && nameChar(rest[n]) {
				n++
			}
			if n > 0 && rest[0] != '.' {
				seen[string(rest[:n])] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	slices.Sort(out)
	return out, nil
}
