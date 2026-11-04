package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"io"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
)

// Outcome is what Regenerate did with one workout (F-30 RG-8).
type Outcome string

// Outcomes of Regenerate.
const (
	Rebuilt Outcome = "rebuilt"
	Deleted Outcome = "deleted"
	Kept    Outcome = "kept"    // unchanged or partly rebuilt; the next run redoes it (RG-4)
	Skipped Outcome = "skipped" // gone or no longer live (RG-2)
)

// RejectedByFile is the F-09 hand-off rule (v1.9) that decides a delete: an
// *ingest.Error that depends only on the file's bytes and the code's
// version. canceled, a time limit and anything that is not an *ingest.Error
// keep the workout.
func RejectedByFile(err error) bool {
	var ie *ingest.Error
	if !errors.As(err, &ie) {
		return false
	}
	switch ie.Kind {
	case ingest.KindNotFIT, ingest.KindUnsupportedType, ingest.KindNoData, ingest.KindUnreadable, ingest.KindInternal:
		return true
	case ingest.KindLimit:
		return ie.Limit != "time"
	}
	return false
}

// Regenerate rebuilds one workout from its stored original (F-30 RG-2 to
// RG-5) with p, the process's parser with the workout's id on its log line.
// The row is read again first; a gone or due row is skipped. Accept's
// rejection of the file deletes the workout through Delete; any other
// failure keeps it. After Accept succeeded, the commit steps run to the end
// even when ctx is cancelled (RG-7): private files, the derived columns,
// then the public build of an unlisted workout. The error is the failure of
// a kept workout, or of a deleted one's leftover folder, already free of
// coordinates and public paths.
func (s *Service) Regenerate(ctx context.Context, publicID string, p Parser) (Outcome, error) {
	before, found, err := s.d.Repo.WorkoutByPublicID(ctx, publicID)
	if err != nil {
		return Kept, err
	}
	if !found || !before.Live(s.d.Now()) {
		return Skipped, nil
	}
	id := before.ID
	orig, err := s.readOriginal(id)
	if err != nil {
		return Kept, err
	}
	acc, err := p.Accept(ctx, orig)
	if err != nil {
		if !RejectedByFile(err) {
			return Kept, err
		}
		// A stopped run never deletes, whatever the parse made of the stop (QA-30-r1-2).
		if ctx.Err() != nil {
			return Kept, err
		}
		// ingest has logged the rejection with the device model and id.
		dctx := context.WithoutCancel(ctx)
		s.mu.Lock()
		err := s.Delete(dctx, id, before.PublicID)
		s.mu.Unlock()
		if err != nil {
			// Only a failure of the row step keeps the workout; after it, a
			// leftover folder goes at serve's next start (QA-30-r1-3).
			if _, found, rerr := s.d.Repo.WorkoutByPublicID(dctx, before.PublicID); rerr != nil || found {
				return Kept, err
			}
		}
		s.d.Logger.Info("workout deleted", "id", id, "by", "regenerate")
		return Deleted, err
	}
	cctx := context.WithoutCancel(ctx)
	w := acc.Workout
	bin, err := datafile.Encode(w)
	if err != nil {
		return Kept, err
	}
	sum, err := datafile.SummaryJSON(w)
	if err != nil {
		return Kept, err
	}
	derived := NewDraft(w, sum, Identity{}, s.d.Now()) // the derived values as the upload makes them
	if !bytes.Equal(acc.Original, orig) {
		if err := s.d.Files.Replace(id, OriginalName, acc.Original); err != nil {
			return Kept, err
		}
	}
	if err := s.d.Files.Replace(id, DraftName, bin); err != nil {
		return Kept, err
	}
	after, found, err := s.d.Repo.UpdateDerived(cctx, id, derived, s.d.Now())
	if err != nil {
		return Kept, err
	}
	if !found {
		return Skipped, nil
	}
	if after.Visibility != Unlisted || s.d.Hook == nil {
		return Rebuilt, nil
	}
	if err := s.d.Hook.WorkoutChanged(cctx, before, after); err != nil {
		return Kept, err
	}
	// P-6's re-read: serve may have deleted the workout or let it expire meanwhile.
	if s.d.Public != nil {
		cur, found, err := s.d.Repo.WorkoutByPublicID(cctx, publicID)
		if err != nil {
			return Kept, err
		}
		if !found || cur.Visibility != Unlisted || !cur.Live(s.d.Now()) {
			if _, err := s.d.Public.Remove(publicID); err != nil {
				return Kept, err
			}
		}
	}
	return Rebuilt, nil
}

func (s *Service) readOriginal(id string) ([]byte, error) {
	f, _, err := s.d.Files.Read(id, OriginalName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// Checkpoint checkpoints the WAL if a row was deleted since the last
// complete checkpoint (F-30 RG-8); a failure is logged, never returned.
func (s *Service) Checkpoint(ctx context.Context) { s.checkpoint(ctx) }
