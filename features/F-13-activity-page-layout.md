# F-13 Activity page layout and template

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** AP-1, AP-2, AP-17 (two columns on wide screens and phones in landscape), AP-18 (map kept at the top on a phone in portrait), SH-6 (copy link on the viewing page), SH-9, UP-7 (expiry shown), INF-1 and INF-2 (footer links), DS-3, NFR Performance (LCP under 2.5 s, summary before map and charts), NFR Compatibility; Technical design: Publish and view flow (View), Front end (Load order, Budget)
- **Depends on:** F-07

## Goal
The activity page viewers see: modules in the specified order, summary numbers in the HTML itself, charts and map attached from the prototype, on phone and desktop.

## Scope
- Modules top to bottom in the one-column layout (phones in portrait): header; summary strip with privacy notice; map, or the "No location data in this workout." or "The entire route has been hidden by the uploader." hint (AP-3, F-16); charts; range selection panel (filled by F-17); laps and splits table (F-18). Footer with About and Report links.
- Header (AP-1): title, sport icon (generic icon for sports without their own), date and start time in the activity's local time zone, description as plain text, expiry: "Expires in …", computed in the browser from the UTC expiry time; a date, where one is shown, in the browser's time zone; the markup carries the expiry date in the workout's own time zone (UTC when the workout has none), marked as an estimate ("Expires around 30 Oct 2026"), which is what shows with JavaScript off.
- Summary strip (AP-2): metrics the workout has; missing metrics hidden, never zero; no per-sport list: every AP-2 metric the workout has; the sport decides only pace or speed (DS-2) and the cadence unit; device totals, the server's computed value where the device recorded none. Maximums of speed or pace, heart rate, power, cadence and temperature, wherever the summary has them, as a second value in the same cell as the average.
- Data file cannot be loaded or decoded: "The workout data could not be loaded.", plus "Reload the page." for an unsupported format version; no technical detail.
- Privacy notice (SH-9), always shown next to map and summary: "Parts of the route, such as the start and end, may be hidden for privacy. Totals include them."
- Copy-link button for the viewing link (SH-6).
- Page template: the frontend authors the HTML template and one stylesheet served from the site, with no inline CSS or script ([ADR-026](../architect/ADR-026-activity-page-template-adopted-not-hydrated.md)); the backend fills it at publish (F-15) through a documented view model. Summary strip and laps table are rendered into the markup; the HTML preloads the bundle and data file; data file URLs come from the HTML, never built.
- The same Svelte components render all modules client-side for the edit page (F-19), which has no server-rendered markup.
- No Svelte hydration: the client adopts the server-rendered summary and laps without re-rendering or shifting them, and mounts Svelte only into empty slots (ADR-026).
- Phone (portrait and landscape) and desktop layouts.
- Two columns from 1024 px when there is a map (AP-17; follow-up decided 24 Oct 2026): columns 1:1; the map alone on the left, full window height, flush with the window edges, staying in view (sticky) while the right column scrolls, for as long as the right column lasts, with its credits at the top; header, summary strip with privacy notice, charts, range selection panel, laps and splits table and footer on the right, in that order. Phones in landscape (user, 27 Oct 2026): a short landscape screen (under about 500 px tall) also uses two columns when there is a map, split about 40 % map, 60 % content, with the same rules otherwise (map sticky on the left, everything else on the right); built with F-17. One column below 1024 px otherwise (phones in portrait), and at every width for a workout with no location data or no visible route (hint or no map). The layout works with JavaScript off and adds no layout shift.
- Pinned map (AP-18), one-column layout (phones in portrait) only: a toggle button on the map, off by default; while on, the map stays at the top of the viewport until the end of the laps and splits table, at 35 % of the screen height, never more than 40 %, and not capped otherwise: on a tablet it may be larger than the unpinned map; and cursor, range highlight and zoom keep working; tapping again puts it back. Not stored (no cookie or browser storage). No toggle in two columns or without a map. Leaving the fullscreen map (F-26) keeps the pinned state. The button reads "Pin map" / "Unpin map". On the phone the map credits are shown for 5 s or until the first touch on the map, then collapse to the "ⓘ" button, and the cursor readout is a card.
- The edit page (F-19) uses the same layout and toggle through the same components.
- Page texts the spec does not word (metric labels, copy link and its copied state, laps headers, footer, the expiry text): proposed by the engineer, reviewed by the user once visible (decision log, 24 Oct 2026).

## Out of scope
- Units and theme (F-14), range selection (F-17), laps interaction and splits (F-18), report form (F-28), download button (F-24), edit panel (F-19).

## Acceptance criteria
- [ ] A rendered page for a ride is at most 15 KB compressed and shows title, sport, local start date/time, description, the expiry as an estimated date in the workout's own time zone (e.g. "Expires around 30 Oct 2026"; in UTC for a workout without a time zone), summary and privacy notice with JavaScript disabled. With JavaScript on, the expiry reads "Expires in …", computed in the browser from the UTC expiry time, and is the same for viewers in UTC−5 and UTC+9.
- [ ] A ride recorded in UTC+2 shows its local start time (e.g. 07:30) to a viewer whose browser is set to UTC-5.
- [ ] A running file without power shows no power metric; a file without heart rate shows no heart-rate value or "0 bpm".
- [ ] The privacy notice text is identical on a workout with hiding on and one with hiding off.
- [ ] A description containing HTML or `<script>` appears as literal text.
- [ ] Copy-link copies the viewing URL; the copied text never contains an edit token.
- [ ] Lighthouse/trace on both presets (mobile: throttled mid-range phone; desktop): LCP under 2.5 s; summary numbers paint before charts and map; no layout shift when the bundle hydrates. The movement caused by a tap that pins or unpins the map is not a layout shift.
- [ ] Indoor workout: hint instead of map, one column at every width; charts and summary render normally.
- [ ] A data file that fails to load shows "The workout data could not be loaded."; one with an unsupported format version also shows "Reload the page."; neither shows technical detail.
- [ ] Footer shows the About link and the Report link.
- [ ] In a browser window 1024 px or wider, a workout with a route shows two columns of equal width: the map alone on the left, full window height and flush with the window edges, with its credits at the top; the header, the summary with privacy notice, the charts, the range panel, the laps and the footer on the right, in that order; the map stays in view while scrolling to the laps. Below 1024 px on a phone in portrait the page is one column in the order header, summary, privacy notice, map, charts, range panel, laps. An indoor workout, and one with no visible route, is one column at every width. All of this holds with JavaScript disabled.
- [ ] On a phone in landscape (under about 500 px tall), a workout with a route shows two columns, about 40 % map and 60 % content, with the same order and sticky map as from 1024 px; with JavaScript disabled too.
- [ ] On a phone in portrait, the toggle on the map ("Pin map", then "Unpin map") pins the map at the top, at 35 % of the screen height and never more than 40 %, while scrolling down to the end of the laps; scrubbing a chart below it moves the marker on it; tapping the toggle again puts it back. After a reload the toggle is off, and no cookie or browser storage entry was written. There is no toggle in either two-column layout (from 1024 px, or a phone in landscape), on an indoor workout or on one with no visible route.
