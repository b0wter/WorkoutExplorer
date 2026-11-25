# F-74 Speed spike filter and robust chart axis

- **Status:** planned (25 Nov 2026); open questions OQ-352 to OQ-358 decided 25 Nov 2026 (user)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first: filter, summary; frontend: chart axis, AP-24 line)
- **Lane:** normal (changes the published speed stream and max speed; not privacy)
- **Requirements:** UP-12, AP-26; AP-2 (max speed), AP-24 (one line), AP-4, AP-6, AP-7, AP-9, AP-11, AP-12, AP-23, UP-6, DS-6, F-30 (regenerate); NFR Performance (entry bundle, 60 fps)
- **Depends on:** F-02, F-05, F-08, F-30, F-42, F-47, F-54, F-58, F-67, F-68

## Goal
A one-second GPS glitch (a ride at ~26 km/h with one sample at ~85 km/h) no longer sets the speed chart's scale or the max speed, and a single freak reading in any chart no longer squashes the rest of the line.

## What exists
- Speed per sample comes only from the device's `record` field (`enhanced_speed`, else `speed`; `ingest/convert.go`); the server never derives it from distance. Without a speed stream, moving time uses distance over time (`workout/moving.go`).
- Summary max speed is the device's session `max_speed` where recorded (`workout/summary.go`), else the sample maximum (`workout/stats.go`); so today an 85 km/h spike shows as max speed.
- Moving time from speed counts a sample with missing speed as stopped (`moving.go`); the range panel's speed is distance ÷ moving time from the server's cumulative moving time (AP-7).
- The data file already has a missing-sample value per stream; no format change is needed.
- Laps use the device's lap averages; splits, best efforts (AP-11) and the comparison chart (AP-23, F-71) use distance and time, not speed samples.
- Chart axes reach each series' maximum (and minimum where not anchored at 0).
- `regenerate` (F-30) rebuilds every live workout from its original; workouts are short-lived, no compatibility needed.

## Scope
- **Filter (UP-12), server, during normalisation:** a run of 1 to 5 s of speed samples clearly above the samples on both sides, after which speed returns to the level before, is removed (missing, not interpolated). Median/Hampel-style comparison with neighbours; thresholds are the architect's and engineer's choice. Only upward spikes; dips stay.
- **Where:** speed in legs with position data. Never pool swims, transitions (speed already dropped, AP-4) or legs without positions (indoor wheel-sensor or footpod speed).
- **Max speed:** a leg where samples were removed takes its max speed from the highest remaining sample instead of the device's `max_speed` (AP-2); legs without removals keep the device value. Multi-leg max as today (maximum over legs). After a trim (UP-6) as today.
- **Unchanged:** moving time (a removed sample counts as moving or stopped as it would have before removal), total distance and the distance stream (the catch-up stays, accepted), average speed, laps, splits, best efforts, comparison chart, heart rate, elevation, power, cadence, temperature, the stored original.
- **Follows the speed stream:** charts (gap in speed or pace), cursor readout (missing-value dash), route colouring by speed (AP-9; removed samples count as missing).
- **Robust axis (AP-26), front end, every line chart:** viewing and edit page, whole-workout and leg views. Top: the true maximum, unless max > 1.5 × p99.5 of the samples shown; then top = p99.5 + 15 %, and samples above are drawn clipped at the top edge. Bottom of an axis not anchored at 0: the mirrored rule (p0.5, 15 % below). Pace charts apply the rule to the pace values as drawn, so a fast outlier is clipped at the fast end. No sample removed or changed, no marker; cursor readout and range panel show true values. Bar charts keep their own rule (AP-23, F-71). Computed once per series and view, not per frame. Chart code stays lazy: nothing added to the viewing page's entry bundle (ADR-011).
- **AP-24 dialog:** one general line that short GPS speed spikes are removed and max speed ignores them; no count; in English, German, Spanish and French (DS-6).
- **Existing workouts:** get the filter at the next `regenerate` (F-30).
- **No setting:** always on, nothing for the sharer to choose, nothing new stored.

