# QA F-74 Speed spike filter and robust chart axis - round 1

- **Date:** 2026-11-25
- **Verdict:** ready with conditions. No Blocker or Major finding. Conditions: the regenerate, leg-view and performance ACs have no automated test and were not checked by hand in this round (see Coverage).
- **Inputs:** `features/F-74-speed-spike-filter.md`; `architect/F-74-speed-spike-filter.md` (SP-1 to SP-8, AX-1 to AX-6); ADR-011 rev. 17; spec UP-12, AP-2, AP-24, AP-26. Worktree fast-forwarded to main 6a24062; diff reviewed c820399..6a24062.

## Build and test results

- `make ci`: exit 0. Go: all packages `ok` (`internal/workout` 27.3 s). Vitest: 98 files, 2,557 tests passed. Deploy tests OK.
- Size: viewing entry `index.html` 61,886 / 62,000 gzip PASS. ADR-011 records 61,738 before F-74, so this build measures **+148 B** (the engineer reported +143). Ceiling 150.
- Probes (Go test overlay in the scratchpad, `src/` unchanged): roll at a stop, spikes at the edge of a leg, two spikes close together, trim ending on a spike.

## Findings

### Minor

- **ID:** QA-74-r1-1
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/workout/spikes.go:15-21` (`spikeMarginAbs = 2.0`)
- **Relates to:** UP-12 ("clearly faster"), SP-4 (absolute parts keep standstill jitter)
- **Evidence:** probe: speed 0 for 60 s, then 2 s at 2.5 m/s (9 km/h), then 0 again. Samples 100 and 101 are removed. The standstill-jitter test only covers 1.5 m/s.
- **Consequence:** a real short roll between two stops (for example at a traffic light) loses its speed samples. The effect is small: moving time stays correct (SP-5) and max speed is not affected. Raising the absolute margin, or requiring a non-zero level before, would avoid it.

- **ID:** QA-74-r1-2
- **Severity / kind:** Minor / risk (accepted deviation, judged)
- **Route:** backend
- **Where:** `spikes.go` `level`, `spikeMinCount = 2`
- **Relates to:** UP-12, AP-2
- **Evidence:** probe: an 85 km/h spike at s = 1 or s = 298 of a 300-s leg is kept, so max speed stays 85. At s = 2 it is removed.
- **Consequence:** a GPS spike in the first or last two samples of a leg, which is common while the fix settles, still sets max speed. Acceptable for now. The deviation should be noted in the F-74 design doc.

- **ID:** QA-74-r1-3
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/components/charts/MetricChart.svelte:96-98`; `lib/charts/specs.ts` `axisClip`; `lib/charts/decimate.ts:78`
- **Relates to:** AX-3 ("without an outlier the axis is exactly today's")
- **Evidence:** read the uPlot source: the default y range is `snapNumY` = `rangeNum(dataMin, dataMax, 0.1, true)`, the same call as the new range function. On the time axis, `decimate` keeps the minimum and maximum of each column, so the unclipped range is identical. On the distance axis, `decimate` drops samples with no x (`xi !== xi`), but `axisClip` still counts them. Not reproduced in a mounted chart.
- **Consequence:** on the distance axis, a sample without distance can set the y range (and the percentiles) without being drawn, which differs from the axis before F-74. Rare.

- **ID:** QA-74-r1-4
- **Severity / kind:** Minor / defect (documentation)
- **Route:** frontend
- **Where:** `architect/ADR-011-*.md` item 4, F-74 row ("open | open")
- **Relates to:** F74-Q1, ADR-011 decision 7, the "Cost" table of the F-74 design
- **Evidence:** the ledger is not booked. Measured +148 B (61,886), 2 B under the ceiling; 114 B of reserve left.
- **Consequence:** the next size review starts from a wrong baseline. Book 61,886 (or a re-measured figure) and the lazy chunk sizes.

