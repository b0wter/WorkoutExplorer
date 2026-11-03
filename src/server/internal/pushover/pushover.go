// Package pushover is the one adapter of the report notifier port: a short
// message to the operator's phone through Pushover's API (ADR-039). The
// message is the report's number and reason only, never a link, public ID,
// title, the reporter's text or anything about the request.
package pushover

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
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

type item struct {
	id     int64
	reason string
}

// Notifier sends notifications from one goroutine (Run). Notify never blocks.
type Notifier struct {
	token, user string
	endpoint    string
	client      *http.Client
	log         *slog.Logger
	now         func() time.Time

	queue   chan item
	dropped atomic.Int64 // queue full; counted into the next message

	mu   sync.Mutex
	held int         // not sent because of the cap; counted into the next message
	sent []time.Time // send times inside the window
}

// New returns a Notifier for the given secrets. endpoint "" is Endpoint;
// nil log writes nothing.
func New(token, user, endpoint string, log *slog.Logger) *Notifier {
	if endpoint == "" {
		endpoint = Endpoint
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Notifier{
		token: token, user: user, endpoint: endpoint, log: log, now: time.Now,
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
func (n *Notifier) Notify(id int64, reason string) {
	select {
	case n.queue <- item{id, reason}:
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
	n.send(ctx, Message(it.id, it.reason, more))
}

// Message is the notification text (ADR-039 item 2).
func Message(id int64, reason string, more int) string {
	m := "New report #" + strconv.FormatInt(id, 10) + ": " + reason
	if more > 0 {
		m += " (+" + strconv.Itoa(more) + " more)"
	}
	return m
}

// send posts one message, no retry. A failure is one WARN line with the HTTP
// status or a fixed class, never the error text, the body or a secret.
func (n *Notifier) send(ctx context.Context, msg string) {
	form := url.Values{"token": {n.token}, "user": {n.user}, "title": {title}, "message": {msg}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		n.log.Warn("notification failed", "class", "request")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := n.client.Do(req)
	if err != nil {
		class := "network"
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			class = "timeout"
		}
		n.log.Warn("notification failed", "class", class)
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		n.log.Warn("notification failed", "status", resp.StatusCode)
	}
}
