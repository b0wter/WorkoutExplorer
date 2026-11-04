# QA F-29 About page and attribution list - round 1

- **Date:** 2026-11-04
- **Verdict:** not ready - the generated list misses every library MapLibre pre-bundles into its own dist (QA-29-r1-1), so the "complete" attribution list is not complete and their MIT/ISC/BSD notices are not reproduced.
- **Inputs:** `features/F-29-about-page-and-attributions.md`; `architect/F-29-about-page-and-attributions.md` v0.2 (AB-1..AB-8, PT table); commits 277043d (backend) and 71f94ab (frontend), working tree clean at 71f94ab.

## Build and test results

- `make ci`: exit 0. Go: all packages `ok` (incl. `internal/about`, `tools/attributions`, `cmd/workout-explorer` with the Caddy rig). Web: 56 files, 1436 tests passed. Bundle sizes: upload 2272/8000 B gzip, index/edit 57159/60000 B gzip, PASS. `make attributions` ran in both the local build and the Docker `go` stage; `docker-check: ok`.
- Built `build/web/about.html`: no `<script>`, no inline style, one same-origin stylesheet, external `<a>` links only from `sources.json` and the repository URL; no `<!--wx:attributions-->` left.
- Go deps in the list (12 modules + stdlib + Caddy) match `go version -m build/workout-explorer` exactly.
- Parser probe (copy of `about.go` in the scratchpad): `<script>`, quotes and `&` escaped; lone CR rejected; BOM case below.

## Findings

### Major

- **ID:** QA-29-r1-1
- **Severity / kind:** Major / defect
- **Route:** architect (source table of AB-5), then backend (generator)
- **Where:** `src/server/tools/attributions/main.go` `npmPackages`; design AB-5 "Sources of the list" row 1
- **Relates to:** feature Goal ("complete, automatically generated attribution list"), AC 4, AB-5
- **Evidence:** `build/web/.vite/license.json` lists 3 packages (maplibre-gl, svelte, uplot). `maplibre-gl/dist/maplibre-gl.mjs` and `maplibre-gl-worker.mjs` contain no `import ... from` at all: they are self-contained bundles of maplibre-gl's runtime `dependencies` (earcut, kdbush, potpack, quickselect, tinyqueue, @maplibre/geojson-vt (ISC); pbf, @mapbox/vector-tile, @mapbox/point-geometry, @mapbox/tiny-sdf, @mapbox/unitbezier (BSD/ISC); gl-matrix, bidi-js, murmurhash-js (MIT); @maplibre/mlt, @maplibre/vt-pbf, maplibre-gl-style-spec). maplibre-gl's `LICENSE.txt` (the only text the list shows) covers mapbox-gl-js, glfx.js and d3-color, not these. Vite's licence plugin only sees module boundaries, so pre-bundled packages are invisible to it by construction.
- **Consequence:** we redistribute their code in `/assets/maplibre-gl-worker-*.js` and the map chunk without the copyright notices their licences require, while the page claims to list "every library". The "by construction" argument of AB-5 does not hold for any pre-bundled npm package; a future MapLibre bump that adds a dependency is also silent. Fix options for the architect: walk the shipped package's `dependencies` in `node_modules` (installed versions approximate the bundled ones) and fail on a missing licence, or a committed per-version list for pre-bundled packages that fails on a version bump (the existing escape-hatch pattern).

### Minor

- **ID:** QA-29-r1-2
- **Severity / kind:** Minor / defect (wording)
- **Route:** architect (PT-10), user owns the wording
- **Where:** `src/web/about.html` "What the operator can do": "The operator cannot tell who uploaded a workout."
- **Relates to:** AB-2 test ("a self-hoster ... with another proxy must find no false sentence"), AC 6, PT-10, PT-11
- **Evidence:** the bundled Caddy logs no requests, but PT-11 leaves the operator's own TLS proxy free to log addresses and paths. Such a log has the `POST /api/workouts` with IP and time; the workout's upload time is in the database. The sentence is then false for that deployment.
- **Consequence:** a template sentence that depends on a deployment fact. Suggest "The app does not record who uploaded a workout" and let the operator file speak for the proxy.

