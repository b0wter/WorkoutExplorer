# QA code review 2026-11-24 (CR-01, CR-02) - round 2

- **Date:** 2026-11-24
- **Verdict:** not ready - one Major regression from the CR-02 fix: when the row delete fails, an owner DELETE answers 404, no ERROR line is written, and the edit page drops the workout from My workouts, while the workout stays live and public (QA-CR-r2-1). Both CR-01 and CR-02 are otherwise fixed and confirmed by probe.
- **Inputs:** `qa/code-review-2026-11-24.md` (r1), `architect/ADR-061-owner-reads-and-deletes-off-the-write-lock.md`, ADR-017 (item 2, item 4 note, CR-01 note), F-12 O-7, F-21 X-3/X-4/X-6, F-47 TR-8/TR-10, `src/server/testdata/corpus/MEASUREMENTS.md`. Diff 7202ff5..06deaf4 (merges e5951c3 for CR-01 and 06deaf4 for CR-02), main at 06deaf4, working tree clean. Critical lane, effort high. Only the fixes were reviewed.

## Build and test results

- `make lint test build size`: rc 0, 2 min 56 s. 23 Go packages ok (ingest 98.8 s, includes `TestParseOverlappingLapsBound`), vitest 95 files / 2,502 tests passed, size check passed.
- `make caddy-check docker-check update-check` (the rest of `make ci`): **not run**, they pull images from external registries (`docker pull`), which this review must not do. The diff touches no deploy file.
- `go test -race -count=1 ./internal/httpapi/`: ok (66 s).
- `go test -race -count=1 ./internal/lifecycle/`: **FAIL**, data race in `TestOwnerWritesAreSerialised` (2 of 2 runs; QA-CR-r2-3). Without `-race`: 300 of 300 runs pass.
- `go test -race -count=30` on the nine new ADR-061 tests: ok (24.7 s).
- `go test -race -count=1 -timeout 30m ./cmd/workout-explorer/`: ok (491.8 s).

## Findings

### Major

