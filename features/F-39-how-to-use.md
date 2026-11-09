# F-39 How to use

- **Phase:** 2
- **Owner:** frontend
- **Lane:** normal
- **Requirements:** AP-22 (Could, phase 2); changes the AP-7 hint on the viewing page; texts describe AP-6, AP-7, AP-8, AP-15, DS-1, DS-4 (and AP-19 once F-40 ships); NFR Performance (budgets unchanged), NFR Privacy (no cookie, nothing to the server)
- **Depends on:** F-14, F-17, F-18, F-26

## Goal
A first-time viewer finds out how to explore the page (cursor, ranges, laps, fullscreen map, units) without anyone explaining it.

## Scope
- **Button.** On the viewing page (`/w/{id}`) only: a text button "How to use" with a small icon in the header's control row, next to Copy link and the units and theme drop-downs; visible on phones too, never in a menu. Until it has been opened once on the device it is emphasised (accent colour); then plain (OQ-129). Not shown without JavaScript.
- **Dialog.** Title "How to use this page"; one line per section, wording by input type (OQ-126); a section the workout cannot use is left out (OQ-128). Closes with its close button, Esc or a tap outside (OQ-129). Its code and text load only when the button is first used.

  | Section | Touch | Mouse |
  | --- | --- | --- |
  | Values at a point | Touch a chart and slide sideways, or drag along the route on the map, to see the values at that point. | Point at a chart or along the route on the map to see the values at that point. |
  | Select a range | Touch and hold a chart, then drag, to select a range and see its totals; Clear removes it. | Drag across a chart to select a range and see its totals; Clear removes it. |
  | Laps and splits | Tap a lap or split to select it as a range. | Click a lap or split to select it as a range. |
  | Fullscreen map | Tap the fullscreen button on the map to enlarge it; tap it again or go back to leave. | Click the fullscreen button on the map to enlarge it; click it again or press Esc to leave. |
  | Units and theme | Change units and theme at the top of the page; this device remembers your choice. | (same) |
  | Share a range (added by F-40) | Select a range, then tap "Copy link to this selection" to share it. | Select a range, then click "Copy link to this selection" to share it. |

  Gestures as built: chart cursor and long press (400 ms) in `lib/charts/gesture.ts`, route cursor in `lib/route/gesture.ts`, Clear in `RangePanel.svelte`, fullscreen through MapLibre's button (Esc, back gesture: AP-15).
- **First-visit range hint.** The existing hint under the charts (AP-7; "Drag across a chart to select a range." / "Touch and hold a chart, then drag to select a range.") gets a dismiss control and, on the viewing page, shows only until the viewer's first range selection on the device or until dismissed (OQ-127). The edit page keeps today's hint.
- **Remembered on the device.** "Opened" and "hint done" are kept in browser storage through `lib/prefs`, like the units choice (DS-1): no cookie, nothing sent to the server. Storage unavailable, blocked or unreadable: button emphasised and hint shown on every load, both still work, no message.
- **Budgets.** Technical design, Front end, Budget: HTML page 15 KB, app bundle 60,000 bytes gzip per entry (ADR-002). Only the button wiring and the two flags are static; they come out of the ADR-011 reserve (2,601 bytes at its last revision). The dialog is a dynamic `import()` (not counted). LCP and 60 fps scrubbing unchanged.
- Technical design (architect): `lib/prefs` keys; ADR-011 share.

## Out of scope
- The edit, published, upload, About and Impressum pages; the pinned map toggle (AP-18), route colouring (AP-9), axis toggle (AP-5) and the splitter in the dialog.
- Tours, animations, videos, other languages (DS-3); keyboard shortcuts.

## Acceptance criteria
- [ ] On `/w/{id}`, the header's control row shows "How to use" with an icon on a 360 px wide phone (portrait and landscape) and on desktop; not on `/edit/`, `/published/`, upload, About or Impressum pages.
- [ ] Fresh browser: the button is emphasised; after opening it once and reloading (also another workout), it is plain.
- [ ] The dialog shows the sections with the texts above, touch wording on a touch phone and mouse wording on desktop; a workout without a route has no fullscreen line and no map part in "Values at a point"; one without laps and splits has no laps line.
- [ ] The dialog closes with its close button, Esc and a tap outside it.
- [ ] Network log: no request for the dialog's code until the button is first used; the entry chunk stays under 60,000 bytes gzip (CI) and within the ADR-011 share.
- [ ] Fresh browser: the hint shows; after the first range selection (drag or lap/split row) or dismissing it, it does not show on a reload or another workout, and the laps table does not jump on the page where it went away.
- [ ] Storage blocked: the page works, button emphasised and hint shown on every load, no error.
- [ ] No cookie is set and no request carries the flags (network log).
- [ ] The edit page shows today's hint, unchanged, and no button.

## Open questions
- **OQ-126 Input type.** Suggest: decide by the primary pointer (CSS `pointer: coarse` means touch), as the existing hint does; a laptop with a touchscreen gets the mouse wording.
- **OQ-127 Hint lifecycle.** Suggest: the AP-7 hint on the viewing page becomes the first-visit hint (no second line); any range selection, by drag or lap/split row, counts; dismissed with a small × at the line's end; on the page where it went away the line empties but keeps its space; from the next load there is no line and no reserved space, without a layout shift (architect decides how, e.g. the inline head script that already applies the theme).
- **OQ-128 Sections left out.** Suggest: no route (no location data or fully hidden): no fullscreen line, "Values at a point" mentions only charts; no laps and splits section: no laps line; no charts: no range or share line, and "Values at a point" mentions only the route.
- **OQ-129 Emphasis and closing.** Suggest: emphasis is the accent background used by the selected axis toggle; plain is the Copy link look; the phone's back gesture does not close the dialog (no history entry).
