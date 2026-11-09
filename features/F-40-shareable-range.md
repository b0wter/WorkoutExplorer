# F-40 Shareable range

- **Phase:** 2
- **Owner:** frontend
- **Lane:** normal
- **Requirements:** AP-19 (Could, phase 2); AP-7, AP-8, SH-6, AP-22 (dialog line); NFR Performance, NFR Privacy
- **Depends on:** F-17, F-18, F-39

## Goal
A viewer sends a friend a link that opens the workout with the selected range already selected.

## Scope
- **Fragment.** As AP-19: on the viewing page (`/w/{id}`) only, the range lives in the link's fragment, as time positions, never coordinates; cut to the published data; invalid values ignored silently; ignored without JavaScript. Format and rounding: OQ-130.
- **Button.** While a range is selected, the range panel on the viewing page shows "Copy link to this selection" (user decision 9 Nov 2026), next to Clear. It copies the viewing link with the range, with the same feedback and fallback as Copy link (SH-6, OQ-132). Not on the edit page; not in fullscreen (no panel there, AP-21).
- **Address bar and Copy link.** OQ-131.
- **F-39 dialog.** Adds the "Share a range" line (texts in F-39).
- **Budget.** Static code comes out of the ADR-011 reserve; LCP unchanged.
- Technical design (architect): link formats (ADR-020) gain the viewing-page fragment.

## Out of scope
- Cursor position, x axis choice, map zoom or scroll position in the link; scrolling or zooming to the range on opening.
- Any change to the preview card (SH-5) or the server.

## Acceptance criteria
- [ ] Selecting a range on `/w/{id}` (drag or lap/split row), using "Copy link to this selection" and opening the copied link in another browser shows the same range: map highlight and range panel with the same values (within OQ-130's rounding).
- [ ] The button shows only while a range is selected, only on the viewing page; on success it reads "Link copied" for 2 s; without the Clipboard API the link appears in a read-only, selected field.
- [ ] The range is only in the fragment: the server access log and the page request show no trace of it, and the cached static page is served unchanged.
- [ ] The fragment holds time positions only (no coordinates, no distance); a link opened after the data file is regenerated selects the same time span.
- [ ] The edit page and the published page neither read nor write the range; their token fragment is untouched, and opening `/edit/{id}#{token}` never selects a range.
- [ ] A range that ends past the workout's data, or starts before it, is cut to the data; with start and end hiding on, a range inside a hidden section shows the range panel but no positions on the map, the same as selecting it by hand.
- [ ] Malformed, empty, zero-length, reversed and entirely out-of-range values are each ignored without a message; the page opens with no range and works as usual.
- [ ] With JavaScript off the page shows as before; the summary still renders before the map and charts (LCP target unchanged).
- [ ] Address bar and Copy link behave as decided in OQ-131.
- [ ] The F-39 dialog shows the "Share a range" line in touch and mouse wording.

## Open questions
- **OQ-130 Format.** Suggest: as the spec assumes, `#range=1830-2415`, whole elapsed seconds from the workout start (AP-5), start rounded down and end up; opening selects the samples within; panel values may differ slightly from the sender's when samples are not on whole seconds (accepted). No cursor, axis, scroll or zoom.
- **OQ-131 Live link or button only.** The spec assumed a live-updating address bar and Copy link (SH-6) copying the range. Suggest, now that there is a button: button only. The address bar is not changed while selecting; Copy link keeps copying the plain viewing link; an opened link's range stays in the address bar until the viewer changes or clears the range, then it is removed without a history entry.
- **OQ-132 Button look.** Suggest: same look and feedback as Copy link ("Link copied", fallback field); on a narrow phone panel it may wrap to its own line, panel height growing accordingly.
