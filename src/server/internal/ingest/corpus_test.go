package ingest

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"workout-explorer/internal/corpustest"
	"workout-explorer/internal/workout"
)

// update rewrites the golden reports: go test ./internal/ingest -run '^TestCorpus$' -update
// (make corpus-update). The diff of expected/ is reviewed like code (ADR-004).
var update = flag.Bool("update", false, "rewrite the corpus golden reports and remove stale ones")

const corpusDir = "../../testdata/corpus"

// TestCorpus parses every corpus file, checks the invariants of the
// normalised workout and compares its report with the golden file byte for
// byte (ADR-004).
func TestCorpus(t *testing.T) {
	seen := map[string]bool{}
	for _, set := range corpustest.Sets {
		files := corpustest.Files(t, corpusDir, set)
		if set == "real" {
			// Real files are git-ignored (ADR-004 rev. 2): a fresh checkout has none.
			t.Logf("real: %d files (local only)", len(files))
		}
		for _, path := range files {
			name := strings.TrimSuffix(filepath.Base(path), ".fit")
			golden := filepath.Join(corpusDir, "expected", set, name+".json")
			seen[golden] = true
			t.Run(set+"/"+name, func(t *testing.T) {
				got := corpusReport(t, path)
				corpusAccept(t, path)
				if *update {
					if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(golden, got, 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("no golden report %s (run make corpus-update and review it): %v", golden, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("report differs from %s; run make corpus to see the diff", golden)
				}
			})
		}
	}

	// Golden reports without a FIT file are stale.
	for _, set := range corpustest.Sets {
		reports, err := filepath.Glob(filepath.Join(corpusDir, "expected", set, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range reports {
			if seen[r] {
				continue
			}
			if *update {
				if err := os.Remove(r); err != nil {
					t.Fatal(err)
				}
				continue
			}
			t.Errorf("stale golden report %s: no FIT file (run make corpus-update)", r)
		}
	}
}

// corpusAccept runs Accept on an accepted corpus file, synthetic or local
// real, and applies the decoder check of FuzzAccept to the stored bytes
// (ADR-004 item 9). A rejected file must be rejected by Accept the same way.
func corpusAccept(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	crossCheckFile(t, data)
	_, perr := testParser.Parse(context.Background(), data)
	acc, aerr := testParser.Accept(context.Background(), data)
	if perr != nil {
		if aerr == nil {
			t.Errorf("Accept accepts a file that Parse rejects: %v", perr)
		}
		return
	}
	if aerr != nil {
		// Strict (ADR-004 rev. 10, D-29): a corpus file is never refused,
		// unreadable included. Only the kind is named, never a value.
		t.Fatalf("Accept refused an accepted corpus file as %s: %v", kindOf(aerr), aerr)
	}
	checkStored(t, testParser, data, acc)
	decodesToEnd(t, acc.Original)
	checkIdempotent(t, testParser, acc.Original)
	crossCheckFile(t, acc.Original)
}

func corpusReport(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w, err := testParser.Parse(context.Background(), data)
	var ingErr *Error
	if errors.As(err, &ingErr) {
		// A rejected file has a golden rejection report (ADR-004 rev. 6).
		b, err := RejectionJSON(ingErr)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	checkInvariants(t, w)
	b, err := workout.ReportJSON(w)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// checkInvariants checks invariants 1 to 4 of the F-02 design that can be
// seen from outside package workout (5, no panic, holds once Parse returned).
func checkInvariants(t *testing.T, w workout.Workout) {
	t.Helper()
	s := w.Samples
	for i := range s.Offset {
		if (i == 0 && s.Offset[0] < 0) || (i > 0 && s.Offset[i] <= s.Offset[i-1]) {
			t.Fatalf("invariant 1: offsets not strictly increasing from >= 0 at %d", i)
		}
	}
	streams := map[string][]float64{
		"latitude": s.Lat, "longitude": s.Lon, "altitude": s.Altitude, "distance": s.Distance, "speed": s.Speed,
		"heart_rate": s.HeartRate, "cadence": s.Cadence, "power": s.Power, "temperature": s.Temperature,
	}
	for name, v := range streams {
		if v == nil {
			continue
		}
		if len(v) != len(s.Offset) {
			t.Errorf("invariant 1: stream %s has %d values for %d samples", name, len(v), len(s.Offset))
		}
		valid := 0
		for _, x := range v {
			if math.IsInf(x, 0) {
				t.Errorf("invariant 2: stream %s contains Inf", name)
				break
			}
			if !math.IsNaN(x) {
				valid++
			}
		}
		if valid == 0 {
			t.Errorf("invariant 2/4: stream %s present without a valid value", name)
		}
	}
	checkTransitions(t, w)
	// R-14 (v1.4): the valid values of the distance stream never decrease.
	prev := math.NaN()
	for i, x := range s.Distance {
		if math.IsNaN(x) {
			continue
		}
		if !math.IsNaN(prev) && x < prev {
			t.Errorf("R-14: distance falls at sample %d", i)
			break
		}
		prev = x
	}
	if (s.Lat == nil) != (s.Lon == nil) {
		t.Errorf("invariant 3: latitude nil %v, longitude nil %v", s.Lat == nil, s.Lon == nil)
	}
	for i := range s.Lat {
		if math.IsNaN(s.Lat[i]) != math.IsNaN(s.Lon[i]) {
			t.Errorf("invariant 3: half a position at %d", i)
			break
		}
	}
}

// checkTransitions is the R-13 invariant (F-08 v1.3): for each session with
// sport transition that is the first session in file order covering its
// samples, no sample inside it has a valid speed, cadence or power, and its
// totals and those of every lap starting in it lack the dropped fields. No
// figure is asserted, so it runs on local real files too.
func checkTransitions(t *testing.T, w workout.Workout) {
	t.Helper()
	first := func(off int32) int { // rule 1 of the sport timeline
		for i, s := range w.Sessions {
			if s.Start <= off && off <= s.End {
				return i
			}
		}
		return -1
	}
	isTransition := func(off int32) bool {
		i := first(off)
		return i >= 0 && w.Sessions[i].Sport == "transition"
	}
	dropped := func(what string, tt workout.Totals) {
		for name, p := range map[string]*float64{"AvgSpeed": tt.AvgSpeed, "MaxSpeed": tt.MaxSpeed,
			"AvgCadence": tt.AvgCadence, "MaxCadence": tt.MaxCadence, "AvgPower": tt.AvgPower, "MaxPower": tt.MaxPower} {
			if p != nil {
				t.Errorf("R-13: %s still has %s", what, name)
			}
		}
	}
	for i, off := range w.Samples.Offset {
		if !isTransition(off) {
			continue
		}
		for name, v := range map[string][]float64{"speed": w.Samples.Speed, "cadence": w.Samples.Cadence, "power": w.Samples.Power} {
			if v != nil && !math.IsNaN(v[i]) {
				t.Errorf("R-13: valid %s at transition sample %d", name, off)
				return
			}
		}
	}
	for i, s := range w.Sessions {
		if s.Sport == "transition" {
			dropped("transition session "+strconv.Itoa(i), s.Totals)
		}
	}
	for i, l := range w.Laps {
		if isTransition(l.Start) {
			dropped("transition lap "+strconv.Itoa(i), l.Totals)
		}
	}
}

// BenchmarkParse12h measures ingest.Parse on the 12-hour corpus file (F-02
// AC 12); the results go into testdata/corpus/MEASUREMENTS.md.
func BenchmarkParse12h(b *testing.B) {
	data, err := os.ReadFile(filepath.Join(corpusDir, "synthetic", "ride-12h.fit"))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := testParser.Parse(context.Background(), data); err != nil {
			b.Fatal(err)
		}
	}
}

// kindOf is the ingest kind of err, for failure messages.
func kindOf(err error) Kind {
	var ie *Error
	if errors.As(err, &ie) {
		return ie.Kind
	}
	return "not an *Error"
}
