package workout

import (
	"math"
	"testing"
)

// F-55 per-end radii (HR-2, HR-3, ADR-056). Synthetic tracks only (ADR-004).

var radii = []int{0, 50, 300, 2000}

// checkCutAt checks Hide at (a, b) against the oracle: visibility per sample
// and the fully hidden flag.
func checkCutAt(t *testing.T, name string, w Workout, records []RawRecord, a, b int) {
	t.Helper()
	got, fully := Hide(w, a, b)
	keep, wantFully := expectCutAt(w, records, float64(a), float64(b))
	if a == 0 && b == 0 || w.Samples.Lat == nil {
		wantFully = false
	}
	if fully != wantFully {
		t.Fatalf("%s (%d, %d): fully %v, want %v", name, a, b, fully, wantFully)
	}
	if fully || w.Samples.Lat == nil {
		if got.Samples.Lat != nil && w.Samples.Lat == nil {
			t.Fatalf("%s: positions appeared", name)
		}
		return
	}
	for i := range keep {
		if a == 0 && b == 0 {
			keep[i] = !math.IsNaN(w.Samples.Lat[i])
		}
		if v := !math.IsNaN(got.Samples.Lat[i]); v != keep[i] {
			t.Fatalf("%s (%d, %d): sample %d visible %v, want %v", name, a, b, i, v, keep[i])
		}
	}
}

// checkSteps: every one of the 41 entries per end equals the oracle's boundary
// at that radius, and the steps are monotone (HR-3 verification).
func checkSteps(t *testing.T, name string, w Workout, records []RawRecord) {
	t.Helper()
	h := StepsOf(w)
	if w.Samples.Lat == nil {
		if h != nil {
			t.Fatalf("%s: steps without positions", name)
		}
		return
	}
	n := len(w.Samples.Offset)
	if len(h.Start) != HideStepCount || len(h.End) != HideStepCount || h.Start[0] != 0 || h.End[0] != n-1 {
		t.Fatalf("%s: steps %v", name, h)
	}
	for k := 1; k < HideStepCount; k++ {
		r := float64(k * HideStepM)
		// The oracle with one end cut: the first (last) sample it keeps.
		bound := func(startM, endM float64, fromEnd bool) int {
			keep, fully := expectCutAt(w, records, startM, endM)
			if fully {
				return -1
			}
			for j := range n {
				i := j
				if fromEnd {
					i = n - 1 - j
				}
				if keep[i] {
					return i
				}
			}
			return -1
		}
		if want := bound(r, 0, false); h.Start[k] != want {
			t.Fatalf("%s: start step %d = %d, want %d", name, k, h.Start[k], want)
		}
		if want := bound(0, r, true); h.End[k] != want {
			t.Fatalf("%s: end step %d = %d, want %d", name, k, h.End[k], want)
		}
		if p := h.Start[k-1]; h.Start[k] >= 0 && h.Start[k] < p || p < 0 && h.Start[k] >= 0 {
			t.Fatalf("%s: start steps not monotone at %d", name, k)
		}
		if p := h.End[k-1]; h.End[k] > p || p < 0 && h.End[k] >= 0 {
			t.Fatalf("%s: end steps not monotone at %d", name, k)
		}
	}
}

func TestHideRadiiProperty(t *testing.T) {
	for seed := range uint64(300) {
		records, starts := genTrack(seed)
		w := Normalise(Recording{FileType: "activity", Records: records, SequenceStarts: starts})
		checkSteps(t, "seed", w, records)
		for _, a := range radii {
			for _, b := range radii {
				checkCutAt(t, "seed", w, records, a, b)
			}
		}
	}
}

// TestHideRadiiLine: a 2793 m line east at 7 m/s. Each end uses its own
// radius (mutant: swapped), radius 0 leaves its end untouched (mutant: a 0 m
// circle hides the first fix).
func TestHideRadiiLine(t *testing.T) {
	records := line(400, 7)
	w := normalised(records)
	for _, c := range []struct {
		a, b     int
		from, to int32
	}{
		{0, 0, 0, 399},
		{0, 500, 0, 327}, // 327·7 = 2289 m is 504 m from the last fix (2793 m)
		{50, 0, 8, 399},  // 8·7 = 56 m
		{300, 300, 43, 356},
		{50, 2000, 8, 113}, // 113·7 = 791 m is 2002 m from the last fix
	} {
		got, fully := Hide(w, c.a, c.b)
		checkCutAt(t, "line", w, records, c.a, c.b)
		if fully {
			t.Fatalf("(%d, %d) fully hidden", c.a, c.b)
		}
		if a, b := visibleRange(t, got); a != c.from || b != c.to {
			t.Errorf("(%d, %d): visible %d..%d, want %d..%d", c.a, c.b, a, b, c.from, c.to)
		}
	}
}

// TestHideRadiiFully: a 500 m walk is fully hidden at (0, 2000) and
// (2000, 0), not at (0, 0) or (50, 50); an indoor workout never is.
func TestHideRadiiFully(t *testing.T) {
	w := normalised(line(101, 5))
	for _, c := range []struct {
		a, b  int
		fully bool
	}{{0, 0, false}, {50, 50, false}, {0, 2000, true}, {2000, 0, true}, {300, 300, true}} {
		if _, f := Hide(w, c.a, c.b); f != c.fully {
			t.Errorf("(%d, %d): fully %v, want %v", c.a, c.b, f, c.fully)
		}
	}
	in := normalised(track(seq(0, 99), func(int) (float64, float64) { return nan, nan }))
	if _, f := Hide(in, 2000, 2000); f || StepsOf(in) != nil {
		t.Error("indoor: fully hidden or steps")
	}
}

func TestHidingStepsShift(t *testing.T) {
	h := &HidingSteps{Start: []int{0, 3, -1}, End: []int{9, -1, 5}}
	got := h.Shift(10)
	if want := (HidingSteps{Start: []int{10, 13, -1}, End: []int{19, -1, 15}}); !equalSteps(*got, want) {
		t.Errorf("shift %v", got)
	}
	if h.Start[1] != 3 {
		t.Error("shift modified its input")
	}
	if (*HidingSteps)(nil).Shift(3) != nil {
		t.Error("nil shift")
	}
}

func equalSteps(a, b HidingSteps) bool {
	eq := func(x, y []int) bool {
		if len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	return eq(a.Start, b.Start) && eq(a.End, b.End)
}

func TestValidHideRadius(t *testing.T) {
	for v, want := range map[int]bool{0: true, 50: true, 2000: true, -50: false, 2050: false, 120: false, 25: false} {
		if ValidHideRadius(v) != want {
			t.Errorf("%d: %v", v, !want)
		}
	}
}
