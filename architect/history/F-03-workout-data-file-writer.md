# F-03 Workout data file writer: history

History of [architect/F-03-workout-data-file-writer.md](../F-03-workout-data-file-writer.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written

- **Date:** 11 Oct 2026 (v1.4: 22 Oct 2026)

## Change log

| Version | Change |
| --- | --- |
| 1.4 | Text only, no format change (still version 1). §9 states the OQ-55 exception, decided by the user on 22 Oct 2026: a workout kept after a failed rebuild stays on its previous data file and bundle until a later run rebuilds it; each bundle still reads exactly one version (ADR-005 rev. 1, F-09 v1.10) |
| 1.3 | F-04 design. No format change (still version 1). §6: rule 4 now says what readers do with nested keys, which the rules did not cover (gap found while designing F-04; Go `Decode` zero-fills missing or `null` nested keys, F-04 rejects them); the known verdict differences between Go and F-04 are listed; rule 7 is implemented by F-04 as well. `.gitignore`: data-file patterns case-insensitive (F-03 QA r2 note, ADR-004 rev. 5) |
| 1.2 | QA round 1 ([qa/F-03-workout-data-file-writer-r1.md](../../qa/F-03-workout-data-file-writer-r1.md)). r1-2: W-1 rejects a lap or session with `end < start` (error, no clamping). r1-1: §6 rule 6 bounds are overflow-safe and `byte_offset` must be ≤ the file length. Gap 2: §6 rule 5 enforces the §2 column order and the latitude/longitude pair; rule 7 adds per-index pairing for Go. Gap 1: top-level fixture keys listed. Q1: the `ride-12h` golden file stays (reason and trigger in Golden fixtures). Q2: `.gitignore` ignores `*.bin` and `*.br` everywhere except the golden folder (ADR-004 rev. 4). Q3: `precompress.Brotli` runs one compression at a time per process |
| 1.1 | Engineer report after implementation. (1) Brotli quality 11 → **9**; the time trigger is replaced (ADR-006 rev. 1). (2) §5.5: the writer emits exactly the §2 spellings (`1e-06` for latitude and longitude, as the implementation and the golden files already do), and readers compare numerically. (3) §4 and §6: trailing bytes after the last column are an error. (4) The fixture JSON uses `MarshalIndent` output with one key per line, and the example only shows the keys. (5) `ride-4h.fit` may share a start date with another file. |
| 1.0 | First version. |

## Closed open question

| ID | Question | Blocks | Owner |
| --- | --- | --- | --- |
| ~~OQ-8~~ | **Closed 11 Oct 2026:** F-04 QA r1 confirmed the TypeScript decoder reproduces all 18 golden fixtures, every field (F-04 design v1.1). F-03 AC 6 is complete | — | — |
