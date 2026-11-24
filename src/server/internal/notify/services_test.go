package notify

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Throwaway values; every endpoint is a local fake (F-70 AC 50).
const (
	bearer    = "tk_throwawayBearerToken123"
	topicPath = "/secretTopicName"
)

type rec struct {
	method, path string
	hdr          http.Header
	body         string
}

func (r rec) String() string { return r.method + " " + r.path }

func fakeService(t *testing.T, status int, loc string) (*httptest.Server, chan rec) {
	t.Helper()
	reqs := make(chan rec, 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reqs <- rec{r.Method, r.URL.Path, r.Header, string(b)}
		if loc != "" {
			w.Header().Set("Location", loc)
		}
		w.WriteHeader(status)
		io.WriteString(w, "RESPONSE-BODY-SECRET")
	}))
	t.Cleanup(srv.Close)
	return srv, reqs
}

// revoked collects the revoked report IDs.
type revoked struct {
	mu  sync.Mutex
	ids []int64
}

func (r *revoked) fn(_ context.Context, id int64) {
	r.mu.Lock()
	r.ids = append(r.ids, id)
	r.mu.Unlock()
}
func (r *revoked) get() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.ids...)
}

func issueLink(context.Context, int64) (string, bool) { return link, true }

func take(t *testing.T, reqs chan rec) rec {
	t.Helper()
	select {
	case g := <-reqs:
		return g
	case <-time.After(5 * time.Second):
		t.Fatal("no request")
		return rec{}
	}
}

// TestNtfyRequest (NS-5): the topic URL itself, text body, the delete link
// only in the action button, bearer token when set, nothing else.
func TestNtfyRequest(t *testing.T) {
	srv, reqs := fakeService(t, 200, "")
	n := New(Config{Kind: Ntfy, URL: srv.URL + topicPath, Token: bearer}, issueLink, nil, nil)
	n.handle(context.Background(), item{12, "Personal data", viewURL})
	g := take(t, reqs)
	if g.method != "POST" || g.path != topicPath || g.body != "New report #12: Personal data" {
		t.Errorf("request %v body %q", g, g.body)
	}
	want := map[string]string{"Title": "Workout Explorer", "Click": viewURL, "Actions": `view, Delete this workout, "` + link + `"`, "Authorization": "Bearer " + bearer}
	for k, v := range want {
		if g.hdr.Get(k) != v {
			t.Errorf("%s: %q", k, g.hdr.Get(k))
		}
	}
	for _, h := range []string{"Markdown", "Priority", "Tags", "Cache", "Firebase"} {
		if g.hdr.Get(h) != "" {
			t.Errorf("header %s set", h)
		}
	}
	if strings.Contains(g.body, "KKKK") || strings.Contains(g.body, "wx.example") {
		t.Errorf("link in the body %q", g.body)
	}
	// No key, no token, folded: no action, no Authorization.
	n = New(Config{Kind: Ntfy, URL: srv.URL + topicPath}, nil, nil, nil)
	n.held = 3
	n.handle(context.Background(), item{13, "Spam", viewURL})
	g = take(t, reqs)
	if g.body != "New report #13: Spam (+3 more)" || g.hdr.Get("Actions") != "" || g.hdr.Get("Authorization") != "" {
		t.Errorf("request %v %q %v", g, g.body, g.hdr)
	}
}

// TestCallbackRequest (NS-6): the fixed object in fixed key order, the bearer
// token, delete_url absent without a key.
func TestCallbackRequest(t *testing.T) {
	srv, reqs := fakeService(t, 204, "")
	n := New(Config{Kind: Callback, URL: srv.URL + "/in", Token: bearer}, issueLink, nil, nil)
	n.handle(context.Background(), item{12, `Personal "data"`, viewURL})
	g := take(t, reqs)
	want := `{"report":12,"reason":"Personal \"data\"","more":0,"view_url":"` + viewURL + `","delete_url":"` + link + `"}`
	if g.method != "POST" || g.path != "/in" || g.body != want || g.hdr.Get("Content-Type") != "application/json" || g.hdr.Get("Authorization") != "Bearer "+bearer {
		t.Errorf("request %v %q %v", g, g.body, g.hdr)
	}
	var m map[string]any
	if json.Unmarshal([]byte(g.body), &m) != nil || len(m) != 5 {
		t.Errorf("body %q", g.body)
	}
	n = New(Config{Kind: Callback, URL: srv.URL}, nil, nil, nil)
	n.held = 2
	n.handle(context.Background(), item{13, "Spam", viewURL})
	g = take(t, reqs)
	if g.body != `{"report":13,"reason":"Spam","more":2,"view_url":"`+viewURL+`"}` || g.hdr.Get("Authorization") != "" {
		t.Errorf("request %q %v", g.body, g.hdr)
	}
}

