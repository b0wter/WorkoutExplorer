package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/textproto"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
)

// ---- helpers ------------------------------------------------------------------

type fakeUploader struct {
	mu    sync.Mutex
	calls int
	last  []byte
	fn    func(ctx context.Context, upload []byte) (lifecycle.Created, error)
}

func (u *fakeUploader) Upload(ctx context.Context, upload []byte) (lifecycle.Created, error) {
	u.mu.Lock()
	u.calls++
	u.last = upload
	fn := u.fn
	u.mu.Unlock()
	if fn != nil {
		return fn(ctx, upload)
	}
	return lifecycle.Created{PublicID: strings.Repeat("P", 22), EditToken: strings.Repeat("T", 22)}, nil
}

func (u *fakeUploader) count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls
}

type logLines struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logLines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logLines) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	h    http.Handler
	up   *fakeUploader
	logs *logLines
	clk  *clock
}

func newEnv(t *testing.T, n int) *env {
	t.Helper()
	e := &env{up: &fakeUploader{}, logs: &logLines{}, clk: &clock{t: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}}
	e.h = NewHandler(Config{Version: "t", Uploads: e.up, Logger: slog.New(slog.NewJSONHandler(e.logs, nil)), ParseTimeout: 10 * time.Second, MaxConcurrentUploads: n, Now: e.clk.now})
	return e
}

