# Architecture overview: history

History of [architect/overview.md](../overview.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Header version line as written (revision list)

- **Version:** 1.23 (1.23, F-09 v1.11, the user's decision of 22 Oct 2026: the configuration rule lists a second environment variable, `WORKOUT_EXPLORER_PARSE_TIMEOUT` (ADR-017 rev. 7; whole seconds, default 10, 5–60), read by `serve`, `parse` and `regenerate`; 1.22, F-09 v1.10, the user's decisions of 22 Oct 2026: glossary, the acceptance rule no longer promises acceptance for files from real devices, and the corpus strictness is named as a regression check (ADR-004 rev. 11, ADR-023 rev. 9); regeneration and format version state the OQ-55 exception, and the new term kept workout (ADR-005 rev. 1); 1.21, F-09 v1.9: glossary, the rejection kind `unreadable` (a deliberate refusal, `Accept` only) and the new term acceptance rule, which replaces "upload accepts whatever `Parse` accepts" (the user's decision of 21 Oct 2026; ADR-018 rev. 5, ADR-023 rev. 8); 1.20, F-09 v1.5: I-8 names the segment account keys (OQ-52) and says `original.fit` holds only records the parser read; glossary: a truncated original ends at the last record the parser read, and "rejected record" (UP-9's "damaged record") joins the truncated file entry (ADR-023 rev. 4); 1.19, F-09 v1.4: I-8 names every ANT+ device number in any message, raw ANT messages, and segment device and leader activity IDs (UP-9 read literally, ADR-023 rev. 3); 1.18, F-09 v1.3: the configuration rule says that Go runtime variables (`GOMEMLIMIT`) are not application settings and are set only by the systemd unit (F-31, ADR-017 rev. 3); 1.17, F-09 v1.2: I-8 lists the whole user profile as removed, not five of its fields; 1.16, F-09 v1.1: invariant I-8 (identifying data removed from the file is stored, shown and logged nowhere); `ingest` exposes `Accept` (parse, strip, reparse); glossary terms original (now the stripped upload), identifying data and strip; the configuration rule allows exactly the environment variables listed in it, first `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS`; ADR-023; 1.15, F-09: invariant I-7 (no request metadata stored or logged); glossary terms internal ID, workout record, viewing link, edit link, upload cap, data directory, staging folder; public ID and edit token made precise; Lifecycle context with `internal/lifecycle`, `internal/store`, `internal/privatedir`; dependency rules for them and for `httpapi → ingest` (error type only); `serve -data` required; logging row; section 8 SQLite decided; ADR-019 to ADR-022; 1.14, F-08 design v1.3: glossary term transition (sample, session, lap) and its dropped values; device totals and combined totals reworded for transitions and the single-recording elapsed-time rule; 1.13, F-08 design v1.2: time offset never rebased per session or FIT sequence; combined totals take the wall time as elapsed time; 1.12, F-08 design v1.1: glossary terms leg, record leg, sport hint, sport timeline; combined totals and parse limits reworded; 1.11, F-08: ingestion exposes `ingest.Parser` (`NewParser(Limits, *slog.Logger)`, `Parse(ctx, []byte)`), parse limits and slots (ADR-017), rejection kinds, messages and the allow-listed parser log line (ADR-018); I-5 enforced by a test in `ingest`; glossary terms FIT sequence, chained file, truncated file, data end, CRC failure, rejection, parse limits, record totals, combined totals; logging convention row; 1.10, F-07: the prototype page replaces the F-01 placeholder as the app entry (`src/prototype/`), lazy scrub recorder `src/measure/`, dev-only `tools/protoserve` and `scripts/prototype-data.sh`, their dependency rules, glossary terms prototype page, scrub record and flush, ADR-016; 1.9, F-06 QA r3: gap, GPS dropout and bridge entries say that both causes are drawn as a dashed bridge on the map; 1.8, F-06 v1.4: glossary terms recorded segment and bridge, route and GPS dropout reworded; 1.7, F-06: invariant I-6 (the tile host is the only third party), `lib/route` and `components/map` in the web layout, MapLibre import rule and static/lazy split, `bench/map.html`, glossary terms route, positioned sample, GPS dropout, route presence, cursor marker, start/finish and lap markers, tile host, ADR-013 to ADR-015, cross-cutting row "Third parties in the browser"; 1.6, F-05: `lib/cursor`, `lib/charts`, `components/charts` in the web layout, uPlot import rule, cursor store rule, glossary terms segment and break, cursor clarified, "gap" note on feature files removed (F-04 and F-05 fixed), ADR-008 to ADR-010, bundle allocation rule (ADR-011), component smoke tests in jsdom (ADR-012); 1.5, F-04: `lib/datafile`, `test-support/`, `bench/` in the web layout, front-end import rule, glossary terms missing sample, metric stream, decoded workout data, gap clarified, ADR-007; 1.4, F-03: packages `datafile` and `precompress`, `testdata/datafile/`, the brotli module rule, glossary terms for the data file, section 8 data file row decided; 1.3, F-02: `ingest.Parse` signature, glossary terms, `tools/` and `scripts/` in the Go layout, FIT library rule; 1.2: section 6 Configuration allows one dev-tooling environment variable, `DEV_ADDR`; 1.1: CI host fixed as GitHub Actions in the layout tree)

## Section 4, layout tree line as written

            ├── main.ts                 mounts the prototype page (F-07; was the F-01 placeholder App.svelte)

## Section 8, rows decided since

| ~~SQLite driver, schema and migrations~~ | Decided: F-09 design, ADR-019 |
| ~~Data file encoder/decoder details~~ | Decided: F-03 design (format v1, normative section), ADR-005, ADR-006; browser decoder in the F-04 design and ADR-007 |

## Retired: merged into the Technical Design (23 Oct 2026)

On 23 Oct 2026 `architect/overview.md` (version 1.23) was merged into `spec/Workout Explorer — Technical Design.md`, which is now the single home of the system-wide technical picture. `architect/overview.md` is a stub that points there. Where the content went:

| Overview section | Technical Design section |
| --- | --- |
| 1. The system in one picture (picture, invariants I-1 to I-8) | Architecture (picture; Invariants, same IDs) |
| 2. Ubiquitous language | Glossary |
| 3. Bounded contexts and modules; published language | Code structure (Bounded contexts and modules; Published language) |
| 4. Repository layout | Code structure (Repository layout) |
| 5. Dependency rules | Code structure (Dependency rules) |
| 6. Cross-cutting conventions | Conventions (Configuration as its own part with a settings table); the Logging row also; the "Third parties in the browser" row went to Security and privacy (Headers and third parties) and I-6 |
| 7. Build and CI | Hosting and operations (Build and CI) |
| 8. Not decided here | Open technical questions |

Dropped in the merge as history: the version tags inside glossary entries ("(1.21 …)", "(1.22 …)"), the clause "Replaces 'upload accepts whatever `Parse` accepts'" in the acceptance rule, "`internal` now always means a defect" (stated as current), and the version numbers of feature designs cited in the glossary.

### Technical Design: decided open questions removed in the merge

These rows of the TD's "Open technical questions" were decided and repeated facts stated elsewhere (TD Tech stack, TD Workout data file Versioning, product spec PIN-3), so they were removed from the live document on 23 Oct 2026:

- [x] **Go or TypeScript on the server?** Decided: Go.
- [x] **Which Go FIT library?** Decided: `muktihari/fit`, confirmed against the test corpus first.
- [x] **Preview image background:** Decided: route on a plain background for now.
- [x] **Tiles:** Decided: OpenFreeMap. A self-hosted Protomaps file served by Caddy stays the fallback if the external dependency becomes a problem.
- [x] **Data file versioning:** Decided: regenerate every workout from its original on every deploy (see Workout data file, Versioning).
- [x] **Photos for pins:** Decided: moved to Later; pins in phase 2 have no photos.

### Final content of architect/overview.md (version 1.23), as it stood

Headings are moved down three levels; the text is otherwise unchanged.

#### Architecture overview

- **Version:** 1.23
- **Status:** accepted
- **History:** [architect/history/overview.md](history/overview.md) (revision list, decided rows of section 8)
- **Date:** 10 Oct 2026
- **Depends on:** `spec/Workout Explorer — Technical Design.md`, `spec/Workout Explorer — Product Specification.md`, `features/README.md` (decisions of 10 Oct 2026)
- **Supersedes:** none
- **Related ADRs:** [ADR-001](ADR-001-repository-layout-and-build-entry-point.md), [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-003](ADR-003-fit-decoding-library.md), [ADR-004](ADR-004-parser-corpus-and-golden-reports.md), [ADR-005](ADR-005-workout-data-file-format-v1.md), [ADR-006](ADR-006-brotli-pre-compression.md), [ADR-007](ADR-007-decoded-workout-data-in-the-browser.md), [ADR-008](ADR-008-shared-cursor-store.md), [ADR-009](ADR-009-uplot-as-renderer-only.md), [ADR-010](ADR-010-touch-gestures-on-charts.md), [ADR-011](ADR-011-activity-page-bundle-allocation.md), [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md), [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md), [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md), [ADR-015](ADR-015-map-interaction-hover-touch-nearest-sample.md), [ADR-016](ADR-016-scrubbing-smoothness-measurement-and-gate.md), [ADR-017](ADR-017-parse-isolation-and-limits.md), [ADR-018](ADR-018-parse-outcomes-messages-and-log.md), [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md), [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md), [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md), [ADR-022](ADR-022-private-workout-files-staging-and-recovery.md), [ADR-023](ADR-023-identifying-data-stripped-by-a-byte-level-rewriter.md)

The system-wide picture: what the parts are, who owns which concept, where code lives and which way dependencies point. It does not design individual features; each feature's design (`F-NN-*.md`) fills in its part and may refine, but not contradict, this document without an ADR.

##### 1. The system in one picture

```
                        view path (no Go)                       publish path
 Browser ───────────────────────────────▶ Caddy ──── /api/* ────▶ workout-explorer (Go, one binary)
   │  /w/{public_id}/index.html             │                        │   writes            reads/writes
   │  bundle (content-hashed)               │ serves public/ files   ├──────────▶ /srv/workouts/public/   (only after hiding)
   │  data.{hash}.bin(.br)                  │ directly               ├──────────▶ /srv/workouts/private/  (original, draft)
   │  preview / export                      │                        └──────────▶ SQLite (WAL): ownership, settings, reports
   └── MapLibre + tiles ──▶ OpenFreeMap (loaded last)
```

Invariants every module must keep (from the technical design, restated because code structure enforces them):

| # | Invariant | Source | Enforced by |
| --- | --- | --- | --- |
| I-1 | No request in the view path reaches Go. | TD Publish and view flow | Caddy config (F-31); Go never serves `public/` |
| I-2 | FIT is decoded only on the server, by one pipeline, for draft and published page alike. | TD Principles "One parser" | Single `ingest` package; front end has no FIT code |
| I-3 | Nothing is written under `public/` until hidden sections are removed; only unlisted workouts go there. | TD Security and privacy, SH-4, SH-2 | One package (`publish`) owns all writes to `public/` |
| I-4 | Everything stored for a workout is deleted on delete or expiry. | UP-7, AC-4, SH-10 | One delete path in `lifecycle`, used by API, expiry job and CLI |
| I-5 | Coordinates never appear in logs. | TD Hosting and operations (Logs) | Logging convention (section 6) and review; for the parser, a key allow-list and a position scan in `TestParseLogs` (ADR-018) |
| I-6 | The viewer's browser contacts no third-party host except the tile host, and sends it no referrer and no credentials. | TD Security and privacy (CSP, GDPR), NFR Privacy | ADR-014; CSP and `Referrer-Policy` (F-22); review |
| I-7 | No request metadata is stored or logged: the client's IP address (in any form), `X-Forwarded-For`, user agent and upload file name never reach the database, a file or a log line. Rate limiters hold only process-local hashes in memory. | NFR Privacy (store as little as possible), TD Security and privacy (Uploads), INF-2 | ADR-021 item 11; `TestNoRequestMetadataStored`, `TestSchema` (F-09) |
| I-8 | Identifying data that UP-9 removes from an upload (serial numbers; ANT+ device numbers and IDs in any message; raw ANT messages; segment device and leader activity IDs; segment account keys of the creator and the leaders' groups; the user profile, saved locations, segment leaderboard entries) is stored, shown and logged nowhere: the upload's own bytes are never written; `original.fit` is the stripped file and holds only records the parser read; `draft.bin` and the row come from parsing the stripped file. | UP-9, NFR Privacy | ADR-023 (rev. 4: items 4 and 6a); `TestStripIdentifyingData`, `TestUploadIdentifyingData`, `TestStripRejectedRecord` (F-09) |

##### 2. Ubiquitous language

Use these names in code, API fields and documents. Add to this table when a feature introduces a term.

| Term | Meaning | Not to be confused with |
| --- | --- | --- |
| Workout | One uploaded activity, from upload until delete or expiry. | "Activity" is the page, not the entity. |
| Original | The uploaded FIT file with identifying data removed (UP-9, ADR-023), kept privately (`original.fit`); not byte-identical to the upload. A truncated original ends at the last record the parser read: an incomplete or rejected record and everything after it are not kept (ADR-023 rev. 4 item 6a). | Export; the upload (never stored) |
| Identifying data / strip | The fields and messages UP-9 removes from an upload / the byte-level rewrite in `ingest.Accept` that removes them, verified by parsing the result again (ADR-023). | Hidden section (positions removed at publish, SH-4) |
| Normalised workout | The parser's output: sessions, laps, events and streams on one time axis. | The FIT messages themselves |
| Stream / sample | One measured quantity over time / one index on the shared time axis. | Record (a FIT message) |
| Draft | A workout that has not been published; visible only with the edit token (SH-2). | Draft data file (`draft.bin`), which exists for every workout |
| Unlisted / private | Visibility after publishing (SH-2). | |
| Data file | The binary `WEXD` file the browser reads (`data.{hash}.bin`, `draft.bin`). | Original |
| Hidden section | Start or end stretch whose positions are removed before publishing (SH-4). | Trim (phase 2, removes samples) |
| Public ID | 128-bit random ID in `/w/{public_id}` (SH-1); 22 base62 characters (ADR-020). Never logged. | Internal ID |
| Internal ID | The workout's 128-bit random key, 32 hex characters; names `private/{id}/`; the only workout identifier allowed in logs; never sent to a browser (ADR-020). | Public ID; a database row number |
| Edit token | The only credential; 22 base62 characters; stored only as its SHA-256 hash (AC-1); travels in the edit link's fragment and, to the API, only as `Authorization: Bearer` (ADR-020). | |
| Workout record | The workout's row in `workouts` (ADR-019); `lifecycle.Record` in code. | Normalised workout; data file |
| Viewing link / edit link | `/w/{public_id}` / `/edit/{public_id}#{token}`, root-relative in API responses (ADR-020). | |
| Upload cap | 20,000,000 bytes of file content per upload, enforced while reading (ADR-021). | Parse limit `bytes` (20 MiB, ADR-017) |
| Data directory | `serve -data`: `workouts.db`, `private/`, `tmp/`, `public/` (ADR-022). | |
| Staging folder | `tmp/{id}/`, where a new workout's files are written before one rename makes them `private/{id}/`; emptied at startup (ADR-022). | |
| Lifetime / expiry | Days until `expires_at`; then everything is deleted (UP-7). | |
| Regeneration | Rebuilding all files of every live workout from its original on deploy. (1.22) A workout whose rebuild fails for a server-side reason becomes a kept workout instead. | Publishing |
| Kept workout | (1.22, OQ-55, user, 22 Oct 2026) A workout whose rebuild failed during regeneration for a server-side reason (`canceled`, a time limit, an error not about the file). It is unchanged and serves its previous files until a later run rebuilds it; an old app bundle is deleted only once no kept workout refers to it. The deploy report counts kept workouts (ADR-005 rev. 1, F-09 F-30 hand-off). | A workout deleted because its file is rejected |
| Cursor | The shared sample index (or none) that drives charts, map marker and readouts (AP-6); held in one cursor store per workout (`lib/cursor`, ADR-008). | Selection (a range, F-17); a time or pixel position |
| Recording | `ingest`'s library-free extraction of a FIT file, input to normalisation (F-02). | Normalised workout |
| Session / lap | The device's `session` / `lap` with its totals as recorded (AP-2, AP-8). A workout has one or more sessions. | Split (computed per 1 km in the browser, F-18) |
| Time offset | Whole seconds since the workout's start time; the shared time axis. One axis across every session and FIT sequence, never rebased; the elapsed time the cursor shows is the sample's offset (F-08 D-19). | Elapsed time (a total) |
| Gap | Two consecutive samples 5 s or more apart; never filled. Drawn as a break on charts and, if both sides have a position, as a dashed bridge on the map (F-06 v1.4). Shorter gaps are filled at 1 s (F-02 R-3). In the browser: an index in `WorkoutData.gaps` (F-04). | Pause (timer stopped); missing sample; GPS dropout |
| Missing sample | A present stream has no value at an index: the sentinel in the file, `NaN` in the browser (F-03 §7, ADR-007). | Gap; absent stream |
| Segment | A maximal run of consecutive samples of one stream that are not missing and do not cross a gap; a chart line is drawn only within a segment (F-05). | Session, lap |
| Break | Where a chart line is not drawn: between two segments, caused by a missing sample or a gap; never drawn as zero (F-05). | Pause (the cause of a gap) |
| Metric stream | A present stream other than time offset and position: altitude, distance, speed, heart rate, cadence, power, temperature (`WorkoutData.metrics`, F-04). | Totals (session values) |
| Decoded workout data | `WorkoutData`: the browser's form of a data file; `Float64Array`s in the format's units plus metadata (F-04, ADR-007). | Data file (the bytes) |
| Moving time | Time with the timer running, or above a speed threshold when there are no timer events (F-02 R-7). | Timer time (device total) |
| Device totals | Session values as the device recorded them; shown as recorded (AP-2), except that running cadence is doubled (F-02 R-5) and transition sessions and laps lose their speed, cadence and power totals (F-08 R-13). | Derived values (computed from streams) |
| Corpus / golden report | Test FIT files in `src/server/testdata/corpus/` and their expected JSON reports (ADR-004). | |
| Format version | The `uint16` in the data file header; exactly one is live, and every deploy regenerates all files (ADR-005). (1.22) Except that kept workouts stay on their previous version and bundle until rebuilt; each bundle reads exactly one version (ADR-005 rev. 1). | Report version (golden report shape) |
| Descriptor | The metadata entry for one column: name, type, encoding, scale, unit, byte offset (F-03). | |
| Sentinel | The stored value that marks a missing sample: the maximum of the column's type (F-03). | Absent stream (no column at all) |
| Pre-compressed file | `data.bin.br`, a byte-identical brotli transport encoding of `data.bin` (ADR-006). | A different format |
| Golden data file / fixture | `testdata/datafile/synthetic/<name>.bin` and `.json`: the writer's output and its decoded-integer digest, the contract for F-04 (F-03). | Golden report |
| Positioned sample | A sample whose position (latitude and longitude) is present (F-06). | A sample of a present position stream whose position is missing |
| Route | The line on the map through every positioned sample in index order, drawn as recorded segments (solid) and bridges (dashed) (AP-3, F-06 v1.4). | The position stream itself; a lap |
| Recorded segment | A maximal run of positioned samples with no bridge between neighbours; drawn solid (F-06 v1.4). | Segment (a chart term, F-05) |
| Bridge | The straight segment between two consecutive positioned samples with a GPS dropout **or** a gap between them; drawn dashed, because nothing was recorded along it (F-06 v1.4). The two causes stay separate terms; on the map both look the same. | Break (where a chart line is not drawn) |
| GPS dropout | A run of samples without position between two positioned samples; the route crosses it with a dashed bridge (AP-3). | Gap (time; on the map also a dashed bridge, on charts a break); hidden section (start or end, removed at publish) |
| Route presence | `no-location-data` (no position stream: hint, no map), `no-visible-route` (stream present, no positioned sample), `route` (F-06). | |
| Cursor marker | The map's marker for the cursor; hidden when the cursor is none or its sample has no position (F-06). | Start/finish and lap markers |
| Start / finish marker; lap marker | Map markers at the first and last positioned sample; at the first sample of lap *k* ≥ 2, only if that sample has a position, labelled *k* (F-06). | Lap (the device's lap) |
| Tile host | The one external host the viewer's browser contacts: OpenFreeMap, `tiles.openfreemap.org` (ADR-014). | Our own origin |
| Prototype page | The app entry from F-07 on: map and charts for the data file named by `?data=` (F-07). F-13 turns it into the activity page. | Bench pages (`bench/`, dev server only) |
| Flush | One delivery of a changed cursor value to every listener, at most once per animation frame (ADR-008). Its duration is split into charts and map in a scrub record (ADR-016). | A frame |
| Scrub record | The in-page record of frame intervals, flush times, cursor changes and finger contact during a 10 s scrub; the "trace" for the 60 fps gate (ADR-016, F-07). | Auto-scrub (synthetic events on a bench page, F-05/F-06) |
| FIT sequence / chained file | One FIT header, its messages and CRC / a file of two or more sequences back to back. Chained files become one workout; later sequences of another file type are skipped (F-08). | Session (a FIT message; one sequence can hold several) |
| Truncated file / data end | A file whose messages break off before their end (watch died, cut-off upload, unclosed file), or whose reading stops at a **rejected record**: a record the pinned decoder refuses, such as a definition with an invalid base type (UP-9 calls it a "damaged record"). Accepted when at least one record survives. The data end is the time offset of the last intact record, shown in the UP-3 notice (`Integrity.DataEnd`, F-08). | Gap; CRC failure |
| CRC failure | A wrong or missing header or file CRC, with the messages intact. Accepted, logged, no notice (F-08, OQ-33). | Truncated file |
| Rejection / rejection kind | A file `ingest` refuses: `not_fit`, `unsupported_type`, `no_data`, `limit`, `internal` (plus `canceled` when the caller gave up), each with one fixed user message (ADR-018). (1.21) `Accept` adds `unreadable`, a **deliberate refusal**: an upload that `Parse` accepts but that has no faithful stored form, for a cause in ADR-023 item 8a's closed list. `internal` now always means a defect on our side. | A recovery (accepted with `Integrity` set) |
| Acceptance rule | (1.21, user, 21 Oct 2026; 1.22, narrowed by the user, 22 Oct 2026) Upload may refuse a file that `Parse` accepts, but only as `unreadable` ("This file could not be read.") for a cause on ADR-023 item 8a's closed list, never as `internal`. No file is promised acceptance, from a real device or not: what errors a device may write cannot be known in advance. Privacy never bends: whatever the strip is unsure of is refused (ADR-023 item 8a). Replaces "upload accepts whatever `Parse` accepts". | Rejection; corpus strictness |
| Corpus strictness | (1.22) Every corpus file that `Parse` accepts, synthetic and local real, must pass `Accept` in the tests. A regression check (the built code accepts them all), not a product promise about devices (ADR-004 rev. 11, F-09 D-29). | Acceptance rule |
| Parse limits | The time limit and the counted budgets (bytes, records, samples, laps, sessions, timer events, field descriptions, sequences, sport messages) plus parse slots, enforced in-process by `ingest.Parser` (ADR-017). The time limit is a server setting (v1.23, ADR-017 rev. 7); every other limit is fixed. | The upload size and rate limits of F-09 |
| Record totals | Totals computed from the samples (`Derived`: elapsed, distance, gain, loss, averages, maxima; R-11); used in the summary wherever the device recorded none (OQ-22, F-08). | Device totals; range statistics (F-17, in the browser) |
| Leg / record leg | A part of the workout the summary combines: each session is a leg; the record leg is the part after the last surviving session (the whole workout if there is no session) in a truncated or session-less file, with values from the records (F-08 R-10 step 0). | Per-leg sections (AP-12, phase 2, a display); session (a FIT message) |
| Sport hint / sport timeline | A sport statement outside the sessions: a `sport` message (placed at the next record) or `lap.sport`. The sport timeline gives every sample its sport: its session, else the latest hint since the previous session ended, else F-02's session fallback (F-08). | Workout sport (`multisport` when the legs' sports differ) |
| Combined totals | The summary of a workout with several legs: additive totals summed, except elapsed time, which is the wall time from the workout start to the last record (F-08 D-18); maxima combined, averages from the records; average speed and cadence hidden when the sports differ (F-08 R-10). The summary elapsed time is that wall time for every workout except one session recorded in one FIT sequence, which keeps the device's value (F-08 R-10.E, v1.3). | Per-leg sections (AP-12, phase 2) |
| Transition / transition sample | A transition (T1, T2) is a session with FIT sport `transition`, and nothing else is (F-08 D-23). A transition sample is one covered by such a session: rule 1 of the sport timeline (the first session in file order whose span contains it) is a transition session. A sport hint or a fallback never makes a transition sample. At transition samples, and in transition session and lap totals, only position, altitude, heart rate, distance and temperature are kept; speed (so pace), cadence, power and any later stream are missing samples or absent totals, in the workout, the report and the data file (F-08 R-13, AP-4). | Gap; GPS dropout; a pause |

##### 3. Bounded contexts and modules

Small system, so contexts map to packages, not services. Core domain is **turning a FIT file into a faithful, private-by-construction, fast activity page**: the normalised workout, the publishing rules and the viewer.

| Context | Kind | Owns | Exposes | Lives in | First built |
| --- | --- | --- | --- | --- | --- |
| Ingestion | Supporting | FIT decoding (ADR-003), validation, recovery, parse limits, the parser log line; anti-corruption layer around `muktihari/fit` | `NewParser(Limits, *slog.Logger) *Parser`, `(*Parser).Parse(ctx, []byte) (workout.Workout, error)`, `(*Parser).Accept(ctx, []byte) (Accepted, error)` for uploads (parse, strip identifying data, reparse; F-09, ADR-023), failures as `*ingest.Error` with `Kind` and `Message()` (F-08, ADR-017, ADR-018); one `Parser` per process; no library type crosses the boundary | `src/server/internal/ingest` | F-02, F-08 |
| Workout model | Core | Normalised workout, sessions, laps, streams, summary; every normalisation rule (time axis, gap filling, moving time, gain, distance, time zone); later hiding | `Normalise(Recording) Workout`, Go types and pure functions, no I/O | `src/server/internal/workout` | F-02 |
| Publishing | Core | Data file encoding, static page, preview image, export file; the only writer to `public/` (I-3) | `Publish`, `WriteDraft`, `Regenerate` style operations | `src/server/internal/publish` (F-15); data file codec in `internal/datafile` and brotli in `internal/precompress` (F-03) | F-03, F-15 |
| Lifecycle | Supporting | Workout record, internal ID, public ID, edit token, links, visibility, lifetime, expiry, storage limit, the single delete path (I-4) | `lifecycle.Service` (`Upload`, `Recover`; later features add theirs), pure `NewDraft`, `NewIdentity`, `HashToken`, `ViewPath`, `EditLink`; ports `Parser`, `Repository`, `PrivateFiles` (F-09) | `src/server/internal/lifecycle`; adapters `internal/store` (SQLite, ADR-019) and `internal/privatedir` (ADR-022) | F-09 |
| Reports | Supporting | Stored reports (INF-2), operator delete (INF-3) | `POST /api/reports`; CLI | `src/server/internal/reports` | F-27 |
| HTTP API | Adapter | Routing, request/response mapping, token header, rate limits | `/api/*` | `src/server/internal/httpapi` | F-01 |
| Viewer | Core (browser) | Data file decoding, cursor store, charts, map, range stats, units | Svelte pages | `src/web/src/` | F-04 onwards |

Package names above other than `httpapi` are the **proposed** names; each feature's design confirms or adjusts them.

**Published language between server and browser** (contracts; a change needs both engineers and a design-doc update):

| Contract | Owner | Defined in |
| --- | --- | --- |
| Data file format (`WEXD`, version) | backend | TD Workout data file; F-03 design |
| HTTP API (`/api/*`) | backend | F-01 (health), F-09, F-10, F-12, F-15, F-27 designs |
| Link formats (viewing link, edit link with the token in the fragment) and the token header | backend | ADR-020 |
| Page view model and HTML template | frontend writes template, backend fills it (review decision 30) | F-13 and F-15 designs |

##### 4. Repository layout

Decided in [ADR-001](ADR-001-repository-layout-and-build-entry-point.md). Rows marked "planned" show where later code goes; they are not created in F-01.

```
/                              repo root
├── Makefile                   single build entry point for people and CI (F-01)
├── README.md                  short: layout, prerequisites, make targets, local dev (F-01)
├── .github/workflows/ci.yml   GitHub Actions: thin wrapper that runs `make -k ci` (F-01)
├── build/                     all build output, git-ignored (F-01)
│   ├── workout-explorer       Go binary
│   └── web/                   Vite output incl. .vite/manifest.json
├── deploy/                    planned (F-31): Caddyfile, systemd unit, deploy script
└── src/
    ├── server/                Go module `workout-explorer` (F-01)
    │   ├── go.mod, go.sum
    │   ├── cmd/workout-explorer/   main: flags, wiring, subcommands (serve; parse (F-02, diagnostic); regenerate, delete later)
    │   ├── internal/httpapi/       HTTP adapter (F-01: health)
    │   ├── internal/ingest/        FIT anti-corruption layer (F-02)
    │   ├── internal/workout/       normalised workout, pure rules (F-02)
    │   ├── scripts/corpus.sh       `make corpus` driver (F-02)
    │   ├── scripts/prototype-data.sh   `make prototype-data` driver; output only in build/prototype/data (F-07)
    │   ├── tools/corpusgen/        dev-only synthetic FIT generator, not in the binary (F-02, ADR-004)
    │   ├── tools/protoserve/       dev-only static server with in-memory brotli for the prototype, never deployed (F-07)
    │   ├── internal/datafile/      data file codec, format v1, pure (F-03, ADR-005)
    │   ├── internal/precompress/   brotli pre-compression (F-03, ADR-006)
    │   ├── internal/corpustest/    corpus listing for tests only, never in the binary (F-03)
    │   ├── internal/publish/       planned (F-15)
    │   ├── internal/lifecycle/     workout record, identity, links, Upload and Recover; no SQL, no os (F-09)
    │   ├── internal/store/         SQLite adapter, embedded migrations in migrations/ (F-09, ADR-019)
    │   ├── internal/privatedir/    the only writer under <data>/private and <data>/tmp (F-09, ADR-022)
    │   ├── testdata/corpus/        FIT corpus and golden reports (F-02, ADR-004)
    │   └── testdata/datafile/      golden data files and fixtures from synthetic files (F-03)
    └── web/                   Svelte 5 + Vite + TypeScript app (F-01)
        ├── package.json, package-lock.json, .nvmrc
        ├── vite.config.ts, svelte.config.js, tsconfig.json
        ├── index.html              dev placeholder page (F-01)
        ├── scripts/check-bundle-size.mjs   budget check (F-01, ADR-002)
        ├── test-support/golden.mjs     reads the golden data files for tests (F-04)
        ├── bench/                      dev-only timing and check pages and MEASUREMENTS.md, never built (F-04; charts.html F-05; map.html F-06)
        └── src/
            ├── main.ts                 mounts the prototype page (F-07)
            ├── prototype/              Prototype.svelte, page-query, load-error: the app entry's page (F-07; F-13 builds on it)
            ├── measure/                scrub recorder, LAZY, only via import() with ?measure (F-07, ADR-016)
            ├── lib/                    pure TypeScript: no Svelte, no DOM (range stats, units; planned)
            │   ├── datafile/           data file decoder, WorkoutData (F-04, ADR-007)
            │   ├── cursor/             cursor store, nearestIndex (F-05, ADR-008)
            │   ├── charts/             chart specs, decimation, formatting, gesture reducer (F-05)
            │   └── route/              route presence, geometry, markers, nearest-sample grid, map gesture reducer (F-06, ADR-015)
            └── components/             Svelte components
                ├── charts/             ChartStack, MetricChart, CursorReadout; the only uPlot importer (F-05, ADR-009)
                └── map/                RouteMap + touch-quiet (static); map-view, map-style (lazy); the only MapLibre importer (F-06, ADR-013)
```

##### 5. Dependency rules

**Go** (arrows mean "may import"):

```
cmd/workout-explorer ──▶ httpapi, lifecycle, publish, store, privatedir, ingest   (wiring only)
httpapi ──▶ lifecycle, reports, publish (application operations); ingest for the *ingest.Error type and kinds only (F-09)
lifecycle, publish, reports ──▶ workout, ingest, datafile; I/O through small interfaces they define
store (SQLite), privatedir, filesystem adapters ──▶ implement those interfaces (and import the package that defines them)
ingest ──▶ workout, muktihari/fit
datafile ──▶ workout (standard library otherwise, no I/O)
precompress ──▶ andybalholm/brotli only
workout ──▶ standard library only, no I/O (no net/http, database/sql, os file access)
```

- Decision logic (summary, hiding, expiry due, storage-limit check) is pure and unit-tested without disk or network; I/O sits at the edges.
- `tools/protoserve` (F-07, dev-only) imports the standard library and `internal/precompress` only; nothing imports it, and it never enters the binary.
- Only `ingest` imports the FIT library (ADR-003); the one exception is the dev-only `tools/corpusgen`, which never enters the binary. Only `precompress` imports the brotli library (ADR-006). Only `publish` writes under `public/`. Only `privatedir` writes under `private/` and `tmp/` (ADR-022). Only `store` imports the SQLite driver (`modernc.org/sqlite`, ADR-019). `lifecycle` imports neither `database/sql` nor `os`. `corpustest` is imported by `_test.go` files only.
- Standard library first; every third-party Go module needs a line of justification in the feature's design doc.

**Front end:**

- `src/lib/` is plain TypeScript with no Svelte or DOM imports, so it is tested by Vitest in Node. Components get smoke tests in jsdom (`*.dom.test.ts`, in CI, ADR-012); pixels, layout and touch stay manual.
- A `lib/` module with an `index.ts` is imported only through it (`lib/datafile`, F-04). Test helpers (`testing/`, `test-support/`) and `bench/` are never imported by app code.
- Components talk through stores (one cursor store, TD Front end), never directly to each other. The cursor store is plain TypeScript (`lib/cursor`); components subscribe and write to the DOM themselves, never through `$cursor` or other Svelte reactivity (ADR-008).
- Only `components/charts` imports `uplot` (ADR-009). `lib/cursor` imports nothing; `lib/charts` imports only `lib/datafile` (F-05 design, Module boundary).
- Only `components/map/map-view.ts` imports `maplibre-gl` as a value, and only `RouteMap.svelte` reaches it, through `import('./map-view')`; `map-style.ts` uses `import type` (ADR-013). `lib/route` imports only `lib/datafile`; static code uses only its `routePresence`. `lib/route` and `lib/charts` never import each other; map and charts meet only in the cursor store (F-06 design, Module boundary).
- `src/measure/` is reached only through `import()` from `src/prototype/Prototype.svelte`, and only with `?measure` (F-07, ADR-016). It never adds to the counted entry.
- Runtime dependencies are limited to Svelte, uPlot, and MapLibre (dynamic import, F-06). Any other runtime dependency needs a design-doc entry with its compressed size.
- Activity page bundle (ADR-011): only what every viewer needs before the map is imported statically. Map-only code, owner-only UI and the report form load with dynamic `import()`. Each feature design states its share of the 60,000 bytes and measures it.

##### 6. Cross-cutting conventions

| Concern | Convention |
| --- | --- |
| Binary | One binary, `workout-explorer`, with subcommands (`serve`; `parse <file>` as a diagnostic, F-02; later `regenerate`, `delete <public_id>` from the TD). Built with `CGO_ENABLED=0` as a static binary; a later feature that needs cgo must say so in an ADR. The SQLite driver is pure Go (ADR-019), so none does so far. |
| Configuration | **Binary:** command-line flags on the subcommand, with defaults that work for local development. **Exception (F-09):** `serve -data <dir>` is required, so the binary never creates a database in an accidental working directory; `make dev-server` passes `build/data`. No config file. **Environment variables (v1.16):** the binary reads exactly the ones listed here, only in `cmd/workout-explorer/settings.go`, once at start; each has a default, a strict format and a range, and an invalid value makes the subcommand exit 2 before it touches the data directory. Listed: `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` (F-09, ADR-021 item 4; default 4, 1–16; read by `serve`); (v1.23) `WORKOUT_EXPLORER_PARSE_TIMEOUT` (F-09 v1.11, ADR-017 rev. 7; the parser's time limit in whole seconds, default `ingest.DefaultLimits.Timeout` = 10, 5–60; read by every subcommand that builds an `ingest.Parser`: `serve`, `parse`, later `regenerate`). A new one needs a design-doc entry and a row here. **Go runtime variables (v1.18)** such as `GOMEMLIMIT` are not application settings. Our code neither reads nor sets them (no `debug.SetMemoryLimit` or `SetGCPercent`). Only the systemd unit may set them (F-31, ADR-017 rev. 3 item 5), and every measured stop rule is taken without them. **Dev tooling:** exactly one environment variable, `DEV_ADDR` (F-01 D-4), set by the Makefile and read only by `vite.config.ts` for the dev proxy target; it never reaches the binary or the build output. No `.env` files. |
| Logging | `log/slog` with the JSON handler to stdout (journald picks it up). Never log coordinates, edit tokens or file contents (I-5). Parse outcomes are logged by `ingest` only, one line per rejection or recovery, with an allow-list of keys (ADR-018); callers add only random IDs through `logger.With`. `workout` never logs. The upload path logs only the lines and keys of the F-09 design (section Logging): the internal ID may appear, never the public ID, the token or its hash, any request metadata (I-7), or any value the strip removes (I-8). |
| HTTP | Standard library `net/http` and `ServeMux` method patterns (`GET /api/health`). All API routes under `/api/`. |
| Versions | Go version pinned in `go.mod` (`go` and `toolchain` lines); Node pinned in `src/web/.nvmrc` and `engines`; npm with committed lockfile, installed with `npm ci`. CI reads both pins from those files, never from its own config. |
| Third parties in the browser | Only the tile host, without referrer or credentials (I-6, ADR-014). Pages that load the map carry `<meta name="referrer" content="no-referrer">`; F-22 sends `Referrer-Policy: no-referrer` site-wide. No other external fonts, scripts, images or analytics. |
| Build stamping | The binary carries a version string set with `-ldflags -X` from `git describe --always --dirty`; `dev` when built with `go run`. |

##### 7. Build and CI

All build logic lives in the root `Makefile`; CI only installs toolchains and runs `make -k ci` (ADR-001). The bundle budget is measured as defined in ADR-002. Target details are in [F-01](F-01-project-skeleton-and-ci.md#make-targets). Deploy (F-31) consumes `build/`.

##### 8. Not decided here

| Topic | Decided in |
| --- | --- |
| Caddy config, paths of the bundle in production, deploy script | F-31 |
| Template location and how Go and Vite share it | F-13, F-15 |
| Whether `/api/health` checks the database or disk | F-31 |
