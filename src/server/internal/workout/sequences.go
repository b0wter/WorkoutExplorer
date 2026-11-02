package workout

import "math"

// continueDistance implements R-12 (F-08 design, QA-08-r1-3): each FIT
// sequence of a chained file is a complete activity whose device distance
// starts again near 0. A later sequence whose first valid distance is below
// half the previous sequence's last distance (v1.2 midpoint rule, D-14) is
// offset by that distance, so the stream is cumulative over the whole
// workout. The first value is either near 0 (the writer restarted its count)
// or near prev (it carried the count on); the midpoint assigns it to
// whichever it is closer to and needs no tolerance constant, so boundary
// smoothing of a carried count is absorbed. Exactly prev/2 does not
// restart. starts holds the index into records of the first record of
// sequence 2, 3, ... It returns records itself when nothing changes, else an
// adjusted copy; the caller's slice is never modified.
func continueDistance(records []RawRecord, starts []int) []RawRecord {
	if len(starts) == 0 {
		return records
	}
	validDistance := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	var out []RawRecord
	prev, havePrev := 0.0, false
	from := 0
	for j := 0; j <= len(starts); j++ {
		to := len(records)
		if j < len(starts) {
			to = min(max(starts[j], from), len(records))
		}
		rng := records[from:to]
		first, haveFirst := 0.0, false
		for _, r := range rng {
			if validDistance(r.Distance) {
				first, haveFirst = r.Distance, true
				break
			}
		}
		base := 0.0
		if havePrev && haveFirst && first < prev/2 {
			base = prev
		}
		if base != 0 && out == nil {
			out = append([]RawRecord(nil), records...)
		}
		last, haveLast := 0.0, false
		for i := from; i < to; i++ {
			d := records[i].Distance
			if !validDistance(d) {
				continue
			}
			if base != 0 {
				out[i].Distance = d + base
			}
			last, haveLast = d+base, true
		}
		if haveLast {
			prev, havePrev = last, true
		}
		from = to
	}
	if out == nil {
		return records
	}
	return out
}

// recordSequences returns the number of FIT sequences that delivered at least
// one record with a valid time (R-10.E, D-20): the ranges of rec.Records split
// at rec.SequenceStarts (as in R-12) that hold such a record. A skipped or
// empty sequence counts 0.
func recordSequences(rec Recording) int {
	n := 0
	from := 0
	for j := 0; j <= len(rec.SequenceStarts); j++ {
		to := len(rec.Records)
		if j < len(rec.SequenceStarts) {
			to = min(max(rec.SequenceStarts[j], from), len(rec.Records))
		}
		for i := from; i < to; i++ {
			if !rec.Records[i].At.IsZero() {
				n++
				break
			}
		}
		from = to
	}
	return n
}
