package lifecycle

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/places"
	"workout-explorer/internal/workout"
)

type draftFiles struct {
	PrivateFiles
	dir string
}

func (d draftFiles) Read(id, name string) (io.ReadCloser, int64, error) {
	f, err := os.Open(filepath.Join(d.dir, name))
	if err != nil {
		return nil, 0, err
	}
	st, _ := f.Stat()
	return f, st.Size(), nil
}

// F-45 ST-6, F-55 HR-7: the owner's suggestion skips the hidden sections at
// the stored radii, from the stored steps, so the town of a hidden start is
// never named.
func TestSuggestedTitle(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	n := 2150
	recs := make([]workout.RawRecord, n)
	for i := range recs {
		recs[i] = workout.RawRecord{At: t0.Add(time.Duration(i) * time.Second), Lat: 50 + float64(i)*1.4/111195.08, Lon: 8,
			Altitude: math.NaN(), Distance: math.NaN(), Speed: math.NaN(), HeartRate: math.NaN(), Cadence: math.NaN(), Power: math.NaN(), Temperature: math.NaN()}
	}
	w := workout.Normalise(workout.Recording{FileType: "activity", Records: recs})
	bin, err := datafile.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DraftName), bin, 0o600); err != nil {
		t.Fatal(err)
	}
	ix, _ := places.Parse("Aville\t50\t8\nBville\t50.003597\t8\n")
	s := New(Deps{Files: draftFiles{dir: dir}, Places: ix})
	rec := Record{ID: "x", HidingSteps: workout.StepsOf(w)}
	for _, c := range []struct {
		start, end int
		want       string
	}{
		{0, 0, "Morning workout near Aville"}, {0, 2000, "Morning workout near Aville"},
		{300, 0, "Morning workout near Bville"}, {300, 300, "Morning workout near Bville"},
		{2000, 0, "Morning workout near Bville"},
	} {
		rec.HideStartM, rec.HideEndM = c.start, c.end
		if got := s.SuggestedTitle(rec); got != c.want {
			t.Errorf("(%d, %d): %q, want %q", c.start, c.end, got, c.want)
		}
	}
	rec.HideStartM, rec.HideEndM = 1550, 1550 // 2 × 1550 m > the 3 km route: fully hidden
	if got := s.SuggestedTitle(rec); got != "Morning workout" {
		t.Errorf("fully hidden: %q", got)
	}
	// HR-7: a radius above 0 and no steps yet (migrated row): no town.
	rec.HideStartM, rec.HideEndM, rec.HidingSteps = 0, 50, nil
	if got := s.SuggestedTitle(rec); got != "Morning workout" {
		t.Errorf("no steps: %q", got)
	}
	if got := New(Deps{Files: draftFiles{dir: dir}}).SuggestedTitle(rec); got != "" {
		t.Errorf("no places: %q", got)
	}
	if got := New(Deps{Files: draftFiles{dir: t.TempDir()}, Places: ix}).SuggestedTitle(rec); got != "" {
		t.Errorf("no draft: %q", got)
	}
}

// QA-55-r1-2 / QA-55-r1-1: the sample just inside the hidden window names the
// town, the samples just outside never do; a one-sample window pins the end
// boundary too.
func TestSuggestedTitleBoundary(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	// 1.4 m between samples for the start boundary, 100 m for a one-sample window.
	for _, c := range []struct {
		name      string
		n         int
		stepM     float64
		oneSample bool
	}{{"start", 2150, 1.4, false}, {"end", 31, 100, true}} {
		recs := make([]workout.RawRecord, c.n)
		for i := range recs {
			recs[i] = workout.RawRecord{At: t0.Add(time.Duration(i) * time.Second), Lat: 50 + float64(i)*c.stepM/111195.08, Lon: 8,
				Altitude: math.NaN(), Distance: math.NaN(), Speed: math.NaN(), HeartRate: math.NaN(), Cadence: math.NaN(), Power: math.NaN(), Temperature: math.NaN()}
		}
		w := workout.Normalise(workout.Recording{FileType: "activity", Records: recs})
		bin, err := datafile.Encode(w)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, DraftName), bin, 0o600); err != nil {
			t.Fatal(err)
		}
		steps := workout.StepsOf(w)
		lat := func(i int) float64 { return w.Samples.Lat[i] }
		suggest := func(from int, rec Record) string {
			ix, err := places.Parse(fmt.Sprintf("Before\t%.6f\t8\nInside\t%.6f\t8\nAfter\t%.6f\t8\n", lat(from-1), lat(from), lat(from+1)))
			if err != nil {
				t.Fatal(err)
			}
			return New(Deps{Files: draftFiles{dir: dir}, Places: ix}).SuggestedTitle(rec)
		}
		found := false
		for a := 0; a <= 2000 && !found; a += 50 {
			for b := 0; b <= 2000 && !found; b += 50 {
				f, to, fully := steps.Window(a, b)
				if fully || f < 1 || f >= c.n-1 || (c.oneSample && f != to) || (!c.oneSample && (a != 300 || b != 0)) {
					continue
				}
				found = true
				rec := Record{ID: "x", HidingSteps: steps, HideStartM: a, HideEndM: b}
				if got := suggest(f, rec); got != "Morning workout near Inside" {
					t.Errorf("%s (%d, %d): %q", c.name, a, b, got)
				}
			}
		}
		if !found {
			t.Errorf("%s: no radii pair for the case", c.name)
		}
	}
}
