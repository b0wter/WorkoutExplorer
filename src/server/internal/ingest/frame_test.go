package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/fieldnum"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"
)

// ccRecord is one record the library's decoder reported: a definition or a
// data message, with the definition it was read against.
type ccRecord struct {
	definition bool
	header     byte
	global     uint16
	def        proto.MessageDefinition // the definition (definition) or the one in force (data message)
	fieldTypes []basetype.BaseType     // data message: the decoder's BaseType per field, in order
}

// ccRecorder collects the decoder's record stream (F-09 design v1.8, D-25).
type ccRecorder struct {
	defs [16]proto.MessageDefinition
	recs []ccRecord
}

func (r *ccRecorder) OnMesgDef(d proto.MessageDefinition) {
	r.defs[proto.LocalMesgNum(d.Header)] = d
	r.recs = append(r.recs, ccRecord{definition: true, header: d.Header, global: uint16(d.MesgNum), def: d})
}

func (r *ccRecorder) OnMesg(m proto.Message) {
	rec := ccRecord{header: m.Header, global: uint16(m.Num), def: r.defs[proto.LocalMesgNum(m.Header)]}
	for _, f := range m.Fields {
		rec.fieldTypes = append(rec.fieldTypes, f.BaseType)
	}
	r.recs = append(r.recs, rec)
}

// crossCheckFraming (F-09 design v1.8, D-25) decodes each span the way
// decodeSequence does and compares the record stream with frame.go's
// iterator, record by record. A failure names the span offset, the record
// index and the attribute that differs, never a value from the file. It uses
// neither Parse, the listener nor checkStored.
func crossCheckFraming(tb testing.TB, data []byte, spans []seqSpan) {
	tb.Helper()
	for _, sp := range spans {
		fail := func(i int, attr string) {
			tb.Helper()
			tb.Fatalf("crossCheckFraming: span at byte %d, record %d: %s differs", sp.pos, i, attr)
		}
		r := bytes.NewReader(data[sp.pos:])
		var src = data[sp.pos:]
		if binary.LittleEndian.Uint32(src[4:8]) == 0 { // patched, as decodeSequence does
			hdr := append([]byte(nil), src[:sp.headerSize]...)
			binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(src)-sp.headerSize))
			if sp.headerSize == 14 {
				hdr[12], hdr[13] = 0, 0
			}
			r = bytes.NewReader(append(hdr, src[sp.headerSize:]...))
		}
		rec := &ccRecorder{}
		_, _ = decoder.New(r, decoder.WithMesgListener(rec), decoder.WithMesgDefListener(rec),
			decoder.WithIgnoreChecksum(), decoder.WithBroadcastOnly(), decoder.WithNoComponentExpansion()).DecodeWithContext(context.Background())
		if len(rec.recs) != sp.records {
			fail(min(len(rec.recs), sp.records), "record count")
		}

		regionEnd := len(data)
		if sp.status == seqTruncated {
			regionEnd = sp.dataEnd
		}
		rest := data[sp.pos+sp.headerSize : regionEnd]
		var defs [16]*stripDef
		for i, want := range rec.recs {
			got, st := nextRecord(rest, &defs)
			if st != framed {
				fail(i, "frame status")
			}
			got.install(&defs)
			if got.header != want.header {
				fail(i, "header byte")
			}
			if got.definition != want.definition {
				fail(i, "record kind")
			}
			if got.local != proto.LocalMesgNum(want.header) {
				fail(i, "local type")
			}
			if uint16(got.def.global) != want.global {
				fail(i, "global number")
			}
			if got.def.arch != want.def.Architecture {
				fail(i, "architecture byte")
			}
			if got.def.big != (want.def.Architecture != proto.LittleEndian) {
				fail(i, "byte order")
			}
			if got.definition {
				if !sameFields(got.def.fields, want.def.FieldDefinitions) {
					fail(i, "field list")
				}
				if !sameDevFields(got.def.devFields, want.def.DeveloperFieldDefinitions) {
					fail(i, "developer field list")
				}
			} else {
				n := 1
				for _, f := range want.def.FieldDefinitions {
					n += int(f.Size)
				}
				for _, f := range want.def.DeveloperFieldDefinitions {
					n += int(f.Size)
				}
				if got.length != n {
					fail(i, "data length")
				}
				if removeTable[got.def.global] != removeTable[want.global] {
					fail(i, "removal")
				}
				crossCheckSpots(got.def, want, func(attr string) { fail(i, attr) })
			}
			rest = rest[got.length:]
		}
	}
}

