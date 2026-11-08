package lifecycle

import (
	"context"
	"errors"
	"io"

	"workout-explorer/internal/preview"
)

// The two owner operations of F-36 (MC-4, MC-5, ADR-041): the frame the
// browser renders the basemap for, and the upload of that basemap. Neither
// logs the frame, the key or any byte of the upload (I-5).

// Frames is the public build's front half (internal/publish): the preview
// frame of the workout's public data file; ok is false without positions.
type Frames interface {
	Frame(ctx context.Context, r Record) (fr preview.Frame, ok bool, err error)
}

// Refusals of PutPreviewMap besides ErrNotFound and *ServerError.
var (
	ErrMapBusy  = errors.New("lifecycle: a preview map is in flight")
	ErrMapStale = errors.New("lifecycle: preview frame changed")
)

// MapRefusedError is a basemap refused at the trust boundary; Reason is
// "size" or one of preview's reasons.
type MapRefusedError struct{ Reason string }

func (e *MapRefusedError) Error() string { return "lifecycle: preview map refused: " + e.Reason }

// MapFrame is the answer of PreviewFrame: OK false means no frame (no
// public positions); Current: a stored basemap matches the frame.
type MapFrame struct {
	Frame   preview.Frame
	OK      bool
	Current bool
}

// PreviewFrame is GET …/preview-map: the token check, then the build's front
// half. It writes nothing.
func (s *Service) PreviewFrame(ctx context.Context, publicID, token string) (MapFrame, error) {
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return MapFrame{}, err
	}
	if s.d.Frames == nil {
		return MapFrame{}, ErrNotFound
	}
	fr, ok, err := s.d.Frames.Frame(ctx, rec)
	if err != nil {
		if gone(ctx, err) {
			return MapFrame{}, ErrNotFound
		}
		return MapFrame{}, s.ownerFail("frame", rec.ID, err)
	}
	if !ok {
		return MapFrame{}, nil
	}
	current := false
	if body, _, err := s.d.Files.Read(rec.ID, PreviewMapName(fr.Key)); err == nil {
		body.Close()
		current = true
	}
	return MapFrame{Frame: fr, OK: true, Current: current}, nil
}

// PutPreviewMap is PUT …/preview-map/{key} (MC-4): token check, key form,
// one upload at a time, the body read under preview.MaxMapBytes, the checks
// of MC-5, then under the O-7 lock the frame of now must have this key; the
// re-encoded map is stored and, for a published workout, the public files
// are rebuilt in place. body must stop the transfer after the cap (the
// handler's http.MaxBytesReader).
func (s *Service) PutPreviewMap(ctx context.Context, publicID, token, key string, body io.Reader) error {
	rec, err := s.lookup(ctx, publicID, token)
	if err != nil {
		return err
	}
	if !preview.ValidKey(key) || s.d.Frames == nil {
		return ErrNotFound
	}
	if !s.mapSlot.CompareAndSwap(false, true) {
		return ErrMapBusy
	}
	defer s.mapSlot.Store(false)
	data, err := io.ReadAll(io.LimitReader(body, preview.MaxMapBytes+1))
	if len(data) > preview.MaxMapBytes {
		return s.refuseMap(rec.ID, "size")
	}
	if err != nil {
		return &MapRefusedError{Reason: "read"} // the client is gone or sent a broken body: not logged
	}
	img, reason := preview.CheckMap(data)
	data = nil
	if reason != "" {
		return s.refuseMap(rec.ID, reason)
	}
	stored, err := preview.EncodeMap(img)
	img = nil
	if err != nil {
		return s.ownerFail("map encode", rec.ID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, err = s.lookup(ctx, publicID, token); err != nil {
		return err
	}
	sctx := context.WithoutCancel(ctx)
	// Under the lock, so no settings change can slip between this frame and the store.
	fr, ok, err := s.d.Frames.Frame(sctx, rec)
	if err != nil {
		return s.ownerFail("frame", rec.ID, err)
	}
	if !ok || fr.Key != key {
		s.d.Logger.Info("preview map refused", "id", rec.ID, "reason", "stale")
		return ErrMapStale
	}
	// Never refused for storage (MC-9): the bytes are counted, not reserved.
	s.d.Storage.Add(int64(len(stored)))
	freed, err := s.d.Files.StoreMap(rec.ID, key, stored)
	if err != nil {
		s.d.Storage.Release(int64(len(stored)))
		return s.ownerFail("map store", rec.ID, err)
	}
	s.d.Storage.Release(freed)
	if rec.Visibility != Unlisted {
		return nil // a draft: the publish build picks the map up
	}
	// before = after: an in-place build (F-15 P-7 "no difference").
	return s.changed(sctx, rec, rec)
}

func (s *Service) refuseMap(id, reason string) error {
	s.d.Logger.Info("preview map refused", "id", id, "reason", reason)
	return &MapRefusedError{Reason: reason}
}
