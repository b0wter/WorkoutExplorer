package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// F-70 through newApp and serve, against local fakes. Throwaway values only.
const (
	testNtfyToken = "tk_throwawayNtfyToken0123456789"
	testTopic     = "/throwawayTopicName"
)

func TestNotifySettings(t *testing.T) {
	secret := "SECRETVALUE"
	for _, c := range []struct {
		name string
		env  map[string]string
		err  string // "": valid
	}{
		{"off", nil, ""},
		{"ntfy", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "https://ntfy.example.org" + testTopic}, ""},
		{"ntfy with token", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "https://ntfy.example.org" + testTopic, envNotifyToken: testNtfyToken}, ""},
		{"callback http loopback", map[string]string{envNotifyKind: "callback", envNotifyURL: "http://127.0.0.1:9000/in"}, ""},
		{"kind only", map[string]string{envNotifyKind: "ntfy"}, envNotifyURL},
		{"url only", map[string]string{envNotifyURL: "https://ntfy.example.org" + testTopic}, envNotifyKind},
		{"token only", map[string]string{envNotifyToken: testNtfyToken}, envNotifyKind},
		{"unknown kind", map[string]string{envNotifyKind: secret, envNotifyURL: "https://h.example.org/t"}, envNotifyKind},
		{"two services", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "https://ntfy.example.org" + testTopic, envPushoverToken: testPushToken, envPushoverUser: testPushUser}, envNotifyKind},
		{"two services, one pushover key", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "https://ntfy.example.org" + testTopic, envPushoverUser: testPushUser}, envPushoverToken},
		{"http public", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "http://ntfy.example.org" + testTopic + secret}, envNotifyURL},
		{"http name", map[string]string{envNotifyKind: "callback", envNotifyURL: "http://ntfy.lan/" + secret}, envNotifyURL},
		{"no topic", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "https://ntfy.example.org/"}, envNotifyURL},
		{"garbage", map[string]string{envNotifyKind: "callback", envNotifyURL: "https://exa mple.org/" + secret}, envNotifyURL},
		{"bad token", map[string]string{envNotifyKind: "ntfy", envNotifyURL: "https://ntfy.example.org" + testTopic, envNotifyToken: "a b" + secret}, envNotifyToken},
	} {
		s, err := readSettings(envOf(c.env))
		switch {
		case c.err == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s: %v", c.name, err)
		case err != nil:
			for _, v := range c.env {
				if len(v) > 12 && strings.Contains(err.Error(), v) || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), testTopic) || strings.Contains(err.Error(), testNtfyToken) || strings.Contains(err.Error(), testPushToken) {
					t.Errorf("%s: a value in %q", c.name, err)
				}
			}
		case c.name == "ntfy with token" && (s.notifyKind != "ntfy" || s.notifyToken != testNtfyToken):
			t.Errorf("%+v", s)
		}
	}
	// serve exits 2, prints no value, creates nothing.
	t.Setenv(envNotifyKind, "ntfy")
	t.Setenv(envNotifyURL, "http://ntfy.example.org"+testTopic)
	t.Setenv(envNotifyToken, testNtfyToken)
	dir := filepath.Join(t.TempDir(), "data")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"serve", "-data", dir, "-web", testWeb, "-addr", "127.0.0.1:0"}, &stdout, &stderr); got != 2 {
		t.Fatalf("exit %d", got)
	}
	all := stdout.String() + stderr.String()
	if !strings.Contains(all, envNotifyURL) || strings.Contains(all, testTopic) || strings.Contains(all, testNtfyToken) || strings.Contains(all, "ntfy.example.org") {
		t.Errorf("output %q", all)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("data directory created")
	}
}

// ntfyFake records requests and answers with status.
type ntfyFake struct {
	srv    *httptest.Server
	mu     sync.Mutex
	status int
	reqs   chan *http.Request
	bodies chan string
}

func newNtfyFake(t *testing.T, status int) *ntfyFake {
	t.Helper()
	f := &ntfyFake{status: status, reqs: make(chan *http.Request, 64), bodies: make(chan string, 64)}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.bodies <- string(b)
		f.reqs <- r
		f.mu.Lock()
		defer f.mu.Unlock()
		w.WriteHeader(f.status)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *ntfyFake) next(t *testing.T) (*http.Request, string) {
	t.Helper()
	select {
	case r := <-f.reqs:
		return r, <-f.bodies
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
		return nil, ""
	}
}

