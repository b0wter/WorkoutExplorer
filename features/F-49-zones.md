# F-49 Heart-rate and power zones

- **Phase:** 2
- **Owner:** both (backend first: zone values read from the file and the sharer's values into the data file and settings; then the Zones section and the edit-page fields)
- **Lane:** normal (short architect note on where zone values live and the lazy section; one QA pass)
- **Status:** ready for architecture — OQ-186 to OQ-194 resolved 13 Nov 2026 (decision log in [README](README.md))
- **Requirements:** AP-10 (Could, phase 2; picked 13 Nov 2026; source order revised 13 Nov 2026); AP-2, AP-4, AP-7, AP-12, UP-3, UP-4, UP-6, UP-7, UP-9, AC-4, AC-5, SH-4, DS-1, DS-4; NFR Performance (budgets unchanged), NFR Privacy, NFR Reliability
- **Depends on:** F-15, F-17, F-19, F-42, F-47

## Goal
A viewer sees how long the workout spent in each heart-rate and power zone, with zones taken from the file or entered by the sharer.

## What exists
- `zones_target` and `time_in_zone` messages are kept on upload, with max, resting and threshold heart rate and threshold power (UP-9, F-09 OQ-50); so are `hr_zone`, `power_zone` and `session.threshold_power`. The parser reads none of them yet (technical design, FIT messages). The user profile is removed and never a source.
- What real files carry (checked 13 Nov 2026): Apple Watch exports one `time_in_zone` per session (reference: session) with `hr_calc_type` = % of max heart rate, 5 or 6 `hr_zone_high_boundary` values in %, the user's configured `max_heart_rate` (not the workout's) and `resting_heart_rate`; no power. Wahoo ELEMNT writes `hr_zone` (5, `high_bpm`), `power_zone` (7, `high_value`) and `session.threshold_power`; no max heart rate. Garmin Edge: no zone messages; max heart rate only in the removed user profile. No file had `zones_target`.
- Heart-rate and power streams in the data file; range selection and panel (F-17); edit-page settings panel (F-19); trim (F-47); multisport sections (F-42).

## Scope (assumed answers in brackets; see Open questions)
- **Zone values** (OQ-186), per kind (heart rate, power), the first that applies:
  1. the sharer's values (OQ-187);
  2. the file's zone boundaries: the `time_in_zone` boundaries of the first session that has them (heart-rate boundaries given in % converted to bpm with that message's `max_heart_rate`), else `hr_zone` (`high_bpm`) and `power_zone` (`high_value`) messages; the number of zones follows the file;
  3. thresholds with the default model (OQ-188): max heart rate from `time_in_zone.max_heart_rate` or `zones_target`; threshold power from `zones_target` or `session.threshold_power`;
  4. none: no zones of that kind.
- `session.max_heart_rate` is the workout's own maximum and is never a zone basis. Resting heart rate is not used and not copied anywhere (it stays only in the stored file, as uploaded, OQ-50). The device's own time-in-zone totals are not shown; time in zone is always computed from the samples, so ranges and trims stay consistent.
- **Sharer entry** (OQ-187): two optional fields on the edit page, before and after publishing (UP-4, AC-5): max heart rate (bpm) and threshold power (W). Filled in, they replace the file's values; empty, the file's values apply. While empty, each field shows the file's value (max heart rate or threshold power) as a grey placeholder, like the suggested title; a typed value is the sharer's, also when it equals the placeholder. Next to the fields: "For the heart-rate and power zones. Empty: the device's zones and values from the FIT file are used." (decided 14 Nov 2026). Stored with the workout's settings, deleted with it (UP-7, AC-4).
- **Zone model** (OQ-188): heart rate 5 zones by % of max heart rate; power 7 zones by % of threshold power.
- **Zones section** (OQ-189): under the charts, a "Zones" section with heart rate and power, one bar per zone with its bounds (bpm or W), time and share of time, highest zone on top. Under each table a source line: "Estimated from max heart rate 180 bpm" / "Estimated from threshold power 240 W" (default model), or "Zones as recorded by the device (max heart rate 171 bpm)" for the file's zone list, the bracket omitted when the file has no max of its own. The public data file states the device's configured max heart rate and threshold power next to the zone bounds (accepted 14 Nov 2026). Shown only for a kind with both zone values and samples. Lazy, needs JavaScript; on viewing and edit page.
- **Which data** (OQ-190): computed from the public data (after the trim); follows the selected range (AP-7); start and end hiding (SH-4) has no effect, as it removes only positions.
- **Multisport** (OQ-191): one set of zones for the whole workout.
- **Corpus:** synthetic files shaped like the real ones: a run with an Apple-style `time_in_zone` (% of max, 5 and 6 boundaries, `session.max_heart_rate` above the configured max); a ride with `hr_zone`, `power_zone` and `session.threshold_power` (Wahoo-style); a ride with only `session.threshold_power`; a ride with only `zones_target`; a ride with none (Garmin-style); golden files regenerated if the data file changes.

