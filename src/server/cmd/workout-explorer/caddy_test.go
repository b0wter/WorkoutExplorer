//go:build caddycheck

package main

// F-22 H-12: deploy/Caddyfile in a real Caddy (WX_CADDY, the pin of
// deploy/caddy.env) in front of the real server wiring (newApp, runServer on
// a TCP port) over a scratch data directory, synthetic files only. Run by
// `make caddy-check`; without the binary it fails (OQ-90).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/shell"
)

// Asserted exactly, so a loosened policy fails the build (H-2, H-12).
var securitySet = map[string]string{
	"Content-Security-Policy":   "default-src 'self'; script-src 'self' 'sha256-oU85smkdewIErbjHrk9QZ1FiICdjcwU0koHX7ul9Dpw=' 'sha256-z5SjeUWaAaA20n9xjEAw3BJBRbo/eoPKtoQHO3aO8bY='; worker-src 'self'; connect-src 'self' https://tiles.openfreemap.org; img-src 'self' data: blob: https://tiles.openfreemap.org; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
	"Referrer-Policy":           "no-referrer",
	"X-Content-Type-Options":    "nosniff",
	"Strict-Transport-Security": "max-age=31536000",
	"X-Robots-Tag":              "noindex, nofollow, noarchive", // OQ-89: every response
}

const (
	ccNoCache   = "no-cache"
	ccNoStore   = "no-store"
	ccWorkout   = "private, max-age=31536000, immutable"
	ccAsset     = "public, max-age=31536000, immutable"
	sh10Limit   = 60 * time.Second
	expiryEvery = 100 * time.Millisecond
)

type rig struct {
	t      *testing.T
	a      *app
	web    string
	site   string
	stopGo func()
	client *http.Client
	logs   *syncBuffer // Caddy's own output
}

type reply struct {
	code int
	h    http.Header
	body []byte
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func startRig(t *testing.T) *rig { return startRigWith(t, "", "") }

// startRigWith can point Caddy at another upstream (apiOverride, host:port)
// and sets WX_TRUSTED_PROXIES (F-31 DP-1b); empty: the real server, trust
// nobody. impressum, if given, is the rendered Impressum (F-38).
func startRigWith(t *testing.T, apiOverride, trusted string, impressum ...string) *rig {
	t.Helper()
	caddy, web := os.Getenv("WX_CADDY"), os.Getenv("WX_WEB_BUILD")
	if caddy == "" || web == "" {
		t.Fatal("WX_CADDY and WX_WEB_BUILD must be set; run `make caddy-check`")
	}
	if fi, err := os.Stat(caddy); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("no Caddy binary at %s (run `make caddy`): %v", caddy, err)
	}
	site := "http://" + freeAddr(t)
	a := startApp(t, settings{webDir: web, expiryEvery: expiryEvery, baseURL: site, impressumHTML: strings.Join(impressum, "")}) // F-23 PC-8
	api, stopGo, _, result := startServer(t, a.h)
	r := &rig{t: t, a: a, web: web, site: site,
		client: &http.Client{
			Timeout:       waitLimit,
			Transport:     &http.Transport{DisableCompression: true}, // Accept-Encoding only where a test sets it
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}}
	r.stopGo = func() { stopGo(); waitResult(t, result) }
	if apiOverride != "" {
		api = "http://" + apiOverride
	}

	home := t.TempDir()
	logs := &syncBuffer{}
	cmd := exec.Command(caddy, "run", "--config", filepath.Join("..", "..", "..", "..", "deploy", "Caddyfile"), "--adapter", "caddyfile")
	cmd.Env = []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "XDG_DATA_HOME=" + home,
		"WX_DATA=" + a.dir, "WX_WEB=" + web, "WX_API=" + strings.TrimPrefix(api, "http://"), "WX_SITE=" + r.site,
		"WX_TRUSTED_PROXIES=" + trusted}
	r.logs = logs
	cmd.Stdout, cmd.Stderr = logs, logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		if t.Failed() {
			t.Logf("caddy log:\n%s", logs.String())
		}
	})
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if resp, err := r.client.Get(r.site + "/"); err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(end) {
			t.Fatalf("caddy did not start:\n%s", logs.String())
		}
	}
	return r
}

