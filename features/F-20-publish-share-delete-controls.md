# F-20 Publish, Share and delete controls

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** UP-4 (publish), SH-6 (Share button), AC-1 and AC-6 (published page with the edit link), AC-4, SH-8 (owner downloads original), SH-10 (notice); Technical design: Publish and view flow
- **Depends on:** F-15, F-19

## Goal
The owner publishes from the edit page, lands on the published page to copy the viewing link and save the edit link, and can download the original and delete the workout from the edit page.

## Scope
- Draft: no viewing link and no edit link on the page (UP-4); no first-open notice. One line: "Unpublished drafts are deleted 48 hours after upload." (UP-7), not shown after publishing.
- Publish button on a draft: saves unsaved changes first and publishes only if that save succeeds (OQ-85); only after a successful publish, the browser goes to `/published/{public_id}#{token}` (token in the fragment only, as on the edit page). A failed save or publish stays on the edit page with the error.
- Published page (AC-6): viewing link with Copy, edit link with Copy and a prominent warning ("Save this link. It is the only way to change or delete this workout, and it cannot be recovered."; wording reviewed by the user), and "Back to edit". A reload shows it again. Opened while the workout is a draft, it sends the owner to the edit page; wrong or missing token, unknown or expired workout: the generic not-found page, as on the edit page.
- Edit page after publishing: Share button showing the viewing link with a copy button, once published (hidden on a draft; OQ-83 revised 30 Oct 2026). Once published, the Share block also has "Show links", which opens the published page `/published/{public_id}#{token}` (OQ-86, 30 Oct 2026).
- No private state (SH-2, 30 Oct 2026): the built private handling (Share and Copy link hidden while private, the published page while private) is removed (removal of built behaviour; architect hand-off).
- Download original FIT (owner only): the stored file, which is the upload with identifying data removed (UP-9).
- Delete with a confirmation stating that deletion is permanent; afterwards a confirmation screen with no workout data.
- Notice in the edit panel: copies viewers already downloaded, or previews messengers already made, cannot be recalled (SH-10).

## Out of scope
- Server side of each action (F-12, F-15).

## Acceptance criteria
- [ ] A fresh draft shows neither link: no viewing link, no edit link, no Share, no Copy link, no edit-link notice.
- [ ] A draft's edit page shows "Unpublished drafts are deleted 48 hours after upload."; after publishing the line is gone.
- [ ] Publishing a draft makes the viewing link show the activity page, and only then the browser opens `/published/{public_id}#{token}`; the token appears only in the fragment, never in a request URL, and requests carry it only as a header.
- [ ] Publish with unsaved changes saves first; if the save or the publish fails, the browser stays on the edit page with the error and the published page is not opened.
- [ ] The published page shows the viewing link `/w/{public_id}` and the edit link `/edit/{public_id}#{token}`, each with a Copy that puts exactly that URL on the clipboard, the save warning, and "Back to edit", which opens the edit page.
- [ ] Reloading the published page shows it again; opening it later for a published workout shows it; opening it for a draft sends the owner to the edit page; a modified token shows the generic not-found page.
- [ ] On the edit page of an unlisted workout, Share shows `/w/{public_id}` and copy puts exactly that URL on the clipboard, with no token; Share is hidden on a draft. There is no control that makes a published workout private or a draft again.
- [ ] Once published, "Show links" in the Share block opens `/published/{public_id}#{token}`; it is absent on a draft.
- [ ] Download original returns the stored original (the uploaded file with identifying data removed, UP-9).
- [ ] Cancelling the delete confirmation keeps the workout; confirming removes it, and both links then show the generic page.
- [ ] The SH-10 notice is visible in the edit panel before and after publishing.
