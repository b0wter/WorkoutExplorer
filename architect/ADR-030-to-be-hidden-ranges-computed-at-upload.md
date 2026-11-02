# ADR-030 To-be-hidden sections on the edit page: computed at upload by the publish rule, stored as sample ranges, returned by the owner API

- **Status:** accepted (29 Oct 2026, with the F-19 design)
- **Date:** 29 Oct 2026
- **Features:** F-19 (introduces it); F-09 (upload computes the ranges), F-12 (settings object), F-16 (the rule), F-30 (regeneration recomputes them)
- **Supersedes:** none. Extends the F-12 settings object (read-only field) and the schema of [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) (migration to v2)
- **Requirements:** SH-4, UP-4, review assumption 15 (the owner sees the full track with the to-be-hidden start and end marked); invariant I-3; F-16 H-2 to H-4, H-8

## Context

F-19 marks on the edit page's map the start and end sections that hiding removes (feature scope, SH-4). F-16's hand-off fixes that the Go rule (`workout.HideEnds`) is the single definition, and no TypeScript copy decides what is hidden. The exact rule needs the normalised positions before quantisation and which samples gap filling inserted (H-3). `draft.bin` carries neither, so a rule run on `draft.bin`, in Go or in the browser, can place a boundary up to three samples away from the public cut. Running the exact rule on request means a full parse of `original.fit` (a parse slot, up to the parse timeout, ADR-017) on every edit-page load. Decoding `draft.bin` on request costs up to tens of MB per request for the longest workouts, on endpoints without a rate limit (ADR-025 item 6), outside ADR-017's memory model.

## Decision

1. **Computed where the exact input exists.** The upload, which already holds the normalised workout, computes the to-be-hidden sample ranges with the same function that publish uses to cut. Refactor `HideEnds` so that the boundaries and the cut come from one loop, so the two cannot drift. F-30's `regenerate` recomputes them whenever it rebuilds a workout.
2. **Stored on the row.** The ranges are stored as nullable integer columns in a forward-only migration to schema v2 (ADR-019). The engineer chooses the column names and `CHECK`s. A row without values (one stored before the migration) reads as "no ranges".
3. **Returned read-only in the settings object** (`GET` and `PATCH`, F-12), as `hide_ends_ranges`: an array of `[first, last]` pairs of inclusive sample indices into `draft.bin`, whose positions hiding removes.
   - `[]` when the workout has no position data, or the row has no values.
   - `[[0, N−1]]` when the route would be fully hidden (F-16 H-5).
   - Otherwise one or two pairs, start section first.

   The field is the same whatever `hide_ends` says: it shows what hiding *would* remove. A `PATCH` that sends it gets `400`, like every other read-only field.
4. **Display only.** The public build never reads these columns. It still cuts from `original.fit` on every build (F-16 H-4, ADR-028 item 2). The ranges are not logged and never leave the owner API.

## Consequences

- The marks match the public cut exactly, sample for sample, and the edit page has no hiding logic.
- Cost: one schema migration and a small change to the upload path, both backend. F-19 becomes owner `both`, with the backend step first.
- Cost: a later change to H-2 or H-3 has stale ranges for existing workouts until `regenerate` runs. Every deploy runs it (TD Versioning), so this window is the deploy itself.
- This stores slightly more than before: up to four integers per workout, with no coordinates. They are derived from data already in `draft.bin`, and deleted with the row (AC-4).

## Would choose differently if

- A later privacy feature (SH-3 privacy zones) needs per-zone marks. Then return the cut as a list of ranges from a stored or cached computation over the same function, and revisit the storage form. The single-definition rule stays.
- The owner can edit hiding parameters (for example the radius). The ranges would then depend on settings, and computing them on `PATCH` from `original.fit` under the O-7 lock is the better home.
