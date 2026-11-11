# F-45 Suggested title

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** critical (uses the route's coordinates and must not reveal a hidden start or end, SH-4; architect design, QA at effort high)
- **Requirements:** UP-5 (Could, phase 2); UP-4, AP-1, SH-4, SH-9, SH-10, UP-6 (phase 2), UP-7, AC-4, INF-1, DS-3; NFR Privacy (store as little as possible, no external services), NFR Performance (budgets unchanged); decision-log items 17 and 26 in [README](README.md); technical design (place names, suggested title)
- **Depends on:** F-16, F-19, F-29, F-42

## Goal
A sharer who leaves the title empty gets a fitting one, such as "Morning ride near Riverton", without typing.

## What exists
- An empty title shows sport and date ("Ride, 9 Oct 2026", UP-4, item 17). Title limits and plain-text rules: F-12.
- The data file carries sport, sessions, start time and the workout's UTC offset (technical design, time zones).
- About lists attributions from a committed sources file; GeoNames is to be added only when UP-5 ships (F-29, item 26).

## Scope
- **Rules as in UP-5** (decided 11 Nov 2026, OQ-154 to OQ-161): empty-title fallback and placeholder, never written into the field; rebuilt with the page; wording, time-of-day boundaries, sport word, 25 km limit and town spelling there. Shown wherever the title appears (header, browser tab, preview card, SH-5).
- **Place lookup.** On the server, offline from GeoNames `cities15000` bundled with the binary; no coordinates or town names leave the server, no new outbound call (technical design).
- **Which point.** The first position that remains public after start and end hiding (SH-4) and, once built, trim (UP-6). Never a position from a hidden or trimmed part, so the town never points closer to a hidden start than the public route does.
- **Without the place data on the server:** no suggestion; the empty title shows sport and date as before (technical design fallback).
- **Storage.** Nothing new is stored beyond the title the sharer keeps; the suggestion is derived, deleted with the workout (UP-7, AC-4).
- **Attribution.** GeoNames (CC BY 4.0) added to the About page sources (INF-1, F-29) in the same change.

## Out of scope
- Other languages for the English words; town names other than the GeoNames `name` field.
- Suggested descriptions; landmarks, streets or regions finer than a town.
- Changing device-recorded data; storing the suggestion.

## Acceptance criteria
- [ ] A ride starting at 07:30 local time 5 km from Köln, title empty, shows "Morning ride near Köln" (accents kept) in the header, the browser tab and the preview card, and as the title field's placeholder; the field itself stays empty.
- [ ] Boundaries: starts at 04:59, 05:00, 11:59, 12:00, 16:59, 17:00, 20:59 and 21:00 give night, morning, morning, afternoon, afternoon, evening, evening, night.
- [ ] The time of day uses the workout's own UTC offset, not the server's or the viewer's time zone; without an offset, UTC; without a start time, no time-of-day word ("Ride near Köln").
- [ ] With start and end hiding on, the town is computed from the first public position; a test route whose hidden start lies next to town A and whose first public point lies nearer town B gets town B.
- [ ] Turning hiding on or off (and, once built, trimming) on a published workout updates the shown title within the 60 s of SH-10; no page, card or placeholder keeps a town from a now-hidden position.
- [ ] An indoor workout, a pool swim, an entirely hidden route and a route whose first public point is more than 25 km from any town in `cities15000` get no "near …" ("Evening run") and no error; 24.9 km still gets it.
- [ ] Swim, bike, run with transitions shows "triathlon"; any other multisport workout (F-42), e.g. run, bike, run, shows "multisport".
- [ ] A network log of upload, edit and publish shows no request to any third party; the server makes no outbound connection for the lookup.
- [ ] A typed title always wins, keeps the F-12 limits and is never overwritten; clearing it brings the suggestion back.
- [ ] The About page lists GeoNames with its CC BY 4.0 licence.
- [ ] Every corpus file still uploads and renders; upload time, data file size and LCP unchanged within noise; the binary grows by no more than the size of the bundled place data.
