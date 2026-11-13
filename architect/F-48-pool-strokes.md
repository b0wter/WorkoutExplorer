# F-48 Pool swims: lengths and stroke type: architecture

- **Version:** 0.1
- **Status:** accepted (13 Nov 2026). Normal lane. OQ-180 to OQ-184 decided by the user on 13 Nov 2026 as suggested and built in here; OQ-185 open, not blocking (suggestion built unless the user objects)
- **Feature:** [`features/F-48-pool-strokes.md`](../features/F-48-pool-strokes.md)
- **Requirements:** AP-13, AP-16, AP-8, AP-7, AP-2, AP-3, AP-20, UP-3, UP-6, DS-1, DS-2, DS-5; NFR Performance, NFR Reliability; decision log 13 Nov 2026
- **Builds on:** new [ADR-048](ADR-048-lengths-and-strokes-in-the-data-file.md) (format version 5); [F-44](F-44-pool-summary.md) PS-1, PS-3 (PS-8 changed here); [F-46](F-46-laps-and-splits.md) LS-2 to LS-4, LS-7 to LS-9 (LS-3, LS-4 changed here); [F-47](F-47-trim.md) TR-5 (changed here), ADR-047; ADR-017 rev. 9 (`MaxLengths`), ADR-018; ADR-011, ADR-026, ADR-027
- **Owner:** both. Backend first (PL-1 to PL-7), then the frontend (PL-8, PL-9)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| PL-1 | **Read in `ingest`.** `length` messages (start, end, timer time, stroke count, average speed, `swim_stroke`, `length_type`), and `swim_stroke` of every `lap` and `session`. `swim_stroke` maps to a closed code set (ADR-048 item 2) in `ingest`; the FIT enum never leaves it. Zero or invalid values are not recorded. New counted budget `MaxLengths` = 10,000, limit name `lengths` (ADR-017 rev. 9, ADR-018). Lengths join *k* in ADR-017 item 2's O((n + k) log(n + k)) bound: no loop over sessions per length beyond a sorted sweep | Anti-corruption layer; a hostile file of empty `length` messages must hit a budget, as laps do |
| PL-2 | **Scope rule in `workout` (`Normalise`).** A length, and a lap's stroke, belongs to the session whose span holds its start. In a session that is not a pool swim (PS-1) the length is dropped and the lap stroke cleared. A length or lap that no session covers (a truncated file that lost its `session`, UP-3) is kept (OQ-185). Lengths are ordered by start; offsets by R-1 and E-2; index ranges by F-03 §5.4 | AP-13 "lap-swimming sessions only", and F-48 AC "a truncated pool swim shows its recorded lengths": devices write `length` only in pool swims, and the session comes last in the file |
| PL-3 | **Per-length values, in `workout`.** Rest = `length_type` idle; anything else is active. `time` = the recorded timer time. `avg_speed` = the session's recorded pool length (metres) ÷ `time`; without a pool length, the recorded average speed; neither: absent. A rest length keeps only `time`. A stroke count of 0 is not recorded (as PS-2) | OQ-183. The fallback rule lives once, on the server; the browser formats pace from `avg_speed` exactly as for laps (DS-2) |
| PL-4 | **Summary stroke, in `workout` beside `setPool`.** Per pool swim: the strokes of its active lengths when it has `length` messages, else its session's stroke. `summary.stroke` is set when there is at least one and all are one known code other than `mixed` and `im`; otherwise absent. Pool swims only, so a session-less truncated file has none (AP-16) | OQ-182; computed once on the server, so the no-JS page and the owner's trimmed view (TR-8) get it with no new rule |
| PL-5 | **Data file version 5** (ADR-048): top-level `lengths`, lap `stroke`, summary `stroke`. Bytes normative in [F-03](F-03-workout-data-file-writer.md) §5 | Decision 7 of 13 Nov 2026 |
| PL-6 | **Trim** (extends TR-5, OQ-180, OQ-181). `Trim` keeps a length only when `start ≥ offset[a]` and `end ≤ offset[b]`: a cut length is dropped. Kept lengths are rebased and their index ranges recomputed. Lap strokes stay with their laps (as `trigger`); a clipped session keeps its stroke (as its pool length). The summary stroke is re-run on the cut (PL-4). Summary `lengths` = the number of remaining active lengths; absent when that is 0 or a remaining pool swim has no `length` messages. Hiding never touches lengths (no positions). `draft.bin`, so the edit page, keeps every length | UP-6 "as if recorded that way"; AP-16 as decided. No trimmed length can reach a public file: the public build sees only the transform's output (TR-7) |
| PL-7 | **Server markup.** Both laps builders (Go `internal/page`, TS `lib/page`): key `stroke`, heading "Stroke", after the numeric columns and before "Trigger" (the numeric columns still line up with splits), shown when at least one lap has a code; empty cell otherwise; splits and separator rows have none. Summary: key `stroke`, label "Stroke", word as value, no unit, right after `lengths`; not in F-41's tint list; `CardItems` ignores it. The units enhancer rewrites both to the same text. Page samples (ADR-026) cover one stroke throughout, mixed strokes with an unknown lap, and no strokes. LS-3 changes: the hidden module is also rendered when the workout has lengths and no device laps | ADR-026: no-JS page and edit page agree cell for cell |
| PL-8 | **Lengths view, lazy** (changes LS-4). It lives in the existing `src/splits/` chunk, which becomes the laps module's lazy part; its pure rules (row numbering across the workout, rest rows unnumbered, which columns show, SWOLF) in a new `lib/lengths`, tested in Node. `src/page` imports the chunk when the distance advances **or** `meta.lengths` is not empty. Views: Laps (device laps), Splits (distance advances), Lengths (at least one length); the switch shows when two or more exist; the page opens on the first; the choice is not remembered (LS-7). Time and SWOLF use one whole-second rounding, half away from zero, so the time cell and SWOLF agree (0:21.6 shows 0:22, SWOLF 36 with 14 strokes). Row taps, rest rows included, go through the LS-8 seam with the length's index range. A units switch re-formats pace only. The table is built only on chunk arrival, a switch to it or a units change, never on a cursor or selection flush (LS-9) | AP-13 "loaded with the laps table"; one tap path; one chunk and one switch instead of two coordinating chunks. Cost: rides fetch a few hundred lazy bytes they never use |
| PL-9 | **Budget.** Static (entry) ceiling **300 bytes gzip**: reader checks for the v5 keys, stroke words, the Stroke column and cell, the extended import condition. The Lengths view is lazy, not counted. Measure the entry before and after and record it in ADR-011 item 4; above 300: stop and report. Data file: every workout gains `"lengths":[]` and a `stroke` per lap; `ride-12h` size within noise (F-48 AC) | ADR-011 reserve 3,608 at entry 56,392 |

