# F-57 GPX route download

- **Status:** draft; reworked 14 Nov 2026 (route only; OQ-241 to OQ-245 settled); acceptance criteria written 19 Nov 2026; OQ-269 to OQ-271 decided 19 Nov 2026; public pins added 21 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** critical (a viewer download of location data)
- **Requirements:** SH-13 (new), PIN-9 (reinstated 21 Nov 2026), SH-4 (radius per end, F-55), UP-6, SH-8 (FIT export stays dropped), AP-3, AP-12, INF-1
- **Depends on:** F-16, F-47, F-55, F-56 (pins)

## Goal
Someone who wants to ride or run the same route downloads it as a GPX file and loads it onto their own device.

## Scope
- A "Download route (GPX)" button on the viewing page of every published workout with a visible route, above the summary strip's stat cells (AP-2), not a map control (OQ-269). No owner setting.
- Not offered for multisport workouts (AP-12), nor when no position is visible (indoor, pool, fully hidden route).
- The GPX holds only positions from the public, privacy-applied data: after trim and start and end hiding. Elevation included where recorded (OQ-245). No timestamps, no heart rate, power or other data, no title or description.
- The workout's public pins (exactly those viewers see; never in hidden or trimmed parts) as `<wpt>` waypoints: `<name>` = the pin's title, or its type name if the title is empty; `<type>` = the pin type (no `<sym>`); `<desc>` = the note when present. A section pin becomes two waypoints, "Title (start)" and "Title (end)". No times, no other data. Without public pins the file is unchanged.
- One track segment, joined across GPS dropouts and pauses as the map draws them (AP-3; OQ-270). Hidden parts lie only at the ends, so no join crosses a hidden part.
- File name without the title: `route-YYYY-MM-DD.gpx`, or `route.gpx` without a start date.
- Built in the browser from the public data file; nothing new on the server (no file, no endpoint, no log). The code loads only when the button is used.
- Texts in English; F-54 (built last) translates them.

## Out of scope
- FIT or TCX export (SH-8, F-24 discarded); owner-only or hidden pins in the export; an edit-page or owner download of the GPX; a download of only the selected range.

## Assumptions
- Every position of the public data file is written, in order; no thinning.
- Elevation is the elevation the page's chart shows; a point without elevation has no `<ele>`, never 0.
- The date in the file name is the start date in the workout's own time zone (AP-1), after a trim the trimmed start.
- A multi-session workout of one sport (two rides, AP-12) is not "Multisport" and gets the download; virtual rides get it like any other (no special handling).
- Without JavaScript there is no button.

## Acceptance criteria
- [ ] The viewing page of a published workout with at least one visible position shows the "Download route (GPX)" button above the summary strip's stat cells (OQ-269); it is not in the map controls or the fullscreen map. It is absent on a Multisport workout, an indoor or pool workout, a route entirely hidden or trimmed away, the edit page, the published page and without JavaScript.
- [ ] The downloaded file is valid GPX 1.1 (checked against the schema), and the user loads one on a device or route app of their choice.
- [ ] Test: the file holds exactly the public data file's positions, in order, with elevation where recorded; it has no `<time>`, no extensions, no track or file name, description, title, sport or other value.
- [ ] Test: the file has one `<wpt>` per public point pin and two per public section pin ("Title (start)", "Title (end)") at the pins' positions, with `<name>` (title, or type name when empty), `<type>` (pin type, no `<sym>`) and `<desc>` only when a note exists; no `<time>` or other element. No waypoint for a pin in a hidden or trimmed part (current radius) or not shown to viewers. A workout without public pins gives a file identical to one without pins.
- [ ] Test, with a trim and both ends hidden: no position from the trimmed or hidden parts is in the file; its first and last points are the first and last visible points on the map.
- [ ] Test: the file has one `<trk>` with one `<trkseg>`, joined across GPS dropouts and pauses (OQ-270); no point of a trimmed or hidden part lies between two of its points.
- [ ] File name `route-YYYY-MM-DD.gpx` with the start date as above; `route.gpx` for a workout without a start date.
- [ ] Downloading requests no workout data (the public data is already loaded; only the download code itself may load on the first click, and, when the workout has public pins, the pins file is fetched lazily on that click), and the server has no new endpoint, file or log line for it.
- [ ] The GPX code is not in the entry bundle and loads only when the button is used; the entry bundle stays within its budget and LCP is unchanged.
- [ ] A 12-hour ride downloads in under 2 s on the reference phone (Non-functional requirements, Performance) and does not freeze the page.
- [ ] The edit page shows, under the hiding sliders, "Viewers can download the visible route as a GPX file: the track and its public pins, without times, heart rate or any other data." The About page's "who sees what" part (INF-1) has one sentence saying the same (OQ-271).

## Decisions (user, 14 Nov 2026)
- Route only: no pins, no times, no other data; for any published workout with a visible route; no owner setting; no export for multisport workouts. Replaces the "GPX export with pins" draft and its OQ-241 to OQ-244.
- The F-24 discard reason still holds for a FIT export; a GPX of the visible route is allowed because it uses only data the page already shows.

- **OQ-245:** elevation is included; it helps route planning.

## Decisions (user, 21 Nov 2026)
- Reverses the 14 Nov "no pins" rule: the GPX also holds the public pins as waypoints (Scope). PIN-9 reinstated in the spec.
- QA-57-r2-1/r2-4: GPX kept as built: pin type only as `<type>`, no `<sym>`; AC 7 allows the lazy pins-file fetch on click.

## Decisions (user, 19 Nov 2026)
- **OQ-269:** changed: a prominent button above the summary strip's stat cells, labelled "Download route (GPX)", not a map control; shown only where the download is offered (Scope).
- **OQ-270:** one segment, joined across dropouts and pauses.
- **OQ-271:** one line on the edit page under the hiding sliders and one sentence on the About page (INF-1), both also saying it is the track only: no times, no heart rate or any other data.
- The 12-hour ride in under 2 s stays the test target (main session; the user did not object).
