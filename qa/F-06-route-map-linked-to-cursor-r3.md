# QA F-06 Route map linked to the cursor - round 3

- **Date:** 2026-10-14
- **Verdict:** **ready.** No Blocker or Major finding is open. Every acceptance criterion is traced to code, and each is covered by a passing automated test, by the operator's recorded browser and phone checks, or by both.
  - The operator's checks are MD1-MD12 and MP1-MP10. They pass on Firefox 156/Linux and on an iPhone 13 mini with Orion, recorded in `bench/MEASUREMENTS.md`.
  - Four Minor findings remain open. None of them blocks the feature:
    - QA-06-r3-1: the pixel-ratio cap is frozen at mount;
    - QA-06-r3-2: no style-spec validation in CI now that production skips it;
    - QA-06-r3-3: the start-up trigger line in `MEASUREMENTS.md` is out of date;
    - QA-06-r3-4: four documents disagree with each other.
  - One commit-hygiene note goes to the main session: `continue.md`.
- **Inputs:**
  - Design and decisions: `architect/F-06-route-map-linked-to-cursor.md` v1.4 (change log read), ADR-013 rev. 2, ADR-014 rev. 2, ADR-015 rev. 2, ADR-011 rev. 5, `architect/overview.md` v1.8, `architect/INDEX.md`.
  - Requirements: `features/F-06-route-map-linked-to-cursor.md`, `features/F-25-route-colouring.md` and `features/README.md` (decisions of 14 Oct 2026); spec AP-3 and the TD (Front end step 4, Budget) as changed in the working tree.
  - Measurements: `src/web/bench/MEASUREMENTS.md`.
  - Previous round: QA r2.
  - Code: the uncommitted working tree on top of `80c79fa` (`git diff`, 20 files).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci`, output captured | **exit 0.** `go vet` clean. svelte-check: 245 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: **629 passed** (24 files), matching the engineer's count. Bundle: `entry index.html: 10613 / 60000 bytes gzip PASS`, unchanged. Node 22 vs `engines` ≥ 24 is an environment warning only. |
| Bundle (a): scratch copy, ChartStack + cursor store | entry 40,528 gzip |
| Bundle (b): (a) + RouteMap | entry 42,557 gzip. Lazy: `map-view-*.js` 283,994 gzip and `map-view-*.css` 10,494 gzip. |
| Bundle (c′) | entry 41,311 gzip. Lazy: JS 280,162 and CSS 10,458 gzip. Worker 143,273 gzip (unchanged). |
| Static share (b) − (a) | **2,029** against the 2,100 ceiling: **PASS**. The engineer measured 2,056; the difference is the form of the temporary `main.ts`. The headroom is now 44 to 71 bytes. |
| Own lazy code (b) − (c′) | **3,868** against 5,000: **PASS**. This equals the engineer's 3,868. |
| Built `index.html`, build (b) | No `map-view`, MapLibre or `modulepreload` reference. |
| Production `map-view` chunk | Contains `validateStyle:!1`, so validation is off in the build. The `wx-map:*` User Timing marks and measures are present: only names and durations, no data. External hosts are the same as in r1 and r2: only `tiles.openfreemap.org` is fetched; the other URLs are links or error text. |
| Mutation probes (scratch copy; each mutant runs `vitest run src/components/map src/lib/route`) | **25 mutants, 24 killed, 1 survived.** New in v1.4:<br>- `routeLines`, killed 6 of 6: gap bridge removed, dropout bridge removed, gap pointer off by one, one-point runs kept, last run dropped, lon/lat swapped.<br>- Bridge layer, killed 4 of 4: not dashed, wrong filter, casing unfiltered, bridge above the line.<br>- `map-view` v1.4, 5 of 6 killed (V1, V3, V4, V5, V6): pixel ratio uncapped, `diff: false` removed, step 6a yield removed, step 6a abort check removed, idle mark removed. The survivor is listed below the table.<br>- r2 regression set, 9 of 9 still killed: M1, M5, M7, M14, T1, Q4, W1, M22, M8.<br>The survivor is V2 (`validateStyle: true` instead of `import.meta.env.DEV`). It is equivalent under Vitest, where `DEV` is true, so the build check above covers it. |
| Style validity probe (scratch Node test with `@maplibre/maplibre-gl-style-spec` 26.4.4 `validateStyleMin`, already installed as a MapLibre dependency) | `baseStyle(routeLines, markers)` and `mergeStyle(base, remote-like)` for gps-dropout, pause-hours and ride-12h: **0 errors**. |
| Working tree | `git status --porcelain` is unchanged apart from this report and its INDEX row. All builds and probes ran in the scratchpad. |

## Findings

### Minor

- **ID:** QA-06-r3-1
- **Severity / kind:** Minor / risk (not reproduced; no browser here)
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.ts`, `pixelRatio: Math.min(window.devicePixelRatio, MAX_MAP_PIXEL_RATIO)`
- **Relates to:** design v1.4 Map options (`pixelRatio`), ADR-013 rev. 2 decision 7, MD12
- **Evidence:** In MapLibre 6.14.0 a `pixelRatio` option sets `_overridePixelRatio` (`maplibre-gl-dev.mjs:56988`). `getPixelRatio()` then returns that value rather than the live `devicePixelRatio` (`:57911`), and only `setPixelRatio()` changes it (`:57921`). The ratio is therefore frozen at mount. Before v1.4 the map followed the device ratio.
- **Consequence:**
  - On desktop, browser zoom (Ctrl +/−) or moving the window to a screen with a different DPR leaves the canvas at the old ratio: blurry after zooming in, wasted pixels after zooming out, until reload.
  - Phones are unaffected.
  - MD12 (resize) does not cover this.
  - Fix: re-cap on DPR change, e.g. in the existing `resize` listener or a `matchMedia('(resolution: …)')` listener, with `map.setPixelRatio(Math.min(devicePixelRatio, 2))`. Alternatively, accept the limitation and write it down in the design.

