# QA F-06 Route map linked to the cursor - round 1

- **Date:** 2026-10-14
- **Verdict:** ready with conditions. No Blocker or Major finding. Every acceptance criterion is traced to code; ACs 2-6 have passing automated tests, while AC 1 (network order), AC 7 and the visual and device parts need the operator's browser checks (MD1-MD12, MP1-MP10, OQ-19). There are four Minor findings and one Question. The most important are QA-06-r1-1 (a failure after `new Map()` leaks the map) and QA-06-r1-2 (the touch-quiet gate treats the browser's `pointercancel` at scroll start as a finger lift).
- **Inputs:** `features/F-06-route-map-linked-to-cursor.md`; `features/README.md` decisions 5 and 28; spec AP-3, AP-6, NFR Performance (60 fps), NFR Privacy; TD Tech stack (Map, Map tiles), Front end (Load order step 4), Security and privacy; `architect/F-06-route-map-linked-to-cursor.md` v1.2 (change log read), ADR-013, ADR-014 rev. 1, ADR-015 rev. 1, ADR-011 rev. 4, ADR-012 rev. 3, ADR-007, ADR-008 rev. 1, ADR-004 item 7, `architect/overview.md` v1.7 (I-6), `architect/INDEX.md`. Code is the uncommitted working tree on top of `19aa4b4`: `src/web/src/lib/route/`, `src/web/src/components/map/`, `src/web/bench/map.*`, `bench/MEASUREMENTS.md`, `package.json` and the lockfile (maplibre-gl 6.14.0), `vite.config.ts`. The installed MapLibre source was read in `node_modules/maplibre-gl/dist/` (6.14.0).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` from the repo root (output captured, exit code recorded) | **exit 0.** `go vet` clean. `npm ci` added 124 packages and found 0 vulnerabilities. It warns `EBADENGINE` because this machine has Node v22.22.2 and `engines` requires >= 24; that is an environment issue, not F-06. `svelte-check`: 245 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: `Test Files 24 passed (24)`, `Tests 601 passed (601)`, which matches the engineer's count. Bundle: `entry index.html: 10613 / 60000 bytes gzip PASS`, unchanged from F-05 r4 (10,613), as the design requires (the counted entry does not change in F-06) |
| Bundle step (a): scratch copy, `main.ts` + `ChartStack` + `createCursorStore` | entry **40,528** gzip |
| Bundle step (b): (a) + `RouteMap` | entry **42,566** gzip. Lazy: `map-view-*.js` 1,056,282 raw / 283,602 gzip / 232,653 br; `map-view-*.css` 83,111 / 10,494 / 8,550. The worker asset `maplibre-gl-worker-*.js` is 507,714 raw, 143,273 gzip (`gzip -9`), 118,664 br |
| Bundle step (c′): (a) + one `import()` of a temporary module with exactly `LngLat, Map, Marker, NavigationControl, setWorkerUrl`, the MapLibre CSS and the `?worker&url` import, each logged | entry 41,311 gzip. That is (a) + 783 for Vite's dynamic-import helper, which agrees with the design's 785. Lazy: JS 280,162, CSS 10,458 gzip |
| Static share (b) − (a) | **2,038** against the 2,100 ceiling. **PASS.** The engineer reported 2,065; the difference comes from the exact form of the temporary `main.ts`. The margin is 35 to 62 bytes |
| Own lazy code (b) − (c′), JS + CSS | (283,602 + 10,494) − (280,162 + 10,458) = **3,476** against 5,000. **PASS.** The engineer reported 3,463 |
| MapLibre total (c′ JS + CSS + worker) | 280,162 + 10,458 + 143,273 = 433,893 by `gzip -9`. The engineer's 434,882 used the size script's zlib, so the figures agree. This is above the 350,000 report line; OQ-20 is with the user |
| `build/web/index.html` of build (b) | Contains only the entry script and its CSS. There is no `modulepreload` and no reference to `map-view` or MapLibre (AC 1 in the build) |
| Production build after `make ci` | `build/web/` contains only `index.html` and one `assets/index-*.js`. No bench file was built: the bench is excluded |
| Mutation probes (scratch copy, `vitest run src/components/map src/lib/route` per mutant) | 23 mutants, 21 killed, **2 survived** (M1, M14; see QA-06-r1-3). Killed: tie removed; tie candidate outside the radius; stale-touch clear removed; NaN check in `onCursor` removed; `_tapDragZoom.disable()` removed; `stopPropagation` removed; Mercator projection not forced; destroy-after-abort removed; quiet re-check after the frame removed; `whenQuiet` before `setStyle` removed; `whenQuiet` before `new Map` removed; cancel without restore; `WORLD_PX` 256; canvas-target check removed; visibility clear removed; base background renamed `wx-`; mouse `buttons` check removed; scroll not invalidating the rect; `error` listener removed; `no-referrer` removed; rect not refreshed on `pointerdown` |
| Failure-path probe (scratch jsdom test, fake `addControl` throws after `new Map`) | `mountRouteMap` rejects, but `maps=1 removed=false children=1 fetchAborted=false` (QA-06-r1-1) |
| Working tree | `git status --porcelain` is identical before and after the review. All probes and builds ran in the scratchpad, and `build/` holds only `make ci`'s own output |

## Findings

### Minor

- **ID:** QA-06-r1-1
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.ts:143-181` (everything between `new MapLibreMap(...)` and the returned handle)
- **Relates to:** F-06 design "Loading and start-up", Failure row (mountRouteMap rejects, "any other throw"); ADR-013 decision 6 ("failure is contained")
- **Evidence:** Only the constructor is inside `try`. If anything after it throws, `mountRouteMap` rejects and leaves the following behind: the live map with its WebGL context, MapLibre's ResizeObserver and window listeners, the inner `div.wx-map`, the still-running style fetch and the `abort` listener on `signal`. Candidates for such a throw are `touchZoomRotate.disableRotation()`, the private `_tapDragZoom.disable()`, `addControl`, `new Marker(...).addTo(map)` and `cursor.subscribe`. Probe: a jsdom test with the recording fake whose `addControl` throws prints `rejected=TypeError maps=1 removed=false children=1 fetchAborted=false`. `RouteMap` then sets `node.textContent`, which detaches the map's DOM but does not call `map.remove()`.
- **Consequence:** The failure text shows, but a WebGL context and its listeners leak for the rest of the page's life, and the style fetch completes for nothing. On iOS, where WebGL contexts are scarce, a leaked context can cost the next map (F-19 data swaps). The most likely trigger is a MapLibre upgrade that renames `_tapDragZoom` (see the Questions section). The fix: move the post-construction set-up into the `try`, or add a second `try` that calls `map.remove()`, `inner.remove()` and `settle()` before rethrowing, plus a test like the probe.

