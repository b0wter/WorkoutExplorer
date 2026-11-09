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
	"time"

	"workout-explorer/internal/about"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string {
		if v, ok := m[k]; ok {
			return v
		}
		if k == envBaseURL {
			return testBaseURL // required (F-23 PC-8)
		}
		return ""
	}
}

// testBaseURL is the base URL of every test run (TestMain sets it for serve).
const testBaseURL = "http://wx.test:8088"

// TestMain points testWeb at a copy of the source page template and a fixture
// About page that has the generated list, so Go tests need neither the web
// build nor the frontend's about.html (F-15 P-10, F-29 AB-4).
func TestMain(m *testing.M) {
	os.Setenv(envBaseURL, testBaseURL)
	dir, err := os.MkdirTemp("", "wx-web")
	if err != nil {
		panic(err)
	}
	idx, err := os.ReadFile(filepath.Join(testWeb, "index.html"))
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), idx, 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "about.html"), []byte(aboutFixture), 0o644); err != nil {
		panic(err)
	}
	testWeb = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// aboutFixture is a finished About page: the list is generated, every marker is present.
const aboutFixture = `<main><!--wx:operator-->|<!--wx:version-->
<!--wx:pushover-->PUSH<!--/wx:pushover--><!--wx:shortcut--><a href="<!--wx:shortcut-url-->">SC</a><!--/wx:shortcut--><!--wx:generated-->{{ list <!--wx:version--> }}</main>`

// TestAboutTemplateContract: the frontend's source template carries every
// marker serve and the generator fill (F-29 AB-3, AB-5).
func TestAboutTemplateContract(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "about.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{about.MarkerList, "<!--wx:operator-->", "<!--wx:pushover-->", "<!--/wx:pushover-->", "<!--wx:shortcut-->", "<!--wx:shortcut-url-->", "<!--/wx:shortcut-->"} {
		if !strings.Contains(string(src), m) {
			t.Errorf("about.html lacks %s", m)
		}
	}
}

