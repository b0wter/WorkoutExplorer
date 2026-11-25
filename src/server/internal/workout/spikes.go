package workout

import (
	"math"
	"slices"
)

// F-74 speed spike filter (UP-12, architect/F-74 SP-1 to SP-6). A run of at
// most spikeMaxSeconds recorded speed samples that is clearly above the level
// before and after, with the speed returning, becomes missing (not
// interpolated). Only upward, only in legs with a position; moving time keeps
// the speed before removal (Samples.unfiltered).
const (
	spikeMaxSeconds = 5    // UP-12
	spikeLevelCount = 5    // samples per side for the level (median)
	spikeMinCount   = 2    // least samples a side needs
	spikeReach      = 30   // s, furthest a level sample may lie from the run
	spikeBandRel    = 0.25 // run = above level + max(rel*level, abs)
	spikeBandAbs    = 1.0  // m/s
	spikeMarginRel  = 0.5  // every run sample >= level + rel*level + abs
	spikeMarginAbs  = 2.0  // m/s
)

func spikeBand(l float64) float64   { return math.Max(spikeBandRel*l, spikeBandAbs) }
func spikeMargin(l float64) float64 { return spikeMarginRel*l + spikeMarginAbs }

// removeSpeedSpikes runs the filter on w.Samples and fixes the max speed of
// the sessions that lost a sample (SP-6).
func removeSpeedSpikes(w *Workout) {
	s := &w.Samples
	if s.Speed == nil || s.Lat == nil {
		return
	}
	legOf := make([]int, len(s.Offset)) // 0 = no session, k+1 = session k (first that covers wins)
	for k, x := range w.Sessions {
		from, to := SampleRange(s.Offset, x.Start, x.End)
		for i := from; i < to; i++ {
			if legOf[i] == 0 {
				legOf[i] = k + 1
			}
		}
	}
	var removed []int
	for leg := 0; leg <= len(w.Sessions); leg++ {
		if leg > 0 {
			if x := w.Sessions[leg-1]; x.Sport == sportTransition || isPoolSwim(x) {
				continue
			}
		}
		var idx []int // present recorded speed samples of the leg
		hasPos := false
		for i, l := range legOf {
			if l != leg {
				continue
			}
			hasPos = hasPos || !math.IsNaN(s.Lat[i])
			if !s.Inserted(i) && !math.IsNaN(s.Speed[i]) {
				idx = append(idx, i)
			}
		}
		if hasPos {
			removed = append(removed, spikesOf(s, idx)...)
		}
	}
	if len(removed) == 0 {
		return
	}
	s.unfiltered = map[int]float64{}
	drop := func(i int) {
		if !math.IsNaN(s.Speed[i]) {
			s.unfiltered[i] = s.Speed[i]
			s.Speed[i] = math.NaN()
		}
	}
	for _, i := range removed {
		drop(i)
		// SP-2: the samples R-3 interpolated from the spike go too.
		for j := i - 1; j >= 0 && s.Inserted(j); j-- {
			drop(j)
		}
		for j := i + 1; j < len(s.Speed) && s.Inserted(j); j++ {
			drop(j)
		}
	}
	for k := range w.Sessions {
		from, to := SampleRange(s.Offset, w.Sessions[k].Start, w.Sessions[k].End)
		hit := false
		for i := from; i < to && !hit; i++ {
			_, hit = s.unfiltered[i]
		}
		if hit {
			_, w.Sessions[k].Totals.MaxSpeed = meanMax(s.Speed[from:to], false)
		}
	}
}

// spikesOf returns the sample indices of the removable runs among idx (SP-4).
func spikesOf(s *Samples, idx []int) []int {
	var out []int
	for k := 0; k < len(idx); {
		lb, ok := level(s, idx, k-1, -1)
		if !ok || s.Speed[idx[k]] <= lb+spikeBand(lb) {
			k++
			continue
		}
		e := k
		for e+1 < len(idx) && s.Speed[idx[e+1]] > lb+spikeBand(lb) {
			e++
		}
		run := idx[k : e+1]
		la, ok := level(s, idx, e+1, 1)
		if ok && s.Offset[run[len(run)-1]]-s.Offset[run[0]]+1 <= spikeMaxSeconds &&
			math.Abs(la-lb) <= spikeBand(lb) &&
			!slices.ContainsFunc(run, func(i int) bool { return s.Speed[i] < lb+spikeMargin(lb) }) {
			out = append(out, run...)
		}
		k = e + 1
	}
	return out
}

// level is the median of up to spikeLevelCount samples of idx, starting at
// position from and walking in direction dir (-1 before, +1 after), within
// spikeReach seconds of the sample at from.
func level(s *Samples, idx []int, from, dir int) (float64, bool) {
	var v []float64
	for j := from; j >= 0 && j < len(idx) && len(v) < spikeLevelCount; j += dir {
		if d := s.Offset[idx[from]] - s.Offset[idx[j]]; d > spikeReach || -d > spikeReach {
			break
		}
		v = append(v, s.Speed[idx[j]])
	}
	if len(v) < spikeMinCount {
		return 0, false
	}
	slices.Sort(v)
	return (v[(len(v)-1)/2] + v[len(v)/2]) / 2, true
}
