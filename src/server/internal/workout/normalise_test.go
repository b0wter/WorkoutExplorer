package workout

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

// ---- R-1, R-2 time axis --------------------------------------------------

func TestTimeAxisSortsRecords(t *testing.T) {
	w := Normalise(Recording{Records: recs([]int{3, 1, 2, 0}, func(i int, r *RawRecord) {
		r.HeartRate = float64(100 + i)
	})})
	if got, want := w.Samples.Offset, []int32{0, 1, 2, 3}; !equalInt32(got, want) {
		t.Fatalf("Offset = %v, want %v", got, want)
	}
	if got, want := w.Samples.HeartRate, []float64{103, 101, 102, 100}; !sameFloats(got, want) {
		t.Errorf("HeartRate = %v, want %v", got, want)
	}
	if !w.StartTime.Equal(at(0)) {
		t.Errorf("StartTime = %v, want %v", w.StartTime, at(0))
	}
}

func TestTimeAxisMergesSameSecond(t *testing.T) {
	first, second := rec(5), rec(5)
	first.HeartRate, first.Power = 100, 200
	second.HeartRate = 110 // Power missing: the earlier value fills it
	w := Normalise(Recording{Records: []RawRecord{rec(4), first, second}})
	if got, want := w.Samples.Offset, []int32{0, 1}; !equalInt32(got, want) {
		t.Fatalf("Offset = %v, want %v", got, want)
	}
	if got := w.Samples.HeartRate[1]; got != 110 {
		t.Errorf("HeartRate = %v, want 110 (later valid value wins)", got)
	}
	if got := w.Samples.Power[1]; got != 200 {
		t.Errorf("Power = %v, want 200 (earlier value fills NaN)", got)
	}
}

func TestStartTimeIsEarliest(t *testing.T) {
	tests := []struct {
		name               string
		session, lap, data int
		want               int
	}{
		{"session earliest", -10, -5, 0, -10},
		{"lap earliest", 0, -5, 3, -5},
		{"record earliest", 2, 1, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(Recording{
				Sessions: []RawSession{{Sport: "cycling", Start: at(tt.session), End: at(100)}},
				Laps:     []RawLap{{Start: at(tt.lap), End: at(100)}},
				Records:  recs([]int{tt.data, tt.data + 1}, func(_ int, r *RawRecord) { r.HeartRate = 120 }),
			})
			if !w.StartTime.Equal(at(tt.want)) {
				t.Errorf("StartTime = %v, want %v", w.StartTime, at(tt.want))
			}
			if w.Samples.Offset[0] < 0 || w.Sessions[0].Start < 0 || w.Laps[0].Start < 0 {
				t.Errorf("negative offset: samples %v, session %d, lap %d", w.Samples.Offset, w.Sessions[0].Start, w.Laps[0].Start)
			}
			if got, want := w.Samples.Offset[0], int32(tt.data-tt.want); got != want {
				t.Errorf("first sample offset = %d, want %d", got, want)
			}
		})
	}
}

func TestTimerEventBeforeStartGetsZero(t *testing.T) {
	w := Normalise(Recording{
		Timer:   []RawTimerEvent{{At: at(-30), Kind: TimerStart}, {At: at(10), Kind: TimerStop}},
		Records: recs(seq(0, 10), nil),
	})
	if got := w.Timer[0].Offset; got != 0 {
		t.Errorf("timer offset = %d, want 0", got)
	}
	if got := w.Timer[1].Offset; got != 10 {
		t.Errorf("timer offset = %d, want 10", got)
	}
}

// ---- R-3 gap filling -----------------------------------------------------

