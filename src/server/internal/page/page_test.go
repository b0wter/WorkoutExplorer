package page

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
)

// The samples (F-13 D-3) are the contract: Build must reproduce every one as
// an equal JSON value (F-15 P-11). The specs mirror src/web/src/lib/page/samples.ts.
type sampleSpec struct {
	name, golden, title, description string
	transform                        func(*datafile.File)
	routeHidden                      bool // F-16
}

// withoutHeartRate mirrors samples.ts withoutHeartRate: the stream and every
// heart-rate total are dropped.
func withoutHeartRate(f *datafile.File) {
	var keep []datafile.Stream
	for _, s := range f.Streams {
		if s.Descriptor.Name != "heart_rate" {
			keep = append(keep, s)
		}
	}
	f.Streams = keep
	f.Meta.Summary.AvgHeartRate, f.Meta.Summary.MaxHeartRate = "", ""
	for i := range f.Meta.Sessions {
		f.Meta.Sessions[i].Totals.AvgHeartRate, f.Meta.Sessions[i].Totals.MaxHeartRate = "", ""
	}
	for i := range f.Meta.Laps {
		f.Meta.Laps[i].Totals.AvgHeartRate, f.Meta.Laps[i].Totals.MaxHeartRate = "", ""
	}
}

// lapCrossing mirrors samples.ts lapCrossing (F-42 MS-3).
func lapCrossing(f *datafile.File) {
	laps := slices.Clone(f.Meta.Laps)
	laps[0].End, laps[0].EndIndex = 629, 630
	laps[1].Start, laps[1].StartIndex = 630, 630
	f.Meta.Laps = laps
}

// legWithoutLaps mirrors samples.ts legWithoutLaps (F-42 MS-3).
func legWithoutLaps(f *datafile.File) {
	f.Meta.Laps = slices.Delete(slices.Clone(f.Meta.Laps), 3, 4)
}

// withoutLengths mirrors samples.ts withoutLengths (F-44 PS-5).
func withoutLengths(f *datafile.File) { f.Meta.Summary.Lengths = "" }

// hostileDevice mirrors samples.ts hostileDevice (QA-43-r1-3): markup in a recorded device name.
func hostileDevice(f *datafile.File) { f.Meta.Device.Name = `<i>Dev</i> &amp; "x"` }

var samples = []sampleSpec{
	{"ride-30min", "ride-30min", "Morning loop", "Easy spin.\nSecond line.", nil, false},
	{"ride-12h", "ride-12h", "", "", nil, false},
	{"run-30min", "run-30min", "", "", nil, false},
	{"ride-no-hr", "ride-30min", "", "", withoutHeartRate, false},
	{"indoor-ride", "indoor-no-gps", "Trainer", "", nil, false},
	{"indoor-unknown-zone", "indoor-no-distance-no-gps", "", "", nil, false},
	{"route-hidden", "indoor-no-gps", "", "", nil, true},
	{"triathlon", "triathlon", "", "", nil, false},
	{"triathlon-lap-crossing", "triathlon", "", "", lapCrossing, false},
	{"triathlon-leg-without-laps", "triathlon", "", "", legWithoutLaps, false},
	{"triathlon-cut", "triathlon-cut-90", "", "", nil, false},
	{"two-rides", "chained-ride-ride", "", "", nil, false},
	{"pool-swim", "pool-swim", "", "", nil, false},
	{"pool-swim-25yd", "pool-swim-25yd", "", "", nil, false},
	{"pool-swim-no-lengths", "pool-swim", "", "", withoutLengths, false},
	{"ride-no-laps", "ride-no-laps", "", "", nil, false},
	{"hostile-text", "ride-30min", `<b>Bold</b> & "quoted" <script>alert(1)</script>`,
		"<script>alert(2)</script>\n<img src=x onerror=alert(3)> &amp; done", hostileDevice, false},
}

const (
	goldenDir  = "../../testdata/datafile/synthetic"
	samplesDir = "../../testdata/page"
)

var expiresAt = time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)

