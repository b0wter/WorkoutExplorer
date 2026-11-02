package ingest

import (
	"encoding/binary"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/proto"
)

// The framing iterator (F-09 design v1.8, D-24, D-25; ADR-023 rev. 7 items 3,
// 3a, 3b). This file is the only place that reads a record byte of an
// upload. Each function is one row of the design's framing audit and cites
// the decoder line of that row (muktihari/fit v0.28.4, decoder/decoder.go
// unless another file is named). The strip reads every byte exactly as the
// decoder does (D-24); a divergence from it is a defect (errRecordCount, kind
// internal), and a file Parse accepts is refused only through the closed
// list of deliberate refusals (D-26, ADR-023 rev. 9 item 8a). The test-side
// crossCheckFraming compares this iterator with the decoder record by record.
// It does no I/O and writes no output.

// stripField is one field of a definition message.
type stripField struct {
	num, size, baseType uint8
	offset              int // within the data message, after its record header
}

// stripDevField is one developer field of a definition (R4, decoder.go:583-611).
// Its third byte is the developer data index, not a base type.
type stripDevField struct {
	num, size, devIndex uint8
	offset              int // within the data message, after its record header
}

// blankSpot is one blank-table field of a definition, ready to be filled in
// every data message of it (computed once per definition, audit F1).
type blankSpot struct {
	offset, size int
	readType     basetype.BaseType
	big          bool
}

// stripDef is a definition message as the strip needs it.
type stripDef struct {
	global    uint16
	arch      byte // as read (audit D2)
	big       bool // arch != proto.LittleEndian
	fields    []stripField
	devFields []stripDevField // offsets continue after the fields
	dataSize  int             // field and developer field bytes of one data message
	blank     []blankSpot
}

// frameStatus is what nextRecord found.
type frameStatus int

const (
	framed       frameStatus = iota // a whole record
	noDefinition                    // a data message of a local type with no definition
	doesNotFit                      // a definition or data message that runs past b
	frameEnd                        // b is empty
)

// framedRecord is one record of the framing.
type framedRecord struct {
	length     int  // bytes, record header included
	header     byte // the record header
	definition bool
	local      uint8     // the local message type
	def        *stripDef // the definition read (definition) or in force (data message)
}

// isDefinitionHeader: R1, decoder.go:530-531. The compressed-timestamp bit is
// tested first, because bit 6 of a compressed header is a local type bit.
func isDefinitionHeader(h byte) bool {
	const mask = proto.MesgCompressedHeaderMask | proto.MesgDefinitionMask
	return h&mask == proto.MesgDefinitionMask
}

// hasDeveloperFields: R4, decoder.go:583; read on definition headers only.
func hasDeveloperFields(h byte) bool { return h&proto.DevDataMask == proto.DevDataMask }

// bigEndian: D2, decoder.go:550-554 and proto/value_unmarshal.go. Only 0 is
// little-endian; every other architecture byte is read as big-endian.
func bigEndian(arch byte) bool { return arch != proto.LittleEndian }

// nextRecord frames the record at b[0] (R1-R5, L1) against the definitions in
// force. It does not change defs: the caller installs a framed definition
// with install (L2, decoder.go:543-548).
func nextRecord(b []byte, defs *[16]*stripDef) (framedRecord, frameStatus) {
	if len(b) == 0 {
		return framedRecord{}, frameEnd
	}
	h := b[0]
	rec := framedRecord{header: h, local: proto.LocalMesgNum(h)} // R2, R3: decoder.go:627-632
	if isDefinitionHeader(h) {
		d, n, ok := readDefinition(b)
		if !ok {
			return rec, doesNotFit
		}
		rec.definition, rec.length, rec.def = true, n, d
		return rec, framed
	}
	d := defs[rec.local] // R5: decoder.go:633-635
	if d == nil {
		return rec, noDefinition
	}
	rec.def = d
	rec.length = 1 + d.dataSize // L1: decoder.go:703, 857
	if rec.length > len(b) {
		return rec, doesNotFit
	}
	return rec, framed
}

// bytes is the record's bytes at the start of b.
func (r framedRecord) bytes(b []byte) []byte { return b[:r.length] }

// install puts a framed definition in force on its local type (L2).
func (r framedRecord) install(defs *[16]*stripDef) {
	if r.definition {
		defs[r.local] = r.def
	}
}

// readDefinition parses the definition message at b[0] (record header
// included) and returns it with its length. Audit D1-D7 and V1,
// decoder.go:538-611. The reserved byte (D1) is not read. A field of size 0
// adds no bytes (D6, 576-579). The base type is not checked (D5): a definition
// with an invalid one is an uncounted record, never framed past `records`.
func readDefinition(b []byte) (*stripDef, int, bool) {
	if len(b) < 6 {
		return nil, 0, false
	}
	d := &stripDef{arch: b[2], big: bigEndian(b[2])}
	if d.big { // D3: decoder.go:550-554
		d.global = binary.BigEndian.Uint16(b[3:5])
	} else {
		d.global = binary.LittleEndian.Uint16(b[3:5])
	}
	n := 6
	nf := int(b[5]) // D4
	if len(b) < n+3*nf {
		return nil, 0, false
	}
	for i := 0; i < nf; i++ {
		f := stripField{num: b[n], size: b[n+1], baseType: b[n+2], offset: d.dataSize}
		d.fields = append(d.fields, f)
		d.dataSize += int(f.size)
		n += 3
	}
	if hasDeveloperFields(b[0]) {
		if len(b) < n+1 {
			return nil, 0, false
		}
		nd := int(b[n])
		n++
		if len(b) < n+3*nd {
			return nil, 0, false
		}
		for i := 0; i < nd; i++ {
			f := stripDevField{num: b[n], size: b[n+1], devIndex: b[n+2], offset: d.dataSize}
			d.devFields = append(d.devFields, f)
			d.dataSize += int(f.size)
			n += 3
		}
	}
	d.blank = blankSpots(d)
	return d, n, true
}

// blankSpots lists, once per definition, where the blank-table fields of its
// message lie (D7: a field number that appears twice is listed twice; D6: a
// field of size 0 has no bytes and no spot).
func blankSpots(d *stripDef) []blankSpot {
	nums := blankTable[d.global]
	if nums == nil {
		return nil
	}
	var spots []blankSpot
	for _, f := range d.fields {
		if f.size == 0 {
			continue
		}
		for _, n := range nums {
			if f.num == n {
				spots = append(spots, blankSpot{
					offset: f.offset, size: int(f.size),
					readType: readType(d.global, f.num, f.baseType), big: d.big,
				})
			}
		}
	}
	return spots
}
