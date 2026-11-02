package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"workout-explorer/internal/ingest"
)

// ---- fakes (the owner ports) ----------------------------------------------------

func (r *fakeRepo) WorkoutByPublicID(ctx context.Context, publicID string) (Record, bool, error) {
	if r.lookupErr != nil {
		return Record{}, false, r.lookupErr
	}
	if err := ctx.Err(); err != nil { // as database/sql does
		return Record{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.rows {
		if rec.PublicID == publicID {
			return rec, true, nil
		}
	}
	return Record{}, false, nil
}

func (r *fakeRepo) UpdateWorkout(_ context.Context, id string, apply func(Record) (Record, error)) (Record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.rows[id]
	if !ok {
		return Record{}, false, nil
	}
	next, err := apply(cur)
	if err != nil {
		return Record{}, false, err
	}
	r.rows[id] = next
	return next, true, nil
}

func (f *fakeFiles) Read(id, name string) (io.ReadCloser, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, file := range f.dirs[id] {
		if file.Name == name {
			return io.NopCloser(bytes.NewReader(file.Data)), int64(len(file.Data)), nil
		}
	}
	return nil, 0, fs.ErrNotExist
}

type fakeHook struct {
	mu      sync.Mutex
	calls   []Record // the after records
	befores []Record
	err     error
	check   func(after Record)
}

func (h *fakeHook) WorkoutChanged(_ context.Context, before, after Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, after)
	h.befores = append(h.befores, before)
	if h.check != nil {
		h.check(after)
	}
	return h.err
}

// owned uploads one workout and returns the service with its credentials.
func owned(t *testing.T, hook ChangeHook) (*Service, *fakeRepo, *fakeFiles, *lines, Created, *time.Time) {
	t.Helper()
	p := &fakeParser{acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("STRIPPED")}}
	r, f, l := newRepo(), newFiles(), &lines{}
	now := t0
	s := New(Deps{Parser: p, Repo: r, Files: f, Hook: hook, Logger: slog.New(slog.NewJSONHandler(l, nil)), Now: func() time.Time { return now }})
	c, err := s.Upload(context.Background(), []byte("upload"))
	if err != nil {
		t.Fatal(err)
	}
	return s, r, f, l, c, &now
}

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }
func bp(b bool) *bool     { return &b }

// ---- pure rules ------------------------------------------------------------------

