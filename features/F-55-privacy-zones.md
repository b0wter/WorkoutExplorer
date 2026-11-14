# F-55 Privacy zones

- **Status:** draft
- **Category:** Planned (backlog scheme of 14 Nov 2026; requirement priority and phase kept from the spec)
- **Owner:** both (backend first)
- **Lane:** critical (location privacy)
- **Requirements:** SH-3, SH-4, SH-9, SH-10, UP-6, UP-7, AC-4, AC-5, AP-3, SH-5, SH-12, UP-5, NFR Privacy
- **Depends on:** F-16 (hiding), F-19 (edit page), F-47 (trim), F-36 (preview image)

## Goal
A sharer hides chosen places along the route (home, work, a friend's house), not only the start and the end.

## Scope
- On the edit page, before and after publishing, the owner adds circles of 200 m to 1 km radius by tapping the map, sets each one's size, moves and removes them (OQ-229).
- Every position inside a zone is removed from all public data: data file, static page, preview image and its map context, suggested title, lap and leg markers, range link results. Hidden on the server, not just in the browser (SH-3).
- Order: trim, then start and end hiding, then zones. A route fully hidden by zones gets the AP-3 "entire route has been hidden" text.
- Totals stay as the device recorded them (as SH-4); ranges, laps and splits behave as they do with start and end hiding.
- The zones themselves are never public: no circle, centre or radius on the viewing page or in public files. The owner sees them on the edit page only.
- Zones are stored with the workout, for its lifetime, and deleted with it (UP-7, AC-4). Adding or growing a zone hides more and is removed within 60 s (SH-10).
- The privacy notice (SH-9) wording stays unless OQ-232 says otherwise.

## Out of scope
- Zones saved across workouts (needs storage beyond one workout; OQ-231). Pins (F-56).

## Acceptance criteria
After the open questions are decided.

## Open questions
- [ ] **OQ-229 How many zones.** Suggested: up to 5 per workout.
- [ ] **OQ-230 The gap reveals the place.** A route that enters and leaves a circle shows a gap whose middle points at the centre. Suggested: accept, as for SH-4's 300 m; optionally shift the hidden circle by a random offset of up to a third of its radius, chosen once per zone (the centre then is not at the gap's middle). Decide: offset yes or no.
- [ ] **OQ-231 Reuse zones on the next upload.** Suggested: not now; a later idea could keep them in this browser only (like F-53), never on the server.
- [ ] **OQ-232 Notice wording.** Suggested: the SH-9 notice stays as it is; it already says parts may be hidden.
- [ ] **OQ-233 Charts inside a zone.** Suggested: only positions are removed; heart rate, speed and the other charts keep their samples, as with start and end hiding today (decide if F-16 does otherwise).
