# QA F-05 Stacked charts with shared cursor - round 1

- **Date:** 2026-10-11
- **Verdict:** ready with conditions.
  - No Blocker or Major findings. The pure modules (`lib/cursor`, `lib/charts`, `uplot-options.ts`) are well tested. Of 49 mutants, 46 were killed; of the 3 survivors, 1 is equivalent and 2 are test gaps (QA-05-r1-2). The component glue matches the design.
  - The conditions belong to the operator (OQ-12). The visual parts of AC 1 to AC 4, and all of AC 5, AC 6 and AC 7, can only be checked in a browser or on a phone, and none of those checks has been done yet. The manual check list is at the end of this report.
  - The five Minor findings and one preference below do not block. Each has a route.
- **Inputs:**
  - Feature `features/F-05-stacked-charts-with-shared-cursor.md`, and `features/README.md` decisions 5 and 6.
  - Spec: AP-4, AP-6 and the Performance NFR. TD Front end ("Implementation notes", budget table) and TD Workout data file ("Charts").
  - Architecture: `architect/F-05-stacked-charts-with-shared-cursor.md` v1.1 (change log read), ADR-008, ADR-009, ADR-010 rev. 1, ADR-011, ADR-002, ADR-007 rev. 1, ADR-004 item 7, `architect/overview.md` v1.6 and `architect/INDEX.md`.
  - The uncommitted working tree on top of `699a1d1`:
    - `src/web/src/lib/cursor/`, `src/web/src/lib/charts/` and `src/web/src/components/charts/`;
    - `src/web/bench/charts.html`, `src/web/bench/charts.ts` and the diff of `src/web/bench/MEASUREMENTS.md`;
    - the diff of `features/README.md` (the F-05 status cell only).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (repo, exit code captured) | **exit 0**. `svelte-check`: 201 files, 0 errors and 0 warnings. `go test ./...`: 8 of 8 `ok` (cached). Vitest: `Test Files 13 passed (13)`, `Tests 438 passed (438)`. Bundle: `entry index.html: 10613 / 60000 bytes gzip (9559 brotli) PASS`, unchanged from F-04, as D-12 requires. The manifest lists only `index.html` |
| Bundle measurement, reproduced in a scratch copy (`qa05/src/web`, `node_modules` symlinked), built with `vite build` and measured with `scripts/check-bundle-size.mjs` | Base: **10,613**. With uPlot and its CSS only: **32,972** (+22,359). With the design's temporary `ChartStack` + `createCursorStore` import: **40,043** (+29,430). F-05's own code: 40,043 − 32,972 = **7,071**. This matches the engineer's numbers exactly: **29,430 / 29,500** and **7,071 / 7,100**, both within the v1.1 ceilings. The margins are thin (70 and 29 bytes), but the ceilings close with F-05 (D-12). CSS is emitted as a separate file, not inline |
| Bench page excluded from production | The `vite.config.ts` build has no extra inputs. The manifest in both the repo build and the scratch build lists only `index.html`. Nothing in `src/` imports `bench/` (`grep`) |
| Mutation run (scratch copy, 49 mutants, `python3 -I mut.py`, each mutant runs its own test file) | **46 killed, 3 survived.** One survivor, `nearestIndex` with `values[mid] < target` instead of `<=`, is equivalent: an exact hit still returns the matching index. The other two are store mutants, reported as QA-05-r1-2. See "Mutation detail" below |
| Store, decimation and readout probes (scratch Vitest file, deleted afterwards) | Results are cited in QA-05-r1-1, QA-05-r1-2 and QA-05-r1-3. Decimating all 6 streams of ride-12h at B = 2,400 takes 33 ms cold and about 7 ms warm in Node. That is on mount and resize only, never on the scrubbing path |
| jsdom probe of the real components (my own script; jsdom from the session scratchpad, canvas stubbed, not in the repo) | brick-run-bike (5 charts, power starts late), pause-hours and ride-12h at 800 px: for indices 0, 1, N/3, N/2, N−2 and N−1, the `transform` of every cursor line is identical and equals `(t[i] − t[0]) / (t[N−1] − t[0]) × 736` px. Hovering each chart at the same `clientX` gives the same index on every chart. **Misaligned: 0** on all three files. This checks uPlot's geometry, not what a browser paints |
| `git status --porcelain`, before and after the round | Identical. I added only this report and the row in `qa/INDEX.md` |

