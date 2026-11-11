package main

import (
	"bytes"
	"encoding/json"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Sample pages, F-13 design B-1: the source template executed with every
// committed sample view model. Covers AC 1, 3, 4, 5, 8 and 10.

const (
	templatePath = "../../../web/index.html"
	samplesGlob  = "../../testdata/page/*.json"
)

var noticeRE = regexp.MustCompile(`(?s)<p class="notice">.*?</p>`)

func TestSamplePages(t *testing.T) {
	tmpl, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(samplesGlob)
	if err != nil || len(paths) < 9 {
		t.Fatalf("samples: %v, %d files", err, len(paths))
	}
	notice := ""
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			out, err := renderPage(string(tmpl), raw)
			if err != nil {
				t.Fatal(err)
			}
			page := string(out)
			text := html.UnescapeString(page) // presence of display text, escaping aside
			vm := decode(t, raw)
			has := func(what, s string) {
				t.Helper()
				if s != "" && !strings.Contains(text, s) {
					t.Errorf("%s %q missing", what, s)
				}
			}

			// AC 1: header, summary, notice, laps.
			hd := vm["Header"].(map[string]any)
			for _, k := range []string{"Title", "SportName", "Start", "Description", "ExpiresText"} {
				has(k, hd[k].(string))
			}
			if hd["Description"] == "" && strings.Contains(page, `class="desc"`) {
				t.Error("empty description rendered")
			}
			if !strings.Contains(page, `datetime="`+hd["ExpiresAt"].(string)+`"`) {
				t.Error("expires datetime missing")
			}
			if !strings.Contains(text, "<title>"+vm["Page"].(map[string]any)["Title"].(string)+"</title>") {
				t.Error("document title missing")
			}
			keys := map[string]bool{}
			for _, c := range vm["Summary"].([]any) {
				m := c.(map[string]any)
				keys[m["Key"].(string)] = true
				has("summary label", m["Label"].(string))
				has("summary value", m["Value"].(string))
				has("summary unit", m["Unit"].(string))
				if m["Max"] != "" {
					has("summary max", "max "+m["Max"].(string))
				}
			}
			if laps, ok := vm["Laps"].(map[string]any); ok {
				for _, c := range laps["Columns"].([]any) {
					has("lap column", c.(map[string]any)["Label"].(string))
				}
				for _, row := range laps["Rows"].([]any) {
					for _, cell := range row.([]any) {
						has("lap cell", cell.(string))
					}
				}
			} else if strings.Contains(page, `class="laps"`) {
				t.Error("laps module without laps")
			}

			// AC 3: no power for the run, no heart rate for ride-no-hr.
			if name == "run-30min" && (keys["power"] || strings.Contains(page, "power")) {
				t.Error("power shown for the run")
			}
			if name == "ride-no-hr" && (keys["heart_rate"] || strings.Contains(strings.ToLower(page), "heart")) {
				t.Error("heart rate shown for ride-no-hr")
			}

			// AC 4: the same notice on every page.
			n := noticeRE.FindString(page)
			if n == "" {
				t.Fatal("notice missing")
			}
			if notice == "" {
				notice = n
			} else if n != notice {
				t.Errorf("notice differs: %q vs %q", n, notice)
			}

			// AC 8: map slot or hint, never both.
			route := vm["Route"].(map[string]any)
			hint := strings.Contains(page, `class="hint"`)
			slot := strings.Contains(page, `id="map-slot"`)
			if route["Hidden"] != "" {
				if !strings.Contains(page, `<p class="hint">The entire route has been hidden by the uploader.</p>`) || slot || route["NoLocation"] != "" || route["Map"] != "" {
					t.Errorf("fully hidden: notice or slot wrong (map slot %v)", slot)
				}
			} else if strings.Contains(page, "hidden by the uploader") {
				t.Error("hidden notice without Route.Hidden")
			}
			if route["NoLocation"] != "" && (!hint || slot) {
				t.Errorf("no location: hint %v, map slot %v", hint, slot)
			}
			if route["Map"] != "" && (hint || !slot) {
				t.Errorf("route: hint %v, map slot %v", hint, slot)
			}

			// F-13 D-17, D-19: the map-case class only with a map; the pin toggle is bundle-only.
			if hasMap := strings.Contains(page, `class="page has-map"`); hasMap != (route["Map"] != "") {
				t.Errorf("has-map class %v, Route.Map %q", hasMap, route["Map"])
			}
			// The two buttons: How to use (F-39 HU-6), in its own row right under the header, and Report.
			if strings.Contains(page, "map-pin") {
				t.Error("pin toggle in the server markup")
			}
			if n := strings.Count(page, "<button"); n != 2 || !regexp.MustCompile(`</header>\s*<div class="howto-row"><button type="button" class="howto" data-howto>`).MatchString(page) {
				t.Errorf("server markup: %d buttons, want How to use (own row under the header) and Report", n)
			}

			// AC 10: About and Report in the footer; Report is a button the bundle unhides (F-28 RF-1, RF-9).
			foot := page[strings.Index(page, `<footer`):]
			if !strings.Contains(foot, `href="/about">About</a>`) || !strings.Contains(foot, `<button type="button" data-report hidden>Report</button>`) {
				t.Error("footer lacks About or Report")
			}

			// AC 5: user text is escaped; the module script and DS-4's inline theme script are the only scripts.
			if c := strings.Count(page, "<script"); c != 2 || strings.Count(page, `<script type="module"`) != 1 || !strings.Contains(page, `<script>try{var s=localStorage,`) {
				t.Errorf("%d script elements, or not the module and theme scripts", c)
			}
			if name == "hostile-text" {
				for _, bad := range []string{"<b>Bold", "<script>alert", "<img", "onerror=alert(3)>", "<i>"} {
					if strings.Contains(page, bad) {
						t.Errorf("unescaped %q", bad)
					}
				}
				for _, good := range []string{"&lt;b&gt;Bold&lt;/b&gt; &amp; &#34;quoted&#34;", "&lt;script&gt;alert(2)&lt;/script&gt;", "&lt;img src=x onerror=alert(3)&gt; &amp;amp; done", `<p class="device">Recorded with &lt;i&gt;Dev&lt;/i&gt; &amp;amp; &#34;x&#34;</p>`} {
					if !strings.Contains(page, good) {
						t.Errorf("escaped form %q missing", good)
					}
				}
			}

			// AC 1 budget: the filled 12-hour sample, gzip -9.
			if name == "ride-12h" {
				if g := gzipSize(out); g > 15000 {
					t.Errorf("gzip -9 size %d > 15000", g)
				} else {
					t.Logf("ride-12h html: raw %d, gzip-9 %d", len(out), g)
				}
			}
		})
	}
}

