package ingest

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/muktihari/fit/kit/datetime"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"workout-explorer/internal/workout"
)

// Every conversion from raw FIT values into the units of the F-02 design.
// Validity is decided by comparing the raw value with the profile's invalid
// sentinel, never by the library's scaled helpers (F-02 design, Conversions).

var nan = math.NaN()

// degreesPerSemicircle: degrees = semicircles × 180 / 2³¹.
const degreesPerSemicircle = 180.0 / 2147483648.0

// semicirclesToDegrees converts a FIT semicircle value to degrees.
func semicirclesToDegrees(s int32) float64 {
	return float64(s) * degreesPerSemicircle
}

// position converts a raw position. It is valid only if both values are
// valid, |lat| ≤ 90, |lon| ≤ 180 and not exactly (0, 0); otherwise both are NaN.
func position(lat, lon int32) (float64, float64) {
	if lat == basetype.Sint32Invalid || lon == basetype.Sint32Invalid || (lat == 0 && lon == 0) {
		return nan, nan
	}
	la, lo := semicirclesToDegrees(lat), semicirclesToDegrees(lon)
	if math.Abs(la) > 90 || math.Abs(lo) > 180 {
		return nan, nan
	}
	return la, lo
}

// altitude prefers enhanced_altitude per sample; metres = raw / 5 − 500.
func altitude(legacy uint16, enhanced uint32) float64 {
	switch {
	case enhanced != basetype.Uint32Invalid:
		return float64(enhanced)/5 - 500
	case legacy != basetype.Uint16Invalid:
		return float64(legacy)/5 - 500
	}
	return nan
}

// speed prefers enhanced_speed per sample; m/s = raw / 1000.
func speed(legacy uint16, enhanced uint32) float64 {
	switch {
	case enhanced != basetype.Uint32Invalid:
		return float64(enhanced) / 1000
	case legacy != basetype.Uint16Invalid:
		return float64(legacy) / 1000
	}
	return nan
}

// distance converts a raw distance; m = raw / 100.
func distance(raw uint32) float64 {
	if raw == basetype.Uint32Invalid {
		return nan
	}
	return float64(raw) / 100
}

// cadence returns cadence + fractional/128 in the FIT unit (the sport
// conversion is R-5). An invalid fraction counts as 0.
func cadence(c, fractional uint8) float64 {
	if c == basetype.Uint8Invalid {
		return nan
	}
	v := float64(c)
	if fractional != basetype.Uint8Invalid {
		v += float64(fractional) / 128
	}
	return v
}

func uint8Value(v uint8) float64 {
	if v == basetype.Uint8Invalid {
		return nan
	}
	return float64(v)
}

func uint16Value(v uint16) float64 {
	if v == basetype.Uint16Invalid {
		return nan
	}
	return float64(v)
}

func sint8Value(v int8) float64 {
	if v == basetype.Sint8Invalid {
		return nan
	}
	return float64(v)
}

// rawTotals are the totals fields shared by session and lap, as raw values.
type rawTotals struct {
	elapsed, timer, distance           uint32 // s×1000, s×1000, m×100
	ascent, descent, calories          uint16 // m, m, kcal
	avgSpeed, maxSpeed                 uint16 // m/s×1000
	enhancedAvgSpeed, enhancedMaxSpeed uint32 // m/s×1000
	avgHR, maxHR                       uint8
	avgCadence, maxCadence             uint8
	avgFractional, maxFractional       uint8 // cadence/128
	avgPower, maxPower                 uint16
	avgTemp, maxTemp                   int8
}

// totals scales raw totals into the units of the F-02 design; nil = not recorded.
func totals(r rawTotals) workout.Totals {
	return workout.Totals{
		ElapsedTime:    ptr(scaledUint32(r.elapsed, 1000)),
		TimerTime:      ptr(scaledUint32(r.timer, 1000)),
		Distance:       ptr(scaledUint32(r.distance, 100)),
		Ascent:         ptr(uint16Value(r.ascent)),
		Descent:        ptr(uint16Value(r.descent)),
		Calories:       ptr(uint16Value(r.calories)),
		AvgSpeed:       ptr(speed(r.avgSpeed, r.enhancedAvgSpeed)),
		MaxSpeed:       ptr(speed(r.maxSpeed, r.enhancedMaxSpeed)),
		AvgHeartRate:   ptr(uint8Value(r.avgHR)),
		MaxHeartRate:   ptr(uint8Value(r.maxHR)),
		AvgCadence:     ptr(cadence(r.avgCadence, r.avgFractional)),
		MaxCadence:     ptr(cadence(r.maxCadence, r.maxFractional)),
		AvgPower:       ptr(uint16Value(r.avgPower)),
		MaxPower:       ptr(uint16Value(r.maxPower)),
		AvgTemperature: ptr(sint8Value(r.avgTemp)),
		MaxTemperature: ptr(sint8Value(r.maxTemp)),
	}
}

