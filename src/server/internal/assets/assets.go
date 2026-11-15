// Package assets copies the web build's assets into {data}/site/assets/, where
// Caddy serves them, and removes the bundles no served page needs (ADR-053,
// F-30 RG-10). It is the only writer of site/assets/ and bundles/.
//
// A bundle list is the sorted file names of one build's assets/, one per line,
// stored as bundles/<key>, where key is derived from the list itself, so
// self-built and dev versions get lists too.
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	dirMode  = 0o750 // as public/: Caddy reads through app's group (F-31 DP-5)
	fileMode = 0o644
	// FirstList holds the files already in site/assets when bundles/ is created
	// (an install migrated from web/assets, F-59 IP-17).
	FirstList = "pre-f59"
)

func siteAssets(dataDir string) string { return filepath.Join(dataDir, "site", "assets") }
func bundles(dataDir string) string    { return filepath.Join(dataDir, "bundles") }

// Copy adds the files of <webDir>/assets to <dataDir>/site/assets: a file with
// the same bytes stays, anything else is written by temporary name and
// renamed. It never removes a file. It records the build's names as a list
// and returns that list's name, the current bundle for Clean.
func Copy(dataDir, webDir string) (string, error) {
	src := filepath.Join(webDir, "assets")
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", fmt.Errorf("assets: %w", err)
	}
	dst := siteAssets(dataDir)
	if err := os.MkdirAll(dst, dirMode); err != nil {
		return "", fmt.Errorf("assets: %w", err)
	}
	if err := firstList(dataDir); err != nil {
		return "", fmt.Errorf("assets: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			return "", fmt.Errorf("assets: %s is not a regular file", e.Name())
		}
		names = append(names, e.Name())
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return "", fmt.Errorf("assets: %w", err)
		}
		if err := put(filepath.Join(dst, e.Name()), b); err != nil {
			return "", fmt.Errorf("assets: %w", err)
		}
	}
	list := []byte(strings.Join(names, "\n") + "\n") // ReadDir sorts
	sum := sha256.Sum256(list)
	key := hex.EncodeToString(sum[:8])
	if err := put(filepath.Join(bundles(dataDir), key), list); err != nil {
		return "", fmt.Errorf("assets: %w", err)
	}
	return key, nil
}

// firstList creates bundles/ once, with the files already in site/assets as
// FirstList (none: no list). The directory appears by rename, so a crash
// leaves no half-made bundles/.
func firstList(dataDir string) error {
	dir := bundles(dataDir)
	if _, err := os.Stat(dir); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp := dir + ".new"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.Mkdir(tmp, dirMode); err != nil {
		return err
	}
	names, err := files(siteAssets(dataDir))
	if err != nil {
		return err
	}
	if len(names) > 0 {
		if err := os.WriteFile(filepath.Join(tmp, FirstList), []byte(strings.Join(names, "\n")+"\n"), fileMode); err != nil {
			return err
		}
	}
	return os.Rename(tmp, dir)
}

// Clean keeps the list current and every other list that names an asset in
// used that is not on current (names are content hashes, QA-30-r1-1); it
// removes every other file of site/assets, then every list not kept. Call it
// only after a run that processed every workout (F-30 RG-10).
func Clean(dataDir, current string, used []string) (filesRemoved, listsRemoved int, err error) {
	keep, err := readList(filepath.Join(bundles(dataDir), current))
	if err != nil {
		return 0, 0, fmt.Errorf("assets: current list: %w", err)
	}
	var old []string
	for _, u := range used {
		if !keep[u] {
			old = append(old, u)
		}
	}
	lists, err := files(bundles(dataDir))
	if err != nil {
		return 0, 0, fmt.Errorf("assets: %w", err)
	}
	var drop []string
	for _, l := range lists {
		if l == current {
			continue
		}
		names, err := readList(filepath.Join(bundles(dataDir), l))
		if err != nil {
			return 0, 0, fmt.Errorf("assets: %w", err)
		}
		if !slices.ContainsFunc(old, func(u string) bool { return names[u] }) {
			drop = append(drop, l)
			continue
		}
		for n := range names {
			keep[n] = true
		}
	}
	present, err := files(siteAssets(dataDir))
	if err != nil {
		return 0, 0, fmt.Errorf("assets: %w", err)
	}
	for _, f := range present {
		if keep[f] {
			continue
		}
		if err := os.Remove(filepath.Join(siteAssets(dataDir), f)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return filesRemoved, 0, fmt.Errorf("assets: %w", err)
		}
		filesRemoved++
	}
	for _, l := range drop {
		if err := os.Remove(filepath.Join(bundles(dataDir), l)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return filesRemoved, listsRemoved, fmt.Errorf("assets: %w", err)
		}
		listsRemoved++
	}
	return filesRemoved, listsRemoved, nil
}

// files lists the regular files of dir, without dot names (in-flight
// temporaries of a concurrent Copy). A missing dir has none.
func files(dir string) ([]string, error) {
	es, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range es {
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func readList(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for _, n := range strings.Split(string(b), "\n") {
		if n != "" {
			m[n] = true
		}
	}
	return m, nil
}

// put writes b to path unless path already holds the same bytes.
func put(path string, b []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	return writeFile(path, b)
}

// writeFile replaces path through a fixed dot-named temporary file, so a
// crash leaves at most one stale temporary, overwritten at the next copy.
func writeFile(path string, b []byte) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".new")
	if err := os.WriteFile(tmp, b, fileMode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, fileMode); err != nil { // a leftover keeps its old mode under WriteFile
		return err
	}
	return os.Rename(tmp, path)
}
