package ingest

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/kit/datetime"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

var t0 = time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)

func at(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }

// encodeFIT encodes messages into a FIT file with the library's encoder.
func encodeFIT(t *testing.T, msgs ...proto.Message) []byte {
	t.Helper()
	fit := proto.FIT{FileHeader: proto.FileHeader{ProtocolVersion: proto.V2}, Messages: msgs}
	var buf bytes.Buffer
	if err := encoder.New(&buf).Encode(&fit); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

// testParser has the production limits and no logger.
var testParser = NewParser(DefaultLimits, nil)

func parseBytes(t *testing.T, b []byte) workout.Workout {
	t.Helper()
	w, err := testParser.Parse(context.Background(), b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return w
}

const serial = 3987654321

func fileID() proto.Message {
	return mesgdef.NewFileId(nil).
		SetType(typedef.FileActivity).
		SetManufacturer(typedef.ManufacturerGarmin).
		SetProduct(3122).
		SetProductName("Edge Test").
		SetSerialNumber(serial).
		SetTimeCreated(t0).
		ToMesg(nil)
}

func record(s int, hr uint8) *mesgdef.Record {
	return mesgdef.NewRecord(nil).SetTimestamp(at(s)).SetHeartRate(hr)
}

func unknownField(num byte, v uint16) proto.Field {
	return proto.Field{FieldBase: &proto.FieldBase{Name: "unknown", Num: num, BaseType: basetype.Uint16, Scale: 1}, Value: proto.Uint16(v)}
}

func TestListenerIgnoresUnknownMessagesFieldsAndDeveloperFields(t *testing.T) {
	r0 := record(0, 120).ToMesg(nil)
	r0.Fields = append(r0.Fields, unknownField(250, 7))
	r1 := record(1, 121).ToMesg(nil)
	r1.DeveloperFields = []proto.DeveloperField{{Num: 0, DeveloperDataIndex: 0, Value: proto.Uint8(42)}}
	mfg := proto.Message{Num: typedef.MesgNum(0xFF01), Fields: []proto.Field{unknownField(1, 99)}}
	unknownGlobal := proto.Message{Num: typedef.MesgNum(0x4000), Fields: []proto.Field{unknownField(0, 5)}}

	b := encodeFIT(t,
		fileID(),
		mesgdef.NewDeveloperDataId(nil).SetDeveloperDataIndex(0).ToMesg(nil),
		mesgdef.NewFieldDescription(nil).SetDeveloperDataIndex(0).SetFieldDefinitionNumber(0).
			SetFitBaseTypeId(basetype.Uint8).SetFieldName([]string{"dev"}).ToMesg(nil),
		r0, mfg, r1, unknownGlobal,
		record(2, 122).ToMesg(nil),
	)
	w := parseBytes(t, b)
	if got := len(w.Samples.Offset); got != 3 {
		t.Fatalf("samples = %d, want 3", got)
	}
	if got := w.Samples.HeartRate; got[0] != 120 || got[1] != 121 || got[2] != 122 {
		t.Errorf("HeartRate = %v", got)
	}
}

func TestListenerDeviceWithoutSerialNumber(t *testing.T) {
	w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil)))
	want := workout.Device{Manufacturer: "garmin", ProductID: 3122, ProductName: "Edge Test", Brand: "Garmin", Model: "Edge Test"}
	if w.Device != want {
		t.Errorf("Device = %+v, want %+v", w.Device, want)
	}
	if w.FileType != "activity" {
		t.Errorf("FileType = %q, want activity", w.FileType)
	}
	report, err := workout.ReportJSON(w)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(report, []byte(strconv.Itoa(serial))) {
		t.Errorf("serial number appears in the normalised workout")
	}
}

