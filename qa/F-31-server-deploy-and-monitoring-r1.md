# QA F-31 Server, deploy and monitoring - round 1

- **Date:** 2026-11-01
- **Verdict:** ready with conditions. No Blocker or Major. Conditions: the real-server checks listed under Coverage (AC 5 to 9 can only be checked on the VPS; AC 1, 3 and 4 need a run through the operator's proxy).
- **Inputs:** `features/F-31-server-deploy-and-monitoring.md`; `architect/F-31-server-deploy-and-monitoring.md` v0.3; ADR-034 rev. 2, ADR-035, ADR-021 rev. 6, ADR-031, ADR-032; TD Configuration (rows at lines 386 to 388). Reviewed: HEAD 6f6701d (clean tree), whole feature (first round, critical lane).

## Build and test results

- `make ci`: exit 0. Lint, Go tests (all packages ok), 1,331 vitest, build, size, `caddy-check` on pinned 2.11.7 (`TestCaddyForwardedFor`, `TestCaddyLogsNoRequests` included) all pass.
- `go test -race -count=1 ./internal/httpapi ./cmd/workout-explorer`: ok, ok.
- `TestLimiterMemory`: 29.5 MB at 60 (bound 40 MB), 108.0 MB at 240 (bound 112 MB).
- **Mutants**, on a copy of the tree in the scratchpad:
  - Go: 10 run, 7 killed. Killed: trust any peer (M1), IPv6 per /128 (M2), IPv6 per /56, loopback not trusted, IPv4/IPv6 limits swapped, no unmap of the setting, setting ignored. M9 (first `X-Forwarded-For` line instead of last) survives but is equivalent: Caddy sends exactly one line, and `TestCaddyForwardedFor` asserts that. M6, M7 and M8 survive (QA-31-r1-2).
  - Caddyfile, against the real Caddy: 7 run, 7 killed. No `header_up`; no `trusted_proxies_strict`; `private_ranges`; `{remote_host}`; `+X-Forwarded-For` (append); an access `log`; no `servers` block.
- **Compose stack run locally** (Docker 29.9.2, Compose 5.6.1, pinned distroless digest, `WX_LISTEN=127.0.0.1:18088`, data and web under the scratchpad). The stack was stopped and removed afterwards (`docker compose down`, no containers or networks left). Results:
  - Both services start. `server started` logs both limits.
  - Upload, then publish with its preview render: works with a read-only root and no `/tmp`. Caddy serves `index.html` and `.br` from `public/` (`0750`/`0640`, group 10001).
  - With both limits at 1, through Caddy from the host (the edge gateway, trusted by default):
    - `X-Forwarded-For` 198.51.100.7: 422, then 429.
    - 198.51.100.8: 422.
    - 2001:db8:5:6::1: 422. 2001:db8:5:6::2: 429 (same /64). 2001:db8:5:7::1: 422.
    - `1.1.1.1, 198.51.100.8`: 429 (rightmost entry counted).
  - Forged `X-Forwarded-For` from an untrusted container on `edge`: 422, 429, 429, so it is counted under its own address. From a container on `backend` straight to Go: 422, 429, the same result.
  - An `edge` container cannot reach `app:8080`. A `backend` container has no default route. `docker inspect`:
    - both services: non-root UIDs, read-only root, `cap_drop ALL`, `no-new-privileges`, `local` log driver, `unless-stopped`;
    - memory: `app` 1200m = memswap;
    - mounts: `bin`, `conf`, `web` and `public` read-only, data read-write for `app` only;
    - published ports: only `127.0.0.1:18088`.
  - No test address, `remote_ip` or `client_ip` in either container's log (`grep -c` = 0).
  - With `app` stopped, `/` and `/w/<id>` answer 200 and `/api/health` answers 502, and Caddy logs nothing for the requests.
  - `caddy reload` inside the container works.
  - `docker compose stop app` stays stopped. After an unexpected exit (SIGTERM to PID 1 from a sibling in the container's PID namespace), both services restart (`RestartCount` 1). `docker kill` counts as a manual stop, so it is not a valid crash test.
- **deploy.sh**, run from a clone at 6f6701d with `ssh`, `scp` and `gh` shims acting on the local stack. A stand-in for `fetch-caddy.sh` was hidden from `git status` (`assume-unchanged`).
  - Plain deploy: exit 0, old `web/assets` file kept, no `.new` leftovers.
  - Caddyfile-only change, with a page probe every 200 ms: reload, 5/5 probes 200, Caddy not restarted.
  - Caddy binary change: Caddy recreated, 2 failed probes, then 200.
  - Artefact with version `deadbee`: exit 1 after 62 s with the app's last log lines; `deadbee` stays running (no rollback, ADR-035).
  - Invalid `WX_TRUSTED_PROXIES`: QA-31-r1-3.
- **Base-image SQLite probe** (QA-31-r1-1): modernc sqlite v1.60.1, in the pinned base image with a read-only root as UID 10001.

## Findings

No Blocker, no Major.

### Minor

- **ID:** QA-31-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/compose.yaml:12` (`read_only: true`, no tmpfs for `app`); `src/server/internal/store/store.go:39` (no `temp_store`)
- **Relates to:** deviation "no /tmp"; DP-5; ADR-019
- **Evidence:** In the pinned base image with a read-only root, a sort that spills to disk fails with `disk I/O error (6410)` (SQLITE_IOERR_GETTEMPPATH). The same probe with `--tmpfs /tmp` succeeds. SQLite finds no writable temp directory (`/var/tmp`, `/tmp`, cwd `/home/nonroot` are all read-only). Today's database is tiny and the default 2 MB cache never spills, so nothing fails now.
- **Consequence:** Any later statement that needs a temp file fails as an I/O error, only in production: a migration building an index on a grown table, a large statement journal, a large `ORDER BY`. `temp_store(MEMORY)` in the pragma list is one line and also keeps temporary data off disk; the alternative is a small tmpfs.

- **ID:** QA-31-r1-2
- **Severity / kind:** Minor / risk (test gap)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/wiring.go:129-131`, `main.go:142`
- **Relates to:** AC 4, DP-1a, DP-1c, ADR-021 rev. 6 item 7 ("both logged in `server started`")
- **Evidence:** All three mutants survive `go test ./cmd/workout-explorer`: dropping `TrustedProxy: s.trustedProxy` (M7), dropping `UploadsPerHourIPv4` (M8), dropping the two log fields (M6). `TestSettingsReachTheStorage` exists for the storage cap; there is nothing like it for these settings.
- **Consequence:** If the wiring regresses, Go treats Caddy as untrusted, so every visitor shares Caddy's key and 60 uploads per hour in total, or the operator's limit is silently ignored. CI stays green. The failure is fail-closed, not a privacy leak.

- **ID:** QA-31-r1-3
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `deploy/deploy.sh:59-60`; `deploy/example.env:24-27`; `deploy/RUNBOOK.md:47`
- **Relates to:** DP-10 (3), DP-1b
- **Evidence:** I set `WX_TRUSTED_PROXIES=10.250.0.1,10.250.0.2` in the server's `.env`. A comma-separated list is the natural guess, and neither `example.env` nor the runbook says the values are space-separated. Caddy then refuses its config and crash-loops (`restarts=10`), so every page is down. deploy.sh exits 1, but prints only the app's log lines, which show a healthy `server started` with the right version. Caddy's error appears only in `docker compose logs caddy`.
- **Consequence:** The one deploy failure that takes all pages down is reported with misleading logs. Print Caddy's last lines too, and document the separator.

- **ID:** QA-31-r1-4
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `.github/workflows/ci.yml:27`
- **Relates to:** ADR-035 item 1
- **Evidence:** `actions/upload-artifact@v4` runs on `node20` (its `action.yml`). The other three actions are pinned at `@v7`, and upload-artifact's latest major is also v7 (v6 moved to `node24`). v4 excludes hidden files by default, which is harmless here: `build/web/.vite/manifest.json` is not used by Go or Caddy.
- **Consequence:** The step runs on a runtime GitHub is phasing out. The pins are inconsistent with the rest of the file. Not run on GitHub here.

- **ID:** QA-31-r1-5
- **Severity / kind:** Minor / defect (runbook accuracy, AC 5)
- **Route:** backend
- **Where:** `deploy/RUNBOOK.md:9`, `:23-24`, `:62`; `deploy/deploy.sh:4`
- **Relates to:** AC 5, DP-15
- **Evidence:**
  - The runbook never states the CPU architecture. Everything is linux/amd64 only: CI builds on ubuntu amd64, `caddy.env` pins only amd64, and deploy.sh runs the downloaded Caddy locally for `validate`, while `fetch-caddy.sh` refuses other platforms. An arm64 VPS (common in EU price tiers) gets `exec format error`. A macOS operator machine fails in `fetch-caddy.sh` and lacks `sha256sum`.
  - deploy.sh's tool list omits `curl` and `sha512sum`.
  - The `{bin,conf,web,web/assets}` brace expansion works in bash only.
  - "A changed `.env` recreates the Caddy container" holds only when one of Caddy's own values changes. Locally, a limit change recreated only `app`.
- **Consequence:** The one-hour rehearsal can fail on the first choice of VPS or laptop.

- **ID:** QA-31-r1-6
- **Severity / kind:** Minor / defect (docs)
- **Route:** architect; backend for `example.env:1`
- **Where:** `architect/F-31-server-deploy-and-monitoring.md` DP-6, the mounts table (lines 39 to 41) and the F-30 hand-off (line 74); `deploy/example.env:1`
- **Relates to:** engineer deviation "deploy user owns /srv/workout-explorer and .env"; ADR-034; F-30, F-27
- **Evidence:**
  - The design and the `example.env` header still say root owns the files and `.env` is `0600`. The runbook and the implementation use the `deploy` user (in the `docker` group, so root-equivalent anyway).
  - The F-30 hand-off says `docker compose exec app workout-explorer regenerate`. The base image's PATH (`/usr/local/sbin:...:/bin`) does not contain `/srv/workout-explorer/bin`, so that command fails. The runbook's full path (`/srv/workout-explorer/bin/workout-explorer delete`) is right.
- **Consequence:** The design and the code disagree on file ownership. F-30 and F-27 would copy a command that does not run.

## Previous findings

None (first round).

## Questions

1. **Runbook "Reports: `sqlite3` on the database"** (RUNBOOK.md:89, INF-2/INF-3 open). If root runs `sqlite3` on the host while `app` is stopped, it creates `workouts.db-wal`/`-shm` owned by root, with root's umask. `app` (UID 10001) then cannot open the database and fails to start. Answer that would settle it: the runbook says to run it as UID 10001 (`setpriv --reuid 10001 --regid 10001 --clear-groups sqlite3 ...`), or only while `app` runs. Or reports get a subcommand later (F-27).

## Coverage

| AC | Code | Test / check | Status |
| --- | --- | --- | --- |
| 1 deploy, one command, pages stay up | `deploy/deploy.sh`, `ci.yml` | Local shim run: three deploy kinds with page probe, failure path | Locally verified; real CI artefact and operator's proxy not run |
| 2 restart after crash/OOM/reboot, stop stays stopped | `compose.yaml` `unless-stopped` | Local: unexpected exit restarts both; `stop` stays stopped | Reboot and OOM kill only on the server |
| 3 plain HTTP only, same headers through proxy | Caddyfile `auto_https off`, `WX_SITE http://:8088`, one port | `ss`, Caddy log (auto HTTPS disabled), H-2 headers via Caddy, `caddy-check` | Through the operator's proxy (HSTS once) only on the server |
| 4 two limits, per address and /64, forged ignored | `settings.go`, `clientaddr.go`, `upload.go`, Caddyfile `servers` and `header_up` | `TestProxyAndUploadSettings`, `TestTrustedProxy`, `TestTwoRateLimits`, `TestCaddyForwardedFor`, mutants, local stack | Wiring untested (r1-2). Real external IPv4/IPv6 clients only on the server |
| 5 rebuild under an hour | `RUNBOOK.md` | Read only | Server (timed rehearsal); see r1-5 |
| 6 2 GB RAM, cap at or above the measured peak | `mem_limit: 1200m` | `docker inspect` 1258291200 | Peak "not yet measured" in the runbook: server |
| 7 parse time recorded | RUNBOOK §8 | — | "not yet measured": server |
| 8 4 uploads under cap, fifth refused | F-09 code | — | Server |
| 9 no backups | Nothing in the repo creates one | grep | Provider panel: server |

Also checked and found correct:
- No secret in the repository or the CI workflow. CI holds no server credential. The artefact holds build output and `COMMIT` only.
- Go publishes no port and has no outbound route.
- The ADR-031 lock: every writer of `public/` runs inside the `app` container (`exec` included) on one bind mount. Caddy mounts `public/` read-only and never locks. No second writer exists yet (F-27, F-30), so there was nothing to test across processes.
- Caddy's admin API listens on the container's own `localhost` only. Its log lines carry `127.0.0.1` (the reload), never a visitor.
- The engineer's deviations are acceptable as built:
  - `10.250.x/29` subnets (the runbook says how to change them);
  - no Caddy log block (`TestCaddyLogsNoRequests` and the mutant prove it);
  - memory cap 1200m (DP-8 arithmetic from F-09's 861 MB; to be replaced by the measured value);
  - base image pinned by digest (pull verified);
  - no `/tmp` (see r1-1);
  - deploy-owned files (see r1-6).

**Only on the real server:**
- AC 1 probe through the operator's proxy for the three deploy kinds.
- AC 2 reboot and OOM kill.
- AC 3: `ss`, an external scan of `WX_LISTEN`'s port and of 8080, the F-12 AC 6 guessed-path list, H-2 through the proxy with HSTS exactly once.
- AC 4 with real external IPv4 and IPv6 clients. Also: the operator's proxy appends `X-Forwarded-For`, and `WX_TRUSTED_PROXIES` matches its real source (10.250.0.1 for a host proxy was confirmed locally with default daemon settings).
- AC 5 timed rehearsal.
- AC 6 to 8 measurements: `memory.peak`, parse time, the fifth upload, `OOMKilled=false`.
- AC 9 provider backups and snapshots.
- DP-13 SSH key only, unattended upgrades, firewall.
- No tester address in the server's Docker logs.
- Carried conditions: F-22 r1 (browser check through Caddy), F-23 AC 8 and 9.

**Not run:**
- The GitHub CI workflow itself (artefact upload, `@v7` actions resolving, `gh run download` against a real run).
- `make corpus` (no parser change in this diff).
