package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const committedDir = "../../testdata/corpus/synthetic"

// TestSyntheticCorpusCurrent checks that the committed synthetic files are
// exactly what the generator writes (ADR-004 item 5).
func TestSyntheticCorpusCurrent(t *testing.T) {
	const fix = "run make corpus-gen, then make corpus-update"
	out := t.TempDir()
	if err := generate(out); err != nil {
		t.Fatalf("generate: %v", err)
	}
	generated := fitFiles(t, out)
	committed := fitFiles(t, committedDir)

	for _, name := range generated {
		want, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(committedDir, name))
		if os.IsNotExist(err) {
			t.Errorf("%s is generated but not committed: %s", name, fix)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from the generator output: %s", name, fix)
		}
	}
	gen := map[string]bool{}
	for _, name := range generated {
		gen[name] = true
	}
	for _, name := range committed {
		if !gen[name] {
			t.Errorf("%s is committed but not generated: %s", name, fix)
		}
	}
}

func fitFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".fit") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
