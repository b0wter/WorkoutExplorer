# ADR-060 Added lap boundaries: stored as instants, merged into the data file's laps (format version 7), rebuilt in place

- **Status:** accepted
- **Date:** 22 Nov 2026
- **Features:** F-65 (introduces it); F-12 (settings object), F-15 (P-7), F-30 (regenerate reads, never writes), F-21 (deleted with the row)
- **Supersedes:** none. Bumps [ADR-005](ADR-005-workout-data-file-format-v1.md) to version 7 under its item 9, adding one code to [ADR-046](ADR-046-lap-trigger-and-recorded-pool-length-in-the-data-file.md) item 2's set
- **Depends on:** [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md), [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md), [ADR-047](ADR-047-trim-cut-of-the-normalised-workout.md), [ADR-057](ADR-057-map-pins-stored-as-instants-published-as-sample-indices.md) (pattern)
- **Requirements:** AP-8, SH-4, UP-6, UP-7, AC-4; invariant I-3

## Context

The owner adds lap boundaries after upload. Viewers must see the resulting laps exactly like device laps: in the server-rendered table without JavaScript, on the map, in the chart and in the per-leg tables. Boundaries are allowed in hidden parts and removed by a trim. Pins solved a similar problem with a separate public file and remove-first rebuilds (ADR-057), and someone will ask why laps differ.

## Decision

1. **Storage as ADR-057 item 1 and 2:** one nullable column `added_lap_boundaries` (schema v11), holding the whole set as ascending Unix seconds of resolved samples. No coordinates.
2. **Merged into the data file, not a separate file.** The public build splits laps in `workout` before trim and hiding. The merged laps become the data file's `laps`, with the new trigger code `added`, so the format goes to version 7. Every existing laps renderer (Go page, entry enhancers, splits chunk, map) works unchanged. A separate file would need a second laps renderer on each side, and the no-JavaScript table would lack the added laps.
3. **On F-15 P-7's allow-list (in-place rebuild), unlike pins.** A boundary carries no text and no position. Every lap value it creates is computed from the public data file's streams, or is a device lap value that was already public. So an old public copy reveals nothing the new one hides. Pins stay off the list because a note text or a moved place can.
4. **The edit page mirrors the merge in the browser** over `draft.bin`, which keeps device laps only, so changes show before Save. Go stays the judge for the public files.

## Consequences

- Each boundary edit rewrites and recompresses the data file. Viewers keep the old page until `index.html` swaps; there is no generic-page second.
- Two implementations of the split (Go, TypeScript), pinned by shared samples. The edit page may differ from the public page in a last shown digit (quantised `draft.bin`).
- After a parser change a stored boundary may no longer be valid. The build skips it, and the owner's next Save drops it.

## Would choose differently if

- Boundaries gain text (names, types): then the item 3 argument fails, so they come off the allow-list.
- Laps must be edited without a Save of the whole set: then a child table (ADR-057's own condition).
