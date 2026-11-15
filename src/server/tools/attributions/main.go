// Command attributions writes the attribution list into the About page
// (F-29 AB-5, AB-6). It is a development and build tool, not part of the
// workout-explorer binary, and uses the standard library only.
//
//	go run ./tools/attributions -web ../../build/web -binary ../../build/workout-explorer \
//	    -compose ../../deploy/compose.yaml -caddyfile ../../deploy/Caddyfile
//	go run ./tools/attributions -base-packages <file>
//
// It replaces the marker <!--wx:attributions--> in <web>/about.html with
// <!--wx:generated--> and the list. Inputs: Vite's .vite/license.json, the
// binary's build info (module texts come from the module cache), the Go
// toolchain's LICENSE, the pinned Caddy minor (compose.yaml's Caddy tag), and
// the committed sources.json. Any input it cannot read or complete is an
// error (exit 1), so the build fails. With -base-packages it only checks that
// the file (one package name per line, from the app image) lists exactly the
// packages of the sources entry for the image's base (F-59 IP-2).
package main

import (
	"bytes"
	"debug/buildinfo"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed sources.json caddy-LICENSE.txt overrides
var embedded embed.FS

const (
	markerList = "<!--wx:attributions-->" // about.MarkerList
	markerDone = "<!--wx:generated-->"    // about.MarkerDone
	// repoURL is this project's source (AGPL section 13); a fork edits it.
	repoURL = "https://github.com/b0wter/WorkoutExplorer"
)

type source struct {
	Name, Attribution, Licence, Link string
	Origins                          []string
	Packages                         map[string]string // base image only: Debian package -> licence (F-59 IP-2)
}

type pkg struct{ name, version, id, text string }

func main() {
	web := flag.String("web", "", "web build directory (required)")
	bin := flag.String("binary", "", "built workout-explorer binary (required)")
	compose := flag.String("compose", "", "deploy/compose.yaml, for the Caddy minor (required)")
	caddyfile := flag.String("caddyfile", "", "deploy/Caddyfile (required)")
	basePkgs := flag.String("base-packages", "", "check this package list of the app image against sources.json, nothing else")
	flag.Parse()
	if *basePkgs != "" {
		if err := checkBase(*basePkgs); err != nil {
			fmt.Fprintln(os.Stderr, "attributions:", err)
			os.Exit(1)
		}
		return
	}
	if *web == "" || *bin == "" || *compose == "" || *caddyfile == "" {
		fmt.Fprintln(os.Stderr, "attributions: -web, -binary, -compose and -caddyfile are required")
		os.Exit(2)
	}
	if err := run(*web, *bin, *compose, *caddyfile); err != nil {
		fmt.Fprintln(os.Stderr, "attributions:", err)
		os.Exit(1)
	}
}

func run(web, bin, compose, caddyfile string) error {
	page := filepath.Join(web, "about.html")
	tmpl, err := os.ReadFile(page)
	if err != nil {
		return err
	}
	if bytes.Count(tmpl, []byte(markerList)) != 1 {
		return fmt.Errorf("%s needs exactly one %s (rebuild the web bundle first)", page, markerList)
	}
	srcs, err := loadSources()
	if err != nil {
		return err
	}
	if err := checkCSP(caddyfile, srcs); err != nil {
		return err
	}
	npm, err := npmPackages(filepath.Join(web, ".vite", "license.json"))
	if err != nil {
		return err
	}
	golang, err := goPackages(bin)
	if err != nil {
		return err
	}
	caddy, err := caddyPackage(compose)
	if err != nil {
		return err
	}
	golang = append(golang, caddy)
	sort.Slice(golang, func(i, j int) bool { return golang[i].name < golang[j].name })
	out := bytes.Replace(tmpl, []byte(markerList), []byte(markerDone+"\n"+render(srcs, npm, golang)), 1)
	tmp := page + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, page)
}

func loadSources() ([]source, error) {
	b, _ := embedded.ReadFile("sources.json")
	return parseSources(b)
}

func parseSources(b []byte) ([]source, error) {
	var s []source
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("sources.json: %w", err)
	}
	for _, e := range s {
		if e.Name == "" || e.Attribution == "" || e.Licence == "" || !strings.HasPrefix(e.Link, "https://") {
			return nil, fmt.Errorf("sources.json: entry %q needs a name, attribution, licence name and an https link", e.Name)
		}
	}
	return s, nil
}