func TestApplyPatch(t *testing.T) {
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	base := Record{CreatedAt: created, ExpiresAt: created.Add(DefaultLifetime), Visibility: Draft, Title: "old", Description: "olddesc"}
	now := created.Add(time.Hour)

	tests := []struct {
		name string
		rec  Record
		p    SettingsPatch
		ok   bool
		chk  func(t *testing.T, r Record)
	}{
		{"empty changes nothing", base, SettingsPatch{}, true, func(t *testing.T, r Record) {
			if !reflect.DeepEqual(r, base) {
				t.Errorf("changed: %+v", r)
			}
		}},
		{"title trimmed", base, SettingsPatch{Title: sp("  \t Morning ride  ")}, true, func(t *testing.T, r Record) {
			if r.Title != "Morning ride" {
				t.Errorf("%q", r.Title)
			}
		}},
		{"title script stays plain text (AC 3)", base, SettingsPatch{Title: sp("<script>alert(1)</script>")}, true, func(t *testing.T, r Record) {
			if r.Title != "<script>alert(1)</script>" {
				t.Errorf("%q", r.Title)
			}
		}},
		{"title 100 code points", base, SettingsPatch{Title: sp(strings.Repeat("ä", 100))}, true, nil},
		{"title 101 code points", base, SettingsPatch{Title: sp(strings.Repeat("ä", 101))}, false, nil},
		{"title 100 emoji (4 bytes each)", base, SettingsPatch{Title: sp(strings.Repeat("🚴", 100))}, true, nil},
		{"title counted after trimming", base, SettingsPatch{Title: sp("  " + strings.Repeat("a", 100) + "  ")}, true, nil},
		{"title with newline inside", base, SettingsPatch{Title: sp("a\nb")}, false, nil},
		{"title with NUL", base, SettingsPatch{Title: sp("a\x00b")}, false, nil},
		{"title empty clears", base, SettingsPatch{Title: sp("   ")}, true, func(t *testing.T, r Record) {
			if r.Title != "" {
				t.Errorf("%q", r.Title)
			}
		}},
		{"description 1000", base, SettingsPatch{Description: sp(strings.Repeat("ö", 1000))}, true, nil},
		{"description 1001", base, SettingsPatch{Description: sp(strings.Repeat("ö", 1001))}, false, nil},
		{"description keeps line feeds and spaces", base, SettingsPatch{Description: sp(" a\n\nb ")}, true, func(t *testing.T, r Record) {
			if r.Description != " a\n\nb " {
				t.Errorf("%q", r.Description)
			}
		}},
		{"description with CR", base, SettingsPatch{Description: sp("a\r\nb")}, false, nil},
		{"description with tab", base, SettingsPatch{Description: sp("a\tb")}, false, nil},
		{"flags", base, SettingsPatch{HideEnds: bp(true), AllowDownload: bp(true)}, true, func(t *testing.T, r Record) {
			if !r.HideEnds || !r.AllowDownload {
				t.Errorf("%+v", r)
			}
		}},
		{"lifetime 30 days after upload (AC 2)", base, SettingsPatch{LifetimeDays: ip(30)}, true, func(t *testing.T, r Record) {
			if want := created.Add(30 * 24 * time.Hour); !r.ExpiresAt.Equal(want) || r.LifetimeDays() != 30 {
				t.Errorf("expires %v", r.ExpiresAt)
			}
		}},
		{"lifetime 31 days (AC 2)", base, SettingsPatch{LifetimeDays: ip(31)}, false, nil},
		{"lifetime 0", base, SettingsPatch{LifetimeDays: ip(0)}, false, nil},
		{"lifetime negative", base, SettingsPatch{LifetimeDays: ip(-1)}, false, nil},
		{"lifetime 1 shortens", base, SettingsPatch{LifetimeDays: ip(1)}, true, nil},
		{"all or nothing: a bad field refuses the good one", base, SettingsPatch{Title: sp("fine"), LifetimeDays: ip(99)}, false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ApplyPatch(tc.rec, tc.p, now)
			var ie *InvalidSettingsError
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				if tc.chk != nil {
					tc.chk(t, got)
				}
				return
			}
			if !errors.As(err, &ie) || ie.Message == "" {
				t.Fatalf("want an InvalidSettingsError, got %v", err)
			}
		})
	}
}

func TestApplyPatchLifetimeInThePast(t *testing.T) {
	created := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	rec := Record{CreatedAt: created, ExpiresAt: created.Add(DefaultLifetime), Visibility: Draft}
	// Two days after upload, "1 day" ends yesterday (AC 2: a time in the past).
	if _, err := ApplyPatch(rec, SettingsPatch{LifetimeDays: ip(1)}, created.Add(48*time.Hour)); err == nil {
		t.Error("lifetime ending in the past accepted")
	}
	// Exactly now is not in the future either.
	if _, err := ApplyPatch(rec, SettingsPatch{LifetimeDays: ip(1)}, created.Add(24*time.Hour)); err == nil {
		t.Error("lifetime ending now accepted")
	}
	if _, err := ApplyPatch(rec, SettingsPatch{LifetimeDays: ip(2)}, created.Add(24*time.Hour)); err != nil {
		t.Error(err)
	}
}

