# QA F-31 Server, deploy and monitoring - round 3

- **Date:** 2026-11-02
- **Verdict:** ready with conditions. No Blocker or Major. The release job and `deploy.sh` do what DP-10 and DP-18 say. Conditions: the GitHub-side checks of AC 1 (a real tag run, a red tag run, a re-pushed tag) and the real-server checks carried from r1 have not been run. Four Minor findings: three runbook inaccuracies, and one hardening item in CI.
- **Inputs:** `features/F-31-server-deploy-and-monitoring.md` (AC 1, AC 2); `architect/F-31-server-deploy-and-monitoring.md` v0.5 (DP-2, DP-10, DP-14, DP-15, DP-18); ADR-035 rev. 2; r2 report. Diff `a078e7e..690b766` (b614e89, 53de7ff, 95a8f95, 52fcbe7, 001a98d, 690b766), limited to `.github/workflows/ci.yml`, `deploy/deploy.sh` and `deploy/RUNBOOK.md`, plus the r2 fixes in b614e89.

## Build and test results

- `make -k ci` on 690b766 with a clean tree: **exit 0**. Go: all packages `ok` (results cached). Vitest: 51 files, 1,331 tests passed. Size check printed. `caddy-check` passed. `docker-check: ok`, which includes the new build-context leak probe.
- `VERSION=v9.9.9 make -n build-server` gives `-ldflags "-X main.version=v9.9.9"`. A `VERSION` set in the environment (as `$GITHUB_ENV` does) beats `VERSION ?=`.
- `make -k` with one failing target exits 2 (throwaway Makefile). So `make -k ci` still fails the `ci` job, and `release` (implicit `success()` on `needs: ci`) is skipped.
- **Fake release and deploy, with shims.** Shims for `gh`, `ssh`, `scp`, `docker` and `sh` (the last one only for `fetch-caddy.sh`, which got the cached pinned Caddy). The test used a local clone with local tags and a scratch "server" directory. On `restart app`, the `docker` shim started the **real** binary from the release, so `/api/health` came from that binary. The release was packed with the exact `tar`/`sha256sum` lines from `ci.yml`. Results:

  | Case | Result |
  | --- | --- |
  | First deploy of `v9.9.9` | exit 0. `{"status":"ok","version":"v9.9.9"}`. Atomic renames; Caddy recreated (new binary), no reload; HTML entries copied after the restart |
  | Redeploy, Caddy and Caddyfile unchanged | exit 0. Only `up -d` and `restart app`; no recreate, no reload |
  | Caddyfile-only change | `caddy reload` via `exec -T`, no recreate |
  | `v9.9.9-rc1` | refused, exit 1, nothing contacted |
  | Untracked file / modified tracked file | refused, exit 1 |
  | HEAD not at tag / tag unknown locally | refused, exit 1 |
  | Release missing | `gh` error, exit 1 |
  | Tarball missing from `SHA256SUMS` / one byte corrupted | refused, exit 1, no `ssh` |
  | Valid checksum, `COMMIT` = HEAD~1 (tag moved) | refused, exit 1 |
  | Binary reports `v9.9.30`, tag `v9.9.3` | no prefix match. After 61 s: `ps` and app/caddy logs, then the failure exit |

- Not run: anything on GitHub (no tag pushed, no workflow run, no `gh release create`) and anything on a real VPS.

## Findings

### Minor

- **ID:** QA-31-r3-1
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/RUNBOOK.md:5`, `:199`, `:341`, `:351`, `:359`
- **Relates to:** DP-15, AC 7 to AC 10
- **Evidence:**
  - Line 5 says "DP-1 to DP-17"; DP-18 now exists.
  - Line 199 shows `{"version":"unknown"}`. The handler returns `{"status":"ok","version":"unknown"}` (`httpapi.go:86`; seen in the probe).
  - Line 341 lists only `WX_LISTEN` and `WX_TRUSTED_PROXIES` as `.env` values that recreate Caddy. `WX_DATA` and `WX_WEB` are interpolated into Caddy's environment and mounts (`compose.yaml`), so they recreate it too.
  - The 13.5 heading cites "AC 6, 7, 8". The memory, parse-time and fifth-upload criteria are AC 8, 9 and 10 in the feature file.
  - Line 359, `time workout-explorer parse`, cannot run as written on the VPS. The binary is `/srv/workout-explorer/bin/workout-explorer` (not on PATH), and the all-caps synthetic file lives only in `testdata/`, which is not deployed. The command does not say where the file comes from.
- **Consequence:** the operator records measurements against the wrong AC, or cannot follow the parse-time step without guessing. A self-hoster checking the health output sees a different body than documented.

- **ID:** QA-31-r3-2
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/RUNBOOK.md:329`; `ci.yml` `retention-days: 1`
- **Relates to:** DP-18 ("the operator may delete a broken release by hand and re-run the job")
- **Evidence:** the hand-off artefact is kept for 1 day. Re-running only the `release` job more than a day after the tag run fails at `download-artifact`. Only "Re-run all jobs" works then: it rebuilds and re-tests, and it also produces new bytes for that version.
- **Consequence:** the documented recovery fails after 24 h with an unhelpful error. The runbook should say "re-run all jobs", or "within a day".

