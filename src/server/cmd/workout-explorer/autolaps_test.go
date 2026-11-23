package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
)

// F-66 end to end through the wiring: the setting from the owner API, the
// in-place rebuild both ways, the laps of the public file, the streams
// unchanged (I-3), and the check of a boundary added on a removed boundary.
// ride-lap-triggers has laps button, distance, position, stop, none at
// 0/300/600/900/1200 s; removal merges the laps 1 to 3 into one.
func TestRemoveAutoLapsEndToEnd(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-lap-triggers.fit")
	url := "/api/workouts/" + pub
	start, _ := time.Parse(time.RFC3339, a.startAt(pub))
	at := func(s int) string { return `"` + start.Add(time.Duration(s)*time.Second).Format(time.RFC3339) + `"` }
	patch := func(code int, body string) {
		t.Helper()
		if rec := a.owner("PATCH", url, token, body); rec.Code != code {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
	patch(200, `{"hide_start_m":300,"hide_end_m":300}`)
	if rec := a.owner("POST", url+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	load := func() datafile.File {
		t.Helper()
		bin, err := os.ReadFile(filepath.Join(a.dir, "public", pub, dataName(t, a.page(pub))))
		if err != nil {
			t.Fatal(err)
		}
		f, err := datafile.Decode(bin)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	off := load()
	if len(off.Meta.Laps) != 5 {
		t.Fatalf("laps %d", len(off.Meta.Laps))
	}
	if rec := a.owner("GET", url, token, ""); !strings.Contains(rec.Body.String(), `"remove_auto_laps":false`) {
		t.Errorf("GET: %s", rec.Body)
	}

	patch(200, `{"remove_auto_laps":true}`)
	if rec := a.owner("GET", url, token, ""); !strings.Contains(rec.Body.String(), `"remove_auto_laps":true`) {
		t.Errorf("GET: %s", rec.Body)
	}
	on := load()
	if l := on.Meta.Laps; len(l) != 3 || l[1].StartIndex != 300 || l[1].EndIndex != 1201 {
		t.Fatalf("laps %+v, want 3 with the middle one over 300..1200", l)
	}
	if len(on.Streams) != len(off.Streams) {
		t.Fatal("streams differ in number")
	}
	for i := range on.Streams {
		if !reflect.DeepEqual(on.Streams[i].Ints, off.Streams[i].Ints) || !reflect.DeepEqual(on.Streams[i].Valid, off.Streams[i].Valid) {
			t.Errorf("stream %s changed with the setting", on.Streams[i].Descriptor.Name)
		}
	}
	if lat, _ := stream(on, "latitude"); lat.Valid[0] || lat.Valid[1499] {
		t.Error("hidden ends have a position")
	}

	// Add lap on a removed boundary: accepted while on, refused when off.
	patch(200, `{"added_lap_boundaries":[`+at(600)+`]}`)
	if l := load().Meta.Laps; len(l) != 4 || l[1].Trigger != "added" {
		t.Errorf("added lap: %+v", l)
	}
	patch(400, `{"remove_auto_laps":false,"added_lap_boundaries":[`+at(600)+`,`+at(450)+`,`+at(300)+`]}`)
	patch(200, `{"remove_auto_laps":false}`) // the stored 600 s entry is skipped by the build
	if l := load().Meta.Laps; len(l) != 5 {
		t.Errorf("restore: %d laps", len(l))
	}

	for _, body := range []string{`{"remove_auto_laps":null}`, `{"remove_auto_laps":1}`, `{"remove_auto_laps":"true"}`} {
		patch(400, body)
	}
	if rec := a.owner("PATCH", url, "x"+token[1:], `{"remove_auto_laps":true}`); rec.Code != 404 {
		t.Errorf("wrong token: %d", rec.Code)
	}
}

// QA-66-r1-2: the setting changes laps only, at radii 0, 300 and 2,000, with
// and without a trim; hidden ends keep no position either way (I-3, UP-9).
func TestRemoveAutoLapsPrivacyMatrix(t *testing.T) {
	for _, radius := range []int{0, 300, 2000} {
		for _, trim := range []bool{false, true} {
			a := startApp(t, settings{})
			pub, token, _ := a.upload("ride-lap-triggers.fit")
			url := "/api/workouts/" + pub
			start, _ := time.Parse(time.RFC3339, a.startAt(pub))
			body := `{"hide_start_m":` + strconv.Itoa(radius) + `,"hide_end_m":` + strconv.Itoa(radius)
			if trim {
				at := func(s int) string { return `"` + start.Add(time.Duration(s)*time.Second).Format(time.RFC3339) + `"` }
				body += `,"trim_start_at":` + at(100) + `,"trim_end_at":` + at(1300)
			}
			name := "r" + strconv.Itoa(radius) + " trim " + strconv.FormatBool(trim)
			if rec := a.owner("PATCH", url, token, body+"}"); rec.Code != 200 {
				t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
			}
			if rec := a.owner("POST", url+"/publish", token, ""); rec.Code != 200 {
				t.Fatalf("%s: publish %d", name, rec.Code)
			}
			load := func() datafile.File {
				bin, err := os.ReadFile(filepath.Join(a.dir, "public", pub, dataName(t, a.page(pub))))
				if err != nil {
					t.Fatal(err)
				}
				f, err := datafile.Decode(bin)
				if err != nil {
					t.Fatal(err)
				}
				return f
			}
			off := load()
			if rec := a.owner("PATCH", url, token, `{"remove_auto_laps":true}`); rec.Code != 200 {
				t.Fatal(rec.Code)
			}
			on := load()
			if len(on.Streams) != len(off.Streams) {
				t.Fatalf("%s: stream count", name)
			}
			for i := range on.Streams {
				if !reflect.DeepEqual(on.Streams[i].Ints, off.Streams[i].Ints) || !reflect.DeepEqual(on.Streams[i].Valid, off.Streams[i].Valid) {
					t.Errorf("%s: stream %s changed with the setting", name, on.Streams[i].Descriptor.Name)
				}
			}
			if len(on.Meta.Laps) >= len(off.Meta.Laps) {
				t.Errorf("%s: laps %d -> %d, want fewer", name, len(off.Meta.Laps), len(on.Meta.Laps))
			}
			if lat, ok := stream(on, "latitude"); radius > 0 && ok && (lat.Valid[0] || lat.Valid[len(lat.Valid)-1]) {
				t.Errorf("%s: hidden ends have a position", name)
			}
		}
	}
}