// TestOperatorFileSetting (F-29 AB-8): unset or missing is the placeholder; a
// relative path, an unreadable path, a long, empty or control-character file is an error.
func TestOperatorFileSetting(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), 0o644)
		return p
	}
	if s, err := readSettings(envOf(map[string]string{envOperatorFile: write("ok", "## Who\nJane <b>\n")})); err != nil || s.operatorHTML != "<h2>Who</h2>\n<p>Jane &lt;b&gt;</p>\n" {
		t.Errorf("ok = %+v, %v", s.operatorHTML, err)
	}
	for _, v := range []string{"", filepath.Join(dir, "nope")} {
		if s, err := readSettings(envOf(map[string]string{envOperatorFile: v})); err != nil || s.operatorHTML != "" {
			t.Errorf("%q = %q, %v", v, s.operatorHTML, err)
		}
	}
	for name, v := range map[string]string{"relative": "about.txt", "directory": dir, "big": write("big", strings.Repeat("a", 16<<10+1)),
		"blank": write("blank", " \n\n"), "control": write("ctl", "a\x00b"), "utf8": write("u8", "a\xffb")} {
		if _, err := readSettings(envOf(map[string]string{envOperatorFile: v})); err == nil || !strings.Contains(err.Error(), envOperatorFile) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestIOSShortcutSetting (F-34, INF-5): unset is off, an https link is
// kept as given, anything else is an error naming the setting.
func TestIOSShortcutSetting(t *testing.T) {
	for _, v := range []string{"", "https://www.icloud.com/shortcuts/0123abcd", "https://example.org/s?a=1&b=<x>"} {
		if s, err := readSettings(envOf(map[string]string{envIOSShortcutURL: v})); err != nil || s.iosShortcutURL != v {
			t.Errorf("%q = %q, %v", v, s.iosShortcutURL, err)
		}
	}
	for _, v := range []string{"http://www.icloud.com/shortcuts/x", "/shortcuts/x", "www.icloud.com/shortcuts/x", "shortcuts://run", "javascript:alert(1)",
		"https://", "https:///x", "https://u:p@example.org/", "https://exa mple.org/", "https://example.org/\x00", "Upload here", " https://example.org/"} {
		if _, err := readSettings(envOf(map[string]string{envIOSShortcutURL: v})); err == nil || !strings.Contains(err.Error(), envIOSShortcutURL) {
			t.Errorf("%q: %v", v, err)
		}
	}
	t.Setenv(envIOSShortcutURL, "http://www.icloud.com/shortcuts/x")
	var stderr bytes.Buffer
	if got := run([]string{"serve", "-data", filepath.Join(t.TempDir(), "data"), "-web", testWeb, "-addr", "127.0.0.1:0"}, nil, &stderr); got != 2 || !strings.Contains(stderr.String(), envIOSShortcutURL) {
		t.Errorf("serve = %d, %q", got, stderr.String())
	}
}

// TestAboutRoute (F-29 AB-4): GET and HEAD /about, values filled and escaped,
// the Pushover block only with both secrets, other methods not served.
func TestAboutRoute(t *testing.T) {
	get := func(s settings, method string) *httptest.ResponseRecorder {
		s.webDir, s.baseURL, s.maxConcurrentUploads = testWeb, testBaseURL, 4
		h, closeApp, err := newApp(context.Background(), t.TempDir(), s, slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		defer closeApp()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/about", nil))
		return rec
	}
	rec := get(settings{operatorHTML: "<p>x</p>"}, "GET")
	want := "<p>x</p>|" + version + "\n{{ list " + version + " }}"
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-cache" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(rec.Body.String(), want) {
		t.Errorf("GET = %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
	if strings.Contains(rec.Body.String(), "PUSH") {
		t.Error("Pushover block present without secrets")
	}
	if rec := get(settings{}, "GET"); !strings.Contains(rec.Body.String(), about.Placeholder+"|") {
		t.Errorf("placeholders: %q", rec.Body)
	}
	on := get(settings{pushoverToken: testPushToken, pushoverUser: testPushUser}, "GET").Body.String()
	if !strings.Contains(on, "PUSH") || strings.Contains(on, "<!--/wx:pushover-->") {
		t.Errorf("Pushover block: %q", on)
	}
	if strings.Contains(rec.Body.String(), "SC") {
		t.Error("Shortcut section present without the setting")
	}
	if sc := get(settings{iosShortcutURL: "https://www.icloud.com/shortcuts/x?a=1&b=2"}, "GET").Body.String(); !strings.Contains(sc, `<a href="https://www.icloud.com/shortcuts/x?a=1&amp;b=2">SC</a>`) {
		t.Errorf("Shortcut section: %q", sc)
	}
	if h := get(settings{}, "HEAD"); h.Code != 200 || h.Body.Len() != 0 {
		t.Errorf("HEAD = %d, %d body bytes", h.Code, h.Body.Len())
	}
	if p := get(settings{}, "POST"); p.Code != 404 && p.Code != 405 {
		t.Errorf("POST = %d", p.Code)
	}
}

// TestAboutMissingOrUngenerated (F-29 AB-4): a missing page, one without the
// generated list, and one without a marker stop the start.
func TestAboutMissingOrUngenerated(t *testing.T) {
	for name, page := range map[string]string{
		"missing":            "",
		"not generated":      strings.ReplaceAll(aboutFixture, "<!--wx:generated-->", "<!--wx:attributions-->"),
		"no operator marker": strings.ReplaceAll(aboutFixture, "<!--wx:operator-->", ""),
		"no block":           strings.ReplaceAll(aboutFixture, "<!--wx:pushover-->", ""),
		"no shortcut block":  strings.ReplaceAll(aboutFixture, "<!--/wx:shortcut-->", ""),
	} {
		dir := t.TempDir()
		idx, _ := os.ReadFile(filepath.Join(testWeb, "index.html"))
		os.WriteFile(filepath.Join(dir, "index.html"), idx, 0o644)
		if page != "" {
			os.WriteFile(filepath.Join(dir, "about.html"), []byte(page), 0o644)
		}
		if _, _, err := newApp(context.Background(), t.TempDir(), settings{webDir: dir, baseURL: testBaseURL}, slog.New(slog.DiscardHandler)); err == nil {
			t.Errorf("%s: started", name)
		}
	}
}

// TestBaseURL (F-23 PC-8): origin only, re-serialised unchanged; no default.
func TestBaseURL(t *testing.T) {
	for _, v := range []string{"http://127.0.0.1:8088", "https://workouts.example.org", "http://localhost:1", "https://[::1]:65535", "https://xn--bcher-kva.example"} {
		if got, err := baseURL(envOf(map[string]string{envBaseURL: v})); err != nil || got != v {
			t.Errorf("%q = %q, %v", v, got, err)
		}
	}
	for _, v := range []string{"", "https://example.org/", "https://example.org/w", "ftp://example.org", "HTTPS://example.org",
		"https://user@example.org", "https://example.org?x", "https://example.org?", "https://example.org#x", "https://example.org:",
		"https://example.org:0", "https://example.org:65536", "https://example.org:080", "https://", "example.org", "//example.org",
		" https://example.org", "https://example.org ", "https://exa mple.org", "https:example.org", "https://ex\"ample.org"} {
		if got, err := baseURL(envOf(map[string]string{envBaseURL: v})); err != errBaseURL {
			t.Errorf("%q = %q, %v; want the error", v, got, err)
		}
	}
}

// TestServeWithoutBaseURL: unset is exit 2 before the data directory exists.
func TestServeWithoutBaseURL(t *testing.T) {
	for _, v := range []string{"", "https://example.org/"} {
		t.Setenv(envBaseURL, v)
		dir := filepath.Join(t.TempDir(), "data")
		var stderr bytes.Buffer
		done := make(chan int, 1)
		go func() {
			done <- run([]string{"serve", "-data", dir, "-web", testWeb, "-addr", "127.0.0.1:0"}, nil, &stderr)
		}()
		select {
		case got := <-done:
			if got != 2 {
				t.Fatalf("%q: exit = %d (stderr %q)", v, got, stderr.String())
			}
		case <-time.After(5 * time.Second): // a regression would start serving and never return
			t.Fatalf("%q: serve did not exit", v)
		}
		if !strings.Contains(stderr.String(), "serve: "+errBaseURL.Error()) {
			t.Errorf("stderr = %q", stderr.String())
		}
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("the data directory was created")
		}
	}
}

// TestSettings (AC 8): the number of uploads processed at once.
func TestSettings(t *testing.T) {
	good := map[string]int{"": 4, "1": 1, "2": 2, "4": 4, "9": 9, "10": 10, "16": 16}
	for v, want := range good {
		got, err := maxConcurrentUploads(envOf(map[string]string{envMaxConcurrentUploads: v}))
		if err != nil || got != want {
			t.Errorf("%q = %d, %v; want %d", v, got, err, want)
		}
	}
	if got, err := maxConcurrentUploads(envOf(nil)); err != nil || got != 4 {
		t.Errorf("unset = %d, %v", got, err)
	}
	for _, v := range []string{"0", "17", "-1", "+2", "02", " 2", "2 ", "abc", "1e3", "2.0", "100", "99999999999999999999", "٣"} {
		if got, err := maxConcurrentUploads(envOf(map[string]string{envMaxConcurrentUploads: v})); err == nil {
			t.Errorf("%q accepted as %d", v, got)
		}
	}
}

// TestServeInvalidSetting: serve exits 2 with the message and creates nothing.
func TestServeInvalidSetting(t *testing.T) {
	t.Setenv(envMaxConcurrentUploads, "17")
	dir := filepath.Join(t.TempDir(), "data")
	var stderr bytes.Buffer
	if got := run([]string{"serve", "-data", dir, "-web", testWeb, "-addr", "127.0.0.1:0"}, nil, &stderr); got != 2 {
		t.Fatalf("exit = %d (stderr %q)", got, stderr.String())
	}
	if want := "serve: WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS must be a whole number from 1 to 16"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("the data directory was created")
	}
}

// TestServeWeb (F-15 P-10): -web is required, and a missing or broken
// template stops serve with exit 1 before it listens or creates anything.
func TestServeWeb(t *testing.T) {
	var stderr bytes.Buffer
	if got := run([]string{"serve", "-data", t.TempDir(), "-addr", "127.0.0.1:0"}, nil, &stderr); got != 2 || !strings.Contains(stderr.String(), "serve: -web must not be empty") {
		t.Errorf("no -web: %d %q", got, stderr.String())
	}
	broken := t.TempDir()
	os.WriteFile(filepath.Join(broken, "index.html"), []byte("{{.Page.Nope}}"), 0o600)
	for _, web := range []string{t.TempDir(), broken} {
		dir := filepath.Join(t.TempDir(), "data")
		if got := run([]string{"serve", "-data", dir, "-web", web, "-addr", "127.0.0.1:0"}, io.Discard, io.Discard); got != 1 {
			t.Errorf("template in %s: exit %d", web, got)
		}
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("the data directory was created")
		}
	}
}

