package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// F-12 end to end through newApp (architect/F-12-owner-api.md).

func (a *app) owner(method, path, token, body string) *httptest.ResponseRecorder {
	a.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, r)
	return rec
}

// sql runs a statement on the database, standing in for F-15's publish.
func (a *app) sql(q string, args ...any) {
	a.t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(a.dir, "workouts.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		a.t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(q, args...); err != nil {
		a.t.Fatal(err)
	}
}

func internalID(a *app) string { return a.rows()[0]["id"].(string) }

const ownerNotFound = `{"error":"not_found","message":"This workout has expired or is not available."}`

func ownerPaths(pub string) map[string][]string {
	p := "/api/workouts/" + pub
	return map[string][]string{
		"GET":    {p, p + "/draft.bin", p + "/original.fit"},
		"PATCH":  {p},
		"DELETE": {p},
	}
}

// TestOwnerNotFound (AC 1).
func TestOwnerNotFound(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	other := strings.Repeat("z", 22)
	wrong := token[:21] + map[bool]string{true: "0", false: "1"}[token[21] != '0']
	var first http.Header

	for method, paths := range ownerPaths(pub) {
		for _, path := range paths {
			for _, c := range []struct{ name, path, token string }{
				{"no token", path, ""},
				{"wrong token", path, wrong},
				{"token of another shape", path, "short"},
				{"valid token in the query", path + "?token=" + token, ""},
				{"valid token in the query and header", path + "?token=" + token, token},
				{"valid header with any query", path + "?x=1", token},
				{"valid header with an empty query", path + "?", token},
				{"unknown workout", strings.Replace(path, pub, other, 1), token},
				{"internal id as handle", strings.Replace(path, pub, internalID(a), 1), token},
			} {
				rec := a.owner(method, c.path, c.token, `{"title":"x"}`)
				if rec.Code != 404 || rec.Body.String() != ownerNotFound || rec.Header().Get("Content-Type") != "application/json" {
					t.Errorf("%s %s (%s): %d %s", method, path, c.name, rec.Code, rec.Body)
				}
				// Whole header set, except Date (QA-12-r1-6).
				hd := rec.Header().Clone()
				hd.Del("Date")
				if first == nil {
					first = hd
				} else if !reflect.DeepEqual(first, hd) {
					t.Errorf("%s %s (%s): headers %v differ from %v", method, path, c.name, hd, first)
				}
			}
		}
	}
	// Nothing changed or went away.
	if rows := a.rows(); len(rows) != 1 || rows[0]["title"] != "" {
		t.Errorf("rows %v", rows)
	}
	// Expired but not yet swept: not found for the owner, to the second.
	a.sql("UPDATE workouts SET created_at = created_at - 864000, expires_at = ? ", time.Now().Unix()-1)
	if rec := a.owner("GET", "/api/workouts/"+pub, token, ""); rec.Code != 404 || rec.Body.String() != ownerNotFound {
		t.Errorf("expired: %d %s", rec.Code, rec.Body)
	}
}

// TestOwnerLifetime (AC 2; the past and the 31st day are also in the
// lifecycle table test, where the clock is injected).
func TestOwnerLifetime(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	url := "/api/workouts/" + pub
	for _, c := range []struct {
		body string
		want int
	}{{`{"lifetime_days":31}`, 400}, {`{"lifetime_days":0}`, 400}, {`{"lifetime_days":-3}`, 400}, {`{"lifetime_days":30}`, 200}, {`{"lifetime_days":1}`, 200}} {
		rec := a.owner("PATCH", url, token, c.body)
		if rec.Code != c.want {
			t.Errorf("%s: %d %s", c.body, rec.Code, rec.Body)
		}
	}
	rec := a.owner("PATCH", url, token, `{"lifetime_days":30}`)
	var got struct {
		LifetimeDays int `json:"lifetime_days"`
	}
	json.Unmarshal(rec.Body.Bytes(), &got)
	row := a.rows()[0]
	if got.LifetimeDays != 30 || row["expires_at"].(int64)-row["created_at"].(int64) != 30*86400 {
		t.Errorf("30 days: %s, row %v", rec.Body, row)
	}
}

