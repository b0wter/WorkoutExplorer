package publish

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/workout"
)

// Synthetic files only (ADR-004).
const (
	synthetic   = "../../testdata/corpus/synthetic"
	webDir      = "../../../web"    // the source template: Go tests stay independent of the web build (P-10)
	testBaseURL = "https://wx.test" // F-23 PC-8
	publicID    = "AbCdEfGhIjKlMnOpQrStUv"
	id          = "0123456789abcdef0123456789abcdef"
)

type originals map[string][]byte

func (o originals) Original(id string) ([]byte, error) {
	b, ok := o[id]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: "/data/private/" + id + "/original.fit", Err: fs.ErrNotExist}
	}
	return b, nil
}

type fixture struct {
	a     *Area
	root  string
	orig  []byte
	store *lifecycle.Storage
	steps []string
}

func newFixture(t *testing.T, file string) *fixture {
	t.Helper()
	tmpl, err := LoadTemplate(webDir, false)
	if err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(filepath.Join(synthetic, file))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fx := &fixture{orig: orig, store: lifecycle.NewStorage(1<<40, 0)}
	fx.a, err = Open(dir, Deps{Template: tmpl, Originals: originals{id: orig}, Parser: ingest.NewParser(ingest.DefaultLimits, nil), Storage: fx.store, BaseURL: testBaseURL})
	if err != nil {
		t.Fatal(err)
	}
	fx.root = filepath.Join(dir, "public")
	fx.a.step = func(s string) error { fx.steps = append(fx.steps, s); return nil }
	return fx
}

func draft() lifecycle.Record {
	created := time.Date(2026, 10, 23, 9, 15, 0, 0, time.UTC)
	return lifecycle.Record{
		ID: id, PublicID: publicID, Sport: "cycling", Visibility: lifecycle.Draft,
		CreatedAt: created, ExpiresAt: created.Add(7 * 24 * time.Hour),
	}
}

func published(r lifecycle.Record) lifecycle.Record {
	return lifecycle.ApplyPublish(r, r.CreatedAt.Add(time.Minute))
}

func (fx *fixture) change(t *testing.T, before, after lifecycle.Record) {
	t.Helper()
	if err := fx.a.WorkoutChanged(context.Background(), before, after); err != nil {
		t.Fatal(err)
	}
	fx.checkTotal(t)
}

// checkTotal: every byte under public/ went through the storage total (P-9).
func (fx *fixture) checkTotal(t *testing.T) {
	t.Helper()
	used, err := usage(fx.root)
	if err != nil {
		t.Fatal(err)
	}
	if fx.store.Used() != used {
		t.Errorf("storage total %d, bytes on disk %d", fx.store.Used(), used)
	}
}

func (fx *fixture) folder() string { return filepath.Join(fx.root, publicID) }

func (fx *fixture) names(t *testing.T) []string {
	t.Helper()
	es, err := os.ReadDir(fx.folder())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

var dataRE = regexp.MustCompile(`href="/w/` + publicID + `/(data\.[0-9a-f]{16}\.bin)"`)

var previewRE = regexp.MustCompile(`<meta property="og:image" content="` + regexp.QuoteMeta(testBaseURL) + `/w/` + publicID + `/(preview\.[0-9a-f]{16}\.png)" />`)

// previewOf is the preview page links (F-23 PC-7).
func previewOf(t *testing.T, page []byte) string {
	t.Helper()
	m := previewRE.FindSubmatch(page)
	if m == nil {
		t.Fatalf("page has no preview link")
	}
	return string(m[1])
}

// whole is the folder of a complete build: data file, its .br, the page and the preview.
func whole(t *testing.T, name string, page []byte) []string {
	return []string{name, name + ".br", indexName, previewOf(t, page)}
}

// complete checks that the folder holds a whole page: index.html and every
// file it points at. It returns the data file name, or "" without index.html.
func (fx *fixture) complete(t *testing.T) (string, []byte) {
	t.Helper()
	page, err := os.ReadFile(filepath.Join(fx.folder(), indexName))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		t.Fatal(err)
	}
	m := dataRE.FindSubmatch(page)
	if m == nil {
		t.Fatalf("page has no data link")
	}
	name := string(m[1])
	bin, err := os.ReadFile(filepath.Join(fx.folder(), name))
	if err != nil {
		t.Fatalf("page points at a missing data file: %v", err)
	}
	if _, err := datafile.Decode(bin); err != nil {
		t.Fatalf("data file: %v", err)
	}
	if img, err := os.ReadFile(filepath.Join(fx.folder(), previewOf(t, page))); err != nil {
		t.Fatalf("page points at a missing preview: %v", err)
	} else if c, err := png.DecodeConfig(bytes.NewReader(img)); err != nil || c.Width != 1200 || c.Height != 630 {
		t.Fatalf("preview: %+v, %v", c, err)
	}
	if br, err := os.ReadFile(filepath.Join(fx.folder(), name+".br")); err != nil || len(br) == 0 {
		t.Fatalf("compressed data file: %v", err)
	}
	return name, page
}

