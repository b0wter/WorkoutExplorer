// Package shell fills the per-deployment markers of the static pages (ADR-043,
// F-38 IM-7) and is the only writer under {data}/site/, where Caddy serves the
// filled page shells.
//
// Contract with the frontend: every page source with a footer carries
// Marker exactly once (the shells below, about.html and the page template
// index.html). It becomes Link while an Impressum is set, else nothing.
//
// upload.html (and about.html, package about) also carries exactly one
// ShortcutOn...ShortcutOff block, kept while the iOS Shortcut link is set and
// dropped with its markers otherwise; ShortcutURL inside it becomes the link,
// HTML-escaped (F-34, F-63).
package shell

import (
	"errors"
	"fmt"
	"html"
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
	// The iOS Shortcut block and link markers (F-34, F-63).
	ShortcutOn  = "<!--wx:shortcut-->"
	ShortcutOff = "<!--/wx:shortcut-->"
	ShortcutURL = "<!--wx:shortcut-url-->"
	upload      = "upload.html" // the one shell with the Shortcut hint (F-63)
	dirMode     = 0o750         // as public/: Caddy reads through app's group (F-31 DP-5)
	fileMode    = 0o644
)

// Shells are the page shells always written (upload, generic, edit, confirmation, my workouts).
var Shells = []string{"upload.html", "generic.html", "edit.html", "report-delete.html", "my-workouts.html"}

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
// set and removed otherwise, so Caddy answers 404. shortcut is the iOS
// Shortcut link of the upload page's hint; "" drops the hint (F-63).
func Write(dataDir, webDir string, impressum bool, shortcut string) error {
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
		if err == nil && name == upload {
			out, err = fillShortcut(out, shortcut)
		}
		if err != nil {
			return fmt.Errorf("page shells: %w", err)
		}
		if err := writeFile(filepath.Join(site, name), []byte(out)); err != nil {
			return fmt.Errorf("page shells: %w", err)
		}
	}
	return nil
}

// CutBlock drops the markers of the one on...off pair in s, and its content
// unless keep. ok is false unless each marker occurs once, in order.
func CutBlock(s, on, off string, keep bool) (string, bool) {
	i, j := strings.Index(s, on), strings.Index(s, off)
	if i < 0 || j < i || strings.Count(s, on) != 1 || strings.Count(s, off) != 1 {
		return s, false
	}
	inner := s[i+len(on) : j]
	if !keep {
		inner = ""
	}
	return s[:i] + inner + s[j+len(off):], true
}

// fillShortcut keeps the upload page's hint block only with a link and puts
// the link in, HTML-escaped (F-63).
func fillShortcut(src, link string) (string, error) {
	s, ok := CutBlock(src, ShortcutOn, ShortcutOff, link != "")
	if !ok {
		return "", fmt.Errorf("%s: the shortcut block is missing or malformed", upload)
	}
	return strings.ReplaceAll(s, ShortcutURL, html.EscapeString(link)), nil
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