// crossCheckSpots compares the blank spots of a data message's definition
// with the fields the decoder read: every blank-table field of size above 0
// has a spot, at the offset the decoder's sizes give, with the decoder's own
// base type for it and the byte order the decoder used.
func crossCheckSpots(d *stripDef, want ccRecord, fail func(string)) {
	var exp []blankSpot
	off := 0
	skip := 0
	if want.header&proto.MesgCompressedHeaderMask != 0 {
		skip = 1 // the decoder puts the timestamp field first
	}
	for i, f := range want.def.FieldDefinitions {
		if slices.Contains(blankTable[uint16(want.def.MesgNum)], f.Num) {
			if skip+i >= len(want.fieldTypes) {
				fail("field list")
				return
			}
			exp = append(exp, blankSpot{offset: off, size: int(f.Size), readType: want.fieldTypes[skip+i], big: want.def.Architecture != proto.LittleEndian})
		}
		off += int(f.Size)
	}
	if len(d.blank) != len(exp) {
		fail("blank spot count")
		return
	}
	for k := range exp {
		if d.blank[k] != exp[k] {
			fail("blank spot")
			return
		}
	}
}

func sameFields(got []stripField, want []proto.FieldDefinition) bool {
	var g []stripField
	for _, f := range got {
		if f.size > 0 {
			g = append(g, f)
		}
	}
	if len(g) != len(want) {
		return false
	}
	for i := range g {
		if g[i].num != want[i].Num || g[i].size != want[i].Size || g[i].baseType != uint8(want[i].BaseType) {
			return false
		}
	}
	return true
}

func sameDevFields(got []stripDevField, want []proto.DeveloperFieldDefinition) bool {
	var g []stripDevField
	for _, f := range got {
		if f.size > 0 {
			g = append(g, f)
		}
	}
	if len(g) != len(want) {
		return false
	}
	for i := range g {
		if g[i].num != want[i].Num || g[i].size != want[i].Size || g[i].devIndex != want[i].DeveloperDataIndex {
			return false
		}
	}
	return true
}

// crossCheckFile runs crossCheckFraming over the spans the walk finds in
// data, if it looks like a FIT file.
func crossCheckFile(tb testing.TB, data []byte) {
	tb.Helper()
	if looksLikeFIT(data) {
		crossCheckFraming(tb, data, spansOf(tb, data))
	}
}

// sweepTB turns the failures of the checks it is handed into values, so that
// TestFramingSweep can count them (F-09 design v1.9, D-29). It overrides every
// failure method of testing.TB: Error, Errorf and Fail record; Fatal, Fatalf
// and FailNow record and panic with sweepStop; Skip, Skipf and SkipNow panic
// with sweepSkip, which counts as a failure ("a check skipped"). Nothing a
// check calls reaches the real *testing.T except Log, Logf, Helper, Name and
// Cleanup (and the accessors that cannot fail it).
type sweepTB struct {
	testing.TB
	msgs []string
}

type sweepStop struct{}
type sweepSkip struct{}

func (s *sweepTB) record(msg string) { s.msgs = append(s.msgs, msg) }

func (s *sweepTB) Error(args ...any)                 { s.record(fmt.Sprint(args...)) }
func (s *sweepTB) Errorf(format string, args ...any) { s.record(fmt.Sprintf(format, args...)) }
func (s *sweepTB) Fail()                             { s.record("Fail called") }
func (s *sweepTB) Failed() bool                      { return len(s.msgs) > 0 }
func (s *sweepTB) Fatal(args ...any)                 { s.record(fmt.Sprint(args...)); panic(sweepStop{}) }
func (s *sweepTB) Fatalf(format string, args ...any) {
	s.record(fmt.Sprintf(format, args...))
	panic(sweepStop{})
}
func (s *sweepTB) FailNow()             { s.record("FailNow called"); panic(sweepStop{}) }
func (s *sweepTB) Skip(args ...any)     { panic(sweepSkip{}) }
func (s *sweepTB) Skipf(string, ...any) { panic(sweepSkip{}) }
func (s *sweepTB) SkipNow()             { panic(sweepSkip{}) }
func (s *sweepTB) Skipped() bool        { return false }
func (s *sweepTB) Helper()              {}

