# QA F-06 Route map linked to the cursor - round 2

- **Date:** 2026-10-14
- **Verdict:** ready with conditions. All five r1 findings are fixed or answered, and no new finding came up in this round. The one remaining condition is the operator's browser and phone record: MD1-MD12 and MP1-MP10, with MD10, MD11 and MP1 written into `bench/MEASUREMENTS.md`. AC 1 (network order), AC 7 and the visual and device parts of the other ACs can only be verified there. Those rows are still `pending`; the operator is running the checks in parallel.
- **Inputs:**
  - Design and decisions: `architect/F-06-route-map-linked-to-cursor.md` v1.3 (change log read), ADR-013 rev. 1, ADR-014 rev. 2, ADR-015 rev. 2, ADR-011 rev. 5, and r1 (`qa/F-06-route-map-linked-to-cursor-r1.md`).
  - Code: the uncommitted working tree on `19aa4b4`, compared file by file with QA's r1 snapshot of `src/web`. Changed since r1: `map-view.ts`, `touch-quiet.ts`, `lib/route/gesture.ts`, `lib/route/index.ts`, `testing/maplibre-fake.ts`, and the tests `map-view.dom.test.ts`, `touch-quiet.dom.test.ts`, `RouteMap.dom.test.ts` and `gesture.test.ts`. Nothing else in `src/web` changed.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (output captured) | **Exit 0.** `svelte-check`: 245 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: **615 passed**, matching the engineer's count (601 in r1, +14). Bundle: `entry index.html: 10613 / 60000 bytes gzip PASS`, unchanged. `npm ci` still warns `EBADENGINE`, because this machine runs Node 22 and the project requires 24; this is an environment issue. |
