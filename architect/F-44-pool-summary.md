# F-44 Pool length and lengths in the summary: architecture

- **Version:** 0.1
- **Status:** accepted (11 Nov 2026). Normal lane. OQ-149, OQ-150, OQ-153 decided by the user on 11 Nov 2026 and built in here
- **Feature:** [`features/F-44-pool-summary.md`](../features/F-44-pool-summary.md)
- **Requirements:** AP-16, AP-2, DS-1, DS-5, UP-3; NFR Performance
- **Builds on:** [ADR-044](ADR-044-device-and-pool-values-in-the-data-file.md) (format version 3, shared with F-43); [F-03](F-03-workout-data-file-writer.md) §5.2; F-08 R-10 (summary); ADR-026, ADR-027, ADR-011; F-41 tints
- **Owner:** both. Backend first, in one step with F-43's backend (one format bump); then the frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| PS-1 | **Pool swim** = a session whose `sub_sport` is `lap_swimming`. Nothing else counts, whatever its `pool_length` (the synthetic open-water triathlon swims keep their 25 m and are the negative case) | OQ-149 |
| PS-2 | **Read.** `ingest` reads the session's `pool_length` (metres, as the profile scales it), `pool_length_unit` and `num_active_lengths` into the workout's session, as recorded. `length` messages are not read. Zero or invalid → not recorded. A session the parser builds without a `session` message (truncated file, UP-3) has none | AP-16: device values only, never computed |
| PS-3 | **Rule, in `workout` beside the summary (F-08 R-10).** Per pool swim: the length in its unit (`statute` → yards, metres ÷ 0.9144; `metric`, other or missing → metres), rounded half away from zero to a whole number; 0 → not recorded. **Pool** is set when every pool swim has a length and all are equal (value and unit). **Lengths** is the sum when every pool swim recorded `num_active_lengths`; otherwise not set (see OQ-153). No pool swim → neither | OQ-150; one rule, computed once on the server; renderers never compute (ADR-026 item 3) |
| PS-4 | **Data file:** `summary` adds `pool_length_m` or `pool_length_yd` (at most one) and `lengths`, integers, after `moving_time` (F-03 §5.2, ADR-044). Sessions get no pool keys: nothing in scope reads them | Keeps the summary all numbers, so the existing builders and the units enhancer read it unchanged |
| PS-5 | **Cells.** Both summary builders (Go `internal/page`, TS `lib/page`) add key `pool` (label "Pool", unit "m" or "yd") and key `lengths` (label "Lengths", no unit), after `calories`, before `ascent`. Their text is the same in both unit systems, so the static units enhancer rewrites them to the same text (DS-1, ADR-027). Neither key is in F-41's tint list, so both stay untinted. `CardItems` (F-23) does not select them. Page samples: metres, yards, Pool only, no pool cells (open water); Go and TS reproduce them | Summary order of 10 Nov 2026; the cell-count check of the units enhancer needs both builders to agree |
| PS-6 | **Budget (ADR-011).** Static: two cells in `summaryItems` (used by the static units enhancer) and the reader's two new summary keys, ceiling **150 bytes gzip**. The engineer measures the entry before and after and records it in ADR-011 item 4; above 150: stop and report | Reserve 3,493 |
| PS-7 | **Corpus.** `pool-swim.fit` and the 25 yd file as the feature says. The two-session cases (same and different length, one session without lengths) may be synthetic files or page samples, the engineer's choice. Every golden data file and fixture changes with the version bump | Feature ACs 32 to 39 |
| PS-8 | **Hand-off to trim (UP-6, phase 2).** A trimmed workout drops both keys: lengths cannot be recomputed from the streams, and the trimmed part may hold lengths | AP-2 "totals recomputed after a trim"; a stale count would be wrong |

## Open questions

None. OQ-153 (hide "Lengths" when any pool swim lacks a lengths count, as PS-3) accepted by the user on 11 Nov 2026 (decision log, `features/README.md`).
