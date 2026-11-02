package ingest

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"

	"workout-explorer/internal/workout"
)

// Hostile files for the limits are built in memory and never committed: some
// would be megabytes, and their outcome depends on the limits the test
// passes (F-08 design, Corpus and Tests).

var crcTable = [16]uint16{0x0000, 0xCC01, 0xD801, 0x1400, 0xF001, 0x3C00, 0x2800, 0xE401, 0xA001, 0x6C00, 0x7800, 0xB401, 0x5000, 0x9C01, 0x8801, 0x4400}

// fitCRC is the FIT CRC-16.
func fitCRC(crc uint16, b []byte) uint16 {
	for _, x := range b {
		tmp := crcTable[crc&0xF]
		crc = (crc >> 4) & 0x0FFF
		crc = crc ^ tmp ^ crcTable[x&0xF]
		tmp = crcTable[crc&0xF]
		crc = (crc >> 4) & 0x0FFF
		crc = crc ^ tmp ^ crcTable[(x>>4)&0xF]
	}
	return crc
}

// fitBuilder writes the bytes of one FIT sequence by hand.
type fitBuilder struct{ body []byte }

type fieldDef struct {
	num, size byte
	base      basetype.BaseType
}

// def appends a message definition; devFields are (num, size, developer data
// index) triples.
func (b *fitBuilder) def(local byte, num typedef.MesgNum, fields []fieldDef, devFields ...[3]byte) {
	h := 0x40 | local
	if len(devFields) > 0 {
		h |= 0x20
	}
	b.body = append(b.body, h, 0, 0, byte(num), byte(num>>8), byte(len(fields)))
	for _, f := range fields {
		b.body = append(b.body, f.num, f.size, byte(f.base))
	}
	if len(devFields) > 0 {
		b.body = append(b.body, byte(len(devFields)))
		for _, d := range devFields {
			b.body = append(b.body, d[0], d[1], d[2])
		}
	}
}

// data appends a normal data message.
func (b *fitBuilder) data(local byte, payload ...byte) {
	b.body = append(b.body, local)
	b.body = append(b.body, payload...)
}

// compressed appends a data message with a compressed timestamp header; the
// time offset is taken modulo 32.
func (b *fitBuilder) compressed(local byte, offset int, payload ...byte) {
	b.body = append(b.body, 0x80|local<<5|byte(offset&0x1F))
	b.body = append(b.body, payload...)
}

// bytes returns header, messages and a correct CRC.
func (b *fitBuilder) bytes() []byte {
	h := make([]byte, 14)
	h[0], h[1] = 14, 0x20
	binary.LittleEndian.PutUint16(h[2:], 21158)
	binary.LittleEndian.PutUint32(h[4:], uint32(len(b.body)))
	copy(h[8:], ".FIT")
	binary.LittleEndian.PutUint16(h[12:], fitCRC(0, h[:12]))
	out := append(h, b.body...)
	return binary.LittleEndian.AppendUint16(out, fitCRC(0, b.body))
}

// fitEpoch is a FIT timestamp in 2026 whose low 5 bits are 0.
const fitEpoch = 1_150_000_000 &^ 0x1F

// startFlood writes a record definition with a timestamp (local 1), one full
// record, and a definition of mesgNum without fields (local 0), the base of
// the one-byte floods: the following compressed-timestamp messages carry a
// time offset and nothing else, so each costs the file one byte.
func startFlood(mesgNum typedef.MesgNum, fields []fieldDef, payload ...byte) *fitBuilder {
	b := &fitBuilder{}
	b.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
	b.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch)...)
	b.def(0, mesgNum, fields)
	return b
}

// floodRecords: n one-byte records, 1 s apart.
func floodRecords(n int) []byte {
	b := startFlood(mesgnum.Record, nil)
	for i := 1; i <= n; i++ {
		b.compressed(0, i)
	}
	return b.bytes()
}

// floodRecordsEvery: n one-byte records, step seconds apart (step <= 31).
func floodRecordsEvery(n, step int) []byte {
	b := startFlood(mesgnum.Record, nil)
	for i := 1; i <= n; i++ {
		b.compressed(0, i*step)
	}
	return b.bytes()
}

// floodMesgs: one valid record, then n one-byte messages of mesgNum.
func floodMesgs(mesgNum typedef.MesgNum, n int) []byte {
	b := startFlood(mesgNum, nil)
	for i := 1; i <= n; i++ {
		b.compressed(0, i)
	}
	return b.bytes()
}

