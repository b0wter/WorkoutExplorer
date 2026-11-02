# QA F-01 Project skeleton and CI - round 2

- **Date:** 2026-10-10
- **Verdict:** ready with conditions. All six r1 findings are fixed, there are no Blocker or Major findings, and everything that can be checked locally passes. Two new Minor findings are open (stale `defaultAddr` comment; empty `DEV_ADDR` makes the dev server bind every interface). The verdict stays "with conditions" because conditions 1-4 from r1 (a real GitHub Actions run, Node 24, `@v7` tags resolving, browser render) still cannot be checked on this machine. They are listed for the user under Conditions.
- **Inputs:** `features/F-01-project-skeleton-and-ci.md`, `architect/F-01-project-skeleton-and-ci.md` v1.2, `architect/overview.md` v1.2, ADR-001, ADR-002, `architect/INDEX.md`, `qa/F-01-project-skeleton-and-ci-r1.md`, TD sections Tech stack, Front end (Budget), Hosting and operations (Local development). Code: uncommitted working tree on `master` at `6be08bc` (`Makefile`, `README.md`, `.gitignore`, `.github/workflows/ci.yml`, `src/server/`, `src/web/`).

## Build and test results

Environment: Go `go1.26.8`, Node `v22.22.2` / npm `10.9.7` (not the pinned Node 24; `npm ci` prints `EBADENGINE` and continues), GNU Make 4.4.1. No browser. `127.0.0.1:8080` is taken on this machine.

| Command | Result |
| --- | --- |
| `make -k ci` (repo root) | exit 0. `npm ci`: 0 vulnerabilities. `svelte-check --fail-on-warnings`: `167 FILES 0 ERRORS 0 WARNINGS`. `go test ./...`: `ok` for both `cmd/workout-explorer` and `internal/httpapi` (no `[no test files]`). `vitest run`: 2 files, **23 tests passed**. Size: `entry index.html: 10613 / 60000 bytes gzip (9559 brotli) PASS` |
| `go test -race -count=1 -v ./...` (src/server) | PASS. `TestRun` (6 subtests), `TestRunServerHealthAndStop`, `TestRunServerGracefulShutdown`, `TestRunServerListenerClosed`, plus the 4 `httpapi` tests |
| `go test -race -count=20 ./cmd/...` | ok (1.17 s), no flakes |
| `go vet ./...`, `gofmt -l .` | clean |
| `vitest run --reporter=verbose` | 11 `health.test.ts`, 5 `resolveEntries`, 1 `withinBudget`, 6 `main` - all pass |

Probes. The mutation probes ran on a throwaway copy of `Makefile` + `src/` + `build/web` in the scratchpad. The dev-path probe ran in the repo with the committed files unchanged. `git status --porcelain` was the same before and after.

| Probe | Result |
| --- | --- |
| Dev path in the repo: `DEV_ADDR=127.0.0.1:18080 make dev-server` (exported) + `make dev-web DEV_ADDR=127.0.0.1:18080` (make argument) | Server logs `server started` on `127.0.0.1:18080`. `GET http://localhost:5173/api/health` through the Vite proxy: 200, `no-store`, `{"status":"ok","version":"dev"}`. `/` serves the placeholder. After SIGINT to the server: `server stopped` logged, then the proxy answers `502`. Both ways of passing `DEV_ADDR` work, and no tracked file was edited (r1-5) |
| H again: script run by absolute path through a symlinked checkout | Prints the total line `... PASS`, exit 0 (r1-2 fixed) |
| G again: a JS file named in the manifest deleted | One line `bundle size: ENOENT: ... open '.../assets/index-B2EdSt_T.js'`, exit 1, no stack trace |
| Module imported with `node --input-type=module -e` (argv[1] unset) | Imports cleanly, guard does not run |
| Module imported from a script on stdin (`node -`) | **Throws at import** (`ENOENT ... lstat '<cwd>/-'` from `realpathSync`), exit 1. It fails loudly, so this is not a silent pass. Not reachable from npm, Make or Vitest. Noted, no finding |
| Mutation: `<=` to `<` in `withinBudget` | 1 test fails |
| Mutation: `report` always returns 0 | 1 test fails |
| Mutation: total-line wording changed | 2 tests fail |
| Mutation: `srv.Shutdown` replaced by `srv.Close` | `TestRunServerGracefulShutdown` fails (`runServer returned <nil> before the in-flight request finished`) |
| Mutation: `server started` logs `srv.Addr` instead of the listener address | `TestRunServerHealthAndStop` fails |
| Mutation: `serve` dispatch replaced by `return 0` | 4 `TestRun` subtests fail |
| `make dev-server DEV_ADDR=` (empty) | `serve -addr ''` → `server started` on `[::]:45485`: every interface, random port. `DEV_ADDR= make -n dev-web` gives `DEV_ADDR='' npm run dev`, so Vite falls back to `127.0.0.1:8080`. See QA-01-r2-2 |

