# F-74 Speed spike filter and robust chart axis: architecture

- **Version:** 0.1
- **Status:** accepted (25 Nov 2026). Normal lane. OQ-352 to OQ-358 and the axis bottom decided (feature file). F74-Q1 decided (user, 25 Nov 2026), ADR-011 size review done (rev. 17); F74-Q2 and F74-Q3 do not block (below)
- **Feature:** [`features/F-74-speed-spike-filter.md`](../features/F-74-speed-spike-filter.md)
- **Requirements:** UP-12, AP-26; AP-2, AP-24; AP-4, AP-6, AP-7, AP-9, AP-12, UP-6, SH-8, DS-6; NFR Performance
- **Builds on:** [F-02](F-02-fit-parsing-and-normalisation.md) R-3, R-7, R-10, R-11; [F-08](F-08-fit-validation-and-recovery.md) R-13; [F-47](F-47-trim.md) TR-5; [F-30](F-30-regenerate-command.md); [F-05](F-05-stacked-charts-with-shared-cursor.md); [F-67](F-67-multisport-leg-picker.md) LP-4; [F-58](F-58-summary-explained.md) SE-1; ADR-009, ADR-011, ADR-029
- **Owner:** both, backend first. No data file format, schema or setting change

## Server: the filter (UP-12)

| # | Decision | Why |
| --- | --- | --- |
| SP-1 | **Place.** A new step of `workout.Normalise` in its own file, after R-13 (transitions masked) and R-4 (presence settled), before R-6, R-7, R-11 and R-10. `ingest` and `convert.go` stay unchanged: they map device fields, the filter is a rule over the time axis | Needs the sport timeline, the legs, positions and the gap-fill marks, which exist only in `workout`. One place, so upload, owner reads, trim, `draft.bin` and `regenerate` all get it |
| SP-2 | **Gap filling (R-3) first; detection on recorded samples only** (`inserted` false). A run's duration is its last recorded offset − first + 1 s. A removal also makes missing every sample R-3 inserted between a removed sample and the next recorded sample on either side | R-3 interpolated those samples from the spike. On the 1-s grid one smart-recorded spike sample would stretch to up to 7 s and escape the 5-s limit |
| SP-3 | **Eligible samples.** Leg = the session that covers the sample, or all samples no session covers as one leg. Eligible when the leg is not a transition (speed already dropped), not `lap_swimming`, and has at least one valid position anywhere in it. Presence is per leg, never per sample. The speed's source (GPS, wheel sensor, footpod) does not matter | OQ-353. The tunnel case has no positions around the spike itself |
| SP-4 | **Detection (Hampel-style, upward only).** Level before *L*b and after *L*a: median of the nearest present recorded samples of the same leg on each side (a count and a maximum reach in seconds). Missing samples and gaps are skipped, not a barrier. A *run* is a maximal sequence of present recorded samples above the level band of *L*b (relative and absolute part). It is removed only if all four hold: (1) duration ≤ 5 s; (2) **every** sample of the run is clearly above, at least *L*b + the spike margin (relative and absolute part); (3) \|*L*a − *L*b\| is within the band (speed returns); (4) both sides have their minimum count. Otherwise every sample stays. Band, spike margin (band < margin), level count and reach are named constants the engineer tunes against the synthetic acceptance cases. The 5 s, upward only and "returns" are UP-12, not tuning | (2) separates a glitch from a sprint: a real acceleration ramps through the band, a catch-up jumps in one step. (3) keeps a step. The absolute parts keep GPS jitter at a standstill (*L*b ≈ 0). O(n), within ADR-017's normalisation bound |
| SP-5 | **Moving time unchanged by construction.** `Samples` keeps the speed before removal for every sample SP-2 made missing. It is unexported, never encoded and nil when nothing was removed, and sparse (index → value) so that ADR-017's memory model is unchanged. `movingTime` reads that value for those samples, and `Trim` carries it over as it does `inserted`. So the moving-time stream M, moving time, range average speed (AP-7: distance ÷ ΔM) and lap speeds (F-18 L-1) are byte-identical | OQ-355. Rejected: running the filter after R-7, because `Trim` recomputes moving time from the trimmed samples (TR-5) and would count removed samples as stopped. Also rejected: "removed counts as moving", which is exact only through a coupling of SP-4's thresholds with R-7's and with R-3's interpolation |
| SP-6 | **Max speed.** After the filter and before R-10, a session with at least one removed sample in its `SampleRange` gets `Totals.MaxSpeed` = the highest remaining present speed in that range (nil if none). R-10 then combines as today (one leg as recorded, several the maximum). The record leg and R-11 already read the filtered samples. Sessions without removals and all laps keep the device values. After a trim, TR-5 recomputes from the filtered samples as today | OQ-354, AP-2. The session totals in the data file then agree with the summary |
| SP-7 | **Data file.** No format change. A removed sample is the speed column's missing value, and the M stream is unchanged (SP-5). Nothing stored, no flag, no count, no setting (OQ-356) | Feature "What exists" |
| SP-8 | **Existing workouts** get the filter at the next `regenerate` (F-30), with no migration and no compatibility. The release notes list "run `regenerate`" as an upgrade step | Workouts are short-lived |

