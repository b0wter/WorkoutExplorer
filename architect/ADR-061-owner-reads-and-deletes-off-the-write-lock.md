# ADR-061 Owner reads and deletes off the O-7 lock; one owner write per workout

- **Status:** accepted (24 Nov 2026). Critical lane (SH-10, UP-7), QA at effort high
- **Date:** 24 Nov 2026
- **Features:** fixes CR-02 of [`qa/code-review-2026-11-24.md`](../qa/code-review-2026-11-24.md); F-12 (O-7), F-21 (X-3, X-4, X-6), F-36 (frame slot), F-47 (TR-8, TR-10)
- **Supersedes:** [F-12](F-12-owner-api.md) O-7's list of locked operations (deletes and expiry leave it); [F-21](F-21-lifetime-and-expiry.md) X-4 "each under the O-7 lock" and X-6's bound; [ADR-047](ADR-047-trim-cut-of-the-normalised-workout.md) item 3 and [F-47](F-47-trim.md) TR-8, TR-10 "under the O-7 lock". All edited in place, old text in `history/`
- **Depends on:** [ADR-017](ADR-017-parse-isolation-and-limits.md) item 4 (memory), [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md), [ADR-031](ADR-031-cross-process-lock-on-the-public-area.md), [ADR-025](ADR-025-owner-api-access-and-not-found-response.md)
- **Requirements:** SH-10, UP-7, AC-4, SH-4; invariants I-3, I-4

## Context

1. O-7 is one mutex per process. Besides owner writes it was taken by the trimmed owner view (a full parse, TR-10), every owner and operator delete, each expiry delete and the expiry sweep.
2. Owner endpoints have no rate limit (ADR-025). One token holder can queue any number of trimmed `GET`s or `PATCH`es. Go's mutex hands over FIFO under contention, so expiry and every other owner's delete wait behind the whole queue (CR-02 probe: k = 32 GETs, 4.4 s; with a slow file, past SH-10's 60 s).
3. Even one locked build can exceed 60 s at the worst-case bound (a trim `PATCH` runs up to three parses, each up to the decode limit T ≤ 60 s). So nothing that SH-10 depends on may wait for O-7, whatever the queue length.
4. Deletes in other processes (F-27 CLI, F-30 `regenerate`) never held `serve`'s O-7. They are already safe through row-first deletes, the `Live` re-check inside every update (F-21 X-3), the public lock (ADR-031), publish's post-build re-read (F-15 P-6), the recheck in `changed`, and the sweep every 30 s.

## Decision

1. **O-7 covers only writes that change a row and run the hook:** `PATCH`, publish, and the commit part of the preview-map `PUT`. Nothing else in `serve` takes it.
2. **Deletes never take O-7.** Owner delete, operator delete (F-37 link), expiry deletes and the expiry sweep rely on context item 4, the same guarantees that already cover deletes from other processes. An owner delete whose call did not remove the row answers not found (AC 5 also when two run at once).
3. **The owner view runs outside O-7, in one owner-read slot shared with the F-36 frame `GET`.** The slot is taken before `original.fit` is read. The view waits for it while its request lasts and then falls back to TR-8's failure view. The frame `GET` does not wait and answers `503` as before. So at most one owner read parses at a time, as before this ADR (the view used to hold O-7, and the frame its own slot), and ADR-017's M(N) does not change.
4. **One owner write per workout at a time.** `PATCH` and publish claim the workout by internal ID after the token check and before O-7. A second write to the same workout gets `503` with code `busy` and does not queue. The claim is released on every return path. A stranger cannot claim, because the token check runs first. The edit page already runs one action at a time and shows the general `busy` text for this code.
5. **A write that loses the race to a delete answers not found.** If a private file is missing after the token check, the workout row is read again, as in F-12 O-8. If the row is gone, the answer is not found and no ERROR line is written.

## Invariants that must keep holding

| Invariant | Why it still holds |
| --- | --- |
| No lost update | Every row change is still one `BEGIN IMMEDIATE` read-modify-write under O-7 (O-3), and hooks still run in row order. The view and deletes write no settings |
| Delete wins | A delete never waits for owner work. A racing write either finds no row, or `Live` is false inside its transaction (X-3). A public folder it wrote after the delete is removed by P-6 or `changed`, at the latest by the next sweep. `privatedir.Replace` never creates a missing folder |
| Memory bound | One O-7 build plus one owner read (view or frame) plus the preview `PUT`, all parses in ADR-017's two slots. This is the same as before |
| SH-10 | From the deadline or delete until the files are gone: at most 30 s (pass period) plus one public-lock write phase in any process (about one compression, ADR-031). This does not depend on T or on how many owner requests are in flight. F-21 X-6 is restated with this bound |
| Due is final | The `Live` re-check inside every update transaction keeps it, without the lock (X-3) |

## Consequences

- Residual, accepted: a holder of K workouts can keep K writes queued on O-7 and delay other owners' saves and publishes. Deletes, expiry and privacy are not affected. K is bounded by the upload rate limit and the storage cap.
- Residual, accepted: repeated trimmed `GET`s starve other owners' trimmed views and frame `GET`s. Those owners see the failure view or keep the plain card. This is the same class as QA-36-r2-2.
- A build that races its own workout's delete may leave the storage total too high until the next start, because `changed` removes without releasing. A total that is too high only refuses uploads earlier, never later.
- The in-process delete-during-write paths, which only another process could trigger before, now occur in `serve` and need tests.

## Rejected

- **A lock per workout instead of O-7:** memory then needs a global build gate, which recreates the same queue, and the change is larger.
- **Storing the owner view (ADR-047's own fallback):** needs a schema change and changes to `regenerate` and the `PATCH` paths, and does not fix write queues or deletes.
- **A rate limit per token or per IP on owner endpoints:** it adds a table that clients can grow (counted in M(N)), and a few worst-case builds would still hold O-7 past 60 s.
- **Expiry first in line for O-7:** Go's mutex has no priority, and context item 3 applies anyway.

## Verification (QA high)

- CR-02 probe through `newApp`, with a fake parse that blocks at the worst-case bound. With k = 0, 8 and 64 concurrent trimmed `GET`s and `PATCH`es on one workout, an expiry pass and another workout's owner `DELETE` each finish within a bound that does not grow with k.
- A second concurrent `PATCH` or publish of the same workout gets `503 busy`. The same requests on another workout proceed after O-7.
- Two owner-read parses (view and view, or view and frame) never overlap.
- A delete at each stage of a `PATCH` or publish (before the transaction, after the commit, during the build, after the write phase) leaves the row and both folders gone within one pass. A transaction after the delete answers `404`, and no ERROR line is written.
- Mutants, each caught by a test: the view under O-7; a delete or expiry step taking O-7; the claim taken before the token check; the claim not released on an error path; the owner-read slot taken after the original is read.

## Would choose differently if

- Friends report `busy` or timeouts on save in normal use: add a per-IP limit on owner writes.
- More than one host writes the data directory (ADR-031's own condition).
