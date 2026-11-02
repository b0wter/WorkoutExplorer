package workout

import "sort"

// legsOf implements step 0 of R-10 (F-08 design v1.1, D-13, QA-08-r1-2) and
// the workout sports S of R-9. The summary combines legs: every session is
// one, and a truncated or session-less workout has one more, the record leg,
// for the samples after the last session end, which no surviving session
// describes. It returns the legs' totals and S, the distinct sports of the
// sessions and of the samples of the record leg, in order of first
// appearance.
func legsOf(w *Workout, tl *sportTimeline) (legs []Totals, sports []string) {
	var set sportSet
	for i := range w.Sessions {
		legs = append(legs, w.Sessions[i].Totals)
		set.add(w.Sessions[i].Sport)
	}
	if t, ok := recordLeg(w, tl, &set); ok {
		legs = append(legs, t)
	}
	return legs, set.list
}

// recordLeg builds the record leg if there is one: the workout is truncated
// or has no session, and at least one sample lies after the largest session
// end. In a complete file the device's sessions are its statement (AP-2); a
// record just after the session end must not split a one-session file.
func recordLeg(w *Workout, tl *sportTimeline, set *sportSet) (Totals, bool) {
	var t Totals
	s := w.Samples
	n := len(s.Offset)
	if n == 0 || !(w.Integrity.Truncated || len(w.Sessions) == 0) {
		return t, false
	}
	i0 := 0
	if len(w.Sessions) > 0 {
		eLast := w.Sessions[0].End
		for _, x := range w.Sessions[1:] {
			eLast = max(eLast, x.End)
		}
		i0 = sort.Search(n, func(i int) bool { return s.Offset[i] > eLast })
		if i0 == n {
			return t, false
		}
	}
	// S: the sports of the leg's samples.
	prev := "\x00"
	for i := i0; i < n; i++ {
		if sp := tl.at(s.Offset[i]); sp != prev {
			set.add(sp)
			prev = sp
		}
	}

	legStart := int32(0)
	if len(w.Sessions) > 0 {
		legStart = s.Offset[i0]
	}
	elapsed := float64(s.Offset[n-1] - legStart)
	t.ElapsedTime = &elapsed

	if s.Distance != nil {
		if last := lastValid(s.Distance[i0:]); last != nil {
			ref := 0.0
			if r := lastValid(s.Distance[:i0]); r != nil {
				ref = *r
			}
			d := max(*last-ref, 0)
			t.Distance = &d
		}
	}
	t.Ascent, t.Descent = elevation(s.Offset[i0:], sliceOrNil(s.Altitude, i0), hysteresisMetres)
	_, t.MaxSpeed = meanMax(sliceOrNil(s.Speed, i0), false)
	_, t.MaxHeartRate = meanMax(sliceOrNil(s.HeartRate, i0), false)
	_, t.MaxCadence = meanMax(sliceOrNil(s.Cadence, i0), true)
	_, t.MaxPower = meanMax(sliceOrNil(s.Power, i0), false)
	_, t.MaxTemperature = meanMax(sliceOrNil(s.Temperature, i0), false)
	return t, true
}

func sliceOrNil(v []float64, from int) []float64 {
	if v == nil {
		return nil
	}
	return v[from:]
}
