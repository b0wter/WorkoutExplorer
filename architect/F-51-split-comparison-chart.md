# F-51 Lap and split comparison chart: architecture

- **Version:** 0.1
- **Status:** accepted (14 Nov 2026). Normal lane. OQ-196 to OQ-209 decided by the user on 14 Nov 2026 (decision log); no open questions
- **Feature:** [`features/F-51-split-comparison-chart.md`](../features/F-51-split-comparison-chart.md)
- **Requirements:** AP-23, AP-7, AP-8, AP-12, AP-13, AP-19, AP-20, UP-6, DS-1, DS-2, DS-4; NFR Performance, NFR Reliability
- **Builds on:** [F-46](F-46-laps-and-splits.md) LS-4, LS-7 to LS-9; [F-48](F-48-pool-strokes.md) PL-8; [F-18](F-18-laps-and-splits-table.md) L-2, L-3, L-7; [F-49](F-49-zones.md) zone colour tokens; ADR-009 items 6, 7; ADR-011; ADR-026. No server, format or template change
- **Owner:** frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| SC-1 | **Plain SVG, no uPlot.** The chart is SVG built in the `src/splits/` chunk. uPlot is not used: ADR-009 item 6 keeps it inside `components/charts`, its `paths.bars` is blanked at build (ADR-009 item 7) and re-enabling it would grow the *entry*, since uPlot lives there; a canvas also needs its own hit-testing and a redraw on every theme switch. At most 200 bars plus labels is cheap DOM | Smallest cost: zero entry bytes for drawing; colours follow the theme through CSS alone |
| SC-2 | **Where it lives.** Drawing, the overlay drop-down, hover and taps in `src/splits/` (the laps module's lazy part, LS-4, PL-8). The rules in a new pure module `lib/compare` (imports `lib/datafile` types only, tested in Node): slots, which rows count, reference, best, class per bar, scales, overlay offering (SC-3 to SC-6). The import condition of `src/splits/` is **unchanged** (distance advances or lengths exist), so a workout with neither gets no chart | Pure decision logic apart from the DOM; no new entry code for loading. A laps-only workout without advancing distance has no lap speeds in practice |
| SC-3 | **One source of row values.** The chart reads exactly the per-row values its table is built from: Laps `data.meta.laps[].totals` (the device values the static table shows, ADR-026), Splits the range values `splitsTable` already computes for the current preset, Lengths `meta.lengths[].avg_speed`. Nothing is recomputed differently, so the chart cannot show what the table does not (AP-23). The page's `data` already is the table's (trimmed public file on the viewing page, `draft.bin` on the edit page), so trim and UP-6 need nothing extra | AC trim and edit rows; one home per value |
| SC-4 | **Slots and counting.** One slot per row in table order. Separator rows and Rest lengths take **no slot**; a row without speed (absent, not finite or ≤ 0) keeps an **empty slot**. Label = the row's number as the table shows it. Counted for reference and best: every row with a bar except the partial last split, i.e. the last split when the boundary rule ended it at the workout's end short of the preset (LS-5, LS-6 tolerance). Laps and lengths have no partial row. No chart when `meta.sport` is `multisport`, when fewer than 2 bars, or when more than 200 slots | OQ-196, OQ-200, OQ-203, OQ-204, OQ-209 |
| SC-5 | **Computed in speed, shown as DS-2.** Reference = median of the counted rows' speeds in m/s (even count: mean of the two middle values); best = the highest counted speed. A bar is **on pace** when its speed is within ±3 % of the reference, inclusive; faster above, slower below. Header, axis and band are then converted to the shown quantity: speed in the units setting, or pace (per km / mi, per 100 m / yd for swims) for the DS-2 pace sports. The value axis is linear in the shown quantity, inverted for pace so faster is always up, symmetric around the reference, its half-range the larger of the furthest drawn bar end and the band edge, so the band is always visible. The pace band is drawn from the converted edges, so it is slightly asymmetric and agrees with the colours | One rule for every sport, so "median" and "on pace" are unambiguous for the hand-computed AC (with an even count, median pace ≠ pace of median speed) |
| SC-6 | **Overlay.** Drop-down "Overlay": None, then Heart rate, Power, Cadence for each metric at least one barred row has; not shown when only None remains (lengths). Starts on None on every load, in memory only (never `lib/prefs`, storage or the fragment, as LS-7); a rebuild keeps the choice when the new rows offer it, else None. One dot per bar whose row has the value, joined by a line that breaks over slots without one; its own linear scale at the right edge; colour `--c-heart_rate`, `--c-power`, `--c-cadence` (page stylesheet) | OQ-201 |
| SC-7 | **Taps through the one tap path** (LS-8). A bar tap forwards a click to its table row, so the laps enhancer's one listener does the selection, the row mark and L-7's scroll fix; bars of rows that cannot be selected do nothing, as their rows. The **marked row is the one source of truth** for the highlighted bar: the LS-8 seam gains a notification from the enhancer's single mark step (row marked or unmarked), and the chart mirrors it. Hover on a bar highlights the bar and its row (a class distinct from the mark); no tooltip, no hover on touch | OQ-202; no second selection path. The seam is explicit and testable; a DOM observer on the mark class would couple to CSS silently |
| SC-8 | **Shared zone colours.** The `--zone-*` blocks (both themes) move from `zones/zones.css` into one stylesheet `src/theme/zones.css`, imported by both lazy chunks (`src/zones/`, `src/splits/`). Faster `--zone-red`, slower `--zone-blue`, on pace and the band `--zone-grey`. Fills by CSS class, so a theme switch needs no redraw (DS-4). The selected bar must be distinguishable without colour (an outline, for example) | OQ-206; one home for the tokens while keeping them out of the entry. The zones chunk loads only for workouts with zone values, so the splits chunk cannot rely on it |
| SC-9 | **Layout.** The chart is the last child of the laps section, after every view container (`build` appends containers, so it must re-place them before the chart or insert before it), outside the horizontally scrolling `.laps-scroll`. About 160 px tall on every screen; horizontal geometry in percent of the plot width (or equivalent CSS), so every bar fits and no resize listener or redraw is needed. Header line, legend (Faster, On pace, Slower) and the view caption ("Laps", "<preset label> splits", "Lengths") as HTML text, not SVG text. No `style` attributes in markup (ADR-032 CSP): geometry as SVG attributes, colours by class. The SVG is an image to assistive technology, labelled with the header and caption; bars are not in the tab order, the table stays the accessible path | OQ-208, OQ-209, AP-23 "under the table". The section is last before the footer, so drawing after the table moves only the footer |
| SC-10 | **When it is built.** With its table only: on chunk arrival, a switch to a view, a split-distance change, a units change (LS-9, PL-8), plus an overlay change. O(rows). Never on a cursor flush; a selection flush only updates the highlight in O(1) via SC-7 | AC 60 fps scrubbing and summary timing unchanged; ADR-008, F-17 R-11 |

**Module boundary** (F-46 and F-48 tables, changed):

| Module | Holds | May import |
| --- | --- | --- |
| `lib/compare` (new) | SC-3 to SC-6 rules | `lib/datafile` (types) |
| `src/splits/` (chunk) | as F-48, plus the chart, overlay drop-down, hover, bar taps | as F-48, plus `lib/compare` |
| `src/theme/zones.css` (new) | `--zone-*` tokens, both themes | none |
| `src/page/laps.ts` (entry) | the LS-8 seam's mark notification | as today |

## Cost

| What | Decision |
| --- | --- |
| Static (entry) | Ceiling **80 bytes gzip**: the seam's mark notification and any preload-map growth from the shared stylesheet. Measure the entry before and after and record it in ADR-011 item 4; above 80: stop and report |
| Lazy | `src/splits/` grows by the chart and `lib/compare`; report its gzip size before and after. Not counted |
| Data file, HTML | None |

## Hand-offs

| To | What |
| --- | --- |
| Frontend | The zones section must look unchanged after SC-8 (both themes). Wording of header, legend and caption is reviewed by the user on screen |
