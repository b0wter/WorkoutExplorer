# F-53 My workouts on this device

- **Status:** draft; OQ-218 to OQ-222 decided 14 Nov 2026; acceptance criteria written 19 Nov 2026; OQ-278, OQ-279 decided 19 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** critical (stores edit links, which are secrets, in the browser)
- **Requirements:** AC-7 (new), UP-1, UP-5, UP-11, SH-2, AC-1, AC-4, AC-6, UP-7, INF-1, INF-5, NFR Privacy (no data outlives its workout; no cookie)
- **Depends on:** F-20 (published page), F-29 (About page)

## Goal
A sharer finds the workouts they uploaded from this browser again, drafts included, without accounts and without keeping the edit links somewhere else.

## Scope
- After every successful upload, the browser keeps an entry for the workout in its own storage (`localStorage`, no cookie, never sent to the server): title, sport, start date, status (draft or published), expiry and edit link; the viewing link is added once it is published (OQ-218, OQ-219).
- A dedicated **My workouts** page, linked from the upload page, lists this browser's entries in a table, newest upload first (OQ-220). Columns:
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
- The server stores nothing new and gets no new endpoint. Texts in English; F-54 (built last) translates them.

## Out of scope
- Syncing between browsers or devices; export or import of the list. Deleting a workout from the list (that stays on its edit page). Copy buttons on the list.

## Assumptions
- One entry per workout (by its public ID); a later update replaces it.
- No limit on the number of entries; expiry (at most 30 days) keeps the list short.
- An empty list shows "No workouts uploaded from this browser yet."; the upload page's link is always shown.
- Edit and View open in the same tab.
- Without JavaScript the page says that it needs JavaScript.
- Only the upload, edit, published and My workouts pages read or write the list; the viewing page never touches it.

## Acceptance criteria
- [ ] After a successful upload from the upload page (FIT or zip, UP-11), the browser holds one entry with title, sport, start date, status Draft, expiry and edit link; no cookie is set, and no request to the server carries any of it (checked in the browser's network view).
- [ ] The upload page links to My workouts; the page lists the entries newest upload first with the columns of Scope; an empty list shows the empty-list line.
- [ ] Expires shows a relative time; for a draft, the earlier of 48 hours after upload and the chosen lifetime (UP-7).
- [ ] Publishing on the edit page sets the entry to Published and adds the viewing link; every successful save updates title, status and expiry; deleting the workout removes its entry.
- [ ] Opening My workouts removes entries past their expiry and makes no request to the server.
- [ ] An edit page that answers not found (expired, deleted by the operator, INF-3, or unknown) removes the entry for that workout.
- [ ] Whenever the edit page opens with a valid edit link and the list has no entry for it (iOS Shortcut, INF-5; a pasted edit link; a workout uploaded before F-53; someone else's edit link), it adds one (OQ-278).
- [ ] Forget removes one entry at once; Clear list asks once, "Remove all workouts from this list? They stay online, but without their edit links you cannot change or delete them.", and removes all entries only on confirm (OQ-279); afterwards the workouts still open on their viewing and edit links.
- [ ] The note "Edit links are saved in this browser. Anyone using this browser can open them." is on My workouts and on the published page (AC-6).
- [ ] With browser storage unavailable, blocked, full or holding an unreadable value, My workouts says the list is not available in this browser; upload, edit and publish still work, without an error.
- [ ] A title with markup or control characters (hostile-text sample) is shown as plain text.
- [ ] Test: the viewing page's code neither reads nor writes the list, so no edit link is in reach of a viewing page.
- [ ] The About page's "In your browser" paragraph names the list and its edit links (INF-1).
- [ ] The server is unchanged: no new endpoint, table, column or log line.

## Decisions (user, 14 Nov 2026)
- **OQ-218:** edit links are kept in the browser, with the note and the Forget and Clear list controls.
- **OQ-219:** changed: every uploaded workout is listed, drafts included.
- **OQ-220:** changed: a dedicated My workouts page with a table, linked from the upload page; columns as in Scope.
- **OQ-221:** no server request for the list; stored expiry, and not-found on the edit page.
- **OQ-222:** title, status and expiry updated on save and publish.

## Decisions (user, 19 Nov 2026)
- **OQ-278:** the edit page adds an entry whenever it opens with a valid edit link and none exists.
- **OQ-279:** Clear list asks once; Forget removes at once.
