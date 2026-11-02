# QA F-17 Range selection and x-axis toggle - round 2

- **Date:** 2026-10-29
- **Verdict:** ready. Both Majors from r1 are fixed and tested. All three Minors are closed. No new findings.
- **Inputs:** r1 report and findings QA-17-r1-1 to r1-5. Decisions in a90e7a8 (F-17 design v0.3, ADR-029 rev. 1, ADR-011 rev. 9, F-13 D-10 exception). Fixes in 4ea3bd9 (backend, r1-1), 02ad199 (frontend, r1-2 and r1-4) and 1f72a3e (TD, r1-5). I reviewed the diff 638a20f..4ea3bd9. Working tree clean at 4ea3bd9.

## Build and test results

- `make ci`: exit 0.
  - svelte-check: 0 errors, 0 warnings.
  - `go test ./...`: all ok.
  - Vitest: 46 files, 1,182 tests passed (r1: 1,174).
  - Entry `index.html`: 54,919 / 60,000 B gzip, PASS. That is 17 B below r1 and inside the 100 B allowance for the r2 fixes.
  - Upload entry: 2,275 / 8,000 B, PASS. No storage API in it.
- `go test -race -count=1 ./internal/workout ./internal/datafile`: ok.
- `make corpus`: 53 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown. The line "ok invariants (M, G) on every accepted file" is printed. Real: 6 files, local only.
- Mutant, in a scratch copy of the module with `testdata` symlinked and nothing in `src/` changed: I removed the `cum[0] = 0` anchor from `moving.go`. The mutant is killed in three places:
  - `TestRoundTripCorpus` fails on 2 real files (the same 2 as r1) and on `synthetic/timer-before-first-record`.
  - `internal/workout` fails too (`TestCumulativeMovingTimeTimerBeforeFirstSample`).
  - The scratch copy has been deleted.
- Not run: any browser. The residual first-paint shift on an uncached bundle is accepted in D-10 and cannot be checked here.

## Findings

None.

## Previous findings

- **QA-17-r1-1 (Major): fixed.**
  - ADR-029 rev. 1 sets `M[0] = 0`. Moving time before the first sample counts in the first interval, and time after the last sample counts in the last one.
  - `moving.go:51-60` implements this. Monotonicity still holds because `0 ≤ cum[1]`, and `cum[1]` already includes the time before the first sample.
  - `checkCumulativeStreams` (`corpus_test.go:321`) runs on every accepted corpus file, synthetic and local real. It checks the normalised series and the decoded stream: first value 0, never decreasing, last value equal to the total. It also checks `M[N−1] − M[0]` exactly against the summary's `moving_time`, which is AC 9 at data-file level.
  - `make corpus` runs it and fails on it. I checked that failures print file names only (`--- FAIL` lines), never values.
  - New synthetic file: `timer-before-first-record`, first record at 3 s, timer running from 0 s.
  - About the brief's note that `range.test.ts` does not use the new golden: it does. `loadGolden()` (`test-support/golden.mjs`) reads every `.bin` in `testdata/datafile/synthetic` and throws on unpaired files. The "AC 7: a whole-workout range has the summary moving time" test therefore covers `timer-before-first-record.bin` (within 0.5 s, so a 1 s gap fails), and so do the brute-force reference tests. Nothing needs to change.
- **QA-17-r1-2 (Major): fixed.**
  - `restoreSplit` (`splitter.ts:34`) runs at the top of `startPage` (`start.ts:72-75`), before any `await`. The splitter element still arrives after the load.
  - The test `units.dom.test.ts` "the stored split is set before the data load resolves" holds the fetch open. It asserts `--map-w` is 35.00 % and that no splitter exists yet, then resolves the fetch and asserts the split survives. Because `startPage` runs synchronously up to its first `await`, the test would fail if the call moved after the load.
  - The residual shift on an uncached bundle is recorded as accepted in F-13 D-10, with a condition that would change the decision (design v0.3 splitter note).
  - The layout-change listener is now registered only once, in `restoreSplit`. Reset and drag still write through `layoutOf`.
- **QA-17-r1-3 (Minor): fixed.** ADR-011 rev. 9 records F-17 at 4,715, the 715 B overrun as accepted, and a reserve of 3,564. The INDEX row and the history entry are updated. The measured r2 entry of 54,919 fits.
- **QA-17-r1-4 (Minor): fixed.** New page-level tests:
  - The axis and split values are written to storage, `document.cookie === ''`, there is exactly one fetch, and neither value appears in the fetch calls.
  - An indoor workout in between keeps `wx.axis = distance`, with `localStorage.length === 1`, and the next outdoor workout starts on distance.
- **QA-17-r1-5 (Minor): fixed.** `lib/prefs` is in the TD layout tree (1f72a3e).

## Questions

None.

## Coverage

- AC 9 now passes on every accepted corpus file, including the 6 local real files (backend invariant). It also passes on all synthetic goldens through `lib/range` (frontend).
- AC 12 and AC 15 are traced to tests for the axis and the split. AC 14 has the load-order test.
- The other ACs were closed in r1 and the diff does not touch them, so I did not re-verify them.
