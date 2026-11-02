# Parser and data file measurements

Parse time and memory for the 12-hour file (F-02 AC 12). No target in F-02; F-08 sets the parse limits from these numbers. Add a row when the parser, the library or the 12-hour file changes.

How to measure (F-02 design, section Measurements):

```
cd src/server && go test ./internal/ingest -run '^$' -bench BenchmarkParse12h -benchtime 20x -count 3
make build-server && /usr/bin/time -v build/workout-explorer parse src/server/testdata/corpus/synthetic/ride-12h.fit > /dev/null
```

`BenchmarkParse12h` reads the file into memory once and measures `ingest.Parse` only. The max RSS is that of the whole `parse` command: decoding, normalisation and the JSON report.

| Date | Commit | CPU | Go | File | Records | ns/op | B/op | allocs/op | Max RSS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-10 | e415d92 + F-02 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.8 linux/amd64 | `synthetic/ride-12h.fit`, 1,299,831 bytes | 43,200 | 52.6 M to 74.5 M (3 runs of 20; about 63 ms) | 27.6 MB | 474 | 23.2 MB to 28.1 MB (3 runs; wall clock 0.09 s to 0.10 s) |
| 2026-10-10 | e415d92 + F-02 v1.1 (uncommitted; component expansion off) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.8 linux/amd64 | `synthetic/ride-12h.fit`, 1,299,831 bytes | 43,200 | 68.6 M to 72.5 M (3 runs of 20; about 71 ms) | 27.6 MB | 473 to 474 | 23.1 MB to 27.7 MB (3 runs) |

## Parse limits (F-08, ADR-017 item 7)

`BenchmarkParseAtCap` parses an in-memory **all-caps** file through `ingest.Parser` (F-08 design v1.1, ADR-017 rev. 1 item 7): 500,000 records at 1 Hz with every stream, 100 sessions alternating `cycling` and `running` (so R-5 and R-7 work on every sample), 10,000 laps with `lap.sport`, 100,000 timer events, 1,000 `sport` messages and 256 field descriptions, in one sequence, 15.9 MB. Its sub-benchmarks `decode` (the sequence walk and listener) and `normalise` (`workout.Normalise` on the decoded recording) time the two stages apart; `workout.BenchmarkNormaliseAllCaps` is the same `Normalise` input built without ingest. The max RSS is that of the whole `parse` command on the twin written by `corpusgen -cap-file` (never committed, 17.4 MB: the library's encoder writes a message definition at every switch of message kind, so the twin's 100,000 timer events sit in 100 bursts of 1,000 instead of being spread over the file, which would not fit in `MaxBytes`). Stop rule: max RSS at the caps at most 400 MB and decode plus normalise at most 2 s on the development machine.

```
cd src/server && go test ./internal/ingest -run '^$' -bench 'ParseAtCap|Parse12h' -benchtime 5x -count 3 -benchmem
cd src/server && go test ./internal/workout -run '^$' -bench NormaliseAllCaps -benchtime 5x -count 3 -benchmem
make build-server && (cd src/server && go run ./tools/corpusgen -cap-file ../../build/cap-500k.fit)
/usr/bin/time -v build/workout-explorer parse build/cap-500k.fit > /dev/null
```

