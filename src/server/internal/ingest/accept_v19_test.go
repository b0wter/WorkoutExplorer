package ingest

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
)

// TestAcceptEmptySequence (F-09 design v1.9, D-26, D-27; U1): a chained
// sequence that the walk calls complete with no record has no faithful
// stored form. Accept refuses it as unreadable before the strip writes a
// byte.
func TestAcceptEmptySequence(t *testing.T) {
	lead := sweepLead()
	if _, err := testParser.Parse(context.Background(), lead); err != nil {
		t.Fatalf("Parse(sweepLead): %v", err)
	}
	upload := append(append([]byte(nil), lead...), emptySeqProbe()...)
	second := len(lead)

	w, err := testParser.Parse(context.Background(), upload)
	if err != nil {
		t.Fatalf("Parse(upload): %v", err)
	}
	if w.Integrity.Sequences != 2 || !w.Integrity.CRCFailed {
		t.Errorf("sequences %d, crc_failed %v; want 2 and true", w.Integrity.Sequences, w.Integrity.CRCFailed)
	}
	spans := spansOf(t, upload)
	if len(spans) != 2 || spans[1].status != seqComplete || spans[1].records != 0 || !emptyComplete(spans[1]) || emptyComplete(spans[0]) {
		t.Fatalf("spans = %+v", spans)
	}

	out, _, err := strip(upload, spans, nil)
	if out != nil || !errors.Is(err, errEmptySequence) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(second)) {
		t.Errorf("strip: out %d bytes, err %v", len(out), err)
	}

	var logs lineBuffer
	p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
	hooked := false
	p.stripHook = func([]byte) { hooked = true }
	acc, err := p.Accept(context.Background(), upload)
	var ie *Error
	if !errors.As(err, &ie) || ie.Kind != KindUnreadable || ie.Message() != "This file could not be read." || !errors.Is(err, errEmptySequence) {
		t.Fatalf("Accept err = %v, want unreadable wrapping errEmptySequence", err)
	}
	if acc.Original != nil || len(acc.Workout.Samples.Offset) != 0 || acc.Stripped != (StripCounts{}) {
		t.Errorf("Accepted is not zero")
	}
	if hooked {
		t.Errorf("the strip hook was called")
	}
	lines := logs.lines(t)
	if len(lines) != 1 || lines[0]["msg"] != "fit rejected" || lines[0]["level"] != "WARN" || lines[0]["kind"] != "unreadable" {
		t.Fatalf("log lines = %v", logs.raw())
	}
	for _, k := range []string{"panic", "frames"} {
		if _, ok := lines[0][k]; ok {
			t.Errorf("log line has %q", k)
		}
	}

	// A canceled context comes first.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewParser(DefaultLimits, nil).Accept(ctx, upload); !errors.As(err, &ie) || ie.Kind != KindCanceled {
		t.Errorf("canceled: err = %v", err)
	}
}

// TestStripEmptyHandBuilt is TestStripHandBuilt (viii): a hand-built complete
// span with no record gives errEmptySequence through strip and through
// stripSeq, and no output.
func TestStripEmptyHandBuilt(t *testing.T) {
	def := rawDef(0, false, uint16(mesgnum.FileId), [3]byte{3, 4, byte(basetype.Uint32z)})
	data := handSeq(def)
	sp := seqSpan{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 0, status: seqComplete}
	out, _, err := strip(data, []seqSpan{sp}, func() error { t.Error("stop called"); return nil })
	if out != nil || !errors.Is(err, errEmptySequence) {
		t.Errorf("strip: out %v, err %v", out, err)
	}
	out, _, err = stripSeq(nil, data, sp)
	if out != nil || !errors.Is(err, errEmptySequence) {
		t.Errorf("stripSeq: out %v, err %v", out, err)
	}
	// An unclosed (truncated) span with no record is a legitimate stored form.
	sp.status = seqTruncated
	if out, _, err = strip(data, []seqSpan{sp}, nil); err != nil || len(out) != 14 {
		t.Errorf("truncated: %d bytes, err %v", len(out), err)
	}
}

