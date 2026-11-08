# F-25 Route colouring by metric: architecture

- **Version:** 0.2
- **Status:** accepted (2 Nov 2026; revised 7 Nov 2026 with the user's decisions after testing on real rides: three-hue palette, 12 classes, quantile edges). Normal lane. OQ-99 decided 3 Nov 2026 (keep both in fullscreen, decision log)
- **Date:** 7 Nov 2026
- **History:** [history/F-25-route-colouring.md](history/F-25-route-colouring.md) (v0.1 scale, palette and legend)
- **Feature:** [`features/F-25-route-colouring.md`](../features/F-25-route-colouring.md)
- **Requirements:** AP-9, AP-3, AP-4 (transitions), DS-1, DS-2, DS-4, NFR Performance (60 fps); F-08 hand-offs to F-25 (user answer 6, R-14)
- **Builds on:** [F-06](F-06-route-map-linked-to-cursor.md) D-7, D-8, D-11, map layers; [F-14](F-14-units-and-theme.md) D-1, D-3, D-8, D-9, D-11, D-12 and its F-25 row; [F-16](F-16-start-and-end-hiding.md) H-7; [F-17](F-17-range-selection-and-axis-toggle.md) R-2, R-10 and its F-25 row; [F-13](F-13-activity-page-layout.md) D-4, D-18, D-19; ADR-007, ADR-011, ADR-013, ADR-016, ADR-027, ADR-032; new [ADR-037](ADR-037-route-colouring-as-colour-classes.md)
- **Owner:** frontend only. No backend step, no data file change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| C-1 | **Colour classes, not a gradient** (ADR-037). Each sample's value maps to one of **12 classes** (gradient: **11**, C-4) or none. New source `wx-colour`: at most one `MultiLineString` per class. New layer `wx-colour-line`: `line-color` by class, the route line's width, round joins and caps. Layer order bottom to top: `wx-route-bridge`, `wx-route-casing`, `wx-route-line`, **`wx-colour-line`**, `wx-hidden-*`, `wx-range-*`, markers. Plain mode = empty source, present from the base style on | `line-gradient` cannot use feature properties; per-edge features cost up to 43,200 features per switch. Range above colouring is F-17's hand-off |
| C-2 | **What is coloured: recorded edges only.** A recorded edge always joins two consecutive samples `i−1, i` (F-06 D-7: anything else is a bridge), and it takes **sample `i`'s class**. Bridges are never in `wx-colour` (AC 7). Hidden samples have no position, so no edge (AC 6, F-16 H-7). An edge with no class is left out, so the plain route line shows through: missing values, pace-sport stops and transitions draw in the plain route colour, never in an end class (AP-9) | Exact by construction; no special layer for "missing" |
| C-3 | **Values per metric** (table below). Smoothing never fills: a sample that is missing stays missing, and a mean covers present samples only and never crosses a time gap (≥ 5 s). Windows are starting values in one place, reviewed on screen by the operator, like `STOP_SPEED` | Raw 1 s power colours the map as confetti; gradient from neighbouring samples is mostly altitude noise |
| C-4 | **Scale.** Speed or pace, heart rate, power: **workout-relative, equal-share (quantile) edges.** The 11 edges are the k/12 quantiles (k = 1…11) of the drawn edges' values (so hidden sections and stops do not move them), so each class covers about a twelfth of the drawn route. No bounds, no clipping: the end classes hold the lowest and highest twelfth. Edges are not rounded; equal edges (ties, e.g. integer heart rate) leave the classes between them empty, by the existing "class = number of edges ≤ value" rule. A constant metric gives one class. Pace sports class in pace; quantiles do not change under the monotonic speed ↔ pace map, and metric ↔ imperial is linear, so a units switch only relabels the legend. Gradient: **fixed** edges, **11 classes** with a neutral flat class in the middle: −12, −8, −5, −3, −1, +1, +3, +5, +8, +12 % (starting values) | Quantiles put the contrast where the ride spends its time; equal width, even between tighter bounds, still gives a whole class to rare values and lumps the dense middle (the "flat vs climb" complaint). Cost: uneven legend numbers, and a steady ride shows small differences strongly. Gradient stays fixed (physical meaning, a flat ride looks flat); an even count would split "flat" at 0 % into two colours, so it keeps an odd count |
| C-5 | **Colour order and palettes.** Speed and pace: slow → fast on the same scale direction for every sport (a run reads like a ride; matches the flipped pace chart). Heart rate, power: low → high. **Sequential: three hues, blue → yellow → red** (12 steps); **diverging** for gradient: descent hue, neutral flat, climb hue (11 steps). Custom properties `--seq-0…11` and `--div-0…10` in the theme block, light and dark (F-14 D-9), read like the other map colours; a scheme change repaints the layer and the legend follows through CSS classes. Constraints: neighbouring classes are distinguishable and the order reads correctly with red–green colour-vision deficiency (the blue–yellow axis carries the order, lightness separates yellow from red); every class stands out from the map in its theme (no pale yellow on the light map, no dark blue on the dark one: light and dark sets differ); no class is confusable with `--route` (magenta: "no data"), so the red end leans orange-red. `--range` is no constraint any more: with colouring on, the range draws the class colours (C-9). The engineer proposes the values; the operator reviews them on screen in both themes | The single-hue palette made neighbouring classes look alike; one home per colour (F-14) |
| C-6 | **Selector and legend: one map control, lazy.** `map-view` adds it at MapLibre's `top-left` in every layout (user, 8 Nov 2026; the pin toggle took the top-right stack under fullscreen); the attribution keeps bottom-right, or the top row left of the zoom in the two-column layouts (F-13 D-18). A native `<select>` (F-14 D-8's reasoning): "plain route" first and default, then the offered metrics in AP-9's order: speed (labelled **pace** for the pace sports by the workout sport, as F-13 D-4: `multisport` shows speed), heart rate, power, gradient. No offered metric: no control. The legend sits under the select only while a metric is on: the swatches, equal width whatever their value span, the bar **no wider than today's 140 px** (so the control does not grow on a phone); **at most three labels** for the relative metrics (lowest edge, median edge, highest edge, marked ≤ / ≥ at the ends) and **at most five** for gradient (a subset of its edges, 0 % implied by the flat middle), none overlapping at a 320 px wide map; the unit (beside the metric select, not under the legend); and a "no data" swatch in the route colour when a drawn edge has no class. Texts through `lib/format` with the units store, rewritten on a units change (AC 3, AC 4). Swatches are coloured by CSS classes, never `style` attributes (ADR-032). The route hit test already ignores anything but the canvas (F-06 v1.2). Wording by the engineer (OQ-63 rule) | No byte before the map; no layout reservation, because it lives inside the map box |
| C-7 | **State.** The choice is held in `map-view` memory only (feature scope), kept across scheme and units changes and style swaps (the latest `wx-colour` data is re-applied on `style.load`, as `wx-range`), reset with a new `data` value. The edit page gets the same control through `RouteMap`, over the full draft route | One code path (F-13 hand-off to F-19) |
| C-9 | **The selected range with colouring on** (user review, 3 Nov 2026, option (a)): the range keeps the metric's class colours (edges without a value in the route colour, bridges as without colouring) over a thick outline, `wx-range-casing` (`--range-outline`); the route outside it fades to `--route-fade` (both light and dark). Colouring off: as before F-25. A range update partitions the kept per-sample classes over the range's edges only (O(log N + range), after F-17's settle); values and edges are never recomputed | The selection stays readable as a selection without hiding the colouring |
| C-8 | **Cost.** *Before the map:* nothing; all F-25 code is in the lazy map chunk. The static change is `RouteMap` passing the units store through. *At map start:* the plain route as today; deciding which metrics are offered is one pass per stream that stops at the first value on a positioned sample, inside step 6 (`wx-map:route-model`). No classes at start-up. *On a selection:* values, edges, classes and geometry for that metric (the quantiles reuse the existing sort; at most 12 features), then one `setData`; caused by the viewer, off the scrub path. *Scrubbing:* nothing added: F-06 D-11 stands (DOM marker, no `setData`, no repaint); no colouring code subscribes to the cursor or the selection | AC 5 |
| C-10 | **Smoothing unchanged** (reviewed 7 Nov 2026): heart rate stays as recorded, speed 10 s, power 30 s | The complaint is about efforts of minutes, which no window blurs; heart rate already lags the effort, a window would add to it. If the operator sees sprints lost under power, lower `POWER_WINDOW_S` (one constant) |
| C-11 | **Class numbers.** `lib/colouring` keeps `NO_CLASS` (255, fits `Uint8Array`). On the map, feature property `c` is 0–11 sequential, 12–22 diverging, and the range's "no value" class follows them; the light fallbacks in `map-style` follow page.css. No chart uses the classes | One layer, one `match` |

**Values per metric** (C-3):

| Option | Offered when | Value per sample | No class when |
| --- | --- | --- | --- |
| Speed (pace for pace sports) | the speed stream has a value at a positioned sample | centred mean over 10 s of time | missing; pace sports: **raw** speed below `STOP_SPEED`, removed before the mean so a stop never slows its neighbours (DS-2: a stop has no pace; QA-25-r1-2). Speed sports: a stop is a real 0 and takes the slowest class; the percentile base leaves out samples below `STOP_SPEED` |
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
| Static | The units prop wiring plus C-5's palette properties in the static theme block (253 bytes gzip measured at v0.1, QA-25-r1-1). v0.2 grows them from 14 to 23 per block (three blocks): delta **at most 600 bytes** gzip, from the reserve (2,601 → at least 2,000, above ADR-011's 1,000 trigger). Above: stop and report; the fallback is moving the palettes into the lazy map CSS, which needs the F-14 D-9 hygiene rule changed |
| Lazy | F-25's own lazy delta at most **3,000** bytes gzip, reported beside F-06's lazy figure. Above: stop and report |