// TestOwnerPlainText (AC 3) and TestOwnerLengths (AC 7).
func TestOwnerPlainText(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	url := "/api/workouts/" + pub
	rec := a.owner("PATCH", url, token, `{"title":"  <script>alert(1)</script> ","description":"<img src=x onerror=1>\nline 2"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body)
	}
	for _, rec := range []*httptest.ResponseRecorder{rec, a.owner("GET", url, token, "")} {
		var got map[string]any
		json.Unmarshal(rec.Body.Bytes(), &got)
		if got["title"] != "<script>alert(1)</script>" || got["description"] != "<img src=x onerror=1>\nline 2" {
			t.Errorf("%v", got)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("content type %q", ct)
		}
	}
	row := a.rows()[0]
	if row["title"] != "<script>alert(1)</script>" {
		t.Errorf("stored %q", row["title"])
	}
}

func TestOwnerLengths(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	url := "/api/workouts/" + pub
	body := func(k, s string) string { b, _ := json.Marshal(map[string]string{k: s}); return string(b) }
	for _, c := range []struct {
		field, unit string
		n           int
		want        int
	}{
		{"title", "a", 100, 200}, {"title", "a", 101, 400},
		{"title", "ä", 100, 200}, {"title", "ä", 101, 400},
		{"title", "🚴", 100, 200}, {"title", "🚴", 101, 400},
		{"description", "a", 1000, 200}, {"description", "a", 1001, 400},
		{"description", "🚴", 1000, 200}, {"description", "🚴", 1001, 400},
	} {
		if rec := a.owner("PATCH", url, token, body(c.field, strings.Repeat(c.unit, c.n))); rec.Code != c.want {
			t.Errorf("%s %q x %d: %d", c.field, c.unit, c.n, rec.Code)
		}
	}
	// The largest valid body fits the body cap: 1,100 non-BMP code points, escaped.
	b, _ := json.Marshal(map[string]string{"title": strings.Repeat("🚴", 100), "description": strings.Repeat("🚴", 1000)})
	esc := strings.NewReplacer("🚴", `🚴`).Replace(string(b))
	if rec := a.owner("PATCH", url, token, esc); rec.Code != 200 {
		t.Errorf("largest body: %d", rec.Code)
	}
}

// TestOwnerSettings: the settings object, flags, visibility rules.
func TestOwnerSettings(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	url := "/api/workouts/" + pub
	rec := a.owner("GET", url, token, "")
	var s map[string]any
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &s) != nil {
		t.Fatal(rec.Code, rec.Body)
	}
	if s["visibility"] != "draft" || s["lifetime_days"] != float64(7) || s["hide_ends"] != false || s["allow_download"] != false ||
		s["view_url"] != "/w/"+pub || s["title"] != "" || len(s) != 11 || s["hide_ends_ranges"] == nil {
		t.Errorf("%v", s)
	}
	if _, err := time.Parse(time.RFC3339, s["expires_at"].(string)); err != nil {
		t.Error(err)
	}
	if rec := a.owner("PATCH", url, token, `{"hide_ends":true,"allow_download":true}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if row := a.rows()[0]; row["hide_ends"] != int64(1) || row["allow_download"] != int64(1) {
		t.Errorf("%v", row)
	}
	// Visibility is read-only (O-6): only publish changes it.
	for _, st := range []string{"draft", "unlisted"} {
		if st == "unlisted" {
			a.sql("UPDATE workouts SET visibility = 'unlisted', published_at = created_at")
		}
		for _, b := range []string{`{"visibility":"private"}`, `{"visibility":"unlisted"}`, `{"visibility":"draft"}`, `{"title":"v","visibility":"public"}`} {
			if rec := a.owner("PATCH", url, token, b); rec.Code != 400 {
				t.Errorf("%s %s: %d", st, b, rec.Code)
			}
		}
	}
	if row := a.rows()[0]; row["visibility"] != "unlisted" || row["published_at"] == nil {
		t.Errorf("%v", row)
	}
}

// TestOwnerFiles (AC 4).
func TestOwnerFiles(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	id := internalID(a)
	url := "/api/workouts/" + pub

	rec := a.owner("GET", url+"/original.fit", token, "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), a.privateFile(id, "original.fit")) {
		t.Fatalf("original: %d", rec.Code)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, `attachment; filename="workout-20`) || !strings.HasSuffix(cd, `.fit"`) || strings.Contains(cd, "ride-30min") {
		t.Errorf("disposition %q", cd)
	}
	if rec.Header().Get("Content-Length") != itoa(rec.Body.Len()) || rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("%v", rec.Header())
	}
	rec = a.owner("GET", url+"/draft.bin", token, "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), a.privateFile(id, "draft.bin")) || !bytes.HasPrefix(rec.Body.Bytes(), []byte("WEXD")) {
		t.Errorf("draft: %d", rec.Code)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// TestOwnerDelete (AC 5, as limited by OQ-60: the row, private/{id},
// public/{public_id} (F-15 AC 9), the storage total and the owner endpoints;
// reports come with F-27, which adds them to dataHolders).
func TestOwnerDelete(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	before := a.st.Used()
	pub, token, _ := a.upload("ride-30min.fit")
	other, otherToken, _ := a.upload("ride-30min.fit")
	id := internalID(&app{t: t, dir: dir})
	for _, r := range a.rows() {
		if r["public_id"] == pub {
			id = r["id"].(string)
		}
	}
	folder := diskUsage(t, filepath.Join(dir, "private", id))
	if a.st.Used()-before != 2*folder {
		t.Fatalf("total %d, folders %d", a.st.Used()-before, folder)
	}
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	if len(a.entries(filepath.Join("public", pub))) != 4 {
		t.Fatal("not published")
	}

	if rec := a.owner("DELETE", "/api/workouts/"+pub, token, ""); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}

	// dataHolders is the one list of places that hold workout data.
	dataHolders := []func() bool{
		func() bool { // the row
			for _, r := range a.rows() {
				if r["id"] == id {
					return true
				}
			}
			return false
		},
		func() bool { // private/{id}
			for _, e := range a.entries("private") {
				if e == id {
					return true
				}
			}
			return false
		},
		func() bool { // public/{public_id} (F-15 AC 9)
			for _, e := range a.entries("public") {
				if e == pub {
					return true
				}
			}
			return false
		},
	}
	for i, holds := range dataHolders {
		if holds() {
			t.Errorf("holder %d still holds the workout", i)
		}
	}
	if got := a.st.Used() - before; got != folder {
		t.Errorf("storage total dropped to %d above start, want %d", got, folder)
	}
	paths := ownerPaths(pub)
	paths["POST"] = []string{"/api/workouts/" + pub + "/publish"}
	for method, paths := range paths {
		for _, p := range paths {
			if rec := a.owner(method, p, token, `{}`); rec.Code != 404 || rec.Body.String() != ownerNotFound {
				t.Errorf("after delete %s %s: %d %s", method, p, rec.Code, rec.Body)
			}
		}
	}
	// The other workout is untouched.
	if rec := a.owner("GET", "/api/workouts/"+other, otherToken, ""); rec.Code != 200 {
		t.Errorf("other: %d", rec.Code)
	}
	if !strings.Contains(a.logs.String(), `"msg":"workout deleted"`) || strings.Contains(a.logs.String(), pub) || strings.Contains(a.logs.String(), token) {
		t.Errorf("log: %s", a.logs.String())
	}
}

// TestNoPublicURLServesPrivateFiles (AC 6, Go side): the handler answers
// none of these with either file's bytes, before and after publishing; after
// publishing, no file under public/ has the bytes of original.fit (F-15 AC 10,
// OQ-69; with hiding off the public data file is draft.bin's bytes).
func TestNoPublicURLServesPrivateFiles(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	id := internalID(a)
	orig, draft := a.privateFile(id, "original.fit"), a.privateFile(id, "draft.bin")
	for _, published := range []bool{false, true} {
		if published {
			if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
				t.Fatalf("publish: %d", rec.Code)
			}
			noOriginalUnderPublic(t, a.dir, orig)
		}
		guessPaths(t, a, pub, id, token, orig, draft)
	}
	// The private modes stay; public/ holds only the published folder.
	if got := a.entries("public"); len(got) != 1 || got[0] != pub {
		t.Errorf("public/ = %v", got)
	}
}

