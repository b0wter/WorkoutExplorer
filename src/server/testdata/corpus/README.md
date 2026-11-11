# Parser corpus

FIT files that every parser change is checked against (ADR-004; F-02 design, section Corpus). `go test ./...` runs `TestCorpus`: every file must parse and its report must equal `expected/<set>/<name>.json` byte for byte. A file the parser rejects (F-08) has a golden **rejection report** instead (`report_version`, `rejected.kind`, `message`, `limit`, `file_type`); `make corpus` prints `rejected` for it where it prints the data file sizes for the others. `make corpus` shows the differences as `diff -u`; `make corpus-update` rewrites the golden reports, and the diff of `expected/` is reviewed like code. `make corpus-gen` regenerates the synthetic files (byte-identical on every run; `TestSyntheticCorpusCurrent` checks this in CI).

```
real/                 real device files, <brand>-<model>-<what>.fit     LOCAL ONLY, git-ignored
synthetic/            files written by tools/corpusgen, <what>.fit
expected/real/        golden report per real file                       LOCAL ONLY, git-ignored
expected/synthetic/   golden report per synthetic file
MEASUREMENTS.md       parse time and memory of the 12-hour file (F-02 AC 12); data file sizes of synthetic files (F-03)
```

Names are lower-case kebab-case and must end in `.fit`. Any other entry in `real/` or `synthetic/` (for example `GARMIN01.FIT`) fails `make corpus` (`UNKNOWN`) and `TestCorpus`; hidden files are skipped. Keep the corpus well under 20 MB.

## Real files (local only)

