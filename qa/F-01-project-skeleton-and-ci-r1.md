# QA F-01 Project skeleton and CI - round 1

- **Date:** 2026-10-10
- **Verdict:** ready with conditions. There are no Blocker or Major findings, and everything that can be checked locally passes. Two things are still missing: (1) no GitHub Actions run has happened yet (no git remote), so ACs 2-4 are verified only locally with `make -k ci`; (2) nobody has opened the AC 1 placeholder page in a browser. Both need to be shown after the first push (see Conditions).
- **Inputs:** `features/F-01-project-skeleton-and-ci.md` (working tree, modified), `features/README.md`, `architect/overview.md` v1.1, `architect/F-01-project-skeleton-and-ci.md` v1.1, `architect/ADR-001-repository-layout-and-build-entry-point.md`, `architect/ADR-002-bundle-size-budget-measurement.md`, `architect/INDEX.md`, TD sections Tech stack, Front end (Budget), Hosting and operations (Local development, Monitoring). Code: uncommitted working tree on `master` at `6be08bc` (`Makefile`, `README.md`, `.gitignore`, `.github/workflows/ci.yml`, `src/server/`, `src/web/`).

## Build and test results

Environment: Go `go1.26.8` (matches the `toolchain` line), Node `v22.22.2` / npm `10.9.7` (**not** the pinned Node 24; `npm ci` prints `EBADENGINE` warnings and continues), GNU Make 4.4.1. No browser is installed.

| Command | Result |
| --- | --- |
| `make -k ci` (repo root) | exit 0. `npm ci`: 66 packages, 0 vulnerabilities. `svelte-check --fail-on-warnings`: 0 errors, 0 warnings. `go test ./...`: ok. `vitest run`: 2 files, 16 tests passed. Build: `build/workout-explorer` (version `6be08bc-dirty`) and `build/web/`. Size: `entry index.html: 10613 / 60000 bytes gzip (9559 brotli) PASS`, which matches the engineers' figure exactly |
| `go test -race -count=1 -v ./...` (src/server) | PASS. 4 tests and 8 subtests in `httpapi`; `cmd/workout-explorer` has no test files |
| `go vet ./...`, `gofmt -l .` | clean |
| `npm ls --depth=0` | runtime: `svelte@5.57.2` and `uplot@1.6.32` only. Dev: the design's set exactly (`vite@8.3.4`, `@sveltejs/vite-plugin-svelte@7.3.1`, `typescript@6.0.3`, `svelte-check@4.7.6`, `@tsconfig/svelte@5.0.8`, `vitest@5.0.3`) |

Probes. Each ran on a throwaway copy of `Makefile` + `src/` in the scratchpad. After the probes, `git status` in the repo was the same as before.

