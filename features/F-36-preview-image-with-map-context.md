# F-36 Preview image with map context

- **Phase:** 2 (after F-35, before F-34)
- **Owner:** both
- **Requirements:** SH-12 (Could), SH-5, SH-4, SH-10, SH-11, UP-7, AC-4, INF-1, NFR Privacy (no third party from the server), NFR Security; extends F-23, F-20, F-30
- **Depends on:** F-23, F-20, F-30

## Goal
A link posted in a chat shows the route on a map, not on a plain background, so friends see at a glance where the workout went.

## Scope
- At publish, the edit page (which already shows the MapLibre map with OpenFreeMap tiles) renders only the map (1200 × 630 PNG) in the browser and sends it; the server draws the public route, sport icon, name and headline values of SH-5 (same values, metric units, same cut at 40 characters) and the attribution over it.
- Public route only: hidden start and end (SH-4) left out; the frame is fitted to the public route only, never to the full draft route. Later privacy zones (SH-3) and trim (UP-6) apply the same way once they exist.
- The image carries the map attribution "© OpenStreetMap, OpenFreeMap". The About page's attribution list already covers the licence obligations (F-29); the About page adds the privacy note of SH-12 (the preview shows where the route is, as the public page does).
- The browser sends the map with the publish request, or in a request authenticated by the edit token (architect decides). Only the edit-link holder can send one.
- Server, at the trust boundary: size cap on the received bytes (architect sets the value); must decode as a PNG of exactly 1200 × 630; re-encoded; only the re-encoded copy (under 300 KB, no ancillary chunks from the upload) is stored and served, never the received bytes; it replaces the server-made image for that workout and is deleted with the workout (UP-7, AC-4). Refused: the server keeps its own image, publish still succeeds.
- Fallback to today's server-made image (F-23) whenever anything fails: no WebGL, tiles fail, time limit, refusal, any error. Publishing never fails or waits because of the image; the edit page may wait at most 10 s for it before opening the published page, so a link copied there already has the map card. No positions in the public route (indoor, fully hidden): server-made image, no browser image.
- After publish, any change of hidden ends, route (later trim, privacy zones) or name drops the browser-made image or replaces it with a new one (architect decides), within SH-10's 60 s; an image with a now hidden part is never served.
- `regenerate` (F-30) cannot redraw a browser-made image: it keeps it while it is still valid for the current public route, otherwise falls back to the server-made image (architect decides how validity is checked).
- No new server calls to external services. The tiles are fetched by the sharer's browser, as on the edit page today.

## Out of scope
- Map context for draft, expired, edit or published-page links (SH-5, SH-11 unchanged).
- Route colouring, pins, lap or start and finish markers on the image; units other than metric (assumed, spec open question).
- The server rendering tiles itself.

## Constraints (for the architect)
- The image code loads only on the edit page, lazily at publish; the viewing page and its budgets are untouched.
- The stored image must stay under 300 KB (F-23, messenger limits); a 1200 × 630 map PNG can exceed that, so size reduction (for example a reduced palette) needs a decision. Above the limit after re-encoding: fallback.
- The upload is untrusted input: decode with limits before allocating for the full image; a crafted PNG fails only its own request.
- Assumed until decided (spec open question SH-12): browser gives up after 10 s; fixed light map style and card, whatever the sharer's theme.
- Test images are synthetic; real routes are never committed.

## Acceptance criteria
- [ ] Publishing a ride with GPS in a browser with WebGL stores a 1200 × 630 PNG under 300 KB showing the route on OpenFreeMap tiles, the ride icon, name and SH-5's values, and "© OpenStreetMap, OpenFreeMap".
- [ ] With start and end hiding on, the image shows no part of the hidden sections and its frame fits the public route only (compared with hiding off).
- [ ] With WebGL unavailable (test stub), with tile requests blocked, and with the browser's image request failing, publishing succeeds and the workout serves the server-made image.
- [ ] A workout without positions, and one hidden entirely, get the server-made image.
- [ ] The server refuses, and keeps its own image: bytes over the size cap, a non-PNG, a PNG of another size, a truncated or crafted PNG; none fails the service or another workout.
- [ ] The served image is never byte-identical to the uploaded bytes, and carries none of the upload's text or other ancillary chunks.
- [ ] A request without a valid edit token cannot set or replace the image.
- [ ] Turning on hiding, or changing the name, after publish: within 60 s the served image no longer shows the old content (new browser image or server-made image).
- [ ] `regenerate` keeps a browser-made image that matches the current public route and replaces one that does not with the server-made image.
- [ ] Deleting or expiry removes the image with the workout.
- [ ] The server makes no request to any external host during publish (request log); the viewing page's bundle and LCP are unchanged.
- [ ] The About page states that the preview image shows the route on a map.
- [ ] Manual check: the card with the map renders in WhatsApp, Signal, Slack, Telegram and X, and the attribution is legible.
