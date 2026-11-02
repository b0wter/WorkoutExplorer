# QA F-15 Publish pipeline and static page - round 2

- **Date:** 2026-10-25
- **Verdict:** ready with conditions. The Major QA-15-r1-1 is fixed: a hides-more build stopped between its two remove steps no longer leaves the older data file behind after the startup sweep. Condition: the AC 7 browser check (generic page, light and dark, JavaScript off) is still with the user. Three Minor findings. The most important one (r2-1) is a cross-process hazard that the new sweep rule introduces for F-21 and F-30, and no hand-off records it yet.
- **Inputs:** diff `a559fd2..fb627dc` ("F-15: QA-15-r1 fixes ..."), routed findings QA-15-r1-1, r1-2, r1-4, r1-5 and r1-6 (r1-3 deferred to F-22), `architect/F-15-publish-pipeline-and-static-page.md` (P-6, P-8, Build notes, Hand-offs), ADR-028 item 5 and Consequences, TD code structure (`cmd` rule) and Deploys row, F-30 feature file, SH-10, NFR Privacy. Engineer's notes from the hand-over.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0, 27.1 s. Go: 14 packages ok. Vitest: 40 files, 1,076 tests passed. Entry `index.html` 50,220 / 60,000 B gzip PASS. `upload.html` 2,270 / 8,000 B PASS. No shared chunk PASS |
| `go test -count=1 ./...` (uncached) | 14 / 14 ok |
| `go test -race -count=1` on `publish`, `lifecycle`, `cmd/workout-explorer` | all ok |
| `go vet ./...` | clean |
| r1-1 probe, rerun (scratch module copy, `qa_r2_test.go`): publish, then a hides-more change (`hide_ends` on, with a transform that stands in for F-16 and drops heart rate). The build is stopped at `remove folder`, once by `EACCES` and once by a panic standing in for a kill. Then `Sweep` runs with the row kept | Both modes: the state before the sweep is the r1 state (old `data.*.bin` and `.br`, no `index.html`). The sweep removed 1 entry (the folder), 0 files left. **Fixed** |
| SIGKILL loop, real binary built from `fb627dc` (port 18391), `ride-12h`, 80 kills at random points during title, `hide_ends` on and off, private, unlisted and `allow_download` changes, restart after each (the real startup sweep) | 61 complete pages, each with exactly `index.html`, its data file (hash matches the name) and `.br`. 19 with no folder. 0 broken, 0 folders without `index.html`, 0 stray entries. 4 recovery lines with `public_removed` > 0. 0 ERROR lines, 0 log lines with the `public_id` |
| Delete probe, real binary (port 18392): a file put where `public/{public_id}` belongs | `DELETE` 500, `private/` empty afterwards, retry 404. The ERROR line has the `id` and `publish: remove page: not a directory`, no `public_id`. **r1-2 fixed** |
| Mutants on the fix (scratch copy) | Killed: sweep keeps a kept folder without `index.html` (`TestKill`); no `.br` rule (`TestKill`); no boundary check after the name (`TestReferenced`); no `/` check (`TestReferenced`); every entry of a kept folder kept (`TestKill`); `Delete` returns early on a public error (`TestDeletePublicOrder`). **Survived:** fixed temp name `.name.tmp` with `O_TRUNC` (r1-4 reverted); the measure warning logs the raw path error (r1-6 reverted), see r2-3. Equivalent mutants (dropping the dot-name check, dropping the regular-file check) survived as expected |

## Findings

### Minor

- **ID:** QA-15-r2-1
- **Severity / kind:** Minor / risk (new with this fix; no code path triggers it until F-21 and F-30 exist)
- **Route:** architect (F-21 and F-30 hand-offs)
- **Where:** `src/server/internal/publish/publish.go:476-495` (`sweepFolder` removes every entry the current `index.html` does not link), `publish.go:260-288` (`write` renames the new data file and `.br` before `index.html`, with brotli in between). TD table "Deploys": the service is restarted, **then** `regenerate` runs while the server is up.
- **Relates to:** ADR-028 item 5, P-8, F-21 hand-off ("every minute ... under the O-7 lock"), F-30 AC "Running it twice in a row leaves every page working"
- **Evidence:** probe `TestQAR2SweepDuringOtherBuild`: a second `Area` on the same data directory, standing in for the server's sweep, runs `Sweep` while the first one, standing in for `regenerate`, is at `write index.html` of an in-place build that has a new data file. The sweep removes the new `data.*.bin` and `.br` (they are not linked from the old page yet). The build then returns `nil` and leaves `index.html` linking a missing file: `BROKEN PAGE: index.html links data.7dcbb7dc349d0092.bin, which is gone`. Before this fix, the same overlap only removed the other process's dot-prefixed temporary file. Its rename then failed and the workout kept its old page, which is F-30's "kept" path. The O-7 lock does not cover another process. The engineer flagged this, but no design document records it.
- **Consequence:** if F-21's minute sweep runs during a deploy's `regenerate`, it can break any workout whose build is between its data-file rename and its page rename. That window includes the brotli step, so it is a large part of each build. The page then loads with a missing data file until the owner's next save or the next deploy. Nothing leaks, but F-30's guarantee fails silently. Settle it in the F-21 and F-30 hand-offs before either is built. Options: a cross-process lock on the public area that both the build and the sweep take, or a sweep that leaves alone unlinked files newer than a short grace period (the r1-1 leftover is an old file, so the grace period does not weaken that fix).

