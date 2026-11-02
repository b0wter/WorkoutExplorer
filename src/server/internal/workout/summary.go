package workout

// summarise implements R-10 as revised by F-08 (decision 12, OQ-22, OQ-32;
// v1.1: combined over legs, D-13).
//
// Step 0 (legsOf): the legs are the sessions plus, in a truncated or
// session-less workout, the record leg. Step 1: values combined over the
// legs. One leg: as recorded. Two or more: additive totals summed, maxima
// maximised, averages not combined (nil). Step 2: every field still nil is
// filled from the records (R-8, R-11), except where sports are mixed (average
// speed and cadence stay hidden), and except timer time and calories, which
// have no record source. Step 3: moving time is always derived (D-5).
//
// legs and sports come from legsOf; sports is S of R-9. singleRecording is
// true when exactly one session is the whole workout in one FIT sequence
// (R-10.E).
func summarise(legs []Totals, sports []string, d Derived, singleRecording bool) Summary {
	var s Summary
	mixed := len(sports) >= 2
	switch len(legs) {
	case 0:
	case 1:
		s.Totals = legs[0]
		if mixed {
			s.MaxCadence = nil
		}
	default:
		s.Totals = combine(legs)
		if !mixed {
			s.MaxCadence = pick(legs, func(t *Totals) *float64 { return t.MaxCadence }, maxOf)
		}
	}

	// R-10.E: wall time unless a single session is the whole recording.
	if len(legs) > 0 && !singleRecording {
		s.ElapsedTime = wallElapsed(d)
	}

	fill := func(dst **float64, src *float64) {
		if *dst == nil {
			*dst = src
		}
	}
	fill(&s.ElapsedTime, d.ElapsedTime)
	fill(&s.Distance, d.Distance)
	fill(&s.Ascent, d.ElevationGain)
	fill(&s.Descent, d.ElevationLoss)
	fill(&s.MaxSpeed, d.MaxSpeed)
	fill(&s.AvgHeartRate, d.AvgHeartRate)
	fill(&s.MaxHeartRate, d.MaxHeartRate)
	fill(&s.AvgPower, d.AvgPower)
	fill(&s.MaxPower, d.MaxPower)
	fill(&s.AvgTemperature, d.AvgTemperature)
	fill(&s.MaxTemperature, d.MaxTemperature)
	if !mixed {
		fill(&s.AvgSpeed, d.AvgSpeed)
		fill(&s.AvgCadence, d.AvgCadence)
		fill(&s.MaxCadence, d.MaxCadence)
	}
	s.MovingTime = d.MovingTime
	return s
}

// wallElapsed is rule R-10.E (D-18, D-20; OQ-35 answered by the user on 15
// Oct 2026): the summary elapsed time of every workout except a single
// session recorded in one FIT sequence is the wall time from the workout
// start (t = 0, F-02 R-1) to the last record, transitions and gaps included.
// It is a copy of d.ElapsedTime, never the sum of the legs, and nil when
// there are no samples.
func wallElapsed(d Derived) *float64 {
	if d.ElapsedTime == nil {
		return nil
	}
	v := *d.ElapsedTime
	return &v
}

// combine sums the additive totals and maximises the maxima over the legs
// that recorded each field; averages stay nil. ElapsedTime is set by
// wallElapsed and MaxCadence by the caller (it depends on the sports).
func combine(legs []Totals) Totals {
	var t Totals
	add := func(f func(*Totals) *float64) *float64 {
		return pick(legs, f, func(a, b float64) float64 { return a + b })
	}
	most := func(f func(*Totals) *float64) *float64 { return pick(legs, f, maxOf) }
	t.TimerTime = add(func(t *Totals) *float64 { return t.TimerTime })
	t.Distance = add(func(t *Totals) *float64 { return t.Distance })
	t.Ascent = add(func(t *Totals) *float64 { return t.Ascent })
	t.Descent = add(func(t *Totals) *float64 { return t.Descent })
	t.Calories = add(func(t *Totals) *float64 { return t.Calories })
	t.MaxSpeed = most(func(t *Totals) *float64 { return t.MaxSpeed })
	t.MaxHeartRate = most(func(t *Totals) *float64 { return t.MaxHeartRate })
	t.MaxPower = most(func(t *Totals) *float64 { return t.MaxPower })
	t.MaxTemperature = most(func(t *Totals) *float64 { return t.MaxTemperature })
	return t
}

func maxOf(a, b float64) float64 { return max(a, b) }

// pick folds the non-nil values of a field over the legs; nil if no leg
// recorded it.
func pick(legs []Totals, field func(*Totals) *float64, op func(a, b float64) float64) *float64 {
	var acc *float64
	for i := range legs {
		v := field(&legs[i])
		if v == nil {
			continue
		}
		if acc == nil {
			x := *v
			acc = &x
			continue
		}
		x := op(*acc, *v)
		acc = &x
	}
	return acc
}