| Date | Commit | CPU | Go | File | Records | ns/op | B/op | allocs/op | Max RSS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | `synthetic/ride-12h.fit` (`BenchmarkParse12h`, through `Parser`) | 43,200 | 57 M to 87 M (3 runs of 5; about 75 ms) | 27.8 MB | 553 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | v1.0 history: records only, 15,041,423 bytes (`BenchmarkParseAtCap`; `build/cap-500k.fit` for the RSS) | 500,000 | about 500 M (0.5 s) | 341.6 MB | 90 | 209 MB to 238 MB (3 runs; wall clock 1.04 s to 1.06 s including the JSON report) |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | **all caps**, parse = decode + normalise (`BenchmarkParseAtCap/parse`) | 500,000 | 542 M to 547 M (3 runs of 5; about 0.55 s) | 375.0 MB | 162,484 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps, decode only (`BenchmarkParseAtCap/decode`) | 500,000 | 419 M to 423 M (about 0.42 s) | 276.8 MB | 161,986 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps, `Normalise` only (`BenchmarkParseAtCap/normalise`; `BenchmarkNormaliseAllCaps` 86 M to 99 M) | 500,000 | 102 M to 107 M (about 0.10 s) | 96.2 MB | 487 | — |
| 2026-10-15 | 0ac678e + F-08 v1.3 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps after R-13 (one timeline lookup per sample), parse = decode + normalise (`BenchmarkParseAtCap/parse`; 3 runs of 5) | 500,000 | 566 M to 583 M (about 0.57 s) | 376.9 MB | 162,414 | — |
| 2026-10-15 | 0ac678e + F-08 v1.3 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps after R-13, `Normalise` only (`BenchmarkParseAtCap/normalise`; 3 runs of 5; the all-caps file has no transition session, so R-13 only pays the lookups) | 500,000 | 115 M to 117 M (about 0.12 s) | 98.1 MB | 416 | — |
| 2026-10-15 | 0ac678e + F-08 v1.3 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | worst case for R-13: all caps with **every** session a `transition` session, `Normalise` only (QA probe of round 3, not a committed benchmark; 2 runs; every sample is masked and every session and lap drops its totals) | 500,000 | 198 M to 201 M (about 0.20 s) | 143.4 MB | — | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps, `parse` command on the 17,360,175 byte twin `build/cap-500k.fit` | 500,000 | — | — | — | 227 MB to 229 MB (3 runs; wall clock 1.19 s to 1.22 s including the JSON report) |

Result (stop rule of ADR-017 rev. 1, figures of F-08 v1.3): **within the limits.** Max RSS 229 MB of 400 MB (v1.1 measurement; the v1.3 changes add no retained memory); decode 0.42 s plus normalise 0.12 s = 0.57 s of 2 s for the all-caps benchmark file, which has no transition session. Worst case, every session a transition: `Normalise` 0.20 s and 143 MB per call, so decode plus normalise is about 0.62 s of 2 s. QA measured `Normalise` at 0.91 s with 100 sessions before v1.1, because `sessionAt` scanned every session for every sample; the sport timeline (one lookup of O(log k) per sample) brings it to 0.12 s. Two parse slots at the caps would peak near 2 x 230 MB in one process (F-31 sizing).

Hostile in-memory files (`TestLimits`, `TotalAlloc` growth during the call): record flood (20 MiB of one-byte records) stopped at the record cap after about 240 ms and 237 MiB; sample flood (150,000 records 4 s apart) 77 MiB; the other budgets under 10 MiB.

## Accept at the caps (F-09, ADR-017 rev. 2 item 4)

`BenchmarkAcceptAtCap` runs `Parser.Accept` (parse, strip, reparse, verify, in one slot) on the all-caps file; the RSS is that of `parse -original` on the 17,360,175 byte twin `build/cap-500k.fit`. Stop rule: max RSS at most 400 MB, each decode + normalise at most 2 s, whole `Accept` at most 4.5 s.

```
cd src/server && go test ./internal/ingest -run '^$' -bench 'AcceptAtCap' -benchtime 5x -count 3 -benchmem
/usr/bin/time -v build/workout-explorer parse -original <path> build/cap-500k.fit > /dev/null
```

| Date | Commit | CPU | Go | File | Records | ns/op | B/op | allocs/op | Max RSS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-16 | 8ab9085 + F-09 round 1 (uncommitted), before ADR-017 rev. 3 | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps, `Accept` (`BenchmarkAcceptAtCap`; 3 runs of 5) | 500,000 | 2,284 M to 2,305 M (about 2.3 s of 4.5 s) | 885.8 MB | 467,700 | — |
| 2026-10-16 | 8ab9085 + F-09 round 1 (uncommitted), before ADR-017 rev. 3 | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps, `parse -original` on the 17,360,175 byte twin (3 runs; wall clock 3.0 s to 3.1 s including the JSON report) | 500,000 | — | — | — | **413 MB to 467 MB: over the 400 MB stop rule** |
| 2026-10-16 | 8ab9085 + F-09 round 1 (uncommitted), before ADR-017 rev. 3 | same | go1.26.9 linux/amd64 | the same with `GOGC=50` / `GOGC=25` (information only, not a production setting) | 500,000 | — | — | — | 349 MB / 330 MB |

Result of round 1 (history): time within the budget (2.3 s of 4.5 s); **the RSS stop rule was exceeded** (reported to the architect; no cap or code was changed to hide it). Cause: the first parse stayed reachable through `run`'s named result and through the outcome's listener.

### Round 2 (D-17, ADR-017 rev. 3)

