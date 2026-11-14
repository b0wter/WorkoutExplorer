# F-56 Map pins

- **Status:** draft
- **Category:** Planned (backlog scheme of 14 Nov 2026; requirement priorities kept, within this feature)
- **Owner:** both (backend first)
- **Lane:** normal (critical only for the privacy-zone check, PIN-7)
- **Requirements:** PIN-1 to PIN-8, AP-15 (pins in fullscreen), AC-5, SH-3, SH-10, UP-7, AC-4, NFR Performance
- **Depends on:** F-55 (privacy zones, for PIN-7), F-19, F-26

## Goal
The owner marks dangers, points of interest and difficult sections on the route; every viewer sees them on the map, the charts and in a list.

## Scope
- PIN-1 to PIN-8 as in the spec: place by tapping the map or a chart; six types with icons and colours; title up to 60 characters and an optional note; points and sections; shown on the map, on the charts' x axis and in a list, linked to each other and to the cursor; a legend to show or hide types; refused inside a privacy zone; edited, moved and deleted after publishing without re-uploading.
- Pins appear in the fullscreen map (AP-15).
- Pins are stored with the workout for its lifetime and deleted with it (UP-7, AC-4). Removing or moving a pin is removed from the public page within 60 s (SH-10).
- Pin code and icons load lazily, only when a workout has pins or the owner edits them.

## Out of scope
- Photos on pins (discarded 14 Nov 2026). Pins from viewers or shared across workouts. GPX export (F-57).

## Acceptance criteria
After the open questions are decided.

## Open questions
- [ ] **OQ-234 Snap to the route.** Suggested: every pin snaps to the nearest point of the visible route; a pin is always on the route.
- [ ] **OQ-235 Section pins and zones.** Suggested: a section that crosses a privacy zone, or into a hidden start or end, is refused like a point pin inside one.
- [ ] **OQ-236 Zone added after pins.** Suggested: pins that end up inside a new zone are removed with a one-line notice on the edit page.
- [ ] **OQ-237 Trim or hiding changed after pins.** Suggested: as OQ-236: pins in trimmed or hidden parts are removed, with the same notice.
- [ ] **OQ-238 Limits.** Suggested: at most 50 pins per workout; note up to 500 characters, same text rules as the description (UP-4).
- [ ] **OQ-239 Legend choice remembered.** Suggested: not remembered, per visit, all types shown.
- [ ] **OQ-240 Pins in the preview image.** Suggested: no pins in the preview image (SH-5, SH-12).
