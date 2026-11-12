// Command corpusgen writes the synthetic FIT files of the parser corpus
// (ADR-004, F-02 design section Corpus). It is a development tool, not part
// of the workout-explorer binary.
//
//	go run ./tools/corpusgen -out testdata/corpus/synthetic
//
// Every file is built from fixed parameters and fixed random seeds, so
// re-running produces identical bytes. The coordinates are synthetic.
package main

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/kit/datetime"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

func main() {
	out := flag.String("out", "testdata/corpus/synthetic", "output directory")
	capPath := flag.String("cap-file", "", "write the all-caps file for the ADR-017 rev. 1 stop-rule measurement to this `path` and nothing else (never committed)")
	flag.Parse()
	if *capPath != "" {
		if err := writeCapFile(*capPath); err != nil {
			fmt.Fprintf(os.Stderr, "corpusgen: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := generate(*out); err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: %v\n", err)
		os.Exit(1)
	}
}

// generate writes every synthetic corpus file into outDir.
func generate(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	var outputs []output
	var base []byte
	for _, f := range corpus() {
		b, err := encode(f)
		if err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
		outputs = append(outputs, output{f.name, b})
		if f.name == "ride-30min.fit" {
			base = b
		}
	}
	more, err := derived(base)
	if err != nil {
		return err
	}
	outputs = append(outputs, more...)
	more, err = standalone()
	if err != nil {
		return err
	}
	outputs = append(outputs, more...)
	more, err = cutDerived(outputs)
	if err != nil {
		return err
	}
	outputs = append(outputs, more...)
	for _, o := range outputs {
		if err := os.WriteFile(filepath.Join(outDir, o.name), o.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeCapFile writes the memory measurement file (ADR-017 item 7).
func writeCapFile(path string) error {
	b, err := encode(capFile(500_000))
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// nan marks a value that is not recorded.
var nan = math.NaN()

// sample is one record at offset t seconds. NaN values are not written.
type sample struct {
	t                            int
	lat, lon                     float64
	alt, dist, speed             float64
	hr, cad, power, temp         float64
	legacyAlt, legacySpeed       float64 // written in the legacy fields as well (enhanced-and-legacy.fit)
	legacyOnly                   bool    // alt and speed go into the legacy fields although the file is enhanced
	unknownField, developerField bool    // unknown-messages.fit
}

func newSample(t int) sample {
	return sample{t: t, lat: nan, lon: nan, alt: nan, dist: nan, speed: nan, hr: nan, cad: nan, power: nan, temp: nan, legacyAlt: nan, legacySpeed: nan}
}

type span struct {
	sport    typedef.Sport
	subSport typedef.SubSport
	from, to int // offsets in s
}

type timerEvent struct {
	t     int
	start bool
}

// file is the specification of one synthetic FIT file.
type file struct {
	name        string
	start       time.Time
	localOffset *int // seconds; nil = no activity message
	enhanced    bool // altitude and speed in enhanced_* fields (modern devices), else legacy fields
	noAscent    bool // session and laps without total_ascent
	bareLaps    bool // laps with start and end times only, no totals (F-18 L-1)
	bothAvg     bool // session avg_speed as well as enhanced_avg_speed, 1 m/s lower
	sessions    []span
	laps        []span // sport fields ignored
	timer       []timerEvent
	samples     []sample
	extra       []timed                    // further messages (lengths, unknown messages)
	developer   bool                       // developer_data_id and field_description at the start
	sessionDist map[int]float64            // total_distance by session start offset, for the session and for the lap with the same span, instead of last - first record distance (F-08 v1.5 and v1.6, swim-bike-distance-fall only)
	sportHints  bool                       // a sport message at each session's start, lap.sport from the containing session (F-08 v1.1)
	fileSerial  uint32                     // file_id.serial_number, 0 = none (F-09 identifying-data.fit)
	devName     string                     // developer field name, "" = "corpus_dev_field"
	preamble    []proto.Message            // messages between the head and the first timed message (F-09 identifying-data.fit; F-43 device_info)
	device      *fileDevice                // file_id manufacturer, product and product name; nil = development, 1, "corpusgen" (F-43)
	trigger     map[int]typedef.LapTrigger // lap_trigger by lap start offset; absent = not written (F-46)
	pool        *poolSpec                  // pool values of every lap-swimming session; nil = 25 m, no lengths count (F-44)
}

// fileDevice is the recording device in file_id (F-43). product 0 and name ""
// are not written.
type fileDevice struct {
	manufacturer typedef.Manufacturer
	product      uint16
	name         string
}

// poolSpec is a lap-swimming session's pool (F-44): pool_length raw
// (centimetres), its unit and num_active_lengths (0 = not written).
type poolSpec struct {
	length  uint16
	unit    typedef.DisplayMeasure
	lengths uint16
}

// sensor is a device_info message of an external sensor (F-43): index,
// source type and device type.
func sensor(t time.Time, index uint8, source typedef.SourceType, deviceType uint8) proto.Message {
	return mesgdef.NewDeviceInfo(nil).SetTimestamp(t).SetDeviceIndex(typedef.DeviceIndex(index)).
		SetSourceType(source).SetDeviceType(deviceType).ToMesg(nil)
}

type timed struct {
	t     int
	order int
	mesg  proto.Message
}

func intp(v int) *int { return &v }

var baseStart = time.Date(2026, time.June, 6, 5, 0, 0, 0, time.UTC)

// corpus lists every synthetic file (F-02 design, Required files).
func corpus() []file {
	return []file{
		indoorNoGPS(),
		indoorNoDistanceNoGPS(),
		gpsDropout(),
		pauseHours(),
		brickRunBike(),
		smartRecording(),
		ride12h(),
		ride4h(),
		enhancedAndLegacy(),
		timerPause(),
		timerBeforeFirstRecord(),
		cyclingNoTimerSlow(),
		flatNoise(),
		climb100m(),
		noDistanceWithGPS(),
		unknownMessages(),
		poolSwim(),
		poolSwimYards(),
		rollingHills(),
		systemTimeRecord(),
		ride30min(),
		run30min(),
		rideLapsNoTotals(),
		rideNoLaps(),
		rideLapTriggers(),
		poolSwim33(),
	}
}

// rideLapsNoTotals is F-18 L-1's golden: a 20 minute ride with every stream
// and three laps that carry start and end times only, so every lap value in
// the data file is computed as for a range. Timer stop 500–560 s with records
// continuing, so lap 2's moving time is shorter than its elapsed time.
func rideLapsNoTotals() file {
	const end = 1199
	f := file{name: "ride-laps-no-totals.fit", start: baseStart.Add(30 * 24 * time.Hour), localOffset: intp(3600), enhanced: true,
		bareLaps: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		laps:     []span{{from: 0, to: 400}, {from: 400, to: 800}, {from: 800, to: end}},
		timer:    []timerEvent{{0, true}, {500, false}, {560, true}, {end, false}}}
	f.samples = outdoorRide(41, end, steady(rand.New(rand.NewPCG(41, 42))), hills)
	return f
}

// rideLapTriggers is F-46's golden: a 25 minute ride with five laps whose
// lap_trigger is manual, distance, position_waypoint, session_end and not
// written (button, distance, position, stop, none).
func rideLapTriggers() file {
	const end = 1499
	f := file{name: "ride-lap-triggers.fit", start: baseStart.Add(32 * 24 * time.Hour), localOffset: intp(3600), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		laps:     []span{{from: 0, to: 300}, {from: 300, to: 600}, {from: 600, to: 900}, {from: 900, to: 1200}, {from: 1200, to: end}},
		trigger: map[int]typedef.LapTrigger{0: typedef.LapTriggerManual, 300: typedef.LapTriggerDistance,
			600: typedef.LapTriggerPositionWaypoint, 900: typedef.LapTriggerSessionEnd},
		timer: []timerEvent{{0, true}, {end, false}}}
	f.samples = outdoorRide(43, end, steady(rand.New(rand.NewPCG(43, 44))), hills)
	return f
}

// poolSwim33 is F-46's 33 m pool: pool_length recorded as 32.5 m (rounds to
// "Pool 33 m", splits every 3 lengths at 100 m).
func poolSwim33() file {
	return lapSwim("pool-swim-33m.fit", 34, 35, poolSpec{3250, typedef.DisplayMeasureMetric, 40})
}

// rideNoLaps is F-18 L-8's golden: a 15 minute ride with distance, one
// session and no lap messages ("No laps recorded.", splits offered).
func rideNoLaps() file {
	const end = 899
	f := file{name: "ride-no-laps.fit", start: baseStart.Add(31 * 24 * time.Hour), localOffset: intp(3600), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}},
		device:   &fileDevice{manufacturer: typedef.ManufacturerDevelopment}} // F-43: no device line
	f.samples = outdoorRide(43, end, steady(rand.New(rand.NewPCG(43, 44))), hills)
	return f
}

// ---- route helpers ----------------------------------------------------------

const metresPerDegree = 111195.08

// route integrates positions and device distance from the speed of each
// sample, heading turning slowly so the track is a wide loop.
type route struct {
	lat, lon, dist float64
}

func newRoute() *route { return &route{lat: 47.05, lon: 8.3} }

// step moves by speed v (m/s) for dt seconds at time t.
func (r *route) step(t int, v float64, dt int) {
	d := v * float64(dt)
	h := float64(t) / 900
	r.lat += d * math.Cos(h) / metresPerDegree
	r.lon += d * math.Sin(h) / (metresPerDegree * math.Cos(r.lat*math.Pi/180))
	r.dist += d
}

func round1(v float64) float64 { return math.Round(v) }

// ---- the files --------------------------------------------------------------

func indoorNoGPS() file {
	rng := rand.New(rand.NewPCG(1, 1))
	// F-43: brand and recorded model; a BLE power meter, an ANT+ heart-rate
	// monitor, a second power meter, a sensor of an unknown type (shifting)
	// and a built-in one: "heart-rate monitor, power meter" only.
	f := file{name: "indoor-no-gps.fit", start: baseStart, localOffset: intp(7200),
		sessions: []span{{typedef.SportCycling, typedef.SubSportIndoorCycling, 0, 3600}},
		timer:    []timerEvent{{0, true}, {3600, false}},
		device:   &fileDevice{typedef.ManufacturerWahooFitness, 31, "ELEMNT BOLT"},
		preamble: []proto.Message{
			sensor(baseStart, 0, typedef.SourceTypeLocal, 0),
			sensor(baseStart, 1, typedef.SourceTypeBluetoothLowEnergy, uint8(typedef.BleDeviceTypeBikePower)),
			sensor(baseStart, 2, typedef.SourceTypeAntplus, uint8(typedef.AntplusDeviceTypeHeartRate)),
			sensor(baseStart, 3, typedef.SourceTypeAntplus, uint8(typedef.AntplusDeviceTypeBikePower)),
			sensor(baseStart, 4, typedef.SourceTypeAntplus, uint8(typedef.AntplusDeviceTypeShifting)),
			sensor(baseStart, 5, typedef.SourceTypeLocal, uint8(typedef.AntplusDeviceTypeBikeSpeed)),
		}}
	for i := 0; i < 4; i++ {
		f.laps = append(f.laps, span{from: i * 900, to: (i + 1) * 900})
	}
	dist := 0.0
	for t := 0; t <= 3600; t++ {
		s := newSample(t)
		s.speed = 8.3 + rng.Float64() - 0.5
		if t > 0 {
			dist += s.speed
		}
		s.dist = dist
		s.hr = round1(140 + 15*math.Sin(float64(t)/300) + rng.Float64()*4)
		s.cad = round1(90 + rng.Float64()*6 - 3)
		s.power = round1(220 + 40*math.Sin(float64(t)/200) + rng.Float64()*20)
		s.temp = 21
		f.samples = append(f.samples, s)
	}
	return f
}

func indoorNoDistanceNoGPS() file {
	rng := rand.New(rand.NewPCG(2, 2))
	// F-43: no device name, one sensor: "Sensors: heart-rate monitor".
	f := file{name: "indoor-no-distance-no-gps.fit", start: baseStart.Add(24 * time.Hour),
		sessions: []span{{typedef.SportTraining, typedef.SubSportCardioTraining, 0, 1800}},
		laps:     []span{{from: 0, to: 1800}},
		timer:    []timerEvent{{0, true}, {1800, false}},
		device:   &fileDevice{manufacturer: typedef.ManufacturerDevelopment},
		preamble: []proto.Message{sensor(baseStart.Add(24*time.Hour), 1, typedef.SourceTypeAntplus, uint8(typedef.AntplusDeviceTypeHeartRate))}}
	for t := 0; t <= 1800; t++ {
		s := newSample(t)
		s.hr = round1(120 + 30*math.Sin(float64(t)/120) + rng.Float64()*3)
		f.samples = append(f.samples, s)
	}
	return f
}

// outdoorRide builds a 1 Hz ride over [0, end] with every stream; speed(t)
// gives the speed, alt(t) the altitude.
func outdoorRide(seed uint64, end int, speed func(t int) float64, alt func(t int) float64) []sample {
	rng := rand.New(rand.NewPCG(seed, seed))
	r := newRoute()
	var out []sample
	for t := 0; t <= end; t++ {
		v := speed(t)
		if t > 0 {
			r.step(t, v, 1)
		}
		s := newSample(t)
		s.lat, s.lon, s.dist, s.speed = r.lat, r.lon, r.dist, v
		s.alt = alt(t)
		s.hr = round1(135 + 20*math.Sin(float64(t)/400) + rng.Float64()*4)
		s.cad = round1(85 + rng.Float64()*8)
		s.power = round1(200 + 60*math.Sin(float64(t)/150) + rng.Float64()*30)
		s.temp = round1(18 + 4*math.Sin(float64(t)/3600))
		if v < 0.5 {
			s.cad, s.power = 0, 0
		}
		out = append(out, s)
	}
	return out
}

func hills(t int) float64 {
	return 420 + 60*math.Sin(float64(t)/700) + 25*math.Sin(float64(t)/170)
}

func steady(rng *rand.Rand) func(int) float64 {
	return func(int) float64 { return 7.5 + rng.Float64() - 0.5 }
}

func gpsDropout() file {
	f := file{name: "gps-dropout.fit", start: baseStart.Add(2 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 3600}},
		laps:     []span{{from: 0, to: 1800}, {from: 1800, to: 3600}},
		timer:    []timerEvent{{0, true}, {3600, false}}}
	f.samples = outdoorRide(3, 3600, steady(rand.New(rand.NewPCG(3, 4))), hills)
	for i := range f.samples {
		if t := f.samples[i].t; t >= 1500 && t < 1800 { // 5 min without positions
			f.samples[i].lat, f.samples[i].lon = nan, nan
		}
	}
	return f
}

func pauseHours() file {
	const resume = 1800 + 3*3600
	f := file{name: "pause-hours.fit", start: baseStart.Add(3 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, resume + 1800}},
		laps:     []span{{from: 0, to: 1800}, {from: 1800, to: resume + 1800}},
		timer:    []timerEvent{{0, true}, {1800, false}, {resume, true}, {resume + 1800, false}}}
	rng := rand.New(rand.NewPCG(5, 6))
	ride := outdoorRide(5, 3600, steady(rng), hills)
	for _, s := range ride {
		if s.t > 1800 {
			s.t += 3 * 3600 // no records while the timer is stopped; the route continues where it stopped
		}
		f.samples = append(f.samples, s)
	}
	return f
}

func brickRunBike() file {
	const runEnd, bikeStart, bikeEnd = 1200, 1260, 3660
	f := file{name: "brick-run-bike.fit", start: baseStart.Add(4 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{
			{typedef.SportRunning, typedef.SubSportRoad, 0, runEnd},
			{typedef.SportCycling, typedef.SubSportRoad, bikeStart, bikeEnd},
		},
		laps:  []span{{from: 0, to: runEnd}, {from: bikeStart, to: bikeEnd}},
		timer: []timerEvent{{0, true}, {runEnd, false}, {bikeStart, true}, {bikeEnd, false}}}
	rng := rand.New(rand.NewPCG(7, 8))
	r := newRoute()
	for t := 0; t <= bikeEnd; t++ {
		if t > runEnd-1 && t < bikeStart {
			continue // transition: no records
		}
		run := t < runEnd
		v := 7.5 + rng.Float64() - 0.5
		if run {
			v = 3.3 + rng.Float64()*0.4 - 0.2
		}
		if t > 0 && t != bikeStart {
			r.step(t, v, 1)
		}
		s := newSample(t)
		s.lat, s.lon, s.dist, s.speed, s.alt = r.lat, r.lon, r.dist, v, hills(t)
		s.hr = round1(150 + rng.Float64()*6)
		if run {
			s.cad = 85 + 64.0/128 // one leg; 171 steps per minute after R-5
		} else {
			s.cad = round1(88 + rng.Float64()*4)
			s.power = round1(210 + rng.Float64()*30)
		}
		f.samples = append(f.samples, s)
	}
	return f
}

func smartRecording() file {
	f := file{name: "smart-recording.fit", start: baseStart.Add(5 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 640}},
		laps:     []span{{from: 0, to: 640}},
		timer:    []timerEvent{{0, true}, {640, false}}}
	// Steps of 1 s except one gap each of 3, 4, 5 and 30 s.
	gaps := map[int]int{100: 3, 200: 4, 300: 5, 400: 30}
	all := outdoorRide(9, 640, steady(rand.New(rand.NewPCG(9, 10))), hills)
	for t := 0; t <= 640; {
		f.samples = append(f.samples, all[t])
		if g, ok := gaps[t]; ok {
			t += g
		} else {
			t++
		}
	}
	return f
}

func ride12h() file {
	const end = 43199 // 43,200 records at 1 Hz
	f := file{name: "ride-12h.fit", start: baseStart.Add(6 * 24 * time.Hour).Add(-2 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 1800 {
		f.laps = append(f.laps, span{from: from, to: min(from+1800, end)})
	}
	rng := rand.New(rand.NewPCG(11, 12))
	f.samples = outdoorRide(11, end, func(t int) float64 {
		return 7 + 1.5*math.Sin(float64(t)/1100) + rng.Float64() - 0.5
	}, hills)
	return f
}

// ride4h is the size-budget file (F-03 AC 5): 14,400 records at 1 Hz with
// every stream, deliberately noisier than the other files so its compressed
// size is not flattered by smooth signals (F-03 design, section Size budget).
func ride4h() file {
	const end = 14399 // 14,400 records at 1 Hz
	f := file{name: "ride-4h.fit", start: baseStart.Add(8 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 900 {
		f.laps = append(f.laps, span{from: from, to: min(from+900, end)})
	}
	rng := rand.New(rand.NewPCG(33, 34))
	f.samples = outdoorRide(33, end, func(t int) float64 {
		return 7 + 1.5*math.Sin(float64(t)/1100) + rng.Float64() - 0.5
	}, hills)
	noise := rand.New(rand.NewPCG(35, 36))
	u := func(a float64) float64 { return (noise.Float64()*2 - 1) * a } // U(-a, a)
	for i := range f.samples {
		s := &f.samples[i]
		// GPS jitter of up to 1.5 m per axis, not accumulated, not in distance.
		s.lat += u(1.5) / metresPerDegree
		s.lon += u(1.5) / (metresPerDegree * math.Cos(s.lat*math.Pi/180))
		s.alt += u(0.3)
		s.power = math.Max(0, math.Round(200+60*math.Sin(float64(s.t)/150)+u(80)))
	}
	return f
}

// enhancedAndLegacy has three segments of 60 records (F-02 design v1.1):
// (1) both fields with different values: altitude 410 / enhanced 420 m,
// speed 6.999 / enhanced 7.999 m/s; (2) legacy fields only; (3) enhanced
// fields only. The session has avg_speed and enhanced_avg_speed.
func enhancedAndLegacy() file {
	f := file{name: "enhanced-and-legacy.fit", start: baseStart.Add(7 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, bothAvg: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 179}},
		laps:     []span{{from: 0, to: 179}},
		timer:    []timerEvent{{0, true}, {179, false}}}
	speed := func(t int) float64 {
		switch {
		case t < 60:
			return 7.999
		case t < 120:
			return 5.5
		}
		return 6.5
	}
	alt := func(t int) float64 {
		switch {
		case t < 60:
			return 420
		case t < 120:
			return 430
		}
		return 440
	}
	f.samples = outdoorRide(13, 179, speed, alt)
	for i := range f.samples {
		switch t := f.samples[i].t; {
		case t < 60:
			f.samples[i].legacyAlt, f.samples[i].legacySpeed = 410, 6.999
		case t < 120:
			f.samples[i].legacyOnly = true
		}
	}
	return f
}

// rollingHills makes the elevation filter (R-8) visible in the corpus: a
// 100 m base; each bump a 20 s rise, 10 s flat top, 20 s fall and 30 s at
// the base; 20 bumps of 2.2 m (below H = 3), then 10 bumps of 6 m; uniform
// ±0.5 m noise; no total_ascent.
func rollingHills() file {
	const end = 30 * 80
	f := file{name: "rolling-hills.fit", start: baseStart.Add(15 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, noAscent: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		laps:     []span{{from: 0, to: end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	rng := rand.New(rand.NewPCG(29, 30))
	f.samples = outdoorRide(29, end, func(int) float64 { return 6 }, func(t int) float64 {
		return bumps(t) + rng.Float64() - 0.5
	})
	return f
}

// systemTimeRecord has 600 records at 1 Hz on a 2026 date and, in the
// middle, one extra record with timestamp 1,000 and one timer stop event
// with timestamp 2,000: FIT system time, seconds since power-on (E-8).
func systemTimeRecord() file {
	f := file{name: "system-time-record.fit", start: baseStart.Add(16 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 599}},
		laps:     []span{{from: 0, to: 599}},
		timer:    []timerEvent{{0, true}, {599, false}}}
	f.samples = outdoorRide(31, 599, steady(rand.New(rand.NewPCG(31, 32))), hills)
	sysRecord := mesgdef.NewRecord(nil).SetTimestamp(datetime.ToTime(1000)).SetHeartRate(99).SetPower(999).ToMesg(nil)
	sysStop := mesgdef.NewEvent(nil).SetTimestamp(datetime.ToTime(2000)).SetEvent(typedef.EventTimer).
		SetEventType(typedef.EventTypeStopAll).SetEventGroup(0).ToMesg(nil)
	f.extra = []timed{
		{t: 300, order: orderExtra, mesg: sysRecord},
		{t: 300, order: orderExtra, mesg: sysStop},
	}
	return f
}

func bumps(t int) float64 {
	bump, phase := t/80, t%80
	if bump >= 30 {
		return 100
	}
	h := 2.2
	if bump >= 20 {
		h = 6
	}
	switch {
	case phase < 20:
		return 100 + h*float64(phase)/20
	case phase < 30:
		return 100 + h
	case phase < 50:
		return 100 + h*float64(50-phase)/20
	}
	return 100
}

func timerPause() file {
	f := file{name: "timer-pause.fit", start: baseStart.Add(8 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 1800}},
		laps:     []span{{from: 0, to: 1800}},
		timer:    []timerEvent{{0, true}, {600, false}, {1200, true}, {1800, false}}}
	rng := rand.New(rand.NewPCG(15, 16))
	// Records continue during the 10 min stop at 2 m/s (bike pushed), so only
	// the timer events can exclude it from moving time.
	f.samples = outdoorRide(15, 1800, func(t int) float64 {
		if t > 600 && t <= 1200 {
			return 2
		}
		return 7.5 + rng.Float64() - 0.5
	}, hills)
	return f
}

// timerBeforeFirstRecord: the timer starts with the session at 0 s, the
// first record comes at 3 s (ADR-029 rev. 1, QA-17-r1-1). The 3 s before it
// count as moving time in the first interval, so M[0] = 0 and M[N-1] equals
// the summary moving time of 900 s.
func timerBeforeFirstRecord() file {
	f := file{name: "timer-before-first-record.fit", start: baseStart.Add(28 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 900}},
		laps:     []span{{from: 0, to: 900}},
		timer:    []timerEvent{{0, true}, {900, false}}}
	rng := rand.New(rand.NewPCG(71, 72))
	f.samples = outdoorRide(71, 900, steady(rng), hills)[3:]
	return f
}

func cyclingNoTimerSlow() file {
	f := file{name: "cycling-no-timer-slow.fit", start: baseStart.Add(9 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 1199}},
		laps:     []span{{from: 0, to: 1199}}}
	// 0.2 m/s = 0.72 km/h: below the cycling threshold (1 km/h), above the running one (0.5 km/h).
	f.samples = outdoorRide(17, 1199, func(t int) float64 {
		switch {
		case t >= 300 && t < 600:
			return 0.2
		case t >= 600 && t < 900:
			return 0
		}
		return 7
	}, hills)
	return f
}

func flatNoise() file {
	f := file{name: "flat-noise.fit", start: baseStart.Add(10 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, noAscent: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 3599}},
		laps:     []span{{from: 0, to: 3599}},
		timer:    []timerEvent{{0, true}, {3599, false}}}
	rng := rand.New(rand.NewPCG(19, 20))
	f.samples = outdoorRide(19, 3599, func(int) float64 { return 7 }, func(int) float64 {
		return 100 + rng.Float64()*2 - 1 // uniform ±1 m
	})
	return f
}

func climb100m() file {
	f := file{name: "climb-100m.fit", start: baseStart.Add(11 * 24 * time.Hour), localOffset: intp(19800), enhanced: true, noAscent: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportMountain, 0, 1119}},
		laps:     []span{{from: 0, to: 1119}},
		timer:    []timerEvent{{0, true}, {1119, false}}}
	f.samples = outdoorRide(21, 1119, func(int) float64 { return 4 }, func(t int) float64 {
		switch {
		case t < 60:
			return 100
		case t <= 1060:
			return 100 + float64(t-60)/10
		}
		return 200
	})
	return f
}

func noDistanceWithGPS() file {
	f := file{name: "no-distance-with-gps.fit", start: baseStart.Add(12 * 24 * time.Hour), localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportRunning, typedef.SubSportTrail, 0, 1800}},
		laps:     []span{{from: 0, to: 900}, {from: 900, to: 1800}},
		timer:    []timerEvent{{0, true}, {1800, false}}}
	rng := rand.New(rand.NewPCG(23, 24))
	for _, s := range outdoorRide(23, 1800, func(int) float64 { return 3 + rng.Float64()*0.4 }, hills) {
		s.dist, s.speed, s.power, s.temp = nan, nan, nan, nan
		s.cad = 84 + 32.0/128
		f.samples = append(f.samples, s)
	}
	return f
}

