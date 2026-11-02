package workout

import "math"

// Elevation gain parameters (R-8): one set for barometric and GPS devices,
// because the device's own ascent is used whenever it exists (R-10).
const (
	smoothingSeconds = 2   // centred moving average over ±2 s
	hysteresisMetres = 3.0 // zig-zag threshold H
)

// elevationGain implements R-8: smooth the valid altitudes with a ±2 s
// centred moving average, then sum the climbs of a 3 m zig-zag filter.
// nil with fewer than 2 valid altitudes.
func elevationGain(offsets []int32, alt []float64) *float64 {
	g, _ := elevation(offsets, alt, hysteresisMetres)
	return g
}

// elevation is R-8 for gain and loss (F-08): the same smoothing and the same
// zig-zag pass; the loss sums the confirmed drops. Both are nil under the
// same condition.
func elevation(offsets []int32, alt []float64, h float64) (gain, loss *float64) {
	gain, loss, _ = elevationCum(offsets, alt, h)
	return gain, loss
}

// elevationCum is elevation plus the cumulative gain series G (F-17 B-1,
// ADR-029): G[i] is the gain confirmed by the zig-zag pass up to the sample's
// smoothed altitude, plus the climb in progress, so G never decreases,
// G[0] = 0 and G[N-1] equals the gain. Samples without an altitude carry the
// previous value. The series is nil whenever the gain is nil.
func elevationCum(offsets []int32, alt []float64, h float64) (gain, loss *float64, cum []float64) {
	if alt == nil {
		return nil, nil, nil
	}
	smooth := make([]float64, 0, len(alt))
	for i := range alt {
		if math.IsNaN(alt[i]) {
			continue
		}
		sum, n := 0.0, 0
		for j := i; j >= 0 && offsets[i]-offsets[j] <= smoothingSeconds; j-- {
			if !math.IsNaN(alt[j]) {
				sum += alt[j]
				n++
			}
		}
		for j := i + 1; j < len(alt) && offsets[j]-offsets[i] <= smoothingSeconds; j++ {
			if !math.IsNaN(alt[j]) {
				sum += alt[j]
				n++
			}
		}
		smooth = append(smooth, sum/float64(n))
	}
	if len(smooth) < 2 {
		return nil, nil, nil
	}
	g, l, per := zigZagCum(smooth, h)
	cum = make([]float64, len(alt))
	k, cur := 0, 0.0
	for i := range alt {
		if !math.IsNaN(alt[i]) {
			cur = per[k]
			k++
		}
		cum[i] = cur
	}
	return &g, &l, cum
}

// elevationGainWith is elevationGain with the hysteresis h as a parameter,
// so tests can show the effect of the threshold.
func elevationGainWith(offsets []int32, alt []float64, h float64) *float64 {
	g, _ := elevation(offsets, alt, h)
	return g
}

// zigZag sums the rises and the drops in a with hysteresis h, exactly as in
// the F-02 design, R-8 step 2 (and F-08, R-8 addition).
func zigZag(a []float64, h float64) (gain, loss float64) {
	gain, loss, _ = zigZagCum(a, h)
	return gain, loss
}

// zigZagCum is zigZag plus, per element, the gain so far including the climb
// in progress (cand - ref while rising). Its last element equals gain.
func zigZagCum(a []float64, h float64) (gain, loss float64, cum []float64) {
	cum = make([]float64, len(a))
	lo, hi := a[0], a[0]
	dir := 0
	var ref, cand float64
	for k, x := range a {
		switch dir {
		case 0:
			lo = math.Min(lo, x)
			hi = math.Max(hi, x)
			if x-lo >= h {
				dir, ref, cand = 1, lo, x
			} else if hi-x >= h {
				dir, ref, cand = -1, hi, x
			}
		case 1:
			if x > cand {
				cand = x
			} else if cand-x >= h {
				gain += cand - ref
				ref, cand, dir = cand, x, -1
			}
		case -1:
			if x < cand {
				cand = x
			} else if x-cand >= h {
				loss += ref - cand
				ref, cand, dir = cand, x, 1
			}
		}
		cum[k] = gain
		if dir == 1 {
			cum[k] += cand - ref
		}
	}
	switch dir {
	case 1:
		gain += cand - ref
	case -1:
		loss += ref - cand
	}
	return gain, loss, cum
}
