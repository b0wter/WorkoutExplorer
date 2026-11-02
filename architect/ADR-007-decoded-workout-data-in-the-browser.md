# ADR-007 Decoded workout data in the browser

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 1 of the same day
- **History:** [architect/history/ADR-007-decoded-workout-data-in-the-browser.md](history/ADR-007-decoded-workout-data-in-the-browser.md)
- **Features:** F-04 (introduces it); every consumer of the decoded data: F-05 (charts), F-06 and F-25 (map), F-07, F-13, F-17 (range statistics), F-18 (laps and splits), F-19
- **Supersedes:** none
- **Depends on:** [ADR-005](ADR-005-workout-data-file-format-v1.md) (format v1), [F-03 design](F-03-workout-data-file-writer.md) §1–§7
- **Requirements:** AP-3, AP-4, NFR Reliability ("a broken or unusual FIT file never breaks the page"), NFR Performance (60 fps on 12 hours); TD Front end, TD Workout data file ("typed arrays without parsing")

## Context

The F-04 decoder turns `data.bin` into what every viewer component reads. Its output shape is a contract with at least eight later features, so it is expensive to change once F-05 and F-06 build on it. Three questions need one answer each:

1. **How is a missing sample represented?** The file stores the column type's maximum (255 for heart rate). If consumers see that integer, one forgotten check draws a 255 bpm spike or a point at 2147.483647°. The feature asks that sentinels become values that are "treated as missing, never as numbers".
2. **Zero-copy views or converted arrays?** The feature's goal says "without parsing loops or copies of raw columns". Raw columns can be zero-copy `Uint8Array`/`Uint16Array`/`Int8Array` views, but integer arrays cannot hold a missing marker other than the sentinel, and they hold stored integers, not values in the unit (speed is stored in mm/s). Delta columns need a new array anyway (prefix sum).
3. **How does a broken file reach the page?** NFR Reliability: a broken file never breaks the page. F-04 AC 3: a clear error state, not an exception.

## Decision

1. **Every decoded stream is a `Float64Array` of values in the §2 unit, with `NaN` for a missing sample.** The same for delta and raw columns, and for time (never `NaN`). Raw columns are read through typed-array views created directly on the file's `ArrayBuffer` (no `DataView` loop, no byte parsing) and converted in one pass: `v === sentinel ? NaN : v × scale`. No consumer ever sees a sentinel or a stored integer.
2. **Delta prefix sums accumulate in a JavaScript number, never in an `Int32Array`.** Go's `Decode` accumulates in `int64`; a crafted file whose sum leaves the int32 range must decode to the same value in both. Decoded value = `acc × scale` with the §2 scale literal (multiply, never divide by the factor), so the bits equal Go's `Stream.Values()`.
3. **Latitude and longitude are one optional `position` object**, never two independent optional streams. The type states the pairing invariant (F-03 §2): a consumer checks `data.position` once.
4. **Errors are values.** `decodeDataFile(buffer)` returns `{ ok: true, data } | { ok: false, error }` and never throws. `error.code` is one of a small closed set for the page; `error.rule` names the F-03 §6 rule for tests and diagnostics. A top-level `catch` maps any unexpected exception to code `internal`; tests assert that no input ever produces `internal`, so the safety net cannot hide a bug.
5. **The decoder is synchronous and runs on the main thread.** The output does not keep the file's `ArrayBuffer` alive.

## Consequences

- One representation for every consumer: chart decimation, the map line, readouts and prefix sums all skip `NaN` the same way. A forgotten check produces `NaN`, which fails every comparison and poisons sums, so the bug shows as a break or a blank readout instead of a plausible wrong number.
- **Cost: raw columns are copied once.** For the 12-hour file (43,200 samples, 10 streams) the output is about 3.5 MB of `Float64Array` instead of about 1.2 MB of views, and the conversion is one linear pass per column. The feature goal was reworded by requirements to "copied at most once" (OQ-9, closed). F-04 AC 5 measures the actual cost on the phone.
- `Float32Array` is not an option: latitude needs about 9 significant digits for 10⁻⁶°, and float32 has about 7 (F-04 AC 2 would fail).
- Page code needs no `try`/`catch` around decoding, and AC 3 is testable without a DOM.

## Would choose differently if

- (rev. 1) The F-04 AC 5 measurement shows a **cold** decode above **50 ms** on the reference phone, as the **median of 5 page loads** (one decode per load, the first in that page), or memory pressure appears on that phone in F-07. The page decodes once per load, so the cold run is the long task a user sees; warm runs are recorded for information only and never trigger anything. Then first move decoding into a Web Worker with transferable buffers (same output type), and only then consider lazy per-stream conversion. Either is a change inside `lib/datafile` that keeps this output type.
- A later format adds sub-second time or many more streams, so memory per workout grows several times. Then revisit decision 1 in a superseding ADR.

## Reversibility

Cheap while only F-04 exists; after F-05, F-06 and F-17 consume `WorkoutData`, changing decision 1 touches every viewer feature. Treat the output type as a published interface from F-05 onwards.