// do sends one request through Caddy; hdr is name, value pairs.
func (r *rig) do(method, path string, body io.Reader, hdr ...string) reply {
	r.t.Helper()
	req, err := http.NewRequest(method, r.site+path, body)
	if err != nil {
		r.t.Fatal(err)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := r.client.Do(req)
	if err != nil {
		r.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		r.t.Fatal(err)
	}
	return reply{resp.StatusCode, resp.Header, b}
}

func (r *rig) get(path string, hdr ...string) reply { return r.do("GET", path, nil, hdr...) }

// check asserts the H-2 set, H-9 and the Cache-Control of one response.
func (r *rig) check(label string, got reply, code int, cc string) {
	r.t.Helper()
	if got.code != code {
		r.t.Errorf("%s: status %d, want %d", label, got.code, code)
	}
	for k, v := range securitySet {
		if vs := got.h.Values(k); len(vs) != 1 || vs[0] != v {
			r.t.Errorf("%s: %s = %q", label, k, vs)
		}
	}
	if vs := got.h.Values("Cache-Control"); len(vs) != 1 || vs[0] != cc {
		r.t.Errorf("%s: Cache-Control = %q, want %q", label, vs, cc)
	}
	if got.h.Values("Set-Cookie") != nil {
		r.t.Errorf("%s: Set-Cookie", label)
	}
}

// upload one synthetic file through Caddy.
func (r *rig) upload(name string) (pub, token string) {
	r.t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, _ := w.CreateFormFile("file", name)
	part.Write(readFile(r.t, name))
	w.Close()
	got := r.do("POST", "/api/workouts", &b, "Content-Type", w.FormDataContentType())
	var links map[string]string
	if got.code != http.StatusCreated || json.Unmarshal(got.body, &links) != nil {
		r.t.Fatalf("upload: %d %s", got.code, got.body)
	}
	r.check("upload", got, http.StatusCreated, ccNoStore)
	pub = strings.TrimPrefix(links["view_url"], "/w/")
	_, token, _ = strings.Cut(links["edit_url"], "#")
	return pub, token
}

// publish uploads and publishes one synthetic file through Caddy.
func (r *rig) publish(name string) (pub, token string) {
	r.t.Helper()
	pub, token = r.upload(name)
	if got := r.do("POST", "/api/workouts/"+pub+"/publish", nil, "Authorization", "Bearer "+token); got.code != 200 {
		r.t.Fatalf("publish: %d %s", got.code, got.body)
	}
	return pub, token
}

// siteFile is a page shell as serve wrote it (ADR-043); it must be the
// build's file with the footer marker filled.
func (r *rig) siteFile(name string) []byte {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.a.dir, "site", name))
	if err != nil {
		r.t.Fatal(err)
	}
	src := string(r.webFile(name))
	on := strings.Contains(string(b), shell.Link)
	if want, err := shell.Fill(name, src, on); err != nil || want != string(b) {
		r.t.Fatalf("site/%s is not the filled build file: %v", name, err)
	}
	return b
}

func (r *rig) webFile(name string) []byte {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.web, name))
	if err != nil {
		r.t.Fatal(err)
	}
	return b
}