// TestServeData: -data is required.
func TestServeData(t *testing.T) {
	for _, args := range [][]string{{"serve"}, {"serve", "-data", ""}, {"serve", "-addr", "127.0.0.1:0"}} {
		var stderr bytes.Buffer
		if got := run(args, nil, &stderr); got != 2 || !strings.Contains(stderr.String(), "serve: -data must not be empty") {
			t.Errorf("run(%q) = %d, stderr %q", args, got, stderr.String())
		}
	}
}

const parseTimeoutMsg = "WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60"

// TestParseTimeoutSetting (F-09 D-30): valid values, the default, invalid values.
func TestParseTimeoutSetting(t *testing.T) {
	good := map[string]time.Duration{"": ingest.DefaultLimits.Timeout, "5": 5 * time.Second, "10": 10 * time.Second, "37": 37 * time.Second, "60": 60 * time.Second}
	for v, want := range good {
		got, err := parseTimeout(envOf(map[string]string{envParseTimeout: v}))
		if err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", v, got, err, want)
		}
	}
	if got, err := parseTimeout(envOf(nil)); err != nil || got != 10*time.Second {
		t.Errorf("unset = %v, %v", got, err)
	}
	for _, v := range []string{"4", "61", "0", "-5", "+10", "010", " 10", "10 ", "10s", "1m", "10.0", "1e1", "abc", "100", "99999999999999999999", "٣"} {
		got, err := parseTimeout(envOf(map[string]string{envParseTimeout: v}))
		if err == nil || err.Error() != parseTimeoutMsg {
			t.Errorf("%q = %v, %v; want the error", v, got, err)
		}
	}
}

