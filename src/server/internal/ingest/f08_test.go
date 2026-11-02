package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/corpustest"
	"workout-explorer/internal/workout"
)

// ---- helpers ----------------------------------------------------------------

func readSynthetic(tb testing.TB, name string) []byte {
	tb.Helper()
	b, err := os.ReadFile(filepath.Join(corpusDir, "synthetic", name))
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func mustParse(t *testing.T, data []byte) workout.Workout {
	t.Helper()
	w, err := testParser.Parse(context.Background(), data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return w
}

func parseErr(t *testing.T, data []byte) *Error {
	t.Helper()
	_, err := testParser.Parse(context.Background(), data)
	var ie *Error
	if !errors.As(err, &ie) {
		t.Fatalf("err = %v, want an *Error", err)
	}
	return ie
}

// lineBuffer collects the JSON log lines.
type lineBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lineBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lineBuffer) raw() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (b *lineBuffer) lines(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range b.raw() {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %v: %s", err, l)
		}
		out = append(out, m)
	}
	return out
}

// frameRE matches "function file:line".
var frameRE = regexp.MustCompile(`^\S+ [^\s/]+:\d+$`)

func activityFile(t *testing.T, records int) []byte {
	t.Helper()
	msgs := []proto.Message{fileID()}
	for i := 0; i < records; i++ {
		msgs = append(msgs, record(i, 100).ToMesg(nil))
	}
	return encodeFIT(t, msgs...)
}

func fileWithType(t *testing.T, ft typedef.File) []byte {
	t.Helper()
	id := fileID()
	for i := range id.Fields {
		if id.Fields[i].Num == 0 { // type
			id.Fields[i].Value = proto.Uint8(uint8(ft))
		}
	}
	return encodeFIT(t, id, record(0, 100).ToMesg(nil), record(1, 100).ToMesg(nil))
}

// ---- kinds, messages, file types ----------------------------------------------

func TestKinds(t *testing.T) {
	headerNoData := make([]byte, 14)
	headerNoData[0] = 14
	copy(headerNoData[8:], ".FIT")
	tooBig := make([]byte, DefaultLimits.MaxBytes+1)

	tests := []struct {
		name string
		data []byte
		kind Kind
		msg  string
	}{
		{"nil", nil, KindNotFIT, "This file is not a FIT file."},
		{"short", []byte("FIT"), KindNotFIT, "This file is not a FIT file."},
		{"png", readSynthetic(t, "not-fit-png.fit"), KindNotFIT, "This file is not a FIT file."},
		{"header size 13", append([]byte{13, 0, 0, 0, 0, 0, 0, 0}, []byte(".FIT\x00\x00\x00")...), KindNotFIT, "This file is not a FIT file."},
		{"data size 0, nothing after", headerNoData, KindNoData, "This file contains no readable workout data."},
		{"header only", readSynthetic(t, "ride-30min-header-only.fit"), KindNoData, "This file contains no readable workout data."},
		{"no records", readSynthetic(t, "no-records.fit"), KindNoData, "This file contains no readable workout data."},
		{"course with records", readSynthetic(t, "course.fit"), KindUnsupportedType, "Unsupported FIT file type."},
		{"monitoring", readSynthetic(t, "monitoring.fit"), KindUnsupportedType, "Unsupported FIT file type."},
		{"too large", tooBig, KindLimit, "This file is too large or too complex to read."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := parseErr(t, tt.data)
			if ie.Kind != tt.kind || ie.Message() != tt.msg {
				t.Errorf("kind %q message %q, want %q %q", ie.Kind, ie.Message(), tt.kind, tt.msg)
			}
			if (ie.Kind == KindLimit) != (ie.Limit != "") {
				t.Errorf("Limit = %q for kind %q", ie.Limit, ie.Kind)
			}
			if !strings.HasPrefix(ie.Error(), "ingest: "+string(tt.kind)) {
				t.Errorf("Error() = %q", ie.Error())
			}
		})
	}

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := testParser.Parse(ctx, readSynthetic(t, "ride-30min.fit"))
		var ie *Error
		if !errors.As(err, &ie) || ie.Kind != KindCanceled || !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want canceled wrapping context.Canceled", err)
		}
		if ie.Message() != "The upload was interrupted." {
			t.Errorf("message = %q", ie.Message())
		}
	})

	t.Run("the exact UP-2 string", func(t *testing.T) {
		for _, name := range []string{"course.fit", "monitoring.fit"} {
			if got := parseErr(t, readSynthetic(t, name)).Message(); got != "Unsupported FIT file type." {
				t.Errorf("%s: %q", name, got)
			}
		}
	})
}

