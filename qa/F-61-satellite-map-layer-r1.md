# QA F-61 Satellite map layer - round 1

- **Date:** 2026-11-20
- **Verdict:** ready with conditions - no Blocker or Major finding. Conditions: the Esri behaviour check that the design requires (CORS, no redirect, no key) is not recorded anywhere I can read, and the real-browser network log and the on-phone checks (AC 3, 6, 8, 9, 12, 14) are still open.
- **Inputs:** `features/F-61-satellite-map-layer.md` (14 ACs), `architect/F-61-satellite-map-layer.md` v0.1 (SL-1 to SL-11), ADR-014 rev. 4, ADR-011 F-61 row, ADR-041, F-54 LG-3/LG-8; spec DS-7, SH-12, INF-1, TD I-6. Commits `8ae812b`, `5b5dd2d`, `38025e2` (branch `f-54-frontend`, clean tree).

## Build and test results
- `make ci` at `38025e2`: **exit 0**. Vitest 86 files, 2,226 tests passed. Go all ok. `caddy-check` (Caddy v2.11.9, pinned CSP string) ok. Built `about.html` lists "Esri World Imagery (satellite map layer) … Source: Esri, Vantor, Earthstar Geographics, and the GIS User Community" (AB-5/AB-6 build step passed).
- Size: entry 60,281 / 62,000. F-61 adds **+67** against its 100 ceiling. The map chunk grows by +364 JS and +128 CSS gzip, well below SL-1's ~3 KB trigger.
- Mutants, run on a copy of the tree in the scratchpad:

| # | Mutant | Result |
| --- | --- | --- |
| S1 | Satellite style applied while Map is on | killed (5 tests, including "Map by default … remote style as is") |
| S2 | `poi` labels kept on Satellite | killed |
| S3 | `basemap.ts` reads `wx.map` (SH-12 preview) | killed (`F-61 AC 11 … storage never read`) |
| S4 | No prefs → Satellite | killed |
| S5 | `mapLayer()` default Satellite | killed (7 tests) |
| S6 | Toggle click not stored | killed |
| S7 | `page/start.ts` does not hand `prefs` to the map | **survived** (553 tests in `src/page`, `src/components/map`, `src/edit` pass; QA-61-r1-1) |
| S8 | Label paint over imagery not applied | killed |
| S9 | Hillshade raster kept on Satellite | killed |
| S10 | Toggle not relabelled on a language change | killed |
| S11 | `applied()` ignores the layer (no switch) | killed |
| S12 | About drops "a US company" | killed (`about.test.ts`) |
| C1 | Esri `sources.json` entry without its origin | killed (`go run ./tools/attributions`: "allows https://server.arcgisonline.com but no entry … claims it") |
| C2 | Second Esri origin `services.arcgisonline.com` in the CSP | killed (attributions: unclaimed origin). The Caddy test also pins the full string |

## Findings

### Minor

