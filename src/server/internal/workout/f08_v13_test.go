package workout

import (
	"math"
	"reflect"
	"testing"
)

// Tests of the F-08 v1.3 rule R-13, transition legs (D-21 to D-24, OQ-38).

// transitionRecords returns records 0..n-1 with every stream; speed 8, cadence
// 90, power 200, heart rate 150, temperature 20, and in [from, to] the
// "escape" constants speed 12, cadence 100, power 400, temperature 30.
func transitionRecords(n, from, to int) []RawRecord {
	return recs(seq(0, n-1), func(i int, r *RawRecord) {
		r.Lat, r.Lon = 50+float64(i)*1e-4, 8+float64(i)*1e-4
		r.Altitude, r.Distance = 100+float64(i%7), float64(i)*5
		r.Speed, r.Cadence, r.Power, r.HeartRate, r.Temperature = 8, 90, 200, 150, 20
		if i >= from && i <= to {
			r.Speed, r.Cadence, r.Power, r.Temperature = 12, 100, 400, 30
		}
	})
}

func triSessions() []RawSession {
	big := Totals{AvgSpeed: f64(12), MaxSpeed: f64(12), AvgCadence: f64(100), MaxCadence: f64(100),
		AvgPower: f64(400), MaxPower: f64(400), Distance: f64(150), Ascent: f64(3), Descent: f64(2),
		AvgHeartRate: f64(150), MaxHeartRate: f64(160), AvgTemperature: f64(30), MaxTemperature: f64(30),
		ElapsedTime: f64(30), TimerTime: f64(30), Calories: f64(9)}
	return []RawSession{
		{Sport: "cycling", SubSport: "road", Start: at(0), End: at(29), Totals: Totals{Distance: f64(145), MaxSpeed: f64(9), MaxPower: f64(250)}},
		{Sport: "transition", Start: at(30), End: at(59), Totals: big},
		{Sport: "running", Start: at(60), End: at(89), Totals: Totals{Distance: f64(145), MaxSpeed: f64(5)}},
	}
}

