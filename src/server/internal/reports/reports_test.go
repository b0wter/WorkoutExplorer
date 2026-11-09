package reports

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"workout-explorer/internal/lifecycle"
)

type fakeStore struct {
	calls      int
	found      bool
	id         int64
	err        error
	got        []string
	pruneAt    time.Time
	pruneN     int64
	gotNow     time.Time
	ctxCancels bool
	setID      int64
	setHash    [32]byte
	keyed      KeyedReport
	keyArgs    []any
}

func (f *fakeStore) AddReport(ctx context.Context, publicID, reason, message string, now time.Time, max int) (int64, string, bool, error) {
	f.calls++
	f.got = []string{publicID, reason, message}
	f.gotNow = now
	f.ctxCancels = ctx.Done() != nil
	if max != MaxPerWorkout {
		panic("max")
	}
	return f.id, "w-internal", f.found, f.err
}

func (f *fakeStore) DeleteReportsFiledBy(_ context.Context, t time.Time) (int64, error) {
	f.pruneAt = t
	return f.pruneN, f.err
}

func (f *fakeStore) SetDeleteKeyHash(_ context.Context, id int64, h [32]byte) (bool, error) {
	f.setID, f.setHash = id, h
	return f.found, f.err
}

func (f *fakeStore) ClearDeleteKeys(context.Context) (int64, error) { return f.pruneN, f.err }

func (f *fakeStore) ReportByDeleteKey(_ context.Context, h [32]byte, after, now time.Time) (KeyedReport, bool, error) {
	f.keyArgs = []any{h, after, now}
	return f.keyed, f.found && h == f.setHash, f.err
}

func (f *fakeStore) ConsumeDeleteKey(_ context.Context, h [32]byte, after, now time.Time) (string, string, bool, error) {
	f.keyArgs = []any{h, after, now}
	ok := f.found && h == f.setHash
	if ok {
		f.setHash = [32]byte{} // spent
	}
	return "w-internal", pub, ok, f.err
}

type fakeNotifier struct{ got []string }

func (n *fakeNotifier) Notify(id int64, label, viewURL string) {
	n.got = append(n.got, note(id, label)+" "+viewURL)
}

func note(id int64, reason string) string { return fmt.Sprintf("%s#%d", reason, id) }

const pub = "AAAAAAAAAAAAAAAAAAAAAA"

func TestCheck(t *testing.T) {
	long := strings.Repeat("é", MaxMessageChars) // 2 bytes each: code points, not bytes
	for _, c := range []struct {
		name, reason, msg, want string
		err                     string
	}{
		{"spam", "spam", "", "", ""},
		{"illegal", "illegal_content", "x", "x", ""},
		{"personal", "personal_data", "x", "x", ""},
		{"other", "other", "x", "x", ""},
		{"unknown reason", "abuse", "", "", MsgReason},
		{"empty reason", "", "", "", MsgReason},
		{"case", "Spam", "", "", MsgReason},
		{"trimmed", "spam", "  a\nb \n", "a\nb", ""},
		{"1000 code points", "spam", long, long, ""},
		{"1000 after trim", "spam", " " + long + " ", long, ""},
		{"1001 code points", "spam", long + "x", "", MsgMessageLong},
		{"carriage return", "spam", "a\r\nb", "", MsgMessageCtrl},
		{"tab", "spam", "a\tb", "", MsgMessageCtrl},
		{"escape", "spam", "a\x1bb", "", MsgMessageCtrl},
		{"C1", "spam", "a\u0085b", "", MsgMessageCtrl},
	} {
		got, err := Check(c.reason, c.msg)
		var ie *InvalidError
		switch {
		case c.err == "" && (err != nil || got != c.want):
			t.Errorf("%s: %q, %v", c.name, got, err)
		case c.err != "" && (!errors.As(err, &ie) || ie.Message != c.err):
			t.Errorf("%s: err %v, want %q", c.name, err, c.err)
		}
	}
}

