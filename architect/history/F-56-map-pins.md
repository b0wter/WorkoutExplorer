# F-56 map pins design: history

## v0.1 → v0.2 (20 Nov 2026), user decision on pins a later edit would hide

Superseded v0.1 text, verbatim in substance:

- **PN-4 (v0.1):** one public rule; "A section touching a hidden or trimmed part is **dropped, never clipped**"; "One definition for the `PATCH` check, the removal and the build".
- **PN-5 (v0.1):** "a sent pin that is not public gets `400` … When the `PATCH` changes a radius or the trim and `pins` is not sent, stored pins that are no longer public are **removed in the same row write** (OQ-236, OQ-237)." Why: "The row never holds a pin the next build would drop because of the owner's own change."
- **PN-8 (v0.1):** "every pin change is remove-first (ADR-057 item 5)", with no exception for changes that leave the public file unchanged.
- **PN-12 (v0.1):** "while unsaved sliders or trim shade a pin, it is drawn dimmed in every view; after a save whose response lacks a pin the browser did not remove itself, the one-line notice (AC 36). With a pin change in the same save, the browser leaves the shaded pins out of the sent set."
- **Verification (v0.1):** "Raising a radius, then trimming: exactly the no-longer-public pins are removed."
- **F-54 hand-off (v0.1):** "the refusal and removal lines".
