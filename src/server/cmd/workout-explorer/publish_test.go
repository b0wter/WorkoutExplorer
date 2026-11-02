package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// F-15 end to end through newApp (architect/F-15-publish-pipeline-and-static-page.md,
// Verification). The generic page is Caddy's rule "no index.html" (P-13), so
// these tests assert on the files.

func (a *app) publicFiles(pub string) []string {
	a.t.Helper()
	es, err := os.ReadDir(filepath.Join(a.dir, "public", pub))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		a.t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func (a *app) page(pub string) string {
	a.t.Helper()
	b, err := os.ReadFile(filepath.Join(a.dir, "public", pub, "index.html"))
	if err != nil {
		a.t.Fatal(err)
	}
	return string(b)
}

var dataLink = regexp.MustCompile(`<link rel="preload" id="wx-data" href="/w/[0-9A-Za-z]{22}/(data\.[0-9a-f]{16}\.bin)"`)

func dataName(t *testing.T, page string) string {
	t.Helper()
	m := dataLink.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no data file link in the page")
	}
	return m[1]
}

// previewName is the preview the page's og:image names (F-23 PC-7).
func previewName(t *testing.T, page string) string {
	t.Helper()
	m := previewLink.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no preview link in the page")
	}
	return m[1]
}

var previewLink = regexp.MustCompile(`<meta property="og:image" content="https?://[^"/]+/w/[0-9A-Za-z]{22}/(preview\.[0-9a-f]{16}\.png)" />`)

// TestPublishEndToEnd covers AC 1 to 5 and 10 through the API.
func TestPublishEndToEnd(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	id := internalID(a.app)
	orig := a.privateFile(id, "original.fit")
	// The storage total follows the files on disk exactly (P-9).
	offset := a.st.Used() - diskUsage(t, filepath.Join(dir, "private")) - diskUsage(t, filepath.Join(dir, "public"))
	counted := func(step string) {
		t.Helper()
		if got := a.st.Used() - diskUsage(t, filepath.Join(dir, "private")) - diskUsage(t, filepath.Join(dir, "public")); got != offset {
			t.Errorf("%s: storage total off by %d", step, got-offset)
		}
	}

	// AC 1: before publishing, nothing under public/.
	if a.publicFiles(pub) != nil {
		t.Fatal("public folder before publish")
	}
	rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, "")
	var s map[string]any
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &s) != nil || s["visibility"] != "unlisted" || s["view_url"] != "/w/"+pub || len(s) != 11 {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	row := a.rows()[0]
	if row["visibility"] != "unlisted" || row["published_at"] == nil {
		t.Errorf("row %v", row)
	}
	counted("publish")
	page := a.page(pub)
	name := dataName(t, page)
	files := a.publicFiles(pub)
	preview := previewName(t, page)
	if !slices.Equal(files, []string{name, name + ".br", "index.html", preview}) {
		t.Errorf("files %v", files)
	}
	// AC 2: summary and laps in the markup, the robots meta.
	for _, want := range []string{
		`<meta name="robots" content="noindex, nofollow, noarchive"`,
		`data-k="distance"`, `<span class="n">13.46</span>`, `<th data-k="avg_power">Power (W)</th>`, `<td>2.24</td>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	// F-23 AC 4 (PC-7): every OG/Twitter tag before the preload, the image URL absolute.
	head, _, _ := strings.Cut(page, `<link rel="preload"`)
	for _, want := range []string{
		`<meta property="og:title" content="Ride, 23 Jun 2026" />`,
		`<meta property="og:description" content="Distance 13.46 km · Moving time 29:59 · Elevation gain 104 m · Avg speed 26.9 km/h · Avg heart rate 142 bpm" />`,
		`<meta property="og:image" content="` + testBaseURL + `/w/` + pub + `/` + preview + `" />`,
		`<meta property="og:image:width" content="1200" />`, `<meta property="og:image:height" content="630" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("head lacks %q", want)
		}
	}
	// AC 10 (OQ-69).
	noOriginalUnderPublic(t, dir, orig)

	// Publishing again rebuilds the same names and keeps the first publish time.
	first := row["published_at"]
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatalf("publish again: %d", rec.Code)
	}
	if !slices.Equal(a.publicFiles(pub), files) || a.rows()[0]["published_at"] != first {
		t.Error("second publish changed the names or the publish time")
	}
	if n := strings.Count(a.logs.String(), `"msg":"workout published"`); n != 1 {
		t.Errorf("%d publish lines", n)
	}

	// AC 3: a title change rewrites index.html only.
	info, _ := os.Stat(filepath.Join(dir, "public", pub, name))
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"title":"<script>alert(1)</script>"}`); rec.Code != 200 {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body)
	}
	page = a.page(pub)
	if dataName(t, page) != name || !strings.Contains(page, "<title>&lt;script&gt;alert(1)&lt;/script&gt;</title>") || strings.Contains(page, "<script>alert") ||
		!strings.Contains(page, `<meta property="og:title" content="&lt;script&gt;alert(1)&lt;/script&gt;" />`) {
		t.Error("title change")
	}
	if after, _ := os.Stat(filepath.Join(dir, "public", pub, name)); !after.ModTime().Equal(info.ModTime()) {
		t.Error("data file rewritten by a title change")
	}
	// The name is in the image (F-23): a new preview, the old one gone.
	titled := previewName(t, page)
	if got := a.publicFiles(pub); titled == preview || !slices.Equal(got, []string{name, name + ".br", "index.html", titled}) {
		t.Errorf("title change: %v, preview was %s", got, preview)
	}
	// A lifetime change reaches the page (F-21 AC 7).
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"lifetime_days":30}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	exp := time.Unix(a.rows()[0]["expires_at"].(int64), 0).UTC().Format(time.RFC3339)
	if !strings.Contains(a.page(pub), `datetime="`+exp+`"`) {
		t.Error("expiry not in the page")
	}
	if previewName(t, a.page(pub)) != titled {
		t.Error("a lifetime change wrote a new preview")
	}
	// A description is not in the image: same preview (PC-2).
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"description":"a new description"}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if p := a.page(pub); !strings.Contains(p, "a new description") || previewName(t, p) != titled {
		t.Error("a description change wrote a new preview or did not reach the page")
	}
	counted("edits")

	noOriginalUnderPublic(t, dir, orig)

	// AC 4 at the API, F-16: turning hiding on is a hides-more change (remove
	// first); it writes a new data file and the old one is gone.
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"hide_ends":true}`); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if got := a.publicFiles(pub); len(got) != 4 || !slices.Contains(got, "index.html") || slices.ContainsFunc(got, func(n string) bool {
		return n != "index.html" && slices.Contains(files, n)
	}) {
		t.Errorf("hiding on: %v, was %v", got, files)
	}
	counted("hiding on")

	// Nothing in the log names the workout publicly.
	if strings.Contains(a.logs.String(), pub) || strings.Contains(a.logs.String(), "public/") {
		t.Errorf("log: %s", a.logs.String())
	}
}