func TestFile(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var logs bytes.Buffer
	st := &fakeStore{found: true, id: 7}
	n := &fakeNotifier{}
	s := New(Deps{Store: st, Notifier: n, BaseURL: "https://wx.example", Now: func() time.Time { return now }, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.File(ctx, pub, "spam", " SECRETMESSAGE "); err != nil {
		t.Fatal(err)
	}
	if st.got[0] != pub || st.got[1] != "spam" || st.got[2] != "SECRETMESSAGE" || !st.gotNow.Equal(now) || st.ctxCancels {
		t.Errorf("store got %v at %v, cancellable ctx %v", st.got, st.gotNow, st.ctxCancels)
	}
	if len(n.got) != 1 || n.got[0] != note(7, "Spam")+" https://wx.example/w/"+pub {
		t.Errorf("notified %v", n.got)
	}
	l := logs.String()
	if !strings.Contains(l, `"msg":"report stored","id":7,"workout_id":"w-internal","reason":"spam"`) || strings.Contains(l, "SECRETMESSAGE") || strings.Contains(l, pub) {
		t.Errorf("log %s", l)
	}

	// Invalid and malformed: no store call.
	st.calls = 0
	for _, id := range []string{"", "short", pub + "A", "AAAAAAAAAAAAAAAAAAAAA_", "AAAAAAAAAAAAAAAAAAAAAé"} {
		if err := s.File(ctx, id, "spam", ""); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: %v", id, err)
		}
	}
	if err := s.File(ctx, pub, "nope", ""); err == nil {
		t.Error("bad reason accepted")
	}
	if st.calls != 0 {
		t.Errorf("%d store calls", st.calls)
	}

	// Not found, cap, store failure: no notification.
	n.got = nil
	st.found = false
	if err := s.File(ctx, pub, "other", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("not found: %v", err)
	}
	st.found, st.id = true, 0
	logs.Reset()
	if err := s.File(ctx, pub, "other", ""); err != nil || logs.Len() != 0 {
		t.Errorf("capped: %v, log %s", err, logs.String())
	}
	st.err = errors.New("disk")
	if err := s.File(ctx, pub, "other", ""); err == nil || err.Error() != "disk" {
		t.Errorf("store failure: %v", err)
	}
	if n.got != nil {
		t.Errorf("notified %v", n.got)
	}

	// No notifier: still stored.
	st.err, st.id = nil, 9
	if err := New(Deps{Store: st}).File(ctx, pub, "spam", ""); err != nil {
		t.Error(err)
	}
}

func TestPrune(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var logs bytes.Buffer
	st := &fakeStore{pruneN: 2}
	s := New(Deps{Store: st, Now: func() time.Time { return now }, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	s.Prune(context.Background())
	if !st.pruneAt.Equal(now.Add(-30*24*time.Hour)) || !strings.Contains(logs.String(), `"msg":"reports pruned","removed":2`) {
		t.Errorf("cutoff %v log %s", st.pruneAt, logs.String())
	}
	st.err = errors.New("disk")
	s.Prune(context.Background())
	if !strings.Contains(logs.String(), `"msg":"report pruning failed"`) {
		t.Errorf("log %s", logs.String())
	}
}

type fakeDeleter struct {
	got        []lifecycle.Record
	err        error
	ctxCancels bool
}

func (d *fakeDeleter) DeleteByOperator(ctx context.Context, r lifecycle.Record) error {
	d.got = append(d.got, r)
	d.ctxCancels = ctx.Done() != nil
	return d.err
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

// TestDeleteKeys (F-37 DK-2 to DK-8): the link carries the key, the store
// only its hash; lookup shows the DK-7 fields; redeem spends the key once and
// runs the operator delete; nothing logs the key.
func TestDeleteKeys(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	var logs bytes.Buffer
	start := time.Date(2026, 11, 8, 6, 30, 0, 0, time.UTC)
	off := 3600
	st := &fakeStore{found: true, keyed: KeyedReport{ID: 5, Reason: "personal_data", FiledAt: now.Add(-time.Hour), Count: 3,
		Workout: lifecycle.Record{Sport: "cycling", StartTime: &start, TimeZoneOffsetS: &off, ExpiresAt: now.Add(48 * time.Hour)}}}
	del := &fakeDeleter{}
	s := New(Deps{Store: st, Deleter: del, BaseURL: "https://wx.example", Now: func() time.Time { return now }, Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	link, ok := s.IssueDeleteLink(ctx, 5)
	key, found := strings.CutPrefix(link, "https://wx.example/report-delete#")
	if !ok || !found || !lifecycle.ValidPublicID(key) || st.setID != 5 || st.setHash != lifecycle.HashToken(key) {
		t.Fatalf("link %q, stored %d %x", link, st.setID, st.setHash)
	}

	for _, bad := range []string{"", "short", key + "A", key[:21] + "_", strings.ToLower(key[:21]) + "é"} {
		if _, err := s.LookupDeleteKey(ctx, bad); !errors.Is(err, ErrLinkInvalid) {
			t.Errorf("lookup %q: %v", bad, err)
		}
		if err := s.RedeemDeleteKey(ctx, bad); !errors.Is(err, ErrLinkInvalid) {
			t.Errorf("redeem %q: %v", bad, err)
		}
	}
	if st.keyArgs != nil || del.got != nil {
		t.Fatal("a malformed key reached the store")
	}

	info, err := s.LookupDeleteKey(ctx, key)
	want := KeyInfo{Title: "Ride, 8 Nov 2026", StartDate: "8 Nov 2026, 07:30", ReportID: 5, Reason: "Personal data",
		FiledAt: now.Add(-time.Hour), ExpiresAt: now.Add(48 * time.Hour), ReportCount: 3}
	if err != nil || info != want {
		t.Errorf("lookup %+v, %v", info, err)
	}
	if st.keyArgs[0] != lifecycle.HashToken(key) || !st.keyArgs[1].(time.Time).Equal(now.Add(-Retention)) || !st.keyArgs[2].(time.Time).Equal(now) {
		t.Errorf("store args %v", st.keyArgs)
	}
	st.keyed.Workout.Title = "  Morning loop "
	if info, _ := s.LookupDeleteKey(ctx, key); info.Title != "  Morning loop " {
		t.Errorf("stored title %q", info.Title)
	}
	if del.got != nil {
		t.Fatal("lookup deleted")
	}

	if err := s.RedeemDeleteKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	if len(del.got) != 1 || del.got[0].ID != "w-internal" || del.got[0].PublicID != pub || del.ctxCancels {
		t.Errorf("deleted %+v, cancellable %v", del.got, del.ctxCancels)
	}
	if err := s.RedeemDeleteKey(ctx, key); !errors.Is(err, ErrLinkInvalid) || len(del.got) != 1 {
		t.Errorf("second redeem: %v, %d deletes", err, len(del.got))
	}

	// A delete failure after the consume is returned; the key is spent.
	link, _ = s.IssueDeleteLink(ctx, 6)
	key2 := link[len(link)-22:]
	del.err = errors.New("disk")
	if err := s.RedeemDeleteKey(ctx, key2); err == nil || err.Error() != "disk" {
		t.Errorf("delete failure: %v", err)
	}
	if err := s.RedeemDeleteKey(ctx, key2); !errors.Is(err, ErrLinkInvalid) {
		t.Errorf("after a failed delete: %v", err)
	}

	// Issue failures: no random bytes, report gone, store error. One WARN with the ID only.
	logs.Reset()
	s.rand = failReader{}
	if l, ok := s.IssueDeleteLink(ctx, 8); ok || l != "" {
		t.Errorf("no entropy: %q", l)
	}
	s.rand = rand.Reader
	st.found = false
	if l, ok := s.IssueDeleteLink(ctx, 9); ok || l != "" {
		t.Errorf("gone: %q", l)
	}
	st.found, st.err = true, errors.New("disk")
	if l, ok := s.IssueDeleteLink(ctx, 10); ok || l != "" {
		t.Errorf("store error: %q", l)
	}
	if got := strings.Count(logs.String(), `"level":"WARN","msg":"delete key not issued","id":`); got != 3 || strings.Contains(logs.String(), "disk") {
		t.Errorf("log %s", logs.String())
	}
	for _, k := range []string{key, key2} {
		if strings.Contains(logs.String(), k) {
			t.Error("a key in the log")
		}
	}

	// Clear (DK-5).
	logs.Reset()
	st.err, st.pruneN = nil, 2
	if err := s.ClearDeleteKeys(ctx); err != nil || !strings.Contains(logs.String(), `"msg":"delete keys cleared","removed":2`) {
		t.Errorf("clear: %v, log %s", err, logs.String())
	}
}

// TestLabels: every reason has its INF-4 label.
func TestLabels(t *testing.T) {
	if len(Labels) != len(Reasons) {
		t.Fatal(Labels)
	}
	for _, r := range Reasons {
		if Labels[r] == "" {
			t.Errorf("no label for %s", r)
		}
	}
}

type countingStore struct {
	*fakeStore
	consumes atomic.Int32
}

func (c *countingStore) ConsumeDeleteKey(ctx context.Context, h [32]byte, a, n time.Time) (string, string, bool, error) {
	c.consumes.Add(1)
	return c.fakeStore.ConsumeDeleteKey(ctx, h, a, n)
}

type blockingDeleter struct {
	entered, release chan struct{}
}

func (d *blockingDeleter) DeleteByOperator(context.Context, lifecycle.Record) error {
	d.entered <- struct{}{}
	<-d.release
	return nil
}

// TestRedeemSerialised (DK-8, QA-37-r1-3): while one redeem sits between its
// consume and its delete, a second must not even consume.
func TestRedeemSerialised(t *testing.T) {
	st := &countingStore{fakeStore: &fakeStore{found: true}}
	del := &blockingDeleter{make(chan struct{}, 2), make(chan struct{})}
	s := New(Deps{Store: st, Deleter: del, BaseURL: "https://wx.example", Now: time.Now, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))})
	link, _ := s.IssueDeleteLink(context.Background(), 1)
	key := strings.TrimPrefix(link, "https://wx.example/report-delete#")
	done := make(chan error, 2)
	go func() { done <- s.RedeemDeleteKey(context.Background(), key) }()
	<-del.entered
	go func() { done <- s.RedeemDeleteKey(context.Background(), key) }()
	time.Sleep(100 * time.Millisecond) // a second consume without the lock happens within microseconds
	if n := st.consumes.Load(); n != 1 {
		t.Errorf("%d consumes while the first redeem is unfinished, want 1", n)
	}
	close(del.release)
	first, second := <-done, <-done
	if n := btoi(first == nil) + btoi(second == nil); n != 1 {
		t.Errorf("results %v, %v", first, second)
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
