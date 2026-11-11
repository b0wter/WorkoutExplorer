// Package page builds the page view model of the activity page from a
// decoded data file (F-15 design P-3, P-11; F-13 design D-2 to D-7;
// ADR-026). It mirrors src/web/src/lib/page, metric only (ADR-027): the
// committed samples in testdata/page are the contract both reproduce. Pure:
// it imports only datafile and does no I/O.
package page

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"workout-explorer/internal/datafile"
)

// ViewModel is the template's only input. Field names are the template's and
// the samples' keys.
type ViewModel struct {
	Page        Group
	Header      Header
	Summary     []SummaryItem
	Device      string // F-43 DI-7: the device line, "" = none
	Route       Route
	ChartsClass string
	Laps        *Laps
	Card        Card
}

// Card is the Open Graph card (F-23 PC-6), built in Go only: lib/page has no
// counterpart, and the sample comparison skips it.
type Card struct{ Title, Description, Image string }

// Group is the page group.
type Group struct{ Title, ViewPath, DataURL string }

// Header is the header group.
type Header struct {
	Title, SportName, SportIcon, Start, Description, ExpiresAt, ExpiresText string
}

// SummaryItem is one summary cell.
type SummaryItem struct{ Key, Label, Value, Unit, Max string }

// Route is the route presence as strings the template only tests.
type Route struct{ Map, NoLocation, Hidden string }

// Column is one laps column.
type Column struct{ Key, Label string }

// Laps is the laps group. None is "yes" for a file without laps whose
// distance advances: the module shows "No laps recorded." (F-18 L-8).
type Laps struct {
	Columns []Column
	Rows    [][]string
	None    string
	// Sections are Rows grouped under their separator rows (F-42 MS-3,
	// MS-6), empty without sections; when set, the template prints them
	// instead of Rows.
	Sections []LapsSection
}

// Settings are the workout's settings besides the data file (lib/page PageSettings).
type Settings struct {
	Title       string
	Description string
	ExpiresAt   time.Time
	PublicID    string
	DataFile    string // "data.{hash}.bin"
	RouteHidden bool   // F-16 H-5: the uploader hid the entire route
	BaseURL     string // F-23 PC-8: origin for absolute links, no trailing slash
	Preview     string // "preview.{hash}.png" (F-23 PC-2)
	// Suggested is the UP-5 title shown while Title is empty ("" = none:
	// sport and date); worked out by the caller from the public file (F-45 ST-5).
	Suggested string
}

// totals is a set of metadata totals as numbers; absent keys are missing.
type totals map[string]float64

func totalsOf(v any) totals {
	// Marshal and read back: the JSON keys are the metadata's own names, and
	// json.Number gives the value as written (ADR-026 item 3).
	b, _ := json.Marshal(v) // cannot fail: strings and json.Number only
	var raw map[string]json.Number
	_ = json.Unmarshal(b, &raw)
	t := totals{}
	for k, n := range raw {
		if f, err := strconv.ParseFloat(string(n), 64); err == nil && finite(f) {
			t[k] = f
		}
	}
	return t
}

func (t totals) get(k string) (float64, bool) { v, ok := t[k]; return v, ok }

func intText(v float64) string { return formatFixed(v, 0) }

