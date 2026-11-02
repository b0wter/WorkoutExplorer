package lifecycle

import (
	"errors"
	"sync"
)

// ErrStorageFull is returned by Upload when the storage cap was reached
// while the upload was being processed (UP-8, F-10 design).
var ErrStorageFull = errors.New("lifecycle: storage full")

// Storage is the running total of the bytes stored under the data directory
// and the cap on it (UP-8, F-10 design). It is pure: callers tell it what they
// wrote and freed. Only new uploads are refused by the cap; publishing, edits,
// regeneration and deletes use Add and Release, which never refuse.
// A nil *Storage means no cap (tests of earlier features).
type Storage struct {
	mu    sync.Mutex
	limit int64
	used  int64
}

// NewStorage returns a total of used bytes under a cap of limit bytes.
func NewStorage(limit, used int64) *Storage {
	return &Storage{limit: limit, used: used}
}

// UploadsOpen reports whether a new upload may start: the total is below the cap.
func (s *Storage) UploadsOpen() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.used < s.limit
}

// Reserve adds n bytes for an upload about to be written if the total is
// still below the cap, and reports whether it did. Check and add are one
// step, so of several uploads racing at the cap only the first one to
// reserve goes over it.
func (s *Storage) Reserve(n int64) bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used >= s.limit {
		return false
	}
	s.used += n
	return true
}

// Add counts n bytes written by an operation the cap never refuses.
func (s *Storage) Add(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.used += n
	s.mu.Unlock()
}

// Release subtracts n freed bytes. The total never goes below zero.
func (s *Storage) Release(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.used = max(s.used-n, 0)
	s.mu.Unlock()
}

// Used returns the running total in bytes.
func (s *Storage) Used() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.used
}

// Limit returns the cap in bytes.
func (s *Storage) Limit() int64 {
	if s == nil {
		return 0
	}
	return s.limit
}
