# Workout Explorer — Features

The product specification and technical design (`../spec/`), cut into features that can each be built and tested on their own in about 1–4 days and end in something that can be demonstrated. Phase 0 (Prototype) and Phase 1 (MVP) are in full detail. Phase 2 is only listed. Later items are not covered.

**How to use this folder**

- Each `F-NN-*.md` file is one feature: goal, scope, out of scope and acceptance criteria. The spec is still the source of truth; requirement IDs link each feature back to it.
- The table order is the build order (mostly by number; F-31 comes before F-24, decided 31 Oct 2026; F-24 deferred to phase 3 and listed last, decided 2 Nov 2026; F-35 first in phase 2, then F-36, then F-34, decided 7 Nov 2026; F-37 after F-34, decided 9 Nov 2026; F-38 after F-37, decided 9 Nov 2026; F-39 then F-40 after F-38, decided 9 Nov 2026). Check **Depends on** before starting. Backend and frontend features without a dependency between them can run in parallel (e.g. F-08–F-12 alongside F-13, F-14 and F-17).
- **Owner** names the agent that builds it: `backend` (senior-backend-engineer), `frontend` (senior-frontend-engineer), or `both`.
- **Status** and **Waiting on** are kept by the main session (orchestrator), not by the requirements engineer. Values: `not started`, `architecture`, `implementation`, `qa rN` (QA round N running or its findings being fixed), `waiting on user` (QA says ready except for conditions only the user can resolve; **Waiting on** names them), `done` (QA ready or conditions met, and committed).
- A feature is done when every acceptance criterion is ticked. Phase 1 starts only after the gate in F-07 has passed.
- Where the spec was silent or ambiguous, the user decided the [Questions for review](#questions-for-review) on 10 Oct 2026; the features and spec follow those decisions.

## Features

| ID | Title | Phase | Owner | Depends on | Status | Waiting on |
| --- | --- | --- | --- | --- | --- | --- |
| [F-01](F-01-project-skeleton-and-ci.md) | Project skeleton and CI | 0 | both | — | waiting on user | GitHub Actions run with Node 24 and the `@v7` tags (browser check passed 2026-10-11) |
| [F-02](F-02-fit-parsing-and-normalisation.md) | FIT parsing and normalisation | 0 | backend | F-01 | done | |
| [F-03](F-03-workout-data-file-writer.md) | Workout data file writer | 0 | backend | F-02 | done | |
| [F-04](F-04-workout-data-file-reader.md) | Workout data file reader | 0 | frontend | F-03 | done | (phone cold median 31 ms, iPhone 13 mini, recorded with F-05) |
| [F-05](F-05-stacked-charts-with-shared-cursor.md) | Stacked charts with shared cursor | 0 | frontend | F-04 | done | |
| [F-06](F-06-route-map-linked-to-cursor.md) | Route map linked to the cursor | 0 | frontend | F-05 | done | |
| [F-07](F-07-prototype-page-and-scrubbing-gate.md) | Prototype page and scrubbing gate | 0 | both | F-03, F-05, F-06 | done | |
| [F-08](F-08-fit-validation-and-recovery.md) | FIT validation, truncated-file recovery and parse limits | 1 | backend | F-02 | done | |
| [F-09](F-09-upload-api-and-storage.md) | Upload API and workout storage | 1 | backend | F-03, F-08 | done | — |
| [F-10](F-10-storage-limit-and-status.md) | Storage limit and status endpoint | 1 | backend | F-09 | done | — |
| [F-11](F-11-upload-page.md) | Upload page | 1 | frontend | F-09, F-10 | done | — |
| [F-12](F-12-owner-api.md) | Owner API: draft data, settings, delete, original download | 1 | backend | F-09 | done | user: tab in description? limits in UP-4? |
| [F-13](F-13-activity-page-layout.md) | Activity page layout and template | 1 | frontend | F-07 | done | user: Lighthouse (AC 7) on Chromium machine |
| [F-14](F-14-units-and-theme.md) | Units, pace or speed, and theme | 1 | frontend | F-13 | done | |
| [F-15](F-15-publish-pipeline-and-static-page.md) | Publish pipeline and static page | 1 | backend | F-12, F-13, F-14 | done | QA-15-r2-1 → F-21/F-30 design; r1-3 → F-22 |
| [F-16](F-16-start-and-end-hiding.md) | Start and end hiding | 1 | both | F-15 | done | |
| [F-17](F-17-range-selection-and-axis-toggle.md) | Range selection and x-axis toggle | 1 | both (backend first) | F-13 | done | |
| [F-18](F-18-laps-and-splits-table.md) | Laps and splits table | 1 | both (backend first) | F-17 | done | |
| [F-19](F-19-edit-page-and-settings.md) | Edit page: draft view and settings panel | 1 | both (backend first) | F-12, F-13 | done | |
| [F-20](F-20-publish-share-delete-controls.md) | Publish, Share and delete controls | 1 | both (backend first) | F-15, F-19 | done | network log check (user) |
| [F-21](F-21-lifetime-and-expiry.md) | Lifetime and automatic expiry | 1 | backend | F-12, F-15 | done | |
| [F-22](F-22-caching-and-security-headers.md) | Caching, fast removal and security headers | 1 | backend | F-16, F-21 | waiting on user | browser hand check via Caddy |
| [F-23](F-23-preview-card-and-indexing.md) | Preview card, Open Graph and indexing | 1 | both (backend first) | F-16 | waiting on user | AC 3 loop-route check; AC 8/9 messengers (deployed) |
| [F-31](F-31-server-deploy-and-monitoring.md) | Server, deploy and monitoring | 1 | backend | F-22 | waiting on user | first deploy: VPS and proxy checks (AC 1, 3–9) |
| [F-25](F-25-route-colouring.md) | Route colouring by metric | 1 | frontend | F-06, F-14 | done (rework 8 Nov 2026) | user: AC 5 scrub record on ride-12h (qa r2) |
| [F-26](F-26-fullscreen-map.md) | Fullscreen map | 1 | frontend | F-06, F-13 | done | |
| [F-27](F-27-reports-api-and-operator-delete.md) | Reports API and operator delete command | 1 | backend | F-15 | built, QA r2 ready | user: VPS checks (no egress without override, delete prompt via exec, report query with sudo) |
| [F-28](F-28-report-form.md) | Report form | 1 | frontend | F-13, F-27 | built, QA r1 ready | user: 429 on a second report (dialog, wording, CSP, real report checked 4 Nov) |
| [F-29](F-29-about-page-and-attributions.md) | About page and attribution list | 1 | both | F-13 | built, QA r2 ready | user: browser check of /about (phone, dark mode, details without JS), privacy wording edit, open the Garmin FIT licence page once |
| [F-30](F-30-regenerate-command.md) | Regenerate command | 1 | backend | F-21, F-23, F-31 | done | r1-6 memory overlap accepted by the user 4 Nov; user: time AC 10 (100 workouts) on the VPS at the first F-30 deploy |
| [F-33](F-33-fullscreen-cursor-readout.md) | Cursor readout in the fullscreen map | 1 | frontend | F-26 | done | device check and fullscreen scrub (responsive) confirmed by the user 5 Nov |
| [F-32](F-32-mvp-launch-checks.md) | MVP launch checks | 1 | both | F-08 to F-33 (except F-24) | QA r1 ready (local checks) | user: tag and push v0.3.0, deploy, U-1 to U-10 (qa r1; U-7c dropped) |
| [F-35](F-35-zip-upload.md) | Zip upload | 2 | frontend | F-11 | QA r1 ready | user: U-1 to U-4 (qa r1) |
| [F-36](F-36-preview-image-with-map-context.md) | Preview image with map context | 2 | both | F-23, F-20, F-30 | done | user: browser, messenger and card look checks (qa r1 list), VPS memory.peak |
| [F-34](F-34-ios-shortcut-upload-link.md) | iOS Shortcut upload link | 2 | both (backend first) | F-29, F-31 | done | user: iPhone Shortcut check (AC 5, qa r1 q1), About look |
| [F-37](F-37-report-notification-links.md) | Report notification links | 2 | both (backend first) | F-27, F-29, F-31 | done | user: lock-screen text shows no key (AC 41; links checked 9 Nov) |
| [F-38](F-38-impressum-page.md) | Impressum page | 2 | both (backend first) | F-29, F-31, F-37 | done | user: /robots.txt check; phone and dark-mode look (page and /api 404 checked 9 Nov) |
| [F-39](F-39-how-to-use.md) | How to use | 2 | frontend | F-14, F-17, F-18, F-26 | specified | |
| [F-40](F-40-shareable-range.md) | Shareable range | 2 | frontend | F-17, F-18, F-39 | specified | |
| [F-24](F-24-viewer-download.md) | Viewer download of the published data | 3 | backend | F-16 | deferred | |

Phase 0: 7 features. Phase 1: 25 features. Phase 2: 7 features (F-35, F-36, F-34, F-37, F-38, F-39, F-40, detailed). Phase 3: 1 feature (F-24, deferred; design kept).

## Requirement coverage

Every requirement with priority Must or Should is in the MVP: the scope table marks the Should items (AP-9, AP-15, SH-4, SH-8 owner download, DS-4) as MVP; the SH-8 viewer download is Later (phase 3), UP-11 is Should within phase 2 (F-35); and the Must items not named in it (e.g. UP-2, UP-4, SH-1, SH-2, SH-6, AC-1, AC-4, AC-5, INF-3) are treated as MVP by their priority.

| Requirement | Priority | Covered by |
| --- | --- | --- |
| UP-1 Upload, two links | Must | F-09, F-11 |
| UP-2 Validation, unsupported types | Must | F-08, F-09, F-11 |
| UP-3 Truncated-file recovery and notice | Must | F-08, F-19 |
| UP-4 Preview before publishing; same page edits | Must | F-12, F-15, F-19, F-20 |
| UP-7 Lifetime and expiry | Must | F-09, F-12, F-13, F-19, F-21 |
| UP-8 Storage limit and notice | Must | F-10, F-11 |
| UP-9 Identifying data removed on upload | Must | F-09, F-12, F-29 |
| UP-10 Upload messages | Must | F-09, F-11 |
| UP-11 Zip archive opened in the browser | Should (phase 2) | F-35 (phase 2) |
| AP-1 Header | Must | F-13 |
| AP-2 Summary strip | Must | F-02, F-08, F-13 |
| AP-3 Map, no-location hint | Must | F-06, F-16 |
| AP-4 Stacked charts | Must | F-04, F-05 |
| AP-5 Time/distance axis | Must | F-17 |
| AP-6 Linked cursor | Must | F-05, F-06 |
| AP-7 Range selection | Must | F-17 |
| AP-8 Laps and splits | Must | F-18 |
| AP-9 Route colouring | Should | F-25 |
| AP-15 Fullscreen map (MVP part) | Should | F-26 (pins in fullscreen: phase 2) |
| AP-17 Two columns on wide screens | Must | F-13, F-17 (splitter), F-19 |
| AP-18 Pinned map on a phone | Must | F-13, F-19, F-26 |
| AP-21 Cursor readout in the fullscreen map | Must | F-33 |
| AP-19 Range in the link | Could (phase 2) | F-40 (phase 2) |
| AP-22 How to use, first-visit range hint | Could (phase 2) | F-39 (phase 2) |
| SH-1 Unguessable URL | Must | F-09, F-15 |
| SH-2 Draft / published (unlisted), no unpublish | Must | F-09, F-12, F-15, F-19 |
| SH-4 Start and end hiding | Should | F-16, F-19 |
| SH-5 Preview card | Must | F-23, F-32; F-36 in phase 2 |
| SH-12 Preview image with map context, made in the browser | Could | F-36 (phase 2) |
| SH-6 Share and copy-link buttons | Must | F-13, F-20 |
| SH-8 Download setting and original | Should | F-12, F-20 (owner original); F-24 (viewer download, phase 3) |
| SH-9 Privacy notice | Must | F-13 |
| SH-10 Fast removal, recall notice | Must | F-15, F-16, F-20, F-21, F-22, F-30; F-24 in phase 3 |
| AC-1 Edit link shown only on the published page | Must | F-09, F-11, F-20 |
| AC-2, AC-3 | Removed | — (removed in the spec) |
| AC-4 Delete removes everything | Must | F-12, F-20, F-21 |
| AC-5 Owner changes everything (MVP fields) | Must | F-12, F-19 (trim and pins: phase 2) |
| AC-6 Published page with both links | Must | F-20, F-22 |
| DS-1 Units, drop-down | Must | F-14, F-18 |
| DS-2 Sport-specific pace/speed | Must | F-14, F-25 |
| DS-3 English | Must | F-11, F-13, F-14, F-28 |
| DS-4 Light and dark theme | Should | F-14, F-15, F-29; F-38 (Impressum page, phase 2) |
| INF-1 About page | Must | F-29 |
| INF-2 Report form, stored reports | Must | F-27, F-28 |
| INF-3 Operator delete | Must | F-27 |
| INF-4 Report notifications (Pushover, opt-in) | Must | F-27; viewing link and readable reason: F-37 (phase 2) |
| INF-5 iOS Shortcut upload link (opt-in) | Could | F-34 (phase 2) |
| INF-6 Single-use delete link in the notification | Should (phase 2, assumed) | F-37 (phase 2) |
| INF-7 Impressum page (opt-in, text loaded by JavaScript, noindex) | Should (phase 2) | F-38 (phase 2) |
| NFR Performance: LCP under 2.5 s | — | F-13, F-32 |
| NFR Performance: 60 fps on 12-hour workout | — | F-05, F-06, F-07, F-25, F-32 |
| NFR Performance: data file under 500 KB | — | F-03, F-07, F-32 |
| NFR Privacy: GDPR (EU hosting, policy, record, deletion) | — | F-12, F-29, F-31, F-32 |
| NFR Privacy: store as little as possible | — | F-09, F-11, F-27, F-31 |
| NFR Privacy: no tracking cookies | — | F-22, F-29 |
| NFR Privacy: `noindex` | — | F-15, F-23; F-38 (Impressum, `robots.txt`) |
| NFR Security: 128-bit IDs, hashed tokens | — | F-09, F-12; delete keys: F-37 |
| NFR Security: upload size, rate, uploads at once, parse limits; zip never reaches the server | — | F-08, F-09, F-11, F-31; F-35 in phase 2 |
| NFR Security: originals never public | — | F-12, F-22; F-24 in phase 3 |
| NFR Privacy: no data outlives its workout | — | F-12, F-21, F-27 |
| NFR Accessibility | — | **Deliberately not covered**: not a goal. Only Esc to leave fullscreen (F-26). |
| NFR Compatibility | — | F-11, F-13, F-26, F-32; F-35 in phase 2 |
| NFR Reliability | — | Guiding principle, not a target: edge-case criteria in every feature; corpus in F-02, F-08; end-to-end sweep in F-32 |
| NFR Operations: parser errors logged with device model | — | F-08, F-30, F-31 |

## Phase 2 (not broken down)

- Suggested title from sport, time of day and nearest town (GeoNames attribution added to About) — UP-5, INF-1
- Trim start and end, with totals recomputed (trims also apply to the viewer download once it ships in phase 3) — UP-6, AP-2, SH-8, AC-5
- Multisport legs and transitions as sections (leg lines and sport labels on charts, separator rows in laps, leg start markers on the map) — AP-12
- Pool swims: lengths and stroke type instead of a map; pool length and number of lengths in the summary (needs a workout data file format change: backend and frontend) — AP-13, AP-16
- Device and sensor info — AP-14
- Heart-rate and power zones; power curve and best efforts — AP-10, AP-11
- Privacy zones of 200 m to 1 km — SH-3
- iframe embed with compact layout — SH-7
- Map pins (types, sections, list, legend, privacy-zone check, owner edits, GPX export), including pins in the fullscreen map — PIN-1 to PIN-9, AP-15, AC-5
- Shareable range — AP-19: detailed in [F-40](F-40-shareable-range.md).
- Summary colours: summary cells tinted with a muted version of their chart's line colour (frontend only) — DS-5, AP-2, AP-4, DS-4. Acceptance criteria, to carry into its feature file:
  - [ ] Elevation gain, speed or pace, heart rate, power, cadence and temperature cells are tinted, each in a muted version of its own chart's line colour; distance, moving time, elapsed time and calories cells stay plain.
  - [ ] In light and dark theme the tint matches that theme's line colour, and every text in a tinted cell has a contrast of at least 4.5:1 against it.
  - [ ] A workout missing a stream shows no cell and no tint for it; a server-computed value is tinted like a device value.
  - [ ] Range panel, laps and splits, cursor readout and preview image are unchanged.
  - [ ] Data file size and the summary's render time (LCP) are unchanged.

## Questions for review

**Decided on 10 Oct 2026.** Every assumption below was accepted as stated, with these answers and changes:

- **1, 4:** fixed in the technical design and the backend agent (expiry job every minute; `robots.txt` does not block `/w/`).
- **2:** wording "This workout has expired or is not available."
- **12:** multi-session and chained files show **"Multisport"** as the sport (not the first session's sport). Narrowed on 15 Oct 2026 (OQ-36, below): only when the sessions have more than one sport.
- **25:** the About page names the operator and a contact address, as **placeholders** to be filled in before launch.
- **29:** cut order if time runs short: SH-8 viewer download, DS-4 dark theme, AP-15 fullscreen map, AP-9 route colouring. SH-4 is not cut.
- **Test corpus (added the same day, from F-02 architecture):** real FIT files are never committed, because they contain the route, times and the device serial number. Only generated synthetic FIT files are in the repo; real files and their expected output stay on the operator's machine, where F-02 AC 1's real part is verified. Broken-file samples (F-08) derive from synthetic files.

**Decided on 14 Oct 2026:**

- **MapLibre size (OQ-20, ADR-013):** the deferred MapLibre budget is "as shipped; 6.14: about 435 KB gzip (JS + worker + CSS)"; an increase of more than 10 % on a MapLibre upgrade is reported to the user (technical design, Performance budgets).
- **GPS dropouts dashed (AP-3):** the straight segment across a GPS dropout (samples without position) or a time gap (5 s or more between consecutive positioned samples) is dashed, so it doesn't look like recorded track (F-06 design v1.4; terms as in the technical design's Glossary). Updated: AP-3, technical design (Front end, step 4), F-06 scope and AC 4, F-25 (new AC: dropouts stay dashed when colouring is on).
- **Dropouts uncoloured (AP-9, F-25):** when the route is coloured by a metric, dashed segments across dropouts and gaps stay dashed and uncoloured; they have no recorded position, so colouring them would suggest data that isn't real.
- **Map start-up pause (OQ-21, F-06):** a single start-up pause of up to 300 ms per page load on the phone, when the map loads, is accepted.

**Decided on 15 Oct 2026** (F-07 metric questions; settles the spec's "Metric definitions" open question and question 5, 10 and 32 below):

- **Reference device (OQ-27):** iPhone 13 mini, iOS 27, Orion, Low Power Mode off, portrait, production build, synthetic `ride-12h` with the map in view; the in-page scrub record counts as the performance trace; desktop reference Firefox on Linux; Android checked in F-32. Reason: a mid-range phone representative of many devices (spec NFR Performance, F-07, F-32). Orion is not in the spec's Compatibility list; accepted, as it is WebKit-based like Safari.
- **Load failure (OQ-10):** "The workout data could not be loaded.", plus "Reload the page." for an unsupported format version; no technical detail (F-13).
- **Summary metrics (OQ-22):** no per-sport list; every AP-2 metric the workout has, missing ones hidden; the sport decides only pace or speed and the cadence unit; device totals, server-computed values where the device recorded none (AP-2, F-13).
- **Range panel (OQ-23), provisional:** distance, elapsed and moving time, gain, average speed or pace (distance ÷ moving time), average heart rate, power, cadence and temperature; missing samples left out, pauses add nothing; power averages include zeros, cadence averages exclude them. Review once visible (AP-7, F-17).
- **Whole-workout range (OQ-24):** may differ from the device totals in the summary; no note (AP-7, F-17).
- **Time axis (OQ-25), provisional:** elapsed time, pauses as breaks of their real width; the distance axis is the compact alternative (AP-5, F-17).
- **Self-crossing route (OQ-26):** nearest sample wins; on a tie within 3 px, the earliest (AP-6, F-06).
- **Laps and splits columns (OQ-28), provisional:** number, time, distance, average speed or pace, gain, average heart rate, power and cadence; device lap values where recorded; splits the same columns, computed (AP-8, F-18).
- **Preview card (OQ-28), the user's own choice:** workout name (truncated), moving time (elapsed time without GPS), distance and average heart rate when available. Replaces question 10's assumption (distance, moving time, gain) (SH-5, F-23).
- **Real long file (OQ-29):** the user provides one for information only; the gate stays on synthetic `ride-12h`; real files are never committed.

**Decided on 15 Oct 2026** (F-08 multi-session questions):

- **Elapsed time across sessions (OQ-35):** elapsed time is tracked across the entire workout. The summary's elapsed time is the time from the workout start to the last record, including transitions and gaps between sessions, not the sum of the sessions; the time axis and cursor count from the workout start across all legs (a swim of 1 h then 30 min of riding shows 1:30:00). Other additive totals stay summed over sessions (AP-2, AP-5, AP-6, F-08).
- **Sport of a multi-session file (OQ-36):** two or more sessions of one sport show that sport (e.g. cycling); "Multisport" only when the sessions have more than one sport. Narrows decision 12 (AP-12, F-08).
- **Anchor of the multi-session elapsed time (OQ-37):** the summary's elapsed time runs from the workout start, as on the time axis and cursor, not from the first record; it equals the cursor's elapsed time at the last record. They differ only when a session or lap starts before the first record. Decided by the user's OQ-35 answer, 15 Oct 2026 (AP-2, AP-5, F-08).
- **Transitions keep only position, distance, heart rate, elevation and temperature (OQ-38):** only sessions with sport "transition" (T1, T2) are transitions. During them, position (the route), distance, heart rate, elevation and temperature are kept and shown; every other per-sample value (speed or pace, cadence, power and the like) is dropped on the server, because changing outfit makes it meaningless. The user's words: "Let's keep the transitions simple. I want gps, heart rate and elevation." Follow-ups settled the same day: transition distance counts toward the total and the distance axis; transition time counts toward moving time ("triathlon counts the total time as result time"); summary average and max speed or pace, cadence and power leave transitions out (device or computed), heart rate includes them; ranges, laps and splits count transition distance and time; route colouring by speed, power or gradient draws transitions in the plain route colour, by heart rate as usual; a transition session counts as a sport, so ride, transition, ride stays "Multisport" (the sharer can edit the title) (AP-2, AP-4, AP-6, AP-7, AP-8, AP-9, AP-12, F-08).
- **File cut inside a transition (OQ-39):** accepted as built. A truncated file whose break lies inside a transition loses that transition's session, so its tail is not treated as a transition and keeps speed, cadence and power. The user's words: "damaged files are a bonus not a must-have". Revisit only if a real truncated file shows tail values the user calls wrong (AP-4, UP-3, F-08 design v1.4).

**Decided on 16 Oct 2026** (F-09 review and architecture questions OQ-40 to OQ-45):

- **Serial numbers removed on upload (UP-9):** the server blanks every serial-number field in the uploaded file, of the recording device and every sensor listed, before storing it as `original.fit`. They are not relevant to the workout, and storing them goes against "store as little as possible". The owner's download is the stored file, not a byte-identical copy. Updated: UP-9 (new), SH-8, INF-1, NFR Privacy, the spec's decided question "Keep the original FIT file?", technical design (Publish 2, Files on disk, Originals), F-09, F-12, F-20, F-24, F-29.
- **Other identifying fields (UP-9, INF-1), the user's answers to the six follow-up questions:** (1) ANT+ device numbers in `device_info` and ANT IDs in profile messages are blanked like serial numbers; (2) the user profile's name, gender, age, height and weight are blanked, resting and maximum heart rate kept; (3) `location` messages (saved places) are removed; (4) segment leaderboard entries are removed; (5) user-chosen names (sport profiles, devices) and developer fields are kept for now, to revisit when extra sensor streams (Later) are planned; (6) the About page lists everything removed, concretely. Updated: UP-9, SH-8, INF-1, NFR Privacy, scope table row, the spec's decided questions "Keep the original FIT file?" and "Other identifying fields in FIT files", technical design (Publish 2, Files on disk, Originals), F-09 (scope, AC on the synthetic file), F-12, F-20, F-29.
- **UP-9 revised and OQ-46 to OQ-48 (F-09 architecture):** the whole `user_profile` message is removed on upload, like saved locations and leaderboard entries; nothing from it is kept, including resting and maximum heart rate (replaces answer 2's "keep resting and max HR"), because the app never reads it and to store as little as possible; AP-10 (phase 2) zones come from zone messages in the file or from the sharer. OQ-46: ANT transmission-type fields are kept (with the device number blank, their 4 bits identify nobody). OQ-47: no longer applies. OQ-48: besides the UP-9 changes, the stored file may differ only in its framing (CRCs repaired, trailing non-FIT bytes and an incomplete last record dropped); the workout data is identical. Updated: UP-9, AP-10, INF-1, NFR Privacy, the spec's decided questions "Keep the original FIT file?" and "Other identifying fields in FIT files", technical design (Publish 2), F-09 (scope, synthetic-file AC, new framing AC, decisions), F-29.
- **OQ-49 and OQ-50 (F-09 architecture):** OQ-49: a truncated file is stored marked as unfinished, the FIT way (data size 0, header CRC 0, no file CRC), so it keeps the UP-3 notice and F-30 can rebuild it; F-09's framing AC now reads "with correct CRCs, or, for a truncated file, marked as unfinished without CRCs". OQ-50: `zones_target` and `time_in_zone` messages are stored as uploaded, including max, resting and threshold heart rate and threshold power; they identify no person or device, and AP-10 names zone messages as its source. Updated: UP-9, the spec's decided question "Other identifying fields in FIT files", technical design (Publish 2), F-09 (scope, synthetic-file AC, framing AC, decisions).
- **QA-09-r1 Q1 (UP-9 ANT IDs literal):** "every ANT+ device number of every sensor" is meant literally; F-09 had narrowed it to `device_info` and the sensor profiles. Also blanked: `ant_channel_id.device_number`, `segment_id.device_id`, and `segment_file.leader_activity_id` (another person's activity ID, consistent with removing leaderboard entries). Removed: the whole `ant_rx` and `ant_tx` messages (raw radio data that can contain sensor IDs; never used). Updated: UP-9, INF-1, technical design (Publish 2), F-09 (scope, AC 6, decisions), F-29.
- **VPS memory (OQ-51):** the server runs on a VPS with at least 2 GB RAM; the concurrent-upload default stays 4; F-31 caps the service's memory with systemd `MemoryMax`, based on the measured worst case with 4 uploads at once (QA measured 620–861 MB). Updated: NFR Operations (new row), technical design (Hosting, Server), F-31 (scope, two new ACs).
- **Segment account keys (OQ-52) and literal-reading additions:** blanked on upload: `user_profile_primary_key` in `segment_id` (148/4) and `segment_file` (151/4) and `leader_group_primary_key` in `segment_file` (151/8), because they point to people and the app never reads them; confirmed under the literal reading of UP-9, also blanked: `segment_file.leader_activity_id_string` (151/10, the leaders' activity IDs as text) and the dive-tank `sensor` fields in `tank_update` (319/0) and `tank_summary` (323/0), which hold an ANT channel ID and so a device number. Full list: `architect/ADR-023` item 4. Updated: UP-9, INF-1, technical design (Publish 2), F-09 (scope, AC 6, decisions), F-29.
- **Damaged record cuts the stored file (QA-09-r2-1, option a, decided 21 Oct 2026):** if the parser stops reading a FIT sequence early because of a damaged record (e.g. a definition with an invalid base type), the stored file ends at that record, like any truncated file; nothing after it is kept. The workout and the UP-3 "file was incomplete" notice are unchanged. Added to UP-9's allowed framing differences next to the OQ-48 items. Updated: UP-9, technical design (Publish 2), F-09 (scope, AC 7, decisions).
- **No long-term storage across upgrades (OQ-53, decided 21 Oct 2026):** (refined by follow-up 1 and OQ-55 below) the app does not store workouts long term. If a future FIT library version reads a stored original differently, or not at all, the workout is lost, and that is accepted. For `regenerate`: the deploy is never stopped to protect old workouts; a workout that regenerates shorter is accepted as is; a workout whose regeneration fails is deleted with everything stored for it, as on expiry (AC-4, UP-7). Replaces the technical design's "keeps its old files and the old bundle stays until the next successful run". Updated: UP-7, spec decided question "Workouts after an app upgrade", technical design (Versioning), F-30 (scope, AC 3, new AC), F-31 (deploy scope and AC, which kept the old bundle on failure).
- **Acceptance rule for malformed or crafted uploads (decided 21 Oct 2026, F-09 v1.9):** on malformed or crafted input, the upload may refuse a file that F-08's parser would accept, with "This file could not be read.". Privacy (UP-9) stays strict. ("Files from real devices are always accepted" was part of this rule until 22 Oct 2026, see below.) **OQ-54** (a compressed timestamp that depends on a removed message) is refused under this rule and closed. Updated: spec decided question "Malformed or crafted uploads", F-09 (requirements line, scope, new AC 14, decisions), coverage row UP-2.
- **Regenerate deletes only rejected files (OQ-53 follow-up 1, decided 21 Oct 2026):** `regenerate` deletes a workout only when the new version rejects its file. On a server-side failure, an interrupted run or a time limit, the workout is kept unchanged for the next run. Refines OQ-53. Updated: UP-7, spec decided question "Workouts after an app upgrade", technical design (Versioning, with a note on OQ-55), F-30 (requirements line, scope, new AC). OQ-55 was decided on 22 Oct 2026 (entry below).
- **Kept workouts and old bundles (OQ-55, decided 22 Oct 2026, the architect's suggestion):** `regenerate` deletes an old app bundle only once no kept workout refers to it. A workout kept after a server-side failure, an interrupted run or a time limit serves its previous files until a later run rebuilds it. The deploy reports kept workouts as well as deleted ones. A run stopped part-way leaves the unprocessed workouts unchanged; the next run rebuilds them. Updated: technical design (Versioning, note removed; Hosting, Deploys), F-30 (goal, scope, AC 1, two new ACs), F-31 (deploy scope, AC 2).
- **"Files from real devices are always accepted" removed (22 Oct 2026):** dropped from the acceptance rule for malformed or crafted uploads, because which errors a device may produce cannot be planned for. The rest of the rule stands, and UP-2 now states it: "A malformed or crafted file that cannot be stored with UP-9 applied may be refused with 'This file could not be read.'" Updated: UP-2, spec decided question "Malformed or crafted uploads", F-09 (scope, decisions).
- **About-page sentence (OQ-53 follow-up 2, decided 21 Oct 2026):** the About page says, exactly, "Workouts are not archived: after an app update, a damaged file may show less than before or be removed." Updated: INF-1, F-29 (scope, AC 2).
- **Parse time limit as a setting (decided 22 Oct 2026; ADR-017 rev. 7, `architect/F-09` D-30):** `WORKOUT_EXPLORER_PARSE_TIMEOUT`, whole seconds, default 10, range 5 to 60; an invalid value stops the server at start. `regenerate` uses the same limit (one `EnvironmentFile=` for the unit and the deploy); Caddy's `/api/*` timeouts stay at or above 120 s + 6·T. Updated: NFR Security, F-09 (scope, AC, decisions), F-30 (scope), F-31 (scope, AC). The technical design already had it (Configuration; Security and privacy, Uploads).
- **Default storage cap 1 GB (decided 23 Oct 2026, F-10 S-1):** when `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` is unset, the cap is 1,000 MB (1 GB), not 10 GB; better safe than sorry, and the operator raises it with the setting. Range unchanged (1 to 1,000,000 MB). Updated: technical design (Configuration), `architect/F-10` S-1.
- **Upload page (F-11 design OQ-56 to OQ-58, and several files dropped, decided 23 Oct 2026):** (OQ-56) F-11 AC 1 is met when the browser opens the new draft's edit link; rendering the edit page is F-19's job (F-19 AC 1). (OQ-57) no Android phone is available now, so the Android Chrome part of F-11 AC 1 is checked in F-32 with the other Android checks. (OQ-58) the page texts suggested in the F-11 design are accepted for now; refined wording can come later. If several files are dropped at once, only the first is uploaded, with no message; while an upload is running, the drop zone and the file picker are disabled, so no second upload starts until the first ends (success or error). Updated: F-11 (scope, AC 1, new AC on the disabled state), F-32 (scope, new AC); technical design (Repository layout, Front end implementation notes: upload page in plain TypeScript, ADR-024).
- **Owner API (F-12 design OQ-59, OQ-60, title and description limits, token only in the header, decided 23 Oct 2026):** (OQ-59) the owner's original download is named `workout-YYYY-MM-DD.fit`, from the local start date, or `workout.fit` when there is no start time; the name is ASCII only and never uses the title. (OQ-60) F-12 AC 5 and AC 6 are ticked for what exists with F-12: the row, `private/{id}/`, the owner endpoints, and the check that no URL served by the Go process returns a private file; the public folder moves to F-15's ACs (gone after delete; no file under `public/` matches `original.fit` or `draft.bin`), reports to F-27's (deleted with the workout); F-22's Caddy AC stays as it is. Title at most 100 characters after trimming leading and trailing white space, description at most 1,000, both counted as Unicode code points; replaces question 17's assumed limits. Owner API calls accept the edit token only in the `Authorization: Bearer` header, and an owner API request with any query string gets the not-found response even when the header is valid (ADR-025, stricter than ADR-020 item 9); the edit link stays `/edit/{public_id}#{token}`, with the token in the fragment, so reloading a tab still works. Updated: F-12 (scope, AC 1, 4, 5, 6, new AC 7), F-15 (scope, publish path, two new ACs), F-27 (scope, new AC); technical design (Publish and view flow 4 and 5, Code structure: Lifecycle row and repository layout, Security and privacy: Private and draft workouts, IDs and tokens, Originals; Glossary: owner API, not-found response).
- **Uploads processed at once (OQ-44):** at most N uploads are processed at once, N a server setting from an environment variable, default 4; the next one gets "Too many uploads right now. Please try again in a minute." Updated: NFR Security, technical design (Uploads), F-09, F-11, F-31.
- **Messages (OQ-40), settles question 20:** too large "This file is larger than 20 MB."; rate limit "Too many uploads. Please try again later."; too many at once "Too many uploads right now. Please try again in a minute."; unreadable upload "The upload could not be read. Please try again."; server error "Something went wrong on the server. Please try again later." (UP-10, F-09, F-11).
- **Refused uploads count (OQ-41):** toward the 60 per hour (NFR Security, F-09).
- **IPv6 (OQ-42):** "per IP address" means per /64 prefix (NFR Security, F-09).
- **20 MB (OQ-43):** 20,000,000 bytes (UP-1, NFR Security, F-09, F-11).
- **Connection drop after storing (OQ-45):** the draft stays until it expires (AC-1, F-09).
- **"At upload" (UP-7, SH-8):** met by the draft edit page that opens right after upload; the upload request carries only the file (UP-7, SH-8, technical design Publish 1, F-09).

**Decided on 24 Oct 2026** (F-12 QA Q1 and the F-13 design questions OQ-61 to OQ-64):

- **Owner API text rules (F-12 QA r1 Q1, limits in the spec):** a tab in the description stays refused; line feed is the only control character allowed, as built. The title limit (100 characters after trimming leading and trailing white space) and the description limit (1,000 characters), counted in Unicode code points, are now in UP-4, with the control-character rule. The edit panel must not send a tab (F-19). Updated: UP-4, F-19 (scope).
- **Activity page (F-13 design OQ-61 to OQ-64, and the two breaks with the technical design):** (OQ-61) yes: the summary shows maximums for heart rate, power, cadence and temperature wherever the summary has them, as a second value in the same cell as the average. (OQ-62) the expiry is shown as "Expires in …", relative, so it depends on no time zone, computed in the browser from the UTC `expires_at`; a date, wherever one is shown, uses the browser's time-zone preference; with JavaScript off the static page cannot know "now", so the markup carries the expiry date as the fallback, in the workout's own time zone or in UTC when the workout has none, marked as an estimate ("Expires around 30 Oct 2026"; refined the same day, replacing a plain UTC date). Replaces the design's suggestion (the workout's own zone) for the JavaScript view; the fallback keeps it. (OQ-63) the engineer proposes the page's own texts, and the user reviews them once they are visible, as OQ-58 did. (OQ-64) the Lighthouse check waits until the user has a machine with Chromium; F-13 AC 7 stays as it is and is ticked later. Both breaks with the technical design are accepted: **no inline critical CSS** (one stylesheet from the site, no inline CSS or script) and **no Svelte hydration** (server markup adopted, Svelte only in empty slots; ADR-026). Updated: AP-2, UP-7, the spec's module list (header), F-13 (requirements line, scope, AC 1), F-18 (scope), F-19 (scope: expiry date in the browser's zone); technical design (Front end, Load order step 1; Published language; Repository layout; Dependency rules; Open technical questions, template row; Glossary: "Prototype page" replaced by "Page view model" and "Sample page / sample view model"). The F-13 design D-7 (expiry in the workout's zone) and D-4 (max values) follow in the architect's next revision.
- **F-13 page texts (OQ-63, follow-up):** the page texts the engineer proposed, as listed in the F-13 architecture document and the code, are accepted for now; review again when needed. No document changed.
- **Pool length and lengths in the summary (new request, for later):** pool swims show the pool length and the number of lengths swum in the summary. Added as AP-16 (Could, phase 2, provisional), next to AP-13 in the phase 2 list; no feature file, as phase 2 is not broken down, and no MVP feature (F-13, F-14, F-18) changes. Assumed, open in the spec: the device's session values (active lengths, rest lengths left out), labelled "lengths"; the pool length in the pool's own unit; hidden when not recorded, never computed. **Needs a workout data file format change** (backend and frontend): the session `pool_length`, `pool_length_unit` and `num_active_lengths` are read by neither F-02 normalisation nor the data file today (ADR-005 metadata is a closed schema; a new field bumps the version, item 9). Updated: spec (viewer story 9, scope table, AP-16, Open questions, Roadmap phase 2), Phase 2 list.
- **Activity page layout (new request, built as an F-13 follow-up before F-14):** on wide screens (desktop) the page has two columns: the map (or the no-location hint) on the left; the summary with the privacy notice, the charts, the range panel and the laps on the right; the header on top across the full width. The switch width is left to the design, as the spec sets no breakpoints elsewhere. On a phone (one column), a toggle button on the map keeps the map at the top of the screen while scrolling; off by default, smaller while kept at the top, not stored (like the units). Assumed: "smaller" means at most 40 % of the screen height; the toggle survives the fullscreen map (AP-15). Added as AP-17 and AP-18 (Must). Updated: spec (viewer stories 10 and 11, scope table, module list intro, AP-17, AP-18), F-13 (requirements line, scope, AC 11 and 12), F-26 (requirements line, scope), requirement coverage. F-17 and F-18 have no conflicting layout wording. Follow-up the same day (F-13 design v1.1, D-17 to D-20, OQ-65): the pinned map is at most 40 % of the screen height; an indoor workout, or one with no visible route, uses one column at every width (replacing the hint in the left column); on desktop the map stays in view (sticky) while the right column scrolls; on the phone the pinned map stays pinned until the end of the laps table; leaving fullscreen keeps the pinned state; the edit page (F-19) uses the same layout and toggle. Two columns start at 1024 px when there is a map. Updated: spec (module list intro, scope table, AP-17, AP-18), F-13 (scope, AC 7, 8, 11, 12), F-19 (requirements line, scope, AC), requirement coverage, technical design glossary ("Pinned map"). Later the same day, from trying the build (F-13 D-10, D-17 to D-19): on desktop the header and footer sit in the right column, the map is sticky, full window height and flush with the edges, the columns are 1:1, and the map credits sit at the top; on the phone the pinned map is 35 % of the screen height (a fixed small height on short landscape screens), the button reads "Pin map" / "Unpin map" (this drops the rule that the label avoids the word "pin"), the map credits are shown for 5 s or until the first touch on the map, then collapse to the "ⓘ" button, and the cursor readout is a card. QA-13-r2-2: the pinned map is not capped and need not be smaller than the unpinned map (on a tablet it may be larger; the user tries it first); the 40 % limit stays. Updated: spec (AP-17, AP-18), F-13 (scope, AC 11, 12).
- **Units, pace and theme (F-14 design OQ-66 to OQ-68, the suggestions, decided 24 Oct 2026):** (OQ-66) the pace sports (running, walking, hiking, swimming, rowing) show pace in the chart and the cursor readout too, with the axis flipped so faster is up and stops clipped at a slowest pace; the frontend engineer proposes that value and the user reviews it on screen. (OQ-67) swim distance is in kilometres or miles in the MVP, like every sport; yards come later with pool lengths (AP-13, AP-16, phase 2). (OQ-68) the upload page gets the same CSS-only light and dark theme in F-14; the About page gets it in F-29. Updated: DS-2, DS-4, F-14 (scope, out of scope, ACs), F-29 (requirements, scope, AC), coverage row DS-4, technical design (dependency rules, glossary "Units store"). Follow-ups, same day: imperial swims show miles and pace per 100 yd in the MVP, aligned when yards come with AP-13/AP-16; the generic "expired or not available" page gets the same CSS-only theme in F-15; route colouring by speed shows pace for the pace sports. Updated: DS-2, DS-4, AP-9, F-15 (depends on, requirements, scope, AC), F-25 (scope, AC), coverage rows DS-2, DS-4. Follow-up, 25 Oct 2026 (changes OQ-66): in the pace chart, stops (speed at or near 0) are gaps in the line and the cursor readout shows "–"; only moving stretches slower than the slowest pace are clipped to it. Changed again, 25 Oct 2026: no fixed per-sport slowest pace; the pace axis range comes from the workout's own data, slow outliers clip to the axis edge, stops stay gaps. Updated: DS-2, F-14 (scope, pace AC).

**Decided on 25 Oct 2026** (F-15 design questions OQ-69 and OQ-70):

- **Nothing private under `public/` (OQ-69):** the "same bytes as `draft.bin`" check was a stand-in for "nothing private leaks"; with hiding off, the public data file may equal `draft.bin`. Replaced by two checks: (a) no file under `public/` has the bytes of `original.fit`, after publish and after a regeneration (F-15); (b) with hiding on, no public file contains hidden data (F-16). Updated: F-15 (AC 10), F-16 (AC 1). F-12 AC 6 and the spec's NFR Privacy did not carry the check; unchanged.
- **Caddy checks for F-15 (OQ-70):** QA downloads a Caddy release binary locally (never committed) and runs it once with `deploy/Caddyfile` to tick F-15 AC 1, 5 and 6; automated Caddy checks stay in F-22. Updated: F-15 (note under the ACs).

**Decided on 26 Oct 2026** (F-16 design questions OQ-15 and OQ-71):

- **Fully hidden route (OQ-15):** when hiding removes every position of a workout that has location data, the public page says so in place of the map: "The entire route has been hidden by the uploader." Indoor workouts keep "No location data in this workout." This knowingly reveals hiding for a fully hidden route only; SH-9 notes the exception. The sharer still gets no warning. The wording names the uploader so viewers know the route was hidden on purpose, not lost by a fault (user, 26 Oct). A route that comes back within 300 m of the start later in the workout stays visible there; accepted, privacy zones (SH-3, phase 2) cover it. Updated: AP-3, SH-9, spec module list; F-16 (scope, AC 5, new ACs), F-06, F-13, F-19, F-23, F-25, F-26.
- **Bad first or last fix (F-16 QA-16-r1, decided 26 Oct 2026):** a stale or coarse first or last GPS fix more than 300 m from the real start or end moves the SH-4 circle, so the real start or end can stay public. Risk accepted for the MVP; revisit if a real recording shows it. No document changed.
- **How the 300 m is measured (OQ-71):** the start section runs until the track first leaves a 300 m circle (straight line) around the first position; the end section likewise around the last. A route that stays near its start or end is hidden more, or entirely. Updated: SH-4; F-16 (goal, scope, AC 1, AC 6, new ACs), F-19; assumption 14 below.
- **Range panel placement and clearing (OQ-72 to OQ-74, F-17 design, decided 26 Oct 2026):** the panel is hidden on every device until a range is selected; a one-line hint under the charts says how to select (drag; on touch, long-press then drag). In the one-column layout (phones) the panel sticks to the bottom of the screen while a range is selected. Only a tap on a chart outside the shaded part clears the range (besides the panel's clear control); taps on the map or elsewhere, and scrolls, never clear. With the design: range moving time and gain come from cumulative streams the server writes in data file format version 2 (ADR-029), so F-17 is owned by both, backend first. Updated: spec (module list, AP-7, AP-17, AP-18); F-17 (owner, requirements line, scope, ACs); technical design (stream table, size, range-stats note, glossary "Range", "Selection store"). F-13 unchanged.

**Decided on 29 Oct 2026** (viewer preferences):

- **Units remembered on the device (DS-1, F-14):** viewer preferences are remembered in the viewer's browser in `localStorage`, never in a cookie, and the server never sees them. The units choice is stored when the viewer uses the drop-down; it applies to every workout and edit page in that browser and wins over the browser-language default; broken or unavailable storage falls back silently to the default. Replaces "the choice is not stored" (DS-1, and "like the units" in the 24 Oct layout entry). The other viewer settings are decided in the next entry. Updated: spec (viewer story 7, DS-1, AP-18 wording, INF-1, NFR Privacy "no data outlives its workout"), F-14 (requirements line, scope, ACs), F-29 (scope, AC), technical design (Front end implementation notes).
- **Other viewer settings (same day, the suggestions):** the x-axis choice (AP-5) and the map/content splitter (AP-17) are remembered in `localStorage` like the units. A workout without distance shows time without overwriting the stored axis. The splitter, built as an F-17 follow-up on 28 Oct 2026, is now in AP-17: both two-column layouts, 30–70 %, on desktop also limited by the column minimums, double click or double tap restores the default and clears the stored split. Assumed: the split is stored separately for wide and short landscape screens, as their defaults differ. Not stored: the pinned map (AP-18) and the route colouring metric (AP-9, question 27). ~~Theme (DS-4) follows the OS, nothing to store.~~ Superseded 3 Nov 2026 (theme drop-down, entry below). Pin legend visibility (PIN-6) is decided with the phase-2 pin details. Updated: spec (AP-5, AP-17, INF-1), F-17 (requirements line, scope, ACs), F-29 (scope, AC), coverage row AP-17.
- **Layout jump with a remembered split (AP-17, 29 Oct 2026):** when a stored map/content split differs from the default, the layout may jump while the page loads, also on a cached reload; accepted for now. Possible later fix: an inline head script allowed by a CSP hash.

**Decided on 27 Oct 2026** (F-17 layout follow-up):

- **Range panel on desktop:** in two columns from 1024 px the panel sits in the page above the laps and splits table, not sticky; the laps move down when it appears (accepted). F-18 checks that the browser keeps a tapped lap row in place.
- **Phones in landscape:** a short landscape screen (under about 500 px tall) uses two columns when there is a map, split about 40 % map, 60 % content: map sticky on the left, range panel sticky at the bottom of the right column, no pin toggle. Phones in portrait keep one column with the pin toggle; desktop rules unchanged. The fixed small pinned-map height for short landscape screens is dropped (no toggle there). Built with F-17. Updated: spec (viewer story 11, scope table rows AP-17 and AP-18, module list intro, AP-7, AP-17, AP-18); F-13 (requirements line, scope, AC 11 to 13); F-17 (scope, ACs); F-18 (scope note).

**Decided on 29 Oct 2026** (F-18 design questions OQ-75 to OQ-80, every suggestion accepted):

- **Tapped row stays in place (OQ-75):** when a lap or split row tap makes the range panel appear, the tapped row stays in place on screen, in all layouts; this replaces "check the browser's scroll anchoring and report" (27 Oct 2026). A chart drag may still move the table down.
- **Laps or splits not remembered (OQ-76):** every page starts on laps.
- **No device laps (OQ-77):** with distance but no device laps the laps section is shown with "No laps recorded." and the switch offers splits; without laps and distance there is no section.
- **Transition laps (OQ-78):** a lap that starts in a transition gets no computed speed, cadence or power; it shows "–", as in the summary (AP-2, AP-4).
- **Panel vs lap row (OQ-79):** after a lap tap the panel's range values may differ from the device's row values; no note, as OQ-24.
- **Panel on a phone in portrait (OQ-80):** after a row tap the panel may be above the screen, as AP-7 places it; accepted.

Updated: spec (AP-7, AP-8); F-18 (owner both, backend first; scope; ACs).

**Decided on 29 Oct 2026** (F-19 design questions OQ-81 and OQ-82, both suggestions accepted):

- **Edit panel place (OQ-81):** at the top of the content column, above the header, always open: above everything in one column, at the top of the right column in two.
- **Saving (OQ-82):** one Save button with a clear saved or error state; unsaved changes are lost on leaving the page, with no warning.
- **Copy link on a draft (F-19):** the edit page hides "Copy link" while the workout is a draft; it appears once F-20's Share publishes the workout.
- **Stale lifetime option (QA-19-r1-2, accepted):** the lifetime options are computed once when the page opens; an option that became stale in a long-open tab is refused by the server (400). No fix.

Updated: F-19 (owner both, backend first, for ADR-030's stored to-be-hidden ranges); technical design (Data model, Repository layout, Dependency rules).

**Decided on 30 Oct 2026** (F-20 design questions OQ-83 to OQ-85, every suggestion accepted):

- ~~**Share while private (OQ-83):** Share and the header Copy link are hidden while the workout is private, as on a draft.~~ Superseded 30 Oct 2026 (private removed): hidden only on a draft.
- ~~**First-open notice (OQ-84):** no close button; it stays for the whole visit and never shows again after a reload or later open.~~ Superseded the same day by the published page (below).
- **Publish with unsaved changes (OQ-85):** Publish saves the changes first and publishes only if the save succeeds; otherwise it shows the error and does not publish.
- **Publish server error (F-20, accepted edge case):** if publish fails with a server error, a reload may show the workout as published with no page until the next saved change or the next deploy. No fix.

**Decided on 30 Oct 2026** (user, edit link after upload and abandoned drafts; replaces the first-open notice, OQ-84 superseded):

- **Draft shows no links:** upload opens `/edit/{public_id}#{token}` with settings and draft preview; no viewing link, no edit-link notice (UP-4, AC-1).
- **Published page (AC-6):** only a successful publish (unsaved changes saved first, OQ-85) opens `/published/{public_id}#{token}`, token in the fragment only. It shows the viewing link and the edit link, each with Copy, a big warning that the edit link is the only way to change or delete the workout and cannot be recovered, and "Back to edit". A reload shows it again; it can be opened later whenever the workout is not a draft; on a draft it sends the owner to the edit page.
- **Edit page Share:** stays after publishing (OQ-83; revised below).
- **Abandoned drafts (UP-7, SH-2):** a workout never published is deleted 48 hours after upload (hours, not calendar days), or at the end of its lifetime if earlier, by the expiry sweep (F-21), only to free space early. Once published, the chosen lifetime counts from upload, unchanged.
- **`&new` marker:** no longer needed (architecture hand-off: ADR-020 rev. 1, F-11, F-19 E-3, F-20 C-5).
- ~~**Open:** the published page while private (spec Open questions; assumed: viewing link hidden, edit link shown).~~ Superseded 30 Oct 2026: no private state.

Updated: spec (UP-1, UP-4, UP-7, SH-2, SH-6, AC-1, new AC-6, scope table, open questions); technical design (Publish 4, 6, 7; Lifetime; glossary); F-11, F-19, F-20, F-21, F-22.

**Decided on 30 Oct 2026** (user, private state removed; supersedes assumption 19 and the private parts of OQ-83):

- **Two states (SH-2):** draft or published (unlisted). Publishing cannot be undone; the owner can still change settings and delete. No visibility control on the edit page.
- **Share and Copy link (OQ-83 revised):** hidden only on a draft.
- **Published page (AC-6):** shown whenever the workout is published; on a draft it sends the owner to the edit page. The open question about it while private is dropped.
- **Draft line (UP-7, F-20; main session, suggestion accepted pending user objection):** on a draft the edit page shows "Unpublished drafts are deleted 48 hours after upload."
- **Open:** confirm before publishing, now that it cannot be undone (spec Open questions; assumed: no confirmation).
- **Built behaviour to remove (architect hand-off):** `private` in the schema (F-09, ADR-019), the owner `PATCH` visibility field (F-12 O-6), the unlisted ↔ private path (F-15, ADR-028), the visibility control (F-19), private handling of Share and the published page (F-20 C-3, C-4, OQ-83).

Updated: spec (stories 4, scope table, UP-4, UP-7, SH-2, SH-5, SH-6, SH-10, AC-5, AC-6, open questions); technical design (Publish 4, Data model, Security and privacy, Open technical questions, glossary); F-09, F-12, F-15, F-19, F-20, F-21, F-22, F-23, F-27, F-32.

**Decided on 30 Oct 2026** (user, F-20 design OQ-86 and the F-21 hand-off, suggestions accepted):

- **Show links (OQ-86, AC-6):** once published, the edit page's Share block has a "Show links" button that opens the published page (`/published/{public_id}#{token}`).
- **Overdue draft (F-21):** the owner API treats a draft past `created_at` + 48 h that the sweep has not yet removed as not found, like an expired workout.

Updated: F-20 (scope, AC), F-21 (AC), F-20 design (OQ-86).

**Decided on 31 Oct 2026** (user, F-21 design OQ-87 and F-31 deployment):

- **Expiry while Go is down (OQ-87, SH-10):** accepted as suggested. While the Go server is down, Caddy may keep serving published pages whose lifetime ended meanwhile; SH-10's 60 s holds only while Go runs. On restart the overdue workout is deleted before Go serves again. Updated: F-21 (AC 6), F-21 design (OQ-87).
- **Deployment (F-31):** Go and Caddy run together in one Docker setup (e.g. compose), so they start and stop together. The Go + Caddy split may be reworked later. Replaces "Go binary under systemd" in F-31 and the technical design (Hosting and operations); that text is flagged for the architect. Updated: F-31 (scope).
- **Remembered map split (OQ-88):** not now. The accepted layout jump stays; no inline `<head>` script in F-22 (ADR-032 item 3 keeps the later fix to one script and one hash). Updated: F-22 design (H-4).
- **Not in search engines (OQ-89):** `X-Robots-Tag: noindex, nofollow, noarchive` on every response of the site, in F-22; F-23 keeps the robots meta and `robots.txt`. Updated: F-22 (scope, AC), F-23 (scope, AC), F-22 design (H-8), technical design (Headers).
- **Caddy in `make ci` (OQ-90):** the F-22 check needs a pinned, downloaded Caddy binary; without it the check fails, not skips. Updated: F-22 design (H-12).
- **Warning card on edit links (SH-11):** `/edit/{id}` and `/published/{id}` carry fixed, workout-independent OG/Twitter tags, "Edit link: do not share" plus one short sentence, optionally a static warning image, so messengers warn when an edit link is pasted. The token stays in the fragment; the tags reveal nothing about the workout. Updated: spec (SH-5, new SH-11, scope table), F-23 (requirements, scope, AC); technical design (Indexing and preview cards) flagged for the architect.
- **Preview card look and name length (OQ-91):** resolved with changes. The card shows a sport icon (simple drawn icons: run, ride, swim, walk/hike, generic) in front of the name; below it distance and duration (moving time, elapsed without GPS); elevation gain only when altitude was recorded and gain > 0; pace for run/walk/hike (min/km), speed for rides (km/h), pace per 100 m for swims; average heart rate when present. Name cut at 40 characters stays. Updated: spec (SH-5), F-23 (scope, AC), F-23 design (PC-3, PC-4, OQ-91).
- **Undrawable name characters (OQ-92):** accepted as suggested. Characters the Go fonts cannot draw are dropped in the image only; if nothing drawable remains, the image shows the default title (sport and date). `og:title` keeps the full name. Updated: F-23 design (PC-4).
- **F-23 hand-offs (design):** no `robots.txt`; edit-link card has no image (PC-9); new required setting `WORKOUT_EXPLORER_BASE_URL`; owner both, backend first. Updated: F-23 (owner, scope, AC 5), technical design (Tech stack, Configuration, Indexing and preview cards).

**Decided on 31 Oct 2026, later** (user, build order and deployment):

- **Build order:** F-31 first, then F-24 to F-30 by number. F-31 depends only on F-22; F-30 now depends on F-31 too, as it adds a deploy step.
- **Deploy without `regenerate`:** F-31 ships without the `workout-explorer regenerate` step; F-30 adds it. Until then a deploy keeps old bundles, and old workouts may break (accepted, OQ-53: no compatibility promise).
- **Launch condition:** the first deployment is for the operator's own testing (messenger preview checks, F-23 AC 8 and 9). Links are shared with friends only after F-29 (privacy policy, attributions). Not a technical block.
- **Docker Compose (refines the Deployment entry above):** Go server and Caddy in one Docker Compose setup, started and stopped together, with Docker restart policies instead of systemd units; Caddy version from `deploy/caddy.env` (pinned 2.11.7); settings from one env file read by Compose. A brief outage when a deploy changes the Caddy version is accepted. The site name comes from that env file: `WORKOUT_EXPLORER_BASE_URL` and Caddy's site address (`WX_SITE`) are set together and must match.

Updated: F-30 (depends on, scope, out of scope, AC), F-31 (depends on, scope, out of scope, AC), technical design (Repository layout, Configuration, Hosting and operations: Server, Deploys, Logs), feature index (order, F-30 and F-31 rows).

**Decided on 1 Nov 2026** (user, F-31 design v0.2 OQ-93, OQ-94 and client addresses):

- **Caddy restart on deploy (OQ-93):** a brief page outage whenever a deploy recreates Caddy (version, service definition or settings change) is accepted.
- **Uptime monitoring (OQ-94):** not part of F-31 for now. The operator runs their own monitor (e.g. Uptime Kuma) later; `/api/health` and `/` stay suitable targets.
- **Upload limit per IP:** becomes a server setting, default 60 per hour (NFR Security, ADR-021 item 7), so an operator who cannot change Docker's daemon settings can raise it.
- **Docker daemon settings:** *Superseded by the F-31 design v0.3 hand-offs below (DP-1d): no runbook duty.* F-31's operator README (`deploy/RUNBOOK.md`) tells the operator to set `"userland-proxy": false` and keep `ip6tables` on. Without them, visitors arriving through `docker-proxy` (at least all IPv6 visitors) appear under one address and share one upload limit; an operator who cannot set them raises the per-IP limit instead.
- **Open (later, not F-31):** how the operator learns about new reports (INF-2). Until decided, the operator checks the `reports` table (technical design, Reports).

Updated: spec (NFR Security), F-31 (goal, scope, out of scope, AC), technical design (Configuration, Hosting and operations: Deploys, Monitoring; Repository layout; Open technical questions).

**Decided on 1 Nov 2026, later** (user, F-31 deployment mode and client addresses; refines the entries above):

- **Deployment mode:** only "behind an existing reverse proxy" for now. Caddy stays the file server and header layer (ADR-032 headers, caching, `.br`, published pages served while Go is down) but listens on a plain HTTP port only; the operator's own reverse proxy does TLS and forwards to it. No ACME, no ports 80/443 in our setup. A standalone mode (Caddy with TLS on 80/443) may come later; out of scope now.
- **Client address:** Caddy trusts the forwarded client address only from the operator's proxy (a setting); Go keeps trusting only Caddy. The Docker daemon duty above (`"userland-proxy": false`, `ip6tables` on) is dropped (DP-1d).
- **Upload limit per IP, split:** two settings replace `WORKOUT_EXPLORER_UPLOAD_RATE_LIMIT`: one for IPv4, counted per address, and one for IPv6, counted per /64 prefix. Default 60 per hour each unless the F-31 design sets otherwise.
- **Operator notes:** the Docker and proxy notes go in `deploy/RUNBOOK.md`, linked from the repository's `README.md`.
- **Open:** whether the report form's per-IP limit (INF-2, F-27) follows the two upload settings or stays fixed (question 21's 60 per hour).

Updated: spec (NFR Security), F-31 (scope, out of scope, AC), technical design (Tech stack, Configuration, Hosting and operations: TLS and static files, Uploads).

**Decided on 1 Nov 2026, F-31 design v0.3** (user; closes the open points above):

- **Proxy location:** the operator's reverse proxy runs on the same server (host process or container); a proxy on another machine is not supported for now.
- **Report limit (INF-2, F-27):** stays fixed at 60 reports per hour per IP address, not a setting and not tied to the upload settings. Closes the open point above and question 21.
- **How the operator learns about reports (INF-2), later idea, not in any feature's scope:** configurable HTTP callbacks (e.g. Pushover) notify the operator of new reports. Until then the operator checks the `reports` table.
- **F-31 design v0.3 hand-offs:** the upload limit settings are `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4` (per address) and `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV6` (per /64), default 60, range 1–240 (DP-1c). The site name lives only in `WORKOUT_EXPLORER_BASE_URL`; there is no Caddy site address, so the 31 Oct 2026 "must match" holds with one key (DP-6). Docker's `"userland-proxy"` and `ip6tables` settings are not needed (DP-1d).

Updated: F-31 (scope), technical design (Configuration, Hosting and operations: Reports).

**Decided on 1 Nov 2026, Dockerfile** (user, F-31):

- **Self-built image:** the repository gets a multi-stage Dockerfile (Go server and web build) so self-hosters can build the app image themselves and use it with the Compose setup (`docker compose build`).
- **Nothing is published:** no image registry, no CI push. *Refined 2 Nov 2026: operator binaries come from GitHub Releases (below).*
- **Operator's deploy (settled by the architect, F-31 design v0.4):** the operator keeps release binaries (2 Nov 2026 entry; DP-18) mounted read-only on a pinned base image (DP-2); self-hosters use `deploy/Dockerfile` with `deploy/compose.build.yaml` (DP-16; ADR-034, ADR-035).

Updated: F-31 (scope, out of scope, AC), technical design (Repository layout, Hosting and operations: Deploys).

**Decided on 1 Nov 2026, F-31 design v0.4** (user, OQ-95, OQ-96):

- **Image platform (OQ-95):** the self-hoster image is linux/amd64 only until a self-hoster asks for another.
- **Self-hoster upgrade (OQ-96):** pages published before an upgrade lose their styles and scripts until F-30's `regenerate` runs; accepted, no compatibility promise (OQ-53).

Updated: nothing else; the F-31 design status line (DP-16 open on OQ-95, OQ-96) is for the architect to close.

**Decided on 2 Nov 2026, operator deploy from GitHub Releases** (user, F-31; refines "Nothing is published" of 1 Nov 2026):

- **Releases instead of CI run artefacts:** pushing a version tag (`vX.Y.Z`) runs CI and, only when it is green, creates a GitHub Release with the binary and web build attached. `deploy.sh` deploys a given release tag; there is no 14-day expiry, and the deployed version is the tag.
- **Visibility:** ~~the repository is private, so releases are visible only to collaborators.~~ Superseded 2 Nov 2026: the repository is public; releases are public downloads (entry below).
- **Still not published:** no public registry or image; CI holds no server credentials. Self-hosters still build the image themselves (`deploy/Dockerfile`).
- **Left to the architect:** workflow layout, permissions, version string.

Updated: F-31 (scope, out of scope, AC 1), technical design (Hosting and operations: Deploys). ADR-035 and the F-31 design (DP-2) follow in the architect's next revision.

**Decided on 2 Nov 2026, CI triggers** (user, F-01):

- **GitHub CI runs only on `v*` tag pushes and on manual runs (`workflow_dispatch`)**, not on every push. Local `make ci` is the gate before each commit.

Updated: F-01 (AC 2), technical design (Build and CI).

**Decided on 2 Nov 2026, public repository** (user, F-31; supersedes "Visibility" above):

- **The repository becomes public, licensed AGPL-3.0.**
- **Releases are public downloads.** Self-hosters either build the image (`deploy/Dockerfile`, `docker compose build`) or download a release.

Updated: F-31 (scope), technical design (Hosting and operations: Deploys).

**Recorded on 2 Nov 2026, shareable range** (new request from the user, phase 2; not yet decided in detail):

- **Range in the link (AP-19, Could, phase 2):** the range selected on the viewing page is kept in the link's fragment, so a shared link opens with it selected. Fixed by the request: fragment only (never sent to the server or logged, works with cached static pages); viewing page only, not the edit or published page with their token fragment; time positions, never coordinates; cut to the published data, revealing nothing a viewer could not select; invalid values ignored silently.
- **Open, with suggestions in the spec's open question "Range in the link":** elapsed seconds rather than sample numbers; live link update plus the copy-link button (SH-6) rather than a separate "copy link to this selection" button; cursor position and x axis not included; no scroll or map zoom on opening.

Updated: spec (viewer story 12, scope table, AP-19, Open questions, Roadmap phase 2), Phase 2 list (with acceptance criteria). No MVP feature changes. Technical design: link formats (ADR-020) gain the viewing-page fragment once phase 2 is designed.

**Recorded on 2 Nov 2026, summary colours** (new request from the user, phase 2; not yet decided in detail):

- **Summary colours (DS-5, Could, phase 2):** each summary cell whose metric has a chart line gets a background tint, a muted version of that line's colour; cells without a chart line (distance, times, calories) stay plain. Fixed by the request: follows the light and dark theme; text contrast kept (at least 4.5:1, an exception to accessibility not being a goal, NFR Accessibility).
- **Open, with suggestions in the spec's open question "Summary colours":** one tint strength for all metrics, reviewed on screen; summary strip only, not range panel or laps; chart line colours changed only where contrast requires it.

Updated: spec (viewer story 13, scope table, DS-5, Open questions, Roadmap phase 2), Phase 2 list (with acceptance criteria). No MVP feature changes.

**Decided on 2 Nov 2026, viewer download deferred** (user, F-24):

- **The viewer download (SH-8 viewer part, F-24) moves to phase 3 (Later).** The owner's download of the original stays in the MVP (F-12, F-20). The F-24 feature file, design and ADR-036 are kept as written for phase 3.
- **Build order:** F-24 leaves phase 1 and is listed last; F-30 no longer depends on it; F-32 covers F-08 to F-31 except F-24. The MVP cut order for Should items no longer names SH-8.

Updated: spec (scope table, SH-8, Should cut order), technical design (Publish 4 and 5, public files, Publishing module, glossary: export marked phase 3), F-24 (phase), F-30 and F-32 (depends on), feature index (order, F-24 row, counts, coverage, Phase 2 list).

**Decided on 3 Nov 2026, theme drop-down** (user, F-14):

- **Theme drop-down on the activity and edit pages:** in the header's control row, next to the units drop-down and Copy link; a small drop-down like the units one with System, Light and Dark; System is the default.
- **Remembered** in `localStorage` like the units choice and applied before first paint, so the page does not flash.
- **Upload, About and generic "expired" pages** stay CSS-only and follow the system setting.

Updated: spec (DS-4), F-14 (requirements line, goal, scope, ACs), "Other viewer settings" entry (theme now stored). Technical design unchanged (it does not tie the theme to the system setting).

**Recorded on 3 Nov 2026, split distance** (new request from the user, phase 3; not yet decided in detail):

- **Split distance (AP-20, Could, phase 3):** the splits view (AP-8, F-18) gets a drop-down of preset split distances per sport, with today's default (1 km or 1 mi) preselected. In the user's words: "We currently have splits but no way to choose the 'chunk size'. The defaults are fine but I think users will want to set them explicitly (based on a combobox with preselected distances for each sport)."
- **Open, with suggestions in the spec's open question "Split distance":** the viewer chooses per visit, no sharer default; not remembered; presets per sport in the current unit system, back to the default on a units switch; no server needed, since splits are computed in the browser (F-18 L-4).

Updated: spec (viewer story 14, scope table, AP-20, Open questions, Roadmap phase 3). No feature file (Later items are not broken down). No MVP feature changes.

**Decided on 3 Nov 2026, fullscreen map keeps route colouring** (user, OQ-99, F-26):

- **In fullscreen (AP-15), the route colour drop-down and its legend stay visible** while colouring is available, since a coloured route cannot be read without its legend.

Updated: architect/INDEX.md (OQ-99 resolved); F-26 design to follow.

**Decided on 3 Nov 2026, fullscreen bars** (user, OQ-100, F-26):

- **Address and system bars stay visible in fullscreen** on Android Chrome and desktop (CSS overlay, ADR-038); accepted. The browser Fullscreen API may be added later as an extra layer.

**Decided on 3 Nov 2026, report retention and notifications** (user, F-27):

- **Reports are kept for 30 days**, so they survive the operator's long holiday, then deleted automatically.
- **Pushover notifications on a new report are in the MVP** (replaces "HTTP callbacks later" for Pushover; other callback targets stay later). The application token and user key are set by env; unset means no notifications. This is an operator-only, opt-in exception to "no external services"; the app container gets outbound access only for it (F-27 design).

**Decided on 3 Nov 2026, F-27 open questions** (user, OQ-101, OQ-102):

- **No auto-hiding of reported workouts.** There is no general fixed EU takedown deadline for reports (DSA: timely handling, expeditious removal of clearly illegal content); the operator acts on the Pushover notification with the delete command. Revisit if a fixed deadline applies.
- **(OQ-101)** An 11th report on one workout is answered `204` and not stored.
- **(OQ-102)** Reports never outlive their workout; the 30-day rule is a ceiling (no data outlives its workout, INF-2).
- **Notification content:** report number and reason only, no link or public id.

**Decided on 3 Nov 2026, report form** (user, OQ-103, OQ-104, F-28):

- **(OQ-103)** Without JavaScript the Report button is not shown, with no "needs JavaScript" note (INF-2's "on every workout page" holds with JavaScript).
- **(OQ-104)** The dialog texts are the F-28 design's RF-8 set; the user reviews them on screen.

**Decided on 4 Nov 2026, About page** (user, OQ-105, OQ-106, F-29):

- **(OQ-105)** The theme choice is listed on the About page among what the viewer's browser keeps.
- **(OQ-106)** FE-1 drafts the privacy text from F-29 PT-1 to PT-17; the user edits it on screen before launch. (a) Legal basis: GDPR Art. 6(1)(b) for workouts and settings, Art. 6(1)(f) for reports, rate limiting, logs and map tiles. (b) withdrawn: the page makes no access-log claim; the proxy's logging is stated in the operator file. (c) Hosting provider as a category: "a hosting provider in an EU data centre". (d) The general right to complain to a supervisory authority, no named authority.
- **Page split:** the About page has an app part (committed template, true for every deployment) and an operator part from a file supplied per deployment (`WORKOUT_EXPLORER_OPERATOR_FILE`); the repository ships only `deploy/operator.example.txt`. The operator name and contact settings are dropped.
- **(OQ-5)** The FIT Protocol licence gets an entry in the attribution list.

Updated: spec (INF-1), technical design (Hosting: About page; Configuration: `WORKOUT_EXPLORER_OPERATOR_FILE`; Repository layout: `about.html`, `tools/attributions`, `deploy/operator.example.txt`), F-29 (scope, ACs), F-32 (AC), F-29 design (Status, PT-14, open questions), architect index.

**Decided on 4 Nov 2026, regenerate partial states** (user, F-30):

- **(a)** A workout whose rebuild fails after its first write stays partly on the new version until the next run finishes it. F-30 AC 6 "kept unchanged" holds only when the first write fails.
- **(b)** Viewers with a page open while old bundles are removed lose lazily loaded parts until they reload.

Updated: F-30 (AC 6, open points), technical design (code tree, Binary row, settings' "Read by").

**Decided on 4 Nov 2026, fullscreen readout and iOS Shortcut** (user):

- **Cursor readout in fullscreen (AP-21, F-33):** the cursor readout stays visible in the fullscreen map as a compact strip at the bottom, on phone and desktop, hidden without a cursor. The range panel stays hidden in fullscreen.
- **iOS Shortcut link (INF-5, F-34):** an optional https setting links the hoster's own Shortcut from the About page ("Upload from your iPhone"); unset means no section, a non-https value refuses startup (exit 2). The Shortcut uses the existing upload API and opens the edit link only. Build instructions go in the documentation for every hoster. Not the Later "mobile share sheet" (a share target built into the app).
- **Build order:** F-33 before F-32; F-34 after F-32 (phase 2).
- **Priorities:** AP-21 is Must. INF-5 is nice to have: Could, so phase 2 in the scope table; F-34 keeps its detailed file. The Should cut order (question 29) is unchanged.

Updated: spec (stories, scope table, AP-15, AP-21 Must, INF-5 Could and phase 2), F-33, F-34 (phase 2), F-32 (depends on), feature index (order, counts, coverage, Should list).

**Decided on 5 Nov 2026, expiry check local only** (user, F-32 AC 8, from `qa/F-32-mvp-launch-checks-r1.md`):

- **Expiry (UP-7, SH-10):** checked locally only; QA r1's local measurement (expiry 7.6 s, worst case about 30 s by the expiry interval; draft past 48 h 29.1 s) and `make caddy-check`'s expiry test stand in for the live check. A live check needs at least a one-day wait, and the runbook forbids editing the database by hand. Delete and hiding-on stay live checks.

Updated: F-32 (AC 8).

**Recorded on 6 Nov 2026, compare two workouts** (user; requested by a friend of the operator; phase 3 idea, picked up after gate 1; no feature file yet):

- **Compare two workouts (Later):** show two workouts together on charts and map, aligned by distance or by time, e.g. start vs end of season.
- **Conflicts with the maximum lifetime of 30 days (UP-7):** the season case is not possible with two live links. Longer lifetimes are not wanted (store as little as possible).
- **Variant "two live viewing links":** client only, loads both public data files, no new server storage; hidden ends stay protected because only public data is used (SH-4); limited to workouts at most 30 days apart.
- **Variant "live workout vs a FIT file opened locally in the browser, never uploaded":** covers the season case without storage, but needs FIT parsing in the browser (cost).
- **Bundle budget is near its limit:** compare would be a separate lazily loaded chunk.

Updated: spec (scope table row, Roadmap phase 3). No requirement ID or feature file yet. No MVP feature changes.

**Decided on 7 Nov 2026, zip upload** (user, phase 2, UP-11, F-35):

- **Browser only:** the upload page takes the FIT file out of a zip archive; the server never receives an archive and is unchanged. The upload API and the iOS Shortcut (F-34) stay FIT only, so a malicious archive never reaches the backend. A file counts as an archive by its content, whatever its name.
- **One root FIT file:** accepted only when the archive root holds exactly one entry ending in `.fit` (case-insensitive); FIT files in folders don't count, other entries are ignored. None, or two or more: refused, each with its own message.
- **Formats:** stored (0) and deflate (8) entries; an encrypted FIT entry, other methods, split or multi-disk archives and Zip64 are refused with a message.
- **Zip-bomb guard:** the existing 20,000,000-byte upload limit applies to the archive and to the extracted file; decompression stops once the output would exceed it, and the archive is refused.
- **Browser without `DecompressionStream`:** zip refused with a message; FIT upload unaffected. Android and iOS share-to-browser is not in scope.
- **Build order:** F-35 first in phase 2, before F-34. Owner frontend.

Updated: spec (sharer story 12, scope table, UP-1, new UP-11, NFR Security, Roadmap phase 2), F-35 (new), feature index (order, counts, coverage). Technical design: front-end implementation notes to mention zip reading on the upload page (architect).

**Decided on 7 Nov 2026, route colouring rework** (user, OQ-107, F-25 design v0.2, ADR-037 rev. 1):

- **(OQ-107) accepted:** speed or pace, heart rate and power in 12 workout-relative equal-share classes; gradient in 11 fixed classes, "flat" one neutral middle class; three colours, colour-blind safe in light and dark. A steady ride showing small differences strongly is accepted; the user iterates later if needed.

Updated: F-25 (scope, two new acceptance criteria appended; AC 3 to 7 unchanged). Spec unchanged (AP-9 states no class count or palette).

**Decided on 7 Nov 2026, zip upload details** (user, UP-11, F-35):

- **Confirmed:** archive limit 20,000,000 bytes; zip detected by content, not name; encryption and method checked on the FIT entry only, Zip64 and split archives refused as a whole; message wording reviewed on screen.
- **Priority:** UP-11 raised from Could to Should within phase 2.

Updated: spec (UP-11), F-35 (priority, scope, one acceptance criterion for unchecked other entries, user check of messages), feature index (coverage).

**Decided on 7 Nov 2026, preview image with map context** (user, phase 2, new SH-12, F-36):

- **Made in the browser at publish:** the edit page renders the public route (hidden ends excluded, frame fitted to the public route only) on OpenFreeMap tiles with SH-5's headline numbers into a 1200 × 630 PNG and sends it with the publish request or an edit-token request (architect decides). Map attribution in the image.
- **Trust boundary:** size cap, must decode as a PNG of the expected size, re-encoded; the received bytes are never served. Stored instead of the server-made image.
- **Fallback** to the server-made image when WebGL is missing or anything fails. `regenerate` keeps a browser-made image only while it fits the current public route. A change of hidden ends or route after publish drops or replaces it; F-36 also counts a name change, since the name is in the image.
- **No new server calls to external services.** Privacy note on About: the preview shows where the route is, as the public page already does.
- **Build order:** after F-35, before F-34. Owner both. Priority assumed Could (phase 2).

Updated: spec (viewer story 15, scope table, SH-5 pointer, new SH-12, open question "Preview image with map context", Roadmap phase 2), F-36 (new), F-23 (out of scope pointer), feature index (order, counts, coverage). Technical design: preview images, publish request and `regenerate` to be updated (architect).

**Decided on 8 Nov 2026, map controls** (user, F-13 D-19, F-25 C-6):

- **Pin map (AP-18):** icon-only pin button in the map's top-right control stack under fullscreen; accessible name "Pin map", state via `aria-pressed`, visibly filled when pressed. Replaces the "Pin map" / "Unpin map" text.
- **Route colouring control (AP-9):** at the map's top-left in every layout.

Updated: spec (AP-18), F-13 (scope, AC).

**Decided on 9 Nov 2026, zip upload open questions** (user, OQ-108 to OQ-111, UP-11, F-35):

- **OQ-108:** split archives are never accepted. The first part is refused in the browser ("could not be opened"); later parts and self-extracting archives don't look like a ZIP archive, are sent and refused by the server as not a FIT file.
- **OQ-109:** root entries whose name starts with `._` (macOS metadata) are ignored when looking for the FIT file.
- **OQ-110:** the picker has no file-type filter (a filter can grey out `.fit` in phone pickers); that meets "accepts `.fit` and `.zip`".
- **OQ-111:** a root `.fit` entry that is itself a zip archive is refused as damaged; nothing is sent.

Updated: spec (UP-11), F-35 (scope, acceptance criteria).

**Decided on 9 Nov 2026, preview image with map context** (user, OQ-112, OQ-113, SH-12, F-36):

- **OQ-112:** publishing itself never waits for the image; the edit page may delay opening the published page by at most 10 s so a link copied there already has the map card (F-36 MC-8(a)).
- **OQ-113:** the browser renders and sends only the basemap; the server composes route, sport icon, name, values and attribution (F-36 MC-1, ADR-041).

Updated: spec (SH-12, SH-5 pointer, scope table row), F-36 (scope; ACs unchanged).

**Decided on 9 Nov 2026, report notification links** (user, INF-4, new INF-6, F-37; critical lane):

- **Viewing link and readable reason:** the Pushover notification opens the reported workout's viewing link (Pushover's `url` field) and names the reason in words ("Personal data", not `personal_data`). Reverses "Pushover learns only the report number and reason" (3 Nov 2026, ADR-039 item 2): the link is unlisted and the reporter had it anyway; harm accepted as negligible. Pushover still never gets the reporter's message or the title.
- **Delete link:** the notification carries a per-report delete key in the link's fragment: valid only for that report's workout, single use, expires (30 days suggested, the architect may shorten), stored only hashed, never logged. Opening it shows a confirmation page (noindex, no referrer) with enough to decide; only an explicit confirm (`POST`) deletes, exactly like the operator delete command (F-27 R-8, SH-10, same log line without the key). Used, expired and unknown keys and a gone workout are handled clearly. Only when Pushover is configured.
- **About page** states that the notification contains a viewing link and a single-use delete link.
- **Build order:** after F-34. Owner both, backend first.
- **OQ-114:** one plain "link no longer valid" message for used, expired and unknown keys and a gone workout; nothing extra stored.
- **OQ-115:** reports folded into `(+N more)` get no delete link of their own; the architect may create keys only when a notification is sent.
- **OQ-116:** the confirmation page does not show the reporter's message.
- **OQ-117:** key lifetime 30 days; the architect may shorten it.
- **OQ-118:** INF-6 is Should within phase 2.
- **OQ-119:** anyone with the operator's Pushover account or unlocked phone can delete reported workouts: accepted; stated on the About page.

Updated: spec (scope table row, INF-1, INF-4, new INF-6, NFR Privacy Pushover line, NFR Security, Roadmap phase 2), F-37 (new, OQ-114 to OQ-119 resolved), feature index (order, row, counts, coverage). Technical design and ADR-039 to be updated by the architect.

**Decided on 9 Nov 2026, Impressum page** (user, new INF-7, F-38):

- **Need:** the hoster can set an Impressum (German legal notice) holding personal data, hard to crawl. Rejected: a scrambling font (screen readers, copy and paste, legibility risk, trivially decoded).
- **Page:** its own page `/impressum`, heading "Impressum / Legal notice", `noindex` (meta and `X-Robots-Tag`) and disallowed in `robots.txt`. The HTML holds no text; the page's script loads it from an address that is also `noindex` and disallowed. Readers without JavaScript see a short note that JavaScript is needed. No external service, no consent needed.
- **Text:** an optional file on the server next to the operator file, following the operator-file pattern (F-29 AB-8: one setting, size cap, plain text with the three-rule format, read at start, invalid exits 2); never in the repository. Unset: no page (not found) and no footer link.
- **Footer:** an "Impressum" link on every page when set.
- Whether an Impressum is legally required is the hoster's call (out of scope).
- **Hardening:** the text loads only after a click on "Show Impressum"; the text address answers only same-origin requests and at most 30 per rolling hour per address (IPv4 or IPv6 /64), at most 1,000 addresses tracked, new addresses refused while full (decided 9 Nov 2026, following architect IM-6). Only raises the effort for scrapers.
- **Design (9 Nov 2026, user):** page shells in `{data}/site/` (ADR-043) accepted; the user accepts the legal risk of a click-to-load, JavaScript-only Impressum.
- **F-23:** `robots.txt` is no longer 404; F-23 scope and AC follow F-38 IM-8 (always served, disallows only `/impressum` and `/api/impressum`, never `/w/`).
- **Details (OQ-120 to OQ-124, as suggested):** static published pages follow the setting after `regenerate`; one static `robots.txt`, always served; operator-file format and limits, missing or invalid file exits 2; Should within phase 2; load-failure wording "The Impressum could not be loaded. Reload the page."
- **Build order:** after F-37. Owner both, backend first.

Updated: spec (viewer story 16, scope table row, footer line under Activity page, DS-4, new INF-7, open question "Impressum details", Roadmap phase 2), F-38 (new), feature index (order, row, counts, coverage DS-4, INF-7, NFR `noindex`). Technical design (Configuration, Hosting, Indexing: `robots.txt` now exists, Repository layout, footer) to be updated by the architect. Later the same day: F-38 rate limit, spec INF-7, F-23 (`robots.txt`).

**Decided on 9 Nov 2026, How to use and shareable range** (user, new AP-22, AP-19, F-39, F-40):

- **How to use (AP-22, Could, phase 2):** viewing page only; a text button "How to use" with an icon in the header's control row, visible on phones, not a bare "?"; emphasised until first opened on the device, then plain; a dialog of one line per section (cursor, range, lap and split rows, fullscreen map, units and theme, sharing a range once F-40 ships), worded for touch or mouse, loaded only when opened.
- **First-visit range hint:** the AP-7 hint on the viewing page shows until the first range selection on the device or until dismissed. Both flags are kept in browser storage like the units choice (no cookie, nothing to the server; works without storage).
- **Shareable range (AP-19):** the range panel gets a "Copy link to this selection" button.
- **Details (later the same day):** OQ-126, OQ-127, OQ-129 to OQ-132 as suggested: button only, address bar not updated live, Copy link (SH-6) stays plain, cursor position never in the link. OQ-128 rejected: no lines are left out; every workout shows the same dialog, the map lines worded "On workouts with GPS, ...".
- **Build order:** F-39, then F-40, after F-38. Both frontend.

Updated: spec (viewer story 17, scope table row, AP-7, AP-19, new AP-22, open questions "Range in the link" and "How to use details", Roadmap phase 2), F-39 and F-40 (new, OQ-126 to OQ-132 open), feature index (order, rows, counts, coverage AP-19, AP-22), Phase 2 list (shareable range moved to F-40). Technical design (`lib/prefs` keys, ADR-011 shares, ADR-020 fragment) to be updated by the architect. Later the same day: OQ-126 to OQ-132 resolved in F-39, F-40, spec AP-19, AP-22 and open questions.

**Decided on 9 Nov 2026, multisport sections shown** (user, AP-12, phase 2 idea, no feature file): charts get a vertical line at each leg boundary with the sport labelled on top; the laps table a separator row with the leg's name before each leg; the map a marker at each leg start. Updated: spec AP-12, Phase 2 list.

The original questions follow for reference.

**Contradictions**

1. **Expiry job frequency vs SH-10.** The technical design runs expiry hourly; SH-10 requires expired content to stop being served within 60 s. *Assumed:* the job runs every minute and at startup (F-21). The technical design needs updating.
2. **"This workout has expired" vs "the page does not exist".** UP-7 wants an expired page with that text; SH-2 wants draft and private to look like a missing page. Once a workout is deleted nothing is left, so the server cannot tell expired, deleted, draft, private and unknown IDs apart without keeping data. *Assumed:* one generic page for all of them, with no title, numbers or Open Graph data (F-15, F-21). **Which wording?** Proposed: "This workout has expired or is not available."
3. **"Generic card" (SH-5) vs "no Open Graph data" (technical design).** *Assumed:* no OG tags at all for draft, private and expired links, so messengers show a plain link (F-23). Alternative: a site-wide card (site name, generic image, no numbers).
4. **Backend agent file says "`robots.txt` disallows `/w/`"**; the technical design says it must not. *Assumed:* the technical design is right (F-23). The agent file should be corrected (outside this folder, not changed).

**Gaps and ambiguities**

5. **Metric definitions** (open question in the spec). Needed before F-13, F-17, F-18 and F-23 are finished; F-07 asks for them to be settled. Until then, *assumed* placeholders: the summary shows every metric the device recorded; range averages use all recorded samples in the range; the time axis is elapsed time with pauses drawn as breaks; hovering a self-crossing route picks the nearest sample, and the earliest one on a tie.
6. **Touch gestures.** AP-6 says touch-drag moves the cursor; AP-7 says drag selects a range; the page also has to scroll on a phone. *Assumed:* horizontal drag scrubs, vertical swipe scrolls, long-press then drag selects a range on touch; on desktop, hover moves the cursor and click-drag selects (F-05, F-17).
7. **Units on the static page.** The summary and laps are pre-rendered in the HTML at publish, but units depend on the viewer's browser. *Assumed:* pre-rendered in metric; the bundle re-renders them for imperial viewers (a brief switch on first paint is accepted). The preview image and `og:description` are always metric (F-13, F-14, F-23).
8. **Browser language → units.** *Assumed:* imperial for US English (`en-US`) and other US-region locales; metric for everything else, including `en-GB` (F-14).
9. **Rowing in imperial.** *Assumed:* pace stays per 500 m in both unit systems; swimming switches to per 100 yd (F-14).
10. **Three headline numbers** on the preview card (SH-5) are not named. *Assumed:* distance, moving time and elevation gain; for workouts without one of them, the next metric the sport has (e.g. average heart rate). Without positions, the card shows numbers only (F-23).
11. **Truncated files.** Device totals (AP-2) don't exist when the file ends before its session message. *Assumed:* totals are computed from the recovered records (F-08). For "data after hh:mm:ss is missing", *assumed:* hh:mm:ss is the elapsed time since start. The notice is shown only to the owner on the edit page (F-19).
12. **Multi-session and chained files in the MVP** (sections per leg are phase 2). *Assumed:* shown as one workout with streams on one time axis, additive totals summed over sessions, and the sport of the first session (F-08). Alternative: show "Multisport" as the sport.
13. **Local time zone.** "Local time at the start position" needs a time zone source. *Assumed:* the offset from the file's local timestamp; if missing, show UTC labelled as such (F-02, F-13).
14. **How the 300 m is measured (SH-4).** *Decided 26 Oct 2026 (OQ-71), replacing "along the GPS track":* from the first and last positioned sample, until the track first leaves a 300 m circle around it; not the device distance stream, so a late GPS fix still hides the start of the visible route. Lap and start/finish markers in hidden sections are not published (F-16).
15. **Does the owner see what will be hidden?** *Assumed:* the edit page shows the full track with the to-be-hidden start and end marked (F-19). Alternative: a "view as viewer" toggle.
16. **"Shown once" edit link (AC-1).** *Superseded 30 Oct 2026 by the published page (AC-6, decision log).* *Assumed:* after upload the browser opens the edit link; on that first open only, a notice shows the edit link with a copy button and warns that it cannot be recovered. The server never returns the token again (F-11, F-20).
17. **Title and description.** The spec gives no limits and no default title before UP-5. **Decided 23 Oct 2026 (decision log, Owner API):** title up to 100 characters after trimming, description up to 1,000, both counted as Unicode code points, both plain text. *Assumed:* an empty title shows the sport and date (e.g. "Ride, 9 Oct 2026") (F-12, F-13, F-19).
18. **Lifetime picker.** *Assumed:* whole days, 1 to 30 from upload; a lifetime that has already ended is not offered (delete instead); shortening is allowed (F-12, F-19).
19. **Publish target.** *Assumed:* Publish makes the workout unlisted; ~~private is only reachable after publishing; private can switch back to unlisted;~~ a published workout cannot return to draft (F-15, F-19). *Superseded in part 30 Oct 2026: no private state (decision log).*
20. **Error texts** for not FIT, no readable records, too large and rate limit are not worded in the spec (only "Unsupported FIT file type." and the storage-full text are). *Assumed:* short plain messages chosen during F-08/F-09; review the wording.
21. **Report rate limit and placement.** *Rate limit decided 1 Nov 2026: fixed at 60 per hour per IP (decision log).* *Assumed:* 60 reports per hour per IP (like uploads); the Report link appears on published workout pages only, not on the edit page (F-27, F-28).
22. **Storage cap semantics (UP-8).** *Assumed:* uploads are refused once the total reaches the cap; one accepted upload may go slightly over; publish, edits, regeneration and deletes are never refused (F-10).
23. **Viewer download contents (SH-8).** *Assumed:* the export contains only what the page shows (file type, manufacturer and product, session totals, laps, public records), with no device serial number or sensor list (F-24).
24. **Uptime check "on one workout page".** Every workout expires within 30 days, so there is no permanent page to monitor. *Assumed:* monitor the About page and the API health endpoint (F-31).
25. **Operator identity on the About page.** GDPR privacy information normally names the controller and a contact; INF-1 doesn't mention it. *Assumed:* the About page names the operator and gives a contact address; please confirm (F-29).
26. **GeoNames attribution.** INF-1 asks for the place-name data source, but place names (UP-5) arrive in phase 2. *Assumed:* GeoNames is listed only once UP-5 ships (F-29).
27. **Route colouring default (AP-9).** *Assumed:* plain single-colour route until the viewer picks a metric; the choice is not stored (F-25).
28. **Dark map (DS-4).** *Assumed:* in dark theme the map uses a dark OpenFreeMap style if one is available; otherwise it stays light (F-14).
29. **"Should" items under time pressure.** All five Should items are planned for the MVP. *Assumed:* if time runs short, they are cut in this order: AP-9, AP-15, DS-4, SH-8. SH-4 is cut last because it is the only privacy control in the MVP.
30. **Who owns the page template.** The summary and laps are rendered by Go (static page) and by Svelte (edit page and unit switching), so formatting exists twice. *Assumed:* the frontend writes the HTML template and CSS; the backend fills it from a documented view model; a test checks that both renderers give the same numbers for the corpus (F-13, F-15).
31. **API endpoints not in the technical design.** Only upload, publish, status and reports are named. *Assumed:* the backend defines and documents the owner endpoints (read draft, update settings, delete, original download) and the edit-link path in F-09/F-12. This is a technical gap, not a product decision.
32. **Phase 0 test conditions.** The reference phone and test conditions are still open. *Assumed:* the operator's own phone, with a performance trace of a 10 s scrub; the gate passes at 60 fps in the trace and when the operator judges it smooth (F-07).
33. **Pool swims in the MVP** (lengths are phase 2). *Assumed:* handled like any indoor workout: no map, charts for the streams present, no lengths table (F-02, F-13).
