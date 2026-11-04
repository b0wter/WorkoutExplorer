package lifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"workout-explorer/internal/ingest"
	"workout-explorer/internal/workout"
)

// ---- fakes -------------------------------------------------------------------

type fakeParser struct {
	acc ingest.Accepted
	err error
	got []byte
}

func (p *fakeParser) Accept(_ context.Context, upload []byte) (ingest.Accepted, error) {
	p.got = upload
	return p.acc, p.err
}

type fakeRepo struct {
	mu        sync.Mutex
	rows      map[string]Record
	insertErr error
	inserts   int
	// storeOnInsertErr makes InsertWorkout store the row and still report an
	// error: the driver's misreport of QA-09-r1-1.
	storeOnInsertErr bool
	existsErr        error
	lookupErr        error // WorkoutByPublicID fails with this
	// onInsert and onExists run first, with the context the repository was given.
	onInsert func(context.Context)
	onExists func(context.Context)
	deleted  []string
	dueErr   error

	checkpoints   int
	checkpointErr error
	derivedErr    error // UpdateDerived fails with this (F-30)
}

func newRepo() *fakeRepo { return &fakeRepo{rows: map[string]Record{}} }

func (r *fakeRepo) InsertWorkout(ctx context.Context, rec Record) error {
	if r.onInsert != nil {
		r.onInsert(ctx)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inserts++
	if r.insertErr != nil {
		if r.storeOnInsertErr {
			r.rows[rec.ID] = rec
		}
		return r.insertErr
	}
	r.rows[rec.ID] = rec
	return nil
}

func (r *fakeRepo) WorkoutExists(ctx context.Context, id string) (bool, error) {
	if r.onExists != nil {
		r.onExists(ctx)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.existsErr != nil {
		return false, r.existsErr
	}
	_, ok := r.rows[id]
	return ok, nil
}

func (r *fakeRepo) DeleteWorkout(_ context.Context, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted = append(r.deleted, id)
	_, ok := r.rows[id]
	delete(r.rows, id)
	return ok, nil
}

func (r *fakeRepo) LivePublicIDs(_ context.Context, now time.Time) (map[string]struct{}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]struct{}{}
	for _, rec := range r.rows {
		if rec.Visibility == Unlisted && rec.ExpiresAt.After(now) {
			out[rec.PublicID] = struct{}{}
		}
	}
	return out, nil
}

func (r *fakeRepo) DueWorkouts(_ context.Context, now time.Time) ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dueErr != nil {
		return nil, r.dueErr
	}
	var out []Record
	for _, rec := range r.rows {
		if !rec.Live(now) {
			out = append(out, rec)
		}
	}
	slices.SortFunc(out, func(a, b Record) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (r *fakeRepo) Checkpoint(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkpoints++
	return r.checkpointErr
}

func (r *fakeRepo) WorkoutIDs(context.Context) (map[string]struct{}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]struct{}{}
	for id := range r.rows {
		out[id] = struct{}{}
	}
	return out, nil
}

type fakeFiles struct {
	mu        sync.Mutex
	dirs      map[string][]File
	createErr error
	removeErr error
	creates   int
	removed   []string
	staging   int
	unknown   int
	// F-30: Replace fails with replaceErr; replaced lists the files written.
	replaceErr error
	replaced   []string
}

func newFiles() *fakeFiles { return &fakeFiles{dirs: map[string][]File{}} }

func (f *fakeFiles) Create(id string, files []File) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	if f.createErr != nil {
		return f.createErr
	}
	f.dirs[id] = files
	return nil
}

func (f *fakeFiles) Remove(id string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	if f.removeErr != nil {
		return 0, f.removeErr
	}
	var n int64
	for _, file := range f.dirs[id] {
		n += int64(len(file.Data))
	}
	delete(f.dirs, id)
	return n, nil
}

func (f *fakeFiles) IDs() ([]string, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for id := range f.dirs {
		ids = append(ids, id)
	}
	return ids, f.unknown, nil
}

func (f *fakeFiles) ClearStaging() (int, error) { return f.staging, nil }

