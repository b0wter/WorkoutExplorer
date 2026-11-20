# QA F-57 GPX route download - round 1

- **Date:** 2026-11-20
- **Verdict:** ready with conditions. No Blocker or Major. The GPX built from Go-published files holds exactly the public positions, with nothing from hidden or trimmed ends and no times (480 builds, see below). Conditions: the device checks in "What the user should check by hand" (AC 2 load on a device, AC 9 on the reference phone), which no test can cover.
- **Inputs:** `features/F-57-gpx-route-download.md`; `architect/F-57-gpx-route-download.md` v0.1 (GX-1 to GX-9); ADR-011 item 4, ADR-032; SH-13, SH-4, SH-8, AP-12, INF-1 in the product spec; TD module boundary section. Code: backend `2fd1dce`, frontend `1b72eb7` (diff `f5ff6f8..HEAD`), clean working tree.

## Build and test results

- `make ci` at HEAD: **exit 0**. Go: every package ok. Vitest: 77 files, **1,983 tests passed**. Size: `entry index.html: 57603 / 60000 bytes gzip PASS` (F-57 +147 against the ceiling of 150, as booked in ADR-011); `assets/gpx-*.js` 990 raw, **651 gzip**, listed under "dynamic imports (not counted)". caddy-check, docker-check, update-check ok.
- `go test -race -count=1 ./internal/page ./tools/protoserve`: ok.
- The built entry `main-*.js` holds only the enhancer and the `import()` call. `trkseg` occurs only in `gpx-*.js`.
- **Real publish path (scratch copy of the repo; repo untouched).** A Go test in the scratch copy published through `Area` (`WorkoutChanged`, the real template, `ingest` parser) 20 sources: ride-30min, run-30min, gps-dropout, pause-hours, timer-pause, chained-ride-ride, ride-transition-ride, triathlon, brick-run-bike, indoor-no-gps, pool-swim, rolling-hills, smart-recording, no-distance-with-gps, enhanced-and-legacy, climb-100m, ride-4h, ride-12h, plus three generated walks: one starting 23:50 local (UTC+2), one in the southern and western hemisphere with negative altitudes (UTC-5), and a 500 m walk. Each was published at radii 0/0, 0/500, 2000/0, 300/300, 50/1950 and 2000/2000, with no trim, a start trim, an end trim and both: **480 builds**. Go wrote the expected points from the stored integers (its own formatting), the hidden set (every position of the untrimmed, unhidden build that is not public) and the expected file name (`start_time` plus the known offset).
  - jsdom: each Go `index.html` and its data file went through the real `startPage`, the real lazy `src/gpx` chunk and a real button click. **480/480 pass.** The button was present in exactly the 321 cases with a public position and a non-multisport sport, and absent in 159 (indoor, pool, triathlon, brick, ride-transition-ride, fully hidden at any radius). The single-sport chained-ride-ride has the button. In every file the points equal Go's list exactly (order, 6 and 1 decimals, `<ele>` only where altitude is stored). Apart from the point lines, each file is exactly the fixed head and tail. No hidden position occurs anywhere. Two clicks gave byte-equal files. The page fetched only the data file, once, and nothing on click. The blob type is `application/gpx+xml`.
  - File names: the midnight walk with a 2,000 m hidden start has its first visible point at 00:13 local the next day. Its file is `route-2026-06-01.gpx`, the header's date. The UTC-5 walk (01:30 UTC = 20:30 local the previous day) gets the local date.
  - Control: with the call site changed to take the date from the first positioned sample (W4 below), the probe fails on all 8 midnight-walk cases with a 2,000 m start radius.
- **Schema (AC 2):** the official XSD is not on this machine and I did not download it. I wrote a subset of the GPX 1.1 schema (`gpxType`, `trkType`, `trksegType`, `wptType`, `latitudeType`, `longitudeType`, with element order and required attributes). `xmllint --schema` accepted **321/321** files and rejected a control file with `lat="91"` and a bad `<time>`.
- **Mutants** (scratch copy, one at a time; web suites `lib/gpx`, `gpx`, `page`, `lib/page`, `edit.dom`; Go `internal/page`, `tools/protoserve`, `internal/publish`): 21 of 23 killed. Killed: builder writes `<time>`; NaN altitude as 0; `String(x)` coordinates; new segment per gap; call site also on the edit path; TS `Route.Gpx` for multisport; TS `Route.Gpx` for a fully hidden route; whole `WorkoutData` spread into the input; revoke in the click task; `Math.trunc` instead of rounding; offset applied with an unknown zone; `data:` URL; button never revealed; edit line removed; `<ele>` with 2 decimals; last chunk dropped; Go `Route.Gpx` for multisport; Go `Route.Gpx` for fully hidden; template row on `Route.Map`; template row always printed. **Survived:** W4, the date taken from the first positioned sample at the call site (QA-57-r1-2), and W11, a static import of the chunk (QA-57-r1-3).

## Findings

### Minor