- **ID:** QA-CR-r2-1
- **Severity / kind:** Major / defect (regression from ADR-061)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner.go:470-476` (`DeleteByOwner`)
- **Relates to:** F-12 O-10 (AC 5 only for a lost race), UP-7/SH-4 (the owner's delete), ADR-061 item 2, ADR-025 item 8 (server faults are logged)
- **Evidence:** the new code checks `deleted` before `err`:
  ```go
  deleted, err := s.deleteRow(context.WithoutCancel(ctx), rec.ID, rec.PublicID)
  if !deleted {
      return ErrNotFound
  }
  if err != nil {
  ```
  `deleteRow` returns `(false, err)` when `Repo.DeleteWorkout` fails (`lifecycle.go:193-196`), so a database failure is reported as "not found". Probe in the scratch copy (`internal/lifecycle/probe_r2_delete_test.go`, a repo whose `DeleteWorkout` returns `SQLITE_BUSY`): answer `lifecycle: not found`, row still there, 0 ERROR lines. Before 06deaf4 the same failure went through `ownerFail` (500 and an ERROR line). On a 404 the edit page calls `mineList.remove()` and shows the generic page (`src/web/src/edit/start.svelte.ts:163-165`); on any other failure it keeps the entry and says trying again is safe (line 166).
- **Consequence:** the owner asks to delete, sees "This workout has expired or is not available", and the workout loses its entry (with the token) in My workouts. The row, the original and, if published, the public page stay until the deadline. The owner cannot retry without the saved link, and the operator gets no ERROR line. The trigger is any row-delete failure (busy timeout, I/O error, disk full). Fix: return the server error when `err != nil && !deleted`; answer not found only for `(false, nil)`. Add a test with a failing `DeleteWorkout`. None of the ADR-061 tests covers this path. QA checked the suggested order in the scratch copy: the probe passes, and `TestDeleteCallsLoseQuietly` and `TestDeleteStorm` still pass.

### Minor

- **ID:** QA-CR-r2-2
- **Severity / kind:** Minor / defect (CR-01 partly fixed)
- **Route:** backend
- **Where:** `src/server/internal/workout/addedlaps.go:25-52` (`lapSpans`, marks every covered sample per lap), called by `LapEnds` and twice by `AddLaps`; callers `publish/laps.go:60` (each build and view with added boundaries) and `:88` (`CheckEdits` on each PATCH with boundaries)
- **Relates to:** CR-01 ("same pattern, owner-triggered: `lapSpans`"), ADR-017 item 2 (no loop over laps × samples)
- **Evidence:** scratch probe `internal/ingest/probe_r2_spans_test.go` on the r1 file (500,000 records, 10,000 full-length laps): `LapEnds` 1.57 s, `AddLaps` with 100 boundaries 3.14 s. By comparison, after the fix `RemoveAutoLaps` takes 12 ms, `Trim` 60 ms and `Parse` 0.64 s.
- **Consequence:** an owner of such a file who adds lap boundaries makes each PATCH, publish build, trimmed view and regenerate spend about 1.6 to 4.7 s more, partly under O-7. That is bounded and within T, so not Major. Since ADR-061, other owners' saves queue behind it, but deletes and expiry do not. The fix is the same as for totals: one difference-array pass instead of the per-lap fill.

- **ID:** QA-CR-r2-3
- **Severity / kind:** Minor / defect (test)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner_test.go:490-532` (`TestOwnerWritesAreSerialised`)
- **Relates to:** ADR-061 Consequences ("the in-process delete-during-write paths ... need tests"), Verification
- **Evidence:** `go test -race ./internal/lifecycle/` fails: the hook check reads `len(r.rows)` without `r.mu` while `DeleteByOwner` (no longer under O-7) deletes from the map. The test's claim "hook never sees a row that is gone" is also no longer an invariant: under ADR-061 a hook may run for a deleted workout, and `changed` cleans up afterwards. The test only passes without `-race` because the claim turns most PATCHes into `ErrBusy` (300/300 passes).
- **Consequence:** the lifecycle race run is red, so a real race in this package would be hidden behind a known failure. The test asserts a superseded rule. Rewrite it to the ADR-061 rule (end state: row and both folders gone) or delete it, since `TestDeleteStorm` covers this.

- **ID:** QA-CR-r2-4
- **Severity / kind:** Minor / defect (documentation)
- **Route:** architect
- **Where:** `architect/F-12-owner-api.md` O-7
- **Relates to:** ADR-061 item 4
- **Evidence:** O-7 reads "Writes are `PATCH`, publish and the preview-map commit, with at most one in flight per workout (a second gets `503 busy`)". ADR-061 item 4 and the code (`PutPreviewMap` takes no claim; it has the global `mapSlot` and queues on O-7) apply the claim to `PATCH` and publish only.
- **Consequence:** a reader expects a preview-map `PUT` that runs during a `PATCH` of the same workout to be refused, but it queues. The behaviour is fine; the sentence should say "PATCH and publish".

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| CR-01 | **fixed** for the parse/Accept path; owner-triggered `lapSpans` part still open (QA-CR-r2-2, Minor) | The r1 probe file (15,150,239 bytes, 500,000 records, 10,000 overlapping laps) re-run on 06deaf4: `Parse` 0.64 s (r1: 24.5 s), `Accept` 2.31 s (r1: 43.1 s), under ADR-017's 2 s / 4.5 s. `parse -original` CLI: 2.9 to 3.1 s vs 42.6 to 43.7 s at 7202ff5. Through `newApp`, a trimmed owner GET of that file takes 0.94 s. Results: data-file goldens are `bytes.Equal` and pass. Differential probe, old `rangeValues` vs new `rangeIndex.values` (copied from 7202ff5), 400 random sample sets (2 to 500,000 samples; NaN rate 0 to 100 %; zeros; nil streams; all-NaN distance; NaN first distance; ranges of two samples, at both ends, full length; 568,400 values): presence (nil vs value) identical in every case. With integer streams (as decoded) every value is bit-identical. With fractional streams (gap-filled thirds) 9 % differ by at most 1.3e-12 relative. `rangeMean` and `carriedDistance` (trim one-offs) agree. Inf cannot reach the sums: decoded HR, cadence and altitude are integer fields, and Stryd power is clamped to 0 to 65,534 (`validWatts`). A single-sample lap still returns early (`to-from < 2`). `TestParseOverlappingLapsBound` fails on the old code (24 s > 6 s). |
| CR-02 | **fixed** (with QA-CR-r2-1 introduced by the fix) | The r1 probe re-run through `newApp` (scratch `cmd/workout-explorer/probe_r2_lock_test.go`; trimmed, published workout; k concurrent owner GETs and trim PATCHes; a published workout made due; another workout's owner DELETE). ride-12h: expiry pass 0.8 / 2.1 / 0.8 ms at k = 0 / 8 / 32 (r1: 1 ms / 1.27 s / 4.43 s); the due public folder is gone after the pass in every case; other DELETE 204 in under 1 ms. CR-01 file (0.94 s per trimmed GET): expiry 2.7 / 2.7 / 3.0 ms, DELETE under 1 ms. At each k exactly one PATCH got 200 and the rest got `503 busy`; all GETs got 200; 0 ERROR lines. |

ADR-061 invariants checked:

- **Delete wins / no resurrected public files:** scratch `probe_r2_leak_test.go` races publish of the CR-01 file (build 1.4 to 2.5 s) against the owner's DELETE sent 0.3 to 2.7 s in, 5 rounds: the public folder is gone, the storage total matches the disk (0 bytes drift), and there are 0 ERROR lines. The ADR's "storage total may stay too high" consequence did not show up here, because the publish hook's own post-build re-read releases the bytes. Publish answers 200 when the delete lands after its commit, which ADR-061 Verification allows.
- **No lost update:** the claim plus O-7 around `BEGIN IMMEDIATE`. `TestSecondWriteIsBusy` passes, and so does the wiring probe above (one 200, the rest 503).
- **Memory bound:** one owner-read slot (buffered channel of 1), taken before `TrimmedView`/`Frame` read the original. `TestOwnerReadsDoNotOverlap` checks a maximum of 1 concurrent parse.
- **Claim release:** `defer s.claims.Delete` right after a successful claim, on both paths. `TestClaimReleasedOnErrors` covers four error paths.
- **503 busy:** `httpapi.fail` maps `ErrBusy` to 503 `busy`, and `error-codes.json` lists the new sentence. The edit page shows its general busy text for the code ("The server is busy. Please try again in a minute.", all languages), as ADR-061 item 4 says. The page runs one action at a time (`ui.run`), so normal use cannot hit the claim from a single tab.
- **SH-10:** X-6 is restated without T. The probe shows expiry independent of k.


## Questions

1. For the architect: ADR-061 Verification asks for the CR-02 probe "through `newApp`, with a fake parse that blocks at the worst-case bound". The engineer's `TestCR02Probe` is at service level with a fake Trimmer (stated deviation). QA ran the real-wiring probe (above) with real parses, not a blocking one. Is the service-level test plus this QA run enough, or should a `newApp` test with a blocking seam be added? Suggestion: enough. The wiring has no lock of its own, and the service test covers the blocking bound.
2. For the architect: `MEASUREMENTS.md` (CR-01 note) extrapolates the 367 MB outlier to "about 416 MB" at 500,000 records, which is above the 400 MB stop rule, and nobody decided anything about it. QA measured the F-68 twin (440,000 records, the largest that fits `MaxBytes` with those fields) 5 times each, alternating: before 335 to 343 MB, after 338 to 370 MB (2 of 5 above 355 MB). On the CR-01 file: before 266 to 296 MB, after 262 to 324 MB. All runs are under 400 MB at the real cap. Suggestion: replace the 500,000 extrapolation with a statement that 440,000 records is the cap for this composition, and record the about 30 MB of headroom.

## Coverage

- **CR-01:** `rangeIndex`/`cumul`/`carriedSeries` read and differentially tested against the old code. Call sites checked: `completeLaps`, `AddLaps`, `RemoveAutoLaps`, `Trim` (sessions and laps). RSS re-measured with `/usr/bin/time -v` on binaries built from 7202ff5 and 06deaf4. Not re-checked: ADR-017's all-caps benchmarks (unchanged inputs; `make test` ran them).
- **CR-02:** diff of `lifecycle` (owner, lifecycle, previewmap, pins, regenerate), `httpapi` (fail, busy), docs (F-12, F-21, F-47, ADR-017 note), the new tests and the front-end busy/delete handling. Pre-existing and not new with ADR-061: a preview-frame GET that races its own workout's delete still goes through `ownerFail` (ERROR line) without a row re-check. That path is a read, outside ADR-061 item 5, and only the token holder can trigger it.
- **Mutants** (scratch copy, the nine ADR-061 tests, `-timeout 40s`), 10 of 10 killed:

  | Mutant | Killed by |
  | --- | --- |
  | View under O-7 | `TestOwnerReadsDoNotOverlap` |
  | Expiry delete takes O-7 | `TestCR02Probe`, `TestDeletesDoNotTakeO7` |
  | Owner delete takes O-7 | `TestCR02Probe`, `TestDeletesDoNotTakeO7` |
  | Sweep takes O-7 | `TestCR02Probe`, `TestDeletesDoNotTakeO7` |
  | Claim before the token check (by public ID) | `TestSecondWriteIsBusy` (hangs, test timeout) |
  | Publish claim released only on success | `TestClaimReleasedOnErrors` |
  | Owner-read slot taken after the read | `TestOwnerReadsDoNotOverlap` |
  | No recheck after a failed hook | `TestDeleteDuringWrite` |
  | `writeFail` without the row re-check | `TestDeleteDuringWrite`, `TestDeleteDuringPreviewMapCommit` |
  | Frame GET waits for the slot instead of 503 | `TestOwnerReadsDoNotOverlap` (hangs, test timeout) |

  Two mutants are only caught by the test timeout; that is acceptable in CI (10 min default), but slow.
- **Not run:** `caddy-check`, `docker-check`, `update-check` (external pulls), fuzzing, real corpus (never read). The probes live under the session scratchpad (`r2/mod`), with synthetic data and the r1 overlap file only.