- **ID:** QA-06-r3-2
- **Severity / kind:** Minor / test gap (risk for F-14, F-17, F-25)
- **Route:** architect
- **Where:** `src/web/src/components/map/map-view.ts` (`validateStyle: import.meta.env.DEV`); `map-style.test.ts`
- **Relates to:** design v1.4 Map options (`validateStyle`: "our tested base style needs no runtime check"), ADR-013 rev. 2 decision 7, ADR-012 (dev dependencies)
- **Evidence:**
  - Production now skips style-spec validation, for both our base style and the merged remote style.
  - `map-style.test.ts` asserts the shape (ids, order, filters, paint values) but never runs the style-spec validator. The only remaining check is the dev server's console.
  - A probe with the already-installed `@maplibre/maplibre-gl-style-spec` `validateStyleMin` finds 0 errors today.
- **Consequence:**
  - A later layer change that breaks the spec would pass CI and reach production unvalidated. Examples: a non-literal `line-dasharray`, a bad expression in F-25's colouring, F-17's range layer. MapLibre's swallowed `error` event would then be the only symptom.
  - The cheap fix is a Node test that validates `baseStyle(...)` and `mergeStyle(base, …)`. It needs the style-spec as a direct devDependency, which under the project's dependency rules is an architect decision.
  - The alternative is to accept dev-page validation only and say so.

- **ID:** QA-06-r3-3
- **Severity / kind:** Minor / doc defect
- **Route:** frontend
- **Where:** `src/web/bench/MEASUREMENTS.md`, "F-06 map", the "Start-up trigger" bullet
- **Relates to:** ADR-013 rev. 2 (trigger 300 ms), INDEX OQ-21 (resolved)
- **Evidence:** The bullet still says "**300 ms** if the user accepts OQ-21, otherwise **100 ms**". The user accepted OQ-21: the design, ADR-013 rev. 2, the INDEX and features/README all say 300 ms.
- **Consequence:** The next person to run MP1 (F-07, a MapLibre upgrade) reads a conditional threshold. The bullet should read: 300 ms, accepted 14 Oct 2026.

