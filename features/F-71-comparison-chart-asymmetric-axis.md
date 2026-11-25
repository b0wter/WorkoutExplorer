# F-71 Comparison chart: non-symmetric value axis

- **Status:** planned (25 Nov 2026); OQ-346, OQ-347 assumed
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (rule in `lib/compare`, drawing in `src/splits`; no server or data file change)
- **Lane:** small (one scale rule changes; the engineer updates F-51 architecture SC-11)
- **Requirements:** AP-23; AP-8, AP-13, AP-20, DS-1, DS-2; NFR Performance, NFR Reliability
- **Depends on:** F-51 (comparison chart); F-64, F-69 (more tables and rows use the same chart)

## Goal
A viewer sees every row's bar whole, including the slow recovery laps of an interval session, instead of a bar cut off at the plot edge.

## What exists
- F-51 draws each bar from the median line (AP-23), faster up, slower down, on an axis symmetric around the median. Its half-range is capped at 3× the 75th-percentile deviation and, in pace sports, at half the reference pace; bars beyond are drawn to the edge with a break mark (architecture SC-11; decision log 14 Nov 2026, QA-51-r2-1). In practice this clips recovery laps.

## Scope
- **Axis not centred.** One linear scale in the shown quantity (speed, or pace inverted, DS-2). The space above the median line is sized to the fastest drawn bar, the space below to the slowest, so the median line sits wherever that puts it within the chart's fixed height (about 160 px, unchanged). Each side is at least the ±3 % band edge, so the band and the line always show.
- **Percentile cap dropped** (OQ-346): the 3× 75th-percentile cap no longer applies; outlier rows such as recovery laps are drawn whole, even when that makes the other bars short.
- **Edge limits stay as a safety net** (OQ-347): the fast edge is at most twice the reference speed (pace: half the reference pace), in every sport; in pace sports the slow edge is at most three times the reference pace (a third of the reference speed); in speed sports the slow edge never goes below 0. A bar beyond a limit is drawn clipped with the existing break mark and accessible label (SC-11).
- **Unchanged:** median, best, header, colours, band, partial last split (drawn, not counted), overlay and its own right-hand scale, bar taps, the 2 to 200 bar range, every table value. The two axis labels show the two edges as today.

## Out of scope
- Log or broken axes; a fixed or viewer-chosen scale; a taller chart.

## Acceptance criteria
- [ ] An interval run with 6 × 1 km between 3:40 and 4:10 /km and 5 recovery laps at 6:30 /km: every recovery bar is drawn whole, no break mark; the fastest bar reaches the top edge and the slowest the bottom edge; the median line sits in the upper part of the chart, not at mid-height.
- [ ] A steady ride whose laps are all within ±3 %: band and median line visible, no bar clipped.
- [ ] A run with one lap at under a third of the reference speed (for example a walk at 15:00 /km against 4:30 /km): that bar is clipped with the break mark at the bottom edge and named in the accessible label; the others are scaled as if it ended at the limit.
- [ ] Rows with no bar slower than the band edge: the space below the median line is the band edge only, and the band still shows.
- [ ] Pace sports keep faster up; switching units redraws the axis labels in the new units (DS-1, DS-2).
- [ ] `lib/compare` tests cover the new scale (both sides, both limits, band minimum); every corpus file renders its chart; entry bundle unchanged; 60 fps scrubbing on `ride-12h` unchanged.

## Open questions
- [ ] **OQ-346 Drop the percentile cap?** Assumed: yes; only the edge limits clip. Affects whether one extreme lap may shrink the other bars (the reason for the cap, QA-51-r1-1).
- [ ] **OQ-347 Edge limits.** Assumed: fast edge at most 2× the reference speed in every sport (today pace sports only); slow edge in pace sports at most 3× the reference pace (a standing or walking lap would otherwise squash all others to a few pixels). Affects which rows are still clipped.