// floodTimerEvents: one valid record, then n two-byte event messages with
// event = timer (0).
func floodTimerEvents(n int) []byte {
	b := startFlood(mesgnum.Event, []fieldDef{{0, 1, basetype.Enum}})
	for i := 1; i <= n; i++ {
		b.compressed(0, i, 0)
	}
	return b.bytes()
}

// floodFieldDescriptions: n field_description messages (index 0, field
// numbers 0 to 255, base type uint8) after a valid record.
func floodFieldDescriptions(b *fitBuilder, n int) {
	b.def(2, mesgnum.FieldDescription, []fieldDef{
		{0, 1, basetype.Uint8}, // developer_data_index
		{1, 1, basetype.Uint8}, // field_definition_number
		{2, 1, basetype.Uint8}, // fit_base_type_id
	})
	for i := 0; i < n; i++ {
		b.data(2, 0, byte(i), 2)
	}
}

// floodSequences: n minimal sequences, each with one record.
func floodSequences(n int) []byte {
	var out []byte
	for i := 0; i < n; i++ {
		b := &fitBuilder{}
		b.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
		b.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch+uint32(i))...)
		out = append(out, b.bytes()...)
	}
	return out
}

// floodDeveloperFields is the CPU attack: 256 field descriptions of
// developer data index 0, a record definition with 255 one-byte developer
// fields of index 1 that no description matches (each lookup scans every
// description), then data messages up to size bytes.
func floodDeveloperFields(size int) []byte {
	b := &fitBuilder{}
	floodFieldDescriptions(b, 256)
	devs := make([][3]byte, 255)
	for i := range devs {
		devs[i] = [3]byte{byte(i), 1, 1}
	}
	b.def(0, mesgnum.Record, nil, devs...)
	for len(b.body) < size {
		b.data(0, make([]byte, 255)...)
	}
	return b.bytes()
}

func newTestParser(lim Limits) *Parser { return NewParser(lim, nil) }

// TestLimits: one hostile file per budget, with the production limits. Each
// is stopped with the right limit name within the timeout, and the memory
// allocated stays far below what the unbounded parse would use.
func TestLimits(t *testing.T) {
	big := floodRecords(20<<20 - 64)
	if len(big) > DefaultLimits.MaxBytes {
		t.Fatalf("test file is %d bytes, over MaxBytes", len(big))
	}
	fd := &fitBuilder{}
	fd.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
	fd.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch)...)
	floodFieldDescriptions(fd, 257)

	tests := []struct {
		name  string
		data  []byte
		limit string
	}{
		{"record flood", big, limitRecords},
		{"lap flood", floodMesgs(mesgnum.Lap, 10_001), limitLaps},
		{"session flood", floodMesgs(mesgnum.Session, 101), limitSessions},
		{"timer event flood", floodTimerEvents(100_001), limitTimerEvents},
		{"field description flood", fd.bytes(), limitFieldDescriptions},
		{"sport message flood", floodMesgs(mesgnum.Sport, 1_001), limitSportMessages},
		{"sequence flood", floodSequences(65), limitSequences},
		{"sample flood", floodRecordsEvery(150_000, 4), limitSamples},
		{"too large", make([]byte, DefaultLimits.MaxBytes+1), limitBytes},
	}
	p := newTestParser(DefaultLimits)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			_, err := p.Parse(context.Background(), tt.data)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)

			var ie *Error
			if !errors.As(err, &ie) || ie.Kind != KindLimit || ie.Limit != tt.limit {
				t.Fatalf("err = %v, want limit (%s)", err, tt.limit)
			}
			if ie.Message() != messages[KindLimit] {
				t.Errorf("message = %q", ie.Message())
			}
			if elapsed > DefaultLimits.Timeout {
				t.Errorf("took %v, more than the timeout", elapsed)
			}
			if grown := after.TotalAlloc - before.TotalAlloc; grown > 1<<30 {
				t.Errorf("allocated %d MiB, want at most 1024", grown>>20)
			}
			t.Logf("%s: %v, %d MiB allocated", tt.limit, elapsed.Round(time.Millisecond), (after.TotalAlloc-before.TotalAlloc)>>20)
		})
	}
}

