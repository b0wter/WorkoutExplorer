package page

import (
	"testing"
	"time"

	"workout-explorer/internal/datafile"
)

func sess(sports ...string) []datafile.Session {
	var out []datafile.Session
	for i, s := range sports {
		out = append(out, datafile.Session{Sport: s, Start: int32(i * 100)})
	}
	return out
}

func TestSuggest(t *testing.T) {
	tz := func(off int32) datafile.TimeZone { return datafile.TimeZone{Known: true, OffsetSeconds: off} }
	cases := []struct {
		name string
		m    datafile.Meta
		town string
		want string
	}{
		{"morning", datafile.Meta{Sport: "cycling", StartTime: "2026-10-09T07:30:00Z", TimeZone: tz(0)}, "Köln", "Morning ride near Köln"},
		{"offset wins over UTC", datafile.Meta{Sport: "running", StartTime: "2026-10-09T22:30:00Z", TimeZone: tz(3600 * 2)}, "Köln", "Night run near Köln"},
		{"offset crosses boundary", datafile.Meta{Sport: "running", StartTime: "2026-10-09T04:30:00Z", TimeZone: tz(1800)}, "", "Morning run"},
		{"no offset is UTC", datafile.Meta{Sport: "swimming", StartTime: "2026-10-09T12:00:00Z"}, "", "Afternoon swim"},
		{"no start time", datafile.Meta{Sport: "cycling", StartTime: "0001-01-01T00:00:00Z"}, "Köln", "Ride near Köln"},
		{"empty start time", datafile.Meta{Sport: "cycling"}, "", "Ride"},
		{"no sport", datafile.Meta{StartTime: "2026-10-09T18:00:00Z"}, "", "Evening workout"},
		{"triathlon", datafile.Meta{Sport: "multisport", StartTime: "2026-10-09T08:00:00Z", Sessions: sess("swimming", "transition", "cycling", "transition", "running")}, "", "Morning triathlon"},
		{"brick is multisport", datafile.Meta{Sport: "multisport", StartTime: "2026-10-09T08:00:00Z", Sessions: sess("running", "transition", "cycling", "transition", "running")}, "", "Morning multisport"},
		{"tri in wrong order", datafile.Meta{Sport: "multisport", StartTime: "2026-10-09T08:00:00Z", Sessions: sess("cycling", "swimming", "running")}, "", "Morning multisport"},
	}
	for _, c := range cases {
		if got := Suggest(c.m, c.town); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSuggestTimeBoundaries(t *testing.T) {
	for _, c := range []struct{ hhmm, want string }{
		{"04:59", "Night"}, {"05:00", "Morning"}, {"11:59", "Morning"}, {"12:00", "Afternoon"},
		{"16:59", "Afternoon"}, {"17:00", "Evening"}, {"20:59", "Evening"}, {"21:00", "Night"},
	} {
		m := datafile.Meta{Sport: "cycling", StartTime: "2026-10-09T" + c.hhmm + ":00Z"}
		if got := Suggest(m, ""); got != c.want+" ride" {
			t.Errorf("%s: %q", c.hhmm, got)
		}
	}
}

func TestBuildUsesSuggestionOnlyForEmptyTitle(t *testing.T) {
	f := datafile.File{Meta: datafile.Meta{Sport: "cycling", StartTime: "2026-10-09T07:30:00Z"}}
	s := Settings{Suggested: "Morning ride near Köln", ExpiresAt: time.Unix(0, 0)}
	if vm := Build(f, s); vm.Header.Title != s.Suggested || vm.Card.Title != s.Suggested || vm.Page.Title != s.Suggested {
		t.Errorf("empty title: %q", vm.Header.Title)
	}
	s.Title = "Mine"
	if vm := Build(f, s); vm.Header.Title != "Mine" {
		t.Errorf("typed title lost: %q", vm.Header.Title)
	}
	s.Title, s.Suggested = "  ", ""
	if vm := Build(f, s); vm.Header.Title != "Ride, 9 Oct 2026" {
		t.Errorf("fallback: %q", vm.Header.Title)
	}
}

// OQ-163: the operator's delete confirmation keeps sport and date.
func TestRowTitleKeepsSportAndDate(t *testing.T) {
	start := time.Date(2026, 10, 9, 7, 30, 0, 0, time.UTC)
	if got, _ := RowTitle("", "cycling", &start, nil); got != "Ride, 9 Oct 2026" {
		t.Errorf("got %q", got)
	}
}
