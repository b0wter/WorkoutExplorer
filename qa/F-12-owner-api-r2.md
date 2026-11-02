# QA F-12 Owner API - round 2

- **Date:** 2026-10-23
- **Verdict:** ready. QA-12-r1-1 (the Major) is fixed. The abort probe from round 1 now gives 0 ERROR lines in 2,500 dropped requests. Run against the round-1 binary as a control, the same probe still gives 146 ERROR lines in 300 requests. A cancelled context cannot cut an `Update` or a `Delete` short once the token check has passed, and a real database error is still logged and still answers `500`. r1-2 to r1-6 are fixed. Two new Minor findings remain: a test gap (no test makes the lookup fail with a real database error) and one duplicated phrase in ADR-025.
- **Inputs:** `qa/F-12-owner-api-r1.md` (QA-12-r1-1 to r1-6); `git diff 0ce6819 HEAD` (commit `fae48cd`, clean working tree): `lifecycle/owner.go`, `lifecycle/owner_test.go`, `httpapi/owner.go`, `httpapi/owner_test.go`, `cmd/workout-explorer/owner_test.go`, `architect/F-12-owner-api.md` (F-15 hand-off), `architect/ADR-025-…md` item 4. ADR-025 items 4, 6 and 8; F-12 design O-7, O-11 and O-13. Q1 (tab in the description) is waiting for the user and is not reviewed here.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. `go vet` clean. `go test ./...`: all 12 packages `ok`. svelte-check passes with `--fail-on-warnings`. Size: activity entry 45,328 / 60,000 gzip PASS, upload entry 2,272 / 8,000 gzip PASS (no front-end change). The only warning is npm's `EBADENGINE` (Node 22 locally), which is not from F-12 |
| `go test -race -count=1 ./internal/lifecycle/ ./internal/httpapi/ ./internal/store/ ./internal/privatedir/ ./cmd/workout-explorer/` | all `ok` (lifecycle 2.9 s, httpapi 14.7 s, store 1.2 s, privatedir 1.0 s, cmd 5.8 s) |
| `go test -race -count=20 -run 'Cancelled\|AnonymousDelete\|PatchBody\|OwnerNotFound'` on lifecycle, httpapi, cmd | all `ok` |
| Abort probe `probe_abort.py` (round 1, port changed only), scratch server built from HEAD, scratch data folder, synthetic `ride-30min.fit` | `GET`, `PATCH`, `DELETE` × 500 each with an unknown ID, plus `GET`, `DELETE` × 500 each with the uploaded workout's public ID, no token: **0 ERROR lines**. The log has only the 4 INFO lines (recovery, storage, started, upload accepted). Control: the round-1 binary (`2ade9db`) with the same probe, `GET` × 300: **146 ERROR lines** |
| QA probes in a scratch copy of the module (`qa_r2_test.go`, not committed), real SQLite and `privatedir`, `-race` | results below, all as expected |

QA probes in the scratch copy:

- **Cancel after the check, `Update`.** The repository wrapper cancels the request context right after the lookup under the lock returns. `Update` succeeds and the row has the new title. `UpdateWorkout` sees an uncancelled context. No log line.
- **The same on a published workout.** The hook is called, and its context is not cancelled.
- **Cancel after the check, `DeleteByOwner`.** The cancel comes after the second lookup, the one under the lock. Delete returns `nil`, the row is gone, `private/` is empty, and `DeleteWorkout` sees an uncancelled context.
- **Cancel between the pre-check and the check under the lock.** The result is `ErrNotFound`, the row stays, and nothing is logged: either all of the delete happens or none of it.
- **A real database error is not hidden.** The database is closed and then `Get` is called. With a live context the result is a `ServerError` and a `stage: lookup` ERROR line. With a cancelled context the result is the same, because `database/sql` returns `sql: database is closed` before it checks the context and `gone` does not match that error.
- **Cancel during the SQLite query** (the modernc `interruptOnDone` path). 8 goroutines × 2,500 `Get` calls each, with the cancel at 0 to 400 µs. Without `-race`, three runs gave `ok` 3,891 / 4,832 / 1,974 and the rest `ErrNotFound`, so the cancels landed on both sides of the query. There were 0 other errors and 0 ERROR lines. I read modernc v1.60.1 `stmt.go:292-304`: an interrupted query returns `ctx.Err()`, not `SQLITE_INTERRUPT`, so `gone` catches it.
- **HTTP level.** Through `newApp` with a cancelled request context, `GET`, `PATCH` and `DELETE`, each with and without the token: `404` with the shared not-found body, the log does not grow, and the row is unchanged.

Mutants (repo tests only, scratch copy):