// guard runs f and returns the first failure it recorded or panicked with,
// "" when none. A panic of any other value is raised again.
func (s *sweepTB) guard(f func()) (fail string) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case sweepStop:
			case sweepSkip:
				s.record("a check skipped")
			default:
				panic(r)
			}
		}
		if len(s.msgs) > 0 {
			fail = s.msgs[0]
		}
	}()
	f()
	return ""
}

// framingProbe builds the probe of TestFramingSweep (F-09 design v1.8, D-25):
// one complete sequence with a 14-byte header and correct CRCs. Every blank
// field byte is 0x5A.
func framingProbe() []byte {
	u8, u32, u32z, u16z := byte(basetype.Uint8), byte(basetype.Uint32), byte(basetype.Uint32z), byte(basetype.Uint16z)
	var recs [][]byte
	// file_id, architecture 0: type, serial_number.
	recs = append(recs,
		rawDefArch(0, 0, uint16(mesgnum.FileId), [3]byte{0, 1, byte(basetype.Enum)}, [3]byte{3, 4, u32z}),
		[]byte{0, 4, 0x5A, 0x5A, 0x5A, 0x5A})
	// device_info, architecture 1: serial_number, ant_device_number.
	recs = append(recs,
		rawDefArch(1, 1, uint16(mesgnum.DeviceInfo), [3]byte{3, 4, u32z}, [3]byte{21, 2, u16z}),
		[]byte{1, 0x5A, 0x5A, 0x5A, 0x5A, 0x5A, 0x5A})
	// segment_id, architecture 0x30: device_id.
	recs = append(recs,
		rawDefArch(2, 0x30, uint16(mesgnum.SegmentId), [3]byte{byte(fieldnum.SegmentIdDeviceId), 4, u32}),
		[]byte{2, 0x5A, 0x5A, 0x5A, 0x5A})
	// user_profile: a normal and a compressed-timestamp data message.
	recs = append(recs,
		rawDefArch(3, 0, uint16(mesgnum.UserProfile), [3]byte{4, 1, u8}),
		[]byte{3, 41},
		[]byte{0x80 | 3<<5 | 5, 42})
	// developer_data_id, field_description, a definition with one developer field and its data message.
	recs = append(recs,
		rawDefArch(4, 0, uint16(mesgnum.DeveloperDataId), [3]byte{3, 1, u8}),
		[]byte{4, 0},
		rawDefArch(5, 0, uint16(mesgnum.FieldDescription), [3]byte{0, 1, u8}, [3]byte{1, 1, u8}, [3]byte{2, 1, u8}),
		[]byte{5, 0, 9, 2})
	devDef := rawDefArch(6, 0, uint16(mesgnum.Record), [3]byte{3, 1, u8})
	devDef[0] |= 0x20
	devDef = append(devDef, 1, 9, 1, 0) // one developer field: number 9, size 1, developer data index 0
	recs = append(recs, devDef, []byte{6, 0x8C, 0x07})
	// a definition with a size-0 field, then the same local type defined again, then a record with a timestamp.
	recs = append(recs,
		rawDefArch(7, 0, uint16(mesgnum.Record), [3]byte{3, 1, u8}, [3]byte{4, 0, u8}),
		[]byte{7, 0x90},
		rawDefArch(7, 0, uint16(mesgnum.Record), [3]byte{253, 4, u32}, [3]byte{3, 1, u8}),
		append(append([]byte{7}, binary.LittleEndian.AppendUint32(nil, fitEpoch)...), 0x91))
	return handSeq(recs...)
}

