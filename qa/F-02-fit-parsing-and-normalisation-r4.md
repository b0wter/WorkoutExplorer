# QA F-02 FIT parsing and normalisation - round 4

- **Date:** 2026-10-10
- **Verdict:** ready with conditions.
  - QA-02-r3-1 is fixed, and my r3 question is settled in ADR-004 revision 3.
  - The third real file (manufacturer `wahoo_fitness`) is git-ignored. It parses, and its report is consistent with the device's own totals.
  - No identifying value from any of the three real files appears in a non-ignored file.
  - No findings are open and I found no regressions.
  - The only thing keeping this from "ready" is AC 2: one more FIT writer is needed (OQ-6, user condition).
- **Inputs:** `architect/F-02-fit-parsing-and-normalisation.md` v1.6 (change log), ADR-004 revision 3 (item 7, `.gitignore` block), `.gitignore`, `qa/F-02-fit-parsing-and-normalisation-r3.md`. Working tree on top of `e415d92`. Three real files are present locally and git-ignored: two from manufacturer `development`, one from `wahoo_fitness`.

**Privacy note for this report.** It names manufacturers, pass/fail results, and ratios between our derived values and the device's totals. It contains no coordinates, times, dates, places or absolute totals from real files. Scratch copies that held real files were deleted after the probes.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | exit 0. `go test ./...` passes for all 5 packages. Vitest 23/23 passed. Bundle `10613 / 60000 bytes gzip PASS` |
| `go test -race -count=1 ./...` | all `ok` |
| `go vet ./...`, `gofmt -l .`, `go mod tidy -diff` | clean |
| `make corpus` (this machine) | exit 0, `real: 3 files (local only)`, `corpus: 20 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` |
| Clean copy (`git ls-files -co --exclude-standard` plus `.git`) | 0 FIT files outside `synthetic/`. `make corpus`: `real: 0 files`, `17 ok`, no failures. `go test ./...`: 5 of 5 `ok` |
| `git status --short --ignored` before and after the round | identical |

**Git-ignore checks** (`git check-ignore --no-index` on example paths, plus `git add -n -A .`):

| Path | Result |
| --- | --- |
| `Ride.Fit`, `ride.fIt`, `x/y/RIDE.fiT`, `ride.fit`, `Ride.FIT` | all ignored (r3 gap closed) |
| `synthetic/a.fit` | **not** ignored, so it can be added |
| `synthetic/A.Fit`, `synthetic/sub/a.fit` | ignored (only lower-case top-level synthetic files are re-included, matching the naming rule) |
| `real/a.txt`, `expected/real/a.json` | ignored |
| `expected/synthetic/a.json` | not ignored |
| Wahoo `.fit` and its report | ignored by `.gitignore:10` and `:12` |
| `git add -n -A .` | 17 `synthetic/*.fit` and 17 `expected/synthetic/*.json`, plus the two `.gitkeep` files. No real file, no real report, and no FIT file anywhere else |

**Leak search.** I collected 54 identifying values from the three real reports: start times, start dates, the `offsets_digest`, latitude and longitude statistics, and the position, altitude, distance and speed digests. A fixed-string search over every non-ignored file found **0 hits**. That includes `qa/`, `architect/`, `features/` and the corpus README.

**Plausibility of the Wahoo report.** These are structure checks and ratios only.
- `file_type` is `activity`, manufacturer is `wahoo_fitness`, sport is `cycling`. One session, one lap. The time zone is known and a multiple of 900 s.
- Streams present: latitude, longitude, altitude, distance, speed, temperature. Heart rate, power and cadence are absent, which matches "no sensors" (AP-4: only streams that are present).
- Missing-sample ratios: positions, distance and speed 0.1 %, altitude 0.8 % (the altitude dropouts the engineer reported). 4 samples are filled and 9 gaps are kept.
- Timer events alternate start and stop and end with two stops. Moving time comes from the timer (`timer` source), and the ratio of our moving time to the device's `total_timer_time` is **1.0000**. This verifies R-7 on a real file, including the ignored repeated stop.
- Derived distance / device distance = 0.9999.
- `avg_speed × timer_time / distance` = 1.000, so the scaling of device totals is consistent.
- Derived elevation gain / device ascent = 1.086. The summary shows the device's ascent and distance, not the derived ones (R-10, AP-2), so the difference does not reach the page. It is in the range expected from one smoothing and hysteresis setting for all devices (R-8), and not a defect.
- The session span matches `total_elapsed_time` within 1 s. Maximum speed and latitude range are plausible. `TestCorpus` checks invariants 1 to 4 on the file, and they hold.

**Mutation spot checks** (scratch copy with all three real files).

| Mutation | `make corpus` | Failing tests |
| --- | --- | --- |
| H 3 → 2.5 | 3 DIFF (both real ride files and `rolling-hills`) | TestCorpus, TestElevationGainRollingHills |
| E-8 removed for both `activity` fields | 0 DIFF | `TestSystemTimeActivityUnknownZone` (3 subtests) |
| Repeated timer stop not ignored (`case e.Kind == TimerStop:`) | 1 DIFF (a real file) | TestCorpus (a real file), `TestMovingTimeFromTimer/repeated_start` |
| Timer "stop first" interval dropped | 0 DIFF | `TestMovingTimeFromTimer/stop_first` |

The Wahoo file adds real coverage: the double-stop mutation shows up as a corpus DIFF on a real device file.

## Findings

None.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-02-r3-1 (`.Fit` not ignored) | **Fixed** | `.gitignore` uses `*.[fF][iI][tT]` (ADR-004 rev. 3). All case variants are ignored everywhere, and the 17 synthetic files stay addable (table above) |
| r3 Question 1 (outputs derived from real files) | **Settled** | ADR-004 item 7 (rev. 3): everything derived from a real file stays local. This covers F-03 `data.bin` and sizes (`MEASUREMENTS.md` lists synthetic files only; real-file sizes appear in QA reports by manufacturer only), F-04 fixtures, F-08 broken files, the F-32 sweep, and any later fixture. Committed fixtures are built from synthetic files only, and real outputs go under `build/` or the ignored `real/` folders |

## Questions

None.

## User conditions

- **AC 2 / OQ-6: real files from at least three distinct FIT writers.** Two are present: `development` (Apple Watch through a third-party app, 2 files) and `wahoo_fitness` (1 file). All three files pass `make corpus`. One more writer is needed, and as noted in r3 it must not be another `development` writer. When it is added, QA reruns `make corpus` on this machine and lists the manufacturers.
- **OQ-4:** the moving-time default for other sports. Not blocking; it is pinned by `TestMovingTimeThresholds`, so that test must be updated together with any decision.
- **OQ-5:** the FIT licence, to settle before launch (F-29, F-32).
- **From F-01:** a real GitHub Actions run (with the Go cache on), Node 24, and the `@v7` tags. None can be checked here because there is no remote.

## Coverage

- AC 1 and AC 3 to AC 12 are traced to code and passing tests as in r3.
- AC 2 is partly met: 2 of 3 writers, each passing on this machine.
- The privacy constraint was checked three ways: the ignore rules, a clean copy, and the leak search.
- Not run:
  - `make corpus-update` (it rewrites `expected/`);
  - a GitHub Actions run;
  - AC 2 with a third writer (no file yet).
