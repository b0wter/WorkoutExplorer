# F-64 Laps and splits per multisport leg

- **Status:** planned (user, 22 Nov 2026, promoted from Ideas "Splits per multisport leg"); no build-order position; OQ-296, OQ-297, OQ-299, OQ-300 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first: the server-rendered laps tables per leg, also without JavaScript; frontend: splits per leg, drop-downs, charts). No data file change: laps and legs are already in it (F-42).
- **Lane:** normal (short architect design for the per-leg tables shared by the server-rendered laps and the browser-computed splits; one QA pass)
- **Requirements:** AP-8, AP-12, AP-20, AP-23; AP-7, AP-13, AP-16, AP-19, UP-6, SH-4, DS-1, DS-2, DS-6; NFR Performance (budgets unchanged), NFR Reliability
- **Depends on:** F-42 (legs, separator rows), F-46 (split presets, lap triggers), F-47 (trim), F-48 (pool lengths), F-51 (comparison chart), F-54 (four languages)

## Goal
A viewer of a triathlon reads each leg's laps and splits on their own, at a split distance that suits the leg's sport (100 m on the swim, 5 km on the ride, 1 km on the run), instead of one table mixing swim, ride and run rows.

## What exists
- A "Multisport" workout (AP-12: more than one sport) has one laps table with a separator row before each leg (F-42) and one splits table running across the legs, without separators, with one drop-down of the multisport list, 1 km default (AP-20, F-46, OQ-142). Lap columns follow the workout sport (OQ-142).
- Transitions (T1, T2) keep only position, distance, heart rate, elevation and temperature (OQ-38); laps and splits count their distance and time.
- No comparison chart for "Multisport" workouts (AP-23, F-51 OQ-204).

## Scope
Only for "Multisport" workouts (AP-12); a workout of one sport in several sessions keeps today's single tables.
- **One table per leg** (user, 22 Nov 2026), in both views: the laps view and the splits view each show a separate table per leg, in leg order, each headed by the leg's name (F-42 names). Replaces the laps table with separator rows (F-42) and the one splits table across legs (F-46); no way back to either. Tapping a leg's heading selects the leg as a range (AP-7), tapping again clears it. The Laps / Splits switch stays one switch for the whole section.
- **Laps per leg:** every device lap stays a lap (AP-8); a lap belongs to the leg its first sample lies in (F-42). A leg without device laps shows its heading alone. Lap numbers restart at 1 in each table. Rendered on the server, so the per-leg laps tables show with JavaScript off and on the edit page, as the laps table does today. Trigger column as AP-8, per table.
- **Splits per leg:** each leg's splits start at the leg's first sample and end at its last; no split crosses a leg boundary. Each leg ends with its own partial split showing its own distance (AP-8 rule, per leg); a leg shorter than its split distance gives one partial split. Split numbers restart at 1 in each table. A sport leg without distance shows its heading and "No splits".
- **Split distance** (OQ-296): each table's list and default from AP-20 for its leg's sport (swim 100 m, ride 5 km, run 1 km); a lap-swimming leg with one pool length uses the AP-20 pool rule (whole lengths), otherwise the open-water list. Presets longer than the leg are hidden; the default is always offered. Choice not remembered, not stored, not in the range link (AP-19); a change keeps the selected range.
- **Transitions** (OQ-297), **comparison chart** (OQ-299), **speed or pace column** (OQ-300): per the answers.
- **Which view opens:** unchanged (AP-8): laps when the workout has device laps, otherwise splits only.
- **Lengths view** (AP-13, F-48): unchanged.
- **Trim (UP-6):** legs, laps and splits follow the remaining part; a leg the trim cuts through gets splits from the trim point; a leg wholly trimmed away has no table. On the edit page the laps tables show the full recording, as today (OQ-178).
- **Privacy:** the tables show only distance, time and values already shown; hidden start and end (SH-4) does not change them, as today. Nothing new is stored, sent or logged.
- **Texts:** new text (e.g. "No splits") in English, German, Spanish and French (DS-6); leg names reuse F-42's.
- **Without JavaScript:** per-leg laps tables (server-rendered); no splits, as today.
- **Bundle:** splits and charts in the lazy laps-and-splits chunk, not in the entry bundle.

