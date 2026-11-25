# F-75 Entry bundle trims

- **Status:** planned (25 Nov 2026); no open product question
- **Category:** Planned maintenance (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** normal, without an architect step: [ADR-011](../architect/ADR-011-activity-page-bundle-allocation.md) decision 7 is its design; QA at medium (T-1 timing)
- **Requirements:** NFR Performance (LCP, entry budget 62,000 gzip); AP-7, AP-17, AP-18, AP-9, AP-21 (behaviour unchanged)
- **Depends on:** F-74

## Goal
Bring the viewing page's entry reserve back above 1,000 bytes gzip, so later features can add entry code (ADR-011 decision 5).

## Scope
- Trims T-1 (range panel lazy), T-2 (map-only glue into the map chunk) and T-3 (route-colouring tokens into the map stylesheet) of ADR-011 decision 7, as one task; T-4 only if T-1 to T-3 save less than 1,500 bytes.
- The engineer measures the entry before and after each step and books the figures in ADR-011.
- No visible change except what ADR-011 accepts (panel, toggle, strip or splitter appear about one round trip later).

## Out of scope
- Raising the 62,000 budget (stays, user 25 Nov 2026).

## Acceptance criteria
- [ ] Entry reserve at least 1,000 bytes gzip after the task (ADR-011 expects about 2,200 to 2,800); `make ci` passes the 62,000 check.
- [ ] Range selection, range link (F-40), lap tap, pinned map, fullscreen readout, splitter drag and route colouring behave as before, in both layouts and themes; a range link or lap tap before the range chunk arrives shows the panel when it lands.
- [ ] A workout without a route downloads no map-only code or tokens.
- [ ] LCP and 60 fps scrubbing on ride-12h unchanged.