// sweepLead is the fixed first sequence of TestFramingSweep's part 2 and of
// the deliberate-refusal tests (F-09 design v1.9, D-29): a 14-byte header,
// file_id (type activity), three records with a timestamp one second apart
// and a heart rate, correct CRCs.
func sweepLead() []byte {
	u8, u32 := byte(basetype.Uint8), byte(basetype.Uint32)
	recs := [][]byte{
		rawDefArch(0, 0, uint16(mesgnum.FileId), [3]byte{0, 1, byte(basetype.Enum)}),
		{0, byte(typedef.FileActivity)},
		rawDefArch(1, 0, uint16(mesgnum.Record), [3]byte{253, 4, u32}, [3]byte{byte(fieldnum.RecordHeartRate), 1, u8}),
	}
	for i := range 3 {
		recs = append(recs, append(append([]byte{1}, binary.LittleEndian.AppendUint32(nil, uint32(fitEpoch+i))...), byte(100+i)))
	}
	return handSeq(recs...)
}

// emptySeqProbe is the probe with byte 19 (the first definition's field
// count) set to 0x2F: the definition runs past the end of the bytes, so the
// walk calls the sequence complete with no record (U1, audit S4).
func emptySeqProbe() []byte {
	b := framingProbe()
	b[19] = 0x2F
	return b
}

// sweepOne is part 1 of TestFramingSweep (strip level): the checks on one
// mutant of the probe. It returns the first failure ("" when everything
// holds), whether the mutant was skipped for lack of spans, and whether the
// strip refused it as U1.
func sweepOne(t *testing.T, in []byte) (fail string, skipped, u1 bool) {
	s := &sweepTB{TB: t}
	if !looksLikeFIT(in) {
		return "", true, false
	}
	fail = s.guard(func() {
		spans := spansOf(s, in)
		if len(spans) == 0 {
			skipped = true
			return
		}
		crossCheckFraming(s, in, spans)
		out, _, err := strip(in, spans, nil)
		if err != nil {
			if errors.Is(err, errEmptySequence) && out == nil {
				u1 = true
				return
			}
			s.Fatalf("strip: %v", err) // errRecordCount and anything else
		}
		checkStoredBytes(s, out)
		decodesToEnd(s, out)
		crossCheckFile(s, out)
	})
	return fail, skipped, u1
}

// sweepAccept is part 2 of TestFramingSweep (Accept level): the mutant chained
// behind lead. It returns the first failure and which deliberate refusal
// ended it ("u1", "u2" or "").
func sweepAccept(t *testing.T, p *Parser, lead, mutant []byte) (fail, refusal string) {
	s := &sweepTB{TB: t}
	in := append(append([]byte(nil), lead...), mutant...)
	fail = s.guard(func() {
		ctx := context.Background()
		crossCheckFile(s, in)
		_, perr := p.Parse(ctx, in)
		acc, aerr := p.Accept(ctx, in)
		for i := 0; i < 2 && aerr != nil; i++ { // canceled and limit/time: once more
			var ie *Error
			if errors.As(aerr, &ie) && (ie.Kind == KindCanceled || (ie.Kind == KindLimit && ie.Limit == limitTime)) {
				acc, aerr = p.Accept(ctx, in)
				continue
			}
			break
		}
		if perr != nil {
			var pe, ae *Error
			if !errors.As(perr, &pe) {
				s.Fatalf("Parse err = %v, not an *Error", perr)
			}
			if !errors.As(aerr, &ae) || ae.Kind != pe.Kind {
				s.Fatalf("Parse rejects as %s, Accept gives %v", pe.Kind, aerr)
			}
			return
		}
		if aerr == nil {
			checkStored(s, p, in, acc)
			decodesToEnd(s, acc.Original)
			checkIdempotent(s, p, acc.Original)
			crossCheckFile(s, acc.Original)
			return
		}
		var ie *Error
		switch {
		case errors.As(aerr, &ie) && ie.Kind == KindUnreadable && errors.Is(aerr, errEmptySequence):
			refusal = "u1"
		case errors.As(aerr, &ie) && ie.Kind == KindUnreadable && errors.Is(aerr, errRemovedTimestamp):
			refusal = "u2"
		default:
			s.Fatalf("Accept refused a file Parse accepts: %v", aerr)
		}
	})
	return fail, refusal
}

