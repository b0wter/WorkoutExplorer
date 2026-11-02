package preview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/page"
	"workout-explorer/internal/workout"
)

// Synthetic files only (ADR-004).
const synthetic = "../../testdata/corpus/synthetic"

func parse(t *testing.T, name string) workout.Workout {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(synthetic, name))
	if err != nil {
		t.Fatal(err)
	}
	w, err := ingest.NewParser(ingest.DefaultLimits, nil).Parse(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// public is the decoded data file of w, as the public build has it.
func public(t *testing.T, w workout.Workout) datafile.File {
	t.Helper()
	bin, err := datafile.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func render(t *testing.T, f datafile.File, title string) (image.Image, []byte) {
	t.Helper()
	b, err := Render(f, page.Build(f, page.Settings{Title: title, PublicID: "x"}))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img, b
}

// TestCorpus (AC 1, PC-5): every synthetic file the parser accepts, with
// hiding off and on, renders 1200 × 630, under 300 KB, the same bytes twice.
func TestCorpus(t *testing.T) {
	entries, err := os.ReadDir(synthetic)
	if err != nil {
		t.Fatal(err)
	}
	p := ingest.NewParser(ingest.DefaultLimits, nil)
	n := 0
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(synthetic, e.Name()))
		w, err := p.Parse(context.Background(), b)
		if err != nil {
			continue
		}
		cut, _ := workout.HideEnds(w)
		for _, v := range []workout.Workout{w, cut} {
			f := public(t, v)
			start := time.Now()
			img, b := render(t, f, "")
			took := time.Since(start)
			if img.Bounds() != image.Rect(0, 0, Width, Height) || len(b) >= 300_000 {
				t.Errorf("%s: %v, %d bytes", e.Name(), img.Bounds(), len(b))
			}
			if _, again := render(t, f, ""); !bytes.Equal(b, again) {
				t.Errorf("%s: not deterministic", e.Name())
			}
			if e.Name() == "ride-12h.fit" {
				t.Logf("ride-12h: %d bytes, %v", len(b), took)
			}
		}
		n++
	}
	if n < 20 {
		t.Errorf("only %d corpus files", n)
	}
}

// routePixels counts pixels close to the route colour.
func routePixels(img image.Image) (n int, box image.Rectangle) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if near(img.At(x, y), routeInk) {
				n++
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return n, box
}

func near(c color.Color, want color.RGBA) bool {
	r, g, b, _ := c.RGBA()
	d := func(a uint32, w uint8) float64 { return math.Abs(float64(a>>8) - float64(w)) }
	return d(r, want.R)+d(g, want.G)+d(b, want.B) < 30
}

// TestHiddenEnds (AC 3, PC-1): the renderer draws only the public file's
// positions; with hiding on their box is smaller than with hiding off, and
// the image differs.
func TestHiddenEnds(t *testing.T) {
	w := parse(t, "ride-30min.fit")
	cut, fully := workout.HideEnds(w)
	if fully {
		t.Fatal("fully hidden")
	}
	off, on := public(t, w), public(t, cut)
	lat, lon := stream(on, "latitude"), stream(on, "longitude")
	points := 0
	for _, run := range Points(on) {
		for _, p := range run {
			for points < len(lat.Valid) && !lat.Valid[points] {
				points++
			}
			if points == len(lat.Valid) || math.Abs(p[0]-float64(lat.Ints[points])*1e-6) > 1e-9 || math.Abs(p[1]-float64(lon.Ints[points])*1e-6) > 1e-9 {
				t.Fatalf("point %v is not a public position", p)
			}
			points++
		}
	}
	area := func(f datafile.File) float64 {
		lo, hi := [2]float64{90, 180}, [2]float64{-90, -180}
		for _, run := range Points(f) {
			for _, p := range run {
				lo, hi = [2]float64{min(lo[0], p[0]), min(lo[1], p[1])}, [2]float64{max(hi[0], p[0]), max(hi[1], p[1])}
			}
		}
		return (hi[0] - lo[0]) * (hi[1] - lo[1])
	}
	if area(on) >= area(off) {
		t.Errorf("box with hiding on %g, off %g", area(on), area(off))
	}
	imgOff, bOff := render(t, off, "")
	imgOn, bOn := render(t, on, "")
	if bytes.Equal(bOff, bOn) {
		t.Error("same image with hiding on")
	}
	nOff, _ := routePixels(imgOff)
	nOn, _ := routePixels(imgOn)
	if nOff < 1000 || nOn < 1000 {
		t.Errorf("route pixels %d off, %d on", nOff, nOn)
	}
}

func stream(f datafile.File, name string) datafile.Stream {
	for _, s := range f.Streams {
		if s.Descriptor.Name == name {
			return s
		}
	}
	return datafile.Stream{}
}

// TestNoRoute (AC 2): indoor and fully hidden draw no route.
func TestNoRoute(t *testing.T) {
	run := parse(t, "run-30min.fit")
	run.Samples.Lat, run.Samples.Lon = nil, nil // as HideEnds leaves a fully hidden route
	for _, f := range []datafile.File{public(t, parse(t, "indoor-no-gps.fit")), public(t, run)} {
		if Points(f) != nil {
			t.Error("points without positions")
		}
		img, _ := render(t, f, "")
		if n, _ := routePixels(img); n != 0 {
			t.Errorf("%d route pixels", n)
		}
	}
}

