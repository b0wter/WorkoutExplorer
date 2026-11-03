// Package reports is filing a report on a published workout (F-27, INF-2):
// the pure report rule (reasons, message) and the operation behind the Store
// port that internal/store implements. A stored report is handed to the
// Notifier port (ADR-039). Nothing here changes a workout.
package reports

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"workout-explorer/internal/lifecycle"
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

// Store is the reports table (internal/store).
type Store interface {
	// AddReport: see store.DB.AddReport. id 0 with found true: not stored (cap).
	AddReport(ctx context.Context, publicID, reason, message string, now time.Time, max int) (id int64, workoutID string, found bool, err error)
	DeleteReportsFiledBy(ctx context.Context, t time.Time) (int64, error)
}

// Notifier learns of each stored report (ADR-039). Notify must not block.
type Notifier interface {
	Notify(id int64, reason string)
}

// Service files reports.
type Service struct {
	store  Store
	notify Notifier // nil: notifications off
	now    func() time.Time
	log    *slog.Logger
}

// New returns a Service; nil notify turns notifications off, nil now is
// time.Now, nil log writes nothing.
func New(store Store, notify Notifier, now func() time.Time, log *slog.Logger) *Service {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, notify: notify, now: now, log: log}
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
	id, workoutID, found, err := s.store.AddReport(context.WithoutCancel(ctx), publicID, reason, m, s.now(), MaxPerWorkout)
	switch {
	case err != nil:
		return err
	case !found:
		return ErrNotFound
	case id == 0:
		return nil
	}
	s.log.Info("report stored", "id", id, "workout_id", workoutID, "reason", reason)
	if s.notify != nil {
		s.notify.Notify(id, reason)
	}
	return nil
}

// Prune deletes every report filed Retention ago or earlier (R-3); serve's
// expiry pass runs it before its checkpoint. A failure is logged.
func (s *Service) Prune(ctx context.Context) {
	n, err := s.store.DeleteReportsFiledBy(ctx, s.now().Add(-Retention))
	if err != nil {
		s.log.Error("report pruning failed", "error", err.Error())
		return
	}
	if n > 0 {
		s.log.Info("reports pruned", "removed", n)
	}
}
