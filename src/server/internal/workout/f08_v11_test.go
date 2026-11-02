package workout

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"
)

// Tests of the F-08 v1.1 rules: R-12, the sport timeline, R-9 sport, the
// record leg and R-10.E (QA-08-r1-1 to r1-3, r1-5).

// ---- R-12 distance across sequences ---------------------------------------------

func distRecs(offsets []int, dist []float64) []RawRecord {
	return recs(offsets, func(i int, r *RawRecord) { r.Distance = dist[i] })
}

func distancesOf(t *testing.T, records []RawRecord, starts []int) []float64 {
	t.Helper()
	w := Normalise(Recording{Records: records, SequenceStarts: starts})
	checkInvariants(t, w)
	return w.Samples.Distance
}

func TestContinueDistance(t *testing.T) {
	tests := []struct {
		name   string
		dist   []float64
		starts []int
		want   []float64
	}{
		{"restart is offset by the previous last value", []float64{0, 10, 20, 0, 5, 10}, []int{3}, []float64{0, 10, 20, 20, 25, 30}},
		{"a continuing count is unchanged", []float64{0, 10, 20, 20, 25, 30}, []int{3}, []float64{0, 10, 20, 20, 25, 30}},
		{"one sequence is unchanged", []float64{0, 10, 20, 0, 5}, nil, []float64{0, 10, 20, 0, 5}},
		{"a decrease inside a sequence is unchanged; the next sequence is compared with its last value",
			[]float64{0, 10, 5, 0, 8, 12}, []int{4}, []float64{0, 10, 5, 0, 8, 12}},
		{"first sequence without distance: second unchanged", []float64{nan, nan, nan, 0, 5, 9}, []int{3}, []float64{nan, nan, nan, 0, 5, 9}},
		{"a sequence without distance in the middle: the third is compared with the first",
			[]float64{0, 10, 20, nan, nan, 0, 4, 8}, []int{3, 5}, []float64{0, 10, 20, nan, nan, 20, 24, 28}},
		{"three sequences are cumulative", []float64{0, 10, 0, 10, 0, 10}, []int{2, 4}, []float64{0, 10, 10, 20, 20, 30}},
		{"v1.2: continuing count 0.01 m below prev is unchanged", []float64{0, 10, 20, 19.99, 25, 30}, []int{3}, []float64{0, 10, 20, 19.99, 25, 30}},
		{"v1.2: first = 0.4 x prev is offset", []float64{0, 10, 20, 8, 12}, []int{3}, []float64{0, 10, 20, 28, 32}},
		{"v1.2: first = 0.6 x prev is unchanged", []float64{0, 10, 20, 12, 15}, []int{3}, []float64{0, 10, 20, 12, 15}},
		{"v1.2: first exactly prev/2 is unchanged (strict <)", []float64{0, 10, 20, 10, 15}, []int{3}, []float64{0, 10, 20, 10, 15}},
		{"v1.2: prev = 0 is unchanged", []float64{0, 0, 0, 0, 5}, []int{3}, []float64{0, 0, 0, 0, 5}},
		{"the first valid value decides, not the first record", []float64{0, 10, nan, 0, 3}, []int{2}, []float64{0, 10, nan, 10, 13}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offsets := seq(0, len(tt.dist)-1)
			// R-12 alone (F-08 v1.4 row 6): R-14 would hold the falls these cases keep.
			var got []float64
			for _, r := range continueDistance(distRecs(offsets, tt.dist), tt.starts) {
				got = append(got, r.Distance)
			}
			if !sameFloats(got, tt.want) {
				t.Errorf("distance = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("the input recording is unchanged", func(t *testing.T) {
		records := distRecs(seq(0, 5), []float64{0, 10, 20, 0, 5, 10})
		starts := []int{3}
		rec := Recording{Records: records, SequenceStarts: starts, Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(5)}}}
		before := Recording{Records: append([]RawRecord(nil), records...), SequenceStarts: append([]int(nil), starts...),
			Sessions: append([]RawSession(nil), rec.Sessions...)}
		Normalise(rec)
		if !reflect.DeepEqual(rec.SequenceStarts, before.SequenceStarts) || !reflect.DeepEqual(rec.Sessions, before.Sessions) {
			t.Errorf("Normalise changed its input")
		}
		for i := range records {
			if records[i].Distance != before.Records[i].Distance {
				t.Errorf("record %d distance changed: %v -> %v", i, before.Records[i].Distance, records[i].Distance)
			}
		}
	})
	t.Run("unsorted times: the arrival order decides the ranges", func(t *testing.T) {
		// Sequence 2 comes first in time; R-2 sorts after R-12 (documented limit).
		records := distRecs([]int{10, 11, 0, 1}, []float64{0, 10, 0, 5})
		w := Normalise(Recording{Records: records, SequenceStarts: []int{2}})
		if w.Derived.Distance == nil {
			t.Fatal("no distance")
		}
	})
	t.Run("start indexes out of range are harmless", func(t *testing.T) {
		for _, starts := range [][]int{{99}, {-3}, {3, 1}, {0, 0}} {
			got := distancesOf(t, distRecs(seq(0, 4), []float64{0, 10, 20, 0, 5}), starts)
			if len(got) != 5 {
				t.Errorf("starts %v: %v", starts, got)
			}
		}
	})
	t.Run("average speed of a same-sport chained file", func(t *testing.T) {
		// Two cycling sequences, 5 m/s each, the second restarting at 0.
		var records []RawRecord
		for i := 0; i < 20; i++ {
			r := rec(i)
			r.Distance = float64(i%10) * 5
			r.Speed = 5
			records = append(records, r)
		}
		w := Normalise(Recording{Records: records, SequenceStarts: []int{10},
			Sessions: []RawSession{{Sport: "cycling", Start: at(0), End: at(9)}, {Sport: "cycling", Start: at(10), End: at(19)}}})
		wantPtr(t, "derived distance", w.Derived.Distance, 90, 1e-9)
		wantPtr(t, "summary avg speed", w.Summary.AvgSpeed, 90.0/19, 1e-9)
		if w.Sport != "cycling" {
			t.Errorf("sport = %q, want cycling (D-17)", w.Sport)
		}
	})
}

