package datafile

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"workout-explorer/internal/workout"
)

// Decode reads a data file of format version 6. It is strict: it implements
// every check of Format §6, including rule 7, and returns an error (prefixed
// "datafile:") instead of panicking on any malformed input.
func Decode(b []byte) (File, error) {
	// Rule 1.
	if len(b) < headerSize {
		return File{}, fmt.Errorf("datafile: %d bytes, shorter than the header", len(b))
	}
	if string(b[:4]) != Magic {
		return File{}, errors.New("datafile: bad magic")
	}
	// Rule 2.
	f := File{Version: binary.LittleEndian.Uint16(b[4:])}
	if f.Version != Version {
		return File{}, fmt.Errorf("datafile: unsupported version %d", f.Version)
	}
	// Rule 3.
	n := int64(binary.LittleEndian.Uint32(b[6:]))
	l := int64(binary.LittleEndian.Uint32(b[10:]))
	if n == 0 {
		return File{}, errors.New("datafile: no samples")
	}
	if headerSize+l > int64(len(b)) {
		return File{}, fmt.Errorf("datafile: metadata length %d beyond the end of the file", l)
	}
	f.SampleCount, f.MetadataLength = int(n), int(l)

	// Rule 4.
	js := b[headerSize : headerSize+l]
	meta, err := decodeMeta(js)
	if err != nil {
		return File{}, err
	}
	f.Meta = meta

	// Rules 5 and 6.
	if len(meta.Streams) == 0 {
		return File{}, errors.New("datafile: no streams")
	}
	if meta.Streams[0].Name != specs[specTime].name {
		return File{}, fmt.Errorf("datafile: first stream is %q, not time_offset", meta.Streams[0].Name)
	}
	seen := map[string]bool{}
	order := -1
	end := int64(align4(headerSize + int(l)))
	size := int64(len(b))
	for _, d := range meta.Streams {
		k, ok := specIndex(d.Name)
		if !ok {
			return File{}, fmt.Errorf("datafile: unknown stream %q", d.Name)
		}
		if seen[d.Name] {
			return File{}, fmt.Errorf("datafile: stream %s repeated", d.Name)
		}
		seen[d.Name] = true
		if k < order {
			return File{}, fmt.Errorf("datafile: stream %s out of order", d.Name)
		}
		order = k
		spec := specs[k]
		if err := checkDescriptor(d, spec); err != nil {
			return File{}, err
		}
		// Rule 6, overflow-safe (v1.2): off <= len(b) before any arithmetic,
		// then N <= (len(b) - off) / size, never off + N*size.
		off := int64(d.ByteOffset)
		if off%4 != 0 {
			return File{}, fmt.Errorf("datafile: column %s at byte %d is not 4-aligned", d.Name, off)
		}
		if off < 0 || off > size {
			return File{}, fmt.Errorf("datafile: column %s out of bounds", d.Name)
		}
		if off < end {
			return File{}, fmt.Errorf("datafile: column %s at byte %d overlaps the metadata or the previous column", d.Name, off)
		}
		if n > (size-off)/int64(spec.size) {
			return File{}, fmt.Errorf("datafile: column %s out of bounds", d.Name)
		}
		end = off + n*int64(spec.size)
		s := Stream{Descriptor: d}
		s.Ints, s.Valid = readColumn(b[off:end], spec, int(n))
		f.Streams = append(f.Streams, s)
	}
	if seen[specs[specLat].name] != seen[specs[specLon].name] {
		return File{}, errors.New("datafile: latitude and longitude must both be present or both absent")
	}

	// Rule 6, v1.1: nothing after the last column.
	if end != int64(len(b)) {
		return File{}, fmt.Errorf("datafile: %d trailing bytes after the last column", int64(len(b))-end)
	}

	// Rule 7 (Go Decode only).
	t := f.Streams[0]
	for i := range t.Ints {
		if !t.Valid[i] {
			return File{}, fmt.Errorf("datafile: time_offset missing at sample %d", i)
		}
		if i > 0 && t.Ints[i] <= t.Ints[i-1] {
			return File{}, fmt.Errorf("datafile: time_offset not strictly increasing at sample %d", i)
		}
	}
	if lat, ok := streamNamed(f.Streams, "latitude"); ok {
		lon, _ := streamNamed(f.Streams, "longitude")
		for i := range lat.Valid {
			if lat.Valid[i] != lon.Valid[i] {
				return File{}, fmt.Errorf("datafile: half a position at sample %d", i)
			}
		}
	}
	for i, s := range meta.Sessions {
		if err := checkRange("session", i, s.Start, s.End, s.StartIndex, s.EndIndex, int(n)); err != nil {
			return File{}, err
		}
	}
	for i, lp := range meta.Laps {
		if err := checkRange("lap", i, lp.Start, lp.End, lp.StartIndex, lp.EndIndex, int(n)); err != nil {
			return File{}, err
		}
	}
	for i, lg := range meta.Lengths {
		if err := checkRange("length", i, 0, 0, lg.StartIndex, lg.EndIndex, int(n)); err != nil {
			return File{}, err
		}
	}
	return f, nil
}

