# QA F-30 Regenerate command - round 1

- **Date:** 2026-11-04
- **Verdict:** not ready. One Major: the bundle clean-up keeps every old release that shares any file name with the current release (for example an unchanged stylesheet), so old bundles stay on disk although no kept workout uses them (AC 1, OQ-55). The data-loss and privacy paths hold: deletes happen only on a file rejection and only through `Delete`, and nothing a served page uses is removed.
- **Inputs:** `features/F-30-regenerate-command.md` (c14d9ca), `architect/F-30-regenerate-command.md` RG-1..RG-14, F-15 P-7 and F-31 DP-10 edits (d3afd0f), F-09 F-30 hand-off (v1.9 to v1.11), decision log of 4 Nov 2026, TD Versioning and Deploys. Code at 12830d5 (working tree clean at c14d9ca).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0: all Go packages ok, vitest 57 files / 1,438 tests passed, entry 57,159 / 60,000 B gzip, caddy-check, docker-check ok |
| `go vet ./...` | clean |
| `go test -race -count=1` on `cmd/workout-explorer`, `lifecycle`, `publish`, `privatedir`, `store` | all ok |
| `deploy/deploy.sh v9.9.1 local`, in a scratch clone with shims for `gh`, `docker` and `sh` (fetch-caddy only). The real binaries were v9.9.0 (old) and v9.9.1 (new) on synthetic files, and `exec` ran the real `regenerate` | run 1: one original replaced by `course.fit`, one private folder made read-only. Exit 0, `rebuilt=2 deleted=1 kept=1 skipped=0 files_removed=0 bundles_removed=0`. The kept page and the whole `pre-f30` list survived; the deleted workout's row, `private/` and `public/` were gone. Run 2 (folder fixed, same tag): `rebuilt=3 kept=0 files_removed=3 bundles_removed=1`, and `old-*` and `pre-f30` were gone. Run 3 (`exec` exits 137): deploy exit 3, nothing removed, a stale list and file kept |
| `/usr/bin/time -v regenerate` over 4 workouts plus the published all-caps file (`build/cap-500k.fit`) | exit 0, max RSS **364,748 KB**, 5.55 s wall |
| Mutants, run on a scratch copy of `src/server` without `testdata/corpus/real` | 23 run, 17 killed. Killed: the in-lock re-read; the allow-list; `Replace` creating the folder; delete on `canceled`; delete on `limit`/`time`; delete on a non-ingest error; migration from regenerate; `UpdateDerived` ignoring liveness; no skip of due rows; asset lines after an interrupted run; whole `pathless` off; `Assets` reading only the first link; no P-6 re-read; `sameRecord` ignoring fields; original always rewritten; a referenced list removed; files of a dropped list never removed. **Survived:** a delete decided from the build's error (r1-5); `pathless` without the `PathError` branch (only `LinkError` is exercised); no `syncDir` in `Replace` (not testable); `clean-assets.sh` without the missing-list guard (equivalent, `cat` fails under `set -e`). Invalid: two that did not build (lock removal; the first form of "original always rewritten", rerun in a valid form and killed) |

## Findings

### Major

- **ID:** QA-30-r1-1
- **Severity / kind:** Major / defect
- **Route:** architect (RG-10 wording), then backend
- **Where:** `deploy/clean-assets.sh:21`; `src/server/cmd/workout-explorer/cleanassets_test.go:80-84`; `architect/F-30-regenerate-command.md` RG-10
- **Relates to:** AC 1, AC 2, OQ-55, SH-10, feature goal ("only one … bundle is ever live")
- **Evidence:** In the probe, every page was rebuilt to v1.1.0 and named `main-B.js` and `main-S.css`. The v1.0.0 list was `main-A.js main-S.css map-A.js`, the v1.1.0 list `main-B.js main-S.css map-B.js`, and no workout was kept. Result: `files_removed=0 bundles_removed=0`, and `main-A.js` and `map-A.js` stay. The page names only the entry script and the main stylesheet. Whenever a release does not change the stylesheet, which is common for JS-only releases, the name in use is also on every older list with the same stylesheet, so all of them are kept. The test case "A used name that an old list shares keeps that list too" pins this behaviour (its `shared.css` is on the current list).
- **Consequence:** Old bundles pile up across releases until the stylesheet changes. AC 1 ("no file of the old version remains … except … kept") fails for an ordinary release shape. This is not a privacy or data-loss problem.
- **Fix direction:** When choosing kept lists, ignore names that are on the current release's list. This is safe because the names are content hashes: a page whose names are all on the current list loads only current chunks.