## Out of scope
- Zones per lap, per leg or in the laps table; zone colours on the route (AP-9) or charts.
- Pace zones, entering each zone bound by hand, zone models other than the default.
- Zones in the summary strip, preview card or cursor readout.
- Reading zones from the removed user profile, or estimating max heart rate from the workout.

## Acceptance criteria
- [ ] An Apple-style file shows heart-rate zones with the file's bounds converted to bpm from its configured max, not from `session.max_heart_rate`; the times add up to the time with heart-rate samples (AP-10).
- [ ] A Wahoo-style ride shows heart-rate and power zones with the bounds of its `hr_zone` and `power_zone` messages.
- [ ] A ride with only `session.threshold_power` shows power zones from the default model and no heart-rate zones.
- [ ] A ride with only `zones_target` shows zones from the default model and its thresholds (OQ-188).
- [ ] A file without zone values and without sharer values (Garmin-style) shows no Zones section; a workout without power shows no power zones.
- [ ] The sharer enters max heart rate and threshold power on the edit page; after saving, edit and viewing pages show zones from these values; clearing them returns to the file's values (AC-5).
- [ ] Entered values are deleted with the workout (UP-7, AC-4) and not stored anywhere else.
- [ ] With a range selected, the section shows the range's zones; without, the whole workout's (AP-7).
- [ ] A trimmed workout's zones leave out the trimmed part on both pages (UP-6); no trimmed sample reaches a public file.
- [ ] A truncated file (UP-3) and a multisport workout (AP-12) show zones without error.
- [ ] Zones are listed highest first; each table has the source line for its case: sharer's or file's threshold with the default model ("Estimated from …"), file zone list with its max ("Zones as recorded by the device (max heart rate 171 bpm)") and without (no bracket).
- [ ] On the edit page, empty max heart rate and threshold power fields show the file's values as grey placeholders (none when the file has none); saving them empty keeps the device's zones; typing the placeholder's value saves it as the sharer's value (source line then "Estimated from …"); the note next to the fields reads as in Scope.
- [ ] The how-to dialog (F-39, AP-22) shows the Zones line.
- [ ] The section's code is not in the entry bundle; data file size, LCP and 60 fps scrubbing on `ride-12h` unchanged within noise.

## Open questions
All resolved by the user on 13 Nov 2026 as suggested, OQ-186 refined after a check of real files (decision log in [README](README.md)); OQ-192 to OQ-194 in [F-50](F-50-power-curve-and-best-efforts.md):
- **OQ-186:** source order as in Scope: sharer's values, file boundaries (`time_in_zone`, then `hr_zone`/`power_zone`), thresholds (`time_in_zone.max_heart_rate`, `zones_target`, `session.threshold_power`) with the default model, none. Never `session.max_heart_rate` or the user profile; resting heart rate not used. Time in zone from samples.
- **OQ-187:** sharer enters only max heart rate and threshold power.
- **OQ-188:** heart rate 5 zones at 50/60/70/80/90 % of max (below 50 % counts as zone 1); power 7 zones at 55/75/90/105/120/150 % of threshold (Coggan); threshold heart rate alone gives no heart-rate zones.
- **OQ-189:** own sections under the charts, after laps and splits: "Zones", then "Best efforts" (F-50); lazy, JavaScript only, on both pages; bounds in bpm and W.
- **OQ-190:** public data after the trim on both pages; hiding (SH-4) has no effect; zones follow the selected range; moving time with a sample of that kind, transitions included (AP-2).
- **OQ-191:** one set of zones for the whole multisport workout.
