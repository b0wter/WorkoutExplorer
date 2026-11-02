# F-17 Range selection and x-axis toggle: architecture

- **Version:** 0.3 (QA r1: `M` anchored at 0, stored split restored before the load, ledger closed at the measured figure)
- **Status:** accepted (26 Oct 2026)
- **Date:** 26 Oct 2026
- **History:** [history/F-17-range-selection-and-axis-toggle.md](history/F-17-range-selection-and-axis-toggle.md)
- **Feature:** [`features/F-17-range-selection-and-axis-toggle.md`](../features/F-17-range-selection-and-axis-toggle.md)
- **Requirements:** AP-5, AP-6, AP-7, NFR Performance (60 fps on 12 hours); TD Front end (Implementation notes: prefix sums); decision log OQ-23, OQ-24, OQ-25, OQ-72, OQ-73, OQ-74
- **Builds on:** [F-05](F-05-stacked-charts-with-shared-cursor.md) (D-5, D-6, D-10, D-17, D-18, gesture reducer), [F-06](F-06-route-map-linked-to-cursor.md) (D-7, the `wx-` merge), [F-08](F-08-fit-validation-and-recovery.md) (F-17 hand-offs: R-13.M, R-14, D-19), [F-13](F-13-activity-page-layout.md) (D-8, D-10, D-17, D-19), [F-14](F-14-units-and-theme.md) (D-3, D-9 to D-11), [F-16](F-16-start-and-end-hiding.md) H-1, ADR-005, ADR-007 to ADR-011, ADR-013, ADR-026 (item 3), ADR-027; new: [ADR-029](ADR-029-cumulative-moving-time-and-gain-in-the-data-file.md)
- **Owner:** both. Backend step B-1 first (format v2 and the page token), then the frontend. The writer, the decoder and the goldens land in one commit (ADR-005 item 9)