func TestDownloadName(t *testing.T) {
	st := time.Date(2026, 6, 1, 23, 30, 0, 0, time.UTC)
	off := func(s int) *int { return &s }
	tests := []struct {
		name string
		rec  Record
		want string
	}{
		{"no start time (OQ-59)", Record{Title: "Ride"}, "workout.fit"},
		{"unknown zone is UTC", Record{StartTime: &st}, "workout-2026-06-01.fit"},
		{"local date after midnight", Record{StartTime: &st, TimeZoneOffsetS: off(7200)}, "workout-2026-06-02.fit"},
		{"local date before midnight", Record{StartTime: &st, TimeZoneOffsetS: off(-8 * 3600)}, "workout-2026-06-01.fit"},
		{"title never used", Record{StartTime: &st, Title: "ü \"x\"\r\n"}, "workout-2026-06-01.fit"},
	}
	for _, tc := range tests {
		if got := DownloadName(tc.rec); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
		for _, c := range DownloadName(tc.rec) {
			if c >= 0x7f || c < 0x20 || c == '"' {
				t.Errorf("%s: not plain ASCII", tc.name)
			}
		}
	}
}

// ---- the token check ---------------------------------------------------------------

func TestOwnerNotFoundCases(t *testing.T) {
	s, r, _, _, c, now := owned(t, nil)
	other := strings.Repeat("a", 22)
	tests := []struct {
		name, pub, token string
		ok               bool
	}{
		{"right", c.PublicID, c.EditToken, true},
		{"wrong token", c.PublicID, other, false},
		{"no token", c.PublicID, "", false},
		{"malformed token", c.PublicID, c.EditToken + "x", false},
		{"token of 22 bytes with a symbol", c.PublicID, c.EditToken[:21] + "-", false},
		{"unknown public id", other, c.EditToken, false},
		{"malformed public id", "short", c.EditToken, false},
		{"internal id instead of public id", func() string {
			for id := range r.rows {
				return id
			}
			return ""
		}(), c.EditToken, false},
		{"public id as token", c.PublicID, c.PublicID, false},
	}
	for _, tc := range tests {
		_, err := s.Get(context.Background(), tc.pub, tc.token)
		if tc.ok != (err == nil) || (!tc.ok && err != ErrNotFound) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	// Expired to the second, though the row is still there (ADR-025 item 4).
	// Published, so that the draft rule (F-21 X-1, TestDeadline) does not apply.
	for id, rec := range r.rows {
		r.rows[id] = ApplyPublish(rec, t0)
	}
	*now = t0.Add(DefaultLifetime)
	if _, err := s.Get(context.Background(), c.PublicID, c.EditToken); err != ErrNotFound {
		t.Errorf("expired: %v", err)
	}
	*now = t0.Add(DefaultLifetime - time.Second)
	if _, err := s.Get(context.Background(), c.PublicID, c.EditToken); err != nil {
		t.Errorf("one second before expiry: %v", err)
	}
}

func TestOwnerOperationsAllCheckTheToken(t *testing.T) {
	s, r, f, _, c, _ := owned(t, nil)
	ctx := context.Background()
	for _, tok := range []string{"", strings.Repeat("b", 22)} {
		if _, err := s.Update(ctx, c.PublicID, tok, SettingsPatch{Title: sp("x")}); err != ErrNotFound {
			t.Errorf("update: %v", err)
		}
		if _, err := s.OpenDraft(ctx, c.PublicID, tok); err != ErrNotFound {
			t.Errorf("draft: %v", err)
		}
		if _, err := s.OpenOriginal(ctx, c.PublicID, tok); err != ErrNotFound {
			t.Errorf("original: %v", err)
		}
		if err := s.DeleteByOwner(ctx, c.PublicID, tok); err != ErrNotFound {
			t.Errorf("delete: %v", err)
		}
	}
	if len(r.rows) != 1 || len(f.dirs) != 1 || len(f.removed) != 0 {
		t.Error("a refused request changed something")
	}
	for _, rec := range r.rows {
		if rec.Title != "" {
			t.Error("title changed")
		}
	}
}

// ---- update, hook, files, delete ---------------------------------------------------

func TestUpdateHook(t *testing.T) {
	hook := &fakeHook{}
	s, r, _, l, c, _ := owned(t, hook)
	ctx := context.Background()

	// A draft is not a published workout: no hook.
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("a")}); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 0 {
		t.Fatal("hook called for a draft")
	}
	// An invalid change stores nothing and calls nothing.
	var ie *InvalidSettingsError
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp(strings.Repeat("x", 101))}); !errors.As(err, &ie) {
		t.Fatal(err)
	}

	// Publish by hand (F-15 does it later).
	for id, rec := range r.rows {
		pub := t0
		rec.Visibility, rec.PublishedAt = Unlisted, &pub
		r.rows[id] = rec
	}
	// Called also when nothing changes (O-11).
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("after")}); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 2 || hook.befores[1].Title == "after" || hook.calls[1].Title != "after" {
		t.Fatalf("hook calls: %+v", hook.calls)
	}

	// A failing hook: 500, the row stays changed, the log has an id and no secrets.
	hook.err = errors.New("disk on fire")
	_, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("kept")})
	var se *ServerError
	if !errors.As(err, &se) || se.Stage != "hook" {
		t.Fatalf("want a hook ServerError, got %v", err)
	}
	for _, rec := range r.rows {
		if rec.Title != "kept" {
			t.Errorf("title %q", rec.Title)
		}
	}
	out := ""
	for _, ln := range l.all(t) {
		if ln["msg"] == "owner request failed" {
			out += ln["stage"].(string)
		}
	}
	if out != "hook" {
		t.Errorf("log: %q", out)
	}
}

