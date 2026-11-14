# F-55 Selectable radius for start and end hiding (two sliders)

- **Status:** draft; reworked 14 Nov 2026 from "Privacy zones" (OQ-229 to OQ-233 superseded); OQ-246, OQ-248, OQ-249 open; OQ-247, OQ-250 superseded by the two-slider decision
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first)
- **Lane:** critical (location privacy)
- **Requirements:** SH-4 (extended), SH-10, SH-9, AP-3, UP-4, UP-5, UP-6, SH-5, SH-12, SH-13, AC-5, PIN-7; SH-3 removed
- **Depends on:** F-16 (hiding), F-19 (edit page), F-36 (preview image), F-47 (trim), F-54 (texts)

## Goal
A sharer chooses how much of the start and of the end of the route is hidden, separately, so the hidden part fits where they live, or hides only one end.

## Scope
- On the edit page, before and after publishing, two sliders replace the single start and end hiding toggle: **Start** and **End**, each 0 to 2,000 m in 50 m steps, with no value between 0 and 200 m (0, 200, 250, … 2,000 m). 0 means that end is not hidden, so only the start or only the end can be hidden.
- Default 0 for both, so nothing is hidden unless the sharer chooses it (as today's default off).
- While dragging, the value is shown ("Start: hide 500 m", "Start: not hidden" at 0), and the edit page's shading of the hidden sections follows live, before saving.
- The SH-4 rule is unchanged except for the radius: the start section runs until the route first gets more than the start radius (straight line) from the first position; the end section likewise from the last position with the end radius. After a trim it applies to the trimmed route.
- Everything that follows hiding today follows the chosen radii: public data file, static page, preview image and its map context, suggested title (first visible point), range links, lap and leg markers, GPX route download (F-57), fully hidden route text (AP-3), pins (PIN-7, F-56).
- The two radii are stored with the workout's settings for its lifetime and deleted with it (UP-7, AC-4).
- Workouts that exist before this feature with hiding on get 300 m at both ends; with hiding off, 0 at both ends.

## Out of scope
- Free values outside the slider steps. Circles around other places (SH-3 privacy zones, discarded 14 Nov 2026).

## Acceptance criteria
To be written once the open questions are decided.

## Decisions (user, 14 Nov 2026)
- Two sliders, 0 and 200 to 2,000 m in 50 m steps, default 0, value shown while dragging, shading live; replaces the toggle. Supersedes OQ-247 (control) and OQ-250 (existing workouts: 300 m at both ends when hiding was on).

## Open questions
- [ ] **OQ-246 Larger radius counts as hiding more.** Suggested: yes; raising either radius on a published workout (including from 0) is a hides-more change (SH-10, removed within 60 s); lowering one is an ordinary change.
- [ ] **OQ-248 Remembered for the next upload.** Suggested: no; every upload starts at 0 for both ends. Nothing is stored in the browser.
- [ ] **OQ-249 Edit-page preview.** Suggested, and now implied by the live shading: the shading follows the sliders at once, before saving; the public page changes only after Save.
