# QA F-02 FIT parsing and normalisation - round 3

- **Date:** 2026-10-10
- **Verdict:** ready with conditions.
  - QA-02-r2-1 is fixed.
  - The user decision "real FIT files never in the repo" is implemented correctly. Every real file and real report on this machine is git-ignored, and all 17 synthetic files and 17 synthetic reports can still be added. A clean copy without real files passes `make corpus` and `go test`.
  - The renumbered acceptance criteria are consistent across the feature file, spec, design, ADRs, READMEs and test names.
  - No regressions.
  - One new Minor finding: a mixed-case extension (`.Fit`) is not ignored outside `real/`.
  - What keeps this from "ready" is AC 2: real files from three FIT writers (OQ-6, user condition). Only one writer is available so far.
- **Inputs:**
  - Feature file: AC 1 is now split into AC 1 (synthetic, CI) and AC 2 (real, operator's machine); later ACs move down one place.
  - `architect/F-02-fit-parsing-and-normalisation.md` v1.5: change log 1.4 and 1.5, AC table, Required files, Open questions.
  - ADR-004 revision 2 (item 7), ADR-003, `architect/INDEX.md`.
  - TD corpus sentence (line 102), `features/README.md` (test-corpus bullet), F-08 scope.
  - `.gitignore`, `qa/F-02-fit-parsing-and-normalisation-r2.md`.
  - Code: the uncommitted working tree on top of `e415d92`.
  - Two real files present locally and git-ignored: `real/development-watch7-5-{run,ride}.fit` with their reports.

**Privacy note for this report.** As agreed with the coordinator, this report names manufacturers and pass/fail only. No coordinates, times, dates, places or totals from the real files appear here. Scratch copies that held real files were deleted after the probes.

## Build and test results

Environment as in r2: Go `go1.26.8 linux/amd64`, Node `v22.22.2`, no remote.

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | exit 0. `go test ./...` passes for all 5 packages. Vitest 23/23 passed. Bundle `10613 / 60000 bytes gzip PASS` |
| `go test -race -count=1 ./...` | all `ok` |
| `go vet ./...`, `gofmt -l .`, `go mod tidy -diff` | clean |
| `make corpus` (this machine, with 2 real files) | `real: 2 files (local only)`, then `corpus: 19 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` |
| `go test -v ./internal/ingest -run '^TestCorpus$'` | PASS; logs `real: 2 files (local only)` |
| `git status --short --ignored` before and after the whole round | identical |

**Clean copy without real files.** I copied only `git ls-files -co --exclude-standard` (tracked files plus untracked files that are not ignored) and `.git` into the scratchpad.
- `real/` and `expected/real/` contain only `.gitkeep`.
- `make corpus` prints `real: 0 files (local only)` and `corpus: 17 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`.
- `go test ./...` passes for all 5 packages, including `TestCorpus` and `TestCorpusFilesEmptyReal`.

**Git-ignore checks.**

| Check | Result |
| --- | --- |
| `git check-ignore -v` on both real `.fit` files | ignored by `.gitignore:11` (`/src/server/testdata/corpus/real/*`) |
| `git check-ignore -v` on both real `.json` reports | ignored by `.gitignore:13` (`/src/server/testdata/corpus/expected/real/*`) |
| `git check-ignore -v` on both `.gitkeep` files | re-included by lines 12 and 14 |
| `git add -n -A .` | lists only the two `.gitkeep` files under `real/` and `expected/real/`, plus 17 `synthetic/*.fit` and 17 `expected/synthetic/*.json`. No real file, no real report, and no `.fit` outside `synthetic/` |
| `git status --short --ignored` | `!!` for the 2 real files, the 2 real reports, `build/` and `node_modules/` |
| Hypothetical paths (`check-ignore --no-index`) | `ride.fit` and `Ride.FIT`: ignored. `real/x.Fit` and `real/sub/x.txt`: ignored. `synthetic/Foo.FIT` and `synthetic/sub/x.fit`: ignored. `build/corpus/real/x.json`: ignored. **`Ride.Fit` (outside `real/`): not ignored** (QA-02-r3-1) |
| Leak check | Search over every non-ignored file in the repo for the real reports' start times, start dates, latitude/longitude statistics and digests (26 values): **0 hits** |

**Mutation probes.** Each ran on a scratch copy that included the local real files, with `corpus.sh` and `go test ./internal/...`.

| Mutation | `make corpus` | Failing tests |
| --- | --- | --- |
| E-8 removed for `activity.local_timestamp` (r2 survivor) | 0 DIFF | `TestSystemTimeActivityUnknownZone/local_timestamp_in_system_time` |
| E-8 removed for `activity.timestamp` (r2 survivor) | 0 DIFF | `TestSystemTimeActivityUnknownZone/timestamp_in_system_time` |
| E-8 removed for both `activity` fields (r2 survivor) | 0 DIFF | all three subtests |
| E-8 predicate, record, event and `span` sites | 1, 1, 1, 0 DIFF | caught as in r2 |
| Thresholds 1.05 and 0.6 km/h, `>`; `tzStep` 1800 | 0 DIFF | `TestMovingTimeThresholds`; `TestTimeZone` (four cases) |
| H 2.5; enhanced/legacy swap; `maxFillGap` 5; same-second merge; `multisport`; semicircle divisor; cadence not doubled | 2, 14, 1, 0, 1, 16, 3 DIFF | all caught. The real files add DIFFs for H, the semicircle divisor and cadence. The acceptance test names now carry the new numbers (`AC5_enhanced…`, `AC7_flat…`, `AC9_smart…`, `AC11_multi-session`) |

**The engineer's claim about lap times in the real files.** I wrote a separate probe that decodes the raw `lap`, `session` and `event` messages and prints counts only.
- Run file: 7 of 7 laps have raw `total_elapsed_time == total_timer_time`.
- Ride file: 3 of 3 laps have the same.
- In both files, our report's lap `elapsed_time` and `timer_time` equal the raw values exactly (7/7 and 3/3).
- The session messages differ (elapsed ≠ timer), and the files contain timer events.
- In the ride file, for 2 of the 3 laps, `timestamp − start_time` does not match `total_elapsed_time`. So the laps contain pauses, but the writing app put timer time into the elapsed field.

**Confirmed: the equal lap times are the app's data, not our parsing.** Keeping them as recorded is correct (AP-2, decision 5).

## Findings

No Blocker or Major findings.

### Minor

- **ID:** QA-02-r3-1
- **Severity / kind:** Minor / risk
- **Route:** architect (ADR-004 rev. 2 item 7 fixes the exact `.gitignore` lines)
- **Where:** `.gitignore:8-9` (`*.fit`, `*.FIT`)
- **Relates to:** user decision "real FIT files are never in the repository" (ADR-004 rev. 2, feature file Scope, TD line 102)
- **Evidence:** `git check-ignore --no-index Ride.Fit` → not ignored. Git matches case-sensitively here, and the two patterns cover only all-lower and all-upper extensions. Inside `real/` every name is ignored, so the gap exists only elsewhere in the tree (repo root, `Downloads`-style folders, `build/` excepted).
- **Consequence:** A real file with a mixed-case extension placed anywhere outside `real/` would be picked up by `git add -A`, against an explicit user decision. I have not seen a device or app that writes `.Fit`, so the likelihood is low. The cost of closing the gap is one pattern (`*.[fF][iI][tT]` instead of the two lines). The current re-include `!/…/synthetic/*.fit` keeps working with it.
- **What would change my mind:** a statement that only `.fit` and `.FIT` occur in practice, recorded in the ADR.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-02-r2-1 (E-8 at the `activity` site not pinned) | **Fixed** | `TestSystemTimeActivityUnknownZone` (`listener_test.go:325-345`) has three cases: both fields in system time, only `timestamp`, only `local_timestamp`. The single-field cases sit just across the `0x10000000` boundary, so their difference is small enough to pass R-9's range if E-8 were missing. Each of the three mutations fails exactly the expected subtests (table above). My r2 probe case is the "both" case |

## Questions

1. **For the architect (F-03, F-04, F-32), not blocking F-02: outputs derived from real files.** ADR-004 rev. 2 item 7 covers real files, their golden reports, and F-08's derived broken files (they "must be derived from synthetic files if they are to be committed"). It does not yet cover outputs that later features derive from "every corpus file":
   - F-03: `data.bin` per file and "the measured sizes for the corpus are recorded".
   - F-04: "the F-03 round-trip fixture for every corpus file".
   - F-32: the end-to-end sweep.

   A `data.bin` or fixture from a real file holds the full route. A size table that names real files says little, but it is still a per-file record. To settle it: one sentence in ADR-004 (or in the F-03 design when it is written) saying that every artefact derived from a real file is local only, and that committed fixtures and measurement rows come from synthetic files only.

## User conditions

- **AC 2 / OQ-6: real files from at least three distinct FIT writers.**
  - Present now: one writer, manufacturer `development` (an Apple Watch recorded through a third-party app), with two files. Both pass `make corpus` (ok) and parse without error.
  - Two more writers are needed.
  - Note for the user: `development` is the generic FIT manufacturer value that many third-party apps write, and our synthetic files use it too. Under the AC 2 rule ("distinct `device.manufacturer` values"), a second app that also writes `development` does not count as a second writer. Files written by a device maker's own software (for example Garmin, Wahoo, Coros, Suunto, Polar) do count.
  - When the files are added, QA verifies AC 2 on this machine as design v1.5 describes and lists only the manufacturers.
- **OQ-4:** the moving-time default for other sports. Not blocking; it is pinned by `TestMovingTimeThresholds`, so that test must be updated together with any decision.
- **OQ-5:** the FIT licence, to settle before launch (F-29, F-32).
- **From F-01:** a real GitHub Actions run (with the Go cache on), Node 24, and the `@v7` tags. None can be checked here because there is no remote.

## Coverage

**Acceptance criteria after renumbering (feature file → design table → code and tests).**

| AC | Status |
| --- | --- |
| 1 Synthetic corpus | Traced: 17 files, `TestCorpus` and `TestSyntheticCorpusCurrent` in CI |
| 2 Real corpus | User condition, 1 of 3 writers (see above) |
| 3 `make corpus` | Traced: run clean on this machine and on the clean copy; mutations show `DIFF` |
| 4 Semicircles | Traced |
| 5 Enhanced fields | Traced: `AC5_…` test |
| 6 Moving time | Traced: `AC6_…` tests and `TestMovingTimeThresholds` |
| 7 Elevation gain | Traced: `AC7_…` tests |
| 8 Distance | Traced: `AC8_…` tests |
| 9 Smart recording | Traced: `AC9_…` test |
| 10 Unknown messages | Traced: `AC10_…` test |
| 11 Multi-session | Traced: `AC11_…` test |
| 12 Measurements | Traced: `MEASUREMENTS.md`, now cited as AC 12 |

**Renumbering consistency.** The new numbers are used in:
- the design's AC table and Required-files table;
- ADR-004 (lines 11 and 20: AC 3 and AC 12) and ADR-003 (lines 35 and 41: AC 5; line 50: AC 2);
- `testdata/corpus/README.md` (AC 1 to AC 12 rows) and `MEASUREMENTS.md` (AC 12);
- the code: `acceptance_test.go` subtest names and the comment in `corpus_test.go`.

The TD (line 102), `features/README.md` (test-corpus bullet) and the F-08 scope (broken files derived from synthetic files) agree with ADR-004 rev. 2. Old numbers remain only where they are history: design change-log entries 1.0 to 1.4 (stated as intentional in 1.5), the `architect/INDEX.md` "Resolved questions" row "F-02 impl Q1" (AC 4), and my r1 and r2 reports. That is acceptable.

**Real-file handling in code.**
- Both drivers print the real-file count.
- An empty `real/` passes (`TestCorpusFilesEmptyReal`, and the clean copy).
- The naming rule from r1 still applies to `real/`.
- A real file copied into `synthetic/` would fail `TestSyntheticCorpusCurrent` ("committed but not generated").
- Nothing prevents `git add -f`. The README forbids it, and a CI check would fire only after the push, so I am not raising it.

**Regressions.** None. The r1 and r2 mutation set is still caught, the synthetic corpus output is unchanged (17 ok), and there is no F-03 or F-08 code.

**Not run.**
- AC 2 with three writers (the files do not exist yet).
- `make corpus-update` (it rewrites `expected/`).
- A GitHub Actions run.
