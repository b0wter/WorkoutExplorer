package ingest

import (
	"math"
	"testing"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

// F-68: running dynamics (XS-1), the Stryd allowlist (XS-3, XS-4) and the
// source rule (XS-5).

func TestDynamicConversion(t *testing.T) {
	for _, c := range []struct{ raw, want uint16 }{
		{0, 0}, {basetype.Uint16Invalid, 0}, {1, 0}, {5, 1}, {14, 1}, {15, 2}, {850, 85}, {10500, 1050}, {65534, 6553},
	} {
		if got := dynamic(c.raw); got != c.want {
			t.Errorf("dynamic(%d) = %d, want %d", c.raw, got, c.want)
		}
	}
}

// TestRunDynamicsStreams: the four streams in their units; the first 20 s
// (raw 0) and the 30 s without the fields are missing.
func TestRunDynamicsStreams(t *testing.T) {
	w := parseCorpus(t, "run-dynamics.fit")
	s := w.Samples
	for name, v := range map[string][]float64{"vo": s.VerticalOscillation, "gct": s.GroundContactTime, "stride": s.StrideLength, "ratio": s.VerticalRatio} {
		if len(v) != len(s.Offset) {
			t.Fatalf("%s has %d samples, want %d", name, len(v), len(s.Offset))
		}
		for _, i := range []int{0, 19, 100, 129} {
			if !math.IsNaN(v[i]) {
				t.Errorf("%s[%d] = %v, want missing", name, i, v[i])
			}
		}
		if math.IsNaN(v[20]) || math.IsNaN(v[130]) {
			t.Errorf("%s: values at 20 and 130 are missing", name)
		}
	}
	if v := s.VerticalOscillation[20]; v < 8.5 || v > 9.1 {
		t.Errorf("vertical oscillation = %v cm, want 8.5 to 9.1", v)
	}
	if v := s.GroundContactTime[20]; v < 240 || v > 260 {
		t.Errorf("ground contact time = %v ms, want 240 to 260", v)
	}
	if v := s.StrideLength[20]; v < 1.05 || v > 1.09 {
		t.Errorf("stride length = %v m, want 1.05 to 1.09", v)
	}
	if v := s.VerticalRatio[20]; v < 8.5 || v > 8.9 {
		t.Errorf("vertical ratio = %v %%, want 8.5 to 8.9", v)
	}
	if s.Power != nil {
		t.Error("a run without power has a power stream")
	}
}

func TestStrydSources(t *testing.T) {
	watts := func(t0 int) float64 { return float64(uint16(240 + 30*math.Sin(float64(t0)/90) + float64(t0%7))) }
	t.Run("Stryd only is the power stream, summary computed", func(t *testing.T) {
		w := parseCorpus(t, "stryd-power.fit")
		if w.Samples.Power == nil {
			t.Fatal("no power stream")
		}
		for _, i := range []int{0, 100, 1000} {
			if got := w.Samples.Power[i]; got != watts(i) {
				t.Errorf("power[%d] = %v, want %v", i, got, watts(i))
			}
		}
		if w.Summary.AvgPower == nil || w.Summary.MaxPower == nil {
			t.Errorf("summary power = %v / %v, want computed", deref(w.Summary.AvgPower), deref(w.Summary.MaxPower))
		}
	})
	t.Run("native power wins, Stryd values are absent", func(t *testing.T) {
		w := parseCorpus(t, "stryd-and-native-power.fit")
		for i, got := range w.Samples.Power {
			if want := float64(150 + i%40); got != want {
				t.Fatalf("power[%d] = %v, want native %v", i, got, want)
			}
		}
	})
	t.Run("two apps: the first described is the source", func(t *testing.T) {
		w := parseCorpus(t, "stryd-two-apps.fit")
		if !math.IsNaN(w.Samples.Power[0]) || !math.IsNaN(w.Samples.Power[59]) {
			t.Errorf("power before 60 s = %v, want missing (the first app has no value yet)", w.Samples.Power[0])
		}
		for _, i := range []int{60, 900} {
			if got := w.Samples.Power[i]; got != watts(i) {
				t.Errorf("power[%d] = %v, want the first app's %v", i, got, watts(i))
			}
		}
	})
	t.Run("redefinition ends the values", func(t *testing.T) {
		w := parseCorpus(t, "stryd-redefined.fit")
		if math.IsNaN(w.Samples.Power[599]) || !math.IsNaN(w.Samples.Power[600]) || !math.IsNaN(w.Samples.Power[1199]) {
			t.Errorf("power at 599, 600, 1199 = %v, %v, %v", w.Samples.Power[599], w.Samples.Power[600], w.Samples.Power[1199])
		}
	})
	t.Run("described after the first record", func(t *testing.T) {
		w := parseCorpus(t, "stryd-described-late.fit")
		if !math.IsNaN(w.Samples.Power[299]) || w.Samples.Power[300] != watts(300) {
			t.Errorf("power at 299, 300 = %v, %v", w.Samples.Power[299], w.Samples.Power[300])
		}
	})
	t.Run("chained: Stryd in the second sequence", func(t *testing.T) {
		w := parseCorpus(t, "stryd-chained.fit")
		n := len(w.Samples.Offset)
		if w.Samples.Power == nil || !math.IsNaN(w.Samples.Power[0]) || math.IsNaN(w.Samples.Power[n-1]) {
			t.Errorf("power = %v", w.Samples.Power == nil)
		}
	})
	t.Run("truncated", func(t *testing.T) {
		w := parseCorpus(t, "stryd-power-cut-60.fit")
		if w.Samples.Power == nil || len(w.Samples.Offset) < 700 {
			t.Errorf("cut file: %d samples, power present %v", len(w.Samples.Offset), w.Samples.Power != nil)
		}
	})
}

// TestDeveloperHostileYieldsNothing: no stream comes from any developer
// field of the hostile file, whatever it is named (XS-3, XS-7).
func TestDeveloperHostileYieldsNothing(t *testing.T) {
	w := parseCorpus(t, "developer-hostile.fit")
	s := w.Samples
	if s.Power != nil || s.VerticalOscillation != nil || s.GroundContactTime != nil || s.StrideLength != nil || s.VerticalRatio != nil {
		t.Error("a developer field became a stream")
	}
}

func desc(idx, num uint8, typ basetype.BaseType, name, units string, mod func(*mesgdef.FieldDescription)) proto.Message {
	d := mesgdef.NewFieldDescription(nil).SetDeveloperDataIndex(idx).SetFieldDefinitionNumber(num).
		SetFitBaseTypeId(typ).SetFieldName([]string{name}).SetUnits([]string{units})
	if mod != nil {
		mod(d)
	}
	return d.ToMesg(nil)
}

// TestQualifies is the XS-3 table: each row fails one condition.
func TestQualifies(t *testing.T) {
	for _, c := range []struct {
		name string
		typ  basetype.BaseType
		n, u string
		mod  func(*mesgdef.FieldDescription)
		want bool
	}{
		{"stryd", basetype.Uint16, "Power", "Watts", nil, true},
		{"float", basetype.Float32, "Power", "Watts", nil, true},
		{"scale 1, offset 0, native power", basetype.Uint16, "Power", "Watts", func(d *mesgdef.FieldDescription) {
			d.SetScale(1).SetOffset(0).SetNativeMesgNum(typedef.MesgNumRecord).SetNativeFieldNum(7)
		}, true},
		{"name case", basetype.Uint16, "power", "Watts", nil, false},
		{"name prefix", basetype.Uint16, "Power(Watts)", "Watts", nil, false},
		{"units", basetype.Uint16, "Power", "W", nil, false},
		{"string", basetype.String, "Power", "Watts", nil, false},
		{"byte", basetype.Byte, "Power", "Watts", nil, false},
		{"enum", basetype.Enum, "Power", "Watts", nil, false},
		{"scale 10", basetype.Uint16, "Power", "Watts", func(d *mesgdef.FieldDescription) { d.SetScale(10) }, false},
		{"offset", basetype.Uint16, "Power", "Watts", func(d *mesgdef.FieldDescription) { d.SetOffset(100) }, false},
		{"native field not power", basetype.Uint16, "Power", "Watts", func(d *mesgdef.FieldDescription) {
			d.SetNativeMesgNum(typedef.MesgNumRecord).SetNativeFieldNum(4)
		}, false},
		{"native message not record", basetype.Uint16, "Power", "Watts", func(d *mesgdef.FieldDescription) {
			d.SetNativeMesgNum(typedef.MesgNumSession).SetNativeFieldNum(7)
		}, false},
		{"native message only", basetype.Uint16, "Power", "Watts", func(d *mesgdef.FieldDescription) { d.SetNativeMesgNum(typedef.MesgNumLap) }, false},
	} {
		m := desc(0, 0, c.typ, c.n, c.u, c.mod)
		_, got := qualifies(mesgdef.NewFieldDescription(&m))
		if got != c.want {
			t.Errorf("%s: qualifies = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestStrydValueRules: invalid sentinels, array and string values, and a
// value read with another type than the current description promises.
func TestStrydValueRules(t *testing.T) {
	d := desc(0, 0, basetype.Uint16, "Power", "Watts", nil)
	newReader := func() *strydReader {
		s := new(strydReader)
		s.description(mesgdef.NewFieldDescription(&d))
		return s
	}
	s := newReader()
	for _, c := range []struct {
		name string
		v    proto.Value
		want bool
	}{
		{"value", proto.Uint16(250), true},
		{"zero is a value", proto.Uint16(0), true},
		{"invalid", proto.Uint16(basetype.Uint16Invalid), false},
		{"array", proto.SliceUint16([]uint16{1, 2}), false},
		{"string", proto.String("250"), false},
		{"other base type", proto.Uint8(250), false},
	} {
		m := proto.Message{Num: typedef.MesgNumRecord, DeveloperFields: []proto.DeveloperField{{Num: 0, DeveloperDataIndex: 0, Value: c.v}}}
		if _, ok := newReader().power(&m); ok != c.want {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.want)
		}
	}
	// A field of an index whose developer_data_id came again is forgotten (XS-4).
	id := mesgdef.NewDeveloperDataId(nil).SetDeveloperDataIndex(0).ToMesg(nil)
	s.developerDataID(&id)
	m := proto.Message{Num: typedef.MesgNumRecord, DeveloperFields: []proto.DeveloperField{{Num: 0, DeveloperDataIndex: 0, Value: proto.Uint16(250)}}}
	if _, ok := s.power(&m); ok {
		t.Error("a value after a new developer_data_id has a description")
	}
	// A sequence start forgets everything.
	s.description(mesgdef.NewFieldDescription(&d))
	s.reset()
	if _, ok := s.power(&m); ok {
		t.Error("a value in a new sequence has a description")
	}
}

// TestFillStryd: one valid native value anywhere in the upload keeps Stryd
// out (XS-5); otherwise Stryd fills the records.
func TestFillStryd(t *testing.T) {
	n := math.NaN()
	samples := []strydSample{{0, 200}, {2, 210}}
	recs := []workout.RawRecord{{Power: n}, {Power: n}, {Power: n}}
	fillStryd(recs, samples)
	if recs[0].Power != 200 || !math.IsNaN(recs[1].Power) || recs[2].Power != 210 {
		t.Errorf("no native: %v", recs)
	}
	recs = []workout.RawRecord{{Power: n}, {Power: 0}, {Power: n}}
	fillStryd(recs, samples)
	if !math.IsNaN(recs[0].Power) || recs[1].Power != 0 || !math.IsNaN(recs[2].Power) {
		t.Errorf("native 0 W present: %v", recs)
	}
}
