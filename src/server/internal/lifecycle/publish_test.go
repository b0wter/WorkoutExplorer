package lifecycle

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// F-15: publish, the recheck after a build, the public folder in Delete and
// Recover (architect/F-15-publish-pipeline-and-static-page.md P-1, P-6, P-8).

type fakePublic struct {
	mu      sync.Mutex
	removed []string
	sizes   map[string]int64
	keep    map[string]struct{}
	swept   int
	freed   int64
	err     error
	onRm    func(publicID string)
}

func (p *fakePublic) Remove(publicID string) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.onRm != nil {
		p.onRm(publicID)
	}
	p.removed = append(p.removed, publicID)
	if p.err != nil {
		return 0, p.err
	}
	n := p.sizes[publicID]
	delete(p.sizes, publicID)
	return n, nil
}

func (p *fakePublic) Sweep(keep map[string]struct{}) (int, int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keep = keep
	return p.swept, p.freed, p.err
}

func TestApplyPublish(t *testing.T) {
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	now := created.Add(90*time.Minute + 500*time.Millisecond)
	d := Record{CreatedAt: created, Visibility: Draft, Title: "t"}
	got := ApplyPublish(d, now)
	if got.Visibility != Unlisted || got.PublishedAt == nil || !got.PublishedAt.Equal(now.Truncate(time.Second)) || got.Title != "t" {
		t.Errorf("draft: %+v", got)
	}
	first := *got.PublishedAt
	if again := ApplyPublish(got, now.Add(time.Hour)); again.Visibility != Unlisted || !again.PublishedAt.Equal(first) {
		t.Errorf("again: %+v", again)
	}
	if d.PublishedAt != nil {
		t.Error("input changed")
	}
}

func TestPublish(t *testing.T) {
	hook := &fakeHook{}
	s, _, _, l, c, now := owned(t, hook)
	pub := &fakePublic{}
	s.d.Public = pub

	// Strangers and expired rows: not found, nothing changes.
	for _, tok := range []string{"", strings.Repeat("x", 22), "short"} {
		if _, err := s.Publish(context.Background(), c.PublicID, tok); !errors.Is(err, ErrNotFound) {
			t.Errorf("token %q: %v", tok, err)
		}
	}
	if len(hook.calls) != 0 {
		t.Fatal("hook ran for a stranger")
	}

	got, err := s.Publish(context.Background(), c.PublicID, c.EditToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.Visibility != Unlisted || got.PublishedAt == nil {
		t.Errorf("record %+v", got)
	}
	if len(hook.calls) != 1 || hook.befores[0].Visibility != Draft || hook.calls[0].Visibility != Unlisted {
		t.Errorf("hook %v / %v", hook.befores, hook.calls)
	}
	if len(pub.removed) != 0 {
		t.Errorf("live workout removed: %v", pub.removed)
	}
	// Again: idempotent on the row, the hook rebuilds, no second log line.
	*now = now.Add(time.Hour)
	again, err := s.Publish(context.Background(), c.PublicID, c.EditToken)
	if err != nil || !again.PublishedAt.Equal(*got.PublishedAt) || len(hook.calls) != 2 {
		t.Errorf("again: %+v %v", again, err)
	}
	n := 0
	for _, line := range l.all(t) {
		if line["msg"] == "workout published" {
			n++
			if len(line) != 4 || line["id"] == nil || line["level"] != "INFO" {
				t.Errorf("line %v", line)
			}
		}
	}
	if n != 1 || strings.Contains(l.buf.String(), c.PublicID) || strings.Contains(l.buf.String(), c.EditToken) {
		t.Errorf("%d lines: %s", n, l.buf.String())
	}
	// Expired: not found, even for the owner.
	*now = now.Add(8 * 24 * time.Hour)
	if _, err := s.Publish(context.Background(), c.PublicID, c.EditToken); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired: %v", err)
	}
}

// TestPublishHookFails: the row stays unlisted (row first, ADR-028 item 1),
// the answer is a server error, and its log line has no public_id.
func TestPublishHookFails(t *testing.T) {
	hook := &fakeHook{err: errors.New("disk full")}
	s, r, _, l, c, _ := owned(t, hook)
	s.d.Public = &fakePublic{}
	_, err := s.Publish(context.Background(), c.PublicID, c.EditToken)
	var se *ServerError
	if !errors.As(err, &se) || se.Stage != "hook" {
		t.Fatalf("err %v", err)
	}
	rec, _, _ := r.WorkoutByPublicID(context.Background(), c.PublicID)
	if rec.Visibility != Unlisted {
		t.Errorf("visibility %s", rec.Visibility)
	}
	if strings.Contains(l.buf.String(), c.PublicID) || !strings.Contains(l.buf.String(), `"stage":"hook"`) {
		t.Errorf("log %s", l.buf.String())
	}
}

