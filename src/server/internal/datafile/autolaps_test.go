package datafile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"workout-explorer/internal/workout"
)

// autoLapsPath is F-66's Go and browser agreement sample, as added-laps.json:
// per case a golden file and the laps of the Go-built file after
// RemoveAutoLaps (and, with At, the added-boundary merge over the result).
// Rewritten with -update (make corpus-update).
const autoLapsPath = "../../testdata/datafile/auto-laps.json"

var autoLapsSpecs = []struct {
	name, golden string
	at           func(n int) []int
}{
	{"ride-lap-triggers", "ride-lap-triggers", nil},
	{"ride-lap-triggers-added", "ride-lap-triggers", func(n int) []int { return []int{n / 7, n / 3} }},
	{"triathlon", "triathlon", nil},
	{"pool-swim", "pool-swim", nil},
	{"pool-swim-mixed", "pool-swim-mixed", nil},
	{"ride-no-laps", "ride-no-laps", nil},
	{"chained-ride-ride", "chained-ride-ride", nil},
	{"chained-ride-run", "chained-ride-run", nil},
	{"chained-ride-run-added", "chained-ride-run", func(n int) []int { return []int{n / 5, n / 2, n * 4 / 5} }},
	{"pool-swim-25yd", "pool-swim-25yd", nil},
	{"pool-swim-33m", "pool-swim-33m", nil},
	{"pool-swim-added", "pool-swim", func(n int) []int { return []int{n / 4, n / 2} }},
	{"pool-swim-mixed-added", "pool-swim-mixed", func(n int) []int { return []int{n / 4, n / 2} }},
	{"triathlon-added", "triathlon", func(n int) []int { return []int{n / 6, n / 2, n * 5 / 6} }},
}

func TestAutoLapsSample(t *testing.T) {
	var cases []addedLapsCase
	merged := 0
	for _, sp := range autoLapsSpecs {
		w := parseFile(t, filepath.Join(corpusDir, "synthetic", sp.golden+".fit"))
		n := len(w.Samples.Offset)
		out := workout.RemoveAutoLaps(w)
		if len(out.Laps) != len(w.Laps) {
			merged++
		}
		at := []int{}
		if sp.at != nil {
			at = sp.at(n)
			var snapped []int
			for _, i := range at {
				snapped = append(snapped, workout.SnapLapBoundary(out, i))
			}
			out = workout.AddLaps(out, snapped, 0, n-1)
		}
		b, err := Encode(out)
		if err != nil {
			t.Fatal(err)
		}
		f, err := Decode(b)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, addedLapsCase{sp.name, sp.golden, at, f.Meta.Laps})
	}
	if merged == 0 {
		t.Error("no case removes a lap")
	}
	got, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.WriteFile(autoLapsPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(autoLapsPath)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("%s differs from the Go build (%v): run make corpus-update and review the diff", autoLapsPath, err)
	}
}