// TestLimitsAtTheCap: a file exactly at a budget is accepted, one over it
// is stopped with that budget's name (QA-08-r1-6 b). Every budget the
// parser has is covered except the time limit (TestTimeLimit).
func TestLimitsAtTheCap(t *testing.T) {
	lim := Limits{MaxRecords: 100, MaxSamples: 101, MaxLaps: 5, MaxSessions: 2, MaxTimerEvents: 5,
		MaxFieldDescriptions: 5, MaxSequences: 3, MaxSportMessages: 5}
	fieldDescs := func(n int) []byte {
		b := &fitBuilder{}
		b.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
		b.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch)...)
		floodFieldDescriptions(b, n)
		return b.bytes()
	}
	tests := []struct {
		name        string
		atCap, over []byte
		limit       string
	}{
		{"records", floodRecords(99), floodRecords(100), limitRecords}, // 1 full + n
		{"samples", floodRecordsEvery(25, 4), floodRecordsEvery(26, 4), limitSamples},
		{"laps", floodMesgs(mesgnum.Lap, 5), floodMesgs(mesgnum.Lap, 6), limitLaps},
		{"sessions", floodMesgs(mesgnum.Session, 2), floodMesgs(mesgnum.Session, 3), limitSessions},
		{"timer events", floodTimerEvents(5), floodTimerEvents(6), limitTimerEvents},
		{"field descriptions", fieldDescs(5), fieldDescs(6), limitFieldDescriptions},
		{"sequences", floodSequences(3), floodSequences(4), limitSequences},
		{"sport messages", floodMesgs(mesgnum.Sport, 5), floodMesgs(mesgnum.Sport, 6), limitSportMessages},
	}
	p := newTestParser(lim)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := p.Parse(context.Background(), tt.atCap); err != nil {
				t.Errorf("at the cap: %v", err)
			}
			_, err := p.Parse(context.Background(), tt.over)
			var ie *Error
			if !errors.As(err, &ie) || ie.Kind != KindLimit || ie.Limit != tt.limit {
				t.Errorf("one over: err = %v, want limit (%s)", err, tt.limit)
			}
		})
	}
	t.Run("bytes", func(t *testing.T) {
		data := floodRecords(10)
		if _, err := newTestParser(Limits{MaxBytes: len(data)}).Parse(context.Background(), data); err != nil {
			t.Errorf("at the cap: %v", err)
		}
		_, err := newTestParser(Limits{MaxBytes: len(data) - 1}).Parse(context.Background(), data)
		var ie *Error
		if !errors.As(err, &ie) || ie.Limit != limitBytes {
			t.Errorf("one over: err = %v, want limit (bytes)", err)
		}
	})
}