### Mutation detail

| Area | Mutants | Result |
| --- | --- | --- |
| `decimate.ts` | No bucket clamp. Gap break at `x[i−1]`. Gaps ignored. `<=` / `>=` in min/max (last occurrence wins). Trailing break kept. Leading break allowed. Duplicate breaks. `0` instead of `null`. NaN skipped without a break. Points out of index order. `Math.round` buckets. Break off the missing sample's time. `bucketCount` with `floor` | 14 of 14 killed |
| `nearest.ts` | Tie goes to the higher index. No non-finite check. `<` instead of `<=` in the search | 2 of 3 killed (the third is equivalent) |
| `store.ts` | No compare before delivery. No pending flag. Synchronous flush. No initial call. No no-op on the same value | 5 of 5 killed. **2 more survived:** initial call with the last delivered value instead of `get()`, and an in-place `splice` on unsubscribe (QA-05-r1-2) |
| `gesture.ts` | Touch down sets the cursor. Stale-touch rule removed. Cancel does not restore. Primary check removed. Tap does not set. pointerId check removed. Touch leave resets state. Mouse up sets the cursor | 8 of 8 killed |
| `uplot-options.ts` | Padding. Y-axis size. X-axis size on every chart. Height without the top padding. y `incrs` one step too fine. 2.5 instead of 2 in the y steps. `space` 50. Last x `incrs` value dropped. Data-driven x range. `spanGaps: true`. `cursor.show: true`. `plotWidth` without padding | 12 of 12 killed |
| `specs.ts` / `format.ts` | Cadence and power swapped. All-NaN stream charted. NaN shown as `0` in the readout. Speed factor 3.5. `-0` not stripped | 5 of 5 killed |

## Findings

### Blocker

None.

### Major

None.

### Minor

- **ID:** QA-05-r1-1
- **Severity / kind:** Minor / defect (edge case)
- **Route:** frontend
- **Where:** `src/web/src/components/charts/CursorReadout.svelte:23-32` (`WIDTH`) and `:74-80` (`.value { overflow: hidden; text-align: right }`)
- **Relates to:** AC 2 ("updates the readout with all present values"), D-9; design "Readout"
- **Evidence:**
  - The readout value boxes have fixed widths in `ch`: elapsed time `8ch`, distance `6ch`, plus others.
  - Probe P6 built `readoutFields` on ride-12h with time shifted by 360,000 s and distance shifted by 1,000 km. The readout text was `elapsed="100:01:40"` (9 characters) and `distance="1000.71"` (7 characters).
  - Under CSS Text 3, a line that is too long for its box is start-aligned, so `overflow: hidden` cuts off the right end. The readout would show `100:01:4` and `1000.7`: wrong-looking values with no sign that they are cut.
  - No limit exists on workout duration or distance (F-08 and the TD set none), so a multi-day file or an ultra-distance ride (for example 1,200 km) hits this.
  - I could not see this in a browser. The string lengths are reproduced; the clipping follows from the CSS rules.
- **Consequence:** For files of 100 h or more, or 1,000 km or more, the readout silently shows a truncated value. Possible fixes: size the boxes from the longest value in the file at build time, which `readoutFields` can compute once, or allow one extra `ch`.

