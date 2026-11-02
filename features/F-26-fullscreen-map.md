# F-26 Fullscreen map

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** AP-15 (Should), AP-18 (map kept at the top survives fullscreen), NFR Accessibility (Esc is the only shortcut), NFR Compatibility; Technical design: Front end (Implementation notes)
- **Depends on:** F-06, F-13

## Goal
A viewer on a phone expands the map to the whole screen to follow the route.

## Scope
- One button expands the map to the whole screen, hiding header, summary, charts, selection and laps.
- Kept visible: route, start and finish markers, lap markers, zoom controls, exit button.
- Exit with the button, Esc, or the phone's back gesture; the page returns to the same scroll position.
- Portrait and landscape.
- If the map was kept at the top (AP-18, F-13), it still is after leaving fullscreen.
- Works on iOS Safari without relying on the element Fullscreen API.

## Out of scope
- Pins and their legend in fullscreen (phase 2).

## Acceptance criteria
- [ ] On iOS Safari and Android Chrome, the button fills the screen with the map; no data field is visible.
- [ ] Route, start, finish and lap markers, zoom controls and exit button are visible in fullscreen.
- [ ] Exiting with the button, Esc (desktop) and the back gesture (Android, iOS) each return to the page at the previous scroll position; back does not leave the page.
- [ ] Rotating the phone in fullscreen keeps the map filling the screen.
- [ ] No fullscreen button on an indoor workout or on a route hidden entirely (F-16).
