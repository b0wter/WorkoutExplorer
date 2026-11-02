package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/precompress"
)

func brotli(t *testing.T, b []byte) []byte {
	t.Helper()
	out, err := precompress.Brotli(b)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func write(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptsBrotli(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"gzip, deflate, br", true},
		{"br", true},
		{"BR", true},
		{"gzip;q=1.0, br;q=0", false},
		{"br;q=0.000", false},
		{"br;q=0.5", true},
		{"br; q=1", true},
		{"br;Q=0", false},
		{"brotli", false},
		{"*", false},
		{"gzip, deflate, br, zstd", true},
		{"br;q=abc", false},
	}
	for _, tt := range tests {
		if got := acceptsBrotli(tt.in); got != tt.want {
			t.Errorf("acceptsBrotli(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		name     string
		ctype    string
		compress bool
	}{
		{"a.html", "text/html; charset=utf-8", true},
		{"a.js", "text/javascript; charset=utf-8", true},
		{"a.mjs", "text/javascript; charset=utf-8", true},
		{"a.css", "text/css; charset=utf-8", true},
		{"a.json", "application/json", true},
		{"a.svg", "image/svg+xml", true},
		{"a.txt", "text/plain; charset=utf-8", true},
		{"a.bin", "application/octet-stream", true},
		{"a.png", "image/png", false},
		{"a.woff2", "font/woff2", false},
		{"a.JS", "text/javascript; charset=utf-8", true},
		{"a.xyz", "application/octet-stream", false},
	}
	for _, tt := range tests {
		c, comp := contentType(tt.name)
		if c != tt.ctype || comp != tt.compress {
			t.Errorf("contentType(%q) = %q, %v; want %q, %v", tt.name, c, comp, tt.ctype, tt.compress)
		}
	}
}

func fixture(t *testing.T) (web, data string) {
	t.Helper()
	root := t.TempDir()
	web, data = filepath.Join(root, "web"), filepath.Join(root, "data")
	write(t, filepath.Join(web, "index.html"), "<!doctype html><title>x</title>"+strings.Repeat("a", 200))
	write(t, filepath.Join(web, "assets/app-abc.js"), strings.Repeat("let a = 1;\n", 100))
	write(t, filepath.Join(web, "assets/style.css"), "body{margin:0}")
	write(t, filepath.Join(web, "assets/img.png"), "\x89PNG not really")
	write(t, filepath.Join(web, ".vite/manifest.json"), "{}")
	write(t, filepath.Join(web, "index.html.br"), "junk")
	write(t, filepath.Join(data, "ride.bin"), strings.Repeat("\x00\x01\x02", 500))
	write(t, filepath.Join(data, "ride.bin.br"), "junk")
	return web, data
}

func TestLoad(t *testing.T) {
	web, data := fixture(t)
	files, err := load(web, data)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"/assets/app-abc.js", "/assets/img.png", "/assets/style.css", "/data/ride.bin", "/index.html"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for k, f := range files {
		if k == "/assets/img.png" {
			if f.br != nil {
				t.Errorf("%s: br should be nil", k)
			}
			continue
		}
		if !bytes.Equal(f.br, brotli(t, f.raw)) {
			t.Errorf("%s: br differs from precompress.Brotli(raw)", k)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	t.Run("no index", func(t *testing.T) {
		root := t.TempDir()
		_, err := load(filepath.Join(root, "web"), root)
		if err == nil || !strings.Contains(err.Error(), "index.html") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing data dir", func(t *testing.T) {
		web, data := fixture(t)
		_ = data
		if _, err := load(web, filepath.Join(web, "nope")); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		web, data := fixture(t)
		write(t, filepath.Join(web, "data/ride.bin"), "x")
		_, err := load(web, data)
		if err == nil || !strings.Contains(err.Error(), "duplicate path /data/ride.bin") {
			t.Fatalf("err = %v", err)
		}
	})
}

func testHandler(t *testing.T) (http.Handler, map[string]file, *bytes.Buffer) {
	t.Helper()
	web, data := fixture(t)
	files, err := load(web, data)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	return newHandler(files, &log), files, &log
}

func do(h http.Handler, method, target, ae string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if ae != "" {
		req.Header.Set("Accept-Encoding", ae)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler(t *testing.T) {
	h, files, _ := testHandler(t)
	idx := files["/index.html"]
	const acc = "gzip, deflate, br"

	checkIndexBr := func(t *testing.T, rec *httptest.ResponseRecorder, body bool) {
		t.Helper()
		if rec.Code != 200 {
			t.Fatalf("status %d", rec.Code)
		}
		hd := rec.Header()
		for k, v := range map[string]string{
			"Content-Encoding":       "br",
			"Content-Type":           "text/html; charset=utf-8",
			"Content-Length":         strconv.Itoa(len(idx.br)),
			"Cache-Control":          "no-store",
			"Referrer-Policy":        "no-referrer",
			"X-Content-Type-Options": "nosniff",
			"Vary":                   "Accept-Encoding",
		} {
			if hd.Get(k) != v {
				t.Errorf("%s = %q, want %q", k, hd.Get(k), v)
			}
		}
		if body && !bytes.Equal(rec.Body.Bytes(), idx.br) {
			t.Error("body is not the brotli index")
		}
		if !body && rec.Body.Len() != 0 {
			t.Error("HEAD has a body")
		}
	}

	t.Run("root br", func(t *testing.T) { checkIndexBr(t, do(h, "GET", "/index.html", acc), true) })
	t.Run("raw", func(t *testing.T) {
		rec := do(h, "GET", "/index.html", "")
		if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), idx.raw) || rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "Accept-Encoding" {
			t.Errorf("raw index wrong: %d %v", rec.Code, rec.Header())
		}
	})
	t.Run("query ignored", func(t *testing.T) { checkIndexBr(t, do(h, "GET", "/index.html?measure", acc), true) })
	t.Run("data", func(t *testing.T) {
		rec := do(h, "GET", "/data/ride.bin", acc)
		if rec.Header().Get("Content-Type") != "application/octet-stream" || !bytes.Equal(rec.Body.Bytes(), files["/data/ride.bin"].br) {
			t.Errorf("data wrong: %v", rec.Header())
		}
	})
	t.Run("png", func(t *testing.T) {
		rec := do(h, "GET", "/assets/img.png", acc)
		if !bytes.Equal(rec.Body.Bytes(), files["/assets/img.png"].raw) || rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "" {
			t.Errorf("png wrong: %v", rec.Header())
		}
	})
	t.Run("head", func(t *testing.T) { checkIndexBr(t, do(h, "HEAD", "/index.html", acc), false) })
	t.Run("404", func(t *testing.T) {
		for _, p := range []string{"/missing.js", "/data/missing.bin", "/assets/", "/assets", "/.vite/manifest.json", "/index.html.br", "/data/ride.bin.br", "//index.html", "/../index.html"} {
			rec := do(h, "GET", p, acc)
			if rec.Code != 404 || rec.Body.String() != "not found\n" {
				t.Errorf("%s: %d %q", p, rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s: headers %v", p, rec.Header())
			}
		}
	})
	t.Run("405", func(t *testing.T) {
		rec := do(h, "POST", "/index.html", "")
		if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("POST: %d %v", rec.Code, rec.Header())
		}
		if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" || rec.Header().Get("Cache-Control") != "no-store" || rec.Body.String() != "method not allowed\n" {
			t.Errorf("POST: headers %v body %q", rec.Header(), rec.Body.String())
		}
	})
}

func TestHandlerLog(t *testing.T) {
	h, files, log := testHandler(t)
	do(h, "GET", "/index.html", "br")
	do(h, "GET", "/missing.js", "br")
	do(h, "POST", "/index.html", "")
	do(h, "HEAD", "/index.html", "br")
	do(h, "HEAD", "/nope", "")
	want := "GET /index.html 200 br " + strconv.Itoa(len(files["/index.html"].br)) + " ae=br\n" +
		"GET /missing.js 404 - 10 ae=br\n" +
		"POST /index.html 405 - 19 ae=no-br\n" +
		"HEAD /index.html 200 br 0 ae=br\n" +
		"HEAD /nope 404 - 0 ae=no-br\n"
	if log.String() != want {
		t.Errorf("log =\n%s\nwant\n%s", log.String(), want)
	}
}

func TestSelfSigned(t *testing.T) {
	now := time.Now()
	extra := net.ParseIP("192.168.1.20")
	cert, err := selfSigned(now, []net.IP{extra})
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(c.DNSNames) != 1 || c.DNSNames[0] != "localhost" {
		t.Errorf("DNS names %v", c.DNSNames)
	}
	for _, ip := range []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1"), extra} {
		found := false
		for _, have := range c.IPAddresses {
			found = found || have.Equal(ip)
		}
		if !found {
			t.Errorf("missing IP %s", ip)
		}
	}
	if len(c.ExtKeyUsage) != 1 || c.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Errorf("ext key usage %v", c.ExtKeyUsage)
	}
	if c.NotAfter.Sub(c.NotBefore) != 7*24*time.Hour+time.Hour {
		t.Errorf("validity %v", c.NotAfter.Sub(c.NotBefore))
	}
	if c.IsCA {
		t.Error("is CA")
	}

	h, _, _ := testHandler(t)
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	defer srv.Close()
	pool := x509.NewCertPool()
	pool.AddCert(c)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	resp, err := client.Get(srv.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestRunArgs(t *testing.T) {
	web, data := fixture(t)
	tests := []struct {
		name string
		args []string
		code int
		pfx  string
	}{
		{"none", nil, 2, "usage:"},
		{"no samples", []string{"-web", web, "-data", data}, 2, "usage:"},
		{"web only", []string{"-web", "x"}, 2, "usage:"},
		{"empty addr", []string{"-web", web, "-data", data, "-samples", data, "-addr", ""}, 2, "usage:"},
		{"positional", []string{"-web", web, "-data", data, "-samples", data, "extra"}, 2, "usage:"},
		{"help", []string{"-h"}, 0, "usage:"},
		{"missing web", []string{"-web", filepath.Join(t.TempDir(), "nope"), "-data", data, "-samples", data}, 1, "protoserve: "},
	}
	for _, tt := range tests {
		var out, errb bytes.Buffer
		if got := run(tt.args, &out, &errb); got != tt.code {
			t.Errorf("%s: code %d, want %d", tt.name, got, tt.code)
		}
		if !strings.HasPrefix(errb.String(), tt.pfx) {
			t.Errorf("%s: stderr %q", tt.name, errb.String())
		}
	}
}
