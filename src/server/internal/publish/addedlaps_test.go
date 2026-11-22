package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/workout"
)

// F-65 AL-2 to AL-7 at the public build (SH-4, UP-6, ADR-060). Synthetic
// files only (ADR-004).

// lapKeys are the keys of a lap of Format §5.3: no coordinate.
var lapKeys = []string{"end", "end_index", "start", "start_index", "stroke", "totals", "trigger"}

// buildOf is the in-memory public build of r: data file bytes and the
// preview bytes, nothing written.
func buildOf(t *testing.T, fx *fixture, r lifecycle.Record) (bin, img []byte, f datafile.File) {
	t.Helper()
	b, err := fx.a.build(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	bin, _ = b.artefacts[0].data()
	img, _ = b.artefacts[2].data()
	if f, err = datafile.Decode(bin); err != nil {
		t.Fatal(err)
	}
	return bin, img, f
}

// metaWithoutLaps is the data file's metadata with "laps" removed, and the
// raw laps.
func metaWithoutLaps(t *testing.T, bin []byte, f datafile.File) (rest map[string]json.RawMessage, laps []map[string]json.RawMessage) {
	t.Helper()
	if err := json.Unmarshal(metadata(bin, f), &rest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rest["laps"], &laps); err != nil {
		t.Fatal(err)
	}
	delete(rest, "laps")
	// Byte offsets move with the metadata length; the columns are compared whole.
	var streams []map[string]any
	if err := json.Unmarshal(rest["streams"], &streams); err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		delete(s, "byte_offset")
	}
	rest["streams"], _ = json.Marshal(streams)
	return rest, laps
}

// checkOnlyLapsDiffer is the privacy probe: the build with boundaries has
// byte-equal columns, metadata other than laps and preview as the build
// without; every lap has exactly §5.3's keys; each boundary inside a hidden
// part has no public position. It returns the public laps.
func checkOnlyLapsDiffer(t *testing.T, fx *fixture, d lifecycle.Record, hiddenAt []int) []datafile.Lap {
	t.Helper()
	plain := d
	plain.AddedLapBoundaries = nil
	b0, i0, f0 := buildOf(t, fx, plain)
	b1, i1, f1 := buildOf(t, fx, d)
	h0, h1 := 14+f0.MetadataLength, 14+f1.MetadataLength
	if !bytes.Equal(b0[align(h0):], b1[align(h1):]) {
		t.Fatal("a column differs")
	}
	if !bytes.Equal(i0, i1) {
		t.Error("the preview differs")
	}
	m0, _ := metaWithoutLaps(t, b0, f0)
	m1, laps := metaWithoutLaps(t, b1, f1)
	j0, _ := json.Marshal(m0)
	j1, _ := json.Marshal(m1)
	if !bytes.Equal(j0, j1) {
		t.Errorf("metadata other than laps differs:\n%s\n%s", j0, j1)
	}
	for _, l := range laps {
		var keys []string
		for k := range l {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, lapKeys) {
			t.Errorf("lap keys %v", keys)
		}
	}
	if lat, ok := stream(f1, "latitude"); ok {
		for _, i := range hiddenAt {
			if i >= 0 && i < len(lat.Valid) && lat.Valid[i] {
				t.Errorf("a boundary in a hidden part at %d has a public position", i)
			}
		}
	}
	return f1.Meta.Laps
}

func align(n int) int { return (n + 3) &^ 3 }

