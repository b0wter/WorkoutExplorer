# F-04 Workout data file reader: architecture

- **Version:** 1.1
- **Status:** accepted
- **Date:** 11 Oct 2026
- **History:** [architect/history/F-04-workout-data-file-reader.md](history/F-04-workout-data-file-reader.md) (change log, closed questions, the feature-file wording hand-off)
- **Feature:** [`features/F-04-workout-data-file-reader.md`](../features/F-04-workout-data-file-reader.md)
- **Depends on:** TD (system-wide picture), [F-01 design](F-01-project-skeleton-and-ci.md) v1.3 (web conventions, D-8 warning policy), [F-03 design](F-03-workout-data-file-writer.md) v1.3 (**the normative format, §1–§7**), [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-004](ADR-004-parser-corpus-and-golden-reports.md) rev. 5, [ADR-005](ADR-005-workout-data-file-format-v1.md), [ADR-007](ADR-007-decoded-workout-data-in-the-browser.md)
- **Requirements:** AP-4 (only present streams), AP-3 (no position → no map, via `position` absent), NFR Performance (60 fps on 12 hours, data side; decode time measured), NFR Reliability (a broken file never breaks the page); TD Workout data file, TD Front end
- **Owner:** frontend (senior-frontend-engineer). Completes F-03 AC 6 (OQ-8): the TypeScript decoder tests reproduce every committed golden fixture.

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | New pure module `src/web/src/lib/datafile/`. Consumers import only from its `index.ts` | Module boundary |
| D-2 | Output `WorkoutData`: `time`, optional `position` pair, `metrics` record; every array a `Float64Array` in the §2 unit, `NaN` = missing sample | Output model; ADR-007 items 1, 3 |
| D-3 | Delta sums accumulate in a JS number; value = `acc × scale` (§2 literal) | Decoding; ADR-007 item 2 |
| D-4 | Time gaps (≥ 5 s) are reported once as `gaps: Uint32Array`; values are never altered for them | Missing samples and gaps |
| D-5 | `decodeDataFile` returns a result and never throws; error codes plus the §6 rule number | Error model; ADR-007 item 4 |
| D-6 | The decoder enforces all of F-03 §6, **including rule 7**, in the same order as Go `Decode`. It is at least as strict as Go, with one documented exception | Validation |
| D-7 | Metadata keeps the wire names (`start_time`, `heart_rate`, `avg_power`); closed model, unknown keys dropped | Output model |
| D-8 | `loadDataFile(request)` is the only async function; it never rejects | Module boundary |
| D-9 | Synchronous, main thread. Worker only on the ADR-007 trigger | ADR-007 item 5 |
| D-10 | Tests read the committed golden fixtures through `src/web/test-support/golden.mjs`; integers compared exactly, digests via Web Crypto. No Go build needed | Tests |
| D-11 | AC 5: dev-only bench page `src/web/bench/decode.html`; results in `src/web/bench/MEASUREMENTS.md` | Decode timing |
| D-12 | Bundle: 0 bytes in F-04 (nothing imports it yet); ceiling 4,000 bytes gzip once imported | Bundle impact |
| D-13 | `.gitignore` data-file patterns become case-insensitive | Repository change; ADR-004 rev. 5 |

## Boundary with later features

| Not in F-04 | F-04 provides instead | Owner |
| --- | --- | --- |
| Charts, min/max per pixel, drawing breaks at gaps | `metrics`, `time`, `gaps` | F-05 |
| Map line, "No location data" hint | `position` (absent ⇒ hint, map module not loaded) | F-06 |
| Cursor store | Nothing; the cursor is a sample index into these arrays | F-05 |
| Units, pace, cadence label by sport | `METRIC_UNITS`, `meta.sessions[].sport` with index ranges | F-14 |
| Range statistics, prefix sums | Arrays with `NaN` for missing, `gaps` | F-17 |
| User-visible error text | `DataFileError.code` | F-07, F-13 (OQ-10) |
| Which URL, edit token header | `loadDataFile(request)` takes the caller's fetch | F-07, F-19 |

