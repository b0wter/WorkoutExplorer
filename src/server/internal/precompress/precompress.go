// Package precompress writes the pre-compressed transport encoding of a
// file (ADR-006). It knows nothing about workouts and is the only package
// that imports github.com/andybalholm/brotli.
package precompress

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/andybalholm/brotli"
)

// Brotli parameters (ADR-006 item 3): the file is compressed once and
// downloaded many times; a 4 MiB window covers a 12-hour data file.
const (
	quality = 9
	lgWin   = 22
)

// mu allows one compression at a time per process: a 12-hour data file
// allocates about 52 MB, so concurrent publishes, uploads and regeneration
// could add up on a small VPS (F-03 design v1.2, Pre-compression).
var mu sync.Mutex

// Brotli returns src brotli-compressed. The result decompresses to exactly
// src. Empty input gives a valid, empty brotli stream.
func Brotli(src []byte) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()
	var buf bytes.Buffer
	w := brotli.NewWriterOptions(&buf, brotli.WriterOptions{Quality: quality, LGWin: lgWin})
	if _, err := w.Write(src); err != nil {
		return nil, fmt.Errorf("precompress: brotli: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("precompress: brotli: %w", err)
	}
	return buf.Bytes(), nil
}
