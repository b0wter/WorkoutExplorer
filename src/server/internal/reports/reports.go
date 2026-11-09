// Package reports is filing a report on a published workout (F-27, INF-2):
// the pure report rule (reasons, message) and the operation behind the Store
// port that internal/store implements. A stored report is handed to the
// Notifier port (ADR-039). Its delete keys (F-37, ADR-042) are issued,
// looked up and redeemed here; a redeemed key deletes through the Deleter
// port, the operator delete. Nothing else here changes a workout.
package reports

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/page"
)

// Limits (F-27 R-3, R-4).
const (
	MaxMessageChars = 1000
	MaxPerWorkout   = 10
	Retention       = 30 * 24 * time.Hour
	publicIDWidth   = 22
)

// Reasons are the four INF-2 reasons; the values are the API's and the table's.
var Reasons = []string{"spam", "illegal_content", "personal_data", "other"}

// Labels are the reasons in words (INF-4, F-37 DK-10), for the notification
// and the delete link's lookup.
var Labels = map[string]string{"spam": "Spam", "illegal_content": "Illegal content", "personal_data": "Personal data", "other": "Other"}

// DeletePath is the confirmation page; the delete link is
// <base URL>DeletePath#<key> (F-37 DK-1).
const DeletePath = "/report-delete"

// ErrLinkInvalid: the delete key is malformed, unknown, used or expired, or
// its workout is gone (F-37 DK-12: one answer for all).
var ErrLinkInvalid = errors.New("reports: delete link invalid")

// ErrNotFound: no unlisted, live workout with this public ID (also a malformed ID).
var ErrNotFound = errors.New("reports: not found")

// InvalidError is a refused report; Message is a plain sentence for F-28.
type InvalidError struct{ Message string }

func (e *InvalidError) Error() string { return "reports: invalid: " + e.Message }

// The refusal sentences of the report rule.
const (
	MsgReason      = "Please choose one of the reasons for the report."
	MsgMessageLong = "The message can have at most 1,000 characters."
	MsgMessageCtrl = "The message cannot contain control characters other than line breaks."
)

// Store is the reports table (internal/store). Delete keys pass only as
// their SHA-256 (ADR-042 item 1).
type Store interface {
	// AddReport: see store.DB.AddReport. id 0 with found true: not stored (cap).
	AddReport(ctx context.Context, publicID, reason, message string, now time.Time, max int) (id int64, workoutID string, found bool, err error)
	DeleteReportsFiledBy(ctx context.Context, t time.Time) (int64, error)
	SetDeleteKeyHash(ctx context.Context, reportID int64, hash [32]byte) (bool, error)
	ClearDeleteKeys(ctx context.Context) (int64, error)
	// ReportByDeleteKey and ConsumeDeleteKey match a key only while it is
	// valid (DK-4): report filed after filedAfter, workout row alive at now.
	ReportByDeleteKey(ctx context.Context, hash [32]byte, filedAfter, now time.Time) (KeyedReport, bool, error)
	ConsumeDeleteKey(ctx context.Context, hash [32]byte, filedAfter, now time.Time) (id, publicID string, ok bool, err error)
}

// KeyedReport is the report a delete key points at, the number of reports on
// its workout, and the workout's Title, Sport, StartTime, TimeZoneOffsetS
// and ExpiresAt (nothing else of the row is read).
type KeyedReport struct {
	ID      int64
	Reason  string
	FiledAt time.Time
	Count   int
	Workout lifecycle.Record
}

// KeyInfo is what a delete key shows (F-37 DK-7), nothing more.
type KeyInfo struct {
	Title       string // stored, or the page's default title
	StartDate   string // the page header's start text, in the workout's zone
	ReportID    int64
	Reason      string // label
	FiledAt     time.Time
	ExpiresAt   time.Time
	ReportCount int
}

// Notifier learns of each stored report (ADR-039): its number, reason label
// and absolute viewing link. Notify must not block.
type Notifier interface {
	Notify(id int64, label, viewURL string)
}

// Deleter is the operator delete; *lifecycle.Service satisfies it (F-27 R-8).
type Deleter interface {
	DeleteByOperator(ctx context.Context, r lifecycle.Record) error
}

// Deps is what a Service needs.
type Deps struct {
	Store    Store
	Notifier Notifier         // nil: notifications off
	Deleter  Deleter          // nil: RedeemDeleteKey panics (tests without keys)
	BaseURL  string           // origin of absolute links, no trailing slash
	Now      func() time.Time // nil: time.Now
	Logger   *slog.Logger     // nil: no log lines
}

// Service files reports and handles their delete keys.
type Service struct {
	d    Deps
	rand io.Reader  // crypto/rand; tests replace it
	mu   sync.Mutex // one redeem at a time: a second key of a deleted workout finds no report
}

// New returns a Service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d, rand: rand.Reader}
}

