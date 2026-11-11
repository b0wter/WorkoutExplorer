package publish

import (
	"bytes"
	"html"
	"math"
	"os"
	"path/filepath"
	"testing"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/places"
	"workout-explorer/internal/workout"
)

// F-45 ST-4, AC "hidden start gives the town of the first public point".
// A lies at the start (50 N, 8 E), B 400 m north; the first public point
// (more than 300 m from the start) is nearer B.
func TestSuggestedTitleFollowsHiding(t *testing.T) {
	ix, err := places.Parse("Aville\t50\t8\nBville\t50.003597\t8\n")
	if err != nil {
		t.Fatal(err)
	}
	w := gen(2150, func(i int) (float64, float64) { return 0, float64(i) * 1.4 })
	p := fakeParser{"walk": w}
	for _, c := range []struct {
		hide bool
		want string
	}{{false, "Morning workout near Aville"}, {true, "Morning workout near Bville"}} {
		fx := newFixtureWith(t, []byte("walk"), p)
		fx.a.d.Places = ix
		d := draft()
		d.HideEnds = c.hide
		fx.change(t, d, published(d))
		out := fx.read(t)
		m := ogTitle.FindSubmatch(out.page)
		if m == nil || html.UnescapeString(string(m[1])) != c.want {
			t.Fatalf("hide %v: og:title %q, want %q", c.hide, m, c.want)
		}
		if !bytes.Contains(out.page, []byte("<title>"+c.want+"</title>")) {
			t.Errorf("hide %v: browser title is not %q", c.hide, c.want)
		}
		if c.hide { // the hidden part's town appears in no public file
			entries, _ := os.ReadDir(fx.folder())
			for _, e := range entries {
				b, _ := os.ReadFile(filepath.Join(fx.folder(), e.Name()))
				if bytes.Contains(b, []byte("Aville")) {
					t.Errorf("%s names the hidden start's town", e.Name())
				}
			}
		}

		// A typed title wins; clearing it brings the suggestion back.
		typed := d
		typed.Title = "Mine"
		fx.change(t, published(d), published(typed))
		if m := ogTitle.FindSubmatch(fx.read(t).page); string(m[1]) != "Mine" {
			t.Errorf("typed title lost: %s", m[1])
		}
		fx.change(t, published(typed), published(d))
		if m := ogTitle.FindSubmatch(fx.read(t).page); html.UnescapeString(string(m[1])) != c.want {
			t.Errorf("cleared title: %s", m[1])
		}
	}
}

// No location, a route entirely hidden, no places: sport only, no error.
func TestSuggestedTitleWithoutTown(t *testing.T) {
	ix, _ := places.Parse("Aville\t50\t8\n")
	p := fakeParser{
		"indoor": gen(600, func(int) (float64, float64) { return math.NaN(), math.NaN() }),
		"short":  gen(357, func(i int) (float64, float64) { return float64(i) * 1.4, 0 }),
		"far":    gen(100, func(i int) (float64, float64) { return 30000 + float64(i), 0 }), // > 25 km from A
	}
	for _, c := range []struct {
		name string
		hide bool
		want string
	}{{"indoor", false, "Morning workout"}, {"short", true, "Morning workout"}, {"far", false, "Morning workout"}, {"short", false, "Morning workout near Aville"}} {
		fx := newFixtureWith(t, []byte(c.name), p)
		fx.a.d.Places = ix
		d := draft()
		d.HideEnds = c.hide
		fx.change(t, d, published(d))
		if m := ogTitle.FindSubmatch(fx.read(t).page); html.UnescapeString(string(m[1])) != c.want {
			t.Errorf("%s hide %v: %q, want %q", c.name, c.hide, m[1], c.want)
		}
	}
	// Without place data: sport and date as before.
	fx := newFixtureWith(t, []byte("short"), p)
	d := draft()
	fx.change(t, d, published(d))
	if m := ogTitle.FindSubmatch(fx.read(t).page); string(m[1]) != "Workout, 1 Jun 2026" {
		t.Errorf("no places: %s", m[1])
	}
}

// Owner side (lifecycle.SuggestedTitle's read) and public side name the same
// town, and turning hiding on after publish removes the old town from every
// public file (F-45 ST-6, AC 4, AC 35).
func TestSuggestedTitleOwnerAgreesAndHidingAfterPublish(t *testing.T) {
	ix, _ := places.Parse("Aville\t50\t8\nBville\t50.003597\t8\nCville\t50.02\t8\n")
	for _, hide := range []bool{false, true} {
		w := gen(2150, func(i int) (float64, float64) { return 0, float64(i) * 1.4 })
		bin, err := datafile.Encode(w)
		if err != nil {
			t.Fatal(err)
		}
		var skip [][2]int
		if hide {
			skip = workout.HiddenRanges(w)
		}
		_, lat, lon, ok, err := datafile.FirstPositionAt(bytes.NewReader(bin), int64(len(bin)), skip)
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
		town, _ := ix.Nearest(lat, lon)
		fx := newFixtureWith(t, []byte("walk"), fakeParser{"walk": w})
		fx.a.d.Places = ix
		d := draft()
		d.HideEnds = hide
		fx.change(t, d, published(d))
		if m := ogTitle.FindSubmatch(fx.read(t).page); html.UnescapeString(string(m[1])) != "Morning workout near "+town {
			t.Errorf("hide %v: public %q, owner town %q", hide, m[1], town)
		}
	}

	fx := newFixtureWith(t, []byte("walk"), fakeParser{"walk": gen(2150, func(i int) (float64, float64) { return 0, float64(i) * 1.4 })})
	fx.a.d.Places = ix
	d := draft()
	pub := published(d)
	fx.change(t, d, pub)
	if m := ogTitle.FindSubmatch(fx.read(t).page); string(m[1]) != "Morning workout near Aville" {
		t.Fatalf("before hiding: %s", m[1])
	}
	on := pub
	on.HideEnds = true
	fx.change(t, pub, on)
	if m := ogTitle.FindSubmatch(fx.read(t).page); string(m[1]) != "Morning workout near Bville" {
		t.Errorf("after hiding: %s", m[1])
	}
	entries, _ := os.ReadDir(fx.folder())
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(fx.folder(), e.Name()))
		if bytes.Contains(b, []byte("Aville")) {
			t.Errorf("%s keeps the old town", e.Name())
		}
	}
}
