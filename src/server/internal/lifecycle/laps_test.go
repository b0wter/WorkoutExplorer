package lifecycle

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"
)

// F-65 AL-2, AL-5 at the service, with a stand-in for workout's rule (the
// real one is tested in workout and publish): the 1,000 s recording of
// pins_test.go with lap ends at 0, 250, 500, 750 and 999.

func lapStandIn(r Record, laps []time.Time) []LapCheck {
	lo, hi := 0, 999
	if r.TrimStartAt != nil {
		lo = int(r.TrimStartAt.Sub(pinBase) / time.Second)
	}
	if r.TrimEndAt != nil {
		hi = int(r.TrimEndAt.Sub(pinBase) / time.Second)
	}
	var out []LapCheck
	for _, t := range laps {
		i := int(t.Sub(pinBase) / time.Second)
		c := LapCheck{Resolved: at(i), Outside: i < 0 || i > 999, Kept: i > lo && i < hi, OnBoundary: i%250 == 0 || i == 999 || i == lo || i == hi}
		out = append(out, c)
	}
	return out
}

func secs(r Record) []int {
	var out []int
	for _, t := range r.AddedLapBoundaries {
		out = append(out, int(t.Sub(pinBase)/time.Second))
	}
	return out
}

func lapSet(is ...int) SettingsPatch {
	ts := make([]time.Time, len(is))
	for k, i := range is {
		ts[k] = at(i)
	}
	return SettingsPatch{AddedLapBoundaries: OptTimes{Set: true, V: ts}}
}

func TestAddedLapsReplaceSortAndRefuse(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	s.d.Trimmer = &gridTrimmer{}
	ctx := context.Background()
	upd := func(p SettingsPatch) (Record, error) { return s.Update(ctx, c.PublicID, c.EditToken, p) }
	rec, err := upd(lapSet(600, 100, 300))
	if err != nil || !slices.Equal(secs(rec), []int{100, 300, 600}) {
		t.Fatalf("%v %v", secs(rec), err)
	}
	stored := func() []int {
		for _, row := range r.rows {
			return secs(row)
		}
		return nil
	}
	// Each refusal: the sentence, all or nothing, the row unchanged.
	for _, c := range []struct {
		p    SettingsPatch
		want string
	}{
		{lapSet(100, -1), MsgLapOutside},
		{lapSet(100, 1000), MsgLapOutside},
		{lapSet(100, 400, 400), MsgLapTwice},
		{lapSet(100, 250), MsgLapOnBoundary},
		{lapSet(100, 0), MsgLapOnBoundary}, // the first sample
		{lapSet(100, 999), MsgLapOnBoundary},
		{lapSet(100, 1500), MsgLapOutside},
	} {
		_, err := upd(c.p)
		refused(t, err, c.want)
		if !slices.Equal(stored(), []int{100, 300, 600}) {
			t.Fatalf("%s: a refused change was stored: %v", c.want, stored())
		}
	}
	// The cap: 100 accepted, 101 refused.
	var many []int
	for i := 1; len(many) < 101; i++ {
		if i%250 != 0 {
			many = append(many, i)
		}
	}
	_, err = upd(lapSet(many...))
	refused(t, err, MsgBadLaps)
	if rec, err = upd(lapSet(many[:100]...)); err != nil || len(rec.AddedLapBoundaries) != 100 {
		t.Fatalf("%d %v", len(rec.AddedLapBoundaries), err)
	}
	// [] clears: NULL.
	if rec, err = upd(lapSet()); err != nil || rec.AddedLapBoundaries != nil {
		t.Fatalf("%v %v", rec.AddedLapBoundaries, err)
	}
}

