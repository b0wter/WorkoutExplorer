package privatedir

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"workout-explorer/internal/lifecycle"
)

var id1 = strings.Repeat("ab", 16)

func files() []lifecycle.File {
	return []lifecycle.File{{Name: lifecycle.OriginalName, Data: []byte("ORIGINAL")}, {Name: lifecycle.DraftName, Data: []byte("DRAFT")}}
}

func open(t *testing.T) (*Dir, string) {
	t.Helper()
	root := t.TempDir()
	d, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return d, root
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestCreate(t *testing.T) {
	d, root := open(t)
	if err := d.Create(id1, files()); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{lifecycle.OriginalName: "ORIGINAL", lifecycle.DraftName: "DRAFT"} {
		p := filepath.Join(root, "private", id1, name)
		got, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(got, []byte(want)) {
			t.Errorf("%s: %q, %v", name, got, err)
		}
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want 0600", name, st.Mode().Perm())
		}
	}
	for _, p := range []string{filepath.Join(root, "private"), filepath.Join(root, "private", id1), filepath.Join(root, "tmp")} {
		if st, _ := os.Stat(p); st.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %v, want 0700", p, st.Mode().Perm())
		}
	}
	if got := entries(t, filepath.Join(root, "tmp")); len(got) != 0 {
		t.Errorf("tmp/ holds %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "public")); err == nil {
		t.Errorf("public/ was created")
	}
}

func TestCreateFails(t *testing.T) {
	t.Run("existing non-empty folder", func(t *testing.T) {
		d, root := open(t)
		if err := d.Create(id1, files()); err != nil {
			t.Fatal(err)
		}
		if err := d.Create(id1, []lifecycle.File{{Name: lifecycle.DraftName, Data: []byte("OTHER")}}); err == nil {
			t.Fatal("second Create succeeded")
		}
		if got := entries(t, filepath.Join(root, "tmp")); len(got) != 0 {
			t.Errorf("tmp/ holds %v", got)
		}
		b, _ := os.ReadFile(filepath.Join(root, "private", id1, lifecycle.DraftName))
		if string(b) != "DRAFT" {
			t.Errorf("existing folder changed: %q", b)
		}
	})
	t.Run("rename cannot succeed", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory permissions")
		}
		d, root := open(t)
		if err := os.Chmod(filepath.Join(root, "private"), 0o500); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(filepath.Join(root, "private"), 0o700)
		if err := d.Create(id1, files()); err == nil {
			t.Fatal("Create succeeded in a read-only private/")
		}
		if got := entries(t, filepath.Join(root, "tmp")); len(got) != 0 {
			t.Errorf("tmp/ holds %v", got)
		}
		if got := entries(t, filepath.Join(root, "private")); len(got) != 0 {
			t.Errorf("private/ holds %v", got)
		}
	})
}

func TestNames(t *testing.T) {
	d, root := open(t)
	for _, id := range []string{"../x", strings.ToUpper(id1), id1[:31], "", id1 + "0", "../../etc"} {
		if err := d.Create(id, files()); err == nil {
			t.Errorf("Create(%q) succeeded", id)
		}
		if _, err := d.Remove(id); err == nil {
			t.Errorf("Remove(%q) succeeded", id)
		}
	}
	for _, name := range []string{"../original.fit", "other.bin", "draft.bin.br", "", "sub/draft.bin"} {
		if err := d.Create(id1, []lifecycle.File{{Name: name, Data: []byte("x")}}); err == nil {
			t.Errorf("file name %q accepted", name)
		}
	}
	if got := entries(t, filepath.Join(root, "tmp")); len(got) != 0 {
		t.Errorf("tmp/ holds %v", got)
	}
	if got := entries(t, filepath.Join(root, "private")); len(got) != 0 {
		t.Errorf("private/ holds %v", got)
	}
}