// TestPublish (AC 1, 2, 10): the first publish writes the page and both data
// files; the summary and laps are in the markup with the robots meta.
func TestPublish(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	if fx.names(t) != nil {
		t.Fatal("folder before publish")
	}
	fx.change(t, d, published(d))
	name, page := fx.complete(t)
	if name == "" {
		t.Fatal("no page")
	}
	if got := fx.names(t); !slices.Equal(got, whole(t, name, page)) {
		t.Errorf("files %v", got)
	}
	// With hiding off the public data file is the draft data file (OQ-69).
	w, err := ingest.NewParser(ingest.DefaultLimits, nil).Parse(context.Background(), fx.orig)
	if err != nil {
		t.Fatal(err)
	}
	draftBin, _ := datafile.Encode(w)
	if bin, _ := os.ReadFile(filepath.Join(fx.folder(), name)); !bytes.Equal(bin, draftBin) {
		t.Error("public data file differs from the draft's")
	}
	for _, s := range []string{
		`<meta name="robots" content="noindex, nofollow, noarchive"`,
		`<title>Ride, 23 Jun 2026</title>`, `<span class="n">13.46</span>`, `<td>2.24</td>`, `data-copy-link`,
		`href="/w/` + publicID + `"`, `Expires around 30 Oct 2026`,
	} {
		if !bytes.Contains(page, []byte(s)) {
			t.Errorf("page lacks %q", s)
		}
	}
	for _, mode := range []struct {
		p    string
		want fs.FileMode
	}{{fx.root, dirMode}, {fx.folder(), dirMode}, {filepath.Join(fx.folder(), name), fileMode}, {filepath.Join(fx.folder(), indexName), fileMode}} {
		info, _ := os.Stat(mode.p)
		if info.Mode().Perm() != mode.want {
			t.Errorf("%s: mode %v", filepath.Base(mode.p), info.Mode().Perm())
		}
	}
	noOriginalBytes(t, fx.root, fx.orig)
}

