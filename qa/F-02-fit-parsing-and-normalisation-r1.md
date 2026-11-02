# QA F-02 FIT parsing and normalisation - round 1

- **Date:** 2026-10-10
- **Verdict:** ready with conditions. There are no Blocker or Major findings. Every acceptance criterion except AC 1 is traced to code and to a passing test, and mutation probes show that the corpus and the tests catch changes to the important parameters. One thing keeps this from "ready": **AC 1 has no real files from three brands yet (OQ-3, user condition)**. Five Minor findings are open. They are about test sharpness and corpus tooling, not wrong behaviour.
- **Inputs:** `features/F-02-fit-parsing-and-normalisation.md`; `features/README.md` decisions 5, 12, 13, 33; TD sections FIT parsing, Tech stack, Hosting and operations (Local development); spec AP-1, AP-2, AP-4, DS-2; `architect/F-02-fit-parsing-and-normalisation.md` v1.2 (change log, D-1 to D-11, E-1 to E-7, R-1 to R-10, Tests, AC table); ADR-003 (with Revision 1); ADR-004; `architect/overview.md` v1.3; `architect/INDEX.md`. Code: the uncommitted working tree on `master` on top of `e415d92` (`git diff e415d92` plus the untracked `src/server/internal/{ingest,workout}`, `cmd/workout-explorer/parse*.go`, `tools/corpusgen`, `scripts/corpus.sh`, `testdata/corpus`, `go.sum`).

## Build and test results

Environment: Go `go1.26.8 linux/amd64`, Node `v22.22.2` (not the pinned 24, as in F-01), no git remote.

| Command | Result |
| --- | --- |
| `make ci` (repo root, real exit code captured) | exit 0. svelte-check passes with `--fail-on-warnings`. `go test ./...`: `ok` for `cmd/workout-explorer`, `internal/httpapi`, `internal/ingest`, `internal/workout`. Vitest 23/23 passed. Bundle `10613 / 60000 bytes gzip PASS` |
| `go test -race -count=1 ./...` (src/server) | all `ok` (ingest 2.8 s); `tools/corpusgen` has no test files |
| `go vet ./...`, `gofmt -l .` | clean |
| `go mod tidy -diff`, `go mod verify` | no diff; `all modules verified` |
| `make corpus` | `corpus: 16 ok, 0 diff, 0 fail, 0 new, 0 stale`, exit 0 |
| `go run ./tools/corpusgen -out <scratch>` twice, then `diff -r` against each other and against `testdata/corpus/synthetic` | byte-identical (16 files), so the generator is deterministic and the committed files are current |
| `go list -deps` over `./cmd/... ./internal/...` | non-standard packages are only `workout-explorer/...` and `github.com/muktihari/fit/...` |
| `grep -rl muktihari --include=*.go` | `internal/ingest/{ingest,listener,convert}.go` and their tests, plus `tools/corpusgen/main.go`. `internal/workout` imports only the standard library (`crypto/sha256 encoding/hex encoding/json math sort strconv strings time`) |
| `go test ./internal/ingest -bench BenchmarkParse12h -benchtime 20x -count 2` | 70.9 ms and 71.8 ms/op, 27.6 MB/op, 473 allocs/op. Matches the second row of `MEASUREMENTS.md` |
| `/usr/bin/time -v build/workout-explorer parse .../ride-12h.fit` | max RSS 23,032 KB, wall clock 0.08 s. Matches `MEASUREMENTS.md` |
| `git status --short` before and after | identical; all probes ran in the scratchpad |

**Probe: decoding the raw `unknown-messages.fit` with a separate listener that keeps everything.** The file really contains 6 messages with number 65281 (0xFF01), field 250 in all 301 records, and a developer field in 31 records, and the decoder returns no error. So the AC 9 corpus test passes for the right reason.

**Mutation probes.** Each probe ran on a scratch copy of `src/server`: one constant or line changed, then `corpus.sh` and `go test ./internal/...` were run on that copy.

