# QA F-01 Project skeleton and CI - round 3

- **Date:** 2026-10-10
- **Verdict:** ready with conditions. Both r2 findings are fixed exactly as design v1.3 specifies. No findings are open and I found no regressions. Everything that can be checked on this machine passes. The only things keeping this from "ready" are the four user conditions carried forward from r1: a real GitHub Actions run, Node 24, the `@v7` tags resolving, and a browser render. None of them can be checked here because there is no remote and no browser.
- **Inputs:** `features/F-01-project-skeleton-and-ci.md`, `architect/F-01-project-skeleton-and-ci.md` v1.3 (change log, sections Server exit codes, Tests, Local development address), `architect/overview.md` v1.2, ADR-001, ADR-002, `qa/F-01-project-skeleton-and-ci-r2.md`. Code: uncommitted working tree on `master` at `6be08bc`; changes since r2 in `Makefile`, `src/server/cmd/workout-explorer/main.go`, `main_test.go`.

## Build and test results

Environment as in r2: Go `go1.26.8`, Node `v22.22.2` (not the pinned 24; `EBADENGINE` warning only), GNU Make 4.4.1.

| Command | Result |
| --- | --- |
| `make -k ci` (repo root) | exit 0. svelte-check `167 FILES 0 ERRORS 0 WARNINGS`. `go test ./...`: `ok` for both packages. Vitest 23/23 passed. `entry index.html: 10613 / 60000 bytes gzip (9559 brotli) PASS` |
| `go test -race -count=1 -v ./...` (src/server) | PASS. `TestRun` now has 7 subtests, including `serve_empty_addr`. The 3 `runServer` tests and 4 `httpapi` tests pass as before |
| `go test -race -count=20 ./cmd/...` | ok (1.19 s) |
| `go vet ./...`, `gofmt -l .` | clean |

Probes. The mutation probe ran on a scratch copy. `git status --porcelain` was identical before and after, and no server process was left running.

| Probe | Result |
| --- | --- |
| `make -n dev-server dev-web DEV_ADDR=` | `-addr '127.0.0.1:8080'`, `DEV_ADDR='127.0.0.1:8080'` |
| `make -n ... "DEV_ADDR= "` (whitespace only) | same, falls back to the default (`$(strip)`) |
| `DEV_ADDR= make -n ...` (exported empty) | same, falls back to the default |
| `make -n ... DEV_ADDR=127.0.0.1:18080` and exported `DEV_ADDR=127.0.0.1:18080` | `127.0.0.1:18080` in both targets (no regression of r1-5) |
| `env -u DEV_ADDR make -n ...` | `127.0.0.1:8080` |
| `go run ./cmd/workout-explorer serve -addr ''` and the CI-built `build/workout-explorer serve -addr ''` | `serve: -addr must not be empty` on stderr, exit 2, nothing logged, nothing bound |
| Mutation: empty-address check disabled | `TestRun/serve_empty_addr` fails (`did not return within 5s`), so the new test catches it |

## Findings

None.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-01-r2-1 (stale `defaultAddr` comment) | **Fixed** | `main.go:33` now reads exactly the wording design v1.3 fixes: `// Must equal DEV_ADDR's default in the Makefile and the fallback in src/web/vite.config.ts (F-01 D-4).` The Makefile comment points to the design, and `vite.config.ts:8-9` names the other two. All three locations now cross-reference each other |
| QA-01-r2-2 (empty `DEV_ADDR` / `-addr ''` binds every interface) | **Fixed** | Makefile lines 16-19: `ifeq ($(strip $(DEV_ADDR)),)` / `override DEV_ADDR := 127.0.0.1:8080`. The `override` is there, so a command-line `DEV_ADDR=` is replaced too (probes above). `main.go` rejects an empty `-addr` after flag parsing and before the logger, signal context or listener exist, which matches the v1.3 exit-code row ("nothing logged, nothing bound"). Test case `serve empty addr` checks exit 2 and the stderr text, with a bounded wait |
| r2 Question 1 | **Settled** by the architect: option (a) for dev tooling and (b) for the binary, recorded in design v1.3 |
| r1 findings r1-1 to r1-6 | Still fixed. All tests that cover them pass, and the `DEV_ADDR` override probes confirm r1-5 |

No regressions. `main.go` changed only in the two places v1.3 allows. Everything else (CI workflow, Make target graph, health contract, server settings, front end, size script, README) is unchanged since r2.

## Conditions (carried forward, for the user after the first push)

These are not findings. They cannot be checked without a git remote and a browser, and they are the only reason the verdict is not plain "ready". Provide run links or log excerpts.

1. **Workflow starts on GitHub.** The first push triggers `ci`. The three `@v7` actions resolve and the run is green. The `setup-go` log shows Go 1.26.8, either directly or through the toolchain switch. The `setup-node` log shows Node 24.x. Every local run so far used Node 22.
2. **AC 2.** On a branch, push a failing Go test: the run goes red. Then push a failing Vitest test: the run goes red. Both logs still contain the `entry index.html: ... PASS` line (AC 4).
3. **AC 3.** On a branch, statically import a large real package (e.g. `maplibre-gl`) in `main.ts`: the log shows `FAIL` and the run goes red. Revert: the log shows `PASS` and the run goes green.
4. **AC 1 in a browser.** Run `make dev-server` and `make dev-web` (with the same `DEV_ADDR` in both if 8080 is taken), open the Vite URL, and check that the page shows `Server: ok (version dev)`. Stop the server, reload, and check that it shows `Server unreachable: HTTP 502`. The HTTP and proxy layers were verified in r2. The browser render was not.

## Questions

None.

## Coverage

| Acceptance criterion | Automated test | Verified locally | Needs real CI / manual |
| --- | --- | --- | --- |
| 1. `go run` + `vite dev` serve the placeholder via the proxy | `httpapi_test.go`, `main_test.go` (incl. empty addr), `health.test.ts` | r2 dev-path probe through the proxy; r3 `make -n` address probes | Browser render (Condition 4) |
| 2. CI on every push, fails on test failure | n/a | r1 probe A; `make -k ci` exit 0 now | GitHub run (Conditions 1, 2) |
| 3. CI fails above 60 KB, passes after removal | `resolveEntries`, `withinBudget`, `main` tests | r1 probes B-D | Real dependency on GitHub (Condition 3) |
| 4. Bundle size printed on every run (with a bundle) | `main` total-line regex | `make -k ci` output | CI log (Condition 2) |

Not run or not verifiable here: any GitHub Actions run, Node 24, whether the `@v7` tags exist (no external calls made), a browser render. I did not re-run the live dev servers in r3: the only dev-path change is the empty-value normalisation, and I verified that with `make -n`. The live proxy path was verified in r2 and has not changed since.
