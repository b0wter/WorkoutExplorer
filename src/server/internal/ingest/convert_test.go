package ingest

import (
	"math"
	"testing"
	"time"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/typedef"
)

func TestSemicirclesToDegrees(t *testing.T) {
	tests := []struct {
		in   int32
		want float64
	}{
		{1 << 30, 90.0},
		{-(1 << 30), -90.0},
		{0, 0},
		{1 << 29, 45.0},
	}
	for _, tt := range tests {
		if got := semicirclesToDegrees(tt.in); got != tt.want {
			t.Errorf("semicirclesToDegrees(%d) = %v, want exactly %v", tt.in, got, tt.want)
		}
	}
}

func TestPosition(t *testing.T) {
	const deg47 = int32(560731834) // ≈ 47°
	tests := []struct {
		name     string
		lat, lon int32
		valid    bool
	}{
		{"valid", deg47, 1 << 26, true},
		{"lat sentinel", basetype.Sint32Invalid, 1 << 26, false},
		{"lon sentinel", deg47, basetype.Sint32Invalid, false},
		{"(0, 0) is missing", 0, 0, false},
		{"lat 0 alone is valid", 0, 1 << 26, true},
		{"lat > 90°", math.MaxInt32 - 1, 1 << 26, false},
		{"lat exactly 90°", 1 << 30, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			la, lo := position(tt.lat, tt.lon)
			if tt.valid && (math.IsNaN(la) || math.IsNaN(lo)) {
				t.Errorf("position = (%v, %v), want valid", la, lo)
			}
			if !tt.valid && !(math.IsNaN(la) && math.IsNaN(lo)) {
				t.Errorf("position = (%v, %v), want both NaN", la, lo)
			}
		})
	}
}