| Mutation | `make corpus` | Failing tests |
| --- | --- | --- |
| `hysteresisMetres` 3.0 → 2.5 | 1 DIFF (`rolling-hills`, `elevation_gain 63.160 → 89.840`, shown as `diff -u`, script exit 1) | TestCorpus, TestElevationGainRollingHills |
| `smoothingSeconds` 2 → 3 | 10 DIFF | TestCorpus (10), TestSummaryOneSession |
| zig-zag descent test `cand-x >= h` → `>= 2*h` | 0 DIFF | TestElevationGainRollingHills |
| `maxFillGap` 4 → 5 | 1 DIFF | AC8 acceptance, TestGapFilling, TestMovingTimeFromSpeed |
| enhanced/legacy preference swapped | 13 DIFF | AC4 acceptance, TestAltitudePrefersEnhanced, TestParseBothFieldVariants, … |
| semicircle divisor 2³¹ → 2³¹−1 | 13 DIFF | TestSemicirclesToDegrees, TestPosition, TestCorpus |
| running cadence not doubled | 2 DIFF | AC10 acceptance, TestCadenceBySport |
| `earthRadius` → 6,378,137 | 1 DIFF | AC7 acceptance, TestHaversine |
| same-second merge: earlier value wins | 0 DIFF | TestTimeAxisMergesSameSecond |
| timer "stop first" interval dropped | 0 DIFF | TestMovingTimeFromTimer/stop_first |
| `multisport` → first session's sport | 1 DIFF | AC10 acceptance, TestSport, … |
| summary ascent fallback removed | 4 DIFF | AC6 acceptance, TestSummaryOneSession |
| HR sentinel → 0 instead of NaN | 0 DIFF | TestScalarConversions, TestTotals |
| `fractional_cadence` ignored | 15 DIFF | several |
| `tzMaxOffset` 14 h → 24 h | 0 DIFF | TestTimeZone/20_h_unknown |
| `tzStep` 900 → 3600 | 1 DIFF | TestTimeZone/India, TestCorpus |
| **`cyclingMovingSpeed` 1.0 → 1.05 km/h** | 0 DIFF | **none** (QA-02-r1-2) |
| **`defaultMovingSpeed` 0.5 → 0.6 km/h** | 0 DIFF | **none** (QA-02-r1-2) |
| **`speed >= threshold` → `>`** | 0 DIFF | **none** (QA-02-r1-2) |
| **`tzStep` 900 → 1800** | 0 DIFF | **none** (QA-02-r1-3) |

**Probe: an upper-case `.FIT` file.** A copy of the corpus with `real/GARMIN01.FIT` added gave `corpus: 16 ok, …`, exit 0. The file was not parsed and not reported (QA-02-r1-1).

**Probe: a FIT "system time" timestamp.** A `workout.Normalise` probe with 600 records on 2026-10-01 and one record at FIT epoch + 1,000 s gave `StartTime 1989-12-31 00:16:40`, `last offset 1159775599`, and a summary elapsed time of 1.16e9 s. See Question 2.

## Findings

No Blocker or Major findings.

### Minor

