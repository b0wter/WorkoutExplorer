package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
)

// The tests of F-09 design v1.6 (D-21, D-22, OQ-54).

// TestAcceptDataSizeSweep (D-21; QA's qa3_overrun_test.go as a test): for
// each input, the first sequence's header data size is moved by k for every
// k from -300 to +300 except 0, each with the header CRC left as it is and
// recomputed. If Parse rejects, Accept gives the same Kind. If Parse
// accepts, Accept accepts and the stored bytes pass checkStored,
// decodesToEnd and idempotence (checkStored's reference compares Parse(stored)
// with Parse(variant), the two repaired fields excepted). To stay within the
// 15 s budget, k is dense only around the end of the data (see below).
func TestAcceptDataSizeSweep(t *testing.T) {
	p := NewParser(DefaultLimits, nil)
	p.slots = make(chan struct{}, runtime.NumCPU()) // the variants run in parallel
	for _, name := range []string{"ride-30min.fit", "identifying-data.fit", "chained-ride-ride.fit"} {
		t.Run(name, func(t *testing.T) {
			base := readSynthetic(t, name)
			if base[0] != 14 {
				t.Fatalf("header size %d", base[0])
			}
			size := int(binary.LittleEndian.Uint32(base[4:8]))
			// Dense around the end: every k that puts the new data end inside the
			// last two records or within 4 bytes after it (the file CRC and what
			// follows); every 10th k elsewhere. The full range of 600 values
			// takes 17 s on the development machine, over the budget of 15 s.
			recs := splitSeq(t, base).records
			dense := len(recs[len(recs)-1].raw) + len(recs[len(recs)-2].raw)
			var accepted, rejected atomic.Int32
			t.Cleanup(func() { t.Logf("%s: %d variants accepted, %d rejected", name, accepted.Load(), rejected.Load()) })
			for k := -300; k <= 300; k++ {
				if k == 0 || size+k < 0 || (k%10 != 0 && (k < -dense || k > 4)) {
					continue
				}
				for _, freshCRC := range []bool{false, true} {
					v := append([]byte(nil), base...)
					binary.LittleEndian.PutUint32(v[4:8], uint32(size+k))
					if freshCRC {
						binary.LittleEndian.PutUint16(v[12:14], fitCRC(0, v[:12]))
					}
					t.Run(fmt.Sprintf("k=%d/fresh=%v", k, freshCRC), func(t *testing.T) {
						t.Parallel()
						crossCheckFile(t, v)
						_, perr := p.Parse(context.Background(), v)
						acc, aerr := p.Accept(context.Background(), v)
						if perr != nil {
							rejected.Add(1)
							var pe, ae *Error
							if !errors.As(perr, &pe) || !errors.As(aerr, &ae) || pe.Kind != ae.Kind {
								t.Errorf("Parse %v, Accept %v", perr, aerr)
							}
							return
						}
						accepted.Add(1)
						if aerr != nil {
							t.Fatalf("Parse accepts, Accept refuses: %v", aerr)
						}
						checkStored(t, p, v, acc)
						decodesToEnd(t, acc.Original)
						checkIdempotent(t, p, acc.Original)
						crossCheckFile(t, acc.Original)
					})
				}
			}
		})
	}
}

