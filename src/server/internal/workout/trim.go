package workout

import (
	"slices"
	"sort"
	"time"
)

// MinTrimSeconds is the least that must remain after a trim (UP-6).
const MinTrimSeconds = 60

// TrimWindow is the window rule of F-47 TR-4: a is the first sample at or
// after start, b the last at or before end (nil: that end is not trimmed). It
// rounds inward, never to a sample of the trimmed part. ok is false when
// fewer than MinTrimSeconds remain or the workout has no start time.
func TrimWindow(w Workout, start, end *time.Time) (a, b int, ok bool) {
	off := w.Samples.Offset
	n := len(off)
	if n < 2 || w.StartTime.IsZero() {
		return 0, 0, false
	}
	a, b = 0, n-1
	base := w.StartTime.Unix()
	if start != nil {
		t := start.Unix() - base
		a = sort.Search(n, func(i int) bool { return int64(off[i]) >= t })
	}
	if end != nil {
		t := end.Unix() - base
		b = sort.Search(n, func(i int) bool { return int64(off[i]) > t }) - 1
	}
	if a >= n || b < 0 || a >= b {
		return 0, 0, false
	}
	return a, b, off[b]-off[a] >= MinTrimSeconds
}

// Trim is the cut of F-47 TR-1 and TR-5 (ADR-047): the samples a to b of the
// normalised workout, everything rebuilt from them so that no value of the
// trimmed part is left. It is the identity when the window is the whole
// workout. w is not modified.
func Trim(w Workout, a, b int) Workout {
	s := w.Samples
	n := len(s.Offset)
	if a == 0 && b == n-1 {
		return w
	}
	oa, ob := s.Offset[a], s.Offset[b]
	out := w
	out.StartTime = w.StartTime.Add(time.Duration(oa) * time.Second)

	// Samples: sliced, offsets and distance rebased; the cumulative series
	// are rebased by their value at a (ADR-029).
	ns := Samples{Offset: make([]int32, b-a+1)}
	for i := range ns.Offset {
		ns.Offset[i] = s.Offset[a+i] - oa
	}
	src, dst := s.streams(), ns.streams()
	for k := range src {
		if *src[k] != nil {
			*dst[k] = (*src[k])[a : b+1] // shares w's arrays: no second copy (TR-10)
		}
	}
	if ns.Distance != nil {
		ns.Distance = slices.Clone(ns.Distance)
		if ref, _, ok := carriedDistance(s.Distance, a, a); ok {
			for i := range ns.Distance {
				ns.Distance[i] -= ref
			}
		}
	}
	if s.inserted != nil {
		ns.inserted = s.inserted[a : b+1]
		for _, x := range ns.inserted {
			if x {
				ns.Filled++
			}
		}
	}
	settlePresence(&ns)

	// Sessions and laps: clipped; those with nothing left are gone.
	inside := func(start, end int32) bool { return start >= oa && end <= ob }
	shift := func(start, end int32) (int32, int32) { return max(start, oa) - oa, min(end, ob) - oa }
	out.Sessions = nil
	var cutSessions []bool
	for _, x := range w.Sessions {
		from, to := SampleRange(s.Offset, x.Start, x.End)
		cut := !inside(x.Start, x.End)
		if cut && max(from, a) >= min(to, b+1) {
			continue
		}
		x.Start, x.End = shift(x.Start, x.End)
		x.Pool.Lengths = 0 // OQ-179: the pool length stays, the device count goes; trimmedLengths recounts (OQ-181)
		if cut {
			x.Totals = Totals{}
		}
		out.Sessions = append(out.Sessions, x)
		cutSessions = append(cutSessions, cut)
	}
	out.Laps = nil
	for _, x := range w.Laps {
		cut := !inside(x.Start, x.End)
		if cut && min(x.End, ob)-max(x.Start, oa) <= 0 {
			continue
		}
		x.Start, x.End = shift(x.Start, x.End)
		if cut {
			x.Totals = Totals{}
		}
		out.Laps = append(out.Laps, x)
	}

	// F-48 PL-6 (OQ-180): only whole lengths inside the window stay.
	out.Lengths = nil
	for _, x := range w.Lengths {
		if inside(x.Start, x.End) {
			x.Start, x.End = x.Start-oa, x.End-oa
			out.Lengths = append(out.Lengths, x)
		}
	}

	// Timer events: the state at the cut becomes a first event at 0.
	out.Timer = nil
	if len(w.Timer) > 0 {
		ev := slices.Clone(w.Timer)
		sort.SliceStable(ev, func(i, j int) bool { return ev[i].Offset < ev[j].Offset })
		running := ev[0].Kind == TimerStop
		var rest []TimerEvent
		for _, e := range ev {
			switch {
			case e.Offset <= oa:
				running = e.Kind == TimerStart
			case e.Offset <= ob:
				rest = append(rest, TimerEvent{Offset: e.Offset - oa, Kind: e.Kind})
			}
		}
		out.Timer = append(out.Timer, TimerEvent{Kind: TimerStart})
		if !running {
			out.Timer = append(out.Timer, TimerEvent{Kind: TimerStop})
		}
		out.Timer = append(out.Timer, rest...)
	}

	// The sport timeline of the remaining part. Hints of the trimmed start go
	// unless no session is left (then they are the only sport source).
	out.hints = nil
	for _, h := range w.hints {
		if h.offset >= oa || len(out.Sessions) == 0 {
			h.offset -= oa
			out.hints = append(out.hints, h)
		}
	}
	tl := newSportTimeline(out.Sessions, out.hints)

	// Derived values over the remaining samples (R-6 to R-8, R-11).
	out.Derived = Derived{}
	e := float64(ns.Offset[len(ns.Offset)-1])
	out.Derived.ElapsedTime = &e
	out.Derived.Distance = lastValid(ns.Distance)
	out.Derived.MovingTime, out.Derived.MovingTimeSource, ns.MovingTime = movingTime(out.Timer, ns, tl)
	if s.Gain != nil && rangeMean(s.Altitude, a, b, false) != nil {
		ns.Gain = make([]float64, b-a+1)
		for i := range ns.Gain {
			ns.Gain[i] = s.Gain[a+i] - s.Gain[a]
		}
		g := ns.Gain[len(ns.Gain)-1]
		out.Derived.ElevationGain = &g
	}
	out.Samples = ns
	recordTotals(&out)

	// Sport and summary: no device total of a session is used (TR-5).
	out.Integrity.DataEnd = 0
	if out.Integrity.Truncated {
		out.Integrity.DataEnd = ns.Offset[len(ns.Offset)-1]
	}
	bare := out
	bare.Sessions = slices.Clone(out.Sessions)
	for i := range bare.Sessions {
		bare.Sessions[i].Totals = Totals{}
	}
	legs, sports := legsOf(&bare, tl)
	out.Sport = workoutSport(sports)
	out.Summary = summarise(legs, sports, out.Derived, false)
	out.Summary.PoolRecordedM = nil
	setPool(&out)
	out.Summary.Lengths = trimmedLengths(&out) // OQ-181
	for i := range out.Sessions {
		if !cutSessions[i] {
			continue
		}
		completeTotals(&out.Samples, &out.Sessions[i].Totals, out.Sessions[i].Start, out.Sessions[i].End, tl)
	}
	completeLaps(&out, tl)
	return out
}
