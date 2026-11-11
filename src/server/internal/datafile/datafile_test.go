package datafile

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"workout-explorer/internal/workout"
)

// ---- header and layout --------------------------------------------------------

func TestHeader(t *testing.T) {
	b := mustEncode(t, fullWorkout(10))
	if string(b[:4]) != "WEXD" {
		t.Errorf("magic = %q", b[:4])
	}
	if v := binary.LittleEndian.Uint16(b[4:]); v != 3 {
		t.Errorf("version = %d, want 3", v)
	}
	if n := binary.LittleEndian.Uint32(b[6:]); n != 10 {
		t.Errorf("sample count = %d, want 10", n)
	}
	l := int(binary.LittleEndian.Uint32(b[10:]))
	js := b[14 : 14+l]
	var m map[string]any
	if err := json.Unmarshal(js, &m); err != nil {
		t.Fatalf("metadata at byte 14 does not parse: %v", err)
	}
	if bytes.HasSuffix(js, []byte("\n")) {
		t.Error("metadata has a trailing newline")
	}
	first := align4(14 + l)
	for i := 14 + l; i < first; i++ {
		if b[i] != 0 {
			t.Errorf("padding byte %d = %#x, want 0", i, b[i])
		}
	}
	f := mustDecode(t, b)
	if got := f.Streams[0].Descriptor.ByteOffset; got != first {
		t.Errorf("first column at %d, want align4(14 + L) = %d", got, first)
	}
	if f.MetadataLength != l || f.SampleCount != 10 || f.Version != 3 {
		t.Errorf("File = version %d, N %d, L %d", f.Version, f.SampleCount, f.MetadataLength)
	}
}

// workedExample is the workout of Format §8.
func workedExample() workout.Workout {
	w := workout.Workout{Sport: "cycling", StartTime: testStart}
	w.Samples = workout.Samples{
		Offset:      []int32{0, 1, 2, 10, 11},
		Lat:         []float64{47.1234564, 47.1234580, nan, 47.1240004, 47.1240012},
		Lon:         []float64{8.5000004, 8.5000011, nan, 8.5000500, 8.5000507},
		HeartRate:   []float64{120.4, 121, nan, 130.6, 132},
		Temperature: []float64{-2.4, -2.6, -3, nan, -3},
	}
	return w
}

func le32(vs ...int32) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.LittleEndian.AppendUint32(b, uint32(v))
	}
	return b
}

// TestWorkedExample checks the column bytes of Format §8, relative to
// align4(14 + L) because this metadata is not exactly 401 bytes long.
func TestWorkedExample(t *testing.T) {
	b := mustEncode(t, workedExample())
	l := int(binary.LittleEndian.Uint32(b[10:]))
	base := align4(14 + l)
	const s = math.MaxInt32
	want := bytes.Join([][]byte{
		le32(0, 1, 1, 8, 1),
		le32(47123456, 2, s, 542, 1),
		le32(8500000, 1, s, 49, 1),
		{0x78, 0x79, 0xFF, 0x83, 0x84},
		{0, 0, 0},
		{0xFE, 0xFD, 0xFD, 0x7F, 0xFD},
	}, nil)
	if !bytes.Equal(b[base:], want) {
		t.Errorf("columns =\n% x\nwant\n% x", b[base:], want)
	}
	if len(b) != base+73 {
		t.Errorf("file length = %d, want %d", len(b), base+73)
	}

	f := mustDecode(t, b)
	wantOffsets := []int{base, base + 20, base + 40, base + 60, base + 68}
	for i, st := range f.Streams {
		if st.Descriptor.ByteOffset != wantOffsets[i] {
			t.Errorf("%s at %d, want %d", st.Descriptor.Name, st.Descriptor.ByteOffset, wantOffsets[i])
		}
	}
	wantVals := map[string][]float64{
		"time_offset": {0, 1, 2, 10, 11},
		"latitude":    {47.123456, 47.123458, nan, 47.124000, 47.124001},
		"longitude":   {8.500000, 8.500001, nan, 8.500050, 8.500051},
		"heart_rate":  {120, 121, nan, 131, 132},
		"temperature": {-2, -3, -3, nan, -3},
	}
	if got := streamNames(f); !reflect.DeepEqual(got, []string{"time_offset", "latitude", "longitude", "heart_rate", "temperature"}) {
		t.Fatalf("streams = %v", got)
	}
	for name, want := range wantVals {
		st, _ := streamByName(f, name)
		for i, v := range st.Values() {
			if math.IsNaN(want[i]) != math.IsNaN(v) || (!math.IsNaN(v) && math.Abs(v-want[i]) > 1e-9) {
				t.Errorf("%s[%d] = %v, want %v", name, i, v, want[i])
			}
		}
	}
}

