package ingest

import (
	"bytes"
	"context"
	"math"
	"testing"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/proto"
)

// TestStrydInvalidBaseTypeID: a description whose base type id is invalid
// makes the decoder fail the sequence at the first value; the records before
// it are kept, nothing panics and no power comes out (F-68 Reliability).
// (A value whose type differs from the description's cannot be encoded by the
// library, so that guard is covered by TestStrydValueRules.)
func TestStrydInvalidBaseTypeID(t *testing.T) {
	rec := func(s int) proto.Message {
		m := record(s, 120).ToMesg(nil)
		m.DeveloperFields = []proto.DeveloperField{{Num: 0, DeveloperDataIndex: 0, Value: proto.Uint16(200)}}
		return m
	}
	id := mesgdef.NewDeveloperDataId(nil).SetDeveloperDataIndex(0).ToMesg(nil)
	b := encodeFIT(t, fileID(), id, desc(0, 0, basetype.Uint16, "Power", "Watts", nil),
		record(0, 100).ToMesg(nil), rec(1), rec(2))
	i := bytes.Index(b, []byte("Power"))
	if i < 0 {
		t.Fatal("no name in the file")
	}
	// field_description fields in number order: index, number, base type id, name.
	j := bytes.LastIndex(b[:i], []byte{0, 0, byte(basetype.Uint16)})
	if j < 0 {
		t.Fatal("no description bytes")
	}
	b[j+2] = 0x99
	w, err := testParser.Parse(context.Background(), b)
	if err != nil {
		t.Logf("refused: %v", err)
		return
	}
	for _, p := range w.Samples.Power {
		if !math.IsNaN(p) {
			t.Fatalf("power from a description with an invalid base type: %v", w.Samples.Power)
		}
	}
	t.Logf("recovered %d samples", len(w.Samples.Offset))
}
