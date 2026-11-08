// Package lifecycle is the application service of a workout's life. F-09
// holds its first operations: Upload (a FIT upload becomes a stored draft)
// and Recover (startup sweep of unfinished uploads). It knows no SQL, no file
// system and no HTTP: it works through the ports of ports.go.
package lifecycle

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"workout-explorer/internal/datafile"
)

// Deps are the collaborators of a Service.
type Deps struct {
	Parser Parser
	Repo   Repository
	Files  PrivateFiles
	// Storage is the running total and cap (F-10); nil: no cap.
	Storage *Storage
	// Hook is F-15's "published workout changed" port (F-12 O-11); nil: nothing to do.
	Hook ChangeHook
	// Public is the public area (F-15 P-3); nil: no public files (tests of earlier features).
	Public PublicFiles
	// Frames computes preview frames (F-36 MC-2); nil: the preview-map operations answer not found.
	Frames Frames
	Logger *slog.Logger
	Now    func() time.Time // nil: time.Now
	Rand   io.Reader        // nil: crypto/rand.Reader
}

// Service runs the workout operations.
type Service struct {
	d Deps
	// mu serialises the owner's writes: token check, row change and hook (F-12 O-7).
	mu sync.Mutex
	// uncheckpointed: a row was deleted and the WAL not yet checkpointed.
	uncheckpointed atomic.Bool
	// mapSlot admits one basemap upload at a time (F-36 MC-4).
	mapSlot atomic.Bool
}

// New returns a Service. A nil Logger writes nothing.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Rand == nil {
		d.Rand = rand.Reader
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

// Created is what the sharer gets back: the two values the links are made from.
type Created struct{ PublicID, EditToken string }

// Upload turns the bytes of an uploaded FIT file into a stored draft (Publish
// 1 to 3). The parser's rejection is returned unchanged (errors.As finds an
// *ingest.Error) and nothing is written. Any other failure leaves neither a
// row nor files behind. Only the stripped bytes (UP-9) are ever stored.
func (s *Service) Upload(ctx context.Context, upload []byte) (Created, error) {
	t0 := time.Now()
	acc, err := s.d.Parser.Accept(ctx, upload)
	if err != nil {
		return Created{}, err
	}
	// acc is not used again, and w not after step 3, so only the data file
	// and the bytes are live during the file writes (ADR-017 rev. 3 item 4).
	w, original, counts := acc.Workout, acc.Original, acc.Stripped
	truncated := w.Integrity.Truncated
	n := len(upload) // from here on only original is used, never upload

	bin, err := datafile.Encode(w)
	if err != nil {
		return Created{}, s.fail("encode", "", err)
	}
	sum, err := datafile.SummaryJSON(w)
	if err != nil {
		return Created{}, s.fail("encode", "", err)
	}
	id, err := NewIdentity(s.d.Rand)
	if err != nil {
		return Created{}, s.fail("identity", "", err)
	}
	rec := NewDraft(w, sum, id, s.d.Now())

	if err := ctx.Err(); err != nil { // the client is gone: nothing written
		return Created{}, err
	}
	// The storage cap (UP-8, F-10 design): the bytes are reserved before they
	// are written, so a concurrent upload sees them at once. Only the first
	// upload to reserve at the cap goes over it.
	size := int64(len(original) + len(bin))
	if !s.d.Storage.Reserve(size) {
		return Created{}, ErrStorageFull
	}
	if err := s.d.Files.Create(id.ID, []File{{OriginalName, original}, {DraftName, bin}}); err != nil {
		s.d.Storage.Release(size) // Create left nothing behind
		return Created{}, s.fail("files", id.ID, err)
	}
	// From here on the request context is never consulted (D-18, ADR-022 rev. 2
	// item 4): the driver can report ctx.Err() for an insert that committed.
	sctx := context.WithoutCancel(ctx)
	if err := s.d.Repo.InsertWorkout(sctx, rec); err != nil {
		exists, xerr := s.d.Repo.WorkoutExists(sctx, id.ID)
		switch {
		case xerr == nil && exists:
			// The row is committed: the upload succeeded, the error is dropped.
		case xerr == nil:
			if _, rerr := s.d.Files.Remove(id.ID); rerr != nil {
				// The folder may be partly there: the reservation stays (counting
				// too much only refuses earlier); the next start recomputes it.
				s.d.Logger.Error("upload failed", "stage", "cleanup", "id", id.ID, "error", rerr.Error())
			} else {
				s.d.Storage.Release(size)
			}
			return Created{}, s.fail("store", id.ID, err)
		default:
			// Unknown whether the row exists: the folder stays, and so does its
			// reservation; recovery settles both at the next start.
			serr := s.fail("store", id.ID, err)
			s.d.Logger.Error("upload failed", "stage", "cleanup", "id", id.ID, "error", xerr.Error())
			return Created{}, serr
		}
	}
	s.d.Logger.Info("upload accepted",
		"id", id.ID,
		"bytes", n,
		"truncated", truncated,
		"blanked_fields", counts.BlankedFields,
		"removed_messages", counts.RemovedMessages,
		"duration_ms", time.Since(t0).Milliseconds(),
	)
	return Created{PublicID: id.PublicID, EditToken: id.Token}, nil
}

