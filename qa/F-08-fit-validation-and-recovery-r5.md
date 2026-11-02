# QA F-08 FIT validation, truncated-file recovery and parse limits - round 5

- **Date:** 2026-10-15
- **Verdict:** **ready.** There is no Blocker or Major finding. Both r4 findings are fixed, and r4 Q1 is answered by a new synthetic file. That file pins R-14 end to end and makes the CI distance invariant able to fail. Two cosmetic Minor findings remain; neither blocks.
- **Scope:** a short round, covering only the r4 follow-ups. ACs 1 to 9, 11 and 12 are unchanged since r4 and were not re-traced.
- **Inputs:**
  - Design: `architect/F-08-fit-validation-and-recovery.md` v1.5. Sections read: the status line, change-log row 1.5, R-14 "Order against R-3", corpus row v1.5, the Tests rows (R-14 and `TestCorpus`), the AC table, and "Changes for the backend engineer (v1.4 follow-up)" rows 1 to 4. Also the F-08 row and the QA-08-r3-2 row of `architect/INDEX.md`.
  - Feature: `features/F-08-fit-validation-and-recovery.md`, AC order only.
  - Earlier round: `qa/F-08-fit-validation-and-recovery-r4.md`.
  - State: base `bd50240` plus the uncommitted working tree (6 modified files and 4 untracked files, all listed in `git status`).
  - QA changed nothing outside `qa/`. Mutants and probes ran through `go test -overlay`, with the replacement files in the scratchpad.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Every Go package `ok`. Bundle 45,582 / 60,000 B gzip, PASS. |
| `make corpus` | `corpus: 49 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`: 43 synthetic files (42 + 1) and 6 local real files. `swim-bike-distance-fall.fit` ok. |
| `go run ./tools/corpusgen -out <scratch>`, then `diff -r` against `testdata/corpus/synthetic` | identical, 43 files |
| `make corpus-gen` | `git status` unchanged afterwards. Every existing `.fit` is byte-identical to `bd50240`; only the new file is added. |
| `gofmt -l tools/corpusgen internal/workout` | empty |

## Mutation probes (overlay on `internal/workout/normalise.go`)

| Mutant | Result |
| --- | --- |
| **H3**: `holdDistance` before `fillGaps` | `TestHoldDistanceOrder` FAILS: `distance = [100 100 125 150 175 200], want [100 100 100 125 162.5 200]`. Killed (it survived r4). |
| `holdDistance` removed, `internal/workout` | FAIL in `TestHoldDistance`, `...FilledSample`, `...Derived`, `...ChainedBoundary` and `...Order`. `TestHoldDistanceDerived` now reports `summary distance = 300, want 400 (session 300 + leg 100)`, so the record-leg assertion is no longer passed by the mutant. |
| `holdDistance` removed, `TestCorpus` | FAIL on `synthetic/swim-bike-distance-fall`: `R-14: distance falls at sample 200`, and its report differs. The two local real files also fail, but CI does not have them; the synthetic file alone makes CI fail. |

## New golden against the corpus row (design v1.5)

- **Report** (`expected/synthetic/swim-bike-distance-fall.json`):
  - accepted, `integrity` clean, `sequences` 1, 2 sessions, sport `multisport`;
  - `summary.distance` = `derived.distance` = 3,545;
  - `moving_time_source` `timer`;
  - no `avg_speed` in the summary;
  - session totals 599 and 2,946.

  This matches the row.
- **Data file** (`datafile/synthetic/swim-bike-distance-fall.bin`): QA decoded it with `datafile.Decode` (a scratch overlay test) and compared all 1,200 distance values with the row's formula. There were **0 mismatches**:
  - 199 at 200–219;
  - 599 at 600–609;
  - 600, 605, ... from 610;
  - the recorded value everywhere else.

  The report's distance mean (1,173.55) also agrees with the held stream; without the hold it would be 1,170.02.
- **Generator** (`tools/corpusgen/f08.go`, `swimBikeDistanceFall`) is as the row describes:
  - the swim writes `d = t`, with 0.5 from 200 to 219;
  - the bike writes `550 + 5 × (t − 600)`;
  - 1.0 and 5.0 m/s, positions and heart rate, timer at 0 and 1,199;
  - no sport hints;
  - `+2 h` offset.

