package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"workout-explorer/internal/ingest"
)

// TestStorage: the gate is open strictly below the cap; Reserve refuses at
// or above it; Add never refuses; Release never goes below zero.
func TestStorage(t *testing.T) {
	tests := []struct {
		name        string
		limit, used int64
		open        bool
	}{
		{"empty", 100, 0, true},
		{"one below", 100, 99, true},
		{"at the cap", 100, 100, false},
		{"over the cap", 100, 150, false},
	}
	for _, tc := range tests {
		s := NewStorage(tc.limit, tc.used)
		if got := s.UploadsOpen(); got != tc.open {
			t.Errorf("%s: UploadsOpen = %v", tc.name, got)
		}
		if got := s.Reserve(30); got != tc.open {
			t.Errorf("%s: Reserve = %v", tc.name, got)
		}
		want := tc.used
		if tc.open {
			want += 30
		}
		if s.Used() != want {
			t.Errorf("%s: Used = %d, want %d", tc.name, s.Used(), want)
		}
	}

	s := NewStorage(100, 99)
	if !s.Reserve(1_000) || s.Used() != 1_099 || s.UploadsOpen() {
		t.Errorf("the upload that reaches the cap goes over it: used %d", s.Used())
	}
	s.Add(5) // publish, edits, regeneration: never refused
	if s.Used() != 1_104 {
		t.Errorf("Add while full: used %d", s.Used())
	}
	s.Release(1_005)
	if s.Used() != 99 || !s.UploadsOpen() {
		t.Errorf("after Release: used %d, open %v", s.Used(), s.UploadsOpen())
	}
	s.Release(500)
	if s.Used() != 0 {
		t.Errorf("Release below zero: used %d", s.Used())
	}

	var none *Storage // no cap
	if !none.UploadsOpen() || !none.Reserve(1<<40) {
		t.Errorf("a nil Storage refuses")
	}
	none.Add(1)
	none.Release(1)
}

// TestStorageRace (AC 1): of many uploads reserving at once just below the
// cap, exactly one gets through; the total goes over by at most that one.
func TestStorageRace(t *testing.T) {
	for range 50 {
		s := NewStorage(1_000, 990)
		var ok atomic.Int32
		var wg sync.WaitGroup
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if s.Reserve(100) {
					ok.Add(1)
				}
			}()
		}
		wg.Wait()
		if ok.Load() != 1 || s.Used() != 1_090 {
			t.Fatalf("%d reservations succeeded, used %d", ok.Load(), s.Used())
		}
	}
}

func storageFixture(t *testing.T, limit, used int64) (*Service, *fakeRepo, *fakeFiles, *Storage) {
	t.Helper()
	p := &fakeParser{acc: ingest.Accepted{Workout: smallWorkout(), Original: []byte("STRIPPED")}}
	r, f := newRepo(), newFiles()
	st := NewStorage(limit, used)
	s := New(Deps{Parser: p, Repo: r, Files: f, Storage: st, Logger: slog.New(slog.DiscardHandler), Now: func() time.Time { return t0 }})
	return s, r, f, st
}

func stored(f *fakeFiles, id string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, file := range f.dirs[id] {
		n += int64(len(file.Data))
	}
	return n
}

// TestUploadStorage (AC 1, AC 2): an upload counts its two files; at the cap
// Upload returns ErrStorageFull and writes nothing; a delete frees the bytes
// and opens uploads again.
func TestUploadStorage(t *testing.T) {
	s, r, f, st := storageFixture(t, 1_000, 990)
	c, err := s.Upload(context.Background(), []byte("UPLOAD"))
	if err != nil {
		t.Fatal(err)
	}
	id := onlyID(t, r)
	size := stored(f, id)
	if size == 0 || st.Used() != 990+size {
		t.Fatalf("used %d after storing %d bytes", st.Used(), size)
	}
	if st.UploadsOpen() {
		t.Fatalf("still open at %d of 1000", st.Used())
	}
	_, err = s.Upload(context.Background(), []byte("UPLOAD"))
	if !errors.Is(err, ErrStorageFull) {
		t.Fatalf("Upload at the cap: %v", err)
	}
	if f.creates != 1 || r.inserts != 1 {
		t.Errorf("refused upload wrote: creates %d, inserts %d", f.creates, r.inserts)
	}
	if c.PublicID == "" {
		t.Error("no public ID")
	}

	// Deletes are never refused by the cap, and free the bytes.
	if err := s.Delete(context.Background(), id, ""); err != nil {
		t.Fatal(err)
	}
	if st.Used() != 990 || !st.UploadsOpen() {
		t.Errorf("after delete: used %d, open %v", st.Used(), st.UploadsOpen())
	}
	if len(r.rows) != 0 || len(f.dirs) != 0 {
		t.Errorf("delete left rows %d, folders %d", len(r.rows), len(f.dirs))
	}
	if err := s.Delete(context.Background(), id, ""); err != nil || st.Used() != 990 {
		t.Errorf("second delete: %v, used %d", err, st.Used())
	}
}

