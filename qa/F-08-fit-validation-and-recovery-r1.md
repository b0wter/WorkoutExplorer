# QA F-08 FIT validation, truncated-file recovery and parse limits - round 1

- **Date:** 2026-10-15
- **Verdict:** **not ready.**
  - The rejection paths, the CRC paths, the limits, the log line and the corpus all work as designed, and every acceptance criterion has code and a passing test.
  - Three Major findings remain open. All three are in the recovered-workout model, not in the code's fidelity to the design: the design does not cover these cases.
    - QA-08-r1-1: a truncated file loses its sport, so running cadence is halved. Shown on every cut of the local real running file.
    - QA-08-r1-2: a truncated multi-session file silently drops its last leg from the summary totals.
    - QA-08-r1-3: in a chained file, the distance stream restarts at 0 in every sequence. This gives a wrong average speed in the summary of a same-sport chained file.
  - All three go to the architect first.
- **Inputs:**
  - `features/F-08-fit-validation-and-recovery.md`, plus `features/README.md` decisions 11, 12 and 20 and OQ-22/23.
  - Spec: UP-2, UP-3, AP-2, AP-12, NFR Security/Reliability/Operations. TD: FIT parsing (Conversions, Edge cases to test), Security and privacy (Uploads), Versioning.
  - Architecture: `architect/F-08-fit-validation-and-recovery.md` 1.0, ADR-017, ADR-018, ADR-003 rev. 2, ADR-004 rev. 6, F-02 design 1.7 and overview 1.11. OQ-30 to OQ-34 were accepted on 2026-10-15.
  - State: HEAD `2ca94a8` plus the uncommitted working tree as listed by `git status` at the start of the review. Nothing was committed.

## Build and test results

