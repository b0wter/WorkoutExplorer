package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
)

// F-30 end to end (architect/F-30-regenerate-command.md, Verification).
// Synthetic files only; the "new release" is a second web directory whose
// template names other assets, and the "old version" is simulated by
// renaming data files and making draft.bin unreadable.

// newWeb is a web directory whose page template names /assets/{asset}.
func newWeb(t *testing.T, asset string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testWeb, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("/src/main.ts")) {
		t.Fatal("template changed: no /src/main.ts to replace")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), bytes.ReplaceAll(b, []byte("/src/main.ts"), []byte("/assets/"+asset)), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type regenResult struct {
	code        int
	counts      map[string]string
	assets      []string
	out, stderr string
}

func runRegen(t *testing.T, ctx context.Context, dir, web string, each func(lifecycle.Outcome)) regenResult {
	t.Helper()
	var out, errb bytes.Buffer
	code := regenerate(ctx, dir, settings{webDir: web, baseURL: testBaseURL}, &out, &errb, each)
	r := regenResult{code: code, counts: map[string]string{}, out: out.String(), stderr: errb.String()}
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		k, v, _ := strings.Cut(l, "=")
		if k == "asset" {
			r.assets = append(r.assets, v)
		} else if k != "" {
			r.counts[k] = v
		}
	}
	return r
}

func (r regenResult) want(t *testing.T, code int, rebuilt, deleted, kept, skipped string) {
	t.Helper()
	want := map[string]string{"rebuilt": rebuilt, "deleted": deleted, "kept": kept, "skipped": skipped}
	if r.code != code || !reflect.DeepEqual(r.counts, want) {
		t.Fatalf("exit %d counts %v, want %d %v\n%s", r.code, r.counts, code, want, r.stderr)
	}
}

// makeOld turns a published page into one of an "old release": its data
// file (and .br) under another content-hashed name, and the page naming that
// file and the old asset. It returns the data file's bytes.
func makeOld(t *testing.T, a *app, pub string) []byte {
	t.Helper()
	page := a.page(pub)
	name := dataName(t, page)
	dir := filepath.Join(a.dir, "public", pub)
	bin, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	old := "data.0000000000000000.bin"
	for _, s := range []string{"", ".br"} {
		if err := os.Rename(filepath.Join(dir, name+s), filepath.Join(dir, old+s)); err != nil {
			t.Fatal(err)
		}
	}
	page = strings.ReplaceAll(page, name, old)
	page = strings.ReplaceAll(page, "/src/main.ts", "/assets/old-1.js")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page), 0o640); err != nil {
		t.Fatal(err)
	}
	return bin
}

