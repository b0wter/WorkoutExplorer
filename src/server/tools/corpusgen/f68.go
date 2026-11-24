package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

// Files of F-68 (extra sensor streams): running dynamics and developer
// fields. Every value and every name is invented. The hostile file's names,
// units, application ID and value series are the markers the publish tests
// search for in every public output (see hostile* constants in
// internal/publish/f68_test.go, which must match).

// devVal is a developer value written into a record message.
type devVal struct {
	index, num uint8
	v          proto.Value
}

// descOpts are the optional parts of a field_description.
type descOpts struct {
	scale       *uint8
	offset      *int8
	nativeMesg  bool // native_mesg_num = record
	nativeField *uint8
}

func u8p(v uint8) *uint8 { return &v }
func i8p(v int8) *int8   { return &v }

func appID(index uint8, id string) proto.Message {
	b := make([]byte, 16)
	copy(b, id)
	return mesgdef.NewDeveloperDataId(nil).SetDeveloperDataIndex(index).SetApplicationId(b).SetApplicationVersion(7).ToMesg(nil)
}

func desc(index, num uint8, typ basetype.BaseType, name, units string, o descOpts) proto.Message {
	d := mesgdef.NewFieldDescription(nil).SetDeveloperDataIndex(index).SetFieldDefinitionNumber(num).
		SetFitBaseTypeId(typ).SetFieldName([]string{name}).SetUnits([]string{units})
	if o.scale != nil {
		d.SetScale(*o.scale)
	}
	if o.offset != nil {
		d.SetOffset(*o.offset)
	}
	if o.nativeMesg {
		d.SetNativeMesgNum(typedef.MesgNumRecord)
	}
	if o.nativeField != nil {
		d.SetNativeFieldNum(*o.nativeField)
	}
	return d.ToMesg(nil)
}

// strydDesc is the field_description of Stryd's power (F68-Q1 strings).
func strydDesc(index, num uint8) proto.Message {
	return desc(index, num, basetype.Uint16, "Power", "Watts", descOpts{})
}

