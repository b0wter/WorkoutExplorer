package workout

import (
	"math"
	"strings"
)

// Sensors is the set of recognised external sensor kinds of a workout
// (F-43 DI-5); ingest maps the FIT profile onto it.
type Sensors uint8

// Sensor kinds, in the order the page lists them (F-43 DI-6).
const (
	SensorHeartRate Sensors = 1 << iota
	SensorPower
	SensorSpeed
	SensorCadence
	SensorSpeedCadence
	SensorFootPod
	SensorRadar
)

// sensorCodes are the data file codes in bit order (F-03 §5.7).
var sensorCodes = [...]string{
	"heart_rate_monitor", "power_meter", "speed_sensor", "cadence_sensor",
	"speed_cadence_sensor", "foot_pod", "radar",
}

// Codes returns the data file codes of s in the fixed order; never nil.
func (s Sensors) Codes() []string {
	out := []string{}
	for i, c := range sensorCodes {
		if s&(1<<i) != 0 {
			out = append(out, c)
		}
	}
	return out
}

// Name is the device's display name (F-43 DI-4): "{brand} {model}" unless
// the model's first word already is the brand's first word; else the model;
// else the brand; else "".
func (d Device) Name() string {
	switch {
	case d.Brand == "" || d.Model == "":
		return d.Model + d.Brand // at most one is set
	case strings.EqualFold(firstWord(d.Model), firstWord(d.Brand)):
		return d.Model
	}
	return d.Brand + " " + d.Model
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

// metresPerYard converts a pool length to yards (F-44 PS-3).
const metresPerYard = 0.9144

// poolSummary is F-44 PS-3 over the pool swims (sub-sport lap_swimming):
// the pool length when every pool swim recorded the same one in whole
// metres or yards, and the lengths when every pool swim recorded them.
func poolSummary(sessions []Session) (metres, yards, lengths *float64) {
	n, sum := 0, 0.0
	allLengths, sameLength := true, true
	var length float64
	var statute bool
	for _, s := range sessions {
		if s.SubSport != "lap_swimming" {
			continue
		}
		v := s.Pool.Length
		if s.Pool.Statute {
			v /= metresPerYard
		}
		v = math.Round(v) // half away from zero
		if v == 0 || (n > 0 && (v != length || s.Pool.Statute != statute)) {
			sameLength = false
		}
		length, statute = v, s.Pool.Statute
		if s.Pool.Lengths == 0 {
			allLengths = false
		}
		sum += float64(s.Pool.Lengths)
		n++
	}
	if n == 0 {
		return nil, nil, nil
	}
	if sameLength {
		if statute {
			yards = &length
		} else {
			metres = &length
		}
	}
	if allLengths {
		lengths = &sum
	}
	return metres, yards, lengths
}
