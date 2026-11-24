# F-68 Extra sensor streams: architecture

- **Version:** 0.1
- **Status:** accepted (24 Nov 2026). Critical lane (UP-9, SH-4, data file format), QA high. Product decisions OQ-326 to OQ-332 taken by the user (feature file). Open questions F68-Q1 to F68-Q3 below: none blocks the build; F68-Q1 blocks the release
- **Feature:** [`features/F-68-extra-sensor-streams.md`](../features/F-68-extra-sensor-streams.md)
- **Requirements:** AP-4; UP-9, SH-4, UP-6, UP-7, AC-4; AP-2, AP-6, AP-7, AP-21, DS-1, DS-4, DS-6; NFR Performance, Privacy, Reliability
- **Builds on:** [F-02](F-02-fit-parsing-and-normalisation.md) and [F-08](F-08-fit-validation-and-recovery.md) R-13 (transitions); [F-03](F-03-workout-data-file-writer.md) §2, §9 (format); [ADR-003](ADR-003-fit-decoding-library.md) (`ingest` as the only FIT reader); [ADR-005](ADR-005-workout-data-file-format-v1.md) item 9; [ADR-011](ADR-011-activity-page-bundle-allocation.md); [ADR-017](ADR-017-parse-isolation-and-limits.md); [ADR-023](ADR-023-identifying-data-stripped-by-a-byte-level-rewriter.md) (strip unchanged); F-54 text groups
- **Owner:** both, backend first (B-1: `ingest`, `workout`, `datafile` version 8, corpus), then frontend (FE-1: reader, lazy chunk, charts, readout, range panel). No schema, setting, template, Caddy or owner API change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| XS-1 | **Four standard streams from `record`**, in this order: vertical oscillation (`vertical_oscillation`), ground contact time (`stance_time`), stride length (`step_length`; Garmin labels this field "stride length"), vertical ratio (`vertical_ratio`). Read by `ingest` with the profile's sentinel rule (F-02 Conversions). **A raw 0 is missing** for all four (devices write 0 while standing; none is 0 while running), so charts, readout and range panel need no zero rule | OQ-326; one rule in one place |
| XS-2 | **Data file version 8** adds four columns after `elevation_gain` (table below) and nothing else. One version live (ADR-005 item 9): writer, F-04 reader and golden fixtures change together; F-03 §1, §2, §9 take the rows on build. **No migration**: the deploy's `regenerate` rebuilds every workout from its original, which still holds the fields (TD Versioning); a workout kept after a failed rebuild shows none until rebuilt (feature Scope "Older workouts") | OQ-332: in `data.bin` while the budget holds |
| XS-3 | **Developer fields: one recognised field, decided inside `ingest`.** `ingest` tracks `developer_data_id` and `field_description` per FIT sequence and reads developer values **only from `record` messages**. A (developer data index, field number) pair is **Stryd power** when its current description has all of: `field_name` first value exactly `Power`, `units` first value exactly `Watts` (F68-Q1), a numeric scalar base type (integer or float, no array, no string, no byte), scale absent or 1 and offset absent or 0, and, where `native_mesg_num`/`native_field_num` are set, exactly `record`/`power`. A value is valid when it is not the base type's invalid value and finite; it is then treated as native power (F-03 W-4 clamp). Anything else (other names, wrong type, array, scale, unknown description, a value with no current description) is ignored silently, never an error or log line | OQ-327; stricter than name and units alone, never looser |
| XS-4 | **Redefinition and duplicates.** A new `field_description` for a pair replaces the old one from that message on; a new `developer_data_id` for an index forgets that index's descriptions. Per sequence, the **first pair in file order** that qualifies is the source; a second qualifying pair (two apps) is ignored, also where the first has no value. State resets at each sequence start (chained files) | Feature Constraints "Reliability"; deterministic |
| XS-5 | **Native power wins per file.** After the decode, `ingest` fills the power stream from Stryd power only if no record in the whole file has a valid native power value; otherwise the Stryd values are dropped. Decided on the full upload, before trim, so trim or hiding never switch the source. `workout` and everything after it see one power stream and never know where it came from | OQ-328; F68-Q3 for bricks |
| XS-6 | **Stryd power is the power stream everywhere** (F68-Q2): chart, readout, range panel, power curve, zones, laps power column, route colouring by power, and the summary's average and max power, computed on the server where the session has no native power (AP-2's existing fallback, `workout/summary.go`) | AC 2 asks for curve and zones; a second set of power rules would cost code |
| XS-7 | **Nothing named leaves `ingest`.** No field name, unit string, developer or application ID, manufacturer ID or developer value other than XS-3's enters `workout.Recording` (only `float64` samples) or any log line; the developer state lives only during the decode. The stored original keeps every developer field as uploaded (UP-9 and the strip unchanged, OQ-331) | UP-9, SH-4; the anti-corruption layer is the privacy boundary |
| XS-8 | **Streams in `workout`** like cadence: resampled (R-3), cut by trim (UP-6), **dropped at transition samples** (R-13: rule `drop` in `transitionStreams`; its completeness test forces the entry), kept in hidden sections (SH-4 hides positions only; none of the four carries a position, and speed, from which they follow, is already published there). No lap, session or summary value (OQ-329); device session averages of the four are not read | AP-4, OQ-329 |
| XS-9 | **Front end: one lazy chunk, one spec list.** `src/dynamics/` holds the four chart specs (label, unit, display conversion, precision, colour tokens for both themes) and their texts (F-54: own text group). The entry imports it only when the data file's descriptors name one of the four streams, started from the streamed metadata prefix (F-54) so it loads alongside the columns. Charts, both readouts (AP-6, AP-21) and the range panel are fed from **one list**, base specs followed by the chunk's specs; the range panel averages any spec in that list by present samples (no per-stream code in the entry). The edit page gets the same through `startPage`; owner chunk 0 bytes | ADR-011: reserve 1,037 at entry 60,963 |
| XS-10 | **Display.** Order below temperature as XS-1, one chart each, same height. Metric: cm (1 decimal), ms (0), m (2), % (1). Imperial (DS-1): vertical oscillation in in (1), stride length in ft (2); ground contact time and vertical ratio unchanged | DS-1 "where one applies" |
| XS-11 | **Memory.** `RawRecord` grows by five values (four streams and the Stryd candidate). The engineer re-measures ADR-017's per-record term on the 12-hour and the 500,000-sample limit files; if M(N) no longer holds, stop and return to the architect | ADR-017 stop rule |

