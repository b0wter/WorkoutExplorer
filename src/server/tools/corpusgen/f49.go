package main

import (
	"math/rand/v2"
	"time"

	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

// Zone files of F-49 (feature scope, Corpus), shaped like the real files
// checked on 13 Nov 2026. Every value is invented.

// zoneRide is a 20 minute ride (or run without power) with laps every 300 s.
func zoneRide(name string, day int, seed uint64, sport typedef.Sport) file {
	const end = 1199
	f := file{name: name, start: baseStart.Add(time.Duration(day) * 24 * time.Hour), localOffset: intp(3600), enhanced: true,
		sessions: []span{{sport, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 300 {
		f.laps = append(f.laps, span{from: from, to: min(from+300, end)})
	}
	f.samples = outdoorRide(seed, end, steady(rand.New(rand.NewPCG(seed, seed+1))), hills)
	if sport == typedef.SportRunning {
		for i := range f.samples {
			f.samples[i].power = nan
		}
	}
	return f
}

// appleRun: one session-referenced time_in_zone, % of max calc type with bpm
// values (50 … 90 % of the configured 190, plus the cap 190 with six values),
// and privacy markers that must reach no data file: resting heart rate 31,
// threshold heart rate 233, the device's time-in-zone totals and a
// session.max_heart_rate of 201 above the configured max (never a basis).
func appleRun(name string, day int, bounds []uint8) file {
	f := zoneRide(name, day, 91, typedef.SportRunning)
	f.session = func(m *mesgdef.Session) { m.SetMaxHeartRate(201) }
	f.extra = append(f.extra, timed{1199, orderSession + 1, mesgdef.NewTimeInZone(nil).SetTimestamp(at(f, 1199)).
		SetReferenceMesg(typedef.MesgNumSession).SetReferenceIndex(0).
		SetTimeInHrZone([]uint32{111_111, 222_222, 333_333, 444_444, 555_555, 666_666}).
		SetHrZoneHighBoundary(bounds).SetHrCalcType(typedef.HrZoneCalcPercentMaxHr).
		SetMaxHeartRate(190).SetRestingHeartRate(31).SetThresholdHeartRate(233).ToMesg(nil)})
	return f
}

func appleRun5() file {
	return appleRun("zones-apple-run-5.fit", 40, []uint8{95, 114, 133, 152, 171})
}

func appleRun6() file {
	return appleRun("zones-apple-run-6.fit", 41, []uint8{95, 114, 133, 152, 171, 190})
}

// wahooRide: hr_zone ×5 and power_zone ×7 (tops) at the start, and
// session.threshold_power 250; no max heart rate anywhere.
func wahooRide() file {
	f := zoneRide("zones-wahoo-ride.fit", 42, 93, typedef.SportCycling)
	f.device = &fileDevice{typedef.ManufacturerWahooFitness, 0, ""}
	f.session = func(m *mesgdef.Session) { m.SetThresholdPower(250) }
	for i, v := range []uint8{120, 140, 160, 175, 190} {
		f.preamble = append(f.preamble, mesgdef.NewHrZone(nil).SetMessageIndex(typedef.MessageIndex(i)).SetHighBpm(v).ToMesg(nil))
	}
	for i, v := range []uint16{138, 188, 225, 263, 300, 375, 2000} {
		f.preamble = append(f.preamble, mesgdef.NewPowerZone(nil).SetMessageIndex(typedef.MessageIndex(i)).SetHighValue(v).ToMesg(nil))
	}
	return f
}

// ftpOnlyRide: session.threshold_power 265 and nothing else about zones.
func ftpOnlyRide() file {
	f := zoneRide("zones-ride-ftp-only.fit", 43, 95, typedef.SportCycling)
	f.session = func(m *mesgdef.Session) { m.SetThresholdPower(265) }
	return f
}

// targetRide: zones_target with max heart rate 186 and threshold power 240
// (threshold heart rate 170 is never read).
func targetRide() file {
	f := zoneRide("zones-ride-target.fit", 44, 97, typedef.SportCycling)
	f.preamble = []proto.Message{mesgdef.NewZonesTarget(nil).SetMaxHeartRate(186).SetThresholdHeartRate(170).
		SetFunctionalThresholdPower(240).SetHrCalcType(typedef.HrZoneCalcPercentMaxHr).SetPwrCalcType(typedef.PwrZoneCalcPercentFtp).ToMesg(nil)}
	return f
}
