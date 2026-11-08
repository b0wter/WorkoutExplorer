package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
)

type fakeOwner struct {
	calls []string
	token string
	err   error
	patch lifecycle.SettingsPatch
	// F-36
	noFrame bool
	key     string
	read    int
}

func (f *fakeOwner) rec() lifecycle.Record {
	c := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return lifecycle.Record{PublicID: strings.Repeat("p", 22), Title: "<b>", CreatedAt: c, ExpiresAt: c.Add(7 * 24 * time.Hour), Visibility: lifecycle.Draft}
}
func (f *fakeOwner) Get(_ context.Context, id, tok string) (lifecycle.Record, error) {
	f.calls, f.token = append(f.calls, "get"), tok
	return f.rec(), f.err
}
func (f *fakeOwner) Update(_ context.Context, id, tok string, p lifecycle.SettingsPatch) (lifecycle.Record, error) {
	f.calls, f.token, f.patch = append(f.calls, "update"), tok, p
	return f.rec(), f.err
}
func (f *fakeOwner) OpenDraft(context.Context, string, string) (lifecycle.Download, error) {
	f.calls = append(f.calls, "draft")
	return lifecycle.Download{Body: io.NopCloser(strings.NewReader("DRAFT")), Size: 5}, f.err
}
func (f *fakeOwner) OpenOriginal(context.Context, string, string) (lifecycle.Download, error) {
	f.calls = append(f.calls, "original")
	return lifecycle.Download{Body: io.NopCloser(strings.NewReader("ORIG")), Size: 4, Name: "workout-2026-06-01.fit"}, f.err
}
func (f *fakeOwner) DeleteByOwner(context.Context, string, string) error {
	f.calls = append(f.calls, "delete")
	return f.err
}
func (f *fakeOwner) Publish(_ context.Context, _, tok string) (lifecycle.Record, error) {
	f.calls, f.token = append(f.calls, "publish"), tok
	r := f.rec()
	r.Visibility = lifecycle.Unlisted
	return r, f.err
}

const pid = "0123456789abcdefghijkA"

func ownerDo(h http.Handler, method, target, auth, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

const wantNotFound = `{"error":"not_found","message":"This workout has expired or is not available."}`

func TestOwnerQueryStringIsNotFound(t *testing.T) {
	o := &fakeOwner{}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})
	tok := "Bearer " + strings.Repeat("t", 22)
	for _, e := range []struct{ method, path string }{
		{"GET", ""}, {"PATCH", ""}, {"DELETE", ""}, {"GET", "/draft.bin"}, {"GET", "/original.fit"}, {"POST", "/publish"},
		{"GET", "/preview-map"}, {"PUT", "/preview-map/0123456789abcdef"},
	} {
		for _, q := range []string{"?x=1", "?", "?token=" + strings.Repeat("t", 22)} {
			rec := ownerDo(h, e.method, "/api/workouts/"+pid+e.path+q, tok, "{}")
			if rec.Code != 404 || rec.Body.String() != wantNotFound || rec.Header().Get("Cache-Control") != "no-store" ||
				rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("WWW-Authenticate") != "" {
				t.Errorf("%s %s%s: %d %s", e.method, e.path, q, rec.Code, rec.Body)
			}
		}
	}
	if len(o.calls) != 0 {
		t.Errorf("service called: %v", o.calls)
	}
}

func TestBearer(t *testing.T) {
	tok := strings.Repeat("t", 22)
	tests := []struct {
		name   string
		header []string
		want   string
	}{
		{"plain", []string{"Bearer " + tok}, tok},
		{"scheme case-insensitive", []string{"bEaReR " + tok}, tok},
		{"none", nil, ""},
		{"other scheme", []string{"Basic " + tok}, ""},
		{"two headers", []string{"Bearer " + tok, "Bearer " + tok}, ""},
		{"no value", []string{"Bearer"}, ""},
		{"bare token", []string{tok}, ""},
	}
	for _, tc := range tests {
		r := httptest.NewRequest("GET", "/", nil)
		for _, v := range tc.header {
			r.Header.Add("Authorization", v)
		}
		if got := bearer(r); got != tc.want {
			t.Errorf("%s: %q", tc.name, got)
		}
	}
	// Only the header counts: a token in the query reaches nobody (see above)
	// and an X-header or cookie is ignored.
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Edit-Token", tok)
	r.AddCookie(&http.Cookie{Name: "token", Value: tok})
	if bearer(r) != "" {
		t.Error("token taken from elsewhere")
	}
}

