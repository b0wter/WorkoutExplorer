# QA F-08 FIT validation, truncated-file recovery and parse limits - round 2

- **Date:** 2026-10-15
- **Verdict:** **ready.**
  - All three Majors from round 1 are fixed and proven by goldens, unit tests and QA probes:
    - r1-1: the sport timeline.
    - r1-2: the record leg.
    - r1-3: R-12.
  - The four Minors from round 1 are fixed. One small test gap remains (QA-08-r2-2).
  - The three new findings are all Minor. None blocks:
    - QA-08-r2-1: R-12 has no tolerance at a sequence boundary. This is a risk, shown with a probe; no known writer triggers it.
    - QA-08-r2-2: two small test gaps.
    - QA-08-r2-3: stale wording in the docs and in a code comment.
  - The two rules pending the user are isolated as the design says (section "Pending user rules"):
    - OQ-35: R-10.E elapsed time.
    - OQ-36: D-17, the sport of a same-sport multi-session file.
- **Inputs:**
  - Feature: `features/F-08-fit-validation-and-recovery.md`.
  - Architecture: `architect/F-08-fit-validation-and-recovery.md` v1.1 (D-12 to D-17, "Changes for the backend engineer (v1.1)"), ADR-017 rev. 1, ADR-018 rev. 1, ADR-003 rev. 3, F-02 design v1.8, overview v1.12, `architect/INDEX.md` (OQ-35, OQ-36).
  - Spec: UP-2, UP-3, AP-2, AP-12, NFR Security/Reliability/Operations. TD: FIT parsing (Edge cases), Security and privacy (Uploads).
  - Round 1 report: `qa/F-08-fit-validation-and-recovery-r1.md`.
  - State: HEAD `2ca94a8` plus the uncommitted working tree. Nothing was committed and nothing under `src/`, `architect/`, `features/` or `spec/` was changed.
  - Round 1 baseline for diffs: the round 1 QA scratchpad copy of `src/server` (v1.0 code, goldens and corpus files).

## Build and test results

| Check | Result |
| --- | --- |
| `make ci` (working tree) | **Exit 0.** `go vet` clean. gofmt clean. svelte-check: 256 files, 0 errors, 0 warnings. Go: 9/9 packages ok. Vitest: 29 files, 781 tests passed. Size: `entry index.html: 45582 / 60000 bytes gzip (40912 brotli) PASS` (unchanged; F-08 is backend only). The Go results were cached, so they were re-run uncached in the next row. |
| `go test -count=1 -race ./internal/ingest ./internal/workout ./internal/datafile ./cmd/workout-explorer ./tools/corpusgen` | 5/5 ok. ingest 84.2 s (includes `TestRealTruncationLocal` on the local real files), workout 1.7 s, datafile 17.9 s, cmd 1.4 s, corpusgen 3.7 s. No race reported. |
| `make corpus` | `corpus: 45 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`. 41 synthetic files: 33 accepted, 8 `rejected` (course, empty, monitoring, no-records, not-fit-png, not-fit-random, ride-30min-header-only, workout-plan). Real: 4 local files, all ok. |
| `go test ./internal/ingest -run '^$' -fuzz FuzzParse -fuzztime 5m -parallel 4` (in a scratchpad copy of `src/server` without real files, so no input could land in the repo) | **PASS.** 1,582,392 execs, baseline 227 entries, 25 new interesting. No failure. No `testdata/fuzz/` directory was created in the copy or in the repo. |
| `BenchmarkParseAtCap` (`-benchtime 3x -count 2 -benchmem`) | parse 549–557 ms/op, 375.0 MB/op. decode 413–421 ms/op, 276.8 MB/op. normalise 99–104 ms/op, 96.2 MB/op. `BenchmarkNormaliseAllCaps` 91–100 ms/op. These match the new `MEASUREMENTS.md` rows. |
| All-caps twin: `corpusgen -cap-file` into the scratchpad, then `/usr/bin/time -v workout-explorer parse` × 3 | Twin 17,360,175 bytes (the same size as recorded). Max RSS 229.1, 229.4 and 229.6 MB. Wall time 1.52–1.63 s, including the JSON report. The report shows 100 sessions, 10,000 laps and 500,000 samples. Within the 400 MB / 2 s stop rule. |
| QA probe: all-caps file cut at 99.5 % and 50 % (record leg and hint path at the cap), and with sessions removed (every sample through the hints) | Decode 0.50 s and normalise 0.12 s at 99.5 % (99 sessions plus the tail). Normalise without sessions: 0.14 s. The O((n+k) log(n+k)) bound holds on the paths the benchmark does not take: the benchmark's sessions cover every sample, so it never reaches rule 2 or the record leg. |
| Golden diff against the round 1 baseline (script) | The only existing report that changes is `chained-ride-run.json`: distance stream max, last, mean and digest, `derived.distance` (2,974.23 → 11,974.46) and `derived.avg_speed` (1.418 → 5.708). `triathlon.fit` changed its bytes (107,118 → 107,233, the sport hints), but **its report is byte-identical**. Every other synthetic `.fit`, report and data file is byte-identical. The new files are the 5 in the design. Exactly as the change table, row 7, requires. |
| Real goldens | The 4 real golden reports were rewritten by `corpus-update` and are byte-identical to the pre-v1.1 copies. The sport and summary of every real complete file are unchanged by the hints. That is the design's stop condition, and it holds. |
| Commit hygiene | Untracked files: code, ADRs, the QA report, and synthetic corpus files, goldens and data files only. Ignored: `corpus/real/*` and `expected/real/*` (4 each), `build/cap-500k.fit`, `build/cap.bin(.br)` and `build/corpus/real/*`. No `internal/ingest/testdata/fuzz/`. `MEASUREMENTS.md` and `README.md` contain no real-file figures. Nothing derived from real files would be committed. QA's own artefacts (probes, all-caps twin, fuzz log) are in the session scratchpad only. |