## Verification

| AC | Automated (`make ci`) | Manual (operator) |
| --- | --- | --- |
| 1 | Offered set on synthetic data: heart rate without power; gradient only with altitude and distance | A corpus ride |
| 2 | Existing presence tests; the control exists only inside a mounted map | Indoor and fully hidden samples |
| 3 | Classes are monotonic in the value; on spread data each class holds about a twelfth of the drawn edges; ties leave classes empty, never out of order; a constant metric gives one class; legend labels in both unit systems | Corpus ride, units switched with colouring on; the feedback ride by heart rate: the steady flat and the climb in clearly different colours; legend readable on the phone |
| 4 | Label and legend per pace sport and unit system (run, swim, row) | A corpus run |
| 5 | Review: no colouring code on the cursor or selection path | ADR-016 scrub record on `ride-12h` on the phone, colouring by speed and by gradient |
| 6, 7 | Partition property: every coloured edge is a recorded edge; no vertex from a sample without a position; bridges never in `wx-colour` (golden `gps-dropout`, `pause-hours`, a hidden-ends variant) | `gps-dropout` and a sample with hiding on |
| C-3 rules | Gradient: standstill and held distance give no class, never ±∞; transition samples give none for speed, power and gradient, and a class for heart rate | Gradient on a hilly corpus ride; palettes in both themes on both map styles, with a colour-vision-deficiency simulation |

## Hand-offs

| To | What |
| --- | --- |
| F-26 | The control and legend are part of the map container and follow it into fullscreen (OQ-99 decided; F-26 FS-3). `wx-range` stays above `wx-colour-line` (F-17) |
| Requirements | TD Code structure: add `lib/colouring` to the layout and the front-end dependency rules; `components/map` may import `lib/units` |

## Open questions

| ID | Question | Suggestion |
| --- | --- | --- |
| OQ-107 | The user decided 12 classes; gradient gets 11 (C-4), because an even count has no flat middle class. Accept? | Accept; the alternative is 12 with two near-neutral classes either side of 0 % |
| OQ-99 | AP-15's fullscreen keeps "the route, markers, zoom controls and an exit button"; the colour selector and legend are not listed. Keep them in fullscreen? | Decided 3 Nov 2026: keep both (decision log) |
