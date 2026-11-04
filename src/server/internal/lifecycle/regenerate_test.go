package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
)

// ---- fakes (the F-30 ports) ----------------------------------------------------

func (r *fakeRepo) UpdateDerived(_ context.Context, id string, d Record, now time.Time) (Record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.derivedErr != nil {
		return Record{}, false, r.derivedErr
	}
	cur, ok := r.rows[id]
	if !ok || !cur.Live(now) {
		return Record{}, false, nil
	}
	cur.Sport, cur.StartTime, cur.TimeZoneOffsetS, cur.SummaryJSON, cur.DataEndS, cur.HiddenRanges =
		d.Sport, d.StartTime, d.TimeZoneOffsetS, d.SummaryJSON, d.DataEndS, d.HiddenRanges
	r.rows[id] = cur
	return cur, true, nil
}

func (f *fakeFiles) Replace(id, name string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return f.replaceErr
	}
	files, ok := f.dirs[id]
	if !ok {
		return errors.New("no folder") // never created
	}
	for i := range files {
		if files[i].Name == name {
			files[i].Data = data
			f.replaced = append(f.replaced, name)
			return nil
		}
	}
	return errors.New("no file")
}

// regenFixture: one stored workout (ID "w1") and a parser that accepts it.
func regenFixture(t *testing.T, vis Visibility) (*Service, *fakeParser, *fakeRepo, *fakeFiles, *fakeHook, *fakePublic, *lines, Record) {
	t.Helper()
	p := &fakeParser{acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("ORIGINAL")}}
	r, f, l, h, pub := newRepo(), newFiles(), &lines{}, &fakeHook{}, &fakePublic{}
	rec := Record{ID: "w1", PublicID: "P1", Title: "kept title", Sport: "old", SummaryJSON: "old summary", Visibility: vis,
		HideEnds: true, CreatedAt: t0.Add(-time.Hour), ExpiresAt: t0.Add(24 * time.Hour)}
	r.rows["w1"] = rec
	f.dirs["w1"] = []File{{OriginalName, []byte("ORIGINAL")}, {DraftName, []byte("old draft")}}
	s := New(Deps{Parser: p, Repo: r, Files: f, Hook: h, Public: pub, Logger: slog.New(slog.NewJSONHandler(l, nil)), Now: func() time.Time { return t0 }})
	return s, p, r, f, h, pub, l, rec
}

// TestRejectedByFile is the F-09 hand-off table (v1.9), RG-3.
func TestRejectedByFile(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{&ingest.Error{Kind: ingest.KindNotFIT}, true},
		{&ingest.Error{Kind: ingest.KindUnsupportedType}, true},
		{&ingest.Error{Kind: ingest.KindNoData}, true},
		{&ingest.Error{Kind: ingest.KindUnreadable}, true},
		{&ingest.Error{Kind: ingest.KindInternal}, true},
		{&ingest.Error{Kind: ingest.KindLimit, Limit: "records"}, true},
		{&ingest.Error{Kind: ingest.KindLimit, Limit: "bytes"}, true},
		{&ingest.Error{Kind: ingest.KindLimit, Limit: "time"}, false},
		{&ingest.Error{Kind: ingest.KindCanceled}, false},
		{errors.New("disk"), false},
		{context.Canceled, false},
	} {
		if got := RejectedByFile(c.err); got != c.want {
			t.Errorf("%v: %v", c.err, got)
		}
	}
}

// TestRegenerateDeletesRejected (AC 4): the file's rejection deletes through
// Delete: row, public folder and private folder, logged by=regenerate.
func TestRegenerateDeletesRejected(t *testing.T) {
	s, p, r, f, h, pub, l, rec := regenFixture(t, Unlisted)
	p.err = &ingest.Error{Kind: ingest.KindUnsupportedType}
	o, err := s.Regenerate(context.Background(), rec.PublicID, p)
	if o != Deleted || err != nil {
		t.Fatalf("%s %v", o, err)
	}
	if len(r.rows) != 0 || !reflect.DeepEqual(pub.removed, []string{"P1"}) || !reflect.DeepEqual(f.removed, []string{"w1"}) || len(h.calls) != 0 {
		t.Errorf("rows %v public %v private %v hook %d", r.rows, pub.removed, f.removed, len(h.calls))
	}
	got := l.all(t)
	if len(got) != 1 || got[0]["msg"] != "workout deleted" || got[0]["by"] != "regenerate" || got[0]["id"] != "w1" {
		t.Errorf("log %v", got)
	}
	s.Checkpoint(context.Background())
	if r.checkpoints != 1 {
		t.Errorf("checkpoints %d", r.checkpoints)
	}
}

// cancelParser stops the run while its parse is in progress.
type cancelParser struct {
	*fakeParser
	cancel func()
}

func (c cancelParser) Accept(ctx context.Context, b []byte) (ingest.Accepted, error) {
	c.cancel()
	return c.fakeParser.Accept(ctx, b)
}

