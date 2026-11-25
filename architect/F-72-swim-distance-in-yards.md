# F-72 Swim distance in yards: architecture

- **Version:** 0.1
- **Status:** accepted (25 Nov 2026). Normal lane. OQ-348 to OQ-351 decided as assumed (feature file). Four assumed points F72-Q1 to F72-Q4 below, not blocking: the suggestion is built unless the user objects
- **Feature:** [`features/F-72-swim-distance-in-yards.md`](../features/F-72-swim-distance-in-yards.md)
- **Requirements:** DS-2, DS-1, DS-6; AP-2, AP-5, AP-6, AP-7, AP-8, AP-12, AP-13, AP-16, AP-20, AP-21, AP-23, PIN-8; NFR Reliability, NFR Performance
- **Builds on:** [F-14](F-14-units-and-theme.md) D-1, D-3, D-5; [ADR-027](ADR-027-units-rewritten-in-place-one-units-store.md); [F-44](F-44-pool-summary.md) PS-5; [F-46](F-46-laps-and-splits.md) LS-5, LS-6; [F-48](F-48-pool-strokes.md) PL-8; [F-64](F-64-laps-and-splits-per-multisport-leg.md) LL-3; [F-67](F-67-multisport-leg-picker.md) LP-3, LP-4, LP-10; [F-33](F-33-fullscreen-cursor-readout.md); [F-54](F-54-translated-interface.md) LG-7; ADR-011
- **Owner:** frontend only. No server, Go builder, template, data file or setting change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| YD-1 | **One rule, in `lib/format`.** The distance unit is a function of two inputs only: the unit system and the *scope sport*. Imperial and `swimming` give yards; otherwise km or mi as today. It sits next to `paceBasis` and shares one swim test with it. Every distance formatter, unit label and the distance axis scale take its result. No other module compares a sport with `swimming` to pick a distance unit. Factors and unit strings stay only in `lib/format` (F-14 D-1): the distance axis loses its own copy of the mile factor in `components/charts/uplot-options.ts` and takes factor and labels from `lib/format` | One place to change, one place to test. The axis copy is the one existing break of the D-1 rule |
| YD-2 | **Scope sport = the sport of what the surface shows.** Single-sport workout: `meta.sport` everywhere. Multisport (OQ-351): summary, whole-workout charts, readout, range panel, pins lists and added-laps table take `meta.sport` (`multisport`, so miles). A leg's laps or splits table takes its block's sport (`data-sport`, F-64 LL-3). The leg view takes `Look.view.sport` (F-67 LP-3). **The distance unit never follows the cursor's or the range's section.** LP-10's per-section speed and pace stay as they are. A readout scrubbing over the swim in the whole view therefore shows pace per 100 yd with distance in miles | OQ-351: the whole-workout view stays in miles. A readout must not switch units in the middle of a scrub |
| YD-3 | **Surfaces.** Switch: summary distance (AP-2, F-58 dialog included, as it reuses `summaryItems`); laps distance column (AP-8, every table built by `lapsGroup`); non-pool splits distance column (AP-20), with the preset label left metric ("100 m" / "109 yd", AC 3); range panel distance (AP-7); readout distance in both instances (AP-6, AP-21); distance axis ticks (AP-5); pins lists on the edit and viewing pages (PIN-8, F72-Q4); added-laps table (F-65). Unchanged: pace basis (already per 100 yd); pool length cell (PS-5); pool split labels **and** pool split distance column, in the pool's unit (LS-6, F72-Q1); Lengths view (it has no distance column, PL-8); split preset lists and labels (LS-5: metric in both systems); F-51 chart (no distance; its caption is the preset label); best-effort names (F-50); hiding radius (F-55, a privacy setting, stays ft/mi); map (no distance drawn); GPX, preview card and image, no-JS page (metric, ADR-027) | Feature scope. The pool and preset rules are already unit-independent and stay that way |
| YD-2a | **Edit page, workout trimmed to one sport.** The summary strip takes the server's trimmed sport (TR-9), so a triathlon cut to its swim reads yd there, as the published page will; charts, readout, range panel and pins keep the draft's `meta.sport` (miles) until published. Accepted: it previews the published summary, and one sport source would cost about 40 bytes of owner chunk (over the ceiling of 30) | QA-72-r1 question |
| YD-4 | **Metric output is byte-identical.** Yards occur only on imperial pages. Every `lib/page` builder therefore gives today's strings for `metric`, so the committed page samples and the Go builder (ADR-026 item 2) need no change. A changed metric sample is a defect | Keeps F-72 frontend-only |
| YD-5 | **Format.** Whole yards, rounded half up from metres ÷ 0.9144, then grouped; unit `yd`; NaN gives "–" as today. English: "," grouped by our own code, no `Intl` (LG-7). de/es/fr: one integer `Intl.NumberFormat` per language, cached as the decimal separator is, never built per call. Its grouping is CLDR's: "1.650" in German, "1 650" with a narrow no-break space in French, "1650" in Spanish below 10,000 (F72-Q3). Each cell is rounded on its own, so laps need not add up to the summary (as km today). The column header is the existing `colDistance` text with `yd`, so no new text | AC 1, AC 2, AC 7; one formatter shared by every surface |
| YD-6 | **Axis.** In yards the distance scale steps through whole yards only, on a round 1-2-5 or 1-2.5-5 sequence. Each tick is labelled like a summary value ("1,000 yd"). There is never a fractional yard label. X_SPACE stays unchanged. A leg view keeps labels counted from the workout start (OQ-323) | Same tick density as km and mi; "10,000 yd" fits 70 px |
| YD-7 | **Rebuild triggers stay the existing ones.** Units store change (ADR-027, F-14 D-5), language change (LG-4), and the leg view's key in `ChartStack` (LP-4). Two places do not hear the view today and must: the **range panel** (it reads the view through `Look` as it reads units, and rewrites its distance unit and value on a view change) and the **fullscreen strip** (it is remounted on a view change as it is on a units change, seeded with the view's sport, so AC "including fullscreen" holds in a swim leg view). Nothing new runs per cursor or selection flush: the readout's distance field is fixed per instance | 60 fps path untouched (ADR-016); no new store |
| YD-7a | **A leg switch clears the range** (picker `onchange` in the sections chunk, `selection.clear()`), so the range panel never shows a range from another leg's unit scope. Supersedes the LP-3 "keeps the range" case; LP-8's automatic return keeps its range | User decision; costs nothing in the entry |
| YD-8 | **Edit page** gets YD-1 to YD-7 through `startPage`, `ChartStack`, the range panel and the builders it already calls. Owner code changes only at the two call sites that format a distance themselves (pins description, added-laps table). They pass the workout's sport | Owner chunk headroom |