func unknownMessages() file {
	f := file{name: "unknown-messages.fit", start: baseStart.Add(13 * 24 * time.Hour), localOffset: intp(7200), enhanced: true, developer: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 300}},
		laps:     []span{{from: 0, to: 300}},
		timer:    []timerEvent{{0, true}, {300, false}}}
	f.samples = outdoorRide(25, 300, steady(rand.New(rand.NewPCG(25, 26))), hills)
	for i := range f.samples {
		f.samples[i].unknownField = true
		f.samples[i].developerField = i%10 == 0
	}
	// A manufacturer-specific message (number in 0xFF00..0xFFFE) every minute.
	for t := 0; t <= 300; t += 60 {
		f.extra = append(f.extra, timed{t: t, order: orderRecord + 1, mesg: proto.Message{
			Num: typedef.MesgNum(0xFF01),
			Fields: []proto.Field{
				rawField(253, basetype.Uint32, proto.Uint32(fitTime(f.start.Add(time.Duration(t)*time.Second)))),
				rawField(1, basetype.Uint16, proto.Uint16(uint16(1000+t))),
				rawField(2, basetype.Sint8, proto.Int8(-3)),
			},
		}})
	}
	return f
}

// poolSwim is pool-swim.fit, a 25 m pool (F-44: num_active_lengths 40
// and one rest length).
func poolSwim() file {
	return lapSwim("pool-swim.fit", 14, 27, poolSpec{2500, typedef.DisplayMeasureMetric, 40})
}

