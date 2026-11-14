# F-55 Selectable radius for start and end hiding

- **Status:** draft; reworked 14 Nov 2026 from "Privacy zones" (OQ-229 to OQ-233 superseded); OQ-246 to OQ-250 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first)
- **Lane:** critical (location privacy)
- **Requirements:** SH-4 (extended), SH-10, SH-9, AP-3, UP-4, UP-5, UP-6, SH-5, SH-12, SH-13, AC-5, PIN-7; SH-3 removed
- **Depends on:** F-16 (hiding), F-19 (edit page), F-36 (preview image), F-47 (trim), F-54 (texts)

## Goal
A sharer whose home is further than 300 m from where the route starts or ends, or who wants less hidden, chooses how large the hidden start and end are.

## Scope
- On the edit page, before and after publishing, a radius choice next to the start and end hiding toggle: 200 m, 300 m (default, today's value), 500 m, 1 km. One value for both ends. Shown only while hiding is on (OQ-247).
- The SH-4 rule is unchanged except for the radius: the start section runs until the route first gets more than the radius (straight line) from the first position; the end section likewise from the last position. After a trim it applies to the trimmed route.
- Everything that follows hiding today follows the chosen radius: public data file, static page, preview image and its map context, suggested title (first visible point), range links, lap and leg markers, GPX route download (F-57), fully hidden route text (AP-3), pins (PIN-7, F-56).
- A larger radius on a published workout hides more: the old content stops being served within 60 s (SH-10, OQ-246).
- The radius is stored with the workout's settings for its lifetime and deleted with it (UP-7, AC-4).
- Workouts that exist before this feature keep 300 m (OQ-250).

## Out of scope
- Different radii for start and end. Free values. Circles around other places (SH-3 privacy zones, discarded 14 Nov 2026).

## Acceptance criteria
To be written once the open questions are decided.

## Open questions
- [ ] **OQ-246 Larger radius counts as hiding more.** Suggested: yes; a larger radius on a published workout is a hides-more change (SH-10, removed within 60 s); a smaller one is an ordinary change.
- [ ] **OQ-247 Control.** Suggested: a drop-down "Hide within" next to the hiding toggle, visible only while hiding is on; the value is kept when hiding is turned off and on again.
- [ ] **OQ-248 Remembered for the next upload.** Suggested: no; every upload starts at 300 m. Nothing is stored in the browser.
- [ ] **OQ-249 Edit-page preview.** Suggested: the edit page's shading of the hidden sections follows the new radius at once, before saving.
- [ ] **OQ-250 Existing workouts.** Suggested: they keep 300 m; no rebuild is needed for them.
