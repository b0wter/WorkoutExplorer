# F-42 Multisport sections

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** normal (OQ-143: short architect design for the leg model shared by Go and TS, one QA pass)
- **Requirements:** AP-12 (Could, phase 2); AP-3, AP-4, AP-5, AP-7, AP-8, AP-15, SH-4; NFR Performance (budgets unchanged); decisions OQ-36, OQ-38, OQ-39 in [README](README.md)
- **Depends on:** F-15, F-17, F-18, F-26

## Goal
A viewer of a triathlon sees where each leg and transition starts, in the charts, the laps table and on the map.

## What exists
- The data file already carries every session with its sport, sub-sport, time and sample range (`meta.sessions`, F-03 §5.3); no format change is needed. Transitions are already sessions with sport `transition` (OQ-38, F-08 R-13).
- Laps carry no session; a lap belongs to the leg its first sample lies in.
- Today a multi-session file is one workout: combined summary (F-08 R-10), one route with start, finish and lap markers, one laps table. The laps table is rendered by the server (`internal/page`, static page and no-JS view) and by `lib/page` (edit page) from one view model; the enhancer adopts it (ADR-026). So the separator rows need the server view model too: owner both, backend first.

## Scope
- **Which workouts.** Workouts with two or more sessions of more than one sport, i.e. sport "Multisport" (OQ-138). A transition counts as a sport (OQ-36, OQ-38).
- **Legs.** One leg per session, in time order, starting at its first sample. A gap between sessions belongs to the leg before it. Samples after the last session of a truncated file stay part of the last leg (OQ-39 rule; damaged files are a bonus).
- **Leg name.** The sport name as shown elsewhere; transitions "T1", "T2", … in order (OQ-139). The same name is used on charts, in the laps table and on the map; repeated sports get no numbers.
- **Charts (AP-4).** Leg names appear in every chart (decision 10 Nov 2026). A vertical line through every chart at the start of each leg after the first, on both the time and the distance axis (AP-5). The leg's name sits at the top of each chart, inside its section; the first leg's name sits at the start of the plot, with no line at its start. A name that does not fit its section (short transitions, zero-distance transitions on the distance axis) is left out; the line stays. Lines and names do not change the cursor, range selection or scrubbing; a range may cross legs (AP-7, unchanged).
- **Laps table (AP-8).** Before each leg's laps a separator row holding the leg's name across all columns, on the viewing page with and without JavaScript and on the edit page. A leg without laps shows its separator row alone. Tapping a separator row selects the leg as a range (OQ-140). Without device laps ("No laps recorded.") there are no separator rows. Splits stay across the whole workout, without separators; lap columns follow the workout sport as today (OQ-142).
- **Map (AP-3, AP-15).** A leg marker labelled with the leg's name at the first sample of each leg after the first, also in the fullscreen map (OQ-141). A leg marker replaces a lap marker at the same sample. Same rule as lap markers: only when that sample has a position, never a substitute point; so a leg start inside a hidden start or end (SH-4) shows no marker.
- **Summary (AP-2).** Unchanged: combined totals, no per-leg summary.
- **Pages.** Viewing page and edit page alike; the edit page also shows leg lines and markers inside the to-be-hidden marks, as it does for lap markers.

## Out of scope
- Per-leg summary or totals in the separator row; per-leg pace or speed columns; separators in the splits table.
- Leg name in the cursor readout (AP-6, AP-21).
- Sections for single-sport multi-session files (two rides) and for truncated tails.
- Any change to the data file format, the summary, route colouring (F-25) or the transition rules (OQ-38).

## Acceptance criteria
- [ ] On the corpus triathlon (swim, T1, ride, T2, run), the charts show 4 leg lines on the time axis and on the distance axis, none at the start; every chart shows the 5 names, the first at the start of the plot; each name shows where its section is wide enough and is left out otherwise, with the line kept.
- [ ] The laps table shows 5 separator rows (e.g. Swim, T1, Ride, T2, Run), each before its leg's laps, on the published page with JavaScript on and off and on the edit page; units switches keep them.
- [ ] Tapping a separator row selects that leg as a range, as a lap row does; the range panel gives the per-leg values.
- [ ] A lap that starts in one leg and ends in the next is listed under the leg it starts in.
- [ ] Where a leg start is also a lap start, only the leg marker is shown.
- [ ] The map shows a leg marker with the leg's name at each leg start after the first, in the page and the fullscreen map; none where the leg's first sample has no position.
- [ ] With start and end hiding on and a leg start inside the hidden part, the published data file and page give no marker or position for that leg start; lines and separator rows stay.
- [ ] A two-ride multi-session file and every single-session corpus file look as before (no lines, separators or leg markers).
- [ ] A truncated multisport file whose last session is lost shows the tail as part of the last surviving leg, without error.
- [ ] Summary, splits, range values, route colouring and cursor are unchanged; scrubbing on `ride-12h` stays at 60 fps; data file size unchanged; LCP unchanged.

## Decisions
OQ-138 to OQ-143 accepted as suggested (9 Nov 2026), see [README](README.md) decision log; OQ-139 partly reversed on 10 Nov 2026 (names in every chart, first leg named).
