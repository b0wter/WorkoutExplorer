package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// F-37 end to end through newApp (architect/F-37-report-notification-links.md,
// Verification focus), against a local fake Pushover.

const linkInvalid = `{"error":"not_found","message":"This delete link is no longer valid: it was already used, has expired, or the workout is gone."}`

// keyOf is the delete key of a notification message that starts with head
// (DK-10); it fails the test if the message has any other form.
func keyOf(t *testing.T, msg, head string) string {
	t.Helper()
	m := regexp.MustCompile(`^` + regexp.QuoteMeta(head) + `\n<a href="` + regexp.QuoteMeta(testBaseURL) +
		`/report-delete#([0-9A-Za-z]{22})">Delete this workout</a>$`).FindStringSubmatch(msg)
	if m == nil {
		t.Fatalf("message %q", msg)
	}
	return m[1]
}

// pushFake is a Pushover that answers 200 and keeps every raw body.
type pushFake struct {
	url  string
	reqs chan string
}

func newPushFake(t *testing.T) *pushFake {
	t.Helper()
	p := &pushFake{reqs: make(chan string, 64)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		p.reqs <- string(b)
	}))
	t.Cleanup(srv.Close)
	p.url = srv.URL + "/1/messages.json"
	return p
}

// next is the next notification's raw body and form.
func (p *pushFake) next(t *testing.T) (string, url.Values) {
	t.Helper()
	select {
	case raw := <-p.reqs:
		f, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		return raw, f
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
		return "", nil
	}
}

// keyApp is serve over dir, with notifications to push ("": off), and every
// response of the delete link's API kept for the byte scan.
type keyApp struct {
	*app
	close     func() error
	responses *syncBuffer
}

