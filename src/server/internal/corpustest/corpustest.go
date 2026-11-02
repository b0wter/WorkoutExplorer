// Package corpustest lists the parser corpus for tests (ADR-004 item 4). It
// is imported by _test.go files only and never enters the binary.
package corpustest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Sets are the corpus folders with FIT files, in the order the drivers walk
// them; expected/<set>/ holds their golden reports.
var Sets = []string{"real", "synthetic"}

// Files returns the FIT files of root/set, sorted. A missing folder gives
// none. Hidden files are skipped; every other entry that is not a regular
// lower-case *.fit file fails the test (ADR-004 item 4).
func Files(t testing.TB, root, set string) []string {
	t.Helper()
	dir := filepath.Join(root, set)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".fit") {
			t.Errorf("%s is not a corpus file: rename to <brand>-<model>-<what>.fit", path)
			continue
		}
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}
