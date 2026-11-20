# F-62 Show my location on the map

- **Status:** planned (user idea, 20 Nov 2026, "for the future"); no build-order position yet; OQ-285 to OQ-290 decided 20 Nov 2026; acceptance criteria to write
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (backend: the `Permissions-Policy: geolocation=(self)` header in the Caddyfile)
- **Lane:** critical (the viewer's own location)
- **Requirements:** AP-25 (new); AP-3, AP-15, AP-19, SH-5, SH-12, DS-7, INF-1; NFR Privacy
- **Depends on:** F-06 (route map), F-22 (headers), F-26 (fullscreen map), F-29 (About page); F-61 if built after it (both tile hosts)

## Goal
A viewer standing on or near the route sees where they are on the map, for example to follow a route a friend shared.

## Scope
- On the viewing page and its fullscreen map (AP-15), not the edit page, a "location" button in the map controls. Only an explicit tap starts it; never automatically, never remembered: every page load starts with it off.
- The browser's Geolocation API, after the browser's own permission prompt. Denied permission, no position, a timeout or an insecure context (only HTTPS, plus `localhost` in development): a short message, nothing else changes.
- The position is drawn as a dot (with the accuracy the browser reports) and keeps updating while location is on (`watchPosition`); updates stop when it is turned off or the page is hidden or closed. A second tap on the location button turns it off and removes the dot.
- **Follow mode:** turning location on centres the map on the position and keeps it centred as the position updates. As soon as the viewer pans, drags or zooms the map, follow mode stops; the dot keeps updating.
- A second "follow" button, shown only while location is on, re-centres the map on the position and resumes follow mode (its visual state, e.g. pressed while following, is the engineer's call).
- The map centres on the position only when location is turned on and when the viewer taps follow; never automatically otherwise.
- `Permissions-Policy: geolocation=(self)` on every response, no other feature listed, so the clipboard and everything else behave as today.
- The position stays in the browser: never sent to our server, never stored (no `lib/prefs`, no storage of any kind), never in the URL or a range link (AP-19), never in the preview image (SH-5, SH-12), never logged.
- Texts in English, German, Spanish and French (DS-6, OQ-228; F-62 is built after F-54).
- About page (INF-1): one sentence that the own position, if a viewer turns it on, is shown only in their browser and never sent to the server, and that turning it on or tapping follow centres the map there, so the map's tile host(s) see that area. No hint on the page itself (screenshots, OQ-290).

## Out of scope
- Sending or sharing the position with anyone; live tracking of the athlete (non-goal); navigation, routing or "distance to route" (an idea at most); remembering the choice.

## Constraints and privacy notes
- **Tile hosts learn the area.** When the map centres on the viewer's position (turning location on, tapping follow, follow mode), tiles for that area are fetched from OpenFreeMap (and Esri when Satellite is on, DS-7), which then learn an approximate area tied to the viewer's IP address This is the viewer's own choice; the About page says so.
- **Permissions-Policy.** The site sends no `Permissions-Policy` today (F-22 H-5); `geolocation=(self)` limits geolocation to our own pages without blocking the clipboard (Copy link).
- **Secure context only.** Geolocation works only over HTTPS; production always serves HTTPS.
- **Screenshots** of the page show the viewer's position; no technical fix and no on-page hint, the About sentence only.
- **Not live tracking:** the dot is the viewer's own position on their own screen, not the athlete's; the non-goal is unaffected.

## Decisions (user, 20 Nov 2026)
- OQ-285 changed: two buttons, location (dot on/off, starts follow mode) and follow (re-centre and resume follow mode, shown only while location is on); any pan, drag or zoom stops follow mode, the dot keeps updating.
- OQ-286 changed: the map centres on the position when location is turned on and on every follow tap, the viewer's own choice; the About page says the tile hosts then see that area; no automatic centring otherwise.
- OQ-287 to OQ-290 as suggested.

## Open questions
- [x] **OQ-285 Show once or follow.** Suggested: the dot keeps updating while it is on (`watchPosition`), but the map never follows on its own; updates stop when it is turned off or the page is hidden or closed.
- [x] **OQ-286 Pan to the position.** Suggested: the tap centres the map on the position once (like panning there by hand); the About page says the tile host(s) then see that map area. Alternative, stricter: never pan, show the dot only when it lies in the current view.
- [x] **OQ-287 Edit page too.** Suggested: no, viewing page only; reconsider if placing pins on site (F-56) needs it.
- [x] **OQ-288 Fullscreen map.** Suggested: yes, the control is also in the fullscreen map (AP-15), the main case of following a route on a phone.
- [x] **OQ-289 Permissions-Policy header.** Suggested: `Permissions-Policy: geolocation=(self)` on every response, no other feature listed, so the clipboard and everything else behave as today.
- [x] **OQ-290 Screenshots.** Suggested: no in-page hint; the About sentence is enough, as the viewer turned it on knowingly.
