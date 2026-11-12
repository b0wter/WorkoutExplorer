// Package datafile encodes a normalised workout as the binary data file the
// browser reads (format version 4, magic WEXD) and decodes it again.
//
// The normative format description is architect/F-03-workout-data-file-writer.md,
// section "Data file format, version 4"; the decisions behind it are in
// ADR-005. The package is pure: it depends on package workout and the
// standard library only, does no I/O and never logs (Technical Design,
// Code structure: Dependency rules).
package datafile

import (
	"encoding/json"
	"math"

	"workout-explorer/internal/workout"
)

// Magic is the first four bytes of every data file (Format §1).
const Magic = "WEXD"

// Version is the only format version this package writes and reads
// (Format §9, ADR-005 item 9).
const Version uint16 = 4

// headerSize is magic, version, sample count and metadata length (Format §1).
const headerSize = 14

// maxAbs bounds every absolute value in a delta column, so no delta can
// reach the sentinel or overflow int32 (ADR-005 item 4).
const maxAbs = 1_000_000_000

// File is a decoded data file.
type File struct {
	Version        uint16
	SampleCount    int
	MetadataLength int
	Meta           Meta
	Streams        []Stream // descriptor order
}

// Stream is one decoded column.
type Stream struct {
	Descriptor Descriptor
	Ints       []int64 // decoded absolute quantised values; 0 where missing
	Valid      []bool
}

