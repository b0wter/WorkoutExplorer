# F-19 Edit page: draft view and settings panel: history

History of [architect/F-19-edit-page-and-settings.md](../F-19-edit-page-and-settings.md). Read it only when you need the history.

## Version 0.3 (30 Oct 2026, F-20 design v0.2): text replaced

- E-1: Caddy and Vite served `edit.html` for `/edit/*` only ("the engineer chooses how" in development).
- E-2: the token came "from the fragment, up to the first `&` (ADR-020 rev. 1)"; no exception to "never in the DOM" was named there.
- E-3 read: "**First-open marker.** F-19 removes `&new` with `history.replaceState` before it renders anything; it drops every parameter after the token, not only `&new` (stricter than ADR-020 rev. 1's "unknown parameters are ignored", and harmless). The token stays in the fragment, so a reload still works (decision log, 23 Oct 2026). F-19 keeps "first open" as an in-memory flag for F-20's save notice. The only time the edit link is shown is F-20's notice, which gets the link from `lib/owner`"
- E-7 read: "*Visibility*: a draft shows "Draft", with no control (publishing is F-20). Once published, unlisted ↔ private (F-12 O-6)."
- Verification, token row, ran on "an `/edit/{id}#{token}&new` location" and checked "`&new` is gone and the token is kept"; mutant "the marker kept".
- Hand-off F-20: "using the owner client and the first-open flag from E-3".

## Version 0.2 (29 Oct 2026, F-19 QA r1, QA-19-r1-1): text replaced

- Header: "**Version:** 0.1" and "**Status:** draft (29 Oct 2026). Normal lane plus critical checks (edit token). OQ-81 and OQ-82 resolved"
- E-9: "The marks go through `RouteMap` into the lazy `map-view` as a style over the existing route source." (built as a separate `wx-hidden` source instead)
- Budget, activity entry cell: "Today the reserve is about 4,000 (entry 55,986 at F-18 QA r1)"
- Hand-offs row: | ADR-011 | Revision at F-19 QA: add F-19's 400, record F-18's measured 1,067 and the new reserve | (done in ADR-011 rev. 10)
- Added in 0.2, no text replaced: E-1 path imports, E-3 all parameters after the token dropped, E-7 "now" once at page open (QA-19-r1-2 accepted), E-10 Copy link hidden on drafts (user, QA r1 question 1)
