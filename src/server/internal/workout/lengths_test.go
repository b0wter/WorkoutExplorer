package workout

import "testing"

// rawLen is an active length [from, to] with the given stroke and count.
func rawLen(from, to int, stroke string, strokes uint16) RawLength {
	return RawLength{Start: at(from), End: at(to), Stroke: stroke, Time: float64(to - from), AvgSpeed: nan, Strokes: strokes}
}

func restLen(from, to int) RawLength {
	return RawLength{Start: at(from), End: at(to), Rest: true, Time: float64(to - from), AvgSpeed: nan}
}

func poolSession(from, to int, stroke string) RawSession {
	return RawSession{Sport: "swimming", SubSport: "lap_swimming", Start: at(from), End: at(to),
		Pool: Pool{Length: 25, Lengths: 4}, Stroke: stroke}
}

// TestLengthsScope is F-48 PL-2 and OQ-185: lengths and lap strokes inside a
// session that is not a pool swim go; those no session covers stay.
func TestLengthsScope(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{
			{Sport: "swimming", SubSport: "open_water", Start: at(0), End: at(99)},
			poolSession(100, 199, "freestyle"),
		},
		Laps: []RawLap{
			{Start: at(0), End: at(99), Stroke: "freestyle"},
			{Start: at(100), End: at(199), Stroke: "backstroke"},
			{Start: at(200), End: at(299), Stroke: "drill"}, // no session: kept
		},
		Lengths: []RawLength{
			rawLen(250, 275, "drill", 0), // no session, out of order on purpose
			rawLen(10, 30, "freestyle", 10),
			rawLen(100, 120, "freestyle", 12),
		},
		Records: recs(seq(0, 299), nil),
	})
	if got := [3]string{w.Laps[0].Stroke, w.Laps[1].Stroke, w.Laps[2].Stroke}; got != [3]string{"", "backstroke", "drill"} {
		t.Errorf("lap strokes = %v", got)
	}
	if len(w.Lengths) != 2 || w.Lengths[0].Start != 100 || w.Lengths[1].Start != 250 {
		t.Fatalf("lengths = %+v", w.Lengths)
	}
	wantPtr(t, "avg_speed from pool length", w.Lengths[0].AvgSpeed, 25.0/20, 1e-12)
	wantNil(t, "no pool length, no recorded speed", w.Lengths[1].AvgSpeed)
}

// TestLengthAvgSpeedFallback is F-48 PL-3: without a pool length the
// recorded average speed is used.
func TestLengthAvgSpeedFallback(t *testing.T) {
	l := rawLen(0, 20, "freestyle", 10)
	l.AvgSpeed = 1.1
	w := Normalise(Recording{Lengths: []RawLength{l, restLen(20, 40)}, Records: recs(seq(0, 40), nil)})
	wantPtr(t, "recorded speed", w.Lengths[0].AvgSpeed, 1.1, 0)
	if r := w.Lengths[1]; !r.Rest || r.AvgSpeed != nil || r.Stroke != "" || r.Time == nil || *r.Time != 20 {
		t.Errorf("rest = %+v", r)
	}
	if w.Summary.Stroke != "" {
		t.Errorf("session-less file: summary stroke %q, want none (AP-16)", w.Summary.Stroke)
	}
}

