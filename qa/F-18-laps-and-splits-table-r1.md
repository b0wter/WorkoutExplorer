# QA F-18 Laps and splits table - round 1

- **Date:** 2026-10-29
- **Verdict:** ready. No Blocker or Major. Four Minor findings and one preference are listed below for the small lane.
- **Inputs:** `features/F-18-laps-and-splits-table.md`; `architect/F-18-laps-and-splits-table.md` v0.2 (L-1 to L-8); F-17 design (range values table, R-2); decision log 29 Oct 2026 (OQ-75 to OQ-80); spec AP-7, AP-8, AP-17; TD glossary. Commits 44c9428 (B-1) and ce01f9e (frontend) on master at 4185966, clean tree. User hand check on iPhone and desktop, 29 Oct (reported by the main session; not repeated, there is no browser on this machine).

## Build and test results

- `make ci`: exit 0. `go vet` clean, `svelte-check --fail-on-warnings` clean, `go test ./...` all ok, vitest 48 files / 1,215 tests passed, build ok. Size: entry `index.html` 55,986 / 60,000 B gzip PASS; upload 2,275 / 8,000 PASS. F-18 delta against F-17 r2's 54,919: +1,067 B, inside the 1,500 share.
- `go test -race -count=1 ./internal/workout ./internal/page ./internal/publish ./internal/datafile`: all ok.
- `make corpus`: 55 ok, 0 diff, 0 fail, 0 new, 0 stale; M/G invariants ok on every accepted file; 6 local real files exercised (local only).
- Privacy: `git ls-files` under `corpus/real` and `expected/real` lists only `.gitkeep`. No real-file figures in this report.

## Findings

### Minor

- **ID:** QA-18-r1-1
- **Severity / kind:** Minor / defect (question to settle)
- **Route:** architect
- **Where:** `src/server/internal/workout/laps.go:73` (`g := s.Gain[b] - s.Gain[a]`, no check for altitude samples in the range); same rule in `src/web/src/lib/range/index.ts` (`out.gain = gain ? gain[b] - gain[a] : NaN`)
- **Relates to:** AP-8 ("missing metrics hidden, never zero"), F-17 design line 45 ("A value with no samples in the range shows '–', never 0"), L-1, OQ-78
- **Evidence:** the triathlon sample's swim lap and both transition laps changed from "–" to "0" gain in `testdata/page/triathlon.json` (44c9428). `corpusgen/f08.go` `triathlon()` writes no altitude in the swim or the transitions, so these laps have no altitude sample at all; the computed gain is a flat `G` difference, not a measurement. Heart rate, power and cadence return nil (shown "–") in the same situation; gain alone shows 0. The "transition laps get ascent 0 m" deviation is the same mechanism.
- **Consequence:** an open-water swim lap (and every transition lap) shows "0" gain on real triathlons where the device recorded no ascent; the range panel does the same for such a range. Settle: either gain is "–" when the range has no valid altitude sample (both languages, panel and laps), or the F-17 sentence is amended to exempt gain. AC 6 ("other columns filled") is satisfied either way only if gain is explicitly exempted for transitions; say which.

- **ID:** QA-18-r1-2
- **Severity / kind:** Minor / undocumented deviation
- **Route:** architect (one line in L-4)
- **Where:** `src/web/src/lib/range/index.ts` `splitBounds` (the `while (...) k++` loop); test `splits.test.ts` "a jump over several multiples ends one split"
- **Relates to:** L-4, feature scope ("Split k ends at the first sample at or beyond k km")
- **Evidence:** `splitBounds([0, 900, 3100, 3500], 1000)` returns `[0, 2, 3]`: row 1 is 3.1 km, row 2 is the 0.4 km partial. L-4 as written gives boundaries 1 to 3 on sample 2, i.e. two empty single-sample splits.
- **Consequence:** after a distance jump the row number no longer equals the kilometre. The engineer's choice is reasonable (no empty, unselectable rows), but it contradicts the accepted decision text. Record it in L-4 (small lane).

