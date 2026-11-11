package lifecycle

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"io/fs"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
)

// The owner operations of F-12 (architect/F-12-owner-api.md,
// ADR-025). Every one starts with lookup, the single token check: the
// handlers in httpapi have no other way to learn that a workout exists.

// ErrNotFound is the one failure of the token check (ADR-025 item 4): no row,
// a row past its deadline (F-21 X-1, drafts after 48 h too), or an absent, malformed or wrong token. Nothing tells them apart.
var ErrNotFound = errors.New("lifecycle: not found")

// InvalidSettingsError is a refused settings change; Message is a short plain
// sentence for the person editing (F-12 design, Endpoints).
type InvalidSettingsError struct{ Message string }

func (e *InvalidSettingsError) Error() string { return "lifecycle: invalid settings: " + e.Message }

func invalid(msg string) error { return &InvalidSettingsError{Message: msg} }

// Limits of the free text (F-12 design, Settings object), in Unicode code points.
const (
	MaxTitleChars       = 100
	MaxDescriptionChars = 1000
)

// ChangeHook is the port "published workout changed" (O-11), implemented by
// F-15. It receives the record before and after a settings change.
type ChangeHook interface {
	WorkoutChanged(ctx context.Context, before, after Record) error
}

// SettingsPatch holds the fields a PATCH sent; nil means not sent.
type SettingsPatch struct {
	Title         *string
	Description   *string
	HideEnds      *bool
	AllowDownload *bool
	LifetimeDays  *int
}

// ApplyPatch is the pure rule of a settings change (O-3, O-4, O-5; visibility is read-only, O-6): the
// new record, or an *InvalidSettingsError. Nothing is changed on an error.
func ApplyPatch(r Record, p SettingsPatch, now time.Time) (Record, error) {
	if p.Title != nil {
		t := trimSpace(*p.Title)
		if utf8.RuneCountInString(t) > MaxTitleChars {
			return r, invalid("The title can have at most 100 characters.")
		}
		for _, c := range t {
			if unicode.IsControl(c) {
				return r, invalid("The title cannot contain control characters.")
			}
		}
		r.Title = t
	}
	if p.Description != nil {
		d := *p.Description
		if utf8.RuneCountInString(d) > MaxDescriptionChars {
			return r, invalid("The description can have at most 1,000 characters.")
		}
		for _, c := range d {
			if unicode.IsControl(c) && c != '\n' {
				return r, invalid("The description cannot contain control characters other than line breaks.")
			}
		}
		r.Description = d
	}
	if p.HideEnds != nil {
		r.HideEnds = *p.HideEnds
	}
	if p.AllowDownload != nil {
		r.AllowDownload = *p.AllowDownload
	}
	if p.LifetimeDays != nil {
		days := *p.LifetimeDays
		if days < 1 || days > int(MaxLifetime/(24*time.Hour)) {
			return r, invalid("The lifetime must be a whole number of days from 1 to 30.")
		}
		exp := r.CreatedAt.Add(time.Duration(days) * 24 * time.Hour)
		if !exp.After(now) {
			return r, invalid("The lifetime must end in the future.")
		}
		r.ExpiresAt = exp
	}
	return r, nil
}

func trimSpace(s string) string { return strings.TrimSpace(s) }

// LifetimeDays is (expires_at - created_at) / 86,400 s (F-12 design).
func (r Record) LifetimeDays() int { return int(r.ExpiresAt.Sub(r.CreatedAt) / (24 * time.Hour)) }

// DownloadName is the file name of the owner's original (OQ-59, decision of
// 23 Oct 2026): workout-YYYY-MM-DD.fit from the local start date (UTC when the
// time zone is unknown), or workout.fit without a start time. ASCII only; the
// title is never used.
func DownloadName(r Record) string {
	if r.StartTime == nil {
		return "workout.fit"
	}
	t := r.StartTime.UTC()
	if r.TimeZoneOffsetS != nil {
		t = t.Add(time.Duration(*r.TimeZoneOffsetS) * time.Second)
	}
	return "workout-" + t.Format("2006-01-02") + ".fit"
}

// Download is an open private file for a response (O-8). The caller closes Body.
type Download struct {
	Body io.ReadCloser
	Size int64
	Name string // file name for Content-Disposition; empty: none
}

var (
	placeholderHash = HashToken("00000000000000000000000000000000")
	dummyHash       = HashToken("11111111111111111111111111111111")
)

// ValidPublicID reports whether s has the form of a public ID (22 base62
// characters); it says nothing about whether the workout exists.
func ValidPublicID(s string) bool { return isBase62Id(s) }