func TestListenerTimerEvents(t *testing.T) {
	ev := func(s int, e typedef.Event, et typedef.EventType) proto.Message {
		return mesgdef.NewEvent(nil).SetTimestamp(at(s)).SetEvent(e).SetEventType(et).ToMesg(nil)
	}
	b := encodeFIT(t,
		fileID(),
		ev(0, typedef.EventTimer, typedef.EventTypeStart),
		record(0, 100).ToMesg(nil),
		ev(1, typedef.EventTimer, typedef.EventTypeStop),
		ev(2, typedef.EventTimer, typedef.EventTypeStopAll),
		ev(3, typedef.EventTimer, typedef.EventTypeStopDisable),
		ev(4, typedef.EventTimer, typedef.EventTypeStopDisableAll),
		ev(5, typedef.EventTimer, typedef.EventTypeMarker), // other type: dropped
		ev(6, typedef.EventLap, typedef.EventTypeStop),     // not a timer event: dropped
		ev(7, typedef.EventTimer, typedef.EventTypeStart),
		record(10, 100).ToMesg(nil),
	)
	w := parseBytes(t, b)
	want := []workout.TimerEvent{
		{Offset: 0, Kind: workout.TimerStart},
		{Offset: 1, Kind: workout.TimerStop},
		{Offset: 2, Kind: workout.TimerStop},
		{Offset: 3, Kind: workout.TimerStop},
		{Offset: 4, Kind: workout.TimerStop},
		{Offset: 7, Kind: workout.TimerStart},
	}
	if len(w.Timer) != len(want) {
		t.Fatalf("Timer = %+v, want %+v", w.Timer, want)
	}
	for i := range want {
		if w.Timer[i] != want[i] {
			t.Errorf("Timer[%d] = %+v, want %+v", i, w.Timer[i], want[i])
		}
	}
}

func TestListenerSessionsLapsAndActivity(t *testing.T) {
	session := func(sport typedef.Sport, from, to int) proto.Message {
		return mesgdef.NewSession(nil).SetStartTime(at(from)).SetTimestamp(at(to)).SetSport(sport).
			SetSubSport(typedef.SubSportGeneric).SetTotalElapsedTime(uint32((to - from) * 1000)).
			SetAvgCadence(85).SetAvgFractionalCadence(64).ToMesg(nil)
	}
	lap := mesgdef.NewLap(nil).SetStartTime(at(0)).SetTotalElapsedTime(10000).ToMesg(nil) // no timestamp: end from elapsed
	activity := mesgdef.NewActivity(nil).SetTimestamp(at(20)).SetLocalTimestamp(at(20 + 3600)).ToMesg(nil)
	b := encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), lap, session(typedef.SportRunning, 0, 10),
		record(20, 100).ToMesg(nil), session(typedef.SportCycling, 10, 20), activity)
	w := parseBytes(t, b)
	if w.Sport != "multisport" || len(w.Sessions) != 2 {
		t.Fatalf("Sport = %q with %d sessions, want multisport with 2", w.Sport, len(w.Sessions))
	}
	if w.Sessions[0].Sport != "running" || w.Sessions[1].Sport != "cycling" || w.Sessions[0].SubSport != "generic" {
		t.Errorf("sessions = %+v", w.Sessions)
	}
	if got := w.Sessions[0].Totals.AvgCadence; got == nil || *got != 171 {
		t.Errorf("running avg cadence = %v, want 171", got)
	}
	if l := w.Laps[0]; l.Start != 0 || l.End != 10 {
		t.Errorf("lap span = [%d, %d], want [0, 10]", l.Start, l.End)
	}
	if w.TimeZone != (workout.TimeZone{Known: true, OffsetSeconds: 3600}) {
		t.Errorf("TimeZone = %+v", w.TimeZone)
	}
}

func TestListenerDropsRecordsWithoutTimestamp(t *testing.T) {
	noTime := mesgdef.NewRecord(nil).SetHeartRate(99).ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fileID(), noTime, record(0, 100).ToMesg(nil)))
	if len(w.Samples.Offset) != 1 || w.Samples.HeartRate[0] != 100 {
		t.Errorf("samples = %v / %v, want one at 100 bpm", w.Samples.Offset, w.Samples.HeartRate)
	}
}