// TestAddedLapsPrivacyCorpus: over the corpus at radii per end, with and
// without a trim, boundaries in hidden parts, on the first visible sample and
// on dropouts change nothing public but the laps.
func TestAddedLapsPrivacyCorpus(t *testing.T) {
	entries, err := os.ReadDir(synthetic)
	if err != nil {
		t.Fatal(err)
	}
	parser := ingest.NewParser(ingest.DefaultLimits, nil)
	var added, hidden atomic.Int64
	t.Run("corpus", func(t *testing.T) { // parallel files; the counts are read after
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".fit") || strings.HasPrefix(e.Name(), "ride-12h") {
				continue
			}
			orig, err := os.ReadFile(filepath.Join(synthetic, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			w, err := parser.Parse(context.Background(), orig)
			if err != nil || len(w.Samples.Offset) < 50 {
				continue
			}
			t.Run(e.Name(), func(t *testing.T) {
				t.Parallel()
				fx := newFixtureWith(t, []byte(e.Name()), fakeParser{e.Name(): w}) // parsed once
				n := len(w.Samples.Offset)
				for _, rr := range [][2]int{{0, 0}, {50, 0}, {0, 300}, {300, 300}, {2000, 2000}} {
					for _, trim := range []bool{false, true} {
						d := published(draft())
						d.HideStartM, d.HideEndM = rr[0], rr[1]
						lo, hi := 0, n-1
						if trim {
							lo, hi = n/10, n-n/10
							d.TrimStartAt, d.TrimEndAt = instant(w, lo), instant(w, hi)
							if _, _, ok := workout.TrimWindow(w, d.TrimStartAt, d.TrimEndAt); !ok {
								continue
							}
						}
						// Candidates: near both ends, the middle, the first visible
						// sample and every dropout start of the public output.
						cands := []int{lo + 1, lo + 3, lo + (hi-lo)/50, (lo + hi) / 2, hi - (hi-lo)/50, hi - 3, hi - 1}
						pw, _ := publicTransform(workout.Trim(w, lo, hi), d)
						var hiddenAt []int
						if pw.Samples.Lat != nil {
							first := true
							for j := 1; j < len(pw.Samples.Lat); j++ {
								has := !math.IsNaN(pw.Samples.Lat[j])
								if has && first {
									cands = append(cands, lo+j)
									first = false
								}
								if !has && !math.IsNaN(pw.Samples.Lat[j-1]) {
									cands = append(cands, lo+j)
								}
							}
						}
						var at []time.Time
						for _, i := range cands {
							k := workout.SnapLapBoundary(w, i)
							at = append(at, *instant(w, k))
							if w.Samples.Lat != nil && pw.Samples.Lat != nil && !math.IsNaN(w.Samples.Lat[k]) && k > lo && k < hi && math.IsNaN(pw.Samples.Lat[k-lo]) {
								hiddenAt = append(hiddenAt, k-lo)
							}
						}
						d.AddedLapBoundaries = at
						hidden.Add(int64(len(hiddenAt)))
						laps := checkOnlyLapsDiffer(t, fx, d, hiddenAt)
						for _, l := range laps {
							if l.Trigger == workout.TriggerAdded {
								added.Add(1)
							}
						}
					}
				}
			})
		}
	})
	if added.Load() < 100 || hidden.Load() < 20 {
		t.Errorf("only %d added laps built, %d boundaries in hidden parts: the probe proves little", added.Load(), hidden.Load())
	}
}

// AL-7: a boundary change rebuilds in place (the old page stays until the
// new one replaces it); the laps table, also without JavaScript, shows the
// added laps; removing the last one on a file without laps returns to splits.
func TestAddedLapsRebuildInPlace(t *testing.T) {
	fx := newFixture(t, "ride-no-laps.fit")
	w, err := ingest.NewParser(ingest.DefaultLimits, nil).Parse(context.Background(), fx.orig)
	if err != nil {
		t.Fatal(err)
	}
	d := published(draft())
	fx.change(t, draft(), d)
	_, html := fx.complete(t)
	if !strings.Contains(lapsSection(html), `class="laps" hidden`) {
		t.Fatal("splits only expected before")
	}
	removed := false
	fx.a.step = func(name string) error {
		if name == "remove "+indexName || name == "remove folder" {
			removed = true
		}
		return nil
	}
	with := d
	with.AddedLapBoundaries = []time.Time{*instant(w, len(w.Samples.Offset)/2)}
	fx.change(t, d, with)
	if removed {
		t.Error("a boundary change removed the page first")
	}
	o := fx.read(t)
	if len(o.file.Meta.Laps) != 2 || o.file.Meta.Laps[0].Trigger != "added" || o.file.Meta.Laps[1].Trigger != "" {
		t.Fatalf("laps %+v", o.file.Meta.Laps)
	}
	laps := lapsSection(o.page)
	if strings.Contains(laps, `class="laps" hidden`) || !strings.Contains(laps, "<table") || !strings.Contains(laps, "Added") {
		t.Errorf("laps module:\n%s", laps)
	}
	fx.change(t, with, d)
	_, html = fx.complete(t)
	if !strings.Contains(lapsSection(html), `class="laps" hidden`) {
		t.Error("removing the last boundary did not return to splits")
	}
	if names := fx.names(t); len(names) != 4 {
		t.Errorf("old files left: %v", names)
	}
}

