package workout

import "math"

// The start and end hiding radii (SH-4; F-55 HR-1): 0 to HideMaxM metres in
// steps of HideStepM, per end. 0 means that end is not cut.
const (
	HideStepM = 50
	HideMaxM  = 2000
	// HideStepCount is the number of radius steps per end, 0 included.
	HideStepCount = HideMaxM/HideStepM + 1
)

// ValidHideRadius reports whether m is a radius the sharer may choose.
func ValidHideRadius(m int) bool { return m >= 0 && m <= HideMaxM && m%HideStepM == 0 }

// HidingSteps is the one measure of the cut (F-55 HR-3, ADR-056): entry k of
// Start is the first sample whose position stays public at a start radius of
// k·HideStepM metres, entry k of End the last one at that end radius, -1 when
// none does. Entry 0 is the window's first or last sample (radius 0 cuts
// nothing). Display only outside this package: the public cut calls it again
// on every build.
type HidingSteps struct {
	Start []int `json:"start"`
	End   []int `json:"end"`
}

// StepsOf computes the hiding steps of w in one pass per end, or nil when w
// has no position. Start section at radius r: every sample before the first
// recorded fix more than r (great circle) from the first positioned sample;
// end section the same backwards from the last positioned sample. Samples
// inserted by gap filling never count as the boundary (F-16 H-3), so the
// first and last visible positions are recorded fixes.
func StepsOf(w Workout) *HidingSteps {
	s := w.Samples
	if s.Lat == nil || s.Lon == nil {
		return nil
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
		return nil // no position at all (cannot happen after R-4)
	}
	// pass fills steps[1:] with the first index in order whose fix lies more
	// than k·HideStepM from the anchor: the distances only need to be passed
	// once, since a larger radius is first exceeded at the same or a later fix.
	pass := func(anchor, from, to, dir int) []int {
		steps := make([]int, HideStepCount)
		k := 1
		for i := from; i != to+dir && k < HideStepCount; i += dir {
			if !recorded(i) {
				continue
			}
			d := haversine(s.Lat[anchor], s.Lon[anchor], s.Lat[i], s.Lon[i])
			for k < HideStepCount && d > float64(k*HideStepM) {
				steps[k] = i
				k++
			}
		}
		for ; k < HideStepCount; k++ {
			steps[k] = -1
		}
		return steps
	}
	h := &HidingSteps{Start: pass(first, first, last, 1), End: pass(last, last, first, -1)}
	h.Start[0], h.End[0] = 0, len(s.Offset)-1
	return h
}

// Window is the one combination of two steps (HR-3): from and to are the
// first and last sample whose position stays public at the radii startM and
// endM, which must be valid; fully is true when no position stays.
func (h *HidingSteps) Window(startM, endM int) (from, to int, fully bool) {
	from, to = h.Start[startM/HideStepM], h.End[endM/HideStepM]
	return from, to, from < 0 || to < 0 || from > to
}

// Shift returns h with every index but -1 moved by a (a trimmed workout's
// steps in the untrimmed sample indices, ADR-056 item 3).
func (h *HidingSteps) Shift(a int) *HidingSteps {
	if h == nil {
		return nil
	}
	shift := func(v []int) []int {
		out := make([]int, len(v))
		for i, x := range v {
			out[i] = x
			if x >= 0 {
				out[i] = x + a
			}
		}
		return out
	}
	return &HidingSteps{Start: shift(h.Start), End: shift(h.End)}
}

// Hide is the start and end hiding of SH-4 (F-16 H-1 to H-5, F-55 HR-2), the
// public transform for the radii startM and endM (valid values): every sample
// outside the window of StepsOf loses its position. Only latitude and
// longitude change; every other field of w stays as it is. With both radii 0
// w is returned unchanged.
//
// fullyHidden reports that w had positions and the cut left none; the result
// then has no position streams. A workout without positions is returned
// unchanged with fullyHidden false (indoor, H-5). w is not modified.
func Hide(w Workout, startM, endM int) (out Workout, fullyHidden bool) {
	if startM == 0 && endM == 0 {
		return w, false
	}
	h := StepsOf(w)
	if h == nil {
		return w, false
	}
	from, to, fully := h.Window(startM, endM)
	if fully {
		w.Samples.Lat, w.Samples.Lon = nil, nil
		return w, true
	}
	s := w.Samples
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
