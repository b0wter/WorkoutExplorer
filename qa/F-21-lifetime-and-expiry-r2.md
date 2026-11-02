# QA F-21 Lifetime and automatic expiry - round 2

- **Date:** 2026-10-31
- **Verdict:** ready. All five r1 findings are fixed. QA-21-r1-1 is closed: after an expiry or an owner delete, the database files no longer hold the row, both while the server runs and after it stops, in the probe and on the real binary. Two Minor findings remain: a row deleted by `Recover` skips the checkpoint, so its bytes stay in `workouts.db` until the next delete or shutdown (QA-21-r2-1); and one test gap (QA-21-r2-2).
- **Inputs:** diff 7350a8e..71b40ff (5ab9c94 docs, 71b40ff code); `architect/F-21-lifetime-and-expiry.md` v0.2 (X-3, X-4, X-5, X-9, Verification, F-27 hand-off); ADR-019 rev. 3 items 3 and 9; ADR-031 (accepted); `qa/F-21-lifetime-and-expiry-r1.md`; spec AC-4, NFR Privacy. Working tree clean.

## Build and test results

- `make ci`: exit 0. Go packages ok, vitest 51 files / 1,331 tests passed, `index.html` and `edit.html` 56,366 / 60,000 B gzip, upload 2,272 / 8,000 B. All PASS.
- `go test -race -count=1 ./...` (src/server): all 14 packages ok.
- `go vet ./...`, `gofmt -l .`: clean. `make corpus`: 55 ok, 0 diff, 0 fail; 6 local real files.
- r1 residual probe (`qaprobe_residual_test.go`, in a `git archive HEAD` copy): **PASS**. Title, description and `public_id` are absent from `workouts.db` and `-wal` while running, after close and after restart. It failed in r1.
- Real binary, real 30 s timer (`f21-e2e-r2.sh`, the r1 script plus a PATCH of title and description and a byte scan of db/-wal/-shm): the scan finds all four markers in `-wal` before the deadline, so the scan works. Both workouts were gone at deadline + 24 s, none early, and both owner GETs answer 404. After the pass `-wal` is 0 B, and the scan finds nothing while running or after shutdown. `strings workouts.db` shows only the schema, with no summary values. The log has two `by=expiry` lines and no `public_id` or token.
- New mutants (`mutants_r2.py`, lifecycle, store, publish and cmd tests): 11 of 12 killed. Killed: `secure_delete` off (R1); no checkpoint in `Expire` (R2) or after an owner delete (R3); no `Live` re-check in Update (R4) or Publish (R5); the sweep skipped on a list failure (R6); a failed checkpoint not retried (R7); PASSIVE (R8) or RESTART (R12) instead of TRUNCATE; `busy` ignored (R9); the flag never set (R10). **Survived:** R11 (QA-21-r2-2).
- r1 mutants rerun on HEAD: M1 to M20 killed, **M19 and M20 now included** (`TestExpireDeleteHoldsLock`, `TestRemoveFirstHoldsLock`). **M21 (startup pass after the total) now survives.** On 7350a8e it is still killed, but only because the WAL growth from the delete made the measured total differ from the disk (65,536 vs 82,048). Now the checkpoint truncates the WAL, and `Release` subtracts exactly what the pass frees, so the order no longer changes the total. I judge it equivalent in effect. The order of the log lines is still checked, but the mutant keeps that order. This is not a finding.

## Findings

### Minor

