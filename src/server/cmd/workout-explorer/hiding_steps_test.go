package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/workout"
)

// TestHidingStepsCorpus (F-55 HR-3, HR-4, ADR-056; F-19 B-1): for every
// synthetic corpus file the upload accepts, the hiding_steps of the owner API,
// looked up at the saved radii with the one combination, mark exactly the
// samples of draft.bin whose positions the public data file has, for several
// radius pairs on the same published workout. It also checks that no owner
// response carries the token and that the log holds neither the token, the
// public_id, the steps nor the radii (E-2, F-12 O-13, HR-10).
func TestHidingStepsCorpus(t *testing.T) {
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
		type view struct {
			HideStartM  int                  `json:"hide_start_m"`
			HideEndM    int                  `json:"hide_end_m"`
			HidingSteps *workout.HidingSteps `json:"hiding_steps"`
		}
		var before view
		if err := json.Unmarshal(owner("GET", url, ""), &before); err != nil || before.HideStartM != 0 || before.HideEndM != 0 {
			t.Fatalf("%s: settings %+v %v", e.Name(), before, err)
		}
		draft, err := datafile.Decode(owner("GET", url+"/draft.bin", ""))
		if err != nil {
			t.Fatal(err)
		}
		n := int(draft.SampleCount)
		lat, hasPos := stream(draft, "latitude")
		h := before.HidingSteps
		if !hasPos {
			indoor++
			if h != nil {
				t.Errorf("%s: indoor: steps %v", e.Name(), h)
			}
		} else if h == nil || len(h.Start) != workout.HideStepCount || len(h.End) != workout.HideStepCount || h.Start[0] != 0 || h.End[0] != n-1 {
			t.Fatalf("%s: steps %v for %d samples", e.Name(), h, n)
		}
		owner("POST", url+"/publish", "")
		for _, rr := range [][2]int{{300, 300}, {0, 500}, {50, 0}, {2000, 2000}} {
			var after view
			if err := json.Unmarshal(owner("PATCH", url, fmt.Sprintf(`{"hide_start_m":%d,"hide_end_m":%d}`, rr[0], rr[1])), &after); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(after.HidingSteps) != fmt.Sprint(h) || after.HideStartM != rr[0] || after.HideEndM != rr[1] {
				t.Errorf("%s: %v: %+v, steps were %v", e.Name(), rr, after, h)
			}
			bin, err := os.ReadFile(filepath.Join(a.dir, "public", pub, dataName(t, a.page(pub))))
			if err != nil {
				t.Fatal(err)
			}
			public, err := datafile.Decode(bin)
			if err != nil {
				t.Fatal(err)
			}
			pubLat, pubPos := stream(public, "latitude")
			if !hasPos {
				if pubPos {
					t.Errorf("%s: indoor: public positions", e.Name())
				}
				continue
			}
			from, to, full := h.Window(rr[0], rr[1])
			switch {
			case full:
				fully++
			case rr == [2]int{300, 300}:
				routes++
			}
			for s := range n {
				public := pubPos && pubLat.Valid[s]
				if want := lat.Valid[s] && !full && s >= from && s <= to; public != want {
					t.Fatalf("%s: %v: sample %d: public %v, want %v (window %d..%d, fully %v)", e.Name(), rr, s, public, want, from, to, full)
				}
			}
		}
	}
	if routes < 10 || indoor < 1 || fully < 1 {
		t.Errorf("corpus covers %d routes, %d fully hidden and %d indoor files", routes, fully, indoor)
	}
	logs := a.logs.String()
	for _, s := range append(secrets, "hiding_steps", "hide_start", "hide_end", "HidingSteps", "HideStartM", "hidden") {
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
