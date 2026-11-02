# F-24 Viewer download of the published data: architecture

- **Version:** 0.1
- **Status:** accepted (2 Nov 2026). Critical lane: hidden positions (SH-4), UP-9 data, originals never public. OQ-93, OQ-94 open; neither blocks B-1 (the suggestions are built unless the user objects)
- **Date:** 2 Nov 2026
- **Feature:** [`features/F-24-viewer-download.md`](../features/F-24-viewer-download.md)
- **Requirements:** SH-8, SH-10, SH-4, UP-9, NFR Security (originals never public); TD Publish and view flow (Publish 5), Security and privacy (Originals), Workout data file (Versioning); invariant I-3
- **Builds on:** [F-15](F-15-publish-pipeline-and-static-page.md) P-4, P-5, P-7, P-9 and its F-24 hand-off; [F-16](F-16-start-and-end-hiding.md) H-1, H-6 and its F-24 hand-off; [F-23](F-23-preview-card-and-indexing.md) PC-1, PC-6 (the artefact pattern); [F-22](F-22-caching-and-security-headers.md) H-6, H-11, H-12; [F-12](F-12-owner-api.md) O-9 (file name rule); [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md), [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md), [ADR-032](ADR-032-response-headers-in-caddy.md); new: [ADR-036](ADR-036-viewer-export-encoded-from-the-public-data-file.md)
- **Owner and order:** backend. **B-1** (`internal/fitexport`, the artefact, the `Download` group, Caddy headers), then **FE-1** (one template element and its CSS; the main session may give it to the backend engineer, as F-23 FE-1). **Bundle:** 0 bytes on every ADR-011 / ADR-024 line (markup and CSS only)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| DL-1 | **Input: the decoded public data file only.** The export is encoded from the same `datafile.File` the page and the preview are built from (`internal/publish/publish.go:290`), never from `workout.Workout`, the record's other fields, or `original.fit`. Encoder and package: ADR-036 | The export can hold only what `data.{hash}.bin` already publishes: no coordinate outside the position columns, no device, serial number, sensor or event (F-03 §5.6). AC 3 and AC 4 hold by construction, and hiding needs no second path (F-16 H-6) |
| DL-2 | **Hidden sections: positions only.** Every sample of the data file becomes one `record`, hidden samples included, without position fields where the data file has none. Gap-filled samples are exported like any other. A fully hidden route gives records without positions | F-16 H-1: totals, laps and every other stream keep the full workout (AP-2, SH-9 "Totals include them"), so records, laps and sessions stay consistent with each other and with the page |
| DL-3 | **Closed message list:** `file_id`, `record`, `lap`, `session`, `activity`, in the usual device order (records in sample order, each lap after its last record, sessions, then `activity`). Nothing else: no `event`, `device_info`, `sport`, `length`, `file_creator`, zones, HRV or developer data. **No position-bearing field outside `record`** (lap and session start and end positions, bounding boxes). `file_id`: type activity, manufacturer `development`, product 0, `time_created` = the start time; **no serial number** (OQ-93). No title or description | Minimal, what a FIT tool needs to open an activity. Title saves then never change the export |
| DL-4 | **Values and consistency.** Records: timestamp = `start_time` + time offset; the present streams in their FIT fields, `enhanced_altitude` and `enhanced_speed` only (no legacy duplicates); `moving_time` and `elevation_gain` are not exported. Sessions and laps: the metadata's offsets and totals as written; an absent total is not written, never 0. The data file's unit conventions are converted back to FIT's where they differ: running, walking and hiking cadence is halved again (F-02 R-5, inverted exactly with `fractional_cadence`), using the covering session's sport, else the workout sport. **Fallbacks** so every file is a valid activity: no sessions → one session over all samples with the `summary` totals and the workout sport; a session with no lap inside → one lap with the session's span and totals. `activity` carries `num_sessions`, the end timestamp, and `local_timestamp` only when the time zone is known | One source of truth: the numbers a FIT tool shows are the ones the page shows (device totals, AP-2). No total is recomputed |
| DL-5 | **Artefact rules as F-15.** Written only when `allow_download` is on and the workout is unlisted. Name `export.{hash}.fit`, `{hash}` as P-5 (first 16 hex of SHA-256 of the bytes), so F-22 H-6 already caches it `private, immutable` and an unchanged export is not rewritten (ADR-028 item 2). Deterministic per binary. Encoded outside the public lock, like the preview. No `.br` (downloads are rare; Caddy's `encode` compresses on the fly). Counted through `Storage` like every public file (P-9). No second parse at build time: the export holds only public data, so a defect is a quality bug, caught by the tests | Same rules as every artefact; nothing new to sweep or remove |
| DL-6 | **Link and file name.** View model group `Download` `{URL, FileName}`, `null` when off. `URL` = `/w/{public_id}/export.{hash}.fit`, printed as a plain `href` ending in the file name, so ADR-028 item 5's sweep keeps the file. `FileName` = `lifecycle.DownloadName(r)` (`workout-YYYY-MM-DD.fit` from the local start date, else `workout.fit`; the owner's rule, OQ-59), printed in the `download` attribute. **Built in Go only:** `lib/page` always emits `Download: null`; the samples gain that key and stay equal with download off; the on case is a Go test (as F-23 PC-6). The edit page never shows it (the owner has the original, F-20) | Works with JavaScript off; no client code. The saved name never reveals the title |
| DL-7 | **Headers (Caddy, ADR-032).** For `/w/{public_id}/export.{hash}.fit`: `Content-Disposition: attachment` without a file name (the `download` attribute names the file; a pasted URL saves under the hashed name) and `Content-Type: application/octet-stream`. Caching and the H-2 set already apply. F-22's H-12 check asserts both | A FIT file opened in a tab is useless; one header line per name pattern |
| DL-8 | **Rebuild and removal: no new path.** Off → on is on P-7's allow-list: in-place build, the link and file appear together. On → off is not: remove-first, so the old export URL answers the generic `404` before the `PATCH` returns (already in `publish.go:245`). Hiding on with download on: remove-first, the new export from the new data file. Delete and expiry remove the folder. A stopped build leaves an unlinked export, which the per-minute sweep removes (F-21 X-5). SH-10's 60 s therefore rests on F-22 H-11's chain. Accepted exceptions, already decided: copies viewers downloaded and the viewer's own HTTP cache (SH-10 recall notice, F-20), Go down (OQ-87) | ADR-028 items 3 to 5 |
| DL-9 | **F-30 regenerate** calls the same build and needs nothing F-24-specific: the export follows `allow_download` and the new data file; a library upgrade that changes the encoder's bytes gives a new name and the old file is removed. A kept workout keeps its old export, which depends on no bundle | ADR-028 item 2; TD Versioning |

