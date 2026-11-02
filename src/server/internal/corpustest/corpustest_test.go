package corpustest

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFilesEmptyReal: a real/ that holds only .gitkeep (a fresh checkout,
// CI) gives no files and no failure; a missing folder gives none either.
func TestFilesEmptyReal(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"real", "expected/real"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, ".gitkeep"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if files := Files(t, root, "real"); len(files) != 0 {
		t.Errorf("files = %v, want none", files)
	}
	if files := Files(t, root, "synthetic"); len(files) != 0 {
		t.Errorf("missing folder: files = %v, want none", files)
	}
}

// recorder captures Errorf calls so the name rule can be tested without
// failing this test.
type recorder struct {
	testing.TB
	errors int
}

func (r *recorder) Helper()               {}
func (r *recorder) Errorf(string, ...any) { r.errors++ }
func (r *recorder) Fatal(args ...any)     { r.TB.Fatal(args...) }

// TestFilesNameRule: only lower-case regular *.fit files count; anything else
// except hidden files is an error (ADR-004 item 4).
func TestFilesNameRule(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "synthetic")
	for _, name := range []string{"b.fit", "a.fit", "GARMIN01.FIT", "notes.txt", ".hidden"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.fit"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &recorder{TB: t}
	files := Files(r, root, "synthetic")
	want := []string{filepath.Join(dir, "a.fit"), filepath.Join(dir, "b.fit")}
	if len(files) != len(want) || files[0] != want[0] || files[1] != want[1] {
		t.Errorf("files = %v, want %v", files, want)
	}
	if r.errors != 3 {
		t.Errorf("errors = %d, want 3 (GARMIN01.FIT, notes.txt, sub.fit)", r.errors)
	}
}
