package workout

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// ReportVersion is bumped whenever the shape of ReportV2 changes (2: F-08,
// integrity and the record totals).
const ReportVersion = 2

// Decimals used in the report and its digests (F-02 design, Golden report).
const (
	decPosition = 7 // latitude, longitude
	decSpeed    = 3
	decAltDist  = 2 // altitude, distance
	decOther    = 2 // heart rate, cadence, power, temperature
	decTotals   = 3 // times and totals
)

// streamNames and streamDecimals are in report order (stream* constants).
var (
	streamNames    = [numStreams]string{"latitude", "longitude", "altitude", "distance", "speed", "heart_rate", "cadence", "power", "temperature"}
	streamDecimals = [numStreams]int{decPosition, decPosition, decAltDist, decAltDist, decSpeed, decOther, decOther, decOther, decOther}
)

// ReportV2 is the golden report of a normalised workout (ADR-004). Field
// order is the JSON key order. Arrays and objects are always written (E-7);
// only absent scalar values are omitted.
type ReportV2 struct {
	ReportVersion int             `json:"report_version"`
	FileType      string          `json:"file_type,omitempty"`
	Device        ReportDevice    `json:"device"`
	Sport         string          `json:"sport,omitempty"`
	StartTime     string          `json:"start_time,omitempty"`
	TimeZone      ReportTimeZone  `json:"time_zone"`
	Integrity     ReportIntegrity `json:"integrity"`
	Sessions      []ReportSession `json:"sessions"`
	Laps          []ReportLap     `json:"laps"`
	TimerEvents   []ReportTimer   `json:"timer_events"`
	Samples       ReportSamples   `json:"samples"`
	Streams       ReportStreams   `json:"streams"`
	Derived       ReportDerived   `json:"derived"`
	Summary       ReportTotals    `json:"summary"`
}

// ReportDevice mirrors Device.
type ReportDevice struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	ProductID    uint16 `json:"product_id,omitempty"`
	ProductName  string `json:"product_name,omitempty"`
}

// ReportTimeZone mirrors TimeZone.
type ReportTimeZone struct {
	Known         bool  `json:"known"`
	OffsetSeconds int32 `json:"offset_seconds"`
}

// ReportIntegrity mirrors Integrity; data_end is written only for a
// truncated file.
type ReportIntegrity struct {
	Truncated        bool   `json:"truncated"`
	DataEnd          *int32 `json:"data_end,omitempty"`
	CRCFailed        bool   `json:"crc_failed"`
	Sequences        int    `json:"sequences"`
	SkippedSequences int    `json:"skipped_sequences"`
	IgnoredBytes     int    `json:"ignored_bytes"`
}

// ReportSession mirrors Session.
type ReportSession struct {
	Sport    string       `json:"sport,omitempty"`
	SubSport string       `json:"sub_sport,omitempty"`
	Start    int32        `json:"start"`
	End      int32        `json:"end"`
	Totals   ReportTotals `json:"totals"`
}

// ReportLap mirrors Lap.
type ReportLap struct {
	Start  int32        `json:"start"`
	End    int32        `json:"end"`
	Totals ReportTotals `json:"totals"`
}

// ReportTimer mirrors TimerEvent.
type ReportTimer struct {
	Offset int32  `json:"offset"`
	Kind   string `json:"kind"`
}

// ReportSamples describes the time axis.
type ReportSamples struct {
	Count         int        `json:"count"`
	LastOffset    *int32     `json:"last_offset,omitempty"`
	Filled        int        `json:"filled"`
	Gaps          ReportGaps `json:"gaps"`
	OffsetsDigest string     `json:"offsets_digest"`
}

// ReportGaps describes the gaps of 5 s or more.
type ReportGaps struct {
	Count          int   `json:"count"`
	TotalSeconds   int64 `json:"total_seconds"`
	LongestSeconds int32 `json:"longest_seconds"`
}

// ReportStreams has one entry per present stream, in fixed order.
type ReportStreams struct {
	Latitude    *ReportStream `json:"latitude,omitempty"`
	Longitude   *ReportStream `json:"longitude,omitempty"`
	Altitude    *ReportStream `json:"altitude,omitempty"`
	Distance    *ReportStream `json:"distance,omitempty"`
	Speed       *ReportStream `json:"speed,omitempty"`
	HeartRate   *ReportStream `json:"heart_rate,omitempty"`
	Cadence     *ReportStream `json:"cadence,omitempty"`
	Power       *ReportStream `json:"power,omitempty"`
	Temperature *ReportStream `json:"temperature,omitempty"`
}

// ReportStream holds statistics over the valid values of a stream and a
// digest of every value.
type ReportStream struct {
	Present int         `json:"present"`
	Missing int         `json:"missing"`
	Min     json.Number `json:"min"`
	Max     json.Number `json:"max"`
	First   json.Number `json:"first"`
	Last    json.Number `json:"last"`
	Mean    json.Number `json:"mean"`
	Digest  string      `json:"digest"`
}