**Module boundary** (F-46's table, changed):

| Module | Holds | May import |
| --- | --- | --- |
| `ingest` (Go) | FIT `length`, `swim_stroke` → codes; `MaxLengths` | unchanged |
| `workout` (Go) | PL-2 to PL-4, PL-6 | unchanged |
| `lib/lengths` (new) | PL-8 row rules | nothing |
| `src/splits/` (chunk) | switch with three views, splits table, lengths table | as F-46, plus `lib/lengths` |

## Verification

Synthetic files only (ADR-004). Beyond the feature's ACs:

| What | Check |
| --- | --- |
| Trim leak (UP-6) | F-47's marker check, extended: the mixed-stroke pool swim with marker stroke counts and times in the lengths before the window and in the length the window cuts through, trimmed at each end. No file under `public/{public_id}/` holds a marker. Owner `trimmed.summary` equals the public summary (TR-8 row), `stroke` and `lengths` included |
| Budget | ADR-017 item 7's all-caps file gains 10,000 lengths; the stop rule is unchanged |
| Mutants | Each must fail a test: a cut length kept; lengths outside a pool swim kept; the summary `lengths` after a trim from the device count; SWOLF from the unrounded time |

## Hand-offs

| To | What |
| --- | --- |
| Backend | F-03 §5 is at version 5 (this round); TD FIT messages, Workout data file and Conversions updated; ADR-017 rev. 9 and ADR-018's limit list updated |
| In place | F-44 PS-8, F-46 LS-3 and LS-4, F-47 TR-5, ADR-047 item 1, ADR-046 "Would choose differently if" point here |

## Open questions

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-185 | A truncated pool swim usually loses its `session` (written last), so no lap-swimming session holds its lengths, yet the feature's AC wants them shown. | Keep lengths and lap strokes that no session covers; drop them only inside a session that is not a pool swim (PL-2). Summary cells stay hidden (AP-16) | Nothing (suggestion built unless the user objects) |