// gone polls until the old page no longer serves page (generic 404 or a page
// without oldData) and the old data URL is 404, and fails after 60 s (SH-10).
func (r *rig) gone(label, pub, oldData string, since time.Time) {
	r.t.Helper()
	generic := r.siteFile("generic.html")
	for {
		p, d := r.get("/w/"+pub), r.get("/w/"+pub+"/"+oldData)
		pageGone := (p.code == 404 && bytes.Equal(p.body, generic)) || (p.code == 200 && !bytes.Contains(p.body, []byte(oldData)))
		if pageGone && d.code == 404 {
			r.check(label+": page", p, p.code, ccNoCache)
			r.check(label+": old data", d, 404, ccNoCache)
			return
		}
		if time.Since(since) > sh10Limit {
			r.t.Errorf("%s: still served %v after the trigger (page %d, data %d)", label, sh10Limit, p.code, d.code)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

var assetRef = regexp.MustCompile(`(?:/assets/|\./|"assets/)([0-9A-Za-z._-]+\.(?:js|css))`)

func TestCaddy(t *testing.T) {
	r := startRig(t)
	pub, token := r.publish("ride-30min.fit")
	id := idOf(r.a, pub)
	auth := []string{"Authorization", "Bearer " + token}
	page := r.get("/w/" + pub)
	data := dataName(t, string(page.body))
	onDisk := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(r.a.dir, "public", pub, name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("routes", func(t *testing.T) { // H-2, H-6, H-8, H-9
		r.check("page", page, 200, ccNoCache)
		if !bytes.Equal(page.body, onDisk("index.html")) {
			t.Error("page is not index.html")
		}
		r.check("page index.html", r.get("/w/"+pub+"/index.html"), 200, ccNoCache)
		if etag := page.h.Get("Etag"); etag == "" {
			t.Error("no ETag on the page")
		} else {
			r.check("page revalidated", r.get("/w/"+pub, "If-None-Match", etag), 304, ccNoCache)
		}
		generic := r.siteFile("generic.html")
		draft, _ := r.upload("ride-30min.fit") // a draft has no public folder: generic page
		for _, p := range []string{"/w/" + draft, "/w/0123456789abcdefABCDEF", "/w/x", "/w/" + pub + "/", "/w/" + pub + "/a/b"} {
			got := r.get(p)
			r.check("generic "+p, got, 404, ccNoCache)
			if !bytes.Equal(got.body, generic) {
				t.Errorf("%s: not the generic page", p)
			}
		}
		r.check("unknown name", r.get("/w/"+pub+"/notes.txt"), 404, ccNoCache)
		r.check("missing hashed name", r.get("/w/"+pub+"/data.0123456789abcdef.bin"), 404, ccNoCache)
		r.check("dot file", r.get("/w/"+pub+"/.index.html"), 404, ccNoCache)
		// F-23 AC 6: a draft (and an expired link, SH-10 below) gets the generic page: no card.
		if bytes.Contains(generic, []byte("og:")) || bytes.Contains(generic, []byte("preview")) {
			t.Error("the generic page has a card")
		}
		// F-23 AC 7 (SH-11): the same fixed warning card for two workouts and a draft.
		other, _ := r.publish("run-30min.fit")
		edit := r.siteFile("edit.html")
		for _, p := range []string{"/edit/" + pub, "/published/" + pub, "/edit/" + other, "/published/" + other, "/edit/" + draft, "/published/" + draft} {
			got := r.get(p)
			r.check(p, got, 200, ccNoStore)
			if !bytes.Equal(got.body, edit) {
				t.Errorf("%s: not edit.html", p)
			}
		}
		for _, want := range []string{`<meta property="og:title" content="Edit link: do not share" />`,
			`<meta property="og:description" content="Anyone with this link can change or delete this workout." />`, `<meta name="twitter:card" content="summary" />`} {
			if !bytes.Contains(edit, []byte(want)) {
				t.Errorf("edit.html lacks %q", want)
			}
		}
		if bytes.Contains(edit, []byte("og:image")) || bytes.Contains(edit, []byte("og:url")) || bytes.Contains(edit, []byte("{{")) ||
			regexp.MustCompile(`[0-9A-Za-z]{22}`).Match(edit) {
			t.Error("edit.html has an image, a URL, a template action or an ID")
		}
		// DS-4: the theme head script, as built and as html/template wrote it, is allowed by its hash (ADR-032 item 3).
		for name, body := range map[string][]byte{"page": page.body, "edit.html": edit} {
			scripts := regexp.MustCompile(`<script>([^<]*)</script>`).FindAllSubmatch(body, -1)
			if len(scripts) != 1 {
				t.Errorf("%s: %d inline scripts, want the theme script", name, len(scripts))
			}
			for _, s := range scripts {
				sum := sha256.Sum256(s[1])
				if !strings.Contains(securitySet["Content-Security-Policy"], "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'") {
					t.Errorf("%s: inline script %q has no hash in the CSP", name, s[1])
				}
			}
		}
		// F-38 IM-8 (F-23 PC-11): robots.txt names only the Impressum addresses, never /w/, also without an Impressum.
		robots := r.get("/robots.txt")
		r.check("robots.txt", robots, 200, ccNoCache)
		if string(robots.body) != robotsTxt || robots.h.Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Errorf("robots.txt: %q %q", robots.body, robots.h.Get("Content-Type"))
		}
		// F-38 AC 1: without an Impressum no page, no text address, no footer link.
		r.check("impressum unset", r.get("/impressum"), 404, ccNoCache)
		if got, unknown := r.get("/api/impressum", impressumHdr...), r.get("/api/nothing-here"); got.code != 404 || !bytes.Equal(got.body, unknown.body) {
			t.Errorf("api impressum unset: %d %q", got.code, got.body)
		}
		got := r.get("/")
		r.check("upload page", got, 200, ccNoCache)
		if !bytes.Equal(got.body, r.siteFile("upload.html")) {
			t.Error("/ is not upload.html")
		}
		assets, err := os.ReadDir(filepath.Join(r.web, "assets"))
		if err != nil || len(assets) == 0 {
			t.Fatal("no assets", err)
		}
		for _, e := range assets { // the map worker among them (ADR-014 item 6)
			r.check("asset "+e.Name(), r.get("/assets/"+e.Name()), 200, ccAsset)
		}
		r.check("missing asset", r.get("/assets/main-00000000.js"), 404, ccNoCache)
		r.check("unknown path", r.get("/about-nothing"), 404, ccNoCache)
		r.check("template", r.get("/index.html"), 404, ccNoCache)
		// F-29 AB-4: Go's filled page through Caddy; the build's file is never served.
		about := r.get("/about")
		r.check("about", about, 200, ccNoCache)
		if bytes.Contains(about.body, []byte("<!--wx:")) || !bytes.Contains(about.body, []byte("[OPERATOR DETAILS")) {
			t.Error("/about is not the filled page")
		}
		r.check("about.html", r.get("/about.html"), 404, ccNoCache)
		for _, p := range []string{"/", "/w/" + draft, "/edit/" + pub, "/report-delete", "/about", "/w/" + pub} {
			if bytes.Contains(r.get(p).body, []byte("Impressum")) {
				t.Errorf("%s names the Impressum while it is unset", p)
			}
		}
		// F-37 DK-11: the delete link's page, never cached; its .html name is not served.
		page := r.siteFile("report-delete.html")
		rd := r.get("/report-delete")
		r.check("report-delete", rd, 200, ccNoStore)
		if !bytes.Equal(rd.body, page) || bytes.Contains(page, []byte("og:")) || bytes.Contains(page, []byte("twitter:")) {
			t.Error("/report-delete is not report-delete.html, or the page has a card")
		}
		r.check("report-delete.html", r.get("/report-delete.html"), 404, ccNoCache)
		r.check("report-delete/", r.get("/report-delete/"), 404, ccNoCache)
		// DK-12 through Caddy: the one invalid answer, Go's no-store once.
		inv := r.get("/api/report-delete", "Authorization", "Bearer "+strings.Repeat("Z", 22))
		r.check("api report-delete", inv, 404, ccNoStore)
		if !strings.Contains(string(inv.body), "This delete link is no longer valid") {
			t.Errorf("api report-delete body %s", inv.body)
		}

		// /api/*: Go's Cache-Control, once; Go's own nosniff not doubled.
		r.check("api health", r.get("/api/health"), 200, ccNoStore)
		r.check("api settings", r.get("/api/workouts/"+pub, auth...), 200, ccNoStore)
		r.check("api original", r.get("/api/workouts/"+pub+"/original.fit", auth...), 200, ccNoStore)
		r.check("api not found", r.get("/api/workouts/"+pub), 404, ccNoStore)
		r.check("api status", r.get("/api/status"), 200, ccNoStore)
	})

	t.Run("preview", func(t *testing.T) { // F-23 AC 1, 4 (PC-7, PC-8)
		m := regexp.MustCompile(`<meta property="og:image" content="` + regexp.QuoteMeta(r.site) + `/w/` + pub + `/(preview\.[0-9a-f]{16}\.png)" />`).FindSubmatch(page.body)
		if m == nil {
			t.Fatal("no absolute og:image under the base URL")
		}
		got := r.get("/w/"+pub+"/"+string(m[1]), "Accept-Encoding", "br, gzip, zstd")
		r.check("preview", got, 200, ccWorkout)
		if got.h.Get("Content-Type") != "image/png" || got.h.Get("Content-Encoding") != "" || !bytes.Equal(got.body, onDisk(string(m[1]))) || len(got.body) >= 300_000 {
			t.Errorf("preview: %q, encoding %q, %d bytes", got.h.Get("Content-Type"), got.h.Get("Content-Encoding"), len(got.body))
		}
	})

	t.Run("data file", func(t *testing.T) { // H-7, QA-15-r1-3
		got := r.get("/w/"+pub+"/"+data, "Accept-Encoding", "br, gzip, zstd")
		r.check("data br", got, 200, ccWorkout)
		if got.h.Get("Content-Encoding") != "br" || !slices.Contains(strings.Split(got.h.Get("Vary"), ", "), "Accept-Encoding") ||
			got.h.Get("Content-Range") != "" || !bytes.Equal(got.body, onDisk(data+".br")) {
			t.Errorf("data br: encoding %q, vary %q, range %q, %d bytes", got.h.Get("Content-Encoding"), got.h.Get("Vary"), got.h.Get("Content-Range"), len(got.body))
		}
		got = r.get("/w/" + pub + "/" + data)
		r.check("data identity", got, 200, ccWorkout)
		if got.h.Get("Content-Encoding") != "" || !bytes.Equal(got.body, onDisk(data)) {
			t.Errorf("data identity: encoding %q", got.h.Get("Content-Encoding"))
		}
		// Everything else is compressed on the fly, draft.bin included.
		for _, p := range [][]string{{"/w/" + pub}, append([]string{"/api/workouts/" + pub + "/draft.bin"}, auth...)} {
			got := r.get(p[0], append([]string{"Accept-Encoding", "zstd"}, p[1:]...)...)
			if got.code != 200 || got.h.Get("Content-Encoding") != "zstd" {
				t.Errorf("%s: %d, encoding %q", p[0], got.code, got.h.Get("Content-Encoding"))
			}
		}
	})

	t.Run("never reachable", func(t *testing.T) { // H-10, F-12 AC 6 through Caddy
		orig, draft := r.a.privateFile(id, "original.fit"), r.a.privateFile(id, "draft.bin")
		db, err := os.ReadFile(filepath.Join(r.a.dir, "workouts.db"))
		if err != nil {
			t.Fatal(err)
		}
		paths := append(guessedPaths(pub, id, token),
			"/workouts.db", "/workouts.db-wal", "/public.lock", "/tmp/", "/public/", "/w/../workouts.db", "/w/%2e%2e/workouts.db",
			"/w/"+pub+"/%2e%2e/%2e%2e/workouts.db", "/w/"+pub+"/..%2f..%2fworkouts.db", "/assets/../index.html", "/assets/%2e%2e/index.html",
			"/w/"+pub+"/%2e%2e/%2e%2e/private/"+id+"/original.fit", "/edit/../private/"+id+"/draft.bin", "/w/"+pub+"/"+data+"/../../../workouts.db")
		for _, p := range paths {
			for _, m := range []string{"GET", "HEAD"} {
				got := r.do(m, p, nil, "Accept-Encoding", "identity")
				if got.code/100 == 2 && (bytes.Contains(got.body, orig) || bytes.Contains(got.body, draft) || bytes.Contains(got.body, []byte("WEXD")) ||
					bytes.Contains(got.body, db[:16]) || bytes.Contains(got.body, []byte("{{"))) {
					t.Errorf("%s %s serves a private file, a data file or the template", m, p)
				}
				for k, v := range securitySet {
					if got.h.Get(k) != v {
						t.Errorf("%s %s: %s = %q", m, p, k, got.h.Get(k))
					}
				}
			}
		}
	})

	t.Run("SH-10", func(t *testing.T) {
		// Hiding on: remove first, the old data URL is gone and the page names a new one.
		since := time.Now()
		oldPreview := previewName(t, string(r.get("/w/"+pub).body))
		if got := r.do("PATCH", "/api/workouts/"+pub, strings.NewReader(`{"hide_ends":true}`), append(auth, "Content-Type", "application/json")...); got.code != 200 {
			t.Fatalf("hiding on: %d %s", got.code, got.body)
		}
		r.gone("hiding on", pub, data, since)
		r.gone("hiding on: preview", pub, oldPreview, since)
		if dataName(t, string(r.get("/w/"+pub).body)) == data {
			t.Error("hiding on: same data name")
		}

		// Delete.
		pubB, tokenB := r.publish("ride-30min.fit")
		dataB := dataName(t, string(r.get("/w/"+pubB).body))
		previewB := previewName(t, string(r.get("/w/"+pubB).body))
		since = time.Now()
		if got := r.do("DELETE", "/api/workouts/"+pubB, nil, "Authorization", "Bearer "+tokenB); got.code != 204 {
			t.Fatalf("delete: %d", got.code)
		}
		r.gone("delete", pubB, dataB, since)
		r.gone("delete: preview", pubB, previewB, since)

		// Expiry: the row's deadline moved to the near future; the timer runs every 100 ms.
		pubC, _ := r.publish("ride-30min.fit")
		dataC := dataName(t, string(r.get("/w/"+pubC).body))
		previewC := previewName(t, string(r.get("/w/"+pubC).body))
		deadline := time.Now().Unix() + 2
		r.a.setTimes(pubC, deadline-7*day, deadline, 0)
		if got := r.get("/w/" + pubC + "/" + dataC); got.code != 200 {
			t.Fatalf("before the deadline: %d", got.code)
		}
		r.gone("expiry", pubC, dataC, time.Unix(deadline, 0))
		r.gone("expiry: preview", pubC, previewC, time.Unix(deadline, 0))
	})

	t.Run("Go stopped", func(t *testing.T) { // AC 2: no request of the view path reaches Go
		r.stopGo()
		page := r.get("/w/" + pub)
		r.check("page", page, 200, ccNoCache)
		data := dataName(t, string(page.body))
		r.check("data", r.get("/w/"+pub+"/"+data, "Accept-Encoding", "br"), 200, ccWorkout)
		if bytes.Contains(page.body, []byte("/api/")) || bytes.Contains(page.body, []byte("wx-edit")) {
			t.Error("the page names the API or the edit entry")
		}
		// Every chunk the page reaches loads; none but the edit chunk (loaded
		// only for #wx-edit, F-19) and the report form (loaded only on a click on
		// Report, F-28 RF-2) names the API.
		// shortcut: a static scan, not a browser network log; the H-12 browser check covers the rest.
		owner := ownerChunks(t, r.web)
		seen := map[string]bool{}
		queue := []string{}
		for _, m := range assetRef.FindAllStringSubmatch(string(page.body), -1) {
			queue = append(queue, m[1])
		}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			if seen[n] {
				continue
			}
			seen[n] = true
			got := r.get("/assets/" + n)
			r.check("asset "+n, got, 200, ccAsset)
			if owner[n] {
				continue
			}
			if bytes.Contains(got.body, []byte("/api/")) {
				t.Errorf("%s names the API", n)
			}
			for _, m := range assetRef.FindAllStringSubmatch(string(got.body), -1) {
				queue = append(queue, m[1])
			}
		}
		if len(seen) < 3 {
			t.Errorf("only %d chunks reached: %v", len(seen), seen)
		}
		r.check("api while Go is down", r.get("/api/health"), 502, ccNoCache)
	})
}

// ownerChunks returns the file names (under assets/) of the edit-flow and
// report-form chunks in the Vite manifest; they may name the API (QA-22-r1-4).
func ownerChunks(t *testing.T, web string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(web, ".vite", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]struct{ File string }
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for k, v := range m {
		if strings.HasPrefix(k, "src/edit/") || strings.HasPrefix(k, "src/report/") {
			out[strings.TrimPrefix(v.File, "assets/")] = true
		}
	}
	if len(out) == 0 {
		t.Fatal("no src/edit/ chunk in the manifest")
	}
	return out
}

// TestCaddyForwardedFor (F-31 DP-1b, AC 4): the client address Go receives. The
// test client connects from 127.0.0.1. From an untrusted peer the header is
// ignored; from a trusted one Go gets the rightmost untrusted entry, as the
// only entry and the only header line.
func TestCaddyForwardedFor(t *testing.T) {
	var got []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Values("X-Forwarded-For")
	}))
	defer up.Close()
	tests := []struct {
		name, trusted string
		sent          []string
		want          string
	}{
		{"untrusted peer: forged header ignored", "", []string{"9.9.9.9"}, "127.0.0.1"},
		{"untrusted peer: forged list ignored", "", []string{"1.1.1.1, 9.9.9.9"}, "127.0.0.1"},
		{"trusted peer: the client", "127.0.0.1", []string{"9.9.9.9"}, "9.9.9.9"},
		{"trusted peer: client-prepended entries dropped", "127.0.0.1", []string{"7.7.7.7, 9.9.9.9"}, "9.9.9.9"},
		{"trusted peer: two header lines", "127.0.0.1", []string{"7.7.7.7", "9.9.9.9"}, "9.9.9.9"},
		{"trusted peer: IPv6 client", "127.0.0.1", []string{"2001:db8::1"}, "2001:db8::1"},
		{"trusted peer, no header: the peer", "127.0.0.1", nil, "127.0.0.1"},
		{"another range: peer not trusted", "10.0.0.0/8", []string{"9.9.9.9"}, "127.0.0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := startRigWith(t, up.Listener.Addr().String(), tc.trusted)
			got = nil
			req, _ := http.NewRequest("GET", r.site+"/api/health", nil)
			for _, v := range tc.sent {
				req.Header.Add("X-Forwarded-For", v)
			}
			resp, err := r.client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("Go received %q, want exactly [%q]", got, tc.want)
			}
		})
	}
}