func TestParseErrors(t *testing.T) {
	valid := encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), record(1, 100).ToMesg(nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testParser.Parse(ctx, valid); err == nil {
		t.Errorf("cancelled context: want error")
	}
	if _, err := testParser.Parse(context.Background(), []byte("this is not a FIT file at all")); err == nil {
		t.Errorf("non-FIT bytes: want error")
	}
	if _, err := testParser.Parse(context.Background(), nil); err == nil {
		t.Errorf("empty input: want error")
	}
}

// TestParseBothFieldVariants pins the decoder option set (D-11): with
// component expansion on, the legacy value would overwrite enhanced_*.
func TestParseBothFieldVariants(t *testing.T) {
	both := mesgdef.NewRecord(nil).SetTimestamp(at(0)).
		SetAltitude(3000).SetEnhancedAltitude(4000).
		SetSpeed(1000).SetEnhancedSpeed(5000).ToMesg(nil)
	legacyOnly := mesgdef.NewRecord(nil).SetTimestamp(at(1)).SetAltitude(3000).SetSpeed(1234).ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fileID(), both, legacyOnly))
	if got := w.Samples.Altitude; got[0] != 300 || got[1] != 100 {
		t.Errorf("Altitude = %v, want [300 100] (enhanced 4000 → 300 m; legacy only 3000 → 100 m)", got)
	}
	if got := w.Samples.Speed; got[0] != 5 || got[1] != 1.234 {
		t.Errorf("Speed = %v, want [5 1.234] (enhanced preferred; legacy-only speed kept)", got)
	}
}

func TestListenerSecondFileIDIgnored(t *testing.T) {
	second := mesgdef.NewFileId(nil).SetType(typedef.FileCourse).SetManufacturer(typedef.ManufacturerWahooFitness).
		SetProduct(7).SetProductName("Other").ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), second))
	if w.FileType != "activity" || w.Device.Manufacturer != "garmin" || w.Device.ProductName != "Edge Test" {
		t.Errorf("FileType %q, Device %+v; want the first file_id (E-4)", w.FileType, w.Device)
	}
}

func TestListenerTimerEventWithoutTimestampDropped(t *testing.T) {
	noTime := mesgdef.NewEvent(nil).SetEvent(typedef.EventTimer).SetEventType(typedef.EventTypeStop).ToMesg(nil)
	start := mesgdef.NewEvent(nil).SetTimestamp(at(0)).SetEvent(typedef.EventTimer).SetEventType(typedef.EventTypeStart).ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fileID(), start, record(0, 100).ToMesg(nil), noTime, record(5, 100).ToMesg(nil)))
	if len(w.Timer) != 1 || w.Timer[0].Kind != workout.TimerStart {
		t.Errorf("Timer = %+v, want only the start (E-3)", w.Timer)
	}
}

func TestListenerEnumSentinel(t *testing.T) {
	// A session and file_id whose enum fields are not recorded (E-1).
	fid := mesgdef.NewFileId(nil).SetProductName("x").ToMesg(nil)
	session := mesgdef.NewSession(nil).SetStartTime(at(0)).SetTimestamp(at(10)).ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fid, record(0, 100).ToMesg(nil), session))
	if w.FileType != "" || w.Device.Manufacturer != "" {
		t.Errorf("FileType %q, Manufacturer %q; want empty", w.FileType, w.Device.Manufacturer)
	}
	if len(w.Sessions) != 1 || w.Sessions[0].Sport != "" || w.Sessions[0].SubSport != "" || w.Sport != "" {
		t.Errorf("Sport %q, sessions %+v; want empty sport fields", w.Sport, w.Sessions)
	}
}

