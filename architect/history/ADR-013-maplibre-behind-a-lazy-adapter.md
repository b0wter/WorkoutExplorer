# ADR-013 MapLibre behind a lazy adapter: history

History of [architect/ADR-013-maplibre-behind-a-lazy-adapter.md](../ADR-013-maplibre-behind-a-lazy-adapter.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 4)

- **Date:** 12 Oct 2026 (revision 1, 14 Oct 2026, F-06 QA r1: touch-quiet counts fingers from touch events, decision 5; a partial mount is cleaned up like `destroy`, decision 6; revision 2, 14 Oct 2026, the phone start-up measurement fired the trigger: decision 7 and a new trigger of 300 ms, accepted by the user (OQ-21); the re-run after decision 7 stayed within it; revision 3, 14 Oct 2026, F-06 QA r3: decision 8 (route as one source with two features and layers), style validation in CI, decision 7; revision 4, 15 Oct 2026, F-07 design D-8: the capped pixel ratio follows changes of `devicePixelRatio` (QA-06-r4-1), decision 7)

## Would choose differently if, superseded trigger

  Rev. 0 had a 100 ms trigger with these remedies in a different order. The first phone run fired it at 264–379 ms while MP1 passed; that run most likely had Low Power Mode on and is superseded.
