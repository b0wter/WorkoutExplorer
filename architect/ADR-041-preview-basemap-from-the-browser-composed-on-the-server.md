# ADR-041 Preview image with map context: the browser sends a basemap for a server-made frame, Go composes the card

- **Status:** accepted (9 Nov 2026), with the F-36 design v0.1
- **Date:** 9 Nov 2026
- **Features:** F-36 (introduces it); F-23 and F-30 (the build that uses it)
- **Supersedes:** none. Extends [ADR-033](ADR-033-preview-image-rendered-with-x-image.md) (a second card variant) and [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md) item 2 (the build reads one more private input)
- **Depends on:** ADR-013, ADR-014, ADR-022, ADR-025, ADR-028, ADR-032
- **Requirements:** SH-12, SH-5, SH-4, SH-10, NFR Privacy, NFR Security; TD invariants I-3, I-5, I-6

## Context

1. SH-12 wants map tiles behind the route in the link preview. The server may not contact the tile host (TD Principles), so the sharer's browser must render the tiles.
2. Hidden ends must never appear in the image, also after later setting changes, and `regenerate` must tell whether a stored browser image still fits the current public route. The server cannot check what arbitrary pixels show.
3. The browser-made part enters through the network and is untrusted input.

## Decision

1. **Split.** The browser renders only the basemap: OpenFreeMap tiles, no route, no text. Go draws the route from the decoded public data file, plus the icon, name, values and attribution, over it (F-36 MC-1, MC-6).
2. **The server dictates the frame.** A pure rule in `internal/preview` computes the camera (centre, zoom; Web Mercator as MapLibre) from the public file's positions and rounds it. Its key is a hash of the rounded frame and the layout version. The browser renders exactly that camera and sends the PNG under that key.
3. **Validity is a key match in every build.** A stored basemap is used only when its key equals the key of the frame the build computes now; otherwise the build draws the server-made card (ADR-033). No row field.
4. **Stored privately, served only as our composite.** `private/{id}/preview-map.{key}.png` holds our re-encoding of the decoded pixels. The served `preview.{hash}.png` is always a new encoding of the composite, never the received bytes.
5. **Separate owner request.** `GET` and `PUT …/preview-map` (F-36 MC-4), not part of publish. A PUT on a published workout triggers the in-place build.

## Consequences

- Privacy rests on Go code and Go tests: the route pixels and the frame come only from the public file. The browser's pixels can only be wrong about the map, which the owner controls anyway (the spec accepts this).
- Rename and description changes keep the basemap. Hiding, trim, zones and parser changes drop it in the same build.
- One more frame rule to keep aligned with MapLibre's camera: a drift shows as a route beside the roads, not as a leak.
- A publish costs up to three parses (publish, GET, PUT's build) and one extra WebGL context in the sharer's browser.
- Phase 2 route colouring or markers in the image would be drawn by Go, not the browser.

## Alternatives rejected

- **Browser draws the whole card:** the server cannot prove the route excludes hidden ends, SH-5's rules and fonts would be duplicated in TypeScript, and every rename would need a new image.
- **Capture the visible map:** themed, wrong size, shows the to-be-hidden sections (F-19 E-9) and the cursor; a preserved drawing buffer there would cost scrubbing.
- **Browser frames, server checks with a tolerance:** two frame rules across languages, and a stored frame that is "close enough" may still be wider than the public route.
- **Image inside the publish request:** publish would wait for the render and take a multipart body.

## Reversibility

Cheap. Stored maps are private and live at most 30 days; cards are rebuilt on every deploy. Both endpoints are used only by our own client.

## Would choose differently if

- More than one in five of the operator's real-route checks falls back for size (300 KB): first drop POI icons and labels from the basemap style, then ask the user about JPEG (the spec fixes PNG).
- MapLibre's camera stops matching the Go projection (a MapLibre major version): pin the projection test values from MapLibre's own `project()` in a browser test.
