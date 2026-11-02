package lifecycle

import (
	"time"

	"workout-explorer/internal/workout"
)

// Visibility is the publication state of a workout (SH-2). Only publish
// changes it, draft to unlisted, and nothing changes it back (F-12 O-6).
type Visibility string

// Visibility values.
const (
	Draft    Visibility = "draft"
	Unlisted Visibility = "unlisted"
)

// Lifetimes (UP-7). DraftLifetime is counted in hours, not calendar days (F-21 X-1).
const (
	DefaultLifetime = 7 * 24 * time.Hour
	MaxLifetime     = 30 * 24 * time.Hour
	DraftLifetime   = 48 * time.Hour
)

// Deadline is the one rule of when a workout ends (F-21 X-1): expires_at for
// an unlisted workout, and for a draft the earlier of expires_at and
// created_at + DraftLifetime. The store's queries are its SQL form.
func (r Record) Deadline() time.Time {
	if d := r.CreatedAt.Add(DraftLifetime); r.Visibility == Draft && d.Before(r.ExpiresAt) {
		return d
	}
	return r.ExpiresAt
}

// Live reports whether the workout is still alive at now: before its
// deadline. From the deadline on it is due for deletion.
func (r Record) Live(now time.Time) bool { return now.Before(r.Deadline()) }

// Record is one row of the workouts table (F-09 design, Schema).
type Record struct {
	ID              string
	PublicID        string
	EditTokenHash   [32]byte
	Title           string
	Description     string
	Sport           string
	StartTime       *time.Time
	TimeZoneOffsetS *int
	Visibility      Visibility
	HideEnds        bool
	AllowDownload   bool
	SummaryJSON     string
	DataEndS        *int
	CreatedAt       time.Time
	PublishedAt     *time.Time
	ExpiresAt       time.Time
	// HiddenRanges is workout.HiddenRanges of the upload (ADR-030): display
	// only, never read by the public build.
	HiddenRanges [][2]int
}

// NewDraft is the row of a fresh upload: a draft with hiding and download off
// that expires DefaultLifetime after now (SH-2, SH-4, SH-8, UP-7). It is pure.
// summary is datafile.SummaryJSON of w.
func NewDraft(w workout.Workout, summary []byte, id Identity, now time.Time) Record {
	created := now.UTC().Truncate(time.Second)
	r := Record{
		ID:            id.ID,
		PublicID:      id.PublicID,
		EditTokenHash: id.TokenHash,
		Sport:         w.Sport,
		Visibility:    Draft,
		SummaryJSON:   string(summary),
		CreatedAt:     created,
		ExpiresAt:     created.Add(DefaultLifetime),
		HiddenRanges:  workout.HiddenRanges(w),
	}
	if !w.StartTime.IsZero() {
		st := w.StartTime.UTC()
		r.StartTime = &st
	}
	if w.TimeZone.Known {
		off := int(w.TimeZone.OffsetSeconds)
		r.TimeZoneOffsetS = &off
	}
	if w.Integrity.Truncated {
		end := int(w.Integrity.DataEnd)
		r.DataEndS = &end
	}
	return r
}