// Check is the report rule (R-5): the reason is one of Reasons; the message
// is trimmed, at most MaxMessageChars code points, and has no control
// character other than line feed. It returns the message to store.
func Check(reason, message string) (string, error) {
	ok := false
	for _, r := range Reasons {
		ok = ok || reason == r
	}
	if !ok {
		return "", &InvalidError{MsgReason}
	}
	m := strings.TrimSpace(message)
	if utf8.RuneCountInString(m) > MaxMessageChars {
		return "", &InvalidError{MsgMessageLong}
	}
	for _, c := range m {
		if unicode.IsControl(c) && c != '\n' {
			return "", &InvalidError{MsgMessageCtrl}
		}
	}
	return m, nil
}

// File checks and stores a report (R-5). The error is an *InvalidError,
// ErrNotFound, or a store failure. A workout that already has MaxPerWorkout
// reports gets nil and nothing is stored or notified (R-4, OQ-101).
func (s *Service) File(ctx context.Context, publicID, reason, message string) error {
	m, err := Check(reason, message)
	if err != nil {
		return err
	}
	if !lifecycle.ValidPublicID(publicID) {
		return ErrNotFound
	}
	// The request context is not used for the write: a committed report must
	// not be reported as failed because the client left.
	id, workoutID, found, err := s.d.Store.AddReport(context.WithoutCancel(ctx), publicID, reason, m, s.d.Now(), MaxPerWorkout)
	switch {
	case err != nil:
		return err
	case !found:
		return ErrNotFound
	case id == 0:
		return nil
	}
	s.d.Logger.Info("report stored", "id", id, "workout_id", workoutID, "reason", reason)
	if s.d.Notifier != nil {
		s.d.Notifier.Notify(id, Labels[reason], s.d.BaseURL+lifecycle.ViewPath(publicID))
	}
	return nil
}

// Prune deletes every report filed Retention ago or earlier (R-3); serve's
// expiry pass runs it before its checkpoint. A failure is logged.
func (s *Service) Prune(ctx context.Context) {
	n, err := s.d.Store.DeleteReportsFiledBy(ctx, s.d.Now().Add(-Retention))
	if err != nil {
		s.d.Logger.Error("report pruning failed", "error", err.Error())
		return
	}
	if n > 0 {
		s.d.Logger.Info("reports pruned", "removed", n)
	}
}

// IssueDeleteLink creates the delete key of a report whose notification is
// about to be sent and returns the absolute delete link (F-37 DK-3). The key
// leaves only in the link; the store gets its hash. A failure, or a report
// already gone, gives false and one WARN line with the report ID only.
func (s *Service) IssueDeleteLink(ctx context.Context, reportID int64) (string, bool) {
	key, err := lifecycle.NewKey(s.rand)
	if err == nil {
		var ok bool
		if ok, err = s.d.Store.SetDeleteKeyHash(ctx, reportID, lifecycle.HashToken(key)); err == nil && ok {
			return s.d.BaseURL + DeletePath + "#" + key, true
		}
	}
	s.d.Logger.Warn("delete key not issued", "id", reportID)
	return "", false
}

// ClearDeleteKeys removes every delete key (F-37 DK-5: serve without
// Pushover); the count is logged when above zero.
func (s *Service) ClearDeleteKeys(ctx context.Context) error {
	n, err := s.d.Store.ClearDeleteKeys(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		s.d.Logger.Info("delete keys cleared", "removed", n)
	}
	return nil
}

// LookupDeleteKey is what a valid key shows (DK-7). It changes nothing. The
// error is ErrLinkInvalid or a store failure.
func (s *Service) LookupDeleteKey(ctx context.Context, key string) (KeyInfo, error) {
	if !lifecycle.ValidPublicID(key) { // the same 22-base62 format
		return KeyInfo{}, ErrLinkInvalid
	}
	now := s.d.Now()
	k, found, err := s.d.Store.ReportByDeleteKey(ctx, lifecycle.HashToken(key), now.Add(-Retention), now)
	switch {
	case err != nil:
		return KeyInfo{}, err
	case !found:
		return KeyInfo{}, ErrLinkInvalid
	}
	title, start := page.RowTitle(k.Workout.Title, k.Workout.Sport, k.Workout.StartTime, k.Workout.TimeZoneOffsetS)
	return KeyInfo{Title: title, StartDate: start, ReportID: k.ID, Reason: Labels[k.Reason],
		FiledAt: k.FiledAt, ExpiresAt: k.Workout.ExpiresAt, ReportCount: k.Count}, nil
}

// RedeemDeleteKey spends a valid key and deletes its workout through the
// operator delete (DK-8). The key is spent first, so a failed delete wastes
// it (the CLI remains). Neither step stops when the client leaves. The error
// is ErrLinkInvalid, a store failure or the delete's.
func (s *Service) RedeemDeleteKey(ctx context.Context, key string) error {
	if !lifecycle.ValidPublicID(key) {
		return ErrLinkInvalid
	}
	ctx = context.WithoutCancel(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.d.Now()
	id, publicID, ok, err := s.d.Store.ConsumeDeleteKey(ctx, lifecycle.HashToken(key), now.Add(-Retention), now)
	switch {
	case err != nil:
		return err
	case !ok:
		return ErrLinkInvalid
	}
	return s.d.Deleter.DeleteByOperator(ctx, lifecycle.Record{ID: id, PublicID: publicID})
}
