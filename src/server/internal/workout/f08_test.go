package workout

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
	"time"
)

// TestSampleCount pins SampleCount to len(Normalise(rec).Samples.Offset)
// (F-08 design, Model changes) for hand-made and seeded random recordings.
func TestSampleCount(t *testing.T) {
	check := func(t *testing.T, name string, r Recording) {
		t.Helper()
		if got, want := SampleCount(r), len(Normalise(r).Samples.Offset); got != want {
			t.Errorf("%s: SampleCount = %d, Normalise gives %d", name, got, want)
		}
	}
	check(t, "empty", Recording{})
	check(t, "records, no valid time", Recording{Records: []RawRecord{{}}})
	check(t, "gaps", Recording{Records: recs([]int{0, 1, 3, 7, 8, 12, 13, 14}, nil)})
	check(t, "session start before records", Recording{
		Sessions: []RawSession{{Start: at(-30)}}, Records: recs([]int{0, 2, 2, 5}, nil)})

	rng := rand.New(rand.NewPCG(8, 8))
	for n := 0; n < 1000; n++ {
		var rs []RawRecord
		for i, count := 0, rng.IntN(60); i < count; i++ {
			r := rec(rng.IntN(120))
			if rng.IntN(10) == 0 {
				r.At = time.Time{} // invalid time
			}
			if rng.IntN(5) == 0 {
				r.At = r.At.Add(time.Duration(rng.IntN(1000)) * time.Millisecond)
			}
			rs = append(rs, r)
		}
		var sessions []RawSession
		if rng.IntN(3) == 0 {
			sessions = []RawSession{{Start: at(rng.IntN(100) - 50)}}
		}
		check(t, "random", Recording{Sessions: sessions, Records: rs})
	}
}

func TestRecordTotals(t *testing.T) {
	w := Normalise(Recording{
		Timer: []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(4), Kind: TimerStop}},
		Records: recs(seq(0, 4), func(i int, r *RawRecord) {
			r.Distance = float64(i) * 10
			r.Speed = []float64{0, 2, 4, 6, 8}[i]
			r.HeartRate = float64(100 + 10*i)
			r.Cadence = []float64{0, 80, 90, 0, 100}[i]
			r.Power = []float64{0, 100, 200, 0, 300}[i]
			r.Temperature = float64(10 + i)
		}),
	})
	d := w.Derived
	wantPtr(t, "AvgSpeed = distance / moving time", d.AvgSpeed, 40.0/4, 1e-9)
	wantPtr(t, "MaxSpeed", d.MaxSpeed, 8, 0)
	wantPtr(t, "AvgHeartRate", d.AvgHeartRate, 120, 1e-9)
	wantPtr(t, "MaxHeartRate", d.MaxHeartRate, 140, 0)
	wantPtr(t, "AvgCadence excludes zeros", d.AvgCadence, 90, 1e-9)
	wantPtr(t, "MaxCadence", d.MaxCadence, 100, 0)
	wantPtr(t, "AvgPower includes zeros", d.AvgPower, 120, 1e-9)
	wantPtr(t, "MaxPower", d.MaxPower, 300, 0)
	wantPtr(t, "AvgTemperature", d.AvgTemperature, 12, 1e-9)
	wantPtr(t, "MaxTemperature", d.MaxTemperature, 14, 0)

	// Absent streams give nil; zero moving time gives no average speed.
	w = Normalise(Recording{Records: recs(seq(0, 3), func(i int, r *RawRecord) { r.Distance = float64(i) })})
	wantNil(t, "AvgHeartRate", w.Derived.AvgHeartRate)
	wantNil(t, "MaxSpeed", w.Derived.MaxSpeed)
	w = Normalise(Recording{
		Timer:   []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(0), Kind: TimerStop}},
		Records: recs(seq(0, 3), func(i int, r *RawRecord) { r.Distance = float64(i) }),
	})
	wantNil(t, "AvgSpeed with zero moving time", w.Derived.AvgSpeed)

	// A cadence stream of zeros has a maximum but no mean.
	w = Normalise(Recording{Records: recs(seq(0, 3), func(i int, r *RawRecord) { r.Cadence = 0 })})
	wantNil(t, "AvgCadence of zeros", w.Derived.AvgCadence)
	wantPtr(t, "MaxCadence of zeros", w.Derived.MaxCadence, 0, 0)
}