// ---- sport timeline ---------------------------------------------------------------

// timelineOf builds the timeline of a recording the way Normalise does.
func timelineOf(rec Recording) *sportTimeline {
	start := startTime(rec)
	var sessions []Session
	for _, s := range rec.Sessions {
		a, b := spanOffsets(start, s.Start, s.End)
		sessions = append(sessions, Session{Sport: s.Sport, Start: a, End: b})
	}
	return newSportTimeline(sessions, collectHints(rec, start))
}

func TestSportTimelineRules(t *testing.T) {
	sess := func(sport string, from, to int) RawSession {
		return RawSession{Sport: sport, Start: at(from), End: at(to)}
	}
	hint := func(sport string, off int) RawSport { return RawSport{At: at(off), Sport: sport} }
	lap := func(sport string, off int) RawLap { return RawLap{Sport: sport, Start: at(off), End: at(off + 5)} }
	// Records anchor the start at offset 0.
	base := recs([]int{0}, nil)

	tests := []struct {
		name string
		rec  Recording
		at   map[int32]string
	}{
		{"rule 1: a session covers; it wins over a hint",
			Recording{Records: base, Sessions: []RawSession{sess("cycling", 10, 20)}, Sports: []RawSport{hint("running", 12)}},
			map[int32]string{10: "cycling", 15: "cycling", 20: "cycling"}},
		{"rule 1: the first covering session in file order",
			Recording{Records: base, Sessions: []RawSession{sess("a", 10, 30), sess("b", 15, 20)}},
			map[int32]string{17: "a"}},
		{"rule 2: a hint after the last session end names an uncovered sample",
			Recording{Records: base, Sessions: []RawSession{sess("cycling", 0, 20)}, Sports: []RawSport{hint("running", 25)}},
			map[int32]string{30: "running"}},
		{"rule 2: a hint exactly at Eprev counts",
			Recording{Records: base, Sessions: []RawSession{sess("cycling", 0, 20)}, Sports: []RawSport{hint("running", 20)}},
			map[int32]string{30: "running"}},
		{"rule 2: a hint before Eprev is ignored and rule 3 gives the session",
			Recording{Records: base, Sessions: []RawSession{sess("cycling", 10, 20)}, Sports: []RawSport{hint("running", 5)}},
			map[int32]string{30: "cycling"}},
		{"rule 2: hints are dropped when empty, multisport or all",
			Recording{Records: base, Sports: []RawSport{hint("", 0), hint("multisport", 5), hint("all", 6), hint("swimming", 8)}},
			map[int32]string{7: "swimming", 20: "swimming"}},
		{"rule 2: the latest hint at or before t",
			Recording{Records: base, Sports: []RawSport{hint("swimming", 0), hint("cycling", 10)}},
			map[int32]string{5: "swimming", 10: "cycling", 50: "cycling"}},
		{"rule 2: a sport message ranks after a lap at one offset",
			Recording{Records: base, Laps: []RawLap{lap("cycling", 10)}, Sports: []RawSport{hint("running", 10)}},
			map[int32]string{12: "running"}},
		{"rule 2: of equal hints the later in arrival order wins",
			Recording{Records: base, Sports: []RawSport{hint("cycling", 10), hint("running", 10)}},
			map[int32]string{12: "running"}},
		{"rule 2: lap.sport is a hint at the lap's start",
			Recording{Records: base, Laps: []RawLap{lap("running", 10)}},
			map[int32]string{12: "running"}},
		{"rule 3: outside every session and without a hint, the latest earlier start",
			Recording{Records: base, Sessions: []RawSession{sess("a", 10, 20), sess("b", 30, 40)}},
			map[int32]string{25: "a", 50: "b"}},
		{"rule 3: equal starts give the first in file order",
			Recording{Records: base, Sessions: []RawSession{sess("a", 10, 20), sess("b", 10, 15)}},
			map[int32]string{30: "a"}},
		{"rule 4: before every session start, the first session",
			Recording{Records: base, Sessions: []RawSession{sess("a", 10, 20), sess("b", 30, 40)}},
			map[int32]string{5: "a"}},
		{"rule 5: before the first hint, the earliest hint",
			Recording{Records: base, Sports: []RawSport{hint("running", 10)}},
			map[int32]string{3: "running"}},
		{"no session and no hint: empty",
			Recording{Records: base},
			map[int32]string{3: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := timelineOf(tt.rec)
			for off, want := range tt.at {
				if got := tl.at(off); got != want {
					t.Errorf("at(%d) = %q, want %q", off, got, want)
				}
			}
		})
	}
}

