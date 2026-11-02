package datafile

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"workout-explorer/internal/workout"
)

// column is one stream ready to be written: absolute quantised values and
// their validity.
type column struct {
	spec  streamSpec
	q     []int64
	valid []bool
}

// Encode writes w as a data file, format version 2. It is deterministic (the
// same workout gives identical bytes, W-9) and returns an error instead of
// panicking when w breaks an invariant the format relies on (W-1). Errors
// never contain coordinates (W-10).
func Encode(w workout.Workout) ([]byte, error) {
	cols, err := columns(&w.Samples)
	if err != nil {
		return nil, err
	}
	for i, s := range w.Sessions {
		if err := checkSpan("session", i, s.Start, s.End); err != nil {
			return nil, err
		}
	}
	for i, l := range w.Laps {
		if err := checkSpan("lap", i, l.Start, l.End); err != nil {
			return nil, err
		}
	}
	n := len(w.Samples.Offset)

	meta := Meta{
		Sport:     w.Sport,
		StartTime: w.StartTime.UTC().Format(time.RFC3339),
		TimeZone:  TimeZone{Known: w.TimeZone.Known},
		Summary:   summaryOf(w),
		Sessions:  make([]Session, 0, len(w.Sessions)),
		Laps:      make([]Lap, 0, len(w.Laps)),
		Streams:   make([]Descriptor, len(cols)),
	}
	if w.TimeZone.Known {
		meta.TimeZone.OffsetSeconds = w.TimeZone.OffsetSeconds
	}
	for _, s := range w.Sessions {
		from, to := workout.SampleRange(w.Samples.Offset, s.Start, s.End)
		meta.Sessions = append(meta.Sessions, Session{
			Sport: s.Sport, SubSport: s.SubSport, Start: s.Start, End: s.End,
			StartIndex: from, EndIndex: to, Totals: totals(s.Totals),
		})
	}
	for _, l := range w.Laps {
		from, to := workout.SampleRange(w.Samples.Offset, l.Start, l.End)
		meta.Laps = append(meta.Laps, Lap{
			Start: l.Start, End: l.End, StartIndex: from, EndIndex: to, Totals: totals(l.Totals),
		})
	}
	for i, c := range cols {
		meta.Streams[i] = Descriptor{
			Name: c.spec.name, Type: c.spec.typ, Encoding: c.spec.encoding,
			Scale: json.Number(c.spec.scaleStr), Unit: c.spec.unit,
		}
	}

	// W-8: the offsets depend on L and L on the offsets. L only grows with
	// each pass, so this settles after two or three passes.
	var js []byte
	for l := -1; ; {
		js, err = json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("datafile: metadata: %w", err)
		}
		if len(js) == l {
			break
		}
		l = len(js)
		off := align4(headerSize + l)
		for i, c := range cols {
			meta.Streams[i].ByteOffset = off
			off = align4(off + n*c.spec.size)
		}
	}

	last := meta.Streams[len(cols)-1]
	size := last.ByteOffset + n*cols[len(cols)-1].spec.size
	buf := make([]byte, size) // zero padding throughout
	copy(buf, Magic)
	binary.LittleEndian.PutUint16(buf[4:], Version)
	binary.LittleEndian.PutUint32(buf[6:], uint32(n))
	binary.LittleEndian.PutUint32(buf[10:], uint32(len(js)))
	copy(buf[headerSize:], js)
	for i, c := range cols {
		writeColumn(buf[meta.Streams[i].ByteOffset:], c)
	}
	return buf, nil
}

func summaryOf(w workout.Workout) Summary {
	return Summary{Totals: totals(w.Summary.Totals), MovingTime: total(w.Summary.MovingTime)}
}

// SummaryJSON returns exactly the bytes of the "summary" value in the
// metadata of Encode(w), for the database row (F-09 design, Schema). The
// format is unchanged.
func SummaryJSON(w workout.Workout) ([]byte, error) {
	b, err := json.Marshal(summaryOf(w))
	if err != nil {
		return nil, fmt.Errorf("datafile: summary: %w", err)
	}
	return b, nil
}