// TestStripOverlap (D-21): a crafted file whose second sequence's header lies
// inside the overrun record of the first. Parse accepts with two sequences;
// Accept writes sequence 1 closed over the whole overrun record, followed by
// sequence 2 closed, so the stored file is longer than the upload.
func TestStripOverlap(t *testing.T) {
	// Sequence 2: a file_id and ten records.
	in := &fitBuilder{}
	in.def(0, mesgnum.FileId, []fieldDef{{0, 1, basetype.Enum}})
	in.data(0, byte(typedef.FileActivity))
	in.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
	for i := range 10 {
		in.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch+uint32(i))...)
	}
	inner := in.bytes()

	// Sequence 1: file_id, a definition of a manufacturer-range message with
	// one byte field of size S, and one data message that carries sequence 2
	// at payload offset p.
	const p = 4
	s := p + len(inner)
	if s > 255 {
		t.Fatalf("payload size %d", s)
	}
	b := &fitBuilder{}
	b.def(0, mesgnum.FileId, []fieldDef{{0, 1, basetype.Enum}})
	b.data(0, byte(typedef.FileActivity))
	b.def(1, typedef.MesgNum(0xFF00), []fieldDef{{0, byte(s), basetype.Byte}})
	prefix := len(b.body) // bytes before the data message
	b.data(1, append(make([]byte, p), inner...)...)
	upload := b.bytes() // header, body, CRC (replaced below)
	innerAt := 14 + prefix + 1 + p
	// dataEnd + 2 falls on the inner header: dataEnd = innerAt - 2.
	binary.LittleEndian.PutUint32(upload[4:8], uint32(innerAt-2-14))
	binary.LittleEndian.PutUint16(upload[12:14], fitCRC(0, upload[:12]))
	if !bytes.Equal(upload[innerAt:innerAt+len(inner)], inner) {
		t.Fatal("the inner sequence is not where it should be")
	}
	// The overrun record is followed by 2 CRC bytes (b.bytes() put them there).

	w, err := testParser.Parse(context.Background(), upload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if w.Integrity.Sequences != 2 {
		t.Fatalf("sequences = %d, want 2", w.Integrity.Sequences)
	}
	acc, err := testParser.Accept(context.Background(), upload)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// Sequence 1 closed over the whole overrun record, then sequence 2 closed.
	first := (&fitBuilder{body: b.body}).bytes()
	want := append(append([]byte(nil), first...), inner...)
	if !bytes.Equal(acc.Original, want) {
		t.Errorf("stored %d bytes, want %d (sequence 1 closed, then sequence 2)", len(acc.Original), len(want))
	}
	if len(acc.Original) <= len(upload) {
		t.Errorf("stored %d bytes is not longer than the upload (%d)", len(acc.Original), len(upload))
	}
	checkStored(t, testParser, upload, acc)
	decodesToEnd(t, acc.Original)
	checkIdempotent(t, testParser, acc.Original)
	crossCheckFile(t, upload)
	crossCheckFile(t, acc.Original)
}

// TestAcceptReparseLimits (D-22, ADR-017 rev. 6, ADR-018 rev. 4).
func TestAcceptReparseLimits(t *testing.T) {
	t.Run("a: the reparse is not bounded by MaxBytes", func(t *testing.T) {
		upload := readSynthetic(t, "ride-30min-no-crc.fit")
		p := NewParser(Limits{MaxBytes: len(upload)}, nil)
		acc, err := p.Accept(context.Background(), upload)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if len(acc.Original) != len(upload)+2 {
			t.Errorf("stored %d bytes, want %d", len(acc.Original), len(upload)+2)
		}
		// The first parse still checks MaxBytes on the upload.
		_, err = NewParser(Limits{MaxBytes: len(upload) - 1}, nil).Accept(context.Background(), upload)
		var ie *Error
		if !errors.As(err, &ie) || ie.Kind != KindLimit || ie.Limit != limitBytes {
			t.Errorf("one byte over: err = %v", err)
		}
	})
	t.Run("b: a reparse time limit is the upload's limit/time", func(t *testing.T) {
		upload := readSynthetic(t, "ride-30min.fit")
		var logs lineBuffer
		p := NewParser(Limits{Timeout: 300 * time.Millisecond}, slog.New(slog.NewJSONHandler(&logs, nil)))
		var reparse atomic.Bool
		p.stripHook = func([]byte) { reparse.Store(true) }
		slept := false
		p.hook = func() {
			if reparse.Load() && !slept {
				slept = true
				time.Sleep(400 * time.Millisecond)
			}
		}
		acc, err := p.Accept(context.Background(), upload)
		var ie *Error
		if !errors.As(err, &ie) || ie.Kind != KindLimit || ie.Limit != limitTime {
			t.Fatalf("err = %v, want limit (time)", err)
		}
		if acc.Original != nil || len(acc.Workout.Samples.Offset) != 0 {
			t.Errorf("Accepted is not zero")
		}
		lines := logs.lines(t)
		if len(lines) != 1 || lines[0]["level"] != "WARN" || lines[0]["msg"] != "fit rejected" || lines[0]["limit"] != "time" {
			t.Fatalf("log lines = %v", logs.raw())
		}
	})
}

