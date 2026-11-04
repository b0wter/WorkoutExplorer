package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"workout-explorer/internal/datafile"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/precompress"
	"workout-explorer/internal/workout"
)

const parseUsage = `usage: workout-explorer parse [-datafile <path>] [-original <path>] <file.fit>

Prints the normalised workout of a FIT file as a JSON report (diagnostic),
or, for a file that is rejected, the rejection report. The report contains
position statistics; it is written to stdout only and never logged.

Environment: WORKOUT_EXPLORER_PARSE_TIMEOUT, the time limit in whole seconds
(5 to 60, default 10).

Exit status: 0 accepted, 3 rejected (rejection report on stdout, one line on
stderr), 2 bad arguments, 1 any other failure.

  -datafile <path>
        also write the data file to <path> and its brotli encoding to
        <path>.br (the folder must exist). Both contain coordinates.
  -original <path>
        run the upload pipeline (parse, strip identifying data, reparse) and
        write the file that would be stored as original.fit to <path>. With
        -datafile, the data file is made from the reparse of that file. The
        output contains the route; keep it local.
`

// parse implements the diagnostic "parse" subcommand (F-02 design, section
// parse subcommand; F-03 design, section CLI; F-08 design, section CLI and
// corpus driver): exit 0 with the report on stdout, 3 with the rejection
// report on stdout and "parse: rejected (<kind>): <message>" on stderr when
// the file is rejected, 2 for bad arguments, 1 with one "parse: <error>"
// line when the file cannot be read or encoded, or the data file cannot be
// written.
func parse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, parseUsage) }
	dataPath := fs.String("datafile", "", "")
	originalPath := fs.String("original", "", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	timeout, err := parseTimeout(os.Getenv)
	if err != nil {
		fmt.Fprintf(stderr, "parse: %v\n", err)
		return 2
	}
	err = parseFile(fs.Arg(0), *dataPath, *originalPath, timeout, stdout)
	var rejected *ingest.Error
	switch {
	case errors.As(err, &rejected):
		report, jerr := ingest.RejectionJSON(rejected)
		if jerr != nil {
			fmt.Fprintf(stderr, "parse: %v\n", jerr)
			return 1
		}
		if _, werr := stdout.Write(report); werr != nil {
			fmt.Fprintf(stderr, "parse: %v\n", werr)
			return 1
		}
		fmt.Fprintf(stderr, "parse: rejected (%s): %s\n", rejected.Kind, rejected.Message())
		return 3
	case err != nil:
		fmt.Fprintf(stderr, "parse: %v\n", err)
		return 1
	}
	return 0
}

// parseFile parses the FIT file at path and writes its golden report to
// stdout. With dataPath set, it first builds the data file and its brotli
// encoding in memory and writes them to dataPath and dataPath.br; nothing is
// printed unless every step succeeded. A rejected file returns the
// *ingest.Error and writes nothing, also not with dataPath. With
// originalPath set the file goes through ingest.Accept instead of Parse and
// the stripped file is written there (F-09 design, parse -original); the
// report and the data file then come from the reparse of that file.
func parseFile(path, dataPath, originalPath string, timeout time.Duration, stdout io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// Read at most one byte more than the limit, so a huge file is not loaded
	// whole; the parser then rejects it with limit (bytes).
	data, err := io.ReadAll(io.LimitReader(f, int64(ingest.DefaultLimits.MaxBytes)+1))
	if err != nil {
		return err
	}
	parser := newParser(timeout, false, nil)
	var w workout.Workout
	var original []byte
	if originalPath != "" {
		acc, err := parser.Accept(context.Background(), data)
		if err != nil {
			return err
		}
		w, original = acc.Workout, acc.Original
	} else {
		w, err = parser.Parse(context.Background(), data)
		if err != nil {
			return err
		}
	}
	report, err := workout.ReportJSON(w)
	if err != nil {
		return err
	}
	if dataPath != "" {
		bin, err := datafile.Encode(w)
		if err != nil {
			return err
		}
		br, err := precompress.Brotli(bin)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dataPath, bin, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(dataPath+".br", br, 0o644); err != nil {
			return err
		}
	}
	if originalPath != "" {
		if err := os.WriteFile(originalPath, original, 0o644); err != nil {
			return err
		}
	}
	_, err = stdout.Write(report)
	return err
}
