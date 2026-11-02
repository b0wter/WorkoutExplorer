package ingest

import (
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/factory"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/fieldnum"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
)

// The two tables of the UP-9 strip (ADR-023 items 4 and 5). Everything the
// strip changes besides the frame is listed here and nowhere else.

// mesgNumLocation is the saved-location message (global number 29). It is
// not in the library's public profile, so it has no mesgnum constant.
const mesgNumLocation = 29

// blankTable lists, per global message number, the fields that are
// overwritten with their base type's invalid value (ADR-023 item 4): serial
// numbers, ANT+ device numbers and ANT IDs. ANT transmission types are
// deliberately absent (OQ-46).
var blankTable = map[uint16][]uint8{
	uint16(mesgnum.FileId):       {fieldnum.FileIdSerialNumber},
	uint16(mesgnum.DeviceInfo):   {fieldnum.DeviceInfoSerialNumber, fieldnum.DeviceInfoAntDeviceNumber},
	uint16(mesgnum.TrainingFile): {fieldnum.TrainingFileSerialNumber},
	uint16(mesgnum.Schedule):     {fieldnum.ScheduleSerialNumber},
	uint16(mesgnum.HrmProfile):   {fieldnum.HrmProfileHrmAntId},
	uint16(mesgnum.SdmProfile):   {fieldnum.SdmProfileSdmAntId},
	uint16(mesgnum.BikeProfile): {
		fieldnum.BikeProfileBikeSpdAntId, fieldnum.BikeProfileBikeCadAntId,
		fieldnum.BikeProfileBikeSpdcadAntId, fieldnum.BikeProfileBikePowerAntId,
	},
	// F-09 v1.4 (ADR-023 rev. 3, D-19; QA-09-r1 Q1): UP-9 (1) read literally.
	uint16(mesgnum.AntChannelId): {fieldnum.AntChannelIdDeviceNumber},
	// The segment messages' account keys (ADR-023 rev. 4 item 4).
	uint16(mesgnum.SegmentId): {fieldnum.SegmentIdDeviceId, fieldnum.SegmentIdUserProfilePrimaryKey},
	uint16(mesgnum.SegmentFile): {
		fieldnum.SegmentFileUserProfilePrimaryKey, fieldnum.SegmentFileLeaderGroupPrimaryKey,
		fieldnum.SegmentFileLeaderActivityId, fieldnum.SegmentFileLeaderActivityIdString,
	},
	// The tank pods' sensor is an ant_channel_id (uint32z) packing the pod's device number.
	uint16(mesgnum.TankUpdate):  {fieldnum.TankUpdateSensor},
	uint16(mesgnum.TankSummary): {fieldnum.TankSummarySensor},
}

// blankKey names one field of one global message.
type blankKey struct {
	mesg  uint16
	field uint8
}

// readTypes holds, per blankTable entry, the base type the decoder reads the
// field as (D-23, ADR-023 rev. 6 item 4): the profile's, from the factory the
// parser's decoder uses. sequence.go passes no WithFactory, so that is
// factory.StandardFactory(); if ingest ever passes another factory, the strip
// must use the same instance (change it here and in sequence.go together).
var readTypes = func() map[blankKey]basetype.BaseType {
	m := make(map[blankKey]basetype.BaseType)
	f := factory.StandardFactory()
	for mesg, fields := range blankTable {
		for _, n := range fields {
			m[blankKey{mesg, n}] = f.CreateField(typedef.MesgNum(mesg), n).BaseType
		}
	}
	return m
}()

// readType is the base type the decoder reads field of mesg as: the
// profile's for a blankTable field, else (a field the profile does not know,
// unreachable for the pinned table) the declared one, as the decoder does.
func readType(mesg uint16, field, declared uint8) basetype.BaseType {
	if t, ok := readTypes[blankKey{mesg, field}]; ok {
		return t
	}
	return basetype.BaseType(declared)
}

// removeTable lists the global message numbers whose data messages are
// removed whole (ADR-023 item 5): user_profile, saved locations, ant_rx,
// ant_tx and segment_leaderboard_entry. Their definition messages stay.
var removeTable = map[uint16]bool{
	uint16(mesgnum.UserProfile):             true,
	mesgNumLocation:                         true,
	uint16(mesgnum.AntRx):                   true,
	uint16(mesgnum.AntTx):                   true,
	uint16(mesgnum.SegmentLeaderboardEntry): true,
}

// keptIdentifier records an identifier-like field that is deliberately not
// blanked (ADR-023 rev. 4 item 4a). It is read only by TestStripTable.
type keptIdentifier struct {
	mesg   uint16
	field  uint8
	reason string
}

// keptIdentifiers is exactly ADR-023 rev. 4 item 4a's table.
var keptIdentifiers = []keptIdentifier{
	{uint16(mesgnum.BikeProfile), fieldnum.BikeProfileId, "the bike profile's slot number on the device"},
	{uint16(mesgnum.DiveAlarm), fieldnum.DiveAlarmId, "the alarm's number"},
	{uint16(mesgnum.DiveApneaAlarm), fieldnum.DiveApneaAlarmId, "the alarm's number"},
	{uint16(mesgnum.FieldDescription), fieldnum.FieldDescriptionFitBaseTypeId, "type code"},
	{uint16(mesgnum.FieldDescription), fieldnum.FieldDescriptionFitBaseUnitId, "unit code"},
	{uint16(mesgnum.DeveloperDataId), fieldnum.DeveloperDataIdDeveloperId, "developer data, kept by UP-9"},
	{uint16(mesgnum.DeveloperDataId), fieldnum.DeveloperDataIdApplicationId, "developer data, kept by UP-9"},
	{uint16(mesgnum.DeveloperDataId), fieldnum.DeveloperDataIdManufacturerId, "developer data, kept by UP-9"},
	{uint16(mesgnum.WeatherAlert), fieldnum.WeatherAlertReportId, "the weather service's report"},
	{uint16(mesgnum.CameraEvent), fieldnum.CameraEventCameraFileUuid, "a video file on the camera"},
	{uint16(mesgnum.SegmentId), fieldnum.SegmentIdUuid, "the segment (a place), not a device or person"},
	{uint16(mesgnum.SegmentLap), fieldnum.SegmentLapUuid, "the segment (a place), not a device or person"},
	{uint16(mesgnum.SegmentFile), fieldnum.SegmentFileFileUuid, "the segment (a place), not a device or person"},
	{uint16(mesgnum.HsaEvent), fieldnum.HsaEventEventId, "an event code"},
	{uint16(mesgnum.ExdDataFieldConfiguration), fieldnum.ExdDataFieldConfigurationFieldId, "a data screen field"},
	{uint16(mesgnum.ExdDataConceptConfiguration), fieldnum.ExdDataConceptConfigurationFieldId, "a data screen field"},
	{uint16(mesgnum.DeviceAuxBatteryInfo), fieldnum.DeviceAuxBatteryInfoBatteryIdentifier, "which battery of the device"},
}