// refSport is a direct implementation of the rule table (design, Sport
// timeline), kept apart from the production lookup.
func refSport(sessions []Session, sports []RawSport, laps []RawLap, start time.Time, t int32) string {
	type hint struct {
		off     int32
		kind    int
		arrival int
		sport   string
	}
	var hs []hint
	for i, s := range sports {
		if namesLeg(s.Sport) {
			hs = append(hs, hint{offsetOf(start, s.At), 1, i, s.Sport})
		}
	}
	for i, l := range laps {
		if namesLeg(l.Sport) {
			off, _ := spanOffsets(start, l.Start, l.End)
			hs = append(hs, hint{off, 0, i, l.Sport})
		}
	}
	better := func(a, b hint) bool { // a wins over b among hints
		if a.off != b.off {
			return a.off > b.off
		}
		if a.kind != b.kind {
			return a.kind > b.kind
		}
		return a.arrival > b.arrival
	}
	for _, s := range sessions { // rule 1
		if s.Start <= t && t <= s.End {
			return s.Sport
		}
	}
	eprev, haveEprev := int32(0), false // rule 2
	for _, s := range sessions {
		if s.End < t && (!haveEprev || s.End > eprev) {
			eprev, haveEprev = s.End, true
		}
	}
	var win *hint
	for i := range hs {
		h := hs[i]
		if h.off <= t && (!haveEprev || h.off >= eprev) && (win == nil || better(h, *win)) {
			win = &hs[i]
		}
	}
	if win != nil {
		return win.sport
	}
	if len(sessions) > 0 {
		idx := -1 // rule 3
		for i, s := range sessions {
			if s.Start < t && (idx < 0 || s.Start > sessions[idx].Start) {
				idx = i
			}
		}
		if idx < 0 {
			idx = 0 // rule 4
		}
		return sessions[idx].Sport
	}
	var first *hint // rule 5: the winner at the earliest offset
	for i := range hs {
		h := hs[i]
		if first == nil || h.off < first.off || (h.off == first.off && better(h, *first)) {
			first = &hs[i]
		}
	}
	if first != nil {
		return first.sport
	}
	return ""
}