// AL-3, AL-6: a stored boundary that no longer fits (outside the trim
// window, on a device lap end, outside the recording) is skipped by the
// build, never an error; a trimmed-away part takes its added laps with it.
func TestAddedLapsBuildSkipsStale(t *testing.T) {
	w := trimTrack(false)
	fx := pinFixture(t, w)
	d := published(draft())
	d.TrimStartAt, d.TrimEndAt = instant(w, 100), instant(w, 800)
	d.AddedLapBoundaries = []time.Time{*instant(w, 50), *instant(w, 100), *instant(w, 400), *instant(w, 850), w.StartTime.Add(-time.Hour)}
	_, _, f := buildOf(t, fx, d)
	if len(f.Meta.Laps) != 2 || f.Meta.Laps[0].EndIndex-1 != 300 || f.Meta.Laps[0].Trigger != "added" {
		t.Fatalf("laps %+v", f.Meta.Laps)
	}
}

// AL-2 through the real port: entries in hidden parts are judged on the
// trim only, at every radius; window ends and lap ends are flagged; a pool
// boundary resolves to the snapped sample.
func TestCheckEditsLaps(t *testing.T) {
	w := trimTrack(false) // no device laps: lap ends are the first and last sample
	fx := pinFixture(t, w)
	for _, rr := range [][2]int{{0, 0}, {50, 50}, {300, 300}, {2000, 2000}} {
		d := draft()
		d.HideStartM, d.HideEndM = rr[0], rr[1]
		at := []time.Time{*instant(w, 1), *instant(w, 898), *instant(w, 0), *instant(w, 899), w.StartTime.Add(-time.Second)}
		_, cs, err := fx.a.CheckEdits(context.Background(), d, nil, at)
		if err != nil {
			t.Fatal(err)
		}
		want := []lifecycle.LapCheck{
			{Resolved: at[0], Kept: true}, {Resolved: at[1], Kept: true},
			{Resolved: at[2], OnBoundary: true}, {Resolved: at[3], OnBoundary: true}, {Outside: true},
		}
		if !reflect.DeepEqual(cs, want) {
			t.Errorf("radii %v: %+v", rr, cs)
		}
		d.TrimStartAt, d.TrimEndAt = instant(w, 100), instant(w, 800)
		at = []time.Time{*instant(w, 101), *instant(w, 100), *instant(w, 800), *instant(w, 50)}
		if _, cs, _ = fx.a.CheckEdits(context.Background(), d, nil, at); !cs[0].Kept || cs[1].Kept || cs[2].Kept || cs[3].Kept {
			t.Errorf("radii %v, trim: %+v", rr, cs)
		}
	}
	pool, err := ingest.NewParser(ingest.DefaultLimits, nil).Parse(context.Background(), mustRead(t, filepath.Join(synthetic, "pool-swim.fit")))
	if err != nil {
		t.Fatal(err)
	}
	fp := pinFixture(t, pool)
	i := len(pool.Samples.Offset)/3 + 1
	k := workout.SnapLapBoundary(pool, i)
	_, cs, err := fp.a.CheckEdits(context.Background(), draft(), nil, []time.Time{*instant(pool, i)})
	if err != nil || k == i || !cs[0].Resolved.Equal(*instant(pool, k)) {
		t.Errorf("pool snap %d → %d: %+v %v", i, k, cs, err)
	}
}

// AL-3 at the real port, where a device lap end and a window end are not also
// each other: the check must flag each on its own.
func TestCheckEditsLapBoundaryKinds(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	at := func(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }
	recs := make([]workout.RawRecord, 900)
	for i := range recs {
		recs[i] = workout.RawRecord{At: at(i), Lat: math.NaN(), Lon: math.NaN(), Cadence: math.NaN(), Power: math.NaN(), Temperature: math.NaN(), Distance: float64(i) * 3}
	}
	w := workout.Normalise(workout.Recording{FileType: "activity", Records: recs,
		Laps: []workout.RawLap{{Start: at(0), End: at(300), Trigger: "time"}, {Start: at(301), End: at(899), Trigger: "time"}}})
	fx := pinFixture(t, w)
	d := draft()
	d.TrimStartAt, d.TrimEndAt = instant(w, 100), instant(w, 800)
	_, cs, err := fx.a.CheckEdits(context.Background(), d, nil, []time.Time{*instant(w, 300), *instant(w, 100), *instant(w, 800), *instant(w, 500)})
	if err != nil {
		t.Fatal(err)
	}
	if !cs[0].OnBoundary || !cs[0].Kept || !cs[1].OnBoundary || !cs[2].OnBoundary || cs[3].OnBoundary || !cs[3].Kept {
		t.Errorf("%+v", cs)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
