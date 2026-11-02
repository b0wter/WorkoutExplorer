package workout

import "math"

// Time zone limits (R-9): offsets are rounded to 15 minutes and must lie
// within the range of real UTC offsets.
const (
	tzStep      = 900        // s
	tzMinOffset = -12 * 3600 // s
	tzMaxOffset = 14 * 3600  // s
)

// timeZone implements R-9 (decision 13): the fixed offset from the file's
// local timestamp, rounded to the nearest 900 s; unknown (UTC) when missing
// or out of range. No IANA zone and no lookup from coordinates.
func timeZone(local *int64) TimeZone {
	if local == nil {
		return TimeZone{}
	}
	rounded := math.Round(float64(*local)/tzStep) * tzStep
	if rounded < tzMinOffset || rounded > tzMaxOffset {
		return TimeZone{}
	}
	return TimeZone{Known: true, OffsetSeconds: int32(rounded)}
}

// workoutSport implements R-9 (sport, F-08 v1.1): sports is S, the distinct
// sports of the sessions and of the record leg. No sport gives "", one gives
// that sport (which may be ""), two or more give "multisport" (decision 12).
//
// D-17 / OQ-36 (confirmed by the user on 15 Oct 2026): a file with several
// sessions of one sport has that sport. The mixed-sports rule of R-10 stays
// on S.
func workoutSport(sports []string) string {
	switch len(sports) {
	case 0:
		return ""
	case 1:
		return sports[0]
	}
	return "multisport"
}
