# F-56 Map pins

- **Status:** draft; OQ-234 to OQ-240 decided 14 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; requirement priorities kept, within this feature)
- **Owner:** both (backend first)
- **Lane:** normal (critical only for the privacy-zone check, PIN-7)
- **Requirements:** PIN-1 to PIN-8, AP-15 (pins in fullscreen), AC-5, SH-4 (radius per end from F-55 once built), UP-6, SH-10, UP-7, AC-4, NFR Performance
- **Depends on:** F-16, F-19, F-26, F-47 (PIN-7 uses the start and end hiding as it stands; F-55's radius applies once built, no dependency)

## Goal
The owner marks dangers, points of interest and difficult sections on the route; every viewer sees them on the map, the charts and in a list.

## Scope
- PIN-1 to PIN-8 as in the spec: place by tapping the map or a chart; six types with icons and colours; title up to 60 characters and an optional note; points and sections; shown on the map, on the charts' x axis and in a list, linked to each other and to the cursor; a legend to show or hide types; refused inside a hidden start or end (each at its own radius); edited, moved and deleted after publishing without re-uploading.
- Pins appear in the fullscreen map (AP-15).
- Pins are stored with the workout for its lifetime and deleted with it (UP-7, AC-4). Removing or moving a pin is removed from the public page within 60 s (SH-10).
- Pin code and icons load lazily, only when a workout has pins or the owner edits them.

## Out of scope
- Photos on pins (discarded 14 Nov 2026). Pins from viewers or shared across workouts. Exporting pins (PIN-9 removed 14 Nov 2026).

## Acceptance criteria
After the open questions are decided.

## Decisions (user, 14 Nov 2026)
OQ-234 to OQ-240 accepted as suggested. Privacy zones were discarded on 14 Nov 2026; their parts now mean the hidden start and end, each at its own radius (SH-4, F-55).
- **OQ-234:** every pin snaps to the nearest point of the visible route.
- **OQ-235:** a section pin that crosses a hidden start or end is refused like a point pin there.
- **OQ-236:** pins that end up inside a hidden start or end after its radius grows (including from 0) are removed, with a one-line notice on the edit page.
- **OQ-237:** pins in parts that a changed trim or start and end hiding removes are removed, with the same notice.
- **OQ-238:** at most 50 pins per workout; note up to 500 characters, same text rules as the description (UP-4).
- **OQ-239:** the legend choice is not remembered; all types shown on every visit.
- **OQ-240:** no pins in the preview image (SH-5, SH-12).
