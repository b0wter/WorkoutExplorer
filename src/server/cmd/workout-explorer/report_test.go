package main

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// F-27 end to end through newApp (architect/F-27-reports-api-and-operator-delete.md,
// Verification).

// report posts a report from remote with extra header pairs.
func (a *app) report(pub, reason, message, remote string, hdr ...string) *httptest.ResponseRecorder {
	a.t.Helper()
	body := `{"public_id":"` + pub + `","reason":"` + reason + `"`
	if message != "" {
		body += `,"message":"` + message + `"`
	}
	r := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body+"}"))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = remote
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, r)
	return rec
}

// published uploads and publishes a workout.
func (a *app) published(name string) (pub, token string) {
	a.t.Helper()
	pub, token, _ = a.upload(name)
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		a.t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	return pub, token
}

// reportRows is every report as reason, message, created_at, workout_id.
func (a *app) reportRows() [][4]any {
	a.t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(a.dir, "workouts.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		a.t.Fatal(err)
	}
	defer db.Close()
	rs, err := db.Query("SELECT reason, message, created_at, workout_id FROM reports ORDER BY id")
	if err != nil {
		a.t.Fatal(err)
	}
	defer rs.Close()
	var out [][4]any
	for rs.Next() {
		var r [4]any
		if err := rs.Scan(&r[0], &r[1], &r[2], &r[3]); err != nil {
			a.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

const reportMarker = "F27REPORTMESSAGEMARKER"

// TestReportEndToEnd (AC 1, AC 3; R-4, OQ-101): stored with reason, message
// and time; a draft and an unknown ID get the owner API's not found; the
// eleventh report is answered 204 and not stored.
func TestReportEndToEnd(t *testing.T) {
	a := startApp(t, settings{})
	pub, _ := a.published("ride-30min.fit")
	draft, _, _ := a.upload("run-30min.fit")
	id := idOf(a, pub)

	t0 := time.Now().Unix()
	if rec := a.report(pub, "personal_data", "  "+reportMarker+` line\nnext  `, "198.51.100.1:1"); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("report: %d %s", rec.Code, rec.Body)
	}
	rows := a.reportRows()
	if len(rows) != 1 || rows[0][0] != "personal_data" || rows[0][1] != reportMarker+" line\nnext" || rows[0][3] != id {
		t.Fatalf("rows %v", rows)
	}
	if at := rows[0][2].(int64); at < t0 || at > time.Now().Unix() {
		t.Errorf("created_at %d", at)
	}
	for _, p := range []string{draft, strings.Repeat("A", 22), "x", ""} {
		if rec := a.report(p, "spam", "", "198.51.100.1:1"); rec.Code != 404 || rec.Body.String() != ownerNotFound {
			t.Errorf("%q: %d %s", p, rec.Code, rec.Body)
		}
	}
	for i := 0; i < 10; i++ {
		if rec := a.report(pub, "spam", "", "198.51.100.1:1"); rec.Code != 204 {
			t.Fatalf("report %d: %d", i+2, rec.Code)
		}
	}
	if n := len(a.reportRows()); n != 10 {
		t.Errorf("%d reports stored, want 10", n)
	}
	logs := a.logs.String()
	if n := strings.Count(logs, `"msg":"report stored"`); n != 10 || strings.Contains(logs, reportMarker) || strings.Contains(logs, pub) {
		t.Errorf("%d stored lines; log %s", n, logs)
	}
}

// TestReportNoRequestMetadataStored (AC 1, AC 4; F-09's check extended): the
// address, user agent, Referer and language appear in no file of the data
// directory and in no log line. The mutant check first shows that the scan
// does see report data: an address put into a message is found.
func TestReportNoRequestMetadataStored(t *testing.T) {
	a := startApp(t, settings{})
	pub, _ := a.published("ride-30min.fit")
	v6 := "2001:db8:1234:5678::abcd"
	needles := []string{"203.0.113.77", "203.0.113.88", v6, "2001:db8", "MARKER-UA", "MARKER-REFERER", "MARKER-LANG"}
	scan := func() (hits []string) {
		a.t.Helper()
		for p, b := range a.dataFiles() {
			for _, n := range needles {
				if bytes.Contains(b, []byte(n)) {
					hits = append(hits, n+" in "+filepath.Base(p))
				}
			}
		}
		for _, n := range needles {
			if strings.Contains(a.logs.String(), n) {
				hits = append(hits, n+" in the log")
			}
		}
		return hits
	}
	for _, tc := range []struct{ remote, xff string }{
		{"127.0.0.1:4242", "203.0.113.77"},
		{"127.0.0.1:4243", v6},
		{"[" + v6 + "]:5000", ""},
		{"203.0.113.88:5001", ""},
	} {
		h := []string{"User-Agent", "MARKER-UA/1.0", "Referer", "https://MARKER-REFERER.example/", "Accept-Language", "MARKER-LANG"}
		if tc.xff != "" {
			h = append(h, "X-Forwarded-For", tc.xff)
		}
		if rec := a.report(pub, "other", "hello", tc.remote, h...); rec.Code != 204 {
			t.Fatalf("%v: %d %s", tc, rec.Code, rec.Body)
		}
	}
	if len(a.reportRows()) != 4 {
		t.Fatalf("%d reports", len(a.reportRows()))
	}
	if hits := scan(); hits != nil {
		t.Errorf("request data stored or logged: %v", hits)
	}
	// Mutant: were the address stored, the scan would find it.
	if rec := a.report(pub, "other", "203.0.113.77", "198.51.100.1:1"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	if hits := scan(); !slices.ContainsFunc(hits, func(h string) bool { return strings.HasPrefix(h, "203.0.113.77 in workouts.db") }) {
		t.Errorf("the scan does not see report rows: %v", hits)
	}
}

// TestReportsGoWithTheirWorkout (AC 5, R-2; F-12 AC 5, F-21 X-9): owner
// delete, expiry, the operator's delete while serve runs, and recovery's row
// removal each leave no report row and no report bytes in the database files.
func TestReportsGoWithTheirWorkout(t *testing.T) {
	for _, path := range []string{"owner", "expiry", "operator", "recovery"} {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			a := openStorageApp(t, dir, 0)
			pub, token := a.published("ride-30min.fit")
			other, _ := a.published("run-30min.fit")
			id := idOf(a.app, pub)
			if rec := a.report(pub, "illegal_content", reportMarker, "198.51.100.1:1"); rec.Code != 204 {
				t.Fatal(rec.Code)
			}
			if rec := a.report(other, "spam", "", "198.51.100.1:1"); rec.Code != 204 {
				t.Fatal(rec.Code)
			}
			if left := dataLeft(t, dir, id, pub, reportMarker); !slices.Contains(left, "table reports") {
				t.Fatalf("before: %v", left)
			}
			switch path {
			case "owner":
				if rec := a.owner("DELETE", "/api/workouts/"+pub, token, ""); rec.Code != 204 {
					t.Fatal(rec.Code)
				}
			case "expiry":
				now := time.Now().Unix()
				a.setTimes(pub, now-7*day, now, 0)
				if err := a.svc.Expire(context.Background()); err != nil {
					t.Fatal(err)
				}
			case "operator":
				var out, errb bytes.Buffer
				if code := deleteCmd([]string{"-data", dir, "-y", pub}, nil, false, &out, &errb); code != 0 {
					t.Fatalf("exit %d: %s %s", code, out.String(), errb.String())
				}
				if !strings.Contains(out.String(), "reports    1\n") || !strings.Contains(errb.String(), `"msg":"workout deleted","id":"`+id+`","by":"operator"`) {
					t.Errorf("output %s / %s", out.String(), errb.String())
				}
				// serve's next pass checkpoints even with nothing of its own to delete (R-10).
				if err := a.svc.Expire(context.Background()); err != nil {
					t.Fatal(err)
				}
				if rec := a.owner("GET", "/api/workouts/"+pub, token, ""); rec.Code != 404 {
					t.Errorf("owner API after operator delete: %d", rec.Code)
				}
			case "recovery":
				a.close()
				if err := os.RemoveAll(filepath.Join(dir, "private", id)); err != nil {
					t.Fatal(err)
				}
				a = openStorageApp(t, dir, 0)
			}
			if left := dataLeft(t, dir, id, pub, reportMarker); left != nil {
				t.Errorf("after the delete: %v", left)
			}
			if n := len(a.reportRows()); n != 1 {
				t.Errorf("%d reports left, want the other workout's 1", n)
			}
			a.close()
			if left := dataLeft(t, dir, id, pub, reportMarker); left != nil {
				t.Errorf("after close: %v", left)
			}
		})
	}
}

// TestReportRetention (R-3): a report filed 30 days ago is gone after one
// pass, its bytes too; a younger one stays.
func TestReportRetention(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, _ := a.published("ride-30min.fit")
	for _, m := range []string{"F27OLDREPORT", "F27YOUNGREPORT"} {
		if rec := a.report(pub, "other", m, "198.51.100.1:1"); rec.Code != 204 {
			t.Fatal(rec.Code)
		}
	}
	a.close()
	cut := time.Now().Unix() - 30*day
	a.sql("UPDATE reports SET created_at = ? WHERE message = 'F27OLDREPORT'", cut)
	a.sql("UPDATE reports SET created_at = ? WHERE message = 'F27YOUNGREPORT'", cut+120)
	b := openStorageApp(t, dir, 0) // the startup pass
	if rows := b.reportRows(); len(rows) != 1 || rows[0][1] != "F27YOUNGREPORT" {
		t.Errorf("rows %v", rows)
	}
	for _, n := range []string{"workouts.db", "workouts.db-wal"} {
		if raw, _ := os.ReadFile(filepath.Join(dir, n)); bytes.Contains(raw, []byte("F27OLDREPORT")) {
			t.Errorf("pruned report's bytes in %s", n)
		}
	}
	if !strings.Contains(b.logs.String(), `"msg":"reports pruned","removed":1`) {
		t.Errorf("log %s", b.logs.String())
	}
}

// TestDeleteCommand (AC 6, R-8, R-9): exit codes, the question, and a draft
// deleted like a published workout.
func TestDeleteCommand(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, _ := a.published("ride-30min.fit")
	draft, draftToken, _ := a.upload("run-30min.fit")
	a.sql("UPDATE workouts SET title = ? WHERE public_id = ?", "Evil \x1b[2J title", pub)
	del := func(stdin string, terminal bool, args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := deleteCmd(args, strings.NewReader(stdin), terminal, &out, &errb)
		return code, out.String(), errb.String()
	}
	for _, c := range []struct {
		name string
		args []string
		want int
	}{
		{"no arguments", nil, 2},
		{"no data", []string{pub}, 2},
		{"empty data", []string{"-data", "", pub}, 2},
		{"two IDs", []string{"-data", dir, pub, draft}, 2},
		{"flag after ID", []string{"-data", dir, pub, "-y"}, 2},
		{"malformed", []string{"-data", dir, "-y", "../" + pub[3:]}, 2},
		{"unknown flag", []string{"-data", dir, "-force", pub}, 2},
		{"no terminal, no -y", []string{"-data", dir, pub}, 2},
		{"unknown ID", []string{"-data", dir, "-y", strings.Repeat("A", 22)}, 3},
	} {
		if code, out, errb := del("", false, c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d (%s %s)", c.name, code, c.want, out, errb)
		}
	}
	if len(a.rows()) != 2 {
		t.Fatal("a refused command deleted something")
	}

	// A missing database: exit 1, and nothing is created.
	empty := t.TempDir()
	if code, _, errb := del("", false, "-data", empty, "-y", pub); code != 1 || !strings.Contains(errb, "no database") {
		t.Errorf("missing database: %d %s", code, errb)
	}
	if es, _ := os.ReadDir(empty); len(es) != 0 {
		t.Errorf("created %v", es)
	}

	// On a terminal it asks; anything but y or yes keeps the workout.
	for _, answer := range []string{"", "n\n", "no\n", "x\n", "Ja\n"} {
		code, out, _ := del(answer, true, "-data", dir, pub)
		if code != 1 || !strings.Contains(out, "Delete? [y/N] ") || !strings.Contains(out, "Not deleted.") {
			t.Errorf("answer %q: %d %s", answer, code, out)
		}
	}
	if len(a.rows()) != 2 {
		t.Fatal("deleted without a yes")
	}
	code, out, _ := del("Y\n", true, "-data", dir, pub)
	if code != 0 || !strings.Contains(out, "public_id  "+pub+"\nstate      unlisted\n") || !strings.Contains(out, `title      "Evil \x1b[2J title"`) || strings.Contains(out, "\x1b") || !strings.Contains(out, "Deleted.") {
		t.Errorf("yes: %d %q", code, out)
	}
	if code, _, errb := del("", false, "-data", dir, "-y", pub); code != 3 || !strings.Contains(errb, "no workout with this ID") {
		t.Errorf("repeat: %d %s", code, errb)
	}
	// A draft past its deadline is still deleted (INF-3 "any workout").
	a.setTimes(draft, time.Now().Unix()-3*day, time.Now().Unix()+day, 0)
	code, out, _ = del("", false, "-data", dir, "-y", draft)
	if code != 0 || !strings.Contains(out, "state      draft, past its deadline\n") {
		t.Errorf("draft: %d %s", code, out)
	}
	if len(a.rows()) != 0 || a.entries("private") != nil || a.entries("public") != nil {
		t.Errorf("left: rows %v private %v public %v", a.rows(), a.entries("private"), a.entries("public"))
	}
	if rec := a.owner("GET", "/api/workouts/"+draft, draftToken, ""); rec.Code != 404 {
		t.Errorf("owner API: %d", rec.Code)
	}

	// A database of another schema version is refused without a change.
	a.sql("PRAGMA user_version = 3")
	if code, _, errb := del("", false, "-data", dir, "-y", strings.Repeat("A", 22)); code != 1 || !strings.Contains(errb, "is not this binary's") {
		t.Errorf("old schema: %d %s", code, errb)
	}

	// run dispatches it; the usage lists it.
	var errb bytes.Buffer
	if code := run([]string{"delete"}, io.Discard, &errb); code != 2 || !strings.Contains(errb.String(), "usage: workout-explorer delete") {
		t.Errorf("run delete: %d %s", code, errb.String())
	}
	errb.Reset()
	run(nil, io.Discard, &errb)
	if !strings.Contains(errb.String(), "delete ") {
		t.Errorf("usage %s", errb.String())
	}
}

// TestDeleteReadsNoSetting (R-8): an invalid serve setting and the
// Pushover secrets do not affect delete.
func TestDeleteReadsNoSetting(t *testing.T) {
	t.Setenv(envMaxConcurrentUploads, "99")
	t.Setenv(envPushoverToken, "x")
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, _ := a.published("ride-30min.fit")
	var out, errb bytes.Buffer
	if code := deleteCmd([]string{"-data", dir, "-y", pub}, nil, false, &out, &errb); code != 0 || strings.Contains(errb.String(), "WORKOUT_EXPLORER") {
		t.Errorf("%d %s", code, errb.String())
	}
}

const (
	testPushToken = "aTokenOfThirtyLettersAndDigit1"
	testPushUser  = "uUserKeyOfThirtyLettersDigits2"
)

// TestPushoverSettings (ADR-039 item 1): both or neither; 30 letters and
// digits; an error names the variable, never its value.
func TestPushoverSettings(t *testing.T) {
	bad := "SECRETVALUE-not-30-chars"
	for _, c := range []struct {
		name        string
		token, user string
		err         string
	}{
		{"neither", "", "", ""},
		{"both", testPushToken, testPushUser, ""},
		{"token only", testPushToken, "", envPushoverUser + " must be set when " + envPushoverToken + " is set"},
		{"user only", "", testPushUser, envPushoverToken + " must be set when " + envPushoverUser + " is set"},
		{"bad token", bad, testPushUser, envPushoverToken + " must be 30 letters and digits"},
		{"bad user", testPushToken, bad, envPushoverUser + " must be 30 letters and digits"},
		{"29", testPushToken[:29], testPushUser, envPushoverToken + " must be"},
		{"31", testPushToken, testPushUser + "x", envPushoverUser + " must be"},
		{"space", " " + testPushToken[1:], testPushUser, envPushoverToken + " must be"},
	} {
		s, err := readSettings(envOf(map[string]string{envPushoverToken: c.token, envPushoverUser: c.user}))
		switch {
		case c.err == "" && (err != nil || s.pushoverToken != c.token || s.pushoverUser != c.user):
			t.Errorf("%s: %+v %v", c.name, s, err)
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s: %v", c.name, err)
		case err != nil && (strings.Contains(err.Error(), bad) || strings.Contains(err.Error(), testPushToken[:10]) || strings.Contains(err.Error(), testPushUser[:10])):
			t.Errorf("%s: value in %q", c.name, err)
		}
	}
	// serve exits 2, prints no value, and creates nothing.
	t.Setenv(envPushoverToken, testPushToken)
	t.Setenv(envPushoverUser, bad)
	dir := filepath.Join(t.TempDir(), "data")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"serve", "-data", dir, "-web", testWeb, "-addr", "127.0.0.1:0"}, &stdout, &stderr); got != 2 {
		t.Fatalf("exit %d", got)
	}
	all := stdout.String() + stderr.String()
	if !strings.Contains(all, envPushoverUser) || strings.Contains(all, bad) || strings.Contains(all, testPushToken) {
		t.Errorf("output %q", all)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("data directory created")
	}
	if f := startedFields(settings{pushoverToken: testPushToken, pushoverUser: testPushUser}); !slices.Contains(f, any("notifications")) || f[len(f)-1] != true || slices.Contains(f, any(testPushToken)) {
		t.Errorf("started fields %v", f)
	}
	if f := startedFields(settings{}); f[len(f)-1] != false {
		t.Errorf("started fields %v", f)
	}
}

