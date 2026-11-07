# QA F-25 Route colouring by metric - round 2

- **Date:** 2026-11-07
- **Verdict:** ready with conditions - no Blocker or Major; conditions are the operator's on-screen checks (palettes on real tiles in both themes, legend at 320 px, AC 5 scrub record, AC 8 heart-rate ride)
- **Inputs:** `features/F-25-route-colouring.md` (incl. new AC 8, AC 9), `architect/F-25-route-colouring.md` v0.2 (C-4, C-5, C-6, C-11, Budget), ADR-037 rev. 1; commit 623f915 diff against 003f042; QA r1

## Build and test results
- `make ci` at 623f915: exit 0. Vitest 58 files, 1,445 tests passed; lint, Go tests, build, size, caddy-check, docker-check ok.
- Size (`npm run size`), 003f042 -> 623f915: entry `index.html` 57,399 -> 57,527 B gzip (+128, cap 600 from the reserve; 60,000 budget PASS). Lazy map-view JS 289,123 -> 289,173 (+50), map-view CSS 10,942 -> 11,017 (+75); within F-25's 3,000 B lazy cap.
- Probe (scratchpad `q.mjs`, a copy of `colourScale`/`classify`): see QA-25-r2-1.

## Findings

**QA-25-r2-1** - Minor / defect - Route: frontend - `src/web/src/lib/colouring/index.ts` (`classify`, `reverse` branch)
- Relates to: C-4 ("quantiles do not change under the monotonic speed <-> pace map"), C-5 ("a run reads like a ride"), AC 4.
- Evidence: the "class = number of edges <= value" rule breaks ties upwards in the value; for pace the value is s/m and the class is reversed, so ties break towards slow. Same speed data, 50/50 two speeds: ride slow 5 / fast 11, run slow 0 / fast 6. 30/70: ride 3 / 11, run 0 / 3 (the fast 70 % of a run in a low, blue-teal class). Constant metric: ride class 11 (red), run class 0 (blue).
- Consequence: with tied values a run and a ride with the same profile colour differently, and a constant pace reads "slowest". Rare on real runs (smoothed float speeds seldom tie), so Minor. Fix: in the reverse case count edges strictly below the value; gives ride-like placement.

**QA-25-r2-2** - Minor / risk - Route: frontend - `src/web/src/components/map/map-style.ts:91`, `src/web/src/page/page.css` (three theme blocks)
- Evidence: I compared all 23 light fallbacks with page.css by hand and they match; the two dark blocks are identical. No test checks this (`map-theme.dom.test.ts` checks only the read-back). The palette is now 69 hand-synced values.
- Consequence: a later palette tweak can drift silently (the pre-paint/no-CSS fallback differs from the CSS). A small hygiene test that parses page.css would catch it.

## Engineer's deviations and open choices
- Unit beside the select (C-6 places it in the legend): a deviation in placement only. It is shown and cleared with the legend and is tested. Acceptable; record it in one line in C-6.
- Gradient legend with four labels (-12, -5, 5, 12) at edges 1, 3, 8, 10 of 11: within C-6's limit of at most five. The CSS percentages match the edge positions.
- End labels aligned to the bar ends: consistent with the <= / >= marks (the end classes are open-ended). Median at 50 % is exactly edge 6 of 12. Fine.
- Constant metric in the top class: C-4 only says "one class", so this is allowed, but it is sport-dependent (see r2-1).
- Low-end ties skip classes: this is what C-4 says ("ties leave the classes between them empty"). The engineer's "fast part in class 7" is off by one: my probe gives class 6 for a 50/50 run.

## Previous findings
- QA-25-r1-1 (static cost not in design): fixed. The Budget table records it, and +128 B is measured, under the 600 B cap.
- QA-25-r1-2 (pace stop removed before the mean): fixed, documented in the values table.
- QA-25-r1-3 (no map-view test for gradient and range): fixed. `map-view.dom.test.ts` has a gradient class/legend test and a range class test.
- QA-25-r1-4 (legend does not mark clipped ends): fixed. There is no clipping now, and the end labels carry <= / >= in the correct direction for pace (tests for run, swim, row in both unit systems).

## Questions
None for the architect. r2-1 is a code fix within C-4's stated intent.

## Coverage
- Equal-share edges: quantile index `floor(k*n/12)` on the drawn edges only. These are tested: exact twelfths on a ramp, about a twelfth on skewed data, only drawn edges count, ties monotonic, constant gives one class, empty gives NaN edges and dash labels. Speed sports keep stops out of the base but put them in class 0 (tested). For pace sports, stops become NO_CLASS before the mean (tested). NaN is excluded from the base (`v >= floor` is false for NaN).
- Class mapping: `c` 0-11 sequential and 12-22 diverging; NO_CLASS maps to NO_VALUE 23, which is left out of wx-colour and drawn in the route colour in wx-range. `CLASS_COLOURS` length = DIVERGING + 11 = NO_VALUE (tested). CSS `.wx-c0...c22` map to the right tokens.
- Legend: labels in metric and imperial for speed, heart rate, power, gradient, and pace for run, swim and row; units switch only relabels (no `setData`, tested).
- AC 1, 2, 6, 7 unchanged from r1 and not touched by the diff. AC 3, 4 automated. AC 5, 8, 9 and the palette constraints (C-5) are manual.
- Not run: no dev server (the user is reviewing on :18080), no on-screen check, no colour-vision simulation.

User checks: palettes on real tiles in light and dark (light yellow `--seq-6` visible on the light map, flat sand `--div-5` distinguishable from hidden-section grey, red end not confusable with magenta `--route`), with a red-green CVD simulation; legend at a 320 px map, longest labels (imperial pace, e.g. "≥ 10:44"), no overlap; AC 5 scrub record on `ride-12h` by speed and gradient on the phone; AC 8 heart-rate ride, flat vs climb clearly different; AC 9 legend in both themes on the phone.
