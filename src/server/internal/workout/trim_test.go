package workout

import (
	"testing"
	"time"
)

// F-47 (UP-6): the cut of the normalised workout. Synthetic records only.

// trimRun is 900 s at 1 s: the first 600 s a run (3 m/s, heart rate 140), the
// last 300 s a drive (15 m/s, heart rate 177, 200 m of altitude more), with
// two laps, one cut by an end trim at 450.
func trimRun() Workout {
	rs := recs(seq(0, 899), func(i int, r *RawRecord) {
		dist := float64(i) * 3
		r.Speed, r.HeartRate, r.Altitude = 3, 140, 100+float64(i%7)
		if i >= 600 {
			dist = 1800 + float64(i-600)*15
			r.Speed, r.HeartRate, r.Altitude = 15, 177, 300+float64(i-600)
		}
		r.Distance = dist
		r.Lat, r.Lon = 50+dist/111195.0, 8
	})
	return Normalise(Recording{
		FileType: "activity",
		Sessions: []RawSession{{Sport: "running", Start: at(0), End: at(899), Totals: Totals{Calories: f64(500), Distance: f64(6300)}}},
		Laps: []RawLap{
			{Start: at(0), End: at(400), Trigger: "distance"},
			{Start: at(400), End: at(899), Trigger: "button"},
		},
		Records: rs,
	})
}

func tm(s int) *time.Time { t := at(s); return &t }

func TestTrimWindowRulesAndMinimum(t *testing.T) {
	w := trimRun()
	for _, c := range []struct {
		name       string
		start, end *time.Time
		a, b       int
		ok         bool
	}{
		{"whole", nil, nil, 0, 899, true},
		{"inward", tm(10), tm(500), 10, 500, true},
		{"60 s", tm(100), tm(160), 100, 160, true},
		{"59 s", tm(100), tm(159), 0, 0, false},
		{"reversed", tm(500), tm(100), 0, 0, false},
		{"after the end", tm(2000), nil, 0, 0, false},
	} {
		a, b, ok := TrimWindow(w, c.start, c.end)
		if ok != c.ok || (ok && (a != c.a || b != c.b)) {
			t.Errorf("%s: %d %d %v", c.name, a, b, ok)
		}
	}
	// An instant between samples rounds inward: never a sample of the trimmed part.
	sparse := Normalise(Recording{Records: recs([]int{0, 5, 100, 200}, nil)})
	a, b, ok := TrimWindow(sparse, tm(1), tm(199))
	if !ok || a != 1 || b != 2 {
		t.Errorf("inward: %d %d %v ", a, b, ok)
	}
}

func TestTrimEndMatchesRangeAndDropsTheDrive(t *testing.T) {
	w := trimRun()
	out := Trim(w, 0, 599)
	if len(out.Samples.Offset) != 600 || out.Samples.Offset[599] != 599 {
		t.Fatalf("samples %d", len(out.Samples.Offset))
	}
	s := out.Summary
	wantPtr(t, "distance", s.Distance, 1797, 1e-9)
	wantPtr(t, "elapsed", s.ElapsedTime, 599, 0)
	wantPtr(t, "max hr", s.MaxHeartRate, 140, 0)
	wantPtr(t, "max speed", s.MaxSpeed, 3, 0)
	wantNil(t, "calories", s.Calories)
	wantNil(t, "timer time", s.TimerTime)
	if out.Sport != "running" {
		t.Errorf("sport %q", out.Sport)
	}
	// Equal to the range values of the untrimmed workout.
	rv := newRangeIndex(&w.Samples).values(0, 599)
	wantPtr(t, "range distance", rv.distance, *s.Distance, 1e-9)
	wantPtr(t, "range gain", rv.gain, *s.Ascent, 1e-9)
	if *out.Derived.MovingTime > 599 {
		t.Errorf("moving %v", *out.Derived.MovingTime)
	}
	// Lap 2 is cut: its remaining part, trigger kept, values as a range.
	if len(out.Laps) != 2 || out.Laps[1].Trigger != "button" || out.Laps[1].End != 599 {
		t.Fatalf("laps %+v", out.Laps)
	}
	wantPtr(t, "cut lap distance", out.Laps[1].Totals.Distance, 3*(599-400), 1e-9)
	// The identity.
	if got := Trim(w, 0, 899); len(got.Samples.Offset) != 900 {
		t.Error("whole window is not the identity")
	}
}

