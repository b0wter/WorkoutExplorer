package ingest

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

// F-43 DI-1 to DI-3, DI-5 and F-44 PS-2.

func TestSanitiseName(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"ELEMNT BOLT", "ELEMNT BOLT"},
		{"  Edge \t\n 530  ", "Edge 530"},
		{"a\u202eb\u2066c\u200fd", "abcd"}, // bidi overrides, isolates and marks (Cf) removed
		{"a\x00b\x1bc\u0085d", "abc d"},    // NUL and ESC removed, NEL is white space
		{"\ufeffName\u200b", "Name"},       // BOM and zero-width space (Cf)
		{"x\u2028y\u00a0z", "x y z"},       // line separator, no-break space
		{"\xff broken", "\ufffd broken"},   // never met: the decoder drops invalid bytes (see TestDeviceNameAndSensors)
		{"\u202e\u202c\t", ""},             // nothing left
		{"<b>&amp;</b>", "<b>&amp;</b>"},   // markup is text; the renderers escape it
		{"\u00dcn\u00efc\u00f6d\u00e9 \u540d\u524d \U0001f6b4", "\u00dcn\u00efc\u00f6d\u00e9 \u540d\u524d \U0001f6b4"}, // kept as written
		{strings.Repeat("a", 41), strings.Repeat("a", 40)},
		{strings.Repeat("\u00e9", 45), strings.Repeat("\u00e9", 40)},
		{strings.Repeat("a", 39) + " b", strings.Repeat("a", 39)}, // the cut falls on the space
		{strings.Repeat("a", 38) + " bc", strings.Repeat("a", 38) + " b"},
		{"Corpusgen\u202e Extremely Long Product Name Beyond Forty Characters", "Corpusgen Extremely Long Product Name Be"},
	} {
		if got := sanitiseName(c.in); got != c.want {
			t.Errorf("sanitiseName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func FuzzSanitiseName(f *testing.F) {
	for _, s := range []string{"ELEMNT BOLT", "a\u202eb", "\xff", strings.Repeat("ab ", 30), "\u0085\u2028 x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := sanitiseName(s)
		if !utf8.ValidString(got) {
			t.Fatalf("invalid UTF-8 %q", got)
		}
		if n := utf8.RuneCountInString(got); n > maxModel {
			t.Fatalf("%d code points", n)
		}
		if got != strings.TrimSpace(got) || strings.Contains(got, "  ") {
			t.Fatalf("untrimmed or doubled space %q", got)
		}
		for _, r := range got {
			if unicode.In(r, unicode.Cc, unicode.Cf) || (unicode.IsSpace(r) && r != ' ') {
				t.Fatalf("rune %U in %q", r, got)
			}
		}
		if sanitiseName(got) != got {
			t.Fatalf("not idempotent: %q", got)
		}
	})
}

func TestReadable(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"wahoo_fitness", "Wahoo Fitness"}, {"edge_530", "Edge 530"}, {"garmin", "Garmin"}, {"fenix6", "Fenix6"},
		{"4iiiis", "4iiiis"}, {"development", ""}, {"unknown_9999", ""}, {"", ""},
	} {
		if got := readable(c.in); got != c.want {
			t.Errorf("readable(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSensorKind(t *testing.T) {
	ant, ble := typedef.SourceTypeAntplus, typedef.SourceTypeBluetoothLowEnergy
	for _, c := range []struct {
		source typedef.SourceType
		dt     uint8
		want   workout.Sensors
	}{
		{ant, uint8(typedef.AntplusDeviceTypeHeartRate), workout.SensorHeartRate},
		{ant, uint8(typedef.AntplusDeviceTypeBikePower), workout.SensorPower},
		{ant, uint8(typedef.AntplusDeviceTypeBikeSpeed), workout.SensorSpeed},
		{ant, uint8(typedef.AntplusDeviceTypeBikeCadence), workout.SensorCadence},
		{ant, uint8(typedef.AntplusDeviceTypeBikeSpeedCadence), workout.SensorSpeedCadence},
		{ant, uint8(typedef.AntplusDeviceTypeStrideSpeedDistance), workout.SensorFootPod},
		{ant, uint8(typedef.AntplusDeviceTypeBikeRadar), workout.SensorRadar},
		{ant, uint8(typedef.AntplusDeviceTypeShifting), 0},
		{ble, uint8(typedef.BleDeviceTypeHeartRate), workout.SensorHeartRate},
		{ble, uint8(typedef.BleDeviceTypeBikePower), workout.SensorPower},
		{ble, uint8(typedef.BleDeviceTypeBikeSpeed), workout.SensorSpeed},
		{ble, uint8(typedef.BleDeviceTypeBikeCadence), workout.SensorCadence},
		{ble, uint8(typedef.BleDeviceTypeBikeSpeedCadence), workout.SensorSpeedCadence},
		{ble, uint8(typedef.BleDeviceTypeFootpod), workout.SensorFootPod},
		{ble, uint8(typedef.BleDeviceTypeBikeTrainer), 0},
		{ble, uint8(typedef.AntplusDeviceTypeHeartRate), 0}, // 120 means nothing under BLE
		{typedef.SourceTypeLocal, uint8(typedef.AntplusDeviceTypeHeartRate), 0},
		{typedef.SourceTypeAnt, uint8(typedef.AntplusDeviceTypeHeartRate), 0},
		{typedef.SourceTypeBluetooth, uint8(typedef.BleDeviceTypeHeartRate), 0},
		{typedef.SourceTypeWifi, uint8(typedef.BleDeviceTypeHeartRate), 0},
		{typedef.SourceTypeInvalid, uint8(typedef.AntplusDeviceTypeHeartRate), 0}, // OQ-152: missing source type
	} {
		if got := sensorKind(c.source, c.dt); got != c.want {
			t.Errorf("sensorKind(%v, %d) = %v, want %v", c.source, c.dt, got, c.want)
		}
	}
}

func deviceFileID(m typedef.Manufacturer, product uint16, name string) proto.Message {
	f := mesgdef.NewFileId(nil).SetType(typedef.FileActivity).SetManufacturer(m).SetTimeCreated(t0)
	if product != 0 {
		f.SetProduct(product)
	}
	if name != "" {
		f.SetProductName(name)
	}
	return f.ToMesg(nil)
}

func devInfo(index uint8, source typedef.SourceType, deviceType uint8, name string) proto.Message {
	d := mesgdef.NewDeviceInfo(nil).SetTimestamp(t0).SetDeviceIndex(typedef.DeviceIndex(index)).SetSerialNumber(2_882_400_099)
	if source != typedef.SourceTypeInvalid {
		d.SetSourceType(source).SetDeviceType(deviceType)
	}
	if name != "" {
		d.SetProductName(name).SetDescriptor("Marker Descriptor")
	}
	return d.ToMesg(nil)
}

func TestDeviceNameAndSensors(t *testing.T) {
	ant := typedef.SourceTypeAntplus
	hr, pm := uint8(typedef.AntplusDeviceTypeHeartRate), uint8(typedef.AntplusDeviceTypeBikePower)
	for _, c := range []struct {
		name    string
		msgs    []proto.Message
		want    string
		sensors []string
	}{
		{"recorded name, brand prefix", []proto.Message{deviceFileID(typedef.ManufacturerWahooFitness, 31, "ELEMNT BOLT")}, "Wahoo Fitness ELEMNT BOLT", []string{}},
		{"no repeated brand", []proto.Message{deviceFileID(typedef.ManufacturerGarmin, 3121, "garmin Edge 530")}, "garmin Edge 530", []string{}},
		{"cut to 40", []proto.Message{deviceFileID(typedef.ManufacturerDevelopment, 1, strings.Repeat("x", 50))}, strings.Repeat("x", 40), []string{}},
		{"garmin profile name", []proto.Message{deviceFileID(typedef.ManufacturerGarmin, uint16(typedef.GarminProductEdge530), "")}, "Garmin Edge 530", []string{}},
		{"tacx is Garmin family", []proto.Message{deviceFileID(typedef.ManufacturerTacx, uint16(typedef.GarminProductEdge530), "")}, "Tacx Edge 530", []string{}},
		{"garmin number not in profile", []proto.Message{deviceFileID(typedef.ManufacturerGarmin, 65000, "")}, "Garmin", []string{}},
		{"other maker's number is not looked up", []proto.Message{deviceFileID(typedef.ManufacturerWahooFitness, uint16(typedef.GarminProductEdge530), "")}, "Wahoo Fitness", []string{}},
		{"creator's device_info name", []proto.Message{deviceFileID(typedef.ManufacturerWahooFitness, 63, ""), devInfo(0, typedef.SourceTypeLocal, 0, "ELEMNT BOLT")}, "Wahoo Fitness ELEMNT BOLT", []string{}},
		{"file_id name wins over the creator's", []proto.Message{deviceFileID(typedef.ManufacturerDevelopment, 1, "A"), devInfo(0, typedef.SourceTypeLocal, 0, "B")}, "A", []string{}},
		{"creator name empty after cleaning does not block a later one", []proto.Message{deviceFileID(typedef.ManufacturerGarmin, uint16(typedef.GarminProductEdge530), ""),
			devInfo(0, typedef.SourceTypeLocal, 0, "\u202e\u200b"), devInfo(0, typedef.SourceTypeLocal, 0, "Real Name")}, "Garmin Real Name", []string{}},
		{"first creator name wins", []proto.Message{deviceFileID(typedef.ManufacturerDevelopment, 1, ""),
			devInfo(0, typedef.SourceTypeLocal, 0, "One"), devInfo(0, typedef.SourceTypeLocal, 0, "Two")}, "One", []string{}},
		{"sanitised to empty falls back", []proto.Message{deviceFileID(typedef.ManufacturerGarmin, uint16(typedef.GarminProductFenix6), "\u202e\u200b")}, "Garmin Fenix6", []string{}},
		{"sensor product names never used", []proto.Message{deviceFileID(typedef.ManufacturerDevelopment, 1, ""), devInfo(1, ant, hr, "Marker Sensor")}, "", []string{"heart_rate_monitor"}},
		{"development, unknown and invalid makers are not known", []proto.Message{deviceFileID(typedef.ManufacturerDevelopment, 1, "")}, "", []string{}},
		{"maker not in the profile", []proto.Message{deviceFileID(typedef.Manufacturer(9999), 1, "")}, "", []string{}},
		{"order, once each, unknown and built-in left out", []proto.Message{deviceFileID(typedef.ManufacturerDevelopment, 1, ""),
			devInfo(1, ant, pm, ""), devInfo(2, ant, hr, ""), devInfo(3, typedef.SourceTypeBluetoothLowEnergy, uint8(typedef.BleDeviceTypeBikePower), ""),
			devInfo(4, ant, uint8(typedef.AntplusDeviceTypeShifting), ""), devInfo(5, typedef.SourceTypeLocal, hr, ""),
			devInfo(0, ant, hr, ""), devInfo(6, typedef.SourceTypeInvalid, 0, "")}, "", []string{"heart_rate_monitor", "power_meter"}},
	} {
		w := parseBytes(t, encodeFIT(t, append(c.msgs, record(0, 100).ToMesg(nil), record(1, 101).ToMesg(nil))...))
		if got := w.Device.Name(); got != c.want {
			t.Errorf("%s: name %q, want %q", c.name, got, c.want)
		}
		if got := w.Device.Sensors.Codes(); strings.Join(got, ",") != strings.Join(c.sensors, ",") {
			t.Errorf("%s: sensors %v, want %v", c.name, got, c.sensors)
		}
	}
}

// TestDeviceNameInvalidUTF8 (DI-2, QA-43-r1-2): the decoder drops invalid
// UTF-8 bytes before sanitiseName sees the name, so the rest is shown.
func TestDeviceNameInvalidUTF8(t *testing.T) {
	b := encodeFIT(t, deviceFileID(typedef.ManufacturerDevelopment, 1, "zzAB"), record(0, 100).ToMesg(nil), record(1, 101).ToMesg(nil))
	i := bytes.Index(b, []byte("zzAB"))
	copy(b[i:], "\xff\xfe")
	binary.LittleEndian.PutUint16(b[len(b)-2:], fitCRC(0, b[:len(b)-2]))
	if got := parseBytes(t, b).Device.Name(); got != "AB" {
		t.Errorf("name %q, want AB", got)
	}
}

// TestDeviceChained (DI-1, DI-5): the first sequence decides the name, the
// sensors are the union over every sequence kept; a skipped sequence adds none.
func TestDeviceChained(t *testing.T) {
	ant := typedef.SourceTypeAntplus
	first := encodeFIT(t, deviceFileID(typedef.ManufacturerWahooFitness, 31, "First"),
		devInfo(1, ant, uint8(typedef.AntplusDeviceTypeHeartRate), ""), record(0, 100).ToMesg(nil), record(1, 100).ToMesg(nil))
	second := encodeFIT(t, deviceFileID(typedef.ManufacturerGarmin, 3121, "Second"), devInfo(0, typedef.SourceTypeLocal, 0, "Creator Two"),
		devInfo(1, ant, uint8(typedef.AntplusDeviceTypeBikeRadar), ""), record(10, 100).ToMesg(nil), record(11, 100).ToMesg(nil))
	skipped := encodeFIT(t, settingsFileID(), devInfo(1, ant, uint8(typedef.AntplusDeviceTypeBikePower), ""))
	data := append(append(append([]byte{}, first...), second...), skipped...)
	w := parseBytes(t, data)
	if got := w.Device.Name(); got != "Wahoo Fitness First" {
		t.Errorf("name %q", got)
	}
	if got := strings.Join(w.Device.Sensors.Codes(), ","); got != "heart_rate_monitor,radar" {
		t.Errorf("sensors %s", got)
	}
}

func TestPoolRead(t *testing.T) {
	ses := func(sub typedef.SubSport, length uint16, unit typedef.DisplayMeasure, active uint16) proto.Message {
		s := mesgdef.NewSession(nil).SetTimestamp(at(1)).SetStartTime(at(0)).SetSport(typedef.SportSwimming).SetSubSport(sub).
			SetTotalElapsedTime(1000)
		if length != 0 {
			s.SetPoolLength(length).SetPoolLengthUnit(unit)
		}
		if active != 0 {
			s.SetNumActiveLengths(active)
		}
		return s.ToMesg(nil)
	}
	for _, c := range []struct {
		name string
		mesg proto.Message
		want workout.Pool
	}{
		{"metres", ses(typedef.SubSportLapSwimming, 2500, typedef.DisplayMeasureMetric, 64), workout.Pool{Length: 25, Lengths: 64}},
		{"yards", ses(typedef.SubSportLapSwimming, 2286, typedef.DisplayMeasureStatute, 40), workout.Pool{Length: 22.86, Statute: true, Lengths: 40}},
		{"not recorded", ses(typedef.SubSportLapSwimming, 0, 0, 0), workout.Pool{}},
	} {
		w := parseBytes(t, encodeFIT(t, fileID(), record(0, 100).ToMesg(nil), record(1, 101).ToMesg(nil), c.mesg))
		if len(w.Sessions) != 1 || w.Sessions[0].Pool != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, w.Sessions, c.want)
		}
	}
}

// TestPoolTruncated (F-44 AC 9, UP-3): a pool swim cut before its session
// message parses without pool values and without an error.
func TestPoolTruncated(t *testing.T) {
	b, err := os.ReadFile("../../testdata/corpus/synthetic/pool-swim.fit")
	if err != nil {
		t.Fatal(err)
	}
	full := parseBytes(t, b)
	if full.Summary.PoolLengthM == nil || *full.Summary.PoolLengthM != 25 || full.Summary.Lengths == nil || *full.Summary.Lengths != 40 {
		t.Fatalf("full file: %+v", full.Summary)
	}
	w := parseBytes(t, b[:len(b)*6/10])
	if !w.Integrity.Truncated || w.Summary.PoolLengthM != nil || w.Summary.PoolLengthYd != nil || w.Summary.Lengths != nil {
		t.Errorf("truncated: %+v, %+v", w.Integrity, w.Summary)
	}
}
