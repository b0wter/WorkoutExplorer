# QA F-08 FIT validation, truncated-file recovery and parse limits - round 4

- **Date:** 2026-10-15
- **Verdict:** **ready.** There is no Blocker or Major finding. Every acceptance criterion (1 to 12) is traced to code and a passing test. The second half of the new distance AC is also verified on the local real files.
  - All four r3 findings are fixed. The M9, M21 and M22 mutants are now killed.
  - R-14 / D-25 is built as designed, and every golden diff stays inside the allowed keys.
  - Two Minor findings remain:
    - QA-08-r4-1 (backend, test gap): the order of R-14 against R-3 is not pinned, and the record-leg assertion passes without R-14.
    - QA-08-r4-2 (architect, documentation): the design header is stale, and the R-14 section quotes real-file counts.
- **F-02 AC 2:** **met** (see section [F-02 AC 2](#f-02-ac-2)).
- **Inputs:**
  - Feature: `features/F-08-fit-validation-and-recovery.md`, including the new distance AC (number 10 in file order) and OQ-39, which is resolved. Also `features/README.md`.
  - Architecture: `architect/F-08-fit-validation-and-recovery.md` v1.4. Sections read: R-14, Tests (the R-12, R-14, `TestChained` and `TestCorpus` rows), Hand-offs v1.4, and "Changes for the backend engineer (v1.4)", rows 1 to 6 and the allowed golden keys. Also `architect/F-02-fit-parsing-and-normalisation.md` v1.11, `architect/INDEX.md` and ADR-004 item 7.
  - Spec: the TD FIT parsing sentence on distance; AP-5, AP-7 and AP-8. The F-17, F-18 and F-25 feature diffs.
  - Earlier round: `qa/F-08-fit-validation-and-recovery-r3.md`.
  - State: HEAD `27ea5af` plus the uncommitted working tree (18 modified files, 1 untracked), reviewed as found.
  - QA changed nothing outside `qa/`. Mutants ran in a scratchpad copy of `src/server` that holds no real files. The baseline was a `git archive 27ea5af` build. One invariant probe used a scratch copy with the real files in it; that copy was deleted after the probe.

## Build and test results

| Check | Result |
| --- | --- |
| `make -k ci` | **Exit 0.** `go vet` and gofmt are clean. svelte-check: 256 files, 0 errors, 0 warnings. Go: 9/9 packages ok. Vitest: 29 files, 786 tests passed. Size: `entry index.html: 45582 / 60000 bytes gzip PASS`. |
| `go test -count=1 -race ./...` | 9/9 ok, no race. ingest took 97.7 s and workout 1.9 s. |
| `make corpus` | `real: 6 files (local only)`, `corpus: 48 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` (42 synthetic files and 6 real ones). |
| `make corpus-gen` | QA generated the corpus into the scratchpad first: `diff -r` against `synthetic/` shows no difference across 42 files. Then QA ran `make corpus-gen` itself: the SHA-256 of all 42 files is unchanged, and `git status --porcelain` is unchanged. **The bytes are identical.** |
| `FuzzParse` | Not run this round. The change is one O(n) pass over an already allocated slice, with no new decoder path. |

## Findings

### Minor

- **ID:** QA-08-r4-1
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:**
  - `src/server/internal/workout/normalise.go:37-39`, the position of `holdDistance` after `fillGaps`.
  - `src/server/internal/workout/f08_v14_test.go:69-75`, the truncated record-leg case.
- **Relates to:** R-14 ("right after R-3"), the R-14 tests row, D-25.
- **Evidence:**
  - **Mutant H3** (run `holdDistance` before `fillGaps`) **survives**, but it does not behave the same as the built order. QA probed records with distance 100, 50 and 200 at 0 s, 1 s and 5 s:
    - As built: `[100 100 100 125 162.5 200]`.
    - With H3: `[100 100 125 150 175 200]`.
    - Both outputs are non-decreasing. The only filled-sample test lies between a high and a low neighbour, and it cannot tell the two orders apart.
  - **The truncated record-leg case passes without R-14.** It asserts `summary distance >= 300`.
    - With R-14 the summary distance is 400.
    - With `holdDistance` removed it is 300, because the leg is clamped to 0, and the assertion still passes.
    - The design's own case ("leg `Distance` ≥ 0") is guaranteed by that clamp, so it cannot fail either.
    - The test file still fails without R-14, but only because of the `Derived.Distance` half.
- **Consequence:**
  - Moving R-14 before R-3 would change filled distance values in every smart-recording file with a fall, and no test would notice.
  - A regression in the record leg after a fall would also go unnoticed.
  - The fix:
    - Add the 100/50/200 case with its expected values.
    - Assert summary distance `== 400` (session 300 plus leg 100).

- **ID:** QA-08-r4-2
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:** `architect/F-08-fit-validation-and-recovery.md:4` and `:450`.
- **Relates to:** the design's own rule at lines 446 and 509 ("Real-file numbers go only into the QA report, never into this document"), and ADR-004 item 7.
- **Evidence:**
  - Line 4 still says that R-14's "code is not built yet".
  - The R-14 "Problem" paragraph quotes QA r3's real-file buckets and counts ("100 m or more", "9 runs (494 samples)", "10 to 100 m").
  - These counts are not personal data: there are no positions, times or totals. But they contradict the document's own rule.
- **Consequence:** the status line misleads readers, and the rule on real-file figures is applied unevenly. Cosmetic.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-08-r3-1 (M9, M21, M22 and the `TestChained` midpoint case) | **Fixed** | M9 is killed (`TestTransitionMovingTime`, timer-branch gap case). M21 is killed: `TestParseLogs` uses `ride-30min.fit` with `MaxSamples: 1000` and asserts `kind` `limit`, `limit` `samples`, `WARN` and a non-empty manufacturer. M22 is killed (`TestTransitionWhichSessions` compares the full totals). The `TestChained` midpoint case now has two sessions and checks `Derived.Distance` at 5,990 ± 0.05, `Summary.AvgSpeed` at 5.0 ± 0.01 and that no step is negative. The reported "0.01 m + 1e-6" tolerance deviation **is not in the tree**: the step check is strictly `< 0`, which is what v1.4 row 3 asks for. M19 and M20 (midpoint) are still killed. |
| QA-08-r3-2 (real triathlon distance falls) | **Fixed by design and code** (R-14, D-25) | See section R-14 below. The design's real-file checks (a) to (c) pass. |
| QA-08-r3-3 (reflection in `transition.go`) | **Fixed** | Streams now use `[numStreams]transitionRule` over `s.streams()`, and the zero value means undecided. The completeness test matches `Samples` fields to `streams()` by address, names `Offset` and `Filled` explicitly, and fails on any other field of any type. Reflection remains only for `Totals`, as QA suggested. Mutant T1 (a stream left undecided) is killed by `TestTransitionAllowListComplete`. T2 (`Temperature` dropped) is killed by `TestCorpus`, `TestTransitionSamples` and `TestTruncationPrefix`. |
| QA-08-r3-4 (`MEASUREMENTS.md`, `sequences.go` comment) | **Fixed** | The base is now `0ac678e`. The "Result" paragraph quotes the v1.3 figures (0.12 s and 0.57 s). The worst case for R-13 is recorded as a QA probe (0.20 s, 143.4 MB). The comment is re-wrapped. |
| r3 Q1 (OQ-39 still open in the design) | **Resolved** | The design lists it under Resolved, Open questions says "None.", and the change also appears in `architect/INDEX.md`, the `features/README.md` decisions and the F-08 feature scope. |
| r3 Q2 (tolerate or normalise distance falls) | **Resolved** (D-25) | The TD sentence, the F-17, F-18 and F-25 hand-offs and the feature texts are updated consistently. |

## R-14 as built

- `holdDistance` (`distance.go:53-64`) matches the design's pseudocode:
  - `NaN` is skipped and leaves `high` unchanged.
  - It works in place on the workout's own copy.
  - It does nothing when there is no stream.
- It is called right after `fillGaps` and before the sport timeline, `maskTransitions`, `settlePresence` and `computeDistance`. The `Normalise` comment says "R-1 to R-14".
- Moving the call after `settlePresence` (mutant H8) gives the same result, because R-13 keeps distance.
- `TestContinueDistance` value cases call `continueDistance` directly with their pre-v1.4 expected values (row 6). The purity case still runs through `Normalise`.
- `checkInvariants` (`corpus_test.go:146-157`) adds the distance invariant over valid values.

**Engineer's open choices:**

| Choice | Judgement |
| --- | --- |
| NaN-aware partial comparison in `TestHoldDistanceInputUnchanged`, which compares `At`, `Distance`, `Lat` and `Speed` | **Accept.** `DeepEqual` always fails on the NaN fields, so the test always falls through to `sameRecs`. That is enough: R-14 never touches `Recording`, and `TestContinueDistance` also covers purity. |
| Record-leg case asserts summary distance ≥ session total | **Weak** → QA-08-r4-1. |
| R-12 boundary case through `SequenceStarts` (`TestHoldDistanceChainedBoundary`) | **Accept.** Sample 4 is held at 15, with no negative step. It also kills M19 and H5. |

## Mutation probes (`go test ./internal/workout ./internal/ingest`, scratchpad copy)

| # | Mutant | Result |
| --- | --- | --- |
| H1 | `holdDistance` not called | Killed (TestChained, TestHoldDistance, …ChainedBoundary, …Derived, …FilledSample) |
| H2 | `NaN` resets `high` | Killed (TestHoldDistance) |
| H3 | hold before `fillGaps` | **Survived, not equivalent** → r4-1 |
| H4 | hold after `Derived.Distance` is taken | Killed (TestHoldDistanceDerived) |
| H5 | 0.05 m tolerance (`d < high-0.05`) | Killed (TestChained, …ChainedBoundary) |
| H6 | a fall becomes `NaN` instead of `high` | Killed |
| H7 | the first valid value does not start `high` | Killed (…FilledSample) |
| H8 | hold after `settlePresence` | Survived, **equivalent** (R-13 keeps distance) |
| M9, M21, M22 | r3 survivors | **All killed** (see Previous findings) |
| M19, M20 | midpoint `< prev`, `<= prev/2` | Killed |
| T1, T2 | stream table | Killed |
| I1 | distance invariant never updates `prev` (vacuous) | Survives in CI, because no synthetic file has a falling distance. On the local real files, with R-14 removed and goldens regenerated without it, the invariant **fails on 2 files (manufacturer `development`)**. With I1 on top, it passes. So the invariant works on real data and is vacuous in CI (Q1). |

## Golden diffs

- **Synthetic:** no report, data file or `.fit` changed (`git status` and `make corpus-gen`).
- **Local real:** QA built HEAD `27ea5af` and the working tree, then compared the six reports and data files by key path only. No values were printed.
  - **2 of 6 reports changed** (both from manufacturer `development`), and only in `/streams/distance/mean` and `/streams/distance/digest`. That is inside the allowed list.
  - In those 2 data files, the metadata is identical, and **only the `distance` column** differs, with the same length and the same valid mask.
  - The other 4 reports and data files are byte-identical.
  - Every current report equals its local golden.
- **R-14 real-file check:**
  - (a) The invariant passes on all 6 files: `go test -race` ran with them present, and 0 falls were found in every decoded data file.
  - (b) `derived.distance ÷ summary.distance` rounds to 1.00 for all 6 files. All 6 are complete and have sessions.
  - (c) No key outside the allowed list changed.

## F-02 AC 2

- **Criterion:** `make corpus` passes over real files from at least three distinct `device.manufacturer` values.
- **Result:**

  | `device.manufacturer` | Files | Parse | `make corpus` |
  | --- | --- | --- | --- |
  | `development` | 4 | pass | ok |
  | `wahoo_fitness` | 1 | pass | ok |
  | `garmin` | 1 | pass | ok |

- **Verdict: F-02 AC 2 is met.** It was the only condition keeping F-02 r4 from "ready". `make ci` and `make corpus` are green, so F-02 is **ready**.
- **Still open, but not F-02 conditions:**
  - OQ-4 (moving-time threshold for other sports): the default applies, and `TestMovingTimeThresholds` pins it.
  - OQ-5 (FIT licence): must be settled before launch, in F-29 and F-32.
  - F-01's GitHub Actions run (Node 24, `@v7`) is tracked under F-01.
- **Hygiene:**
  - The new file `real/garmin-edge-explore-ride.fit` and its golden are ignored (`.gitignore:10, 12`).
  - The r3 name `garmin_edge_explorer.fit` is gone, so there is no stale golden.

## Commit hygiene

- `git ls-files` lists only the two `.gitkeep` files under `real/` and `expected/real/`.
- `git add -A --dry-run` would stage 19 paths. None of them is a real file, a golden derived from one, a `.bin`, a `.br` or a `build/` path.
- The 6 real `.fit` files, the 6 real goldens, `build/` and `node_modules/` are ignored.
- No `testdata/fuzz/` exists.
- `MEASUREMENTS.md` contains no real-file figures.
- `git status --porcelain` was the same before and after this round.

## Questions

- **Q1 (architect):** in CI, no synthetic file has a falling device distance. The `TestCorpus` distance invariant can therefore never fail there, and no golden pins R-14 end to end. Only `TestChained`'s 1 cm step reaches it through `Parse`. Should a small synthetic file get a fall and a near-zero dropout, so that CI pins the held stream in the report and the data file? What would settle it: a yes with a corpus row, or a sentence in the design accepting `TestChained` plus the unit cases as sufficient.

## Coverage

| AC | Status |
| --- | --- |
| 1 to 9, 11, 12 (limits, log) | Traced as in r3. They pass. M9, M21 and M22 are now killed, which strengthens ACs 8 and 12. |
| **10** Distance never decreases; a falling device keeps the last high; the summary equals the session total | Code: `holdDistance`, called from `Normalise`. Tests: `TestHoldDistance*` (6 tests), `TestChained` (no negative step and `AvgSpeed` through `Parse`), and the `TestCorpus` invariant (vacuous on synthetic files, Q1). The second half was verified manually on the 2 local real files with falls: the summary is unchanged against HEAD, and the ratio is 1.00. |

Not run: `FuzzParse`, the benchmarks (no `MEASUREMENTS.md` row is needed, per the design), and the operator's own real-file report. QA ran checks (a) to (c) itself and records only manufacturers and booleans.
