package lifecycle

import (
	"context"
	"io"
	"time"

	"workout-explorer/internal/ingest"
)

// Parser is the upload parser; *ingest.Parser satisfies it.
type Parser interface {
	Accept(ctx context.Context, upload []byte) (ingest.Accepted, error)
}

// Repository stores workout rows (internal/store).
type Repository interface {
	InsertWorkout(ctx context.Context, r Record) error
	WorkoutExists(ctx context.Context, id string) (bool, error) // D-18: SELECT 1 ... WHERE id = ?
	DeleteWorkout(ctx context.Context, id string) (bool, error) // true only if this call removed the row; absent is false, nil; F-12, F-21 and F-27 reuse it
	WorkoutIDs(ctx context.Context) (map[string]struct{}, error)
	WorkoutByPublicID(ctx context.Context, publicID string) (r Record, found bool, err error)
	// UpdateWorkout reads the row, calls apply and stores the settings columns
	// it returns, in one immediate transaction (F-12 O-3). An error from apply
	// is returned unchanged and nothing is stored; a missing row is found=false
	// and apply is not called.
	UpdateWorkout(ctx context.Context, id string, apply func(Record) (Record, error)) (r Record, found bool, err error)
	// UpdateDerived writes only the columns derived from the original, from
	// r, in one statement that changes nothing when the row is gone or not Live
	// at now, and returns the row afterwards (F-30 RG-4 (b)).
	UpdateDerived(ctx context.Context, id string, r Record, now time.Time) (Record, bool, error)
	// LivePublicIDs is the public_id of every unlisted row that is Live at
	// now: the only folders allowed under public/ (F-15 P-8, F-21 X-2).
	LivePublicIDs(ctx context.Context, now time.Time) (map[string]struct{}, error)
	// DueWorkouts is every row that is not Live at now (F-21 X-1, X-2): the
	// SQL form of Record.Deadline.
	DueWorkouts(ctx context.Context, now time.Time) ([]Record, error)
	// Checkpoint moves the WAL into the database file and truncates it, so
	// a deleted row leaves no copy (QA-21-r1-1); it fails if a reader blocks it.
	Checkpoint(ctx context.Context) error
}

// PublicFiles is the public area (internal/publish, F-15 P-3). Only that
// package creates, writes or removes anything under public/ (I-3). Errors
// never contain a public_id or a public path (P-14).
type PublicFiles interface {
	// Remove deletes public/{publicID}, index.html first, and returns the
	// sizes of the files it held; nil and 0 if absent. It releases nothing
	// from the storage total: the caller does (F-10 S-6).
	Remove(publicID string) (freed int64, err error)
	// Sweep removes every entry of public/ whose name is not in keep, every
	// entry of a kept folder that its index.html does not reference, and a
	// kept folder without index.html (P-8); freed is the sizes of
	// the files removed. It releases nothing from the storage total: at
	// startup the total is measured afterwards; F-21 releases freed.
	Sweep(keep map[string]struct{}) (removed int, freed int64, err error)
}

// PrivateFiles is the private file area (internal/privatedir).
type PrivateFiles interface {
	Create(id string, files []File) error                             // fails if private/{id} exists; staged, fsynced, renamed; cleans its own staging on error
	Remove(id string) (freed int64, err error)                        // private/{id}; freed = the sizes of its files (F-10); nil and 0 if absent
	Read(id, name string) (body io.ReadCloser, size int64, err error) // an open file of private/{id}; an error wrapping fs.ErrNotExist if absent (F-12 O-8)
	Replace(id, name string, data []byte) error                       // temp and rename into an existing private/{id}, never creating it (F-30 RG-4)
	IDs() (ids []string, unknown int, err error)
	ClearStaging() (removed int, err error)
}

// File is one file of a private workout folder.
type File struct {
	Name string
	Data []byte
}

// The two file names of a private workout folder (ADR-022 item 1).
const (
	OriginalName = "original.fit"
	DraftName    = "draft.bin"
)
