package publish

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
)

func zonesJSON(t *testing.T, f datafile.File) string {
	t.Helper()
	b, err := json.Marshal(f.Meta.Zones)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestSharerZones (F-49 ZN-3, ZN-4; AC-5): the sharer's values replace the
// file's as basis in the public data file, clearing them returns to the
// file's values, and each change rebuilds in place (P-7).
func TestSharerZones(t *testing.T) {
	fx := newFixture(t, "zones-wahoo-ride.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	const file = `{"heart_rate":{"from":[121,141,161,176]},"power":{"from":[139,189,226,264,301,376]}}`
	if got := zonesJSON(t, fx.read(t).file); got != file {
		t.Errorf("file zones %s", got)
	}

	hr, ftp := 185, 255
	z := p
	z.ZonesMaxHeartRate, z.ZonesThresholdPower = &hr, &ftp
	fx.steps = nil
	fx.change(t, p, z)
	if slices.Contains(fx.steps, "remove index.html") {
		t.Errorf("steps %v: a zone change rebuilds in place", fx.steps)
	}
	if got := zonesJSON(t, fx.read(t).file); got != `{"heart_rate":{"basis":185},"power":{"basis":255}}` {
		t.Errorf("sharer zones %s", got)
	}

	only := p
	only.ZonesThresholdPower = &ftp
	fx.change(t, z, only)
	if got := zonesJSON(t, fx.read(t).file); got != `{"heart_rate":{"from":[121,141,161,176]},"power":{"basis":255}}` {
		t.Errorf("one kind cleared %s", got)
	}

	fx.change(t, only, p)
	if got := zonesJSON(t, fx.read(t).file); got != file {
		t.Errorf("cleared %s", got)
	}
}

// TestZonesTrimmed (ZN-3, UP-6): a trim never changes zone values.
func TestZonesTrimmed(t *testing.T) {
	fx := newFixture(t, "zones-ride-target.fit")
	d := draft()
	p := published(d)
	fx.change(t, d, p)
	full := fx.read(t).file
	start, err := time.Parse(time.RFC3339, full.Meta.StartTime)
	if err != nil {
		t.Fatal(err)
	}
	a, b := start.Add(300*time.Second), start.Add(900*time.Second)
	tr := p
	tr.TrimStartAt, tr.TrimEndAt = &a, &b
	fx.change(t, p, tr)
	got := fx.read(t).file
	if got.SampleCount >= full.SampleCount || zonesJSON(t, got) != `{"heart_rate":{"basis":186},"power":{"basis":240}}` {
		t.Errorf("trimmed: %d samples, zones %s", got.SampleCount, zonesJSON(t, got))
	}
}

// TestZonePrivacy (F-49 verification, Privacy): of the Apple-style file's
// zone data only the bounds reach the public data file. Resting and threshold
// heart rate, the calc type and the device's time-in-zone totals do not, and
// session.max_heart_rate (201) is no zone value.
func TestZonePrivacy(t *testing.T) {
	fx := newFixture(t, "zones-apple-run-6.fit")
	d := draft()
	fx.change(t, d, published(d))
	o := fx.read(t)
	if got := zonesJSON(t, o.file); got != `{"heart_rate":{"from":[114,133,152,171]}}` {
		t.Errorf("zones %s", got)
	}
	noMarkers(t, o.bin)
	// The draft data file (the upload's encode of the same parse, F-03 golden).
	golden, err := os.ReadFile("../../testdata/datafile/synthetic/zones-apple-run-6.bin")
	if err != nil {
		t.Fatal(err)
	}
	noMarkers(t, golden)
}

// noMarkers walks every number of a data file's metadata outside "zones":
// none is one of the Apple-style file's privacy markers.
func noMarkers(t *testing.T, bin []byte) {
	t.Helper()
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	markers := map[string]bool{"31": true, "233": true}
	for _, v := range []string{"111", "222", "333", "444", "555", "666"} {
		markers[v+"."+v] = true
		markers[v+v] = true
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if k == "zones" {
					continue // checked exactly above
				}
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case json.Number:
			if markers[x.String()] {
				t.Errorf("public metadata holds the marker %s", x)
			}
		}
	}
	dec := json.NewDecoder(bytes.NewReader(bin[14 : 14+f.MetadataLength]))
	dec.UseNumber()
	var meta any
	if err := dec.Decode(&meta); err != nil {
		t.Fatal(err)
	}
	walk(meta)
}
