# F-21 Lifetime and automatic expiry

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** UP-7, AC-4, SH-10 (expiry within 60 s), NFR Privacy (no data outlives its workout); Technical design: Publish and view flow (Publish 7), Security and privacy (Lifetime)
- **Depends on:** F-12, F-15

## Goal
Every workout is deleted with all its data when its lifetime ends; a workout never published is deleted 48 hours after upload.

## Scope
- Expiry job using the F-12 delete routine: row, reports, `private/{id}/`, `public/{public_id}/`.
- Runs often enough to meet SH-10's 60 s (assumed every minute instead of the technical design's hourly; see README), and once at startup.
- Drafts included. A draft (never published) is deleted 48 hours after upload, counted in hours, or at the end of its chosen lifetime if that comes first (UP-7, decided 30 Oct 2026). Publishing ends the 48-hour rule: the chosen lifetime then counts from upload.
- After expiry the viewing link shows the generic page (wording: see README); no Open Graph data.
- Expiry date in the published page (F-13) and regenerated when the owner changes the lifetime.
- No reminder before expiry.

## Out of scope
- Lifetime setting UI (F-19), lifetime validation (F-12).

## Acceptance criteria
- [ ] A workout with `expires_at` one minute ahead is gone within 60 s after that time: no row, no reports, no private or public files; the viewing link shows the generic page.
- [ ] An unpublished draft is deleted the same way within 60 s after 48 hours from upload, even when its chosen lifetime is longer; one minute before that it still exists.
- [ ] A workout published 47 hours after upload with a 7-day lifetime still exists 49 hours after upload and is deleted 7 days after upload.
- [ ] Publishing a draft that the 48-hour rule has already deleted gets the not-found response.
- [ ] A draft past 48 hours from upload that the sweep has not yet deleted gets the owner API's not-found response, like an expired workout (30 Oct 2026).
- [ ] Server stopped over an expiry time: on restart the overdue workout is deleted before Go serves it again. While Go is down, Caddy may keep serving its published page (OQ-87, 31 Oct 2026); SH-10's 60 s holds only while Go runs.
- [ ] Changing the lifetime from 7 to 20 days updates the expiry date shown on the public page.
- [ ] A test lists every table and folder that can hold workout data and asserts each is empty for an expired workout.