- **ID:** QA-15-r2-2
- **Severity / kind:** Minor / preference (documentation)
- **Route:** architect (F-23 and F-24 hand-offs, ADR-028 item 5)
- **Where:** `architect/F-15-publish-pipeline-and-static-page.md` Hand-offs (F-23 and F-24 rows), `architect/ADR-028-...md` item 5 ("every entry that `index.html` does not reference")
- **Relates to:** P-4 (artefacts), P-8, ADR-028 item 5
- **Evidence:** reading. `referenced` is a byte match for `/` + the file name, where the next character is not a file-name character or `/`. `X.br` counts as linked when `X` is. It is not an HTML parse. The documents say only "reference". Neither the F-23 nor the F-24 row says that a new artefact must appear in `index.html` as a plain path ending in its file name. Today the data file passes because the preload `href` carries it as a plain path, and `TestKill` and `TestSweepAfterStoppedRemoveFirst` use the real template, so they would catch a template change for the data file.
- **Consequence:** if F-24 builds its download URL in JavaScript, or F-23 puts the preview URL only in a JSON or script context (where `html/template` writes `\/`), the sweep deletes the live artefact at every start and, once F-21 lands, every minute. Fix: one sentence in ADR-028 item 5 that defines "reference", plus a line in the F-23 and F-24 hand-off rows.

- **ID:** QA-15-r2-3
- **Severity / kind:** Minor / defect (tests)
- **Route:** backend
- **Where:** `src/server/internal/publish/publish.go:327` (`os.CreateTemp`), `publish.go:536-542` (`measure`), `publish_test.go`
- **Relates to:** QA-15-r1-4, QA-15-r1-6, P-6, P-14
- **Evidence:** mutants. Going back to the fixed name `.{name}.tmp` with `O_TRUNC` passes the whole `publish` suite. So does logging `err.Error()` instead of `fail("measure", ...)`, which puts the path and so the `public_id` into a WARN line.
- **Consequence:** both r1 fixes can be undone without any test failing. The second one is a P-14 privacy rule (no `public_id` in logs). Fix: a test that runs two `place` calls for the same name, or checks the temporary name pattern. And a test with a logger that makes `usage` fail (for example, a directory without read permission inside a removed entry) and checks that the line has no `public_id` and no `public/`.

## Previous findings

| r1 finding | Status | How verified |
| --- | --- | --- |
| QA-15-r1-1 (Major) | **fixed** | r1 probe rerun, both modes: the folder is removed. `TestKill` now runs the sweep after every kill point, and `TestSweepAfterStoppedRemoveFirst` was added (mutant killed). 80-kill loop on the real binary: 0 folders without `index.html`, 0 stray entries. ADR-028 item 5 and P-8 were updated. SH-10's bound on this path is the next sweep: startup today, every minute once F-21 lands |
| QA-15-r1-2 | **fixed** | Real-binary probe: the private folder is removed after a public failure, the errors are joined, and the retry gets 404. `TestDeletePublicOrder` covers one failing step and both failing steps |
| QA-15-r1-3 | deferred to F-22 (as routed) | Now recorded in the design's Build notes |
| QA-15-r1-4 | **fixed** in code, no test (r2-3) | `os.CreateTemp` with a dot prefix, then `Chmod` 0640. A failure to create releases the counted bytes. The cross-process hazard moved to the sweep (r2-1) |
| QA-15-r1-5 | **fixed** | Design "Build notes" lists the port shape, `Delete(ctx, id, public_id)`, the overcount after the post-build reread, the deadline, measure as 0 bytes, and the 206. TD `cmd` rule matches the code: only `parse.go` imports `datafile`, `precompress` and `workout` (test files aside) |
| QA-15-r1-6 | **fixed** at the three sites, no test (r2-3) | `measure` logs a WARN reduced by `fail` and counts 0. Note: `removeFolder` still stops the removal on a measuring error (`publish.go:379`; this predates r1 and was not in the r1 list). In practice that needs a walk error that `RemoveAll` would hit too, so I raise no finding |

## Questions

None for the user. r2-1 is a technical choice for the architect (lock or grace period), to settle before F-21 or F-30 is designed.

## Coverage

Only the diff and the routed findings were reviewed (follow-up round). The diff changes the behaviour of AC 8 (stopped builds, now including the sweep) and AC 9 (delete), both re-verified above. The other ACs are unchanged since r1 and were not re-run beyond `make ci`. Also checked and found correct:

- Text the owner writes (title, description) can only add matches to `referenced`, so it can keep a file but never remove one. A remove-first build that stops always leaves a folder without `index.html`, which the sweep removes whole. So an owner's text cannot keep a less hidden file alive.
- `Sweep` on a kept folder whose `index.html` cannot be read for a reason other than "missing" keeps the folder and reports the error. This fails towards keeping the folder, but it needs a broken file system.
- Startup order: recovery and the sweep run before the listener serves (log order in the kill loop).

Not run: the AC 7 browser check (user, condition), F-21 and F-30 paths (not built), and the Caddy run (the Caddyfile is unchanged since r1).