// noOriginalBytes is AC 10 (OQ-69): no file under public/ has the bytes of original.fit.
func noOriginalBytes(t *testing.T, root string, orig []byte) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if bytes.Equal(b, orig) || bytes.Contains(b, orig) {
			t.Errorf("%s has the bytes of original.fit", filepath.Base(p))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTitleOnly (AC 3): a title change rewrites index.html and the preview
// (the name is in the image, F-23); nothing is compressed again.
func TestTitleOnly(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	name, oldPage := fx.complete(t)
	binInfo, _ := os.Stat(filepath.Join(fx.folder(), name))
	fx.steps = nil
	q := p
	q.Title = "<b>Evening</b>"
	fx.change(t, p, q)
	got, page := fx.complete(t)
	if got != name {
		t.Errorf("data file %s became %s", name, got)
	}
	if previewOf(t, page) == previewOf(t, oldPage) || !slices.Equal(fx.names(t), whole(t, name, page)) {
		t.Errorf("title change: preview not replaced: %v", fx.names(t))
	}
	if !bytes.Contains(page, []byte("<title>&lt;b&gt;Evening&lt;/b&gt;</title>")) {
		t.Error("title not escaped into the page")
	}
	for _, s := range fx.steps {
		if strings.Contains(s, ".bin") || strings.HasPrefix(s, "remove") && s != "remove "+previewOf(t, oldPage) {
			t.Errorf("title change: step %q", s)
		}
	}
	if info, _ := os.Stat(filepath.Join(fx.folder(), name)); !info.ModTime().Equal(binInfo.ModTime()) {
		t.Error("data file rewritten")
	}
	noOriginalBytes(t, fx.root, fx.orig)
}

// TestNewDataFile (AC 4): a change of the public transform writes a new
// name; the old .bin and .br are gone.
func TestNewDataFile(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	old, _ := fx.complete(t)
	fx.a.transform = func(w workout.Workout, _ lifecycle.Record) (workout.Workout, bool) {
		w.Samples.HeartRate = nil
		return w, false
	}
	fx.change(t, p, p)
	name, page := fx.complete(t)
	if name == old {
		t.Fatal("same data file name")
	}
	if got := fx.names(t); !slices.Equal(got, whole(t, name, page)) {
		t.Errorf("files %v", got)
	}
	if bytes.Contains(page, []byte(old)) {
		t.Error("page names the old file")
	}
	noOriginalBytes(t, fx.root, fx.orig)
}

// TestNotUnlisted: a record that is not unlisted only loses its folder,
// index.html first; publishing again writes the same names.
func TestNotUnlisted(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	before := fx.names(t)
	fx.steps = nil
	fx.change(t, p, d)
	if _, err := os.Lstat(fx.folder()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("folder kept for a draft")
	}
	if len(fx.steps) < 1 || fx.steps[0] != "remove index.html" {
		t.Errorf("steps %v: index.html must go first", fx.steps)
	}
	fx.change(t, d, p)
	if got := fx.names(t); !slices.Equal(got, before) {
		t.Errorf("restored %v, was %v", got, before)
	}
}

func TestHidesMore(t *testing.T) {
	d := draft()
	p := published(d)
	with := func(r lifecycle.Record, edit func(*lifecycle.Record)) lifecycle.Record { edit(&r); return r }
	hidden := with(p, func(r *lifecycle.Record) { r.HideEnds = true })
	download := with(p, func(r *lifecycle.Record) { r.AllowDownload = true })
	for _, c := range []struct {
		name          string
		before, after lifecycle.Record
		want          bool
	}{
		{"nothing", p, p, false},
		{"title", p, with(p, func(r *lifecycle.Record) { r.Title = "x" }), false},
		{"description", p, with(p, func(r *lifecycle.Record) { r.Description = "x" }), false},
		{"lifetime", p, with(p, func(r *lifecycle.Record) { r.ExpiresAt = r.ExpiresAt.Add(time.Hour) }), false},
		{"publish", d, p, false},
		{"hiding off", hidden, p, false},
		{"download on", p, download, false},
		{"hiding on", p, hidden, true},
		{"download off", download, p, true},
		{"back to draft", p, with(p, func(r *lifecycle.Record) { r.Visibility = lifecycle.Draft }), true},
		// F-30 RG-6: derived from the original, never read by the public build.
		{"sport", p, with(p, func(r *lifecycle.Record) { r.Sport = "running" }), false},
		{"summary", p, with(p, func(r *lifecycle.Record) { r.SummaryJSON = "{}" }), false},
		{"start", p, with(p, func(r *lifecycle.Record) { t := r.CreatedAt; r.StartTime = &t }), false},
		{"time zone", p, with(p, func(r *lifecycle.Record) { o := 3600; r.TimeZoneOffsetS = &o }), false},
		{"data end", p, with(p, func(r *lifecycle.Record) { e := 60; r.DataEndS = &e }), false},
		{"hidden ranges", p, with(p, func(r *lifecycle.Record) { r.HiddenRanges = [][2]int{{0, 5}} }), false},
		{"derived and hiding on", p, with(hidden, func(r *lifecycle.Record) { r.SummaryJSON = "{}" }), true},
		{"created (unclassified)", p, with(p, func(r *lifecycle.Record) { r.CreatedAt = r.CreatedAt.Add(time.Second) }), true},
		{"token (unclassified)", p, with(p, func(r *lifecycle.Record) { r.EditTokenHash[0] ^= 1 }), true},
		{"title and hiding on", p, with(hidden, func(r *lifecycle.Record) { r.Title = "x" }), true},
	} {
		if got := HidesMore(c.before, c.after); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// TestRemoveFirst: a change that hides more removes index.html, then the
// folder, before anything is written (P-7).
func TestRemoveFirst(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	q := p
	q.HideEnds = true
	fx.steps = nil
	fx.change(t, p, q)
	if len(fx.steps) < 3 || fx.steps[0] != "remove index.html" || fx.steps[1] != "remove folder" || fx.steps[2] != "create folder" {
		t.Errorf("steps %v", fx.steps)
	}
	if name, _ := fx.complete(t); name == "" {
		t.Error("no page after the rebuild")
	}
}

type killed struct{}

// TestKill (AC 8): a stop at any file-system step of an in-place build, a
// remove-first build and a switch to a draft leaves the old complete page,
// the new complete page, or no index.html.
func TestKill(t *testing.T) {
	d := draft()
	p := published(d)
	titled := p
	titled.Title = "Renamed"
	hidden := p
	hidden.HideEnds = true
	changeData := func(a *Area) {
		a.transform = func(w workout.Workout, _ lifecycle.Record) (workout.Workout, bool) {
			w.Samples.Power = nil
			return w, false
		}
	}
	for _, c := range []struct {
		name          string
		before, after lifecycle.Record
		setup         func(*Area)
	}{
		{"first publish", d, p, nil},
		{"title", p, titled, nil},
		{"new data file", p, p, changeData},
		{"remove first", p, hidden, nil}, // the real F-16 cut: hiding on after publish
		{"to draft", p, d, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			// A dry run lists the steps.
			fx := prepared(t, c.before)
			if c.setup != nil {
				c.setup(fx.a)
			}
			oldName, oldPage := fx.complete(t)
			fx.steps = nil
			fx.change(t, c.before, c.after)
			newName, newPage := fx.complete(t)
			steps := fx.steps
			if len(steps) == 0 {
				t.Fatal("no steps")
			}
			for i := range steps {
				fx := prepared(t, c.before)
				if c.setup != nil {
					c.setup(fx.a)
				}
				n := 0
				fx.a.step = func(string) error {
					if n == i {
						panic(killed{})
					}
					n++
					return nil
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							if _, ok := r.(killed); !ok {
								panic(r)
							}
						}
					}()
					_ = fx.a.WorkoutChanged(context.Background(), c.before, c.after)
				}()
				name, page := fx.complete(t)
				switch {
				case name == "":
				case name == oldName && bytes.Equal(page, oldPage):
				case name == newName && bytes.Equal(page, newPage):
				default:
					t.Errorf("kill before %q: a page that is neither old nor new", steps[i])
				}
				// The startup sweep (row still unlisted) leaves a complete
				// page and nothing else, or no folder (QA-15-r1-1).
				fx.a.step = nil
				if _, _, err := fx.a.Sweep(map[string]struct{}{publicID: {}}); err != nil {
					t.Fatalf("kill before %q: sweep: %v", steps[i], err)
				}
				got := fx.names(t)
				switch {
				case name == "" && got != nil:
					t.Errorf("kill before %q: sweep left %v without a page", steps[i], got)
				case name != "" && !slices.Equal(got, whole(t, name, page)):
					t.Errorf("kill before %q: sweep left %v", steps[i], got)
				}
			}
		})
	}
}

