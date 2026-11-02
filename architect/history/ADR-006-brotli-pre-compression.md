# ADR-006 Brotli pre-compression: history

History of [architect/ADR-006-brotli-pre-compression.md](../ADR-006-brotli-pre-compression.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 2)

- **Date:** 10 Oct 2026 (revision 1, 11 Oct 2026: quality 11 → 9 after measurement, and the time trigger is replaced by triggers tied to the measuring machine. See Revision 1. Revision 2, 11 Oct 2026, F-03 QA r2 note: item 6, one compression at a time per process; two "Would choose differently if" bullets moved back to that section from Revision 1, unchanged)

## Superseded and struck-through text

- **Item 3:** 3. **Parameters: quality 9 (rev. 1; was 11), window `LGWin` 22 (4 MiB).** See Revision 1 for the measurements.
- **Consequences:** - ~~Quality 11 is slow in pure Go~~ (rev. 1): quality 9 compresses a 12-hour file in about 0.13 s.
- **Would choose differently if, first bullet:** - (rev. 1, replaces the 3 s trigger) **Size:** ...
- **Section heading:** `## Revision 1 (11 Oct 2026)` (the section's content is kept in the live ADR as the rationale for quality 9)
