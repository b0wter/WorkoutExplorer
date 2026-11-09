package publish

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
)

// lapsSection returns the laps module of a rendered page, or "" without one.
func lapsSection(html []byte) string {
	s := string(html)
	i := strings.Index(s, `<section class="laps">`)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], "</section>")
	return s[i : i+j]
}

// TestLapsMarkup (F-18 AC 6, L-8): the published page has the laps table in
// its markup; a file with distance but no laps has the module with "No laps
// recorded." and no table.
func TestLapsMarkup(t *testing.T) {
	for _, c := range []struct {
		file, want, not string
	}{
		{"ride-laps-no-totals.fit", "<td>3</td>", "No laps recorded."},
		{"ride-no-laps.fit", "No laps recorded.", "<table"},
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