// TestSweepAfterStoppedRemoveFirst (QA-15-r1-1): a remove-first build that
// fails after index.html is gone leaves the older data file; the sweep with
// the row's folder kept removes it, and a kept folder whose page references
// only some files loses the rest.
func TestSweepAfterStoppedRemoveFirst(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	old, _ := fx.complete(t)
	hidden := p
	hidden.HideEnds = true
	fx.a.step = func(s string) error {
		if s == "remove folder" {
			return syscall.EACCES
		}
		return nil
	}
	if err := fx.a.WorkoutChanged(context.Background(), p, hidden); err == nil {
		t.Fatal("no error")
	}
	if got := fx.names(t); !slices.Contains(got, old) || slices.Contains(got, indexName) {
		t.Fatalf("probe state not reached: %v", got)
	}
	fx.a.step = nil
	n, freed, err := fx.a.Sweep(map[string]struct{}{publicID: {}})
	if err != nil || n != 1 {
		t.Errorf("sweep: %d %v", n, err)
	}
	fx.store.Release(freed) // the caller releases
	if _, err := os.Lstat(fx.folder()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("folder without a page kept: %v", fx.names(t))
	}

	// A page that references a newer data file: the older one and stray
	// entries go, the page and its files stay.
	fx.change(t, d, p)
	files := fx.names(t)
	for _, f := range []string{"data.0000000000000000.bin", "data.0000000000000000.bin.br", "preview.0000000000000000.png", "preview.png", ".x.tmp"} {
		if err := os.WriteFile(filepath.Join(fx.folder(), f), []byte("x"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(fx.folder(), "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	n, freed, err = fx.a.Sweep(map[string]struct{}{publicID: {}})
	if err != nil || n != 6 || freed != 5 { // the linked preview stays, an older one goes (F-23 PC-7)
		t.Errorf("sweep: %d %d %v", n, freed, err)
	}
	if got := fx.names(t); !slices.Equal(got, files) {
		t.Errorf("kept folder %v, want %v", got, files)
	}
}

func TestReferenced(t *testing.T) {
	page := []byte(`<link href="/w/X/data.ab.bin"><meta content="https://h/w/X/preview.cd.png">preview.ef.png`)
	for _, c := range []struct {
		name string
		want bool
	}{
		{"data.ab.bin", true},
		{"data.ab.bin.br", true},
		{"preview.cd.png", true},
		{"preview.cd.png.br", true},
		{"preview.ef.png", false},
		{"ab.bin", false},
		{"data.ab", false},
		{"data.ab.bin.gz", false},
		{".br", false},
		{".data.ab.bin.123.tmp", false},
		{"w", false},
		{"", false},
	} {
		if got := referenced(page, c.name); got != c.want {
			t.Errorf("referenced(%q) = %v", c.name, got)
		}
	}
}

// prepared returns a fixture whose workout is in state r (published if unlisted).
func prepared(t *testing.T, r lifecycle.Record) *fixture {
	t.Helper()
	fx := newFixture(t, "ride-30min.fit")
	if r.Visibility == lifecycle.Unlisted {
		fx.change(t, draft(), r)
	}
	return fx
}

// TestErrorsCarryNoPublicPath (P-14, privacy row): an injected file-system
// failure at any step gives an error without the public_id or a public path.
func TestErrorsCarryNoPublicPath(t *testing.T) {
	d := draft()
	p := published(d)
	hidden := p
	hidden.HideEnds = true
	for _, c := range []struct {
		name          string
		before, after lifecycle.Record
	}{{"publish", d, p}, {"remove first", p, hidden}, {"to draft", p, d}} {
		fx := prepared(t, c.before)
		fx.steps = nil
		fx.change(t, c.before, c.after)
		steps := fx.steps
		for i := range steps {
			fx := prepared(t, c.before)
			n := 0
			fx.a.step = func(string) error {
				defer func() { n++ }()
				if n == i {
					return &fs.PathError{Op: "open", Path: filepath.Join(fx.folder(), "x"), Err: syscall.EACCES}
				}
				return nil
			}
			err := fx.a.WorkoutChanged(context.Background(), c.before, c.after)
			if err == nil {
				t.Errorf("%s, step %q: no error", c.name, steps[i])
				continue
			}
			if strings.Contains(err.Error(), publicID) || strings.Contains(err.Error(), "public/") || strings.Contains(err.Error(), fx.root) {
				t.Errorf("%s, step %q: %v", c.name, steps[i], err)
			}
			if !errors.Is(err, syscall.EACCES) {
				t.Errorf("%s: cause lost: %v", c.name, err)
			}
			fx.checkTotal(t)
		}
	}
	// Remove and a missing original, too.
	fx := prepared(t, p)
	fx.a.step = func(string) error { return &fs.PathError{Op: "remove", Path: fx.folder(), Err: syscall.EACCES} }
	if _, err := fx.a.Remove(publicID); err == nil || strings.Contains(err.Error(), publicID) {
		t.Errorf("Remove: %v", err)
	}
	fx.a.step = nil
	fx.a.d.Originals = originals{}
	if err := fx.a.WorkoutChanged(context.Background(), p, p); err == nil || strings.Contains(err.Error(), publicID) || strings.Contains(err.Error(), id) {
		t.Errorf("original: %v", err)
	}
}

// TestRejectedOriginal: a stored original the parser rejects is an error,
// and the folder is not touched beyond a remove-first step (P-14).
func TestRejectedOriginal(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	fx.a.d.Originals = originals{id: []byte("not a fit file at all")}
	d := draft()
	if err := fx.a.WorkoutChanged(context.Background(), d, published(d)); err == nil {
		t.Fatal("no error")
	}
	if fx.names(t) != nil {
		t.Error("files written")
	}
}

func TestRemoveAndSweep(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	held, _ := usage(fx.folder())
	freed, err := fx.a.Remove(publicID)
	if err != nil || freed != held || held == 0 {
		t.Fatalf("freed %d of %d: %v", freed, held, err)
	}
	fx.store.Release(freed) // the caller releases (F-10 S-6)
	if n, err := fx.a.Remove(publicID); n != 0 || err != nil {
		t.Errorf("absent: %d %v", n, err)
	}
	for _, bad := range []string{"", "..", "../private", strings.Repeat("a", 21), strings.Repeat("a", 22) + "/x"} {
		if _, err := fx.a.Remove(bad); err == nil {
			t.Errorf("Remove(%q)", bad)
		}
		if err := fx.a.WorkoutChanged(context.Background(), d, lifecycle.Record{PublicID: bad, Visibility: lifecycle.Unlisted}); err == nil {
			t.Errorf("WorkoutChanged(%q)", bad)
		}
	}

	// Sweep: the kept folder keeps its files and loses temporary ones;
	// everything else goes.
	fx.change(t, d, p)
	files := fx.names(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(fx.folder(), ".index.html.tmp"), []byte("x"), 0o640))
	other := filepath.Join(fx.root, "ZZZZZZZZZZZZZZZZZZZZZZ")
	must(os.MkdirAll(filepath.Join(other, "sub"), 0o750))
	must(os.WriteFile(filepath.Join(other, indexName), []byte("old"), 0o640))
	must(os.WriteFile(filepath.Join(fx.root, "stray.txt"), []byte("x"), 0o640))
	must(os.WriteFile(filepath.Join(fx.root, ".tmp"), []byte("x"), 0o640))
	must(os.Symlink("/etc", filepath.Join(fx.root, "link")))
	n, freed, err := fx.a.Sweep(map[string]struct{}{publicID: {}, "stray.txt": {}})
	if err != nil || n != 5 || freed != 6 {
		t.Errorf("sweep: %d %d %v", n, freed, err)
	}
	es, _ := os.ReadDir(fx.root)
	if len(es) != 1 || es[0].Name() != publicID {
		t.Errorf("left %v", es)
	}
	if got := fx.names(t); !slices.Equal(got, files) {
		t.Errorf("kept folder %v, was %v", got, files)
	}
	if _, err := os.Stat("/etc"); err != nil {
		t.Error("link target touched")
	}
}

// TestLoadTemplate (P-10): missing or unparsable templates and a template
// that names an unknown field fail at load.
func TestLoadTemplate(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadTemplate(dir, false); err == nil {
		t.Error("missing template")
	}
	for _, src := range []string{"{{.Page.Title", "{{.Page.Nope}}", "{{range .Laps.Missing}}{{end}}"} {
		os.WriteFile(filepath.Join(dir, "index.html"), []byte(src+"<!--wx:impressum-->"), 0o600)
		if _, err := LoadTemplate(dir, false); err == nil {
			t.Errorf("%q loaded", src)
		}
	}
	// F-38 IM-7: the footer marker is required once and filled before parsing.
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>{{.Page.Title}}</title>"), 0o600)
	if _, err := LoadTemplate(dir, false); err == nil {
		t.Error("template without the footer marker loaded")
	}
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>{{.Page.Title}}</title><footer><!--wx:impressum--></footer>"), 0o600)
	for on, want := range map[bool]string{true: `<title>t</title><footer><a href="/impressum">Impressum</a></footer>`, false: "<title>t</title><footer></footer>"} {
		tmpl, err := LoadTemplate(dir, on)
		var b strings.Builder
		if err != nil || tmpl.Execute(&b, probe) != nil || b.String() != want {
			t.Errorf("impressum=%v: %q %v", on, b.String(), err)
		}
	}
}

// ADR-028's trigger: a title save of synthetic ride-12h above 1 s on the
// reference machine. Run: go test ./internal/publish -run '^$' -bench Ride12h
// (recorded in testdata/corpus/MEASUREMENTS.md).
func benchFixture(b *testing.B) *fixture {
	b.Helper()
	tmpl, err := LoadTemplate(webDir, false)
	if err != nil {
		b.Fatal(err)
	}
	orig, err := os.ReadFile(filepath.Join(synthetic, "ride-12h.fit"))
	if err != nil {
		b.Fatal(err)
	}
	dir := b.TempDir()
	fx := &fixture{orig: orig}
	fx.a, err = Open(dir, Deps{Template: tmpl, Originals: originals{id: orig}, Parser: ingest.NewParser(ingest.DefaultLimits, nil), BaseURL: testBaseURL})
	if err != nil {
		b.Fatal(err)
	}
	fx.root = filepath.Join(dir, "public")
	return fx
}

func BenchmarkPublishRide12h(b *testing.B) {
	fx := benchFixture(b)
	d := draft()
	p := published(d)
	for b.Loop() {
		if _, err := fx.a.Remove(publicID); err != nil {
			b.Fatal(err)
		}
		if err := fx.a.WorkoutChanged(context.Background(), d, p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTitleSaveRide12h(b *testing.B) {
	fx := benchFixture(b)
	d := draft()
	p := published(d)
	if err := fx.a.WorkoutChanged(context.Background(), d, p); err != nil {
		b.Fatal(err)
	}
	q := p
	for i := 0; b.Loop(); i++ {
		next := q
		next.Title = "Title " + strings.Repeat("x", i%7)
		if err := fx.a.WorkoutChanged(context.Background(), q, next); err != nil {
			b.Fatal(err)
		}
		q = next
	}
}

// TestTempNamesUnique (QA-15-r1-4): a file of the old fixed temporary name,
// put there by another build just before index.html is written, is neither
// reused nor renamed away by this build.
func TestTempNamesUnique(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	stale := filepath.Join(fx.folder(), "."+indexName+".tmp")
	q := p
	q.Title = "Another title"
	var checked bool
	fx.a.step = func(s string) error {
		switch s {
		case "write " + indexName:
			return os.WriteFile(stale, []byte("other build"), 0o640)
		case "sync folder": // after the rename of index.html
			checked = true
			if b, err := os.ReadFile(stale); err != nil || string(b) != "other build" {
				t.Errorf("fixed temporary name used: %q %v", b, err)
			}
		}
		return nil
	}
	if err := fx.a.WorkoutChanged(context.Background(), p, q); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("hook not reached")
	}
}

// TestMeasureWarnNoPath (QA-15-r1-6, P-14): the WARN line of a failed
// measurement carries neither a path nor the public_id.
func TestMeasureWarnNoPath(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	var buf bytes.Buffer
	fx.a.d.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	if n := fx.a.measure(filepath.Join(fx.folder(), "gone")); n != 0 {
		t.Errorf("size %d", n)
	}
	line := buf.String()
	if !strings.Contains(line, `"WARN"`) {
		t.Fatalf("no WARN line: %q", line)
	}
	for _, bad := range []string{publicID, "public/", fx.root} {
		if strings.Contains(line, bad) {
			t.Errorf("line contains %q: %s", bad, line)
		}
	}
}

// TestQAR2SweepDuringOtherBuild (QA-15-r2-1, ADR-031): a second Area on the
// same data directory, standing in for serve's sweep, sweeps while the first,
// standing in for regenerate, is between its new data file and its page. The
// sweep waits for the public lock, so the page never links a missing file.
// The same holds for Remove from the other Area.
func TestQAR2SweepDuringOtherBuild(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	other, err := Open(filepath.Dir(fx.root), fx.a.d)
	if err != nil {
		t.Fatal(err)
	}
	d := draft()
	hidden := published(d)
	hidden.HideEnds = true
	fx.change(t, d, hidden)
	old, _ := fx.complete(t)
	shown := hidden
	shown.HideEnds = false // shows more: in place, with a new data file

	keep := map[string]struct{}{publicID: {}}
	var swept chan error
	fx.a.step = func(s string) error {
		if s != "write "+indexName || swept != nil {
			return nil
		}
		swept = make(chan error, 1)
		go func() { _, _, err := other.Sweep(keep); swept <- err }()
		select {
		case err := <-swept:
			t.Errorf("sweep ran during the write phase: %v", err)
			swept <- err
		case <-time.After(200 * time.Millisecond):
		}
		return nil
	}
	if err := fx.a.WorkoutChanged(context.Background(), hidden, shown); err != nil {
		t.Fatal(err)
	}
	if swept == nil {
		t.Fatal("no write of index.html")
	}
	if err := <-swept; err != nil {
		t.Fatal(err)
	}
	name, _ := fx.complete(t)
	if name == "" || name == old {
		t.Fatalf("page %q, old %q", name, old)
	}

	// Remove from the other Area waits too: the folder is either whole or gone.
	var removed chan error
	fx.a.step = func(s string) error {
		if s != "write "+indexName || removed != nil {
			return nil
		}
		removed = make(chan error, 1)
		go func() { _, err := other.Remove(publicID); removed <- err }()
		select {
		case err := <-removed:
			t.Errorf("remove ran during the write phase: %v", err)
			removed <- err
		case <-time.After(200 * time.Millisecond):
		}
		return nil
	}
	fx.a.transform = func(w workout.Workout, _ lifecycle.Record) (workout.Workout, bool) {
		w.Samples.HeartRate = nil
		return w, false
	}
	if err := fx.a.WorkoutChanged(context.Background(), shown, shown); err != nil {
		t.Fatal(err)
	}
	if err := <-removed; err != nil {
		t.Fatal(err)
	}
	if got := fx.names(t); got != nil {
		t.Errorf("left %v", got)
	}
}

// TestRemoveFirstHoldsLock (ADR-031 item 2, QA-21-r1-3): another Area's
// sweep waits while the remove-first step runs.
func TestRemoveFirstHoldsLock(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	other, err := Open(filepath.Dir(fx.root), fx.a.d)
	if err != nil {
		t.Fatal(err)
	}
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	q := p
	q.HideEnds = true
	var swept chan error
	fx.a.step = func(s string) error {
		if s != "remove folder" || swept != nil {
			return nil
		}
		swept = make(chan error, 1)
		go func() { _, _, err := other.Sweep(nil); swept <- err }()
		select {
		case err := <-swept:
			t.Errorf("sweep ran during the remove-first step: %v", err)
			swept <- err
		case <-time.After(200 * time.Millisecond):
		}
		return nil
	}
	if err := fx.a.WorkoutChanged(context.Background(), p, q); err != nil {
		t.Fatal(err)
	}
	if swept == nil {
		t.Fatal("no remove-first step")
	}
	if err := <-swept; err != nil {
		t.Fatal(err)
	}
}

// TestPublicLockFile (ADR-031 item 1): public.lock is beside public/, mode
// 0600, and an Area holding it blocks another until it lets go.
func TestPublicLockFile(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	p := filepath.Join(filepath.Dir(fx.root), "public.lock")
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("lock file: %v %v", info, err)
	}
	unlock, err := fx.a.acquire()
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{})
	go func() {
		u, err := fx.a.acquire()
		if err == nil {
			u()
		}
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("second acquire did not wait")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	<-got
}

// TestPublicLockAcrossProcesses (ADR-031 items 1, 4): another process holding
// the lock blocks Sweep, and its death releases the lock.
func TestPublicLockAcrossProcesses(t *testing.T) {
	if dir := os.Getenv("WX_HOLD_PUBLIC_LOCK"); dir != "" {
		a, err := Open(dir, Deps{})
		if err != nil {
			os.Exit(2)
		}
		if _, err := a.acquire(); err != nil {
			os.Exit(3)
		}
		os.Stdout.WriteString("locked\n")
		time.Sleep(time.Minute) // until killed
		os.Exit(0)
	}
	fx := newFixture(t, "ride-30min.fit")
	cmd := exec.Command(os.Args[0], "-test.run=^TestPublicLockAcrossProcesses$")
	cmd.Env = append(os.Environ(), "WX_HOLD_PUBLIC_LOCK="+filepath.Dir(fx.root))
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	buf := make([]byte, 7)
	if _, err := io.ReadFull(out, buf); err != nil || string(buf) != "locked\n" {
		t.Fatalf("helper: %q %v", buf, err)
	}
	swept := make(chan error, 1)
	go func() { _, _, err := fx.a.Sweep(nil); swept <- err }()
	select {
	case err := <-swept:
		t.Fatalf("sweep ran while another process held the lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-swept:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock not released when its holder died")
	}
}