func TestGapFilling(t *testing.T) {
	tests := []struct {
		name        string
		offsets     []int
		wantOffsets []int32
		wantFilled  int
	}{
		{"1 s steps untouched", []int{0, 1, 2}, []int32{0, 1, 2}, 0},
		{"gap 3 s filled", []int{0, 3}, []int32{0, 1, 2, 3}, 2},
		{"gap 4 s filled", []int{0, 4}, []int32{0, 1, 2, 3, 4}, 3},
		{"gap 5 s kept", []int{0, 5}, []int32{0, 5}, 0},
		{"gap 30 s kept", []int{0, 30}, []int32{0, 30}, 0},
		{"mixed", []int{0, 2, 7, 10, 40}, []int32{0, 1, 2, 7, 8, 9, 10, 40}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(Recording{Records: recs(tt.offsets, func(i int, r *RawRecord) { r.HeartRate = 100 })})
			if !equalInt32(w.Samples.Offset, tt.wantOffsets) {
				t.Errorf("Offset = %v, want %v", w.Samples.Offset, tt.wantOffsets)
			}
			if w.Samples.Filled != tt.wantFilled {
				t.Errorf("Filled = %d, want %d", w.Samples.Filled, tt.wantFilled)
			}
			checkInvariants(t, w)
		})
	}
}

func TestGapFillingInterpolates(t *testing.T) {
	a, b := rec(0), rec(3)
	a.HeartRate, b.HeartRate = 100, 130
	a.Lat, a.Lon, b.Lat, b.Lon = 47, 8, 47.3, 8.6
	a.Power = 250 // b.Power missing: inserted samples get NaN in power only
	b.Cadence = 90
	w := Normalise(Recording{Records: []RawRecord{a, b}})
	s := w.Samples
	if want := []float64{100, 110, 120, 130}; !sameFloats(s.HeartRate, want) {
		t.Errorf("HeartRate = %v, want %v", s.HeartRate, want)
	}
	if want := []float64{47, 47.1, 47.2, 47.3}; !sameFloats(s.Lat, want) {
		t.Errorf("Lat = %v, want %v", s.Lat, want)
	}
	if want := []float64{8, 8.2, 8.4, 8.6}; !sameFloats(s.Lon, want) {
		t.Errorf("Lon = %v, want %v", s.Lon, want)
	}
	if want := []float64{250, nan, nan, nan}; !sameFloats(s.Power, want) {
		t.Errorf("Power = %v, want %v", s.Power, want)
	}
	if want := []float64{nan, nan, nan, 90}; !sameFloats(s.Cadence, want) {
		t.Errorf("Cadence = %v, want %v", s.Cadence, want)
	}
	if s.Filled != 2 {
		t.Errorf("Filled = %d, want 2", s.Filled)
	}
}

// ---- R-4 presence and invariants ------------------------------------------

func TestStreamPresence(t *testing.T) {
	w := Normalise(Recording{Records: recs([]int{0, 1, 2}, func(i int, r *RawRecord) {
		r.HeartRate = 120
		r.Lat, r.Lon = 47, 8
		if i == 1 {
			r.Lon = nan // half a position: both missing
		}
	})})
	if w.Samples.Power != nil || w.Samples.Altitude != nil || w.Samples.Speed != nil {
		t.Errorf("streams with only NaN must be nil")
	}
	if !math.IsNaN(w.Samples.Lat[1]) || !math.IsNaN(w.Samples.Lon[1]) {
		t.Errorf("position 1 = (%v, %v), want both NaN", w.Samples.Lat[1], w.Samples.Lon[1])
	}
	checkInvariants(t, w)
}

func TestInvariants(t *testing.T) {
	tests := map[string]Recording{
		"empty":            {},
		"sessions only":    {Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(10)}}},
		"timer only":       {Timer: []RawTimerEvent{{At: at(0), Kind: TimerStart}}},
		"zero times":       {Sessions: []RawSession{{}}, Laps: []RawLap{{}}, Timer: []RawTimerEvent{{}}, Records: []RawRecord{{}}},
		"one record":       {Records: recs([]int{0}, func(_ int, r *RawRecord) { r.HeartRate = 1 })},
		"infinite values":  {Records: recs([]int{0, 1}, func(i int, r *RawRecord) { r.Speed = math.Inf(1); r.HeartRate = 100 })},
		"only lat":         {Records: recs([]int{0, 1}, func(_ int, r *RawRecord) { r.Lat = 47 })},
		"duplicates, gaps": {Records: recs([]int{0, 0, 2, 2, 9, 12, 12}, func(i int, r *RawRecord) { r.Altitude = float64(i) })},
		"unknown timer":    {Timer: []RawTimerEvent{{At: at(0), Kind: TimerKind(9)}}, Records: recs([]int{0, 1}, nil)},
		"end before start": {Sessions: []RawSession{{Start: at(10), End: at(5)}}, Records: recs([]int{0}, nil)},
	}
	for name, r := range tests {
		t.Run(name, func(t *testing.T) {
			w := Normalise(r)
			checkInvariants(t, w)
		})
	}

	w := Normalise(Recording{})
	if len(w.Samples.Offset) != 0 || w.Samples.Lat != nil || w.Samples.HeartRate != nil {
		t.Errorf("empty recording has samples or streams: %+v", w.Samples)
	}
	if w.Derived != (Derived{}) {
		t.Errorf("empty recording: Derived = %+v, want all nil", w.Derived)
	}
	if !w.StartTime.IsZero() {
		t.Errorf("empty recording: StartTime = %v, want zero", w.StartTime)
	}
}