**Layout note (user, 27 Oct 2026; overrides R-9's placement where they differ).** From 1024 px with a map, the panel sits in place above the laps, not sticky; the laps move down when it appears (accepted). On short landscape screens (below 1024 px wide and at most 500 px tall) with a map, the page has two columns, 40 % map and 60 % content: the map box is `position: fixed` on the left at `100lvh` (with a `100vh` fallback), `.layout` stays one block flow with the content padded to the right, so the range slot's containing block reaches above the charts and the panel sticks to the bottom of the right column. No pin toggle there, and the pinned-map rules apply only to portrait or taller windows; the readout is the 4-column card (F-13 readout note); the map keeps the phone's credit collapse, with the credits at the top as on desktop. Portrait phones keep one column, the pin toggle and the bottom-sticky panel. CSS only, no device detection.

**Splitter note (user, 28 Oct 2026; changes F-13 D-17's fixed 1:1 and the 40 % above into defaults).** Both two-column layouts get a splitter (`src/page/splitter.ts`, bundle-only, a sibling of RouteMap's container in `#map-slot` like D-19's toggle): dragging sets `--map-w` on `.layout` to 30–70 % of its width (desktop still bounded by the 320 px and 608 px column minimums), a double click or double tap restores the default. *(29 Oct 2026, AP-17: the dragged split is remembered in `localStorage` through `lib/prefs`, per layout, desktop and phone landscape; the reset clears it.)* *(29 Oct 2026, QA-17-r1-2: the stored split is applied before the first `await` of the page glue, not with the splitter after the load; the splitter element itself still arrives after the load. The remaining shift is accepted, also on a cached reload (user, 29 Oct 2026; module scripts run after the first paint); a later fix may be an inline `<head>` script setting `--map-w`, allowed by its CSP hash in F-22 (F-13 D-10).)*

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| R-1 | **Selection store** `lib/selection`: a range of two sample indices `a < b`, both inclusive, or none. It follows the cursor store's rules (ADR-008 items 2 to 5): framework-free, an invalid value throws `RangeError`, delivery coalesced to one flush per frame, a synchronous first call on subscribe, listener errors caught per listener, listeners never write. Neither a write nor a flush allocates. One store per `WorkoutData`, created by the page glue. Writers: chart gestures, the panel's clear control, F-18's rows. Readers: charts, map, panel, the glue (panel visibility, R-9). The map never writes it (OQ-73) | Indices, as for the cursor: every view (time axis, distance axis, map, laps) maps them without a search. A drag updates every frame, so the store needs the cursor's hot-path rules |
| R-2 | **Range values are defined once** (table below). They are also the rules for the server's computed lap values (F-18) | One definition for the panel, the splits and the laps |
| R-3 | **Moving time and gain come from the server** as cumulative streams in data file format v2 ([ADR-029](ADR-029-cumulative-moving-time-and-gain-in-the-data-file.md)). The browser takes differences and holds no moving-time or gain rule | Resolves the feature file's architect note. R-7 needs timer events, which the data file does not carry (ADR-005 item 7). ADR-026 item 3: values the page lacks are added by the server |
| R-4 | **`lib/range`**, pure: built once per `WorkoutData` at full resolution (prefix sums of value and count for heart rate, power, non-zero cadence and temperature; distance carried forward), and queried in O(1) into a reused result. It is built after the charts' first paint, never on the scrub path, never per update | AC 2 and AC 5. All stored values are integers in their scale, so the sums are exact in a double |
| R-5 | **Gestures** (table below), added to F-05's pure reducer. ADR-010's rows stay as they are. The long-press timer lives in `MetricChart` and reaches the reducer as an input. `blocksScroll` also holds while a range is being selected (the case ADR-010 foresaw under "Would choose differently if"). On charts, the touch callout and the touch `contextmenu` are suppressed | AP-7 (touch: long-press, then drag), AP-6 kept |
| R-6 | **Shading:** one overlay element per chart in `u.over`, placed from `valToPos` at the range's ends, like the cursor line (ADR-009 item 4). uPlot's `select` stays off. It is placed again after a resize, a unit or scheme rebuild (F-14 D-11) and an axis switch | No canvas redraw while dragging |
| R-7 | **Distance axis.** A sample's x is its distance. A sample without distance has no x and is a break in every chart. Time gaps (≥ 5 s) stay breaks. A stretch where the distance does not advance has no width (F-08 R-14): it folds into one pixel column, with its min and max kept. Decimation accepts non-decreasing x with missing values and emits non-decreasing x (equal values allowed). F-05's other properties hold, and the time axis is unchanged. The x range is the first to the last valid distance, the same on every chart (F-05 D-6). The pointer maps to the sample nearest its distance (earliest on a tie, decision 5), through a lookup built once per data value, with no allocation. On a sample without distance the cursor line is hidden and the readout is unchanged. Ticks are round numbers in km or mi (`lib/format` with the units store) | AP-5. The browser does not repair distance (feature scope) |
| R-8 | **Toggle.** A bundle-only "Time / Distance" control in a toolbar row at the top of the charts module, above the readout, rendered by `ChartStack`. The axis is `ChartStack`'s own state, because nobody else reads it. The default is time. *(29 Oct 2026, AP-5: the page passes a toggle choice remembered in `localStorage` through `lib/prefs` as the start axis; a workout without distance shows time and leaves it alone.)* A switch rebuilds the uPlot instances on F-14 D-11's path, and the stores are untouched, so the cursor stays on the same sample and the range stays shaded. The page CSS reserves the row only when `ChartsClass` has the new token `has-distance`, which `lib/page` and Go's `internal/page` set when the data file has a distance stream (samples updated). If the distance never advances, the row stays but the control is not shown | AC 1. Nothing moves when the control arrives (F-13 D-10, ADR-026 item 4). The token states the fact explicitly instead of deriving it from the `cells-N`/`charts-N` combination |
| R-9 | **Panel** (`components/range`), mounted by the glue into `#range-slot` (F-13 D-8) after the charts' first paint. **Hidden on every device while there is no range**: the slot keeps F-13's "zero height until filled", nothing is reserved. It shows when a range appears and hides when it is cleared; only these two transitions touch visibility, never a frame of a drag. Content below the slot moves then; that is caused by the viewer (F-13 D-10's user-action rule) and accepted (OQ-74). The fields are fixed per workout (the table's "shown when" column), in a fixed grid, so the panel's height does not change while a range is dragged. The clear control is in the panel. The listener writes text only (F-05 D-10). It re-renders on a units change. On a load failure the slot stays empty. **Placement** (OQ-72), CSS only, decided by the layout and so by width (F-13 D-17), no device detection: in **two columns** the slot stays in place in the right column. In **one column** (below 1024 px, or any width without a map box) the slot is `position: sticky; bottom: 0`, so while a range exists the panel sits at the bottom of the screen until its own place below the charts scrolls into view. As with D-19's pinned map, its containing block must reach up past the top of the charts (a one-column grid row does not). No viewport units. With the map pinned and the readout stuck, at least one whole chart stays visible between the readout and the panel on the reference phone, portrait and landscape, bars shown (D-19's constraint, now with the panel); the engineer makes the one-column panel compact enough for that | User decisions OQ-72, OQ-74. A sticky panel keeps the values in view while dragging on an upper chart on the phone. Sticky, not fixed: it keeps the document order and never covers the laps or the footer |
| R-12 | **Hint.** One line under the charts, rendered by `ChartStack`, always shown with the charts (with or without a range), saying how to select: drag; on touch, long-press then drag. The wording is chosen by the `pointer: coarse` media query in CSS, not by script or device detection; the engineer proposes both texts (OQ-63 rule). Its row is reserved with the chart area whenever the page has charts (the `charts-N` class, F-13 D-10), one fixed line per breakpoint, clipped to one line at the narrowest width. Bundle-only (no charts without JavaScript); on a load failure it collapses with the chart area | OQ-74: long-press is not discoverable. The reserved row keeps CLS 0 at load |
| R-10 | **Map highlight.** `RouteMap` gets the selection store, and the lazy `map-view` subscribes to it like the cursor. It draws a `wx-range` source with its layers above the route and below the marker, coloured from the theme's properties. The merge keeps it across style swaps, and the style-spec validation test covers it (ADR-013 item 7). The geometry covers the positioned samples in `[a, b]`, split into recorded segments and bridges by F-06 D-7's `lib/route` rule over the sub-range. A hidden section has no positions, so only the visible part is highlighted (AC 3). It is updated with **one `setData` 150 ms after the range stops changing**, never once per frame during a drag | Rebuilding the geometry is O(range length), and it costs a worker round trip. The charts and the panel show the drag live. Would choose differently if the phone shows a 12-hour `setData` under 4 ms; then throttle to live updates |
| R-11 | **Hot path.** One frame of a drag runs the selection flush: one shading update per chart, about 9 panel texts and one O(1) query. It allocates nothing except short strings, reads no layout and redraws no canvas. No code that runs on an update loops over the samples | AC 5, AC 6, ADR-016 unchanged |

**Range values** (the range is `a < b`, sample indices, both inclusive; `t` time, `d` distance, `M` and `G` the ADR-029 streams):

| Value | Definition | Shown when the workout has |
| --- | --- | --- |
| Elapsed time | `t[b] − t[a]` (pauses included, D-19) | always |
| Moving time | `M[b] − M[a]` (transitions count as moving, R-13.M, already in `M`) | `M` |
| Distance | `d̄[b] − d̄[a]`. `d̄[i]` is the valid distance at or before `i`, or else the first valid distance. Never negative; 0 inside a flat stretch | distance |
| Gain | `G[b] − G[a]`; "–" when the range has no valid altitude sample (QA-18-r1-1, as the server's lap gain) | `G` (altitude) |
| Average speed or pace | distance ÷ moving time; "–" when the moving time is 0. Speed or pace, and units, as in the summary (F-13 D-4, F-14) | distance and `M` |
| Average heart rate, power, temperature | mean of the present samples `a…b`, per sample, not weighted by time (power counts zeros) | that stream |
| Average cadence | mean of the present samples `a…b` that are not 0 | cadence |

A pause is a gap and has no samples, so it adds nothing to any average. Transitions have no speed, cadence or power samples, so they drop out by themselves (F-08 hand-off). A value with no samples in the range shows "–", never 0. Selecting the whole workout may differ from the summary (OQ-24). Moving time matches the summary, and gain is R-8's derived value, not the device's ascent.

**Gestures:**

| Input | Result |
| --- | --- |
| Mouse: press, move past a small slop, release | Range from the press sample to the pointer sample, updated live; the cursor follows the pointer |
| Mouse click, or touch tap (up without scrub, pan or long-press), on a chart | Cursor as today. If the sample is outside the range, the range is also cleared. A range is cleared only on up, never on down. Besides the panel's clear control, this is the only way to clear (OQ-73): map taps, other taps and scrolls never do |
| Touch: hold within `TOUCH_SLOP` for `LONG_PRESS_MS` = 400, then drag | Range anchored at the touch-down sample, following the finger, page scroll blocked until up |
| Touch: scrub or pan before the long-press fires | As in F-05 (ADR-010); the range is untouched |
| `pointercancel` while selecting | Range and cursor restored to what they were before the touch |
| A drag that ends with `a = b` | No change to the range |

400 ms is assumed. The phone checks confirm it.

## Module boundary

| Module | Adds | May import |
| --- | --- | --- |
| `lib/selection` | selection store | nothing (it may share the coalescing helper with `lib/cursor`) |
| `lib/range` | range statistics | `lib/datafile` |
| `lib/charts` | gesture states, distance-axis decimation and lookup | unchanged |
| `components/charts` | toolbar, toggle, shading, hint | + `lib/selection` |
| `components/range` | `RangePanel` | `lib/range`, `lib/selection`, `lib/units`, `lib/format`, `lib/datafile` (types) |
| `components/map` | selection prop; `wx-range` (lazy only) | + `lib/selection` |
| `src/page` | one selection store per data value; mounts the panel | + `lib/selection`, `components/range` |

The chart, panel and map components never import each other; they meet in the stores.

## B-1 Backend step

- `internal/workout`: the cumulative series of ADR-029 item 1, with their invariants as tests on every accepted corpus file, synthetic and local real (ADR-029 rev. 1). Hiding (F-16 H-1) leaves them unchanged.
- `internal/datafile`: format version 2 with the two streams. Update F-03's normative section (stream names, scale, the per-sample size bound) and regenerate the goldens and fixtures. The frontend's decoder change lands in the same commit.
- `internal/page`: the `has-distance` token (R-8), reproducing the updated samples.

## Budget

| What | Value |
| --- | --- |
| Share | 4,000 bytes gzip (ADR-011), measured early with a temporary import. Estimate about 3,800: gestures, axis and shading about 1,700; panel and CSS about 1,000; statistics and store about 700; toggle, decoder and glue about 400. The map part is lazy (0 counted); its lazy delta is reported against F-06's 5,000 |
| Over the share | Up to 500 from the reserve, reported, while the reserve stays ≥ 1,000 (F-14 precedent). Beyond that, stop and report. The first remedy is to load `RangePanel` with `import()` on the first range |
| Ledger | Measured at F-17 QA r1: entry 54,936, F-17 with its follow-ups (splitter, prefs, readout restyle) 4,715 against F-14's closing entry of 50,221 (ADR-011's base; QA's 4,716 used 50,220). The 715 over the share are accepted (more than the 500 allowance; ADR-011 rev. 9). Reserve 3,564 after F-18's 1,500. The r2 fixes may add at most 100 more; the figure at F-17 close is final |
| Data file | +8 raw bytes per sample. The compressed growth is measured on `ride-4h` and `ride-12h` and reported (ADR-029) |

## Verification

| AC | Automated (`make ci`) | Manual (operator) |
| --- | --- | --- |
| 1 | Samples and Go: `has-distance` appears only with distance. jsdom: no toggle without distance; after a switch, the cursor index and the range are unchanged | Desktop and phone |
| 2 | `lib/range` against a brute-force reference on seeded data (missing runs, gaps, flat and missing distance, zero cadence and power) and on the goldens. Distance, moving time, gain and elapsed time of `[a, c]` equal `[a, b]` + `[b, c]`. Go: `M` and `G` invariants | A climb on a corpus ride |
| 3 | `lib/route` sub-range: no vertex from a sample without a position. jsdom with the recording fake: `wx-range` data | A published sample with hiding on |
| 4 | jsdom: clear removes the shading and the `wx-range` data and hides the panel; the panel is hidden before the first range | — |
| Layout (R-9, R-12; F-13 D-10) | jsdom: the hint is present with charts and absent on a load failure | Desktop: CLS 0 at load with the hint; the panel appears in place in two columns. Phone: while dragging on the top chart the panel sits at the bottom of the screen; with the map pinned one whole chart stays visible; a map tap keeps the range |
| 5 | Review: no loop over the samples on an update path | Phone drag on `ride-12h` |
| 6 | Reducer: a scrub with a range leaves the range unchanged | Phone: scrub with a range. Also on the phone: long-press selects; no callout or menu; a vertical swipe still scrolls; a scroll that starts on a chart does not clear; a cancel restores |

## Hand-offs

| To | What |
| --- | --- |
| F-18 | Rows set the store (R-1). The lap-to-indices mapping is F-18's. Splits use `lib/range`. The range values table is the rule for the server's computed lap values (F-13 hand-off "Backend, before F-18"), on the same `M` and `G`. The first row selection makes the panel appear above the laps and moves them down by its height (R-9, accepted with OQ-74); whether F-18 keeps the tapped row in place is F-18's decision |
| F-19 | Same stores, panel and toolbar. `draft.bin` is v2 |
| F-25, F-26 | `wx-range` stays above the route colouring and is visible in fullscreen |
| Requirements | TD: the prefix-sum note becomes "distance, moving time and gain as differences of cumulative streams (moving time and gain from the server, ADR-029); averages from prefix sums built in the browser". The stream table gains two rows, and the size note uses 35 bytes per sample. Glossary: "Range", "Selection store". F-17 Owner: both |

## Open questions

None. OQ-72 to OQ-74 were decided on 26 Oct 2026 (decision log).
