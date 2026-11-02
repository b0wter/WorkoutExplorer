# F-11 Upload page

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** UP-1, UP-2 (messages), UP-8 (notice), UP-10 (upload messages), AC-1, DS-3, INF-1 (footer link), NFR Compatibility; Technical design: Front end (Implementation notes)
- **Depends on:** F-09, F-10

## Goal
The start page where a sharer drops or picks a FIT file and lands on the draft of their workout.

## Scope
- Drop zone and file picker (no PWA, no share target). One file per upload: if several files are dropped at once, only the first is uploaded, with no message.
- While an upload is running, the drop zone and the file picker are disabled, so no second upload starts until the first ends (success or error).
- Client-side 20 MB check (20,000,000 bytes) before sending, with the same message as the server: "This file is larger than 20 MB."
- Progress while uploading and parsing; plain error messages from the server shown as text (not FIT, unsupported type, too large, rate limit, too many uploads at once, unreadable upload, server error; wording in F-09 and UP-10).
- On load, `GET /api/status`; when `uploads_open` is false, the notice "Uploads are paused because storage is full. Please try again later." replaces the drop zone.
- On success, the browser opens the new draft's edit link. The edit page there (F-19) shows no links while the workout is a draft; both links are shown after publishing (F-20, AC-6).
- Page texts (accepted for now, 23 Oct 2026; refined wording can come later): heading "Workout Explorer"; intro "Upload a FIT file to get a page with map, charts and laps that you can share."; drop zone "Drop a FIT file here" with the button "Choose a file"; progress "Uploading… 42 %", then "Reading the workout…"; footer "About". Messages are as above, in F-09 and UP-10.
- Footer with the About link.
- Works on phone and desktop browsers in the compatibility list.

## Out of scope
- The edit page itself (F-19, F-20). Remembering uploads in the browser (Later).

## Acceptance criteria
- [ ] Dropping a valid `.fit` file on desktop, and picking one on iOS Safari, both open the new draft's edit link in the browser. The edit page rendering there is checked in F-19 (AC 1); the same pick on Android Chrome is checked in F-32.
- [ ] A 25 MB file is refused in the browser with a plain message; no request is sent.
- [ ] A non-FIT file and a course file show the server's message on the page; the drop zone stays usable for another try.
- [ ] With storage full, the notice shows before any file is picked and no drop zone is offered.
- [ ] When the rate limit is hit, the page shows "Too many uploads. Please try again later."; when too many uploads are being processed at once, "Too many uploads right now. Please try again in a minute." The drop zone stays usable in both cases.
- [ ] A file of exactly 20,000,000 bytes is sent; one of 20,000,001 bytes is refused in the browser with "This file is larger than 20 MB."
- [ ] While an upload is running, the drop zone and the file picker are disabled, and dropping another file starts no second upload; after an error both are usable again.
- [ ] The page sets no cookies and stores nothing in browser storage.
