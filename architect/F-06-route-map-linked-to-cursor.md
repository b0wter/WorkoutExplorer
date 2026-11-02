# F-06 Route map linked to the cursor: architecture

- **Version:** 1.6
- **Status:** accepted. Built and closed out (14 Oct 2026): phone start-up 37–93 ms, under the 300 ms trigger (OQ-21); MD1–MD12 and MP1–MP10 pass
- **History:** [architect/history/F-06-route-map-linked-to-cursor.md](history/F-06-route-map-linked-to-cursor.md) (revision list, answered questions)
- **Date:** 12 Oct 2026
- **Feature:** [`features/F-06-route-map-linked-to-cursor.md`](../features/F-06-route-map-linked-to-cursor.md)
- **Depends on:** TD (system-wide picture), [F-04 design](F-04-workout-data-file-reader.md) v1.1 (`WorkoutData`, `position`), [F-05 design](F-05-stacked-charts-with-shared-cursor.md) v1.4 (cursor store, `onready`, hot-path rules D-10, scroll lock D-18, dev page), [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-007](ADR-007-decoded-workout-data-in-the-browser.md) rev. 1, [ADR-008](ADR-008-shared-cursor-store.md) rev. 1, [ADR-010](ADR-010-touch-gestures-on-charts.md) rev. 2, [ADR-011](ADR-011-activity-page-bundle-allocation.md) rev. 5, [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md) rev. 3, [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md), [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md) rev. 2, [ADR-015](ADR-015-map-interaction-hover-touch-nearest-sample.md) rev. 2
- **Requirements:** AP-3, AP-6 (map side), NFR Performance (60 fps, LCP), NFR Privacy (no tracking, store as little as possible), INF-1 (attribution, what the tile host sees); TD Tech stack (Map, Map tiles), TD Front end (Load order step 4, Implementation notes), TD Security and privacy (CSP, Referrer-Policy, GDPR); review decisions 5 (self-crossing route: nearest sample, earliest on a tie) and 28 (dark map style, F-14)
- **Owner:** frontend (senior-frontend-engineer)
- **Supersedes:** none

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | Three new modules: `lib/route` (pure route geometry, nearest-sample index, map gesture reducer), `components/map/RouteMap.svelte` plus `touch-quiet.ts` (static glue), and the lazy adapter `components/map/map-view.ts` plus `map-style.ts`. Only `map-view.ts` imports `maplibre-gl` as a value, and only `RouteMap` reaches it, through `import()` | Module boundary; ADR-013 |
| D-2 | MapLibre GL JS **v6** (ESM, WebGL2), worker bundled and served from our origin (`?worker&url` + `setWorkerUrl`), so CSP needs no `blob:` | ADR-013 |
| D-3 | `RouteMap` decides from `routePresence(data)`: `no-location-data` → the hint "No location data in this workout.", MapLibre never loaded; `no-visible-route` → nothing rendered, nothing loaded (OQ-15); `route` → a container of reserved height, loaded once the page sets `load` | Route presence |
| D-4 | Start-up runs only in a **touch-quiet** moment: no finger on the screen and none lifted in the last 250 ms (v1.3: counted from touch events, not pointer events). Checked before `import()`, before `new Map()` and before the remote style is applied. Answers the F-05 QA note on the non-passive chart `touchmove`. (v1.4) A single start-up pause of up to 300 ms per page load on the phone is accepted (user, OQ-21); measured after the v1.4 changes: 37–93 ms | Loading and start-up; ADR-013 |
| D-5 | **Base style first:** the map starts with a local style (background, route, markers), so the route draws with no tile host. The OpenFreeMap style JSON is fetched by us and applied after the map's `load`, merged with our layers through `transformStyle`. Blocked or slow tiles leave the route on a plain background; the charts never wait | Style and layers; ADR-013 |
| D-6 | Tiles: OpenFreeMap **liberty** (`https://tiles.openfreemap.org/styles/liberty`, user decision on OQ-18); no credentials, no referrer, no third party other than `tiles.openfreemap.org`; attribution through MapLibre's control | ADR-014 rev. 1 |
| D-7 | The route connects every positioned sample in index order (AP-3). (v1.4, user request) It is drawn as **recorded segments** (solid) and **bridges** (dashed). A bridge is the straight segment between two consecutive positioned samples with a GPS dropout or a gap (≥ 5 s) between them. (v1.5) One source `wx-route` with two features (recorded, bridge) and their layers, which refines the TD's "one GeoJSON line" (ADR-013 decision 8). Start and finish at the first and last positioned sample. Lap marker *k* (k ≥ 2) at lap *k*'s `start_index`, only if that sample has a position | Route, markers |
| D-8 | The cursor marker is **one MapLibre `Marker` (DOM element)**, moved with `setLngLat` in the cursor listener, hidden (`visibility`) when the cursor is `null` or its sample has no position. No `setData`, no WebGL repaint while scrubbing | Cursor marker; ADR-013 |
| D-9 | Hover and touch on the route set the cursor to the **nearest positioned sample within a pixel radius**; samples within `TIE_PX` = 3 CSS px of the nearest count as a tie, and the **earliest** wins (decision 5). A uniform grid over Web Mercator coordinates answers it without allocation | Route interaction; ADR-015 |
| D-10 | Touch: **cooperative gestures** (one finger scrolls the page, two fingers move and zoom the map). A one-finger touch that **starts on the route** (≤ 20 CSS px) scrubs the cursor for the rest of that touch and locks page scrolling, like the chart lock (ADR-010). Desktop: hover sets the cursor; drag pans; Ctrl/⌘ + wheel or the buttons zoom. No rotation, no pitch | Route interaction; ADR-015; OQ-14 |
| D-11 | Hot path: the cursor listener allocates nothing of its own; MapLibre's few short-lived objects inside `setLngLat` are accepted like F-05's transform strings (D-10 there) | Performance |
| D-12 | Bundle: static share **2,100** bytes gzip (ADR-011 rev. 4; v1.0–1.1: 1,800). It includes Vite's dynamic-import helper (785 measured), a one-time cost per entry that F-19, F-20 and F-28 reuse and do not pay again. Own lazy code ≤ **5,000**; MapLibre JS, CSS and worker reported, not limited (TD: "as shipped") | Bundle impact |
| D-13 | Dev page `bench/map.html`: map above charts, all checks and both measurements (scrubbing with the map, start-up frame gaps), with a "tiles blocked" switch | Dev page |

## Boundary with neighbouring features

| Not in F-06 | F-06 provides instead | Owner |
| --- | --- | --- |
| Prototype page, its layout, the phone gate, settling the tie rule with the metric definitions | `RouteMap` with `load` and `onmapready`; wiring below; `bench/map.html` | F-07 |
| Page layout, map height, server-rendered hint and height reservation | `height` prop (default 320 CSS px); the hint text; `routePresence` for F-13's client part | F-13 |
| Dark map style and themed colours (decision 28) | `styleUrl` option; every colour and the style URL as constants in `map-style.ts`; `mergeStyle` keeps our layers across a style swap | F-14 |
| Start and end hiding (server) | Markers and the route use positioned samples only, so hidden sections draw nothing; the all-hidden case is OQ-15 | F-16 |
| Range highlight on the map | `wx-` layer naming and the merge rule; F-17 adds its own source and layer through the handle | F-17 |
| CSP and `Referrer-Policy` headers | The required sources and policy (ADR-014 "Hand-off to F-22") | F-22 |
| Route colouring | The route source `wx-route` (recorded segments and bridges as separate features, v1.4) and layer `wx-route-line`; F-25 colours recorded segments only; `wx-route-bridge` stays dashed and uncoloured, because nothing was recorded along a bridge (user decision, 14 Oct 2026; recorded in the F-25 feature file) | F-25 |
| Privacy zones (phase 2, SH-3) | A zone removes positions mid-route, which would become a bridge straight across the zone. SH-3's design must decide whether bridges over zones are drawn at all | phase 2 |
| Fullscreen map | The handle; F-26 turns cooperative gestures off in fullscreen | F-26 |
| About page wording and attribution list | What the tile host sees (ADR-014); OQ-17 | F-29 |
| Auto-pan to keep the marker in view | Not done: the map stays where the viewer left it | — |

## Module boundary

