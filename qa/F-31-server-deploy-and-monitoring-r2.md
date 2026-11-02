# QA F-31 Server, deploy and monitoring - round 2

- **Date:** 2026-11-01
- **Verdict:** ready with conditions. No Blocker or Major. The conditions are the real-server checks carried from r1 (AC 1 to 9 on the VPS). Two new Minor findings: the build-context allow-list misses upper-case `.FIT` and nested `testdata/` (r2-1), and the `server started` log fields are still untested (r2-2).
- **Inputs:** `features/F-31-server-deploy-and-monitoring.md` (AC "On a clean checkout, `docker compose build` ..."); `architect/F-31-server-deploy-and-monitoring.md` v0.4 (DP-2, DP-6, DP-15 to DP-17, verification row "AC 5, DP-16, DP-17", F-30 hand-off); ADR-034 rev. 3, ADR-035 rev. 1; TD Deploys and Repository layout. Reviewed the diff a4be3c8..ea62cc4 (3643b0b r1 minors; ea62cc4 `deploy/Dockerfile`, `Dockerfile.dockerignore`, `compose.build.yaml`, `docker-check`, RUNBOOK section 10, `ci.yml`, `.gitignore`). Clean tree.

## Build and test results

- `make ci` at ea62cc4: exit 0 in 1 min 25 s (warm local caches). Go packages ok, 1,331 vitest, size, `caddy-check`, and `docker-check: ok` (`docker build --no-cache`). Afterwards: no `deploy/.env` and no `wx-ci` container left. The `workout-explorer:ci` image stays, as designed.
- `go test -race -count=1 -run TestProxyAndLimitsReachTheHandler ./cmd/workout-explorer`: ok.
- **docker-check pin mutants**, run on copies of `deploy/`, `Makefile`, `go.mod` and `.nvmrc` in the scratchpad: 3 run, 3 killed, each with its own message:
  - final-stage digest changed in the Dockerfile;
  - `golang:1.26.7`;
  - `.nvmrc` set to 22.
- **Wiring mutants** (QA-31-r1-2), on a copy of `src/server`:
  - Deleting `wiring.go` line 129, 130 or 131 (`TrustedProxy`, `UploadsPerHourIPv4`, `UploadsPerHourIPv6`): all 3 killed by the new test.
  - Dropping the two `uploads_per_hour_*` fields from the `server started` log (`main.go:142`): survives (r2-2).
- **Self-hoster path on a clean checkout:** `git clone` of ea62cc4 into the scratchpad; `.env` from `example.env` with the `COMPOSE_FILE` line uncommented, plus `WX_DATA` (scratchpad, owner 10001, `0750`) and `WX_LISTEN=127.0.0.1:18089`.
  - `docker compose config --images` gives `workout-explorer:local` for both services. `docker compose build` builds it once. `docker compose up -d` starts both services.
  - Also checked: with the image deleted first, a plain `up -d` builds it and starts both services, with no pull error for `caddy`.
  - Through Caddy:
    - `/api/health` answers `{"status":"ok","version":"unknown"}`.
    - `/` carries the H-2 headers: CSP, `Referrer-Policy: no-referrer`, HSTS, `nosniff`, `X-Robots-Tag: noindex, nofollow, noarchive`.
    - A synthetic upload (`brick-run-bike.fit`) followed by publish (200) gives `/w/<id>` 200.
    - `/.vite/manifest.json` answers 404.
  - `docker inspect`, same as the operator path in r1:
    - users: `app` 10001:10001; `caddy` 10002:10002 with group 10001;
    - both services: read-only root, `cap_drop ALL`, `no-new-privileges`, `local` log driver, `unless-stopped`;
    - `app` memory 1258291200 = memswap;
    - networks and addresses: `app` 10.250.1.3 on `backend`; `caddy` on `backend` and `edge`;
    - published ports: only `127.0.0.1:18089`.
  - Mounts are the data mounts only: the data directory read-write for `app`, `public/` read-only for `caddy`.
  - No test address, `remote_ip` or `client_ip` in the logs (count 0).
  - `docker export`: the image holds only `/srv/workout-explorer/{bin/workout-explorer, bin/caddy, conf/Caddyfile, web/...}` on the base image (16 layers = 13 base + 3 COPY). Default user 65532.
  - Stack down, `workout-explorer:local` and the probe images removed, scratch data deleted. No containers or networks left.
