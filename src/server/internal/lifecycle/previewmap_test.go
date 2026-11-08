package lifecycle

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"

	"workout-explorer/internal/preview"
)

// F-36 MC-4, MC-5, MC-9 at the service (ADR-041).

func (f *fakeFiles) StoreMap(id, key string, data []byte) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	files, ok := f.dirs[id]
	if !ok {
		return 0, errors.New("no folder") // never created
	}
	var kept []File
	var freed int64
	for _, file := range files {
		if strings.HasPrefix(file.Name, "preview-map.") {
			freed += int64(len(file.Data))
			continue
		}
		kept = append(kept, file)
	}
	f.dirs[id] = append(kept, File{PreviewMapName(key), data})
	return freed, nil
}

type fakeFrames struct {
	mu    sync.Mutex
	fr    preview.Frame
	ok    bool
	calls int
	gate  chan struct{} // non-nil: Frame waits for it
	in    chan struct{} // non-nil: Frame signals entry
}

func (f *fakeFrames) Frame(context.Context, Record) (preview.Frame, bool, error) {
	if f.in != nil {
		f.in <- struct{}{}
	}
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.fr, f.ok, nil
}

const mapKey = "0123456789abcdef"

func mapPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, preview.Width, preview.Height))); err != nil {
		t.Fatal(err)
	}
	// An ancillary chunk after the signature and IHDR that must never be stored.
	b := buf.Bytes()
	chunk := []byte("\x00\x00\x00\x09tEXtComment\x00x")
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	return append(append(append([]byte{}, b[:33]...), chunk...), b[33:]...)
}

func mapService(t *testing.T) (*Service, *fakeRepo, *fakeFiles, *lines, Created, *fakeFrames, *fakeHook) {
	t.Helper()
	hook := &fakeHook{}
	s, r, f, l, c, _ := owned(t, hook)
	fr := &fakeFrames{fr: preview.Frame{Lon: 8, Lat: 50, Zoom: 12, Key: mapKey}, ok: true}
	s.d.Frames = fr
	return s, r, f, l, c, fr, hook
}

func storedMaps(f *fakeFiles) (n int, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, files := range f.dirs {
		for _, file := range files {
			if strings.HasPrefix(file.Name, "preview-map.") {
				n++
				data = file.Data
			}
		}
	}
	return n, data
}

// countingReader counts the bytes read from it.
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// TestPutPreviewMapRefusals (AC 39, 41; MC-4, MC-5): each refusal stores
// nothing, logs a fixed reason at most, and never the key.
func TestPutPreviewMapRefusals(t *testing.T) {
	ctx := context.Background()
	good := mapPNG(t)
	for _, c := range []struct {
		name, token, key string
		body             []byte
		want             string // error text part
		reason           string // logged reason; "" none
	}{
		{"no token", "", mapKey, good, "not found", ""},
		{"wrong token", strings.Repeat("A", 22), mapKey, good, "not found", ""},
		{"key not hex", "", "0123456789ABCDEF", good, "not found", ""},
		{"key short", "", "0123", good, "not found", ""},
		{"over the cap", "", mapKey, make([]byte, preview.MaxMapBytes+1), "size", "size"},
		{"not a PNG", "", mapKey, []byte("GIF89a"), "format", "format"},
		{"stale key", "", "fedcba9876543210", good, "frame changed", "stale"},
	} {
		s, _, f, l, cr, frames, hook := mapService(t)
		tok := cr.EditToken
		if c.name == "no token" {
			tok = ""
		} else if c.token != "" {
			tok = c.token
		}
		body := &countingReader{r: io.MultiReader(bytes.NewReader(c.body), strings.NewReader(strings.Repeat("x", 1<<20)))}
		if c.name != "over the cap" {
			body.r = bytes.NewReader(c.body)
		}
		err := s.PutPreviewMap(ctx, cr.PublicID, tok, c.key, body)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
		if n, _ := storedMaps(f); n != 0 || len(hook.calls) != 0 {
			t.Errorf("%s: stored %d, hook %d", c.name, n, len(hook.calls))
		}
		if c.want == "not found" && (frames.calls != 0 || body.n != 0) {
			t.Errorf("%s: parsed %d times, read %d bytes before the token and key check", c.name, frames.calls, body.n)
		}
		if c.name == "over the cap" && body.n > preview.MaxMapBytes+1 {
			t.Errorf("read on after the cap: %d bytes", body.n)
		}
		logs := l.buf.String()
		if c.reason != "" && (!strings.Contains(logs, `"msg":"preview map refused","id":"`) || !strings.Contains(logs, `"reason":"`+c.reason+`"`)) {
			t.Errorf("%s: log %s", c.name, logs)
		}
		if c.reason == "" && strings.Contains(logs, "preview map") {
			t.Errorf("%s: logged %s", c.name, logs)
		}
		if strings.Contains(logs, c.key) || strings.Contains(logs, mapKey) {
			t.Errorf("%s: key logged", c.name)
		}
	}
}

