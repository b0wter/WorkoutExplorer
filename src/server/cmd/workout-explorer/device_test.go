package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestDeviceIdentifiersNeverPublished (F-43 AC 5, critical lane): the sensors
// of identifying-data.fit are shown by type, while their serial numbers, ANT+
// device numbers, descriptors and product names reach neither draft.bin, the
// public data file, the viewing page nor the owner API. original.fit keeps
// descriptors and product names by design (ADR-023) and is never public.
func TestDeviceIdentifiersNeverPublished(t *testing.T) {
	a := startApp(t, settings{})
	pub, token := a.published("identifying-data.fit")

	markers := [][]byte{
		[]byte("Marker Descriptor"), []byte("Marker Sensor Product"), []byte("Descriptor Chest"),
		[]byte("Descriptor Left Crank"), []byte("Product HR"), []byte("Product PM"),
	}
	for _, n := range []uint32{3_735_928_559, 2_882_400_001, 2_882_400_002, 2_882_400_003} {
		markers = append(markers, []byte(strconv.FormatUint(uint64(n), 10)), binary.LittleEndian.AppendUint32(nil, n))
	}
	for _, n := range []int{54_321, 12_345} { // the 2-byte forms would match random bytes by chance
		markers = append(markers, []byte(strconv.Itoa(n)))
	}

	searched := map[string][]byte{}
	for p, b := range a.dataFiles() {
		switch name := filepath.Base(p); {
		case name == "draft.bin", strings.HasPrefix(name, "data.") && strings.HasSuffix(name, ".bin"), name == "index.html":
			searched[name] = b
		}
	}
	rec := a.owner("GET", "/api/workouts/"+pub, token, "")
	if rec.Code != 200 {
		t.Fatalf("owner GET: %d %s", rec.Code, rec.Body)
	}
	searched["owner API"] = rec.Body.Bytes()
	if len(searched) != 4 {
		t.Fatalf("searched %d sources, want draft.bin, data file, index.html and the owner API: %v", len(searched), keys(searched))
	}
	for name, b := range searched {
		for _, m := range markers {
			if bytes.Contains(b, m) {
				t.Errorf("%s contains %q", name, m)
			}
		}
	}

	// The shown values are there: the line on the page, name and codes in the data file.
	if !bytes.Contains(searched["index.html"], []byte(`<p class="device">Recorded with corpusgen · heart-rate monitor, power meter</p>`)) {
		t.Errorf("viewing page lacks the device line")
	}
	if !bytes.Contains(searched["draft.bin"], []byte(`"device":{"name":"corpusgen","sensors":["heart_rate_monitor","power_meter"]}`)) {
		t.Errorf("draft.bin lacks the device")
	}
	// original.fit still holds the descriptor: the search above would find it.
	orig, err := os.ReadFile(filepath.Join(a.dir, "private", a.rows()[0]["id"].(string), "original.fit"))
	if err != nil || !bytes.Contains(orig, []byte("Marker Descriptor Chest")) {
		t.Errorf("original.fit lacks the descriptor (err %v): the marker would prove nothing", err)
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
