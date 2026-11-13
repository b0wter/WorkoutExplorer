package ingest

import (
	"math"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"workout-explorer/internal/workout"
)

// maxZoneValues: a boundary list longer than this is dropped while reading
// (F-49 ZN-1), so a list never grows past ten values.
const maxZoneValues = 10

// zoneReader collects the zone candidates of F-49 ZN-1 per source; zones
// puts them in source order. Resting and threshold heart rate, calc types
// other than the heart-rate percent rule, and time-in-zone totals are never
// read. Each slice holds at most maxLists entries (MaxSessions).
type zoneReader struct {
	maxLists int

	tizHR, tizPower    []workout.ZoneList // session-referenced time_in_zone lists, file order
	hrTops, powerTops  []float64          // hr_zone.high_bpm, power_zone.high_value, file order
	hrLong, powerLong  bool               // more than maxZoneValues zone messages: the list is dropped
	tizMax, targetMax  []float64          // max heart rate bases
	targetFTP, sessFTP []float64          // threshold power bases
}

// add appends v to *s unless s is full.
func (z *zoneReader) add(s *[]float64, v float64) {
	if len(*s) < z.maxLists {
		*s = append(*s, v)
	}
}

func (z *zoneReader) timeInZone(m *mesgdef.TimeInZone) {
	if m.ReferenceMesg != typedef.MesgNumSession {
		return
	}
	maxHR := uint8Value(m.MaxHeartRate)
	z.add(&z.tizMax, maxHR)
	if hr, ok := hrList(m.HrZoneHighBoundary, m.HrCalcType, maxHR); ok && len(z.tizHR) < z.maxLists {
		z.tizHR = append(z.tizHR, workout.ZoneList{Values: hr, Cap: maxHR})
	}
	if n := len(m.PowerZoneHighBoundary); n > 0 && n <= maxZoneValues && len(z.tizPower) < z.maxLists {
		p := make([]float64, n)
		for i, v := range m.PowerZoneHighBoundary {
			p[i] = uint16Value(v)
		}
		z.tizPower = append(z.tizPower, workout.ZoneList{Values: p, Cap: math.NaN()})
	}
}

// hrList is the unit part of ZN-2: a time_in_zone heart-rate list in bpm.
// Under a percent calc type with every valid value ≤ 100 the values are
// percent: of max heart rate → bpm with the message's max (valid by ZN-4's
// range), half away from zero; of reserve or threshold → unusable.
func hrList(raw []uint8, calc typedef.HrZoneCalc, maxHR float64) ([]float64, bool) {
	if len(raw) == 0 || len(raw) > maxZoneValues {
		return nil, false
	}
	out := make([]float64, len(raw))
	percent := true
	for i, v := range raw {
		out[i] = uint8Value(v)
		if !math.IsNaN(out[i]) && out[i] > 100 {
			percent = false
		}
	}
	switch {
	case !percent || calc == typedef.HrZoneCalcCustom || calc == typedef.HrZoneCalcInvalid:
		return out, true
	case calc != typedef.HrZoneCalcPercentMaxHr:
		return nil, false // percent of reserve or threshold (or unknown): needs values we never read
	case !workout.ValidBasis(maxHR, workout.MinMaxHeartRate, workout.MaxMaxHeartRate):
		return nil, false
	}
	for i := range out {
		out[i] = math.Round(out[i] * maxHR / 100) // NaN stays NaN
	}
	return out, true
}

func (z *zoneReader) hrZone(m *mesgdef.HrZone) {
	z.hrLong = z.hrLong || len(z.hrTops) == maxZoneValues
	if !z.hrLong {
		z.hrTops = append(z.hrTops, uint8Value(m.HighBpm))
	}
}

func (z *zoneReader) powerZone(m *mesgdef.PowerZone) {
	z.powerLong = z.powerLong || len(z.powerTops) == maxZoneValues
	if !z.powerLong {
		z.powerTops = append(z.powerTops, uint16Value(m.HighValue))
	}
}

func (z *zoneReader) zonesTarget(m *mesgdef.ZonesTarget) {
	z.add(&z.targetMax, uint8Value(m.MaxHeartRate))
	z.add(&z.targetFTP, uint16Value(m.FunctionalThresholdPower))
}

func (z *zoneReader) session(m *mesgdef.Session) {
	if m.ThresholdPower != basetype.Uint16Invalid {
		z.sessFTP = append(z.sessFTP, float64(m.ThresholdPower)) // bounded by the session budget
	}
}

// zones returns the candidates in source order (ZN-3).
func (z *zoneReader) zones() workout.RawZones {
	r := workout.RawZones{
		HeartRate:      z.tizHR,
		Power:          z.tizPower,
		MaxHeartRate:   append(z.tizMax, z.targetMax...),
		ThresholdPower: append(z.targetFTP, z.sessFTP...),
	}
	if len(z.hrTops) > 0 && !z.hrLong {
		r.HeartRate = append(r.HeartRate, workout.ZoneList{Values: z.hrTops, Tops: true, Cap: math.NaN()})
	}
	if len(z.powerTops) > 0 && !z.powerLong {
		r.Power = append(r.Power, workout.ZoneList{Values: z.powerTops, Tops: true, Cap: math.NaN()})
	}
	return r
}