// Build returns the view model of f with settings s.
func Build(f datafile.File, s Settings) ViewModel {
	m := f.Meta
	offset := int64(0)
	if m.TimeZone.Known {
		offset = int64(m.TimeZone.OffsetSeconds)
	}
	title := s.Title
	if jsTrim(title) == "" {
		title = DefaultTitle(f)
		if s.Suggested != "" {
			title = s.Suggested
		}
	}
	expires := s.ExpiresAt.UTC().Format(time.RFC3339)
	base := "/w/" + s.PublicID
	presence := routePresence(f)
	route := Route{}
	switch presence {
	case "route":
		route.Map = "yes"
	case "no-location-data":
		if s.RouteHidden {
			route.Hidden = "yes"
		} else {
			route.NoLocation = "yes"
		}
	}
	vm := ViewModel{
		Page: Group{Title: title, ViewPath: base, DataURL: base + "/" + s.DataFile},
		Header: Header{
			Title:       title,
			SportName:   sportName(m.Sport),
			SportIcon:   sportIcon(m.Sport),
			Start:       formatLocalStart(m.StartTime, m.TimeZone.Known, int64(m.TimeZone.OffsetSeconds)),
			Description: s.Description,
			ExpiresAt:   expires,
			ExpiresText: "Expires around " + formatDate(expires, offset),
		},
		Summary:     summaryItems(m.Sport, totalsOf(m.Summary)),
		Device:      deviceLine(m.Device),
		Route:       route,
		ChartsClass: chartsClass(f),
		Laps:        lapsOrNone(f),
	}
	if vm.Laps != nil && vm.Laps.None == "" {
		vm.Laps.Sections = lapSections(sections(m), m.Laps, vm.Laps.Rows, len(vm.Laps.Columns))
	}
	desc := make([]string, 0, 3)
	for _, i := range CardItems(f, vm) {
		desc = append(desc, strings.TrimSpace(i.Label+" "+i.Value+" "+i.Unit))
	}
	vm.Card = Card{Title: title, Description: strings.Join(desc, " · "), Image: s.BaseURL + base + "/" + s.Preview}
	return vm
}

// DefaultTitle is the title of a workout without one: "Ride, 9 Oct 2026".
func DefaultTitle(f datafile.File) string {
	offset := int64(0)
	if f.Meta.TimeZone.Known {
		offset = int64(f.Meta.TimeZone.OffsetSeconds)
	}
	return sportName(f.Meta.Sport) + ", " + formatDate(f.Meta.StartTime, offset)
}

// RowTitle is the title and the header's start text of a workout from its
// row alone, without the data file (F-37 DK-7): the stored title, else
// DefaultTitle's rule over the row's sport, start and offset (nil: unknown
// zone). A nil start reads as the zero time, as the data file writes it.
func RowTitle(title, sport string, start *time.Time, offsetS *int) (string, string) {
	var t time.Time
	if start != nil {
		t = *start
	}
	rfc := t.UTC().Format(time.RFC3339)
	off := int64(0)
	if offsetS != nil {
		off = int64(*offsetS)
	}
	if jsTrim(title) == "" {
		title = sportName(sport) + ", " + formatDate(rfc, off)
	}
	return title, formatLocalStart(rfc, offsetS != nil, off)
}

// CardItems are the card's values (F-23 PC-3), selected from the summary:
// distance above zero; moving time, or elapsed time without positions (either
// as fallback); elevation gain above zero when altitude was recorded; average
// pace or speed of a ride, run, walk, hike or swim; average heart rate. Never
// a "Max" fallback.
func CardItems(f datafile.File, vm ViewModel) []SummaryItem {
	by := map[string]SummaryItem{}
	for _, i := range vm.Summary {
		by[i.Key] = i
	}
	var out []SummaryItem
	add := func(k string, ok bool) {
		if i, found := by[k]; found && ok && !strings.HasPrefix(i.Label, "Max ") {
			out = append(out, SummaryItem{Key: i.Key, Label: i.Label, Value: i.Value, Unit: i.Unit})
		}
	}
	hasDist := strings.Trim(by["distance"].Value, "0.") != ""
	add("distance", hasDist)
	order := []string{"moving_time", "elapsed_time"}
	if vm.Route.Map == "" {
		order[0], order[1] = order[1], order[0]
	}
	if _, ok := by[order[0]]; ok {
		add(order[0], true)
	} else {
		add(order[1], true)
	}
	alt, ok := stream(f, "altitude")
	add("ascent", ok && anyValid(alt) && strings.Trim(by["ascent"].Value, "0.") != "")
	add("speed", hasDist && cardSpeedSports[vm.Header.SportIcon])
	add("heart_rate", true)
	return out
}

var cardSpeedSports = map[string]bool{"cycling": true, "running": true, "walking": true, "hiking": true, "swimming": true}

