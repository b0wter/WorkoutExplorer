package workout

// stepSports count cadence in steps per minute; FIT records them per leg
// (R-5, D-8).
var stepSports = map[string]bool{"running": true, "walking": true, "hiking": true}

// convertCadence implements R-5: cadence of running, walking and hiking is
// doubled (steps per minute), in the samples and in the session and lap
// averages and maxima. A sample's sport comes from the sport timeline (D-12),
// so a truncated file without sessions is converted through its sport hints.
func convertCadence(w *Workout, tl *sportTimeline) {
	for i := range w.Samples.Cadence {
		if stepSports[tl.at(w.Samples.Offset[i])] {
			w.Samples.Cadence[i] *= 2 // NaN stays NaN
		}
	}
	for i := range w.Sessions {
		if stepSports[w.Sessions[i].Sport] {
			doubleCadence(&w.Sessions[i].Totals)
		}
	}
	for i := range w.Laps {
		if stepSports[tl.at(w.Laps[i].Start)] {
			doubleCadence(&w.Laps[i].Totals)
		}
	}
}

func doubleCadence(t *Totals) {
	t.AvgCadence = scaled(t.AvgCadence, 2)
	t.MaxCadence = scaled(t.MaxCadence, 2)
}

func scaled(p *float64, f float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p * f
	return &v
}