- **ID:** QA-57-r1-1
- **Severity / kind:** Minor / defect (wording)
- **Route:** frontend; requirements for the About sentence if wording changes
- **Where:** `src/web/src/components/edit/EditPanel.svelte:184`; `src/web/about.html:54`
- **Relates to:** AC 10, OQ-271, SH-13, AP-12
- **Evidence:** The hiding sliders, and with them the new line, show on every draft, including indoor, pool and multisport. On a multisport workout the line says "Viewers can download the visible route as a GPX file", but `Route.Gpx` is empty and viewers get no download (probe: triathlon, brick-run-bike and ride-transition-ride have no button). The About sentence makes the same unconditional claim.
- **Consequence:** For a multisport workout the owner is told about an exposure that does not exist. This overstates rather than understates, so it does no privacy harm, but the text is wrong for that case. On indoor and pool workouts the line is moot rather than false. Suggested fix: show the line only when the draft has positions and the sport is not `multisport`. Do not make it depend on the slider positions: a fully hidden route is covered by "visible route", and the line should not flicker while dragging. The AC text is unconditional, so see Question 1.

- **ID:** QA-57-r1-2
- **Severity / kind:** Minor / risk (test gap, privacy-relevant)
- **Route:** frontend
- **Where:** `src/web/src/page/page.dom.test.ts` (F-57 block); `src/web/src/lib/gpx/gpx.test.ts` "takes no sample"
- **Relates to:** GX-4, AC 6, SH-4; design Verification "Mutants: date from the first positioned sample"
- **Evidence:** Mutant W4 sets `startTime` at the call site to start plus the time of the first positioned sample. The repo suites pass. The call-site test uses ride-4h, whose first sample has a position, so both values are the same string. The builder's closed signature (`gpxFileName.length === 1`) does not guard the call site, which is where a sample time could enter. The Go-published midnight-walk probe kills W4.
- **Consequence:** A later refactor could name the file after the first visible sample. With a hidden start across midnight, that would reveal that the start is hidden and roughly how long it is. Suggested fix: one call-site test with a data file whose start is hidden across local midnight, asserting the clicked file name.

- **ID:** QA-57-r1-3
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/page/enhance.ts:223`; `src/web/scripts/check-bundle-size.mjs`
- **Relates to:** AC 8, GX-6, ADR-011
- **Evidence:** Mutant W11 imports `../gpx` statically. All tests pass, and `make size` passes at 58,192 / 60,000 (+589 B, the builder moves into `main-*.js`). Only someone reading the ADR-011 ledger would notice.
- **Consequence:** AC 8 ("not in the entry bundle") has no automated guard. Suggested fix: a hygiene assertion that no non-test module outside `src/gpx` imports `lib/gpx` or `src/gpx` as a value, or a size-script check that the `gpx` chunk stays dynamic.

- **ID:** QA-57-r1-4
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/lib/gpx/gpx.test.ts`
- **Relates to:** AC 2, AC 4; design Verification table ("over published synthetic samples", "schema-valid")
- **Evidence:** No repo test validates against the GPX 1.1 schema. The AC 4 and AC 5 tests apply hiding and trim through a TypeScript re-implementation (`publicOf`: symmetric trim, its own distance rule), not through files the server publishes. The code is correct: this round's 480 Go-published builds and the schema subset check show it. The repo, however, does not keep that proof.
- **Consequence:** Little risk today, because the builder is a pure function of the public file and Go's hiding has its own tests. `publicOf` does suggest that the server's hiding is covered here when it is not. Suggested fix: either a test-time schema check, or a note in the test that the exact-shape assertion stands in for the schema; and either rename `publicOf` as a stand-in or use a committed Go-published golden.

- **ID:** QA-57-r1-5
- **Severity / kind:** Minor / defect (documentation hand-off)
- **Route:** main session, then requirements
- **Where:** `spec/Workout Explorer — Technical Design.md`, module boundary list (lines 370-378)
- **Relates to:** design Hand-offs ("TD Code structure: `lib/gpx` and the lazy `src/gpx/`; view model `Route.Gpx`")
- **Evidence:** The TD does not mention `lib/gpx` or `src/gpx`. The code follows the intended rule (`lib/gpx` imports nothing; `src/page` imports it as a type only and reaches `src/gpx` only through `import()`), but the rule is not written down.
- **Consequence:** A later reader has no stated boundary to check QA-57-r1-3 against.