// ReportDerived mirrors Derived.
type ReportDerived struct {
	ElapsedTime      json.Number `json:"elapsed_time,omitempty"`
	MovingTime       json.Number `json:"moving_time,omitempty"`
	MovingTimeSource string      `json:"moving_time_source,omitempty"`
	ElevationGain    json.Number `json:"elevation_gain,omitempty"`
	Distance         json.Number `json:"distance,omitempty"`
	ElevationLoss    json.Number `json:"elevation_loss,omitempty"`
	AvgSpeed         json.Number `json:"avg_speed,omitempty"`
	MaxSpeed         json.Number `json:"max_speed,omitempty"`
	AvgHeartRate     json.Number `json:"avg_heart_rate,omitempty"`
	MaxHeartRate     json.Number `json:"max_heart_rate,omitempty"`
	AvgCadence       json.Number `json:"avg_cadence,omitempty"`
	MaxCadence       json.Number `json:"max_cadence,omitempty"`
	AvgPower         json.Number `json:"avg_power,omitempty"`
	MaxPower         json.Number `json:"max_power,omitempty"`
	AvgTemperature   json.Number `json:"avg_temperature,omitempty"`
	MaxTemperature   json.Number `json:"max_temperature,omitempty"`
}

// ReportTotals mirrors Totals plus the summary's moving time.
type ReportTotals struct {
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
	MovingTime     json.Number `json:"moving_time,omitempty"`
}

// Report builds the golden report of w.
func Report(w Workout) ReportV2 {
	r := ReportV2{
		ReportVersion: ReportVersion,
		FileType:      w.FileType,
		Sport:         w.Sport,
		TimeZone:      ReportTimeZone{Known: w.TimeZone.Known, OffsetSeconds: w.TimeZone.OffsetSeconds},
		Device:        ReportDevice{Manufacturer: w.Device.Manufacturer, ProductID: w.Device.ProductID, ProductName: w.Device.ProductName},
		Sessions:      []ReportSession{},
		Laps:          []ReportLap{},
		TimerEvents:   []ReportTimer{},
	}
	if !w.StartTime.IsZero() {
		r.StartTime = w.StartTime.UTC().Format(time.RFC3339)
	}
	for _, s := range w.Sessions {
		r.Sessions = append(r.Sessions, ReportSession{
			Sport: s.Sport, SubSport: s.SubSport, Start: s.Start, End: s.End, Totals: reportTotals(s.Totals, nil),
		})
	}
	for _, l := range w.Laps {
		r.Laps = append(r.Laps, ReportLap{Start: l.Start, End: l.End, Totals: reportTotals(l.Totals, nil)})
	}
	for _, e := range w.Timer {
		r.TimerEvents = append(r.TimerEvents, ReportTimer{Offset: e.Offset, Kind: timerKindName(e.Kind)})
	}
	r.Samples = reportSamples(w.Samples)
	r.Streams = reportStreams(w.Samples)
	r.Derived = ReportDerived{
		ElapsedTime:      num(w.Derived.ElapsedTime, decTotals),
		MovingTime:       num(w.Derived.MovingTime, decTotals),
		MovingTimeSource: w.Derived.MovingTimeSource,
		ElevationGain:    num(w.Derived.ElevationGain, decTotals),
		Distance:         num(w.Derived.Distance, decTotals),
		ElevationLoss:    num(w.Derived.ElevationLoss, decTotals),
		AvgSpeed:         num(w.Derived.AvgSpeed, decTotals),
		MaxSpeed:         num(w.Derived.MaxSpeed, decTotals),
		AvgHeartRate:     num(w.Derived.AvgHeartRate, decTotals),
		MaxHeartRate:     num(w.Derived.MaxHeartRate, decTotals),
		AvgCadence:       num(w.Derived.AvgCadence, decTotals),
		MaxCadence:       num(w.Derived.MaxCadence, decTotals),
		AvgPower:         num(w.Derived.AvgPower, decTotals),
		MaxPower:         num(w.Derived.MaxPower, decTotals),
		AvgTemperature:   num(w.Derived.AvgTemperature, decTotals),
		MaxTemperature:   num(w.Derived.MaxTemperature, decTotals),
	}
	ig := w.Integrity
	r.Integrity = ReportIntegrity{
		Truncated: ig.Truncated, CRCFailed: ig.CRCFailed, Sequences: ig.Sequences,
		SkippedSequences: ig.SkippedSequences, IgnoredBytes: ig.IgnoredBytes,
	}
	if ig.Truncated {
		end := ig.DataEnd
		r.Integrity.DataEnd = &end
	}
	r.Summary = reportTotals(w.Summary.Totals, w.Summary.MovingTime)
	return r
}

