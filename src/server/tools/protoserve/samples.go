package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"workout-explorer/internal/precompress"
)

// dataFor maps a sample to the data file (build/prototype/data/<name>.bin)
// that its data URL serves. Samples not listed use their own name. F-13 design,
// B-1: ride-no-hr reuses ride-30min's file, so its charts show that file's
// heart rate; the page markup (summary) is what the sample tests.
var dataFor = map[string]string{
	"ride-no-hr":          "ride-30min",
	"indoor-ride":         "indoor-no-gps",
	"route-hidden":        "indoor-no-gps",
	"ride-suggested":      "ride-30min",
	"indoor-unknown-zone": "indoor-no-distance-no-gps",
	"hostile-text":        "ride-30min",
}

// renderPage executes the template with one sample view model, decoded into a
// map (ADR-026 item 2; missing keys are errors).
func renderPage(tmpl string, sample []byte) ([]byte, error) {
	t, err := template.New("index.html").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var vm map[string]any
	if err := json.Unmarshal(sample, &vm); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vm); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipSize(b []byte) int {
	var buf bytes.Buffer
	// Cannot fail: the level is a valid constant and bytes.Buffer writes never error.
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(b)
	_ = zw.Close()
	return buf.Len()
}

// addSamples removes the raw template from files and adds, for every
// <samplesDir>/*.json, the filled page at /w/<name> and its data file at the
// sample's data URL (taken from /data/<mapped>.bin when that file exists).
// It writes a note to log for each data file that is missing.
func addSamples(files map[string]file, webDir, samplesDir string, log io.Writer) error {
	delete(files, "/index.html")
	tmpl, err := os.ReadFile(filepath.Join(webDir, "index.html"))
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(samplesDir, "*.json"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no sample view models in %s", samplesDir)
	}
	sort.Strings(paths)
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".json")
		sample, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		page, err := renderPage(string(tmpl), sample)
		if err != nil {
			return fmt.Errorf("sample %s: %w", name, err)
		}
		br, err := precompress.Brotli(page)
		if err != nil {
			return err
		}
		files["/w/"+name] = file{raw: page, br: br, contentType: "text/html; charset=utf-8", gz: gzipSize(page)}

		var vm struct{ Page struct{ DataURL string } }
		if err := json.Unmarshal(sample, &vm); err != nil {
			return fmt.Errorf("sample %s: %w", name, err)
		}
		url := vm.Page.DataURL
		if !strings.HasPrefix(url, "/w/"+name+"/") || path.Clean(url) != url {
			return fmt.Errorf("sample %s: data URL %q is not under /w/%s/", name, url, name)
		}
		src := name
		if m, ok := dataFor[name]; ok {
			src = m
		}
		if f, ok := files["/data/"+src+".bin"]; ok {
			files[url] = f
		} else {
			fmt.Fprintf(log, "note: %s: data file %s.bin missing (run make prototype-data); %s will 404\n", name, src, url)
		}
	}
	return nil
}
