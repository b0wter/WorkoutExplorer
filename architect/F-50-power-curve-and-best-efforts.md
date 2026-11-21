# F-50 Power curve and best efforts: architecture

- **Version:** 0.1
- **Status:** accepted (14 Nov 2026). Normal lane. OQ-189 to OQ-194 decided by the user on 13 Nov 2026 (decision log) and built in here; no open questions
- **Feature:** [`features/F-50-power-curve-and-best-efforts.md`](../features/F-50-power-curve-and-best-efforts.md)
- **Requirements:** AP-11, AP-5, AP-7, AP-8, AP-12, AP-19, UP-3, UP-6, SH-4, DS-1, DS-2, DS-4; NFR Performance, NFR Reliability
- **Builds on:** [F-49](F-49-zones.md) ZN-7, ZN-8 (public window, lazy section after the charts' first paint); [F-51](F-51-split-comparison-chart.md) SC-1 (plain SVG); [F-42](F-42-multisport-sections.md) MS-2, MS-6 (`lib/sections` legs); [F-18](F-18-laps-and-splits-table.md) L-3, L-7; [F-17](F-17-range-selection-and-axis-toggle.md) R-11; ADR-007, ADR-009, ADR-011, ADR-016, ADR-026, ADR-029, ADR-032. No server, format or template-data change
- **Owner:** frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| EF-1 | **Which samples.** The public window of ZN-7 without the range narrowing: `[0, N−1]` on the viewing page, the saved trim window on the edit page (the edit glue's existing window function, one home for both sections), recomputed after a save that changes it. Never follows the selected range (OQ-190). Hiding has no effect (SH-4 removes positions only). Requires the cumulative moving-time stream (ADR-029: the browser holds no moving-time rule); without it there is no section | OQ-190; edit page shows what the public build writes |
| EF-2 | **Legs.** The F-42 legs (`lib/sections`, MS-6 ranges, shared boundary sample) intersected with the window, each with its session's sport; when `sections()` gives none, the window is one leg with `meta.sport` (single sport, or a truncated file without sessions, UP-3). An effort lies inside one leg. **Distance efforts** only in legs whose sport is `running`, `walking` or `hiking` (not rowing or swimming, although DS-2 paces them). **Power efforts** in every leg with at least one power sample, except `transition` and `swimming` legs. Results over several eligible legs: the best per value; ties go to the earliest | AP-11, AP-12, OQ-191, OQ-192 |
| EF-3 | **Time is moving time.** Sample `i` covers the moving time `M[i] − M[i−1]` (as ZN-5); a step where `M` does not advance adds nothing, so pauses add no time. On 1 s recordings the time-weighted mean equals the sample mean | OQ-193; one convention with the zones |
| EF-4 | **Distance effort.** Distances 1,000 m; 1,609.344 m; 5,000 m; 10,000 m; 21,097.5 m; 42,195 m. Distance is `lib/range`'s carried distance (F-08 R-14 non-decreasing). Fastest = the smallest moving time over a stretch of exactly that distance. One end of the stretch sits on a sample, the other is interpolated linearly in distance between two samples, with moving time and elapsed time interpolated by the same fraction; with linear interpolation the optimum always has one end on a sample, so this is exact. Two pointers, O(n) per distance. Shown only when the leg's distance advance reaches it | OQ-193 "window ends interpolated"; hand-computable for the AC |
| EF-5 | **Power on a moving-second grid.** Per leg, power is resampled once onto whole seconds of moving time from the leg's start: second `k` holds the energy of `[k, k+1)` from EF-3's intervals, missing samples as 0 W (AP-7). Best `d`-second power = the highest mean of `d` consecutive grid seconds (prefix sums, O(L) per duration, L = the leg's whole moving seconds). Windows therefore start on whole moving seconds; on 1 s recordings this is exact. Table durations 5 s, 60 s, 300 s, 1,200 s, 3,600 s, each only when some leg with power has that much moving time | OQ-193; one O(N) resample, then sliding sums without allocation |
| EF-6 | **Power curve on a log grid of durations.** "Every duration" is drawn through a grid: every whole second up to 30 s, then each next duration at most 3 % longer than the last (rounded to whole seconds), plus the five table durations and the longest leg's L; about 280 durations for `ride-12h`. Per duration the best over all legs with power, as EF-5. 3 % is about one pixel on a log axis from 1 s to 12 h across a phone-wide plot, so the polyline equals the full curve on screen; the grid is an approximation: between grid points the curve can rise slightly, invisible at plot resolution. Cost on `ride-12h`: about 280 × 43,200 ≈ 12 M additions | Exact for every second would be O(L²) ≈ 1.9 × 10⁹; no sub-quadratic exact method exists |
| EF-7 | **Off the hot path, on the main thread.** Chunk import with the same trigger as ZN-8 (after the charts' first paint). Tables first (cheap), then the curve in time slices of at most about 4 ms, yielding to the browser between slices, so a scrub frame is never blocked for long (ADR-016). No Worker: ADR-007 keeps decoding on the main thread, and a Worker would need a copy of the streams and its own chunk for a sub-second job. The pure module is resumable per duration and holds no scheduling. Never recomputed on a cursor or selection flush; a selection flush only updates the row mark (O(rows)); a units change rewrites pace text only. Done = curve drawn; on `ride-12h` within 1 s of the section's insertion, measured with User Timing marks on the reference phone | AC timing, LCP and 60 fps unchanged; ADR-008, F-17 R-11 |
| EF-8 | **Table and tap.** One section "Best efforts", after the Zones slot: a distance table (effort name, moving time, pace in DS-2 units of the page's unit system, start), a power table (duration, average W, start), the curve under the power table; each part only when it has rows; no section when none has. Distance names stay as named in both unit systems (DS-1). Start = elapsed time on the charts' time axis. A row tap selects the effort through the selection store under F-18 L-3's rules (same range again does nothing, never clears, row marked while the store equals its range) and L-7's keep-the-row-in-place step; rows become tappable only once the range panel is mounted (QA-18-r1-3). The range is the **covering** sample range: `a` = the last sample at or before the effort's start, `b` = the first at or after its end, `b > a`. The panel's values may then exceed the row's by up to one sample step at each end; no note (as OQ-79). AP-19's range link follows from the store | OQ-189, OQ-194; the selection always contains the whole effort |
| EF-9 | **Curve as plain SVG** in the chunk, as F-51 SC-1: uPlot stays in `components/charts` (ADR-009 item 6) and its `rangeLog` is blanked at build (item 7), so a log axis in uPlot would grow the entry. Log x axis with ticks at the table durations the curve reaches, linear y from 0 W; line colour `--c-power`; geometry in a fixed `viewBox` scaled by CSS, no resize listener; no `style` attributes in markup (ADR-032). No hover, no tap (OQ-194). The SVG is an image to assistive technology with a label; the table is the accessible path | Zero entry bytes for drawing; theme by CSS (DS-4) |

**Module boundary**

| Module | Holds | May import |
| --- | --- | --- |
| `lib/efforts` (new, pure, tested in Node) | EF-2 eligibility, EF-3 to EF-6 | `lib/datafile` (types), `lib/sections`, `lib/range` (carried distance) |
| `src/efforts/` (new chunk, own CSS) | section, tables, SVG curve, EF-7 slicing, EF-8 taps and mark | `lib/efforts`, `lib/format`, `lib/selection` (types), `lib/units` |
| `src/page` (entry) | the slot, the `import()` and its condition | as today |
| `src/edit` (owner chunk) | passes the saved window, refreshes after a save | as today |

**Import condition** (entry, coarse), as built: a power stream, or `distanceAdvances` and a `meta.sport` or session sport that `isPaceSport` accepts (`page/start.ts`); no moving-time check. A pool swim or row may load the chunk and show nothing; accepted to save entry bytes. The chunk applies the exact rules; a false positive costs one chunk download, never a visible section.

## Cost

| What | Decision |
| --- | --- |
| Static (entry) | Ceiling **150 bytes gzip**: the slot, the `import()` call site and condition, preload-map entries for the chunk and its CSS. Measure the entry before and after and record it in ADR-011 item 4; above 150: stop and report. Baseline 57,030, reserve 2,970, 2,820 after this ceiling |
| Lazy | `src/efforts/` and `lib/efforts`: report gzip JS and CSS. Not counted |
| Memory | Per compute one grid of L doubles per leg with power and its prefix sums (≈ 0.7 MB for `ride-12h`), released after the curve is drawn; nothing per flush |
| Data file, HTML, server | None |

## Verification

Synthetic files only (ADR-004), generated by `corpusgen`.

| What | Check |
| --- | --- |
| Hand-computed | The 12 km run of AC 1 with irregular sampling and a pause inside the fastest kilometre: times, paces and starts equal values computed by hand from EF-4 |
| Mutants | Each must fail a test: elapsed instead of moving time; missing power skipped instead of 0 W; an effort crossing a leg or entering a transition; distance efforts in a ride or rowing leg; a value shown that the leg cannot cover; window ends snapped to samples (EF-4); the selection or untrimmed edit window used (EF-1); the inward instead of the covering range (EF-8); a ties-latest rule |
| Edit equals public | For a trimmed corpus file: the edit page's efforts and curve equal those of the published data file |

## Hand-offs

| To | What |
| --- | --- |
| Frontend | Wording (section heading, column headers, effort names, curve labels) is reviewed by the user on screen. The template slot is mirrored in `components/page` as the Zones slot is (hygiene test) |
