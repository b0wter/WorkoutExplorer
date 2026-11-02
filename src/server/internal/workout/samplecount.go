package workout

import (
	"math"
	"slices"
)

// SampleCount returns exactly len(Normalise(rec).Samples.Offset) without
// building the samples (F-08 design, Model changes): the valid record times
// as offsets (R-1), sorted, equal seconds merged (R-2), plus d-1 for every
// step of 2 to 4 s (R-3). Memory: one int32 per record. ingest calls it to
// enforce the sample budget before Normalise allocates.
func SampleCount(rec Recording) int {
	start := startTime(rec)
	if start.IsZero() || len(rec.Records) == 0 {
		return 0
	}
	offs := make([]int32, 0, len(rec.Records))
	for i := range rec.Records {
		if !rec.Records[i].At.IsZero() {
			offs = append(offs, offsetOf(start, rec.Records[i].At))
		}
	}
	slices.Sort(offs)
	n := 0
	var prev int32
	for i, o := range offs {
		if i > 0 && o == prev {
			continue
		}
		if n > 0 {
			if d := int64(o) - int64(prev); d >= minFillGap && d <= maxFillGap {
				n += int(d) - 1
			}
		}
		n++
		prev = o
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return n
}
