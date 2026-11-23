# F-57 GPX route download: architecture

- **Version:** 0.1
- **Status:** accepted (20 Nov 2026). Critical lane (a viewer download of location data): QA at effort high. No open product questions
- **Feature:** [`features/F-57-gpx-route-download.md`](../features/F-57-gpx-route-download.md); decisions OQ-245, OQ-269 to OQ-271
- **Requirements:** SH-13, SH-4, SH-8, SH-9, SH-10, AP-2, AP-3, AP-12, UP-6, INF-1, NFR Performance
- **Builds on:** [F-15](F-15-publish-pipeline-and-static-page.md) P-4 (public data file), [F-16](F-16-start-and-end-hiding.md) H-1, H-6, [F-55](F-55-start-end-hiding-radius.md) HR-6, [F-47](F-47-trim.md), [F-06](F-06-route-map-linked-to-cursor.md) (route = every positioned sample, bridged), [F-39](F-39-how-to-use.md) HU-6 (reserved box), [F-58](F-58-summary-explained.md) SE-1, SE-5 (closed inputs, summary bar), [ADR-005](ADR-005-workout-data-file-format-v1.md), [ADR-011](ADR-011-activity-page-bundle-allocation.md), [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md), [ADR-032](ADR-032-response-headers-in-caddy.md). No new ADR, no format, schema, setting, endpoint or Caddy change
- **Owner:** **both** (was frontend). Frontend does everything except one view-model string that F-15's Go builder must reproduce (GX-5); backend adds it in the same commit, as F-58 SE-2

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| GX-1 | **Built in the browser, on click, from the public data file the viewing page already decoded** (SH-13). No public artefact, endpoint, header, log line or Caddy rule. Rejected: a static `route.{hash}.gpx` written at publish. It would work without JavaScript and cache well, but it repeats F-24's costs (a new artefact under ADR-028's remove-first, allow-list and sweep, SH-10's recall chain, an F-22 `Content-Disposition` rule, storage counting, `regenerate`, a second encoder of the public route in Go), and the product wants no button without JavaScript anyway. **Choose differently** only if a no-JS download or support for in-app browsers that block downloads becomes a requirement | The GPX cannot hold more than the page shows: its only source is the file every viewer already has |
| GX-2 | **Closed inputs.** The viewing glue gives the chunk only: the public data file's `latitude`, `longitude`, the `altitude` stream when present, and `meta.start_time`, `meta.time_zone` (file name). The chunk never gets `WorkoutData` as a whole, the time stream, `gaps`, other metrics, title, description, view model, URL or `public_id`. **Viewing page only:** the call site is on `startPage`'s path without an edit seam (`src/web/src/page/start.ts:169`); on the edit page `data` is the unhidden `draft.bin`, so a wiring there would leak the hidden ends | Privacy by construction (as F-58 SE-1): no input holds a time, a radius or an owner fact |
| GX-3 | **Content, exactly** (below). Every sample with a position, in index order, none skipped, thinned or added. One `<trk>`, one `<trkseg>`: dropouts, pauses and gaps are not split (OQ-270, the map's bridges). Hidden and trimmed parts have no positions in the public file (F-16, F-47) and lie only at the ends, so the first and last points are the first and last visible ones. `lat`, `lon` with exactly 6 decimals (the data file's 10⁻⁶° quantum, F-03 §2), never more digits from float formatting. `<ele>` in metres with exactly 1 decimal (0.1 m quantum), never feet, only where that sample's altitude is not `NaN`: no 0, no interpolation (OQ-245). No `<metadata>`, `<time>`, `<name>`, `<desc>`, `<type>`, `<bounds>`, `<extensions>`, `<wpt>`, `<rte>`. `creator` is required by the schema: the fixed text `Workout Explorer`, no version, no host. The file holds only fixed text and numbers, so no workout or viewer text can enter it and no escaping path exists. Deterministic: the same data file gives the same bytes; LF line ends | AC 2 to 5. Precision equal to the public data's: no false precision, nothing more |
| GX-4 | **File name** `route-YYYY-MM-DD.gpx`, the date the header shows (AP-1): `start_time` shifted by `time_zone.offset_seconds` when known, else UTC, by `lib/format/dates`' rule. After a trim that is the trimmed start (the data file starts there, F-47). **Never derived from a sample's time**: with a hidden start, the first visible sample's date can differ and would reveal the hiding. `route.gpx` when `start_time` does not parse. No title, sport or ID | AC 6, SH-4 |
| GX-5 | **Where and when shown.** New view-model string `Route.Gpx`: `"yes"` exactly when `Route.Map` is `"yes"` (at least one visible position) and the data file's `sport` is not `multisport`, else `""`. Built by `lib/page` and reproduced by the Go builder (`src/server/internal/page/page.go:47`); new or regenerated samples. The template prints, under `{{if .Route.Gpx}}`, a row directly above `<section class="summary">` holding `<button type="button" data-gpx>` "Download route (GPX)", in a reserved box, invisible and inert until enhanced (HU-6: a class, not `hidden`). It sits outside the strip's grid. Revealed once the data file is decoded; on a failed load the box stays empty. **Without JavaScript: the empty box, no button** (feature assumption, F-39 precedent). Not on the edit page (own summary component, no row), the published page or the fullscreen map | AC 1. The condition equals the map's, which the page already shows, so the button itself reveals nothing. A server-printed reserve avoids a shift above the summary (F-13 D-10); a multisport route must not leave an empty gap |
| GX-6 | **Code layout.** Pure builder in `lib/gpx` (no DOM; GX-2 inputs in, string parts and file name out); download glue in the lazy chunk `src/gpx/`, reached by one `import()` from `src/page` through the existing `lazyClick` helper (`src/web/src/page/enhance.ts:176`), with its failure line. Neither is imported statically | AC 8; the TD's `lib`/lazy-chunk split |
| GX-7 | **Delivery.** A `Blob` of type `application/gpx+xml` from the string parts (never one concatenated string), an object URL, a temporary `<a download="{name}">` clicked, the URL revoked asynchronously after the click (not in the same task). No `data:` URL, no `fetch` or XHR of the blob, no `window.open`. **CSP and Caddy unchanged:** an anchor download of a same-origin `blob:` URL is a navigation, not governed by the fetch directives of ADR-032's policy, which has no `sandbox`. Accepted limit: in-app browsers that ignore downloads give nothing, without a message (a fallback needs a server file, GX-1) | AC 7 |
| GX-8 | **Cost.** One synchronous pass on click; no Worker. The 2 s target for the 12-hour ride (43,200 samples) includes the chunk load. If the build itself measures above 100 ms on the reference phone, time-slice it per frame as F-50 does | AC 9 |
| GX-10 | **Public pins as waypoints (user 21 Nov 2026; supersedes GX-3's "no `<wpt>`, `<name>`, `<desc>`, `<type>`, no escaping" for pins only).** Source: the public `pins.{hash}.json` only (URL from the pins slot's `data-url`, `GpxInput.pinsUrl`), fetched lazily on click inside the gpx chunk, only when the page has that URL; no pins file means no fetch, no pins code and a byte-identical file. A failed or invalid file gives the route alone. Never the edit page (GX-2). Point pin: one `<wpt>` at its sample's public position with `<name>` title (else the type name in the page language), `<desc>` when a note exists, `<type>` the pin type; section: "Title (start)" and "Title (end)". `<wpt>`s precede `<trk>`; no time, no `<sym>`, no extensions (`<type>` is portable, an unknown `<sym>` can show a wrong icon). A pin end without a position is skipped. Title and note are XML-escaped (`& < >`). Accepted: the download starts after the fetch, outside the click's task | The pins are what every viewer already sees; positions come from the same privacy-applied data file as the track |
| GX-9 | **Texts.** The edit line goes under the hiding sliders (`src/web/src/components/edit/EditPanel.svelte`, owner chunk); the About sentence goes into `src/web/about.html` next to "A hidden start or end of the route is removed from the public data" (INF-1). Each text in one place, for F-54 | AC 10, OQ-271 |

