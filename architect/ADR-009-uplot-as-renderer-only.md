# ADR-009 uPlot as renderer only: our decimation, our pointer handling, our cursor line

- **Status:** accepted
- **Date:** 11 Oct 2026
- **Features:** F-05 (introduces it); F-14 (units on the charts), F-17 (selection, distance axis), F-13 (layout)
- **Supersedes:** none
- **Depends on:** [ADR-002](ADR-002-bundle-size-budget-measurement.md) (bundle budget; uPlot is in the counted bundle), [ADR-008](ADR-008-shared-cursor-store.md) (cursor store)
- **Requirements:** AP-4, AP-6; TD Workout data file, Charts ("min/max-per-pixel view computed in the browser"); NFR Performance (60 fps on a 12-hour workout)

## Context

uPlot (1.6.32, already a dependency since F-01) brings its own cursor, legend, selection, chart sync (`cursor.sync`), and drawing of every point it is given. Using those features would be the shortest path, but:

- uPlot's cursor redraws its overlay and fires hooks per instance. Its sync is a uPlot-to-uPlot channel the map (F-06) cannot join, and it carries a value position, not our sample index. That makes a second source of truth next to the cursor store (ADR-008).
- uPlot's mouse handling does not implement decision 6 (horizontal drag scrubs, vertical swipe scrolls) on touch.
- Giving uPlot 43,200 points × 6 charts works, but every resize and redraw then scales with N, not with pixels. The TD asks for min/max per pixel explicitly.
- uPlot draws `null` as a break only with `spanGaps: false`. Its handling of `NaN` is not specified.

## Decision

1. **uPlot only draws.** Every instance has `cursor.show`, `legend.show` and `select.show` set to `false`. With the cursor off, uPlot binds no mouse listeners (`uPlot.esm.js:5988`, 1.6.32). No `cursor.sync`, no hooks on the hot path.
2. **We decimate.** `lib/charts/decimate` turns full-resolution `WorkoutData` arrays into a min/max-per-pixel-column series with `null` at every break (missing sample or gap). uPlot never receives `WorkoutData` arrays or `NaN`.
3. **We own interaction.** Pointer Events on `u.over` go through a pure gesture reducer into `cursor.set` (ADR-010).
4. **We draw the cursor line** as one CSS-transformed `div` per chart, positioned with `u.valToPos`. A cursor move never redraws a canvas.
5. **Alignment is explicit:** an identical fixed x range, a fixed y-axis width and fixed padding on every chart (uPlot's auto padding differs between charts with and without a bottom axis).
6. **Only `components/charts` imports `uplot`.** Everything else talks to charts through the cursor store and, later, other stores.

## Consequences

- Scrubbing cost does not depend on N or on uPlot internals: one `transform` per chart per frame.
- uPlot can be replaced (or upgraded) inside one folder. Decimation, gestures and the cursor are library-free and unit-tested in Node.
- **Cost:** we write and maintain about 300–500 lines that uPlot would partly provide (decimation, pointer mapping, cursor line), and must track uPlot layout details (padding, axis size) that the defaults would hide. The padding finding is pinned by a unit test of the options builder.
- Points shorter than one pixel column are folded into their column's min/max. A segment that decimates to a single isolated point is not drawn; at most one pixel column wide, accepted.

## Would choose differently if

- uPlot's built-in cursor turns out cheaper than our `div` in the F-05 measurement and can be driven by our index without its own listeners. Then use `u.setCursor` from the store listener, behind the same `MetricChart` boundary.
- The bundle budget becomes tighter than uPlot's share allows (F-07 projection over 60,000). Then a hand-written canvas line renderer (we already own decimation and interaction) replaces uPlot, in a superseding ADR.