func isBase62Id(s string) bool {
	if len(s) != base62Width {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

// lookup is the one token check (ADR-025 item 4). For a well-formed public ID
// the same steps run in every case: the lookup, one hash, one comparison.
func (s *Service) lookup(ctx context.Context, publicID, token string) (Record, error) {
	if !isBase62Id(publicID) {
		return Record{}, ErrNotFound
	}
	rec, found, err := s.d.Repo.WorkoutByPublicID(ctx, publicID)
	if err != nil {
		if gone(ctx, err) {
			return Record{}, ErrNotFound
		}
		return Record{}, s.ownerFail("lookup", "", err)
	}
	presented, tokenOK := placeholderHash, isBase62Id(token)
	if tokenOK {
		presented = HashToken(token)
	}
	want := dummyHash
	if found {
		want = rec.EditTokenHash
	}
	match := subtle.ConstantTimeCompare(presented[:], want[:]) == 1
	if !found || !tokenOK || !match || !rec.Live(s.d.Now()) {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

// gone reports a failure caused by the request ending (the client dropped
// the connection): not a server failure, never logged, so that an anonymous
// client cannot provoke ERROR lines (ADR-025 item 8, QA-12-r1-1). The caller
// answers not found, which nobody reads.
func gone(ctx context.Context, err error) bool {
	return ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
}

func (s *Service) ownerFail(stage, id string, err error) error {
	if id == "" {
		s.d.Logger.Error("owner request failed", "stage", stage, "error", err.Error())
	} else {
		s.d.Logger.Error("owner request failed", "stage", stage, "id", id, "error", err.Error())
	}
	return &ServerError{Stage: stage, Err: err}
}

// Get returns the row of the workout (F-12: read the settings).
func (s *Service) Get(ctx context.Context, publicID, token string) (Record, error) {
	return s.lookup(ctx, publicID, token)
}

// Update applies a settings change (PATCH, O-3, O-7, O-11). The error is
// ErrNotFound, an *InvalidSettingsError or a *ServerError.
func (s *Service) Update(ctx context.Context, publicID, token string, p SettingsPatch) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return Record{}, err
	}
	sctx := context.WithoutCancel(ctx)
	var before Record
	after, found, err := s.d.Repo.UpdateWorkout(sctx, rec.ID, func(cur Record) (Record, error) {
		before = cur
		now := s.d.Now()
		if !cur.Live(now) { // the deadline passed after the lookup (QA-21-r1-2)
			return Record{}, ErrNotFound
		}
		return ApplyPatch(cur, p, now)
	})
	var ie *InvalidSettingsError
	switch {
	case errors.Is(err, ErrNotFound):
		return Record{}, ErrNotFound
	case errors.As(err, &ie):
		return Record{}, ie
	case err != nil:
		return Record{}, s.ownerFail("update", rec.ID, err)
	case !found:
		return Record{}, ErrNotFound
	}
	if after.Visibility != Draft {
		if err := s.changed(sctx, before, after); err != nil {
			return Record{}, err
		}
	}
	return after, nil
}

// ApplyPublish is the pure rule of publishing (F-15 P-1, UP-4): visibility
// becomes unlisted from any state, and published_at is set only while it is
// null, so the first publish time stays.
func ApplyPublish(r Record, now time.Time) Record {
	r.Visibility = Unlisted
	if r.PublishedAt == nil {
		t := now.UTC().Truncate(time.Second)
		r.PublishedAt = &t
	}
	return r
}

// Publish makes the workout unlisted and builds its public files (F-15 P-1,
// ADR-028): the row first, then the hook under the O-7 lock. Publishing an
// unlisted workout changes nothing on the row and rebuilds its files, so a
// failed publish can be repeated. The error is ErrNotFound or a *ServerError;
// after a ServerError the row may already be unlisted with no page.
func (s *Service) Publish(ctx context.Context, publicID, token string) (Record, error) {
	// Strangers must not queue on the write lock (QA-12-r1-2); the check is
	// repeated under the lock.
	if _, err := s.lookup(ctx, publicID, token); err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return Record{}, err
	}
	sctx := context.WithoutCancel(ctx)
	var before Record
	after, found, err := s.d.Repo.UpdateWorkout(sctx, rec.ID, func(cur Record) (Record, error) {
		before = cur
		now := s.d.Now()
		if !cur.Live(now) { // the deadline passed after the lookup (QA-21-r1-2)
			return Record{}, ErrNotFound
		}
		return ApplyPublish(cur, now), nil
	})
	switch {
	case errors.Is(err, ErrNotFound):
		return Record{}, ErrNotFound
	case err != nil:
		return Record{}, s.ownerFail("publish", rec.ID, err)
	case !found:
		return Record{}, ErrNotFound
	}
	if before.Visibility == Draft {
		s.d.Logger.Info("workout published", "id", rec.ID)
	}
	if err := s.changed(sctx, before, after); err != nil {
		return Record{}, err
	}
	return after, nil
}

// changed runs the F-15 hook for a committed change of a published workout,
// then reads the row again: if another process deleted it, or it is no
// longer a live unlisted workout, the public folder is removed (P-6, the F-12
// cross-process hand-off). Called under the O-7 lock.
func (s *Service) changed(ctx context.Context, before, after Record) error {
	if s.d.Hook != nil {
		if err := s.d.Hook.WorkoutChanged(ctx, before, after); err != nil {
			return s.ownerFail("hook", after.ID, err)
		}
	}
	if s.d.Public == nil {
		return nil
	}
	cur, found, err := s.d.Repo.WorkoutByPublicID(ctx, after.PublicID)
	if err != nil {
		return s.ownerFail("recheck", after.ID, err)
	}
	if found && cur.Visibility == Unlisted && cur.Live(s.d.Now()) {
		return nil
	}
	if _, err := s.d.Public.Remove(after.PublicID); err != nil {
		return s.ownerFail("recheck", after.ID, err)
	}
	return nil
}

// OpenDraft opens draft.bin for the owner (O-8).
func (s *Service) OpenDraft(ctx context.Context, publicID, token string) (Download, error) {
	return s.open(ctx, publicID, token, DraftName, nil)
}

// OpenOriginal opens original.fit for the owner (O-9); Name is the OQ-59 name.
func (s *Service) OpenOriginal(ctx context.Context, publicID, token string) (Download, error) {
	return s.open(ctx, publicID, token, OriginalName, DownloadName)
}

func (s *Service) open(ctx context.Context, publicID, token, file string, name func(Record) string) (Download, error) {
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return Download{}, err
	}
	body, size, err := s.d.Files.Read(rec.ID, file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// A delete may have won the race (no row), or the file was lost
			// outside the process (ADR-022 Consequences).
			exists, xerr := s.d.Repo.WorkoutExists(ctx, rec.ID)
			if xerr == nil && !exists || xerr != nil && gone(ctx, xerr) {
				return Download{}, ErrNotFound
			}
		}
		return Download{}, s.ownerFail("read", rec.ID, err)
	}
	d := Download{Body: body, Size: size}
	if name != nil {
		d.Name = name(rec)
	}
	return d, nil
}