## Browser: the robust axis (AP-26) and the dialog line (AP-24)

| # | Decision | Why |
| --- | --- | --- |
| AX-1 | **Scope:** every `MetricChart` (the AP-4 stack, F-68 dynamics charts included) on the viewing and edit page, whole and leg view. Not affected: F-51/F-71 bars, the F-50 power curve (own SVG scales), route colouring (AP-9 reads the stream, where a removed sample is already missing) | Feature scope |
| AX-2 | **The rule, in an offset-free form.** It works on the present values the chart draws: after `toDisplay` and the existing pace clamp, over the view's samples a..b. *p*lo and *p*hi are the 0.5th and 99.5th percentiles, with the index rule of the pace clamp (`floor((n−1)·q)`) and one sort shared with it. Reference *r* = 0 for **zero-based** metrics (speed, not pace; power; cadence), else *p*lo. **Top:** if max − *r* > 1.5·(*p*hi − *r*), top = *p*hi + 0.15·(*p*hi − *r*). **Bottom**, only for metrics that are not zero-based: if *p*hi − min > 1.5·(*p*hi − *p*lo), bottom = *p*lo − 0.15·(*p*hi − *p*lo). Otherwise the axis keeps the data extreme. With no spread (*p*hi = *r*) the axis is today's. Zero-based is a flag on `ChartSpec`; elevation, heart rate, temperature, pace and the running dynamics are not zero-based | For zero-based metrics this is AP-26 word for word. Elsewhere "1.5 × p99.5" depends on where the unit puts 0 (°C vs °F, elevation below sea level) and would not clip the AC's 200 bpm in a 120–150 series (1.5 × 150 = 225). Pace needs no special case: a fast outlier is the low end, clipped by the bottom rule and drawn at the top edge by `dir: -1`; the slow end is already clamped. With n ≤ 200 the percentiles are the extremes, so nothing is clipped |
| AX-3 | **Where.** The pure rule sits in `lib/charts` next to `chartValues`. It is computed once in `MetricChart` where `values` is computed (one instance per series, data, view and units; ChartStack keys on them). The y scale's range function (only `components/charts`, ADR-009) returns uPlot's own `rangeNum(lo, hi, 0.1, true)`, the call its default y range makes. lo and hi are uPlot's data extremes, replaced by AX-2's bound only on a clipped end. The empty-leg fixed range stays. No value is changed: uPlot clips the series path to the plot area, so an outlier is drawn at the edge, and readout and range panel read the raw arrays | Without an outlier the axis is exactly today's (padding, 1-2-5 rounding, soft 0). No marker (OQ-358) |
| AX-4 | **Cost per redraw: none.** The range function returns cached numbers. uPlot calls it only on `setData` and `setSize`, and the cursor (ADR-008, ADR-009) never redraws a canvas. One sort per chart mount on the view's samples. If the ChartStack mount on ride-12h grows by more than 10 ms on the desktop bench, use selection instead of a sort | 60 fps path (ADR-016) untouched |
| AX-5 | **Bundle: static, in the chart code.** The stacked charts and uPlot are in the viewing entry (ADR-011 B0), so AP-26's premise "the chart code loads lazily" does not hold. A lazy chunk would cost its own `import()` call site and preload entry, about as many entry bytes as the rule (F-68's measurement), plus a request awaited before every chart's first paint. **Target 100, ceiling 150 bytes gzip**, see F74-Q1 | ADR-011 item 1: static is what every viewer needs before the map |
| AX-6 | **AP-24 line** in the lazy summary-dialog chunk and its `help` text group (ADR-011 R-2), in en, de, es and fr. Shown when the summary has a speed or pace value. The dialog never gets streams, positions or a count (F-58 SE-1) | Entry 0. "Only for what the workout has" (AP-24) at the granularity the dialog has |