// TestSportTimelineEquivalence: the production lookup equals the reference
// implementation of the rule table for every sample of 1,000 seeded random
// cases (0 to 8 sessions, overlapping and equal starts, 0 to 20 hints at and
// between the session bounds, unsorted arrival).
func TestSportTimelineEquivalence(t *testing.T) {
	rng := rand.New(rand.NewPCG(12, 34))
	names := []string{"cycling", "running", "swimming", "transition", "", "multisport", "all"}
	for n := 0; n < 1000; n++ {
		rec := Recording{Records: recs([]int{0}, nil)}
		var bounds []int
		for i, count := 0, rng.IntN(9); i < count; i++ {
			from := rng.IntN(100)
			to := from + rng.IntN(30)
			rec.Sessions = append(rec.Sessions, RawSession{Sport: fmt.Sprintf("s%d", i), Start: at(from), End: at(to)})
			bounds = append(bounds, from, to, to+1, from+1)
		}
		pick := func() int { // at a session bound, or anywhere
			if len(bounds) > 0 && rng.IntN(2) == 0 {
				return bounds[rng.IntN(len(bounds))]
			}
			return rng.IntN(130)
		}
		for i, count := 0, rng.IntN(21); i < count; i++ {
			sp := names[rng.IntN(len(names))]
			if rng.IntN(2) == 0 {
				rec.Sports = append(rec.Sports, RawSport{At: at(pick()), Sport: sp})
			} else {
				off := pick()
				rec.Laps = append(rec.Laps, RawLap{Sport: sp, Start: at(off), End: at(off + 3)})
			}
		}
		start := startTime(rec)
		var sessions []Session
		for _, s := range rec.Sessions {
			a, b := spanOffsets(start, s.Start, s.End)
			sessions = append(sessions, Session{Sport: s.Sport, Start: a, End: b})
		}
		tl := newSportTimeline(sessions, collectHints(rec, start))
		for off := int32(0); off < 140; off++ {
			if got, want := tl.at(off), refSport(sessions, rec.Sports, rec.Laps, start, off); got != want {
				t.Fatalf("case %d offset %d: at = %q, reference %q\nsessions %+v\nsports %+v\nlaps %+v", n, off, got, want, sessions, rec.Sports, rec.Laps)
			}
		}
	}
}

// TestSportDrivesCadenceAndMoving: R-5 and R-7 read the sport timeline, so a
// session-less recording with a running hint is converted and uses the
// running threshold (QA-08-r1-1).
func TestSportDrivesCadenceAndMoving(t *testing.T) {
	records := recs(seq(0, 9), func(i int, r *RawRecord) {
		r.Cadence = 85 + 64.0/128
		r.Speed = 0.2 // 0.72 km/h: moving for running (0.5 km/h), not for cycling (1 km/h)
	})
	run := Normalise(Recording{Records: records, Sports: []RawSport{{At: at(0), Sport: "running"}}})
	if run.Sport != "running" {
		t.Errorf("sport = %q", run.Sport)
	}
	for i, c := range run.Samples.Cadence {
		if !near(c, 171, 1e-9) {
			t.Fatalf("cadence[%d] = %v, want 171", i, c)
		}
	}
	wantPtr(t, "summary avg cadence", run.Summary.AvgCadence, 171, 1e-9)
	wantPtr(t, "summary max cadence", run.Summary.MaxCadence, 171, 1e-9)
	wantPtr(t, "moving time, running threshold", run.Derived.MovingTime, 9, 0)

	ride := Normalise(Recording{Records: records, Sports: []RawSport{{At: at(0), Sport: "cycling"}}})
	wantPtr(t, "summary avg cadence, cycling", ride.Summary.AvgCadence, 85.5, 1e-9)
	wantPtr(t, "moving time, cycling threshold", ride.Derived.MovingTime, 0, 0)

	// Laps use the sport at their start.
	laps := Normalise(Recording{Records: records, Sports: []RawSport{{At: at(0), Sport: "running"}},
		Laps: []RawLap{{Start: at(0), End: at(9), Totals: Totals{AvgCadence: f64(85.5)}}}})
	wantPtr(t, "lap avg cadence", laps.Laps[0].Totals.AvgCadence, 171, 1e-9)

	none := Normalise(Recording{Records: records})
	if none.Sport != "" {
		t.Errorf("sport without a source = %q, want empty", none.Sport)
	}
}

