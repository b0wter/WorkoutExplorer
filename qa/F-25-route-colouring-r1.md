# QA F-25 Route colouring by metric - round 1

- **Date:** 2026-11-02
- **Verdict:** ready with conditions. No Blocker or Major. Conditions: AC 5 (60 fps scrub on `ride-12h` on the operator's phone, coloured by speed and by gradient) and an on-screen check of the palettes and the legend layout in both themes; nothing has run in a real browser.
- **Inputs:** `features/F-25-route-colouring.md`; spec AP-9, AP-3, DS-1, DS-2, NFR Performance; TD Code structure and front-end dependency rules; `architect/F-25-route-colouring.md` v0.1; ADR-037; ADR-011 (budget). Commit `213fd7e` (`git show HEAD`), normal lane.

## Build and test results

- `make ci`: exit 0. svelte-check 0 errors, 0 warnings; Go packages ok; vitest 53 files, 1,381 tests passed; size check `index.html` and `edit.html` entries 56,619 / 60,000 B gzip PASS; docker-check ok.
- Static delta (+253 B) and lazy delta (+2,635 B of 3,000) are the engineer's figures; I did not rebuild the parent commit. Current totals above are measured.
- Probe (scratchpad vitest, synthetic golden `ride-12h`, 43,200 samples, desktop Node): `offeredMetrics` 0.2 ms; full selection pipeline (edges, values, scale, classes, lines) 28 ms speed (first call, JIT), 13-15 ms for heart rate, power, gradient. Every drawn edge with a value gets a class; gradient lands in the three middle classes on the synthetic data, as expected.
- Probe: held distance (100 samples) followed by a jump gives NaN for the held samples and the regular gradient on both sides, never an extreme value.

## Findings

### Minor

- **ID:** QA-25-r1-1
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `architect/F-25-route-colouring.md` Budget; `src/web/src/page/page.css:35-50, 80-93`
- **Relates to:** ADR-011, C-5, C-8
- **Evidence:** The design's own C-5 puts the 28 palette properties in the static theme block, yet its Budget says the static change is "only the units prop wiring, under 50 bytes". Actual: +253 B static (engineer). Entry now 56,619 / 60,000 B.
- **Consequence:** The design records a wrong static cost and the static reserve shrinks by 5x the planned amount without a note. Either record the figure (one line), or declare `--seq-*`/`--div-*` in the lazy `map-view.css` (same `:root` and dark media query), which makes the static cost about zero; the colours are read only by map code.

- **ID:** QA-25-r1-2
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `src/web/src/lib/colouring/index.ts:125-127`; design "Values per metric"
- **Relates to:** C-3, DS-2
- **Evidence:** For pace sports the engineer removes stops (raw speed below `STOP_SPEED`) **before** the 10 s mean; the design's table does not say whether the stop test applies to raw or smoothed speed.
- **Consequence:** None now; the choice is reasonable (a stop never slows its neighbours' pace) but is a decision a later reader may question. One line in the design closes it.

- **ID:** QA-25-r1-3
- **Severity / kind:** Minor / defect (test gap)
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.dom.test.ts:1218-1302`
- **Relates to:** C-5, C-6, C-8, AC 5
- **Evidence:** The map-view tests pick only speed and heart rate. Nothing at map-view level picks gradient, so the `DIVERGING` offset passed to `colourData` and the gradient legend branch (`wx-l1`..`wx-l6` plus unit) are untested there (by inspection, not run: a mutant dropping the offset in `map-view.ts` would survive; `map-style.validate.test.ts` covers `colourData` with the offset only directly). The "no colouring on the scrub path" test covers the cursor; the range selection path is covered by review only.
- **Consequence:** A regression painting gradient in the sequential palette, or a range update rewriting `wx-colour`, would pass CI.

- **ID:** QA-25-r1-4
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/lib/colouring/index.ts:203-217`
- **Relates to:** AP-9, C-4, C-6
- **Evidence:** Relative scales label the end swatches with the 5th and 95th percentile, and values beyond them are clipped into the end classes, but the legend shows plain numbers (for example "117 ... 157 bpm").
- **Consequence:** A viewer may read the right-hand label as the workout's maximum. A "≤"/"≥" marker or the operator's on-screen review settles it.

## Previous findings

None (round 1).

## Questions

- **Palette contrast (operator):** the light theme's flat gradient class `#cfc6b0` and the dark theme's `#5c5850`, and dark `--seq-0` `#8a3a1c`, may be hard to see on the base map. C-5 leaves the values to the operator's review; answer: seen on screen in both themes, acceptable or replace.

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 HR without power: speed, HR, gradient, not power | `offeredMetrics` | `colouring.test.ts` offeredMetrics; map-view options test | traced, passing |
| 2 Indoor or fully hidden: no selector | control created only inside `mountRouteMap`, which needs a route; no offered metric gives no control | existing presence tests; "no offered metric: no control" | traced, passing |
| 3 Faster sections differ; legend in the units drop-down's units | `colourScale`, `classify`, `legend`; units subscription relabels without `setData` | monotonic/clipping tests; legend km/h and mph; map-view units switch | traced, passing; corpus ride on screen not done (operator) |
| 4 Run: label pace, legend pace | `metricLabel`, reversed pace scale, `formatSpeedOrPace` | label per sport; pace legend min/km; map-view run test | traced, passing |
| 5 60 fps on `ride-12h` with colouring | no colouring code subscribes to cursor or selection (reviewed `map-view.ts`); `setData` only in `choose` and `style.load` | cursor test asserts no `wx-colour` write | review only; phone run not done (condition) |
| 6 Hidden sections stay hidden | `recordedEdgeEnds`/`classLines` use positioned samples only | partition property on `gps-dropout`, `pause-hours`, hidden-ends variant | traced, passing |
| 7 Bridges stay dashed and uncoloured | `recordedEdgeEnds` uses `routeLinesIn`'s bridge rule (dropout or gap start) | `recordedEdgeEnds` equals `routeLines` recorded edges on goldens; map-view dropout edge count | traced, passing |

Also checked and correct: gradient gives no class at standstill, held distance and transitions, never ±∞ (test and probe); transition samples have no class for speed, power and gradient, a class for heart rate (as C-3); centred means never cross a time gap and never fill; `end_index` treated as exclusive, matching `decode.ts`; equal bounds give one class; layer order and `wx-colour` source in the base style and validated style; scheme change rebuilds the `match` expression, `style.load` re-applies the latest data; swatches use CSS classes, no `style` attributes (ADR-032); `lib/colouring` imports only `lib/datafile` and `lib/format`; `lib/route` stays metric-free; units listener removed in teardown; static and lazy budgets within limits.

Not run: any real browser, the phone scrub record (ADR-016), real corpus files (none present under `corpus/real/` in this checkout), mutation testing beyond reasoning about the gap in QA-25-r1-3.