// ---- R-5 cadence ------------------------------------------------------------

func TestCadenceBySport(t *testing.T) {
	sessions := []RawSession{
		{Sport: "running", Start: at(0), End: at(9), Totals: Totals{AvgCadence: f64(85.5), MaxCadence: f64(90)}},
		{Sport: "cycling", Start: at(20), End: at(29), Totals: Totals{AvgCadence: f64(90), MaxCadence: f64(110)}},
	}
	laps := []RawLap{
		{Start: at(0), End: at(9), Totals: Totals{AvgCadence: f64(85.5)}},
		{Start: at(20), End: at(29), Totals: Totals{AvgCadence: f64(90)}},
	}
	// Samples at 0 (running), 15 (between: last session started before it = running),
	// 25 (cycling), 40 (after both: cycling).
	records := recs([]int{0, 15, 25, 40}, func(i int, r *RawRecord) {
		r.Cadence = []float64{85 + 64.0/128, 80, 90, 90}[i]
	})
	w := Normalise(Recording{Sessions: sessions, Laps: laps, Records: records})
	if want := []float64{171, 160, 90, 90}; !sameFloats(w.Samples.Cadence, want) {
		t.Errorf("Cadence = %v, want %v", w.Samples.Cadence, want)
	}
	wantPtr(t, "running session avg", w.Sessions[0].Totals.AvgCadence, 171, 0)
	wantPtr(t, "running session max", w.Sessions[0].Totals.MaxCadence, 180, 0)
	wantPtr(t, "cycling session avg", w.Sessions[1].Totals.AvgCadence, 90, 0)
	wantPtr(t, "cycling session max", w.Sessions[1].Totals.MaxCadence, 110, 0)
	wantPtr(t, "running lap avg", w.Laps[0].Totals.AvgCadence, 171, 0)
	wantPtr(t, "cycling lap avg", w.Laps[1].Totals.AvgCadence, 90, 0)
	if *sessions[0].Totals.AvgCadence != 85.5 {
		t.Errorf("Normalise changed its input")
	}
}

// sessionSports returns sessions named s0, s1, ... so a lookup shows which one it found.
func sessionSports(spans ...[2]int32) []Session {
	var out []Session
	for i, sp := range spans {
		out = append(out, Session{Sport: "s" + string(rune('0'+i)), Start: sp[0], End: sp[1]})
	}
	return out
}

func TestSessionRules(t *testing.T) {
	tl := newSportTimeline(sessionSports([2]int32{10, 20}, [2]int32{30, 40}), nil)
	tests := []struct {
		off  int32
		want string
	}{{5, "s0"}, {10, "s0"}, {20, "s0"}, {25, "s0"}, {30, "s1"}, {50, "s1"}}
	for _, tt := range tests {
		if got := tl.at(tt.off); got != tt.want {
			t.Errorf("at(%d) = %q, want %q", tt.off, got, tt.want)
		}
	}
	if got := newSportTimeline(nil, nil).at(3); got != "" {
		t.Errorf("at without sessions = %q, want empty", got)
	}
}

func TestSessionRulesEqualStarts(t *testing.T) {
	// E-5: outside every session, equal starts resolve to the first in file order.
	tl := newSportTimeline(sessionSports([2]int32{10, 20}, [2]int32{10, 15}, [2]int32{0, 5}), nil)
	if got := tl.at(30); got != "s0" {
		t.Errorf("at(30) = %q, want s0", got)
	}
	if got := tl.at(7); got != "s2" {
		t.Errorf("at(7) = %q, want s2 (only session started before)", got)
	}
}