type lines struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lines) all(t *testing.T) []map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	for _, s := range strings.Split(strings.TrimSpace(l.buf.String()), "\n") {
		if s == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

var t0 = time.Date(2026, 10, 16, 12, 30, 15, 123_456_789, time.FixedZone("x", 3600))

func smallWorkout() workout.Workout {
	return workout.Workout{
		Sport:     "cycling",
		StartTime: time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC),
		TimeZone:  workout.TimeZone{Known: true, OffsetSeconds: 7200},
		Samples:   workout.Samples{Offset: []int32{0, 1, 2}, Speed: []float64{1, 2, 3}},
	}
}

func fixture(t *testing.T) (*Service, *fakeParser, *fakeRepo, *fakeFiles, *lines) {
	t.Helper()
	p := &fakeParser{acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("STRIPPED")}}
	r, f, l := newRepo(), newFiles(), &lines{}
	s := New(Deps{Parser: p, Repo: r, Files: f, Logger: slog.New(slog.NewJSONHandler(l, nil)), Now: func() time.Time { return t0 }})
	return s, p, r, f, l
}

// ---- identity ---------------------------------------------------------------------

// divBase62 is the hand-written reference: repeated division by 62.
func divBase62(b []byte) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	v := new(big.Int).SetBytes(b)
	m, r := big.NewInt(62), new(big.Int)
	out := make([]byte, 22)
	for i := 21; i >= 0; i-- {
		v.DivMod(v, m, r)
		out[i] = digits[r.Int64()]
	}
	return string(out)
}

