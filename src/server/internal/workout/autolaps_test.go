package workout

import (
	"reflect"
	"testing"
)

// F-66 RA-3, RA-5. Synthetic records only.

func lapsWorkout(sessions []RawSession, laps []RawLap, n int) Workout {
	return Normalise(Recording{FileType: "activity", Sessions: sessions, Laps: laps,
		Records: recs(seq(0, n-1), func(i int, r *RawRecord) { r.Distance, r.Speed, r.HeartRate = float64(i)*4, 4, 130 })})
}

func oneSport(laps []RawLap) Workout {
	return lapsWorkout([]RawSession{{Sport: "running", Start: at(0), End: at(599)}}, laps, 600)
}

func TestRemoveAutoLaps(t *testing.T) {
	for _, c := range []struct {
		name string
		laps []RawLap
		want []lapRow // nil: unchanged
	}{
		{"keeps button, stop, equipment", []RawLap{
			{Start: at(0), End: at(100), Trigger: "button"}, {Start: at(100), End: at(200), Trigger: "equipment"},
			{Start: at(200), End: at(599), Trigger: "stop"}}, nil},
		{"merges into the next, trigger of the last part", []RawLap{
			{Start: at(0), End: at(100), Trigger: "distance"}, {Start: at(100), End: at(200), Trigger: "time"},
			{Start: at(200), End: at(300), Trigger: "position"}, {Start: at(300), End: at(400), Trigger: "button"}, {Start: at(400), End: at(599), Trigger: "stop"}},
			[]lapRow{{0, 400, "button"}, {400, 599, "stop"}}},
		{"button lap ends a run and stays", []RawLap{
			{Start: at(0), End: at(100), Trigger: "distance"}, {Start: at(100), End: at(200), Trigger: "button"},
			{Start: at(200), End: at(300), Trigger: ""}, {Start: at(300), End: at(400), Trigger: "distance"},
			{Start: at(400), End: at(599), Trigger: "stop"}},
			[]lapRow{{0, 200, "button"}, {200, 599, "stop"}}},
		{"trailing auto-lap has no next: stays", []RawLap{
			{Start: at(0), End: at(300), Trigger: "button"}, {Start: at(300), End: at(599), Trigger: "distance"}}, nil},
		{"one lap", []RawLap{{Start: at(0), End: at(599), Trigger: "distance"}}, nil},
		{"gap between laps lies inside the merged lap", []RawLap{
			{Start: at(0), End: at(100), Trigger: "distance"}, {Start: at(150), End: at(300), Trigger: "distance"},
			{Start: at(300), End: at(400), Trigger: "button"}, {Start: at(400), End: at(500), Trigger: "stop"}},
			[]lapRow{{0, 400, "button"}, {400, 500, "stop"}}},
		{"truncated tail stays uncovered", []RawLap{
			{Start: at(0), End: at(100), Trigger: "distance"}, {Start: at(100), End: at(200), Trigger: "distance"},
			{Start: at(200), End: at(300), Trigger: "button"}, {Start: at(300), End: at(400), Trigger: "distance"}},
			[]lapRow{{0, 300, "button"}, {300, 400, "distance"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := oneSport(c.laps)
			before := append([]Lap(nil), w.Laps...)
			out := RemoveAutoLaps(w)
			if !reflect.DeepEqual(w.Laps, before) {
				t.Fatal("input modified")
			}
			if c.want == nil {
				if !reflect.DeepEqual(out.Laps, w.Laps) {
					t.Fatalf("changed: %v", rows(out))
				}
				return
			}
			if !reflect.DeepEqual(rows(out), c.want) {
				t.Fatalf("laps %v, want %v", rows(out), c.want)
			}
			checkParts(t, w, out)
		})
	}
}

// OQ-307: everything merged into one lap means no device laps; unmerged laps
// stay byte-equal.
func TestRemoveAutoLapsAllRemovedAndUntouched(t *testing.T) {
	w := oneSport([]RawLap{{Start: at(0), End: at(300), Trigger: "distance"}, {Start: at(300), End: at(599)}})
	if got := RemoveAutoLaps(w); len(got.Laps) != 0 {
		t.Errorf("laps %v", rows(got))
	}
	w = oneSport([]RawLap{{Start: at(0), End: at(100), Trigger: "button"}, {Start: at(100), End: at(200), Trigger: "distance"},
		{Start: at(200), End: at(599), Trigger: "button"}})
	out := RemoveAutoLaps(w)
	if len(out.Laps) != 2 || !reflect.DeepEqual(out.Laps[0], w.Laps[0]) {
		t.Errorf("laps %v", rows(out))
	}
	if !reflect.DeepEqual(RemoveAutoLaps(out).Laps, out.Laps) {
		t.Error("not idempotent")
	}
}

// OQ-314: a boundary at a leg boundary stays, also in a transition.
func TestRemoveAutoLapsKeepsLegBoundaries(t *testing.T) {
	w := lapsWorkout([]RawSession{
		{Sport: "running", Start: at(0), End: at(199)},
		{Sport: "transition", Start: at(200), End: at(249)},
		{Sport: "cycling", Start: at(250), End: at(599)},
	}, []RawLap{
		{Start: at(0), End: at(100), Trigger: "distance"}, {Start: at(100), End: at(199), Trigger: "distance"},
		{Start: at(200), End: at(249), Trigger: "time"},
		{Start: at(250), End: at(400), Trigger: "distance"}, {Start: at(400), End: at(599), Trigger: "button"},
	}, 600)
	out := RemoveAutoLaps(w)
	want := []lapRow{{0, 199, "distance"}, {200, 249, "time"}, {250, 599, "button"}}
	if !reflect.DeepEqual(rows(out), want) {
		t.Fatalf("laps %v", rows(out))
	}
	checkParts(t, w, out)
	if out.Laps[2].Totals.AvgSpeed == nil || out.Laps[1].Totals.AvgSpeed != nil {
		t.Errorf("transition rule: %+v / %+v", out.Laps[1].Totals, out.Laps[2].Totals)
	}
}

// RA-5: the common stroke of the parts, rest laps (no stroke) ignored.
func TestRemoveAutoLapsStroke(t *testing.T) {
	for _, c := range []struct{ name, a, b, c, want string }{
		{"one stroke, rest between", "freestyle", "", "freestyle", "freestyle"},
		{"mixed", "freestyle", "backstroke", "freestyle", "mixed"},
		{"none", "", "", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := oneSport([]RawLap{{Start: at(0), End: at(100), Trigger: "distance"}, {Start: at(100), End: at(200), Trigger: "time"},
				{Start: at(200), End: at(599), Trigger: "button"}})
			w.Laps[0].Stroke, w.Laps[1].Stroke, w.Laps[2].Stroke = c.a, c.b, c.c
			if out := RemoveAutoLaps(w); len(out.Laps) != 0 {
				t.Fatalf("laps %v", rows(out))
			}
			w.Laps = append(w.Laps, Lap{Start: 599, End: 599, Trigger: "button"}) // keeps the merged lap visible
			out := RemoveAutoLaps(w)
			if len(out.Laps) != 2 || out.Laps[0].Stroke != c.want {
				t.Fatalf("laps %+v", out.Laps)
			}
		})
	}
}
