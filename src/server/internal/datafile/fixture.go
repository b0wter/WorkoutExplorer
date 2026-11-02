package datafile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

// fixtureVersion is bumped whenever the shape of the fixture JSON changes.
const fixtureVersion = 1

type fixture struct {
	FixtureVersion int             `json:"fixture_version"`
	FormatVersion  uint16          `json:"format_version"`
	ByteLength     int             `json:"byte_length"`
	SampleCount    int             `json:"sample_count"`
	MetadataLength int             `json:"metadata_length"`
	Streams        []fixtureStream `json:"streams"`
}

type fixtureStream struct {
	Name       string `json:"name"`
	ByteOffset int    `json:"byte_offset"`
	Valid      int    `json:"valid"`
	Missing    int    `json:"missing"`
	First      int64  `json:"first"`
	Last       int64  `json:"last"`
	Min        int64  `json:"min"`
	Max        int64  `json:"max"`
	Digest     string `json:"digest"`
}

// Fixture returns the golden JSON of a decoded file: counts, statistics and
// a digest of the stored absolute integers of every stream (F-03 design,
// section Golden fixtures). It is the cross-language contract for F-04.
//
// byte_length is the end of the last column, which Decode guarantees is the
// file length (Format §4, §6 rule 6).
func Fixture(f File) ([]byte, error) {
	fx := fixture{
		FixtureVersion: fixtureVersion,
		FormatVersion:  f.Version,
		SampleCount:    f.SampleCount,
		MetadataLength: f.MetadataLength,
		Streams:        make([]fixtureStream, 0, len(f.Streams)),
	}
	for _, s := range f.Streams {
		spec, ok := specByName(s.Descriptor.Name)
		if !ok {
			return nil, fmt.Errorf("datafile: unknown stream %q", s.Descriptor.Name)
		}
		fs := fixtureStream{Name: s.Descriptor.Name, ByteOffset: s.Descriptor.ByteOffset}
		h := sha256.New()
		var buf []byte
		for i, v := range s.Ints {
			if !s.Valid[i] {
				fs.Missing++
				h.Write([]byte("-\n"))
				continue
			}
			buf = strconv.AppendInt(buf[:0], v, 10)
			buf = append(buf, '\n')
			h.Write(buf)
			if fs.Valid == 0 {
				fs.First, fs.Min, fs.Max = v, v, v
			}
			fs.Last, fs.Min, fs.Max = v, min(fs.Min, v), max(fs.Max, v)
			fs.Valid++
		}
		if fs.Valid == 0 {
			return nil, fmt.Errorf("datafile: stream %s has no valid sample", fs.Name)
		}
		fs.Digest = hex.EncodeToString(h.Sum(nil))[:16]
		fx.ByteLength = s.Descriptor.ByteOffset + len(s.Ints)*spec.size
		fx.Streams = append(fx.Streams, fs)
	}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