| Probe | Result |
| --- | --- |
| A: failing Go test and failing Vitest test added | `make -k ci` exit 2. Output has `--- FAIL: TestQAFail`, `FAIL src/lib/qa.test.ts`, `make: *** [test-server]` and `[test-web]`. The size line is still printed: `entry index.html: 10613 / 60000 bytes gzip (9559 brotli) PASS` (AC 2, AC 4) |
| B: `main.ts` statically imports a ~124 KB raw module of random strings | `entry index.html: 87253 / 60000 bytes gzip (83218 brotli) FAIL`, `make: *** [Makefile:70: size] Error 1`, exit 2 (AC 3) |
| C: same module loaded through dynamic `import()` | Listed under `dynamic imports (not counted)` (75589 gzip). Entry 11230 bytes (this includes Vite's preload helper, correctly counted), PASS, exit 0 (ADR-002 item 1) |
| D: probe removed | `10613 / 60000 ... PASS`, exit 0 (AC 3, "passes again") |
| E: `<style>` added to `App.svelte` | The manifest gets a `css` entry, the size check counts `assets/index-*.css`, and the total goes up by 108 bytes (ADR-002 item 1) |
| F: manifest deleted | `bundle size: cannot read .../manifest.json: ENOENT`, exit 1 (ADR-002 item 5) |
| G: a JS file named in the manifest deleted | Uncaught `ENOENT` stack trace, exit 1. It fails, but not cleanly |
| H: script run by absolute path through a **symlinked** checkout | Prints nothing, **exit 0** (see QA-01-r1-2). `npm run size` and `make size` through the same symlink work correctly |
| Server CLI | No args → usage, exit 2. Unknown command → exit 2. `serve extra` → exit 2. `serve -h` → exit 0. Busy address (`127.0.0.1:8080` is taken on this machine) → JSON `listen failed` line, exit 1, no "started" line |
| Server HTTP (port 18081) | `GET /api/health` 200, `Cache-Control: no-store`, `Content-Type: application/json`, body `{"status":"ok","version":"6be08bc-dirty"}`. `HEAD` 200 with no body. `POST` 405 `Allow: GET, HEAD`. `/` 404. SIGTERM: `server stopped` logged, exit 0 |
| Dev path: `go run ... serve -addr 127.0.0.1:18082` + `vite` (proxy target temporarily changed in the scratch copy only) | `GET :15173/api/health` through the Vite proxy: 200, `{"status":"ok","version":"dev"}`. `/` serves the placeholder HTML. The transformed `App.svelte` contains `fetch('/api/health', { cache: 'no-store' })`. With the server stopped, the proxy answers `502`, so the page will show `Server unreachable: HTTP 502` (per `health.ts`). SIGINT on `go run` gives a graceful stop |

## Findings

No Blocker or Major findings.

### Minor

- **ID:** QA-01-r1-1
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/scripts/check-bundle-size.mjs:84-128` (`main`), `src/web/scripts/check-bundle-size.test.mjs`
- **Relates to:** ADR-002 items 3-5; AC 3, AC 4
- **Evidence:** Only `resolveEntries` is tested. The parts that make CI fail have no automated test: the PASS/FAIL comparison against `LIMIT_BYTES` (including the `<=` boundary), the non-zero exit code, the missing-manifest and empty-manifest exits, and the total-line format that AC 4 depends on. I checked them by hand (probes B, D, F). ADR-002 only requires a test for the manifest walk, so this is not a deviation.
- **Consequence:** A refactor that drops `process.exitCode`, flips the comparison or changes the total line would leave every test green. It would only show up when someone runs the AC 3 probe again by hand. `main(buildDir)` already takes a directory, so a test with a temporary build directory (one file just under and one just over 60,000 gzip bytes, and a missing manifest) is cheap.

- **ID:** QA-01-r1-2
- **Severity / kind:** Minor / defect (only on a path CI does not use)
- **Route:** frontend
- **Where:** `src/web/scripts/check-bundle-size.mjs:131`
- **Relates to:** ADR-002 item 5 ("a broken build cannot pass silently")
- **Evidence:** Probe H: `node <symlinked-checkout>/src/web/scripts/check-bundle-size.mjs` prints nothing and exits 0. The cause: `path.resolve(process.argv[1])` keeps the symlink, but `import.meta.filename` is the real path, so the "run directly" guard is false. Through `npm run size` / `make size` the relative argv resolves against the physical cwd, so it works there. The repo itself sits under a Syncthing folder, where symlinks are common.
- **Consequence:** If the script is called directly (by hand, or by a future CI or deploy script) through a symlinked path, the budget check is skipped and reported as success. Fix options: compare against `fs.realpathSync(process.argv[1])`, or move the CLI into a separate entry file that always runs.

- **ID:** QA-01-r1-3
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/main.go:50-135`
- **Relates to:** F-01 design, Server (exit code 2 for unknown/missing command, 10 s graceful shutdown, listen-before-"started")
- **Evidence:** `go test` reports `[no test files]` for `cmd/workout-explorer`. The design lists only `httpapi_test.go`, so this is not a deviation. I checked the exit codes and the SIGTERM/SIGINT shutdown by hand. `run(args, stderr)` and `runServer(ctx, srv, ln, logger)` were clearly written to be testable but have no tests.
- **Consequence:** The command-line contract (exit codes the systemd unit and deploy script in F-31 will rely on) and the shutdown path can regress without anyone noticing. A table test for `run` and one `runServer` test with a cancelled context would cover it. If the architect prefers to keep `main` untested, the design should say so.

- **ID:** QA-01-r1-4
- **Severity / kind:** Minor / defect (documentation drift)
- **Route:** backend, frontend
- **Where:** `README.md:62-72`
- **Relates to:** F-01 design, README ("Short, and only: ...")
- **Evidence:** The design limits the README to: a one-line description, the layout, prerequisites, the target table and local development. The README adds "Server" and "Front end" sections. These repeat the health contract and ADR-002, and line 72 hard-codes a measured value (`10613 / 60000 bytes gzip (9559 brotli)`).
- **Consequence:** The README now holds a second copy of contracts owned by `architect/`, and that copy will go stale. The hard-coded size goes stale with the first front-end change. Remove the two sections or reduce them to links to the design doc and ADR-002, or have the architect widen the README spec.

- **ID:** QA-01-r1-5
- **Severity / kind:** Minor / risk
- **Route:** architect
- **Where:** `Makefile:81-82` (`dev-server`), `src/web/vite.config.ts:10`, F-01 design D-4
- **Relates to:** AC 1; TD Hosting and operations (Local development)
- **Evidence:** On this machine `127.0.0.1:8080` is in use, and `workout-explorer serve` exits 1 with `listen failed` (reproduced). The documented dev workflow cannot run without editing two tracked files: there is no make variable for the address, and the Vite proxy target is hard-coded. The engineers had to point the proxy at a temporary target to check AC 1, and so did I.
- **Consequence:** AC 1 cannot be reproduced on a machine where 8080 is taken without uncommitted edits, and those edits get committed by accident. A fix needs a design decision, because overview section 6 rules out environment variables "unless a feature justifies one". One option: `make dev-server ADDR=...` plus a proxy target in `vite.config.ts` that reads from one place.

- **ID:** QA-01-r1-6
- **Severity / kind:** Minor / documentation (undocumented decisions)
- **Route:** architect
- **Where:** `src/web/package.json:12` (`--fail-on-warnings`), `.github/workflows/ci.yml:13,14,18` (`@v7`), `src/web/package.json:24` (`typescript ~6.0.3`)
- **Relates to:** F-01 design (npm scripts, CI table), overview section 5 ("anything else needs a design-doc entry")
- **Evidence:** All three deviations are reasonable, but the design doc still says `check` = plain `svelte-check` and actions `@v6`.
  - `--fail-on-warnings` changes CI policy: from now on, every Svelte a11y or compiler warning fails CI for all front-end features. That is a real decision and it is only recorded in a hand-off message.
  - `@v7` is allowed by the design's own rule, and all three actions use the same major. I could not check offline that `v7` exists for `checkout`, `setup-go` and `setup-node` (see Conditions).
  - The TypeScript cap at 6.x because of `svelte-check` is a constraint later features will hit when they upgrade.
- **Consequence:** The next engineer who reads the design will "fix" the code back to the design or get surprised by CI failing on warnings. The architect should add these to the design doc as v1.2. The extra unknown-chunk failure in the size script is within the intent of ADR-002 item 5 and needs no entry. The `go 1.26.0` + `toolchain go1.26.8` pair matches design D-7.

## Conditions (to be shown in round 2, after the first push to GitHub)

These cannot be verified without a remote. They are why the verdict is "ready with conditions" and not "ready". Provide run links or log excerpts.

1. **Workflow starts.** The first push triggers `ci`, the three `@v7` actions resolve, and the run is green. The `setup-go` step log shows Go **1.26.8** being used (if it installs 1.26.0, the log should show the toolchain switching to 1.26.8). The `setup-node` step log shows Node **24.x**. All local runs so far used Node 22.
2. **AC 2.** On a branch: push a failing Go test → red run. Then a failing Vitest test → red run. In both logs the `entry index.html: ... PASS` line is still present (AC 4).
3. **AC 3.** On a branch: statically import a large real dependency (e.g. `maplibre-gl`) in `main.ts` → log shows `FAIL` and the run is red. Revert → `PASS` and green. Locally this was shown with a large local module, not an npm dependency (probe B).
4. **AC 1 (manual, any machine with a browser and port 8080 free).** Run `make dev-server` + `make dev-web`, open the Vite URL, and confirm the page shows `Server: ok (version dev)`. Stop the server, reload, and confirm `Server unreachable: HTTP 502`. Only the proxy and HTTP layers were checked here; no browser render.

## Previous findings

None (round 1).

## Questions

1. **For architect / requirements: `cancel-in-progress` vs AC 2 "CI runs on every push".** When two pushes to the same branch come close together, the first commit's run is cancelled. That commit gets neither a red nor a green result. Is that within AC 2? *Settled by:* a sentence in the design (or AC) saying that only the latest commit per branch is guaranteed a result. Otherwise, set `cancel-in-progress: false`.
2. **For architect: AC 4 "printed on every run".** If `npm ci` or `vite build` fails, there is no bundle to measure, so no size line is printed. The run is red anyway. *Settled by:* confirming this is acceptable. I assume yes and do not count it as a finding.

## Coverage

| Acceptance criterion | Code | Automated test | Verified locally | Needs real CI / manual |
| --- | --- | --- | --- | --- |
| 1. `go run` + `vite dev` serve placeholder calling health through proxy | `main.go`, `httpapi.go`, `App.svelte`, `health.ts`, `vite.config.ts:9-11`, `Makefile:81-85` | `httpapi_test.go` (contract, HEAD, 405, 404), `health.test.ts` (11 cases) | Yes, at HTTP level through the proxy (alternate ports) | Browser render (Condition 4) |
| 2. CI on every push; fails on Go or front-end test failure | `ci.yml` (`on: push`, `make -k ci`), `Makefile:47-53,74` | n/a | Yes, `make -k ci` exit 2 with both failures (probe A) | Red run on GitHub (Conditions 1, 2) |
| 3. CI fails above 60 KB, passes after removal | `check-bundle-size.mjs`, `Makefile:69-70` | `resolveEntries` only (QA-01-r1-1) | Yes (probes B, C, D) | Red then green run with a real dependency (Condition 3) |
| 4. Bundle size printed on every run | `check-bundle-size.mjs:110-125`; `size` depends only on `build-web` | none for output format (QA-01-r1-1) | Yes, also in failing-test runs (probe A) | CI log (Condition 2) |

What I checked and found correct:
- The layout matches ADR-001 and overview section 4 exactly. No extra packages or empty folders.
- The Make target graph matches the design table, including `size` → `build-web` only and `corpus` as a placeholder that exits 0.
- `ci.yml` matches the design table item by item, except the documented `@v7`: `on: push`, `contents: read`, concurrency, `ubuntu-24.04`, 15 min, `cache: false` for Go, npm cache keyed on the lockfile, single `make -k ci` step.
- `http.Server` timeouts (`ReadHeaderTimeout` 10 s, `IdleTimeout` 60 s, no global Read/Write timeout), 10 s shutdown, and slog JSON to stdout with start and stop lines. No access log.
- The health contract matches the design byte for byte. The response has no side effects.
- The front end uses a relative fetch URL and has no third-party requests. The built `index.html` has no inline script (CSP-safe). `src/lib/health.ts` is pure.
- The size script uses only `node:fs`, `node:zlib` and `node:path`, gzip -9, with the limit as a single constant `60000`. It excludes dynamic chunks but counts the preload helper they pull into the entry.
- `.gitignore` covers `build/` and `node_modules/`.
- The `panic` in `httpapi.go:33` is on a path that cannot be reached (marshalling two strings). Accepted.

Not run or not verifiable here:
- Any GitHub Actions run.
- Node 24.
- A browser render.
- Whether the `@v7` action tags exist (no external calls made).
- An npm-installed large dependency for AC 3; a local module was used instead.