// lapsOrNone is F-18 L-8: the laps group, or the "No laps recorded." module
// when the file has no laps but its distance advances, or nil.
func lapsOrNone(f datafile.File) *Laps {
	if l := lapsGroup(f.Meta.Sport, f.Meta.Laps); l != nil {
		return l
	}
	if distanceAdvances(f) {
		return &Laps{Columns: []Column{}, Rows: [][]string{}, None: "yes", Sections: []LapsSection{}}
	}
	return nil
}

// distanceAdvances mirrors lib/page distanceAdvances (F-18 L-6, L-8): the
// last valid distance is above the first.
func distanceAdvances(f datafile.File) bool {
	d, ok := stream(f, "distance")
	if !ok {
		return false
	}
	i, j := 0, len(d.Ints)-1
	for i < len(d.Ints) && !d.Valid[i] {
		i++
	}
	for j > i && !d.Valid[j] {
		j--
	}
	return j > i && d.Ints[j] > d.Ints[i]
}

// pair is a cell with an average and an optional maximum (OQ-61).
func pair(key, label, unit string, t totals, avgKey, maxKey string, fmt func(float64) string) *SummaryItem {
	a, m := "", ""
	if v, ok := t.get(avgKey); ok {
		a = fmt(v)
	}
	if v, ok := t.get(maxKey); ok {
		m = fmt(v)
	}
	if a != "" {
		return &SummaryItem{Key: key, Label: "Avg " + label, Value: a, Unit: unit, Max: m}
	}
	if m != "" {
		return &SummaryItem{Key: key, Label: "Max " + label, Value: m, Unit: unit}
	}
	return nil
}

// summaryItems: the AP-2 metrics the summary has, in AP-2 order.
func summaryItems(sport string, t totals) []SummaryItem {
	items := []SummaryItem{}
	one := func(key, label, tk, unit string, fmt func(float64) string) {
		if v, ok := t.get(tk); ok {
			items = append(items, SummaryItem{Key: key, Label: label, Value: fmt(v), Unit: unit})
		}
	}
	add := func(i *SummaryItem) {
		if i != nil {
			items = append(items, *i)
		}
	}
	one("distance", "Distance", "distance", "km", formatDistance)
	one("moving_time", "Moving time", "moving_time", "", formatElapsed)
	one("elapsed_time", "Elapsed time", "elapsed_time", "", formatElapsed)
	one("calories", "Calories", "calories", "kcal", intText)
	one("pool", "Pool", "pool_length_m", "m", intText)
	one("pool", "Pool", "pool_length_yd", "yd", intText)
	one("lengths", "Lengths", "lengths", "", intText)
	speedLabel := "speed"
	if isPaceSport(sport) {
		speedLabel = "pace"
	}
	speed := func(v float64) string { return formatSpeedOrPace(sport, v) }
	add(pair("speed", speedLabel, speedOrPaceUnit(sport), t, "avg_speed", "max_speed", speed))
	add(pair("heart_rate", "heart rate", "bpm", t, "avg_heart_rate", "max_heart_rate", intText))
	add(pair("power", "power", "W", t, "avg_power", "max_power", intText))
	add(pair("cadence", "cadence", cadenceUnit(sport), t, "avg_cadence", "max_cadence", intText))
	one("ascent", "Elevation gain", "ascent", "m", intText)
	add(pair("temperature", "temperature", "°C", t, "avg_temperature", "max_temperature", intText))
	return items
}

// sensorLabels are the device line's names of the sensor codes (F-43 DI-7).
var sensorLabels = map[string]string{
	"heart_rate_monitor": "heart-rate monitor", "power_meter": "power meter", "speed_sensor": "speed sensor",
	"cadence_sensor": "cadence sensor", "speed_cadence_sensor": "speed and cadence sensor", "foot_pod": "foot pod", "radar": "radar",
}

