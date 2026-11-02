package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite" // tests read the database directly; production code reaches it only through internal/store

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/httpapi"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/privatedir"
	"workout-explorer/internal/store"
	"workout-explorer/internal/workout"
)

var synthetic = filepath.Join("..", "..", "testdata", "corpus", "synthetic")

// testWeb holds the source page template: Go tests stay independent of the
// web build (F-15 P-10).
var testWeb = filepath.Join("..", "..", "..", "web")

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(synthetic, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type app struct {
	t    *testing.T
	h    http.Handler
	dir  string
	logs *syncBuffer
}

func startApp(t *testing.T, s settings) *app {
	t.Helper()
	if s.maxConcurrentUploads == 0 {
		s.maxConcurrentUploads = 4
	}
	if s.webDir == "" {
		s.webDir = testWeb
	}
	if s.baseURL == "" {
		s.baseURL = testBaseURL
	}
	a := &app{t: t, dir: filepath.Join(t.TempDir(), ".wx-data") /* dot folder: Caddy must still serve it (QA-22-r1-1) */, logs: &syncBuffer{}}
	logger := slog.New(slog.NewJSONHandler(a.logs, nil))
	h, closeApp, err := newApp(context.Background(), a.dir, s, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeApp() })
	a.h = h
	return a
}

// post sends file as the multipart field "file" from the given peer.
func (a *app) post(file []byte, remote, xff, ua, fileName string) *httptest.ResponseRecorder {
	a.t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+fileName+`"`)
	h.Set("Content-Type", "application/octet-stream")
	part, _ := w.CreatePart(h)
	part.Write(file)
	w.Close()
	return a.postRaw(&b, w.FormDataContentType(), remote, xff, ua)
}

func (a *app) postRaw(body io.Reader, ctype, remote, xff, ua string) *httptest.ResponseRecorder {
	a.t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/workouts", body)
	r.Header.Set("Content-Type", ctype)
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	if ua != "" {
		r.Header.Set("User-Agent", ua)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, r)
	return rec
}

func (a *app) upload(name string) (publicID, token string, rec *httptest.ResponseRecorder) {
	a.t.Helper()
	rec = a.post(readFile(a.t, name), "198.51.100.1:1000", "", "", name)
	if rec.Code != http.StatusCreated {
		return "", "", rec
	}
	var got struct{ ViewURL, EditURL string }
	var raw map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		a.t.Fatal(err)
	}
	got.ViewURL, got.EditURL = raw["view_url"], raw["edit_url"]
	publicID = strings.TrimPrefix(got.ViewURL, "/w/")
	_, token, _ = strings.Cut(got.EditURL, "#")
	return publicID, token, rec
}

// rows opens the database separately and returns the rows as maps.
func (a *app) rows() []map[string]any {
	a.t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(a.dir, "workouts.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		a.t.Fatal(err)
	}
	defer db.Close()
	rs, err := db.Query("SELECT * FROM workouts")
	if err != nil {
		a.t.Fatal(err)
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	var out []map[string]any
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			a.t.Fatal(err)
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out
}

// dataFiles returns every file under the data directory with its bytes.
func (a *app) dataFiles() map[string][]byte {
	a.t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(a.dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		out[p] = b
		return rerr
	})
	if err != nil {
		a.t.Fatal(err)
	}
	return out
}

