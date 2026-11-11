# F-43 Device and sensor info

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** normal (short architect note on the new metadata fields; QA checks that no identifier reaches the public data)
- **Requirements:** AP-14 (Could, phase 2); AP-2, UP-9, INF-1; NFR Privacy (store as little as possible), NFR Performance (budgets unchanged); decisions of 11 Nov 2026 in [README](README.md)
- **Depends on:** F-15, F-19, F-29

## Goal
A viewer sees what the workout was recorded with: "Recorded with Wahoo Fitness ELEMNT BOLT · power meter, heart-rate monitor".

## What exists
- The parser reads `file_id` manufacturer, product number and product name (never the serial number) into `Workout.Device`; it is used only for the parse log line and the parse report. `device_info` is not read.
- The data file metadata holds no device (ADR-005 item 7); the page shows none.
- UP-9 blanks every serial number and ANT+ device number in the stored file; `device_info` messages are otherwise kept, including user-chosen names (decision log, "Other identifying fields", answer 5).

## Scope
- **Device name, fallback chain (decision 1).** Manufacturer and model, for example "Wahoo Fitness ELEMNT BOLT" or "Garmin Edge 530" (no repeated brand when the model already starts with it); if the model is not known, the manufacturer only, for example "Wahoo Fitness"; if neither is known, no device name.
- **Model name source.** The product name the device recorded (`file_id`, else the creator's `device_info`), cut to 40 characters; else, for Garmin-family devices, the FIT profile's product name made readable ("edge_530" → "Edge 530"). No maintained name table. Text an app writes into the product name is shown as recorded (accepted risk).
- **Manufacturer.** The FIT profile name made readable ("wahoo_fitness" → "Wahoo Fitness"); `development`, `unknown` and numbers not in the profile count as not known.
- **Sensors (decision 1).** Only sensors of a recognised type, named by their type only, each type once, in this order: heart-rate monitor, power meter, speed sensor, cadence sensor, speed and cadence sensor, foot pod, radar. Unknown types and the recording device's built-in sensors are left out. A chained file lists the sensors of every recording it shows.
- **Never shown or published:** serial numbers, ANT+ device numbers, ANT transmission types, `device_info` descriptors (user-chosen names), software and hardware versions, battery values. The data file and the page carry only the shown names and types.
- **Where.** One muted line directly under the summary strip (AP-2), on the viewing page (rendered on the server, works without JavaScript) and on the edit page. Not in the header (AP-1) and not a summary cell (cells are metrics, DS-5). On a phone the line wraps. Without a device name and without sensors, no line and no space.
- **Without a device name but with sensors** the line lists the sensors alone ("Sensors: heart-rate monitor").
- **Older workouts (decision 2).** A workout built before this feature may show no device line; acceptable, no compatibility work. A data file format change bumps the version (ADR-005 item 9).
- **About page (INF-1).** One sentence: the page shows the device's model or manufacturer and the types of sensors used, never their serial numbers or IDs.

## Out of scope
- Sensor makes and models, battery state, firmware.
- An owner setting to hide the line.
- Preview card and image (SH-5, SH-12), range panel, laps, How to use dialog.
- The viewer download (SH-8, F-24, phase 3).

## Acceptance criteria
- [ ] A synthetic file with a recorded product name longer than 40 characters shows it cut to 40; a Garmin-family file without one shows the profile name made readable; manufacturer `development` or `unknown` counts as not known.
- [ ] A file without a device name but with sensors shows "Sensors: …" only.
- [ ] A synthetic file with a known model name shows "Recorded with <model>"; with a known manufacturer but no known model, the manufacturer only; with neither and no sensors, no line and no reserved space.
- [ ] A synthetic file with a heart-rate monitor, a power meter, a second power meter and a sensor of an unknown type lists "heart-rate monitor" and "power meter" once each, in that order, and nothing else.
- [ ] A synthetic file whose `device_info` messages carry serial numbers, ANT+ device numbers and a descriptor (`identifying-data.fit`, given a descriptor): neither the data file nor the viewing page, edit page or owner API contains any of these values (byte search for each).
- [ ] The line appears under the summary on the viewing page with JavaScript off and on the edit page, the same text on both.
- [ ] A phone at 360 px wraps the line without horizontal scroll, in both themes.
- [ ] Multisport and chained files (`triathlon.fit`, `chained-ride-run.fit`) show one line for the whole workout.
- [ ] The About page states what the line shows and that serial numbers and IDs are never shown.
- [ ] Data file size and LCP are unchanged within noise; every corpus file still uploads and renders.