- **ID:** QA-29-r1-3
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/compose.yaml:39`, `deploy/compose.build.yaml:12`, `deploy/deploy.sh`
- **Relates to:** AB-8 ("missing → placeholder keeps ... test deploys working"), F-31 DP-3
- **Evidence:** the operator folder is a bind mount with `create_host_path: false`, so Compose refuses to start `app` when `/srv/workout-explorer/operator` does not exist, whether or not the setting is used. `deploy.sh` has no preflight for it; the runbook adds the `install -d` line only to first-time setup.
- **Consequence:** the first deploy of this release onto an existing server (or a self-hoster who skips the optional-looking step) fails at `docker compose up` with app down until the folder is created. A one-line `r "test -d operator"` check with a clear message in `deploy.sh`, or an upgrade note, closes it.

- **ID:** QA-29-r1-4
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/about/about.go:62-84` (`ParseOperator`)
- **Relates to:** AB-8
- **Evidence:** probe: input `"\xef\xbb\xbf## Who runs this site\nJane\n"` renders `<p>﻿## Who runs this site Jane</p>` (U+FEFF is not a control or space character, so neither the control check nor `TrimSpace` catches it).
- **Consequence:** a file saved with a UTF-8 BOM silently loses its first heading. Strip a leading BOM or reject it with exit 2.

- **ID:** QA-29-r1-5
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/tools/attributions/main.go:117-139` (`cspLine`, `originRE`, `checkCSP`)
- **Relates to:** AB-6 (unlisted CSP origin fails the build)
- **Evidence:** only the first `Content-Security-Policy` line is read (`FindSubmatch`), and only `https?://host` sources are recognised. A scheme-less host source (`img-src cdn.example.org`), `wss://...`, or a second CSP header on another route passes the check.
- **Consequence:** the guard holds for today's single CSP line but can be bypassed by a normal-looking edit. Check every CSP line and treat any source that is not a keyword, `'self'`, `data:`/`blob:` or a hash as an origin.

- **ID:** QA-29-r1-6
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/tools/attributions/main_test.go`
- **Relates to:** AC 4 ("CI fails if a dependency's licence cannot be determined"), AB-6
- **Evidence:** `goPackages` (module without licence file, override lookup), `loadSources` validation (entry without licence or with `http:` link), the missing `.vite/license.json` and the template without marker have no test; only the npm and CSP failures do. Behaviour read correct in code.
- **Consequence:** the Go half of AC 4 is verified by reading only; a regression would ship a list with a silently missing module.

## Previous findings

None (round 1).

## Questions

1. (architect) QA-29-r1-1: is listing the pre-bundled dependencies of a shipped npm package in scope of AB-5? Settled by a decision in the design, either way; if "out of scope", the page's "every library" sentence must change.
2. (user, via architect) The template's map-tiles paragraph states OpenFreeMap's log retention ("deletes its logs after 7 days"). That is a third-party fact the template asserts for every release; PT-8 says it is re-checked at launch (F-32), but not on later releases. Acceptable as is? Settled by a yes, or by moving the claim into a "per their statement of <date>" form.

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 Footer link on every page | `upload.html`, `generic.html`, `index.html`, `PageFooter.svelte`, `PublishedView.svelte`, `edit/start.svelte.ts`, `about.html` | `about.test.ts` (About), existing page tests | traced |
| 2 Stored data, browser storage, tile host, 7/30 days, no backups, exact sentence | `about.html` | `about.test.ts` "states the facts" | traced; every *app* PT row (2-10, 12-17) found in the template |
| 3 UP-9 list | `about.html` | `about.test.ts` | traced, word for word |
| 4 Deps appear automatically; CI fails on unknown licence | `tools/attributions`, Makefile `build`, Dockerfile | npm failures tested; Go side untested (r1-6) | partly: complete for direct deps, not for pre-bundled ones (r1-1) |
| 5 OSM/OpenFreeMap and FIT licence listed | `sources.json` | rendered in built page (checked) | traced; FIT link not fetched (known) |
| 6 No deployment facts in template; example prompts; escaped operator text or placeholder | `about.html`, `operator.example.txt`, `about.go`, `settings.go` | `TestParseOperator`, `TestOperatorFileSetting` (exit-2 cases), `TestAboutRoute`, `TestFillNoRescan`, Caddy rig placeholder check | traced; one wording issue (r1-2); PT-1, 11, 12a, 15a each have a prompt |
| 7 No cookies, no third-party resources | `about.html`, Caddy | `about.test.ts`, Caddy rig (`Set-Cookie`, H-2 set) | traced |
| 8 OS theme without JS | `about.css` imports `plain.css` (`prefers-color-scheme`) | `about.test.ts` | traced; not checked in a browser |

Also checked: `/about` through Caddy (200, `no-cache` once, H-2 set), `/about.html` 404, 502 with Go down; template missing/ungenerated/marker missing → exit 1; operator file errors → exit 2 naming the setting; Pushover block only with both secrets (both-or-neither enforced in `pushoverKeys`); one-pass fill so markers in operator text stay inert; `deploy.sh` copies `about.html` and never touches `operator/`; release tarball includes `web/` (with `about.html`); operator mount read-only in both compose files.

Not run: a browser check of the page (theme, `<details>` without JS); a real Compose start with and without the operator folder (r1-3 is from the Compose mount semantics, not reproduced); the FIT licence URL.
