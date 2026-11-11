package datafile

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

// FirstPosition is the first sample of f with a position, in degrees; ok is
// false without one (F-45 ST-4).
func (f File) FirstPosition() (lat, lon float64, ok bool) {
	la, okLat := streamNamed(f.Streams, "latitude")
	lo, okLon := streamNamed(f.Streams, "longitude")
	if !okLat || !okLon {
		return 0, 0, false
	}
	for i := range la.Valid {
		if la.Valid[i] && lo.Valid[i] {
			return float64(la.Ints[i]) * 1e-6, float64(lo.Ints[i]) * 1e-6, true
		}
	}
	return 0, 0, false
}

// FirstPositionAt is FirstPosition for an encoded file, reading the header,
// the metadata (returned) and the two position columns up to the answer, outside the
// inclusive sample ranges skip (F-45 ST-6): no decode, memory independent of
// the sample count. Any malformation is an error.
func FirstPositionAt(r io.ReaderAt, size int64, skip [][2]int) (m Meta, lat, lon float64, ok bool, err error) {
	bad := errors.New("datafile: unreadable")
	var h [headerSize]byte
	if _, err := r.ReadAt(h[:], 0); err != nil || string(h[:4]) != Magic || binary.LittleEndian.Uint16(h[4:]) != Version {
		return m, 0, 0, false, bad
	}
	n := int64(binary.LittleEndian.Uint32(h[6:]))
	l := int64(binary.LittleEndian.Uint32(h[10:]))
	if headerSize+l > size {
		return m, 0, 0, false, bad
	}
	js := make([]byte, l)
	if _, err := r.ReadAt(js, headerSize); err != nil {
		return m, 0, 0, false, bad
	}
	if err := json.Unmarshal(js, &m); err != nil {
		return m, 0, 0, false, bad
	}
	meta := m
	offs := map[string]int64{}
	for _, d := range meta.Streams {
		offs[d.Name] = int64(d.ByteOffset)
	}
	lao, okLat := offs["latitude"]
	loo, okLon := offs["longitude"]
	if !okLat || !okLon {
		return m, 0, 0, false, nil
	}
	for _, o := range []int64{lao, loo} {
		if o < 0 || o%4 != 0 || o > size || n > (size-o)/4 {
			return m, 0, 0, false, bad
		}
	}
	la := bufio.NewReaderSize(io.NewSectionReader(r, lao, n*4), 4096)
	lo := bufio.NewReaderSize(io.NewSectionReader(r, loo, n*4), 4096)
	var a, b, x, y int64
	var v [4]byte // one buffer for every read: no allocation per sample
	next := func(br *bufio.Reader, acc *int64) (bool, error) {
		if _, err := io.ReadFull(br, v[:]); err != nil {
			return false, bad
		}
		d := int64(int32(binary.LittleEndian.Uint32(v[:])))
		if d == specs[specLat].sentinel {
			return false, nil
		}
		*acc += d
		return true, nil
	}
	for i := 0; int64(i) < n; i++ {
		okA, e1 := next(la, &a)
		okB, e2 := next(lo, &b)
		if e1 != nil || e2 != nil {
			return m, 0, 0, false, bad
		}
		x, y = a, b
		skipped := false
		for _, s := range skip {
			skipped = skipped || i >= s[0] && i <= s[1]
			if i >= s[0] && s[1] >= int(n)-1 { // the rest is hidden: nothing left to find
				return m, 0, 0, false, nil
			}
		}
		if okA && okB && !skipped {
			return m, float64(x) * 1e-6, float64(y) * 1e-6, true, nil
		}
	}
	return m, 0, 0, false, nil
}