### GX-3 file shape

```xml
<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="Workout Explorer" xmlns="http://www.topografix.com/GPX/1/1">
<trk><trkseg>
<trkpt lat="47.123456" lon="8.654321"><ele>412.3</ele></trkpt>
<trkpt lat="47.123501" lon="8.654410"/>
</trkseg></trk>
</gpx>
```

## What cannot leak (SH-4, F-55 HR-6)

- **Radius, ends, amount hidden:** no input holds them (GX-2); the GPX is a subset of the map, whose ends are already the visible ends. The button shows exactly where the map shows (GX-5).
- **Dates:** from the metadata, which is the same with hiding on or off (F-16 H-1), never from samples (GX-4).
- **Open pages after a raise:** a page loaded before a remove-first change can still download the older, less hidden route until it is reloaded. That page already shows that route on its map; SH-10's recall notice covers copies. No new exposure.
- **Requests:** the first click fetches the chunk, a static asset sent without referrer and not tied to a workout. That is AC 8's on-demand load. AC 7's "no network request" is read as "no request for workout data, nothing new on the server".

## Cost

| What | Limit |
| --- | --- |
| Entry (static) | Enhancer, `import()` call site, row CSS: ceiling **150 bytes gzip** from the ADR-011 reserve (2,544 after F-58). The engineer records the measured figure in ADR-011 item 4; above: stop and report |
| Lazy chunk | About 1,000 bytes gzip; above: report |
| HTML, LCP | One row of markup on route pages; nothing new on the first-paint path |

## Verification (critical lane: QA at effort high)

Synthetic files only (ADR-004). The GPX schema validator is a test-time tool only; nothing ships with it.

| What | Check |
| --- | --- |
| AC 2 to 5 | Over published synthetic samples (ride with dropouts and pauses, trimmed, start only, end only, both ends hidden at several radii): the points equal the public data file's positioned samples, in order, at 6 and 1 decimals; ele exactly where altitude is present; one segment; schema-valid; no element outside GX-3; two builds byte-equal |
| GX-4 | A hidden start that crosses local midnight: the file name keeps the header's date |
| AC 1 | Button row per `Route.Gpx`: ride shown; indoor, pool, fully hidden, fully trimmed away, multisport (also ride, transition, ride) absent; two cycling sessions shown; Go and `lib/page` agree on every sample; edit and published pages have no button |
| AC 7, 8, 9 | No request besides the chunk; entry ceiling; 12-hour ride on the reference phone |
| Mutants | Each must fail a check: builder given `WorkoutData` and writing a time; `NaN` altitude written as 0; coordinates through `String(x)`; date from the first positioned sample; a new segment per gap; the call site also on the edit path; `Route.Gpx` set for multisport or for a fully hidden route |

## Not repeated from F-24 (ADR-036, superseded)

No server encoder, no artefact or header, no owner setting, no unit conversions, no samples written without a position, no fallback structures.

## Hand-offs

| To | What |
| --- | --- |
| Main session, requirements | Owner becomes both (GX-5, one commit). TD Code structure: `lib/gpx` and the lazy `src/gpx/`; view model `Route.Gpx`. AC 7 reading as above |
| Backend (B-1) | `Route.Gpx` in the Go builder, samples equal |
| F-54 | Translate the label and the two sentences (GX-9) |
