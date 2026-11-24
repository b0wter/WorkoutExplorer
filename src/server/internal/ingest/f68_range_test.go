package ingest

import (
	"math"
	"testing"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

// TestStrydRange: values outside native power's range (0 to 65,534) are
// missing, so summary and laps cannot disagree with the chart (QA-68-r1-1).
func TestStrydRange(t *testing.T) {
	for _, c := range []struct {
		v    proto.Value
		want bool
	}{
		{proto.Int16(-30000), false}, {proto.Int16(-1), false}, {proto.Int16(0), true},
		{proto.Int32(65534), true}, {proto.Int32(65535), false}, {proto.Float64(1e12), false},
		{proto.Float32(float32(math.Inf(1))), false}, {proto.Float32(250.5), true},
	} {
		if _, ok := validWatts(c.v); ok != c.want {
			t.Errorf("%v: ok = %v, want %v", c.v.Any(), ok, c.want)
		}
	}
}

// TestStrydRangeParse: a synthetic file whose Stryd field is a float64 with
// one 1e12 value has no such value in summary or samples (QA-68-r1-1).
func TestStrydRangeParse(t *testing.T) {
	id := mesgdef.NewDeveloperDataId(nil).SetDeveloperDataIndex(0).ToMesg(nil)
	rec := func(s int, v float64) proto.Message {
		m := record(s, 120).ToMesg(nil)
		m.DeveloperFields = []proto.DeveloperField{{Num: 0, DeveloperDataIndex: 0, Value: proto.Float64(v)}}
		return m
	}
	b := encodeFIT(t, fileID(), id, desc(0, 0, basetype.Float64, "Power", "Watts", nil),
		rec(0, 200), rec(1, 1e12), rec(2, 220))
	w, err := testParser.Parse(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range w.Samples.Power {
		if !math.IsNaN(p) && p > 65534 {
			t.Fatalf("power out of range: %v", w.Samples.Power)
		}
	}
	if w.Summary.MaxPower != nil && *w.Summary.MaxPower > 65534 {
		t.Errorf("max power %v", *w.Summary.MaxPower)
	}
}

// TestStrydSourceLocked: after the first valid value, a later wrong-shaped
// value of the source is missing for that record and never switches to the
// second app (QA-68-r1-2, XS-4).
func TestStrydSourceLocked(t *testing.T) {
	s := new(strydReader)
	for i := uint8(0); i < 2; i++ {
		d := desc(i, 0, basetype.Uint16, "Power", "Watts", nil)
		s.description(mesgdef.NewFieldDescription(&d))
	}
	rec := func(idx uint8, v proto.Value) *proto.Message {
		return &proto.Message{Num: typedef.MesgNumRecord, DeveloperFields: []proto.DeveloperField{{Num: 0, DeveloperDataIndex: idx, Value: v}}}
	}
	if w, ok := s.power(rec(0, proto.Uint16(200))); !ok || w != 200 {
		t.Fatalf("first value: %v %v", w, ok)
	}
	if _, ok := s.power(rec(0, proto.SliceUint16([]uint16{1, 2}))); ok {
		t.Error("wrong shape gave a value")
	}
	if _, ok := s.power(rec(1, proto.Uint16(900))); ok {
		t.Error("the second app became the source")
	}
	if w, ok := s.power(rec(0, proto.Uint16(210))); !ok || w != 210 {
		t.Errorf("the source did not recover: %v %v", w, ok)
	}
}
