# F-44 Pool length and lengths in the summary

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** normal (short architect note on the metadata fields; one QA pass)
- **Requirements:** AP-16 (Could, phase 2); AP-2, DS-1, DS-5, UP-3; NFR Performance (budgets unchanged); decisions in [README](README.md) (AP-16 request, summary order of 10 Nov 2026, 11 Nov 2026)
- **Depends on:** F-15, F-19, F-41

## Goal
A viewer of a pool swim sees the pool length and the number of lengths swum, for example "25 m" and "64".

## What exists
- The parser reads neither the session's `pool_length`, `pool_length_unit` and `num_active_lengths` nor `length` messages; the data file metadata has no pool fields (ADR-005 item 7).
- `pool-swim.fit` (synthetic) has a lap-swimming session with `pool_length` 25 m and 40 active `length` messages, but no `num_active_lengths`. Every synthetic swimming session gets `pool_length` 25 m, including the open-water swims of the triathlon samples.

## Scope
- **Values, as AP-16.** The device's session values only, never computed: the number of active lengths (rest lengths not counted) and the pool length in the pool's own unit (`pool_length_unit`: metres or yards), not converted by the units setting (DS-1). Several pool-swim sessions: lengths summed; pool length shown only when every pool-swim session has the same one.
- **Which sessions count:** OQ-149.
- **Cells.** Two summary cells, "Pool" ("25 m", "25 yd") and "Lengths" ("64"), each hidden on its own when its value is missing, never shown as zero. Precision: OQ-150.
- **Place and tint.** No chart line, so untinted (DS-5, F-41), placed with the untinted cells: after Calories, before the first tinted cell (summary order, 10 Nov 2026).
- **Pages.** Viewing page (rendered on the server, works without JavaScript) and edit page, the same cells.
- **Without pool data** (older workouts, files without these values, truncated files without session totals, UP-3, open water, other sports): no cells, the summary exactly as today.
- **Data file.** New metadata fields bump the version (ADR-005 item 9). No compatibility work: a workout built before the change may break or be lost (decision of 11 Nov 2026).
- **Corpus.** `pool-swim.fit` gains `num_active_lengths` and one rest length; a 25 yd pool swim is added; golden files regenerated.

## Out of scope
- Lengths table and stroke type, no map for pool swims (AP-13).
- Swim distance in yards (DS-2 stays as it is).
- Pool values in the range panel, laps and splits, cursor readout and preview card.

## Acceptance criteria
- [ ] `pool-swim.fit` shows "Pool 25 m" and "Lengths" equal to the session's `num_active_lengths`; the rest length is not counted.
- [ ] A 25 yd pool swim shows "25 yd" in both metric and imperial units.
- [ ] A lap-swimming session with `pool_length` but no `num_active_lengths` shows "Pool" only; with neither, no pool cells.
- [ ] Open-water swims (`triathlon.fit`), rides and runs show no pool cells.
- [ ] Two pool-swim sessions with the same pool length show it once with the lengths summed; with different pool lengths, "Lengths" only.
- [ ] Both cells are untinted in both themes and sit after Calories and before the first tinted cell.
- [ ] Viewing page with JavaScript off and edit page show the same cells.
- [ ] A truncated pool swim without session totals (UP-3) shows no pool cells and no error.
- [ ] Data file size and LCP are unchanged within noise; every corpus file still uploads and renders.

## Open questions
- [ ] **OQ-149 Which sessions count as pool swims?** Devices may record a pool length on open-water swims (the synthetic triathlons do). Suggestion: only sessions with sub-sport "lap swimming"; their values are used as recorded. Affects which workouts show the cells and the multi-session rule.
- [ ] **OQ-150 How precise is the pool length?** Suggestion: whole metres or yards, one decimal only when not whole (33.3 m), recorded value rounded to the nearest 0.1 in the pool's unit. Affects the "Pool" cell.
