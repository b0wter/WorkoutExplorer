package publish

import (
	"context"
	"slices"
	"time"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/workout"
)

// Added lap boundaries (F-65 AL-2 to AL-6, ADR-060). The owner check and the
// public build resolve stored instants the same way, on the whole
// normalised workout: nearest sample (PN-3), then the pool snap (AL-4).

// resolveLap is AL-2's resolution of one instant: ok is false outside the
// recording.
func resolveLap(w workout.Workout, t time.Time) (int, bool) {
	i, ok := pinGrid{off: w.Samples.Offset, base: w.StartTime.Unix()}.resolve(t)
	if !ok {
		return 0, false
	}
	return workout.SnapLapBoundary(w, i), true
}

// window is the trim window [lo, hi] of r on w (the whole recording without
// a trim); ok is false when the trim no longer fits.
func window(w workout.Workout, r lifecycle.Record) (lo, hi int, ok bool) {
	if r.TrimStartAt == nil && r.TrimEndAt == nil {
		return 0, len(w.Samples.Offset) - 1, true
	}
	return workout.TrimWindow(w, r.TrimStartAt, r.TrimEndAt)
}

// deviceLaps is F-66 RA-4's step before the added boundaries: the laps after
// the removal of auto-laps when r asks for it.
func deviceLaps(w workout.Workout, r lifecycle.Record) workout.Workout {
	if r.RemoveAutoLaps {
		return workout.RemoveAutoLaps(w)
	}
	return w
}

// withAddedLaps is AL-6's step before the trim: r's boundaries split w's
// laps. A stored instant that no longer fits is skipped (AL-3).
func withAddedLaps(w workout.Workout, r lifecycle.Record) workout.Workout {
	if len(r.AddedLapBoundaries) == 0 {
		return w
	}
	lo, hi, ok := window(w, r)
	if !ok { // trimmed() fails the build
		return w
	}
	at := make([]int, 0, len(r.AddedLapBoundaries))
	for _, t := range r.AddedLapBoundaries {
		if i, ok := resolveLap(w, t); ok {
			at = append(at, i)
		}
	}
	return workout.AddLaps(w, at, lo, hi)
}

// CheckEdits implements lifecycle.Trimmer: from one parse, PN-4's verdict on
// each pin against r's trim and hiding, and AL-2's on each added lap
// boundary against r's trim. Positions play no part in the latter.
func (a *Area) CheckEdits(ctx context.Context, r lifecycle.Record, pins []lifecycle.Pin, laps []time.Time) ([]lifecycle.PinCheck, []lifecycle.LapCheck, error) {
	w, err := a.parseOriginal(ctx, r)
	if err != nil {
		return nil, nil, err
	}
	var pcs []lifecycle.PinCheck
	if len(pins) > 0 {
		g, err := a.pinGridFrom(w, r)
		if err != nil {
			return nil, nil, err
		}
		pcs = make([]lifecycle.PinCheck, len(pins))
		for i, p := range pins {
			pcs[i], _, _ = g.judge(p)
		}
	}
	var lcs []lifecycle.LapCheck
	if len(laps) > 0 {
		lo, hi, ok := window(w, r)
		if !ok {
			return nil, nil, lifecycle.ErrTrimTooShort
		}
		ends := workout.LapEnds(deviceLaps(w, r))
		lcs = make([]lifecycle.LapCheck, len(laps))
		for i, t := range laps {
			k, ok := resolveLap(w, t)
			if !ok {
				lcs[i].Outside = true
				continue
			}
			lcs[i].Resolved = w.StartTime.Add(time.Duration(w.Samples.Offset[k]) * time.Second).UTC()
			lcs[i].Kept = k > lo && k < hi
			_, onEnd := slices.BinarySearch(ends, k)
			lcs[i].OnBoundary = onEnd || k == lo || k == hi // the window's ends are boundaries too (AL-3)
		}
	}
	return pcs, lcs, nil
}
