package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workout-explorer/internal/lifecycle"
)

// storageApp is one server run on a data directory that outlives it.
type storageApp struct {
	*app
	svc   *lifecycle.Service
	st    *lifecycle.Storage
	close func() error
}

func openStorageApp(t *testing.T, dir string, limit int64) *storageApp {
	t.Helper()
	sa := &storageApp{app: &app{t: t, dir: dir, logs: &syncBuffer{}}}
	s := settings{maxConcurrentUploads: 4, storageLimit: limit, webDir: testWeb, baseURL: testBaseURL, onService: func(svc *lifecycle.Service, st *lifecycle.Storage) {
		sa.svc, sa.st = svc, st
	}}
	h, closeApp, err := newApp(context.Background(), dir, s, slog.New(slog.NewJSONHandler(sa.logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	sa.h, sa.close = h, closeApp
	t.Cleanup(func() { closeApp() })
	return sa
}

func (a *storageApp) uploadsOpen() bool {
	a.t.Helper()
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || len(body) != 1 {
		a.t.Fatalf("status: %d %s", rec.Code, rec.Body)
	}
	open, ok := body["uploads_open"].(bool)
	if !ok {
		a.t.Fatalf("status body %s", rec.Body)
	}
	return open
}

// diskUsage is the test's own measure: the sizes of the regular files under root.
func diskUsage(t *testing.T, root string) int64 {
	t.Helper()
	var n int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		n += info.Size()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

const storageFullMsg = "Uploads are paused because storage is full. Please try again later."

// TestStorageLimitEndToEnd (AC 1, AC 2, AC 3).
func TestStorageLimitEndToEnd(t *testing.T) {
	dir := t.TempDir()

	// A first run stores one workout under the default cap.
	a := openStorageApp(t, dir, 0)
	if a.st.Limit() != 1_000*1_000_000 {
		t.Errorf("default limit %d", a.st.Limit())
	}
	if _, _, rec := a.upload("ride-30min.fit"); rec.Code != http.StatusCreated {
		t.Fatalf("first upload: %d %s", rec.Code, rec.Body)
	}
	a.close()

	// AC 3: a restart recomputes the total from the disk; crash leftovers
	// (staging, a folder without a row) are removed first and never count.
	for _, p := range []string{filepath.Join("tmp", strings.Repeat("1", 32)), filepath.Join("private", strings.Repeat("2", 32))} {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p, "draft.bin"), make([]byte, 50_000), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	used := int64(0)
	{
		probe := openStorageApp(t, dir, 0)
		used = probe.st.Used()
		if disk := diskUsage(t, dir); used != disk {
			t.Errorf("AC 3: total after restart %d, on disk %d", used, disk)
		}
		if !strings.Contains(probe.logs.String(), `"msg":"storage","used_bytes":`) {
			t.Errorf("no storage line: %s", probe.logs.String())
		}
		probe.close()
	}

	// AC 1: the cap just above current usage. The next upload reaches it and
	// succeeds; every upload after that gets 503 with UP-8's sentence.
	a = openStorageApp(t, dir, used+1)
	privateAtStart := diskUsage(t, filepath.Join(dir, "private"))
	if !a.uploadsOpen() {
		t.Fatal("closed below the cap")
	}
	if _, _, rec := a.upload("ride-30min.fit"); rec.Code != http.StatusCreated {
		t.Fatalf("upload reaching the cap: %d %s", rec.Code, rec.Body)
	}
	if a.uploadsOpen() {
		t.Fatalf("AC 2: open at %d of %d", a.st.Used(), a.st.Limit())
	}
	for i := range 3 {
		_, _, rec := a.upload("ride-30min.fit")
		var b struct{ Error, Message string }
		_ = json.Unmarshal(rec.Body.Bytes(), &b)
		if rec.Code != http.StatusServiceUnavailable || b.Error != "storage_full" || b.Message != storageFullMsg {
			t.Fatalf("upload %d while full: %d %s", i, rec.Code, rec.Body)
		}
	}
	rows := a.rows()
	if len(rows) != 2 {
		t.Fatalf("%d rows", len(rows))
	}
	// The running total follows every write exactly (the database's own
	// growth is measured only at startup, F-10 design).
	if got, want := a.st.Used()-used, diskUsage(t, filepath.Join(dir, "private"))-privateAtStart; got != want {
		t.Errorf("running total grew by %d, private files by %d", got, want)
	}

	// AC 2: a delete frees space and reopens uploads without a restart.
	// Expiry (F-21) calls the same Delete.
	if err := a.svc.Delete(context.Background(), rows[0]["id"].(string), rows[0]["public_id"].(string)); err != nil {
		t.Fatal(err)
	}
	if !a.uploadsOpen() {
		t.Fatalf("AC 2: still closed at %d of %d after a delete", a.st.Used(), a.st.Limit())
	}
	if got, want := a.st.Used()-used, diskUsage(t, filepath.Join(dir, "private"))-privateAtStart; got != want {
		t.Errorf("after delete: running total %+d, private files %+d", got, want)
	}
	if _, _, rec := a.upload("ride-30min.fit"); rec.Code != http.StatusCreated {
		t.Fatalf("upload after delete: %d %s", rec.Code, rec.Body)
	}
	a.close()

	// AC 3 again, after uploads and a delete.
	a = openStorageApp(t, dir, used+1)
	if disk := diskUsage(t, dir); a.st.Used() != disk {
		t.Errorf("AC 3: total after restart %d, on disk %d", a.st.Used(), disk)
	}
	if a.uploadsOpen() {
		t.Error("open after restart while over the cap")
	}
}

// TestStorageFullRefusesOnlyUploads (AC 4, as far as F-10 can show it): while
// uploads are paused, the operations the cap must never refuse still run.
// Publishing (F-15) does not exist yet; it counts its files with Storage.Add,
// which never refuses, and F-15's end-to-end test publishes a draft while
// uploads are paused.
func TestStorageFullRefusesOnlyUploads(t *testing.T) {
	dir := t.TempDir()
	probe := openStorageApp(t, dir, 0)
	used := probe.st.Used()
	probe.close()
	a := openStorageApp(t, dir, used+1) // full after the first upload
	if _, _, rec := a.upload("ride-30min.fit"); rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d", rec.Code)
	}
	if a.uploadsOpen() {
		t.Fatal("open")
	}
	before := a.st.Used()
	a.st.Add(1_000) // what publish, edits and regeneration call
	if a.st.Used() != before+1_000 {
		t.Errorf("Add refused while full")
	}
	rows := a.rows()
	if err := a.svc.Delete(context.Background(), rows[0]["id"].(string), rows[0]["public_id"].(string)); err != nil {
		t.Errorf("delete while full: %v", err)
	}
	if len(a.rows()) != 0 || len(a.entries("private")) != 0 {
		t.Error("delete left data")
	}
	// Health stays up while full.
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("health: %d", rec.Code)
	}
}