// multipartParts returns the body prefix and suffix around a file part.
func multipartParts(field string) (prefix, suffix []byte, ctype string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename="MARKER-FILE-NAME.fit"`, field))
	h.Set("Content-Type", "application/octet-stream")
	if _, err := w.CreatePart(h); err != nil {
		panic(err)
	}
	prefix = append([]byte(nil), b.Bytes()...)
	b.Reset()
	w.Close()
	return prefix, append([]byte(nil), b.Bytes()...), w.FormDataContentType()
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// zeros yields n zero bytes without allocating them.
type zeros struct{ left int64 }

func (z *zeros) Read(p []byte) (int, error) {
	if z.left <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.left {
		n = z.left
	}
	clear(p[:n])
	z.left -= n
	return int(n), nil
}

// bigRequest is a request whose file part is size zero bytes. contentLength
// < 0 leaves the length unknown. The body is read through the counter.
func bigRequest(size int64, setLength bool) (*http.Request, *countingReader) {
	prefix, suffix, ctype := multipartParts("file")
	body := &countingReader{r: io.MultiReader(bytes.NewReader(prefix), &zeros{size}, bytes.NewReader(suffix))}
	r := httptest.NewRequest(http.MethodPost, "/api/workouts", body)
	r.Header.Set("Content-Type", ctype)
	r.ContentLength = -1
	if setLength {
		r.ContentLength = int64(len(prefix)+len(suffix)) + size
	}
	r.RemoteAddr = "198.51.100.1:5555"
	return r, body
}

func smallRequest(content []byte, remote string, xff string) *http.Request {
	prefix, suffix, ctype := multipartParts("file")
	body := append(append(append([]byte(nil), prefix...), content...), suffix...)
	r := httptest.NewRequest(http.MethodPost, "/api/workouts", bytes.NewReader(body))
	r.Header.Set("Content-Type", ctype)
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func do(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

type errBody struct{ Error, Message string }

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) errBody {
	t.Helper()
	var b errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return b
}

const (
	msgTooLarge    = "This file is larger than 20 MB."
	msgRate        = "Too many uploads. Please try again later."
	msgBusy        = "Too many uploads right now. Please try again in a minute."
	msgUnreadable  = "The upload could not be read. Please try again."
	msgServerError = "Something went wrong on the server. Please try again later."
)

// ---- tests --------------------------------------------------------------------

func TestUploadCreated(t *testing.T) {
	e := newEnv(t, 4)
	rec := do(e.h, smallRequest([]byte("FIT"), "198.51.100.1:1", ""))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	var m map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	pub, tok := strings.Repeat("P", 22), strings.Repeat("T", 22)
	if len(m) != 2 || m["view_url"] != "/w/"+pub || m["edit_url"] != "/edit/"+pub+"#"+tok {
		t.Errorf("body = %v", m)
	}
	for k, v := range m {
		if strings.Contains(v, "?") {
			t.Errorf("%s has a query string", k)
		}
	}
	if strings.Contains(m["view_url"], tok) {
		t.Errorf("the viewing link holds the token")
	}
	if string(e.up.last) != "FIT" {
		t.Errorf("uploader got %q", e.up.last)
	}
	// The token appears in no log line (AC 2).
	if strings.Contains(e.logs.String(), tok) {
		t.Errorf("token in the log")
	}
}

// TestUploadSizeCap (AC 3).
func TestUploadSizeCap(t *testing.T) {
	t.Run("a: Content-Length past the body cap is refused before any byte is read", func(t *testing.T) {
		e := newEnv(t, 4)
		r, body := bigRequest(MaxUploadBytes+maxBodyBytes-MaxUploadBytes, true) // 20,016,384 bytes of file: more than the body cap
		rec := do(e.h, r)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status %d", rec.Code)
		}
		if b := decodeErr(t, rec); b.Error != "too_large" || b.Message != msgTooLarge {
			t.Errorf("body = %+v", b)
		}
		if n := body.n.Load(); n != 0 {
			t.Errorf("%d body bytes read, want 0", n)
		}
		if e.up.count() != 0 {
			t.Errorf("uploader called")
		}
	})
	t.Run("a2: 20,000,001 bytes with Content-Length is refused after reading the file part", func(t *testing.T) {
		// The multipart framing is far below the 16 KiB of slack, so Content-Length
		// cannot refuse this file early; the reader stops at MaxUploadBytes+1.
		e := newEnv(t, 4)
		r, body := bigRequest(MaxUploadBytes+1, true)
		rec := do(e.h, r)
		if rec.Code != http.StatusRequestEntityTooLarge || decodeErr(t, rec).Message != msgTooLarge {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if n := body.n.Load(); n > MaxUploadBytes+1+64<<10 {
			t.Errorf("%d bytes read", n)
		}
		if e.up.count() != 0 {
			t.Errorf("uploader called")
		}
	})
	t.Run("b: unknown length stops reading early", func(t *testing.T) {
		for _, size := range []int64{MaxUploadBytes + 1, 30_000_000} {
			e := newEnv(t, 4)
			r, body := bigRequest(size, false)
			rec := do(e.h, r)
			if rec.Code != http.StatusRequestEntityTooLarge || decodeErr(t, rec).Message != msgTooLarge {
				t.Fatalf("size %d: status %d: %s", size, rec.Code, rec.Body)
			}
			if n := body.n.Load(); n > maxBodyBytes+64<<10 {
				t.Errorf("size %d: %d bytes read, bound %d", size, n, maxBodyBytes+64<<10)
			}
			if size == 30_000_000 && body.n.Load() > 25_000_000 {
				t.Errorf("read %d of %d bytes: not early", body.n.Load(), size)
			}
			if e.up.count() != 0 {
				t.Errorf("uploader called")
			}
		}
	})
	t.Run("c: exactly 20,000,000 bytes are read in full", func(t *testing.T) {
		e := newEnv(t, 4)
		for _, known := range []bool{true, false} {
			r, _ := bigRequest(MaxUploadBytes, known)
			rec := do(e.h, r)
			if rec.Code != http.StatusCreated {
				t.Fatalf("known=%v: status %d: %s", known, rec.Code, rec.Body)
			}
			if len(e.up.last) != MaxUploadBytes {
				t.Errorf("uploader got %d bytes", len(e.up.last))
			}
		}
	})
}

// TestUploadShape (AC 9).
func TestUploadShape(t *testing.T) {
	prefix, suffix, ctype := multipartParts("file")
	otherPrefix, otherSuffix, otherType := multipartParts("other")
	two := append(append(append(append([]byte(nil), prefix...), "A"...), "\r\n--"+boundaryOf(ctype)+"\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\nB"...), suffix...)
	cases := []struct {
		name   string
		ctype  string
		body   []byte
		reader io.Reader
	}{
		{"not multipart", "application/octet-stream", []byte("FIT"), nil},
		{"no boundary", "multipart/form-data", append(append(append([]byte(nil), prefix...), "A"...), suffix...), nil},
		{"wrong field name", otherType, append(append(append([]byte(nil), otherPrefix...), "A"...), otherSuffix...), nil},
		{"second part", ctype, two, nil},
		{"broken multipart", ctype, []byte("this is not multipart at all"), nil},
		{"empty body", ctype, nil, nil},
		{"cut off mid-file", ctype, nil, io.MultiReader(bytes.NewReader(prefix), strings.NewReader("half a file"), failingReader{io.ErrUnexpectedEOF})},
		{"cut off without terminator", ctype, append(append([]byte(nil), prefix...), "abc"...), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, 4)
			var body io.Reader = bytes.NewReader(tc.body)
			if tc.reader != nil {
				body = tc.reader
			}
			r := httptest.NewRequest(http.MethodPost, "/api/workouts", body)
			r.Header.Set("Content-Type", tc.ctype)
			r.RemoteAddr = "198.51.100.1:1"
			rec := do(e.h, r)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if b := decodeErr(t, rec); b.Error != "bad_request" || b.Message != msgUnreadable {
				t.Errorf("body = %+v", b)
			}
			if e.up.count() != 0 {
				t.Errorf("uploader called")
			}
		})
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

func boundaryOf(ctype string) string {
	_, b, _ := strings.Cut(ctype, "boundary=")
	return b
}

// TestRateLimit (AC 4).
func TestRateLimit(t *testing.T) {
	post := func(e *env, remote, xff string) *httptest.ResponseRecorder {
		return do(e.h, smallRequest([]byte("FIT"), remote, xff))
	}
	t.Run("60 per hour per address, other addresses unaffected, rolling window", func(t *testing.T) {
		e := newEnv(t, 4)
		for i := 0; i < 60; i++ {
			if rec := post(e, "127.0.0.1:9", "203.0.113.7"); rec.Code != http.StatusCreated {
				t.Fatalf("request %d: %d", i+1, rec.Code)
			}
		}
		e.clk.advance(30 * time.Minute)
		rec := post(e, "127.0.0.1:9", "203.0.113.7")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("61st: status %d", rec.Code)
		}
		if b := decodeErr(t, rec); b.Error != "rate_limited" || b.Message != msgRate {
			t.Errorf("body = %+v", b)
		}
		if ra := rec.Header().Get("Retry-After"); ra != "1800" {
			t.Errorf("Retry-After = %q, want 1800", ra)
		}
		if e.up.count() != 60 {
			t.Errorf("uploader called %d times", e.up.count())
		}
		if rec := post(e, "127.0.0.1:9", "198.51.100.9"); rec.Code != http.StatusCreated {
			t.Errorf("another address: %d", rec.Code)
		}
		// Refused attempts do not count or extend the window: an hour after the first, it is allowed again.
		for range 5 {
			post(e, "127.0.0.1:9", "203.0.113.7")
		}
		e.clk.advance(30*time.Minute + time.Second)
		if rec := post(e, "127.0.0.1:9", "203.0.113.7"); rec.Code != http.StatusCreated {
			t.Errorf("after the window: %d", rec.Code)
		}
	})
	t.Run("IPv6 per /64", func(t *testing.T) {
		e := newEnv(t, 4)
		for i := 0; i < 60; i++ {
			from := "[2001:db8:1:2::a]:1"
			if i%2 == 1 {
				from = "[2001:db8:1:2:ffff::b]:1"
			}
			if rec := post(e, from, ""); rec.Code != http.StatusCreated {
				t.Fatalf("request %d: %d", i+1, rec.Code)
			}
		}
		if rec := post(e, "[2001:db8:1:2::c]:1", ""); rec.Code != http.StatusTooManyRequests {
			t.Errorf("same /64: %d", rec.Code)
		}
		if rec := post(e, "[2001:db8:1:3::a]:1", ""); rec.Code != http.StatusCreated {
			t.Errorf("another /64: %d", rec.Code)
		}
	})
	t.Run("rejected files count", func(t *testing.T) {
		e := newEnv(t, 4)
		e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) {
			return lifecycle.Created{}, &ingest.Error{Kind: ingest.KindNotFIT}
		}
		for i := 0; i < 60; i++ {
			if rec := post(e, "198.51.100.1:1", ""); rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("request %d: %d", i+1, rec.Code)
			}
		}
		if rec := post(e, "198.51.100.1:1", ""); rec.Code != http.StatusTooManyRequests {
			t.Errorf("61st: %d", rec.Code)
		}
	})
	t.Run("files over the cap count", func(t *testing.T) {
		e := newEnv(t, 4)
		for i := 0; i < 60; i++ {
			r, _ := bigRequest(maxBodyBytes, true)
			if rec := do(e.h, r); rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("request %d: %d", i+1, rec.Code)
			}
		}
		if rec := post(e, "198.51.100.1:5555", ""); rec.Code != http.StatusTooManyRequests {
			t.Errorf("61st: %d", rec.Code)
		}
	})
}

func TestClientPrefix(t *testing.T) {
	tests := []struct {
		name, remote, xff, want string
	}{
		{"non-loopback peer ignores the header", "198.51.100.5:1", "1.1.1.1", "198.51.100.5/32"},
		{"loopback takes the last entry", "127.0.0.1:1", "1.1.1.1, 2.2.2.2", "2.2.2.2/32"},
		{"loopback, one entry", "127.0.0.1:1", "2.2.2.2", "2.2.2.2/32"},
		{"loopback without header", "127.0.0.1:1", "", "127.0.0.1/32"},
		{"unparsable entry falls back to the peer", "127.0.0.1:1", "1.1.1.1, not-an-ip", "127.0.0.1/32"},
		{"IPv6 loopback", "[::1]:1", "2001:db8:1:2::9", "2001:db8:1:2::/64"},
		{"IPv6 peer", "[2001:db8:1:2:3:4:5:6]:1", "", "2001:db8:1:2::/64"},
		{"IPv4-mapped", "[::ffff:192.0.2.1]:1", "", "192.0.2.1/32"},
		{"mapped in the header", "127.0.0.1:1", "::ffff:192.0.2.1", "192.0.2.1/32"},
		{"zone is dropped", "127.0.0.1:1", "fe80::1%eth0", "fe80::/64"},
	}
	for _, tc := range tests {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := clientPrefix(r, netip.Addr{}).String(); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "[2001:db8:1:2::a]:1"
	r2 := httptest.NewRequest(http.MethodPost, "/", nil)
	r2.RemoteAddr = "[2001:db8:1:2::b]:1"
	r3 := httptest.NewRequest(http.MethodPost, "/", nil)
	r3.RemoteAddr = "[2001:db8:1:3::a]:1"
	seed := newUploadHandler(Config{ParseTimeout: 10 * time.Second, MaxConcurrentUploads: 1}).seed
	k1, k2, k3 := limiterKey(seed, clientPrefix(r, netip.Addr{})), limiterKey(seed, clientPrefix(r2, netip.Addr{})), limiterKey(seed, clientPrefix(r3, netip.Addr{}))
	if k1 != k2 || k1 == k3 {
		t.Errorf("keys %x %x %x", k1, k2, k3)
	}
}

// TestRateTableFull (v1.4, QA-09-r1-2). Every clock step stays under 10
// minutes since the last sweep, so the periodic sweep can never stand in for
// the full-table sweep. Whole-second clock, two keys.
func TestRateTableFull(t *testing.T) {
	type step struct {
		at      time.Duration // after T
		wantOK  bool
		warnsAt int // total onFull calls after this step
	}
	run := func(t *testing.T, window time.Duration, steps []step) {
		t.Helper()
		clk := &clock{t: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}
		T := clk.now()
		warns := 0
		l := newLimiter(window, 2, clk.now)
		l.onFull = func(int) { warns++ }
		for _, k := range []uint64{1, 2} {
			if ok, _, _ := l.allow(k, 60); !ok {
				t.Fatalf("key %d refused at T", k)
			}
		}
		for _, st := range steps {
			clk.advance(T.Add(st.at).Sub(clk.now()))
			ok, _, _ := l.allow(3, 60)
			if ok != st.wantOK || warns != st.warnsAt {
				t.Errorf("T+%v: allowed=%v (want %v), WARN lines=%d (want %d)", st.at, ok, st.wantOK, warns, st.warnsAt)
			}
		}
	}
	t.Run("a: window 1h, full of live keys", func(t *testing.T) {
		run(t, time.Hour, []step{
			{30 * time.Second, false, 1}, // refused, one WARN
			{59 * time.Second, false, 1}, // refused, no further WARN
			{61 * time.Second, false, 2}, // the full-table sweep ran (it re-arms the WARN) and found nothing to drop
		})
	})
	t.Run("b: window 30s, expired keys", func(t *testing.T) {
		run(t, 30*time.Second, []step{
			{59 * time.Second, false, 1}, // keys expired, but under a minute since T: no sweep yet
			{61 * time.Second, true, 1},  // the full-table sweep dropped them
		})
	})
}

// TestRateTableFullHandler is the earlier, handler-level form of the same
// behaviour: the WARN line is written by the handler and a known key still works.
func TestRateTableFullHandler(t *testing.T) {
	e := newEnv(t, 4)
	h := newUploadHandler(Config{Uploads: e.up, Logger: slog.New(slog.NewJSONHandler(e.logs, nil)), ParseTimeout: 10 * time.Second, MaxConcurrentUploads: 4, Now: e.clk.now})
	h.lim = newLimiter(time.Hour, 2, e.clk.now)
	h.lim.onFull = func(keys int) { h.log.Warn("rate table full", "keys", keys) }
	for i, remote := range []string{"198.51.100.1:1", "198.51.100.2:1", "198.51.100.3:1", "198.51.100.4:1"} {
		rec := do(h, smallRequest([]byte("x"), remote, ""))
		want := http.StatusCreated
		if i >= 2 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Errorf("request %d: %d, want %d", i, rec.Code, want)
		}
	}
	// A known key still works.
	if rec := do(h, smallRequest([]byte("x"), "198.51.100.1:1", "")); rec.Code != http.StatusCreated {
		t.Errorf("known key: %d", rec.Code)
	}
	if n := strings.Count(e.logs.String(), `"msg":"rate table full"`); n != 1 {
		t.Errorf("%d WARN lines, want 1: %s", n, e.logs)
	}
	// After the window the keys are swept and new ones fit.
	e.clk.advance(2 * time.Hour)
	if rec := do(h, smallRequest([]byte("x"), "198.51.100.9:1", "")); rec.Code != http.StatusCreated {
		t.Errorf("after sweep: %d", rec.Code)
	}
}

// blockingUploader holds uploads inside Upload until released.
type blockingUploader struct {
	entered chan struct{}
	release chan struct{}
}

func newBlocking() *blockingUploader {
	return &blockingUploader{entered: make(chan struct{}, 32), release: make(chan struct{})}
}

func (b *blockingUploader) Upload(ctx context.Context, _ []byte) (lifecycle.Created, error) {
	b.entered <- struct{}{}
	select {
	case <-b.release:
	case <-ctx.Done():
	}
	return lifecycle.Created{PublicID: strings.Repeat("P", 22), EditToken: strings.Repeat("T", 22)}, nil
}

func waitEntered(t *testing.T, b *blockingUploader, n int) {
	t.Helper()
	for range n {
		select {
		case <-b.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("upload did not reach the uploader")
		}
	}
}

// TestInFlight (AC 8).
func TestInFlight(t *testing.T) {
	for _, n := range []int{4, 2} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			bu := newBlocking()
			clk := &clock{t: time.Now()}
			logs := &logLines{}
			h := NewHandler(Config{Uploads: bu, Logger: slog.New(slog.NewJSONHandler(logs, nil)), ParseTimeout: 10 * time.Second, MaxConcurrentUploads: n, Now: clk.now})
			var wg sync.WaitGroup
			codes := make(chan int, n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					codes <- do(h, smallRequest([]byte("x"), "198.51.100.1:1", "")).Code
				}()
			}
			waitEntered(t, bu, n)
			// More than 60 refused attempts: they must not use up the rate limit.
			for i := 0; i < 70; i++ {
				rec := do(h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("attempt %d: status %d", i, rec.Code)
				}
				if b := decodeErr(t, rec); b.Error != "busy" || b.Message != msgBusy {
					t.Fatalf("body = %+v", b)
				}
				if ra := rec.Header().Get("Retry-After"); ra != "30" {
					t.Fatalf("Retry-After = %q", ra)
				}
			}
			bu.release <- struct{}{} // one finishes
			if c := <-codes; c != http.StatusCreated {
				t.Errorf("released upload: %d", c)
			}
			// The next upload is accepted and blocks in turn.
			next := make(chan int, 1)
			go func() { next <- do(h, smallRequest([]byte("x"), "198.51.100.1:1", "")).Code }()
			waitEntered(t, bu, 1)
			close(bu.release)
			wg.Wait()
			if c := <-next; c != http.StatusCreated {
				t.Errorf("next upload: %d", c)
			}
		})
	}
}

func TestParseErrorMapping(t *testing.T) {
	for _, k := range []ingest.Kind{ingest.KindNotFIT, ingest.KindUnsupportedType, ingest.KindNoData, ingest.KindLimit, ingest.KindInternal, ingest.KindUnreadable, ingest.KindCanceled} {
		e := newEnv(t, 4)
		ie := &ingest.Error{Kind: k}
		e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) { return lifecycle.Created{}, ie }
		rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
		want := http.StatusUnprocessableEntity
		if k == ingest.KindCanceled {
			want = http.StatusBadRequest
		}
		b := decodeErr(t, rec)
		if rec.Code != want || b.Error != string(k) || b.Message != ie.Message() || b.Message == "" {
			t.Errorf("%s: status %d body %+v", k, rec.Code, b)
		}
		// Wrapped errors are found too.
		e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) {
			return lifecycle.Created{}, fmt.Errorf("wrapped: %w", ie)
		}
		if rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:2", "")); rec.Code != want {
			t.Errorf("%s wrapped: %d", k, rec.Code)
		}
	}
	// The file-content messages are the plain F-08 sentences.
	if m := (&ingest.Error{Kind: ingest.KindUnreadable}).Message(); m != "This file could not be read." {
		t.Errorf("unreadable message = %q", m)
	}
	e := newEnv(t, 4)
	e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) {
		return lifecycle.Created{}, &ingest.Error{Kind: ingest.KindUnsupportedType}
	}
	if b := decodeErr(t, do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", ""))); b.Message != "Unsupported FIT file type." {
		t.Errorf("message = %q", b.Message)
	}
}

func TestServerErrorMapping(t *testing.T) {
	for name, err := range map[string]error{
		"plain":        errors.New("disk write failed"),
		"service":      &lifecycle.ServerError{Stage: "files", Err: errors.New("x")},
		"wrapped":      fmt.Errorf("wrapped: %w", &lifecycle.ServerError{Stage: "store", Err: errors.New("x")}),
		"sqlite style": errors.New("constraint failed: UNIQUE constraint failed: workouts.public_id"),
	} {
		e := newEnv(t, 4)
		e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) { return lifecycle.Created{}, err }
		rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
		if b := decodeErr(t, rec); rec.Code != 500 || b.Error != "server_error" || b.Message != msgServerError {
			t.Errorf("%s: %d %+v", name, rec.Code, b)
		}
	}
	// A request that ended before the files were written is a 400 and logs nothing.
	e := newEnv(t, 4)
	e.up.fn = func(context.Context, []byte) (lifecycle.Created, error) { return lifecycle.Created{}, context.Canceled }
	rec := do(e.h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
	if b := decodeErr(t, rec); rec.Code != 400 || b.Error != "bad_request" || b.Message != msgUnreadable {
		t.Errorf("canceled: %d %+v", rec.Code, b)
	}
}

// TestHealthDuringUpload: health answers while uploads are held, also when
// all N are held (F-08 hand-off).
func TestHealthDuringUpload(t *testing.T) {
	const n = 2
	bu := newBlocking()
	srv := httptest.NewServer(NewHandler(Config{Version: "t", Uploads: bu, ParseTimeout: 10 * time.Second, MaxConcurrentUploads: n}))
	defer srv.Close()
	prefix, suffix, ctype := multipartParts("file")
	body := append(append(append([]byte(nil), prefix...), "x"...), suffix...)
	var wg sync.WaitGroup
	health := func() {
		t.Helper()
		start := time.Now()
		c := &http.Client{Timeout: time.Second}
		resp, err := c.Get(srv.URL + "/api/health")
		if err != nil {
			t.Fatalf("health: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || time.Since(start) > time.Second {
			t.Errorf("health: %d after %v", resp.StatusCode, time.Since(start))
		}
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Post(srv.URL+"/api/workouts", ctype, bytes.NewReader(body))
			if err == nil {
				resp.Body.Close()
			}
		}()
		waitEntered(t, bu, 1)
		health()
	}
	close(bu.release)
	wg.Wait()
}

// TestUploadDeadlines: on a real http.Server the per-request read deadline
// works (ADR-021 item 5): a body that stalls is cut off with the 400.
func TestUploadDeadlines(t *testing.T) {
	old := uploadReadTimeout
	uploadReadTimeout = 300 * time.Millisecond
	defer func() { uploadReadTimeout = old }()
	e := newEnv(t, 4)
	srv := httptest.NewServer(e.h)
	defer srv.Close()
	_, _, ctype := multipartParts("file")
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /api/workouts HTTP/1.1\r\nHost: x\r\nContent-Type: %s\r\nContent-Length: 100000\r\n\r\n--partial", ctype)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no response to a stalled upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || time.Since(start) > 3*time.Second {
		t.Errorf("status %d after %v", resp.StatusCode, time.Since(start))
	}
	if e.up.count() != 0 {
		t.Errorf("uploader called")
	}
}

func TestCapsOrder(t *testing.T) {
	if MaxUploadBytes > ingest.DefaultLimits.MaxBytes {
		t.Errorf("MaxUploadBytes %d > ingest MaxBytes %d", MaxUploadBytes, ingest.DefaultLimits.MaxBytes)
	}
	if MaxUploadBytes != 20_000_000 || maxBodyBytes != MaxUploadBytes+16<<10 {
		t.Errorf("constants: %d %d", MaxUploadBytes, maxBodyBytes)
	}
}

// TestNoViewPath (AC 12, Go side): Go never serves the viewing link (I-1).
func TestNoViewPath(t *testing.T) {
	e := newEnv(t, 4)
	for _, p := range []string{"/w/" + strings.Repeat("P", 22), "/w/", "/edit/" + strings.Repeat("P", 22)} {
		if rec := do(e.h, httptest.NewRequest(http.MethodGet, p, nil)); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", p, rec.Code)
		}
	}
	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		if rec := do(e.h, httptest.NewRequest(m, "/api/workouts", nil)); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/workouts: %d, want 405", m, rec.Code)
		}
	}
}

func TestNewHandlerPanicsWithoutSlots(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic for MaxConcurrentUploads 0")
		}
	}()
	NewHandler(Config{Version: "x"})
}

// TestUploadLogs: keys only from the design's Logging section; no line has an
// address, the file name or the token.
func TestUploadLogs(t *testing.T) {
	allowed := map[string]map[string]bool{
		"upload refused":  {"reason": true, "status": true},
		"rate table full": {"keys": true},
		"upload failed":   {"stage": true, "error": true}, // only the deadline WARN is reachable here
	}
	e := newEnv(t, 1)
	remote, xff := "127.0.0.1:4321", "203.0.113.77"
	for i := 0; i < 62; i++ {
		do(e.h, smallRequest([]byte("x"), remote, xff))
	}
	do(e.h, smallRequest([]byte("x"), "[2001:db8:1234:5678::abcd]:1", ""))
	bad := httptest.NewRequest(http.MethodPost, "/api/workouts", strings.NewReader("junk"))
	bad.Header.Set("Content-Type", "text/plain")
	bad.Header.Set("User-Agent", "MARKER-USER-AGENT")
	bad.RemoteAddr = "198.51.100.200:1"
	do(e.h, bad)
	big, _ := bigRequest(maxBodyBytes, true)
	do(e.h, big)
	bu := newBlocking()
	h := NewHandler(Config{Uploads: bu, Logger: slog.New(slog.NewJSONHandler(e.logs, nil)), ParseTimeout: 10 * time.Second, MaxConcurrentUploads: 1})
	go do(h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
	waitEntered(t, bu, 1)
	do(h, smallRequest([]byte("x"), "198.51.100.1:1", ""))
	close(bu.release)
	// The deadline lines (v1.4, QA-09-r1-6): a writer whose deadlines cannot be set.
	dw := &deadlineFailWriter{ResponseRecorder: httptest.NewRecorder()}
	e.h.ServeHTTP(dw, smallRequest([]byte("x"), "192.0.2.50:1", ""))
	if dw.Code != http.StatusCreated {
		t.Errorf("the upload must still complete: status %d", dw.Code)
	}

	deadlineLines := 0
	lines := strings.Split(strings.TrimSpace(e.logs.String()), "\n")
	var refusals, rateLines int
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		msg, _ := m["msg"].(string)
		ok := allowed[msg]
		if ok == nil {
			t.Errorf("unexpected message %q", msg)
			continue
		}
		for k := range m {
			if k != "time" && k != "level" && k != "msg" && !ok[k] {
				t.Errorf("%s: key %q is not in the design", msg, k)
			}
		}
		if msg == "upload failed" {
			if m["level"] != "WARN" || m["stage"] != "deadline" || m["error"] != "no deadlines here" {
				t.Errorf("upload failed line: %s", l)
			}
			deadlineLines++
			continue
		}
		refusals++
		if m["reason"] == "rate_limited" {
			rateLines++
		}
		for _, s := range []string{"203.0.113.77", "127.0.0.1", "2001:db8", "198.51.100", "MARKER"} {
			if strings.Contains(l, s) {
				t.Errorf("log line contains %q: %s", s, l)
			}
		}
	}
	if refusals == 0 {
		t.Fatal("no log lines")
	}
	if rateLines != 1 {
		t.Errorf("%d rate_limited lines for one key, want 1 (the first refusal)", rateLines)
	}
	if deadlineLines != 2 {
		t.Errorf("%d deadline WARN lines, want 2 (read and write)", deadlineLines)
	}
}

// deadlineFailWriter is a ResponseWriter whose deadlines cannot be set, for a
// reason other than http.ErrNotSupported.
type deadlineFailWriter struct{ *httptest.ResponseRecorder }

func (*deadlineFailWriter) SetReadDeadline(time.Time) error  { return errors.New("no deadlines here") }
func (*deadlineFailWriter) SetWriteDeadline(time.Time) error { return errors.New("no deadlines here") }

// TestLimiterMemory (v1.4, ADR-021 rev. 3, rev. 6): the table at its cap holds
// at most L = 100,000 x (4*limit + 160) bytes: 40 MB at 60, 112 MB at 240. Not
// parallel: it reads the process heap. The copy-down rule (no forward
// reslicing) is checked by code review: in this fill nothing expires.
func TestLimiterMemory(t *testing.T) {
	for _, limit := range []int{60, 240} {
		bound := int64(maxRateKeys * (4*limit + 160))
		clk := &clock{t: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}
		l := newLimiter(uploadRateWindow, maxRateKeys, clk.now)
		heap := func() uint64 {
			runtime.GC()
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			return ms.HeapAlloc
		}
		base := heap()
		for round := range limit {
			for k := range maxRateKeys {
				if ok, _, _ := l.allow(uint64(k)*0x9E3779B97F4A7C15, limit); !ok { // spread like hash values
					t.Fatalf("limit %d round %d key %d refused", limit, round, k)
				}
			}
			clk.advance(time.Second)
		}
		used := int64(heap()) - int64(base)
		runtime.KeepAlive(l)
		t.Logf("limit %d: limiter heap at %d keys x %d attempts: %d bytes (bound %d)", limit, maxRateKeys, limit, used, bound)
		if len(l.hits) != maxRateKeys {
			t.Fatalf("%d keys", len(l.hits))
		}
		if used > bound {
			t.Errorf("limit %d: limiter heap = %d bytes, over %d", limit, used, bound)
		}
	}
}

// TestTrustedProxy (F-31 DP-1a, AC 4): the forwarded address is believed from
// loopback and from the one configured address, from no other peer.
func TestTrustedProxy(t *testing.T) {
	caddy := netip.MustParseAddr("172.30.0.2")
	tests := []struct {
		name, remote string
		trusted      netip.Addr
		want         string
	}{
		{"configured peer is believed", "172.30.0.2:5", caddy, "2.2.2.2/32"},
		{"other peer is not, with a proxy set", "172.30.0.9:5", caddy, "172.30.0.9/32"},
		{"configured address unset: that peer is not believed", "172.30.0.2:5", netip.Addr{}, "172.30.0.2/32"},
		{"loopback stays trusted", "127.0.0.1:5", caddy, "2.2.2.2/32"},
		{"mapped form of the configured peer", "[::ffff:172.30.0.2]:5", caddy, "2.2.2.2/32"},
	}
	for _, tc := range tests {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = tc.remote
		r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
		if got := clientPrefix(r, tc.trusted).String(); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

// TestTwoRateLimits (F-31 DP-1c, AC 4): IPv4 per address, IPv6 per /64, each
// with its own limit.
func TestTwoRateLimits(t *testing.T) {
	e := newEnv(t, 4)
	h := newUploadHandler(Config{Uploads: e.up, Logger: slog.New(slog.DiscardHandler), ParseTimeout: 10 * time.Second, MaxConcurrentUploads: 4, Now: e.clk.now, UploadsPerHourIPv4: 1, UploadsPerHourIPv6: 2})
	code := func(remote string) int { return do(h, smallRequest([]byte("FIT"), remote, "")).Code }
	for i, tc := range []struct {
		remote string
		want   int
	}{
		{"198.51.100.1:1", http.StatusCreated},
		{"198.51.100.1:2", http.StatusTooManyRequests},
		{"198.51.100.2:1", http.StatusCreated}, // another address
		{"[2001:db8:1:2::a]:1", http.StatusCreated},
		{"[2001:db8:1:2::b]:1", http.StatusCreated}, // same /64, second of 2
		{"[2001:db8:1:2::c]:1", http.StatusTooManyRequests},
		{"[2001:db8:1:3::a]:1", http.StatusCreated}, // another /64
	} {
		if got := code(tc.remote); got != tc.want {
			t.Errorf("step %d (%s): %d, want %d", i, tc.remote, got, tc.want)
		}
	}
}

// deadlineRecorder records the deadlines the handler sets.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	read, write time.Time
}

func (d *deadlineRecorder) SetReadDeadline(t time.Time) error  { d.read = t; return nil }
func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error { d.write = t; return nil }

// TestUploadWriteDeadline (ADR-021 rev. 4): read = 120 s, write = 120 s + 6·T.
func TestUploadWriteDeadline(t *testing.T) {
	for _, tt := range []struct{ parse, write time.Duration }{{10 * time.Second, 180 * time.Second}, {60 * time.Second, 480 * time.Second}} {
		h := NewHandler(Config{Uploads: &fakeUploader{}, ParseTimeout: tt.parse, MaxConcurrentUploads: 1})
		rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
		req := httptest.NewRequest(http.MethodPost, "/api/workouts", strings.NewReader(""))
		start := time.Now()
		h.ServeHTTP(rec, req)
		end := time.Now()
		for _, c := range []struct {
			name string
			got  time.Time
			d    time.Duration
		}{{"read", rec.read, 120 * time.Second}, {"write", rec.write, tt.write}} {
			if c.got.Before(start.Add(c.d)) || c.got.After(end.Add(c.d)) {
				t.Errorf("T=%v: %s deadline %v from start, want %v", tt.parse, c.name, c.got.Sub(start), c.d)
			}
		}
	}
}

// TestNewHandlerParseTimeoutPanics: Uploads without a time limit is a wiring bug.
func TestNewHandlerParseTimeoutPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic for ParseTimeout 0")
		}
	}()
	NewHandler(Config{Uploads: &fakeUploader{}, MaxConcurrentUploads: 1})
}
