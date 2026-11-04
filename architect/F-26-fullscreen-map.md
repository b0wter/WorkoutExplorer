# F-26 Fullscreen map: architecture

- **Version:** 0.1
- **Status:** accepted (3 Nov 2026). Normal lane. OQ-100 open, not blocking
- **Date:** 3 Nov 2026
- **Feature:** [`features/F-26-fullscreen-map.md`](../features/F-26-fullscreen-map.md)
- **Requirements:** AP-15, AP-18, NFR Accessibility (Esc), NFR Compatibility, NFR Performance (60 fps); decision log 3 Nov 2026 (OQ-99: colour control stays)
- **Builds on:** [F-06](F-06-route-map-linked-to-cursor.md) D-11, [F-13](F-13-activity-page-layout.md) D-18 to D-20 and its F-26 hand-off, [F-17](F-17-range-selection-and-axis-toggle.md) R-10, [F-25](F-25-route-colouring.md) C-6, C-7; ADR-011, ADR-013, ADR-015, ADR-016, ADR-032, ADR-037; new [ADR-038](ADR-038-fullscreen-map-as-css-overlay-with-a-history-entry.md)
- **Owner:** frontend only. No backend step, no template or view-model change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| FS-1 | **CSS overlay on every platform, no Fullscreen API** (ADR-038). The map container that `RouteMap` hands to `map-view` (`div.route-map`) becomes the fullscreen element, through MapLibre's own `FullscreenControl` with `pseudo: true` and that container. The control owns the state, the enter/exit button, the resize and the cooperative-gesture switch; our code listens to its `fullscreenstart` / `fullscreenend` events and drives Esc and back through its button, so there is one owner of the state and no private MapLibre member is used | iPhone Safari needs the CSS path anyway; one path means one set of exits. The control ships with MapLibre |
| FS-2 | **Everything stays in the lazy map chunk.** The control, the exit paths and the CSS (hide the page, lock scrolling) live in `components/map` beside `map-view` (ADR-011 item 2). They act only on the map's container and the document root, and name no page module, so the viewing page, the edit page (owner UI included) and `bench/map.html` behave alike. No change to `RouteMap`, `src/page`, `src/edit`, `page.css` or the template | 0 bytes before the map; one code path for both pages |
| FS-3 | **What is visible.** Inside the map container only: tiles, route and bridges, colouring (`wx-colour-line`), range highlight (`wx-range-*`, F-17 R-10), to-be-hidden marks (edit page), start, finish and lap markers, the cursor marker, zoom control, attribution (ADR-014: the credits must stay), the F-25 colour control with its legend (OQ-99, decided), and the fullscreen button, which shows MapLibre's exit icon while fullscreen. Everything outside the container is hidden: header, summary, notice, charts, **cursor readout** (AP-15: every data field; F-33 adds a second readout inside the container, AP-21), range panel, laps, footer, owner UI, and the pin toggle and splitter (siblings of the container in `#map-slot`). Controls keep their corners and the existing two-column corner moves in `map-view.css`; the fullscreen button sits in its own group under the zoom buttons. Wording and label are MapLibre's English defaults ("Enter fullscreen" / "Exit fullscreen"); the user reviews them on screen (OQ-63 rule) | AP-15 list plus OQ-99. Pins and their legend: phase 2 |
| FS-4 | **Hidden, not removed.** The rest of the document is hidden by inherited `visibility` and document scrolling is locked; nothing changes `display`, size or class on the page. So the scroll offset, the pinned state (AP-18), the splitter width and the sticky or fixed map slot are untouched by construction. As a guard, the scroll offset taken on entry is re-applied on exit if the browser moved it. Hidden content leaves the tab order. Constraint: no ancestor of the map container may get `transform`, `filter`, `contain`, `will-change` or `backdrop-filter`, which would trap the fixed box | AC 3, AP-18; no chart resize or remount |
| FS-5 | **One map, camera by F-13 D-20.** Fullscreen never remounts `RouteMap` or recreates MapLibre (D-20 "one map per data value"). Entering, leaving and rotating resize the container; MapLibre resizes, and D-20's refit-until-moved applies unchanged: unmoved, the route is fitted to the new box; moved (in the page or in fullscreen), centre and zoom are kept. No camera rule of our own | AC 4 by MapLibre's resize; no new state |
| FS-6 | **Gestures in fullscreen.** Cooperative gestures off (the control does it, and restores them on exit): one finger pans, two pinch, a plain wheel zooms on desktop, since there is no page to scroll. ADR-015 items 2 and 3 stand: a one-finger touch that starts on the route scrubs, and the capture-phase lock keeps its moves from MapLibre's touch pan (`map-view.ts:10-20`). Double-tap zoom **stays off** (ADR-015 rev. 1 reason: a short scrub looks like a tap); `touchZoomRotate` is never re-enabled | No page scroll to protect; scrub rule unchanged |
| FS-7 | **Exit paths.** Button, Esc (one `keydown` listener on the document, present only while fullscreen; the only shortcut), back gesture (`popstate` on the one entry pushed on entry, ADR-038 item 3). All three end in the same exit; the history entry is removed exactly once; back never leaves the page while fullscreen. Destroying the map while fullscreen (new data value, page teardown) exits first: root class, scroll lock, listeners and history entry all go | AC 3 |
| FS-8 | **Theme and colouring carried over by identity.** Same map instance, so the scheme (F-14 D-12 `setScheme`), the colouring choice and legend (F-25 C-7), the units of the legend and the range need no hand-over. A scheme or units change while fullscreen (system theme, another tab) applies as on the page | Nothing to copy |
| FS-9 | **Availability.** The button exists only once the map exists (inside the mounted map), so there is none on an indoor workout, on a fully hidden route (F-16), before the map loads, or after a failed map (AC 5). Offered on the viewing and edit pages and at every width, desktop included (AP-15 names Esc) | By construction, no new presence check |

