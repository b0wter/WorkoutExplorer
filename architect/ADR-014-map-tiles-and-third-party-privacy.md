# ADR-014 Map tiles: OpenFreeMap liberty, the tile host as the only third party, no referrer, no credentials

- **Status:** accepted
- **Date:** 12 Oct 2026; revision 2 of 14 Oct 2026; revision 3 of 24 Oct 2026 (F-14: dark style named and verified)
- **History:** [architect/history/ADR-014-map-tiles-and-third-party-privacy.md](history/ADR-014-map-tiles-and-third-party-privacy.md)
- **Features:** F-06 (introduces it); F-07, F-13 (page `<meta name="referrer">`), F-14 (dark style, decision 28), F-22 (CSP and `Referrer-Policy` headers), F-29 (About page and attribution), F-31 (Caddy), F-32 (launch checks)
- **Supersedes:** none
- **Depends on:** [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md)
- **Requirements:** TD Tech stack (Map tiles: OpenFreeMap, free, no key; fallback a self-hosted Protomaps file), TD Security and privacy (CSP: "tiles and fonts only from the tile host"; `Referrer-Policy: no-referrer` on workout pages; GDPR: "the tile host sees viewers' IP addresses"); NFR Privacy (no tracking, store as little as possible); INF-1 (attribution of the map data; who sees what); AP-3

## Context

The map is the only part of the viewer that talks to a host other than ours. OpenFreeMap's published policy (checked 12 Oct 2026, openfreemap.org/privacy): no cookies or tracking; access logs anonymised and without IP addresses by default, but with "referring pages"; error logs may hold IP and URL; logs deleted after 7 days (30 after security incidents); Cloudflare may serve as CDN.

What reaches the tile host from a viewer's browser, whatever we do: the IP address, the user agent, and the tiles requested, which reveal the map area being viewed and so roughly where the route is. What we can control: the `Referer` header (otherwise the origin, or with a looser policy the full `/w/{public_id}` URL), cookies or credentials, and whether any other third party is contacted.

A subtlety: MapLibre fetches vector tiles inside its Web Worker. A module worker loaded from our origin takes its referrer policy from the worker script's own response headers, not from the page's `<meta name="referrer">`. A policy set only on workout pages therefore does not cover tile requests.

## Decision

1. **Tile host:** OpenFreeMap, style **liberty** (`https://tiles.openfreemap.org/styles/liberty`; rev. 1, user decision on OQ-18, was positron). Verified 12 Oct 2026: style, TileJSON (`/planet`), vector tiles, the raster hillshade source `ne2_shaded` (PNG, `/natural_earth/ne2sr/{z}/{x}/{y}.png`), glyphs (`/fonts/…`) and sprite (`/sprites/ofm_f384/ofm`) all come from `https://tiles.openfreemap.org`, all as absolute URLs. No other host is fetched; the attribution only *links* to openfreemap.org, openmaptiles.org and openstreetmap.org. If a later version of the style adds another host, it violates I-6 and stays blocked by the CSP: decide here first. (rev. 3) In dark mode F-14 uses OpenFreeMap **dark** (`https://tiles.openfreemap.org/styles/dark`), verified 24 Oct 2026: same host only, same `openmaptiles` source, `ne2_shaded` raster, sprite, glyphs and fontstack as liberty; CSP, referrer and attribution unchanged (F-14 design D-12). Each URL is one constant in `map-style.ts`.
2. **The tile host is the only third party the viewer's browser contacts** (TD invariant I-6). No fonts, scripts, analytics or images from anywhere else. MapLibre has no telemetry; its logo stays off.
3. **No referrer.** Every page that loads the map carries `<meta name="referrer" content="no-referrer">` (dev pages from F-06, the prototype page in F-07, the template in F-13). F-22 sends `Referrer-Policy: no-referrer` on **every** response of the site, including JavaScript assets, so the MapLibre worker's tile requests carry no referrer either. Until F-22, the browser default (`strict-origin-when-cross-origin`) sends at most the origin, never the workout path.
4. **No credentials.** Our style fetch uses `credentials: 'omit'`; MapLibre's requests use its default (`same-origin`), which sends no cookies cross-origin. Never set `credentials: 'include'`.
5. **Attribution:** MapLibre's attribution control, `compact: true`, shows the attribution of the remote style's sources. Expanded, it must name OpenFreeMap, OpenMapTiles and OpenStreetMap, each linked. Liberty's TileJSON carries all three (verified), so no `customAttribution` is needed; add it only if MD9 shows one missing. The raster source `ne2_shaded` (Natural Earth, public domain) carries no attribution and needs none. F-29 lists OpenFreeMap, OpenMapTiles, OpenStreetMap (ODbL) and Natural Earth on the About page.
6. **Hand-off to F-22 (CSP for pages that load the map):**

   (rev. 2) A dedicated worker does not inherit the page's CSP or referrer policy. It takes both from **its own script response**. F-22 therefore sends the CSP below **and** `Referrer-Policy: no-referrer` on the MapLibre worker asset (`/assets/maplibre-gl-worker-*.js`, emitted by `?worker&url`), not only on the HTML. The simplest correct rule is both headers on every response of the site. The F-22 acceptance check includes a tile request from the worker with no `Referer` header, and the worker response carrying the CSP.

   | Directive | Sources | For |
   | --- | --- | --- |
   | `script-src` | `'self'` (no `'unsafe-eval'`, no `'unsafe-inline'`, no `blob:`) | Bundle and MapLibre chunk |
   | `worker-src` | `'self'` | MapLibre worker from `?worker&url` (no `blob:`) |
   | `connect-src` | `'self' https://tiles.openfreemap.org` | Data file; style, TileJSON, vector and raster tiles, glyphs, sprite |
   | `img-src` | `'self' data: blob: https://tiles.openfreemap.org` | MapLibre's sprite and control images (MapLibre docs); rev. 1: the tile host too, for liberty's raster tiles and sprite if MapLibre loads an image through an element instead of `fetch` |
   | `style-src` | `'self'` | Bundle CSS and MapLibre CSS files; MapLibre sets styles through the DOM API, which CSP allows |
   | Header | `Referrer-Policy: no-referrer` | Site-wide, decision 3 |

## Consequences

- OpenFreeMap never learns which workout is viewed; it learns an IP address, a user agent and a map area. The user decided (OQ-17) that the About page says so, including the CDN; the recommended wording is in the F-06 design v1.1, "Hand-off to F-29".
- Viewers who block `tiles.openfreemap.org` still see the route on a plain background (ADR-013 decision 2).
- Changing the tile host is a style URL, a CSP line, an attribution check and an About-page line.
- **Cost (rev. 1):** liberty is heavier than positron: 111 layers, a raster hillshade source and three fontstacks. That means more requests to the tile host and more start-up and pan work. Scrubbing is unaffected (ADR-013 decision 4). Its orange roads and blue water take those hues, so the route uses magenta (F-06 design). The start-up measurement (F-06 MD11, MP1) covers the extra work.

## Would choose differently if

- OpenFreeMap's terms or availability change, or the user wants no third party at all. Then serve a self-hosted Protomaps file through Caddy (the TD fallback): no third party, at the cost of disk space, an update job and our own glyphs and sprites.
- The start-up trigger fires on the phone and the ADR-013 remedies are not enough. Then the user may choose positron, which is lighter, in a revision of this ADR.
