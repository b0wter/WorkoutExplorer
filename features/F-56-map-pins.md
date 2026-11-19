# F-56 Map pins

- **Status:** draft; OQ-234 to OQ-240 decided 14 Nov 2026; acceptance criteria written 19 Nov 2026; OQ-272 to OQ-277 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; requirement priorities kept, within this feature)
- **Owner:** both (backend first)
- **Lane:** normal (critical only for the hidden-section check, PIN-7)
- **Requirements:** PIN-1 to PIN-8, AP-15 (pins in fullscreen), AP-22 (one line), AC-5, SH-4 (radius per end, F-55), UP-6, SH-10, UP-7, AC-4, INF-1, NFR Performance
- **Depends on:** F-16, F-19, F-26, F-47, F-55

## Goal
The owner marks dangers, points of interest and difficult sections on the route; every viewer sees them on the map, the charts and in a list.

## Scope
- PIN-1 to PIN-8 as in the spec, Should items (PIN-4 sections, PIN-6 legend) included: place by tapping the map or a chart; six types with icons and colours; title up to 60 characters and an optional note; points and sections; shown on the map, on the charts' x axis and in a list, linked to each other and to the cursor; a legend to show or hide types; refused inside a hidden start or end (each at its own radius); edited, moved and deleted after publishing without re-uploading.
- Pins appear in the fullscreen map (AP-15).
- Pins are stored with the workout for its lifetime and deleted with it (UP-7, AC-4). Removing or moving a pin is removed from the public page within 60 s (SH-10).
- Pin code and icons load lazily, only when a workout has pins or the owner edits them.
- Texts in English; F-54 (built last) translates them.

## Out of scope
- Photos on pins (discarded 14 Nov 2026). Pins from viewers or shared across workouts. Exporting pins (PIN-9 removed 14 Nov 2026). Clustering of overlapping markers.

## Assumptions
- Icons and colours are drafted by the engineer and reviewed by the user on screen in both themes.
- Selecting a section pin selects its section as a range (AP-7), like a lap row; selecting it again clears it.
- Hiding a type in the legend hides its pins on the map, the charts and in the list; the legend lists only types the workout has.
- Pin title follows the workout title's text rules (UP-4: trimmed, code points, no line breaks); the note the description's (OQ-238).
- Pins can be placed on a draft and on a published workout; viewers see them once published.
- The How to use dialog (AP-22) gets one line on pins, worded like the map lines (for workouts that have pins). The About page (INF-1) lists pins among the stored data.
- Without JavaScript there are no pins.

## Acceptance criteria
- [ ] On the edit page, the owner places a point pin by tapping the map or a chart (as decided in OQ-272); it lands on the nearest point of the visible route (OQ-234); type (six, PIN-2), title (OQ-277) and optional note are set in its form; saved as decided in OQ-274.
- [ ] The owner creates a section pin (OQ-273); both ends lie on the visible route; it is highlighted on the map and shaded on the charts.
- [ ] A point pin inside a hidden start or end, or a section that touches one, is refused with a one-line message on the edit page, at each end's own radius (PIN-7, OQ-235). The server refuses it too; a test sends such a pin to the owner API directly.
- [ ] After raising a radius or changing the trim, pins now inside a hidden or trimmed part are removed and the edit page shows a one-line notice (OQ-236, OQ-237); no public file ever contains them.
- [ ] On the viewing page, pins show on the map, as markers on the charts' x axis and in the list (OQ-276); selecting a pin in one view selects it in all and moves the cursor there.
- [ ] The legend shows or hides each type in every view; every visit starts with all types shown (OQ-239).
- [ ] In the fullscreen map, pins and the legend stay visible, and tapping a pin opens its card over the map (AP-15).
- [ ] After publishing, editing, moving and deleting a pin shows on the public page without re-uploading (PIN-8); a removed or moved pin's old place is no longer served after 60 s (SH-10).
- [ ] At most 50 pins per workout: the 51st cannot be added; title (60) and note (500) limits and text rules are enforced on the server as well (OQ-238). A hostile title or note is shown as plain text.
- [ ] Workouts without a visible route follow OQ-275.
- [ ] The preview image shows no pins (OQ-240).
- [ ] Test: after delete or expiry, nothing of the pins remains (database rows and public pins file; UP-7, AC-4).
- [ ] A workout without pins loads no pin code, icons or pins file for viewers; the entry bundle stays within budget and LCP is unchanged; with 50 pins on the synthetic 12-hour ride, cursor scrubbing keeps 60 fps.
- [ ] Works in both themes (DS-4), on phone and desktop, portrait and landscape, by touch and mouse.

## Decisions (user, 14 Nov 2026)
OQ-234 to OQ-240 accepted as suggested. Privacy zones were discarded on 14 Nov 2026; their parts now mean the hidden start and end, each at its own radius (SH-4, F-55).
- **OQ-234:** every pin snaps to the nearest point of the visible route.
- **OQ-235:** a section pin that crosses a hidden start or end is refused like a point pin there.
- **OQ-236:** pins that end up inside a hidden start or end after its radius grows (including from 0) are removed, with a one-line notice on the edit page.
- **OQ-237:** pins in parts that a changed trim or start and end hiding removes are removed, with the same notice.
- **OQ-238:** at most 50 pins per workout; note up to 500 characters, same text rules as the description (UP-4).
- **OQ-239:** the legend choice is not remembered; all types shown on every visit.
- **OQ-240:** no pins in the preview image (SH-5, SH-12).

## Open questions
- [ ] **OQ-272 Placing a pin.** A tap on the map or a chart moves the cursor today (AP-6). Suggested: an "Add pin" button on the edit page; the next tap on the map or a chart places the pin and opens its form; Cancel or Esc leaves without a pin. Affects the edit page.
- [ ] **OQ-273 Section pins.** Suggested: made from a selected range (AP-7): with a range selected, the pin form offers "Section"; its ends are moved by selecting a new range while editing it. Affects PIN-4, the edit page.
- [ ] **OQ-274 Saving.** Suggested: pin changes are saved with the edit page's Save, like every other setting; the edit page shows them at once, the public page after Save. Alternative: each pin saved on its own at once. Affects PIN-8, the owner API.
- [ ] **OQ-275 Workouts without a visible route** (indoor, pool, entirely hidden or trimmed away). Suggested: no pins; the Add pin button is hidden, as every pin sits on a visible route point. Affects PIN-1, PIN-5.
- [ ] **OQ-276 Pin list: place and rows.** The left column of the two-column layout holds only the map (AP-17). Suggested: one column: directly under the map; two columns: in the right column under the charts, above the range panel and laps; each row: type icon, title, distance from start in the units setting (a section: from–to); the note shows when the row is selected; no list without pins. Affects PIN-5, AP-17, the module order.
- [ ] **OQ-277 Empty title.** Suggested: allowed; the pin then shows its type's name ("Danger"). Alternative: title required. Affects PIN-3.
