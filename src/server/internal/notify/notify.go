// Package notify is the adapter of the report notifier port (F-70 NS-1): a
// short message to the operator through Pushover (ADR-039, F-37 DK-10), an
// ntfy topic or a callback URL. One queue, one hourly limit, one client and
// one failure rule for every kind; each kind only builds its request. The
// message is the report's number and reason label, the viewing link and,
// when a key was issued, the single-use delete link. Never the title, the
// reporter's text or anything about the request. The delete link, the URL
// and the token are never logged.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Endpoint is Pushover's message API (https://pushover.net/api).
const Endpoint = "https://api.pushover.net/1/messages.json"

const (
	title     = "Workout Explorer"
	timeout   = 10 * time.Second
	perWindow = 10 // sent per rolling hour per process (ADR-039 item 4)
	window    = time.Hour
	queueSize = 16
)

// ValidKey reports whether s has the form of a Pushover application token or
// user key: 30 ASCII letters and digits.
var ValidKey = regexp.MustCompile(`^[A-Za-z0-9]{30}$`).MatchString

// Kind is the service a notification goes through.
type Kind string

const (
	Pushover Kind = "pushover"
	Ntfy     Kind = "ntfy"
	Callback Kind = "callback"
)

// Config selects the service. Pushover: Token is the application token, User
// the user key, URL "" is Endpoint (tests point it at a fake). Ntfy and
// callback: URL is the validated URL (CheckURL), Token the optional bearer.
type Config struct {
	Kind             Kind
	URL, Token, User string
}

// RevokeFunc removes the delete key of a report whose message failed (NS-7).
type RevokeFunc func(ctx context.Context, reportID int64)

type item struct {
	id             int64
	label, viewURL string
}

// IssueFunc creates the delete link of a report right before its message is
// sent (F-37 DK-3); false: send without one.
type IssueFunc func(ctx context.Context, reportID int64) (link string, ok bool)

// Notifier sends notifications from one goroutine (Run). Notify never blocks.
type Notifier struct {
	cfg    Config
	client *http.Client
	log    *slog.Logger
	now    func() time.Time
	issue  IssueFunc // nil: no delete links
	revoke RevokeFunc

	queue   chan item
	dropped atomic.Int64 // queue full; counted into the next message

	mu   sync.Mutex
	held int         // not sent because of the cap; counted into the next message
	sent []time.Time // send times inside the window
}

