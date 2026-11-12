# F-46 Laps and splits: lap triggers and split presets: architecture

- **Version:** 0.1
- **Status:** accepted (12 Nov 2026). Normal lane. OQ-167, OQ-168 open, not blocking (suggestions built unless the user objects)
- **Feature:** [`features/F-46-laps-and-splits.md`](../features/F-46-laps-and-splits.md)
- **Requirements:** AP-8, AP-20, AP-7, AP-16, AP-19, DS-1, DS-2; NFR Performance; decision log 11–12 Nov 2026, OQ-164 to OQ-166
- **Builds on:** [ADR-046](ADR-046-lap-trigger-and-recorded-pool-length-in-the-data-file.md) (format version 4); [F-18](F-18-laps-and-splits-table.md) L-2 to L-7 (L-4, L-6, L-8 changed here); [F-40](F-40-shareable-range.md) SR-1; [F-42](F-42-multisport-sections.md) MS-6, MS-9; [F-44](F-44-pool-summary.md) PS-1, PS-3; ADR-011, ADR-026, ADR-027
- **Owner:** both. B-1 backend first (LS-1 to LS-3, samples, template), then the frontend (LS-4 to LS-9)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| LS-1 | **Trigger in the data file** (ADR-046 items 2, 3): a `trigger` string per lap in the metadata (no column), `""` for unknown. Mapped from FIT `lap_trigger` in `ingest`; `workout.Lap` carries the domain value; `datafile` writes it. Hiding and trim never change it | Privacy merge before anything is stored; one mapping, in the anti-corruption layer |
| LS-2 | **Trigger column in both laps builders** (Go `internal/page`, TS `lib/page`): key `trigger`, heading "Trigger", **last column** (the numeric columns then line up with the splits table, which has none). Shown when at least one lap has one of the six codes; a lap without one gets an empty cell, not "–". Splits and separator rows never have a trigger cell; separator spans include the column. The static units enhancer rewrites it to the same text. Page samples: mixed triggers (Button, Distance, Position), all unknown (no column), a multisport sample | ADR-026: the no-JS page and the edit page agree cell for cell through the samples |
| LS-3 | **No device laps** (replaces F-18 L-8). Distance advances (F-18 L-6 condition), no laps: the server renders the laps module with the `hidden` attribute, heading "Splits" (OQ-168), the empty heading slot and no table. "No laps recorded." is removed from template, `components/page` and both builders. Neither laps nor advancing distance: no module, as today. CSS must not override `hidden` on the module. The view-model shape is the engineer's; the `ride-no-laps` sample pins it | AC 33: JavaScript off shows nothing; one markup home, revealed by the chunk. The module is last before the footer, so revealing it moves only the footer |
| LS-4 | **Splits are lazy** (replaces the static part of F-18 L-4 to L-6). One chunk `src/splits/` holds the laps/splits switch, the splits table, the drop-down and the split rules (LS-5, LS-6). `src/page` imports it once, after the charts' first paint, when the distance advances (the F-42 MS-9 trigger), on the viewing and the edit page. It fills the heading slot (switch on a page with laps, drop-down in the splits view only) and, on a page without laps, inserts the splits table and removes `hidden`. Until it arrives, without JavaScript or if it fails: laps only, no switch; a no-laps page shows no module | Feature scope (drop-down not in the entry); moving the existing splits code out pays for the trigger column |
| LS-5 | **Pure split rules in `lib/splits`** (imports nothing but `lib/range`'s boundary function; tested in Node): the preset list per `meta.sport` (`running`, `rowing`, `cycling`; `swimming` with a pool length in `summary` → pool list, without → open water; everything else, `multisport` included → 1 km list; OQ-167), the default, which presets are offered, and the split length in the stream. Offered: presets whose stream length ≤ the distance stream's advance (`d̄[N−1] − d̄[0]`), and always the default. Non-pool boundaries: F-18 L-4 unchanged with `L` = the preset in metres | Pure decision logic apart from the DOM; AC 34, 35 |
| LS-6 | **Pool rule** (AP-20, OQ-165). Shown length `P` = `summary.pool_length_m` or `_yd` in the pool's unit, taken as exactly 100/3 when it is 33. Lengths per split `n` = target ÷ `P`, rounded half away from zero, at least 1. Label and distance cell: the target, or `n·P` when that differs by more than 1 %; in the pool's unit, header included, whatever the units setting; the last partial split shows its whole lengths × `P`. Boundaries in the stream: every `n` recorded lengths, `n·k·pool_length_recorded_m`, with a tolerance of 1 % of a recorded length for quantisation and device rounding (checked on the corpus pool files). The summary's "Pool 33 m" is untouched | AC 36: 32.5, 33.0 and 33.33 m give 3 lengths; the label is never derived from the stream |
| LS-7 | **Choice state.** The chosen preset lives in the chunk's memory only: never `lib/prefs`, storage or the fragment (F-40 SR-1 unchanged). Each load starts on the default. A units switch re-formats with the same preset (boundaries do not depend on units). A change rebuilds only the splits table; the selection store is not touched, so the range stays in charts, map and panel; a marked split row is unmarked | AC 37, 38, 40 |
| LS-8 | **Taps through the static enhancer.** Split rows use F-18 L-3 and L-7's one tap path: the static laps enhancer offers one seam through which the chunk registers the current splits container and its row ranges (like F-42's `legs`). No second click listener, no second L-7 | One hot-path implementation |
| LS-9 | **Performance.** Building splits is O(N) over the existing range prefix sums, only on chunk arrival, a drop-down change, a switch to splits or a units change; never on a cursor or selection flush. Nothing new before the summary; the chunk loads after first paint | AC 41; ADR-008, F-17 R-11 |

**Module boundary** (F-18's table, changed):

| Module | Holds | May import |
| --- | --- | --- |
| `ingest` (Go) | FIT `lap_trigger` → code | unchanged |
| `lib/splits` (new) | LS-5, LS-6 | `lib/range` |
| `src/splits/` (new chunk) | switch, drop-down, splits table, reveal | `lib/splits`, `lib/range`, `lib/page` (`lapsGroup`), `lib/units`, `lib/format` |
| `src/page` laps enhancer | row taps, mark, L-7, the LS-8 seam, the `import()` | no longer `splitBounds` or the switch |

## Cost

| What | Decision |
| --- | --- |
| Static | Net change **≤ +100 bytes gzip**, expected negative (switch, splits table and boundaries leave the entry; the trigger map and the seam join). Measure the entry before and after and record it in ADR-011 item 4. Above +100: stop and report |
| Lazy | `src/splits/` with `lib/splits` and its CSS. Not counted |
| Data file, HTML | ADR-046 Consequences; one cell per lap row when a trigger is known |

## Hand-offs

| To | What |
| --- | --- |
| Backend | F-03 §5.2, §5.3, §9 are updated to version 4 (this round); `datafile.go`'s package comment names the section |
| Requirements | AP-20 names "cycling" only; OQ-167 settles the other bike sports |

## Open questions

| ID | Question | Suggestion |
| --- | --- | --- |
| OQ-167 | AP-20 lists by sport; `e_biking` is its own FIT sport | Cycling list for `e_biking` too; every other sport by AP-20's "every other sport" |
| OQ-168 | Heading of the module on a workout without device laps (spec silent) | "Splits" |