var cspLine = regexp.MustCompile(`Content-Security-Policy\s+"([^"]*)"`)

// hostOf drops a scheme, so https://h, wss://h and a bare h name the same origin.
func hostOf(s string) string {
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	return strings.TrimSuffix(s, "/")
}

// checkCSP: every external source in every CSP line of the Caddyfile is claimed by a sources entry (AB-6).
// Keywords, data:/blob: and hashes are not origins; anything else is, whatever its scheme.
func checkCSP(caddyfile string, srcs []source) error {
	b, err := os.ReadFile(caddyfile)
	if err != nil {
		return err
	}
	lines := cspLine.FindAllSubmatch(b, -1)
	if lines == nil {
		return fmt.Errorf("%s: no Content-Security-Policy line found", caddyfile)
	}
	claimed := map[string]bool{}
	for _, s := range srcs {
		for _, o := range s.Origins {
			claimed[hostOf(o)] = true
		}
	}
	for _, m := range lines {
		for _, dir := range strings.Split(string(m[1]), ";") {
			f := strings.Fields(dir)
			for _, tok := range f[min(1, len(f)):] { // f[0] is the directive name
				if strings.HasPrefix(tok, "'") || tok == "data:" || tok == "blob:" || claimed[hostOf(tok)] {
					continue
				}
				return fmt.Errorf("the CSP in %s allows %s but no entry in sources.json claims it (add attribution and licence)", caddyfile, tok)
			}
		}
	}
	return nil
}

func override(kind, name, version string) string {
	b, err := embedded.ReadFile("overrides/" + kind + "_" + strings.ReplaceAll(name, "/", "__") + "@" + version + ".txt")
	if err != nil {
		return ""
	}
	return string(b)
}

