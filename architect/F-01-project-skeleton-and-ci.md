# F-01 Project skeleton and CI: architecture

- **Version:** 1.3
- **Status:** accepted
- **Date:** 10 Oct 2026
- **History:** [architect/history/F-01-project-skeleton-and-ci.md](history/F-01-project-skeleton-and-ci.md) (change log, closed questions)
- **Feature:** [`features/F-01-project-skeleton-and-ci.md`](../features/F-01-project-skeleton-and-ci.md)
- **Depends on:** TD (system-wide picture), [ADR-001](ADR-001-repository-layout-and-build-entry-point.md), [ADR-002](ADR-002-bundle-size-budget-measurement.md)
- **Requirements:** NFR Performance (page budgets); TD Tech stack; TD Front end, Budget; TD Hosting and operations (Local development, Monitoring: API health endpoint, used by F-31)
- **Owner:** both. Backend first (server, Makefile skeleton), then frontend (web app, size check). CI workflow last (backend engineer).

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | Layout `src/server/` (Go module) + `src/web/` (npm package); root `Makefile`, `README.md`, `build/` | ADR-001 |
| D-2 | CI on GitHub Actions runs only `make -k ci`; `.github/workflows/ci.yml` is a thin wrapper | ADR-001, section CI |
| D-3 | Health endpoint `GET /api/health` returns `{"status":"ok","version":"..."}` | this doc |
| D-4 | Server listens on `127.0.0.1:8080` by default, flag `-addr`. For local development one Make variable `DEV_ADDR` (default `127.0.0.1:8080`) feeds both the server's `-addr` and the Vite proxy target | this doc, section Local development address |
| D-5 | Bundle budget: gzip -9 of entry + static imports + CSS, 60,000 bytes, from the Vite manifest, no third-party tool | ADR-002 |
| D-6 | Front-end tests with Vitest (Node environment); type check with `svelte-check` | this doc |
| D-7 | Toolchains pinned in `go.mod` and `.nvmrc`; Node 24 LTS; npm with lockfile | this doc, TD Conventions (Versions) |
| D-8 | Every `svelte-check` warning (compiler, a11y, TypeScript) fails CI | this doc, section Front end |

## Files created in F-01

```
Makefile
README.md
.gitignore                                  build/, src/web/node_modules/
.github/workflows/ci.yml                    GitHub Actions wrapper, see section CI
src/server/go.mod                           module workout-explorer; go + toolchain lines = current stable Go
src/server/cmd/workout-explorer/main.go     subcommand dispatch, flags, slog, server lifecycle
src/server/cmd/workout-explorer/main_test.go   exit codes and server lifecycle (section Server, Tests)
src/server/internal/httpapi/httpapi.go      NewHandler(version string) http.Handler
src/server/internal/httpapi/httpapi_test.go
src/web/package.json, package-lock.json, .nvmrc
src/web/vite.config.ts, svelte.config.js, tsconfig.json
src/web/index.html
src/web/src/main.ts, src/web/src/App.svelte
src/web/src/lib/health.ts (+ health.test.ts)  pure: turns a fetch result into display text
src/web/scripts/check-bundle-size.mjs (+ check-bundle-size.test.mjs)
```

Nothing else. No `internal/` packages for later contexts, no empty placeholder folders.

## Server

**Command.** `workout-explorer serve [-addr 127.0.0.1:8080]`. Unknown or missing subcommand prints usage to stderr and exits 2. `main` only parses flags, builds the logger and handler, and runs the server; no logic beyond wiring (TD Code structure, Dependency rules). The binary reads only the settings listed in TD Conventions (Configuration), none in F-01; `DEV_ADDR` is a Makefile concern only.

**Exit codes** (a contract: the systemd unit and deploy script in F-31 rely on it):

