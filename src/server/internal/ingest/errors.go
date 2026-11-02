package ingest

import (
	"encoding/json"
	"fmt"

	"workout-explorer/internal/workout"
)

// Kind is the class of a parse failure (ADR-018 item 1). The names are
// stable: F-09 may return them in API responses.
type Kind string

// Parse failure kinds.
const (
	KindNotFIT          Kind = "not_fit"
	KindUnsupportedType Kind = "unsupported_type"
	KindNoData          Kind = "no_data"
	KindLimit           Kind = "limit"
	KindInternal        Kind = "internal"
	KindUnreadable      Kind = "unreadable" // a deliberate refusal of crafted input (F-09 D-26, ADR-018 rev. 5)
	KindCanceled        Kind = "canceled"
)

// messages are the user-facing sentences, one per kind, in one place
// (ADR-018 item 1; the wording is accepted, OQ-30). Changing a sentence
// changes the corpus rejection reports.
var messages = map[Kind]string{
	KindNotFIT:          "This file is not a FIT file.",
	KindUnsupportedType: "Unsupported FIT file type.", // exact, UP-2
	KindNoData:          "This file contains no readable workout data.",
	KindLimit:           "This file is too large or too complex to read.",
	KindInternal:        "This file could not be read.",
	KindUnreadable:      "This file could not be read.", // the same sentence as internal (ADR-018 rev. 5)
	KindCanceled:        "The upload was interrupted.",
}

// Error is the only error that leaves (*Parser).Parse (ADR-018).
type Error struct {
	Kind     Kind
	Limit    string         // limit name when Kind == KindLimit, else ""
	FileType string         // file_id.type name if known
	Device   workout.Device // if known
	cause    error
}

// Error returns "ingest: <kind>[ (<limit>)]: <cause>". It never contains a
// field value of the file.
func (e *Error) Error() string {
	s := "ingest: " + string(e.Kind)
	if e.Limit != "" {
		s += " (" + e.Limit + ")"
	}
	if e.cause != nil {
		s += ": " + e.cause.Error()
	}
	return s
}

// Message is the sentence for the sharer.
func (e *Error) Message() string { return messages[e.Kind] }

// Unwrap returns the cause; errors.Is(err, context.Canceled) works for
// KindCanceled.
func (e *Error) Unwrap() error { return e.cause }

// rejectionReport is the golden rejection report (F-08 design, Report).
type rejectionReport struct {
	ReportVersion int           `json:"report_version"`
	Rejected      rejectionBody `json:"rejected"`
}

type rejectionBody struct {
	Kind     Kind   `json:"kind"`
	Message  string `json:"message"`
	Limit    string `json:"limit,omitempty"`
	FileType string `json:"file_type,omitempty"`
}

// RejectionJSON is the only producer of the golden rejection report used by
// the corpus (ADR-018 item 6): indented with two spaces, trailing newline.
func RejectionJSON(e *Error) ([]byte, error) {
	b, err := json.MarshalIndent(rejectionReport{
		ReportVersion: workout.ReportVersion,
		Rejected:      rejectionBody{Kind: e.Kind, Message: e.Message(), Limit: e.Limit, FileType: e.FileType},
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("ingest: rejection report: %w", err)
	}
	return append(b, '\n'), nil
}