## Module boundary

```
src/web/
├── src/lib/datafile/                  pure TypeScript: no Svelte, no DOM, no Node APIs
│   ├── index.ts                       public surface (the only import path for app code)
│   ├── format.ts                      F-03 §1–§2 as constants: MAGIC, VERSION, HEADER_SIZE, STREAM_SPECS, GAP_SECONDS
│   ├── model.ts                       types only: WorkoutData, WorkoutMeta, Totals, Session, Lap, MetricName, DataFileError, DataFileResult
│   ├── meta.ts                        parseMeta(bytes): rule 4 (UTF-8, JSON, shapes) → WorkoutMeta + descriptors, or error
│   ├── decode.ts                      decodeDataFile(buffer): rules 1–7, views, prefix sums, conversion, gaps
│   ├── load.ts                        loadDataFile(request): fetch result → DataFileResult
│   ├── testing/build-file.ts          test-only builder (never imported by app code)
│   └── *.test.ts                      Vitest, Node environment
├── test-support/
│   ├── golden.mjs                     reads the golden fixtures with node:fs (untyped JS, like scripts/*.mjs)
│   └── golden.d.mts                   its type declarations
└── bench/
    ├── decode.html, decode.ts         dev-only timing page (AC 5); not a build entry
    └── MEASUREMENTS.md                recorded timings
```

