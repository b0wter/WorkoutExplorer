package workout

import (
	"math"
	"testing"
	"time"
)

const kmh = 1 / 3.6

// spikeRide is a ride at 26 km/h with positions at the given offsets; the
// session carries max_speed 85 km/h and the lap device values.
func spikeRide(sport string, offsets []int, speed func(s int) float64, pos bool) Recording {
	rs := recs(offsets, func(i int, r *RawRecord) {
		s := offsets[i]
		r.Speed = speed(s)
		r.Distance = float64(s) * 26 * kmh
		r.HeartRate = 140
		if pos {
			r.Lat, r.Lon = 48+float64(s)*1e-5, 11
		}
	})
	last := offsets[len(offsets)-1]
	return Recording{
		Records:  rs,
		Sessions: []RawSession{{Sport: sport, Start: at(0), End: at(last), Totals: Totals{MaxSpeed: f64(85 * kmh)}}},
		Laps:     []RawLap{{Start: at(0), End: at(last), Totals: Totals{MaxSpeed: f64(85 * kmh), AvgSpeed: f64(26 * kmh)}}},
	}
}

func steady(bump map[int]float64) func(int) float64 {
	return func(s int) float64 {
		if v, ok := bump[s]; ok {
			return v * kmh
		}
		return 26 * kmh
	}
}

func burst(from, n int, v float64) map[int]float64 {
	m := map[int]float64{}
	for i := 0; i < n; i++ {
		m[from+i] = v
	}
	return m
}

func TestSpeedSpikeFilter(t *testing.T) {
	sprint := map[int]float64{} // 26 -> 45 -> 26 over 10 s
	for i := 1; i <= 9; i++ {
		sprint[100+i] = 26 + 19*(1-math.Abs(float64(i)-5)/5)
	}
	tests := []struct {
		name    string
		bump    map[int]float64
		removed []int // seconds that must be missing
		maxKmh  float64
	}{
		{"1 s spike", burst(100, 1, 85), []int{100}, 26},
		{"5 s spike", burst(100, 5, 85), []int{100, 101, 102, 103, 104}, 26},
		{"6 s glitch kept", burst(100, 6, 85), nil, 85},
		{"10 s sprint kept", sprint, nil, 85}, // nothing removed: device max_speed stays
		{"step kept", burst(100, 60, 40), nil, 85},
		{"dip kept", burst(100, 1, 10), nil, 85},
		{"no spike", nil, nil, 85},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(spikeRide("cycling", seq(0, 299), steady(tt.bump), true))
			base := Normalise(spikeRide("cycling", seq(0, 299), steady(nil), true))
			rm := map[int]bool{}
			for _, s := range tt.removed {
				rm[s] = true
			}
			for i, off := range w.Samples.Offset {
				want := steady(tt.bump)(int(off))
				if rm[int(off)] != math.IsNaN(w.Samples.Speed[i]) || (!rm[int(off)] && !near(w.Samples.Speed[i], want, 1e-9)) {
					t.Fatalf("speed at %d = %v, removed want %v", off, w.Samples.Speed[i], rm[int(off)])
				}
			}
			wantPtr(t, "max speed", w.Summary.MaxSpeed, tt.maxKmh*kmh, 1e-9)
			wantPtr(t, "moving", w.Summary.MovingTime, *base.Summary.MovingTime, 0)
			if !sameFloats(w.Samples.MovingTime, base.Samples.MovingTime) || !sameFloats(w.Samples.Distance, base.Samples.Distance) {
				t.Error("moving time or distance stream changed")
			}
			if *w.Laps[0].Totals.MaxSpeed != 85*kmh || *w.Laps[0].Totals.AvgSpeed != 26*kmh {
				t.Error("lap speed changed")
			}
			checkInvariants(t, w)
		})
	}
}

