package workout

import (
	"math"
	"reflect"
	"testing"
)

func bottoms(cp float64, v ...float64) ZoneList { return ZoneList{Values: v, Cap: cp} }
func tops(v ...float64) ZoneList                { return ZoneList{Values: v, Tops: true, Cap: math.NaN()} }

// TestZoneFrom is F-49 ZN-2 per list.
func TestZoneFrom(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		name string
		l    ZoneList
		top  float64
		want []int // nil: unusable
	}{
		{"apple 5 values: bottoms", bottoms(190, 95, 114, 133, 152, 171), 254, []int{114, 133, 152, 171}},
		{"apple 6 values: last = max is the cap", bottoms(190, 95, 114, 133, 152, 171, 190), 254, []int{114, 133, 152, 171}},
		{"last value not the max: kept", bottoms(195, 95, 114, 133, 152, 171, 190), 254, []int{114, 133, 152, 171, 190}},
		{"no cap without a max", bottoms(nan, 95, 114, 133, 152, 171, 190), 254, []int{114, 133, 152, 171, 190}},
		{"wahoo hr_zone: tops", tops(120, 140, 160, 175, 190), 254, []int{121, 141, 161, 176}},
		{"wahoo power_zone: tops, 7 zones", tops(138, 188, 225, 263, 300, 375, 2000), 65534, []int{139, 189, 226, 264, 301, 376}},
		{"trailing invalid values dropped", tops(120, 140, nan, nan), 254, []int{121}},
		{"trailing invalid before the cap check", bottoms(190, 95, 114, 190, nan), 254, []int{114}},
		{"invalid inside", bottoms(nan, 95, nan, 133), 254, nil},
		{"not ascending", tops(120, 140, 140, 175), 254, nil},
		{"descending", bottoms(nan, 171, 152, 133), 254, nil},
		{"one zone only", bottoms(190, 95, 190), 254, nil},
		{"single top", tops(150), 254, nil},
		{"10 zones", tops(1, 2, 3, 4, 5, 6, 7, 8, 9, 10), 254, []int{2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{"11 zones", tops(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11), 254, nil},
		{"top + 1 above the stream", tops(200, 254, 255), 254, nil},
		{"bottom 0 is allowed", bottoms(nan, 0, 120), 254, []int{120}},
		{"empty", tops(), 254, nil},
	} {
		got, ok := zoneFrom(c.l, c.top)
		if ok != (c.want != nil) || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v %v, want %v", c.name, got, ok, c.want)
		}
	}
}

// TestPickZones is ZN-3 steps 2 to 4, per kind.
func TestPickZones(t *testing.T) {
	nan := math.NaN()
	for _, c := range []struct {
		name string
		r    RawZones
		want Zones
	}{
		{"none (garmin)", RawZones{}, Zones{}},
		{"first session's list", RawZones{HeartRate: []ZoneList{bottoms(190, 95, 114, 133, 152, 171), tops(120, 140)}},
			Zones{HeartRate: ZoneValues{From: []int{114, 133, 152, 171}}}},
		{"second session's list when the first is unusable", RawZones{HeartRate: []ZoneList{bottoms(nan, 150, 140), bottoms(nan, 100, 130)}},
			Zones{HeartRate: ZoneValues{From: []int{130}}}},
		{"zone messages after time_in_zone", RawZones{HeartRate: []ZoneList{bottoms(nan, 150, 140), tops(120, 140)}},
			Zones{HeartRate: ZoneValues{From: []int{121}}}},
		{"a list beats a basis", RawZones{Power: []ZoneList{tops(138, 188)}, ThresholdPower: []float64{250}},
			Zones{Power: ZoneValues{From: []int{139}}}},
		{"first valid basis", RawZones{MaxHeartRate: []float64{nan, 99, 251, 186, 190}, ThresholdPower: []float64{49, 1001, 265}},
			Zones{HeartRate: ZoneValues{Basis: 186}, Power: ZoneValues{Basis: 265}}},
		{"range ends are valid", RawZones{MaxHeartRate: []float64{100}, ThresholdPower: []float64{1000}},
			Zones{HeartRate: ZoneValues{Basis: 100}, Power: ZoneValues{Basis: 1000}}},
		{"unusable list falls to the basis", RawZones{HeartRate: []ZoneList{tops(1)}, MaxHeartRate: []float64{190}},
			Zones{HeartRate: ZoneValues{Basis: 190}}},
	} {
		if got := pickZones(c.r); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

// TestNormaliseZones: Normalise applies the source order to the recording's
// candidates (the trim keeps them: publish TestZonesTrimmed).
func TestNormaliseZones(t *testing.T) {
	w := Normalise(Recording{Zones: RawZones{MaxHeartRate: []float64{190}}})
	if w.Zones.HeartRate.Basis != 190 {
		t.Fatalf("normalised zones %+v", w.Zones)
	}
}
