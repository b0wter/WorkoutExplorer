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

// F-45 ST-6: with hide_ends on the owner's suggestion skips the stored
// hidden ranges, so the town of a hidden start is never named.
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
	rec := Record{ID: "x", HiddenRanges: workout.HiddenRanges(w)}
	for _, c := range []struct {
		hide bool
		want string
	}{{false, "Morning workout near Aville"}, {true, "Morning workout near Bville"}} {
		rec.HideEnds = c.hide
		if got := s.SuggestedTitle(rec); got != c.want {
			t.Errorf("hide %v: %q, want %q", c.hide, got, c.want)
		}
	}
	rec.HideEnds, rec.HiddenRanges = true, [][2]int{{0, n - 1}}
	if got := s.SuggestedTitle(rec); got != "Morning workout" {
		t.Errorf("fully hidden: %q", got)
	}
	if got := New(Deps{Files: draftFiles{dir: dir}}).SuggestedTitle(rec); got != "" {
		t.Errorf("no places: %q", got)
	}
	if got := New(Deps{Files: draftFiles{dir: t.TempDir()}, Places: ix}).SuggestedTitle(rec); got != "" {
		t.Errorf("no draft: %q", got)
	}
}