// TestDefaultParseTimeoutInRange: the default is a value the setting accepts.
func TestDefaultParseTimeoutInRange(t *testing.T) {
	d := ingest.DefaultLimits.Timeout
	if d%time.Second != 0 || d < 5*time.Second || d > 60*time.Second {
		t.Errorf("default %v is outside the setting's range", d)
	}
}

// TestReadSettings: both variables, the in-flight cap checked first.
func TestReadSettings(t *testing.T) {
	s, err := readSettings(envOf(nil))
	if err != nil || s.maxConcurrentUploads != 4 || s.parseTimeout != 10*time.Second {
		t.Errorf("defaults = %+v, %v", s, err)
	}
	s, err = readSettings(envOf(map[string]string{envMaxConcurrentUploads: "7", envParseTimeout: "42"}))
	if err != nil || s.maxConcurrentUploads != 7 || s.parseTimeout != 42*time.Second {
		t.Errorf("set = %+v, %v", s, err)
	}
	_, err = readSettings(envOf(map[string]string{envMaxConcurrentUploads: "0", envParseTimeout: "1"}))
	if err != errMaxConcurrentUploads {
		t.Errorf("both invalid: %v", err)
	}
	if _, err = readSettings(envOf(map[string]string{envParseTimeout: "1"})); err != errParseTimeout {
		t.Errorf("timeout invalid: %v", err)
	}
}

// TestServeInvalidParseTimeout: serve exits 2 with the message and creates nothing.
func TestServeInvalidParseTimeout(t *testing.T) {
	t.Setenv(envParseTimeout, "61")
	dir := filepath.Join(t.TempDir(), "data")
	var stderr bytes.Buffer
	if got := run([]string{"serve", "-data", dir, "-web", testWeb, "-addr", "127.0.0.1:0"}, nil, &stderr); got != 2 {
		t.Fatalf("exit = %d (stderr %q)", got, stderr.String())
	}
	if want := "serve: " + parseTimeoutMsg; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("the data directory was created")
	}
}

