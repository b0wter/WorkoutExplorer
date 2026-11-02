# QA F-21 Lifetime and automatic expiry - round 1

- **Date:** 2026-10-31
- **Verdict:** not ready. One Major: a deleted or expired workout's row (title, description, start time, heart-rate and other summary values, `id`, `public_id`) can still be read in `workouts.db` after the delete, and the AC 8 check passes anyway because it counts rows and does not look at the bytes on disk (QA-21-r1-1, AC-4 "nothing remains"). Everything else in the expiry design works: the boundaries are right, nothing is deleted early, the public lock settles QA-15-r2-1, and the real binary removed an expired published workout and an overdue draft 24 s after their deadlines.
- **Inputs:** `features/F-21-lifetime-and-expiry.md`; `architect/F-21-lifetime-and-expiry.md` v0.1 (X-1 to X-10, Verification, Hand-offs); ADR-031, ADR-028 item 5, ADR-025; spec UP-7, SH-2, SH-10, AC-1, AC-4, NFR Privacy; TD Publish 7 and Files on disk; decision log 30 and 31 Oct 2026 (48 h draft rule, an overdue draft answers not found, OQ-87 accepted). Commits 460ddf9, e6ba535, 193a99c, 2c7d838 (HEAD). Working tree clean.

## Build and test results

- `make -k ci`: exit 0. Go packages ok (cached), vitest 51 files / 1,331 tests passed, entry `index.html` and `edit.html` 56,366 / 60,000 B gzip, upload 2,272 / 8,000 B. All PASS.
- `make corpus`: 55 ok, 0 diff, 0 fail, 0 new, 0 stale; M/G invariants ok; 6 local real files checked.
- `go test -race -count=1 ./...` (src/server): all 14 packages ok.
- `go test -race -count=10 -run 'Expir|Deadline|Draft|PublicLock|QAR2|DataLeft|PublishedAt47'` on lifecycle, store, publish, cmd: ok, no flakes.
- `go vet ./...`, `gofmt -l .`: clean.
- Mutants (a scratch copy made with `git archive HEAD src`, running the lifecycle, store, publish and cmd tests): 21 of 23 killed. Killed: draft rule dropped from the lookup (M1), from the due query (M2) and from `Deadline` (M6); `<=` changed to `<` at the draft boundary (M3) and at the `expires_at` boundary (M4); `Live` made inclusive (M5); 49 h (M7); the pass without the sweep (M8); one failed row stops the pass (M9); the sweep (M10), the write phase (M11) and `Remove` (M12) without the public lock; `flock` replaced by a no-op (M17); no startup pass (M13); the startup pass after the total (M21); `LivePublicIDs` changed to `>=` (M14); the sweep's freed bytes not released (M15); the log line `by=owner` (M16); a timer that never fires (M18). **Survived:** M19, the pass's `Delete` without O-7, and M20, the remove-first step without the public lock (QA-21-r1-3).
- The engineer's two surviving mutants are equivalent, and I agree. The re-read in `changed` with `ExpiresAt` is the same as `Live` because `cur` is unlisted. "Calendar days" cannot differ: the store returns times in UTC (`store.go:238`), and in UTC `Add(48h)` equals `AddDate(0,0,2)`.
- Real binary, real 30 s timer (`f21-e2e.sh`): I uploaded two workouts, published one, and used `sqlite3` to move the published workout's `expires_at` and the draft's `created_at + 48 h` to now + 5 s. Neither was removed before its deadline. Both were gone at deadline + 24 s: no row, no `private/{id}`, no `public/{public_id}`. After that, every owner `GET` answered 404. The log has two `workout deleted … by=expiry` lines and no `public_id` or token.

## Findings

### Major