| Situation | Exit code |
| --- | --- |
| Missing or unknown subcommand, bad flag, extra positional arguments | 2 (usage on stderr) |
| Empty `-addr` (`-addr ''`) | 2, message `serve: -addr must not be empty` on stderr, nothing logged, nothing bound. An empty address would make `net.Listen` bind every interface on a random port; that is never what the caller wants, and production (F-31) must not fall into it silently |
| `-h` / `-help` | 0 |
| Listen fails (bad or busy address); logged `listen failed`, no `server started` line | 1 |
| Serve or shutdown fails | 1 |
| Clean stop after SIGINT or SIGTERM | 0 |

**`http.Server` settings.**

| Setting | Value | Why |
| --- | --- | --- |
| `ReadHeaderTimeout` | 10 s | Slow-header protection |
| `IdleTimeout` | 60 s | Free idle keep-alive connections |
| `ReadTimeout`, `WriteTimeout` | not set globally | A 20 MB upload from a phone (F-09) would hit a global limit; handlers that need deadlines set them per request with `http.ResponseController` |
| Shutdown | On SIGINT or SIGTERM, `Shutdown` with a 10 s grace period | systemd restarts during deploy (TD Deploys) |

**Logging.** `slog.NewJSONHandler(os.Stdout, nil)`. Log one line at start (`addr`, `version`) and one at shutdown. No per-request access log in F-01 (Caddy logs requests; F-31 decides whether Go adds one).

