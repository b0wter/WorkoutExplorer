package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/factory"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/fieldnum"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/corpustest"
	"workout-explorer/internal/workout"
)

// ---- helpers ----------------------------------------------------------------

// stRecord is one record of a sequence's data region: a definition message or
// a data message, with the global number of its definition.
type stRecord struct {
	global uint16
	def    bool
	raw    []byte
	fields []stripField // of the definition in force; data messages only
	big    bool
}

// stSeq is one FIT sequence split into its parts.
type stSeq struct {
	header  []byte
	records []stRecord
	tail    []byte // bytes after the last complete record of the data region (an incomplete record)
	crc     []byte // the two file CRC bytes, if present
	rest    []byte // bytes after the sequence
}

// splitSeq splits the sequence at the start of b with the same framing the
// strip uses, but written independently of stripSeq: it follows the header's
// data size, or, for data size 0, the rest of b.
func splitSeq(t *testing.T, b []byte) stSeq {
	t.Helper()
	hs := int(b[0])
	size := int(binary.LittleEndian.Uint32(b[4:8]))
	s := stSeq{header: b[:hs]}
	region := b[hs:]
	if size > 0 && hs+size <= len(b) {
		region = b[hs : hs+size]
		after := b[hs+size:]
		if len(after) >= 2 {
			s.crc, s.rest = after[:2], after[2:]
		} else {
			s.rest = after
		}
	}
	var defs [16]*stripDef
	for at := 0; at < len(region); {
		h := region[at]
		var rec stRecord
		var n int
		switch {
		case h&0x80 != 0, h&0x40 == 0:
			lt := h & 0x0F
			if h&0x80 != 0 {
				lt = (h >> 5) & 3
			}
			d := defs[lt]
			if d == nil {
				t.Fatalf("data message without definition at %d", at)
			}
			n = 1 + d.dataSize
			if at+n > len(region) {
				s.tail = region[at:]
				return s
			}
			rec = stRecord{global: d.global, fields: d.fields, big: d.big}
		default:
			d, dn, ok := readDefinition(region[at:])
			if !ok {
				s.tail = region[at:]
				return s
			}
			defs[h&0x0F] = d
			n = dn
			rec = stRecord{global: d.global, def: true}
		}
		rec.raw = region[at : at+n]
		s.records = append(s.records, rec)
		at += n
	}
	return s
}

// checkCRCs fails unless the header CRC (14-byte header) and the file CRC of
// the sequence at the start of b are right.
func checkCRCs(t *testing.T, b []byte) {
	t.Helper()
	hs := int(b[0])
	size := int(binary.LittleEndian.Uint32(b[4:8]))
	if hs == 14 && binary.LittleEndian.Uint16(b[12:14]) != fitCRC(0, b[:12]) {
		t.Errorf("wrong header CRC")
	}
	if len(b) < hs+size+2 {
		t.Fatalf("no file CRC: %d bytes for header %d + data %d", len(b), hs, size)
	}
	if got, want := binary.LittleEndian.Uint16(b[hs+size:]), fitCRC(0, b[:hs+size]); got != want {
		t.Errorf("file CRC %#x, want %#x", got, want)
	}
}

// decodeAll decodes every sequence of b with the library's full decoder
// (checksums included) and returns the messages in order.
func decodeAll(t *testing.T, b []byte) []proto.Message {
	t.Helper()
	var out []proto.Message
	dec := decoder.New(bytes.NewReader(b))
	for dec.Next() {
		fit, err := dec.Decode()
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		out = append(out, fit.Messages...)
	}
	return out
}

func fieldOf(m proto.Message, num byte) (proto.Field, bool) {
	for _, f := range m.Fields {
		if f.Num == num {
			return f, true
		}
	}
	return proto.Field{}, false
}

// absent reports whether field num of m is missing or has the invalid value.
func absent(m proto.Message, num byte) bool {
	f, ok := fieldOf(m, num)
	return !ok || !f.Value.Valid(f.BaseType)
}

func uploadOf(t *testing.T, name string) []byte {
	t.Helper()
	return readSynthetic(t, name)
}

func accept(t *testing.T, data []byte) Accepted {
	t.Helper()
	acc, err := testParser.Accept(context.Background(), data)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	return acc
}

