package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/corpustest"
)

// checkStored is the decoder check of FuzzAccept and TestCorpus (ADR-004
// item 9): the stored bytes are accepted by Parse with an equal report (after
// the two fields the rewrite repairs), no data message 3, 29 or 149 is left,
// and no blank-table field has a valid value. It decodes with the library,
// not with the strip's own framing.
func checkStored(t testing.TB, p *Parser, upload []byte, acc Accepted) {
	t.Helper()
	w1, err := p.Parse(context.Background(), upload)
	if err != nil {
		t.Fatalf("Parse(upload): %v", err)
	}
	w2, err := p.Parse(context.Background(), acc.Original)
	if err != nil {
		t.Fatalf("Parse(stored): %v", err)
	}
	r1, err := reference(w1)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := reference(w2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r1, r2) {
		t.Fatalf("the stored file is a different workout")
	}
	checkStoredBytes(t, acc.Original)
}

// checkStoredBytes decodes each sequence of the stored bytes with a
// storedListener: no removed data message and no valid blank-table field. It
// decodes with the library, not with the strip's own framing. An unclosed
// sequence with no bytes after its header is skipped, as decodesToEnd does:
// it is the stored form of a truncated span with no record (ADR-023 item 6),
// and it holds nothing to check (F-09 design v1.9, FuzzAccept row).
func checkStoredBytes(tb testing.TB, b []byte) {
	tb.Helper()
	for pos := 0; pos < len(b); {
		hs := int(b[pos])
		size := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		end := len(b)
		seq := append([]byte(nil), b[pos:]...)
		if size == 0 { // the unclosed form: decode the rest of the data
			if len(seq) == hs {
				return
			}
			binary.LittleEndian.PutUint32(seq[4:8], uint32(len(seq)-hs))
			if hs == 14 {
				seq[12], seq[13] = 0, 0
			}
		} else {
			end = pos + hs + size + 2
			seq = seq[:end-pos]
		}
		l := &storedListener{t: tb}
		dec := decoder.New(bytes.NewReader(seq), decoder.WithMesgListener(l), decoder.WithMesgDefListener(l), decoder.WithIgnoreChecksum())
		if _, err := dec.Decode(); err != nil && !(size == 0 && isEOF(err)) {
			// The unclosed form has no CRC, so the decoder ends at the missing bytes; any other error means broken bytes.
			tb.Fatalf("decoding the stored file: %v", err)
		}
		pos = end
	}
}

// decodesToEnd proves that every byte of a stored file is a record the
// library's decoder accepts (F-09 design v1.5, D-20; ADR-023 rev. 4 item 6a).
// It reads each sequence with the library's decoder only. A closed sequence
// (data size above 0) must decode with checksums on. An unclosed one (data
// size 0) gets its data size set to the bytes present, header CRC 0 and two
// placeholder bytes, and must decode with the checksum ignored: it then stops
// only at the missing file CRC. An unclosed sequence with no bytes after its
// header is accepted without decoding.
func decodesToEnd(tb testing.TB, stored []byte) {
	tb.Helper()
	for pos := 0; pos < len(stored); {
		if !looksLikeFIT(stored[pos:]) {
			tb.Fatalf("decodesToEnd: not a FIT header at byte %d", pos)
		}
		hs := int(stored[pos])
		size := int(binary.LittleEndian.Uint32(stored[pos+4 : pos+8]))
		var seq []byte
		var opts []decoder.Option
		if size > 0 {
			end := pos + hs + size + 2
			if end > len(stored) {
				tb.Fatalf("decodesToEnd: closed sequence at byte %d runs past the end", pos)
			}
			seq = stored[pos:end]
			pos = end
		} else {
			if len(stored)-pos == hs {
				return
			}
			seq = append([]byte(nil), stored[pos:]...)
			binary.LittleEndian.PutUint32(seq[4:8], uint32(len(seq)-hs))
			if hs == 14 {
				seq[12], seq[13] = 0, 0
			}
			seq = append(seq, 0, 0)
			opts = append(opts, decoder.WithIgnoreChecksum())
			pos = len(stored)
		}
		if _, err := decoder.New(bytes.NewReader(seq), opts...).Decode(); err != nil {
			tb.Fatalf("decodesToEnd: the sequence before byte %d does not decode to its end: %v", pos, err)
		}
	}
}

