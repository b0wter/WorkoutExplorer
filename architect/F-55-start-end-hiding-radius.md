# F-55 Selectable radius for start and end hiding: architecture

- **Version:** 0.1
- **Status:** accepted (19 Nov 2026). Critical lane (location privacy): QA at effort high. No open questions
- **Feature:** [`features/F-55-start-end-hiding-radius.md`](../features/F-55-start-end-hiding-radius.md); decisions OQ-246, OQ-248, OQ-249, OQ-268
- **Requirements:** SH-4, SH-9, SH-10, SH-12, SH-5, AP-3, AP-19, AP-24, UP-4, UP-5, UP-6, UP-7, AC-4, AC-5, DS-1; invariants I-3, I-5
- **Builds on:** [F-16](F-16-start-and-end-hiding.md) H-1 to H-9; new [ADR-056](ADR-056-hiding-steps-for-every-radius.md) (supersedes [ADR-030](ADR-030-to-be-hidden-ranges-computed-at-upload.md) items 2, 3); [F-15](F-15-publish-pipeline-and-static-page.md) P-4, P-7; [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md); [F-47](F-47-trim.md) TR-2, TR-6, TR-8, TR-9; [F-12](F-12-owner-api.md) settings object; [F-19](F-19-edit-page-and-settings.md) E-6, E-9; [F-30](F-30-regenerate-command.md) RG-4, RG-6; [F-36](F-36-preview-image-with-map-context.md) MC-3, MC-8; [F-45](F-45-suggested-title.md) ST-4, ST-6; [F-58](F-58-summary-explained.md) SE-4; [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md). No data file format change, no setting
- **Owner:** both. Backend first (HR-1 to HR-7), then frontend (HR-8, HR-9). The owner API's `hide_ends` disappears, so the current edit page breaks against B-1: both land before the next release, `make ci` green at each commit

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| HR-1 | **Storage: two settings columns `hide_start_m`, `hide_end_m`**, integer metres, `NOT NULL DEFAULT 0`, `CHECK` 0 ≤ v ≤ 2,000 and v % 50 = 0. One forward-only migration (schema v9, ADR-019; **one-way door**): rows with `hide_ends = 1` get 300 and 300, all others 0 and 0; `hide_ends` and ADR-030's four range columns are dropped; one nullable column `hiding_steps` is added (ADR-056), `NULL` until the next `regenerate` fills it. The step (50) and the maximum (2,000) are constants in `workout`, the one home of the rule; `lifecycle` validates against them, the `CHECK` is the second guard | Feature scope, AC 41. Metres, not step numbers: the stored value means the same thing if the steps ever change. A deploy migrates in `serve`, then runs `regenerate` (TD Versioning), so the `NULL` window is the deploy itself |
| HR-2 | **The rule per end.** Radius 0: that end is **not cut at all** (identity), never a 0 m circle, which would hide the first fix. Radius r > 0: F-16 H-2 and H-3 with r in place of 300 m (≤ r hidden, measured from the first or last positioned sample, recorded fixes only, before quantisation, after trim, TR-6). **Fully hidden** (F-16 H-5): at least one radius above 0, the workout has positions, and the cut leaves none. **(300, 300) is today's hiding on and (0, 0) today's hiding off, byte for byte** | AC 34, AC 37, AC 41. Equality with today's builds makes the migration invisible on public pages: `regenerate` writes the same names |
| HR-3 | **One measure, all steps** (ADR-056). A pure function in `workout` computes, in one pass per end, the boundary for every radius 0, 50, …, 2,000: the first (start) or last (end) sample whose position stays public, `-1` when none does; entry 0 is the window's first or last sample. The public cut takes its two boundaries from that result for the record's radii, through one Go combination (fully hidden when an entry is `-1` or start > end). There is no second measure anywhere | ADR-030 item 1's "cannot drift", extended to every step. One pass keeps the cost O(n) per end, whatever the step count |
| HR-4 | **Owner API** (F-12 settings object, edited there). `hide_start_m`, `hide_end_m`: integers, in `PATCH`. A value below 0, above 2,000, not a multiple of 50, fractional or not a number gets `400 invalid_settings` with the ordinary sentence, and nothing is stored (O-3, all or nothing). `hide_ends` and `hide_ends_ranges` are removed; a `PATCH` sending either gets `400` like any unknown field. Read-only **`hiding_steps`**: `{"start": [41 integers], "end": [41 integers]}` in `draft.bin` sample indices, entry k for radius 50·k m, or `null` without positions or while the row has none. With a trim stored, computed per read on the trimmed workout (TR-8) and shifted by `a`; `-1` stays `-1` | AC 33. Metric on the wire (OQ-268). The table, not ranges for the saved radii, because the shading must follow unsaved slider values (OQ-249) |
| HR-5 | **Hides more, per end** (F-15 P-7, edited there). `HideStartM` and `HideEndM` leave the plain allow-list: a change where either value **rises** (from 0 too) is remove-first; equal or lower values are in place. One save that raises one and lowers the other is remove-first. `HidingSteps` joins RG-6's derived fields the public build never reads (replacing `HiddenRanges`). The rule compares fields only, never the cut: a raise that happens to hide nothing more is still remove-first | OQ-246, AC 38. Lowering can only add public positions (the boundaries are monotone in the radius), and the town and frame it changes derive from positions that stay public. A pure field rule needs no parse and fails toward less public content |
| HR-6 | **Public side: no new path.** The radii are inputs of the public transform only (`publicTransform`, `internal/publish/publish.go:158`): never a view-model input, never in the metadata, the page, the preview, a log line or an error. Everything that follows hiding today (data file, fully hidden notice, preview route and frame with key match MC-3, suggested title ST-4, start, finish, lap and leg markers, range links) follows the radii through the transform's output, unchanged | AC 39, AC 40 by construction: no public artefact has an input that holds a radius. Partial hiding stays unmarked (F-16 H-6), whichever end is cut |
| HR-7 | **Upload and `regenerate`.** The upload computes `hiding_steps` where it computed ADR-030's ranges; `regenerate` writes it with the other derived columns (RG-4(b)) and never writes the radii (settings columns). The owner's suggested title (ST-6) skips the hidden sections at the **stored** radii, taken from `hiding_steps` by the same combination as HR-3; with a radius above 0 and `hiding_steps` still `NULL`, the suggestion is `""` (fails toward no town) | A hidden start never names its town, also in the deploy window. Before `regenerate` the suggestion drops only the town, as for indoor files (QA-55-r1-2) |
| HR-8 | **Edit page (frontend, owner chunk only).** Two range inputs, Start and End, 0 to 2,000, step 50, in place of the toggle; filled from the saved settings, new uploads 0 (F-09 defaults; nothing in browser storage, OQ-248). **Labels** follow the units store through one formatter in `lib/format` (OQ-268: feet below 0.1 mi rounded to 10 ft, miles with two decimals from 0.1 mi; "not hidden" at 0), also used by F-58's dialog. **Shading** follows the slider values at once, unsaved too: the hidden ranges are `hiding_steps.start[s/50]`, `hiding_steps.end[e/50]` and HR-3's one comparison, the browser's only hiding logic (a display mirror, like TR-4's; it never measures). Shown greyed on the map through E-9's marks path and on the charts through the decoration seam the trim already uses (MS-7, TR-9), at most once per animation frame. With `hiding_steps` `null`, nothing is shaded and nothing fails (indoor, feature assumption). Saving: E-6 unchanged, only changed radii sent. MC-8(b)'s basemap job runs after a save that changed either radius. Summary, title placeholder and dialog follow the **saved** values | OQ-249: the public page changes only after Save; the server alone decides what is public (F-16 hand-off). `hiding_steps` follow the saved trim, so while the unsaved trim differs and a radius is above 0 the panel says "Save to update the hidden sections." (QA-55-r1-3) |
| HR-9 | **F-58 dialog.** `radii()` (`src/web/src/edit/start.svelte.ts:52`, F-58 SE-4) returns the saved `[hide_start_m, hide_end_m]`; the radius text uses HR-8's formatter. The viewing page's dialog keeps its general line only (SE-1, SE-3) | One source for the radius text; AC 40 |
| HR-10 | **Lifetime and logs.** The radii and `hiding_steps` go with the row (I-4, AC-4, `secure_delete`, ADR-019 rev. 3). Neither is ever logged (I-5) | AC 42 |

