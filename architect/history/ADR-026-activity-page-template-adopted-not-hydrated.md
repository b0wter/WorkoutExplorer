# ADR-026 Activity page template: history

History of [architect/ADR-026-activity-page-template-adopted-not-hydrated.md](../ADR-026-activity-page-template-adopted-not-hydrated.md).

## Rev. 1, 24 Oct 2026

F-13 design 1.1 (two columns on desktop, pinnable map on the phone). Additions only, no decision reversed:

- Item 4: controls that are useless without JavaScript (the map pin toggle) are never in the template; the client adds them inside a slot it owns, as an overlay.
- Item 5: layouts are chosen by media queries and a template class, never by script.

## Rev. 0, 24 Oct 2026

Accepted with the F-13 design 1.0.