The first parse is released before the strip, and the outcome copies the listener's findings as values. `TestAcceptReleasesFirstParse` guards it: on `synthetic/ride-12h.fit` the heap at the strip hook, minus the baseline, is at most the stripped size + 1 MiB. Measured excess: -19 KB with the fix; 3.3 MB with only the first workout kept reachable; 7.6 MB with the workout and the listener's recording kept reachable (the round-1 state); limit 2.3 MB. `GOGC` and `GOMEMLIMIT` unset everywhere.

Server run (not a Go test): `make build-server`, then `build/workout-explorer serve -data <fresh dir> -addr 127.0.0.1:<port>` with N uploads of `build/cap-500k.fit` sent at once, N in-flight slots (`WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS=16` for N = 16), 2 parse slots; max RSS of the `serve` process (`wait4`). Every answer was `201`. M(N) is the worst case 2·A + (N − 2)·B of the F-09 design, section Memory.

| Date | Commit | CPU | Go | File | Records | ns/op | B/op | allocs/op | Max RSS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-16 | 8ab9085 + F-09 round 2 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.9 linux/amd64 | all caps, `Accept` (`BenchmarkAcceptAtCap`; 3 runs of 5) | 500,000 | 2,458 M to 2,493 M (about 2.5 s of 4.5 s) | 885.8 MB | 467,700 | — |
| 2026-10-16 | same | same | same | A: all caps, `parse -original` on the 17,360,175 byte twin (3 runs; wall clock 3.06 s to 3.11 s) | 500,000 | — | — | — | **253 MB to 262 MB** (limit 400 MB) |
| 2026-10-16 | same | same | same | M(4): `serve`, 4 uploads of the twin at once, default settings (3 runs) | 500,000 each | — | — | — | **707 MB, 709 MB, 861 MB** |
| 2026-10-16 | same | same | same | M(16): `serve`, 16 uploads at once, 16 slots (3 runs) | 500,000 each | — | — | — | **1,120 MB, 1,155 MB, 1,237 MB** |

Result: A, 262 MB at most, is within 400 MB; `Accept` 2.5 s is within 4.5 s. M(4) is at most 861 MB, so within 1,000 MB (no F-31 escalation from this figure). The worst-case model gives B = (M(4) − 2·A) / 2 = at most 169 MB; M(16) at most 1,237 MB is below 2·A + 14·B (about 2.9 GB), since the uploads do not all peak together. M(16) is information for F-31 and OQ-51.


### Round 3 (F-09 v1.4)

`TestAcceptReleasesFirstParse` now keeps the upload live through the hook (`runtime.KeepAlive`, QA-09-r1-3), as the request buffer is on the server. Measured excess on `synthetic/ride-12h.fit` with the fix: 1.28 MB (the stripped size is 1.30 MB; limit stripped size + 1 MiB), the same in 3 runs. With only the first workout kept reachable: 4.6 MB. With only the listener kept in the outcome: 5.6 MB. Both fail the test.

L, the heap of the upload limiter at its cap (`TestLimiterMemory`: 100,000 keys x 60 attempts, `uint32` seconds, one array per key): **29.5 MB** in each of 3 runs (limit 40,000,000 bytes, about 295 bytes per key). The `[]time.Time` representation of round 1 gives 159 MB and fails the test. L is added on top of M(4) (ADR-017 rev. 4); the server run does not fill the table and was not repeated.


### Round 4 (F-09 v1.5, ADR-017 rev. 5)

The decoder now also has a definition listener (the record counter, ADR-003 rev. 5). `BenchmarkAcceptDefinitions` runs `Accept` on in-memory definition floods that are never committed: one closed sequence of about 20,000,000 bytes, a `file_id`, one `record`, then definitions of a manufacturer-range message to the end, with 1 field (9 bytes) or 255 fields (771 bytes) each. Stop rule: at most 4.5 s per `Accept`; A at most 400 MB. `GOGC` and `GOMEMLIMIT` unset.

```
cd src/server && go test ./internal/ingest -run '^$' -bench 'AcceptDefinitions' -benchtime 3x -count 2 -benchmem
cd src/server && go test ./internal/ingest -run '^$' -bench 'AcceptAtCap' -benchtime 3x -count 2 -benchmem
/usr/bin/time -v build/workout-explorer parse -original <path> build/cap-500k.fit
```