// TestRegenerateFormatBump (AC 1, 3, 9; privacy): every live workout serves
// the new version and template, nothing of the old remains, settings are
// unchanged, and a second run writes nothing new.
func TestRegenerateFormatBump(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	hidden, token := a.published("ride-30min.fit")
	if rec := a.owner("PATCH", "/api/workouts/"+hidden, token, `{"hide_ends":true,"allow_download":true,"title":"T","lifetime_days":12}`); rec.Code != 200 {
		t.Fatalf("patch %d", rec.Code)
	}
	plain, _ := a.published("run-30min.fit")
	draft, _, _ := a.upload("triathlon.fit")
	draftID := idOf(a.app, draft)
	rowsBefore := a.rows()
	bins := map[string][]byte{hidden: makeOld(t, a.app, hidden), plain: makeOld(t, a.app, plain)}
	goodDraft := a.privateFile(draftID, "draft.bin")
	oldDraft := slices.Clone(goodDraft)
	oldDraft[4] ^= 0xFF // another format version
	if err := os.WriteFile(filepath.Join(dir, "private", draftID, "draft.bin"), oldDraft, 0o600); err != nil {
		t.Fatal(err)
	}

	web := newWeb(t, "new-1.js")
	r := runRegen(t, context.Background(), dir, web, nil)
	r.want(t, 0, "3", "0", "0", "0")
	if !slices.Equal(r.assets, []string{"new-1.js"}) {
		t.Errorf("assets %v", r.assets)
	}
	check := func() {
		t.Helper()
		for pub, bin := range bins {
			page := a.page(pub)
			name := dataName(t, page)
			if strings.Contains(page, "old-1.js") || !strings.Contains(page, `src="/assets/new-1.js"`) || strings.Contains(page, "data.0000000000000000") {
				t.Errorf("page not rebuilt with the new template")
			}
			if !slices.Equal(a.publicFiles(pub), []string{name, name + ".br", "index.html", previewName(t, page)}) {
				t.Errorf("files %v", a.publicFiles(pub))
			}
			// The same build as serve's (hiding included, F-16), and never the original's bytes (F-15 AC 10).
			got, _ := os.ReadFile(filepath.Join(dir, "public", pub, name))
			if !bytes.Equal(got, bin) {
				t.Error("data file differs from serve's build")
			}
			orig := a.privateFile(idOf(a.app, pub), "original.fit")
			for _, f := range a.publicFiles(pub) {
				b, _ := os.ReadFile(filepath.Join(dir, "public", pub, f))
				if bytes.Contains(b, orig[14:]) {
					t.Errorf("%s holds the original's bytes", f)
				}
			}
		}
		if _, err := datafile.Decode(a.privateFile(draftID, "draft.bin")); err != nil || !bytes.Equal(a.privateFile(draftID, "draft.bin"), goodDraft) {
			t.Errorf("draft.bin: %v", err)
		}
		if a.publicFiles(draft) != nil {
			t.Error("a draft got a public folder")
		}
		if !reflect.DeepEqual(a.rows(), rowsBefore) {
			t.Errorf("rows changed:\n%v\n%v", a.rows(), rowsBefore)
		}
		if es := a.entries("tmp"); len(es) != 0 {
			t.Errorf("tmp/ %v", es)
		}
	}
	check()

	// AC 3: a second run writes no new file and every page still works.
	snapshot := a.dataFiles()
	r = runRegen(t, context.Background(), dir, web, nil)
	r.want(t, 0, "3", "0", "0", "0")
	check()
	after := a.dataFiles()
	for p := range after {
		if _, ok := snapshot[p]; !ok && !strings.HasPrefix(filepath.Base(p), "workouts.db") {
			t.Errorf("second run wrote %s", p)
		}
	}
}