## Out of scope
- Time-based splits (Idea).
- Per-leg totals in the table heading; a per-leg summary.
- Any change to workouts that are not "Multisport".
- A sharer-chosen split distance.

## Constraints
- **Budgets:** entry bundle within the ADR-011 reserve; LCP, summary timing and 60 fps scrubbing on `ride-12h` unchanged within noise; data file unchanged; the server-rendered page grows only by the extra table markup.
- **Reliability:** truncated files keep the F-42 rule (samples after the last session belong to the last leg); a leg without laps, distance, speed or samples must not break the tables.

## Acceptance criteria
- [ ] On the corpus triathlon (swim, T1, ride, T2, run) the laps view shows one table per leg, each headed by the leg's name, on the viewing page with JavaScript on and off and on the edit page; no separator rows remain.
- [ ] The splits view shows tables Swim, Ride, Run in that order; T1 and T2 in both views as decided in OQ-297; no split spans two legs.
- [ ] A lap that starts in one leg and ends in the next is listed in the table of the leg it starts in; every device lap appears exactly once.
- [ ] Each splits table ends with a partial split showing its own distance; the sum of a table's split distances equals the leg's distance (within display rounding); lap and split numbers restart at 1 per table.
- [ ] Split distances per OQ-296: swim 100 m, ride 5 km, run 1 km preselected from their AP-20 lists; a change rebuilds only the affected table(s) and keeps the selected range (AP-7).
- [ ] A lap-swimming leg with one pool length splits by whole lengths (AP-20 pool rule); one without uses the open-water list.
- [ ] A run, ride, run file shows three tables per view, Run, Ride, Run.
- [ ] Tapping a lap or split selects its range; tapping a leg heading selects the leg; tapping again clears either.
- [ ] Speed or pace column as decided in OQ-300; trigger column as AP-8, per table.
- [ ] A reload, another workout and a range link open on the defaults; nothing is written to browser storage; the range link holds no split distance.
- [ ] A ride, ride workout (one sport) shows today's single laps and splits tables unchanged.
- [ ] Trimmed through the ride: the ride tables start at the trim point on the viewing page; a leg wholly trimmed away has no table.
- [ ] The comparison chart behaves as decided in OQ-299 (AP-23).
- [ ] New texts exist in all four languages and follow the language drop-down (DS-6).
- [ ] The splits and chart code is not in the entry bundle; scrubbing on `ride-12h` stays at 60 fps; LCP and summary timing unchanged within noise.
- [ ] Every corpus file renders; multisport corpus files show no error in either view.

## Decisions (user, 22 Nov 2026)
- Laps and splits are one table per sport leg; for "Multisport" workouts this replaces the laps table with separator rows (F-42) and the one splits table across legs (F-46). OQ-298 (laps view) is decided by this.

## Open questions
- [ ] **OQ-296 Split distance drop-down.** Suggested: one drop-down in each leg's splits table heading, with that sport's AP-20 list and default; two legs of the same sport are set independently. Alternative: one drop-down per sport, shared by its legs. Affects the table heading and AC 5.
- [ ] **OQ-297 Transitions.** Suggested: in the laps view a table like any leg (every device lap stays a lap, AP-8); in the splits view no table, only a one-line heading ("T1 · 1:12") that selects the transition when tapped. Alternative: a one-line heading in both views, transition laps not listed. Affects AC 2.
- [x] **OQ-298 Laps view.** Decided (user, 22 Nov 2026): one table per leg, as the splits.
- [ ] **OQ-299 Comparison chart (F-51).** Suggested: one chart under each leg's table, laps and splits alike, same rules as AP-23 (median per table, 2 to 200 bars); none for transitions (no speed). Alternative: still no chart for "Multisport". Affects AP-23 and AC 13; same lazy chunk.
- [ ] **OQ-300 Speed or pace column.** Suggested: each table shows speed or pace by its leg's sport (DS-2: swim /100 m, ride km/h, run /km), laps and splits alike; replaces OQ-142's workout sport for "Multisport" workouts. Alternative: by workout sport, as today. Affects AP-8 and AC 9.