func onlyID(t *testing.T, r *fakeRepo) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.rows) != 1 {
		t.Fatalf("%d rows", len(r.rows))
	}
	for id := range r.rows {
		return id
	}
	return ""
}

// TestUploadStorageFailures: a failed upload gives its reservation back,
// except when its folder may still be there (recomputed at the next start).
func TestUploadStorageFailures(t *testing.T) {
	t.Run("files", func(t *testing.T) {
		s, _, f, st := storageFixture(t, 1_000, 10)
		f.createErr = errors.New("disk")
		if _, err := s.Upload(context.Background(), nil); err == nil {
			t.Fatal("no error")
		}
		if st.Used() != 10 {
			t.Errorf("used %d", st.Used())
		}
	})
	t.Run("store, folder removed", func(t *testing.T) {
		s, r, _, st := storageFixture(t, 1_000, 10)
		r.insertErr = errors.New("db")
		if _, err := s.Upload(context.Background(), nil); err == nil {
			t.Fatal("no error")
		}
		if st.Used() != 10 {
			t.Errorf("used %d", st.Used())
		}
	})
	t.Run("store, folder left", func(t *testing.T) {
		s, r, f, st := storageFixture(t, 1_000, 10)
		r.insertErr = errors.New("db")
		f.removeErr = errors.New("rm")
		if _, err := s.Upload(context.Background(), nil); err == nil {
			t.Fatal("no error")
		}
		if st.Used() <= 10 {
			t.Errorf("used %d: the folder that may remain must stay counted", st.Used())
		}
	})
	t.Run("delete, folder left", func(t *testing.T) {
		s, r, f, st := storageFixture(t, 1_000, 10)
		if _, err := s.Upload(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		used := st.Used()
		f.removeErr = errors.New("rm")
		if err := s.Delete(context.Background(), onlyID(t, r), ""); err == nil {
			t.Fatal("no error")
		}
		if st.Used() != used {
			t.Errorf("used %d, want %d", st.Used(), used)
		}
		if len(r.rows) != 0 {
			t.Errorf("the row must be deleted first (ADR-022 item 6)")
		}
	})
}

// measuringFiles makes both racing removers measure the folder before either
// one returns: the interleaving behind QA-10-r1-1, forced.
type measuringFiles struct {
	*fakeFiles
	sizes sync.WaitGroup
}

func (m *measuringFiles) Remove(id string) (int64, error) {
	m.fakeFiles.mu.Lock()
	var n int64
	for _, file := range m.fakeFiles.dirs[id] {
		n += int64(len(file.Data))
	}
	m.fakeFiles.mu.Unlock()
	m.sizes.Done()
	m.sizes.Wait()
	return n, nil
}

// QA-10-r1-1: of two concurrent deletes of the same workout, only the one
// that deleted the row releases its bytes.
func TestConcurrentDeleteReleasesOnce(t *testing.T) {
	for i := 0; i < 200; i++ {
		f := &measuringFiles{fakeFiles: newFiles()}
		r := newRepo()
		st := NewStorage(0, 0)
		s := New(Deps{Parser: &fakeParser{}, Repo: r, Files: f, Storage: st, Logger: slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))})
		r.rows["w"] = Record{}
		f.dirs["w"] = []File{{Name: "original.fit", Data: make([]byte, 100)}}
		st.Add(500) // other workouts, so a double release is not hidden by the clamp at 0
		st.Add(100)
		f.sizes.Add(2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); _ = s.Delete(context.Background(), "w", "") }()
		}
		wg.Wait()
		if got := st.Used(); got != 500 {
			t.Fatalf("iteration %d: used %d, want 500", i, got)
		}
	}
}
