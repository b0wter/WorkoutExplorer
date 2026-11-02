# QA F-05 Stacked charts with shared cursor - round 3

- **Date:** 2026-10-12
- **Verdict:** ready with conditions.
  - QA-05-r2-1, r2-2 and r2-3 are fixed, and this round has no new findings.
  - The user's touch decisions are implemented as the v1.4 design and ADR-010 rev. 2 describe them: option B (touch down sets the cursor), the scroll lock (D-18), and P2 not required.
  - The remaining conditions belong to the operator:
    - the phone checks the scroll lock touches (P1, P3, P5, P11, P12, P13), the readout check (P7) and rotation (P10);
    - the desktop checks still open from r1 and r2;
    - **AC 7**, the desktop 60 fps measurement, still pending in `MEASUREMENTS.md`.
  - AC 5 and AC 7 cannot be ticked until those runs are recorded.
- **Inputs:**
  - The F-05 design v1.4 (change log, D-16 to D-18, "Charts", "Interaction", "Readout", "Tests", "Manual checks, v1.4").
  - ADR-010 rev. 2, ADR-011 rev. 2 and ADR-012 rev. 2.
  - `features/README.md` (F-04 done, F-05 "qa r3").
  - `src/web/bench/MEASUREMENTS.md` (the F-04 phone row).
  - The uncommitted working tree on top of `699a1d1`, including `src/web/src/lib/charts/gesture.ts`, `src/web/src/components/charts/*` and the new `swap-props.test-helper.svelte.ts`.
  - `qa/F-05-stacked-charts-with-shared-cursor-r2.md`.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (repo, exit code captured) | **exit 0**. `svelte-check`: 203 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: `Test Files 14 passed (14)`, `Tests 495 passed (495)`. Bundle: `10613 / 60000 bytes gzip PASS`, unchanged |