**Health endpoint contract** (consumed by F-31's uptime check and by the placeholder page):

| Item | Value |
| --- | --- |
| Route | `GET /api/health` (ServeMux pattern; also answers `HEAD`) |
| Other methods | `405` with `Allow` header (ServeMux default) |
| Status | `200` |
| Headers | `Content-Type: application/json`, `Cache-Control: no-store` |
| Body | `{"status":"ok","version":"<version>"}` |
| `version` | From `-ldflags "-X main.version=$(git describe --always --dirty)"`; `dev` under `go run` |
| Side effects | None. No disk, no database in F-01 (F-31 decides whether it later checks storage) |

**Tests.** Both files are required; `go test ./...` must not report `[no test files]` for `cmd/workout-explorer`.

| File | Must cover |
| --- | --- |
| `internal/httpapi/httpapi_test.go` (`httptest`) | `GET` gives 200, JSON content type, `no-store`, body with `status` `ok` and the given version; `HEAD` gives 200 with no body; `POST` gives 405 with `Allow`; unknown path gives 404 |
| `cmd/workout-explorer/main_test.go` (package `main`) | **`run` table test**, asserting the exit code and that stderr is non-empty where usage is expected: no args → 2; unknown command → 2; `serve extra` → 2; `serve -nosuchflag` → 2; `serve -h` → 0; `serve -addr ""` (args `[]string{"serve", "-addr", ""}`) → 2 with stderr containing `-addr must not be empty`, returning without blocking; `serve -addr <addr>` where the test already holds a listener on that address (open `127.0.0.1:0` first, pass its address) → 1, and the call returns without blocking. **`runServer` tests**, each with a listener on `127.0.0.1:0`, a cancellable context and a logger writing JSON to a buffer: (a) while running, `GET /api/health` on the listener's address returns 200; after `cancel()` the function returns `nil`, and the log contains `server started` (with the real `addr`) and `server stopped`; (b) graceful shutdown: a test handler that blocks until released receives a request, the context is cancelled, the handler is then released, the in-flight request completes with 200, and `runServer` returns `nil`; (c) a listener closed before the call → `runServer` returns a non-nil error and logs `server failed`. Every test bounds its wait (e.g. 5 s) so a regression fails instead of hanging |

Not tested, by decision: the signal wiring (`signal.NotifyContext`, standard library) and expiry of the 10 s grace period (would cost 10 s per run). Do not add a hook into `main.go` to make the grace period injectable just for tests.

## Front end

**Dependencies** (exact set; anything else needs a design-doc entry, TD Code structure, Dependency rules):

| Package | Kind | Why |
| --- | --- | --- |
| `svelte` (5.x) | runtime (compiled in) | TD Tech stack |
| `uplot` | runtime | TD Tech stack; installed now, enters the bundle when first imported (F-05) |
| `vite`, `@sveltejs/vite-plugin-svelte` | dev | Build and dev server |
| `typescript` (below 7, currently `~6.0.x`), `svelte-check`, `@tsconfig/svelte` | dev | Type checking. TypeScript stays below 7 until a `svelte-check` release supports TypeScript 7 in its release notes; the feature that lifts the cap records it in its design doc |
| `vitest` | dev | Tests; shares Vite config |

No SvelteKit, no router, no CSS framework, no jsdom (F-01 tests are Node-only; since F-05, jsdom is allowed for component smoke tests only, [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md)), no `@types/node` (see Local development address).

**Placeholder page** (`index.html` + `App.svelte`): on mount, `fetch('/api/health')` (relative URL, never a host) and show either `Server: ok (version <v>)` or `Server unreachable: <reason>` (network error, non-200 status, or invalid JSON). The formatting is a pure function in `src/lib/health.ts` so it can be tested without a DOM. No other UI (feature Out of scope).

**Vite config.**

| Setting | Value |
| --- | --- |
| `server.proxy` | `'/api' → 'http://' + DEV_ADDR`, falling back to `127.0.0.1:8080` (section Local development address) |
| `build.outDir` | `../../build/web`, `emptyOutDir: true` |
| `build.manifest` | `true` (needed by the size check, ADR-002) |
| `test.environment` | `node` |

**npm scripts:** `dev` (vite), `build` (vite build), `check` (`svelte-check --tsconfig ./tsconfig.json --fail-on-warnings`), `test` (vitest run), `size` (node scripts/check-bundle-size.mjs). The Makefile calls these; nobody needs to remember them.

**Warning policy (D-8).** `check` runs with `--fail-on-warnings`, so every Svelte compiler, accessibility or TypeScript warning reported by `svelte-check` fails `make lint` and CI, for every front-end feature from now on. A warning that is wrong in a specific place may be silenced only at that place, with `<!-- svelte-ignore <code> -->` and a comment on the same or previous line saying why. Silencing warnings globally (`compilerOptions.warningFilter`, `onwarn`, svelte-check `--compiler-warnings` options, or dropping the flag) needs a design-doc entry. Rationale: the a11y warnings are the cheapest guard for the accessibility parts of the spec, and a warning budget that only grows is never paid back.

**Bundle size check:** exactly as ADR-002. It reads `build/web/.vite/manifest.json`, walks each entry with `isEntry: true` through `imports` (not `dynamicImports`), adds `css`, gzips each file at level 9 and prints the lines defined in ADR-002. Failure to read any file named in the manifest is reported as a one-line `bundle size: ...` message and exit 1, like a missing manifest; no uncaught stack trace. When run directly, the "run as script" guard must also work when the script path goes through a symlink (QA-01-r1-2): compare `fs.realpathSync(process.argv[1])` with `import.meta.filename`. Do not add a separate CLI file.

**Size check tests** (`check-bundle-size.test.mjs`, Vitest, Node). Tests may use `node:os`, `node:fs` and `node:path` for temporary directories; the ADR-002 dependency rule applies to the script, not its test.

| Area | Must cover |
| --- | --- |
| `resolveEntries` (pure) | Fixture manifest with one entry, one static import, one dynamic import, one CSS file: the dynamic chunk is not counted. Transitive static imports, shared chunks once per entry, one set per entry, no entry, unknown chunk (as now) |
| Budget comparison | Export the comparison as a pure function, e.g. `withinBudget(gzipBytes)` returning `gzipBytes <= LIMIT_BYTES`, used by `main`. Test `60000` → true, `60001` → false, and that `LIMIT_BYTES === 60000` |
| `main(buildDir)` with a temporary build directory (`fs.mkdtempSync` under `os.tmpdir()`, removed afterwards) containing `.vite/manifest.json` and the named files | (1) Small entry → returns 0, and the last line logged for the entry matches `/^entry index\.html: \d+ \/ 60000 bytes gzip \(\d+ brotli\) PASS$/`. (2) Entry whose file is incompressible and clearly over the limit (about 100 KB of deterministic pseudo-random bytes, generated in the test from a fixed seed; no committed binary fixture) → returns 1 and the total line ends in `FAIL`. (3) Same over-limit file reached only through `dynamicImports` → returns 0 (not counted). (4) No manifest → returns 1. (5) Manifest without any `isEntry` → returns 1. (6) Manifest naming a file that does not exist → returns 1 and does not throw. Capture output with `vi.spyOn(console, 'log')` / `console.error` |

## Local development address (D-4)

**Problem.** The dev workflow needs the server address in two places: the server's `-addr` and the Vite proxy target. With both hard-coded, a machine where 8080 is taken can only run the dev setup by editing tracked files (QA-01-r1-5).

**Decision.** One Make variable, one name everywhere: `DEV_ADDR`.

| Where | Exactly |
| --- | --- |
| `Makefile` | `DEV_ADDR ?= 127.0.0.1:8080` next to the other variables, with a one-line comment pointing to this section. `?=` means a `DEV_ADDR` exported in the shell also works. Directly below, empty counts as unset (v1.3), so the Makefile matches Vite's fallback: `ifeq ($(strip $(DEV_ADDR)),)` / `override DEV_ADDR := 127.0.0.1:8080` / `endif`. `override` is required: without it a command-line `DEV_ADDR=` wins over the assignment |
| `make dev-server` | `cd $(SERVER_DIR) && go run ./cmd/workout-explorer serve -addr '$(DEV_ADDR)'` |
| `make dev-web` | `cd $(WEB_DIR) && DEV_ADDR='$(DEV_ADDR)' npm run dev` (still depends on `deps-web`) |
| `src/web/vite.config.ts` | Reads `process.env.DEV_ADDR`; empty or unset falls back to `'127.0.0.1:8080'`; proxy target is `` `http://${devAddr}` ``. To type `process` without adding `@types/node` (which would leak Node globals into the browser code's type check), declare it locally in the config module: `declare const process: { env: Record<string, string \| undefined> };`. Comment: default must equal the Makefile's `DEV_ADDR` default and Go's `defaultAddr` |
| `main.go` | Behaviour unchanged for every non-empty address. `defaultAddr` stays `127.0.0.1:8080`; the binary does not read `DEV_ADDR`. Two edits only (v1.3): the empty-`-addr` check (section Server, exit codes), and the comment on `defaultAddr`, which must read exactly `// Must equal DEV_ADDR's default in the Makefile and the fallback in src/web/vite.config.ts (F-01 D-4).` |

**Empty value (v1.3).** Empty `DEV_ADDR` means "use the default" in both dev consumers (Makefile and Vite), however it is passed (`make ... DEV_ADDR=`, `export DEV_ADDR=`). The binary is stricter: an empty `-addr` is a usage error (exit 2), because the binary is also the production contract and has no dev fallback. Defence in depth: if the Makefile normalisation is ever lost, `make dev-server DEV_ADDR=` fails loudly instead of exposing the dev server on the LAN.

**Usage.** `make dev-server DEV_ADDR=127.0.0.1:18080` and `make dev-web DEV_ADDR=127.0.0.1:18080` in the two terminals, or `export DEV_ADDR=127.0.0.1:18080` once per shell. Without it, both use `127.0.0.1:8080`. The Vite dev server's own port is unchanged (Vite moves to the next free port by itself).

**Constraint.** `DEV_ADDR` is `host:port` with an explicit host (`127.0.0.1:18080`, `localhost:18080`, `[::1]:18080`), because it is also used as a URL authority. `:18080` is not supported. No validation code for this case; the proxy simply fails (only the empty value is handled, see above).

**Why an environment variable.** Vite's config is a module evaluated by Vite, with no way to pass it custom command-line arguments; an environment variable is the only channel that does not need a second tracked file. The configuration rule (TD Conventions, Configuration) allows exactly this one variable for dev tooling. It never reaches the binary or the build output (`server.proxy` is used only by `vite dev`). No `.env` files.

**Cost.** The default `127.0.0.1:8080` now appears in three places (Go `defaultAddr`, Makefile, Vite fallback). Accepted: they are cross-referenced by comments, and a mismatch shows up immediately as a 502 on the placeholder page.

**Would choose differently if** more dev settings appear (more than this one variable): then a single dev config source would be worth a design entry.

## Make targets

All targets run from the repo root. `make ci` is what CI runs, with `-k` so independent steps all run and report.

| Target | Does | Depends on |
| --- | --- | --- |
| `deps` | `npm ci` in `src/web`; `go mod download` in `src/server` | |
| `lint` | `gofmt -l` (fails if any file listed), `go vet ./...`, `npm run check` | `deps` |
| `test` | `go test ./...` and `npm run test` | `deps` |
| `build` | `build-server` and `build-web` | |
| `build-server` | `CGO_ENABLED=0 go build -ldflags "-X main.version=..." -o ../../build/workout-explorer ./cmd/workout-explorer` | |
| `build-web` | `npm run build` | `deps` |
| `size` | `npm run size` | `build-web` |
| `ci` | `lint`, `test`, `build`, `size` | |
| `corpus` | Placeholder: prints "make corpus: no corpus yet (F-02)" and exits 0 | |
| `dev-server` | `go run ./cmd/workout-explorer serve -addr '$(DEV_ADDR)'` | |
| `dev-web` | `DEV_ADDR='$(DEV_ADDR)' npm run dev` | `deps` |
| `clean` | removes `build/` | |

`size` depends only on `build-web`, never on `test`, so with `make -k ci` the bundle size is printed even when a test fails (acceptance criterion 4).

## CI

Host: **GitHub Actions** (user decision 10 Oct 2026, closes OQ-1). One file, one job, no build logic (ADR-001 item 4).

| Item | Value | Why |
| --- | --- | --- |
| File | `.github/workflows/ci.yml` | Only workflow file |
| Workflow `name` / job id | `ci` / `ci` | Matches `make ci` |
| Triggers | `on: push` with no branch filter (every branch, every push) | Acceptance criterion 2. No `pull_request` (no outside contributors; a PR from a branch in this repo is already covered by its push) |
| `permissions` | `contents: read` (workflow level) | Least privilege; the job only reads code |
| `concurrency` | `group: ci-${{ github.ref }}`, `cancel-in-progress: true` | A newer push to the same branch cancels any run still in progress for that branch. See Result guarantee below |
| Runner | `runs-on: ubuntu-24.04` (not `ubuntu-latest`) | Image changes become a visible edit, not a surprise. `make`, `git`, `gzip` are preinstalled |
| `timeout-minutes` | `15` | A hung step fails instead of burning 6 h |
| Step 1 | `actions/checkout@v7`, default inputs (shallow, `fetch-depth: 1`) | `git describe --always --dirty` in `build-server` then yields the short commit hash; no tags needed yet (F-31 revisits) |
| Step 2 | `actions/setup-go@v7` with `go-version-file: src/server/go.mod`, `cache: false` | Go version comes only from `go.mod` (D-7). Cache off: F-01 has no Go dependencies, so there is no `go.sum` to key on. When the first Go dependency arrives, that feature switches to `cache: true` + `cache-dependency-path: src/server/go.sum` |
| Step 3 | `actions/setup-node@v7` with `node-version-file: src/web/.nvmrc`, `cache: npm`, `cache-dependency-path: src/web/package-lock.json` | Node version comes only from `.nvmrc` (D-7). Caches the npm download cache (not `node_modules`); `npm ci` still runs in `make deps` |
| Step 4 | `run: make -k ci` (default shell, repo root as working directory) | The only build step. `-k` keeps the size output in failing runs (criterion 4) |
| Not included | matrices, extra `run` steps, `env` blocks, artefact upload, badges, scheduled runs | Belong in the Makefile or in F-31 |

**Action pinning.** We pin to **major version tags**, not commit SHAs; all three actions use the same major, currently **`@v7`** (v1.2: the engineers found v7 to be the newest major when writing the file, which this rule asks for; QA round 2 condition 1 confirms the tags resolve). Major tags give non-breaking fixes automatically. SHA pinning protects against a compromised tag, but it is not worth the upkeep here: the workflow has no secrets, a read-only token and no deploy step. Revisit this if F-31 adds secrets or deploys from CI; at that point SHA pinning becomes the rule. When a newer major appears, a feature may move all three together and say so in its hand-off; do not mix majors for no reason.

**Result guarantee (settles QA round 1 question 1, user decision 10 Oct 2026).** Acceptance criterion 2 ("CI runs on every push") is met when every push starts a run and the **latest commit on each branch** always gets a finished red or green result. A commit superseded by a newer push to the same branch while its run is still in progress may be cancelled and get no result; that is accepted. Runs on different branches never cancel each other (the group includes `github.ref`).

Shape (for orientation; the table is normative):

```yaml
name: ci
on: push
permissions:
  contents: read
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
jobs:
  ci:
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: src/server/go.mod
          cache: false
      - uses: actions/setup-node@v7
        with:
          node-version-file: src/web/.nvmrc
          cache: npm
          cache-dependency-path: src/web/package-lock.json
      - run: make -k ci
```

## README (root)

**Rule.** The README says how to set up and run things. It does not restate what the system guarantees; contracts live in `architect/` and are linked, not copied. It contains no measured values (bundle sizes, timings, test counts) and no copies of contracts (health response, ADR-002 rules, shutdown behaviour, timeouts).

**Allowed sections, in this order, and nothing else:**

| Section | Content |
| --- | --- |
| Title + one line | What the repo is |
| Layout | Tree from TD Code structure (Repository layout), trimmed to what exists |
| Prerequisites | Go version from `go.mod`, Node from `.nvmrc`, make. Point to the files; do not copy version numbers |
| Make targets | The target table: one short line per target saying what it does. Naming the budget (`60,000-byte gzip budget, ADR-002`) is allowed; describing how it is measured is not |
| Local development | `make dev-server` and `make dev-web` in two terminals, then open the Vite URL; what the placeholder shows (`Server: ok (version dev)` / `Server unreachable: <reason>`); how to change the address with `DEV_ADDR` (one example) |
| Design docs | At most three links, one line each: `architect/INDEX.md` (all architecture docs and ADRs), this design doc (server command, health contract, size check), `spec/` (what the app does) |

## Acceptance criteria to verification

| Criterion | Verified by |
| --- | --- |
| `go run` + `vite dev` serve a placeholder page that calls the health endpoint through the proxy and shows it | Manual: `make dev-server`, `make dev-web` (with the same `DEV_ADDR` if 8080 is taken), open the page, see `Server: ok (version dev)`; stop the server, reload, see `Server unreachable`. Automated parts: `httpapi_test.go`, `main_test.go`, `health.test.ts` |
| CI runs on every push and fails when a Go or front-end test fails | CI trigger on all pushes, with the result guarantee in section CI; manual check by pushing a branch with a failing Go test, then a failing Vitest test |
| CI fails above 60 KB compressed and passes again after removal | Manual: on a branch, statically import a large package (e.g. `maplibre-gl`) in `main.ts`, see `FAIL` and a red run; revert, see `PASS`. Automated part: size-check tests (`resolveEntries`, `withinBudget`, `main` pass/fail/exit paths) |
| Bundle size printed on every run | `size` step output in every CI log, including runs with failing tests (`make -k`). Automated part: `main` test asserting the total-line format. **Scope (settles QA round 1 question 2, user decision 10 Oct 2026):** "every run" means every run that produces a bundle. If `npm ci` or `vite build` fails there is nothing to measure, no size line is printed, and the run is red anyway; that is accepted |

## Open questions

None. The questions closed on 10 Oct 2026 (OQ-1, OQ-2, QA r1 Q1 and Q2, QA r2 Q1) are in `INDEX.md` and history.