func TestAlignment(t *testing.T) {
	uint8Only := func(n int) workout.Workout {
		w := offsetsOnly(n)
		w.Samples.HeartRate = make([]float64, n)
		w.Samples.Cadence = make([]float64, n)
		w.Samples.Temperature = make([]float64, n)
		for i := range n {
			w.Samples.HeartRate[i], w.Samples.Cadence[i], w.Samples.Temperature[i] = 130, 90, 20
		}
		return w
	}
	for _, n := range []int{1, 2, 3, 5} {
		for name, w := range map[string]workout.Workout{"every stream": fullWorkout(n), "uint8 streams": uint8Only(n)} {
			b := mustEncode(t, w)
			f := mustDecode(t, b)
			end := 0
			for _, s := range f.Streams {
				off := s.Descriptor.ByteOffset
				if off%4 != 0 {
					t.Errorf("%s N=%d: %s at %d is not 4-aligned", name, n, s.Descriptor.Name, off)
				}
				if end != 0 && off != align4(end) {
					t.Errorf("%s N=%d: %s at %d, previous column ends at %d", name, n, s.Descriptor.Name, off, end)
				}
				spec, _ := specByName(s.Descriptor.Name)
				end = off + n*spec.size
				for i := end; i < align4(end) && i < len(b); i++ {
					if b[i] != 0 {
						t.Errorf("%s N=%d: padding after %s not zero", name, n, s.Descriptor.Name)
					}
				}
			}
			if len(b) != end {
				t.Errorf("%s N=%d: file length %d, last column ends at %d (no trailing padding)", name, n, len(b), end)
			}
		}
	}
}

func TestOrderAndPresence(t *testing.T) {
	f := mustDecode(t, mustEncode(t, fullWorkout(4)))
	want := []string{"time_offset", "latitude", "longitude", "altitude", "distance", "speed", "heart_rate", "cadence", "power", "temperature"}
	if got := streamNames(f); !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}

	w := fullWorkout(4)
	w.Samples.Power = nil                                          // absent
	w.Samples.Cadence = []float64{nan, nan, nan, nan}              // all missing
	w.Samples.Speed = []float64{math.Inf(1), 5, math.Inf(-1), nan} // ±Inf is missing
	f = mustDecode(t, mustEncode(t, w))
	want = []string{"time_offset", "latitude", "longitude", "altitude", "distance", "speed", "heart_rate", "temperature"}
	if got := streamNames(f); !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	sp, _ := streamByName(f, "speed")
	if !reflect.DeepEqual(sp.Valid, []bool{false, true, false, false}) || sp.Ints[1] != 5000 {
		t.Errorf("speed = %v %v", sp.Ints, sp.Valid)
	}
	var m map[string]any
	if err := json.Unmarshal(metaBytes(mustEncode(t, w)), &m); err != nil {
		t.Fatal(err)
	}
	for _, d := range m["streams"].([]any) {
		if n := d.(map[string]any)["name"]; n == "power" || n == "cadence" {
			t.Errorf("descriptor for %v written", n)
		}
	}
}

func TestPairing(t *testing.T) {
	w := fullWorkout(4)
	w.Samples.Lon[1] = nan
	w.Samples.Lat[2] = nan
	f := mustDecode(t, mustEncode(t, w))
	lat, _ := streamByName(f, "latitude")
	lon, _ := streamByName(f, "longitude")
	want := []bool{true, false, false, true}
	if !reflect.DeepEqual(lat.Valid, want) || !reflect.DeepEqual(lon.Valid, want) {
		t.Errorf("valid lat %v lon %v, want both %v", lat.Valid, lon.Valid, want)
	}

	// All positions missing (F-16 AC 5): no position columns at all.
	for i := range w.Samples.Lat {
		w.Samples.Lat[i] = nan
	}
	f = mustDecode(t, mustEncode(t, w))
	for _, n := range streamNames(f) {
		if n == "latitude" || n == "longitude" {
			t.Errorf("descriptor %s written for a workout without positions", n)
		}
	}

	// One side nil: neither is written.
	w = fullWorkout(4)
	w.Samples.Lon = nil
	f = mustDecode(t, mustEncode(t, w))
	if _, ok := streamByName(f, "latitude"); ok {
		t.Error("latitude written without longitude")
	}
}

