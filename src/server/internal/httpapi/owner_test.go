package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
)

func (o *fakeOwner) View(_ context.Context, r lifecycle.Record) lifecycle.OwnerView {
	return lifecycle.OwnerView{Suggested: "Morning ride near Köln", Steps: r.HidingSteps}
}

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
		!strings.Contains(rec.Body.String(), `"hiding_steps":null`) || !strings.Contains(rec.Body.String(), `"hide_start_m":0,"hide_end_m":0`) || strings.Contains(rec.Body.String(), `hide_ends`) || !strings.Contains(rec.Body.String(), `"suggested_title":"Morning ride near Köln"`) {
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
		`{"hide_ends_ranges":[]}`, `{"suggested_title":"x"}`, `{"hide_ends_ranges":[[0,1]]}`, `{"title":"x","hide_ends_ranges":null}`,
		`{"title":null}`, `{"title":null,"title":"x"}`, `{"title":"a","title":"b"}`, `{"title":1}`, `{"hide_ends":true}`, `{"hide_ends":false}`, `{"hiding_steps":null}`, `{"hide_start_m":300.5}`, `{"hide_start_m":"300"}`, `{"hide_end_m":null}`, `{"hide_end_m":true}`, `{"title":"x","hide_end_m":3e2}`, // F-55 HR-4 `{"allow_download":false}`, `{"title":"x","allow_download":true}`, `{"lifetime_days":7.5}`,
		`{"lifetime_days":"7"}`, `{"visibility":3}`, `{"description":["a"]}`,
		`{"visibility":"unlisted"}`, `{"visibility":"draft"}`, `{"visibility":"private"}`, `{"title":"x","visibility":"unlisted"}`, // read-only (O-6)
		`{"trimmed":null}`, `{"trim_start_at":"x"}`, `{"trim_end_at":1}`, `{"trim_start_at":"2026-06-01T06:00:00.5Z"}`, // F-47
		`{"pins":null}`, `{"pins":{}}`, `{"pins":[null]}`, `{"pins":[{}]}`, `{"pins":[1]}`, // F-56 PN-2
		`{"pins":[{"type":"danger","title":"t","note":"","start_at":"2026-06-01T06:00:00Z"}]}`, // end_at missing
		`{"pins":[{"type":"danger","title":"t","note":"","start_at":"2026-06-01T06:00:00Z","end_at":null,"x":1}]}`,
		`{"pins":[{"type":1,"title":"t","note":"","start_at":"2026-06-01T06:00:00Z","end_at":null}]}`,
		`{"pins":[{"type":"danger","title":null,"note":"","start_at":"2026-06-01T06:00:00Z","end_at":null}]}`,
		`{"pins":[{"type":"danger","title":"t","note":"","start_at":"2026-06-01T06:00:00.5Z","end_at":null}]}`,
		`{"pins":[{"type":"danger","title":"t","note":"","start_at":"x","end_at":null}]}`,
		`{"pins":[{"type":"danger","title":"t","note":"","start_at":"2026-06-01T06:00:00Z","end_at":5}]}`,
		`{"zones_max_heart_rate":190.5}`, `{"zones_max_heart_rate":"190"}`, `{"zones_threshold_power":1e2}`, `{"zones_threshold_power":true}`, // F-49
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

	if rec := ownerDo(h, "PATCH", url, auth, `{"hide_start_m":"300"}`); !strings.Contains(rec.Body.String(), lifecycle.MsgBadHiding) {
		t.Errorf("a string as a radius: %s", rec.Body)
	}
	if rec := ownerDo(h, "PATCH", url, auth, `{"trim_start_at":12345}`); !strings.Contains(rec.Body.String(), "trim_start_at") {
		t.Errorf("a number as a trim time: %s", rec.Body)
	}

	o.calls = nil
	rec := ownerDo(h, "PATCH", url, auth, `{"title":"T","description":"D","hide_start_m":50,"hide_end_m":2000,"lifetime_days":30}`)
	p := o.patch
	if rec.Code != 200 || *p.Title != "T" || *p.Description != "D" || *p.HideStartM != 50 || *p.HideEndM != 2000 || *p.LifetimeDays != 30 {
		t.Errorf("full: %d %s", rec.Code, rec.Body)
	}
	rec = ownerDo(h, "PATCH", url, auth, `{"trim_start_at":"2026-06-01T08:00:00+02:00","trim_end_at":null}`)
	if rec.Code != 200 || !o.patch.TrimStart.Set || !o.patch.TrimStart.At.Equal(time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)) || !o.patch.TrimEnd.Set || o.patch.TrimEnd.At != nil {
		t.Errorf("trim: %d %s %+v", rec.Code, rec.Body, o.patch)
	}
	rec = ownerDo(h, "PATCH", url, auth, `{"zones_max_heart_rate":190,"zones_threshold_power":null}`)
	if z := o.patch; rec.Code != 200 || !z.ZonesMaxHeartRate.Set || *z.ZonesMaxHeartRate.V != 190 || !z.ZonesThresholdPower.Set || z.ZonesThresholdPower.V != nil {
		t.Errorf("zones: %d %s %+v", rec.Code, rec.Body, o.patch)
	}
	rec = ownerDo(h, "PATCH", url, auth, ` {} `)
	if rec.Code != 200 || !reflect.DeepEqual(o.patch, lifecycle.SettingsPatch{}) {
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

// TestPatchPins (F-56 PN-2): a valid set parses with every field; [] is a
// clear; the settings object answers with the whole stored set; the body cap
// holds the largest valid body.
func TestPatchPins(t *testing.T) {
	p, msg := parsePatch([]byte(`{"pins":[{"type":"danger","title":"<b>","note":"a\nb","start_at":"2026-06-01T08:00:00+02:00","end_at":null},
		{"type":"difficult_section","title":"","note":"","start_at":"2026-06-01T06:00:00Z","end_at":"2026-06-01T06:05:00Z"}]}`))
	if msg != "" || !p.Pins.Set || len(p.Pins.V) != 2 || p.Pins.V[0].Title != "<b>" || p.Pins.V[0].Note != "a\nb" ||
		!p.Pins.V[0].StartAt.Equal(time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC)) || p.Pins.V[0].EndAt != nil || p.Pins.V[1].EndAt == nil {
		t.Fatalf("%q %+v", msg, p.Pins)
	}
	if p, msg = parsePatch([]byte(`{"pins":[]}`)); msg != "" || !p.Pins.Set || len(p.Pins.V) != 0 {
		t.Fatalf("%q %+v", msg, p.Pins)
	}
	e := time.Date(2026, 6, 1, 6, 5, 0, 0, time.UTC)
	r := (&fakeOwner{}).rec()
	r.Pins = []lifecycle.Pin{{Type: "note", Title: "x", StartAt: e.Add(-time.Minute)}, {Type: "danger", StartAt: e.Add(-time.Hour), EndAt: &e}}
	out, _ := json.Marshal(settingsOf(r, lifecycle.OwnerView{}))
	if !strings.Contains(string(out), `"pins":[{"type":"note","title":"x","note":"","start_at":"2026-06-01T06:04:00Z","end_at":null},{"type":"danger","title":"","note":"","start_at":"2026-06-01T05:05:00Z","end_at":"2026-06-01T06:05:00Z"}]`) {
		t.Errorf("%s", out)
	}
	r.Pins = nil
	if out, _ = json.Marshal(settingsOf(r, lifecycle.OwnerView{})); !strings.Contains(string(out), `"pins":[]`) {
		t.Errorf("no pins is not []: %s", out)
	}
	// 50 pins of 60 + 500 code points, each a surrogate pair escaped (12 bytes).
	pair := strings.Repeat(`😀`, 1)
	one := `{"type":"difficult_section","title":"` + strings.Repeat(pair, 60) + `","note":"` + strings.Repeat(pair, 500) + `","start_at":"2026-06-01T06:00:00Z","end_at":"2026-06-01T06:05:00Z"}`
	body := `{"title":"` + strings.Repeat(pair, 100) + `","description":"` + strings.Repeat(pair, 1000) + `","pins":[` + strings.TrimSuffix(strings.Repeat(one+",", 50), ",") + `]}`
	if len(body) > maxPatchBytes {
		t.Fatalf("largest valid body %d exceeds the cap %d", len(body), maxPatchBytes)
	}
	o := &fakeOwner{}
	h := NewHandler(Config{MaxConcurrentUploads: 1, Owner: o})
	if rec := ownerDo(h, "PATCH", "/api/workouts/"+pid, "Bearer "+strings.Repeat("t", 22), body); rec.Code != 200 || len(o.patch.Pins.V) != 50 {
		t.Errorf("%d %.200s", rec.Code, rec.Body)
	}
}
