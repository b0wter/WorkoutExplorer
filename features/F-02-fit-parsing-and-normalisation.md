# F-02 FIT parsing and normalisation

- **Phase:** 0 Prototype
- **Owner:** backend
- **Requirements:** AP-2 (device totals), AP-4 (streams), DS-2 (all sports), NFR Reliability; Technical design: FIT parsing, Tech stack (FIT decoding)
- **Depends on:** F-01

## Goal
Turn a well-formed FIT activity file of any length into one normalised workout (sessions, laps, events, streams on one time axis), verified against a corpus of synthetic files in the repo and real files kept only on the operator's machine.

## Scope
- Confirm `muktihari/fit` against the corpus before building on it; record any file it cannot decode. Fallback: another open-source FIT library, never Garmin's SDK.
- Read `file_id`, `session`, `lap`, `record` and `event`. Other messages (`length`, `device_info`, zones, developer fields, running dynamics) are ignored in the MVP.
- Streams: time offset, position, altitude, distance, speed, heart rate, cadence, power, temperature; only those present.
- Conversions and derived values exactly as in the technical design: semicircles to degrees, `enhanced_*` fields preferred, moving time from timer events (fallback: speed threshold 1 km/h cycling, 0.5 km/h running), elevation gain with smoothing and 2–3 m hysteresis, distance from the file with haversine fallback, resampling to 1 s only across gaps under 5 s.
- Start time and the activity's local time zone (see Questions for review in the README).
- Session totals kept as the device recorded them.
- Test corpus and `make corpus`: runs the parser over every file and diffs the output against stored expected output. Only generated synthetic FIT files are committed. Real FIT files contain the route, times and the device serial number, so they and their expected output are git-ignored and kept only on the operator's machine (user decision, 10 Oct 2026).
- A diagnostic CLI subcommand, shipped in the server binary, that parses a file and prints the normalised result (used by F-03 and F-07).

## Out of scope
- Rejecting non-activity files, CRC handling, truncated-file recovery, parse limits, error logging (F-08).
- Writing `data.bin` (F-03).
- Multisport sections (AP-12), pool lengths (AP-13), device info (AP-14): phase 2.

## Acceptance criteria
- [ ] Synthetic part, committed and checked in CI: the corpus contains synthetic files for indoor without GPS, a GPS dropout of several minutes, a pause of several hours, a multi-session file (triathlon or brick), smart recording, and a 12-hour ride at 1 Hz (about 43,000 records).
- [ ] Real part, verified on the operator's machine (real files are never committed): `make corpus` passes over real files from at least three different FIT writers, i.e. at least three distinct `device.manufacturer` values (the manufacturer that wrote the file, not the watch maker).
- [ ] `make corpus` parses every corpus file without a crash and shows a diff when a parser change alters any output.
- [ ] A semicircle value of 2³⁰ converts to 90.0°.
- [ ] A file with both `altitude` and `enhanced_altitude` uses `enhanced_altitude` (same for speed).
- [ ] A file with timer stop/start events: moving time excludes the stopped interval. A cycling file without timer events: samples below 1 km/h are not counted as moving.
- [ ] A synthetic flat ride with ±1 m altitude noise yields 0 m elevation gain; a clean 100 m climb yields 100 m ± 2 m.
- [ ] A file without a distance stream gets distance computed from positions; a file with neither distance nor positions has no distance stream (not zeros).
- [ ] Smart recording: a 3 s gap is filled at 1 s resolution; a 30 s gap stays a gap.
- [ ] Unknown messages and fields are ignored without error.
- [ ] The multi-session file parses into one workout listing every session in order, without a crash.
- [ ] Parse time and peak memory for the 12-hour file are recorded (input for the limits in F-08).