// ---- values ---------------------------------------------------------------

func TestQuantisation(t *testing.T) {
	spec := func(name string) streamSpec { s, _ := specByName(name); return s }
	tests := []struct {
		stream string
		v      float64
		want   int64
	}{
		{"heart_rate", 120.5, 121},
		{"heart_rate", 120.49, 120},
		{"temperature", -2.5, -3},
		{"temperature", -2.4, -2},
		{"latitude", -33.8688197, -33868820},
		{"speed", 7.999, 7999},
		{"altitude", 420.05, 4201},
		{"distance", 0.04, 0},
	}
	for _, tt := range tests {
		s := spec(tt.stream)
		got, ok := quantise(tt.v, s.factor, s.min, s.max)
		if !ok || got != tt.want {
			t.Errorf("%s %v → %d (%v), want %d", tt.stream, tt.v, got, ok, tt.want)
		}
	}
	for _, v := range []float64{nan, math.Inf(1), math.Inf(-1)} {
		if _, ok := quantise(v, 1, 0, 254); ok {
			t.Errorf("%v is not missing", v)
		}
	}
}

func TestClamping(t *testing.T) {
	spec := func(name string) streamSpec { s, _ := specByName(name); return s }
	tests := []struct {
		stream string
		v      float64
		want   int64
	}{
		{"cadence", 300, 254},
		{"heart_rate", 255, 254},
		{"temperature", -200, -128},
		{"temperature", 130, 126},
		{"temperature", 127, 126},
		{"speed", 70, 65534},
		{"power", -5, 0},
		{"power", 65535, 65534},
		{"altitude", 2e8, 1_000_000_000},
		{"altitude", -2e8, -1_000_000_000},
		{"distance", 1e300, 1_000_000_000},
		{"latitude", 91, 90_000_000},
		{"longitude", -181, -180_000_000},
	}
	for _, tt := range tests {
		s := spec(tt.stream)
		if got, _ := quantise(tt.v, s.factor, s.min, s.max); got != tt.want {
			t.Errorf("%s %v → %d, want %d", tt.stream, tt.v, got, tt.want)
		}
	}
	// A valid value never equals the sentinel, at either end of any range.
	for _, s := range specs {
		for _, v := range []float64{-1e300, 1e300, float64(s.sentinel) * s.scale} {
			if q, _ := quantise(v, s.factor, s.min, s.max); q == s.sentinel {
				t.Errorf("%s: %v quantises to the sentinel", s.name, v)
			}
		}
	}
	// Through the encoder: a clamped cadence of 300 is valid 254.
	w := offsetsOnly(2)
	w.Samples.Cadence = []float64{300, 90}
	f := mustDecode(t, mustEncode(t, w))
	c, _ := streamByName(f, "cadence")
	if !c.Valid[0] || c.Ints[0] != 254 {
		t.Errorf("cadence 300 → %d valid %v", c.Ints[0], c.Valid[0])
	}
}

func TestDelta(t *testing.T) {
	const s = math.MaxInt32
	tests := []struct {
		name  string
		q     []int64
		valid []bool
		want  []int64
	}{
		{"first value absolute", []int64{100, 101, 99}, []bool{true, true, true}, []int64{100, 1, -2}},
		{"missing in the middle", []int64{100, 0, 105}, []bool{true, false, true}, []int64{100, s, 5}},
		{"leading missing", []int64{0, 0, -7, -9}, []bool{false, false, true, true}, []int64{s, s, -7, -2}},
		{"trailing missing", []int64{3, 4, 0}, []bool{true, true, false}, []int64{3, 1, s}},
		{"one valid value", []int64{0, 42, 0}, []bool{false, true, false}, []int64{s, 42, s}},
		{"negative deltas", []int64{-1_000_000_000, 1_000_000_000, -1_000_000_000}, []bool{true, true, true}, []int64{-1_000_000_000, 2_000_000_000, -2_000_000_000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := deltas(tt.q, tt.valid, s)
			if !reflect.DeepEqual(d, tt.want) {
				t.Fatalf("deltas = %v, want %v", d, tt.want)
			}
			for _, x := range d {
				if x != s && (x > math.MaxInt32 || x < math.MinInt32) {
					t.Errorf("delta %d does not fit int32", x)
				}
			}
			got, valid := undelta(d, s)
			if !reflect.DeepEqual(valid, tt.valid) {
				t.Errorf("valid = %v, want %v", valid, tt.valid)
			}
			for i := range got {
				if tt.valid[i] && got[i] != tt.q[i] {
					t.Errorf("value[%d] = %d, want %d", i, got[i], tt.q[i])
				}
			}
		})
	}
}

