package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/proto"

	"workout-explorer/internal/workout"
)

// header is the part of a FIT file header the walk needs.
type header struct {
	Size     int    // 12 or 14
	DataSize uint32 // bytes of messages after the header, excluding the 2-byte file CRC
}

// looksLikeFIT is ingest's own header check (ADR-018, kind not_fit): at least
// 12 bytes, a header size of 12 or 14 that fits in the data, and ".FIT" at
// bytes 8 to 11. The data size and the header CRC are not part of it: a
// header with data size 0 is a FIT file (R-c), a wrong header CRC a recovery
// (R-b).
func looksLikeFIT(b []byte) bool {
	return len(b) >= 12 && (b[0] == 12 || b[0] == 14) && len(b) >= int(b[0]) && string(b[8:12]) == ".FIT"
}

func parseHeader(b []byte) header {
	return header{Size: int(b[0]), DataSize: binary.LittleEndian.Uint32(b[4:8])}
}

// walkResult is what the sequence walk found besides the listener's data.
type walkResult struct {
	integ   workout.Integrity
	lastErr error     // the decoder error that ended the walk, for the log
	spans   []seqSpan // one per sequence the walk handled, for the strip (ADR-023 item 2)
}

// seqStatus is what the walk made of a sequence (ADR-023 item 2).
type seqStatus int

const (
	seqComplete  seqStatus = iota // decoded to the end of its data region; a CRC failure or a missing CRC does not matter
	seqTruncated                  // ended the walk: unclosed header, decode error or missing data
)

// seqSpan is where a sequence lies in the upload: the strip rewrites exactly
// these sequences and drops every byte after the last one.
type seqSpan struct {
	pos        int // offset of the header
	headerSize int // 12 or 14
	dataEnd    int // end of the records (excluding the file CRC)
	records    int // records the decoder accepted in the sequence's last decoder call (D-20)
	status     seqStatus
	skipped    bool // another file type, still framed and stripped
}

// walk decodes the FIT sequences of data into l (F-08 design, Sequence walk
// and recovery rules). Each sequence is decoded by one decoder call on a
// fresh decoder, because the decoder's errors are sticky. The caller has
// checked looksLikeFIT(data). The walk ends early when dctx ends; the caller
// finds out why with context.Cause.
func walk(dctx context.Context, data []byte, l *listener, lim Limits) walkResult {
	var res walkResult
	pos := 0
	for {
		if dctx.Err() != nil {
			return res
		}
		if res.integ.Sequences == lim.MaxSequences {
			l.stop(&limitError{limitSequences})
			return res
		}
		h := parseHeader(data[pos:])
		res.integ.Sequences++
		l.beginSequence(res.integ.Sequences)
		avail := len(data) - pos - h.Size
		patched := h.DataSize == 0 // R-c: the device never closed the file

		records, err := decodeSequence(dctx, data, pos, h, patched, !patched, l)
		crcFailed := false
		if errors.Is(err, decoder.ErrCRCChecksumMismatch) {
			crcFailed = true
			if l.seqMessages == 0 && !patched {
				// Header CRC (reported before any message): decode again without checks.
				records, err = decodeSequence(dctx, data, pos, h, false, false, l)
			} else {
				err = nil // file CRC: reported after the last message, the data is complete
			}
		}
		if dctx.Err() != nil {
			return res
		}
		skipped := l.skip
		if skipped {
			res.integ.SkippedSequences++
		}

		next := false
		switch {
		case patched:
			res.integ.Truncated = res.integ.Truncated || !skipped
		case err == nil:
			next = true
		case isEOF(err) && avail >= int(h.DataSize):
			crcFailed = true // only the CRC bytes are missing (R-b)
		default:
			res.integ.Truncated = res.integ.Truncated || !skipped
		}
		span := seqSpan{pos: pos, headerSize: h.Size, records: records, skipped: skipped}
		switch {
		case patched:
			span.status, span.dataEnd = seqTruncated, len(data)
		case err == nil || (isEOF(err) && avail >= int(h.DataSize)):
			span.status, span.dataEnd = seqComplete, pos+h.Size+int(h.DataSize)
		default:
			span.status, span.dataEnd = seqTruncated, min(len(data), pos+h.Size+int(h.DataSize))
		}
		res.spans = append(res.spans, span)
		if err != nil {
			res.lastErr = err
		}
		if crcFailed && !skipped {
			res.integ.CRCFailed = true
		}
		if !next {
			return res
		}
		pos += h.Size + int(h.DataSize) + 2
		if pos >= len(data) {
			return res
		}
		if !looksLikeFIT(data[pos:]) {
			res.integ.IgnoredBytes = len(data) - pos // R-e
			return res
		}
	}
}

// recordCounter counts the records the decoder accepted in one decoder call
// (D-20, ADR-003 rev. 5): definition messages through OnMesgDef, data messages
// through OnMesg. Both hooks run only after a whole record was read and
// accepted. It does nothing else.
type recordCounter struct{ n int }

func (c *recordCounter) OnMesg(proto.Message)              { c.n++ }
func (c *recordCounter) OnMesgDef(proto.MessageDefinition) { c.n++ }

// decodeSequence decodes the sequence that starts at data[pos] into l. A
// patched sequence (header data size 0) is decoded with the data size set to
// the bytes present and the header CRC cleared; the library treats a header
// CRC of 0 as "not computed". Decoder options (ADR-003 rev. 5): the listener
// and a fresh record counter on both hooks, broadcast only, no component
// expansion, and no checksum for a retry or a patched header; nothing else.
// It returns the counter's count for this call.
func decodeSequence(dctx context.Context, data []byte, pos int, h header, patched, checksum bool, l *listener) (int, error) {
	var r io.Reader
	if patched {
		hdr := append([]byte(nil), data[pos:pos+h.Size]...)
		binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(data)-pos-h.Size))
		if h.Size == 14 {
			hdr[12], hdr[13] = 0, 0
		}
		r = io.MultiReader(bytes.NewReader(hdr), bytes.NewReader(data[pos+h.Size:]))
	} else {
		r = bytes.NewReader(data[pos:])
	}
	c := &recordCounter{}
	opts := []decoder.Option{
		decoder.WithMesgListener(l, c),
		decoder.WithMesgDefListener(c),
		decoder.WithBroadcastOnly(),
		decoder.WithNoComponentExpansion(),
	}
	if !checksum {
		opts = append(opts, decoder.WithIgnoreChecksum())
	}
	_, err := decoder.New(r, opts...).DecodeWithContext(dctx)
	return c.n, err
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