// TestPutPreviewMapStores (MC-4, MC-9): the re-encoded map is stored, the
// previous one replaced; a draft runs no build, a published workout an
// in-place build (before = after).
func TestPutPreviewMapStores(t *testing.T) {
	ctx := context.Background()
	s, r, f, _, c, _, hook := mapService(t)
	up := mapPNG(t)
	if err := s.PutPreviewMap(ctx, c.PublicID, c.EditToken, mapKey, bytes.NewReader(up)); err != nil {
		t.Fatal(err)
	}
	n, stored := storedMaps(f)
	if n != 1 || bytes.Equal(stored, up) || bytes.Contains(stored, []byte("tEXt")) || len(hook.calls) != 0 {
		t.Fatalf("draft: %d maps, hook %d", n, len(hook.calls))
	}
	if mf, err := s.PreviewFrame(ctx, c.PublicID, c.EditToken); err != nil || !mf.OK || !mf.Current || mf.Frame.Key != mapKey {
		t.Errorf("frame %+v %v", mf, err)
	}
	for id, rec := range r.rows {
		rec.Visibility = Unlisted
		r.rows[id] = rec
	}
	if err := s.PutPreviewMap(ctx, c.PublicID, c.EditToken, mapKey, bytes.NewReader(up)); err != nil {
		t.Fatal(err)
	}
	if n, _ := storedMaps(f); n != 1 || len(hook.calls) != 1 || hook.befores[0].Visibility != Unlisted || !sameRec(hook.befores[0], hook.calls[0]) {
		t.Errorf("published: %d maps, hook %v", n, hook.calls)
	}
}

func sameRec(a, b Record) bool {
	return a.ID == b.ID && a.HideEnds == b.HideEnds && a.Title == b.Title && a.Visibility == b.Visibility
}

// TestPreviewFrame: no positions answers no frame; a wrong token not found.
func TestPreviewFrame(t *testing.T) {
	ctx := context.Background()
	s, _, _, _, c, frames, _ := mapService(t)
	if mf, err := s.PreviewFrame(ctx, c.PublicID, c.EditToken); err != nil || !mf.OK || mf.Current {
		t.Errorf("%+v %v", mf, err)
	}
	if _, err := s.PreviewFrame(ctx, c.PublicID, strings.Repeat("A", 22)); !errors.Is(err, ErrNotFound) {
		t.Errorf("wrong token: %v", err)
	}
	frames.ok = false
	if mf, err := s.PreviewFrame(ctx, c.PublicID, c.EditToken); err != nil || mf.OK {
		t.Errorf("no positions: %+v %v", mf, err)
	}
}

// TestPutPreviewMapBusy (MC-4): one upload in flight; a second one is
// refused with ErrMapBusy and stores nothing.
func TestPutPreviewMapBusy(t *testing.T) {
	ctx := context.Background()
	s, _, f, _, c, frames, _ := mapService(t)
	frames.gate, frames.in = make(chan struct{}), make(chan struct{})
	done := make(chan error)
	go func() { done <- s.PutPreviewMap(ctx, c.PublicID, c.EditToken, mapKey, bytes.NewReader(mapPNG(t))) }()
	<-frames.in
	if err := s.PutPreviewMap(ctx, c.PublicID, c.EditToken, mapKey, bytes.NewReader(mapPNG(t))); !errors.Is(err, ErrMapBusy) {
		t.Errorf("second: %v", err)
	}
	frames.in = nil
	close(frames.gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n, _ := storedMaps(f); n != 1 {
		t.Errorf("%d maps", n)
	}
	// The slot is free again.
	if err := s.PutPreviewMap(ctx, c.PublicID, c.EditToken, mapKey, bytes.NewReader(mapPNG(t))); err != nil {
		t.Error(err)
	}
}

// TestPutPreviewMapDeleted (AC 44): after the workout is gone no map is
// stored and no folder appears.
func TestPutPreviewMapDeleted(t *testing.T) {
	ctx := context.Background()
	s, _, f, _, c, _, _ := mapService(t)
	if err := s.DeleteByOwner(ctx, c.PublicID, c.EditToken); err != nil {
		t.Fatal(err)
	}
	if err := s.PutPreviewMap(ctx, c.PublicID, c.EditToken, mapKey, bytes.NewReader(mapPNG(t))); !errors.Is(err, ErrNotFound) {
		t.Errorf("%v", err)
	}
	if len(f.dirs) != 0 {
		t.Error("a folder appeared")
	}
}
