package ingest

import (
	"errors"
	"time"
)

// Limits are the parse limits of ADR-017. Production uses DefaultLimits
// unchanged (a different value needs a revision of the ADR); tests pass
// smaller values. A zero field means the default.
type Limits struct {
	MaxBytes             int           // input size, checked before any decoding
	Timeout              time.Duration // decode deadline (per message, per sequence)
	MaxRecords           int           // record messages
	MaxSamples           int           // samples after gap filling
	MaxLaps              int
	MaxSessions          int
	MaxTimerEvents       int // event messages with event = timer
	MaxFieldDescriptions int
	MaxSequences         int // FIT sequences of a chained file
	MaxSportMessages     int // sport messages (ADR-017 rev. 1)
	MaxConcurrent        int // parse slots per Parser
}

// DefaultLimits are the ADR-017 item 4 defaults, derived from the F-02
// measurements with headroom.
var DefaultLimits = Limits{
	MaxBytes:             20 << 20,
	Timeout:              10 * time.Second,
	MaxRecords:           500_000,
	MaxSamples:           500_000,
	MaxLaps:              10_000,
	MaxSessions:          100,
	MaxTimerEvents:       100_000,
	MaxFieldDescriptions: 256,
	MaxSequences:         64,
	MaxSportMessages:     1_000,
	MaxConcurrent:        2,
}

// MaxStripGrowth is how much longer a stored original can be than its upload
// (ADR-023 rev. 5 item 7): per sequence 2 bytes and at most 130,051 bytes of
// an overlapping last record. F-30 reads stored originals with MaxBytes
// raised by it, so that our own output is never refused for its size.
const MaxStripGrowth = 64 * (2 + 130_051)

// Limit names, as they appear in Error.Limit and in the log (ADR-018).
const (
	limitBytes             = "bytes"
	limitTime              = "time"
	limitRecords           = "records"
	limitSamples           = "samples"
	limitLaps              = "laps"
	limitSessions          = "sessions"
	limitTimerEvents       = "timer_events"
	limitFieldDescriptions = "field_descriptions"
	limitSequences         = "sequences"
	limitSportMessages     = "sport_messages"
)

func (l Limits) withDefaults() Limits {
	d := DefaultLimits
	if l.MaxBytes > 0 {
		d.MaxBytes = l.MaxBytes
	}
	if l.Timeout > 0 {
		d.Timeout = l.Timeout
	}
	if l.MaxRecords > 0 {
		d.MaxRecords = l.MaxRecords
	}
	if l.MaxSamples > 0 {
		d.MaxSamples = l.MaxSamples
	}
	if l.MaxLaps > 0 {
		d.MaxLaps = l.MaxLaps
	}
	if l.MaxSessions > 0 {
		d.MaxSessions = l.MaxSessions
	}
	if l.MaxTimerEvents > 0 {
		d.MaxTimerEvents = l.MaxTimerEvents
	}
	if l.MaxFieldDescriptions > 0 {
		d.MaxFieldDescriptions = l.MaxFieldDescriptions
	}
	if l.MaxSequences > 0 {
		d.MaxSequences = l.MaxSequences
	}
	if l.MaxSportMessages > 0 {
		d.MaxSportMessages = l.MaxSportMessages
	}
	if l.MaxConcurrent > 0 {
		d.MaxConcurrent = l.MaxConcurrent
	}
	return d
}

// Cancellation causes of the decode context.
var (
	errTimeLimit       = errors.New("time limit")
	errUnsupportedType = errors.New("unsupported file type")
)

// limitError is the cancellation cause for an exceeded budget.
type limitError struct{ name string }

func (e *limitError) Error() string { return "limit " + e.name + " exceeded" }

// budget counts messages of one kind against a limit.
type budget struct {
	name  string
	limit int
	n     int
}

// add counts one message and reports whether the limit is now exceeded.
func (b *budget) add() bool {
	b.n++
	return b.n > b.limit
}
