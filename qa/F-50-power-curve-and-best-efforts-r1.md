# QA F-50 Power curve and best efforts - round 1

- **Date:** 2026-11-14
- **Verdict:** ready. No Blocker or Major. Four Minor findings: a false premise in EF-6 (the curve is not non-increasing), the import condition no longer matching the architecture doc, edit-equals-public tested against a TS-cut copy only, and the AC 8 phone timing measured in Node only.
- **Inputs:** `features/F-50-power-curve-and-best-efforts.md`; spec AP-11; decision log OQ-189 to OQ-194 in `features/README.md`; `architect/F-50-power-curve-and-best-efforts.md` (EF-1 to EF-9); ADR-011 item 4 (F-50 ceiling 150). Commit `58b5ea5`, working tree clean.

## Build and test results

- `make ci`: exit 0. It covers lint, svelte-check, Go tests, vitest, build, size, caddy-check and docker-check.
- vitest: 74 files, 1,923 tests passed. `src/lib/efforts` and `src/efforts`: 21 tests passed.
- `npm run size`: entry `index.html` and `edit.html` 57,176 / 60,000 gzip, PASS. That is +142 over 57,034, within the 150 ceiling. ADR-011 records it.
- The efforts chunk is 2,610 gzip JS and 369 gzip CSS. `Best efforts` and `curveDurations` appear only in `assets/efforts-*.js`, not in the entry.
- `ride-12h` curve in Node: 298 durations in 37.7 ms (the test's own log).
- **Mutants**, run in a scratch copy of `58b5ea5`. 8 of 8 meaningful mutants were killed:
  - ties go to the latest start;
  - missing power carried from the previous sample instead of 0 W;
  - the inward range instead of the covering range (power);
  - elapsed time instead of moving time (distance);
  - the pause-start skip removed;
  - transitions allowed for power;
  - window ends snapped to samples (the interpolated-start pass dropped);
  - the edit page given no window (the untrimmed window).
  - A first "missing power skipped" mutant (`continue` on NaN) survived. It is equivalent to 0 W in this grid, so it is not a test gap.

## Findings

### Minor

- **ID:** QA-50-r1-1
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `architect/F-50-power-curve-and-best-efforts.md` EF-6
- **Relates to:** EF-6, AC 2
- **Evidence:** EF-6 says "the curve is non-increasing, so no peak hides between grid points". That is false for mean-maximal power. On `[10, 0, 10]` W the best 2 s mean is 5 W and the best 3 s mean is 6.7 W. Only `MMP(k·d) ≤ MMP(d)` holds. The engineer measured a 0.02 W rise on `ride-12h`. The test now asserts only that the 1 s value is the top (`efforts.test.ts`, the "ride-12h" test).
- **Consequence:** None visible today; at 3 % steps a missed bump is sub-pixel on real data. But the doc's reason for calling the grid exact is wrong. Reword it to "an approximation; between grid points the curve can rise slightly, invisible at plot resolution". I accept the code as it is.

- **ID:** QA-50-r1-2
- **Severity / kind:** Minor / defect (doc drift)
- **Route:** frontend (small lane: one line in the architecture doc)
- **Where:** `src/web/src/page/start.ts:240`; `architect/F-50-…md` "Import condition"
- **Relates to:** EF-7, ADR-011
- **Evidence:** The doc's condition requires the moving-time stream, and limits distance sports to running, walking and hiking. The code checks neither: it uses `isPaceSport`, which adds swimming and rowing, and `distanceAdvances`. A pool swim with distance, a row, or a file without moving time therefore downloads the chunk (about 3 KB gzip) and then shows no section. The engineer chose this to save entry bytes. That is an acceptable trade, but the doc does not record it.
- **Consequence:** The doc and the code disagree on a fact (one home per fact). The extra download is harmless.

- **ID:** QA-50-r1-3
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/lib/efforts/efforts.test.ts`, the "AC 5" test
- **Relates to:** AC 5, EF-1, architecture Verification "Edit equals public"
- **Evidence:** The test compares the window's efforts with a copy that TypeScript slices and rebases itself. It covers power rows only: no distance efforts, no curve. It never touches the Go-published file. `workout.Trim` recomputes moving time on the cut from clipped timer events (`src/server/internal/workout/trim.go:123-141, 161`), so it does not simply rebase it. It also carries distance at the cut from the last valid value before it, while the TS `carriedDistance` fills a leading NaN forward. I read both paths and found them equivalent for moving time from speed or from timer events. They differ at the edge only when the trim lands on a NaN distance. I did not run a Go-built comparison.
- **Consequence:** A later change to the Go moving-time rule could make the edit page show different efforts than the published page, and no test would catch it. The edit DOM test (`edit.dom.test.ts`, "F-50 EF-1") checks row names only, not values.

- **ID:** QA-50-r1-4
- **Severity / kind:** Minor / gap (verification)
- **Route:** frontend (manual measurement by the user)
- **Where:** AC 8
- **Relates to:** AC 8, EF-7, NFR Performance
- **Evidence:** No measurement on the reference phone: curve ready within 1 s of insertion, LCP and 60 fps scrubbing unchanged on `ride-12h`. The User Timing mark `wx-efforts:curve` is in place (`src/web/src/efforts/index.ts`). In Node the whole computation takes 37.7 ms. Slices are at least one duration each and yield with `setTimeout`; the synchronous part before the tables is O(N). By construction the risk is low. F-49 r1-3 was handled the same way.
- **Consequence:** AC 8's timing parts are verified only by construction and the Node timing. One phone reading of `wx-efforts:curve` would close it.

## Deviations listed by the engineer

| Deviation | Judgement |
| --- | --- |
| Looser import condition than the doc (bytes) | Acceptable; record it in the doc (QA-50-r1-2) |
| Pause-end handling: a stretch starts at a pause's last sample and ends at its first | Correct and needed for the earliest-start tie rule; tested ("a stretch starts after a pause…"), and the mutant was killed |
| Curve not strictly non-increasing (0.02 W on `ride-12h`) | Mathematically expected; the doc is wrong, not the code (QA-50-r1-1) |
| Grid `floor(d×1.03)`, 298 durations | Fine; the test checks steps ≤ 3 % and the five table durations; cost stays far inside budget |
| Rounding (time to whole seconds, W to whole watts, pace from the unrounded time) | Fine |
| Wording picks | Not judged; the architecture's hand-off leaves wording to the user's on-screen review |
| Edit-equals-public tested against a TS-cut copy | Weaker than the doc asks for (QA-50-r1-3) |

## Previous findings

None (first round).

## Questions

None that block. QA-50-r1-1 needs only a wording change from the architect.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 12 km run, hand-computed | `distanceEfforts` | `efforts.test.ts` "distance efforts" (irregular sampling, a pause inside the fastest km, values to 1e-9) |
| 2 ride with power / without / swim | `powerLegs`, `powerEfforts`, section | `efforts.test.ts` AC 2; `efforts.dom.test.ts` AC 2 |
| 3 pause excluded; no crossing legs in `triathlon` | EF-3 grid, `effortLegs` | the pause tests; AC 3 corpus test |
| 4 tap selects range | `selectRow` (shared with laps), click handler | `efforts.dom.test.ts` AC 4 (through the store; charts, map and panel follow the store as in F-18) |
| 5 trim only; hiding changes nothing | `pub` window, edit `zoneSource.window` | AC 5 lib test, edit DOM test. Hiding: by inspection only; nothing in `lib/efforts` or `src/efforts` reads hide settings |
| 6 imperial pace, names stay | `paceText`, units subscription | `efforts.dom.test.ts` AC 1, 6 |
| 7 truncated files | `effortLegs` fallback | 4 cut goldens (lib) plus `triathlon-cut-90` (DOM) |
| 8 not in entry; LCP, fps, 1 s | lazy `import()` after `ready()` | size check PASS, chunk grep; phone timing not run (QA-50-r1-4) |

Also checked and found correct:
- Taps work only after the range panel mounts (`ready()` runs after `laps.taps()`).
- A rebuild stops the older curve run (run id).
- No `style` attributes; the SVG `innerHTML` holds only constants and numbers (CSP, ADR-032).
- No listeners need removal for the page's lifetime.
- NaN distance is carried and NaN power counts as 0 W.

Not run:
- reference-phone timings;
- `go test -race` (no Go change in this commit);
- a Go-built trimmed file comparison.
