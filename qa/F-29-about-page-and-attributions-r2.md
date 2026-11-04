# QA F-29 About page and attribution list - round 2

- **Date:** 2026-11-04
- **Verdict:** ready - r1-1 (Major) is fixed: the list now holds every package MapLibre pre-bundles, checked against MapLibre's source maps. Two Minor findings remain (an incomplete copyright line in one override, and doc drift from the deploy.sh change).
- **Inputs:** `qa/F-29-about-page-and-attributions-r1.md`; commit db46fd5 (diff only); `architect/F-29-about-page-and-attributions.md` (AB-5, PT-8, PT-10, F-31 hand-off); user decision of 4 Nov 2026 on PT-8 (no OpenFreeMap retention figure, link to https://openfreemap.org/privacy/ instead); the user's deliberate bend of DP-3 (`deploy.sh` creates the empty `operator/` folder).

## Build and test results

- `make ci`: exit 0. Go: all packages `ok`, including `internal/about` and `tools/attributions`. Web: 57 test files, 1438 tests passed (including the new `scripts/bundled-licenses.test.mjs`). Bundle sizes: upload 2272/8000 B gzip PASS, index/edit PASS. `bundled-licenses: 42 added to 3` in both the local build and the Docker web stage. `make attributions` ran in the Docker `go` stage. `docker-check: ok`.
- Built `build/web/.vite/license.json`: 45 entries. All have an identifier. Three have no text (`murmurhash-js`, `is-reference`, `locate-character`); the committed overrides fill them, and the built `about.html` shows them.
- Built `build/web/about.html`: links to `https://openfreemap.org/privacy/`; "deletes its logs" is gone.

## Findings

### Minor

- **ID:** QA-29-r2-1
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/tools/attributions/overrides/npm_murmurhash-js@1.0.0.txt:3`
- **Relates to:** AC 4, AB-5 (full licence text)
- **Evidence:** the package's `README.md` (npm tarball and github.com/mikolalysenko/murmurhash-js) holds the licence as "License (MIT) / Copyright (c) 2011 Gary Court" plus the MIT text. The override has "Copyright (c) Gary Court" with no year and the note "the package ships no licence file". The package does ship the notice, just not as a separate file.
- **Consequence:** MIT asks us to reproduce "the above copyright notice", and the override changes it slightly. Use the README's line word for word and say the text comes from the README.
- **Checked correct:** `is-reference@3.0.3` (github.com/Rich-Harris/is-reference) and `locate-character@3.0.0` (gitlab.com/Rich-Harris/locate-character) have no LICENSE file in the repository root and no copyright line in the README (only "MIT"). `package.json` has author "Rich Harris" and license "MIT". "Copyright (c) Rich Harris" is the best the sources support. Both are dependencies of the Svelte compiler and not in the runtime bundle; listing them comes from the over-inclusion that AB-5 accepts.

- **ID:** QA-29-r2-2
- **Severity / kind:** Minor / defect (docs)
- **Route:** architect (F-29 design F-31 hand-off row), backend (compose comment)
- **Where:** `architect/F-29-about-page-and-attributions.md` F-31 row ("created by the runbook, **never written by `deploy.sh`**"); `deploy/compose.yaml:5` ("the runbook creates operator/")
- **Relates to:** F-31 DP-3, AB-8; QA-29-r1-3
- **Evidence:** `deploy.sh:52` now runs `r "mkdir -p operator"`. The runbook was updated; the design row and the compose comment were not. The bend is deliberate, but DP-3 itself ("the runbook creates them") does not mention it either.
- **Consequence:** the docs disagree with the code. A later reader may "fix" `deploy.sh` back, or not see why it creates the folder. One line in the design row (deploy.sh creates the folder but writes nothing into it) closes it.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-29-r1-1 (Major) | fixed | `scripts/bundled-licenses.mjs` walks the installed `dependencies` tree of each package in Vite's list, and AB-5 documents this. MapLibre 6.14.0 has 18 runtime `dependencies`; it has no `optionalDependencies`, `peerDependencies` or `bundledDependencies`. All 18 are in `license.json`. Cross-check: the `node_modules` packages in the `sources` of `maplibre-gl.mjs.map` and `maplibre-gl-worker.mjs.map` are exactly the 17 non-type dependencies, and each of them is listed. Transitive packages (style-spec's jsonlint, minimist, json-stringify-pretty-compact; pbf's resolve-protobuf-schema) are walked too. If a dependency is not installed, the script throws (tested); if its text is missing, the Go generator fails (existing rule). |
| QA-29-r1-2 | fixed | Now reads "The app does not record who uploaded a workout." PT-10 in the design still says "cannot tell who uploaded"; the template wording is the safer of the two and is acceptable. |
| QA-29-r1-3 | fixed (deliberate DP-3 bend) | `deploy.sh:52` creates `$dir/operator` before `docker compose up`. Docs drift: r2-2 above. |
| QA-29-r1-4 | fixed | `ParseOperator` strips a leading U+FEFF; `TestParseOperatorBOM` checks the heading survives. |
| QA-29-r1-5 | fixed | `checkCSP` reads every CSP line and treats every non-keyword, non-`data:`/`blob:` token as an origin, whatever its scheme. Tested: second line, a host with no scheme, `wss:`, a look-alike suffix host. Fails closed on `https:`, wildcards and ports. |
| QA-29-r1-6 | fixed | `TestModuleText` (no licence, missing from the cache, picks only licence files), `TestParseSourcesRejects`, `TestRunFailures` (no marker, no `license.json`). |

## Questions

None. The r1 question about PT-8 is settled by the user decision: the template paragraph carries no retention figure and links to the privacy statement; the design's PT-8 and F-32 rows match.

## Coverage

Diff only. AC 4 is now traced for npm (direct and pre-bundled) and Go, with tests. AC 2 and PT-8 match the user decision in the template and the built page. No test checks for the privacy link or for the absence of a retention figure; I checked that by reading the built page. I did not fetch openfreemap.org/privacy/ (F-32 checks it). I did not run a real deploy of `deploy.sh`.
