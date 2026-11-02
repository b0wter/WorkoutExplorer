package workout

import (
	"math"
	"testing"
	"time"
)

var (
	nan = math.NaN()
	t0  = time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
)

func at(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }

func f64(v float64) *float64 { return &v }

// rec returns a record at offset s with every value missing.
func rec(s int) RawRecord {
	return RawRecord{At: at(s), Lat: nan, Lon: nan, Altitude: nan, Distance: nan, Speed: nan,
		HeartRate: nan, Cadence: nan, Power: nan, Temperature: nan}
}

// recs returns records at the given offsets, each changed by set.
func recs(offsets []int, set func(i int, r *RawRecord)) []RawRecord {
	out := make([]RawRecord, len(offsets))
	for i, s := range offsets {
		out[i] = rec(s)
		if set != nil {
			set(i, &out[i])
		}
	}
	return out
}

// seq returns from, from+1, ..., to.
func seq(from, to int) []int {
	var out []int
	for s := from; s <= to; s++ {
		out = append(out, s)
	}
	return out
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// sameFloats compares slices treating NaN as equal to NaN.
func sameFloats(a, b []float64) bool {
	if len(a) != len(b) || (a == nil) != (b == nil) {
		return false
	}
	for i := range a {
		if math.IsNaN(a[i]) != math.IsNaN(b[i]) || (!math.IsNaN(a[i]) && !near(a[i], b[i], 1e-9)) {
			return false
		}
	}
	return true
}

func wantPtr(t *testing.T, name string, got *float64, want float64, eps float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %v", name, want)
		return
	}
	if !near(*got, want, eps) {
		t.Errorf("%s = %v, want %v ± %v", name, *got, want, eps)
	}
}

func wantNil(t *testing.T, name string, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %v, want nil", name, *got)
	}
}

// checkInvariants checks invariants 1 to 3 of the F-02 design. Invariant 4
// (nil, never zero) is covered by the rule tests; 5 (no panic) holds when
// Normalise returned.
func checkInvariants(t *testing.T, w Workout) {
	t.Helper()
	s := w.Samples
	for i := range s.Offset {
		if i == 0 && s.Offset[0] < 0 {
			t.Errorf("Offset[0] = %d < 0", s.Offset[0])
		}
		if i > 0 && s.Offset[i] <= s.Offset[i-1] {
			t.Errorf("Offset not strictly increasing at %d: %d after %d", i, s.Offset[i], s.Offset[i-1])
		}
	}
	if len(s.inserted) != 0 && len(s.inserted) != len(s.Offset) {
		t.Errorf("inserted: len %d, want 0 or %d", len(s.inserted), len(s.Offset))
	}
	for k, p := range s.streams() {
		v := *p
		if v == nil {
			continue
		}
		if len(v) != len(s.Offset) {
			t.Errorf("stream %s: len %d, want %d", streamNames[k], len(v), len(s.Offset))
		}
		if !hasValid(v) {
			t.Errorf("stream %s present without a valid value", streamNames[k])
		}
		for _, x := range v {
			if math.IsInf(x, 0) {
				t.Errorf("stream %s contains Inf", streamNames[k])
				break
			}
		}
	}
	if (s.Lat == nil) != (s.Lon == nil) {
		t.Errorf("Lat nil = %v, Lon nil = %v", s.Lat == nil, s.Lon == nil)
	}
	for i := range s.Lat {
		if math.IsNaN(s.Lat[i]) != math.IsNaN(s.Lon[i]) {
			t.Errorf("position %d half missing: lat %v lon %v", i, s.Lat[i], s.Lon[i])
		}
	}
}

// TestInvariantsCatchInsertedLength: a marker list that does not match the
// samples fails checkInvariants (QA-16-r1-1).
func TestInvariantsCatchInsertedLength(t *testing.T) {
	ft := &testing.T{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		w := Workout{Samples: Samples{Offset: []int32{0, 1, 2}, inserted: []bool{false, true}}}
		checkInvariants(ft, w)
	}()
	<-done
	if !ft.Failed() {
		t.Error("checkInvariants accepted a short inserted list")
	}
}