func TestSpeedSpikeSprintKeptMax(t *testing.T) {
	sprint := map[int]float64{}
	for i := 1; i <= 9; i++ {
		sprint[100+i] = 26 + 19*(1-math.Abs(float64(i)-5)/5)
	}
	r := spikeRide("cycling", seq(0, 299), steady(sprint), true)
	r.Sessions[0].Totals.MaxSpeed = nil
	w := Normalise(r)
	wantPtr(t, "max speed", w.Summary.MaxSpeed, 45*kmh, 1e-9)
}

func TestSpeedSpikeNoSpikeNoRecord(t *testing.T) {
	w := Normalise(spikeRide("cycling", seq(0, 299), steady(nil), true))
	if w.Samples.unfiltered != nil {
		t.Error("record kept without a removal")
	}
}

func TestSpeedSpikeRunning(t *testing.T) {
	r := spikeRide("running", seq(0, 299), func(s int) float64 {
		if s == 100 {
			return 12
		}
		return 3
	}, true)
	r.Sessions[0].Totals.MaxSpeed = f64(12)
	w := Normalise(r)
	if !math.IsNaN(w.Samples.Speed[100]) {
		t.Error("running spike kept")
	}
	wantPtr(t, "max", w.Summary.MaxSpeed, 3, 1e-9)
}

func TestSpeedSpikeStandstillJitter(t *testing.T) {
	// Stopped with 1.5 m/s jitter for 2 s, then a start from stop.
	f := func(s int) float64 {
		switch {
		case s == 50 || s == 51:
			return 1.5
		case s >= 100:
			return float64(min(s-100, 8))
		}
		return 0
	}
	w := Normalise(spikeRide("cycling", seq(0, 199), f, true))
	for i, off := range w.Samples.Offset {
		if math.IsNaN(w.Samples.Speed[i]) {
			t.Fatalf("speed at %d removed", off)
		}
	}
}

// QA-74-r1-1: a real roll between stops stays; a glitch from a stop goes.
func TestSpeedSpikeRollBetweenStops(t *testing.T) {
	for _, tc := range []struct {
		v    float64
		gone bool
	}{{2.5, false}, {85 * kmh, true}} {
		w := Normalise(spikeRide("cycling", seq(0, 199), func(s int) float64 {
			if s == 100 || s == 101 {
				return tc.v
			}
			return 0
		}, true))
		if math.IsNaN(w.Samples.Speed[100]) != tc.gone {
			t.Errorf("v=%v: removed=%v, want %v", tc.v, !tc.gone, tc.gone)
		}
	}
}

func TestSpeedSpikeUntouched(t *testing.T) {
	bump := burst(100, 1, 85)
	t.Run("indoor without positions", func(t *testing.T) {
		w := Normalise(spikeRide("cycling", seq(0, 299), steady(bump), false))
		if math.IsNaN(w.Samples.Speed[100]) {
			t.Error("removed without positions")
		}
	})
	t.Run("pool swim", func(t *testing.T) {
		r := spikeRide("swimming", seq(0, 299), steady(bump), true)
		r.Sessions[0].SubSport = "lap_swimming"
		if w := Normalise(r); math.IsNaN(w.Samples.Speed[100]) {
			t.Error("removed in a pool swim")
		}
	})
	t.Run("transition has no speed", func(t *testing.T) {
		w := Normalise(spikeRide("transition", seq(0, 299), steady(bump), true))
		if w.Samples.Speed != nil || w.Samples.unfiltered != nil {
			t.Error("transition handled by the filter")
		}
	})
}