The new tests fail when the behaviour they guard is broken. They do not pass for the wrong reason.

## Findings

No Blocker or Major findings.

### Minor

- **ID:** QA-01-r2-1
- **Severity / kind:** Minor / defect (documentation drift in code)
- **Route:** architect (lift "`main.go` unchanged" for a comment-only edit), then backend
- **Where:** `src/server/cmd/workout-explorer/main.go:33`
- **Relates to:** F-01 design D-4, "Local development address", Cost paragraph
- **Evidence:** The comment reads `// Vite proxies /api here; change both together.` Since v1.2 the default exists in three places (Go `defaultAddr`, `Makefile:16`, `vite.config.ts:10`). The design accepts that cost on the grounds that "they are cross-referenced by comments". The Vite comment names all three. The Go comment names two and does not mention the Makefile. The backend engineer saw this and left it, because the design says `main.go` stays "Unchanged". That was correct: engineers must not deviate from the design.
- **Consequence:** Someone who changes the default starting from `main.go` follows the comment, updates Go and Vite, and leaves the Makefile at 8080. The 502 from the mismatch makes it visible at once, so the harm is small. But the design's justification for the triplication depends on these comments being right. Fix: the architect clarifies that "unchanged" means behaviour, and the backend changes the comment to something like `// Must equal DEV_ADDR's default in the Makefile and the fallback in src/web/vite.config.ts (F-01 D-4).`

- **ID:** QA-01-r2-2
- **Severity / kind:** Minor / risk
- **Route:** architect
- **Where:** `Makefile:16,85,88`, `src/web/vite.config.ts:10`, `src/server/cmd/workout-explorer/main.go:67,95`
- **Relates to:** F-01 design D-4 ("empty or unset falls back to `127.0.0.1:8080`", stated for Vite only), Server command contract
- **Evidence:** An empty `DEV_ADDR` (passed as `make dev-server DEV_ADDR=`, or `export DEV_ADDR=` in the shell, which `?=` does not replace because the variable is defined) is handled differently by the two consumers. Vite treats empty as unset and proxies to `127.0.0.1:8080`. The Makefile passes `-addr ''`. The binary accepts it, and `net.Listen("tcp", "")` binds **all interfaces on a random port** (reproduced: `addr":"[::]:45485"`, `ss` shows `*:45485`). The binary has the same gap: `serve -addr ''` is accepted and does not count as a bad flag.
- **Consequence:** For dev this is operator error with a visible symptom (502 on the page). Two things make it worth a decision rather than a shrug. (1) The dev server is exposed to the LAN, which the `127.0.0.1` default was clearly meant to avoid. (2) The production contract in F-31 (systemd unit, behind Caddy) will rely on `-addr`, and the exit-code table says a "bad ... address" gives exit 1. An empty address should arguably be one, but today it silently means "every interface". Options: in the Makefile, treat empty as unset (`ifeq ($(strip $(DEV_ADDR)),)` → default), and/or have `serve` reject an empty `-addr` with exit 2. Either needs a design line. If the architect decides "documented constraint, no validation" (as for `:18080`), say so in D-4 and in the exit-code table, and I will close this.

## Previous findings

| r1 finding | Status | Evidence |
| --- | --- | --- |
| QA-01-r1-1 (size script `main` untested) | **Fixed** | `withinBudget` exported and used by `report` (`check-bundle-size.mjs:67-69,135`). The test covers `LIMIT_BYTES === 60000`, 60000 → true, 60001 → false. Six `main(dir)` tests cover every case the design lists: PASS line regex, FAIL with 100 KB of xorshift bytes, dynamic-only big chunk not counted, no manifest, no entry, missing file without throwing and with a single-line error. Mutation probes confirm the tests bite |
| QA-01-r1-2 (symlink guard) | **Fixed** | `check-bundle-size.mjs:146` uses `fs.realpathSync(process.argv[1])`. Probe H now prints the total line. No separate CLI file, as the design asks |
| QA-01-r1-3 (`main` untested) | **Fixed** | `main_test.go` matches the design's table: `run` table test (6 cases, bounded waits, stderr checked), `runServer` (a) health + start/stop log with the real addr, (b) graceful shutdown with an in-flight request, (c) closed listener → error + `server failed`. Race-clean over 20 runs. The mutation probes catch regressions in shutdown, addr logging and dispatch |
| QA-01-r1-4 (README restates contracts) | **Fixed** | `README.md` has exactly the allowed sections in order (title, Layout, Prerequisites, Make targets, Local development, Design docs). No measured values, no health contract and no ADR-002 rules (`grep` for `10613`, `no-store` and `gzip -9`: none). Three design-doc links |
| QA-01-r1-5 (dev address hard-coded) | **Fixed** | `DEV_ADDR ?= 127.0.0.1:8080` (`Makefile:16`), used by `dev-server` (`:85`) and `dev-web` (`:88`). `vite.config.ts:6,10,16` declares `process` locally (no `@types/node`; `svelte-check` clean over `vite.config.ts`). Overview section 6 is amended. Probe: works through both the make argument and the exported variable with no edits. Residual edge case: QA-01-r2-2 |
| QA-01-r1-6 (undocumented deviations) | **Fixed** | Design v1.2 records `@v7` (section CI, Action pinning), TypeScript below 7 (Dependencies table) and D-8 `--fail-on-warnings` with a local-silencing rule and its rationale. `package.json:12,24` and `ci.yml:13,14,18` match |
| r1 Question 1 (`cancel-in-progress`) | **Settled** by the user. Design v1.2 section CI, "Result guarantee". `ci.yml:5-7` matches |
| r1 Question 2 (no size line without a bundle) | **Settled** by the user. Design v1.2, criterion 4 scope |

