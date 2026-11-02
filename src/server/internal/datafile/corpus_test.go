package datafile

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"

	"workout-explorer/internal/corpustest"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/precompress"
	"workout-explorer/internal/workout"
)

// update rewrites the golden data files: go test ./internal/datafile -run
// '^TestDataFileCorpus$' -update (make corpus-update). The diff of
// testdata/datafile/ is reviewed like code (F-03 design, Golden fixtures).
var update = flag.Bool("update", false, "rewrite the golden data files and remove stale ones")

const (
	corpusDir = "../../testdata/corpus"
	goldenDir = "../../testdata/datafile/synthetic"
)

// noPositions are the corpus files without latitude and longitude (AC 4).
var noPositions = map[string]bool{"indoor-no-gps": true, "indoor-no-distance-no-gps": true, "pool-swim": true}

var parser = ingest.NewParser(ingest.DefaultLimits, nil)

// tryParse parses the FIT file at path; ok is false for a rejected file,
// which has no data file (F-08 design, CLI and corpus driver).
func tryParse(t testing.TB, path string) (w workout.Workout, ok bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w, err = parser.Parse(context.Background(), data)
	var rejected *ingest.Error
	if errors.As(err, &rejected) {
		return workout.Workout{}, false
	}
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return w, true
}

func parseFile(t testing.TB, path string) workout.Workout {
	t.Helper()
	w, ok := tryParse(t, path)
	if !ok {
		t.Fatalf("%s is rejected", path)
	}
	return w
}

func corpusName(path string) string { return strings.TrimSuffix(filepath.Base(path), ".fit") }

// TestRoundTripCorpus encodes and decodes every corpus file, synthetic and
// (where present locally) real: AC 1 to AC 4, and the .br round trip. Data
// files of real files are never written anywhere (ADR-004 item 7, rev. 3).
func TestRoundTripCorpus(t *testing.T) {
	for _, set := range corpustest.Sets {
		files := corpustest.Files(t, corpusDir, set)
		if set == "real" {
			t.Logf("real: %d files (local only)", len(files))
		}
		for _, path := range files {
			name := corpusName(path)
			t.Run(set+"/"+name, func(t *testing.T) {
				w, ok := tryParse(t, path)
				if !ok {
					t.Skip("rejected file: no data file")
				}
				b, err := Encode(w)
				if err != nil {
					t.Fatalf("Encode: %v", err)
				}
				// AC 1: header.
				if string(b[:4]) != Magic {
					t.Errorf("magic = %q", b[:4])
				}
				f, err := Decode(b)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				if f.Version != 2 || f.SampleCount != len(w.Samples.Offset) {
					t.Errorf("version %d, N %d; want 2, %d", f.Version, f.SampleCount, len(w.Samples.Offset))
				}
				// AC 2: alignment.
				for _, s := range f.Streams {
					if s.Descriptor.ByteOffset%4 != 0 {
						t.Errorf("%s at byte %d is not 4-aligned", s.Descriptor.Name, s.Descriptor.ByteOffset)
					}
				}
				// AC 3: every value within its resolution, sentinels where missing.
				compareStreams(t, w, f)
				// ADR-029 rev. 1 (QA-17-r1-1): M and G invariants.
				checkCumulativeStreams(t, w, f)
				// AC 4: no positions, no descriptors for them.
				if set == "synthetic" && noPositions[name] {
					for _, n := range streamNames(f) {
						if n == "latitude" || n == "longitude" {
							t.Errorf("descriptor %s in a file without positions", n)
						}
					}
				}
				// ADR-006 item 4: the .br is a transport encoding of the same bytes.
				br, err := precompress.Brotli(b)
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(brotli.NewReader(bytes.NewReader(br)))
				if err != nil || !bytes.Equal(got, b) {
					t.Errorf(".br does not decompress to the data file (err %v)", err)
				}
			})
		}
	}
	// The three files without positions must exist, or AC 4 is not checked.
	for name := range noPositions {
		if _, err := os.Stat(filepath.Join(corpusDir, "synthetic", name+".fit")); err != nil {
			t.Errorf("AC 4 file missing: %v", err)
		}
	}
}