- **ID:** QA-21-r2-1
- **Severity / kind:** Minor / defect (privacy)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/lifecycle.go:242` (`Recover` calls `Repo.DeleteWorkout` directly, so `uncheckpointed` is never set)
- **Relates to:** AC-4, ADR-019 item 9 ("a deleted row leaves no bytes in the database files"), X-4 step 4
- **Evidence:** Probe `TestQAProbeRecoverRowResidue`. Upload, PATCH title `QARECOVERTITLE`, close, remove `private/{id}`, reopen. `Recover` deletes the row. While the server runs, `workouts.db` still contains the title and the `public_id`, even after an extra idle `Expire`. The startup pass deleted nothing, so it does not checkpoint.
- **Consequence:** A row that `Recover` removes keeps its title, description, summary and `public_id` in the main file until the next expiry or owner delete triggers a checkpoint, or until a clean shutdown. This happens only when a private folder was lost outside the process, so it is rare and bounded. It is still a gap in the rule the user just decided. The likely fix is one line: set the flag when `removedRows > 0`, and the startup pass then checkpoints.

- **ID:** QA-21-r2-2
- **Severity / kind:** Minor / risk (test gap)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner.go:215` and `:272` (the new `errors.Is(err, ErrNotFound)` cases); `expiry_test.go` `TestDeadlineBetweenLookupAndCommit`
- **Relates to:** X-3, QA-21-r1-2, ADR-025 (no ERROR line for not found)
- **Evidence:** Mutant R11 removes the `ErrNotFound` case in Update and survives. The error then goes through `ownerFail`: an ERROR line `owner request failed` with `id`, and a `*ServerError` that wraps `ErrNotFound`. The test's `errors.Is` still matches, and `httpapi.fail` still answers 404.
- **Consequence:** A refactor can turn the late-deadline refusal into an ERROR log line on every such request without any test noticing. Assert that the error is not a `*ServerError`, or that the log has no ERROR line, for both publish and patch.

## Engineer's deviations

| Deviation | Judgement |
| --- | --- |
| A failed checkpoint sets the retry flag again, and the next pass checkpoints even if nothing was deleted | Accept. It is written down in ADR-019 item 9 and X-4 step 4, and pinned by `TestExpireCheckpoint` (R7 killed) |
| No runtime check that `secure_delete` is on | Accept. The pragma is a constant in the DSN, applied to every connection, and the driver version is pinned. Unlike `journal_mode`, it has no silent fallback on some file systems. `TestOpen` reads it back, and R1 is killed by `TestOpen` and `TestCheckpoint` |

I also checked:
- An owner delete whose public or private removal fails returns before the checkpoint. The flag is already set, so the next pass (at most 30 s later) checkpoints. Accepted under the X-4 retry.
- In the owner delete, the checkpoint runs under O-7 with a busy timeout of 5 s. An external reader can delay that one delete, no more.
- `UpdateWorkout` rolls back when the apply function returns `ErrNotFound`, so the row is unchanged in the real store too.

## Previous findings

- **QA-21-r1-1** (row bytes left in db/WAL; AC 8 passes for the wrong reason): **fixed.** `secure_delete(ON)` plus `wal_checkpoint(TRUNCATE)` after a deleting pass and after an owner delete. `dataLeft` now scans the bytes of db, `-wal` and `-shm`. `TestOwnerDeleteLeavesNoBytes` covers the owner delete while running and after close. The r1 probe passes, and so does the real-binary scan. The gap in `Recover` is new: QA-21-r2-1.
- **QA-21-r1-2** (publish or PATCH succeeds across the deadline): **fixed.** `Live(now)` is re-checked inside the update. `TestDeadlineBetweenLookupAndCommit` covers it, and R4 and R5 are killed. Test gap: QA-21-r2-2.
- **QA-21-r1-3** (M19 and M20 survived): **fixed.** Both are killed now.
- **QA-21-r1-4** (a failed due query skips the sweep): **fixed.** `TestExpire` checks that the sweep ran (`pub.keep`), and R6 is killed.
- **QA-21-r1-5** (docs): **fixed.** F-21 v0.2 is accepted, with X-5 recording the startup-failure deviation. ADR-031 is accepted, ADR-019 rev. 3 records item 9 and its accepted limit, and the INDEX rows and history are updated.

## Questions

None.

## Coverage

Only the r1 diff and the routed findings were reviewed. The AC table of r1 still applies. AC 8 now traces to a byte scan that can fail: it caught the markers before the delete and failed under R2, R3, R8, R10 and R12. `TestExpiryEndToEnd` and `TestExpiryTimer` now check "before the deadline" with `len(left) < 3` instead of `!= 3`. M3 and M4 (early deletion) are still killed, so the weaker check is enough. Not run: Caddy in front of the expiry, and a concurrent second writer on the real binary (both as in r1). Probes, scripts and logs:
- `<scratchpad>/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/qa-f21-probes-r2/qaprobe_r2_test.go`
- `<scratchpad>/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/qa-f21-probes-r2/f21-e2e-r2.sh`
- `<scratchpad>/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/qa-f21-probes-r2/mutants_r2.py`
- `<scratchpad>/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/qa-f21-probes-r2/mutants_r2.log`
- `<scratchpad>/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/qa-f21-probes-r2/mutants_r1_rerun.log`