func TestTrimStartRebasesEverything(t *testing.T) {
	w := trimRun()
	out := Trim(w, 700, 899)
	if !out.StartTime.Equal(at(700)) || out.Samples.Offset[0] != 0 {
		t.Fatalf("start %v offset %d", out.StartTime, out.Samples.Offset[0])
	}
	if out.Samples.Distance[0] != 0 || out.Samples.Gain[0] != 0 || out.Samples.MovingTime[0] != 0 {
		t.Errorf("not rebased: %v %v %v", out.Samples.Distance[0], out.Samples.Gain[0], out.Samples.MovingTime[0])
	}
	wantPtr(t, "distance", out.Summary.Distance, 199*15, 1e-9)
	// The run lap and the run are gone from the lap list; the cut lap starts at 0, numbered first.
	if len(out.Laps) != 1 || out.Laps[0].Start != 0 || out.Laps[0].Trigger != "button" {
		t.Fatalf("laps %+v", out.Laps)
	}
	// Only the remaining samples' values remain.
	for _, v := range out.Samples.HeartRate {
		if v != 177 {
			t.Fatalf("heart rate %v from the trimmed part", v)
		}
	}
}

func TestTrimDropsWhollyTrimmedLegsAndFollowsSport(t *testing.T) {
	rs := recs(seq(0, 899), func(i int, r *RawRecord) { r.Speed, r.Distance = 3, float64(i)*3 })
	w := Normalise(Recording{
		FileType: "activity",
		Sessions: []RawSession{
			{Sport: "swimming", Start: at(0), End: at(299)},
			{Sport: "cycling", Start: at(300), End: at(599)},
			{Sport: "running", Start: at(600), End: at(899)},
		},
		Records: rs,
	})
	if w.Sport != "multisport" {
		t.Fatalf("sport %q", w.Sport)
	}
	bike := Trim(w, 300, 899)
	if bike.Sport != "multisport" || len(bike.Sessions) != 2 || bike.Sessions[0].Sport != "cycling" || bike.Sessions[0].Start != 0 {
		t.Errorf("bike+run: %q %+v", bike.Sport, bike.Sessions)
	}
	run := Trim(w, 650, 899)
	if run.Sport != "running" || len(run.Sessions) != 1 || run.Sessions[0].Start != 0 || run.Sessions[0].End != 249 {
		t.Errorf("run: %q %+v", run.Sport, run.Sessions)
	}
}

func TestTrimPoolKeepsLengthDropsLengthsAndAbsentStreams(t *testing.T) {
	rs := recs(seq(0, 299), func(i int, r *RawRecord) {
		r.Speed, r.Distance = 1, float64(i)
		if i < 100 {
			r.Power = 200 // only in the part that is trimmed away
		}
	})
	w := Normalise(Recording{
		FileType: "activity",
		Sessions: []RawSession{{Sport: "swimming", SubSport: "lap_swimming", Start: at(0), End: at(299),
			Pool: Pool{Length: 25, Lengths: 12}}},
		Records: rs,
	})
	if w.Summary.Lengths == nil || w.Samples.Power == nil {
		t.Fatal("setup")
	}
	out := Trim(w, 150, 299)
	wantPtr(t, "pool", out.Summary.PoolLengthM, 25, 0)
	wantNil(t, "lengths", out.Summary.Lengths)
	if out.Samples.Power != nil || out.Derived.AvgPower != nil {
		t.Error("a stream without a valid value in the window is present")
	}
}