func reportOf(t *testing.T, b []byte) []byte {
	t.Helper()
	r, err := workout.ReportJSON(mustParse(t, b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// markers are the invented values of identifying-data.fit that must not be in
// a stored file (F-09 design, TestStripIdentifyingData).
func markers() [][]byte {
	var out [][]byte
	for _, s := range []string{
		"Marker Person", "Marker Home Place", "Marker Rival Person", "MkrGid",
		"MkrAntRx", "MkrAntTx", "MarkerLeaderOne", "MarkerLeaderTwo", // v1.4
	} {
		out = append(out, []byte(s))
	}
	for _, n := range []uint32{
		3_735_928_559, 2_882_400_001, 2_882_400_002, 2_882_400_003, 424_242,
		// v1.4: segment_id.device_id, the segment account keys (OQ-52), the leaders'
		// activity and group keys, and the tank pods' sensor.
		2_882_400_004, 2_882_400_005, 2_882_400_006, 2_882_400_007, 2_882_400_008, 2_882_400_009, 2_882_400_010, 2_882_400_011,
	} {
		out = append(out, []byte(strconv.FormatUint(uint64(n), 10)), binary.LittleEndian.AppendUint32(nil, n))
	}
	// The ANT device numbers are 16-bit fields: their little-endian form is 2 bytes.
	for _, n := range []uint16{54_321, 12_345, 23_456} {
		out = append(out, []byte(strconv.Itoa(int(n))), binary.LittleEndian.AppendUint16(nil, n))
	}
	return out
}

// markerFields is how many fields of identifying-data.fit carry each binary
// marker. The upload must contain the marker exactly that often, so a marker
// can never be found elsewhere by chance (F-09 design, Tests).
func markerFields() map[string]int {
	le32 := func(n uint32) string { return string(binary.LittleEndian.AppendUint32(nil, n)) }
	le16 := func(n uint16) string { return string(binary.LittleEndian.AppendUint16(nil, n)) }
	return map[string]int{
		le32(3_735_928_559): 1, le32(2_882_400_001): 1, le32(2_882_400_002): 1, le32(2_882_400_003): 1,
		le32(424_242):       1,
		le32(2_882_400_004): 1, le32(2_882_400_005): 1, le32(2_882_400_006): 1, le32(2_882_400_007): 1,
		le32(2_882_400_008): 1, le32(2_882_400_009): 1, le32(2_882_400_010): 1,
		le32(2_882_400_011): 2, // tank_update and tank_summary
		le16(54_321):        2, // device_info.ant_device_number and hrm_profile.hrm_ant_id
		le16(12_345):        1, le16(23_456): 1,
		"Marker Person": 1, "Marker Home Place": 1, "Marker Rival Person": 1, "MkrGid": 1,
		"MkrAntRx": 1, "MkrAntTx": 1, "MarkerLeaderOne": 1, "MarkerLeaderTwo": 1,
	}
}

func isDecimal(b []byte) bool {
	_, err := strconv.ParseUint(string(b), 10, 64)
	return err == nil && len(b) > 4
}

// ---- TestStripIdentifyingData (AC 6) -----------------------------------------

func TestStripIdentifyingData(t *testing.T) {
	upload := uploadOf(t, "identifying-data.fit")
	for _, m := range markers() {
		// The decimal text forms are not in a binary file; they are checked in logs and the database.
		if isDecimal(m) {
			continue
		}
		if !bytes.Contains(upload, m) {
			t.Fatalf("marker %q is not in the upload: the test would prove nothing", m)
		}
	}
	for m, want := range markerFields() {
		if got := bytes.Count(upload, []byte(m)); got != want {
			t.Errorf("marker %q occurs %d times in the upload, want %d (carrying fields only)", m, got, want)
		}
	}
	acc := accept(t, upload)
	stored := acc.Original
	// 13 = 7 (serial numbers, ANT device numbers, hrm_ant_id) + device_number, device_id,
	// the two leader fields and the two sensor fields; +3 account keys (OQ-52: blank) = 16.
	if want := (StripCounts{BlankedFields: 16, RemovedMessages: 5}); acc.Stripped != want {
		t.Errorf("counts = %+v, want %+v", acc.Stripped, want)
	}
	for _, m := range markers() {
		if bytes.Contains(stored, m) {
			t.Errorf("marker %q is still in the stored file", m)
		}
	}
	checkCRCs(t, stored)

	// Decoded with the library: blanks, removals, kept values.
	msgs := decodeAll(t, stored)
	var deviceInfos, zones, inZone, sports, devIDs, descs int
	var channels, segIDs, segFiles, tankUpdates, tankSummaries int
	str := func(m proto.Message, n byte) string {
		if f, ok := fieldOf(m, n); ok && f.Value.Valid(f.BaseType) {
			return f.Value.String()
		}
		return ""
	}
	for _, m := range msgs {
		// Every blank-table field of every message is invalid, whatever the message (table-driven).
		for _, n := range blankTable[uint16(m.Num)] {
			if !absent(m, n) {
				t.Errorf("message %d field %d is not blank", m.Num, n)
			}
		}
		switch m.Num {
		case mesgnum.UserProfile, mesgNumLocation, mesgnum.SegmentLeaderboardEntry, mesgnum.AntRx, mesgnum.AntTx:
			t.Errorf("data message %d is still in the stored file", m.Num)
		case mesgnum.AntChannelId:
			// Transmission type, device type and device index stay (OQ-46).
			if f, ok := fieldOf(m, fieldnum.AntChannelIdTransmissionType); !ok || f.Value.Uint8() != 1 {
				t.Errorf("ant_channel_id.transmission_type changed")
			}
			if f, ok := fieldOf(m, fieldnum.AntChannelIdDeviceType); !ok || f.Value.Uint8() != 120 {
				t.Errorf("ant_channel_id.device_type changed")
			}
			if f, ok := fieldOf(m, fieldnum.AntChannelIdDeviceIndex); !ok || f.Value.Uint8() != 1 {
				t.Errorf("ant_channel_id.device_index changed")
			}
			channels++
		case mesgnum.SegmentId:
			if str(m, fieldnum.SegmentIdName) != "Marker Segment" || str(m, fieldnum.SegmentIdUuid) != "marker-segment-uuid" {
				t.Errorf("segment_id name or uuid changed")
			}
			segIDs++
		case mesgnum.SegmentFile:
			if str(m, fieldnum.SegmentFileFileUuid) != "marker-segment-uuid" {
				t.Errorf("segment_file.file_uuid changed")
			}
			segFiles++
		case mesgnum.TankUpdate:
			if f, ok := fieldOf(m, fieldnum.TankUpdatePressure); !ok || f.Value.Uint16() != 20_000 {
				t.Errorf("tank_update.pressure changed")
			}
			tankUpdates++
		case mesgnum.TankSummary:
			if f, ok := fieldOf(m, fieldnum.TankSummaryVolumeUsed); !ok || f.Value.Uint32() != 1_000 {
				t.Errorf("tank_summary.volume_used changed")
			}
			tankSummaries++
		case mesgnum.FileId:
			if !absent(m, fieldnum.FileIdSerialNumber) {
				t.Errorf("file_id serial number not blank")
			}
		case mesgnum.DeviceInfo:
			if !absent(m, fieldnum.DeviceInfoSerialNumber) || !absent(m, fieldnum.DeviceInfoAntDeviceNumber) {
				t.Errorf("device_info %d: serial number or ANT device number not blank", deviceInfos)
			}
			// Transmission types stay (OQ-46).
			if f, ok := fieldOf(m, 20); ok && f.Value.Valid(f.BaseType) {
				if want := []uint8{1, 5}[deviceInfos-1]; f.Value.Uint8() != want {
					t.Errorf("device_info %d transmission type = %d, want %d", deviceInfos, f.Value.Uint8(), want)
				}
			} else if deviceInfos > 0 {
				t.Errorf("device_info %d lost its transmission type", deviceInfos)
			}
			deviceInfos++
		case mesgnum.HrmProfile:
			if !absent(m, fieldnum.HrmProfileHrmAntId) {
				t.Errorf("hrm_ant_id not blank")
			}
			if f, ok := fieldOf(m, fieldnum.HrmProfileHrmAntIdTransType); !ok || f.Value.Uint8() != 1 {
				t.Errorf("hrm_ant_id_trans_type changed")
			}
		case mesgnum.Sport:
			if f, ok := fieldOf(m, fieldnum.SportName); !ok || f.Value.String() != "Marker Sport Name" {
				t.Errorf("sport name changed")
			}
			sports++
		case mesgnum.ZonesTarget:
			zones++
		case mesgnum.TimeInZone:
			inZone++
		case mesgnum.DeveloperDataId:
			devIDs++
		case mesgnum.FieldDescription:
			descs++
		}
	}
	if deviceInfos != 3 || zones != 1 || inZone != 1 || sports != 1 || devIDs != 1 || descs != 1 ||
		channels != 1 || segIDs != 1 || segFiles != 1 || tankUpdates != 1 || tankSummaries != 1 {
		t.Errorf("kept messages: device_info %d zones_target %d time_in_zone %d sport %d developer_data_id %d field_description %d "+
			"ant_channel_id %d segment_id %d segment_file %d tank_update %d tank_summary %d",
			deviceInfos, zones, inZone, sports, devIDs, descs, channels, segIDs, segFiles, tankUpdates, tankSummaries)
	}

	// Byte level: every other record is equal, in order; only blank-table
	// fields differ; removed messages are gone, their definitions are not;
	// zone messages are byte for byte as uploaded (OQ-50).
	up, st := splitSeq(t, upload), splitSeq(t, stored)
	var kept []stRecord
	removedDefs, storedDefs := 0, 0
	for _, r := range up.records {
		if r.def {
			if removeTable[r.global] {
				removedDefs++
			}
			kept = append(kept, r)
			continue
		}
		if !removeTable[r.global] {
			kept = append(kept, r)
		}
	}
	for _, r := range st.records {
		if r.def && removeTable[r.global] {
			storedDefs++
		}
	}
	if removedDefs != 5 || storedDefs != 5 {
		t.Errorf("definitions of the removed messages: upload %d, stored %d, want 5 and 5", removedDefs, storedDefs)
	}
	if len(kept) != len(st.records) {
		t.Fatalf("stored has %d records, want %d", len(st.records), len(kept))
	}
	zoneRecords := 0
	for i, r := range kept {
		got := st.records[i]
		if !r.def && (r.global == uint16(mesgnum.ZonesTarget) || r.global == uint16(mesgnum.TimeInZone)) {
			zoneRecords++
			if !bytes.Equal(r.raw, got.raw) {
				t.Errorf("zone message %d changed", r.global)
			}
			continue
		}
		if bytes.Equal(r.raw, got.raw) {
			continue
		}
		if r.def || got.global != r.global {
			t.Errorf("record %d differs", i)
			continue
		}
		diffOnlyIn(t, i, r, got)
	}
	if zoneRecords != 2 {
		t.Errorf("zone data messages in the upload: %d, want 2", zoneRecords)
	}

	// The same workout, and the same bytes for the developer field.
	if !bytes.Equal(reportOf(t, upload), reportOf(t, stored)) {
		t.Errorf("the stored file parses to a different report")
	}
	if !bytes.Contains(stored, []byte("marker_dev_field")) {
		t.Errorf("developer field description lost")
	}
}

// diffOnlyIn fails unless the two data messages differ only inside fields of
// the blank table.
func diffOnlyIn(t *testing.T, i int, a, b stRecord) {
	t.Helper()
	allowed := make([]bool, len(a.raw))
	off := 1
	for _, f := range a.fields {
		for _, n := range blankTable[a.global] {
			if n == f.num {
				for k := 0; k < int(f.size); k++ {
					allowed[off+k] = true
				}
			}
		}
		off += int(f.size)
	}
	for k := range a.raw {
		if a.raw[k] != b.raw[k] && !allowed[k] {
			t.Errorf("record %d (message %d) differs at byte %d outside the blank table", i, a.global, k)
			return
		}
	}
}

// ---- TestStripFrames (AC 7) --------------------------------------------------

func TestStripFrames(t *testing.T) {
	base := uploadOf(t, "identifying-data.fit")
	pristine := accept(t, base).Original

	// A cut at 60 % that falls inside a record.
	bounds := recordBounds(t, base)
	cut := len(base) * 6 / 10
	for bounds[cut] {
		cut++
	}
	cutFile := append([]byte(nil), base[:cut]...)
	sizeZero := append([]byte(nil), cutFile...)
	binary.LittleEndian.PutUint32(sizeZero[4:8], 0)
	sizeZero[12], sizeZero[13] = 0, 0

	unclosed := func(name string, upload []byte) []byte {
		t.Helper()
		acc := accept(t, upload)
		s := acc.Original
		if got := binary.LittleEndian.Uint32(s[4:8]); got != 0 {
			t.Errorf("%s: data size %d, want 0", name, got)
		}
		if s[12] != 0 || s[13] != 0 {
			t.Errorf("%s: header CRC not 0", name)
		}
		sp := splitSeq(t, s)
		if len(sp.tail) != 0 || len(sp.crc) != 0 || len(sp.rest) != 0 {
			t.Errorf("%s: stored sequence is not exactly header and complete records (tail %d, crc %d, rest %d)", name, len(sp.tail), len(sp.crc), len(sp.rest))
		}
		if len(s) >= len(upload) {
			t.Errorf("%s: the incomplete record was not dropped (%d >= %d bytes)", name, len(s), len(upload))
		}
		w1, w2 := mustParse(t, upload), mustParse(t, s)
		if !w1.Integrity.Truncated || !w2.Integrity.Truncated || w1.Integrity.DataEnd != w2.Integrity.DataEnd {
			t.Errorf("%s: truncated %v/%v data_end %d/%d", name, w1.Integrity.Truncated, w2.Integrity.Truncated, w1.Integrity.DataEnd, w2.Integrity.DataEnd)
		}
		if !bytes.Equal(reportOf(t, upload), reportOf(t, s)) {
			t.Errorf("%s: report differs", name)
		}
		return s
	}
	a := unclosed("cut at 60 %", cutFile)
	b := unclosed("data size 0", sizeZero)
	if !bytes.Equal(a, b) {
		t.Errorf("the cut file and its data size 0 form are stored differently")
	}

	// Damaged CRCs are repaired.
	badFile := append([]byte(nil), base...)
	badFile[len(badFile)-1] ^= 0xFF
	badHeader := append([]byte(nil), base...)
	badHeader[12] ^= 0xFF
	badHeader[13] ^= 0xFF
	for name, upload := range map[string][]byte{"wrong file CRC": badFile, "wrong header CRC": badHeader, "no file CRC": base[:len(base)-2]} {
		got := accept(t, upload).Original
		checkCRCs(t, got)
		if !bytes.Equal(got, pristine) {
			t.Errorf("%s: stored differently from the clean file", name)
		}
	}

	// Trailing bytes are dropped.
	if got := accept(t, append(append([]byte(nil), base...), make([]byte, 64)...)).Original; !bytes.Equal(got, pristine) {
		t.Errorf("trailing zeros: stored differently from the clean file")
	}

	// A chained file: both sequences are stripped and framed.
	chain := append(append([]byte(nil), base...), base...)
	got := accept(t, chain).Original
	if want := append(append([]byte(nil), pristine...), pristine...); !bytes.Equal(got, want) {
		t.Errorf("chained twice: stored differently from two clean files")
	}
	checkCRCs(t, got[len(pristine):])

	// A settings sequence behind the activity: kept, only its user_profile goes.
	settings := encodeSettings(t)
	got = accept(t, append(append([]byte(nil), base...), settings...)).Original
	if !bytes.Equal(got[:len(pristine)], pristine) {
		t.Fatalf("settings: the activity sequence changed")
	}
	tail := got[len(pristine):]
	checkCRCs(t, tail)
	for _, m := range decodeAll(t, tail) {
		if m.Num == mesgnum.UserProfile {
			t.Errorf("settings: user_profile kept")
		}
	}
	if w := mustParse(t, got); w.Integrity.SkippedSequences != 1 || w.Integrity.Sequences != 2 {
		t.Errorf("settings: sequences %d skipped %d, want 2 and 1", w.Integrity.Sequences, w.Integrity.SkippedSequences)
	}
	if n := len(splitSeq(t, tail).records); n >= len(splitSeq(t, settings).records) {
		t.Errorf("settings: no record was removed")
	}

	// D-20, a rejected developer field: the decoder accepts a field
	// description with an invalid base type and a definition that uses it, and
	// rejects the first data message. The stored sequence ends right before
	// that data message and keeps the description and the definition.
	t.Run("rejected developer field", func(t *testing.T) {
		starts, recs := recordStarts(t, base)
		var descDef, descData = -1, -1
		var recIdx []int
		for i, r := range recs {
			switch {
			case r.global == uint16(mesgnum.FieldDescription) && r.def && descDef < 0:
				descDef = i
			case r.global == uint16(mesgnum.FieldDescription) && !r.def && descData < 0:
				descData = i
			case r.global == uint16(mesgnum.Record) && !r.def:
				recIdx = append(recIdx, i)
			}
		}
		ins := recIdx[len(recIdx)*6/10]
		free := freeLocals(recs, ins)
		if len(free) < 2 {
			t.Fatalf("fewer than two free local types: %v", free)
		}
		lDesc, lRec := free[0], free[1]
		// The copied field_description: developer data index 0, field number 1
		// (the file's own is 0), base type 0x30.
		def := append([]byte(nil), recs[descDef].raw...)
		def[0] = def[0]&^0x0F | lDesc
		msg := append([]byte(nil), recs[descData].raw...)
		msg[0] = msg[0]&^0x0F | lDesc
		var devIdx, num, bt = -1, -1, -1
		for _, f := range recs[descData].fields {
			switch f.num {
			case 0:
				devIdx = f.offset
			case 1:
				num = f.offset
			case 2:
				bt = f.offset
			}
		}
		if devIdx < 0 || num < 0 || bt < 0 || msg[1+devIdx] != 0 || msg[1+num] != 0 {
			t.Fatalf("unexpected field_description in identifying-data.fit: % x", recs[descData].raw)
		}
		msg[1+num], msg[1+bt] = 1, 0x30
		// A record definition on another free local type with only developer field 1 (size 1).
		recDef := []byte{0x60 | lRec, 0, 0, byte(mesgnum.Record), 0, 0, 1, 1, 1, 0}
		rejected := []byte{lRec, 7}
		hs := int(base[0])
		end := hs + int(binary.LittleEndian.Uint32(base[4:8]))
		var up []byte
		up = append(up, base[:starts[ins]]...)
		up = append(up, def...)
		up = append(up, msg...)
		up = append(up, recDef...)
		cutAt := len(up)
		up = append(up, rejected...)
		up = append(up, base[starts[ins]:end]...)
		up = closeSeq(up)

		w := mustParse(t, up)
		if !w.Integrity.Truncated || w.Integrity.CRCFailed {
			t.Fatalf("upload: truncated %v, crc_failed %v; the decoder should end the sequence at the data message", w.Integrity.Truncated, w.Integrity.CRCFailed)
		}
		acc := accept(t, up)
		// The stored records are the upload's up to the data message, less the removed messages.
		_, upRecs := recordStarts(t, up)
		var want []stRecord
		for _, r := range upRecs[:ins+3] {
			if !r.def && removeTable[r.global] {
				continue
			}
			want = append(want, r)
		}
		if cutAt != starts[ins]+len(def)+len(msg)+len(recDef) {
			t.Fatalf("cutAt is wrong")
		}
		if binary.LittleEndian.Uint32(acc.Original[4:8]) != 0 || acc.Original[12] != 0 || acc.Original[13] != 0 {
			t.Errorf("not in the unclosed form")
		}
		sp := splitSeq(t, acc.Original)
		if got, w := shapesOf(sp.records), shapesOf(want); !slices.Equal(got, w) {
			t.Errorf("stored %d records, want the %d before the rejected data message", len(got), len(w))
		}
		if n := len(sp.records); n < 3 || !bytes.Equal(sp.records[n-1].raw, recDef) || !bytes.Equal(sp.records[n-2].raw, msg) || !bytes.Equal(sp.records[n-3].raw, def) {
			t.Errorf("the inserted description and definition are not the last stored records")
		}
		decodesToEnd(t, acc.Original)
		if !bytes.Equal(reportOf(t, up), reportOf(t, acc.Original)) {
			t.Errorf("report differs")
		}
	})

	// A 12-byte header gets no header CRC.
	twelve := rebuildSeq(base, 12)
	got = accept(t, twelve).Original
	if got[0] != 12 || len(got) != len(pristine)-2 {
		t.Errorf("12-byte header: header size %d, %d bytes (want 12, %d)", got[0], len(got), len(pristine)-2)
	}
	checkCRCs(t, got)
	if !bytes.Equal(reportOf(t, twelve), reportOf(t, got)) {
		t.Errorf("12-byte header: report differs")
	}
}

// recordBounds returns the offsets (in b) at which a record starts, plus the end.
func recordBounds(t *testing.T, b []byte) map[int]bool {
	t.Helper()
	out := map[int]bool{}
	sp := splitSeq(t, b)
	at := len(sp.header)
	for _, r := range sp.records {
		out[at] = true
		at += len(r.raw)
	}
	out[at] = true
	return out
}

// rebuildSeq rewrites the sequence in b with a header of the given size and
// correct CRCs.
func rebuildSeq(b []byte, headerSize int) []byte {
	hs := int(b[0])
	size := int(binary.LittleEndian.Uint32(b[4:8]))
	h := append([]byte(nil), b[:headerSize]...)
	h[0] = byte(headerSize)
	out := append(h, b[hs:hs+size]...)
	if headerSize == 14 {
		binary.LittleEndian.PutUint16(out[12:], fitCRC(0, out[:12]))
	}
	return binary.LittleEndian.AppendUint16(out, fitCRC(0, out))
}

func encodeSettings(t *testing.T) []byte {
	t.Helper()
	msgs := []proto.Message{
		mesgdef.NewFileId(nil).SetType(typedef.FileSettings).SetManufacturer(typedef.ManufacturerDevelopment).SetProduct(1).ToMesg(nil),
		mesgdef.NewUserProfile(nil).SetFriendlyName("Marker Settings Person").SetWeightScaled(70).ToMesg(nil),
	}
	return encodeFIT(t, msgs...)
}

// ---- hand-built sequences ----------------------------------------------------

// handSeq builds a sequence with a 14-byte header from raw records.
func handSeq(records ...[]byte) []byte {
	return (&fitBuilder{body: bytes.Join(records, nil)}).bytes()
}

// rawDef is a definition message in either byte order.
func rawDef(local byte, big bool, global uint16, fields ...[3]byte) []byte {
	arch := byte(0)
	if big {
		arch = 1
	}
	return rawDefArch(local, arch, global, fields...)
}

// rawDefArch is a definition message with any architecture byte. The decoder
// reads the global number little-endian for 0 and big-endian for every other
// value (audit D2), and so does this.
func rawDefArch(local, arch byte, global uint16, fields ...[3]byte) []byte {
	g := binary.LittleEndian.AppendUint16(nil, global)
	if arch != 0 {
		g = binary.BigEndian.AppendUint16(nil, global)
	}
	out := append([]byte{0x40 | local, 0, arch}, g...)
	out = append(out, byte(len(fields)))
	for _, f := range fields {
		out = append(out, f[:]...)
	}
	return out
}

func spansOf(t testing.TB, data []byte) []seqSpan {
	t.Helper()
	l := newListener(DefaultLimits.withDefaults(), nil)
	res := walk(context.Background(), data, l, DefaultLimits.withDefaults())
	return res.spans
}

// TestStripHandBuilt covers the framing cases a decoder would not hand us in
// a form the F-08 walk accepts: a big-endian definition, a field whose size
// is not a multiple of its base type, and a compressed-timestamp header.
func TestStripHandBuilt(t *testing.T) {
	u32z, u16z := byte(basetype.Uint32z), byte(basetype.Uint16z)
	t.Run("big-endian definition", func(t *testing.T) {
		// device_info, big-endian: serial number (uint32z) and ANT device number (uint16z).
		def := rawDef(1, true, uint16(mesgnum.DeviceInfo), [3]byte{3, 4, u32z}, [3]byte{21, 2, u16z}, [3]byte{2, 2, byte(basetype.Uint16)})
		msg := []byte{1, 0x12, 0x34, 0x56, 0x78, 0xAB, 0xCD, 0x00, 0x07}
		data := handSeq(def, msg)
		got, c, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 2}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := handSeq(def, []byte{1, 0, 0, 0, 0, 0, 0, 0x00, 0x07})
		if !bytes.Equal(got, want) || c.BlankedFields != 2 {
			t.Errorf("got % x (%+v), want % x", got, c, want)
		}
	})
	t.Run("z-type field shorter than its read type", func(t *testing.T) {
		def := rawDef(0, false, uint16(mesgnum.FileId), [3]byte{3, 3, u32z}, [3]byte{1, 2, byte(basetype.Uint16)})
		data := handSeq(def, []byte{0, 1, 2, 3, 9, 9})
		got, _, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 2}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := handSeq(def, []byte{0, 0, 0, 0, 9, 9}); !bytes.Equal(got, want) { // D-23: a short z-type field is filled with 0x00
			t.Errorf("got % x, want % x", got, want)
		}
	})
	// Audit D2 (D-24): the decoder reads every architecture byte but 0 as big-endian.
	t.Run("vi: user_profile, architecture 0x30, global number big-endian", func(t *testing.T) {
		def := rawDefArch(0, 0x30, uint16(mesgnum.UserProfile), [3]byte{4, 1, byte(basetype.Uint8)})
		data := handSeq(def, []byte{0, 41})
		got, c, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 2}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := handSeq(def); !bytes.Equal(got, want) || c.RemovedMessages != 1 {
			t.Errorf("got % x (%+v), want % x and 1 removed", got, c, want)
		}
		checkCRCs(t, got)
	})
	t.Run("vii: device_info, architecture 0x02, two z-type fields", func(t *testing.T) {
		def := rawDefArch(0, 0x02, uint16(mesgnum.DeviceInfo), [3]byte{3, 4, u32z}, [3]byte{21, 2, u16z})
		data := handSeq(def, []byte{0, 0x5A, 0x5A, 0x5A, 0x5A, 0x5A, 0x5A})
		got, c, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 2}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := handSeq(def, make([]byte, 7)); !bytes.Equal(got, want) || c.BlankedFields != 2 {
			t.Errorf("got % x (%+v), want % x and 2 blanked", got, c, want)
		}
		checkCRCs(t, got)
	})
	t.Run("compressed timestamp header of a removed message", func(t *testing.T) {
		def := rawDef(2, false, uint16(mesgnum.UserProfile), [3]byte{4, 1, byte(basetype.Uint8)})
		data := handSeq(def, []byte{0x80 | 2<<5 | 3, 41}, []byte{2, 41})
		got, c, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 3}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := handSeq(def); !bytes.Equal(got, want) || c.RemovedMessages != 2 {
			t.Errorf("got % x (%+v), want % x and 2 removed", got, c, want)
		}
	})
	t.Run("unreadable complete sequence fails, unreadable truncated one is cut", func(t *testing.T) {
		def := rawDef(0, false, uint16(mesgnum.FileId), [3]byte{3, 4, u32z})
		data := handSeq(def, []byte{5, 1, 2, 3, 4}) // local type 5 has no definition
		_, _, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, records: 2}}, nil)
		if !errors.Is(err, errRecordCount) {
			t.Errorf("complete: err = %v", err)
		}
		got, _, err := strip(data, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, status: seqTruncated, records: 1}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 14+len(def) || binary.LittleEndian.Uint32(got[4:8]) != 0 {
			t.Errorf("truncated: got % x", got)
		}
	})
	// D-20, ADR-023 rev. 4 item 8: the framing and the walk's count must agree.
	t.Run("count disagreement fails closed", func(t *testing.T) {
		def := rawDef(0, false, uint16(mesgnum.FileId), [3]byte{3, 4, u32z})
		msg := []byte{0, 1, 2, 3, 4}
		data := handSeq(def, msg, msg) // 3 records
		span := func(status seqStatus, records int) []seqSpan {
			return []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(data) - 2, status: status, records: records}}
		}
		for _, tc := range []struct {
			name    string
			status  seqStatus
			records int
			wantErr bool
		}{
			{"complete, records = N", seqComplete, 3, false},
			{"complete, records = N+1", seqComplete, 4, true},
			{"complete, records = N-1", seqComplete, 2, true},
			{"truncated, records = N", seqTruncated, 3, false},
			{"truncated, records > N", seqTruncated, 4, true},
		} {
			got, _, err := strip(data, span(tc.status, tc.records), nil)
			if tc.wantErr {
				if !errors.Is(err, errRecordCount) || got != nil {
					t.Errorf("%s: got %d bytes, err = %v", tc.name, len(got), err)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		}
		// A truncated span with fewer records than present stops at the count.
		got, _, err := strip(data, span(seqTruncated, 2), nil)
		if err != nil || len(got) != 14+len(def)+1+len(msg)-1 {
			t.Errorf("truncated, records = 2: %d bytes, err = %v", len(got), err)
		}
	})
	// D-21, ADR-023 rev. 5 item 6: a complete span is framed by records, read up
	// to the end of the upload, then checked against where the decoder's loop
	// ends. Output bytes are compared, not only the error.
	t.Run("complete spans framed by count", func(t *testing.T) {
		def := rawDef(0, false, uint16(mesgnum.Record), [3]byte{253, 4, byte(basetype.Uint32)})
		msg := []byte{0, 1, 2, 3, 4}
		full := handSeq(def, msg, msg, msg) // 4 records
		start4 := 14 + len(def) + 2*len(msg)
		span := func(dataEnd, records int) []seqSpan {
			return []seqSpan{{pos: 0, headerSize: 14, dataEnd: dataEnd, records: records}}
		}
		// (i) dataEnd inside the last record: written in full, data size = N records.
		got, _, err := strip(full, span(len(full)-2-2, 4), nil)
		if err != nil || !bytes.Equal(got, full) {
			t.Errorf("(i) err = %v, got % x, want % x", err, got, full)
		}
		// (ii) records = N-1, dataEnd after the end of record N-1, record N fits: check (b).
		got, _, err = strip(full, span(start4+2, 3), nil)
		if !errors.Is(err, errRecordCount) || got != nil {
			t.Errorf("(ii) err = %v, %d bytes", err, len(got))
		}
		// (iii) record N starts before dataEnd and runs past the end of the bytes.
		cut := full[:len(full)-3]
		got, _, err = strip(cut, span(start4+2, 3), nil)
		if want := handSeq(def, msg, msg); err != nil || !bytes.Equal(got, want) {
			t.Errorf("(iii) err = %v, got % x, want % x", err, got, want)
		}
		// (iv) records = N but dataEnd at or before the start of record N: check (a).
		for _, de := range []int{start4, start4 - 3} {
			got, _, err = strip(full, span(de, 4), nil)
			if !errors.Is(err, errRecordCount) || got != nil {
				t.Errorf("(iv) dataEnd %d: err = %v, %d bytes", de, err, len(got))
			}
		}
	})
}