## Cost

| What | Decision |
| --- | --- |
| Entry (ADR-011) | Rule and yard formatter in `lib/format`, the yard branch of the axis scale, the view hook for the range panel and the strip. **Target 150, ceiling 200 bytes gzip.** The reserve is 618 (entry 61,382 of 62,000) and leaves 418 after the ceiling. The engineer measures before and after and records the figure in ADR-011 item 4. Above 200: stop and report |
| ADR-011 decision 5 | The reserve has been under the 1,000 trigger since F-68. Revisited for F-72: its code is a first-view rewrite (ADR-027), so remedy 1 (`import()`) cannot take it, and it is accepted from the reserve. **The next feature that adds entry code needs a size review first** (hand-off) |
| Owner chunk `start.svelte` | Call sites only, nothing new imported into `src/edit/`. **Ceiling 30** (19,424 of about 20,000). Above it: stop and report |
| Lazy (splits, pins chunks) | Call-site changes; report sizes. Not counted |
| Server, data file, HTML, LCP | Unchanged |

## Verification (what must be shown, not how)

- The rule: imperial × {swimming, running, cycling, multisport} and metric × swimming, on its own and through every builder. Metric builder output unchanged: the existing page samples and tests pass without edits (an edit is a deviation to report).
- AC 1 to AC 4 and AC 6 on synthetic files: a 25 m pool, a 25 yd pool of 66 lengths (1,650 yd from the recorded 22.86 m step), open water with 100 m splits, and a triathlon in the whole view and each leg view, in both readouts and the range panel.
- Formatting in en, de, es and fr at 999, 1,000, 1,650 and 10,000 yd. Readout and summary at 360 px portrait.
- Units switch, language switch and leg switch rewrite every YD-3 surface without a reload; ADR-016 scrubbing on the longest swim and the corpus triathlon.

## Assumed points (built unless the user objects)

| ID | Point | Suggestion (built) |
| --- | --- | --- |
| F72-Q1 | AC 1 asks for yards "in the splits and Lengths columns" of a 25 m pool. AP-20 and the feature's "What exists" keep pool splits in the pool's unit, and the Lengths view has no distance column | Pool splits keep the pool's unit ("100 m" / Distance (m) "100"). Only the laps column of a metre pool switches to yards. AC 1 to be reworded |
| F72-Q2 | A range inside the swim leg in the **whole** view: the panel shows pace per 100 yd (LP-10) but distance in miles (YD-2) | Keep miles: OQ-351 keeps the whole view in miles, and the readout must not switch units mid-scrub |
| F72-Q3 | "Their thousands separator" in Spanish: CLDR does not group four-digit numbers ("1650 yd") | Follow `Intl` (CLDR) in de/es/fr. Spanish groups from 10,000 ("16.500 yd") |
| F72-Q4 | The viewing page's pins list (F-56) shows distances too. The feature names only the edit page's | It switches as well (lazy pins chunk, no entry cost). Otherwise the two lists disagree |

## Hand-offs

| To | What |
| --- | --- |
| Requirements | F-72 AC 1 and "What changes": pool splits keep the pool's unit, and the Lengths view has no distance column (F72-Q1). Add the viewing pins list (F72-Q4) |
| Architect, next static feature | ADR-011 size review (decision 5) before it is built |
