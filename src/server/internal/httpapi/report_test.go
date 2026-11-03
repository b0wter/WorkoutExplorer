package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/reports"
)

// fakeReporter records calls and answers err (reports' own rule runs first,
// so the handler is tested with the real Check).
type fakeReporter struct {
	calls [][3]string
	err   error
}

func (f *fakeReporter) File(_ context.Context, publicID, reason, message string) error {
	if _, err := reports.Check(reason, message); err != nil {
		return err
	}
	f.calls = append(f.calls, [3]string{publicID, reason, message})
	return f.err
}

func reportHandlerFor(rep Reporter, logs *bytes.Buffer, now func() time.Time) http.Handler {
	return NewHandler(Config{Reports: rep, MaxConcurrentUploads: 1, Logger: slog.New(slog.NewJSONHandler(logs, nil)), Now: now})
}

func postReport(h http.Handler, ctype, body, remote string, hdr ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	r.RemoteAddr = remote
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

const goodID = "AAAAAAAAAAAAAAAAAAAAAA"

// TestReportValidation (F-27 R-5, AC 2): the trust boundary.
func TestReportValidation(t *testing.T) {
	long := strings.Repeat("ü", 1000)
	for _, c := range []struct {
		name, ctype, body string
		status            int
		msg               string
	}{
		{"valid", "application/json", `{"public_id":"` + goodID + `","reason":"spam","message":"hi"}`, 204, ""},
		{"no message", "application/json", `{"public_id":"` + goodID + `","reason":"other"}`, 204, ""},
		{"charset", "application/json; charset=utf-8", `{"public_id":"` + goodID + `","reason":"other"}`, 204, ""},
		{"1000 code points", "application/json", `{"public_id":"` + goodID + `","reason":"other","message":"` + long + `"}`, 204, ""},
		{"line feed", "application/json", `{"public_id":"` + goodID + `","reason":"other","message":"a\nb"}`, 204, ""},
		{"1001 code points", "application/json", `{"public_id":"` + goodID + `","reason":"other","message":"` + long + `x"}`, 400, reports.MsgMessageLong},
		{"control", "application/json", `{"public_id":"` + goodID + `","reason":"other","message":"a\u0007"}`, 400, reports.MsgMessageCtrl},
		{"unknown reason", "application/json", `{"public_id":"` + goodID + `","reason":"rude"}`, 400, reports.MsgReason},
		{"no content type", "", `{"public_id":"` + goodID + `","reason":"spam"}`, 400, msgReportType},
		{"form", "application/x-www-form-urlencoded", `public_id=x&reason=spam`, 400, msgReportType},
		{"text/plain (no CORS preflight)", "text/plain", `{"public_id":"` + goodID + `","reason":"spam"}`, 400, msgReportType},
		{"json-ish", "application/json-patch+json", `{"public_id":"` + goodID + `","reason":"spam"}`, 400, msgReportType},
		{"too large", "application/json", `{"public_id":"` + goodID + `","reason":"spam","message":"` + strings.Repeat(" ", 16<<10) + `"}`, 400, msgReportBody},
		{"empty", "application/json", ``, 400, msgReportBody},
		{"array", "application/json", `[]`, 400, msgReportBody},
		{"null", "application/json", `null`, 400, msgReportBody},
		{"trailing", "application/json", `{"public_id":"` + goodID + `","reason":"spam"} {}`, 400, msgReportBody},
		{"extra key", "application/json", `{"public_id":"` + goodID + `","reason":"spam","email":"a@b"}`, 400, msgReportBody},
		{"duplicate", "application/json", `{"public_id":"` + goodID + `","reason":"rude","reason":"spam"}`, 400, msgReportBody},
		{"null message", "application/json", `{"public_id":"` + goodID + `","reason":"spam","message":null}`, 400, msgReportBody},
		{"number reason", "application/json", `{"public_id":"` + goodID + `","reason":1}`, 400, msgReportBody},
		{"object id", "application/json", `{"public_id":{},"reason":"spam"}`, 400, msgReportBody},
		{"no reason", "application/json", `{"public_id":"` + goodID + `"}`, 400, msgReportBody},
		{"no id", "application/json", `{"reason":"spam"}`, 400, msgReportBody},
	} {
		rep := &fakeReporter{}
		var logs bytes.Buffer
		rec := postReport(reportHandlerFor(rep, &logs, nil), c.ctype, c.body, "198.51.100.7:1")
		if rec.Code != c.status || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body)
			continue
		}
		if c.status == 204 {
			if rec.Body.Len() != 0 || len(rep.calls) != 1 {
				t.Errorf("%s: body %q, calls %v", c.name, rec.Body, rep.calls)
			}
			continue
		}
		if want := `{"error":"invalid_report","message":"` + c.msg + `"}`; rec.Body.String() != want {
			t.Errorf("%s: %s, want %s", c.name, rec.Body, want)
		}
		if len(rep.calls) != 0 || logs.Len() != 0 {
			t.Errorf("%s: calls %v, log %s", c.name, rep.calls, logs.String())
		}
	}
}