// checkIdempotent: Accept on a stored original succeeds and returns the same
// bytes (F-30 regeneration; ADR-004 rev. 8). A time limit caused by machine
// load is retried once, like TestFramingSweep (QA-09-r4-3); it fails if the
// retry also hits the limit or fails otherwise.
func checkIdempotent(tb testing.TB, p *Parser, stored []byte) {
	tb.Helper()
	acc, err := p.Accept(context.Background(), stored)
	var ie *Error
	if errors.As(err, &ie) && ie.Kind == KindLimit && ie.Limit == limitTime {
		acc, err = p.Accept(context.Background(), stored)
	}
	if err != nil {
		tb.Fatalf("Accept(stored): %v", err)
	}
	if !bytes.Equal(acc.Original, stored) {
		tb.Fatalf("Accept(stored) returns other bytes (%d, stored %d)", len(acc.Original), len(stored))
	}
}

type storedListener struct {
	t     testing.TB
	short [16]map[blankKey]bool // per local type: (message, field) its current definition declares shorter than its read type (F-09 v1.7, D-23; v1.8 FuzzAccept (e))
}

// OnMesgDef records the blank-table fields that a definition declares with a
// size below their read type's, a non-z, non-string type: a short field, which
// has no invalid encoding and reads as 0 (ADR-023 rev. 6 item 4). A definition
// replaces the record of its local type, so 0 is accepted only for a data
// message whose own definition declares the field short (F-09 v1.8).
func (l *storedListener) OnMesgDef(d proto.MessageDefinition) {
	local := proto.LocalMesgNum(d.Header)
	l.short[local] = nil
	for _, f := range d.FieldDefinitions {
		k := blankKey{uint16(d.MesgNum), f.Num}
		if rt, ok := readTypes[k]; ok && int(f.Size) < int(rt.Size()) &&
			rt != basetype.String && rt != basetype.Uint8z && rt != basetype.Uint16z && rt != basetype.Uint32z && rt != basetype.Uint64z {
			if l.short[local] == nil {
				l.short[local] = map[blankKey]bool{}
			}
			l.short[local][k] = true
		}
	}
}

func (l *storedListener) OnMesg(m proto.Message) {
	if removeTable[uint16(m.Num)] {
		l.t.Errorf("data message %d in the stored file", m.Num)
	}
	short := l.short[proto.LocalMesgNum(m.Header)]
	for _, n := range blankTable[uint16(m.Num)] {
		if !absent(m, n) && !(short[blankKey{uint16(m.Num), n}] && shortIsZero(m, n)) {
			l.t.Errorf("message %d field %d has a valid value in the stored file", m.Num, n)
		}
	}
}

// shortIsZero: every element of the field's value is 0.
func shortIsZero(m proto.Message, n byte) bool {
	f, _ := fieldOf(m, n)
	return isZeroValue(f.Value)
}

// TestAcceptRejections: every rejected corpus file gives the same *Error from
// Accept as from Parse, and one log line (F-09 design, Tests).
func TestAcceptRejections(t *testing.T) {
	var logs lineBuffer
	p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	rejected := 0
	for _, path := range corpustest.Files(t, corpusDir, "synthetic") {
		name := filepath.Base(path)
		data := readSynthetic(t, name)
		_, perr := testParser.Parse(context.Background(), data)
		var pe *Error
		if !errors.As(perr, &pe) {
			continue
		}
		rejected++
		before := len(logs.raw())
		_, aerr := p.Accept(context.Background(), data)
		var ae *Error
		if !errors.As(aerr, &ae) {
			t.Errorf("%s: Accept err = %v, want an *Error", name, aerr)
			continue
		}
		if ae.Kind != pe.Kind || ae.Message() != pe.Message() || ae.Limit != pe.Limit || ae.FileType != pe.FileType {
			t.Errorf("%s: Accept gives %v, Parse %v", name, ae, pe)
		}
		if n := len(logs.raw()) - before; n != 1 {
			t.Errorf("%s: %d log lines, want 1", name, n)
		}
	}
	if rejected < 8 {
		t.Errorf("only %d rejected corpus files", rejected)
	}
}

