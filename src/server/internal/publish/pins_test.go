package publish

import (
	"bytes"
	"context"
	"encoding/json"
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
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/workout"
)

// F-56 PN-4, PN-6, PN-8 (PIN-7, SH-4, SH-10): no pin that is hidden, trimmed
// or dimmed reaches any public file. Synthetic files only (ADR-004).

var pinsRE = regexp.MustCompile(`/w/` + publicID + `/(pins\.[0-9a-f]{16}\.json)`)

const (
	dimTitle = "DIMMED-TITLE-5071"
	dimNote  = "DIMMED-NOTE-43219"
)

func pinFixture(t *testing.T, w workout.Workout) *fixture {
	t.Helper()
	return newFixtureWith(t, []byte("trk"), fakeParser{"trk": w})
}

func point(w workout.Workout, i int, title string) lifecycle.Pin {
	return lifecycle.Pin{Type: "danger", Title: title, StartAt: *instant(w, i)}
}

func section(w workout.Workout, i, j int, title string) lifecycle.Pin {
	return lifecycle.Pin{Type: "difficult_section", Title: title, StartAt: *instant(w, i), EndAt: instant(w, j)}
}

// publicBytes is every byte under public/, brotli files decompressed.
func publicBytes(t *testing.T, fx *fixture) []byte {
	t.Helper()
	var all []byte
	_ = filepath.WalkDir(fx.root, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, _ := os.ReadFile(p)
		all = append(all, b...)
		if strings.HasSuffix(p, ".br") {
			d, _ := io.ReadAll(brotli.NewReader(bytes.NewReader(b)))
			all = append(all, d...)
		}
		all = append(all, '\n')
		return nil
	})
	return all
}

// pinsFile returns the pins file the page links, or nil.
func pinsFile(t *testing.T, fx *fixture) (name string, body []byte) {
	t.Helper()
	page, err := os.ReadFile(filepath.Join(fx.folder(), indexName))
	if err != nil {
		t.Fatal(err)
	}
	m := pinsRE.FindSubmatch(page)
	if m == nil {
		return "", nil
	}
	body, err = os.ReadFile(filepath.Join(fx.folder(), string(m[1])))
	if err != nil {
		t.Fatalf("page links a missing pins file: %v", err)
	}
	return string(m[1]), body
}

func pinFiles(fx *fixture, t *testing.T) []string {
	var out []string
	for _, n := range fx.names(t) {
		if strings.HasPrefix(n, "pins.") {
			out = append(out, n)
		}
	}
	return out
}

// publicLat is the decoded latitude column of the page's data file.
func publicLat(t *testing.T, fx *fixture) datafile.Stream {
	t.Helper()
	name, _ := fx.complete(t)
	bin, _ := os.ReadFile(filepath.Join(fx.folder(), name))
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	lat, _ := stream(f, "latitude")
	return lat
}

// TestPinPredicatesAgreeWithTheDataFile (AC 35, PN-4): over radii per end,
// with and without a trim, a point is public exactly where the public data
// file has a position, kept exactly inside the trim window, and a section is
// public only when both ends are.
func TestPinPredicatesAgreeWithTheDataFile(t *testing.T) {
	for _, driveFirst := range []bool{false, true} {
		for _, rr := range [][2]int{{0, 0}, {50, 0}, {0, 50}, {300, 300}, {2000, 0}, {0, 2000}, {2000, 2000}} {
			for _, trim := range []bool{false, true} {
				w := trimTrack(driveFirst)
				d := draft()
				d.HideStartM, d.HideEndM = rr[0], rr[1]
				lo, hi := 0, len(w.Samples.Offset)-1
				if trim {
					lo, hi = 100, 800
					d.TrimStartAt, d.TrimEndAt = instant(w, lo), instant(w, hi)
				}
				fx := pinFixture(t, w)
				fx.change(t, d, published(d))
				lat := publicLat(t, fx)
				at := func(i int) bool { return i >= lo && i <= hi && lat.Valid[i-lo] } // oracle: the file

				var pins []lifecycle.Pin
				for i := 0; i < len(w.Samples.Offset); i += 3 {
					pins = append(pins, point(w, i, ""))
				}
				pins = append(pins, point(w, len(w.Samples.Offset)-1, ""))
				checks, err := fx.a.CheckPins(context.Background(), d, pins)
				if err != nil {
					t.Fatal(err)
				}
				for k, c := range checks {
					i := int(c.Resolved.StartAt.Sub(w.StartTime) / time.Second)
					if c.Outside || c.Kept != (i >= lo && i <= hi) || c.Public != at(i) {
						t.Fatalf("drive %v radii %v trim %v sample %d: %+v, file says public %v", driveFirst, rr, trim, k, c, at(i))
					}
				}
				// Sections: both ends must be public; one end hidden or trimmed is not.
				for i := lo; i+40 <= hi; i += 37 {
					cs, _ := fx.a.CheckPins(context.Background(), d, []lifecycle.Pin{section(w, i, i+40, "")})
					want := at(i) && at(i+40)
					if cs[0].Public != want || cs[0].Kept != true {
						t.Fatalf("section %d: %+v want public %v", i, cs[0], want)
					}
				}
				// An end in the trimmed part: not kept, never clipped.
				if trim {
					cs, _ := fx.a.CheckPins(context.Background(), d, []lifecycle.Pin{section(w, lo-10, lo+50, ""), section(w, hi-50, hi+10, "")})
					if cs[0].Kept || cs[0].Public || cs[1].Kept || cs[1].Public {
						t.Fatalf("a clipped section is kept: %+v", cs)
					}
				}
			}
		}
	}
}

