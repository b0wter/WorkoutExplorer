package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapePath(t *testing.T) {
	if got := escapePath("github.com/BurntSushi/toml"); got != "github.com/!burnt!sushi/toml" {
		t.Error(got)
	}
}

func TestCheckCSP(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "Caddyfile")
	csp := func(s string) { os.WriteFile(f, []byte(`header { Content-Security-Policy "`+s+`" }`), 0o644) }
	srcs, err := loadSources() // the committed file claims the real tile host
	if err != nil {
		t.Fatal(err)
	}
	csp("default-src 'self'; connect-src 'self' https://tiles.openfreemap.org")
	if err := checkCSP(f, srcs); err != nil {
		t.Error(err)
	}
	csp("default-src 'self'; font-src https://fonts.example.net")
	if err := checkCSP(f, srcs); err == nil || !strings.Contains(err.Error(), "https://fonts.example.net") {
		t.Errorf("unclaimed origin: %v", err)
	}
	os.WriteFile(f, []byte("nothing"), 0o644)
	if checkCSP(f, srcs) == nil {
		t.Error("no CSP line must fail")
	}
}

func TestNpmPackagesFailures(t *testing.T) {
	f := filepath.Join(t.TempDir(), "license.json")
	for name, body := range map[string]string{
		"no id":   `[{"name":"a","version":"1","identifier":"","text":"t"}]`,
		"no text": `[{"name":"a","version":"1","identifier":"MIT","text":" "}]`,
		"empty":   `[]`,
	} {
		os.WriteFile(f, []byte(body), 0o644)
		if _, err := npmPackages(f); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	os.WriteFile(f, []byte(`[{"name":"b","version":"1","identifier":"MIT","text":"x"},{"name":"a","version":"1","identifier":"MIT","text":"y"}]`), 0o644)
	if p, err := npmPackages(f); err != nil || p[0].name != "a" {
		t.Errorf("sorted: %v %v", p, err)
	}
}

// Names and texts are escaped; a text with a marker or a template action stays inert text.
func TestRenderEscapes(t *testing.T) {
	out := render(nil, []pkg{{"<x>", "1", "MIT", "<!--wx:version--> {{ . }} <script>"}}, nil)
	if strings.Contains(out, "<script>") || strings.Contains(out, "<x>") || strings.Count(out, "<!--wx:version-->") != 1 {
		t.Error(out)
	}
}

func TestCaddyPackage(t *testing.T) {
	f := filepath.Join(t.TempDir(), "caddy.env")
	os.WriteFile(f, []byte("# c\nCADDY_VERSION=2.11.7\n"), 0o644)
	p, err := caddyPackage(f)
	if err != nil || p.version != "2.11.7" || !strings.Contains(p.text, "Apache License") {
		t.Errorf("%+v %v", p, err)
	}
}
