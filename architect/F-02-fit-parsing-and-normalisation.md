# F-02 FIT parsing and normalisation: architecture

- **Version:** 1.11
- **Status:** accepted. Built. The F-08 design replaces some parts of this document (entry point, R-10 summary, report version 2, sport rule); each is marked where it appears, and the F-08 design is authoritative for it
- **Date:** 10 Oct 2026
- **History:** [architect/history/F-02-fit-parsing-and-normalisation.md](history/F-02-fit-parsing-and-normalisation.md) (change log, replaced rules as written, closed questions)
- **Feature:** [`features/F-02-fit-parsing-and-normalisation.md`](../features/F-02-fit-parsing-and-normalisation.md)
- **Depends on:** TD (system-wide picture), [ADR-001](ADR-001-repository-layout-and-build-entry-point.md), [ADR-003](ADR-003-fit-decoding-library.md), [ADR-004](ADR-004-parser-corpus-and-golden-reports.md), F-01 design v1.3
- **Requirements:** AP-2 (device totals), AP-4 (streams, only those present), DS-2 (all sports), AP-1 (start time in local time zone: data side), NFR Reliability; TD FIT parsing, TD Tech stack (FIT decoding), TD Hosting and operations (Local development: `make corpus`); review decisions 5, 12, 13, 33 (`features/README.md`)
- **Owner:** backend (senior-backend-engineer)

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | `muktihari/fit` v0.28.4, custom listener, broadcast-only; only `internal/ingest` imports it | ADR-003 |
| D-2 | Two packages: `ingest` (FIT anti-corruption layer, I/O) and `workout` (types and every normalisation rule, pure) | section Packages |
| D-3 | Time axis: whole seconds since `StartTime` (`[]int32`), strictly increasing; streams are `[]float64` with `NaN` for a missing sample and `nil` for an absent stream | section Normalised workout |
| D-4 | Gaps of 2 to 4 s are filled at 1 s by linear interpolation; gaps of 5 s or more stay gaps | section Rules, R-3 |
| D-5 | Moving time: from timer events if any, else speed threshold per sport; always computed, never taken from the device | R-7 |
| D-6 | Elevation gain: ±2 s centred moving average, then a 3 m zig-zag (hysteresis) filter; used only when the device recorded no ascent | R-8 |
| D-7 | Time zone: fixed UTC offset from `activity.local_timestamp − activity.timestamp`, rounded to 15 min; otherwise unknown (UTC) | R-9 |
| D-8 | Running, walking and hiking cadence converted to steps per minute (`2 × (cadence + fractional/128)`) | R-5 |
| D-9 | Corpus under `src/server/testdata/corpus/`, golden JSON reports, `TestCorpus` in `go test`, `make corpus` shows `diff -u` | ADR-004 |
| D-10 | Diagnostic subcommand `workout-explorer parse <file>` prints the golden report. It ships in the production binary on purpose (v1.3): the operator can diagnose a failing original on the server with exactly the parser that failed, it adds no dependency and only a few KB, and it reveals nothing that shell access does not already reveal (`original.fit` is on the same disk) | section `parse` subcommand |
| D-11 | Decoder component expansion **off** (`WithNoComponentExpansion`); enhanced-or-legacy fallback done in `ingest`; `compressed_speed_distance` not supported in the MVP (v1.1) | section Decoding, ADR-003 |

## Boundary with later features

F-02 delivers the happy path for well-formed files. These belong elsewhere and must **not** be built now:

| Not in F-02 | F-02 behaviour instead | Owner |
| --- | --- | --- |
| Rejecting non-activity files | `file_id.type` is read and exposed as `Workout.FileType`; nothing is rejected | F-08 |
| Ignoring a bad CRC, truncated-file recovery | Decoder defaults (CRC checked). Any decode error: `Parse` returns the error and no workout; partial data is discarded | F-08 |
| Chained FIT files | Only the first FIT sequence is decoded (one `DecodeWithContext` call); trailing bytes are not read | F-08 |
| Parse time and memory limits | `Parse` takes a `context.Context` and passes it to the decoder; no limits are set | F-08 (uses F-02 measurements) |
| Error categories and user-facing messages, logging | Errors are wrapped (`ingest: decode: %w`); `ingest` and `workout` do not log | F-08 |
| Summing totals over several sessions | With 2+ sessions the summary holds only derived values (R-10) | F-08 |
| Writing `data.bin`, quantising values, sample indices for laps | Values stay `float64`; laps and sessions carry time offsets | F-03 |
| Hiding start and end | none | F-16 |
| `length`, `device_info`, zones, developer fields, running dynamics | Ignored | phase 2 |

The workout sport rule is F-08's "Workout sports `S` and R-9 sport" (F-08 D-17, confirmed by the user, OQ-36): `multisport` only when the sessions and the record leg have two or more distinct sports; two sessions of one sport give that sport. It lives in `workout` (R-9) and is the only sport rule.

## Packages and files

Dependency direction: `cmd/workout-explorer → ingest → workout`; `ingest → github.com/muktihari/fit/...`; `workout → standard library only` (no `os`, no `io` readers, no logging).

