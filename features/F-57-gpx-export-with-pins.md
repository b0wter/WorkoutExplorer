# F-57 GPX export with pins

- **Status:** draft
- **Category:** Planned (backlog scheme of 14 Nov 2026)
- **Owner:** frontend
- **Lane:** critical (a viewer download of location data)
- **Requirements:** PIN-9, SH-3, SH-4, UP-6, SH-8 (viewer download dropped 14 Nov 2026, see OQ-241)
- **Depends on:** F-56

## Goal
Anyone with the viewing link loads the route and its dangers onto their own device.

## Scope
- A "Download GPX" button on the viewing page of a workout with pins (OQ-242).
- The GPX file is built in the browser from the public data only: the visible route (trim, start and end hiding and privacy zones already applied) as a track, and every pin as a waypoint (type, title, note); a section pin as two waypoints, start and end (OQ-243).
- Nothing new on the server: no file, no endpoint, no log.

## Out of scope
- FIT or TCX export; export of the original; device-specific icons.

## Acceptance criteria
After the open questions are decided.

## Open questions
- [ ] **OQ-241 Conflict with the dropped viewer download.** F-24 was discarded as "minimal use, not worth the risks and effort"; a GPX of the route is also a viewer download. Suggested: allowed, because it uses only the public data already in the page and needs no server work; the risk is no larger than the page itself.
- [ ] **OQ-242 Owner setting.** Suggested: no setting; the button shows on every workout with at least one pin.
- [ ] **OQ-243 Route or pins only.** Suggested: route and pins; a pins-only GPX is not enough to follow the route.
- [ ] **OQ-244 Times in the GPX.** Suggested: no timestamps, positions and elevation only, so the file shows the route, not when it was ridden.
