# QA F-02 FIT parsing and normalisation - round 2

- **Date:** 2026-10-10
- **Verdict:** ready with conditions. All five r1 findings are fixed, and all three r1 questions are settled in design v1.3 and ADR-004 revision 1. I found no regressions: the 16 r1 synthetic files and their golden reports are byte-identical to my r1 snapshot. One new Minor finding is open: E-8 at the `activity` call site is not pinned by any test. As in r1, the only thing keeping this from "ready" is AC 1, the real files from three brands (OQ-3, user condition).
- **Inputs:** `features/F-02-fit-parsing-and-normalisation.md` (scope line now reads "diagnostic CLI subcommand, shipped in the server binary"); `architect/F-02-fit-parsing-and-normalisation.md` v1.3 (change log, D-10, E-8, the F-08 note in the boundary section, Corpus row `system-time-record.fit`, Tests); ADR-004 revision 1 (items 4 and 5); `qa/F-02-fit-parsing-and-normalisation-r1.md`. Code: the uncommitted working tree on `master` on top of `e415d92`. Files changed since r1: `internal/ingest/{convert,listener}.go` and tests, `internal/ingest/corpus_test.go`, `internal/workout/normalise_test.go`, `scripts/corpus.sh`, `tools/corpusgen/{main,main_test}.go`, `testdata/corpus/README.md`, new `synthetic/system-time-record.fit` and its golden report.

## Build and test results

Environment as in r1: Go `go1.26.8 linux/amd64`, Node `v22.22.2`, no remote.

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | exit 0. `go test ./...` passes for 5 packages (now including `tools/corpusgen`). Vitest 23/23 passed. Bundle `10613 / 60000 bytes gzip PASS` |
| `go test -race -count=1 ./...` | all `ok`; `tools/corpusgen` 2.6 s; whole run 3.6 s wall clock |
| `go vet ./...`, `gofmt -l .`, `go mod tidy -diff` | clean |
| `make corpus` | `corpus: 17 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` |
| `go run ./tools/corpusgen -out <scratch>` + `diff -r` against `synthetic/` | identical (17 files) |
| `diff -rq` of my r1 corpus snapshot against the current corpus | only `system-time-record.{fit,json}` are new and the README changed; **all 16 r1 `.fit` files and golden reports are byte-identical**, so nothing in v1.3 changed existing output |
| `BenchmarkParse12h -benchtime 20x` | 63.9 ms/op, 27.6 MB/op, 473 allocs/op. Same as `MEASUREMENTS.md`; E-8 adds no measurable cost |
| `git status --short` before and after | identical |

**Probes for the corpus naming rule.** A scratch copy of the corpus with `real/GARMIN01.FIT` and `real/notes.txt` added:
- `corpus.sh` prints `UNKNOWN` for both, ends with `17 ok, …, 2 unknown`, and exits 1.
- `TestCorpus` fails with "`…/GARMIN01.FIT is not a corpus file: rename to <brand>-<model>-<what>.fit`", and the same for `notes.txt`.

**Probes for `TestSyntheticCorpusCurrent`.** Changing one seed in `corpusgen` (`NewPCG(19, 20)` → `(19, 21)`) gives "`flat-noise.fit differs from the generator output`". An extra committed `synthetic/extra.fit` gives "`extra.fit is committed but not generated`".

**Mutation probes.** Each ran on a scratch copy, with `corpus.sh` and `go test ./internal/...`.