// deviceLine is F-43 DI-7: "Recorded with {name} · {labels}", "Recorded with
// {name}", "Sensors: {labels}" or "". Codes without a label are skipped.
func deviceLine(d datafile.Device) string {
	var labels []string
	for _, c := range d.Sensors {
		if l, ok := sensorLabels[c]; ok {
			labels = append(labels, l)
		}
	}
	list := strings.Join(labels, ", ")
	switch {
	case d.Name != "" && list != "":
		return "Recorded with " + d.Name + " · " + list
	case d.Name != "":
		return "Recorded with " + d.Name
	case list != "":
		return "Sensors: " + list
	}
	return ""
}

type lapColumn struct {
	key, label string
	fmt        func(float64) string
}

// lapsGroup: AP-8 laps from the lap totals as written (D-5): device values,
// completed by the server where the device recorded none (F-18 L-1); nil
// without laps.
func lapsGroup(sport string, laps []datafile.Lap) *Laps {
	if len(laps) == 0 {
		return nil
	}
	speedLabel := "Speed"
	if isPaceSport(sport) {
		speedLabel = "Pace"
	}
	cols := []lapColumn{
		{"elapsed_time", "Time", formatElapsed},
		{"distance", "Distance (km)", formatDistance},
		{"avg_speed", speedLabel + " (" + speedOrPaceUnit(sport) + ")", func(v float64) string { return formatSpeedOrPace(sport, v) }},
		{"ascent", "Gain (m)", intText},
		{"avg_heart_rate", "HR (bpm)", intText},
		{"avg_power", "Power (W)", intText},
		{"avg_cadence", "Cadence (" + cadenceUnit(sport) + ")", intText},
	}
	ts := make([]totals, len(laps))
	for i, l := range laps {
		ts[i] = totalsOf(l.Totals)
	}
	cell := func(t totals, c lapColumn) string {
		v, ok := t.get(c.key)
		if !ok {
			return missingText
		}
		if s := c.fmt(v); s != "" {
			return s
		}
		return missingText
	}
	var shown []lapColumn
	for _, c := range cols {
		for _, t := range ts {
			if cell(t, c) != missingText {
				shown = append(shown, c)
				break
			}
		}
	}
	out := &Laps{Columns: []Column{{Key: "number", Label: "#"}}, Rows: make([][]string, len(laps)), Sections: []LapsSection{}}
	for _, c := range shown {
		out.Columns = append(out.Columns, Column{Key: c.key, Label: c.label})
	}
	for i, t := range ts {
		row := []string{strconv.Itoa(i + 1)}
		for _, c := range shown {
			row = append(row, cell(t, c))
		}
		out.Rows[i] = row
	}
	return out
}

func stream(f datafile.File, name string) (datafile.Stream, bool) {
	for _, s := range f.Streams {
		if s.Descriptor.Name == name {
			return s, true
		}
	}
	return datafile.Stream{}, false
}

func anyValid(s datafile.Stream) bool {
	for _, v := range s.Valid {
		if v {
			return true
		}
	}
	return false
}

// routePresence mirrors lib/route/presence.
func routePresence(f datafile.File) string {
	lat, ok := stream(f, "latitude")
	if !ok {
		return "no-location-data"
	}
	if anyValid(lat) {
		return "route"
	}
	return "no-visible-route"
}

// chartMetrics is AP-4's chart order (lib/charts CHART_ORDER).
var chartMetrics = []string{"altitude", "speed", "heart_rate", "power", "cadence", "temperature"}

// chartsClass: "charts-n cells-k" (readout cells: time, distance if present,
// one per chart), plus " has-distance" with a distance stream (the axis
// toolbar row, F-17 R-8), or "" with no charts.
func chartsClass(f datafile.File) string {
	n := 0
	for _, name := range chartMetrics {
		if s, ok := stream(f, name); ok && anyValid(s) {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	cells := 1 + n
	_, distance := stream(f, "distance")
	if distance {
		cells++
	}
	class := "charts-" + strconv.Itoa(n) + " cells-" + strconv.Itoa(cells)
	if distance {
		class += " has-distance"
	}
	return class
}