// TestDeliveryAndRevocation (NS-7, NS-8): any 2xx succeeds and keeps the key;
// anything else (also a redirect, never followed) is one request, one WARN
// line without a secret, and revokes that report's key.
func TestDeliveryAndRevocation(t *testing.T) {
	for _, kind := range []Kind{Pushover, Ntfy, Callback} {
		for _, c := range []struct {
			status int
			ok     bool
		}{{200, true}, {201, true}, {204, true}, {299, true}, {301, false}, {307, false}, {400, false}, {404, false}, {500, false}} {
			target, treqs := fakeService(t, 200, "")
			loc := ""
			if c.status/100 == 3 {
				loc = target.URL + "/followed"
			}
			srv, reqs := fakeService(t, c.status, loc)
			var logs syncBuf
			var rv revoked
			n := New(Config{Kind: kind, URL: srv.URL + topicPath + "?k=querySecret", Token: bearer, User: user}, issueLink, rv.fn, slog.New(slog.NewJSONHandler(&logs, nil)))
			n.handle(context.Background(), item{9, "spam", viewURL})
			if len(reqs) != 1 || len(treqs) != 0 {
				t.Errorf("%s %d: %d requests, %d followed (single try, no redirect)", kind, c.status, len(reqs), len(treqs))
			}
			l := logs.String()
			if c.ok && (l != "" || len(rv.get()) != 0) {
				t.Errorf("%s %d: log %q revoked %v", kind, c.status, l, rv.get())
			}
			if !c.ok && (len(rv.get()) != 1 || rv.get()[0] != 9 || strings.Count(l, "\n") != 1 || !strings.Contains(l, `"kind":"`+string(kind)+`"`)) {
				t.Errorf("%s %d: log %q revoked %v", kind, c.status, l, rv.get())
			}
			for _, bad := range []string{bearer, token, user, "querySecret", "secretTopic", "KKKK", "PPPP", "RESPONSE-BODY", srv.URL, "127.0.0.1"} {
				if strings.Contains(l, bad) {
					t.Errorf("%s %d: %q in the log %s", kind, c.status, bad, l)
				}
			}
		}
	}
}

// TestFailureClassesRevoke: timeout and network failure revoke too, also with
// an already cancelled context (shutdown); a message without a key revokes
// nothing.
func TestFailureClassesRevoke(t *testing.T) {
	hang := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hang:
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	defer close(hang) // runs first: the handler returns, then Close can finish
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	for class, url := range map[string]string{"timeout": slow.URL + topicPath, "network": dead.URL + topicPath} {
		var logs syncBuf
		var rv revoked
		n := New(Config{Kind: Ntfy, URL: url, Token: bearer}, issueLink, rv.fn, slog.New(slog.NewJSONHandler(&logs, nil)))
		n.client.Timeout = 50 * time.Millisecond
		n.handle(context.Background(), item{4, "spam", viewURL})
		if !strings.Contains(logs.String(), `"class":"`+class+`"`) {
			t.Errorf("%s: log %s", class, logs.String())
		}
		if got := rv.get(); len(got) != 1 || got[0] != 4 {
			t.Errorf("%s: revoked %v", class, got)
		}
		if strings.Contains(logs.String(), "secretTopic") || strings.Contains(logs.String(), "127.0.0.1") {
			t.Errorf("%s: URL in the log %s", class, logs.String())
		}
	}
	var rv revoked
	n := New(Config{Kind: Ntfy, URL: dead.URL + topicPath}, nil, rv.fn, nil)
	n.handle(context.Background(), item{5, "spam", viewURL})
	if len(rv.get()) != 0 {
		t.Errorf("revoked without a key: %v", rv.get())
	}
}

// TestSharedClient (NS-7): 10 s timeout, no environment proxy, no redirect,
// and the hourly limit with its folding, for every kind (AC 48).
func TestSharedClient(t *testing.T) {
	for _, kind := range []Kind{Pushover, Ntfy, Callback} {
		n := New(Config{Kind: kind, URL: "https://x.example/t"}, nil, nil, nil)
		if n.client.Timeout != 10*time.Second || n.client.Transport.(*http.Transport).Proxy != nil || n.client.CheckRedirect == nil {
			t.Errorf("%s client", kind)
		}
		srv, reqs := fakeService(t, 200, "")
		n = New(Config{Kind: kind, URL: srv.URL + "/t"}, nil, nil, nil)
		for i := 1; i <= 11; i++ {
			n.handle(context.Background(), item{int64(i), "spam", viewURL})
		}
		if len(reqs) != 10 || n.held != 1 {
			t.Fatalf("%s: %d sent, %d held", kind, len(reqs), n.held)
		}
		for i := 0; i < 10; i++ {
			<-reqs
		}
		n.sent = nil
		n.handle(context.Background(), item{12, "spam", viewURL})
		if g := take(t, reqs); !strings.Contains(g.body, "+1 more") && !strings.Contains(g.body, "%2B1+more") && !strings.Contains(g.body, `"more":1`) {
			t.Errorf("%s: folded message %q", kind, g.body)
		}
	}
}

// TestCheckURL (NS-3) runs the cases that update.sh's tests run too.
func TestCheckURL(t *testing.T) {
	f, err := os.Open("../../../../deploy/test/notify-url-cases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "#") || sc.Text() == "" {
			continue
		}
		p := strings.Split(sc.Text(), "\t")
		if len(p) != 3 {
			t.Fatalf("line %q", sc.Text())
		}
		err := CheckURL(Kind(p[1]), p[2])
		if (err == nil) != (p[0] == "valid") {
			t.Errorf("%s %s: error %v", p[1], p[2], err)
		}
		if err != nil && strings.Contains(err.Error(), p[2]) {
			t.Errorf("URL in the error text for %q", p[2])
		}
		n++
	}
	if n < 30 {
		t.Errorf("%d cases", n)
	}
	for tok, ok := range map[string]bool{"tk_abc123": true, "a": true, "abc==": true, "A-._~+/": true, "": false, "a b": false, "a\"b": false, "=": false, strings.Repeat("a", 256): true, strings.Repeat("a", 257): false, "é": false} {
		if CheckToken(tok) != ok {
			t.Errorf("CheckToken(%q) = %v", tok, !ok)
		}
	}
	if Host("https://ntfy.sh:8443/a?b=c") != "ntfy.sh:8443" || Host("http://[::1]:80") != "[::1]:80" {
		t.Error("Host")
	}
}
