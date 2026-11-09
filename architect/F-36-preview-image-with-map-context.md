# F-36 Preview image with map context: architecture

- **Version:** 0.1
- **Status:** draft (9 Nov 2026). Critical lane: hidden route ends (SH-4) and an image uploaded from the browser
- **Feature:** [`features/F-36-preview-image-with-map-context.md`](../features/F-36-preview-image-with-map-context.md)
- **Requirements:** SH-12, SH-5, SH-4, SH-10, SH-11, UP-7, AC-4, INF-1, NFR Privacy, NFR Security; TD invariants I-3, I-4, I-5, I-6
- **Builds on:** [F-23](F-23-preview-card-and-indexing.md) PC-1 to PC-5, [ADR-033](ADR-033-preview-image-rendered-with-x-image.md); [F-15](F-15-publish-pipeline-and-static-page.md) P-1, P-4, P-7; [F-20](F-20-publish-share-delete-controls.md) C-3; [F-30](F-30-regenerate-command.md) RG-4, RG-5; ADR-013, ADR-014, ADR-017, ADR-022, ADR-025, ADR-028, ADR-032. New: [ADR-041](ADR-041-preview-basemap-from-the-browser-composed-on-the-server.md)
- **Owner and order:** both. **B-1 backend first** (frame, validation, composition, storage, two owner endpoints), then **FE-1** (basemap job on the edit page, About paragraph)
- **Bundle:** 0 bytes on the activity entry and on ADR-024's line. The basemap module is its own lazy chunk, reached only from the owner chunk

## Shape

