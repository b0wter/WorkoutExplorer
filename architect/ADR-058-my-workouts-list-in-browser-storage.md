# ADR-058 My workouts list: one browser-storage key holding IDs and tokens, written only by the owner pages

- **Status:** accepted (20 Nov 2026), with the F-53 design v0.1
- **Date:** 20 Nov 2026
- **Features:** F-53 (introduces it); F-54 (translates what it displays)
- **Supersedes:** none. [F-11](F-11-upload-page.md) U-10 is unchanged (the upload page still touches no storage). Widens the TD rule "`lib/prefs` is the only code touching browser storage" by one module and one key; adds one exit for the token to [F-19](F-19-edit-page-and-settings.md) E-2 (`lib/owner`)
- **Requirements:** AC-7, AC-1, AC-4, UP-5, UP-7, NFR Privacy

## Context

AC-7 keeps every edit link uploaded from a browser in that browser's storage (OQ-218). An edit link is a secret: whoever reads it can change or delete the workout. Storage survives releases, can be edited by hand or by another script on a shared machine, and is read back into links the page renders. Whatever is stored now is read by every later release.

## Decision

1. **One `localStorage` key, `wx.workouts`, value JSON `{"v":1,"workouts":[…]}`.** Never a cookie, never `sessionStorage` or IndexedDB, never sent anywhere.
2. **An entry stores parts, not links:** `id` (public ID), `token` (edit token), `title`, `sport`, `date`, `created`, `expires`, `published` (types and sources in F-53 MW-2). No URL, origin, description, pins, summary or file name. Edit and viewing links are **rebuilt at display** from `id` and `token` after both pass ADR-020's 22-character base62 check, as root-relative paths (`/edit/{id}#{token}`, `/w/{id}`). A stored value can therefore never become a `javascript:`, foreign or other-path link.
3. **Every read validates.** A value that is not version 1 JSON of that shape is unreadable (the list page says "not available" and never rewrites it). Per entry, each field is checked on its own; an invalid field counts as absent, never repaired.
4. **Writers:** only the owner pages (`src/edit`) add, update and remove entries; the My workouts page only prunes, forgets and clears. The upload page and the viewing page never touch the key. The edit page replaces an unreadable value with a fresh list.
5. **Not found removes only the matching link:** an owner request answering ADR-025's not-found removes the entry only when its stored token equals the address's token, because that answer is the same for a wrong token and an unknown workout.
6. **Version rule:** a format change bumps `v`. The new reader converts the old value or treats it as unreadable; no compatibility is promised (entries live at most 30 days, like their workouts).

## Consequences

- AC 54 holds by construction; the only stored text that reaches the screen is `title`, always as text.
- `lib/mine` is a second storage module next to `lib/prefs`; `lib/owner` hands the token to `src/edit` for the list write (its second exit after the published page's edit link).
- Anyone using the browser, or a script running on this origin, can read the tokens. Accepted by OQ-218 and stated in the page note; the CSP (ADR-032) allows only our own scripts.
- Lost-update race between two owner tabs writing in the same moment: one update may be lost until that page's next save. Accepted.

## Rejected

- **Store the edit and viewing links as strings:** every reader must re-validate a URL (scheme, origin, path, fragment), and a format change in ADR-020 would leave stale links in storage.
- **One key per workout:** Clear list and ordering need a key scan, and an interrupted Clear leaves fragments.
- **Upload page writes the first entry:** it has no sport, date or title (the upload answer carries only the links), it would break F-11 U-10 and pull storage code into a second entry; the edit page it always navigates to writes the entry with complete values.
- **Remove on any not-found by ID:** a mistyped or foreign link for the same ID would erase a working edit link.

## Would choose differently if

- Tokens ever change (rotation, revocation): then entries need the server's word on validity, not only not-found by token.
- The list must survive between devices or be exported: then it is server data and AC-3's "no accounts" decision is reopened.
