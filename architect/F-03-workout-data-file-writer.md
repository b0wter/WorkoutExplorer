# F-03 Workout data file writer: architecture

- **Version:** 1.5
- **Status:** accepted
- **Date:** 11 Oct 2026; v1.4 of 22 Oct 2026; v1.5 of 26 Oct 2026 (format version 2, F-17 B-1, ADR-029)
- **History:** [architect/history/F-03-workout-data-file-writer.md](history/F-03-workout-data-file-writer.md) (change log, closed questions)
- **Feature:** [`features/F-03-workout-data-file-writer.md`](../features/F-03-workout-data-file-writer.md)
- **Depends on:** TD (system-wide picture), [F-02 design](F-02-fit-parsing-and-normalisation.md) v1.6, [ADR-001](ADR-001-repository-layout-and-build-entry-point.md), [ADR-002](ADR-002-bundle-size-budget-measurement.md) (decimal KB), [ADR-004](ADR-004-parser-corpus-and-golden-reports.md) rev. 3, [ADR-005](ADR-005-workout-data-file-format-v1.md), [ADR-006](ADR-006-brotli-pre-compression.md)
- **Requirements:** NFR Performance (data file under 500 KB compressed for a 4-hour ride; 60 fps scrubbing on 12 hours, data side), AP-4 (only present streams), AP-1 and AP-2 (start time, time zone and summary carried for the page), AP-8 (laps reference the streams), SH-4 (no coordinates in the metadata, prepared for F-16); TD Workout data file, TD Hosting and operations (Local development: `make corpus`)
- **Owner:** backend (senior-backend-engineer). **Reader of the format section:** frontend (senior-frontend-engineer, F-04)

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | New pure package `internal/datafile`: `Encode(workout.Workout) ([]byte, error)` and `Decode([]byte) (File, error)`. Depends on `workout` only | Packages |
| D-2 | New package `internal/precompress`: `Brotli([]byte) ([]byte, error)`, quality 9, window 22 (v1.1). The only importer of `github.com/andybalholm/brotli` v1.2.6 | ADR-006 |
| D-3 | Header exactly as in the TD (10 bytes) plus a uint32 metadata length; columns at absolute byte offsets, each a multiple of 4 | Format §1, §4; ADR-005 items 1–2 |
| D-4 | Quantise the absolute value, then take deltas; a missing sample is the sentinel and is skipped by the running sum | Format §3; ADR-005 items 3–4 |
| D-5 | Out-of-range values are clamped to the type's range without the sentinel, never an error | Format §3; ADR-005 item 5 |
| D-6 | A stream with no valid value is not written; latitude and longitude are written as a pair | Format §2; ADR-005 item 6 |
| D-7 | Metadata v1 is a closed JSON schema with **no coordinates**, no device and no timer events | Format §5; ADR-005 item 7 |
| D-8 | Laps and sessions carry time offsets and half-open sample index ranges | Format §5.4 |
| D-9 | A workout with zero samples cannot be encoded (error). F-08 rejects such files before encoding | Encoder rules |
| D-10 | Golden files `testdata/datafile/synthetic/<name>.bin` and `<name>.json` (decoded-integer digests) for every synthetic corpus file. They are the contract for F-04. No golden files from real files | Golden fixtures; ADR-005 item 10 |
| D-11 | The size budget is guaranteed by construction and asserted in CI on a new synthetic `ride-4h.fit`. Synthetic sizes go in `MEASUREMENTS.md`; real sizes go only in QA reports, by manufacturer | Size budget |
| D-12 | `parse -datafile <path>` writes `<path>` and `<path>.br`; `make corpus` prints both sizes per file | CLI and `make corpus` |

## Boundary with later features

| Not in F-03 | F-03 provides instead | Owner |
| --- | --- | --- |
| Writing `private/{id}/draft.bin`, temp file and rename, deciding whether drafts get a `.br` | `datafile.Encode`, `precompress.Brotli` (pure, no file I/O) | F-09 |
| `public/` folder, content-hashed names (`data.{hash}.bin`), atomic swap, deleting old files | The same two functions; the hash is computed by F-15 over the `.bin` bytes | F-15 |
| Hiding start and end | Encoder rule D-6: positions set to `NaN` in the workout lead to missing samples, or to no position columns at all. The metadata has no coordinates (D-7). F-16 encodes a modified `workout.Workout` and adds no format rule | F-16 |
| Summing totals over several sessions | The metadata `summary` is `workout.Summary` as F-02 or F-08 computes it | F-08 |
| Decoding in the browser, scale application, gaps in the charts | The normative format section below, plus golden fixtures | F-04 |
| Caddy `precompressed br`, `Content-Type`, caching headers | The `.br` is a byte-identical transport encoding of the `.bin` (ADR-006 item 4) | F-22, F-31 |
| `regenerate` | Encoding is deterministic, so regenerating an unchanged original gives identical bytes and therefore the same content hash | F-30 |
| Trim, pins (`pins.json`), preview image, export file | Nothing | phase 2, F-23, F-24 |

## Packages and files

Dependency direction (in TD Code structure, Dependency rules): `cmd/workout-explorer → datafile, precompress`; `datafile → workout` (standard library otherwise, no I/O); `precompress → github.com/andybalholm/brotli` only. `publish` (F-15) and `lifecycle` (F-09) will import both.