| Date | Commit | CPU | Go | File | Records | ns/op | B/op | allocs/op | Max RSS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-21 | 8ab9085 + F-09 round 4 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.10 linux/amd64 | definition flood, 1 field per definition, `Accept` (`BenchmarkAcceptDefinitions/fields=1`; 2 runs of 3) | about 2.2 million definitions | 702 M to 709 M (about 0.7 s of 4.5 s) | 176.5 MB | 8,889,000 | — |
| 2026-10-21 | same | same | go1.26.10 | definition flood, 255 fields per definition, `Accept` (`BenchmarkAcceptDefinitions/fields=255`; 2 runs of 3) | about 26,000 definitions | 444 M to 468 M (about 0.5 s of 4.5 s) | 273.2 MB | 311,500 | — |
| 2026-10-21 | same | same | go1.26.10 | all caps, `Accept` (`BenchmarkAcceptAtCap`; 2 runs of 3) | 500,000 | 2,320 M to 2,346 M (about 2.3 s of 4.5 s) | 885.8 MB | 467,700 to 468,000 | — |
| 2026-10-21 | same | same | go1.26.10 | A: all caps, `parse -original` on the 17,360,175 byte twin (3 runs; wall clock 3.15 s to 3.37 s) | 500,000 | — | — | — | **237 MB to 270 MB** (limit 400 MB) |

Result: both floods are within 4.5 s (the 1-field flood is the slower one, 0.7 s); `Accept` at the caps is unchanged (2.3 s); A, 270 MB at most, is within 400 MB. The server run and `TestLimiterMemory` were not repeated (the design says they need not be).


### Round 8 re-measurement of the definition floods (F-09, QA-09-r4-5)

The round-4 row above predates the framing iterator (round 7) and the cross-check of the stored file (rounds 7 and 8): `Accept` now frames every record through `frame.go` and compares the reparse with the first parse, so it allocates more per definition. Same inputs and stop rule as round 4 (synthetic floods only, nothing committed); same command as above.

| Date | Commit | CPU | Go | File | Records | ns/op | B/op | allocs/op | Max RSS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-22 | 1a00574 + F-09 round 8 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.10 linux/amd64 | definition flood, 1 field per definition, `Accept` (`BenchmarkAcceptDefinitions/fields=1`; 2 runs of 3) | about 2.2 million definitions | 846 M to 850 M (about 0.85 s of 4.5 s) | 283.2 MB (round 4: 176.5 MB) | 8,889,100 to 8,889,400 | — |
| 2026-10-22 | same | same | go1.26.10 | definition flood, 255 fields per definition, `Accept` (`BenchmarkAcceptDefinitions/fields=255`; 2 runs of 3) | about 26,000 definitions | 470 M to 481 M (about 0.5 s of 4.5 s) | 274.5 MB (round 4: 273.2 MB) | 311,500 | — |

Result: both floods are still within 4.5 s (the 1-field flood is the slower one, 0.85 s). The 400 MB limit applies to max RSS (A), not to B/op, and was not re-measured. The round-4 rows above are kept as history.

## Data file sizes (F-03)

`data.bin` and `data.bin.br` sizes of every **synthetic** corpus file (F-03 AC 5; F-03 design, section Size budget). Budget: a 4-hour ride with every stream under 500,000 bytes as `.br` (decimal KB, ADR-002); `TestSizeBudget4h` asserts it on `ride-4h.fit` in CI. Sizes of real files are never recorded here: they go into the QA report, by manufacturer only (ADR-004 item 7, rev. 3). Add rows when the format, the normaliser, the brotli parameters or a synthetic file changes.

How to measure: `make corpus` prints `bin <bytes> br <bytes>` per file. Streams counts the columns including `time_offset`. `.br per 4 h` is `br × 14,400 / samples`, only for files whose last time offset is 1 h or more. Brotli: `andybalholm/brotli` v1.2.6, window 22; quality 9 since F-03 design v1.1 (ADR-006 rev. 1). The first block of rows was measured at quality 11.

