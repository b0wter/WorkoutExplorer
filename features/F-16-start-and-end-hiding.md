# F-16 Start and end hiding

- **Phase:** 1 MVP
- **Owner:** both (backend first)
- **Requirements:** SH-4 (Should), AP-2 (totals unchanged), AP-3 (fully hidden route), AP-17 (one column), SH-9 (its exception), SH-10 (hiding more after publish); Technical design: Security and privacy (Hidden sections)
- **Depends on:** F-15

## Goal
When the sharer turns it on, the start and end of the route (until it first leaves a 300 m circle around its first and last position) lose their positions on the server before any public file is written.

## Scope
- `hide_ends` off by default.
- At publish and on every regeneration: every sample in the start and end sections loses its position; other streams are kept so charts stay continuous.
- Start section: from the first positioned sample until the track first gets more than 300 m (straight line) from it; end section: the same backwards from the last positioned sample (SH-4, decided 26 Oct 2026). GPS jitter while standing near the start therefore does not shorten the hidden section.
- Everything public is derived after the cut: data file, bounding box, lap and start/finish marker positions, preview image (F-23), export file (F-24). Metadata in the public data file contains no coordinate from a hidden section.
- Summary keeps the device totals.
- Turning hiding on after publishing counts as "hides more of the route": files regenerated, old files deleted (SH-10). Turning it off restores the positions from the original.
- A route of 600 m or less, or one that stays within 300 m of its start or end, becomes entirely hidden; the sharer gets no warning. The public page then shows "The entire route has been hidden by the uploader." in place of the map, one column at every width (AP-3, AP-17). A workout without any positions shows the no-location hint, with hiding on or off.

## Out of scope
- Privacy zones (SH-3, phase 2), trim (UP-6, phase 2). Showing the hidden sections in the edit page (F-19).

## Acceptance criteria
- [ ] Test: with hiding on, no file under `public/{public_id}/` contains hidden data: no position from the start or end section appears anywhere, data file columns, metadata JSON, `index.html`. (Preview image and export file are checked again in F-23 and F-24.)
- [ ] With hiding on, heart rate, speed and altitude in the hidden sections are still present in the public data file.
- [ ] Summary distance and time are identical with hiding on and off.
- [ ] With hiding off (default), the public route equals the full track.
- [ ] A 500 m walk with hiding on publishes with no positions; the page shows "The entire route has been hidden by the uploader." in place of the map, one column at every width, and does not break.
- [ ] A 5 km run of laps on a 400 m track whose start lies within 300 m of every point publishes with no positions and shows the same hint.
- [ ] A recording that starts with two minutes of GPS jitter within 50 m of the start hides every position until the track is first more than 300 m (straight line) from the first position.
- [ ] An indoor file with hiding on shows "No location data in this workout.", not the fully hidden hint.
- [ ] A ride whose GPS fix starts 1 km after the start of recording measures the start section from the first positioned sample.
- [ ] Turning hiding on after publish: within the regeneration, the old data file is deleted from disk.