func TestBase62(t *testing.T) {
	ff := bytes.Repeat([]byte{0xFF}, 16)
	one := make([]byte, 16)
	one[15] = 1
	sixtytwo := make([]byte, 16)
	sixtytwo[15] = 62
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"zero", make([]byte, 16), strings.Repeat("0", 22)},
		{"one", one, strings.Repeat("0", 21) + "1"},
		{"62", sixtytwo, strings.Repeat("0", 20) + "10"},
		{"all ff", ff, divBase62(ff)},
	}
	for _, tc := range tests {
		if got := base62(tc.in); got != tc.want {
			t.Errorf("%s: base62 = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := base62(ff); len(got) != 22 {
		t.Errorf("all ff: %d characters", len(got))
	}
	// 10,000 seeded values against the division loop.
	seed := uint64(1)
	for range 10_000 {
		b := make([]byte, 16)
		for i := range b {
			seed = seed*6364136223846793005 + 1442695040888963407
			b[i] = byte(seed >> 56)
		}
		if got, want := base62(b), divBase62(b); got != want {
			t.Fatalf("base62(% x) = %q, want %q", b, got, want)
		}
	}
}

func TestNewIdentity(t *testing.T) {
	src := make([]byte, 48)
	for i := range src {
		src[i] = byte(i + 1)
	}
	id, err := NewIdentity(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if want := "0102030405060708090a0b0c0d0e0f10"; id.ID != want {
		t.Errorf("ID = %q, want %q", id.ID, want)
	}
	if want := divBase62(src[16:32]); id.PublicID != want {
		t.Errorf("PublicID = %q, want %q", id.PublicID, want)
	}
	if want := divBase62(src[32:]); id.Token != want {
		t.Errorf("Token = %q, want %q", id.Token, want)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id.ID) ||
		!regexp.MustCompile(`^[0-9A-Za-z]{22}$`).MatchString(id.PublicID) ||
		!regexp.MustCompile(`^[0-9A-Za-z]{22}$`).MatchString(id.Token) {
		t.Errorf("formats: %+v", id)
	}
	if want := sha256.Sum256([]byte(id.Token)); id.TokenHash != want {
		t.Errorf("hash is not SHA-256 of the token")
	}
	if bytes.Contains(id.TokenHash[:], []byte(id.Token)) || id.TokenHash != HashToken(id.Token) {
		t.Errorf("hash equals or contains the token")
	}
	if _, err := NewIdentity(bytes.NewReader(src[:47])); err == nil {
		t.Errorf("a short reader gives no error")
	}
	if ViewPath("abc") != "/w/abc" || EditLink("abc", "tok") != "/edit/abc#tok" {
		t.Errorf("links: %q %q", ViewPath("abc"), EditLink("abc", "tok"))
	}
}

// TestUploadPublicIDs (AC 13): 10,000 uploads with the real crypto/rand.
func TestUploadPublicIDs(t *testing.T) {
	p := &fakeParser{acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("x")}}
	s := New(Deps{Parser: p, Repo: newRepo(), Files: newFiles()})
	re := regexp.MustCompile(`^[0-9A-Za-z]{22}$`)
	publics, tokens, hashes := map[string]bool{}, map[string]bool{}, map[[32]byte]bool{}
	for range 10_000 {
		c, err := s.Upload(context.Background(), []byte("up"))
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(c.PublicID) || !re.MatchString(c.EditToken) {
			t.Fatalf("formats: %+v", c)
		}
		publics[c.PublicID], tokens[c.EditToken], hashes[HashToken(c.EditToken)] = true, true, true
	}
	if len(publics) != 10_000 || len(tokens) != 10_000 || len(hashes) != 10_000 {
		t.Errorf("distinct: %d public IDs, %d tokens, %d hashes", len(publics), len(tokens), len(hashes))
	}
	r := s.d.Repo.(*fakeRepo)
	ids := map[string]bool{}
	for id := range r.rows {
		ids[id] = true
	}
	if len(ids) != 10_000 {
		t.Errorf("%d distinct IDs", len(ids))
	}
}

func TestNewDraft(t *testing.T) {
	id := Identity{ID: strings.Repeat("a", 32), PublicID: strings.Repeat("P", 22), TokenHash: [32]byte{1}}
	w := smallWorkout()
	r := NewDraft(w, []byte(`{"a":1}`), id, t0)
	if r.Visibility != Draft || r.PublishedAt != nil || r.HideEnds || r.AllowDownload || r.Title != "" || r.Description != "" {
		t.Errorf("defaults: %+v", r)
	}
	if got := r.ExpiresAt.Sub(r.CreatedAt); got != 604_800*time.Second {
		t.Errorf("lifetime = %v", got)
	}
	if r.CreatedAt.Location() != time.UTC || r.CreatedAt.Nanosecond() != 0 || !r.CreatedAt.Equal(t0.Truncate(time.Second)) {
		t.Errorf("CreatedAt = %v", r.CreatedAt)
	}
	if r.DataEndS != nil || r.StartTime == nil || !r.StartTime.Equal(w.StartTime) || r.TimeZoneOffsetS == nil || *r.TimeZoneOffsetS != 7200 {
		t.Errorf("fields: %+v", r)
	}
	if r.SummaryJSON != `{"a":1}` || r.Sport != "cycling" || r.EditTokenHash != id.TokenHash {
		t.Errorf("fields: %+v", r)
	}
	if r.HiddenRanges != nil {
		t.Errorf("no positions: ranges %v", r.HiddenRanges)
	}
	// ADR-030: the upload stores what hiding would cut, with hiding off.
	// About 333 m between fixes: samples 0 and 4 are hidden.
	line := w
	line.Samples.Offset = []int32{0, 1, 2, 3, 4}
	line.Samples.Lat = []float64{50, 50.003, 50.006, 50.009, 50.012}
	line.Samples.Lon = []float64{8, 8, 8, 8, 8}
	if r := NewDraft(line, nil, id, t0); !reflect.DeepEqual(r.HiddenRanges, [][2]int{{0, 0}, {4, 4}}) || r.HideEnds {
		t.Errorf("ranges %v, hide_ends %v", r.HiddenRanges, r.HideEnds)
	}
	w.Integrity = workout.Integrity{Truncated: true, DataEnd: 321}
	w.StartTime = time.Time{}
	w.TimeZone = workout.TimeZone{}
	r = NewDraft(w, nil, id, t0)
	if r.DataEndS == nil || *r.DataEndS != 321 || r.StartTime != nil || r.TimeZoneOffsetS != nil {
		t.Errorf("truncated/unknown: %+v", r)
	}
	if MaxLifetime != 30*24*time.Hour {
		t.Errorf("MaxLifetime = %v", MaxLifetime)
	}
}

// ---- Upload -----------------------------------------------------------------------

func TestUploadStoresStripped(t *testing.T) {
	s, p, r, f, l := fixture(t)
	upload := []byte("UPLOAD-WITH-A-SERIAL-NUMBER")
	c, err := s.Upload(context.Background(), upload)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.got, upload) {
		t.Errorf("the parser got other bytes")
	}
	if len(f.dirs) != 1 || len(r.rows) != 1 {
		t.Fatalf("%d folders, %d rows", len(f.dirs), len(r.rows))
	}
	for id, files := range f.dirs {
		if len(files) != 2 || files[0].Name != OriginalName || files[1].Name != DraftName {
			t.Fatalf("files = %+v", files)
		}
		if string(files[0].Data) != "STRIPPED" {
			t.Errorf("original.fit = %q, want the stripped bytes", files[0].Data)
		}
		if bytes.Contains(files[1].Data, upload) {
			t.Errorf("draft.bin holds the upload")
		}
		rec := r.rows[id]
		if rec.PublicID != c.PublicID || rec.EditTokenHash != HashToken(c.EditToken) {
			t.Errorf("row does not match the response")
		}
	}
	for _, m := range l.all(t) {
		if b, _ := json.Marshal(m); strings.Contains(string(b), string(upload)) {
			t.Errorf("the upload appears in a log line")
		}
	}
}

