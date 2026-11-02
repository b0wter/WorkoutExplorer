# QA F-08 FIT validation, truncated-file recovery and parse limits - round 3

- **Date:** 2026-10-15
- **Verdict:** **ready.** No Blocker or Major finding. Every acceptance criterion (1 to 11) is traced to code and a passing test.
  - v1.2 (R-10.E wall time, D-17 confirmed, R-12 midpoint, D-20 pin) and v1.3 (R-13 transition legs, R-13.M, R-10.E restated, `recordSequences`) are built as designed. Every golden diff is inside the allowed lists.
  - 20 of 23 QA mutants are killed. The 3 survivors are test gaps. Together with two tests that are weaker than the design asked for, they make up QA-08-r3-1 (Minor, backend). That finding also leaves QA-08-r2-2 only partly fixed.
  - QA-08-r3-2 (Minor, architect): in the local real triathlon file, the distance stream falls by 100 m or more at the T1 to bike boundary, and has long near-zero runs inside the swim. This fails point 3 of the design's real-file check. v1.3 did not cause it.
  - QA-08-r3-3 (Minor, preference, backend): the reflection in `transition.go` is correct, but it does not match the codebase's `Samples.streams()` idiom. Its completeness test also ignores any stream that is not `[]float64`.
  - QA-08-r3-4 (Minor, documentation, backend): the `MEASUREMENTS.md` v1.3 rows give the wrong base commit, and the "Result" paragraph still quotes the v1.1 figures.
  - OQ-39 was answered "accept" by the user ("damaged files are a bonus not a must-have"), so the behaviour as built stands. The design still lists it as open (see Questions).
- **Inputs:**
  - Feature: `features/F-08-fit-validation-and-recovery.md` (ACs 1 to 11; the new AC 8 is the transition criterion and AC 9 is "Multisport"). `features/README.md`, decisions of 15 Oct 2026 (OQ-35 to OQ-38).
  - Architecture: `architect/F-08-fit-validation-and-recovery.md` v1.3. Sections read: R-10.E, R-12, Sport timeline, R-13, Corpus, Tests, AC table, "Changes for the backend engineer" v1.2 and v1.3. Also F-02 design v1.10, `architect/overview.md`, ADR-004 rev. 6, ADR-017 rev. 1 and ADR-018 rev. 1.
  - Spec: AP-2, AP-4, AP-5, AP-7, AP-8, AP-9 and AP-12 as amended; UP-2, UP-3; NFR Security, Reliability and Operations.
  - Earlier rounds: `qa/F-08-fit-validation-and-recovery-r1.md` and `-r2.md`.
  - State: HEAD `0ac678e` plus the uncommitted working tree (35 modified files, 6 untracked), reviewed as found. QA changed nothing under `src/`, `architect/`, `features/` or `spec/`. QA's probes and mutants ran in a scratchpad copy of `src/server` that contains no real files, plus a `git archive HEAD` copy used as the baseline.

## Build and test results

