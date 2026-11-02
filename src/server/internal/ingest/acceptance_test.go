package ingest

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"workout-explorer/internal/workout"
)

// parseCorpus parses testdata/corpus/synthetic/<name>.
func parseCorpus(t *testing.T, name string) workout.Workout {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(corpusDir, "synthetic", name))
	if err != nil {
		t.Fatal(err)
	}
	w, err := testParser.Parse(context.Background(), data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return w
}

func approx(t *testing.T, name string, got *float64, want, eps float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > eps {
		t.Errorf("%s = %v, want %v ± %v", name, deref(got), want, eps)
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// TestCorpusAcceptance checks the F-02 acceptance criteria on the corpus
// files that cover them, independent of the golden reports.
func TestCorpusAcceptance(t *testing.T) {
	t.Run("AC5 enhanced fields preferred", func(t *testing.T) {
		// enhanced-and-legacy.fit, segment 1 (offsets 0-59): both fields,
		// altitude 410 / enhanced 420 m, speed 6.999 / enhanced 7.999 m/s;
		// segment 2 (60-119): legacy only, 430 m and 5.5 m/s; segment 3
		// (120-179): enhanced only, 440 m and 6.5 m/s.
		w := parseCorpus(t, "enhanced-and-legacy.fit")
		for _, c := range []struct {
			i          int
			alt, speed float64
		}{{0, 420, 7.999}, {59, 420, 7.999}, {60, 430, 5.5}, {119, 430, 5.5}, {120, 440, 6.5}, {179, 440, 6.5}} {
			if a, v := w.Samples.Altitude[c.i], w.Samples.Speed[c.i]; a != c.alt || math.Abs(v-c.speed) > 1e-9 {
				t.Errorf("sample %d: altitude %v, speed %v; want %v, %v", c.i, a, v, c.alt, c.speed)
			}
		}
		if s := w.Sessions[0].Totals.AvgSpeed; s == nil || *s < 6.5 {
			t.Errorf("session avg speed = %v, want enhanced_avg_speed (legacy is 1 m/s lower)", deref(s))
		}
	})

	t.Run("AC6 timer stop excluded", func(t *testing.T) {
		w := parseCorpus(t, "timer-pause.fit")
		approx(t, "moving time", w.Derived.MovingTime, 1200, 0)
		if w.Derived.MovingTimeSource != "timer" {
			t.Errorf("source = %q, want timer", w.Derived.MovingTimeSource)
		}
	})

	t.Run("AC6 cycling below 1 km/h not moving", func(t *testing.T) {
		w := parseCorpus(t, "cycling-no-timer-slow.fit")
		approx(t, "moving time", w.Derived.MovingTime, 599, 0)
		if w.Derived.MovingTimeSource != "speed" {
			t.Errorf("source = %q, want speed", w.Derived.MovingTimeSource)
		}
	})

	t.Run("AC7 flat with noise", func(t *testing.T) {
		w := parseCorpus(t, "flat-noise.fit")
		approx(t, "elevation gain", w.Derived.ElevationGain, 0, 0)
		approx(t, "summary ascent (no device total)", w.Summary.Ascent, 0, 0)
	})

	t.Run("AC7 clean 100 m climb", func(t *testing.T) {
		w := parseCorpus(t, "climb-100m.fit")
		approx(t, "elevation gain", w.Derived.ElevationGain, 100, 2)
		approx(t, "summary ascent (no device total)", w.Summary.Ascent, 100, 2)
	})

	t.Run("AC8 distance from positions", func(t *testing.T) {
		w := parseCorpus(t, "no-distance-with-gps.fit")
		if w.Samples.Distance == nil {
			t.Fatal("no distance stream")
		}
		approx(t, "derived distance vs session total", w.Derived.Distance, *w.Sessions[0].Totals.Distance, 1)
	})

	t.Run("AC8 no distance and no positions", func(t *testing.T) {
		w := parseCorpus(t, "indoor-no-distance-no-gps.fit")
		if w.Samples.Distance != nil || w.Samples.Lat != nil {
			t.Errorf("distance or position stream present")
		}
		if w.TimeZone.Known {
			t.Errorf("file without activity message: time zone known")
		}
	})

	t.Run("AC9 smart recording", func(t *testing.T) {
		w := parseCorpus(t, "smart-recording.fit")
		if w.Samples.Filled != 5 { // 2 for the 3 s gap, 3 for the 4 s gap
			t.Errorf("filled = %d, want 5", w.Samples.Filled)
		}
		gaps := map[int32]int{}
		for i := 1; i < len(w.Samples.Offset); i++ {
			if d := w.Samples.Offset[i] - w.Samples.Offset[i-1]; d > 1 {
				gaps[d]++
			}
		}
		if len(gaps) != 2 || gaps[5] != 1 || gaps[30] != 1 {
			t.Errorf("gaps after filling = %v, want one of 5 s and one of 30 s", gaps)
		}
	})

	t.Run("AC10 unknown messages and fields", func(t *testing.T) {
		w := parseCorpus(t, "unknown-messages.fit")
		if len(w.Samples.Offset) != 301 {
			t.Errorf("samples = %d, want 301", len(w.Samples.Offset))
		}
	})

	t.Run("AC11 multi-session", func(t *testing.T) {
		w := parseCorpus(t, "brick-run-bike.fit")
		if w.Sport != "multisport" || len(w.Sessions) != 2 ||
			w.Sessions[0].Sport != "running" || w.Sessions[1].Sport != "cycling" ||
			w.Sessions[0].Start >= w.Sessions[1].Start {
			t.Errorf("sport %q, sessions %+v", w.Sport, w.Sessions)
		}
		approx(t, "running avg cadence in spm", w.Sessions[0].Totals.AvgCadence, 171, 0)
		if got := w.Samples.Cadence[0]; got != 171 {
			t.Errorf("running cadence = %v, want 171", got)
		}
	})

	t.Run("12-hour ride", func(t *testing.T) {
		w := parseCorpus(t, "ride-12h.fit")
		if n := len(w.Samples.Offset); n != 43200 {
			t.Errorf("samples = %d, want 43200", n)
		}
		if len(w.Laps) != 24 {
			t.Errorf("laps = %d, want 24", len(w.Laps))
		}
	})

	t.Run("pause of hours", func(t *testing.T) {
		w := parseCorpus(t, "pause-hours.fit")
		approx(t, "moving time", w.Derived.MovingTime, 3600, 0)
		approx(t, "elapsed time", w.Summary.ElapsedTime, 14400, 0)
	})

	t.Run("GPS dropout", func(t *testing.T) {
		w := parseCorpus(t, "gps-dropout.fit")
		missing := 0
		for _, v := range w.Samples.Lat {
			if math.IsNaN(v) {
				missing++
			}
		}
		if missing != 300 {
			t.Errorf("samples without position = %d, want 300", missing)
		}
	})

	t.Run("pool swim", func(t *testing.T) {
		w := parseCorpus(t, "pool-swim.fit")
		if w.Samples.Lat != nil || w.Sport != "swimming" {
			t.Errorf("sport %q, positions present %v", w.Sport, w.Samples.Lat != nil)
		}
	})
}
