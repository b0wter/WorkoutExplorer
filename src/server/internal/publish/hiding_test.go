package publish

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
	pagevm "workout-explorer/internal/page"
	"workout-explorer/internal/preview"
	"workout-explorer/internal/workout"
)

// F-16 start and end hiding at the public build (SH-4, AP-3; design H-4 to
// H-9). Synthetic files only (ADR-004).

const (
	notice     = `<p class="hint">The entire route has been hidden by the uploader.</p>`
	indoorHint = `<p class="hint">No location data in this workout.</p>`
	mapSlot    = `id="map-slot"`
)

// fakeParser returns a workout normalised from generated records, keyed by
// the "original" bytes, for tracks no corpus file has (a 500 m walk).
type fakeParser map[string]workout.Workout

func (p fakeParser) Parse(_ context.Context, data []byte) (workout.Workout, error) {
	w, ok := p[string(data)]
	if !ok {
		return workout.Workout{}, fmt.Errorf("no such workout")
	}
	return w, nil
}

// output is what one build left in the folder.
type output struct {
	names []string
	bin   []byte
	file  datafile.File
	page  []byte
}

func newFixtureWith(t *testing.T, orig []byte, parser Parser) *fixture {
	t.Helper()
	tmpl, err := LoadTemplate(webDir)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fx := &fixture{orig: orig, store: lifecycle.NewStorage(1<<40, 0)}
	fx.a, err = Open(dir, Deps{Template: tmpl, Originals: originals{id: orig}, Parser: parser, Storage: fx.store, BaseURL: testBaseURL})
	if err != nil {
		t.Fatal(err)
	}
	fx.root = filepath.Join(dir, "public")
	return fx
}

// read checks the folder holds exactly the data file, its .br (which
// decompresses to it), index.html and the preview (drawn from that data file
// and the title only, F-23 PC-1), and returns them.
func (fx *fixture) read(t *testing.T) output {
	t.Helper()
	name, page := fx.complete(t)
	if name == "" {
		t.Fatal("no page")
	}
	o := output{names: fx.names(t), page: page}
	if !slices.Equal(o.names, whole(t, name, page)) {
		t.Fatalf("files %v: a new public file must be checked for hidden data here", o.names)
	}
	var err error
	if o.bin, err = os.ReadFile(filepath.Join(fx.folder(), name)); err != nil {
		t.Fatal(err)
	}
	br, err := os.ReadFile(filepath.Join(fx.folder(), name+".br"))
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := io.ReadAll(brotli.NewReader(bytes.NewReader(br))); err != nil || !bytes.Equal(plain, o.bin) {
		t.Fatalf(".br does not decompress to the data file: %v", err)
	}
	if o.file, err = datafile.Decode(o.bin); err != nil {
		t.Fatal(err)
	}
	img, err := os.ReadFile(filepath.Join(fx.folder(), previewOf(t, page)))
	if err != nil {
		t.Fatal(err)
	}
	m := ogTitle.FindSubmatch(page)
	if m == nil {
		t.Fatal("no og:title")
	}
	want, err := preview.Render(o.file, pagevm.Build(o.file, pagevm.Settings{Title: html.UnescapeString(string(m[1]))}))
	if err != nil || !bytes.Equal(img, want) {
		t.Fatalf("the preview is not the render of the public data file: %v", err)
	}
	return o
}

var (
	ogTitle       = regexp.MustCompile(`<meta property="og:title" content="([^"]*)" />`)
	ogDescription = regexp.MustCompile(`<meta property="og:description" content="[^"]*" />`)
)

// publishBoth publishes the same original once with hiding off and once
// with it on, each in a fresh public area.
func publishBoth(t *testing.T, orig []byte, parser Parser) (off, on output) {
	t.Helper()
	for _, hide := range []bool{false, true} {
		fx := newFixtureWith(t, orig, parser)
		d := draft()
		d.HideEnds = hide
		fx.change(t, d, published(d))
		if hide {
			on = fx.read(t)
		} else {
			off = fx.read(t)
		}
	}
	return off, on
}

func stream(f datafile.File, name string) (datafile.Stream, bool) {
	for _, s := range f.Streams {
		if s.Descriptor.Name == name {
			return s, true
		}
	}
	return datafile.Stream{}, false
}

func metadata(b []byte, f datafile.File) []byte { return b[14 : 14+f.MetadataLength] }

