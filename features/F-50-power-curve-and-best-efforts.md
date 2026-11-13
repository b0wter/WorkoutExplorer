# F-50 Power curve and best efforts

- **Phase:** 2
- **Owner:** frontend (computed in the browser from the streams already in the data file; no server change assumed)
- **Lane:** normal (short architect note on the computation's cost on long workouts and the lazy section; one QA pass)
- **Status:** ready for architecture (after F-49) — OQ-186 to OQ-194 resolved 13 Nov 2026 (decision log in [README](README.md))
- **Requirements:** AP-11 (Could, phase 2; picked 13 Nov 2026); AP-5, AP-7, AP-8, AP-12, AP-19, UP-3, UP-6, SH-4, DS-1, DS-2, DS-4; NFR Performance (budgets unchanged), NFR Reliability
- **Depends on:** F-17, F-18, F-40, F-42, F-47, F-49 (shared section layout)

## Goal
A viewer sees the workout's best efforts (fastest distances, best power over set durations) and its power curve, and can jump to each effort.

## What exists
- Power, distance and speed streams in the data file; range selection (F-17), tap-to-select rows (F-18), range in the link (F-40), multisport legs (F-42), trim (F-47).

## Scope (assumed answers in brackets; see Open questions)
- **Which sports** (OQ-192): distance efforts for run, walk and hike (the pace sports, DS-2); power efforts and the power curve for any sport with power. Swims, and rides without power, get nothing.
- **Values** (OQ-192): distances 1 km, 1 mi, 5 km, 10 km, half marathon, marathon; power durations 5 s, 1 min, 5 min, 20 min, 60 min. Only those the leg is long enough for.
- **Definition** (OQ-193): fastest distance = shortest moving time over that distance; best power = highest average power over that much moving time, missing samples counted as 0 W (AP-7). Pauses add no time; an effort never crosses a leg (OQ-191).
- **Best efforts table** (OQ-189): effort, time and pace (distance, DS-2 and units DS-1) or average watts (power), and where it starts (elapsed time). Tapping a row selects the effort as a range (AP-7), which the range link can share (AP-19) (OQ-194).
- **Power curve** (OQ-189): best average power for every duration from 1 s to the longest leg with power, logarithmic time axis.
- **Which data** (OQ-190): the public data after the trim; hiding (SH-4) has no effect; whole workout only, not the selected range.
- **Lazy and cheap:** section code and computation load after the page is interactive and must not block scrubbing; a 12-hour ride stays within the budgets.

## Out of scope
- Records across workouts, personal bests, estimated FTP or VO2max, critical power models.
- Best efforts for heart rate, cadence, elevation or swims; distance efforts for rides.
- Efforts in the summary, preview card or charts; following the selected range.

## Acceptance criteria
- [ ] A synthetic run of 12 km shows 1 km, 1 mi, 5 km and 10 km efforts, not half marathon or marathon; times and paces match hand-computed values on the synthetic data (AP-11, DS-2).
- [ ] A ride with power shows 5 s to 60 min power (those the ride covers) and a power curve; a ride without power shows no Best efforts section; a swim shows none.
- [ ] An effort spanning a pause excludes the pause time; an effort never crosses legs or transitions in `triathlon.fit` (AP-12).
- [ ] Tapping a row selects that effort as a range in charts, map and panel (AP-7).
- [ ] A trimmed workout's efforts come only from the remaining part (UP-6); hiding the start and end changes nothing.
- [ ] Imperial pages show paces per mile; distances keep their names (DS-1).
- [ ] A truncated file (UP-3) shows efforts without error.
- [ ] The section's code is not in the entry bundle; LCP and 60 fps scrubbing on `ride-12h` unchanged within noise; the curve for `ride-12h` is ready within 1 s on the reference phone after the section is shown.

## Open questions
All resolved by the user on 13 Nov 2026 as suggested (decision log in [README](README.md)); OQ-189 to OQ-191 in [F-49](F-49-zones.md) apply here too:
- **OQ-192:** sports and values as in Scope.
- **OQ-193:** moving time, missing power as 0 W, within one leg; distance window ends interpolated between samples.
- **OQ-194:** tapping a row selects the effort as a range, like laps (AP-8); tapping the curve does nothing.