// decodeMeta checks Format §6 rule 4: valid UTF-8 JSON, every top-level key
// of §5.1 present with the right type.
func decodeMeta(js []byte) (Meta, error) {
	if !utf8.Valid(js) {
		return Meta{}, errors.New("datafile: metadata is not valid UTF-8")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(js, &top); err != nil {
		return Meta{}, fmt.Errorf("datafile: metadata: %w", err)
	}
	for _, k := range []struct{ key, kind string }{
		{"sport", "string"}, {"start_time", "string"}, {"time_zone", "object"}, {"summary", "object"},
		{"device", "object"}, {"sessions", "array"}, {"laps", "array"}, {"lengths", "array"}, {"zones", "object"}, {"streams", "array"},
	} {
		raw, ok := top[k.key]
		if !ok {
			return Meta{}, fmt.Errorf("datafile: metadata key %s missing", k.key)
		}
		if jsonKind(raw) != k.kind {
			return Meta{}, fmt.Errorf("datafile: metadata key %s is not a JSON %s", k.key, k.kind)
		}
	}
	if err := checkZones(top["zones"]); err != nil {
		return Meta{}, err
	}
	var m Meta
	d := json.NewDecoder(bytes.NewReader(js))
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		return Meta{}, fmt.Errorf("datafile: metadata: %w", err)
	}
	return m, nil
}

// jsonKind returns "string", "object", "array" or "other" for a raw value.
func jsonKind(raw json.RawMessage) string {
	raw = bytes.TrimLeft(raw, " \t\r\n")
	if len(raw) == 0 {
		return "other"
	}
	switch raw[0] {
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	}
	return "other"
}

// checkDescriptor compares a descriptor with its Format §2 row.
func checkDescriptor(d Descriptor, spec streamSpec) error {
	scale, err := d.Scale.Float64()
	if d.Type != spec.typ || d.Encoding != spec.encoding || d.Unit != spec.unit || err != nil || scale != spec.scale {
		return fmt.Errorf("datafile: descriptor of %s differs from the format", d.Name)
	}
	return nil
}

// readColumn decodes one column into absolute values and validity.
func readColumn(b []byte, spec streamSpec, n int) ([]int64, []bool) {
	raw := make([]int64, n)
	for i := range raw {
		switch spec.typ {
		case typeInt32:
			raw[i] = int64(int32(binary.LittleEndian.Uint32(b[4*i:])))
		case typeUint16:
			raw[i] = int64(binary.LittleEndian.Uint16(b[2*i:]))
		case typeUint8:
			raw[i] = int64(b[i])
		case typeInt8:
			raw[i] = int64(int8(b[i]))
		}
	}
	if spec.encoding == encDelta {
		return undelta(raw, spec.sentinel)
	}
	valid := make([]bool, n)
	for i, v := range raw {
		if v == spec.sentinel {
			raw[i] = 0
		} else {
			valid[i] = true
		}
	}
	return raw, valid
}

func streamNamed(streams []Stream, name string) (Stream, bool) {
	for _, s := range streams {
		if s.Descriptor.Name == name {
			return s, true
		}
	}
	return Stream{}, false
}

func checkRange(kind string, i int, start, end int32, from, to, n int) error {
	if end < start {
		return fmt.Errorf("datafile: %s %d: end before start", kind, i)
	}
	if from < 0 || from > to || to > n {
		return fmt.Errorf("datafile: %s %d index range [%d, %d) outside [0, %d]", kind, i, from, to, n)
	}
	return nil
}

// checkZones is Format §5.9 (rule 4, v1.9): only the kinds heart_rate and
// power, each from, basis or both (v1.10), every value in range.
func checkZones(raw json.RawMessage) error {
	var kinds map[string]json.RawMessage
	if err := json.Unmarshal(raw, &kinds); err != nil {
		return fmt.Errorf("datafile: zones: %w", err)
	}
	for kind, v := range kinds {
		bound, lo, hi := zoneLimits(kind)
		if bound == nil {
			return fmt.Errorf("datafile: zones: unknown kind %q", kind)
		}
		var e map[string]json.RawMessage
		if json.Unmarshal(v, &e) != nil {
			return fmt.Errorf("datafile: zones: %s needs from, basis or both", kind)
		}
		from, hasFrom := e["from"]
		b, hasBasis := e["basis"]
		if len(e) == 0 || len(e) != btoi(hasFrom)+btoi(hasBasis) {
			return fmt.Errorf("datafile: zones: %s needs from, basis or both", kind)
		}
		var xs []int64
		if hasFrom && (json.Unmarshal(from, &xs) != nil || !validFrom(xs, bound.min, bound.max)) {
			return fmt.Errorf("datafile: zones: %s from out of range", kind)
		}
		var basis int64
		if hasBasis && (json.Unmarshal(b, &basis) != nil || basis < int64(lo) || basis > int64(hi)) {
			return fmt.Errorf("datafile: zones: %s basis out of range", kind)
		}
	}
	return nil
}

// zoneLimits returns the kind's stream spec and basis range; nil spec for an
// unknown kind.
func zoneLimits(kind string) (*streamSpec, int, int) {
	lo, hi := workout.MinMaxHeartRate, workout.MaxMaxHeartRate
	if kind == "power" {
		lo, hi = workout.MinThresholdPower, workout.MaxThresholdPower
	} else if kind != "heart_rate" {
		return nil, 0, 0
	}
	s, _ := specByName(kind)
	return &s, lo, hi
}

// validFrom: 1 to 9 values, strictly ascending, within [lo, hi].
func validFrom(xs []int64, lo, hi int64) bool {
	if len(xs) < 1 || len(xs) > 9 {
		return false
	}
	for i, x := range xs {
		if x < lo || x > hi || i > 0 && x <= xs[i-1] {
			return false
		}
	}
	return true
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