func TestIndexRanges(t *testing.T) {
	off := []int32{0, 1, 2, 10, 11}
	tests := []struct {
		start, end int32
		from, to   int
	}{
		{0, 2, 0, 3},
		{3, 9, 3, 3},
		{2, 10, 2, 4},
		{12, 20, 5, 5},
		{0, 11, 0, 5},
	}
	for _, tt := range tests {
		if from, to := workout.SampleRange(off, tt.start, tt.end); from != tt.from || to != tt.to {
			t.Errorf("(%d–%d) → [%d, %d), want [%d, %d)", tt.start, tt.end, from, to, tt.from, tt.to)
		}
	}
	// Through the encoder.
	w := workout.Workout{StartTime: testStart}
	w.Samples.Offset = off
	w.Laps = []workout.Lap{{Start: 3, End: 9}}
	f := mustDecode(t, mustEncode(t, w))
	if l := f.Meta.Laps[0]; l.StartIndex != 3 || l.EndIndex != 3 {
		t.Errorf("lap range [%d, %d), want [3, 3)", l.StartIndex, l.EndIndex)
	}
}

// ---- metadata -------------------------------------------------------------

func TestMetadata(t *testing.T) {
	w := offsetsOnly(3601)
	w.Sport = "cycling"
	w.TimeZone = workout.TimeZone{Known: true, OffsetSeconds: 7200}
	w.Summary = workout.Summary{
		Totals:     workout.Totals{ElapsedTime: fp(3600), Distance: fp(27012.5), Ascent: fp(-0.0001), AvgSpeed: fp(7.9994), AvgHeartRate: fp(141)},
		MovingTime: fp(3580),
	}
	w.Sessions = []workout.Session{{Sport: "cycling", SubSport: "road", Start: 0, End: 3600,
		Totals: workout.Totals{ElapsedTime: fp(3600), MaxTemperature: fp(-0.0004), Calories: fp(612.3456)}}}
	w.Samples.HeartRate = make([]float64, 3601)

	js := string(metaBytes(mustEncode(t, w)))
	want := `{"sport":"cycling","start_time":"2026-06-06T05:00:00Z","time_zone":{"known":true,"offset_seconds":7200},` +
		`"summary":{"elapsed_time":3600,"distance":27012.5,"ascent":0,"avg_speed":7.999,"avg_heart_rate":141,"moving_time":3580},` +
		`"device":{"name":"","sensors":[]},"sessions":[{"sport":"cycling","sub_sport":"road","start":0,"end":3600,"start_index":0,"end_index":3601,` +
		`"totals":{"elapsed_time":3600,"calories":612.346,"max_temperature":0}}],` +
		`"laps":[],"streams":[{"name":"time_offset","type":"int32","encoding":"delta","scale":1,"unit":"s","byte_offset":`
	if !strings.HasPrefix(js, want) {
		t.Errorf("metadata =\n%s\nwant prefix\n%s", js, want)
	}
	if !strings.Contains(js, `{"name":"heart_rate","type":"uint8","encoding":"raw","scale":1,"unit":"bpm","byte_offset":`) {
		t.Errorf("heart_rate descriptor missing or malformed:\n%s", js)
	}

	// Unknown time zone, no sessions, empty summary.
	w = offsetsOnly(2)
	w.TimeZone = workout.TimeZone{Known: false, OffsetSeconds: 3600}
	js = string(metaBytes(mustEncode(t, w)))
	want = `{"sport":"cycling","start_time":"2026-06-06T05:00:00Z","time_zone":{"known":false,"offset_seconds":0},"summary":{},"device":{"name":"","sensors":[]},"sessions":[],"laps":[],"streams":[`
	if !strings.HasPrefix(js, want) {
		t.Errorf("metadata =\n%s\nwant prefix\n%s", js, want)
	}
}

func TestDescriptorScales(t *testing.T) {
	js := string(metaBytes(mustEncode(t, fullWorkout(2))))
	for _, want := range []string{`"scale":1e-06`, `"scale":0.1`, `"scale":0.001`, `"scale":1,`} {
		if !strings.Contains(js, want) {
			t.Errorf("metadata lacks %s:\n%s", want, js)
		}
	}
}

