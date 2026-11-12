# ADR-047 Trim: a cut of the normalised workout between two stored instants; the owner's trimmed values computed per read

- **Status:** accepted (12 Nov 2026, with the F-47 design)
- **Date:** 12 Nov 2026
- **Features:** F-47 (introduces it); F-12 (settings object), F-15 (public transform), F-16 (hiding after trim), F-19 (edit page), F-30 (regenerate), F-45 (owner suggestion)
- **Supersedes:** none. Narrows [ADR-030](ADR-030-to-be-hidden-ranges-computed-at-upload.md) for trimmed workouts (its "Would choose differently if" applies). Replaces the TD's reserved columns `trim_start_s`, `trim_end_s`
- **Requirements:** UP-6, SH-4, SH-10, UP-5, AP-2; invariant I-3; NFR Privacy

## Context

The trim must keep the trimmed part (often the way home) out of every public file. The original must stay unchanged, and the trim must survive `regenerate` on every deploy, including a parser change. The edit page shows the full recording from `draft.bin` but must show the summary, hidden marks and title suggestion of the *trimmed* workout. Those values depend on the sport timeline, the legs rule (R-10) and the 300 m circles measured on positions before quantisation. None of these is in `draft.bin`, and a TypeScript copy would be a second definition.

## Decision

1. **Cut the normalised workout, not the FIT records** (a trim keeps the pool length and drops only the number of lengths and the calories, OQ-179). `workout.Trim` slices and rebases the output of `Normalise` and re-runs the summary rules on the cut (F-47 TR-1, TR-5). Hiding then runs on the result (TR-2). Sample positions of the trim are therefore positions in `draft.bin`.
2. **Store instants.** `trim_start_at`, `trim_end_at` are Unix seconds of the resolved samples, nullable, settings columns. Each build resolves them inward: first sample at or after the start, last at or before the end (TR-4). Not sample indices: a parser change that alters gap filling or resampling would move an index to another moment. Not offsets: a change to R-1's start time would shift every offset, and an offset-stored start trim would then reveal that shift of the trimmed part. An instant stays at the same moment of the recording's clock, and inward rounding cannot reach outside it.
3. **Owner view computed per read, only when a trim is stored.** `GET` and `PATCH` parse `original.fit` under the O-7 lock and run the public transform's code to return the trimmed `sport` and `summary`, the hidden ranges in `draft.bin` indices, and the suggested title (TR-8). Untrimmed workouts keep ADR-030's stored ranges and ST-6's per-request read, with no parse. Nothing derived from the trim is stored.

## Consequences

- One definition of everything the trim changes, in Go. The owner sees exactly what the public build makes, and the browser holds only a display mirror of the window rule.
- Cost: a parse per owner read of a trimmed workout, serialised with owner writes on the O-7 lock. ADR-017's M(N) is unchanged. Latency is up to one parse per edit-page load or save of a trimmed workout.
- Cost: ADR-030's stored ranges describe the untrimmed workout and are unused while a trim is set. They are still kept, for untrimmed workouts and a removed trim.
- Every trim change is remove-first (P-7 default). The page shows the generic page for about a second, also when widening.
- The trim instants are timestamps of the owner's workout, stored in the row and deleted with it. They hold no coordinates.

## Would choose differently if

- Owner reads of trimmed workouts become frequent or slow on the VPS (a parse above about 1 s at the 95th percentile, or owners queueing on O-7). Then compute the owner view on a trim `PATCH` and at `regenerate`, and store the summary and ranges (not the town) on the row, as ADR-030 does for hiding.
- Privacy zones (SH-3) or a mid-workout cut arrive. The transform then removes several windows, and the owner view should return ranges for each from the same per-read computation.
