package httpapi

import (
	"sync"
	"time"
)

// limiter is a sliding-log rate limiter over opaque keys (ADR-021 items 7,
// 8). Per key it keeps the times of the allowed attempts inside the window,
// at most limit. It is reused by F-27 with its own instance.
//
// Representation (ADR-021 rev. 3 item 8): a time is stored as whole seconds
// since epoch, rounded up, in a uint32; an entry s is live while
// now < epoch + s*1s + window. So an attempt counts for at most 1 s longer
// than the window, never shorter. Each key has one array of cap its limit,
// allocated once; expired entries are removed by copying the live ones down.
type limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	epoch   time.Time // now() at newLimiter
	window  time.Duration
	maxKeys int

	hits      map[uint64]*keyLog
	lastSweep time.Time
	onFull    func(keys int) // called at most once per sweep period when the table is full of live keys
	fullSent  bool
}

type keyLog struct {
	times   []uint32 // whole seconds since epoch, rounded up; cap == the key's limit
	refused bool     // refused since the last allowed attempt
}

const (
	sweepEvery     = 10 * time.Minute
	fullSweepEvery = time.Minute
)

func newLimiter(window time.Duration, maxKeys int, now func() time.Time) *limiter {
	if now == nil {
		now = time.Now
	}
	t := now()
	return &limiter{now: now, epoch: t, window: window, maxKeys: maxKeys, hits: map[uint64]*keyLog{}, lastSweep: t}
}

// stamp is t in whole seconds since the epoch, rounded up (a time before the
// epoch, which only a clock stepping backwards produces, counts as 0).
func (l *limiter) stamp(t time.Time) uint32 {
	d := t.Sub(l.epoch)
	if d <= 0 {
		return 0
	}
	return uint32((d + time.Second - 1) / time.Second)
}

// expiry is the instant at which the entry s stops counting.
func (l *limiter) expiry(s uint32) time.Time {
	return l.epoch.Add(time.Duration(s)*time.Second + l.window)
}

// allow counts the attempt if it is allowed. retryAfter is set when refused;
// firstRefusal is true for the first refusal of a key since its last allowed
// attempt. limit is the key's own (IPv4 and IPv6 differ, ADR-021 rev. 6). A refused attempt does not extend the window. A new key is refused
// (fail closed) when the table is full of live keys.
func (l *limiter) allow(key uint64, limit int) (ok bool, retryAfter time.Duration, firstRefusal bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= sweepEvery {
		l.sweep(now)
	}
	k := l.hits[key]
	if k == nil {
		if len(l.hits) >= l.maxKeys && now.Sub(l.lastSweep) >= fullSweepEvery {
			l.sweep(now) // at most once a minute, so a full table is not an O(n) cost per request
		}
		if len(l.hits) >= l.maxKeys {
			if !l.fullSent && l.onFull != nil {
				l.fullSent = true
				l.onFull(len(l.hits))
			}
			return false, time.Minute, false
		}
		k = &keyLog{times: make([]uint32, 0, limit)}
		l.hits[key] = k
	}
	i := 0
	for i < len(k.times) && !now.Before(l.expiry(k.times[i])) {
		i++
	}
	if i > 0 {
		n := copy(k.times, k.times[i:]) // never k.times = k.times[i:]: append must not reallocate
		k.times = k.times[:n]
	}
	if len(k.times) >= limit {
		first := !k.refused
		k.refused = true
		return false, l.expiry(k.times[0]).Sub(now), first
	}
	k.times = append(k.times, l.stamp(now))
	k.refused = false
	return true, 0, false
}

// sweep drops every key whose attempts have all left the window.
func (l *limiter) sweep(now time.Time) {
	for key, k := range l.hits {
		if len(k.times) == 0 || !now.Before(l.expiry(k.times[len(k.times)-1])) {
			delete(l.hits, key)
		}
	}
	l.lastSweep = now
	l.fullSent = false
}
