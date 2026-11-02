# F-08 FIT validation, truncated-file recovery and parse limits

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** UP-2, UP-3 (parser side), AP-5, AP-7, AP-8 (distance stream never decreases), NFR Security (time and memory limit), NFR Reliability, NFR Operations; Technical design: FIT parsing (Edge cases to test), Security and privacy (Uploads)
- **Depends on:** F-02

## Goal
Make the parser safe for any uploaded file: reject what is not a recorded activity with a plain message, recover what can be recovered, and never let one file harm the service.

## Scope
- Header and CRC check. Not a FIT file, or no readable records: rejected with a plain error message.
- `file_id` type other than activity (course, workout plan, settings, monitoring): rejected with "Unsupported FIT file type."
- Truncated file or bad CRC: accepted when the records before the break are intact; the parse result carries the elapsed time of the last good record, for the UP-3 notice (shown by F-19).
- Truncated files without a `session` message: totals computed from the recovered records (assumption, see README).
- Chained FIT files and multi-session files: processed as one workout without error: streams on one time axis, additive totals summed over sessions, except elapsed time, which runs from the workout start to the last record including gaps between sessions (AP-2); sport shown as "Multisport" when the sessions have more than one sport, otherwise that sport (AP-12; sections per leg are phase 2).
- Transition sessions (sport "transition", T1, T2) keep only position, distance, heart rate, elevation and temperature; every other per-sample value recorded during them (speed, cadence, power and the like) is dropped. Transitions count toward total distance and moving time; summary speed or pace, cadence and power leave them out, heart rate includes them; a transition counts as a sport for "Multisport" (AP-2, AP-4, AP-12, OQ-38 in README). A file cut inside a transition loses that transition's session, so its tail keeps speed, cadence and power; accepted by the user (OQ-39 in README).
- The distance stream never decreases: where the device's distance falls (a correction, a dropout to near zero), the last higher value is kept until the device's count passes it; missing samples stay missing. Session and lap totals are unchanged (AP-5, AP-7, AP-8; F-08 design R-14, D-25).
- Parse runs with a time limit and a memory limit, set from the F-02 measurements with ample headroom; exceeding either fails only that parse, with "This file is too large or too complex to read."
- Parser failures logged as structured JSON with device make and model, never coordinates.
- Corpus extended with truncated, bad-CRC, chained, non-activity and garbage files, all derived from synthetic files (real FIT files are never committed, see F-02); all run in `make corpus`.

## Out of scope
- HTTP upload handling, size and rate limits (F-09). Showing the truncation notice (F-19).

## Acceptance criteria
- [ ] A PNG renamed to `.fit` is rejected with a plain "not a FIT file" message.
- [ ] A valid FIT header followed by no records is rejected with a plain "no readable workout data" message.
- [ ] A course file and a monitoring file are each rejected with exactly "Unsupported FIT file type."
- [ ] A ride file cut off at 60 % of its length is accepted; streams end at the last intact record; the result reports that record's elapsed time.
- [ ] A complete file with a corrupted CRC but intact records is accepted.
- [ ] A truncated file without a session message gets distance, moving time and other totals from its records; no total is shown as zero because the session message is missing.
- [ ] A chained FIT file and a triathlon file each produce one workout without error.
- [ ] In the triathlon file, the samples of the transition sessions carry position, distance, heart rate, elevation and temperature, and no speed, cadence or power values; total distance and moving time include the transitions; average and max speed, cadence and power exclude them.
- [ ] A ride, transition, ride file shows the sport "Multisport".
- [ ] In every accepted corpus file, the valid values of the distance stream never decrease; a file whose device distance falls keeps the last higher value there, and its summary distance still equals the device's session total.
- [ ] A crafted file that makes the decoder loop or allocate excessively is stopped by the limits; the process keeps serving other requests.
- [ ] Every rejection and recovery writes a log line with device make and model; a test asserts no latitude/longitude appears in any log line.
