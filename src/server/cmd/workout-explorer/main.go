// Command workout-explorer is the single server binary of Workout Explorer.
//
// Usage:
//
//	workout-explorer serve -data <dir> -web <dir> [-addr 127.0.0.1:8080]
//	workout-explorer parse [-datafile <path>] [-original <path>] <file.fit>
//
// main only parses flags, builds the logger and handler, and runs the
// server; no logic beyond wiring (Technical Design, Code structure:
// Dependency rules).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// version is set at build time with
// -ldflags "-X main.version=$(git describe --always --dirty)".
var version = "dev"

const (
	defaultAddr       = "127.0.0.1:8080" // Must equal DEV_ADDR's default in the Makefile and the fallback in src/web/vite.config.ts (F-01 D-4).
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownGrace     = 10 * time.Second
)

const usage = `usage: workout-explorer <command> [flags]

commands:
  serve    run the HTTP API server (-data <dir> and -web <dir> are required)
  parse    print the normalised workout of a FIT file, or why it is rejected (diagnostic)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches the subcommand and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "serve":
		return serve(args[1:], stderr)
	case "parse":
		return parse(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func serve(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr, "listen address (host:port)")
	data := fs.String("data", "", "data directory (required; /srv/workouts in production)")
	web := fs.String("web", "", "deployed web build (required); its index.html is the page template")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "serve: unexpected arguments: %v\n", fs.Args())
		return 2
	}
	if *addr == "" {
		fmt.Fprintln(stderr, "serve: -addr must not be empty")
		return 2
	}

	if *data == "" {
		fmt.Fprintln(stderr, "serve: -data must not be empty")
		return 2
	}
	if *web == "" {
		fmt.Fprintln(stderr, "serve: -web must not be empty")
		return 2
	}
	// Settings are read before the database is opened, so an invalid value changes nothing on disk.
	cfg, err := readSettings(os.Getenv)
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 2
	}
	cfg.webDir = *web

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler, closeApp, err := newApp(ctx, *data, cfg, logger)
	if err != nil {
		logger.Error("startup failed", "error", err.Error())
		return 1
	}
	// closeApp is called explicitly on every return below, not deferred: its
	// error (the database close) must change the exit code (QA-09-r1-6).
	closeDB := func() bool {
		if err := closeApp(); err != nil {
			logger.Error("shutdown incomplete", "error", err.Error())
			return false
		}
		return true
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		// ReadTimeout and WriteTimeout are deliberately unset: large uploads
		// (F-09) set per-request deadlines via http.ResponseController.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	// Listen first so a bad or busy address fails before "started" is logged.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Error("listen failed", "addr", *addr, "error", err.Error())
		closeDB()
		return 1
	}

	serveErr := runServer(ctx, srv, ln, logger, startedFields(cfg)...)
	if !closeDB() || serveErr != nil {
		return 1
	}
	return 0
}

// startedFields are the limits logged on "server started" (ADR-021 rev. 6 item 7).
func startedFields(cfg settings) []any {
	return []any{"max_concurrent_uploads", cfg.maxConcurrentUploads, "parse_timeout_s", int(cfg.parseTimeout / time.Second), "uploads_per_hour_ipv4", cfg.uploadsIPv4, "uploads_per_hour_ipv6", cfg.uploadsIPv6}
}

// runServer serves on ln until ctx is cancelled, then shuts down with a
// grace period. It returns a non-nil error if serving or shutdown failed.
// extra are further key-value pairs for the "server started" line.
func runServer(ctx context.Context, srv *http.Server, ln net.Listener, logger *slog.Logger, extra ...any) error {
	logger.Info("server started", append([]any{"addr", ln.Addr().String(), "version", version}, extra...)...)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		// Serve returned without a shutdown request.
		logger.Error("server failed", "error", err.Error())
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown incomplete", "error", err.Error())
		return err
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err.Error())
		return err
	}
	logger.Info("server stopped")
	return nil
}
