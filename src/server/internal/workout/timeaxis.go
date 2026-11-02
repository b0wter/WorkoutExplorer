package workout

import (
	"math"
	"sort"
	"time"
)

// Stream indices for loops over all streams, in report order.
const (
	streamLat = iota
	streamLon
	streamAltitude
	streamDistance
	streamSpeed
	streamHeartRate
	streamCadence
	streamPower
	streamTemperature
	numStreams
)

// streams returns pointers to every stream of s, indexed by the stream* constants.
func (s *Samples) streams() [numStreams]*[]float64 {
	return [numStreams]*[]float64{
		&s.Lat, &s.Lon, &s.Altitude, &s.Distance, &s.Speed,
		&s.HeartRate, &s.Cadence, &s.Power, &s.Temperature,
	}
}

// values returns the values of r in stream order; non-finite values become NaN.
func (r RawRecord) values() [numStreams]float64 {
	v := [numStreams]float64{
		r.Lat, r.Lon, r.Altitude, r.Distance, r.Speed,
		r.HeartRate, r.Cadence, r.Power, r.Temperature,
	}
	for i := range v {
		if math.IsInf(v[i], 0) {
			v[i] = math.NaN()
		}
	}
	return v
}

// startTime implements R-1: the earliest valid of the first session's start,
// the first lap's start and the earliest record time; zero if none is valid.
func startTime(rec Recording) time.Time {
	var start time.Time
	consider := func(t time.Time) {
		if !t.IsZero() && (start.IsZero() || t.Before(start)) {
			start = t
		}
	}
	if len(rec.Sessions) > 0 {
		consider(rec.Sessions[0].Start)
	}
	if len(rec.Laps) > 0 {
		consider(rec.Laps[0].Start)
	}
	for i := range rec.Records {
		consider(rec.Records[i].At)
	}
	if start.IsZero() {
		return time.Time{}
	}
	return start.UTC()
}

// offsetOf returns whole seconds from start to t, clamped to [0, MaxInt32].
// A missing time (zero) or a missing start gives 0.
func offsetOf(start, t time.Time) int32 {
	if start.IsZero() || t.IsZero() {
		return 0
	}
	d := t.Unix() - start.Unix()
	switch {
	case d < 0:
		return 0
	case d > math.MaxInt32:
		return math.MaxInt32
	}
	return int32(d)
}

// spanOffsets returns the start and end offsets of a session or lap. A
// missing start maps to 0; a missing or earlier end maps to the start.
func spanOffsets(start, from, to time.Time) (int32, int32) {
	s := offsetOf(start, from)
	if to.IsZero() {
		return s, s
	}
	e := offsetOf(start, to)
	if e < s {
		e = s
	}
	return s, e
}

// buildTimeAxis implements R-2: records sorted stably by time, records in
// the same second merged (per stream the later record's valid value wins).
// Every stream is allocated; R-4 removes the empty ones.
func buildTimeAxis(records []RawRecord, start time.Time) Samples {
	var s Samples
	if len(records) == 0 || start.IsZero() {
		return s
	}
	sorted := make([]RawRecord, 0, len(records))
	for _, r := range records {
		if !r.At.IsZero() {
			sorted = append(sorted, r)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].At.Before(sorted[j].At) })

	s.Offset = make([]int32, 0, len(sorted))
	ptrs := s.streams()
	for _, p := range ptrs {
		*p = make([]float64, 0, len(sorted))
	}
	for _, r := range sorted {
		off := offsetOf(start, r.At)
		vals := r.values()
		n := len(s.Offset)
		if n > 0 && s.Offset[n-1] == off {
			for k, p := range ptrs {
				if !math.IsNaN(vals[k]) {
					(*p)[n-1] = vals[k]
				}
			}
			continue
		}
		s.Offset = append(s.Offset, off)
		for k, p := range ptrs {
			*p = append(*p, vals[k])
		}
	}
	return s
}