| Check | Result |
| --- | --- |
| `make ci` (working tree) | **Exit 0.** `go vet` clean. svelte-check: 256 files, 0 errors, 0 warnings. Go: 9/9 packages ok. Vitest: 29 files, 756 passed. Size: `entry index.html: 45582 / 60000 bytes gzip (40912 brotli) PASS` (unchanged; F-08 is backend only). |
| `go test -count=1 -race ./internal/ingest ./internal/workout ./internal/datafile ./cmd/workout-explorer ./tools/corpusgen` | 5/5 ok (ingest 25.1 s, datafile 17.2 s) |
| `make corpus` | `corpus: 40 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`. 36 synthetic files: 27 accepted, 9 `rejected` lines, as the design table says. Real: 4 local files from 2 manufacturers, all ok, all report version 2, all clean (`sequences 1`, not truncated, no CRC failure, 1 session). |
| `go test ./internal/ingest -run '^$' -fuzz FuzzParse -fuzztime 5m -parallel 4` (in a scratchpad copy of `src/server`, without real files, so no failing input could land in the repo) | **PASS.** 1,456,720 execs, 160 corpus entries, no failure. |
| `BenchmarkParseAtCap` / `BenchmarkParse12h` (`-benchtime 3x -count 2 -benchmem`) | At the cap: 515–518 ms/op, 341.6 MB/op, 89–91 allocs. 12 h: 60–120 ms/op, 27.8 MB/op. Matches `MEASUREMENTS.md`. |
| `/usr/bin/time -v build/workout-explorer parse build/cap-500k.fit` × 3 | Max RSS 209, 223 and 243 MB. Wall time 1.01–1.09 s, including the JSON report. Within the 400 MB / 2 s stop rule. The 243 MB run is slightly above the engineer's 209–238 MB, which does not matter. |
| QA probe: every budget at its cap in one file (499,999 records, 100 sessions, 10,000 laps, 100,000 timer events), in memory | Accepted in 1.13–1.15 s (2.26 s while the fuzzer ran). TotalAlloc +357 MiB. See QA-08-r1-5. |
| Golden diff review (script, HEAD vs working tree, all 18 existing synthetic reports) | Every existing report changes only in the version line, `integrity` and the new `derived` keys. The one exception is `brick-run-bike`, whose `summary` changes as the revised R-10 requires (see Engineer's assumptions). |
| Commit hygiene | `git status --porcelain --untracked-files=all`: every untracked file is synthetic (corpus files, golden reports and data files named after synthetic files), plus code and ADRs. `--ignored` shows `corpus/real/*` and `expected/real/*` (4 each), `build/cap-500k.fit`, `build/cap.bin(.br)` and `build/corpus/real/*` as ignored. `internal/ingest/testdata/fuzz/` does not exist. `MEASUREMENTS.md` and `README.md` contain no real-file figures. Nothing derived from real files would be committed. |

QA probes live only in `…/scratchpad/copy/server/internal/ingest/qa_probe_test.go`, a copy of `src/server` in the session scratchpad. They read the real files in memory, write nothing and print only aggregates.

## Findings

### Major

- **ID:** QA-08-r1-1
- **Severity / kind:** Major / defect
- **Route:** architect, then backend
- **Where:** `src/server/internal/workout/timezone.go:29` (`workoutSport` reads sessions only). `src/server/internal/workout/cadence.go:43` (`convertCadence` does nothing without sessions). `src/server/internal/ingest/listener.go:123-132` (the `sport` message, mesg 12, is never read).
- **Relates to:** UP-3, AC 4 ("streams end at the last intact record"), AP-2 (cadence unit and speed or pace depend on the sport), F-02 R-5 and R-9, decision 11.
- **Evidence:**
  - A truncated file has no `session` message, so the workout's sport is `""`. The R-5 doubling of running cadence also does not happen.
  - QA cut each local real file in memory at 1 %…99 % of its length.
    - All 396 cuts are accepted, which is correct.
    - For the one real running file, **99 of 99 cuts have a cadence stream that is not the prefix of the full file's stream**. The sample at the middle is 79.4 instead of 158.9 (one leg instead of steps per minute), and the sport is `""`.
  - All 4 real files (2 manufacturers) write a `sport` message as the 12th or 76th message, before the first record. The session is in the last 3 messages. So in practice the sport is always available in a truncated file, but it is never read.
  - The same cause hits the synthetic triathlon cut at 90 %. Run-leg cadence at offset 3000 is 85.5 instead of 171: `sessionAt` falls back to the T2 transition session.
  - `ride-30min-cut-60.json` also has no `sport` key.
  - `TestTruncationPrefix` passes only because its fixture is a ride, where cadence is not doubled.
- **Consequence:** the most common UP-3 case ("watch died mid-run") shows the run at half its cadence and labels it with neither sport nor pace (DS-2). It also gives F-13/F-23/UP-5 no sport. The design needs a sport source for session-less recordings, for example the `sport` message or `lap.sport`, used by R-5 and R-9. It also needs a truncated running file in the corpus.

- **ID:** QA-08-r1-2
- **Severity / kind:** Major / defect (design gap)
- **Route:** architect, then backend
- **Where:** `src/server/internal/workout/summary.go:11-53` (step 2 fills only `nil` fields). F-08 design, section Summary (R-10 revised).
- **Relates to:** AC 6, UP-3, AP-2 ("totals … for the whole workout"), decision 11, TD edge cases ("Files with several sessions", "Truncated files").
- **Evidence:** QA probe cuts of `synthetic/triathlon.fit`. The run session is lost; the other 4 survive.

  | Cut | Data ends at | Summary elapsed | Summary distance | Record distance |
  | --- | --- | --- | --- | --- |
  | 80 % | 2,990 s | 2,516 s | 14,376.5 m | 15,938.3 m |
  | 95 % | 3,544 s | 2,516 s | 14,376.5 m | 17,765.4 m |

  Ascent, calories and timer time are also sums over the four surviving sessions. The same happens to a chained file whose second sequence breaks before its session: probe with 2 × 600 records, summary distance 2,995 m and elapsed 599 s while the data spans 1,181 s.
- **Consequence:** the summary silently under-reports the workout and stops changing as more data is recovered. The design's rule fills only missing fields, which works only when no session survives. AC 6's intent ("no total is shown as zero because the session message is missing") is met literally but not in substance. The design needs a rule for records that run past the last surviving session's end: either use record totals for the additive values when `Truncated`, or add the uncovered tail.

- **ID:** QA-08-r1-3
- **Severity / kind:** Major / defect (design gap)
- **Route:** architect, then backend
- **Where:**
  - `src/server/internal/workout/normalise.go:44`: `Derived.Distance = lastValid(distance)`.
  - `src/server/internal/workout/stats.go:109`: `AvgSpeed = Derived.Distance / MovingTime`.
  - `src/server/internal/workout/summary.go:47`: average speed filled when the sports are the same.
  - Distance stream across sequences: the F-08 design, R-d, does not mention it.
- **Relates to:** AC 7, feature scope ("streams on one time axis"), AP-2, AP-5 (distance axis), AP-8, OQ-23.
- **Evidence:**
  - Every chained sequence restarts its `distance` at 0, and `ingest` concatenates the records unchanged.
  - Committed golden `chained-ride-run.json`:
    - `derived.distance 2974.23` (run leg only), while `summary.distance` is 11,974.46.
    - `derived.avg_speed 1.418` m/s, while the true average is about 5.7 m/s.
  - QA probe, a ride split into two cycling sequences (600 records each at 5 m/s):
    - The distance stream at sample 599 is 2,995 and at sample 600 it is 0.
    - `Summary.AvgSpeed = 2.5` m/s. This value is shown (same sport, so no mixed-sports hiding).
- **Consequence:**
  - A chained file of one sport shows an average speed that is wrong by the share of earlier sequences.
  - Every chained file carries a non-monotonic distance stream into the data file. The distance axis (AP-5, F-17), range distance (AP-7) and km splits (AP-8) will fold back at each sequence boundary.
  - The design must decide how distance continues across sequences, for example by offsetting each later sequence by the last valid distance before it. It must also add a same-sport chained corpus file.

### Minor

- **ID:** QA-08-r1-4
- **Severity / kind:** Minor / defect
- **Route:** architect (clarify R-d versus the first-sequence gate), then backend
- **Where:** `src/server/internal/ingest/listener.go:146` (`if l.seenFileID` decides between the "first" and "later" rules per workout, not per sequence).
- **Relates to:** UP-2, R-d, ADR-018 item 1, NFR Reliability.
- **Evidence:** QA probe. Sequence 1 has 10 records and no `file_id`. It is followed by a `settings` sequence. Result: **rejected `unsupported_type`, `file_type: settings`**. R-d says a later non-activity sequence is skipped. The first-sequence row of the Listener table says the first `file_id` of the workout gates the file. The two rules conflict here, and the code silently picks the second. This is a different case from engineer assumption 4.
- **Consequence:** a file is rejected whose activity data is intact. This is rare, because a FIT file without a leading `file_id` is malformed, but the behaviour is undecided and untested.

- **ID:** QA-08-r1-5
- **Severity / kind:** Minor / risk
- **Route:** backend (measurement); architect if ADR-017 item 7 should name the combined case
- **Where:** `src/server/internal/workout/cadence.go:10` (`sessionAt` scans the sessions linearly). It is called once per sample by `convertCadence`, and by `movingTime` when there are no timer events.
- **Relates to:** ADR-017 items 2 and 7, NFR Security.
- **Evidence:**
  - The QA probe with every budget at its cap takes 1.13–1.15 s. Split: decode 0.40 s, `Normalise` 0.91 s with 100 sessions, against 0.12 s without them.
  - `MEASUREMENTS.md` and `BenchmarkParseAtCap` cover the record cap only (0.5 s).
  - `Normalise` runs outside the decode deadline (ADR-017 item 2).
- **Consequence:** the real worst case is more than double the recorded one. It is still inside the 2 s stop rule, but with half the margin, and on a VPS core two to three times slower it is about 3 s that no deadline covers. Record the combined case in `MEASUREMENTS.md`, or make `sessionAt` O(log n).

- **ID:** QA-08-r1-6
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/workout/f08_test.go:12`, `src/server/internal/ingest/hostile_test.go:242`, `src/server/internal/ingest/f08_test.go:493`
- **Relates to:** F-08 design, section Tests.
- **Evidence:**
  - (a) `TestSampleCount` does not run "every corpus file's recording", which the design asks for. A QA probe ran it on all synthetic files: equal, so the code is right and only the test is missing.
  - (b) `TestLimitsAtTheCap` sets `MaxSessions: 2` but tests only records.
  - (c) `TestParseLogs` runs the record limit, too-large and cancel cases, but not the time limit or the other budgets. The design says "every hostile, panic and cancel case". The panic line is checked in `TestPanicContained`, but it is not scanned for positions.
  - (d) The truncation fixtures are all rides. See QA-08-r1-1.
- **Consequence:** regressions in these paths would not be caught.

- **ID:** QA-08-r1-7
- **Severity / kind:** Minor / preference
- **Route:** backend
- **Where:**
  - `src/server/cmd/workout-explorer/parse.go:63`: the result of `stdout.Write(report)` is ignored.
  - `src/server/internal/ingest/errors.go:25`: the comment calls the wording "provisional", but OQ-30 was accepted.
  - `src/server/testdata/corpus/MEASUREMENTS.md`, F-08 rows: they say `go1.26.8`, but the local toolchain is `go1.26.9` (`GOTOOLCHAIN=local`).
- **Relates to:** code hygiene.
- **Evidence / Consequence:** cosmetic. A failed stdout write in the diagnostic CLI still exits 3.

## Engineer's assumptions and deviations

| # | Item | Judgement |
| --- | --- | --- |
| 1 | Cut-60 `data_end` 1083 against the design's "somewhat below 1,080" | **Accept.** The design number was illustrative. The binding rule `data_end == samples.last_offset` holds (1,084 samples, last offset 1,083). |
| 2 | Session-less summary, all-zero cadence: max 0, no average | **Accept.** OQ-23 excludes zeros from the average. A recorded maximum of 0 is data, not a missing metric (AP-2 "never shown as zero" is about missing values). F-13 can still choose to hide it. |
| 3 | A skipped later sequence does not set the workout's truncation or CRC flags | **Accept.** Nothing of the workout is missing, so no UP-3 notice is due (OQ-33 logic). |
| 4 | A first-sequence `file_id` that arrives after records is the workout's `file_id` and is gated | **Accept** (E-4). The neighbouring case where the first sequence has no `file_id` at all is QA-08-r1-4. |
| 5 | `parse` usage line no longer mentions `-datafile` | **Accept.** It is the design's exact text. `parse -h` still documents the flag. |
| 6 | `TestParseLogs` drops slog's `time` key before the position scan | **Accept.** The value is the server clock, not file data. Dropping it avoids false positives. ADR-018 lists `time` outside the allow-list. |
| 7 | `crc_failed` also when only the CRC bytes are missing | **Accept.** This is R-b verbatim ("CRC bytes missing … treated as a failed CRC"). |
| 8 | Zero fields of `Limits` mean their defaults | **Accept.** This is documented on the type and on `NewParser`, and production uses `DefaultLimits` unchanged. Note for F-09: a limit of 0 cannot mean "none". |
| 9 | `BenchmarkParseAtCap` uses the in-memory builder; `corpusgen -cap-file` writes the twin to `build/` | **Accept.** Same shape: 500,000 records at 1 Hz, every stream, about 15.0 MB. QA reproduced both the benchmark and the RSS. The twin is git-ignored. |
| — | `brick-run-bike` summary: elapsed 3,600 instead of 3,660, plus device sums and maxima | **Accept** as conforming to the revised R-10: `ElapsedTime` is "Sum" for 2+ sessions, 1,200 + 2,400 = 3,600. The other changes (timer time, distance 21,956.86, ascent 159, descent 204, calories 600, maxima, record averages, no average speed or cadence for mixed sports) all follow the table. The data-file golden changes only by the summary bytes (+144). Whether "elapsed time" should mean wall time is Question Q1. |

## Previous findings

None. This is round 1.

## Questions

- **Q1 (user, via architect):** should AP-2 "elapsed time" of a multi-session workout be the sum of session elapsed times (current, revised R-10) or the wall time from first to last record?
  - With the sum, `chained-ride-run` shows 2,098 s elapsed, equal to its moving time, while its chart spans 2,998 s. `brick-run-bike` shows 3,600 s for a 3,660 s span.
  - What settles it: a one-line decision. "Sum" closes the question. "Wall time" changes step 1 of R-10 for `ElapsedTime` only.
- **Q2 (architect):** R-9 gives `multisport` to every file with 2+ sessions, including a ride split into two cycling sequences (QA-08-r1-3 probe). AP-12's wording allows this. Is it intended for same-sport chained files, or should R-9 say "2+ sessions of different sports"? What settles it: a sentence in the design. This does not block.
- **Q3 (architect, with QA-08-r1-1):** is the `sport` message (mesg 12) an acceptable sport source when no session exists? All local real files write it before the first record. If not, `lap.sport` is present in 3 of 4 real files, but only from the first lap's end.

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 PNG → "not a FIT file" | `sequence.go:26` `looksLikeFIT`, `ingest.go:89` | `not-fit-png`, `not-fit-random`, `empty` golden rejection reports; `TestKinds`; `TestParseLogs` | Traced, passes |
| 2 Header, no records → "no readable workout data" | `ingest.go:129-135` | `ride-30min-header-only`, `no-records` goldens; `TestKinds` (data size 0 header) | Traced, passes |
| 3 Course and monitoring → exactly "Unsupported FIT file type." | `listener.go:140-161`, `errors.go:29` | `course`, `monitoring`, `workout-plan` goldens; `TestFileTypes` (all 254 non-activity values); `TestKinds` exact string; CLI `TestParseRejected` | Traced, passes |
| 4 Cut at 60 %: accepted, streams end at the last intact record, elapsed reported | `sequence.go:45-110`, `normalise.go` (DataEnd) | `ride-30min-cut-60(-size-0)` goldens; `TestTruncationPrefix`; `TestUnclosedHeader`; `FuzzParse` | Traced and passes for rides. **Fails in substance for running files** (QA-08-r1-1) |
| 5 Corrupted CRC, intact records → accepted | `sequence.go:64-72, 87-88` | `bad-crc`, `bad-header-crc`, `no-crc` goldens; `TestRecoveredEqualsSource`; `TestChained` (CRC failure in sequence 1 of 2). QA real-file probe: bad file CRC and missing CRC correct on all 4 | Traced, passes |
| 6 Truncated, no session → totals from records, none zero for lack of a session | `stats.go`, `summary.go`, `elevation.go` | `cut-60` golden summary has every listed key; `TestSummaryRevised/no session`; `TestRecordTotals` | Passes for 0 sessions. Gap for partial sessions (QA-08-r1-2) |
| 7 Chained file and triathlon → one workout each | `sequence.go` walk, `listener.go` gate, `summary.go` | `chained-ride-run`, `chained-with-settings`, `triathlon` goldens; `TestChained`; `TestSummaryRevised` | Passes ("without error"). Distance and average speed wrong for chained files (QA-08-r1-3) |
| 8 Crafted loop or allocation stopped; process keeps serving | `limits.go`, `listener.go:69-119`, `ingest.go:93-119`, slots and recover in `ingest.go:42-72` | `TestLimits` (8 budgets, each within 10 s and TotalAlloc ≤ 1 GiB; record flood 20 MiB), `TestTimeLimit`, `TestHostileDoesNotBlockOthers` (goroutine count restored), `TestSlots`, `TestPanicContained`, `FuzzParse` (seeds in CI; QA 5 min) | Traced, passes. The HTTP-level check is F-09's |
| 9 Rejection and recovery log device make and model; no lat/lon in any line | `log.go`, `convert.go` `garminProductName` | `TestParseLogs` (allow-list, one line per outcome, none for clean files, device keys, semicircle and degree scan, `lat`/`lon`/`position`, serial absent) | Traced, passes. Test breadth in QA-08-r1-6(c) |

Also checked and found correct:
- Precedence (`limit (bytes)` before `not_fit`; `canceled` first; a course with records is `unsupported_type`).
- `Error.Error()` contains no field values.
- Log levels and messages per ADR-018.
- `product` is filled only for the Garmin family.
- `product_name` is cut at 64 bytes on a rune boundary.
- Panic frames are `function file:line`, with no argument words.
- The slot wait does not count towards the timeout or `duration_ms`.
- Decoder options are exactly as in ADR-003 rev. 2: `WithIgnoreChecksum` is used only for the retry or a patched header. QA verified this against the v0.28.4 source: a header CRC of 0 is skipped, and the context is checked per message.
- `workout` does no I/O, and `ingest` adds only `log/slog`.
- `corpus.sh` treats exit 3 as a rejection outcome.
- The datafile corpus tests skip rejected files and flag stale golden data files for them.
- `TestSyntheticCorpusCurrent` covers the derived files.

Not run or not traceable:
- The real-file `make corpus-update` diff cannot be reviewed from git, because the files are ignored. QA saw only the result: 4 files, report version 2, all ok.
- Behaviour on a real truncated file from a device, as opposed to cut copies, was not available.
- The 10-minute fuzz run the design asks of the engineer was not re-run in full (QA ran 5 minutes).