// TestAcceptFailsClosed: a strip that cannot be verified refuses the upload
// as internal with a "strip:" cause and one ERROR line without a field value
// (ADR-023 item 8).
func TestAcceptFailsClosed(t *testing.T) {
	upload := readSynthetic(t, "identifying-data.fit")
	hooks := map[string]func([]byte){
		"a record byte altered": func(b []byte) { b[len(b)/2] ^= 0xFF },
		"a different workout":   func(b []byte) { copy(b[len(b)-60:], make([]byte, 40)) },
		"not a FIT file":        func(b []byte) { clear(b[:12]) },
	}
	for name, hook := range hooks {
		t.Run(name, func(t *testing.T) {
			var logs lineBuffer
			p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
			p.stripHook = hook
			acc, err := p.Accept(context.Background(), upload)
			var ie *Error
			if !errors.As(err, &ie) || ie.Kind != KindInternal || !strings.Contains(ie.Error(), "strip:") {
				t.Fatalf("err = %v, want internal with a strip: cause", err)
			}
			if ie.Message() != "This file could not be read." {
				t.Errorf("message = %q", ie.Message())
			}
			if acc.Original != nil || len(acc.Workout.Samples.Offset) != 0 {
				t.Errorf("Accepted is not zero")
			}
			lines := logs.lines(t)
			if len(lines) != 1 || lines[0]["level"] != "ERROR" || lines[0]["kind"] != "internal" {
				t.Fatalf("log lines = %v", logs.raw())
			}
			for _, m := range markers() {
				if len(m) >= 5 && strings.Contains(logs.raw()[0], string(m)) {
					t.Errorf("log line contains the marker %q", m)
				}
			}
		})
	}
}

// TestAcceptLogs: one line per Accept (none for a clean upload); a truncated
// upload logs truncated from the first parse; the reparse writes nothing;
// neither positions nor the markers of identifying-data.fit are in a line.
func TestAcceptLogs(t *testing.T) {
	var logs lineBuffer
	p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	identifying := readSynthetic(t, "identifying-data.fit")
	settings := append(append([]byte(nil), identifying...), encodeSettings(t)...)
	inputs := map[string][]byte{
		"ride-30min.fit":            readSynthetic(t, "ride-30min.fit"),
		"identifying-data.fit":      identifying,
		"ride-30min-cut-60.fit":     readSynthetic(t, "ride-30min-cut-60.fit"),
		"ride-30min-bad-crc.fit":    readSynthetic(t, "ride-30min-bad-crc.fit"),
		"identifying + settings":    settings,
		"cut identifying-data":      identifying[:len(identifying)*6/10],
		"ride-30min-header-only":    readSynthetic(t, "ride-30min-header-only.fit"),
		"chained-with-settings":     readSynthetic(t, "chained-with-settings.fit"),
		"ride-30min-trailing-0.fit": readSynthetic(t, "ride-30min-trailing-zeros.fit"),
	}
	var scan []string
	for _, in := range inputs {
		scan = append(scan, positions(t, in)...)
	}
	if len(scan) < 1000 {
		t.Fatalf("only %d position strings", len(scan))
	}
	for name, data := range inputs {
		t.Run(name, func(t *testing.T) {
			before := len(logs.raw())
			_, perr := testParser.Parse(context.Background(), data)
			acc, aerr := p.Accept(context.Background(), data)
			lines := logs.raw()[before:]
			recovered := false
			if aerr == nil {
				w := mustParse(t, data)
				recovered = w.Integrity.Truncated || w.Integrity.CRCFailed || w.Integrity.IgnoredBytes > 0 || w.Integrity.SkippedSequences > 0
				if acc.Original == nil {
					t.Fatal("no original")
				}
			}
			want := 0
			if aerr != nil || recovered {
				want = 1
			}
			if (perr == nil) != (aerr == nil) {
				t.Fatalf("Parse err %v, Accept err %v", perr, aerr)
			}
			if len(lines) != want {
				t.Fatalf("%d log lines, want %d: %v", len(lines), want, lines)
			}
			for _, l := range lines {
				for _, m := range markers() {
					if len(m) >= 5 && strings.Contains(l, string(m)) { // binary forms are no log text
						t.Errorf("log line contains the marker %q", m)
					}
				}
				for _, s := range scan {
					if strings.Contains(l[strings.Index(l, `"level"`):], s) {
						t.Errorf("log line contains the position value %q", s)
					}
				}
			}
			if name == "ride-30min-cut-60.fit" && (len(lines) != 1 || !strings.Contains(lines[0], `"truncated":true`)) {
				t.Errorf("truncated upload: %v", lines)
			}
		})
	}
}

