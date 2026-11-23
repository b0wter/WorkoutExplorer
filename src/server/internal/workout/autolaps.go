package workout

// Remove auto-laps (F-66 RA-3, RA-5). Runs on the whole normalised workout,
// before the added-boundary merge (AddLaps), the trim and hiding; the browser
// mirrors it over draft.bin.

// IsAutoLap is OQ-313: a device lap ended by distance, time, position, or by
// nothing the parser knows.
func IsAutoLap(trigger string) bool {
	switch trigger {
	case "distance", "time", "position", "":
		return true
	}
	return false
}

// RemoveAutoLaps removes the end boundary of every auto-lap that has a next
// device lap in the same leg (MS-3: the last session starting at or before
// the lap's first sample). A run of such laps and the lap after it become one
// lap: first start, latest end, trigger of the last part, the RA-5 stroke,
// totals computed as for a range. Other laps stay as they were. If a
// boundary was removed and one lap is left, the laps list is empty (OQ-307).
// w is not modified; samples are shared.
func RemoveAutoLaps(w Workout) Workout {
	off := w.Samples.Offset
	leg := func(l Lap) int {
		from, _ := SampleRange(off, l.Start, l.End)
		at := l.Start
		if from < len(off) {
			at = off[from]
		}
		k := -1
		for i, s := range w.Sessions {
			if s.Start <= at {
				k = i
			}
		}
		return k
	}
	var tl *sportTimeline
	out := make([]Lap, 0, len(w.Laps))
	removed := false
	for i := 0; i < len(w.Laps); {
		j := i
		for j+1 < len(w.Laps) && IsAutoLap(w.Laps[j].Trigger) && leg(w.Laps[j]) == leg(w.Laps[j+1]) {
			j++
		}
		if j == i {
			out = append(out, w.Laps[i])
			i++
			continue
		}
		removed = true
		if tl == nil {
			tl = newSportTimeline(w.Sessions, w.hints)
		}
		m := Lap{Start: w.Laps[i].Start, End: w.Laps[i].End, Trigger: w.Laps[j].Trigger}
		for _, p := range w.Laps[i : j+1] {
			m.End = max(m.End, p.End)
			switch {
			case p.Stroke == "" || p.Stroke == m.Stroke:
			case m.Stroke == "":
				m.Stroke = p.Stroke
			default:
				m.Stroke = "mixed"
			}
		}
		completeTotals(&w.Samples, &m.Totals, m.Start, m.End, tl)
		out = append(out, m)
		i = j + 1
	}
	if !removed {
		return w
	}
	if len(out) == 1 {
		out = nil
	}
	w.Laps = out
	return w
}
