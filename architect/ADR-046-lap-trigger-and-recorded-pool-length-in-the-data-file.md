# ADR-046 Lap trigger and recorded pool length in the data file (format version 4)

- **Status:** accepted
- **Date:** 12 Nov 2026
- **Features:** F-46 (laps and splits); F-19 (`draft.bin`), F-30 (regenerate)
- **Supersedes:** none. Bumps [ADR-005](ADR-005-workout-data-file-format-v1.md) to version 4 under its item 9; narrows [ADR-044](ADR-044-device-and-pool-values-in-the-data-file.md) item 4 (one pool key that is not whole and is never printed)
- **Requirements:** AP-8, AP-20, AP-16, UP-9, I-2; NFR Privacy
- **Normative bytes:** [F-03 design](F-03-workout-data-file-writer.md#data-file-format-version-4-normative) §5.2, §5.3

## Context

AP-8 shows a trigger word per lap on the server-made page and on the edit page, which the browser builds from `draft.bin`; both builders read only the data file (ADR-026). AP-8 merges the four position triggers "so the table does not hint at saved places", and the public data file is as public as the page.

AP-20 splits a pool swim every *n* whole lengths. The distance stream advances by the device's recorded length per length (32.5, 33.0 or 33.33 m for a "33 m" pool), so the browser must know that recorded length to find *n* lengths in the stream. The summary holds only the rounded length (ADR-044 item 4): with 33 instead of 32.5 the boundary drifts by half a metre per length and lands a length late after about 30 lengths.

Rejected:
- **Raw `lap_trigger` enum in the file, mapped in the browser:** the public file would keep `position_waypoint` and `position_marked` that the page merges away, and the browser would need FIT enum values (I-2).
- **The display word in the file:** wording would become format; a wording change would bump the version.
- **A per-sample trigger column:** a lap has one value; laps live in the metadata.
- **Per-length distance derived in the browser** (summary distance ÷ lengths, or the stream's first step): rest lengths, drills and missing `lengths` make it wrong.
- **Pool length per session:** the "one shared length" rule already lives in `workout` (F-44 PS-3); a session key would make the browser repeat it.

## Decision

1. **Format version 4**, one bump for both keys. Old workouts are rebuilt by `regenerate`; losses accepted (decision log 11–12 Nov 2026).
2. **`trigger` on every lap**, a string from the closed set `button`, `time`, `distance`, `position`, `stop`, `equipment`, or `""` for a missing, invalid or unlisted value. `ingest`, the anti-corruption layer, maps FIT `lap_trigger` to this set (AP-8 mapping); the raw value never leaves `ingest`. The builders print the AP-8 word for the six codes and an empty cell for anything else.
3. **`pool_length_recorded_m` in `summary`**: the first pool swim's recorded length in metres (3 decimals), present exactly when `pool_length_m` or `pool_length_yd` is. Used only to locate whole lengths in the distance stream. No renderer prints it; `CardItems` and `summaryItems` ignore it.

## Consequences

- The metadata grows by about 20 bytes per lap and one summary key.
- Every golden data file and fixture is regenerated; the deploy runs `regenerate` as for every bump.
- `summary_json` in the database carries the new key unused; nothing reads it there.

## Reversibility

Cheap: the format is rebuilt from originals on every deploy (ADR-005). A new trigger word is a new code plus a bump.

## Would choose differently if

- A lengths table (AP-13) arrives: then per-session or per-length pool data replaces item 3 in that feature's bump.
- Garmin or another writer adds a trigger AP-8 should show separately: add a code, bump.
