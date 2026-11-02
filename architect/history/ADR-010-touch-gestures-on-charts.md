# ADR-010 Touch gestures on charts: history

History of [architect/ADR-010-touch-gestures-on-charts.md](../ADR-010-touch-gestures-on-charts.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 2)

- **Date:** 11 Oct 2026. Revision 1, same day: a new primary touch replaces a stale tracked touch. Revision 2, 12 Oct 2026: the operator's phone run and the user's touch decisions (P2, P3, scroll lock); see "Revision 2".

## Revision 2

| Changed | From (rev. 1) | To (rev. 2) | Reason |
| --- | --- | --- | --- |
| Touch down | No action | Sets the cursor; the value from before is kept for `pointercancel` | User decision on P3 (option B) |
| Listeners | All passive, no `preventDefault`, no `touch*` | One non-passive `touchmove` per chart, the scroll lock | User decision 2; WebKit takes over drags mid-gesture (P2) |
| Reducer state | `touch { pointerId, before }` | Adds `x0, y0, axis` | Direction rule |
| P2 | Implicit capture claimed to carry a scrub across charts | Not required | User decision on P2 |

Revision 1's "Would choose differently if" named a superseding ADR for JavaScript direction detection. Revision 2 keeps the browser's `touch-action` model and only adds the lock on top of it, so it is recorded here as a revision instead.