func TestFileTypes(t *testing.T) {
	for v := 0; v < 0xFF; v++ {
		ft := typedef.File(v)
		if ft == typedef.FileActivity {
			continue
		}
		ie := parseErr(t, fileWithType(t, ft))
		want := enumName(ft.String(), uint64(ft), uint64(typedef.FileInvalid))
		if ie.Kind != KindUnsupportedType || ie.FileType != want {
			t.Errorf("type %d: kind %q file type %q, want unsupported_type %q", v, ie.Kind, ie.FileType, want)
		}
	}
	if w := mustParse(t, fileWithType(t, typedef.FileActivity)); w.FileType != "activity" {
		t.Errorf("activity: file type %q", w.FileType)
	}
	// An invalid type is judged by its records (NFR Reliability).
	if w := mustParse(t, fileWithType(t, typedef.FileInvalid)); len(w.Samples.Offset) != 2 {
		t.Errorf("invalid type: %d samples, want 2", len(w.Samples.Offset))
	}
}

// ---- truncation and CRC ---------------------------------------------------------

func streamsOf(s workout.Samples) [][]float64 {
	return [][]float64{s.Lat, s.Lon, s.Altitude, s.Distance, s.Speed, s.HeartRate, s.Cadence, s.Power, s.Temperature}
}

func sameNaN(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.IsNaN(a[i]) != math.IsNaN(b[i]) || (!math.IsNaN(a[i]) && a[i] != b[i]) {
			return false
		}
	}
	return true
}

// TestTruncationPrefix cuts each base at every 1 % of its length from 10 % to
// 99 % and at 20 seeded random positions. An accepted cut has streams that
// equal the base's prefix exactly (cadence included, through the sport
// timeline, D-12) and the sport the base's own hints and sessions give
// (QA-08-r1-1).
func TestTruncationPrefix(t *testing.T) {
	t.Run("ride-30min", func(t *testing.T) {
		truncationPrefix(t, "ride-30min.fit", 1800, false, func(w workout.Workout) string { return "" }) // no sport source in the file
	})
	t.Run("run-30min", func(t *testing.T) {
		truncationPrefix(t, "run-30min.fit", 1800, false, func(w workout.Workout) string { return "running" })
	})
	t.Run("triathlon", func(t *testing.T) {
		truncationPrefix(t, "triathlon.fit", 3720, false, func(w workout.Workout) string {
			if w.Integrity.DataEnd <= 599 { // the last sample lies in the swim
				return "swimming"
			}
			return "multisport"
		})
	})
	// R-13, answer 9 (D-23): a transition is only a session. A cut that loses
	// a transition's session keeps its recorded speed, cadence and power.
	t.Run("ride-transition-ride", func(t *testing.T) {
		truncationPrefix(t, "ride-transition-ride.fit", 1260, true, func(w workout.Workout) string {
			if w.Integrity.DataEnd <= 599 { // the last sample lies in the first ride
				return "cycling"
			}
			return "multisport"
		})
	})
}

