# ADR-005 Workout data file format, version 1 (`WEXD`)

- **Status:** accepted
- **Date:** 10 Oct 2026; revision 1 of 22 Oct 2026 (item 9, kept workouts); note of 26 Oct 2026 (item 9, version 2 by ADR-029); note of 11 Nov 2026 (items 7 and 9, version 3 by ADR-044); note of 12 Nov 2026 (item 9, version 4 by ADR-046)
- **History:** [architect/history/ADR-005-workout-data-file-format-v1.md](history/ADR-005-workout-data-file-format-v1.md)
- **Features:** F-03 (writer, introduces it), F-04 (browser reader), F-09 (`draft.bin`), F-15 (public data file), F-16 (hiding changes columns only), F-17 (version 2, ADR-029), F-30 (regenerate)
- **Supersedes:** none
- **Requirements:** AP-4, NFR Performance (data file under 500 KB compressed for a 4-hour ride; 60 fps on 12 hours), SH-4 (via F-16); TD Workout data file (Layout, stream table, Size, Versioning), TD Principles ("Fast for viewers")
- **Normative specification:** [F-03 design, section "Data file format, version 4"](F-03-workout-data-file-writer.md#data-file-format-version-4-normative). This ADR records the decisions and why they were made. The byte-level rules are in that section.

## Context

The TD fixes the outline: magic `WEXD`, a uint16 version, a uint32 sample count, length-prefixed JSON metadata, one little-endian column per present stream on a 4-byte boundary, a type, resolution and encoding per stream, and the type's maximum as the missing-sample sentinel. It leaves several things open. An independent TypeScript decoder (F-04) needs every one of them settled:

1. How a delta-coded column represents a missing sample, and what the next delta is relative to.
2. Whether values are quantised before or after taking differences (rounding drift).
3. What happens to values outside a type's range (a 300 spm cadence, garbage altitude).
4. Whether column offsets are absolute or relative, and how padding works.
5. What the metadata contains, and with which keys and number formats.
6. How laps and sessions point into the streams.

Two constraints from later features shape the answers. F-16 must be able to prove that no hidden coordinate reaches `public/`. TD Versioning says only one format version is ever live: every deploy regenerates every workout from its original. (rev. 1) Except workouts kept after a failed rebuild (item 9).

## Decision

1. **Header exactly as in the TD, plus a metadata length.** Bytes 0–3 `WEXD`, 4–5 version (uint16), 6–9 sample count (uint32), 10–13 metadata length (uint32), then the JSON. The header fields are not aligned, and that is acceptable: readers read them with `DataView`. Only columns need alignment, because only columns become typed-array views. A reserved field to align the header would add bytes and deviate from the TD for no benefit.
2. **Columns at absolute byte offsets.** Each descriptor's `byte_offset` counts from the start of the file and is a multiple of 4. The writer places columns in a fixed order, back to back, with zero padding up to the next multiple of 4. Readers use `byte_offset` and never compute positions themselves. The file ends exactly at the end of the last column. Readers reject trailing bytes, which catches truncated or concatenated files (F-03 v1.1). `fetch().arrayBuffer()` starts at byte 0, so `new Int32Array(buf, byte_offset, n)` works without a copy.
3. **Quantise the absolute value first, then take differences.** `q[i] = clamp(round(v[i] × factor))`. A delta column stores `q[i] − q[previous valid]`. The decoded value is exactly `q[i]`, so the error is at most half a step at every index and does not grow along the stream. Rounding is half away from zero (Go `math.Round`).
4. **Missing samples in delta columns.** The sentinel `2147483647` (int32 maximum, as in the TD) marks a missing sample. The running sum skips it. The next valid sample's delta is relative to the last valid value, or to 0 if there is none yet, so the first valid value is stored as itself. Every absolute value is clamped to ±1,000,000,000, so a delta can never reach the sentinel or overflow int32, and the running sum never leaves int32 range.
5. **Clamping instead of errors for out-of-range values.** Every quantised value is clamped to the type's range without its sentinel (for example cadence 0–254, temperature −128–126). A valid value therefore never reads as missing, and one implausible device value cannot fail a whole upload. Physically real values are inside every range (speed up to 65.534 m/s, power up to 65,534 W).
6. **Presence.** A stream with no valid value is not written: no column and no descriptor. Latitude and longitude are written as a pair, so a sample with only one of them valid is missing in both. Consequence for F-16: hiding replaces positions with "missing" in the normalised workout and calls the same encoder. A fully hidden route then has no position columns, which is what F-16 AC 5 expects.
7. **The metadata is a closed schema without coordinates.** It holds sport, start time, time zone, summary, sessions, laps and stream descriptors. It holds no positions, no bounding box, no device and no timer events. Coordinates exist only in the latitude and longitude columns. F-16 can therefore prove its AC 1 by inspecting two columns, and the public file reveals nothing that the page does not show. Fields are added when a feature needs them, with a version bump (item 9).
   - (11 Nov 2026, [ADR-044](ADR-044-device-and-pool-values-in-the-data-file.md)) The metadata holds the device's display name and sensor types (F-43), never an identifier, and the summary's pool values (F-44).
8. **Laps and sessions carry both time offsets and half-open sample index ranges** (`start_index`, `end_index`). The browser places markers by offset and selects ranges by index with no search. A lap that lies inside a gap has an empty range (`start_index == end_index`).
9. **Versioning: one live version, strict readers.** Version is `1`. Any change to the header, the metadata schema, a descriptor field, a stream's type, scale, encoding or unit, or the column rules bumps the version. The writer and the reader change in the same release. A reader rejects every version other than its own (F-04 AC 3). Regeneration on deploy (TD Versioning, F-30) rewrites every live file, so no reader for old versions is ever needed.
   - (26 Oct 2026, [ADR-029](ADR-029-cumulative-moving-time-and-gain-in-the-data-file.md)) **The live version is 2.** It adds the cumulative moving-time and gain streams (int32 delta, no missing samples); every other decision here is unchanged. The worst case becomes 35 bytes per sample (Consequences). Byte rules in F-03's normative section.
   - (11 Nov 2026, [ADR-044](ADR-044-device-and-pool-values-in-the-data-file.md)) **The live version is 3.** It adds `device` and the summary's pool keys to the metadata; streams and columns unchanged.
   - (12 Nov 2026, [ADR-046](ADR-046-lap-trigger-and-recorded-pool-length-in-the-data-file.md)) **The live version is 4.** It adds the lap `trigger` and the summary's `pool_length_recorded_m`; streams and columns unchanged.
   - (rev. 1, OQ-55, the user, 22 Oct 2026) **Kept workouts.** A workout whose rebuild fails for a server-side reason (F-30: `canceled`, a time limit, or an error that is not about the file) is kept unchanged: it serves its previous data file and pages, under their old content-hashed names, until a later run rebuilds it. Its pages refer to the previous content-hashed app bundle, and **an old bundle is deleted only once no kept workout refers to it**. So more than one format version and bundle can be live for a while, but each bundle reads exactly one version, and no reader for an old version is ever written. The deploy report counts kept workouts (F-09 v1.10, F-30 hand-off).
10. **The committed contract is a set of golden files.** For every synthetic corpus file, the repository holds the writer's `data.bin` and a JSON fixture of the decoded integer values (count, statistics and digest). Go's `TestDataFileCorpus` keeps both current. F-04's decoder tests read them. Fixtures exist for synthetic files only (ADR-004 item 7, rev. 3).

## Consequences

- The browser needs one O(n) prefix-sum pass per delta column (time, positions, altitude, distance). Raw columns are zero-copy views. The TD's "nothing to parse" holds for the raw columns; for delta columns it means a single pass with no parsing. F-04 already plans this pass.
- The file size has a hard upper bound: at most one sample per second (F-02 R-2), and at most 27 bytes per sample with every stream present. A 4-hour workout therefore has at most 388,827 bytes of columns, so the 500 KB budget holds even without compression, as long as the metadata stays under about 110 KB. Brotli adds the margin (ADR-006). (Version 2, ADR-029: 35 bytes per sample, about 504 KB of columns for 4 hours; the budget is then held by compression, measured in F-17.)
- Clamping can change an extreme value without any notice. The round-trip test over the corpus uses a strict tolerance, so a clamp in a corpus file shows up as a test failure and is investigated.
- Golden `.bin` files change whenever normalisation or the format changes. Each change adds a few MB to the git history (the 12-hour file alone is about 1.2 MB). This is accepted for a reviewable, cross-language contract.
- Because readers are strict and regeneration is assumed, a deploy that bumps the version must run `regenerate` before the old bundle is removed. TD Hosting and operations already orders the deploy steps this way.

## Reversibility

The format can be reversed cheaply: every data file is rebuilt from its original on every deploy, and the files live at most 30 days. The parts that are expensive to change are the golden fixtures and the F-04 decoder, which change together in one version bump. No one-way door.

## Would choose differently if

- Measured `.br` sizes for real 4-hour files approach 400 KB. In that case, consider byte-plane splitting or coarser altitude and distance resolution. Either change costs zero-copy views or precision, so it needs a new ADR.
- A future feature needs sub-second samples (for example running dynamics). Then the time column needs milliseconds, which is a version bump and a new scale.
- Metadata grows past about 50 KB in real files, for example through thousands of auto-laps. Then laps move to their own column group.
