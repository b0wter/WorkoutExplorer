package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	const version = "v1.2.3-test"
	h := NewHandler(Config{Version: version, MaxConcurrentUploads: 4})

	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   bool
	}{
		{name: "GET", method: http.MethodGet, wantStatus: http.StatusOK, wantBody: true},
		{name: "HEAD", method: http.MethodHead, wantStatus: http.StatusOK, wantBody: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A real server is used so HEAD behaves as in production
			// (body suppressed by net/http).
			srv := httptest.NewServer(h)
			defer srv.Close()

			req, err := http.NewRequest(tt.method, srv.URL+"/api/health", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := resp.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			if !tt.wantBody {
				return
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			want := map[string]any{"status": "ok", "version": version}
			if len(body) != len(want) {
				t.Errorf("body has keys %v, want exactly %v", body, want)
			}
			for k, v := range want {
				if body[k] != v {
					t.Errorf("body[%q] = %v, want %v", k, body[k], v)
				}
			}
		})
	}
}

func TestHealthExactBody(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(Config{Version: "dev", MaxConcurrentUploads: 4}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if got, want := rec.Body.String(), `{"status":"ok","version":"dev"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestHealthMethodNotAllowed(t *testing.T) {
	h := NewHandler(Config{Version: "dev", MaxConcurrentUploads: 4})
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(method, "/api/health", nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
			if got := rec.Header().Get("Allow"); got == "" {
				t.Error("Allow header missing on 405")
			}
		})
	}
}

func TestUnknownRouteIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(Config{Version: "dev", MaxConcurrentUploads: 4}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
