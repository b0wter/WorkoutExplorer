# F-33 Cursor readout in the fullscreen map

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** AP-21 (Must), AP-15, AP-6; extends F-26 (design ADR-038)
- **Depends on:** F-26

## Goal
A viewer following the route in fullscreen still sees the values at the point they touch or hover.

## Scope
- In fullscreen, the cursor readout (`CursorReadout.svelte`) shows as a compact strip pinned to the bottom of the map, on phone and desktop, portrait and landscape.
- Same values as outside fullscreen; follows the cursor while hovering or dragging along the route.
- Hidden while there is no cursor.

## Out of scope
- The range panel (AP-7) in fullscreen: ranges cannot be selected from the map.

## Acceptance criteria
- [ ] In fullscreen, hovering (desktop) or touching (phone) the route shows the strip at the bottom of the map with the same values as the readout outside fullscreen.
- [ ] Without a cursor no strip is shown.
- [ ] Zoom controls, exit button and the route colour legend stay visible and usable with the strip shown, in portrait and landscape.
- [ ] The range panel is not shown in fullscreen, also with a range selected.
- [ ] Leaving fullscreen shows the normal readout again; F-26's acceptance criteria still pass.
