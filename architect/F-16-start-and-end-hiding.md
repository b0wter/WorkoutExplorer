# F-16 Start and end hiding: architecture

- **Version:** 0.2 (user's decisions of 26 Oct 2026: OQ-15 notice for a fully hidden route, OQ-71 circle measure)
- **Status:** accepted (26 Oct 2026). Critical lane. No open questions
- **History:** [architect/history/F-16-start-and-end-hiding.md](history/F-16-start-and-end-hiding.md)
- **Date:** 26 Oct 2026
- **Feature:** [`features/F-16-start-and-end-hiding.md`](../features/F-16-start-and-end-hiding.md)
- **Requirements:** SH-4, AP-2, AP-3, AP-17, SH-9, SH-10, UP-4; TD Security and privacy (Hidden sections), invariant I-3
- **Builds on:** [F-15](F-15-publish-pipeline-and-static-page.md) P-4, P-7 and its F-16 hand-off; [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md); [ADR-005](ADR-005-workout-data-file-format-v1.md) items 6, 7; [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md) items 2, 3; [F-13](F-13-activity-page-layout.md) D-2, D-3, D-17; [F-06](F-06-route-map-linked-to-cursor.md) route presence and markers; [F-12](F-12-owner-api.md) O-11. No new ADR, no migration, no setting, no format change
- **Owner:** backend, plus one frontend step (FE-1) that lands first, because the Go builder reproduces its sample

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| H-1 | **Only positions are cut.** For each hidden sample, latitude and longitude become missing (always as a pair, ADR-005 item 6). Every sample and offset stays, and so does every other stream (altitude, distance, speed, heart rate, cadence, power, temperature), bit for bit as with hiding off. Sessions, laps (offsets, index ranges, totals), summary and derived values are computed in `Normalise` before the transform and are never recomputed after it. Splits and range values come from the distance and time streams in the browser, so they are unchanged too | SH-4, AP-2, AC 2, AC 3. None of these values carries a location. Metadata has no coordinates (ADR-005 item 7), so the cut touches two columns only |
| H-2 | **Measure: the 300 m circle (OQ-71).** Start: every positioned sample before the first *recorded* fix that is **> 300 m** great-circle distance (R-6b haversine) from the first positioned sample is hidden. End: the same backwards, from the last positioned sample. Later returns into a circle stay visible. The measure covers the whole workout across all sessions and sequences, not each leg, and runs on the normalised positions before quantisation. 300 m is a constant in `workout`, not a setting. If no sample lies at or after the start's first visible fix and at or before the end's last visible fix, the route is **fully hidden** | User decision 26 Oct 2026: GPS jitter near home inflates an along-track measure. The straight line is never longer than the track, so this hides at least what the along-track rule did, and ACs 1 to 7 hold. `≤ 300 m` hidden errs toward hiding. A track of 600 m or less is always fully hidden; so is one that never gets 300 m from its start or its end (a running track near home): accepted cost. A pass near the start in mid-route is phase 2's SH-3 |
| H-3 | **The boundary sits on a recorded fix.** H-2 measures over recorded fixes only, so a sample that gap filling (R-3) inserted between the last hidden fix and the first visible one is hidden too. The first and last visible positions are therefore always recorded ones. Which samples were inserted is internal to `workout` and never encoded; the engineer chooses how it is tracked | Inserted positions lie at even fractions of the straight line to the hidden fix. Two of them plus their offsets give that fix exactly. Cost: up to 3 more samples (a few tens of metres) per end. Inserted samples between two fixes inside a circle lie inside it, so measuring recorded fixes only loses nothing |
| H-4 | **Placement.** The cut is one pure function in `internal/workout`, which owns "later hiding" in the TD. It also reports whether the route is fully hidden (H-5). `publish` composes the public transform from the record's `hide_ends` at the existing seam (`internal/publish/publish.go:65`, `:227`) and is its only caller. Nothing about the cut is stored (no column, no hidden ranges, no flag), logged or cached. Every build computes it again from `original.fit` (ADR-028 item 2). It is deterministic, so the same settings give the same names | I-3: one place, before anything public exists. Store as little as possible. Determinism keeps a title save at one written file (ADR-028) |
| H-5 | **A fully hidden route shows a notice in place of the map (OQ-15).** "Fully hidden" means: `hide_ends` on, the normalised workout has at least one positioned sample, and the cut leaves none. An indoor workout with hiding on is not fully hidden; it keeps the indoor hint. The fact travels **in the markup only**: publish sets the view-model test string `Route.Hidden`, and the template prints the notice. The data file is that of an indoor workout (no position columns, ADR-005 item 6); its metadata gets no flag and the format stays version 1. Layout: one column at every width, as for the indoor hint (F-13 D-17: no map box) | User decision 26 Oct 2026. The viewing page's client never decides this: it mounts the map only into `#map-slot` (`src/web/src/page/start.ts:86`), which the template omits here. A metadata flag would cost a version bump and every golden for a fact the page already states. Only "fully hidden" is revealed; partial hiding leaves no trace (H-6) |
| H-6 | **What each public file gets.** `data.{hash}.bin`: sentinels in the hidden positions, or no position columns (H-5). Its metadata equals the hiding-off metadata, except for the position descriptors when the route is fully hidden. `.br`: the same bytes, compressed. `index.html`: built from the decoded public metadata, the settings (F-15 P-4) and the fully-hidden fact, so it equals the hiding-off page except for the data file name, and the notice instead of the map box when the route is fully hidden. Preview (F-23) and export (F-24): built from the transform's output only | AC 1 holds by construction for `index.html` and the metadata: no input of theirs holds a coordinate |
| H-7 | **Geometry follows the cut, totals do not.** Everything computed from positions uses public positions only: the bounding box and map fit, the start and finish markers (first and last visible fix), lap markers (none in a hidden section, F-06), route colouring, the preview route, the "has GPS" test in SH-5, and a future UP-5 title suggestion. Totals keep the full workout (AP-2) | Scope ("derived after the cut"). F-06 places markers only on positioned samples, so markers in hidden sections are not published without extra code |
| H-8 | **The edit page always sees everything.** `draft.bin` is written once at upload from the full parse (F-09). `hide_ends` and publish never touch it. `hide_ends` affects only `public/`. The edit page's view model never sets `Route.Hidden` | UP-4: the owner edits with the full route. `draft.bin` is served only with the token (F-12) |
| H-9 | **Changes use the existing rules, with no new code path.** Off → on is not on P-7's allow-list, so it is remove-first (`publish.go:186`; AC 10). On → off is in place, and the positions come back from `original.fit`, so nothing hidden needs keeping. `PATCH` on a draft only stores the setting. Publish with hiding on cuts in the first build. F-30 regeneration runs the same build. If a later release changes H-2 or H-3 to hide more, the old file still goes in that same in-place rebuild (ADR-028 item 3), well inside SH-10's 60 s | ADR-028 items 2 to 4. SH-10 on failure paths stays F-21's per-minute sweep (F-15 hand-off) |

**Not hidden, by the spec.** That a section is hidden is not revealed, except a fully hidden route (H-5). Its length (distance stream), elevation, speed, heart rate and timing are public (AC 2, SH-9 "Totals include them").

## FE-1 Frontend step (senior-frontend-engineer)

Decided; details are the engineer's.

- **View model.** `Route` gets a third test string, `Hidden` (F-13 D-2). At most one of `Map`, `NoLocation`, `Hidden` is set. `lib/page` takes "fully hidden" as an input next to the fixed settings (it cannot be derived from the data file) and sets `Hidden` only when the presence is `no-location-data`; otherwise `NoLocation` as today.
- **Template.** `{{if .Route.Hidden}}` prints the notice in the hint's place, with the hint's element and class, as a literal (like the privacy notice, F-13 D-15): "The entire route has been hidden by the uploader." No map box, no `has-map` class, so one column (F-13 D-17). `ActivityPage.svelte` mirrors the branch (F-13 D-9).
- **Sample.** One new sample in `src/server/testdata/page/` with `Hidden` set (F-13 D-3), so the Go builder and the B-1 template test cover it.
- No client code path changes; the bundle share is unchanged.

## Verification

Synthetic files only (ADR-004). The critical lane adds the property and mutant checks.

| AC | Check |
| --- | --- |
| 1, 7 | Measured on the full track from the parse, never re-measured on quantised public coordinates. Every public positioned sample lies at or after a recorded fix > 300 m from the first position, and at or before one > 300 m from the last; none is an inserted sample next to a hidden fix (H-3). Property or fuzz run on generated tracks: jitter around the start, loops, out-and-back, a mid-route pass near the start, dropouts and filled gaps across the boundary, a late fix, ≤ 600 m, a single fix, several sequences. Then the same over every synthetic corpus file. `index.html` and the metadata compare equal to hiding off as stated in H-6. `.br` decompresses to the `.bin` |
| 2, 3 | Hiding on and off: offsets and every non-position column are bit-identical. The summary and laps in the metadata are identical |
| 4 | Hiding off: public positions equal `draft.bin`'s |
| 6 | 5 km of laps on a 400 m track: fully hidden, no position columns, the same notice. `TestHideEndsFullyHidden` ("400 m track, 5 km") and `TestHidingFullyHidden` |
| 5, 8 | 500 m walk: no position columns, `Route.Hidden`, the notice in `index.html`, no map box. Indoor file with hiding on: `Route.NoLocation`, the indoor hint |
| 9 | Late fix: the circles are centred on the first and last positioned sample, not on sample 0 |
| 10 | F-15's remove-first fault injection, run with an off → on change. After any stop, the folder has no `index.html` or the new complete page. An old data file left by a stop is unreferenced, and the sweep removes it (F-15 P-8). After a full run, no old data file is on disk |
| Mutants | Each must fail a test: `<` for `≤`, the circle centred on sample 0, the end side dropped, H-3 dropped, along-track distance instead of the circle, every in-circle sample hidden (not only until the first exit), the transform skipped on rebuild, `Hidden` set for an indoor file |

## Hand-offs

| To | What |
| --- | --- |
| F-19 | The edit page shows the full route (H-8). Hidden sections are shown from the Go rule, the single definition; no TypeScript copy decides what is hidden. A display that is off by one sample at a boundary is acceptable; the public build alone decides what is public |
| F-20 | The SH-10 recall notice applies when hiding is turned on after publishing |
| F-23 | Preview route, bounding box and SH-5's "no GPS" choice come from the transform's output. A fully hidden route draws like a workout without GPS |
| F-24 | The export is written from the transform's output only. Copy no position-bearing field from the original (session or lap start and end positions, bounding-box fields, location or course messages). Only record positions from the output |
| Requirements | TD Security and privacy (Hidden sections): the 300 m circle (H-2, H-3) replaces "along the track"; a fully hidden route shows the notice (H-5), the one exception to "not revealed". Feature file: AC 1 and README assumption 14 to the circle measure, AC 5 to the notice and one column; Owner to both (FE-1) |
