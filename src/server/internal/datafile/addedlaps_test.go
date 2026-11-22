package datafile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"workout-explorer/internal/workout"
)

// addedLapsPath is the shared sample of F-65 (Go and browser agree): per
// case a golden data file of goldenDir (the draft.bin the edit page has),
// the sample indices the owner picked, and the laps of the Go-built version
// 7 file with those boundaries. The browser's merge over the golden file
// must give the same ranges, order, triggers and strokes, and values within
// quantisation. Rewritten with -update (make corpus-update).
const addedLapsPath = "../../testdata/datafile/added-laps.json"

type addedLapsCase struct {
	Name   string `json:"name"`
	Golden string `json:"golden"`
	// At are the picked samples before the pool snap; an entry that is not
	// a valid boundary (a lap end) is skipped by both sides.
	At   []int `json:"at"`
	Laps []Lap `json:"laps"`
}

// addedLapsSpecs pick the samples from the parsed workout: single sport with
// device laps, without device laps, triathlon with a boundary in T1 and one
// on a leg start, pool swim (snapped).
var addedLapsSpecs = []struct {
	name, golden string
	at           func(w workout.Workout) []int
}{
	{"device-laps", "ride-lap-triggers", func(w workout.Workout) []int {
		n := len(w.Samples.Offset)
		return []int{n / 7, n / 3, n/3 + 60}
	}},
	{"no-device-laps", "ride-no-laps", func(w workout.Workout) []int {
		n := len(w.Samples.Offset)
		return []int{n / 4, n / 2}
	}},
	{"triathlon-t1", "triathlon", func(w workout.Workout) []int {
		var t1 workout.Session
		for _, s := range w.Sessions {
			if s.Sport == "transition" {
				t1 = s
				break
			}
		}
		from, to := workout.SampleRange(w.Samples.Offset, t1.Start, t1.End)
		return []int{(from + to) / 2, from, len(w.Samples.Offset) - 100}
	}},
	{"pool-snap", "pool-swim", func(w workout.Workout) []int {
		n := len(w.Samples.Offset)
		return []int{n/3 + 1, n / 2}
	}},
}

func TestAddedLapsSample(t *testing.T) {
	var cases []addedLapsCase
	for _, sp := range addedLapsSpecs {
		w := parseFile(t, filepath.Join(corpusDir, "synthetic", sp.golden+".fit"))
		at := sp.at(w)
		var snapped []int
		for _, i := range at {
			snapped = append(snapped, workout.SnapLapBoundary(w, i))
		}
		out := workout.AddLaps(w, snapped, 0, len(w.Samples.Offset)-1)
		if len(out.Laps) == len(w.Laps) {
			t.Errorf("%s: no lap added", sp.name)
		}
		b, err := Encode(out)
		if err != nil {
			t.Fatal(err)
		}
		f, err := Decode(b)
		if err != nil {
			t.Fatal(err)
		}
		// The draft file of the case is the golden one: same samples.
		g, err := os.ReadFile(filepath.Join(goldenDir, sp.golden+".bin"))
		if err != nil {
			t.Fatal(err)
		}
		gf, err := Decode(g)
		if err != nil || gf.SampleCount != f.SampleCount {
			t.Fatalf("%s: golden file %v", sp.name, err)
		}
		cases = append(cases, addedLapsCase{sp.name, sp.golden, at, f.Meta.Laps})
	}
	got, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.WriteFile(addedLapsPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(addedLapsPath)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("%s differs from the Go build (%v): run make corpus-update and review the diff", addedLapsPath, err)
	}
}
