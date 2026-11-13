# F-48 Pool swims: lengths and stroke type

- **Phase:** 2
- **Owner:** both (backend first: reading `length` messages and strokes, data file version 5, the server-made Stroke column and summary cell; then the Lengths view)
- **Lane:** normal (short architect note on the data file change and the lazy Lengths view; one QA pass)
- **Requirements:** AP-13 (Could, phase 2; decided 13 Nov 2026); AP-2, AP-3, AP-7, AP-8, AP-12, AP-16, AP-19, AP-20, UP-3, UP-6, UP-7, AC-4, DS-1, DS-2, DS-5; NFR Performance (budgets unchanged), NFR Reliability; decisions of 13 Nov 2026 in [README](README.md); OQ-180 to OQ-184 (resolved 13 Nov 2026)
- **Depends on:** F-44, F-46, F-47

## Goal
A viewer of a pool swim sees which stroke each lap was and every length with its stroke, time, pace, stroke count and SWOLF.

## What exists
- The summary's "Pool" and "Lengths" cells from the session values of lap-swimming sessions (F-44, AP-16). The parser reads no `length` messages and no stroke (technical design, FIT messages).
- Laps and splits views with the "Trigger" column, server-rendered laps, browser-computed splits, data file format version 4 (F-46).
- Trim keeps the pool length and drops the lengths count (F-47, OQ-179; the count changes with this feature, OQ-181); on the edit page laps show the full recording (OQ-178).
- `pool-swim.fit` (synthetic): 25 m, 40 active lengths, one rest length, `num_active_lengths` (F-44).

## Scope
- **Which swims.** Only lap-swimming sessions (as AP-16). Open-water swims, other sports and other legs of a multisport workout get no stroke and no lengths (OQ-182).
- **Stroke words.** From the FIT `swim_stroke`: Freestyle, Backstroke, Breaststroke, Butterfly, Drill, Mixed, IM (`im`, `im_by_round`, `rimo`); unknown or missing: empty cell (OQ-182).
- **Laps view: "Stroke" column** (decision 1). One word per lap from the lap's recorded stroke; shown only when at least one lap has a known stroke; splits and multisport separator rows have none. Server-rendered, so it shows with JavaScript off, like "Trigger".
- **Summary: "Stroke" cell** (decision 1). Shown when every active length of every lap-swimming session has the same known stroke other than Mixed or IM; without `length` messages, when every lap-swimming session records the same such stroke (OQ-182). Untinted (DS-5), right after "Lengths". Hidden otherwise, never shown empty.
- **Lengths view** (decisions 2–4). A third view next to Laps and Splits, shown only when the workout has at least one length in a lap-swimming session. One row per length in recorded order:
  - Active length: number (1, 2, …, across the whole workout, no separator rows), stroke, time, pace, strokes, SWOLF.
  - Rest (idle) length: a "Rest" row with its time only; never numbered or counted (decision 3).
  - Time: the length's recorded time (`m:ss`). Pace per 100 m or 100 yd as DS-2 and the units setting (DS-1), from the pool length and the length's time; without a pool length, from the length's recorded average speed; neither: "–" (OQ-183).
  - "Strokes" and "SWOLF" columns only when at least one active length has a stroke count; SWOLF = the length's time rounded to the nearest whole second plus its stroke count; a length without a stroke count shows "–" in both (decision 4, rounding OQ-183).
  - The page still opens on Laps (or Splits without device laps, AP-8); the choice is not remembered. Tapping a row selects that length as a range (AP-7), as for laps (OQ-183).
  - Loaded with the laps table, not in the entry bundle; needs JavaScript, so no Lengths view with JavaScript off (OQ-183).
  - Untrimmed, the "Lengths" summary cell stays the device's count (AP-16), even where it differs from the table's numbering.