// TestStripStop: a stop that fails before the second sequence of a chained
// file ends the loop with that error and no output (F-09 design, round 2 row 3).
func TestStripStop(t *testing.T) {
	def := rawDef(0, false, uint16(mesgnum.FileId), [3]byte{3, 4, byte(basetype.Uint32z)})
	one := handSeq(def, []byte{0, 1, 2, 3, 4})
	data := append(append([]byte(nil), one...), one...)
	spans := []seqSpan{
		{pos: 0, headerSize: 14, dataEnd: len(one) - 2, records: 2},
		{pos: len(one), headerSize: 14, dataEnd: len(data) - 2, records: 2},
	}
	errStop := errors.New("stop")
	calls := 0
	out, _, err := strip(data, spans, func() error {
		calls++
		if calls == 2 {
			return errStop
		}
		return nil
	})
	if !errors.Is(err, errStop) || out != nil || calls != 2 {
		t.Errorf("out %d bytes, err = %v, calls = %d", len(out), err, calls)
	}
}

// blankFillCase is one hand-built sequence of TestBlankFill.
type blankFillCase struct {
	mesg     uint16
	field    uint8
	declared basetype.BaseType
	size     int
	arch     byte // architecture byte: 0 little-endian, any other value big-endian
}

// blankFillSeq builds the case's sequence: a definition of the field's message
// with the blank field and an unknown neighbour (byte, size 2), and one data
// message of 0x5A throughout. It returns the sequence and the offset of the
// blank field's bytes.
func blankFillSeq(c blankFillCase, neighbour uint8) ([]byte, int) {
	def := rawDefArch(0, c.arch, c.mesg, [3]byte{c.field, byte(c.size), byte(c.declared)}, [3]byte{neighbour, 2, byte(basetype.Byte)})
	msg := append([]byte{0}, bytes.Repeat([]byte{0x5A}, c.size+2)...)
	data := handSeq(def, msg)
	return data, 14 + len(def) + 1
}