// TestSettingsReachTheParser: the configured time limit reaches the parser
// and the handler's write deadline input; every other limit stays default.
func TestSettingsReachTheParser(t *testing.T) {
	for _, tt := range []struct {
		set  time.Duration
		want time.Duration
	}{{0, 10 * time.Second}, {5 * time.Second, 5 * time.Second}, {60 * time.Second, 60 * time.Second}} {
		var got *ingest.Parser
		startApp(t, settings{parseTimeout: tt.set, onParser: func(p *ingest.Parser) { got = p }})
		if got == nil {
			t.Fatal("onParser not called")
		}
		want := ingest.DefaultLimits
		want.Timeout = tt.want
		if got.Limits() != want {
			t.Errorf("set %v: limits = %+v, want %+v", tt.set, got.Limits(), want)
		}
	}
}

// TestParseCommandTimeout: parse reads the setting before it opens the input.
func TestParseCommandTimeout(t *testing.T) {
	fit := filepath.Join("..", "..", "testdata", "corpus", "synthetic", "smart-recording.fit")
	for _, args := range [][]string{{"parse", fit}, {"parse", filepath.Join(t.TempDir(), "nope.fit")}} {
		t.Setenv(envParseTimeout, "4")
		var stdout, stderr bytes.Buffer
		if got := run(args, &stdout, &stderr); got != 2 || stdout.Len() != 0 || strings.TrimSpace(stderr.String()) != "parse: "+parseTimeoutMsg {
			t.Errorf("run(%q) = %d, stdout %q, stderr %q", args, got, stdout.String(), stderr.String())
		}
	}
	t.Setenv(envParseTimeout, "60")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"parse", fit}, &stdout, &stderr); got != 0 || stdout.Len() == 0 {
		t.Errorf("valid 60: exit %d, stderr %q", got, stderr.String())
	}
}

const storageLimitMsg = "WORKOUT_EXPLORER_STORAGE_LIMIT_MB must be a whole number of megabytes from 1 to 1000000"

// TestStorageLimitSetting (F-10): decimal megabytes, default 1,000, 1 to 1,000,000.
func TestStorageLimitSetting(t *testing.T) {
	good := map[string]int64{"": 1_000_000_000, "1": 1_000_000, "20": 20_000_000, "512": 512_000_000, "10000": 10_000_000_000, "1000000": 1_000_000_000_000}
	for v, want := range good {
		got, err := storageLimit(envOf(map[string]string{envStorageLimit: v}))
		if err != nil || got != want {
			t.Errorf("%q = %d, %v; want %d", v, got, err, want)
		}
	}
	if got, err := storageLimit(envOf(nil)); err != nil || got != 1_000_000_000 {
		t.Errorf("unset = %d, %v", got, err)
	}
	for _, v := range []string{"0", "1000001", "9999999", "-1", "+5", "05", " 5", "5 ", "5MB", "5G", "1e3", "1.5", "1_000", "1,000", "abc", "99999999999999999999", "٣"} {
		got, err := storageLimit(envOf(map[string]string{envStorageLimit: v}))
		if err == nil || err.Error() != storageLimitMsg {
			t.Errorf("%q = %d, %v; want the error", v, got, err)
		}
	}
	s, err := readSettings(envOf(map[string]string{envStorageLimit: "250"}))
	if err != nil || s.storageLimit != 250_000_000 {
		t.Errorf("readSettings = %+v, %v", s, err)
	}
	if s, err := readSettings(envOf(nil)); err != nil || s.storageLimit != 1_000_000_000 {
		t.Errorf("readSettings default = %+v, %v", s, err)
	}
	if _, err := readSettings(envOf(map[string]string{envStorageLimit: "0"})); err != errStorageLimit {
		t.Errorf("readSettings invalid = %v", err)
	}
}