// recordedSamples returns the streams of the file as recorded, with the
// transition sessions renamed so that R-13 masks nothing (the reference for
// a cut that loses a transition's session).
func recordedSamples(t *testing.T, data []byte) workout.Samples {
	t.Helper()
	l := newListener(DefaultLimits, func(error) {})
	res := walk(context.Background(), data, l, DefaultLimits)
	rec := l.rec
	rec.Integrity = res.integ
	rec.Sessions = append([]workout.RawSession(nil), rec.Sessions...)
	for i := range rec.Sessions {
		if rec.Sessions[i].Sport == "transition" {
			rec.Sessions[i].Sport = "generic"
		}
	}
	return workout.Normalise(rec).Samples
}

func truncationPrefix(t *testing.T, name string, samples int, fine bool, wantSport func(workout.Workout) string) {
	base := readSynthetic(t, name)
	full := mustParse(t, base)
	if len(full.Samples.Offset) != samples {
		t.Fatalf("base has %d samples", len(full.Samples.Offset))
	}
	recorded := streamsOf(recordedSamples(t, base))
	inTransition := func(off int32, ss []workout.Session) bool {
		for _, s := range ss {
			if s.Start <= off && off <= s.End {
				return s.Sport == "transition" // the first session covering off decides (rule 1)
			}
		}
		return false
	}
	cutInTransition := 0

	check := func(t *testing.T, cut int) int {
		t.Helper()
		w, err := testParser.Parse(context.Background(), base[:cut])
		if err != nil {
			var ie *Error
			if !errors.As(err, &ie) || ie.Kind != KindNoData {
				t.Fatalf("cut %d: err = %v, want no_data", cut, err)
			}
			return 0
		}
		n := len(w.Samples.Offset)
		if !w.Integrity.Truncated || w.Integrity.CRCFailed {
			t.Errorf("cut %d: integrity %+v, want truncated only", cut, w.Integrity)
		}
		if n == 0 || w.Integrity.DataEnd != w.Samples.Offset[n-1] {
			t.Errorf("cut %d: DataEnd %d, last offset %d", cut, w.Integrity.DataEnd, w.Samples.Offset[max(n-1, 0)])
		}
		if !reflect.DeepEqual(w.Samples.Offset, full.Samples.Offset[:n]) {
			t.Errorf("cut %d: offsets are not a prefix", cut)
		}
		if want := wantSport(w); w.Sport != want {
			t.Errorf("cut %d (data end %d): sport %q, want %q", cut, w.Integrity.DataEnd, w.Sport, want)
		}
		got, want := streamsOf(w.Samples), streamsOf(full.Samples)
		if n > 0 && inTransition(w.Samples.Offset[n-1], full.Sessions) {
			cutInTransition++
		}
		for k := range got {
			if got[k] == nil {
				continue // a stream may have no value yet; never a wrong one
			}
			// R-13 / D-23: where the base masks a transition sample but the
			// cut lost that transition's session, the cut keeps the recorded
			// speed, cadence and power. Everything else is the base's prefix.
			exp := append([]float64(nil), want[k][:n]...)
			if k == 4 || k == 6 || k == 7 {
				for i := range exp {
					off := w.Samples.Offset[i]
					if inTransition(off, full.Sessions) && !inTransition(off, w.Sessions) {
						exp[i] = recorded[k][i]
					}
				}
			}
			if !sameNaN(got[k], exp) {
				t.Errorf("cut %d: stream %d is not a prefix of the base stream", cut, k)
			}
		}
		return n
	}

	prev := 0
	for pct := 10; pct <= 99; pct++ {
		n := check(t, len(base)*pct/100)
		if n < prev {
			t.Errorf("%d %%: %d samples, %d at the previous cut", pct, n, prev)
		}
		prev = n
	}
	rng := rand.New(rand.NewPCG(5, 5))
	for i := 0; i < 20; i++ {
		check(t, len(base)/10+rng.IntN(len(base)*9/10-3))
	}
	if fine { // the 1 % steps may miss the transition
		for cut := len(base) / 1000; cut < len(base); cut += len(base) / 1000 {
			check(t, cut)
		}
		if cutInTransition == 0 {
			t.Errorf("no accepted cut ends inside a transition; the D-23 case is not covered")
		}
	}
	// Cuts too early to hold a record.
	for _, cut := range []int{14, 20, 60} {
		if _, err := testParser.Parse(context.Background(), base[:cut]); err == nil {
			t.Errorf("cut %d: want no_data", cut)
		}
	}
}

