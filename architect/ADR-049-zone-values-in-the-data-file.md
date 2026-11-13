# ADR-049 Zone values in the data file (format version 6): file bounds or a basis; the default model in the browser

- **Status:** accepted
- **Date:** 13 Nov 2026
- **Features:** F-49 (zones); F-12 (settings object), F-15 (public build, P-7), F-19 (edit page), F-30 (regenerate)
- **Supersedes:** none. Bumps [ADR-005](ADR-005-workout-data-file-format-v1.md) to version 6 under its item 9
- **Requirements:** AP-10, AP-7, UP-6, UP-9, I-2; NFR Privacy; decision log 13 Nov 2026 (OQ-186 to OQ-191)
- **Normative bytes:** [F-03 design](F-03-workout-data-file-writer.md#data-file-format-version-6-normative) §5.1, §5.9

## Context

AP-10 takes zones from, in order: the sharer's max heart rate or threshold power, the file's zone boundaries, the file's max heart rate or threshold power with a default model, else none. Time in zone is counted in the browser, because it follows the selected range. The edit page shows `draft.bin` (built at upload, before any sharer value exists) and must show the zones of the *saved* sharer values without a second draft file (F-47 TR-9). The real files checked on 13 Nov 2026 show that the two kinds of boundary messages mean different things. Apple's `time_in_zone` writes 5 values at 50 … 90 % of its `max_heart_rate`, or 6 values at 50 … 100 % with the last equal to `max_heart_rate`, and in both cases 6 time totals, the first being time below the first value: so the values are zone *bottoms*, and a last value equal to the max is a cap. Wahoo's `hr_zone` and `power_zone` messages write zone *tops*: power zone 1's value is 55 % of the threshold, the top of Coggan zone 1.

Rejected:
- **Resolved bounds only, model in Go:** the edit page would then need the model again, or a new owner-API field computed per read, to show the sharer's zones before a rebuild of `draft.bin` that never happens.
- **Model in both Go and TypeScript:** two definitions of the same percentages and rounding.
- **`hr_calc_type` alone decides the unit:** the checked files write bpm under a percent calc type.
- **One reading for every source:** read as tops, Apple's 6-value list would give 6 zones and its 5-value list would merge 80–90 % and 90 %+; read as bottoms, Wahoo's 7 `power_zone` messages would give one zone too few.
- **Device time-in-zone totals:** they cannot follow a range or a trim (OQ-186).

## Decision

1. **Format version 6** adds one top-level key, `zones`, with at most `heart_rate` and `power`. Each holds exactly one of:
   - `from`: the file's zones, as the lowest whole bpm or W of zones 2 … k (strictly ascending, 1 to 9 values, so 2 to 10 zones);
   - `basis`: max heart rate (bpm) or threshold power (W); the zones follow the default model.
   A kind is written only when its stream is present in that file. Nothing else about zones (resting or threshold heart rate, calc types, device totals) is written.
2. **The meaning depends on the source.** `time_in_zone` values are the **bottoms** of zones 1 … k; a last heart-rate value equal to the same message's `max_heart_rate` is a cap and is dropped; `from` = values 2 … k. `hr_zone` and `power_zone` messages are the **tops** of zones 1 … n; `from` = the first n − 1 values + 1. In both, time below zone 1 counts in zone 1 (as the model's "below 50 % counts as zone 1") and the last zone has no upper bound. Both Apple files give 5 zones; Wahoo gives 5 and 7.
3. **Heart-rate unit:** bpm (all checked files). A list under a percent `hr_calc_type` whose values are all ≤ 100 is taken as percent and converted with the same message's `max_heart_rate` before the cap check; percent of reserve or of threshold is unusable (resting and threshold heart rate are not used, OQ-186). Power boundaries are always W.
4. **Source order in Go, model in TypeScript.** `workout` picks the source (pure). `draft.bin` gets steps 2 to 4; the public data file gets steps 1 to 4, the sharer's values written as `basis`. The browser turns a `basis` into bounds with the one model in `lib/zones`; on the edit page it applies the saved sharer value as `basis` before the draft's `zones`, the same step 1. Go never applies the model.
5. **Unusable is not an error.** A list or basis that fails the checks of F-49 ZN-2 and ZN-4 is skipped and the next source applies.

## Consequences

- One model, one source-order rule. The edit page shows exactly the public zones after a save, with no owner-API change and no draft rebuild.
- The model is a reader rule: the format description names it (F-03 §5.9) instead of carrying bounds. A model change is a browser change, not a format bump.
- The public data file carries the sharer's max heart rate or threshold power, or the file's. The page shows them as bounds anyway.
- Golden data files, fixtures and page samples are regenerated; the deploy runs `regenerate`. Metadata grows by at most about 80 bytes.

## Reversibility

Cheap: every data file is rebuilt from its original on every deploy (ADR-005). Moving the model to Go is a bump plus an owner-API field.

## Would choose differently if

- A real file writes power boundaries in percent of threshold: then add the ≤-threshold rule of item 3 for power, keyed on `pwr_calc_type`.
- A writer uses `time_in_zone` with tops, or zone messages with bottoms: then key the reading on the writer for that source, not on the message.
- Zones are wanted in server markup (no-JS page) or the preview card: then the model moves to Go and the file carries bounds only.