- **ID:** QA-61-r1-1
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:286` (`}, edit, prefs) : null;`), `src/web/src/page/map-props.svelte.ts:110`
- **Relates to:** AC 4 (a reload, another workout and the edit page open in the last chosen layer)
- **Evidence:** mutant S7 hands `undefined` instead of the page's prefs, and no test fails. `RouteMap.dom.test.ts` checks only that whatever prefs the props carry reach the map. `map-view.dom.test.ts` gets its prefs directly.
- **Consequence:** if the page-glue line regresses, the toggle still works but is never remembered or read back, and the tests stay green. One `startPage` test with `wx.map=satellite` in storage would close the gap.

## Previous findings
None (round 1).

## Questions
1. **Esri behaviour check (design "Verification", SL-7).** The design says to stop if the tile URL lacks CORS, redirects or needs a key. I did not contact Esri (no live calls), and the commits record only the `copyrightText` check (`map-style.ts:408`). Has the engineer confirmed `Access-Control-Allow-Origin` and no redirect on `https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}`? A one-line note in the F-61 design ("verified 20 Nov 2026: ACAO \*, 200, no redirect") would settle it.
2. **AC 7 "only where the map needs it".** The CSP allows Esri on every page, because there is one site-wide policy (F-22 H-3). ADR-014 rev. 4 Consequences and SL-7 record this, so I read the AC as met by that decision. If the user meant per-page CSPs, that is a new decision.

## Coverage

| AC | Code | Test / check | State |
| --- | --- | --- | --- |
| 1 Toggle in top-right stack, pressed state, fullscreen; none without a map | `map-view.ts:370-393`, `map-view.css` (flex order) | "Map by default: a top-right control…" | traced. Order with the phone pin toggle and in fullscreen: hand check (CSS) |
| 2 Map default, also after storage cleared | `lib/prefs` `mapLayer` | prefs test, S4, S5 killed | traced |
| 3 Esri imagery + OpenFreeMap labels with halo; switch keeps viewport, colouring, markers, pins, shading | `satelliteStyle`, `applyRemote` via `mergeStyle` | style tests, S2, S8, S9 killed; "viewport stays" (no `fitBounds`) | traced. Legibility and real MapLibre re-apply on `style.load`: hand check on the phone |
| 4 Remembered: reload, other workout, edit page; only toggle writes; nothing to the server | `prefs.setMapLayer` in click only; `start.ts` → `createMapProps` | S6 killed; **S7 survived** (r1-1) | partly traced |
| 5 Broken storage → Map, toggle works | `createViewerPrefs` | "unreadable storage … toggle still switches" | traced |
| 6 Network: no Esri with Map (first load and theme change); only the imagery host and OpenFreeMap with Satellite; no Referer, no credentials | only `setStyle` site is `map-view.ts:705`; scheme change goes through `applyRemote` (S11); `Referrer-Policy: no-referrer` site-wide (Caddy test); no `transformRequest`, no `credentials` | S1, S11 killed | traced by code and fake. **Real network log: hand check** |
| 7 CSP: Esri origin in `connect-src` and `img-src` only, pinned | `deploy/Caddyfile:47`, `caddy_test.go:34` | `caddy-check` ok, C1, C2 killed | traced (see Question 2) |
| 8 Attribution while Satellite / Map | raster source `attribution` | style test ("Powered by Esri", sources line) | traced. MapLibre dropping the Esri line after a switch back to Map: hand check |
| 9 Imagery failing: plain background, no message, stays Satellite, same zoom range | `background` layer kept, `error` swallowed, `maxzoom: 18` overzoom | style tests | traced. Offline and blocked behaviour: hand check |
| 10 Visible on imagery in both themes | theme tokens unchanged (no SL-10 override added) | — | hand check (user, phone) |
| 11 SH-12 preview always OpenFreeMap | `basemap.ts` (no prefs read) | S3 killed | traced |
| 12 About: In your browser, who sees what (Esri, IP, map area, US company, only after Satellite), attribution list | `about.html:68`, `:102`, `sources.json` | `about.test.ts`, S12, C1 killed; built page checked | traced. Esri privacy link target (`/en-us/privacy/overview`) not opened (no live call) |
| 13 Texts in four languages | `map-texts.ts` `satellite`, `ab_ifYouSwitch`, `ab_esriPrivacyLink`, `ab_yourBrowserRemembers` in de/es/fr | F-54 completeness tests | traced. Wording reads correctly in all three languages |
| 14 Map: entry and LCP unchanged; Satellite: 60 fps on the 12-hour ride | toggle and style in the lazy chunk | size +67 / 100 | entry traced. 60 fps: hand check |

Checked and found correct: the toggle's SVG is a constant in `innerHTML`, and its labels are set as text. The Satellite style keeps only the `background` layer, one `satellite` raster source, and `symbol` layers of `transportation_name` and `place` with their own source. Glyphs and sprite are kept, and `ne2_shaded` is gone. Going back to Map gives the remote style object unchanged (no Esri source survives `mergeStyle`). A quick Satellite→Map double tap within the quiet period applies nothing (the `applied()` check after `whenQuiet`). `DOCKER.md`, TD I-6, ADR-014 rev. 4, the spec SH-12 text and `sources.json` all say the same thing.

Hand checks (user or main session): Esri CORS and redirect confirmation (Question 1); browser network log on viewing, edit, published, upload and About pages with Map and with `wx.map=satellite` (no Esri with Map, only the tile path with Satellite, no Referer, no cookie); legibility and route/marker/pin/shading contrast in both themes on the phone; attribution after switching both ways; imagery blocked or offline; control order with the pin toggle on the phone and in fullscreen; 60 fps scrubbing on the 12-hour ride with Satellite.