// noOriginalUnderPublic: no file under public/ has (or contains) the bytes of original.fit.
func noOriginalUnderPublic(t *testing.T, dir string, orig []byte) {
	t.Helper()
	n := 0
	err := filepath.WalkDir(filepath.Join(dir, "public"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		n++
		b, err := os.ReadFile(p)
		if bytes.Contains(b, orig) {
			t.Errorf("%s has the bytes of original.fit", filepath.Base(p))
		}
		return err
	})
	if err != nil || n == 0 {
		t.Fatalf("walk: %v, %d files", err, n)
	}
}

// guessedPaths is the F-12 AC 6 list of URLs that must never serve a private
// file; the F-22 Caddy check sends it through Caddy too.
func guessedPaths(pub, id, token string) []string {
	var paths []string
	for _, base := range []string{"/", "/w/" + pub + "/", "/w/" + pub, "/private/" + id + "/", "/private/", "/api/workouts/" + pub + "/",
		"/api/workouts/" + id + "/", "/public/" + pub + "/", "/edit/" + pub + "/", "/api/"} {
		for _, f := range []string{"", "original.fit", "draft.bin"} {
			paths = append(paths, base+f)
		}
	}
	paths = append(paths,
		"/api/workouts/"+pub+"/original.fit?token="+token, "/api/workouts/"+pub+"/draft.bin?token="+token,
		"/api/workouts/"+pub+"/%2e%2e/"+pub+"/original.fit", "/api/workouts/%2e%2e/%2e%2e/private/"+id+"/original.fit",
		"/%2e%2e/private/"+id+"/draft.bin", "/w/%2e%2e/private/"+id+"/original.fit", "/w/"+pub+"/..%2fprivate%2f"+id+"%2foriginal.fit",
		"/private/"+id+"/original.fit", "/private/"+id+"/draft.bin", "//private/"+id+"/original.fit",
		"/api/workouts/"+pub+"/original.fit/", "/api/workouts/"+pub+"/ORIGINAL.FIT", "/api/workouts/"+pub+"/original.fit%00")
	return paths
}

func guessPaths(t *testing.T, a *app, pub, id, token string, orig, draft []byte) {
	t.Helper()
	for _, p := range guessedPaths(pub, id, token) {
		for _, m := range []string{"GET", "HEAD"} {
			r := httptest.NewRequest(m, "http://example.com"+p, nil)
			rec := httptest.NewRecorder()
			a.h.ServeHTTP(rec, r)
			b := rec.Body.Bytes()
			if rec.Code/100 == 2 && (bytes.Contains(b, orig) || bytes.Contains(b, draft)) {
				t.Errorf("%s %s serves a private file", m, p)
			}
			if rec.Code == http.StatusOK && len(b) > 0 && bytes.Contains(b, []byte("WEXD")) {
				t.Errorf("%s %s serves a data file", m, p)
			}
		}
	}
}