func TestEmptySportUsesDefaults(t *testing.T) {
	// E-1: a "" sport is "every other sport": no cadence doubling, 0.5 km/h threshold.
	w := Normalise(Recording{
		Sessions: []RawSession{{Sport: "", Start: at(0), End: at(10)}},
		Records:  recs([]int{0, 1, 2}, func(i int, r *RawRecord) { r.Cadence = 80; r.Speed = []float64{1, 0.2, 0.1}[i] }),
	})
	if w.Sport != "" || w.Samples.Cadence[0] != 80 {
		t.Errorf("Sport %q, cadence %v", w.Sport, w.Samples.Cadence[0])
	}
	wantPtr(t, "MovingTime", w.Derived.MovingTime, 1, 0)
}

func TestSpanOffsets(t *testing.T) {
	// E-2 in workout: missing start → 0; missing or earlier end → start; negative → 0.
	tests := []struct {
		name         string
		from, to     time.Time
		wantS, wantE int32
	}{
		{"both", at(5), at(15), 5, 15},
		{"start missing", time.Time{}, at(15), 0, 15},
		{"all missing", time.Time{}, time.Time{}, 0, 0},
		{"end missing", at(5), time.Time{}, 5, 5},
		{"end before start", at(5), at(2), 5, 5},
		{"before workout start", at(-10), at(-5), 0, 0},
	}
	for _, tt := range tests {
		s, e := spanOffsets(at(0), tt.from, tt.to)
		if s != tt.wantS || e != tt.wantE {
			t.Errorf("%s: [%d, %d], want [%d, %d]", tt.name, s, e, tt.wantS, tt.wantE)
		}
	}
}

func TestCadenceWithoutSessionsUnchanged(t *testing.T) {
	w := Normalise(Recording{Records: recs([]int{0}, func(_ int, r *RawRecord) { r.Cadence = 85 })})
	if w.Samples.Cadence[0] != 85 {
		t.Errorf("Cadence = %v, want 85", w.Samples.Cadence[0])
	}
}

// ---- R-6 distance -------------------------------------------------------------

func TestHaversine(t *testing.T) {
	if got := haversine(0, 0, 1, 0); !near(got, 111195.08, 0.01) {
		t.Errorf("haversine((0,0),(1,0)) = %.4f, want 111195.08 ± 0.01", got)
	}
	if got := haversine(47, 8, 47, 8); got != 0 {
		t.Errorf("haversine of one point = %v, want 0", got)
	}
}

func TestComputedDistance(t *testing.T) {
	// No fix at 0; fixes at 1 and 2; dropout at 3 and 4; fix at 5.
	pos := map[int][2]float64{1: {0, 0}, 2: {0.001, 0}, 5: {0.003, 0}}
	w := Normalise(Recording{Records: recs(seq(0, 5), func(i int, r *RawRecord) {
		if p, ok := pos[i]; ok {
			r.Lat, r.Lon = p[0], p[1]
		}
	})})
	d1 := haversine(0, 0, 0.001, 0)
	d2 := haversine(0.001, 0, 0.003, 0)
	want := []float64{0, 0, d1, d1, d1, d1 + d2}
	if !sameFloats(w.Samples.Distance, want) {
		t.Errorf("Distance = %v, want %v", w.Samples.Distance, want)
	}
	wantPtr(t, "Derived.Distance", w.Derived.Distance, d1+d2, 1e-9)
}

func TestDeviceDistanceKept(t *testing.T) {
	w := Normalise(Recording{Records: recs(seq(0, 2), func(i int, r *RawRecord) {
		r.Lat, r.Lon = 47+float64(i)*0.01, 8
		if i != 1 {
			r.Distance = float64(i) * 1000
		}
	})})
	if want := []float64{0, nan, 2000}; !sameFloats(w.Samples.Distance, want) {
		t.Errorf("Distance = %v, want %v", w.Samples.Distance, want)
	}
	wantPtr(t, "Derived.Distance", w.Derived.Distance, 2000, 0)
}