func openNotifyApp(t *testing.T, dir string, s settings) *keyApp {
	t.Helper()
	s.maxConcurrentUploads, s.webDir, s.baseURL = 4, testWeb, testBaseURL
	k := &keyApp{app: &app{t: t, dir: dir, logs: &syncBuffer{}}, responses: &syncBuffer{}}
	h, closeApp, err := newApp(context.Background(), dir, s, slog.New(slog.NewJSONHandler(k.logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	k.h, k.close = h, closeApp
	t.Cleanup(func() { closeApp() })
	return k
}

// TestNtfyEndToEnd (F-70 AC 1, 5; NS-5, NS-7): the delete link only in the
// action button, usable once; a failed send revokes that report's key; no
// URL path, token, key or link in any log line.
func TestNtfyEndToEnd(t *testing.T) {
	dir := t.TempDir()
	ok := newNtfyFake(t, 200)
	a := openNotifyApp(t, dir, settings{notifyKind: "ntfy", notifyURL: ok.srv.URL + testTopic, notifyToken: testNtfyToken})
	pub, _ := a.published("ride-30min.fit")
	if rec := a.report(pub, "illegal_content", reportMarker, "198.51.100.1:1"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	r, body := ok.next(t)
	m := regexp.MustCompile(`^view, Delete this workout, "` + regexp.QuoteMeta(testBaseURL) + `/report-delete#([0-9A-Za-z]{22})"$`).FindStringSubmatch(r.Header.Get("Actions"))
	if r.URL.Path != testTopic || body != "New report #1: Illegal content" || m == nil || r.Header.Get("Click") != testBaseURL+"/w/"+pub ||
		r.Header.Get("Authorization") != "Bearer "+testNtfyToken || r.Header.Get("Title") != "Workout Explorer" {
		t.Fatalf("request %s %q %v", r.URL.Path, body, r.Header)
	}
	if strings.Contains(body, m[1]) || strings.Contains(body, reportMarker) {
		t.Errorf("key or report text in the body %q", body)
	}
	if rec := a.key("GET", m[1]); rec.Code != 200 {
		t.Errorf("delivered key: %d", rec.Code)
	}
	if h := a.hashes(); len(h) != 1 || h[0] != hashHex(m[1]) {
		t.Errorf("hashes %v", h)
	}
	// A redirect is a failure: one request, the key revoked.
	target := newNtfyFake(t, 200)
	redir := httptest.NewServer(http.RedirectHandler(target.srv.URL+testTopic, http.StatusFound))
	defer redir.Close()
	a.close()
	b := openNotifyApp(t, dir, settings{notifyKind: "ntfy", notifyURL: redir.URL + testTopic, notifyToken: testNtfyToken})
	if rec := b.report(pub, "spam", "", "198.51.100.1:1"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(b.logs.String(), `"msg":"notification failed"`) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if len(target.reqs) != 0 || len(b.hashes()) != 0 {
		t.Errorf("followed: %d, keys left %v", len(target.reqs), b.hashes())
	}
	for _, bad := range []string{testTopic, testNtfyToken, "report-delete", m[1], reportMarker, "127.0.0.1"} {
		if strings.Contains(a.logs.String()+b.logs.String(), bad) {
			t.Errorf("%q in the logs", bad)
		}
	}
	if !strings.Contains(b.logs.String(), `"msg":"notification failed","kind":"ntfy","status":302`) {
		t.Errorf("log %s", b.logs.String())
	}
}

// TestCallbackEndToEnd (AC 2): the fixed JSON body and the bearer token.
func TestCallbackEndToEnd(t *testing.T) {
	f := newNtfyFake(t, 204)
	a := openNotifyApp(t, t.TempDir(), settings{notifyKind: "callback", notifyURL: f.srv.URL + "/in", notifyToken: testNtfyToken})
	pub, _ := a.published("ride-30min.fit")
	if rec := a.report(pub, "personal_data", reportMarker, "198.51.100.1:1"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	r, body := f.next(t)
	re := `^\{"report":1,"reason":"[A-Za-z ]+","more":0,"view_url":"` + regexp.QuoteMeta(testBaseURL+"/w/"+pub) + `","delete_url":"` + regexp.QuoteMeta(testBaseURL) + `/report-delete#[0-9A-Za-z]{22}"\}$`
	if r.Method != "POST" || r.URL.Path != "/in" || !regexp.MustCompile(re).MatchString(body) || r.Header.Get("Content-Type") != "application/json" ||
		r.Header.Get("Authorization") != "Bearer "+testNtfyToken || strings.Contains(body, reportMarker) {
		t.Errorf("request %s %q %v", r.URL.Path, body, r.Header)
	}
}

// TestNotifyChangeRevokes (F-70 NS-13, F-70 Q1): a change of service, URL or
// token revokes every key at the next start; the same settings do not; the
// state file is 0600 and holds no setting in the clear.
func TestNotifyChangeRevokes(t *testing.T) {
	dir := t.TempDir()
	f := newNtfyFake(t, 200)
	base := settings{notifyKind: "callback", notifyURL: f.srv.URL + "/in", notifyToken: testNtfyToken}
	a := openNotifyApp(t, dir, base)
	pub, _ := a.published("ride-30min.fit")
	report := func(k *keyApp) {
		t.Helper()
		if rec := k.report(pub, "spam", "", "198.51.100.1:1"); rec.Code != 204 {
			t.Fatal(rec.Code)
		}
		f.next(t)
	}
	report(a)
	if len(a.hashes()) != 1 {
		t.Fatalf("keys %v", a.hashes())
	}
	a.close()
	st, err := os.Stat(filepath.Join(dir, notifyStateFile))
	if err != nil || st.Mode().Perm() != 0o600 || st.Size() != 48 {
		t.Fatalf("state file %v %v", st, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, notifyStateFile)); bytes.Contains(b, []byte(testNtfyToken)) || bytes.Contains(b, []byte("/in")) {
		t.Error("a setting in the state file")
	}
	// Same settings: the key survives.
	b := openNotifyApp(t, dir, base)
	if len(b.hashes()) != 1 {
		t.Errorf("same settings revoked: %v", b.hashes())
	}
	b.close()
	for name, change := range map[string]func(*settings){
		"url":     func(s *settings) { s.notifyURL += "x" },
		"token":   func(s *settings) { s.notifyToken += "x" },
		"service": func(s *settings) { s.notifyKind = "ntfy" },
		"off":     func(s *settings) { *s = settings{} },
		"pushover": func(s *settings) {
			*s = settings{pushoverToken: testPushToken, pushoverUser: testPushUser, pushoverEndpoint: f.srv.URL}
		},
	} {
		k := openNotifyApp(t, dir, base) // back to base (a change from the previous variant; no key exists yet)
		report(k)
		if len(k.hashes()) == 0 {
			t.Fatalf("%s: no key issued", name)
		}
		k.close()
		s := base
		change(&s)
		c := openNotifyApp(t, dir, s)
		if h := c.hashes(); len(h) != 0 {
			t.Errorf("%s: keys survived: %v", name, h)
		}
		c.close()
	}
	// A crash between the clearing and the file write clears again: no file, keys cleared.
	k := openNotifyApp(t, dir, base)
	report(k)
	k.close()
	os.Remove(filepath.Join(dir, notifyStateFile))
	if h := openNotifyApp(t, dir, base).hashes(); len(h) != 0 {
		t.Errorf("missing state file (first start after the upgrade): %v", h)
	}
}

// TestAboutNotifyBlocks (NS-8): each service gets its own text; ntfy names
// its host (escaped) but never the path; a callback shows no host.
func TestAboutNotifyBlocks(t *testing.T) {
	get := func(s settings) string {
		s.webDir, s.baseURL, s.maxConcurrentUploads = testWeb, testBaseURL, 4
		h, closeApp, err := newApp(context.Background(), t.TempDir(), s, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		defer closeApp()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/about", nil))
		return rec.Body.String()
	}
	if got := get(settings{notifyKind: "ntfy", notifyURL: "https://ntfy.example.org:8443" + testTopic + "?a=1"}); !strings.Contains(got, "NTFY ntfy.example.org:8443") ||
		strings.Contains(got, "PUSH") || strings.Contains(got, "CB") || strings.Contains(got, "throwaway") {
		t.Errorf("ntfy: %q", got)
	}
	if got := get(settings{notifyKind: "callback", notifyURL: "https://hooks.example.org/secretpath"}); !strings.Contains(got, "CB") ||
		strings.Contains(got, "NTFY") || strings.Contains(got, "PUSH") || strings.Contains(got, "hooks.example.org") || strings.Contains(got, "secretpath") {
		t.Errorf("callback: %q", got)
	}
	if got := get(settings{}); strings.Contains(got, "NTFY") || strings.Contains(got, "CB") || strings.Contains(got, "PUSH") {
		t.Errorf("off: %q", got)
	}
}

// TestShutdownRevokesInFlight (QA-70-r1-1, F-70 AC 5, NS-7): closeApp waits for
// a send in flight and its revoke, so a key never confirmed stays not valid.
func TestShutdownRevokesInFlight(t *testing.T) {
	got, release := make(chan struct{}, 1), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	a := openNotifyApp(t, t.TempDir(), settings{notifyKind: "ntfy", notifyURL: srv.URL + testTopic})
	pub, _ := a.published("ride-30min.fit")
	if rec := a.report(pub, "spam", "", "198.51.100.1:1"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("send never started")
	}
	if len(a.hashes()) != 1 {
		t.Fatalf("no key issued: %v", a.hashes())
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	if h := a.hashes(); len(h) != 0 {
		t.Errorf("key left valid after shutdown: %v", h)
	}
}