- **ID:** QA-05-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/lib/cursor/store.ts:61` (`listener(latest)`) and `:67` (copy-on-write unsubscribe); `src/web/src/lib/cursor/store.test.ts`
- **Relates to:** ADR-008 item 3; design "Cursor store" (rule "Listener list"; `subscribe` calls the listener "with get()")
- **Evidence:** Two design rules have no test that can fail:
  - **Initial value.** The mutant `listener(delivered)` survives all 22 store tests. No test subscribes between a `set` and its flush. Probe P1 shows the current behaviour, which follows the contract: a late subscriber gets `[4]` immediately, while the earlier subscriber still shows `null` until the frame. After the frame, the late subscriber has `[4, 4]`.
  - **Copy-on-write.** The mutant that replaces copy-on-write with an in-place `splice` survives. The existing test only covers a listener that unsubscribes a *later* listener. In that case `splice` shifts the array and the third listener is still reached by chance. With `splice`, a listener that unsubscribes *itself* during a flush makes the next listener miss that flush. My probe P4 asserts exactly that case; it passes on the real code and would kill the mutant.
- **Consequence:** F-06 will rely on both rules, because the map subscribes late and may unsubscribe while a flush is running. A refactor can break either rule with all tests still green. Add the two tests: subscribe between `set` and flush expecting `get()`, and self-unsubscribe during a flush.

- **ID:** QA-05-r1-3
- **Severity / kind:** Minor / risk
- **Route:** architect (policy), then frontend
- **Where:** `src/web/src/lib/cursor/store.ts:36-43` (`flush`)
- **Relates to:** ADR-008 item 3; design "Cursor store"
- **Evidence:** Probe P2: listener A throws on value 3, listener B is subscribed after A, and `set(3)` is followed by a frame.
  - The `requestAnimationFrame` callback throws.
  - B never receives 3, and still has not received it after another frame, because `delivered` was already set to 3. B stays stale until the value changes again.
  - Neither ADR-008 nor the design says what a listener must do on error.
- **Consequence:** Today the only listeners are the readout and the six charts, which are unlikely to throw. F-06 adds the map marker, which can throw while the style or source is not ready, and F-17 adds selection readouts. One throwing listener then freezes every view subscribed after it for that value, and the cause shows up somewhere else. Either catch per listener in `flush` and report the error once, or write "listeners must not throw" into ADR-008 as a review rule. The architect decides which.

- **ID:** QA-05-r1-4
- **Severity / kind:** Minor / risk
- **Route:** architect (state the contract), frontend (doc comment or handling)
- **Where:**
  - `src/web/src/components/charts/MetricChart.svelte:46-53`: `height` and `isBottom` are captured in `onMount`, and the `$effect` at `:181-184` tracks only `width`;
  - `src/web/src/components/charts/ChartStack.svelte:19-20` (prop doc).
- **Relates to:** design "Charts", Size row ("F-13 may change the height"); Boundary table (`chartHeight` prop for F-13)
- **Evidence:**
  - The `chartHeight` prop is read once, when each chart mounts. A later change reaches `MetricChart` as a new `height` prop, but nothing reads it.
  - `{#key data}` only rebuilds the stack when `data` changes.
  - The design says both "F-13 may change the height" and "Height never changes on resize". It does not say whether the prop is fixed for the life of a `data`.
- **Consequence:** If F-13 changes the chart height at a breakpoint, or the user toggles it, nothing happens, with no error and no warning. Either document "`chartHeight` is read at mount; change it together with `data` (remount)", or call `setSize` from the effect.

- **ID:** QA-05-r1-5
- **Severity / kind:** Minor / risk (test architecture)
- **Route:** architect
- **Where:** `src/web/src/components/charts/*.svelte` (about 400 lines). F-01 D-6 (Vitest in Node, no jsdom).
- **Relates to:** AC 2, AC 6; design "Tests" ("Not unit-testable")
- **Evidence:**
  - These behaviours have no test in CI:
    - `onready` fires exactly once, including when there are no charts or the width is 0;
    - the resize order (`setSize`, then bucket count from `u.bbox`, then `decimate`, then `setData`, then re-placing the line);
    - the cursor line is re-placed after a resize;
    - the rectangle cache is cleared on scroll and resize;
    - the `sampleCount` wiring check;
    - pointer event to reducer to `cursor.set`;
    - every listener is removed on destroy.
  - The engineer's 49-check jsdom smoke test and my alignment probe exercise all of them, but both live only in the session scratchpad. They will be lost and cannot catch regressions.
  - F-06, F-13, F-14 and F-17 will all edit these components.
- **Consequence:** A regression in the glue (for example `onready` firing twice, or a missing re-place after a resize) is only caught by an operator browser run, which happens once per feature at best. Either commit the smoke test as a non-CI dev script under `bench/`, or record in the F-05 design that the glue is covered by manual checks only. Either way it should be a written decision. Evidence that would change my mind: a decision that every later feature's QA re-runs the manual list in a browser.

### Preference

- **ID:** QA-05-r1-6
- **Severity / kind:** Preference / magic number
- **Route:** frontend
- **Where:** `src/web/src/components/charts/CursorReadout.svelte:61` (`padding: 4px 16px 4px 48px`) and `src/web/src/components/charts/MetricChart.svelte:197` (`padding-left: 48px`), against `Y_AXIS_SIZE = 48` in `uplot-options.ts:16`
- **Relates to:** D-6 (alignment)
- **Evidence:** The y-axis width is defined once in TypeScript and repeated twice as a CSS literal.
- **Consequence:** If F-14 widens the y axis (for example 5-digit elevation in feet), the plots stay aligned with each other, but titles and readout no longer line up with the plot edge. Pass it as a CSS custom property set from `Y_AXIS_SIZE`, or leave a comment that ties the two together.

## Previous findings

None. This is round 1.

## Questions

1. **For the architect (F-13):** `ChartStack` has no height until the first `ResizeObserver` frame. The readout renders first, and the charts appear one frame later, adding 6 × 144 + 28 = 892 px at the default height. That pushes everything below them down. Does F-13 reserve this height, for example from `presentCharts(data).length` in the pre-rendered HTML? **Settled by:** a line in the F-13 design that names who reserves the height.
2. **For the user (OQ-13):** the cursor stays where it is when the mouse leaves (D-8). The code does exactly this. **Settled by:** a confirmation in F-07, as the design plans.
3. **For the architect:** see QA-05-r1-3 and QA-05-r1-5. Each needs a one-line decision, not a design change.

## Code review notes (checked and found correct)

**Module rules**
- `uplot` is imported only in `components/charts`: as a value in `MetricChart.svelte`, and as a type only in `uplot-options.ts` and its test.
- `lib/cursor` imports nothing.
- `lib/charts` imports only `../datafile`, type only.
- `lib/` has no Svelte, DOM or `window` references. The only hit was `requestAnimationFrame` inside a doc comment.
- Components import every module through its `index.ts`.
- `grep -rn '\$cursor' src/web/src src/web/bench` is empty. No `$state`, `$derived` or `$effect` reads the cursor: the only `$state` values are `width` and `measured` in `ChartStack`, and both change only on resize.

**Hot path (D-10)**
- `flush` iterates a captured array with no allocation.
- `moveLine` writes one `transform` string, and toggles `visibility` only when crossing `null`.
- `u.valToPos` reads cached uPlot geometry; I checked this in the uPlot 1.6.32 source.
- The readout listener assigns `textContent` from formatters that are built once.
- No `setData`, `setSize` or `redraw` on the hot path, and no layout reads: `getBoundingClientRect` runs only on enter, on down, or once after the cache was cleared, all on the input path.
- The gesture reducer allocates one or two small objects per pointer event. The design defines the hot path as the flush, so this is allowed. I noted it, but it is not a finding.

**Resize ordering**
- uPlot's `_setSize`, then `calcSize` computes `bbox` synchronously even inside `batch` (`uPlot.esm.js:3357-3386`). The bucket count therefore always comes from the new width.
- After that come `decimate`, then `setData` (inside one `batch`), then `rect = null`, then `moveLine(cursor.get())`. This matches the design.
- `ResizeObserver` updates are coalesced to one `requestAnimationFrame`. On teardown the observer is disconnected and the pending frame is cancelled.

**Breaks**
- uPlot builds gaps in pixel space, from the previous non-null x to the next one (`findGaps`, `uPlot.esm.js:1951`), and clips them when `spanGaps: false`.
- Our break points sit strictly between their neighbours, so a dropout or a pause becomes an unstroked region and never a zero.
- `decimate` never emits `NaN` (property tests plus mutants).

**Interaction**
- All six listeners are passive and are removed with the same options. The window `scroll` listener uses capture, and is removed with capture too.
- There is no `preventDefault`, no `touch*` or `mouse*` listener, and no `on…` attribute in markup.
- `touch-action: pan-y pinch-zoom` reaches the built CSS on `.u-over`.
- uPlot binds no mouse listeners when `cursor.show` is false (`uPlot.esm.js:5988`).
- The stale-touch rule from ADR-010 rev. 1 is implemented and tested.

**Other checks**
- **Ties:** `nearestIndex` gives the lower index on an exact midpoint (decision 5). The tie mutant was killed.
- **Bench page:**
  - It uses synthetic golden files only (ADR-004 item 7).
  - The heart-rate dropout copies the stream and does not mutate the original.
  - The measurement subscribes a timing listener before and after the stack, as the design specifies.
  - The pass rule in the code matches the design: 99 % of frames at 20 ms or less, flush p95 at most 2 ms, flush max at most 8 ms.
  - The `MEASUREMENTS.md` row from the engineer's jsdom run is correctly labelled "Not AC 7 evidence".
- **Privacy:** no logging, no network calls other than the bench page fetching its local golden files, and no coordinates are used at all.

## Coverage

| AC | Code | Automated test | Status |
| --- | --- | --- | --- |
| 1 Six charts aligned; no power chart without power; no elevation chart without altitude | `presentCharts`, `CHART_ORDER`; fixed x range, padding and y-axis width in `uplot-options.ts`; `ChartStack` | `specs.test.ts` (goldens ride-12h, indoor-no-gps, no-distance-with-gps, all-NaN stream); `uplot-options.test.ts` (alignment inputs, all killed by mutants); jsdom probe, misaligned 0 | Logic traced and passing. **The visual alignment needs the operator.** No synthetic running file without power exists; no-distance-with-gps stands in, and is valid because chart choice does not depend on sport |
| 2 Hover or touch-drag moves the line in every chart and updates the readout | `MetricChart` handlers, `step`, `nearestIndex`, store, `moveLine`, `CursorReadout` | `store.test.ts`, `gesture.test.ts`, `nearest.test.ts`, `specs.test.ts` (`readoutFields` full resolution, `–` for NaN) | Logic traced and passing. **The browser check needs the operator** |
| 3 A heart-rate dropout is a break | `decimate` (break at the missing sample); `spanGaps: false` | `decimate.test.ts` (properties and hand cases; mutants) | Traced and passing. **The visual check (bench "ride-4h with heart-rate dropout") needs the operator** |
| 4 A multi-hour pause is a break | `decimate` (break at the gap midpoint) | `decimate.test.ts` golden pause-hours, at B = 1, 640 and 2,400 | Traced and passing. **The visual check needs the operator** |
| 5 A vertical swipe on a chart scrolls on a phone | `touch-action: pan-y pinch-zoom`; passive listeners; cancel restores | `gesture.test.ts` (cancel restores `before`) | **Not verifiable here: phone (OQ-12)** |
| 6 Resize or rotation redraws at the new width | `ResizeObserver`, then `requestAnimationFrame`, then `layout` | none in CI (QA-05-r1-5); engineer's jsdom smoke test | **Desktop resize and phone rotation needed (OQ-12)** |
| 7 Scrubbing the 12-hour file at 60 fps on desktop | Bench auto-scrub, frame monitor, flush timing | none (timings are not asserted, by design) | **Desktop browser run needed. The `MEASUREMENTS.md` row is still pending** |

**Requirements:**
- AP-4: order, present streams only, one shared x axis. Traced.
- AP-6, chart side: linked cursor, readout of every value, and touch gestures, which are assumptions per decision 6. Traced.
- TD Charts: min/max per pixel computed in the browser; full-resolution arrays are not changed (the "does not change its input" test). Traced.
- TD Implementation notes: one cursor store holding a sample index. Traced.
- NFR 60 fps: the code follows the hot-path rules; the number is pending the measurement.

**Not run:**
- No real browser and no phone are available on this machine (OQ-12).
- Brotli was not looked at beyond the size script's informational column.
- Real (non-synthetic) files were not used, per ADR-004 item 7.

## Manual check list for the operator

Start the dev server (`npm run dev` in `src/web`, or `vite --host` for the phone, as for the F-04 bench) and open `/bench/charts.html`. Record AC 7 in `src/web/bench/MEASUREMENTS.md`. For everything else, a short note of pass or fail per item is enough for QA r2.

### Desktop (Chrome or Firefox, window at least 1,200 CSS px wide, DevTools console open except during the AC 7 run)

| # | File | Check | AC |
| --- | --- | --- | --- |
| D1 | ride-12h | Six charts, in this order: Elevation, Speed, Heart rate, Power, Cadence, Temperature. The left and right edges of all plots line up vertically. Only the bottom chart has time labels (`h:mm:ss`), and the labels do not overlap. The y labels have no duplicates, and Speed never steps finer than 0.1 | 1 |
| D2 | ride-12h | Hover any chart: one vertical line appears at the same x in all six charts. The readout shows Time, Distance and six values, with no `–`. The "second subscriber" index follows. Moving from one chart to the next does not make the line jump | 2 |
| D3 | ride-12h | Move the mouse off the charts: the line and the readout stay where they were (OQ-13). Click-drag also moves the cursor (until F-17) | 2 |
| D4 | ride-12h | While scrubbing, the readout cells do not change width or jitter, and the content below the charts does not move | 2 |
| D5 | indoor-no-gps / no-distance-with-gps / brick-run-bike | indoor-no-gps: no Elevation chart. no-distance-with-gps: only Elevation, Heart rate and Cadence, and no Distance in the readout. brick-run-bike: no power line during the run part, the cursor line is still drawn there, and the readout shows `Power – W` | 1, 2 |
| D6 | ride-4h with heart-rate dropout | A hole in the heart-rate line only, from about 1:00:00 to 1:10:00. No drop to zero, and the heart-rate y axis does not stretch down to 0. Inside the hole the readout shows `Heart rate – bpm`, and the other charts are continuous | 3 |
| D7 | pause-hours | An empty stretch with no connecting line in every chart during the pause. Hovering inside the pause snaps the line to the nearest sample at one of its edges, which is expected | 4 |
| D8 | ride-12h | Drag-resize the window narrower and wider: the charts redraw at the new width, the plots stay aligned, and the cursor line stays on the same sample. After widening, the lines are not blocky (detail matches the new width). Very narrow (under 100 px): no console errors | 6 |
| D9 | ride-12h | Scroll the page with the mouse wheel while hovering, then hover again: the line is under the pointer, not offset | 2 |
| D10 | ride-12h | **AC 7:** run the procedure in `MEASUREMENTS.md`, steps 1 to 3: DevTools closed, mouse off the page, Auto-scrub 10 s. Add the row. Pass: at least 99 % of frame intervals at 20 ms or less, flush p95 at most 2 ms, flush max at most 8 ms | 7 |
| D11 | ride-12h | Confirmation (step 4): a 10 s hover scrub recorded in the Performance panel shows no long task over 50 ms, no dropped frames, and no uPlot canvas draw during the scrub. Optional (step 5): the same run with 4× CPU throttling, for information | 7 |
| D12 | all | No console errors or warnings during D1 to D11 | all |

### Phone (Android Chrome; iOS Safari as well if available)

| # | File | Check | AC |
| --- | --- | --- | --- |
| P1 | ride-12h | A vertical swipe that starts on a chart scrolls the page. Try it on the top, middle and bottom charts. After the scroll, the cursor is back where it was before the swipe (restored on `pointercancel`) | 5 |
| P2 | ride-12h | A horizontal drag on a chart scrubs: all lines and the readout follow. Dragging past the chart onto a neighbouring chart, or off the screen edge, keeps scrubbing | 2 |
| P3 | ride-12h | A tap places the cursor. Touching and holding without moving does not move it. Long-press: no text selection, callout or magnifier (worth noting for F-17) | 2 |
| P4 | ride-12h | Diagonal drags at about 30° and 60°: note whether each one scrolls or scrubs. This is the evidence named in ADR-010 "Would choose differently if" | 5 |
| P5 | ride-12h | Pinch on a chart zooms the page. After zooming back out, scrubbing still works. A second finger during a scrub causes no jumps | 2, 5 |
| P6 | ride-12h | Rotate portrait to landscape and back: the charts redraw at the new width, stay aligned, and the cursor line stays on the same sample | 6 |
| P7 | ride-12h | In portrait, the readout wraps over several lines without overlapping the charts | 2 |
| P8 | ride-12h | Switch to another app in the middle of a scrub, come back, and drag again: scrubbing still works (stale-touch rule, ADR-010 rev. 1) | 2 |
| P9 | ride-12h | Subjective smoothness of scrubbing. The formal phone measurement is F-07; note any visible stutter | 7 (information) |