// TestTimeLimit: the developer-field flood is a loop in the decoder that no
// budget counts; the deadline stops it.
func TestTimeLimit(t *testing.T) {
	p := newTestParser(Limits{Timeout: 100 * time.Millisecond})
	start := time.Now()
	_, err := p.Parse(context.Background(), floodDeveloperFields(8<<20))
	var ie *Error
	if !errors.As(err, &ie) || ie.Kind != KindLimit || ie.Limit != limitTime {
		t.Fatalf("err = %v, want limit (time)", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("took %v, want under 1 s", d)
	}
}

// TestHostileDoesNotBlockOthers: while a hostile file is being parsed, a
// normal file in another goroutine is parsed quickly; afterwards no
// goroutine is left behind (F-08 AC 8; the HTTP-level check is F-09's).
func TestHostileDoesNotBlockOthers(t *testing.T) {
	hostile := floodDeveloperFields(20<<20 - 1024)
	normal := readSynthetic(t, "ride-30min.fit")
	p := newTestParser(Limits{Timeout: time.Second})
	goroutines := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var hostileErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, hostileErr = p.Parse(context.Background(), hostile)
	}()
	time.Sleep(50 * time.Millisecond) // let the flood start

	start := time.Now()
	w, err := p.Parse(context.Background(), normal)
	if err != nil || len(w.Samples.Offset) != 1800 {
		t.Fatalf("normal file: %d samples, err %v", len(w.Samples.Offset), err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("normal file took %v while the flood ran, want under 1 s", d)
	}
	wg.Wait()
	var ie *Error
	if !errors.As(hostileErr, &ie) || ie.Limit != limitTime {
		t.Errorf("flood result = %v, want limit (time)", hostileErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > goroutines && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > goroutines {
		t.Errorf("goroutines = %d, started with %d", n, goroutines)
	}
}

// TestSlots: with one slot held, a second parse waits and then gives up with
// canceled when its context ends.
func TestSlots(t *testing.T) {
	normal := readSynthetic(t, "ride-30min.fit")
	p := newTestParser(Limits{MaxConcurrent: 1})
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	p.hook = func() {
		once.Do(func() { close(entered) })
		<-release
	}
	done := make(chan error, 1)
	go func() {
		_, err := p.Parse(context.Background(), normal)
		done <- err
	}()
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Parse(ctx, normal)
	var ie *Error
	if !errors.As(err, &ie) || ie.Kind != KindCanceled || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("second parse: err = %v, want canceled wrapping DeadlineExceeded", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Errorf("first parse: %v", err)
	}
	// The slot is free again.
	if _, err := p.Parse(context.Background(), normal); err != nil {
		t.Errorf("third parse: %v", err)
	}
}

// TestPanicContained: a panic in the decode path becomes an internal error
// with one ERROR log line holding the panic and its frames; the parser keeps
// working.
func TestPanicContained(t *testing.T) {
	normal := readSynthetic(t, "ride-30min.fit")
	var logs lineBuffer
	p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	calls := 0
	p.hook = func() {
		if calls++; calls == 5 { // after file_id
			panic("boom")
		}
	}
	_, err := p.Parse(context.Background(), normal)
	var ie *Error
	if !errors.As(err, &ie) || ie.Kind != KindInternal || ie.Message() != messages[KindInternal] {
		t.Fatalf("err = %v, want internal", err)
	}
	if ie.Device.Manufacturer != "development" {
		t.Errorf("device = %+v, want the file_id seen before the panic", ie.Device)
	}
	lines := logs.lines(t)
	if len(lines) != 1 {
		t.Fatalf("%d log lines, want 1", len(lines))
	}
	l := lines[0]
	if l["level"] != "ERROR" || l["panic"] != "boom" || l["kind"] != "internal" {
		t.Errorf("line = %v", l)
	}
	// The panic line holds no position and no coordinate vocabulary (QA-08-r1-6 c).
	raw := strings.ToLower(logs.raw()[0])
	for _, sub := range []string{"lat", "lon", "position"} {
		if strings.Contains(raw, sub) {
			t.Errorf("panic line contains %q: %s", sub, raw)
		}
	}
	delete(l, "time")
	noTime, _ := json.Marshal(l)
	for _, pos := range positions(t, normal) {
		if strings.Contains(string(noTime), pos) {
			t.Errorf("panic line contains the position value %q", pos)
			break
		}
	}
	frames, _ := l["frames"].([]any)
	if len(frames) == 0 || len(frames) > maxFrames {
		t.Fatalf("frames = %v", l["frames"])
	}
	for _, f := range frames {
		if !frameRE.MatchString(f.(string)) {
			t.Errorf("frame %q is not \"function file:line\"", f)
		}
	}

	p.hook = nil
	if _, err := p.Parse(context.Background(), normal); err != nil {
		t.Errorf("parse after a panic: %v", err)
	}
}

// capFile builds the all-caps file in memory (the shape of corpusgen
// -cap-file, ADR-017 rev. 1 item 7): n records at 1 Hz with every stream,
// 100 sessions alternating cycling and running, 10,000 laps with lap.sport,
// 100,000 timer events, 1,000 sport messages and 256 field descriptions. It
// is a single sequence and must stay within MaxBytes. Messages are written
// where they belong in time, so each sport message sits before a record.
func capFile(n int) []byte {
	const (
		sessionLen = 5000 // 100 sessions over 500,000 s
		lapLen     = 50   // 10,000 laps
		timerStep  = 5    // 100,000 timer events
	)
	b := &fitBuilder{}
	floodFieldDescriptions(b, 256)
	b.def(0, mesgnum.Record, []fieldDef{
		{253, 4, basetype.Uint32}, {0, 4, basetype.Sint32}, {1, 4, basetype.Sint32},
		{78, 4, basetype.Uint32}, {5, 4, basetype.Uint32}, {73, 4, basetype.Uint32},
		{3, 1, basetype.Uint8}, {4, 1, basetype.Uint8}, {7, 2, basetype.Uint16}, {13, 1, basetype.Sint8},
	})
	b.def(3, mesgnum.Session, []fieldDef{
		{253, 4, basetype.Uint32}, {2, 4, basetype.Uint32}, {5, 1, basetype.Enum}, {6, 1, basetype.Enum},
		{7, 4, basetype.Uint32}, {8, 4, basetype.Uint32}, {9, 4, basetype.Uint32}, {18, 1, basetype.Uint8}, {19, 1, basetype.Uint8},
	})
	b.def(4, mesgnum.Lap, []fieldDef{
		{253, 4, basetype.Uint32}, {2, 4, basetype.Uint32}, {7, 4, basetype.Uint32}, {8, 4, basetype.Uint32},
		{9, 4, basetype.Uint32}, {25, 1, basetype.Enum},
	})
	b.def(5, mesgnum.Event, []fieldDef{{253, 4, basetype.Uint32}, {0, 1, basetype.Enum}, {1, 1, basetype.Enum}})
	b.def(6, mesgnum.Sport, []fieldDef{{0, 1, basetype.Enum}, {1, 1, basetype.Enum}})
	sportOf := func(session int) byte { // FIT sport: running 1, cycling 2
		if session%2 == 0 {
			return 2
		}
		return 1
	}
	u32 := func(buf []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(buf, v) }
	sports := 0
	rec := make([]byte, 0, 40)
	for i := 0; i < n; i++ {
		ts := uint32(fitEpoch + i)
		if i%sessionLen == 0 {
			b.data(6, sportOf(i/sessionLen), 0)
			sports++
		} else if i%555 == 7 && sports < 1000 { // the other 900 sport messages
			b.data(6, sportOf(i/555), 0)
			sports++
		}
		if i%timerStep == 0 {
			ev := byte(0) // start
			if (i/timerStep)%2 == 1 {
				ev = 9 // stop_all
			}
			b.data(5, append(u32(nil, ts), 0, ev)...)
		}
		rec = u32(rec[:0], ts)
		rec = u32(rec, uint32(int32(560_000_000+i*30)))
		rec = u32(rec, uint32(int32(99_000_000+i*20)))
		rec = u32(rec, uint32(2_600+i%500)) // enhanced altitude
		rec = u32(rec, uint32(i*700))       // distance, cm
		rec = u32(rec, uint32(7000+i%900))  // enhanced speed, mm/s
		rec = append(rec, byte(120+i%40), byte(80+i%20))
		rec = binary.LittleEndian.AppendUint16(rec, uint16(180+i%90))
		rec = append(rec, 18)
		b.data(0, rec...)
		if i%lapLen == lapLen-1 {
			start := ts - (lapLen - 1)
			p := u32(u32(nil, ts), start)
			p = u32(u32(u32(p, lapLen*1000), lapLen*1000), lapLen*700*100)
			b.data(4, append(p, sportOf(i/sessionLen))...)
		}
		if i%sessionLen == sessionLen-1 {
			start := ts - (sessionLen - 1)
			p := u32(u32(nil, ts), start)
			p = append(p, sportOf(i/sessionLen), 0)
			p = u32(u32(u32(p, sessionLen*1000), sessionLen*1000), sessionLen*700*100)
			b.data(3, append(p, 90, 110)...)
		}
	}
	return b.bytes()
}

// BenchmarkParseAtCap parses the all-caps file (ADR-017 rev. 1 item 7):
// every budget at its limit, so decode and normalisation are at their worst
// together. The sub-benchmarks time the two stages apart. The results and
// the RSS of `parse` on the corpusgen -cap-file twin go into
// testdata/corpus/MEASUREMENTS.md.
func BenchmarkParseAtCap(b *testing.B) {
	data := capFile(DefaultLimits.MaxRecords)
	if len(data) > DefaultLimits.MaxBytes {
		b.Fatalf("the all-caps file is %d bytes, over MaxBytes", len(data))
	}
	b.Run("parse", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(data)))
		p := newTestParser(DefaultLimits)
		for b.Loop() {
			w, err := p.Parse(context.Background(), data)
			if err != nil {
				b.Fatal(err)
			}
			if len(w.Samples.Offset) != DefaultLimits.MaxRecords || len(w.Sessions) != DefaultLimits.MaxSessions {
				b.Fatalf("%d samples, %d sessions", len(w.Samples.Offset), len(w.Sessions))
			}
		}
	})
	l := newListener(DefaultLimits.withDefaults(), nil)
	res := walk(context.Background(), data, l, DefaultLimits.withDefaults())
	if res.lastErr != nil {
		b.Fatal(res.lastErr)
	}
	rec := l.rec
	rec.Integrity = res.integ
	if len(rec.Laps) != DefaultLimits.MaxLaps || len(rec.Timer) != DefaultLimits.MaxTimerEvents || len(rec.Sports) != DefaultLimits.MaxSportMessages {
		b.Fatalf("laps %d, timer events %d, sport messages %d", len(rec.Laps), len(rec.Timer), len(rec.Sports))
	}
	b.Run("decode", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			l := newListener(DefaultLimits.withDefaults(), nil)
			walk(context.Background(), data, l, DefaultLimits.withDefaults())
		}
	})
	b.Run("normalise", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			workout.Normalise(rec)
		}
	})
}