// TestPublishWhileFull (F-10 AC 4): publishing while uploads are paused
// succeeds, and its bytes are counted.
func TestPublishWhileFull(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, token, _ := a.upload("ride-30min.fit")
	a.st.Add(a.st.Limit()) // full
	if a.uploadsOpen() {
		t.Fatal("open")
	}
	before := a.st.Used()
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 200 {
		t.Fatalf("publish while full: %d", rec.Code)
	}
	if got := a.st.Used() - before; got != diskUsage(t, filepath.Join(dir, "public")) || got == 0 {
		t.Errorf("counted %d", got)
	}
}

// TestPublishNotFound: the publish endpoint answers strangers as every owner
// endpoint does (ADR-025), and a draft stays without a public folder.
func TestPublishNotFound(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	for _, c := range []struct{ path, token string }{
		{"/api/workouts/" + pub + "/publish", ""},
		{"/api/workouts/" + pub + "/publish", strings.Repeat("x", 22)},
		{"/api/workouts/" + pub + "/publish?token=" + token, token},
		{"/api/workouts/" + strings.Repeat("z", 22) + "/publish", token},
	} {
		if rec := a.owner("POST", c.path, c.token, ""); rec.Code != 404 || rec.Body.String() != ownerNotFound {
			t.Errorf("%s: %d %s", c.path, rec.Code, rec.Body)
		}
	}
	if a.publicFiles(pub) != nil || a.rows()[0]["visibility"] != "draft" {
		t.Error("published by a stranger")
	}
}

