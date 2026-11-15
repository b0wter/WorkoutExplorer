package assets

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// web makes a web build whose assets/ holds the given name/content pairs.
func web(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "assets"), 0o755)
	for i := 0; i < len(files); i += 2 {
		write(t, filepath.Join(dir, "assets", files[i]), files[i+1])
	}
	return dir
}

// ADR-053 item 2: additive, identical files untouched, changed ones replaced,
// one list per build keyed by its names, the first list from what was there.
func TestCopy(t *testing.T) {
	data := t.TempDir()
	site := filepath.Join(data, "site", "assets")
	// A migrated install: the old web/assets were copied in by hand (F-59 IP-17).
	write(t, filepath.Join(site, "old.js"), "old")
	write(t, filepath.Join(site, "same.css"), "same")
	write(t, filepath.Join(site, "changed.js"), "stale")
	past := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(site, "same.css"), past, past)

	w1 := web(t, "a.js", "a", "same.css", "same", "changed.js", "new")
	key, err := Copy(data, w1)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(t, site); !slices.Equal(got, []string{"a.js", "changed.js", "old.js", "same.css"}) {
		t.Errorf("site/assets %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(site, "changed.js")); string(b) != "new" {
		t.Errorf("changed.js %q", b)
	}
	if fi, _ := os.Stat(filepath.Join(site, "same.css")); !fi.ModTime().Equal(past) {
		t.Error("an identical file was rewritten")
	}
	if fi, _ := os.Stat(filepath.Join(site, "a.js")); fi.Mode().Perm() != fileMode {
		t.Errorf("mode %v", fi.Mode())
	}
	if fi, _ := os.Stat(filepath.Join(data, "bundles")); fi.Mode().Perm() != dirMode {
		t.Errorf("bundles mode %v", fi.Mode())
	}
	lists := names(t, filepath.Join(data, "bundles"))
	if !slices.Equal(lists, []string{key, FirstList}) {
		t.Fatalf("lists %v (key %s)", lists, key)
	}
	if b, _ := os.ReadFile(filepath.Join(data, "bundles", FirstList)); string(b) != "changed.js\nold.js\nsame.css\n" {
		t.Errorf("first list %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(data, "bundles", key)); string(b) != "a.js\nchanged.js\nsame.css\n" {
		t.Errorf("list %q", b)
	}

	// Again (serve's next start, regenerate): same key, nothing new; the first list is written once.
	if k, err := Copy(data, w1); err != nil || k != key || !slices.Equal(names(t, filepath.Join(data, "bundles")), lists) {
		t.Errorf("second copy: %s %v %v", k, err, names(t, filepath.Join(data, "bundles")))
	}

	// The next release adds its list and files and removes nothing.
	w2 := web(t, "b.js", "b", "same.css", "same")
	key2, err := Copy(data, w2)
	if err != nil || key2 == key {
		t.Fatal(key2, err)
	}
	if got := names(t, site); !slices.Equal(got, []string{"a.js", "b.js", "changed.js", "old.js", "same.css"}) {
		t.Errorf("a copy removed something: %v", got)
	}
}

// A fresh install has no first list; a build without assets/ is an error.
func TestCopyFresh(t *testing.T) {
	data := t.TempDir()
	key, err := Copy(data, web(t, "a.js", "a"))
	if err != nil || !slices.Equal(names(t, filepath.Join(data, "bundles")), []string{key}) {
		t.Errorf("%v %v", err, names(t, filepath.Join(data, "bundles")))
	}
	if _, err := Copy(data, t.TempDir()); err == nil {
		t.Error("no assets/: accepted")
	}
	// A half-made bundles/ of a crash is redone.
	data = t.TempDir()
	write(t, filepath.Join(data, "bundles.new", "junk"), "x")
	if _, err := Copy(data, web(t, "a.js", "a")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "bundles.new")); err == nil {
		t.Error("bundles.new left")
	}
}

// F-30 RG-10: whole lists are kept for any old asset a served page names.
func TestClean(t *testing.T) {
	data := t.TempDir()
	site := filepath.Join(data, "site", "assets")
	lists := map[string][]string{
		FirstList: {"a.js", "shared.css"},
		"v1":      {"b.js", "b-map.js", "b-worker.js", "shared.css"}, // lazy chunks are named only inside JavaScript
		"v2":      {"c.js", "c-map.js", "shared.css"},
	}
	for l, ns := range lists {
		write(t, filepath.Join(data, "bundles", l), strings.Join(ns, "\n")+"\n")
		for _, n := range ns {
			write(t, filepath.Join(site, n), n)
		}
	}
	write(t, filepath.Join(site, "stray.js"), "x")
	write(t, filepath.Join(site, ".c.js.new"), "in flight") // a concurrent copy's temporary

	// No list for the current build: an error, nothing removed.
	if _, _, err := Clean(data, "v9", []string{"c.js"}); err == nil || len(names(t, site)) != 9 {
		t.Errorf("missing list: %v %v", err, names(t, site))
	}

	// A kept page names v1's entry: its whole bundle stays, chunks included.
	f, l, err := Clean(data, "v2", []string{"b.js", "c.js"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".c.js.new", "b-map.js", "b-worker.js", "b.js", "c-map.js", "c.js", "shared.css"}; !slices.Equal(names(t, site), want) {
		t.Errorf("assets %v", names(t, site))
	}
	if !slices.Equal(names(t, filepath.Join(data, "bundles")), []string{"v1", "v2"}) || f != 2 || l != 1 {
		t.Errorf("bundles %v, %d %d", names(t, filepath.Join(data, "bundles")), f, l)
	}

	// A later run rebuilt the kept page: v1 goes, files and list.
	if _, _, err := Clean(data, "v2", []string{"c.js"}); err != nil {
		t.Fatal(err)
	}
	if want := []string{".c.js.new", "c-map.js", "c.js", "shared.css"}; !slices.Equal(names(t, site), want) || !slices.Equal(names(t, filepath.Join(data, "bundles")), []string{"v2"}) {
		t.Errorf("assets %v bundles %v", names(t, site), names(t, filepath.Join(data, "bundles")))
	}

	// A JS-only release: a page names the unchanged stylesheet an old list
	// shares. That name keeps no old list (QA-30-r1-1).
	write(t, filepath.Join(data, "bundles", FirstList), "a.js\nshared.css\n")
	write(t, filepath.Join(site, "a.js"), "a")
	if f, l, err := Clean(data, "v2", []string{"c.js", "shared.css"}); err != nil || slices.Contains(names(t, site), "a.js") ||
		!slices.Equal(names(t, filepath.Join(data, "bundles")), []string{"v2"}) || f != 1 || l != 1 {
		t.Errorf("shared name: %v %d %d assets %v", err, f, l, names(t, site))
	}

	// No page at all: the current bundle stays whole, list included.
	if _, _, err := Clean(data, "v2", nil); err != nil || !slices.Equal(names(t, site), []string{".c.js.new", "c-map.js", "c.js", "shared.css"}) ||
		!slices.Equal(names(t, filepath.Join(data, "bundles")), []string{"v2"}) {
		t.Errorf("%v assets %v bundles %v", err, names(t, site), names(t, filepath.Join(data, "bundles")))
	}
}
