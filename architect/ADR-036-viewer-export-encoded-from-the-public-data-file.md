# ADR-036 Viewer export encoded from the public data file, with the pinned library's stream encoder in `internal/fitexport`

- **Status:** accepted (2 Nov 2026), with the F-24 design v0.1
- **Date:** 2 Nov 2026
- **Features:** F-24 (introduces it), F-30 (reuses the build), F-31 (memory measure)
- **Supersedes:** none. Narrows [ADR-003](ADR-003-fit-decoding-library.md) item 2 ("only `ingest` imports the library") by one production package
- **Depends on:** [ADR-003](ADR-003-fit-decoding-library.md) (pinned `muktihari/fit` v0.28.4), [ADR-005](ADR-005-workout-data-file-format-v1.md) item 7 (no coordinates in the metadata), [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md)
- **Requirements:** SH-8, SH-4, UP-9, NFR Security; TD invariant I-3

## Context

Viewers may download a FIT file built from the published data (SH-8), never the original. The public build already holds three candidate inputs: the original's bytes, the parsed and transformed `workout.Workout`, and the decoded public data file. Writing FIT needs an encoder. The pinned library has one (`encoder`, with a message-by-message `StreamEncoder` that needs an `io.WriterAt` or `io.WriteSeeker`); the dev-only `corpusgen` and `ingest`'s tests already use it. ADR-003 lets only `ingest` import the library in production, and `ingest` is the ingestion context, not publishing.

## Decision

1. **Input is the decoded public data file only** (`datafile.File`), as for the preview (F-23 PC-1). Never the original, never the workout before or after the transform.
2. **Encoder: the pinned library's `StreamEncoder`**, not an own writer. Messages are written one at a time into an in-memory buffer; the code never holds all records as library values at once. Peak memory is about the export's size (about 30 bytes per sample, so about 15 MB at the 500,000-sample limit of ADR-017).
3. **New pure package `internal/fitexport`:** `datafile.File` → FIT bytes, no other input; no I/O, no clock. It and `ingest` are the only production packages that import the FIT library; no library type leaves either. `publish` is its only caller. The messages and fields it writes are a closed list in this package (F-24 DL-3).
4. **Upgrade check** (adds to ADR-003 item 1): the export of every synthetic corpus file still parses with the pinned decoder with no integrity notice; a changed export byte stream is explained in the upgrade hand-off.

## Consequences

- AC 3 and AC 4 of F-24 hold by construction: the data file has no coordinates outside its position columns, no device, serial number or events (F-03 §5.6), so the export cannot have them either.
- The export inherits the data file's quantisation (10⁻⁶°, 0.1 m; altitude to FIT's 0.2 m). Accepted: it is a copy of what the page shows.
- Encoder and decoder come from one library, so a symmetric bug would pass the round trip; the manual third-party-tool check (F-24 AC 2) is the guard.
- **Cost:** a second production importer of the FIT library; a library swap now touches two packages.

## Reversibility

Cheap. The export is rebuilt on every save and deploy (ADR-028, F-30); swapping the encoder changes one package and the export names.

## Would choose differently if

- The library's encoder changes its output in a way third-party tools reject, or a library swap leaves no encoder: write an own minimal writer for DL-3's five messages behind the same `fitexport` function (`ingest` already frames FIT with `kit/hash/crc16`).
- A later feature needs a field the data file does not carry (for example device info, AP-14): add it to the data file's metadata first (a format change under ADR-005), never a second input to `fitexport`.