- **ID:** QA-31-r3-3
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `.github/workflows/ci.yml`, `release` job, `actions/download-artifact@v7`
- **Relates to:** DP-14, ADR-035 decision 1 ("the only holder of `contents: write` ... runs no repository code")
- **Evidence:** the one job that holds `contents: write` runs a third-party action pinned by a movable major tag, not by commit SHA. The token is not passed to the action as an input, but the job's token is live while the action runs.
- **Consequence:** low today, because the action is GitHub-owned. Pinning the action in this job to a full SHA makes "no code but ours meets the write token" a fact rather than trust in a tag. What would change my mind: a written decision that GitHub-owned actions are trusted at tag level.

- **ID:** QA-31-r3-4 (carried from r2-3, partially fixed)
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `Makefile` `docker-check`, `grep -c "$$base" deploy/Dockerfile`
- **Relates to:** DP-16, DP-17
- **Evidence:** image clean-up is fixed (trap `rmi`). The base digest check still counts occurrences anywhere in the Dockerfile. The digest in a comment plus a different digest on the `FROM` line still gives 1 and passes.
- **Consequence:** the pin check can pass while the pins disagree.

## Previous findings

| r2 finding | Status |
| --- | --- |
| r2-1 dockerignore misses `.FIT`, `*.bin`/`*.br`, nested `testdata` | **Fixed.** Case-insensitive globs, `**/testdata` and `**/.env*`. Enforced by the new probe in `docker-check`, which passed |
| r2-2 `server started` limit fields untested | **Fixed.** `startedFields` is shared by `serve` and the test, and the test asserts all four fields |
| r2-3 docker-check leaves images; digest grep not tied to `FROM` | **Partially fixed.** Images are removed; the grep is still not tied to `FROM` (QA-31-r3-4) |
| r1 conditions: real-server AC 1 to 9 | Open; not in scope of this round |

## Questions

None that block. For the architect: should the write job pin actions by SHA (QA-31-r3-3)? A yes or a no recorded in DP-14 settles it.

## Coverage

| Item | Evidence | Status |
| --- | --- | --- |
| DP-18 gating: only `^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$` | regex read; `rc1` and branches leave `release` unset | Verified by reading |
| DP-18 gating: release only after green `ci` | `needs: ci`, implicit `success()`; `make -k` exits 2 on failure | Verified (read and probe) |
| DP-18 least privilege | workflow `contents: read`; only `release` has `contents: write`; no checkout, no repository code; `GITHUB_REF_NAME` reaches `run:` through env, not `${{ }}` interpolation | Verified by reading; r3-3 |
| DP-18 `VERSION` = tag in the binary and `/api/health` | `make -n`; real binary in the probe reported the tag | Verified |
| DP-18 assets and checksum | tarball holds `workout-explorer`, `web/`, `COMMIT`; `SHA256SUMS` has one bare-name line; `deploy.sh` verifies it | Verified (probe) |
| DP-18 branch pushes upload nothing; 1-day retention | step conditions read | Verified by reading |
| DP-18 re-pushed tag: no `--clobber`, release fails red, deploy refuses a moved tag | no clobber or delete in `ci.yml`; moved-tag probe refused | Deploy side verified; GitHub side not run |
| DP-10 checks and failure paths | probe table above | Verified |
| DP-10 exact version match | prefix probe `v9.9.3` vs `v9.9.30` | Verified |
| DP-10 Caddy recreate vs reload vs neither | call logs | Verified |
| DP-15 Releasing section; runbook commands vs scripts | section 13.3 matches `deploy.sh` usage, refusals and output | Verified; r3-1, r3-2 |
| AC 1 on GitHub (green tag gives a release, red tag gives none, re-push leaves it unchanged, `rc1` gives none) | needs a real push | **Not run** (condition) |
| AC 2 probes through the operator's proxy | needs the VPS | **Not run** (condition, from r1) |

Not reviewed: commits before a078e7e, and the decision-log and TD edits beyond the runbook (except where the runbook cites them).