// checkHidden is AC 1 to 4 for one original: w is the full parse, as the
// draft has it. It returns whether the route was fully hidden.
func checkHidden(t *testing.T, name string, orig []byte, parser Parser, w workout.Workout) bool {
	t.Helper()
	off, on := publishBoth(t, orig, parser)

	// AC 4: hiding off publishes the full track, the draft's data file.
	if draftBin, err := datafile.Encode(w); err != nil || !bytes.Equal(off.bin, draftBin) {
		t.Fatalf("%s: hiding off: public data file is not the draft's (%v)", name, err)
	}
	// The build used the transform's output, nothing else.
	cut, fully := workout.HideEnds(w)
	if want, _ := datafile.Encode(cut); !bytes.Equal(on.bin, want) {
		t.Fatalf("%s: hiding on: data file is not the cut workout's", name)
	}

	// AC 2, 3: offsets and every other column bit for bit, summary, laps
	// and sessions identical.
	var onNames, offNames []string
	for _, s := range off.file.Streams {
		if n := s.Descriptor.Name; n != "latitude" && n != "longitude" {
			offNames = append(offNames, n)
			o, ok := stream(on.file, n)
			if !ok || !slices.Equal(o.Ints, s.Ints) || !slices.Equal(o.Valid, s.Valid) {
				t.Fatalf("%s: stream %s differs with hiding on", name, n)
			}
		}
	}
	for _, s := range on.file.Streams {
		if n := s.Descriptor.Name; n != "latitude" && n != "longitude" {
			onNames = append(onNames, n)
		}
	}
	if !slices.Equal(onNames, offNames) {
		t.Fatalf("%s: streams %v, hiding off %v", name, onNames, offNames)
	}
	mOn, mOff := on.file.Meta, off.file.Meta
	mOn.Streams, mOff.Streams = nil, nil
	if fmt.Sprintf("%#v", mOn) != fmt.Sprintf("%#v", mOff) {
		t.Fatalf("%s: metadata other than the descriptors differs", name)
	}
	_, hadPos := stream(off.file, "latitude")
	_, hasPos := stream(on.file, "latitude")
	if !fully && !bytes.Equal(metadata(on.bin, on.file), metadata(off.bin, off.file)) {
		t.Fatalf("%s: metadata differs although the route is not fully hidden", name)
	}

	// AC 1: no hidden position anywhere. Hidden samples are sentinels; every
	// public position is a hiding-off one and lies at or after the first
	// sample more than 300 m from the first position, and at or before the
	// last one more than 300 m from the last (measured on the full parse).
	if hadPos {
		latOff, _ := stream(off.file, "latitude")
		lonOff, _ := stream(off.file, "longitude")
		lo, hi := circleBounds(w)
		if hasPos {
			latOn, _ := stream(on.file, "latitude")
			lonOn, _ := stream(on.file, "longitude")
			for i := range latOn.Valid {
				if latOn.Valid[i] != lonOn.Valid[i] {
					t.Fatalf("%s: half position at %d", name, i)
				}
				if !latOn.Valid[i] {
					continue
				}
				if i < lo || i > hi {
					t.Fatalf("%s: sample %d is public but inside a hidden section", name, i)
				}
				if !latOff.Valid[i] || latOn.Ints[i] != latOff.Ints[i] || lonOn.Ints[i] != lonOff.Ints[i] {
					t.Fatalf("%s: public position %d is not the recorded one", name, i)
				}
			}
		}
		// No hidden coordinate as text in the page or the metadata.
		text := [][]byte{on.page, metadata(on.bin, on.file)}
		seen := map[string]bool{}
		for i := range latOff.Valid {
			visible := false
			if hasPos {
				l, _ := stream(on.file, "latitude")
				visible = l.Valid[i]
			}
			if !latOff.Valid[i] || visible {
				continue
			}
			for _, v := range []int64{latOff.Ints[i], lonOff.Ints[i]} {
				s := fmt.Sprintf("%.4f", float64(v)/1e6)
				if seen[s] {
					continue
				}
				seen[s] = true
				for _, b := range text {
					if bytes.Contains(b, []byte(s)) {
						t.Fatalf("%s: hidden coordinate %s appears as text", name, s)
					}
				}
			}
		}
	}

	// ADR-030: the ranges the upload stores mark exactly the samples whose
	// position the public file lacks although the draft has one.
	ranges := workout.HiddenRanges(w)
	if hadPos {
		latOff, _ := stream(off.file, "latitude")
		latOn, _ := stream(on.file, "latitude")
		for i := range latOff.Valid {
			in := false
			for _, r := range ranges {
				in = in || (r[0] <= i && i <= r[1])
			}
			if public := hasPos && latOn.Valid[i]; public != (latOff.Valid[i] && !in) {
				t.Fatalf("%s: sample %d public %v, in a range %v (%v)", name, i, public, in, ranges)
			}
		}
		if fully && (len(ranges) != 1 || ranges[0] != [2]int{0, off.file.SampleCount - 1}) {
			t.Fatalf("%s: fully hidden, ranges %v", name, ranges)
		}
	} else if ranges != nil {
		t.Fatalf("%s: ranges %v without positions", name, ranges)
	}

	// H-5, H-6: the page equals the hiding-off page but for the data file
	// and preview names; a fully hidden route shows the notice in place of
	// the map (and the card's elapsed time, F-23 PC-3).
	offName, onName := dataName(t, off.page), dataName(t, on.page)
	same := bytes.ReplaceAll(off.page, []byte(offName), []byte(onName))
	same = bytes.ReplaceAll(same, []byte(previewOf(t, off.page)), []byte(previewOf(t, on.page)))
	if fully {
		same = ogDescription.ReplaceAll(same, ogDescription.Find(on.page))
	}
	switch {
	case fully:
		if hasPos || !hadPos {
			t.Fatalf("%s: fully hidden with position columns %v (had %v)", name, hasPos, hadPos)
		}
		if !bytes.Contains(on.page, []byte(notice)) || bytes.Contains(on.page, []byte(mapSlot)) ||
			bytes.Contains(on.page, []byte("has-map")) || bytes.Contains(on.page, []byte(indoorHint)) {
			t.Fatalf("%s: fully hidden page lacks the notice or keeps the map", name)
		}
		same = bytes.Replace(same, []byte(`<div class="page has-map">`), []byte(`<div class="page">`), 1)
		same = bytes.Replace(same, []byte(`<div id="map-slot" class="map-slot"></div>`), nil, 1)
		got := bytes.Replace(on.page, []byte(notice), nil, 1)
		if !bytes.Equal(got, same) {
			t.Fatalf("%s: fully hidden page differs from hiding off in more than the map", name)
		}
	default:
		if bytes.Contains(on.page, []byte(notice)) {
			t.Fatalf("%s: notice without a fully hidden route", name)
		}
		if !bytes.Equal(on.page, same) {
			t.Fatalf("%s: page differs from hiding off in more than the data file name", name)
		}
	}
	return fully
}