func TestRemoveIDsStaging(t *testing.T) {
	d, root := open(t)
	if _, err := d.Remove(id1); err != nil {
		t.Errorf("Remove of an absent ID: %v", err)
	}
	id2 := strings.Repeat("cd", 16)
	for _, id := range []string{id1, id2} {
		if err := d.Create(id, files()); err != nil {
			t.Fatal(err)
		}
	}
	// Entries we did not make.
	if err := os.Mkdir(filepath.Join(root, "private", "not-an-id"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private", id1[:30]+"zz"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ids, unknown, err := d.IDs()
	if err != nil || len(ids) != 2 || unknown != 2 {
		t.Fatalf("IDs = %v, %d, %v", ids, unknown, err)
	}
	if _, err := d.Remove(id1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "private", id1)); err == nil {
		t.Errorf("folder still there")
	}
	// Staging.
	if err := os.MkdirAll(filepath.Join(root, "tmp", "x", "y"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tmp", "f"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := d.ClearStaging()
	if err != nil || n != 2 || len(entries(t, filepath.Join(root, "tmp"))) != 0 {
		t.Errorf("ClearStaging = %d, %v", n, err)
	}
}

// TestUsageAndFreed (F-10): Usage sums regular file sizes under a root,
// links not followed; Remove returns the bytes its folder held.
func TestUsageAndFreed(t *testing.T) {
	d, root := open(t)
	if n, err := Usage(root); err != nil || n != 0 {
		t.Fatalf("empty: %d, %v", n, err)
	}
	if err := d.Create(id1, files()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workouts.db"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "public", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "x", "index.html"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A link to a big file outside the root counts 0.
	big := filepath.Join(t.TempDir(), "big")
	if err := os.WriteFile(big, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(big, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	want := int64(len("ORIGINAL") + len("DRAFT") + 4096 + 10)
	if n, err := Usage(root); err != nil || n != want {
		t.Errorf("Usage = %d, %v; want %d", n, err, want)
	}
	if _, err := Usage(filepath.Join(root, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("absent root: %v", err)
	}
	freed, err := d.Remove(id1)
	if err != nil || freed != int64(len("ORIGINAL")+len("DRAFT")) {
		t.Errorf("Remove = %d, %v", freed, err)
	}
	if freed, err := d.Remove(id1); err != nil || freed != 0 {
		t.Errorf("Remove of an absent folder = %d, %v", freed, err)
	}
}

func TestRead(t *testing.T) {
	d, _ := open(t)
	id := strings.Repeat("a", 32)
	if err := d.Create(id, []lifecycle.File{{Name: lifecycle.OriginalName, Data: []byte("abc")}}); err != nil {
		t.Fatal(err)
	}
	f, size, err := d.Read(id, lifecycle.OriginalName)
	if err != nil || size != 3 {
		t.Fatal(size, err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "abc" {
		t.Errorf("%q", b)
	}
	if b, err := d.Original(id); err != nil || string(b) != "abc" {
		t.Errorf("Original: %q %v", b, err)
	}
	if _, err := d.Original("../x"); err == nil {
		t.Error("Original of an invalid ID")
	}
	if _, _, err := d.Read(id, lifecycle.DraftName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("absent file: %v", err)
	}
	if _, _, err := d.Read(strings.Repeat("b", 32), lifecycle.DraftName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("absent folder: %v", err)
	}
	for _, c := range [][2]string{{"../x", "original.fit"}, {id, "../original.fit"}, {id, "other"}, {id, ""}} {
		if _, _, err := d.Read(c[0], c[1]); err == nil || errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%q: %v", c, err)
		}
	}
}

// TestReplace (F-30 RG-4 (a)): the file is replaced through tmp/, nothing is
// left in tmp/, and a missing folder is never created.
func TestReplace(t *testing.T) {
	d, root := open(t)
	id := "0123456789abcdef0123456789abcdef"
	if err := d.Create(id, []lifecycle.File{{Name: lifecycle.OriginalName, Data: []byte("a")}, {Name: lifecycle.DraftName, Data: []byte("b")}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Replace(id, lifecycle.DraftName, []byte("new")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "private", id, lifecycle.DraftName))
	info, _ := os.Stat(filepath.Join(root, "private", id, lifecycle.DraftName))
	if string(b) != "new" || info.Mode().Perm() != 0o600 || len(entries(t, filepath.Join(root, "tmp"))) != 0 {
		t.Errorf("%q %v tmp %v", b, info.Mode(), entries(t, filepath.Join(root, "tmp")))
	}
	gone := "fedcba9876543210fedcba9876543210"
	if err := d.Replace(gone, lifecycle.DraftName, []byte("x")); err == nil {
		t.Error("replaced into a missing folder")
	}
	if _, err := os.Lstat(filepath.Join(root, "private", gone)); !os.IsNotExist(err) || len(entries(t, filepath.Join(root, "tmp"))) != 0 {
		t.Errorf("created the folder or left staging: %v", err)
	}
	for _, c := range [][2]string{{"../x", lifecycle.DraftName}, {id, "other"}} {
		if err := d.Replace(c[0], c[1], nil); err == nil {
			t.Errorf("%v accepted", c)
		}
	}
}