| Date | Commit | File | Samples | Streams | `data.bin` bytes | `data.bin.br` bytes | `.br` per 4 h |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/brick-run-bike.fit` | 3,601 | 9 | 95,978 | 15,639 | 62,539 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/climb-100m.fit` | 1,120 | 10 | 32,360 | 3,177 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/cycling-no-timer-slow.fit` | 1,200 | 10 | 34,544 | 2,663 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/enhanced-and-legacy.fit` | 180 | 10 | 7,008 | 1,131 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/flat-noise.fit` | 3,600 | 10 | 99,324 | 11,234 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/gps-dropout.fit` | 3,601 | 10 | 99,749 | 18,854 | 75,395 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/indoor-no-distance-no-gps.fit` | 1,801 | 2 | 9,833 | 1,232 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/indoor-no-gps.fit` | 3,601 | 7 | 56,773 | 13,652 | 54,593 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/no-distance-with-gps.fit` | 1,801 | 7 | 41,485 | 3,969 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/pause-hours.fit` | 3,601 | 10 | 99,769 | 19,008 | 76,011 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/pool-swim.fit` | 1,201 | 5 | 17,929 | 1,423 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/ride-12h.fit` | 43,200 | 10 | 1,176,640 | 212,617 | 70,872 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/ride-4h.fit` | 14,400 | 10 | 396,156 | 93,668 | 93,668 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/rolling-hills.fit` | 2,401 | 10 | 66,965 | 7,562 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/smart-recording.fit` | 608 | 10 | 18,576 | 4,026 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/system-time-record.fit` | 600 | 10 | 18,348 | 3,961 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/timer-pause.fit` | 1,801 | 10 | 50,817 | 8,271 | — |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | `synthetic/unknown-messages.fit` | 301 | 10 | 10,285 | 2,293 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/brick-run-bike.fit` | 3,601 | 9 | 95,978 | 18,953 | 75,791 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/climb-100m.fit` | 1,120 | 10 | 32,360 | 3,908 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/cycling-no-timer-slow.fit` | 1,200 | 10 | 34,544 | 3,014 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/enhanced-and-legacy.fit` | 180 | 10 | 7,008 | 1,275 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/flat-noise.fit` | 3,600 | 10 | 99,324 | 13,832 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/gps-dropout.fit` | 3,601 | 10 | 99,749 | 22,140 | 88,535 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/indoor-no-distance-no-gps.fit` | 1,801 | 2 | 9,833 | 1,470 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/indoor-no-gps.fit` | 3,601 | 7 | 56,773 | 15,739 | 62,939 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/no-distance-with-gps.fit` | 1,801 | 7 | 41,485 | 4,957 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/pause-hours.fit` | 3,601 | 10 | 99,769 | 22,469 | 89,851 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/pool-swim.fit` | 1,201 | 5 | 17,929 | 1,790 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/ride-12h.fit` | 43,200 | 10 | 1,176,640 | 266,150 | 88,717 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/ride-4h.fit` | 14,400 | 10 | 396,156 | 106,951 | 106,951 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/rolling-hills.fit` | 2,401 | 10 | 66,965 | 9,165 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/smart-recording.fit` | 608 | 10 | 18,576 | 4,673 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/system-time-record.fit` | 600 | 10 | 18,348 | 4,644 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/timer-pause.fit` | 1,801 | 10 | 50,817 | 10,024 | — |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | `synthetic/unknown-messages.fit` | 301 | 10 | 10,285 | 2,690 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/chained-ride-run.fit` | 2,100 | 10 | 59,840 | 11,862 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/chained-with-settings.fit` | 1,800 | 10 | 52,460 | 12,175 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min.fit` | 1,800 | 10 | 52,460 | 12,175 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min-bad-crc.fit` | 1,800 | 10 | 52,460 | 12,175 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min-bad-header-crc.fit` | 1,800 | 10 | 52,460 | 12,175 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min-cut-60.fit` | 1,084 | 10 | 31,684 | 7,841 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min-cut-60-size-0.fit` | 1,084 | 10 | 31,684 | 7,841 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min-no-crc.fit` | 1,800 | 10 | 52,460 | 12,175 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/ride-30min-trailing-zeros.fit` | 1,800 | 10 | 52,460 | 12,175 | — |
| 2026-10-15 | 2ca94a8 + F-08 (uncommitted), quality 9 | `synthetic/triathlon.fit` | 3,720 | 10 | 104,736 | 18,368 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted), quality 9 | `synthetic/run-30min.fit` | 1,800 | 9 | 48,508 | 8,579 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted), quality 9 | `synthetic/run-30min-cut-60.fit` | 1,084 | 9 | 29,308 | 5,508 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted), quality 9 | `synthetic/triathlon-cut-90.fit` | 3,360 | 10 | 94,352 | 16,888 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted), quality 9 | `synthetic/chained-ride-ride.fit` | 1,200 | 10 | 35,984 | 7,673 | — |
| 2026-10-15 | 2ca94a8 + F-08 v1.1 (uncommitted), quality 9 | `synthetic/chained-ride-ride-cut-75.fit` | 901 | 10 | 27,185 | 6,120 | — |
| 2026-10-26 | F-17 B-1, format v2 (uncommitted), quality 9 | `synthetic/ride-4h.fit` | 14,400 | 12 | 511,560 | 109,596 | 109,596 |
| 2026-10-26 | F-17 B-1, format v2 (uncommitted), quality 9 | `synthetic/ride-12h.fit` | 43,200 | 12 | 1,522,444 | 268,769 | 89,590 |

Format version 2 (F-17 B-1, ADR-029) adds `moving_time` and `elevation_gain` (8 bytes per sample, 35 at most). Growth against the version 1 files at the same quality (396,156 and 1,176,640 raw bytes; 106,951 and 266,150 brotli bytes): `ride-4h` +115,404 raw (+204 metadata) and +2,645 brotli (+2.5 %); `ride-12h` +345,804 raw and +2,619 brotli (+1.0 %). ADR-029 trigger (more than 5 KB on `ride-4h`) not reached; the 500,000-byte budget holds with 390 KB to spare. The other synthetic files grow by the same 8 bytes per sample raw; `make corpus` prints the current sizes.

Quality comparison (window 22) that led to quality 9 (F-03 design v1.1, ADR-006 rev. 1). Single runs on an Intel Core i7-4770T @ 2.50 GHz, go1.26.8, with the 10 Oct 2026 data files (unchanged since):

| Quality | `ride-4h` `.br` bytes | time | `ride-12h` `.br` bytes | time |
| --- | --- | --- | --- | --- |
| 9 | 106,951 | 0.06 s | 266,150 | 0.13 s |
| 10 | 93,279 | 0.55 s | 220,248 | 1.32 s |
| 11 | 93,668 | 1.10 s | 212,617 | 3.43 s |

Encode and brotli time for the 12-hour file (ADR-006 item 5). `ride-12h.fit` is parsed once outside the loop; `BenchmarkBrotli12h` compresses the 1,176,640-byte data file.

```
cd src/server && go test ./internal/datafile -run '^$' -bench '12h' -benchtime 10x -count 3
```

| Date | Commit | CPU | Go | Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.8 linux/amd64 | `BenchmarkEncode12h` | 7.1 M to 12.9 M (3 runs of 10; about 9 ms) | 8.8 MB | 461 to 514 |
| 2026-10-10 | f8a6c24 + F-03 (uncommitted), quality 11 | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.8 linux/amd64 | `BenchmarkBrotli12h` | 3,467 M to 3,646 M (3 runs of 10; about 3.6 s) | 154.1 MB | 123 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.8 linux/amd64 | `BenchmarkEncode12h` | 8.1 M to 15.8 M (3 runs of 10; about 11 ms) | 8.8 MB | 461 to 516 |
| 2026-10-11 | f8a6c24 + F-03 v1.1 (uncommitted), quality 9 | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.8 linux/amd64 | `BenchmarkBrotli12h` | 117.9 M to 122.1 M (3 runs of 10; about 120 ms) | 51.9 MB | 146 to 148 |

## Public build (F-15, ADR-028)

A whole public build of synthetic `ride-12h.fit` in `internal/publish`: read the original, parse, encode, decode, page, template, the file writes and `fsync`s. `BenchmarkPublishRide12h` is a first publish (both data files written, brotli included); `BenchmarkTitleSaveRide12h` is a title save (only `index.html` written, no compression). ADR-028 trigger: a title save above 1 s adds a page-only path.

```
cd src/server && go test ./internal/publish -run '^$' -bench Ride12h -benchtime 10x -count 3 -benchmem
```

| Date | Commit | CPU | Go | Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-25 | 96ece12 + F-15 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.10 linux/amd64 | `BenchmarkPublishRide12h` (3 runs of 10) | 167 M to 171 M (about 0.17 s) | 95.3 MB | 8,958 to 9,012 |
| 2026-10-25 | 96ece12 + F-15 (uncommitted) | Intel Core i7-4770T @ 2.50 GHz (8 threads) | go1.26.10 linux/amd64 | `BenchmarkTitleSaveRide12h` (3 runs of 10) | 115 M to 168 M (about 0.13 s) | 42.7 MB | 8,702 |
