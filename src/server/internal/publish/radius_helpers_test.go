package publish

import "workout-explorer/internal/workout"

// radius is the radius of both ends for the old toggle.
func radius(on bool) int {
	if on {
		return 300
	}
	return 0
}

// hiddenRanges is what hiding at 300 m at both ends removes, from the steps
// (ADR-056): nil without positions, [[0, n-1]] fully hidden, else both ends.
func hiddenRanges(w workout.Workout) [][2]int { return hiddenRangesAt(w, 300, 300) }

// hiddenRangesAt is hiddenRanges at the radii a and b.
func hiddenRangesAt(w workout.Workout, a, b int) [][2]int {
	h := workout.StepsOf(w)
	if h == nil {
		return nil
	}
	n := len(w.Samples.Offset)
	from, to, fully := h.Window(a, b)
	if fully {
		return [][2]int{{0, n - 1}}
	}
	return [][2]int{{0, from - 1}, {to + 1, n - 1}}
}
