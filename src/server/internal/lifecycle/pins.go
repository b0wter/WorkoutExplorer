package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"
	"unicode"
	"unicode/utf8"
)

// Map pins (F-56, ADR-057). The row holds the whole set; the public rule
// (kept, public) is publish's, reached through Trimmer.CheckEdits.

// Limits of PN-2 (OQ-238), in Unicode code points.
const (
	MaxPins          = 50
	MaxPinTitleChars = 60
	MaxPinNoteChars  = 500
)

// Refusals of pins (F-12 settings object).
const (
	MsgBadPins    = "pins must be a list of at most 50 pins, each with exactly type, title, note, start_at and end_at."
	MsgPinHidden  = "This pin is in a hidden or trimmed part of the route."
	MsgPinOutside = "A pin must lie within the recording."
)

// PinTypes are the six types of PIN-2.
var PinTypes = []string{"point_of_interest", "danger", "difficult_section", "water_food", "photo_spot", "note"}

// Pin is one pin: StartAt is the instant of its sample, EndAt the instant of
// the last sample of a section (nil: a point). Stored resolved (PN-3).
type Pin struct {
	Type, Title, Note string
	StartAt           time.Time
	EndAt             *time.Time
}

// OptPins is the pins of a PATCH; Set with no pins clears them.
type OptPins struct {
	Set bool
	V   []Pin
}

// PinCheck is the verdict of Trimmer.CheckEdits on one pin: Resolved has the
// instants of the nearest samples; Kept: every sample of the pin lies in the
// trim window; Public: kept, and each such sample has a position in the
// output of trim then hiding (PN-4). Outside: an instant is outside the recording.
type PinCheck struct {
	Resolved        Pin
	Kept, Public    bool
	Outside, Merged bool // Merged: a section whose ends resolve to one sample
}

// checkPinShape is the shape rule of PN-2 for one pin as sent.
func checkPinShape(p Pin) error {
	if !slices.Contains(PinTypes, p.Type) {
		return invalid(MsgBadPins)
	}
	if utf8.RuneCountInString(p.Title) > MaxPinTitleChars {
		return invalid("A pin title can have at most 60 characters.")
	}
	for _, c := range p.Title {
		if unicode.IsControl(c) {
			return invalid("A pin title cannot contain control characters.")
		}
	}
	if utf8.RuneCountInString(p.Note) > MaxPinNoteChars {
		return invalid("A pin note can have at most 500 characters.")
	}
	for _, c := range p.Note {
		if unicode.IsControl(c) && c != '\n' {
			return invalid("A pin note cannot contain control characters other than line breaks.")
		}
	}
	if p.EndAt != nil && !p.EndAt.After(p.StartAt) {
		return invalid(MsgBadPins)
	}
	return nil
}

// sortPins is the canonical order (ADR-057 item 1): start instant, point
// before section, end instant, then the order given.
func sortPins(ps []Pin) {
	slices.SortStableFunc(ps, func(a, b Pin) int {
		if c := a.StartAt.Compare(b.StartAt); c != 0 {
			return c
		}
		if (a.EndAt == nil) != (b.EndAt == nil) {
			if a.EndAt == nil {
				return -1
			}
			return 1
		}
		if a.EndAt != nil {
			return a.EndAt.Compare(*b.EndAt)
		}
		return 0
	})
}

func sameAnchor(a, b Pin) bool {
	return a.StartAt.Equal(b.StartAt) && (a.EndAt == nil) == (b.EndAt == nil) && (a.EndAt == nil || a.EndAt.Equal(*b.EndAt))
}

// pinJSON is the stored form of a pin (instants as Unix seconds).
type pinJSON struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Note  string `json:"note"`
	Start int64  `json:"start"`
	End   *int64 `json:"end"`
}

// EncodePins is the pins column: JSON, "" for none (stored as NULL).
func EncodePins(ps []Pin) string {
	if len(ps) == 0 {
		return ""
	}
	out := make([]pinJSON, len(ps))
	for i, p := range ps {
		out[i] = pinJSON{p.Type, p.Title, p.Note, p.StartAt.Unix(), nil}
		if p.EndAt != nil {
			e := p.EndAt.Unix()
			out[i].End = &e
		}
	}
	b, err := json.Marshal(out) // strings and integers always marshal
	if err != nil {
		panic(err)
	}
	return string(b)
}