// ---- R-9 sport (D-17 / OQ-36) ------------------------------------------------------

func TestWorkoutSport(t *testing.T) {
	records := recs(seq(0, 60), nil)
	sess := func(sport string, from, to int) RawSession {
		return RawSession{Sport: sport, Start: at(from), End: at(to)}
	}
	hint := func(sport string, off int) RawSport { return RawSport{At: at(off), Sport: sport} }
	tests := []struct {
		name string
		rec  Recording
		want string
	}{
		{"1 session", Recording{Records: records, Sessions: []RawSession{sess("cycling", 0, 60)}}, "cycling"},
		{"2 sessions of one sport give that sport (D-17)",
			Recording{Records: records, Sessions: []RawSession{sess("cycling", 0, 30), sess("cycling", 31, 60)}}, "cycling"},
		{"2 sports give multisport",
			Recording{Records: records, Sessions: []RawSession{sess("cycling", 0, 30), sess("running", 31, 60)}}, "multisport"},
		{"no session, one hint", Recording{Records: records, Sports: []RawSport{hint("running", 0)}}, "running"},
		{"no session, no hint", Recording{Records: records}, ""},
		{"truncated, 1 cycling session and a tail hinted running",
			Recording{Records: records, Sessions: []RawSession{sess("cycling", 0, 30)}, Sports: []RawSport{hint("running", 31)},
				Integrity: Integrity{Truncated: true}}, "multisport"},
		{"complete, 1 session and a sample after its end with another hint: the session's sport (no record leg)",
			Recording{Records: records, Sessions: []RawSession{sess("cycling", 0, 30)}, Sports: []RawSport{hint("running", 31)}}, "cycling"},
		{"truncated, session and tail of the same sport",
			Recording{Records: records, Sessions: []RawSession{sess("cycling", 0, 30)}, Sports: []RawSport{hint("cycling", 31)},
				Integrity: Integrity{Truncated: true}}, "cycling"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalise(tt.rec).Sport; got != tt.want {
				t.Errorf("sport = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- record leg (QA-08-r1-2) ---------------------------------------------------------

// legRecords: 1 Hz for 0..100, distance 10 m/s, altitude rising 1 m/s in the
// first half and falling in the second, heart rate 100 + offset.
func legRecords() []RawRecord {
	return recs(seq(0, 100), func(i int, r *RawRecord) {
		r.Distance = float64(i) * 10
		r.Speed = 10
		r.HeartRate = float64(100 + i)
		r.Cadence = 80
		if i <= 50 {
			r.Altitude = 100 + float64(i)
		} else {
			r.Altitude = 150 - float64(i-50)
		}
	})
}

func TestRecordLeg(t *testing.T) {
	sessions := func() []RawSession {
		return []RawSession{
			{Sport: "cycling", Start: at(0), End: at(40), Totals: Totals{ElapsedTime: f64(40), TimerTime: f64(38), Distance: f64(400),
				Ascent: f64(40), Descent: f64(0), Calories: f64(10), MaxSpeed: f64(10), MaxHeartRate: f64(130), MaxCadence: f64(90)}},
			{Sport: "cycling", Start: at(41), End: at(60), Totals: Totals{ElapsedTime: f64(19), TimerTime: f64(19), Distance: f64(190),
				Ascent: f64(10), Descent: f64(9), Calories: f64(5), MaxSpeed: f64(10), MaxHeartRate: f64(150), MaxCadence: f64(95)}},
		}
	}
	truncated := Integrity{Truncated: true}

	t.Run("truncated: sessions and the tail", func(t *testing.T) {
		w := Normalise(Recording{Records: legRecords(), Sessions: sessions(), Integrity: truncated})
		s := w.Summary
		// Tail: samples 61..100. Distance 1000 - 600 = 400.
		wantPtr(t, "Distance = sessions + tail growth", s.Distance, 400+190+400, 1e-9)
		wantPtr(t, "MaxHeartRate includes the tail", s.MaxHeartRate, 200, 0)
		wantPtr(t, "MaxSpeed", s.MaxSpeed, 10, 0)
		wantPtr(t, "TimerTime = session sums only", s.TimerTime, 57, 0)
		wantPtr(t, "Calories = session sums only", s.Calories, 15, 0)
		wantPtr(t, "ElapsedTime = wall time to the last record (R-10.E)", s.ElapsedTime, 100, 0)
		if s.Descent == nil || *s.Descent < 9+30 {
			t.Errorf("Descent = %v, want at least the sessions' 9 plus the tail's drop", deref(s.Descent))
		}
		wantPtr(t, "MaxCadence from the legs", s.MaxCadence, 95, 0) // tail stream max 80
	})
	t.Run("not truncated: a sample after the last End makes no leg", func(t *testing.T) {
		w := Normalise(Recording{Records: legRecords(), Sessions: sessions()})
		wantPtr(t, "Distance = sessions only", w.Summary.Distance, 590, 0)
		wantPtr(t, "MaxHeartRate = sessions only", w.Summary.MaxHeartRate, 150, 0)
		one := Normalise(Recording{Records: legRecords(), Sessions: sessions()[:1]})
		wantPtr(t, "one session keeps its device value", one.Summary.Distance, 400, 0)
		wantPtr(t, "one session keeps its elapsed time", one.Summary.ElapsedTime, 40, 0)
	})
	t.Run("truncated: a sample exactly at the last End is not in the leg", func(t *testing.T) {
		recsUntil60 := legRecords()[:61]
		w := Normalise(Recording{Records: recsUntil60, Sessions: sessions(), Integrity: truncated})
		wantPtr(t, "Distance = sessions only", w.Summary.Distance, 590, 0)
		wantPtr(t, "ElapsedTime = wall time (R-10.E)", w.Summary.ElapsedTime, 60, 0)
	})
	t.Run("no session: the whole recording is the leg and equals the records", func(t *testing.T) {
		for _, ig := range []Integrity{{}, truncated} {
			w := Normalise(Recording{Records: legRecords(), Integrity: ig})
			wantPtr(t, "Distance", w.Summary.Distance, 1000, 0)
			wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 100, 0)
			wantPtr(t, "MaxHeartRate", w.Summary.MaxHeartRate, 200, 0)
			wantPtr(t, "Ascent equals derived", w.Summary.Ascent, *w.Derived.ElevationGain, 0)
			wantNil(t, "TimerTime", w.Summary.TimerTime)
			wantNil(t, "Calories", w.Summary.Calories)
		}
	})
	t.Run("a tail of two sports: no cadence and no average speed", func(t *testing.T) {
		w := Normalise(Recording{Records: legRecords(), Sports: []RawSport{{At: at(0), Sport: "cycling"}, {At: at(50), Sport: "running"}}})
		if w.Sport != "multisport" {
			t.Fatalf("sport = %q", w.Sport)
		}
		wantNil(t, "MaxCadence", w.Summary.MaxCadence)
		wantNil(t, "AvgCadence", w.Summary.AvgCadence)
		wantNil(t, "AvgSpeed", w.Summary.AvgSpeed)
	})
	t.Run("a tail without a valid distance: the sessions' sum", func(t *testing.T) {
		records := legRecords()
		for i := 61; i < len(records); i++ {
			records[i].Distance = nan
		}
		w := Normalise(Recording{Records: records, Sessions: sessions(), Integrity: truncated})
		wantPtr(t, "Distance", w.Summary.Distance, 590, 0)
	})
	t.Run("distance decreasing inside the tail: growth is clamped at 0", func(t *testing.T) {
		records := legRecords()
		for i := 61; i < len(records); i++ {
			records[i].Distance = 600 - float64(i-60) // below the distance before the tail
		}
		w := Normalise(Recording{Records: records, Sessions: sessions(), Integrity: truncated})
		wantPtr(t, "Distance", w.Summary.Distance, 590, 0)
	})
	t.Run("the cut triathlon shape: other sports in the tail", func(t *testing.T) {
		ss := sessions()
		ss[1].Sport = "transition"
		w := Normalise(Recording{Records: legRecords(), Sessions: ss, Integrity: truncated,
			Sports: []RawSport{{At: at(61), Sport: "running"}}})
		if w.Sport != "multisport" {
			t.Errorf("sport = %q", w.Sport)
		}
		wantNil(t, "AvgSpeed", w.Summary.AvgSpeed)
	})
}

// TestLegsElapsed pins R-10.E (D-18, D-20; OQ-35 answered, v1.3 restated):
// the summary elapsed time is the wall time unless one session is the whole
// recording in one FIT sequence.
func TestLegsElapsed(t *testing.T) {
	t.Run("2 sessions with a gap: wall time, not the sum (R-10.E)", func(t *testing.T) {
		records := recs(append(seq(0, 20), seq(220, 240)...), nil)
		w := Normalise(Recording{Records: records, Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(20), Totals: Totals{ElapsedTime: f64(20)}},
			{Sport: "running", Start: at(220), End: at(240), Totals: Totals{ElapsedTime: f64(15)}},
		}})
		wantPtr(t, "ElapsedTime = wall time", w.Summary.ElapsedTime, 240, 0)
		if w.Derived.ElapsedTime == nil || *w.Derived.ElapsedTime != 240 {
			t.Errorf("derived elapsed time = %v, want 240", deref(w.Derived.ElapsedTime))
		}
	})
	t.Run("session totals without ElapsedTime: still the wall time", func(t *testing.T) {
		records := recs(append(seq(0, 20), seq(220, 240)...), nil)
		w := Normalise(Recording{Records: records, Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(20)},
			{Sport: "running", Start: at(220), End: at(240)},
		}})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 240, 0)
	})
	t.Run("1 session: the device value", func(t *testing.T) {
		w := Normalise(Recording{Records: recs(seq(0, 20), nil), Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(20), Totals: Totals{ElapsedTime: f64(18)}},
		}})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 18, 0)
	})
	t.Run("truncated, 1 session + record leg: wall time", func(t *testing.T) {
		w := Normalise(Recording{Records: recs(seq(0, 40), nil), Integrity: Integrity{Truncated: true}, Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(20), Totals: Totals{ElapsedTime: f64(18)}},
		}})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 40, 0)
	})
	t.Run("2 sessions and no samples: nil", func(t *testing.T) {
		w := Normalise(Recording{Sessions: []RawSession{
			{Sport: "cycling", Start: at(0), End: at(20), Totals: Totals{ElapsedTime: f64(20)}},
			{Sport: "running", Start: at(220), End: at(240), Totals: Totals{ElapsedTime: f64(15)}},
		}})
		if w.Summary.ElapsedTime != nil {
			t.Errorf("ElapsedTime = %v, want nil", *w.Summary.ElapsedTime)
		}
	})
	t.Run("the returned pointer is not Derived's", func(t *testing.T) {
		d := Derived{ElapsedTime: f64(100)}
		got := wallElapsed(d)
		*got = 1
		if *d.ElapsedTime != 100 {
			t.Errorf("wallElapsed aliases Derived.ElapsedTime")
		}
	})
	one := []RawSession{{Sport: "cycling", Start: at(0), End: at(20), Totals: Totals{ElapsedTime: f64(18)}}}
	t.Run("1 session in sequence 1, records in sequences 1 and 2: wall time (v1.3)", func(t *testing.T) {
		w := Normalise(Recording{Records: recs(append(seq(0, 20), seq(100, 110)...), nil), SequenceStarts: []int{21}, Sessions: one})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 110, 0)
	})
	t.Run("1 session in sequence 2, records in sequences 1 and 2: wall time (v1.3)", func(t *testing.T) {
		ss := []RawSession{{Sport: "cycling", Start: at(100), End: at(110), Totals: Totals{ElapsedTime: f64(9)}}}
		w := Normalise(Recording{Records: recs(append(seq(0, 20), seq(100, 110)...), nil), SequenceStarts: []int{21}, Sessions: ss})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 110, 0)
	})
	t.Run("repeated SequenceStarts (skipped sequence), records in sequence 1: device value", func(t *testing.T) {
		w := Normalise(Recording{Records: recs(seq(0, 20), nil), SequenceStarts: []int{21, 21}, Sessions: one})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 18, 0)
	})
	t.Run("1 session, 1 sequence, a record 1 s after End: device value", func(t *testing.T) {
		w := Normalise(Recording{Records: recs(seq(0, 21), nil), Sessions: one})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 18, 0)
	})
	t.Run("1 session with a device value and no samples: the device value", func(t *testing.T) {
		w := Normalise(Recording{Sessions: one})
		wantPtr(t, "ElapsedTime", w.Summary.ElapsedTime, 18, 0)
	})
}