```
Publish click, after the pending save's 200 (F-20 C-3)
 ├─ POST …/publish ─────────────▶ public build as today (server-made card)
 └─ basemap job, one 10 s budget:
      GET …/preview-map          ◀─ {frame, key, current}, computed from the public data file
      offscreen MapLibre at frame: OpenFreeMap liberty only, no route, no text
      PUT …/preview-map/{key}    ─▶ validate, re-encode, store private/{id}/preview-map.{key}.png
                                     → in-place public build: Go draws route, icon, name, values and
                                       attribution over the basemap when key = the build's frame key
 └─ published page opens once publish succeeded and the job ended
```

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| MC-1 | **The browser draws only the basemap; Go draws everything else** (route, sport icon, name, values, attribution) over it (ADR-041 item 1). The route still comes only from the decoded public data file (F-23 PC-1 unchanged) | Hidden ends can reach the image only through Go code that already never sees them, so privacy is proven in Go tests. SH-5's selection, the 40-character cut and OQ-92 stay in one language. A rename needs no new browser image |
| MC-2 | **Preview frame, server-made** (pure, `internal/preview`): Web Mercator as MapLibre draws it (512 px world tile, bearing 0, pitch 0, canvas 1200 × 630, pixel ratio 1). The bounding box of the public file's positions is fitted into the route area right of the text panel, with padding. Zoom is fractional and capped; degenerate routes use F-23's minimum span. Centre and zoom are rounded to fixed decimals, and Go projects the route with the rounded values. **Key** = first 16 hex of SHA-256 over the canonical text of the rounded frame plus a layout version. No public positions (indoor, fully hidden): no frame, server-made card only. Frame and key are never logged (I-5) | One frame rule, in the language that knows the public route. Rounding makes the browser's camera and Go's projection use identical numbers. The engineer picks the cap, padding and decimals |
| MC-3 | **Validity = key match, checked by every build.** The public build (P-4) uses the stored basemap only when its key equals the key of the frame it computes now; otherwise it draws F-23's server-made card. No row field, no migration | One rule covers hiding, later trim and zones, parser changes and `regenerate` (AC 43). A basemap fitted to a wider route can never be used for a narrower one |
| MC-4 | **Two owner endpoints** (ADR-025 check, no query string, `no-store`). `GET /api/workouts/{public_id}/preview-map` → `{"frame":null}` or `{"frame":{"lon":…,"lat":…,"zoom":…},"key":"…","current":bool}` (the build's front half: parse, transform, encode, decode; one parse slot; writes nothing). `PUT /api/workouts/{public_id}/preview-map/{key}`, body = PNG bytes. Order: token check → `{key}` 16 lowercase hex, else not found → image slot (one in flight, else `503`) → read under the cap → MC-5 → current key must equal `{key}`, else `409` → under the O-7 lock, `privatedir` stores the map → if unlisted, the F-12 hook with before = after (an in-place build, P-7 "no difference"). `204` on success; refusals are fixed JSON errors that the browser ignores. Drafts accepted too: the publish build picks the map up | Separate from publish, so publish never waits for or fails on the image (SH-12). Either order of publish and PUT converges, because the build decides (MC-3). The `409` keeps a late PUT from a stale tab replacing a current map |
| MC-5 | **Trust boundary** (pure function in `internal/preview`). Cap **4,000,000 bytes**, read through `http.MaxBytesReader`; over the cap, refused without reading on. `png.DecodeConfig` first: PNG, exactly 1200 × 630, else refused before any pixel allocation. Then `png.Decode` of the bytes in memory: allocation is bounded by the fixed size (at most about 6 MB at 16 bits); excess pixel data, bad CRCs, truncation and trailing garbage are errors. Any PNG colour model is accepted and flattened to opaque RGB. Stored: our own PNG encoding of the decoded pixels. Served: only MC-6's composite. One INFO line per refusal: `id` and a fixed reason (`size`, `format`, `dimensions`, `decode`, `stale`), never the frame, key or bytes | The largest PNG a canvas writes at 1200 × 630 (RGBA, stored deflate) is about 3.05 MB, so the cap admits every honest upload. 300 KB applies to the output, not the input. The served file can never carry upload bytes or chunks (AC 40) |
| MC-6 | **Composition** (pure, deterministic, `internal/preview`): basemap on the whole canvas; an opaque light panel on the left with F-23's icon, name and values; the route with a light casing in the route area, projected by MC-2; "© OpenStreetMap, OpenFreeMap" on a light box at the bottom right, inside the central 1200 × 600 band, legible (at least 18 px). Output is a paletted PNG (at most 256 colours, our own deterministic quantizer, standard library only; the engineer may step down to 128 and 64 colours). If the output is still over 300 KB, or reading or decoding the stored map fails, the build uses F-23's card. The user reviews the look once on three synthetic cards, as for OQ-91 | A full-colour map PNG at this size exceeds 300 KB. A failure on the image side never fails a build |
| MC-7 | **Browser rendering** (`components/map/basemap.ts`, the second MapLibre value importer, reached only through `import()` from `src/edit`; note in ADR-013). **Offscreen, never a capture of the visible map**: the visible map follows the theme and draws the route, the grey to-be-hidden sections (F-19 E-9) and the cursor, and a preserved drawing buffer there would slow scrubbing (ADR-016). It uses a 1200 × 630 CSS-pixel container outside the viewport, styled by a class (no inline style), with `pixelRatio` 1, not interactive, no attribution control, `fadeDuration` 0, and a preserved drawing buffer (canvas context attributes) on this map only. The style is liberty through `mergeStyle` (Mercator, no terrain or sky) whatever the theme, with no `wx-` sources or layers and no GeoJSON. The camera is exactly the server's frame. Capture after the first `idle` with `canvas.toBlob('image/png')`. Any `error` event (style, tile, glyph, sprite) abandons the job. `map.remove()` runs on every path | MapLibre fetches every resource with CORS and OpenFreeMap answers `Access-Control-Allow-Origin: *`, so the canvas is not tainted; a `SecurityError` would only be one more fallback. **CSP unchanged** (ADR-032): `connect-src` already allows the tile host, `img-src blob:` covers MapLibre's images, and the PUT is same-origin |
| MC-8 | **When the job runs.** (a) **Publish:** it starts after the pending save's `200`, so the frame reflects the saved hiding, and runs alongside `POST …/publish`. The published page opens once publish has succeeded and the job has ended. One 10 s budget (one abort signal) covers GET, render and PUT. (b) **A save that changed a public-transform setting** (`hide_ends`; later trim and zones) on a published workout: the same job in the background, nothing awaited or shown. The job does nothing when GET returns `current` or no frame. At most one job per page; a newer one aborts the older. Never on the published, viewing or upload page | (a) A link copied from the published page then already has the map card; publish itself never waits (OQ-112, resolved). (b) Turning hiding on replaces the dropped map rather than leaving the plain card for good |
| MC-9 | **Storage:** `private/{id}/preview-map.{key}.png`, written only by `privatedir` (temporary file, `fsync`, `rename`, **only into an existing folder, never creating it**, as F-30 RG-4(a)), removing any other `preview-map.*` in the same lock. Bytes count in the storage total (F-10) and are never refused for storage. `publish` reads the map through a port, as it reads the original. A stale map stays private and unserved until the next PUT or the workout's deletion | Deleted with `private/{id}` by the one delete path (I-4, AC 44): no new delete code. A PUT racing a delete cannot bring the folder back |
| MC-10 | **About** (F-29 template, frontend): one privacy paragraph. A published workout with a route gets a link preview showing the route on an OpenFreeMap map, drawn by the publisher's browser at publish. Anyone who sees the link in a chat sees where the route is, as on the public page. The server never contacts OpenFreeMap. No new attribution entry (F-29's list covers OpenStreetMap and OpenFreeMap) | SH-12, INF-1 |

