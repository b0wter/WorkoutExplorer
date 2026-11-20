# F-61 Satellite map layer

- **Status:** decided (20 Nov 2026); no open user questions
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (backend: the CSP line in the Caddyfile and its test)
- **Lane:** normal, QA at effort high (changes the privacy rule "the tile host is the only third party", TD I-6, ADR-014)
- **Requirements:** DS-7 (new); AP-3, AP-9, AP-15, SH-4, SH-12, PIN-6, DS-4, DS-6, INF-1; NFR Privacy, NFR Performance
- **Depends on:** F-06 (route map), F-14 (`lib/prefs`, theme), F-22 (headers), F-26 (fullscreen map), F-29 (About page and attributions), F-36 (preview image with map), F-56 (pins), F-54 (translations; F-61 comes right after it)

## Goal
A viewer switches the route map to satellite imagery to see the terrain, trails and surroundings, and the next workout opened in the same browser keeps that choice.

## Scope
- A "Map / Satellite" toggle on the route map of the viewing page, the edit page and the fullscreen map (AP-15): an icon button in the map's top-right control stack, under fullscreen and pin (decision of 8 Nov 2026), pressed while Satellite is on. **Map** is OpenFreeMap as today (ADR-014) and the default; **Satellite** is Esri World Imagery raster tiles, keyless.
- With Satellite, street and place names (the label/symbol layers of the OpenFreeMap vector tiles already in use) are drawn over the imagery with a halo so they stay legible. No further host: only Esri imagery plus the existing OpenFreeMap host. Esri's own reference/label layers are not used.
- The choice is remembered in this browser like the theme (DS-4): through `lib/prefs`, no cookie, never sent to the server; only a choice made with the toggle is stored. Broken, blocked or unreadable storage falls back to Map without a message; the toggle still works for that page.
- Switching keeps everything else as it was: viewport, route and its colouring (AP-9), start, finish and lap markers, pins and legend (PIN-6), cursor and selection, the edit page's hidden-end shading (SH-4).
- The route, markers and pins stay clearly visible on imagery in both themes; the imagery itself is the same in light and dark theme.
- While Satellite is on, the map shows Esri's required attribution and the OpenFreeMap attribution (for the labels); while Map is on, the OpenFreeMap attribution as today.
- Esri's host is contacted only while Satellite is shown: a viewer who never chooses it makes no request to Esri. With Satellite remembered, later pages load imagery from Esri without a new tap.
- Same rules as for OpenFreeMap: no referrer, no credentials, allowed by the CSP only for that host.
- The SH-12 preview image is always made on OpenFreeMap, whatever the sharer chose.
- About page (INF-1): the "In your browser" paragraph names the map choice; "who sees what" says that with Satellite the imagery provider (Esri, a US company) sees the viewer's IP address and the map area viewed, like the tile host; the attributions list names Esri and its imagery sources.
- Texts in English, German, Spanish and French (F-54 is built before F-61).

## Out of scope
- Other imagery providers; MapTiler or any keyed source (fallback only if Esri becomes a problem, a new decision then). A hoster setting to turn satellite off or choose the provider.
- Esri's reference/label layers; roads or other vector layers over the imagery beyond the labels. Satellite in the preview image (SH-12). A per-workout default set by the sharer.
- Changes to ADR-014's OpenFreeMap rules other than the third-party list.

## Assumptions
- The rule changes from "the tile host is the only third party" to "the map's tile hosts (OpenFreeMap; Esri only after the viewer chose Satellite) are the only third parties"; the architect updates TD I-6, ADR-014 (or a new ADR), the CSP and the Caddyfile test.
- The licensing grey area of using Esri World Imagery keyless is accepted for a friends-only app (user, 20 Nov 2026).
- No new server data, endpoint or log; the server contacts no one new.

