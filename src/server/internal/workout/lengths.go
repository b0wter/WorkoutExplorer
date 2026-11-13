package workout

import (
	"math"
	"sort"
)

// isPoolSwim is F-44 PS-1: a lap-swimming session.
func isPoolSwim(s Session) bool { return s.SubSport == "lap_swimming" }

// sessionLookup returns a function that gives the index of the session whose
// span holds offset t, or -1 (F-48 PL-2). Binary search over the sessions by
// start; of overlapping sessions the latest-starting one is asked.
func sessionLookup(ss []Session) func(t int32) int {
	idx := make([]int, len(ss))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return ss[idx[a]].Start < ss[idx[b]].Start })
	return func(t int32) int {
		k := sort.Search(len(idx), func(i int) bool { return ss[idx[i]].Start > t }) - 1
		if k < 0 || ss[idx[k]].End < t {
			return -1
		}
		return idx[k]
	}
}

// scopeLengths applies F-48 PL-2 and PL-3: lap strokes are cleared and
// lengths dropped inside a session that is not a pool swim; lengths no
// session covers are kept (OQ-185). Lengths are ordered by start.
func scopeLengths(w *Workout, raw []RawLength) {
	at := sessionLookup(w.Sessions)
	for i := range w.Laps {
		if k := at(w.Laps[i].Start); k >= 0 && !isPoolSwim(w.Sessions[k]) {
			w.Laps[i].Stroke = ""
		}
	}
	w.Lengths = make([]Length, 0, len(raw))
	for _, r := range raw {
		start, end := spanOffsets(w.StartTime, r.Start, r.End)
		k := at(start)
		if k >= 0 && !isPoolSwim(w.Sessions[k]) {
			continue
		}
		pool := 0.0
		if k >= 0 {
			w.Sessions[k].hasLengths = true
			pool = w.Sessions[k].Pool.Length
		}
		l := Length{Start: start, End: end, Rest: r.Rest}
		if !math.IsNaN(r.Time) {
			t := r.Time
			l.Time = &t
		}
		if !r.Rest {
			l.Stroke, l.Strokes = r.Stroke, r.Strokes
			switch {
			case pool > 0:
				if l.Time != nil && *l.Time > 0 {
					v := pool / *l.Time
					l.AvgSpeed = &v
				}
			case r.AvgSpeed > 0: // NaN compares false
				v := r.AvgSpeed
				l.AvgSpeed = &v
			}
		}
		w.Lengths = append(w.Lengths, l)
	}
	sort.SliceStable(w.Lengths, func(i, j int) bool { return w.Lengths[i].Start < w.Lengths[j].Start })
}

// summaryStroke is F-48 PL-4: per pool swim the strokes of its active
// lengths when it had length messages, else its session's stroke; the code
// when there is at least one and all are one known stroke other than mixed
// and im, else "".
func summaryStroke(w *Workout) string {
	at := sessionLookup(w.Sessions)
	var strokes []string
	for _, s := range w.Sessions {
		if isPoolSwim(s) && !s.hasLengths {
			strokes = append(strokes, s.Stroke)
		}
	}
	for _, l := range w.Lengths {
		if k := at(l.Start); k >= 0 && isPoolSwim(w.Sessions[k]) && !l.Rest {
			strokes = append(strokes, l.Stroke)
		}
	}
	if len(strokes) == 0 {
		return ""
	}
	for _, s := range strokes {
		if s != strokes[0] {
			return ""
		}
	}
	switch strokes[0] {
	case "", "mixed", "im":
		return ""
	}
	return strokes[0]
}

// trimmedLengths is F-48 PL-6 (OQ-181): after a trim, the number of
// remaining active lengths of the pool swims; nil when that is 0 or a
// remaining pool swim had no length messages.
func trimmedLengths(w *Workout) *float64 {
	at := sessionLookup(w.Sessions)
	for _, s := range w.Sessions {
		if isPoolSwim(s) && !s.hasLengths {
			return nil
		}
	}
	n := 0.0
	for _, l := range w.Lengths {
		if k := at(l.Start); k >= 0 && isPoolSwim(w.Sessions[k]) && !l.Rest {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	return &n
}