func TestSessionLapTimes(t *testing.T) {
	// E-2: start and end completed in ingest, then mapped to offsets.
	lap := func(start, end int, elapsed uint32) proto.Message {
		m := mesgdef.NewLap(nil).SetTotalDistance(100) // the encoder refuses a message without fields
		if start >= 0 {
			m.SetStartTime(at(start))
		}
		if end >= 0 {
			m.SetTimestamp(at(end))
		}
		if elapsed != basetype.Uint32Invalid {
			m.SetTotalElapsedTime(elapsed)
		}
		return m.ToMesg(nil)
	}
	tests := []struct {
		name       string
		lap        proto.Message
		start, end int32
	}{
		{"start from end − elapsed", lap(-1, 30, 20000), 10, 30},
		{"end from start + elapsed", lap(10, -1, 5000), 10, 15},
		{"everything missing", lap(-1, -1, basetype.Uint32Invalid), 0, 0},
		{"end before start", lap(20, 15, basetype.Uint32Invalid), 20, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), record(40, 100).ToMesg(nil), tt.lap))
			if l := w.Laps[0]; l.Start != tt.start || l.End != tt.end {
				t.Errorf("lap = [%d, %d], want [%d, %d]", l.Start, l.End, tt.start, tt.end)
			}
		})
	}
}

// systemTime returns the time.Time the library decodes from a raw FIT
// date_time below 0x10000000 (seconds since the device powered on, E-8).
func systemTime(raw uint32) time.Time { return datetime.ToTime(raw) }

func TestSystemTimeRecordDropped(t *testing.T) {
	msgs := []proto.Message{fileID()}
	for s := 0; s < 600; s++ {
		msgs = append(msgs, record(s, 120).ToMesg(nil))
		if s == 300 {
			msgs = append(msgs, mesgdef.NewRecord(nil).SetTimestamp(systemTime(1000)).SetHeartRate(99).ToMesg(nil))
		}
	}
	w := parseBytes(t, encodeFIT(t, msgs...))
	if !w.StartTime.Equal(at(0)) {
		t.Errorf("StartTime = %v, want %v", w.StartTime, at(0))
	}
	if n := len(w.Samples.Offset); n != 600 {
		t.Errorf("samples = %d, want 600", n)
	}
	if e := w.Derived.ElapsedTime; e == nil || *e != 599 {
		t.Errorf("elapsed = %v, want 599", e)
	}
}

func TestSystemTimeSessionStartFallsBack(t *testing.T) {
	session := mesgdef.NewSession(nil).SetStartTime(systemTime(1000)).SetTimestamp(at(30)).
		SetTotalElapsedTime(20000).SetSport(typedef.SportCycling).ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), record(30, 100).ToMesg(nil), session))
	if s := w.Sessions[0]; s.Start != 10 || s.End != 30 {
		t.Errorf("session = [%d, %d], want [10, 30] (start from end − elapsed, E-2)", s.Start, s.End)
	}
	if !w.StartTime.Equal(at(0)) {
		t.Errorf("StartTime = %v, want %v", w.StartTime, at(0))
	}
}

func TestSystemTimeLocalTimestampUnknownZone(t *testing.T) {
	activity := mesgdef.NewActivity(nil).SetTimestamp(at(10)).SetLocalTimestamp(systemTime(5000)).ToMesg(nil)
	w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), activity))
	if w.TimeZone.Known {
		t.Errorf("TimeZone = %+v, want unknown", w.TimeZone)
	}
}

// TestSystemTimeActivityUnknownZone covers E-8 at the activity call site
// (QA-02-r2-1): each case would give a known zone of ±7,200 s if that
// field's system-time check were missing.
func TestSystemTimeActivityUnknownZone(t *testing.T) {
	const boundary = 0x10000000
	tests := []struct {
		name             string
		timestamp, local uint32
	}{
		{"both in system time", 1000, 1000 + 7200},
		{"timestamp in system time", boundary - 3600, boundary + 3600},
		{"local_timestamp in system time", boundary + 3600, boundary - 3600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := mesgdef.NewActivity(nil).SetTimestamp(datetime.ToTime(tt.timestamp)).
				SetLocalTimestamp(datetime.ToTime(tt.local)).ToMesg(nil)
			w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), activity))
			if w.TimeZone.Known {
				t.Errorf("TimeZone = %+v, want unknown (E-8)", w.TimeZone)
			}
		})
	}
}
