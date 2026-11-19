# F-55 Selectable radius for start and end hiding (two sliders)

- **Status:** decided (14 Nov 2026); acceptance criteria written 19 Nov 2026; OQ-268 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first)
- **Lane:** critical (location privacy)
- **Requirements:** SH-4 (extended), SH-10, SH-9, AP-3, UP-4, UP-5, UP-6, SH-5, SH-12, AC-5, AP-24 (edit-page radii); SH-3 removed. SH-13 (F-57) and PIN-7 (F-56) use the radii; both are built after this feature.
- **Depends on:** F-16 (hiding), F-19 (edit page), F-36 (preview image), F-47 (trim), F-58 (edit-page dialog shows the radii)

## Goal
A sharer chooses how much of the start and of the end of the route is hidden, separately, so the hidden part fits where they live, or hides only one end.

## Scope
- On the edit page, before and after publishing, two sliders replace the single start and end hiding toggle: **Start** and **End**, each 0 to 2,000 m in 50 m steps (0, 50, 100, 150, … 2,000 m). 0 means that end is not hidden, so only the start or only the end can be hidden.
- Default 0 for both, so nothing is hidden unless the sharer chooses it (as today's default off).
- While dragging, the value is shown ("Start: hide 500 m", "Start: not hidden" at 0), and the edit page's shading of the hidden sections follows live, before saving.
- The SH-4 rule is unchanged except for the radius: the start section runs until the route first gets more than the start radius (straight line) from the first position; the end section likewise from the last position with the end radius. After a trim it applies to the trimmed route.
- Everything that follows hiding today follows the chosen radii: public data file, static page, preview image and its map context, suggested title (first visible point), range links, lap and leg markers, fully hidden route text (AP-3). Built later on the same rule: GPX route download (F-57), pins (PIN-7, F-56).
- The two radii are stored with the workout's settings for its lifetime and deleted with it (UP-7, AC-4).
- Workouts that exist before this feature with hiding on get 300 m at both ends; with hiding off, 0 at both ends.
- Texts in English; F-54 (built last) translates them.

## Out of scope
- Free values outside the slider steps. Circles around other places (SH-3 privacy zones, discarded 14 Nov 2026).

## Assumptions
- A workout without positions (indoor, pool) shows the sliders; they have no visible effect and cause no error (as F-19's toggle).
- One save that raises one radius and lowers the other counts as hiding more (OQ-246: raising either).

## Acceptance criteria
- [ ] The edit page, on a draft and on a published workout, has two sliders, Start and End, in place of the toggle; each moves from 0 to 2,000 m in 50 m steps and no other value. A new upload starts at 0 for both; nothing about them is kept in browser storage (OQ-248).
- [ ] While a slider is dragged, its label shows the value ("Start: hide 500 m", "Start: not hidden" at 0; unit per OQ-268), and the shaded start or end section on the map and charts follows at once, before saving. The public page does not change until Save (OQ-249).
- [ ] The owner API refuses a radius below 0, above 2,000 m or not a multiple of 50 with the edit page's ordinary error, and stores nothing.
- [ ] Test, for start and end radii of 0, 50, 300 and 2,000 m in every combination: the hidden sections follow the SH-4 rule with each end's own radius; with start 0 and end 500 m, only the end is hidden. No position from a hidden section appears in any file under `public/{public_id}/` (data file, metadata JSON, `index.html`, preview image), extending F-16's privacy test.
- [ ] With a trim set, both sections are measured from the first and last remaining positions (F-47).
- [ ] Summary values are identical at every radius (AP-2, SH-9).
- [ ] A route that stays within the start or end radius is hidden entirely and shows "The entire route has been hidden by the uploader." (AP-3), without a warning before publishing; an indoor file shows "No location data in this workout." at any radius.
- [ ] Raising either radius on a published workout, also from 0, removes the old public files within 60 s (SH-10, OQ-246); lowering one shows the returned positions after the rebuild, as an ordinary change.
- [ ] The preview image (SH-5, SH-12: route, frame and map), the suggested title's town (UP-5: first visible point), the start, finish, lap and leg markers and a range link opened inside a hidden section (AP-19) follow the saved radii.
- [ ] No public file, no viewing-page text and no request a viewer's page makes contains either radius or hints at which end is hidden, beyond what the missing positions themselves show; the edit page's summary dialog shows both radii (F-58).
- [ ] An existing workout with hiding on shows 300 m on both sliders after the migration, one with hiding off 0 on both; their public pages are unchanged after a `regenerate`.
- [ ] Deleting the workout or its expiry leaves no radius stored (UP-7, AC-4).

## Decisions (user, 14 Nov 2026)
- Two sliders, 0 to 2,000 m in 50 m steps (the 200 m minimum was dropped the same day), default 0, value shown while dragging, shading live; replaces the toggle. Supersedes OQ-247 (control) and OQ-250 (existing workouts: 300 m at both ends when hiding was on).

- **OQ-246 (14 Nov 2026):** raising either radius on a published workout (including from 0) is a hides-more change (SH-10, removed within 60 s); lowering one is an ordinary change.
- **OQ-248 (14 Nov 2026):** not remembered; every upload starts at 0 for both ends; nothing is stored in the browser.
- **OQ-249 (14 Nov 2026):** the shading follows the sliders at once, before saving; the public page changes only after Save.

## Open questions
- [ ] **OQ-268 Radius on imperial pages.** The steps are metric. Suggested: the label always shows metres ("Start: hide 1,500 m"), also with imperial units, as split presets stay metric (AP-20). Affects the slider labels and the F-58 edit-page example.