func dataName(t *testing.T, page []byte) string {
	t.Helper()
	m := dataRE.FindSubmatch(page)
	if m == nil {
		t.Fatal("page has no data link")
	}
	return string(m[1])
}

// circleBounds is the first recorded sample (gap-filled ones never count,
// H-3) more than 300 m from the first position, and the last one more than
// 300 m from the last.
func circleBounds(w workout.Workout) (lo, hi int) {
	s := w.Samples
	first, last := -1, -1
	for i := range s.Lat {
		if !math.IsNaN(s.Lat[i]) && !s.Inserted(i) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	lo, hi = len(s.Lat), -1
	for i := first; i <= last; i++ {
		if !math.IsNaN(s.Lat[i]) && !s.Inserted(i) && dist(s.Lat[first], s.Lon[first], s.Lat[i], s.Lon[i]) > workout.HideRadius {
			lo = i
			break
		}
	}
	for i := last; i >= first; i-- {
		if !math.IsNaN(s.Lat[i]) && !s.Inserted(i) && dist(s.Lat[last], s.Lon[last], s.Lat[i], s.Lon[i]) > workout.HideRadius {
			hi = i
			break
		}
	}
	return lo, hi
}

// dist is the haversine distance in metres (R-6b), restated for the test.
func dist(lat1, lon1, lat2, lon2 float64) float64 {
	const r, rad = 6371008.8, math.Pi / 180
	dφ, dλ := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return 2 * r * math.Asin(math.Sqrt(min(a, 1)))
}

// TestHidingCorpus runs AC 1 to 4 over every synthetic corpus file the
// parser accepts.
func TestHidingCorpus(t *testing.T) {
	entries, err := os.ReadDir(synthetic)
	if err != nil {
		t.Fatal(err)
	}
	parser := ingest.NewParser(ingest.DefaultLimits, nil)
	var partial, indoor int
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".fit") {
			continue
		}
		orig, err := os.ReadFile(filepath.Join(synthetic, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		w, err := parser.Parse(context.Background(), orig)
		if err != nil {
			continue // rejected files never reach a public build
		}
		t.Run(e.Name(), func(t *testing.T) {
			if checkHidden(t, e.Name(), orig, parser, w) {
				t.Errorf("fully hidden")
			}
		})
		if w.Samples.Lat == nil {
			indoor++
		} else {
			partial++
		}
	}
	if partial < 10 || indoor < 1 {
		t.Errorf("corpus covers %d routes and %d indoor files", partial, indoor)
	}
}

// Generated tracks (metres east and north of 50° N, 8° E).
func gen(n int, xy func(i int) (x, y float64)) workout.Workout {
	t0 := time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)
	recs := make([]workout.RawRecord, n)
	for i := range recs {
		x, y := xy(i)
		lat, lon := math.NaN(), math.NaN()
		if !math.IsNaN(x) {
			lat, lon = 50+y/111195.08, 8+x/(111195.08*math.Cos(50*math.Pi/180))
		}
		recs[i] = workout.RawRecord{At: t0.Add(time.Duration(i) * time.Second), Lat: lat, Lon: lon,
			Altitude: 200 + float64(i%20), Distance: float64(i) * 1.4, Speed: 1.4, HeartRate: float64(90 + i%30),
			Cadence: math.NaN(), Power: math.NaN(), Temperature: math.NaN()}
	}
	return workout.Normalise(workout.Recording{FileType: "activity", Records: recs})
}

