package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/privatedir"
	"workout-explorer/internal/publish"
	"workout-explorer/internal/store"
)

// regenerateCmd is `workout-explorer regenerate -data <dir> -web <dir>` (F-30
// RG-1): every live workout is rebuilt from its original with this binary
// and template. Log lines (JSON) go to stderr, the report to stdout (RG-8).
// Exit 0: the whole list was processed; 1: it could not start or did not
// finish; 2: usage or settings.
func regenerateCmd(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("regenerate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	data := fs.String("data", "", "data directory (required; /srv/workouts in production)")
	web := fs.String("web", "", "deployed web build (required); its index.html is the page template")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	switch {
	case fs.NArg() > 0:
		fmt.Fprintf(stderr, "regenerate: unexpected arguments: %v\n", fs.Args())
		return 2
	case *data == "":
		fmt.Fprintln(stderr, "regenerate: -data must not be empty")
		return 2
	case *web == "":
		fmt.Fprintln(stderr, "regenerate: -web must not be empty")
		return 2
	}
	// Only the two settings the build uses, with serve's checks, before anything is opened.
	t, err := parseTimeout(getenv)
	if err != nil {
		fmt.Fprintf(stderr, "regenerate: %v\n", err)
		return 2
	}
	b, err := baseURL(getenv)
	if err != nil {
		fmt.Fprintf(stderr, "regenerate: %v\n", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return regenerate(ctx, *data, settings{parseTimeout: t, baseURL: b, webDir: *web}, stdout, stderr, nil)
}

// regenerate is the run; each, if set, is called after every workout (tests).
func regenerate(ctx context.Context, dataDir string, s settings, stdout, stderr io.Writer, each func(lifecycle.Outcome)) int {
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	failed := func(err error) int {
		logger.Error("regenerate failed", "error", pathless(err))
		return 1
	}
	tmpl, err := publish.LoadTemplate(s.webDir)
	if err != nil {
		return failed(err)
	}
	// An existing database of this binary's schema only; never migrated, never Recover (RG-1).
	db, err := store.OpenExisting(ctx, filepath.Join(dataDir, "workouts.db"))
	if err != nil {
		return failed(err)
	}
	defer db.Close()
	files, err := privatedir.Open(dataDir)
	if err != nil {
		return failed(err)
	}
	parser := newParser(s.parseTimeout, true, logger)
	public, err := publish.Open(dataDir, publish.Deps{Template: tmpl, Originals: files, Maps: files, Parser: parser, Logger: logger, BaseURL: s.baseURL, Rows: db})
	if err != nil {
		return failed(err)
	}
	svc := lifecycle.New(lifecycle.Deps{Parser: parser, Repo: db, Files: files, Hook: public, Public: public, Logger: logger})
	list, err := db.LiveWorkouts(ctx, time.Now())
	if err != nil {
		return failed(err)
	}
	counts := map[lifecycle.Outcome]int{}
	report := func() {
		for _, o := range []lifecycle.Outcome{lifecycle.Rebuilt, lifecycle.Deleted, lifecycle.Kept, lifecycle.Skipped} {
			fmt.Fprintf(stdout, "%s=%d\n", o, counts[o])
		}
	}
	done := 0
	for _, r := range list {
		if ctx.Err() != nil {
			break
		}
		l := logger.With("id", r.ID)
		o, err := svc.Regenerate(ctx, r.PublicID, parser.WithLogger(l))
		counts[o]++
		done++
		if err != nil {
			msg := "workout kept"
			if o == lifecycle.Deleted {
				msg = "workout deleted, files left until serve's next start"
			}
			l.Warn(msg, "error", pathless(err))
		}
		if each != nil {
			each(o)
		}
	}
	if counts[lifecycle.Deleted] > 0 {
		svc.Checkpoint(context.WithoutCancel(ctx)) // busy is fine: serve checkpoints every pass (F-27 R-10)
	}
	logger.Info("regenerate finished", "workouts", len(list), "processed", done,
		"rebuilt", counts[lifecycle.Rebuilt], "deleted", counts[lifecycle.Deleted], "kept", counts[lifecycle.Kept], "skipped", counts[lifecycle.Skipped])
	report()
	if done < len(list) || ctx.Err() != nil {
		fmt.Fprintln(stderr, "regenerate: stopped before the end; run it again")
		return 1
	}
	// RG-9: the assets the served pages still name, for the deploy's clean-up.
	assets, err := public.Assets()
	if err != nil {
		return failed(err)
	}
	for _, a := range assets {
		fmt.Fprintf(stdout, "asset=%s\n", a)
	}
	return 0
}

// pathless reduces path errors to the operation and the system error, so no
// path reaches a log line (RG-3; private paths hold the internal id only,
// public ones are already reduced by publish, P-14).
func pathless(err error) string {
	var pe *fs.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		return pe.Op + ": " + pe.Err.Error()
	case errors.As(err, &le):
		return le.Op + ": " + le.Err.Error()
	}
	return err.Error()
}