func scaledUint32(v uint32, scale float64) float64 {
	if v == basetype.Uint32Invalid {
		return nan
	}
	return float64(v) / scale
}

// ptr returns nil for NaN, else a pointer to v.
func ptr(v float64) *float64 {
	if math.IsNaN(v) {
		return nil
	}
	return &v
}

// minDateTime is the smallest FIT date_time that is a calendar time; smaller
// values count seconds since the device powered on (system time, E-8).
const minDateTime = 0x10000000

// validDateTime reports whether a raw FIT date_time or local_date_time is a
// calendar time: not the 0xFFFFFFFF sentinel and not system time (E-8).
func validDateTime(raw uint32) bool {
	return raw >= minDateTime && raw != basetype.Uint32Invalid
}

// dateTime returns a decoded FIT timestamp in UTC, or the zero time if its
// raw value is not valid (validDateTime). Every timestamp ingest reads goes
// through it.
func dateTime(t time.Time) time.Time {
	if t.IsZero() || !validDateTime(datetime.ToUint32(t)) {
		return time.Time{}
	}
	return t.UTC()
}

// spanEnd returns a session's or lap's end: timestamp if valid, else
// start + total_elapsed_time, else zero time.
func spanEnd(start, timestamp time.Time, elapsedRaw uint32) time.Time {
	switch {
	case !timestamp.IsZero():
		return timestamp
	case !start.IsZero() && elapsedRaw != basetype.Uint32Invalid:
		return start.Add(time.Duration(elapsedRaw) * time.Millisecond)
	}
	return time.Time{}
}

// spanStart returns a session's or lap's start: start_time if valid, else
// end − total_elapsed_time if both are valid, else zero time (E-2).
func spanStart(startTime, end time.Time, elapsedRaw uint32) time.Time {
	switch {
	case !startTime.IsZero():
		return startTime
	case !end.IsZero() && elapsedRaw != basetype.Uint32Invalid:
		return end.Add(-time.Duration(elapsedRaw) * time.Millisecond)
	}
	return time.Time{}
}

// utc returns t in UTC, keeping the zero value.
func utc(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}

// enumName returns the FIT profile's snake_case name of an enum value as the
// library's String() gives it. The invalid sentinel (field not recorded)
// gives ""; a value not in the profile gives "unknown_<n>".
func enumName(name string, value, invalid uint64) string {
	if value == invalid {
		return ""
	}
	// The library formats values outside the profile as "<Type>Invalid(<n>)".
	// The sentinel itself is handled above (E-1).
	if strings.Contains(name, "Invalid(") {
		return "unknown_" + strconv.FormatUint(value, 10)
	}
	return name
}

// localOffset returns activity.local_timestamp − activity.timestamp in
// seconds, nil if either is invalid.
func localOffset(timestamp, local time.Time) *int64 {
	if timestamp.IsZero() || local.IsZero() {
		return nil
	}
	d := local.Unix() - timestamp.Unix()
	return &d
}

// garminProductName returns the product name from the FIT profile for the
// manufacturers whose product numbers are in it (garmin, dynastream,
// dynastream_oem, tacx; ADR-018 item 4), "unknown_<n>" if the number is not
// in the profile, and "" for other manufacturers or an absent product. Used
// for the log line only.
func garminProductName(f *mesgdef.FileId) string {
	switch f.Manufacturer {
	case typedef.ManufacturerGarmin, typedef.ManufacturerDynastream, typedef.ManufacturerDynastreamOem, typedef.ManufacturerTacx:
	default:
		return ""
	}
	if f.Product == basetype.Uint16Invalid {
		return ""
	}
	gp := typedef.GarminProduct(f.Product)
	return enumName(gp.String(), uint64(gp), uint64(typedef.GarminProductInvalid))
}
