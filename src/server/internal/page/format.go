package page

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Metric formatting, mirroring src/web/src/lib/format (F-13 D-6, F-15 P-11).
// Rounding follows JavaScript, not Go: formatFixed is Number.prototype.toFixed
// (an exact binary tie rounds away from zero) and jsRound is Math.round (ties
// toward +Inf). strconv rounds exact ties to even, so it is only used where
// no tie is possible.

// missingText is U+2013 EN DASH, never "0".
const missingText = "–"

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// formatElapsed: seconds as "m:ss" under one hour, else "h:mm:ss"; rounds down.
func formatElapsed(seconds float64) string {
	if !finite(seconds) {
		return missingText
	}
	negative := seconds < 0
	if negative {
		seconds = -seconds
	}
	total := int64(math.Floor(seconds))
	h, m, s := total/3600, (total%3600)/60, total%60
	var text string
	if h > 0 {
		text = strconv.FormatInt(h, 10) + ":" + pad2(m) + ":" + pad2(s)
	} else {
		text = strconv.FormatInt(m, 10) + ":" + pad2(s)
	}
	if negative {
		return "-" + text
	}
	return text
}

// toFixed is JavaScript's (x).toFixed(d) for finite |x| < 1e21: the n with
// n / 10^d closest to the exact value of x; of two, the larger magnitude.
func toFixed(x float64, d int) string {
	neg := x < 0
	if neg {
		x = -x
	}
	var text string
	if isTie(x, d) {
		// Exact tie: strconv would round to even; JavaScript rounds up.
		text = roundUpTie(x, d)
	} else {
		text = strconv.FormatFloat(x, 'f', d, 64)
	}
	if neg {
		return "-" + text
	}
	return text
}

// isTie reports whether x·10^d lies exactly halfway between two integers.
func isTie(x float64, d int) bool {
	r := new(big.Rat).SetFloat64(x)
	if r == nil {
		return false
	}
	r.Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d)), nil)))
	// frac = r - floor(r) == 1/2  <=>  2r is an odd integer.
	r.Mul(r, big.NewRat(2, 1))
	if !r.IsInt() {
		return false
	}
	return r.Num().Bit(0) == 1
}

// roundUpTie formats x (a non-negative exact tie at d decimals) rounded up.
func roundUpTie(x float64, d int) string {
	r := new(big.Rat).SetFloat64(x)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	// r = k + 1/2, so the rounded value is (2r + 1) / 2 = k + 1.
	two := new(big.Int).Mul(r.Num(), big.NewInt(2))
	two.Quo(two, r.Denom()) // 2r, an odd integer
	n := two.Add(two, big.NewInt(1))
	n.Quo(n, big.NewInt(2))
	s := n.String()
	if d == 0 {
		return s
	}
	if len(s) <= d {
		s = strings.Repeat("0", d-len(s)+1) + s
	}
	return s[:len(s)-d] + "." + s[len(s)-d:]
}

// formatFixed: v with a fixed number of decimals; NaN gives "–"; never "-0".
func formatFixed(v float64, decimals int) string {
	if !finite(v) {
		return missingText
	}
	text := toFixed(v, decimals)
	if strings.HasPrefix(text, "-") && strings.Trim(text[1:], "0.") == "" {
		return text[1:]
	}
	return text
}

// jsRound is Math.round: the nearest integer, ties toward +Inf.
func jsRound(x float64) float64 {
	if !finite(x) {
		return x
	}
	f := math.Floor(x)
	if x-f >= 0.5 {
		return f + 1
	}
	return f
}

func formatDistance(metres float64) string { return formatFixed(metres/1000, 2) }

var paceSports = map[string]bool{"running": true, "walking": true, "hiking": true, "swimming": true, "rowing": true}

func isPaceSport(sport string) bool { return paceSports[sport] }

// paceBasis: metres per pace unit and its label (metric).
func paceBasis(sport string) (float64, string) {
	switch sport {
	case "swimming":
		return 100, "/100 m"
	case "rowing":
		return 500, "/500 m"
	}
	return 1000, "/km"
}

// formatSpeedOrPace: pace, or km/h with one decimal; v in m/s; v <= 0 is "".
func formatSpeedOrPace(sport string, v float64) string {
	if !(v > 0) || !finite(v) {
		return ""
	}
	if !isPaceSport(sport) {
		return formatFixed(v*3.6, 1)
	}
	basis, _ := paceBasis(sport)
	return formatPace(basis / v)
}

// formatPace: seconds per pace unit as "m:ss".
func formatPace(seconds float64) string {
	total := int64(jsRound(seconds)) // positive: v > 0
	return strconv.FormatInt(total/60, 10) + ":" + pad2(total%60)
}

func speedOrPaceUnit(sport string) string {
	if isPaceSport(sport) {
		_, u := paceBasis(sport)
		return u
	}
	return "km/h"
}

func cadenceUnit(sport string) string {
	switch {
	case isPaceSport(sport):
		return "spm"
	case sport == "multisport":
		return "/min"
	}
	return "rpm"
}

var sportNames = map[string]string{
	"cycling": "Ride", "running": "Run", "walking": "Walk", "hiking": "Hike",
	"swimming": "Swim", "rowing": "Row", "multisport": "Multisport",
}

// sportName: "cycling" → "Ride"; unknown → capitalised with spaces; "" → "Workout".
func sportName(sport string) string {
	if n, ok := sportNames[sport]; ok {
		return n
	}
	words := jsTrim(strings.ReplaceAll(sport, "_", " "))
	if words == "" {
		return "Workout"
	}
	r := []rune(words)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

func sportIcon(sport string) string {
	if _, ok := sportNames[sport]; ok && sport != "multisport" {
		return sport
	}
	return "generic"
}

// jsTrim is String.prototype.trim: JavaScript's white space and line
// terminators. It differs from strings.TrimSpace in U+0085 (not trimmed) and
// U+FEFF (trimmed).
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		if r == 0x85 {
			return false
		}
		return r == 0xFEFF || unicode.IsSpace(r)
	})
}

var months = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// shifted is the wall clock at an offset, read as UTC; ok is false for a
// time that does not parse (JavaScript would print NaN; never in a data file).
func shifted(rfc3339 string, offsetSeconds int64) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC().Add(time.Duration(offsetSeconds) * time.Second), true
}

// formatDate: "9 Oct 2026" in the zone given by its offset.
func formatDate(rfc3339 string, offsetSeconds int64) string {
	d, ok := shifted(rfc3339, offsetSeconds)
	if !ok {
		return "NaN undefined NaN"
	}
	return strconv.Itoa(d.Day()) + " " + months[d.Month()-1] + " " + strconv.Itoa(d.Year())
}

// formatClock: "07:30", 24-hour.
func formatClock(rfc3339 string, offsetSeconds int64) string {
	d, ok := shifted(rfc3339, offsetSeconds)
	if !ok {
		return "NaN:NaN"
	}
	return pad2(int64(d.Hour())) + ":" + pad2(int64(d.Minute()))
}

// formatLocalStart: "9 Oct 2026, 07:30", or with " UTC" when the zone is unknown.
func formatLocalStart(rfc3339 string, known bool, offsetSeconds int64) string {
	if !known {
		offsetSeconds = 0
	}
	s := formatDate(rfc3339, offsetSeconds) + ", " + formatClock(rfc3339, offsetSeconds)
	if !known {
		s += " UTC"
	}
	return s
}
