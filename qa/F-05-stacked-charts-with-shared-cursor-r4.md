# QA F-05 Stacked charts with shared cursor - round 4

- **Date:** 2026-10-12
- **Verdict:** ready. There are no open findings, and every acceptance criterion is backed by a passing automated test, a recorded manual check, or both.
- **Inputs:**
  - Commit `ff21c02` ("Add F-05 stacked charts with shared cursor") and the working tree on top of it.
  - `src/web/bench/MEASUREMENTS.md`, which holds the operator's AC 7 row and the manual check results for the r3 list.
  - `qa/F-05-stacked-charts-with-shared-cursor-r3.md`.

## Build and test results

| Command | Result |
| --- | --- |
| `git status --short` and `git diff ff21c02 --stat` | Only `src/web/bench/MEASUREMENTS.md` has changed (+7/−1). There are no untracked files. That change replaces the pending desktop row with the AC 7 row and adds a "Manual checks" section |
| `diff -rq` of `src/web` (excluding `node_modules`) against my r3 scratch snapshot | Only `MEASUREMENTS.md` differs. The committed code is therefore the code reviewed and mutation-tested in r3 |
| `make ci` (exit code captured) | **exit 0**. `svelte-check`: 203 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: `Test Files 14 passed (14)`, `Tests 495 passed (495)`. Bundle: `10613 / 60000 bytes gzip PASS` |

## AC 7 record against the pass rule

The pass rule comes from the design section "Performance". The run used Firefox 156.0 on Linux x86_64 at commit `ff21c02`, with ride-12h (N 43,200, 6 charts), a plot 2,217 px wide and DPR 1.5. It recorded 599 frames and 599 flushes.

| Criterion | Threshold | Recorded | Met |
| --- | --- | --- | --- |
| Share of frame intervals ≤ 20 ms | ≥ 99 % | 99.83 % (1 of 599 over, the 33.12 ms maximum, one missed frame) | yes |
| Flush p95 | ≤ 2 ms | 1.00 ms | yes |
| Flush max | ≤ 8 ms | 2.00 ms | yes |
| Desktop browser, plot at least 1,200 px | required | Firefox 156, plot 2,217 px | yes |

**Notes. None of them changes the result.**
- **Timer resolution.** The flush times read 0.00 / 1.00 / 2.00 ms. That pattern shows Firefox's `performance.now()` resolution of about 1 ms. Even allowing for one tick of error, p95 stays below 2 ms and the maximum stays below 3 ms, well under the 8 ms limit.
- **Refresh rate.** The interval median of 17.06 ms means a display slightly below 60 Hz (about 58.6 Hz). The 20 ms bound was chosen to absorb exactly this, so 99.83 % passes.
- **Throttling** is recorded as "not stated". With p95 at 17.08 ms, the run was not throttled in any way that would hide a failure. The 4× row is information only and stays pending; it does not decide the pass.
- **Confirmation run.** The Performance-panel check (D11: no long task, no dropped frames, no canvas redraw while scrubbing) is recorded as passed.

## Findings

None.

## Previous findings

- **r3:** no findings. Nothing in the code has changed since r3.
- **r1 and r2:** all findings were fixed and verified by r3. That verification still stands.
- **OQ-13** (the cursor stays when the mouse leaves) is not an F-05 condition. It belongs to F-07.

## Acceptance criteria to evidence

| AC | Automated (CI) | Manual (recorded in `MEASUREMENTS.md`) |
| --- | --- | --- |
| 1 Six charts aligned; no power chart without power; no elevation chart without altitude | `specs.test.ts` (goldens ride-12h, indoor-no-gps, no-distance-with-gps); alignment options in `uplot-options.test.ts`; chart count and chart-area height in `ChartStack.dom.test.ts` | D1 (earlier), D5, D13 |
| 2 Hover or touch-drag moves the cursor in every chart and updates the readout | `store.test.ts`, `gesture.test.ts`, `nearest.test.ts`, `specs.test.ts` (readout values); jsdom tests for pointer, tap and readout | D2, D3, D4, D9; P3, P7 (all values visible, readout pinned), P13 |
| 3 Heart-rate dropout is a break | `decimate.test.ts` (properties and hand cases) | D6 (earlier) |
| 4 Multi-hour pause is a break | `decimate.test.ts` (golden pause-hours) | D7 (earlier) |
| 5 Vertical swipe on a chart scrolls on a phone | `gesture.test.ts` (direction classification, cancel restores the cursor); jsdom scroll-lock wiring tests | P1 (re-run after the lock was added), P5, P11, P12, all on iPhone 13 mini with Orion |
| 6 Resize and rotation redraw at the new width | jsdom: resize order, re-placed cursor line, in-place swap | D8 (down to about 300 px), D14; P10 (rotation) |
| 7 12-hour scrub at 60 fps on desktop | The bench page logic, by review; Vitest asserts no timings, by design | D10 row above (PASS), D11 confirmation |

**Information only:** P2 (a drag continues onto the next chart) passes as well, although the user decided it is not required.

## Notes (not conditions)

- **Android not checked.** AC 5 asks for "a phone", and it passes on iOS WebKit, the engine where the scroll lock was actually needed. On Chrome, by the design's reasoning, `touch-action: pan-y` already fixes the axis once per touch. A Chrome-on-Android run would still be worth adding to F-07's phone gate.
- **F-06 and F-07 should watch for a sticky first scroll on charts** while MapLibre is starting, because the touchmove listener on the charts is non-passive (r3, "Passive-listener cost").
- **F-13 must not put an ancestor with non-visible `overflow`** around the stack, or the sticky readout stops working.