// poolSwimYards is pool-swim-25yd.fit (F-44): the same swim in a 25 yd pool
// (22.86 m).
func poolSwimYards() file {
	return lapSwim("pool-swim-25yd.fit", 29, 31, poolSpec{2286, typedef.DisplayMeasureStatute, 40})
}

// lapSwim is 40 active lengths of 30 s each, one lap per 4 lengths, and a
// rest length of 0 s after the last one, in pool p.
func lapSwim(name string, day int, seed uint64, p poolSpec) file {
	const lengths, lengthSecs = 40, 30
	poolM := float64(p.length) / 100
	end := lengths * lengthSecs
	f := file{name: name, start: baseStart.Add(time.Duration(day) * 24 * time.Hour), localOffset: intp(-14400),
		sessions: []span{{typedef.SportSwimming, typedef.SubSportLapSwimming, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}, pool: &p}
	for i := 0; i < lengths; i += 4 { // one lap per 100 m
		f.laps = append(f.laps, span{from: i * lengthSecs, to: (i + 4) * lengthSecs})
	}
	rng := rand.New(rand.NewPCG(seed, seed+1))
	for t := 0; t <= end; t++ {
		s := newSample(t)
		s.dist = poolM * float64(t/lengthSecs)
		s.speed = poolM / lengthSecs
		s.hr = round1(128 + rng.Float64()*8)
		s.cad = round1(28 + rng.Float64()*4) // strokes per minute
		f.samples = append(f.samples, s)
	}
	for i := 0; i < lengths; i++ {
		from := i * lengthSecs
		l := mesgdef.NewLength(nil).
			SetTimestamp(f.start.Add(time.Duration(from+lengthSecs) * time.Second)).
			SetStartTime(f.start.Add(time.Duration(from) * time.Second)).
			SetTotalElapsedTime(lengthSecs * 1000).
			SetTotalTimerTime(lengthSecs * 1000).
			SetTotalStrokes(18).
			SetSwimStroke(typedef.SwimStrokeFreestyle).
			SetLengthType(typedef.LengthTypeActive)
		f.extra = append(f.extra, timed{t: from + lengthSecs, order: orderRecord + 1, mesg: l.ToMesg(nil)})
	}
	rest := mesgdef.NewLength(nil).SetTimestamp(at(f, end)).SetStartTime(at(f, end)).
		SetTotalElapsedTime(0).SetTotalTimerTime(0).SetLengthType(typedef.LengthTypeIdle)
	f.extra = append(f.extra, timed{t: end, order: orderRecord + 1, mesg: rest.ToMesg(nil)})
	return f
}

// ---- encoding ---------------------------------------------------------------

// Message order at the same second: start events, sport messages, records, other messages,
// stop events, laps, sessions, activity.
const (
	orderStart = iota
	orderSport
	orderRecord
	orderExtra
	orderStop
	orderLap
	orderSession
	orderActivity
)

func fitTime(t time.Time) uint32 {
	return uint32(t.Unix() - time.Date(1989, time.December, 31, 0, 0, 0, 0, time.UTC).Unix())
}

func rawField(num byte, bt basetype.BaseType, v proto.Value) proto.Field {
	return proto.Field{
		FieldBase: &proto.FieldBase{Name: "unknown", Num: num, BaseType: bt, Scale: 1},
		Value:     v,
	}
}

func at(f file, t int) time.Time { return f.start.Add(time.Duration(t) * time.Second) }

func semicircles(deg float64) int32 { return int32(math.Round(deg * 2147483648.0 / 180)) }

func altRaw(m float64) uint32 { return uint32(math.Round((m + 500) * 5)) }

func speedRaw(v float64) uint32 { return uint32(math.Round(v * 1000)) }

func encode(f file) ([]byte, error) {
	var msgs []timed
	add := func(t, order int, m proto.Message) { msgs = append(msgs, timed{t, order, m}) }

	for _, e := range f.timer {
		et, order := typedef.EventTypeStart, orderStart
		if !e.start {
			et, order = typedef.EventTypeStopAll, orderStop
		}
		ev := mesgdef.NewEvent(nil).SetTimestamp(at(f, e.t)).SetEvent(typedef.EventTimer).SetEventType(et).SetEventGroup(0)
		add(e.t, order, ev.ToMesg(nil))
	}
	for _, s := range f.samples {
		add(s.t, orderRecord, recordMesg(f, s))
	}
	if f.sportHints {
		for _, sp := range f.sessions {
			add(sp.from, orderSport, mesgdef.NewSport(nil).SetSport(sp.sport).SetSubSport(sp.subSport).ToMesg(nil))
		}
	}
	for _, l := range f.laps {
		add(l.to, orderLap, lapMesg(f, l))
	}
	for _, s := range f.sessions {
		add(s.to, orderSession, sessionMesg(f, s))
	}
	msgs = append(msgs, f.extra...)
	last := 0
	for _, m := range msgs {
		last = max(last, m.t)
	}
	if f.localOffset != nil {
		a := mesgdef.NewActivity(nil).
			SetTimestamp(at(f, last)).
			SetLocalTimestamp(at(f, last+*f.localOffset)).
			SetNumSessions(uint16(len(f.sessions))).
			SetType(typedef.ActivityManual).
			SetEvent(typedef.EventActivity).
			SetEventType(typedef.EventTypeStop).
			SetTotalTimerTime(uint32(timerTime(f, 0, last) * 1000))
		add(last, orderActivity, a.ToMesg(nil))
	}
	sort.SliceStable(msgs, func(i, j int) bool {
		if msgs[i].t != msgs[j].t {
			return msgs[i].t < msgs[j].t
		}
		return msgs[i].order < msgs[j].order
	})

	dev := cmp.Or(f.device, &fileDevice{typedef.ManufacturerDevelopment, 1, "corpusgen"})
	fid := mesgdef.NewFileId(nil).
		SetType(typedef.FileActivity).
		SetManufacturer(dev.manufacturer).
		SetTimeCreated(f.start)
	if dev.product != 0 {
		fid.SetProduct(dev.product)
	}
	if dev.name != "" {
		fid.SetProductName(dev.name)
	}
	if f.fileSerial != 0 {
		fid.SetSerialNumber(f.fileSerial)
	}
	head := []proto.Message{fid.ToMesg(nil)}
	devName := cmp.Or(f.devName, "corpus_dev_field")
	if f.developer {
		head = append(head,
			mesgdef.NewDeveloperDataId(nil).SetDeveloperDataIndex(0).SetApplicationVersion(1).ToMesg(nil),
			mesgdef.NewFieldDescription(nil).
				SetDeveloperDataIndex(0).
				SetFieldDefinitionNumber(0).
				SetFitBaseTypeId(basetype.Uint8).
				SetFieldName([]string{devName}).
				SetUnits([]string{"count"}).
				SetNativeMesgNum(typedef.MesgNumRecord).
				ToMesg(nil))
	}
	fit := proto.FIT{FileHeader: proto.FileHeader{ProtocolVersion: proto.V2}}
	fit.Messages = append(head, f.preamble...)
	for _, m := range msgs {
		fit.Messages = append(fit.Messages, m.mesg)
	}
	var buf bytes.Buffer
	if err := encoder.New(&buf).Encode(&fit); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func recordMesg(f file, s sample) proto.Message {
	r := mesgdef.NewRecord(nil).SetTimestamp(at(f, s.t))
	if !math.IsNaN(s.lat) {
		r.SetPositionLat(semicircles(s.lat)).SetPositionLong(semicircles(s.lon))
	}
	if !math.IsNaN(s.alt) {
		if f.enhanced && !s.legacyOnly {
			r.SetEnhancedAltitude(altRaw(s.alt))
		} else {
			r.SetAltitude(uint16(altRaw(s.alt)))
		}
	}
	if !math.IsNaN(s.legacyAlt) {
		r.SetAltitude(uint16(altRaw(s.legacyAlt)))
	}
	if !math.IsNaN(s.speed) {
		if f.enhanced && !s.legacyOnly {
			r.SetEnhancedSpeed(speedRaw(s.speed))
		} else {
			r.SetSpeed(uint16(speedRaw(s.speed)))
		}
	}
	if !math.IsNaN(s.legacySpeed) {
		r.SetSpeed(uint16(speedRaw(s.legacySpeed)))
	}
	if !math.IsNaN(s.dist) {
		r.SetDistance(uint32(math.Round(s.dist * 100)))
	}
	if !math.IsNaN(s.hr) {
		r.SetHeartRate(uint8(s.hr))
	}
	if !math.IsNaN(s.cad) {
		whole := math.Floor(s.cad)
		r.SetCadence(uint8(whole))
		if frac := s.cad - whole; frac > 0 {
			r.SetFractionalCadence(uint8(math.Round(frac * 128)))
		}
	}
	if !math.IsNaN(s.power) {
		r.SetPower(uint16(s.power))
	}
	if !math.IsNaN(s.temp) {
		r.SetTemperature(int8(s.temp))
	}
	m := r.ToMesg(nil)
	if s.unknownField {
		m.Fields = append(m.Fields, rawField(250, basetype.Uint16, proto.Uint16(uint16(s.t))))
	}
	if s.developerField {
		m.DeveloperFields = append(m.DeveloperFields, proto.DeveloperField{Num: 0, DeveloperDataIndex: 0, Value: proto.Uint8(uint8(s.t % 200))})
	}
	return m
}

// stats are the totals of the samples within [from, to].
type stats struct {
	elapsed, timer, dist                 float64
	ascent, descent                      float64
	avgSpeed, maxSpeed                   float64
	avgHR, maxHR, avgCad, maxCad         float64
	avgPower, maxPower, avgTemp, maxTemp float64
	hasDist, hasAlt, hasSpeed            bool
	hasHR, hasCad, hasPower, hasTemp     bool
}

func computeStats(f file, from, to int) stats {
	st := stats{elapsed: float64(to - from), timer: timerTime(f, from, to)}
	var in []sample
	for _, s := range f.samples {
		if s.t >= from && s.t <= to {
			in = append(in, s)
		}
	}
	type acc struct {
		sum, max float64
		n        int
	}
	var hr, cad, pw, tp, sp acc
	add := func(a *acc, v float64) {
		if math.IsNaN(v) {
			return
		}
		a.sum += v
		a.n++
		a.max = math.Max(a.max, v)
	}
	var firstDist, lastDist, posDist float64 = nan, nan, 0
	var prev *sample
	var prevAlt = nan
	for i := range in {
		s := in[i]
		add(&hr, s.hr)
		add(&cad, s.cad)
		add(&pw, s.power)
		add(&tp, s.temp)
		add(&sp, s.speed)
		if !math.IsNaN(s.dist) {
			if math.IsNaN(firstDist) {
				firstDist = s.dist
			}
			lastDist = s.dist
		}
		if !math.IsNaN(s.lat) {
			if prev != nil {
				posDist += haversine(prev.lat, prev.lon, s.lat, s.lon)
			}
			prev = &in[i]
		}
		if !math.IsNaN(s.alt) {
			st.hasAlt = true
			if !math.IsNaN(prevAlt) {
				if d := s.alt - prevAlt; d > 0 {
					st.ascent += d
				} else {
					st.descent -= d
				}
			}
			prevAlt = s.alt
		}
	}
	switch {
	case !math.IsNaN(firstDist):
		st.dist, st.hasDist = lastDist-firstDist, true
	case prev != nil:
		st.dist, st.hasDist = posDist, true
	}
	if st.hasDist && st.timer > 0 {
		st.avgSpeed, st.hasSpeed = st.dist/st.timer, true
	}
	st.maxSpeed = nan
	if sp.n > 0 {
		st.maxSpeed = sp.max
	}
	mean := func(a acc) float64 { return a.sum / float64(a.n) }
	st.hasHR, st.hasCad, st.hasPower, st.hasTemp = hr.n > 0, cad.n > 0, pw.n > 0, tp.n > 0
	if st.hasHR {
		st.avgHR, st.maxHR = mean(hr), hr.max
	}
	if st.hasCad {
		st.avgCad, st.maxCad = mean(cad), cad.max
	}
	if st.hasPower {
		st.avgPower, st.maxPower = mean(pw), pw.max
	}
	if st.hasTemp {
		st.avgTemp, st.maxTemp = mean(tp), tp.max
	}
	return st
}

// timerTime is the time within [from, to] with the timer running; the whole
// span without timer events.
func timerTime(f file, from, to int) float64 {
	if len(f.timer) == 0 {
		return float64(to - from)
	}
	total, running, since := 0, false, 0
	for _, e := range f.timer {
		switch {
		case e.start && !running:
			running, since = true, e.t
		case !e.start && running:
			total += max(0, min(e.t, to)-max(since, from))
			running = false
		}
	}
	return float64(total)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	a := math.Pow(math.Sin((lat2-lat1)*rad/2), 2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Pow(math.Sin((lon2-lon1)*rad/2), 2)
	return 2 * 6371008.8 * math.Asin(math.Sqrt(a))
}

func sessionMesg(f file, sp span) proto.Message {
	st := computeStats(f, sp.from, sp.to)
	if v, ok := f.sessionDist[sp.from]; ok {
		st.dist, st.hasDist = v, true
		if st.timer > 0 {
			st.avgSpeed, st.hasSpeed = v/st.timer, true
		}
	}
	m := mesgdef.NewSession(nil).
		SetTimestamp(at(f, sp.to)).
		SetStartTime(at(f, sp.from)).
		SetSport(sp.sport).
		SetSubSport(sp.subSport).
		SetEvent(typedef.EventSession).
		SetEventType(typedef.EventTypeStop).
		SetTotalElapsedTime(uint32(st.elapsed * 1000)).
		SetTotalTimerTime(uint32(st.timer * 1000)).
		SetTotalCalories(uint16(st.timer / 60 * 10))
	if st.hasDist {
		m.SetTotalDistance(uint32(math.Round(st.dist * 100)))
	}
	if st.hasAlt {
		if !f.noAscent {
			m.SetTotalAscent(uint16(math.Round(st.ascent)))
		}
		m.SetTotalDescent(uint16(math.Round(st.descent)))
	}
	if st.hasSpeed {
		if f.enhanced {
			m.SetEnhancedAvgSpeed(speedRaw(st.avgSpeed))
		} else {
			m.SetAvgSpeed(uint16(speedRaw(st.avgSpeed)))
		}
	}
	if !math.IsNaN(st.maxSpeed) {
		if f.enhanced {
			m.SetEnhancedMaxSpeed(speedRaw(st.maxSpeed))
		} else {
			m.SetMaxSpeed(uint16(speedRaw(st.maxSpeed)))
		}
	}
	if st.hasHR {
		m.SetAvgHeartRate(uint8(math.Round(st.avgHR))).SetMaxHeartRate(uint8(st.maxHR))
	}
	if st.hasCad {
		whole := math.Floor(st.avgCad)
		m.SetAvgCadence(uint8(whole)).SetAvgFractionalCadence(uint8(math.Round((st.avgCad - whole) * 128)))
		m.SetMaxCadence(uint8(math.Floor(st.maxCad)))
	}
	if st.hasPower {
		m.SetAvgPower(uint16(math.Round(st.avgPower))).SetMaxPower(uint16(st.maxPower))
	}
	if st.hasTemp {
		m.SetAvgTemperature(int8(math.Round(st.avgTemp))).SetMaxTemperature(int8(st.maxTemp))
	}
	if f.bothAvg && st.hasSpeed {
		m.SetAvgSpeed(uint16(speedRaw(st.avgSpeed - 1)))
	}
	if sp.sport == typedef.SportSwimming {
		// Open-water swims keep 25 m too: the negative case of F-44 PS-1.
		p := cmp.Or(f.pool, &poolSpec{2500, typedef.DisplayMeasureMetric, 0})
		m.SetPoolLength(p.length).SetPoolLengthUnit(p.unit)
		if p.lengths != 0 && sp.subSport == typedef.SubSportLapSwimming {
			m.SetNumActiveLengths(p.lengths).SetNumLengths(p.lengths + 1) // one rest length
		}
	}
	return m.ToMesg(nil)
}

func lapMesg(f file, sp span) proto.Message {
	st := computeStats(f, sp.from, sp.to)
	// F-08 v1.6: a lap covering exactly a session's span gets the session's override.
	for _, ss := range f.sessions {
		if ss.from == sp.from && ss.to == sp.to {
			if v, ok := f.sessionDist[ss.from]; ok {
				st.dist, st.hasDist = v, true
				if st.timer > 0 {
					st.avgSpeed, st.hasSpeed = v/st.timer, true
				}
			}
			break
		}
	}
	m := mesgdef.NewLap(nil).
		SetTimestamp(at(f, sp.to)).
		SetStartTime(at(f, sp.from)).
		SetEvent(typedef.EventLap).
		SetEventType(typedef.EventTypeStop)
	if t, ok := f.trigger[sp.from]; ok {
		m.SetLapTrigger(t)
	}
	if f.bareLaps {
		return m.ToMesg(nil)
	}
	m.SetTotalElapsedTime(uint32(st.elapsed * 1000)).
		SetTotalTimerTime(uint32(st.timer * 1000))
	if f.sportHints {
		for _, sp2 := range f.sessions {
			if sp2.from <= sp.from && sp.from <= sp2.to {
				m.SetSport(sp2.sport).SetSubSport(sp2.subSport)
				break
			}
		}
	}
	if st.hasDist {
		m.SetTotalDistance(uint32(math.Round(st.dist * 100)))
	}
	if st.hasAlt {
		if !f.noAscent {
			m.SetTotalAscent(uint16(math.Round(st.ascent)))
		}
		m.SetTotalDescent(uint16(math.Round(st.descent)))
	}
	if st.hasSpeed {
		if f.enhanced {
			m.SetEnhancedAvgSpeed(speedRaw(st.avgSpeed))
		} else {
			m.SetAvgSpeed(uint16(speedRaw(st.avgSpeed)))
		}
	}
	if !math.IsNaN(st.maxSpeed) {
		if f.enhanced {
			m.SetEnhancedMaxSpeed(speedRaw(st.maxSpeed))
		} else {
			m.SetMaxSpeed(uint16(speedRaw(st.maxSpeed)))
		}
	}
	if st.hasHR {
		m.SetAvgHeartRate(uint8(math.Round(st.avgHR))).SetMaxHeartRate(uint8(st.maxHR))
	}
	if st.hasCad {
		whole := math.Floor(st.avgCad)
		m.SetAvgCadence(uint8(whole)).SetAvgFractionalCadence(uint8(math.Round((st.avgCad - whole) * 128)))
		m.SetMaxCadence(uint8(math.Floor(st.maxCad)))
	}
	if st.hasPower {
		m.SetAvgPower(uint16(math.Round(st.avgPower))).SetMaxPower(uint16(st.maxPower))
	}
	if st.hasTemp {
		m.SetAvgTemperature(int8(math.Round(st.avgTemp))).SetMaxTemperature(int8(st.maxTemp))
	}
	return m.ToMesg(nil)
}
