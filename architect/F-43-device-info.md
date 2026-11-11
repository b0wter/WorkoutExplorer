# F-43 Device and sensor info: architecture

- **Version:** 0.1
- **Status:** accepted (11 Nov 2026). Normal lane. OQ-146 to OQ-148, OQ-151, OQ-152 decided by the user on 11 Nov 2026 and built in here
- **Feature:** [`features/F-43-device-info.md`](../features/F-43-device-info.md)
- **Requirements:** AP-14, AP-2, UP-9, INF-1; NFR Privacy, NFR Performance
- **Builds on:** [ADR-044](ADR-044-device-and-pool-values-in-the-data-file.md) (format version 3, shared with F-44); [F-03](F-03-workout-data-file-writer.md) §5.7; ADR-003 (only `ingest` knows the FIT profile), ADR-023 (UP-9 strip), ADR-026, ADR-011
- **Owner:** both. Backend first, in one step with F-44's backend (one format bump); then the frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| DI-1 | **Read allow-list.** `ingest` reads from `file_id` manufacturer, product, product name (first sequence decides, as today, F-08 E-4); from `device_info` only `device_index`, `source_type`, `device_type`, and for the creator (`device_index` 0) also `product_name`. Never read: descriptor, serial number, ANT+ device number, transmission type, software and hardware versions, battery, any non-creator product name. State kept is bounded: one name, seven flags, however many `device_info` messages the file has | UP-9, INF-1: what is never read cannot leak. ADR-017 needs no new budget |
| DI-2 | **Model.** The recorded product name (`file_id`, else the creator's `device_info`), if it is valid UTF-8; Unicode control and format characters (Cc, Cf: bidi overrides included) removed, whitespace runs collapsed to one space, trimmed, cut to 40 code points, trimmed again; empty → not known. Else, for manufacturers `garmin`, `dynastream`, `dynastream_oem`, `tacx` (the profile's `garmin_product` sub-field), the product number's profile name made readable; a number not in the profile → not known | OQ-146. A crafted name can be odd text but never markup: both renderers print text only (ADR-026) |
| DI-3 | **Manufacturer.** The profile name made readable; `development`, numbers not in the profile and invalid → not known. "Readable": `_` → space, first letter of each word upper-case, the rest as written (`wahoo_fitness` → "Wahoo Fitness", `edge_530` → "Edge 530"). The profile is the pinned library's (ADR-003); a library upgrade may rename a device, accepted | OQ-147; no maintained name table |
| DI-4 | **Device name** = "{manufacturer} {model}" when both are known and the model does not already start with the manufacturer's first word (case-insensitive); else the model; else the manufacturer; else none. Composed once on the server | Feature example "Wahoo ELEMNT BOLT": brand plus model. See OQ-151 |
| DI-5 | **Sensors.** A non-creator `device_info` counts when its `device_type` read under its `source_type` is one of: ANT+ (`antplus`) `heart_rate`, `bike_power`, `bike_speed`, `bike_cadence`, `bike_speed_cadence`, `stride_speed_distance`, `bike_radar`; Bluetooth LE (`bluetooth_low_energy`) `heart_rate`, `bike_power`, `bike_speed`, `bike_cadence`, `bike_speed_cadence`, `footpod`. Every other source type (`ant`, `bluetooth`, `wifi`, `local`) and a missing `source_type` → left out (the profile gives `device_type` no meaning without it). Union over every sequence the workout keeps; each kind once | OQ-148; built-in sensors are the creator's or `local`. See OQ-152 |
| DI-6 | **Data file:** top-level `device` = `name` (string, `""` = none) and `sensors` (codes in the fixed order `heart_rate_monitor`, `power_meter`, `speed_sensor`, `cadence_sensor`, `speed_cadence_sensor`, `foot_pod`, `radar`). Format version 3 with F-44 ([F-03](F-03-workout-data-file-writer.md) §5.7, ADR-044). Package roles: `ingest` holds all profile knowledge (DI-1 to DI-3, DI-5), `workout` composes the name and the order (DI-4), `datafile` writes | One home per rule; the browser never sees FIT names |
| DI-7 | **Line text,** built by both page builders (Go `internal/page`, TS `lib/page`) into the view model field `Device`: name and sensors → "Recorded with {name} · {labels}"; name only → "Recorded with {name}"; sensors only → "Sensors: {labels}"; neither → `""`. Labels joined with ", ": heart-rate monitor, power meter, speed sensor, cadence sensor, speed and cadence sensor, foot pod, radar. The page samples pin all four forms (ADR-026 contract) | Feature scope; renderers format, never compute (ADR-026 item 3) |
| DI-8 | **Placement.** Template: one muted `<p>` after the summary `<section>`, before the privacy notice, printed only when `Device` is non-empty (no space otherwise). Edit page: the same element in `SummaryStrip.svelte`. No hook, no units rewrite, no script on the viewing page | AP-2 "under the summary"; works without JS |
| DI-9 | **Budget (ADR-011).** Static: only `lib/datafile`'s check of the new key and the line's CSS, ceiling **100 bytes gzip**. The TS line builder must stay out of the entry (owner chunk only; the viewing page's line is server markup). The engineer measures the entry before and after and records it in ADR-011 item 4; above 100: stop and report | Reserve 3,493 |
| DI-10 | **Older workouts.** `regenerate` re-parses `original.fit`, which keeps `device_info` (the strip blanks only serial and ANT+ numbers, ADR-023), so every rebuilt workout gets its line; kept workouts stay on their old files (ADR-005 item 9). No compatibility code | Decision of 11 Nov 2026 |

## Not changed

Preview card and image (F-23 `CardItems`), the viewer export (F-24 still writes `development`, product 0; OQ-97), the parse log line and `regenerate`'s deletion log. The golden parse report may gain the sensors; if it does, its report version goes up (engineer's choice).

## Privacy check

QA's byte search (feature AC 35) covers the public data file, `draft.bin`, both pages and the owner API. `identifying-data.fit` must therefore carry a descriptor, serial and ANT+ numbers, and a product name on a non-creator `device_info`; none of these may appear anywhere.

## Open questions

None. OQ-151 (brand prefix as in DI-4) and OQ-152 (engineer checks local real files for `source_type`; if sensors are lost, a missing `source_type` counts as ANT+) accepted by the user on 11 Nov 2026 (decision log, `features/README.md`).