// TestBlankFill (F-09 v1.7, D-23, ADR-023 rev. 6 item 4; replaces
// TestFillInvalid): every blank-table field against every valid declared base
// type, a range of declared sizes and both byte orders, hand-built bytes
// through strip and read back with the library's decoder.
func TestBlankFill(t *testing.T) {
	var neighbour [1 << 16]uint8 // per message: a field number the profile lacks
	sizes := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 16, 17}
	declared := basetype.List()
	if len(declared) != 17 {
		t.Fatalf("%d base types, want 17", len(declared))
	}
	byFill := map[blankFillCase]map[string]bool{} // case with declared 0 -> distinct outputs
	cases := 0
	for mesg, fields := range blankTable {
		for n := 250; n > 0; n-- {
			if factory.CreateField(typedef.MesgNum(mesg), uint8(n)).Name == factory.NameUnknown {
				neighbour[mesg] = uint8(n)
				break
			}
		}
		if neighbour[mesg] == 0 {
			t.Fatalf("message %d: no unknown field number", mesg)
		}
		for _, field := range fields {
			rt := readType(mesg, field, 0)
			for _, arch := range []byte{0x00, 0x01, 0x30, 0xFF} {
				for _, size := range sizes {
					for _, dt := range declared {
						c := blankFillCase{mesg, field, dt, size, arch}
						cases++
						in, at := blankFillSeq(c, neighbour[mesg])
						spans := []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(in) - 2, records: 2}}
						out, counts, err := strip(in, spans, nil)
						if err != nil {
							t.Fatalf("%+v: strip: %v", c, err)
						}
						checkBlankFill(t, c, rt, in, out, at, counts)
						crossCheckFraming(t, in, spans)
						crossCheckFraming(t, out, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(out) - 2, records: 2}})
						key := c
						key.declared = 0
						if byFill[key] == nil {
							byFill[key] = map[string]bool{}
						}
						byFill[key][string(out[at:at+size])] = true
						// (5) idempotent.
						again, counts2, err := strip(out, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(out) - 2, records: 2}}, nil)
						if err != nil || !bytes.Equal(again, out) || counts2.BlankedFields != 0 {
							t.Fatalf("%+v: strip of the output: err %v, same %v, blanked %d", c, err, bytes.Equal(again, out), counts2.BlankedFields)
						}
					}
				}
			}
		}
	}
	// (4) the fill does not depend on the declared type.
	for k, outs := range byFill {
		if len(outs) != 1 {
			t.Errorf("message %d field %d size %d arch=%#x: %d different fills across the declared types", k.mesg, k.field, k.size, k.arch, len(outs))
		}
	}
	if cases != 20*17*13*4 {
		t.Errorf("%d cases, want %d", cases, 20*17*13*4)
	}
	t.Logf("%d cases", cases)

	// Pinned bytes: literal expectations (little-endian unless noted).
	ff := func(n int) []byte { return bytes.Repeat([]byte{0xFF}, n) }
	pinned := []struct {
		name string
		c    blankFillCase
		want []byte
	}{
		{"file_id.serial_number uint8, size 1 (the fuzz input)", blankFillCase{uint16(mesgnum.FileId), fieldnum.FileIdSerialNumber, basetype.Uint8, 1, 0}, []byte{0}},
		{"file_id.serial_number uint32, size 4", blankFillCase{uint16(mesgnum.FileId), fieldnum.FileIdSerialNumber, basetype.Uint32, 4, 0}, make([]byte, 4)},
		{"file_id.serial_number uint32z, size 5", blankFillCase{uint16(mesgnum.FileId), fieldnum.FileIdSerialNumber, basetype.Uint32z, 5, 0}, make([]byte, 5)},
		{"device_info.ant_device_number uint32, size 4, big-endian", blankFillCase{uint16(mesgnum.DeviceInfo), fieldnum.DeviceInfoAntDeviceNumber, basetype.Uint32, 4, 1}, make([]byte, 4)},
		{"segment_id.device_id uint8, size 1 (short: reads 0)", blankFillCase{uint16(mesgnum.SegmentId), fieldnum.SegmentIdDeviceId, basetype.Uint8, 1, 0}, []byte{0}},
		{"segment_id.device_id uint16, size 4, big-endian", blankFillCase{uint16(mesgnum.SegmentId), fieldnum.SegmentIdDeviceId, basetype.Uint16, 4, 1}, ff(4)},
		{"segment_file.leader_activity_id, size 10", blankFillCase{uint16(mesgnum.SegmentFile), fieldnum.SegmentFileLeaderActivityId, basetype.Uint32, 10, 0}, append(ff(8), 0, 0)},
		{"segment_file.leader_activity_id_string uint8, size 5", blankFillCase{uint16(mesgnum.SegmentFile), fieldnum.SegmentFileLeaderActivityIdString, basetype.Uint8, 5, 0}, make([]byte, 5)},
		{"device_info.serial_number uint32z, size 4, architecture 0x30 (round-6 fuzz input)", blankFillCase{uint16(mesgnum.DeviceInfo), fieldnum.DeviceInfoSerialNumber, basetype.Uint32z, 4, 0x30}, make([]byte, 4)},
		{"tank_update.sensor sint32, size 4", blankFillCase{uint16(mesgnum.TankUpdate), fieldnum.TankUpdateSensor, basetype.Sint32, 4, 0}, make([]byte, 4)},
	}
	for _, p := range pinned {
		in, at := blankFillSeq(p.c, neighbour[p.c.mesg])
		out, counts, err := strip(in, []seqSpan{{pos: 0, headerSize: 14, dataEnd: len(in) - 2, records: 2}}, nil)
		if err != nil {
			t.Fatalf("%s: %v", p.name, err)
		}
		if got := out[at : at+p.c.size]; !bytes.Equal(got, p.want) {
			t.Errorf("%s: got % x, want % x", p.name, got, p.want)
		}
		if counts.BlankedFields != 1 {
			t.Errorf("%s: BlankedFields = %d, want 1", p.name, counts.BlankedFields)
		}
	}
}

