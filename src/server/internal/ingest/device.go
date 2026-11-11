package ingest

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/fieldnum"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

// F-43 DI-1 to DI-3, DI-5: the device name parts and the sensor kinds.

// maxModel is the cut of a recorded product name, in code points (DI-2).
const maxModel = 40

// deviceInfo reads the allow-listed fields of one device_info message by
// field number (DI-1): device_index, source_type, device_type and, for the
// creator in the first sequence, product_name. Every other field (serial
// and ANT+ numbers, descriptor, versions, battery, other product names) is
// never read.
func (l *listener) deviceInfo(m *proto.Message) {
	index := m.FieldValueByNum(fieldnum.DeviceInfoDeviceIndex).Uint8()
	if typedef.DeviceIndex(index) == typedef.DeviceIndexCreator {
		if l.seqNo <= 1 && l.creatorName == "" { // the first name left after cleaning wins (decision of 11 Nov 2026)
			l.creatorName = sanitiseName(m.FieldValueByNum(fieldnum.DeviceInfoProductName).String())
		}
		return // the recording device's own sensors are built in
	}
	source := typedef.SourceType(m.FieldValueByNum(fieldnum.DeviceInfoSourceType).Uint8())
	l.rec.Device.Sensors |= sensorKind(source, m.FieldValueByNum(fieldnum.DeviceInfoDeviceType).Uint8())
}

// sensorKind maps device_type, read under its source_type, to a recognised
// kind; 0 for everything else (DI-5). A missing source_type gives 0: the
// profile gives device_type no meaning without it (OQ-152).
func sensorKind(source typedef.SourceType, deviceType uint8) workout.Sensors {
	switch source {
	case typedef.SourceTypeAntplus:
		switch typedef.AntplusDeviceType(deviceType) {
		case typedef.AntplusDeviceTypeHeartRate:
			return workout.SensorHeartRate
		case typedef.AntplusDeviceTypeBikePower:
			return workout.SensorPower
		case typedef.AntplusDeviceTypeBikeSpeed:
			return workout.SensorSpeed
		case typedef.AntplusDeviceTypeBikeCadence:
			return workout.SensorCadence
		case typedef.AntplusDeviceTypeBikeSpeedCadence:
			return workout.SensorSpeedCadence
		case typedef.AntplusDeviceTypeStrideSpeedDistance:
			return workout.SensorFootPod
		case typedef.AntplusDeviceTypeBikeRadar:
			return workout.SensorRadar
		}
	case typedef.SourceTypeBluetoothLowEnergy:
		switch typedef.BleDeviceType(deviceType) {
		case typedef.BleDeviceTypeHeartRate:
			return workout.SensorHeartRate
		case typedef.BleDeviceTypeBikePower:
			return workout.SensorPower
		case typedef.BleDeviceTypeBikeSpeed:
			return workout.SensorSpeed
		case typedef.BleDeviceTypeBikeCadence:
			return workout.SensorCadence
		case typedef.BleDeviceTypeBikeSpeedCadence:
			return workout.SensorSpeedCadence
		case typedef.BleDeviceTypeFootpod:
			return workout.SensorFootPod
		}
	}
	return 0
}

// model is DI-2: the recorded product name (file_id, else the creator's
// device_info), sanitised; else the Garmin-family profile name made
// readable; else "".
func (l *listener) model() string {
	if s := sanitiseName(l.rec.Device.ProductName); s != "" {
		return s
	}
	if l.creatorName != "" {
		return l.creatorName
	}
	return readable(l.garminProduct)
}

// readable is DI-3: a profile name with "_" as spaces and the first letter of
// each word upper-case ("wahoo_fitness" → "Wahoo Fitness"); "" for a name
// that is not known (absent, development, not in the profile).
func readable(name string) string {
	if name == "" || name == "development" || strings.HasPrefix(name, "unknown_") {
		return ""
	}
	words := strings.Fields(strings.ReplaceAll(name, "_", " "))
	for i, w := range words {
		r, n := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[n:]
	}
	return strings.Join(words, " ")
}

// sanitiseName is DI-2's text rule: white space becomes a space, other control and format characters (Cc, Cf: bidi
// controls included) are removed, space runs collapse to one, trimmed, cut
// to maxModel code points and trimmed again.
// The decoder has already dropped invalid UTF-8 bytes (QA-43-r1-2); any left
// would come out as U+FFFD, so the result is always valid UTF-8.
func sanitiseName(s string) string {
	var b strings.Builder
	space := false
	n := 0
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			space = b.Len() > 0
			continue
		case unicode.In(r, unicode.Cc, unicode.Cf):
			continue
		}
		if n == maxModel {
			break
		}
		if space {
			if n+1 == maxModel {
				break // the cut falls on the space: trimmed again
			}
			b.WriteByte(' ')
			n++
			space = false
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// pool is F-44 PS-2: a session's pool values as recorded; zero or invalid
// is not recorded.
func pool(length uint16, unit typedef.DisplayMeasure, active uint16) workout.Pool {
	var p workout.Pool
	if length != basetype.Uint16Invalid {
		p.Length = float64(length) / 100
		p.Statute = unit == typedef.DisplayMeasureStatute
	}
	if active != basetype.Uint16Invalid {
		p.Lengths = active
	}
	return p
}