| Check | Result |
| --- | --- |
| `make -k ci` | **Exit 0.** `go vet` clean. gofmt clean. svelte-check: 256 files, 0 errors, 0 warnings. Go: 9/9 packages ok (cached). Vitest: 29 files, 786 tests passed. Size: `entry index.html: 45582 / 60000 bytes gzip (40912 brotli) PASS`. |
| `go test -count=1 -race ./...` (uncached) | 9/9 ok, no race. ingest 97.2 s, datafile 18.3 s, workout 1.8 s, corpusgen 3.7 s. |
| `make corpus` (first run) | `corpus: 47 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`. That is 42 synthetic files (41 + `ride-transition-ride`) and 5 local real files. |
| `make corpus` (after the user added `real/garmin_edge_explorer.fit` mid-round) | `47 ok, 0 diff, 0 fail, 1 new` and exit 1. The only non-ok line is `NEW real/garmin_edge_explorer.fit`. The file **parses and is accepted**: it has a data file, and the QA probe accepted it. `TestCorpus` fails only on `real/garmin_edge_explorer` with "no golden report". This is not a finding: the main session runs `corpus-update` after this round. No value from the file is recorded here. |
| `make corpus-gen` | Ran it into the scratchpad first: `diff -r` against `testdata/corpus/synthetic` shows no difference (42 files). Then ran `make corpus-gen` itself: SHA-256 of all 42 files unchanged, and `git status --porcelain` unchanged (41 entries). **Identical bytes.** |
| `BenchmarkParseAtCap -benchtime 3x -count 2 -benchmem` | parse 552–591 ms/op, 376.9 MB/op. decode 414–423 ms/op. normalise 114–116 ms/op, 98.1 MB/op. These match the new `MEASUREMENTS.md` rows. |
| QA probe: all-caps `Normalise` with **every** session a transition (the recorded benchmark has none) | 198–201 ms/op and 143.4 MB/op, against 105–117 ms/op and 94.1 MB/op for the benchmark shape. Decode plus this gives about 0.62 s of the 2 s budget. Within ADR-017's stop rule, but not recorded (QA-08-r3-4). |
| `FuzzParse`, 3 min, `-parallel 4`, scratchpad copy without real files | **PASS.** 763,302 execs, baseline 253 entries, 17 new interesting, no failure. Throughput stalled for long stretches; this looks like the fuzzer minimising large seeds, not a hang, because the run ended with PASS inside its deadline. No `testdata/fuzz/` was created in the copy or the repo. |

## Findings

### Minor