func TestUnclosedHeader(t *testing.T) {
	cut := mustParse(t, readSynthetic(t, "ride-30min-cut-60.fit"))
	unclosed := mustParse(t, readSynthetic(t, "ride-30min-cut-60-size-0.fit"))
	a, _ := workout.ReportJSON(cut)
	b, _ := workout.ReportJSON(unclosed)
	if !bytes.Equal(a, b) || !unclosed.Integrity.Truncated {
		t.Errorf("unclosed header differs from the cut file (truncated %v)", unclosed.Integrity.Truncated)
	}

	// A complete file with data size 0 is always truncated (R-c).
	whole := append([]byte(nil), readSynthetic(t, "ride-30min.fit")...)
	binary.LittleEndian.PutUint32(whole[4:8], 0)
	w := mustParse(t, whole)
	if !w.Integrity.Truncated || len(w.Samples.Offset) != 1800 || w.Integrity.DataEnd != 1799 {
		t.Errorf("complete file with data size 0: %+v, %d samples", w.Integrity, len(w.Samples.Offset))
	}

	// A 12-byte header works the same way.
	short := append([]byte(nil), whole[:12]...)
	short[0] = 12
	short = append(short, whole[14:]...)
	if w := mustParse(t, short); !w.Integrity.Truncated || len(w.Samples.Offset) != 1800 {
		t.Errorf("12 byte header: %+v, %d samples", w.Integrity, len(w.Samples.Offset))
	}
}

