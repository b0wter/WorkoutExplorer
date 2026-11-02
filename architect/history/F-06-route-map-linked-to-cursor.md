# F-06 Route map linked to the cursor: history

History of [architect/F-06-route-map-linked-to-cursor.md](../F-06-route-map-linked-to-cursor.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Header version line as written (revision list)

- **Version:** 1.6 (1.6, 15 Oct 2026, text only: the `pixelRatio` row's known limitation (QA-06-r4-1) is fixed by F-07 design D-8, in the lazy `map-view`, at no static cost; ADR-013 rev. 4. 1.5, 14 Oct 2026, QA r3 follow-ups approved by the user:
  - a Node test validates `baseStyle` and `mergeStyle` output with `@maplibre/maplibre-gl-style-spec`, a new direct dev dependency; this is a rule for every feature that adds layers (QA-06-r3-2);
  - the route source's two features and their layers recorded as a refinement of the TD's "one GeoJSON line" (ADR-013 rev. 3, r3-4 d);
  - MP1 gains the r2 cases (r3-4 c);
  - final static numbers in ADR-011 rev. 5 (r3-4 b).
  1.4, 14 Oct 2026, operator results and user request:
  - the D-4 trigger fired on the phone (longest frame interval 264–379 ms), so start-up is made cheaper (`validateStyle` only in dev, `setStyle` with `diff: false`, pixel ratio capped at 2, one more yield) and instrumented with User Timing marks;
  - the re-measure records the frame-rate baseline (30 Hz cap check), and the trigger becomes a product decision (ADR-013 rev. 2, OQ-21);
  - **bridges**: route segments over a GPS dropout or a gap are drawn dashed (D-7 revised, user request; `routeLines` replaces `routeCoordinates`);
  - OQ-20 closed (MapLibre size accepted).
  - **Close-out (14 Oct 2026):**
    - OQ-21 accepted by the user; the trigger bound is 300 ms.
    - The phone re-run (Low Power Mode off) gave a longest start-up interval of 37–93 ms; the trigger did not fire and no ADR-013 deferral remedy is needed. The earlier 264–379 ms runs most likely had Low Power Mode on and are superseded.
    - Phone scrub with the map: 99.16 % of intervals ≤ 20 ms.
    - MD1–MD12 and MP1–MP10 pass, including MD5, MP7 (dashed bridge) and MD9's network part (only `tiles.openfreemap.org`).
    - F-25: bridges stay dashed and uncoloured (user).
  1.3, QA r1: touch-quiet counts fingers with `touchstart`/`touchend`/`touchcancel` instead of pointers (QA-06-r1-2); cleanup of a partial mount (r1-1); input-path allocation rule aligned with F-05, plus `wantsHit` to skip discarded hit tests (r1-4); test additions for coordinates and radius per kind (r1-3); `_tapDragZoom` named (ADR-015 rev. 2); CSP and referrer policy on the worker asset (ADR-014 rev. 2, r1-5); static ceiling 2,200 if the r1 fixes need it (ADR-011 rev. 5). 1.2, engineer stop at implementation: static ceiling 2,100 (ADR-011 rev. 4; the dynamic-import helper measured 785 bytes, not 300); measurement step (c′) replaces (c); confirmed engineer additions (`hitRadiusPx` `kind` parameter, hits only on MapLibre's canvas, base layer named `background`, `bench/map-props.svelte.ts`); double-click and double-tap zoom off; MapLibre's measured size recorded. 1.1, user answers to OQ-14 to OQ-19: basemap **liberty** instead of positron, verified against the live style and TileJSON on 12 Oct 2026 (D-6, "Style and layers", route colour, dev page, MD9, Performance); failure text fixed; OQ-14, OQ-16 to OQ-19 closed, OQ-15 deferred to F-16; About-page wording handed to F-29. 1.0: first version)

## Open questions answered by the user on 12 Oct 2026 (v1.1) and 14 Oct 2026

| ID | Question | Answer |
| --- | --- | --- |
| OQ-14 | Map gestures on the phone (D-10, ADR-015) | Keep as designed; MP2–MP5 confirm it |
| OQ-15 | A route that is entirely hidden: nothing, an empty map, or the hint? | Deferred to F-16. Until then F-06 implements the default: nothing shown, nothing loaded |
| OQ-16 | Text when the map cannot load | "The map could not be loaded." |
| OQ-17 | Tell About-page readers that the tile host sees the map area and may use Cloudflare? | Yes; wording handed to F-29 above |
| OQ-18 | Basemap | **liberty** (D-6) |
| OQ-19 | Browser checks by the operator | The operator runs MD1–MD12 and MP1–MP10 once F-06 is built and records MD10, MD11, MP1 in `MEASUREMENTS.md`; QA reviews the record |
| OQ-20 | MapLibre at 434,882 bytes gzip vs the TD's "roughly 250 KB" | Accepted (14 Oct 2026). TD wording routed to requirements: "as shipped; 6.14: about 435 KB gzip (JS + worker + CSS)", report trigger +10 % on upgrades |
| OQ-21 | Accept one main-thread pause per page load of up to 300 ms on the phone during map start-up (never under a finger, D-4)? | **Accepted** (14 Oct 2026): "this is a heavy app and the iphone mini 13 is not a particularly strong phone". The trigger bound is 300 ms (ADR-013 rev. 2). The re-run after v1.4 measured 37–93 ms |
