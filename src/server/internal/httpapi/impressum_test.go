package httpapi

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestImpressumAPI (F-38 IM-4 to IM-6).
func TestImpressumAPI(t *testing.T) {
	body := []byte("<h2>Imp &lt;b&gt;</h2>\n")
	var mu sync.Mutex
	now := time.Unix(1_000_000, 0)
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	var logs bytes.Buffer
	h := NewHandler(Config{MaxConcurrentUploads: 1, Impressum: body, Now: clock, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	unset := NewHandler(Config{MaxConcurrentUploads: 1})
	ok := []string{"Wx-Impressum", "1", "Sec-Fetch-Site", "same-origin"}
	const a, b = "198.51.100.7:1234", "198.51.100.8:1234"

	unknown := deleteReq(h, "GET", "/api/nothing-here", a)
	same := func(label string, got *httptest.ResponseRecorder) {
		t.Helper()
		if got.Code != unknown.Code || !bytes.Equal(got.Body.Bytes(), unknown.Body.Bytes()) || !reflect.DeepEqual(got.Header(), unknown.Header()) {
			t.Errorf("%s: %d %v %q, want the unknown-route answer", label, got.Code, got.Header(), got.Body)
		}
	}
	same("unset", deleteReq(unset, "GET", "/api/impressum", a, ok...))
	for label, hdr := range map[string][]string{
		"no header":          nil,
		"header 0":           {"Wx-Impressum", "0"},
		"header twice":       {"Wx-Impressum", "1", "Wx-Impressum", "1"},
		"only Sec-Fetch":     {"Sec-Fetch-Site", "same-origin"},
		"cross-site":         {"Wx-Impressum", "1", "Sec-Fetch-Site", "cross-site"},
		"same-site":          {"Wx-Impressum", "1", "Sec-Fetch-Site", "same-site"},
		"none":               {"Wx-Impressum", "1", "Sec-Fetch-Site", "none"},
		"Origin and Referer": {"Origin", "http://wx.test", "Referer", "http://wx.test/impressum"},
	} {
		for i := 0; i < impressumPerHour+1; i++ { // refusals cost no limiter entry
			same(label, deleteReq(h, "GET", "/api/impressum", a, hdr...))
		}
	}
	// Browsers without Sec-Fetch-* (before Safari 16.4) still pass.
	for i := 0; i < impressumPerHour; i++ {
		hdr := ok
		if i == 0 {
			hdr = ok[:2]
		}
		got := deleteReq(h, "GET", "/api/impressum", a, hdr...)
		if got.Code != 200 || !bytes.Equal(got.Body.Bytes(), body) || got.Header().Get("Content-Type") != "text/html; charset=utf-8" || got.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("request %d: %d %v %q", i+1, got.Code, got.Header(), got.Body)
		}
	}
	got := deleteReq(h, "GET", "/api/impressum", a, ok...)
	if got.Code != 429 || got.Header().Get("Retry-After") == "" || bytes.Contains(got.Body.Bytes(), []byte("Imp")) {
		t.Errorf("31st: %d %v %q", got.Code, got.Header(), got.Body)
	}
	if got := deleteReq(h, "GET", "/api/impressum", b, ok...); got.Code != 200 {
		t.Errorf("other address: %d", got.Code)
	}
	mu.Lock()
	now = now.Add(time.Hour + 2*time.Second)
	mu.Unlock()
	if got := deleteReq(h, "GET", "/api/impressum", a, ok...); got.Code != 200 {
		t.Errorf("after the window: %d", got.Code)
	}
	if logs.Len() != 0 {
		t.Errorf("log lines: %s", logs.String())
	}
}