| Mutation | `make corpus` | Failing tests |
| --- | --- | --- |
| r1 survivor: cycling threshold 1.0 → 1.05 km/h | 0 DIFF | TestMovingTimeThresholds (caught now) |
| r1 survivor: default threshold 0.5 → 0.6 km/h | 0 DIFF | TestMovingTimeThresholds (caught now) |
| r1 survivor: `speed >= threshold` → `>` | 0 DIFF | TestMovingTimeThresholds (caught now) |
| r1 survivor: `tzStep` 900 → 1800 | 0 DIFF | TestTimeZone/Nepal_+05:45, Chatham_+12:45, the two rounding cases (caught now) |
| `validDateTime` without the `>= 0x10000000` check | 1 DIFF (`system-time-record`) | TestValidDateTime, TestSystemTimeRecordDropped, TestSystemTimeSessionStartFallsBack, TestCorpus |
| E-8 removed at the record site only | 1 DIFF | TestSystemTimeRecordDropped, TestCorpus |
| E-8 removed at the event site only | 1 DIFF | TestCorpus (`system-time-record`) |
| E-8 removed in `span` (session and lap) | 0 DIFF | TestSystemTimeSessionStartFallsBack |
| **E-8 removed for `activity.local_timestamp`** | 0 DIFF | **none** |
| **E-8 removed for `activity.timestamp`** | 0 DIFF | **none** |
| **E-8 removed for both `activity` fields** | 0 DIFF | **none** (QA-02-r2-1) |
| Regression check: H 3 → 2.5, enhanced/legacy swap, `maxFillGap` 5, same-second merge, `multisport` | as in r1 (1, 14, 1, 0, 1 DIFF) | as in r1, all caught |

**Probe for the engineer's caveat.** I added a scratch test, `TestProbeE8BothActivityTimesSystem`: an `activity` message with `timestamp` = system time 5,000 and `local_timestamp` = system time 12,200.
- On the real code it passes: time zone unknown, as E-8 requires.
- On the mutant with E-8 removed for both `activity` fields it fails: `TimeZone = {Known:true OffsetSeconds:7200}`.

So E-8 at this site **is** observable, and a one-message test detects its removal.

## Findings

No Blocker or Major findings.

### Minor