// TestRemovedTimestampDependency (F-09 design v1.9, D-28): the U2 classifier
// on hand-built spans. crossCheckFraming runs on each input.
func TestRemovedTimestampDependency(t *testing.T) {
	u8, u32 := byte(basetype.Uint8), byte(basetype.Uint32)
	fileID := [][]byte{rawDef(0, false, uint16(mesgnum.FileId), [3]byte{0, 1, byte(basetype.Enum)}), {0, 4}}
	antTx := func(size byte) []byte { return rawDef(2, false, uint16(mesgnum.AntTx), [3]byte{253, size, u32}) }
	antTxData := []byte{2, 1, 2, 3, 4}
	recDef := rawDef(3, false, uint16(mesgnum.Record), [3]byte{3, 1, u8})
	compressed := []byte{0x80 | 3<<5 | 9, 120}
	normal := []byte{3, 120}

	seq := func(parts ...[]byte) []byte { return handSeq(append(append([][]byte(nil), fileID...), parts...)...) }
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"removed with 253, then compressed kept", seq(antTx(4), antTxData, recDef, compressed), true},
		{"removed with compressed header, then compressed kept", seq(rawDef(2, false, uint16(mesgnum.AntTx), [3]byte{0, 1, u8}), []byte{0x80 | 2<<5 | 5, 1}, recDef, compressed), true},
		{"removed with compressed header, then normal kept", seq(rawDef(2, false, uint16(mesgnum.AntTx), [3]byte{0, 1, u8}), []byte{0x80 | 2<<5 | 5, 1}, recDef, normal), false},
		{"field 253 of size 0", seq(antTx(0), antTxData[:1], recDef, compressed), false},
		{"removed without 253", seq(rawDef(2, false, uint16(mesgnum.AntTx), [3]byte{0, 1, u8}), []byte{2, 1}, recDef, compressed), false},
		{"only normal headers after", seq(antTx(4), antTxData, recDef, normal), false},
		{"compressed before the removed", seq(recDef, compressed, antTx(4), antTxData), false},
		{"the compressed message itself removed", seq(antTx(4), antTxData, rawDef(3, false, uint16(mesgnum.AntTx), [3]byte{3, 1, u8}), compressed), false},
		{"removed in span 0, compressed in span 1", append(seq(antTx(4), antTxData), handSeq(recDef, compressed)...), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spans := spansOf(t, c.data)
			if len(spans) == 0 {
				t.Fatal("no spans")
			}
			if strings.HasPrefix(c.name, "removed in span 0") && len(spans) != 2 {
				t.Fatalf("%d spans, want 2", len(spans))
			}
			crossCheckFraming(t, c.data, spans)
			if got := removedTimestampDependency(c.data, spans); got != c.want {
				t.Errorf("got %v, want %v (spans %+v)", got, c.want, spans)
			}
		})
	}
}

// TestCheckStoredBytes (F-09 design v1.9): a closed sequence followed by an
// unclosed one with only its header passes; a stored form with a leak still
// fails.
func TestCheckStoredBytes(t *testing.T) {
	closed := handSeq(rawDef(0, false, uint16(mesgnum.FileId), [3]byte{0, 1, byte(basetype.Enum)}), []byte{0, 4})
	unclosed := handSeq()[:14]
	for i := 4; i < 8; i++ {
		unclosed[i] = 0
	}
	unclosed[12], unclosed[13] = 0, 0
	checkStoredBytes(t, append(append([]byte(nil), closed...), unclosed...))

	s := &sweepTB{TB: t}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(sweepStop); !ok {
					panic(r)
				}
			}
		}()
		leak := handSeq(rawDef(0, false, uint16(mesgnum.UserProfile), [3]byte{4, 1, byte(basetype.Uint8)}), []byte{0, 41})
		checkStoredBytes(s, leak)
	}()
	if len(s.msgs) == 0 {
		t.Errorf("a stored removed message was not reported")
	}
}
