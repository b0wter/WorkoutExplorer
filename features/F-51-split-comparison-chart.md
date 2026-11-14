# F-51 Lap and split comparison chart

- **Phase:** 2
- **Owner:** frontend (drawn in the browser from the rows the laps and splits section already has; no server or data file change)
- **Lane:** normal (short architect note on the lazy chunk and shared colour tokens; one QA pass)
- **Status:** ready for architecture (after F-50) — OQ-196 to OQ-209 resolved 14 Nov 2026
- **Requirements:** AP-23 (Could, phase 2; decided 14 Nov 2026); AP-7, AP-8, AP-12, AP-13, AP-19, AP-20, UP-6, DS-1, DS-2, DS-4; NFR Performance (budgets unchanged), NFR Reliability
- **Depends on:** F-18, F-42, F-46, F-47, F-48, F-49 (colour tokens)

## Goal
A viewer sees at a glance which laps, splits or lengths were faster or slower than usual, in a bar chart under the laps and splits table (example: Apple Fitness "Split Variation").

## What exists
- Laps (server-rendered), splits at the chosen preset distance (F-46, AP-20) and pool lengths (F-48) with the Laps / Splits / Lengths switch; a row tap selects its range (F-18), which the range link can share (F-40).
- Speed or pace, heart rate, power and cadence per row, in the units setting (DS-1, DS-2).
- Zone colour tokens `--zone-red`, `--zone-blue`, `--zone-grey` for both themes (F-49, in the zones chunk's stylesheet); chart line colours `--c-heart_rate`, `--c-power`, `--c-cadence` (page stylesheet).

## Scope (decided 14 Nov 2026; see Open questions)
- **Place:** directly under the laps and splits table, inside that section, on the viewing and edit page; needs JavaScript (no chart with JavaScript off).
- **Rows** (OQ-196): one bar per row of the view shown, in table order, labelled by row number; rebuilt when the view or the split distance changes. Separator rows and Rest lengths get no bar.
- **Bars** (OQ-197 to OQ-199): each bar starts at the reference (median speed of the rows) and its height is the row's difference from it; faster up, slower down, also for pace sports. Header line: "Median 24.1 km/h · Best 29.9 km/h" (pace sports: "Median 5:12 /km · Best 4:48 /km"). Rows within ±3 % of the median are "on pace", drawn neutral, with the band shaded behind the bars. Legend: Faster, On pace, Slower. Caption under the axis: the view ("Laps", "1 km splits", "Lengths").
- **Partial rows** (OQ-200): the last partial split is drawn like the others (equal bar width) but left out of median and Best.
- **Overlay** (OQ-201): a drop-down "Overlay": None, Heart rate, Power, Cadence, listing only those the rows have; default None; per visit, not remembered, not in the range link. A line with dots at each bar, on its own scale at the right edge, in that metric's chart line colour.
- **Interaction** (OQ-202): tapping or clicking a bar does what a row tap does (AP-7, AP-8); the bar of the selected row is highlighted. Hover highlights the bar and its table row; no tooltip.
- **Rows without speed** (OQ-203): no bar, the slot stays empty; with fewer than 2 bars there is no chart.
- **Multisport** (OQ-204): no chart for "Multisport" workouts (AP-12).
- **Units** (OQ-205): speed or pace as DS-2, units as DS-1; a units switch redraws.
- **Colours** (OQ-206): faster `--zone-red` (zone-5 colour), slower `--zone-blue` (zone-1 colour), on pace `--zone-grey`, as the zones' intensity scale; overlay line in its chart colour; both themes (DS-4).
- **Trim and range** (OQ-207): the chart shows exactly the table's rows (viewing page trimmed, edit page the full recording as the table, OQ-178); it does not follow a range selected on the charts.
- **Bundle** (OQ-208): in the lazy laps-and-splits chunk, not in the entry bundle; no new library; drawn after the table.
- **Many rows** (OQ-209): every bar fits the width, no horizontal scrolling; above 200 bars no chart.
- **Size:** about 160 px tall on every screen.
- **Storage:** nothing new is loaded or stored.

## Out of scope
- Comparing rows of different workouts, or against a target pace.
- Bars for metrics other than speed or pace; time-based splits and splits per leg (Later).
- The chart in the preview card, fullscreen map or how-to dialog.

## Acceptance criteria
- [ ] A 10.4 km run on 1 km splits shows 11 bars under the table, the 0.4 km bar drawn but not counted in "Median" or "Best"; median and best match hand-computed values (AP-23).
- [ ] Bars faster than the median by more than 3 % point up in red (`--zone-red`), slower ones down in blue (`--zone-blue`), those within ±3 % grey (`--zone-grey`); on a run (pace) faster bars also point up and the header shows pace (DS-2).
- [ ] Switching Laps / Splits / Lengths or the split distance redraws the chart for the rows shown; a pool swim's Lengths view has one bar per active length and none for Rest rows (AP-13, AP-20).
- [ ] Tapping a bar selects the same range as tapping its row, in charts, map and panel, and highlights the bar (AP-7).
- [ ] The overlay offers only metrics the rows have, starts on None on every load, and draws one dot per bar at the row's value.
- [ ] A lap without speed has an empty slot; a workout with fewer than 2 bars, a "Multisport" workout and a view with more than 200 rows show no chart.
- [ ] Switching units redraws header and axis in the new units (DS-1); both themes use the zone colours (DS-4).
- [ ] A trimmed workout's chart on the viewing page shows only the remaining rows; the edit page shows the same rows as its table (UP-6).
- [ ] With JavaScript off there is no chart and nothing breaks; a truncated file (UP-3) shows its chart without error.
- [ ] The chart's code is not in the entry bundle (entry within the ADR-011 reserve); LCP, summary timing and 60 fps scrubbing on `ride-12h` unchanged within noise.

## Open questions
All resolved by the user on 14 Nov 2026 (decision log in [README](README.md)): OQ-196 to OQ-205 and OQ-207 to OQ-209 as suggested in Scope; OQ-206 changed:
- **OQ-206 Colours:** as the zones' intensity scale: faster red (`--zone-red`, the heart-rate zone-5 colour), slower blue (`--zone-blue`, the zone-1 colour), on pace grey (`--zone-grey`); overlay line in its chart line colour.
