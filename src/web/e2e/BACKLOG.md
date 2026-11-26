# UI test backlog

One row per piece: one feature or one small group, doable by one engineer in well under an hour and committable on its own.
Most-used features first; work top to bottom. Set Status to `done` (with the file name) when the piece is merged. How to write a
test: [README.md](README.md). Data: synthetic only; shared fixtures are in `fixtures.mjs`, new shapes go into
`src/server/tools/corpusgen/e2e.go`.

Already covered (not listed below): F-73, F-74, F-71, F-72 (swim yards, except the open items in B-18), F-67 leg switch and
aligned charts, the dialog shell, the header and readout layout sweep, the upload page (reserved drop zone, paused, fail-open,
refusal, one upload).

| # | Status | Features | What to check | Data needed |
| --- | --- | --- | --- | --- |
| B-01 | done (`viewing.pw.mjs`; dropout check: chart lines run on, the map segment moved to B-02) | F-05, F-17, F-04 | Viewing page: cursor readout follows the mouse, range drag shows the range panel with distance/time/gain, Clear and a click outside clear it, Time/Distance axis toggle keeps the range, `.axis-toggle`; gaps (gps-dropout) draw as gaps; no console error | `ride-30min`, `gps-dropout` (both in fixtures) |
| B-02 | done (`map.pw.mjs`) | F-06, F-26, F-33, AP-3 | Map: route layer present (stub tiles with `page.route`), cursor marker moves with the chart cursor (Chromium and WebKit only: WebGL), fullscreen button, Esc leaves, readout in fullscreen; `gps-dropout` route bridges the dropout with a straight segment (AP-3); `indoor-no-gps` shows the "No location data" hint and never requests the map chunk | `ride-30min`, `indoor-no-gps`, `gps-dropout` |
| B-03 | done (`laps.pw.mjs`; added `ride-lap-triggers` to FIXTURES) | F-18, F-46, F-69, F-48, F-64 | Laps and splits table: Laps/Splits switch, split presets (400 m, 1 km, time presets), lap triggers column, stroke column on pool swims, per-leg tables on the triathlon, clicking a row selects its range | `run-30min`, `pool-swim`, `triathlon`, `interval-run` |
| B-04 | done (`edit.pw.mjs`) | F-19, F-45, F-21 | Edit page of a draft: title and description fields save (reload shows them), suggested title, visibility, lifetime select, expiry date shown on the view page; token only in the URL fragment (no request carries it in a query string) | fresh `upload()` per test |
| B-05 | done (`publish.pw.mjs`) | F-20, F-12 | Publish and Share: Publish button, viewing link dialog, Copy link, the viewing page opens; edit link still opens the edit view after publishing; Delete removes the workout (viewing link then shows the generic gone page) | fresh `upload()` per test |
| B-06 | done (`hiding.pw.mjs`) | F-16, F-47, F-55, SH-8 | Start/end hiding radius and trim on the edit page: set, save, the published page has no route inside the hidden part and totals updated; fully hidden route shows no route | fresh `upload()` of `ride-30min`, `run-30min` |
| B-07 | open | F-56, F-65, F-66 | Pins (add, move, delete, pin dialog), owner-added laps (add boundary, remove, table and comparison chart update), remove auto-laps | fresh `upload()` of `ride-30min` (map: Chromium/WebKit only) |
| B-08 | open | F-14, F-54, F-41 | Units and language and theme: Metric/Imperial switch changes values live without a data request (count requests), pace for runs vs speed for rides vs /100 m swims, language en/de/es/fr switch applies at once, theme System/Light/Dark, nothing stored server side; summary tint cards in both themes | `run-30min`, `ride-30min`, `pool-swim` |
| B-09 | open | F-53 | My workouts: after an upload the list on `/my-workouts` shows it, remove from the list, empty state, entries survive a reload (browser storage only) | fresh `upload()` through the UI |
| B-10 | open | F-29, F-38, F-28, F-37 | About page (attributions list present), Impressum page, generic expired/404 page, Report form on a workout page (opens, validation, submit answers), report-delete link page | any published fixture |
| B-11 | open | F-39, F-58, F-40 | How-to bar (first-visit hint shows, dismiss with x, stays dismissed), "How these numbers are computed" dialog contents, shareable range: copy range link then opening it selects the same range | `ride-30min` |
| B-12 | open | F-42, F-44, F-43 | Multisport: sections and leg picker labels, pool summary (length, lengths), device and sensor info block | `triathlon`, `pool-swim`, `ride-30min` |
| B-13 | open | F-49, F-50, F-51 | Heart-rate and power zones panel, power curve and best efforts (SVG present, values formatted) | `ride-30min` (zones: `zones-wahoo-ride`, `zones-apple-run-6` from the corpus) |
| B-14 | open | F-25, F-61, F-62 | Route colouring by metric (select changes the legend), satellite layer toggle (stub tiles), show-my-location opt-in (permission stub) | `ride-30min` (Chromium/WebKit only) |
| B-15 | open | F-57, F-35, F-63 | GPX route download (a file with the route comes), zip upload (zip holding one FIT), iOS Shortcut hint on the upload page (only when configured) | `ride-30min` zipped in test setup |
| B-16 | open | F-68 | Extra sensor streams: running dynamics and Stryd power charts appear only when present, labels fit at 360 px in de/es/fr | `run-dynamics`, `stryd-power` (corpus) |
| B-17 | open | F-22, F-23, F-36 | Response smoke: published page and static pages carry CSP, `Referrer-Policy: no-referrer`, `X-Robots-Tag`; Open Graph tags in the head; no request leaves the site except tiles | any published fixture |
| B-18 | open | F-72 (open items) | Yards: open-water swim with a map, pins and added laps in yd, fullscreen strip; imperial default from browser language (`locale: 'en-US'` vs `de-DE`) | `swim-bike-distance-fall` added to `FIXTURES` |
| B-19 | open | F-05, F-17, F-67 | Touch projects: helper that scrubs with real touch moves (CDP `Input.dispatchTouchEvent` on Chromium; WebKit has none: use `page.touchscreen.tap` plus long-press where the app allows) and a range selection on touch; then enable the readout and range checks now skipped on phones (F-74 readout, F-72 range, legs) | `ride-30min`, `triathlon` |
| B-20 | done | suite | Stub every non-local request in the shared `open()`/page setup (as `map.pw.mjs` does): tests must not reach the tile host; then drop the openfreemap error filter in `edit.pw.mjs` | none |
| B-21 | open | suite | Flakes under load: `f76-chart-size.pw.mjs:44` on WebKit (reload aborts lazy chunks, "Unable to preload CSS": wait for networkidle before reload as in `edit.pw.mjs`), `f76-chart-size.pw.mjs:84`, `dialogs.pw.mjs:6`, `laps.pw.mjs:145` on iPad | none |

Known limits of the suite: headless Firefox has no WebGL (the map never starts there), WebKit has no layout-shift observer (the F-73
shift check runs in Chromium only), the phone projects are emulation, not devices.