func openKeyApp(t *testing.T, dir, push string) *keyApp {
	t.Helper()
	s := settings{maxConcurrentUploads: 4, webDir: testWeb, baseURL: testBaseURL}
	if push != "" {
		s.pushoverToken, s.pushoverUser, s.pushoverEndpoint = testPushToken, testPushUser, push
	}
	k := &keyApp{app: &app{t: t, dir: dir, logs: &syncBuffer{}}, responses: &syncBuffer{}}
	h, closeApp, err := newApp(context.Background(), dir, s, slog.New(slog.NewJSONHandler(k.logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	k.h, k.close = h, closeApp
	t.Cleanup(func() { closeApp() })
	return k
}

var keyClient atomic.Uint32

// key sends one lookup (GET) or confirm (POST) with the key as Bearer ("": no
// header), from a new address each time so the DK-9 limit stays out of the way.
func (k *keyApp) key(method, key string) *httptest.ResponseRecorder {
	k.t.Helper()
	r := httptest.NewRequest(method, "/api/report-delete", nil)
	n := keyClient.Add(1)
	r.RemoteAddr = fmt.Sprintf("10.%d.%d.%d:1", n>>16&255, n>>8&255, n&255)
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	k.h.ServeHTTP(rec, r)
	fmt.Fprintf(k.responses, "%v %s\n", rec.Header(), rec.Body)
	return rec
}

// reportKey files a report on pub and returns the key of its notification.
func (k *keyApp) reportKey(p *pushFake, pub, reason, label string, n int) (key, raw string, form url.Values) {
	k.t.Helper()
	if rec := k.report(pub, reason, reportMarker, "198.51.100.1:1"); rec.Code != 204 {
		k.t.Fatalf("report: %d", rec.Code)
	}
	raw, form = p.next(k.t)
	return keyOf(k.t, form.Get("message"), fmt.Sprintf("New report #%d: %s", n, label)), raw, form
}

// hashes is every non-NULL delete_key_hash, hex.
func (k *keyApp) hashes() []string {
	k.t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(k.dir, "workouts.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		k.t.Fatal(err)
	}
	defer db.Close()
	rs, err := db.Query("SELECT hex(delete_key_hash) FROM reports WHERE delete_key_hash IS NOT NULL ORDER BY id")
	if err != nil {
		k.t.Fatal(err)
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var h string
		rs.Scan(&h)
		out = append(out, strings.ToLower(h))
	}
	return out
}

func hashHex(key string) string { s := sha256.Sum256([]byte(key)); return hex.EncodeToString(s[:]) }

func isInvalid(rec *httptest.ResponseRecorder) bool {
	return rec.Code == 404 && rec.Body.String() == linkInvalid && rec.Header().Get("Cache-Control") == "no-store"
}

// TestReportDeleteLink (AC 27, 29, 30, 32, 33, 34): the notification's
// fields, the key stored only as its hash, a lookup that changes nothing,
// one confirm that deletes like the operator command, and no key anywhere
// but the outgoing Pushover request.
func TestReportDeleteLink(t *testing.T) {
	dir := t.TempDir()
	push := newPushFake(t)
	a := openKeyApp(t, dir, push.url)
	pub, token := a.published("ride-30min.fit")
	other, _ := a.published("run-30min.fit")
	id := idOf(a.app, pub)
	// Through the API: a.sql's connection has no secure_delete and would leave the old row's bytes.
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"title":"F37TITLEMARKER","description":"F37DESCMARKER"}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}

	key, raw, form := a.reportKey(push, pub, "personal_data", "Personal data", 1)
	if form.Get("url") != testBaseURL+"/w/"+pub || form.Get("url_title") != "Open workout" || form.Get("html") != "1" || len(form) != 7 {
		t.Errorf("notification %v", form)
	}
	for _, s := range []string{reportMarker, "F37TITLEMARKER", "F37DESCMARKER", "198.51.100", id, other} {
		if strings.Contains(raw, s) {
			t.Errorf("%q reached Pushover", s)
		}
	}
	otherKey, _, _ := a.reportKey(push, other, "spam", "Spam", 2)
	if h := a.hashes(); !slices.Equal(h, []string{hashHex(key), hashHex(otherKey)}) {
		t.Fatalf("stored %v", h)
	}

	// Lookup, twice: the DK-7 fields, nothing deleted or spent.
	var row map[string]any
	for _, r := range a.rows() {
		if r["public_id"] == pub {
			row = r
		}
	}
	for i := 0; i < 2; i++ {
		rec := a.key("GET", key)
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != 200 || err != nil {
			t.Fatalf("lookup %d %s", rec.Code, rec.Body)
		}
		filed := got["filed_at"].(float64)
		if got["title"] != "F37TITLEMARKER" || got["start_date"] == "" || got["report_id"] != 1.0 || got["reason"] != "Personal data" ||
			got["expires_at"] != float64(row["expires_at"].(int64)) || got["report_count"] != 1.0 || len(got) != 7 ||
			filed < float64(time.Now().Unix()-60) || filed > float64(time.Now().Unix()) {
			t.Errorf("lookup %v", got)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "public", pub, "index.html")); err != nil || len(a.hashes()) != 2 {
		t.Fatalf("the lookup changed something: %v, %v", err, a.hashes())
	}
	// A key shows only its own workout.
	if rec := a.key("GET", otherKey); !strings.Contains(rec.Body.String(), `"report_id":2`) || strings.Contains(rec.Body.String(), "F37TITLEMARKER") {
		t.Errorf("other key: %s", rec.Body)
	}

	// Confirm: deleted exactly like `workout-explorer delete`.
	if rec := a.key("POST", key); rec.Code != 204 || rec.Body.Len() != 0 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("confirm %d %s", rec.Code, rec.Body)
	}
	if left := dataLeft(t, dir, id, pub, "F37TITLEMARKER", "F37DESCMARKER", key); left != nil {
		t.Errorf("left after the confirm: %v", left)
	}
	var lines []map[string]any
	for _, l := range strings.Split(a.logs.String(), "\n") {
		if strings.Contains(l, `"msg":"workout deleted"`) {
			var m map[string]any
			json.Unmarshal([]byte(l), &m)
			lines = append(lines, m)
		}
	}
	if len(lines) != 1 || lines[0]["id"] != id || lines[0]["by"] != "operator" || len(lines[0]) != 5 {
		t.Errorf("delete lines %v", lines)
	}
	idOf(a.app, other) // the other workout stays
	// Single use.
	for _, m := range []string{"POST", "GET"} {
		if rec := a.key(m, key); !isInvalid(rec) {
			t.Errorf("%s after use: %d %s", m, rec.Code, rec.Body)
		}
	}
	if !slices.Equal(a.hashes(), []string{hashHex(otherKey)}) {
		t.Errorf("hashes %v", a.hashes())
	}

	// AC 29: the plain key and its hash only in the Pushover request.
	sum := sha256.Sum256([]byte(key))
	a.close()
	for p, b := range a.dataFiles() {
		if bytes.Contains(b, []byte(key)) || bytes.Contains(b, []byte(otherKey)) || bytes.Contains(b, sum[:]) {
			t.Errorf("key or spent hash in %s", p)
		}
	}
	for name, s := range map[string]string{"log": a.logs.String(), "responses": a.responses.String()} {
		for _, k := range []string{key, otherKey, hashHex(key), hashHex(otherKey), "report-delete#"} {
			if strings.Contains(strings.ToLower(s), strings.ToLower(k)) {
				t.Errorf("%q in the %s", k, name)
			}
		}
	}
}

