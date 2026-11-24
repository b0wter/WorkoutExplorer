package ingest

import (
	"math"
	"slices"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/fieldnum"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

// F-68 XS-3, XS-4, XS-7: the one developer field that is read, Stryd power.
// Names, units and IDs are compared here and never leave this file: the
// state lives during the decode, and only a float64 watt value goes on.

// F68-Q1: the strings Stryd writes; verify on a real file before release.
const (
	strydName  = "Power"
	strydUnits = "Watts"
)

type devKey struct{ index, num uint8 }

// strydReader tracks developer_data_id and field_description messages of one
// FIT sequence and reads the Stryd power value from record messages.
type strydReader struct {
	cands []devKey              // pairs whose current description qualifies, in file order of first description
	types map[devKey]proto.Type // the value type each promises
	bad   map[devKey]bool       // pairs that wrote a value of another shape (array, string, other type)
	src   *devKey               // the pair that gave the first valid value: locked as the source (XS-4)
}

// reset starts a new FIT sequence (XS-4).
func (s *strydReader) reset() {
	s.cands, s.types, s.bad, s.src = nil, nil, nil, nil
}

func (s *strydReader) drop(k devKey) {
	s.cands = slices.DeleteFunc(s.cands, func(c devKey) bool { return c == k })
	delete(s.types, k)
}

// developerDataID forgets the descriptions of an index (XS-4).
func (s *strydReader) developerDataID(m *proto.Message) {
	idx := m.FieldValueByNum(fieldnum.DeveloperDataIdDeveloperDataIndex).Uint8()
	for _, k := range slices.Clone(s.cands) {
		if k.index == idx {
			s.drop(k)
		}
	}
}

// description replaces the description of a pair (XS-4).
func (s *strydReader) description(f *mesgdef.FieldDescription) {
	k := devKey{f.DeveloperDataIndex, f.FieldDefinitionNumber}
	typ, ok := qualifies(f)
	if !ok {
		s.drop(k)
		return
	}
	if s.types == nil {
		s.types, s.bad = map[devKey]proto.Type{}, map[devKey]bool{}
	}
	if _, had := s.types[k]; !had {
		s.cands = append(s.cands, k)
	}
	s.types[k] = typ
	delete(s.bad, k)
}

// qualifies is the XS-3 test on a description; it returns the value type the
// description promises.
func qualifies(f *mesgdef.FieldDescription) (proto.Type, bool) {
	if len(f.FieldName) == 0 || f.FieldName[0] != strydName || len(f.Units) == 0 || f.Units[0] != strydUnits {
		return 0, false
	}
	if f.Scale != basetype.Uint8Invalid && f.Scale != 1 {
		return 0, false
	}
	if f.Offset != basetype.Sint8Invalid && f.Offset != 0 {
		return 0, false
	}
	if f.NativeMesgNum != typedef.MesgNum(basetype.Uint16Invalid) || f.NativeFieldNum != basetype.Uint8Invalid {
		if f.NativeMesgNum != mesgnum.Record || f.NativeFieldNum != fieldnum.RecordPower {
			return 0, false
		}
	}
	switch f.FitBaseTypeId {
	case basetype.Sint8:
		return proto.TypeInt8, true
	case basetype.Uint8:
		return proto.TypeUint8, true
	case basetype.Sint16:
		return proto.TypeInt16, true
	case basetype.Uint16:
		return proto.TypeUint16, true
	case basetype.Sint32:
		return proto.TypeInt32, true
	case basetype.Uint32:
		return proto.TypeUint32, true
	case basetype.Float32:
		return proto.TypeFloat32, true
	case basetype.Float64:
		return proto.TypeFloat64, true
	}
	return 0, false
}

// power returns the Stryd power of one record message and whether it has a
// valid one. The source is the first qualifying pair in file order that has
// not written a value of another shape (XS-4), also while it has no value yet.
// Array and string are known only from a value, and the decoder reads a value
// with the first description of a pair, so a value of another type than the
// current description promises marks its pair as not usable.
func (s *strydReader) power(m *proto.Message) (float64, bool) {
	for _, k := range s.cands {
		if s.src != nil && k != *s.src {
			continue
		}
		if s.bad[k] {
			continue
		}
		for i := range m.DeveloperFields {
			d := &m.DeveloperFields[i]
			if d.DeveloperDataIndex != k.index || d.Num != k.num {
				continue
			}
			if d.Value.Type() != s.types[k] {
				if s.src == nil { // before the lock the pair is unusable; after it only this record is missing
					s.bad[k] = true
				}
				return 0, false
			}
			w, ok := validWatts(d.Value)
			if ok && s.src == nil {
				s.src = &k
			}
			return w, ok
		}
		return 0, false // the source has no value in this record
	}
	return 0, false
}

// validWatts converts a scalar value that is not its base type's invalid
// value and lies in native power's range, 0 to 65,534 W (QA-68-r1-1).
func validWatts(v proto.Value) (float64, bool) {
	var x float64
	switch v.Type() {
	case proto.TypeInt8:
		if v.Int8() == basetype.Sint8Invalid {
			return 0, false
		}
		x = float64(v.Int8())
	case proto.TypeUint8:
		if v.Uint8() == basetype.Uint8Invalid {
			return 0, false
		}
		x = float64(v.Uint8())
	case proto.TypeInt16:
		if v.Int16() == basetype.Sint16Invalid {
			return 0, false
		}
		x = float64(v.Int16())
	case proto.TypeUint16:
		if v.Uint16() == basetype.Uint16Invalid {
			return 0, false
		}
		x = float64(v.Uint16())
	case proto.TypeInt32:
		if v.Int32() == basetype.Sint32Invalid {
			return 0, false
		}
		x = float64(v.Int32())
	case proto.TypeUint32:
		if v.Uint32() == basetype.Uint32Invalid {
			return 0, false
		}
		x = float64(v.Uint32())
	case proto.TypeFloat32:
		x = float64(v.Float32())
	case proto.TypeFloat64:
		x = v.Float64()
	default:
		return 0, false
	}
	if math.IsNaN(x) || x < 0 || x > math.MaxUint16-1 { // native power's range; also rejects Inf
		return 0, false
	}
	return x, true
}

// dynamic converts a raw record value of a running dynamics field (profile
// scale 10 for the first three, 100 for vertical ratio) to the stored
// resolution of data file version 8 (XS-1): 0 and the invalid value are
// missing, otherwise round(raw / 10) half up, clamped to the column's range.
func dynamic(raw uint16) uint16 {
	if raw == 0 || raw == basetype.Uint16Invalid {
		return 0
	}
	return uint16((uint32(raw) + 5) / 10) // raw <= 65534: at most 6553
}

// strydSample is a valid Stryd power value of record Records[index].
type strydSample struct {
	index int
	watts float64
}

// fillStryd implements XS-5: the Stryd values become the power stream only
// when no record of the whole upload has a valid native power value.
func fillStryd(recs []workout.RawRecord, samples []strydSample) {
	if len(samples) == 0 {
		return
	}
	for i := range recs {
		if !math.IsNaN(recs[i].Power) {
			return
		}
	}
	for _, s := range samples {
		recs[s.index].Power = s.watts
	}
}