// dynamicsRun is a 30 minute run whose device writes the four standard
// running dynamics fields (XS-1): 20 s of zeros (standing), a stretch of 30 s
// without the fields, then values. Native power is absent.
func dynamicsRun(name string, day int, seed uint64, end int) file {
	f := file{name: name, start: baseStart.Add(time.Duration(day) * 24 * time.Hour), localOffset: intp(7200), enhanced: true, sportHints: true,
		sessions: []span{{typedef.SportRunning, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 300 {
		f.laps = append(f.laps, span{from: from, to: min(from+300, end)})
	}
	rng := rand.New(rand.NewPCG(seed, seed+1))
	f.samples = outdoorRide(seed, end, func(int) float64 { return 3.3 + rng.Float64()*0.4 - 0.2 }, hills)
	for i := range f.samples {
		s := &f.samples[i]
		s.cad, s.power = 85+64.0/128+float64(rng.IntN(5)-2), nan
		switch {
		case s.t < 20: // standing: raw 0 is missing (XS-1)
			s.hasDyn = true
		case s.t >= 100 && s.t < 130: // fields not written
		default:
			s.hasDyn = true
			// raw units: mm x 10, ms x 10, mm x 10, % x 100
			s.dyn = [4]uint16{uint16(850 + rng.IntN(60)), uint16(2400 + rng.IntN(200)), uint16(10500 + rng.IntN(400)), uint16(850 + rng.IntN(40))}
		}
	}
	return f
}

// runDynamics4h is the size-budget run (F68 stop rule): 14,400 records with
// every stream, native power and the four dynamics, noisy like ride-4h.
func runDynamics4h() file { return runDynamicsLong("run-4h-dynamics.fit", 62, 71, 73, 14399) }

// runDynamics12h is the 12-hour gate file (F-68 AC 7): 43,200 records, all ten charts.
func runDynamics12h() file { return runDynamicsLong("run-12h-dynamics.fit", 63, 75, 77, 43199) }

func runDynamicsLong(name string, day int, seed, seed2 uint64, end int) file {
	f := dynamicsRun(name, day, seed, end)
	f.laps = nil
	for from := 0; from < end; from += 900 {
		f.laps = append(f.laps, span{from: from, to: min(from+900, end)})
	}
	rng := rand.New(rand.NewPCG(seed2, seed2+1))
	u := func(a float64) float64 { return (rng.Float64()*2 - 1) * a }
	for i := range f.samples {
		s := &f.samples[i]
		s.lat += u(1.5) / metresPerDegree
		s.lon += u(1.5) / (metresPerDegree * math.Cos(s.lat*math.Pi/180))
		s.alt += u(0.3)
		s.power = math.Max(0, math.Round(250+50*math.Sin(float64(s.t)/150)+u(40)))
		s.hasDyn = true
		s.dyn = [4]uint16{uint16(850 + rng.IntN(300)), uint16(2300 + rng.IntN(500)), uint16(10000 + rng.IntN(1500)), uint16(800 + rng.IntN(150))}
	}
	return f
}

// capDynamics adds the four dynamics and a Stryd power developer field (the
// description number 1 of the cap file) to every record of the all-caps file,
// for the memory measurement of ADR-017 (F-68 XS-11). Native power stays, so
// the Stryd values are collected and then dropped, the worst case.
func capDynamics(f *file) {
	for i, e := range f.extra { // description 1 of the 256 becomes the Stryd one: the budget stays at 256
		if e.mesg.Num == typedef.MesgNumFieldDescription && e.mesg.FieldValueByNum(1).Uint8() == 1 {
			f.extra[i].mesg = strydDesc(0, 1)
		}
	}
	for i := range f.samples {
		s := &f.samples[i]
		s.hasDyn = true
		s.dyn = [4]uint16{uint16(850 + i%60), uint16(2400 + i%200), uint16(10500 + i%400), uint16(850 + i%40)}
		s.dev = append(s.dev, devVal{0, 1, proto.Uint16(uint16(200 + i%90))})
	}
}

// strydRun is a 20 minute run without native power; the device writes
// Stryd's fields (power and three others the parser must ignore).
func strydRun(name string, day int, seed uint64) file {
	const end = 1199
	f := file{name: name, start: baseStart.Add(time.Duration(day) * 24 * time.Hour), localOffset: intp(7200), enhanced: true, sportHints: true,
		sessions: []span{{typedef.SportRunning, typedef.SubSportRoad, 0, end}},
		laps:     []span{{from: 0, to: 600}, {from: 600, to: end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	rng := rand.New(rand.NewPCG(seed, seed+1))
	f.samples = outdoorRide(seed, end, func(int) float64 { return 3.3 + rng.Float64()*0.4 - 0.2 }, hills)
	for i := range f.samples {
		f.samples[i].cad, f.samples[i].power = 85+64.0/128, nan
	}
	return f
}

// strydWatts is the invented Stryd power of second t.
func strydWatts(t int) uint16 { return uint16(240 + 30*math.Sin(float64(t)/90) + float64(t%7)) }

func strydFields(f *file, index uint8, from, to int) {
	for i := range f.samples {
		if t := f.samples[i].t; t >= from && t <= to {
			f.samples[i].dev = append(f.samples[i].dev,
				devVal{index, 0, proto.Uint16(strydWatts(t))},
				devVal{index, 1, proto.Uint16(uint16(300 + t%11))},           // "Form Power"
				devVal{index, 2, proto.Float32(float32(9.5 + float64(t%5)))}, // "Leg Spring Stiffness"
			)
		}
	}
}

func strydHead(index uint8, app string) []proto.Message {
	return []proto.Message{
		appID(index, app),
		strydDesc(index, 0),
		desc(index, 1, basetype.Uint16, "Form Power", "Watts", descOpts{}),
		desc(index, 2, basetype.Float32, "Leg Spring Stiffness", "kN/m", descOpts{}),
	}
}

func strydOnly() file {
	f := strydRun("stryd-power.fit", 63, 81)
	f.preamble = strydHead(0, "stryd-app-one")
	strydFields(&f, 0, 0, 1199)
	return f
}

func strydAndNative() file {
	f := strydRun("stryd-and-native-power.fit", 64, 83)
	f.preamble = strydHead(0, "stryd-app-one")
	strydFields(&f, 0, 0, 1199)
	for i := range f.samples {
		f.samples[i].power = float64(150 + f.samples[i].t%40) // native: differs from Stryd
	}
	return f
}

// strydTwoApps: two apps write a field named Power in Watts; the first one in
// file order is the source (XS-4), also while it has no value at the start.
func strydTwoApps() file {
	f := strydRun("stryd-two-apps.fit", 65, 85)
	f.preamble = append(strydHead(0, "stryd-app-one"), appID(1, "other-power-app"), strydDesc(1, 0))
	strydFields(&f, 0, 60, 1199) // the first app starts at 60 s
	for i := range f.samples {
		f.samples[i].dev = append(f.samples[i].dev, devVal{1, 0, proto.Uint16(strydWatts(f.samples[i].t) + 100)})
	}
	return f
}

// strydRedefined: the field is redescribed at 600 s as something else.
func strydRedefined() file {
	f := strydRun("stryd-redefined.fit", 66, 87)
	f.preamble = strydHead(0, "stryd-app-one")
	strydFields(&f, 0, 0, 1199)
	f.extra = []timed{{t: 600, order: orderStart - 1, mesg: desc(0, 0, basetype.Uint16, "Distance", "m", descOpts{})}}
	return f
}

// strydLate: the description comes after the first 300 records, which carry
// no developer field (the encoder refuses undescribed ones).
func strydLate() file {
	f := strydRun("stryd-described-late.fit", 67, 89)
	f.preamble = []proto.Message{appID(0, "stryd-app-one")}
	strydFields(&f, 0, 300, 1199)
	f.extra = []timed{{t: 300, order: orderStart - 1, mesg: strydDesc(0, 0)},
		{t: 300, order: orderStart - 1, mesg: desc(0, 1, basetype.Uint16, "Form Power", "Watts", descOpts{})},
		{t: 300, order: orderStart - 1, mesg: desc(0, 2, basetype.Float32, "Leg Spring Stiffness", "kN/m", descOpts{})}}
	return f
}

// strydChained: a ride without power and Stryd, then a run with Stryd in the
// second FIT sequence.
func strydChained() ([]byte, error) {
	rideStart := baseStart.Add(68 * 24 * time.Hour)
	ride := file{name: "ride", start: rideStart, localOffset: intp(7200), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, 599}},
		laps:     []span{{from: 0, to: 599}},
		timer:    []timerEvent{{0, true}, {599, false}}}
	ride.samples = outdoorRide(91, 599, steady(rand.New(rand.NewPCG(91, 92))), hills)
	for i := range ride.samples {
		ride.samples[i].power = nan
	}
	run := strydRun("run", 68, 93)
	run.start = rideStart.Add(1800 * time.Second)
	run.preamble = strydHead(0, "stryd-app-one")
	strydFields(&run, 0, 0, 1199)
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

// developerHostile carries developer fields that must publish nothing: a
// text, a numeric field named like a place, a latitude and longitude pair as
// semicircles and as degrees, and six fields that look like Stryd's power but
// fail one condition each (text, array, scale 10, offset, native field
// not power, lower-case name, wrong units), one of them only in the session.
func developerHostile() file {
	f := strydRun("developer-hostile.fit", 69, 95)
	f.preamble = []proto.Message{
		appID(0, "SECRET-APP-ID-47"),
		desc(0, 0, basetype.String, "Home address", "Elm Street 5, Springfield", descOpts{}),
		desc(0, 1, basetype.Uint16, "Hometown population", "inhabitants", descOpts{}),
		desc(0, 2, basetype.Sint32, "Start latitude", "semicircles", descOpts{}),
		desc(0, 3, basetype.Sint32, "Start longitude", "semicircles", descOpts{}),
		desc(0, 4, basetype.Float64, "Latitude degrees", "deg", descOpts{}),
		desc(0, 5, basetype.Float64, "Longitude degrees", "deg", descOpts{}),
		desc(0, 6, basetype.String, "Power", "Watts", descOpts{}),
		desc(0, 7, basetype.Uint16, "Power", "Watts", descOpts{}),
		desc(0, 8, basetype.Uint16, "Power", "Watts", descOpts{scale: u8p(10)}),
		desc(0, 9, basetype.Uint16, "Power", "Watts", descOpts{offset: i8p(100)}),
		desc(0, 10, basetype.Uint16, "Power", "Watts", descOpts{nativeMesg: true, nativeField: u8p(4)}),
		desc(0, 11, basetype.Uint16, "power", "Watts", descOpts{}),
		desc(0, 12, basetype.Uint16, "Power", "W", descOpts{}),
		desc(0, 13, basetype.Uint16, "Power", "Watts", descOpts{}), // last: written in the session only
	}
	for i := range f.samples {
		t := f.samples[i].t
		lat, lon := 47.3769+float64(t)*1e-6, 8.5417+float64(t)*1e-6
		f.samples[i].dev = []devVal{
			{0, 0, proto.String("Elm Street 5, Springfield")},
			{0, 1, proto.Uint16(uint16(4000 + t%50))},
			{0, 2, proto.Int32(semicircles(lat))},
			{0, 3, proto.Int32(semicircles(lon))},
			{0, 4, proto.Float64(lat)},
			{0, 5, proto.Float64(lon)},
			{0, 6, proto.String("250")},
			{0, 7, proto.SliceUint16([]uint16{uint16(1111 + t%7), uint16(1112 + t%7)})},
			{0, 8, proto.Uint16(uint16(2500 + t%7))},
			{0, 9, proto.Uint16(uint16(300 + t%7))},
			{0, 10, proto.Uint16(uint16(310 + t%7))},
			{0, 11, proto.Uint16(uint16(320 + t%7))},
			{0, 12, proto.Uint16(uint16(330 + t%7))},
		}
	}
	f.sessionDev = []proto.DeveloperField{{Num: 13, DeveloperDataIndex: 0, Value: proto.Uint16(777)}}
	return f
}