- **ID:** QA-06-r3-4
- **Severity / kind:** Minor / doc inconsistency
- **Route:** architect (items a and d together with the requirements engineer)
- **Where / evidence:**
  - (a) **Terminology.**
    - In `architect/overview.md` v1.8, "GPS dropout" is a run of samples without position, listed as "Not: Gap (time)", and "Bridge" covers "a GPS dropout or a gap".
    - `features/F-06-route-map-linked-to-cursor.md` (scope) and `features/README.md` (decision of 14 Oct) instead define "a dropout" as including "a time gap of 5 s or more".
    - The code follows the glossary: `routeLines` bridges both cases.
    - Fix: one definition. The feature files should say "dropout or gap", or the glossary should change.
  - (b) **ADR-011 rev. 5** still says the final F-06 share "is recorded at QA r2" and leaves the 2,200 fallback open. QA r2 recorded it, but the ADR does not. It should state: F-06 share **2,100**, reserve **1,600**, fallback unused (measured 2,005-2,056 across r2 and r3).
  - (c) **Design v1.4 MP1 row** lacks the QA r2 additions: held slow drag-scrolls (a) and a resting finger that must keep the map from starting (b). The operator ran both ("MP1 a and b" in `MEASUREMENTS.md`), so the design and the record disagree on what MP1 is.
  - (d) The **TD Front end step 4** and the F-06 feature scope still say "the route is added as one GeoJSON line". The design draws one source with up to two `MultiLineString` features (recorded and bridge). The visible result is one connected line, so AC 4 holds. ADR-013 records a similar refinement for the cursor marker ("single feature"), but this one is not recorded anywhere.
- **Consequence:** None in code. Readers of the requirements, the glossary and the bundle ADR get different answers. F-25's new AC ("dropouts stay dashed") depends on which definition of "dropout" applies.

### Commit hygiene (main session, not a code finding)

- `continue.md` at the repo root is untracked and not ignored, and its first line says "Do not commit it". A `git add -A` or `git add .` for the F-06 commit would include it. Delete it before committing, or stage paths explicitly.
- Everything else that must stay out of git is out:
  - The real FIT files and their expected JSON in `src/server/testdata/corpus/real/` and `expected/real/` are present but ignored (`git check-ignore` confirms the rules).
  - `*.bin`/`*.br` are ignored except the synthetic golden `.bin` files, which are allowed.
  - Nothing is staged.
  - No liberty style JSON exists in the tree. There is no `.gitignore` rule for it; the design relies on review ("The JSON is not committed"). That is acceptable while no tool writes one.

## Previous findings

- QA r2 had no findings.
- r2 question (F-07's page wiring: F-07's share or F-06's 2,100?) is **still open, not blocking.** No document answers it yet. It matters more now that the headroom is 44-71 bytes. It is for F-07's design.
- r1 fixes are still in place: the r2 regression mutants above are all killed. The partial-mount teardown, finger-count gate, `wantsHit`, hit geometry and radius per kind are unchanged apart from the v1.4 insertions.

## Questions

1. (Architect, F-07 design) As in r2: who pays for the page wiring, F-07 or F-06's 2,100?
2. (Architect) QA-06-r3-2: add a style-spec validation test (a direct devDependency on `@maplibre/maplibre-gl-style-spec`, already installed transitively), or accept dev-page validation only? *Settled by:* a line in the F-06 design or ADR-012.

## Coverage

**v1.4 code against design v1.4 and ADR-013 rev. 2**

- **D-7 bridges.**
  - `routeLines` makes a bridge where b − a > 1, or where `gaps` contains b. `gaps[i]` means time[i] − time[i−1] ≥ 5 s (`lib/datafile/model.ts`), matching the TD's "longer gaps stay gaps". One walk with a second pointer over the gaps; one-point runs are left out of the recorded segments.
  - Tests:
    - a seeded partition property: every consecutive pair is exactly one edge or one bridge;
    - golden gps-dropout: one bridge from 1,499 to 1,800, with recorded segments of 1,500 and 1,801 points;
    - golden pause-hours: one bridge;
    - golden ride-12h: no bridge;
    - a lone point, a gap inside a missing run, hidden ends, M = 1 and M = 2.
  - Layers: `wx-route-bridge` (dashed, butt caps, filtered to `bridge`) sits below the casing and the line, and both of those are filtered to `recorded`.
  - Bridges carry no samples, so hover and touch on a bridge's middle hit nothing (AC 6's rule).
  - F-25's AC and ADR-013 rev. 2 keep bridges dashed and uncoloured.
