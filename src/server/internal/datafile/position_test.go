package datafile

import (
	"bytes"
	"testing"

	"workout-explorer/internal/workout"
)

func TestFirstPositionAgreesWithDecode(t *testing.T) {
	w := workout.Workout{Sport: "cycling", Samples: workout.Samples{
		Offset: []int32{0, 1, 2, 3, 4},
		Lat:    []float64{nan, 10.5, 10.6, 10.7, 10.8},
		Lon:    []float64{nan, 20.5, 20.6, 20.7, 20.8},
	}}
	bin, err := Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	if la, lo, ok := f.FirstPosition(); !ok || la != 10.5 || lo != 20.5 {
		t.Fatalf("decoded: %v %v %v", la, lo, ok)
	}
	for _, c := range []struct {
		skip [][2]int
		lat  float64
		ok   bool
	}{{nil, 10.5, true}, {[][2]int{{0, 2}}, 10.7, true}, {[][2]int{{0, 1}, {3, 4}}, 10.6, true}, {[][2]int{{0, 4}}, 0, false}} {
		m, la, _, ok, err := FirstPositionAt(bytes.NewReader(bin), int64(len(bin)), c.skip)
		if err != nil || m.Sport != "cycling" || ok != c.ok || la != c.lat {
			t.Errorf("%v: %v %v %v", c.skip, la, ok, err)
		}
	}
	if _, _, _, _, err := FirstPositionAt(bytes.NewReader(bin[:20]), 20, nil); err == nil {
		t.Error("truncated file accepted")
	}
}
