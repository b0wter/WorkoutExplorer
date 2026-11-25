# F-72 Swim distance in yards

- **Status:** planned (25 Nov 2026); OQ-348 to OQ-351 assumed
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (`lib/format` owns every unit label and factor; the server markup stays metric and is rewritten in place, ADR-027)
- **Lane:** normal (short design note on where the unit is chosen per sport and leg; one QA pass)
- **Requirements:** DS-2, DS-1; AP-2, AP-5, AP-6, AP-7, AP-8, AP-12, AP-13, AP-16, AP-20, AP-21, AP-23, PIN-8, DS-6; NFR Reliability
- **Depends on:** F-14 (units), F-44, F-48 (pool), F-46, F-69 (splits), F-64, F-67 (multisport legs), F-56, F-65 (distances on the edit page), F-54 (four languages)

## Goal
A swimmer on an imperial page reads swim distances in yards ("1,650 yd"), not in miles ("0.94 mi").

## What exists
- DS-2 today: every distance is in km or mi, swims included; an imperial swim shows miles and pace per 100 yd. The units drop-down has Metric and Imperial (DS-1).
- The pool length cell (AP-16) and pool split labels and distances (AP-20) already use the pool's own unit (m or yd), whatever the units setting.

## Scope
- **Where** (OQ-348): workouts whose sport is swimming (pool and open water) on an imperial page show every distance in yards instead of miles. Metric pages are unchanged (km), a yard pool included.
- **Units drop-down unchanged** (OQ-349): still Metric and Imperial; no yards option, nothing new stored.
- **Format** (OQ-350): whole yards with the language's thousands separator (DS-6), unit "yd": "1,650 yd", "1.650 yd" in German.
- **What changes:** summary distance (AP-2); distance column of laps, splits and the Lengths view (AP-8, AP-13; a non-pool split label stays metric, its distance column shows yards, AP-20); range panel distance (AP-7); cursor readout distance, including fullscreen (AP-6, AP-21); distance x axis ticks and label (AP-5); distances in the edit page's pins list and added-laps table (PIN-8, F-65).
- **Unchanged:** pace per 100 yd (DS-2); pool length cell and pool split labels (pool's unit); elevation in ft; speed chart; the comparison chart (shows no distance); the server-rendered page with JavaScript off, the preview card and image, and the GPX download (metric or unitless, as today).
- **Multisport** (OQ-351): only the swim leg's own laps and splits table and the leg view of a swim leg (F-67: axis, readout, range panel inside the leg) use yards; the summary, the whole-workout view and other legs stay in miles.

## Out of scope
- Metres for metric swims (OQ-350 note); a yards option for other sports; per-workout unit set by the sharer.

## Acceptance criteria
- [ ] A 1,500 m pool swim (25 m pool) on an imperial page shows "1,640 yd" in the summary, yards in the laps, splits and Lengths columns, range panel, cursor readout and distance axis; pace stays per 100 yd; the pool cell stays "25 m".
- [ ] A 25 yd pool swim of 66 lengths on an imperial page shows "1,650 yd"; on a metric page "1.51 km", pool cell "25 yd".
- [ ] An open-water swim on an imperial page shows yards; its 100 m splits keep the label "100 m" with "109 yd" in the distance column.
- [ ] A run and a ride on an imperial page still show miles.
- [ ] Switching Metric ↔ Imperial on a swim rewrites every listed place without a reload; the choice is remembered as today (DS-1).
- [ ] A triathlon on an imperial page: summary and whole-workout axis in miles; the swim leg's tables and the swim leg view in yards; bike and run legs in miles.
- [ ] German, Spanish and French pages use their thousands separator; 360 px portrait shows "1,650 yd" without overflow in the summary and readout.
- [ ] With JavaScript off the page is unchanged (metric); entry bundle within the ADR-011 reserve; 60 fps scrubbing unchanged; every corpus file renders.

## Open questions
- [ ] **OQ-348 Where yards apply.** Assumed: every swim (pool and open water) on an imperial page; a yard pool on a metric page stays km. Alternative: yards only for swims in a yard pool, whatever the units setting. Affects DS-2 and every distance on a swim page.
- [ ] **OQ-349 Units drop-down.** Assumed: unchanged (Metric / Imperial), yards follow from Imperial plus the sport. Affects DS-1 and stored preferences.
- [ ] **OQ-350 Metric swims.** Assumed: stay in km with 2 decimals (only yards were asked for); metres ("1,500 m") would be the matching change for metric swimmers. Affects DS-2 and this feature's size.
- [ ] **OQ-351 Multisport.** Assumed: yards only in the swim leg's tables and swim leg view; summary and whole-workout view stay in miles. Affects AP-12, F-64, F-67.