// DecodePins reads the pins column; anything unreadable is an error.
func DecodePins(s string) ([]Pin, error) {
	if s == "" {
		return nil, nil
	}
	var in []pinJSON
	if err := json.Unmarshal([]byte(s), &in); err != nil {
		return nil, err
	}
	out := make([]Pin, len(in))
	for i, p := range in {
		out[i] = Pin{Type: p.Type, Title: p.Title, Note: p.Note, StartAt: time.Unix(p.Start, 0).UTC()}
		if p.End != nil {
			e := time.Unix(*p.End, 0).UTC()
			out[i].EndAt = &e
		}
	}
	return out, nil
}

// pinCandidates is the first half of PN-5: the pins to judge, and whether
// they need judging (a parse). They do only when pins are sent, or a radius
// or the trim changes on a row with pins; otherwise the stored set stays.
func pinCandidates(rec Record, p SettingsPatch) (cand []Pin, judge bool, err error) {
	settingsChanged := p.HideStartM != nil || p.HideEndM != nil || p.TrimStart.Set || p.TrimEnd.Set
	if !p.Pins.Set && (len(rec.Pins) == 0 || !settingsChanged) {
		return rec.Pins, false, nil
	}
	cand = rec.Pins
	if p.Pins.Set {
		cand = slices.Clone(p.Pins.V)
		if len(cand) > MaxPins {
			return nil, false, invalid(MsgBadPins)
		}
		for i, q := range cand {
			cand[i].Title = trimSpace(q.Title) // as the workout title (UP-4)
			if err := checkPinShape(cand[i]); err != nil {
				return nil, false, err
			}
		}
	}
	return cand, len(cand) > 0, nil
}

// judgePins is the second half of PN-5 (under the O-7 lock): the pins to
// store from the verdicts on cand against the record with the patch
// applied, or an *InvalidSettingsError.
func judgePins(rec Record, p SettingsPatch, checks []PinCheck) ([]Pin, error) {
	var out []Pin
	for _, c := range checks {
		if c.Outside || c.Merged {
			if p.Pins.Set {
				return nil, invalid(MsgPinOutside)
			}
			continue // a stored pin a parser change moved out of reach
		}
		stored := slices.ContainsFunc(rec.Pins, func(q Pin) bool { return sameAnchor(q, c.Resolved) })
		switch {
		case !c.Kept && stored: // PN-5 a: the trim removed it
			continue
		case !c.Public && !stored: // new or moved into a hidden or trimmed part
			return nil, invalid(MsgPinHidden)
		}
		out = append(out, c.Resolved)
	}
	sortPins(out)
	return out, nil
}

// resolveEdits is PN-5 and F-65 AL-5 (under the O-7 lock), against eff, the
// record with the patch applied: the pins and added lap boundaries to
// store, or an *InvalidSettingsError. Both are judged from one parse
// (through the Trimmer), and only when one of them needs it.
func (s *Service) resolveEdits(ctx context.Context, rec, eff Record, p SettingsPatch) ([]Pin, []time.Time, error) {
	pins, judgeP, err := pinCandidates(rec, p)
	if err != nil {
		return nil, nil, err
	}
	laps, judgeL, err := lapCandidates(rec, p)
	if err != nil {
		return nil, nil, err
	}
	if !judgeP && !judgeL {
		if p.Pins.Set { // sent empty
			pins = nil
		}
		if p.AddedLapBoundaries.Set {
			laps = nil
		}
		return pins, laps, nil
	}
	if s.d.Trimmer == nil {
		return nil, nil, s.ownerFail("edits", rec.ID, errors.New("no trimmer"))
	}
	var pc []Pin
	var lc []time.Time
	if judgeP {
		pc = pins
	}
	if judgeL {
		lc = laps
	}
	pcs, lcs, err := s.d.Trimmer.CheckEdits(ctx, eff, pc, lc)
	if err != nil {
		return nil, nil, s.ownerFail("edits", rec.ID, err)
	}
	switch {
	case judgeP:
		if pins, err = judgePins(rec, p, pcs); err != nil {
			return nil, nil, err
		}
	case p.Pins.Set:
		pins = nil
	}
	switch {
	case judgeL:
		if laps, err = judgeLaps(rec, p, lcs); err != nil {
			return nil, nil, err
		}
	case p.AddedLapBoundaries.Set:
		laps = nil
	}
	return pins, laps, nil
}
