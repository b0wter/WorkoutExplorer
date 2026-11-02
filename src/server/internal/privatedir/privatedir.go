// Package privatedir is the only writer under <data>/private and <data>/tmp
// (ADR-022 item 7): staged, fsynced creation of a workout's private folder,
// removal, listing and the startup sweep of staging. Every ID and file name is
// checked before a path is built.
package privatedir

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"workout-explorer/internal/lifecycle"
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Dir is the private file area of one data directory.
type Dir struct {
	private, tmp string
}

// Open creates <data>/private and <data>/tmp (mode 0700) if missing. The data
// directory itself must exist.
func Open(dataDir string) (*Dir, error) {
	d := &Dir{private: filepath.Join(dataDir, "private"), tmp: filepath.Join(dataDir, "tmp")}
	for _, p := range []string{d.private, d.tmp} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			return nil, fmt.Errorf("privatedir: %w", err)
		}
	}
	return d, nil
}

// ValidID reports whether id is an internal workout ID (32 lowercase hex).
func ValidID(id string) bool { return idPattern.MatchString(id) }

func validName(name string) bool {
	return name == lifecycle.OriginalName || name == lifecycle.DraftName
}

// Create writes files into tmp/{id}, fsyncs them, and renames the folder to
// private/{id} (ADR-022 item 3). It fails if private/{id} exists. On any error
// nothing is left behind.
func (d *Dir) Create(id string, files []lifecycle.File) (err error) {
	if !ValidID(id) {
		return errors.New("privatedir: invalid workout ID")
	}
	for _, f := range files {
		if !validName(f.Name) {
			return errors.New("privatedir: invalid file name")
		}
	}
	staged, final := filepath.Join(d.tmp, id), filepath.Join(d.private, id)
	if _, err := os.Lstat(final); err == nil {
		return fmt.Errorf("privatedir: %s exists", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("privatedir: %w", err)
	}
	if err := os.Mkdir(staged, 0o700); err != nil {
		return fmt.Errorf("privatedir: %w", err)
	}
	renamed := false
	defer func() {
		if err != nil {
			_ = os.RemoveAll(staged)
			if renamed {
				_ = os.RemoveAll(final)
			}
		}
	}()
	for _, f := range files {
		if err := writeSynced(filepath.Join(staged, f.Name), f.Data); err != nil {
			return err
		}
	}
	if err := syncDir(staged); err != nil {
		return err
	}
	if err := os.Rename(staged, final); err != nil {
		return fmt.Errorf("privatedir: %w", err)
	}
	renamed = true
	return syncDir(d.private)
}

func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("privatedir: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("privatedir: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("privatedir: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("privatedir: %w", err)
	}
	return nil
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("privatedir: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("privatedir: %w", err)
	}
	return nil
}

// Read opens one file of private/{id} for reading and returns its size (F-12
// O-8). A missing file is an error wrapping fs.ErrNotExist. The caller closes it.
func (d *Dir) Read(id, name string) (io.ReadCloser, int64, error) {
	if !ValidID(id) || !validName(name) {
		return nil, 0, errors.New("privatedir: invalid workout ID or file name")
	}
	f, err := os.Open(filepath.Join(d.private, id, name))
	if err != nil {
		return nil, 0, fmt.Errorf("privatedir: %w", err)
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = errors.New("not a regular file")
	}
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("privatedir: %w", err)
	}
	return f, info.Size(), nil
}

// Original returns the bytes of private/{id}/original.fit (F-15 P-3: the
// public build reads it whole; at most the 20 MB upload cap).
func (d *Dir) Original(id string) ([]byte, error) {
	f, _, err := d.Read(id, lifecycle.OriginalName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("privatedir: %w", err)
	}
	return b, nil
}

// Remove deletes private/{id} and returns the sizes of the files it held
// (F-10's running total); it is 0 and nil if the folder is absent. On an
// error freed is 0: the folder may be partly removed, and counting too much
// is the safe side until the next start recomputes the total.
func (d *Dir) Remove(id string) (freed int64, err error) {
	if !ValidID(id) {
		return 0, errors.New("privatedir: invalid workout ID")
	}
	folder := filepath.Join(d.private, id)
	size, err := Usage(folder)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("privatedir: %w", err)
	}
	if err := os.RemoveAll(folder); err != nil {
		return 0, fmt.Errorf("privatedir: %w", err)
	}
	return size, nil
}

// Usage is the sum of the sizes of the regular files under root, root
// included if it is a file (F-10 design: the measure of the storage total).
// Directories, symbolic links and other entries count 0; links are not
// followed. It only reads. A root that does not exist is an error wrapping
// os.ErrNotExist.
func Usage(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.Type().IsRegular() {
			return nil
		}
		info, err := e.Info()
		if errors.Is(err, os.ErrNotExist) {
			return nil // removed while walking
		}
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

// IDs lists the folders of private/ whose names are internal IDs; unknown
// counts every other entry (they were not made by us and are left alone).
func (d *Dir) IDs() (ids []string, unknown int, err error) {
	entries, err := os.ReadDir(d.private)
	if err != nil {
		return nil, 0, fmt.Errorf("privatedir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && ValidID(e.Name()) {
			ids = append(ids, e.Name())
		} else {
			unknown++
		}
	}
	return ids, unknown, nil
}

// ClearStaging removes everything in tmp/ and returns how many entries it removed.
func (d *Dir) ClearStaging() (removed int, err error) {
	entries, err := os.ReadDir(d.tmp)
	if err != nil {
		return 0, fmt.Errorf("privatedir: %w", err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(d.tmp, e.Name())); err != nil {
			return removed, fmt.Errorf("privatedir: %w", err)
		}
		removed++
	}
	return removed, nil
}
