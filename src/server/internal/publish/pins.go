package publish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"time"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/workout"
)

// Map pins (F-56 PN-4, PN-6, ADR-057). The two predicates "kept" and
// "public" live here, once: the owner PATCH check, the trim removal and the
// public build all read them from a pinGrid, which is made from the output
// of trim then hiding, never from the radii or from draft.bin.

// pinGrid is what the pin predicates read: the sample instants of the whole
// normalised workout, the trim window [a, b] in them, and the positions of
// the workout after trim and hiding (index i of the whole workout is index
// i-a of that output, and of the data file).
type pinGrid struct {
	off      []int32
	base     int64
	a, b     int
	lat, lon []float64 // after trim and hiding; nil without positions
}

// newPinGrid reads the public transform's output pw of the trimmed workout
// that starts at sample a of w.
func newPinGrid(off []int32, base int64, a int, pw workout.Workout) pinGrid {
	n := len(pw.Samples.Offset)
	return pinGrid{off: off, base: base, a: a, b: a + n - 1, lat: pw.Samples.Lat, lon: pw.Samples.Lon}
}

// resolve is PN-3: the nearest sample of the whole workout, ties to the
// earlier; ok is false for an instant before the first or after the last.
func (g pinGrid) resolve(t time.Time) (int, bool) {
	n := len(g.off)
	if n == 0 {
		return 0, false
	}
	x := t.Unix() - g.base
	if x < int64(g.off[0]) || x > int64(g.off[n-1]) {
		return 0, false
	}
	i := sort.Search(n, func(i int) bool { return int64(g.off[i]) >= x })
	if i > 0 && x-int64(g.off[i-1]) <= int64(g.off[i])-x {
		i--
	}
	return i, true
}

func (g pinGrid) instant(i int) time.Time {
	return time.Unix(g.base+int64(g.off[i]), 0).UTC()
}

func (g pinGrid) kept(i int) bool { return i >= g.a && i <= g.b }

// positioned: sample i has a position in the public output.
func (g pinGrid) positioned(i int) bool {
	j := i - g.a
	return g.kept(i) && j < len(g.lat) && j < len(g.lon) && !math.IsNaN(g.lat[j]) && !math.IsNaN(g.lon[j])
}

// judge is PN-4 for one pin.
func (g pinGrid) judge(p lifecycle.Pin) (c lifecycle.PinCheck, from, to int) {
	c.Resolved = p
	from, to = -1, -1
	si, ok := g.resolve(p.StartAt)
	if !ok {
		c.Outside = true
		return
	}
	c.Resolved.StartAt = g.instant(si)
	from = si
	c.Kept, c.Public = g.kept(si), g.positioned(si)
	if p.EndAt != nil {
		ei, ok := g.resolve(*p.EndAt)
		if !ok {
			c.Outside = true
			return
		}
		if ei <= si {
			c.Merged = true
			return
		}
		e := g.instant(ei)
		c.Resolved.EndAt = &e
		to = ei
		c.Kept = c.Kept && g.kept(ei)
		c.Public = c.Public && g.positioned(ei)
	}
	return
}

// pinGridOf parses the original and applies trim and the public transform.
func (a *Area) pinGridOf(ctx context.Context, r lifecycle.Record) (pinGrid, error) {
	w, err := a.parseOriginal(ctx, r)
	if err != nil {
		return pinGrid{}, err
	}
	return a.pinGridFrom(w, r)
}

// pinGridFrom applies trim and the public transform to the parsed w.
func (a *Area) pinGridFrom(w workout.Workout, r lifecycle.Record) (pinGrid, error) {
	wt, first, err := trimmed(w, r)
	if err != nil {
		return pinGrid{}, err
	}
	pw, _ := a.transform(wt, r)
	return newPinGrid(w.Samples.Offset, w.StartTime.Unix(), first, pw), nil
}

// pubPin is one entry of pins.{hash}.json (PN-6): type, texts and sample
// indices of the data file of the same build; no coordinate, no instant.
type pubPin struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Note  string `json:"note"`
	From  int    `json:"from"`
	To    int    `json:"to,omitempty"` // sections only; to > from > = 0, so never 0
}

// publicPins is the public projection of pins (PN-6): only public pins,
// sorted by from, then point before section, then to, then stored order.
func publicPins(g pinGrid, pins []lifecycle.Pin) []pubPin {
	var out []pubPin
	for _, p := range pins {
		c, from, to := g.judge(p)
		if !c.Public || c.Outside || c.Merged {
			continue
		}
		q := pubPin{Type: p.Type, Title: p.Title, Note: p.Note, From: from - g.a}
		if to >= 0 {
			q.To = to - g.a
		}
		out = append(out, q)
	}
	slices.SortStableFunc(out, func(x, y pubPin) int {
		if x.From != y.From {
			return x.From - y.From
		}
		return x.To - y.To // a point (0) before a section
	})
	return out
}

// pinsJSON is the file content, nil for no public pin.
func pinsJSON(ps []pubPin) []byte {
	if len(ps) == 0 {
		return nil
	}
	b, err := json.Marshal(ps) // strings and integers always marshal
	if err != nil {
		panic(err)
	}
	return b
}

// pinsFileName is the content-hashed name (P-5).
func pinsFileName(js []byte) string {
	sum := sha256.Sum256(js)
	return "pins." + hex.EncodeToString(sum[:8]) + ".json"
}

// samePublicPins is ADR-057 item 5 / PN-8: a change that touches only pins
// and leaves the public projection as it was causes no rebuild, so a dimmed
// pin never shows, not even as a page swap. Any doubt (a failure, no page on
// disk) answers false, which takes the full remove-first path.
func (a *Area) samePublicPins(ctx context.Context, before, after lifecycle.Record) bool {
	b2 := before
	b2.Pins = after.Pins
	// Identical rows (regenerate, a retried publish) rebuild in place: only a pins difference qualifies.
	if reflect.DeepEqual(before.Pins, after.Pins) || !sameRecord(b2, after) || after.Visibility != lifecycle.Unlisted {
		return false
	}
	if info, err := os.Lstat(filepath.Join(a.root, after.PublicID, indexName)); err != nil || !info.Mode().IsRegular() {
		return false
	}
	g, err := a.pinGridOf(ctx, after)
	if err != nil {
		return false
	}
	return slices.Equal(pinsJSON(publicPins(g, before.Pins)), pinsJSON(publicPins(g, after.Pins)))
}