- **ID:** QA-02-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/scripts/corpus.sh:32`, `src/server/internal/ingest/corpus_test.go:32`
- **Relates to:** AC 1, AC 2, ADR-004 item 4
- **Evidence:** Both drivers glob only `*.fit`. In the probe above, `real/GARMIN01.FIT` was silently skipped: no `NEW`, no `FAIL`, and the count line looks clean. Many devices name files in upper case (`*.FIT`). The only guard is step 1 of the README procedure ("copy it to `real/<brand>-<model>-<what>.fit`").
- **Consequence:** When the operator adds the real files for AC 1, a file under the device's own name would be committed but never parsed. Both `make corpus` and `TestCorpus` would stay green, so AC 1 would look covered while the brand is not exercised. To settle it: any file in `real/` or `synthetic/` that is not `*.fit` is reported (for example as `FAIL` or `UNKNOWN`) and fails the run, in both drivers.

- **ID:** QA-02-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/workout/normalise_test.go:384-388` (TestMovingTimeFromSpeed), `src/server/internal/workout/moving.go:11-12,40`
- **Relates to:** AC 5, TD "1 km/h cycling, 0.5 km/h running", design R-7 tests ("running threshold 0.5 km/h")
- **Evidence:** The tests only bracket the thresholds. Cycling uses 0.2 and 0.3 m/s, which accepts any threshold in (0.72, 1.08] km/h. Running and other sports use 0.1 and 0.2 m/s, which accepts (0.36, 0.72] km/h. No corpus file sits near either threshold. Three mutations pass every test and leave the corpus unchanged: cycling 1.05 km/h, default 0.6 km/h, and `>=` → `>`.
- **Consequence:** The TD rule, and the OQ-4 default once the user confirms or changes it, can drift without any signal. AC 2 holds only because no corpus output changes, which is the gap itself. To settle it: speeds just below and just above each threshold (for example 0.99 and 1.01 km/h for cycling, 0.49 and 0.51 km/h for running and the default), so the constant is pinned to within a few percent.

