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

const (
	viewURL = "https://wx.example/w/PPPPPPPPPPPPPPPPPPPPPP"
	link    = "https://wx.example/report-delete#KKKKKKKKKKKKKKKKKKKKKK"
)

// TestRequest (ADR-039 items 2, 3 rev. 1; F-37 DK-10): one form POST with
// exactly token, user, title, message, html, url and url_title; the secrets
// only in the body; the message is the number, the label and the delete link
// as an anchor; url is the viewing link.
func TestRequest(t *testing.T) {
	srv, reqs := fake(t, 200, nil)
	var logs syncBuf
	var issued []int64
	n := New(token, user, srv.URL+"/1/messages.json", func(_ context.Context, id int64) (string, bool) {
		issued = append(issued, id)
		return link, true
	}, slog.New(slog.NewJSONHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go n.Run(ctx)
	n.Notify(42, "Personal data", viewURL)
	g := wait(t, reqs)
	want := url.Values{"token": {token}, "user": {user}, "title": {"Workout Explorer"}, "html": {"1"}, "url": {viewURL}, "url_title": {"Open workout"},
		"message": {"New report #42: Personal data\n<a href=\"" + link + "\">Delete this workout</a>"}}
	if g.method != "POST" || g.path != "/1/messages.json" || g.query != "" || g.ctype != "application/x-www-form-urlencoded" || !reflect.DeepEqual(g.form, want) {
		t.Errorf("request %+v", g)
	}
	if !reflect.DeepEqual(issued, []int64{42}) {
		t.Errorf("issued %v", issued)
	}
	time.Sleep(50 * time.Millisecond)
	if logs.String() != "" {
		t.Errorf("log on success: %s", logs.String())
	}
}

// TestMessage: without a key no anchor; the link is HTML-escaped.
func TestMessage(t *testing.T) {
	for _, c := range []struct {
		label, link, want string
		more              int
	}{
		{"Spam", "", "New report #7: Spam", 0},
		{"Other", "", "New report #7: Other (+3 more)", 3},
		{"Illegal content", "https://a.example/x?a=1&b=\"#k", "New report #7: Illegal content (+1 more)\n<a href=\"https://a.example/x?a=1&amp;b=&#34;#k\">Delete this workout</a>", 1},
	} {
		if got := Message(7, c.label, c.more, c.link); got != c.want {
			t.Errorf("%q", got)
		}
	}
}

func TestDefaults(t *testing.T) {
	n := New(token, user, "", nil, nil)
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
	n := New(token, user, srv.URL, nil, nil)
	t0 := time.Now()
	for i := 1; i <= 100; i++ {
		n.Notify(int64(i), "spam", "")
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
	n.Notify(101, "spam", "") // the sender hangs on #1
	if d := time.Since(t0); d > 10*time.Millisecond {
		t.Errorf("Notify took %v", d)
	}
}

// TestCap (ADR-039 item 4): at most 10 sent per rolling hour; the rest ride
// on the next sent message.
func TestCap(t *testing.T) {
	srv, reqs := fake(t, 200, nil)
	var issued []int64
	n := New(token, user, srv.URL, func(_ context.Context, id int64) (string, bool) {
		issued = append(issued, id)
		return "", false // a failed issue: the message goes without the link
	}, nil)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	n.now = func() time.Time { return now }
	ctx := context.Background()
	for i := 1; i <= 13; i++ {
		n.handle(ctx, item{int64(i), "spam", ""})
	}
	if len(reqs) != 10 {
		t.Fatalf("%d sent", len(reqs))
	}
	for i := 0; i < 10; i++ {
		<-reqs
	}
	now = now.Add(time.Hour - time.Second)
	n.handle(ctx, item{14, "other", ""})
	if len(reqs) != 0 {
		t.Fatal("sent inside the hour")
	}
	now = now.Add(time.Second)
	n.handle(ctx, item{15, "other", ""})
	if g := wait(t, reqs); g.form.Get("message") != "New report #15: other (+4 more)" {
		t.Errorf("message %q", g.form.Get("message"))
	}
	n.handle(ctx, item{16, "spam", ""})
	if g := wait(t, reqs); g.form.Get("message") != "New report #16: spam" {
		t.Errorf("message %q", g.form.Get("message"))
	}
	// F-37 DK-3: a key only for a message actually sent; held ones get none.
	if !reflect.DeepEqual(issued, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 16}) {
		t.Errorf("issued %v", issued)
	}
}

// TestFailures (ADR-039 item 3): one WARN with the status or a fixed class,
// never a secret, the body or the error text; no retry, no redirect.
func TestFailures(t *testing.T) {
	ctx := context.Background()
	for _, status := range []int{400, 429, 500} {
		srv, reqs := fake(t, status, nil)
		var logs syncBuf
		n := New(token, user, srv.URL, func(context.Context, int64) (string, bool) { return link, true }, slog.New(slog.NewJSONHandler(&logs, nil)))
		n.handle(ctx, item{1, "spam", viewURL})
		if len(reqs) != 1 {
			t.Errorf("%d: %d requests (no retry)", status, len(reqs))
		}
		if l := logs.String(); !strings.Contains(l, `"level":"WARN","msg":"notification failed","status":`) || strings.Contains(l, "echo") {
			t.Errorf("%d: log %s", status, l)
		}
		noSecrets(t, logs.String())
		if strings.Contains(logs.String(), "KKKK") || strings.Contains(logs.String(), "PPPP") {
			t.Errorf("%d: a link in the log %s", status, logs.String())
		}
	}

	// A redirect is not followed.
	target, treqs := fake(t, 200, nil)
	redir := httptest.NewServer(http.RedirectHandler(target.URL+"/?token="+token, http.StatusTemporaryRedirect))
	defer redir.Close()
	var logs syncBuf
	n := New(token, user, redir.URL, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
	n.handle(ctx, item{1, "spam", ""})
	if len(treqs) != 0 || !strings.Contains(logs.String(), `"status":307`) {
		t.Errorf("redirect: %d requests, log %s", len(treqs), logs.String())
	}
	noSecrets(t, logs.String())

	// Timeout and network failures.
	hang := make(chan struct{})
	defer close(hang)
	slow, _ := fake(t, 200, hang)
	logs = syncBuf{}
	n = New(token, user, slow.URL, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
	n.client.Timeout = 50 * time.Millisecond
	n.handle(ctx, item{1, "spam", ""})
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	n.endpoint = dead.URL + "/x?token=" + token // even a secret in the URL never reaches the log
	n.handle(ctx, item{2, "spam", ""})
	l := logs.String()
	if !strings.Contains(l, `"class":"timeout"`) || !strings.Contains(l, `"class":"network"`) || strings.Count(l, "\n") != 2 {
		t.Errorf("log %s", l)
	}
	noSecrets(t, l)
}

// TestStop: Run returns when its context ends; queued ones are dropped.
func TestStop(t *testing.T) {
	n := New(token, user, "http://127.0.0.1:1", nil, nil)
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