// TestMetadataHasNoCoordinates: no key or value of the metadata holds a
// coordinate (D-7, Format §5.6).
func TestMetadataHasNoCoordinates(t *testing.T) {
	w := fullWorkout(5)
	for i := range w.Samples.Lat {
		w.Samples.Lat[i] = 47.6543217
		w.Samples.Lon[i] = 9.8765432
	}
	w.Sessions = []workout.Session{{Sport: "cycling", Start: 0, End: 4, Totals: workout.Totals{Distance: fp(30)}}}
	w.Laps = []workout.Lap{{Start: 0, End: 4}}
	js := string(metaBytes(mustEncode(t, w)))
	for _, digits := range []string{"47.654", "47654", "654321", "9.8765", "98765", "876543"} {
		if strings.Contains(js, digits) {
			t.Errorf("metadata contains %q:\n%s", digits, js)
		}
	}
	// Keys only: "latitude" and "longitude" occur as descriptor names; "device"
	// is the device line's object (§5.7) and no other key starts with it.
	if !strings.Contains(js, `"device":{"name":"","sensors":[]},`) {
		t.Errorf("device object missing or not exactly name and sensors:\n%s", js)
	}
	if k := regexp.MustCompile(`"(lat|lon|position|bbox|bound|device_|timer|start_lat|end_lat)[a-z_]*":`).FindString(js); k != "" {
		t.Errorf("metadata has the key %s", k)
	}
}

