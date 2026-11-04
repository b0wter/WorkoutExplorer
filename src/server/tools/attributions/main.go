// Command attributions writes the attribution list into the About page
// (F-29 AB-5, AB-6). It is a development and build tool, not part of the
// workout-explorer binary, and uses the standard library only.
//
//	go run ./tools/attributions -web ../../build/web -binary ../../build/workout-explorer \
//	    -caddy-env ../../deploy/caddy.env -caddyfile ../../deploy/Caddyfile
//
// It replaces the marker <!--wx:attributions--> in <web>/about.html with
// <!--wx:generated--> and the list. Inputs: Vite's .vite/license.json, the
// binary's build info (module texts come from the module cache), the Go
// toolchain's LICENSE, the pinned Caddy version, and the committed sources.json.
// Any input it cannot read or complete is an error (exit 1), so the build fails.
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
}

type pkg struct{ name, version, id, text string }

func main() {
	web := flag.String("web", "", "web build directory (required)")
	bin := flag.String("binary", "", "built workout-explorer binary (required)")
	caddyEnv := flag.String("caddy-env", "", "deploy/caddy.env (required)")
	caddyfile := flag.String("caddyfile", "", "deploy/Caddyfile (required)")
	flag.Parse()
	if *web == "" || *bin == "" || *caddyEnv == "" || *caddyfile == "" {
		fmt.Fprintln(os.Stderr, "attributions: -web, -binary, -caddy-env and -caddyfile are required")
		os.Exit(2)
	}
	if err := run(*web, *bin, *caddyEnv, *caddyfile); err != nil {
		fmt.Fprintln(os.Stderr, "attributions:", err)
		os.Exit(1)
	}
}

func run(web, bin, caddyEnv, caddyfile string) error {
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
	caddy, err := caddyPackage(caddyEnv)
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
var originRE = regexp.MustCompile(`https?://[^/\s;'"]+`)

// checkCSP: every external origin the Caddyfile's CSP allows is claimed by a sources entry (AB-6).
func checkCSP(caddyfile string, srcs []source) error {
	b, err := os.ReadFile(caddyfile)
	if err != nil {
		return err
	}
	m := cspLine.FindSubmatch(b)
	if m == nil {
		return fmt.Errorf("%s: no Content-Security-Policy line found", caddyfile)
	}
	claimed := map[string]bool{}
	for _, s := range srcs {
		for _, o := range s.Origins {
			claimed[o] = true
		}
	}
	for _, o := range originRE.FindAllString(string(m[1]), -1) {
		if !claimed[o] {
			return fmt.Errorf("the CSP in %s allows %s but no entry in sources.json claims it (add attribution and licence)", caddyfile, o)
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
		dir := filepath.Join(cache, escapePath(d.Path)+"@"+d.Version)
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("go module %s@%s: %w", d.Path, d.Version, err)
		}
		var text []string
		for _, e := range ents {
			if e.Type().IsRegular() && licenceFile.MatchString(e.Name()) {
				b, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					return nil, err
				}
				text = append(text, "--- "+e.Name()+" ---\n"+string(b))
			}
		}
		if len(text) == 0 {
			if o := override("go", d.Path, d.Version); o != "" {
				text = append(text, o)
			}
		}
		if len(text) == 0 {
			return nil, fmt.Errorf("go module %s@%s has no licence file", d.Path, d.Version)
		}
		out = append(out, pkg{d.Path, d.Version, "", strings.Join(text, "\n")})
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

var caddyVersion = regexp.MustCompile(`(?m)^CADDY_VERSION=([0-9][0-9A-Za-z.+-]*)\s*$`)

func caddyPackage(envFile string) (pkg, error) {
	b, err := os.ReadFile(envFile)
	if err != nil {
		return pkg{}, err
	}
	m := caddyVersion.FindSubmatch(b)
	if m == nil {
		return pkg{}, errors.New(envFile + ": no CADDY_VERSION")
	}
	t, _ := embedded.ReadFile("caddy-LICENSE.txt")
	return pkg{"Caddy (web server, run next to this app)", string(m[1]), "Apache-2.0", string(t)}, nil
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
	sb.WriteString("<h3>Map data, styles and fonts</h3>\n<ul>\n")
	for _, s := range srcs {
		sb.WriteString("<li>" + esc(s.Name) + ": " + esc(s.Attribution) + ". Licence: <a href=\"" + esc(s.Link) + "\">" + esc(s.Licence) + "</a></li>\n")
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
