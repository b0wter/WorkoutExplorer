package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/datafile"
)

// F-65 end to end through the wiring (AC 5, 6, 8): a boundary in a hidden
// start is accepted and published as a lap with no position; the owner API
// refuses a boundary on a lap end with the plain sentence; the page shows
// the added lap without JavaScript.
func TestAddedLapsEndToEnd(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	url := "/api/workouts/" + pub
	start, _ := time.Parse(time.RFC3339, a.startAt(pub))
	at := func(s int) string { return `"` + start.Add(time.Duration(s)*time.Second).Format(time.RFC3339) + `"` }
	if rec := a.owner("PATCH", url, token, `{"hide_start_m":300,"added_lap_boundaries":[`+at(5)+`]}`); rec.Code != 200 {
		t.Fatalf("hidden boundary: %d %s", rec.Code, rec.Body)
	}
	if rec := a.owner("PATCH", url, token, `{"added_lap_boundaries":[`+at(5)+`,`+at(0)+`]}`); rec.Code != 400 ||
		!strings.Contains(rec.Body.String(), `"error":"invalid_settings"`) || !strings.Contains(rec.Body.String(), "There is a lap boundary here already.") {
		t.Fatalf("lap end: %d %s", rec.Code, rec.Body)
	}
	if rec := a.owner("POST", url+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	page := a.page(pub)
	bin, err := os.ReadFile(filepath.Join(a.dir, "public", pub, dataName(t, page)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatal(err)
	}
	lat, ok := stream(f, "latitude")
	if !ok || f.Meta.Laps[0].Trigger != "added" || f.Meta.Laps[0].EndIndex-1 != 5 || f.Meta.Laps[1].StartIndex != 5 {
		t.Fatalf("laps %+v", f.Meta.Laps[:2])
	}
	if lat.Valid[5] || lat.Valid[0] {
		t.Error("the boundary in the hidden start has a public position")
	}
	if !strings.Contains(page, "<td>Added</td>") {
		t.Error("the page has no added lap")
	}
}
