# ADR-057 history

## Original → rev. 1 (20 Nov 2026, F-56 v0.2)

- **Item 3 (original), last sentences:** "A section that touches a hidden or trimmed part is **dropped, never clipped**. One function computes this; the owner `PATCH` check, the removal after a radius or trim change, and the build all call it." (Radius-hidden pins were deleted from the row.)
- **Item 5 (original):** "**Every pin change is remove-first.**" (No exception for changes that leave the public pins file unchanged.)