## Acceptance criteria
- [ ] The viewing page and the edit page of a workout with a visible route show the toggle as an icon button in the map's top-right control stack, pressed while Satellite is on, also in the fullscreen map; a page without a map (no location data, route entirely hidden, AP-3) shows none.
- [ ] Map is the default on a first visit and after storage is cleared.
- [ ] Choosing Satellite shows Esri imagery under the route, with street and place names from the OpenFreeMap vector tiles drawn over it with a halo, legible on imagery (checked by the user on the phone); choosing Map returns to OpenFreeMap. Viewport, route colouring, markers, pins and legend, cursor, selection and the edit page's shading are unchanged by a switch.
- [ ] The choice is remembered: a reload, another workout and the edit page open in the last chosen layer. Only a toggle choice is stored, through `lib/prefs`; nothing reaches the server (no cookie, header, URL or request carries it).
- [ ] Broken, blocked or unreadable storage: the page opens with Map, no error or message, and the toggle works for that page.
- [ ] Test (network log): with Map, no request goes to an Esri host from any page. With Satellite, the only hosts contacted for the map are Esri's imagery host and the existing OpenFreeMap host (labels); no request goes to any Esri reference/label service; Esri requests carry no `Referer` and no cookies or credentials.
- [ ] The CSP allows Esri's imagery host only where the map needs it and nothing else new; every other directive is unchanged. A test pins the new CSP string.
- [ ] While Satellite is on, the map shows Esri's required attribution and the OpenFreeMap attribution; while Map is on, the OpenFreeMap attribution as today.
- [ ] Imagery failing (blocked, offline, an error, no imagery at the zoom): the route stays on a plain background with no message; the choice stays Satellite; the zoom range stays as on Map; the page never breaks and the toggle keeps working.
- [ ] The route, start, finish and lap markers, pins and the edit page's shading are clearly visible on imagery in light and dark theme, checked by the user on the phone.
- [ ] The SH-12 preview image is made on OpenFreeMap even when the sharer's browser has Satellite remembered (test).
- [ ] About page: the "In your browser" paragraph names the map choice; "who sees what" names Esri, that it sees the IP address and the map area only after Satellite is chosen, and that it is a US company; the attributions list names Esri and its imagery sources.
- [ ] Toggle texts and accessible names exist in all four languages (DS-6).
- [ ] With Map, LCP and the entry bundle are unchanged (no satellite code or request before the toggle is used or Satellite is remembered); with Satellite, scrubbing stays at 60 fps on the 12-hour ride (NFR Performance).

## Decisions (user, 20 Nov 2026)
- Toggle on the route map of the viewing and edit pages; remembered per browser like the theme; OpenFreeMap stays the default.
- Imagery from Esri World Imagery, keyless, with the required attribution; licensing grey area accepted; MapTiler (free key) is the fallback if it becomes a problem.
- Esri is a second third party, contacted only after the viewer chose Satellite: I-6, ADR-014, CSP, referrer and credentials rules, About page and attribution change accordingly. The SH-12 preview image stays on OpenFreeMap.
- Normal lane, QA at effort high. Build order: right after F-54.
- OQ-282 as suggested; Satellite off by default (Map is the default).
- OQ-283 changed: street and place names over the imagery, from the OpenFreeMap vector tiles already in use, with a halo; no further host; Esri's own label layers not used.
- OQ-284 as suggested.

## Open questions
- [x] **OQ-282 Toggle placement.** Suggested: an icon button in the map's top-right control stack, under fullscreen and pin (decision of 8 Nov 2026), pressed state when Satellite is on; also shown in the fullscreen map (AP-15).
- [x] **OQ-283 Labels over the imagery.** Suggested: none, imagery only. Decided (changed): street and place names from the OpenFreeMap vector tiles over the imagery, with a halo; no further host.
- [x] **OQ-284 Imagery not available** (blocked, offline, error, no imagery at high zoom). Suggested: as with OpenFreeMap today, the route stays on a plain background with no message; the choice stays Satellite; the zoom range stays as on Map (how deep zoom without imagery looks is the engineer's call, checked on the phone).