func TestSamples(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(samplesDir, "*.json"))
	if len(files) != len(samples) {
		t.Fatalf("%d committed samples, %d specs: keep the list in step with samples.ts", len(files), len(samples))
	}
	for _, sp := range samples {
		t.Run(sp.name, func(t *testing.T) {
			bin, err := os.ReadFile(filepath.Join(goldenDir, sp.golden+".bin"))
			if err != nil {
				t.Fatal(err)
			}
			f, err := datafile.Decode(bin)
			if err != nil {
				t.Fatal(err)
			}
			if sp.transform != nil {
				sp.transform(&f)
			}
			sum := sha256.Sum256(bin)
			vm := Build(f, Settings{
				Title: sp.title, Description: sp.description, ExpiresAt: expiresAt,
				PublicID: sp.name, DataFile: "data." + hex.EncodeToString(sum[:8]) + ".bin", RouteHidden: sp.routeHidden,
			})
			got, err := json.Marshal(vm)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(samplesDir, sp.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var g, w map[string]any
			if err := json.Unmarshal(got, &g); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(want, &w); err != nil {
				t.Fatal(err)
			}
			// Go only (F-23 PC-6): the samples carry a placeholder; TestCard covers it.
			delete(g, "Card")
			delete(w, "Card")
			if !reflect.DeepEqual(g, w) {
				t.Errorf("view model differs from the sample\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// Constructed ties (P-11): no sample may contain one, so they are tested
// here. The expected strings are what V8 prints; lib/format's tests hold the
// same table on the TypeScript side.
func TestJavaScriptRounding(t *testing.T) {
	for _, c := range []struct {
		v    float64
		d    int
		want string
	}{
		{0.5, 0, "1"}, {1.5, 0, "2"}, {2.5, 0, "3"}, {-2.5, 0, "-3"}, {-0.5, 0, "-1"},
		{0.25, 1, "0.3"}, {0.125, 2, "0.13"}, {2.675, 2, "2.67"}, {1.005, 2, "1.00"},
		{0.35, 1, "0.3"}, {12.5, 0, "13"}, {13.5, 0, "14"}, {-0.4, 0, "0"}, {-0.04, 1, "0.0"},
		{0, 2, "0.00"}, {9.995, 2, "9.99"}, {9.96, 1, "10.0"}, {7.5 / 3.6 * 3.6, 1, "7.5"},
		{1e-7, 2, "0.00"}, {123456.785, 2, "123456.79"},
	} {
		if got := formatFixed(c.v, c.d); got != c.want {
			t.Errorf("formatFixed(%v, %d) = %q, want %q", c.v, c.d, got, c.want)
		}
	}
	for _, c := range []struct{ v, want float64 }{
		{0.5, 1}, {1.5, 2}, {2.5, 3}, {-0.5, 0}, {-1.5, -1}, {-2.5, -2}, {0.49999999999999994, 0}, {299.5, 300}, {-0.4, 0},
	} {
		if got := jsRound(c.v); got != c.want {
			t.Errorf("jsRound(%v) = %v, want %v", c.v, got, c.want)
		}
	}
	// Pace: 1000 m at 4 m/s is 250 s exactly; at 1000/299.5 m/s the pace is a tie.
	if got := formatPace(299.5); got != "5:00" {
		t.Errorf("pace tie: %q", got)
	}
	if got := formatPace(298.5); got != "4:59" {
		t.Errorf("pace tie: %q", got)
	}
	if got := formatSpeedOrPace("running", 4); got != "4:10" {
		t.Errorf("pace: %q", got)
	}
	if got := formatSpeedOrPace("cycling", 0); got != "" {
		t.Errorf("zero speed: %q", got)
	}
}

func TestTexts(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"cycling", "Ride"}, {"multisport", "Multisport"}, {"cross_country_skiing", "Cross country skiing"},
		{"", "Workout"}, {"_", "Workout"}, {"training", "Training"},
	} {
		if got := sportName(c.in); got != c.want {
			t.Errorf("sportName(%q) = %q", c.in, got)
		}
	}
	if sportIcon("multisport") != "generic" || sportIcon("swimming") != "swimming" || sportIcon("yoga") != "generic" {
		t.Error("sportIcon")
	}
	if got := formatElapsed(3600); got != "1:00:00" {
		t.Error(got)
	}
	if got := formatElapsed(59.9); got != "0:59" {
		t.Error(got)
	}
	if got := formatLocalStart("2026-10-09T23:30:00Z", true, 7200); got != "10 Oct 2026, 01:30" {
		t.Error(got)
	}
	if got := formatLocalStart("2026-10-09T23:30:00Z", false, 7200); got != "9 Oct 2026, 23:30 UTC" {
		t.Error(got)
	}
	if jsTrim("\ufeff x \u00a0") != "x" || jsTrim("\u0085") != "\u0085" {
		t.Error("jsTrim")
	}
}

// TestDistanceAdvances (F-18 L-6, L-8): the last valid distance above the
// first; missing samples at either end are skipped. lib/page holds the same table.
func TestDistanceAdvances(t *testing.T) {
	file := func(ints []int64, valid []bool) datafile.File {
		return datafile.File{Streams: []datafile.Stream{{Descriptor: datafile.Descriptor{Name: "distance"}, Ints: ints, Valid: valid}}}
	}
	for _, c := range []struct {
		name  string
		ints  []int64
		valid []bool
		want  bool
	}{
		{"advances", []int64{0, 5, 9}, []bool{true, true, true}, true},
		{"flat", []int64{4, 4, 4}, []bool{true, true, true}, false},
		{"missing ends", []int64{0, 3, 7, 0}, []bool{false, true, true, false}, true},
		{"one valid", []int64{0, 3, 0}, []bool{false, true, false}, false},
		{"none valid", []int64{0, 0}, []bool{false, false}, false},
	} {
		if got := distanceAdvances(file(c.ints, c.valid)); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if distanceAdvances(datafile.File{}) {
		t.Error("no stream")
	}
}

// TestCard (F-23 PC-3, PC-6): Go only, so tested here rather than by the samples.
func TestCard(t *testing.T) {
	load := func(golden string) datafile.File {
		bin, err := os.ReadFile(filepath.Join(goldenDir, golden+".bin"))
		if err != nil {
			t.Fatal(err)
		}
		f, err := datafile.Decode(bin)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	maxOnly := load("ride-30min")
	maxOnly.Meta.Summary.AvgHeartRate = ""
	zero := load("ride-30min")
	zero.Meta.Summary.Distance = "0"
	noHR := load("indoor-no-distance-no-gps")
	withoutHeartRate(&noHR)
	flat := load("ride-30min")
	flat.Meta.Summary.Ascent = "0.4"
	noAltitude := load("ride-30min")
	for i, s := range noAltitude.Streams {
		if s.Descriptor.Name == "altitude" {
			noAltitude.Streams = append(noAltitude.Streams[:i], noAltitude.Streams[i+1:]...)
			break
		}
	}
	rowing := load("ride-30min")
	rowing.Meta.Sport = "rowing"
	const img = "https://wx.test/w/id/preview.0123456789abcdef.png"
	for _, c := range []struct {
		name, title string
		f           datafile.File
		hidden      bool
		want        Card
	}{
		{"ride", "Morning loop", load("ride-30min"), false, Card{"Morning loop", "Distance 13.46 km · Moving time 29:59 · Elevation gain 104 m · Avg speed 26.9 km/h · Avg heart rate 142 bpm", img}},
		{"run: pace", "", load("run-30min"), false, Card{"Run, 30 Jun 2026", "Distance 5.94 km · Moving time 29:59 · Elevation gain 104 m · Avg pace 5:03 /km · Avg heart rate 142 bpm", img}},
		{"swim: pace per 100 m", "x", load("pool-swim"), false, Card{"x", "Distance 1.00 km · Elapsed time 20:00 · Avg pace 2:00 /100 m · Avg heart rate 132 bpm", img}},
		{"max heart rate only", "x", maxOnly, false, Card{"x", "Distance 13.46 km · Moving time 29:59 · Elevation gain 104 m · Avg speed 26.9 km/h", img}},
		{"distance 0", "x", zero, false, Card{"x", "Moving time 29:59 · Elevation gain 104 m · Avg heart rate 142 bpm", img}},
		{"gain rounds to 0", "x", flat, false, Card{"x", "Distance 13.46 km · Moving time 29:59 · Avg speed 26.9 km/h · Avg heart rate 142 bpm", img}},
		{"no altitude recorded", "x", noAltitude, false, Card{"x", "Distance 13.46 km · Moving time 29:59 · Avg speed 26.9 km/h · Avg heart rate 142 bpm", img}},
		{"other sport: no speed", "x", rowing, false, Card{"x", "Distance 13.46 km · Moving time 29:59 · Elevation gain 104 m · Avg heart rate 142 bpm", img}},
		{"indoor without heart rate", "", noHR, false, Card{"Training, 7 Jun 2026", "Elapsed time 30:00", img}},
		{"fully hidden", "", load("indoor-no-gps"), true, Card{"Ride, 6 Jun 2026", "Distance 29.89 km · Elapsed time 1:00:00 · Avg speed 29.9 km/h · Avg heart rate 142 bpm", img}},
	} {
		vm := Build(c.f, Settings{Title: c.title, PublicID: "id", DataFile: "data.0.bin", RouteHidden: c.hidden,
			BaseURL: "https://wx.test", Preview: "preview.0123456789abcdef.png"})
		if vm.Card != c.want {
			t.Errorf("%s: %+v\nwant %+v", c.name, vm.Card, c.want)
		}
	}
}

// TestRowTitle (F-37 DK-7): from the row alone, the same title and start
// text as Build makes from the data file.
func TestRowTitle(t *testing.T) {
	start := time.Date(2026, 10, 9, 23, 30, 0, 0, time.UTC)
	off := 3600
	for _, c := range []struct {
		title, sport string
		start        *time.Time
		off          *int
	}{
		{"", "cycling", &start, &off},
		{"  ", "trail_running", &start, nil},
		{"Morning loop", "running", &start, &off},
		{"", "", nil, nil},
	} {
		f := datafile.File{Meta: datafile.Meta{Sport: c.sport, StartTime: time.Time{}.Format(time.RFC3339)}}
		if c.start != nil {
			f.Meta.StartTime = c.start.Format(time.RFC3339)
		}
		if c.off != nil {
			f.Meta.TimeZone = datafile.TimeZone{Known: true, OffsetSeconds: int32(*c.off)}
		}
		vm := Build(f, Settings{Title: c.title})
		if title, startText := RowTitle(c.title, c.sport, c.start, c.off); title != vm.Header.Title || startText != vm.Header.Start {
			t.Errorf("%+v: %q %q, want %q %q", c, title, startText, vm.Header.Title, vm.Header.Start)
		}
	}
	if title, s := RowTitle("", "cycling", &start, &off); title != "Ride, 10 Oct 2026" || s != "10 Oct 2026, 00:30" {
		t.Errorf("%q %q", title, s)
	}
}

// TestSections (F-42 MS-1 to MS-3) holds the edge cases no sample has;
// lib/sections' tests hold the same table. Rows are lap numbers.
func TestSections(t *testing.T) {
	type s = datafile.Session
	type l = datafile.Lap
	for _, c := range []struct {
		name     string
		sessions []s
		laps     []l
		want     string
	}{
		{"one sport", []s{{Sport: "cycling"}, {Sport: "cycling", Start: 9, StartIndex: 5}}, []l{{}}, ""},
		{"no laps", []s{{Sport: "running"}, {Sport: "cycling", Start: 9, StartIndex: 5}}, nil, ""},
		{"start order, ties in file order, T numbers",
			[]s{{Sport: "running", Start: 20, StartIndex: 8}, {Sport: "transition", Start: 10, StartIndex: 4}, {Sport: "swimming", Start: 0, StartIndex: 3}, {Sport: "transition", Start: 20, StartIndex: 8}},
			[]l{{StartIndex: 0}}, "Swim[1] T1[] Run[] T2[]"},
		{"lap out of order stays under the separator above; trailing sections",
			[]s{{Sport: "swimming"}, {Sport: "cycling", Start: 5, StartIndex: 5}, {Sport: "running", Start: 9, StartIndex: 9}},
			[]l{{StartIndex: 6}, {StartIndex: 1}}, "Swim[] Ride[1 2] Run[]"},
	} {
		rows := make([][]string, len(c.laps))
		for i := range rows {
			rows[i] = []string{strconv.Itoa(i + 1)}
		}
		var got []string
		for _, x := range lapSections(sections(datafile.Meta{Sessions: c.sessions}), c.laps, rows, 3) {
			n := []string{}
			for _, r := range x.Rows {
				n = append(n, r[0])
			}
			got = append(got, x.Name+"["+strings.Join(n, " ")+"]")
		}
		if g := strings.Join(got, " "); g != c.want {
			t.Errorf("%s: %q, want %q", c.name, g, c.want)
		}
	}
}
