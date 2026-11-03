package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/privatedir"
	"workout-explorer/internal/publish"
	"workout-explorer/internal/store"
)

// Exit codes of delete (F-27 R-9).
const (
	exitDeleted  = 0
	exitFailed   = 1 // not deleted, or not completely; also "answered no"
	exitUsage    = 2
	exitNotFound = 3
)

// isTerminal reports whether f is a character device (a terminal under
// `docker compose exec`); shortcut: /dev/null counts too, and then the
// question reads EOF, which is "no".
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// deleteCmd is `workout-explorer delete -data <dir> [-y] <public_id>` (F-27
// R-8): the operator's delete of one workout in any state, through the one
// delete path (I-4). It reads no WORKOUT_EXPLORER_* setting. Human output goes
// to stdout, log lines (JSON) to stderr.
func deleteCmd(args []string, stdin io.Reader, terminal bool, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	data := fs.String("data", "", "data directory (required; /srv/workouts in production)")
	yes := fs.Bool("y", false, "delete without asking")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: workout-explorer delete -data <dir> [-y] <public_id>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	if *data == "" {
		fmt.Fprintln(stderr, "delete: -data must not be empty")
		return exitUsage
	}
	pub := fs.Arg(0)
	if !lifecycle.ValidPublicID(pub) {
		fmt.Fprintln(stderr, "delete: not a public ID (22 letters and digits)")
		return exitUsage
	}
	if !*yes && !terminal {
		fmt.Fprintln(stderr, "delete: no terminal to ask on; add -y to delete without asking")
		return exitUsage
	}

	ctx := context.Background()
	db, err := store.OpenExisting(ctx, filepath.Join(*data, "workouts.db"))
	if err != nil {
		fmt.Fprintf(stderr, "delete: %v\n", err)
		return exitFailed
	}
	defer db.Close()
	rec, found, err := db.WorkoutByPublicID(ctx, pub)
	if err != nil {
		fmt.Fprintf(stderr, "delete: %v\n", err)
		return exitFailed
	}
	if !found {
		fmt.Fprintln(stderr, "delete: no workout with this ID")
		return exitNotFound
	}
	n, err := db.ReportCount(ctx, rec.ID)
	if err != nil {
		fmt.Fprintf(stderr, "delete: %v\n", err)
		return exitFailed
	}
	state := string(rec.Visibility)
	if !rec.Live(time.Now()) {
		state += ", past its deadline"
	}
	// %q: the title is the sharer's text; never let it drive the terminal.
	fmt.Fprintf(stdout, "public_id  %s\nstate      %s\ncreated    %s\nexpires    %s\ntitle      %q\nreports    %d\n",
		rec.PublicID, state, rec.CreatedAt.UTC().Format(time.RFC3339), rec.ExpiresAt.UTC().Format(time.RFC3339), rec.Title, n)
	if !*yes {
		fmt.Fprint(stdout, "Delete? [y/N] ")
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Fprintln(stdout, "Not deleted.")
			return exitFailed
		}
	}

	files, err := privatedir.Open(*data)
	if err != nil {
		fmt.Fprintf(stderr, "delete: %v\n", err)
		return exitFailed
	}
	public, err := publish.Open(*data, publish.Deps{}) // Remove only: no template, parser or total
	if err != nil {
		fmt.Fprintf(stderr, "delete: %v\n", err)
		return exitFailed
	}
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	svc := lifecycle.New(lifecycle.Deps{Repo: db, Files: files, Public: public, Logger: logger})
	if err := svc.DeleteByOperator(ctx, rec); err != nil {
		if _, still, xerr := db.WorkoutByPublicID(ctx, pub); xerr == nil && !still {
			fmt.Fprintf(stderr, "delete: partly deleted: the row and its reports are gone, some files are not (%v); "+
				"serve removes a public folder within about a minute and a private folder at its next start\n", err)
		} else {
			fmt.Fprintf(stderr, "delete: not deleted: %v\n", err)
		}
		return exitFailed
	}
	fmt.Fprintln(stdout, "Deleted.")
	return exitDeleted
}
