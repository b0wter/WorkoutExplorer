# F-69 Time-based splits: architecture

- **Version:** 0.1
- **Status:** accepted (24 Nov 2026). Normal lane. Product decisions OQ-333 to OQ-337 taken by the user (feature file); one open question (F69-Q1), not blocking: the suggestion is built unless the user objects
- **Feature:** [`features/F-69-time-based-splits.md`](../features/F-69-time-based-splits.md)
- **Requirements:** AP-20, AP-8; AP-5, AP-7, AP-12, AP-13, AP-19, AP-23, DS-1, DS-6; NFR Performance, NFR Reliability
- **Builds on:** [F-46](F-46-laps-and-splits.md) LS-5 to LS-9; [F-64](F-64-laps-and-splits-per-multisport-leg.md) LL-4 to LL-7; [F-51](F-51-split-comparison-chart.md) SC-3, SC-4, SC-10; [F-18](F-18-laps-and-splits-table.md) L-4; [ADR-029](ADR-029-cumulative-moving-time-and-gain-in-the-data-file.md); ADR-011; F-54 text groups. No server, data file, template or setting change
- **Owner:** frontend only. No backend step: the moving-time stream is already published (ADR-029), and the server-rendered page has no splits (AP-20, feature Out of scope)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| TS-1 | **The clock is the published moving-time stream `M`** (ADR-029: cumulative whole seconds per sample, never missing, on the viewing page's public file and on the edit page's `draft.bin`). The browser derives nothing and holds no moving rule (ADR-029 item 3). A table's clock is `M[i] − M[a]`, `a` its window's first sample, so trimmed or hidden starts need nothing. **No `M` → no "Time" group.** A stop with the timer running counts as moving, as R-7 counts it (timer events win over the speed rule); the AC 3 fixture must stop the timer or have no timer events | One rule in one language; a row's time equals what the range panel shows for it, by construction |
| TS-2 | **Same boundary rule, other series.** Time splits run the existing windowed rule (F-18 L-4, F-64 LL-4) over `M` instead of `d̄`, with the preset in seconds; no second boundary function, no interpolation (a row is a sample range, AP-7). Consequences, all from that rule: boundary *k* is the first sample with `M − M[a] ≥ k·L`; multiples are absolute, never re-anchored, so rows sum to the window's moving time and an overshoot (sample spacing, smart recording, or a gap) is taken from the next row; a gap across several multiples ends one row only (no empty rows); a pause (flat `M`) lies inside its row, and a boundary reached at a pause's first sample puts the pause into the next row; the last partial row ends at the window's end when `M` still advances, a tail without moving time is in no row; "partial" for the chart uses the same rule (SC-4) | AC 2 to AC 4 follow without special cases; distance and time splits cannot drift apart |
| TS-3 | **Offer.** A table gets the "Time" group when (a) it is not a pool swim, (b) `M` exists and (c) its window's moving time is ≥ 60 s. Pool swim = the table splits by whole lengths (LS-6, LL-6), or its window's session is lap swimming (AP-16) even without a shared pool length, so the spec's "not for pool swims" holds literally. A preset is offered when its length ≤ the window's moving time (equal is offered, as for distance). Drop-down: distance presets as today, then one native `<optgroup>` "Time" with 1, 5, 10, 15, 30, 60 min; no new control. The default stays the sport's distance default (OQ-337) and is never in the time group. The drop-down's accessible name ("Split distance") is reworded to cover both kinds, in all four languages | Feature Scope "Presets", "Where offered" |
| TS-4 | **Columns.** Same columns, same `lapsGroup` formatting, same range values per row (distance, speed = distance ÷ moving time, gain, averages). Only the time cell differs: a time split shows the row's moving time `M[b] − M[a]`; distance splits keep elapsed time. Header: see F69-Q1 | Feature Scope "Columns"; one formatting home |
| TS-5 | **Row tap and range.** Unchanged path: row *i* selects `[boundary i−1, boundary i]` through the LS-8 seam. The range panel then shows the pause in elapsed time and the row's time as moving time (AC 3). Rows do not line up with the time axis (AP-5); no marker or axis change is added for that. A row's range in the link (AP-19) is an ordinary range; the preset itself is never in the fragment | AP-7, AP-19; no second selection path |
| TS-6 | **Comparison chart** (F-51) unchanged: slots from the rows the table is built from, caption with the preset label. AP-23's limit of 200 bars stands, so 1-min splits of long workouts (the 12-h file: about 720 rows) show the table without a chart | User decision of 14 Nov 2026 (AP-23); no new chart code |
| TS-7 | **Multisport** (F-64): each sport leg's window `[a, b]` from LL-4, clock from the leg's first sample, offer rule TS-3 on the leg's own moving time `M[b] − M[a]`. Transitions have their own windows and no drop-down, so their time counts toward no row. A pool-swim leg has no time group; an open-water leg has one | AC 5; windows partition the stream |
| TS-8 | **State and rebuild** as LS-7, LL-6: the choice is an index into one fixed list per table, in chunk memory only; a units switch keeps it (time presets do not depend on units); a language change relabels and keeps the index (the list's order does not depend on the language); the selection is untouched. Built only on LS-9's triggers, O(window + rows), never on a cursor or selection flush | AP-20 behaviour; AC 9 scrubbing |
| TS-9 | **Where the code lives.** Preset list and offer rule in `lib/splits` (pure, Node-tested); series choice, optgroup and the time cell in `src/splits/` (lazy chunk). Texts ("Time", "N min", the reworded accessible name) in the splits text files, so the de/es/fr strings land in the existing `page` text group (F-54 size cap applies). Nothing in the viewing entry; the chunk's import condition is unchanged (LS-4) | ADR-011: entry at about 60,955 of 62,000 |

## Cost

| What | Decision |
| --- | --- |
| Static (entry) | **Target and ceiling 0 bytes.** Measure before and after; any growth: stop and report (AC 9) |
| Lazy | `src/splits/` and the `page` text group grow by the presets, offer rule and texts; report the chunk's gzip size before and after. Not counted |
| Data file, HTML, LCP | Unchanged |

## Verification (what must be shown, not how)

- Boundaries over `M`: AC 1 to AC 4 on synthetic series, including a stop at a boundary, a gap across one and across two multiples, smart-recording spacing, a window under 1 min, a trimmed start (`M[a] ≠ 0`).
- The rows of one table sum to the window's moving time; a row tap's panel moving time equals the row's time.
- Distance splits unchanged: the existing splits, lengths and chart tests pass without edits (an edit is a deviation to report).
- Manual: AC 8 and AC 9 on `ride-12h` (1-min splits, no chart, 60 fps), the corpus triathlon, German.

## Open question

| ID | Question | Suggestion |
| --- | --- | --- |
| F69-Q1 | Time column header of a time split: it shows moving time, while a distance split's "Time" shows elapsed time. Keep "Time", or "Moving time"? | Keep "Time": no new text, no wider column at 360 px; rows of exactly the preset make the clock evident, and the range panel names moving time |

## Hand-offs

| To | What |
| --- | --- |
| Requirements | Feature file Constraints: "and its comparison chart" for 1-min splits on the 12-hour file conflicts with AP-23's 200-bar limit (TS-6); reword to "the table" |
| In place, on build | F-46 LS-5 and F-64 LL-6: noted "time presets: F-69 TS-3" |
