package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workout-explorer/internal/datafile"
)

// TestHiddenRangesCorpus (F-19 B-1, ADR-030): for every synthetic corpus file
// the upload accepts, the stored hide_ends_ranges mark exactly the samples of
// draft.bin whose positions the public data file lacks once hiding is on.
// It also checks that no owner response carries the token and that the log
// holds neither the token, the public_id nor the ranges (E-2, F-12 O-13).
func TestHiddenRangesCorpus(t *testing.T) {
	entries, err := os.ReadDir(synthetic)
	if err != nil {
		t.Fatal(err)
	}
	a := startApp(t, settings{})
	var routes, indoor, fully int
	var secrets []string
	for i, e := range entries {
		if !strings.HasSuffix(e.Name(), ".fit") {
			continue
		}
		up := a.post(readFile(t, e.Name()), fmt.Sprintf("198.51.100.%d:1000", i+1), "", "", e.Name())
		if up.Code != 201 {
			continue // rejected files have no row
		}
		var links map[string]string
		if err := json.Unmarshal(up.Body.Bytes(), &links); err != nil {
			t.Fatal(err)
		}
		pub := strings.TrimPrefix(links["view_url"], "/w/")
		_, token, _ := strings.Cut(links["edit_url"], "#")
		secrets = append(secrets, pub, token)
		url := "/api/workouts/" + pub
		owner := func(method, path, body string) []byte {
			t.Helper()
			rec := a.owner(method, path, token, body)
			if rec.Code != 200 {
				t.Fatalf("%s: %s %s: %d %s", e.Name(), method, path, rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), token) || strings.Contains(fmt.Sprint(rec.Header()), token) {
				t.Fatalf("%s: %s %s: token in the response", e.Name(), method, path)
			}
			return rec.Body.Bytes()
		}
		var before, after struct {
			HideEndsRanges [][2]int `json:"hide_ends_ranges"`
		}
		if err := json.Unmarshal(owner("GET", url, ""), &before); err != nil || before.HideEndsRanges == nil {
			t.Fatalf("%s: settings %v", e.Name(), err)
		}
		draft, err := datafile.Decode(owner("GET", url+"/draft.bin", ""))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(owner("PATCH", url, `{"hide_ends":true}`), &after); err != nil {
			t.Fatal(err)
		}
		owner("POST", url+"/publish", "")
		bin, err := os.ReadFile(filepath.Join(a.dir, "public", pub, dataName(t, a.page(pub))))
		if err != nil {
			t.Fatal(err)
		}
		public, err := datafile.Decode(bin)
		if err != nil {
			t.Fatal(err)
		}
		// The field shows what hiding would remove, whatever hide_ends says.
		r := before.HideEndsRanges
		if fmt.Sprint(after.HideEndsRanges) != fmt.Sprint(r) {
			t.Errorf("%s: ranges %v with hiding on, %v with it off", e.Name(), after.HideEndsRanges, r)
		}
		n := int(draft.SampleCount)
		lat, hasPos := stream(draft, "latitude")
		pubLat, pubPos := stream(public, "latitude")
		switch {
		case !hasPos:
			indoor++
			if len(r) != 0 || pubPos {
				t.Errorf("%s: indoor: ranges %v, public positions %v", e.Name(), r, pubPos)
			}
			continue
		case len(r) == 1 && r[0] == [2]int{0, n - 1}:
			fully++
		case len(r) == 2:
			routes++
		default:
			t.Errorf("%s: ranges %v for %d samples", e.Name(), r, n)
			continue
		}
		for s := range n {
			in := false
			for _, p := range r {
				in = in || (p[0] <= s && s <= p[1])
			}
			public := pubPos && pubLat.Valid[s]
			if public != (lat.Valid[s] && !in) {
				t.Fatalf("%s: sample %d: public %v, draft %v, in a range %v (%v)", e.Name(), s, public, lat.Valid[s], in, r)
			}
		}
	}
	if routes < 10 || indoor < 1 {
		t.Errorf("corpus covers %d routes, %d fully hidden and %d indoor files", routes, fully, indoor)
	}
	logs := a.logs.String()
	for _, s := range append(secrets, "hide_ends_ranges", "hidden") {
		if strings.Contains(logs, s) {
			t.Errorf("log contains %q", s)
		}
	}
}

func stream(f datafile.File, name string) (datafile.Stream, bool) {
	for _, s := range f.Streams {
		if s.Descriptor.Name == name {
			return s, true
		}
	}
	return datafile.Stream{}, false
}