func TestTotalFormat(t *testing.T) {
	tests := []struct {
		v    *float64
		want string
	}{
		{nil, ""},
		{fp(3600), "3600"},
		{fp(27012.5), "27012.5"},
		{fp(7.9994), "7.999"},
		{fp(7.9995), "8"},
		{fp(-0.0004), "0"},
		{fp(-1.2345), "-1.235"},
		{fp(nan), ""},
	}
	for _, tt := range tests {
		if got := total(tt.v); string(got) != tt.want {
			t.Errorf("total(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestDeterminism(t *testing.T) {
	w := fullWorkout(500)
	w.Sessions = []workout.Session{{Sport: "cycling", Start: 0, End: 499, Totals: workout.Totals{Distance: fp(3742.5)}}}
	if !bytes.Equal(mustEncode(t, w), mustEncode(t, w)) {
		t.Error("Encode is not deterministic")
	}
}

// ---- errors ---------------------------------------------------------------

func TestEncodeErrors(t *testing.T) {
	tests := []struct {
		name string
		w    func() workout.Workout
		msg  string
	}{
		{"zero samples", func() workout.Workout { return offsetsOnly(0) }, "datafile: no samples"},
		{"stream length", func() workout.Workout {
			w := fullWorkout(3)
			w.Samples.Power = w.Samples.Power[:2]
			return w
		}, "datafile: stream power"},
		{"offsets not increasing", func() workout.Workout {
			w := offsetsOnly(3)
			w.Samples.Offset[2] = 1
			return w
		}, "not strictly increasing"},
		{"offset below 0", func() workout.Workout {
			w := offsetsOnly(3)
			w.Samples.Offset[0] = -1
			return w
		}, "out of range"},
		{"offset above 1e9", func() workout.Workout {
			w := offsetsOnly(3)
			w.Samples.Offset[2] = 1_000_000_001
			return w
		}, "out of range"},
		{"lap end before start", func() workout.Workout {
			w := offsetsOnly(5)
			w.Laps = []workout.Lap{{Start: 0, End: 2}, {Start: 3, End: 1}}
			return w
		}, "datafile: lap 1: end before start"},
		{"session end before start", func() workout.Workout {
			w := offsetsOnly(5)
			w.Sessions = []workout.Session{{Start: 4, End: 3}}
			return w
		}, "datafile: session 0: end before start"},
		{"lap start below 0", func() workout.Workout {
			w := offsetsOnly(5)
			w.Laps = []workout.Lap{{Start: -1, End: 2}}
			return w
		}, "datafile: lap 0: start before 0"},
		{"session start below 0", func() workout.Workout {
			w := offsetsOnly(5)
			w.Sessions = []workout.Session{{Start: -5, End: 2}}
			return w
		}, "datafile: session 0: start before 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Encode(tt.w())
			if err == nil || b != nil {
				t.Fatalf("Encode = %d bytes, %v; want an error", len(b), err)
			}
			if !strings.HasPrefix(err.Error(), "datafile: ") || !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("error %q does not contain %q", err, tt.msg)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	w := fullWorkout(6)
	w.Sport = strings.Repeat("s", 200) // slack for mutateMeta
	w.Laps = []workout.Lap{{Start: 0, End: 3}}
	w.Sessions = []workout.Session{{Sport: "cycling", Start: 0, End: 5}}
	good := mustEncode(t, w)
	mustDecode(t, good)

	set := func(off int, v ...byte) []byte {
		b := bytes.Clone(good)
		copy(b[off:], v)
		return b
	}
	meta := func(f func(m map[string]any)) []byte { return mutateMeta(t, good, f) }
	withN := func(b []byte, n uint32) []byte { binary.LittleEndian.PutUint32(b[6:], n); return b }
	session := func(m map[string]any) map[string]any { return m["sessions"].([]any)[0].(map[string]any) }
	lap := func(m map[string]any) map[string]any { return m["laps"].([]any)[0].(map[string]any) }
	l := int(binary.LittleEndian.Uint32(good[10:]))
	timeAt := align4(14 + l)

	tests := []struct {
		name string
		b    []byte
		msg  string
	}{
		{"13 bytes", good[:13], "shorter than the header"},
		{"one trailing byte", append(bytes.Clone(good), 0), "1 trailing bytes"},
		{"bad magic", set(0, 'W', 'E', 'X', 'X'), "bad magic"},
		{"version 1", set(4, 1, 0), "unsupported version 1"},
		{"N = 0", set(6, 0, 0, 0, 0), "no samples"},
		{"L beyond the end", set(10, 0xFF, 0xFF, 0, 0), "beyond the end"},
		{"invalid JSON", set(14, 'x'), "metadata"},
		{"invalid UTF-8", set(15, 0xFF), "UTF-8"},
		{"missing key", meta(func(m map[string]any) { delete(m, "laps") }), "laps missing"},
		{"wrong type", meta(func(m map[string]any) { m["sport"] = json.Number("5") }), "sport is not a JSON string"},
		{"empty streams", meta(func(m map[string]any) { m["streams"] = []any{} }), "no streams"},
		{"first stream not time_offset", meta(func(m map[string]any) {
			d := descriptors(m)
			d[0], d[1] = d[1], d[0]
		}), "first stream"},
		{"unknown name", meta(func(m map[string]any) { descriptor(m, 3)["name"] = "altitudX" }), "unknown stream"},
		{"repeated name", meta(func(m map[string]any) { descriptor(m, 4)["name"] = "altitude" }), "repeated"},
		{"scale differs", meta(func(m map[string]any) { descriptor(m, 3)["scale"] = json.Number("0.2") }), "descriptor of altitude"},
		{"type differs", meta(func(m map[string]any) { descriptor(m, 6)["type"] = "int8" }), "descriptor of heart_rate"},
		{"misaligned byte_offset", meta(func(m map[string]any) {
			descriptor(m, 1)["byte_offset"] = json.Number(strconv.Itoa(timeAt + 24 + 2))
		}), "not 4-aligned"},
		{"column inside the metadata", meta(func(m map[string]any) {
			descriptor(m, 0)["byte_offset"] = json.Number(strconv.Itoa(timeAt - 4))
		}), "overlaps"},
		{"overlapping columns", meta(func(m map[string]any) {
			descriptor(m, 2)["byte_offset"] = descriptor(m, 1)["byte_offset"]
		}), "overlaps"},
		{"byte_offset 4294967292", meta(func(m map[string]any) {
			descriptor(m, 9)["byte_offset"] = json.Number("4294967292")
		}), "column temperature out of bounds"},
		{"byte_offset near 2^63 and large N", withN(meta(func(m map[string]any) {
			descriptor(m, 0)["byte_offset"] = json.Number("9223372036854775804")
		}), math.MaxUint32), "column time_offset out of bounds"},
		{"byte_offset near 2^63 on a later column", meta(func(m map[string]any) {
			descriptor(m, 5)["byte_offset"] = json.Number("9223372036854775804")
		}), "column speed out of bounds"},
		{"streams out of order", meta(func(m map[string]any) {
			d := descriptors(m)
			d[3], d[4] = d[4], d[3]
		}), "stream altitude out of order"},
		{"latitude without longitude", meta(func(m map[string]any) {
			m["streams"] = append(descriptors(m)[:2:2], descriptors(m)[3:]...)
		}), "both be present"},
		{"longitude without latitude", meta(func(m map[string]any) {
			m["streams"] = append(descriptors(m)[:1:1], descriptors(m)[2:]...)
		}), "both be present"},
		{"half a position", set(timeAt+24+8, 0xFF, 0xFF, 0xFF, 0x7F), "half a position at sample 2"},
		{"session end before start", meta(func(m map[string]any) {
			session(m)["start"], session(m)["end"] = json.Number("4"), json.Number("3")
		}), "session 0: end before start"},
		{"lap end before start", meta(func(m map[string]any) {
			lap(m)["start"], lap(m)["end"] = json.Number("2"), json.Number("1")
		}), "lap 0: end before start"},
		{"session index range reversed", meta(func(m map[string]any) {
			session(m)["start_index"] = json.Number("6")
			session(m)["end_index"] = json.Number("5")
		}), "session 0 index range"},
		{"session index range beyond N", meta(func(m map[string]any) {
			session(m)["end_index"] = json.Number("7")
		}), "session 0 index range"},
		{"column beyond the end", good[:len(good)-1], "column temperature out of bounds"},
		{"missing time sample", set(timeAt+4, 0xFF, 0xFF, 0xFF, 0x7F), "time_offset missing at sample 1"},
		{"time not increasing", set(timeAt+4, 0, 0, 0, 0), "not strictly increasing"},
		{"lap index range beyond N", meta(func(m map[string]any) {
			m["laps"].([]any)[0].(map[string]any)["end_index"] = json.Number("7")
		}), "lap 0 index range"},
		{"lap index range reversed", meta(func(m map[string]any) {
			m["laps"].([]any)[0].(map[string]any)["start_index"] = json.Number("5")
		}), "lap 0 index range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Decode(tt.b)
			if err == nil {
				t.Fatalf("Decode succeeded: %+v", f.Meta)
			}
			if !strings.HasPrefix(err.Error(), "datafile: ") || !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("error %q does not contain %q", err, tt.msg)
			}
		})
	}
}

// FuzzDecode: Decode never panics on any input.
func FuzzDecode(f *testing.F) {
	f.Add(mustEncodeF(workedExample()))
	f.Add([]byte("WEXD\x01\x00\x01\x00\x00\x00\x02\x00\x00\x00{}"))
	// QA-03-r1-1: a 1-sample file whose only descriptor has byte_offset
	// 9223372036854775804; off + N*size overflowed int64 and panicked.
	w := offsetsOnly(1)
	w.Sport = strings.Repeat("s", 40)
	f.Add(mutateMeta(f, mustEncodeF(w), func(m map[string]any) {
		descriptor(m, 0)["byte_offset"] = json.Number("9223372036854775804")
	}))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = Decode(b)
	})
}

func mustEncodeF(w workout.Workout) []byte {
	b, err := Encode(w)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- round trip -----------------------------------------------------------

// TestRoundTripRandom: 50 random workouts; every valid value decodes within
// scale / 2, and missing stays missing (AC 3).
func TestRoundTripRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(2026, 10))
	uniform := func(lo, hi float64) float64 { return lo + rng.Float64()*(hi-lo) }
	for k := range 50 {
		n := 1 + rng.IntN(2000)
		w := workout.Workout{Sport: "running", StartTime: testStart}
		s := &w.Samples
		s.Offset = make([]int32, n)
		var off int32
		for i := range n {
			if i > 0 {
				off++
				if rng.IntN(50) == 0 {
					off += int32(rng.IntN(4000)) // gap
				}
			}
			s.Offset[i] = off
		}
		gen := func(lo, hi float64) []float64 {
			if rng.IntN(4) == 0 {
				return nil // random subset of streams
			}
			v := make([]float64, n)
			for i := range v {
				if rng.IntN(10) == 0 {
					v[i] = nan
				} else {
					v[i] = uniform(lo, hi)
				}
			}
			return v
		}
		s.Lat, s.Lon = gen(-90, 90), gen(-180, 180)
		if s.Lat == nil || s.Lon == nil {
			s.Lat, s.Lon = nil, nil
		} else {
			for i := range n { // F-02 invariant 3: positions are pairs
				if math.IsNaN(s.Lat[i]) || math.IsNaN(s.Lon[i]) {
					s.Lat[i], s.Lon[i] = nan, nan
				}
			}
		}
		s.Altitude, s.Distance = gen(-400, 8848), gen(0, 1e6)
		s.Speed, s.HeartRate, s.Cadence = gen(0, 65.534), gen(0, 254), gen(0, 254)
		s.Power, s.Temperature = gen(0, 65534), gen(-128, 126)

		f := mustDecode(t, mustEncode(t, w))
		if f.SampleCount != n {
			t.Fatalf("workout %d: N = %d, want %d", k, f.SampleCount, n)
		}
		compareStreams(t, w, f)
	}
}