// checkBlankFill checks one TestBlankFill case: (1) decodes, has the field (a field of size 0 is dropped by the decoder: absent),
// (2) absent or (short field) 0, (3) only the field's bytes and the file CRC
// differ, (6) the count.
func checkBlankFill(t *testing.T, c blankFillCase, rt basetype.BaseType, in, out []byte, at int, counts StripCounts) {
	t.Helper()
	var msg *proto.Message
	dec := decoder.New(bytes.NewReader(out), decoder.WithMesgListener(mesgFunc(func(m proto.Message) {
		if msg == nil {
			m := m
			msg = &m
		}
	})))
	if _, err := dec.Decode(); err != nil {
		t.Fatalf("%+v: the output does not decode: %v (field bytes % x)", c, err, out[at:at+c.size])
	}
	if msg == nil {
		t.Fatalf("%+v: no data message in the output", c)
	}
	f, ok := fieldOf(*msg, c.field)
	if !ok && c.size != 0 {
		t.Fatalf("%+v: the data message has no field %d (read type %v)", c, c.field, rt)
	}
	short := c.size > 0 && c.size < int(rt.Size()) && rt != basetype.String &&
		rt != basetype.Uint8z && rt != basetype.Uint16z && rt != basetype.Uint32z && rt != basetype.Uint64z
	switch {
	case !ok: // size 0: the decoder drops the field, which reads as absent
	case short:
		if !isZeroValue(f.Value) {
			t.Errorf("%+v: short field reads %v, want 0 (field bytes % x)", c, f.Value.Any(), out[at:at+c.size])
		}
	case f.Value.Valid(f.BaseType):
		t.Errorf("%+v: reads the valid value %v (field bytes % x, read type %v)", c, f.Value.Any(), out[at:at+c.size], rt)
	}
	for i := 0; i < c.size; i++ {
		if out[at+i] == 0x5A {
			t.Errorf("%+v: byte %d of the field is still 0x5A", c, i)
		}
	}
	if len(out) != len(in) || !bytes.Equal(out[:at], in[:at]) || !bytes.Equal(out[at+c.size:len(out)-2], in[at+c.size:len(in)-2]) {
		t.Errorf("%+v: bytes outside the field (or the length) changed", c)
	}
	if want := map[bool]int{true: 1, false: 0}[c.size > 0]; counts.BlankedFields != want {
		t.Errorf("%+v: BlankedFields = %d, want %d", c, counts.BlankedFields, want)
	}
}