func TestMissingKeyIsError(t *testing.T) {
	if _, err := renderPage("{{.Nope}}", []byte(`{}`)); err == nil {
		t.Error("missing key rendered")
	}
}

func TestAddSamples(t *testing.T) {
	web, data := fixture(t)
	root := t.TempDir()
	write(t, filepath.Join(web, "index.html"), "<p>{{.Page.Title}}</p>")
	write(t, filepath.Join(data, "ride-30min.bin"), "bin")
	write(t, filepath.Join(root, "ride-no-hr.json"), `{"Page":{"Title":"T","DataURL":"/w/ride-no-hr/data.ab.bin"}}`)
	write(t, filepath.Join(root, "other.json"), `{"Page":{"Title":"U","DataURL":"/w/other/data.cd.bin"}}`)
	files, err := load(web, data)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if err := addSamples(files, web, root, &log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "other.bin missing") {
		t.Errorf("log %q", log.String())
	}
	h := newHandler(files, &bytes.Buffer{})
	for path, want := range map[string]int{
		"/": 404, "/index.html": 404, "/w/ride-no-hr": 200, "/w/other": 200,
		"/w/ride-no-hr/data.ab.bin": 200, "/w/other/data.cd.bin": 404,
	} {
		if got := do(h, http.MethodGet, path, "br").Code; got != want {
			t.Errorf("GET %s = %d, want %d", path, got, want)
		}
	}
	if b := do(h, http.MethodGet, "/w/ride-no-hr", "").Body.String(); b != "<p>T</p>" {
		t.Errorf("body %q", b)
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