// TestPinFirstVisibleFixAndDropout: a section may start at the first visible
// fix; a sample in a GPS dropout is never public; no visible route, no pin.
func TestPinFirstVisibleFixAndDropout(t *testing.T) {
	w := trimTrack(false)
	lat, lon := slices.Clone(w.Samples.Lat), slices.Clone(w.Samples.Lon)
	for i := 400; i < 420; i++ {
		lat[i], lon[i] = math.NaN(), math.NaN()
	}
	w.Samples.Lat, w.Samples.Lon = lat, lon
	d := draft()
	d.HideStartM = 300
	fx := pinFixture(t, w)
	first := slices.IndexFunc(publicLatFlags(t, fx, d), func(v bool) bool { return v })
	if first <= 0 {
		t.Fatalf("first visible fix %d", first)
	}
	cs, err := fx.a.CheckPins(context.Background(), d, []lifecycle.Pin{
		section(w, first, first+10, ""), section(w, first-1, first+10, ""), point(w, 410, ""), section(w, 300, 410, ""), point(w, 500, ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range []bool{true, false, false, false, true} {
		if cs[k].Public != want {
			t.Errorf("pin %d: %+v want public %v", k, cs[k], want)
		}
	}
	// A workout without a visible route: radii that cover everything.
	d.HideStartM, d.HideEndM = 2000, 2000
	w2 := trimTrack(false)
	w2.Samples.Lat, w2.Samples.Lon = nil, nil // indoor
	fx2 := pinFixture(t, w2)
	cs, _ = fx2.a.CheckPins(context.Background(), d, []lifecycle.Pin{point(w2, 10, "")})
	if cs[0].Public || !cs[0].Kept {
		t.Errorf("indoor: %+v", cs[0])
	}
	// Outside the recording.
	cs, _ = fx.a.CheckPins(context.Background(), d, []lifecycle.Pin{{Type: "note", StartAt: w.StartTime.Add(-time.Second)}, {Type: "note", StartAt: *instant(w, 899), EndAt: ptrTime(w.StartTime.Add(time.Hour))}})
	if !cs[0].Outside || !cs[1].Outside {
		t.Errorf("%+v", cs)
	}
}

func publicLatFlags(t *testing.T, fx *fixture, d lifecycle.Record) []bool {
	t.Helper()
	fx.change(t, draft(), published(d))
	lat := publicLat(t, fx)
	return lat.Valid
}

// Nearest sample, ties to the earlier (PN-3).
func TestPinResolve(t *testing.T) {
	g := pinGrid{off: []int32{0, 1, 2, 10, 12}, base: 1000}
	for _, tc := range []struct {
		at   int64
		want int
		ok   bool
	}{{1000, 0, true}, {1002, 2, true}, {1006, 2, true}, {1007, 3, true}, {1011, 3, true}, {1012, 4, true}, {999, 0, false}, {1013, 0, false}} {
		got, ok := g.resolve(time.Unix(tc.at, 0))
		if ok != tc.ok || ok && got != tc.want {
			t.Errorf("%d: %d %v", tc.at, got, ok)
		}
	}
}

// TestPinsFile (PN-6, PN-7): the file holds only the keys of its shape and
// indices that have a position in the data file of the same build; the same
// pins give the same name; a change removes the old file; no pins, no file.
func TestPinsFile(t *testing.T) {
	w := trimTrack(false)
	d := draft()
	d.HideStartM, d.HideEndM = 300, 0
	d.TrimEndAt = instant(w, 599)
	fx := pinFixture(t, w)
	fx.change(t, draft(), published(draft()))
	if len(pinFiles(fx, t)) != 0 {
		t.Fatal("a pins file without pins")
	}
	if page, _ := os.ReadFile(filepath.Join(fx.folder(), indexName)); bytes.Contains(page, []byte("pins-slot")) {
		t.Fatal("a pins slot without pins")
	}

	d.Pins = []lifecycle.Pin{point(w, 300, "Loose gravel"), section(w, 350, 400, "Busy"), point(w, 10, dimTitle)}
	d.Pins[1].Note = "No <b>shoulder</b>"
	fx.change(t, draft(), published(d))
	name, body := pinsFile(t, fx)
	if name == "" {
		t.Fatal("no pins file")
	}
	var got []map[string]any
	if err := json.Unmarshal(body, &got); err != nil || len(got) != 2 {
		t.Fatalf("%s %v", body, err)
	}
	lat := publicLat(t, fx)
	for _, p := range got {
		for k := range p {
			if !slices.Contains([]string{"type", "title", "note", "from", "to"}, k) {
				t.Errorf("key %q in the pins file", k)
			}
		}
		for _, k := range []string{"from", "to"} {
			if v, ok := p[k]; ok && !lat.Valid[int(v.(float64))] {
				t.Errorf("%s %v has no position", k, v)
			}
		}
	}
	if got[0]["from"].(float64) != 300 || got[1]["from"].(float64) != 350 || got[1]["to"].(float64) != 400 || got[0]["to"] != nil {
		t.Errorf("%s", body)
	}
	page, _ := os.ReadFile(filepath.Join(fx.folder(), indexName))
	if !bytes.Contains(page, []byte(`data-url="/w/`+publicID+`/`+name+`" data-count="2"`)) {
		t.Errorf("the page does not carry URL and Count of public pins only")
	}
	if all := publicBytes(t, fx); bytes.Contains(all, []byte(dimTitle)) {
		t.Error("a dimmed pin's title is public")
	}
	// Same pins, same name.
	fx.change(t, published(d), published(d))
	if n2, _ := pinsFile(t, fx); n2 != name {
		t.Error("the name changed")
	}
	// A pin change: the old file is gone.
	d2 := d
	d2.Pins = slices.Clone(d.Pins)
	d2.Pins[0].Title = "Edited"
	fx.change(t, published(d), published(d2))
	n3, _ := pinsFile(t, fx)
	if n3 == name || len(pinFiles(fx, t)) != 1 {
		t.Errorf("%s -> %s, files %v", name, n3, pinFiles(fx, t))
	}
	// No pins left: no file, no slot.
	d3 := d2
	d3.Pins = nil
	fx.change(t, published(d2), published(d3))
	if len(pinFiles(fx, t)) != 0 {
		t.Error("a pins file without pins")
	}
}

// TestPinsDimmedNeverPublic (AC 35, 40): a raised radius keeps the pins in the
// row and takes exactly the no-longer-public ones out of every public file; a
// lowered radius brings them back; a change to dimmed pins only rebuilds nothing.
func TestPinsDimmedNeverPublic(t *testing.T) {
	w := trimTrack(false)
	base := draft()
	base.Pins = []lifecycle.Pin{point(w, 350, "shown"), point(w, 20, dimTitle), section(w, 30, 500, dimTitle+"-s")}
	base.Pins[1].Note, base.Pins[2].Note = dimNote, dimNote
	fx := pinFixture(t, w)

	// Radius 300 from the start: the first pin of the row is dimmed from the first build.
	d := base
	d.HideStartM = 300
	fx.change(t, draft(), published(d))
	_, body := pinsFile(t, fx)
	if all := publicBytes(t, fx); bytes.Contains(all, []byte(dimTitle)) || bytes.Contains(all, []byte(dimNote)) {
		t.Fatal("a dimmed pin is public")
	}
	var got []pubPin
	if err := json.Unmarshal(body, &got); err != nil || len(got) != 1 || got[0].Title != "shown" {
		t.Fatalf("%s %v", body, err)
	}
	page, _ := os.ReadFile(filepath.Join(fx.folder(), indexName))
	if !bytes.Contains(page, []byte(`data-count="1"`)) {
		t.Error("Count counts a dimmed pin")
	}

	// A change to dimmed pins alone: no file-system step at all (PN-8).
	var steps []string
	fx.a.step = func(n string) error { steps = append(steps, n); return nil }
	d2 := d
	d2.Pins = slices.Clone(d.Pins)
	d2.Pins[1].Title, d2.Pins[1].Note = dimTitle+"-edited", dimNote+"-edited"
	d2.Pins[1].Type = "note"
	namesBefore := fx.names(t)
	fx.change(t, published(d), published(d2))
	if len(steps) != 0 || !slices.Equal(namesBefore, fx.names(t)) {
		t.Errorf("a dimmed-only change touched public/: %v", steps)
	}
	d3 := d2
	d3.Pins = d2.Pins[:1] // a dimmed pin deleted
	d3.Pins = []lifecycle.Pin{d2.Pins[0], d2.Pins[2]}
	fx.change(t, published(d2), published(d3))
	if len(steps) != 0 {
		t.Errorf("deleting a dimmed pin touched public/: %v", steps)
	}
	// A change to a public pin is remove-first.
	steps = nil
	d4 := d3
	d4.Pins = slices.Clone(d3.Pins)
	d4.Pins[0].Title = "shown, edited"
	fx.change(t, published(d3), published(d4))
	if len(steps) == 0 || steps[0] != "remove "+indexName {
		t.Errorf("steps %v", steps)
	}
	fx.a.step = nil

	// Raising the radius further: the pin at 350 is hidden at 2000; only it leaves.
	d5 := d4
	d5.HideStartM = 2000
	fx.change(t, published(d4), published(d5))
	if n, _ := pinsFile(t, fx); n != "" || len(pinFiles(fx, t)) != 0 {
		t.Errorf("a pins file with every pin dimmed: %v", pinFiles(fx, t))
	}
	if all := publicBytes(t, fx); bytes.Contains(all, []byte("shown")) || bytes.Contains(all, []byte(dimTitle)) {
		t.Error("a pin of a hidden place is public")
	}
	page, _ = os.ReadFile(filepath.Join(fx.folder(), indexName))
	if bytes.Contains(page, []byte("pins-slot")) {
		t.Error("a slot for dimmed pins")
	}
	// Lowering again brings the pin back, in place.
	fx.change(t, published(d5), published(d4))
	if _, body := pinsFile(t, fx); !bytes.Contains(body, []byte("shown, edited")) {
		t.Errorf("not republished: %s", body)
	}
}

// TestPinsTrimRemovesFromTheBuild: a pin in the trimmed part is never public,
// also when it is still in the row (defence in depth: the row is cleaned by PN-5).
func TestPinsTrimmedPartNeverPublic(t *testing.T) {
	w := trimTrack(false)
	d := draft()
	d.TrimEndAt = instant(w, 599)
	d.Pins = []lifecycle.Pin{point(w, 100, "in"), point(w, 700, dimTitle), section(w, 590, 700, dimTitle)}
	fx := pinFixture(t, w)
	fx.change(t, draft(), published(d))
	_, body := pinsFile(t, fx)
	if bytes.Contains(publicBytes(t, fx), []byte(dimTitle)) || !bytes.Contains(body, []byte(`"in"`)) {
		t.Errorf("%s", body)
	}
}

// Mutant guard: pins are not on P-7's allow-list.
func TestHidesMoreForPins(t *testing.T) {
	a, b := published(draft()), published(draft())
	b.Pins = []lifecycle.Pin{{Type: "note", StartAt: time.Unix(1, 0)}}
	if !HidesMore(a, b) || !HidesMore(b, a) {
		t.Error("a pin change is written in place")
	}
}

// Deterministic file: the same pins, the same bytes.
func TestPinsJSONDeterministic(t *testing.T) {
	ps := []pubPin{{Type: "note", Title: "a", From: 1}, {Type: "danger", Title: "b", From: 2, To: 9}}
	if !bytes.Equal(pinsJSON(ps), pinsJSON(slices.Clone(ps))) || pinsJSON(nil) != nil {
		t.Error("not deterministic")
	}
	if string(pinsJSON(ps)) != `[{"type":"note","title":"a","note":"","from":1},{"type":"danger","title":"b","note":"","from":2,"to":9}]` {
		t.Errorf("%s", pinsJSON(ps))
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// AC 40, UP-7: removing the folder (delete, expiry) leaves no pins file.
func TestPinsFileRemovedWithFolder(t *testing.T) {
	w := trimTrack(false)
	d := draft()
	d.Pins = []lifecycle.Pin{point(w, 300, dimTitle)}
	fx := pinFixture(t, w)
	fx.change(t, draft(), published(d))
	if len(pinFiles(fx, t)) != 1 {
		t.Fatal("no pins file")
	}
	if _, err := fx.a.Remove(publicID); err != nil || len(fx.names(t)) != 0 || bytes.Contains(publicBytes(t, fx), []byte(dimTitle)) {
		t.Fatalf("%v %v", err, fx.names(t))
	}
}

// An unchanged row (regenerate, a retried publish) is rebuilt, never skipped.
func TestPinsSameRowStillRebuilds(t *testing.T) {
	w := trimTrack(false)
	d := published(draft())
	d.Pins = []lifecycle.Pin{point(w, 300, "x")}
	fx := pinFixture(t, w)
	var steps []string
	fx.a.step = func(n string) error { steps = append(steps, n); return nil }
	fx.change(t, d, d)
	if len(steps) == 0 {
		t.Error("an identical row was skipped")
	}
}