func TestOwnerLogsHaveNoSecrets(t *testing.T) {
	hook := &fakeHook{err: errors.New("boom")}
	s, r, _, l, c, _ := owned(t, hook)
	for id, rec := range r.rows {
		pub := t0
		rec.Visibility, rec.PublishedAt = Unlisted, &pub
		r.rows[id] = rec
	}
	_, _ = s.Update(context.Background(), c.PublicID, c.EditToken, SettingsPatch{Title: sp("Secret title"), Description: sp("Secret desc")})
	_ = s.DeleteByOwner(context.Background(), c.PublicID, c.EditToken)
	_, _ = s.Get(context.Background(), c.PublicID, "x")
	var all strings.Builder
	l.mu.Lock()
	all.Write(l.buf.Bytes())
	l.mu.Unlock()
	for _, secret := range []string{c.PublicID, c.EditToken, "Secret title", "Secret desc"} {
		if strings.Contains(all.String(), secret) {
			t.Errorf("log contains %q", secret)
		}
	}
	if !strings.Contains(all.String(), `"msg":"workout deleted"`) || !strings.Contains(all.String(), `"by":"owner"`) {
		t.Errorf("no delete line: %s", all.String())
	}
}

func TestOpenFiles(t *testing.T) {
	s, _, f, _, c, _ := owned(t, nil)
	ctx := context.Background()
	d, err := s.OpenOriginal(ctx, c.PublicID, c.EditToken)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(d.Body)
	d.Body.Close()
	if string(b) != "STRIPPED" || d.Size != 8 || d.Name != "workout-2026-06-01.fit" {
		t.Errorf("%q %d %q", b, d.Size, d.Name)
	}
	d, err = s.OpenDraft(ctx, c.PublicID, c.EditToken)
	if err != nil {
		t.Fatal(err)
	}
	d.Body.Close()
	if d.Name != "" || d.Size == 0 {
		t.Errorf("draft: %+v", d)
	}

	// A file lost while the row is there is a server error ...
	for id := range f.dirs {
		f.dirs[id] = nil
	}
	var se *ServerError
	if _, err := s.OpenOriginal(ctx, c.PublicID, c.EditToken); !errors.As(err, &se) {
		t.Errorf("lost file: %v", err)
	}
}