// AL-5: a trim change deletes the entries not strictly inside the new window,
// sent or not; undoing it restores none; adding one there is refused; a
// radius change keeps all and does not parse; hidden parts are allowed.
func TestAddedLapsTrimAndRadius(t *testing.T) {
	s, _, _, _, c, _ := owned(t, nil)
	g := &gridTrimmer{}
	s.d.Trimmer = g
	ctx := context.Background()
	upd := func(p SettingsPatch) (Record, error) { return s.Update(ctx, c.PublicID, c.EditToken, p) }
	if _, err := upd(lapSet(5, 100, 600, 990)); err != nil { // 5 and 990 lie in any hidden part
		t.Fatal(err)
	}
	calls := g.pinCalls
	rec, err := upd(SettingsPatch{HideStartM: ip(2000), HideEndM: ip(300)})
	if err != nil || g.pinCalls != calls || !slices.Equal(secs(rec), []int{5, 100, 600, 990}) {
		t.Fatalf("radius: %v %v %d", secs(rec), err, g.pinCalls-calls)
	}
	// Trim from 99 (snapped to 100 by the stand-in) to 900: 5 and 990 go, and
	// 100 is now the window's first sample, so it goes too.
	a, b := at(99), at(900)
	if rec, err = upd(SettingsPatch{TrimStart: OptTime{Set: true, At: &a}, TrimEnd: OptTime{Set: true, At: &b}}); err != nil || !slices.Equal(secs(rec), []int{600}) {
		t.Fatalf("trim: %v %v", secs(rec), err)
	}
	// Sent with the trim: stored entries outside go too, new ones are refused.
	_, err = upd(lapSet(600, 950))
	refused(t, err, MsgLapTrimmed)
	if rec, err = upd(SettingsPatch{TrimStart: OptTime{Set: true}, TrimEnd: OptTime{Set: true}}); err != nil || !slices.Equal(secs(rec), []int{600}) {
		t.Fatalf("undo: %v %v", secs(rec), err)
	}
	c2, b2 := lapSet(600, 700), at(650)
	c2.TrimEnd = OptTime{Set: true, At: &b2}
	if rec, err = upd(c2); err == nil {
		t.Fatalf("700 after a trim to 650 was accepted: %v", secs(rec))
	}
	refused(t, err, MsgLapTrimmed)
	// A title change neither parses nor touches them.
	calls = g.pinCalls
	if rec, err = upd(SettingsPatch{Title: sp("x")}); err != nil || g.pinCalls != calls || !slices.Equal(secs(rec), []int{600}) {
		t.Fatalf("title: %v %v", secs(rec), err)
	}
}

// AL-3's last rule: a stored entry that is no longer valid (a parser change)
// is dropped by the next judging when not sent; sent, it is refused.
func TestAddedLapsStoredInvalidDropped(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	s.d.Trimmer = &gridTrimmer{}
	ctx := context.Background()
	for id, row := range r.rows {
		row.AddedLapBoundaries = []time.Time{at(250), at(300)} // 250 became a device lap end
		r.rows[id] = row
	}
	_, err := s.Update(ctx, c.PublicID, c.EditToken, lapSet(250, 300))
	refused(t, err, MsgLapOnBoundary)
	a := at(10)
	rec, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{TrimStart: OptTime{Set: true, At: &a}})
	if err != nil || !slices.Equal(secs(rec), []int{300}) {
		t.Fatalf("%v %v", secs(rec), err)
	}
}

func TestLapBoundaryEncoding(t *testing.T) {
	in := []time.Time{at(3), at(9)}
	s := EncodeLapBoundaries(in)
	if s != "["+itoa(at(3).Unix())+","+itoa(at(9).Unix())+"]" {
		t.Fatal(s)
	}
	out, err := DecodeLapBoundaries(s)
	if err != nil || !slices.EqualFunc(out, in, time.Time.Equal) {
		t.Fatalf("%v %v", out, err)
	}
	if EncodeLapBoundaries(nil) != "" {
		t.Error("none is not empty")
	}
	if _, err := DecodeLapBoundaries(`[1.5]`); err == nil {
		t.Error("a fraction was read")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