- **ID:** QA-02-r2-1
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/ingest/listener.go:59`; test `TestSystemTimeLocalTimestampUnknownZone` in `src/server/internal/ingest/listener_test.go:314-320`
- **Relates to:** design v1.3 E-8 ("used for every timestamp `ingest` reads … activity `timestamp` and `local_timestamp`"), R-9, decision 13
- **Evidence:** Three mutations survive all tests and leave the corpus unchanged: removing `dateTime(...)` around `a.Timestamp`, around `a.LocalTimestamp`, or around both. The existing test uses a *valid* `timestamp` with a system-time `local_timestamp`. Their difference (about −1.17e9 s) falls outside the ±14 h range of R-9, so the zone is unknown whether or not E-8 applies. That is the engineer's caveat. The caveat is not the whole story, though. When **both** fields are system time, their difference is small and plausible. Without E-8 the time zone then becomes "known" (probe: +7,200 s), while E-8 says it must be unknown. `TestValidDateTime` tests the predicate but not that the `activity` path calls it.
- **Consequence:** If a later refactor drops the wrapper on these two fields (for example when F-08 reworks the listener), a file from a device that writes its activity message before getting a clock fix would show a time zone that E-8 forbids, and no test would fail. Low likelihood, but it is exactly the path E-8 names, and the fix is one test case. **Answer to the coordinator's question:** `TestValidDateTime` alone is not enough, because it pins the predicate, not the call site. To settle it: add a case with both `activity` fields in system time and expect `TimeZone.Known == false`. I ran exactly that test against the mutant and it fails.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-02-r1-1 (upper-case `.FIT` skipped silently) | **Fixed** | ADR-004 rev. 1 item 4. `corpus.sh` loops over `*` and reports anything not ending in `.fit`, or not a regular file, as `UNKNOWN`, which fails the run. `TestCorpus` uses `os.ReadDir` and fails on the same entries. Hidden files (`.gitkeep`) are skipped by both, as the ADR says. Probe above. Minor inconsistency, not a finding: `[ -f ]` follows symlinks, `DirEntry.Type().IsRegular()` does not, so a symlinked `.fit` passes `make corpus` but fails `TestCorpus`. That direction is the safe one |
| QA-02-r1-2 (moving-time thresholds not pinned) | **Fixed** | `TestMovingTimeThresholds` catches all three r1 survivors (1.05 km/h, 0.6 km/h, `>`) |
| QA-02-r1-3 (15-minute rounding not pinned) | **Fixed** | `TestTimeZone` has Nepal +05:45, Chatham +12:45, 20,690 → +05:45 and 20,500 → +05:45. `tzStep 1800` now fails four cases |
| QA-02-r1-4 (rolling-hills README row) | **Fixed** | Row 44 now reads "Expected elevation gain with this noise: 63.16 m (the noise-free shape gives 60 m; design v1.2+, R-8 tests)". The hand-off reference is gone |
| QA-02-r1-5 (no check that synthetic files are current) | **Fixed** | ADR-004 rev. 1 item 5. `TestSyntheticCorpusCurrent` in `tools/corpusgen` runs in `go test ./...` (2.6 s), compares byte for byte, and reports missing and extra files (probes above). `corpusgen` is split into `generate(outDir)`, used by both `main` and the test |
| r1 Question 1 (`parse` in the binary) | **Settled** | D-10 gives the reason: diagnose a failing original on the server with exactly the parser that failed, no dependency, a few KB, nothing beyond what shell access already shows. The feature file now says "diagnostic CLI subcommand, shipped in the server binary"; F-03 and F-07 wording is aligned |
| r1 Question 2 (FIT system time) | **Settled and implemented** | E-8: one predicate `validDateTime` (`raw >= 0x10000000 && raw != 0xFFFFFFFF`), applied through `dateTime()` at every timestamp read: record, event, session and lap `span`, activity (`listener.go:59,105,112,134`). New corpus file `system-time-record.fit`, whose golden report shows start `2026-06-22T05:00:00Z`, 600 samples, last offset 599, and timer events `start@0` and `stop@599` only (the stop at system time 2,000 is gone). My r1 probe scenario (one record at FIT epoch + 1,000 s) is now dropped. Only the activity call site is unpinned (QA-02-r2-1) |
| r1 Question 3 (F-08 and `multisport`) | **Settled** | The boundary section of design v1.3 says F-08 reuses R-9 and must not rebuild it, and that chained files get `multisport` only once F-08 decodes the later sequences |

## Questions

None.

## User conditions (unchanged from r1)

- **AC 1 / OQ-3:** real files from three brands. The slots, the README procedure and both drivers are ready. The naming trap from r1 is closed: a file under the device's own name now fails both drivers instead of being skipped. Committed real files still contain the device serial number; OQ-3 must cover that.
- **OQ-4:** the moving-time default for other sports. Not blocking. A change to it is now caught by `TestMovingTimeThresholds`, so the test must be updated together with any decision.
- **OQ-5:** the FIT licence. Must be settled before launch (F-29, F-32).
- **From F-01:** a real GitHub Actions run (with the Go cache on), Node 24, and the `@v7` tags. None can be checked here because there is no remote.

## Coverage

- AC 2 to AC 11 are traced to code and passing tests as in r1, with stronger tests for AC 5 (thresholds), R-9 (rounding) and the corpus mechanism (names, generator currency).
- AC 1 is still only partly met: the synthetic part is done, the real files wait for OQ-3.
- E-8 is traced to the design, `convert.go`, `listener.go`, unit tests and a corpus file. The `activity` site is the exception (QA-02-r2-1).
- Regression check: existing corpus output is byte-identical to r1, the r1 mutation set is still caught, the benchmark is unchanged, and the boundaries are unchanged (no F-03 or F-08 code). The new `validDateTime` uses the library's `datetime.ToUint32` only inside `ingest`, so no library type crosses the boundary.
- Not run: `make corpus-update` (it rewrites `expected/`), a GitHub Actions run, and real device files.
