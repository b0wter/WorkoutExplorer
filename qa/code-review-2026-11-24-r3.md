# QA code review 2026-11-24 (CR-01, CR-02) - round 3

- **Date:** 2026-11-24
- **Verdict:** ready. QA-CR-r2-1 to r2-4 are fixed and confirmed by probe, mutant or reading. Two Minor findings stay open: a leftover 500,000-record extrapolation in `MEASUREMENTS.md`, and a weak replacement test.
- **Inputs:** `qa/code-review-2026-11-24-r2.md`, ADR-061 items 2 and 4, ADR-017 (item 4 note, CR-01 note), F-12 O-7 and O-10, `src/server/testdata/corpus/MEASUREMENTS.md`. Diff 99480fd..6bf2791 (merge of 5072713), main at 6bf2791, working tree clean. Critical lane, effort high. Only the diff and the routed findings were reviewed.

## Build and test results

- `make lint test build size`: rc 0, 2 min 59 s. 23 Go packages ok (ingest 99.0 s, includes the extended `TestParseOverlappingLapsBound`). Vitest: 95 files and 2,502 tests passed. Size check passed (entry 61,364 / 62,000 bytes gzip).
- `go test -race -count=1 ./internal/lifecycle/`: ok in 5 of 5 runs (7.7 s each). In r2 this run failed in 2 of 2.
- `go test -race -count=20` on `TestOwnerWritesAndDeleteEndGone`, `TestDeleteRowErrorIsServerError`, `TestDeleteStorm` and `TestDeleteCallsLoseQuietly`: ok.
- `go test -race -count=1 ./internal/httpapi/ ./internal/workout/`: ok (72.5 s, 36.3 s).
- **Not run:** `caddy-check`, `docker-check`, `update-check`. They pull images from external registries, and the diff touches no deploy file.

## Findings

### Minor

- **ID:** QA-CR-r3-1
- **Severity / kind:** Minor / defect (documentation)
- **Route:** backend (`MEASUREMENTS.md` is the engineer's record)
- **Where:** `src/server/testdata/corpus/MEASUREMENTS.md:234` ("Extra sensor streams", Result paragraph)
- **Relates to:** ADR-017 item 4 note, r2 Question 2
- **Evidence:** the fix removed the 500,000-record extrapolation from the ADR-017 note and from the CR-01 paragraph. Both now say "no extrapolation to 500,000 records". Two lines above the CR-01 paragraph, the Result paragraph still says "so about 350 to 380 MB at 500,000 records. The stop rule ... hold[s] with about 5 % margin". The CR-01 paragraph also gives two ranges for the same build and file: the engineer's "332 to 342 MB in 6 runs and 367 MB in one", then "measured 338 to 370 MB" (QA r2's runs), with no note on who measured which.
- **Consequence:** the measurement record now contradicts itself and the ADR on the memory bound that justifies the caps. Change the Result sentence to the 440,000-record bound, and attribute the second range to QA r2.