func TestOpenFileLosesRaceToDelete(t *testing.T) {
	s, r, f, _, c, _ := owned(t, nil)
	// The delete lands between the token check and the file open.
	fr := &racingFiles{fakeFiles: f, onRead: func() {
		for id := range r.rows {
			delete(r.rows, id)
			delete(f.dirs, id)
		}
	}}
	s.d.Files = fr
	if _, err := s.OpenOriginal(context.Background(), c.PublicID, c.EditToken); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

type racingFiles struct {
	*fakeFiles
	onRead func()
}

func (r *racingFiles) Read(id, name string) (io.ReadCloser, int64, error) {
	r.onRead()
	return r.fakeFiles.Read(id, name)
}

func TestDeleteByOwner(t *testing.T) {
	s, r, f, _, c, _ := owned(t, nil)
	ctx := context.Background()
	if err := s.DeleteByOwner(ctx, c.PublicID, c.EditToken); err != nil {
		t.Fatal(err)
	}
	if len(r.rows) != 0 || len(f.dirs) != 0 {
		t.Error("something is left")
	}
	// Every owner operation, including a second delete, is now not found (AC 5).
	if err := s.DeleteByOwner(ctx, c.PublicID, c.EditToken); err != ErrNotFound {
		t.Errorf("second delete: %v", err)
	}
	if _, err := s.Get(ctx, c.PublicID, c.EditToken); err != ErrNotFound {
		t.Errorf("get: %v", err)
	}
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{}); err != ErrNotFound {
		t.Errorf("update: %v", err)
	}
}

func TestDeleteFailureIsServerError(t *testing.T) {
	s, _, f, _, c, _ := owned(t, nil)
	f.removeErr = errors.New("busy")
	var se *ServerError
	if err := s.DeleteByOwner(context.Background(), c.PublicID, c.EditToken); !errors.As(err, &se) {
		t.Errorf("%v", err)
	}
}