// TestDataFileCorpus keeps the golden data files and fixtures current: for
// every synthetic corpus file, testdata/datafile/synthetic/<name>.bin is the
// Encode output and <name>.json its Fixture, byte for byte (D-10). There is
// no real side (ADR-004 item 7, rev. 3).
func TestDataFileCorpus(t *testing.T) {
	const fix = "run make corpus-update and review the diff of testdata/datafile/"
	seen := map[string]bool{}
	for _, path := range corpustest.Files(t, corpusDir, "synthetic") {
		name := corpusName(path)
		binPath := filepath.Join(goldenDir, name+".bin")
		jsonPath := filepath.Join(goldenDir, name+".json")
		w, ok := tryParse(t, path)
		if !ok {
			// A rejected file has no data file; a leftover golden one is stale.
			continue
		}
		seen[binPath], seen[jsonPath] = true, true
		t.Run(name, func(t *testing.T) {
			bin, err := Encode(w)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			f, err := Decode(bin)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			fx, err := Fixture(f)
			if err != nil {
				t.Fatalf("Fixture: %v", err)
			}
			// Same digest convention as the F-02 report (F-03 design, Golden fixtures).
			var parsed struct {
				Streams []struct{ Digest string }
			}
			if err := json.Unmarshal(fx, &parsed); err != nil {
				t.Fatal(err)
			}
			if got, want := parsed.Streams[0].Digest, workout.Report(w).Samples.OffsetsDigest; got != want {
				t.Errorf("time_offset digest %s, report offsets_digest %s", got, want)
			}
			if *update {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				for p, b := range map[string][]byte{binPath: bin, jsonPath: fx} {
					if err := os.WriteFile(p, b, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				return
			}
			for p, got := range map[string][]byte{binPath: bin, jsonPath: fx} {
				want, err := os.ReadFile(p)
				if os.IsNotExist(err) {
					t.Errorf("new: no golden file %s: %s", p, fix)
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s differs from the writer output: %s", p, fix)
				}
			}
		})
	}

	// Golden files without a corpus file are stale; so is anything else.
	entries, err := os.ReadDir(goldenDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		p := filepath.Join(goldenDir, e.Name())
		if seen[p] || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if *update && e.Type().IsRegular() {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			continue
		}
		t.Errorf("stale: %s has no synthetic corpus file: %s", p, fix)
	}
}

// TestSizeBudget4h: a 4-hour ride with every stream is under 500 KB
// (500,000 bytes, ADR-002) as data.bin.br (AC 5, NFR Performance).
func TestSizeBudget4h(t *testing.T) {
	const budget = 500_000
	w := parseFile(t, filepath.Join(corpusDir, "synthetic", "ride-4h.fit"))
	if n := len(w.Samples.Offset); n != 14_400 {
		t.Fatalf("ride-4h has %d samples, want 14,400", n)
	}
	bin, err := Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Streams) != len(specs) {
		t.Fatalf("ride-4h has streams %v, want all %d", streamNames(f), len(specs))
	}
	br, err := precompress.Brotli(bin)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ride-4h: data.bin %d bytes, data.bin.br %d bytes (budget %d)", len(bin), len(br), budget)
	if len(br) >= budget {
		t.Errorf("data.bin.br is %d bytes, budget is under %d", len(br), budget)
	}
}

func ride12h(b *testing.B) workout.Workout {
	return parseFile(b, filepath.Join(corpusDir, "synthetic", "ride-12h.fit"))
}

// BenchmarkEncode12h measures Encode on the 12-hour corpus file; the results
// go into testdata/corpus/MEASUREMENTS.md.
func BenchmarkEncode12h(b *testing.B) {
	w := ride12h(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(w); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBrotli12h measures precompress.Brotli on the 12-hour data file
// (ADR-006 item 5).
func BenchmarkBrotli12h(b *testing.B) {
	bin, err := Encode(ride12h(b))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(bin)))
	for b.Loop() {
		if _, err := precompress.Brotli(bin); err != nil {
			b.Fatal(err)
		}
	}
}

// TestSummaryJSON: for every accepted synthetic corpus file, SummaryJSON is
// byte for byte the "summary" value inside Encode's metadata (F-09 round 1,
// row 7).
func TestSummaryJSON(t *testing.T) {
	checked := 0
	for _, path := range corpustest.Files(t, corpusDir, "synthetic") {
		w, ok := tryParse(t, path)
		if !ok {
			continue
		}
		bin, err := Encode(w)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		n := int(binary.LittleEndian.Uint32(bin[10:14]))
		var meta struct {
			Summary json.RawMessage `json:"summary"`
		}
		if err := json.Unmarshal(bin[headerSize:headerSize+n], &meta); err != nil {
			t.Fatal(err)
		}
		got, err := SummaryJSON(w)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, meta.Summary) {
			t.Errorf("%s: SummaryJSON = %s, metadata summary = %s", filepath.Base(path), got, meta.Summary)
		}
		checked++
	}
	if checked < 30 {
		t.Errorf("only %d files checked", checked)
	}
}

// checkCumulativeStreams asserts the ADR-029 item 1 invariants on the
// normalised series and on the decoded streams: present exactly when the
// total is, first value 0, never decreasing, last value equal to the total
// (moving time: the summary's moving_time; gain: R-8's total, to the
// stream's 0.1 m). A single sample has no interval: only M[0] = 0 holds.
func checkCumulativeStreams(t *testing.T, w workout.Workout, f File) {
	t.Helper()
	n := len(w.Samples.Offset)
	for _, c := range []struct {
		name  string
		cum   []float64
		total *float64
		tol   float64
	}{
		{"moving_time", w.Samples.MovingTime, w.Derived.MovingTime, 0},
		{"elevation_gain", w.Samples.Gain, w.Derived.ElevationGain, 0.05 + 1e-9},
	} {
		st, ok := streamByName(f, c.name)
		if (c.total == nil) != (c.cum == nil) || ok != (c.cum != nil) {
			t.Errorf("%s: total %v, series %v, stream written %v", c.name, c.total != nil, c.cum != nil, ok)
			continue
		}
		if !ok {
			continue
		}
		for _, v := range [][]float64{c.cum, st.Values()} {
			if len(v) != n || v[0] != 0 {
				t.Errorf("%s: %d values for %d samples, first %v; want first 0", c.name, len(v), n, v[0])
				continue
			}
			for i := 1; i < n; i++ {
				if math.IsNaN(v[i]) || v[i] < v[i-1] {
					t.Errorf("%s decreases or is missing at sample %d", c.name, i)
					break
				}
			}
			if n > 1 && math.Abs(v[n-1]-*c.total) > c.tol {
				t.Errorf("%s ends at %v, total %v", c.name, v[n-1], *c.total)
			}
		}
	}
	// The browser shows M[N-1] - M[0] against the summary's moving_time.
	if st, ok := streamByName(f, "moving_time"); ok && n > 1 {
		want, err := f.Meta.Summary.MovingTime.Float64()
		if got := st.Values()[n-1] - st.Values()[0]; err != nil || got != want {
			t.Errorf("whole-workout moving time %v, summary moving_time %q", got, f.Meta.Summary.MovingTime)
		}
	}
}