// TestRegenerateStoppedNeverDeletes (QA-30-r1-2): a parse stopped by the
// run's cancellation keeps the workout even when it reads as a rejection.
func TestRegenerateStoppedNeverDeletes(t *testing.T) {
	s, p, r, f, _, pub, _, rec := regenFixture(t, Unlisted)
	p.err = &ingest.Error{Kind: ingest.KindInternal}
	ctx, cancel := context.WithCancel(context.Background())
	if o, err := s.Regenerate(ctx, rec.PublicID, cancelParser{p, cancel}); o != Kept || err == nil {
		t.Fatalf("%s %v", o, err)
	}
	if len(r.deleted) != 0 || len(f.removed) != 0 || len(pub.removed) != 0 {
		t.Errorf("deleted something: %v %v %v", r.deleted, f.removed, pub.removed)
	}
}

// TestRegenerateDeleteLeftover (QA-30-r1-3): a delete that fails after the
// row is gone counts as deleted, with the error and the O-10 line; when the
// row may still be there, the workout is kept.
func TestRegenerateDeleteLeftover(t *testing.T) {
	s, p, r, f, _, pub, l, rec := regenFixture(t, Unlisted)
	p.err = &ingest.Error{Kind: ingest.KindNotFIT}
	pub.err = errors.New("permission denied")
	o, err := s.Regenerate(context.Background(), rec.PublicID, p)
	if o != Deleted || !errors.Is(err, pub.err) || len(r.rows) != 0 || !reflect.DeepEqual(f.removed, []string{"w1"}) {
		t.Fatalf("%s %v rows %v private %v", o, err, r.rows, f.removed)
	}
	if got := l.all(t); len(got) != 1 || got[0]["msg"] != "workout deleted" || got[0]["by"] != "regenerate" {
		t.Errorf("log %v", got)
	}

	s, p, r, _, _, pub, _, rec = regenFixture(t, Unlisted)
	p.err = &ingest.Error{Kind: ingest.KindNotFIT}
	pub.err = errors.New("permission denied")
	pub.onRm = func(string) { r.lookupErr = errors.New("database") } // the re-read cannot tell
	if o, err := s.Regenerate(context.Background(), rec.PublicID, p); o != Kept || err == nil {
		t.Errorf("%s %v", o, err)
	}
}

// TestRegenerateKeeps (AC 6, RG-3, RG-4): canceled, a time limit, a server
// error before or at any commit step: counted kept, never deleted; a failure
// at (a) leaves row and files unchanged; at (b) and (c) the private side is
// new and the next run redoes it.
func TestRegenerateKeeps(t *testing.T) {
	for _, c := range []struct {
		name       string
		set        func(*fakeParser, *fakeRepo, *fakeFiles, *fakeHook)
		filesNew   bool
		rowNew     bool
		hookCalled bool
		accept     bool
	}{
		{"canceled", func(p *fakeParser, _ *fakeRepo, _ *fakeFiles, _ *fakeHook) {
			p.err = &ingest.Error{Kind: ingest.KindCanceled}
		}, false, false, false, false},
		{"time limit", func(p *fakeParser, _ *fakeRepo, _ *fakeFiles, _ *fakeHook) {
			p.err = &ingest.Error{Kind: ingest.KindLimit, Limit: "time"}
		}, false, false, false, false},
		{"not an ingest error", func(p *fakeParser, _ *fakeRepo, _ *fakeFiles, _ *fakeHook) {
			p.err = errors.New("slot")
		}, false, false, false, false},
		{"(a) private write", func(_ *fakeParser, _ *fakeRepo, f *fakeFiles, _ *fakeHook) {
			f.replaceErr = errors.New("disk full")
		}, false, false, false, true},
		{"(b) row", func(_ *fakeParser, r *fakeRepo, _ *fakeFiles, _ *fakeHook) {
			r.derivedErr = errors.New("database")
		}, true, false, false, true},
		{"(c) public build", func(_ *fakeParser, _ *fakeRepo, _ *fakeFiles, h *fakeHook) {
			h.err = errors.New("build")
		}, true, true, true, true},
		{"(c) public build rejected by its parse", func(_ *fakeParser, _ *fakeRepo, _ *fakeFiles, h *fakeHook) {
			h.err = &ingest.Error{Kind: ingest.KindNoData} // only Accept's error decides a delete (QA-30-r1-5)
		}, true, true, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, p, r, f, h, pub, _, rec := regenFixture(t, Unlisted)
			c.set(p, r, f, h)
			o, err := s.Regenerate(context.Background(), rec.PublicID, p)
			if o != Kept || err == nil {
				t.Fatalf("%s %v", o, err)
			}
			if len(r.deleted) != 0 || len(f.removed) != 0 || len(pub.removed) != 0 {
				t.Errorf("deleted something: %v %v %v", r.deleted, f.removed, pub.removed)
			}
			if got := len(f.replaced) > 0; got != c.filesNew {
				t.Errorf("files replaced %v", f.replaced)
			}
			if got := r.rows["w1"].Sport != "old"; got != c.rowNew {
				t.Errorf("row %+v", r.rows["w1"])
			}
			if r.rows["w1"].Title != "kept title" || !r.rows["w1"].HideEnds {
				t.Errorf("settings changed: %+v", r.rows["w1"])
			}
			if got := len(h.calls) > 0; got != c.hookCalled {
				t.Errorf("hook %d", len(h.calls))
			}
		})
	}
}

