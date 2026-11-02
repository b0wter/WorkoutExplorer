package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// F-21: the deadline rule (X-1), its use in the owner lookup (X-2) and the
// expiry pass (X-4) (architect/F-21-lifetime-and-expiry.md).

// deadlineCases are the X-1 cases; the store's TestDeadlineBoundary pins its
// SQL to the same cases.
func deadlineCases() []struct {
	name     string
	rec      Record
	deadline time.Duration // after created_at
} {
	c := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	pub := c.Add(47 * time.Hour)
	return []struct {
		name     string
		rec      Record
		deadline time.Duration
	}{
		{"unlisted 7 days", Record{Visibility: Unlisted, CreatedAt: c, PublishedAt: &c, ExpiresAt: c.Add(7 * 24 * time.Hour)}, 7 * 24 * time.Hour},
		{"draft 7 days: 48 h", Record{Visibility: Draft, CreatedAt: c, ExpiresAt: c.Add(7 * 24 * time.Hour)}, 48 * time.Hour},
		{"draft 1 day: its lifetime", Record{Visibility: Draft, CreatedAt: c, ExpiresAt: c.Add(24 * time.Hour)}, 24 * time.Hour},
		{"draft 2 days: both", Record{Visibility: Draft, CreatedAt: c, ExpiresAt: c.Add(48 * time.Hour)}, 48 * time.Hour},
		{"published at 47 h, 7 days", Record{Visibility: Unlisted, CreatedAt: c, PublishedAt: &pub, ExpiresAt: c.Add(7 * 24 * time.Hour)}, 7 * 24 * time.Hour},
	}
}

func TestDeadline(t *testing.T) {
	if DraftLifetime != 172800*time.Second {
		t.Fatalf("DraftLifetime %v", DraftLifetime)
	}
	for _, c := range deadlineCases() {
		d := c.rec.CreatedAt.Add(c.deadline)
		if got := c.rec.Deadline(); !got.Equal(d) {
			t.Errorf("%s: deadline %v, want %v", c.name, got, d)
		}
		if !c.rec.Live(d.Add(-time.Second)) || !c.rec.Live(d.Add(-time.Nanosecond)) {
			t.Errorf("%s: not live before the deadline", c.name)
		}
		if c.rec.Live(d) || c.rec.Live(d.Add(time.Second)) {
			t.Errorf("%s: live from the deadline on", c.name)
		}
	}
	// AC 3: published at 47 h it is alive at 49 h.
	c := deadlineCases()[4].rec
	if !c.Live(c.CreatedAt.Add(49 * time.Hour)) {
		t.Error("published at 47 h: gone at 49 h")
	}
}

// TestOverdueDraftNotFound (AC 4, AC 5): a draft past 48 h that no pass has
// deleted is ErrNotFound for every owner operation, one second before it is not.
func TestOverdueDraftNotFound(t *testing.T) {
	s, _, _, _, c, now := owned(t, &fakeHook{})
	s.d.Public = &fakePublic{}
	ctx := context.Background()
	ops := map[string]func() error{
		"get": func() error { _, err := s.Get(ctx, c.PublicID, c.EditToken); return err },
		"patch": func() error {
			_, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("x")})
			return err
		},
		"draft":    func() error { d, err := s.OpenDraft(ctx, c.PublicID, c.EditToken); closeBody(d); return err },
		"original": func() error { d, err := s.OpenOriginal(ctx, c.PublicID, c.EditToken); closeBody(d); return err },
	}
	start := *now
	*now = start.Add(DraftLifetime - time.Second)
	for name, op := range ops {
		if err := op(); err != nil {
			t.Errorf("%s at 48 h - 1 s: %v", name, err)
		}
	}
	*now = start.Add(DraftLifetime)
	ops["publish"] = func() error { _, err := s.Publish(ctx, c.PublicID, c.EditToken); return err }
	ops["delete"] = func() error { return s.DeleteByOwner(ctx, c.PublicID, c.EditToken) }
	for name, op := range ops {
		if err := op(); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s at 48 h: %v", name, err)
		}
	}
}

func closeBody(d Download) {
	if d.Body != nil {
		d.Body.Close()
	}
}