| Mutant | Result |
| --- | --- |
| Remove the `DELETE` pre-check (r1-2) | killed by `TestAnonymousDeleteDoesNotQueue` |
| `gone` returns `true` for every error (every lookup failure becomes an unlogged not found) | **survives** (QA-12-r2-1) |
| `gone` without the `ctx.Err() != nil` condition | survives. This is harmless today, because the store returns a context error only when the context ended. Covered by the same fix |

## Findings

### Minor

- **ID:** QA-12-r2-1
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner.go:162-165` (`gone` in `lookup`) and `:186-188`; `src/server/internal/lifecycle/owner_test.go:546` (`TestOwnerCancelledRequestIsSilent`, the only test that touches this branch)
- **Relates to:** ADR-025 item 8, O-13 (a server failure is logged and answers `500`); the r1-1 fix
- **Evidence:** No repo test makes `WorkoutByPublicID` fail with a non-context error. A grep finds no test that checks `stage: lookup` or a lookup `ServerError`. The mutant "`gone` always true" turns every database failure in the lookup into a silent `404`, and it passes `go test` on lifecycle, httpapi and cmd. My scratch probe (closed database, live and cancelled context) kills it.
- **Consequence:** the code is correct today. But the property the r1-1 fix had to keep ("do not hide real database errors") has no test, so a later change that widens `gone` would turn an outage into silent "not found" answers with no ERROR line. Fix: a fake repository error with a live context gives a `*ServerError` and exactly one ERROR line. Add the case of a non-context error with a cancelled context.

- **ID:** QA-12-r2-2
- **Severity / kind:** Minor / defect (documentation)
- **Route:** architect (small lane)
- **Where:** `architect/ADR-025-owner-api-access-and-not-found-response.md:23` (item 4)
- **Relates to:** QA-12-r1-4
- **Evidence:** the reworded sentence reads "…and against a fixed dummy hash when there is no row (with a fixed dummy hash when there is no row)". The parenthesis repeats the clause before it, and the sentence has two colons in a row ("the lookup always runs, the comparison always runs: against…").
- **Consequence:** cosmetic. The meaning is right and now matches the code. Delete the parenthesis.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-12-r1-1 (Major) anonymous dropped request gives an ERROR line and `500` | **fixed** | `lookup` maps a context error on an ended context to `ErrNotFound`, unlogged (`owner.go:162-164`). The `WorkoutExists` re-check in `open` does the same (`:256-258`). Live probe: 0 / 2,500 against HEAD, 146 / 300 against the r1 binary. No `500`: every `ServerError` goes through `ownerFail`, which always logs, so 0 ERROR lines means no `500` from these paths. The HTTP-level probe also saw `404` directly. Writes after the check use `WithoutCancel` (`:213`, `:284`), confirmed by the cancel-after-check probes. Real database errors are still logged (probe). The test gap is in r2-1 |
| QA-12-r1-2 `DELETE` takes the O-7 lock before the token check | **fixed** | pre-check at `owner.go:275-277`, then the check is repeated under the lock. `TestAnonymousDeleteDoesNotQueue` fails if the pre-check is removed (mutant). O-7 still holds as written ("check the token under the lock"), so the design needs no change |
| QA-12-r1-3 hook and delete errors would carry `public_id` after F-15 | **fixed** (hand-off) | F-15 row of the F-12 design hand-off (`architect/F-12-owner-api.md:73`): errors from the hook and from `Delete` must not carry a `public_id` or a `public/{public_id}` path. F-15 QA must check it |
| QA-12-r1-4 ADR-025 item 4 wording vs the precomputed placeholder hash | **fixed**, with a wording slip | the text now matches the code. The duplicated parenthesis is r2-2 |
| QA-12-r1-5 a duplicate key lets `null` through | **fixed** | `hasDuplicateKey` (`httpapi/owner.go:229, 265`) refuses any repeated top-level key with `400`. Test cases `{"title":null,"title":"x"}` and `{"title":"a","title":"b"}` are in `TestPatchBody`. Probed: `{"title":"a","title":null}` → `400`, `{"title":"a","Title":"b"}` → `400` (unknown key), and a key-like string inside a value is not mistaken for a key |
| QA-12-r1-6 the AC 1 end-to-end test checks only one header | **fixed** | `cmd/workout-explorer/owner_test.go:80-87` compares the whole header map (minus `Date`) of every refusal case on every endpoint against the first one, so a `WWW-Authenticate` or `Cache-Control` difference on any one path now fails |

## Questions

None new. Q1 from round 1 (tab in the description) is waiting for the user.

## Coverage

Only the diff and the routed findings were reviewed. AC 1 to 7 were traced in round 1. The diff touches AC 1 (header test, lookup on a cancelled context) and AC 5 (delete pre-check). Both were re-run: `TestOwnerNotFound` and `TestOwnerDelete` pass under `-race`, and the live probe and the scratch probes above cover the new branches. Not run: `make corpus` (no parsing change); anything behind Caddy (F-22, F-31). Only the synthetic `ride-30min.fit` was used.
