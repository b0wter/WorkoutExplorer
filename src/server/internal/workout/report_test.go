package workout

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func sampleWorkout() Workout {
	return Normalise(Recording{
		FileType:    "activity",
		Device:      Device{Manufacturer: "garmin", ProductID: 3122, ProductName: "Edge"},
		LocalOffset: func() *int64 { v := int64(7200); return &v }(),
		Sessions:    []RawSession{{Sport: "cycling", SubSport: "road", Start: at(0), End: at(20), Totals: Totals{Distance: f64(100)}}},
		Laps:        []RawLap{{Start: at(0), End: at(20)}},
		Timer:       []RawTimerEvent{{At: at(0), Kind: TimerStart}, {At: at(20), Kind: TimerStop}},
		Records: recs([]int{0, 1, 2, 10, 11, 20}, func(i int, r *RawRecord) {
			r.HeartRate = float64(120 + i)
			r.Lat, r.Lon = 47+float64(i)*0.001, 8
			if i == 1 {
				r.HeartRate = nan
			}
		}),
	})
}

func TestReportJSONDeterministic(t *testing.T) {
	a, err := ReportJSON(sampleWorkout())
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReportJSON(sampleWorkout())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("two reports of the same workout differ")
	}
	if !bytes.HasSuffix(a, []byte("}\n")) || !bytes.HasPrefix(a, []byte("{\n  \"report_version\": 2,")) {
		t.Errorf("report not indented with two spaces and a trailing newline:\n%s", a)
	}
}

func TestReportContents(t *testing.T) {
	b, err := ReportJSON(sampleWorkout())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}
	streams := m["streams"].(map[string]any)
	for _, absent := range []string{"altitude", "speed", "cadence", "power", "temperature"} {
		if _, ok := streams[absent]; ok {
			t.Errorf("absent stream %q in report", absent)
		}
	}
	for _, present := range []string{"latitude", "longitude", "distance", "heart_rate"} {
		if _, ok := streams[present]; !ok {
			t.Errorf("present stream %q missing from report", present)
		}
	}
	hr := streams["heart_rate"].(map[string]any)
	if hr["missing"].(float64) != 1 {
		t.Errorf("heart_rate missing = %v, want 1", hr["missing"])
	}
	// Keys in the documented order.
	keys := []string{`"report_version"`, `"file_type"`, `"device"`, `"sport"`, `"start_time"`, `"time_zone"`,
		`"sessions"`, `"laps"`, `"timer_events"`, `"samples"`, `"streams"`, `"derived"`, `"summary"`}
	last := -1
	for _, k := range keys {
		i := bytes.Index(b, []byte(k))
		if i < 0 || i < last {
			t.Errorf("key %s missing or out of order", k)
		}
		last = i
	}
	if !strings.Contains(string(b), `"start_time": "2026-06-01T06:00:00Z"`) {
		t.Errorf("start_time not RFC 3339 UTC")
	}
	samples := m["samples"].(map[string]any)
	gaps := samples["gaps"].(map[string]any)
	if gaps["count"].(float64) != 2 || gaps["total_seconds"].(float64) != 17 || gaps["longest_seconds"].(float64) != 9 {
		t.Errorf("gaps = %v, want count 2, total 17, longest 9", gaps)
	}
}

func TestReportDigestNaN(t *testing.T) {
	r := reportStream([]float64{1, nan, 2.5}, 2)
	h := sha256.Sum256([]byte("1.00\n-\n2.50\n"))
	if want := hex.EncodeToString(h[:])[:16]; r.Digest != want {
		t.Errorf("digest = %s, want %s", r.Digest, want)
	}
	if r.Present != 2 || r.Missing != 1 || r.Mean != "1.75" || r.First != "1.00" || r.Last != "2.50" {
		t.Errorf("stats = %+v", r)
	}
}

func TestReportNegativeZero(t *testing.T) {
	tests := []struct {
		v    float64
		dec  int
		want string
	}{
		{-0.0001, 2, "0.00"},
		{negZero(), 3, "0.000"},
		{-0.004, 2, "0.00"},
		{-0.005, 3, "-0.005"},
		{-1, 0, "-1"},
		{12.3456, 2, "12.35"},
	}
	for _, tt := range tests {
		if got := string(number(tt.v, tt.dec)); got != tt.want {
			t.Errorf("number(%v, %d) = %q, want %q", tt.v, tt.dec, got, tt.want)
		}
	}
	if got := string(appendValue(nil, nan, 2)); got != "-" {
		t.Errorf("NaN formatted as %q, want -", got)
	}
}

func negZero() float64 {
	z := 0.0
	return -z
}

func TestReportEmptyWorkout(t *testing.T) {
	b, err := ReportJSON(Normalise(Recording{}))
	if err != nil {
		t.Fatal(err)
	}
	// E-7: arrays and objects are always written; absent scalars are omitted.
	for _, present := range []string{`"device": {}`, `"time_zone": {`, `"sessions": []`, `"laps": []`, `"timer_events": []`,
		`"count": 0`, `"filled": 0`, `"total_seconds": 0`, `"longest_seconds": 0`, `"offsets_digest"`,
		`"streams": {}`, `"derived": {}`, `"summary": {}`} {
		if !bytes.Contains(b, []byte(present)) {
			t.Errorf("empty workout report lacks %s:\n%s", present, b)
		}
	}
	for _, absent := range []string{`"start_time"`, `"last_offset"`, `"file_type"`, `"sport"`} {
		if bytes.Contains(b, []byte(absent)) {
			t.Errorf("empty workout report contains %s:\n%s", absent, b)
		}
	}
}

func TestReportOffsetsDigestIntegers(t *testing.T) {
	r := reportSamples(Samples{Offset: []int32{0, 1, 7}})
	h := sha256.Sum256([]byte("0\n1\n7\n"))
	if want := hex.EncodeToString(h[:])[:16]; r.OffsetsDigest != want {
		t.Errorf("offsets digest = %s, want %s (integers, E-6)", r.OffsetsDigest, want)
	}
}