No regressions found. CI workflow, Make target graph, health contract, `http.Server` settings and dependency set are unchanged from r1 and still match the design.

## Conditions (carried forward from r1, for the user after the first push)

These cannot be checked here: there is no git remote and no browser. They are not findings. They are why the verdict is not plain "ready". Provide run links or log excerpts.

1. **Workflow starts on GitHub.** The first push triggers `ci`, the three `@v7` actions resolve, and the run is green. The `setup-go` log shows Go 1.26.8 in use (directly, or through the toolchain switch from 1.26.0). The `setup-node` log shows Node 24.x. Every local run so far has used Node 22.
2. **AC 2.** On a branch: a failing Go test gives a red run, then a failing Vitest test gives a red run. Both logs still contain the `entry index.html: ... PASS` line (AC 4).
3. **AC 3.** On a branch: statically import a large real package (e.g. `maplibre-gl`) in `main.ts`. The log should show `FAIL` and the run should be red. Revert: `PASS` and green.
4. **AC 1 in a browser.** `make dev-server` + `make dev-web` (with a shared `DEV_ADDR` if 8080 is taken). Open the Vite URL and see `Server: ok (version dev)`. Stop the server, reload, and see `Server unreachable: HTTP 502`. The HTTP and proxy layers were verified here (probe above). The browser render was not.

## Questions

1. **For the architect (QA-01-r2-2):** Should an empty `DEV_ADDR` and an empty `serve -addr` be (a) treated as the default, (b) rejected, or (c) left as a documented constraint? *Settled by:* one line in D-4 and, for the binary, in the exit-code table.

## Coverage

| Acceptance criterion | Code | Automated test | Verified locally | Needs real CI / manual |
| --- | --- | --- | --- | --- |
| 1. `go run` + `vite dev` serve the placeholder, which calls health through the proxy | `main.go`, `httpapi.go`, `App.svelte`, `health.ts`, `vite.config.ts:10,16`, `Makefile:16,84-88` | `httpapi_test.go`, `main_test.go`, `health.test.ts` (11) | Yes, HTTP level through the proxy with `DEV_ADDR=127.0.0.1:18080`, including the 502 with the server down | Browser render (Condition 4) |
| 2. CI on every push; fails on a Go or front-end test failure | `ci.yml`, `Makefile:50-56,77` | n/a | r1 probe A (Makefile unchanged since, apart from `DEV_ADDR`) | Red run on GitHub (Conditions 1, 2) |
| 3. CI fails above 60 KB and passes after removal | `check-bundle-size.mjs`, `Makefile:72-73` | `resolveEntries` (5), `withinBudget` (1), `main` pass/fail/dynamic (3) | r1 probes B-D; r2 `main` tests | Real dependency on GitHub (Condition 3) |
| 4. Bundle size printed on every run (with a bundle) | `check-bundle-size.mjs:137-139`; `size` depends only on `build-web` | `main` total-line regex (PASS and FAIL) | Yes, `make -k ci` output | CI log (Condition 2) |

Checked and correct: README scope; design v1.2 test lists implemented one for one; overview section 6 amendment matches the code (the binary reads no environment; `DEV_ADDR` reaches only the Makefile and `vite.config.ts`, and the built bundle never sees it because `server.proxy` is dev-only); no `@types/node` in `package.json`; `svelte-check` covers `vite.config.ts` and reports 0 warnings.

Not run or not verifiable here: any GitHub Actions run, Node 24, whether the `@v7` tags exist (no external calls made), a browser render. The "run as script" branch (`process.exitCode = main(...)`) has no automated test. The design does not require one, and it was verified by hand through `make size` and probe H.