// TestSummaryStroke is F-48 PL-4.
func TestSummaryStroke(t *testing.T) {
	tests := []struct {
		name     string
		sessions []RawSession
		lengths  []RawLength
		want     string
	}{
		{"one stroke throughout, rest ignored", []RawSession{poolSession(0, 199, "mixed")},
			[]RawLength{rawLen(0, 20, "freestyle", 1), restLen(20, 40), rawLen(40, 60, "freestyle", 1)}, "freestyle"},
		{"two strokes", []RawSession{poolSession(0, 199, "")},
			[]RawLength{rawLen(0, 20, "freestyle", 1), rawLen(40, 60, "backstroke", 1)}, ""},
		{"unknown stroke", []RawSession{poolSession(0, 199, "")},
			[]RawLength{rawLen(0, 20, "freestyle", 1), rawLen(40, 60, "", 1)}, ""},
		{"all mixed", []RawSession{poolSession(0, 199, "")}, []RawLength{rawLen(0, 20, "mixed", 1)}, ""},
		{"all im", []RawSession{poolSession(0, 199, "")}, []RawLength{rawLen(0, 20, "im", 1)}, ""},
		{"no lengths: session stroke", []RawSession{poolSession(0, 199, "breaststroke")}, nil, "breaststroke"},
		{"no lengths: session mixed", []RawSession{poolSession(0, 199, "mixed")}, nil, ""},
		{"two pool swims agree", []RawSession{poolSession(0, 99, "butterfly"), poolSession(100, 199, "")},
			[]RawLength{rawLen(100, 120, "butterfly", 1)}, "butterfly"},
		{"not a pool swim", []RawSession{{Sport: "swimming", SubSport: "open_water", Start: at(0), End: at(199), Stroke: "freestyle"}}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Normalise(Recording{Sessions: tt.sessions, Lengths: tt.lengths, Records: recs(seq(0, 199), nil)})
			if w.Summary.Stroke != tt.want {
				t.Errorf("stroke = %q, want %q", w.Summary.Stroke, tt.want)
			}
		})
	}
}

// TestTrimLengths is F-48 PL-6, OQ-180, OQ-181: trimmed and cut lengths go,
// the rest are rebased, "lengths" counts the remaining active ones, the
// stroke is re-run on them.
func TestTrimLengths(t *testing.T) {
	w := Normalise(Recording{
		Sessions: []RawSession{poolSession(0, 199, "mixed")},
		Lengths: []RawLength{
			rawLen(0, 40, "backstroke", 30), rawLen(40, 80, "freestyle", 20), restLen(80, 100),
			rawLen(100, 140, "freestyle", 21), rawLen(140, 180, "backstroke", 31),
		},
		Records: recs(seq(0, 199), func(i int, r *RawRecord) { r.Distance = float64(i) }),
	})
	wantPtr(t, "device lengths", w.Summary.Lengths, 4, 0)
	out := Trim(w, 30, 150) // cuts the first and the last length
	if len(out.Lengths) != 3 || out.Lengths[0].Start != 10 || out.Lengths[0].End != 50 || !out.Lengths[1].Rest {
		t.Fatalf("lengths = %+v", out.Lengths)
	}
	for _, l := range out.Lengths {
		if l.Strokes == 30 || l.Strokes == 31 {
			t.Errorf("a cut length is kept: %+v", l)
		}
	}
	wantPtr(t, "remaining active lengths", out.Summary.Lengths, 2, 0)
	if out.Summary.Stroke != "freestyle" {
		t.Errorf("stroke = %q, want freestyle from the remaining lengths", out.Summary.Stroke)
	}
	if len(w.Lengths) != 5 {
		t.Error("Trim changed its input")
	}

	none := Trim(w, 82, 99) // only the rest is left (shorter than TrimWindow allows; Trim itself copes)
	wantNil(t, "no active length left", none.Summary.Lengths)
}

// TestTrimWithoutLengthMessages: a trimmed pool swim without length messages
// has no "Lengths" (OQ-181).
func TestTrimWithoutLengthMessages(t *testing.T) {
	w := Normalise(Recording{Sessions: []RawSession{poolSession(0, 199, "freestyle")}, Records: recs(seq(0, 199), nil)})
	wantPtr(t, "device lengths", w.Summary.Lengths, 4, 0)
	out := Trim(w, 20, 150)
	wantNil(t, "lengths", out.Summary.Lengths)
	if out.Summary.Stroke != "freestyle" {
		t.Errorf("stroke = %q, want the session's", out.Summary.Stroke)
	}
}
