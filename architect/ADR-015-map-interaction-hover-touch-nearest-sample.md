# ADR-015 Map interaction: cooperative gestures, a one-finger scrub that starts on the route, nearest sample with earliest-on-tie

- **Status:** accepted
- **Date:** 12 Oct 2026; revision 2 of 14 Oct 2026
- **History:** [architect/history/ADR-015-map-interaction-hover-touch-nearest-sample.md](history/ADR-015-map-interaction-hover-touch-nearest-sample.md)
- **Features:** F-06 (introduces it); F-07 (route-drag smoothness, metric definitions), F-17 (range highlight on the map), F-26 (fullscreen turns cooperative gestures off)
- **Supersedes:** none
- **Depends on:** [ADR-008](ADR-008-shared-cursor-store.md) rev. 1, [ADR-010](ADR-010-touch-gestures-on-charts.md) rev. 2, [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md)
- **Requirements:** AP-3 (pan, zoom), AP-6 ("hovering the route moves the chart cursor"; "on touch, a horizontal drag moves the cursor and a vertical swipe scrolls the page"); F-06 scope ("hovering or touch-dragging the route sets the cursor store to the nearest sample"); F-07 AC 3 ("hovering/dragging on the route"); review decisions 5 (nearest sample, earliest on a tie) and 6 (touch gestures)

## Context

On a phone the map sits in a scrolling page above the charts. Three demands meet on it:
- the page must scroll (AP-6's spirit, decision 6);
- the map must pan and zoom (AP-3);
- a drag along the route must move the cursor (F-06 scope, F-07 AC 3).

MapLibre's default (one finger pans) traps page scrolling wherever the map is. Its `cooperativeGestures` option gives one-finger swipes to the page and two-finger gestures to the map, and on desktop zooms only with Ctrl/⌘ + wheel; but then a one-finger drag along the route scrolls the page instead of scrubbing.

The chart rule (ADR-010: classify by direction after 8 px) does not carry over: a route runs in every direction, so direction says nothing about intent on the map. Location does: a touch that starts on the drawn route means the route.

"Nearest sample, earliest on a tie" (decision 5) needs a definition of tie. Exact float ties almost never happen. But out-and-back routes and repeated loops lie a few metres apart, i.e. within a pixel or two. There, a strict nearest-sample rule flickers between passes as the pointer moves.

## Decision

1. **Cooperative gestures on** for the embedded map: one finger scrolls the page, two fingers pan and zoom the map; desktop drag pans, Ctrl/⌘ + wheel and the zoom buttons zoom. No rotation, no pitch. (rev. 1) **No double-click or double-tap zoom** (`doubleClickZoom: false`, and any separate tap-drag zoom off). MapLibre counts a route scrub under 500 ms as a tap, so two quick scrubs or taps on the route zoomed the map. F-26's fullscreen map turns cooperative gestures off and keeps double-tap zoom off (F-26 FS-6, 3 Nov 2026).
   - (rev. 2) **Mechanism, MapLibre 6.14.0:** `doubleClickZoom: false` in the options, plus `map.touchZoomRotate._tapDragZoom.disable()` after construction. Despite the underscore, `_tapDragZoom` is a public, non-`@internal` field in 6.14.0's `.d.ts`, so a rename in an upgrade fails `svelte-check`; the lockfile pins 6.14.0. **Caveat:** `touchZoomRotate.enable()` turns `_tapDragZoom` back on. Any later code that re-enables `touchZoomRotate` (F-26 fullscreen) must disable `_tapDragZoom` again if it wants double-tap-drag off.
   - (rev. 1) Only pointers whose target is MapLibre's canvas can hit the route, so controls lying over the route (zoom buttons, attribution) never move the cursor or start a scrub. A touch `down` always uses the 20 px start radius, also when it replaces a stale scrub.
2. **A one-finger touch that starts within 20 CSS px of the route scrubs.** Touch down sets the cursor to the route sample under the finger. Each move sets it to the nearest sample within 48 px; beyond that the cursor stays. Lifting keeps the cursor; `pointercancel` restores the value from before the touch. A second finger ends the scrub and hands the gesture to MapLibre. Touches that start elsewhere belong to the page and MapLibre.
3. **Route-scrub lock.** The outer map element has one non-passive, capture-phase `touchmove` listener, added at mount (never during a touch, WebKit bug 184251, as ADR-010). While one finger scrubs, it calls `preventDefault()` (if `cancelable`) and `stopPropagation()`. The page does not scroll, and MapLibre never sees that touch's moves, so it neither pans nor shows its two-finger hint. `touchstart` and `touchend` are never stopped. Every other listener is passive.
4. **Mouse and pen:** hovering within 12 px of the route sets the cursor; moving off it, leaving the map or dragging the map leaves the cursor where it is (F-05 D-8).
5. **Nearest sample:** the positioned sample nearest to the pointer in screen distance (computed in Web Mercator, valid because pitch is 0 and the projection is Mercator, ADR-013 decision 2). Samples within `TIE_PX` = 3 CSS px of the nearest distance are tied, and the **lowest sample index** wins: the earliest pass, as decision 5 says. Implemented with a uniform grid over the route's Mercator coordinates, built once per map, queried without allocation.
6. **One pure reducer** (`lib/route/gesture.ts`) holds the transitions and radii; the adapter does the hit test and the DOM. The constants (`HOVER_RADIUS_PX` 12, `TOUCH_START_RADIUS_PX` 20, `TOUCH_DRAG_RADIUS_PX` 48, `TIE_PX` 3) are assumed, defined once, and checked by the operator (F-06 MP2–MP5).

Transition table and listener list: [F-06 design](F-06-route-map-linked-to-cursor.md), "Route interaction".

## Consequences

- One finger scrolls the page over the whole map except a band of about 40 px around the route line. A scroll that starts in that band scrubs instead. That is the cost of route scrubbing without a mode switch.
- Two-finger panning and pinching work anywhere on the map, but a two-finger gesture whose first finger lands on the route starts as a scrub for that instant. When the second finger lands, MapLibre takes over. MapLibre saw both `touchstart`s, so its state is consistent.
- Desktop users must hold Ctrl/⌘ to zoom with the wheel; a plain wheel over the map scrolls the page (MapLibre shows a short hint).
- Over overlapping passes, hover and route scrubbing always give the earliest pass. To reach a later pass there, scrub the chart. The rule is a placeholder until F-07 settles the metric definitions; a change is local to `TIE_PX` and `nearest`.
- **Cost:** the lock depends on MapLibre listening for touch moves below our outer element and on touch events. The F-06 design requires the engineer to verify this against the installed version and stop if it does not hold.

## Would choose differently if

- The user prefers normal one-finger map panning or tap-only on touch (OQ-14). The first turns decision 1 off on touch devices and keeps 2–3. The second drops 2–3 and keeps a tap rule.
- MP4 shows route scrubs starting by accident during page scrolls. Then shrink `TOUCH_START_RADIUS_PX`, or require the touch to start on the drawn line (within the line width plus 4 px).
- F-07's metric definitions prefer continuity (stay on the pass the pointer came from). Then `nearest` gains a preferred index window, in a revision of this ADR.
