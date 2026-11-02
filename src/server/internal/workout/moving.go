package workout

import (
	"math"
	"sort"
)

// Moving-time speed thresholds (R-7). The TD names cycling (1 km/h) and
// running (0.5 km/h); every other sport uses the running value (OQ-4 default).
const (
	cyclingMovingSpeed = 1.0 / 3.6 // m/s
	defaultMovingSpeed = 0.5 / 3.6 // m/s
)

// movingThreshold returns the moving-speed threshold in m/s for a sport.
func movingThreshold(sport string) float64 {
	switch sport {
	case "cycling", "e_biking":
		return cyclingMovingSpeed
	}
	return defaultMovingSpeed
}

// movingTime implements R-7: from timer events if there are any, else from
// speed (or distance over time) against a per-sport threshold; nil without
// either source. It returns the moving time in seconds, its source and the
// cumulative series M (F-17 B-1, ADR-029 rev. 1): M[0] = 0 and, for i >= 1,
// M[i] is the moving time within [0, Offset[i]]; never decreasing, with
// M[N-1] equal to the total.
func movingTime(timer []TimerEvent, s Samples, tl *sportTimeline) (*float64, string, []float64) {
	n := len(s.Offset)
	if len(timer) > 0 {
		merged := mergeSpans(timerSpans(timer, s.Offset, transitionIntervals(s.Offset, tl)))
		total := 0.0
		for _, sp := range merged {
			total += float64(sp.to - sp.from)
		}
		cum := make([]float64, n)
		covered, k := 0.0, 0 // length of merged[:k], clipped at offset 0
		for i, off := range s.Offset {
			for k < len(merged) && merged[k].to <= off {
				covered += float64(max(merged[k].to-max(merged[k].from, 0), 0))
				k++
			}
			cum[i] = covered
			if k < len(merged) && merged[k].from < off {
				cum[i] += float64(off - max(merged[k].from, 0))
			}
			cum[i] = math.Min(cum[i], total)
		}
		// ADR-029 rev. 1 (QA-17-r1-1): M[0] = 0, so moving time before the
		// first sample counts in the first interval. Timer time after the
		// last sample counts in the last one, so that M[N-1] equals the
		// total (R-7). With one sample M[0] = 0 wins: there is no interval.
		if n > 0 {
			cum[0] = 0
			if n > 1 {
				cum[n-1] = total
			}
		}
		return &total, movingTimeFromTimer, cum
	}
	if s.Speed == nil && s.Distance == nil {
		return nil, "", nil
	}
	v := 0.0
	cum := make([]float64, n)
	for i := 1; i < n; i++ {
		dt := s.Offset[i] - s.Offset[i-1]
		if dt < gapSeconds { // gaps count as stopped
			var speed float64
			if s.Speed != nil {
				speed = s.Speed[i]
			} else {
				speed = (s.Distance[i] - s.Distance[i-1]) / float64(dt)
			}
			// R-13.M (user answer 2, OQ-38b: "triathlon counts the total time
			// as result time"): a transition sample counts as moving whatever
			// its speed, which R-13 dropped.
			if tl.inTransition(s.Offset[i]) {
				v += float64(dt)
			} else if !math.IsNaN(speed) && speed >= movingThreshold(tl.at(s.Offset[i])) {
				v += float64(dt)
			}
		}
		cum[i] = v
	}
	return &v, movingTimeFromSpeed, cum
}

// span is the time interval (from, to] in seconds.
type span struct{ from, to int32 }

// transitionIntervals returns the intervals (Offset[i-1], Offset[i]] of every
// transition sample i >= 1 with a step below the gap length (R-13.M).
func transitionIntervals(offsets []int32, tl *sportTimeline) []span {
	var out []span
	for i := 1; i < len(offsets); i++ {
		if offsets[i]-offsets[i-1] < gapSeconds && tl.inTransition(offsets[i]) {
			out = append(out, span{offsets[i-1], offsets[i]})
		}
	}
	return out
}

// timerSpans lists the intervals in which the timer runs, united with the
// extra intervals (R-13.M: transition time counts as moving whatever the
// device's auto-pause did; overlaps are counted once). The walk starts
// stopped; repeated starts or stops are ignored; a first event that is a stop
// assumes an interval from 0; an interval still open closes at the last
// sample offset (0 if that is earlier, or if there are no samples).
func timerSpans(timer []TimerEvent, offsets []int32, extra []span) []span {
	events := append([]TimerEvent(nil), timer...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].Offset < events[j].Offset })

	spans := append([]span(nil), extra...)
	running := false
	var from int32
	if events[0].Kind == TimerStop {
		running, from = true, 0
	}
	for _, e := range events {
		switch {
		case e.Kind == TimerStart && !running:
			running, from = true, e.Offset
		case e.Kind == TimerStop && running:
			spans = append(spans, span{from, e.Offset})
			running = false
		}
	}
	if running && len(offsets) > 0 {
		if last := offsets[len(offsets)-1]; last > from {
			spans = append(spans, span{from, last})
		}
	}
	return spans
}

// mergeSpans returns the union of the spans as sorted, disjoint, non-empty
// spans.
func mergeSpans(spans []span) []span {
	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	var out []span
	for _, sp := range spans {
		if sp.to <= sp.from {
			continue
		}
		if n := len(out); n == 0 || sp.from > out[n-1].to {
			out = append(out, sp)
		} else if sp.to > out[n-1].to {
			out[n-1].to = sp.to
		}
	}
	return out
}
