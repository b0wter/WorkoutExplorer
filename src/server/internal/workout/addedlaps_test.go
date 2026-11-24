package workout

import (
	"reflect"
	"slices"
	"testing"
)

// F-65 AL-3, AL-4: added lap boundaries. Synthetic records only.

type lapRow struct {
	start, end int32
	trigger    string
}

func rows(w Workout) []lapRow {
	var out []lapRow
	for _, l := range w.Laps {
		out = append(out, lapRow{l.Start, l.End, l.Trigger})
	}
	return out
}

func whole(w Workout) (int, int) { return 0, len(w.Samples.Offset) - 1 }

// checkParts: every lap not in w is a part whose totals are the range values
// over its samples (transition rule by start), and the samples are untouched.
func checkParts(t *testing.T, w, out Workout) {
	t.Helper()
	if !reflect.DeepEqual(w.Samples, out.Samples) {
		t.Error("samples changed")
	}
	tl := newSportTimeline(w.Sessions, w.hints)
	for i, l := range out.Laps {
		if slices.ContainsFunc(w.Laps, func(d Lap) bool { return reflect.DeepEqual(d, l) }) {
			continue
		}
		from, to := SampleRange(w.Samples.Offset, l.Start, l.End)
		if to-from < 2 {
			t.Errorf("lap %d has %d samples", i, to-from)
			continue
		}
		var want Totals
		completeTotals(newRangeIndex(&w.Samples), &want, l.Start, l.End, tl)
		if !reflect.DeepEqual(l.Totals, want) {
			t.Errorf("lap %d [%d, %d]: totals %+v, want the range values %+v", i, l.Start, l.End, l.Totals, want)
		}
	}
}

func TestAddLapsSplitsDeviceLap(t *testing.T) {
	w := trimRun() // laps [0, 400] distance, [400, 899] button
	lo, hi := whole(w)
	out := AddLaps(w, []int{300, 100}, lo, hi)
	want := []lapRow{{0, 100, "added"}, {100, 300, "added"}, {300, 400, "distance"}, {400, 899, "button"}}
	if !reflect.DeepEqual(rows(out), want) {
		t.Fatalf("laps %v", rows(out))
	}
	if !reflect.DeepEqual(out.Laps[3], w.Laps[1]) {
		t.Error("the unsplit lap changed")
	}
	if out.Laps[0].Totals.Distance == nil || *out.Laps[0].Totals.Distance != 300 || *out.Laps[1].Totals.ElapsedTime != 200 {
		t.Errorf("part values %+v", out.Laps[0].Totals)
	}
	checkParts(t, w, out)
	// Idempotent: its own boundaries are lap ends now.
	if again := AddLaps(out, []int{100, 300}, lo, hi); !reflect.DeepEqual(again.Laps, out.Laps) {
		t.Error("not idempotent")
	}
	if !reflect.DeepEqual(w.Laps, trimRun().Laps) {
		t.Error("w was modified")
	}
}

func TestAddLapsSkips(t *testing.T) {
	w := trimRun()
	for _, c := range []struct {
		name   string
		at     []int
		lo, hi int
	}{
		{"none", nil, 0, 899},
		{"device lap end", []int{400}, 0, 899},
		{"first and last sample", []int{0, 899}, 0, 899},
		{"window ends", []int{100, 700}, 100, 700},
		{"outside the window", []int{50, 800}, 100, 700},
		{"outside the recording", []int{-1, 900, 5000}, 0, 899},
	} {
		if out := AddLaps(w, c.at, c.lo, c.hi); !reflect.DeepEqual(out.Laps, w.Laps) {
			t.Errorf("%s: laps %v", c.name, rows(out))
		}
	}
	// A repeated index splits once.
	if out := AddLaps(w, []int{200, 200}, 0, 899); len(out.Laps) != 3 {
		t.Errorf("repeat: %v", rows(out))
	}
}

// Without device laps the whole recording is one implicit lap: the first
// boundary gives two laps, the last part's trigger is empty.
func TestAddLapsWithoutDeviceLaps(t *testing.T) {
	w := Normalise(Recording{Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(599)}},
		Records: recs(seq(0, 599), func(i int, r *RawRecord) { r.Distance, r.Speed = float64(i)*5, 5 })})
	if len(w.Laps) != 0 {
		t.Fatal("fixture has laps")
	}
	out := AddLaps(w, []int{250}, 0, 599)
	if want := []lapRow{{0, 250, "added"}, {250, 599, ""}}; !reflect.DeepEqual(rows(out), want) {
		t.Fatalf("laps %v", rows(out))
	}
	checkParts(t, w, out)
	if len(AddLaps(w, []int{0, 599}, 0, 599).Laps) != 0 {
		t.Error("an implicit lap without a split became a lap")
	}
}