func TestNoDistanceNoPositions(t *testing.T) {
	w := Normalise(Recording{Records: recs(seq(0, 5), func(_ int, r *RawRecord) { r.HeartRate = 120 })})
	if w.Samples.Distance != nil {
		t.Errorf("Distance = %v, want nil (not zeros)", w.Samples.Distance)
	}
	wantNil(t, "Derived.Distance", w.Derived.Distance)
	wantNil(t, "Summary.Distance", w.Summary.Distance)
}

// ---- R-7 moving time ------------------------------------------------------------

func TestMovingTimeFromTimer(t *testing.T) {
	ev := func(s int, k TimerKind) RawTimerEvent { return RawTimerEvent{At: at(s), Kind: k} }
	tests := []struct {
		name  string
		timer []RawTimerEvent
		last  int
		want  float64
	}{
		{"start stop start stop", []RawTimerEvent{ev(0, TimerStart), ev(600, TimerStop), ev(1200, TimerStart), ev(1800, TimerStop)}, 1800, 1200},
		{"stop first", []RawTimerEvent{ev(300, TimerStop), ev(400, TimerStart), ev(500, TimerStop)}, 600, 400},
		{"open at end", []RawTimerEvent{ev(0, TimerStart), ev(100, TimerStop), ev(200, TimerStart)}, 500, 400},
		{"open after last sample", []RawTimerEvent{ev(0, TimerStart), ev(100, TimerStop), ev(800, TimerStart)}, 500, 100},
		{"repeated start", []RawTimerEvent{ev(0, TimerStart), ev(50, TimerStart), ev(100, TimerStop), ev(150, TimerStop)}, 200, 100},
		{"unsorted", []RawTimerEvent{ev(600, TimerStop), ev(0, TimerStart)}, 600, 600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(Recording{Timer: tt.timer, Records: recs([]int{0, tt.last}, func(_ int, r *RawRecord) { r.Speed = 0 })})
			wantPtr(t, "MovingTime", w.Derived.MovingTime, tt.want, 0)
			if w.Derived.MovingTimeSource != "timer" {
				t.Errorf("source = %q, want timer", w.Derived.MovingTimeSource)
			}
		})
	}
}

func TestMovingTimeFromSpeed(t *testing.T) {
	speeds := func(v ...float64) func(int, *RawRecord) {
		return func(i int, r *RawRecord) { r.Speed = v[i] }
	}
	tests := []struct {
		name    string
		sport   string
		offsets []int
		set     func(int, *RawRecord)
		want    float64
	}{
		// 0.2 m/s = 0.72 km/h: below 1 km/h (cycling), above 0.5 km/h (running).
		{"cycling below threshold", "cycling", []int{0, 1, 2, 3}, speeds(5, 0.2, 0.2, 5), 1},
		{"cycling above threshold", "cycling", []int{0, 1, 2}, speeds(5, 0.3, 0.3), 2},
		{"e-biking threshold", "e_biking", []int{0, 1, 2}, speeds(5, 0.2, 0.3), 1},
		{"running threshold", "running", []int{0, 1, 2, 3}, speeds(3, 0.2, 0.1, 3), 2},
		{"other sport uses 0.5 km/h", "rowing", []int{0, 1, 2}, speeds(3, 0.2, 0.1), 1},
		{"NaN speed not moving", "cycling", []int{0, 1, 2}, speeds(5, nan, 5), 1},
		{"gap of 5 s not counted", "cycling", []int{0, 1, 6, 7}, speeds(5, 5, 5, 5), 2},
		{"gap of 4 s filled and counted", "cycling", []int{0, 4}, speeds(5, 5), 4},
		{"from distance", "cycling", []int{0, 1, 2, 3}, func(i int, r *RawRecord) {
			r.Distance = []float64{0, 5, 5.1, 10}[i] // 5, 0.1, 4.9 m/s
		}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(Recording{
				Sessions: []RawSession{{Sport: tt.sport, Start: at(0), End: at(100)}},
				Records:  recs(tt.offsets, tt.set),
			})
			wantPtr(t, "MovingTime", w.Derived.MovingTime, tt.want, 0)
			if w.Derived.MovingTimeSource != "speed" {
				t.Errorf("source = %q, want speed", w.Derived.MovingTimeSource)
			}
		})
	}
}