// DeleteByOwner deletes the workout through Delete (O-10). A second call
// finds no row and answers ErrNotFound (AC 5).
func (s *Service) DeleteByOwner(ctx context.Context, publicID, token string) error {
	// Strangers must not queue on the write lock (QA-12-r1-2); the check is
	// repeated under the lock.
	if _, err := s.lookup(ctx, publicID, token); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return err
	}
	if err := s.Delete(context.WithoutCancel(ctx), rec.ID, rec.PublicID); err != nil {
		return s.ownerFail("delete", rec.ID, err)
	}
	s.d.Logger.Info("workout deleted", "id", rec.ID, "by", "owner")
	s.checkpoint(context.WithoutCancel(ctx))
	return nil
}

// DeleteByOperator deletes the workout of this row in any state through
// Delete, under the O-7 lock, then checkpoints (F-27 R-8). A failed
// checkpoint is logged and not an error.
func (s *Service) DeleteByOperator(ctx context.Context, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted, err := s.deleteRow(ctx, r.ID, r.PublicID)
	if err != nil {
		return err
	}
	if deleted { // a delete that lost the race logs nothing (QA-37-r1-1)
		s.d.Logger.Info("workout deleted", "id", r.ID, "by", "operator")
	}
	s.checkpoint(ctx)
	return nil
}

// SuggestedTitle is the UP-5 suggestion for the owner's settings object
// (F-45 ST-6): read-only, "" when none or on any failure (F-19 E-5). The town
// comes from the first position of draft.bin outside the stored hiding ranges
// (ADR-030) when hide_ends is on, so a hidden start never names its town. Never
// logged (I-5).
func (s *Service) SuggestedTitle(rec Record) string {
	if s.d.Places == nil {
		return ""
	}
	body, size, err := s.d.Files.Read(rec.ID, DraftName)
	if err != nil {
		return ""
	}
	defer body.Close()
	ra, ok := body.(io.ReaderAt)
	if !ok {
		return ""
	}
	var skip [][2]int
	if rec.HideEnds {
		skip = rec.HiddenRanges
	}
	m, lat, lon, ok, err := datafile.FirstPositionAt(ra, size, skip)
	if err != nil {
		return ""
	}
	town := ""
	if ok {
		town, _ = s.d.Places.Nearest(lat, lon)
	}
	return page.Suggest(m, town)
}