// A delete cannot fall between an update's commit and its hook (O-7): the
// hook never sees a row that is gone.
func TestOwnerWritesAreSerialised(t *testing.T) {
	var r *fakeRepo
	hook := &fakeHook{}
	s, repo, _, _, c, _ := owned(t, hook)
	r = repo
	for id, rec := range r.rows {
		pub := t0
		rec.Visibility, rec.PublishedAt = Unlisted, &pub
		r.rows[id] = rec
	}
	var gone bool
	var mu sync.Mutex
	hook.check = func(Record) {
		time.Sleep(time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if len(r.rows) == 0 || gone {
			t.Error("hook ran for a deleted workout")
		}
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i == 10 {
				time.Sleep(5 * time.Millisecond)
				if err := s.DeleteByOwner(context.Background(), c.PublicID, c.EditToken); err != nil {
					t.Error(err)
				}
				mu.Lock()
				gone = true
				mu.Unlock()
				return
			}
			_, err := s.Update(context.Background(), c.PublicID, c.EditToken, SettingsPatch{HideEnds: bp(i%2 == 0)})
			if err != nil && err != ErrNotFound {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestOwnerDeleteReleasesStorage(t *testing.T) {
	st := NewStorage(1<<30, 0)
	p := &fakeParser{acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("STRIPPED")}}
	s := New(Deps{Parser: p, Repo: newRepo(), Files: newFiles(), Storage: st, Now: func() time.Time { return t0 }})
	c, err := s.Upload(context.Background(), []byte("u"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Used() == 0 {
		t.Fatal("nothing counted")
	}
	if err := s.DeleteByOwner(context.Background(), c.PublicID, c.EditToken); err != nil {
		t.Fatal(err)
	}
	if st.Used() != 0 {
		t.Errorf("used %d", st.Used())
	}
}

// TestOwnerCancelledRequestIsSilent (QA-12-r1-1, ADR-025 item 8): a request
// whose context ended gives the not-found refusal and no log line, with or
// without a token.
func TestOwnerCancelledRequestIsSilent(t *testing.T) {
	s, r, f, l, c, _ := owned(t, nil)
	before := l.buf.Len()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, token := range []string{"", c.EditToken} {
		if _, err := s.Get(ctx, c.PublicID, token); err != ErrNotFound {
			t.Errorf("Get: %v", err)
		}
		if _, err := s.Update(ctx, c.PublicID, token, SettingsPatch{Title: sp("x")}); err != ErrNotFound {
			t.Errorf("Update: %v", err)
		}
		if _, err := s.OpenOriginal(ctx, c.PublicID, token); err != ErrNotFound {
			t.Errorf("OpenOriginal: %v", err)
		}
		if err := s.DeleteByOwner(ctx, c.PublicID, token); err != ErrNotFound {
			t.Errorf("DeleteByOwner: %v", err)
		}
	}
	// The WorkoutExists re-check after a missing file.
	ctx2, cancel2 := context.WithCancel(context.Background())
	s.d.Files = &racingFiles{fakeFiles: f, onRead: func() {
		cancel2()
		for id := range r.rows {
			delete(f.dirs, id) // file lost, row still there
		}
	}}
	r.existsErr = context.Canceled
	if _, err := s.OpenOriginal(ctx2, c.PublicID, c.EditToken); err != ErrNotFound {
		t.Errorf("re-check: %v", err)
	}
	if l.buf.Len() != before {
		t.Errorf("log lines written: %s", l.buf.String()[before:])
	}
	if len(r.rows) != 1 {
		t.Error("a cancelled request must not delete")
	}
}

// TestAnonymousDeleteDoesNotQueue (QA-12-r1-2): a stranger's DELETE is
// refused without waiting for the write lock.
func TestAnonymousDeleteDoesNotQueue(t *testing.T) {
	s, _, _, _, c, _ := owned(t, nil)
	s.mu.Lock()
	defer s.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- s.DeleteByOwner(context.Background(), c.PublicID, "") }()
	select {
	case err := <-done:
		if err != ErrNotFound {
			t.Errorf("got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("anonymous DELETE waits for the write lock")
	}
}

// TestOwnerRealRepositoryErrorIsLogged (QA-12-r2-1, ADR-025 item 8, O-13): a
// repository failure that is not the request ending gives a server error and
// exactly one ERROR line, not a silent not found; so does the WorkoutExists
// re-check. The cancelled context must not hide a non-context error either.
func TestOwnerRealRepositoryErrorIsLogged(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for name, ctx := range map[string]context.Context{"live": context.Background(), "cancelled": cancelled} {
		t.Run("lookup/"+name, func(t *testing.T) {
			s, r, _, l, c, _ := owned(t, nil)
			before := l.buf.Len()
			r.lookupErr = errors.New("database is closed")
			_, err := s.Get(ctx, c.PublicID, c.EditToken)
			var se *ServerError
			if !errors.As(err, &se) || se.Stage != "lookup" || err == ErrNotFound {
				t.Fatalf("err = %v", err)
			}
			if n := strings.Count(l.buf.String()[before:], `"level":"ERROR"`); n != 1 {
				t.Errorf("%d ERROR lines: %s", n, l.buf.String()[before:])
			}
		})
	}
	t.Run("recheck", func(t *testing.T) {
		s, r, f, l, c, _ := owned(t, nil)
		s.d.Files = &racingFiles{fakeFiles: f, onRead: func() {
			for id := range r.rows {
				delete(f.dirs, id) // file lost, row still there
			}
		}}
		r.existsErr = errors.New("database is closed")
		before := l.buf.Len()
		_, err := s.OpenOriginal(context.Background(), c.PublicID, c.EditToken)
		var se *ServerError
		if !errors.As(err, &se) {
			t.Fatalf("err = %v", err)
		}
		if n := strings.Count(l.buf.String()[before:], `"level":"ERROR"`); n != 1 {
			t.Errorf("%d ERROR lines: %s", n, l.buf.String()[before:])
		}
	})
}
