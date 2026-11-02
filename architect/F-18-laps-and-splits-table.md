# F-18 Laps and splits table: architecture

- **Version:** 0.2 (OQ-75 to OQ-80 decided)
- **Status:** accepted (29 Oct 2026; OQ-75 to OQ-80 decided as suggested, decision log)
- **Date:** 29 Oct 2026
- **History:** [history/F-18-laps-and-splits-table.md](history/F-18-laps-and-splits-table.md)
- **Feature:** [`features/F-18-laps-and-splits-table.md`](../features/F-18-laps-and-splits-table.md)
- **Requirements:** AP-8, AP-7 (row selects range), AP-17, DS-1, DS-2; TD Front end (Load order step 1); decision log OQ-28 (columns, provisional), 27 Oct 2026 (panel above the laps)
- **Builds on:** [F-03](F-03-workout-data-file-writer.md) §5.3, §5.4; [F-13](F-13-activity-page-layout.md) D-5, D-8, D-10, hand-off "Backend, before F-18"; [F-14](F-14-units-and-theme.md) (laps rewritten in place); [F-17](F-17-range-selection-and-axis-toggle.md) R-1, R-2, R-4, R-9 to R-11 and its F-18 hand-off; ADR-005, ADR-008, ADR-011, ADR-026, ADR-027, ADR-029. No new ADR
- **Owner:** both. Backend step B-1 first (computed lap values, laps module without device laps), then the frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| L-1 | **Lap values are completed on the server.** Per key, a lap keeps the device's value; where the device recorded none, `internal/workout` computes it with F-17's range values table over the lap's range (L-2), using the same `M` and `G` series (ADR-029). Keys completed: the AP-8 columns only: `elapsed_time`, `distance`, `ascent` (= gain), `avg_speed` (distance ÷ moving time; absent when the moving time is 0), `avg_heart_rate`, `avg_power`, `avg_cadence`. Not: `timer_time`, `descent`, `calories`, maxima, temperature. A lap with fewer than 2 samples gets nothing computed. A computed `ascent` is absent ("–", never 0) when the lap's range has no valid altitude sample (AP-8; F-17 "no samples → –"); device-recorded values stay. Keys that F-08 R-13 removed from transition laps (speed, cadence, power) are not refilled; they show "–", as in the summary (OQ-78). Splits that cross a transition still count it as a range does. Computed from the normalised streams before hiding; hiding changes no lap value (F-16 AC 2, 3 test stays). **Format version stays 2**: the key set and schema are unchanged, only the meaning of a lap total (ADR-005 item 9 bumps on schema changes). A kept old workout shows "–" until it is rebuilt; accepted (workouts are short-lived) | F-13 D-5 and ADR-026 item 3: the template prints, never computes; AC 6 needs the values without JavaScript. One computation for page, edit page and units rewrite |
| L-2 | **Lap → range:** `a = start_index`, `b = end_index − 1` (F-03 §5.4; the shared boundary sample is included). A row is selectable only when `b > a`; otherwise it is shown but a tap does nothing. Rows map to `meta.laps` by DOM order; the enhancer attaches only when the row count equals `meta.laps.length` (as F-14's units enhancer checks), so the view model and the Go builder do not change for this | F-17 R-1 needs `a < b`. No index strings in the template |
| L-3 | **Row tap.** One delegated `click` listener per table (no click after a scroll or a horizontal pan of `.laps-scroll`) writes `(a, b)` to the selection store. A tap on the row already selected does nothing; rows never clear (AP-7's clear list is closed). No cursor change, no map refit (F-06: no auto-pan). The panel then shows the range values from the samples, which may differ from the row's device values; no note (OQ-79). The row whose range equals the store value is marked (background only, no size change); the enhancer remembers the marked row and compares the store value only with it, so a drag frame costs O(1) (F-17 R-11) | AC 5. The hot-path rules stay intact |
| L-4 | **Splits in the browser** (feature scope), boundaries from a pure function in `lib/range` over its carried distance `d̄`. `L` = 1,000 m or 1,609.344 m. `d0 = d̄[0]` (the first valid distance). Boundary 0 is sample 0; boundary *k* is the first sample with a valid distance and `d − d0 ≥ k·L`, with `k·L` computed per *k*, never accumulated (a sample at exactly 10,000 m ends split 10). Split *k* is `[boundary k−1, boundary k]` (adjacent splits share a sample, like laps). The last partial split is `[last boundary, N−1]` when its range distance is > 0; a tail where the distance does not advance is in no split. Row values are `lib/range` queries; the distance shown is the range distance. Computed once per unit system, on the first switch to splits and on a units change while built; O(N), never at load or on an update path. A sample that jumps over several multiples of `L` ends one split only (no empty splits); the row number then no longer equals the kilometre (QA-18-r1-2) | AC 1, AC 3. The stream never decreases (F-08 R-14), so each boundary is crossed once |
| L-5 | **Splits are formatted by the laps builder.** Each split's range values become a `Totals` object (`elapsed`→`elapsed_time`, `gain`→`ascent`, `speed`→`avg_speed`, `NaN` → key absent) and go through `lib/page`'s `lapsGroup`. Columns, units, pace or speed and "–" therefore follow F-13 D-5 and F-14 by construction. The two views may show different column sets (each has its own header). The mapping lives in the enhancer, since `lib/page` does not import `lib/range` | One formatting home (F-13 D-6) |
| L-6 | **Switch and splits table.** A bundle-only "Laps / Splits" control (wording by the engineer, OQ-63 rule) in a new empty slot in the laps heading row (template change, mirrored in `components/page`). The heading row's height is fixed by CSS, so inserting the control moves nothing (F-13 D-10, ADR-026 rev. 1). Shown only when the distance advances (`d̄[N−1] > d̄[0]`, O(1)); otherwise there is no splits option (AC 4). Default: laps, held in memory only, never in `lib/prefs` (OQ-76). The splits table is built by the enhancer inside the same module, with the server table's classes; the switch toggles `hidden` between the two. A switch may change the module's height (viewer-caused, F-13 D-10). On a units change the laps table is rewritten in place as today; a built splits table is rebuilt (km and mi give different rows). The selection is untouched by a switch or a units change; the row mark follows L-3 | AC 1, AC 4, AC 6. Server markup adopted, not re-rendered (ADR-026 item 4) |
| L-7 | **Tapped row stays in place** (OQ-75). When a row tap makes the panel appear (no range → range), the enhancer reads the row's viewport top before the store write and again in the same selection flush, after the glue's slot-visibility listener, and scrolls the window by the difference before paint. All three layouts (the slot is above the laps in each). Native scroll anchoring is turned off for the document (`overflow-anchor: none`), so only one mechanism moves the scroll. Two layout reads per tap; never on a drag. Rows become tappable only once the range panel is mounted (after the charts' first paint), so the slot listener always precedes the L-7 read (QA-18-r1-3) | Native anchoring helps only when the slot is above the top of the screen. When the slot is on screen, the anchor is the chart above it and the row moves down by the panel's height in every browser. WebKit support for scroll anchoring is unconfirmed. On the phone the panel may then sit above the screen (its sticky place is above the laps); accepted (OQ-80) |
| L-8 | **Laps module without device laps** (OQ-77; replaces F-13 D-5's "no laps module when the file has no laps"). The server renders the module when the file has laps **or** its distance advances (L-6's condition, evaluated identically by `lib/page` and Go's `internal/page`). Without laps the module holds the line "No laps recorded." instead of the table, and the switch (L-6) offers splits; the page still starts on laps. The view-model shape (for example a test string for the empty case) is the engineer's choice; a synthetic sample with distance and no laps pins it | A truncated recording loses its lap messages but keeps its distance. Starting on splits would insert rows after load (F-13 D-10) |

**Module boundary** (F-17's table, extended):

| Module | Adds | May import |
| --- | --- | --- |
| `lib/range` | split boundaries | unchanged |
| `src/page` (laps enhancer) | row taps, row mark, switch, splits table, L-7 | + `lib/range`, `lib/selection`, `lib/page` (`lapsGroup`), `lib/units` |
| `internal/workout` (Go) | lap completion (L-1) | unchanged |

F-03 §5.4's index rule keeps one Go implementation. If L-1 needs it in `workout`, it moves there and `datafile` calls it.

## B-1 Backend step

- `internal/workout`: L-1, with tests: per key, device value kept; missing key equals a brute-force evaluation of the range table; laps of 0 and 1 samples; a transition lap; hiding on and off identical.
- `internal/page` and `lib/page`: L-8 (module present with laps or advancing distance, the "No laps recorded." case), with a new synthetic sample (distance, no laps); Go template test and `components/page` follow.
- Update F-03 §5.3's Lap meaning ("device value where recorded, else computed, F-18 L-1"), regenerate the goldens and the page samples (`lib/page` writes them; the Go builder must reproduce them) in one commit.
- Cross-language check (frontend test, same commit or the frontend step): on a golden whose laps carry no device values, `lib/range` over each lap's `[a, b]` equals the lap totals within the stored streams' quantisation step plus the 3-decimal rounding.

## Budget

| What | Value |
| --- | --- |
| Share | 1,500 bytes gzip (ADR-011 rev. 9); follow-ups count against it. Estimate about 1,200: enhancer (taps, mark, switch) 450, boundaries and mapping 300, splits table 200, L-7 100, CSS 150. Measure early with a temporary import |
| Over the share | Up to 500 from the reserve (3,564), reported, while the reserve stays ≥ 1,000. Beyond that, stop and report |

## Verification

| AC | Automated (`make ci`) | Manual (operator) |
| --- | --- | --- |
| 1 | `lib/range` boundaries: 10.4 km → 10 + 0.4 km, 6 + 1 mi; a sample at exactly k·L; flat stretches and missing distance; no zero-distance tail row | — |
| 2 | Go L-1 tests; sample with 5 device laps: cells equal the device values | — |
| 3 | jsdom: one lap row and a working switch | — |
| 4 | jsdom: no switch without distance, or when it never advances | — |
| 5 | jsdom: a tap writes `(start_index, end_index − 1)`; a 1-sample lap writes nothing; the mark follows the store | Desktop and phone: charts, map and panel show the lap |
| 6 | Go template test: laps table present, no switch in the server markup | JavaScript off |
| L-8 | Samples and Go: the no-laps sample has the module with "No laps recorded."; a file with neither laps nor advancing distance has no module. jsdom: the switch offers splits there | — |
| L-7 | — | Desktop (Firefox, and Safari or iPad if available) and phone, slot on screen and above it: the tapped row does not move |

## Hand-offs

| To | What |
| --- | --- |
| Requirements | F-18 Owner becomes both (backend first); the feature scope gains L-8's no-laps case. TD glossary "Session / lap" and "Device totals": lap totals are the device's values where recorded, else computed as for a range (AP-8) |
| F-19 | The enhancer runs on `components/page` markup with the same heading slot |
| F-24 | Laps in the viewer export: decide whether computed lap values are written as lap totals (they would look device-recorded) |
| F-30 | Regeneration completes the lap values of existing workouts; nothing extra |

## Open questions

None. OQ-75 to OQ-80 were decided as suggested on 29 Oct 2026 (decision log; the questions are in the history file).
