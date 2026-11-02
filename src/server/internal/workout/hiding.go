package workout

import "math"

// HideRadius is the start and end hiding circle in metres (SH-4; F-16 H-2).
const HideRadius = 300.0

// hideBounds is the one definition of the cut (F-16 H-2, H-3; ADR-030 item 1):
// from and to are the first and last sample whose position stays public.
// positions is false for a workout without positions (nothing to cut);
// from < 0 means the route is fully hidden. Start section: every sample before
// the first recorded fix more than HideRadius (great circle) from the first
// positioned sample; end section: the same backwards from the last positioned
// sample. Samples inserted by gap filling never count as the boundary (H-3),
// so the first and last visible positions are recorded fixes.
func hideBounds(s Samples) (from, to int, positions bool) {
	if s.Lat == nil || s.Lon == nil {
		return 0, 0, false
	}
	recorded := func(i int) bool {
		return !math.IsNaN(s.Lat[i]) && !math.IsNaN(s.Lon[i]) && (s.inserted == nil || !s.inserted[i])
	}
	first, last := -1, -1
	for i := range s.Offset {
		if recorded(i) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return 0, 0, false // no position at all (cannot happen after R-4)
	}
	from, to = -1, -1
	for i := first; i <= last; i++ {
		if recorded(i) && haversine(s.Lat[first], s.Lon[first], s.Lat[i], s.Lon[i]) > HideRadius {
			from = i
			break
		}
	}
	for i := last; i >= first; i-- {
		if recorded(i) && haversine(s.Lat[last], s.Lon[last], s.Lat[i], s.Lon[i]) > HideRadius {
			to = i
			break
		}
	}
	if from < 0 || to < 0 || from > to {
		return -1, -1, true
	}
	return from, to, true
}

// HideEnds is the start and end hiding of SH-4 (F-16 design H-1 to H-5), the
// public transform when the sharer turned hiding on: every sample outside
// hideBounds loses its position. Only latitude and longitude change; every
// other field of w stays as it is.
//
// fullyHidden reports that w had positions and the cut left none; the result
// then has no position streams. A workout without positions is returned
// unchanged with fullyHidden false (indoor, H-5). w is not modified.
func HideEnds(w Workout) (out Workout, fullyHidden bool) {
	s := w.Samples
	from, to, positions := hideBounds(s)
	if !positions {
		return w, false
	}
	if from < 0 {
		w.Samples.Lat, w.Samples.Lon = nil, nil
		return w, true
	}
	n := len(s.Offset)
	lat := make([]float64, n)
	lon := make([]float64, n)
	for i := range lat {
		if i < from || i > to {
			lat[i], lon[i] = math.NaN(), math.NaN()
		} else {
			lat[i], lon[i] = s.Lat[i], s.Lon[i]
		}
	}
	w.Samples.Lat, w.Samples.Lon = lat, lon
	return w, false
}

// HiddenRanges is what HideEnds would remove, as inclusive sample index pairs
// (ADR-030 item 3): nil without positions, [[0, n-1]] when the route would be
// fully hidden, else the start section and the end section. It is the same
// whatever the workout's hiding setting; it is display only and never decides
// what is public.
func HiddenRanges(w Workout) [][2]int {
	from, to, positions := hideBounds(w.Samples)
	n := len(w.Samples.Offset)
	switch {
	case !positions:
		return nil
	case from < 0:
		return [][2]int{{0, n - 1}}
	}
	// from > first >= 0 and to < last <= n-1, so both sections are non-empty.
	return [][2]int{{0, from - 1}, {to + 1, n - 1}}
}
