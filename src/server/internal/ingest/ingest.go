// Package ingest is the anti-corruption layer around the FIT decoding
// library github.com/muktihari/fit (ADR-003): it decodes a FIT file into a
// library-free workout.Recording and hands it to workout.Normalise. It is
// the only production package that imports the library; no library type
// crosses its boundary.
//
// F-08 makes it safe for any uploaded file: Parser.Parse rejects what is not
// a recorded activity with an *Error (ADR-018), recovers truncated and
// CRC-damaged files (F-08 design, Sequence walk), and runs under the limits
// of ADR-017. It writes the one parser log line (ADR-018).
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"workout-explorer/internal/workout"
)

// Parser parses FIT files under the limits of ADR-017. Create one per process
// with NewParser; Parse is safe for concurrent use.
type Parser struct {
	lim    Limits
	logger *slog.Logger
	slots  chan struct{}
	hook   func() // test hook, called from OnMesg; nil in production

	stripHook func(stripped []byte) // test hook, called with the stripped bytes before the reparse; nil in production
}

// NewParser returns a Parser. Zero fields of lim take their defaults. A nil
// logger writes no log lines (the CLI).
func NewParser(lim Limits, logger *slog.Logger) *Parser {
	lim = lim.withDefaults()
	return &Parser{lim: lim, logger: logger, slots: make(chan struct{}, lim.MaxConcurrent)}
}

// WithLogger returns a Parser that shares p's limits and parse slots and
// writes its log lines to logger (F-30: the line gains the workout's id).
func (p *Parser) WithLogger(logger *slog.Logger) *Parser {
	q := *p
	q.logger = logger
	return &q
}

// Limits returns the effective limits, defaults filled in.
func (p *Parser) Limits() Limits { return p.lim }

// Parse decodes data, a whole FIT file, and returns the normalised workout.
// Every failure is an *Error. It waits for a parse slot (at most
// MaxConcurrent parses run at once); ctx ends the wait and the parse.
func (p *Parser) Parse(ctx context.Context, data []byte) (workout.Workout, error) {
	w, _, err := p.run(ctx, data, false)
	if err != nil {
		return workout.Workout{}, err
	}
	return w, nil
}

// run is Parse and Accept: with accept set it goes on, in the same parse
// slot and with the same log line, to strip the upload, reparse the result
// and verify it (accept.go).
func (p *Parser) run(ctx context.Context, data []byte, accept bool) (w workout.Workout, acc Accepted, err error) {
	o := &outcome{bytes: len(data)}
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		o.err = &Error{Kind: KindCanceled, cause: ctx.Err()}
		p.log(o)
		return workout.Workout{}, Accepted{}, o.err
	}
	t0 := time.Now() // slot wait excluded
	defer func() { <-p.slots }()
	defer func() {
		if r := recover(); r != nil {
			// Frames 0 to 2 are runtime.Callers, stackFrames and this function.
			o.panicVal, o.frames = fmt.Sprint(r), stackFrames(3)
			o.err = p.errorOf(o, KindInternal, "", errors.New("panic"))
			w, acc, err = workout.Workout{}, Accepted{}, o.err
		}
		o.duration = time.Since(t0)
		if o.err == nil && !accept {
			o.integ, o.dataEnd = w.Integrity, w.Integrity.DataEnd
		}
		p.log(o)
	}()

	w, o.err = p.parse(ctx, data, o)
	if o.err != nil {
		return workout.Workout{}, Accepted{}, o.err
	}
	if accept {
		o.dataEnd = w.Integrity.DataEnd
		ref, err := reference(w)
		if err != nil {
			o.err = p.errorOf(o, KindInternal, "", fmt.Errorf("strip: reference: %v", err))
			return workout.Workout{}, Accepted{}, o.err
		}
		w = workout.Workout{} // the first parse is unreachable from here on (D-17)
		acc, o.err = p.strip(ctx, data, ref, o)
		if o.err != nil {
			return workout.Workout{}, Accepted{}, o.err
		}
		return workout.Workout{}, acc, nil
	}
	return w, Accepted{}, nil
}

// errorOf builds an *Error that names the file type and device if the
// listener saw a file_id.
func (p *Parser) errorOf(o *outcome, kind Kind, limit string, cause error) *Error {
	e := &Error{Kind: kind, Limit: limit, cause: cause}
	e.FileType, e.Device = o.fileType, o.device
	return e
}

// parse is the pipeline of the F-08 design (section Pipeline), steps 2 to 8.
func (p *Parser) parse(ctx context.Context, data []byte, o *outcome) (workout.Workout, *Error) {
	if !o.reparse && len(data) > p.lim.MaxBytes {
		return workout.Workout{}, p.errorOf(o, KindLimit, limitBytes, fmt.Errorf("%d bytes", len(data)))
	}
	if !looksLikeFIT(data) {
		return workout.Workout{}, p.errorOf(o, KindNotFIT, "", errors.New("no FIT header"))
	}

	dctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	dctx, cancelTimeout := context.WithTimeoutCause(dctx, p.lim.Timeout, errTimeLimit)
	defer cancelTimeout()
	l := newListener(p.lim, cancel)
	l.hook = p.hook
	defer o.take(l) // also on a panic inside the walk; the error paths below call it first

	res := walk(dctx, data, l, p.lim)
	o.take(l)
	o.integ = res.integ
	o.spans = res.spans

	if ctx.Err() != nil {
		return workout.Workout{}, p.errorOf(o, KindCanceled, "", ctx.Err())
	}
	if dctx.Err() != nil {
		cause := context.Cause(dctx)
		var le *limitError
		switch {
		case errors.Is(cause, errTimeLimit):
			return workout.Workout{}, p.errorOf(o, KindLimit, limitTime, cause)
		case errors.As(cause, &le):
			return workout.Workout{}, p.errorOf(o, KindLimit, le.name, cause)
		case errors.Is(cause, errUnsupportedType):
			return workout.Workout{}, p.errorOf(o, KindUnsupportedType, "", fmt.Errorf("file type %s", l.rec.FileType))
		}
		return workout.Workout{}, p.errorOf(o, KindInternal, "", fmt.Errorf("decode stopped: %w", cause))
	}

	rec := l.rec
	noData := func(why string) *Error {
		var cause error = errors.New(why)
		if res.lastErr != nil {
			cause = fmt.Errorf("%s; decoder: %w", why, res.lastErr)
		}
		return p.errorOf(o, KindNoData, "", cause)
	}
	if len(rec.Records) == 0 {
		return workout.Workout{}, noData("no record with a valid timestamp")
	}
	n := workout.SampleCount(rec)
	if n == 0 {
		return workout.Workout{}, noData("no samples")
	}
	if n > p.lim.MaxSamples {
		return workout.Workout{}, p.errorOf(o, KindLimit, limitSamples, fmt.Errorf("%d samples", n))
	}
	rec.Integrity = res.integ
	return workout.Normalise(rec), nil
}