func (a *app) entries(sub string) []string {
	a.t.Helper()
	es, err := os.ReadDir(filepath.Join(a.dir, sub))
	if err != nil {
		a.t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func (a *app) privateFile(id, name string) []byte {
	a.t.Helper()
	b, err := os.ReadFile(filepath.Join(a.dir, "private", id, name))
	if err != nil {
		a.t.Fatal(err)
	}
	return b
}

func parser() *ingest.Parser { return ingest.NewParser(ingest.DefaultLimits, nil) }

// TestUploadEndToEnd (AC 1, AC 2, AC 12).
func TestUploadEndToEnd(t *testing.T) {
	a := startApp(t, settings{})
	upload := readFile(t, "ride-30min.fit")
	pub, token, rec := a.upload("ride-30min.fit")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body) != 2 || body["view_url"] != "/w/"+pub || body["edit_url"] != "/edit/"+pub+"#"+token {
		t.Fatalf("body = %v", body)
	}
	if len(pub) != 22 || len(token) != 22 || strings.Contains(body["view_url"], token) {
		t.Errorf("public ID %q token %q", pub, token)
	}

	rows := a.rows()
	if len(rows) != 1 {
		t.Fatalf("%d rows", len(rows))
	}
	r := rows[0]
	if r["visibility"] != "draft" || r["published_at"] != nil || r["hide_ends"] != int64(0) || r["allow_download"] != int64(0) || r["data_end_s"] != nil {
		t.Errorf("row = %v", r)
	}
	if r["public_id"] != pub || r["title"] != "" || r["description"] != "" || r["sport"] != "cycling" {
		t.Errorf("row = %v", r)
	}
	if r["expires_at"].(int64)-r["created_at"].(int64) != 604_800 {
		t.Errorf("lifetime = %d s", r["expires_at"].(int64)-r["created_at"].(int64))
	}
	if now := time.Now().Unix(); r["created_at"].(int64) < now-60 || r["created_at"].(int64) > now+1 {
		t.Errorf("created_at = %v", r["created_at"])
	}
	hash := sha256.Sum256([]byte(token))
	stored := r["edit_token_hash"].([]byte)
	if !bytes.Equal(stored, hash[:]) || bytes.Equal(stored, []byte(token)) {
		t.Errorf("edit_token_hash is not the SHA-256 of the token (or is the token)")
	}

	id := r["id"].(string)
	original := a.privateFile(id, "original.fit")
	if !bytes.Equal(original, upload) {
		t.Errorf("original.fit differs from the upload although it has nothing to strip")
	}
	w, err := parser().Parse(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	wantBin, err := datafile.Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	draft := a.privateFile(id, "draft.bin")
	if !bytes.Equal(draft, wantBin) {
		t.Errorf("draft.bin is not Encode of a fresh parse of original.fit")
	}
	n := int(binary.LittleEndian.Uint32(draft[10:14]))
	var meta struct {
		Summary json.RawMessage `json:"summary"`
	}
	if err := json.Unmarshal(draft[14:14+n], &meta); err != nil {
		t.Fatal(err)
	}
	if r["summary_json"] != string(meta.Summary) {
		t.Errorf("summary_json = %v, want the data file's summary %s", r["summary_json"], meta.Summary)
	}
	if got := a.entries("private"); len(got) != 1 || got[0] != id {
		t.Errorf("private/ = %v", got)
	}
	if got := a.entries("tmp"); len(got) != 0 {
		t.Errorf("tmp/ = %v", got)
	}
	if got := a.entries("public"); len(got) != 0 { // a draft writes nothing public (F-15)
		t.Errorf("public/ = %v", got)
	}
	// The token is nowhere: not in the log, not in any file under the data directory.
	if strings.Contains(a.logs.String(), token) {
		t.Errorf("token in the log")
	}
	for p, b := range a.dataFiles() {
		if bytes.Contains(b, []byte(token)) {
			t.Errorf("token in %s", p)
		}
	}
	// The log line of the accepted upload names the workout by its internal ID only.
	if !strings.Contains(a.logs.String(), `"msg":"upload accepted"`) || strings.Contains(a.logs.String(), pub) {
		t.Errorf("log: %s", a.logs.String())
	}
}

// identifying markers (see the ingest tests).
func identifyingMarkers() [][]byte {
	out := [][]byte{[]byte("Marker Person"), []byte("Marker Home Place"), []byte("Marker Rival Person"), []byte("MkrGid"),
		[]byte("MkrAntRx"), []byte("MkrAntTx"), []byte("MarkerLeaderOne"), []byte("MarkerLeaderTwo")} // v1.4
	for _, n := range []uint32{3_735_928_559, 2_882_400_001, 2_882_400_002, 2_882_400_003, 424_242,
		// v1.4: segment device ID, the segment account keys (OQ-52), leader keys, tank sensor.
		2_882_400_004, 2_882_400_005, 2_882_400_006, 2_882_400_007, 2_882_400_008, 2_882_400_009, 2_882_400_010, 2_882_400_011} {
		out = append(out, []byte(strconv.FormatUint(uint64(n), 10)), binary.LittleEndian.AppendUint32(nil, n))
	}
	for _, n := range []uint16{54_321, 12_345, 23_456} {
		out = append(out, []byte(strconv.Itoa(int(n))), binary.LittleEndian.AppendUint16(nil, n))
	}
	return out
}

// TestUploadIdentifyingData (AC 6).
func TestUploadIdentifyingData(t *testing.T) {
	a := startApp(t, settings{})
	upload := readFile(t, "identifying-data.fit")
	if _, _, rec := a.upload("identifying-data.fit"); rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	acc, err := parser().Accept(context.Background(), upload)
	if err != nil {
		t.Fatal(err)
	}
	id := a.rows()[0]["id"].(string)
	if !bytes.Equal(a.privateFile(id, "original.fit"), acc.Original) {
		t.Errorf("original.fit is not Accept(upload).Original")
	}
	files := a.dataFiles()
	if len(files) < 4 {
		t.Fatalf("only %d files: %v", len(files), files)
	}
	for _, m := range identifyingMarkers() {
		// The 2-byte ANT forms are searched in the FIT file only: the other
		// files hold random IDs, so a 2-byte pattern there is chance.
		for p, b := range files {
			if len(m) < 4 && filepath.Base(p) != "original.fit" {
				continue
			}
			if bytes.Contains(b, m) {
				t.Errorf("marker %q in %s", m, p)
			}
		}
		if len(m) >= 5 && strings.Contains(a.logs.String(), string(m)) {
			t.Errorf("marker %q in the log", m)
		}
	}
	// The log carries counts only.
	if !strings.Contains(a.logs.String(), `"blanked_fields":16`) || !strings.Contains(a.logs.String(), `"removed_messages":5`) {
		t.Errorf("log lacks the strip counts: %s", a.logs.String())
	}
}

func reportZeroed(t *testing.T, b []byte) []byte {
	t.Helper()
	w, err := parser().Parse(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	w.Integrity.CRCFailed, w.Integrity.IgnoredBytes = false, 0
	r, err := workout.ReportJSON(w)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestUploadFraming (AC 7).
func TestUploadFraming(t *testing.T) {
	base := readFile(t, "ride-30min.fit")
	for _, name := range []string{"ride-30min-trailing-zeros.fit", "ride-30min-bad-crc.fit", "ride-30min-size-short.fit"} {
		t.Run(name, func(t *testing.T) {
			a := startApp(t, settings{})
			if _, _, rec := a.upload(name); rec.Code != http.StatusCreated {
				t.Fatalf("status %d", rec.Code)
			}
			got := a.privateFile(a.rows()[0]["id"].(string), "original.fit")
			if !bytes.Equal(got, base) {
				t.Errorf("original.fit differs from the clean file")
			}
			if r := a.rows()[0]; r["data_end_s"] != nil {
				t.Errorf("data_end_s = %v, want NULL", r["data_end_s"])
			}
			if !bytes.Equal(reportZeroed(t, got), reportZeroed(t, readFile(t, name))) {
				t.Errorf("report differs")
			}
		})
	}
	for _, name := range []string{"ride-30min-cut-60.fit", "ride-30min-cut-60-size-0.fit", "ride-30min-bad-basetype.fit"} {
		t.Run(name, func(t *testing.T) {
			a := startApp(t, settings{})
			upload := readFile(t, name)
			if _, _, rec := a.upload(name); rec.Code != http.StatusCreated {
				t.Fatalf("status %d", rec.Code)
			}
			r := a.rows()[0]
			got := a.privateFile(r["id"].(string), "original.fit")
			// The size-0 form of the upload, up to its last complete record.
			if len(got) >= len(upload) {
				t.Errorf("the incomplete record was not dropped: %d >= %d", len(got), len(upload))
			}
			if binary.LittleEndian.Uint32(got[4:8]) != 0 || got[12] != 0 || got[13] != 0 {
				t.Errorf("not in the unclosed form")
			}
			if !bytes.Equal(got[14:], upload[14:14+len(got)-14]) || !bytes.Equal(got[8:12], upload[8:12]) {
				t.Errorf("the records are not a prefix of the upload's")
			}
			if name == "ride-30min-bad-basetype.fit" {
				// D-20: the upload up to the inserted definition, exclusive.
				ins := 14
				for base[ins] == upload[ins] {
					ins++
				}
				if len(got) != ins {
					t.Errorf("original.fit is %d bytes, want %d (up to the inserted definition)", len(got), ins)
				}
			}
			w, err := parser().Parse(context.Background(), got)
			if err != nil || !w.Integrity.Truncated {
				t.Fatalf("stored file: %v, truncated %v", err, w.Integrity.Truncated)
			}
			up, _ := parser().Parse(context.Background(), upload)
			if r["data_end_s"] != int64(up.Integrity.DataEnd) || w.Integrity.DataEnd != up.Integrity.DataEnd {
				t.Errorf("data_end_s = %v, want %d", r["data_end_s"], up.Integrity.DataEnd)
			}
			if !bytes.Equal(reportZeroed(t, got), reportZeroed(t, upload)) {
				t.Errorf("report differs")
			}
		})
	}
}

// TestUploadRejectedLeavesNothing (AC 11).
func TestUploadRejectedLeavesNothing(t *testing.T) {
	for _, tc := range []struct{ file, message string }{
		{"not-fit-png.fit", "This file is not a FIT file."},
		{"course.fit", "Unsupported FIT file type."},
		{"ride-30min-header-only.fit", "This file contains no readable workout data."},
	} {
		t.Run(tc.file, func(t *testing.T) {
			a := startApp(t, settings{})
			before := strings.Count(a.logs.String(), "\n")
			_, _, rec := a.upload(tc.file)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var b struct{ Error, Message string }
			json.Unmarshal(rec.Body.Bytes(), &b)
			if b.Message != tc.message || b.Error == "" {
				t.Errorf("body = %+v", b)
			}
			if len(a.rows()) != 0 || len(a.entries("private")) != 0 || len(a.entries("tmp")) != 0 {
				t.Errorf("rows %d private %v tmp %v", len(a.rows()), a.entries("private"), a.entries("tmp"))
			}
			if n := strings.Count(a.logs.String(), "\n") - before; n != 1 || !strings.Contains(a.logs.String(), `"msg":"fit rejected"`) {
				t.Errorf("%d new log lines: %s", n, a.logs.String())
			}
		})
	}
}

// TestUploadEmptySequence (F-09 AC 14): a crafted chained file with an empty
// complete sequence is refused through the real upload path, and leaves no row
// and no file. The bytes are the committed synthetic fuzz seed, so the fuzz
// target and this test share one input (QA-09-r4-4).
func TestUploadEmptySequence(t *testing.T) {
	seed, err := os.ReadFile(filepath.Join("..", "..", "internal", "ingest", "testdata", "fuzz", "FuzzAccept", "empty-complete-seq"))
	if err != nil {
		t.Fatal(err)
	}
	_, quoted, ok := strings.Cut(strings.TrimSpace(string(seed)), "\n")
	quoted = strings.TrimSuffix(strings.TrimPrefix(quoted, "[]byte("), ")")
	data, uerr := strconv.Unquote(quoted)
	if !ok || uerr != nil {
		t.Fatalf("seed format: %v", uerr)
	}
	a := startApp(t, settings{})
	before := strings.Count(a.logs.String(), "\n")
	rec := a.post([]byte(data), "198.51.100.1:1000", "", "", "empty.fit")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var b struct{ Error, Message string }
	json.Unmarshal(rec.Body.Bytes(), &b)
	if b.Error != "unreadable" || b.Message != "This file could not be read." {
		t.Errorf("body = %+v", b)
	}
	if len(a.rows()) != 0 || len(a.entries("private")) != 0 || len(a.entries("tmp")) != 0 {
		t.Errorf("rows %d private %v tmp %v", len(a.rows()), a.entries("private"), a.entries("tmp"))
	}
	if n := strings.Count(a.logs.String(), "\n") - before; n != 1 || !strings.Contains(a.logs.String(), `"msg":"fit rejected"`) {
		t.Errorf("%d new log lines: %s", n, a.logs.String())
	}
}

// TestUploadUnreadable (AC 9).
func TestUploadUnreadable(t *testing.T) {
	a := startApp(t, settings{})
	const msg = "The upload could not be read. Please try again."
	check := func(name string, rec *httptest.ResponseRecorder) {
		t.Helper()
		var b struct{ Error, Message string }
		json.Unmarshal(rec.Body.Bytes(), &b)
		if rec.Code != 400 || b.Message != msg {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
		if len(a.rows()) != 0 || len(a.entries("private")) != 0 || len(a.entries("tmp")) != 0 {
			t.Errorf("%s: rows %d private %v tmp %v", name, len(a.rows()), a.entries("private"), a.entries("tmp"))
		}
	}
	check("malformed", a.postRaw(strings.NewReader("not a multipart body"), "multipart/form-data; boundary=xyz", "198.51.100.1:1", "", ""))
	// A body cut off mid-file: a reader that fails after half the file.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, _ := w.CreateFormFile("file", "x.fit")
	file := readFile(t, "ride-30min.fit")
	part.Write(file[:len(file)/2])
	check("cut off", a.postRaw(io.MultiReader(&b, failing{io.ErrUnexpectedEOF}), w.FormDataContentType(), "198.51.100.1:1", "", ""))
}

type failing struct{ err error }

func (f failing) Read([]byte) (int, error) { return 0, f.err }

// TestUploadServerError (AC 10).
func TestUploadServerError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	a := startApp(t, settings{})
	priv := filepath.Join(a.dir, "private")
	if err := os.Chmod(priv, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(priv, 0o700)
	_, _, rec := a.upload("ride-30min.fit")
	var b struct{ Error, Message string }
	json.Unmarshal(rec.Body.Bytes(), &b)
	if rec.Code != 500 || b.Error != "server_error" || b.Message != "Something went wrong on the server. Please try again later." {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(a.rows()) != 0 || len(a.entries("tmp")) != 0 || len(a.entries("private")) != 0 {
		t.Errorf("rows %d tmp %v private %v", len(a.rows()), a.entries("tmp"), a.entries("private"))
	}
	if !strings.Contains(a.logs.String(), `"msg":"upload failed"`) || !strings.Contains(a.logs.String(), `"stage":"files"`) {
		t.Errorf("log: %s", a.logs.String())
	}
}

// TestNoRequestMetadataStored (AC 5).
func TestNoRequestMetadataStored(t *testing.T) {
	a := startApp(t, settings{})
	file := readFile(t, "ride-30min.fit")
	v6 := "2001:db8:1234:5678::abcd"
	v6bytes := []byte{0x20, 0x01, 0x0d, 0xb8, 0x12, 0x34, 0x56, 0x78, 0, 0, 0, 0, 0, 0, 0xab, 0xcd}
	for _, tc := range []struct{ remote, xff string }{
		{"127.0.0.1:4242", "203.0.113.77"},
		{"127.0.0.1:4243", v6},
		{"[" + v6 + "]:5000", ""},
		{"203.0.113.88:5001", ""},
	} {
		if rec := a.post(file, tc.remote, tc.xff, "MARKER-USER-AGENT/1.0", "MARKER-FILE-NAME.fit"); rec.Code != http.StatusCreated {
			t.Fatalf("%v: %d %s", tc, rec.Code, rec.Body)
		}
	}
	needles := [][]byte{[]byte("203.0.113.77"), []byte("203.0.113.88"), []byte(v6), v6bytes, []byte("2001:db8"), []byte("MARKER-USER-AGENT"), []byte("MARKER-FILE-NAME"), []byte("4242")}
	for p, b := range a.dataFiles() {
		for _, n := range needles[:7] {
			if bytes.Contains(b, n) {
				t.Errorf("%q in %s", n, p)
			}
		}
	}
	for _, n := range needles {
		if strings.Contains(a.logs.String(), string(n)) {
			t.Errorf("%q in the log", n)
		}
	}
	if len(a.rows()) != 4 {
		t.Errorf("%d rows", len(a.rows()))
	}
}

// blockingUploads holds uploads inside the service's place until released.
type blockingUploads struct {
	inner   httpapi.Uploader
	entered chan struct{}
	release chan struct{}
}

func (b *blockingUploads) Upload(ctx context.Context, upload []byte) (lifecycle.Created, error) {
	b.entered <- struct{}{}
	<-b.release
	return b.inner.Upload(ctx, upload)
}

// TestSettingsReachTheHandler (AC 8): newApp with 2 refuses the third
// concurrent upload; the default of 4 the fifth.
func TestSettingsReachTheHandler(t *testing.T) {
	for _, n := range []int{2, 4} {
		blocker := &blockingUploads{entered: make(chan struct{}, 8), release: make(chan struct{})}
		a := startApp(t, settings{maxConcurrentUploads: n, wrapUploader: func(u httpapi.Uploader) httpapi.Uploader {
			blocker.inner = u
			return blocker
		}})
		file := readFile(t, "ride-30min.fit")
		var wg sync.WaitGroup
		codes := make(chan int, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); codes <- a.post(file, "198.51.100.1:1", "", "", "x.fit").Code }()
		}
		for i := 0; i < n; i++ {
			select {
			case <-blocker.entered:
			case <-time.After(5 * time.Second):
				t.Fatalf("N=%d: upload %d did not arrive", n, i)
			}
		}
		rec := a.post(file, "198.51.100.1:1", "", "", "x.fit")
		var b struct{ Message string }
		json.Unmarshal(rec.Body.Bytes(), &b)
		if rec.Code != http.StatusServiceUnavailable || b.Message != "Too many uploads right now. Please try again in a minute." {
			t.Errorf("N=%d: extra upload: %d %s", n, rec.Code, rec.Body)
		}
		close(blocker.release)
		wg.Wait()
		for i := 0; i < n; i++ {
			if c := <-codes; c != http.StatusCreated {
				t.Errorf("N=%d: held upload: %d", n, c)
			}
		}
	}
}

// TestServeRecovers: orphans and staging are gone after newApp; a folder with a row stays;
// (v1.4) a row whose folder is gone is deleted.
func TestServeRecovers(t *testing.T) {
	dir := t.TempDir()
	orphan := strings.Repeat("a", 32)
	kept := strings.Repeat("b", 32)
	lost := strings.Repeat("d", 32) // a row whose folder was removed outside the process
	for _, p := range []string{filepath.Join("private", orphan), filepath.Join("private", kept), filepath.Join("tmp", "x")} {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p, "f"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := store.Open(context.Background(), filepath.Join(dir, "workouts.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	for id, pub := range map[string]string{kept: strings.Repeat("c", 22), lost: strings.Repeat("e", 22)} {
		err = db.InsertWorkout(context.Background(), lifecycle.Record{
			ID: id, PublicID: pub, Visibility: lifecycle.Draft, SummaryJSON: "{}", Sport: "x",
			CreatedAt: now, ExpiresAt: now.Add(lifecycle.DefaultLifetime),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	logs := &syncBuffer{}
	_, closeApp, err := newApp(context.Background(), dir, settings{maxConcurrentUploads: 4, webDir: testWeb, baseURL: testBaseURL}, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeApp()
	db, err = store.Open(context.Background(), filepath.Join(dir, "workouts.db"))
	if err != nil {
		t.Fatal(err)
	}
	ids, err := db.WorkoutIDs(context.Background())
	db.Close()
	if _, ok := ids[lost]; err != nil || ok || len(ids) != 1 {
		t.Errorf("rows after recovery = %v, %v: the row without a folder must be gone, the other kept", ids, err)
	}
	for p, want := range map[string]bool{filepath.Join("private", orphan): false, filepath.Join("private", kept): true, filepath.Join("tmp", "x"): false} {
		_, err := os.Stat(filepath.Join(dir, p))
		if (err == nil) != want {
			t.Errorf("%s exists = %v, want %v", p, err == nil, want)
		}
	}
	if !strings.Contains(logs.String(), `"msg":"recovery"`) || !strings.Contains(logs.String(), `"orphans_removed":1`) ||
		!strings.Contains(logs.String(), `"rows_removed":1`) {
		t.Errorf("log: %s", logs.String())
	}
}

// ---- TestUploadCancelDuringInsert (v1.4, QA-09-r1-1, D-18) ----------------------

// cancellingRepo wraps the real store. Its InsertWorkout cancels the current
// upload's context from another goroutine after a seeded random 0 to 100 us
// and then runs the real insert, which is the race of QA-09-r1-1: the driver
// can report ctx.Err() for an INSERT that has committed.
type cancellingRepo struct {
	*store.DB
	mu     sync.Mutex
	rng    *rand.Rand
	cancel context.CancelFunc // of the upload in flight; set by the test before each upload
	wg     sync.WaitGroup
}

func (r *cancellingRepo) InsertWorkout(ctx context.Context, rec lifecycle.Record) error {
	r.mu.Lock()
	delay := time.Duration(r.rng.IntN(100_001)) * time.Nanosecond // 0 to 100 us
	cancel := r.cancel
	r.mu.Unlock()
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		time.Sleep(delay)
		cancel()
	}()
	return r.DB.InsertWorkout(ctx, rec)
}

type fixedParser struct{ acc ingest.Accepted }

func (p fixedParser) Accept(context.Context, []byte) (ingest.Accepted, error) { return p.acc, nil }

func TestUploadCancelDuringInsert(t *testing.T) {
	const uploads = 200
	dir := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(dir, "workouts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	files, err := privatedir.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	upload := readFile(t, "ride-30min.fit")
	acc, err := ingest.NewParser(ingest.Limits{}, nil).Accept(context.Background(), upload)
	if err != nil {
		t.Fatal(err)
	}
	repo := &cancellingRepo{DB: db, rng: rand.New(rand.NewPCG(20261016, 1))}
	svc := lifecycle.New(lifecycle.Deps{Parser: fixedParser{acc}, Repo: repo, Files: files})

	for i := range uploads {
		ctx, cancel := context.WithCancel(context.Background())
		repo.mu.Lock()
		repo.cancel = cancel
		repo.mu.Unlock()
		if _, err := svc.Upload(ctx, upload); err != nil {
			t.Errorf("upload %d: %v", i, err)
		}
		cancel()
	}
	repo.wg.Wait()

	rows, err := db.WorkoutIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	folders, unknown, err := files.IDs()
	if err != nil || unknown != 0 {
		t.Fatalf("IDs: %v, %d unknown", err, unknown)
	}
	if len(rows) != uploads || len(folders) != uploads {
		t.Errorf("%d rows and %d folders after %d uploads", len(rows), len(folders), uploads)
	}
	for _, id := range folders {
		if _, ok := rows[id]; !ok {
			t.Errorf("folder %s has no row", id)
		}
	}
	for id := range rows {
		if !slices.Contains(folders, id) {
			t.Errorf("row %s has no folder", id)
		}
	}
}