## Cost

| What | Decision |
| --- | --- |
| Entry (ADR-011) | AX-2 and AX-3 only: **ceiling 150 gzip** (F74-Q1). The engineer measures before and after and books the figure in ADR-011 item 4. Above 150: stop and report |
| Lazy | Summary-dialog chunk and `help` files: one line. Report the sizes |
| Owner chunk | 0 (the edit page gets everything through `startPage`) |
| Server | One pass over the speed stream per normalisation; sparse SP-5 record. ADR-017 unchanged |

Notes (QA-74-r1-2, -5): a spike in the first or last two samples of a leg is kept (a side needs 2 samples) and its value may become max speed. The p99.5 uses a ceil index (needed for the AC's at most 200 samples); pace sorts twice; zero-based metrics are detected by name. The absolute spike margin is 5 m/s (QA-74-r1-1): a roll between stops (about 2.5 m/s) stays, an 85 km/h glitch from a stop is removed; a glitch below about 5 m/s from a stop is kept.

## Verification (what must be shown)

- Every feature AC on synthetic FIT files and series. In addition: a smart-recorded spike (one recorded sample, 3 s and 4 s spacing) is removed together with its interpolated neighbours. A spike right after a stretch without speed (tunnel) is removed. Jitter at a standstill and a start from a stop are kept. A trimmed workout whose window holds a removed spike has the same moving time as the trimmed workout without the spike.
- Moving time, M, distance, laps and every other stream are byte-identical with and without a spike. Corpus: synthetic goldens change only for files that hold a spike, each change listed. The operator reruns the local real-file corpus (ADR-004) and reports only how many files changed.
- AX-2 in Node across the AC series, °C and °F with the same decision, pace with a fast glitch, n ≤ 200. Today's y range is unchanged without an outlier, and the entry is measured.

## Open questions

| ID | Question | Blocks | Owner |
| --- | --- | --- | --- |
| F74-Q1 | **Decided (user, 25 Nov 2026): accepted.** Up to 150 bytes gzip from the reserve (262); size review done, ADR-011 rev. 17 decision 7. Requirements reword the AC "entry bundle size unchanged" | Nothing | requirements |
| F74-Q2 | AP-26's "1.5 × p99.5 / + 15 %" read relative to 0 for speed, power and cadence and relative to the 0.5th percentile for every other metric (AX-2). For heart rate the AC's top is then 154.5, not 172.5 bpm, while the clipping itself is as the AC expects. Suggestion: accept; requirements reword AP-26 and the AC | Nothing (suggestion built) | user, requirements |
| F74-Q3 | AP-24 line shown whenever the summary has a speed or pace, also for indoor workouts, because the dialog cannot see positions (AX-6). Suggestion: accept | Nothing (suggestion built) | user |

## Hand-offs

| To | What |
| --- | --- |
| Requirements | AP-26 wording and the F-74 axis ACs per F74-Q2; the AC "entry unchanged" per F74-Q1 |
| Main session | Release notes: "run `regenerate`" as an upgrade step (SP-8) |