// TestServeInvalidStorageLimit: serve exits 2 with the message and creates nothing.
func TestServeInvalidStorageLimit(t *testing.T) {
	t.Setenv(envStorageLimit, "0")
	dir := filepath.Join(t.TempDir(), "data")
	var stderr bytes.Buffer
	if got := run([]string{"serve", "-data", dir, "-web", testWeb, "-addr", "127.0.0.1:0"}, nil, &stderr); got != 2 {
		t.Fatalf("exit = %d (stderr %q)", got, stderr.String())
	}
	if want := "serve: " + storageLimitMsg; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("the data directory was created")
	}
}

// TestSettingsReachTheStorage: the configured cap is the one the service and
// the handler use.
func TestSettingsReachTheStorage(t *testing.T) {
	s, err := readSettings(envOf(map[string]string{envStorageLimit: "3"}))
	if err != nil {
		t.Fatal(err)
	}
	var st *lifecycle.Storage
	s.onService = func(_ *lifecycle.Service, x *lifecycle.Storage) { st = x }
	s.webDir = testWeb
	_, closeApp, err := newApp(context.Background(), t.TempDir(), s, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer closeApp()
	if st == nil || st.Limit() != 3_000_000 {
		t.Errorf("storage = %+v", st)
	}
}

// TestProxyAndUploadSettings (F-31 DP-1a, DP-1c): defaults, ranges, fast-fail.
func TestProxyAndUploadSettings(t *testing.T) {
	s, err := readSettings(envOf(nil))
	if err != nil || s.uploadsIPv4 != 60 || s.uploadsIPv6 != 60 || s.trustedProxy.IsValid() {
		t.Errorf("defaults = %+v, %v", s, err)
	}
	s, err = readSettings(envOf(map[string]string{envTrustedProxy: "::ffff:172.30.0.2", envUploadsIPv4: "1", envUploadsIPv6: "240"}))
	if err != nil || s.trustedProxy.String() != "172.30.0.2" || s.uploadsIPv4 != 1 || s.uploadsIPv6 != 240 {
		t.Errorf("set = %+v, %v", s, err)
	}
	for _, tc := range []struct {
		key, val string
		want     error
	}{
		{envUploadsIPv4, "0", errUploadsIPv4}, {envUploadsIPv4, "241", errUploadsIPv4}, {envUploadsIPv4, "060", errUploadsIPv4},
		{envUploadsIPv4, "abc", errUploadsIPv4}, {envUploadsIPv6, "-1", errUploadsIPv6}, {envUploadsIPv6, "1000", errUploadsIPv6},
		{envTrustedProxy, "172.30.0.0/24", errTrustedProxy}, {envTrustedProxy, "caddy", errTrustedProxy},
		{envTrustedProxy, "fe80::1%eth0", errTrustedProxy}, {envTrustedProxy, "1.2.3.4, 5.6.7.8", errTrustedProxy},
	} {
		if _, err := readSettings(envOf(map[string]string{tc.key: tc.val})); err != tc.want {
			t.Errorf("%s=%q: %v, want %v", tc.key, tc.val, err, tc.want)
		}
	}
}

// TestProxyAndLimitsReachTheHandler (QA-31-r1-2, DP-1a, DP-1c): the trusted
// proxy and both upload limits are wired into the handler.
func TestProxyAndLimitsReachTheHandler(t *testing.T) {
	s, err := readSettings(envOf(map[string]string{envTrustedProxy: "10.250.0.9", envUploadsIPv4: "1", envUploadsIPv6: "1"}))
	if err != nil {
		t.Fatal(err)
	}
	s.webDir = testWeb
	h, closeApp, err := newApp(context.Background(), t.TempDir(), s, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer closeApp()
	post := func(xff string) int {
		r := httptest.NewRequest("POST", "/api/workouts", strings.NewReader("x"))
		r.RemoteAddr = "10.250.0.9:5555"
		r.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	for _, tc := range []struct {
		xff  string
		want bool // refused with 429
	}{
		{"198.51.100.7", false}, {"198.51.100.7", true}, {"198.51.100.8", false},
		{"2001:db8:5:6::1", false}, {"2001:db8:5:6::2", true}, {"2001:db8:5:7::1", false},
	} {
		if got := post(tc.xff) == 429; got != tc.want {
			t.Errorf("%s: 429 = %v, want %v", tc.xff, got, tc.want)
		}
	}
}