```
src/server/
├── go.mod, go.sum                           + github.com/muktihari/fit v0.28.4
├── cmd/workout-explorer/
│   ├── main.go                              + "parse" in dispatch and usage
│   ├── parse.go                             parse subcommand
│   └── parse_test.go
├── internal/ingest/                         anti-corruption layer (ADR-003)
│   ├── ingest.go                            Parse
│   ├── listener.go                          decoder.MesgListener → workout.Recording
│   ├── convert.go                           field mapping, sentinels, semicircles, enhanced preference, enum names
│   ├── *_test.go                            unit tests
│   ├── corpus_test.go                       TestCorpus (+ -update flag), BenchmarkParse12h
├── internal/workout/                        pure domain
│   ├── workout.go                           Workout, Session, Lap, Totals, TimerEvent, Samples, TimeZone, Device, Derived, Summary
│   ├── recording.go                         Recording and Raw* input types
│   ├── normalise.go                         Normalise: the pipeline (R-1 … R-10)
│   ├── timeaxis.go, gaps.go, cadence.go, distance.go, moving.go, elevation.go, timezone.go, summary.go
│   ├── report.go                            Report, ReportJSON (golden format)
│   └── *_test.go
├── scripts/corpus.sh                        make corpus driver (POSIX sh)
├── tools/corpusgen/main.go                  dev-only synthetic file generator (not in the binary)
└── testdata/corpus/                         ADR-004 layout
```

Public surface (nothing else is exported beyond the types below and their fields):

```go
// package ingest (entry point: ADR-017 and the F-08 design)
func NewParser(lim Limits, log *slog.Logger) *Parser
func (p *Parser) Parse(ctx context.Context, data []byte) (workout.Workout, error)

// package workout
func Normalise(rec Recording) Workout          // never panics, deterministic
func Report(w Workout) ReportV1
func ReportJSON(w Workout) ([]byte, error)     // indented 2 spaces, trailing newline
```

`Parse` = decode into a `Recording` (listener) + `workout.Normalise`. The golden report is version 2 (F-08 design, section Report).

## Decoding (ingest)

Decoder: `decoder.New(r, decoder.WithMesgListener(l), decoder.WithBroadcastOnly(), decoder.WithNoComponentExpansion())`, then one `DecodeWithContext(ctx)`. These three options are the complete set; do not add others in F-02.

**Why expansion is off (v1.1, D-11).** In v0.28.4, `expandComponents` (`decoder/decoder.go`, lines 813–835) writes an expanded component into an existing field with the same number. So `altitude` expands into `enhanced_altitude` and **replaces** a recorded `enhanced_altitude`; the same happens with `speed` and `enhanced_speed`. The legacy value then wins, which breaks AC 5. With expansion off, each field keeps exactly what the device wrote, and the enhanced-or-legacy choice below is made by our code only. What we give up: fields that exist only as components of another field. Of the fields we read, that is only `record.compressed_speed_distance` (field 8), which only very old devices write. It is **not supported in the MVP**: such records have no device speed or distance. Distance is then computed from positions (R-6b) if there are any. If a real file ever needs it, `ingest` will expand field 8 itself, with a new design entry. Accumulated fields are not affected, because the library only accumulates through components.

The listener's `OnMesg` switches on `mesg.Num` and converts with `mesgdef.New<Mesg>(&mesg)` before returning (the message is only valid during the call). Every other message number, every unknown field and every developer field is ignored without error.

| Message | Fields read | Into |
| --- | --- | --- |
| `file_id` | `type`, `manufacturer`, `product`, `product_name` | `FileType`, `Device`. **Not** `serial_number` (store as little as possible) |
| `activity` | `timestamp`, `local_timestamp` (raw `uint32` seconds since the FIT epoch, both) | `Recording.LocalOffset` |
| `session` | `sport`, `sub_sport`, `start_time`, `timestamp`, totals (table below) | `RawSession`, arrival order |
| `lap` | `start_time`, `timestamp`, totals | `RawLap`, arrival order |
| `record` | `timestamp`, `position_lat`, `position_long`, `altitude`, `enhanced_altitude`, `distance`, `speed`, `enhanced_speed`, `heart_rate`, `cadence`, `fractional_cadence`, `power`, `temperature` | `RawRecord`; records without a valid timestamp are dropped |
| `event` | `timestamp`, `event`, `event_type`; only `event = timer` | `RawTimerEvent`: `event_type` `start` → start; `stop`, `stop_all`, `stop_disable`, `stop_disable_all` → stop; other types dropped |

**Conversions** (in `convert.go`, each a small tested function):

| Rule | Exactly |
| --- | --- |
| Invalid values | Compare the **raw** field with the profile's invalid sentinel (`basetype.*Invalid`); invalid → `NaN` (numbers) or zero `time.Time` (timestamps). Do not rely on the library's scaled helpers for validity |
| Semicircles | `deg = float64(s) * (180.0 / 2147483648.0)`. 2³⁰ → exactly 90.0. A position is valid only if both lat and lon are valid, `|lat| ≤ 90`, `|lon| ≤ 180`, and not exactly (0, 0); otherwise both are `NaN` |
| Altitude | Per sample: `enhanced_altitude` if valid, else `altitude`; metres = raw / 5 − 500 |
| Speed | Per sample: `enhanced_speed` if valid, else `speed`; m/s = raw / 1000 |
| Distance | m = raw / 100 |
| Cadence | `cadence + fractional_cadence / 128` (fraction 0 if invalid); cadence invalid → `NaN`. Unit stays the FIT unit here; sport conversion is R-5 |
| Heart rate, power, temperature | bpm, W, °C as raw |
| Totals | Scale per FIT profile to the units in section Units; `enhanced_avg_speed` / `enhanced_max_speed` if valid, else `avg_speed` / `max_speed` (the fallback is ours, since expansion is off); `avg_cadence + avg_fractional_cadence/128`, same for max |
| Session/lap end | `timestamp` if valid, else `start_time + total_elapsed_time`, else zero time |
| Session/lap start (v1.1) | `start_time` if valid, else `end − total_elapsed_time` if both are valid, else zero time |
| Enum names (`sport`, `sub_sport`, `manufacturer`, `file_id.type`) | The FIT profile's snake_case name (`cycling`, `e_biking`, `indoor_cycling`, `garmin`, `activity`). Take it from the library's `String()`; a value that is not in the profile becomes `unknown_<n>`. The invalid sentinel (field not recorded) becomes `""` (E-1). A unit test pins `String()` of `typedef.SportCycling` to `cycling`; if the library's format differs, map in `convert.go` |
| Local offset | `int64(local_timestamp) − int64(timestamp)` seconds, `nil` if no `activity` message or either value invalid. Last `activity` message wins if there are several |

