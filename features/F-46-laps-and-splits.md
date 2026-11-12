# F-46 Laps and splits: lap triggers and split presets

- **Phase:** 2
- **Owner:** both (backend first: lap trigger in the data file and the server-rendered laps table)
- **Lane:** normal (short architect note on the data file change and the lazy drop-down; one QA pass)
- **Requirements:** AP-8 (phase-2 part), AP-20 (Could, phase 2); AP-7, AP-16, AP-19, DS-1, DS-2; NFR Performance (budgets unchanged); decisions of 11–12 Nov 2026 in [README](README.md); OQ-164 to OQ-166 (decided 12 Nov 2026)
- **Depends on:** F-18, F-40, F-42, F-44

## Goal
A viewer sees what ended each lap and picks a split distance that suits the sport, for example 5 km splits on a long ride.

## What exists
- Laps and 1 km or 1 mi splits, the laps/splits switch and "No laps recorded." (F-18). Laps are rendered on the server; splits are computed in the browser (F-18 design L-4).
- The lap trigger is not read; the data file is format version 3 (technical design).
- Pool length per lap-swimming session in the metadata (F-44).

## Scope
- **Rules as in AP-8 and AP-20** (decided 11–12 Nov 2026): every device lap stays a lap; trigger words and when the column shows; splits only without device laps; preset lists, defaults, hidden long presets, imperial pages and pool rules; choice not remembered and not in the range link (AP-19); range kept on a change.
- **Lap trigger.** Read from each lap message on the server, carried in the data file (format version 4; `regenerate` rebuilds old workouts on deploy, losses accepted), rendered in the server-made laps table so it shows without JavaScript.
- **Drop-down.** Loaded with the laps table, not in the entry bundle (ADR-011 reserve: 3,345 bytes gzip).
- **Storage.** Nothing new beyond one trigger value per lap in the data file, deleted with the workout (UP-7, AC-4).

## Out of scope
- Time-based splits and splits per multisport leg (Later, spec scope table).
- Filtering or merging device laps; a sharer-chosen default split distance.
- Swim lengths table (AP-13).

## Acceptance criteria
- [ ] A file with manual, auto-distance and auto-position laps shows every lap, in order, with triggers Button, Distance and Position (AP-8).
- [ ] Each FIT trigger maps as AP-8: `manual` Button, `time` Time, `distance` Distance, `position_start`, `position_lap`, `position_waypoint`, `position_marked` Position, `session_end` Stop, `fitness_equipment` Equipment; an unknown or invalid value gives an empty cell.
- [ ] The column is headed "Trigger". A file whose laps all lack a known trigger shows no trigger column; splits and multisport separator rows never have a trigger cell.
- [ ] The trigger column shows on the viewing page with JavaScript off and on the edit page.
- [ ] A file with distance but no device laps opens on its splits: no laps view, no "No laps recorded.", no switch. A file with laps opens on laps, as today. No laps and no distance: no section. No laps, with distance, JavaScript off: no section.
- [ ] Each sport offers exactly its AP-20 list with its default preselected: a run 1 km, a row 500 m, a ride 5 km, an open-water swim 100 m, a pool swim 100 m, a hike and a multisport workout 1 km (one drop-down for the multisport workout).
- [ ] A 10.4 km run offers 400 m, 1 km, 5 km and 10 km, not 20 km or half marathon; at 5 km it gives splits of 5, 5 and 0.4 km. A 0.8 km run still offers 1 km (the default), as one partial split.
- [ ] A 25 m pool swim at 100 m splits every 4 lengths; recorded pool lengths of 32.5, 33.0 and 33.33 m split every 3 lengths, labelled "100 m" with distance "100 m", and the summary still shows "Pool 33 m"; 33.5 m is not taken as 33⅓; a 30 m pool at 100 m splits every 3 lengths, labelled "90 m"; a 50 m pool at 100 m every 2 lengths; a 25 yd pool at 100 yd every 4 lengths, labelled "100 yd", in metric and imperial units. Pool-swim sessions with different pool lengths, or none, get the open-water list.
- [ ] With a range selected, changing the distance keeps the range in charts, map and panel.
- [ ] A reload, another workout and a copied range link (AP-19) all open on the sport's default; the link holds no split distance and nothing is written to browser storage.
- [ ] The drop-down shows only in the splits view and only where splits exist.
- [ ] On an imperial page a run offers the metric list with 1 km preselected (no 1 mi splits); a 1 km split's distance shows 0.62 mi; switching units keeps the chosen distance.
- [ ] The drop-down's code is not in the entry bundle (entry within the ADR-011 reserve); scrubbing on `ride-12h` stays at 60 fps; LCP and summary timing unchanged within noise.
- [ ] Every corpus file uploads and renders after `regenerate` to format version 4.