// compareStreams checks every stream of f against w: absent exactly where w
// has no valid value, each valid value within scale / 2 + 1e-9, missing
// exactly where w is NaN.
func compareStreams(t *testing.T, w workout.Workout, f File) {
	t.Helper()
	tm, _ := streamByName(f, "time_offset")
	for i, v := range tm.Values() {
		if v != float64(w.Samples.Offset[i]) {
			t.Fatalf("time_offset[%d] = %v, want %d", i, v, w.Samples.Offset[i])
		}
	}
	for _, spec := range specs[1:] {
		src := spec.samples(&w.Samples)
		st, ok := streamByName(f, spec.name)
		hasValid := false
		for _, x := range src {
			if !math.IsNaN(x) {
				hasValid = true
				break
			}
		}
		if ok != hasValid {
			t.Errorf("%s: written %v, has valid values %v", spec.name, ok, hasValid)
			continue
		}
		if !ok {
			continue
		}
		tol := spec.scale/2 + 1e-9
		for i, v := range st.Values() {
			if math.IsNaN(src[i]) != math.IsNaN(v) {
				t.Errorf("%s[%d] = %v, want %v", spec.name, i, v, src[i])
				return
			}
			if !math.IsNaN(v) && math.Abs(v-src[i]) > tol {
				t.Errorf("%s[%d] = %v, want %v ± %v", spec.name, i, v, src[i], tol)
				return
			}
		}
	}
}