// TestAcceptRemovedTimestamp (OQ-54 closed: refuse; F-09 design v1.9, D-28; U2):
// a removed message's timestamp that a later compressed-timestamp record
// depends on. The decoder computes a compressed timestamp from the last
// timestamp it read, so without the removed ant_tx the record lands at least
// 32 s earlier and the stripped file is a different workout. Accept refuses
// it as unreadable. Variants (b) and (c) show the classifier refuses nothing
// more than before.
func TestAcceptRemovedTimestamp(t *testing.T) {
	build := func(antTxFields []fieldDef, antTx []byte, tail func(b *fitBuilder)) []byte {
		b := &fitBuilder{}
		b.def(0, mesgnum.FileId, []fieldDef{{0, 1, basetype.Enum}})
		b.data(0, byte(typedef.FileActivity))
		b.def(1, mesgnum.Record, []fieldDef{{253, 4, basetype.Uint32}})
		for i := range 5 {
			b.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch+uint32(i))...)
		}
		b.def(2, mesgnum.AntTx, antTxFields)
		b.data(2, antTx...)
		tail(b)
		return b.bytes()
	}
	ts := []fieldDef{{253, 4, basetype.Uint32}}
	late := binary.LittleEndian.AppendUint32(nil, fitEpoch+4+64) // 64 s later
	compressedTail := func(b *fitBuilder) {
		b.def(3, mesgnum.Record, []fieldDef{{3, 1, basetype.Uint8}}) // no timestamp field: a compressed header carries it
		b.compressed(3, 9, 120)                                      // 5 s after the ant_tx on the decoder's clock
	}

	t.Run("a: refused as unreadable", func(t *testing.T) {
		upload := build(ts, late, compressedTail)
		if _, err := testParser.Parse(context.Background(), upload); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		var logs lineBuffer
		p := NewParser(DefaultLimits, slog.New(slog.NewJSONHandler(&logs, nil)))
		acc, err := p.Accept(context.Background(), upload)
		var ie *Error
		if !errors.As(err, &ie) || ie.Kind != KindUnreadable || ie.Message() != "This file could not be read." || !errors.Is(err, errRemovedTimestamp) {
			t.Fatalf("err = %v, want unreadable wrapping errRemovedTimestamp", err)
		}
		if acc.Original != nil || len(acc.Workout.Samples.Offset) != 0 {
			t.Errorf("Accepted is not zero")
		}
		lines := logs.lines(t)
		if len(lines) != 1 || lines[0]["msg"] != "fit rejected" || lines[0]["level"] != "WARN" || lines[0]["kind"] != "unreadable" {
			t.Fatalf("log lines = %v", logs.raw())
		}
	})
	t.Run("a2: a removed message with a compressed header is refused as unreadable (QA-09-r4-1)", func(t *testing.T) {
		upload := build([]fieldDef{{0, 1, basetype.Uint8}}, []byte{7}, func(b *fitBuilder) {
			b.compressed(2, 20, 7) // the ant_tx moves the clock 20 s on
			compressedTail(b)
		})
		if _, err := testParser.Parse(context.Background(), upload); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		_, err := testParser.Accept(context.Background(), upload)
		var ie *Error
		if !errors.As(err, &ie) || ie.Kind != KindUnreadable || !errors.Is(err, errRemovedTimestamp) {
			t.Fatalf("err = %v, want unreadable wrapping errRemovedTimestamp", err)
		}
	})
	t.Run("b: the ant_tx definition without field 253 is accepted", func(t *testing.T) {
		upload := build([]fieldDef{{0, 1, basetype.Uint8}}, []byte{7}, compressedTail)
		acc, err := testParser.Accept(context.Background(), upload)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		checkStored(t, testParser, upload, acc)
	})
	t.Run("c: a normal header with a timestamp after the ant_tx is accepted", func(t *testing.T) {
		upload := build(ts, late, func(b *fitBuilder) {
			b.data(1, binary.LittleEndian.AppendUint32(nil, fitEpoch+4+70)...)
		})
		acc, err := testParser.Accept(context.Background(), upload)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		checkStored(t, testParser, upload, acc)
	})
}