// TestRegenerateDeletesRejected (AC 4): an original the new version rejects
// is deleted with everything stored for it; the line names the device and no
// line holds the public_id, a path or a position; the others are rebuilt.
func TestRegenerateDeletesRejected(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	bad, token := a.published("ride-30min.fit")
	if rec := a.report(bad, "spam", "x", "198.51.100.7:1"); rec.Code != 204 && rec.Code != 201 && rec.Code != 200 {
		t.Fatalf("report %d", rec.Code)
	}
	ok, _ := a.published("run-30min.fit")
	badID := idOf(a.app, bad)
	if err := os.WriteFile(filepath.Join(dir, "private", badID, "original.fit"), readFile(t, "course.fit"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := runRegen(t, context.Background(), dir, testWeb, nil)
	r.want(t, 0, "1", "1", "0", "0")
	if left := dataLeft(t, dir, badID, bad); left != nil {
		t.Errorf("left %v", left)
	}
	if rec := a.owner("GET", "/api/workouts/"+bad, token, ""); rec.Code != 404 {
		t.Errorf("owner API %d", rec.Code)
	}
	if a.publicFiles(ok) == nil {
		t.Error("the other workout lost its page")
	}
	var rejected, deleted bool
	for _, l := range strings.Split(strings.TrimSpace(r.stderr), "\n") {
		if strings.Contains(l, `"msg":"fit rejected"`) && strings.Contains(l, `"id":"`+badID+`"`) && strings.Contains(l, `"product_name":"corpusgen"`) {
			rejected = true
		}
		if strings.Contains(l, `"msg":"workout deleted"`) && strings.Contains(l, `"by":"regenerate"`) && strings.Contains(l, badID) {
			deleted = true
		}
	}
	if !rejected || !deleted {
		t.Errorf("log lines:\n%s", r.stderr)
	}
	for _, s := range []string{bad, ok, dir, "/private/", "/public/", "lat", "lon", "position"} {
		if strings.Contains(r.stderr, s) {
			t.Errorf("log holds %q", s)
		}
	}
}

// TestRegenerateShorter (AC 5): a stored original the new version reads
// shorter is kept and serves the shorter workout.
func TestRegenerateShorter(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, _ := a.published("run-30min.fit")
	id := idOf(a.app, pub)
	before := a.page(pub)
	cut := readFile(t, "run-30min-cut-60.fit") // what a parser that stops earlier reads
	if err := os.WriteFile(filepath.Join(dir, "private", id, "original.fit"), cut, 0o600); err != nil {
		t.Fatal(err)
	}
	r := runRegen(t, context.Background(), dir, testWeb, nil)
	r.want(t, 0, "1", "0", "0", "0")
	acc, err := ingest.NewParser(ingest.DefaultLimits, nil).Accept(context.Background(), cut)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.privateFile(id, "original.fit"), acc.Original) {
		t.Error("original.fit is not Accept's output")
	}
	row := a.rows()[0]
	if row["data_end_s"] == nil || dataName(t, a.page(pub)) == dataName(t, before) {
		t.Errorf("not shorter: %v", row["data_end_s"])
	}
	want, _ := datafile.Encode(acc.Workout)
	if !bytes.Equal(a.privateFile(id, "draft.bin"), want) {
		t.Error("draft.bin is not the shorter workout")
	}
}

// TestRegenerateKeptOnServerFailure (AC 6, RG-4): a failed private write
// (a), row update (b) or public write (c) keeps the workout; at (a) row and
// files are unchanged; the next run rebuilds it.
func TestRegenerateKeptOnServerFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-only modes")
	}
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	pub, _ := a.published("ride-30min.fit")
	id := idOf(a.app, pub)
	makeOld(t, a.app, pub)
	web := newWeb(t, "new-1.js")
	for _, c := range []struct {
		name        string
		break_, fix func()
		privateSame bool
	}{
		{"(a) private", func() { os.Chmod(filepath.Join(dir, "private", id), 0o500) }, func() { os.Chmod(filepath.Join(dir, "private", id), 0o700) }, true},
		{"(b) row", func() {
			a.sql(`CREATE TRIGGER fail BEFORE UPDATE OF summary_json ON workouts BEGIN SELECT RAISE(ABORT, 'disk'); END`)
		}, func() { a.sql(`DROP TRIGGER fail`) }, false},
		{"(c) public", func() { os.Chmod(filepath.Join(dir, "public", pub), 0o550) }, func() { os.Chmod(filepath.Join(dir, "public", pub), 0o750) }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			draftPath := filepath.Join(dir, "private", id, "draft.bin")
			os.WriteFile(draftPath, []byte("old draft"), 0o600)
			rows, page, files := a.rows(), a.page(pub), a.publicFiles(pub)
			c.break_()
			r := runRegen(t, context.Background(), dir, web, nil)
			c.fix()
			r.want(t, 0, "0", "0", "1", "0")
			if !reflect.DeepEqual(a.rows(), rows) || a.page(pub) != page || !slices.Equal(a.publicFiles(pub), files) {
				t.Error("row or page changed")
			}
			if got := string(a.privateFile(id, "draft.bin")) == "old draft"; got != c.privateSame {
				t.Errorf("draft.bin unchanged: %v", got)
			}
			if !strings.Contains(r.stderr, `"msg":"workout kept"`) || strings.Contains(r.stderr, dir) || strings.Contains(r.stderr, pub) {
				t.Errorf("log:\n%s", r.stderr)
			}
			if es := a.entries("tmp"); len(es) != 0 {
				t.Errorf("tmp/ %v", es)
			}
			runRegen(t, context.Background(), dir, web, nil).want(t, 0, "1", "0", "0", "0")
			if !strings.Contains(a.page(pub), "new-1.js") || string(a.privateFile(id, "draft.bin")) == "old draft" {
				t.Error("the next run did not rebuild it")
			}
			makeOld(t, a.app, pub)
		})
	}
}

