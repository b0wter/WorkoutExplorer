# F-49 Heart-rate and power zones: architecture

- **Version:** 0.1
- **Status:** accepted (13 Nov 2026). Normal lane. OQ-186 to OQ-191 decided by the user on 13 Nov 2026 and built in here; ZN-2 revised the same day after a check of real files; OQ-195 open, not blocking (suggestion built unless the user objects)
- **Feature:** [`features/F-49-zones.md`](../features/F-49-zones.md)
- **Requirements:** AP-10, AP-7, AP-2, AP-4, AP-12, UP-3, UP-4, UP-6, UP-7, UP-9, AC-4, AC-5, SH-4; NFR Privacy, NFR Performance
- **Builds on:** new [ADR-049](ADR-049-zone-values-in-the-data-file.md) (format version 6); [F-12](F-12-owner-api.md) settings object, [F-15](F-15-publish-pipeline-and-static-page.md) P-7 (both noted in place); [F-17](F-17-range-selection-and-axis-toggle.md) selection store; [F-19](F-19-edit-page-and-settings.md) E-5 to E-7; [F-42](F-42-multisport-sections.md) MS-9 (import trigger); [F-47](F-47-trim.md) TR-9; ADR-011, ADR-019, ADR-023, ADR-026, ADR-029
- **Owner:** both. Backend first (ZN-1 to ZN-4, ZN-6), then the frontend (ZN-5, ZN-7 to ZN-9)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| ZN-1 | **Read in `ingest`, FIT-free out.** `time_in_zone` with `reference_mesg` = session only (lap-referenced ones ignored): `hr_zone_high_boundary`, `hr_calc_type`, `max_heart_rate`, `power_zone_high_boundary`; `hr_zone.high_bpm`, `power_zone.high_value`; `zones_target.max_heart_rate`, `zones_target.functional_threshold_power`; `session.threshold_power`. `ingest` hands `workout` candidate lists already in bpm or W (ADR-049 item 3) and candidate bases, each in source order. Calc types, resting and threshold heart rate and the device's time-in-zone totals are never read. No new parse budget: a list longer than 10 values is dropped while reading, so memory stays bounded by `MaxSessions` | Anti-corruption layer (I-2). The unit quirk is a FIT-writer matter; source order is domain |
| ZN-2 | **Boundary list → zones, by source** (ADR-049 items 2, 3). **`time_in_zone`:** the values are the **bottoms** of zones 1 … k (value 0 tops "zone 0", below zone 1); a last heart-rate value equal to that message's `max_heart_rate` is the cap and is dropped first; `from` = values 2 … k. **`hr_zone` / `power_zone` messages:** one list each in file order; the values are the **tops** of zones 1 … n; `from` = the first n − 1 values + 1. In both, time below zone 1 counts in zone 1 and the last zone is open above. Usable when, after trailing FIT-invalid values are removed (and the cap dropped): 2 to 10 zones, all values valid, strictly ascending, every `from` value within the stream's range (bpm 1–254, W 1–65,534). Heart rate under a percent `hr_calc_type` with all values ≤ 100 is percent: % of max → bpm = p × that message's `max_heart_rate` / 100, half away from zero, the max valid by ZN-4's range, before the cap check; % of reserve or threshold → unusable. Real files (checked 13 Nov 2026): Apple 5 values (50 … 90 % of max) and 6 values (50 … 100 %, last = max) both give 5 zones; Wahoo 5 `hr_zone`, 7 `power_zone` give 5 and 7 zones (power zone 1 top = 55 % of threshold) | The writers' own meaning, which differs by message: `time_in_zone`'s 6 time totals for 5 values show value 0 as the top of a below-zone-1 bucket |
| ZN-3 | **Source order, pure in `workout`, per kind independently.** (1) the sharer's value as basis, public build only; (2) the first usable list: session `time_in_zone` messages in file order (so in multisport the first session that has a usable list wins), then the `hr_zone` / `power_zone` list; (3) the first valid basis: heart rate `time_in_zone.max_heart_rate` (file order), then `zones_target.max_heart_rate`; power `zones_target.functional_threshold_power`, then `session.threshold_power` (session order); (4) none. `session.max_heart_rate` has no path into this rule. `draft.bin` uses (2) to (4). One set for the whole workout (OQ-191). Trim and hiding never change zone values: they are the athlete's settings, not samples. A kind is written only when its stream is present after the public transform | One rule, applied by `publish` with the row's values and by the draft build without them |
| ZN-4 | **Sharer values.** Settings `zones_max_heart_rate` (integer 100–250 bpm) and `zones_threshold_power` (integer 50–1,000 W), or `null` = use the file (OQ-195). The same ranges make a file basis valid. Two nullable `INTEGER` columns of the same names, one forward-only migration (ADR-019); deleted with the row (I-4, AC-4); never logged (I-5). `PATCH` accepts `null` for these two (as F-47 TR-3). Both join the P-7 allow-list: a change rebuilds in place, it cannot hide anything. On a draft they are stored and applied at publish. The names keep them apart from the summary's `max_heart_rate`, the workout's own maximum | AC-5, UP-7. The field name prevents the one confusion the spec forbids |
| ZN-5 | **Model and time in zone, in `lib/zones`** (pure TypeScript, imports `lib/datafile` types only, tested in Node). Basis → `from`: heart rate b × 60, 70, 80, 90 %, power b × 55, 75, 90, 105, 120, 150 %, each half away from zero. A sample `v` is in zone 1 + (number of `from` values ≤ `v`). Over a window `[a, b]`: each `i` in `(a, b]` with a value adds `M[i] − M[i−1]` of the cumulative moving-time stream (ADR-029) to its zone; without that stream, `t[i] − t[i−1]` when under 5 s, else 0. The total is the moving time with a sample of that kind, transitions included (OQ-190): power has none there, heart rate has. Share = zone time ÷ total. A kind whose total is 0 in the window is not shown. Zone 1 is printed with its upper bound only, the last zone with its lower bound only | One model (ADR-049 item 4). The time counted for a range equals the range's moving time where samples exist |
| ZN-6 | **Data file version 6** (ADR-049): top-level `zones`. Bytes normative in [F-03](F-03-workout-data-file-writer.md) §5.1, §5.9 | ADR-005 item 9 |
| ZN-7 | **Which samples and values.** Viewing page: the public data, window `[0, N−1]`, narrowed to the selected range (AP-7). Edit page: `draft.bin` within the **saved** trim window (TR-9's mirror of TR-4), narrowed to the range; zone values = the saved sharer value as `basis` when set, else the draft's `zones`; only for a kind whose stream `draft.bin` has. Unsaved field input changes nothing until saved (E-6). Hiding has no effect (SH-4 removes positions only) | OQ-190 on both pages; the edit page shows what the public build writes |
| ZN-8 | **Section, lazy.** The template gains one empty, client-owned slot after the laps module (ADR-026 rev. 1); no markup without JavaScript. Chunk `src/zones/` (section and its CSS), imported by `src/page` after the charts' first paint (MS-9 trigger) when a kind has zone values and its stream is present; on the edit page the owner glue checks again after each save. It recomputes on a selection flush only, O(b − a), without allocating, never on a cursor flush. If the range-drag or scrubbing check on `ride-12h` regresses, it recomputes when the drag ends instead. A units switch changes nothing (bpm and W) | AC "not in the entry bundle"; ADR-008, F-17 R-11 |
| ZN-9 | **Budget.** Static (entry) ceiling **150 bytes gzip**: the reader's `zones` check, the slot, the `import()` condition. Section and `lib/zones` are lazy. Measure the entry before and after and record it in ADR-011 item 4; above 150: stop and report. Data file: `ride-12h` within noise | ADR-011 reserve 3,349 at entry 56,651 |

**F-50 starts from here.** `src/page` gives every lazy analysis section one value, the **public window**: `[0, N−1]` on the viewing page, the saved trim window on the edit page, updated after a save. F-49 builds it for ZN-7; F-50 reuses it and adds its own slot after the Zones slot and its own chunk, with no server or format change.

**UP-9, ADR-023: no change.** `zones_target` and `time_in_zone` stay in `original.fit` as uploaded, resting heart rate included (OQ-50). Since `ingest` never reads resting or threshold heart rate (ZN-1), neither reaches the data file, the database or a log.

**Module boundary**

| Module | Holds | May import |
| --- | --- | --- |
| `ingest` (Go) | ZN-1, the unit part of ZN-2 | unchanged |
| `workout` (Go) | ZN-2 checks, ZN-3, the ZN-4 range check for file bases | unchanged |
| `lifecycle`, `store` | ZN-4 settings and columns | unchanged |
| `lib/zones` (new) | ZN-5 | `lib/datafile` (types) |
| `src/zones/` (new chunk) | section, ZN-7 selection and window glue | `lib/zones`, `lib/format`, `lib/datafile` (types) |
| `src/page` | slot, `import()`, public window | as today |

## Verification

Synthetic files only (ADR-004); the corpus files of the feature scope, generated by `corpusgen`.

| What | Check |
| --- | --- |
| Privacy | The Apple-style file carries marker values for resting heart rate, threshold heart rate and the device's time-in-zone totals. None appears in any data file's metadata (draft and public); the other files are derived from it, so no separate `.br` or database check is needed. `session.max_heart_rate` is a shown summary total (AP-2) and must never be a zone value or basis: `TestZonePrivacy` leaves it out of the markers and pins `zones` to the file's bounds |
| Edit equals public | For each corpus file, with and without sharer values and with a trim: `lib/zones` over the edit page's inputs (ZN-7) gives the same bounds and times as over the published data file |
| ZN-2 | Apple-shaped `time_in_zone` with 5 values and with 6 values ending at `max_heart_rate` (both 5 zones, same bounds); Wahoo-shaped messages (tops); a percent list; % of reserve (falls to the max-heart-rate basis); a non-ascending list; 11 values; trailing invalid values; a second session with a usable list when the first has none |
| Mutants | Each must fail a test: `session.max_heart_rate` as basis; % list not converted; `time_in_zone` values read as tops, or messages as bottoms; the cap kept as a zone; samples counted instead of moving time; selection or trim window ignored; sharer value in `draft.bin`; a kind written without its stream |

## Hand-offs

| To | What |
| --- | --- |
| Backend | F-03 §5 is at version 6 (this round); TD FIT messages, Workout data file, Data model and Glossary updated; F-12 settings rows and F-15 P-7 allow-list noted in place |
| Frontend | Wording of the section (heading "Zones", bound labels) is reviewed by the user on screen |

## Open questions

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-195 | Input ranges for the two edit-page fields (spec silent) | Max heart rate 100–250 bpm, threshold power 50–1,000 W, whole numbers; the same ranges make a file's values valid | Nothing (suggestion built unless the user objects) |