func TestElevationLoss(t *testing.T) {
	offsets := make([]int32, 200)
	climb := make([]float64, 200)
	descent := make([]float64, 200)
	for i := range offsets {
		offsets[i] = int32(i)
		climb[i] = float64(i) * 0.5
		descent[i] = 100 - float64(i)*0.5
	}
	up, _ := elevation(offsets, climb, hysteresisMetres)
	_, down := elevation(offsets, descent, hysteresisMetres)
	if up == nil || down == nil || *up != *down || *up < 90 {
		t.Errorf("mirrored profile: gain %v, loss %v", deref(up), deref(down))
	}
	// Up, down, up: loss equals the mirrored gain of the middle leg.
	zig := []float64{0, 10, 0, 10}
	g, l := zigZag(zig, 3)
	if g != 20 || l != 10 {
		t.Errorf("zigzag gain %v loss %v, want 20 and 10", g, l)
	}
	g, l = zigZag([]float64{10, 0, 10, 0}, 3)
	if g != 10 || l != 20 {
		t.Errorf("mirrored zigzag gain %v loss %v, want 10 and 20", g, l)
	}
	flat := make([]float64, 200)
	rng := rand.New(rand.NewPCG(1, 1))
	for i := range flat {
		flat[i] = 100 + rng.Float64() - 0.5
	}
	g2, l2 := elevation(offsets, flat, hysteresisMetres)
	if g2 == nil || *g2 != 0 || *l2 != 0 {
		t.Errorf("flat noise: gain %v loss %v, want 0", deref(g2), deref(l2))
	}
	if g3, l3 := elevation(offsets, nil, hysteresisMetres); g3 != nil || l3 != nil {
		t.Errorf("no altitude: want nil, nil")
	}
}

// TestSummaryRevised covers R-10 as revised by F-08.
func TestSummaryRevised(t *testing.T) {
	records := recs(seq(0, 40), func(i int, r *RawRecord) {
		r.Distance = float64(i) * 5
		r.Speed = 5
		r.HeartRate = 130
		r.Cadence = 80
		r.Power = 100
	})
	run := Totals{ElapsedTime: f64(20), TimerTime: f64(18), Distance: f64(30), Ascent: f64(5), Calories: f64(100),
		MaxSpeed: f64(4), MaxHeartRate: f64(170), AvgHeartRate: f64(150), MaxCadence: f64(95), AvgCadence: f64(90)}
	ride := Totals{ElapsedTime: f64(20), TimerTime: f64(19), Distance: f64(80), Ascent: f64(7), Calories: f64(50),
		MaxSpeed: f64(9), MaxHeartRate: f64(160), AvgHeartRate: f64(140), MaxCadence: f64(110)}

	t.Run("mixed sports", func(t *testing.T) {
		w := Normalise(Recording{Records: records, Sessions: []RawSession{
			{Sport: "running", Start: at(0), End: at(20), Totals: run},
			{Sport: "cycling", Start: at(20), End: at(40), Totals: ride}}})
		s := w.Summary
		wantPtr(t, "ElapsedTime sum", s.ElapsedTime, 40, 0)
		wantPtr(t, "TimerTime sum", s.TimerTime, 37, 0)
		wantPtr(t, "Distance sum", s.Distance, 110, 0)
		wantPtr(t, "Ascent sum", s.Ascent, 12, 0)
		wantPtr(t, "Calories sum", s.Calories, 150, 0)
		wantPtr(t, "MaxSpeed max", s.MaxSpeed, 9, 0)
		wantPtr(t, "MaxHeartRate max", s.MaxHeartRate, 170, 0)
		wantPtr(t, "AvgHeartRate from records", s.AvgHeartRate, 130, 1e-9)
		wantPtr(t, "AvgPower from records", s.AvgPower, 100, 1e-9)
		wantNil(t, "AvgSpeed hidden", s.AvgSpeed)
		wantNil(t, "AvgCadence hidden", s.AvgCadence)
		wantNil(t, "MaxCadence hidden", s.MaxCadence)
		wantNil(t, "Descent has no source", s.Descent)
	})
	t.Run("same sport", func(t *testing.T) {
		w := Normalise(Recording{Records: records, Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(20), Totals: run},
			{Sport: "cycling", Start: at(20), End: at(40), Totals: ride}}})
		wantPtr(t, "MaxCadence max of sessions", w.Summary.MaxCadence, 110, 0)
		wantPtr(t, "AvgSpeed from records", w.Summary.AvgSpeed, 5, 1e-9)
		wantPtr(t, "AvgCadence from records", w.Summary.AvgCadence, 80, 1e-9)
	})
	t.Run("one session fills only what is missing", func(t *testing.T) {
		w := Normalise(Recording{Records: records, Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(40), Totals: run}}})
		s := w.Summary
		wantPtr(t, "AvgHeartRate device", s.AvgHeartRate, 150, 0)
		wantPtr(t, "MaxCadence device", s.MaxCadence, 95, 0)
		wantPtr(t, "AvgPower filled", s.AvgPower, 100, 1e-9)
		wantPtr(t, "TimerTime device", s.TimerTime, 18, 0)
	})
	t.Run("no session", func(t *testing.T) {
		w := Normalise(Recording{Records: records})
		s := w.Summary
		for name, p := range map[string]*float64{"ElapsedTime": s.ElapsedTime, "Distance": s.Distance, "MovingTime": s.MovingTime,
			"AvgSpeed": s.AvgSpeed, "AvgHeartRate": s.AvgHeartRate, "AvgPower": s.AvgPower} {
			if p == nil || *p == 0 {
				t.Errorf("%s = %v, want a non-zero value", name, deref(p))
			}
		}
		wantNil(t, "TimerTime never filled", s.TimerTime)
		wantNil(t, "Calories never filled", s.Calories)
	})
}

