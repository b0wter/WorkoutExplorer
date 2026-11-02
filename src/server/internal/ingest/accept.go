package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"workout-explorer/internal/workout"
)

// Accepted is the result of Accept (ADR-023 item 1).
type Accepted struct {
	Workout  workout.Workout // parsed from Original (the reparse)
	Original []byte          // the upload with UP-9 data removed; what is stored as original.fit
	Stripped StripCounts
}

// Accept parses upload, strips it (UP-9), reparses the result and verifies
// the workout is unchanged (ADR-023). Every failure is an *Error: the
// upload's own rejection (as Parse would give it), Kind unreadable for a
// deliberate refusal of a file Parse accepts (ADR-023 rev. 9 item 8a: U1 an
// empty complete sequence, U2 a removed timestamp that a compressed-timestamp
// record depends on; ADR-018 rev. 6), or Kind internal with a "strip:" cause
// (item 8), which always means a defect.
// The whole flow holds one parse slot and writes at most one log line.
func (p *Parser) Accept(ctx context.Context, upload []byte) (Accepted, error) {
	_, acc, err := p.run(ctx, upload, true)
	if err != nil {
		return Accepted{}, err
	}
	return acc, nil
}

// strip is steps 2 to 6 of ADR-023 item 1, after the upload's own parse. ref
// is the digest of that parse (reference); the parse itself is already
// released (D-17).
func (p *Parser) strip(ctx context.Context, upload []byte, ref []byte, o *outcome) (Accepted, *Error) {
	fail := func(format string, a ...any) (Accepted, *Error) {
		return Accepted{}, p.errorOf(o, KindInternal, "", fmt.Errorf("strip: "+format, a...))
	}
	out, counts, err := strip(upload, o.spans, ctx.Err)
	if err != nil {
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return Accepted{}, p.errorOf(o, KindCanceled, "", ctx.Err())
		}
		if errors.Is(err, errEmptySequence) { // U1, D-26/D-27: a deliberate refusal
			if ctx.Err() != nil { // a canceled context comes first (design, "Order inside Accept")
				return Accepted{}, p.errorOf(o, KindCanceled, "", ctx.Err())
			}
			return Accepted{}, p.errorOf(o, KindUnreadable, "", fmt.Errorf("strip: %w", err))
		}
		return fail("%w", err)
	}
	if p.stripHook != nil {
		p.stripHook(out)
	}

	re := &outcome{bytes: len(out), reparse: true}
	w2, rerr := p.parse(ctx, out, re)
	if rerr != nil {
		if rerr.Kind == KindCanceled {
			return Accepted{}, p.errorOf(o, KindCanceled, "", rerr.cause)
		}
		if rerr.Kind == KindLimit && rerr.Limit == limitTime {
			// A time limit depends on load; the second decode can hit it when
			// the first did not. It is the upload's limit, not an internal
			// error (D-22, ADR-018 rev. 4).
			return Accepted{}, p.errorOf(o, KindLimit, limitTime, rerr.cause)
		}
		return fail("reparse rejected (%s)", rerr.Kind)
	}
	got, err := reference(w2)
	if err != nil {
		return fail("reference: %v", err)
	}
	if !bytes.Equal(ref, got) {
		if removedTimestampDependency(upload, o.spans) { // U2, D-28: classified only after a mismatch
			return Accepted{}, p.errorOf(o, KindUnreadable, "", fmt.Errorf("strip: %w", errRemovedTimestamp))
		}
		return fail("the stripped file is a different workout")
	}
	return Accepted{Workout: w2, Original: out, Stripped: counts}, nil
}

// reference is the digest the strip must preserve: sha256 of the golden
// report, with the two fields the rewrite repairs set to zero. The golden
// report covers every field of the workout and a digest of every stream
// (ADR-004 item 2).
func reference(w workout.Workout) ([]byte, error) {
	w.Integrity.CRCFailed, w.Integrity.IgnoredBytes = false, 0
	b, err := workout.ReportJSON(w)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	return sum[:], nil
}