- **ID:** QA-06-r1-2
- **Severity / kind:** Minor / risk (not reproduced: no browser here; the reasoning follows the Pointer Events spec)
- **Route:** architect
- **Where:** `src/web/src/components/map/touch-quiet.ts:188-196`, which follows the F-06 design "Loading and start-up", Touch-quiet row (`pointercancel` ends a touch)
- **Relates to:** D-4, ADR-013 decision 5, F-05 QA note on the chart's non-passive `touchmove`, check MP1
- **Evidence:** The browser fires `pointercancel` as soon as it takes a touch for panning or scrolling. The charts use `touch-action: pan-y`, and MapLibre's canvas uses `pan-x pan-y` with cooperative gestures (verified in `maplibre-gl.css` 6.14.0). After that it sends no more pointer events for that finger, even though the finger is still down and the page is scrolling. `onEnd` therefore empties `active` and starts the 250 ms cool-down about 10 px into every scroll. In a slow drag-scroll the gate opens while the finger is still down. In a quick flick it opens about 100 ms after the lift instead of 250 ms. The design's promise that start-up "never *begins* while a finger is down" holds only for touches that do not scroll, but scrolling touches are the case MP1 is about. `touchstart`/`touchend`/`touchcancel` (passive, on `window`) report the physical lift even after a scroll has started.
- **Consequence:** MapLibre's long tasks can start during or right after a scroll. The next touch on a chart then waits for them, which is the sticky first scroll D-4 was meant to prevent. MP1 measures this, so a real problem would show up there. The design's reasoning, however, is weaker than stated. The architect should either track physical touches with passive touch events or record that `pointercancel` is accepted as "lifted" and why. MP1 should include slow, held drag-scrolls (added to the check list below).

