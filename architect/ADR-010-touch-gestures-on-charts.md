# ADR-010 Touch gestures on charts: `touch-action: pan-y`, plus a scroll lock once a scrub has started

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 2 of 12 Oct 2026
- **History:** [architect/history/ADR-010-touch-gestures-on-charts.md](history/ADR-010-touch-gestures-on-charts.md) (revision list, the rev. 1 to rev. 2 change table)
- **Features:** F-05 (introduces it); F-17 (long-press then drag selects a range), F-06 (map has its own gestures, not covered here)
- **Supersedes:** none
- **Depends on:** [ADR-008](ADR-008-shared-cursor-store.md), [ADR-009](ADR-009-uplot-as-renderer-only.md)
- **Requirements:** AP-6 ("On touch, a horizontal drag moves the cursor and a vertical swipe scrolls the page"); review decision 6 (assumed gestures); NFR Performance

## Context

Charts fill most of the phone screen. If a chart swallows touches, the page cannot be scrolled; if it ignores them, it cannot be scrubbed. There are two ways to tell the gestures apart:

- **JavaScript direction detection:** a non-passive `touchmove` listener measures the first few pixels and calls `preventDefault` for horizontal moves.
- **CSS `touch-action: pan-y`:** the browser handles vertical panning and sends `pointercancel` when it takes the gesture. Horizontal movement is not a browser gesture on that element, so pointer events keep coming.

Revision 0 and 1 used only the second. The operator's run on an iPhone 13 mini (Orion, iOS WebKit) showed:

- P1 (vertical swipe on a chart scrolls), P4 and P5 (pinch on a chart) pass.
- P2: a horizontal drag that drifts vertically onto the next chart is taken over by WebKit as a scroll in mid-drag. WebKit sends `pointercancel`, and the cursor slides back to where it was. Android Chrome checks `touch-action` once, when the touch leaves its slop, and does not do this. WebKit does.

The user then decided three things:
1. **P3, option B:** the cursor jumps under the finger on touch down. If the gesture becomes a page scroll, the cursor returns to where it was. The brief flicker when scrolling past the charts is accepted.
2. **Scroll lock:** once a horizontal swipe (scrub) has started, the page must not scroll for the rest of that touch. Swipes that start vertical still scroll (P1).
3. **P2:** one drag does not need to continue onto another chart. Do not redesign the touch surface for it.

`touch-action` alone cannot meet decision 2 on WebKit: it cannot withdraw a scroll the browser has decided to start. Only cancelling the `touchmove` before the browser starts scrolling can.

## Decision

1. The plot area (`u.over`) keeps `touch-action: pan-y pinch-zoom`. Vertical swipes scroll the page and pinch zooms it; both stay the browser's.
2. A pure gesture reducer (`lib/charts/gesture.ts`) maps pointer input to cursor actions:
   - **Touch (rev. 2):** `pointerdown` sets the cursor to the index under the finger and remembers the value from before. Each `pointermove` and the `pointerup` (a tap) set the cursor. `pointercancel` (the browser took a scroll or pinch) restores the remembered value. Only the primary pointer counts. A primary `pointerdown` with a new ID while a touch is still tracked replaces the stale touch, because its end was missed (rev. 1).
   - **Mouse and pen:** hover and click set the cursor. Leaving the chart keeps the cursor where it is (F-05 D-8, OQ-13).
3. **Direction rule (rev. 2).** The reducer classifies each touch once, on its first move at least `TOUCH_SLOP` = 8 CSS px (Euclidean) from the touch-down point. Within 30° of horizontal (`3·dy² ≤ dx²`) it is a *scrub*; anything else is a *pan*. The class never changes during the touch.
4. **Scroll lock (rev. 2).** Each chart's `u.over` has exactly one non-passive listener, `touchmove` with `{ passive: false }`. It is added at mount and never during a touch, because WebKit ignores blocking listeners added after the touch started. It calls `preventDefault()` on **every** `touchmove` for which `e.cancelable` is true and `blocksScroll(state, e.touches.length)` holds (touch state, axis scrub, exactly one touch). Every other listener stays passive and never calls `preventDefault`.
5. **Pinch:** never prevented. With two touches `blocksScroll` is false, so the browser may zoom, and its `pointercancel` restores the cursor.
6. **P2 is not a requirement.** A drag that drifts onto a neighbouring chart may stop or carry on; either is accepted. With the lock it will probably carry on, because touch events and implicit pointer capture stay with the start chart.
7. The reducer keeps room for F-17's long-press state; the existing transitions stay as they are.

Full transition table, the browser notes and the tests: [F-05 design](F-05-stacked-charts-with-shared-cursor.md) v1.4, "Interaction".

## Consequences

- A scrub can no longer turn into a page scroll on iOS WebKit. On Chrome, which already locks the direction, the lock changes nothing.
- A vertical swipe that starts on a chart still scrolls: it is classified *pan* and never prevented. A scroll that starts on a chart moves the cursor under the finger first and back again on `pointercancel`. This is the accepted flicker (decision 1).
- **Cost (rev. 2):** the non-passive `touchmove` makes the browser wait for the main thread before it scrolls a touch that starts on a chart. The handler does a few comparisons, so a delay appears only while the main thread is busy (decode, MapLibre start-up). F-06 and F-07 phone checks watch for it. Touches that start outside the charts are unaffected.
- **Cost:** the 8 px / 30° thresholds are ours, assumed and not measured. They are constants in one place, and phone checks P11 and P12 (F-05 design) verify them. Ambiguous diagonals are deliberately left to scrolling: a missed lock costs a retry, but a wrong lock traps the page under the finger.
- If the browser has already started a scroll (`cancelable` false), the lock does nothing and the gesture is a scroll, as in revision 1.
- Desktop click-drag moves the cursor until F-17 gives it selection semantics.

## Would choose differently if

- Phone checks show that the lock delays scrolls that start on charts noticeably (F-06, F-07), or that WebKit ignores `preventDefault` on these `touchmove`s. Then remove the lock and the user decides between `touch-action: none` with full JavaScript gesture handling (a new ADR) and accepting mid-drag takeovers.
- F-17's long-press needs to stop scrolling *before* any movement. Then F-17 extends `blocksScroll` with its pressing state; the listener stays the same.
