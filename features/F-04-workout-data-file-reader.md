# F-04 Workout data file reader

- **Phase:** 0 Prototype
- **Owner:** frontend
- **Requirements:** AP-4, NFR Performance (60 fps, data size), NFR Reliability; Technical design: Workout data file, Front end
- **Depends on:** F-03

## Goal
Decode `data.bin` in the browser into typed arrays and metadata, ready for charts, map and range statistics, without parsing: raw columns are read through typed-array views and copied at most once, and missing samples become `NaN`.

## Scope
- Validate magic and version; read the metadata JSON; create typed-array views directly on the `ArrayBuffer` for raw columns.
- Prefix-sum delta columns once after load; apply scale from each descriptor.
- Turn sentinels into missing samples (`NaN`): values the charts and map treat as missing, never as numbers.
- Report time gaps (two consecutive samples 5 s or more apart) as sample indices; the values themselves are not changed.
- Expose present streams only; absent streams are simply missing from the result.
- Only the current format version is read (every deploy regenerates all workouts, technical design: Versioning).
- Unit tests using files produced by the F-03 writer.

## Out of scope
- Rendering (F-05, F-06); unit conversion (F-14); range statistics (F-17).

## Acceptance criteria
- [ ] Tests cover: 4-byte alignment, delta decoding, sentinels for each type (e.g. 255 in a uint8 heart-rate column becomes a missing sample, `NaN`), absent streams, scale application.
- [ ] Decoded values match the F-03 round-trip fixture for every corpus file within each stream's resolution.
- [ ] A file with a wrong magic or an unknown version produces a clear error state, not an exception that breaks the page.
- [ ] An indoor file decodes with no position stream and no error.
- [ ] Decoding the 12-hour corpus file on a mid-range phone is measured and the time recorded.
