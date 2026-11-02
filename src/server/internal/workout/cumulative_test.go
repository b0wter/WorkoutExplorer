package workout

import (
	"math"
	"math/rand/v2"
	"testing"
)

// checkCumulative asserts the ADR-029 invariants: length, non-decreasing,
// first value 0, last value equals the total.
func checkCumulative(t *testing.T, name string, cum []float64, n int, total *float64) {
	t.Helper()
	if total == nil {
		if cum != nil {
			t.Errorf("%s: series present without a total", name)
		}
		return
	}
	if len(cum) != n {
		t.Fatalf("%s: %d values for %d samples", name, len(cum), n)
	}
	if cum[0] != 0 {
		t.Errorf("%s[0] = %v, want 0", name, cum[0])
	}
	for i := 1; i < n; i++ {
		if cum[i] < cum[i-1] {
			t.Fatalf("%s decreases at %d: %v < %v", name, i, cum[i], cum[i-1])
		}
	}
	if math.Abs(cum[n-1]-*total) > 1e-9 {
		t.Errorf("%s ends at %v, total is %v", name, cum[n-1], *total)
	}
}

func TestCumulativeMovingTimeTimer(t *testing.T) {
	w := Normalise(Recording{
		Records: recs(seq(0, 40), nil),
		Timer: []RawTimerEvent{
			{At: at(0), Kind: TimerStart}, {At: at(10), Kind: TimerStop},
			{At: at(20), Kind: TimerStart}, {At: at(30), Kind: TimerStop},
		},
	})
	checkCumulative(t, "M", w.Samples.MovingTime, 41, w.Derived.MovingTime)
	for off, want := range map[int]float64{0: 0, 5: 5, 10: 10, 15: 10, 20: 10, 25: 15, 30: 20, 40: 20} {
		if got := w.Samples.MovingTime[off]; got != want {
			t.Errorf("M[%d] = %v, want %v", off, got, want)
		}
	}
}

func TestCumulativeMovingTimeTimerOutsideSamples(t *testing.T) {
	// A timer span that starts before the first sample and one that stops
	// after the last: the total counts them (R-7), the last sample carries
	// them so that M[N-1] equals the total.
	w := Normalise(Recording{
		Records: recs(seq(10, 20), nil),
		Timer:   []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(30), Kind: TimerStop}},
	})
	checkCumulative(t, "M", w.Samples.MovingTime, 11, w.Derived.MovingTime)
}

func TestCumulativeMovingTimeTimerBeforeFirstSample(t *testing.T) {
	// ADR-029 rev. 1 (QA-17-r1-1): the session and the timer start at 0, the
	// first record comes at 3 s. M[0] = 0, the 3 s count in the first
	// interval, and the stop 5 s after the last record counts in the last.
	w := Normalise(Recording{
		Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(25)}},
		Records:  recs(seq(3, 20), nil),
		Timer:    []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(25), Kind: TimerStop}},
	})
	if w.Samples.Offset[0] != 3 {
		t.Fatalf("first offset %d, want 3", w.Samples.Offset[0])
	}
	checkCumulative(t, "M", w.Samples.MovingTime, 18, w.Derived.MovingTime)
	for i, want := range map[int]float64{0: 0, 1: 4, 16: 19, 17: 25} {
		if got := w.Samples.MovingTime[i]; got != want {
			t.Errorf("M[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestCumulativeMovingTimeSpeed(t *testing.T) {
	// Moving for 10 s, stopped for 10 s, moving for 10 s (cycling, 1 km/h).
	w := Normalise(Recording{
		Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(30)}},
		Records: recs(seq(0, 30), func(i int, r *RawRecord) {
			if i >= 1 && i <= 10 || i > 20 {
				r.Speed = 5
			} else {
				r.Speed = 0
			}
		}),
	})
	if w.Derived.MovingTimeSource != movingTimeFromSpeed {
		t.Fatalf("source = %q", w.Derived.MovingTimeSource)
	}
	checkCumulative(t, "M", w.Samples.MovingTime, 31, w.Derived.MovingTime)
	for off, want := range map[int]float64{0: 0, 10: 10, 20: 10, 30: 20} {
		if got := w.Samples.MovingTime[off]; got != want {
			t.Errorf("M[%d] = %v, want %v", off, got, want)
		}
	}
}

func TestCumulativeAbsent(t *testing.T) {
	w := Normalise(Recording{Records: recs(seq(0, 20), nil)})
	if w.Samples.MovingTime != nil || w.Samples.Gain != nil || w.Derived.MovingTime != nil || w.Derived.ElevationGain != nil {
		t.Errorf("no source, no altitude: want no series, got M %v G %v", w.Samples.MovingTime, w.Samples.Gain)
	}
}

func TestCumulativeGain(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	const n = 3000
	alt := 100.0
	w := Normalise(Recording{Records: recs(seq(0, n-1), func(i int, r *RawRecord) {
		alt += rng.Float64()*1.4 - 0.6 // a drifting climb with noise
		if i%7 != 0 || i < 5 || i > n-5 {
			r.Altitude = alt
		} // else: missing altitude carries the previous value
	})})
	if w.Derived.ElevationGain == nil || *w.Derived.ElevationGain < 50 {
		t.Fatalf("gain = %v, want a real climb", deref(w.Derived.ElevationGain))
	}
	checkCumulative(t, "G", w.Samples.Gain, n, w.Derived.ElevationGain)
}

func TestCumulativeGainZigZag(t *testing.T) {
	// Up 10, down 10, up 10: the confirmed gain is 20 and G steps up with
	// the climbs only.
	var a []float64
	for _, x := range []float64{0, 10, 0, 10} {
		a = append(a, x)
	}
	g, l, cum := zigZagCum(a, 3)
	if g != 20 || l != 10 || cum[len(cum)-1] != g || cum[0] != 0 {
		t.Errorf("gain %v loss %v cum %v", g, l, cum)
	}
}

func TestCumulativeSeriesSurviveHiding(t *testing.T) {
	// F-16 H-1: the cut touches latitude and longitude only.
	w := Normalise(Recording{Records: recs(seq(0, 2000), func(i int, r *RawRecord) {
		r.Lat, r.Lon = 47+float64(i)*0.0001, 8
		r.Altitude, r.Speed, r.Distance = float64(i)/10, 5, float64(i)*5
	})})
	if w.Samples.MovingTime == nil || w.Samples.Gain == nil {
		t.Fatal("expected both series")
	}
	out, hidden := HideEnds(w)
	if hidden {
		t.Fatal("route fully hidden")
	}
	if !sameFloats(out.Samples.MovingTime, w.Samples.MovingTime) || !sameFloats(out.Samples.Gain, w.Samples.Gain) {
		t.Error("hiding changed a cumulative series")
	}
}
