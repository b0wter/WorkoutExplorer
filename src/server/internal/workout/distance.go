package workout

import "math"

// earthRadius is the mean Earth radius in metres used by haversine (R-6).
const earthRadius = 6371008.8

// haversine returns the great-circle distance in metres between two points
// given in degrees.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	φ1, φ2 := lat1*rad, lat2*rad
	dφ := (lat2 - lat1) * rad
	dλ := (lon2 - lon1) * rad
	a := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	if a > 1 {
		a = 1
	}
	return 2 * earthRadius * math.Asin(math.Sqrt(a))
}

// computeDistance implements R-6 (b): without a device distance stream but
// with positions, distance is the cumulative haversine distance between
// consecutive positioned samples. Samples before the first fix get 0;
// samples without a position carry the previous value; after a dropout the
// straight line from the last to the next fix is added. Device distance
// (R-6 a) and the absence of both (R-6 c) leave s unchanged.
func computeDistance(s *Samples) {
	if s.Distance != nil || s.Lat == nil {
		return
	}
	d := make([]float64, len(s.Offset))
	total := 0.0
	prev := -1
	for i := range d {
		if !math.IsNaN(s.Lat[i]) {
			if prev >= 0 {
				total += haversine(s.Lat[prev], s.Lon[prev], s.Lat[i], s.Lon[i])
			}
			prev = i
		}
		d[i] = total
	}
	s.Distance = d
}

// holdDistance implements R-14 (F-08 v1.4, D-25, QA-08-r3-2): the device
// distance stream never falls. Each valid value becomes the largest valid
// value up to and including it; NaN stays NaN and neither resets nor
// advances the high. It runs right after R-3, in place on the workout's own
// stream, before R-13, R-4 and R-6b. Without a distance stream it does
// nothing. A non-decreasing stream is left unchanged bit for bit.
func holdDistance(s *Samples) {
	high := math.NaN()
	for i, d := range s.Distance {
		switch {
		case math.IsNaN(d):
		case !math.IsNaN(high) && d < high:
			s.Distance[i] = high
		default:
			high = d
		}
	}
}