// TestHidingFullyHidden (AC 5, 6, 8): a 500 m walk and 5 km on a 400 m
// track near the start publish without positions and with the notice; an
// indoor workout keeps the indoor hint.
func TestHidingFullyHidden(t *testing.T) {
	r := 400 / (2 * math.Pi)
	walks := map[string]workout.Workout{
		"500 m walk": gen(357, func(i int) (float64, float64) { return float64(i) * 1.4, 0 }),
		"400 m track": gen(3570, func(i int) (float64, float64) {
			a := float64(i) * 1.4 / r
			return 150 + r*math.Cos(a), r * math.Sin(a)
		}),
		"a 2 km walk": gen(1430, func(i int) (float64, float64) { return 0, float64(i) * 1.4 }),
		"indoor":      gen(600, func(int) (float64, float64) { return math.NaN(), math.NaN() }),
	}
	p := fakeParser{}
	for name, w := range walks {
		p[name] = w
	}
	for name, w := range walks {
		fully := checkHidden(t, name, []byte(name), p, w)
		if want := name == "500 m walk" || name == "400 m track"; fully != want {
			t.Errorf("%s: fully hidden %v, want %v", name, fully, want)
		}
		if name == "indoor" {
			_, on := publishBoth(t, []byte(name), p)
			if !bytes.Contains(on.page, []byte(indoorHint)) || bytes.Contains(on.page, []byte(notice)) {
				t.Error("indoor with hiding on: not the indoor hint")
			}
		}
	}
}

// TestHidingOnAfterPublish (last AC, H-9): off → on removes first and
// deletes the old data file; a rebuild with hiding still on keeps the cut;
// on → off restores the full route in place.
func TestHidingOnAfterPublish(t *testing.T) {
	fx := newFixture(t, "ride-30min.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	full := fx.read(t)
	fullName := dataName(t, full.page)

	hidden := p
	hidden.HideEnds = true
	fx.steps = nil
	fx.change(t, p, hidden)
	if len(fx.steps) < 2 || fx.steps[0] != "remove index.html" || fx.steps[1] != "remove folder" {
		t.Errorf("steps %v: hiding on must remove first", fx.steps)
	}
	cut := fx.read(t)
	cutName := dataName(t, cut.page)
	if cutName == fullName || slices.Contains(cut.names, fullName) || slices.Contains(cut.names, fullName+".br") {
		t.Fatalf("old data file still on disk: %v", cut.names)
	}
	lat, _ := stream(cut.file, "latitude")
	if !slices.Contains(lat.Valid, false) || !slices.Contains(lat.Valid, true) {
		t.Fatal("ride-30min is not partly hidden")
	}

	// A later save with hiding on (in place) builds the cut again.
	titled := hidden
	titled.Title = "Renamed"
	fx.change(t, hidden, titled)
	if again := fx.read(t); !bytes.Equal(again.bin, cut.bin) {
		t.Error("rebuild with hiding on does not cut")
	}

	// Off again: in place, the full route from the original.
	fx.steps = nil
	fx.change(t, titled, p)
	if len(fx.steps) > 0 && fx.steps[0] == "remove index.html" {
		t.Errorf("hiding off removed first: %v", fx.steps)
	}
	if back := fx.read(t); !bytes.Equal(back.bin, full.bin) {
		t.Error("hiding off does not restore the full route")
	}
	noOriginalBytes(t, fx.root, fx.orig)
}