// TestCaddyLogsNoRequests (F-31 DP-9): with Go down, a 502 and a 404 leave no
// client address, path, query or credential in Caddy's output.
func TestCaddyLogsNoRequests(t *testing.T) {
	r := startRigWith(t, freeAddr(t), "127.0.0.1") // nothing listens there
	r.check("api down", r.get("/api/workouts?q=secret-query", "Authorization", "Bearer secret-token", "X-Forwarded-For", "203.0.113.77"), 502, ccNoCache)
	r.check("about down", r.get("/about"), 502, ccNoCache) // F-29 AB-4
	r.check("missing", r.get("/w/0123456789abcdefABCDEF/secret-name.bin", "X-Forwarded-For", "203.0.113.77"), 404, ccNoCache)
	time.Sleep(200 * time.Millisecond)
	out := r.logs.String()
	for _, bad := range []string{"203.0.113.77", "secret-query", "secret-token", "secret-name", "remote_ip", "/api/workouts"} {
		if strings.Contains(out, bad) {
			t.Errorf("Caddy's output contains %q:\n%s", bad, out)
		}
	}
}

// robotsTxt is F-38 IM-8's body, exactly.
const robotsTxt = "User-agent: *\nDisallow: /impressum\nDisallow: /api/impressum\n"

// impressumHdr is what the /impressum page's script sends (IM-5).
var impressumHdr = []string{"Wx-Impressum", "1", "Sec-Fetch-Site", "same-origin"}

