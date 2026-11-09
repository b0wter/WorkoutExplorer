# F-41 Summary tints

- **Phase:** 2
- **Owner:** frontend
- **Lane:** small (suggested, OQ-137)
- **Requirements:** DS-5 (Could, phase 2); AP-2, AP-4, DS-2, DS-4; NFR Performance (budgets unchanged)
- **Depends on:** F-13, F-14

## Goal
A viewer sees at a glance which summary value belongs to which chart.

## Scope
- **Tinted cells.** As DS-5, by the cell's `data-k` in the summary strip: `ascent` takes the elevation colour (`--c-altitude`), `speed` (speed or pace, DS-2) `--c-speed`, and `heart_rate`, `power`, `cadence`, `temperature` their own `--c-*`. `distance`, `moving_time`, `elapsed_time` and `calories` stay plain (`--card`).
- **Tint.** One strength for every metric: the theme's line colour mixed over the page background (OQ-135). Derived from the existing `--c-*` theme properties (`page.css`), so a theme change (DS-4 drop-down or system) and any later line-colour change carry over without a code change.
- **Contrast.** Every text in a tinted cell (label and unit in `--muted`, value in `--fg`, the "max" line in `--muted`) keeps at least 4.5:1 against the tint in light and dark theme. Estimate at 15 % strength: `--muted` text 5.9 to 6.3:1 in light and 5.7 to 6.1:1 in dark theme, so no line colour changes; at 25 % dark theme drops to 4.6:1.
- **Pages.** The summary is the same markup on the viewing page (server template, `src/web/index.html`) and the edit page (`SummaryStrip.svelte` via `ActivityPage`), so both are tinted (OQ-136). Styling only: works without JavaScript, nothing loaded or stored.

## Out of scope
- Range panel (AP-7) and cursor readout (they share one cell look), laps and splits (AP-8), the preview image (SH-5, SH-12), the How to use dialog (AP-22).
- Route colouring (AP-9, F-25): its plain route colour and its blue-yellow-red ramps are independent of the chart line colours and stay unchanged; the summary tints are not a map legend.
- Changing chart line colours, unless OQ-135's answer requires it.

## Acceptance criteria
- [ ] On a corpus ride with every stream, the elevation gain, speed, heart rate, power, cadence and temperature cells are tinted, each in a muted version of its own chart's line colour; distance, moving time, elapsed time and calories stay plain.
- [ ] A pace sport (DS-2) tints its pace cell in the speed chart's colour.
- [ ] In light and dark theme the tint follows that theme's line colour; switching the theme drop-down retints at once, without reload or flash.
- [ ] Every text in a tinted cell has a contrast of at least 4.5:1 against its tint in both themes (measured per metric).
- [ ] A cell appears only when its value exists (AP-2) and is tinted whether or not its chart is shown; a server-computed value looks like a device value.
- [ ] The viewing page and the edit page show the same tints; with JavaScript off the viewing page is tinted.
- [ ] Range panel, cursor readout, laps and splits, preview image and route colours are unchanged.
- [ ] Data file size and LCP are unchanged.

## Open questions
- **OQ-135 Tint strength.** *Suggested:* about 15 % of the line colour over the page background, one value for every metric and both themes, checked on screen by the user in both themes (spec open question "Summary colours").
- **OQ-136 Edit page.** The edit page shows the same summary strip. *Suggested:* tinted too (one CSS rule, the owner sees what viewers see).
- **OQ-137 Lane.** CSS-only change with a contrast check. *Suggested:* small lane (frontend engineer, no architect, no QA round), with the user's on-screen review as the acceptance check.
