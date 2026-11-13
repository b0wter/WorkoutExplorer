# F-49 Heart-rate and power zones

- **Phase:** 2
- **Owner:** both (backend first: zone values read from the file and the sharer's values into the data file and settings; then the Zones section and the edit-page fields)
- **Lane:** normal (short architect note on where zone values live and the lazy section; one QA pass)
- **Status:** draft — open questions OQ-186 to OQ-191 (shared with [F-50](F-50-power-curve-and-best-efforts.md))
- **Requirements:** AP-10 (Could, phase 2; picked 13 Nov 2026); AP-2, AP-4, AP-7, AP-12, UP-3, UP-4, UP-6, UP-7, UP-9, AC-4, AC-5, SH-4, DS-1, DS-4; NFR Performance (budgets unchanged), NFR Privacy, NFR Reliability
- **Depends on:** F-15, F-17, F-19, F-42, F-47

## Goal
A viewer sees how long the workout spent in each heart-rate and power zone, with zones taken from the file or entered by the sharer.

## What exists
- `zones_target` and `time_in_zone` messages are kept on upload, with max, resting and threshold heart rate and threshold power (UP-9, F-09 OQ-50). The parser does not read them (technical design, FIT messages). The user profile is removed and never a source.
- Heart-rate and power streams in the data file; range selection and panel (F-17); edit-page settings panel (F-19); trim (F-47); multisport sections (F-42).

## Scope (assumed answers in brackets; see Open questions)
- **Zone values** (OQ-186): the sharer's values first, then the session's `time_in_zone` zone boundaries, then `zones_target` thresholds with the default model (OQ-188), else no zones of that kind. The device's own time-in-zone totals are not shown; time in zone is always computed from the samples, so ranges and trims stay consistent.
- **Sharer entry** (OQ-187): two optional fields on the edit page, before and after publishing (UP-4, AC-5): max heart rate (bpm) and threshold power (W). Filled in, they replace the file's values; empty, the file's values apply. Stored with the workout's settings, deleted with it (UP-7, AC-4).
- **Zone model** (OQ-188): heart rate 5 zones by % of max heart rate; power 7 zones by % of threshold power.
- **Zones section** (OQ-189): under the charts, a "Zones" section with heart rate and power, one bar per zone with its bounds (bpm or W), time and share of time. Shown only for a kind with both zone values and samples. Lazy, needs JavaScript; on viewing and edit page.
- **Which data** (OQ-190): computed from the public data (after the trim); follows the selected range (AP-7); start and end hiding (SH-4) has no effect, as it removes only positions.
- **Multisport** (OQ-191): one set of zones for the whole workout.
- **Corpus:** synthetic rides with `time_in_zone`, with only `zones_target`, and without either; a run with heart rate only; golden files regenerated if the data file changes.

## Out of scope
- Zones per lap, per leg or in the laps table; zone colours on the route (AP-9) or charts.
- Pace zones, entering each zone bound by hand, zone models other than the default.
- Zones in the summary strip, preview card or cursor readout.
- Reading zones from the removed user profile, or estimating max heart rate from the workout.

## Acceptance criteria
- [ ] A ride with `time_in_zone` shows heart-rate and power zones with the file's bounds; the times add up to the time with samples of that kind (AP-10).
- [ ] A ride with only `zones_target` shows zones from the default model and its thresholds (OQ-188).
- [ ] A file without zone values and without sharer values shows no Zones section; a workout without power shows no power zones.
- [ ] The sharer enters max heart rate and threshold power on the edit page; after saving, edit and viewing pages show zones from these values; clearing them returns to the file's values (AC-5).
- [ ] Entered values are deleted with the workout (UP-7, AC-4) and not stored anywhere else.
- [ ] With a range selected, the section shows the range's zones; without, the whole workout's (AP-7).
- [ ] A trimmed workout's zones leave out the trimmed part on both pages (UP-6); no trimmed sample reaches a public file.
- [ ] A truncated file (UP-3) and a multisport workout (AP-12) show zones without error.
- [ ] The section's code is not in the entry bundle; data file size, LCP and 60 fps scrubbing on `ride-12h` unchanged within noise.

## Open questions
Numbered list shared with F-50; see [F-50](F-50-power-curve-and-best-efforts.md) for OQ-192 to OQ-194.

1. **OQ-186 Where zone values come from.** Assumed: sharer's values, then `time_in_zone` bounds of the first session that has them, then `zones_target` thresholds with the default model, else none; time in zone computed from samples, never the device's totals. Affects: parser, data file metadata.
2. **OQ-187 Sharer entry in the first cut.** Assumed: yes, but only two numbers (max heart rate, threshold power), not every zone bound. Alternative: no entry now, file zones only (smaller; most Wahoo and older files then show no zones). Affects: edit page, settings storage, owner API.
3. **OQ-188 Default zone model.** Assumed: heart rate 5 zones at 50/60/70/80/90 % of max (below 50 % counts as zone 1); power 7 zones at 55/75/90/105/120/150 % of threshold (Coggan). A file with only threshold heart rate and no max: no heart-rate zones. Affects: zone bounds.
4. **OQ-189 Where it shows (also F-50).** Assumed: own sections under the charts, after laps and splits: "Zones" (F-49), then "Best efforts" (F-50); lazy chunks loaded with the laps table, JavaScript only, on viewing and edit page; bounds shown in bpm and W (they reveal max heart rate and threshold power: accepted, not identifying). Affects: layout, bundles.
5. **OQ-190 Which data (also F-50).** Assumed: the public data after the trim, on both pages (like the summary, not like the edit page's laps, OQ-178); hiding (SH-4) has no effect; zones follow the selected range, best efforts and power curve show the whole workout only. Time counted is moving time with a sample of that kind; heart rate in transitions counts (AP-2). Affects: computation, edit page.
6. **OQ-191 Multisport (also F-50).** Assumed: zones over the whole workout with one set of zones (OQ-186); best efforts and power curve only within one leg of the matching sport, never across legs or transitions. Affects: computation.
