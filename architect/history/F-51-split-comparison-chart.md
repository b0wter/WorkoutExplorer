# F-51 Lap and split comparison chart: history

## 0.1 to 0.2 (14 Nov 2026, QA-51-r1-1)

SC-5 scale rule, superseded by SC-11: "symmetric around the reference, its half-range the larger of the furthest drawn bar end and the band edge, so the band is always visible." One very slow lap flattened every other bar and, in pace, pushed the fast edge past infinite speed (empty top label).

## 0.2 to 0.3 (14 Nov 2026)

SC-11 gap rule, superseded by the 75th-percentile cap: "sort the deviations ascending; from the second on, the first deviation more than 3× the larger of the one before it and the band edge starts the outliers, which are it and every larger one; the others are everything below. Cap = 3× the largest of the others (at least 3× the band edge)." Found wrong by the frontend engineer: 6 laps at 4.5, 5 at 2.5 and 2 rest laps at 0.3 m/s put the median inside the recovery group (deviations ~0), so the work laps started the outliers and 8 of 13 bars were clipped.

SC-11 percentile cap, superseded by F-71 (25 Nov 2026): "cap = 3x the 75th-percentile deviation of the drawn bars (nearest rank), at least 3x the band edge; half-range = furthest bar, at most cap and edge limit, at least band edge", symmetric around the reference. It clipped the recovery laps of interval runs (QA-51-r2-1). Replaced by an asymmetric axis with edge limits only.
