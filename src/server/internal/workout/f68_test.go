package workout

import (
	"math"
	"testing"
)

// F-68 XS-1, XS-8: running dynamics are streams like cadence: converted from
// the stored resolution, dropped at transition samples, cut by trim, absent
// when no record has a value.

func dynRecords(n int) []RawRecord {
	return recs(seq(0, n-1), func(i int, r *RawRecord) {
		r.Lat, r.Lon = 50+float64(i)*1e-4, 8+float64(i)*1e-4
		r.Distance, r.Speed = float64(i)*5, 8
		r.VerticalOscillation, r.GroundContactTime, r.StrideLength, r.VerticalRatio = 85, 245, 1050, 87
	})
}

func TestDynamicsConversion(t *testing.T) {
	w := Normalise(Recording{Records: dynRecords(10)})
	for name, c := range map[string]struct {
		got  []float64
		want float64
	}{
		"vertical oscillation cm": {w.Samples.VerticalOscillation, 8.5},
		"ground contact ms":       {w.Samples.GroundContactTime, 245},
		"stride length m":         {w.Samples.StrideLength, 1.05},
		"vertical ratio %":        {w.Samples.VerticalRatio, 8.7},
	} {
		if len(c.got) != 10 || !near(c.got[3], c.want, 1e-9) {
			t.Errorf("%s = %v, want %v", name, c.got, c.want)
		}
	}
	w = Normalise(Recording{Records: recs(seq(0, 9), nil)})
	if w.Samples.VerticalOscillation != nil || w.Samples.GroundContactTime != nil || w.Samples.StrideLength != nil || w.Samples.VerticalRatio != nil {
		t.Error("streams without a value are present")
	}
}

func TestDynamicsDroppedAtTransitions(t *testing.T) {
	rec := Recording{Records: dynRecords(90), Sessions: triSessions()}
	w := Normalise(rec)
	s := w.Samples
	for i, off := range s.Offset {
		in := off >= 30 && off <= 59
		for name, v := range map[string][]float64{"vo": s.VerticalOscillation, "gct": s.GroundContactTime, "stride": s.StrideLength, "ratio": s.VerticalRatio} {
			if in != math.IsNaN(v[i]) {
				t.Fatalf("%s at %d: NaN = %v, want %v", name, off, math.IsNaN(v[i]), in)
			}
		}
	}
}

func TestDynamicsTrim(t *testing.T) {
	w := Normalise(Recording{Records: dynRecords(30)})
	tw := Trim(w, 10, 19)
	if len(tw.Samples.GroundContactTime) != 10 || len(tw.Samples.StrideLength) != 10 {
		t.Errorf("trimmed lengths %d, %d, want 10", len(tw.Samples.GroundContactTime), len(tw.Samples.StrideLength))
	}
}