// ServerError is a failure of the server itself (as opposed to a rejected
// file): the sharer is told to try again later.
type ServerError struct {
	Stage string
	Err   error
}

func (e *ServerError) Error() string { return "lifecycle: " + e.Stage + ": " + e.Err.Error() }
func (e *ServerError) Unwrap() error { return e.Err }

func (s *Service) fail(stage, id string, err error) error {
	if id == "" {
		s.d.Logger.Error("upload failed", "stage", stage, "error", err.Error())
	} else {
		s.d.Logger.Error("upload failed", "stage", stage, "id", id, "error", err.Error())
	}
	return &ServerError{Stage: stage, Err: err}
}

// Delete removes a workout: the row first, then public/{publicID}, then its
// private folder, also when the public step fails (ADR-022 item 6, F-15 P-8;
// the errors are joined), and releases the freed bytes
// from the storage total (F-10). It is nil if the workout does not exist. It
// is the single delete path (I-4) that the owner's delete (F-12), expiry
// (F-21) and the operator's delete (F-27) call. The storage cap never
// refuses it.
func (s *Service) Delete(ctx context.Context, id, publicID string) error {
	deleted, err := s.d.Repo.DeleteWorkout(ctx, id)
	if err != nil {
		return fmt.Errorf("lifecycle: delete: %w", err)
	}
	var public int64
	var errs []error
	if s.d.Public != nil {
		// A public folder without a row is removed at the next start (P-8);
		// until then its bytes stay counted. Its failure never keeps the
		// private folder (QA-15-r1-2).
		var perr error
		if public, perr = s.d.Public.Remove(publicID); perr != nil {
			public = 0
			errs = append(errs, perr)
		}
	}
	freed, err := s.d.Files.Remove(id)
	if err != nil {
		// A folder without a row is removed at the next start (ADR-022 item 5);
		// until then its bytes stay counted.
		freed = 0
		errs = append(errs, err)
	}
	// Only the call that deleted the row releases: two concurrent deletes of
	// the same workout would otherwise both measure the folder and both
	// release it (QA-10-r1-1).
	if deleted {
		s.uncheckpointed.Store(true)
		s.d.Storage.Release(public + freed)
	}
	if len(errs) > 0 {
		return fmt.Errorf("lifecycle: delete: %w", errors.Join(errs...))
	}
	return nil
}

// Recover is the startup sweep (ADR-022 rev. 2 item 5): staging is emptied,
// every private folder without a row is removed, and every row without a
// folder is deleted. Entries of private/ that are not workout folders are
// left alone and counted. Then every entry of public/ that is not the folder
// of a live unlisted row is removed (F-15 P-8): anything left there is served.
func (s *Service) Recover(ctx context.Context) error {
	staging, err := s.d.Files.ClearStaging()
	if err != nil {
		return fmt.Errorf("lifecycle: recover: %w", err)
	}
	folders, unknown, err := s.d.Files.IDs()
	if err != nil {
		return fmt.Errorf("lifecycle: recover: %w", err)
	}
	rows, err := s.d.Repo.WorkoutIDs(ctx)
	if err != nil {
		return fmt.Errorf("lifecycle: recover: %w", err)
	}
	orphans, removedRows := 0, 0
	var errs []error
	have := make(map[string]struct{}, len(folders))
	for _, id := range folders {
		have[id] = struct{}{}
		if _, ok := rows[id]; ok {
			continue
		}
		if _, err := s.d.Files.Remove(id); err != nil {
			errs = append(errs, err)
			continue
		}
		orphans++
	}
	for id := range rows {
		if _, ok := have[id]; ok {
			continue
		}
		if _, err := s.d.Repo.DeleteWorkout(ctx, id); err != nil {
			errs = append(errs, err)
			continue
		}
		removedRows++
		s.uncheckpointed.Store(true) // QA-21-r2-1: the row's bytes must leave the WAL too
	}
	publicRemoved := 0
	if s.d.Public != nil {
		if n, _, err := s.SweepPublic(ctx); err != nil {
			errs = append(errs, err)
		} else {
			publicRemoved = n
		}
	}
	s.d.Logger.Info("recovery", "staging_removed", staging, "orphans_removed", orphans,
		"rows_removed", removedRows, "unknown_entries", unknown, "public_removed", publicRemoved)
	if len(errs) > 0 {
		return fmt.Errorf("lifecycle: recover: %w", errors.Join(errs...))
	}
	return nil
}