// TestReportDeleteLinkInvalid (AC 33, 35; DK-4, DK-12): unknown, malformed,
// absent, used, expired, a workout past its deadline or gone, the other key
// of a deleted workout: the same status and bytes for lookup and confirm,
// nothing deleted, nothing logged.
func TestReportDeleteLinkInvalid(t *testing.T) {
	push := newPushFake(t)
	a := openKeyApp(t, t.TempDir(), push.url)
	pubA, _ := a.published("ride-30min.fit")
	pubB, _ := a.published("run-30min.fit")
	pubC, _ := a.published("ride-30min.fit")
	pubD, tokenD := a.published("run-30min.fit")
	kA1, _, _ := a.reportKey(push, pubA, "spam", "Spam", 1)
	kA2, _, _ := a.reportKey(push, pubA, "other", "Other", 2)
	kB, _, _ := a.reportKey(push, pubB, "illegal_content", "Illegal content", 3)
	kC, _, _ := a.reportKey(push, pubC, "spam", "Spam", 4)
	kD, _, _ := a.reportKey(push, pubD, "spam", "Spam", 5)

	now := time.Now().Unix()
	a.sql("UPDATE reports SET created_at = ? WHERE id = 3", now-30*day) // B's report at the 30-day bound
	a.setTimes(pubC, now-day, now, 0)                                   // C at its deadline, not yet swept
	if rec := a.owner("DELETE", "/api/workouts/"+pubD, tokenD, ""); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	if rec := a.key("POST", kA1); rec.Code != 204 {
		t.Fatalf("confirm A: %d", rec.Code)
	}
	logs := a.logs.String()

	for name, k := range map[string]string{
		"absent": "", "short": "abc", "long": kA2 + "A", "not base62": kA2[:21] + "-", "unknown": strings.Repeat("Z", 22),
		"used": kA1, "other key of a deleted workout": kA2, "report 30 days old": kB, "workout at its deadline": kC, "workout deleted by its owner": kD,
	} {
		for _, m := range []string{"GET", "POST"} {
			if rec := a.key(m, k); !isInvalid(rec) {
				t.Errorf("%s %s: %d %s", name, m, rec.Code, rec.Body)
			}
		}
	}
	if len(idOf(a.app, pubB)) != 32 || len(idOf(a.app, pubC)) != 32 {
		t.Error("an invalid key deleted a workout")
	}
	if a.logs.String() != logs {
		t.Errorf("refusals logged: %s", strings.TrimPrefix(a.logs.String(), logs))
	}
}

// TestReportDeleteConcurrent (AC 33, DK-8): parallel confirms of one key, and
// one confirm per key of two reports on one workout: exactly one 204 and one
// operator line.
func TestReportDeleteConcurrent(t *testing.T) {
	push := newPushFake(t)
	a := openKeyApp(t, t.TempDir(), push.url)
	pub, _ := a.published("ride-30min.fit")
	k1, _, _ := a.reportKey(push, pub, "spam", "Spam", 1)
	k2, _, _ := a.reportKey(push, pub, "spam", "Spam", 2)
	pub2, _ := a.published("run-30min.fit")
	k3, _, _ := a.reportKey(push, pub2, "spam", "Spam", 3)

	for _, keys := range [][]string{{k1, k2, k1, k2, k1, k2, k1, k2}, {k3, k3, k3, k3, k3, k3, k3, k3}} {
		codes := make(chan int, len(keys))
		var wg sync.WaitGroup
		for _, k := range keys {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- a.key("POST", k).Code }()
		}
		wg.Wait()
		close(codes)
		var got []int
		for c := range codes {
			got = append(got, c)
		}
		slices.Sort(got)
		if got[0] != 204 || got[1] != 404 || got[len(got)-1] != 404 {
			t.Errorf("codes %v", got)
		}
	}
	if n := strings.Count(a.logs.String(), `"msg":"workout deleted"`); n != 2 {
		t.Errorf("%d delete lines, want 2", n)
	}
}

// TestReportDeleteWithoutPushover (AC 28, DK-5): off, a report gets no key;
// a start without Pushover clears every key, and turning it on again does not
// bring them back.
func TestReportDeleteWithoutPushover(t *testing.T) {
	dir := t.TempDir()
	push := newPushFake(t)
	a := openKeyApp(t, dir, push.url)
	pub, _ := a.published("ride-30min.fit")
	key, _, _ := a.reportKey(push, pub, "spam", "Spam", 1)
	if rec := a.key("GET", key); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	a.close()

	b := openKeyApp(t, dir, "")
	if h := b.hashes(); h != nil {
		t.Errorf("keys after a start without Pushover: %v", h)
	}
	if !strings.Contains(b.logs.String(), `"msg":"delete keys cleared","removed":1`) {
		t.Errorf("log %s", b.logs.String())
	}
	if rec := b.report(pub, "other", "", "198.51.100.1:1"); rec.Code != 204 || len(b.reportRows()) != 2 || b.hashes() != nil {
		t.Errorf("report without Pushover: %d, %v", rec.Code, b.hashes())
	}
	for _, m := range []string{"GET", "POST"} {
		if rec := b.key(m, key); !isInvalid(rec) {
			t.Errorf("%s: %d %s", m, rec.Code, rec.Body)
		}
	}
	b.close()

	c := openKeyApp(t, dir, push.url)
	if rec := c.key("POST", key); !isInvalid(rec) || len(idOf(c.app, pub)) != 32 {
		t.Errorf("revoked key after turning Pushover on again: %d", rec.Code)
	}
}