## Verification (critical lane: QA at effort high)

Synthetic files only (ADR-004).

| AC | Check |
| --- | --- |
| 1 | Default publish: no `export.` file in the folder, no `download` link in `index.html` |
| 2 | Every synthetic corpus file, hiding on and off: the export parses with `ingest.Parser.Parse` with no integrity notice; `Accept` strips nothing; the parsed streams equal the public data file's within FIT resolution (altitude 0.2 m); two builds give equal bytes. Third-party tool: QA opens one synthetic export (with laps and positions) in a FIT tool of its choice and records which |
| 3 | Hiding on: every positioned record matches a positioned sample of the public data file at the same timestamp; a field enumeration of the parsed export finds only DL-3's messages and no position field outside `record`. Fully hidden: no position field at all |
| 4 | The export differs from `original.fit`; no `file_id` serial number, no `device_info` |
| 5 | On → off: the link and the file are gone after the `PATCH`; through Caddy (H-12) the old URL answers the generic page or `404` within 60 s. Build then sweep keeps a linked export and removes an unlinked one. F-15's remove-first fault injection with an on → off change |
| Headers | H-12 asserts DL-7 and `private, immutable` on the export |
| Mutants | Each must fail a check: export from the workout before the transform; `file_id` copied from the original; a lap or session start position written; on → off moved onto the allow-list; link printed without the plain path; running cadence not halved; a missing total written as 0 |

## Hand-offs

| To | What |
| --- | --- |
| FE-1 (template) | `{{with .Download}}` an `<a>` with `href` and `download` from the group, styled as a button, in the header next to the copy-link button; label per OQ-94. `lib/page` adds `Download: null`; regenerate the samples. No script, no `style` attribute |
| F-22 | DL-7's two headers in `deploy/Caddyfile` and their assertions in H-12 |
| F-30 | DL-9. Its AC "download settings preserved" checks that the export exists exactly when `allow_download` is on |
| F-31 | Include one export encode of `ride-12h` in the `MemoryMax` measurement (ADR-036 bounds it) |
| Requirements (TD, README) | **TD Code structure:** Publishing row "export file via `internal/fitexport` (F-24, ADR-036)"; repository layout gains `internal/fitexport/`; dependency rules: "`publish ──▶ … fitexport`", "`fitexport ──▶ datafile, muktihari/fit` (encoder and profile packages), no I/O", and "Only `ingest` and `fitexport` import the FIT library (ADR-036)". **TD Security and privacy, Originals:** "…`export.{hash}.fit`, encoded from the public data file only (F-24 DL-1)". **Glossary:** new term *Export*: "The viewer download `export.{hash}.fit`, encoded from the public data file only, written while `allow_download` is on (F-24, ADR-036)"; not to be confused with Original. **README assumption 23:** per OQ-93 |

## Open questions (product; the user answers by number)

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-93 | README assumption 23 lists manufacturer and product in the export. DL-1 builds only from the public data file, which holds no device, and the MVP page shows none (AP-14 is phase 2). Drop them? | **Yes:** `manufacturer` `development`, product 0. Alternative: copy both from the original's `file_id`, a second input next to the data file | DL-3 detail only |
| OQ-94 | Label of the download button (spec silent) | "Download FIT file" | FE-1 wording only |