// ReportJSON returns the golden report of w as JSON, indented with two
// spaces and ending in a newline. It is the only producer of report bytes
// (ADR-004).
func ReportJSON(w Workout) ([]byte, error) {
	b, err := json.MarshalIndent(Report(w), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func timerKindName(k TimerKind) string {
	switch k {
	case TimerStart:
		return "start"
	case TimerStop:
		return "stop"
	}
	return "unknown"
}

func reportTotals(t Totals, moving *float64) ReportTotals {
	return ReportTotals{
		ElapsedTime:    num(t.ElapsedTime, decTotals),
		TimerTime:      num(t.TimerTime, decTotals),
		Distance:       num(t.Distance, decTotals),
		Ascent:         num(t.Ascent, decTotals),
		Descent:        num(t.Descent, decTotals),
		Calories:       num(t.Calories, decTotals),
		AvgSpeed:       num(t.AvgSpeed, decTotals),
		MaxSpeed:       num(t.MaxSpeed, decTotals),
		AvgHeartRate:   num(t.AvgHeartRate, decTotals),
		MaxHeartRate:   num(t.MaxHeartRate, decTotals),
		AvgCadence:     num(t.AvgCadence, decTotals),
		MaxCadence:     num(t.MaxCadence, decTotals),
		AvgPower:       num(t.AvgPower, decTotals),
		MaxPower:       num(t.MaxPower, decTotals),
		AvgTemperature: num(t.AvgTemperature, decTotals),
		MaxTemperature: num(t.MaxTemperature, decTotals),
		MovingTime:     num(moving, decTotals),
	}
}

func reportSamples(s Samples) ReportSamples {
	r := ReportSamples{Count: len(s.Offset), Filled: s.Filled}
	if n := len(s.Offset); n > 0 {
		last := s.Offset[n-1]
		r.LastOffset = &last
	}
	h := sha256.New()
	var buf []byte
	for i, off := range s.Offset {
		buf = strconv.AppendInt(buf[:0], int64(off), 10) // integers (E-6)
		buf = append(buf, '\n')
		h.Write(buf)
		if i > 0 {
			if d := off - s.Offset[i-1]; d >= gapSeconds {
				r.Gaps.Count++
				r.Gaps.TotalSeconds += int64(d)
				r.Gaps.LongestSeconds = max(r.Gaps.LongestSeconds, d)
			}
		}
	}
	r.OffsetsDigest = hex.EncodeToString(h.Sum(nil))[:16]
	return r
}

func reportStreams(s Samples) ReportStreams {
	var out [numStreams]*ReportStream
	for k, p := range s.streams() {
		if *p != nil {
			out[k] = reportStream(*p, streamDecimals[k])
		}
	}
	return ReportStreams{
		Latitude: out[streamLat], Longitude: out[streamLon], Altitude: out[streamAltitude],
		Distance: out[streamDistance], Speed: out[streamSpeed], HeartRate: out[streamHeartRate],
		Cadence: out[streamCadence], Power: out[streamPower], Temperature: out[streamTemperature],
	}
}

func reportStream(v []float64, dec int) *ReportStream {
	r := &ReportStream{}
	h := sha256.New()
	var buf []byte
	var minV, maxV, first, last, sum float64
	for _, x := range v {
		buf = appendValue(buf[:0], x, dec)
		buf = append(buf, '\n')
		h.Write(buf)
		if math.IsNaN(x) {
			r.Missing++
			continue
		}
		if r.Present == 0 {
			minV, maxV, first = x, x, x
		}
		minV, maxV, last = math.Min(minV, x), math.Max(maxV, x), x
		sum += x
		r.Present++
	}
	r.Digest = hex.EncodeToString(h.Sum(nil))[:16]
	if r.Present > 0 {
		r.Min = number(minV, dec)
		r.Max = number(maxV, dec)
		r.First = number(first, dec)
		r.Last = number(last, dec)
		r.Mean = number(sum/float64(r.Present), dec)
	}
	return r
}

// num formats an optional value; nil gives "" (omitted).
func num(p *float64, dec int) json.Number {
	if p == nil {
		return ""
	}
	return number(*p, dec)
}

// number formats v with dec fixed decimals; -0 is written as 0. NaN cannot
// occur here (callers pass valid values only).
func number(v float64, dec int) json.Number {
	return json.Number(appendValue(nil, v, dec))
}

// appendValue appends v with dec fixed decimals, "-" for NaN, and without
// the sign of a value that rounds to zero.
func appendValue(b []byte, v float64, dec int) []byte {
	if math.IsNaN(v) {
		return append(b, '-')
	}
	start := len(b)
	b = strconv.AppendFloat(b, v, 'f', dec, 64)
	if b[start] == '-' && strings.Trim(string(b[start+1:]), "0.") == "" {
		b = append(b[:start], b[start+1:]...)
	}
	return b
}