```
src/server/
├── go.mod, go.sum                       + github.com/andybalholm/brotli v1.2.6 (ADR-006)
├── cmd/workout-explorer/
│   ├── parse.go                         + -datafile flag
│   └── parse_test.go                    + -datafile cases
├── internal/datafile/                   pure codec, format v2 (ADR-005, ADR-029)
│   ├── datafile.go                      Magic, Version, stream table, File, Stream, Meta types
│   ├── encode.go                        Encode
│   ├── quantise.go                      quantise, clamp, delta (small tested functions)
│   ├── decode.go                        Decode (strict, used by tests, F-16 tests and fixtures)
│   ├── fixture.go                       Fixture(File) ([]byte, error): golden JSON for F-04
│   ├── *_test.go                        unit tests (in-memory workouts)
│   └── corpus_test.go                   TestRoundTripCorpus, TestDataFileCorpus (-update), TestSizeBudget4h, benchmarks
├── internal/precompress/
│   ├── precompress.go                   Brotli
│   └── precompress_test.go
├── internal/corpustest/                 test support, imported by _test.go files only
│   └── corpustest.go                    Files(t, root, set): the corpus listing and name rule, moved from ingest/corpus_test.go
├── scripts/corpus.sh                    + -datafile, sizes in the status line
├── tools/corpusgen/main.go              + ride4h()
└── testdata/
    ├── corpus/synthetic/ride-4h.fit     new synthetic file (+ golden report, README row)
    ├── corpus/MEASUREMENTS.md           + section "Data file sizes (F-03)"
    └── datafile/                        golden data files (D-10)
        ├── README.md
        └── synthetic/<name>.bin, <name>.json
```

`internal/corpustest` is an ordinary package so that `ingest` and `datafile` tests share the listing and the ADR-004 item 4 name rule instead of copying it. Only `_test.go` files import it, so it never enters the binary. The QA dependency check (`go list -deps ./cmd/...`) must not list it.

**Public surface** (nothing else exported apart from these types and their fields):

```go
package datafile

const Magic = "WEXD"
const Version uint16 = 1

func Encode(w workout.Workout) ([]byte, error)   // deterministic; never panics
func Decode(b []byte) (File, error)              // strict: every check in Format §6
func Fixture(f File) ([]byte, error)             // golden JSON (section Golden fixtures)

type File struct {
    Version        uint16
    SampleCount    int
    MetadataLength int
    Meta           Meta
    Streams        []Stream          // descriptor order
}
type Stream struct {
    Descriptor Descriptor
    Ints       []int64              // decoded absolute quantised values; 0 where missing
    Valid      []bool
}
func (s Stream) Values() []float64  // Ints[i] × scale; NaN where !Valid[i]

type Meta struct {                  // JSON key order = field order (Format §5)
    Sport     string       `json:"sport"`
    StartTime string       `json:"start_time"`
    TimeZone  TimeZone     `json:"time_zone"`
    Summary   Summary      `json:"summary"`
    Sessions  []Session    `json:"sessions"`
    Laps      []Lap        `json:"laps"`
    Streams   []Descriptor `json:"streams"`
}
// TimeZone, Totals, Summary, Session, Lap, Descriptor: exactly the keys in Format §5.
// Totals fields are json.Number with omitempty.

package precompress

func Brotli(src []byte) ([]byte, error)
```

**Third-party module justification** (TD Code structure, Dependency rules): `github.com/andybalholm/brotli`. The standard library has no brotli encoder, Caddy cannot compress with brotli on the fly, and cgo is excluded (ADR-006).

---

## Data file format, version 2 (normative)

(v1.5) Version 2 adds the streams 11 and 12 of §2 and nothing else; the header, the metadata schema and every other rule are those of version 1.

This section is the format description required by the feature's scope. It is written for an independent decoder in any language. Terms: *N* is the sample count; *L* is the metadata length; `align4(x)` is the smallest multiple of 4 that is ≥ x. All multi-byte integers are **little-endian two's complement**. Byte positions count from 0 at the start of the file.

### 1. Header

| Bytes | Type | Field | Value |
| --- | --- | --- | --- |
| 0–3 | 4 × ASCII | magic | `WEXD` (`57 45 58 44`) |
| 4–5 | uint16 | format version | `2` |
| 6–9 | uint32 | sample count *N* | ≥ 1 |
| 10–13 | uint32 | metadata length *L* | length of the JSON in bytes |
| 14 … 14+L−1 | UTF-8 | metadata | one JSON object (§5), no BOM, no trailing newline |
| 14+L … align4(14+L)−1 | | padding | 0 to 3 bytes `00` |

