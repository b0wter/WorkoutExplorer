# F-25 Route colouring by metric: architecture

- **Version:** 0.1
- **Status:** accepted (2 Nov 2026). Normal lane. OQ-99 decided 3 Nov 2026 (keep both in fullscreen, decision log)
- **Date:** 2 Nov 2026
- **Feature:** [`features/F-25-route-colouring.md`](../features/F-25-route-colouring.md)
- **Requirements:** AP-9, AP-3, AP-4 (transitions), DS-1, DS-2, DS-4, NFR Performance (60 fps); F-08 hand-offs to F-25 (user answer 6, R-14)
- **Builds on:** [F-06](F-06-route-map-linked-to-cursor.md) D-7, D-8, D-11, map layers; [F-14](F-14-units-and-theme.md) D-1, D-3, D-8, D-9, D-11, D-12 and its F-25 row; [F-16](F-16-start-and-end-hiding.md) H-7; [F-17](F-17-range-selection-and-axis-toggle.md) R-2, R-10 and its F-25 row; [F-13](F-13-activity-page-layout.md) D-4, D-18, D-19; ADR-007, ADR-011, ADR-013, ADR-016, ADR-027, ADR-032; new [ADR-037](ADR-037-route-colouring-as-colour-classes.md)
- **Owner:** frontend only. No backend step, no data file change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| C-1 | **Colour classes, not a gradient** (ADR-037). Each sample's value maps to one of **7 classes** or none. New source `wx-colour`: at most one `MultiLineString` per class. New layer `wx-colour-line`: `line-color` by class, the route line's width, round joins and caps. Layer order bottom to top: `wx-route-bridge`, `wx-route-casing`, `wx-route-line`, **`wx-colour-line`**, `wx-hidden-*`, `wx-range-*`, markers. Plain mode = empty source, present from the base style on | `line-gradient` cannot use feature properties; per-edge features cost up to 43,200 features per switch. Range above colouring is F-17's hand-off |
| C-2 | **What is coloured: recorded edges only.** A recorded edge always joins two consecutive samples `i−1, i` (F-06 D-7: anything else is a bridge), and it takes **sample `i`'s class**. Bridges are never in `wx-colour` (AC 7). Hidden samples have no position, so no edge (AC 6, F-16 H-7). An edge with no class is left out, so the plain route line shows through: missing values, pace-sport stops and transitions draw in the plain route colour, never in an end class (AP-9) | Exact by construction; no special layer for "missing" |
| C-3 | **Values per metric** (table below). Smoothing never fills: a sample that is missing stays missing, and a mean covers present samples only and never crosses a time gap (≥ 5 s). Windows are starting values in one place, reviewed on screen by the operator, like `STOP_SPEED` | Raw 1 s power colours the map as confetti; gradient from neighbouring samples is mostly altitude noise |
| C-4 | **Scale.** Speed or pace, heart rate, power: **workout-relative**. Bounds are the 5th and 95th percentile of the drawn edges' values (so hidden sections and stops do not move them); values outside clip to the end classes; 7 equal-width classes between the bounds. Equal bounds give one class, never a division by zero. Bounds may be rounded outward to the legend's precision. Pace sports class **in pace** (time per distance), so the legend steps evenly; metric ↔ imperial is linear in both speed and pace, so a units switch only relabels the legend. Gradient: **fixed** class edges −8, −4, −1, +1, +4, +8 % (starting values), not percentiles | A gradient has a physical meaning; a flat ride must look flat instead of spreading 1 % noise over the whole scale |
| C-5 | **Colour order and palettes.** Speed and pace: slow → fast on the same scale direction for every sport (a run reads like a ride; matches the flipped pace chart). Heart rate, power: low → high. One sequential palette for these, one diverging palette for gradient (descent, flat in the middle, climb). Both are 7 custom properties each in the theme block, light and dark (F-14 D-9), read like the other map colours; a scheme change repaints the layer and the legend follows through CSS classes. Constraints: sequential lightness is monotonic; no class is confusable with `--route` (missing) or `--range` (drawn over the colouring) in either theme; the order stays readable with red–green colour-vision deficiency. The engineer proposes the values; the operator reviews them on screen | One home per colour (F-14) |
| C-6 | **Selector and legend: one map control, lazy.** `map-view` adds it at MapLibre's `bottom-left`. `map-view.css` already moves that corner to the top in both two-column layouts (F-13 D-18: the 100lvh map's bottom strip may be out of view); in one column it stays bottom left, clear of the pin toggle (top left), zoom (top right) and attribution (bottom right). A native `<select>` (F-14 D-8's reasoning): "plain route" first and default, then the offered metrics in AP-9's order: speed (labelled **pace** for the pace sports by the workout sport, as F-13 D-4: `multisport` shows speed), heart rate, power, gradient. No offered metric: no control. The legend sits under the select only while a metric is on: the 7 swatches, the bound labels (gradient: its class edges) with the unit, and a "no data" swatch in the route colour when a drawn edge has no class. Texts through `lib/format` with the units store, rewritten on a units change (AC 3, AC 4). Swatches are coloured by CSS classes, never `style` attributes (ADR-032). The route hit test already ignores anything but the canvas (F-06 v1.2). Wording by the engineer (OQ-63 rule) | No byte before the map; no layout reservation, because it lives inside the map box |
| C-7 | **State.** The choice is held in `map-view` memory only (feature scope), kept across scheme and units changes and style swaps (the latest `wx-colour` data is re-applied on `style.load`, as `wx-range`), reset with a new `data` value. The edit page gets the same control through `RouteMap`, over the full draft route | One code path (F-13 hand-off to F-19) |
| C-9 | **The selected range with colouring on** (user review, 3 Nov 2026, option (a)): the range keeps the metric's class colours (edges without a value in the route colour, bridges as without colouring) over a thick outline, `wx-range-casing` (`--range-outline`); the route outside it fades to `--route-fade` (both light and dark). Colouring off: as before F-25. A range update partitions the kept per-sample classes over the range's edges only (O(log N + range), after F-17's settle); values and bounds are never recomputed | The selection stays readable as a selection without hiding the colouring |
| C-8 | **Cost.** *Before the map:* nothing; all F-25 code is in the lazy map chunk. The static change is `RouteMap` passing the units store through. *At map start:* the plain route as today; deciding which metrics are offered is one pass per stream that stops at the first value on a positioned sample, inside step 6 (`wx-map:route-model`). No classes at start-up. *On a selection:* values, bounds, classes and geometry for that metric (O(N), a few ms on `ride-12h`), then one `setData`; caused by the viewer, off the scrub path. *Scrubbing:* nothing added: F-06 D-11 stands (DOM marker, no `setData`, no repaint); no colouring code subscribes to the cursor or the selection | AC 5 |

**Values per metric** (C-3):

| Option | Offered when | Value per sample | No class when |
| --- | --- | --- | --- |
| Speed (pace for pace sports) | the speed stream has a value at a positioned sample | centred mean over 10 s of time | missing; pace sports: speed below `STOP_SPEED` (DS-2: a stop has no pace). Speed sports: a stop is a real 0 and takes the slowest class; the percentile base leaves out samples below `STOP_SPEED` |
| Heart rate | the stream has a value at a positioned sample | as recorded | missing. Transitions are coloured (AP-9) |
| Power | the stream has a value at a positioned sample | centred mean over 30 s of time, zeros counted (as F-17 R-2) | missing (transitions have none, F-08 R-13) |
| Gradient | altitude **and** distance streams both have a value at a positioned sample | altitude change ÷ distance over a centred 100 m distance window, altitude interpolated in distance between valid samples, in % | altitude or distance missing at the sample; **distance not above the previous valid distance** (standstill or held stretch, F-08 R-14); the window spans less than 50 m; the sample lies in a session with sport `transition` (F-08 hand-off, user answer 6) |

## Module boundary

| Module | Adds | May import |
| --- | --- | --- |
| `lib/colouring` (new, pure) | offered metrics, per-sample values with smoothing and gradient, bounds, classes, the legend's numbers | `lib/datafile`, `lib/format` |
| `lib/route` | the recorded edges partitioned by a per-sample class array into per-class lines, by F-06 D-7's edge rule; no metric knowledge | unchanged |
| `components/map` (lazy: `map-view`, `map-style`, `map-view.css`) | `wx-colour` source and layer, the control, the legend | + `lib/colouring`, `lib/units` |
| `RouteMap` (static), `src/page`, `src/edit` | the units store passed to the map | unchanged |

`lib/colouring` does not import `lib/route` or `lib/charts`; `lib/route` stays metric-free. Every new `wx-` source and layer joins `map-style.validate.test.ts` (F-06 rule).

## Budget (ADR-011)

| What | Decision |
| --- | --- |
| Static | Only the units prop wiring, expected under 50 bytes gzip, from the reserve; reported |
| Lazy | F-25's own lazy delta at most **3,000** bytes gzip, reported beside F-06's lazy figure. Above: stop and report |

## Verification

| AC | Automated (`make ci`) | Manual (operator) |
| --- | --- | --- |
| 1 | Offered set on synthetic data: heart rate without power; gradient only with altitude and distance | A corpus ride |
| 2 | Existing presence tests; the control exists only inside a mounted map | Indoor and fully hidden samples |
| 3 | Classes are monotonic in the value; clipping at the bounds; legend labels in both unit systems | Corpus ride, units switched with colouring on |
| 4 | Label and legend per pace sport and unit system (run, swim, row) | A corpus run |
| 5 | Review: no colouring code on the cursor or selection path | ADR-016 scrub record on `ride-12h` on the phone, colouring by speed and by gradient |
| 6, 7 | Partition property: every coloured edge is a recorded edge; no vertex from a sample without a position; bridges never in `wx-colour` (golden `gps-dropout`, `pause-hours`, a hidden-ends variant) | `gps-dropout` and a sample with hiding on |
| C-3 rules | Gradient: standstill and held distance give no class, never ±∞; transition samples give none for speed, power and gradient, and a class for heart rate | Gradient on a hilly corpus ride; palettes in both themes |

## Hand-offs

| To | What |
| --- | --- |
| F-26 | The control and legend are part of the map container and follow it into fullscreen (OQ-99 decided; F-26 FS-3). `wx-range` stays above `wx-colour-line` (F-17) |
| Requirements | TD Code structure: add `lib/colouring` to the layout and the front-end dependency rules; `components/map` may import `lib/units` |

## Open questions

| ID | Question | Suggestion |
| --- | --- | --- |
| OQ-99 | AP-15's fullscreen keeps "the route, markers, zoom controls and an exit button"; the colour selector and legend are not listed. Keep them in fullscreen? | Decided 3 Nov 2026: keep both (decision log) |
