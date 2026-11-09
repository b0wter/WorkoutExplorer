package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/reports"
)

// fakeKeys records the keys it was given; the real format check runs in
// reports, so the fake applies the same rule.
type fakeKeys struct {
	lookups, redeems []string
	err              error
}

func (f *fakeKeys) LookupDeleteKey(_ context.Context, key string) (reports.KeyInfo, error) {
	f.lookups = append(f.lookups, key)
	if f.err != nil {
		return reports.KeyInfo{}, f.err
	}
	return reports.KeyInfo{Title: "Ride, 8 Nov 2026", StartDate: "8 Nov 2026, 07:30", ReportID: 5, Reason: "Personal data",
		FiledAt: time.Unix(1_000, 0), ExpiresAt: time.Unix(2_000, 0), ReportCount: 3}, nil
}

func (f *fakeKeys) RedeemDeleteKey(_ context.Context, key string) error {
	f.redeems = append(f.redeems, key)
	return f.err
}

const testKey = "KKKKKKKKKKKKKKKKKKKKKK"

func deleteReq(h http.Handler, method, target, remote string, hdr ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.RemoteAddr = remote
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Add(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func isLinkInvalid(rec *httptest.ResponseRecorder) bool {
	return rec.Code == 404 && bytes.Equal(rec.Body.Bytes(), linkInvalidBody) &&
		rec.Header().Get("Content-Type") == "application/json" && rec.Header().Get("Cache-Control") == "no-store" &&
		rec.Header().Get("WWW-Authenticate") == ""
}

// TestReportDeleteAPI (F-37 DK-6, DK-7, DK-12): the key only from one Bearer
// header; any query string is invalid before the key is looked at; the
// lookup's exact fields; the confirm reads no body; errors map to the one
// invalid answer or 500 without the key in the log.
func TestReportDeleteAPI(t *testing.T) {
	var logs bytes.Buffer
	k := &fakeKeys{}
	h := NewHandler(Config{ReportKeys: k, MaxConcurrentUploads: 1, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	auth := []string{"Authorization", "Bearer " + testKey}
	ip := "198.51.100.7:1"

	rec := deleteReq(h, "GET", "/api/report-delete", ip, auth...)
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != 200 || err != nil || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("lookup %d %s", rec.Code, rec.Body)
	}
	want := map[string]any{"title": "Ride, 8 Nov 2026", "start_date": "8 Nov 2026, 07:30", "report_id": 5.0, "reason": "Personal data",
		"filed_at": 1000.0, "expires_at": 2000.0, "report_count": 3.0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lookup body %v", got)
	}
	// POST: no body, no content type needed; a form-like body is ignored.
	r := httptest.NewRequest("POST", "/api/report-delete", strings.NewReader("key="+testKey))
	r.RemoteAddr = ip
	r.Header.Set("Authorization", "bearer "+testKey) // scheme without regard to case
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 204 || rec.Body.Len() != 0 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("confirm %d %s", rec.Code, rec.Body)
	}
	if !reflect.DeepEqual(k.lookups, []string{testKey}) || !reflect.DeepEqual(k.redeems, []string{testKey}) {
		t.Fatalf("keys %v %v", k.lookups, k.redeems)
	}

	// The Bearer parser: anything else is "" (reports then answers invalid).
	k.lookups = nil
	for _, hdr := range [][]string{nil, {"Authorization", "Basic " + testKey}, {"Authorization", testKey},
		{"Authorization", "Bearer " + testKey, "Authorization", "Bearer " + testKey}} {
		deleteReq(h, "GET", "/api/report-delete", ip, hdr...)
	}
	if !reflect.DeepEqual(k.lookups, []string{"", "", "", ""}) {
		t.Errorf("keys passed %q", k.lookups)
	}

	// A query string is invalid even with a valid header, and the key is not used.
	k.lookups, k.redeems = nil, nil
	for _, m := range []string{"GET", "POST"} {
		for _, target := range []string{"/api/report-delete?key=" + testKey, "/api/report-delete?"} {
			if rec := deleteReq(h, m, target, ip, auth...); !isLinkInvalid(rec) {
				t.Errorf("%s %s: %d %s", m, target, rec.Code, rec.Body)
			}
		}
	}
	if k.lookups != nil || k.redeems != nil {
		t.Errorf("a request with a query reached reports: %v %v", k.lookups, k.redeems)
	}

	// Errors.
	k.err = reports.ErrLinkInvalid
	for _, m := range []string{"GET", "POST"} {
		if rec := deleteReq(h, m, "/api/report-delete", ip, auth...); !isLinkInvalid(rec) {
			t.Errorf("%s invalid: %d %s", m, rec.Code, rec.Body)
		}
	}
	if logs.Len() != 0 {
		t.Errorf("refusals logged: %s", logs.String())
	}
	k.err = errors.New("disk")
	for _, m := range []string{"GET", "POST"} {
		if rec := deleteReq(h, m, "/api/report-delete", ip, auth...); rec.Code != 500 {
			t.Errorf("%s failure: %d", m, rec.Code)
		}
	}
	if l := logs.String(); strings.Count(l, `"msg":"report delete failed"`) != 2 || strings.Contains(l, testKey) || strings.Contains(l, "198.51.100") {
		t.Errorf("log %s", l)
	}

	// Other methods: not served.
	if rec := deleteReq(h, "DELETE", "/api/report-delete", ip, auth...); rec.Code != 405 {
		t.Errorf("DELETE: %d", rec.Code)
	}
}

// TestReportDeleteRateLimit (DK-9, AC 38): 30 per rolling hour per address,
// lookup and confirm together, checked before anything else; 429 with
// Retry-After; one log line per address, without it.
func TestReportDeleteRateLimit(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	var logs bytes.Buffer
	k := &fakeKeys{err: reports.ErrLinkInvalid}
	h := NewHandler(Config{ReportKeys: k, MaxConcurrentUploads: 1, Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Now: func() time.Time { return now }})
	if reportDeletePerHour != 30 {
		t.Fatalf("limit %d, DK-9 says 30", reportDeletePerHour)
	}
	for i := 0; i < 30; i++ {
		m := "GET"
		if i%2 == 1 {
			m = "POST"
		}
		if rec := deleteReq(h, m, "/api/report-delete?x", "203.0.113.9:1"); !isLinkInvalid(rec) { // queries count too
			t.Fatalf("request %d: %d", i+1, rec.Code)
		}
	}
	for i := 0; i < 2; i++ {
		rec := deleteReq(h, "POST", "/api/report-delete", "203.0.113.9:2", "Authorization", "Bearer "+testKey)
		var b errorBody
		json.Unmarshal(rec.Body.Bytes(), &b)
		if rec.Code != 429 || rec.Header().Get("Retry-After") != "3600" || b.Error != codeRateLimited || b.Message != msgReportDeleteRate {
			t.Fatalf("refusal %d %v %s", rec.Code, rec.Header(), rec.Body)
		}
	}
	if k.redeems != nil {
		t.Error("a refused request reached reports")
	}
	if rec := deleteReq(h, "GET", "/api/report-delete", "203.0.113.10:1"); rec.Code != 404 {
		t.Errorf("another address: %d", rec.Code)
	}
	if l := logs.String(); strings.Count(l, `"msg":"report delete refused","reason":"rate_limited","status":429`) != 1 || strings.Contains(l, "203.0.113") {
		t.Errorf("log %s", l)
	}
	now = now.Add(time.Hour + time.Second)
	if rec := deleteReq(h, "GET", "/api/report-delete", "203.0.113.9:1"); rec.Code != 404 {
		t.Errorf("after the hour: %d", rec.Code)
	}
}