// New returns a Notifier for cfg. nil issue sends no delete links; nil revoke
// revokes nothing; nil log writes nothing.
func New(cfg Config, issue IssueFunc, revoke RevokeFunc, log *slog.Logger) *Notifier {
	if cfg.Kind == Pushover && cfg.URL == "" {
		cfg.URL = Endpoint
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Notifier{
		cfg: cfg, log: log, now: time.Now, issue: issue, revoke: revoke,
		queue: make(chan item, queueSize),
		client: &http.Client{
			Timeout: timeout,
			// No proxy from the environment; TLS against the system (image) CA bundle.
			Transport: &http.Transport{Proxy: nil, ForceAttemptHTTP2: true, TLSHandshakeTimeout: timeout},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Notify queues a notification for a stored report; with the queue full it
// is counted into the next message instead.
func (n *Notifier) Notify(id int64, label, viewURL string) {
	select {
	case n.queue <- item{id, label, viewURL}:
	default:
		n.dropped.Add(1)
	}
}

// Run sends queued notifications until ctx ends; pending ones are dropped.
func (n *Notifier) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case it := <-n.queue:
			if ctx.Err() != nil { // select is random: after shutdown starts, take no more work (QA-70-r2-1)
				return
			}
			n.handle(ctx, it)
		}
	}
}

func (n *Notifier) handle(ctx context.Context, it item) {
	now := n.now()
	n.mu.Lock()
	i := 0
	for i < len(n.sent) && !now.Before(n.sent[i].Add(window)) {
		i++
	}
	n.sent = n.sent[i:]
	if len(n.sent) >= perWindow {
		n.held++
		n.mu.Unlock()
		return
	}
	n.sent = append(n.sent, now)
	more := n.held + int(n.dropped.Swap(0))
	n.held = 0
	n.mu.Unlock()
	var link string
	if n.issue != nil {
		link, _ = n.issue(ctx, it.id) // after the cap admitted it: only a sent message gets a key
	}
	if !n.send(ctx, it, more, link) && link != "" && n.revoke != nil {
		// NS-7: a key never confirmed delivered stays invalid; own deadline, so it also runs at shutdown.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		n.revoke(rctx, it.id)
	}
}

// Message is the notification text (ADR-039 item 2 rev. 1, F-37 DK-10), for
// html=1: the anchor text, not the link, is what the lock screen shows.
func Message(id int64, label string, more int, deleteLink string) string {
	m := "New report #" + strconv.FormatInt(id, 10) + ": " + html.EscapeString(label)
	if more > 0 {
		m += " (+" + strconv.Itoa(more) + " more)"
	}
	if deleteLink != "" {
		m += "\n<a href=\"" + html.EscapeString(deleteLink) + "\">Delete this workout</a>"
	}
	return m
}

// request builds the one request of a message for the configured kind.
func (n *Notifier) request(ctx context.Context, it item, more int, link string) (*http.Request, error) {
	var (
		body string
		hdr  = http.Header{}
	)
	switch n.cfg.Kind {
	case Ntfy: // NS-5: the delete link only in the action, never in the body
		body = textMessage(it.id, it.label, more)
		hdr.Set("Title", title)
		hdr.Set("Click", it.viewURL)
		if link != "" {
			hdr.Set("Actions", `view, Delete this workout, "`+link+`"`)
		}
	case Callback: // NS-6: fixed object, fixed key order
		b := []byte(`{"report":` + strconv.FormatInt(it.id, 10) + `,"reason":`)
		b = appendJSON(b, it.label)
		b = append(b, `,"more":`+strconv.Itoa(more)+`,"view_url":`...)
		b = appendJSON(b, it.viewURL)
		if link != "" {
			b = append(b, `,"delete_url":`...)
			b = appendJSON(b, link)
		}
		body = string(append(b, '}'))
		hdr.Set("Content-Type", "application/json")
	default: // NS-4: unchanged
		form := url.Values{"token": {n.cfg.Token}, "user": {n.cfg.User}, "title": {title}, "message": {Message(it.id, it.label, more, link)},
			"html": {"1"}, "url": {it.viewURL}, "url_title": {"Open workout"}}
		body = form.Encode()
		hdr.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if n.cfg.Kind != Pushover && n.cfg.Token != "" {
		hdr.Set("Authorization", "Bearer "+n.cfg.Token)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.URL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	return req, nil
}

func appendJSON(b []byte, s string) []byte {
	j, _ := json.Marshal(s) // a string cannot fail
	return append(b, j...)
}

func textMessage(id int64, label string, more int) string {
	m := "New report #" + strconv.FormatInt(id, 10) + ": " + label
	if more > 0 {
		m += " (+" + strconv.Itoa(more) + " more)"
	}
	return m
}

// send posts one message, no retry, and reports whether any 2xx answered. A
// failure is one WARN line with the kind and the HTTP status or a fixed
// class, never the error text, the body, the URL or a secret (NS-7, NS-8).
func (n *Notifier) send(ctx context.Context, it item, more int, link string) bool {
	fail := func(kv ...any) bool {
		n.log.Warn("notification failed", append([]any{"kind", string(n.cfg.Kind)}, kv...)...)
		return false
	}
	req, err := n.request(ctx, it, more, link)
	if err != nil {
		return fail("class", "request")
	}
	resp, err := n.client.Do(req)
	if err != nil {
		class := "network"
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			class = "timeout"
		}
		return fail("class", class)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fail("status", resp.StatusCode)
	}
	return true
}

// Host is the host and port of a validated URL, the only part that is logged
// or shown (NS-8).
func Host(raw string) string {
	_, rest, _ := strings.Cut(raw, "://")
	return rest[:strings.IndexAny(rest+"/", "/?")]
}

var (
	urlChars   = regexp.MustCompile(`^[A-Za-z0-9\-._~:/?\[\]@!$&'()*+,;=%]+$`)
	authority  = regexp.MustCompile(`^([A-Za-z0-9.-]+|\[[0-9A-Fa-f:.]+\])(:[0-9]{1,5})?$`)
	badPercent = regexp.MustCompile(`%($|[^0-9A-Fa-f]|[0-9A-Fa-f]($|[^0-9A-Fa-f]))`) // url.Parse checks the path only; update.sh checks the whole URL
	tokenChars = regexp.MustCompile(`^[A-Za-z0-9._~+/-]+=*$`)
)

// CheckURL applies F-70 NS-3 to the URL of kind Ntfy or Callback. The error
// text is fixed: url.Parse's own text would echo the URL.
func CheckURL(kind Kind, raw string) error {
	bad := errors.New("must be an https URL of at most 2048 characters (http only to a loopback or private IP address; ntfy needs a topic path)")
	plain := strings.HasPrefix(raw, "http://")
	if len(raw) > 2048 || !urlChars.MatchString(raw) || strings.Contains(raw, "#") || badPercent.MatchString(raw) || !(plain || strings.HasPrefix(raw, "https://")) {
		return bad
	}
	if _, err := url.Parse(raw); err != nil {
		return bad
	}
	_, rest, _ := strings.Cut(raw, "://")
	auth := Host(raw)
	path, _, _ := strings.Cut(rest[len(auth):], "?")
	m := authority.FindStringSubmatch(auth)
	if m == nil || (kind == Ntfy && (path == "" || path == "/")) {
		return bad
	}
	if p := strings.TrimPrefix(auth[len(m[1]):], ":"); p != "" {
		if n, _ := strconv.Atoi(p); n < 1 || n > 65535 {
			return bad
		}
	}
	addr, aerr := netip.ParseAddr(strings.Trim(m[1], "[]"))
	if strings.HasPrefix(m[1], "[") && aerr != nil {
		return bad
	}
	// Plain http to IPv6: the canonical ::1 or a fc00::/7 literal without an IPv4 part; update.sh accepts the same set.
	if plain && (aerr != nil || addr.Is4In6() || addr.Is6() && strings.Contains(m[1], ".") ||
		!(addr.IsPrivate() || addr.IsLoopback() && (addr.Is4() || m[1] == "[::1]"))) {
		return bad
	}
	return nil
}

// CheckToken applies F-70 NS-3 to the bearer token: 1 to 256 characters of
// RFC 7235 token68.
func CheckToken(s string) bool { return len(s) <= 256 && tokenChars.MatchString(s) }