- **ID:** QA-18-r1-3
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:115` (`enhanceLaps` before mount) versus `start.ts:153` (`mountRangePanel` after the charts' first paint); `src/web/src/page/laps.ts:97-109`
- **Relates to:** L-7, OQ-75, AC 8
- **Evidence:** read only. Row clicks are live as soon as the data file is decoded. A tap before the charts' first paint subscribes the L-7 listener before the glue's slot listener exists; the flush measures no move (`scrollBy(0, 0)`), then `mountRangePanel` subscribes, gets the range synchronously and shows the slot. The row moves by the panel's height without compensation. Not reproduced (no browser); the window is short.
- **Consequence:** AC 8 can fail on a slow phone if the viewer taps a lap within the first frames. Fix: attach the row listener (or the L-7 path) only once the panel is mounted, or have the glue's listener own the compensation.

- **ID:** QA-18-r1-4
- **Severity / kind:** Minor / doc hand-off not done
- **Route:** requirements (main session)
- **Where:** `spec/Workout Explorer — Technical Design.md:467` ("Session / lap ... with its totals as recorded") and `:476` ("Device totals")
- **Relates to:** F-18 design hand-off "Requirements", L-1
- **Evidence:** grep; neither entry mentions that lap totals are computed where the device recorded none. F-03 §5.3 was updated in 44c9428.
- **Consequence:** the glossary contradicts the data file's lap meaning.

### Preference

- **ID:** QA-18-r1-5
- **Severity / kind:** Preference
- **Route:** frontend (no action required)
- **Where:** `src/web/src/page/laps.ts:115` (`stats ??= createRangeStats(data)`)
- **Relates to:** L-4, the declared deviation "splits hold their own RangeStats"
- **Evidence:** a second set of prefix arrays (heart rate, power, cadence, temperature, carried distance) next to the panel's; built lazily on the first switch, so load and drag paths are untouched.
- **Consequence:** a few MB more on a 43,000-sample workout once splits are opened. Acceptable; would only matter if memory on old phones becomes an issue.

## Deviations judged

- Transition laps get a computed ascent: matches AC 6 and the range rule, but see QA-18-r1-1 (the value is 0 because there is no altitude, not because there was no climb).
- Lap values computed from float streams, not quantised ones: acceptable. The summary is computed the same way, and `splits.test.ts` bounds the difference to the stored step plus 3-decimal rounding for every L-1 key on `ride-laps-no-totals`.
- A GPS jump over several split lengths ends one split: acceptable, document it (QA-18-r1-2).
- No click-after-scroll guard: acceptable. Browsers fire no click after a touch scroll or a pan of an overflow container. Covered by the user's hand check, not by a test.

## Previous findings

None (first round).

## Questions

1. QA-18-r1-1: for a range or lap with no valid altitude sample, is gain "–" or "0"? Settled by a line in F-17's range table (and L-1) saying which. If "–", both `lib/range` and `workout/laps.go` need a presence count for altitude.

## Coverage

| AC | Code | Test | Result |
| --- | --- | --- | --- |
| 1 10.4 km: 10 + 0.4 km; 6 mi + partial | `splitBounds`, `laps.ts` build/units rebuild | `splits.test.ts` AC 1; `laps.dom.test.ts` AC 1 (imperial rebuild, selection kept) | pass |
| 2 5 device laps, device values | `completeLaps` keeps device keys; `lapsGroup` | `TestCompleteLapsKeepsDeviceValues` (full and partial device laps); triathlon page sample (5 laps) reproduced by Go and `lib/page` | pass |
| 3 single lap, splits work | `laps.ts` | `laps.dom.test.ts` AC 3; `splits.test.ts` AC 3 | pass |
| 4 no distance: laps, no switch | `distanceAdvances` guard | `laps.dom.test.ts` AC 4 (absent and non-advancing) | pass |
| 5 no laps: "No laps recorded.", splits | `lapsOrNone` (Go), `LapsTable.svelte`, template | `TestLapsMarkup`, `TestNoLapsModule`, `page_test.go`, `laps.dom.test.ts` AC 5 | pass |
| 6 transition lap "–" for speed, cadence, power | `completeLaps` `inTransition(l.Start)` (same test as R-13) | `TestCompleteLapsTransition`; `laps.dom.test.ts` AC 6 | pass (gain: QA-18-r1-1) |
| 7 tap selects lap range | `laps.ts` click handler, `(start_index, end_index − 1)` | `laps.dom.test.ts` row taps (incl. 1-sample lap, mismatched rows, same row again) | pass in jsdom; charts/map/panel by user hand check |
| 8 tapped row stays in place | `laps.ts` L-7, `overflow-anchor: none` | `laps.dom.test.ts` L-7 (mocked layout) | jsdom pass; real layouts by user hand check only (QA-18-r1-3) |
| 9 starts on laps, not stored | in-memory flag | `laps.dom.test.ts` AC 9 | pass |
| 10 laps visible without JavaScript | template | `TestLapsMarkup` (published page); `hygiene.test.ts` (empty slot, no switch in markup) | pass |

Also checked and correct: L-1 key set (no `timer_time`, descent, maxima filled; `notCompleted` asserts it); laps with 0 or 1 samples get nothing; hiding changes no lap value (`TestCompleteLapsUnchangedByHiding` checks that a cut actually happened); `SampleRange` now has a single Go home, used by `datafile`; brute-force reference in `laps_test.go` independent of `laps.go` helpers; the server table is adopted, not re-rendered (`page.dom.test.ts` outerHTML comparison); the units enhancer ignores the splits table; the drag path costs O(1) per flush (marked-row comparison only); the switch is static markup with no inline script or style (CSP); nothing is stored in `lib/prefs`; Go and `lib/page` evaluate `distanceAdvances` the same way.

Not run: real browsers (none on this machine). Touch click suppression after scroll and the L-7 behaviour in WebKit rely on the user's hand check of 29 Oct.