// ---- fixture --------------------------------------------------------------

func digestOf(lines ...string) string {
	h := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return hex.EncodeToString(h[:])[:16]
}

func TestFixtureWorkedExample(t *testing.T) {
	b := mustEncode(t, workedExample())
	got, err := Fixture(mustDecode(t, b))
	if err != nil {
		t.Fatal(err)
	}
	l := int(binary.LittleEndian.Uint32(b[10:]))
	base := align4(14 + l)
	type st struct {
		Name                  string
		ByteOffset            int `json:"byte_offset"`
		Valid, Missing        int
		First, Last, Min, Max int64
		Digest                string
	}
	var fx struct {
		FixtureVersion int `json:"fixture_version"`
		FormatVersion  int `json:"format_version"`
		ByteLength     int `json:"byte_length"`
		SampleCount    int `json:"sample_count"`
		MetadataLength int `json:"metadata_length"`
		Streams        []st
	}
	if err := json.Unmarshal(got, &fx); err != nil {
		t.Fatal(err)
	}
	if fx.FixtureVersion != 1 || fx.FormatVersion != 3 || fx.ByteLength != len(b) || fx.SampleCount != 5 || fx.MetadataLength != l {
		t.Errorf("fixture header = %+v", fx)
	}
	want := []st{
		{"time_offset", base, 5, 0, 0, 11, 0, 11, digestOf("0", "1", "2", "10", "11")},
		{"latitude", base + 20, 4, 1, 47123456, 47124001, 47123456, 47124001, digestOf("47123456", "47123458", "-", "47124000", "47124001")},
		{"longitude", base + 40, 4, 1, 8500000, 8500051, 8500000, 8500051, digestOf("8500000", "8500001", "-", "8500050", "8500051")},
		{"heart_rate", base + 60, 4, 1, 120, 132, 120, 132, digestOf("120", "121", "-", "131", "132")},
		{"temperature", base + 68, 4, 1, -2, -3, -3, -2, digestOf("-2", "-3", "-3", "-", "-3")},
	}
	if !reflect.DeepEqual(fx.Streams, want) {
		t.Errorf("streams =\n%+v\nwant\n%+v", fx.Streams, want)
	}
	if !bytes.HasSuffix(got, []byte("}\n")) || !bytes.Contains(got, []byte("\n  \"streams\"")) {
		t.Errorf("fixture is not indented with 2 spaces and a trailing newline:\n%s", got)
	}
}