// TestCaddyImpressum (F-38 AC 1 to 3, 5, 6, 10 to 12, through Caddy): the
// footer link on every page type, the page without the text, the text only
// from /api/impressum and only for the page's own request, 30 per hour.
func TestCaddyImpressum(t *testing.T) {
	const marker = "Distinctive-Impressum-Marker-7Q2"
	block := "<h2>Impressum</h2>\n<p>" + marker + " &lt;b&gt;</p>\n"
	r := startRigWith(t, "", "", block)
	pub, _ := r.publish("ride-30min.fit")
	draft, _ := r.upload("ride-30min.fit")

	page := r.get("/impressum")
	r.check("impressum", page, 200, ccNoCache)
	if !bytes.Equal(page.body, r.siteFile("impressum.html")) || !bytes.Contains(page.body, []byte(`<meta name="robots" content="noindex, nofollow, noarchive"`)) {
		t.Error("/impressum is not the filled shell or has no robots meta")
	}
	r.check("impressum.html", r.get("/impressum.html"), 404, ccNoCache)
	r.check("impressum/", r.get("/impressum/"), 404, ccNoCache)
	// AC 12: every page type has the link once; AC 3: none but the text address has the text.
	for _, p := range []string{"/", "/w/" + draft, "/w/0123456789abcdefABCDEF", "/edit/" + pub, "/published/" + pub, "/report-delete", "/about", "/w/" + pub, "/impressum"} {
		got := r.get(p)
		if bytes.Count(got.body, []byte(shell.Link)) != 1 || bytes.Contains(got.body, []byte(marker)) || bytes.Contains(got.body, []byte(shell.Marker)) {
			t.Errorf("%s: %d links, or the text or a marker", p, bytes.Count(got.body, []byte(shell.Link)))
		}
	}
	unknown := r.get("/api/nothing-here")
	for label, hdr := range map[string][]string{"curl": nil, "cross-site": {"Wx-Impressum", "1", "Sec-Fetch-Site", "cross-site"},
		"other origin": {"Origin", "https://evil.example", "Referer", "https://evil.example/"}} {
		got := r.get("/api/impressum", hdr...)
		if got.code != 404 || !bytes.Equal(got.body, unknown.body) || got.h.Get("Content-Type") != unknown.h.Get("Content-Type") {
			t.Errorf("%s: %d %q", label, got.code, got.body)
		}
	}
	for i := 1; i <= 30; i++ {
		got := r.get("/api/impressum", impressumHdr...)
		if i == 1 {
			r.check("api impressum", got, 200, ccNoStore)
			if string(got.body) != block || got.h.Get("Content-Type") != "text/html; charset=utf-8" {
				t.Errorf("api impressum: %q %q", got.body, got.h.Get("Content-Type"))
			}
		} else if got.code != 200 {
			t.Fatalf("request %d: %d", i, got.code)
		}
	}
	got := r.get("/api/impressum", impressumHdr...)
	r.check("31st", got, 429, ccNoStore)
	if bytes.Contains(got.body, []byte(marker)) || got.h.Get("Retry-After") == "" {
		t.Errorf("31st: %q", got.body)
	}
}
