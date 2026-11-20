# F-62 Show my location on the map

- **Status:** planned (user idea, 20 Nov 2026, "for the future"); no build-order position yet; OQ-285 to OQ-290 open; acceptance criteria to write once they are decided
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (backend: a `Permissions-Policy` header in the Caddyfile, OQ-289)
- **Lane:** critical (the viewer's own location)
- **Requirements:** AP-25 (new); AP-3, AP-15, AP-19, SH-5, SH-12, DS-7, INF-1; NFR Privacy
- **Depends on:** F-06 (route map), F-22 (headers), F-26 (fullscreen map), F-29 (About page); F-61 if built after it (both tile hosts)

## Goal
A viewer standing on or near the route sees where they are on the map, for example to follow a route a friend shared.

## Scope
- On the viewing page, a map control "Show my location". Only an explicit tap starts it; never automatically, never remembered: every page load starts with it off.
- The browser's Geolocation API, after the browser's own permission prompt. Denied permission, no position, a timeout or an insecure context (only HTTPS, plus `localhost` in development): a short message, nothing else changes.
- The position is drawn as a dot (with the accuracy the browser reports) on the map; a second tap turns it off and removes the dot.
- The position stays in the browser: never sent to our server, never stored (no `lib/prefs`, no storage of any kind), never in the URL or a range link (AP-19), never in the preview image (SH-5, SH-12), never logged.
- About page (INF-1): one sentence that the own position, if a viewer turns it on, is shown only in their browser and never sent to the server, and that the map's tile host(s) see the map area shown (OQ-286).

## Out of scope
- Sending or sharing the position with anyone; live tracking of the athlete (non-goal); navigation, routing or "distance to route" (an idea at most); remembering the choice.

## Constraints and privacy notes
- **Tile hosts learn the area.** If the map pans or zooms to the viewer's position, tiles for that area are fetched from OpenFreeMap (and Esri when Satellite is on, DS-7), which then learn an approximate area tied to the viewer's IP address (OQ-286).
- **Permissions-Policy.** The site sends no `Permissions-Policy` today (F-22 H-5); geolocation should be limited to our own pages (OQ-289) without blocking the clipboard (Copy link).
- **Secure context only.** Geolocation works only over HTTPS; production always serves HTTPS.
- **Screenshots** of the page show the viewer's position; no technical fix (OQ-290).
- **Not live tracking:** the dot is the viewer's own position on their own screen, not the athlete's; the non-goal is unaffected.

## Open questions
- [ ] **OQ-285 Show once or follow.** Suggested: the dot keeps updating while it is on (`watchPosition`), but the map never follows on its own; updates stop when it is turned off or the page is hidden or closed.
- [ ] **OQ-286 Pan to the position.** Suggested: the tap centres the map on the position once (like panning there by hand); the About page says the tile host(s) then see that map area. Alternative, stricter: never pan, show the dot only when it lies in the current view.
- [ ] **OQ-287 Edit page too.** Suggested: no, viewing page only; reconsider if placing pins on site (F-56) needs it.
- [ ] **OQ-288 Fullscreen map.** Suggested: yes, the control is also in the fullscreen map (AP-15), the main case of following a route on a phone.
- [ ] **OQ-289 Permissions-Policy header.** Suggested: `Permissions-Policy: geolocation=(self)` on every response, no other feature listed, so the clipboard and everything else behave as today.
- [ ] **OQ-290 Screenshots.** Suggested: no in-page hint; the About sentence is enough, as the viewer turned it on knowingly.
