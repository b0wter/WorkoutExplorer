package lifecycle

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/page"
	"workout-explorer/internal/workout"
)

// The owner operations of F-12 (architect/F-12-owner-api.md,
// ADR-025). Every one starts with lookup, the single token check: the
// handlers in httpapi have no other way to learn that a workout exists.

// ErrNotFound is the one failure of the token check (ADR-025 item 4): no row,
// a row past its deadline (F-21 X-1, drafts after 48 h too), or an absent, malformed or wrong token. Nothing tells them apart.
var ErrNotFound = errors.New("lifecycle: not found")

// ErrBusy: another owner write of the same workout is in flight (ADR-061 item 4).
var ErrBusy = errors.New("lifecycle: another change of this workout is in progress")

// InvalidSettingsError is a refused settings change; Message is a short plain
// sentence for the person editing (F-12 design, Endpoints).
type InvalidSettingsError struct{ Message string }

func (e *InvalidSettingsError) Error() string { return "lifecycle: invalid settings: " + e.Message }

func invalid(msg string) error { return &InvalidSettingsError{Message: msg} }

// MsgBadHiding is the refusal of a hiding radius (F-55 HR-4), also for a
// value of the wrong type.
const MsgBadHiding = "The start and end hiding must each be a whole number of metres from 0 to 2,000 in steps of 50."

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
	Title       *string
	Description *string
	// HideStartM and HideEndM are F-55 HR-4's radii in metres.
	HideStartM, HideEndM *int
	LifetimeDays         *int
	// TrimStart and TrimEnd are the instants of F-47 TR-3; Set with a nil At
	// is null: that end is not trimmed.
	TrimStart, TrimEnd OptTime
	// ZonesMaxHeartRate and ZonesThresholdPower are F-49 ZN-4; Set with a nil
	// V is null: use the file's values.
	ZonesMaxHeartRate, ZonesThresholdPower OptInt
	// Pins replaces the whole set (F-56 PN-2).
	Pins OptPins
	// AddedLapBoundaries replaces the whole set (F-65 AL-2).
	AddedLapBoundaries OptTimes
	// RemoveAutoLaps is F-66 RA-2.
	RemoveAutoLaps *bool
}

// OptInt is an optional integer that may be sent as null.
type OptInt struct {
	Set bool
	V   *int
}

