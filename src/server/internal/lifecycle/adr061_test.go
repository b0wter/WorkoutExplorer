package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"workout-explorer/internal/preview"
)

// ADR-061 (CR-02): owner reads and deletes stay off the O-7 lock, one owner
// write per workout, a write that loses to a delete answers not found.

// slowTrimmer is the parse seam: every parse waits for the gate (the
// worst-case decode bound) and counts how many run at once.
type slowTrimmer struct {
	gate     chan struct{}
	entered  atomic.Int64
	running  atomic.Int64
	maxAtOne atomic.Int64
	err      error
}

func (f *slowTrimmer) parse(ctx context.Context) error {
	f.entered.Add(1)
	n := f.running.Add(1)
	defer f.running.Add(-1)
	for {
		m := f.maxAtOne.Load()
		if n <= m || f.maxAtOne.CompareAndSwap(m, n) {
			break
		}
	}
	select {
	case <-f.gate:
	case <-ctx.Done():
	}
	return f.err
}

func (f *slowTrimmer) ResolveTrim(ctx context.Context, _ Record, s, e *time.Time) (*time.Time, *time.Time, error) {
	return s, e, f.parse(ctx)
}
func (f *slowTrimmer) CheckEdits(context.Context, Record, []Pin, []time.Time) ([]PinCheck, []LapCheck, error) {
	return nil, nil, errors.New("not used")
}
func (f *slowTrimmer) TrimmedView(ctx context.Context, _ Record) (OwnerView, error) {
	return OwnerView{Suggested: "v"}, f.parse(ctx)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for " + what)
}