```
src/web/
├── src/lib/route/                 pure TypeScript, no Svelte, no DOM, no MapLibre; imports only lib/datafile (index.ts)
│   ├── index.ts                   public surface
│   ├── presence.ts                routePresence (the only part static code uses; imports nothing else from lib/route)
│   ├── route.ts                   buildRoute, routeLines (v1.4; replaces routeCoordinates)
│   ├── markers.ts                 routeMarkers
│   ├── mercator.ts                mercatorX, mercatorY
│   ├── route-index.ts             createRouteIndex (grid), nearest
│   ├── gesture.ts                 map gesture reducer, hitRadiusPx, interceptsTouch, constants
│   └── *.test.ts
├── src/components/map/
│   ├── RouteMap.svelte            STATIC. Presence decision, hint, container, touch-quiet, import('./map-view'), failure text
│   ├── touch-quiet.ts             STATIC. createTouchQuiet(window): whenQuiet(), dispose()
│   ├── map-view.ts                LAZY. The only value importer of 'maplibre-gl'; mountRouteMap()
│   ├── map-view.css               LAZY. Cursor-marker style
│   ├── map-style.ts               LAZY. baseStyle(), mergeStyle(), layer specs, colours, style URL; `import type` from maplibre-gl only
│   ├── map-style.test.ts          Node
│   └── *.dom.test.ts              jsdom (ADR-012 rev. 3)
└── bench/
    ├── map.html, map.ts           dev-only (not a build entry)
    ├── map-props.svelte.ts        dev-only (v1.2): `$state` holder for the props the page swaps; imported by map.ts only
    └── MEASUREMENTS.md            gains "F-06 map"
```

**Dependency rules** (in TD Code structure, Dependency rules):

| Module | May import |
| --- | --- |
| `lib/route` | `lib/datafile` (its `index.ts`; types and constants) |
| `components/map/RouteMap.svelte` | `svelte`, `lib/route` (only `routePresence`), `lib/cursor` and `lib/datafile` (types only), `./touch-quiet`, and `./map-view` **only** through `import('./map-view')` |
| `components/map/map-view.ts` | `maplibre-gl`, `maplibre-gl/dist/maplibre-gl.css`, `maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url`, `./map-style`, `./map-view.css`, `lib/route`, `lib/cursor`, `lib/datafile` (each via `index.ts`) |
| `components/map/map-style.ts` | `lib/route` (types), `import type` from `maplibre-gl` |
| `bench/map.ts` | the above and `components/charts`; never imported by `src/` |

- Nothing outside `components/map` imports `maplibre-gl`. Nothing imports `map-view.ts` or `map-style.ts` statically except each other and their tests.
- Files in `lib/route` have no top-level side effects (no module-level `new`, no calls), so the bundler can keep everything but `presence.ts` out of the entry chunk. QA checks this through the static measurement.
- `lib/route` and `lib/charts` never import each other. Map and charts meet only in the cursor store (ADR-008).
- No `style=`/`style:` attributes and no `on…` markup handlers in `components/map` (same rule and reasons as F-05: Svelte's style runtime, F-01 D-8 a11y warnings). Heights are written with `style.cssText`, listeners with `addEventListener`.

## Wiring (for F-07 and the dev page)

```svelte
<script lang="ts">
  let chartsReady = $state(false);
</script>
<RouteMap {data} {cursor} load={chartsReady} />
<ChartStack {data} {cursor} onready={() => (chartsReady = true)} />
```

`RouteMap` props:

| Prop | Type | Meaning |
| --- | --- | --- |
| `data` | `WorkoutData` | One map per `data` value: everything is inside `{#key data}`; a new value destroys the old map and builds a new one |
| `cursor` | `CursorStore` | Same store as the charts. `RouteMap` checks `cursor.sampleCount === data.sampleCount` in `$effect.pre` and throws otherwise (as `ChartStack`) |
| `load` | `boolean` | `false` until the page wants the map. MapLibre is requested only after it becomes `true` (AC 1). Never unloads when it turns `false` again |
| `height` | `number?` | Container height in CSS px, default `MAP_HEIGHT` = 320, read once per `data` value |
| `styleUrl` | `string?` | Remote style; default liberty (ADR-014 rev. 1). F-14 passes the dark style; the dev page passes a blocked URL |
| `onmapready` | `() => void`? | Called once per `data` value, after the map exists and the marker has subscribed to the cursor |

## Route presence (AP-3)

```ts
export type RoutePresence = 'no-location-data' | 'no-visible-route' | 'route';
/** no-location-data: data.position is absent. no-visible-route: present, but no sample has a position.
 *  route: at least one positioned sample. One pass over latitude; stops at the first finite value. */
export function routePresence(data: WorkoutData): RoutePresence;
```

| Presence | `RouteMap` renders | Network |
| --- | --- | --- |
| `no-location-data` (indoor, pool) | `<p class="map-hint">No location data in this workout.</p>` (exact text, AC 5) | No MapLibre, no worker, no tile host request, ever |
| `no-visible-route` (route entirely hidden, SH-4) | Nothing: no container, no hint, no warning (feature scope) | None. Default; product question OQ-15 |
| `route` | `div.route-map` with the reserved height | MapLibre after `load` |

## Loading and start-up (D-4, ADR-013)

```
page: ChartStack.onready ──▶ load = true
RouteMap (static):  presence === 'route' && load
  1. await whenQuiet()
  2. import('./map-view')                        ← network; Vite loads the JS chunk(s) and the CSS
  3. mountRouteMap({ container, data, cursor, whenQuiet, styleUrl, signal })
map-view (lazy):
  4. start fetch(styleUrl) (no main-thread cost; referrerPolicy 'no-referrer', credentials 'omit', signal)
  5. await whenQuiet()
  6. route = buildRoute(data); index = createRouteIndex(route); lines = routeLines(data, route); markers = routeMarkers(data, route)   (a few ms)
  6a. (v1.4) await whenQuiet()                    ← our work and MapLibre's never share one task
  7. setWorkerUrl(workerUrl); map = new Map({ container: inner, style: baseStyle(lines, markers), bounds, pixelRatio, validateStyle, ... })   ← MapLibre's long task(s)
  8. marker = new Marker({ element }).addTo(map); unsubscribe = cursor.subscribe(onCursor); add input listeners
  9. return handle                                 → RouteMap calls onmapready
 10. when the map has fired 'load' AND the style JSON has arrived: await whenQuiet(); map.setStyle(json, { diff: false, transformStyle: mergeStyle })

(v1.4) User Timing, for the start-up measurement (no data, no network; harmless in production):
  performance.mark('wx-map:imported') right after the import resolves (RouteMap);
  performance.measure('wx-map:route-model', …) around step 6; 'wx-map:new-map' around the `new Map()` call;
  'wx-map:set-style' around the `setStyle` call; performance.mark('wx-map:load') on the map's `load`;
  performance.mark('wx-map:idle') on the first `idle` after the remote style was applied.
```

| Rule | Detail |
| --- | --- |
| Touch-quiet | `createTouchQuiet(window)` is created by `RouteMap` at mount, so it sees touches from the start. (v1.3, QA-06-r1-2) It tracks **fingers, not pointers**, with passive capture listeners on `window` for `touchstart`, `touchend` and `touchcancel`. The finger count is `e.touches.length` of the latest of these events, so it corrects itself on every touch event and a missed end can never block the map for good. When the count reaches 0, the end time is recorded. Pointer events are not used: browsers send `pointercancel` as soon as a scroll starts, while the finger is still down and scrolling, and that is exactly the touch the gate must wait for. `whenQuiet()` resolves when the count is 0 and the last end is ≥ `QUIET_MS` = 250 ms ago, then after one `requestAnimationFrame` plus a `setTimeout(0)`, so the work starts right after a frame. `visibilitychange` to hidden sets the count to 0. Mouse and pen never delay anything (they fire no touch events). The listeners are passive, so they never delay scrolling. `dispose()` removes them with the same options |
| What it protects | A touch that starts on a chart while the main thread runs MapLibre's start-up waits for that task (the chart's non-passive `touchmove`, F-05 D-18). The gate makes start-up never *begin* while a finger is down or right after one lifted. It cannot stop a touch that starts *during* a long task; the measurement below bounds that |
| Not gated | The browser's evaluation of the fetched chunk (it runs when the network delivers it), and work MapLibre schedules itself after `new Map()` (style parse, shader compilation, tile uploads). Both are measured |
| Cursor before the map | The store works without the map. The marker subscribes in step 8 and gets the current index at once (ADR-008), so a late map is in sync |
| Before load | The container shows a flat neutral background (`#e8eaed` until F-14) at its reserved height, so nothing moves when the map appears. No spinner, no text. Touches there scroll the page (no listeners yet) |
| Failure | If the `import()` rejects, or `mountRouteMap` rejects (no WebGL2: `GPUInitializationError`; any other throw), the container shows "The map could not be loaded." (exact text, user decision on OQ-16) and nothing else. No retry. The charts are untouched. The error is rethrown in a microtask (console only), never with coordinates or tile URLs |
| Partial mount (v1.3, QA-06-r1-1) | Once `new Map()` has returned, any throw before `mountRouteMap` resolves runs the same cleanup as `destroy()` for whatever exists so far, in this order: unsubscribe from the cursor if subscribed; remove the listeners added so far; abort the style fetch; `map.remove()`. Then the error is rethrown, so `RouteMap` shows the failure text. A failed mount never leaves a WebGL context, a listener or a pending fetch behind |
| Unmount | `RouteMap` aborts (`AbortController` passed as `signal`): a pending import or mount that resolves after unmount is destroyed at once and `onmapready` is not called. `handle.destroy()` unsubscribes from the cursor first, removes every listener with the same options it was added with, aborts the style fetch and calls `map.remove()` |
| Tile and style errors | One `map.on('error', …)` listener swallows them (no console flood while offline, no tile URLs, which encode the area, in the console). A failed style fetch leaves the base style; a failed tile leaves its square blank |

