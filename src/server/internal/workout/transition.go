package workout

import (
	"math"
	"reflect"
)

// R-13 transition legs (F-08 design v1.3, OQ-38; D-21 to D-24).
//
// A transition session is a session whose own sport is "transition"; a
// transition sample is a sample covered by one under rule 1 of the sport
// timeline; a transition lap is a lap that starts in one (D-23: a sport hint
// or a fallback never makes a transition). At those samples and in those
// totals only the allow-list below survives (D-21; user answers 1, 3, 7):
// everything else becomes a missing sample (NaN) or an absent total (nil).
// No sample is removed and the data file format does not change (D-22).

// transitionRule is the R-13 decision for one stream. The zero value means
// "not decided": maskTransitions drops such a stream and
// TestTransitionAllowListComplete fails, so a new stream needs a decision.
type transitionRule uint8

const (
	ruleUndecided transitionRule = iota
	ruleKeep
	ruleDrop
)

// transitionStreams holds the decision for every stream, indexed by the
// stream* constants like Samples.streams().
var transitionStreams = [numStreams]transitionRule{
	streamLat: ruleKeep, streamLon: ruleKeep, // the user's "gps"
	streamAltitude:    ruleKeep, // "elevation"
	streamDistance:    ruleKeep, // answer 1
	streamHeartRate:   ruleKeep,
	streamTemperature: ruleKeep, // answer 3
	streamSpeed:       ruleDrop,
	streamCadence:     ruleDrop,
	streamPower:       ruleDrop,
}

// transitionTotalKept lists every field of Totals the same way. Time totals
// and calories describe the leg, not a pace; they stay (answer 4 drops only
// the rate totals, including the device's own transition session totals).
var transitionTotalKept = map[string]bool{
	"ElapsedTime": true, "TimerTime": true, "Calories": true,
	"Distance": true, "Ascent": true, "Descent": true,
	"AvgHeartRate": true, "MaxHeartRate": true,
	"AvgTemperature": true, "MaxTemperature": true,
	"AvgSpeed": false, "MaxSpeed": false,
	"AvgCadence": false, "MaxCadence": false,
	"AvgPower": false, "MaxPower": false,
}

// maskTransitions applies R-13 to the samples, the transition sessions and
// the transition laps of w. It runs after R-3 (a filled sample is masked like
// a recorded one) and before R-4 (a stream whose only values were in
// transitions becomes absent).
func maskTransitions(w *Workout, tl *sportTimeline) {
	if len(tl.sessions) == 0 {
		return
	}
	for i := range w.Sessions {
		if w.Sessions[i].Sport == sportTransition {
			dropTotals(&w.Sessions[i].Totals)
		}
	}
	for i := range w.Laps {
		if tl.inTransition(w.Laps[i].Start) {
			dropTotals(&w.Laps[i].Totals)
		}
	}

	s := &w.Samples
	var masked []int
	for i, off := range s.Offset {
		if tl.inTransition(off) {
			masked = append(masked, i)
		}
	}
	if len(masked) == 0 {
		return
	}
	for k, p := range s.streams() {
		if *p == nil || transitionStreams[k] == ruleKeep {
			continue
		}
		for _, i := range masked {
			(*p)[i] = math.NaN()
		}
	}
}

// dropTotals sets every Totals field that is not on the allow-list to nil.
func dropTotals(t *Totals) {
	v := reflect.ValueOf(t).Elem()
	ty := v.Type()
	for f := 0; f < ty.NumField(); f++ {
		if !transitionTotalKept[ty.Field(f).Name] {
			v.Field(f).Set(reflect.Zero(v.Field(f).Type()))
		}
	}
}
