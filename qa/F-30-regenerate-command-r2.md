# QA F-30 Regenerate command - round 2

- **Date:** 2026-11-04
- **Verdict:** ready with conditions. r1-1 to r1-5 are fixed and confirmed by mutants and an end-to-end `deploy.sh` run, including a JS-only release. r1-6 is settled technically by ADR-017 rev. 8. The conditions: (1) the user accepts the r1-6 overlap risk, which is still pending; (2) AC 10's 100-workout timing on the VPS at the first F-30 deploy. One Minor finding about documentation.
- **Inputs:** `qa/F-30-regenerate-command-r1.md`; diff of 764f990 (backend fixes, `deploy/`, ADR-017 rev. 8, F-30 RG-10 and RG-13, F-31 DP-8, runbook). Working tree clean at 764f990.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0: every Go package ok, vitest 57 files / 1,438 tests passed, entry 57,159 / 60,000 B gzip, caddy-check and docker-check ok |
| `go vet ./...` | clean |
| `go test -race -count=1 ./internal/lifecycle ./cmd/workout-explorer` | ok |
| Mutants on a scratch copy of `src/server` (without `testdata/corpus/real`) | 14 run, 12 killed. **Killed:** R1 `clean-assets.sh` without the current-list filter (`TestCleanAssets`); R2a no stop guard in `Regenerate` (`TestRegenerateStoppedNeverDeletes`); R3a delete error always kept, R3b never kept, R3c re-read error ignored, R3d deleted error swallowed (all `TestRegenerateDeleteLeftover`); R5 delete decided from the build's error (`TestRegenerateKeeps`, new case); `pathless` without the `PathError` branch (r1 survivor, now `TestPathless`) and without the `LinkError` branch. **Survived:** R2b, the `ingest.parse` fall-through fix off. The window is a few instructions between two `ctx.Err()` checks and cannot be hit without a seam; R2a covers the delete. R4x, the cmd "workout deleted, files left" message off: a log text only. The first forms of R3a to R3c did not build; they were rerun in valid forms and are counted above |
| `deploy/deploy.sh <tag> local` in a scratch clone at 764f990 with shims for `gh`, `docker` and `sh` (fetch-caddy only), real binaries, synthetic corpus files | **v9.9.2** (release A): pre-f30 list with one legacy file. `rebuilt=0 … files_removed=1 bundles_removed=1`. 3 published workouts and 1 draft were then uploaded; the pages name `main-OLDAAAAA.js` and `main-Bwnjl-_1.css`. **v9.9.3** is a JS-only release: a new entry, the stale lazy chunk `map-view-OLDBBBBB.js` gone, the same stylesheet. Run 1 had one private folder read-only: `rebuilt=3 kept=1 files_removed=0 bundles_removed=0`, and the kept page keeps list v9.9.2. Run 2 (folder fixed, same tag): `rebuilt=4 kept=0 files_removed=2 bundles_removed=1`. `web/assets` then equals the v9.9.3 release exactly, and every page names only current files (r1-1 fixed). **Wrong `DEPLOY_DIR`:** exit 1 on the first remote step with "cd: … No such file or directory". Clone status empty, no `bundles.new` (r1-4 fixed) |
| `regenerate` directly: the stored original of one published workout replaced by `not-fit-png.fit`, `public/` read-only | exit 0, `rebuilt=3 deleted=1 kept=0`. Log: `fit rejected`, then `workout deleted … by=regenerate`, then WARN `workout deleted, files left until serve's next start` with `error: "lifecycle: delete: publish: remove folder: permission denied"` (no path, no public_id). Row and `private/{id}` gone (r1-3 fixed) |

## Findings

### Minor

- **ID:** QA-30-r2-1
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `architect/ADR-017-parse-isolation-and-limits.md:202` (Would choose differently, rev. 3) against `:108` (rev. 8) and F-31 DP-8
- **Relates to:** ADR-017 rev. 3 and rev. 8, F-31 DP-8, F-30 RG-13
- **Evidence:** Rev. 8 and DP-8 now say that the gate of 2 does not lower M(4). That is correct: M(4) = 2·A + 2·B + L, and the gated bound at N = 4 is the same expression. The rev. 3 bullet still says "The measured M(4) is above 1 GB and F-31's machine cannot hold it … put lifecycle steps 2 and 3 behind a gate of 2".
- **Consequence:** The ADR contradicts itself. A reader who follows the rev. 3 trigger after a high VPS measurement would build a gate that gains nothing. Fix: limit that bullet to "N above 4", or point it to DP-8's "ask the user".