// checkpoint runs after a delete so the deleted row leaves the WAL too
// (QA-21-r1-1); it does nothing while no row was deleted since the last
// complete one. A failure never fails the delete: it is logged, and the next
// pass tries again.
func (s *Service) checkpoint(ctx context.Context) {
	if !s.uncheckpointed.Swap(false) {
		return
	}
	if err := s.d.Repo.Checkpoint(ctx); err != nil {
		s.uncheckpointed.Store(true)
		s.d.Logger.Warn("checkpoint failed", "error", err.Error())
	}
}

// SweepPublic removes every entry of public/ that is not the folder of a live
// unlisted row, and in the live ones every entry their page does not
// reference, or the whole folder when it has no page (F-15 P-8). It
// returns how many entries it removed and the bytes it freed, and releases
// nothing: at startup the storage total is measured afterwards; Expire
// releases them.
func (s *Service) SweepPublic(ctx context.Context) (int, int64, error) {
	keep, err := s.d.Repo.LivePublicIDs(ctx, s.d.Now())
	if err != nil {
		return 0, 0, fmt.Errorf("lifecycle: sweep public: %w", err)
	}
	n, freed, err := s.d.Public.Sweep(keep)
	if err != nil {
		return n, freed, fmt.Errorf("lifecycle: sweep public: %w", err)
	}
	return n, freed, nil
}

// Expire is the expiry pass (F-21 X-4): every row past its deadline (drafts
// after 48 h too) is deleted through Delete, then the public sweep runs and
// its freed bytes are released, then the WAL is checkpointed if a row went.
// The WAL is checkpointed on every pass (F-27 R-10).
// Each step holds the O-7 lock, so no owner write can revive a due row (X-3).
// A failed row or list is logged and the pass goes on; the next pass
// retries. The returned error joins every failure.
func (s *Service) Expire(ctx context.Context) error {
	var errs []error
	// A failed list still runs the sweep, which has its own query (X-4).
	due, err := s.d.Repo.DueWorkouts(ctx, s.d.Now())
	if err != nil {
		s.d.Logger.Error("expiry failed", "stage", "list", "error", err.Error())
		errs = append(errs, err)
	}
	for _, r := range due {
		s.mu.Lock()
		err := s.Delete(ctx, r.ID, r.PublicID)
		s.mu.Unlock()
		if err != nil {
			s.d.Logger.Error("expiry failed", "stage", "delete", "id", r.ID, "error", err.Error())
			errs = append(errs, err)
			continue
		}
		s.d.Logger.Info("workout deleted", "id", r.ID, "by", "expiry")
	}
	if s.d.Public != nil {
		s.mu.Lock()
		n, freed, err := s.SweepPublic(ctx)
		s.mu.Unlock()
		s.d.Storage.Release(freed)
		if err != nil {
			s.d.Logger.Error("expiry failed", "stage", "sweep", "error", err.Error())
			errs = append(errs, err)
		} else if n > 0 {
			s.d.Logger.Info("public sweep", "removed", n)
		}
	}
	// Every pass checkpoints, so a delete by another process (F-27's delete
	// command, F-30) leaves the WAL within one pass (F-27 R-10).
	s.uncheckpointed.Store(true)
	s.checkpoint(ctx)
	if len(errs) > 0 {
		return fmt.Errorf("lifecycle: expire: %w", errors.Join(errs...))
	}
	return nil
}
