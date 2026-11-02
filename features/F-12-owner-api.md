# F-12 Owner API: draft data, settings, delete, original download

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** AC-4 (delete), AC-5, UP-4 (server), UP-7 (lifetime change), SH-2, SH-8 (owner downloads original), UP-9 (original without identifying data), NFR Security (originals never public); Technical design: Publish and view flow, Security and privacy (IDs and tokens, Draft workouts, Originals)
- **Depends on:** F-09

## Goal
Every action the edit-link holder can take, as token-protected API endpoints, plus the one delete routine that expiry and the operator reuse.

## Scope
- Endpoint paths as in the [F-12 design](../architect/F-12-owner-api.md) (Endpoints), all under `/api/workouts/{public_id}`.
- Edit token accepted only in the `Authorization: Bearer` header; compared against the stored hash. Wrong or missing token returns the same not-found response as an unknown workout. A request with any query string gets the not-found response too, even with a valid header ([ADR-025](../architect/ADR-025-owner-api-access-and-not-found-response.md)). The edit link stays `/edit/{public_id}#{token}`, token in the fragment.
- Read the draft: settings, metadata and `draft.bin` (served by Go only with the token).
- Update settings: title, description, `hide_ends`, `allow_download`, lifetime. Visibility changes only through publish (F-15) and cannot be undone (SH-2, 30 Oct 2026); the built `visibility` field (unlisted ↔ private) is removed (removal of built behaviour; architect hand-off).
- Lifetime: `expires_at` must be in the future and at most `created_at` + 30 days.
- Title and description stored as plain text: title at most 100 characters after trimming leading and trailing white space, description at most 1,000; characters are Unicode code points (decision log, 23 Oct 2026).
- Download `original.fit` (owner only): the stored file, i.e. the upload with identifying data removed (UP-9, F-09), not a byte-identical copy. Offered as `workout-YYYY-MM-DD.fit`, from the local start date, or `workout.fit` without a start time; ASCII only, never the title.
- Delete: one routine removing the row and `private/{id}/`; reused by F-21 and F-27. F-15 extends it to `public/{public_id}/`, F-27 to reports.
- Changes to a published workout trigger regeneration through the F-15 hook once it exists.

## Out of scope
- Publishing and file regeneration (F-15), hiding (F-16), export file (F-24), UI (F-19, F-20).

## Acceptance criteria
- [ ] Every owner endpoint returns the not-found response without the token, with a wrong token, with the token in a query string instead of the header, and with any query string even when the header carries the valid token.
- [ ] No owner request turns a published workout back into a draft or hides it again; it stays unlisted until delete or expiry (SH-2, 30 Oct 2026).
- [ ] Setting the lifetime to 31 days after upload, or to a time in the past, is refused; 30 days after upload is accepted.
- [ ] A title containing `<script>` is stored and returned as plain text.
- [ ] Downloading the original returns a file byte-identical to the stored `original.fit`: the upload with only the UP-9 removals applied (F-09). Its file name is `workout-YYYY-MM-DD.fit` from the local start date, or `workout.fit` for a workout without a start time.
- [ ] After delete, the row and `private/{id}/` are gone; every owner endpoint returns not found for that workout, including a second delete. (Public folder: F-15; reports: F-27.)
- [ ] No URL served by the Go process returns `original.fit` or `draft.bin` without the valid token header, checked with guessed paths (file names under `/`, `/w/{public_id}/`, `/private/{id}/`, owner paths without a token or with the token in the query string, the internal ID instead of the public ID, encoded traversal). (`public/`: F-15; Caddy: F-22.)
- [ ] A title of 100 characters after trimming and a description of 1,000 are accepted; 101 and 1,001 are refused, counted as Unicode code points (e.g. `😀` counts as one).