func TestOwnerResponses(t *testing.T) {
	o := &fakeOwner{}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})
	auth := "Bearer " + strings.Repeat("t", 22)
	base := "/api/workouts/" + pid

	rec := ownerDo(h, "GET", base, auth, "")
	if rec.Code != 200 || o.token != strings.Repeat("t", 22) ||
		!strings.Contains(rec.Body.String(), `"title":"\u003cb\u003e"`) || !strings.Contains(rec.Body.String(), `"lifetime_days":7`) ||
		!strings.Contains(rec.Body.String(), `"created_at":"2026-10-01T00:00:00Z"`) || !strings.Contains(rec.Body.String(), `"data_end_s":null`) ||
		!strings.Contains(rec.Body.String(), `"view_url":"/w/pppppppppppppppppppppp"`) ||
		!strings.Contains(rec.Body.String(), `"hide_ends_ranges":[]`) {
		t.Errorf("get: %d %s", rec.Code, rec.Body)
	}

	rec = ownerDo(h, "GET", base+"/original.fit", auth, "")
	if rec.Code != 200 || rec.Body.String() != "ORIG" || rec.Header().Get("Content-Length") != "4" ||
		rec.Header().Get("Content-Disposition") != `attachment; filename="workout-2026-06-01.fit"` ||
		rec.Header().Get("Content-Type") != "application/octet-stream" || rec.Header().Get("Cache-Control") != "no-store" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("original: %d %v", rec.Code, rec.Header())
	}
	rec = ownerDo(h, "GET", base+"/draft.bin", auth, "")
	if rec.Code != 200 || rec.Body.String() != "DRAFT" || rec.Header().Get("Content-Disposition") != "" {
		t.Errorf("draft: %d %v", rec.Code, rec.Header())
	}
	rec = ownerDo(h, "DELETE", base, auth, "")
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Errorf("delete: %d %q", rec.Code, rec.Body)
	}

	// Other methods get the mux's 405, the same for every ID.
	for _, m := range []string{"POST", "PUT"} {
		if rec := ownerDo(h, m, base, auth, ""); rec.Code != 405 {
			t.Errorf("%s: %d", m, rec.Code)
		}
	}
	if rec := ownerDo(h, "POST", base+"/original.fit", auth, ""); rec.Code != 405 {
		t.Errorf("POST file: %d", rec.Code)
	}

	// Service answers map to the three responses.
	o.err = lifecycle.ErrNotFound
	for _, p := range []string{"", "/draft.bin", "/original.fit"} {
		if rec := ownerDo(h, "GET", base+p, auth, ""); rec.Code != 404 || rec.Body.String() != wantNotFound {
			t.Errorf("not found %s: %d %s", p, rec.Code, rec.Body)
		}
	}
	if rec := ownerDo(h, "DELETE", base, auth, ""); rec.Code != 404 || rec.Body.String() != wantNotFound {
		t.Errorf("delete not found: %d", rec.Code)
	}
	o.err = &lifecycle.ServerError{Stage: "x", Err: errors.New("boom")}
	rec = ownerDo(h, "GET", base, auth, "")
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"error":"server_error"`) || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("500: %d %s", rec.Code, rec.Body)
	}
}

func TestPatchBody(t *testing.T) {
	o := &fakeOwner{}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})
	auth := "Bearer " + strings.Repeat("t", 22)
	url := "/api/workouts/" + pid

	bad := []string{
		``, `[]`, `"x"`, `null`, `{`, `{} {}`, `{}x`,
		`{"unknown":1}`, `{"created_at":"2026-01-01T00:00:00Z"}`, `{"expires_at":"x"}`, `{"view_url":"/x"}`, `{"data_end_s":1}`,
		`{"hide_ends_ranges":[]}`, `{"hide_ends_ranges":[[0,1]]}`, `{"title":"x","hide_ends_ranges":null}`,
		`{"title":null}`, `{"title":null,"title":"x"}`, `{"title":"a","title":"b"}`, `{"title":1}`, `{"hide_ends":"yes"}`, `{"allow_download":1}`, `{"lifetime_days":7.5}`,
		`{"lifetime_days":"7"}`, `{"visibility":3}`, `{"description":["a"]}`,
		`{"visibility":"unlisted"}`, `{"visibility":"draft"}`, `{"visibility":"private"}`, `{"title":"x","visibility":"unlisted"}`, // read-only (O-6)
		`{"title":"` + strings.Repeat("a", maxPatchBytes) + `"}`,
	}
	for _, b := range bad {
		rec := ownerDo(h, "PATCH", url, auth, b)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"error":"invalid_settings"`) {
			t.Errorf("%q: %d %s", b[:min(len(b), 40)], rec.Code, rec.Body)
		}
	}
	if len(o.calls) != len(bad) { // only the token check (Get) ran, never Update
		for _, c := range o.calls {
			if c == "update" {
				t.Fatal("update called for a bad body")
			}
		}
	}

	o.calls = nil
	rec := ownerDo(h, "PATCH", url, auth, `{"title":"T","description":"D","hide_ends":true,"allow_download":false,"lifetime_days":30}`)
	p := o.patch
	if rec.Code != 200 || *p.Title != "T" || *p.Description != "D" || !*p.HideEnds || *p.AllowDownload || *p.LifetimeDays != 30 {
		t.Errorf("full: %d %s", rec.Code, rec.Body)
	}
	rec = ownerDo(h, "PATCH", url, auth, ` {} `)
	if rec.Code != 200 || o.patch != (lifecycle.SettingsPatch{}) {
		t.Errorf("empty: %d", rec.Code)
	}

	// A service refusal is 400 with its sentence; a stranger never sees body rules.
	o.err = &lifecycle.InvalidSettingsError{Message: "No."}
	rec = ownerDo(h, "PATCH", url, auth, `{"title":"x"}`)
	if rec.Code != 400 || rec.Body.String() != `{"error":"invalid_settings","message":"No."}` {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	o.err = lifecycle.ErrNotFound
	if rec := ownerDo(h, "PATCH", url, "", `garbage`); rec.Code != 404 || rec.Body.String() != wantNotFound {
		t.Errorf("stranger: %d %s", rec.Code, rec.Body)
	}
}