- **ID:** QA-21-r1-1
- **Severity / kind:** Major / defect (privacy)
- **Route:** architect, then backend
- **Where:** `src/server/internal/store/store.go:38` (no `secure_delete`, no checkpoint after deletes); `src/server/cmd/workout-explorer/expiry_test.go:87` (`dataLeft` treats `workouts.db` and `-wal` as "the tables above")
- **Relates to:** AC-4 ("removes … all stored data at once … nothing remains afterwards"), NFR Privacy ("no data outlives its workout"), F-21 AC 8, X-9
- **Evidence:** Probe `qaprobe_residual_test.go`: upload, PATCH title `QAPROBETITLE` and description `QAPROBEDESC`, run an expiry pass. The row count is 0. Even so, `workouts.db-wal` still contains the title, the description and the `public_id` while the server runs. After `close` and after a restart, `workouts.db` still contains all three. On the real binary after the run above (clean shutdown, WAL checkpointed), `strings workouts.db` shows both deleted rows' `id` and `public_id` and their `summary_json` (distance, avg/max heart rate, power, calories, …). With `secure_delete(ON)` added in the scratch copy, the main file is clean after close. The WAL still holds the old frames while the server runs, so a full fix also needs a checkpoint (for example `wal_checkpoint(TRUNCATE)` after a pass that deleted, or `journal_size_limit`).
- **Consequence:** Expiry and owner delete leave the workout's settings and health summary readable on disk for as long as SQLite does not reuse the page. At this volume that can be weeks. AC 8's "complete check" reports nothing left, so the test passes for the wrong reason. This already applied to F-12 deletes; F-21 is the feature whose acceptance criterion claims it. The fix is small, but how strong it must be is a decision: is a free page on disk "data"? The OS-level remains of removed files are a separate question and are not covered here. If the user explicitly accepts the residue, this becomes a recorded decision and the finding closes.

### Minor

