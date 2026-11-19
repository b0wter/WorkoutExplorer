# F-57 GPX route download

- **Status:** draft; reworked 14 Nov 2026 (route only; OQ-241 to OQ-245 settled); acceptance criteria written 19 Nov 2026; OQ-269 to OQ-271 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** critical (a viewer download of location data)
- **Requirements:** SH-13 (new), SH-4 (radius per end, F-55), UP-6, SH-8 (FIT export stays dropped), AP-3, AP-12, INF-1
- **Depends on:** F-16, F-47, F-55; not on F-56

## Goal
Someone who wants to ride or run the same route downloads it as a GPX file and loads it onto their own device.

## Scope
- A "Download route (GPX)" control on the viewing page of every published workout with a visible route (placement: OQ-269). No owner setting.
- Not offered for multisport workouts (AP-12), nor when no position is visible (indoor, pool, fully hidden route).
- The GPX holds only positions from the public, privacy-applied data: after trim and start and end hiding. Elevation included where recorded (OQ-245). No timestamps, no heart rate, power or other data, no pins, no title or description.
- Where the public route has gaps (hidden parts, GPS dropouts, AP-3), the track is split into separate segments, so no straight line crosses a hidden area (under review: OQ-270).
- File name without the title: `route-YYYY-MM-DD.gpx`, or `route.gpx` without a start date.
- Built in the browser from the public data file; nothing new on the server (no file, no endpoint, no log). The code loads only when the control is used.
- Texts in English; F-54 (built last) translates them.

## Out of scope
- FIT or TCX export (SH-8, F-24 discarded); pins in the export (PIN-9 removed); an edit-page or owner download of the GPX; a download of only the selected range.

## Assumptions
- Every position of the public data file is written, in order; no thinning.
- Elevation is the elevation the page's chart shows; a point without elevation has no `<ele>`, never 0.
- The date in the file name is the start date in the workout's own time zone (AP-1), after a trim the trimmed start.
- A multi-session workout of one sport (two rides, AP-12) is not "Multisport" and gets the download; virtual rides get it like any other (no special handling).
- Without JavaScript there is no control.

## Acceptance criteria
- [ ] The viewing page of a published workout with at least one visible position shows the "Download route (GPX)" control (placement per OQ-269). It is absent on a Multisport workout, an indoor or pool workout, a route entirely hidden or trimmed away, the edit page, the published page and without JavaScript.
- [ ] The downloaded file is valid GPX 1.1 (checked against the schema), and the user loads one on a device or route app of their choice.
- [ ] Test: the file holds exactly the public data file's positions, in order, with elevation where recorded; it has no `<time>`, no extensions, no name, description, title, sport, pin or other value.
- [ ] Test, with a trim and both ends hidden: no position from the trimmed or hidden parts is in the file; its first and last points are the first and last visible points on the map.
- [ ] Segments follow OQ-270; no straight line in the file joins two points across a hidden part.
- [ ] File name `route-YYYY-MM-DD.gpx` with the start date as above; `route.gpx` for a workout without a start date.
- [ ] Downloading makes no network request (the public data is already loaded), and the server has no new endpoint, file or log line for it.
- [ ] The GPX code is not in the entry bundle and loads only when the control is used; the entry bundle stays within its budget and LCP is unchanged.
- [ ] A 12-hour ride downloads in under 2 s on the reference phone (Non-functional requirements, Performance) and does not freeze the page.
- [ ] The sharer is told about the download as decided in OQ-271; the About page (INF-1) names it.

## Decisions (user, 14 Nov 2026)
- Route only: no pins, no times, no other data; for any published workout with a visible route; no owner setting; no export for multisport workouts. Replaces the "GPX export with pins" draft and its OQ-241 to OQ-244.
- The F-24 discard reason still holds for a FIT export; a GPX of the visible route is allowed because it uses only data the page already shows.

- **OQ-245:** elevation is included; it helps route planning.

## Open questions
- [ ] **OQ-269 Where the control sits.** The left column of the two-column layout holds only the map (AP-17), and the header row is full on a phone. Suggested: a download icon button in the map's controls, tooltip "Download route (GPX)", also in the fullscreen map (AP-15). Affects AP-15, the map controls and the How to use dialog (no new line).
- [ ] **OQ-270 Segments.** Since privacy zones were dropped, hidden parts lie only at the ends, so a gap in the middle (GPS dropout, pause) hides nothing; and some devices load only a file's first segment. Suggested: one segment, straight across dropouts and pauses, as the map draws them (AP-3); this replaces the scope line on gaps. Affects the GPX contents.
- [ ] **OQ-271 Telling the sharer.** Any viewer can now take the visible route as a file. Suggested: one line on the edit page under the hiding sliders, "Viewers can download the visible route as a GPX file.", and one sentence in the About page's "who sees what" (INF-1). Affects the edit page, INF-1.
