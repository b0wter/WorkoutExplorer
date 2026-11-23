# F-56 Map pins

- **Status:** draft; OQ-234 to OQ-240 decided 14 Nov 2026; acceptance criteria written 19 Nov 2026; OQ-272 to OQ-277 decided 19 Nov 2026; OQ-272 changed 21 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; requirement priorities kept, within this feature)
- **Owner:** both (backend first)
- **Lane:** normal (critical only for the hidden-section check, PIN-7)
- **Requirements:** PIN-1 to PIN-8, AP-15 (pins in fullscreen), AP-22 (one line), AC-5, SH-4 (radius per end, F-55), UP-6, SH-10, UP-7, AC-4, INF-1, NFR Performance
- **Depends on:** F-16, F-19, F-26, F-47, F-55

## Goal
The owner marks dangers, points of interest and difficult sections on the route; every viewer sees them on the map, the charts and in a list.

## Scope
- PIN-1 to PIN-8 as in the spec, Should items (PIN-4 sections, PIN-6 legend) included: "Add pin" places a pin at the cursor (a section over a selected range); six types with icons and colours; title up to 60 characters and an optional note; points and sections; shown on the map, on the charts' x axis and in a list, linked to each other and to the cursor; a legend to show or hide types; refused inside a hidden start or end (each at its own radius); edited in a dialog and deleted after publishing without re-uploading; no move (delete and add again).
- Pins appear in the fullscreen map (AP-15).
- Pins are stored with the workout for its lifetime and deleted with it (UP-7, AC-4). A removed pin is removed from the public page within 60 s (SH-10).
- Pin code and icons load lazily, only when a workout has pins or the owner edits them.
- Texts in English; F-54 (built last) translates them.

## Out of scope
- Photos on pins (discarded 14 Nov 2026). Pins from viewers or shared across workouts. Exporting pins other than as waypoints in the F-57 route GPX (PIN-9, reinstated 21 Nov 2026). Clustering of overlapping markers.

## Assumptions
- Icons and colours are drafted by the engineer and reviewed by the user on screen in both themes.
- Selecting a section pin selects its section as a range (AP-7), like a lap row; selecting it again clears it.
- Hiding a type in the legend hides its pins on the map, the charts and in the list; the legend lists only types the workout has.
- Pin title follows the workout title's text rules (UP-4: trimmed, code points, no line breaks); the note the description's (OQ-238).
- Pins can be placed on a draft and on a published workout; viewers see them once published.
- The How to use dialog (AP-22) gets one line on pins, worded like the map lines (for workouts that have pins). The About page (INF-1) lists pins among the stored data.
- Without JavaScript there are no pins.

## Acceptance criteria
- [ ] On the edit page, "Add pin" places a point pin at the cursor (the point selected on the map or a chart, AP-6) and opens the pin dialog in both layouts (23 Nov 2026, [UI](../architect/UI-pin-dialog.md)); the pin is created only on OK, and Cancel, Esc, back or tapping outside leave no pin. A pin's row in the pins list or its marker on the map opens the same dialog, filled in. The pins list is read-only (icon, title, distance or dimmed reason, ×); all editing is in the dialog; delete is the list's × or the dialog's Delete; there is no tap-to-place mode, a tap only moves the cursor (OQ-272). "Add pin" is disabled with no cursor point, and with the cursor (or, for a section, any part of the range) inside a hidden start or end at its current radius or in a part the trim removes (PIN-7). The pin lands on the nearest point of the visible route (OQ-234); type (six, PIN-2), title and optional note are set in the dialog; an empty title is allowed and the pin then shows its type's name, e.g. "Danger" (OQ-277). Pin changes show on the edit page at once and on the public page only after the edit page's Save (OQ-274).
- [ ] With a range selected (AP-7), "Add pin" creates a section pin over that range instead of a point pin (OQ-272); a section pin's ends cannot be moved: delete and add again (OQ-273, changed 23 Nov 2026); both ends lie on the visible route; it is highlighted on the map and shaded on the charts.
- [ ] A point pin inside a hidden start or end, or a section that touches one, is refused with a one-line message on the edit page, at each end's own radius (PIN-7, OQ-235). The server refuses it too; a test sends such a pin to the owner API directly.
- [ ] Pins in a part the trim removes are deleted on the edit page's Save, also if the trim is later undone; while the trim is unsaved they show dimmed with "Removed on Save", and after Save one line on the edit page says how many were removed (OQ-237). Pins inside a hidden start or end after its radius is raised are kept: dimmed on the edit page with the reason (e.g. "Hidden by the start radius"), still editable and deletable, never published, and public again once the radius is lowered so they are no longer hidden (OQ-236). No public file ever contains a trimmed or hidden pin; adding a pin into a hidden or trimmed part is still refused (AC 35).
- [ ] On the viewing page, pins show on the map, as markers on the charts' x axis and in the list: one column: directly under the map; two columns: in the right column under the charts, above the range panel and laps; each row: type icon, title, distance from start in the units setting (a section: from–to), the note when the row is selected; no list without pins (OQ-276); selecting a pin in one view selects it in all and moves the cursor there.
- [ ] The legend shows or hides each type in every view; every visit starts with all types shown (OQ-239).
- [ ] In the fullscreen map, pins and the legend stay visible, and tapping a pin opens its card over the map (AP-15).
- [ ] After publishing, editing and deleting a pin shows on the public page without re-uploading (PIN-8); a removed pin's old place is no longer served after 60 s (SH-10).
- [ ] At most 50 pins per workout: the 51st cannot be added; title (60) and note (500) limits and text rules are enforced on the server as well (OQ-238). A hostile title or note is shown as plain text.
- [ ] A workout without a visible route (indoor, pool, entirely hidden or trimmed away) shows no pins to viewers and has no "Add pin" button (OQ-275); pins kept from before a radius was raised stay listed, dimmed, on the edit page (OQ-236).
- [ ] The preview image shows no pins (OQ-240).
- [ ] Test: after delete or expiry, nothing of the pins remains (database rows and public pins file; UP-7, AC-4).
- [ ] A workout without pins loads no pin code, icons or pins file for viewers; the entry bundle stays within budget and LCP is unchanged; with 50 pins on the synthetic 12-hour ride, cursor scrubbing keeps 60 fps.
- [ ] Works in both themes (DS-4), on phone and desktop, portrait and landscape, by touch and mouse.