func TestSpeedSpikeTriathlonLegs(t *testing.T) {
	// Swim without positions, bike with a spike, run with a spike.
	r := spikeRide("cycling", seq(0, 299), steady(nil), true)
	r.Sessions = []RawSession{
		{Sport: "swimming", Start: at(0), End: at(99), Totals: Totals{MaxSpeed: f64(20)}},
		{Sport: "cycling", Start: at(100), End: at(199), Totals: Totals{MaxSpeed: f64(85 * kmh)}},
		{Sport: "running", Start: at(200), End: at(299), Totals: Totals{MaxSpeed: f64(12)}},
	}
	for s := range r.Records {
		switch {
		case s < 100:
			r.Records[s].Lat, r.Records[s].Lon = nan, nan
			r.Records[s].Speed = 2
		case s >= 200:
			r.Records[s].Speed = 3
		}
	}
	r.Records[50].Speed = 20
	r.Records[110].Speed = 85 * kmh
	r.Records[250].Speed = 12
	w := Normalise(r)
	sp := w.Samples.Speed
	if math.IsNaN(sp[50]) || !math.IsNaN(sp[110]) || !math.IsNaN(sp[250]) {
		t.Errorf("legs: swim %v bike %v run %v", sp[50], sp[110], sp[250])
	}
	wantPtr(t, "swim max kept", w.Sessions[0].Totals.MaxSpeed, 20, 0)
	wantPtr(t, "bike max", w.Sessions[1].Totals.MaxSpeed, 26*kmh, 1e-9)
	wantPtr(t, "run max", w.Sessions[2].Totals.MaxSpeed, 3, 1e-9)
}

func TestSpeedSpikeSmartRecording(t *testing.T) {
	// 3-s spacing: R-3 interpolated the neighbours of the one recorded spike
	// sample; they go with it (SP-2).
	var offs []int
	for s := 0; s < 300; s += 3 {
		offs = append(offs, s)
	}
	w := Normalise(spikeRide("cycling", offs, steady(burst(150, 1, 85)), true))
	for i, off := range w.Samples.Offset {
		gone := off >= 148 && off <= 152
		if gone != math.IsNaN(w.Samples.Speed[i]) {
			t.Fatalf("speed at %d: missing = %v, want %v", off, math.IsNaN(w.Samples.Speed[i]), gone)
		}
	}
	base := Normalise(spikeRide("cycling", offs, steady(nil), true))
	wantPtr(t, "moving", w.Summary.MovingTime, *base.Summary.MovingTime, 0)
}

func TestSpeedSpikeTunnel(t *testing.T) {
	// No speed for a stretch, then a spike right after it.
	f := func(s int) float64 {
		switch {
		case s >= 100 && s < 140:
			return nan
		case s == 142:
			return 85 * kmh
		}
		return 26 * kmh
	}
	w := Normalise(spikeRide("cycling", seq(0, 299), f, true))
	if !math.IsNaN(w.Samples.Speed[142]) || math.IsNaN(w.Samples.Speed[141]) || math.IsNaN(w.Samples.Speed[143]) {
		t.Error("tunnel spike not removed alone")
	}
}

// A spike sample inside a stopped stretch still counts as moving, also after
// a trim (SP-5).
func TestSpeedSpikeMovingTimeAndTrim(t *testing.T) {
	f := func(s int) float64 {
		switch {
		case s == 120:
			return 30 * kmh
		case s >= 100 && s < 140:
			return 0
		}
		return 26 * kmh
	}
	w := Normalise(spikeRide("cycling", seq(0, 299), f, true))
	if !math.IsNaN(w.Samples.Speed[120]) {
		t.Fatal("setup: spike not removed")
	}
	wantPtr(t, "moving", w.Summary.MovingTime, 260, 0) // 299 intervals, 39 stopped
	start, end := w.StartTime.Add(60*time.Second), w.StartTime.Add(200*time.Second)
	a, b, ok := TrimWindow(w, &start, &end)
	if !ok {
		t.Fatal("window")
	}
	tw := Trim(w, a, b)
	if !math.IsNaN(tw.Samples.Speed[120-a]) {
		t.Error("trim lost the removal")
	}
	wantPtr(t, "trimmed moving", tw.Summary.MovingTime, 101, 0) // 140 intervals, 39 stopped
}