// TestRegenerateInterrupted (AC 7, RG-7): a stop after the first workout
// leaves the others unchanged, prints the report and exits 1; the next run
// converges.
func TestRegenerateInterrupted(t *testing.T) {
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	var pubs []string
	for _, f := range []string{"ride-30min.fit", "run-30min.fit", "pool-swim.fit"} {
		p, _ := a.published(f)
		makeOld(t, a.app, p)
		pubs = append(pubs, p)
	}
	snapshot := a.dataFiles()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	web := newWeb(t, "new-1.js")
	r := runRegen(t, ctx, dir, web, func(lifecycle.Outcome) { cancel() })
	r.want(t, 1, "1", "0", "0", "0")
	if r.assets != nil || !strings.Contains(r.stderr, "stopped before the end") {
		t.Errorf("assets %v\n%s", r.assets, r.stderr)
	}
	changed := 0
	for _, p := range pubs {
		if strings.Contains(a.page(p), "new-1.js") {
			changed++
		} else {
			// unprocessed: every file byte for byte as before
			for _, f := range a.publicFiles(p) {
				path := filepath.Join(dir, "public", p, f)
				if b, _ := os.ReadFile(path); !bytes.Equal(b, snapshot[path]) {
					t.Errorf("unprocessed workout changed: %s", f)
				}
			}
		}
	}
	if changed != 1 {
		t.Errorf("%d rebuilt", changed)
	}
	runRegen(t, context.Background(), dir, web, nil).want(t, 0, "3", "0", "0", "0")
	for _, p := range pubs {
		if !strings.Contains(a.page(p), "new-1.js") {
			t.Error("not converged")
		}
	}
}

// TestRegenerateCommand (RG-1): usage, settings and database checks before
// anything changes; dispatch and usage text.
func TestRegenerateCommand(t *testing.T) {
	env := map[string]string{envBaseURL: testBaseURL}
	getenv := func(k string) string { return env[k] }
	empty := t.TempDir()
	cmd := func(args ...string) (int, string) {
		var errb bytes.Buffer
		code := regenerateCmd(args, getenv, io.Discard, &errb)
		return code, errb.String()
	}
	for _, c := range []struct {
		args []string
		want int
	}{
		{nil, 2},
		{[]string{"-data", empty}, 2},
		{[]string{"-web", testWeb}, 2},
		{[]string{"-data", empty, "-web", testWeb, "x"}, 2},
		{[]string{"-data", empty, "-web", testWeb}, 1}, // no database: never created
	} {
		if code, errb := cmd(c.args...); code != c.want {
			t.Errorf("%v: %d %s", c.args, code, errb)
		}
	}
	env[envParseTimeout] = "4"
	if code, errb := cmd("-data", empty, "-web", testWeb); code != 2 || !strings.Contains(errb, "regenerate: "+envParseTimeout) {
		t.Errorf("timeout: %d %s", code, errb)
	}
	delete(env, envParseTimeout)
	delete(env, envBaseURL)
	if code, errb := cmd("-data", empty, "-web", testWeb); code != 2 || !strings.Contains(errb, envBaseURL) {
		t.Errorf("base URL: %d %s", code, errb)
	}
	if es, _ := os.ReadDir(empty); len(es) != 0 {
		t.Errorf("created %v", es)
	}
	// Another schema version is refused, not migrated.
	env[envBaseURL] = testBaseURL
	dir := t.TempDir()
	a := openStorageApp(t, dir, 0)
	a.published("ride-30min.fit")
	a.sql("PRAGMA user_version = 3")
	if code, errb := cmd("-data", dir, "-web", testWeb); code != 1 || !strings.Contains(errb, "is not this binary's") {
		t.Errorf("old schema: %d %s", code, errb)
	}
	var errb bytes.Buffer
	if code := run([]string{"regenerate"}, io.Discard, &errb); code != 2 {
		t.Errorf("run regenerate: %d", code)
	}
	errb.Reset()
	run(nil, io.Discard, &errb)
	if !strings.Contains(errb.String(), "regenerate ") {
		t.Errorf("usage %s", errb.String())
	}
}
