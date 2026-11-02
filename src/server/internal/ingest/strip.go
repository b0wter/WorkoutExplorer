package ingest

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/muktihari/fit/kit/hash/crc16"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/proto"
)

// StripCounts says what the strip changed: counts only, never values.
type StripCounts struct {
	BlankedFields   int // field values overwritten with the invalid value
	RemovedMessages int // data messages removed
}

// errRecordCount is the strip's fail-closed sentinel (ADR-023 rev. 5 item 8):
// the framing and the walk's record count, or the end of the decoder's loop,
// disagree. Its text names the sequence's byte offset and what disagreed,
// never a value from the file.
var errRecordCount = errors.New("record count")

// errEmptySequence is the deliberate refusal U1 (F-09 design v1.9, D-27;
// ADR-023 rev. 8 item 8a): a complete span with no record. A sequence with a
// data size of 0 reads as unclosed, so it has no faithful stored form; it is
// refused before any byte is written. Its text names the sequence's offset,
// never a value from the file.
var errEmptySequence = errors.New("empty complete sequence")

// errRemovedTimestamp is the deliberate refusal U2 (D-28, OQ-54): removing a
// message with a timestamp changes what a later compressed-timestamp record
// of the same sequence means.
var errRemovedTimestamp = errors.New("removed timestamp before a compressed-timestamp record")

// emptyComplete is the one predicate of U1: strip and stripSeq use it.
func emptyComplete(sp seqSpan) bool { return sp.status == seqComplete && sp.records == 0 }

// stripSeq rewrites one sequence (ADR-023 items 3, 5, 6, 6a, rev. 5) and
// appends it to out. Both statuses write exactly the first sp.records
// records, and nothing after them. A complete sequence is read up to the end
// of the upload, because the decoder reads its last record in full even when
// the header's data size ends inside it (D-21); it then gets its data size
// and CRCs written. A truncated one is written in the unclosed form.
func stripSeq(out, data []byte, sp seqSpan) ([]byte, StripCounts, error) {
	var counts StripCounts
	if emptyComplete(sp) { // U1: never write the ambiguous 16 bytes (D-27)
		return nil, counts, fmt.Errorf("%w: sequence at byte %d", errEmptySequence, sp.pos)
	}
	hdrAt := len(out)
	out = append(out, data[sp.pos:sp.pos+sp.headerSize]...)
	truncated := sp.status == seqTruncated
	regionEnd := len(data)
	if truncated {
		regionEnd = sp.dataEnd
	}
	region := data[sp.pos+sp.headerSize : regionEnd]
	dataEndRel := sp.dataEnd - (sp.pos + sp.headerSize)
	var defs [16]*stripDef

	fail := func(what string) error {
		return fmt.Errorf("%w: sequence at byte %d: %s", errRecordCount, sp.pos, what)
	}
	countErr := func(n int) error {
		return fail(fmt.Sprintf("walk counted %d records, framing found %d", sp.records, n))
	}

	rest := region // the bytes not yet framed
	n, lastStart := 0, 0
	for n < sp.records {
		lastStart = len(region) - len(rest)
		rec, st := nextRecord(rest, &defs)
		if st != framed {
			return nil, counts, countErr(n)
		}
		rec.install(&defs)
		switch {
		case rec.definition:
			out = append(out, rec.bytes(rest)...)
		case removeTable[rec.def.global]:
			counts.RemovedMessages++
		default:
			start := len(out)
			out = append(out, rec.bytes(rest)...)
			counts.BlankedFields += blankFields(out[start:], rec.def)
		}
		rest = rest[rec.length:]
		n++
	}
	if !truncated {
		// Loop-end check: the framing must stop where the decoder's loop
		// stopped. (a) the last record starts before the declared data end;
		// (b) the framing reached it, or the next record runs past the upload.
		if sp.records > 0 && lastStart >= dataEndRel {
			return nil, counts, fail("the last record starts at or after the end the header declares")
		}
		at := len(region) - len(rest)
		if at < dataEndRel && !runsPast(rest, &defs) {
			return nil, counts, fail("the framing stops before the end the header declares and the next record fits")
		}
	}
	hdr := out[hdrAt : hdrAt+sp.headerSize]
	if truncated {
		// The unclosed form of F-08 R-c: data size 0, header CRC 0, no file CRC.
		binary.LittleEndian.PutUint32(hdr[4:8], 0)
		if sp.headerSize == 14 {
			hdr[12], hdr[13] = 0, 0
		}
		return out, counts, nil
	}
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(out)-hdrAt-sp.headerSize))
	if sp.headerSize == 14 {
		c := crc16.New()
		_, _ = c.Write(hdr[:12])
		binary.LittleEndian.PutUint16(hdr[12:14], c.Sum16())
	}
	// File CRC over the header and the records, for both header sizes. The
	// decoder resets its CRC after a 14-byte header and checks the records
	// alone (decoder.go:499-502, 934-946). The result is the same: a CRC-16
	// over a block followed by that block's correct CRC, little-endian, is 0,
	// which is the reset state, and the header CRC written above is the
	// correct CRC of bytes 0-11 (audit H6; decodesToEnd proves it on every
	// stored file).
	c := crc16.New()
	_, _ = c.Write(out[hdrAt:])
	return binary.LittleEndian.AppendUint16(out, c.Sum16()), counts, nil
}