- **Start-up, ADR-013 rev. 2 decision 7.**
  - `validateStyle` is on in dev only and off in the build (checked in the chunk).
  - `setStyle(remote, { diff: false, transformStyle: mergeStyle })`: the `_updateStyle` path passes `this.style.serialize()` as `previous`, so the `wx-` layers carry over (read in 6.14.0, as in r1).
  - `pixelRatio` is capped at 2 (see r3-1).
  - Step 6a: an extra `whenQuiet` plus an abort check, both covered by tests and killed mutants.
  - User Timing: measures and marks with names only. `performance.measure(name, { start })` needs User Timing L3, which every browser in the spec's compatibility list ("last two versions…") supports.
- **Teardown and failure paths:** unchanged since r2. The new `map.once('idle')` sits inside the `try`.
- **Module and import rules:** unchanged. `RouteMap` adds only a `performance.mark` call and its static imports are unchanged.
- **Privacy (I-6):** unchanged; see the build table. ADR-014 rev. 2 makes F-22 send the CSP and `Referrer-Policy` on the worker asset.

**Documents against each other.** The spec AP-3, the TD step 4 and Budget row, the F-06 and F-25 feature files, features/README (decisions and Status "in QA, QA r3"), the design v1.4, ADR-013 rev. 2, the architect INDEX (OQ-20 and OQ-21 resolved) and overview v1.8 agree on these points:

- dashed bridges;
- 300 ms trigger;
- MapLibre "about 435 KB gzip, +10 % report trigger";
- F-25 keeps bridges uncoloured.

The exceptions are listed in r3-3 and r3-4.

**Acceptance criteria**

| AC | Code | Automated (run, passing) | Operator (recorded in `MEASUREMENTS.md`) |
| --- | --- | --- | --- |
| 1 MapLibre requested after charts render | `RouteMap.svelte` | `RouteMap.dom.test.ts`; built `index.html` has no MapLibre reference | MD1 pass |
| 2 Chart scrub moves the marker; route hover moves the cursor | `map-view.ts`, `lib/route` | `map-view.dom.test.ts`, `gesture.test.ts`, `route-index.test.ts` | MD3, MP4, MP5 pass. Scrub: MD10 99.67 % ≤ 20 ms, PASS; phone 99.16 % (information) |
| 3 Start, finish and lap markers | `markers.ts`, `map-style.ts` | `markers.test.ts` (golden ride-12h) | MD4 pass |
| 4 Dropout: one connected line with a straight, **dashed** segment | `routeLines`, `wx-route-bridge` | `route.test.ts` (golden gps-dropout bridge), `map-style.test.ts` (bridge layer) | MD5 pass, re-checked after v1.4; MP7 pass (dashed) |
| 5 Indoor: hint, no MapLibre or tile request | `presence.ts`, `RouteMap.svelte` | `presence.test.ts`, `RouteMap.dom.test.ts` | MD2 pass |
| 6 Marker hidden on a sample without position | `map-view.ts` `onCursor` | `map-view.dom.test.ts` | MD5, MD6, MP7 pass |
| 7 Charts usable with slow or blocked tiles | base style first, swallowed errors | `map-view.dom.test.ts`, `RouteMap.dom.test.ts` | MD7, MP8 pass |

**Measurements.**

| Run | Result | Bound | Status |
| --- | --- | --- | --- |
| Phone start-up re-run (Low Power Mode off, 290-300 intervals per 5 s, so not capped at 30 Hz) | longest frame interval 37-93 ms | 300 ms | Trigger not fired; MP1 (a and b) passes |
| Phone first run | 264-379 ms | – | Superseded (probably 30 Hz capped); kept in the record as history, which is right |
| Desktop start-up | 130-217 ms | – | Information only |

**Operator check list.** No changes this round. All MD1-MD12 and MP1-MP10 are recorded as passing.

- The desktop and most phone checks ran on `80c79fa`. MD5, MP1, MP6, MP7 and the network part of MD9 were re-run on v1.4. The other checks are not affected by the v1.4 changes: the scrub path, touch and controls are untouched, and on the desktop's DPR of 1.5 the pixel-ratio cap is a no-op.
- MD9's Referer part (QA r1/r2 addition for worker `.pbf` requests) is not explicitly in the record. Until F-22 the browser default sends at most the origin, and ADR-014 rev. 2 makes the worker-request Referer an F-22 acceptance check, so it moves there.
- Android: not checked (allowed until F-32).

**Not run.** No browser on this machine, so r3-1 is not reproduced. QA made no live call to the tile host.
