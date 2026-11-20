# ADR-014 Map tiles and third-party privacy: history

History of [architect/ADR-014-map-tiles-and-third-party-privacy.md](../ADR-014-map-tiles-and-third-party-privacy.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 2)

- **Date:** 12 Oct 2026 (revision 1, same day: user decision on OQ-18, style liberty instead of positron, verified against the live style; tile host added to `img-src`; About-page wording handed to F-29 after OQ-17; revision 2, 14 Oct 2026, F-06 QA r1-5: the CSP and `Referrer-Policy` are sent on the worker asset too; exact CSP for F-22)

## Revision 3 (24 Oct 2026, F-14 design): text replaced

- Decision 1: "F-14 uses OpenFreeMap's dark style if one exists (decision 28). The URL is one constant in `map-style.ts`."

## Revision 4 (20 Nov 2026, F-61 design): text replaced

- Title: "ADR-014 Map tiles: OpenFreeMap liberty, the tile host as the only third party, no referrer, no credentials"
- Decision 2: "**The tile host is the only third party the viewer's browser contacts** (TD invariant I-6). No fonts, scripts, analytics or images from anywhere else. MapLibre has no telemetry; its logo stays off."
- Decision 6 table: `connect-src 'self' https://tiles.openfreemap.org`; `img-src 'self' data: blob: https://tiles.openfreemap.org`