## Scrubbing (60 fps) and cost

| What | Decision |
| --- | --- |
| Scrub path | Unchanged: a route scrub sets the cursor store; the map moves its DOM marker (F-06 D-11, no `setData`, no repaint). The hidden charts and readout still receive the cursor, as on the page, but are not painted. If the fullscreen scrub record fails, first stop the hidden chart subscribers from drawing while fullscreen |
| Gate | ADR-016 gate 0, route part, re-run in fullscreen on the reference phone: 3 route touch scrubs on `ride-12h`, portrait, colouring by speed on (largest canvas, worst layer set), passing and judged smooth. Recorded in `bench/MEASUREMENTS.md` |
| Static (before the map) | **0 bytes**: no static code or CSS changes (FS-2). The CI entry line must not move |
| Lazy | F-26's own lazy delta at most **1,500 bytes gzip** (map chunk plus its CSS, including `FullscreenControl` if the bundler had dropped it until now), reported beside F-06's and F-25's lazy figures. Above: stop and report. F-25's spent budget is not touched |

## Verification

| AC | Automated (`make ci`, jsdom with the recording fake) | Manual (operator) |
| --- | --- | --- |
| 1, 2 | The control is added with `pseudo: true` on the map's container; on enter the root class is set and removed on exit | iPhone Safari and Android Chrome, portrait: no data field visible; route, markers, zoom, attribution, colour control, exit button visible |
| 3 | Each of button, Esc, `popstate` exits; one entry pushed per entry, removed once; destroy while fullscreen cleans up; no `pushState` URL argument | Each exit on both phones and on desktop, from a scrolled position, pinned and unpinned; back after exit leaves the page normally |
| 4 | — | Rotate in fullscreen on both phones; route stays fitted until moved |
| 5 | Existing presence tests: no map, no control | Indoor sample, fully hidden sample |
| FS-6 | — | Route scrub in fullscreen does not pan; one finger elsewhere pans; no double-tap zoom |

## Hand-offs

| To | What |
| --- | --- |
| AP-19 (phase 2) | Write the range fragment with `replaceState` only (ADR-038) |
| Phase 2 pins (PIN-6) | Pins and their legend go inside the map container to stay visible in fullscreen |

## Open questions

| ID | Question | Suggestion |
| --- | --- | --- |
| OQ-100 | With the CSS overlay, Android Chrome and desktop browsers keep their address and system bars in fullscreen; the map fills the browser viewport, not the physical screen (iPhone cannot do more anyway). Accept? | Accept. If not, ADR-038's "would choose differently" adds the browser API as an extra layer without changing the layout. Blocks nothing |
