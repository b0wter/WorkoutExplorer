# F-57 GPX route download

- **Status:** draft; reworked 14 Nov 2026 (route only; OQ-241 to OQ-244 settled, OQ-245 open)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** critical (a viewer download of location data)
- **Requirements:** SH-13 (new), SH-4, UP-6, SH-3 (with F-55), SH-8 (FIT export stays dropped), AP-3, AP-12
- **Depends on:** F-16, F-47, F-54 (texts); not on F-56

## Goal
Someone who wants to ride or run the same route downloads it as a GPX file and loads it onto their own device.

## Scope
- A "Download route (GPX)" control on the viewing page of every published workout with a visible route. No owner setting.
- Not offered for multisport workouts (AP-12), nor when no position is visible (indoor, pool, fully hidden route).
- The GPX holds only positions from the public, privacy-applied data: after trim, start and end hiding and (with F-55) privacy zones. No timestamps, no heart rate, power or other data, no pins, no title or description (OQ-245 for elevation).
- Where the public route has gaps (hidden parts, GPS dropouts, AP-3), the track is split into separate segments, so no straight line crosses a hidden area.
- File name without the title: `route-YYYY-MM-DD.gpx`, or `route.gpx` without a start date.
- Built in the browser from the public data file; nothing new on the server (no file, no endpoint, no log). The code loads only when the control is used.

## Out of scope
- FIT or TCX export (SH-8, F-24 discarded); pins in the export (PIN-9 removed); an edit-page or owner download of the GPX.

## Acceptance criteria
To be written from the scope.

## Decisions (user, 14 Nov 2026)
- Route only: no pins, no times, no other data; for any published workout with a visible route; no owner setting; no export for multisport workouts. Replaces the "GPX export with pins" draft and its OQ-241 to OQ-244.
- The F-24 discard reason still holds for a FIT export; a GPX of the visible route is allowed because it uses only data the page already shows.

## Open questions
- [ ] **OQ-245 Elevation.** Suggested: include elevation, it helps route planning.
