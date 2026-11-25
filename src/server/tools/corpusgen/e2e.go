package main

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/muktihari/fit/profile/typedef"
)

// writeE2E writes the files only the browser suite (src/web/e2e) needs: they are not part of the
// parser corpus, so they stay out of testdata/corpus/synthetic and its goldens.
func writeE2E(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, f := range []file{intervalRun(), spikeRide("spike-ride.fit", 1), spikeRide("glitch-ride.fit", 6)} {
		b, err := encode(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outDir, f.name), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// intervalRun: warm-up 2 km, 6 x 1 km (3:40 to 4:10 /km) with 5 recovery laps of 400 m, cool-down;
// every segment is a lap (the F-71 comparison chart needs uneven laps).
func intervalRun() file {
	type seg struct{ m, v float64 }
	segs := []seg{{2000, 1000.0 / 330}}
	for i, p := range []float64{220, 230, 240, 250, 235, 225} {
		segs = append(segs, seg{1000, 1000 / p})
		if i < 5 {
			segs = append(segs, seg{400, 1000.0 / 390})
		}
	}
	segs = append(segs, seg{1500, 1000.0 / 340})
	var speeds []float64
	var laps []span
	t := 0
	for _, s := range segs {
		n := int(s.m / s.v)
		laps = append(laps, span{from: t, to: t + n})
		for i := 0; i < n; i++ {
			speeds = append(speeds, s.v)
		}
		t += n
	}
	end := t - 1
	laps[len(laps)-1].to = end
	f := file{name: "interval-run.fit", start: baseStart.Add(40 * 24 * time.Hour), localOffset: intp(3600), enhanced: true, sportHints: true,
		sessions: []span{{typedef.SportRunning, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}, laps: laps}
	f.samples = outdoorRide(77, end, func(t int) float64 { return speeds[min(t, len(speeds)-1)] }, hills)
	for i := range f.samples {
		f.samples[i].cad = 88
		f.samples[i].power = nan
	}
	return f
}

// spikeRide: 30 min at about 26 km/h (7.2 m/s) with one glitch of secs seconds at 85 km/h (23.6 m/s)
// at t = 900 s. 1 s is removed by the F-74 filter (max speed back to about 27 km/h), 6 s is kept
// (AP-26 clips the chart axis, the readout still shows 85 km/h).
func spikeRide(name string, secs int) file {
	const end = 1799
	f := file{name: name, start: baseStart.Add(41 * 24 * time.Hour), localOffset: intp(3600), enhanced: true,
		sessions: []span{{typedef.SportCycling, typedef.SubSportRoad, 0, end}},
		timer:    []timerEvent{{0, true}, {end, false}}}
	for from := 0; from < end; from += 600 {
		f.laps = append(f.laps, span{from: from, to: min(from+600, end)})
	}
	rng := rand.New(rand.NewPCG(91, 92))
	f.samples = outdoorRide(91, end, func(t int) float64 {
		if t >= 900 && t < 900+secs {
			return 23.6
		}
		return 7.2 + rng.Float64()*0.4 - 0.2
	}, hills)
	return f
}