// TestRegenerateRebuilds (RG-4, RG-6): the draft file, the derived columns
// only, then the public build with before = the row read and after = the row
// written; the original only when Accept changed it.
func TestRegenerateRebuilds(t *testing.T) {
	s, p, r, f, h, pub, _, rec := regenFixture(t, Unlisted)
	o, err := s.Regenerate(context.Background(), rec.PublicID, p)
	if o != Rebuilt || err != nil {
		t.Fatalf("%s %v", o, err)
	}
	if !reflect.DeepEqual(f.replaced, []string{DraftName}) {
		t.Errorf("replaced %v (the original is unchanged)", f.replaced)
	}
	bin, _ := datafile.Encode(smallWorkout())
	if string(f.dirs["w1"][1].Data) != string(bin) {
		t.Error("draft.bin not rewritten")
	}
	got := r.rows["w1"]
	want := rec
	want.Sport, want.SummaryJSON = "cycling", got.SummaryJSON
	want.StartTime, want.TimeZoneOffsetS, want.HiddenRanges = got.StartTime, got.TimeZoneOffsetS, got.HiddenRanges
	if !reflect.DeepEqual(got, want) || got.SummaryJSON == "old summary" || got.StartTime == nil || *got.TimeZoneOffsetS != 7200 {
		t.Errorf("row\n%+v\nwant\n%+v", got, want)
	}
	if len(h.calls) != 1 || !reflect.DeepEqual(h.befores[0], rec) || !reflect.DeepEqual(h.calls[0], got) || len(pub.removed) != 0 {
		t.Errorf("hook %v %v, removed %v", h.befores, h.calls, pub.removed)
	}

	// A changed original is written first; a draft gets no public build.
	s, p, r, f, h, _, _, rec = regenFixture(t, Draft)
	p.acc.Original = []byte("STRIPPED MORE")
	if o, err := s.Regenerate(context.Background(), rec.PublicID, p); o != Rebuilt || err != nil {
		t.Fatalf("%s %v", o, err)
	}
	if !reflect.DeepEqual(f.replaced, []string{OriginalName, DraftName}) || string(f.dirs["w1"][0].Data) != "STRIPPED MORE" || len(h.calls) != 0 {
		t.Errorf("replaced %v hook %d", f.replaced, len(h.calls))
	}
}

// TestRegenerateCommitsAfterCancel (RG-7): a stop after Accept succeeded
// still completes the workout's commit steps.
func TestRegenerateCommitsAfterCancel(t *testing.T) {
	s, p, r, _, h, _, _, rec := regenFixture(t, Unlisted)
	ctx, cancel := context.WithCancel(context.Background())
	p2 := &cancelAfterAccept{p, cancel}
	if o, err := s.Regenerate(ctx, rec.PublicID, p2); o != Rebuilt || err != nil {
		t.Fatalf("%s %v", o, err)
	}
	if r.rows["w1"].Sport != "cycling" || len(h.calls) != 1 {
		t.Errorf("row %+v hook %d", r.rows["w1"], len(h.calls))
	}
}

type cancelAfterAccept struct {
	p      *fakeParser
	cancel func()
}

func (c *cancelAfterAccept) Accept(ctx context.Context, b []byte) (ingest.Accepted, error) {
	defer c.cancel()
	return c.p.Accept(ctx, b)
}

// TestRegenerateSkips (RG-2): a row gone or due when it is read again, or
// gone before (b), is skipped; nothing is deleted.
func TestRegenerateSkips(t *testing.T) {
	s, p, r, f, h, _, _, rec := regenFixture(t, Draft)
	cur := r.rows["w1"]
	cur.CreatedAt = t0.Add(-DraftLifetime) // a draft due at t0
	r.rows["w1"] = cur
	if o, err := s.Regenerate(context.Background(), rec.PublicID, p); o != Skipped || err != nil || p.got != nil {
		t.Errorf("due: %s %v", o, err)
	}
	delete(r.rows, "w1")
	if o, err := s.Regenerate(context.Background(), rec.PublicID, p); o != Skipped || err != nil {
		t.Errorf("gone: %s %v", o, err)
	}
	if len(f.removed) != 0 || len(h.calls) != 0 {
		t.Error("touched")
	}
}

// TestRegenerateRemovesWhenGoneAfterBuild (P-6): serve deleted the row while
// the public build ran; the folder is removed again.
func TestRegenerateRemovesWhenGoneAfterBuild(t *testing.T) {
	s, p, r, _, h, pub, _, rec := regenFixture(t, Unlisted)
	h.check = func(Record) { r.mu.Lock(); delete(r.rows, "w1"); r.mu.Unlock() }
	if o, err := s.Regenerate(context.Background(), rec.PublicID, p); o != Rebuilt || err != nil {
		t.Fatalf("%s %v", o, err)
	}
	if !reflect.DeepEqual(pub.removed, []string{"P1"}) {
		t.Errorf("removed %v", pub.removed)
	}
}
