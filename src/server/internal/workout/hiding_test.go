package workout

import (
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// F-16 start and end hiding (SH-4, design H-1 to H-5). Synthetic tracks
// only (ADR-004).

const (
	lat0 = 50.0
	lon0 = 8.0
	mLat = 111195.08 // metres per degree of latitude at earthRadius
)

// pt returns the position x metres east and y metres north of (lat0, lon0).
func pt(x, y float64) (lat, lon float64) {
	return lat0 + y/mLat, lon0 + x/(mLat*math.Cos(lat0*math.Pi/180))
}

// track builds records at the given offsets with positions from xy (NaN x:
// no fix), plus heart rate, speed, altitude and distance on every record.
func track(offsets []int, xy func(i int) (x, y float64)) []RawRecord {
	return recs(offsets, func(i int, r *RawRecord) {
		if x, y := xy(i); !math.IsNaN(x) {
			r.Lat, r.Lon = pt(x, y)
		}
		r.HeartRate = float64(100 + i%50)
		r.Speed = 3 + float64(i%7)/10
		r.Altitude = 200 + float64(i%30)
		r.Distance = float64(i) * 3
	})
}

func normalised(records []RawRecord) Workout {
	return Normalise(Recording{FileType: "activity", Records: records})
}

// expectCut is the oracle, from the raw records rather than the package's
// own bookkeeping: the recorded fixes are the samples at a raw record's
// offset that have a position in w. It returns, per sample, whether the
// position stays public, and whether the route is fully hidden.
func expectCut(w Workout, records []RawRecord) (keep []bool, fully bool) {
	s := w.Samples
	keep = make([]bool, len(s.Offset))
	if s.Lat == nil {
		return keep, false
	}
	raw := map[int32]bool{}
	for _, r := range records {
		raw[int32(r.At.Sub(w.StartTime).Seconds())] = true
	}
	var fixes []int
	for i, off := range s.Offset {
		if raw[off] && !math.IsNaN(s.Lat[i]) {
			fixes = append(fixes, i)
		}
	}
	f, l := fixes[0], fixes[len(fixes)-1]
	from, to := -1, -1
	for _, i := range fixes {
		if haversine(s.Lat[f], s.Lon[f], s.Lat[i], s.Lon[i]) > HideRadius {
			from = i
			break
		}
	}
	for _, i := range slices.Backward(fixes) {
		if haversine(s.Lat[l], s.Lon[l], s.Lat[i], s.Lon[i]) > HideRadius {
			to = i
			break
		}
	}
	if from < 0 || to < 0 || from > to {
		return keep, true
	}
	for i := from; i <= to; i++ {
		keep[i] = !math.IsNaN(s.Lat[i])
	}
	return keep, false
}

// checkCut checks HideEnds on w against the oracle, and that nothing but
// the position streams changed (H-1, AC 2, AC 3) and w itself is untouched.
func checkCut(t *testing.T, name string, records []RawRecord) (Workout, Workout, bool) {
	t.Helper()
	w := normalised(records)
	before := normalised(records)
	got, fully := HideEnds(w)
	keep, wantFully := expectCut(w, records)
	if !reflect.DeepEqual(sameNaN(w), sameNaN(before)) {
		t.Fatalf("%s: input modified", name)
	}
	if fully != wantFully {
		t.Fatalf("%s: fully hidden %v, want %v", name, fully, wantFully)
	}
	s, g := w.Samples, got.Samples
	switch {
	case s.Lat == nil:
		if g.Lat != nil || g.Lon != nil {
			t.Fatalf("%s: positions appeared", name)
		}
	case fully:
		if g.Lat != nil || g.Lon != nil {
			t.Fatalf("%s: fully hidden route keeps position streams", name)
		}
	default:
		if len(g.Lat) != len(s.Offset) || len(g.Lon) != len(s.Offset) {
			t.Fatalf("%s: position stream length", name)
		}
		for i := range keep {
			visible := !math.IsNaN(g.Lat[i])
			if visible != !math.IsNaN(g.Lon[i]) {
				t.Fatalf("%s: half position at %d", name, i)
			}
			if visible != keep[i] {
				t.Fatalf("%s: sample %d (offset %d) visible %v, want %v", name, i, s.Offset[i], visible, keep[i])
			}
			if visible && (g.Lat[i] != s.Lat[i] || g.Lon[i] != s.Lon[i]) {
				t.Fatalf("%s: visible position %d changed", name, i)
			}
		}
	}
	// Everything else is identical.
	rest, restGot := w, got
	rest.Samples.Lat, rest.Samples.Lon, restGot.Samples.Lat, restGot.Samples.Lon = nil, nil, nil, nil
	if !reflect.DeepEqual(sameNaN(rest), sameNaN(restGot)) {
		t.Fatalf("%s: more than the positions changed", name)
	}
	checkRanges(t, name, w, got, fully)
	return w, got, fully
}

// checkRanges (ADR-030): HiddenRanges marks exactly the samples whose
// position HideEnds removes: a sample keeps its position iff it had one and
// lies outside every range. [] without positions, [[0, n-1]] when fully
// hidden, else two ordered, disjoint, non-empty sections at the ends.
func checkRanges(t *testing.T, name string, w, cut Workout, fully bool) {
	t.Helper()
	r := HiddenRanges(w)
	n := len(w.Samples.Offset)
	switch {
	case w.Samples.Lat == nil:
		if r != nil {
			t.Fatalf("%s: ranges %v without positions", name, r)
		}
		return
	case fully:
		if !slices.Equal(r, [][2]int{{0, n - 1}}) {
			t.Fatalf("%s: fully hidden, ranges %v, want [[0 %d]]", name, r, n-1)
		}
		return
	}
	if len(r) != 2 || r[0][0] != 0 || r[0][1] < r[0][0] || r[1][0] <= r[0][1]+1 || r[1][1] != n-1 || r[1][1] < r[1][0] {
		t.Fatalf("%s: ranges %v for %d samples", name, r, n)
	}
	for i := range n {
		in := i <= r[0][1] || i >= r[1][0]
		had := !math.IsNaN(w.Samples.Lat[i])
		if has := !math.IsNaN(cut.Samples.Lat[i]); has != (had && !in) {
			t.Fatalf("%s: sample %d public %v, had %v, in a range %v (%v)", name, i, has, had, in, r)
		}
	}
}

// sameNaN replaces NaN by a fixed value in every stream so DeepEqual can
// compare workouts.
func sameNaN(w Workout) Workout {
	for _, p := range w.Samples.streams() {
		if *p == nil {
			continue
		}
		v := slices.Clone(*p)
		for i := range v {
			if math.IsNaN(v[i]) {
				v[i] = -1e300
			}
		}
		*p = v
	}
	return w
}

// visibleRange returns the offsets of the first and last public position.
func visibleRange(t *testing.T, w Workout) (int32, int32) {
	t.Helper()
	first, last := int32(-1), int32(-1)
	for i := range w.Samples.Lat {
		if !math.IsNaN(w.Samples.Lat[i]) {
			if first < 0 {
				first = w.Samples.Offset[i]
			}
			last = w.Samples.Offset[i]
		}
	}
	if first < 0 {
		t.Fatal("no visible position")
	}
	return first, last
}

// line: a straight track east at v m/s from x0, one record per second.
func line(n int, v float64) []RawRecord {
	return track(seq(0, n-1), func(i int) (float64, float64) { return float64(i) * v, 0 })
}

func TestHideEndsStraightLine(t *testing.T) {
	// 2793 m east at 7 m/s: offset 43 (301 m) is the first fix more than
	// 300 m from the start, 356 (301 m) the last one from the end.
	_, got, fully := checkCut(t, "line", line(400, 7))
	if fully {
		t.Fatal("fully hidden")
	}
	if a, b := visibleRange(t, got); a != 43 || b != 356 {
		t.Errorf("visible %d..%d, want 43..356", a, b)
	}
}

func TestHideEndsOffByDefaultIsIdentity(t *testing.T) {
	// The transform is only applied with hiding on (publish); the cut keeps
	// nothing beyond what it must, and an indoor workout is unchanged.
	w := normalised(track(seq(0, 99), func(int) (float64, float64) { return nan, nan }))
	got, fully := HideEnds(w)
	if fully || got.Samples.Lat != nil {
		t.Errorf("indoor: fully %v, positions %v", fully, got.Samples.Lat != nil)
	}
	if !reflect.DeepEqual(sameNaN(got), sameNaN(w)) {
		t.Error("indoor workout changed")
	}
	if r := HiddenRanges(w); r != nil {
		t.Errorf("indoor: ranges %v", r)
	}
}

// TestHideEndsExactly300 (mutant "< for ≤"): a fix exactly 300 m from the
// first position is hidden.
func TestHideEndsExactly300(t *testing.T) {
	// On the equator, latitude steps are fine enough to hit 300.0 exactly.
	lo, hi := 0.0, 0.01
	for range 200 {
		mid := (lo + hi) / 2
		if haversine(0, 0, mid, 0) < HideRadius {
			lo = mid
		} else {
			hi = mid
		}
	}
	if haversine(0, 0, hi, 0) != HideRadius {
		t.Fatalf("no latitude at exactly %v m: %v", HideRadius, haversine(0, 0, hi, 0))
	}
	// Out 2.2 km and back to the start: the end circle is the same one.
	lats := []float64{0, hi / 2, hi, hi + 1e-6, 0.02, hi + 1e-6, hi, hi / 2, 0}
	w := Workout{Samples: Samples{Offset: []int32{0, 1, 2, 3, 4, 5, 6, 7, 8}, Lat: lats, Lon: make([]float64, len(lats))}}
	got, fully := HideEnds(w)
	if fully {
		t.Fatal("fully hidden")
	}
	var vis []int
	for i, v := range got.Samples.Lat {
		if !math.IsNaN(v) {
			vis = append(vis, i)
		}
	}
	if !slices.Equal(vis, []int{3, 4, 5}) {
		t.Errorf("visible %v, want [3 4 5]", vis)
	}
}

// TestHideEndsJitterAtStart (AC: two minutes of jitter within 50 m): the
// along-track length of the jitter is far above 300 m, but every position
// stays hidden until the track first gets more than 300 m from the start.
func TestHideEndsJitterAtStart(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	jx := make([]float64, 120)
	jy := make([]float64, 120)
	for i := range jx {
		a, d := r.Float64()*2*math.Pi, r.Float64()*50
		jx[i], jy[i] = d*math.Cos(a), d*math.Sin(a)
	}
	records := track(seq(0, 519), func(i int) (float64, float64) {
		if i < 120 {
			return jx[i], jy[i]
		}
		return float64(i-120) * 5, 0
	})
	w, got, _ := checkCut(t, "jitter", records)
	a, _ := visibleRange(t, got)
	i := slices.Index(w.Samples.Offset, a)
	x, y := jx[0], jy[0]
	lat, lon := pt(x, y)
	if d := haversine(lat, lon, w.Samples.Lat[i], w.Samples.Lon[i]); d <= HideRadius || a < 120 {
		t.Errorf("first visible at offset %d, %.0f m from the start", a, d)
	}
	if prev := w.Samples.Lat[i-1]; haversine(lat, lon, prev, w.Samples.Lon[i-1]) > HideRadius {
		t.Error("the sample before the first visible one is already outside the circle")
	}
}

// TestHideEndsFullyHidden: AC 500 m walk, AC 5 km on a 400 m track near
// the start, a single fix.
func TestHideEndsFullyHidden(t *testing.T) {
	r := 400 / (2 * math.Pi) // a 400 m circle
	for _, c := range []struct {
		name    string
		records []RawRecord
	}{
		{"500 m walk", line(357, 1.4)},
		{"599 m out", line(600, 1)},
		{"400 m track, 5 km", track(seq(0, 1499), func(i int) (float64, float64) {
			a := float64(i) * 5000 / 1500 / r
			return 100 + r*math.Cos(a), r * math.Sin(a)
		})},
		{"single fix", track(seq(0, 59), func(i int) (float64, float64) {
			if i == 30 {
				return 0, 0
			}
			return nan, nan
		})},
	} {
		_, got, fully := checkCut(t, c.name, c.records)
		if !fully || got.Samples.Lat != nil || got.Samples.Lon != nil {
			t.Errorf("%s: fully hidden %v", c.name, fully)
		}
		if got.Samples.HeartRate == nil || got.Samples.Speed == nil || got.Samples.Altitude == nil {
			t.Errorf("%s: other streams lost", c.name)
		}
	}
}

// TestHideEndsLateFix (AC: the fix starts 1 km after the start of
// recording): the circle is centred on the first positioned sample.
func TestHideEndsLateFix(t *testing.T) {
	records := track(seq(0, 799), func(i int) (float64, float64) {
		if i < 200 {
			return nan, nan
		}
		return float64(i) * 7, 0 // first fix at x = 1400
	})
	_, got, _ := checkCut(t, "late fix", records)
	if a, _ := visibleRange(t, got); a != 243 { // 301 m from the first fix
		t.Errorf("first visible offset %d, want 243", a)
	}
}

// TestHideEndsEndSide: the end circle around the last position; a track
// ending far from its start still hides its end.
func TestHideEndsEndSide(t *testing.T) {
	records := track(seq(0, 399), func(i int) (float64, float64) {
		if i >= 350 {
			return nan, nan // fix lost before the end of recording
		}
		return 0, float64(i) * 7
	})
	_, got, _ := checkCut(t, "end", records)
	if _, b := visibleRange(t, got); b != 306 { // last fix y = 2443; 306 is 301 m from it
		t.Errorf("last visible offset %d, want 306", b)
	}
}

// TestHideEndsMidRouteReturn: a pass near the start later in the route
// stays visible (only until the first exit is hidden).
func TestHideEndsMidRouteReturn(t *testing.T) {
	records := track(seq(0, 799), func(i int) (float64, float64) {
		switch {
		case i < 200:
			return float64(i) * 5, 0 // out to 1 km
		case i < 400:
			return float64(400-i) * 5, 0 // back to the start
		default:
			return 0, float64(i-400) * 5 // away north
		}
	})
	w, got, _ := checkCut(t, "return", records)
	i := slices.Index(w.Samples.Offset, 399)
	if math.IsNaN(got.Samples.Lat[i]) {
		t.Error("mid-route pass through the start circle hidden")
	}
}

// TestHideEndsBoundaryAfterFilledGap (H-3): a 3 s gap from a fix at 290 m to
// one at 320 m is filled with samples at 300 m and 310 m; they are hidden,
// the first visible position is the recorded fix. The same at the end.
func TestHideEndsBoundaryAfterFilledGap(t *testing.T) {
	// Recorded offsets 0..29, 32..270, 273..302 at 10 m/s: the gaps 29 → 32
	// and 270 → 273 are filled (R-3).
	offsets := append(seq(0, 29), seq(32, 270)...)
	offsets = append(offsets, seq(273, 302)...)
	records := track(offsets, func(i int) (float64, float64) { return float64(offsets[i]) * 10, 0 })
	w, got, _ := checkCut(t, "filled gap", records)
	if w.Samples.Filled == 0 {
		t.Fatal("no gap filled")
	}
	a, b := visibleRange(t, got)
	if a != 32 {
		t.Errorf("first visible offset %d, want the recorded fix at 32", a)
	}
	// Last fix at 302 (3020 m); 273 is 290 m from it, 270 is 320 m.
	if b != 270 {
		t.Errorf("last visible offset %d, want the recorded fix at 270", b)
	}
}

// TestHideEndsProperty runs the oracle over generated tracks: jitter, loops,
// out-and-back, returns near the start, dropouts, filled and unfilled gaps
// across the boundary, late fixes, short tracks, several sequences.
func TestHideEndsProperty(t *testing.T) {
	for seed := range uint64(2000) {
		records, starts := genTrack(seed)
		checkCutSeq(t, seed, records, starts)
	}
}

// FuzzHideEnds explores generated tracks by seed.
func FuzzHideEnds(f *testing.F) {
	for _, s := range []uint64{0, 1, 7, 42, 1234} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, seed uint64) {
		records, starts := genTrack(seed)
		checkCutSeq(t, seed, records, starts)
	})
}

