package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// F-56 PN-2, PN-5 at the service, with a stand-in for the public rule
// (publish owns the real one): a recording of 1,000 s from pinBase, one
// sample a second; the hiding radii hide that many seconds at each end; a
// trim keeps the samples from its start to its end instant.
var pinBase = time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)

func at(i int) time.Time { return pinBase.Add(time.Duration(i) * time.Second) }

type gridTrimmer struct {
	fakeTrimmer
	pinCalls int
}

func (g *gridTrimmer) CheckPins(_ context.Context, r Record, pins []Pin) ([]PinCheck, error) {
	g.pinCalls++
	lo, hi := 0, 999
	if r.TrimStartAt != nil {
		lo = int(r.TrimStartAt.Sub(pinBase) / time.Second)
	}
	if r.TrimEndAt != nil {
		hi = int(r.TrimEndAt.Sub(pinBase) / time.Second)
	}
	idx := func(t time.Time) int { return int(t.Sub(pinBase) / time.Second) }
	var out []PinCheck
	for _, p := range pins {
		c := PinCheck{Resolved: p}
		ends := []int{idx(p.StartAt)}
		if p.EndAt != nil {
			ends = append(ends, idx(*p.EndAt))
		}
		c.Kept, c.Public = true, true
		for _, i := range ends {
			if i < 0 || i > 999 {
				c.Outside = true
			}
			c.Kept = c.Kept && i >= lo && i <= hi
			c.Public = c.Public && i >= lo && i <= hi && i >= r.HideStartM && i <= 999-r.HideEndM
		}
		out = append(out, c)
	}
	return out, nil
}

func pt(i int, title string) Pin { return Pin{Type: "danger", Title: title, StartAt: at(i)} }
func sec(i, j int, title string) Pin {
	e := at(j)
	return Pin{Type: "difficult_section", Title: title, StartAt: at(i), EndAt: &e}
}

func pinsOf(r *fakeRepo) []Pin {
	for _, row := range r.rows {
		return row.Pins
	}
	return nil
}

func titles(ps []Pin) string {
	var t []string
	for _, p := range ps {
		t = append(t, p.Title)
	}
	return strings.Join(t, ",")
}

func refused(t *testing.T, err error, want string) {
	t.Helper()
	var ie *InvalidSettingsError
	if !errors.As(err, &ie) || (want != "" && ie.Message != want) {
		t.Fatalf("want a refusal %q, got %v", want, err)
	}
}

func TestPinsReplaceAndCanonicalOrder(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	g := &gridTrimmer{}
	s.d.Trimmer = g
	ctx := context.Background()
	send := func(ps ...Pin) (Record, error) {
		return s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Pins: OptPins{Set: true, V: ps}})
	}
	// Canonical order: start, point before section, end, then as sent.
	rec, err := send(sec(100, 300, "s300"), pt(100, "p100"), sec(100, 200, "s200"), pt(50, "p50"), pt(50, "p50b"))
	if err != nil || titles(rec.Pins) != "p50,p50b,p100,s200,s300" {
		t.Fatalf("%v %v", titles(rec.Pins), err)
	}
	// An unrelated change neither parses nor touches the pins.
	calls := g.pinCalls
	if rec, err = s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Title: sp("x")}); err != nil || g.pinCalls != calls || len(rec.Pins) != 5 {
		t.Fatalf("%v %d", err, len(rec.Pins))
	}
	// [] clears; the whole set is replaced.
	if rec, err = send(pt(7, "only")); err != nil || titles(rec.Pins) != "only" {
		t.Fatal(titles(rec.Pins), err)
	}
	if rec, err = send(); err != nil || rec.Pins != nil || pinsOf(r) != nil {
		t.Fatal(err, rec.Pins)
	}
}

