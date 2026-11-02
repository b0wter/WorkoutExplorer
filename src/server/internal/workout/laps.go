package workout

import (
	"math"
	"sort"
)

// SampleRange returns the half-open sample range [from, to) of a lap or
// session with time offsets start and end (F-03 §5.4): from is the first
// sample at or after start, to the first sample after end. Adjacent laps
// share their boundary sample. The one Go implementation of the rule
// (F-18 design: datafile calls it).
func SampleRange(off []int32, start, end int32) (from, to int) {
	from = sort.Search(len(off), func(i int) bool { return off[i] >= start })
	to = sort.Search(len(off), func(i int) bool { return off[i] > end })
	return from, to
}

// completeLaps implements F-18 L-1: per AP-8 column key, a lap keeps the
// device's value; where the device recorded none, the value is computed over
// the lap's range [a, b] = [start_index, end_index − 1] with the range rules
// of F-17 R-2 (the same M and G series, ADR-029). A lap with fewer than two
// samples gets nothing. A lap that starts in a transition gets no speed,
// cadence or power (R-13 removed them; OQ-78). Runs on the normalised
// samples, before any public transform, so hiding changes no lap value.
func completeLaps(w *Workout, tl *sportTimeline) {
	s := &w.Samples
	for i := range w.Laps {
		l := &w.Laps[i]
		from, to := SampleRange(s.Offset, l.Start, l.End)
		if to-from < 2 {
			continue
		}
		v := rangeValues(s, from, to-1)
		t := &l.Totals
		fill := func(dst **float64, src *float64) {
			if *dst == nil {
				*dst = src
			}
		}
		fill(&t.ElapsedTime, v.elapsed)
		fill(&t.Distance, v.distance)
		fill(&t.Ascent, v.gain)
		fill(&t.AvgHeartRate, v.heartRate)
		if !tl.inTransition(l.Start) {
			fill(&t.AvgSpeed, v.speed)
			fill(&t.AvgPower, v.power)
			fill(&t.AvgCadence, v.cadence)
		}
	}
}

// rangeStats are the F-17 R-2 range values that a lap row can show; nil
// where the range has none.
type rangeStats struct {
	elapsed, distance, gain, speed, heartRate, power, cadence *float64
}

// rangeValues evaluates the F-17 range values table over the inclusive
// sample range [a, b], a < b.
func rangeValues(s *Samples, a, b int) rangeStats {
	var r rangeStats
	e := float64(s.Offset[b] - s.Offset[a])
	r.elapsed = &e
	if s.Distance != nil {
		if da, db, ok := carriedDistance(s.Distance, a, b); ok {
			d := db - da
			r.distance = &d
			if s.MovingTime != nil {
				if m := s.MovingTime[b] - s.MovingTime[a]; m > 0 {
					v := d / m
					r.speed = &v
				}
			}
		}
	}
	// No valid altitude in the range: gain is absent, never 0 (AP-8, QA-18-r1-1).
	if s.Gain != nil && rangeMean(s.Altitude, a, b, false) != nil {
		g := s.Gain[b] - s.Gain[a]
		r.gain = &g
	}
	r.heartRate = rangeMean(s.HeartRate, a, b, false)
	r.power = rangeMean(s.Power, a, b, false)    // zeros count (OQ-23)
	r.cadence = rangeMean(s.Cadence, a, b, true) // zeros do not
	return r
}

// carriedDistance returns d̄[a] and d̄[b] of F-17 R-2: the valid distance at
// or before the index, or else the first valid distance. ok is false when the
// stream has no valid value.
func carriedDistance(d []float64, a, b int) (da, db float64, ok bool) {
	first := math.NaN()
	for _, x := range d {
		if !math.IsNaN(x) {
			first = x
			break
		}
	}
	if math.IsNaN(first) {
		return 0, 0, false
	}
	at := func(i int) float64 {
		for ; i >= 0; i-- {
			if !math.IsNaN(d[i]) {
				return d[i]
			}
		}
		return first
	}
	return at(a), at(b), true
}

// rangeMean is the mean of the present samples of v in [a, b], zeros left
// out with skipZero; nil without such samples or without the stream.
func rangeMean(v []float64, a, b int, skipZero bool) *float64 {
	if v == nil {
		return nil
	}
	var sum float64
	n := 0
	for _, x := range v[a : b+1] {
		if math.IsNaN(x) || (skipZero && x == 0) {
			continue
		}
		sum += x
		n++
	}
	if n == 0 {
		return nil
	}
	m := sum / float64(n)
	return &m
}
