# QA F-17 Range selection and x-axis toggle - round 1

- **Date:** 2026-10-29
- **Verdict:** not ready - a whole-workout range misses the summary's moving time on 2 of 6 local real files (QA-17-r1-1). A stored split is applied only after the data file has loaded, so the content column reflows on every load for a viewer who has dragged the splitter (QA-17-r1-2).
- **Inputs:** `features/F-17-range-selection-and-axis-toggle.md` (incl. the 29 Oct ACs for the stored axis and split), `features/F-13-activity-page-layout.md` (phone-landscape AC), `features/F-14-units-and-theme.md` (DS-1 storage ACs), decision log 29 Oct 2026; `architect/F-17-range-selection-and-axis-toggle.md` v0.2, ADR-029, ADR-011, F-13 D-10/D-17; spec AP-5, AP-7, AP-17, AP-18, DS-1. Commits 3ff55ce, a9f8ca8, b249c0f, 23b73cd, 2e4303b, 3fa88fb, b44f8a5 (plus spec commits 9e34b1b, 7b2dbf9, a50fc52). Working tree clean at 3fa88fb.

## Build and test results

- `make ci`: exit 0. svelte-check 297 files, 0 errors, 0 warnings. `go test ./...` all ok. Vitest 46 files, 1,174 tests passed. Entry `index.html` 54,936 / 60,000 B gzip PASS; upload entry 2,275 / 8,000 B PASS. (npm warns about Node 22 against `engines >=24`; not new.)
- `go test -race -count=1 ./internal/workout ./internal/datafile ./internal/page`: ok.
- `make corpus`: 52 ok, 0 diff, 0 fail; real: 6 files (local only).
- Probe (scratchpad copy of the module, `zz_probe_test.go`, nothing in `src/` touched): for every accepted corpus file, the ADR-029 invariants on `Samples.MovingTime` / `Samples.Gain` and, after encode and decode, `M[N−1] − M[0]` against the rounded derived moving time. Synthetic: 38 ok, 0 bad (8 rejected by design). Real: 4 ok, 2 bad (see r1-1). Real figures as aggregates only.
- Not run: any browser (none on this machine). The user's hand check on iPhone and desktop (selection, sticky panel, toggle, splitter, readout) is taken as the manual verification for the layout and gesture ACs.

## Findings

### Major

- **ID:** QA-17-r1-1
- **Severity / kind:** Major / defect
- **Route:** architect, then backend
- **Where:** `src/server/internal/workout/moving.go:31-56` (timer branch), `src/web/src/lib/range/index.ts:150` (`moving[b] − moving[a]`); ADR-029 item 1
- **Relates to:** AC 9 ("for a whole-workout range on a corpus ride, moving time equals the summary's moving time"), ADR-029 Consequences, AP-7
- **Evidence:** ADR-029 defines `M[i]` as the moving time within `[0, Offset[i]]`, and the range is `M[b] − M[a]`. When the first sample is not at offset 0 and the timer already runs, `M[0] > 0`. In the probe, 2 of the 6 local real files (both timer source) have `M[0] = 1`, so the whole-workout range is 1 s short of the summary moving time. All synthetic files start at offset 0, so `range.test.ts` "AC 7: a whole-workout range has the summary moving time" passes on goldens only. `checkCumulative` in `cumulative_test.go` asserts `cum[0] == 0`, but only on hand-built recordings, never on the corpus.
- **Consequence:** AC 9 fails on real device files. The panel shows `h:mm:ss`, so the 1 s mismatch is visible. Avoiding this mismatch is the reason ADR-029 gives for its design. The definition is inconsistent: either `M` must start at 0 at sample 0 (time before the first sample goes to the first interval or is dropped from the range), or the AC must accept the gap. The architect decides which one. The backend then adds a corpus-level invariant (synthetic and local real), like the probe.

- **ID:** QA-17-r1-2
- **Severity / kind:** Major / defect
- **Route:** architect, then frontend
- **Where:** `src/web/src/page/start.ts:122-125` (`addSplitter` is called after `await loading`), `src/web/src/page/splitter.ts:33-41` (`restore`)
- **Relates to:** F-13 D-10 ("changes the user causes may move content; nothing else may"), F-13 AC 7 (no layout shift when the bundle hydrates), AP-17 (remembered split), F-17 AC 14
- **Evidence:** The server markup paints with the default split (`--map-w` unset). The stored split is written to `.layout` only after the data file has been fetched and decoded and RouteMap has been mounted. From that moment the map column and the whole right column (header, summary, notice, reserved chart area) change width, and the text reflows. Lighthouse runs with empty storage, so it will never see this. The F-17 design's splitter note and the 29 Oct decision do not mention load-time shift.
- **Consequence:** Every viewer who has stored a split sees the page jump on every load. On a phone in landscape that can be a second or more after first paint. This is the shift D-10 forbids. A cheap mitigation is to restore the split synchronously at the start of `startPage`, before any `await`. Even then it applies after the first paint whenever the bundle is not cached, so the architect decides whether that is enough or the shift is accepted and recorded in D-10.

### Minor