// OptTime is an optional time that may be sent as null.
type OptTime struct {
	Set bool
	At  *time.Time
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
	for _, h := range []struct{ v, dst *int }{{p.HideStartM, &r.HideStartM}, {p.HideEndM, &r.HideEndM}} {
		if h.v == nil {
			continue
		}
		if !workout.ValidHideRadius(*h.v) {
			return r, invalid(MsgBadHiding)
		}
		*h.dst = *h.v
	}
	if p.TrimStart.Set {
		r.TrimStartAt = p.TrimStart.At
	}
	if p.TrimEnd.Set {
		r.TrimEndAt = p.TrimEnd.At
	}
	if p.ZonesMaxHeartRate.Set {
		if v := p.ZonesMaxHeartRate.V; v != nil && (*v < workout.MinMaxHeartRate || *v > workout.MaxMaxHeartRate) {
			return r, invalid("The max heart rate must be a whole number from 100 to 250 bpm.")
		}
		r.ZonesMaxHeartRate = p.ZonesMaxHeartRate.V
	}
	if p.ZonesThresholdPower.Set {
		if v := p.ZonesThresholdPower.V; v != nil && (*v < workout.MinThresholdPower || *v > workout.MaxThresholdPower) {
			return r, invalid("The threshold power must be a whole number from 50 to 1,000 W.")
		}
		r.ZonesThresholdPower = p.ZonesThresholdPower.V
	}
	if p.RemoveAutoLaps != nil {
		r.RemoveAutoLaps = *p.RemoveAutoLaps
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

// writeFail is ownerFail for the stages of a write: if the workout's row is
// gone, a delete won the race and the answer is not found, with no ERROR line
// (ADR-061 item 5).
func (s *Service) writeFail(ctx context.Context, stage, id string, err error) error {
	if exists, xerr := s.d.Repo.WorkoutExists(ctx, id); xerr == nil && !exists {
		return ErrNotFound
	}
	return s.ownerFail(stage, id, err)
}

// claim takes the one-write-per-workout claim of ADR-061 item 4.
func (s *Service) claim(id string) bool {
	_, loaded := s.claims.LoadOrStore(id, struct{}{})
	return !loaded
}

// Get returns the row of the workout (F-12: read the settings).
func (s *Service) Get(ctx context.Context, publicID, token string) (Record, error) {
	return s.lookup(ctx, publicID, token)
}

// Update applies a settings change (PATCH, O-3, O-7, O-11). The error is
// ErrNotFound, an *InvalidSettingsError or a *ServerError.
func (s *Service) Update(ctx context.Context, publicID, token string, p SettingsPatch) (Record, error) {
	rec, err := s.lookup(ctx, publicID, token) // strangers neither claim nor queue (QA-12-r1-2)
	if err != nil {
		return Record{}, err
	}
	if !s.claim(rec.ID) {
		return Record{}, ErrBusy
	}
	defer s.claims.Delete(rec.ID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, err = s.lookup(ctx, publicID, token); err != nil {
		return Record{}, err
	}
	sctx := context.WithoutCancel(ctx)
	if p.TrimStart.Set || p.TrimEnd.Set {
		if p, err = s.resolveTrim(sctx, rec, p); err != nil {
			return Record{}, err
		}
	}
	// PN-5, AL-5: pins and added lap boundaries are judged against the
	// settings the patch produces.
	eff, err := ApplyPatch(rec, p, s.d.Now())
	if err != nil {
		return Record{}, err
	}
	pins, laps, err := s.resolveEdits(sctx, rec, eff, p)
	if err != nil {
		return Record{}, err
	}
	var before Record
	after, found, err := s.d.Repo.UpdateWorkout(sctx, rec.ID, func(cur Record) (Record, error) {
		before = cur
		now := s.d.Now()
		if !cur.Live(now) { // the deadline passed after the lookup (QA-21-r1-2)
			return Record{}, ErrNotFound
		}
		next, err := ApplyPatch(cur, p, now)
		next.Pins, next.AddedLapBoundaries = pins, laps
		return next, err
	})
	var ie *InvalidSettingsError
	switch {
	case errors.Is(err, ErrNotFound):
		return Record{}, ErrNotFound
	case errors.As(err, &ie):
		return Record{}, ie
	case err != nil:
		return Record{}, s.writeFail(sctx, "update", rec.ID, err)
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
	// Strangers must not claim or queue (QA-12-r1-2); the check is repeated
	// under the lock.
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return Record{}, err
	}
	if !s.claim(rec.ID) {
		return Record{}, ErrBusy
	}
	defer s.claims.Delete(rec.ID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, err = s.lookup(ctx, publicID, token); err != nil {
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
		return Record{}, s.writeFail(sctx, "publish", rec.ID, err)
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
	var herr error
	if s.d.Hook != nil {
		herr = s.d.Hook.WorkoutChanged(ctx, before, after)
	}
	if s.d.Public != nil {
		// Also after a failed hook: a build that raced a delete may have
		// written a folder (ADR-061 item 5).
		cur, found, err := s.d.Repo.WorkoutByPublicID(ctx, after.PublicID)
		switch {
		case err != nil:
			if herr == nil {
				return s.ownerFail("recheck", after.ID, err)
			}
		case !(found && cur.Visibility == Unlisted && cur.Live(s.d.Now())):
			if _, err := s.d.Public.Remove(after.PublicID); err != nil && herr == nil {
				return s.ownerFail("recheck", after.ID, err)
			}
		}
	}
	if herr != nil {
		return s.writeFail(ctx, "hook", after.ID, herr)
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
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return err
	}
	// No O-7 (ADR-061 item 2). A call that did not remove the row lost to
	// another delete: not found (AC 5).
	deleted, err := s.deleteRow(context.WithoutCancel(ctx), rec.ID, rec.PublicID)
	if !deleted {
		return ErrNotFound
	}
	if err != nil {
		return s.ownerFail("delete", rec.ID, err)
	}
	s.d.Logger.Info("workout deleted", "id", rec.ID, "by", "owner")
	s.checkpoint(context.WithoutCancel(ctx))
	return nil
}

// DeleteByOperator deletes the workout of this row in any state through
// Delete, without the O-7 lock (ADR-061), then checkpoints (F-27 R-8). A failed
// checkpoint is logged and not an error.
func (s *Service) DeleteByOperator(ctx context.Context, r Record) error {
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
// comes from the first position of draft.bin outside the hidden sections at
// the stored radii (F-55 HR-7, from HidingSteps), so a hidden start never
// names its town; with a radius above 0 and no steps yet there is no town.
// Never logged (I-5).
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
	skip := hiddenSkip(rec)
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

// hiddenSkip is the samples of draft.bin hidden at rec's radii, as inclusive
// ranges for datafile.FirstPositionAt: everything when the route is fully
// hidden or the steps are missing with a radius above 0.
func hiddenSkip(rec Record) [][2]int {
	if rec.HideStartM == 0 && rec.HideEndM == 0 {
		return nil
	}
	all := [][2]int{{0, math.MaxInt32}}
	if rec.HidingSteps == nil {
		return all
	}
	from, to, fully := rec.HidingSteps.Window(rec.HideStartM, rec.HideEndM)
	if fully {
		return all
	}
	return [][2]int{{0, from - 1}, {to + 1, math.MaxInt32}}
}

// resolveTrim checks a trim request against a fresh parse (F-47 TR-4) and
// returns the patch with the snapped instants. Under the O-7 lock.
func (s *Service) resolveTrim(ctx context.Context, rec Record, p SettingsPatch) (SettingsPatch, error) {
	start, end := rec.TrimStartAt, rec.TrimEndAt
	if p.TrimStart.Set {
		start = p.TrimStart.At
	}
	if p.TrimEnd.Set {
		end = p.TrimEnd.At
	}
	if start == nil && end == nil { // no trim: nothing to check
		p.TrimStart, p.TrimEnd = OptTime{Set: true}, OptTime{Set: true}
		return p, nil
	}
	if s.d.Trimmer == nil {
		return p, s.ownerFail("trim", rec.ID, errors.New("no trimmer"))
	}
	a, b, err := s.d.Trimmer.ResolveTrim(ctx, rec, start, end)
	switch {
	case errors.Is(err, ErrTrimTooShort):
		return p, invalid("At least 1 minute of the workout must remain.")
	case err != nil:
		return p, s.writeFail(ctx, "trim", rec.ID, err)
	}
	p.TrimStart, p.TrimEnd = OptTime{Set: true, At: a}, OptTime{Set: true, At: b}
	return p, nil
}

// OwnerView is what the owner's settings object shows beyond the row: the
// hiding steps and the suggested title, and with a trim stored the trimmed
// sport and summary (F-47 TR-8, ADR-047 item 3).
type OwnerView struct {
	Trimmed   *TrimmedView
	Steps     *workout.HidingSteps
	Suggested string
}

// TrimmedView is the sport and summary as the public metadata has them.
type TrimmedView struct {
	Sport   string          `json:"sport"`
	Summary json.RawMessage `json:"summary"`
}

// View returns the owner view of rec. Without a trim it is the stored
// steps and ST-6's suggestion, with no parse. With a trim it is computed
// per read from one parse in the owner-read slot, waiting for it while the request lasts; any failure gives no trimmed
// values, no steps and no suggestion, never an error.
func (s *Service) View(ctx context.Context, rec Record) OwnerView {
	if (rec.TrimStartAt == nil && rec.TrimEndAt == nil) || s.d.Trimmer == nil {
		return OwnerView{Steps: rec.HidingSteps, Suggested: s.SuggestedTitle(rec)}
	}
	select { // taken before the original is read (ADR-061 item 3)
	case s.ownerRead <- struct{}{}:
		defer func() { <-s.ownerRead }()
	case <-ctx.Done():
		return OwnerView{}
	}
	v, err := s.d.Trimmer.TrimmedView(ctx, rec)
	if err != nil {
		return OwnerView{}
	}
	return v
}