func TestIntegrity(t *testing.T) {
	rs := recs(seq(0, 9), nil)
	w := Normalise(Recording{Records: rs, Integrity: Integrity{Truncated: true, DataEnd: 99, Sequences: 1, IgnoredBytes: 3}})
	if w.Integrity.DataEnd != 9 || !w.Integrity.Truncated || w.Integrity.Sequences != 1 || w.Integrity.IgnoredBytes != 3 {
		t.Errorf("integrity = %+v, want DataEnd 9 and the other fields copied", w.Integrity)
	}
	w = Normalise(Recording{Records: rs, Integrity: Integrity{CRCFailed: true, DataEnd: 99}})
	if w.Integrity.DataEnd != 0 || !w.Integrity.CRCFailed {
		t.Errorf("integrity = %+v, want DataEnd 0 when not truncated", w.Integrity)
	}
	w = Normalise(Recording{Integrity: Integrity{Truncated: true}})
	if w.Integrity.DataEnd != 0 {
		t.Errorf("DataEnd without samples = %d, want 0", w.Integrity.DataEnd)
	}
}

func TestReportVersion2(t *testing.T) {
	rep := func(ig Integrity) map[string]any {
		b, err := ReportJSON(Normalise(Recording{Records: recs(seq(0, 9), nil), Integrity: ig}))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := rep(Integrity{Sequences: 1})
	if m["report_version"] != float64(2) {
		t.Errorf("report_version = %v", m["report_version"])
	}
	ig := m["integrity"].(map[string]any)
	if _, ok := ig["data_end"]; ok {
		t.Errorf("data_end written for a file that is not truncated")
	}
	for _, k := range []string{"truncated", "crc_failed", "sequences", "skipped_sequences", "ignored_bytes"} {
		if _, ok := ig[k]; !ok {
			t.Errorf("integrity.%s missing", k)
		}
	}
	ig = rep(Integrity{Truncated: true, Sequences: 1})["integrity"].(map[string]any)
	if ig["data_end"] != float64(9) {
		t.Errorf("data_end = %v, want 9", ig["data_end"])
	}
}