func TestTransitionSamples(t *testing.T) {
	rec := Recording{Records: transitionRecords(90, 30, 59), Sessions: triSessions(),
		Laps: []RawLap{
			{Start: at(0), End: at(44), Totals: Totals{MaxSpeed: f64(9), MaxPower: f64(250)}}, // runs into the transition
			{Start: at(45), End: at(59), Totals: Totals{MaxSpeed: f64(12), AvgCadence: f64(100), Distance: f64(75)}},
		}}
	w := Normalise(rec)
	s := w.Samples
	for i, off := range s.Offset {
		in := off >= 30 && off <= 59
		for name, v := range map[string][]float64{"speed": s.Speed, "cadence": s.Cadence, "power": s.Power} {
			if in != math.IsNaN(v[i]) {
				t.Fatalf("%s at %d: NaN = %v, want %v", name, off, math.IsNaN(v[i]), in)
			}
		}
		for name, v := range map[string][]float64{"lat": s.Lat, "lon": s.Lon, "alt": s.Altitude, "dist": s.Distance, "hr": s.HeartRate, "temp": s.Temperature} {
			if math.IsNaN(v[i]) {
				t.Fatalf("%s at %d dropped", name, off)
			}
		}
	}
	// Cadence is in steps per minute for the run (R-5): 90 -> 180.
	if s.Cadence[70] != 180 {
		t.Errorf("run cadence = %v", s.Cadence[70])
	}

	// Totals of the transition session and of the lap that starts in it.
	tr := w.Sessions[1].Totals
	for _, p := range []*float64{tr.AvgSpeed, tr.MaxSpeed, tr.AvgCadence, tr.MaxCadence, tr.AvgPower, tr.MaxPower} {
		wantNil(t, "transition total", p)
	}
	wantPtr(t, "transition distance", tr.Distance, 150, 0)
	wantPtr(t, "transition ascent", tr.Ascent, 3, 0)
	wantPtr(t, "transition max temperature", tr.MaxTemperature, 30, 0)
	wantPtr(t, "transition calories", tr.Calories, 9, 0)
	wantPtr(t, "transition elapsed", tr.ElapsedTime, 30, 0)
	wantNil(t, "transition lap max speed", w.Laps[1].Totals.MaxSpeed)
	wantNil(t, "transition lap cadence", w.Laps[1].Totals.AvgCadence)
	wantPtr(t, "transition lap distance", w.Laps[1].Totals.Distance, 75, 0)
	// The lap that starts in the cycling session keeps everything.
	wantPtr(t, "lap into transition max speed", w.Laps[0].Totals.MaxSpeed, 9, 0)
	wantPtr(t, "lap into transition max power", w.Laps[0].Totals.MaxPower, 250, 0)
	// Other sessions untouched.
	wantPtr(t, "cycling max speed", w.Sessions[0].Totals.MaxSpeed, 9, 0)

	// Summary and Derived ignore the escape constants.
	wantPtr(t, "summary max speed", w.Summary.MaxSpeed, 9, 0)
	wantPtr(t, "summary max power", w.Summary.MaxPower, 250, 0)
	wantPtr(t, "derived max speed", w.Derived.MaxSpeed, 8, 0)
	wantPtr(t, "derived max power", w.Derived.MaxPower, 200, 0)
	wantPtr(t, "derived max cadence", w.Derived.MaxCadence, 180, 0)
	wantPtr(t, "derived avg power", w.Derived.AvgPower, 200, 1e-9)
	// Heart rate and temperature count in transitions.
	wantPtr(t, "derived max temperature", w.Derived.MaxTemperature, 30, 0)
	wantPtr(t, "summary max temperature", w.Summary.MaxTemperature, 30, 0)
	wantPtr(t, "summary max hr", w.Summary.MaxHeartRate, 160, 0)
	// Distance, calories, timer time include the transition.
	wantPtr(t, "summary distance", w.Summary.Distance, 145+150+145, 0)
	wantPtr(t, "summary calories", w.Summary.Calories, 9, 0)
	wantPtr(t, "summary timer time", w.Summary.TimerTime, 30, 0)
	if w.Sport != "multisport" {
		t.Errorf("sport = %q, want multisport (AC 9)", w.Sport)
	}
	checkInvariants(t, w)
}

func TestTransitionFilledSampleAndAbsentStream(t *testing.T) {
	// A 3 s step inside the transition is filled by R-3; the filled sample is masked.
	offsets := []int{0, 1, 2, 3, 4, 5, 6, 9, 10, 11, 12}
	records := recs(offsets, func(i int, r *RawRecord) {
		r.Lat, r.Lon, r.Distance, r.Speed = 50, 8, float64(i), 3
		if offsets[i] >= 6 && offsets[i] <= 10 {
			r.Cadence = 77 // values only in the transition
		}
	})
	w := Normalise(Recording{Records: records, Sessions: []RawSession{
		{Sport: "cycling", Start: at(0), End: at(5)},
		{Sport: "transition", Start: at(6), End: at(10)},
		{Sport: "cycling", Start: at(11), End: at(12)},
	}})
	if w.Samples.Filled == 0 {
		t.Fatal("no sample was filled")
	}
	for i, off := range w.Samples.Offset {
		if off >= 6 && off <= 10 && !math.IsNaN(w.Samples.Speed[i]) {
			t.Errorf("speed at %d not masked", off)
		}
	}
	if w.Samples.Cadence != nil {
		t.Errorf("cadence stream with values only in a transition must be absent (R-4 after R-13)")
	}
}