// TestRecordSequences pins recordSequences (R-10.E, v1.3).
func TestRecordSequences(t *testing.T) {
	r := func(n int) []RawRecord { return recs(seq(0, n-1), nil) }
	bad := func(n int) []RawRecord {
		out := r(n)
		for i := range out {
			out[i].At = time.Time{}
		}
		return out
	}
	tests := []struct {
		name string
		rec  Recording
		want int
	}{
		{"no starts, records", Recording{Records: r(5)}, 1},
		{"no starts, no records", Recording{}, 0},
		{"one start, records on both sides", Recording{Records: r(6), SequenceStarts: []int{3}}, 2},
		{"repeated start", Recording{Records: r(6), SequenceStarts: []int{3, 3}}, 2},
		{"range with only invalid times", Recording{Records: append(r(3), bad(3)...), SequenceStarts: []int{3}}, 1},
		{"start equal to len(Records)", Recording{Records: r(4), SequenceStarts: []int{4}}, 1},
	}
	for _, tt := range tests {
		if got := recordSequences(tt.rec); got != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, got, tt.want)
		}
	}
}

// BenchmarkNormaliseAllCaps runs the all-caps shape once, as a smoke test that
// normalisation with 100 sessions, 10,000 laps and 1,000 hints stays far
// below a second (QA-08-r1-5). The measurement is BenchmarkNormaliseAllCaps.
func BenchmarkNormaliseAllCaps(b *testing.B) {
	const n = 500_000
	rec := Recording{Records: make([]RawRecord, n)}
	for i := range rec.Records {
		r := rec2(i)
		r.Distance, r.Speed, r.Cadence, r.HeartRate = float64(i)*7, 7, 85, 140
		rec.Records[i] = r
	}
	for s := 0; s < 100; s++ {
		sport := "cycling"
		if s%2 == 1 {
			sport = "running"
		}
		rec.Sessions = append(rec.Sessions, RawSession{Sport: sport, Start: at(s * 5000), End: at(s*5000 + 4999)})
	}
	for l := 0; l < 10_000; l++ {
		rec.Laps = append(rec.Laps, RawLap{Sport: "cycling", Start: at(l * 50), End: at(l*50 + 49)})
	}
	for i := 0; i < 1000; i++ {
		rec.Sports = append(rec.Sports, RawSport{At: at(i * 500), Sport: "running"})
	}
	for i := 0; i < 100_000; i++ {
		kind := TimerStart
		if i%2 == 1 {
			kind = TimerStop
		}
		rec.Timer = append(rec.Timer, RawTimerEvent{At: at(i * 5), Kind: kind})
	}
	b.ReportAllocs()
	for b.Loop() {
		Normalise(rec)
	}
}

func rec2(s int) RawRecord { return rec(s) }

// TestRecordLegCompleteChained pins D-20 (QA r2 Q1): a complete chained file
// whose later sequence has records but no session gets no record leg.
func TestRecordLegCompleteChained(t *testing.T) {
	records := distRecs(append(seq(0, 9), seq(100, 109)...),
		[]float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 0, 10, 20, 30, 40, 50, 60, 70, 80, 90})
	w := Normalise(Recording{Records: records, SequenceStarts: []int{10}, Sessions: []RawSession{
		{Sport: "cycling", Start: at(0), End: at(9), Totals: Totals{Distance: f64(90), ElapsedTime: f64(9)}},
	}})
	wantPtr(t, "Distance = the session's", w.Summary.Distance, 90, 0)
	// D-20, R-10.E (v1.3): the records come from two FIT sequences, so the
	// elapsed time is the wall time over both, not the session's 9.
	wantPtr(t, "summary ElapsedTime = wall time", w.Summary.ElapsedTime, 109, 0)
	if w.Derived.ElapsedTime == nil || *w.Derived.ElapsedTime != 109 {
		t.Errorf("derived elapsed = %v, want 109", deref(w.Derived.ElapsedTime))
	}
}
