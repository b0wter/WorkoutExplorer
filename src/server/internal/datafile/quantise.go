package datafile

import (
	"encoding/json"
	"math"
	"strconv"
)

// quantise returns clamp(round(v × factor)) and true, or false when v is
// missing (NaN or ±Inf). Rounding is half away from zero (Format §3, W-4).
// The clamp happens in float64, so a huge v cannot overflow int64.
func quantise(v, factor float64, lo, hi int64) (int64, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return clamp(math.Round(v*factor), lo, hi), true
}

// clamp limits r to [lo, hi] and converts it to int64.
func clamp(r float64, lo, hi int64) int64 {
	switch {
	case r < float64(lo):
		return lo
	case r > float64(hi):
		return hi
	}
	return int64(r)
}

// deltas turns absolute quantised values into a delta column (Format §3):
// the sentinel where a sample is missing, otherwise the difference to the
// last valid value (or the value itself for the first valid one).
func deltas(q []int64, valid []bool, sentinel int64) []int64 {
	out := make([]int64, len(q))
	var prev int64
	for i := range q {
		if !valid[i] {
			out[i] = sentinel
			continue
		}
		out[i] = q[i] - prev
		prev = q[i]
	}
	return out
}

// undelta is the normative reader of a delta column (Format §3): the running
// sum skips the sentinel. It returns the absolute values (0 where missing).
func undelta(d []int64, sentinel int64) ([]int64, []bool) {
	out := make([]int64, len(d))
	valid := make([]bool, len(d))
	var acc int64
	for i, x := range d {
		if x == sentinel {
			continue
		}
		acc += x
		out[i], valid[i] = acc, true
	}
	return out, valid
}

// total formats a device total for the metadata (Format §5.2): rounded to 3
// decimals half away from zero, shortest exact form, -0 as 0. nil and
// non-finite values give "" (key omitted).
func total(p *float64) json.Number {
	if p == nil || math.IsNaN(*p) || math.IsInf(*p, 0) {
		return ""
	}
	r := math.Round(*p*1000) / 1000
	if r == 0 {
		r = 0 // drops the sign of -0
	}
	return json.Number(strconv.FormatFloat(r, 'f', -1, 64))
}