func TestTransitionWhichSessions(t *testing.T) {
	speeds := func(w Workout) []float64 { return w.Samples.Speed }
	allKept := func(t *testing.T, name string, w Workout) {
		t.Helper()
		for i, v := range speeds(w) {
			if math.IsNaN(v) {
				t.Errorf("%s: speed masked at %d", name, w.Samples.Offset[i])
				return
			}
		}
	}
	base := func() []RawRecord { return transitionRecords(30, 99, 99) }
	t.Run("sub-sport transition on another sport: not masked", func(t *testing.T) {
		totals := Totals{AvgSpeed: f64(12), MaxSpeed: f64(12), AvgCadence: f64(100), MaxCadence: f64(100), AvgPower: f64(400), MaxPower: f64(400), Distance: f64(150)}
		w := Normalise(Recording{Records: base(), Sessions: []RawSession{{Sport: "generic", SubSport: "transition", Start: at(0), End: at(29), Totals: totals}}})
		allKept(t, "sub-sport", w)
		// D-23: nothing masked, the totals too (QA-08-r3-1, M22).
		if len(w.Sessions) != 1 || !reflect.DeepEqual(w.Sessions[0].Totals, totals) {
			t.Errorf("session totals = %+v, want %+v", w.Sessions[0].Totals, totals)
		}
	})
	t.Run("slow generic session: not masked", func(t *testing.T) {
		recs := recs(seq(0, 29), func(i int, r *RawRecord) { r.Speed = 0.2; r.Distance = float64(i) })
		w := Normalise(Recording{Records: recs, Sessions: []RawSession{{Sport: "generic", Start: at(0), End: at(29)}}})
		allKept(t, "generic", w)
	})
	t.Run("tail with a transition hint only: not masked", func(t *testing.T) {
		w := Normalise(Recording{Records: base(), Sports: []RawSport{{At: at(10), Sport: "transition"}}})
		allKept(t, "hint", w)
	})
	t.Run("tail falling to rule 3 on a transition session: not masked", func(t *testing.T) {
		w := Normalise(Recording{Records: base(), Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(9)},
			{Sport: "transition", Start: at(10), End: at(19)},
		}})
		if got := w.Samples.Speed[25]; math.IsNaN(got) {
			t.Errorf("speed at 25 masked")
		}
		if got := w.Samples.Speed[15]; !math.IsNaN(got) {
			t.Errorf("speed at 15 not masked")
		}
	})
	t.Run("overlap, first in file order not a transition: not masked", func(t *testing.T) {
		w := Normalise(Recording{Records: base(), Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(29)},
			{Sport: "transition", Start: at(10), End: at(19)},
		}})
		allKept(t, "overlap", w)
	})
}

func TestTransitionMovingTime(t *testing.T) {
	// R-13.M, speed source: no timer events. Speed is below the threshold.
	t.Run("speed source", func(t *testing.T) {
		offsets := append(seq(0, 9), 14, 15, 16, 17, 18, 19, 25)
		records := recs(offsets, func(i int, r *RawRecord) { r.Speed = 0.1; r.Distance = float64(i) / 10 })
		records[0].Speed = 5
		w := Normalise(Recording{Records: records, Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(9)},
			{Sport: "transition", Start: at(10), End: at(30)},
		}})
		// Without R-13.M the moving time would be 0. The 5 s step 9 -> 14
		// and the 6 s step 19 -> 25 are gaps (dt >= 5) and do not count;
		// samples 15..19 add 1 s each.
		if w.Derived.MovingTimeSource != movingTimeFromSpeed {
			t.Fatalf("source = %q", w.Derived.MovingTimeSource)
		}
		wantPtr(t, "moving time", w.Derived.MovingTime, 5, 0)
	})
	t.Run("timer stopped during the transition: union", func(t *testing.T) {
		records := transitionRecords(40, 99, 99)
		w := Normalise(Recording{Records: records,
			Timer: []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(20), Kind: TimerStop},
				{At: at(25), Kind: TimerStart}, {At: at(39), Kind: TimerStop}},
			Sessions: []RawSession{
				{Sport: "cycling", Start: at(0), End: at(14)},
				{Sport: "transition", Start: at(15), End: at(29)},
				{Sport: "running", Start: at(30), End: at(39)},
			}})
		// Timer: 20 + 14 = 34. Transition intervals (14,29] = 15 s, of which
		// (14,20] and (25,29] overlap the timer; the stopped part adds 5 s.
		wantPtr(t, "moving time", w.Derived.MovingTime, 39, 0)
	})
	t.Run("timer source: a gap inside the transition does not count (QA-08-r3-1, M9)", func(t *testing.T) {
		records := recs(append(seq(0, 19), seq(30, 39)...), func(i int, r *RawRecord) { r.Speed = 0.1; r.Distance = float64(i) / 10 })
		w := Normalise(Recording{Records: records,
			Timer: []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(14), Kind: TimerStop}},
			Sessions: []RawSession{
				{Sport: "cycling", Start: at(0), End: at(14)},
				{Sport: "transition", Start: at(15), End: at(39)},
			}})
		// Timer 14 s, plus (14,19] = 5 s; the step 19 -> 30 is a gap (11 s)
		// and adds nothing; 31..39 add 9 s.
		if w.Derived.MovingTimeSource != movingTimeFromTimer {
			t.Fatalf("source = %q", w.Derived.MovingTimeSource)
		}
		wantPtr(t, "moving time", w.Derived.MovingTime, 28, 0)
	})
	t.Run("timer running throughout: unchanged", func(t *testing.T) {
		records := transitionRecords(40, 99, 99)
		w := Normalise(Recording{Records: records,
			Timer: []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(39), Kind: TimerStop}},
			Sessions: []RawSession{
				{Sport: "cycling", Start: at(0), End: at(14)},
				{Sport: "transition", Start: at(15), End: at(29)},
			}})
		wantPtr(t, "moving time", w.Derived.MovingTime, 39, 0)
	})
}

