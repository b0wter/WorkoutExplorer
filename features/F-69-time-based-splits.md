# F-69 Time-based splits

- **Status:** planned (user, 24 Nov 2026); listed last in the build order; open questions OQ-333 to OQ-337 open
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

## Scope
- **Presets:** 1, 5, 10, 15, 30 and 60 min, added to the same drop-down below the distance presets, in their own group headed "Time" (OQ-334). Labels "1 min" … "60 min". Presets longer than the workout's (or leg's) duration are hidden. The page still opens on the sport's distance default (AP-20).
- **Boundaries** (OQ-333): a split ends at the first sample at or after each multiple of the preset, counted from the workout's (or leg's) start on the chosen clock. The last partial split shows its own time. Inside a pause or time gap, the boundary falls on the first sample after it.
- **Columns** as for distance splits (AP-8): number, time, distance, speed or pace, gain, heart rate, power, cadence, each only when present; the time column is the split's duration, the distance column what was covered.
- **Where offered:** wherever distance splits are offered today (OQ-336); pool swims: OQ-335. Multisport (F-64): each leg's drop-down gets the time group; splits start at the leg's start.
- **Behaviour as AP-20:** choosing a preset rebuilds the table and its comparison chart (AP-23); the selected range is kept; a row tap selects its range (AP-7); per visit, not remembered, not in the range link (AP-19); a units switch keeps the choice.
- **Texts:** "Time" group heading and labels in all four languages (DS-6).

## Out of scope
- Free-entry durations; splits by heart rate, power or other values.
- Time splits on the server-rendered page or with JavaScript off (no splits there today).
- Storing a default chosen by the sharer.

## Constraints
- Code stays in the splits chunk; nothing new in the viewing entry (ADR-011).
- Building 1-min splits on the 12-hour corpus file (about 720 rows) and its comparison chart stays within F-46's rebuild budget; scrubbing stays at 60 fps.
- **Reliability:** smart recording (irregular intervals), long pauses, gaps between sessions, a workout shorter than 1 min, trimmed starts (UP-6) and truncated files must not break the table.

## Acceptance criteria
- [ ] A 50-min run offers 1, 5, 10, 15 and 30 min under "Time", not 60; choosing 10 min shows five rows of 10:00 each; the page opened on 1 km.
- [ ] A 47-min ride with 10 min shows four rows of 10:00 and a last row of 7:00 (partial).
- [ ] A ride with a 20-min stop follows OQ-333 (clock and boundary in the stop).
- [ ] A triathlon: each leg's drop-down has the time group; a run leg's 5-min splits start at the run's start; transitions unchanged (F-64).
- [ ] Pool swims follow OQ-335; workouts without distance follow OQ-336.
- [ ] Row tap selects the split's range; comparison chart shows one bar per row; the range is kept on a preset change; not part of `#range=`.
- [ ] Phone portrait at 360 px (drop-down fits, no horizontal page scroll), desktop; both themes; all four languages.
- [ ] Entry bundle unchanged; 60 fps scrubbing on the 12-hour file with 1-min splits shown; every corpus file renders.

## Open questions
- [ ] **OQ-333 Which clock.** Suggest: elapsed time on the chart's time axis (AP-5), so split rows line up with the time axis and ranges; a stop falls into the split it happens in. Alternative: moving time (stops skipped, rows no longer match the time axis). Affects Scope "Boundaries", AC 3.
- [ ] **OQ-334 Placement.** Suggest: the same drop-down, time presets in a second group "Time" below the distance ones; no new control. Alternative: a Distance/Time toggle next to the drop-down. Affects AP-20, UI.
- [ ] **OQ-335 Pool swims.** Suggest: no time presets for pool swims (split by whole lengths only; the Lengths view covers it). Alternative: time splits whose boundaries snap to the nearest length end. Affects AP-13, AP-20, AC 5.
- [ ] **OQ-336 Workouts without distance.** Suggest: unchanged; no splits section where none exists today (AP-8). Alternative: offer time splits also on workouts without distance (e.g. indoor strength with heart rate), which adds a splits section there. Affects AP-8, AC 5.
- [ ] **OQ-337 Default view.** Suggest: the sport's distance default stays the opening choice everywhere. Alternative: open indoor or virtual workouts on 5 min. Affects AP-20, AC 1.