// TestPublishEndpoint (F-15 P-1): the settings object, the not-found and
// server-error answers, no body read, other methods 405.
func TestPublishEndpoint(t *testing.T) {
	o := &fakeOwner{}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})
	auth := "Bearer " + strings.Repeat("t", 22)
	url := "/api/workouts/" + pid + "/publish"

	body := &countingReader{r: strings.NewReader("ignored")}
	r := httptest.NewRequest("POST", url, body)
	r.Header.Set("Authorization", auth)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"visibility":"unlisted"`) || o.token != strings.Repeat("t", 22) {
		t.Errorf("publish: %d %s", rec.Code, rec.Body)
	}
	if body.n.Load() != 0 {
		t.Errorf("body read: %d bytes", body.n.Load())
	}
	for _, m := range []string{"GET", "PUT", "PATCH", "DELETE"} {
		if rec := ownerDo(h, m, url, auth, ""); rec.Code != 405 {
			t.Errorf("%s: %d", m, rec.Code)
		}
	}
	o.err = lifecycle.ErrNotFound
	if rec := ownerDo(h, "POST", url, "", ""); rec.Code != 404 || rec.Body.String() != wantNotFound {
		t.Errorf("not found: %d %s", rec.Code, rec.Body)
	}
	o.err = &lifecycle.ServerError{Stage: "hook", Err: errors.New("boom")}
	if rec := ownerDo(h, "POST", url, auth, ""); rec.Code != 500 || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("500: %d %s", rec.Code, rec.Body)
	}
}