// TestExpire (X-4): due rows go with their three places, live ones stay; one
// failing row does not stop the pass; the sweep runs and its bytes are released.
func TestExpire(t *testing.T) {
	r, f, l := newRepo(), newFiles(), &lines{}
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	st := NewStorage(1<<30, 1000)
	pub := &fakePublic{sizes: map[string]int64{}, freed: 7}
	s := New(Deps{Repo: r, Files: f, Storage: st, Public: pub, Logger: slog.New(slog.NewJSONHandler(l, nil)), Now: func() time.Time { return now }})
	add := func(id string, v Visibility, created, expires time.Duration) {
		c := now.Add(-created)
		rec := Record{ID: id, PublicID: strings.Repeat(id[:1], 22), Visibility: v, CreatedAt: c, ExpiresAt: c.Add(expires)}
		if v == Unlisted {
			rec.PublishedAt = &c
		}
		r.rows[id] = rec
		f.dirs[id] = []File{{OriginalName, make([]byte, 100)}}
		pub.sizes[rec.PublicID] = 10
	}
	add("a-expired", Unlisted, 7*24*time.Hour, 7*24*time.Hour) // at its deadline
	add("b-draft", Draft, 48*time.Hour, 7*24*time.Hour)        // 48 h rule
	add("c-live", Unlisted, 49*time.Hour, 7*24*time.Hour)      // published, alive
	add("d-young", Draft, 48*time.Hour-time.Second, 7*24*time.Hour)
	add("e-short", Draft, 24*time.Hour, 24*time.Hour) // its own lifetime

	if err := s.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	var left []string
	for id := range r.rows {
		left = append(left, id)
	}
	slices.Sort(left)
	if !slices.Equal(left, []string{"c-live", "d-young"}) {
		t.Errorf("rows left %v", left)
	}
	if !slices.Equal(f.removed, []string{"a-expired", "b-draft", "e-short"}) || len(pub.removed) != 3 {
		t.Errorf("private removed %v, public %v", f.removed, pub.removed)
	}
	if _, ok := pub.keep["cccccccccccccccccccccc"]; !ok || len(pub.keep) != 1 {
		t.Errorf("sweep keep %v", pub.keep)
	}
	if got := st.Used(); got != 1000-3*110-7 {
		t.Errorf("total %d", got)
	}
	n := 0
	for _, line := range l.all(t) {
		if line["msg"] == "workout deleted" && line["by"] == "expiry" && line["level"] == "INFO" && len(line) == 5 {
			n++
		}
	}
	if n != 3 || strings.Contains(l.buf.String(), "aaaaaaaaaaaaaaaaaaaaaa") {
		t.Errorf("log: %s", l.buf.String())
	}

	// Nothing due, nothing swept: no log line.
	pub.freed = 0
	l.buf.Reset()
	if err := s.Expire(context.Background()); err != nil || l.buf.Len() != 0 {
		t.Errorf("idle pass: %v %s", err, l.buf.String())
	}

	// A failing public removal: the private folder and the next row still go,
	// and the sweep still runs.
	now = now.Add(7 * 24 * time.Hour)
	pub.err = errors.New("disk")
	pub.keep = nil
	err := s.Expire(context.Background())
	if err == nil || len(r.rows) != 0 || len(f.dirs) != 0 || pub.keep == nil {
		t.Errorf("failing public: %v, rows %v, dirs %v, keep %v", err, r.rows, f.dirs, pub.keep)
	}
	lines := l.all(t)
	var stages []string
	for _, line := range lines {
		if line["msg"] == "expiry failed" {
			stages = append(stages, line["stage"].(string))
			if line["stage"] == "delete" && line["id"] == nil {
				t.Errorf("no id: %v", line)
			}
		}
	}
	if !slices.Equal(stages, []string{"delete", "delete", "sweep"}) {
		t.Errorf("stages %v", stages)
	}
	if strings.Contains(l.buf.String(), "cccccccccccccccccccccc") || strings.Contains(l.buf.String(), "dddddddddddddddddddddd") {
		t.Errorf("public_id in log: %s", l.buf.String())
	}

	// The list itself failing: logged, returned, and the sweep still runs
	// (QA-21-r1-4).
	r.dueErr = errors.New("db")
	pub.err, pub.keep = nil, nil
	l.buf.Reset()
	if err := s.Expire(context.Background()); err == nil || !strings.Contains(l.buf.String(), `"stage":"list"`) || pub.keep == nil {
		t.Errorf("list: %v %s keep %v", err, l.buf.String(), pub.keep)
	}
}

// TestExpireCheckpoint (QA-21-r1-1): a pass that deleted a row checkpoints the
// WAL, an idle pass does not; a failed checkpoint is logged, fails nothing,
// and the next pass retries it. The owner's delete checkpoints too.
func TestExpireCheckpoint(t *testing.T) {
	r, f, l := newRepo(), newFiles(), &lines{}
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	s := New(Deps{Repo: r, Files: f, Logger: slog.New(slog.NewJSONHandler(l, nil)), Now: func() time.Time { return now }})
	due := func(id string) {
		r.rows[id] = Record{ID: id, Visibility: Draft, CreatedAt: now.Add(-49 * time.Hour), ExpiresAt: now.Add(time.Hour)}
	}
	pass := func(wantCheckpoints int) {
		t.Helper()
		if err := s.Expire(context.Background()); err != nil {
			t.Fatal(err)
		}
		if r.checkpoints != wantCheckpoints {
			t.Fatalf("checkpoints %d, want %d", r.checkpoints, wantCheckpoints)
		}
	}
	pass(0)
	due("a")
	pass(1)
	pass(1)
	due("b")
	r.checkpointErr = errors.New("busy")
	pass(2)
	if len(r.rows) != 0 || !strings.Contains(l.buf.String(), `"msg":"checkpoint failed"`) {
		t.Errorf("rows %v log %s", r.rows, l.buf.String())
	}
	pass(3) // retried with nothing due
	r.checkpointErr = nil
	pass(4)
	pass(4)

	s2, r2, _, _, c, _ := owned(t, &fakeHook{})
	r2.checkpointErr = errors.New("busy")
	if err := s2.DeleteByOwner(context.Background(), c.PublicID, c.EditToken); err != nil || r2.checkpoints != 1 {
		t.Errorf("owner delete: %v, checkpoints %d", err, r2.checkpoints)
	}
}