- **ID:** QA-CR-r3-2
- **Severity / kind:** Minor / defect (test strength)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner_test.go:489-518` (`TestOwnerWritesAndDeleteEndGone`)
- **Relates to:** QA-CR-r2-3, ADR-061 Consequences
- **Evidence:** the fixture's workout stays a draft, and `Update` runs `changed` (the hook) only when `after.Visibility != Draft` (`owner.go:329`). The `&fakeHook{}` argument is therefore never called. The comment says "a delete may land anywhere between a write's steps", but the hook step that r2-3 was about is never exercised. The race run is green, so r2-3 is fixed. The published case is covered by `TestDeleteStorm` and `TestDeleteDuringWrite` (`adr061_test.go`).
- **Consequence:** the test looks like it covers delete-versus-hook and does not. The fix is to publish the row first (as the old test did), or pass `nil` and narrow the comment to drafts.

No Blocker or Major finding.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-CR-r2-1 | **fixed** | `owner.go:471-473` returns `ownerFail` when `err != nil && !deleted`. The r2 probe re-run on a copy of 6bf2791 (scratchpad `r3/mod/internal/lifecycle/probe_r3_delete_test.go`, a repo whose `DeleteWorkout` fails with `SQLITE_BUSY`): the answer is a `ServerError`, not `ErrNotFound`; the row is kept; 1 ERROR line. `httpapi` maps it to the generic 500 `server_error` text, so no internals leak. The new `TestDeleteRowErrorIsServerError` fails on the r2 `owner.go` ("answer lifecycle: not found"), so the mutant is killed. `TestDeleteCallsLoseQuietly` (7 of 8 losers get 404, no ERROR line) passes 20 of 20 times under `-race`. Checked: a lost race whose folder removal fails now answers 500 instead of 404. That matches O-10 ("any error is 500"). It cannot come from a missing folder, because `privatedir.Remove` and `publish.removeFolder` return nil when the folder is absent. |
| QA-CR-r2-2 | **fixed** | `lapSpans` now uses a difference array, O(n + laps). Differential probe (`r3/mod/internal/workout/probe_r3_spans_test.go`), old `lapSpans` from 99480fd against the new one: 200,000 random cases (0 to 5,000 samples, duplicate and negative offsets, 0 to 11 laps, laps with end < start, empty ranges, nested, adjacent, overlapping and full-range laps, out-of-range bounds) gave identical output (`reflect.DeepEqual`). CR-01 file (500,000 records, 10,000 full-length laps), 3 runs: `LapEnds` 1.7 to 2.3 ms (r2: 1.57 s); `AddLaps` with 100 boundaries 17 to 20 ms (r2: 3.14 s). The extension to `TestParseOverlappingLapsBound` fails on the old `addedlaps.go` ("LapEnds+AddLaps took 4.69s"), so the mutant is killed. The 2 s bound gives about 100 times headroom. The rest of `AddLaps` (100 boundaries × spans) was already linear in laps. |
| QA-CR-r2-3 | **fixed** (see QA-CR-r3-2 for the test's weakness) | The old test was replaced by `TestOwnerWritesAndDeleteEndGone`, which reads `r.rows` under `r.mu` and asserts the ADR-061 end state (row gone, Update answers only nil, `ErrNotFound` or `ErrBusy`, the delete succeeds). The lifecycle race run is green in 5 of 5 runs. |
| QA-CR-r2-4 | **fixed** | F-12 O-7 now says "Writes are `PATCH` and publish (the preview-map commit queues on this lock but takes no per-workout claim, ADR-061 item 4)". This matches ADR-061 item 4 and `previewmap.go` (global `mapSlot` at :89, `s.mu.Lock` at :111, no `claim`). |
| r2 Question 2 (memory wording) | **partly answered** | The ADR-017 item 4 note and the CR-01 paragraph now state the 440,000-record bound (338 to 370 MB, within 400 MB) instead of the "about 416 MB" extrapolation. The older Result paragraph still extrapolates (QA-CR-r3-1). |

## Questions

None.

## Coverage

- **Read:** the whole diff (7 files). Also `deleteRow`, `ownerFail`/`writeFail`, `changed`, `privatedir.Remove`, `publish.Area.Remove`/`removeFolder`, `AddLaps`/`LapEnds`, the httpapi `ServerError` mapping and the preview-map lock path.
- **Probes and mutants:** in a `git archive` copy of 6bf2791 under the session scratchpad `r3/`, with synthetic data and the r1 overlap file only. 2 of 2 mutants killed (old `lapSpans`, old `DeleteByOwner` order).
- **Preference, not a finding:** in `DeleteByOwner` the three branches reduce to two (`if err != nil` → `ownerFail`, then `if !deleted` → not found), with the same behaviour. `ServerError.Error()` repeats the "lifecycle: delete:" prefix. It appears only in Go error values, not in the log line or the response.
- **Not run:** `caddy-check`, `docker-check`, `update-check` (external pulls), fuzzing, and RSS re-measurement (the diff changes no allocation on the parse path; `lapSpans` keeps one `[]bool` and adds one `[]int` of n+1, both owner-path only). The real corpus was not read.
