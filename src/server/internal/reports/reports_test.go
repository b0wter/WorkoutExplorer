package reports

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
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

type fakeNotifier struct{ got []string }

func (n *fakeNotifier) Notify(id int64, reason string) {
	n.got = append(n.got, note(id, reason))
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
	s := New(st, n, func() time.Time { return now }, slog.New(slog.NewJSONHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.File(ctx, pub, "spam", " SECRETMESSAGE "); err != nil {
		t.Fatal(err)
	}
	if st.got[0] != pub || st.got[1] != "spam" || st.got[2] != "SECRETMESSAGE" || !st.gotNow.Equal(now) || st.ctxCancels {
		t.Errorf("store got %v at %v, cancellable ctx %v", st.got, st.gotNow, st.ctxCancels)
	}
	if len(n.got) != 1 || n.got[0] != note(7, "spam") {
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
	if err := New(st, nil, nil, nil).File(ctx, pub, "spam", ""); err != nil {
		t.Error(err)
	}
}

func TestPrune(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var logs bytes.Buffer
	st := &fakeStore{pruneN: 2}
	s := New(st, nil, func() time.Time { return now }, slog.New(slog.NewJSONHandler(&logs, nil)))
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
