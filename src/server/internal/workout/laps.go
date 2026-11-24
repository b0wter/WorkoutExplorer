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
	ix := newRangeIndex(&w.Samples)
	for i := range w.Laps {
		l := &w.Laps[i]
		completeTotals(ix, &l.Totals, l.Start, l.End, tl)
	}
}

// completeTotals fills the nil AP-8 columns of t, a lap's or session's totals
// over the offsets [start, end], as completeLaps describes.
func completeTotals(ix *rangeIndex, t *Totals, start, end int32, tl *sportTimeline) {
	from, to := SampleRange(ix.s.Offset, start, end)
	if to-from < 2 {
		return
	}
	v := ix.values(from, to-1)
	fill := func(dst **float64, src *float64) {
		if *dst == nil {
			*dst = src
		}
	}
	fill(&t.ElapsedTime, v.elapsed)
	fill(&t.Distance, v.distance)
	fill(&t.Ascent, v.gain)
	fill(&t.AvgHeartRate, v.heartRate)
	if !tl.inTransition(start) {
		fill(&t.AvgSpeed, v.speed)
		fill(&t.AvgPower, v.power)
		fill(&t.AvgCadence, v.cadence)
	}
}

// cumul is a prefix sum of the present samples of one stream: sum[i] and
// n[i] cover v[:i]. A range mean is then O(1) (ADR-017: normalisation stays
// O((n + k) log n), never laps × samples).
type cumul struct {
	sum []float64
	n   []int32
}

func newCumul(v []float64, skipZero bool) *cumul {
	if v == nil {
		return nil
	}
	c := &cumul{sum: make([]float64, len(v)+1), n: make([]int32, len(v)+1)}
	for i, x := range v {
		c.sum[i+1], c.n[i+1] = c.sum[i], c.n[i]
		if !math.IsNaN(x) && !(skipZero && x == 0) {
			c.sum[i+1] += x
			c.n[i+1]++
		}
	}
	return c
}

// mean is the mean of the present samples in [a, b]; nil without any or
// without the stream.
func (c *cumul) mean(a, b int) *float64 {
	if c == nil {
		return nil
	}
	n := c.n[b+1] - c.n[a]
	if n == 0 {
		return nil
	}
	m := (c.sum[b+1] - c.sum[a]) / float64(n)
	return &m
}

// rangeIndex answers the F-17 R-2 range values of one Samples in O(1) per
// range after one O(n) pass.
type rangeIndex struct {
	s                *Samples
	alt, hr, pw, cad *cumul
	dist             []float64 // d̄: carried distance, nil without a valid value
}

func newRangeIndex(s *Samples) *rangeIndex {
	ix := &rangeIndex{s: s, hr: newCumul(s.HeartRate, false), pw: newCumul(s.Power, false), // zeros count (OQ-23)
		cad: newCumul(s.Cadence, true)} // zeros do not
	if s.Gain != nil {
		ix.alt = newCumul(s.Altitude, false)
	}
	ix.dist = carriedSeries(s.Distance)
	return ix
}

// carriedSeries is d̄ of F-17 R-2 for every index: the valid distance at or
// before it, or else the first valid distance; nil when none is valid.
func carriedSeries(d []float64) []float64 {
	first := math.NaN()
	for _, x := range d {
		if !math.IsNaN(x) {
			first = x
			break
		}
	}
	if math.IsNaN(first) {
		return nil
	}
	out := make([]float64, len(d))
	cur := first
	for i, x := range d {
		if !math.IsNaN(x) {
			cur = x
		}
		out[i] = cur
	}
	return out
}

// rangeStats are the F-17 R-2 range values that a lap row can show; nil
// where the range has none.
type rangeStats struct {
	elapsed, distance, gain, speed, heartRate, power, cadence *float64
}

// values evaluates the F-17 range values table over the inclusive sample
// range [a, b], a < b.
func (ix *rangeIndex) values(a, b int) rangeStats {
	s := ix.s
	var r rangeStats
	e := float64(s.Offset[b] - s.Offset[a])
	r.elapsed = &e
	if ix.dist != nil {
		d := ix.dist[b] - ix.dist[a]
		r.distance = &d
		if s.MovingTime != nil {
			if m := s.MovingTime[b] - s.MovingTime[a]; m > 0 {
				v := d / m
				r.speed = &v
			}
		}
	}
	// No valid altitude in the range: gain is absent, never 0 (AP-8, QA-18-r1-1).
	if ix.alt != nil && ix.alt.mean(a, b) != nil {
		g := s.Gain[b] - s.Gain[a]
		r.gain = &g
	}
	r.heartRate = ix.hr.mean(a, b)
	r.power = ix.pw.mean(a, b)
	r.cadence = ix.cad.mean(a, b)
	return r
}

// carriedDistance returns d̄[a] and d̄[b] of F-17 R-2. ok is false when the
// stream has no valid value. O(n): for one-off use (trim), not per lap.
func carriedDistance(d []float64, a, b int) (da, db float64, ok bool) {
	c := carriedSeries(d)
	if c == nil {
		return 0, 0, false
	}
	return c[a], c[b], true
}

// rangeMean is the mean of the present samples of v in [a, b], zeros left
// out with skipZero; nil without such samples or without the stream. O(b-a):
// one-off use (trim), not per lap.
func rangeMean(v []float64, a, b int, skipZero bool) *float64 {
	if v == nil {
		return nil
	}
	return newCumul(v[a:b+1], skipZero).mean(0, b-a)
}
