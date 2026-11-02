# F-17 Range selection and x-axis toggle

- **Phase:** 1 MVP
- **Owner:** both (backend first)
- **Requirements:** AP-5, AP-7, AP-6 (cursor keeps working), AP-17 (splitter, follow-up 28 Oct 2026), decision log 29 Oct 2026 (axis and split remembered), NFR Performance (60 fps, data file size); Technical design: Workout data file (stream table, size), Front end (Implementation notes: range stats)
- **Depends on:** F-13
- **Design:** [`architect/F-17-range-selection-and-axis-toggle.md`](../architect/F-17-range-selection-and-axis-toggle.md), ADR-029

## Goal
Viewers select any section of the workout and see its stats; the x axis switches between time and distance.

## Scope
- X-axis toggle time ↔ distance across all charts; hidden when the workout has no distance data. The choice is remembered in `localStorage` (no cookie, never sent to the server) for every workout and edit page; a workout without distance shows time and does not overwrite it; broken storage falls back silently to time.
- Splitter between map and content in both two-column layouts (AP-17): 30–70 % of the page width, on desktop also limited by the column minimums; double click or double tap restores the default and clears the stored value. The dragged split is remembered in `localStorage`, separately for desktop and for phones in landscape.
- Time axis: elapsed time, pauses drawn as breaks of their real width (provisional; review once visible).
- Distance from the server never decreases but is not strictly increasing (F-08 R-14): consecutive samples can share a value (standstills, held device dropouts) and samples can have no distance. On the distance axis a stretch that does not advance has no width, and a sample without distance is drawn as a break. Range distance is never negative; a range inside such a stretch has distance 0. The browser does not repair distance.
- Range selection by dragging across a chart (desktop: click-drag; touch: long-press, then drag). Taps and horizontal scrubs keep moving the cursor (AP-6); a vertical swipe still scrolls the page. Selected range shaded on every chart and highlighted on the map (only where positions exist).
- Selection panel (provisional; review once visible): hidden on every device until a range is selected; until then a one-line hint under the charts says how to select (drag; on touch, long-press then drag). In the one-column layout it sticks to the bottom of the screen while a range is selected (user, 26 Oct 2026, OQ-72, OQ-74). In the two-column layout of a phone in landscape it sticks to the bottom of the right column; from 1024 px it sits in the page above the laps and splits table, which moves down when it appears (accepted; user, 27 Oct 2026). Values: distance, elapsed and moving time, elevation gain, average speed or pace (distance ÷ moving time), average heart rate, power, cadence and temperature, each only when present. Averages leave out missing samples; pauses add nothing; power counts zeros, cadence does not.
- A whole-workout selection may differ from the device totals in the summary; no note is shown.
- Clear with one tap: the panel's clear control, or a tap on a chart outside the shaded part. Taps on the map or elsewhere, and scrolls, never clear (user, 26 Oct 2026, OQ-73). Clearing hides the panel.
- Range stats in constant time per range, always at full resolution: distance, moving time and gain as differences of cumulative streams; averages from prefix sums built once after load. Moving time and gain follow the server's rules, so a whole-workout range shows the summary's moving time.
- Backend first (B-1): the server writes cumulative moving time and gain streams in data file format version 2 (ADR-029); the writer, decoder and goldens change together. Hiding leaves them unchanged.
- Phone-in-landscape two-column layout (AP-17, F-13 follow-up, user 27 Oct 2026): built here, since the panel placement depends on it; rules and acceptance criterion in F-13.
- A shared selection store (alongside the cursor store) so the laps table (F-18) can set a range.

## Out of scope
- Laps and splits table (F-18). Units (F-14) are applied via the shared formatters.

## Acceptance criteria
- [ ] An indoor trainer file without distance shows no axis toggle; a ride shows it, and switching redraws all charts with the cursor kept on the same sample.
- [ ] Dragging across the main climb of a corpus ride shades it on every chart, highlights it on the map, and shows distance, duration, gain and averages that match a reference calculation from the same samples.
- [ ] Selecting a range that overlaps a hidden start section highlights only the visible part on the map; the stats still use all samples.
- [ ] One tap on the clear control removes shading and highlight and hides the panel; so does a tap on a chart outside the shaded part. A tap on the map or elsewhere on the page, a tap inside the shaded part, and a scroll that starts on a chart leave the range unchanged.
- [ ] Before any range is selected, no panel is shown on any device and a one-line hint under the charts says how to select (on a touch screen: long-press, then drag).
- [ ] On a phone in portrait (one-column layout), the panel sticks to the bottom of the screen while a range is selected, also while scrolling, and disappears when the range is cleared.
- [ ] On a phone in landscape (two columns), the panel sticks to the bottom of the right column while a range is selected, also while scrolling, and the map stays in view on the left.
- [ ] In a window 1024 px or wider, the panel appears in the page between the charts and the laps when a range is selected and is not sticky; the laps move down.
- [ ] For a whole-workout range on a corpus ride, moving time equals the summary's moving time; data files are format version 2 and the 4-hour ride stays within the 500 KB budget.
- [ ] Selecting and changing a range on the 12-hour file updates the panel without visible lag (no full-array loops per update, verified in code review or profile).
- [ ] Scrubbing still works while a range is selected.
- [ ] After switching a ride to distance, a reload and another ride's page both open on distance; the edit page too. Opening an indoor file without distance in between shows time and no toggle, and the next ride still opens on distance.
- [ ] With browser storage unavailable or throwing, or holding an unknown value, the page opens on time and the default split with no message or console error; the toggle and the splitter still work for that page.
- [ ] In both two-column layouts, dragging the splitter resizes the map column between 30 % and 70 % (on desktop, never below either column minimum); the map keeps working. A reload and another workout's page keep the split; a double click (or double tap) restores the default and the stored split is gone from `localStorage`. A split set on desktop does not change the phone-in-landscape split, and the reverse.
- [ ] After using the toggle and the splitter, no cookie exists and no request sent by the page contains either choice.