- **Rule:** real FIT files and their golden reports are git-ignored (user decision, 10 Oct 2026; ADR-004 rev. 2). Never force-add them (`git add -f`). A fresh checkout and CI have no real files; an empty `real/` passes.
- **Adding a file:** copy it to `real/<brand>-<model>-<what>.fit` in lower case. `<brand>` is the manufacturer that **wrote the FIT file**, as `workout-explorer parse <file>` reports it in `device.manufacturer`, not the watch maker (a watch recorded by a third-party app counts under that app's manufacturer value). Then run `make corpus-update`, review `expected/real/<name>.json`, and run `make corpus`.
- **Checking:** before accepting a parser change (F-03, F-08, library upgrades), run `make corpus` on the machine that holds the real files. The line `real: N files (local only)` shows how many real files were checked.
- **Privacy:** Real files: route, times and the device serial number. Kept only on the operator's machine. Their reports contain start times, totals and coordinate extremes, so they are local only too.

AC 2 needs real files from three different FIT writers (manufacturers); see OQ-6 in the F-02 design.

## Synthetic files

| Path | Covers | Privacy treatment |
| --- | --- | --- |
| `synthetic/indoor-no-gps.fit` | Indoor ride: heart rate, power, cadence, speed, device distance, temperature; no positions, no altitude; legacy speed and altitude fields; F-43: `wahoo_fitness` with product name "ELEMNT BOLT", sensors BLE power meter, ANT+ heart-rate monitor, second power meter, ANT+ shifting (unknown type) and a built-in one ("heart-rate monitor, power meter") | synthetic, no personal data |
| `synthetic/indoor-no-distance-no-gps.fit` | AC 8: heart rate only, no distance stream; no `activity` message, so the time zone is unknown (decision 13); F-43: `development` without product name, one ANT+ heart-rate monitor ("Sensors: …" only) | synthetic, no personal data |
| `synthetic/gps-dropout.fit` | 5 min (300 s) without positions mid-ride; device distance present throughout | synthetic, no personal data |
| `synthetic/pause-hours.fit` | Timer stop, 3 h without records, timer start | synthetic, no personal data |
| `synthetic/brick-run-bike.fit` | AC 11: run then ride with a 60 s transition, 2 sessions in order, sport `multisport`; running cadence 85 + 64/128 → 171 spm (R-5) | synthetic, no personal data |
| `synthetic/smart-recording.fit` | AC 9: one gap each of 3 s, 4 s (filled), 5 s and 30 s (kept) | synthetic, no personal data |
| `synthetic/ride-12h.fit` | AC 1, AC 12: 43,200 records at 1 Hz, every stream, laps every 30 min | synthetic, no personal data |
| `synthetic/ride-4h.fit` | Size budget (F-03 AC 5): 4 h at 1 Hz, every stream, extra noise on positions, altitude and power | synthetic, no personal data |
| `synthetic/enhanced-and-legacy.fit` | AC 5, three segments of 60 records: (1) both fields, altitude 410 / enhanced 420 m, speed 6.999 / enhanced 7.999 m/s → 420 m, 7.999 m/s; (2) legacy fields only, 430 m, 5.5 m/s; (3) enhanced fields only, 440 m, 6.5 m/s. Session with `avg_speed` 1 m/s below `enhanced_avg_speed` | synthetic, no personal data |
| `synthetic/timer-pause.fit` | AC 6: timer stop at 600 s, start at 1,200 s; records continue during the stop at 2 m/s, so only the timer events exclude it (moving time 1,200 s) | synthetic, no personal data |
| `synthetic/timer-before-first-record.fit` | ADR-029 rev. 1 (QA-17-r1-1): timer starts with the session at 0 s, first record at 3 s, timer stop at 900 s; the moving-time stream starts at 0 and ends at the summary moving time (900 s) | synthetic, no personal data |
| `synthetic/cycling-no-timer-slow.fit` | AC 6: no timer events; 300 s at 0.2 m/s (0.72 km/h, below the cycling threshold) and 300 s at 0 m/s (moving time 599 s) | synthetic, no personal data |
| `synthetic/flat-noise.fit` | AC 7: 3,600 s flat at 100 m with uniform ±1 m noise (fixed seed), no `total_ascent` → 0 m | synthetic, no personal data |
| `synthetic/climb-100m.fit` | AC 7: 60 s flat, 100 m climb over 1,000 s, 60 s flat, no `total_ascent` → 100 m; time zone +05:30 | synthetic, no personal data |
| `synthetic/no-distance-with-gps.fit` | AC 8: trail run with positions but no distance or speed field: distance and moving time from positions | synthetic, no personal data |
| `synthetic/unknown-messages.fit` | AC 10: manufacturer-specific message 0xFF01, unknown field 250 in every record, a developer field | synthetic, no personal data |
| `synthetic/rolling-hills.fit` | R-8 visible in the corpus: 100 m base, bumps of 20 s rise, 10 s top, 20 s fall, 30 s base; 20 bumps of 2.2 m then 10 of 6 m; uniform ±0.5 m noise; no `total_ascent`. Expected elevation gain with this noise: 63.16 m (the noise-free shape gives 60 m; design v1.2+, R-8 tests). A change of `H` or of the smoothing changes it | synthetic, no personal data |
| `synthetic/system-time-record.fit` | E-8: 600 records at 1 Hz on a 2026 date; in the middle one extra record with `timestamp` 1,000 and one timer stop with `timestamp` 2,000 (device system time, both dropped). Report: 600 samples, 2026 start, last offset 599, no bad timer event | synthetic, no personal data |
| `synthetic/pool-swim.fit` | Decision 33: 25 m pool, 40 active `length` messages and one rest length (ignored), no positions; F-44: `num_active_lengths` 40 | synthetic, no personal data |
| `synthetic/pool-swim-25yd.fit` | F-44: the same swim in a 25 yd pool (`pool_length` 22.86 m, unit statute), `num_active_lengths` 40 | synthetic, no personal data |
| `synthetic/ride-30min.fit` | F-08 base: clean road ride, 1,800 records at 1 Hz (offsets 0 to 1,799), every stream, laps every 300 s, one session, timer start and stop, `activity` (+2 h). Reference for the derived files below | synthetic, no personal data |
| `synthetic/ride-30min-cut-60.fit` | F-08 AC 4, AC 6: the first 60 % of the bytes of `ride-30min.fit`. Truncated, no session and no `activity` (time zone unknown, OQ-34); totals come from the records | derived from synthetic `ride-30min.fit` |
| `synthetic/ride-30min-cut-60-size-0.fit` | F-08 R-c: the cut file with header data size 0 and header CRC 0 (a device that never closed the file). Same report as the cut file | derived from synthetic `ride-30min.fit` |
| `synthetic/ride-30min-bad-crc.fit` | F-08 AC 5: file CRC damaged, records intact. `crc_failed`, otherwise the base report | derived from synthetic `ride-30min.fit` |
| `synthetic/ride-30min-bad-header-crc.fit` | F-08 AC 5: header CRC damaged. `crc_failed`, otherwise the base report | derived from synthetic `ride-30min.fit` |
| `synthetic/ride-30min-no-crc.fit` | F-08 R-b: the two CRC bytes missing. `crc_failed`, not truncated | derived from synthetic `ride-30min.fit` |
| `synthetic/ride-30min-trailing-zeros.fit` | F-08 R-e: 64 zero bytes after the file. `ignored_bytes` 64 | derived from synthetic `ride-30min.fit` |
| `synthetic/ride-30min-bad-basetype.fit` | F-08 R-a, F-09 cut (ADR-023 item 6a): the decoder rejects a definition (invalid base type) with records after it. `truncated`, 1080 samples, the records behind the definition are not stored | synthetic, derived from `ride-30min.fit` |
| `synthetic/ride-30min-size-short.fit` | F-09 D-21, ADR-003 rev. 6: `ride-30min.fit` with the header data size lowered by 5 so that it ends inside the last record (k = 5; the last record is 18 bytes; the header is 14 bytes; header CRC recomputed, every other byte unchanged). The decoder reads that record in full; the report equals `ride-30min.fit`'s except `ignored_bytes` 5. F-09 stores the records read with the data size repaired, byte-identical to `ride-30min.fit` | synthetic; header data size ends inside the last record; the decoder reads that record in full (ADR-003 rev. 6); F-09 stores the records read with the data size repaired (ADR-023 rev. 5 item 6) |
| `synthetic/ride-30min-header-only.fit` | F-08 AC 2: the 14 header bytes only. Rejected `no_data` | derived from synthetic `ride-30min.fit` |
| `synthetic/no-records.fit` | F-08 AC 2: a complete activity file without `record` messages. Rejected `no_data` | synthetic, no personal data |
| `synthetic/not-fit-png.fit` | F-08 AC 1: a fixed 1x1 PNG. Rejected `not_fit` | synthetic, no personal data |
| `synthetic/not-fit-random.fit` | F-08 AC 1: 4,096 seeded random bytes, first byte 0. Rejected `not_fit` | synthetic, no personal data |
| `synthetic/empty.fit` | F-08 AC 1: 0 bytes. Rejected `not_fit` | synthetic, no personal data |
| `synthetic/course.fit` | F-08 AC 3: `file_id.type` course with 300 records, a lap and 3 course points. Rejected `unsupported_type` | synthetic, no personal data |
| `synthetic/monitoring.fit` | F-08 AC 3: `file_id.type` monitoring_b, 96 monitoring messages. Rejected `unsupported_type` | synthetic, no personal data |
| `synthetic/workout-plan.fit` | F-08 scope: `file_id.type` workout with 3 steps. Rejected `unsupported_type` | synthetic, no personal data |
| `synthetic/chained-ride-run.fit` | F-08 AC 7: two complete activity files in a row: a 20 min ride, then, 15 min after its last record, a 15 min run. 2 sequences, sport `multisport`, additive totals summed, average speed and cadence absent (mixed sports). The distance stream continues across the sequences (R-12); F-43: `garmin` product `edge_530` without product name ("Garmin Edge 530"), ride sensors ANT+ radar and speed and cadence sensor, run sensor BLE foot pod | synthetic, no personal data |
| `synthetic/chained-with-settings.fit` | F-08 R-d: `ride-30min.fit` followed by a settings sequence. `skipped_sequences` 1, otherwise the base report | derived from synthetic `ride-30min.fit` |
| `synthetic/triathlon.fit` | F-08 AC 7: one sequence, 5 sessions (open water swim, transition, ride, transition, run), laps per session, records at 1 Hz; v1.1: a `sport` message at each session start and `lap.sport` (sessions win over these hints); F-43: `garmin` product `fenix6` without product name, ANT+ foot pod, heart-rate monitor and power meter (open-water swims keep `pool_length` 25 m: F-44 negative case) | synthetic, no personal data |
| `synthetic/run-30min.fit` | F-08 v1.1 base of the run cut: clean road run, 1,800 records at 1 Hz, no power, cadence about 171 steps per minute after R-5, laps every 300 s, one `running` session, `sport` message and `lap.sport` | synthetic, no personal data |
| `synthetic/run-30min-cut-60.fit` | F-08 v1.1 AC 4, AC 6 (QA-08-r1-1): the first 60 % of the bytes of `run-30min.fit`. Truncated, no session; sport `running` from the `sport` message, cadence doubled like the base's | derived from synthetic `run-30min.fit` |
| `synthetic/triathlon-cut-90.fit` | F-08 v1.1 AC 4, AC 6, AC 7 (QA-08-r1-1, r1-2): the first 90 % of the bytes of `triathlon.fit`; the run session and its lap are lost. 4 sessions plus a record leg for the run tail; every stream equals the base's prefix | derived from synthetic `triathlon.fit` |
| `synthetic/chained-ride-ride.fit` | F-08 v1.1 AC 7 (QA-08-r1-3): two complete cycling activity files in a row, 600 records each, the second starting 300 s later with its distance back at 0. Sport `cycling`, one non-decreasing distance stream, average speed present; F-43: `wahoo_fitness` without product (manufacturer only) | synthetic, no personal data |
| `synthetic/chained-ride-ride-cut-75.fit` | F-08 v1.1 AC 4, AC 6, AC 7 (QA-08-r1-2): the first 75 % of the bytes of `chained-ride-ride.fit`; the second sequence breaks off before its session. 1 session plus a record leg | derived from synthetic `chained-ride-ride.fit` |
| `synthetic/ride-transition-ride.fit` | F-08 v1.3 AC 8, AC 9 (R-13, OQ-38): ride, transition, ride in one sequence, 3 sessions, no timer events. Every stream is recorded in the transition, with speed (12 m/s at one sample), cadence and power constants above the rides' maxima, and temperature 30, so a value that escapes the transition mask shows in a maximum | synthetic, no personal data |
| `synthetic/swim-bike-distance-fall.fit` | F-08 v1.5 AC 10 (R-14): open-water swim and road ride in one sequence, 2 sessions, explicit device distance that falls twice (0.5 m for 20 s inside the swim; 49 m downward correction at the leg boundary); each lap carries its session's total distance (v1.6) | synthetic, no personal data |
| `synthetic/identifying-data.fit` | F-09 UP-9: a 10-minute ride with invented markers for everything the strip takes out (serial numbers in `file_id` and three `device_info`; ANT device numbers with transmission types; `user_profile`; `hrm_profile` ANT ID; a saved location (message 29); a segment leaderboard entry; `user_profile.global_id`; `ant_channel_id`; `ant_rx` and `ant_tx` with their payloads; `segment_id` with its device ID; `segment_file` with the leader activity IDs and leader strings; `tank_update` and `tank_summary` sensors; and the three segment account keys, `segment_id`'s and `segment_file`'s `user_profile_primary_key` and `leader_group_primary_key`) and for what must stay (`sport.name`, a developer field, `zones_target` and `time_in_zone` with heart-rate and power settings, OQ-50). The golden report shows none of the markers; F-43: both sensors are recognised types (ANT+ heart-rate monitor, power meter) and carry a descriptor and a product name ("Marker Descriptor …", "Marker Sensor Product …") that must reach no page or data file | synthetic, no personal data; markers for the UP-9 strip |
| `synthetic/ride-laps-no-totals.fit` | F-18 L-1: 20 min road ride, every stream, 3 laps with start and end times only (no totals), timer stop 500–560 s with records continuing. Every lap value in the data file is computed as for a range; the golden for the frontend's `lib/range` cross-check | synthetic, no personal data |
| `synthetic/ride-no-laps.fit` | F-18 L-8: 15 min road ride with distance, one session, no lap messages ("No laps recorded.", splits offered); F-43: `development` without product name and no sensors (no device line) | synthetic, no personal data |

Hostile files for the parse limits are not in the corpus: `internal/ingest/hostile_test.go` builds them in memory. `corpusgen -cap-file <path>` writes the all-caps file for the stop-rule measurement (ADR-017 rev. 1 item 7: 500,000 records, 100 sessions, 10,000 laps, 100,000 timer events, 1,000 sport messages, 256 field descriptions) and nothing else; it is never committed.

Synthetic files all use manufacturer `development`, product 1, product name `corpusgen`, no serial number. Their coordinates are generated, not recorded.