// TestFramingSweep (F-09 design v1.8, D-25; v1.9, D-29): change every byte of
// the probe to every other value, nothing else (no CRC repair), and check two
// ways. Part 1 (strip level): the strip and the decoder frame the mutant
// alike, and the strip either writes a file that decodes, or refuses it as U1.
// Part 2 (Accept level): the mutant chained behind sweepLead is rejected as
// Parse rejects it, accepted with a faithful stored file, or refused as U1 or
// U2; internal fails. A failure names the offset and the value.
func TestFramingSweep(t *testing.T) {
	probe := framingProbe()
	if len(probe) > 400 {
		t.Fatalf("the probe is %d bytes, over 400", len(probe))
	}
	lead := sweepLead()
	if len(lead) > 120 {
		t.Fatalf("the lead is %d bytes, over 120", len(lead))
	}
	if _, err := NewParser(DefaultLimits, nil).Parse(context.Background(), lead); err != nil {
		t.Fatalf("Parse(sweepLead): %v", err)
	}
	if msg, skipped, u1 := sweepOne(t, probe); skipped || u1 || msg != "" {
		t.Fatalf("part 1, the unchanged probe: skipped %v, u1 %v, %s", skipped, u1, msg)
	}
	p := NewParser(DefaultLimits, nil)
	if msg, ref := sweepAccept(t, p, lead, probe); msg != "" || ref != "" {
		t.Fatalf("part 2, the unchanged probe: refusal %q, %s", ref, msg)
	}

	buf := make([]byte, len(probe))
	// full[off]: every value is tried at offset off. Part 2 is slow (Accept
	// and its checks, about 1 ms per mutant), so it takes every value only for
	// record-header and definition bytes and the reduced set of the design
	// (v1.9) for the file header, payload and CRC bytes.
	full := make([]bool, len(probe))
	for rest, at, defs := probe[14:len(probe)-2], 14, new([16]*stripDef); len(rest) > 0; {
		rec, st := nextRecord(rest, defs)
		if st != framed {
			t.Fatalf("probe record at byte %d: status %d", at, st)
		}
		rec.install(defs)
		n := 1 // the record header
		if rec.definition {
			n = rec.length
		}
		for i := range n {
			full[at+i] = true
		}
		rest, at = rest[rec.length:], at+rec.length
	}
	reduced := []int{0x00, 0x01, 0x02, 0x0F, 0x10, 0x20, 0x30, 0x40, 0x7F, 0x80, 0xFE, 0xFF}
	mutants := func(onlyReduced bool, f func(off int, v int, in []byte)) {
		for off := range probe {
			vals := make([]int, 0, 256)
			if onlyReduced && !full[off] {
				vals = append(vals, reduced...)
				vals = append(vals, int(probe[off])-1, int(probe[off])+1)
				slices.Sort(vals)
				vals = slices.Compact(vals)
			} else {
				for v := range 256 {
					vals = append(vals, v)
				}
			}
			for _, v := range vals {
				if v < 0 || v > 255 || byte(v) == probe[off] {
					continue
				}
				copy(buf, probe)
				buf[off] = byte(v)
				f(off, v, buf)
			}
		}
	}

	start := time.Now()
	cases, skipped, refused, failed := 0, 0, 0, 0
	mutants(false, func(off, v int, in []byte) {
		msg, skip, u1 := sweepOne(t, in)
		cases++
		switch {
		case skip:
			skipped++
		case u1:
			refused++
		case msg != "":
			failed++
			if failed <= 20 {
				t.Errorf("part 1: offset %d value %#02x: %s", off, v, msg)
			}
		}
	})
	t.Logf("part 1: probe %d bytes, %d mutants (%d without spans), %d refused as U1, %d failed, %v", len(probe), cases, skipped, refused, failed, time.Since(start))
	if failed > 0 {
		t.Errorf("part 1: %d mutants failed", failed)
	}

	start = time.Now()
	cases, failed = 0, 0
	u1, u2 := 0, 0
	mutants(true, func(off, v int, in []byte) {
		msg, ref := sweepAccept(t, p, lead, in)
		cases++
		switch {
		case msg != "":
			failed++
			if failed <= 20 {
				t.Errorf("part 2: offset %d value %#02x: %s", off, v, msg)
			}
		case ref == "u1":
			u1++
		case ref == "u2":
			u2++
		}
	})
	t.Logf("part 2: lead %d bytes, %d mutants, %d refused as U1, %d as U2, %d failed, %v", len(lead), cases, u1, u2, failed, time.Since(start))
	if failed > 0 {
		t.Errorf("part 2: %d mutants failed", failed)
	}
}