// TestReportNotification (R-11, ADR-039): with a hanging fake Pushover the
// report is still answered 204 at once; the request carries only the four
// fields, number and reason; unstored (capped) reports send nothing; no
// secret in any log line, also on failure.
func TestReportNotification(t *testing.T) {
	type got struct {
		form url.Values
		raw  string
	}
	reqs := make(chan got, 32)
	release := make(chan struct{})
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f, _ := url.ParseQuery(string(b))
		reqs <- got{f, r.URL.String()}
		<-release
		w.WriteHeader(http.StatusInternalServerError) // a failure, logged without secrets
	}))
	defer fake.Close()
	defer close(release)
	a := startApp(t, settings{pushoverToken: testPushToken, pushoverUser: testPushUser, pushoverEndpoint: fake.URL + "/1/messages.json"})
	pub, _ := a.published("ride-30min.fit")
	t0 := time.Now()
	if rec := a.report(pub, "illegal_content", reportMarker, "198.51.100.1:1"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	if d := time.Since(t0); d > time.Second {
		t.Errorf("report took %v", d)
	}
	var g got
	select {
	case g = <-reqs:
	case <-time.After(5 * time.Second):
		t.Fatal("no notification")
	}
	want := url.Values{"token": {testPushToken}, "user": {testPushUser}, "title": {"Workout Explorer"}, "message": {"New report #1: illegal_content"}}
	if len(g.form) != 4 || g.form.Encode() != want.Encode() || g.raw != "/1/messages.json" {
		t.Errorf("notification %v at %s", g.form, g.raw)
	}
	// While the sender hangs, more reports are still answered at once.
	for i := 0; i < 9; i++ {
		t0 = time.Now()
		if rec := a.report(pub, "spam", "", "198.51.100.1:1"); rec.Code != 204 || time.Since(t0) > time.Second {
			t.Fatalf("report %d: %d after %v", i, rec.Code, time.Since(t0))
		}
	}
	// The eleventh report is not stored and not notified.
	if rec := a.report(pub, "spam", "", "198.51.100.1:1"); rec.Code != 204 || len(a.reportRows()) != 10 {
		t.Fatalf("capped: %d, %d rows", rec.Code, len(a.reportRows()))
	}
	release <- struct{}{} // #1 fails with 500
	var next got
	select {
	case next = <-reqs:
	case <-time.After(5 * time.Second):
		t.Fatal("no second notification")
	}
	if m := next.form.Get("message"); m != "New report #2: spam" {
		t.Errorf("second message %q", m) // 9 queued fit the queue: none folded
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(a.logs.String(), `"msg":"notification failed","status":500`) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	logs := a.logs.String()
	if !strings.Contains(logs, `"msg":"notification failed","status":500`) {
		t.Errorf("no failure line: %s", logs)
	}
	if strings.Contains(logs, testPushToken) || strings.Contains(logs, testPushUser) || strings.Contains(logs, reportMarker) || strings.Contains(logs, pub) {
		t.Errorf("secret or report data in the log: %s", logs)
	}
	for p, b := range a.dataFiles() {
		if bytes.Contains(b, []byte(testPushToken)) || bytes.Contains(b, []byte(testPushUser)) {
			t.Errorf("secret in %s", p)
		}
	}
}
