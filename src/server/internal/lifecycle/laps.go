package lifecycle

import (
	"encoding/json"
	"slices"
	"time"

	"workout-explorer/internal/workout"
)

// Added lap boundaries (F-65 AL-1, AL-2, AL-5; ADR-060). The row holds the
// whole set; the rules on samples are workout's, reached through
// Trimmer.CheckEdits.

// Refusals of added_lap_boundaries (F-12 settings object), one per case.
const (
	MsgBadLaps       = "added_lap_boundaries must be a list of at most 100 instants."
	MsgLapOutside    = "An added lap boundary must lie within the recording."
	MsgLapTwice      = "Two added lap boundaries fall on the same point."
	MsgLapOnBoundary = "There is a lap boundary here already."
	MsgLapTrimmed    = "An added lap boundary cannot lie in a trimmed part."
)

// OptTimes is the added lap boundaries of a PATCH; Set with none clears them.
type OptTimes struct {
	Set bool
	V   []time.Time
}

// LapCheck is the verdict of Trimmer.CheckEdits on one added lap boundary:
// Resolved is the instant of the nearest sample after the pool snap (AL-4);
// Outside: the instant is outside the recording; OnBoundary: the sample is a
// device or implicit lap end or an end of the trim window (AL-3); Kept: it
// lies strictly inside the trim window.
type LapCheck struct {
	Resolved                  time.Time
	Outside, OnBoundary, Kept bool
}

// lapCandidates is the first half of AL-5: the boundaries to judge and
// whether they need judging (a parse): only when they are sent, or the trim
// changes on a row with boundaries. A radius change never touches them.
func lapCandidates(rec Record, p SettingsPatch) (cand []time.Time, judge bool, err error) {
	if !p.AddedLapBoundaries.Set && (len(rec.AddedLapBoundaries) == 0 || !p.TrimStart.Set && !p.TrimEnd.Set) {
		return rec.AddedLapBoundaries, false, nil
	}
	cand = rec.AddedLapBoundaries
	if p.AddedLapBoundaries.Set {
		cand = p.AddedLapBoundaries.V
		if len(cand) > workout.MaxAddedLapBoundaries {
			return nil, false, invalid(MsgBadLaps)
		}
	}
	return cand, len(cand) > 0, nil
}

// judgeLaps is the second half of AL-2 and AL-5: the set to store, or an
// *InvalidSettingsError, all or nothing. A stored entry the trim leaves
// outside its window is deleted, sent or not; a stored entry a parser change
// made invalid is dropped when not sent.
func judgeLaps(rec Record, p SettingsPatch, checks []LapCheck) ([]time.Time, error) {
	var out []time.Time
	for _, c := range checks {
		stored := !c.Outside && slices.ContainsFunc(rec.AddedLapBoundaries, c.Resolved.Equal)
		refusal := ""
		switch {
		case c.Outside:
			refusal = MsgLapOutside
		case !c.Kept && stored: // the trim removed it (OQ-237), sent or not
			continue
		case c.OnBoundary: // a lap end or an end of the trim window
			refusal = MsgLapOnBoundary
		case !c.Kept:
			refusal = MsgLapTrimmed
		case slices.ContainsFunc(out, c.Resolved.Equal):
			refusal = MsgLapTwice
		default:
			out = append(out, c.Resolved)
			continue
		}
		if p.AddedLapBoundaries.Set {
			return nil, invalid(refusal)
		}
		// Not sent: a stored entry a parser change made invalid is dropped.
	}
	slices.SortFunc(out, time.Time.Compare)
	return out, nil
}

// EncodeLapBoundaries is the added_lap_boundaries column: a JSON array of
// Unix seconds, "" for none (stored as NULL).
func EncodeLapBoundaries(ts []time.Time) string {
	if len(ts) == 0 {
		return ""
	}
	out := make([]int64, len(ts))
	for i, t := range ts {
		out[i] = t.Unix()
	}
	b, _ := json.Marshal(out) // integers always marshal
	return string(b)
}

// DecodeLapBoundaries reads the column; anything unreadable is an error.
func DecodeLapBoundaries(s string) ([]time.Time, error) {
	if s == "" {
		return nil, nil
	}
	var in []int64
	if err := json.Unmarshal([]byte(s), &in); err != nil {
		return nil, err
	}
	out := make([]time.Time, len(in))
	for i, v := range in {
		out[i] = time.Unix(v, 0).UTC()
	}
	return out, nil
}
