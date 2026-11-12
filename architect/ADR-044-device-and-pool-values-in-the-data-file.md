# ADR-044 Device name, sensor types and pool values in the data file metadata (format version 3)

- **Status:** accepted
- **Date:** 11 Nov 2026
- **Features:** F-43 (device line), F-44 (pool cells); F-19 (`draft.bin`), F-30 (regenerate)
- **Supersedes:** none. Bumps [ADR-005](ADR-005-workout-data-file-format-v1.md) to version 3 under its item 9 and narrows its item 7 ("no device")
- **Requirements:** AP-14, AP-16, AP-2, UP-9, INF-1; NFR Privacy
- **Normative bytes:** [F-03 design](F-03-workout-data-file-writer.md#data-file-format-version-4-normative) §5.2, §5.7
- **Note (12 Nov 2026):** item 4 narrowed by [ADR-046](ADR-046-lap-trigger-and-recorded-pool-length-in-the-data-file.md) item 3 (`pool_length_recorded_m`, not whole, never printed)

## Context

The device line (F-43) and the pool cells (F-44) appear on the server-rendered viewing page and on the edit page, which the browser builds from `draft.bin` (F-19). Both page builders read only the data file (ADR-026), so the values must be in its metadata. ADR-005 item 7 keeps the device out of the public file, and the public file must reveal nothing the page does not show.

Rejected:
- **Device in the owner API or the database row:** the edit page would then have a second source beside the data file, and the viewing page's builder would need the row; one source is simpler.
- **Raw FIT values (manufacturer number, product number, `device_type` per sensor):** the browser would need the FIT profile, against I-2, and the public file would carry more than the page shows.
- **Pool as its own object with a unit string:** the summary builders and the units enhancer read `summary` as numbers; a unit in the key name keeps it all numbers.

## Decision

1. **One version bump, 3, for both features**, built in one backend step. If one feature slips, the other bumps alone and the later one bumps again; a bump costs only regenerated goldens (ADR-005 item 9).
2. **`device` (top level, always present):** `name`, the display name composed on the server (at most 40 code points from the recorded product name plus a manufacturer name, F-43 DI-2 to DI-4), and `sensors`, fixed codes in a fixed order. This is the only text in the data file that comes from the uploaded file. It is sanitised and printed as text only.
3. **Nothing else about devices** reaches the metadata: no numbers, serials, ANT+ numbers, descriptors, versions or battery values. `ingest` never reads them (F-43 DI-1).
4. **Pool values in `summary`** as whole numbers: `pool_length_m` or `pool_length_yd`, and `lengths`, computed by `workout` (F-44 PS-3). Renderers print them and never convert them (DS-1).

## Consequences

- The public file carries a device name the uploader cannot remove. The user accepted this, and the risk of app-written text in it, on 11 Nov 2026 (OQ-146). An owner setting to hide it is out of scope (F-43).
- Every golden data file and fixture is regenerated; the deploy runs `regenerate` as for every bump.
- The metadata grows by under 150 bytes.

## Reversibility

Cheap: the format is rebuilt from originals on every deploy (ADR-005). Removing the device again is one more bump.

## Would choose differently if

- An owner setting to hide the device is wanted: then `publish` writes `device` empty into the public file only, the draft keeps it. Still no format change.
- A later feature needs per-session pool values (lengths table, AP-13): then sessions get pool keys in that feature's bump.