func TestTransitionAllowListComplete(t *testing.T) {
	// Streams: every entry of the table is decided, and every field of
	// Samples is either a stream (whatever its type) or one of the two
	// named non-stream fields.
	for k, rule := range transitionStreams {
		if rule == ruleUndecided {
			t.Errorf("stream %d is in neither column of the R-13 allow-list", k)
		}
	}
	var s Samples
	isStream := map[uintptr]bool{}
	for _, p := range s.streams() {
		isStream[reflect.ValueOf(p).Pointer()] = true
	}
	v := reflect.ValueOf(&s).Elem()
	matched := 0
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		switch {
		case name == "Offset" || name == "Filled" || name == "inserted" || name == "MovingTime" || name == "Gain":
			// the cumulative series of F-17 B-1 (ADR-029) are derived after R-13
		case isStream[v.Field(i).Addr().Pointer()]:
			matched++
		default:
			t.Errorf("Samples.%s is neither a stream in streams() nor Offset/Filled: decide it for R-13", name)
		}
	}
	if matched != numStreams {
		t.Errorf("streams() lists %d streams but Samples has %d stream fields", numStreams, matched)
	}

	// Totals: reflection, there is no enumerator.
	ty := reflect.TypeOf(Totals{})
	seen := map[string]bool{}
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		seen[name] = true
		if _, ok := transitionTotalKept[name]; !ok {
			t.Errorf("Totals.%s is in neither column of the R-13 allow-list", name)
		}
	}
	for name := range transitionTotalKept {
		if !seen[name] {
			t.Errorf("allow-list names Totals.%s, which does not exist", name)
		}
	}
}

func TestTransitionPurity(t *testing.T) {
	rec := Recording{Records: transitionRecords(90, 30, 59), Sessions: triSessions()}
	before := Recording{Records: append([]RawRecord(nil), rec.Records...), Sessions: append([]RawSession(nil), rec.Sessions...)}
	Normalise(rec)
	if !reflect.DeepEqual(sessionTotals(rec), sessionTotals(before)) {
		t.Error("input session totals changed")
	}
	if !reflect.DeepEqual(rec.Records[40], before.Records[40]) {
		t.Error("input records changed")
	}
}

func sessionTotals(r Recording) []Totals {
	var out []Totals
	for _, s := range r.Sessions {
		out = append(out, s.Totals)
	}
	return out
}