// TestAcceptReleasesFirstParse: when the strip hook runs, nothing of the first
// parse is reachable any more (D-17, ADR-017 rev. 3 item 2). Not parallel: it
// reads the process heap.
func TestAcceptReleasesFirstParse(t *testing.T) {
	upload := readSynthetic(t, "ride-12h.fit")
	p := NewParser(DefaultLimits, nil)
	if _, err := p.Accept(context.Background(), upload); err != nil {
		t.Fatal(err)
	}
	heap := func() uint64 {
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapAlloc
	}
	base := heap()
	var atHook uint64
	var stripped int
	p.stripHook = func(b []byte) { atHook, stripped = heap(), len(b) }
	if _, err := p.Accept(context.Background(), upload); err != nil {
		t.Fatal(err)
	}
	// The upload stays live through the hook, as the request's buffer does on
	// the server; without this the GC frees it while the stripped copy is live.
	runtime.KeepAlive(upload)
	excess := int64(atHook) - int64(base)
	limit := int64(stripped) + 1<<20
	t.Logf("heap at the hook minus baseline: %d bytes (limit %d)", excess, limit)
	if excess > limit {
		t.Errorf("first parse still reachable: excess %d > %d", excess, limit)
	}
}

// TestAcceptKeepsOneSlot: Accept holds one parse slot for the whole flow, so
// with one slot a second Accept waits instead of the reparse deadlocking.
func TestAcceptKeepsOneSlot(t *testing.T) {
	p := NewParser(Limits{MaxConcurrent: 1}, nil)
	data := readSynthetic(t, "ride-30min.fit")
	done := make(chan error, 3)
	for range 3 {
		go func() { _, err := p.Accept(context.Background(), data); done <- err }()
	}
	for range 3 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("Accept deadlocked on its own slot")
		}
	}
}

// classTB prefixes every failure message with its class (F-09 design v1.9,
// D-29): privacy, divergence, integrity, internal refusal, robustness. A
// non-nil classify picks the class from the message.
type classTB struct {
	testing.TB
	class    string
	classify func(msg string) string
}

func (c classTB) prefixed(msg string) string {
	class := c.class
	if c.classify != nil {
		class = c.classify(msg)
	}
	return class + ": " + msg
}
func (c classTB) Errorf(format string, args ...any) {
	c.TB.Helper()
	c.TB.Errorf("%s", c.prefixed(fmt.Sprintf(format, args...)))
}
func (c classTB) Fatalf(format string, args ...any) {
	c.TB.Helper()
	c.TB.Fatalf("%s", c.prefixed(fmt.Sprintf(format, args...)))
}

// storedClass: a removed message or a valid blank field in the stored bytes
// is a privacy failure; every other failure of checkStored is integrity.
func storedClass(msg string) string {
	if strings.Contains(msg, "in the stored file") || strings.Contains(msg, "has a valid value") {
		return "privacy"
	}
	return "integrity"
}

