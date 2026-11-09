package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workout-explorer/internal/shell"
)

// F-38 end to end without Caddy (architect/F-38-impressum-page.md); the
// Caddy side is TestCaddyImpressum.

const impressumMarker = "Distinctive-Impressum-Marker-7Q2"

// TestImpressumSetting (IM-1, AC 13): unset is off; a valid file is rendered
// by the operator file's parser; every bad file stops serve and regenerate
// with exit 2 naming the setting, before the data directory is touched.
func TestImpressumSetting(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), mode)
		return p
	}
	if s, err := readSettings(envOf(map[string]string{envImpressumFile: write("ok", "## Imp\n"+impressumMarker+" <b>\n", 0o644)})); err != nil ||
		s.impressumHTML != "<h2>Imp</h2>\n<p>"+impressumMarker+" &lt;b&gt;</p>\n" {
		t.Errorf("ok = %q, %v", s.impressumHTML, err)
	}
	if s, err := readSettings(envOf(map[string]string{})); err != nil || s.impressumHTML != "" {
		t.Errorf("unset = %q, %v", s.impressumHTML, err)
	}
	bad := map[string]string{"relative": "impressum.txt", "missing": filepath.Join(dir, "nope"), "directory": dir,
		"big": write("big", strings.Repeat("a", 16<<10+1), 0o644), "utf8": write("u8", "a\xffb", 0o644), "control": write("ctl", "a\x00b", 0o644),
		"empty": write("empty", "", 0o644), "blank": write("blank", " \n\t\n", 0o644)}
	if os.Geteuid() != 0 { // root reads anything
		bad["unreadable"] = write("unreadable", "## x\n", 0o000)
	}
	for name, v := range bad {
		if _, err := readSettings(envOf(map[string]string{envImpressumFile: v})); err == nil || !strings.Contains(err.Error(), envImpressumFile) {
			t.Errorf("%s: %v", name, err)
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv(envImpressumFile, v)
			data := filepath.Join(t.TempDir(), "data")
			var stderr bytes.Buffer
			if got := run([]string{"serve", "-data", data, "-web", testWeb, "-addr", "127.0.0.1:0"}, nil, &stderr); got != 2 || !strings.Contains(stderr.String(), envImpressumFile) {
				t.Errorf("serve = %d, %q", got, stderr.String())
			}
			env := map[string]string{envBaseURL: testBaseURL, envImpressumFile: v}
			stderr.Reset()
			if got := regenerateCmd([]string{"-data", data, "-web", testWeb}, func(k string) string { return env[k] }, io.Discard, &stderr); got != 2 ||
				!strings.Contains(stderr.String(), "regenerate: "+envImpressumFile) {
				t.Errorf("regenerate = %d, %q", got, stderr.String())
			}
			if _, err := os.Stat(data); err == nil {
				t.Error("data directory created")
			}
		})
	}
}

// TestImpressumFooterAndAPI (IM-4, IM-7, ADR-043; AC 1, 3, 12): with the
// setting every page type has the footer link and only /api/impressum has the
// text; after a start without it and a regenerate, none has either.
func TestImpressumFooterAndAPI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	block := "<h2>Imp</h2>\n<p>" + impressumMarker + "</p>\n"
	start := func(im string) (*app, func() error) {
		a := &app{t: t, dir: dir, logs: &syncBuffer{}}
		h, closeApp, err := newApp(context.Background(), dir, settings{webDir: testWeb, baseURL: testBaseURL, maxConcurrentUploads: 4, impressumHTML: im},
			slog.New(slog.NewJSONHandler(a.logs, nil)))
		if err != nil {
			t.Fatal(err)
		}
		a.h = h
		return a, closeApp
	}
	get := func(a *app, path string, hdr ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		for i := 0; i+1 < len(hdr); i += 2 {
			r.Header.Set(hdr[i], hdr[i+1])
		}
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, r)
		return rec
	}
	// pages are every page body a reader can get: the shells, About and the published page.
	pages := func(a *app, pub string) map[string]string {
		out := map[string]string{"about": get(a, "/about").Body.String(), "published": a.page(pub)}
		for _, n := range append(shell.Shells, shell.Impressum) {
			if b, err := os.ReadFile(filepath.Join(dir, "site", n)); err == nil {
				out[n] = string(b)
			}
		}
		return out
	}
	hdr := []string{"Wx-Impressum", "1", "Sec-Fetch-Site", "same-origin"}

	a, closeA := start(block)
	pub, _ := a.published("ride-30min.fit")
	got := pages(a, pub)
	if len(got) != len(shell.Shells)+3 {
		t.Errorf("pages %d", len(got))
	}
	for name, body := range got {
		if strings.Count(body, shell.Link) != 1 || strings.Contains(body, shell.Marker) || strings.Contains(body, impressumMarker) {
			t.Errorf("set: %s has %d links, a marker or the text", name, strings.Count(body, shell.Link))
		}
	}
	if rec := get(a, "/api/impressum", hdr...); rec.Code != 200 || rec.Body.String() != block {
		t.Errorf("api: %d %q", rec.Code, rec.Body)
	}
	unknown := get(a, "/api/nothing-here")
	if rec := get(a, "/api/impressum"); rec.Code != 404 || rec.Body.String() != unknown.Body.String() {
		t.Errorf("api without the header: %d %q", rec.Code, rec.Body)
	}
	if fi, err := os.Stat(filepath.Join(dir, "site")); err != nil || fi.Mode().Perm() != 0o750 {
		t.Errorf("site/: %v %v", fi, err)
	}
	closeA()

	// Unset: restart, then regenerate for the published page (OQ-120).
	b, closeB := start("")
	defer closeB()
	if strings.Count(b.page(pub), shell.Link) != 1 {
		t.Error("published page changed before regenerate")
	}
	if r := runRegen(t, context.Background(), dir, testWeb, nil); r.code != 0 {
		t.Fatalf("regenerate: %d %s", r.code, r.stderr)
	}
	got = pages(b, pub)
	if len(got) != len(shell.Shells)+2 {
		t.Errorf("unset: impressum shell left (%d pages)", len(got))
	}
	for name, body := range got {
		if strings.Contains(body, "Impressum") || strings.Contains(body, shell.Marker) {
			t.Errorf("unset: %s names the Impressum", name)
		}
	}
	if rec := get(b, "/api/impressum", hdr...); rec.Code != 404 || rec.Body.String() != unknown.Body.String() {
		t.Errorf("unset api: %d %q", rec.Code, rec.Body)
	}
	// And back on through regenerate alone.
	var out, errb bytes.Buffer
	if code := regenerate(context.Background(), dir, settings{webDir: testWeb, baseURL: testBaseURL, impressumHTML: block}, &out, &errb, nil); code != 0 ||
		strings.Count(b.page(pub), shell.Link) != 1 {
		t.Errorf("regenerate with the setting: %d %s", code, errb.String())
	}
}