QA probes for this round are in `…/scratchpad/r2/server/internal/ingest/qa_r2_probe_test.go`. The round 1 probes were re-run unchanged from `qa_probe_test.go` in the same copy. They read real files in memory and print only aggregates.

## Findings

### Minor

- **ID:** QA-08-r2-1
- **Severity / kind:** Minor / risk
- **Route:** architect (rule), then backend
- **Where:** `src/server/internal/workout/sequences.go:35-37` (`if havePrev && haveFirst && first < prev { base = prev }`). F-08 design, R-12 table, row "Later sequence continues the count (first ≥ prev) → Unchanged".
- **Relates to:** R-12 (D-14), AC 7, AP-2 (average speed), AP-5/AP-7/AP-8 (distance axis), F-02 R-6a.
- **Evidence:**
  - QA probe: two cycling sequences, 600 records each at 5 m/s. The second continues the count but starts 1 cm below the first's last value (2,994.99 against 2,995).
  - Result: `derived.distance` 8,984.99 m instead of 5,990 m, and `derived.avg_speed` 7.49 m/s instead of 5.0.
  - The same sport means 2 legs with no device average, so the **summary shows 7.49 m/s**. The distance stream jumps by 2,995 m at the boundary.
  - With the second sequence starting at exactly 2,995, or at 0, the result is correct.
- **Consequence:**
  - The rule switches between "continue" and "restart" on a strict `<` with no tolerance. The design itself allows small decreases inside a sequence (R-6a).
  - A writer that carries the count across sequences, with any smoothing at the boundary, gets its distance inflated by the whole previous distance.
  - No such writer is known, which is why this is a risk and not a defect.
  - What would settle it: a restart test that needs a real restart, for example "first < prev / 2" or "first below a small absolute value". Or a sentence in the design saying the case is accepted as is.