// TestMovingTimeThresholds pins the R-7 thresholds (QA-02-r1-2): just below
// and just above each, and exactly at it (counted, >=).
func TestMovingTimeThresholds(t *testing.T) {
	kmh := func(v float64) float64 { return v / 3.6 }
	tests := []struct {
		sport string
		speed float64 // m/s
		want  float64
	}{
		{"cycling", kmh(0.99), 0},
		{"cycling", kmh(1.01), 1},
		{"cycling", 1.0 / 3.6, 1},
		{"e_biking", kmh(0.99), 0},
		{"e_biking", kmh(1.01), 1},
		{"e_biking", 1.0 / 3.6, 1},
		{"running", kmh(0.49), 0},
		{"running", kmh(0.51), 1},
		{"running", 0.5 / 3.6, 1},
		{"walking", kmh(0.49), 0},
		{"walking", kmh(0.51), 1},
		{"walking", 0.5 / 3.6, 1},
		{"", kmh(0.49), 0},
		{"", kmh(0.51), 1},
		{"", 0.5 / 3.6, 1},
	}
	for _, tt := range tests {
		w := Normalise(Recording{
			Sessions: []RawSession{{Sport: tt.sport, Start: at(0), End: at(10)}},
			Records:  recs([]int{0, 1}, func(i int, r *RawRecord) { r.Speed = tt.speed }),
		})
		if got := w.Derived.MovingTime; got == nil || *got != tt.want {
			t.Errorf("sport %q at %.4f km/h: moving time %v, want %v", tt.sport, tt.speed*3.6, deref(got), tt.want)
		}
	}
}

func TestMovingTimeFromComputedDistance(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{{Sport: "running", Start: at(0), End: at(10)}},
		Records: recs(seq(0, 3), func(i int, r *RawRecord) {
			r.Lat, r.Lon = 0, 0.00003*float64([]int{1, 2, 2, 3}[i]) // 3.3 m steps, then a stop
		}),
	})
	wantPtr(t, "MovingTime", w.Derived.MovingTime, 2, 0)
}

func TestMovingTimeWithoutSource(t *testing.T) {
	w := Normalise(Recording{Records: recs(seq(0, 5), func(_ int, r *RawRecord) { r.HeartRate = 120 })})
	wantNil(t, "MovingTime", w.Derived.MovingTime)
	if w.Derived.MovingTimeSource != "" {
		t.Errorf("source = %q, want empty", w.Derived.MovingTimeSource)
	}
	wantNil(t, "Summary.MovingTime", w.Summary.MovingTime)
}

// ---- R-8 elevation gain -----------------------------------------------------------

func altitudes(f func(s int) float64, n int) []RawRecord {
	return recs(seq(0, n-1), func(i int, r *RawRecord) { r.Altitude = f(i) })
}

// climbShape is the altitude of climb-100m.fit: 60 s flat, 100 m over 1,000 s, 60 s flat.
func climbShape(s int) float64 {
	switch {
	case s < 60:
		return 100
	case s <= 1060:
		return 100 + float64(s-60)/10
	}
	return 200
}

func TestElevationGain(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 42))
	tests := []struct {
		name    string
		records []RawRecord
		want    float64
		eps     float64
	}{
		{"flat with ±1 m noise", altitudes(func(int) float64 { return 100 + rng.Float64()*2 - 1 }, 3600), 0, 0},
		{"clean 100 m climb", altitudes(climbShape, 1120), 100, 2},
		{"climb 50, descend 50, climb 50", altitudes(func(s int) float64 {
			switch {
			case s <= 500:
				return float64(s) / 10
			case s <= 1000:
				return 50 - float64(s-500)/10
			}
			return float64(s-1000) / 10
		}, 1501), 100, 2},
		{"NaN altitudes skipped", recs(seq(0, 1119), func(i int, r *RawRecord) {
			if i%7 == 3 {
				r.Altitude = nan
			} else {
				r.Altitude = climbShape(i)
			}
		}), 100, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(Recording{Records: tt.records})
			wantPtr(t, "ElevationGain", w.Derived.ElevationGain, tt.want, tt.eps)
		})
	}
}