`mountRouteMap` signature:

```ts
export interface MountOptions {
  readonly container: HTMLElement;      // RouteMap's div.route-map; map-view creates MapLibre's container inside it
  readonly data: WorkoutData;
  readonly cursor: CursorStore;
  readonly whenQuiet: () => Promise<void>;
  readonly styleUrl?: string;           // default OPENFREEMAP_STYLE_URL (map-style.ts)
  readonly signal: AbortSignal;
}
export interface RouteMapHandle { destroy(): void }
export function mountRouteMap(options: MountOptions): Promise<RouteMapHandle>;
```

F-14, F-17 and F-26 extend `RouteMapHandle` in their designs (style swap, range layer, gesture mode); F-06 needs only `destroy`.

## Route and markers (D-7)

`lib/route`:

```ts
export interface Route {
  /** Sample indices with a position, ascending; length M ≥ 1. */
  readonly indices: Uint32Array;
  /** Web Mercator x and y in [0, 1] per positioned sample, same order as indices. */
  readonly mx: Float64Array;
  readonly my: Float64Array;
  /** [west, south, east, north] in degrees over the positioned samples. */
  readonly bounds: readonly [number, number, number, number];
}
/** null unless routePresence(data) === 'route'. */
export function buildRoute(data: WorkoutData): Route | null;
/** v1.4. The route split into recorded segments and bridges, coordinates [longitude, latitude] (GeoJSON order).
 *  Every pair of consecutive positioned samples (indices[k], indices[k+1]) is either an edge of exactly one
 *  recorded segment or exactly one bridge. */
export interface RouteLines {
  /** Maximal runs of positioned samples with no bridge between neighbours; only runs of ≥ 2 points. */
  readonly recorded: [number, number][][];
  /** One 2-point line per bridge. */
  readonly bridges: [[number, number], [number, number]][];
}
export function routeLines(data: WorkoutData, route: Route): RouteLines;

export interface RouteMarker {
  readonly kind: 'start' | 'finish' | 'lap';
  readonly index: number;               // sample index
  readonly lon: number;
  readonly lat: number;
  readonly lap?: number;                // 1-based lap number, kind 'lap' only
}
export function routeMarkers(data: WorkoutData, route: Route): RouteMarker[];

/** Web Mercator, unit square: x = (lon + 180) / 360; y = (1 − ln(tan(π/4 + φ/2)) / π) / 2, φ clamped to ±85.051129°. */
export function mercatorX(lon: number): number;
export function mercatorY(lat: number): number;
```

