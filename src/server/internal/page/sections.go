package page

import (
	"cmp"
	"slices"
	"strconv"

	"workout-explorer/internal/datafile"
)

// section is one section of a multisport workout: its name and first sample.
type section struct {
	name  string
	sport string
	start int
}

// sections is F-42 MS-1, MS-2 (mirrors lib/sections): nil unless the
// sessions hold two or more distinct sports; else one section per session in
// start order (file order on ties), the first starting at sample 0,
// transitions named T1, T2, … in order.
func sections(m datafile.Meta) []section {
	sports := map[string]bool{}
	for _, s := range m.Sessions {
		sports[s.Sport] = true
	}
	if len(sports) < 2 {
		return nil
	}
	ss := slices.Clone(m.Sessions)
	slices.SortStableFunc(ss, func(a, b datafile.Session) int { return cmp.Compare(a.Start, b.Start) })
	out := make([]section, len(ss))
	t := 0
	for i, s := range ss {
		name := sportName(s.Sport)
		if s.Sport == "transition" {
			t++
			name = "T" + strconv.Itoa(t)
		}
		if i == 0 {
			s.StartIndex = 0
		}
		out[i] = section{name, s.Sport, s.StartIndex}
	}
	return out
}

// lapLegs is F-64 LL-2 (F-42 MS-3): one block per section, every section in
// order, each with its own laps table built for the section's sport (speed or
// pace, cadence unit, columns, trigger per leg); a section without laps keeps
// its heading alone. A lap belongs to the last section whose start <= its
// start_index; a lap out of order stays in the block above it. Empty without
// sections or laps.
func lapLegs(secs []section, laps []datafile.Lap) []LapsLeg {
	if secs == nil || len(laps) == 0 {
		return []LapsLeg{}
	}
	per := make([][]datafile.Lap, len(secs))
	cur := 0
	for _, l := range laps {
		for j, s := range secs {
			if s.start <= l.StartIndex {
				cur = max(cur, j)
			}
		}
		per[cur] = append(per[cur], l)
	}
	out := make([]LapsLeg, len(secs))
	for k, s := range secs {
		leg := LapsLeg{Name: s.name, Sport: s.sport, Columns: []Column{}, Rows: [][]string{}}
		if g := lapsGroup(s.sport, per[k]); g != nil {
			leg.Columns, leg.Rows, leg.NotePace, leg.NoteSpeed = g.Columns, g.Rows, g.NotePace, g.NoteSpeed
		}
		out[k] = leg
	}
	return out
}