// TestExpireDeleteHoldsLock (X-3, QA-21-r1-3): the pass deletes a due row
// only under the O-7 lock.
func TestExpireDeleteHoldsLock(t *testing.T) {
	r, f := newRepo(), newFiles()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	s := New(Deps{Repo: r, Files: f, Now: func() time.Time { return now }})
	r.rows["a"] = Record{ID: "a", Visibility: Draft, CreatedAt: now.Add(-49 * time.Hour), ExpiresAt: now.Add(time.Hour)}
	s.mu.Lock()
	done := make(chan error, 1)
	go func() { done <- s.Expire(context.Background()) }()
	time.Sleep(100 * time.Millisecond)
	r.mu.Lock()
	n := len(r.deleted)
	r.mu.Unlock()
	s.mu.Unlock()
	if n != 0 {
		t.Error("the pass deleted while an owner write held the lock")
	}
	if err := <-done; err != nil || len(r.rows) != 0 {
		t.Errorf("%v rows %v", err, r.rows)
	}
}

// TestDeadlineBetweenLookupAndCommit (QA-21-r1-2, X-3): a publish or PATCH
// whose lookup ran before the deadline but whose commit runs after it is not
// found, and the row is unchanged.
func TestDeadlineBetweenLookupAndCommit(t *testing.T) {
	ops := map[string]func(*Service, Created) error{
		"publish": func(s *Service, c Created) error {
			_, err := s.Publish(context.Background(), c.PublicID, c.EditToken)
			return err
		},
		"patch": func(s *Service, c Created) error {
			_, err := s.Update(context.Background(), c.PublicID, c.EditToken, SettingsPatch{LifetimeDays: ip(30)})
			return err
		},
	}
	for name, op := range ops {
		_, r, f, l, c, _ := owned(t, &fakeHook{})
		var id string
		for k := range r.rows {
			id = k
		}
		before := r.rows[id]
		now := before.CreatedAt.Add(DraftLifetime - time.Millisecond)
		q := &lateRepo{fakeRepo: r, late: func() { now = now.Add(2 * time.Millisecond) }}
		s := New(Deps{Repo: q, Files: f, Public: &fakePublic{}, Hook: &fakeHook{}, Logger: slog.New(slog.NewJSONHandler(l, nil)), Now: func() time.Time { return now }})
		if err := op(s, c); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v, want not found", name, err)
		}
		if !reflect.DeepEqual(r.rows[id], before) {
			t.Errorf("%s: row changed", name)
		}
		if strings.Contains(l.buf.String(), "owner request failed") { // QA-21-r2-2: not found is no ERROR
			t.Errorf("%s: logged an error: %s", name, l.buf.String())
		}
	}
}

// lateRepo moves the clock past the deadline between the lookup and the
// update's apply.
type lateRepo struct {
	*fakeRepo
	late func()
}

func (q *lateRepo) UpdateWorkout(ctx context.Context, id string, apply func(Record) (Record, error)) (Record, bool, error) {
	q.late()
	return q.fakeRepo.UpdateWorkout(ctx, id, apply)
}

// TestExpireFailingPrivate: a row whose private folder cannot be removed is
// still deleted, the next row too (X-4, X-8).
func TestExpireFailingPrivate(t *testing.T) {
	r, f := newRepo(), newFiles()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	s := New(Deps{Repo: r, Files: f, Now: func() time.Time { return now }})
	for _, id := range []string{"x", "y"} {
		r.rows[id] = Record{ID: id, Visibility: Draft, CreatedAt: now.Add(-49 * time.Hour), ExpiresAt: now.Add(time.Hour)}
	}
	f.removeErr = errors.New("busy")
	if err := s.Expire(context.Background()); err == nil || len(r.rows) != 0 || !slices.Equal(f.removed, []string{"x", "y"}) {
		t.Errorf("%v rows %v removed %v", err, r.rows, f.removed)
	}
}