// Values returns the decoded values in the descriptor's unit: Ints[i] ×
// scale, NaN where the sample is missing.
func (s Stream) Values() []float64 {
	scale := 0.0
	if spec, ok := specByName(s.Descriptor.Name); ok {
		scale = spec.scale
	}
	out := make([]float64, len(s.Ints))
	for i, v := range s.Ints {
		if i < len(s.Valid) && s.Valid[i] {
			out[i] = float64(v) * scale
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

// Meta is the metadata JSON (Format §5). JSON key order is field order.
type Meta struct {
	Sport     string       `json:"sport"`
	StartTime string       `json:"start_time"`
	TimeZone  TimeZone     `json:"time_zone"`
	Summary   Summary      `json:"summary"`
	Device    Device       `json:"device"` // version 3, §5.7
	Sessions  []Session    `json:"sessions"`
	Laps      []Lap        `json:"laps"`
	Streams   []Descriptor `json:"streams"`
}

// TimeZone is the workout's fixed UTC offset (Format §5.1).
type TimeZone struct {
	Known         bool  `json:"known"`
	OffsetSeconds int32 `json:"offset_seconds"`
}

// Totals are device values as recorded (Format §5.2). An empty field means
// "not recorded" and is omitted.
type Totals struct {
	ElapsedTime    json.Number `json:"elapsed_time,omitempty"`
	TimerTime      json.Number `json:"timer_time,omitempty"`
	Distance       json.Number `json:"distance,omitempty"`
	Ascent         json.Number `json:"ascent,omitempty"`
	Descent        json.Number `json:"descent,omitempty"`
	Calories       json.Number `json:"calories,omitempty"`
	AvgSpeed       json.Number `json:"avg_speed,omitempty"`
	MaxSpeed       json.Number `json:"max_speed,omitempty"`
	AvgHeartRate   json.Number `json:"avg_heart_rate,omitempty"`
	MaxHeartRate   json.Number `json:"max_heart_rate,omitempty"`
	AvgCadence     json.Number `json:"avg_cadence,omitempty"`
	MaxCadence     json.Number `json:"max_cadence,omitempty"`
	AvgPower       json.Number `json:"avg_power,omitempty"`
	MaxPower       json.Number `json:"max_power,omitempty"`
	AvgTemperature json.Number `json:"avg_temperature,omitempty"`
	MaxTemperature json.Number `json:"max_temperature,omitempty"`
}

// Summary is the workout summary: Totals plus moving_time and (version 3)
// the pool keys (Format §5.2).
type Summary struct {
	Totals
	MovingTime   json.Number `json:"moving_time,omitempty"`
	PoolLengthM  json.Number `json:"pool_length_m,omitempty"`
	PoolLengthYd json.Number `json:"pool_length_yd,omitempty"`
	Lengths      json.Number `json:"lengths,omitempty"`
	// Version 4 (ADR-046 item 3): used only to find whole lengths in the distance stream.
	PoolLengthRecordedM json.Number `json:"pool_length_recorded_m,omitempty"`
}

// Device is the device line's data (Format §5.7): the display name ("" =
// none) and the sensor codes in their fixed order.
type Device struct {
	Name    string   `json:"name"`
	Sensors []string `json:"sensors"`
}

// Session is a session with its sample range (Format §5.3, §5.4).
type Session struct {
	Sport      string `json:"sport"`
	SubSport   string `json:"sub_sport"`
	Start      int32  `json:"start"`
	End        int32  `json:"end"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	Totals     Totals `json:"totals"`
}

// Lap is a lap with its sample range (Format §5.3, §5.4).
type Lap struct {
	Start      int32  `json:"start"`
	End        int32  `json:"end"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	Trigger    string `json:"trigger"` // version 4, §5.3
	Totals     Totals `json:"totals"`
}

// Descriptor describes one column (Format §5.5).
type Descriptor struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Encoding   string      `json:"encoding"`
	Scale      json.Number `json:"scale"`
	Unit       string      `json:"unit"`
	ByteOffset int         `json:"byte_offset"`
}

// Column types and encodings.
const (
	typeInt32  = "int32"
	typeUint16 = "uint16"
	typeUint8  = "uint8"
	typeInt8   = "int8"

	encDelta = "delta"
	encRaw   = "raw"
)

// streamSpec is one row of the Format §2 table.
type streamSpec struct {
	name     string
	typ      string
	encoding string
	scaleStr string  // as written in the descriptor (Format §5.5)
	scale    float64 // multiplier from stored integer to unit
	factor   float64 // 1/scale as an exact integer constant (W-4)
	unit     string
	min, max int64 // valid stored range
	sentinel int64 // missing sample
	size     int   // bytes per sample
	samples  func(*workout.Samples) []float64
}

// specs is the Format §2 table in column order. time_offset has no float
// accessor: Encode reads Samples.Offset directly.
var specs = []streamSpec{
	{"time_offset", typeInt32, encDelta, "1", 1, 1, "s", 0, maxAbs, math.MaxInt32, 4, nil},
	{"latitude", typeInt32, encDelta, "1e-06", 1e-6, 1e6, "deg", -90_000_000, 90_000_000, math.MaxInt32, 4,
		func(s *workout.Samples) []float64 { return s.Lat }},
	{"longitude", typeInt32, encDelta, "1e-06", 1e-6, 1e6, "deg", -180_000_000, 180_000_000, math.MaxInt32, 4,
		func(s *workout.Samples) []float64 { return s.Lon }},
	{"altitude", typeInt32, encDelta, "0.1", 0.1, 10, "m", -maxAbs, maxAbs, math.MaxInt32, 4,
		func(s *workout.Samples) []float64 { return s.Altitude }},
	{"distance", typeInt32, encDelta, "0.1", 0.1, 10, "m", -maxAbs, maxAbs, math.MaxInt32, 4,
		func(s *workout.Samples) []float64 { return s.Distance }},
	{"speed", typeUint16, encRaw, "0.001", 0.001, 1000, "m/s", 0, 65534, 65535, 2,
		func(s *workout.Samples) []float64 { return s.Speed }},
	{"heart_rate", typeUint8, encRaw, "1", 1, 1, "bpm", 0, 254, 255, 1,
		func(s *workout.Samples) []float64 { return s.HeartRate }},
	{"cadence", typeUint8, encRaw, "1", 1, 1, "1/min", 0, 254, 255, 1,
		func(s *workout.Samples) []float64 { return s.Cadence }},
	{"power", typeUint16, encRaw, "1", 1, 1, "W", 0, 65534, 65535, 2,
		func(s *workout.Samples) []float64 { return s.Power }},
	{"temperature", typeInt8, encRaw, "1", 1, 1, "degC", -128, 126, 127, 1,
		func(s *workout.Samples) []float64 { return s.Temperature }},
	// Version 2 (ADR-029): cumulative moving time and gain, never charted.
	{"moving_time", typeInt32, encDelta, "1", 1, 1, "s", 0, maxAbs, math.MaxInt32, 4,
		func(s *workout.Samples) []float64 { return s.MovingTime }},
	{"elevation_gain", typeInt32, encDelta, "0.1", 0.1, 10, "m", 0, maxAbs, math.MaxInt32, 4,
		func(s *workout.Samples) []float64 { return s.Gain }},
}

// Indices into specs used by the pairing rule (W-3).
const (
	specTime = 0
	specLat  = 1
	specLon  = 2
)

// specIndex returns the position of a stream name in the §2 column order.
func specIndex(name string) (int, bool) {
	for k, s := range specs {
		if s.name == name {
			return k, true
		}
	}
	return 0, false
}

func specByName(name string) (streamSpec, bool) {
	for _, s := range specs {
		if s.name == name {
			return s, true
		}
	}
	return streamSpec{}, false
}

// align4 returns the smallest multiple of 4 that is >= x.
func align4(x int) int { return (x + 3) &^ 3 }
