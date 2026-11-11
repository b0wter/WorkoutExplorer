package places

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"workout-explorer/internal/workout"
)

func TestEmbeddedTable(t *testing.T) {
	ix := Default()
	if ix == nil || len(ix.towns) < 20000 {
		t.Fatalf("table not loaded or too small")
	}
	for _, tw := range ix.towns {
		if !utf8.ValidString(tw.name) || utf8.RuneCountInString(tw.name) > 60 || strings.TrimSpace(tw.name) == "" {
			t.Fatalf("bad name %q", tw.name)
		}
		for _, r := range tw.name {
			if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
				t.Fatalf("control character in %q", tw.name)
			}
		}
	}
	// Longest "<Evening> triathlon near <town>" must stay within UP-4's 100 points.
	if utf8.RuneCountInString("Afternoon multisport near ")+60 > 100 {
		t.Fatal("suggestion could exceed 100 code points")
	}
	if n, ok := ix.Nearest(50.9375, 6.9603); !ok || n != "Köln" { // district rows are dropped (OQ-162)
		t.Fatalf("Köln: got %q %v", n, ok)
	}
	if n, ok := ix.Nearest(25.2, 55.27); !ok || strings.Contains(n, "Jumayr") {
		t.Fatalf("Dubai district named: %q", n)
	}
}

func TestNearestRadius(t *testing.T) {
	ix, err := Parse("A\t10\t10\nB\t10\t10\n")
	if err != nil {
		t.Fatal(err)
	}
	// A point exactly Radius north of the town, found by bisection on the haversine.
	lo, hi := 10.0, 11.0
	for range 60 {
		mid := (lo + hi) / 2
		if workout.Haversine(mid, 10, 10, 10) <= Radius {
			lo = mid
		} else {
			hi = mid
		}
	}
	for _, c := range []struct {
		lat float64
		ok  bool
	}{{10, true}, {lo, true}, {hi + 1e-5, false}} {
		n, ok := ix.Nearest(c.lat, 10)
		if ok != c.ok || ok && n != "A" { // tie: earlier row
			t.Errorf("lat %v: %q %v", c.lat, n, ok)
		}
	}
	if _, ok := (*Index)(nil).Nearest(0, 0); ok {
		t.Error("nil index found a town")
	}
}

func TestParseRejects(t *testing.T) {
	for _, s := range []string{"A\t1\n", "A\tx\t1\n", "A\t91\t1\n"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}