// columns checks the invariants of W-1 and returns the present streams in
// column order, with the pairing rule W-3 applied.
func columns(s *workout.Samples) ([]column, error) {
	n := len(s.Offset)
	if n == 0 {
		return nil, errors.New("datafile: no samples")
	}
	t := column{spec: specs[specTime], q: make([]int64, n), valid: make([]bool, n)}
	for i, off := range s.Offset {
		if off < 0 || off > maxAbs {
			return nil, fmt.Errorf("datafile: time offset %d out of range at sample %d", off, i)
		}
		if i > 0 && off <= s.Offset[i-1] {
			return nil, fmt.Errorf("datafile: time offsets not strictly increasing at sample %d", i)
		}
		t.q[i], t.valid[i] = int64(off), true
	}
	cols := []column{t}

	all := make([]*column, len(specs))
	for k, spec := range specs[1:] {
		v := spec.samples(s)
		if v == nil {
			continue
		}
		if len(v) != n {
			return nil, fmt.Errorf("datafile: stream %s has %d values for %d samples", spec.name, len(v), n)
		}
		c := column{spec: spec, q: make([]int64, n), valid: make([]bool, n)}
		for i, x := range v {
			c.q[i], c.valid[i] = quantise(x, spec.factor, spec.min, spec.max)
		}
		all[k+1] = &c
	}

	// W-3: positions are a pair; either both are written or neither.
	lat, lon := all[specLat], all[specLon]
	if lat == nil || lon == nil {
		all[specLat], all[specLon] = nil, nil
	} else {
		for i := range n {
			if !lat.valid[i] || !lon.valid[i] {
				lat.q[i], lat.valid[i] = 0, false
				lon.q[i], lon.valid[i] = 0, false
			}
		}
	}

	// W-2: a stream without a valid value is not written.
	for _, c := range all[1:] {
		if c != nil && anyValid(c.valid) {
			cols = append(cols, *c)
		}
	}
	return cols, nil
}

func anyValid(v []bool) bool {
	for _, ok := range v {
		if ok {
			return true
		}
	}
	return false
}

// writeColumn writes one column little-endian at the start of dst.
func writeColumn(dst []byte, c column) {
	vals := c.q
	if c.spec.encoding == encDelta {
		vals = deltas(c.q, c.valid, c.spec.sentinel)
	} else {
		vals = make([]int64, len(c.q))
		for i := range c.q {
			if c.valid[i] {
				vals[i] = c.q[i]
			} else {
				vals[i] = c.spec.sentinel
			}
		}
	}
	for i, v := range vals {
		switch c.spec.typ {
		case typeInt32:
			binary.LittleEndian.PutUint32(dst[4*i:], uint32(int32(v)))
		case typeUint16:
			binary.LittleEndian.PutUint16(dst[2*i:], uint16(v))
		case typeUint8:
			dst[i] = uint8(v)
		case typeInt8:
			dst[i] = uint8(int8(v))
		}
	}
}

// checkSpan rejects a session or lap with a negative start or an end before
// its start (W-1, v1.2). The message names the index, never the values.
func checkSpan(kind string, i int, start, end int32) error {
	if start < 0 {
		return fmt.Errorf("datafile: %s %d: start before 0", kind, i)
	}
	if end < start {
		return fmt.Errorf("datafile: %s %d: end before start", kind, i)
	}
	return nil
}

// totals converts device totals for the metadata (Format §5.2, W-6).
func totals(t workout.Totals) Totals {
	return Totals{
		ElapsedTime: total(t.ElapsedTime), TimerTime: total(t.TimerTime),
		Distance: total(t.Distance), Ascent: total(t.Ascent), Descent: total(t.Descent),
		Calories: total(t.Calories), AvgSpeed: total(t.AvgSpeed), MaxSpeed: total(t.MaxSpeed),
		AvgHeartRate: total(t.AvgHeartRate), MaxHeartRate: total(t.MaxHeartRate),
		AvgCadence: total(t.AvgCadence), MaxCadence: total(t.MaxCadence),
		AvgPower: total(t.AvgPower), MaxPower: total(t.MaxPower),
		AvgTemperature: total(t.AvgTemperature), MaxTemperature: total(t.MaxTemperature),
	}
}