// TestRecheck (P-6): after the hook, a row that another process deleted or
// that is no longer live and unlisted loses its public folder.
func TestRecheck(t *testing.T) {
	for _, c := range []struct {
		name   string
		mutate func(r *fakeRepo, id string)
		remove bool
	}{
		{"still live", func(*fakeRepo, string) {}, false},
		{"deleted by another process", func(r *fakeRepo, id string) { delete(r.rows, id) }, true},
		{"not unlisted any more", func(r *fakeRepo, id string) { x := r.rows[id]; x.Visibility = Draft; r.rows[id] = x }, true},
		{"expired meanwhile", func(r *fakeRepo, id string) { x := r.rows[id]; x.ExpiresAt = x.CreatedAt; r.rows[id] = x }, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			hook := &fakeHook{}
			s, r, _, _, cr, _ := owned(t, hook)
			hook.check = func(after Record) { c.mutate(r, after.ID) } // runs with r.mu unlocked
			pub := &fakePublic{}
			s.d.Public = pub
			if _, err := s.Publish(context.Background(), cr.PublicID, cr.EditToken); err != nil {
				t.Fatal(err)
			}
			if got := len(pub.removed) == 1 && pub.removed[0] == cr.PublicID; got != c.remove {
				t.Errorf("removed %v", pub.removed)
			}
		})
	}
	// The same check follows a PATCH.
	hook := &fakeHook{}
	s, r, _, _, cr, _ := owned(t, hook)
	pub := &fakePublic{}
	s.d.Public = pub
	if _, err := s.Publish(context.Background(), cr.PublicID, cr.EditToken); err != nil {
		t.Fatal(err)
	}
	hook.check = func(after Record) { delete(r.rows, after.ID) }
	if _, err := s.Update(context.Background(), cr.PublicID, cr.EditToken, SettingsPatch{Title: sp("x")}); err != nil {
		t.Fatal(err)
	}
	if len(pub.removed) != 1 {
		t.Errorf("removed %v", pub.removed)
	}
}

// TestDeletePublicOrder (P-8, ADR-022 item 6): the row, then
// public/{public_id}, then private/{id}; the bytes of both are released once.
func TestDeletePublicOrder(t *testing.T) {
	s, r, f, st := storageFixture(t, 1<<20, 0)
	c, err := s.Upload(context.Background(), []byte("UPLOAD"))
	if err != nil {
		t.Fatal(err)
	}
	id := onlyID(t, r)
	pub := &fakePublic{sizes: map[string]int64{c.PublicID: 300}}
	st.Add(300)
	s.d.Public = pub
	pub.onRm = func(string) {
		if _, ok := r.rows[id]; ok {
			t.Error("public folder removed before the row")
		}
		if _, ok := f.dirs[id]; !ok {
			t.Error("private folder removed before the public one")
		}
	}
	used := st.Used()
	if err := s.Delete(context.Background(), id, c.PublicID); err != nil {
		t.Fatal(err)
	}
	if len(pub.removed) != 1 || pub.removed[0] != c.PublicID || len(f.dirs) != 0 {
		t.Errorf("public %v, private %v", pub.removed, f.dirs)
	}
	if st.Used() != 0 {
		t.Errorf("used %d after delete, %d before", st.Used(), used)
	}
	// A second delete releases nothing.
	pub.onRm = nil
	pub.sizes = map[string]int64{c.PublicID: 50}
	st.Add(50)
	if err := s.Delete(context.Background(), id, c.PublicID); err != nil || st.Used() != 50 {
		t.Errorf("second delete: %v, used %d", err, st.Used())
	}
	// A public failure does not keep the private folder (QA-15-r1-2): it is
	// removed and released, and the error is returned.
	s2, r2, f2, st2 := storageFixture(t, 1<<20, 0)
	c2, _ := s2.Upload(context.Background(), []byte("UPLOAD"))
	rm := errors.New("rm")
	s2.d.Public = &fakePublic{err: rm}
	if err := s2.Delete(context.Background(), onlyID(t, r2), c2.PublicID); !errors.Is(err, rm) || len(f2.dirs) != 0 || st2.Used() != 0 {
		t.Errorf("err %v, dirs %v, used %d", err, f2.dirs, st2.Used())
	}
	// Both failing: both errors are returned.
	s3, r3, f3, _ := storageFixture(t, 1<<20, 0)
	c3, _ := s3.Upload(context.Background(), []byte("UPLOAD"))
	s3.d.Public = &fakePublic{err: rm}
	priv := errors.New("private")
	f3.removeErr = priv
	if err := s3.Delete(context.Background(), onlyID(t, r3), c3.PublicID); !errors.Is(err, rm) || !errors.Is(err, priv) {
		t.Errorf("err %v", err)
	}
}

// TestRecoverPublic (P-8): only live unlisted rows keep a public folder, and
// the recovery line counts what was removed.
func TestRecoverPublic(t *testing.T) {
	s, _, r, f, l := fixture(t)
	pub := &fakePublic{swept: 4}
	s.d.Public = pub
	add := func(n int, v Visibility, exp time.Time) string {
		id := strings.Repeat(string(rune('a'+n)), 32)
		pid := strings.Repeat(string(rune('A'+n)), 22)
		r.rows[id] = Record{ID: id, PublicID: pid, Visibility: v, ExpiresAt: exp}
		f.dirs[id] = nil
		return pid
	}
	live := add(0, Unlisted, t0.Add(time.Hour))
	add(2, Draft, t0.Add(time.Hour))
	add(3, Unlisted, t0) // expired at now
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := pub.keep[live]; !ok || len(pub.keep) != 1 {
		t.Errorf("keep %v", pub.keep)
	}
	got := l.all(t)
	if len(got) != 1 || got[0]["public_removed"] != float64(4) {
		t.Errorf("log %v", got)
	}
	pub.err = errors.New("sweep")
	if err := s.Recover(context.Background()); err == nil {
		t.Error("sweep failure hidden")
	}
}