## Decisions (user, 14 Nov 2026)
OQ-234 to OQ-240 accepted as suggested. Privacy zones were discarded on 14 Nov 2026; their parts now mean the hidden start and end, each at its own radius (SH-4, F-55).
- **OQ-234:** every pin snaps to the nearest point of the visible route.
- **OQ-235:** a section pin that crosses a hidden start or end is refused like a point pin there.
- **OQ-236:** pins that end up inside a hidden start or end after its radius grows (including from 0) are removed, with a one-line notice on the edit page. *Changed 20 Nov 2026, see below.*
- **OQ-237:** pins in parts that a changed trim or start and end hiding removes are removed, with the same notice. *Changed 20 Nov 2026, see below.*
- **OQ-238:** at most 50 pins per workout; note up to 500 characters, same text rules as the description (UP-4).
- **OQ-239:** the legend choice is not remembered; all types shown on every visit.
- **OQ-240:** no pins in the preview image (SH-5, SH-12).

## Decisions (user, 19 Nov 2026)
OQ-272 to OQ-277 accepted as suggested.
- **OQ-272:** an "Add pin" button; the next tap on the map or a chart places the pin and opens its form; Cancel or Esc leaves without a pin. *Changed 21 Nov 2026, see below.*
- **OQ-273:** section pins are made from a selected range; a new range moves their ends while editing. *Changed 23 Nov 2026, see below.*
- **OQ-274:** pin changes are saved with the edit page's Save; the public page changes after Save.
- **OQ-275:** no pins without a visible route; the Add pin button is hidden.
- **OQ-276:** list under the map (one column) or in the right column under the charts, above the range panel and laps (two columns); rows: type icon, title, distance from start (a section: from–to); note on selection; no list without pins.
- **OQ-277:** an empty title is allowed; the pin shows its type's name.

## Decisions (user, 21 Nov 2026)
- **OQ-272 changed:** pins only on the route; "Add pin" places a point pin at the cursor and opens its form, or a section pin over a selected range (AP-7; replaces the form's "Section" option). No "next tap places" mode. Disabled without a cursor point, and when the cursor or range lies in a hidden start or end (current radius) or a trimmed part; the refusal rule (PIN-7) is unchanged. OQ-273 unchanged (changed 23 Nov 2026).
- The reason text "There is a pin here already." is approved as written.

## Decisions (user, 20 Nov 2026)
OQ-236 and OQ-237 changed (architecture: `architect/F-56-map-pins.md` PN-4, PN-5, PN-7, PN-12; ADR-057 rev. 1).
- **OQ-236:** pins inside a hidden start or end after its radius is raised are kept, not removed: dimmed on the edit page with the reason, editable and deletable, never published or counted for viewers; lowering the radius makes them public again. Raising or lowering a radius never deletes a pin.
- **OQ-237:** pins in a part the trim removes are deleted on Save; the edit page shows them dimmed with "Removed on Save" before, and one line with the number removed after. Undoing the trim does not restore them.
- Adding a pin into a hidden or trimmed part stays refused (PIN-7, AC 35); moving was dropped on 23 Nov 2026.

## Decisions (user, 23 Nov 2026)
- Where "Add pin" sits (right-click menu, "+" on touch, one-column box) and its where-it-acts reason: see [F-65](F-65-owner-added-laps.md) Scope.
- **Pin dialog** ([UI](../architect/UI-pin-dialog.md); replaces the same day's "after Add pin the pin form scrolls into view with Title focused"): "Add pin" (the "+" menu, the right-click menu, the one-column box) opens a dialog in both layouts; the pin is created only on OK; Cancel, Esc, back or tapping outside leave no pin. A pin's row in the pins list or its marker on the map opens the same dialog, filled in. The pins list is read-only (icon, title, distance or dimmed reason, ×); all editing is in the dialog; delete is the list's × and the dialog's Delete.
- **OQ-273 changed, no move:** no Move button and no range-moving of section pins; moving a pin is delete and add again (as F-65 OQ-302). PIN-8's "move" removed.
