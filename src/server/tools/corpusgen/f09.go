package main

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/muktihari/fit/kit/hash/crc16"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

// identifyingData is synthetic/identifying-data.fit (F-09 design, Corpus):
// every value that UP-9 takes out of a stored file is present as an invented
// marker, and so are the messages that must stay (OQ-50: zones_target and
// time_in_zone with heart-rate and power settings; sport name; developer
// field). Nothing here comes from a real file.
func identifyingData() file {
	const end = 599
	start := baseStart.Add(27 * 24 * time.Hour)
	ts := func(t int) time.Time { return start.Add(time.Duration(t) * time.Second) }

	pre := []proto.Message{
		// The device that wrote the file and two sensors.
		mesgdef.NewDeviceInfo(nil).SetTimestamp(ts(0)).SetDeviceIndex(0).SetManufacturer(typedef.ManufacturerDevelopment).
			SetProduct(1).SetSerialNumber(2_882_400_001).ToMesg(nil),
		// F-43: recognised sensors whose identifiers, descriptors and product
		// names must never reach a page or data file.
		mesgdef.NewDeviceInfo(nil).SetTimestamp(ts(0)).SetDeviceIndex(1).SetSerialNumber(2_882_400_002).
			SetAntDeviceNumber(54_321).SetAntTransmissionType(1).
			SetSourceType(typedef.SourceTypeAntplus).SetDeviceType(uint8(typedef.AntplusDeviceTypeHeartRate)).
			SetDescriptor("Marker Descriptor Chest").SetProductName("Marker Sensor Product HR").ToMesg(nil),
		mesgdef.NewDeviceInfo(nil).SetTimestamp(ts(0)).SetDeviceIndex(2).SetSerialNumber(2_882_400_003).
			SetAntDeviceNumber(12_345).SetAntTransmissionType(5).
			SetSourceType(typedef.SourceTypeAntplus).SetDeviceType(uint8(typedef.AntplusDeviceTypeBikePower)).
			SetDescriptor("Marker Descriptor Left Crank").SetProductName("Marker Sensor Product PM").ToMesg(nil),
		mesgdef.NewUserProfile(nil).SetFriendlyName("Marker Person").SetGender(typedef.GenderFemale).SetAge(41).
			SetHeightScaled(1.71).SetWeightScaled(63.5).SetRestingHeartRate(47).SetDefaultMaxHeartRate(187).
			SetGlobalId([6]byte{'M', 'k', 'r', 'G', 'i', 'd'}).ToMesg(nil),
		mesgdef.NewHrmProfile(nil).SetHrmAntId(54_321).SetHrmAntIdTransType(1).ToMesg(nil),
		// Message 29, a saved location: not in the library's profile, so raw fields.
		{Num: typedef.MesgNum(29), Fields: []proto.Field{
			rawField(253, basetype.Uint32, proto.Uint32(fitTime(ts(-60)))),
			rawField(0, basetype.String, proto.String("Marker Home Place")),
			rawField(1, basetype.Sint32, proto.Int32(semicircles(47.05))),
			rawField(2, basetype.Sint32, proto.Int32(semicircles(8.3))),
		}},
		mesgdef.NewSegmentLeaderboardEntry(nil).SetName("Marker Rival Person").SetActivityId(424_242).ToMesg(nil),
		// F-09 v1.4 (QA-09-r1 Q1, OQ-52): the ANT and segment identifiers UP-9 takes out.
		mesgdef.NewAntChannelId(nil).SetChannelNumber(0).SetDeviceType(120).SetDeviceNumber(23_456).
			SetTransmissionType(1).SetDeviceIndex(1).ToMesg(nil),
		mesgdef.NewAntRx(nil).SetTimestamp(ts(0)).SetMesgId(0x4E).
			SetMesgData([]byte{0, 'M', 'k', 'r', 'A', 'n', 't', 'R', 'x'}).ToMesg(nil),
		mesgdef.NewAntTx(nil).SetTimestamp(ts(0)).SetMesgId(0x4F).
			SetMesgData([]byte{0, 'M', 'k', 'r', 'A', 'n', 't', 'T', 'x'}).ToMesg(nil),
		mesgdef.NewSegmentId(nil).SetName("Marker Segment").SetUuid("marker-segment-uuid").SetSport(typedef.SportCycling).
			SetUserProfilePrimaryKey(2_882_400_005).SetDeviceId(2_882_400_004).ToMesg(nil),
		mesgdef.NewSegmentFile(nil).SetFileUuid("marker-segment-uuid").SetUserProfilePrimaryKey(2_882_400_010).
			SetLeaderType([]typedef.SegmentLeaderboardType{typedef.SegmentLeaderboardTypeOverall, typedef.SegmentLeaderboardTypePersonalBest}).
			SetLeaderGroupPrimaryKey([]uint32{2_882_400_006, 2_882_400_007}).
			SetLeaderActivityId([]uint32{2_882_400_008, 2_882_400_009}).
			SetLeaderActivityIdString([]string{"MarkerLeaderOne", "MarkerLeaderTwo"}).ToMesg(nil),
		mesgdef.NewTankUpdate(nil).SetTimestamp(ts(60)).SetSensor(typedef.AntChannelId(2_882_400_011)).SetPressureScaled(200).ToMesg(nil),
		mesgdef.NewTankSummary(nil).SetTimestamp(ts(end)).SetSensor(typedef.AntChannelId(2_882_400_011)).
			SetStartPressureScaled(200).SetEndPressureScaled(50).SetVolumeUsedScaled(10).ToMesg(nil),
		mesgdef.NewSport(nil).SetSport(typedef.SportCycling).SetSubSport(typedef.SubSportRoad).SetName("Marker Sport Name").ToMesg(nil),
		// Zone messages stay as uploaded (OQ-50).
		mesgdef.NewZonesTarget(nil).SetMaxHeartRate(187).SetThresholdHeartRate(168).SetFunctionalThresholdPower(251).
			SetHrCalcType(typedef.HrZoneCalcPercentMaxHr).SetPwrCalcType(typedef.PwrZoneCalcPercentFtp).ToMesg(nil),
		mesgdef.NewTimeInZone(nil).SetTimestamp(ts(end)).SetReferenceMesg(typedef.MesgNumSession).SetReferenceIndex(0).
			SetTimeInHrZone([]uint32{60_000, 120_000, 240_000, 120_000, 60_000}).
			SetHrZoneHighBoundary([]uint8{112, 131, 150, 168, 187}).
			SetPowerZoneHighBoundary([]uint16{140, 190, 226, 264, 302}).
			SetTimeInPowerZone([]uint32{90_000, 150_000, 200_000, 100_000, 60_000}).
			SetHrCalcType(typedef.HrZoneCalcPercentMaxHr).SetMaxHeartRate(187).SetRestingHeartRate(47).
			SetThresholdHeartRate(168).SetPwrCalcType(typedef.PwrZoneCalcPercentFtp).SetFunctionalThresholdPower(251).ToMesg(nil),
	}

	f := file{name: "identifying-data.fit", start: start, localOffset: intp(7200), enhanced: true, developer: true,
		fileSerial: 3_735_928_559, devName: "marker_dev_field", preamble: pre,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		laps:     []span{{from: 0, to: end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	f.samples = outdoorRide(81, end, steady(rand.New(rand.NewPCG(81, 82))), hills)
	for i := range f.samples {
		f.samples[i].developerField = true
	}
	return f
}

// badBaseType makes ride-30min-bad-basetype.fit from the base ride (F-09
// design v1.5, section Corpus): one 9-byte definition message with an invalid
// base type (0x30) is inserted before the first record data message at or
// after 60 % of the record messages. The local type L it uses is one that no
// message after the insert point uses, so the old framing could still read
// everything behind it; L is returned for the hand-off. Data size, header CRC
// and file CRC are computed afresh.
func badBaseType(base []byte) (file []byte, local byte, err error) {
	if len(base) < 16 || base[0] != 14 {
		return nil, 0, fmt.Errorf("ride-30min.fit: want a 14 byte header")
	}
	end := 14 + int(binary.LittleEndian.Uint32(base[4:8]))
	type rec struct {
		at     int
		local  byte
		global uint16
		isDef  bool
	}
	var recs []rec
	var size [16]int
	var glob [16]uint16
	for at := 14; at < end; {
		h := base[at]
		switch {
		case h&0x80 != 0:
			return nil, 0, fmt.Errorf("compressed timestamp header at byte %d", at)
		case h&0x40 != 0:
			nf := int(base[at+5])
			n := 6 + 3*nf
			sz := 0
			for i := 0; i < nf; i++ {
				sz += int(base[at+6+3*i+1])
			}
			if h&0x20 != 0 {
				nd := int(base[at+n])
				for i := 0; i < nd; i++ {
					sz += int(base[at+n+1+3*i+1])
				}
				n += 1 + 3*nd
			}
			l := h & 0x0F
			size[l], glob[l] = sz, binary.LittleEndian.Uint16(base[at+3:])
			recs = append(recs, rec{at, l, glob[l], true})
			at += n
		default:
			l := h & 0x0F
			recs = append(recs, rec{at, l, glob[l], false})
			at += 1 + size[l]
		}
	}
	nrec := 0
	for _, r := range recs {
		if !r.isDef && r.global == 20 {
			nrec++
		}
	}
	target, seen, ins := nrec*6/10, 0, -1
	for i, r := range recs {
		if !r.isDef && r.global == 20 {
			if seen == target {
				ins = i
				break
			}
			seen++
		}
	}
	if ins < 0 {
		return nil, 0, fmt.Errorf("no insert point")
	}
	var used [16]bool
	for _, r := range recs[ins:] {
		used[r.local] = true
	}
	local = 255
	for l := 15; l >= 0; l-- {
		if !used[l] {
			local = byte(l)
			break
		}
	}
	if local == 255 {
		return nil, 0, fmt.Errorf("no free local type after the insert point")
	}
	at := recs[ins].at
	def := []byte{0x40 | local, 0, 0, 0x00, 0xFF, 1, 0, 1, 0x30}
	out := append([]byte(nil), base[:at]...)
	out = append(out, def...)
	out = append(out, base[at:end]...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-14))
	c := crc16.New()
	_, _ = c.Write(out[:12])
	binary.LittleEndian.PutUint16(out[12:14], c.Sum16())
	c = crc16.New()
	_, _ = c.Write(out)
	return binary.LittleEndian.AppendUint16(out, c.Sum16()), local, nil
}

// sizeShort makes ride-30min-size-short.fit from the base ride (F-09 design
// v1.6, section Corpus): the header data size is lowered by k (5, or the
// smallest larger value that lands inside the last record) and the header CRC
// is computed afresh. Every other byte, the file CRC included, is unchanged.
// It returns k, the last record's length and the header size for the
// hand-off. The base has a 14 byte header, so the header CRC does not depend
// on the data size of the stored file and the decoder reads the unchanged file
// CRC after the last record.
func sizeShort(base []byte) (file []byte, k, lastLen, headerSize int, err error) {
	if len(base) < 16 || base[0] != 14 {
		return nil, 0, 0, 0, fmt.Errorf("ride-30min.fit: want a 14 byte header")
	}
	end := 14 + int(binary.LittleEndian.Uint32(base[4:8]))
	var size [16]int
	last := -1
	for at := 14; at < end; {
		last = at
		h := base[at]
		switch {
		case h&0x80 != 0:
			return nil, 0, 0, 0, fmt.Errorf("compressed timestamp header at byte %d", at)
		case h&0x40 != 0:
			nf := int(base[at+5])
			n, sz := 6+3*nf, 0
			for i := 0; i < nf; i++ {
				sz += int(base[at+6+3*i+1])
			}
			if h&0x20 != 0 {
				nd := int(base[at+n])
				for i := 0; i < nd; i++ {
					sz += int(base[at+n+1+3*i+1])
				}
				n += 1 + 3*nd
			}
			size[h&0x0F] = sz
			at += n
		default:
			at += 1 + size[h&0x0F]
		}
	}
	lastLen = end - last
	// The smallest k >= 5 that lands inside the last record: end-k must be
	// after the record's first byte and before its end.
	k = 5
	if k >= lastLen {
		return nil, 0, 0, 0, fmt.Errorf("the last record is %d bytes; a data size lowered by %d does not end inside it", lastLen, k)
	}
	out := append([]byte(nil), base...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(end-14-k))
	c := crc16.New()
	_, _ = c.Write(out[:12])
	binary.LittleEndian.PutUint16(out[12:14], c.Sum16())
	return out, k, lastLen, 14, nil
}