// within fails the test unless f returns before d.
func within(t *testing.T, what string, d time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { f(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s took longer than %v", what, d)
	}
}

func noErrorLines(t *testing.T, l *lines) {
	t.Helper()
	for _, m := range l.all(t) {
		if m["level"] == "ERROR" {
			t.Errorf("ERROR line: %v", m)
		}
	}
}

// second uploads one more workout into s.
func second(t *testing.T, s *Service) Created {
	t.Helper()
	c, err := s.Upload(context.Background(), []byte("again"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func idOf(r *fakeRepo, publicID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rec := range r.rows {
		if rec.PublicID == publicID {
			return id
		}
	}
	return ""
}

func edit(r *fakeRepo, id string, f func(*Record)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.rows[id]
	f(&rec)
	r.rows[id] = rec
}

// TestCR02Probe: with k trimmed GETs and PATCHes of one workout stuck in a
// slow parse, an expiry pass and another workout's owner DELETE finish within
// a bound that does not depend on k.
func TestCR02Probe(t *testing.T) {
	for _, k := range []int{0, 8, 64} {
		s, r, _, l, a, now := owned(t, nil)
		s.d.Public = &fakePublic{}
		tr := &slowTrimmer{gate: make(chan struct{})}
		s.d.Trimmer = tr
		b, due := second(t, s), second(t, s)
		trimAt := t0.Add(time.Minute)
		edit(r, idOf(r, a.PublicID), func(rec *Record) { rec.TrimStartAt = &trimAt })
		dueID := idOf(r, due.PublicID)
		ctx := context.Background()
		var wg sync.WaitGroup
		for range k {
			wg.Add(2)
			go func() {
				defer wg.Done()
				if rec, err := s.Get(ctx, a.PublicID, a.EditToken); err == nil {
					s.View(ctx, rec)
				}
			}()
			go func() {
				defer wg.Done()
				start := t0.Add(2 * time.Minute)
				_, _ = s.Update(ctx, a.PublicID, a.EditToken, SettingsPatch{TrimStart: OptTime{Set: true, At: &start}})
			}()
		}
		if k > 0 {
			waitFor(t, "parses to start", func() bool { return tr.entered.Load() >= 1 })
		}
		edit(r, dueID, func(rec *Record) { rec.ExpiresAt = now.Add(-time.Hour) })
		within(t, "expiry pass", 2*time.Second, func() {
			if err := s.Expire(ctx); err != nil {
				t.Error(err)
			}
		})
		within(t, "owner delete", 2*time.Second, func() {
			if err := s.DeleteByOwner(ctx, b.PublicID, b.EditToken); err != nil {
				t.Error(err)
			}
		})
		if idOf(r, due.PublicID) != "" {
			t.Errorf("k=%d: the due row is still there", k)
		}
		close(tr.gate)
		wg.Wait()
		noErrorLines(t, l)
	}
}

// TestDeletesDoNotTakeO7: owner delete, operator delete, the expiry pass and
// the sweep finish while the O-7 lock is held (mutant: a delete taking it).
func TestDeletesDoNotTakeO7(t *testing.T) {
	s, r, _, _, a, now := owned(t, nil)
	s.d.Public = &fakePublic{}
	b, c, d := second(t, s), second(t, s), second(t, s)
	edit(r, idOf(r, c.PublicID), func(rec *Record) { rec.ExpiresAt = now.Add(-time.Hour) })
	dRec := r.rows[idOf(r, d.PublicID)]
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	within(t, "deletes under a held O-7", 2*time.Second, func() {
		if err := s.DeleteByOwner(ctx, a.PublicID, a.EditToken); err != nil {
			t.Error(err)
		}
		if err := s.Expire(ctx); err != nil { // deletes c, runs the sweep
			t.Error(err)
		}
		if err := s.DeleteByOperator(ctx, dRec); err != nil {
			t.Error(err)
		}
	})
	if idOf(r, b.PublicID) == "" || idOf(r, a.PublicID)+idOf(r, c.PublicID)+idOf(r, d.PublicID) != "" {
		t.Errorf("rows left: %d, want only b", len(r.rows))
	}
}

// TestSecondWriteIsBusy: a second PATCH or publish of a workout with a write
// in flight gets ErrBusy; another workout's write waits for O-7 and then
// proceeds; a stranger neither claims nor sees busy.
func TestSecondWriteIsBusy(t *testing.T) {
	s, _, _, _, a, _ := owned(t, nil)
	tr := &slowTrimmer{gate: make(chan struct{})}
	s.d.Trimmer = tr
	b := second(t, s)
	ctx := context.Background()
	start := t0.Add(time.Minute)
	first := make(chan error, 1)
	go func() {
		_, err := s.Update(ctx, a.PublicID, a.EditToken, SettingsPatch{TrimStart: OptTime{Set: true, At: &start}})
		first <- err
	}()
	waitFor(t, "first write in its parse", func() bool { return tr.entered.Load() == 1 })

	if _, err := s.Update(ctx, a.PublicID, a.EditToken, SettingsPatch{Title: sp("x")}); !errors.Is(err, ErrBusy) {
		t.Errorf("second PATCH: %v", err)
	}
	if _, err := s.Publish(ctx, a.PublicID, a.EditToken); !errors.Is(err, ErrBusy) {
		t.Errorf("publish: %v", err)
	}
	// The token check runs first: a wrong token learns nothing.
	if _, err := s.Update(ctx, a.PublicID, "wrongwrongwrong", SettingsPatch{Title: sp("x")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger: %v", err)
	}
	other := make(chan error, 1)
	go func() {
		_, err := s.Update(ctx, b.PublicID, b.EditToken, SettingsPatch{Title: sp("b")})
		other <- err
	}()
	select {
	case err := <-other:
		t.Fatalf("another workout's write did not wait for O-7: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(tr.gate)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-other; err != nil {
		t.Errorf("other workout: %v", err)
	}
	if _, err := s.Update(ctx, a.PublicID, a.EditToken, SettingsPatch{Title: sp("ok")}); err != nil {
		t.Errorf("claim not released: %v", err)
	}
}

func countClaims(s *Service) (n int) {
	s.claims.Range(func(any, any) bool { n++; return true })
	return
}

// TestClaimReleasedOnErrors: every return path of PATCH and publish frees the claim.
func TestClaimReleasedOnErrors(t *testing.T) {
	hook := &fakeHook{err: errors.New("build failed")}
	s, _, _, _, a, _ := owned(t, hook)
	ctx := context.Background()
	at := t0.Add(time.Minute)
	trim := SettingsPatch{TrimStart: OptTime{Set: true, At: &at}}
	long := SettingsPatch{Title: sp(strings.Repeat("x", 101))}
	tr := &slowTrimmer{gate: make(chan struct{}), err: errors.New("parse failed")}
	close(tr.gate)
	s.d.Trimmer = tr
	steps := map[string]func() error{
		"publish, hook fails": func() error { _, err := s.Publish(ctx, a.PublicID, a.EditToken); return err },
		"invalid patch":       func() error { _, err := s.Update(ctx, a.PublicID, a.EditToken, long); return err },
		"trim parse fails":    func() error { _, err := s.Update(ctx, a.PublicID, a.EditToken, trim); return err },
		"wrong token":         func() error { _, err := s.Publish(ctx, a.PublicID, "wrongwrongwrong"); return err },
	}
	for name, step := range steps {
		for range 2 {
			if err := step(); errors.Is(err, ErrBusy) {
				t.Errorf("%s: claim kept", name)
			}
		}
		if n := countClaims(s); n != 0 {
			t.Errorf("%s: %d claims left", name, n)
		}
	}
}

// TestOwnerReadsDoNotOverlap: view and view never parse at once; a frame GET
// during a view is refused (503, no wait) before it builds; a view during a
// frame GET waits within its request, then falls back to the failure view
// without parsing; one that waits less than the frame lasts gets the slot.
func TestOwnerReadsDoNotOverlap(t *testing.T) {
	s, _, _, _, a, _ := owned(t, nil)
	tr := &slowTrimmer{gate: make(chan struct{})}
	s.d.Trimmer = tr
	fr := &fakeFrames{fr: preview.Frame{Key: mapKey}, ok: true, gate: make(chan struct{}), in: make(chan struct{})}
	s.d.Frames = fr
	at := t0
	rec := Record{ID: "x", TrimEndAt: &at}
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); s.View(ctx, rec) }()
	}
	waitFor(t, "first view parsing", func() bool { return tr.entered.Load() == 1 })
	time.Sleep(50 * time.Millisecond)
	if n := tr.entered.Load(); n != 1 {
		t.Errorf("%d views parse at once, want 1 (the others wait for the slot)", n)
	}
	b := second(t, s)
	within(t, "a write during a view", 2*time.Second, func() { // the view is not under O-7
		if _, err := s.Update(ctx, b.PublicID, b.EditToken, SettingsPatch{Title: sp("b")}); err != nil {
			t.Error(err)
		}
	})
	if _, err := s.PreviewFrame(ctx, a.PublicID, a.EditToken); !errors.Is(err, ErrMapBusy) {
		t.Errorf("frame during a view: %v", err)
	}
	if fr.calls != 0 {
		t.Error("the frame was built while a view held the slot")
	}
	close(tr.gate)
	wg.Wait()
	if m := tr.maxAtOne.Load(); m != 1 {
		t.Errorf("max concurrent view parses %d", m)
	}

	done := make(chan error, 1)
	go func() { _, err := s.PreviewFrame(ctx, a.PublicID, a.EditToken); done <- err }()
	<-fr.in
	before := tr.entered.Load()
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if v := s.View(short, rec); v.Suggested != "" || v.Trimmed != nil || v.Steps != nil {
		t.Errorf("view during a frame build: %+v", v)
	}
	if tr.entered.Load() != before {
		t.Error("the view parsed without the slot")
	}
	res := make(chan OwnerView, 1)
	go func() { res <- s.View(ctx, rec) }()
	time.Sleep(20 * time.Millisecond)
	fr.in = nil
	close(fr.gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if v := <-res; v.Suggested != "v" {
		t.Errorf("waiting view: %+v", v)
	}
}

// ---- races of a delete with a write -------------------------------------------

// raceFixture: a published workout (row unlisted).
func raceFixture(t *testing.T) (*Service, *fakeRepo, *fakeFiles, *fakePublic, *fakeHook, *lines, Created) {
	t.Helper()
	hook := &fakeHook{}
	s, r, f, l, c, _ := owned(t, hook)
	pub := &fakePublic{sizes: map[string]int64{}}
	s.d.Public = pub
	p := t0
	edit(r, idOf(r, c.PublicID), func(rec *Record) { rec.Visibility, rec.PublishedAt = Unlisted, &p })
	return s, r, f, pub, hook, l, c
}

func assertGone(t *testing.T, r *fakeRepo, f *fakeFiles, pub *fakePublic) {
	t.Helper()
	r.mu.Lock()
	rows := len(r.rows)
	r.mu.Unlock()
	f.mu.Lock()
	dirs := len(f.dirs)
	f.mu.Unlock()
	pub.mu.Lock()
	left := len(pub.sizes)
	pub.mu.Unlock()
	if rows != 0 || dirs != 0 || left != 0 {
		t.Errorf("not gone: %d rows, %d private folders, %d public folders", rows, dirs, left)
	}
}

func deleter(t *testing.T, s *Service, c Created) func() {
	return func() {
		if err := s.DeleteByOwner(context.Background(), c.PublicID, c.EditToken); err != nil {
			t.Error(err)
		}
	}
}

type deletingTrimmer struct{ del func() }

func (d *deletingTrimmer) ResolveTrim(_ context.Context, _ Record, s, e *time.Time) (*time.Time, *time.Time, error) {
	d.del()
	return s, e, nil
}
func (d *deletingTrimmer) CheckEdits(context.Context, Record, []Pin, []time.Time) ([]PinCheck, []LapCheck, error) {
	return nil, nil, nil
}
func (d *deletingTrimmer) TrimmedView(context.Context, Record) (OwnerView, error) {
	return OwnerView{}, nil
}

// TestDeleteDuringWrite: a delete at each stage of a PATCH or publish leaves
// row and both folders gone; a write whose transaction comes after the delete
// answers not found; no ERROR line.
func TestDeleteDuringWrite(t *testing.T) {
	patch := func(s *Service, c Created) error {
		_, err := s.Update(context.Background(), c.PublicID, c.EditToken, SettingsPatch{Title: sp("x")})
		return err
	}
	trimPatch := func(s *Service, c Created) error {
		at := t0.Add(time.Minute)
		_, err := s.Update(context.Background(), c.PublicID, c.EditToken, SettingsPatch{TrimStart: OptTime{Set: true, At: &at}})
		return err
	}
	publish := func(s *Service, c Created) error {
		_, err := s.Publish(context.Background(), c.PublicID, c.EditToken)
		return err
	}
	for _, tc := range []struct {
		name  string
		op    func(*Service, Created) error
		stage string
	}{
		{"patch before the transaction", trimPatch, "before-tx"},
		{"patch during the build", patch, "during-build"},
		{"patch after the write phase", patch, "after-write"},
		{"publish during the build", publish, "during-build"},
		{"publish after the write phase", publish, "after-write"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, f, pub, hook, l, c := raceFixture(t)
			del := deleter(t, s, c)
			hook.check = func(after Record) {
				if tc.stage == "during-build" {
					del()
				}
				pub.mu.Lock()
				pub.sizes[after.PublicID] = 10 // the build writes its folder, even after the delete
				pub.mu.Unlock()
				if tc.stage == "after-write" {
					del()
				}
			}
			if tc.stage == "during-build" {
				hook.err = fs.ErrNotExist // privatedir.Replace never creates a missing folder
			}
			if tc.stage == "before-tx" {
				s.d.Trimmer = &deletingTrimmer{del: del}
			}
			err := tc.op(s, c)
			if tc.stage != "after-write" && !errors.Is(err, ErrNotFound) {
				t.Errorf("answer %v, want not found", err)
			}
			if err != nil && !errors.Is(err, ErrNotFound) {
				t.Errorf("answer %v", err)
			}
			assertGone(t, r, f, pub)
			noErrorLines(t, l)
		})
	}
}

type deletingFrames struct {
	*fakeFrames
	del func()
}

func (d *deletingFrames) Frame(ctx context.Context, r Record) (preview.Frame, bool, error) {
	d.del()
	return d.fakeFrames.Frame(ctx, r)
}

// TestDeleteDuringPreviewMapCommit: the commit part of the preview-map PUT
// loses to a delete: answer not found, no ERROR line, both folders gone.
func TestDeleteDuringPreviewMapCommit(t *testing.T) {
	s, r, f, l, c, frames, _ := mapService(t)
	pub := &fakePublic{sizes: map[string]int64{}}
	s.d.Public = pub
	s.d.Frames = &deletingFrames{fakeFrames: frames, del: deleter(t, s, c)}
	err := s.PutPreviewMap(context.Background(), c.PublicID, c.EditToken, mapKey, bytes.NewReader(mapPNG(t)))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("answer %v, want not found", err)
	}
	assertGone(t, r, f, pub)
	noErrorLines(t, l)
}

// TestDeleteStorm: deletes and expiry racing PATCHes and publishes of one
// workout (run with -race): everything is gone at the end, no ERROR line.
func TestDeleteStorm(t *testing.T) {
	for range 20 {
		s, r, f, pub, hook, l, c := raceFixture(t)
		hook.check = func(after Record) {
			pub.mu.Lock()
			pub.sizes[after.PublicID] = 10
			pub.mu.Unlock()
		}
		var wg sync.WaitGroup
		ctx := context.Background()
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch i % 4 {
				case 0:
					_, _ = s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("x")})
				case 1:
					_, _ = s.Publish(ctx, c.PublicID, c.EditToken)
				case 2:
					_ = s.DeleteByOwner(ctx, c.PublicID, c.EditToken)
				case 3:
					_ = s.Expire(ctx)
				}
			}()
		}
		wg.Wait()
		assertGone(t, r, f, pub)
		noErrorLines(t, l)
	}
}

// TestDeleteCallsLoseQuietly: concurrent deletes of one workout; the losers
// answer not found (AC 5) with no ERROR line.
func TestDeleteCallsLoseQuietly(t *testing.T) {
	s, _, _, l, c, _ := owned(t, nil)
	var wg sync.WaitGroup
	var ok, nf atomic.Int64
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := s.DeleteByOwner(context.Background(), c.PublicID, c.EditToken); {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrNotFound):
				nf.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || nf.Load() != 7 {
		t.Errorf("%d deleted, %d not found", ok.Load(), nf.Load())
	}
	noErrorLines(t, l)
}