// TestReportNotFound (AC 3, ADR-025 item 5): byte-identical to the owner API.
func TestReportNotFound(t *testing.T) {
	rep := &fakeReporter{err: reports.ErrNotFound}
	rec := postReport(reportHandlerFor(rep, &bytes.Buffer{}, nil), "application/json", `{"public_id":"x","reason":"spam"}`, "198.51.100.7:1")
	if rec.Code != 404 || rec.Body.String() != string(notFoundBody) || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	var logs bytes.Buffer
	rep.err = errors.New("disk on fire")
	rec = postReport(reportHandlerFor(rep, &logs, nil), "application/json", `{"public_id":"x","reason":"spam","message":"MSGMARKER"}`, "198.51.100.7:1")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"error":"server_error"`) || !strings.Contains(logs.String(), `"msg":"report failed"`) || strings.Contains(logs.String(), "MSGMARKER") {
		t.Errorf("%d %s log %s", rec.Code, rec.Body, logs.String())
	}
}

// TestReportRateLimit (R-6, AC 4): 60 per rolling hour per IPv4 address and
// per IPv6 /64, checked before anything else; one log line per key, without
// the address.
func TestReportRateLimit(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	rep := &fakeReporter{}
	var logs bytes.Buffer
	h := reportHandlerFor(rep, &logs, func() time.Time { return now })
	for i := 0; i < 60; i++ {
		// Invalid bodies count too (every request past the rate step).
		if rec := postReport(h, "text/plain", "x", "127.0.0.1:1", "X-Forwarded-For", "203.0.113.9"); rec.Code != 400 {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
	}
	for i := 0; i < 2; i++ {
		rec := postReport(h, "application/json", `{"public_id":"`+goodID+`","reason":"spam"}`, "127.0.0.1:1", "X-Forwarded-For", "203.0.113.9")
		if rec.Code != 429 || rec.Header().Get("Retry-After") != "3600" || rec.Body.String() != `{"error":"rate_limited","message":"Too many reports. Please try again later."}` {
			t.Errorf("61st: %d %q %s", rec.Code, rec.Header().Get("Retry-After"), rec.Body)
		}
	}
	if len(rep.calls) != 0 {
		t.Errorf("refused requests reached the service: %v", rep.calls)
	}
	if n := strings.Count(logs.String(), `"msg":"report refused","reason":"rate_limited","status":429`); n != 1 || strings.Contains(logs.String(), "203.0.113") {
		t.Errorf("log %s", logs.String())
	}
	// Another address, and the same IPv6 /64 shares one budget.
	if rec := postReport(h, "application/json", `{"public_id":"`+goodID+`","reason":"spam"}`, "127.0.0.1:1", "X-Forwarded-For", "203.0.113.10"); rec.Code != 204 {
		t.Errorf("other address: %d", rec.Code)
	}
	for i := 0; i < 60; i++ {
		postReport(h, "application/json", `{"public_id":"`+goodID+`","reason":"spam"}`, "[2001:db8:1:2::1]:1")
	}
	if rec := postReport(h, "application/json", `{"public_id":"`+goodID+`","reason":"spam"}`, "[2001:db8:1:2:ffff::9]:1"); rec.Code != 429 {
		t.Errorf("same /64: %d", rec.Code)
	}
	if strings.Contains(logs.String(), "2001:db8") {
		t.Errorf("address in log %s", logs.String())
	}
	// The window is rolling: an hour later the first address may report again.
	now = now.Add(time.Hour + time.Second)
	if rec := postReport(h, "application/json", `{"public_id":"`+goodID+`","reason":"spam"}`, "127.0.0.1:1", "X-Forwarded-For", "203.0.113.9"); rec.Code != 204 {
		t.Errorf("after an hour: %d", rec.Code)
	}
}

// TestReportsNotServedWithoutReporter: the route exists only when wired.
func TestReportsNotServedWithoutReporter(t *testing.T) {
	h := NewHandler(Config{MaxConcurrentUploads: 1})
	if rec := postReport(h, "application/json", `{}`, "198.51.100.7:1"); rec.Code != 404 {
		t.Errorf("%d", rec.Code)
	}
}
