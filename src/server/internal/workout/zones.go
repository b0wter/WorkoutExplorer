package workout

import "math"

// Valid zone bases (F-49 ZN-4, OQ-195): whole numbers in these ranges. The
// same ranges decide whether a file's basis is usable and whether a sharer's
// value is accepted.
const (
	MinMaxHeartRate   = 100
	MaxMaxHeartRate   = 250
	MinThresholdPower = 50
	MaxThresholdPower = 1000
)

// Highest valid zone bound per kind: the data file's stored range (F-03 §2).
const (
	maxHeartRateBound = 254
	maxPowerBound     = 65534
	maxZones          = 10
)

// Zones are the workout's zone values per kind (F-49 ZN-3, ADR-049).
type Zones struct {
	HeartRate, Power ZoneValues
}

// ZoneValues are a kind's zone values: either From (the lowest whole bpm or W
// of zones 2 … k) or Basis (max heart rate or threshold power for the default
// model). Both zero: no zones of that kind.
type ZoneValues struct {
	From  []int
	Basis int
}

// None reports whether the kind has no zone values.
func (z ZoneValues) None() bool { return len(z.From) == 0 && z.Basis == 0 }

// RawZones are a file's zone candidates in source order (F-49 ZN-1), set by
// ingest: heart rate in bpm, power in W, NaN = FIT invalid.
type RawZones struct {
	// HeartRate, Power: the session-referenced time_in_zone lists in file
	// order, then the hr_zone / power_zone list.
	HeartRate, Power []ZoneList
	// MaxHeartRate: time_in_zone.max_heart_rate (file order), then
	// zones_target.max_heart_rate. ThresholdPower:
	// zones_target.functional_threshold_power, then session.threshold_power.
	MaxHeartRate, ThresholdPower []float64
}

// ZoneList is one boundary list as the file holds it, in bpm or W.
type ZoneList struct {
	Values []float64
	// Tops: hr_zone / power_zone messages, the tops of zones 1 … n.
	// Otherwise time_in_zone, the bottoms of zones 1 … k.
	Tops bool
	// Cap is the time_in_zone heart-rate list's max_heart_rate: a last value
	// equal to it is dropped. NaN: none.
	Cap float64
}

// pickZones is the source order of ZN-3 steps 2 to 4; step 1 (the sharer's
// values) is applied by the public build.
func pickZones(r RawZones) Zones {
	return Zones{
		HeartRate: pickKind(r.HeartRate, r.MaxHeartRate, maxHeartRateBound, MinMaxHeartRate, MaxMaxHeartRate),
		Power:     pickKind(r.Power, r.ThresholdPower, maxPowerBound, MinThresholdPower, MaxThresholdPower),
	}
}

func pickKind(lists []ZoneList, bases []float64, top float64, lo, hi int) ZoneValues {
	for _, l := range lists {
		if from, ok := zoneFrom(l, top); ok {
			return ZoneValues{From: from}
		}
	}
	for _, b := range bases {
		if ValidBasis(b, lo, hi) {
			return ZoneValues{Basis: int(b)}
		}
	}
	return ZoneValues{}
}

// ValidBasis reports whether b is a whole number in [lo, hi].
func ValidBasis(b float64, lo, hi int) bool {
	return b == math.Trunc(b) && b >= float64(lo) && b <= float64(hi)
}

// zoneFrom is ZN-2: the list's `from` values, or false if it is unusable.
func zoneFrom(l ZoneList, top float64) ([]int, bool) {
	v := l.Values
	for len(v) > 0 && math.IsNaN(v[len(v)-1]) {
		v = v[:len(v)-1]
	}
	if !l.Tops && len(v) > 0 && v[len(v)-1] == l.Cap { // NaN Cap never matches
		v = v[:len(v)-1]
	}
	if len(v) < 2 || len(v) > maxZones {
		return nil, false
	}
	for i, x := range v {
		if math.IsNaN(x) || x != math.Trunc(x) || x < 0 || i > 0 && x <= v[i-1] {
			return nil, false
		}
	}
	from := make([]int, 0, len(v)-1)
	for i := range len(v) - 1 {
		f := v[i+1] // bottoms: values 2 … k
		if l.Tops {
			f = v[i] + 1 // tops: the first n − 1 values + 1
		}
		if f < 1 || f > top {
			return nil, false
		}
		from = append(from, int(f))
	}
	return from, true
}
