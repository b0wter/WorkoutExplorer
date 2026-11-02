package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"

	"workout-explorer/internal/datafile"
)

// TestParseCommand covers the exit codes of the parse subcommand (F-02
// design, section parse subcommand).
func TestParseCommand(t *testing.T) {
	corpus := filepath.Join("..", "..", "testdata", "corpus")
	fit := filepath.Join(corpus, "synthetic", "smart-recording.fit")
	tests := []struct {
		name      string
		args      []string
		want      int
		stderrHas string
	}{
		{"no argument", []string{"parse"}, 2, "usage"},
		{"two arguments", []string{"parse", fit, fit}, 2, "usage"},
		{"bad flag", []string{"parse", "-nosuchflag", fit}, 2, ""},
		{"missing file", []string{"parse", filepath.Join(t.TempDir(), "nope.fit")}, 1, "parse: "},
		{"-datafile without a file", []string{"parse", "-datafile", filepath.Join(t.TempDir(), "x.bin")}, 2, "usage"},
		{"-datafile without a value", []string{"parse", "-datafile"}, 2, ""},
		{"-datafile folder missing", []string{"parse", "-datafile", filepath.Join(t.TempDir(), "nope", "x.bin"), fit}, 1, "parse: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.want {
				t.Errorf("exit = %d, want %d (stderr %q)", got, tt.want, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout not empty on error: %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.stderrHas) || stderr.Len() == 0 {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.stderrHas)
			}
			if tt.want == 1 && strings.Count(stderr.String(), "\n") != 1 {
				t.Errorf("stderr is not one line: %q", stderr.String())
			}
		})
	}

	t.Run("corpus file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if got := run([]string{"parse", fit}, &stdout, &stderr); got != 0 {
			t.Fatalf("exit = %d, want 0 (stderr %q)", got, stderr.String())
		}
		want, err := os.ReadFile(filepath.Join(corpus, "expected", "synthetic", "smart-recording.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(stdout.Bytes(), want) {
			t.Errorf("stdout differs from the golden report")
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr not empty: %q", stderr.String())
		}
	})
}

// TestParseDatafile covers -datafile (F-03 design, section CLI): both files
// are written, the .br decompresses to the .bin, the .bin decodes with the
// report's sample count, and stdout is still the golden report.
func TestParseDatafile(t *testing.T) {
	corpus := filepath.Join("..", "..", "testdata", "corpus")
	fit := filepath.Join(corpus, "synthetic", "gps-dropout.fit")
	path := filepath.Join(t.TempDir(), "data.bin")

	var stdout, stderr bytes.Buffer
	if got := run([]string{"parse", "-datafile", path, fit}, &stdout, &stderr); got != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr not empty: %q", stderr.String())
	}
	want, err := os.ReadFile(filepath.Join(corpus, "expected", "synthetic", "gps-dropout.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Errorf("stdout differs from the golden report")
	}

	bin, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	br, err := os.ReadFile(path + ".br")
	if err != nil {
		t.Fatal(err)
	}
	unbr, err := io.ReadAll(brotli.NewReader(bytes.NewReader(br)))
	if err != nil || !bytes.Equal(unbr, bin) {
		t.Errorf(".br does not decompress to the .bin (err %v)", err)
	}
	f, err := datafile.Decode(bin)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	var report struct {
		Samples struct{ Count int }
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if f.SampleCount != report.Samples.Count || f.SampleCount == 0 {
		t.Errorf("data file N = %d, report samples.count = %d", f.SampleCount, report.Samples.Count)
	}
}

func TestUsageListsParse(t *testing.T) {
	if !strings.Contains(usage, "parse    print the normalised workout of a FIT file, or why it is rejected (diagnostic)") {
		t.Errorf("usage does not list the parse command:\n%s", usage)
	}
}

// TestParseRejected: a rejected file exits 3 with the rejection report on
// stdout and one line on stderr; an accepted truncated file exits 0;
// -datafile writes nothing for a rejected file (F-08 design, CLI).
func TestParseRejected(t *testing.T) {
	corpus := filepath.Join("..", "..", "testdata", "corpus", "synthetic")

	course := filepath.Join(corpus, "course.fit")
	dataPath := filepath.Join(t.TempDir(), "data.bin")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"parse", "-datafile", dataPath, course}, &stdout, &stderr); got != 3 {
		t.Fatalf("exit = %d, want 3 (stderr %q)", got, stderr.String())
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "corpus", "expected", "synthetic", "course.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Errorf("stdout differs from the golden rejection report:\n%s", stdout.String())
	}
	if got, want := stderr.String(), "parse: rejected (unsupported_type): Unsupported FIT file type.\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	for _, p := range []string{dataPath, dataPath + ".br"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s written for a rejected file (err %v)", p, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"parse", "parse_test.go"}, &stdout, &stderr); got != 3 {
		t.Errorf("non-FIT file: exit = %d, want 3", got)
	}

	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"parse", filepath.Join(corpus, "ride-30min-cut-60.fit")}, &stdout, &stderr); got != 0 {
		t.Errorf("truncated file: exit = %d, want 0 (stderr %q)", got, stderr.String())
	}
}

// TestParseOriginal covers `parse -original` (F-09 design, parse -original):
// it writes the file that would be stored, exit 0; a rejected file writes
// nothing and exits 3.
func TestParseOriginal(t *testing.T) {
	synth := filepath.Join("..", "..", "testdata", "corpus", "synthetic")
	t.Run("identifying data is stripped", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "original.fit")
		data := filepath.Join(t.TempDir(), "data.bin")
		var stdout, stderr bytes.Buffer
		in := filepath.Join(synth, "identifying-data.fit")
		if got := run([]string{"parse", "-original", out, "-datafile", data, in}, &stdout, &stderr); got != 0 {
			t.Fatalf("exit = %d (stderr %q)", got, stderr.String())
		}
		stored, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		upload, _ := os.ReadFile(in)
		if bytes.Equal(stored, upload) || bytes.Contains(stored, []byte("Marker Person")) || !bytes.Contains(upload, []byte("Marker Person")) {
			t.Errorf("the written file is not the stripped upload")
		}
		// The report is the plain parse's.
		var plain bytes.Buffer
		if got := run([]string{"parse", in}, &plain, io.Discard); got != 0 || !bytes.Equal(plain.Bytes(), stdout.Bytes()) {
			t.Errorf("report differs from a plain parse (exit %d)", got)
		}
		if _, err := os.Stat(data); err != nil {
			t.Errorf("data file not written: %v", err)
		}
	})
	t.Run("rejected file writes nothing", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "original.fit")
		var stdout, stderr bytes.Buffer
		if got := run([]string{"parse", "-original", out, filepath.Join(synth, "course.fit")}, &stdout, &stderr); got != 3 {
			t.Fatalf("exit = %d, want 3 (stderr %q)", got, stderr.String())
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("a file was written for a rejected upload")
		}
		if !strings.Contains(stderr.String(), "Unsupported FIT file type.") {
			t.Errorf("stderr = %q", stderr.String())
		}
	})
}