The header fields are not 4-aligned. Read them with `DataView` (or the language's equivalent), not with typed arrays.

### 2. Streams

There is one column per **present** stream. A stream that has no valid sample in the workout has no column and no descriptor. Columns appear in this order (absent ones are skipped). `time_offset` is always present and always first.

| # | `name` | `type` | `encoding` | `scale` | `unit` | Factor (= 1/scale) | Valid stored range | Sentinel (missing) | Resolution |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `time_offset` | `int32` | `delta` | `1` | `s` | 1 | 0 … 1,000,000,000 | 2147483647 (never occurs) | 1 s |
| 2 | `latitude` | `int32` | `delta` | `1e-06` | `deg` | 1,000,000 | −90,000,000 … 90,000,000 | 2147483647 | 10⁻⁶° |
| 3 | `longitude` | `int32` | `delta` | `1e-06` | `deg` | 1,000,000 | −180,000,000 … 180,000,000 | 2147483647 | 10⁻⁶° |
| 4 | `altitude` | `int32` | `delta` | `0.1` | `m` | 10 | −1,000,000,000 … 1,000,000,000 | 2147483647 | 0.1 m |
| 5 | `distance` | `int32` | `delta` | `0.1` | `m` | 10 | −1,000,000,000 … 1,000,000,000 | 2147483647 | 0.1 m |
| 6 | `speed` | `uint16` | `raw` | `0.001` | `m/s` | 1,000 | 0 … 65,534 | 65535 | 1 mm/s |
| 7 | `heart_rate` | `uint8` | `raw` | `1` | `bpm` | 1 | 0 … 254 | 255 | 1 bpm |
| 8 | `cadence` | `uint8` | `raw` | `1` | `1/min` | 1 | 0 … 254 | 255 | 1 per minute |
| 9 | `power` | `uint16` | `raw` | `1` | `W` | 1 | 0 … 65,534 | 65535 | 1 W |
| 10 | `temperature` | `int8` | `raw` | `1` | `degC` | 1 | −128 … 126 | 127 | 1 °C |
| 11 | `moving_time` | `int32` | `delta` | `1` | `s` | 1 | 0 … 1,000,000,000 | 2147483647 (never occurs) | 1 s |
| 12 | `elevation_gain` | `int32` | `delta` | `0.1` | `m` | 10 | 0 … 1,000,000,000 | 2147483647 (never occurs) | 0.1 m |

- **Decoded value** = stored absolute integer × `scale`, in `unit`. For delta columns, the "stored absolute integer" is the running sum from §3.
- **Cadence** is per minute: steps for samples in running, walking and hiking sessions, revolutions or strokes otherwise (F-02 R-5). The viewer chooses the label from the session's sport (`sessions[].sport` and its sample range).
- **Latitude and longitude** are both present or both absent. At every index both are valid or both are missing.
- **Sizes in bytes:** `int32` 4, `uint16` 2, `uint8` 1, `int8` 1. With every stream present, one sample takes 35 bytes (v1.5; 27 in version 1).
- **Cumulative streams (v1.5, ADR-029).** `moving_time` is the moving time in seconds within `[0, time_offset[i]]` (F-02 R-7, R-13.M included); `elevation_gain` is the elevation gain in metres up to sample *i* (F-02 R-8, a climb counts as it rises). Both start at 0 (moving time counts from offset 0), never decrease, and end at the summary's derived moving time and gain (`elevation_gain` to 0.1 m; if timer events lie outside the samples, the last `moving_time` value carries them). They have no missing samples. `moving_time` is absent when the workout has no moving-time source, `elevation_gain` when it has fewer than two altitudes; absence is by W-2 as for any stream. They are not charted. A range's moving time or gain is the difference of two values. Hiding (F-16) does not change them

### 3. Values, quantisation and encodings

**Writer.** For each stream and sample, with the input value `v` in the unit above (F-02 Units):

1. If `v` is missing (`NaN`, or `±Inf`), the sample is **missing**.
2. Otherwise `q = clamp(round(v × factor))`, where `round` is half away from zero and `clamp` limits to the valid stored range of the stream (the table above).

**`raw` columns** store `q` at index *i*, or the sentinel if the sample is missing.

**`delta` columns** store, at index *i*:
- the sentinel `2147483647` if the sample is missing;
- otherwise `q[i] − q[p]`, where *p* is the nearest earlier index with a valid sample, or `q[i]` itself if there is no such index.

**Reader (normative decoding of a `delta` column):**

```
acc := 0
for i in 0 … N−1:
    d := column[i]
    if d == 2147483647: value[i] = missing
    else:               acc = acc + d; value[i] = acc × scale
```

Every intermediate `acc` is a valid stored value, so it stays within ±1,000,000,000 and fits int32 and float64 exactly. A `raw` column is missing where it holds the sentinel, otherwise its value is `stored × scale`.

**Accuracy guarantee.** For every valid sample that was not clamped, |decoded − original| ≤ scale / 2 (plus floating-point error of the multiplication). Missing samples stay missing, and valid samples never become missing.

### 4. Columns and alignment

- Column *k* starts at its descriptor's `byte_offset` and is `N × size(type)` bytes long.
- The writer places the first column at `align4(14 + L)`. Each following column starts at `align4(end of the previous column)`. Padding bytes are `00`. There is **no** padding after the last column, and no other bytes either: the file length is exactly `byte_offset(last) + N × size(last)`. Trailing bytes make the file invalid (§6 rule 6, v1.1).
- Every `byte_offset` is a multiple of 4. A reader can therefore create `Int32Array`, `Uint16Array`, `Uint8Array` or `Int8Array` views directly on the `ArrayBuffer` of the whole file, without copying. Readers must use `byte_offset` and must not compute positions themselves.
- Typed arrays use the platform's byte order. Every browser platform in scope is little-endian. A reader may assert this once and otherwise fall back to `DataView`.

### 5. Metadata JSON (schema unchanged since v1, closed)

One JSON object with exactly these keys, in this order. Readers use a standard JSON parser. Numbers are JSON numbers. Unknown keys do not occur.

```json
{
  "sport": "cycling",
  "start_time": "2026-06-06T05:00:00Z",
  "time_zone": {"known": true, "offset_seconds": 7200},
  "summary": {"elapsed_time": 3600, "distance": 27012.5, "ascent": 112, "avg_heart_rate": 141, "moving_time": 3580},
  "sessions": [
    {"sport": "cycling", "sub_sport": "road", "start": 0, "end": 3600, "start_index": 0, "end_index": 3601,
     "totals": {"elapsed_time": 3600, "distance": 27012.5}}
  ],
  "laps": [
    {"start": 0, "end": 1800, "start_index": 0, "end_index": 1801, "totals": {"elapsed_time": 1800}},
    {"start": 1800, "end": 3600, "start_index": 1800, "end_index": 3601, "totals": {"elapsed_time": 1800}}
  ],
  "streams": [
    {"name": "time_offset", "type": "int32", "encoding": "delta", "scale": 1, "unit": "s", "byte_offset": 1104},
    {"name": "heart_rate", "type": "uint8", "encoding": "raw", "scale": 1, "unit": "bpm", "byte_offset": 15508}
  ]
}
```

(The writer emits compact JSON without whitespace. The example is formatted for reading, and its numbers are illustrative.)

#### 5.1 Top level

| Key | Type | Always present | Meaning |
| --- | --- | --- | --- |
| `sport` | string | yes | Workout sport (F-02 R-9): FIT profile name in snake_case (`cycling`, `running`, `e_biking`, …), `multisport` for 2+ sessions, `unknown_<n>` for values not in the profile, `""` when not recorded |
| `start_time` | string | yes | Time offset 0 in UTC, RFC 3339 with whole seconds and `Z`: `YYYY-MM-DDTHH:MM:SSZ` |
| `time_zone` | object | yes | `known` (bool), `offset_seconds` (integer, multiple of 900). Local start time = `start_time` + `offset_seconds` when `known`; otherwise show UTC, labelled as UTC (decision 13, F-02 R-9) |
| `summary` | Totals + `moving_time` | yes (may be `{}`) | `workout.Summary` (F-02 R-10): the numbers the page shows (AP-2) |
| `sessions` | array of Session | yes (may be `[]`) | File order |
| `laps` | array of Lap | yes (may be `[]`) | File order |
| `streams` | array of Descriptor | yes, ≥ 1 | Column order (§2), `time_offset` first |

#### 5.2 Totals

An object whose keys are a subset of the list below, in this order. **A key that is absent means "not recorded".** The viewer hides that metric and never shows 0 (AP-2). Units are those of F-02 section Units.

`elapsed_time` (s), `timer_time` (s), `distance` (m), `ascent` (m), `descent` (m), `calories` (kcal), `avg_speed` (m/s), `max_speed` (m/s), `avg_heart_rate` (bpm), `max_heart_rate` (bpm), `avg_cadence` (1/min), `max_cadence` (1/min), `avg_power` (W), `max_power` (W), `avg_temperature` (°C), `max_temperature` (°C). `summary` alone adds `moving_time` (s) as its last key.

**Number format:** the value rounded to 3 decimals (half away from zero), then written in the shortest form that reads back exactly (Go `strconv.FormatFloat(r, 'f', -1, 64)`), with `-0` written as `0`. Examples: `3600`, `27012.5`, `7.999`.

#### 5.3 Session and Lap

| Key | Type | Session | Lap | Meaning |
| --- | --- | --- | --- | --- |
| `sport`, `sub_sport` | string | yes | — | As `sport` above (`""` when not recorded) |
| `start`, `end` | integer | yes | yes | Time offsets in seconds (F-02 E-2). `end ≥ start` |
| `start_index`, `end_index` | integer | yes | yes | Half-open sample range (§5.4) |
| `totals` | Totals | yes | yes | Session: device values as recorded (AP-2). Lap: the device value where recorded, else computed as for a range over the lap's samples ([F-18](F-18-laps-and-splits-table.md) L-1; AP-8 columns only) |

#### 5.4 Sample index ranges

For a lap or session with offsets `start` and `end`:

- `start_index` = the smallest *i* with `time_offset[i] ≥ start`, or *N* if there is none;
- `end_index` = the smallest *i* with `time_offset[i] > end`, or *N* if there is none.

The lap's samples are `start_index ≤ i < end_index`. The range is empty when the two are equal (for example a lap inside a gap). Adjacent laps share their boundary sample when one ends at exactly the offset where the next starts. This is deliberate: a lap row selects a range that includes its own end point.

#### 5.5 Descriptor

| Key | Type | Meaning |
| --- | --- | --- |
| `name` | string | One of the names in §2, unique |
| `type` | string | `int32`, `uint16`, `uint8` or `int8` |
| `encoding` | string | `delta` or `raw` |
| `scale` | number | Multiplier from stored integer to `unit`. The writer emits exactly the spellings of the §2 table: `1`, `1e-06`, `0.1` and `0.001` (v1.1; stored as `json.Number`, so the golden bytes are stable). **Readers compare numerically**: the parsed value must equal the parsed §2 literal (`JSON.parse` already does this), so the spelling never matters to a reader |
| `unit` | string | As in §2 |
| `byte_offset` | integer | Absolute position of the column in the file, a multiple of 4 |

`type`, `encoding`, `scale` and `unit` must equal the §2 row for `name`. The descriptor is self-describing, but the format does not allow variants.

#### 5.6 What the metadata never contains

No latitude or longitude in any form (no positions, bounding box, start point or lap positions), no device or serial number, no file type, no timer events, no derived values beyond `summary` and the computed lap values (§5.3). Coordinates exist **only** in the `latitude` and `longitude` columns (D-7; F-16 relies on this).

### 6. Reader validation (both the Go `Decode` and the F-04 decoder)

A reader rejects the file, with an error and never a crash, when any of these holds:

1. Fewer than 14 bytes, or magic ≠ `WEXD`.
2. Version ≠ the reader's version (exactly `2`; there are no readers for other versions, TD Versioning).
3. *N* = 0, or `14 + L` > file length.
4. The metadata is not valid UTF-8 JSON, or a key from §5.1 is missing or has the wrong type. **Nested keys (v1.3):** a nested value of the wrong type (for example a lap `start` that is a string) is rejected by every reader. A **missing or `null`** nested key (a key of `time_zone`, §5.3 or §5.5, or a `null` array element) is not covered by the writer's output, since the writer always emits every key; Go `Decode` zero-fills it, F-04 rejects it. Both are allowed by this format; neither affects a file the writer produced.
5. `streams` is empty, its first entry is not `time_offset`, a name is unknown or repeated, the names are not in §2 order (v1.2), exactly one of `latitude` and `longitude` is present (v1.2), or a descriptor differs from its §2 row.
6. A `byte_offset` is not a multiple of 4, is greater than the file length (v1.2), is below `align4(14 + L)`, is below the end of the previous column, or `N × size` > file length − `byte_offset`; or the file is longer than the end of the last column (trailing bytes, v1.1). Evaluate the bounds in a form that cannot overflow: in Go, check `byte_offset ≤ len` first, then `N ≤ (len − byte_offset) / size`. In JS, numbers up to 2⁵³ are exact (v1.2). This rule applies to F-04 as well.
7. (Go `Decode`; optional in this format, implemented by F-04 too, v1.3.) `time_offset` has a missing sample or is not strictly increasing; a session or lap index range is not `0 ≤ start_index ≤ end_index ≤ N`; a session or lap has `end < start`; latitude and longitude are not missing at the same indices (v1.2); `moving_time` or `elevation_gain` has a missing sample (v1.5).

**Verdict differences between Go `Decode` and F-04 (v1.3).** Every writer output is accepted by both, and F-04 checks in the same order as Go. For malformed metadata they differ only here (details in the [F-04 design](F-04-workout-data-file-reader.md#validation)): missing or `null` nested keys and totals written as quoted numbers (Go accepts, F-04 rule 4); integer fields beyond ±2⁶³ (Go rule 4, F-04 rule 6 or 7); integers written with a fraction or exponent such as `4.0` (Go rule 4, F-04 accepts, because `JSON.parse` cannot tell `4.0` from `4`).

### 7. Time axis, gaps and missing data

- `time_offset[i]` is whole seconds since `start_time`, strictly increasing. There is at most one sample per second.
- **Gap:** two consecutive samples whose offsets differ by **5 s or more** (F-02 R-3, TD Glossary). The file has no other gap marker. Charts draw a break, and range statistics do not bridge it. Differences of 2 to 4 s do not occur, because F-02 fills them.
- **Missing sample:** a present stream holds the sentinel at that index (for example a GPS dropout: latitude and longitude missing, other streams valid).
- **Absent stream:** no descriptor (for example indoor files: no `latitude`, no `longitude`).

### 8. Worked example

Five samples. Offsets 0, 1, 2, 10, 11 (one gap of 8 s). Assume the metadata is *L* = 401 bytes.

| i | offset | latitude | longitude | heart rate | temperature |
| --- | --- | --- | --- | --- | --- |
| 0 | 0 | 47.1234564 | 8.5000004 | 120.4 | −2.4 |
| 1 | 1 | 47.1234580 | 8.5000011 | 121 | −2.6 |
| 2 | 2 | NaN | NaN | NaN | −3 |
| 3 | 10 | 47.1240004 | 8.5000500 | 130.6 | NaN |
| 4 | 11 | 47.1240012 | 8.5000507 | 132 | −3 |

| Bytes | Content |
| --- | --- |
| 0–13 | `57 45 58 44` · `02 00` · `05 00 00 00` · `91 01 00 00` (L = 401) |
| 14–414 | metadata JSON |
| 415 | `00` (padding to 416) |
| 416–435 | `time_offset` int32 delta: `0, 1, 1, 8, 1` |
| 436–455 | `latitude` int32 delta: `47123456, 2, 2147483647, 542, 1` → decodes to 47.123456, 47.123458, missing, 47.124000, 47.124001 |
| 456–475 | `longitude` int32 delta: `8500000, 1, 2147483647, 49, 1` → 8.500000, 8.500001, missing, 8.500050, 8.500051 |
| 476–480 | `heart_rate` uint8: `78 79 FF 83 84` (120, 121, missing, 131, 132) |
| 481–483 | `00 00 00` (padding) |
| 484–488 | `temperature` int8: `FE FD FD 7F FD` (−2, −3, −3, missing, −3) |

The descriptors carry `byte_offset` 416, 436, 456, 476 and 484. The file length is 489 bytes.

### 9. Versioning

`Version = 2` (v1.5; 1 before F-17). Any change to §1–§7 bumps it. The writer, the F-04 reader, this section and the golden files change together. Only one version is live at a time: every deploy regenerates every workout (TD Versioning, F-30), so readers support exactly one version (ADR-005 item 9). (v1.4, OQ-55) Exception: a workout kept after a failed rebuild keeps its previous data file and the previous bundle that reads it until a later run rebuilds it; an old bundle is deleted only once no kept workout refers to it. Each reader still supports exactly one version (ADR-005 rev. 1 item 9).

---

## Encoder rules (`datafile.Encode`)

| # | Rule |
| --- | --- |
| W-1 | Input is a normalised `workout.Workout` (F-02 invariants hold). `Encode` checks the invariants it relies on and returns an error, never panics: `len(Samples.Offset) == 0` → `datafile: no samples` (D-9); a present stream whose length ≠ *N*; an offset < 0, > 1,000,000,000 or not strictly increasing; **(v1.2) a session or lap with `End < Start`, or with `Start < 0`** → `datafile: lap 3: end before start` (index, no values). This is an error, not a clamp: F-02 E-2 already guarantees `End ≥ Start`, so a violation means a caller (F-16, F-30) built a broken workout, and repairing it silently would hide that bug |
| W-2 | Presence: a stream is written if it is non-nil **and** has at least one valid value after the pairing rule W-3. Otherwise it is skipped |
| W-3 | Pairing: at index *i*, if either `Lat[i]` or `Lon[i]` is missing, both are written as missing. If either slice is nil, neither is written |
| W-4 | Quantisation per Format §3: `math.Round(v * factor)`, then clamp. Factors as integer constants (`1e6`, `10`, `1000`, `1`), multiplied, never divided |
| W-5 | `start_time`: `w.StartTime.UTC().Format(time.RFC3339)`. With *N* ≥ 1 it is never zero (F-02 R-1) |
| W-6 | Totals per Format §5.2, built from `workout.Totals` and `workout.Summary`; `nil` → key omitted |
| W-7 | Index ranges per Format §5.4 with `sort.Search` on `Samples.Offset` |
| W-8 | Metadata: `json.Marshal` of `Meta` (compact, no trailing newline). The descriptors' `byte_offset` values depend on *L*, and *L* depends on the descriptors, so compute them as follows: marshal once with `byte_offset` 0 to find the field widths, assign offsets, marshal again, and repeat until *L* is stable (two or three passes at most, since *L* only grows). Any other method that produces the same bytes is fine |
| W-9 | Deterministic: the same `Workout` gives identical bytes (no maps in `Meta`, no time-dependent values) |
| W-10 | Errors are prefixed `datafile:`. `Encode` never logs and holds no coordinates in error messages (I-5) |

`Decode` implements Format §6 completely, including rule 7. Its errors are prefixed `datafile:` and name the failing check (`datafile: bad magic`, `datafile: unsupported version 2`, `datafile: column heart_rate out of bounds`, …).

## Pre-compression (`precompress.Brotli`)

`Brotli(src)` returns `src` brotli-compressed with `WriterOptions{Quality: 9, LGWin: 22}` (v1.1, ADR-006 rev. 1). **One compression at a time per process (v1.2, QA r1 Q3):** `Brotli` holds a package-level mutex for the whole compression. QA measured about 52 MB allocated per 12-hour compression, so concurrent publishes, uploads and regeneration could otherwise add up on a small VPS. Callers (F-09, F-15, F-30) need no locking of their own. F-09 should not compress `draft.bin` at all unless its design gives a reason. Empty input is allowed and returns a valid brotli stream. The result decompresses to exactly `src` (ADR-006 items 3–4).

## Size budget

**Target:** a 4-hour ride with every stream is under **500,000 bytes** (decimal KB, as in ADR-002) as `data.bin.br`. The measured quantity is the length of the `.br` file in bytes, which equals what Caddy transfers as the body.

**Why it holds by construction:** at most one sample per second (F-02 R-2), so a 4-hour workout has at most 14,401 samples. With at most 35 bytes per sample (27 in version 1), the columns take at most 504,035 bytes uncompressed; the budget is held by compression (ADR-029, measured in `MEASUREMENTS.md`: `ride-4h` is 109,596 bytes brotli). The header and all padding add at most 27 bytes (14 for the header, at most 13 for padding). The uncompressed file therefore stays under 500,000 bytes while the metadata is under about 111 KB, and brotli adds the margin. The CI test below makes this observable and catches a regression (for example a stream type widened by mistake).

**Measured, three places:**

| What | Where | Committed |
| --- | --- | --- |
| `TestSizeBudget4h`: `synthetic/ride-4h.fit` → `Encode` → `Brotli` → length < 500,000. It also asserts 14,400 samples and all 10 streams, so it cannot pass vacuously. It logs both sizes | `internal/datafile/corpus_test.go`, in `go test` and CI | yes (test) |
| `.bin` and `.br` size of every **synthetic** corpus file, the `.br` size extrapolated to 4 h (`br × 14,400 / N`, only for files of 1 h or more), plus `BenchmarkEncode12h` and `BenchmarkBrotli12h` | `testdata/corpus/MEASUREMENTS.md`, new section "Data file sizes (F-03)": Date, Commit, File, Samples, Streams, `data.bin` bytes, `data.bin.br` bytes, `.br` per 4 h; and a benchmark table: Date, Commit, CPU, Go, benchmark, ns/op, B/op, allocs/op | yes, synthetic only |
| Sizes of **real** files | The QA report only. One row per file, identified by `device.manufacturer` and sport, with duration rounded to 0.5 h, samples, present stream names, `.bin` bytes, `.br` bytes and `.br` per 4 h. No file names beyond the manufacturer, no dates, no routes (ADR-004 item 7, rev. 3) | no |

**New synthetic file `ride-4h.fit`** (via `corpusgen`, ADR-004 item 5). It is deliberately noisier than the other synthetic files, so its compressed size is not flattered by smooth signals:

- 14,400 records at 1 Hz (offsets 0 to 14,399), one `cycling`/`road` session, laps every 900 s (16), timer start at 0 and stop at the end, enhanced fields, local offset +7,200 s. Any start date will do; sharing a date with another synthetic file is fine (v1.1).
- Base: `outdoorRide` with its own seed (for example 33), speed `7 + 1.5·sin(t/1100) + U(−0.5, 0.5)` m/s, altitude `hills(t)`.
- Then add, from a separate fixed seed: **GPS jitter** `U(−1.5, 1.5)` m per axis per sample on latitude and longitude only (not accumulated, not added to distance); **altitude noise** `U(−0.3, 0.3)` m; **power** replaced by `round(200 + 60·sin(t/150) + U(−80, 80))`, with a minimum of 0.
- Corpus README row: "Size budget (F-03 AC 5): 4 h at 1 Hz, every stream, extra noise on positions, altitude and power". The golden report goes into `expected/synthetic/` as usual.

## Golden fixtures (the contract for F-04)

**Location:** `src/server/testdata/datafile/synthetic/`. For every file in `testdata/corpus/synthetic/` there are two files:

- `<name>.bin`: the exact output of `Encode` for that corpus file.
- `<name>.json`: `Fixture(Decode(<name>.bin))`, the expected decoded values.

The folder sits outside `testdata/corpus/` on purpose, because the corpus drivers would treat these files as stale reports. There is no `real/` side. Data files from real files are written only to `build/corpus/real/` by `make corpus` (ADR-004 item 7, rev. 3).

**`.gitignore` (v1.2, QA r1 Q2; ADR-004 rev. 4; case-insensitive since v1.3, ADR-004 rev. 5).** `parse -datafile` can write a data file with real coordinates anywhere, so data files are ignored everywhere except the golden folder, in every spelling of the extension (applied in F-04):

```
# Data files are never committed, except golden ones from synthetic files (ADR-004 rev. 4, rev. 5)
*.[bB][iI][nN]
*.[bB][rR]
!/src/server/testdata/datafile/synthetic/*.bin
```

**`ride-12h` golden file stays (v1.2, QA r1 Q1).** It is the only fixture with a large *N*: it exercises the 43,200-sample prefix sums in F-04 and is the input for F-04 AC 5 (decode timing on the phone), and F-04's CI tests must not need a Go build. Its cost, about 1.2 MB rewritten on each normaliser or format change, is accepted. **Drop it** (and let F-04 generate it from `make corpus` output) if `testdata/datafile/` passes 5 MB or the repository's `.git` passes 100 MB.

**Fixture JSON** (`json.MarshalIndent`, 2 spaces, trailing newline; integers and strings only, so every language reads it identically). The real files have **one key per line**, exactly as `MarshalIndent` writes them. The example below is condensed and shows only the keys (v1.1):

```json
{
  "fixture_version": 1,
  "format_version": 1,
  "byte_length": 489,
  "sample_count": 5,
  "metadata_length": 401,
  "streams": [
    {"name": "time_offset", "byte_offset": 416, "valid": 5, "missing": 0,
     "first": 0, "last": 11, "min": 0, "max": 11, "digest": "0123456789abcdef"}
  ]
}
```

| Key | Meaning |
| --- | --- |
| `fixture_version` | `1`; bumped when the fixture's shape changes (v1.2) |
| `format_version` | The data file's header version |
| `byte_length` | File length in bytes = end of the last column (§4) |
| `sample_count` | *N* from the header |
| `metadata_length` | *L* from the header |
| `streams[]` | One entry per descriptor, in order |
| `valid`, `missing` | Counts of valid and missing samples |
| `first`, `last`, `min`, `max` | Over the valid samples, as **stored absolute integers** (after the delta sum, before `scale`) |
| `digest` | SHA-256 over every sample in index order. Each sample is written as its stored absolute integer in decimal (`-` for missing) followed by `\n`. The result is in lower-case hex, first 16 characters. Same convention as the F-02 report digest. For `time_offset` it therefore equals the report's `samples.offsets_digest` |

The digest compares integers, not floats, so Go and TypeScript cannot disagree about rounding or formatting. The F-04 tests check scale application separately, for example `first × scale`.

**Kept current by `TestDataFileCorpus`** (in `go test`, so CI). For each synthetic corpus file it encodes, decodes and builds the fixture, then compares both with the committed files **byte for byte**. A missing golden file fails as "new", and a golden file without a corpus file fails as "stale". With `-update`, it rewrites the golden files and removes stale ones. `make corpus-update` runs it next to `TestCorpus`, and the diff of `testdata/datafile/` is reviewed with the code. The reference platform is linux/amd64, as in ADR-004 item 6.

`testdata/datafile/README.md` (a few lines): what the files are, that they are generated by `make corpus-update` and never edited by hand, that F-04's decoder tests read them, and that they come from synthetic files only.

## CLI and `make corpus`

**`workout-explorer parse [-datafile <path>] <file.fit>`.** Without the flag, the behaviour is unchanged (F-02). With the flag:

1. Parse, build the report, `Encode` and `Brotli` everything in memory first.
2. Write `<path>` and `<path>.br` (mode 0644; the folder must exist).
3. Print the report to stdout.

Any failure in 1 or 2 exits 1 with one line `parse: <error>` on stderr and nothing on stdout. A file that parses but cannot be encoded (zero samples) therefore fails. Usage text: `parse    print the normalised workout of a FIT file; -datafile also writes data.bin and data.bin.br (diagnostic)`. The written files contain coordinates. Like the report, they go only where the person running the command chooses (I-5 is unaffected).

**`scripts/corpus.sh`.** For each file, call `"$bin" parse -datafile "$out/$set/$name.bin" "$fit"`. For every file whose parse exits 0 (lines `ok`, `DIFF`, `NEW`), the status line ends with two spaces and `bin <bytes> br <bytes>` (sizes from `wc -c`, plain integers). For example (illustrative numbers):

```
ok     synthetic/ride-4h.fit  bin 388876 br 141220
```

Nothing else changes: the counts, the exit status and the `real: N files` line stay as they are. Sizes of real files appear only in the terminal.

**`Makefile`.** `corpus-update` runs both golden updates:

```
cd $(SERVER_DIR) && go test ./internal/ingest -run '^TestCorpus$$' -update && go test ./internal/datafile -run '^TestDataFileCorpus$$' -update
```

`make corpus-gen` is unchanged (it picks up `ride4h` from `corpus()`). CI is unchanged: the new tests run in `go test ./...`.

## Tests

**`internal/datafile` unit tests** (in-memory workouts, no files):

| Area | Must cover |
| --- | --- |
| Header | Magic, version 2, *N*, *L* at bytes 10–13; the JSON at 14 parses; padding bytes are `00`; the first column is at `align4(14 + L)` |
| Worked example | The Format §8 workout encodes to exactly those column bytes, offsets relative to `align4(14 + L)`, and decodes back to the listed values |
| Alignment | Every `byte_offset % 4 == 0` for all of these: every stream; time with only uint8 streams (padding of 1–3 bytes between columns); *N* = 1, 2, 3, 5; no trailing padding |
| Order and presence | Canonical order; absent stream (nil) → no descriptor; all-`NaN` stream → no descriptor; `±Inf` → missing |
| Pairing (W-3) | Lat valid and lon `NaN` → both missing; all positions `NaN` → no `latitude` or `longitude` descriptor (prepares F-16 AC 5) |
| Quantisation | Half away from zero: heart rate 120.5 → 121, temperature −2.5 → −3; latitude −33.8688197 → −33,868,820; speed 7.999 → 7999 |
| Clamping | Cadence 300 → 254; temperature −200 → −128 and 130 → 126; speed 70 m/s → 65,534; power −5 → 0; altitude 2×10⁸ m → 1,000,000,000; a valid value never equals the sentinel |
| Delta | First value absolute; missing in the middle (the next delta is relative to the last valid value); leading and trailing missing; one valid value only; negative deltas |
| Index ranges | Offsets 0, 1, 2, 10, 11 with laps (0–2) → [0, 3); (3–9) → [3, 3); (2–10) → [2, 4); (12–20) → [5, 5); (0–11) → [0, 5) |
| Metadata | Key set and order exactly as Format §5; totals rounded to 3 decimals; `-0` → `0`; nil totals omitted; `summary.moving_time` last; sessions and laps `[]` when empty; time zone unknown → `{"known":false,"offset_seconds":0}`; no key or value holds a coordinate (encode a workout with distinctive positions and check that the metadata JSON contains none of their digit strings) |
| Determinism | `Encode` twice → identical bytes |
| Encode errors | Zero samples; stream length ≠ *N*; offsets not increasing; offset < 0; lap with `End < Start`; session with `End < Start`; lap with `Start < 0` (v1.2) |
| Decode errors | One test per Format §6 rule: 13 bytes; one trailing byte after the last column (v1.1); (v1.2) `byte_offset` 4,294,967,292 and a `byte_offset` near 2⁶³ with a large *N* (error, no panic); streams out of §2 order; `latitude` without `longitude`; a sample where only one of the two is missing; **session** `start_index` > `end_index` and `end_index` > *N* (the lap case alone is not enough); lap `end < start`; bad magic; version 2; *N* = 0; *L* beyond the end; invalid JSON; missing key; empty `streams`; first stream not `time_offset`; unknown or repeated name; descriptor scale differs from §2; misaligned `byte_offset`; overlapping columns; column beyond the end; missing time sample; index range outside [0, *N*] |
| Round trip | 50 random workouts (fixed seed, random *N* 1–2,000, random gaps, about 10 % `NaN` per stream, random stream subsets): every valid value within scale / 2 + 1e-9, missing exactly where `NaN` |
| Fixture | `Fixture` of the §8 example: counts, statistics and digest computed by hand in the test |

**`internal/precompress`:** round trip for empty input, 1 byte, and 2 MB of pseudo-random bytes; round trip of a real `Encode` output.

**`internal/datafile/corpus_test.go`** (uses `ingest.Parse` and `corpustest.Files`):

| Test | Must cover |
| --- | --- |
| `TestRoundTripCorpus` | Every corpus file, synthetic and (if present locally) real: `Encode` → `Decode`. Header `WEXD`, version 2, *N* = number of samples (AC 1); every `byte_offset % 4 == 0` (AC 2); every value of every present stream within scale / 2 + 1e-9 of the workout's value, missing exactly where it is `NaN`, a strict tolerance so a clamp fails (AC 3); for `indoor-no-gps`, `indoor-no-distance-no-gps` and `pool-swim`, no `latitude` or `longitude` descriptor (AC 4); `Brotli` output decompresses to the identical bytes. Logs `real: N files (local only)` |
| `TestDataFileCorpus` | Golden `.bin` and `.json` per synthetic file, byte for byte, new and stale detection, `-update` (section Golden fixtures) |
| `TestSizeBudget4h` | Section Size budget (AC 5) |
| `BenchmarkEncode12h`, `BenchmarkBrotli12h` | `ride-12h.fit` parsed once outside the loop; `b.ReportAllocs()`; results go into `MEASUREMENTS.md` with the CPU named (that machine is the reference for the ADR-006 triggers) |

**`internal/ingest`:** `corpus_test.go` uses `corpustest.Files` instead of its own `corpusFiles`, with unchanged behaviour (`TestCorpusFilesEmptyReal` moves along with it).

**`cmd/workout-explorer/parse_test.go`:** `-datafile` writes both files; the `.br` decompresses to the `.bin`; the `.bin` decodes and its *N* matches the report's `samples.count`; stdout still equals the golden report; a non-existent folder → exit 1, `parse:` on stderr, empty stdout; `-datafile` without a file argument → exit 2.

**`tools/corpusgen`:** `TestSyntheticCorpusCurrent` covers `ride-4h.fit` automatically once it is in `corpus()`.

**Dependency check for QA:** `cd src/server && go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/... ./internal/...` lists only `workout-explorer/...`, `github.com/muktihari/fit/...` and `github.com/andybalholm/brotli/...`. Separately, `go list -deps ./cmd/...` does not list `workout-explorer/internal/corpustest`.

## Acceptance criteria to verification

| AC | Verified by |
| --- | --- |
| 1 Header `WEXD`, version, sample count for every corpus file | `TestRoundTripCorpus` (synthetic in CI; real on the operator's machine); `make corpus` writes a `.bin` for every file with no `FAIL` |
| 2 Column offsets divisible by 4 | `TestRoundTripCorpus`; unit test Alignment |
| 3 Round trip within resolution, sentinels where missing | `TestRoundTripCorpus`; unit tests Round trip, Delta, Quantisation |
| 4 Indoor: no latitude/longitude | `TestRoundTripCorpus` on the three files without positions; `testdata/datafile/synthetic/indoor-no-gps.json` has no position entries |
| 5 4-hour ride under 500 KB as `.br`; sizes recorded | `TestSizeBudget4h` in CI; `MEASUREMENTS.md` section for synthetic files; real-file sizes by manufacturer in the QA report |
| 6 Format description exists and matches the writer | This document, section "Data file format, version 2", which `Decode` implements; the golden files in `testdata/datafile/synthetic/`. **The cross-language check ("checked by the F-04 decoder tests") completes in F-04** (F-04 AC 2 reads these fixtures). F-03 QA verifies that the description, `Decode` and the fixtures agree, and does not block on F-04's tests |

## Open questions

None. No product decision is needed: every choice above is technical and follows the TD. OQ-8 is closed (F-03 AC 6 complete; history).