| Rule | Detail |
| --- | --- |
| Line (v1.4) | Consecutive positioned samples a = `indices[k]`, b = `indices[k+1]` are joined by a **bridge** if b − a > 1 (at least one sample without position between them: a GPS dropout) **or** b is in `data.gaps` (a gap of ≥ 5 s with no samples: a pause, or a recording drop). Otherwise they are an edge of a **recorded segment**. One walk over `indices` with a second pointer over `gaps`. A positioned sample with bridges on both sides belongs to no recorded segment (a one-point line is invalid GeoJSON) and is just the joint of the two bridges. Hidden ends never make a bridge (no positioned sample beyond them). The line is never filled and never broken: recorded segments and bridges together connect every positioned sample (AP-3, AC 4) |
| Why gaps too | The user's concern is that a straight segment looks like recorded track. Across a gap nothing was recorded either. After a café stop both ends lie together and the dash is invisible; after a recording drop while moving it shows what is missing |
| Start | The first positioned sample (`indices[0]`). After hiding (SH-4) this is the first visible point |
| Finish | The last positioned sample. Drawn above the start when they coincide (loops, M = 1) |
| Lap markers | For each lap *k* ≥ 2 (1-based, metadata order) with `start_index < N` and a position at `start_index`: a marker labelled *k* (the lap that begins there, matching the laps table, F-18). No marker when that sample has no position (dropout or hidden section, decision 14), never a substitute point (AC 6's rule: no marker at a wrong point). Lap 1 begins at the start marker |
| Sessions | One line across all sessions (AP-12 is phase 2); no session markers |
| Precision | `Float64Array`s throughout; never `Float32Array` (ADR-007: 10⁻⁶° needs float64) |
| Antimeridian | A route crossing ±180° draws across the world. Known limitation, not handled |

**Map layers** (`map-style.ts`, all ids prefixed `wx-`; colours are constants there, placeholders until F-14):

| Order (bottom → top) | Id | Type | Source | Look |
| --- | --- | --- | --- | --- |
| 0 | `wx-route-bridge` (v1.4) | line | `wx-route`, `kind == 'bridge'` | `#c2187a`, 2.5 px, opacity 0.85, `line-dasharray: [2, 1.5]` (in line widths), butt caps, no casing. Below the recorded line, so solid ends cover the dash ends |
| 1 | `wx-route-casing` | line | `wx-route`, `kind == 'recorded'` | white, 6 px, round joins and caps |
| 2 | `wx-route-line` | line | `wx-route`, `kind == 'recorded'` | `#c2187a` (magenta), 3 px. v1.1: liberty draws major roads in orange and yellow and water in blue, so the route uses a hue liberty does not use |

(v1.4) Source `wx-route` is a `FeatureCollection` of at most two features: one `MultiLineString` with `kind: 'recorded'` and one with `kind: 'bridge'`, each present only if non-empty. `line-dasharray` is not data-driven here, so the kinds are separate layers selected by filter.
| 3 | `wx-lap-circle` | circle | `wx-markers`, `kind == 'lap'` | radius 6, white fill, dark stroke |
| 4 | `wx-lap-label` | symbol | `wx-markers`, `kind == 'lap'` | `text-field` = lap number, 10 px, font `Noto Sans Regular` from the tile host's glyphs (liberty uses it, verified 12 Oct 2026); overlapping labels hide (default collision) |
| 5 | `wx-start` | circle | `wx-markers`, `kind == 'start'` | radius 7, green, white stroke |
| 6 | `wx-finish` | circle | `wx-markers`, `kind == 'finish'` | radius 7, red, white stroke |

The cursor marker is not a layer (D-8).

## Style and layers (D-5, ADR-013, ADR-014)

```ts
export const OPENFREEMAP_STYLE_URL = 'https://tiles.openfreemap.org/styles/liberty';
export const TILE_HOST = 'https://tiles.openfreemap.org';
export const GLYPHS_URL = 'https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf';
/** Local style: a background layer with id 'background' (deliberately not wx-: mergeStyle must drop it so the
 *  remote style's own background shows, v1.2), glyphs URL, sources wx-route and wx-markers (inline GeoJSON),
 *  the wx- layers above. Pure. */
export function baseStyle(lines: RouteLines, markers: readonly RouteMarker[]): StyleSpecification;   // v1.4: RouteLines
/** transformStyle callback: next's sources and layers, then every wx- source and layer from previous on top;
 *  projection forced to mercator; terrain and sky removed. Idempotent: a wx- layer never appears twice. Pure. */
export function mergeStyle(previous: StyleSpecification | undefined, next: StyleSpecification): StyleSpecification;
```

| Rule | Why |
| --- | --- |
| The map is created with `baseStyle`; the remote style is applied only after the map's `load` event | A diff or a `transformStyle` call against a style that has not finished loading can lose our layers |
| (v1.4) `setStyle(json, { diff: false, transformStyle: mergeStyle })` | A diff from the small base style to liberty's 111 layers fails anyway and falls back to a full rebuild. `diff: false` skips the attempt (serialising and diffing both styles on the main thread) |
| We fetch the remote style JSON ourselves (`fetch(url, { credentials: 'omit', referrerPolicy: 'no-referrer', signal })`) and pass the object to `setStyle` | We decide *when* it is applied (touch-quiet), a failure leaves the base style, and our request carries no referrer. Relative URLs would break when the style is passed as an object. On 12 Oct 2026 liberty had none (see "Liberty, as verified" below). If the style at implementation time has a relative URL or a host other than `TILE_HOST`, stop and report |
| `mergeStyle` keeps liberty's sources as they are, including the raster source `ne2_shaded` | Removing or rewriting the remote style's sources is not ours to do; it would also break F-14's dark style. The only exceptions are projection, terrain and sky (next row) |
| Mercator forced, no terrain | The hit test (D-9) assumes screen distance ∝ Mercator distance (pitch 0, mercator). A future remote style must not switch on globe or terrain |
| `glyphs` in the base style | A symbol layer needs it. If the tile host is blocked, lap labels are missing and lap circles still show |

**Liberty, as verified (12 Oct 2026, v1.1).** Fetched from `https://tiles.openfreemap.org/styles/liberty` and the TileJSON it references:

| Item | Value | Consequence |
| --- | --- | --- |
| `sprite` | `https://tiles.openfreemap.org/sprites/ofm_f384/ofm` (absolute, versioned path) | Fetched by MapLibre → `connect-src`; images → `img-src` (ADR-014 rev. 1) |
| `glyphs` | `https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf`, the same as `GLYPHS_URL` | Fontstacks used: Noto Sans Regular, Italic, Bold, so up to three glyph sets per character range |
| Source `openmaptiles` | vector, `url: https://tiles.openfreemap.org/planet` (TileJSON: tiles `https://tiles.openfreemap.org/planet/<build>/{z}/{x}/{y}.pbf`, zoom 0–14) | As expected |
| Source `ne2_shaded` | **raster**, `tiles: https://tiles.openfreemap.org/natural_earth/ne2sr/{z}/{x}/{y}.png`, no attribution (Natural Earth, public domain) | PNG tile requests at low zoom, same host; a raster shader at start-up |
| `projection`, `terrain`, `sky`, `light` | not present | `mergeStyle` still forces Mercator and drops terrain and sky, for future versions |
| Other hosts | none fetched. The TileJSON attribution *links* to `openfreemap.org`, `openmaptiles.org` and `openstreetmap.org`; links are not requests | I-6 holds |
| Relative URLs | none | The stop rule above stays as a guard |
| Layers | 111 | More start-up and pan work than positron; see Performance |

The JSON is not committed (it changes with the tile host's builds, and the engineer has no reason to pin it). The guard is the stop rule above. MD9, MD11 and MP10 check it in the browser.

**Map options** (`new Map({...})`):

| Option | Value | Why |
| --- | --- | --- |
| `container` | an inner `div` created by `map-view` inside `div.route-map` | Our capture listeners sit on the outer element, above all of MapLibre's (D-10) |
| `style` | `baseStyle(...)` | D-5 |
| `bounds`, `fitBoundsOptions` | `route.bounds`, `{ padding: 32, maxZoom: 16 }`; if west = east and south = north: `center` that point, `zoom: 15` | Initial view fits the route (feature scope) |
| `cooperativeGestures` | `true` | D-10, ADR-015 |
| `dragRotate`, `pitchWithRotate`, `touchPitch` | `false`; `touchZoomRotate.disableRotation()` after construction; `maxPitch: 0` | No rotation or tilt; keeps the hit-test assumption |
| `doubleClickZoom` | `false` (v1.2), and (v1.3) `map.touchZoomRotate._tapDragZoom.disable()` after construction: MapLibre 6.14.0's one-finger double-tap-and-drag zoom, a public field in its `.d.ts` despite the underscore (ADR-015 rev. 2) | MapLibre counts a route scrub shorter than 500 ms as a tap, so two quick scrubs or taps on the route zoomed the map (engineer's MP4/MP5 observation). The route is a tap target, so double-tap must not zoom. Zoom stays available through the buttons, pinch and Ctrl/⌘ + wheel. Desktop loses double-click zoom (accepted) |
| `attributionControl` | `{ compact: true }` | ADR-014 |
| `maplibreLogo` | `false` (default) | Nothing extra |
| `renderWorldCopies` | `false` | One copy of the route |
| `pixelRatio` (v1.4) | `Math.min(devicePixelRatio, MAX_MAP_PIXEL_RATIO)`, `MAX_MAP_PIXEL_RATIO` = 2 (constant in `map-style.ts`) | The iPhone has dpr 3: a 3× canvas has 2.25 times the pixels of 2× for every frame, first render included, with little visible gain on a map. F-26 may raise it in fullscreen. **Known limitation (QA-06-r4-1, owner F-07):** the ratio is fixed at mount, so after a desktop zoom or a move to another monitor the map stays blurry or over-sampled until reload (phones unaffected). Suggested fix in F-07: on a `devicePixelRatio` change, call `map.setPixelRatio(Math.min(devicePixelRatio, MAX_MAP_PIXEL_RATIO))`. F-06 has only 44–71 bytes of static headroom left (2,029–2,056 of 2,100), which F-07's design must account for if the fix touches static code. **(v1.6) Resolved by F-07 design D-8:** `mapPixelRatio()` in `map-style.ts`, and a re-armed `matchMedia` resolution listener in `map-view.ts` that calls `setPixelRatio` on a change. Lazy only, so 0 static bytes |
| `validateStyle` (v1.4) | `import.meta.env.DEV` (true in the dev server and Vitest, false in the build) | Style-spec validation of liberty's 111 layers runs on the main thread at every style application. The tile host's style and our tested base style need no runtime check in production; the dev page still shows validation errors in the console |
| Controls | `NavigationControl({ showCompass: false })` top right | Zoom buttons (AP-3 pan and zoom on every device) |

## Cursor marker (D-8, D-11)

- One `HTMLDivElement` (class `wx-cursor-marker`, styled in `map-view.css`: 14 px circle, white 2 px border, dark fill, `pointer-events: none`), wrapped in one `new Marker({ element, anchor: 'center', subpixelPositioning: true })`, added once. Created with `visibility: hidden`.
- The cursor listener:

```ts
const onCursor = (i: CursorIndex): void => {
  if (i === null || lat[i] !== lat[i]) { if (shown) { element.style.visibility = 'hidden'; shown = false; } return; }
  at.lng = lon[i]; at.lat = lat[i];                     // one LngLat created at mount, reused
  marker.setLngLat(at);
  if (!shown) { element.style.visibility = 'visible'; shown = true; }
};
```

- `lat`/`lon` are `data.position.latitude`/`longitude` (checked once at mount). `lat[i] !== lat[i]` is the `NaN` test (ADR-007: latitude and longitude are missing together).
- Hidden for `null`, a dropout sample and a hidden-section sample (AC 6). Never placed at a neighbouring point.
- MapLibre re-projects the marker itself when the map moves.

## Route interaction (D-9, D-10, ADR-015)

**Nearest sample.** `lib/route/route-index.ts`:

```ts
export interface RouteIndex {
  /** Sample index of the nearest positioned sample to Mercator point (x, y) within radius; samples within
   *  tie of the nearest distance count as tied and the lowest sample index wins; −1 if none within radius.
   *  radius, tie in Mercator units (> 0). No allocation. */
  nearest(x: number, y: number, radius: number, tie: number): number;
}
export function createRouteIndex(route: Route): RouteIndex;
```

- Grid over `route`'s Mercator bounding box with about 8 samples per cell (`cells = ceil(M / 8)`, square cells, at least 1), stored as two `Uint32Array`s (cell start offsets, items). A counting sort fills it, so each cell lists its samples in ascending index order. Built once per map in O(M).
- Query: visit the cells overlapping `[x ± radius] × [y ± radius]`, take the minimum squared distance d², then the lowest index among candidates with distance ≤ √d² + tie and ≤ radius. Two passes over the same cells; no allocation.
- Pixel to Mercator: with pitch 0 and Mercator, 1 CSS px = 1 / (512 · 2^zoom) Mercator units (MapLibre's world size). The adapter converts `radiusPx` and `TIE_PX` with `map.getZoom()` and the pointer with `map.unproject` → `mercatorX/Y`.
- `TIE_PX` = 3: two passes of an out-and-back or a repeated loop that lie within 3 CSS px of each other are a tie, so hovering there always gives the earlier pass instead of flickering between passes. This is decision 5's "earliest on a tie" applied to what a viewer can tell apart. F-07 settles the metric definitions; a change is local to this constant.

**Gesture reducer** (`lib/route/gesture.ts`, pure). State `idle` or `scrub { pointerId, before }`. Input `{ kind: 'down' | 'move' | 'up' | 'cancel' | 'leave', pointerType, pointerId, isPrimary, buttons, hit, current }`; `hit` is `nearest(…)` with the radius from `hitRadiusPx(state, pointerType, kind)`, or −1; `current` = `cursor.get()`. Output `{ state, action: none | set(index) }`.

| Pointer | Input | Action | Next state |
| --- | --- | --- | --- |
| mouse, pen | move with `buttons === 0`, hit ≥ 0 | set(hit) | idle |
| mouse, pen | anything else (moves while dragging the map, down, up, leave, hit −1) | none (cursor stays, as F-05 D-8) | idle |
| touch | down, primary, hit ≥ 0 (within 20 px) | set(hit) | scrub { pointerId, before: current } |
| touch | down, primary, hit −1 | none: the touch belongs to the page and MapLibre | idle |
| touch | move, same pointerId, in scrub, hit ≥ 0 (within 48 px) | set(hit) | scrub |
| touch | move, same pointerId, in scrub, hit −1 | none (cursor stays) | scrub |
| touch | up, same pointerId | none (the cursor stays where the finger was) | idle |
| touch | cancel, same pointerId | set(before) (`null` too) | idle |
| touch | down, **not** primary, while in scrub (second finger) | none | idle: the gesture goes to MapLibre (two-finger pan or pinch) |
| touch | down, primary, other pointerId, while in scrub (stale touch) | as a primary down in idle | per that row |
| touch | anything else | none | unchanged |

```ts
export const HOVER_RADIUS_PX = 12;        // mouse, pen
export const TOUCH_START_RADIUS_PX = 20;  // a touch must start this close to the route to scrub
export const TOUCH_DRAG_RADIUS_PX = 48;   // during a scrub
export const TIE_PX = 3;
/** v1.2: kind defaults to 'move'. A touch 'down' always gets TOUCH_START_RADIUS_PX, also in scrub state
 *  (a primary down replacing a stale scrub must start like a new touch); a touch move in scrub gets
 *  TOUCH_DRAG_RADIUS_PX; mouse and pen get HOVER_RADIUS_PX. */
export function hitRadiusPx(state: RouteGestureState, pointerType: string, kind?: RouteGestureInputKind): number;
/** True only while one finger scrubs: state scrub and touchCount === 1. */
export function interceptsTouch(state: RouteGestureState, touchCount: number): boolean;
```

**Listeners** (`map-view.ts`, on the outer `div.route-map`, added once at mount, removed in `destroy` with the same options):

| Listener | Options | Does |
| --- | --- | --- |
| `pointerenter`, `pointerdown`, `pointermove`, `pointerup`, `pointercancel`, `pointerleave` | `{ passive: true, capture: true }` | (v1.3) Hit test only if `wantsHit(…)`, then reducer → `cursor.set`. Never `preventDefault`, never `stopPropagation` |
| `touchmove` | `{ passive: false, capture: true }` | `if (interceptsTouch(state, e.touches.length)) { if (e.cancelable) e.preventDefault(); e.stopPropagation(); }` — the route-scrub lock: the page does not scroll, and MapLibre never sees the moves of that touch, so it neither pans nor shows its two-finger message |
| `scroll` on `window` | `{ passive: true, capture: true }` | Marks the cached container rect stale |

- (v1.2) **Hits only on the canvas.** A `down` or a mouse/pen `move` whose `event.target` is not MapLibre's canvas (`map.getCanvas()`), for example a zoom button, the attribution or the cursor marker's container, gets `hit` = −1. Zoom buttons and attribution over the route therefore never move the cursor, and a touch starting on a control never scrubs. During a scrub, touch moves keep the hit test (implicit capture keeps the target at the start element anyway).
- Pointer to map point: `x = clientX − rect.left`, `y = clientY − rect.top` with a cached rect of the inner container (refreshed on `pointerenter`, `pointerdown`, after MapLibre's `resize` event; stale after `scroll`). One reused `[x, y]` array is passed to `map.unproject`.
- `touchstart` and `touchend` are never stopped, so MapLibre's touch state stays consistent; a second finger reaches MapLibre with both touches known.
- **Verify in the installed MapLibre source** which events its touch-pan, cooperative-gesture and tap handlers use, and where they listen. If any of them uses pointer events or listens on the outer element itself, so that stopping `touchmove` in capture on the outer element does not hide a scrub's moves from MapLibre, stop and report. Say in a code comment which events and version you checked.
- Like the chart lock, the `touchmove` handler allocates nothing, reads no layout and never calls `set`.
- (v1.3, QA-06-r1-4; replaces "our code allocates nothing per event") The input path follows F-05's rule: it runs per pointer event, not per flush. The reducer's result object (one small object per event) and MapLibre's `LngLat` from `unproject` are accepted. The cursor listener (the flush) and the `touchmove` handler still allocate nothing.
- (v1.3) **No hit test the reducer would discard.** `lib/route/gesture.ts` exports a pure `wantsHit(state, kind, pointerType, pointerId, isPrimary, buttons): boolean`. It is true exactly for the inputs whose table row reads `hit`:
  - a mouse or pen `move` with `buttons === 0`;
  - a primary touch `down`;
  - a touch `move` of the scrubbing `pointerId`.

  For everything else the adapter passes `hit` = −1 without calling `unproject` or `nearest`, so touch moves of a page scroll over the map and mouse drags of the map cost no grid query.

## Performance (NFR: 60 fps; F-05 QA note)

| Path | Cost | Rule |
| --- | --- | --- |
| Cursor flush (scrubbing a chart) | + one `setLngLat`: one projection and one `transform` write by MapLibre. No WebGL repaint (a DOM marker), no worker round trip | D-11. QA greps the listener: no `setData`, `triggerRepaint`, `setPaintProperty`, `setFilter`, `setStyle`, `project` calls of our own, no allocation |
| Hover / route scrub input | `unproject` + one grid query (two passes over a few cells; a full scan of 43,200 samples only when zoomed out so far that the whole route is within 12 px) + `cursor.set` | Input path; the store coalesces to one flush per frame |
| Map pan and zoom | MapLibre's own rendering | Not on the scrubbing path |
| Start-up | Chunk evaluation, `new Map()`, first render, remote style, tile uploads | Gated by touch-quiet (D-4); measured below |
| Liberty's weight (v1.1) | 111 layers, a raster hillshade source, three fontstacks: more shader programs and style parsing when the remote style is applied, more tile and glyph requests, more work per pan frame than positron would cost. The base style (route first) is unaffected, and so is scrubbing: the DOM marker triggers no repaint | The style application is gated by touch-quiet and falls inside the start-up window that MD11 and MP1 measure. If the trigger fires, the first remedy in ADR-013 still applies; switching back to positron is a user decision, not an engineering fallback |

The chart `touchmove` lock (F-05 D-18) makes a touch that starts on a chart wait for the main thread. MapLibre's own non-passive touch listeners do the same for touches that start on the map, with or without F-06's lock. The touch-quiet gate keeps start-up from beginning during or just after a touch; the start-up measurement and check MP1 test the result.

## Dev page `bench/map.html` (D-13)

Dev-only like `bench/charts.html`: served by `npm run dev`, not a build entry, never imported by `src/`. Add `maplibre-gl` to `optimizeDeps.include` in `vite.config.ts`, so the dev server does not reload the page the first time the map is imported. The page carries `<meta name="referrer" content="no-referrer">` (ADR-014).

- Layout: `RouteMap` above `ChartStack`, wired as in "Wiring"; a spacer below, so the page scrolls.
- Files (synthetic only, `?url` imports): `ride-12h` (24 laps of 30 min; the heading turns once in about 94 min, so the route loops about 7 times and passes lie close to or across each other), `ride-4h`, `gps-dropout` (5 min without positions at 25:00–30:00), `pause-hours`, `brick-run-bike`, `no-distance-with-gps`, `indoor-no-gps` (hint), and two variants built in the page: **"ride-4h, start and end hidden"** (positions `NaN` for the first and last 300 samples) and **"ride-4h, all positions hidden"** (every position `NaN`).
- **Tiles** switch: normal, or blocked (`styleUrl` = `https://tiles.invalid/styles/liberty`; `.invalid` never resolves).
- Timestamps shown: charts `onready`, `onmapready` (ms since navigation start).
- **Auto-scrub 10 s**: F-05's procedure, unchanged, with the map visible above the charts. Flush timing: listener A subscribed before `ChartStack` (as in F-05), listener B subscribed in `onmapready`, so the measured flush covers charts, readout and marker. The button is enabled after `onmapready`.
- **Start-up window**: a `requestAnimationFrame` monitor (preallocated buffer) from charts `onready` to 5 s later: longest frame interval, number of intervals > 50 ms, time to `onmapready`.
- (v1.4) **Frame-rate baseline**: right after the 5 s window, the monitor runs 2 more seconds with nothing happening and records the median frame interval. The page shows "**30 Hz cap: the run does not count**" when that median is ≥ 25 ms. iOS Low Power Mode caps `requestAnimationFrame` at 30 Hz in WebKit, and the phone run (124–137 intervals in 5 s, about 37 ms average) looks capped.
- (v1.4) **Step table**: after the window, the page lists every `wx-map:*` mark and measure (start relative to charts `onready`, duration) and, for every frame interval > 50 ms, the measures that overlap it, plus "import evaluation" = the `wx-map:imported` mark minus the `responseEnd` of the MapLibre chunk's Resource Timing entry. The operator copies this block with the row.
- "second subscriber" line as in F-05.

**Record:** new section "F-06 map" in `bench/MEASUREMENTS.md`:

| Table | Columns | Pass or trigger |
| --- | --- | --- |
| Scrubbing with the map | F-05's columns plus "Map visible" | Desktop: F-05's pass rule (≥ 99 % of intervals ≤ 20 ms, flush p95 ≤ 2 ms, max ≤ 8 ms) now including the marker. Phone: information only (the gate is F-07) |
| Map start-up | Date, Commit, Device, OS, Browser, File, Tiles, Low Power Mode, Baseline median interval ms, Time to `onmapready` ms, Longest frame interval ms, Intervals > 50 ms, Step at the longest interval | (v1.4, ADR-013 rev. 2) **Trigger:** check MP1 fails, **or** the longest interval on the phone (Low Power Mode off, no 30 Hz cap) exceeds **300 ms** (user, OQ-21, 14 Oct 2026; the v1.0 bound of 100 ms was an assumption). Re-run after v1.4 (phone, Low Power Mode off): 37–93 ms, MP1 passes, so the trigger did not fire |

## Bundle impact (ADR-002, ADR-011 rev. 4)

- **The counted entry does not change in F-06**: `main.ts` still mounts the placeholder; the page that imports `RouteMap` is F-07. QA checks the CI size line is unchanged.
- **Measured now**, as in F-05, with temporary imports in `main.ts`, then reverted:
  - (a) `ChartStack` and `createCursorStore` only;
  - (b) (a) plus `RouteMap`;
  - (c′) (a) plus, behind **one** dynamic import of a temporary module, exactly the named imports `map-view.ts` takes from `maplibre-gl`, the MapLibre CSS and the `?worker&url` import, each used (for example logged) so nothing is dropped. (v1.2: replaces (c), a bare `import('maplibre-gl')`. That kept all of MapLibre, while `map-view` lets the bundler drop unused parts, so (b) − (c) came out negative.) When `map-view`'s MapLibre imports change, (c′) changes with them.
- **Static ceiling: (b) − (a) ≤ 2,100** bytes gzip (ADR-011 rev. 4). It contains `RouteMap`, `touch-quiet`, `routePresence`, the hint and Vite's dynamic-import helper (`__vitePreload` with the lazy CSS list, 785 measured). `build.modulePreload: false` does not remove the helper, because the helper also loads the lazy chunk's CSS. The helper is loaded once per entry and reused by F-19, F-20 and F-28. Measured at implementation: 2,066, of which F-06's own code is about 1,281. Above 2,100: stop and report. (v1.3) QA r1 measured 2,038–2,065. The r1 fixes touch static code only in `touch-quiet` (other event names, a count instead of a set), which should cost about the same. If they cross 2,100, the ceiling is **2,200** for them (ADR-011 rev. 5, up to 100 bytes from the reserve). The engineer reports the exact number, and QA r2 records it. Above 2,200: stop and report. Do not trim readability to stay under.
- **Lazy chunks** (from the size script's "dynamic imports" lines in build (b)): report every file, raw / gzip / brotli. The worker from `?worker&url` is an asset, not an import, so the script does not list it; report its sizes from `build/web/assets/` by hand.
- **Own lazy code: lazy JS and CSS of (b) − lazy JS and CSS of (c′) ≤ 5,000** bytes gzip (worker excluded from both) (`map-view`, `map-style`, `lib/route` minus `presence`, `map-view.css`). Above it: stop and report.
- **MapLibre** (main chunk, CSS, worker): no limit (TD: "as shipped, roughly 250 KB"). If its gzip total is above **350,000**, report it; the architect then decides on the TD figure with the user. (v1.2) Measured with 6.14.0: **434,882** (JS 280,175, CSS 10,458, worker 144,249). Reported to the user with a recommendation (INDEX OQ-20). Until the user decides, nothing changes in the code.
- Check: `build/web/index.html` (build (b)) contains no reference to a MapLibre file (no `modulepreload` of it), which is AC 1 in the build.

**New runtime dependency:** `maplibre-gl`, latest 6.x at implementation (6.14.0 when this was written; caret range, lockfile pins it). BSD-3-Clause; its bundled dependencies are BSD/ISC/MIT and F-29's generated list picks them up. Already allowed by TD Code structure, Dependency rules ("MapLibre, dynamic import").

## Tests

All Vitest, synthetic data only (ADR-004 item 7): arrays built in the test (seeded PRNG where random), `lib/datafile/testing/build-file.ts`, or golden synthetic files through `test-support/golden.mjs`.

| File | Must cover |
| --- | --- |
| `lib/route/presence.test.ts` | No `position` → `no-location-data`; position all `NaN` → `no-visible-route`; one finite sample anywhere (first, middle, last) → `route`. Golden: `indoor-no-gps` → `no-location-data`; `gps-dropout`, `ride-12h` → `route` |
| `lib/route/route.test.ts` | `buildRoute`: `indices` ascending and exactly the positioned samples; `mx`/`my` equal `mercatorX/Y` of each; `bounds` equal brute-force min/max; `null` for the two other presences. (v1.4, replaces the `routeCoordinates` checks) `routeLines`: `[lon, lat]` order (a test where lat ≠ lon would catch a swap), no `NaN`; **partition property** on seeded random data (random missing runs, random gaps): every consecutive positioned pair is exactly one recorded edge or exactly one bridge, and a pair is a bridge iff b − a > 1 or b ∈ `gaps`; every recorded segment has ≥ 2 points. **Golden `gps-dropout` (AC 4):** exactly one bridge, from sample 1,499 to sample 1,800 (the dropout is 1,500–1,799), and two recorded segments of 1,500 and 1,801 points. **Golden `pause-hours`:** exactly one bridge, across the gap (from the last sample before the 3-hour gap to the first after it). **Golden `ride-12h`:** no bridge, one recorded segment of 43,200 points. Hand cases: one positioned sample between two missing runs → two bridges and no recorded segment containing it; two adjacent positioned samples across a gap → one bridge, no recorded segment; hidden ends → no bridge at either end; M = 1 → no segment and no bridge; M = 2 adjacent → one recorded segment |
| `lib/route/markers.test.ts` | Start = first positioned sample, finish = last. **Golden `ride-12h` (AC 3):** lap markers for laps 2…n, each at exactly `latitude`/`longitude[start_index]`, `lap` = its number, count = laps − 1 (every boundary has a position). A lap whose `start_index` sample has no position → no marker; `start_index === N` (empty last lap) → no marker; `brick-run-bike`: markers across sessions. M = 1: start and finish at the same point |
| `lib/route/mercator.test.ts` | (0, 0) → (0.5, 0.5); lon −180 / 180 → 0 / 1; lat ±85.051129 → about 1 / 0; above it clamped; strictly monotonic in lon and decreasing in lat |
| `lib/route/route-index.test.ts` | 1,000 seeded random queries (positions, radii, tie) on `ride-12h`'s positions (repeated loops) equal a brute-force reference implementing the same rule. Hand cases: nothing within radius → −1; a query exactly at a sample → that sample; **two samples at identical coordinates → the lower index** (decision 5); a later sample nearer by less than `tie` → the earlier one; nearer by more than `tie` → the nearer one; a candidate within `tie` of the best but outside `radius` → never returned; samples without position never returned (sample indices, not route positions, are returned); M = 1; zero-area bounding box (all samples at one point); a radius covering the whole route |
| `lib/route/gesture.test.ts` | Every row of the reducer table; `hitRadiusPx` per state, pointer type and kind (v1.2: touch `down` in scrub state → 20, touch `move` in scrub → 48, default kind `move`); (v1.3) `wantsHit` true exactly for the three cases in "Route interaction" and false for every other row (touch move in idle, touch move of another pointer, mouse move with buttons, up, cancel, leave, non-primary down); `interceptsTouch` true only for scrub with one touch (false for idle, 0 or 2 touches); cancel restores `before` including `null`; a second finger ends the scrub; mouse never enters scrub |
| `components/map/map-style.test.ts` (Node, `import type` only) | `baseStyle`: version 8, `glyphs` = `GLYPHS_URL`, sources `wx-route` (absent when there is neither a recorded segment nor a bridge) and `wx-markers`, layer ids and order as in the table. (v1.4) `wx-route` holds a `recorded` feature only when there is a recorded segment and a `bridge` feature only when there is a bridge; `wx-route-bridge` filters `kind == 'bridge'` and has `line-dasharray` [2, 1.5]; casing and line filter `kind == 'recorded'`; the bridge layer lies below the casing. `MAX_MAP_PIXEL_RATIO` is 2. `mergeStyle`: next's layers first, then every `wx-` layer of previous in its order; `wx-` sources added; next's own sources unchanged, including a raster source like liberty's `ne2_shaded` (v1.1); the base style's `background` layer is not carried over, and next's background stays the bottom layer (v1.2); `projection` mercator; `terrain` and `sky` removed; applying it twice yields no duplicate `wx-` layer or source; `previous` undefined → next unchanged except projection and terrain |
| `components/map/map-style.validate.test.ts` (v1.5, Node, QA-06-r3-2; user-approved) | `@maplibre/maplibre-gl-style-spec` becomes a **direct devDependency**. Pin the exact version that the locked `maplibre-gl` 6.14.0 depends on, so validator and renderer agree. Its `validateStyleMin` (or `validate`, whichever the package exports for a style object) is run on:<br>(1) `baseStyle` for ride-12h, gps-dropout (recorded and bridge features), M = 1 (no route source) and a route with no lap markers;<br>(2) `mergeStyle(baseStyle(…), next)` with a small synthetic "remote" style written in the test (a vector source with a fill and a symbol layer, a raster source, a background, `projection`/`terrain` keys present). The synthetic style stands in for liberty, which is not committed.<br>Each result must be an empty error list. This replaces the runtime validation that production skips (`validateStyle` is dev-only, ADR-013 decision 7). **Rule for later features:** every feature that adds or changes `wx-` sources or layers (F-14 dark style, F-17 range layer, F-25 route colouring, F-26) adds its styles to this test |
| `components/map/touch-quiet.dom.test.ts` (jsdom, fake timers, stubbed rAF; touch events as plain `Event`s with a `touches` property, ADR-012 rev. 2) | (v1.3) Resolves without delay beyond rAF + timeout when nothing happened; stays pending after `touchstart` (1 touch); **a `pointercancel` (scroll started) does not open the gate**; a `touchend` with `touches.length` 1 (one finger still down) keeps it pending; it resolves only 250 ms after the event that brings the count to 0 (`touchend` or `touchcancel`); a later `touchstart` with a lower count corrects a missed end; pointer events of any type never delay; `visibilitychange` to hidden clears; `dispose` removes every listener with the same capture flag (match target, type, listener and capture, as F-05's cleanup test) |
| `components/map/RouteMap.dom.test.ts` (jsdom, `vi.mock('./map-view')`) | No position → the exact hint text and `mountRouteMap` never called, also after `load` = true (AC 5). All positions hidden → nothing rendered, never called. Route → container with `style.height` = 320 (or `height`) before `load`; **not called while `load` is false; called exactly once after `load` = true** (AC 1); receives the container, data and cursor. `load` back to false does not destroy. Import or mount rejection → the failure text, no throw from the component. Unmount before the mount resolves → `destroy` called once, `onmapready` never. Unmount after → `destroy` once. `onmapready` once. Wiring check throws on a sample-count mismatch. A new `data` value (test-only wrapper, ADR-012 rev. 2) → old handle destroyed, new mount |
| `components/map/map-view.dom.test.ts` (jsdom, `vi.mock('maplibre-gl')` with the fake of ADR-012 rev. 3; the worker URL import mocked) | `setWorkerUrl` called before `new Map`; Map options as in the table (`cooperativeGestures`, no rotation or pitch, `bounds` from `buildRoute`, attribution compact); `whenQuiet` awaited before `new Map` and before `setStyle`; `setStyle` only after the fake fires `load` and the style fetch resolves, with `transformStyle` = `mergeStyle`; style fetch rejected → no `setStyle`, nothing thrown; fetch called with `referrerPolicy: 'no-referrer'` and `credentials: 'omit'`. **Marker (AC 2, AC 6):** a cursor flush to a positioned sample → `setLngLat` with (lon, lat) of that sample and `visibility: visible`; to a `NaN` sample or `null` → `visibility: hidden` and no `setLngLat`. **Hover (AC 2):** a mouse `pointermove` whose fake `unproject` lands within 12 px of a sample → `cursor.set` with that index; farther → no set; with buttons pressed → no set, and (v1.3) `unproject` not called. **Coordinates (v1.3, QA-06-r1-3):** the fake `unproject` maps the point it receives linearly to a longitude and latitude (no constant result); with the inner container's rect stubbed at a non-zero `left`/`top`, a pointer at `clientX`/`clientY` is passed as `[clientX − left, clientY − top]` and selects the sample the linear mapping places there; after a `scroll` event and a moved rect, the next `pointerenter` or `pointerdown` uses the new rect. **Radius per kind (v1.3):** with a sample 30 px from the pointer, a primary touch `down` (also one replacing a stale scrub) does not hit it (20 px), while a touch `move` of the active scrub does (48 px). **Partial mount (v1.3, QA-06-r1-1):** the fake throws from `Marker.addTo` (and, separately, from `cursor.subscribe`) → `mountRouteMap` rejects, `map.remove()` called once, the style fetch's signal aborted, no listener left on the outer element or `window`, no cursor subscription left. **Zoom handlers (v1.3):** `doubleClickZoom: false` in the options and `touchZoomRotate._tapDragZoom.disable()` called. **Route touch:** touch `pointerdown` on the route → set; a following cancelable `touchmove` with one touch is `defaultPrevented` and does not reach a listener on the inner container; touch down off the route → not prevented and reaches it; two touches → not prevented; non-cancelable → `preventDefault` not called (spy). Error events are swallowed. **`destroy`:** unsubscribed (a later flush does not touch the marker), every `addEventListener` matched by a `removeEventListener` with the same capture flag, `map.remove()` called, style fetch aborted |

**Not covered by automated tests** (no WebGL, no layout, no real touch, no network in jsdom; verified on `bench/map.html`, by measurement and by QA review):

| What | How |
| --- | --- |
| The map draws, fits the route, markers in the right places, a straight segment over the dropout (AC 3, AC 4) | MD3, MD4, MD5 |
| Network order and absence of requests (AC 1, AC 5) | MD1, MD2 (Firefox Network panel); RouteMap jsdom test for the order |
| Hover and touch on the route against the real MapLibre (AC 2) | MD3, MP4, MP5 |
| Tiles blocked or slow (AC 7) | MD7, MP8 |
| Cooperative gestures, page scroll vs map | MD8, MP2, MP3 |
| 60 fps with the map; start-up stalls (F-05 QA note) | Measurements above; MP1, MP6 |
| Hot-path rules | Review of `onCursor` and the `touchmove` handler; `grep -rn '\$cursor' src/web/src/components` stays empty |

## Manual checks (operator)

On `bench/map.html` (`npm run dev`; phone over the home network with `npm run dev -- --host`, F-04 "Phone access"). Desktop: Firefox 156 on Linux, window ≥ 1,200 CSS px. Phone: iPhone 13 mini, Orion (iOS WebKit), portrait unless stated, (v1.4) **Low Power Mode off** (the battery icon is not yellow); a run that the page flags as 30 Hz capped is repeated. If the cap remains with Low Power Mode off, repeat MP1's measurement once in Safari and record both; the cap is then Orion's. **Android:** if a Chrome-on-Android phone is at hand, repeat MP1–MP5; otherwise record "Android not checked". F-07's gate is on the operator's phone only, so Android stays open until F-32 (NFR Compatibility).

| # | Device | Check | Pass | Covers |
| --- | --- | --- | --- | --- |
| MD1 | Desktop | Network panel open, reload with ride-12h | The MapLibre chunk(s), its CSS and the worker are requested after the page shows charts `onready`. No request to `tiles.openfreemap.org` before that | AC 1 |
| MD2 | Desktop | indoor-no-gps; then "ride-4h, all positions hidden" | indoor: "No location data in this workout." and no MapLibre file, worker or `tiles.openfreemap.org` request at all. All hidden: no map, no hint, no such request | AC 5, OQ-15 default |
| MD3 | Desktop | ride-12h: scrub a chart; hover the route; move off the route; hover where loops overlap | The marker follows the chart cursor. Hovering the route moves every chart's cursor line and the readout. Off the route the cursor stays. Where passes overlap, the readout shows the earliest pass's time | AC 2, D-9 |
| MD4 | Desktop | ride-12h | Green start, red finish, lap markers 2…24. Scrub a chart to 0:30:00: the marker sits on lap marker 2 | AC 3 |
| MD5 | Desktop | gps-dropout: look at the line; scrub a chart across 25:00–30:00. (v1.4) Also pause-hours, zoomed in on the stop | One connected line; (v1.4) the straight segment over the dropout is **dashed**, the rest solid, the solid ends cover the dash ends. The marker disappears inside the dropout and reappears after it. pause-hours: at most a tiny dashed piece at the stop | AC 4, AC 6, D-7 |
| MD6 | Desktop | "ride-4h, start and end hidden" | The line, start and finish are at the visible part; scrubbing into the hidden parts hides the marker | AC 6, D-7 |
| MD7 | Desktop | Tiles: blocked. Then tiles normal with the Network panel throttled to "Regular 3G" | Blocked: route and markers on a plain background; charts scrub normally; no repeating errors in the console. Throttled: charts usable at once; the map fills in later | AC 7 |
| MD8 | Desktop | Drag the map; + and − buttons; plain wheel over the map; Ctrl + wheel; right-drag | Drag pans; buttons zoom; plain wheel scrolls the page (MapLibre's hint may show); Ctrl + wheel zooms; nothing rotates or tilts | D-10 |
| MD9 | Desktop | Attribution; Network panel filtered to third-party hosts | Expanding the attribution shows OpenFreeMap, OpenMapTiles and OpenStreetMap, each linked (liberty's TileJSON carries all three, so `customAttribution` is not needed unless this fails). Every request outside our origin goes to `tiles.openfreemap.org`, with no `Referer` header beyond the origin | ADR-014, I-6 |
| MD10 | Desktop | Auto-scrub ride-12h with the map in view; record the row | Pass rule above | NFR 60 fps |
| MD11 | Desktop | Reload ride-12h five times; record the start-up rows | Recorded (information) | D-4 |
| MD12 | Desktop | Resize the window narrower and wider | The map resizes; nothing jumps; the route stays | Layout |
| MP1 | Phone | Reload ride-12h. (a) As soon as the charts appear, swipe vertically on a chart again and again until the map has tiles. (b) (v1.5, from QA r2) Reload and keep drag-scrolling the page with the finger held down (no lift) while the map would start. (c) (v1.5) Reload and rest a finger on the page without moving it. Record the start-up row (reload five times) | (a) Scrolling never feels stuck. (b) and (c) The map does not start while the finger is down and starts shortly after it lifts (touch-quiet gate). Start-up row recorded; trigger above. Passed 14 Oct 2026 (a, b, c) | F-05 QA note, D-4 |
| MP2 | Phone | One-finger vertical swipe on the map, away from the route | The page scrolls (MapLibre's "use two fingers" hint may show) | D-10 |
| MP3 | Phone | Two fingers on the map: pan, pinch | The map pans and zooms; the page does not zoom | D-10 |
| MP4 | Phone | Touch down on the route and drag along it, also up and down | The cursor (readout, chart lines, marker) follows; the page does not scroll; no "use two fingers" hint. Lift: the cursor stays | D-10, AC 2 |
| MP5 | Phone | Tap the route; tap the map away from the route; (v1.2) tap the route twice quickly, and do two short scrubs in quick succession; tap the + button where it lies over the route | Route: the cursor jumps there. Elsewhere: nothing changes. The double tap and the quick scrubs never zoom the map. The + button zooms and does not move the cursor | D-10 |
| MP6 | Phone | Scrub a chart on ride-12h with the map in view | The marker follows smoothly (information; the gate is F-07) | NFR 60 fps |
| MP7 | Phone | gps-dropout: scrub across 25:00–30:00 | The marker disappears in the dropout. (v1.4) The dropout segment is visibly dashed at the default zoom | AC 6, D-7 |
| MP8 | Phone | Tiles blocked | Route on plain background; charts work | AC 7 |
| MP9 | Phone | Rotate portrait → landscape → portrait | The map resizes, no jump | Layout |
| MP10 | Phone | Tiles normal | Tiles appear. If Orion's content blocker interferes, record it | ADR-014 |

## Acceptance criteria to verification

| AC | Verified by |
| --- | --- |
| 1 MapLibre requested only after the charts rendered | `RouteMap.dom.test.ts` (no mount before `load`); the built `index.html` check; MD1 |
| 2 Chart scrub moves the marker; route hover moves chart cursor and readout | `map-view.dom.test.ts` (marker, hover, touch); `gesture.test.ts`; `route-index.test.ts`; MD3, MP4 |
| 3 Start, finish and lap markers at the correct positions (several laps) | `markers.test.ts` on golden `ride-12h` (24 laps); MD4. Real corpus files: optional local run by the operator (ADR-004 rev. 2) |
| 4 Several-minute dropout: one connected line, straight segment (v1.4: dashed, user request; feature wording routed to requirements) | `route.test.ts` (`routeLines`) on golden `gps-dropout`; `map-style.test.ts`; MD5, MP7 |
| 5 Indoor: hint, no MapLibre or tile request | `presence.test.ts`, `RouteMap.dom.test.ts`; MD2 |
| 6 Cursor on a sample without position → marker hidden | `map-view.dom.test.ts`; MD5, MD6, MP7 |
| 7 Charts usable while tiles slow or blocked | Design (D-5: base style, tiles in workers, charts independent); MD7, MP8 |

## Hand-off to F-29 (OQ-17, user: yes)

Recommended About-page text for "who else sees what", replacing the bare IP sentence:

> When a workout has a map, your browser loads the map from OpenFreeMap (tiles.openfreemap.org), which may deliver it through Cloudflare. They see your IP address, your browser type and which part of the map you look at, so roughly where the route is. They do not learn which workout you are viewing. OpenFreeMap says it uses no cookies or tracking and deletes its logs after 7 days.

The attribution list names OpenFreeMap, OpenMapTiles and OpenStreetMap (ODbL), and Natural Earth (public domain, the hillshade in liberty).

## Open questions

No F-06 question is open. OQ-15 (entirely hidden route) is F-16's; until then F-06 shows nothing and loads nothing. OQ-14 and OQ-16 to OQ-21 are answered (`INDEX.md`, decision log of `features/README.md`; history).
