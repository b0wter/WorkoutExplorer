package page

import (
	"cmp"
	"slices"
	"strconv"

	"workout-explorer/internal/datafile"
)

// LapsSection is one separator row of the laps table and the lap rows under
// it (F-42 MS-3, MS-6): name and column span as the template prints them.
type LapsSection struct {
	Name, Span string
	Rows       [][]string
}

// section is one section of a multisport workout: its name and first sample.
type section struct {
	name  string
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
		out[i] = section{name, s.StartIndex}
	}
	return out
}

// lapSections is F-42 MS-3: rows (one per lap) grouped by section, every
// section in order, a section without laps with no rows. A lap belongs to
// the last section whose start ≤ its start_index; a lap out of order stays
// under the separator above it. Empty without sections or laps.
func lapSections(secs []section, laps []datafile.Lap, rows [][]string, span int) []LapsSection {
	out := []LapsSection{}
	if secs == nil || len(laps) == 0 {
		return out
	}
	sp := strconv.Itoa(span)
	upTo := func(k int) {
		for len(out) <= k {
			out = append(out, LapsSection{Name: secs[len(out)].name, Span: sp, Rows: [][]string{}})
		}
	}
	for i, l := range laps {
		k := 0
		for j, s := range secs {
			if s.start <= l.StartIndex {
				k = j
			}
		}
		upTo(k)
		out[len(out)-1].Rows = append(out[len(out)-1].Rows, rows[i])
	}
	upTo(len(secs) - 1)
	return out
}
