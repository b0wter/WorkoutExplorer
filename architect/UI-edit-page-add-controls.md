# UI: edit page, add lap / add pin controls

- **Status:** accepted, built (23 Nov 2026). Current state only; the original review and its problem list are in [history/UI-edit-page-add-controls.md](history/UI-edit-page-add-controls.md).
- **Scope:** the "+" on the map (two columns, touch), the add box under the map (one column), the right-click menu (mouse), and how they sit with the map controls, pin form, pins list, added-laps table and laps table. AP-8, PIN-1, PIN-7, AP-22, DS-4; [F-65](F-65-owner-added-laps.md) AL-8, AL-9; [F-56](F-56-map-pins.md) PN-12; decision log of `features/README.md`, 21–23 Nov 2026.
- **Code:** `src/web/src/edit/plus.ts`, `pins.ts`, `mapmenu.ts`, `laps.ts`, `edit.css`; `components/map/map-view.css`.

## Rules

1. **One model, one item list.** Select a spot (cursor or range) first, then add. Every entry point shows the same items in the same order, Add lap then Add pin, with the same states and reasons (`PlusItem`). An enabled item's reason slot shows where it acts ("at 12.4 km", "12.4–13.1 km", elapsed time without distance). No Add pin item without positions.
2. **Entry points by layout and pointer.**

   | Layout / pointer | Entry point |
   | --- | --- |
   | Two columns (`TWO_COLUMN_QUERY`), touch | "+" as its own group in MapLibre's **top-right** stack, below the location button (order: zoom, fullscreen, pin toggle, satellite, location, "+"); its list opens to the left inside the map. Not in fullscreen |
   | Any layout, mouse (`(hover: hover) and (pointer: fine)`) | No "+". Right-click on the map canvas opens a themed `menu`, mounted in `.route-map`, so it also works in fullscreen. Known edge: touch laptops |
   | One column | No "+". A full-width muted box under the map with two rows, "[+ Lap] reason" and "[+ Pin] reason"; "Select a point or range first" once, in the first row, while nothing is selected |
   | No visible route | A plain "Add lap" below the no-location hint (OQ-305) |
3. **Map overlays have two looks only.** *Buttons* look like MapLibre's buttons in both themes: `#fff`, black text, 29 px (44 px on a coarse pointer), radius 4 px, `box-shadow: 0 0 0 2px rgba(0,0,0,.1)`. *Panels with text* (pin legend, colour box, reasons, right-click menu) follow the theme: `--bg`, `--fg`, radius 4 px, the same shadow. Nothing transparent on the tiles.
4. **One owner per map corner.** Top-left: colour control. Top-right: the button stack of rule 2 (attribution beside it in two columns). Bottom-left: pin legend. Bottom-right: attribution (one column).
5. **Never shift layout.** Menus are overlays. Reserved lines keep their height only for content that can appear in this visit.
6. **A disabled item stays readable.** `aria-disabled`, opacity 0.6, the reason on the same line or right under it.
7. **Sizes.** 32 px with a fine pointer, at least 44 px with `(pointer: coarse)`, add controls and the lap row × included.
8. **Show the result where the eye is.** Add pin opens the pin dialog: see [UI-pin-dialog.md](UI-pin-dialog.md). After Add lap nothing moves; the map's lap marker and the new row are the feedback.
9. **Motion.** 150 ms ease-out rise and fade, none under `prefers-reduced-motion`.

## Below the map (edit page, one column order)

```
 map
 add box (rule 2, one column)
 pin form (when a pin is selected)
 Pins ─ list
 Added laps ─ table (lap, time, distance, ×), only while a boundary exists (F-65 AL-8)
 charts …
 laps and splits (× on added rows; F-66 Remove/Restore auto-laps line)
 range panel (sticky bottom)
```

## Open (not decided)

From the review's broader observations; none blocks work.

1. Settings panel above the page header (F-19 E-10): on a phone Save is far from the map, pins and laps edits. Suggestion: ask the user whether to put the header first or add a sticky Save bar in one column.
2. Save and Publish are both filled primary buttons on a draft. Suggestion: one primary per row.
3. Button heights still vary outside the add controls (29, 32, 34, 40 px). Suggestion: align to rule 7; the 40 px bars stay.
4. Satellite in dark mode: a dark themed panel on dark imagery may lack an edge. Suggestion: check in a browser; if so, a 1 px `--line` border on themed map panels.