- **Map.** Nothing new (decision 5): pool swims without positions show the no-location hint (AP-3).
- **Trim** (decision 6, UP-6). Viewing page: lengths wholly trimmed away disappear; a length the trim cuts through is dropped (OQ-180); numbering starts again at 1; the summary "Stroke" cell is taken from the remaining lengths; the summary "Lengths" cell counts the remaining whole active lengths, and stays dropped without `length` messages (OQ-181; replaces OQ-179's dropped count, UP-6, AP-16). Edit page: all lengths, as laps show the full recording (OQ-178). No trimmed length reaches any public file.
- **Truncated files** (UP-3): the lengths recorded before the cut are shown; the summary cells follow AP-16.
- **Data file.** Lengths and strokes go in the data file, format version 5 (decision 7); `regenerate` rebuilds old workouts on deploy, losses accepted. Nothing else is stored; deleted with the workout (UP-7, AC-4). Lengths carry no positions; hiding (SH-4) does not affect them.
- **Corpus.** `pool-swim.fit` gains strokes and stroke counts per length and lap; a mixed-stroke pool swim (freestyle, backstroke, drill without stroke counts, two rest lengths) and a 25 yd pool swim with strokes are added; golden files regenerated.

## Out of scope
- Swim distance in yards (Later, DS-2; OQ-184).
- Stroke or length values in the range panel, cursor readout, charts, splits and preview card.
- A distance column in the Lengths view (every length is one pool length).
- Lengths for open-water swims; drawing or detecting lengths from data.

## Acceptance criteria
- [ ] `pool-swim.fit` (all freestyle) shows a "Stroke" column with "Freestyle" in every lap, and a summary cell "Stroke Freestyle" right after "Lengths", untinted in both themes (AP-13, DS-5).
- [ ] The mixed-stroke swim shows each lap's own stroke and no summary "Stroke" cell; a lap without a known stroke has an empty cell; each FIT stroke maps to its word as in Scope.
- [ ] A swim whose laps all lack a stroke shows no Stroke column; open-water swims (`triathlon.fit`), rides and runs show no Stroke column, no "Stroke" cell and no Lengths view.
- [ ] The Stroke column and the summary cell show on the viewing page with JavaScript off and on the edit page (AP-8).
- [ ] The section offers Laps, Splits and Lengths on a pool swim with lengths and opens on Laps; a pool swim without `length` messages has no Lengths view (AP-8, AP-20).
- [ ] The Lengths view of `pool-swim.fit` has 40 numbered rows and one "Rest" row; the rest row has no number, shows only its time, and the numbering continues after it.
- [ ] Each active row shows stroke, time, pace per 100 m (per 100 yd on an imperial page, DS-1, DS-2), strokes and SWOLF; a 25 m length of 0:21.6 with 14 strokes shows SWOLF 36.
- [ ] Drill lengths without stroke counts show "–" in Strokes and SWOLF; a swim without any stroke count has no Strokes and SWOLF columns.
- [ ] Tapping a length row selects that length as a range in charts and panel (AP-7).
- [ ] The summary "Lengths" cell still shows the device's `num_active_lengths` (AP-16).
- [ ] A pool swim trimmed into its middle: the viewing page drops the trimmed lengths and the length the trim cuts through, numbers from 1, and no public file holds a trimmed length; the edit page lists every length; the "Stroke" and "Lengths" cells follow the remaining lengths, "Lengths" equal to the remaining numbered rows (UP-6, AP-16). A trimmed pool swim without `length` messages shows no "Lengths" cell.
- [ ] A truncated pool swim (UP-3) shows its recorded lengths and no error.
- [ ] Every corpus file uploads and renders after `regenerate` to format version 5.
- [ ] The Lengths view's code is not in the entry bundle (within the ADR-011 reserve); data file size, LCP and 60 fps scrubbing on `ride-12h` unchanged within noise.

## Open questions
All resolved by the user on 13 Nov 2026 as suggested (decision log in [README](README.md)):
- **OQ-180:** a length the trim cuts through is dropped; laps keep their remaining part, lengths do not.
- **OQ-181:** after a trim, the summary "Lengths" counts the remaining whole active lengths; without `length` messages it stays dropped. Replaces OQ-179's dropped count (UP-6, AP-16).
- **OQ-182:** strokes for lap-swimming sessions only; words and summary rule as in Scope.
- **OQ-183:** Lengths view details as in Scope.
- **OQ-184:** swim distance in yards is Later; DS-2 reworded.
