package publish

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/andybalholm/brotli"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
)

// F-68 extra sensor streams at the public build (XS-3, XS-7; UP-9, SH-4).
// The markers below are the names, units, application IDs and values that
// tools/corpusgen/f68.go writes into developer-hostile.fit and the Stryd files.

var hostileMarkers = []string{
	"Home address", "Elm Street", "Springfield", "Hometown population", "inhabitants",
	"Start latitude", "Start longitude", "Latitude degrees", "Longitude degrees", "semicircles",
	"SECRET-APP-ID-47", "Watts", "stryd-app-one", "other-power-app", "Form Power", "Leg Spring Stiffness", "kN/m",
}

func semicircles(deg float64) int32 { return int32(math.Round(deg * 2147483648.0 / 180)) }

// hostileValueBytes are byte patterns of injected values that no honest
// column holds: the semicircle values and degrees of the injected position.
func hostileValueBytes() [][]byte {
	var out [][]byte
	for _, t := range []int{0, 1, 500, 1199} {
		lat, lon := 47.3769+float64(t)*1e-6, 8.5417+float64(t)*1e-6
		for _, v := range []int32{semicircles(lat), semicircles(lon)} {
			out = append(out, binary.LittleEndian.AppendUint32(nil, uint32(v)))
		}
		for _, v := range []float64{lat, lon} {
			out = append(out, binary.LittleEndian.AppendUint64(nil, math.Float64bits(v)))
		}
	}
	return out
}

// injectedSeries are the numeric developer series of the hostile file by second.
func injectedSeries(n int) [][]int64 {
	f := []func(t int) int64{
		func(t int) int64 { return int64(4000 + t%50) },
		func(t int) int64 { return int64(1111 + t%7) },
		func(t int) int64 { return int64(1112 + t%7) },
		func(t int) int64 { return int64(2500 + t%7) },
		func(t int) int64 { return int64(300 + t%7) },
		func(t int) int64 { return int64(310 + t%7) },
		func(t int) int64 { return int64(320 + t%7) },
		func(t int) int64 { return int64(330 + t%7) },
	}
	var out [][]int64
	for _, g := range f {
		s := make([]int64, n)
		for t := range s {
			s[t] = g(t)
		}
		out = append(out, s)
	}
	return out
}

