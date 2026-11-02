package workout

import (
	"slices"
	"sort"
	"time"
)

// The sport timeline (F-08 design v1.1, D-12; replaces F-02 R-5's membership
// rule): the sport of every sample comes from the sessions first, then from
// sport hints (sport messages and lap.sport) for samples no session covers.

// sportTransition is the FIT sport of a transition session (R-13).
const sportTransition = "transition"

// hintKind orders hints that share an offset: a sport message ranks after a
// lap, so it wins.
const (
	hintLap = iota
	hintSport
)

// sportHint is the sport named by a sport message or a lap, at a time offset.
type sportHint struct {
	offset int32
	kind   int
	sport  string
}

// namesLeg reports whether a hint's sport names the sport of a leg: "",
// "multisport" and "all" (FIT profile names) do not.
func namesLeg(sport string) bool {
	return sport != "" && sport != "multisport" && sport != "all"
}

// collectHints builds the hints of rec: each sport message at the offset of
// its time (R-1), each lap with a sport at its start offset. Unusable sports
// are dropped. The result is sorted stably by offset and kind, so among equal
// ones the later in arrival order is last and wins.
func collectHints(rec Recording, start time.Time) []sportHint {
	var hints []sportHint
	for _, s := range rec.Sports {
		if namesLeg(s.Sport) {
			hints = append(hints, sportHint{offset: offsetOf(start, s.At), kind: hintSport, sport: s.Sport})
		}
	}
	for _, l := range rec.Laps {
		if namesLeg(l.Sport) {
			off, _ := spanOffsets(start, l.Start, l.End)
			hints = append(hints, sportHint{offset: off, kind: hintLap, sport: l.Sport})
		}
	}
	// Laps were appended after the sport messages; the stable sort by
	// (offset, kind) restores arrival order within a kind.
	sort.SliceStable(hints, func(i, j int) bool {
		if hints[i].offset != hints[j].offset {
			return hints[i].offset < hints[j].offset
		}
		return hints[i].kind < hints[j].kind
	})
	return hints
}

// sportTimeline answers sportAt(t) for any t in O(log k) after an O(k²)
// preparation (k = sessions, at most MaxSessions; hints are sorted once), so
// no loop over sessions, laps or hints runs per sample (ADR-017 rev. 1).
type sportTimeline struct {
	sessions []Session
	// Rules 1, 3 and 4 change their answer only at session starts, at start+1
	// and at end+1: bounds are those points, and for the interval
	// [bounds[i], bounds[i+1]) cover[i] is the rule-1 session (-1 if none)
	// and fallback[i] the session of rules 3 and 4.
	bounds   []int64
	cover    []int
	fallback []int
	ends     []int32     // session ends, sorted, for Eprev
	hints    []sportHint // sorted by offset, kind, arrival
}

func newSportTimeline(sessions []Session, hints []sportHint) *sportTimeline {
	tl := &sportTimeline{sessions: sessions, hints: hints}
	for _, s := range sessions {
		tl.bounds = append(tl.bounds, int64(s.Start), int64(s.Start)+1, int64(s.End)+1)
		tl.ends = append(tl.ends, s.End)
	}
	slices.Sort(tl.bounds)
	tl.bounds = slices.Compact(tl.bounds)
	slices.Sort(tl.ends)
	tl.cover = make([]int, len(tl.bounds))
	tl.fallback = make([]int, len(tl.bounds))
	for i, b := range tl.bounds {
		tl.cover[i], tl.fallback[i] = sessionRules(sessions, b)
	}
	return tl
}

// sessionRules evaluates rules 1, 3 and 4 directly at t: the first session
// that covers t (or -1), and the session of rules 3 and 4.
func sessionRules(sessions []Session, t int64) (cover, fallback int) {
	cover = -1
	for i, s := range sessions {
		if int64(s.Start) <= t && t <= int64(s.End) {
			cover = i
			break
		}
	}
	// Rule 3: the latest start earlier than t; on equal starts the first in
	// file order (F-02 E-5). Rule 4: the first session.
	fallback = -1
	for i, s := range sessions {
		if int64(s.Start) < t && (fallback < 0 || s.Start > sessions[fallback].Start) {
			fallback = i
		}
	}
	if fallback < 0 {
		fallback = 0
	}
	return cover, fallback
}

// at returns the sport at offset t (rules 1 to 5 of the design).
func (tl *sportTimeline) at(t int32) string {
	if len(tl.sessions) > 0 {
		cover, fallback := -1, 0
		if i := sort.Search(len(tl.bounds), func(i int) bool { return tl.bounds[i] > int64(t) }) - 1; i >= 0 {
			cover, fallback = tl.cover[i], tl.fallback[i]
		}
		if cover >= 0 {
			return tl.sessions[cover].Sport // rule 1
		}
		if h, ok := tl.hintBefore(t); ok {
			return h.sport // rule 2
		}
		return tl.sessions[fallback].Sport // rules 3 and 4
	}
	if h, ok := tl.hintBefore(t); ok {
		return h.sport // rule 2 without sessions: Eprev is minus infinity
	}
	if len(tl.hints) > 0 {
		return tl.earliestHint() // rule 5
	}
	return ""
}

// inTransition reports whether sample time t is a transition sample (R-13,
// D-23): rule 1 of the timeline, the first session in file order covering t,
// is a session with sport "transition". Sport hints and the fallback rules
// never make a transition.
func (tl *sportTimeline) inTransition(t int32) bool {
	if len(tl.sessions) == 0 {
		return false
	}
	i := sort.Search(len(tl.bounds), func(i int) bool { return tl.bounds[i] > int64(t) }) - 1
	return i >= 0 && tl.cover[i] >= 0 && tl.sessions[tl.cover[i]].Sport == sportTransition
}

// hintBefore is rule 2: the winning hint with Eprev(t) <= offset <= t, which
// is the last hint at or before t if its offset is not before Eprev(t).
func (tl *sportTimeline) hintBefore(t int32) (sportHint, bool) {
	i := sort.Search(len(tl.hints), func(i int) bool { return tl.hints[i].offset > t }) - 1
	if i < 0 {
		return sportHint{}, false
	}
	h := tl.hints[i]
	// Eprev: the largest session End below t.
	if j := sort.Search(len(tl.ends), func(j int) bool { return tl.ends[j] >= t }) - 1; j >= 0 && h.offset < tl.ends[j] {
		return sportHint{}, false
	}
	return h, true
}

// earliestHint is rule 5: the winning hint at the earliest hint offset.
func (tl *sportTimeline) earliestHint() string {
	first := tl.hints[0].offset
	i := sort.Search(len(tl.hints), func(i int) bool { return tl.hints[i].offset > first }) - 1
	return tl.hints[i].sport
}

// sportSet collects the distinct sports in order of first appearance.
type sportSet struct {
	list []string
}

func (s *sportSet) add(sport string) {
	if !slices.Contains(s.list, sport) {
		s.list = append(s.list, sport)
	}
}
