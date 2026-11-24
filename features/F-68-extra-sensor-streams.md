# F-68 Extra sensor streams

- **Status:** planned (user, 24 Nov 2026); listed last in the build order; open questions OQ-326 to OQ-332 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first: parser, data file version 8, publish; frontend: charts, readout, range panel)
- **Lane:** critical: developer fields can carry anything (text, identifiers, positions), so publishing any of them is a privacy decision (UP-9, SH-4); the data file format changes
- **Requirements:** AP-4; UP-9, SH-4, UP-6, UP-7, AC-4; AP-2, AP-6, AP-7, AP-8, AP-11, AP-21, DS-1, DS-4, DS-6; NFR Performance, NFR Privacy, NFR Reliability
- **Depends on:** F-02 (parsing), F-03 and F-04 (data file), F-05 (charts), F-08 R-13 (transitions), F-17 (range panel), F-47 (trim), F-49 and F-50 (power), F-54 (four languages)

## Goal
A runner sees the running dynamics their watch or pod recorded (for example vertical oscillation, ground contact time, stride length) and power from an app such as Stryd, as charts linked to the cursor like heart rate or cadence.

## What exists
- The parser reads `record` messages for position, altitude, distance, speed, heart rate, cadence, power and temperature only; running dynamics and developer fields are not parsed (technical design, FIT parsing). The ingest step already walks developer field definitions to strip and reframe the file (`src/server/internal/ingest/frame.go`) but keeps their bytes.
- UP-9 keeps developer fields and user-chosen names in the stored original, as uploaded. The original is never public (SH-8: owner download only).
- Data file version 7 (F-03, ADR-060): one column per stream from a fixed list. Charts come from a fixed list too (`CHART_ORDER` in `src/web/src/lib/charts/specs.ts`: elevation, speed, heart rate, power, cadence, temperature).
- At transition samples every stream other than position, altitude, heart rate, distance and temperature is dropped, "and any later stream" (glossary, F-08 R-13).

## Scope
- **Streams** (OQ-326): a fixed list of standard FIT record fields, each a new chart below temperature, shown only when the workout has it (AP-4). Suggested: vertical oscillation (cm), ground contact time (ms), stride length (m), vertical ratio (%); unit conversion for imperial (DS-1) where one applies.
- **Developer fields** (OQ-327): only a fixed allowlist of recognised app fields, numeric, one value per sample, mapped onto a stream from the list (e.g. Stryd "Power" in W onto the power stream, OQ-328). Every other developer field, every name and every app identifier stays out of the published data.
- **Where values appear** (OQ-329): chart, cursor readout (AP-6, AP-21) and range panel average (AP-7). Not in the summary strip, laps tables, route colouring or zones.
- **Chart space** (OQ-330): how up to four more charts fit on a phone.
- **Privacy:** an extra stream is published like cadence: trimmed parts removed (UP-6), dropped at transition samples (AP-4), deleted with the workout (UP-7, AC-4). Hiding (SH-4) removes positions only; no stream on the list may carry a position or anything from which a hidden place follows.
- **Older workouts:** a workout built before this feature shows no extra streams until rebuilt (regenerate reads the original, which keeps developer fields). No compatibility promise (workouts are short-lived).
- **Texts:** chart labels and units in English, German, Spanish and French (DS-6).

## Out of scope
- Any developer field outside the allowlist; letting the sharer pick fields; showing app-chosen field names.
- Cycling dynamics (pedal balance, torque effectiveness, platform offset), respiration rate, core temperature, SmO2, unless OQ-326 adds them.
- Owner controls to hide a stream; new summary cells; zones or best efforts for the new streams.
- Any other app integration (non-goals: FIT files only).

## Constraints
- **Data file budget:** 4-hour ride still under 500 KB (NFR Performance); a run with every new stream measured on the corpus. If the new columns threaten LCP, they load after the first charts (OQ-332; the design decides how).
- **Entry bundle (ADR-011):** new chart specs and labels must not push the viewing entry over 62,000 bytes gzip; lazy loading where needed.
- **Scrubbing:** 60 fps on the 12-hour corpus file with every new chart drawn.
- **Reliability:** a field present in only part of the file, invalid sentinels, a developer field redefined mid-file, two apps writing the same field, a developer field with a wrong base type or array size, and truncated files must not break the page or the upload; an unrecognised developer field is ignored, never an error.

## Acceptance criteria
- [ ] A run with running dynamics shows one chart per present stream from the OQ-326 list, below temperature, with unit and the cursor readout (also in fullscreen, AP-21); a run without them looks as today.
- [ ] A run with Stryd power and no native power shows the power chart, power curve and zones from it (OQ-328); with both, the rule of OQ-328 holds.
- [ ] A file with a non-allowlisted developer field (test file with a text field, a numeric field named like a place, and a latitude/longitude pair) publishes none of it: the public data file, page HTML and preview card hold none of its values or names.
- [ ] The range panel shows the average of each new stream for the range (AP-7); laps tables and summary are unchanged.
- [ ] Trim (UP-6) and transitions (AP-4) remove the new values as for cadence; a multisport file's transitions show no running dynamics.
- [ ] Units switch (DS-1) updates labels and values where a unit converts; both themes (DS-4); phone portrait at 360 px, landscape and desktop; new texts in all four languages (DS-6).
- [ ] Data file and entry bundle budgets met; LCP unchanged within noise; 60 fps scrubbing on the 12-hour file; every corpus file renders; the fuzz corpus for the parser includes developer fields.
- [ ] UP-9 behaviour follows OQ-331; About page (INF-1) lists the new data if OQ-331 changes what is stored.

## Open questions
- [ ] **OQ-326 Which standard streams.** Suggest: vertical oscillation, ground contact time, stride length, vertical ratio (the common Garmin, Coros and pod set). Alternative: also ground contact balance and cycling dynamics. Affects Scope "Streams", data file size, AC 1.
- [ ] **OQ-327 Developer field allowlist.** Suggest: only Stryd's "Power" (W) at first, recognised by field name and units in its field description; the list grows only by a spec change. Alternative: accept any numeric developer field whose description maps to a native field (`native_field_num`). Affects privacy, AC 3.
- [ ] **OQ-328 Native power and Stryd power together.** Suggest: native record power wins; Stryd power only fills the power stream when the file has no native power; never two power charts. Alternative: Stryd always wins for runs. Affects AP-4, AP-10, AP-11, AC 2.
- [ ] **OQ-329 Where the values appear.** Suggest: chart, cursor readout and range panel average only. Alternative: also summary averages (AP-2) and laps columns (AP-8). Affects AP-2, AP-7, AP-8, AC 4.
- [ ] **OQ-330 Chart space on a phone.** Suggest: every present stream gets its own chart below temperature, same height as the others, no toggle (a run seldom has power, temperature and all four). Alternative: extra charts collapsed behind one "More charts" line. Affects AP-4, AP-18, LCP.
- [ ] **OQ-331 UP-9 revisit.** Suggest: keep UP-9 as is: the private original keeps every developer field and user-chosen name (owner download only, SH-8); the public file gets only allowlisted values, never names or app IDs. Alternative: also remove non-allowlisted developer fields from the stored original (less stored, but the owner's download is no longer the file as recorded). Affects UP-9, INF-1, AC 8.
- [ ] **OQ-332 Loading.** Suggest: new streams in `data.bin` (version 8) as long as the 500 KB budget and LCP hold on the corpus; otherwise the design moves them to a file loaded after the first charts. Alternative: always a separate lazy file. Affects NFR Performance, technical design.
