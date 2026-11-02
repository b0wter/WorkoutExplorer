# F-07 Prototype page and scrubbing gate: history

History of [architect/F-07-prototype-page-and-scrubbing-gate.md](../F-07-prototype-page-and-scrubbing-gate.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Header as written

- **Version:** 1.4 (1.4, 15 Oct 2026, QA r3:
  - QA-07-r3-2: the OQ-29 row keeps only the decision. Facts about the real file live in the QA report only (ADR-004 rev. 3 item 7, PP8).
  - QA-07-r3-3: PP8 says how to name a real file and to run `make corpus-update` before `make ci`.
  - The bundle figures quote the close-out E = 45,582 (ADR-011 rev. 7).

  1.3, 15 Oct 2026, user answers and the operator's first runs:
  - OQ-10, OQ-22, OQ-24, OQ-26, OQ-27 and OQ-29 are closed; OQ-23 and OQ-25 are accepted provisionally; OQ-28's preview card is the user's own choice, and its laps and splits columns are accepted provisionally.
  - The first runs got no brotli over plain HTTP from Firefox or Orion, so `-tls` is required for realistic transfer sizes (procedure step 4).
  - PP2 and PD4 say when the finger or mouse must be moving.

  1.2, 15 Oct 2026, QA r1:
  - QA-07-r1-1: every frame of the window without a finger counts as a no-finger frame, also before the first touch (F-4 step 4). The reason text and the tests change with it.
  - QA-07-r1-5: the body rule moves from an inline `<style>` in `index.html` into the bundle CSS, because ADR-014's `style-src 'self'` would block an inline style.

  1.1, 15 Oct 2026, frontend engineer stop at the bundle measurement:
  - The F-06 (b) baseline does not include the data file reader. The share is now measured against **B0′** = (b) + `loadDataFile`, and the ledger is corrected in ADR-011 rev. 7.
  - The step 5 check greps `(resolution: `, because uPlot already contains `dppx`.
  - The operator procedure uses port 18090.
  - The engineer's choices where the design was silent are confirmed (section "Engineer choices, confirmed").

  1.0: first version)
- **Status:** accepted. The open questions (OQ-10, OQ-22 to OQ-29) block no implementation step. They are product decisions for the spec (AC 8).

## Hand-off to requirements (AC 8)

When the user has answered OQ-22 to OQ-28 (and OQ-10), the main session routes them to `senior-requirements-engineer`. The changes go to:
- the spec: tick the open question "Metric definitions" with the decisions, and amend AP-2, AP-5, AP-6, AP-7 and SH-5 where an answer adds a rule;
- the feature files F-13, F-17, F-18 and F-23;
- the README "Questions for review" item 5, which then stops being an assumption.

The architect updates `TIE_PX` (F-06) if OQ-26 changes it, and ADR-016 if OQ-27 changes the conditions.

## Open questions

**Answered by the user on 15 Oct 2026 (v1.3).** The requirements engineer writes these decisions into `spec/` and `features/`.

| ID | Answer | Status |
| --- | --- | --- |
| OQ-10 | Accepted as recommended: "The workout data could not be loaded.", plus "Reload the page." for an unsupported version. F-07's default stands | Closed |
| OQ-22 | Accepted as recommended | Closed |
| OQ-23 | Accepted as recommended, **provisionally**: the user wants to see the range panel in action (F-17) before it is final | Provisional; revisit in F-17 |
| OQ-24 | Accepted as recommended (summary = device totals, range = computed, no note) | Closed |
| OQ-25 | Accepted as recommended, **provisionally**: elapsed time with pauses as breaks; the user may revisit the time axis later | Provisional |
| OQ-26 | Keep what is built (nearest sample, ties within 3 CSS px → earliest; `TIE_PX` unchanged) | Closed |
| OQ-27 | Accepted as recommended. The user: "it's a midrange phone that should be representative for a lot of devices". ADR-016 decision 5 holds, and the Gate 0 verdict becomes final when the gate passes | Closed |
| OQ-28 | **Preview card: the user's own choice**, not the recommendation. It shows the workout name (truncated); moving time, or elapsed time if the workout has no GPS; distance if available; average heart rate if available. **Laps and splits columns: accepted as recommended, provisionally** ("accepted for now"). Laps: number, time, distance, average speed or pace, gain, average heart rate, power and cadence, with device lap values where recorded. Splits: the same columns, computed | Card closed; columns provisional (F-18) |
| OQ-29 | The gate stays on synthetic `ride-12h`. Real files are information only (PP8), kept local and never committed (ADR-004 rev. 3 item 7). The PP8 results are in the QA report (`qa/F-07-prototype-page-and-scrubbing-gate-r3.md`), not in architecture docs | Closed |

The questions and recommendations as asked:

| ID | Question (spec location) | Recommendation | Blocks |
| --- | --- | --- | --- |
| OQ-10 | What does the viewer see when the data file cannot be loaded or decoded? (spec silent; F-04) | "The workout data could not be loaded.", plus "Reload the page." for an unsupported format version. No technical detail. The summary and laps in the HTML stay (F-13). F-07 implements this as the default | Wording in F-13 only |
| OQ-22 | Which summary metrics each sport shows (AP-2; spec open question) | No per-sport list: show every AP-2 metric the workout has, hidden when missing, never zero. The sport decides only pace or speed (DS-2) and the cadence unit (rpm for cycling, spm for running, walking and hiking). Values are the device's session totals; where the device recorded none (for example no total ascent), the server's computed value fills in (F-02) | F-13 summary |
| OQ-23 | Which values the range panel shows, and how averages treat zeros, missing samples and pauses (AP-7) | Distance, elapsed time, moving time, elevation gain, average speed or pace (= distance ÷ moving time), and average heart rate, power, cadence and temperature. Averages are over the samples in the range (1 s each). Missing samples are left out, never counted as zero. Pauses add nothing, because a gap of 5 s or more has no samples. Power counts zeros (coasting is real zero output); cadence leaves zeros out (average while pedalling or stepping) | F-17, F-18 |
| OQ-24 | Selecting the whole workout gives values computed from the streams, which can differ from the device totals in the summary (synthetic `ride-4h`: device ascent 1,631 m, computed gain 768 m). Is that acceptable? (AP-2 vs AP-7) | Accept it. The summary shows the device's totals (AP-2) and the range panel shows values computed from the streams (TD). No note on the page | F-17 |
| OQ-25 | How the time axis shows pauses (AP-5) | Elapsed time since the start; a pause (gap of 5 s or more) is a break that takes its real width. This is as built in F-05. The distance axis (AP-5 toggle) is the compact alternative for long pauses | F-17 axis toggle (confirms F-05) |
| OQ-26 | Which point wins when hovering a route that crosses itself (AP-6) | As built: the nearest positioned sample within the hit radius. Samples within 3 CSS px of the nearest count as a tie, and the earliest wins (F-06 D-9, ADR-015 decision 5, `TIE_PX` = 3) | Nothing (confirms F-06) |
| OQ-27 | Reference phone and test conditions for the performance targets (NFR Performance; spec open question) | 60 fps: the operator's iPhone 13 mini, iOS 27, Orion; Low Power Mode off; portrait; production build over the home network; synthetic `ride-12h` with six charts and the map in view; the in-page scrub record and pass rule of ADR-016. This record counts as the "performance trace" of F-07 ACs 2 and 3, because no profiler is available for the phone in this setup. Desktop reference: Firefox on Linux. Android: checked in F-32. LCP conditions (4G): defined in F-13 | The Gate 0 verdict is valid only with this answer; implementation is not blocked |
| OQ-28 | Columns of the laps and splits table (AP-8) and the fallback order of the preview card's three numbers (SH-5) | Laps: number, time, distance, average speed or pace, gain, average heart rate, power and cadence. Device lap values are used where recorded, otherwise values computed as in OQ-23. Splits (1 km or 1 mi): the same columns, computed, with the last partial split showing its own distance. Preview card: distance, moving time, elevation gain; a missing one is replaced by the next of elapsed time, average heart rate, average power | F-18, F-23 |
| OQ-29 | Does the operator have a real ride of about 4 h, or a real workout of 10 h or more, to load as information? (operator input) | The gate stays on synthetic `ride-12h`; real files are optional information (PP8) and are never committed | Nothing |
