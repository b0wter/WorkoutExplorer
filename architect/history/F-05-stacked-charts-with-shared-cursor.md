# F-05 Stacked charts with shared cursor: history

History of [architect/F-05-stacked-charts-with-shared-cursor.md](../F-05-stacked-charts-with-shared-cursor.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Header version line as written (revision list)

- **Version:** 1.4 (1.4, QA r2 and the operator's manual run: the readout wraps with auto height and is sticky; only the chart area reserves a fixed height, inside `{#key data}`, rewritten per `data` (D-16 revised, D-17; QA-05-r2-1, QA-05-r2-2); touch down places the cursor at once, `pointercancel` restores it (D-7 revised, user decision on P3); a touch is classified once at 8 px / 30°, and a scrub blocks page scrolling for the rest of the touch through one non-passive `touchmove` listener (D-18, user decision); a drag need not continue onto another chart (user decision on P2); all in ADR-010 rev. 2; four tests for QA-05-r2-3; ceilings +30,000 / 7,600, 200 bytes from the reserve (ADR-011 rev. 2). 1.3, engineer stop at bundle check after v1.2: D-16 height set in `onMount` via `cssText`, no `style=` attributes (measured +29,624 / 7,265 within +29,800 / 7,400, ceilings unchanged); Vitest `browser` resolve condition, `golden.mjs` path, `Path2D` stub and scrolling readout recorded. 1.2, QA r1: listener errors caught per listener and reported (D-14, ADR-008 rev. 1); `chartHeight` fixed per data value (D-15); the stack reserves its own height from the first render (D-16); readout cells grow instead of clipping; y-axis width from one constant; component smoke tests in jsdom in CI (ADR-012); ceilings +29,800 / 7,400 from the ADR-011 reserve. 1.1, engineer stop at the bundle measurement: ceilings raised to +29,500 / 7,100 (D-12, ADR-011); transform strings allowed on the hot path (D-10); x `space: 70` and y steps by `decimals` (D-13); sample-count check in `$effect.pre`; `onready` without charts; a new primary touch replaces a stale one; `uplot-options.ts` helpers and `height` meaning. 1.0: first version)

## Closed open question

| OQ-13 | The spec does not say what happens to the cursor when the mouse leaves the chart or the finger lifts. Decided for now (D-8): it stays, so the map marker and readout keep the last position. Alternative: hide on mouse leave (desktop only) | Nothing (default applies); confirm in F-07 | user (product) |

Decided by the user on 15 Oct 2026: the cursor stays (`INDEX.md`, OQ-13).
