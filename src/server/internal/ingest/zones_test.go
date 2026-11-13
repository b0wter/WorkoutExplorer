package ingest

import (
	"math"
	"reflect"
	"testing"

	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
)

// TestHRList is the unit part of F-49 ZN-2.
func TestHRList(t *testing.T) {
	for _, c := range []struct {
		name string
		raw  []uint8
		calc typedef.HrZoneCalc
		max  float64
		want []float64 // nil: unusable
	}{
		{"bpm under % of max (apple)", []uint8{95, 114, 133, 152, 171, 190}, typedef.HrZoneCalcPercentMaxHr, 190, []float64{95, 114, 133, 152, 171, 190}},
		{"percent of max → bpm", []uint8{50, 60, 70, 80, 90, 100}, typedef.HrZoneCalcPercentMaxHr, 190, []float64{95, 114, 133, 152, 171, 190}},
		{"percent rounds half away from zero", []uint8{55, 65}, typedef.HrZoneCalcPercentMaxHr, 187, []float64{103, 122}},
		{"percent with an invalid max", []uint8{50, 60}, typedef.HrZoneCalcPercentMaxHr, math.NaN(), nil},
		{"percent with a max out of range", []uint8{50, 60}, typedef.HrZoneCalcPercentMaxHr, 99, nil},
		{"percent of reserve", []uint8{50, 60, 70}, typedef.HrZoneCalcPercentHrr, 190, nil},
		{"percent of threshold", []uint8{50, 60, 70}, typedef.HrZoneCalcPercentLthr, 190, nil},
		{"bpm under % of reserve: values above 100", []uint8{95, 120, 140}, typedef.HrZoneCalcPercentHrr, 190, []float64{95, 120, 140}},
		{"custom ≤ 100 stays bpm", []uint8{80, 90, 100}, typedef.HrZoneCalcCustom, 190, []float64{80, 90, 100}},
		{"invalid values stay invalid", []uint8{50, 60, 0xFF}, typedef.HrZoneCalcPercentMaxHr, 190, []float64{95, 114, math.NaN()}},
		{"11 values dropped", make([]uint8, 11), typedef.HrZoneCalcCustom, 190, nil},
	} {
		got, ok := hrList(c.raw, c.calc, c.max)
		if ok != (c.want != nil) || len(got) != len(c.want) {
			t.Errorf("%s: %v %v, want %v", c.name, got, ok, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] && !(math.IsNaN(got[i]) && math.IsNaN(c.want[i])) {
				t.Errorf("%s: %v, want %v", c.name, got, c.want)
			}
		}
	}
}

// TestZoneReader: source order, lap-referenced time_in_zone ignored, long
// zone message lists dropped, nothing but bounds and bases read.
func TestZoneReader(t *testing.T) {
	z := zoneReader{maxLists: 2}
	z.zonesTarget(mesgdef.NewZonesTarget(nil).SetMaxHeartRate(186).SetFunctionalThresholdPower(240))
	z.session(mesgdef.NewSession(nil).SetThresholdPower(250).SetMaxHeartRate(201))
	z.timeInZone(mesgdef.NewTimeInZone(nil).SetReferenceMesg(typedef.MesgNumLap).SetHrZoneHighBoundary([]uint8{1, 2}).SetMaxHeartRate(150))
	for range 3 { // the third is over maxLists
		z.timeInZone(mesgdef.NewTimeInZone(nil).SetReferenceMesg(typedef.MesgNumSession).SetMaxHeartRate(190).
			SetHrZoneHighBoundary([]uint8{95, 114}).SetPowerZoneHighBoundary([]uint16{140, 190}).SetRestingHeartRate(31))
	}
	for i := range 5 {
		z.hrZone(mesgdef.NewHrZone(nil).SetHighBpm(uint8(120 + i)))
	}
	for i := range 11 {
		z.powerZone(mesgdef.NewPowerZone(nil).SetHighValue(uint16(100 + i)))
	}
	r := z.zones()
	if len(r.HeartRate) != 3 || !r.HeartRate[2].Tops || r.HeartRate[0].Tops || r.HeartRate[0].Cap != 190 {
		t.Errorf("heart rate lists %+v", r.HeartRate)
	}
	if len(r.Power) != 2 { // two time_in_zone lists; the 11 power_zone messages are dropped
		t.Errorf("power lists %+v", r.Power)
	}
	if !reflect.DeepEqual(r.MaxHeartRate, []float64{190, 190, 186}) || !reflect.DeepEqual(r.ThresholdPower, []float64{240, 250}) {
		t.Errorf("bases %v %v", r.MaxHeartRate, r.ThresholdPower)
	}
}
