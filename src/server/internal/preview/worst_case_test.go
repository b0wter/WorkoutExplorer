package preview

import (
	"math"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
	"workout-explorer/internal/workout"
)

// TestWorstCaseCost (CR2-01, ADR-017 limits): a crafted zigzag of 500k samples,
// each on a new pixel, must render in bounded time (was 2.5 s plain, 6 s on a map).
func TestWorstCaseCost(t *testing.T) {
	const n = 500_000
	off := make([]int32, n)
	lat := make([]float64, n)
	lon := make([]float64, n)
	for i := range n {
		off[i] = int32(i)
		lat[i] = 47 + 0.045*math.Abs(math.Sin(float64(i)*0.7))
		lon[i] = 8 + 0.06*float64(i)/float64(n) + 0.01*math.Sin(float64(i)*1.3)
	}
	w := workout.Workout{Sport: "running", StartTime: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC), Samples: workout.Samples{Offset: off, Lat: lat, Lon: lon}}
	bin, err := datafile.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	vm := page.Build(f, page.Settings{Title: "x", PublicID: "x"})
	t0 := time.Now()
	if _, err := Render(f, vm); err != nil {
		t.Fatal(err)
	}
	d1 := time.Since(t0)
	fr, _ := FrameOf(f)
	bm, _ := EncodeMap(synthMap(1))
	t0 = time.Now()
	if _, err := RenderOnMap(f, vm, fr, bm); err != nil {
		t.Fatal(err)
	}
	d2 := time.Since(t0)
	t.Logf("Render %v, RenderOnMap %v", d1, d2)
	if d1 > 2*time.Second || d2 > 3*time.Second {
		t.Errorf("worst case too slow: plain %v, map %v", d1, d2)
	}
}
