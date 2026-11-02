package workout

import "math"

// Gap filling (R-3): gaps of 2 to 4 s are filled at 1 s; 5 s or more stay
// gaps (TD: "Resample smart recording to 1 s only across gaps < 5 s").
const (
	minFillGap = 2 // seconds between consecutive samples
	maxFillGap = 4
	// gapSeconds is the smallest distance between two samples that counts
	// as a gap (Technical Design, Glossary "Gap").
	gapSeconds = maxFillGap + 1
)

// fillGaps implements R-3. Inserted values are the linear interpolation
// between the two neighbours, or NaN if either neighbour is NaN for that
// stream. It sets s.Filled.
func fillGaps(s *Samples) {
	n := len(s.Offset)
	extra := 0
	for i := 0; i+1 < n; i++ {
		if d := s.Offset[i+1] - s.Offset[i]; d >= minFillGap && d <= maxFillGap {
			extra += int(d) - 1
		}
	}
	s.Filled = extra
	if extra == 0 {
		return
	}

	ptrs := s.streams()
	offsets := make([]int32, 0, n+extra)
	inserted := make([]bool, 0, n+extra)
	var out [numStreams][]float64
	for k, p := range ptrs {
		if *p != nil {
			out[k] = make([]float64, 0, n+extra)
		}
	}
	for i := 0; i < n; i++ {
		offsets = append(offsets, s.Offset[i])
		inserted = append(inserted, false)
		for k, p := range ptrs {
			if *p != nil {
				out[k] = append(out[k], (*p)[i])
			}
		}
		if i+1 == n {
			break
		}
		d := s.Offset[i+1] - s.Offset[i]
		if d < minFillGap || d > maxFillGap {
			continue
		}
		for step := int32(1); step < d; step++ {
			offsets = append(offsets, s.Offset[i]+step)
			inserted = append(inserted, true)
			f := float64(step) / float64(d)
			for k, p := range ptrs {
				if *p == nil {
					continue
				}
				a, b := (*p)[i], (*p)[i+1]
				v := math.NaN()
				if !math.IsNaN(a) && !math.IsNaN(b) {
					v = a + (b-a)*f
				}
				out[k] = append(out[k], v)
			}
		}
	}
	s.Offset = offsets
	s.inserted = inserted
	for k, p := range ptrs {
		*p = out[k]
	}
}

// settlePresence implements R-4 and invariant 3: latitude and longitude are
// missing together, and a stream without any valid value becomes nil.
func settlePresence(s *Samples) {
	if s.Lat != nil && s.Lon != nil {
		for i := range s.Lat {
			if math.IsNaN(s.Lat[i]) != math.IsNaN(s.Lon[i]) {
				s.Lat[i], s.Lon[i] = math.NaN(), math.NaN()
			}
		}
	}
	for _, p := range s.streams() {
		if !hasValid(*p) {
			*p = nil
		}
	}
	if s.Lat == nil || s.Lon == nil {
		s.Lat, s.Lon = nil, nil
	}
}

// hasValid reports whether v contains at least one non-NaN value.
func hasValid(v []float64) bool {
	for _, x := range v {
		if !math.IsNaN(x) {
			return true
		}
	}
	return false
}

// lastValid returns the last non-NaN value of v, or nil.
func lastValid(v []float64) *float64 {
	for i := len(v) - 1; i >= 0; i-- {
		if !math.IsNaN(v[i]) {
			x := v[i]
			return &x
		}
	}
	return nil
}
