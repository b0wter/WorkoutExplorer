package publish

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/page"
)

// lapsSection returns the laps module of a rendered page, or "" without one.
func lapsSection(html []byte) string {
	s := string(html)
	i := strings.Index(s, `<section class="laps"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], "</section>")
	return s[i : i+j]
}

// TestLapsMarkup (F-18 AC 6, L-8): the published page has the laps table in
// its markup; a file with distance but no laps has the module hidden,
// headed "Splits", with no table (F-46 LS-3).
func TestLapsMarkup(t *testing.T) {
	for _, c := range []struct {
		file, want, not string
	}{
		{"ride-laps-no-totals.fit", "<td>3</td>", "Splits"},
		{"ride-no-laps.fit", `<h2 data-t="headSplits">Splits</h2>`, "<table"},
	} {
		t.Run(c.file, func(t *testing.T) {
			fx := newFixture(t, c.file)
			d := draft()
			fx.change(t, d, published(d))
			_, html := fx.complete(t)
			laps := lapsSection(html)
			if laps == "" {
				t.Fatal("no laps module")
			}
			if hidden := strings.Contains(laps, `class="laps" hidden`); hidden != (c.file == "ride-no-laps.fit") {
				t.Errorf("hidden attribute = %v:\n%s", hidden, laps)
			}
			if !strings.Contains(laps, c.want) || strings.Contains(laps, c.not) {
				t.Errorf("laps module: want %q, not %q:\n%s", c.want, c.not, laps)
			}
		})
	}
}

// TestNoLapsModule (L-8): without laps and without an advancing distance
// the page has no laps module.
func TestNoLapsModule(t *testing.T) {
	tmpl, err := LoadTemplate(webDir, false)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile("../../testdata/datafile/synthetic/ride-no-laps.bin")
	if err != nil {
		t.Fatal(err)
	}
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*datafile.Stream){
		"no distance": nil,
		"flat distance": func(s *datafile.Stream) {
			for i := range s.Ints {
				s.Ints[i] = 5
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := f
			g.Streams = nil
			for _, s := range f.Streams {
				if s.Descriptor.Name == "distance" {
					if change == nil {
						continue
					}
					s.Ints = append([]int64(nil), s.Ints...)
					change(&s)
				}
				g.Streams = append(g.Streams, s)
			}
			vm := page.Build(g, page.Settings{ExpiresAt: time.Unix(0, 0), PublicID: "x", DataFile: "data.0.bin"})
			if vm.Laps != nil {
				t.Fatalf("laps group %+v", vm.Laps)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, vm); err != nil {
				t.Fatal(err)
			}
			if lapsSection(buf.Bytes()) != "" {
				t.Error("laps module without laps and distance")
			}
		})
	}
}

// TestLegBlocks (F-64 LL-2, AC 1, no JavaScript): the published triathlon has
// one block per leg with its own heading and table, numbered from 1, and no
// separator rows; a two-ride file has a single plain table.
func TestLegBlocks(t *testing.T) {
	for _, c := range []struct {
		file string
		want []string
	}{
		{"triathlon.fit", []string{"Swim", "T1", "Ride", "T2", "Run"}},
		{"chained-ride-ride.fit", nil},
	} {
		t.Run(c.file, func(t *testing.T) {
			fx := newFixture(t, c.file)
			d := draft()
			fx.change(t, d, published(d))
			_, html := fx.complete(t)
			laps := lapsSection(html)
			if strings.Contains(laps, "laps-section") {
				t.Errorf("separator rows remain:\n%s", laps)
			}
			if n := strings.Count(laps, `<div class="laps-leg"`); n != len(c.want) {
				t.Fatalf("%d leg blocks, want %d:\n%s", n, len(c.want), laps)
			}
			if n := strings.Count(laps, "<table>"); n != max(len(c.want), 1) {
				t.Errorf("%d tables, want one per leg", n)
			}
			for _, name := range c.want {
				if !strings.Contains(laps, "<h3>"+name+"</h3>") {
					t.Errorf("no heading %q:\n%s", name, laps)
				}
			}
			if len(c.want) > 0 {
				if n := strings.Count(laps, "<tr><td>1</td>"); n != len(c.want) {
					t.Errorf("%d tables start at lap 1, want %d", n, len(c.want))
				}
				// Speed or pace follows the leg (OQ-300): the swim table shows pace.
				swim := laps[:strings.Index(laps, "<h3>T1</h3>")]
				if !strings.Contains(swim, `data-sport="swimming"`) || !strings.Contains(swim, "Pace (/100 m)") {
					t.Errorf("swim block lacks its sport or pace column:\n%s", swim)
				}
			}
		})
	}
}

// TestSectionsHidden (F-42 AC 7, SH-4, QA-42-r1-1): the triathlon published
// with start and end hiding keeps its sessions (the leg lines) and leg blocks, and a leg start inside a hidden part has no public position, so the
// browser draws no leg marker there (routeMarkers reads public positions only).
// The swim and T1 are folded onto the start, so T1 and the ride begin inside
// the hidden start.
func TestSectionsHidden(t *testing.T) {
	orig, err := os.ReadFile(filepath.Join(synthetic, "triathlon.fit"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := ingest.NewParser(ingest.DefaultLimits, nil).Parse(context.Background(), orig)
	if err != nil {
		t.Fatal(err)
	}
	lat, lon := slices.Clone(w.Samples.Lat), slices.Clone(w.Samples.Lon)
	for i := 1; i <= 700; i++ { // 1 Hz: swim 0..599, T1 600..659
		lat[i], lon[i] = lat[0], lon[0]
	}
	w.Samples.Lat, w.Samples.Lon = lat, lon
	fx := newFixtureWith(t, orig, fakeParser{string(orig): w})
	d := draft()
	d.HideStartM, d.HideEndM = 300, 300
	fx.change(t, d, published(d))
	o := fx.read(t)
	if n := strings.Count(lapsSection(o.page), `<div class="laps-leg"`); n != 5 {
		t.Errorf("%d leg blocks with hiding on, want 5", n)
	}
	if n := len(o.file.Meta.Sessions); n != 5 {
		t.Fatalf("%d sessions with hiding on, want 5", n)
	}
	pos, ok := stream(o.file, "latitude")
	if !ok {
		t.Fatal("route fully hidden; the case needs a partly hidden route")
	}
	var hidden []bool
	for _, s := range o.file.Meta.Sessions[1:] {
		hidden = append(hidden, !pos.Valid[s.StartIndex])
	}
	if want := []bool{true, true, false, false}; !slices.Equal(hidden, want) {
		t.Errorf("leg starts hidden %v, want %v (T1, Ride, T2, Run)", hidden, want)
	}
}
