# F-69 Time-based splits

- **Status:** decided (24 Nov 2026)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (splits are built in the browser, F-46; no data file or server change)
- **Lane:** normal (computed from published streams; nothing stored, nothing new published)
- **Requirements:** AP-20, AP-8; AP-7, AP-12, AP-13, AP-19, AP-23, DS-1, DS-6; NFR Performance, NFR Reliability
- **Depends on:** F-46 (split presets), F-48 (pool lengths), F-51 (comparison chart), F-64 (per-leg splits), F-54 (four languages)

## Goal
A viewer splits the workout every 1, 5, 10, 15, 30 or 60 minutes, for example to compare 5-minute blocks of a ride or an interval run, next to today's distance splits.

## What exists
- The splits view (AP-8) has one split-distance drop-down per table (AP-20, F-46): metric presets per sport, default marked, presets longer than the workout hidden, per visit, not in the range link. Code: `src/web/src/splits` (lazy chunk) and `src/web/src/lib/splits` (`splitChoice`, `presetBounds`).
- Multisport workouts have one splits table and one drop-down per sport leg; transitions are one-line headings (F-64).
- Pool swims split by whole lengths (AP-20); the comparison chart (F-51) draws one bar per split row.
- The published data has a cumulative moving-time stream (ADR-029); the rule for what counts as moving is in the technical design (glossary "Moving time", F-02 R-7).

## Scope
- **Presets:** 1, 5, 10, 15, 30 and 60 min, added to the same drop-down below the distance presets, in their own group headed "Time" (OQ-334); no new control. Labels "1 min" … "60 min". Presets longer than the workout's (or leg's) moving time are hidden; with under 1 min of moving time there is no "Time" group. The page still opens on the sport's distance default (AP-20, OQ-337).
- **Clock: moving time** (OQ-333). Splits are counted in moving time as the published moving-time stream counts it (rule: technical design, F-02 R-7); the clock starts at 0 at the workout's start, or at each leg's start on a multisport workout.
- **Boundaries:** a split ends at the first sample whose moving time is at or past the next multiple of the preset. The last partial split shows its own moving time.
  - A pause or stop (timer stopped, or below the speed threshold) adds nothing to any split's time; it lies inside the split in which it happens. If a boundary is reached exactly where a pause begins, the pause belongs to the next split.
  - Where moving time jumps past a multiple between two samples (a time gap the rule counts as moving), the boundary falls on the first sample after the gap; that split's time is longer than the preset.
  - Split rows do not line up with the chart's time axis (AP-5, elapsed time): a row's range (AP-7) runs from its first to its last sample and includes any pause in it, so the range panel shows the pause in elapsed time and the split's time as moving time.
- **Columns** as for distance splits (AP-8): number, time, distance, speed or pace, gain, heart rate, power, cadence, each only when present; the time column is the split's moving time, the distance column what was covered.
- **Where offered:** wherever distance splits are offered today; no splits section is added where none exists (AP-8, OQ-336). Not for pool swims, including a pool-swim leg: they keep whole-length splits only (OQ-335; the Lengths view covers time). Multisport (F-64): each other leg's drop-down gets the time group; each leg's moving time counts from that leg's start; transitions are part of no leg, so their time counts toward no split.
- **Behaviour as AP-20:** choosing a preset rebuilds the table and its comparison chart (AP-23); the selected range is kept; a row tap selects its range (AP-7); per visit, not remembered, not in the range link (AP-19); a units switch keeps the choice.
- **Texts:** "Time" group heading and labels in all four languages (DS-6).

## Out of scope
- Free-entry durations; splits by heart rate, power or other values; splits on elapsed time.
- Time splits on the server-rendered page or with JavaScript off (no splits there today).
- Storing a default chosen by the sharer.

## Constraints
- Code stays in the splits chunk; nothing new in the viewing entry (ADR-011).
- Building 1-min splits on the 12-hour corpus file (about 720 rows) (the table; with about 720 rows there is no chart under AP-23) stays within F-46's rebuild budget; scrubbing stays at 60 fps.
- **Reliability:** smart recording (irregular intervals), long pauses, gaps between sessions, a workout shorter than 1 min, trimmed starts (UP-6) and truncated files must not break the table.

## Acceptance criteria
- [ ] A 50-min run (moving time) offers 1, 5, 10, 15 and 30 min under "Time", not 60; choosing 10 min shows five rows of 10:00 each; the page opened on 1 km.
- [ ] A 47-min ride (moving time) with 10 min shows four rows of 10:00 and a last row of 7:00 (partial).
- [ ] A ride with a 20-min stop at 14:00 moving time, 10 min: rows show 10:00 each; the stop adds nothing to any row and lies inside row 2; tapping row 2 selects a range whose panel shows elapsed time about 30:00 and moving time 10:00. A boundary reached exactly at the stop's start puts the stop in the next row.
- [ ] A file with a time gap that moving time counts across a multiple: the boundary falls on the first sample after the gap and that row's time is longer than the preset.
- [ ] A triathlon: the bike and run legs' drop-downs have the time group; a run leg's first 5-min split ends 5:00 of moving time after the run's start; T1 and T2 count toward no split and stay as today (F-64).
- [ ] A pool swim, and a pool-swim leg, offer no time group; a workout without distance has no splits section, as today.
- [ ] Row tap selects the split's range; comparison chart shows one bar per row; the range is kept on a preset change; not part of `#range=`.
- [ ] Phone portrait at 360 px (drop-down fits, no horizontal page scroll), desktop; both themes; all four languages.
- [ ] Entry bundle unchanged; 60 fps scrubbing on the 12-hour file with the 1-min splits table shown (no chart at that row count, AP-23); every corpus file renders.

## Open questions
- [x] **OQ-333 Which clock.** Decided (user, 24 Nov 2026), alternative chosen: moving time instead of the suggested elapsed time; now in Scope "Clock" and "Boundaries".
- [x] **OQ-334 Placement.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Presets".
- [x] **OQ-335 Pool swims.** Decided (user, 24 Nov 2026) as suggested (whole lengths would force uneven splits; the Lengths view covers it); now in Scope "Where offered".
- [x] **OQ-336 Workouts without distance.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Where offered".
- [x] **OQ-337 Default view.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Presets".