## Previous findings

| ID | Status | How verified |
| --- | --- | --- |
| QA-30-r1-1 | fixed | `clean-assets.sh:18` filters names that are on the current list. Mutant R1 is killed. End to end with a JS-only release, the kept page keeps its release and a full rebuild removes it. The premise is checked on the build: the entry names every lazily loaded chunk, and `map-view` names the worker, so a changed chunk changes the entry's hash. RG-10 wording updated |
| QA-30-r1-2 | fixed | Root cause in `ingest.go:156` (untestable window, R2b survives). The guard in `Regenerate` (`regenerate.go:69`) catches every stop and is mutant-backed (R2a) |
| QA-30-r1-3 | fixed | Re-read after a failed `Delete`. A gone row counts as deleted, with the error and the O-10 line; a found row or a failed re-read counts as kept. R3a to R3d killed, probe above |
| QA-30-r1-4 | fixed | `r()` is now `cd '$dir' \|\| exit 1; $1`, for both local and ssh. Confirmed locally; ssh by reading only (same string) |
| QA-30-r1-5 | fixed | New case "(c) public build rejected by its parse" kills R5 |
| QA-30-r1-6 | decided (ADR-017 rev. 8); user acceptance pending | See the decision review below |

**Decision review, ADR-017 rev. 8 (r1-6).**
- *Problem:* a deploy's `regenerate` (365 MB) next to `serve` at its peak can exceed the 1,200m cap.
- *Choice:* accept the overlap and check `OOMKilled` after each deploy. The trigger for a remedy is cross-process parse slots.
- *Cost:* an OOM kill can end in-flight uploads, and the run ends with exit 3 and a retry.
- *Alternatives rejected:* raising the cap (about 1.6 GB, which leaves under 600 MB on 2 GB); the gate of 2 (no effect at N = 4).
- *Assessment:* the reasoning is sound and written down. Going over needs serve above about 835 MB within the seconds of a run. `serve` is restarted just before `regenerate`, so no retained heap carries over. Every kill is recovered: ADR-022, the per-file renames of RG-4, and RG-11's retry.
- *Exit codes:* any OOM kill that hits the run is already visible as exit 3, because killing PID 1 ends the exec'd process too.
- *What would change my mind:* friends really uploading near-cap files, or repeated exit 3 without another cause.

## Questions

1. **Architect:** Does `docker inspect -f '{{.State.OOMKilled}}'` report an OOM kill of a process started with `docker compose exec` (not the container's init)? And does it still show `true` after `restart: unless-stopped` has restarted `app` following a kill of PID 1? I did not probe this (no throwaway containers outside the scratchpad); Engine 29.9.2 is installed here. If either answer is no, the runbook check can miss exactly the event rev. 8 relies on. Two ways settle it: one `exec` of a memory hog above the cap on a test container, reading `OOMKilled` and `RestartCount` afterwards; or a runbook line that also checks `RestartCount` and reads exit 3 as a possible OOM.

## Coverage

- **AC 1, AC 2:** now met for shared names. `TestCleanAssets` (JS-only case) and the end-to-end runs above.
- **AC 4, AC 8 (deleted/kept counts):** `TestRegenerateDeleteLeftover` and the probe above. `kept=0` now holds for a deleted workout whose public folder could not be removed, so RG-14 is no longer blocked.
- **AC 7 / RG-7:** `TestRegenerateStoppedNeverDeletes`.
- **Everything else:** unchanged since r1 and not re-verified (the diff did not touch it).
- **Not run:** `deploy.sh` over ssh; VPS checks (AC 10, the first-deploy timing and the `OOMKilled` check); Caddy in front; a browser check of a kept page after the clean-up.