// file is a data file with the given positions (NaN: missing).
func file(points ...[2]float64) datafile.File {
	lat := datafile.Stream{Descriptor: datafile.Descriptor{Name: "latitude"}}
	lon := datafile.Stream{Descriptor: datafile.Descriptor{Name: "longitude"}}
	for _, p := range points {
		ok := !math.IsNaN(p[0])
		lat.Valid, lon.Valid = append(lat.Valid, ok), append(lon.Valid, ok)
		var a, b int64
		if ok {
			a, b = int64(math.Round(p[0]*1e6)), int64(math.Round(p[1]*1e6))
		}
		lat.Ints, lon.Ints = append(lat.Ints, a), append(lon.Ints, b)
	}
	return datafile.File{SampleCount: len(points), Meta: datafile.Meta{Sport: "cycling", StartTime: "2026-06-01T06:00:00Z"},
		Streams: []datafile.Stream{lat, lon}}
}

// TestShapes: crossings stay filled (one winding for every shape), a gap
// breaks the line, one point and a route within metres render (PC-5).
func TestShapes(t *testing.T) {
	// An X: the centre of the box must be route colour.
	x, _ := render(t, file([2]float64{50, 8}, [2]float64{50.01, 8.01}, [2]float64{50.01, 8}, [2]float64{50, 8.01}), "")
	c := image.Pt((routeBox.Min.X+routeBox.Max.X)/2, (routeBox.Min.Y+routeBox.Max.Y)/2)
	if !near(x.At(c.X, c.Y), routeInk) {
		t.Errorf("crossing at %v is %v", c, x.At(c.X, c.Y))
	}
	// A gap: no line between the two runs.
	gap, _ := render(t, file([2]float64{50, 8}, [2]float64{50, 8.001}, [2]float64{math.NaN(), 0}, [2]float64{50, 8.009}, [2]float64{50, 8.01}), "")
	if near(gap.At(c.X, c.Y), routeInk) {
		t.Error("a gap is bridged")
	}
	for _, f := range []datafile.File{file([2]float64{50, 8}), file([2]float64{50, 8}, [2]float64{50.00001, 8.00001}), file([2]float64{50, 8}, [2]float64{50, 8})} {
		img, _ := render(t, f, "")
		n, box := routePixels(img)
		if n == 0 || !box.In(routeBox) || box.Dx() > 60 {
			t.Errorf("degenerate route: %d pixels in %v", n, box)
		}
	}
}

// TestName (PC-4, OQ-92).
func TestName(t *testing.T) {
	f := face(bold, nameSize)
	defer f.Close()
	const wide = Width - 2*margin
	long := strings.Repeat("abcdefghij", 5)
	for _, c := range []struct{ title, want string }{
		{"Morning loop", "Morning loop"},
		{"  Morning\n\tloop  ", "Morning loop"},
		{long[:40], long[:40]},
		{long[:41], long[:40] + "…"},
		{long[:39] + " x", long[:39] + "…"},
		{"Run 🏃 with 日本 friends\u200d\ufe0f", "Run with friends"},
		{"🏃 日本", "Ride, 1 Jun 2026"},
		{"", "Ride, 1 Jun 2026"},
		{"Ελληνικά и кириллица…", "Ελληνικά и кириллица…"},
	} {
		if got := Name(c.title, "Ride, 1 Jun 2026", f, 1<<20); got != c.want {
			t.Errorf("%q: %q, want %q", c.title, got, c.want)
		}
	}
	// Wide glyphs: cut further until it fits.
	got := Name(strings.Repeat("W", 40), "", f, wide)
	if !strings.HasSuffix(got, "…") || utf8.RuneCountInString(got) >= 40 || f.Metrics().Height == 0 {
		t.Errorf("wide name %q", got)
	}
}

// TestSampleCards writes review cards when WX_CARDS names a directory.
func TestSampleCards(t *testing.T) {
	dir := os.Getenv("WX_CARDS")
	if dir == "" {
		t.Skip("set WX_CARDS to a directory to write the review cards")
	}
	indoor := public(t, parse(t, "indoor-no-gps.fit"))
	indoor.Meta.Summary.AvgHeartRate, indoor.Meta.Summary.MaxHeartRate = "", ""
	hike := public(t, parse(t, "rolling-hills.fit"))
	hike.Meta.Sport = "hiking"
	for _, c := range []struct {
		name, title string
		f           datafile.File
	}{
		{"ride-hr-gain", "Morning loop", public(t, parse(t, "ride-30min.fit"))},
		{"run-pace", "Evening run", public(t, parse(t, "run-30min.fit"))},
		{"pool-swim", "", public(t, parse(t, "pool-swim.fit"))},
		{"indoor-ride-no-hr", "", indoor},
		{"long-name-hike", "Saturday gravel loop over the hills with Anna and Ben", hike},
		{"generic-triathlon", "", public(t, parse(t, "triathlon.fit"))},
	} {
		_, b := render(t, c.f, c.title)
		if err := os.WriteFile(filepath.Join(dir, c.name+".png"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestIconPerSport: each icon key draws its own pixels; sports sharing a key match.
func TestIconPerSport(t *testing.T) {
	draw := func(sport string) []byte {
		img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
		icon(img, sport, 0, 0)
		return img.Pix
	}
	keys := []string{"cycling", "running", "walking", "swimming", "other"}
	for i, a := range keys {
		for _, b := range keys[i+1:] {
			if bytes.Equal(draw(a), draw(b)) {
				t.Errorf("%s and %s draw the same icon", a, b)
			}
		}
	}
	if !bytes.Equal(draw("walking"), draw("hiking")) {
		t.Error("hiking should share the walking icon")
	}
}