## Out of scope
- Correcting the distance catch-up (a short best effort or range across the tunnel can still be too fast; accepted); dips in speed; GPS position glitches on the map; removing heart-rate, power or cadence spikes (the axis rule only scales around them); any marker or hover text.

## Acceptance criteria
Tests use synthetic FIT files and synthetic series only (real files are never committed).
- [ ] Synthetic ride at a steady 26 km/h with one 1-s sample at 85 km/h: that sample is missing in the published speed stream; the samples before and after are unchanged; max speed in the summary is 26 km/h, also when the file's session `max_speed` is 85 km/h.
- [ ] Same ride with a 5-s spike: removed. With a 6-s glitch of the same height: kept, every sample unchanged.
- [ ] A real 10-s sprint (26 → 45 → 26 km/h over 10 s) is kept, every sample unchanged; max speed 45 km/h.
- [ ] A step that does not return (26 km/h, then 40 km/h for a minute): kept. A short dip (26 → 10 → 26 km/h for 1 s): kept.
- [ ] A ride with no spike: the published speed stream and summary are identical to the build before F-74.
- [ ] Moving time, total distance, the distance stream, average speed, laps, splits, best efforts and every other stream are identical with and without the spike; a leg without a removed spike keeps the device's `max_speed`.
- [ ] Running file with a 1-s spike: removed; best pace ignores it.
- [ ] Pool swim, an indoor ride without positions, and a triathlon's transitions: speed unchanged. A triathlon's bike leg with a spike: removed.
- [ ] Axis, normal series (3,600 samples, 20 to 30 km/h, max 30): the axis top reaches 30 km/h, as before F-74.
- [ ] Axis, one far outlier (the same series plus one sample at 200 bpm in a heart-rate series of 120 to 150 bpm, or one 85 km/h speed sample in a file the filter did not touch, such as a 6-s glitch): the axis top is p99.5 + 15 % (before any tick rounding the chart already does); the outlier is drawn at the top edge; the cursor readout at it shows its true value.
- [ ] Axis, outliers above 0.5 % of the samples (e.g. a 10-min climb at higher power): no clipping, axis reaches the maximum.
- [ ] Axis bottom: an elevation series with one sample 500 m below the rest gets a clipped bottom by the mirrored rule; an axis anchored at 0 is unchanged.
- [ ] Pace chart of a run with a 6-s fast glitch (kept by the filter): the fast end of the axis is clipped by the rule, the glitch drawn at that edge.
- [ ] Leg view (F-67) and edit page: the rule uses the samples of that view.
- [ ] AP-24 dialog shows the spike line in all four languages.
- [ ] `regenerate` on a workout uploaded before F-74 with a spike: the rebuilt page shows the gap and the corrected max speed.
- [ ] Viewing page entry bundle size unchanged; LCP and 60 fps scrubbing on ride-12h unchanged.

## Decisions (user, 25 Nov 2026)
- [x] **OQ-352 Dips.** Only upward spikes are removed; dips stay.
- [x] **OQ-353 Which speed.** Only legs with position data; sensor-only legs untouched.
- [x] **OQ-354 Max speed source.** Highest remaining sample in a leg where a spike was removed; device `max_speed` elsewhere.
- [x] **OQ-355 Moving time.** Unchanged by the filter.
- [x] **OQ-356 Owner switch.** None; always on.
- [x] **OQ-357 Distance catch-up.** Accepted; stays in the distance stream.
- [x] **OQ-358 Explanation.** No marker and no hover text (the marker is replaced by the robust axis, AP-26); one general line in the AP-24 dialog, in all four languages.
- [x] **Axis bottom** (requirements engineer, delegated by the user): the rule is mirrored for axes not anchored at 0, so a single low glitch (barometric elevation, slow end of a pace axis) is clipped the same way.