// A truncated tail (no device lap covers it) and a gap between laps are
// implicit laps, widened to the touching lap ends.
func TestAddLapsImplicitLaps(t *testing.T) {
	w := Normalise(Recording{
		Laps:    []RawLap{{Start: at(100), End: at(300), Trigger: "time"}, {Start: at(400), End: at(500), Trigger: "time"}},
		Records: recs(seq(0, 799), func(i int, r *RawRecord) { r.Distance = float64(i) * 3 }),
	})
	if ends := LapEnds(w); !reflect.DeepEqual(ends, []int{0, 100, 300, 400, 500, 799}) {
		t.Fatalf("ends %v", ends)
	}
	out := AddLaps(w, []int{50, 350, 650}, 0, 799)
	want := []lapRow{{0, 50, "added"}, {50, 100, ""}, {100, 300, "time"}, {300, 350, "added"}, {350, 400, ""},
		{400, 500, "time"}, {500, 650, "added"}, {650, 799, ""}}
	if !reflect.DeepEqual(rows(out), want) {
		t.Fatalf("laps %v", rows(out))
	}
	checkParts(t, w, out)
}

// A lap ending in a gap, and laps of two samples: no part with fewer than
// two samples, nothing splits a two-sample lap.
func TestAddLapsGapsAndShortLaps(t *testing.T) {
	offs := append(seq(0, 100), seq(400, 500)...) // a 300 s gap (not filled)
	w := Normalise(Recording{
		Laps:    []RawLap{{Start: at(0), End: at(250), Trigger: "button"}, {Start: at(250), End: at(251)}, {Start: at(251), End: at(500)}},
		Records: recs(offs, func(i int, r *RawRecord) { r.HeartRate = 120 }),
	})
	lo, hi := whole(w)
	out := AddLaps(w, []int{50, 101, 102}, lo, hi)
	checkParts(t, w, out)
	if n := len(out.Laps); n != 5 {
		t.Fatalf("laps %v", rows(out))
	}
}

// Overlapping device laps: the later-starting one is split.
func TestAddLapsOverlap(t *testing.T) {
	w := Normalise(Recording{
		Laps:    []RawLap{{Start: at(0), End: at(500), Trigger: "time"}, {Start: at(300), End: at(899), Trigger: "button"}},
		Records: recs(seq(0, 899), nil),
	})
	out := AddLaps(w, []int{400}, 0, 899)
	want := []lapRow{{0, 500, "time"}, {300, 400, "added"}, {400, 899, "button"}}
	if !reflect.DeepEqual(rows(out), want) {
		t.Fatalf("laps %v", rows(out))
	}
}

// Multisport: a boundary in T1 gives parts that start in the transition,
// so they get no speed, cadence or power (R-13).
func TestAddLapsTransition(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{
			{Sport: "running", Start: at(0), End: at(299)},
			{Sport: "transition", Start: at(300), End: at(399)},
			{Sport: "cycling", Start: at(400), End: at(899)},
		},
		Laps: []RawLap{{Start: at(0), End: at(299)}, {Start: at(300), End: at(399)}, {Start: at(400), End: at(899)}},
		Records: recs(seq(0, 899), func(i int, r *RawRecord) {
			r.Distance, r.Speed, r.HeartRate, r.Power = float64(i)*4, 4, 130, 200
		}),
	})
	out := AddLaps(w, []int{350, 600}, 0, 899)
	checkParts(t, w, out)
	if len(out.Laps) != 5 || out.Laps[1].Trigger != "added" || out.Laps[2].Start != 350 {
		t.Fatalf("laps %v", rows(out))
	}
	if out.Laps[2].Totals.AvgSpeed != nil || out.Laps[2].Totals.ElapsedTime == nil || out.Laps[3].Totals.AvgPower == nil {
		t.Errorf("transition rule: %+v / %+v", out.Laps[2].Totals, out.Laps[3].Totals)
	}
}

// AL-4: inside a pool swim with lengths a boundary snaps to the nearest last
// sample of a length (rest included), ties to the earlier; without lengths,
// or outside a pool swim, it stays. Both parts keep the lap's stroke.
func TestSnapLapBoundaryAndStroke(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{poolSession(0, 199, "freestyle"), {Sport: "running", Start: at(200), End: at(299)}},
		Laps:     []RawLap{{Start: at(0), End: at(199), Stroke: "freestyle", Trigger: "button"}, {Start: at(200), End: at(299)}},
		Lengths:  []RawLength{rawLen(0, 30, "freestyle", 10), restLen(30, 60), rawLen(60, 100, "freestyle", 12)},
		Records:  recs(seq(0, 299), nil),
	})
	for _, c := range []struct{ in, want int }{
		{10, 30}, {20, 30}, {40, 30}, {45, 30}, {46, 60}, {70, 60}, {90, 100}, {150, 100}, {250, 250}, {-1, -1},
	} {
		if got := SnapLapBoundary(w, c.in); got != c.want {
			t.Errorf("snap %d = %d, want %d", c.in, got, c.want)
		}
	}
	out := AddLaps(w, []int{SnapLapBoundary(w, 40)}, 0, 299)
	if len(out.Laps) != 3 || out.Laps[0].Stroke != "freestyle" || out.Laps[1].Stroke != "freestyle" || out.Laps[1].Trigger != "button" {
		t.Fatalf("%+v", out.Laps)
	}
	noLengths := Normalise(Recording{Sessions: []RawSession{poolSession(0, 199, "")}, Records: recs(seq(0, 199), nil)})
	if got := SnapLapBoundary(noLengths, 40); got != 40 {
		t.Errorf("no lengths: %d", got)
	}
}
