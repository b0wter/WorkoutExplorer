# ADR-013 MapLibre GL JS v6 behind one lazy adapter: base style first, DOM cursor marker, start-up in touch-quiet moments

- **Status:** accepted
- **Date:** 12 Oct 2026; revision 4 of 15 Oct 2026
- **History:** [architect/history/ADR-013-maplibre-behind-a-lazy-adapter.md](history/ADR-013-maplibre-behind-a-lazy-adapter.md)
- **Features:** F-06 (introduces it); F-07 (page wiring, phone gate), F-14 (dark style), F-17 (range layer), F-25 (route colouring), F-26 (fullscreen) build on it
- **Note (9 Nov 2026, F-36 MC-7, ADR-041):** item 1 gains one exception: `components/map/basemap.ts` also imports `maplibre-gl` as a value, reached only through `import()` from `src/edit`, for the offscreen preview basemap (light style, no `wx-` layers). Everything else in this ADR applies to the visible map only
- **Supersedes:** none. Refines the TD sentences "the cursor marker as a single feature updated on each cursor move" (decision 4) and (rev. 3) "the route is added as one GeoJSON line" (decision 8)
- **Depends on:** [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-008](ADR-008-shared-cursor-store.md) rev. 1, [ADR-010](ADR-010-touch-gestures-on-charts.md) rev. 2, [ADR-011](ADR-011-activity-page-bundle-allocation.md) rev. 5, [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md)
- **Requirements:** AP-3, AP-6; NFR Performance (60 fps scrubbing, LCP); TD Tech stack (Map: MapLibre GL JS, loaded after the charts), TD Front end (Load order step 4: "MapLibre loaded with a dynamic `import()`, then tiles; the route is added as one GeoJSON line, the cursor marker as a single feature updated on each cursor move"), TD Security (strict CSP)

## Context

The TD fixes MapLibre as the map and says it loads last. Four questions remain, and each one is expensive to change once F-14, F-17, F-25 and F-26 build on the map:

