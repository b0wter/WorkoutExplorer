# F-53 My workouts on this device

- **Status:** draft
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** critical (stores edit links, which are secrets, in the browser)
- **Requirements:** AC-7 (new), AC-1, AC-4, AC-6, UP-7, INF-1, NFR Privacy (no data outlives its workout; no cookie)
- **Depends on:** F-20 (published page), F-29 (About page)

## Goal
A sharer finds the workouts they uploaded from this browser again, without accounts and without keeping the edit links somewhere else.

## Scope
- After a successful publish (AC-6), the browser keeps an entry for the workout in its own storage (`localStorage`, no cookie, never sent to the server): title, sport, start date, expiry, viewing link and edit link (OQ-218, OQ-219).
- A "My workouts" list on the upload page (OQ-220) shows the entries of this browser, newest first, each with the viewing link and "Edit".
- Entries past their expiry are removed when the list is shown; a workout the server no longer has is removed when its edit page answers not found (OQ-221).
- Deleting a workout from its edit page (AC-4) removes its entry. The owner can remove any entry from the list ("Forget"), which deletes nothing on the server, and clear the whole list.
- Broken or unavailable storage: no list, no error.
- About page (INF-1): says what the list stores, that it stays in this browser only, and that anyone using this browser can open those edit links.
- The server stores nothing new and gets no new endpoint.

## Out of scope
- Syncing between browsers or devices; export or import of the list; drafts (OQ-219).

## Acceptance criteria
After the open questions are decided.

## Open questions
- [ ] **OQ-218 Keep edit links in the browser at all?** Anyone with access to this browser profile can edit or delete the workouts. Suggested: yes, with a one-line note on the list and on the published page ("Edit links are saved in this browser"), and the "Forget" and "Clear list" controls.
- [ ] **OQ-219 Which workouts.** Suggested: published workouts only; drafts are deleted after 48 h anyway and have no viewing link.
- [ ] **OQ-220 Where the list lives.** Suggested: on the upload page under the drop zone, hidden when empty; no new page.
- [ ] **OQ-221 Checking whether a workout still exists.** Suggested: no server request just for the list; rely on the stored expiry, and remove an entry when its edit page answers not found.
- [ ] **OQ-222 Title changes.** Suggested: the edit page updates the stored title and expiry on every successful save.
