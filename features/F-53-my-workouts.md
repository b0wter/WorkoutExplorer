# F-53 My workouts on this device

- **Status:** draft; OQ-218 to OQ-222 decided 14 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** critical (stores edit links, which are secrets, in the browser)
- **Requirements:** AC-7 (new), UP-1, UP-5, SH-2, AC-1, AC-4, AC-6, UP-7, INF-1, NFR Privacy (no data outlives its workout; no cookie)
- **Depends on:** F-20 (published page), F-29 (About page)

## Goal
A sharer finds the workouts they uploaded from this browser again, drafts included, without accounts and without keeping the edit links somewhere else.

## Scope
- After every successful upload, the browser keeps an entry for the workout in its own storage (`localStorage`, no cookie, never sent to the server): title, sport, start date, status (draft or published), expiry and edit link; the viewing link is added once it is published (OQ-218, OQ-219).
- A dedicated **My workouts** page, linked from the upload page, lists this browser's entries in a table, newest upload first (OQ-220). Proposed columns:
  - **Workout:** title, or the suggested or default title when empty (UP-5).
  - **Sport.**
  - **Date:** the workout's start date.
  - **Status:** Draft or Published.
  - **Expires:** relative ("in 3 days"); for a draft, the 48-hour draft deadline when it comes first.
  - **Links:** Edit, and View once published.
  - **Forget:** removes the entry only.
- Entries past their expiry are removed when the page is shown. A workout the server no longer has is removed when its edit page answers not found; no server request just for the list (OQ-221).
- The edit page updates the stored title, status and expiry on every successful save and on publish (OQ-222). Deleting a workout from its edit page (AC-4) removes its entry. "Clear list" removes all entries; nothing on the server is deleted.
- A one-line note on the page and on the published page: "Edit links are saved in this browser. Anyone using this browser can open them." (OQ-218)
- Broken or unavailable storage: the page says the list is not available in this browser; no error.
- About page (INF-1): the "In your browser" paragraph names the list and its edit links.
- The server stores nothing new and gets no new endpoint. Texts in the four languages of F-54 if F-54 is built first; otherwise F-54 translates them.

## Out of scope
- Syncing between browsers or devices; export or import of the list.

## Acceptance criteria
To be written from the scope.

## Decisions (user, 14 Nov 2026)
- **OQ-218:** edit links are kept in the browser, with the note and the Forget and Clear list controls.
- **OQ-219:** changed: every uploaded workout is listed, drafts included.
- **OQ-220:** changed: a dedicated My workouts page with a table, linked from the upload page; columns as proposed above.
- **OQ-221:** no server request for the list; stored expiry, and not-found on the edit page.
- **OQ-222:** title, status and expiry updated on save and publish.