type mesgFunc func(proto.Message)

func (f mesgFunc) OnMesg(m proto.Message) { f(m) }

// isZeroValue reports whether every element of v is 0.
func isZeroValue(v proto.Value) bool {
	x := reflect.ValueOf(v.Any())
	if x.Kind() == reflect.Slice {
		for i := 0; i < x.Len(); i++ {
			if !x.Index(i).IsZero() {
				return false
			}
		}
		return true
	}
	return x.IsZero()
}

// ---- TestStripCorpus (AC 6, AC 7) ---------------------------------------------

func TestStripCorpus(t *testing.T) {
	base := readSynthetic(t, "ride-30min.fit")
	for _, path := range corpustest.Files(t, corpusDir, "synthetic") {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			upload := readSynthetic(t, name)
			crossCheckFile(t, upload)
			acc, err := testParser.Accept(context.Background(), upload)
			if err != nil {
				if _, perr := testParser.Parse(context.Background(), upload); perr == nil {
					t.Fatalf("Accept refused an accepted corpus file as %s: %v", kindOf(err), err) // strict: ADR-004 rev. 10, D-29
				}
				return // a rejected file: TestAcceptRejections
			}
			got := acc.Original
			crossCheckFile(t, got)
			switch {
			case name == "identifying-data.fit":
				return // TestStripIdentifyingData
			case name == "chained-with-settings.fit":
				if !bytes.Equal(got[:len(base)], base) {
					t.Errorf("the activity sequence changed")
				}
				checkCRCs(t, got[len(base):])
				if acc.Stripped.RemovedMessages != 1 || acc.Stripped.BlankedFields != 0 {
					t.Errorf("counts = %+v, want one removed message", acc.Stripped)
				}
			case strings.HasSuffix(name, "-bad-crc.fit"), strings.HasSuffix(name, "-bad-header-crc.fit"),
				strings.HasSuffix(name, "-no-crc.fit"), strings.HasSuffix(name, "-trailing-zeros.fit"),
				name == "ride-30min-size-short.fit": // D-21: the data size is repaired, nothing is dropped
				if !bytes.Equal(got, base) {
					t.Errorf("stored differently from the clean file it was derived from")
				}
			case strings.Contains(name, "-cut-"):
				sameRecords(t, upload, got)
			case name == "ride-30min-bad-basetype.fit":
				// The upload's bytes up to the inserted definition, exclusive, in
				// the unclosed form (D-20; ADR-023 rev. 4 item 6a).
				ins := 14
				for base[ins] == upload[ins] {
					ins++
				}
				want := append([]byte(nil), upload[:ins]...)
				binary.LittleEndian.PutUint32(want[4:8], 0)
				want[12], want[13] = 0, 0
				if !bytes.Equal(got, want) {
					t.Errorf("stored %d bytes, want the upload up to the inserted definition (%d bytes) in the unclosed form", len(got), len(want))
				}
			default:
				if !bytes.Equal(got, upload) {
					t.Errorf("stored differently from the upload (%d, %d bytes) although there is nothing to strip", len(got), len(upload))
				}
			}
			if acc.Stripped.BlankedFields != 0 && name != "identifying-data.fit" {
				t.Errorf("blanked %d fields in a file without identifying data", acc.Stripped.BlankedFields)
			}
		})
	}
}

// sameRecords checks a cut file: the stored sequences have the upload's
// records (the last one only if it was complete), the unclosed header form,
// and no CRC or bytes after the last record.
func sameRecords(t *testing.T, upload, stored []byte) {
	t.Helper()
	pos, spos := 0, 0
	for pos < len(upload) {
		u, s := splitSeq(t, upload[pos:]), splitSeq(t, stored[spos:])
		if len(s.records) > len(u.records) {
			t.Fatalf("stored sequence has more records than the upload")
		}
		for i, r := range s.records {
			if !bytes.Equal(r.raw, u.records[i].raw) {
				t.Fatalf("record %d differs", i)
			}
		}
		truncated := len(u.crc) == 0 || len(u.tail) > 0 || binary.LittleEndian.Uint32(upload[pos+4:pos+8]) == 0
		if !truncated {
			if !bytes.Equal(stored[spos:spos+len(s.header)+sumLen(s.records)+2], upload[pos:pos+len(u.header)+sumLen(u.records)+2]) {
				t.Fatalf("a complete sequence changed")
			}
			pos += len(u.header) + sumLen(u.records) + 2 + 0
			spos += len(s.header) + sumLen(s.records) + 2
			continue
		}
		if binary.LittleEndian.Uint32(stored[spos+4:spos+8]) != 0 || (stored[spos] == 14 && (stored[spos+12] != 0 || stored[spos+13] != 0)) {
			t.Errorf("truncated sequence not in the unclosed form")
		}
		if len(s.tail) != 0 || len(s.crc) != 0 || len(s.rest) != 0 || spos+len(s.header)+sumLen(s.records) != len(stored) {
			t.Errorf("truncated sequence does not end on its last complete record")
		}
		return
	}
}

func sumLen(rs []stRecord) int {
	n := 0
	for _, r := range rs {
		n += len(r.raw)
	}
	return n
}

// ---- TestStripTable ---------------------------------------------------------

// idRule is one line of ADR-023 rev. 4 item 4a's identifier rule.
type idRule struct {
	name  string
	match func(name string, typ profile.ProfileType) bool
}

var idRules = []idRule{
	{"contains serial", func(n string, _ profile.ProfileType) bool { return strings.Contains(n, "serial") }},
	{"ends with device_number", func(n string, _ profile.ProfileType) bool { return strings.HasSuffix(n, "device_number") }},
	{"contains activity_id", func(n string, _ profile.ProfileType) bool { return strings.Contains(n, "activity_id") }},
	{"contains primary_key", func(n string, _ profile.ProfileType) bool { return strings.Contains(n, "primary_key") }},
	{"ends with _id, _ids, uuid or identifier, or is id", func(n string, _ profile.ProfileType) bool {
		return n == "id" || strings.HasSuffix(n, "_id") || strings.HasSuffix(n, "_ids") ||
			strings.HasSuffix(n, "uuid") || strings.HasSuffix(n, "identifier")
	}},
	{"profile type ant_channel_id", func(_ string, t profile.ProfileType) bool { return t == profile.AntChannelId }},
}

// identifierLike applies the rules to the field's name, to each of its
// subfields' names, and to its profile type.
func identifierLike(rules []idRule, f *proto.FieldBase) bool {
	for _, r := range rules {
		if r.match(f.Name, f.Type) {
			return true
		}
		for _, sf := range f.SubFields {
			if r.match(sf.Name, sf.Type) {
				return true
			}
		}
	}
	return false
}

type msgField struct {
	mesg  uint16
	field uint8
}

// wantBlank is ADR-023 rev. 4 item 4's table with the profile's field names.
var wantBlank = map[msgField]string{
	{0, 3}:    "serial_number",
	{23, 3}:   "serial_number",
	{23, 21}:  "ant_device_number",
	{72, 3}:   "serial_number",
	{28, 2}:   "serial_number",
	{4, 1}:    "hrm_ant_id",
	{5, 1}:    "sdm_ant_id",
	{6, 4}:    "bike_spd_ant_id",
	{6, 5}:    "bike_cad_ant_id",
	{6, 6}:    "bike_spdcad_ant_id",
	{6, 7}:    "bike_power_ant_id",
	{82, 2}:   "device_number",
	{148, 5}:  "device_id",
	{148, 4}:  "user_profile_primary_key",
	{151, 4}:  "user_profile_primary_key",
	{151, 8}:  "leader_group_primary_key",
	{151, 9}:  "leader_activity_id",
	{151, 10}: "leader_activity_id_string",
	{319, 0}:  "sensor",
	{323, 0}:  "sensor",
}

// wantKept is ADR-023 rev. 4 item 4a's keptIdentifiers table, exactly.
var wantKept = map[msgField]string{
	{6, 14}:   "id",
	{262, 6}:  "id",
	{393, 6}:  "id",
	{206, 2}:  "fit_base_type_id",
	{206, 13}: "fit_base_unit_id",
	{207, 0}:  "developer_id",
	{207, 1}:  "application_id",
	{207, 2}:  "manufacturer_id",
	{129, 0}:  "report_id",
	{161, 2}:  "camera_file_uuid",
	{148, 1}:  "uuid",
	{142, 65}: "uuid",
	{151, 1}:  "file_uuid",
	{315, 0}:  "event_id",
	{201, 2}:  "field_id",
	{202, 2}:  "field_id",
	{375, 3}:  "battery_identifier",
}