func reportWithout(t *testing.T, name string, drop string) []byte {
	t.Helper()
	b, err := workout.ReportJSON(mustParse(t, readSynthetic(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, drop)
	out, _ := json.Marshal(m) // map keys are sorted: stable
	return out
}

func TestRecoveredEqualsSource(t *testing.T) {
	want := reportWithout(t, "ride-30min.fit", "integrity")
	cases := []struct {
		file string
		want workout.Integrity
	}{
		{"ride-30min-bad-crc.fit", workout.Integrity{CRCFailed: true, Sequences: 1}},
		{"ride-30min-bad-header-crc.fit", workout.Integrity{CRCFailed: true, Sequences: 1}},
		{"ride-30min-no-crc.fit", workout.Integrity{CRCFailed: true, Sequences: 1}},
		{"ride-30min-trailing-zeros.fit", workout.Integrity{Sequences: 1, IgnoredBytes: 64}},
		{"chained-with-settings.fit", workout.Integrity{Sequences: 2, SkippedSequences: 1}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			if got := reportWithout(t, c.file, "integrity"); !bytes.Equal(got, want) {
				t.Errorf("report differs from the base report")
			}
			if got := mustParse(t, readSynthetic(t, c.file)).Integrity; got != c.want {
				t.Errorf("integrity = %+v, want %+v", got, c.want)
			}
		})
	}
}

// ---- chained files -----------------------------------------------------------------

func TestChained(t *testing.T) {
	seq := func(first, n int) []byte {
		msgs := []proto.Message{fileID()}
		for i := 0; i < n; i++ {
			msgs = append(msgs, record(first+i, 100).ToMesg(nil))
		}
		return encodeFIT(t, msgs...)
	}
	noFileID := func(first, n int) []byte {
		var msgs []proto.Message
		for i := 0; i < n; i++ {
			msgs = append(msgs, record(first+i, 100).ToMesg(nil))
		}
		return encodeFIT(t, msgs...)
	}
	other := func(ft typedef.File, first, n int) []byte {
		id := fileID()
		for i := range id.Fields {
			if id.Fields[i].Num == 0 {
				id.Fields[i].Value = proto.Uint8(uint8(ft))
			}
		}
		msgs := []proto.Message{id}
		for i := 0; i < n; i++ {
			msgs = append(msgs, record(first+i, 100).ToMesg(nil))
		}
		return encodeFIT(t, msgs...)
	}
	cat := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

	t.Run("two and three sequences", func(t *testing.T) {
		w := mustParse(t, cat(seq(0, 10), seq(100, 10)))
		if w.Integrity.Sequences != 2 || len(w.Samples.Offset) != 20 {
			t.Errorf("two: %+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
		w = mustParse(t, cat(seq(0, 10), seq(100, 10), seq(200, 10)))
		if w.Integrity.Sequences != 3 || len(w.Samples.Offset) != 30 {
			t.Errorf("three: %+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
	})
	t.Run("a later sequence without file_id is used", func(t *testing.T) {
		w := mustParse(t, cat(seq(0, 10), noFileID(100, 10)))
		if w.Integrity.Sequences != 2 || w.Integrity.SkippedSequences != 0 || len(w.Samples.Offset) != 20 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
	})
	t.Run("later settings and course sequences are skipped", func(t *testing.T) {
		w := mustParse(t, cat(seq(0, 10), other(typedef.FileSettings, 100, 10), other(typedef.FileCourse, 200, 10), seq(300, 10)))
		if w.Integrity.Sequences != 4 || w.Integrity.SkippedSequences != 2 || len(w.Samples.Offset) != 20 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
		if w.FileType != "activity" || w.Device.Manufacturer != "garmin" {
			t.Errorf("file type %q device %+v: the first file_id decides (E-4)", w.FileType, w.Device)
		}
	})
	t.Run("a broken header after a complete sequence is ignored bytes", func(t *testing.T) {
		junk := append([]byte{14, 0x20, 0, 0, 1, 0, 0, 0}, []byte("NOPE\x00\x00")...)
		w := mustParse(t, cat(seq(0, 10), junk))
		if w.Integrity.Sequences != 1 || w.Integrity.IgnoredBytes != len(junk) || len(w.Samples.Offset) != 10 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
	})
	t.Run("a carried distance count with boundary smoothing is not offset (R-12 midpoint, QA-08-r2-1)", func(t *testing.T) {
		// QA's probe shape: two cycling sequences, 600 records each at 5 m/s,
		// each with a session; the second continues the count 1 cm below the
		// first's last value (design v1.2 row 3).
		dseq := func(first int, d0 float64) []byte {
			msgs := []proto.Message{fileID()}
			for i := 0; i < 600; i++ {
				msgs = append(msgs, record(first+i, 100).SetDistance(uint32(math.Round((d0+5*float64(i))*100))).ToMesg(nil))
			}
			msgs = append(msgs, mesgdef.NewSession(nil).SetStartTime(at(first)).SetTimestamp(at(first+599)).
				SetSport(typedef.SportCycling).SetTotalElapsedTime(599_000).SetTotalTimerTime(599_000).
				SetTotalDistance(299_500).ToMesg(nil))
			return encodeFIT(t, msgs...)
		}
		w := mustParse(t, cat(dseq(0, 0), dseq(600, 2994.99)))
		if w.Derived.Distance == nil || math.Abs(*w.Derived.Distance-5990) > 0.05 {
			t.Errorf("derived distance = %v, want 5,990 +- 0.05 (not offset by the first's 2,995)", deref(w.Derived.Distance))
		}
		if len(w.Sessions) != 2 {
			t.Errorf("%d sessions, want 2", len(w.Sessions))
		}
		if w.Summary.AvgSpeed == nil || math.Abs(*w.Summary.AvgSpeed-5.0) > 0.01 {
			t.Errorf("summary avg speed = %v, want 5.0 +- 0.01", deref(w.Summary.AvgSpeed))
		}
		for i := 1; i < len(w.Samples.Distance); i++ {
			if step := w.Samples.Distance[i] - w.Samples.Distance[i-1]; step < 0 { // R-14 (v1.4): the 1 cm carry-over step is held
				t.Errorf("distance step %d is %v m, want no negative step (R-14)", i, step)
				break
			}
		}
	})
	t.Run("a CRC failure in sequence 1 does not stop sequence 2", func(t *testing.T) {
		first := seq(0, 10)
		first[len(first)-1] ^= 0xFF
		w := mustParse(t, cat(first, seq(100, 10)))
		if !w.Integrity.CRCFailed || w.Integrity.Sequences != 2 || len(w.Samples.Offset) != 20 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
	})
	t.Run("truncation in sequence 2 keeps sequence 1", func(t *testing.T) {
		second := seq(100, 10)
		w := mustParse(t, cat(seq(0, 10), second[:len(second)-12]))
		if !w.Integrity.Truncated || w.Integrity.Sequences != 2 || len(w.Samples.Offset) < 11 {
			t.Errorf("%+v, %d samples", w.Integrity, len(w.Samples.Offset))
		}
	})
	t.Run("corpus: chained ride and run, triathlon", func(t *testing.T) {
		w := mustParse(t, readSynthetic(t, "chained-ride-run.fit"))
		if w.Integrity.Sequences != 2 || w.Sport != "multisport" || len(w.Sessions) != 2 {
			t.Errorf("chained: %+v sport %q sessions %d", w.Integrity, w.Sport, len(w.Sessions))
		}
		w = mustParse(t, readSynthetic(t, "triathlon.fit"))
		if w.Integrity.Sequences != 1 || w.Sport != "multisport" || len(w.Sessions) != 5 {
			t.Errorf("triathlon: %+v sport %q sessions %d", w.Integrity, w.Sport, len(w.Sessions))
		}
	})
}

// ---- the log ----------------------------------------------------------------------

var logKeys = []string{"outcome", "kind", "limit", "file_type", "manufacturer", "product", "product_id", "product_name",
	"bytes", "sequences", "skipped_sequences", "ignored_bytes", "truncated", "crc_failed", "data_end_s", "records",
	"duration_ms", "cause", "panic", "frames"}

// positions returns every position of data as the strings a leak could show:
// the semicircle integer and the degrees with 4 to 7 decimals.
func positions(t *testing.T, data []byte) []string {
	t.Helper()
	var out []string
	dec := decoder.New(bytes.NewReader(data), decoder.WithIgnoreChecksum())
	fit, err := dec.Decode()
	if err != nil || fit == nil {
		return nil // not decodable: positions of a truncated file are covered by its base
	}
	for _, m := range fit.Messages {
		for _, f := range m.Fields {
			if f.Value.Type() != proto.TypeInt32 || !strings.Contains(f.Name, "position_") {
				continue
			}
			v := f.Value.Int32()
			if v == math.MaxInt32 {
				continue
			}
			out = append(out, strconv.Itoa(int(v)))
			deg := float64(v) * 180 / (1 << 31)
			for d := 4; d <= 7; d++ {
				out = append(out, strconv.FormatFloat(deg, 'f', d, 64))
			}
		}
	}
	return out
}

func TestParseLogs(t *testing.T) {
	var logs lineBuffer
	p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	handler := slog.New(slog.NewJSONHandler(&logs, nil))
	small := NewParser(Limits{MaxRecords: 100, MaxLaps: 5, MaxSessions: 2, MaxTimerEvents: 5, MaxFieldDescriptions: 5,
		MaxSequences: 3, MaxSportMessages: 5}, handler)
	few := NewParser(Limits{MaxSamples: 1000}, handler)
	slow := NewParser(Limits{Timeout: 100 * time.Millisecond}, handler)
	panicky := NewParser(DefaultLimits, handler)
	calls := 0
	panicky.hook = func() {
		if calls++; calls == 5 { // after file_id
			panic("boom")
		}
	}
	fd := &fitBuilder{}
	fd.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
	fd.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch)...)
	floodFieldDescriptions(fd, 6)

	type input struct {
		name string
		data []byte
		p    *Parser
		ctx  context.Context
	}
	var inputs []input
	for _, path := range corpustest.Files(t, corpusDir, "synthetic") {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input{name: filepath.Base(path), data: b, p: p})
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	inputs = append(inputs,
		input{"record limit", floodRecords(500), small, nil},
		input{"lap limit", floodMesgs(mesgnum.Lap, 6), small, nil},
		input{"session limit", floodMesgs(mesgnum.Session, 3), small, nil},
		input{"timer event limit", floodTimerEvents(6), small, nil},
		input{"field description limit", fd.bytes(), small, nil},
		input{"sequence limit", floodSequences(4), small, nil},
		input{"sport message limit", floodMesgs(mesgnum.Sport, 6), small, nil},
		input{"samples limit", readSynthetic(t, "ride-30min.fit"), few, nil},
		input{"time limit", floodDeveloperFields(8 << 20), slow, nil},
		input{"panic", readSynthetic(t, "ride-30min.fit"), panicky, nil},
		input{"too large", make([]byte, DefaultLimits.MaxBytes+1), p, nil},
		input{"canceled", readSynthetic(t, "ride-30min.fit"), p, canceled},
	)

	var positionStrings []string
	for _, in := range inputs {
		positionStrings = append(positionStrings, positions(t, in.data)...)
	}
	if len(positionStrings) < 1000 {
		t.Fatalf("only %d position strings to scan for: the sources are wrong", len(positionStrings))
	}

	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			before := len(logs.raw())
			ctx := in.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			w, err := in.p.Parse(ctx, in.data)
			lines := logs.raw()[before:]
			var ie *Error
			failed := errors.As(err, &ie)
			recovered := !failed && (w.Integrity.Truncated || w.Integrity.CRCFailed || w.Integrity.IgnoredBytes > 0 || w.Integrity.SkippedSequences > 0)
			want := 0
			if failed || recovered {
				want = 1
			}
			if len(lines) != want {
				t.Fatalf("%d log lines, want %d: %v", len(lines), want, lines)
			}
			if want == 0 {
				return
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
				t.Fatal(err)
			}
			allowed := map[string]bool{"time": true, "level": true, "msg": true}
			for _, k := range logKeys {
				allowed[k] = true
			}
			for k := range m {
				if !allowed[k] {
					t.Errorf("key %q is not in the allow-list", k)
				}
			}
			for _, k := range []string{"outcome", "file_type", "manufacturer", "product", "product_id", "product_name", "bytes",
				"sequences", "skipped_sequences", "ignored_bytes", "truncated", "crc_failed", "records", "duration_ms"} {
				if _, ok := m[k]; !ok {
					t.Errorf("key %q missing: %s", k, lines[0])
				}
			}
			// Device make and model (AC 9).
			dev, ftype := w.Device, w.FileType
			if failed {
				dev, ftype = ie.Device, ie.FileType
			}
			if m["manufacturer"] != dev.Manufacturer || m["product_name"] != dev.ProductName ||
				m["product_id"] != float64(dev.ProductID) || m["file_type"] != ftype {
				t.Errorf("device keys %v/%v/%v/%v, want %+v type %q", m["manufacturer"], m["product_id"], m["product_name"], m["file_type"], dev, ftype)
			}
			if failed && m["kind"] != string(ie.Kind) {
				t.Errorf("kind = %v, want %s", m["kind"], ie.Kind)
			}
			if in.name == "samples limit" {
				// QA-08-r3-1 (M21): the file's own device keys, not an empty pair.
				if !failed || ie.Kind != KindLimit || m["limit"] != limitSamples || m["level"] != "WARN" || dev.Manufacturer == "" {
					t.Errorf("samples limit: failed %v, line %s, device %+v", failed, lines[0], dev)
				}
			}
			if recovered {
				if m["msg"] != "fit recovered" || m["level"] != "INFO" || m["outcome"] != "recovered" {
					t.Errorf("line = %s", lines[0])
				}
				if _, ok := m["data_end_s"]; ok != w.Integrity.Truncated {
					t.Errorf("data_end_s present = %v, truncated = %v", ok, w.Integrity.Truncated)
				}
			}
			// No position, no coordinate vocabulary. slog's own time stamp is
			// not ours and can hold digits that look like a coordinate.
			delete(m, "time")
			line, _ := json.Marshal(m)
			for _, s := range positionStrings {
				if strings.Contains(string(line), s) {
					t.Errorf("log line contains the position value %q: %s", s, line)
				}
			}
			for _, sub := range []string{"lat", "lon", "position"} {
				if strings.Contains(strings.ToLower(string(line)), sub) {
					t.Errorf("log line contains %q: %s", sub, line)
				}
			}
		})
	}

	t.Run("clean files write nothing", func(t *testing.T) {
		before := len(logs.raw())
		for _, name := range []string{"ride-30min.fit", "chained-ride-run.fit", "triathlon.fit"} {
			if _, err := p.Parse(context.Background(), readSynthetic(t, name)); err != nil {
				t.Fatal(err)
			}
		}
		if n := len(logs.raw()) - before; n != 0 {
			t.Errorf("%d log lines for clean files", n)
		}
	})
	t.Run("the device keys name a Garmin product", func(t *testing.T) {
		id := fileID() // garmin, product 3122
		data := encodeFIT(t, id)
		before := len(logs.raw())
		if _, err := p.Parse(context.Background(), data); err == nil {
			t.Fatal("want no_data")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(logs.raw()[before]), &m); err != nil {
			t.Fatal(err)
		}
		if m["manufacturer"] != "garmin" || m["product"] == "" || m["product_name"] != "Edge Test" || m["product_id"] != float64(3122) {
			t.Errorf("line = %v", m)
		}
		if strings.Contains(logs.raw()[before], strconv.Itoa(serial)) {
			t.Errorf("serial number in the log line")
		}
	})
}

