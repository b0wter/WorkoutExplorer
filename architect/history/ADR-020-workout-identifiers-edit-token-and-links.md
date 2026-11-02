# ADR-020 Workout identifiers, edit token and link formats: history

History of [architect/ADR-020-workout-identifiers-edit-token-and-links.md](../ADR-020-workout-identifiers-edit-token-and-links.md). Read it only when you need the history.

## Revision 1 (23 Oct 2026, F-11 design)

Added the first-open marker `&new` in item 7 and its consequence. The superseded sentence in item 7 read: "The fragment is the bare token, nothing else." It now reads "The fragment the server returns is the bare token, nothing else."

## Revision 2 (30 Oct 2026, F-20 design v0.2): first-open marker removed, published page added

The user replaced the first-open notice with the published page (AC-6, decision log 30 Oct 2026). Removed from item 7, verbatim in substance:

- "**(rev. 1) First-open marker.** Right after an upload, the upload page (F-11) opens `/edit/{public_id}#{token}&new`. This tells the edit page that this is the first open, so it shows the edit link once with the save warning (AC-1, F-20)." The marker existed only in the browser; the edit page took the token up to the first `&` (kept), and removed `&new` with `history.replaceState` before showing anything.
- Alternatives considered then: browser storage (ruled out, F-11 AC 8), `history.state` (does not cross a document navigation), the Navigation API (Baseline 2026 only).
- Consequence removed: "The browser's global history may keep the address with `&new`. Opening it from there later shows the save warning again."
- Requirements line read "AC-1 (secret edit link, shown once)".

Also superseded (rev. 1): the F-09 design's F-11 hand-off (v1.11) said to pass "first open" to the edit page "without browser storage (for example `history.state`), never in the URL". `history.state` cannot cross a document navigation, so revision 1 puts the marker in the fragment instead (reasons in item 7).