- **ID:** QA-74-r1-5
- **Severity / kind:** Minor / deviation not written down
- **Route:** architect
- **Where:** `lib/charts/specs.ts` `axisClip`
- **Relates to:** AX-2
- **Evidence:**
  - p99.5 uses `ceil` where AX-2 says `floor((n−1)·q)`. This is needed for the AC "≤ 200 samples: nothing clipped" (with floor, n = 200 clips), so AX-2 itself is inconsistent with the AC.
  - The sort is not shared with the pace clamp: there are two sorts on pace charts.
  - Zero-based is detected by matching the metric name against the string `'speed power cadence'`, not a `ChartSpec` flag as AX-2 says. This is correct for the current closed `MetricName` set.
- **Consequence:** the design doc no longer describes the code. Update AX-2 to `ceil` and to name-based detection, or ask for the flag.

- **ID:** QA-74-r1-6
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `MetricChart.svelte` (range wiring)
- **Relates to:** AC "Leg view (F-67) and edit page: the rule uses the samples of that view"; AC "Axis, normal series: as before"
- **Evidence:** the tests cover only the pure `axisClip`. No DOM test checks that the y scale's range function returns the clipped bounds, or that a leg view uses the leg's samples. The code computes `values` from `subarray(a, b+1)`, and ChartStack keys on `view?.a`, so it remounts on a leg switch: correct by reading. If line 98 were deleted, every test would still pass.
- **Consequence:** a regression in the wiring would go unnoticed.

## Deviations reported by the engineers, judged

- Spike at the edge of a leg kept: accepted (QA-74-r1-2).
- Ramped spikes (26, 40, 85, 40, 26) kept: consistent with SP-4 (2), which requires every sample of the run to be clearly above. Accepted.
- No regenerate test: regenerate re-parses the original through `Parser.Accept`, which calls `Normalise`, so it gets the filter. Traced only; see Coverage.
- Corpus goldens unchanged: consistent with "goldens change only for files that hold a spike". `make ci` corpus tests pass.
- Range always set through `rangeNum(..., 0.1, true)`: equal to uPlot's default on the time axis (QA-74-r1-3 covers the distance-axis exception).
- p99.5 with `ceil`: needed for the AC (QA-74-r1-5).
- Name-based detection of zero-based metrics: QA-74-r1-5.
- Mount benchmark not run: open condition (AX-4, 10 ms).
- de/es/fr machine translations: the texts read correctly. Preference only.

## Checked and correct

- Filter placement (SP-1). Recorded samples only, with the interpolated neighbours dropped (SP-2; smart-recording test).
- Eligible legs (SP-3): transition and lap swim skipped, positions per leg.
- Moving time from `unfiltered` in `movingTime`, carried over by `Trim`. These are the only two `movingTime` callers. The trim test with a spike inside a stop gives the same moving time.
- SP-6: session max speed recomputed only for sessions with a removal. A trim ending on a spike keeps the removal; max 26.
- `unfiltered` is unexported and never encoded; `TestTransitionAllowListComplete` was updated.
- Two spikes 2 s and 3 s apart: both removed.
- AP-24 line in the lazy summary-info chunk, keyed on speed; keys present in en, de, es and fr.
- AX-2 values for 34.5 km/h, 154.5 bpm, °C/°F, elevation bottom 85 m, power above 0.5 %, n ≤ 200, and pace fast end: tested in `specs.test.ts`.

## Previous findings

None (round 1).

## Questions

None. The decisions F74-Q1 to F74-Q3 are recorded.

## Coverage

- Traced to code and a passing test: the AC cases for 1-s, 5-s and 6-s spikes, sprint, step, dip, no spike, unchanged streams and laps, running, pool/indoor/transition/triathlon, every axis case, and the AP-24 line (en test; key parity covered by the i18n tests).
- Traced to code only:
  - regenerate (via `Parser.Accept` → `Normalise`);
  - leg view and edit page uses the view's samples;
  - the time-axis y range equals the pre-F-74 default (uPlot source).
- Not run:
  - LCP and 60 fps scrubbing on ride-12h;
  - the AX-4 mount benchmark;
  - browser hand checks of clipped drawing and of the readout showing the true value at the edge;
  - the operator's local real-file corpus count (SP verification, operator task).