// tableProblems applies the identifier rule to the whole pinned profile and
// compares it with the two tables. It returns one line per problem.
func tableProblems(rules []idRule, blank map[uint16][]uint8, kept []keptIdentifier, remove map[uint16]bool) []string {
	var out []string
	inBlank := map[msgField]bool{}
	for m, fs := range blank {
		for _, n := range fs {
			inBlank[msgField{m, n}] = true
		}
	}
	inKept := map[msgField]bool{}
	for _, k := range kept {
		mf := msgField{k.mesg, k.field}
		if inKept[mf] {
			out = append(out, fmt.Sprintf("%v is in keptIdentifiers twice", mf))
		}
		inKept[mf] = true
	}
	seen := map[msgField]bool{}
	for n := 0; n <= math.MaxUint16; n++ {
		if remove[uint16(n)] {
			continue
		}
		m := factory.CreateMesg(typedef.MesgNum(n))
		for _, f := range m.Fields {
			mf := msgField{uint16(n), f.Num}
			seen[mf] = true
			like := identifierLike(rules, f.FieldBase)
			switch {
			case like && !inBlank[mf] && !inKept[mf]:
				out = append(out, fmt.Sprintf("message %d field %d (%s) is identifier-like and in neither table", n, f.Num, f.Name))
			case like && inBlank[mf] && inKept[mf]:
				out = append(out, fmt.Sprintf("message %d field %d (%s) is in both tables", n, f.Num, f.Name))
			case !like && (inBlank[mf] || inKept[mf]):
				out = append(out, fmt.Sprintf("message %d field %d (%s) is in a table but not identifier-like", n, f.Num, f.Name))
			}
			if inBlank[mf] && (strings.HasSuffix(f.Name, "trans_type") || strings.HasSuffix(f.Name, "transmission_type")) {
				out = append(out, fmt.Sprintf("message %d field %d (%s) is a transmission type and must be kept (OQ-46)", n, f.Num, f.Name))
			}
		}
	}
	for mf := range inBlank {
		if !seen[mf] {
			out = append(out, fmt.Sprintf("blank table entry %v is not in the profile", mf))
		}
		if remove[mf.mesg] {
			out = append(out, fmt.Sprintf("message %d is both blanked and removed", mf.mesg))
		}
	}
	for mf := range inKept {
		if !seen[mf] {
			out = append(out, fmt.Sprintf("keptIdentifiers entry %v is not in the profile", mf))
		}
	}
	sort.Strings(out)
	return out
}

// readTypeProblems compares rt with the factory: one entry per blankTable
// field, each the factory's base type for it, and none of an unknown field.
func readTypeProblems(rt map[blankKey]basetype.BaseType) []string {
	var out []string
	n := 0
	for m, fs := range blankTable {
		for _, f := range fs {
			n++
			want := factory.CreateField(typedef.MesgNum(m), f)
			if want.Name == factory.NameUnknown {
				out = append(out, fmt.Sprintf("message %d field %d is unknown to the factory", m, f))
			}
			if got, ok := rt[blankKey{m, f}]; !ok || got != want.BaseType {
				out = append(out, fmt.Sprintf("message %d field %d: read type %v, the factory says %v", m, f, got, want.BaseType))
			}
		}
	}
	if len(rt) != n {
		out = append(out, fmt.Sprintf("%d read types for %d blank-table fields", len(rt), n))
	}
	return out
}

func TestStripTable(t *testing.T) {
	// The remove list is exactly {3, 29, 80, 81, 149}, and the numbers match the library.
	if len(removeTable) != 5 || !removeTable[3] || !removeTable[29] || !removeTable[80] || !removeTable[81] || !removeTable[149] {
		t.Errorf("remove list = %v, want {3, 29, 80, 81, 149}", removeTable)
	}
	if uint16(mesgnum.UserProfile) != 3 || uint16(mesgnum.AntRx) != 80 || uint16(mesgnum.AntTx) != 81 ||
		uint16(mesgnum.SegmentLeaderboardEntry) != 149 || mesgNumLocation != 29 {
		t.Errorf("remove list numbers do not match the library")
	}

	// The blank table is exactly the wanted one, with the profile's field names.
	got := map[msgField]bool{}
	for m, fs := range blankTable {
		for _, n := range fs {
			mf := msgField{m, n}
			if got[mf] {
				t.Errorf("blank table lists %v twice", mf)
			}
			got[mf] = true
			name, ok := wantBlank[mf]
			if !ok {
				t.Errorf("blank table entry %v is not in ADR-023 rev. 4 item 4", mf)
				continue
			}
			if f := factory.CreateField(typedef.MesgNum(m), n); f.Name != name {
				t.Errorf("message %d field %d is %q in the profile, want %q", m, n, f.Name, name)
			}
		}
	}
	for mf := range wantBlank {
		if !got[mf] {
			t.Errorf("ADR-023 rev. 4 item 4 entry %v is missing from the blank table", mf)
		}
	}

	// keptIdentifiers is exactly item 4a's table.
	gotKept := map[msgField]bool{}
	for _, k := range keptIdentifiers {
		mf := msgField{k.mesg, k.field}
		gotKept[mf] = true
		name, ok := wantKept[mf]
		if !ok {
			t.Errorf("keptIdentifiers entry %v is not in ADR-023 rev. 4 item 4a", mf)
			continue
		}
		if f := factory.CreateField(typedef.MesgNum(k.mesg), k.field); f.Name != name {
			t.Errorf("message %d field %d is %q in the profile, want %q", k.mesg, k.field, f.Name, name)
		}
		if k.reason == "" {
			t.Errorf("keptIdentifiers entry %v has no reason", mf)
		}
	}
	for mf := range wantKept {
		if !gotKept[mf] {
			t.Errorf("ADR-023 rev. 4 item 4a entry %v is missing from keptIdentifiers", mf)
		}
	}

	// The identifier rule over the whole pinned profile, both directions.
	for _, p := range tableProblems(idRules, blankTable, keptIdentifiers, removeTable) {
		t.Error(p)
	}

	// D-23: every read type is the factory's, and an unknown field falls back to the declared type.
	for _, p := range readTypeProblems(readTypes) {
		t.Error(p)
	}
	if got := readType(uint16(mesgnum.FileId), 250, uint8(basetype.Uint16)); got != basetype.Uint16 {
		t.Errorf("readType of a field the profile lacks = %v, want the declared uint16", got)
	}

	// Mutants: each must make the check fail.
	t.Run("mutants", func(t *testing.T) {
		for k, bt := range readTypes {
			mutant := maps.Clone(readTypes)
			if bt == basetype.Uint8 {
				mutant[k] = basetype.Uint16
			} else {
				mutant[k] = basetype.Uint8
			}
			if len(readTypeProblems(mutant)) == 0 {
				t.Errorf("read type of %v changed: TestStripTable would still pass", k)
			}
		}
		for i, r := range idRules {
			rules := append(append([]idRule{}, idRules[:i]...), idRules[i+1:]...)
			if len(tableProblems(rules, blankTable, keptIdentifiers, removeTable)) == 0 {
				t.Errorf("rule %q removed: TestStripTable would still pass", r.name)
			}
		}
		noDeviceNumber := map[uint16][]uint8{}
		for m, fs := range blankTable {
			noDeviceNumber[m] = fs
		}
		noDeviceNumber[uint16(mesgnum.AntChannelId)] = nil
		if len(tableProblems(idRules, noDeviceNumber, keptIdentifiers, removeTable)) == 0 {
			t.Errorf("device_number dropped from the blank table: check still passes")
		}
		for i := range keptIdentifiers {
			fewer := append(append([]keptIdentifier{}, keptIdentifiers[:i]...), keptIdentifiers[i+1:]...)
			if len(tableProblems(idRules, blankTable, fewer, removeTable)) == 0 {
				t.Errorf("keptIdentifiers entry %d dropped: check still passes", i)
			}
		}
		withTrans := map[uint16][]uint8{}
		for m, fs := range blankTable {
			withTrans[m] = fs
		}
		withTrans[uint16(mesgnum.AntChannelId)] = []uint8{fieldnum.AntChannelIdDeviceNumber, fieldnum.AntChannelIdTransmissionType}
		if len(tableProblems(idRules, withTrans, keptIdentifiers, removeTable)) == 0 {
			t.Errorf("a transmission type in the blank table: check still passes")
		}
		withRemoved := map[uint16][]uint8{}
		for m, fs := range blankTable {
			withRemoved[m] = fs
		}
		withRemoved[uint16(mesgnum.UserProfile)] = []uint8{fieldnum.UserProfileGlobalId}
		if len(tableProblems(idRules, withRemoved, keptIdentifiers, removeTable)) == 0 {
			t.Errorf("a removed message in the blank table: check still passes")
		}
	})
}

// ---- D-20: the cut at a record the decoder rejects (F-09 design v1.5) ----------

// recordStarts returns the offset (in b) at which each record of the first
// sequence of b starts, using splitSeq's framing, and the records themselves.
func recordStarts(t *testing.T, b []byte) ([]int, []stRecord) {
	t.Helper()
	sp := splitSeq(t, b)
	at := len(sp.header)
	starts := make([]int, len(sp.records))
	for i, r := range sp.records {
		starts[i] = at
		at += len(r.raw)
	}
	return starts, sp.records
}

