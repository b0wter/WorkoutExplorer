# ADR-048 Lengths and strokes in the data file (format version 5)

- **Status:** accepted
- **Date:** 13 Nov 2026
- **Features:** F-48 (pool strokes); F-19 (`draft.bin`), F-30 (regenerate), F-47 (trim)
- **Supersedes:** none. Bumps [ADR-005](ADR-005-workout-data-file-format-v1.md) to version 5 under its item 9; narrows [F-44](F-44-pool-summary.md) PS-4 ("the summary stays all numbers"); keeps [ADR-046](ADR-046-lap-trigger-and-recorded-pool-length-in-the-data-file.md) item 3
- **Requirements:** AP-13, AP-16, AP-8, UP-6, I-2; NFR Performance
- **Normative bytes:** [F-03 design](F-03-workout-data-file-writer.md#data-file-format-version-6-normative) §5.1 to §5.3, §5.8

## Context

AP-13 adds a Stroke column to the server-made laps table, a summary Stroke cell, and a browser-only Lengths view with stroke, time, pace, stroke count and SWOLF per length. Both page builders read only the data file (ADR-026), and the owner's trimmed view returns the public summary unchanged (F-47 TR-8). Pace per length needs the pool length of the length's own session, which the file does not hold (the summary has one shared length, F-44 PS-3).

Rejected:
- **Raw `swim_stroke` enum in the file:** FIT values in the browser (I-2); the IM variants would merge in two builders.
- **Display words in the file:** wording would become format (as ADR-046).
- **Summary stroke as a top-level key:** F-12's `trimmed` would need a third member, and the stroke cell would read outside `summary` while every other cell reads inside.
- **Pace or SWOLF computed in the browser from a per-session pool length:** the pool-length-else-recorded-speed fallback would live in TypeScript; a session key would make the browser repeat "which session holds this length".
- **SWOLF stored per length:** a sum of two shown values; storing it adds bytes for no rule worth centralising.
- **Lengths with `start`, `end` offsets as laps have:** nothing draws a length at an offset; selection uses indices. About 25 bytes per length saved.

## Decision

1. **Format version 5**, one bump for three additions. Old workouts are rebuilt by `regenerate`; losses accepted (decision log 13 Nov 2026).
2. **Stroke codes:** `freestyle`, `backstroke`, `breaststroke`, `butterfly`, `drill`, `mixed`, `im` (FIT `im`, `im_by_round`, `rimo`), or `""` for missing, invalid or unlisted. `ingest` maps; the builders print the AP-13 word or an empty cell.
3. **Lap `stroke`** on every lap, `""` outside pool swims (F-48 PL-2).
4. **Summary `stroke`**, a code, present only when the whole swim used one stroke other than `mixed` and `im` (F-48 PL-4). The summary's only string; readers type it apart from the numbers.
5. **Top-level `lengths`**, always present (may be `[]`), recorded order. Each: `start_index`, `end_index` (F-03 §5.4 from the length's offsets), `rest` (bool), `stroke` (code), and, each absent when not recorded or derivable, `time` (s), `avg_speed` (m/s, server-derived, F-48 PL-3), `strokes` (count). A rest length carries only `time`. Numbering and SWOLF are the browser's display.
6. **ADR-046 item 3 stays:** splits of a pool swim without `length` messages still need `pool_length_recorded_m`.

## Consequences

- Every workout gains `"lengths":[]` and about 12 bytes per lap. A pool swim gains about 110 bytes of JSON per length: 200 lengths (5 km in 25 m) about 22 KB before brotli. ADR-005's 50 KB metadata trigger is reached near 450 lengths; at the `MaxLengths` cap (10,000) a crafted file's metadata is about 1.1 MB, as the laps cap already allows.
- Golden data files, fixtures and page samples are regenerated; the deploy runs `regenerate`.
- `summary_json` in the database carries `stroke` unused.

## Reversibility

Cheap: every data file is rebuilt from its original on every deploy (ADR-005). A new stroke code is a mapping plus a bump.

## Would choose differently if

- Real pool files exceed the 50 KB metadata trigger (ADR-005): then lengths move to their own column group (index, stroke code, time, strokes as columns).
- A later feature charts or selects by stroke: then a per-sample stroke stream beats looking up lengths.