### Minor

- **ID:** QA-30-r1-2
- **Severity / kind:** Minor / risk (found by reading, not reproduced)
- **Route:** backend
- **Where:** `src/server/internal/ingest/ingest.go:142-156`, reached again through the reparse at `accept.go:72`
- **Relates to:** RG-3, RG-7, AC 7, F-09 hand-off follow-up 1
- **Evidence:** `parse` checks `ctx.Err()` first and then `dctx.Err()`. If SIGTERM cancels `ctx` between the two checks, `dctx` carries `context.Canceled`. That cause matches none of the cases, so the parse returns `KindInternal` "decode stopped", and `RejectedByFile` treats `internal` as a file rejection, which means the workout is deleted. The window is a few instructions wide. Before F-30 this only mislabelled an upload error; now it can delete a workout.
- **Consequence:** A stopped run can, very rarely, delete a workout that should be kept.
- **Fix direction:** On the fall-through, return `KindCanceled` when `ctx.Err() != nil`.

- **ID:** QA-30-r1-3
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/regenerate.go:70-74`
- **Relates to:** RG-3, RG-8, AC 4, AC 8, RG-14 (`kept=0`)
- **Evidence:** A rejected original with `public/` made read-only. Result: `deleted=0 kept=1`, log `"workout kept" … "lifecycle: delete: publish: remove folder: permission denied"`, and no `workout deleted … by=regenerate` line. The row and `private/{id}` were already gone, and `public/{public_id}` stayed served until `serve`'s next start (P-8; that part is `Delete`'s existing behaviour).
- **Consequence:** The counts and the O-10 log line are wrong for a deleted workout, and `kept=1` blocks RG-14's hash clean-up for no reason.

- **ID:** QA-30-r1-4
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `deploy/deploy.sh:36,40,51`
- **Relates to:** RG-10, RG-11
- **Evidence:** `r` runs `cd '$dir' && $1`. In the pre-f30 line, `$1` is `[ -d bundles ] || { rm -rf bundles.new && mkdir bundles.new && … }`, which the shell reads as `(cd && [ -d ]) || {…}`. With a wrong `DEPLOY_DIR`, the block ran in the caller's directory (local mode: the checkout; ssh: the remote home). It left `bundles.new/` in the checkout, and the next deploy refused with "the working tree is not clean".
- **Consequence:** A wrong `DEPLOY_DIR` leaves litter behind (and runs `rm -rf bundles.new` in the wrong place) instead of failing cleanly.
- **Fix direction:** `cd '$dir' || exit 1; $1`, or wrap the line in a subshell.

- **ID:** QA-30-r1-5
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/regenerate_test.go` (`TestRegenerateKeeps`, case (c))
- **Relates to:** RG-3, Verification row "Mutants" (a delete decided from the build's error)
- **Evidence:** I added a mutant that deletes when `Hook.WorkoutChanged` returns an error for which `RejectedByFile` is true. It survives, because the fake hook only returns `errors.New("build")`, never an `*ingest.Error`.
- **Consequence:** A regression that deletes on a public-build parse failure would pass CI. The design names this mutant as required.

- **ID:** QA-30-r1-6
- **Severity / kind:** Minor / risk
- **Route:** architect (decision before the first F-30 deploy), backend (runbook figure)
- **Where:** `deploy/RUNBOOK.md:469`; `deploy/compose.yaml:33` (`mem_limit: 1200m`)
- **Relates to:** RG-13, F-31 DP-8, ADR-017
- **Evidence:** I measured a max RSS of 365 MB for a run that rebuilds a *published* all-caps workout (Accept, public build, preview, compression). The runbook records 305 MB. With F-09's serve peak (up to 861 MB) plus L (112 MB) plus regenerate, the cgroup needs about 1,338 MB, above the 1,200 MB cap. Applying DP-8's formula with regenerate included gives (861 + 365 + 112) × 1.2 ≈ 1.6 to 1.7 GB, which leaves less than 600 MB of the 2 GB free, so RG-13's gate of 2 would trip.
- **Consequence:** If a deploy coincides with 4 uploads at the limits, the OOM killer can end `serve` (PID 1): in-flight uploads fail and the run is interrupted (RG-7, retry). This is an accepted outcome, but the recorded number understates it and the VPS measurement is still open.

## Previous findings

None (round 1).

## Questions

1. **Architect:** Is RG-10 meant to keep a list for a name that is also on the current release's list (r1-1)? If "no", it is a one-line fix plus a test change. If "yes", AC 1 needs an exception from requirements.
2. **Architect / user:** Should the RG-13 measurement on the VPS be a condition before the first deploy of F-30, given r1-6's arithmetic? Settled by the `memory.peak` row filled in, or by a decision to raise the cap or apply the gate of 2.

## Coverage

- **AC 1:** `TestRegenerateFormatBump` (new template and data file, `draft.bin` re-encoded, no old file, settings rows equal), deploy.sh run 2. Not met for shared names (r1-1).
- **AC 2:** `TestCleanAssets`, deploy.sh runs 1 and 2 (kept page's list survives; removed after the rebuild). I did not check in a browser that the kept page loads every chunk; all its files were present.
- **AC 3:** `TestRegenerateFormatBump`, second run: no new file, same counts.
- **AC 4:** cmd and lifecycle `TestRegenerateDeletesRejected`, deploy.sh run 1. The log names `product_name`/`manufacturer`; no public_id, path or coordinate field. The test's coordinate check only looks for the strings `lat`, `lon` and `position`. The generic page itself was not checked through Caddy; the folder is removed at once.
- **AC 5:** `TestRegenerateShorter`. It swaps the stored original instead of using a parser seam (deviation 5); the code path is the same.
- **AC 6:** `TestRegenerateKeptOnServerFailure` (a), (b), (c) end to end; `limit`/`time` at unit level only. The decision of 4 Nov 2026 (partial state) holds.
- **AC 7:** `TestRegenerateInterrupted`, `TestRegenerateCommitsAfterCancel`. A kill inside a commit step was not tested; by reading, each step is a rename or one statement.
- **AC 8:** deploy.sh exit 0 with deleted and kept counts; exit 3 and nothing removed on a failed run; exit 1 vs 3 are distinct.
- **AC 9:** whole-row equality in `TestRegenerateFormatBump`; settings checks in `TestRegenerateKeeps`. The export file waits for F-24.
- **AC 10:** not run on the VPS. Dev machine: 5 workouts including the all-caps file in 5.6 s.
- **RG-5/RG-6 (concurrent serve):** `TestRecheckUnderLock` and `TestRegenerateInPlace` are mutant-backed. By reading, every interleaving of an owner save and a run ends with the last row writer's build. Serve's own builds also pass through the re-read, and the full suite including `-race` passes. No process waits for the public lock while holding an SQLite transaction.
- **Privacy:** no `public/` file holds the original's bytes (`TestRegenerateFormatBump`). Hiding is applied through the same build. Regenerate writes nothing public outside `publish`, and `Assets` names are limited to `[A-Za-z0-9._-]`, so text in a page title can only cause over-keeping.
- **Not run:** VPS checks (AC 10, RG-13), Caddy in front of the deploy, a browser check of an old page after clean-up, `deploy.sh` over ssh.