// closeSeq sets the data size and both CRCs of a single-sequence file.
func closeSeq(b []byte) []byte {
	hs := int(b[0])
	binary.LittleEndian.PutUint32(b[4:8], uint32(len(b)-hs))
	if hs == 14 {
		binary.LittleEndian.PutUint16(b[12:14], fitCRC(0, b[:12]))
	}
	return binary.LittleEndian.AppendUint16(b, fitCRC(0, b))
}

// freeLocals returns the local types that no record of recs[from:] uses.
func freeLocals(recs []stRecord, from int) []byte {
	var used [16]bool
	for _, r := range recs[from:] {
		used[r.raw[0]&0x0F] = true
	}
	var free []byte
	for l := 15; l >= 0; l-- {
		if !used[l] {
			free = append(free, byte(l))
		}
	}
	return free
}

// recordShape is what the test compares when it says two files hold the same
// records: kind, local type, global number and length.
type recordShape struct {
	def    bool
	local  byte
	global uint16
	length int
}

func shapesOf(recs []stRecord) []recordShape {
	out := make([]recordShape, len(recs))
	for i, r := range recs {
		out[i] = recordShape{r.def, r.raw[0] & 0x0F, r.global, len(r.raw)}
	}
	return out
}

// TestStripRejectedRecord: QA's probe for QA-09-r2-1 as a test (D-20). A
// definition with an invalid base type, then a copy of the creator's
// device_info (with its serial-number marker), then the rest of the file. The
// decoder stops at the definition; the stored file must too.
func TestStripRejectedRecord(t *testing.T) {
	data := uploadOf(t, "identifying-data.fit")
	starts, recs := recordStarts(t, data)
	var devDef, devData = -1, -1
	var recIdx []int
	for i, r := range recs {
		switch {
		case r.global == uint16(mesgnum.DeviceInfo) && r.def && devDef < 0:
			devDef = i
		case r.global == uint16(mesgnum.DeviceInfo) && !r.def && devData < 0:
			devData = i
		case r.global == uint16(mesgnum.Record) && !r.def:
			recIdx = append(recIdx, i)
		}
	}
	ins := recIdx[len(recIdx)/10]
	if free := freeLocals(recs, ins); len(free) < 2 || free[0] != 15 || free[1] != 14 {
		t.Fatalf("local types 15 and 14 are not free behind the insert point: %v", free)
	}
	at := starts[ins]
	bad := []byte{0x4F, 0, 0, 0x00, 0xFF, 1, 0, 1, 0x30} // local 15, global 0xFF00, one field of base type 0x30
	dd := append([]byte(nil), recs[devDef].raw...)
	dd[0] = dd[0]&^0x0F | 14
	dm := append([]byte(nil), recs[devData].raw...)
	dm[0] = dm[0]&^0x0F | 14
	hs := int(data[0])
	end := hs + int(binary.LittleEndian.Uint32(data[4:8]))
	var up []byte
	up = append(up, data[:at]...)
	up = append(up, bad...)
	up = append(up, dd...)
	up = append(up, dm...)
	up = append(up, data[at:end]...)
	up = closeSeq(up)
	marker := binary.LittleEndian.AppendUint32(nil, 2882400001)
	if bytes.Count(up, marker) < 2 {
		t.Fatalf("the serial-number marker occurs %d times in the upload, want the original and the copy", bytes.Count(up, marker))
	}

	acc, err := testParser.Accept(context.Background(), up)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if !acc.Workout.Integrity.Truncated {
		t.Errorf("truncated = false")
	}
	stored := acc.Original

	// (a) The stored records are the upload's before the inserted definition,
	// less the data messages the strip removes.
	_, upRecs := recordStarts(t, up)
	var want []stRecord
	for _, r := range upRecs[:ins] {
		if !r.def && removeTable[r.global] {
			continue
		}
		want = append(want, r)
	}
	sp := splitSeq(t, stored)
	if len(sp.tail) != 0 || len(sp.crc) != 0 || len(sp.rest) != 0 {
		t.Errorf("the stored sequence is not exactly header and complete records")
	}
	if got, w := shapesOf(sp.records), shapesOf(want); !slices.Equal(got, w) {
		t.Errorf("(a) stored %d records, want the %d before the rejected definition", len(got), len(w))
	}
	if binary.LittleEndian.Uint32(stored[4:8]) != 0 || stored[12] != 0 || stored[13] != 0 {
		t.Errorf("not in the unclosed form")
	}
	// (b)
	decodesToEnd(t, stored)
	// (c)
	if n := bytes.Count(stored, marker); n != 0 {
		t.Errorf("(c) the serial-number marker occurs %d times in the stored bytes", n)
	}
	// (d)
	checkIdempotent(t, testParser, stored)
	// (e)
	checkStored(t, testParser, up, acc)
	crossCheckFile(t, up)
	crossCheckFile(t, stored)
}

// TestWalkRecords: seqSpan.records equals the number of records the
// independent framing (splitSeq) reads in the same span (D-20).
func TestWalkRecords(t *testing.T) {
	lim := DefaultLimits.withDefaults()
	for _, name := range []string{
		"ride-30min.fit", "ride-30min-bad-header-crc.fit", "ride-30min-cut-60.fit",
		"ride-30min-cut-60-size-0.fit", "chained-with-settings.fit", "chained-ride-ride.fit",
		"triathlon.fit", "identifying-data.fit", "ride-30min-bad-basetype.fit", "ride-30min-size-short.fit",
	} {
		t.Run(name, func(t *testing.T) {
			data := readSynthetic(t, name)
			res := walk(context.Background(), data, newListener(lim, nil), lim)
			if len(res.spans) == 0 {
				t.Fatal("no spans")
			}
			for i, sp := range res.spans {
				framed := splitSeq(t, data[sp.pos:])
				want := len(framed.records)
				if name == "ride-30min-size-short.fit" {
					// The header's data size ends inside the last record; the decoder
					// reads that record in full, so the count is the clean file's (D-21).
					clean := readSynthetic(t, "ride-30min.fit")
					cres := walk(context.Background(), clean, newListener(lim, nil), lim)
					if sp.status != seqComplete || len(cres.spans) != 1 {
						t.Fatalf("status %v, clean file has %d spans", sp.status, len(cres.spans))
					}
					want = cres.spans[0].records
				}
				if name == "ride-30min-bad-basetype.fit" {
					// The framing reads past the inserted definition; the decoder does not.
					base := readSynthetic(t, "ride-30min.fit")
					ins := 14
					for base[ins] == data[ins] {
						ins++
					}
					starts, _ := recordStarts(t, data)
					want = 0
					for _, s := range starts {
						if s < ins {
							want++
						}
					}
				}
				if sp.records != want {
					t.Errorf("sequence %d: records = %d, framing reads %d", i, sp.records, want)
				}
			}
		})
	}
}

// errTB records the errors a storedListener reports instead of failing the test.
type errTB struct {
	testing.TB
	errs []string
}

func (e *errTB) Errorf(format string, args ...any) {
	e.errs = append(e.errs, fmt.Sprintf(format, args...))
}

// TestStoredListenerShortPerLocalType (F-09 v1.8, round 7 row 6): the short-field
// exception of checkStored holds per local type. segment_id.device_id is
// declared uint8 size 1 (short) on local type 0 and uint32 size 4 (full) on
// local type 1. A 0 on local type 0 is a short field; the same 0 on local type
// 1 is a valid value and must be reported. Built by hand, without strip.
func TestStoredListenerShortPerLocalType(t *testing.T) {
	dev := byte(fieldnum.SegmentIdDeviceId)
	def0 := rawDef(0, false, uint16(mesgnum.SegmentId), [3]byte{dev, 1, byte(basetype.Uint8)})
	def1 := rawDef(1, false, uint16(mesgnum.SegmentId), [3]byte{dev, 4, byte(basetype.Uint32)})
	msg0 := []byte{0, 0x00}
	msg1 := []byte{1, 0, 0, 0, 0}
	for _, tc := range []struct {
		name string
		recs [][]byte
		want int
	}{
		{"local type 0 only: the short field reads 0", [][]byte{def0, msg0}, 0},
		{"local type 1 only: a full field reads 0", [][]byte{def1, msg1}, 1},
		{"both: only the full field is reported", [][]byte{def0, def1, msg0, msg1}, 1},
		{"redefinition: local type 0 becomes a full field", [][]byte{def0, msg0, rawDef(0, false, uint16(mesgnum.SegmentId), [3]byte{dev, 4, byte(basetype.Uint32)}), {0, 0, 0, 0, 0}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &errTB{TB: t}
			l := &storedListener{t: rec}
			seq := handSeq(tc.recs...)
			if _, err := decoder.New(bytes.NewReader(seq), decoder.WithMesgListener(l), decoder.WithMesgDefListener(l)).Decode(); err != nil {
				t.Fatal(err)
			}
			if len(rec.errs) != tc.want {
				t.Fatalf("%d errors %q, want %d", len(rec.errs), rec.errs, tc.want)
			}
			for _, e := range rec.errs {
				if !strings.Contains(e, "has a valid value") {
					t.Errorf("unexpected error %q", e)
				}
			}
		})
	}
}
