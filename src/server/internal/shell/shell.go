// Package shell fills the per-deployment markers of the static pages (ADR-043,
// F-38 IM-7) and is the only writer under {data}/site/, where Caddy serves the
// filled page shells.
//
// Contract with the frontend: every page source with a footer carries
// Marker exactly once (the shells below, about.html and the page template
// index.html). It becomes Link while an Impressum is set, else nothing.
package shell

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Marker is the one footer marker of every page source.
	Marker = "<!--wx:impressum-->"
	// Link is what Marker becomes while an Impressum is set.
	Link = `<a href="/impressum">Impressum</a>`
	// Impressum is the shell written only while an Impressum is set (IM-2).
	Impressum = "impressum.html"
	dirMode   = 0o750 // as public/: Caddy reads through app's group (F-31 DP-5)
	fileMode  = 0o644
)

// Shells are the page shells always written (upload, generic, edit, confirmation).
var Shells = []string{"upload.html", "generic.html", "edit.html", "report-delete.html"}

// Fill replaces the one Marker in src (named name in the error) with Link when
// on, else with nothing. Anything but exactly one marker is an error.
func Fill(name, src string, on bool) (string, error) {
	if n := strings.Count(src, Marker); n != 1 {
		return "", fmt.Errorf("%s: needs exactly one %s marker, has %d", name, Marker, n)
	}
	link := ""
	if on {
		link = Link
	}
	return strings.Replace(src, Marker, link, 1), nil
}

// Write fills every shell of webDir and writes it to <dataDir>/site/ (temporary
// file, then rename). The Impressum shell is written only when impressum is
// set and removed otherwise, so Caddy answers 404.
func Write(dataDir, webDir string, impressum bool) error {
	site := filepath.Join(dataDir, "site")
	if err := os.MkdirAll(site, dirMode); err != nil {
		return fmt.Errorf("page shells: %w", err)
	}
	names := Shells
	if impressum {
		names = append(names[:len(names):len(names)], Impressum)
	} else if err := os.Remove(filepath.Join(site, Impressum)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("page shells: %w", err)
	}
	for _, name := range names {
		src, err := os.ReadFile(filepath.Join(webDir, name))
		if err != nil {
			return fmt.Errorf("page shells: %w", err)
		}
		out, err := Fill(name, string(src), impressum)
		if err != nil {
			return fmt.Errorf("page shells: %w", err)
		}
		if err := writeFile(filepath.Join(site, name), []byte(out)); err != nil {
			return fmt.Errorf("page shells: %w", err)
		}
	}
	return nil
}

// writeFile replaces path through a fixed dot-named temporary file, so a
// crash leaves at most one stale temporary, overwritten at the next start.
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
