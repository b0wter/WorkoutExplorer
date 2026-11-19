package lifecycle

import (
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
