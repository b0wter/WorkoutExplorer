package pushover

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests run against local fake servers only; nothing reaches the network.

const (
	token = "aTokenOfThirtyLettersAndDigit1"
	user  = "uUserKeyOfThirtyLettersDigits2"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

type got struct {
	method, path, query, ctype string
	form                       url.Values
}

// fake records each request and answers with status (after hang, if set).
func fake(t *testing.T, status int, hang chan struct{}) (*httptest.Server, chan got) {
	t.Helper()
	reqs := make(chan got, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f, _ := url.ParseQuery(string(b))
		reqs <- got{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Content-Type"), f}
		if hang != nil {
			select {
			case <-hang:
			case <-r.Context().Done():
			}
		}
		w.WriteHeader(status)
		io.WriteString(w, `{"status":1,"request":"x","echo":"`+token+`"}`)
	}))
	t.Cleanup(srv.Close)
	return srv, reqs
}

func start(t *testing.T, endpoint string, logs io.Writer) *Notifier {
	t.Helper()
	n := New(token, user, endpoint, slog.New(slog.NewJSONHandler(logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go n.Run(ctx)
	return n
}

func wait(t *testing.T, reqs chan got) got {
	t.Helper()
	select {
	case g := <-reqs:
		return g
	case <-time.After(5 * time.Second):
		t.Fatal("no request")
		return got{}
	}
}

func noSecrets(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, token) || strings.Contains(s, user) {
		t.Errorf("a secret in %q", s)
	}
}

// TestRequest (ADR-039 items 2, 3): one form POST with exactly token, user,
// title and message; the secrets only in the body; the message is the
// number and the reason only.
func TestRequest(t *testing.T) {
	srv, reqs := fake(t, 200, nil)
	var logs syncBuf
	n := start(t, srv.URL+"/1/messages.json", &logs)
	n.Notify(42, "personal_data")
	g := wait(t, reqs)
	want := url.Values{"token": {token}, "user": {user}, "title": {"Workout Explorer"}, "message": {"New report #42: personal_data"}}
	if g.method != "POST" || g.path != "/1/messages.json" || g.query != "" || g.ctype != "application/x-www-form-urlencoded" || !reflect.DeepEqual(g.form, want) {
		t.Errorf("request %+v", g)
	}
	time.Sleep(50 * time.Millisecond)
	if logs.String() != "" {
		t.Errorf("log on success: %s", logs.String())
	}
}

func TestDefaults(t *testing.T) {
	n := New(token, user, "", nil)
	tr := n.client.Transport.(*http.Transport)
	if n.endpoint != "https://api.pushover.net/1/messages.json" || tr.Proxy != nil || n.client.Timeout != 10*time.Second || n.client.CheckRedirect == nil {
		t.Errorf("endpoint %s, proxy set %v, timeout %v", n.endpoint, tr.Proxy != nil, n.client.Timeout)
	}
	for k, ok := range map[string]bool{token: true, user: true, "": false, token[:29]: false, token + "a": false, token[:29] + "-": false, token[:29] + "é": false} {
		if ValidKey(k) != ok {
			t.Errorf("ValidKey(%q) = %v", k, !ok)
		}
	}
}

// TestNotifyNeverBlocks: Notify returns at once, also with the queue full
// and the server hanging; what the queue could not take rides on the next
// message.
func TestNotifyNeverBlocks(t *testing.T) {
	hang := make(chan struct{})
	defer close(hang)
	srv, reqs := fake(t, 200, hang)
	n := New(token, user, srv.URL, nil)
	t0 := time.Now()
	for i := 1; i <= 100; i++ {
		n.Notify(int64(i), "spam")
	}
	if d := time.Since(t0); d > 100*time.Millisecond {
		t.Errorf("100 Notify calls took %v", d)
	}
	if len(n.queue) != queueSize || n.dropped.Load() != 100-queueSize {
		t.Fatalf("queued %d, dropped %d", len(n.queue), n.dropped.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.Run(ctx)
	if m := wait(t, reqs).form.Get("message"); m != "New report #1: spam (+84 more)" {
		t.Errorf("first message %q", m)
	}
	t0 = time.Now()
	n.Notify(101, "spam") // the sender hangs on #1
	if d := time.Since(t0); d > 10*time.Millisecond {
		t.Errorf("Notify took %v", d)
	}
}

// TestCap (ADR-039 item 4): at most 10 sent per rolling hour; the rest ride
// on the next sent message.
func TestCap(t *testing.T) {
	srv, reqs := fake(t, 200, nil)
	n := New(token, user, srv.URL, nil)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	n.now = func() time.Time { return now }
	ctx := context.Background()
	for i := 1; i <= 13; i++ {
		n.handle(ctx, item{int64(i), "spam"})
	}
	if len(reqs) != 10 {
		t.Fatalf("%d sent", len(reqs))
	}
	for i := 0; i < 10; i++ {
		<-reqs
	}
	now = now.Add(time.Hour - time.Second)
	n.handle(ctx, item{14, "other"})
	if len(reqs) != 0 {
		t.Fatal("sent inside the hour")
	}
	now = now.Add(time.Second)
	n.handle(ctx, item{15, "other"})
	if g := wait(t, reqs); g.form.Get("message") != "New report #15: other (+4 more)" {
		t.Errorf("message %q", g.form.Get("message"))
	}
	n.handle(ctx, item{16, "spam"})
	if g := wait(t, reqs); g.form.Get("message") != "New report #16: spam" {
		t.Errorf("message %q", g.form.Get("message"))
	}
}

// TestFailures (ADR-039 item 3): one WARN with the status or a fixed class,
// never a secret, the body or the error text; no retry, no redirect.
func TestFailures(t *testing.T) {
	ctx := context.Background()
	for _, status := range []int{400, 429, 500} {
		srv, reqs := fake(t, status, nil)
		var logs syncBuf
		n := New(token, user, srv.URL, slog.New(slog.NewJSONHandler(&logs, nil)))
		n.handle(ctx, item{1, "spam"})
		if len(reqs) != 1 {
			t.Errorf("%d: %d requests (no retry)", status, len(reqs))
		}
		if l := logs.String(); !strings.Contains(l, `"level":"WARN","msg":"notification failed","status":`) || strings.Contains(l, "echo") {
			t.Errorf("%d: log %s", status, l)
		}
		noSecrets(t, logs.String())
	}

	// A redirect is not followed.
	target, treqs := fake(t, 200, nil)
	redir := httptest.NewServer(http.RedirectHandler(target.URL+"/?token="+token, http.StatusTemporaryRedirect))
	defer redir.Close()
	var logs syncBuf
	n := New(token, user, redir.URL, slog.New(slog.NewJSONHandler(&logs, nil)))
	n.handle(ctx, item{1, "spam"})
	if len(treqs) != 0 || !strings.Contains(logs.String(), `"status":307`) {
		t.Errorf("redirect: %d requests, log %s", len(treqs), logs.String())
	}
	noSecrets(t, logs.String())

	// Timeout and network failures.
	hang := make(chan struct{})
	defer close(hang)
	slow, _ := fake(t, 200, hang)
	logs = syncBuf{}
	n = New(token, user, slow.URL, slog.New(slog.NewJSONHandler(&logs, nil)))
	n.client.Timeout = 50 * time.Millisecond
	n.handle(ctx, item{1, "spam"})
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	n.endpoint = dead.URL + "/x?token=" + token // even a secret in the URL never reaches the log
	n.handle(ctx, item{2, "spam"})
	l := logs.String()
	if !strings.Contains(l, `"class":"timeout"`) || !strings.Contains(l, `"class":"network"`) || strings.Count(l, "\n") != 2 {
		t.Errorf("log %s", l)
	}
	noSecrets(t, l)
}

// TestStop: Run returns when its context ends; queued ones are dropped.
func TestStop(t *testing.T) {
	n := New(token, user, "http://127.0.0.1:1", nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { n.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}