### Edge rules (v1.1, settle the engineer's assumptions)

| # | Rule | Status |
| --- | --- | --- |
| E-1 | An enum field holding its invalid sentinel maps to `""` (not recorded). A `""` sport counts as "every other sport" in R-5 and R-7. With one session, `Workout.Sport` is then `""` | Confirmed |
| E-2 | Session and lap times: start and end are first completed in `ingest` with the start and end rows of the conversion table. Then, in `workout`: a start that is still missing maps to offset 0. An end that is missing or earlier than the start maps to the start offset. Any negative offset (only possible for timer events, R-1) is clamped to 0 | Confirmed, with one addition: the start fallback `end − total_elapsed_time` |
| E-3 | Timer events without a valid timestamp are dropped | Confirmed |
| E-4 | Only the first `file_id` message is read; later ones are ignored (chained files are F-08) | Confirmed. **v1.8:** refined by F-08 D-15: only the first `file_id` of the first FIT sequence is read for `FileType` and `Device` |
| E-5 | R-5 "the last session that started before it" = the session with the latest `Start` that is earlier than the offset; on equal starts, the first in file order | Confirmed, tie rule added |
| E-6 | `offsets_digest` formats each offset as an integer (`strconv.FormatInt`). Offsets are whole seconds, so decimals carry no information | **Corrected** (integers) |
| E-7 | Always written in the report, even when empty or zero: `time_zone`; `samples.count`, `samples.filled`, `samples.gaps` (all three of its fields); `sessions`, `laps`, `timer_events` as `[]`; `device`, `derived`, `summary`, `streams` as objects (may be `{}`; `device` confirmed in v1.2). Only scalar values that are absent (`nil`) are omitted | Confirmed and extended to arrays and objects; adjust if the code omits these |
| E-8 | **Device system time (v1.3, QA r1 Q2).** The FIT profile defines a `date_time` or `local_date_time` value below `0x10000000` as seconds since the device powered on, not a calendar time. `ingest` treats every such value as **invalid**, exactly like the `0xFFFFFFFF` sentinel. This happens in one function (`validDateTime`) used for every timestamp `ingest` reads: record `timestamp`, event `timestamp`, session and lap `start_time` and `timestamp`, activity `timestamp` and `local_timestamp`. The existing rules then apply: such records and timer events are dropped (R-2 input, E-3), session and lap times fall back (E-2), and the time zone becomes unknown (R-9). We do not convert system time through `timestamp_correlation` in the MVP. If every record has system time, the workout is empty, and rejecting it is F-08's job | Decided. Owner F-02: correctly reading the FIT time type is part of the anti-corruption layer, not validation. No product decision needed |

## Recording (ingest → workout)

Plain values only; `time.Time` in UTC, zero value = missing; `NaN` = missing number.

```go
type Recording struct {
    FileType    string
    Device      Device
    LocalOffset *int64          // seconds; validated in R-9
    Sessions    []RawSession    // arrival order
    Laps        []RawLap        // arrival order
    Timer       []RawTimerEvent // arrival order
    Records     []RawRecord     // arrival order, not yet sorted
}
type RawSession struct { Sport, SubSport string; Start, End time.Time; Totals Totals }
type RawLap     struct { Start, End time.Time; Totals Totals }
type RawTimerEvent struct { At time.Time; Kind TimerKind } // TimerStart | TimerStop
type RawRecord struct {
    At time.Time
    Lat, Lon, Altitude, Distance, Speed, HeartRate, Cadence, Power, Temperature float64
}
```

*(v1.7/v1.8: F-08 adds `Integrity`, `Sports`, `SequenceStarts` and `RawLap.Sport`; see the F-08 design, Model changes.)*

## Normalised workout

```go
type Workout struct {
    FileType  string
    Device    Device      // Manufacturer string; ProductID uint16 (0 = absent); ProductName string
    Sport     string      // R-9
    StartTime time.Time   // UTC; t = 0 of the time axis (R-1)
    TimeZone  TimeZone    // R-9
    Sessions  []Session   // file order
    Laps      []Lap       // file order
    Timer     []TimerEvent
    Samples   Samples
    Derived   Derived
    Summary   Summary
}
type TimeZone   struct { Known bool; OffsetSeconds int32 }   // Known=false: show UTC, labelled (decision 13)
type Session    struct { Sport, SubSport string; Start, End int32; Totals Totals } // offsets in s
type Lap        struct { Start, End int32; Totals Totals }
type TimerEvent struct { Offset int32; Kind TimerKind }
type Totals struct {  // device values as recorded; nil = not recorded
    ElapsedTime, TimerTime, Distance, Ascent, Descent, Calories,
    AvgSpeed, MaxSpeed, AvgHeartRate, MaxHeartRate, AvgCadence, MaxCadence,
    AvgPower, MaxPower, AvgTemperature, MaxTemperature *float64
}
type Samples struct {
    Offset []int32   // seconds since StartTime
    Lat, Lon, Altitude, Distance, Speed, HeartRate, Cadence, Power, Temperature []float64
    Filled int       // samples inserted by R-3
}
type Derived struct { // computed from the samples
    ElapsedTime      *float64 // last offset; nil if no samples
    MovingTime       *float64 // R-7
    MovingTimeSource string   // "timer" | "speed" | "" (nil)
    ElevationGain    *float64 // R-8
    Distance         *float64 // last valid value of the distance stream
}
type Summary struct { Totals; MovingTime *float64 } // R-10
```