- **ID:** QA-17-r1-3
- **Severity / kind:** Minor / defect (process)
- **Route:** architect
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md` ledger (F-17 row still "4,000"); `architect/F-17-range-selection-and-axis-toggle.md` Budget
- **Relates to:** ADR-011, F-17 Budget ("up to 500 from the reserve ... Beyond that, stop and report")
- **Evidence:** Entry 54,936 B against the design's entry of 50,220, which is +4,716 for F-17 with its follow-ups (splitter, prefs, readout restyle, notch). That is 716 over the share, beyond the 500 allowance. At a9f8ca8 it was 54,114 (`CONTINUE.md`); the follow-ups added 822. The overrun is not recorded anywhere. The reserve is now about 3,564 (60,000 − 54,936 − F-18 1,500), still above the 1,000 trigger.
- **Consequence:** The ledger no longer matches CI. F-18 and later shares are planned on a reserve that is about 700 B smaller than recorded. Close F-17 in ADR-011 at the measured figure and accept the overrun explicitly.

- **ID:** QA-17-r1-4
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/page/units.dom.test.ts:175`, `src/web/src/page/splitter.dom.test.ts`
- **Relates to:** F-17 AC 15 (no cookie, no request carries either choice), AC 12 (an indoor file in between does not overwrite the stored axis)
- **Evidence:** The cookie and request checks exist for units only (`units.dom.test.ts:97`). For the axis and the split, no test asserts `document.cookie === ''` or that the fetch calls do not contain the values. No page-level test opens a no-distance workout with `wx.axis = distance` stored and asserts that the entry survives. By code reading all of this holds: `onaxis` fires only from the toggle, and `lib/prefs` is the only storage writer.
- **Consequence:** Low today. A later change that calls `onaxis` on mount, or a "time fallback" write, would pass CI.

- **ID:** QA-17-r1-5
- **Severity / kind:** Minor / doc
- **Route:** architect
- **Where:** `spec/Workout Explorer — Technical Design.md`, repository layout tree
- **Relates to:** TD code structure
- **Evidence:** `lib/prefs` is described in the Front end notes (line 352) but missing from the layout tree (known gap, confirmed).
- **Consequence:** Doc drift only.

## Previous findings

None (first round).

## Questions

1. (architect, r1-1) Should `M` be anchored so that `M[0] = 0`, which counts the timer time before the first sample into the first interval of the range or drops it from ranges? Or should AC 9 change to "within the time before the first sample"? Either answer settles it. The first keeps the AC as written.
2. (architect, r1-2) Is a split restored before the first `await`, still after first paint on an uncached bundle, enough under D-10, or should the shift be recorded as accepted for a remembered choice? Evidence that would change my mind: a trace from the phone with a stored split showing no shift on a cached reload.

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 toggle only with distance, cursor kept | `ChartStack.svelte` (toggle on `distanceX`), `lib/charts/axis.ts` | ChartStack dom tests, page samples `has-distance` | traced, pass |
| 2 climb stats vs reference | `lib/range` | `range.test.ts` brute-force reference on seeded data and goldens | traced, pass; map highlight by user hand check |
| 3 hidden start: map only visible part | `lib/route` sub-range, `map-view` `wx-range` | `route-range.test.ts`, map-view dom | traced, pass |
| 4 clear rules | gesture reducer, panel clear, `start.ts` slot visibility | `gesture.test.ts`, `page.dom.test.ts:258` | traced, pass; scroll and map tap by hand check |
| 5 hint, no panel before a range | ChartStack hint, `range-slot:empty` CSS | `page.dom.test.ts:248` | traced, pass |
| 6-8 panel placement per layout | `page.css` sticky rules | hygiene tests (CSS text) | manual (user hand check, iPhone and desktop) |
| 9 whole range = summary moving time; v2; 4 h under 500 KB | ADR-029 streams, `TestSizeBudget4h` | `range.test.ts:168` (synthetic only) | **fails on 2/6 real files (r1-1)**; v2 and budget pass (ride-4h 109,596 B br) |
| 10 12 h without lag | O(1) `query`, store coalescing, shading by transform, map `setData` debounced 150 ms | review | verified in code review |
| 11 scrubbing with a range | gesture reducer | `gesture.test.ts` (scrub with a range leaves it) | traced, pass |
| 12 axis remembered, indoor in between | `start.ts` `initialAxis`/`onaxis`, `lib/prefs` | `units.dom.test.ts:175` | traced; indoor-in-between untested (r1-4) |
| 13 broken storage | `lib/prefs` guards | `prefs.test.ts`, `units.dom.test.ts:149,158` | traced, pass |
| 14 splitter 30-70 %, per layout, reset clears | `splitter.ts`, `page.css` | `splitter.dom.test.ts` | traced, pass; load-time shift (r1-2) |
| 15 no cookie, no request | `lib/prefs` | units only (r1-4) | partly traced |
| F-13 phone-landscape two columns | `page.css:416` | hygiene tests, user hand check | traced |
| F-14 DS-1 stored units | `lib/prefs`, `start.ts` | `units.dom.test.ts:97-172` | traced, pass |

Backend B-1: M and G invariants hold on all synthetic files and on G for all real files. `-race` is clean. Hiding leaves both series unchanged (`TestCumulativeSeriesSurviveHiding`). The upload entry has no storage API (size check). Not checked in depth: the gesture reducer's touch paths beyond its unit tests, and the CSP effect of `style.cssText` (pre-existing pattern).
