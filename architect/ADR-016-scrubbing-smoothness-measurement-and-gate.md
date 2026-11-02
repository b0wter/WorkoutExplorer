# ADR-016 Scrubbing smoothness: how 60 fps is measured, when a run counts, and when the phase 0 gate passes

- **Status:** accepted. (15 Oct 2026) Decision 5 was accepted by the user as proposed (OQ-27): "it's a midrange phone that should be representative for a lot of devices". A passing gate run is now a final Gate 0 verdict.
- **Date:** 15 Oct 2026
- **Features:** F-07 (introduces it; gate 0); F-17, F-25 and F-26 (re-measure with it when they touch the cursor path); F-32 (launch check)
- **Supersedes:** none. It collects the pass rule that F-05 (design, "Performance") and F-06 (design, "Dev page") stated for their bench pages. Those pages keep their auto-scrub as the desktop regression check
- **Depends on:** [ADR-008](ADR-008-shared-cursor-store.md) rev. 1 (listeners run in subscription order, once per frame), [ADR-011](ADR-011-activity-page-bundle-allocation.md) rev. 7 (the recorder is lazy), [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md) rev. 4
- **Requirements:** NFR Performance ("Cursor scrubbing stays at 60 frames per second on a 12-hour workout"); Roadmap gate 0; F-07 ACs 2, 3 and 6; spec open question "Metric definitions" (reference phone and test conditions)

## Context

F-07 AC 2 asks for a touch-drag of at least 10 s on the operator's phone, a "trace" showing 60 fps, and the operator's judgement that it is smooth. So far:
- F-05 and F-06 measured with an auto-scrub: synthetic mouse `pointermove` events on dev-server bench pages. The phone reached 99.16 % of frame intervals ≤ 20 ms with the map, and the desktop 99.67 %.
- An auto-scrub skips the real touch path: touch events, the chart's non-passive `touchmove` lock (ADR-010 rev. 2) and the route scrub's capture listeners (ADR-015).

No profiler is available for the phone in the operator's setup. The dev machine has no browser, and Web Inspector for iOS needs Safari on a Mac. iOS Low Power Mode caps `requestAnimationFrame` at 30 Hz, which made one F-06 run meaningless.

## Decision

1. **The trace is an in-page scrub record** (F-07 design, F-4), taken on the production build served with brotli:
   - a `requestAnimationFrame` loop that stores every frame interval for 10 s in a preallocated buffer;
   - three cursor-store listeners. **A** is subscribed before the charts, **M** after the charts and the readout, and **Z** after the map marker. Each flush therefore splits into charts (M − A) and map (Z − M), with the total Z − A;
   - per frame, whether the cursor changed and whether a finger was down.

   The record makes no DOM write and no layout read during the window. It is loaded with `import()` only when the page URL has `?measure`.
2. **Pass rule** (unchanged from F-05 and F-06): at least 99 % of frame intervals ≤ 20 ms, **and** total flush p95 ≤ 2 ms, **and** total flush max ≤ 8 ms. The 20 ms bound absorbs timer jitter at 60 Hz; on faster displays it still means at least 60 fps.
3. **A run counts only if** all of these hold:
   - the median frame interval is below 25 ms (no 30 Hz cap);
   - in a touch run, a finger was down in every frame of the window, including the frames before the first touch. A finger that lands late disqualifies the run, because its idle frames would raise the pass share (F-07 QA-07-r1-1);
   - the cursor changed in at least 75 % of frames, so an idle finger cannot pass;
   - the map was in view at the start of the window (when the workout has a route).

   A run that does not count is repeated. It never counts as a pass or as a fail.
4. **Gate 0 passes** when, on the reference phone:
   - three counted chart touch scrubs (AC 2) and three counted route touch scrubs (AC 3) each pass the rule;
   - the operator judges every one of them smooth, with the map marker moving continuously.

   One failing counted run may be repeated once; both are recorded. Otherwise the gate fails, and the F-07 design's "If the gate fails" table applies, starting from the run's charts/map split.
5. **Conditions (accepted by the user, OQ-27, 15 Oct 2026):**
   - Reference phone: the operator's iPhone 13 mini, iOS 27, Orion, Low Power Mode off, portrait.
   - Network and file: home network; synthetic `ride-12h` (43,200 samples, every stream, six charts); map with tiles in view.
   - Desktop reference: Firefox on Linux, window ≥ 1,200 CSS px.
   - Android is checked in F-32.
6. **Later features** that touch the cursor path or the map's drawing (F-17 range layer, F-25 route colouring, F-26 fullscreen) re-run the desktop auto-scrub on their bench page and one phone chart-scrub record with this rule. F-32 repeats the gate procedure.

## Consequences

- The gate is judged on the real input path and the real build, not on synthetic events or a dev build.
- The charts/map split makes a failure actionable without a profiler. It names the hot path (F-05 D-10) or the marker (ADR-013 decision 4) at once.
- **Cost:** measurement code ships in the product behind `?measure`, as a lazy chunk (≤ 4,000 bytes gzip, outside the 60,000). F-13 decides whether it stays; the recommendation is to keep it for F-25 and F-32.
- **Cost:** the operator scrubs by hand, so runs vary. The validity rules and three runs per input absorb that, and the operator's judgement remains part of the gate as the roadmap asks.
- `performance.now()` is coarse on WebKit (1 ms), so phone flush values are quantised. The flush limits are generous enough for that (previous phone runs: p95 1 ms, max 1 ms).

## Would choose differently if

- A Mac with Safari Web Inspector becomes available. Then a Web Inspector timeline of one 10 s scrub joins the record as the trace, and the in-page record stays as the pass rule.
- The record itself costs frames (the "no mutation during the window" test fails in practice, or recorded runs are worse than unmeasured runs feel). Then move the recorder to a separate measurement build.
- The reference phone changes (OQ-27). Then decision 5 is revised and the gate is re-run on the new phone.
