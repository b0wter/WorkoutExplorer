package page

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode"

	"workout-explorer/internal/datafile"
)

// Suggest is the suggested title of UP-5 (F-45 ST-3): "<Time of day> <sport>
// near <town>". town "" leaves out " near …". Never empty. It holds no place
// data and is never logged: it contains the town (I-5).
func Suggest(m datafile.Meta, town string) string {
	var parts []string
	if t, err := time.Parse(time.RFC3339, m.StartTime); err == nil && t.Year() > 1 {
		off := 0
		if m.TimeZone.Known {
			off = int(m.TimeZone.OffsetSeconds)
		}
		parts = append(parts, timeOfDay(t.UTC().Add(time.Duration(off)*time.Second).Hour()))
	}
	parts = append(parts, suggestSport(m))
	s := strings.Join(parts, " ")
	if town != "" {
		s += " near " + town
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func timeOfDay(h int) string {
	switch {
	case h >= 5 && h < 12:
		return "morning"
	case h >= 12 && h < 17:
		return "afternoon"
	case h >= 17 && h < 21:
		return "evening"
	}
	return "night"
}

// suggestSport: "triathlon" for swim, bike, run in that order (transitions
// ignored), "multisport" for any other workout with sections, else the sport
// name in lower case.
func suggestSport(m datafile.Meta) string {
	if sections(m) == nil {
		return strings.ToLower(sportName(m.Sport))
	}
	ss := slices.Clone(m.Sessions)
	slices.SortStableFunc(ss, func(a, b datafile.Session) int { return cmp.Compare(a.Start, b.Start) })
	var legs []string
	for _, s := range ss {
		if s.Sport != "transition" {
			legs = append(legs, s.Sport)
		}
	}
	if slices.Equal(legs, []string{"swimming", "cycling", "running"}) {
		return "triathlon"
	}
	return "multisport"
}
