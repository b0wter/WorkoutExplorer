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

func TestCheckCSPAllLinesAndSchemes(t *testing.T) {
	srcs, _ := loadSources()
	f := filepath.Join(t.TempDir(), "Caddyfile")
	for name, body := range map[string]string{
		"second line":   "Content-Security-Policy \"default-src 'self'\"\nContent-Security-Policy \"img-src 'self' https://evil.example\"",
		"scheme-less":   `Content-Security-Policy "img-src 'self' cdn.example.org"`,
		"wss":           `Content-Security-Policy "connect-src 'self' wss://sock.example.org"`,
		"wss unclaimed": `Content-Security-Policy "connect-src wss://tiles.openfreemap.org.evil.net"`,
	} {
		os.WriteFile(f, []byte(body), 0o644)
		if checkCSP(f, srcs) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	os.WriteFile(f, []byte(`Content-Security-Policy "connect-src 'self' wss://tiles.openfreemap.org; img-src data: blob: 'sha256-x' tiles.openfreemap.org"`), 0o644)
	if err := checkCSP(f, srcs); err != nil {
		t.Error(err)
	}
}

func TestModuleText(t *testing.T) {
	cache := t.TempDir()
	os.MkdirAll(filepath.Join(cache, "example.com", "!no@v1.0.0"), 0o755)
	os.MkdirAll(filepath.Join(cache, "example.com", "ok@v1.0.0"), 0o755)
	os.WriteFile(filepath.Join(cache, "example.com", "ok@v1.0.0", "LICENSE"), []byte("MIT text"), 0o644)
	os.WriteFile(filepath.Join(cache, "example.com", "ok@v1.0.0", "main.go"), []byte("x"), 0o644)
	if _, err := moduleText(cache, "example.com/No", "v1.0.0"); err == nil || !strings.Contains(err.Error(), "no licence file") {
		t.Errorf("module without licence: %v", err)
	}
	if _, err := moduleText(cache, "example.com/gone", "v1.0.0"); err == nil {
		t.Error("module missing from the cache accepted")
	}
	if got, err := moduleText(cache, "example.com/ok", "v1.0.0"); err != nil || !strings.Contains(got, "MIT text") || strings.Contains(got, "main.go") {
		t.Errorf("%q %v", got, err)
	}
}

func TestParseSourcesRejects(t *testing.T) {
	for name, body := range map[string]string{
		"no licence": `[{"name":"n","attribution":"a","licence":"","link":"https://x"}]`,
		"http link":  `[{"name":"n","attribution":"a","licence":"L","link":"http://x"}]`,
		"no link":    `[{"name":"n","attribution":"a","licence":"L"}]`,
		"bad json":   `[`,
	} {
		if _, err := parseSources([]byte(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRunFailures(t *testing.T) {
	web := t.TempDir()
	page := filepath.Join(web, "about.html")
	caddyfile := filepath.Join(t.TempDir(), "Caddyfile")
	os.WriteFile(caddyfile, []byte(`Content-Security-Policy "default-src 'self'"`), 0o644)
	os.WriteFile(page, []byte("<html>no marker</html>"), 0o644)
	if err := run(web, "no-binary", "no-env", caddyfile); err == nil || !strings.Contains(err.Error(), markerList) {
		t.Errorf("template without marker: %v", err)
	}
	os.WriteFile(page, []byte("<html>"+markerList+"</html>"), 0o644)
	if err := run(web, "no-binary", "no-env", caddyfile); err == nil || !strings.Contains(err.Error(), "license.json") {
		t.Errorf("missing license.json: %v", err)
	}
}