**Not revealed, by the spec.** Which end is cut and how far shows only through the missing positions themselves (AC 40); the SH-9 notice stays on every page.

## Cost

| What | Limit |
| --- | --- |
| Activity entry (static) | Target **0 bytes**: sliders, formatter use and shading glue are in the owner chunk, the seams exist (E-9 marks, MS-7). Ceiling 50; above: stop and report |
| Server | One extra pass per end at upload, build and owner read with a trim (O(n), no extra parse). Row grows by about 400 bytes |

## Verification (critical lane: QA at effort high)

Synthetic files only (ADR-004); generated tracks as in F-16's verification.

| What | Check |
| --- | --- |
| AC 34, 35, 37 | For start and end radii 0, 50, 300, 2,000 in every combination, with and without a trim: every public positioned sample lies at or after a recorded fix more than the start radius from the first (remaining) position, and at or before one more than the end radius from the last; radius 0 leaves its end untouched. Marker positions in hidden sections appear in no file under `public/{public_id}/` (`.bin`, decompressed `.br`, metadata, `index.html`, PNG). Start 0, end 500: only the end is hidden. Fully hidden and indoor notices as AC 37 |
| HR-3, HR-4 | Over the corpus and generated tracks, for every one of the 41 steps per end: the `hiding_steps` entry equals the boundary of the public cut built at that radius, sample for sample; with a trim, after the shift. Monotone in the radius |
| AC 40 | Two radius pairs that give the same cut produce byte-identical public folders. The F-58 AC 39 check re-run with one end hidden and several radii (F-58 hand-off) |
| AC 36 | Metadata summary and laps identical at every radius (F-16 H-1) |
| AC 38 | Raise start only, end only, from 0, raise one and lower the other: remove-first, with F-15's fault injection; lower one: in place |
| AC 41 | Migration on a v8 database with hiding on and off: 300/300 and 0/0; then `regenerate`: every public file name unchanged against a build of the same rows before F-55 |
| AC 33 | `PATCH` refusals: −50, 2,050, 120, 300.5, `"300"`, `hide_ends`; the row unchanged after each |
| Mutants | Each must fail a test: radius 0 as a 0 m circle; start and end radius swapped; step index off by one (k ↔ 50·(k+1)); `HidesMore` checking one end only or treating a lower value as hiding more; migration mapping hiding on to 0; `-1` shifted by `a`; shading from the saved radii instead of the slider values; the dialog from the slider values instead of the saved ones; a radius in the view model |

## Earlier decisions this changes

Edited or noted in place: F-12 settings object, F-15 P-7, F-16 header note, ADR-030 status (superseded in part by ADR-056), TD (Data model, Security and privacy, glossary). Read the following with this design, unedited:

| Where | Now reads |
| --- | --- |
| F-16 H-2, H-4, H-5, H-9 | The 300 m constant becomes the record's radius per end (HR-2); "`hide_ends` on" means "either radius above 0"; "off → on" means "a radius rises" (HR-5) |
| F-19 E-7, E-9 | The toggle becomes two sliders; marks follow the slider values from `hiding_steps` (HR-8), on map and charts |
| F-30 RG-4, RG-6; ADR-047 item 3 | "Hidden ranges" means `hiding_steps` (HR-7, ADR-056) |
| F-36 MC-8(b); F-45 ST-6 | "`hide_ends`" means either radius (HR-7, HR-8) |

## Hand-offs

| To | What |
| --- | --- |
| F-56 (PIN-7) | A pin check against hidden sections uses HR-3's function at the stored radii, server side; `hiding_steps` is display only |
| F-57 | Nothing: the route comes from the public data file |
| F-54 | Translate the slider labels (HR-8) |
