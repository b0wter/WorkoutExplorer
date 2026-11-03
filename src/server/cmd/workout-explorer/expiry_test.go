package main

import (
	"bytes"
	"context"
	"database/sql"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// F-21 end to end through newApp (architect/F-21-lifetime-and-expiry.md,
// Verification). The server runs on the real clock, so the tests move the
// row's times instead.

const day = 86400

func idOf(a *app, pub string) string {
	a.t.Helper()
	for _, r := range a.rows() {
		if r["public_id"] == pub {
			return r["id"].(string)
		}
	}
	a.t.Fatalf("no row for the workout")
	return ""
}

// setTimes moves a row: created_at and expires_at as given (unix seconds),
// published_at (if published) to created_at + publishedAfter.
func (a *app) setTimes(pub string, created, expires, publishedAfter int64) {
	a.t.Helper()
	a.sql(`UPDATE workouts SET created_at = ?1, expires_at = ?2,
		published_at = CASE WHEN published_at IS NULL THEN NULL ELSE ?1 + ?3 END WHERE public_id = ?4`,
		created, expires, publishedAfter, pub)
}

// dataLeft is the complete check of F-21 X-9 (AC 8): every table of the
// database and every entry of the data directory is either classified and
// checked for the workout, or reported. The database files are also searched
// byte for byte for id, pub and the workout's texts in more, so a deleted row
// left in a free page or the WAL is found (QA-21-r1-1). Reports (F-27) are
// checked; phase 2 adds pins.
func dataLeft(t *testing.T, dir, id, pub string, more ...string) []string {
	t.Helper()
	var left []string
	for _, n := range []string{"workouts.db", "workouts.db-wal", "workouts.db-shm"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		for _, s := range append([]string{id, pub}, more...) {
			if bytes.Contains(b, []byte(s)) {
				left = append(left, "bytes in "+n)
				break
			}
		}
	}
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "workouts.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tables, err := db.Query("SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var n string
		if err := tables.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	tables.Close()
	for _, n := range names {
		switch n {
		case "workouts":
			var c int
			if err := db.QueryRow("SELECT count(*) FROM workouts WHERE id = ? OR public_id = ?", id, pub).Scan(&c); err != nil {
				t.Fatal(err)
			}
			if c > 0 {
				left = append(left, "table workouts")
			}
		case "reports": // F-27: deleted with the row (ON DELETE CASCADE)
			var c int
			if err := db.QueryRow("SELECT count(*) FROM reports WHERE workout_id = ?", id).Scan(&c); err != nil {
				t.Fatal(err)
			}
			if c > 0 {
				left = append(left, "table reports")
			}
		default:
			left = append(left, "unclassified table "+n)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	exists := func(p string) bool { _, err := os.Lstat(p); return err == nil }
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		switch e.Name() {
		case "workouts.db", "workouts.db-wal", "workouts.db-shm": // the tables and bytes above
		case "public.lock": // empty, ADR-031
		case "private":
			if exists(filepath.Join(p, id)) {
				left = append(left, "private/{id}")
			}
		case "public":
			if exists(filepath.Join(p, pub)) {
				left = append(left, "public/{public_id}")
			}
		case "tmp": // staging: empty when no upload is in flight
			if es, err := os.ReadDir(p); err != nil || len(es) > 0 {
				left = append(left, "tmp/")
			}
		default:
			left = append(left, "unclassified entry "+e.Name())
		}
	}
	return left
}

// checkTotal: the storage total is base plus the workout files on disk (F-10;
// base is the total before the first upload, the database measured at start).
func checkTotal(t *testing.T, a *storageApp, base int64) {
	t.Helper()
	if want := base + diskUsage(t, filepath.Join(a.dir, "private")) + diskUsage(t, filepath.Join(a.dir, "public")); a.st.Used() != want {
		t.Errorf("storage total %d, want %d", a.st.Used(), want)
	}
}

// TestExpiryEndToEnd (AC 1, AC 8; X-4): a published workout at its deadline
// is gone after one pass, row and both folders; the generic-page condition
// (no index.html) holds; the total drops; the other workout stays.
func TestExpiryEndToEnd(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	base := a.st.Used()
	pub, token, _ := a.upload("ride-30min.fit")
	other, otherToken, _ := a.upload("run-30min.fit")
	id := idOf(a.app, pub)
	a.setTexts(pub, token)
	for _, w := range [][2]string{{pub, token}, {other, otherToken}} {
		if rec := a.owner("POST", "/api/workouts/"+w[0]+"/publish", w[1], ""); rec.Code != 200 {
			t.Fatal(rec.Code)
		}
	}
	if left := dataLeft(t, dir, id, pub); len(left) < 3 {
		t.Fatalf("before expiry: %v", left)
	}
	now := time.Now().Unix()
	a.setTimes(pub, now-7*day+60, now+60, 0)
	if err := a.svc.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left := dataLeft(t, dir, id, pub); len(left) < 3 {
		t.Fatalf("a minute before the deadline: %v", left)
	}
	a.setTimes(pub, now-7*day, now, 0)
	if err := a.svc.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left := dataLeft(t, dir, id, pub, texts...); left != nil {
		t.Errorf("after expiry: %v", left)
	}
	if _, err := os.Stat(filepath.Join(dir, "public", pub, "index.html")); !os.IsNotExist(err) {
		t.Errorf("page still there: %v", err)
	}
	checkTotal(t, a, base)
	for method, paths := range ownerPaths(pub) {
		for _, p := range paths {
			if rec := a.owner(method, p, token, `{}`); rec.Code != 404 || rec.Body.String() != ownerNotFound {
				t.Errorf("%s %s: %d", method, p, rec.Code)
			}
		}
	}
	if rec := a.owner("GET", "/api/workouts/"+other, otherToken, ""); rec.Code != 200 || len(a.publicFiles(other)) != 4 {
		t.Errorf("other: %d %v", rec.Code, a.publicFiles(other))
	}
	logs := a.logs.String()
	if !strings.Contains(logs, `"msg":"workout deleted","id":"`+id+`","by":"expiry"`) || strings.Contains(logs, pub) || strings.Contains(logs, token) {
		t.Errorf("log: %s", logs)
	}
	a.close()
	if left := dataLeft(t, dir, id, pub, texts...); left != nil {
		t.Errorf("after close: %v", left)
	}
}

// texts are a title and description no fixture contains, so the byte search
// of dataLeft finds only the workout's own row (QA-21-r1-1).
var texts = []string{"F21TITLEMARKER", "F21DESCRIPTIONMARKER"}

func (a *app) setTexts(pub, token string) {
	a.t.Helper()
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"title":"`+texts[0]+`","description":"`+texts[1]+`"}`); rec.Code != 200 {
		a.t.Fatalf("patch: %d %s", rec.Code, rec.Body)
	}
}

// TestOwnerDeleteLeavesNoBytes (AC-4, QA-21-r1-1): after the owner's delete
// the database files hold nothing of the row, while running and after close.
func TestOwnerDeleteLeavesNoBytes(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	id := idOf(a.app, pub)
	a.setTexts(pub, token)
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if left := dataLeft(t, dir, id, pub, texts...); !slices.ContainsFunc(left, func(s string) bool { return strings.HasPrefix(s, "bytes in ") }) {
		t.Fatalf("before the delete the search finds nothing: %v", left)
	}
	if rec := a.owner("DELETE", "/api/workouts/"+pub, token, ""); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	if left := dataLeft(t, dir, id, pub, texts...); left != nil {
		t.Errorf("after the delete: %v", left)
	}
	a.close()
	if left := dataLeft(t, dir, id, pub, texts...); left != nil {
		t.Errorf("after close: %v", left)
	}
}

// TestRecoverRowLeavesNoBytes (QA-21-r2-1): a row that Recover deletes
// because its private folder is gone leaves no bytes in the database files
// while the server runs.
func TestRecoverRowLeavesNoBytes(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	id := idOf(a.app, pub)
	a.setTexts(pub, token)
	a.close()
	if err := os.RemoveAll(filepath.Join(dir, "private", id)); err != nil {
		t.Fatal(err)
	}
	b := openStorageApp(t, dir, 0)
	if left := dataLeft(t, dir, id, pub, texts...); left != nil {
		t.Errorf("after recovery: %v", left)
	}
	b.close()
}

// TestDataLeftClassifiesEverything (X-9): an unknown table or data-directory
// entry fails the check.
func TestDataLeftClassifiesEverything(t *testing.T) {
	a := startApp(t, settings{})
	a.sql("CREATE TABLE pins (workout_id TEXT)")
	if err := os.WriteFile(filepath.Join(a.dir, "extra"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := dataLeft(t, a.dir, strings.Repeat("0", 32), strings.Repeat("0", 22))
	if !reflect.DeepEqual(got, []string{"unclassified table pins", "unclassified entry extra"}) {
		t.Errorf("got %v", got)
	}
}

// ownerResponses answers every owner route (publish included) for pub.
func ownerResponses(a *app, pub, token string) map[string]*httptest.ResponseRecorder {
	paths := ownerPaths(pub)
	paths["POST"] = []string{"/api/workouts/" + pub + "/publish"}
	out := map[string]*httptest.ResponseRecorder{}
	for method, ps := range paths {
		for _, p := range ps {
			out[method+" "+strings.Replace(p, pub, "{id}", 1)] = a.owner(method, p, token, `{"title":"x"}`)
		}
	}
	return out
}

// TestDraftExpiry (AC 2, AC 4, AC 5): a draft with a 7-day lifetime lives
// until 48 h after upload. Past that and not yet swept, every owner route,
// publish included, gives the byte-identical not-found response; a pass
// then deletes it.
func TestDraftExpiry(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	base := a.st.Used()
	pub, token, _ := a.upload("ride-30min.fit")
	id := idOf(a.app, pub)
	a.setTexts(pub, token)
	now := time.Now().Unix()
	a.setTimes(pub, now-2*day+60, now+5*day+60, 0)
	if err := a.svc.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := a.owner("GET", "/api/workouts/"+pub, token, ""); rec.Code != 200 {
		t.Fatalf("a minute before 48 h: %d", rec.Code)
	}

	a.setTimes(pub, now-2*day, now+5*day, 0)
	unknown := a.owner("GET", "/api/workouts/"+strings.Repeat("z", 22), token, "")
	want := unknown.Header().Clone()
	want.Del("Date")
	for route, rec := range ownerResponses(a.app, pub, token) {
		got := rec.Header().Clone()
		got.Del("Date")
		if rec.Code != 404 || rec.Body.String() != ownerNotFound || !reflect.DeepEqual(got, want) {
			t.Errorf("%s at 48 h: %d %s %v", route, rec.Code, rec.Body, got)
		}
	}
	if len(a.rows()) != 1 || a.rows()[0]["visibility"] != "draft" || a.publicFiles(pub) != nil {
		t.Fatalf("the overdue draft changed: %v %v", a.rows(), a.publicFiles(pub))
	}
	if err := a.svc.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left := dataLeft(t, dir, id, pub, texts...); left != nil {
		t.Errorf("after the pass: %v", left)
	}
	checkTotal(t, a, base)
}

// TestPublishedAt47Hours (AC 3): published 47 h after upload with 7 days,
// it is there at 49 h and gone at 7 days.
func TestPublishedAt47Hours(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	id := idOf(a.app, pub)
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	now := time.Now().Unix()
	a.setTimes(pub, now-49*3600, now-49*3600+7*day, 47*3600)
	if err := a.svc.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec := a.owner("GET", "/api/workouts/"+pub, token, ""); rec.Code != 200 || len(a.publicFiles(pub)) != 4 {
		t.Fatalf("at 49 h: %d %v", rec.Code, a.publicFiles(pub))
	}
	a.setTimes(pub, now-7*day, now, 47*3600)
	if err := a.svc.Expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left := dataLeft(t, dir, id, pub); left != nil {
		t.Errorf("at 7 days: %v", left)
	}
}

// TestExpiryAtStartup (AC 6): a workout that expired while the server was
// stopped is gone when newApp returns, before serve opens the listener, and
// the total is measured after it.
func TestExpiryAtStartup(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	id := idOf(a.app, pub)
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	a.close()
	now := time.Now().Unix()
	a.setTimes(pub, now-8*day, now-day, 0)

	b := openStorageApp(t, dir, 0)
	if left := dataLeft(t, dir, id, pub); left != nil {
		t.Errorf("after start: %v", left)
	}
	if want := diskUsage(t, dir); b.st.Used() != want { // measured after the pass
		t.Errorf("total %d, on disk %d", b.st.Used(), want)
	}
	logs := b.logs.String()
	if d, s := strings.Index(logs, `"by":"expiry"`), strings.Index(logs, `"msg":"storage"`); d < 0 || s < d {
		t.Errorf("log: %s", logs)
	}
}

// TestExpiryTimer (AC 1, X-5): the timer deletes a workout soon after its
// deadline without any request. The period is short here; in production it
// is 30 s.
func TestExpiryTimer(t *testing.T) {
	if expiryPeriod != 30*time.Second {
		t.Fatalf("period %v", expiryPeriod)
	}
	a := startApp(t, settings{expiryEvery: 20 * time.Millisecond})
	pub, token, _ := a.upload("ride-30min.fit")
	id := idOf(a, pub)
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	deadline := time.Now().Unix() + 2
	a.setTimes(pub, deadline-7*day, deadline, 0)
	time.Sleep(200 * time.Millisecond)
	if left := dataLeft(t, a.dir, id, pub); len(left) < 3 {
		t.Fatalf("before the deadline: %v", left)
	}
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if dataLeft(t, a.dir, id, pub) == nil {
			if late := time.Since(time.Unix(deadline, 0)); late > 2*time.Second {
				t.Errorf("gone %v after the deadline", late)
			}
			return
		}
	}
	t.Fatal("not deleted")
}