- **ID:** QA-21-r1-2
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/lifecycle.go:283-291` (due list taken outside O-7, no re-check before `Delete`); `owner.go:205`, `owner.go:257` (the apply funcs in `UpdateWorkout` do not re-check `Live`)
- **Relates to:** X-3 ("due is final"), AC 2, UP-7
- **Evidence:** Probe `qaprobe_publish_deadline_test.go`. The lookup runs at deadline - 1 ms. Then the clock passes the deadline, a pass lists the draft as due, and the publish commits. `Publish` returns success, and the pass then deletes the workout. The same applies to a PATCH that lengthens the lifetime.
- **Consequence:** The window is the few milliseconds between the lookup and the commit, at the deadline. The end state (deleted) matches the rule, but the owner was told "published" (or "saved"), and then the page disappears within a pass. X-3's argument ("no request can revive a due row") does not cover a request that starts while the row is live and commits after its deadline. A one-line fix: in both apply funcs, return `ErrNotFound` when `!cur.Live(now)`.

- **ID:** QA-21-r1-3
- **Severity / kind:** Minor / risk (test gap)
- **Route:** backend
- **Where:** `lifecycle.go:291-293`; `publish.go:207`
- **Relates to:** X-3, X-4, ADR-031 item 2
- **Evidence:** M19 (the pass deletes without O-7) and M20 (remove-first without the public lock) both survive all tests.
- **Consequence:** Today the P-6 re-read makes a race between the pass and an owner build converge, so this is no defect now. Still, X-3's "each under the O-7 lock" and ADR-031's "every folder removal" are not pinned by any test, so a refactor can drop them without any test noticing.

- **ID:** QA-21-r1-4
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `lifecycle.go:284-288`; the comment in `wiring.go:96-98`
- **Relates to:** X-4 ("what is served converges independently of the row"), SH-10
- **Evidence:** Reading the code. When `DueWorkouts` fails, `Expire` returns before the sweep runs. The wiring comment says the sweep removes the public folder in that case, but in that pass it does not.
- **Consequence:** If the due query fails but `LivePublicIDs` would succeed, an expired page keeps being served past 60 s. This is unlikely because both queries use the same database. Running the sweep even when the list fails (it has its own query) makes X-4 hold as written.

- **ID:** QA-21-r1-5
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:** `architect/ADR-031-…md` (Status "proposed"), `architect/F-21-lifetime-and-expiry.md` (Status "draft", X-5), `architect/INDEX.md`
- **Relates to:** X-5, ADR-031
- **Evidence:** Reading the docs. The engineer's deviation (a failed startup pass is logged and serving goes on) is not recorded in X-5. ADR-031, the F-21 design and the INDEX row are still draft or proposed.
- **Consequence:** A later reader cannot tell that AC 6 relies on the lookup and on Recover's sweep when the startup pass fails. Record it in one line and mark the documents accepted.

## Engineer's deviations

| Deviation | Judgement |
| --- | --- |
| A failed startup pass is logged and serving continues | Accept. The lookup answers a due row as not found, and `Recover`'s sweep, which already ran, removed its public folder. Record it in X-5 (QA-21-r1-5) |
| O-7 taken per row, not per pass | Accept. This is X-4 as written, and it keeps owner writes from waiting for a whole pass |
| A `public sweep` line only when something was removed | Accept. It carries a count only, no `public_id` |
| A 30 s constant plus a test-only `expiryEvery` | Accept. `TestExpiryTimer` pins the constant |
| The real-binary concurrent-writer check deferred to F-30/F-27 | Accept. No second writer binary exists yet. `TestPublicLockAcrossProcesses` uses a real second process, and the real-binary single-writer run passed |
| `flock` only | Accept (ADR-031 item 4, Linux and a local file system, F-31) |
| No doc edits | See QA-21-r1-5 |

## Previous findings

- **QA-15-r2-1** (another process's build loses its new data file to a sweep): **fixed.** `TestQAR2SweepDuringOtherBuild` and `TestPublicLockAcrossProcesses` pass, and M10, M11, M12 and M17 are killed.
- No earlier F-21 round.

## Questions

1. QA-21-r1-1, for the user or architect: must a deleted workout's row be gone from the database file itself (`secure_delete` plus a WAL checkpoint after the deleting pass), or is the residue in free pages accepted? Suggestion: fix it. It costs two pragmas and one checkpoint, and AC-4 says "nothing remains".

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 expiry within 60 s, all places, generic page | `Expire`, `Delete`, timer in `wiring.go` | `TestExpiryEndToEnd`, `TestExpiryTimer`, `TestExpire`; real binary: 24 s |
| 2 draft at 48 h, 1 min before still there | `Record.Deadline`/`Live`, `DueWorkouts` | `TestDraftExpiry`, `TestDeadline`, `TestDeadlineBoundary`; real binary: 24 s |
| 3 published at 47 h, there at 49 h, gone at 7 d | X-1 | `TestPublishedAt47Hours`, `TestDeadline`, `TestDeadlineBoundary` |
| 4 publishing a deleted draft gives not found | lookup | `TestOverdueDraftNotFound` (publish of an overdue row); after deletion through the generic missing-row path (F-12 tests). No end-to-end publish after a 48 h deletion |
| 5 overdue, not yet swept, gives not found | lookup `Live` | `TestDraftExpiry` (all six routes byte-identical, headers included), `TestOverdueDraftNotFound` |
| 6 restart over a deadline | startup `Expire` before `Usage` | `TestExpiryAtStartup`; M13 and M21 killed |
| 7 lifetime change on the page | F-15 P-11 | `TestPublishEndToEnd` (30 days, `datetime` attribute) |
| 8 every table and folder empty | `dataLeft` | `TestExpiryEndToEnd`, `TestDraftExpiry`, `TestDataLeftClassifiesEverything`. Traced, but it passes for the wrong reason (QA-21-r1-1) |

The logs contain no `public_id` or token (`TestExpire`, `TestExpiryEndToEnd`, real binary). Not run: Caddy in front of the expiry (the generic page on 404 is F-15/F-19, already verified), and an X-6 bound with a slow build overlapping a deadline (accepted in the design). Probes and scripts: `<scratchpad>/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/qa-f21-probes/`.