| Bundle, reproduced in a scratch copy (the design's temporary imports) | Base **10,613**. uPlot only **32,972**. F-05 import **40,469**, which is **+29,856 / 30,000**. Own code 40,469 − 32,972 = **7,497 / 7,600**. Matches the engineer's numbers exactly |
| Test code kept out of the build | `swap-props.test-helper.svelte.ts` is imported only by `ChartStack.dom.test.ts`. Its name does not match Vitest's test pattern, so the test file count stays 14. None of the three builds contains `jsdom`, `vitest` or `swap-props` |
| Mutation run (scratch copy, `mut3.py`): the r1 and r2 sets, with obsolete patterns replaced, plus 20 new mutants for the touch-direction logic, the scroll lock, the chart-area height and the sticky readout | **96 run, 91 killed, 5 survived.** All four r2 survivors (QA-05-r2-3 a–d) are now killed. None of the survivors is a test gap; see "Mutation detail" |
| `git status --porcelain`, before and after | Identical, apart from this report and `qa/INDEX.md` |

### Mutation detail (new or changed in r3)

| Mutant | Result |
| --- | --- |
| Touch down does not set the cursor; 45° instead of 30°; axis re-classified after it was decided; 4 px slop; no cursor update on moves after a pan; `touchCount >= 1`; an `'open'` axis already blocks; the stale-touch replacement keeps the old axis | killed (`gesture.test.ts`) |
| `cancelable` check removed; `touchmove` registered passive; handler empty; touch count fixed at 1; `y` read from `clientX`; `touchmove` not removed on unmount | killed (`ChartStack.dom.test.ts`) |
| Chart-area height `$effect` removed; root custom properties not written; `chartAreaHeight` without the top padding | killed |
| r2 survivors: touch `pointerup` without an index; scroll listener removed without `capture`; `minWidth` → `width`; `delivered = null` after a listener error | **all killed** |
| Survived, no effect: `nearestIndex` `<`; resize coalescing; `3·dy² < dx²` instead of `≤` (the exact 30° boundary cannot be represented as a float, so the two cannot differ on real input); chart-area height computed `untrack`ed (`{#key}` creates a new element per `data` value, so the effect re-runs anyway) | survived, not a gap |
| Survived, out of jsdom's reach: `position: sticky` removed from the readout. ADR-012 excludes layout from these tests | manual check P7 |

## Findings

None. No Blocker, Major, Minor or preference findings.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-05-r2-1 (readout hid values on phones) | **fixed** in code; visual check P7 pending | See "QA-05-r2-1: wrap widths" below |
| QA-05-r2-2 (stack height after an in-place `data` swap) | **fixed** | See "QA-05-r2-2: in-place swap" below |
| QA-05-r2-3 (4 surviving glue mutants) | **fixed** | The new tests are: touch down and up at different x; capture flag normalised in the cleanup test, which also covers `touchmove`; `minWidth` set and `width` empty on every value cell; `set(null)` after a listener error. All four r2 mutants are killed |
| QA-05-r1-1 to r1-6 | fixed (r2), still fixed | Unchanged code paths. The r1 and r2 mutants are still killed |
| OQ-13 (cursor on mouse leave) | open, user | To be confirmed in F-07 |

### QA-05-r2-1: wrap widths

- The readout is now `flex-wrap: wrap` with auto height and no overflow rules. Cells are `flex: none` and `nowrap`. It is `position: sticky; top: 0; z-index: 1` on `#fff`, and it is a direct child of the stack root, so it sticks only inside the stack.
- **Wrap widths, by the r2 arithmetic:**
  - **390 px phone:** the bench page leaves 358 px for the stack. Minus 48 + 16 px padding, there are **294 px** for cells.
  - **375 px (iPhone 13 mini):** **279 px** for cells.
  - **Cell widths** at about 6.6 px per character at 13 px:

    | Cell | Characters | Width |
    | --- | --- | --- |
    | Time | 13 | ≈ 88 px |
    | Distance | 18 | ≈ 120 px |
    | Elevation | 17 | ≈ 115 px |
    | Speed | 16 | ≈ 108 px |
    | Heart rate | 18 | ≈ 122 px |
    | Power | 12 | ≈ 80 px |
    | Cadence | 15 | ≈ 100 px |
    | Temperature | 18 | ≈ 120 px |

  - **Line packing:** every pair in field order fits on one line with its 16 px gap; the widest pair is 239 px. Three cells never fit. So ride-12h takes **4 lines, about 78 px**, with all 8 values on screen and none clipped. This matches the design's "about 80 px".
  - The longest realistic cell (`Distance 1000.71 km`, about 130 px) still fits on its own on a 320 px phone, which leaves 224 px.
- **Sticky readout while scrubbing lower charts:**
  - The readout overlays the charts. It is `z-index: 1`; uPlot's `u-over` and `u-under` and the cursor lines have no z-index, so they paint below it.
  - It stays pinned while any part of the stack lies more than its own height below the viewport top. Only when the stack's bottom edge rises above about 80 px does the readout move up with it.
  - When the temperature chart sits right under the pinned readout, the top ~80 px of that 172 px chart are covered and that part cannot be touched. The rest of the chart can be scrubbed. This is acceptable, and P7 checks it.
  - Sticky positioning breaks under an ancestor with `overflow` other than `visible`. The bench page has none. F-13 must keep it that way, or move the readout.

### QA-05-r2-2: in-place swap

- The root no longer has a height.
- `div.chart-area` sits inside `{#key data}`. Its height is written by an `$effect` that tracks `area`, which is a new element for each `data` value.
- The new jsdom test swaps 6 → 3 → 6 charts and checks `chartAreaHeight` after each swap. It goes through a test-only `$state.raw` helper.
- My r2 probe scenario is now that test. The `$effect` removed mutant fails 4 tests.

## Scroll lock against browser behaviour (D-18)

The design's reasoning holds on every point I could check by reading the code. Where it depends on engine behaviour, the manual list checks it.

**Registration**
- `touchmove` is added once in `onMount` with `{ passive: false }`, on `u.over`, before any touch can start. It is removed with the same options; the mutant that skips the removal is killed.
- That satisfies WebKit, which decides whether a touch has a blocking listener at `touchstart`. A listener added mid-touch would not stop scrolling (WebKit bug 184251).
- Nothing adds the listener on `pointerdown`.

**`cancelable`**
- The handler calls `preventDefault` only when `e.cancelable` is true, and does so on every `touchmove` of a scrub, not only the first.
- That is correct for both engines:
  - Chrome sends `cancelable: false` once it has started a scroll or a fling, and checking first avoids its console intervention warning.
  - WebKit keeps `touchmove` cancelable until its pan has begun, so preventing each move keeps a later vertical drift from starting one. This is exactly the iOS behaviour behind the operator's P2 result.
- If the flag is false, the browser scrolls, sends `pointercancel`, and the reducer restores the cursor. That path is tested in the reducer.

**Event order**
- The axis is decided in the reducer, on a pointer event. The lock reads it on the touch event. So the design assumes a movement's `pointermove` arrives before its `touchmove`.
- Chrome dispatches in that order. I believe WebKit dispatches the pointer events for a touch before the touch event too, but I have not verified it in WebKit's source or in a browser.
- If the order were reversed, the lock would start one `touchmove` late, still well before a horizontal start can turn into a scroll. P11 is the check.

**Pinch**
- `blocksScroll` requires `touches.length === 1` (mutant killed), so a second finger is never prevented.
- One caveat: in both engines, a touch sequence whose earlier `touchmove`s were prevented may not be allowed to start a pinch. The browser may then neither zoom nor send `pointercancel` until the fingers lift.
- The design's P5 pass rule ("the page zooms, or at least nothing gets stuck") already allows for this. The stale-touch rule guarantees the next touch starts in the `open` axis state.

**Passive-listener cost**
- A touch that starts on a chart now waits for the main thread before the browser may scroll. In Chrome, the wait applies to the first `touchmove` of a touch starting in the listener's region. WebKit dispatches touch events to a non-passive listener synchronously.
- The handler does two comparisons and a property read (`e.touches` returns the event's cached `TouchList`), allocates nothing and reads no layout. So the cost is only the main thread's existing backlog.
- On the activity page, the charts cover most of a phone screen, so most scrolls will start on them. The risk is real when the main thread is busy. Decoding happens before the charts mount, so it is not a factor; MapLibre start-up and map rendering in F-06 are.
- The design names it and assigns the "sticky first scroll" check to F-06 and F-07. I agree, and I carry it as a note, not a finding.

**Mouse and pen**
- Mouse and pen leave the reducer `idle`, so they never block (mutant killed).
- On hybrid devices, a mouse event during a touch resets the state to `idle` and ends that touch's lock. That is harmless.

**Hot path**
- `onTouchMove` never calls `set`.
- The reducer's per-event objects are on the input path, as in r1.
- The flush path is unchanged.

## Questions

None new. OQ-13 stays with the user, for F-07.

## Coverage

| AC | Status |
| --- | --- |
| 1 Charts per file, aligned | Traced, and passing in Node and jsdom. **Operator: D1 passed** (iPhone 13 mini). Desktop alignment and gaps (D13) are still open |
| 2 Cursor in every chart, readout | Traced: reducer, store, jsdom pointer, tap and readout tests. The visible readout on a phone (P7) and desktop hover (D2) are still open |
| 3 Dropout is a break | Traced and passing. **D6 passed** |
| 4 Pause is a break | Traced and passing. **D7 passed** |
| 5 Vertical swipe scrolls on a phone | **P1 passed before the scroll lock existed. It must be re-run with v1.4**, because a non-passive listener now sits on every chart. The lock wiring is tested in jsdom |
| 6 Resize and rotation | Resize order and in-place swap are in jsdom. P10 rotation and D8/D14 are still open |
| 7 60 fps on desktop | **Still pending**: no browser row in `MEASUREMENTS.md` |

**Not run here:**
- a real browser or phone (OQ-12);
- brotli beyond the size script's column;
- real (non-synthetic) files, per ADR-004 item 7.

## Manual check list for the operator (round 3, final)

Open `/bench/charts.html` with `npm run dev`, or `vite --host` for the phone.

**Already passed, not repeated:** D1, D6, D7 and P4 (the latter superseded by P12). The rendering paths behind D1, D6 and D7 are unchanged.

**P1 and P5 must be re-run**, because v1.4 added the non-passive `touchmove` listener and the axis lock after they passed.

If an Android phone with Chrome is at hand, repeat P1, P3, P11 and P12 on it. Otherwise record "Android not checked".

### Phone (iPhone 13 mini, Orion; portrait unless stated)

| # | Check | Pass | Covers |
| --- | --- | --- | --- |
| P1 (re-run) | Vertical swipe starting on a chart; try the top, middle and bottom charts | The page scrolls, and the first scroll is not noticeably sticky | AC 5, D-18 |
| P3 | Touch down on a chart and hold; then a vertical swipe starting on a chart | The cursor jumps under the finger at once. After the swipe, the page has scrolled and the cursor is back where it was before (a brief flicker is fine) | D-7, user P3 B |
| P5 (re-run) | Pinch on a chart. Then start a horizontal scrub, put a second finger down and pinch | The first pinch zooms the page. In the second case, zoom is optional, but nothing gets stuck: after lifting both fingers, a vertical swipe on a chart scrolls and a horizontal drag scrubs | D-18 pinch |
| P7 (replaced) | ride-12h: scrub any chart, then scroll down to the Temperature chart and scrub it | All 8 values (Time, Distance and 6 metrics) are visible on about 4 lines, with no sideways scrolling and nothing clipped. The readout stays pinned at the top of the screen and updates while the lower charts are scrubbed. Note how much of the chart under the readout is covered | AP-6, D-17, QA-05-r2-1 |
| P8 | Switch to another app in the middle of a scrub, come back and drag again | Scrubbing works, and a vertical swipe on a chart scrolls (no lock left over) | ADR-010 rev. 1 and 2 |
| P9 | Scrub ride-12h | Smooth, no visible stutter (information; the formal phone measurement is F-07) | 7 (information) |
| P10 | Load ride-12h; rotate portrait → landscape → portrait | No page jump when the charts appear. After each rotation the charts redraw at the new width, the readout re-wraps, the cursor stays on the same sample, and there is no gap or overlap below the stack | AC 6, D-16 |
| P11 (new) | Start a clearly horizontal drag on a chart, then move the finger up and down while still dragging | The page does not scroll for the rest of that touch, and the cursor follows the finger's x. Lift, then swipe vertically from a chart: the page scrolls | D-18, event order |
| P12 (new) | Drags from a chart at roughly 20° and roughly 45° from horizontal | At 20°, the drag scrubs and locks as in P11. At 45°, the page scrolls | D-18, 30° rule |
| P13 (new) | Tap a chart without moving | The cursor lands where the finger touched and stays after lifting | AC 2, QA-05-r2-3 a |
| P2 (information) | A horizontal drag that drifts onto the next chart | Record whether the cursor keeps following. Not required (user decision) | ADR-010 rev. 2 |

### Desktop (Chrome or Firefox; DevTools console open except during the AC 7 run)

| # | File | Check | AC |
| --- | --- | --- | --- |
| D2 | ride-12h | Hovering any chart shows one line at the same x in all six charts. The readout shows Time, Distance and six values, with no `–`. The "second subscriber" index follows. Moving between charts causes no jump | 2 |
| D3 | ride-12h | Moving the mouse off the charts keeps the line and the readout (OQ-13). Click-drag also moves the cursor | 2 |
| D4 (replaced) | ride-12h | At 1,200 px or wider, the readout is one line, and its cells keep their width while scrubbing. Narrowing to about 600 px wraps the readout onto more lines, with no horizontal scrollbar and nothing clipped. Scroll so the lower charts are at the top: the readout stays pinned while you hover them | 2, D-17 |
| D5 | indoor-no-gps / no-distance-with-gps / brick-run-bike | indoor-no-gps: no Elevation chart. no-distance-with-gps: Elevation, Heart rate and Cadence only, and no Distance in the readout. brick-run-bike: no power line in the run part, the cursor line still drawn there, and `Power – W` in the readout | 1, 2 |
| D8 | ride-12h | Drag-resize narrower and wider: the charts redraw at the new width, stay aligned, and the cursor stays on the same sample. The lines are not blocky after widening. Very narrow: no console errors | 6 |
| D9 | ride-12h | Scroll with the mouse wheel while hovering, then hover again: the line is under the pointer | 2 |
| D10 | ride-12h | **AC 7:** follow the `MEASUREMENTS.md` procedure, steps 1–3: DevTools closed, mouse off the page, Auto-scrub 10 s. Add the row. Pass: at least 99 % of frame intervals at 20 ms or less, flush p95 at most 2 ms, max at most 8 ms | 7 |
| D11 | ride-12h | Performance panel during a 10 s hover: no long task over 50 ms, no dropped frames, no uPlot canvas draw while scrubbing. Optional: the same run with 4× CPU throttling, for information | 7 |
| D12 | all | No console errors or warnings during D2 to D14 | all |
| D13 | no-distance-with-gps, brick-run-bike, ride-12h | No page jump when the charts appear: the spacer text stays where it was at first paint. No gap and no overlap between the bottom chart's time axis and the next element | 1, D-16 |
| D14 | 3-chart file and ride-12h | Drag-resize: the chart-area height stays the same (the readout may re-wrap), and nothing overlaps the content below | 6, D-16 |

**Record:** add the AC 7 row (D10) to `src/web/bench/MEASUREMENTS.md`. For everything else, a pass or fail per item, with the browser and its version, is enough for QA to close F-05.