- **ID:** QA-08-r3-1
- **Severity / kind:** Minor / test gap (includes two silent deviations from the design's test table)
- **Route:** backend
- **Where:**
  - `src/server/internal/workout/moving.go:69` (gap condition in `transitionIntervals`).
  - `src/server/internal/ingest/f08_test.go:580, 618` (`TestParseLogs` "samples limit").
  - `src/server/internal/workout/f08_v13_test.go:142` (sub-sport case).
  - `src/server/internal/ingest/f08_test.go:497` (`TestChained` midpoint case).
- **Relates to:** R-13.M, D-23, QA-08-r2-2, QA-08-r2-1; ACs 7, 8 and 11.
- **Evidence:** QA ran 23 mutants against `./internal/workout` and `./internal/ingest`. Three survive:
  - **M9:** the `Δt < 5` gap condition is removed from `transitionIntervals`, the timer branch of R-13.M. Every test still passes. The design's rule ("every transition sample with `Δt < 5` counts as moving") holds in both branches, but only the speed branch has a gap case.
  - **M21:** the `samples`-limit error is stripped of `Device` and `FileType` (`ingest.go:137`). `TestParseLogs` still passes. The design (v1.2 row 5) asked for `ride-30min.fit` with `MaxSamples: 1000`: one WARN line with `kind` `limit`, `limit` `samples` and **the file's device keys**. The test instead uses `floodRecordsEvery(26, 4)`, which has no `file_id`. Its device keys are therefore empty on both sides, so the device assertion can never fail. The test also does not check that `limit` is `samples`. This was the point of QA-08-r2-2.
  - **M22:** a session with sub-sport `transition` also has its totals masked. Still passes, because the "sub-sport transition on another sport" case gives the session no totals and checks only the speed samples. The design says "nothing masked".
  - The fourth gap is not a mutant. The `TestChained` midpoint case (v1.2 row 3) uses sequences **without sessions**. It asserts only `Derived.Distance`, within ±0.1 m instead of the ±0.05 m the design gives. It leaves out the asked-for `Summary.AvgSpeed` 5.0 ± 0.01 and the "no negative step > 0.01 m" check.
  - M15 (dropping `len(legs) == 1` from `single`) also survives, but it is an **equivalent** mutant: with 2+ legs `combine` never sets `ElapsedTime`, so step 2 gives the same `Derived` value. Not a gap.
  - The engineer reported none of these deviations.
- **Consequence:**
  - A regression in the timer-branch gap rule, in the `samples` log line's device keys, or in the D-23 sub-sport rule for totals would ship unnoticed.
  - The behaviour today is correct; QA confirmed it by reading the code and by the killed mutants on the neighbouring paths.

- **ID:** QA-08-r3-2
- **Severity / kind:** Minor / risk (real-file evidence; not caused by v1.3)
- **Route:** architect
- **Where:** `architect/F-08-fit-validation-and-recovery.md`, section R-13 "Real-file check", point 3. F-02 R-6a. The F-17 and F-18 hand-offs.
- **Relates to:** AP-5 (distance axis), AP-7, AP-8, R-6a; AC 8 ("total distance … include the transitions").
- **Evidence:** QA probe on the local real triathlon file, parsed in memory, buckets only:
  - The distance stream falls by **100 m or more at the T1 to cycling boundary** (1 case).
  - Inside the open-water swim it drops to **below 1 m in 9 runs (494 samples)** after it has already passed 100 m.
  - One fall of 10 to 100 m sits at a lap start inside the bike leg.
  - R-13 never touches distance, and R-12 does nothing for a one-sequence file, so this is the writer's own data, unchanged since v1.1.
  - The summary is not affected: the distance comes from the sessions, and `derived.distance` divided by `summary.distance` rounds to 1.00.
  - The design's other real-file points hold on this file:
    - (1) No speed, cadence or power value is left at any transition sample, and every transition sample has a position.
    - (2) The summary's maximum speed, cadence and power did not rise against HEAD. `summary.elapsed_time` equals `derived.elapsed_time`. Moving time did not fall.
    - (4) The file has at least as many `sport` messages as sessions, which suggests one per leg.
    - (5) Only `ElapsedTime` changed in its summary against HEAD. The 4 real files without a transition have summaries and sports identical to HEAD.
  - No operator report on point 3 reached QA.
- **Consequence:**
  - The design's real-file check (point 3) is not met.
  - On this writer's triathlons, the distance axis, range distances and km splits (F-05, F-17, F-18) fold back by hundreds of metres. The F-17/F-18 hand-off warns only about "a small decrease".
  - What settles it: a sentence in the design that accepts large device falls as R-6a data, with the browser hand-offs widened to match, or a rule for them (a later feature). Not an F-08 code defect.

- **ID:** QA-08-r3-3
- **Severity / kind:** Minor / preference (with a small risk)
- **Route:** backend
- **Where:** `src/server/internal/workout/transition.go:75-84` (`maskTransitions`), `:90-95` (`dropTotals`); `src/server/internal/workout/f08_v13_test.go:241` (type filter in `TestTransitionAllowListComplete`).
- **Relates to:** D-21 ("any stream added later is dropped unless it is put on the list"; "a test fails if a stream … is in neither column").
- **Evidence:**
  - Correctness is fine. Mutants M1 to M4 and M23 on the tables and the mask are all killed. The completeness test fails on a missing or misspelt name in both directions. `dropTotals` zeroes pointer fields only and leaves the caller's values alone (`TestTransitionPurity`).
  - This is the **only `reflect` in production code**. Every other stream walk in `workout` goes through `Samples.streams()` with the `stream*` index constants: `timeaxis.go:24`, `gaps.go:31, 85`, `report.go:309`.
  - Both the mask and the completeness test skip any `Samples` field whose type is not `[]float64` (`!ok … continue`, and `f.Type != []float64`). A future stream of another element type would be neither masked nor flagged. That contradicts the comment at `transition.go:18-20` and D-21.
- **Consequence:**
  - The risk is low today, because every stream is `[]float64` and `streams()` forces it.
  - Two enumerations of the streams must be kept in step, and one of them is invisible to the compiler.
  - Suggested fix:
    - Key the stream table by the `stream*` constants and loop over `s.streams()`.
    - In the test, name the non-stream fields (`Offset`, `Filled`) explicitly and fail on any other field.
    - Reflection over `Totals` is reasonable, since there is no enumerator for it.

- **ID:** QA-08-r3-4
- **Severity / kind:** Minor / preference (documentation)
- **Route:** backend
- **Where:** `src/server/testdata/corpus/MEASUREMENTS.md:37-38, 41`; `src/server/internal/workout/sequences.go:13`.
- **Relates to:** ADR-017 rev. 1 item 7, v1.3 row 10.
- **Evidence:**
  - The v1.3 rows say "2ca94a8 + F-08 v1.3 (uncommitted)". v1.1 was committed as `0ac678e`, which is the base of this work tree.
  - The "Result" paragraph still states "normalise 0.10 s … = 0.55 s". The v1.3 rows give 0.12 s and 0.57 s.
  - The worst case for R-13, every session a transition (0.20 s, 143 MB/op for `Normalise`), is not recorded.
  - The `continueDistance` comment runs two sentences into one 120-character line.
- **Consequence:** a reader of the measurements takes the wrong baseline and the old figures. Cosmetic otherwise.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-08-r2-1 (R-12 has no tolerance at a boundary) | **Fixed** | `sequences.go:39` `first < prev/2`. `TestContinueDistance` has the 0.01 m, 0.4×, 0.6×, exactly-half and `prev = 0` cases. Mutants M19 (`< prev`) and M20 (`<=`) are killed. QA's r2 probe shape through `Parse` gives 5,989.99 m (`TestChained`, which is weaker than designed: QA-08-r3-1). |
| QA-08-r2-2 (`samples` log line not asserted; dead `skipOverTest`) | **Partly fixed** | `skipOverTest` is removed. A `samples` case was added, but its input has no device, so the device-key assertion is vacuous (mutant M21 survives) → QA-08-r3-1. |
| QA-08-r2-3 (stale F-02 line 57; `workoutSport` comment) | **Fixed** | F-02 v1.9 marks the paragraph as history (line 59) and the R-9 bullet (line 278). The `timezone.go:31-33` comment is reworded. |
| r2 Q1 (complete later sequence without a session) | **Resolved by design** (D-20) | `TestRecordLegCompleteChained` asserts `Distance` 90 and elapsed 109. Mutant M14 (no sequence count) is killed. |
| r2 Q2 (always offset later sequences) | **Resolved by design** (D-14 v1.2, rejected with reasons) | — |

## Engineer's choices

| Choice | Judgement |
| --- | --- |
| Reflection in `maskTransitions` and `dropTotals`, keyed by field name, plus `TestTransitionAllowListComplete` | **Accept with QA-08-r3-3.** It is correct, and the design allows "keyed by stream and by `Totals` field". But it departs from the `streams()` idiom and has a type-filter blind spot. |
| Heart rate in the `ride-transition-ride.fit` transition | **Accept.** The corpus row lists heart rate among the transition records. It proves that heart rate is kept (min 130 and session averages present). |
| `TestTruncationPrefix` reference built in the test (`recordedSamples` renames `transition` to `generic`), plus cuts at every `len/1000` | **Accept.** It is what the design asks for. The rename changes no stream: R-5 doubles only running, walking and hiking. Mutant M5 (hints make transitions) is killed by this test, so OQ-39's behaviour is pinned. |
| `MEASUREMENTS.md` commit label | **Wrong base** → QA-08-r3-4. |
| Not reported: the `TestParseLogs` `samples` input and the `TestChained` midpoint case deviate from the design's test table | → QA-08-r3-1. |

## Golden diffs (checked by hand against the allowed lists)

- **v1.2 row 7:** `brick-run-bike`, `chained-ride-run`, `chained-ride-ride`, `chained-ride-ride-cut-75`, `triathlon` and `triathlon-cut-90` change `summary.elapsed_time` only. The new values are 3,660, 2,998, 1,498, 1,199, 3,719 and 3,359. A script confirmed that `summary.elapsed_time == derived.elapsed_time` for **every** accepted synthetic golden (34 files). In the data files, the metadata length changes by the digit count only (for example +1 byte for 899 → 1,199), and the stream offsets shift accordingly.
- **v1.3 row 8:**
  - `triathlon.json` and `triathlon-cut-90.json`: `avg_speed` and `max_speed` are removed from 4 totals each (T1 and T2 sessions and laps). The `speed` stream goes from present 3,720 to 3,600 and from 3,360 to 3,240, with 120 missing; its mean and digest change, and its min, max, first and last do not. Nothing else changes: `derived`, `sport`, `integrity`, `samples` and the other streams are byte-identical.
  - The data files show the same in data-file form: the metadata is 128 bytes shorter (4 × 32 bytes of `"avg_speed":1.5,"max_speed":1.5,`), and the speed column has `valid` 3,600 and `missing` 120.
- **New: `ride-transition-ride`.** It matches its corpus row on every point:
  - `multisport`, 3 sessions, integrity clean.
  - Speed, cadence and power are missing at exactly 60 samples (600–659). Position, altitude, distance, heart rate and temperature are present at all 1,260.
  - The transition session and lap have no speed, cadence or power keys.
  - `summary.max_speed` is 8.0 (< 12), `max_power` 240 (< 400) and `derived.max_cadence` 92 (< 100). `max_temperature` is 30 in both the summary and `derived`.
  - `derived.moving_time` is 1,259 (R-13.M; mutant M7 gives 1,199 and fails `TestCorpus`). `summary.elapsed_time` is 1,259.
  - `summary.distance` 9,079.5 is the sum of the 3 sessions.
  - There is no summary `avg_speed`, `avg_cadence` or `max_cadence`.
  - The data file's metadata drops the same keys from the transition totals.
- No other synthetic `.fit`, report or data file changed. Every `.fit` except the new one is byte-identical (`make corpus-gen`).

## Mutation probes (`go test ./internal/workout ./internal/ingest`, scratchpad copy)

| # | Mutant | Result |
| --- | --- | --- |
| M1 | `Speed` on the keep list | Killed (TestTransitionSamples, TestCorpus, …) |
| M2 | `Temperature` dropped | Killed (TestTransitionSamples, TestCorpus, TestTruncationPrefix) |
| M3 | `dropTotals` a no-op | Killed |
| M4 | `AvgHeartRate` dropped from totals | Killed (TestCorpus only) |
| M5 | `inTransition` = `sportAt(t) == transition` (hints count) | Killed (TestTransitionWhichSessions, TestTruncationPrefix) |
| M6 | `inTransition` = any covering session, not the first | Killed (overlap case) |
| M7 | R-13.M speed branch removed | Killed (TestTransitionMovingTime, TestCorpus) |
| M8 | R-13.M timer branch removed | Killed |
| M9 | Timer branch ignores `Δt < 5` | **Survived** → r3-1 |
| M10 | Union replaced by a plain sum | Killed |
| M11 | Lap placed by `End`, not `Start` | Killed |
| M12 | Mask after `settlePresence` | Killed |
| M13 | `single` always false | Killed |
| M14 | `single` without `recordSequences` | Killed |
| M15 | `single` without `len(legs) == 1` | Survived, **equivalent** (see r3-1) |
| M16 | `wallElapsed` aliases `Derived` | Killed |
| M17 | `recordSequences` ignores invalid times | Killed |
| M18 | `recordSequences` = number of ranges | Killed |
| M19 | Midpoint → `first < prev` | Killed |
| M20 | Midpoint → `<=` | Killed |
| M21 | `samples` error without device | **Survived** → r3-1 |
| M22 | Sessions masked by sub-sport too | **Survived** → r3-1 |
| M23 | Sample mask skipped | Killed |

## Questions

- **Q1 (architect):** OQ-39 was answered by the user on 15 Oct 2026: "accept" ("damaged files are a bonus not a must-have"). The design v1.3 still lists it under Open questions, the header says "listed for confirmation", and `features/README.md` names it in Waiting on. What settles it: move OQ-39 to Resolved questions. No code change; `TestTruncationPrefix` already pins the behaviour.
- **Q2 (architect, with QA-08-r3-2):** do large device distance falls (hundreds of metres at a leg boundary, near-zero runs in an open-water swim) count as R-6a device data that the browser must tolerate? Or do they need a rule (for example, per-session continuation in one sequence, like R-12)? What settles it: one sentence in F-08 or F-02, and the F-17/F-18 hand-off widened from "a small decrease" if the answer is "tolerate".

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 PNG → "not a FIT file" | `sequence.go` `looksLikeFIT`, `ingest.go` | `not-fit-png`, `not-fit-random`, `empty` goldens; `TestKinds`; `TestParseLogs` | Traced, passes (unchanged since r2) |
| 2 Header, no records → "no readable workout data" | `ingest.go` steps 6–7 | `ride-30min-header-only`, `no-records` goldens; `TestKinds` | Traced, passes |
| 3 Course and monitoring → "Unsupported FIT file type." | `listener.go` `fileID`, `errors.go` | `course`, `monitoring`, `workout-plan` goldens; `TestFileTypes`; `TestKinds` | Traced, passes |
| 4 Ride cut at 60 %, streams end at the last intact record, elapsed reported | sequence walk, `normalise.go` `DataEnd` | `ride-30min-cut-60(-size-0)`, `run-30min-cut-60`, `triathlon-cut-90`, `chained-ride-ride-cut-75` goldens; `TestTruncationPrefix` (4 bases now, including 1,000 fine cuts of `ride-transition-ride` with at least one inside the transition) | Traced, passes |
| 5 Corrupted CRC, intact records → accepted | `sequence.go` CRC rules | `bad-crc`, `bad-header-crc`, `no-crc` goldens; `TestRecoveredEqualsSource` | Traced, passes |
| 6 Truncated, no session → totals from records | `legs.go`, `summary.go` | `ride-30min-cut-60`, `run-30min-cut-60` summaries; `TestRecordLeg` (updated to wall time) | Traced, passes |
| 7 Chained file and triathlon → one workout each | sequence walk, `sequences.go` (R-12 midpoint, `recordSequences`), `summary.go` (R-10.E) | `chained-*`, `triathlon`, `brick-run-bike` goldens; `TestChained`; `TestContinueDistance`; `TestLegsElapsed`; `TestRecordSequences`; `TestRecordLegCompleteChained` | Traced, passes (TestChained midpoint case weaker than designed, r3-1) |
| **8** Transition samples keep position, distance, HR, elevation, temperature; no speed, cadence, power; total distance and moving time include transitions; speed, cadence and power averages and maxima exclude them | `transition.go` (`maskTransitions`, `dropTotals`), `sports.go` `inTransition`, `normalise.go` order, `moving.go` R-13.M | `triathlon` and `triathlon-cut-90` goldens and data files; `ride-transition-ride` golden and data file; `TestTransitionSamples`, `…FilledSampleAndAbsentStream`, `…WhichSessions`, `…MovingTime`, `…AllowListComplete`, `…Purity`; `checkTransitions` in `TestCorpus`; `TestTruncationPrefix` | Traced, passes. Real triathlon: points 1, 2, 4 and 5 hold; point 3 fails on device data (r3-2) |
| **9** Ride, transition, ride → "Multisport" | `timezone.go` `workoutSport` over `S` (D-17 unchanged) | `ride-transition-ride` golden `sport: multisport`; `TestTransitionSamples` sport assertion; `TestTruncationPrefix` sport per cut | Traced, passes |
| 10 Crafted loop or allocation stopped; process keeps serving | `limits.go`, listener budgets, `ingest.go` | `TestLimits`, `TestLimitsAtTheCap`, `TestTimeLimit`, `TestHostileDoesNotBlockOthers`, `TestSlots`, `TestPanicContained`, `FuzzParse` | Traced, passes. HTTP-level check belongs to F-09 |
| 11 Rejection and recovery log device make and model; no lat/lon | `log.go` | `TestParseLogs` | Traced, passes; `samples` line device keys not really asserted (r3-1) |

Also checked and found correct:
- `workout` still imports no I/O package. `reflect` is new, and it is pure.
- `Normalise` does not modify its input: `TestTransitionPurity`, and `w.Sessions` is a copy.
- The timer union equals the old sum when there are no transitions: the timer spans never overlap, and negative offsets are handled by `started`.
- `inTransition` reuses the precomputed `cover` at O(log k) per lookup.
- R-13 runs after R-3 and before R-4 (M12 is killed).
- Commit hygiene:
  - Every tracked or untracked path is code, design, or synthetic corpus, golden or data file.
  - `git ls-files` lists only the two `.gitkeep` files under `real/`.
  - The 6 real `.fit` files, the 5 real goldens and `build/` are ignored (`.gitignore:9-12`), the new `garmin_edge_explorer.fit` included.
  - No `testdata/fuzz/` directory exists. `MEASUREMENTS.md` and `README.md` contain no real-file figures.

Not run or not traceable:
- The operator's own real-file report (design row 9). QA probed the real files in memory instead (QA-08-r3-2) and records only booleans and buckets.
- The 10-minute fuzz run; QA ran 3 minutes.
- The new real file has no golden yet (main session).
