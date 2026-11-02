package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

// Files of F-08 (architect/F-08-fit-validation-and-recovery.md, Corpus):
// rejected, truncated, CRC-damaged, chained and multi-session files. All are
// generated; the broken ones are byte operations on a generated file, done
// inside generate so that TestSyntheticCorpusCurrent covers them. Coordinates
// are generated, as in every synthetic file.

// output is one file written by generate.
type output struct {
	name string
	data []byte
}

// ride30min is the base of the derived files: a clean 30 minute road ride,
// 1,800 records at 1 Hz with every stream, laps every 300 s, one session,
// timer start and stop, and an activity message (+2 h).
func ride30min() file {
	const end = 1799
	f := file{name: "ride-30min.fit", start: baseStart.Add(17 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 300 {
		f.laps = append(f.laps, span{from: from, to: min(from+300, end)})
	}
	f.samples = outdoorRide(37, end, steady(rand.New(rand.NewPCG(37, 38))), hills)
	return f
}

// run30min is the base of the run cut (v1.1): a clean 30 minute road run,
// 1,800 records at 1 Hz with positions, distance, speed about 3.3 m/s,
// altitude, heart rate, cadence about 171 steps per minute after R-5,
// temperature and no power. Laps every 300 s, one session, timer start and
// stop, activity (+2 h), and sport hints.
func run30min() file {
	const end = 1799
	f := file{name: "run-30min.fit", start: baseStart.Add(24 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, sportHints: true,
		sessions: []span{{typedef.SportRunning, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 300 {
		f.laps = append(f.laps, span{from: from, to: min(from+300, end)})
	}
	rng := rand.New(rand.NewPCG(51, 52))
	f.samples = outdoorRide(51, end, func(int) float64 { return 3.3 + rng.Float64()*0.4 - 0.2 }, hills)
	for i := range f.samples {
		f.samples[i].cad = 85 + 64.0/128 + float64(rng.IntN(5)-2) // one leg; about 171 steps per minute after R-5
		f.samples[i].power = nan
	}
	return f
}

// cutDerived returns the cut files of the v1.1 bases, found by name in the
// outputs already generated: run-30min, triathlon and chained-ride-ride.
func cutDerived(outputs []output) ([]output, error) {
	find := func(name string) ([]byte, error) {
		for _, o := range outputs {
			if o.name == name {
				return o.data, nil
			}
		}
		return nil, fmt.Errorf("%s: base file not generated", name)
	}
	var out []output
	for _, c := range []struct {
		base, name string
		num, den   int
	}{
		{"run-30min.fit", "run-30min-cut-60.fit", 6, 10},
		{"triathlon.fit", "triathlon-cut-90.fit", 9, 10},
		{"chained-ride-ride.fit", "chained-ride-ride-cut-75.fit", 3, 4},
	} {
		b, err := find(c.base)
		if err != nil {
			return nil, err
		}
		out = append(out, output{c.name, append([]byte(nil), b[:len(b)*c.num/c.den]...)})
	}
	return out, nil
}

// derived returns the files built by byte operations on the base ride, plus
// the chained file that appends a settings sequence to it.
func derived(base []byte) ([]output, error) {
	if len(base) < 14+2 || base[0] != 14 {
		return nil, fmt.Errorf("ride-30min.fit: want a 14 byte header")
	}
	clone := func() []byte { return append([]byte(nil), base...) }

	cut := clone()[:len(base)*6/10]

	unclosed := append([]byte(nil), cut...)
	binary.LittleEndian.PutUint32(unclosed[4:8], 0) // data size
	unclosed[12], unclosed[13] = 0, 0               // header CRC: not computed

	badCRC := clone()
	badCRC[len(badCRC)-2] ^= 0xFF
	badCRC[len(badCRC)-1] ^= 0xFF

	badHeader := clone()
	badHeader[12] ^= 0xFF
	badHeader[13] ^= 0xFF
	if badHeader[12] == 0 && badHeader[13] == 0 {
		return nil, fmt.Errorf("ride-30min-bad-header-crc.fit: the damaged header CRC is 0, which means \"not computed\"")
	}

	settings, err := encodeMessages([]proto.Message{
		mesgdef.NewFileId(nil).SetType(typedef.FileSettings).SetManufacturer(typedef.ManufacturerDevelopment).
			SetProduct(1).SetProductName("corpusgen").ToMesg(nil),
		mesgdef.NewUserProfile(nil).SetFriendlyName("corpusgen").ToMesg(nil),
	})
	if err != nil {
		return nil, err
	}

	badBase, _, err := badBaseType(base)
	if err != nil {
		return nil, fmt.Errorf("ride-30min-bad-basetype.fit: %w", err)
	}

	sizeShortFile, _, _, _, err := sizeShort(base)
	if err != nil {
		return nil, fmt.Errorf("ride-30min-size-short.fit: %w", err)
	}

	return []output{
		{"ride-30min-size-short.fit", sizeShortFile},
		{"ride-30min-bad-basetype.fit", badBase},
		{"ride-30min-cut-60.fit", cut},
		{"ride-30min-cut-60-size-0.fit", unclosed},
		{"ride-30min-bad-crc.fit", badCRC},
		{"ride-30min-bad-header-crc.fit", badHeader},
		{"ride-30min-no-crc.fit", clone()[:len(base)-2]},
		{"ride-30min-trailing-zeros.fit", append(clone(), make([]byte, 64)...)},
		{"ride-30min-header-only.fit", clone()[:14]},
		{"chained-with-settings.fit", append(clone(), settings...)},
	}, nil
}

// standalone returns the files that are not derived from another one.
func standalone() ([]output, error) {
	var out []output
	add := func(name string, data []byte, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, output{name, data})
		return nil
	}

	noRec := file{name: "no-records.fit", start: baseStart.Add(18 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 600}},
		laps:     []span{{from: 0, to: 600}},
		timer:    []timerEvent{{0, true}, {600, false}}}
	b, err := encode(noRec)
	if err := add(noRec.name, b, err); err != nil {
		return nil, err
	}

	png, _ := hex.DecodeString(pngHex)
	out = append(out, output{"not-fit-png.fit", png})

	rng := rand.New(rand.NewPCG(41, 42))
	random := make([]byte, 4096)
	for i := range random {
		random[i] = byte(rng.Uint32())
	}
	random[0] = 0x00
	out = append(out, output{"not-fit-random.fit", random}, output{"empty.fit", []byte{}})

	b, err = courseFile()
	if err := add("course.fit", b, err); err != nil {
		return nil, err
	}
	b, err = monitoringFile()
	if err := add("monitoring.fit", b, err); err != nil {
		return nil, err
	}
	b, err = workoutPlanFile()
	if err := add("workout-plan.fit", b, err); err != nil {
		return nil, err
	}
	b, err = chainedRideRun()
	if err := add("chained-ride-run.fit", b, err); err != nil {
		return nil, err
	}
	b, err = encode(triathlon())
	if err := add("triathlon.fit", b, err); err != nil {
		return nil, err
	}
	b, err = chainedRideRide()
	if err := add("chained-ride-ride.fit", b, err); err != nil {
		return nil, err
	}
	b, err = encode(rideTransitionRide())
	if err := add("ride-transition-ride.fit", b, err); err != nil {
		return nil, err
	}
	b, err = encode(identifyingData())
	if err := add("identifying-data.fit", b, err); err != nil {
		return nil, err
	}
	b, err = encode(swimBikeDistanceFall())
	if err := add("swim-bike-distance-fall.fit", b, err); err != nil {
		return nil, err
	}
	return out, nil
}

// pngHex is a valid 1x1 PNG, fixed so that the file does not depend on the
// Go version's image/png output.
const pngHex = "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d4944415478da63f8cfc0f01f00050001ff56c72f0d0000000049454e44ae426082"

// encodeMessages encodes messages into one FIT sequence.
func encodeMessages(msgs []proto.Message) ([]byte, error) {
	fit := proto.FIT{FileHeader: proto.FileHeader{ProtocolVersion: proto.V2}, Messages: msgs}
	var buf bytes.Buffer
	if err := encoder.New(&buf).Encode(&fit); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fileID(t typedef.File) proto.Message {
	return mesgdef.NewFileId(nil).SetType(t).SetManufacturer(typedef.ManufacturerDevelopment).
		SetProduct(1).SetProductName("corpusgen").SetTimeCreated(baseStart.Add(19 * 24 * time.Hour)).ToMesg(nil)
}

// courseFile: file_id type course, a course message, one lap, 300 records
// with positions and timestamps, 3 course points.
func courseFile() ([]byte, error) {
	start := baseStart.Add(19 * 24 * time.Hour)
	msgs := []proto.Message{
		fileID(typedef.FileCourse),
		mesgdef.NewCourse(nil).SetSport(typedef.SportCycling).SetName("corpus course").ToMesg(nil),
		mesgdef.NewLap(nil).SetTimestamp(start.Add(299 * time.Second)).SetStartTime(start).
			SetTotalElapsedTime(299_000).SetTotalDistance(150_000).ToMesg(nil),
	}
	r := newRoute()
	for t := 0; t < 300; t++ {
		if t > 0 {
			r.step(t, 5, 1)
		}
		msgs = append(msgs, mesgdef.NewRecord(nil).SetTimestamp(start.Add(time.Duration(t)*time.Second)).
			SetPositionLat(semicircles(r.lat)).SetPositionLong(semicircles(r.lon)).
			SetDistance(uint32(r.dist*100)).ToMesg(nil))
		if t%100 == 99 {
			msgs = append(msgs, mesgdef.NewCoursePoint(nil).SetTimestamp(start.Add(time.Duration(t)*time.Second)).
				SetPositionLat(semicircles(r.lat)).SetPositionLong(semicircles(r.lon)).
				SetType(typedef.CoursePointGeneric).SetName("point").ToMesg(nil))
		}
	}
	return encodeMessages(msgs)
}

// monitoringFile: file_id type monitoring_b, 96 monitoring messages at
// 15 minute steps with a heart rate.
func monitoringFile() ([]byte, error) {
	start := baseStart.Add(20 * 24 * time.Hour)
	msgs := []proto.Message{fileID(typedef.FileMonitoringB)}
	for i := 0; i < 96; i++ {
		msgs = append(msgs, mesgdef.NewMonitoring(nil).SetTimestamp(start.Add(time.Duration(i)*15*time.Minute)).
			SetHeartRate(uint8(55+i%20)).ToMesg(nil))
	}
	return encodeMessages(msgs)
}

// workoutPlanFile: file_id type workout, a workout message and 3 steps.
func workoutPlanFile() ([]byte, error) {
	msgs := []proto.Message{
		fileID(typedef.FileWorkout),
		mesgdef.NewWorkout(nil).SetSport(typedef.SportRunning).SetNumValidSteps(3).SetWktName("corpus plan").ToMesg(nil),
	}
	for i := 0; i < 3; i++ {
		msgs = append(msgs, mesgdef.NewWorkoutStep(nil).SetMessageIndex(typedef.MessageIndex(i)).
			SetDurationType(typedef.WktStepDurationTime).SetDurationValue(300_000).ToMesg(nil))
	}
	return encodeMessages(msgs)
}

// chainedRideRun: two complete activity files concatenated: a 20 minute ride
// (1,200 records), then, starting 15 minutes after its last record, a
// 15 minute run (900 records). Each has its own file_id, laps, session and
// activity.
func chainedRideRun() ([]byte, error) {
	const rideEnd, runEnd, pause = 1199, 899, 900
	rideStart := baseStart.Add(21 * 24 * time.Hour)
	ride := file{name: "ride", start: rideStart, localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, rideEnd}},
		laps:     []span{{from: 0, to: 600}, {from: 600, to: rideEnd}},
		timer:    []timerEvent{{0, true}, {rideEnd, false}}}
	ride.samples = outdoorRide(43, rideEnd, steady(rand.New(rand.NewPCG(43, 44))), hills)

	run := file{name: "run", start: rideStart.Add((rideEnd + pause) * time.Second), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportRunning, typedef.SubSportRoad, 0, runEnd}},
		laps:     []span{{from: 0, to: runEnd}},
		timer:    []timerEvent{{0, true}, {runEnd, false}}}
	rng := rand.New(rand.NewPCG(45, 46))
	for _, s := range outdoorRide(45, runEnd, func(int) float64 { return 3.3 + rng.Float64()*0.4 - 0.2 }, hills) {
		s.cad = 85 + 64.0/128 // one leg; 171 steps per minute after R-5
		s.power = nan
		f := s
		run.samples = append(run.samples, f)
	}

	a, err := encode(ride)
	if err != nil {
		return nil, err
	}
	b, err := encode(run)
	if err != nil {
		return nil, err
	}
	return append(a, b...), nil
}

// chainedRideRide (v1.1): two complete cycling activity files concatenated,
// 600 records each at 1 Hz and about 5 m/s with every stream; the second
// starts 300 s after the first one's last record and its device distance
// starts again at 0. Each has its own file_id, laps every 300 s, session,
// timer start and stop, activity, and sport hints.
func chainedRideRide() ([]byte, error) {
	const end, pause = 599, 300
	first := baseStart.Add(25 * 24 * time.Hour)
	var out []byte
	for i, start := range []time.Time{first, first.Add((end + pause) * time.Second)} {
		f := file{name: "ride", start: start, localOffset: intp(7200), enhanced: true, sportHints: true,
			sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
			laps:     []span{{from: 0, to: 300}, {from: 300, to: end}},
			timer:    []timerEvent{{0, true}, {end, false}}}
		rng := rand.New(rand.NewPCG(uint64(53+2*i), uint64(54+2*i)))
		f.samples = outdoorRide(uint64(53+2*i), end, func(int) float64 { return 5 + rng.Float64()*0.4 - 0.2 }, hills)
		b, err := encode(f)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

// triathlon: one sequence with 5 sessions in order: open water swim 600 s,
// transition 60 s, road ride 1,800 s, transition 60 s, run 1,200 s; laps per
// session; timer start and stop. Records at 1 Hz throughout.
func triathlon() file {
	const swimEnd, t1End, bikeEnd, t2End, runEnd = 599, 659, 2459, 2519, 3719
	f := file{name: "triathlon.fit", start: baseStart.Add(22 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, sportHints: true,
		sessions: []span{
			{typedef.SportSwimming, typedef.SubSportOpenWater, 0, swimEnd},
			{typedef.SportTransition, typedef.SubSportGeneric, swimEnd + 1, t1End},
			{typedef.SportCycling, typedef.SubSportRoad, t1End + 1, bikeEnd},
			{typedef.SportTransition, typedef.SubSportGeneric, bikeEnd + 1, t2End},
			{typedef.SportRunning, typedef.SubSportRoad, t2End + 1, runEnd},
		},
		timer: []timerEvent{{0, true}, {runEnd, false}}}
	for _, s := range f.sessions {
		f.laps = append(f.laps, span{from: s.from, to: s.to})
	}
	rng := rand.New(rand.NewPCG(47, 48))
	r := newRoute()
	for t := 0; t <= runEnd; t++ {
		var v float64
		switch {
		case t <= swimEnd:
			v = 1.1 + rng.Float64()*0.2
		case t <= t1End || (t > bikeEnd && t <= t2End):
			v = 1.5
		case t <= bikeEnd:
			v = 7.5 + rng.Float64() - 0.5
		default:
			v = 3.3 + rng.Float64()*0.4 - 0.2
		}
		if t > 0 {
			r.step(t, v, 1)
		}
		s := newSample(t)
		s.lat, s.lon, s.dist, s.speed = r.lat, r.lon, r.dist, v
		switch {
		case t <= swimEnd:
			s.hr = round1(140 + rng.Float64()*6)
			s.cad = round1(30 + rng.Float64()*4) // strokes per minute
			s.temp = 17
		case t <= t1End || (t > bikeEnd && t <= t2End):
			s.hr = round1(130 + rng.Float64()*6)
		case t <= bikeEnd:
			s.alt = hills(t)
			s.hr = round1(150 + rng.Float64()*6)
			s.cad = round1(88 + rng.Float64()*4)
			s.power = round1(210 + rng.Float64()*30)
			s.temp = 20
		default:
			s.alt = hills(t)
			s.hr = round1(160 + rng.Float64()*6)
			s.cad = 85 + 64.0/128 // 171 steps per minute after R-5
			s.temp = 21
		}
		f.samples = append(f.samples, s)
	}
	return f
}

// rideTransitionRide (F-08 v1.3, R-13): one sequence with 3 sessions, a
// 600 s road ride, a 60 s transition and another 600 s road ride; one lap per
// session; no timer events (moving time then comes from speed, so R-13.M is
// what counts the transition); sport hints; records at 1 Hz throughout. The
// transition records carry every stream, with speed, cadence and power
// constants above the rides' maxima (speed 12 m/s at offset 600, cadence 100,
// power 400 W) and a temperature of 30, so a value that escapes the mask
// shows in a maximum or an average, and the temperature shows that it is kept.
func rideTransitionRide() file {
	const ride1End, trEnd, ride2End = 599, 659, 1259
	f := file{name: "ride-transition-ride.fit", start: baseStart.Add(25 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, sportHints: true,
		sessions: []span{
			{typedef.SportCycling, typedef.SubSportRoad, 0, ride1End},
			{typedef.SportTransition, typedef.SubSportGeneric, ride1End + 1, trEnd},
			{typedef.SportCycling, typedef.SubSportRoad, trEnd + 1, ride2End},
		}}
	for _, s := range f.sessions {
		f.laps = append(f.laps, span{from: s.from, to: s.to})
	}
	rng := rand.New(rand.NewPCG(61, 62))
	r := newRoute()
	for t := 0; t <= ride2End; t++ {
		inTransition := t > ride1End && t <= trEnd
		v := 7.5 + rng.Float64() - 0.5
		routeSpeed := v
		if inTransition {
			v, routeSpeed = 1.5, 1.5 // the route advances at 1.5 m/s also at the 12 m/s sample
			if t == ride1End+1 {
				v = 12
			}
		}
		if t > 0 {
			r.step(t, routeSpeed, 1)
		}
		s := newSample(t)
		s.lat, s.lon, s.dist, s.speed, s.alt = r.lat, r.lon, r.dist, v, hills(t)
		if inTransition {
			s.hr = round1(130 + rng.Float64()*6)
			s.cad, s.power, s.temp = 100, 400, 30
		} else {
			s.hr = round1(150 + rng.Float64()*6)
			s.cad = round1(88 + rng.Float64()*4)
			s.power = round1(210 + rng.Float64()*30)
			s.temp = 20
		}
		f.samples = append(f.samples, s)
	}
	return f
}

// swimBikeDistanceFall (F-08 v1.5, R-14, feature AC 10): one sequence, a
// 600 s open-water swim and a 600 s road ride, one lap per session, timer
// start and stop, no sport hints. The device distance is written explicitly
// and falls twice: the swim writes 0.5 m at offsets 200-219 (a near-zero
// dropout run), and the ride starts 49 m lower than the swim ended (599 to
// 550). The session totals are the swim's 599 and the ride's 2,946, which sum
// to the last record's 3,545.
func swimBikeDistanceFall() file {
	const swimEnd, bikeEnd = 599, 1199
	f := file{name: "swim-bike-distance-fall.fit", start: baseStart.Add(26 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{
			{typedef.SportSwimming, typedef.SubSportOpenWater, 0, swimEnd},
			{typedef.SportCycling, typedef.SubSportRoad, swimEnd + 1, bikeEnd},
		},
		timer:       []timerEvent{{0, true}, {bikeEnd, false}},
		sessionDist: map[int]float64{0: 599, swimEnd + 1: 2946}}
	for _, s := range f.sessions {
		f.laps = append(f.laps, span{from: s.from, to: s.to})
	}
	rng := rand.New(rand.NewPCG(71, 72))
	r := newRoute()
	for t := 0; t <= bikeEnd; t++ {
		v := 1.0
		if t > swimEnd {
			v = 5.0
		}
		if t > 0 {
			r.step(t, v, 1)
		}
		s := newSample(t)
		s.lat, s.lon, s.speed = r.lat, r.lon, v
		switch {
		case t >= 200 && t <= 219:
			s.dist = 0.5
		case t <= swimEnd:
			s.dist = float64(t)
		default:
			s.dist = 550 + 5*float64(t-600)
		}
		if t <= swimEnd {
			s.hr = round1(140 + rng.Float64()*6)
		} else {
			s.hr = round1(150 + rng.Float64()*6)
		}
		f.samples = append(f.samples, s)
	}
	return f
}

// capFile is the all-caps file for the stop-rule measurement of ADR-017
// rev. 1 item 7: records at 1 Hz with every stream, 100 sessions alternating
// cycling and running, 10,000 laps with lap.sport, 100,000 timer events,
// 1,000 sport messages and 256 field descriptions. With 500,000 records it is
// about 16 MB, inside MaxBytes. It is written only on request (-cap-file)
// and never committed. The in-memory twin is ingest's BenchmarkParseAtCap.
func capFile(records int) file {
	const sessions, laps, sportMessages, fieldDescriptions = 100, 10_000, 1_000, 256
	end := records - 1
	f := file{name: "cap-500k.fit", start: baseStart.Add(23 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sportHints: true, developer: true}
	sportOf := func(i int) typedef.Sport {
		if i%2 == 0 {
			return typedef.SportCycling
		}
		return typedef.SportRunning
	}
	for i := 0; i < sessions; i++ {
		f.sessions = append(f.sessions, span{sportOf(i), typedef.SubSportGeneric, i * records / sessions, (i+1)*records/sessions - 1})
	}
	for i := 0; i < laps; i++ {
		f.laps = append(f.laps, span{from: i * records / laps, to: (i+1)*records/laps - 1})
	}
	// 100,000 timer events in 100 bursts of 1,000: the library's encoder has
	// one local message type and writes a message definition at every switch
	// of message kind, so events spread over the file would add about 7 MB.
	for i := 0; i < 100_000; i++ {
		f.timer = append(f.timer, timerEvent{i / 1000 * records / 100, i%2 == 0})
	}
	// The sessions bring one sport message each; the rest sit between them.
	for i := 0; i < sportMessages-sessions; i++ {
		t := (i*records/(sportMessages-sessions) + 7) % records
		f.extra = append(f.extra, timed{t: t, order: orderSport,
			mesg: mesgdef.NewSport(nil).SetSport(sportOf(i)).SetSubSport(typedef.SubSportGeneric).ToMesg(nil)})
	}
	for i := 1; i < fieldDescriptions; i++ { // the first comes from developer: true
		f.extra = append(f.extra, timed{t: 0, order: -1,
			mesg: mesgdef.NewFieldDescription(nil).SetDeveloperDataIndex(0).SetFieldDefinitionNumber(uint8(i)).
				SetFitBaseTypeId(basetype.Uint8).SetFieldName([]string{"cap_field"}).ToMesg(nil)})
	}
	rng := rand.New(rand.NewPCG(49, 50))
	f.samples = outdoorRide(49, end, func(t int) float64 { return 7 + rng.Float64() - 0.5 }, hills)
	return f
}
