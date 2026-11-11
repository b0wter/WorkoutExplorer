package workout

import (
	"strconv"
	"strings"
	"testing"
)

// F-43 DI-4, DI-6 and F-44 PS-3.

func TestDeviceName(t *testing.T) {
	for _, c := range []struct{ brand, model, want string }{
		{"Wahoo Fitness", "ELEMNT BOLT", "Wahoo Fitness ELEMNT BOLT"},
		{"Garmin", "Edge 530", "Garmin Edge 530"},
		{"Garmin", "GARMIN Edge 530", "GARMIN Edge 530"},
		{"Wahoo Fitness", "WAHOO ELEMNT", "WAHOO ELEMNT"},
		{"Polar Electro", "Polaris X", "Polar Electro Polaris X"}, // a word, not a prefix
		{"", "corpusgen", "corpusgen"},
		{"Wahoo Fitness", "", "Wahoo Fitness"},
		{"", "", ""},
	} {
		if got := (Device{Brand: c.brand, Model: c.model}).Name(); got != c.want {
			t.Errorf("%q + %q = %q, want %q", c.brand, c.model, got, c.want)
		}
	}
}

func TestSensorCodes(t *testing.T) {
	if got := Sensors(0).Codes(); got == nil || len(got) != 0 {
		t.Errorf("none: %#v", got)
	}
	all := SensorRadar | SensorFootPod | SensorSpeedCadence | SensorCadence | SensorSpeed | SensorPower | SensorHeartRate
	want := "heart_rate_monitor,power_meter,speed_sensor,cadence_sensor,speed_cadence_sensor,foot_pod,radar"
	if got := strings.Join(all.Codes(), ","); got != want {
		t.Errorf("all: %s", got)
	}
}

func TestPoolSummary(t *testing.T) {
	pool := func(sub string, length float64, statute bool, lengths uint16) Session {
		return Session{Sport: "swimming", SubSport: sub, Pool: Pool{Length: length, Statute: statute, Lengths: lengths}}
	}
	lap := "lap_swimming"
	str := func(p *float64) string {
		if p == nil {
			return "-"
		}
		return strconv.FormatFloat(*p, 'f', -1, 64)
	}
	for _, c := range []struct {
		name     string
		sessions []Session
		m, yd, n string
	}{
		{"25 m, 64 lengths", []Session{pool(lap, 25, false, 64)}, "25", "-", "64"},
		{"33.3 m rounds to 33", []Session{pool(lap, 33.3, false, 10)}, "33", "-", "10"},
		{"33.5 m rounds half away from zero", []Session{pool(lap, 33.5, false, 10)}, "34", "-", "10"},
		{"25 yd", []Session{pool(lap, 22.86, true, 40)}, "-", "25", "40"},
		{"pool length only", []Session{pool(lap, 25, false, 0)}, "25", "-", "-"},
		{"neither", []Session{pool(lap, 0, false, 0)}, "-", "-", "-"},
		{"lengths only", []Session{pool(lap, 0, false, 12)}, "-", "-", "12"},
		{"open water ignored", []Session{pool("open_water", 25, false, 30)}, "-", "-", "-"},
		{"no sessions", nil, "-", "-", "-"},
		{"two pools, same length: summed", []Session{pool(lap, 25, false, 40), pool("open_water", 50, false, 9), pool(lap, 25, false, 24)}, "25", "-", "64"},
		{"two pools, different lengths", []Session{pool(lap, 25, false, 40), pool(lap, 50, false, 24)}, "-", "-", "64"},
		{"25 m and 25 yd differ", []Session{pool(lap, 25, false, 40), pool(lap, 22.86, true, 24)}, "-", "-", "64"},
		{"one without lengths hides Lengths (OQ-153)", []Session{pool(lap, 25, false, 40), pool(lap, 25, false, 0)}, "25", "-", "-"},
		{"one without length hides Pool", []Session{pool(lap, 25, false, 40), pool(lap, 0, false, 24)}, "-", "-", "64"},
	} {
		m, yd, n := poolSummary(c.sessions)
		if str(m) != c.m || str(yd) != c.yd || str(n) != c.n {
			t.Errorf("%s: %s m, %s yd, %s lengths; want %s, %s, %s", c.name, str(m), str(yd), str(n), c.m, c.yd, c.n)
		}
	}
}