// runsPast reports whether the record at the start of b does not fit in b:
// a definition whose bytes do not fit, or a data message of a known local
// type whose length does not fit. A data message without a definition does
// not (the decoder would have rejected it, so the walk said truncated).
func runsPast(b []byte, defs *[16]*stripDef) bool {
	_, st := nextRecord(b, defs)
	return st == doesNotFit
}

// blankFields overwrites the blank-table fields of one data message
// (record header first) and returns how many values changed.
func blankFields(rec []byte, d *stripDef) int {
	changed := 0
	for _, sp := range d.blank {
		v := rec[1+sp.offset : 1+sp.offset+sp.size]
		before := append([]byte(nil), v...)
		fillInvalid(v, sp.readType, sp.big)
		if string(before) != string(v) {
			changed++
		}
	}
	return changed
}

// baseTypeSize is the size of one element, and its invalid value as a
// little-endian integer, per FIT base type number (ADR-023 item 4).
var baseTypes = map[uint8]struct {
	size    int
	invalid uint64
}{
	0: {1, 0xFF}, 1: {1, 0x7F}, 2: {1, 0xFF}, 3: {2, 0x7FFF}, 4: {2, 0xFFFF}, 5: {4, 0x7FFFFFFF},
	6: {4, 0xFFFFFFFF}, 8: {4, 0xFFFFFFFF}, 9: {8, 0xFFFFFFFFFFFFFFFF}, 10: {1, 0}, 11: {2, 0}, 12: {4, 0},
	13: {1, 0xFF}, 14: {8, 0x7FFFFFFFFFFFFFFF}, 15: {8, 0xFFFFFFFFFFFFFFFF}, 16: {8, 0},
}

// fillInvalid writes the fill for a field the decoder reads as t (D-23,
// ADR-023 rev. 6 item 4): t's invalid value for each whole element, in the
// definition's byte order, and 0x00 for the bytes after the last whole
// element, for a field shorter than one element and for a string. A short
// field of a non-z type then reads as 0: no pattern of fewer bytes reads as
// invalid.
func fillInvalid(v []byte, t basetype.BaseType, big bool) {
	clear(v)
	bt, ok := baseTypes[uint8(t&basetype.BaseTypeNumMask)]
	if !ok || t&basetype.BaseTypeNumMask == basetype.String { // string, or a type the decoder would have refused
		return
	}
	for i := 0; i+bt.size <= len(v); i += bt.size {
		e := v[i : i+bt.size]
		for k := range e {
			shift := 8 * k
			if big {
				shift = 8 * (bt.size - 1 - k)
			}
			e[k] = byte(bt.invalid >> shift)
		}
	}
}

// strip rewrites the sequences of data listed in spans and drops every byte
// after the last one (ADR-023 item 6). It is the one production loop. stop,
// if not nil, is called before each sequence; a non-nil result ends the loop
// with that error and no output.
func strip(data []byte, spans []seqSpan, stop func() error) ([]byte, StripCounts, error) {
	for _, sp := range spans { // U1 first: no byte is written, and stop is not called (D-27)
		if emptyComplete(sp) {
			return nil, StripCounts{}, fmt.Errorf("%w: sequence at byte %d", errEmptySequence, sp.pos)
		}
	}
	out := make([]byte, 0, len(data)+2*len(spans))
	var total StripCounts
	for _, sp := range spans {
		if stop != nil {
			if err := stop(); err != nil {
				return nil, total, err
			}
		}
		var c StripCounts
		var err error
		out, c, err = stripSeq(out, data, sp)
		if err != nil {
			return nil, total, err
		}
		total.BlankedFields += c.BlankedFields
		total.RemovedMessages += c.RemovedMessages
	}
	return out, total, nil
}

// removedTimestampDependency reports whether the strip's removal of a message
// can change what a later compressed-timestamp record means (U2, F-09 design
// v1.9, D-28; OQ-54): a kept data message with a compressed-timestamp header
// that follows, in the same span, a removed data message whose definition has
// field 253 (timestamp) with a size above 0 or that has a compressed-timestamp
// header itself (its header also moves the decoder's clock). The decoder computes a
// compressed timestamp from the last timestamp it read, and each sequence
// gets a fresh decoder (audit X1), so a removed message in another span does
// not count. It is pure, walks the first sp.records records as stripSeq
// does, reads only through frame.go, and runs only after a reference
// mismatch, never on the success path.
func removedTimestampDependency(data []byte, spans []seqSpan) bool {
	for _, sp := range spans {
		if emptyComplete(sp) {
			continue
		}
		regionEnd := len(data)
		if sp.status == seqTruncated {
			regionEnd = sp.dataEnd
		}
		rest := data[sp.pos+sp.headerSize : regionEnd]
		var defs [16]*stripDef
		removedTS := false
		for n := 0; n < sp.records; n++ {
			rec, st := nextRecord(rest, &defs)
			if st != framed {
				break
			}
			rec.install(&defs)
			if !rec.definition {
				if removeTable[rec.def.global] {
					removedTS = removedTS || hasTimestamp(rec.def) ||
						rec.header&proto.MesgCompressedHeaderMask != 0 // a compressed header also moves the clock (QA-09-r4-1)
				} else if removedTS && rec.header&proto.MesgCompressedHeaderMask != 0 {
					return true
				}
			}
			rest = rest[rec.length:]
		}
	}
	return false
}

// hasTimestamp: the definition declares field 253 with a size above 0.
func hasTimestamp(d *stripDef) bool {
	for _, f := range d.fields {
		if f.num == proto.FieldNumTimestamp && f.size > 0 {
			return true
		}
	}
	return false
}
