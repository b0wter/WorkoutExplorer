package workout

import "math"

// recordTotals implements R-11 (F-08): totals from the final samples, so the
// server and the browser agree on them. Filled samples (R-3) count like
// recorded ones. A stream that is absent gives nil.
func recordTotals(w *Workout) {
	s, d := w.Samples, &w.Derived

	// Average speed is distance over moving time (OQ-23), not a sample mean.
	if d.Distance != nil && d.MovingTime != nil && *d.MovingTime > 0 {
		v := *d.Distance / *d.MovingTime
		d.AvgSpeed = &v
	}
	_, d.MaxSpeed = meanMax(s.Speed, false)
	d.AvgHeartRate, d.MaxHeartRate = meanMax(s.HeartRate, false)
	d.AvgCadence, d.MaxCadence = meanMax(s.Cadence, true) // zeros excluded from the mean (OQ-23)
	d.AvgPower, d.MaxPower = meanMax(s.Power, false)      // zeros included (OQ-23)
	d.AvgTemperature, d.MaxTemperature = meanMax(s.Temperature, false)
}

// meanMax returns the mean and the maximum of the present values of v. With
// skipZero the mean ignores zeros (the maximum does not); a stream that is
// all zeros then has a maximum but no mean.
func meanMax(v []float64, skipZero bool) (mean, maxV *float64) {
	var sum float64
	n, nz := 0, 0
	best := math.Inf(-1)
	for _, x := range v {
		if math.IsNaN(x) {
			continue
		}
		n++
		best = math.Max(best, x)
		if skipZero && x == 0 {
			continue
		}
		nz++
		sum += x
	}
	if n == 0 {
		return nil, nil
	}
	maxV = &best
	if nz > 0 {
		m := sum / float64(nz)
		mean = &m
	}
	return mean, maxV
}