func npmPackages(path string) ([]pkg, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w (is build.license set in vite.config.ts?)", err)
	}
	var raw []struct{ Name, Version, Identifier, Text string }
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s lists no package", path)
	}
	var out []pkg
	for _, r := range raw {
		if r.Text == "" {
			r.Text = override("npm", r.Name, r.Version)
		}
		if r.Name == "" || r.Identifier == "" || strings.TrimSpace(r.Text) == "" {
			return nil, fmt.Errorf("npm package %s@%s: licence identifier or text missing", r.Name, r.Version)
		}
		out = append(out, pkg{r.Name, r.Version, r.Identifier, r.Text})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

var licenceFile = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice)`)

// escapePath is the module cache's case encoding: upper case becomes "!" and lower case.
func escapePath(p string) string {
	var sb strings.Builder
	for _, r := range p {
		if r >= 'A' && r <= 'Z' {
			sb.WriteString("!" + strings.ToLower(string(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func goEnv(name string) (string, error) {
	b, err := exec.Command("go", "env", name).Output()
	return strings.TrimSpace(string(b)), err
}

// moduleText is the module root's licence files, or the committed override; none is an error.
func moduleText(cache, path, version string) (string, error) {
	dir := filepath.Join(cache, escapePath(path)+"@"+version)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("go module %s@%s: %w", path, version, err)
	}
	var text []string
	for _, e := range ents {
		if e.Type().IsRegular() && licenceFile.MatchString(e.Name()) {
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return "", err
			}
			text = append(text, "--- "+e.Name()+" ---\n"+string(b))
		}
	}
	if len(text) == 0 {
		if o := override("go", path, version); o != "" {
			text = append(text, o)
		}
	}
	if len(text) == 0 {
		return "", fmt.Errorf("go module %s@%s has no licence file", path, version)
	}
	return strings.Join(text, "\n"), nil
}

func goPackages(bin string) ([]pkg, error) {
	info, err := buildinfo.ReadFile(bin)
	if err != nil {
		return nil, err
	}
	cache, err := goEnv("GOMODCACHE")
	if err != nil || cache == "" {
		return nil, fmt.Errorf("go env GOMODCACHE: %v", err)
	}
	var out []pkg
	for _, d := range info.Deps {
		if d.Replace != nil {
			d = d.Replace
		}
		text, err := moduleText(cache, d.Path, d.Version)
		if err != nil {
			return nil, err
		}
		out = append(out, pkg{d.Path, d.Version, "", text})
	}
	root, err := goEnv("GOROOT")
	if err != nil || root == "" {
		return nil, fmt.Errorf("go env GOROOT: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil { // distro toolchains (Fedora) keep it outside GOROOT
		b, err = os.ReadFile("/usr/share/licenses/golang/LICENSE")
	}
	if err != nil {
		return nil, fmt.Errorf("go standard library licence: %w", err)
	}
	return append(out, pkg{"Go standard library and runtime", strings.TrimPrefix(info.GoVersion, "go"), "BSD-3-Clause", string(b)}), nil
}

// caddyImage is the one home of the Caddy minor (F-59 IP-6): compose.yaml's tag of the official image.
var caddyImage = regexp.MustCompile(`(?m)^\s*image:\s*docker\.io/library/caddy:([0-9]+\.[0-9]+)\s*$`)

func caddyPackage(compose string) (pkg, error) {
	b, err := os.ReadFile(compose)
	if err != nil {
		return pkg{}, err
	}
	m := caddyImage.FindSubmatch(b)
	if m == nil {
		return pkg{}, errors.New(compose + ": no docker.io/library/caddy:<minor> image")
	}
	t, _ := embedded.ReadFile("caddy-LICENSE.txt")
	return pkg{"Caddy (web server, run next to this app)", string(m[1]) + ".x", "Apache-2.0", string(t)}, nil
}

// checkBase compares the app image's package list (file) with the one sources
// entry that has packages, so a base bump cannot silently change what the
// published image redistributes (F-59 IP-2).
func checkBase(file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	srcs, err := loadSources()
	if err != nil {
		return err
	}
	var want []string
	for _, s := range srcs {
		for p := range s.Packages {
			want = append(want, p)
		}
	}
	var got []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			got = append(got, l)
		}
	}
	sort.Strings(want)
	sort.Strings(got)
	if len(got) == 0 || strings.Join(got, " ") != strings.Join(want, " ") {
		return fmt.Errorf("the app image's base packages %v differ from sources.json's %v: update the base image entry (names and licences)", got, want)
	}
	return nil
}

func esc(s string) string { return html.EscapeString(s) }

func entry(p pkg) string {
	title := p.name + " " + p.version
	if p.id != "" {
		title += " (" + p.id + ")"
	}
	return "<li><details><summary>" + esc(title) + "</summary><pre>" + esc(p.text) + "</pre></details></li>\n"
}

// render builds the fragment; the version marker is filled by serve.
func render(srcs []source, npm, golang []pkg) string {
	var sb strings.Builder
	sb.WriteString("<div class=\"attributions\">\n<h3>This app</h3>\n<ul>\n")
	sb.WriteString("<li>Workout Explorer <!--wx:version-->, licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). <a href=\"" + esc(repoURL) + "\">Source code</a></li>\n</ul>\n")
	sb.WriteString("<h3>Data, styles, fonts and the container image</h3>\n<ul>\n")
	for _, s := range srcs {
		sb.WriteString("<li>" + esc(s.Name) + ": " + esc(s.Attribution) + ". ")
		if len(s.Packages) > 0 {
			names := make([]string, 0, len(s.Packages))
			for p := range s.Packages {
				names = append(names, p)
			}
			sort.Strings(names)
			for i, p := range names {
				names[i] = p + " (" + s.Packages[p] + ")"
			}
			sb.WriteString("Packages: " + esc(strings.Join(names, ", ")) + ". ")
		}
		sb.WriteString("Licence: <a href=\"" + esc(s.Link) + "\">" + esc(s.Licence) + "</a></li>\n")
	}
	sb.WriteString("</ul>\n<h3>Libraries in the web page</h3>\n<ul>\n")
	for _, p := range npm {
		sb.WriteString(entry(p))
	}
	sb.WriteString("</ul>\n<h3>Libraries in the server</h3>\n<ul>\n")
	for _, p := range golang {
		sb.WriteString(entry(p))
	}
	sb.WriteString("</ul>\n</div>\n")
	return sb.String()
}