## `sessionDist` isolation

- `sessionMesg` (`tools/corpusgen/main.go:927`) reads `f.sessionDist[sp.from]`. For every other generator the map is nil, so the lookup gives `ok == false` and the code path is unchanged.
- This is confirmed empirically: all 42 existing files regenerate byte-identical.
- The override also sets the session `avg_speed` to `v / timer`, which is consistent.
- Seeds PCG(71, 72) and start day +26 are not used by any other generator.

## Commit hygiene

- Nothing is staged.
- The 4 untracked files are all synthetic: `.fit`, report JSON, and the data-file `.bin` and `.json`. `.gitignore:18` allows the `.bin` (`!/src/server/testdata/datafile/synthetic/*.bin`).
- No real or derived-from-real file is tracked. The real `.fit` files and their reports show as ignored (`!!`).
- The README manifest row says "synthetic, no personal data".

## Findings

- **ID:** QA-08-r5-1
- **Severity / kind:** Minor / documentation (test comment)
- **Route:** backend
- **Where:** `src/server/internal/workout/f08_v14_test.go:134-135`
- **Relates to:** QA-08-r4-1, design v1.5 row 1
- **Evidence:**
  - The comment says that holding before the fill "interpolates 100 -> 200 and gives [100 100 100 125 162.5 200] only if the hold kept 50 at 1 s; it does not". That vector is the *correct* result.
  - The H3 mutant actually gives `[100 100 125 150 175 200]` (see Mutation probes above), as the design's worked example says.
- **Consequence:** the comment misleads the next reader about what the test distinguishes. The test itself is correct. Fix: state the mutant's output, `[100 100 125 150 175 200]`.

- **ID:** QA-08-r5-2
- **Severity / kind:** Minor / preference (fixture consistency)
- **Route:** architect (whether the corpus row should say so), backend if the lap is to change
- **Where:** `src/server/tools/corpusgen/main.go:927` (session-only override); `testdata/corpus/expected/synthetic/swim-bike-distance-fall.json:76`
- **Relates to:** design v1.5 corpus row `swim-bike-distance-fall.fit`
- **Evidence:**
  - The bike session and its only lap cover the same span, 600–1,199.
  - The session says 2,946 m (the override); the lap says 2,995 m (last − first record, 3,545 − 550). `lapMesg` still uses `computeStats`.
  - The corpus row says nothing about lap totals.
- **Consequence:** none for R-14, which does not touch totals. But the fixture has a writer pattern no device produces: a one-lap session whose lap is 49 m longer than the session. A later lap-table feature or a "laps sum to session" check would inherit it as a golden. Either override the lap too, or note the difference in the corpus row. Evidence that would close this: the architect states that the lap difference is intended.

## Previous findings

| r4 item | Status |
| --- | --- |
| QA-08-r4-1 (order not pinned; record-leg `>= 300` passes without R-14) | **Fixed.** `TestHoldDistanceOrder` kills H3. The record leg asserts `== 400` and fails without `holdDistance`. |
| QA-08-r4-2 (stale status line; real-file counts in the design) | **Fixed.** The status line says R-14 is built and that v1.5 is pending. The R-14 Problem paragraph, the v1.4 change-log row and the `INDEX.md` QA-08-r3-2 row no longer quote counts ("100 m or more", "9 runs", "494" are gone). The only remaining "below 1 m after 150 m" is a synthetic unit-test case. |
| Q1 (CI distance invariant vacuous) | **Answered.** There is a new corpus row and file, and the invariant now fails in CI without R-14. |
| AC table numbering | Matches the feature file's numbering: distance is 10, limits 11, logs 12. The design explains the renumbering in a note. |

## Questions

None.

## Coverage

- **AC 10** is now traced to `holdDistance`, the 7 `TestHoldDistance*` tests, `TestChained`, the `TestCorpus` invariant (no longer vacuous in CI) and the `swim-bike-distance-fall.fit` report and data-file goldens. All of them pass. Without R-14, five of the seven unit tests fail, and so does the new golden; `...PositionsOnly` and `...InputUnchanged` are not meant to depend on R-14.
- The other ACs are unchanged since r4 and were not re-traced.
- **Not run:** `go test -race` (the change is test and generator code only), `FuzzParse` beyond its seeds, and the benchmarks.