- **ID:** QA-08-r2-2
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/ingest/f08_test.go:502-546` (`TestParseLogs` inputs). `src/server/internal/ingest/hostile_test.go:263` (`skipOverTest`).
- **Relates to:** AC 9, F-08 design section Tests ("every hostile, panic and cancel case"), QA-08-r1-6 (c).
- **Evidence:**
  - `TestParseLogs` now covers the time limit, 7 budgets, too large, panic and cancel. It does not cover the `samples` budget: `small` leaves `MaxSamples` at its default, and no input reaches it. The `samples` rejection runs through a different step (Pipeline step 7, after the walk) from the listener budgets, so its log line is the one budget line not asserted.
  - `TestLimitsAtTheCap` declares a struct field `skipOverTest` that is always `false` and never read. This is dead code.
- **Consequence:** a regression in the `samples` log line (missing device keys, a key outside the allow-list) would not be caught. The dead field is cosmetic.

- **ID:** QA-08-r2-3
- **Severity / kind:** Minor / preference (documentation)
- **Route:** architect (F-02 text), backend (comment)
- **Where:**
  - `architect/F-02-fit-parsing-and-normalisation.md:57`.
  - `src/server/internal/workout/timezone.go:32-34`.
- **Relates to:** D-17, OQ-36, F-02 v1.8 change log.
- **Evidence:**
  - F-02 line 57 still says the sport is `multisport` "when there are 2+ sessions" and that "F-08's design reuses R-9 … and must not rebuild it". v1.8 replaced that rule (line 276 is marked; line 57 is not).
  - The `workoutSport` comment says it is "the only function to change (add 'or more than one session')" if the user answers OQ-36 with "multisport". But the function receives only `S` and no session count, so the answer also changes its signature and its one caller (`normalise.go`).
- **Consequence:** a reader of F-02 gets the superseded rule. The OQ-36 change is one call site larger than the comment says. That is still well isolated.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-08-r1-1 (truncated file loses its sport; running cadence halved) | **Fixed** | `sport` messages and `lap.sport` are read (`listener.go:181-184, 246-249`). `sportTimeline` (`sports.go`) drives R-5 and R-7. Golden `run-30min-cut-60`: sport `running`, `avg_cadence` 170.836, lap cadences about 171. `TestTruncationPrefix` covers `run-30min` and `triathlon` (cadence and sport at every 1 % and 20 random cuts). Round 1 probe: triathlon cut at 90 % has run cadence 171 (full 171; round 1: 85.5). **Real files: 396 of 396 cuts (1–99 %, 4 files) are accepted, have the full file's sport, and every stream, cadence included, is a prefix of the full file's** (round 1: 99 of 99 running-file cuts had wrong cadence). |
| QA-08-r1-2 (truncated multi-session file drops its last leg) | **Fixed** | `legs.go` record leg; `summarise` combines over legs. Round 1 probe, triathlon cuts: at 80 %, summary distance 15,925.03 m with derived 15,935.12 m (round 1: 14,376.5 against 15,938.3), elapsed 2,985 s; at 95 %, 17,755.31 m with derived 17,765.4 m. Chained 2 × 600 records with the second cut before its session: 5,400 m, elapsed 1,080 s (round 1: 2,995 m, 599 s). Goldens `triathlon-cut-90` and `chained-ride-ride-cut-75` match the design's formulas (for example, elapsed 2,516 + 839 = 3,355). `TestRecordLeg` covers every case in the design's list. |
| QA-08-r1-3 (distance restarts at 0 in every chained sequence) | **Fixed** (see QA-08-r2-1 for a boundary risk) | `continueDistance` (`sequences.go`) runs before R-2 on a copy. The input is unchanged (tested). Round 1 probe: the stream at samples 599/600 is 2,995/2,995 (round 1: 2,995/0), and summary average speed is 5 m/s (round 1: 2.5). `chained-ride-run` golden: `derived.distance` 11,974.46 and `avg_speed` 5.708. `chained-ride-ride` golden: sport `cycling` and a non-decreasing stream (`TestChainedSameSport`). `TestContinueDistance` covers 8 table cases plus input immutability and out-of-range starts. |
| QA-08-r1-4 (later `settings` sequence rejects a file whose first sequence has no `file_id`) | **Fixed** | `fileID` gates on `l.seqNo` (`listener.go:157-178`). Round 1 probe: now accepted. `TestChainedFirstSequenceGate`: 3 cases, including `FileType ""` and an empty `Device`. |
| QA-08-r1-5 (per-sample session scan; all-caps case unmeasured) | **Fixed** | `sessionAt` is gone. The timeline does O(log k) per sample, with breakpoints at Start, Start+1 and End+1 (QA checked these against rules 1, 3 and 4). `TestSportTimelineEquivalence` compares it with a direct reference on 1,000 random cases. `MEASUREMENTS.md` all-caps rows: QA reproduced normalise at 0.10 s (round 1: 0.91 s) and RSS at 229 MB. |
| QA-08-r1-6 (test gaps a–d) | **Fixed, one remainder** | (a) `TestSampleCountCorpus` checks every corpus file. The random part (1,000 cases) is in `workout`. (b) `TestLimitsAtTheCap` checks every budget at its cap and one over it. (c) `TestParseLogs` adds the time limit, 7 budgets and the position scan of the panic line. The `samples` budget is missing: QA-08-r2-2. (d) Truncation fixtures now include `run-30min` and `triathlon`. |
| QA-08-r1-7 (hygiene) | **Fixed** | `stdout.Write` error checked (`parse.go:63`). The "provisional" comment is gone (`errors.go:25`). `MEASUREMENTS.md` F-08 rows say `go1.26.9`. |

## Engineer's assumptions

| # | Assumption | Judgement |
| --- | --- | --- |
| 1 | The all-caps twin puts the 100,000 timer events in 100 bursts of 1,000 to stay under `MaxBytes`; the in-memory benchmark spreads them | **Accept.** The design's fallback was "reduce the timer events first and say so". The engineer kept the full count and changed only their placement, which is the stronger measurement. It is documented in `MEASUREMENTS.md`. `Normalise` sorts and walks the timer events once, so their placement does not change its cost. The timing comes from the in-memory file, which spreads them. QA reproduced both numbers. |
| 2 | Rule 5's tie at the earliest offset uses rule 2's tie order | **Accept.** The design does not say otherwise, and one order for all hints is the simplest reading. The reference implementation in the equivalence test uses the same order. |
| 3 | Invalid sport hints are kept by `ingest` as `RawSport{Sport: ""}` and dropped in `workout` | **Accept.** This is the design verbatim: the listener converts "like a session's (E-1: invalid → `""`)", and `workout` drops `""`, `multisport` and `all`. It is tested at both ends. |
| 4 | An empty session sport counts as a value in `S`, so a `""` session plus a hinted tail of another sport gives `multisport` | **Accept.** This follows "S = the distinct values among the `Sport` of every session … one value (may be `""`)". QA probe: a hint written *before* such a session is not used for the tail (it lies before `Eprev`, rule 2), so the tail falls back to the session's `""` and the sport stays `""`. Only a hint inside the tail produces `multisport`. That is consistent with the table. |
| 5 | The corpus part of `TestSampleCount` lives in `ingest` | **Accept.** The design permits it ("may live in `ingest`'s tests"). |

## Pending user rules (isolation check only)

- **OQ-35, R-10.E.**
  - The rule is one function, `legsElapsed` (`summary.go:58-65`), with a comment that names OQ-35 and says what to return for "wall time". Its own test is `TestLegsElapsed` (`f08_v11_test.go:496`).
  - QA listed every accepted golden with 2+ legs (sessions plus the record leg): `brick-run-bike`, `chained-ride-ride`, `chained-ride-ride-cut-75`, `chained-ride-run`, `triathlon`, `triathlon-cut-90`. These are exactly the six the design names.
  - No real file has 2+ legs, so the real goldens are unaffected.
  - Isolated as designed.
- **OQ-36, D-17.**
  - The rule is `workoutSport` (`timezone.go:35`). Its test is "2 sessions of one sport give that sport (D-17)" in `TestWorkoutSport`. Its golden is `chained-ride-ride` (the only golden with 2+ sessions of one sport; `chained-ride-ride-cut-75` has 1 session).
  - A "no" also touches the caller in `normalise.go` (QA-08-r2-3).
  - The mixed-sports summary rule stays on `S`.
  - Isolated as designed.

## Questions

- **Q1 (architect):** a **complete** chained file whose later sequence has no `session` gets no record leg: the workout is neither truncated nor session-less.
  - QA probe: cycling sequence with a session, then a complete cycling sequence with 600 records and no session. Summary distance 2,995 m and elapsed 599 s, while the data holds 5,990 m.
  - This meets AC 7 and the feature's "additive totals summed over sessions" literally. It is the same under-reporting as r1-2, but for a writer that omits sessions in a later sequence. No such writer is known.
  - What settles it: one sentence in the design. Either "accepted: complete sequences are described by their sessions", or "a record leg also covers samples of a later sequence after the last session end".
- **Q2 (architect, with QA-08-r2-1):** is any device known to continue the distance count across chained sequences? If none is, the "continues the count" row of R-12 could be dropped and every later sequence offset unconditionally. That would remove the boundary ambiguity entirely.

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 PNG → "not a FIT file" | `sequence.go` `looksLikeFIT`, `ingest.go` | `not-fit-png`, `not-fit-random`, `empty` goldens; `TestKinds`; `TestParseLogs` | Traced, passes |
| 2 Header, no records → "no readable workout data" | `ingest.go` steps 6–7 | `ride-30min-header-only`, `no-records` goldens; `TestKinds` | Traced, passes |
| 3 Course and monitoring → exactly "Unsupported FIT file type." | `listener.go` `fileID` (first sequence only, D-15), `errors.go` | `course`, `monitoring`, `workout-plan` goldens; `TestFileTypes`; `TestKinds`; `TestChainedFirstSequenceGate`; CLI `TestParseRejected` | Traced, passes |
| 4 Ride cut at 60 %: accepted, streams end at the last intact record, elapsed reported | `sequence.go` walk, `normalise.go` `DataEnd` | `ride-30min-cut-60(-size-0)`, `run-30min-cut-60`, `triathlon-cut-90`, `chained-ride-ride-cut-75` goldens; `TestTruncationPrefix` (3 bases, cadence and sport); `TestUnclosedHeader`; `TestRealTruncationLocal`; `FuzzParse` | Traced, passes. Also holds for running and multisport files and on all 396 real cuts |
| 5 Corrupted CRC, intact records → accepted | `sequence.go` CRC rules | `bad-crc`, `bad-header-crc`, `no-crc` goldens; `TestRecoveredEqualsSource`; `TestChained` | Traced, passes |
| 6 Truncated, no session → totals from records, none zero | `legs.go`, `summary.go`, `stats.go`, `elevation.go` | `ride-30min-cut-60`, `run-30min-cut-60` golden summaries; `TestSummaryRevised`; `TestRecordLeg`; `TestTruncatedRecordLeg`; `triathlon-cut-90`, `chained-ride-ride-cut-75` for partial sessions | Traced, passes, including partly surviving sessions |
| 7 Chained file and triathlon → one workout each | Sequence walk, `sequences.go` (R-12), `sports.go`, `legs.go` | `chained-ride-run`, `chained-ride-ride`, `chained-with-settings`, `triathlon` goldens; `TestChained`; `TestChainedSameSport`; `TestContinueDistance`; `TestWorkoutSport` | Traced, passes. Boundary risk in QA-08-r2-1 |
| 8 Crafted loop or allocation stopped; process keeps serving | `limits.go` (+ `sport_messages`), listener budgets, `ingest.go` slots and recover | `TestLimits` (9 cases, incl. 1,001 sport messages), `TestLimitsAtTheCap`, `TestTimeLimit`, `TestHostileDoesNotBlockOthers`, `TestSlots`, `TestPanicContained`, `FuzzParse` (QA 5 min); all-caps `MEASUREMENTS.md` rows reproduced | Traced, passes. HTTP-level check is F-09's |
| 9 Rejection and recovery log device make and model; no lat/lon | `log.go` | `TestParseLogs` (every synthetic file, 7 budgets, time, size, panic, cancel; allow-list, device keys, position scan) | Traced, passes. `samples` budget line not asserted (QA-08-r2-2) |

Also checked and found correct:
- Sport timeline:
  - Breakpoints at Start, Start+1 and End+1 cover every change of rules 1, 3 and 4.
  - `hintBefore` implements `Eprev ≤ offset ≤ t` with "at `Eprev` counts".
  - Hints of `""`, `multisport` and `all` are dropped.
  - On equal offsets, a `sport` message ranks after a lap, and arrival order is kept.
- Listener:
  - Pending hints are cleared at `beginSequence` and are never attached across sequences or from a skipped sequence.
  - They are bounded by the `sport_messages` budget.
- `SequenceStarts` has repeated values for skipped sequences, and `continueDistance` clamps out-of-range starts.
- An unclosed header (R-c) on a complete file keeps the device summary. QA probe: 6 synthetic files and 4 real files, sport and summary unchanged.
- `workout` imports no I/O package. `ingest` adds only `log/slog`.
- ADR-017 rev. 1 and ADR-018 rev. 1 match `limits.go`, the limit names and the message table.

Not run or not traceable:
- Behaviour on a real truncated file from a device, rather than cut copies, and on a real multisport or chained file. None is available locally.
- Whether multisport devices write a `sport` message per leg is still unknown, as the design says.
- The full 10-minute fuzz run was not repeated; QA ran 5 minutes.
