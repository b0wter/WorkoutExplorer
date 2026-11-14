# QA F-51 Lap and split comparison chart - round 2

- **Date:** 2026-11-14
- **Verdict:** not ready - SC-11's pace edge limit clips the whole recovery group of an ordinary interval run with no rest lap, including the r1 example's own work/recovery laps, which drew correctly before the fix (QA-51-r2-1)
- **Inputs:** `git diff 46840da..HEAD` (`c217e69` design v0.2, `700665f` fixes and v0.3); `architect/F-51-split-comparison-chart.md` v0.3 (SC-5, SC-11) and its history; `features/README.md` decision log entry "comparison chart scale"; ADR-011 item 4; `qa/F-51-split-comparison-chart-r1.md`. Working tree clean at `700665f`

## Build and test results
- `make ci`: exit 0. svelte-check 0 errors, 0 warnings. `go test ./...` all packages ok. Vitest 72 files, **1,899 passed** (r1: 1,889). Build ok.
- Size: `entry edit.html: 57030 / 60000 bytes gzip PASS`, `index.html` the same. Splits chunk `splits-*.js` 10.10 kB / 4.55 kB gzip, `splits-*.css` 0.73 kB gzip. ADR-011 figures match: entry 57,030, F-51 +36 (57,030 − 56,994) within the 80 ceiling, reserve 2,970 (60,000 − 57,030), chunk 4,545 JS / 728 CSS.
- Probe: `lib/compare/index.ts` copied to the scratchpad and run under `node --experimental-strip-types` against 11 lap shapes (`f51probe.mts`, not committed). Results are in QA-51-r2-1.

## Findings

### Major

- **ID:** QA-51-r2-1
- **Severity / kind:** Major / defect (design rule, regression)
- **Route:** architect (SC-11 edge limit). The user decided the scale rule, so this is likely a user decision.
- **Where:** `src/web/src/lib/compare/index.ts:74` (`limit = pace ? -qr / 2 : qr`); SC-11 "in pace the fast edge is at most twice the reference speed (half-range ≤ half the reference pace)"
- **Relates to:** AP-23 goal, AC 2, SC-11 (its "known limit" covers only the case where more than a quarter of bars are outliers)
- **Evidence:** the pace axis is symmetric, so the fast-edge limit also caps the slow side. Any bar slower than reference/1.5 is clipped, however many such bars there are. Probe results, pace sport:
  - 6 × 4.5 + 5 × 2.5 m/s (the r1 shape without its rest lap): reference 4.5. All 5 recoveries are clipped, and the 6 work laps are at 0 deviation (2 px minimum). No bar is drawn to scale. In r1 this shape drew 51 px bars.
  - 10 × (5.0 work, 2.0 jog): 10 of 20 bars clipped, all at the same length. The accessible label lists 10 "Lap N beyond the scale".
  - 8 × (4.17 run, 1.4 walk): 8 of 16 clipped.
  - No wrong clipping in: a swim 10 × (1.4, 0.9), warm-up/cool-down + 5 × (5.0, 2.5), 8 easy + 4 sprints, hill reps, steady 1 km splits, ride 4 × (11, 7). A ride with one coffee-stop lap clips only that lap, which is correct.
  - With the limit relaxed to 0.9 × reference pace (fast edge at 10 × reference speed, every label still a real value), the first two shapes clip nothing and the r1 rest-lap test still clips its rest lap. Walk recoveries are still clipped; only an asymmetric pace axis would fix those.
- **Consequence:** in an interval run without a rest lap, the most common Laps-view use, half the bars become identical clipped stubs. The recoveries cannot be compared with each other, and the screen reader label becomes a long list. The new tests cover only the shapes with rest laps, so this case is untested.
- **Would change my mind:** the user sees the 10 × (5.0, 2.0) chart on screen and accepts clipped recoveries as intended.

### Minor

- **ID:** QA-51-r2-2
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:** `features/README.md` decision log, "Decided on 14 Nov 2026, comparison chart scale"
- **Relates to:** SC-11, one home per fact
- **Evidence:** the log says "capped at 3× the largest deviation among the other bars". That is the v0.2 gap rule's wording. v0.3 changed the meaning to the 75th percentile and added the edge limit. The log says nothing about either change, so its wording only works through SC-11's redefinition of "the other bars".
- **Consequence:** a reader of the decision log sees the superseded rule. It is also unclear whether the user approved the 75th-percentile rule and the edge limit, or only the v0.2 cap.

## Previous findings
- **QA-51-r1-1** (slow lap flattens the scale): **fixed** for the reported case. With 6 × 4.5, 5 × 2.5 and 1 × 0.3 m/s, only the rest lap is clipped, work and recovery bars are 44 % and 80 % of the half-height, and every axis label is a value (`compare.test.ts` "QA example", DOM test "SC-11: a standing rest lap ..."). SC-11 is implemented as written: 3 × the nearest-rank 75th percentile with at least 3 × the band edge, half-range at least the band edge, the edge limit (in pace at most 2 × reference speed; in speed the bottom edge stays ≥ 0), a break mark as a shape (`line.cut`, stroke `--bg`), the accessible label "Lap N beyond the scale" with Split and Length nouns, and class, hover, tap, median and best unchanged. The edge limit has a new side effect, reported as QA-51-r2-1.
- **QA-51-r1-2** (overlay focus): **fixed**. The header and `<select>` persist across draws. Options are rebuilt only when the offering changes, and there is one `change` listener. A DOM test asserts the same element and `document.activeElement`.
- **QA-51-r1-3** (AC 8-10 untested): **partially fixed**. AC 8 is covered by the edit page with a stored trim (bar count equals table rows) and by a trimmed file on the viewing page. AC 9 is covered by `ride-30min-cut-60` with no console errors. AC 10 (LCP, 60 fps on `ride-12h`) is still not measured; it holds by construction (r1). Acceptable as Minor.
- **QA-51-r1-4** (chunk size): **fixed**. ADR-011 records the splits chunk at 2,095 → 4,545 JS and 235 → 728 CSS gzip, plus the shared zone stylesheet at 308. This matches the build output.

## Questions
1. QA-51-r2-1: which fix for the pace edge limit? Suggestion: relax the limit to 0.9 × reference pace, so the fast edge is at most 10 × reference speed. This is a one-constant change, and the probe shows it keeps the jog-recovery shapes unclipped. The alternative is an asymmetric pace axis, which also keeps walk recoveries unclipped but changes SC-5's symmetric axis. Either way, add a test for an interval shape without a rest lap.
2. QA-51-r2-2: did the user approve v0.3 (the 75th percentile and the edge limit)? If yes, update the log wording. If no, show them the rule together with question 1.

## Coverage
- Diff reviewed in full: design v0.2/v0.3, history, decision log, ADR-011, `lib/compare`, `splits/chart.ts`, `splits/index.ts`, `splits.css`, and the new tests in `compare.test.ts`, `laps.dom.test.ts` and `edit.dom.test.ts`.
- SC-11 traced to code and tests: cap, band floor, edge limit in both pace and speed, the `clipped` flag, the break mark, and the label. The label's noun comes from `current.slice(0, -1)` (Lap, Split, Length). Splits and Lengths nouns have no test of their own.
- CSP: the break mark is SVG attributes plus a CSS class, with no `style` attribute. OK.
- Not run: browser, theme or screen-reader check of the break mark and label (the wording is still to be reviewed by the user on screen, per SC-11), Lighthouse, fps, mutants.