func TestCutRunes(t *testing.T) {
	long := strings.Repeat("ä", 40) // 80 bytes
	got := cutRunes(long, maxProductName)
	if len(got) > maxProductName || !strings.HasPrefix(long, got) || len(got)%2 != 0 {
		t.Errorf("cut to %d bytes: %q", len(got), got)
	}
	if cutRunes("abc", 64) != "abc" {
		t.Errorf("short string changed")
	}
}

// ---- fuzzing ------------------------------------------------------------------------

// FuzzParse: for any input there is no internal error, the call returns in
// time, and an accepted workout passes the invariants. Seeds are the
// synthetic files, never real/ (ADR-004 item 8). `go test` runs the seeds;
// the fuzz run is manual: go test ./internal/ingest -run '^$' -fuzz FuzzParse -fuzztime 10m
func FuzzParse(f *testing.F) {
	for _, path := range corpustest.Files(f, corpusDir, "synthetic") {
		b, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	lim := Limits{Timeout: time.Second}
	p := NewParser(lim, nil)
	f.Fuzz(func(t *testing.T, data []byte) {
		start := time.Now()
		w, err := p.Parse(context.Background(), data)
		if d := time.Since(start); d > lim.Timeout+2*time.Second {
			t.Fatalf("took %v", d)
		}
		if err != nil {
			var ie *Error
			if !errors.As(err, &ie) {
				t.Fatalf("err = %v, not an *Error", err)
			}
			if ie.Kind == KindInternal {
				t.Fatalf("internal error: %v", err)
			}
			if ie.Message() == "" {
				t.Fatalf("no message for %v", ie.Kind)
			}
			return
		}
		checkInvariants(t, w)
		if n := len(w.Samples.Offset); w.Integrity.Truncated && n > 0 && w.Integrity.DataEnd != w.Samples.Offset[n-1] {
			t.Fatalf("DataEnd %d, last offset %d", w.Integrity.DataEnd, w.Samples.Offset[n-1])
		}
		if !w.Integrity.Truncated && w.Integrity.DataEnd != 0 {
			t.Fatalf("DataEnd %d without truncation", w.Integrity.DataEnd)
		}
	})
}