// FuzzAccept (F-09 design v1.9, D-29). For any input: (P1) no panic, and the
// call returns within 2 x (Timeout + 2 s); (P2) crossCheckFile(upload); (P3)
// the result is exactly one of: accepted, with checkStored, decodesToEnd,
// checkIdempotent and crossCheckFile(stored); the upload's own rejection
// (not_fit, unsupported_type, no_data, limit), canceled, or a reparse
// limit/time; or a deliberate refusal, KindUnreadable wrapping errEmptySequence
// or errRemovedTimestamp. Anything else fails, with its class first:
// privacy:, divergence:, integrity:, internal refusal: or robustness:. Seeds
// are the synthetic files, never real/. Manual run, on a scratch copy of
// src/server without testdata/corpus/real/ (F-09 design v1.6, Tests; ADR-004
// item 9). -fuzzminimizetime keeps minimization from eating the run:
// go test ./internal/ingest -run '^$' -fuzz '^FuzzAccept$' -fuzztime 30m -fuzzminimizetime 5s
func FuzzAccept(f *testing.F) {
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
		div := classTB{TB: t, class: "divergence"}
		robust := classTB{TB: t, class: "robustness"}
		integ := classTB{TB: t, class: "integrity"}
		crossCheckFile(div, data) // (P2) the strip and the decoder frame the upload alike
		start := time.Now()
		acc, err := p.Accept(context.Background(), data)
		if d := time.Since(start); d > 2*(lim.Timeout+2*time.Second) {
			robust.Fatalf("took %v", d)
		}
		if errors.Is(err, errRecordCount) {
			div.Fatalf("errRecordCount: %v", err)
		}
		if err != nil {
			var ie *Error
			if !errors.As(err, &ie) {
				t.Fatalf("internal refusal: err = %v, not an *Error", err)
			}
			switch ie.Kind {
			case KindNotFIT, KindUnsupportedType, KindNoData, KindLimit, KindCanceled:
				return
			case KindUnreadable:
				if errors.Is(err, errEmptySequence) || errors.Is(err, errRemovedTimestamp) {
					return
				}
				t.Fatalf("internal refusal: unlisted unreadable cause: %v", err)
			case KindInternal:
				if ie.cause != nil && ie.cause.Error() == "panic" {
					t.Fatalf("robustness: panic in Accept: %v", err)
				}
				t.Fatalf("internal refusal: %v", err)
			}
			t.Fatalf("internal refusal: unexpected kind %q: %v", ie.Kind, err)
		}
		checkStored(classTB{TB: t, classify: storedClass}, p, data, acc)
		decodesToEnd(integ, acc.Original)
		checkIdempotent(integ, p, acc.Original)
		crossCheckFile(div, acc.Original)
	})
}

// BenchmarkAcceptAtCap is Accept on the all-caps file (ADR-017 rev. 2 item 4):
// parse, strip, reparse and verify in one slot. The results and the RSS of
// `parse -original` on the corpusgen -cap-file twin go into
// testdata/corpus/MEASUREMENTS.md. Stop rule: whole Accept at most 4.5 s.
func BenchmarkAcceptAtCap(b *testing.B) {
	data := capFile(DefaultLimits.MaxRecords)
	if len(data) > DefaultLimits.MaxBytes {
		b.Fatalf("the all-caps file is %d bytes, over MaxBytes", len(data))
	}
	p := newTestParser(DefaultLimits)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		acc, err := p.Accept(context.Background(), data)
		if err != nil {
			b.Fatal(err)
		}
		if len(acc.Workout.Samples.Offset) != DefaultLimits.MaxRecords {
			b.Fatalf("%d samples", len(acc.Workout.Samples.Offset))
		}
	}
}

// definitionFlood is one closed sequence of about size bytes: a file_id, one
// record, then definition messages of a manufacturer-range message with
// fields fields each, to the end (ADR-017 rev. 5). Never committed.
func definitionFlood(size, fields int) []byte {
	b := &fitBuilder{}
	b.def(0, mesgnum.FileId, []fieldDef{{0, 1, basetype.Enum}})
	b.data(0, byte(typedef.FileActivity))
	b.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}, {3, 1, basetype.Uint8}})
	b.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch)...)
	b.body = append(b.body, 120)
	fs := make([]fieldDef, fields)
	for i := range fs {
		fs[i] = fieldDef{byte(i), 1, basetype.Uint8}
	}
	for len(b.body)+14+2+6+3*fields <= size {
		b.def(3, typedef.MesgNum(0xFF00), fs)
	}
	return b.bytes()
}

// BenchmarkAcceptDefinitions is Accept on two definition floods of about
// 20,000,000 bytes: one with 1 field per definition, one with 255 (ADR-017
// rev. 5, the cost of the definition listener). Stop rule: at most 4.5 s per
// Accept. The results go into testdata/corpus/MEASUREMENTS.md.
func BenchmarkAcceptDefinitions(b *testing.B) {
	for _, fields := range []int{1, 255} {
		b.Run(fmt.Sprintf("fields=%d", fields), func(b *testing.B) {
			data := definitionFlood(20_000_000, fields)
			if len(data) > DefaultLimits.MaxBytes {
				b.Fatalf("%d bytes, over MaxBytes", len(data))
			}
			p := newTestParser(DefaultLimits)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				if _, err := p.Accept(context.Background(), data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
