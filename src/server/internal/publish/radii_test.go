package publish

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	pagevm "workout-explorer/internal/page"
	"workout-explorer/internal/workout"
)

// F-55 per-end radii at the public build (AC 34 to 37, 40). Synthetic files
// and generated tracks only (ADR-004).

var testRadii = []int{0, 50, 300, 2000}

// TestHidingRadii (AC 34, 36, 37): every combination of 0, 50, 300 and
// 2,000 m per end, on a corpus ride and generated tracks: the public cut
// follows each end's radius, no hidden position is in any public file, the
// summary and laps are those of the full route, and the fully hidden and
// indoor notices hold.
func TestHidingRadii(t *testing.T) {
	ride, err := os.ReadFile(filepath.Join(synthetic, "ride-30min.fit"))
	if err != nil {
		t.Fatal(err)
	}
	rw, err := ingest.NewParser(ingest.DefaultLimits, nil).Parse(context.Background(), ride)
	if err != nil {
		t.Fatal(err)
	}
	tracks := map[string]workout.Workout{
		"ride":        rw,
		"a 2 km walk": gen(1400, func(i int) (float64, float64) { return 0, float64(i) * 1.4 }),
		"indoor":      gen(300, func(int) (float64, float64) { return math.NaN(), math.NaN() }),
	}
	p := fakeParser{}
	for name, w := range tracks {
		p[name] = w
	}
	for name, w := range tracks {
		for _, a := range testRadii {
			for _, b := range testRadii {
				fully := checkHiddenAt(t, name, []byte(name), p, w, a, b)
				// The 1.96 km walk lies within 2,000 m of either end.
				want := name == "a 2 km walk" && (a == 2000 || b == 2000)
				if fully != want {
					t.Errorf("%s (%d, %d): fully hidden %v, want %v", name, a, b, fully, want)
				}
			}
		}
	}
	// Start 0, end 500: only the end is hidden.
	_, on := publishAt(t, []byte("ride"), p, 0, 500)
	lat, _ := stream(on.file, "latitude")
	if !lat.Valid[0] || lat.Valid[len(lat.Valid)-1] {
		t.Error("start 0, end 500: not only the end hidden")
	}
}

// TestHidingRadiiNotPublic (AC 40, HR-6): two radius pairs that give the same
// cut produce byte-identical public folders, so no public file holds a
// radius; and neither the page view model nor the data file metadata has a
// field that could carry one.
func TestHidingRadiiNotPublic(t *testing.T) {
	// 10 fixes at the start point, then a jump of 1 km (an unfilled gap):
	// every start radius from 50 to 950 m cuts the same samples.
	w := gen(1000, func(i int) (float64, float64) {
		if i < 10 {
			return 0, 0
		}
		return 1000 + float64(i)*1.4, 0
	})
	p := fakeParser{"jump": w}
	var folders [][]string
	var contents [][][]byte
	for _, rr := range [][2]int{{50, 300}, {950, 300}} {
		fx := newFixtureWith(t, []byte("jump"), p)
		d := draft()
		d.HideStartM, d.HideEndM = rr[0], rr[1]
		fx.change(t, d, published(d))
		fx.read(t)
		names := fx.names(t)
		var c [][]byte
		for _, n := range names {
			b, err := os.ReadFile(filepath.Join(fx.folder(), n))
			if err != nil {
				t.Fatal(err)
			}
			c = append(c, b)
			if bytes.Contains(b, []byte("950")) && n == "index.html" {
				t.Errorf("%v: index.html contains 950", rr)
			}
		}
		folders, contents = append(folders, names), append(contents, c)
	}
	if !slices.Equal(folders[0], folders[1]) || !reflect.DeepEqual(contents[0], contents[1]) {
		t.Errorf("same cut, different public folders: %v %v", folders[0], folders[1])
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(pagevm.Settings{}), reflect.TypeOf(pagevm.ViewModel{}), reflect.TypeOf(datafile.Meta{})} {
		for i := 0; i < typ.NumField(); i++ {
			if n := strings.ToLower(typ.Field(i).Name); n != "routehidden" && strings.Contains(n, "hid") || strings.Contains(n, "radius") || strings.Contains(n, "radii") {
				t.Errorf("%s.%s may carry a radius", typ.Name(), typ.Field(i).Name)
			}
		}
	}
}
