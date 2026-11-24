# F-67 Multisport leg picker

- **Status:** planned (user, 24 Nov 2026); listed last in the build order; decided (24 Nov 2026): OQ-315 to OQ-325 as suggested (user)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (no data file or server change: the legs come from `meta.sessions`, F-42; backend only if the architect wants the picker's row reserved in the server-rendered page, see Constraints)
- **Lane:** normal (no new data, nothing stored, no coordinates leave the published data)
- **Requirements:** AP-12; AP-4, AP-5, AP-6, AP-7, AP-8, AP-15, AP-17, AP-18, AP-19, AP-20, AP-21, DS-1, DS-2, DS-4, DS-6; NFR Performance, NFR Reliability
- **Depends on:** F-42 (sections, leg names, lazy sections chunk), F-17 (range, axis toggle), F-40 (range link), F-64 (per-leg tables), F-54 (four languages)

## Goal
On a "Multisport" workout, a viewer looks at one leg at a time with its speed in that leg's own unit (pace /100 m in the swim, km/h on the bike, pace /km in the run), while speed and heart rate stay visible together. Today the speed chart and its readout are one km/h line across all legs (`chartSpecs` picks the spec from the workout sport, `multisport` → speed), so swim and run are neither comparable nor shown as pace.

## What exists
- F-42: sections per session (`lib/sections`), leg names as sport names, transitions T1, T2, …; leg lines and names in every chart; leg markers on the map. The browser part is a lazy chunk (`src/web/src/sections`), loaded only for workouts whose sessions hold two or more sports.
- Charts have a fixed x range: the whole workout. There is no chart zoom today.
- The map fits the whole route until the viewer moves it (F-13 D-20); a range highlights the route but never moves the camera (AP-19).
- The cursor readout (AP-6, AP-21) and the range panel (AP-7) use the workout sport, so a multisport workout reads km/h everywhere. F-64 tables already use the leg's sport (OQ-300); best efforts (F-50) already work per leg.

## Scope
- **Picker** (AP-12): a one-row control directly above the charts: "Whole workout" followed by each sport leg in order, named as the page names them (F-42, F-64). Shown only when the page has sections (two or more sports, F-42); never for other workouts. Opens on "Whole workout" (today's view). Transitions: OQ-315. Repeated sports: OQ-324.
- **Leg view:** choosing a leg sets the x range of every chart to that leg's samples (from its first sample to the next leg's start, `sectionRanges`), on the time and the distance axis (AP-5); axis origin: OQ-323. Each chart's y axis fits the leg's own values (assumed; the pace axis bound, `chartValues`, is computed from the leg's samples). Leg lines and names (F-42) stay where they fall in view.
- **Speed chart in a leg:** label, unit, axis direction and ticks follow the leg's sport (DS-2: per 100 m or 100 yd for swimming, per 500 m for rowing, per km or mi for running, walking and hiking, km/h or mph otherwise; units setting DS-1). A units switch keeps the chosen leg.
- **Whole-workout view:** the speed chart stays one speed line in km/h or mph as today (OQ-321). The cursor readout shows the speed value in the unit of the leg under the cursor, labelled accordingly (e.g. "Pace 4:58 /km" in the run, "Speed 32.4 km/h" on the bike); "–" in a transition (AP-4), at a stop (DS-2) or without a value. The fullscreen readout (AP-21) does the same. Applies in every view, also with the picker not yet loaded.
- **Cursor and range** keep working in a leg view: hover, touch-drag, long-press range selection (AP-6, AP-7). Whether picking a leg is a range: OQ-316; an existing range: OQ-319.
- **Map** (AP-3, AP-15, AP-18): OQ-317. **Laps and splits** (AP-8, AP-20, AP-23, F-64): OQ-318.
- **State:** not stored, not in the link (OQ-320).
- **Pages:** viewing page and edit page (OQ-325). Published page (AC-6) only in the sense that it is the viewing page.
- **Texts:** "Whole workout" and any new readout label in English, German, Spanish and French (DS-6); leg names already translated.

## Out of scope
- Splitting the charts per leg, or one chart per sport. A whole-workout speed axis in mixed units.
- Summary strip (AP-2) per leg; route colouring by speed (AP-9) per leg unit (stays as today); per-leg zones (AP-10).
- Chart zoom for non-multisport workouts, free zoom or pan in the charts, pinch zoom.
- Any change to the data file, the server-rendered laps tables or the no-JS page.

## Constraints
- **Entry bundle (ADR-011):** the viewing entry is at 60,565 of 62,000 bytes gzip. The picker, leg view and per-leg speed specs live in the lazy sections chunk; the entry may only gain a hook to set the charts' x range and swap a spec (measured, tens of bytes). The whole-view readout fix may need per-leg unit logic in the entry; the design states its size.
- **No layout jump:** the picker's row must not push the charts down after first paint (LCP, F-13 D-10). The architect chooses: a row reserved from first paint for multisport workouts (server-rendered, small backend change) or a placement that takes no new row.
- **Performance:** switching legs redraws all charts within 100 ms on a mid-range phone; scrubbing in a leg view stays at 60 fps on the longest multisport corpus file; charts drawn in a leg view show at least the detail of the whole view (decimation per visible range is the design's choice).
- **Privacy:** nothing new is published or stored; any map fit uses only published, privacy-applied positions (SH-4, UP-6), so a hidden start or end gives no hint.
- **Reliability:** a leg without samples (F-42 MS-7), a leg without speed, without distance (distance axis), with all positions hidden, a trimmed workout whose first or last leg is cut (UP-6), one-sample legs, sessions out of order and truncated files must not break the charts or the picker.

## Acceptance criteria
- [ ] A triathlon shows "Whole workout · Swim · Bike · Run" above the charts; a single-sport and a two-cycling-session workout show no picker.
- [ ] Choosing Run zooms every chart to the run leg on the time axis and on the distance axis; the speed chart reads "Pace", per km (or mi), faster up; Swim reads per 100 m (or 100 yd); Bike reads km/h (or mph). "Whole workout" restores today's view exactly.
- [ ] In the whole-workout view the readout shows "/km" pace in the run, "/100 m" in the swim, km/h on the bike, "–" in T1 and T2; the fullscreen readout (AP-21) matches.
- [ ] Units switch (DS-1) and language switch (DS-6) keep the chosen leg and update the labels.
- [ ] Hover, touch-drag and long-press range selection work in a leg view; behaviour with an existing range, a lap row tap and a `#range=` link follows OQ-316 and OQ-319.
- [ ] Map, laps and splits follow OQ-317 and OQ-318; with a hidden start or end, nothing about the hidden part can be inferred from the map view.
- [ ] Reload opens on "Whole workout"; Back does not step through leg choices (OQ-320).
- [ ] Both themes (DS-4); phone portrait at 360 px (no horizontal page scroll), phone landscape and desktop two-column layouts (AP-17), pinned map (AP-18); touch and mouse; the picker is reachable and operable with Tab and Enter or Space.
- [ ] JavaScript off: page unchanged (no picker, laps tables as today).
- [ ] Entry bundle within 62,000 bytes gzip (size check in CI) and LCP unchanged within noise; no layout jump of the charts when the picker appears; leg switch and scrubbing budgets above met; every corpus file renders.
- [ ] New texts in all four languages.

## Decisions (user, 24 Nov 2026)
- OQ-315 to OQ-325: decided as suggested (see Open questions); Scope and acceptance criteria follow them.

## Open questions
- [x] **OQ-315 Transitions in the picker.** Decided (user, 24 Nov 2026) as suggested: not listed; only sport legs. A leg view shows only that leg; T1 and T2 stay visible in "Whole workout" and in the laps tables (F-64). Alternative: T1 and T2 as small entries. Affects Scope "Picker", AC 1.
- [x] **OQ-316 Leg view or range.** Decided (user, 24 Nov 2026) as suggested: picking a leg is a view (zoom) only: no range is selected, no range panel, no `#range=` link. Selecting the leg as a range stays with the leg heading row (AP-8, F-64), which is already shareable (AP-19). Alternative: picking also selects the leg as a range. Affects AP-7, AP-19, AC 5.
- [x] **OQ-317 Map.** Decided (user, 24 Nov 2026) as suggested: picking a leg fits the map once to that leg's visible route (as the initial fit, padding as today); "Whole workout" refits the whole route; the rest of the route stays drawn as now; no move when the leg has no visible positions (pool swim, hidden). Not in fullscreen (no picker there). Alternative: the map never moves (as for ranges, AP-19). Affects AP-3, AC 6.
- [x] **OQ-318 Laps and splits tables.** Decided (user, 24 Nov 2026) as suggested: unchanged; all per-leg tables stay (F-64). Alternative: show only the picked leg's tables. Affects AP-8, AC 6.
- [x] **OQ-319 Range and leg view together.** Decided (user, 24 Nov 2026) as suggested: an existing range stays selected when a leg is picked (panel unchanged, shaded where in view); selecting a range from a row, bar, best effort or `#range=` link that is not inside the shown leg switches the picker to "Whole workout". Alternative: picking a leg clears the range. Affects AP-7, AP-8, AP-19, AC 5.
- [x] **OQ-320 URL and history.** Decided (user, 24 Nov 2026) as suggested: the choice is not stored anywhere (no fragment, no browser storage, no history entry); every load opens on "Whole workout". Alternative: `#leg=2` next to `#range=`. Affects AP-19, AC 7.
- [x] **OQ-321 Whole-workout speed axis.** Decided (user, 24 Nov 2026) as suggested: stays one speed axis in km/h or mph labelled "Speed", as today; only the readout follows the leg (Scope). Alternative: hide the speed chart in the whole view (contradicts "keep speed visible"). Affects AP-4, AC 2.
- [x] **OQ-322 Range panel unit.** Decided (user, 24 Nov 2026) as suggested: a range inside one leg shows its average in that leg's unit (as F-64 tables); a range across legs shows km/h or mph. Alternative: leave the panel as today (km/h). Affects AP-7.
- [x] **OQ-323 Axis origin in a leg view.** Decided (user, 24 Nov 2026) as suggested: time and distance stay counted from the workout start (AP-5), so the readout, laps and range links agree. Alternative: from the leg start. Affects AP-5, AC 2.
- [x] **OQ-324 Repeated sports** (duathlon: run, bike, run). Decided (user, 24 Nov 2026) as suggested: number them only when a sport repeats ("Run 1 · Bike · Run 2"), in the picker only. Alternative: plain names as F-42 ("Run · Bike · Run"). Affects AC 1.
- [x] **OQ-325 Edit page.** Decided (user, 24 Nov 2026) as suggested: the edit page gets the same picker (same charts). Alternative: viewing page only. Affects Scope "Pages".
