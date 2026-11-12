package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	js := []byte(strings.Repeat("let a = 1;\n", 500))
	os.WriteFile(filepath.Join(dir, "a-1.js"), js, 0o644)
	os.WriteFile(filepath.Join(dir, "tiny.css"), []byte("a"), 0o644) // brotli saves nothing
	os.WriteFile(filepath.Join(dir, "p.png"), js, 0o644)             // not text-like
	for range 2 {                                                    // idempotent
		if n, err := run(dir); err != nil || n != 1 {
			t.Fatalf("run: %d, %v", n, err)
		}
	}
	for _, none := range []string{"tiny.css.br", "p.png.br", "a-1.js.br.br"} {
		if _, err := os.Stat(filepath.Join(dir, none)); err == nil {
			t.Errorf("%s written", none)
		}
	}
	f, _ := os.Open(filepath.Join(dir, "a-1.js.br"))
	got, err := io.ReadAll(brotli.NewReader(f))
	if err != nil || !bytes.Equal(got, js) {
		t.Errorf("round trip: %v", err)
	}
}
