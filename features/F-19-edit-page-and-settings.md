# F-19 Edit page: draft view and settings panel

- **Phase:** 1 MVP
- **Owner:** both (backend first)
- **Requirements:** UP-4, UP-3 (notice), AC-5, AP-17 and AP-18 (same layout and pinned map as the activity page), UP-7 (lifetime setting), SH-2, SH-4 (toggle), SH-8 (download setting); Technical design: Publish and view flow (Publish 4, 6), Security and privacy (IDs and tokens)
- **Depends on:** F-12, F-13

## Goal
One page, opened by the edit link, that shows the workout as viewers will see it plus a panel where the owner sets everything, before and after publishing.

## Scope
- Reads the token from the URL fragment, sends it only as a header; never puts it in a link, query string or log. Page has Referrer-Policy as in F-22, and `noindex`.
- Loads draft settings and `draft.bin` through the owner API; renders all activity modules client-side with the F-13 components.
- Edit panel: title, description, start and end hiding (off by default), lifetime (up to 30 days from upload; shows the resulting expiry date in the browser's time zone), viewer download (off by default).
- No visibility control (SH-2, 30 Oct 2026): publishing is F-20's Publish button and cannot be undone. The built unlisted/private control is removed (removal of built behaviour; architect hand-off).
- Description: line breaks are kept; the server refuses a tab or any other control character (UP-4, F-12), so the panel never sends one (for example by replacing a pasted tab with a space or blocking it; the engineer's choice) and never loses the rest of the text.
- Saves through the owner API; clear saved/error state.
- Truncated upload: "The file was incomplete; data after hh:mm:ss is missing." shown to the owner (format assumed in README).
- With hiding on, the to-be-hidden start and end sections (SH-4: until the track first leaves a 300 m circle) are marked on the map; when they cover the whole route, the whole track is marked, with no extra warning (assumption, see README).
- Same page before and after publishing.
- Same layout as the activity page: two columns from 1024 px with a map, one column otherwise, and the pinned-map toggle on the phone (F-13).
- Wrong or missing token: the generic not-found page.

## Out of scope
- Publish, the published page, Share, delete, original download and SH-10 notice (F-20). Trim and pins (phase 2).

## Acceptance criteria
- [ ] Opening the edit link of a fresh upload shows the activity page from `draft.bin` with the edit panel; network log shows the token only in a request header.
- [ ] Changing the title and reloading shows the new title.
- [ ] The lifetime control offers nothing beyond 30 days from upload and shows the resulting expiry date.
- [ ] Hiding and download toggles are off on a new upload.
- [ ] A truncated corpus file shows the incomplete-file notice with the time of the last intact record; a complete file shows no notice.
- [ ] An edit link with a modified token shows the generic not-found page.
- [ ] In a window 1024 px or wider the edit page shows the F-13 two-column layout; on a phone the map toggle pins the map as on the activity page.
- [ ] Indoor file: edit page works, one column at every width, hiding toggle has no visible effect on the (absent) map and causes no error.