// rollingHills is the altitude of rolling-hills.fit without noise: 100 m base;
// each bump is a 20 s rise, 10 s flat top, 20 s fall and 30 s at the base;
// 20 bumps of 2.2 m, then 10 bumps of 6 m.
func rollingHills(s int) float64 {
	bump, phase := s/80, s%80
	if bump >= 30 {
		return 100
	}
	h := 2.2
	if bump >= 20 {
		h = 6
	}
	switch {
	case phase < 20:
		return 100 + h*float64(phase)/20
	case phase < 30:
		return 100 + h
	case phase < 50:
		return 100 + h*float64(50-phase)/20
	}
	return 100
}

func TestElevationGainRollingHills(t *testing.T) {
	// Same shape as rolling-hills.fit; elevationGainWith applies the production
	// smoothing and takes H as a parameter (F-02 design v1.2, R-8 tests).
	n := 30*80 + 1
	offsets := make([]int32, n)
	clean := make([]float64, n)
	for i := range offsets {
		offsets[i] = int32(i)
		clean[i] = rollingHills(i)
	}
	wantPtr(t, "noise-free, H = 3", elevationGainWith(offsets, clean, hysteresisMetres), 60, 0.01)
	if g := elevationGainWith(offsets, clean, 2); g == nil || *g <= 100 {
		t.Errorf("noise-free, H = 2 = %v, want > 100", deref(g))
	}

	for seed := uint64(1); seed <= 10; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed))
		noisy := make([]float64, n)
		for i := range noisy {
			noisy[i] = clean[i] + rng.Float64() - 0.5 // uniform ±0.5 m
		}
		if g := elevationGainWith(offsets, noisy, hysteresisMetres); g == nil || *g < 60 || *g > 66 {
			t.Errorf("seed %d, H = 3 = %v, want 60 ≤ gain ≤ 66", seed, deref(g))
		}
		if g := elevationGainWith(offsets, noisy, 2); g == nil || *g <= 100 {
			t.Errorf("seed %d, H = 2 = %v, want > 100", seed, deref(g))
		}
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestElevationGainNeedsTwoAltitudes(t *testing.T) {
	w := Normalise(Recording{Records: recs([]int{0, 1}, func(i int, r *RawRecord) {
		if i == 0 {
			r.Altitude = 100
		}
		r.HeartRate = 120
	})})
	wantNil(t, "ElevationGain", w.Derived.ElevationGain)
	wantNil(t, "Summary.Ascent", w.Summary.Ascent)
}

// ---- R-9 time zone and sport ----------------------------------------------------------

