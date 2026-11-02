package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"workout-explorer/internal/httpapi"
)

// waitLimit bounds every wait so a regression fails instead of hanging.
const waitLimit = 5 * time.Second

// TestRun covers the exit-code contract of the command line
// (F-01 design, Server, "Exit codes").
func TestRun(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer busy.Close()

	tests := []struct {
		name       string
		args       []string
		want       int
		wantStderr bool
		stderrHas  string // substring stderr must contain, if set
	}{
		{"no args", nil, 2, true, ""},
		{"unknown command", []string{"nosuchcommand"}, 2, true, ""},
		{"serve extra positional", []string{"serve", "-data", t.TempDir(), "extra"}, 2, true, ""},
		{"serve unknown flag", []string{"serve", "-nosuchflag"}, 2, true, ""},
		{"serve help", []string{"serve", "-h"}, 0, true, ""},
		{"serve empty addr", []string{"serve", "-data", t.TempDir(), "-web", testWeb, "-addr", ""}, 2, true, "-addr must not be empty"},
		{"serve busy address", []string{"serve", "-data", t.TempDir(), "-web", testWeb, "-addr", busy.Addr().String()}, 1, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			done := make(chan int, 1)
			go func() { done <- run(tt.args, io.Discard, &stderr) }()

			var got int
			select {
			case got = <-done:
			case <-time.After(waitLimit):
				t.Fatalf("run(%q) did not return within %v", tt.args, waitLimit)
			}
			if got != tt.want {
				t.Errorf("run(%q) = %d, want %d (stderr: %q)", tt.args, got, tt.want, stderr.String())
			}
			if tt.wantStderr && stderr.Len() == 0 {
				t.Errorf("run(%q): want usage on stderr, got nothing", tt.args)
			}
			if tt.stderrHas != "" && !strings.Contains(stderr.String(), tt.stderrHas) {
				t.Errorf("run(%q): stderr %q does not contain %q", tt.args, stderr.String(), tt.stderrHas)
			}
		})
	}
}

// syncBuffer is a bytes.Buffer safe for the concurrent writes of the
// server goroutine and the reads of the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startServer runs runServer in a goroutine on a fresh 127.0.0.1:0 listener.
// It returns the base URL, the cancel func, the log buffer and the channel
// that receives runServer's result.
func startServer(t *testing.T, handler http.Handler) (string, context.CancelFunc, *syncBuffer, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: readHeaderTimeout}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	result := make(chan error, 1)
	go func() { result <- runServer(ctx, srv, ln, logger) }()
	return "http://" + ln.Addr().String(), cancel, logs, result
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(waitLimit):
		t.Fatalf("runServer did not return within %v", waitLimit)
		return nil
	}
}

var client = &http.Client{Timeout: waitLimit}

// (a) health answers while running; cancel stops cleanly and both
// lifecycle lines are logged.
func TestRunServerHealthAndStop(t *testing.T) {
	base, cancel, logs, result := startServer(t, httpapi.NewHandler(httpapi.Config{Version: version, MaxConcurrentUploads: 4}))

	resp, err := client.Get(base + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/health: status %d, want 200", resp.StatusCode)
	}

	cancel()
	if err := waitResult(t, result); err != nil {
		t.Fatalf("runServer returned %v, want nil", err)
	}

	out := logs.String()
	addr := strings.TrimPrefix(base, "http://")
	if !strings.Contains(out, `"msg":"server started"`) || !strings.Contains(out, `"addr":"`+addr+`"`) {
		t.Errorf("log lacks \"server started\" with addr %s:\n%s", addr, out)
	}
	if !strings.Contains(out, `"msg":"server stopped"`) {
		t.Errorf("log lacks \"server stopped\":\n%s", out)
	}
}

// (b) graceful shutdown: an in-flight request survives cancel and completes.
func TestRunServerGracefulShutdown(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	base, cancel, _, result := startServer(t, handler)

	type reply struct {
		status int
		err    error
	}
	replies := make(chan reply, 1)
	go func() {
		resp, err := client.Get(base + "/slow")
		if err != nil {
			replies <- reply{err: err}
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		replies <- reply{status: resp.StatusCode}
	}()

	select {
	case <-entered:
	case <-time.After(waitLimit):
		close(release)
		t.Fatalf("handler not reached within %v", waitLimit)
	}

	cancel()

	// Wait until Shutdown has closed the listener, so the release below
	// really happens during shutdown.
	addr := strings.TrimPrefix(base, "http://")
	deadline := time.Now().Add(waitLimit)
	for {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			break
		}
		c.Close()
		if time.Now().After(deadline) {
			close(release)
			t.Fatalf("listener still accepting %v after cancel", waitLimit)
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case err := <-result:
		close(release)
		t.Fatalf("runServer returned %v before the in-flight request finished", err)
	default:
	}

	close(release)

	select {
	case r := <-replies:
		if r.err != nil {
			t.Fatalf("in-flight request failed: %v", r.err)
		}
		if r.status != http.StatusOK {
			t.Fatalf("in-flight request: status %d, want 200", r.status)
		}
	case <-time.After(waitLimit):
		t.Fatalf("in-flight request did not complete within %v", waitLimit)
	}

	if err := waitResult(t, result); err != nil {
		t.Fatalf("runServer returned %v, want nil", err)
	}
}

// (c) a listener that is already closed makes runServer fail and log it.
func TestRunServerListenerClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ln.Close()

	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	srv := &http.Server{Handler: httpapi.NewHandler(httpapi.Config{Version: version, MaxConcurrentUploads: 4})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() { result <- runServer(ctx, srv, ln, logger) }()

	if err := waitResult(t, result); err == nil {
		t.Fatal("runServer returned nil, want an error")
	}
	if out := logs.String(); !strings.Contains(out, `"msg":"server failed"`) {
		t.Errorf("log lacks \"server failed\":\n%s", out)
	}
}

// The "server started" line carries the effective upload setting (F-09 design, Wiring).
func TestRunServerLogsUploadSetting(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	srv := &http.Server{Handler: http.NotFoundHandler()}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- runServer(ctx, srv, ln, logger, startedFields(settings{maxConcurrentUploads: 3, parseTimeout: 25 * time.Second, uploadsIPv4: 7, uploadsIPv6: 9})...)
	}()
	deadline := time.Now().Add(waitLimit)
	for !strings.Contains(logs.String(), "server started") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	waitResult(t, result)
	if !strings.Contains(logs.String(), `"max_concurrent_uploads":3`) || !strings.Contains(logs.String(), `"parse_timeout_s":25`) ||
		!strings.Contains(logs.String(), `"uploads_per_hour_ipv4":7`) || !strings.Contains(logs.String(), `"uploads_per_hour_ipv6":9`) {
		t.Errorf("log: %s", logs.String())
	}
}