1. **Which MapLibre, and how does it load under a strict CSP?** MapLibre v6 (current: 6.14.0) is ESM-only, needs WebGL2 (the constructor throws `GPUInitializationError` without it) and runs its tile work in a Web Worker. With a bundler, the worker URL is set by `setWorkerUrl`; Vite's documented form is `import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'`. A worker served from our origin needs only `worker-src 'self'`; the CDN form needs `blob:`. v5's separate CSP build no longer exists.
2. **What happens when the tile host is slow or blocked?** If the route layers are added to the remote style, nothing draws until that style has loaded; if it never loads, the map stays empty (F-06 AC 7 asks that charts stay usable; an empty map is the minimum, the route on a plain background is better).
3. **How does the cursor marker move?** The TD's "single feature" read literally is a GeoJSON source updated with `setData` on every cursor move. `setData` serialises the data to the worker, re-tiles it there and repaints the whole WebGL map: a worker round trip (so the marker lags the chart cursor by a frame or more) and a full repaint per frame while scrubbing. A MapLibre `Marker` is one DOM element moved with a CSS transform: same frame, no repaint.
4. **How does start-up avoid stalling scrolls?** F-05 D-18 added a non-passive `touchmove` on the charts (the scroll lock). A touch that starts on a chart while the main thread is busy waits for it, so MapLibre's start-up (chunk evaluation, WebGL context, style setup, shader compilation) can make the first scroll feel sticky (F-05 QA note).

## Decision

1. **MapLibre GL JS v6** (caret range on the 6.x current at implementation; lockfile pins it). Only `components/map/map-view.ts` imports it as a value, and only `components/map/RouteMap.svelte` reaches that module, through `import('./map-view')`. The worker is bundled with `?worker&url`, served from our origin and passed to `setWorkerUrl`. `map-style.ts` uses `import type` only. Nothing else in the app imports `maplibre-gl`.
2. **The map starts with a local base style**: a background, our route and marker sources (inline GeoJSON) and layers, and the tile host's glyphs URL. The adapter fetches the remote style JSON itself and, after the map's `load` event, applies it with `setStyle(json, { transformStyle: mergeStyle })`. `mergeStyle` (pure) puts every `wx-` source and layer of the current style on top of the remote style, forces Mercator and drops terrain and sky. A failed style fetch keeps the base style: the route and markers stay visible on a plain background. F-14's dark style switch uses the same merge.
3. **One adapter, one handle.** `mountRouteMap(options): Promise<RouteMapHandle>` creates the map, the marker, the cursor subscription and the input listeners; `destroy()` removes all of it. Pure decisions (route geometry, markers, nearest sample, gesture reducer, style merge) live outside the adapter in `lib/route` and `map-style.ts`, so the adapter stays thin enough for a fake-based smoke test (ADR-012 rev. 3).
4. **The cursor marker is one MapLibre `Marker` with our own element**, moved with `setLngLat` in the cursor listener and hidden with `visibility` when the cursor is `null` or its sample has no position. This replaces the TD's "single feature" reading; the TD's intent (one object updated, the route never rebuilt) is kept. Our listener allocates nothing (one reused `LngLat`); MapLibre's few short-lived objects inside `setLngLat` are accepted, as F-05 D-10 accepts transform strings.
5. **Start-up only in touch-quiet moments.** A static `touch-quiet` helper counts fingers on `window` from the moment `RouteMap` mounts. It uses passive `touchstart`, `touchend` and `touchcancel` listeners and the count `e.touches.length` (F-06 v1.3). Not pointer events: `pointercancel` fires when a scroll starts, while the finger is still down. `whenQuiet()` resolves when no finger is down and none lifted in the last 250 ms, then after one frame. The adapter awaits it before `import()`, before `new Map()` and before applying the remote style. Mouse and pen never delay anything.
6. **Failure is contained.** No WebGL2, a failed chunk or a throw during mount shows one line of text in the map's place; the charts and the cursor store are not touched. (rev. 1) A throw after `new Map()` first undoes what exists: subscription, listeners, style fetch, `map.remove()`. No WebGL context is leaked. Tile and style errors are swallowed by one `error` listener (no console flood offline, no tile coordinates in the console).

7. **(rev. 2) Start-up kept cheap and observable.** The first phone measurement (iPhone 13 mini, dpr 3) showed a longest frame interval of 264–379 ms; desktop showed 130–217 ms. Changes:
   - `validateStyle` only in development (`import.meta.env.DEV`);
   - the remote style applied with `diff: false` (the diff from the base style to liberty fails and rebuilds anyway);
   - `pixelRatio` capped at 2. (rev. 4) The cap is re-applied with `map.setPixelRatio` whenever `devicePixelRatio` changes (desktop zoom, a move to another monitor), through a re-armed `matchMedia('(resolution: …dppx)')` listener in the lazy adapter. It costs no static bytes (F-07 design D-8);
   - one more touch-quiet yield between our route model and `new Map()`;
   - User Timing marks and measures (`wx-map:*`), so the dev page can say which step holds the longest frame.

   The work inside `new Map()`, `setStyle` and MapLibre's first renders cannot be split further from outside.

   (rev. 3, QA-06-r3-2) Because production skips validation, a Node test in CI validates `baseStyle` and `mergeStyle` output with `@maplibre/maplibre-gl-style-spec`, a direct devDependency pinned to the version `maplibre-gl` uses. Every later feature that adds or changes `wx-` sources or layers (F-14, F-17, F-25, F-26) extends that test (F-06 design, Tests).
8. **(rev. 3) The route is one GeoJSON source, `wx-route`, with two features**, both `MultiLineString`s, each present only when non-empty:
   - `kind: 'recorded'`, drawn solid by `wx-route-casing` and `wx-route-line`;
   - `kind: 'bridge'`, drawn dashed by `wx-route-bridge`, for straight segments over a GPS dropout or a gap.

   This refines the TD's "the route is added as one GeoJSON line" (user request, F-06 v1.4). The TD's intent is kept: one source, built once, connecting every positioned sample, never rebuilt while scrubbing. Two features are needed because `line-dasharray` is selected per layer through a filter. F-25 colours only the recorded feature; bridges stay dashed and uncoloured.

Details: [F-06 design](F-06-route-map-linked-to-cursor.md), "Loading and start-up", "Style and layers", "Cursor marker".

## Consequences

- The route appears as soon as the map exists, whatever the tile host does. Lap labels need the tile host's glyphs and are missing while it is unreachable; lap circles are not.
- Scrubbing a chart costs one marker transform per frame and never repaints the WebGL map, so the 60 fps budget measured in F-05 barely changes.
- The CSP needs no `blob:` for workers (ADR-014 lists the sources for F-22).
- The worker is an extra asset (not listed by the size script, which follows imports only); its size is reported by hand (F-06 Bundle impact).
- **Measured (6.14.0, F-06 implementation):** MapLibre is 434,882 bytes gzip in total: JS 280,175, CSS 10,458, worker 144,249. That is above the TD's "roughly 250 KB" and F-06's 350,000 report threshold. It is lazy, after the charts, outside the 60,000, and cached under a content hash until the next MapLibre upgrade. The user accepted it (OQ-20, 14 Oct 2026); the TD figure is updated by requirements.
- **Cost:** the remote style is fetched by our code instead of MapLibre's, so a style with relative URLs would break (the F-06 design makes that a stop-and-report). `mergeStyle` must keep up with style-spec keys that affect the hit test (projection, terrain); its tests pin both.
- **Cost:** the touch-quiet gate cannot stop a touch that starts *during* a long task, and it does not cover the browser's evaluation of the chunk or MapLibre's self-scheduled work. The start-up measurement bounds what is left (trigger below).
- **Cost:** a new `data` value builds a new map (one WebGL context at a time; `map.remove()` releases the old one). F-19, which swaps data on the edit page, may ask for in-place source updates later.

## Would choose differently if

- **(rev. 2) Trigger.** It fires when check MP1 (sticky first scroll) fails, or when the phone's longest start-up frame interval, measured after decision 7 with Low Power Mode off and no 30 Hz cap, exceeds **300 ms**. The user accepted that bound on 14 Oct 2026 (OQ-21: "this is a heavy app and the iphone mini 13 is not a particularly strong phone"). The re-run after decision 7 (Low Power Mode off) measured 37–93 ms and MP1 passes, so the trigger has not fired and none of the remedies below is in use. If it fires later (for example after a MapLibre or style upgrade), apply them in this order:
  1. Defer the remote style. Apply it only when the map container is in the viewport and the page has been touch-quiet for 1 s, so the route on its base style is all that starts with the page.
  2. If the step table points at `new Map()` itself: start the map only when its container is in the viewport and the page has been touch-quiet for 1 s.
  3. `pixelRatio: 1` until the first `idle`, then 2.
- (rev. 2) Bridges (`wx-route-bridge`) stay dashed and uncoloured when F-25 colours the route (user decision, 14 Oct 2026).
- The F-07 phone trace shows the marker update itself costing frames. Then draw the marker on our own absolutely positioned element from `map.project`, outside MapLibre's `Marker` (same decision 4, less library code per frame).
- MapLibre v7 changes the worker or style API. Then the adapter and `mergeStyle` change; this ADR is revised, not superseded, as long as decisions 1–8 hold.
- The tile host goes away or its terms change. Then switch to the TD's fallback, a self-hosted Protomaps file served by Caddy (ADR-014); decision 2 makes the swap a style URL and a CSP change.

## Reversibility

Cheap while only F-06 uses the map. Once F-14, F-17, F-25 and F-26 add layers and handle methods, the base-style-plus-merge model and the `wx-` naming are a published interface inside the front end.