func TestTimeZone(t *testing.T) {
	i64 := func(v int64) *int64 { return &v }
	tests := []struct {
		name  string
		local *int64
		want  TimeZone
	}{
		{"exact", i64(7200), TimeZone{Known: true, OffsetSeconds: 7200}},
		{"rounded to 15 min", i64(7190), TimeZone{Known: true, OffsetSeconds: 7200}},
		{"negative", i64(-14390), TimeZone{Known: true, OffsetSeconds: -14400}},
		{"India", i64(19800), TimeZone{Known: true, OffsetSeconds: 19800}},
		{"Nepal +05:45", i64(20700), TimeZone{Known: true, OffsetSeconds: 20700}},
		{"20,690 rounds to +05:45", i64(20690), TimeZone{Known: true, OffsetSeconds: 20700}},
		{"20,500 rounds to +05:45", i64(20500), TimeZone{Known: true, OffsetSeconds: 20700}},
		{"Chatham +12:45", i64(45900), TimeZone{Known: true, OffsetSeconds: 45900}},
		{"upper bound", i64(14 * 3600), TimeZone{Known: true, OffsetSeconds: 14 * 3600}},
		{"lower bound", i64(-12 * 3600), TimeZone{Known: true, OffsetSeconds: -12 * 3600}},
		{"20 h unknown", i64(20 * 3600), TimeZone{}},
		{"-13 h unknown", i64(-13 * 3600), TimeZone{}},
		{"nil unknown", nil, TimeZone{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalise(Recording{LocalOffset: tt.local}).TimeZone; got != tt.want {
				t.Errorf("TimeZone = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSport(t *testing.T) {
	s := func(sport string) RawSession { return RawSession{Sport: sport, Start: at(0), End: at(1)} }
	tests := []struct {
		sessions []RawSession
		want     string
	}{
		{nil, ""},
		{[]RawSession{s("running")}, "running"},
		{[]RawSession{s("running"), s("cycling")}, "multisport"},
		{[]RawSession{s("swimming"), s("cycling"), s("running")}, "multisport"},
	}
	for _, tt := range tests {
		if got := Normalise(Recording{Sessions: tt.sessions}).Sport; got != tt.want {
			t.Errorf("%d sessions: Sport = %q, want %q", len(tt.sessions), got, tt.want)
		}
	}
}

// ---- R-10 summary ---------------------------------------------------------------------

func TestSummaryOneSession(t *testing.T) {
	records := recs(seq(0, 100), func(i int, r *RawRecord) {
		r.Distance = float64(i) * 5
		r.Speed = 5
		r.Altitude = float64(i) // 100 m gain
	})
	device := Totals{ElapsedTime: f64(3600), Distance: f64(12345), Ascent: f64(77), AvgHeartRate: f64(140)}
	w := Normalise(Recording{
		Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(100), Totals: device}},
		Timer:    []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(50), Kind: TimerStop}},
		Records:  records,
	})
	wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 3600, 0)
	wantPtr(t, "Distance", w.Summary.Distance, 12345, 0)
	wantPtr(t, "Ascent", w.Summary.Ascent, 77, 0)
	wantPtr(t, "AvgHeartRate", w.Summary.AvgHeartRate, 140, 0)
	wantPtr(t, "MovingTime (always derived)", w.Summary.MovingTime, 50, 0)
	wantNil(t, "AvgPower (not recorded, not computed)", w.Summary.AvgPower)
	wantNil(t, "TimerTime (not recorded, not computed)", w.Summary.TimerTime)

	// Without device values: fallbacks for elapsed, distance and ascent only.
	w = Normalise(Recording{
		Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(100)}},
		Records:  records,
	})
	wantPtr(t, "ElapsedTime fallback", w.Summary.ElapsedTime, 100, 0)
	wantPtr(t, "Distance fallback", w.Summary.Distance, 500, 0)
	wantPtr(t, "Ascent fallback", w.Summary.Ascent, 100, 2)
	wantPtr(t, "MovingTime", w.Summary.MovingTime, 100, 0)
	wantPtr(t, "AvgSpeed (R-11 fallback: distance / moving time)", w.Summary.AvgSpeed, 5, 1e-9)
}

func TestSummaryTwoSessions(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{
			{Sport: "running", Start: at(0), End: at(10), Totals: Totals{Distance: f64(30), AvgHeartRate: f64(150)}},
			{Sport: "cycling", Start: at(10), End: at(20), Totals: Totals{Distance: f64(80), AvgHeartRate: f64(140)}},
		},
		Records: recs(seq(0, 20), func(i int, r *RawRecord) { r.Distance = float64(i) * 5; r.Speed = 5 }),
	})
	wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 20, 0)
	wantPtr(t, "Distance (summed over sessions)", w.Summary.Distance, 110, 0)
	wantPtr(t, "MovingTime", w.Summary.MovingTime, 20, 0)
	wantNil(t, "AvgHeartRate", w.Summary.AvgHeartRate)
	wantNil(t, "Ascent", w.Summary.Ascent)
	if len(w.Sessions) != 2 || w.Sessions[0].Sport != "running" || w.Sessions[1].Sport != "cycling" {
		t.Errorf("sessions not kept in order: %+v", w.Sessions)
	}
}

func TestSpanEndMissing(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{{Sport: "cycling", Start: at(0)}},
		Records:  recs([]int{0, 10}, nil),
	})
	if s := w.Sessions[0]; s.Start != 0 || s.End != 0 {
		t.Errorf("session span = [%d, %d], want [0, 0]", s.Start, s.End)
	}
}

func equalInt32(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
