package ingest

import (
	"context"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"workout-explorer/internal/workout"
)

// maxProductName and maxFrames bound the free-text parts of the log line.
const (
	maxProductName = 64
	maxFrames      = 32
)

// outcome is what one Parse call did, for the log line.
type outcome struct {
	bytes   int
	reparse bool // the strip's reparse: every limit but MaxBytes applies (D-22, ADR-017 rev. 6)
	// The listener's findings, copied as values after the walk so the outcome
	// keeps nothing of the first parse alive (ADR-017 rev. 3 item 2). Zero if
	// the parse never reached the walk (no slot, too large, not FIT).
	fileType       string
	device         workout.Device
	garminProduct  string
	recordMessages int
	integ          workout.Integrity
	dataEnd        int32 // seconds to the last intact record (accepted and truncated)
	err            *Error
	panicVal       string
	frames         []string
	duration       time.Duration
	spans          []seqSpan // the first parse's sequences, for the strip
}

// take copies the listener's findings into o as values (ADR-017 rev. 3
// item 2), so o never points into the parse.
func (o *outcome) take(l *listener) {
	o.fileType, o.device = l.rec.FileType, l.rec.Device
	o.garminProduct, o.recordMessages = l.garminProduct, l.recordMessages
}

// log writes the one line of ADR-018 for a rejection, a cancel or a
// recovery, and nothing for a clean success. It writes only the allow-listed
// keys, in this order. Never logged: positions in any form, times, totals,
// stream values, serial number, file name.
func (p *Parser) log(o *outcome) {
	if p.logger == nil {
		return
	}
	level, msg, kind := slog.LevelInfo, "fit recovered", ""
	outcomeName := "recovered"
	switch {
	case o.err != nil && o.err.Kind == KindCanceled:
		msg, outcomeName = "fit canceled", "canceled"
	case o.err != nil && o.err.Kind == KindInternal:
		level, msg, outcomeName = slog.LevelError, "fit rejected", "rejected"
	case o.err != nil:
		level, msg, outcomeName = slog.LevelWarn, "fit rejected", "rejected"
	case o.integ.Truncated || o.integ.CRCFailed || o.integ.IgnoredBytes > 0 || o.integ.SkippedSequences > 0:
	default:
		return // clean success
	}

	attrs := []slog.Attr{slog.String("outcome", outcomeName)}
	if o.err != nil {
		kind = string(o.err.Kind)
		attrs = append(attrs, slog.String("kind", kind))
		if o.err.Limit != "" {
			attrs = append(attrs, slog.String("limit", o.err.Limit))
		}
	}
	dev, fileType, product, records := o.device, o.fileType, o.garminProduct, o.recordMessages
	attrs = append(attrs,
		slog.String("file_type", fileType),
		slog.String("manufacturer", dev.Manufacturer),
		slog.String("product", product),
		slog.Int("product_id", int(dev.ProductID)),
		slog.String("product_name", cutRunes(dev.ProductName, maxProductName)),
		slog.Int("bytes", o.bytes),
		slog.Int("sequences", o.integ.Sequences),
		slog.Int("skipped_sequences", o.integ.SkippedSequences),
		slog.Int("ignored_bytes", o.integ.IgnoredBytes),
		slog.Bool("truncated", o.integ.Truncated),
		slog.Bool("crc_failed", o.integ.CRCFailed),
	)
	if o.err == nil && o.integ.Truncated {
		attrs = append(attrs, slog.Int("data_end_s", int(o.dataEnd)))
	}
	attrs = append(attrs,
		slog.Int("records", records),
		slog.Int64("duration_ms", o.duration.Milliseconds()),
	)
	if o.err != nil {
		attrs = append(attrs, slog.String("cause", o.err.Error()))
	}
	if o.err != nil && o.err.Kind == KindInternal {
		attrs = append(attrs, slog.String("panic", o.panicVal), slog.Any("frames", o.frames))
	}
	p.logger.LogAttrs(context.Background(), level, msg, attrs...)
}

// cutRunes shortens s to at most n bytes at a rune boundary.
func cutRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// stackFrames returns up to maxFrames frames as "function file:line", with
// the file reduced to its base name. No argument words: Go's default
// traceback prints raw argument words, which can be the bits of a coordinate
// (ADR-018 item 4).
func stackFrames(skip int) []string {
	pcs := make([]uintptr, maxFrames+8)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var out []string
	for len(out) < maxFrames {
		f, more := frames.Next()
		file := f.File
		if i := strings.LastIndexByte(file, '/'); i >= 0 {
			file = file[i+1:]
		}
		out = append(out, f.Function+" "+file+":"+strconv.Itoa(f.Line))
		if !more {
			break
		}
	}
	return out
}