func TestAltitudePrefersEnhanced(t *testing.T) {
	tests := []struct {
		name     string
		legacy   uint16
		enhanced uint32
		want     float64
	}{
		{"enhanced preferred", 3000, 4000, 300},
		{"legacy when enhanced invalid", 3000, basetype.Uint32Invalid, 100},
		{"raw 2,500 is 0 m", 2500, basetype.Uint32Invalid, 0},
		{"below sea level", basetype.Uint16Invalid, 2000, -100},
		{"both invalid", basetype.Uint16Invalid, basetype.Uint32Invalid, math.NaN()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := altitude(tt.legacy, tt.enhanced)
			if !same(got, tt.want) {
				t.Errorf("altitude = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpeedPrefersEnhanced(t *testing.T) {
	tests := []struct {
		name     string
		legacy   uint16
		enhanced uint32
		want     float64
	}{
		{"enhanced preferred", 1000, 5000, 5},
		{"legacy when enhanced invalid", 1000, basetype.Uint32Invalid, 1},
		{"both invalid", basetype.Uint16Invalid, basetype.Uint32Invalid, math.NaN()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := speed(tt.legacy, tt.enhanced); !same(got, tt.want) {
				t.Errorf("speed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScalarConversions(t *testing.T) {
	tests := []struct {
		name      string
		got, want float64
	}{
		{"distance", distance(123456), 1234.56},
		{"distance invalid", distance(basetype.Uint32Invalid), math.NaN()},
		{"cadence with fraction", cadence(85, 64), 85.5},
		{"cadence invalid fraction", cadence(85, basetype.Uint8Invalid), 85},
		{"cadence invalid", cadence(basetype.Uint8Invalid, 64), math.NaN()},
		{"heart rate", uint8Value(150), 150},
		{"heart rate invalid", uint8Value(basetype.Uint8Invalid), math.NaN()},
		{"power", uint16Value(250), 250},
		{"power invalid", uint16Value(basetype.Uint16Invalid), math.NaN()},
		{"temperature negative", sint8Value(-5), -5},
		{"temperature invalid", sint8Value(basetype.Sint8Invalid), math.NaN()},
	}
	for _, tt := range tests {
		if !same(tt.got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestTotals(t *testing.T) {
	r := rawTotals{
		elapsed: 3600500, timer: basetype.Uint32Invalid, distance: 2500000,
		ascent: 300, descent: basetype.Uint16Invalid, calories: 700,
		avgSpeed: 6000, maxSpeed: 12000, enhancedAvgSpeed: 6944, enhancedMaxSpeed: basetype.Uint32Invalid,
		avgHR: 140, maxHR: basetype.Uint8Invalid, avgCadence: 85, maxCadence: 100,
		avgFractional: 32, maxFractional: basetype.Uint8Invalid,
		avgPower: basetype.Uint16Invalid, maxPower: 900, avgTemp: -2, maxTemp: basetype.Sint8Invalid,
	}
	got := totals(r)
	check := func(name string, p *float64, want float64) {
		t.Helper()
		if math.IsNaN(want) {
			if p != nil {
				t.Errorf("%s = %v, want nil", name, *p)
			}
			return
		}
		if p == nil || *p != want {
			t.Errorf("%s = %v, want %v", name, p, want)
		}
	}
	check("ElapsedTime", got.ElapsedTime, 3600.5)
	check("TimerTime", got.TimerTime, math.NaN())
	check("Distance", got.Distance, 25000)
	check("Ascent", got.Ascent, 300)
	check("Descent", got.Descent, math.NaN())
	check("Calories", got.Calories, 700)
	check("AvgSpeed (enhanced)", got.AvgSpeed, 6.944)
	check("MaxSpeed (legacy)", got.MaxSpeed, 12)
	check("AvgHeartRate", got.AvgHeartRate, 140)
	check("MaxHeartRate", got.MaxHeartRate, math.NaN())
	check("AvgCadence", got.AvgCadence, 85.25)
	check("MaxCadence", got.MaxCadence, 100)
	check("AvgPower", got.AvgPower, math.NaN())
	check("MaxPower", got.MaxPower, 900)
	check("AvgTemperature", got.AvgTemperature, -2)
	check("MaxTemperature", got.MaxTemperature, math.NaN())
}

func TestSpanEnd(t *testing.T) {
	start := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	ts := start.Add(time.Hour)
	if got := spanEnd(start, ts, 5000); !got.Equal(ts) {
		t.Errorf("timestamp valid: %v, want %v", got, ts)
	}
	if got, want := spanEnd(start, time.Time{}, 90500), start.Add(90500*time.Millisecond); !got.Equal(want) {
		t.Errorf("from elapsed: %v, want %v", got, want)
	}
	if got := spanEnd(start, time.Time{}, basetype.Uint32Invalid); !got.IsZero() {
		t.Errorf("no end: %v, want zero", got)
	}
	if got := spanEnd(time.Time{}, time.Time{}, 5000); !got.IsZero() {
		t.Errorf("no start: %v, want zero", got)
	}
}

func TestEnumName(t *testing.T) {
	// Pins the library's String() format (F-02 design, Conversions).
	if got := typedef.SportCycling.String(); got != "cycling" {
		t.Fatalf("typedef.SportCycling.String() = %q, want cycling; map names in convert.go", got)
	}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"sport", enumName(typedef.SportCycling.String(), uint64(typedef.SportCycling), uint64(typedef.SportInvalid)), "cycling"},
		{"sub sport", enumName(typedef.SubSportIndoorCycling.String(), uint64(typedef.SubSportIndoorCycling), uint64(typedef.SubSportInvalid)), "indoor_cycling"},
		{"e-biking", enumName(typedef.SportEBiking.String(), uint64(typedef.SportEBiking), uint64(typedef.SportInvalid)), "e_biking"},
		{"manufacturer", enumName(typedef.ManufacturerGarmin.String(), uint64(typedef.ManufacturerGarmin), uint64(typedef.ManufacturerInvalid)), "garmin"},
		{"file type", enumName(typedef.FileActivity.String(), uint64(typedef.FileActivity), uint64(typedef.FileInvalid)), "activity"},
		{"sport not in profile", enumName(typedef.Sport(200).String(), 200, uint64(typedef.SportInvalid)), "unknown_200"},
		{"manufacturer not in profile", enumName(typedef.Manufacturer(9999).String(), 9999, uint64(typedef.ManufacturerInvalid)), "unknown_9999"},
		{"invalid sentinel", enumName(typedef.SportInvalid.String(), uint64(typedef.SportInvalid), uint64(typedef.SportInvalid)), ""},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestLocalOffset(t *testing.T) {
	ts := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	if got := localOffset(ts, ts.Add(2*time.Hour)); got == nil || *got != 7200 {
		t.Errorf("localOffset = %v, want 7200", got)
	}
	if got := localOffset(ts, ts.Add(-5*time.Hour)); got == nil || *got != -18000 {
		t.Errorf("localOffset = %v, want -18000", got)
	}
	if got := localOffset(time.Time{}, ts); got != nil {
		t.Errorf("invalid timestamp: %v, want nil", *got)
	}
	if got := localOffset(ts, time.Time{}); got != nil {
		t.Errorf("invalid local timestamp: %v, want nil", *got)
	}
}

func same(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Abs(a-b) < 1e-9
}

func TestSpanStart(t *testing.T) {
	end := time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC)
	if got, want := spanStart(time.Time{}, end, 3600000), end.Add(-time.Hour); !got.Equal(want) {
		t.Errorf("from end − elapsed: %v, want %v", got, want)
	}
	if got := spanStart(end, time.Time{}, 5); !got.Equal(end) {
		t.Errorf("start_time valid: %v, want %v", got, end)
	}
	if got := spanStart(time.Time{}, end, basetype.Uint32Invalid); !got.IsZero() {
		t.Errorf("no elapsed: %v, want zero", got)
	}
	if got := spanStart(time.Time{}, time.Time{}, 5000); !got.IsZero() {
		t.Errorf("no end: %v, want zero", got)
	}
}

func TestValidDateTime(t *testing.T) {
	tests := []struct {
		raw  uint32
		want bool
	}{
		{0x0FFFFFFF, false}, // system time (E-8)
		{0x10000000, true},
		{0xFFFFFFFF, false}, // sentinel
		{0, false},
		{1000, false},
	}
	for _, tt := range tests {
		if got := validDateTime(tt.raw); got != tt.want {
			t.Errorf("validDateTime(%#x) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

// TestLapTrigger (F-46 LS-1, AP-8): every FIT lap_trigger maps to its code.
func TestLapTrigger(t *testing.T) {
	for in, want := range map[typedef.LapTrigger]string{
		typedef.LapTriggerManual: "button", typedef.LapTriggerTime: "time", typedef.LapTriggerDistance: "distance",
		typedef.LapTriggerPositionStart: "position", typedef.LapTriggerPositionLap: "position",
		typedef.LapTriggerPositionWaypoint: "position", typedef.LapTriggerPositionMarked: "position",
		typedef.LapTriggerSessionEnd: "stop", typedef.LapTriggerFitnessEquipment: "equipment",
		typedef.LapTriggerInvalid: "", typedef.LapTrigger(99): "",
	} {
		if got := lapTrigger(in); got != want {
			t.Errorf("lapTrigger(%d) = %q, want %q", in, got, want)
		}
	}
}
