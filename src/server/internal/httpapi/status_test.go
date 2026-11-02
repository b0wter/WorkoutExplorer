package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
)

const msgStorageFull = "Uploads are paused because storage is full. Please try again later."

// gate is a StorageGate the test opens and closes.
type gate struct{ open atomic.Bool }

func (g *gate) UploadsOpen() bool { return g.open.Load() }

func newGateEnv(t *testing.T) (*env, *gate) {
	t.Helper()
	g := &gate{}
	g.open.Store(true)
	e := &env{up: &fakeUploader{}, logs: &logLines{}, clk: &clock{t: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}}
	e.h = NewHandler(Config{Version: "t", Uploads: e.up, Storage: g, Logger: slog.New(slog.NewJSONHandler(e.logs, nil)), ParseTimeout: 10 * time.Second, MaxConcurrentUploads: 4, Now: e.clk.now})
	return e, g
}

// TestStatus (AC 2): GET /api/status is {"uploads_open": <bool>} and nothing
// else, follows the gate without a restart, and is never cached.
func TestStatus(t *testing.T) {
	e, g := newGateEnv(t)
	for _, open := range []bool{true, false, true} {
		g.open.Store(open)
		rec := do(e.h, httptest.NewRequest(http.MethodGet, "/api/status", nil))
		want := fmt.Sprintf(`{"uploads_open":%v}`, open)
		if rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Errorf("open=%v: %d %s, want %s", open, rec.Code, rec.Body, want)
		}
		if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "application/json" || cc != "no-store" {
			t.Errorf("headers: %q %q", ct, cc)
		}
	}
	if rec := do(e.h, httptest.NewRequest(http.MethodHead, "/api/status", nil)); rec.Code != http.StatusOK {
		t.Errorf("HEAD: %d", rec.Code)
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := do(e.h, httptest.NewRequest(m, "/api/status", nil)); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want 405", m, rec.Code)
		}
	}
	// Without a gate (the health-only handler of older tests) there is no route.
	if rec := do(NewHandler(Config{MaxConcurrentUploads: 1}), httptest.NewRequest(http.MethodGet, "/api/status", nil)); rec.Code != http.StatusNotFound {
		t.Errorf("no gate: %d", rec.Code)
	}
}

// TestStorageGate (AC 1): while the gate is closed every upload gets 503
// storage_full with UP-8's sentence, before the in-flight slot and the rate
// limit (so it does not count), and the uploader is never called.
func TestStorageGate(t *testing.T) {
	e, g := newGateEnv(t)
	g.open.Store(false)
	for i := range 70 {
		rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
		b := decodeErr(t, rec)
		if rec.Code != http.StatusServiceUnavailable || b.Error != "storage_full" || b.Message != msgStorageFull {
			t.Fatalf("attempt %d: %d %+v", i, rec.Code, b)
		}
		if rec.Header().Get("Retry-After") != "" {
			t.Fatalf("Retry-After set")
		}
	}
	if e.up.count() != 0 {
		t.Errorf("uploader called %d times", e.up.count())
	}
	if !strings.Contains(e.logs.String(), `"msg":"upload refused","reason":"storage_full","status":503`) {
		t.Errorf("log: %s", e.logs.String())
	}
	// Open again: the refused attempts did not use up the rate limit.
	g.open.Store(true)
	for i := range 60 {
		if rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", "")); rec.Code != http.StatusCreated {
			t.Fatalf("upload %d after reopening: %d", i, rec.Code)
		}
	}
}

// TestStorageFullFromUpload (AC 1): an upload that passed the gate but lost
// the race at the cap (lifecycle.ErrStorageFull) gets the same answer.
func TestStorageFullFromUpload(t *testing.T) {
	e, _ := newGateEnv(t)
	e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) {
		return lifecycle.Created{}, fmt.Errorf("wrapped: %w", lifecycle.ErrStorageFull)
	}
	rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
	if b := decodeErr(t, rec); rec.Code != http.StatusServiceUnavailable || b.Error != "storage_full" || b.Message != msgStorageFull {
		t.Errorf("%d %+v", rec.Code, b)
	}
}