**Data file version 8, rows 13 to 16** (F-03 §2 columns; sentinel 65535, valid range 0 … 65,534, half away from zero, clamped):

| # | `name` | `type` | `encoding` | `scale` | `unit` | Resolution |
| --- | --- | --- | --- | --- | --- | --- |
| 13 | `vertical_oscillation` | `uint16` | `raw` | `0.1` | `cm` | 1 mm |
| 14 | `ground_contact_time` | `uint16` | `raw` | `1` | `ms` | 1 ms |
| 15 | `stride_length` | `uint16` | `raw` | `0.001` | `m` | 1 mm |
| 16 | `vertical_ratio` | `uint16` | `raw` | `0.1` | `%` | 0.1 % |

All four scales are existing §5.5 literals. With every stream a sample takes 43 bytes (was 35).

## What is dropped

Every other `record` field (`stance_time_percent`, `stance_time_balance` and the rest); every developer field but XS-3's, including Stryd's own running dynamics; every developer value in non-`record` messages (app session and lap totals); Stryd power whenever native power exists anywhere in the file; all developer names, units and IDs. All of it stays only in the private original (UP-9).

## Cost

| What | Decision |
| --- | --- |
| Static (entry) | Reader rows, the conditional `import()`, the one-list seam. **Target 150, ceiling 250 bytes gzip.** The reserve then falls below ADR-011's review trigger (about 790 to 890): this design is that review; F-68 needs no further remedy, and the next feature adding static code applies a decision 5 remedy before it is built |
| Lazy | `src/dynamics/` and its text group; report gzip sizes. Owner chunk 0 |
| Data file | **Stop rule:** a 4-hour run with all four streams and power above 500,000 bytes `.br`, or LCP worse beyond noise on the corpus: stop and return to the architect (OQ-332's separate file) |
| Scrubbing | ADR-016 gate on a 12-hour synthetic run with every stream (ten charts) |

## Verification for QA (critical, effort high)

1. **Non-allowlisted fields publish nothing** (AC 3). Synthetic files from `corpusgen` only: a text field; a numeric field named like a place; a latitude/longitude pair (sint32 semicircles and float degrees); a field named `Power` in `Watts` that fails one XS-3 condition each (string, array, scale 10, offset, `native_field_num` ≠ power, in a `session` message only). Sweep the public `data.bin` (raw and decoded), `data.bin.br`, `index.html`, preview PNG, `pins.json`, `draft.bin` and the server log for every name, unit string, application ID and injected value series; the decoded stream list is exactly the expected one, and no column equals an injected series.
2. **Source rules:** Stryd only → power from it, summary average computed; native and Stryd → native values exactly, Stryd absent; two qualifying apps → the first; redefinition mid-file; chained file with Stryd in the second sequence; truncated file; field described after its first record.
3. **Trim, transitions, hiding:** new columns hold the sentinel at every transition sample and are cut by trim; full hiding leaves them as recorded.
4. **UP-9:** the strip corpus check passes unchanged; the owner download keeps every developer field byte for byte as today.
5. **Fuzz** seeds include the files of item 1 (ADR-004: from `synthetic/` only). No real file, and no figure derived from one, is committed.

## Hand-offs

| To | What |
| --- | --- |
| Requirements | TD "FIT parsing" (`record` row, "Developer fields" row → XS-1, XS-3) and "Workout data file" table (rows 13 to 16); glossary: stride length = FIT `step_length`, ground contact time = FIT `stance_time` |
| Backend, on build | F-03 §1, §2, §9 and the ADR-005 version line: version 8 with the rows above; ADR-017 note if XS-11 changes a number |
| Frontend, on build | ADR-011 ledger row with the measured entry delta |
| Release notes | Version 8: `regenerate` runs on deploy; workouts that fail to rebuild show no extra streams |

## Open questions

| ID | Question | Suggestion |
| --- | --- | --- |
| F68-Q1 | The exact strings Stryd writes (`Power`, `Watts`) are not documented publicly; forum reports show other apps writing `Power(Watts)`-like names and Stryd variants. Blocks release, not build | Build with `Power`/`Watts`; before release the operator runs `parse` on one real Stryd file locally and reports only the two strings; a difference is a small-lane change |
| F68-Q2 | Stryd power also feeds the summary (computed average and max), laps, route colouring, zones and power curve (XS-6), while the feature file says the new values stay out of the summary | Yes: Stryd power is the power stream; "not in the summary" applies to the four running dynamics |
| F68-Q3 | Native wins per file (XS-5): a brick with a bike power meter and Stryd on the run shows no run power | Keep per file as decided; per-leg filling would mix two power sources in one chart |