- **ID:** QA-02-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/workout/normalise_test.go:561-569` (TestTimeZone), `src/server/internal/workout/timezone.go:8`
- **Relates to:** design R-9 ("rounded to the nearest 900 s"), decision 13, AP-1
- **Evidence:** Every test offset is a multiple of 1,800 s (7,200, −14,400, 19,800 for India, ±whole hours), and so is the corpus (`climb-100m`, +05:30). The mutation `tzStep = 1800` passes every test and leaves the corpus unchanged.
- **Consequence:** A regression to 30-minute rounding would shift start times for UTC+05:45 (Nepal), +08:45 and +12:45 (Chatham) by 15 minutes without any test failing. To settle it: one case with a :45 offset (for example 20,700 → 20,700, and 20,690 → 20,700).

- **ID:** QA-02-r1-4
- **Severity / kind:** Minor / documentation
- **Route:** backend
- **Where:** `src/server/testdata/corpus/README.md:44` (row `synthetic/rolling-hills.fit`)
- **Relates to:** design v1.2 change log and Corpus table, ADR-004 item 1 (README as manifest)
- **Evidence:** The row says "Design expects 60 m; the report shows 63.16 m (noise bias, see the F-02 hand-off)". Design v1.2 now expects "about 63 to 64 m" in the corpus, with 60 m only for the noise-free shape. "The F-02 hand-off" is a chat message, not a file in the repository.
- **Consequence:** A reader of the manifest sees 63.16 m reported as a deviation from the design when it is the expected value. The reference cannot be followed. To settle it: the row cites design v1.2 R-8 tests, and 63.16 m is described as the expected value.

- **ID:** QA-02-r1-5
- **Severity / kind:** Minor / risk (decision)
- **Route:** architect
- **Where:** ADR-004 item 5 ("CI does not regenerate; it tests the committed files"), `Makefile` target `corpus-gen`
- **Relates to:** ADR-004, AC 1 (corpus contents as documented)
- **Evidence:** Nothing checks that the committed synthetic files are still what `tools/corpusgen` produces. Today they are byte-identical (verified above). But someone could change a parameter in `corpusgen` (and the README row describing it) without running `make corpus-gen`. CI would stay green, and the golden reports would then describe files the generator no longer makes. The check itself is cheap: two generator runs plus `diff -r` took a few seconds here.
- **Consequence:** The corpus README, the generator and the tested bytes can drift apart silently. To settle it, either: the ADR records why the check is not worth its CI time, or a test or CI step regenerates into a temporary directory and diffs. Evidence that would change my mind: a measured CI cost that matters, or a statement that `corpusgen` is frozen after F-02.

## Previous findings

None. This is round 1.

## Questions

1. **For the architect: `parse` in the production binary.** The feature file says "A development-only CLI subcommand". Design D-10 says it is "shipped in the binary", and the section "`parse` subcommand" explains only what it must not do (it never logs, the service never runs it). It does not say why it is not dev-only, for example behind a build tag or under `tools/`. Today the subcommand prints position statistics (min, max, first and last latitude and longitude) to stdout for anyone with shell access to the server. That is acceptable for an operator tool, but the decision should be written down. To settle it: one sentence in D-10 giving the reason (for example "F-30 and on-server diagnosis need it on the host"), or a change to make it dev-only.
2. **For the architect, possibly the user: FIT `date_time` below 0x10000000.** The FIT profile defines values below 0x10000000 as relative "system time" (seconds since the device powered on), not UTC. The library (`kit/datetime.ToTime`) and `ingest` treat them as absolute dates from 1989 to 1998. In the probe above, one such record moved `StartTime` back 36 years and made the summary elapsed time 1.16e9 s. The spec and the design are silent, and the F-08 scope does not mention it. I have not seen such a record in a real file, so this is a question, not a defect. To settle it: say which feature owns it (F-02 `ingest` or F-08), and the rule (probably "drop records and events with a `date_time` below 0x10000000"), or confirm that it is not handled in the MVP.
3. **For the architect: sport `multisport` in F-02 (decision 12).** This is coherent and documented as the single exception in the design's boundary table. The value belongs to the normalised workout, F-03 needs it, and it is tested (AC10 acceptance, TestSport, `brick-run-bike.json`). Two points remain open for F-08. The F-08 feature file still lists "sport shown as Multisport" as F-08 scope, so its design should cite F-02 R-9 instead of rebuilding it. Chained files (decision 12 covers them too) only get `multisport` when F-08 decodes the following sequences. No change is needed in F-02.

## User conditions (not findings against the code)

- **AC 1 / OQ-3: real files from three brands.** `real/` and `expected/real/` exist (with `.gitkeep`). The corpus README has three placeholder rows marked "missing (OQ-3)", and its procedure "Adding a real file" (copy, `make corpus-update`, review, add a row with privacy treatment and consent) is in place. Both `corpus.sh` and `TestCorpus` already loop over `real/`, and a new file appears as `NEW` or fails with "no golden report". One caveat: file names must end in lower-case `.fit` (QA-02-r1-1). Real files also carry the device serial number in `file_id`. The parser never reads it, but the committed file contains it. The README says so, and OQ-3 must cover it.
- **OQ-4: moving-time threshold for sports other than cycling and running.** The default (0.5 km/h; `cycling` and `e_biking` 1 km/h) is in use as documented. It is not blocking. Note that QA-02-r1-2 means a change to this default is not caught by any test today.
- **OQ-5: FIT Protocol Licence.** Open with the user. Not blocking for F-02; it must be settled before launch (F-29, F-32).
- **CI conditions from F-01:** a real GitHub Actions run (now including `cache: true` with `cache-dependency-path: src/server/go.sum`), Node 24, and the `@v7` tags. These cannot be checked here because there is no remote.

## Coverage

**Acceptance criteria to code and test**

| AC | Code | Test(s), all passing | Status |
| --- | --- | --- | --- |
| 1 Corpus contents | `testdata/corpus/` (16 synthetic files: indoor without GPS, a 5 min GPS dropout, a 3 h pause, a run-and-ride brick, smart recording, the 12 h ride with 43,200 records, …), README manifest | TestCorpus, TestCorpusAcceptance (12-hour ride: 43,200 samples, 24 laps; pause; dropout: 300 samples without position) | **Synthetic part done; real files from three brands missing (user, OQ-3)** |
| 2 `make corpus` | `scripts/corpus.sh`, `Makefile` `corpus` / `corpus-update` / `corpus-gen` | Run clean (16 ok); mutation probes show `DIFF` with `diff -u` and exit 1 | Traced |
| 3 Semicircles | `convert.go` `semicirclesToDegrees` | TestSemicirclesToDegrees (2³⁰ → 90.0 exactly), TestPosition | Traced |
| 4 Enhanced preferred | `convert.go` `altitude`, `speed`, `totals`; `ingest.go` with `WithNoComponentExpansion` | TestAltitudePrefersEnhanced, TestSpeedPrefersEnhanced, TestParseBothFieldVariants (pins the option set), AC4 acceptance on all three segments of `enhanced-and-legacy.fit` plus the session average | Traced at file level |
| 5 Moving time | `moving.go` | TestMovingTimeFromTimer, TestMovingTimeFromSpeed, AC5 acceptance (`timer-pause` 1,200 s from the timer while records continue at 2 m/s; `cycling-no-timer-slow` 599 s) | Traced; threshold not pinned (QA-02-r1-2) |
| 6 Elevation gain | `elevation.go` | TestElevationGain, TestElevationGainRollingHills (noise-free, and seeds 1 to 10, no `t.Skip` left), AC6 acceptance (`flat-noise` 0 m, `climb-100m` 100 ± 2 m) | Traced |
| 7 Distance | `distance.go`, `gaps.go` (R-4) | TestHaversine, TestComputedDistance, TestNoDistanceNoPositions, AC7 acceptance (two files; no `distance` key in `indoor-no-distance-no-gps.json`) | Traced |
| 8 Smart recording | `gaps.go` | TestGapFilling, TestGapFillingInterpolates, AC8 acceptance (filled 5; gaps of 5 s and 30 s kept) | Traced |
| 9 Unknown messages | `listener.go` (switch on message number, everything else dropped) | TestListenerIgnoresUnknownMessagesFieldsAndDeveloperFields, AC9 acceptance; contents of `unknown-messages.fit` confirmed by the raw decode probe | Traced for synthetic files; real-device quirks wait for AC 1 |
| 10 Multi-session | `listener.go` (arrival order), `timezone.go` `workoutSport` | AC10 acceptance (2 sessions in order, `multisport`, running cadence 171 spm) | Traced |
| 11 Parse time and memory | `BenchmarkParse12h`, `MEASUREMENTS.md` | Reproduced: 71 ms/op, 27.6 MB/op, 473 allocs, max RSS 23 MB | Traced |

**Architecture checks done and found correct.**
- Dependency directions: `cmd → ingest → workout`; `workout` uses the standard library only and does no I/O or logging; only `ingest` imports the library in production code, and `tools/corpusgen` is the documented exception.
- Decoder options are exactly the set in the design; `serial_number` is not read (with a test); E-1 to E-7 are implemented and E-7's always-written keys appear in the empty-workout report test.
- R-1 to R-10 match the design text, including the zig-zag pseudocode, the E-5 tie rule, the summary fallbacks, the 15-minute rounding and the −12/+14 h range.
- The report format follows the design: key order, decimals, the `-0` rule, the NaN digest and integer offsets.
- `parse` exit codes 0, 1 and 2 are tested.
- `go.sum` exists and the CI cache switch matches F-01 Step 2. README targets are updated. The overview v1.3 edits are consistent with the code.

**Boundary checks.** Nothing from F-03 (no `data.bin`, no quantising) or F-08 (no rejection by `file_id.type`, CRC checked by the decoder defaults, no `WithIgnoreChecksum`, no limits, no logging in `ingest` or `workout`) has leaked in. Everything F-02 owns is present: start time, time zone, device totals as recorded, the `parse` subcommand, the corpus and `make corpus`. The only deliberate exception is `multisport`; see Question 3.

**Privacy and security.** No logging in the new packages. `parse` writes to stdout only. No serial number is read. The synthetic files use manufacturer `development` and have no serial number. `Parse` takes a `context` (a cancellation test exists). Unbounded input is accepted by design until F-08.

**Not run or not traceable.**
- No real FIT files exist (AC 1), so no cross-check against an independent encoder. ADR-003 names this symmetric-bug risk itself.
- No GitHub Actions run (no remote).
- `make corpus-update` was not run, because it rewrites `expected/`. Its code path was read instead.
- Antimeridian interpolation (R-3 interpolates longitude linearly) was not probed, because it is too rare to report.