| Bundle step (a), scratch copy | Entry 40,528 gzip. |
| Bundle step (b) = (a) + `RouteMap` | Entry 42,533 gzip. Lazy `map-view-*.js` 283,636 gzip and `map-view-*.css` 10,494 gzip. Worker 143,273 gzip (`gzip -9`). |
| Bundle step (c′) | Entry 41,311 gzip. Lazy JS 280,162 gzip, CSS 10,458 gzip. |
| Static share (b) − (a) | **2,005** against the 2,100 ceiling: **PASS**. The engineer reported 2,034. ADR-011 rev. 5's fallback ceiling of 2,200 is not needed. Recorded here as ADR-011 rev. 5 asks: the F-06 share stays **2,100** and the reserve stays **1,600**. |
| Own lazy code (b) − (c′) | **3,510** against 5,000: **PASS**. The engineer reported 3,497. |
| `build/web/index.html`, build (b) | No reference to `map-view` or MapLibre and no `modulepreload`. |
| Production build | `build/web/` holds `index.html` and one `index-*.js`; the bench is not built. |
| Mutation probes (scratch copy; each run executes `vitest run src/components/map src/lib/route`) | **42 mutants, 40 killed, 2 survived**, one of them equivalent. The r1 set (21 rerun) is all killed, **including M1** (x offset sign) **and M14** (`kind` not passed to `hitRadiusPx`), plus a new M1b (y offset). Teardown: 6 of 7 killed. Touch-quiet: 7 of 8 killed. `wantsHit`: 6 of 6 killed. The two survivors are listed under Coverage. |
| Partial-mount probe (r1's probe, rerun with the fake throwing in `Map.addControl`, `Marker.addTo`, `Map.on:resize`, `Map.on:load`) | Each case gives `rejected=Error`, `removeCount=1`, `children=0`, `fetchAborted=true`. |
| Working tree | `git status --porcelain` is the same before and after the review, apart from this report and the INDEX row. All builds and probes ran in the scratchpad. |

## Findings

None new.

## Previous findings

| r1 finding | Status | Evidence |
| --- | --- | --- |
| QA-06-r1-1: a throw after `new Map()` leaked the map, listeners and the style fetch (frontend) | **Fixed** | `map-view.ts`: everything after construction now sits in one `try`, and its `catch` calls `teardown()` and rethrows. `teardown()` is shared with `destroy()` and runs in this order: unsubscribe, remove the recorded listeners with their own options, `settle()` (aborts the fetch and removes the signal listener), `marker?.remove()`, `map.remove()`, `inner.remove()`. The r1 probe now prints `removeCount=1 children=0 fetchAborted=true` for four throw sites. The design's own tests ("partial mount", including a throw in `cursor.subscribe`) pass, and mutants T1-T5 and T7 are killed. This matches design v1.3 "Partial mount" and ADR-013 rev. 1 decision 6. |
| QA-06-r1-2: the touch-quiet gate took `pointercancel` as a lift (architect) | **Fixed** | Design v1.3 and ADR-013 rev. 1 decision 5 now count fingers, not pointers. `touch-quiet.ts` follows them: passive capture listeners on `window` for `touchstart`/`touchend`/`touchcancel`, `fingers = e.touches.length`, and a 250 ms cool-down that starts only when the count reaches 0 and is cleared by any new touch. `visibilitychange` to hidden resets the count. Pointer events are no longer used. Because the listeners are passive window-capture listeners, a descendant's `stopPropagation` cannot hide an end from them, and they never delay scrolling. The count corrects itself on the next touch event, so a missed end cannot block the map for good. Tests: "a pointercancel … does not open the gate", one finger still down, touchcancel to 0, a lower count correcting a missed end, pointer events never delay. Mutants Q1, Q2 and Q4-Q8 are killed. |
| QA-06-r1-3: the hit-test geometry and the radius per kind were untested (frontend) | **Fixed** | The fake's `unproject` is now linear in the point it receives and records that point. New tests assert `[clientX − left, clientY − top]`, the rect refresh after a scroll, a primary touch down missing a sample 30 px away (also when it replaces a stale scrub), and a scrub move hitting at 30 px. Mutants M1, M1b and M14 are killed. |
| QA-06-r1-4: the input path did work the design said it didn't (architect or frontend) | **Fixed** | Design v1.3 replaces the sentence with F-05's rule: one reducer result object per event is accepted, while the flush and `touchmove` still allocate nothing. The new pure `wantsHit` is true exactly for the inputs whose reducer row reads `hit`. I checked it row by row against `routeStep`: a mouse or pen move with no button, a primary touch down, a move of the scrubbing pointer. The adapter skips `unproject` and the grid query otherwise. Tests include an agreement test against `routeStep` and an adapter test with no `unproject`. Mutants W1-W6 are killed. |
| QA-06-r1-5: worker CSP and referrer policy for F-22 (architect) | **Answered** | ADR-014 rev. 2 decision 6: a dedicated worker takes its CSP and referrer policy from its own script response, so F-22 sends both headers on the worker asset (simplest: on every response). F-22's acceptance check includes a worker tile request without `Referer` and a worker response that carries the CSP. |
| r1 Q1: the `_tapDragZoom` internal API | **Answered** | ADR-015 rev. 2 names the mechanism and gives the reasons: a public, non-`@internal` field in the 6.14.0 `.d.ts`; `svelte-check` catches a rename; the lockfile pins the version. It also records the F-26 caveat that `touchZoomRotate.enable()` re-enables it. The code comment matches. The test asserts both `doubleClickZoom: false` and the `_tapDragZoom.disable()` call (M6 killed). QA-06-r1-1 removes the leak risk this question depended on. |
| r1 Q2: pointer events vs touch events in the gate | **Answered** (see r1-2) | |
| r1 Q3: who pays for F-07's page wiring in the bundle | **Open, not blocking** | No document answers it yet. It belongs in F-07's design. The static margin is now 95 bytes by QA's measurement (2,005 of 2,100) and 66 by the engineer's. |

## Questions

1. (Carried from r1 Q3, for the architect in F-07's design.) Is F-07's real page wiring (`chartsReady` state, `onmapready`) charged to F-07 or to F-06's 2,100? *Settled by:* one line in the F-07 design.

## Coverage

**Acceptance criteria.** Unchanged from r1, and every row is still traced. The r1 code paths for ACs 2-6 and their tests still pass. The new tests strengthen AC 2: real coordinate mapping and the radius per kind.

| AC | Automated (run, passing) | Operator |
| --- | --- | --- |
| 1 | `RouteMap.dom.test.ts` (no mount before `load`; touch-quiet delays it); build (b) `index.html` has no MapLibre reference | MD1 |
| 2 | `map-view.dom.test.ts` (marker, hover, geometry, radius per kind, touch scrub and lock), `gesture.test.ts`, `route-index.test.ts` | MD3, MP4, MP5 |
| 3 | `markers.test.ts` (golden ride-12h, 23 lap markers) | MD4 |
| 4 | `route.test.ts` (golden gps-dropout, 3,301 coordinates) | MD5 |
| 5 | `presence.test.ts`, `RouteMap.dom.test.ts` | MD2 |
| 6 | `map-view.dom.test.ts` (NaN and `null` hide the marker) | MD5, MD6, MP7 |
| 7 | `map-view.dom.test.ts` (failed or invalid style keeps the base style), `RouteMap.dom.test.ts` (failure text) | MD7, MP8 |

**Re-checked quickly and found correct**
- **Module rules.** Only `map-view.ts` imports `maplibre-gl` as a value. `map-style.ts` and one test use `import type` only. Only `RouteMap.svelte` reaches `map-view`, through `import()`. `RouteMap` uses only `routePresence` from `lib/route`. `lib/route` and `lib/charts` do not import each other. There are no `$cursor`, `style=` or `on…` markup handlers in `components/map`, and the bench is not imported by `src/`.
- **Privacy (I-6).** The lazy chunk and the worker reference the same URLs as in r1. The only fetched third-party host is `tiles.openfreemap.org`; the rest are links, error texts or a URL parse base. The style fetch still uses `credentials: 'omit'` and `no-referrer` (M22 killed). The worker is still self-hosted as a same-origin module worker. Nothing addable contains Liberty JSON or data from real FIT files.
- **Hot paths.** `onCursor` and `onTouchMove` are textually unchanged apart from moving inside the `try` (`marker` is now the `const m`). They still allocate nothing of their own, and there is no `setData` and no repaint.
- **Teardown ordering.** It matches design v1.3. `destroy()` is guarded by `destroyed`, and `applyRemote` checks `destroyed` after its `await`.

**Surviving mutants**
- **T6 (low value).** Removing the `if (!destroyed)` guard in `destroy()` survives. The double-destroy test checks only "does not throw", and the fake's `remove()` is idempotent; a `removeCount === 1` check after two `destroy()` calls would kill it. This is not a finding: the guard is present, and `teardown` sets `destroyed` first.
- **Q3 (equivalent).** Not clearing the cool-down on `touchstart` survives, because `quiet()` also requires `fingers === 0` and the next end restarts the timer.

**Doc precision (preference, no action required).** ADR-013 decision 5 and the gate's comment say "mouse and pen never delay anything". On iPadOS an Apple Pencil fires touch events, so a pencil on the screen does delay start-up, which is harmless. This does not affect the operator's iPhone.

**Not run.** There is no browser on this machine, so the operator's checks below cover the browser and phone parts.

## Operator manual checks: changes since r1

Only MP1 changes, because of the new touch-quiet behaviour: start-up now waits while any finger is on the screen, scrolling or not, and for 250 ms after the last one lifts. MD1-MD12 and MP2-MP10 are unchanged from the r1 list, which is design v1.3's list plus the QA addition to MD9: check the `Referer` on a worker `.pbf` tile request.

| # | Device | Check | Pass | Covers |
| --- | --- | --- | --- | --- |
| MP1 (r2) | Phone | (a) Reload ride-12h. As soon as the charts appear, swipe vertically on a chart again and again until the map has tiles: flicks, plus at least two slow drag-scrolls with the finger held for more than 1 s. Record the start-up row, and reload five times. (b) New: reload, and as soon as the charts appear, rest one finger still on the page for about 3 s, then lift. | (a) Scrolling never feels stuck. Start-up rows recorded. Trigger: a longest interval above 100 ms means the architect revisits D-4 before F-07. (b) The page's `onmapready` time stays "–" while the finger is down, and the map appears only after the lift (≥ 250 ms later). | F-05 QA note, D-4, ADR-013 rev. 1 decision 5 |
