package workout

import (
	"math"
	"reflect"
	"testing"
)

// Tests of the F-08 v1.4 rule R-14, the distance stream never falls (D-25,
// QA-08-r3-2).

// dRecs returns 1 s records carrying the given distances (nan = missing).
func dRecs(d ...float64) []RawRecord {
	return recs(seq(0, len(d)-1), func(i int, r *RawRecord) { r.Distance = d[i] })
}

func TestHoldDistance(t *testing.T) {
	cases := []struct {
		name     string
		in, want []float64
	}{
		{"non-decreasing is unchanged", []float64{0, 5, 5, 10, 15}, []float64{0, 5, 5, 10, 15}},
		{"a fall is held until the count passes it", []float64{0, 100, 150, 120, 140, 160, 170},
			[]float64{0, 100, 150, 150, 150, 160, 170}},
		{"a run of values below 1 m after 150 m is held at 150", []float64{0, 100, 150, 0.2, 0.3, 0.4, 0.9, 155},
			[]float64{0, 100, 150, 150, 150, 150, 150, 155}},
		{"NaN stays NaN and does not reset the high", []float64{0, 100, nan, 50, nan, 120},
			[]float64{0, 100, nan, 100, nan, 120}},
		{"leading NaNs, then the first valid value starts the high", []float64{nan, nan, 30, 20, 40},
			[]float64{nan, nan, 30, 30, 40}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := Normalise(Recording{Records: dRecs(c.in...)})
			if !sameFloats(w.Samples.Distance, c.want) {
				t.Errorf("distance = %v, want %v", w.Samples.Distance, c.want)
			}
			if c.name == "non-decreasing is unchanged" {
				for i, v := range w.Samples.Distance {
					if math.Float64bits(v) != math.Float64bits(c.in[i]) {
						t.Errorf("not bit for bit at %d", i)
					}
				}
			}
		})
	}
}

func TestHoldDistanceFilledSample(t *testing.T) {
	// A gap of 4 s between a high (100 at 0 s) and a low (50 at 4 s) is filled
	// by R-3; the filled samples lie between the neighbours and are held too.
	rs := recs([]int{0, 4, 5}, func(i int, r *RawRecord) { r.Distance = []float64{100, 50, 60}[i] })
	w := Normalise(Recording{Records: rs})
	for i, v := range w.Samples.Distance {
		if v != 100 {
			t.Errorf("sample %d (offset %d) = %v, want 100", i, w.Samples.Offset[i], v)
		}
	}
	if len(w.Samples.Distance) < 3 {
		t.Fatalf("%d samples", len(w.Samples.Distance))
	}
}

func TestHoldDistanceDerived(t *testing.T) {
	w := Normalise(Recording{Records: dRecs(0, 100, 200, 5)})
	if w.Derived.Distance == nil || *w.Derived.Distance != 200 {
		t.Errorf("Derived.Distance = %v, want the high 200", deref(w.Derived.Distance))
	}
	// Truncated, a fall in the record-leg tail: the leg's distance is >= 0.
	rs := dRecs(0, 100, 200, 300, 400, 50, 60, 70)
	w = Normalise(Recording{Records: rs, Integrity: Integrity{Truncated: true},
		Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(3), Totals: Totals{Distance: f64(300)}}}})
	if w.Summary.Distance == nil || *w.Summary.Distance != 400 {
		t.Errorf("summary distance = %v, want 400 (session 300 + leg 100)", deref(w.Summary.Distance))
	}
}

func TestHoldDistancePositionsOnly(t *testing.T) {
	// No device distance: R-6b is non-decreasing and unaffected.
	rs := recs(seq(0, 5), func(i int, r *RawRecord) { r.Lat, r.Lon = 50+float64(i)*1e-4, 8 })
	w := Normalise(Recording{Records: rs})
	if w.Samples.Distance == nil {
		t.Fatal("no computed distance")
	}
	for i := 1; i < len(w.Samples.Distance); i++ {
		if w.Samples.Distance[i] < w.Samples.Distance[i-1] {
			t.Fatalf("falls at %d", i)
		}
	}
}

func TestHoldDistanceChainedBoundary(t *testing.T) {
	// A continuing count 0.01 m below prev (R-12 unchanged) is held: no
	// negative step.
	a := dRecs(0, 5, 10, 15)
	b := recs(seq(10, 13), func(i int, r *RawRecord) { r.Distance = 14.99 + 5*float64(i) })
	w := Normalise(Recording{Records: append(a, b...), SequenceStarts: []int{4}})
	for i := 1; i < len(w.Samples.Distance); i++ {
		if w.Samples.Distance[i]-w.Samples.Distance[i-1] < 0 {
			t.Errorf("negative step at %d", i)
		}
	}
	if w.Samples.Distance[4] != 15 {
		t.Errorf("boundary sample = %v, want 15 (held)", w.Samples.Distance[4])
	}
}

func TestHoldDistanceInputUnchanged(t *testing.T) {
	in := dRecs(0, 100, 5, 200)
	want := append([]RawRecord(nil), in...)
	rec := Recording{Records: in}
	Normalise(rec)
	if !reflect.DeepEqual(in, want) && !sameRecs(in, want) {
		t.Error("the input Recording changed")
	}
}

// sameRecs compares records treating NaN as equal to NaN (DeepEqual does not).
func sameRecs(a, b []RawRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if !x.At.Equal(y.At) || !sameFloats([]float64{x.Distance, x.Lat, x.Speed}, []float64{y.Distance, y.Lat, y.Speed}) {
			return false
		}
	}
	return true
}

// TestHoldDistanceOrder pins R-14 after R-3 (QA-08-r4-1): R-3 interpolates the
// recorded 50 -> 200 over 4 s, and the hold then applies to the filled stream.
// Holding before the fill (mutant H3) turns the 50 into 100 first, so the fill
// interpolates 100 -> 200 and gives [100 100 125 150 175 200].
func TestHoldDistanceOrder(t *testing.T) {
	rs := recs([]int{0, 1, 5}, func(i int, r *RawRecord) { r.Distance = []float64{100, 50, 200}[i] })
	w := Normalise(Recording{Records: rs})
	want := []float64{100, 100, 100, 125, 162.5, 200}
	if !sameFloats(w.Samples.Distance, want) {
		t.Errorf("distance = %v, want %v", w.Samples.Distance, want)
	}
}