**Invariants** (checked by `Normalise` tests on every corpus file, `TestInvariants`):

1. Every present stream has `len == len(Offset)`; `Offset` is strictly increasing and `Offset[0] ≥ 0`.
2. A stream is `nil` if and only if it has no valid value. A present stream contains at least one non-`NaN` value; no `±Inf` anywhere.
3. `Lat` and `Lon` are both `nil` or both present; at every index both are `NaN` or both are valid.
4. Missing totals are `nil`, never `0` (AP-2). Absent streams are `nil`, never zeros (AP-4, F-02 AC 8).
5. `Normalise` never panics, including on an empty `Recording`.

**Units** (the data file in F-03 quantises from these):

| Quantity | Unit |
| --- | --- |
| Time offsets, elapsed, timer, moving time | s |
| Latitude, longitude | degrees (WGS 84) |
| Altitude, distance, ascent, descent, elevation gain | m |
| Speed | m/s |
| Heart rate | bpm |
| Cadence | per minute: steps for running, walking, hiking (R-5); revolutions or strokes otherwise |
| Power | W |
| Temperature | °C |
| Calories | kcal |

## Rules (`workout.Normalise`, in this order)

**R-1 Start time.** `StartTime` = earliest valid of: first session `Start`, first lap `Start`, earliest record `At`. No valid time at all: zero `StartTime`, empty workout. Session, lap and timer offsets = whole seconds since `StartTime`; timer events earlier than `StartTime` get offset 0. *(v1.9, F-08 D-19: this is the one time axis of the workout, across every session and FIT sequence; the cursor's elapsed time is the sample's offset, and no rule rebases offsets per session or sequence. The summary's elapsed time of a multi-leg workout is `Derived.ElapsedTime` on this axis, F-08 D-18.)*

**R-2 Time axis.** Sort records stably by `At`. Records with the same second are merged into one sample: per stream, the later record's valid value wins. Offsets are `int32` seconds.

**R-3 Gap filling (smart recording).** For consecutive samples with `d = Offset[i+1] − Offset[i]`: if `2 ≤ d ≤ 4`, insert `d − 1` samples at 1 s steps. Each inserted value is the linear interpolation between the two neighbours for that stream (lat and lon interpolated independently), or `NaN` if either neighbour is `NaN` for that stream. `d ≥ 5` stays a gap (no samples). Count the inserted samples in `Samples.Filled`.

**R-4 Stream presence.** After R-3, a stream with no valid value becomes `nil`. *(v1.10: F-08's R-13, transition legs, runs between R-3 and R-4, so a stream whose only values were in transitions becomes `nil` here.)*

**R-5 Cadence by sport.** For samples belonging to a session whose sport is `running`, `walking` or `hiking`, cadence × 2 (steps per minute; the FIT value counts one leg). The same applies to that session's and its laps' `AvgCadence`/`MaxCadence`. Which sport a sample or lap has comes from the F-08 design's sport timeline: the covering session first, then sport hints for samples no session covers, then the session fallback. The same timeline serves R-7.

**R-6 Distance.**
- (a) The device's distance stream is kept as recorded, `NaN` where missing. *(v1.11: then made non-decreasing: each valid value becomes the largest valid distance so far; F-08 R-14, D-25.)*
- (b) If there is no distance stream and positions are present: compute it by haversine, `R = 6,371,008.8 m`, between consecutive positioned samples, cumulative. Samples before the first fix get 0; samples without a position after it carry the previous value; after a dropout the straight-line distance from the last to the next fix is added (consistent with AP-3's straight segments). Device totals are unaffected.
- (c) No distance and no positions: no distance stream (F-02 AC 8).
- `Derived.Distance` = last valid value of the distance stream, `nil` without one.

**R-7 Moving time.**
- **Timer events present** (at least one): walk them in offset order with state *running*/*stopped*, starting *stopped*. Start while stopped opens an interval; stop while running closes it; repeated starts or stops are ignored. If the first event is a stop, an interval `[0, stop)` is assumed. An interval still open at the end closes at the last sample offset (contributes 0 if that is earlier). Moving time = sum of intervals. Source `timer`. *(v1.10: moving time is the length of the union of these intervals and the transition intervals `(Offset[i−1], Offset[i]]` of every transition sample `i ≥ 1` with `Δt < 5`; F-08 R-13.M.)*
- **Otherwise**, from speed: per sample `i ≥ 1` with `Δt = Offset[i] − Offset[i−1] < 5` (gaps count as stopped), add `Δt` if `speed[i] ≥ threshold`. Speed = the speed stream; if absent, `Δdistance/Δt` from the distance stream (including one computed in R-6b). Threshold by the sample's session sport (same membership rule as R-5): `cycling`, `e_biking` 1 km/h (0.27778 m/s); every other sport, and no session, 0.5 km/h (0.13889 m/s) (TD names cycling and running only; the default for other sports is OQ-4). `NaN` speed counts as not moving. Source `speed`. *(v1.10: except at a transition sample, which counts as moving whenever `Δt < 5`; F-08 R-13.M.)*
- **Neither timer events nor a speed source:** `nil`.

**R-8 Elevation gain** (TD: smoothing plus 2–3 m hysteresis; one parameter set for barometric and GPS devices, because the device's own ascent is used when it exists, R-10).
1. Smooth: for each sample with valid altitude, the mean of valid altitudes with `|Offset[j] − Offset[i]| ≤ 2` (window shrinks at edges and gaps).
2. Zig-zag filter with `H = 3 m` over the smoothed valid values `a₀, a₁, …`:
   ```
   lo, hi := a0, a0; dir := 0; gain := 0
   for each a:
     dir 0:  lo = min(lo,a); hi = max(hi,a)
             if a - lo >= H { dir = +1; ref = lo; cand = a }
             else if hi - a >= H { dir = -1; ref = hi; cand = a }
     dir +1: if a > cand { cand = a } else if cand - a >= H { gain += cand - ref; ref = cand; cand = a; dir = -1 }
     dir -1: if a < cand { cand = a } else if a - cand >= H { ref = cand; cand = a; dir = +1 }
   end: if dir == +1 { gain += cand - ref }
   ```
3. Fewer than 2 valid altitudes: `nil`.

**R-9 Time zone and sport.**
- Time zone (decision 13): `LocalOffset` rounded to the nearest 900 s; valid if within −12 h to +14 h → `Known=true`. Missing or out of range → `Known=false, OffsetSeconds=0` (UTC). The offset is fixed for the whole workout; no IANA zone, no lookup from coordinates (no external services).
- Sport: the F-08 design, "Workout sports `S` and R-9 sport": the distinct sports of the sessions and of the record leg; one → it, two or more → `multisport` (confirmed by the user, OQ-36). Pool swims need nothing special (decision 33): no position stream, other streams as present.

**R-10 Summary.** The F-08 design, section Summary (R-10 revised, with R-11 record totals).

### Missing data at a glance

| Situation | Result |
| --- | --- |
| Indoor, no GPS | `Lat`/`Lon` `nil`; distance from the device if recorded, otherwise `nil` |
| GPS dropout | `Lat`/`Lon` `NaN` over the dropout; device distance unchanged; computed distance carries forward |
| Pause of hours | A gap in `Offset` (no samples); moving time excludes it (timer or gap rule) |
| Smart recording | 2–4 s gaps filled, 5 s+ gaps kept |
| Stream with some missing samples | `NaN` at those indices |
| Device total not recorded | `nil` in `Totals`; summary fallback only for elapsed, distance, ascent |
| No records at all | Empty `Samples`, no streams, `Derived` all `nil`; no error (rejection is F-08) |

## `parse` subcommand

`workout-explorer parse <file.fit>`: opens the file, `ingest.Parse(context.Background(), f)`, writes `workout.ReportJSON` to stdout.

| Situation | Exit |
| --- | --- |
| Success | 0, report on stdout |
| Missing or extra arguments, bad flag | 2, usage on stderr |
| Cannot open file, parse error | 1, one line `parse: <error>` on stderr |

Diagnostic tool: it prints coordinates (statistics) to **stdout** for the person running it; it never logs and is never run by the service (I-5). It ships in the production binary (D-10 gives the reason). The feature file calls it "development-only"; that wording is being changed to "diagnostic" through requirements. F-03 extends it to write `data.bin`. Add it to the usage text as `parse    print the normalised workout of a FIT file (diagnostic)`.

## Golden report (`ReportV1`)

JSON object, keys in this order; absent values omitted (`omitempty`); numbers written as fixed-decimal strings turned into JSON numbers (`json.Number`), `-0` written as `0`.

| Key | Content |
| --- | --- |
| `report_version` | `1`. Bump when the shape changes |
| `file_type`, `device`, `sport`, `start_time` (RFC 3339, UTC), `time_zone` | As in `Workout` |
| `sessions`, `laps` | Each with `start`, `end` (s), sport fields (sessions), `totals` |
| `timer_events` | `[{"offset": 0, "kind": "start"}, …]` |
| `samples` | `count`, `last_offset`, `filled`, `gaps` (`count`, `total_seconds`, `longest_seconds` for gaps ≥ 5 s), `offsets_digest` |
| `streams` | One object per present stream, in the order `latitude, longitude, altitude, distance, speed, heart_rate, cadence, power, temperature`: `present`, `missing`, `min`, `max`, `first`, `last`, `mean` (over valid values), `digest` |
| `derived`, `summary` | As in `Workout`; totals keys `elapsed_time, timer_time, distance, ascent, descent, calories, avg_speed, max_speed, avg_heart_rate, max_heart_rate, avg_cadence, max_cadence, avg_power, max_power, avg_temperature, max_temperature`, plus `moving_time` |

Decimals: latitude/longitude 7, speed 3, altitude/distance 2, heart rate/cadence/power/temperature 2, times and totals 3. **Digest:** SHA-256 over every value of the stream formatted with those decimals (offsets as integers, E-6) (`strconv.FormatFloat(v, 'f', d, 64)`, `-` for `NaN`), each followed by `\n`; lower-case hex, first 16 characters.

## Corpus

Layout and mechanism: ADR-004. `README.md` in the corpus folder has one row per **synthetic** file: path, what it covers (AC number), privacy treatment ("synthetic, no personal data"). Names: lower-case kebab-case.

**Real files (v1.4, ADR-004 rev. 2 item 7): local only, never committed.** The README has no per-file rows for them, because a committed row would point at a file other checkouts do not have, and would describe someone's workout. Instead it has a section **"Real files (local only)"** containing:

1. **Rule:** real FIT files and their golden reports are git-ignored (user decision, 10 Oct 2026). Never force-add them (`git add -f`).
2. **Adding a file:** copy it to `real/<brand>-<model>-<what>.fit` in lower case. `<brand>` is the **manufacturer that wrote the FIT file**, as `parse` reports it in `device.manufacturer`, not the watch maker. For example, an Apple Watch file written by a third-party app counts under that app's manufacturer value. Then run `make corpus-update`, review `expected/real/<name>.json`, and run `make corpus`.
3. **Checking:** before accepting a parser change, the operator runs `make corpus` on the machine that holds the real files. The count line shows how many real files were checked.
4. **Privacy row:** "Real files: route, times and the device serial number. Kept only on the operator's machine. Their reports contain start times, totals and coordinate extremes, so they are local only too."

**The two files received so far** (Apple Watch `Watch7,5`, via a third-party app, one run and one ride) go to `real/` under that rule. Name them from `parse`'s `device.manufacturer`, e.g. `<manufacturer>-watch7-5-run.fit` and `<manufacturer>-watch7-5-ride.fit`. Together they count as **one brand** for AC 2.

**Required files** (AC numbers follow the feature file as reworded on 10 Oct 2026, v1.5). Every `synthetic/` file is required and committed (AC 1). Real files are additional and local only (AC 2).

| File | Kind | Covers |
| --- | --- | --- |
| `real/<brand>-<model>-<what>.fit`, three different FIT writers | real, **local only** (v1.4) | AC 2, AC 10 (unknown fields from real devices) |
| `synthetic/indoor-no-gps.fit` | synthetic | AC 1: indoor, HR, power, cadence, device distance; no positions |
| `synthetic/indoor-no-distance-no-gps.fit` | synthetic | AC 8: no distance stream |
| `synthetic/gps-dropout.fit` | synthetic | AC 1: 5 min without positions mid-ride, device distance present |
| `synthetic/pause-hours.fit` | synthetic | AC 1: timer stop, 3 h without records, timer start |
| `synthetic/brick-run-bike.fit` | synthetic | AC 1, AC 11: 2+ sessions in order, running cadence R-5, sport `multisport` |
| `synthetic/smart-recording.fit` | synthetic | AC 1, AC 9: gaps of 3 s, 4 s, 5 s, 30 s |
| `synthetic/ride-12h.fit` | synthetic | 43,200 records at 1 Hz, every stream, laps every 30 min; AC 1, AC 12 |
| `synthetic/enhanced-and-legacy.fit` | synthetic | AC 5, three segments of 60 records each (v1.1): (1) both fields, different values (`altitude` 410 m / `enhanced_altitude` 420 m, `speed` 6.999 / `enhanced_speed` 7.999 m/s) → enhanced is used; (2) only the legacy fields → legacy is used; (3) only the enhanced fields → enhanced is used. Session: both `avg_speed` and `enhanced_avg_speed`, different values |
| `synthetic/timer-pause.fit` | synthetic | AC 6: timer stop/start around a 10 min stop. Records **continue at 2 m/s** during the stop, so the golden moving time differs from the speed rule and proves the timer source is used (v1.1, confirmed) |
| `synthetic/rolling-hills.fit` | synthetic | R-8 observable (v1.1). 1 Hz, no `total_ascent` in the session. A 100 m base. Each bump has a 20 s linear rise, a 10 s flat top, a 20 s linear fall and 30 s flat at the base, so the smoothing window never cuts a peak or a valley. First 20 bumps of 2.2 m (below `H = 3`, above 2), then 10 bumps of 6 m. Uniform ±0.5 m noise with a fixed seed. Without noise the shape gives exactly 60 m (the 6 m bumps only); `H = 2` adds about 44 m. With noise the filter adds about 0.35 m per bump (it keeps the smoothed noise extreme at each top and valley), so the golden value is about 63 to 64 m. Purpose in the corpus: a change of `H` or of the smoothing shows up as a diff. The exact assertions are in the R-8 tests (v1.2) |
| `synthetic/cycling-no-timer-slow.fit` | synthetic | AC 6: no timer events, stretches below 1 km/h |
| `synthetic/flat-noise.fit` | synthetic | AC 7: 3,600 s flat at 100 m, uniform ±1 m noise (fixed seed), no `total_ascent` in the session |
| `synthetic/climb-100m.fit` | synthetic | AC 7: 60 s flat, 100 m climb over 1,000 s, 60 s flat; no `total_ascent` |
| `synthetic/no-distance-with-gps.fit` | synthetic | AC 8: positions, no distance field |
| `synthetic/unknown-messages.fit` | synthetic | AC 10: a manufacturer-specific message (number ≥ 0xFF00), an unknown field number in `record`, a developer field |
| `synthetic/pool-swim.fit` | synthetic | Decision 33: no positions, `length` messages ignored |

`tools/corpusgen` writes every synthetic file from fixed parameters (`-out testdata/corpus/synthetic`), deterministic byte for byte. It uses the library's encoder; this is the only production-external import of the library (ADR-003).

**Added in v1.3:**

| File | Kind | Covers |
| --- | --- | --- |
| `synthetic/system-time-record.fit` | synthetic | E-8: 600 records at 1 Hz on a 2026 date. In the middle, one extra record with `timestamp` = 1,000 (system time) and one timer stop event with `timestamp` = 2,000. Golden report: 600 samples, start time on the 2026 date, last offset 599, the bad timer event absent |

**Generator check (v1.3, ADR-004 item 5).** `corpusgen` exposes `generate(outDir string) error`; `main` only parses `-out` and calls it. `tools/corpusgen/main_test.go` has `TestSyntheticCorpusCurrent`: it generates into `t.TempDir()` and compares with `testdata/corpus/synthetic/`. Every generated file must be byte-identical to the committed one. A committed `*.fit` that is not generated, or a generated file that is not committed, also fails. The failure message names the file and says `run make corpus-gen, then make corpus-update`. It runs in `go test ./...`, so in `make test` and CI.

**File names (v1.3, ADR-004 item 4).** Both drivers (`corpus.sh` and `TestCorpus`) list every entry in `real/` and `synthetic/`. Hidden files (`.gitkeep`) are skipped. Every other entry that does not end in lower-case `.fit` (`GARMIN01.FIT`, `x.fit.bak`, a subdirectory) is reported: `corpus.sh` prints `UNKNOWN <path>` and counts it in the final line, and `TestCorpus` fails with `rename to <brand>-<model>-<what>.fit`. Either one fails the run. Upper-case `.FIT` is not matched case-insensitively, on purpose: the naming rule (lower-case kebab-case) exists so that every file in the manifest is identifiable.

**Measurements** (F-02 AC 12): `BenchmarkParse12h` (reads the file into memory once, `b.ReportAllocs()`, measures `ingest.Parse` only) and the peak RSS of `build/workout-explorer parse testdata/corpus/synthetic/ride-12h.fit > /dev/null` under `/usr/bin/time -v`. Record in `testdata/corpus/MEASUREMENTS.md`: date, commit, CPU, Go version, records, ns/op, B/op, allocs/op, max RSS. No target in F-02; F-08 sets limits from these numbers.

## Build and CI changes

| Item | Change |
| --- | --- |
| `go.mod` / `go.sum` | `require github.com/muktihari/fit v0.28.4`; `go.sum` committed |
| `make corpus` | Replace the placeholder: depends on `build-server`; runs `sh src/server/scripts/corpus.sh build/workout-explorer src/server/testdata/corpus build/corpus`. The script runs `parse` on every `*.fit` under `real/` and `synthetic/` (sorted), writes `build/corpus/<rel>.json`, prints `ok`, `DIFF` (followed by `diff -u expected actual`), `FAIL` (parse exit ≠ 0, with stderr), `NEW` (no expected file) or `STALE` (expected file without FIT file) per file, and a final count line. Exit 1 if any line is not `ok` |
| `make corpus-update` | `cd src/server && go test ./internal/ingest -run '^TestCorpus$' -update`: rewrites all expected reports and removes stale ones; fails if any file does not parse |
| `make corpus-gen` | `cd src/server && go run ./tools/corpusgen -out testdata/corpus/synthetic` |
| `.PHONY` | add `corpus-update corpus-gen` |
| `.github/workflows/ci.yml` | `setup-go`: `cache: true`, `cache-dependency-path: src/server/go.sum` (planned in F-01 design, section CI, Step 2). Nothing else |
| README | Add the two new targets to the target table (F-01 README rules apply) |

Dependency check for QA: `cd src/server && go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/... ./internal/...` lists only `workout-explorer/...` and `github.com/muktihari/fit/...`.

## Tests

**`internal/workout`** (pure, in-memory `Recording`s built in the test; no files):

| Area | Must cover |
| --- | --- |
| R-1, R-2 | Unsorted records are sorted; two records in the same second merge (later valid value wins, earlier fills a `NaN`); `StartTime` is the earliest of session, lap, record; offsets ≥ 0; timer event before start → 0 |
| R-3 | 1 s steps untouched; gap 3 s → 2 inserted samples with exact interpolated values; gap 4 s filled; gap 5 s and 30 s untouched; `NaN` neighbour → `NaN` in that stream only; `Filled` count |
| R-4 / invariants | A stream with only `NaN` becomes `nil`; lat valid but lon `NaN` → both `NaN`; empty `Recording` → no panic, no streams, `Derived` all `nil` |
| R-5 | Running 85 + 64/128 → 171 spm; cycling 90 → 90; session and lap averages converted for running only; sample outside sessions uses the rule |
| R-6 | Haversine (0°, 0°)–(1°, 0°) = 111,195.08 m ± 0.01; computed distance 0 before first fix, carried through a dropout, then increased by the straight line; no distance and no positions → `nil` stream |
| R-7 | Start, stop at 600 s, start at 1,200 s, stop at 1,800 s → 1,200 s; stop first; open interval at end; repeated start; no timer events, cycling, samples at 0.2 m/s not counted and 0.3 m/s counted; running threshold 0.5 km/h; **thresholds pinned (v1.3):** for `cycling` and `e_biking`, samples at 0.99 km/h are not counted and samples at 1.01 km/h are counted; for `running`, `walking` and `""`, 0.49 km/h is not counted and 0.51 km/h is counted; a sample exactly at the threshold (the same `km/h / 3.6` expression as the constant) is counted (`>=`); no speed stream → from distance; neither → `nil`; gap ≥ 5 s not counted |
| R-8 | 3,600 samples flat with uniform ±1 m noise (fixed seed) → 0 m; clean climb 100 m (shape as `climb-100m.fit`) → 100 ± 2 m; climb 50, descend 50, climb 50 → 100 ± 2 m; rolling-hills shape (as `rolling-hills.fit`, v1.2), with `H` passed to the unexported filter function and smoothing applied as in production. **Noise-free:** `H = 3` → 60 ± 0.01 m; `H = 2` → more than 100 m. **With ±0.5 m noise, for each seed 1 to 10:** `H = 3` → 60 m ≤ gain ≤ 66 m; `H = 2` → more than 100 m. The noise band is one-sided on purpose: the filter may add a little noise per bump, but it must never count a 2.2 m bump at `H = 3` (that would add about 44 m). Remove the `t.Skip`; `NaN` altitudes skipped; one valid altitude → `nil` |
| R-9 | Offset 7,200 → +7,200 known; 7,190 → 7,200; **:45 zones (v1.3):** 20,700 → 20,700 (Nepal +05:45), 20,690 → 20,700, 20,500 → 20,700, 45,900 → 45,900 (Chatham +12:45); 20 h → unknown; `nil` → unknown; sport: 1, 2, 0 sessions |
| R-10 | Device value wins over derived; fallbacks for elapsed, distance, ascent; moving time always derived; 2 sessions → only derived values |
| Report | Same workout twice → identical bytes; `NaN` digested as `-`; absent streams omitted; `-0` written as `0` |

**`internal/ingest`**:

| Area | Must cover |
| --- | --- |
| `convert.go` | Semicircles 2³⁰ → 90.0 exactly, −2³⁰ → −90.0, sentinel → `NaN`, (0, 0) → missing, lat 2³¹−2 (> 90°) → missing; enhanced altitude preferred per sample, legacy used when enhanced invalid (same for speed); altitude scale/offset (raw 2,500 → 0 m); cadence fraction; enum name `cycling`, unknown → `unknown_<n>`; local offset `nil` cases; **E-8 (v1.3):** `validDateTime` with `0x0FFFFFFF` → invalid, `0x10000000` → valid, `0xFFFFFFFF` → invalid |
| E-8 through `Parse` (v1.3) | Encoded bytes: 600 records in 2026 plus one record at system time 1,000 → `StartTime` on the 2026 date, 600 samples, `Derived.ElapsedTime` 599 s. A session whose `start_time` is system time falls back through E-2. An activity whose `local_timestamp` is system time → time zone unknown |
| Listener | Built with the library encoder in the test: unknown message number, unknown field, developer field → ignored, no error; event types mapped; `serial_number` not read; second `file_id` ignored (E-4); timer event without timestamp dropped (E-3); enum sentinel → `""` (E-1) |
| Decoding of both field variants (v1.1) | Through `Parse` on bytes from the encoder: a record with `altitude` 3000 raw and `enhanced_altitude` 4000 raw gives (4000 / 5 − 500) = 300 m, not 100 m; a record with only legacy `speed` gives that speed, which proves the fallback works with expansion off. Pins the option set: if expansion is turned back on, this test fails |
| Session/lap times (E-2) | Start missing but end and elapsed time present → `end − elapsed`; everything missing → offset 0; end before start → start offset |
| `Parse` | Cancelled context → error; non-FIT bytes → error (no category; F-08) |
| `TestCorpus` | Every corpus file parses; report equals `expected/` byte for byte; invariants 1–5 hold; `-update` rewrites |
| `BenchmarkParse12h` | As section Measurements |

**`cmd/workout-explorer/parse_test.go`**: no argument → 2; two arguments → 2; missing file → 1 with `parse:` on stderr; a corpus file → 0 and stdout equals its expected report.

## Acceptance criteria to verification

| AC | Verified by |
| --- | --- |
| 1 Synthetic corpus (in CI) | Corpus `README.md` rows; files from the Required files table; `TestCorpus` and `TestSyntheticCorpusCurrent` in CI |
| 2 Real corpus (operator's machine) | On the operator's machine, `make corpus` shows `real: N files` with no failures, and `parse` gives at least three distinct `device.manufacturer` values across `real/`. QA either runs this on that machine or records the operator's output, listing the manufacturers in the QA report, but not routes or times |
| 3 `make corpus` | Run it clean (all `ok`); change a constant (e.g. `H`) locally, see `DIFF` with `diff -u` output, revert |
| 4 Semicircles | `convert.go` test |
| 5 Enhanced fields | `convert.go` test; decoding test (v1.1); `enhanced-and-legacy.fit` report at **file level**: segment 1 shows 420 m and 7.999 m/s |
| 6 Moving time | R-7 tests; `timer-pause.fit`, `cycling-no-timer-slow.fit` reports |
| 7 Elevation gain | R-8 tests; `flat-noise.fit` (0 m), `climb-100m.fit` (100 ± 2 m) reports; `rolling-hills.fit` makes a change of `H` or of the smoothing show up in `make corpus` |
| 8 Distance | R-6 tests; `no-distance-with-gps.fit`, `indoor-no-distance-no-gps.fit` (no `distance` key in `streams`) |
| 9 Smart recording | R-3 tests; `smart-recording.fit` report (`filled`, `gaps`) |
| 10 Unknown messages | Listener test; `unknown-messages.fit` parses |
| 11 Multi-session | `brick-run-bike.fit` report lists every session in order |
| 12 Parse time and memory | `MEASUREMENTS.md` row |

## Open questions

| ID | Question | Blocks | Owner |
| --- | --- | --- | --- |
| OQ-4 | Moving-time speed threshold for sports other than cycling and running (TD names only those two). Applied default: `cycling`, `e_biking` 1 km/h, everything else 0.5 km/h. | Nothing (default applies); changes R-7 constants only | user / requirements |
| OQ-5 | FIT Protocol License (ADR-003, Open). | Nothing in F-02; must be settled before launch | user |