func TestPinsRules(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	s.d.Trimmer = &gridTrimmer{}
	ctx := context.Background()
	long := func(n int) string { return strings.Repeat("é", n) }
	tests := []struct {
		name string
		pin  Pin
		ok   bool
	}{
		{"fine", pt(1, "T"), true},
		{"empty title allowed", pt(1, ""), true},
		{"title 60", pt(1, long(60)), true},
		{"title 61", pt(1, long(61)), false},
		{"title control", pt(1, "a\nb"), false},
		{"title hostile is text", pt(1, "<img src=x onerror=alert(1)>"), true},
		{"note 500", Pin{Type: "note", Note: long(500), StartAt: at(1)}, true},
		{"note 501", Pin{Type: "note", Note: long(501), StartAt: at(1)}, false},
		{"note line feed", Pin{Type: "note", Note: "a\nb", StartAt: at(1)}, true},
		{"note tab", Pin{Type: "note", Note: "a\tb", StartAt: at(1)}, false},
		{"unknown type", Pin{Type: "x", StartAt: at(1)}, false},
		{"section end not after start", sec(5, 5, "s"), false},
		{"section end before start", sec(6, 5, "s"), false},
		{"before the recording", Pin{Type: "note", StartAt: at(-1)}, false},
		{"after the recording", Pin{Type: "note", StartAt: at(1000)}, false},
	}
	for _, tc := range tests {
		_, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Pins: OptPins{Set: true, V: []Pin{tc.pin}}})
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", tc.name, err)
		}
		if err != nil {
			refused(t, err, "")
		}
	}
	// 50 pins, not 51; all or nothing.
	var fifty []Pin
	for i := range 50 {
		fifty = append(fifty, pt(i, "p"))
	}
	if rec, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Pins: OptPins{Set: true, V: fifty}}); err != nil || len(rec.Pins) != 50 {
		t.Fatal(err)
	}
	_, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Pins: OptPins{Set: true, V: append(fifty, pt(60, "p"))}})
	refused(t, err, MsgBadPins)
	if len(pinsOf(r)) != 50 {
		t.Error("a refused set was stored")
	}
	// Title is trimmed as the workout title.
	rec, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Pins: OptPins{Set: true, V: []Pin{pt(1, "  hi  ")}}})
	if err != nil || rec.Pins[0].Title != "hi" {
		t.Fatalf("%+v %v", rec.Pins, err)
	}
}

// TestPinsHiddenAndTrimmed is PN-5 (AC 35, 36): a new anchor in a hidden or
// trimmed part is refused; a radius keeps pins stored; a trim deletes them.
func TestPinsHiddenAndTrimmed(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	g := &gridTrimmer{}
	s.d.Trimmer = g
	ctx := context.Background()
	upd := func(p SettingsPatch) (Record, error) { return s.Update(ctx, c.PublicID, c.EditToken, p) }
	set := func(ps ...Pin) SettingsPatch { return SettingsPatch{Pins: OptPins{Set: true, V: ps}} }

	rec, err := upd(set(pt(10, "early"), pt(500, "mid"), sec(480, 520, "midsec"), pt(990, "late")))
	if err != nil || len(rec.Pins) != 4 {
		t.Fatal(err)
	}
	// A radius never deletes: pins stay stored, dimmed ones included.
	if rec, err = upd(SettingsPatch{HideStartM: ip(50), HideEndM: ip(50)}); err != nil || len(rec.Pins) != 4 {
		t.Fatalf("%v %v", titles(rec.Pins), err)
	}
	// A new or moved anchor in a hidden part is refused; the row is unchanged.
	before := pinsOf(r)
	for _, p := range []SettingsPatch{
		set(pt(10, "early"), pt(20, "new")),
		set(pt(10, "early"), pt(995, "new")),
		set(sec(40, 100, "new")),
		set(pt(11, "early moved")),
	} {
		_, err := upd(p)
		refused(t, err, MsgPinHidden)
	}
	if titles(pinsOf(r)) != titles(before) {
		t.Fatal("a refused change was stored")
	}
	// A dimmed pin keeps its anchor: texts and type can change, it can be deleted.
	early := pt(10, "early edited")
	early.Type = "note"
	if rec, err = upd(set(early, pt(500, "mid"), sec(480, 520, "midsec"), pt(990, "late"))); err != nil || rec.Pins[0].Title != "early edited" || rec.Pins[0].Type != "note" {
		t.Fatalf("%+v %v", rec.Pins, err)
	}
	if rec, err = upd(set(pt(500, "mid"), sec(480, 520, "midsec"))); err != nil || len(rec.Pins) != 2 {
		t.Fatal(titles(rec.Pins), err)
	}
	// Lowering the radius again: nothing changes in the row.
	if rec, err = upd(SettingsPatch{HideStartM: ip(0), HideEndM: ip(0)}); err != nil || len(rec.Pins) != 2 {
		t.Fatal(err)
	}
	if rec, err = upd(set(pt(10, "again"), pt(500, "mid"), sec(480, 520, "midsec"))); err != nil || len(rec.Pins) != 3 {
		t.Fatal(titles(rec.Pins), err)
	}
	// A trim deletes every pin outside its window, sent or not.
	start, end := at(100), at(900)
	rec, err = upd(SettingsPatch{TrimStart: OptTime{Set: true, At: &start}, TrimEnd: OptTime{Set: true, At: &end}})
	if err != nil || titles(rec.Pins) != "midsec,mid" && titles(rec.Pins) != "mid,midsec" {
		t.Fatalf("%v %v", titles(rec.Pins), err)
	}
	// ... also when the client sends them again; a new anchor there is refused.
	rec, err = upd(set(pt(10, "again"), pt(500, "mid")))
	refused(t, err, MsgPinHidden)
	// Undoing the trim does not bring them back.
	if rec, err = upd(SettingsPatch{TrimStart: OptTime{Set: true}, TrimEnd: OptTime{Set: true}}); err != nil || len(rec.Pins) != 2 {
		t.Fatalf("%v %v", titles(rec.Pins), err)
	}
	// A section with one end trimmed is deleted whole (never clipped).
	end2 := at(500)
	if rec, err = upd(SettingsPatch{TrimEnd: OptTime{Set: true, At: &end2}}); err != nil || titles(rec.Pins) != "mid" {
		t.Fatalf("%v %v", titles(rec.Pins), err)
	}
}