func TestUploadFailures(t *testing.T) {
	t.Run("parser error is returned intact", func(t *testing.T) {
		s, p, r, f, _ := fixture(t)
		p.err = &ingest.Error{Kind: ingest.KindNotFIT}
		_, err := s.Upload(context.Background(), []byte("x"))
		var ie *ingest.Error
		if !errors.As(err, &ie) || ie.Kind != ingest.KindNotFIT {
			t.Fatalf("err = %v", err)
		}
		if f.creates != 0 || r.inserts != 0 {
			t.Errorf("creates %d inserts %d", f.creates, r.inserts)
		}
	})
	t.Run("encode error", func(t *testing.T) {
		s, p, r, f, l := fixture(t)
		p.acc.Workout.Laps = []workout.Lap{{Start: 10, End: 5}}
		_, err := s.Upload(context.Background(), []byte("x"))
		var se *ServerError
		if !errors.As(err, &se) || se.Stage != "encode" {
			t.Fatalf("err = %v", err)
		}
		if f.creates != 0 || r.inserts != 0 {
			t.Errorf("creates %d inserts %d", f.creates, r.inserts)
		}
		if got := l.all(t); len(got) != 1 || got[0]["stage"] != "encode" || got[0]["level"] != "ERROR" {
			t.Errorf("log = %v", got)
		}
	})
	t.Run("identity error", func(t *testing.T) {
		s, _, r, f, _ := fixture(t)
		s.d.Rand = strings.NewReader("short")
		_, err := s.Upload(context.Background(), []byte("x"))
		var se *ServerError
		if !errors.As(err, &se) || se.Stage != "identity" || f.creates != 0 || r.inserts != 0 {
			t.Fatalf("err = %v creates %d", err, f.creates)
		}
	})
	t.Run("create error", func(t *testing.T) {
		s, _, r, f, _ := fixture(t)
		f.createErr = errors.New("disk full")
		_, err := s.Upload(context.Background(), []byte("x"))
		var se *ServerError
		if !errors.As(err, &se) || se.Stage != "files" || r.inserts != 0 {
			t.Fatalf("err = %v inserts %d", err, r.inserts)
		}
	})
	t.Run("insert error removes the folder", func(t *testing.T) {
		s, _, r, f, l := fixture(t)
		r.insertErr = errors.New("db down")
		_, err := s.Upload(context.Background(), []byte("x"))
		var se *ServerError
		if !errors.As(err, &se) || se.Stage != "store" {
			t.Fatalf("err = %v", err)
		}
		if len(f.removed) != 1 || len(f.dirs) != 0 {
			t.Errorf("removed %v, dirs %d", f.removed, len(f.dirs))
		}
		for id := range r.rows {
			t.Errorf("row %s left behind", id)
		}
		if len(l.all(t)) != 1 {
			t.Errorf("log = %v", l.all(t))
		}
	})
	t.Run("insert and cleanup error", func(t *testing.T) {
		s, _, r, f, l := fixture(t)
		r.insertErr, f.removeErr = errors.New("db down"), errors.New("fs down")
		if _, err := s.Upload(context.Background(), []byte("x")); err == nil {
			t.Fatal("no error")
		}
		got := l.all(t)
		if len(got) != 2 || got[0]["stage"] != "cleanup" || got[1]["stage"] != "store" {
			t.Errorf("log = %v", got)
		}
	})
	t.Run("insert error but the row exists", func(t *testing.T) {
		s, _, r, f, l := fixture(t)
		r.insertErr, r.storeOnInsertErr = errors.New("context canceled"), true
		c, err := s.Upload(context.Background(), []byte("x"))
		if err != nil || c.PublicID == "" || c.EditToken == "" {
			t.Fatalf("Created %+v, err %v", c, err)
		}
		if len(f.removed) != 0 || len(f.dirs) != 1 || len(r.rows) != 1 {
			t.Errorf("removed %v, dirs %d, rows %d", f.removed, len(f.dirs), len(r.rows))
		}
		got := l.all(t)
		if len(got) != 1 || got[0]["msg"] != "upload accepted" {
			t.Errorf("log = %v (the insert error must be dropped, not logged)", got)
		}
	})
	t.Run("insert error and the existence check fails", func(t *testing.T) {
		s, _, r, f, l := fixture(t)
		r.insertErr, r.existsErr = errors.New("db down"), errors.New("db still down")
		_, err := s.Upload(context.Background(), []byte("x"))
		var se *ServerError
		if !errors.As(err, &se) || se.Stage != "store" {
			t.Fatalf("err = %v", err)
		}
		if len(f.removed) != 0 || len(f.dirs) != 1 {
			t.Errorf("the folder must stay for recovery: removed %v, dirs %d", f.removed, len(f.dirs))
		}
		got := l.all(t)
		if len(got) != 2 || got[0]["stage"] != "store" || got[1]["stage"] != "cleanup" ||
			got[0]["error"] != "db down" || got[1]["error"] != "db still down" {
			t.Errorf("log = %v", got)
		}
	})
	t.Run("context cancelled after the parse", func(t *testing.T) {
		s, _, r, f, l := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		s.d.Parser = cancelingParser{cancel: cancel, acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("x")}}
		_, err := s.Upload(ctx, []byte("x"))
		if !errors.Is(err, context.Canceled) || f.creates != 0 || r.inserts != 0 {
			t.Fatalf("err = %v creates %d inserts %d", err, f.creates, r.inserts)
		}
		if len(l.all(t)) != 0 {
			t.Errorf("a cancelled upload logged %v", l.all(t))
		}
	})
}