- **Build-context probe:** `COPY . /ctx` with a copy of `Dockerfile.dockerignore`, context = repository root. Result: 305 files, only `Makefile`, the three `deploy/` files, `src/server` and `src/web`. Nothing from `testdata/corpus`, `build/`, `node_modules`, `deploy/.env*` or lower-case `*.fit` gets in. Gaps are in r2-1.
- Not run: the GitHub workflow (a cold `docker-check` against the 20-min timeout); `docker-check` under dash (Ubuntu's `/bin/sh`): this host's `/bin/sh` is bash, and SIGINT cleanup worked there.

## Findings

No Blocker, no Major.

### Minor

- **ID:** QA-31-r2-1
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `deploy/Dockerfile.dockerignore:10-11`
- **Relates to:** DP-16 ("real FIT files never enter the build context"), design verification row "no `testdata/` file reaches any stage"; real-files-never-committed rule
- **Evidence:** I added `src/web/PROBE.FIT`, `src/server/PROBE.FIT`, `src/server/probe.bin` and `src/web/.env.local`, then ran the context probe. All four reached the context.
  - `**/*.fit` is case-sensitive, and Garmin devices write `.FIT`. `.gitignore` uses `*.[fF][iI][tT]`.
  - Derived data files (`*.bin`, `*.br`) are not excluded.
  - Only the top-level `src/server/testdata` is excluded. `src/server/internal/ingest/testdata/fuzz/...` (9 files) reaches the Go stage, against the design's verification row.
  - With `**/testdata`, `**/*.[fF][iI][tT]`, `**/*.[bB][iI][nN]` and `**/*.[bB][rR]`, the probe drops to 292 files and none of these gets in. The build still needs none of them.
  - `src/web/.env*` also gets in, and Vite would read `VITE_*` keys from it into the bundle.
- **Consequence:** A stray real file in a developer's or self-hoster's `src/` goes into the daemon's build context and an intermediate stage's cache layer. It never reaches the final image, which `docker export` shows is clean, and nothing is published, so the exposure stays on that machine. Still, the stated invariant does not hold.

- **ID:** QA-31-r2-2
- **Severity / kind:** Minor / risk (test gap; remainder of QA-31-r1-2)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/main.go:142`
- **Relates to:** ADR-021 rev. 6 item 7 (both limits logged in `server started`)
- **Evidence:** Removing `"uploads_per_hour_ipv4", ..., "uploads_per_hour_ipv6", ...` from the `runServer` call leaves `go test ./cmd/workout-explorer` green (run on a copy with `build/` and the HTML templates).
- **Consequence:** The operator's only startup confirmation of the limits can disappear without CI noticing. The behaviour itself is now covered.

- **ID:** QA-31-r2-3
- **Severity / kind:** Minor / preference
- **Route:** backend
- **Where:** `Makefile` `docker-check`
- **Relates to:** DP-17
- **Evidence:** Each run leaves `workout-explorer:ci`, and every `--no-cache` build orphans the previous image and its build cache. The digest check counts the digest string in the Dockerfile, not in its `FROM` line, so a stale copy of the digest in a comment would satisfy it after a `FROM` change. Today the comment holds no digest.
- **Consequence:** Disk use grows on developer machines. The check is a little weaker than its message suggests. Not a CI problem (fresh runner).

## Previous findings

| r1 finding | Status | How verified |
| --- | --- | --- |
| QA-31-r1-1 SQLite temp dir | fixed | `temp_store(MEMORY)` added to the pragma list (`store.go:39`). Store tests pass. Probe not re-run |
| QA-31-r1-2 wiring untested | partially fixed | `TestProxyAndLimitsReachTheHandler` kills M7, M8 and the IPv6 variant. M6 (log fields) survives: r2-2 |
| QA-31-r1-3 separator, Caddy logs | fixed | `example.env` and RUNBOOK say "spaces, not commas". On failure, deploy.sh prints `ps` and the `caddy` logs (read; not re-run through shims) |
| QA-31-r1-4 upload-artifact@v4 | fixed | `@v7` |
| QA-31-r1-5 runbook | fixed | RUNBOOK and deploy.sh state amd64; the tool list is complete; brace expansion removed; the `.env` recreate wording is corrected |
| QA-31-r1-6 ownership, F-30 command | fixed | DP-6, the mounts table and `example.env` name the `deploy` user. The F-30 hand-off and RUNBOOK section 10 use the full binary path |

## Questions

1. Carried from r1, unanswered: RUNBOOK section 9 still says "Reports: `sqlite3` on the database". If root runs it on the host while `app` is stopped, it leaves root-owned `-wal`/`-shm` files, and `app` then cannot open the database. This would settle it: the runbook names the UID (`setpriv --reuid 10001 --regid 10001 --clear-groups sqlite3 ...`) or says "only while `app` runs", or F-27 gets a subcommand.

## Coverage

| Item | Code | Check | Status |
| --- | --- | --- | --- |
| AC `docker compose build` on a clean checkout, serves the same pages and API | Dockerfile, compose.build.yaml, RUNBOOK §10 | Clean clone: build, up, health, upload, publish, page, H-2 headers | Verified locally (amd64) |
| DP-16 allow-list | Dockerfile.dockerignore | Context probe | No corpus, `build/` or `.env` gets in; gaps listed in r2-1 |
| DP-16 one image, same paths, same hardening | Dockerfile, compose.build.yaml | `docker export`, `docker inspect` | Verified; equals the r1 operator path except for the mounts, which is intended |
| DP-16 pins agree | Dockerfile, compose.yaml, caddy.env, go.mod, .nvmrc | docker-check plus 3 mutants; Caddy `v2.11.7` checked in `docker-check` | Verified |
| DP-17 in `make ci`, fails without Docker | Makefile | `make ci` green. Fail-without-Docker read (`|| exit 1`), not run | Verified, partly by reading |
| DP-2 operator path untouched | compose.yaml unchanged | diff | Verified |
| r1 conditions (AC 1 to 9 on the VPS) | — | — | Still open: server only |
