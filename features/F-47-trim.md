# F-47 Trim

- **Phase:** 2
- **Owner:** both (backend first: trim in the public transform, settings and recomputed summary; then the edit-page slider)
- **Lane:** critical (the trimmed part, typically the drive or walk home after a forgotten stop, must never reach any public file; it shares the public transform and the hidden-range computation with SH-4; architect design, QA at effort high). Not data-loss critical: the original stays unchanged and a trim can always be moved or removed.
- **Requirements:** UP-6 (Could, phase 2; rules decided 12 Nov 2026, OQ-169 to OQ-179); AP-2, AP-3, AP-7, AP-8, AP-12, AP-16, AP-19, UP-4, UP-5, UP-7, AC-4, AC-5, SH-4, SH-5, SH-8 (phase 3), SH-9, SH-10, SH-12; NFR Privacy (store as little as possible), NFR Performance (budgets unchanged), NFR Reliability; technical design (Publish and view flow 4–6, derived values, `workouts` table, Glossary: Public build)
- **Depends on:** F-16, F-19, F-20, F-30, F-36, F-42, F-44, F-45, F-46

## Goal
A sharer who forgot to stop the watch cuts off the unwanted start or end, and viewers see only the workout that remains, with totals that match it.

## What exists
- The public build cuts from `original.fit` on every build through one public transform (hiding today; technical design, Glossary: Public build). The `workouts` table reserves `trim_start_s`, `trim_end_s` for phase 2 (technical design, ADR-019).
- Hiding (F-16) and its to-be-hidden marks on the edit page, computed at upload (ADR-030; its "Would choose differently if" applies now that hiding follows the trim).
- Hand-offs already written: pool keys after a trim (F-44 PS-8; changed by OQ-179: only the number of lengths is dropped); the town from the first point visible after hiding and trim, and the owner side's trimmed start (F-45 ST-4, ST-6); lap triggers unchanged by trim (F-46 LS-1); the preview basemap dropped when the route changes (F-36 MC-3, ADR-041).

## Scope
- **Rules as in UP-6** (decided 12 Nov 2026): two-handle slider on the edit page, before and after publishing; full recording shown with trimmed parts shaded; non-destructive, movable and removable; at least 60 s remain; cuts anywhere, creates nothing new; workout starts at the trim start; recomputed and dropped values; laps and legs; trim before hiding; published pages. Route without remaining positions: AP-3.
- **Public result.** Everything public is built from the remaining part only: data file and metadata (sessions, laps, bounding box, start and finish), static page, preview image and card values (SH-5, SH-12), suggested title (UP-5), and, in phase 3, the viewer download (SH-8).
- **Edit-page marks.** The to-be-hidden marks (SH-4) follow the trim; how they are computed is the architect's call (ADR-030).
- **Rebuilds.** `regenerate` (F-30) rebuilds with the stored trim.
- **Storage.** Only the trim setting on the workout row; deleted with the workout (UP-7, AC-4). Lifetime still counts from upload.
- **Privacy notice.** SH-9 unchanged (OQ-177).

## Out of scope
- Cutting out a part in the middle; several trims; trimming per leg.
- Changing the original file or the owner's original download.
- Privacy zones (SH-3), pins (PIN-), viewer download (F-24, phase 3; it must apply the trim when built).

## Acceptance criteria
- [ ] The edit page has one slider with two handles, start and end; moving them and saving trims the workout, on a draft and on a published workout. The slider is not in the viewing page's bundle.
- [ ] After a trim the edit page's header (start time), charts, time axis, laps table and sections show the full recording, with the trimmed parts shaded in the charts and on the map; only its summary shows the trimmed totals.
- [ ] A 60 min run with a 20 min drive home at the end, trimmed to the run: no file under `public/{public_id}/` (data file, metadata JSON, `index.html`, preview image) contains a position, timestamp or value from the drive; a test searches for them. The same for a trim at the start.
- [ ] After the trim, distance, moving and elapsed time, speed or pace, heart rate, power, cadence, gain and temperature in the summary match the remaining part, not the device's session totals; calories are hidden, not shown as zero.
- [ ] A 25 m pool swim with a trim still shows "Pool 25 m" but no "Lengths" (until F-48: from then on the remaining whole lengths, OQ-181); its splits still go by whole lengths (100 m every 4 lengths, AP-20).
- [ ] Start time, time of day, elapsed time and the time axis count from the trim start; the cursor at the first remaining sample shows 0:00:00.
- [ ] Laps wholly inside a trimmed part are gone from the table and the map. A run whose last lap the trim cuts through shows that lap with only its remaining part: distance and time of that part, values computed as for a range (AP-7), trigger kept. Laps are numbered from 1.
- [ ] With hiding on and a trim, the hidden start and end sections are measured from the first and last remaining positions; no position within 300 m of them, before the route first leaves that circle, appears in any public file.
- [ ] A triathlon trimmed into the bike leg loses the swim and T1: the sport, sections, separator rows, leg markers and suggested title follow the remaining legs (bike and run: "multisport"; a single remaining run: "run").
- [ ] The suggested title takes its town and time of day from the trimmed workout: a route whose trimmed start lies next to town A and whose first remaining public point lies nearer town B gets town B.
- [ ] The slider cannot leave less than 60 s; the server refuses such a trim with "At least 1 minute of the workout must remain."
- [ ] A ride whose remaining part has no positions shows "No location data in this workout." and does not break.
- [ ] Moving the handles back to the ends restores the full workout from the original; trimming again gives the same public files as the first time.
- [ ] On a published workout, a trim that removes more stops serving the old data file, page and preview image within 60 s, and the old files are deleted (SH-10); the browser-made preview basemap is dropped or replaced (F-36). No extra warning is shown.
- [ ] The owner's original download is still the full upload after a trim.
- [ ] Indoor, truncated (UP-3) and pool files can be trimmed and still render.
- [ ] `regenerate` keeps every trim; deleting or expiring the workout removes the trim setting with the row.
- [ ] Viewer page budgets unchanged within noise (entry bundle, LCP, 60 fps on `ride-12h`).

## Open questions

All resolved by the user on 12 Nov 2026, OQ-178 and OQ-179 on the architect's design (decision log in [README](README.md)):

- **OQ-169:** one slider with two handles on the edit page, not the range selection.
- **OQ-170:** the edit page shows the full recording, trimmed parts shaded; the summary as viewers see it.
- **OQ-171:** the trim cuts anywhere and creates nothing new; totals recomputed; calories and lengths dropped (pool length kept, OQ-179); laps wholly trimmed disappear, a cut lap keeps its remaining part (values as a range, trigger kept); laps renumbered from 1.
- **OQ-172:** the workout starts at the trim start.
- **OQ-173:** trim first, then hiding.
- **OQ-174:** legs wholly trimmed away disappear; sport, sections and title follow the remaining legs.
- **OQ-175:** at least 60 s remain; no remaining positions: "No location data in this workout."
- **OQ-176:** no extra warning on a published page; old range links may open shifted or cut short; widening shows more without confirmation.
- **OQ-177:** SH-9 wording kept.
- **OQ-178:** on the edit page only the summary shows trimmed values; header, laps, sections and time axis show the full recording, trimmed parts shaded.
- **OQ-179:** a trim keeps the pool length and drops only the number of lengths; calories stay dropped; pool splits by whole lengths keep working.
