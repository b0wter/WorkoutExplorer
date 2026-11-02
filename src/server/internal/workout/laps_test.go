package workout

import (
	"math"
	"reflect"
	"testing"
)

// lapRide is a 600 s ride with every stream, a timer pause (200–260 s), a
// stretch without distance (100–109 s), zeros in power and cadence, and a
// climb, so that every L-1 key has a value and every R-2 rule matters.
func lapRide(laps []RawLap) Recording {
	records := recs(seq(0, 600), func(i int, r *RawRecord) {
		s := float64(i)
		r.Lat, r.Lon = 47+s*1e-4, 8+s*1e-4
		r.Altitude = 400 + 30*math.Sin(s/60)
		r.Distance = 7 * s
		if i >= 100 && i < 110 {
			r.Distance = nan
		}
		r.Speed = 7
		r.HeartRate = 120 + float64(i%30)
		r.Cadence = float64(80 + i%7)
		r.Power = float64(150 + i%50)
		if i%11 == 0 {
			r.Cadence, r.Power = 0, 0
		}
		r.Temperature = 20
	})
	return Recording{
		Records:  records,
		Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(600)}},
		Laps:     laps,
		Timer: []RawTimerEvent{
			{At: at(0), Kind: TimerStart}, {At: at(200), Kind: TimerStop},
			{At: at(260), Kind: TimerStart}, {At: at(600), Kind: TimerStop},
		},
	}
}

// bareLaps returns laps over the given spans with no device values.
func bareLaps(spans ...[2]int) []RawLap {
	out := make([]RawLap, len(spans))
	for i, sp := range spans {
		out[i] = RawLap{Start: at(sp[0]), End: at(sp[1])}
	}
	return out
}

// bruteRange evaluates the F-17 range values table over [a, b] directly,
// without the helpers of laps.go.
func bruteRange(s Samples, a, b int) map[string]*float64 {
	out := map[string]*float64{}
	e := float64(s.Offset[b] - s.Offset[a])
	out["elapsed_time"] = &e
	dbar := func(i int) (float64, bool) {
		for j := i; j >= 0; j-- {
			if !math.IsNaN(s.Distance[j]) {
				return s.Distance[j], true
			}
		}
		for _, x := range s.Distance {
			if !math.IsNaN(x) {
				return x, true
			}
		}
		return 0, false
	}
	if s.Distance != nil {
		da, ok := dbar(a)
		db, _ := dbar(b)
		if ok {
			d := db - da
			out["distance"] = &d
			if s.MovingTime != nil && s.MovingTime[b]-s.MovingTime[a] > 0 {
				v := d / (s.MovingTime[b] - s.MovingTime[a])
				out["avg_speed"] = &v
			}
		}
	}
	if s.Gain != nil {
		g := s.Gain[b] - s.Gain[a]
		out["ascent"] = &g
	}
	mean := func(v []float64, skipZero bool) *float64 {
		sum, n := 0.0, 0
		for i := a; v != nil && i <= b; i++ {
			if !math.IsNaN(v[i]) && !(skipZero && v[i] == 0) {
				sum += v[i]
				n++
			}
		}
		if n == 0 {
			return nil
		}
		m := sum / float64(n)
		return &m
	}
	out["avg_heart_rate"] = mean(s.HeartRate, false)
	out["avg_power"] = mean(s.Power, false)
	out["avg_cadence"] = mean(s.Cadence, true)
	return out
}

// lapKeys are the L-1 keys and their Totals fields.
var lapKeys = map[string]func(*Totals) *float64{
	"elapsed_time":   func(t *Totals) *float64 { return t.ElapsedTime },
	"distance":       func(t *Totals) *float64 { return t.Distance },
	"ascent":         func(t *Totals) *float64 { return t.Ascent },
	"avg_speed":      func(t *Totals) *float64 { return t.AvgSpeed },
	"avg_heart_rate": func(t *Totals) *float64 { return t.AvgHeartRate },
	"avg_power":      func(t *Totals) *float64 { return t.AvgPower },
	"avg_cadence":    func(t *Totals) *float64 { return t.AvgCadence },
}

// notCompleted are the Totals fields L-1 never fills.
func notCompleted(t Totals) []*float64 {
	return []*float64{t.TimerTime, t.Descent, t.Calories, t.MaxSpeed, t.MaxHeartRate, t.MaxCadence,
		t.MaxPower, t.AvgTemperature, t.MaxTemperature}
}