- **ID:** QA-57-r1-6
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/page/page.css:314`
- **Relates to:** DS-4
- **Evidence:** `.gpx` uses `color: var(--bg)` on `var(--accent)`. The page's other accent-filled control (`.map-pin[aria-pressed]`) uses `--on-accent`. Contrast is fine today (5.1:1 light, 7.6:1 dark).
- **Consequence:** A future theme change to `--bg` would change the button text by accident.

### Engineer's deviations

1. Blob from about 45 parts of 1,024 lines: **accepted.** GX-7 asks for string parts, never one joined string. The 1,024-line parts keep that intent and avoid a slow Blob from tens of thousands of parts. Flushing is keyed to the sample index, so a part can grow across a long dropout. That costs nothing.
2. Call site spreads `data.position`: **accepted.** The chunk reads only `latitude`, `longitude` and `altitude`, and the 5-key test kills the "whole `WorkoutData`" mutant (W9).
3. Edit-panel line on every workout: **reported** as QA-57-r1-1. It should follow "has positions and is not multisport". It should not follow the live `Route.Gpx`, because that would change with the sliders.
4. F-49 test adapted (`.edit-small` first match changed to "contains"): **accepted**. A second `.edit-small` now comes earlier in the form, and the text check stays exact.
5. Entry +147 of 150, chunk 651 gzip: confirmed by `make size`.

## Previous findings

None (first round).

## Questions

1. **QA-57-r1-1:** should the edit line (and the About sentence) say nothing on multisport, indoor and pool workouts? Suggestion: show the edit line only for a draft with positions that is not multisport. Leave the About sentence as it is, or add "except multisport workouts". Answer: yes or no to each.
2. **Revoke delay (GX-7):** the object URL is revoked after 10 s. On iOS Safari the download prompt can stay open longer. FileSaver.js uses 40 s for this reason. Is 10 s enough? Hand check 2 below settles it. If it fails, raise the delay (small lane).

## What the user should check by hand

1. AC 2: on a published GPS ride, tap "Download route (GPX)" on Android Chrome, iOS Safari, desktop Firefox and Chrome. The file should be named `route-YYYY-MM-DD.gpx` with the header's date. Load it into at least one device or route app you use (for example Garmin Connect course import, Komoot, OsmAnd, Wahoo). The route should match the map's visible line, with no hidden ends.
2. iOS Safari: wait more than 10 s at the download prompt before confirming. The file should still arrive.
3. AC 9: on the reference phone, open the 12-hour ride (ride-12h) and download. It should take under 2 s, including the first-click chunk load, and the page should not freeze. Only Node was measured (engineer: about 49 ms).
4. Look: the button above the summary in light and dark themes and at phone width. No jump when the page loads (the row is reserved). With JavaScript off, an empty 48 px row and no button.
5. A multisport page, an indoor page and a fully hidden route: no button. The edit page: no button, and the line under the sliders.
6. An in-app browser (messenger): an accepted limit (GX-7), so it may do nothing, but the page must not break.

## Coverage

| AC | Code | Test / check |
| --- | --- | --- |
| 1 Button where shown, absent elsewhere | `index.html:44`, `page.go:151`, `view.ts:84`, `start.ts:215`, `enhance.ts:223` | `samples_test.go`, `page_test.go` (Go = TS samples), `hygiene.test.ts`, `page.dom.test.ts` (edit seam, failed load, no positions), `edit.dom.test.ts`; probe 321 shown / 159 absent; mutants W6-W8, G1, G2, T1, T2 killed. The published page does not run `startPage` (`edit.html` has no row) |
| 2 Valid GPX 1.1, loads on a device | `lib/gpx/index.ts` | Exact-shape test; this round: 321 files valid against a transcribed schema subset (not the official file). Device load: by hand (1) |
| 3 Exactly the public positions, ele where recorded, nothing else | `gpxParts` | `gpx.test.ts`; probe 480 builds; mutants W1-W3, W12, W17, W18 |
| 4 Trim + both ends hidden: nothing hidden, first and last visible | public data file (F-16, F-47, F-55) + `gpxParts` | probe (6 radius pairs x 4 trims, hidden-set check); repo test via `publicOf` only (QA-57-r1-4) |
| 5 One trk/trkseg, joined | `gpxParts` | `gpx.test.ts` gps-dropout, pause-hours; probe; W5 killed |
| 6 File name | `gpxFileName`, `start.ts:215` | `gpx.test.ts`, `gpx.dom.test.ts`; probe (midnight walk, UTC-5); call-site gap QA-57-r1-2 |
| 7 No workout-data request, nothing new on the server | blob download, no endpoint | `gpx.dom.test.ts` (no fetch), `page.dom.test.ts` (one fetch); probe (one fetch per page incl. two downloads); diff: no `httpapi`, Caddy or log change |
| 8 Lazy, entry within budget, LCP | `import('../gpx')` | `make size` 57,603/60,000, chunk dynamic; laziness unguarded (QA-57-r1-3); LCP not measured (row reserved in server markup) |
| 9 12-hour ride < 2 s, no freeze | single pass | Node only (`gpx.test.ts` bound 500 ms); phone by hand (3) |
| 10 Edit line and About sentence | `EditPanel.svelte:184`, `about.html:54` | `edit.dom.test.ts`, `about.test.ts` (exact text); condition QA-57-r1-1 |

Not run: a real browser or device, the official GPX XSD (not available offline, not downloaded), `-race` on `internal/publish`, LCP measurement. The probe and mutant scripts stay in the scratchpad only: `qa57_probe_test.go.keep`, `qa57.dom.test.ts.keep`, `mutants.py`, `gpx11-subset.xsd`.
