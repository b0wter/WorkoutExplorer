package datafile

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"
	"time"

	"workout-explorer/internal/workout"
)

var nan = math.NaN()

func fp(v float64) *float64 { return &v }

var testStart = time.Date(2026, time.June, 6, 5, 0, 0, 0, time.UTC)

// fullWorkout has n samples at 1 Hz with every stream present.
func fullWorkout(n int) workout.Workout {
	w := workout.Workout{Sport: "cycling", StartTime: testStart, TimeZone: workout.TimeZone{Known: true, OffsetSeconds: 7200}}
	s := &w.Samples
	s.Offset = make([]int32, n)
	for _, p := range []*[]float64{&s.Lat, &s.Lon, &s.Altitude, &s.Distance, &s.Speed, &s.HeartRate, &s.Cadence, &s.Power, &s.Temperature} {
		*p = make([]float64, n)
	}
	for i := range n {
		s.Offset[i] = int32(i)
		s.Lat[i] = 47.05 + float64(i)*1e-5
		s.Lon[i] = 8.3 + float64(i)*2e-5
		s.Altitude[i] = 420 + float64(i%50)/10
		s.Distance[i] = float64(i) * 7.5
		s.Speed[i] = 7.5
		s.HeartRate[i] = 140
		s.Cadence[i] = 88
		s.Power[i] = 210
		s.Temperature[i] = 18
	}
	return w
}

// offsetsOnly has n samples at 1 Hz and no stream besides time.
func offsetsOnly(n int) workout.Workout {
	w := workout.Workout{Sport: "cycling", StartTime: testStart}
	w.Samples.Offset = make([]int32, n)
	for i := range n {
		w.Samples.Offset[i] = int32(i)
	}
	return w
}

func mustEncode(t *testing.T, w workout.Workout) []byte {
	t.Helper()
	b, err := Encode(w)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return b
}

func mustDecode(t *testing.T, b []byte) File {
	t.Helper()
	f, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return f
}

func streamNames(f File) []string {
	var names []string
	for _, s := range f.Streams {
		names = append(names, s.Descriptor.Name)
	}
	return names
}

func streamByName(f File, name string) (Stream, bool) {
	for _, s := range f.Streams {
		if s.Descriptor.Name == name {
			return s, true
		}
	}
	return Stream{}, false
}

// metaBytes returns the metadata JSON of an encoded file.
func metaBytes(b []byte) []byte {
	l := binary.LittleEndian.Uint32(b[10:])
	return b[headerSize : headerSize+l]
}

// mutateMeta rewrites the metadata JSON of b through f. The new JSON must not
// be longer than the old one (a long "sport" string is shortened to make
// room); it is padded with spaces (valid JSON
// whitespace) so the columns stay where they are.
func mutateMeta(t testing.TB, b []byte, f func(m map[string]any)) []byte {
	t.Helper()
	old := metaBytes(b)
	d := json.NewDecoder(bytes.NewReader(old))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	f(m)
	js, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// Use the base workout's long sport string as slack.
	if sport, ok := m["sport"].(string); ok && len(js) > len(old) && len(sport) >= len(js)-len(old) {
		m["sport"] = sport[:len(sport)-(len(js)-len(old))]
		if js, err = json.Marshal(m); err != nil {
			t.Fatal(err)
		}
	}
	if len(js) > len(old) {
		t.Fatalf("mutated metadata is %d bytes longer; give the base workout more slack", len(js)-len(old))
	}
	out := bytes.Clone(b)
	copy(out[headerSize:], js)
	for i := headerSize + len(js); i < headerSize+len(old); i++ {
		out[i] = ' '
	}
	return out
}

// descriptors returns the "streams" array of a mutable metadata map.
func descriptors(m map[string]any) []any { return m["streams"].([]any) }

func descriptor(m map[string]any, i int) map[string]any {
	return descriptors(m)[i].(map[string]any)
}