**Dependency rules.** `lib/datafile` imports nothing outside itself (no npm packages, no other `lib/` module). App code imports `lib/datafile` (the folder's `index.ts`), never a file inside it. Tests may import any file in the folder. Nothing in `src/` imports `testing/`, `test-support/` or `bench/`.

**Public surface** (`index.ts` exports exactly these):

```ts
export function decodeDataFile(buffer: ArrayBuffer): DataFileResult;          // pure, synchronous, never throws
export function loadDataFile(request: () => Promise<Response>): Promise<DataFileResult>; // never rejects
export type { WorkoutData, Position, MetricName, WorkoutMeta, TimeZone, Totals, Summary, TotalKey,
              Session, Lap, DataFileResult, DataFileError, DataFileErrorCode };
export const METRIC_ORDER: readonly MetricName[];          // §2 order: altitude … temperature
export const METRIC_UNITS: Readonly<Record<MetricName, string>>; // §2 unit strings: 'm', 'm/s', 'bpm', '1/min', 'W', 'degC'
export const GAP_SECONDS = 5;                               // F-02 R-3, F-03 §7
```

`loadDataFile` usage: `loadDataFile(() => fetch(url))`, or with headers for the draft (F-19). It maps: the promise rejects (network error, abort) → `network`; `!response.ok` → `http-status` with `status`; `arrayBuffer()` rejects → `network`; otherwise `decodeDataFile`. Brotli is the browser's business (`Content-Encoding: br`, ADR-006 item 4); the decoder never sees it.

## Output model

```ts
export type MetricName = 'altitude' | 'distance' | 'speed' | 'heart_rate' | 'cadence' | 'power' | 'temperature';

export interface WorkoutData {
  readonly sampleCount: number;            // N from the header, ≥ 1
  readonly time: Float64Array;             // time_offset in s; length N; strictly increasing; never NaN
  readonly gaps: Uint32Array;              // ascending indices i, 1 ≤ i < N, with time[i] − time[i−1] ≥ GAP_SECONDS
  readonly position?: Position;            // present iff latitude and longitude columns exist
  readonly metrics: Readonly<Partial<Record<MetricName, Float64Array>>>; // present streams only, keys in METRIC_ORDER
  readonly meta: WorkoutMeta;
}
export interface Position {
  readonly latitude: Float64Array;         // degrees; NaN exactly where longitude is NaN
  readonly longitude: Float64Array;
}

export type TotalKey = 'elapsed_time' | 'timer_time' | 'distance' | 'ascent' | 'descent' | 'calories'
  | 'avg_speed' | 'max_speed' | 'avg_heart_rate' | 'max_heart_rate' | 'avg_cadence' | 'max_cadence'
  | 'avg_power' | 'max_power' | 'avg_temperature' | 'max_temperature';
export type Totals  = Readonly<Partial<Record<TotalKey, number>>>;                  // absent key = not recorded
export type Summary = Readonly<Partial<Record<TotalKey | 'moving_time', number>>>;
export interface TimeZone { readonly known: boolean; readonly offset_seconds: number }
export interface Session { readonly sport: string; readonly sub_sport: string; readonly start: number; readonly end: number;
                           readonly start_index: number; readonly end_index: number; readonly totals: Totals }
export interface Lap     { readonly start: number; readonly end: number;
                           readonly start_index: number; readonly end_index: number; readonly totals: Totals }
export interface WorkoutMeta { readonly sport: string; readonly start_time: string; readonly time_zone: TimeZone;
                               readonly summary: Summary; readonly sessions: readonly Session[]; readonly laps: readonly Lap[] }
```

| Rule | Detail |
| --- | --- |
| Every array has length N and is index-aligned with `time` | Index *i* is the cursor value (TD Glossary) |
| Values are in the §2 unit | `m`, `m/s`, `bpm`, `1/min`, `W`, `degC`, degrees, seconds. Unit conversion is F-14 |
| Absent stream | Its key is not in `metrics` (`'power' in data.metrics === false`); `position` is `undefined`. Never an all-`NaN` array |
| Metadata | Wire names verbatim (D-7): one name per concept across Go, TypeScript, JSON and these documents. `meta` holds no descriptors (`streams`), they are consumed by the decoder. Unknown keys anywhere are dropped; totals contain only `TotalKey` keys (plus `moving_time` in `summary`). `start_time` stays the RFC 3339 string (F-13 formats it) |
| Lifetime | The result does not reference the input `ArrayBuffer`; the caller may drop it |
| Immutability | `readonly` at type level; no `Object.freeze` (cost without benefit here) |

## Missing samples and gaps

The feature file used to say "turn sentinels into gaps" (corrected wording below, v1.1). In the ubiquitous language these are two different things, and the decoder keeps them apart (TD Glossary):

| Term | In the file | In `WorkoutData` | Who acts on it |
| --- | --- | --- | --- |
| **Missing sample** | Sentinel at index *i* of a present column | `NaN` at index *i* | Charts skip the point, map skips the vertex (AP-3 connects the rest), stats skip it |
| **Gap** | Nothing; `time_offset[i] − time_offset[i−1] ≥ 5` | `i` is in `gaps`; arrays unchanged | Charts draw a break; range statistics do not bridge it (F-03 §7) |
| **Absent stream** | No descriptor, no column | No key / no `position` | Chart and map not shown (AP-3, AP-4) |

`gaps` is computed in the `time_offset` prefix-sum loop (no second pass). Sentinels per type, all turned into `NaN`: int32 `2147483647` (all delta columns), uint16 `65535` (speed, power), uint8 `255` (heart rate, cadence), int8 `127` (temperature).

## Decoding

1. Check rules 1–3 on the header with one `DataView` (little-endian reads; F-03 §1). Fields are not aligned, so no typed arrays here.
2. `parseMeta` (rule 4), then descriptor and layout checks (rules 5–6) for **all** columns before creating any view, so a bad offset can never raise a `RangeError`.
3. Per column, a view `new Int32Array | Uint16Array | Uint8Array | Int8Array (buffer, byte_offset, N)` and one loop into a new `Float64Array(N)`:
   - **delta:** `acc = 0`; `d === 2147483647 ? NaN : (acc += d) × scale`; `acc` is a plain number (ADR-007 item 2).
   - **raw:** `v === sentinel ? NaN : v × scale`.
   - `scale` is the §2 constant from `format.ts`. Rule 5 has already proved the descriptor's scale equal to it, so this is "the scale from each descriptor" (feature scope) without trusting a second copy.
4. Rule 7 checks inside those loops: time missing or not strictly increasing; latitude/longitude `NaN` at different indices; then sessions, then laps (`end < start`, `0 ≤ start_index ≤ end_index ≤ N`).
5. Platform: check little-endian once at module load (`new Uint8Array(new Uint16Array([1]).buffer)[0] === 1`). On a big-endian platform return `unsupported-platform`; no `DataView` fallback (no such browser is in scope, F-03 §4).
6. No per-sample function calls, no `Array` intermediates, no `map`/`forEach` over samples. One `JSON.parse` for the metadata.

**Metadata decoding (rule 4).** `new TextDecoder('utf-8', { fatal: true, ignoreBOM: true })` on bytes `14 … 14+L−1`; invalid UTF-8 → rule 4. `ignoreBOM: true` keeps a BOM in the string so `JSON.parse` rejects it, as Go does. Then `JSON.parse`; the result must be a non-array object. Shape checks:

| Field | Required type |
| --- | --- |
| `sport`, `start_time` | string |
| `time_zone` | object with `known` boolean and `offset_seconds` integer in int32 range |
| `summary`, `totals` | object; each known key, if present, a finite number; unknown keys ignored |
| `sessions`, `laps`, `streams` | array of objects (an element that is not an object, including `null`, is rule 4) |
| Session / lap | `sport`, `sub_sport` (session only) strings; `start`, `end` integers in int32 range; `start_index`, `end_index` integers; `totals` object |
| Descriptor | `name`, `type`, `encoding`, `unit` strings; `scale` number; `byte_offset` integer |

"Integer" means `Number.isInteger`. Every listed key is required, also inside nested objects (see Validation for why this is stricter than Go).

## Validation

The decoder implements every rule of F-03 §6 (rule 7 is optional there; F-04 implements it because cursor search, gap detection and lap ranges rely on it). **Order** (the same as Go `Decode`, so even files with several faults give the same rule):

| Step | Check | Rule |
| --- | --- | --- |
| 1 | length < 14; magic ≠ `WEXD` | 1 |
| 2 | version ≠ 1 | 2 |
| 3 | N = 0; 14 + L > length | 3 |
| 4 | metadata UTF-8, JSON, top-level object, §5.1 keys and kinds, nested shapes (table above) | 4 |
| 5 | `streams` empty; first not `time_offset` | 5 |
| 6 | for each descriptor in order: unknown name; repeated; out of §2 order; type/encoding/unit/scale ≠ §2 row (rule 5); then `byte_offset % 4 ≠ 0`; `< 0` or `> length`; `<` end of previous (the first "previous end" is `align4(14 + L)`); `N × size > length − byte_offset` (rule 6). The next "previous end" is `byte_offset + N × size` | 5, 6 |
| 7 | exactly one of latitude and longitude present | 5 |
| 8 | end of last column ≠ length (trailing bytes) | 6 |
| 9 | time missing or not strictly increasing; latitude/longitude missing at different indices; each session, then each lap: `end < start`, then `start_index < 0`, `start_index > end_index` or `end_index > N` | 7 |

JS numbers are exact up to 2⁵³; N ≤ 2³²−1 and size ≤ 4, so `N × size` is exact (F-03 §6 rule 6).

**Verdict parity with Go `Decode`.** Every file the writer produces is accepted by both. For malformed input the TypeScript decoder is at least as strict, with one exception. The known differences, all outside anything the writer emits (also recorded in F-03 v1.3 §6):

| Input | Go `Decode` | F-04 |
| --- | --- | --- |
| A nested key missing or `null` (lap without `start_index`, `"sessions": [null]`) | zero value, accepted unless a later rule fails | rule 4 |
| A total written as a quoted number (`"distance": "12"`) | accepted (`json.Number`) | rule 4 |
| An `int` field beyond ±2⁶³ (`byte_offset: 1e20`) | rule 4 | rule 6 or 7 (still rejected) |
| An integer spelled with a fraction or exponent (`"start": 4.0`, `4e0`) | rule 4 | **accepted** as 4 (`JSON.parse` cannot tell them apart). The only case where F-04 accepts what Go rejects |

## Error model

```ts
export type DataFileErrorCode =
  | 'not-a-data-file'       // rule 1
  | 'unsupported-version'   // rule 2
  | 'corrupt'               // rules 3–7
  | 'unsupported-platform'  // big-endian
  | 'network'               // loadDataFile: request rejected or body unreadable
  | 'http-status'           // loadDataFile: response not ok
  | 'internal';             // unexpected exception: a bug, never expected
export interface DataFileError {
  readonly code: DataFileErrorCode;
  readonly rule?: 1 | 2 | 3 | 4 | 5 | 6 | 7;  // F-03 §6 rule, for decode errors
  readonly status?: number;                   // for http-status
  readonly message: string;                   // English, for developers and the console
}
export type DataFileResult = { readonly ok: true; readonly data: WorkoutData }
                           | { readonly ok: false; readonly error: DataFileError };
```

- `message` starts with `data file: ` and names the check (`data file: unsupported version 2`, `data file: column heart_rate out of bounds`, `data file: lap 3: end before start`). It may contain names, indices, byte offsets and counts. It never contains a sample value or a metadata value (no coordinates in anything that may end up in a bug report, I-5 in spirit).
- The module never logs. The page decides what to show (OQ-10) and may `console.warn` the message.
- `decodeDataFile` body runs inside one `try`/`catch` that returns code `internal` with the exception's message. Tests assert `internal` never occurs (fuzz tests included).

## Bundle impact (ADR-002)

- **F-04 adds 0 bytes** to the counted bundle: `main.ts` does not import `lib/datafile` until F-05/F-07. The `size` line in CI is unchanged; QA checks it.
- **Ceiling once imported: 4,000 bytes gzip** for everything in `lib/datafile` reachable from `index.ts` (estimate 2,000–3,000). Measured once in F-04: the engineer builds with a temporary `import { decodeDataFile, loadDataFile } from './lib/datafile'; console.log(decodeDataFile, loadDataFile);` in `main.ts`, notes the difference of the `entry index.html` gzip total against the unchanged build, reverts, and reports both numbers in the hand-off. Above 4,000: stop and report (no new dependency would explain it; something is pulled in by mistake).
- No new runtime or dev dependency. `TextDecoder`, typed arrays and `crypto.subtle` (tests) are built in.

## Decode timing (AC 5)

**What is timed:** `decodeDataFile(buffer)` on the in-memory `ArrayBuffer` of `ride-12h.bin` (43,200 samples, 10 streams, 1,176,640 bytes). Not the network, not brotli.

**Bench page** (dev only, not in the production build):

| Item | Value |
| --- | --- |
| Files | `src/web/bench/decode.html`, `src/web/bench/decode.ts` (add `"bench/**/*.ts"` to `tsconfig.json` `include`, so `svelte-check` covers it) |
| Data | `import url from '../../server/testdata/datafile/synthetic/ride-12h.bin?url'`, fetched once into an `ArrayBuffer` before any timing |
| Vite config (amends F-01 "Vite config") | `server.fs.allow: ['.', '../server/testdata/datafile/synthetic']`, so the dev server may serve the synthetic golden files and nothing else outside `src/web`. Comment pointing here. `build` options unchanged: the bench is not a build input, and the manifest must still list only `index.html` as an entry |
| Procedure (v1.1) | A "Run" button (so page-load work is not measured). Per page load: **exactly one cold run** (the first `decodeDataFile` call in that page), then 20 warm runs. After that the button stays disabled and reads "Reload the page for the next cold run". The cold value is appended to `sessionStorage` key `f04-bench-cold` (a JSON array of numbers), so it survives reloads in the same tab. A "Reset" button clears the key. Shows: `ok`, N, streams decoded, this load's cold ms, **all stored cold values and, once there are 5 or more, the median of the last 5**, warm mean (total of 20 / 20), warm max, `navigator.userAgent`. Uses `performance.now()`; its resolution can be 1 ms on phones, which is fine for a cold run in the tens of ms |
| Measurement (v1.1) | Reset, then 5 × (reload, Run). Record the median of the 5 cold values, their min–max, and the last load's warm mean and max |
| Phone access | Preferred: Android Chrome remote debugging with port forwarding of the Vite port (nothing exposed on the network). Otherwise `npm run dev -- --host` on the home network for the duration of the measurement only. Serves synthetic data only |
| Device | The operator's own phone (review decision 32), the same one used for the F-07 gate |
| Record (v1.1) | `src/web/bench/MEASUREMENTS.md`, columns: Date, Commit, Device, OS, Browser and version, File, Samples, **Cold median ms (5 loads)**, **Cold min–max ms**, Warm mean ms, Warm max ms. One row from the engineer's desktop (reference; without a browser, 5 separate Node processes stand in for 5 page loads, and the row says so) and one from the phone. The existing desktop row is re-measured in the new columns |
| Browser check | The operator's phone run (OQ-11) is also the check that the page works in a browser: it counts only if the page shows `ok: true`, N 43,200 and 10 streams. No separate browser run is needed (QA r1 Q2) |

No target in AC 5. The **trigger** from ADR-007 rev. 1 applies: **cold median above 50 ms on the phone** → Worker, in a later change. Warm values are for information only. Vitest does not assert timings (CI runners vary).

## Repository change: `.gitignore` (ADR-004 rev. 5)

Replace the data-file block with exactly:

```
# Data files are never committed, except golden ones from synthetic files (ADR-004 rev. 4, rev. 5)
*.[bB][iI][nN]
*.[bB][rR]
!/src/server/testdata/datafile/synthetic/*.bin
```

Only lower-case `synthetic/*.bin` is re-included, as for FIT files: `X.BIN` there stays ignored. The engineer checks with `git check-ignore -v` that `X.BIN`, `a.Bin.BR`, `src/web/x.bin` are ignored and `src/server/testdata/datafile/synthetic/ride-12h.bin` is not, and that `git status` shows no golden file as deleted.

## Tests

All in Vitest (Node environment, F-01), run by `make test` and CI. No Go toolchain needed.

**How tests find the fixtures.** `src/web/test-support/golden.mjs` (plain JS so `node:fs`/`node:path`/`node:url` need no `@types/node`, F-01) with `golden.d.mts`:

```ts
export interface Golden { name: string; bin: ArrayBuffer; fixture: unknown }
export function loadGolden(): Golden[];   // sorted by name
```

- Folder: `new URL('../../server/testdata/datafile/synthetic/', import.meta.url)`.
- Every `<name>.bin` needs `<name>.json` and the reverse; any other entry, an unpaired file or an empty folder throws (the test fails, never passes vacuously).
- `bin` is a **fresh** `ArrayBuffer` holding exactly the file: `buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength)`. A Node `Buffer` may sit inside a larger pooled buffer, and the decoder requires byte 0 of the file at offset 0.

**Test-only builder.** `lib/datafile/testing/build-file.ts` builds files from **stored integers** (`null` = missing): `buildDataFile({ time, position?, metrics?, meta?, rawMetadata?, offsets?, trailing? }): ArrayBuffer`. It writes the header, the metadata JSON, zero padding and columns at `align4`, delta-encoding delta columns. Overrides let a test break exactly one thing. It does no quantisation (tests pass integers), so it cannot share a rounding bug with anything. The golden fixtures and the hand-written §8 bytes are the independent checks.

| File | Must cover |
| --- | --- |
| `golden.test.ts` (**AC 2**, OQ-8) | For every golden pair: `fixture_version` is 1 (else fail with "fixture shape changed"); `decodeDataFile` ok; `byte_length` = `bin.byteLength`; `sample_count` = `sampleCount`; `metadata_length` = bytes 10–13 read by the test; `format_version` = bytes 4–5; the stream names in order (`time_offset`, then `latitude`, `longitude` if `position`, then `metrics` keys) equal the fixture's names. Per stream: recover stored integers as `k = Math.round(v / s)` with the test's **own** §2 scale literal, assert `Math.abs(v / s − k) < 1e-6` (values lie on the grid, so the right scale was applied); `valid`/`missing` = non-`NaN`/`NaN` counts; `first`, `last`, `min`, `max` equal; `digest` = first 16 hex chars of SHA-256 (`crypto.subtle.digest`) over `k\n` per sample and `-\n` per missing sample, exactly as F-03 "Golden fixtures"; `values[firstValid] === first × s`. Required names present: `ride-12h` (N = 43,200), `gps-dropout` (missing positions), `brick-run-bike`, `indoor-no-gps`, `indoor-no-distance-no-gps`, `pool-swim`. **Derived rejects per fixture**: magic byte changed → rule 1; version set to 2 → rule 2; one byte appended → rule 6; last byte removed → rule 6 |
| `golden.test.ts` (**AC 4**) | `indoor-no-gps`, `indoor-no-distance-no-gps`, `pool-swim`: ok, `position === undefined`, no error |
| `decode.test.ts`: worked example | F-03 §8 written by hand in the test (header bytes, a metadata JSON of the test's choice, column bytes exactly as listed in §8, offsets computed from L; values compared within 1e-9): latitude 47.123456, 47.123458, NaN, 47.124, 47.124001; longitude likewise; heart rate 120, 121, NaN, 131, 132; temperature −2, −3, −3, NaN, −3; `time` 0, 1, 2, 10, 11; `gaps` [3] |
| Alignment (**AC 1**) | L with `(14 + L) % 4` = 0, 1, 2, 3; time plus only uint8/int8 metrics (1–3 padding bytes between columns); N = 1, 2, 3, 5; each decodes correctly. `byte_offset` 2 bytes past a correct offset → rule 6, no `RangeError` |
| Delta decoding (**AC 1**) | First value absolute; missing in the middle (next delta relative to the last valid); leading and trailing missing; one valid value; negative deltas; deltas 2,000,000,000 + 2,000,000,000 on altitude → 400,000,000 m, not wrapped (parity with Go `int64`) |
| Sentinels per type (**AC 1**) | int32 `2147483647` in each delta stream, uint16 `65535` (speed, power), uint8 `255` (heart rate, cadence), int8 `127` (temperature) → `NaN`. Not missing, decoded as numbers: a delta of 2147483646; raw 65534, 254, 126, −128 and 0 |
| Scale (**AC 1**) | speed 7999 → 7.999; latitude 47123456 → 47.123456; altitude 1234 → 123.4; distance 5 → 0.5; temperature −128 → −128; each value `===` stored × §2 literal (bit-identical, multiply not divide). Descriptor `scale` spelled `0.000001` accepted; `1e-05` → rule 5 |
| Absent streams (**AC 1**) | Time only: `metrics` has no keys, `position` undefined; every subset of metrics keeps `METRIC_ORDER`; never an all-`NaN` array for a stream without a column |
| Gaps | Offsets 0, 1, 2, 10, 11 → [3]; a difference of exactly 5 → gap, 4 → none; N = 1 → empty |
| Metadata | All §5 keys mapped with wire names; absent totals stay absent (not 0); `summary` `{}`; `sessions`/`laps` `[]`; unknown keys at every level dropped; `time_zone` unknown |
| Rejects (**AC 3**), one test each, asserting `ok: false`, `code`, `rule`, and that the call does not throw | 0 and 13 bytes; bad magic; version 0 and 2 (`unsupported-version`); N = 0; L beyond end; invalid UTF-8; BOM before the JSON; invalid JSON; top-level array; each of the 7 §5.1 keys missing, and each with the wrong type; lap `start` as string; `"sessions": [null]`; session `start` 2³¹; descriptor `byte_offset` as string; empty `streams`; first stream not `time_offset`; unknown name; repeated name; out of order; latitude without longitude and the reverse; type, encoding, unit, scale each differing from §2; `byte_offset` not a multiple of 4; negative; > length; 4,294,967,292; 9,000,000,000,000,000,000 with N = 4,294,967,295; below `align4(14 + L)`; overlapping the previous column; `N × size` beyond the end; one trailing byte; time missing at one index; time equal and decreasing; half a position at one index; session and lap each: `end < start`, `start_index > end_index`, `end_index > N`, `start_index < 0` |
| Robustness | Seeded PRNG (in the test): 500 random buffers of length 0–600, and 500 copies of `gps-dropout.bin` with one random byte changed. No throw, never code `internal` (a changed column byte may still decode: fine) |
| Platform | Not tested (big-endian not reachable in Node); one line of code |
| `load.test.ts` | Injected `request`: rejects → `network`; status 404 → `http-status`, `status` 404; `arrayBuffer()` rejects → `network`; ok with a golden file → ok; ok with a bad body → `not-a-data-file`; the returned promise never rejects |
| `format.test.ts` | `STREAM_SPECS` equals the F-03 §2 table written out literally in the test (name, type, encoding, scale, unit, size, sentinel, order); `METRIC_UNITS`, `GAP_SECONDS` |

## Acceptance criteria to verification

| AC | Verified by |
| --- | --- |
| 1 Alignment, delta, sentinels per type, absent streams, scale | `decode.test.ts` rows marked AC 1 |
| 2 Decoded values match the F-03 fixture for every corpus file within resolution | `golden.test.ts`: exact stored integers (stronger than "within resolution"), digests, statistics, for every committed synthetic fixture. This completed F-03 AC 6 (OQ-8, closed in v1.1). **Real files (v1.1, QA r1 Q1): a manual QA run is enough.** QA repeats it in every round of a feature that changes `lib/datafile`, the format or the writer, as in F-04 r1 (Go fixtures for `build/corpus/real/*.bin` via a scratch `go test -overlay`, then `golden.test.ts` unchanged against a scratch folder), and reports pass/fail by manufacturer only. Nothing is committed and no repository file changes. **Trigger to automate** (a `GOLDEN_DIR` override in `golden.mjs`, a Go path writing real fixtures under `build/`, a local make target): a real file finds a decoder or writer discrepancy that the synthetic fixtures missed, or QA needs the manual run in a third feature |
| 3 Wrong magic or unknown version → clear error state, no exception | Reject tests (`code`, `rule`, no throw), robustness tests, `load.test.ts`. The visible state is shown by F-07 |
| 4 Indoor file: no position stream, no error | `golden.test.ts` AC 4 row |
| 5 12-hour decode on a mid-range phone measured and recorded | Bench page; rows in `src/web/bench/MEASUREMENTS.md` (desktop by the engineer, phone by the operator); the number read against the ADR-007 trigger is the cold median of 5 loads |

## Open questions

None. OQ-9, OQ-10 and OQ-11 are decided (`INDEX.md`, decision log of `features/README.md`; history). Glossary: a *missing sample* is `NaN`, a *gap* is two samples 5 s or more apart.