// TestPublishFailureLogs (P-14, privacy row): a file-system failure in the
// hook and in Delete gives a 500 whose log line has no public_id or public path.
func TestPublishFailureLogs(t *testing.T) {
	a := startApp(t, settings{})
	pub, token, _ := a.upload("ride-30min.fit")
	// A file where the folder belongs: every write into it fails.
	if err := os.WriteFile(filepath.Join(a.dir, "public", pub), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if rec := a.owner("POST", "/api/workouts/"+pub+"/publish", token, ""); rec.Code != 500 {
		t.Errorf("publish: %d", rec.Code)
	}
	if rec := a.owner("PATCH", "/api/workouts/"+pub, token, `{"title":"x"}`); rec.Code != 500 {
		t.Errorf("patch: %d", rec.Code)
	}
	if rec := a.owner("DELETE", "/api/workouts/"+pub, token, ""); rec.Code != 500 {
		t.Errorf("delete: %d", rec.Code)
	}
	logs := a.logs.String()
	if n := strings.Count(logs, `"msg":"owner request failed"`); n != 3 {
		t.Errorf("%d failure lines: %s", n, logs)
	}
	if strings.Contains(logs, pub) || strings.Contains(logs, "public/") || strings.Contains(logs, filepath.Join(a.dir, "public")) {
		t.Errorf("log: %s", logs)
	}
}

// TestRecoverPublic (P-8): at startup, public/ keeps only the folders of live
// unlisted rows, without temporary files.
func TestRecoverPublic(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	live, liveToken, _ := a.upload("ride-30min.fit")
	other, otherToken, _ := a.upload("run-30min.fit")
	for _, w := range [][2]string{{live, liveToken}, {other, otherToken}} {
		if rec := a.owner("POST", "/api/workouts/"+w[0]+"/publish", w[1], ""); rec.Code != 200 {
			t.Fatal(rec.Code)
		}
	}
	files := a.publicFiles(live)
	// Another process made one a draft without its hook; a crash left a
	// temporary file and strangers left entries.
	a.sql("UPDATE workouts SET visibility = 'draft', published_at = NULL WHERE public_id = ?", other)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "public", live, ".index.html.tmp"), []byte("x"), 0o640))
	must(os.MkdirAll(filepath.Join(dir, "public", strings.Repeat("Q", 22)), 0o750))
	must(os.WriteFile(filepath.Join(dir, "public", "stray"), []byte("x"), 0o640))
	a.close()

	b := openStorageApp(t, dir, 0)
	if got := b.entries("public"); !slices.Equal(got, []string{live}) {
		t.Errorf("public/ = %v", got)
	}
	if got := b.publicFiles(live); !slices.Equal(got, files) {
		t.Errorf("live folder %v, was %v", got, files)
	}
	if !strings.Contains(b.logs.String(), `"public_removed":4`) {
		t.Errorf("log: %s", b.logs.String())
	}
}

// TestMigrationDropsPrivate (ADR-019 item 8, F-20 B-1): a v2 database with a
// private row starts as v3 without that row, and recovery removes its private
// folder. The draft is kept with its folder.
func TestMigrationDropsPrivate(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	kept, _, _ := a.upload("ride-30min.fit")
	priv, _, _ := a.upload("run-30min.fit")
	ids := map[string]string{}
	for _, r := range a.rows() {
		ids[r["public_id"].(string)] = r["id"].(string)
	}
	a.close()

	// Back to schema v2, then the row made private as the old API did.
	var v2 []string
	for _, f := range []string{"0001_workouts.sql", "0002_hidden_ranges.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "internal", "store", "migrations", f))
		if err != nil {
			t.Fatal(err)
		}
		v2 = append(v2, string(b))
	}
	a.sql("DROP INDEX workouts_expires_at")
	a.sql("ALTER TABLE workouts RENAME TO v3")
	a.sql(strings.Join(v2, "\n"))
	a.sql("INSERT INTO workouts SELECT * FROM v3")
	a.sql("DROP TABLE v3")
	a.sql("UPDATE workouts SET visibility = 'private', published_at = created_at WHERE public_id = ?", priv)
	a.sql("PRAGMA user_version = 2")

	b := openStorageApp(t, dir, 0)
	if rows := b.rows(); len(rows) != 1 || rows[0]["public_id"] != kept || rows[0]["visibility"] != "draft" {
		t.Errorf("rows %v", rows)
	}
	if got := b.entries("private"); !slices.Equal(got, []string{ids[kept]}) {
		t.Errorf("private/ = %v, want only %s", got, ids[kept])
	}
	if b.entries("public") != nil {
		t.Errorf("public/ = %v", b.entries("public"))
	}
}