func readSynthetic(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(synthetic, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// allFiles returns the content of every file under dir (names included in the key).
func allFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// sweep fails when any marker appears in any byte of the files, the decoded
// data files, or log.
func sweep(t *testing.T, files map[string][]byte, log []byte) {
	t.Helper()
	files["log"] = log
	for name, b := range files {
		blobs := [][]byte{b}
		if strings.HasSuffix(name, ".br") {
			plain, err := io.ReadAll(brotli.NewReader(bytes.NewReader(b)))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			blobs = append(blobs, plain)
		}
		for _, blob := range blobs {
			for _, m := range hostileMarkers {
				if bytes.Contains(blob, []byte(m)) {
					t.Errorf("%s holds %q", name, m)
				}
			}
			for _, v := range hostileValueBytes() {
				if bytes.Contains(blob, v) {
					t.Errorf("%s holds an injected value % x", name, v)
				}
			}
		}
	}
}

func TestDeveloperHostilePublishesNothing(t *testing.T) {
	orig := readSynthetic(t, "developer-hostile.fit")
	var logs bytes.Buffer
	parser := ingest.NewParser(ingest.DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	for _, m := range hostileMarkers[:11] { // the sweep must not be vacuous
		if !bytes.Contains(orig, []byte(m)) {
			t.Fatalf("the hostile file lacks %q", m)
		}
	}
	for _, v := range hostileValueBytes() {
		if !bytes.Contains(orig, v) {
			t.Fatalf("the hostile file lacks the value % x", v)
		}
	}
	for _, hide := range []bool{false, true} {
		fx := newFixtureWith(t, orig, parser)
		d := draft()
		if hide {
			d.HideStartM, d.HideEndM = 300, 300
		}
		fx.change(t, d, published(d))
		o := fx.read(t)

		var names []string
		for _, s := range o.file.Streams {
			names = append(names, s.Descriptor.Name)
		}
		want := []string{"time_offset", "latitude", "longitude", "altitude", "distance", "speed", "heart_rate", "cadence", "temperature", "moving_time", "elevation_gain"}
		if !slices.Equal(names, want) {
			t.Errorf("hide %v: streams %v, want %v", hide, names, want)
		}
		series := injectedSeries(o.file.SampleCount)
		for _, s := range o.file.Streams {
			for i, inj := range series {
				if s.Descriptor.Name != "time_offset" && len(s.Ints) == len(inj) && slices.Equal(s.Ints, inj) {
					t.Errorf("column %s equals injected series %d", s.Descriptor.Name, i)
				}
			}
		}
		sweep(t, allFiles(t, filepath.Dir(fx.root)), logs.Bytes())
	}
	// The draft file is the same pipeline's encode of the parse.
	w, err := parser.Parse(context.Background(), orig)
	if err != nil {
		t.Fatal(err)
	}
	draftBin, err := datafile.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	sweep(t, map[string][]byte{"draft.bin": draftBin}, nil)
}

// TestStrydPublishes: Stryd power is the power stream of the public data
// file (XS-6), and none of the app's names or other fields come with it.
func TestStrydPublishes(t *testing.T) {
	orig := readSynthetic(t, "stryd-power.fit")
	var logs bytes.Buffer
	parser := ingest.NewParser(ingest.DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	fx := newFixtureWith(t, orig, parser)
	d := draft()
	fx.change(t, d, published(d))
	o := fx.read(t)
	s, ok := stream(o.file, "power")
	if !ok {
		t.Fatal("no power stream")
	}
	if !s.Valid[0] || s.Ints[0] != 240 {
		t.Errorf("power[0] = %d, want Stryd's 240", s.Ints[0])
	}
	if !bytes.Contains(o.page, []byte("Avg power")) && !bytes.Contains(o.page, []byte("Power")) {
		t.Error("the page markup has no power in its summary strip")
	}
	files := allFiles(t, filepath.Dir(fx.root))
	// "Watts" and "Power" are the allowlist's own strings: only the others must be absent.
	markers := hostileMarkers
	hostileMarkers = slices.DeleteFunc(slices.Clone(markers), func(m string) bool { return m == "Watts" })
	defer func() { hostileMarkers = markers }()
	sweep(t, files, logs.Bytes())
}

// TestStrydNativeWins: with both, the public power stream holds the native
// values only.
func TestStrydNativeWins(t *testing.T) {
	fx := newFixtureWith(t, readSynthetic(t, "stryd-and-native-power.fit"), ingest.NewParser(ingest.DefaultLimits, nil))
	d := draft()
	fx.change(t, d, published(d))
	o := fx.read(t)
	s, _ := stream(o.file, "power")
	for i, v := range s.Ints {
		if want := int64(150 + i%40); v != want {
			t.Fatalf("power[%d] = %d, want native %d", i, v, want)
		}
	}
}

// TestDynamicsPublish: the four streams are public like cadence, hidden
// sections keep them (SH-4 hides positions only) and the missing seconds stay
// missing.
func TestDynamicsPublish(t *testing.T) {
	orig := readSynthetic(t, "run-dynamics.fit")
	parser := ingest.NewParser(ingest.DefaultLimits, nil)
	off, on := publishBoth(t, orig, parser)
	for name, o := range map[string]output{"off": off, "on": on} {
		for _, n := range []string{"vertical_oscillation", "ground_contact_time", "stride_length", "vertical_ratio"} {
			s, ok := stream(o.file, n)
			if !ok {
				t.Fatalf("%s: no %s", name, n)
			}
			if s.Valid[0] || s.Valid[100] || !s.Valid[50] || !s.Valid[200] {
				t.Errorf("%s %s: valid at 0, 100, 200 = %v %v %v", name, n, s.Valid[0], s.Valid[100], s.Valid[200])
			}
		}
	}
	for _, n := range []string{"vertical_oscillation", "ground_contact_time", "stride_length", "vertical_ratio"} {
		a, _ := stream(off.file, n)
		b, _ := stream(on.file, n)
		if !slices.Equal(a.Ints, b.Ints) {
			t.Errorf("%s changes with hiding", n)
		}
	}
	lat, _ := stream(on.file, "latitude")
	if lat.Valid[50] {
		t.Error("the first 300 m keep their position while hidden")
	}
}
