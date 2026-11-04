package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// deploy/clean-assets.sh (F-30 RG-10) in a scratch deploy directory.
var cleanAssets, _ = filepath.Abs(filepath.Join("..", "..", "..", "..", "deploy", "clean-assets.sh"))

func TestCleanAssets(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lists := map[string][]string{
		"pre-f30": {"a.js", "shared.css"},
		"v1.0.0":  {"b.js", "b-map.js", "b-worker.js", "shared.css"}, // lazy chunks are named only inside JavaScript
		"v1.1.0":  {"c.js", "c-map.js", "shared.css"},
	}
	for tag, names := range lists {
		write("bundles/"+tag, strings.Join(names, "\n")+"\n")
		for _, n := range names {
			write("web/assets/"+n, n)
		}
	}
	write("web/assets/stray.js", "x")
	clean := func(tag string, used ...string) (string, error) {
		t.Helper()
		cmd := exec.Command("sh", cleanAssets, tag)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(strings.Join(used, "\n") + "\n")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	names := func(sub string) []string {
		t.Helper()
		es, _ := os.ReadDir(filepath.Join(dir, sub))
		var out []string
		for _, e := range es {
			out = append(out, e.Name())
		}
		return out
	}

	// No list for the current release: nothing is removed.
	if out, err := clean("v9.9.9", "c.js"); err == nil || len(names("web/assets")) != 8 {
		t.Errorf("missing list: %v %s", err, out)
	}

	// A kept page names v1.0.0's entry: its whole release stays, chunks included.
	out, err := clean("v1.1.0", "b.js", "c.js")
	if err != nil {
		t.Fatal(err, out)
	}
	if want := []string{"b-map.js", "b-worker.js", "b.js", "c-map.js", "c.js", "shared.css"}; !slices.Equal(names("web/assets"), want) {
		t.Errorf("assets %v", names("web/assets"))
	}
	// pre-f30 is not kept: its list goes; shared.css stays on the kept lists.
	if !slices.Equal(names("bundles"), []string{"v1.0.0", "v1.1.0"}) || out != "files_removed=2\nbundles_removed=1\n" {
		t.Errorf("bundles %v, %q", names("bundles"), out)
	}

	// A later run rebuilt the kept page: v1.0.0 goes, files and list.
	if out, err = clean("v1.1.0", "c.js"); err != nil {
		t.Fatal(err, out)
	}
	if want := []string{"c-map.js", "c.js", "shared.css"}; !slices.Equal(names("web/assets"), want) || !slices.Equal(names("bundles"), []string{"v1.1.0"}) {
		t.Errorf("assets %v bundles %v", names("web/assets"), names("bundles"))
	}

	// A used name that an old list shares keeps that list too (over-keeping is the safe side).
	write("bundles/pre-f30", "a.js\nshared.css\n")
	write("web/assets/a.js", "a")
	if out, err = clean("v1.1.0", "shared.css"); err != nil || !slices.Contains(names("web/assets"), "a.js") || !slices.Contains(names("bundles"), "pre-f30") {
		t.Errorf("shared name: %v %q", err, out)
	}
	os.Remove(filepath.Join(dir, "web/assets/a.js"))
	os.Remove(filepath.Join(dir, "bundles/pre-f30"))

	// No page at all: only the current release stays; a list with no file left goes.
	write("bundles/old", "gone.js\n")
	if out, err = clean("v1.1.0"); err != nil || !slices.Equal(names("web/assets"), []string{"c-map.js", "c.js", "shared.css"}) ||
		!slices.Contains(strings.Split(out, "\n"), "bundles_removed=1") {
		t.Errorf("%v %q assets %v bundles %v", err, out, names("web/assets"), names("bundles"))
	}
}