- **ID:** QA-06-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/components/map/testing/maplibre-fake.ts:114-117` (`unproject` ignores its point); `src/web/src/components/map/map-view.ts:195-199`
- **Relates to:** AC 2; ADR-015 rev. 1 ("a touch `down` always uses the 20 px start radius, also when it replaces a stale scrub"); ADR-012 rev. 3
- **Evidence:** Two mutants survive the full map and route suites. **M1:** `xy[0] = e.clientX + rect.left` instead of `-`. The fake's `unproject` returns `recorder.unprojectTo` whatever point it receives, so nothing checks the client-to-map conversion. **M14:** `hitRadiusPx(state, e.pointerType)` without `kind`. `gesture.test.ts` tests `hitRadiusPx` itself, but no test checks that the adapter passes `kind`. With M14 a primary touch down that replaces a stale scrub gets the 48 px drag radius, so a touch 21-48 px from the route would start a scrub and lock scrolling.
- **Consequence:** A regression in the hit-test geometry or in the stale-scrub rule would pass CI and appear only on a device. The fix: have the fake record the point it receives and assert `[clientX − rect.left, clientY − rect.top]` with a mocked rect. Add an adapter test: start a scrub, then send a primary `pointerdown` with a new `pointerId` aimed 30 px from the route, and expect no set and no lock.

- **ID:** QA-06-r1-4
- **Severity / kind:** Minor / deviation from the design text (cost negligible)
- **Route:** architect (amend the text, F-05 precedent) or frontend (make it true)
- **Where:** `src/web/src/lib/route/gesture.ts:179-212`; `src/web/src/components/map/map-view.ts:201-212`
- **Relates to:** F-06 design "Route interaction": "our code allocates nothing per event"
- **Evidence:** `routeStep` returns a new `{ state, action }` object for every pointer event, and a new `{ kind: 'set', index }` for every hit. A new scrub state is allocated on every touch down on the route. `handle` also runs the full hit test (`unproject` plus two grid passes) on every touch `pointermove` in idle and every mouse move with a button pressed, which the reducer then discards. F-05's chart reducer allocates the same way, and F-05 QA accepted that, but F-06's design says the opposite and the engineer did not report the deviation.
- **Consequence:** None measurable now, because this is the input path, not the flush. The doc and the code disagree, though, and the next reviewer cannot tell which one is meant. Either amend the sentence ("one small result object per event, as F-05"), or return preallocated results and skip the hit test when the reducer cannot use it (touch move in idle, mouse move with `buttons !== 0`).

### Question (risk for a later feature)

- **ID:** QA-06-r1-5
- **Severity / kind:** Question / risk for F-22
- **Route:** architect
- **Where:** ADR-014 decision 6 ("Hand-off to F-22 (CSP for pages that load the map)")
- **Relates to:** I-6, TD Security (CSP: "tiles and fonts only from the tile host")
- **Evidence:** MapLibre 6.14.0 creates the worker directly as a module worker from our origin (`workerFactory`: same origin → `new Worker(url, { type: 'module' })`, no blob; read in the source). The vector tiles are fetched inside that worker. Under CSP Level 3 a dedicated worker loaded from an http(s) URL takes its CSP from **its own script response**, not from the page. ADR-014 already says `Referrer-Policy` goes on every response for this reason, but the CSP table is scoped to "pages that load the map".
- **Consequence:** If F-22 sends the CSP only on HTML pages, `connect-src` does not limit what the worker fetches, so I-6 would rest on the code alone for tile requests. The ADR-014 hand-off should state that the CSP must also go on the worker asset response (simplest: the same headers on every response, as for `Referrer-Policy`).

## Previous findings

None. This is round 1.

## Questions

1. **The `_tapDragZoom` internal API (engineer asked for a judgement).** QA judges it acceptable as implemented, on these grounds:
   - In 6.14.0's `maplibre-gl.d.ts` it is a plain field of `TwoFingersTouchZoomRotateHandler`: not `private` and not marked `@internal`. Only the constructor is `@internal`.
   - `svelte-check` in `make ci` therefore fails if an upgrade renames or hides it.
   - The lockfile pins 6.14.0, the fake mirrors it, and the code comment names the version and the reason.
   - No public option switches tap-drag zoom off in 6.14.0. `touchZoomRotate.disable()` would also lose pinch zoom, which AP-3 needs on phones.

   Side effect: after the call, `map.touchZoomRotate.isEnabled()` returns `false` even though pinch zoom works. In 6.14.0 the only reader is the URL-hash feature, which is not used. F-26 must know that `touchZoomRotate.enable()` turns it back on (the code comment says so).

   Two conditions:
   - fix QA-06-r1-1, so that a runtime throw here (for example from a version a type check did not see) cannot leak a map;
   - the architect records the choice in ADR-015 (rev. 1 currently says "any separate tap-drag zoom off" without naming the mechanism).

   *Settled by:* the architect adding one line to ADR-015 or the F-06 design.
2. **QA-06-r1-2.** Should the touch-quiet gate track physical touches (`touchstart`/`touchend`/`touchcancel`, passive) instead of pointer events, or is `pointercancel` = "lifted" accepted? *Settled by:* an architect decision in the F-06 design, or MP1 passing with slow held drag-scrolls (added below).
3. **Static margin.** The static share measures 2,038 to 2,065 against 2,100. F-07 adds the real page wiring (`chartsReady` state, `onmapready`). Does F-07's measurement charge that wiring to F-07, or to F-06's share? *Settled by:* one line in F-07's design.

## Coverage

**Acceptance criteria**

| AC | Code | Automated test (run, passing) | Remaining |
| --- | --- | --- | --- |
| 1 MapLibre requested only after the charts rendered | `RouteMap.svelte` (`load` gate, `import('./map-view')` only, touch-quiet first) | `RouteMap.dom.test.ts`: not called while `load` is false, once after it turns true, touch-quiet delays it. Build (b) `index.html` has no MapLibre reference (QA build) | MD1 (network trace) |
| 2 Chart scrub moves the marker; route hover moves the chart cursor and readout | `map-view.ts` `onCursor`, `handle`/`hitAt`; `lib/route` reducer and grid | `map-view.dom.test.ts` (marker, hover 11 px hit / 13 px miss, buttons, canvas-only, touch scrub and lock, cancel restore), `gesture.test.ts`, `route-index.test.ts` (1,000 seeded queries against an independent brute force on ride-12h; tie and earliest-wins hand cases) | Readout through the shared store (F-05, unchanged); MD3, MP4, MP5. Test gap QA-06-r1-3 |
| 3 Start, finish and lap markers at the correct positions | `markers.ts`, `map-style.ts` layers | `markers.test.ts`: golden ride-12h, 23 lap markers at exactly `start_index`; brick-run-bike across sessions; dropout lap → no marker | MD4 (drawn). Real corpus files: optional operator run (ADR-004 rev. 2) |
| 4 Several-minute dropout: one connected line | `route.ts` (skip NaN, one `LineString`) | `route.test.ts`: golden gps-dropout, 3,301 coordinates, last-before followed by first-after, 300 missing | MD5 (drawn) |
| 5 Indoor: hint, no MapLibre or tile request | `presence.ts`, `RouteMap.svelte` | `presence.test.ts`, `RouteMap.dom.test.ts` (exact text, never mounted, also after `load`) | MD2 (network trace) |
| 6 Cursor on a sample without a position → marker hidden | `map-view.ts:165-180` (NaN test, `visibility`) | `map-view.dom.test.ts` (dropout sample and `null` → hidden, no `setLngLat`); mutant M5 killed | MD5, MD6, MP7 |
| 7 Charts usable while tiles are slow or blocked | Base style first; style fetch `.catch(() => null)`; `error` swallowed; tiles in the worker; charts never wait on the map | `map-view.dom.test.ts` (failed or invalid style → base style, nothing thrown); `RouteMap.dom.test.ts` (import or mount rejection → failure text only) | MD7, MP8 |

**Reviewed and found correct**
- **Module and import rules.**
  - Only `map-view.ts` imports `maplibre-gl` as a value, plus its CSS and `?worker&url`; `map-style.ts` uses `import type` only.
  - Only `RouteMap.svelte` reaches `map-view`, and only through `import()`; the test imports are type-only.
  - `RouteMap` uses only `routePresence` from `lib/route`. `lib/route` imports only `../datafile` types.
  - `lib/route` and `lib/charts` never import each other, and `lib/route` has no top-level calls; the static measurement confirms both.
  - `components/map` has no `style=`/`style:` attributes and no `on…` markup, and no `$cursor` appears anywhere in `components`.
  - `bench/` is never imported by `src/`.
- **Start-up gating.**
  - The gate's listeners are passive capture listeners on `window` and only for pointer events, so they cannot block or delay any scroll. Nothing non-passive is added before mount, and `RouteMap` adds nothing to the charts.
  - `whenQuiet` is awaited before `import()`, before `new Map()` (all synchronous after the await) and before `setStyle`. Each await is covered by a killed mutant.
  - The quiet state is re-checked after the frame, stale primaries are cleared, and `visibilitychange` clears the set.
  - The gate never touches the charts: `load` comes from `onready`, after the first chart render.
- **Remote style.**
  - It is fetched with `credentials: 'omit'`, `referrerPolicy: 'no-referrer'` and an abort signal, and applied once, after both `load` and the JSON, with `transformStyle: mergeStyle`.
  - `mergeStyle` keeps the remote style's non-`wx-` sources untouched (including a raster source), drops the base `background`, puts the `wx-` layers on top, forces Mercator and drops terrain and sky.
  - I read MapLibre 6.14.0's `setStyle`: the diff path (`Style.setState`) calls `transformStyle(this.serialize(), next)`. The fallback `_updateStyle` passes `this.style.serialize()` as `previous`. Both give `mergeStyle` the live `wx-` layers, and every diff operation the merge produces is implemented in 6.14.0.
- **Cursor marker.**
  - One DOM `Marker`, moved with `setLngLat` on a reused `LngLat`; visibility is toggled only on change. There is no `setData`, `triggerRepaint`, `setPaintProperty`, `setFilter`, `setStyle` or `project` call of our own, and no allocation of our own.
  - In 6.14.0, `Marker._update` writes one `transform` and may set `opacity`. It allocates inside MapLibre (`smartWrap`, `project`), which the design accepts, and it never repaints the map.
  - `smartWrap` replaces `_lngLat` with a copy, so mutating `at` creates no aliasing hazard.
- **Nearest-sample grid.**
  - It is correct for clamped cells and ascending order within cells (the early `break` is valid).
  - The cell count is bounded by about 2·⌈M/8⌉ + 2, and queries allocate nothing.
  - The pixel-to-Mercator conversion uses MapLibre's 512 px world.
  - The 3 px tie and earliest-wins rule match decision 5 and survive the tie mutants.
- **Touch scrub lock against MapLibre 6.14.0 source.**
  - The HandlerManager's touch listeners sit on the canvas container without capture. MapLibre has no pointer-event listeners; the only window `touchmove` listener belongs to the compass, which is off.
  - The capture `touchmove` on the outer element therefore hides scrub moves from MapLibre.
  - `doubleClickZoom: false` disables both click and tap zoom (`DoubleClickZoomHandler` wraps both). `TapDragZoom` is off.
  - The cooperative-gestures CSS gives the canvas `touch-action: pan-x pan-y`.
- **Teardown.**
  - `destroy` unsubscribes first and removes every listener with the same options. It also aborts the style fetch, removes the signal listener, the marker, the map and the inner div. It is idempotent.
  - `RouteMap` aborts on unmount, destroys a handle that resolves late, never calls `onmapready` after unmount, and disposes the gate. A `{#key data}` swap destroys and remounts.
  - QA-06-r1-1 is the exception.
- **Failure paths.**
  - A rejected import, a rejected mount and a constructor throw (no WebGL2) show "The map could not be loaded." exactly, inside the reserved box. The error goes to the console through a microtask, and its messages contain no coordinates or tile URLs.
  - A failed or invalid style fetch keeps the base style; tile and style errors are swallowed.
  - A blocked tile host costs only basemap tiles and lap labels (glyphs).
- **Privacy (I-6).**
  - The only third-party host in the lazy chunk's request code is `tiles.openfreemap.org` (our constants). The other URLs in MapLibre are links or error texts (`maplibre.org` attribution and logo, which are off, `github.com`, `wiki.openstreetmap.org`) or a `URL` parse base (`maplibre.invalid`). They are never fetched.
  - There is no `eval`, `new Function` or WebAssembly on the main thread. The worker's `globalThis.eval` is only in `loadScript` (RTL plugin and `importScriptInWorkers`, both unused).
  - The worker is self-hosted (`/assets/maplibre-gl-worker-*.js`, same origin, module worker, no `blob:`).
  - The bench page carries `<meta name="referrer" content="no-referrer">`.
  - Nothing is logged with coordinates. No Liberty JSON and no data derived from real FIT files is addable: the untracked files are the four architect docs and the source, test and bench files listed in `git status`. The only `openmaptiles` strings are in a three-line fake style in a test and in the docs. Fixtures are synthetic (`test-support/golden.mjs`, arrays built in tests).
- **Layout.** The container is reserved at 320 px (via `cssText`) before load, the map is absolutely positioned inside it, and the failure text replaces the map in the same box.
- **Bench.** `bench/map.html` and `map.ts` follow the design and use synthetic files only. `optimizeDeps.include` is dev only.

**What F-22 must do (from ADR-014 and this review)**
- Send `Referrer-Policy: no-referrer` on every response, JS assets and the worker script included.
- Send the CSP: `script-src 'self'`; `worker-src 'self'` (no `blob:`); `connect-src 'self' https://tiles.openfreemap.org`; `img-src 'self' data: blob: https://tiles.openfreemap.org`; `style-src 'self'`. Send it on the worker script response as well (QA-06-r1-5).
- Never allow `'unsafe-eval'` or `blob:` in `script-src`. They are not needed unless someone adds the RTL plugin.

**Not traced or not run**
- No browser on this machine, so every visual, network-trace, touch and timing check is the operator's (below). The `MEASUREMENTS.md` "F-06 map" rows are `pending`. The operator is running them in parallel with this review.
- The live Liberty style was not fetched by QA (no external calls). Its host and absolute-URL guard rests on the architect's 12 Oct verification and MD9/MP10.
- OQ-20 (MapLibre 434,882 bytes gzip against the TD's ~250 KB) is with the user and does not affect the verdict.

**Conditions for closing F-06**
1. The operator runs MD1-MD12 and MP1-MP10 on `bench/map.html` and records the results. MD10, MD11 and MP1 go in `bench/MEASUREMENTS.md`, including "Android not checked" if applicable. MP1 must not fire the 100 ms trigger.
2. QA-06-r1-1 and QA-06-r1-3 are fixed (frontend). QA-06-r1-2, QA-06-r1-4 and QA-06-r1-5 are answered by the architect (a decision or a doc line).
3. QA reviews the record in round 2.

## Operator manual checks (final list)

On `bench/map.html` (`npm run dev`; for the phone, `npm run dev -- --host` on the home network for the duration of the run only). Desktop: Firefox 156 on Linux, window at least 1,200 CSS px wide. Phone: iPhone 13 mini, Orion (iOS WebKit), portrait unless stated. Android: repeat MP1-MP5 if a Chrome-on-Android phone is at hand, otherwise record "Android not checked".

| # | Device | Check | Pass | Covers |
| --- | --- | --- | --- | --- |
| MD1 | Desktop | Open the Network panel and reload with ride-12h | The MapLibre chunk, its CSS and the worker are requested after the page shows charts `onready`. No request goes to `tiles.openfreemap.org` before that | AC 1 |
| MD2 | Desktop | Open indoor-no-gps, then "ride-4h, all positions hidden" | indoor-no-gps: shows "No location data in this workout." and makes no MapLibre, worker or `tiles.openfreemap.org` request at all. All hidden: no map, no hint, no such request | AC 5, OQ-15 default |
| MD3 | Desktop | ride-12h: scrub a chart; hover the route; move off the route; hover where loops overlap | The marker follows the chart cursor. Hovering the route moves every chart's cursor line and the readout. Off the route the cursor stays. Where passes overlap, the readout shows the earliest pass's time | AC 2, D-9 |
| MD4 | Desktop | ride-12h | Green start, red finish, lap markers 2…24. Scrubbing a chart to 0:30:00 puts the marker on lap marker 2 | AC 3 |
| MD5 | Desktop | gps-dropout: look at the line; scrub a chart across 25:00-30:00 | One connected line with a straight segment over the dropout. The marker disappears inside the dropout and reappears after it | AC 4, AC 6 |
| MD6 | Desktop | "ride-4h, start and end hidden" | The line, start and finish are at the visible part. Scrubbing into the hidden parts hides the marker | AC 6, D-7 |
| MD7 | Desktop | Set Tiles to blocked. Then set tiles to normal with the Network panel throttled to "Regular 3G" | Blocked: route and markers on a plain background, charts scrub normally, no repeating errors in the console. Throttled: the charts are usable at once and the map fills in later | AC 7 |
| MD8 | Desktop | Drag the map; use the + and − buttons; plain wheel over the map; Ctrl + wheel; right-drag; double-click | Drag pans; the buttons zoom; a plain wheel scrolls the page (MapLibre's hint may show); Ctrl + wheel zooms; nothing rotates or tilts; double-click does not zoom (v1.2) | D-10 |
| MD9 | Desktop | Expand the attribution; filter the Network panel to third-party hosts | The attribution shows OpenFreeMap, OpenMapTiles and OpenStreetMap, each linked. Every request outside our origin goes to `tiles.openfreemap.org`, with no `Referer` beyond the origin. QA addition: check the `Referer` on a vector-tile (`.pbf`) request too, because the worker makes those | ADR-014, I-6 |
| MD10 | Desktop | Auto-scrub ride-12h with the map in view; record the row | F-05's pass rule, now including the marker (≥ 99 % of intervals ≤ 20 ms, flush p95 ≤ 2 ms, flush max ≤ 8 ms) | NFR 60 fps |
| MD11 | Desktop | Reload ride-12h five times; record the start-up rows | Recorded (information only) | D-4 |
| MD12 | Desktop | Resize the window narrower and wider | The map resizes, nothing jumps, the route stays | Layout |
| MP1 | Phone | Reload ride-12h. As soon as the charts appear, swipe vertically on a chart again and again until the map has tiles. Record the start-up row; reload five times. QA addition (QA-06-r1-2): in at least two of the reloads, use slow drag-scrolls with the finger held down for more than 1 s, as well as flicks | Scrolling never feels stuck. Start-up rows recorded. Trigger: longest interval above 100 ms → the architect revisits D-4 before F-07 | F-05 QA note, D-4 |
| MP2 | Phone | One-finger vertical swipe on the map, away from the route | The page scrolls (MapLibre's "use two fingers" hint may show) | D-10 |
| MP3 | Phone | Two fingers on the map: pan, pinch | The map pans and zooms; the page does not zoom | D-10 |
| MP4 | Phone | Touch down on the route and drag along it, also up and down | The cursor (readout, chart lines, marker) follows; the page does not scroll; no "use two fingers" hint. On lift the cursor stays | D-10, AC 2 |
| MP5 | Phone | Tap the route; tap the map away from the route; tap the route twice quickly; do two short scrubs in quick succession; tap the + button where it lies over the route | Route tap: the cursor jumps there. Elsewhere: nothing changes. The double tap and the quick scrubs never zoom the map. The + button zooms and does not move the cursor | D-10 |
| MP6 | Phone | Scrub a chart on ride-12h with the map in view | The marker follows smoothly (information only; the gate is F-07) | NFR 60 fps |
| MP7 | Phone | gps-dropout: scrub across 25:00-30:00 | The marker disappears in the dropout | AC 6 |
| MP8 | Phone | Set Tiles to blocked | The route shows on a plain background; the charts work | AC 7 |
| MP9 | Phone | Rotate portrait → landscape → portrait | The map resizes with no jump | Layout |
| MP10 | Phone | Set Tiles to normal | Tiles appear. If Orion's content blocker interferes, record it | ADR-014 |