type cancelingParser struct {
	cancel func()
	acc    ingest.Accepted
}

func (p cancelingParser) Accept(context.Context, []byte) (ingest.Accepted, error) {
	p.cancel()
	return p.acc, nil
}

// TestUploadInsertNotCancelled (D-18): the insert and the existence check
// never see the request's cancellation. The fake repository cancels the
// request first and records the error of the context it was given.
func TestUploadInsertNotCancelled(t *testing.T) {
	s, _, r, f, _ := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	var insertErr, existsErr error
	r.onInsert = func(c context.Context) { cancel(); insertErr = c.Err() }
	r.onExists = func(c context.Context) { existsErr = c.Err() }
	// The insert reports an error after storing the row, so the existence check runs too.
	r.insertErr, r.storeOnInsertErr = errors.New("context canceled"), true
	c, err := s.Upload(ctx, []byte("x"))
	if err != nil || c.PublicID == "" {
		t.Fatalf("Created %+v, err %v", c, err)
	}
	if insertErr != nil || existsErr != nil {
		t.Errorf("context errors seen by the repository: insert %v, exists %v", insertErr, existsErr)
	}
	if ctx.Err() == nil {
		t.Fatal("the request was never cancelled: the test proves nothing")
	}
	if len(f.removed) != 0 || len(f.dirs) != 1 || len(r.rows) != 1 {
		t.Errorf("removed %v, folders %d, rows %d", f.removed, len(f.dirs), len(r.rows))
	}
}