func TestCompleteLapsEqualsRangeTable(t *testing.T) {
	// Laps across the missing distance, across the pause, and inside the pause.
	w := Normalise(lapRide(bareLaps([2]int{0, 150}, [2]int{150, 300}, [2]int{300, 600}, [2]int{205, 255}, [2]int{95, 105})))
	if len(w.Laps) != 5 {
		t.Fatalf("%d laps", len(w.Laps))
	}
	for i, l := range w.Laps {
		from, to := SampleRange(w.Samples.Offset, l.Start, l.End)
		want := bruteRange(w.Samples, from, to-1)
		for k, get := range lapKeys {
			got := get(&l.Totals)
			if want[k] == nil {
				wantNil(t, k, got)
				continue
			}
			if got == nil || !near(*got, *want[k], 1e-9) {
				t.Errorf("lap %d %s = %v, want %v", i, k, deref(got), *want[k])
			}
		}
		for _, p := range notCompleted(l.Totals) {
			if p != nil {
				t.Errorf("lap %d: a key outside L-1 was filled", i)
			}
		}
	}
	// The pause lap: elapsed time without moving time gives no speed.
	if l := w.Laps[3].Totals; l.AvgSpeed != nil || l.ElapsedTime == nil || *l.ElapsedTime != 50 {
		t.Errorf("pause lap: speed %v, elapsed %v", deref(l.AvgSpeed), deref(l.ElapsedTime))
	}
	// Power counts zeros, cadence does not (spot check against the input).
	if w.Laps[0].Totals.AvgCadence == nil || *w.Laps[0].Totals.AvgCadence < 80 {
		t.Errorf("cadence mean includes zeros: %v", deref(w.Laps[0].Totals.AvgCadence))
	}
}

func TestCompleteLapsKeepsDeviceValues(t *testing.T) {
	dev := Totals{ElapsedTime: f64(1), Distance: f64(2), Ascent: f64(3), AvgSpeed: f64(4),
		AvgHeartRate: f64(5), AvgPower: f64(6), AvgCadence: f64(7)}
	partial := Totals{Distance: f64(1234), AvgHeartRate: f64(99)}
	w := Normalise(lapRide([]RawLap{
		{Start: at(0), End: at(300), Totals: dev},
		{Start: at(300), End: at(600), Totals: partial},
	}))
	if got := w.Laps[0].Totals; !reflect.DeepEqual(got, dev) {
		t.Errorf("full device lap changed: %+v", got)
	}
	got := w.Laps[1].Totals
	wantPtr(t, "device distance", got.Distance, 1234, 0)
	wantPtr(t, "device heart rate", got.AvgHeartRate, 99, 0)
	from, to := SampleRange(w.Samples.Offset, 300, 600)
	want := bruteRange(w.Samples, from, to-1)
	for _, k := range []string{"elapsed_time", "ascent", "avg_speed", "avg_power", "avg_cadence"} {
		wantPtr(t, k, lapKeys[k](&got), *want[k], 1e-9)
	}
}

func TestCompleteLapsTooFewSamples(t *testing.T) {
	rec := lapRide(bareLaps([2]int{205, 255}, [2]int{42, 42}, [2]int{600, 700}))
	// A lap inside a gap: drop the records of 205–255 so it has none.
	var keep []RawRecord
	for _, r := range rec.Records {
		if s := r.At.Sub(t0).Seconds(); s < 205 || s > 255 {
			keep = append(keep, r)
		}
	}
	rec.Records = keep
	w := Normalise(rec)
	for i, l := range w.Laps {
		from, to := SampleRange(w.Samples.Offset, l.Start, l.End)
		if to-from >= 2 {
			t.Fatalf("lap %d has %d samples; the test needs 0 or 1", i, to-from)
		}
		if !reflect.DeepEqual(l.Totals, Totals{}) {
			t.Errorf("lap %d (%d samples) got values: %+v", i, to-from, l.Totals)
		}
	}
}

func TestCompleteLapsTransition(t *testing.T) {
	rec := lapRide(bareLaps([2]int{0, 200}, [2]int{200, 400}, [2]int{400, 600}))
	rec.Sessions = []RawSession{
		{Sport: "cycling", Start: at(0), End: at(199)},
		{Sport: "transition", Start: at(200), End: at(399)},
		{Sport: "running", Start: at(400), End: at(600)},
	}
	rec.Timer = nil
	w := Normalise(rec)
	tr := w.Laps[1].Totals
	for name, p := range map[string]*float64{"speed": tr.AvgSpeed, "cadence": tr.AvgCadence, "power": tr.AvgPower} {
		wantNil(t, "transition lap "+name, p)
	}
	for name, p := range map[string]*float64{"elapsed": tr.ElapsedTime, "distance": tr.Distance, "gain": tr.Ascent, "heart rate": tr.AvgHeartRate} {
		if p == nil {
			t.Errorf("transition lap %s missing", name)
		}
	}
	// The lap after it is not a transition lap: its rate values are computed.
	if r := w.Laps[2].Totals; r.AvgSpeed == nil || r.AvgCadence == nil || r.AvgPower == nil {
		t.Errorf("run lap: speed %v cadence %v power %v", deref(r.AvgSpeed), deref(r.AvgCadence), deref(r.AvgPower))
	}
}

func TestCompleteLapsUnchangedByHiding(t *testing.T) {
	w := Normalise(lapRide(bareLaps([2]int{0, 300}, [2]int{300, 600})))
	h, full := HideEnds(w)
	if full {
		t.Fatal("route fully hidden; the test needs a visible part")
	}
	if reflect.DeepEqual(h.Samples.Lat, w.Samples.Lat) {
		t.Fatal("hiding changed no position; the test needs a cut")
	}
	if !reflect.DeepEqual(h.Laps, w.Laps) {
		t.Errorf("hiding changed the laps:\n got %+v\nwant %+v", h.Laps, w.Laps)
	}
}