**What drops or replaces the map** (SH-10; no unpublish exists, SH-2):

| Change | Served card |
| --- | --- |
| Job fails (no WebGL2, tile or style error, 10 s, `toBlob`, PUT refused or lost) | F-23 card; nothing stored |
| Hiding on (later trim, zones) after publish | Same build: remove-first (P-7), new frame, key mismatch → F-23 card at once; MC-8(b) may store a new map |
| Name or description | Map kept, text redrawn in the same build |
| `regenerate` | Same build and rule: kept while the key matches, else the F-23 card |
| Delete, expiry | Gone with `private/{id}` and `public/{public_id}` |

## Verification (critical lane: QA at effort high)

Synthetic files and synthetic basemaps only (ADR-004; no rendered tiles committed).

| What must be proven | How |
| --- | --- |
| No hidden part in the image (AC 2) | Go: with hiding on, the composer receives only public-file positions, and the frame equals `Frame(public file)`. For a file whose ends widen the box, the frames with hiding on and off differ. A map stored for the hiding-off key is not used after hiding is turned on (the served file equals the F-23 card). Front end (ADR-012 fake): the basemap map gets no GeoJSON source and no `wx-` layer, the light style whatever the theme, and exactly the server's camera |
| Key binding (AC 42, 43) | Title change: map kept, new card. Hiding toggled, or a parser seam that moves one position: F-23 card. `regenerate` keeps a matching map and drops a stale one |
| Crafted PNGs (AC 39, 40) | Table, each refused with the previous card and map unchanged: cap + 1 bytes (read stops), empty, JPEG, GIF, text, 1201 × 630, 1200 × 631, a 65535 × 65535 header (no allocation; checked by measuring), truncated IDAT, bad CRC, a zlib stream far larger than the image, trailing data. Accepted: interlaced, 16-bit, paletted, grey, and text, `iTXt` and `eXIf` chunks, whose bytes never appear in the served file. Served PNG chunks are only `IHDR`, `PLTE`, `tRNS`, `IDAT`, `IEND`, and never equal the upload. Go fuzz target over validation and composition (synthetic seeds): no panic, bounded memory. A random-noise basemap → F-23 card (300 KB). Concurrent PUTs: one `503`, no other workout affected |
| Access (AC 41) | No, wrong or malformed token, or a query string: the not-found response, nothing stored. A `{key}` that is not 16 hex characters: not found. Stale key: `409` |
| Fallbacks (AC 37, 38) | Front end: WebGL2 missing, a tile `error`, fake timers past 10 s, a PUT failure → no PUT or an ignored one, and the published page still opens. Indoor and fully hidden: GET says no frame, MapLibre is not loaded |
| Deletion (AC 44) | After delete and expiry no `preview-map.*` remains; a PUT after delete creates no folder |
| No egress, budgets (AC 45) | Publish and PUT in Go with an `http.DefaultTransport` that fails the test on use. CI: the activity entry line is unchanged, and the viewing page never requests the basemap chunk |
| Manual (AC 1, 47) | Operator: route on the roads (alignment) on a few real routes, locally only; card size for a dense city and for countryside; iPhone and Android; WhatsApp, Signal, Slack, Telegram and X; attribution legible |
| Mutants | Each must fail a test: map used without key comparison; frame from the transform's input; PUT that creates `private/{id}`; served bytes = stored or uploaded bytes; `DecodeConfig` check removed; visible map captured |

## Hand-offs

| To | What |
| --- | --- |
| F-30 | `regenerate`'s wiring gives the public build the map port; nothing else changes |
| F-31 | ADR-017's M(N) gains P ≤ 16 MB, one image request (note there). No deploy, network or setting change; `app` still has no route out |
| F-23, ADR-033 | The card gains the map variant (MC-6); notes added in both |
| Requirements | SH-12 and the F-36 scope reworded to MC-1 (OQ-113, done) |

## Open questions

None. OQ-112 and OQ-113 resolved on 9 Nov 2026 as suggested ([decision log](../features/README.md)).