func TestRecover(t *testing.T) {
	s, _, r, f, l := fixture(t)
	f.staging, f.unknown = 2, 3
	f.dirs["a"+strings.Repeat("0", 31)] = nil // orphan
	f.dirs["b"+strings.Repeat("0", 31)] = nil // has a row
	r.rows["b"+strings.Repeat("0", 31)] = Record{}
	r.rows["c"+strings.Repeat("0", 31)] = Record{} // row without a folder
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, keptRow := f.dirs["b"+strings.Repeat("0", 31)]
	_, keptOrphan := f.dirs["a"+strings.Repeat("0", 31)]
	if !keptRow || keptOrphan || len(f.dirs) != 1 || len(f.removed) != 1 {
		t.Errorf("dirs %v removed %v", f.dirs, f.removed)
	}
	if _, ok := r.rows["b"+strings.Repeat("0", 31)]; !ok || len(r.rows) != 1 || len(r.deleted) != 1 || r.deleted[0] != "c"+strings.Repeat("0", 31) {
		t.Errorf("rows %v deleted %v: only the row without a folder goes", r.rows, r.deleted)
	}
	got := l.all(t)
	if len(got) != 1 || got[0]["msg"] != "recovery" || got[0]["level"] != "INFO" || got[0]["staging_removed"] != float64(2) ||
		got[0]["orphans_removed"] != float64(1) || got[0]["rows_removed"] != float64(1) || got[0]["unknown_entries"] != float64(3) {
		t.Errorf("log = %v", got)
	}
}

// TestUploadLogKeys: every line from Upload uses only the keys of the design.
func TestUploadLogKeys(t *testing.T) {
	allowed := map[string]map[string]bool{
		"upload accepted": keys("id", "bytes", "truncated", "blanked_fields", "removed_messages", "duration_ms"),
		"upload failed":   keys("stage", "id", "error"),
		"recovery":        keys("staging_removed", "orphans_removed", "rows_removed", "unknown_entries", "public_removed"),
	}
	run := func(setup func(*Service, *fakeRepo, *fakeFiles)) []map[string]any {
		s, _, r, f, l := fixture(t)
		setup(s, r, f)
		_, _ = s.Upload(context.Background(), []byte("x"))
		_ = s.Recover(context.Background())
		return l.all(t)
	}
	var all []map[string]any
	all = append(all, run(func(*Service, *fakeRepo, *fakeFiles) {})...)
	all = append(all, run(func(_ *Service, r *fakeRepo, _ *fakeFiles) { r.insertErr = errors.New("x") })...)
	all = append(all, run(func(_ *Service, _ *fakeRepo, f *fakeFiles) { f.createErr = errors.New("x") })...)
	all = append(all, run(func(s *Service, _ *fakeRepo, _ *fakeFiles) { s.d.Rand = strings.NewReader("") })...)
	for _, m := range all {
		msg := m["msg"].(string)
		ok := allowed[msg]
		if ok == nil {
			t.Errorf("unexpected message %q", msg)
			continue
		}
		for k := range m {
			if k != "time" && k != "level" && k != "msg" && !ok[k] {
				t.Errorf("%s: key %q is not in the design", msg, k)
			}
		}
	}
	if len(all) < 8 {
		t.Errorf("only %d lines", len(all))
	}
}

func keys(k ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range k {
		m[x] = true
	}
	return m
}
