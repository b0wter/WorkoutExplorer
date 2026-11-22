package workout

import (
	"slices"
	"sort"
)

// Added lap boundaries (F-65 AL-3, AL-4, ADR-060). Sample indices are of the
// whole normalised workout, before trim and hiding; the browser mirrors these
// rules over draft.bin, which has the same samples.

// TriggerAdded is the trigger of a lap part that ends at an added boundary.
const TriggerAdded = "added"

// MaxAddedLapBoundaries is the cap of F-65 (AL-2).
const MaxAddedLapBoundaries = 100

// lapSpan is a lap that can be split: a device lap (lap >= 0) or an implicit
// lap (lap = -1), over the inclusive samples [first, last].
type lapSpan struct{ first, last, lap int }

// lapSpans returns the device laps with at least one sample and the
// implicit laps: every run of samples no device lap covers, widened to the
// touching samples. Without device laps that is one lap over the recording.
func lapSpans(w *Workout) []lapSpan {
	off := w.Samples.Offset
	n := len(off)
	var out []lapSpan
	covered := make([]bool, n)
	for i, l := range w.Laps {
		from, to := SampleRange(off, l.Start, l.End)
		if to-from < 1 {
			continue
		}
		out = append(out, lapSpan{from, to - 1, i})
		for k := from; k < to; k++ {
			covered[k] = true
		}
	}
	for k := 0; k < n; {
		if covered[k] {
			k++
			continue
		}
		p := k
		for k < n && !covered[k] {
			k++
		}
		out = append(out, lapSpan{max(p-1, 0), min(k, n-1), -1})
	}
	return out
}

// LapEnds returns, sorted and without repeats, the first and last sample of
// every device and implicit lap: samples where an added boundary is refused.
func LapEnds(w Workout) []int {
	var out []int
	for _, s := range lapSpans(&w) {
		out = append(out, s.first, s.last)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// SnapLapBoundary is AL-4: sample i inside a pool-swim session (the
// latest-starting one whose samples hold i) that has lengths moves to the
// nearest last sample of that session's lengths (a length belongs to the
// session that holds its last sample; rest lengths count), by time, ties to
// the earlier. Otherwise i is returned unchanged.
func SnapLapBoundary(w Workout, i int) int {
	off := w.Samples.Offset
	if i < 0 || i >= len(off) {
		return i
	}
	sf, st, pool := -1, 0, false
	for _, s := range w.Sessions {
		if from, to := SampleRange(off, s.Start, s.End); i >= from && i < to && from >= sf {
			sf, st, pool = from, to, isPoolSwim(s)
		}
	}
	best, bestD := -1, int64(0)
	if pool {
		for _, l := range w.Lengths {
			from, to := SampleRange(off, l.Start, l.End)
			last := to - 1
			if to-from < 1 || last < sf || last >= st {
				continue
			}
			d := int64(off[last]) - int64(off[i])
			if d < 0 {
				d = -d
			}
			if best < 0 || d < bestD || d == bestD && last < best {
				best, bestD = last, d
			}
		}
	}
	if best < 0 {
		return i
	}
	return best
}

// AddLaps is AL-3: every index of at that lies strictly inside (lo, hi) (the
// trim window) and is not a lap end splits the lap that holds it (of
// overlapping laps, the later-starting). Each part copies the lap, shares
// the split sample with its neighbour, and gets its totals computed as for a
// range (F-18 L-1, transition rule by start); every part but the last gets
// TriggerAdded, the last keeps the lap's trigger ("" for an implicit lap).
// Unsplit laps stay as they were; laps are returned in time order. Any other
// index of at is skipped, never an error. w is not modified; samples are
// shared.
func AddLaps(w Workout, at []int, lo, hi int) Workout {
	if len(at) == 0 {
		return w
	}
	spans := lapSpans(&w)
	ends := LapEnds(w)
	cuts := make(map[int][]int) // span index → split samples
	for _, k := range at {
		if k <= lo || k >= hi {
			continue
		}
		if _, on := slices.BinarySearch(ends, k); on {
			continue
		}
		best := -1
		for j, s := range spans {
			if k > s.first && k < s.last && (best < 0 || s.first >= spans[best].first) {
				best = j
			}
		}
		if best >= 0 && !slices.Contains(cuts[best], k) {
			cuts[best] = append(cuts[best], k)
		}
	}
	if len(cuts) == 0 {
		return w
	}
	off := w.Samples.Offset
	tl := newSportTimeline(w.Sessions, w.hints)
	partsOf := func(j int, base Lap) []Lap {
		ks := cuts[j]
		slices.Sort(ks)
		out := make([]Lap, 0, len(ks)+1)
		start := base.Start
		for i := 0; i <= len(ks); i++ {
			p := base
			p.Totals, p.Start = Totals{}, start
			if i < len(ks) {
				p.End, p.Trigger = off[ks[i]], TriggerAdded
				start = off[ks[i]]
			}
			completeTotals(&w.Samples, &p.Totals, p.Start, p.End, tl)
			out = append(out, p)
		}
		return out
	}
	spanOf := make(map[int]int, len(w.Laps)) // device lap → span index
	for j, s := range spans {
		if s.lap >= 0 {
			spanOf[s.lap] = j
		}
	}
	out := w
	out.Laps = make([]Lap, 0, len(w.Laps)+len(at)+1)
	for i, l := range w.Laps {
		if j, ok := spanOf[i]; ok && cuts[j] != nil {
			out.Laps = append(out.Laps, partsOf(j, l)...)
		} else {
			out.Laps = append(out.Laps, l)
		}
	}
	for j, s := range spans {
		if s.lap < 0 && cuts[j] != nil {
			out.Laps = append(out.Laps, partsOf(j, Lap{Start: off[s.first], End: off[s.last]})...)
		}
	}
	sort.SliceStable(out.Laps, func(i, j int) bool { return out.Laps[i].Start < out.Laps[j].Start })
	return out
}
