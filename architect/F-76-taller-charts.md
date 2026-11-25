# F-76 Taller charts: architecture

- **Version:** 0.1
- **Status:** accepted (25 Nov 2026); normal lane; built after F-75 is merged; four assumptions under Open points
- **Feature:** [`features/F-76-taller-charts.md`](../features/F-76-taller-charts.md) (OQ-359 to OQ-367 decided)
- **Requirements:** DS-8; AP-4, AP-6, AP-7, AP-17, AP-18, AP-19, DS-1, DS-6, INF-1; NFR Performance
- **Builds on:** [F-73](F-73-remembered-split-without-layout-jump.md) RS-1 to RS-7 (head script, hash rotation); [F-05](F-05-stacked-charts-with-shared-cursor.md) D-15, D-16; [F-17](F-17-range-selection-and-axis-toggle.md) R-8; [F-67](F-67-multisport-leg-picker.md) LP-2; [ADR-011](ADR-011-activity-page-bundle-allocation.md) rev. 17; [ADR-016](ADR-016-scrubbing-smoothness-measurement-and-gate.md); [ADR-032](ADR-032-response-headers-in-caddy.md) item 3. No new ADR
- **Owner:** both, one commit: frontend (head script, CSS, toggle, glue), backend part = the hash in `deploy/Caddyfile` and `caddy_test.go`'s policy copy

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| TC-1 | **Storage.** One key `wx.charts`, value exactly `tall`. Turning the toggle off removes the key (as theme `system`). Written only by `lib/prefs` (`PREF_KEYS.charts`, a setter only; the bundle never reads it, TC-3). Any other value, a missing key or a throwing storage = normal, silently | DS-8; least stored; TD storage rule |
| TC-2 | **Head script: one statement, one new hash.** `if(s.getItem("wx.charts")=="tall")D.chartSize="tall"` inside the existing `try`, **after the hint statement and before the split statement** (the split part can throw on a missing `matchMedia`; nothing after it in the `try` would run). Sets `<html data-chart-size="tall">`, nothing else. Same text in all six shells (existing identity check). Hash rotation exactly as F-73 RS-6: new hash first after `'self'` in both policy copies, the F-73 hash and the older ones stay. Pages published before the release keep their old script and old bundle (ADR-053): no toggle there until rebuilt or expired | OQ-367; RS-2's "a failure never stops theme, hint, language" extended to chart size |
| TC-3 | **One runtime state: the `<html>` attribute.** The glue reads the initial size from `data-chart-size`, not from storage, so the bundle starts with exactly what the CSS reserved (also when the head script was blocked by CSP). A toggle flips the attribute (dataset, no `style`), calls the prefs setter, then the stack re-reads its height (TC-5) | No second validation; CSS and canvas cannot disagree |
| TC-4 | **CSS owns the plot height.** `page.css` registers `@property --plot { syntax: '<length>'; inherits: true; initial-value: 120px }`. `.charts` sets `--plot: 120px`; `[data-chart-size='tall'] .charts` sets `200px`; inside `LANDSCAPE_QUERY`'s media query the tall value is `clamp(120px, 100svh - var(--r) * 29px - 16px - 24px, 200px)` (viewport minus the sticky readout card, minus title row and top padding; `vh` fallback before `svh` as for `lvh`). The ten `.charts-N { --area }` literals become `--n: N`, and `.charts` computes `--area: calc(var(--n) * (var(--plot) + 24px) + 28px)` (= `chartAreaHeight`). Scope is `.charts` only, so comparison chart, power curve and zones never see it | One rule for reservation and drawing; the cap ignores the range panel so a selection never resizes charts |
| TC-5 | **The stack reads the height from CSS, never computes it.** `ChartStack` reads `parseFloat(getComputedStyle(stack).getPropertyValue('--plot'))` at mount, on every width measurement (rotation always changes width; `svh` does not move with the browser bar) and after a toggle. A value that is not a number (no page CSS: bench pages, jsdom; or a browser without `@property` in landscape) gives `chartHeight ?? DEFAULT_CHART_HEIGHT`, i.e. never taller than the reservation: a gap at worst, never an overlap. One style read, no layout, never on the scrub path | Registered properties compute to px without layout; `getComputedStyle` per chart is already done for colours |
| TC-6 | **Resize without remount.** Plot height becomes reactive: it is **not** in the `{#key}` expressions and the `{#each}` stays keyed by metric. `MetricChart`'s `layout` runs on a width **or** height change: `setSize` with the new outer height, cached plot rectangle set to `null`, cursor line, shade and decorator re-placed as after a width change; re-decimation only when the width changed (buckets depend on width only). `ChartStack`'s inline `.chart-area` height follows the same value. Replaces F-05 D-15 for the height; the chart set stays fixed per data value | One `setSize` per chart per toggle; cursor and shade are DOM transforms, so the scrub path (ADR-008, ADR-009) is unchanged at 200 px |
| TC-7 | **Toggle placement: the toolbar row exists whenever there are charts.** `--bar: 40px` on every `.charts`; the `.charts.has-distance` rule goes (the class stays in the view model, unused by CSS: no server or golden change). Row order in the DOM: axis toggle (only with an advancing distance, R-8), leg picker (F-67), size toggle last, pushed right (`margin-left: auto`, `flex: none`; the picker keeps shrinking). The sections chunk inserts the picker before the size toggle and checks for a distance stream itself (LP-2 kept: no picker without distance). One `<button aria-pressed>` styled like one axis-toggle segment, label constant | DS-8 "every layout": a workout without distance needs the row too; DOM order = focus order |
| TC-8 | **Entry code, no lazy chunk.** The button, its handler, the height read and the glue are a few hundred bytes; a lazy chunk would add a preload entry and make the button appear late | ADR-011 decision 2 rule: it serves every viewer with charts |
| TC-9 | **Layouts.** Pinned map (AP-18, portrait): unchanged, no automatic unpin (OQ-365); the readout keeps sticking under it. Splitter (both two-column layouts, F-75 lazy handle): a drag changes width only; `--plot` does not depend on width, so the drag re-decimates as today and never changes the height. Leg view (F-67), units, theme, axis and language remounts read the current height at mount, so they keep the size | No new coupling to map or splitter |
| TC-10 | **Edit page.** Same `ChartStack` and the same glue through `startPage`; same shell script. Owner chunk 0 bytes | OQ-364 |
| TC-11 | **Texts.** Label key in the charts' text group (`lib/charts/texts`, English in code), de/es/fr in that group's language files; must fit at 360 px beside the German axis toggle. About: the "In your browser" paragraph names the chart size in all four languages (About entry, own ceiling) | DS-6, INF-1; ADR-011 decision 6 |
| TC-12 | **Cost.** Activity entry (ADR-002 count): **target 300, ceiling 400 gzip**, from the reserve F-75 leaves (about 2,900, so decision 5's net-zero rule does not apply; if the measured reserve after F-75 is below 1,400, so that this ceiling could cross the 1,000 trigger, stop and report). Measured before and after, booked in ADR-011. Head script about 50 bytes raw per page (not counted). LCP: one more `getItem`, no network, no layout; unchanged within noise. JavaScript off: no attribute, 120 px reservation, page unchanged | AC "Entry bundle", NFR |

## What the tests must prove

| Claim | Proof |
| --- | --- |
| TC-1, TC-2 | Head script against stand-ins: attribute only for exactly `tall` (not `Tall`, `tall `, `1`, empty, missing); throwing storage and missing `matchMedia` leave theme, hint, language and chart size decided as before; the script contains `PREF_KEYS.charts` verbatim |
| TC-4 | CSS: for n = 1 to 10, `--area` equals `chartAreaHeight(n, 120)` and `(n, 200)`; the cap rule sits inside the `LANDSCAPE_QUERY` media query and nowhere else |
| TC-6 | jsdom with the uPlot fake: a toggle calls `setSize` once per chart with the new height and creates no new instance; the cursor and selection stores are untouched |
| TC-2, AC | Hash new first in both copies, F-73 hash still listed (existing checks); the e2e file of the feature (AC list), including the landscape cap and CLS 0 with tall stored; ADR-016 record at the tall size on ride-12h |

Mutants that must fail a check: chart-size statement after the split statement; `--plot` in a `{#key}`; the cap outside the landscape query; the F-73 hash dropped.

## Hand-offs

| To | What |
| --- | --- |
| Architect (in place, with the build) | F-05 D-15: height live per TC-6. F-17 R-8 and F-67 LP-2: toolbar row always with charts, picker checks distance itself (TC-7) |
| Requirements | TD Load order step 1 and the `lib/prefs` line: `wx.charts`, `data-chart-size` (TC-1, TC-2) |

## Open points (assumed; the user answers by number)

1. Workouts without a distance stream gain the 40 px toolbar row for the toggle alone (TC-7). Suggestion: accept.
2. Browsers without registered custom properties (Firefox before 128, Safari before 16.4): in short landscape the charts draw at 120 px inside the capped reservation, a gap below (TC-5). Suggestion: accept.
3. Pages published before the release ignore the stored size until rebuilt or expired (TC-2, as F-73 RS-6). Suggestion: accept, no forced rebuild.
4. Label wording in de/es/fr: proposed by the engineer, reviewed on screen at 360 px.