func checkCutSeq(t *testing.T, seed uint64, records []RawRecord, starts []int) {
	t.Helper()
	w := Normalise(Recording{FileType: "activity", Records: records, SequenceStarts: starts})
	if w.Samples.Lat == nil {
		return
	}
	checkInvariants(t, w)
	// checkCut normalises without sequences; the cut does not depend on them
	// (only the distance stream does), so check both.
	got, fully := HideEnds(w)
	keep, wantFully := expectCut(w, records)
	if fully != wantFully {
		t.Fatalf("seed %d: fully %v, want %v", seed, fully, wantFully)
	}
	if !fully {
		for i := range keep {
			if !math.IsNaN(got.Samples.Lat[i]) != keep[i] {
				t.Fatalf("seed %d: sample %d visible %v, want %v", seed, i, !keep[i], keep[i])
			}
		}
	}
	checkRanges(t, "seed", w, got, fully)
	checkCut(t, "seed", records)
}

// genTrack generates a random track in metres around (lat0, lon0).
func genTrack(seed uint64) ([]RawRecord, []int) {
	r := rand.New(rand.NewPCG(seed, 16))
	n := 1 + r.IntN(900)
	if r.IntN(10) == 0 {
		n = 1 + r.IntN(5) // tiny
	}
	var offsets []int
	off := r.IntN(3)
	for range n {
		offsets = append(offsets, off)
		switch g := r.IntN(20); {
		case g == 0:
			off += 2 + r.IntN(3) // filled gap
		case g == 1:
			off += 5 + r.IntN(60) // real gap
		default:
			off++
		}
	}
	xs, ys := make([]float64, n), make([]float64, n)
	x, y, heading := 0.0, 0.0, r.Float64()*2*math.Pi
	speed := 1 + r.Float64()*10
	mode := 0
	lateFix := 0
	if r.IntN(5) == 0 {
		lateFix = r.IntN(n)
	}
	dropFrom, dropTo := -1, -1
	if r.IntN(3) == 0 {
		dropFrom = r.IntN(n)
		dropTo = dropFrom + r.IntN(200)
	}
	for i := range n {
		if r.IntN(60) == 0 {
			mode = r.IntN(4)
		}
		switch mode {
		case 0: // wander
			heading += (r.Float64() - 0.5) * 0.4
		case 1: // head home
			heading = math.Atan2(-y, -x)
		case 2: // jitter on the spot
			xs[i], ys[i] = x+(r.Float64()-0.5)*40, y+(r.Float64()-0.5)*40
			continue
		case 3: // loop
			heading += 0.05
		}
		x += math.Cos(heading) * speed
		y += math.Sin(heading) * speed
		xs[i], ys[i] = x, y
	}
	records := track(offsets, func(i int) (float64, float64) {
		if i < lateFix || (i >= dropFrom && i < dropTo) || r.IntN(50) == 0 {
			return nan, nan
		}
		return xs[i], ys[i]
	})
	var starts []int
	if n > 10 && r.IntN(4) == 0 {
		starts = []int{n / 2}
	}
	return records, starts
}