// A parse only when pins are sent, or a radius or the trim changes on a row with pins.
func TestPinsParseOnlyWhenNeeded(t *testing.T) {
	s, _, _, _, c, _ := owned(t, nil)
	g := &gridTrimmer{}
	s.d.Trimmer = g
	ctx := context.Background()
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{HideStartM: ip(50)}); err != nil || g.pinCalls != 0 {
		t.Fatalf("%v %d", err, g.pinCalls)
	}
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{Pins: OptPins{Set: true, V: []Pin{pt(100, "a")}}}); err != nil || g.pinCalls != 1 {
		t.Fatalf("%v %d", err, g.pinCalls)
	}
	if _, err := s.Update(ctx, c.PublicID, c.EditToken, SettingsPatch{HideEndM: ip(50)}); err != nil || g.pinCalls != 2 {
		t.Fatalf("%v %d", err, g.pinCalls)
	}
}

func TestPinEncodingRoundTrip(t *testing.T) {
	in := []Pin{pt(3, "a \"<b>"), sec(4, 9, "")}
	in[0].Note = "line\nbreak"
	out, err := DecodePins(EncodePins(in))
	if err != nil || len(out) != 2 || out[0].Title != in[0].Title || out[0].Note != in[0].Note || !out[1].EndAt.Equal(*in[1].EndAt) || out[0].EndAt != nil {
		t.Fatalf("%+v %v", out, err)
	}
	if EncodePins(nil) != "" {
		t.Error("none is not empty")
	}
}

// QA-56-r1-5 (AC 35): a stored section keeps its start but moves its end into
// a hidden part: refused, though the start is a stored anchor. A start moved
// into a hidden part with the end kept is refused too.
func TestPinsSectionEndMovedIntoHiddenPart(t *testing.T) {
	s, r, _, _, c, _ := owned(t, nil)
	s.d.Trimmer = &gridTrimmer{}
	ctx := context.Background()
	upd := func(p SettingsPatch) (Record, error) { return s.Update(ctx, c.PublicID, c.EditToken, p) }
	set := func(ps ...Pin) SettingsPatch { return SettingsPatch{Pins: OptPins{Set: true, V: ps}} }
	if _, err := upd(set(sec(100, 500, "s"))); err != nil {
		t.Fatal(err)
	}
	if _, err := upd(SettingsPatch{HideStartM: ip(200)}); err != nil { // the start is now hidden, the end public
		t.Fatal(err)
	}
	for _, p := range []Pin{sec(100, 520, "end moved"), sec(100, 600, "end moved to a public sample"), sec(150, 500, "start moved")} {
		_, err := upd(set(p))
		refused(t, err, MsgPinHidden)
	}
	if titles(pinsOf(r)) != "s" {
		t.Fatal("a refused change was stored")
	}
}
