# QA F-36 Preview image with map context - round 1

- **Date:** 2026-11-09
- **Verdict:** not ready. One Major (QA-36-r1-1): `GET …/preview-map` has no concurrency limit and reads the whole original into memory before it waits for a parse slot. Anyone who uploads one file can exhaust the app's memory or block the two parse slots for every user. I found no privacy defect: no race order I traced or tested lets a hidden-end position into a served card.
- **Inputs:** `features/F-36-preview-image-with-map-context.md` (13 ACs); SH-12, SH-5, SH-4, SH-10, SH-11 in the product spec; `architect/F-36-preview-image-with-map-context.md` v0.1 (MC-1 to MC-10); ADR-041; the F-36 notes in ADR-013, ADR-017, ADR-033 and F-23. Commits f0931b9 (backend), 0ad8540 (wording), f438b8f (frontend, HEAD). The working tree was clean.

## Build and test results
- `make ci`: exit 0 in 2 min 57 s. Every Go package passed. Vitest: 61 files, 1,578 tests passed. The Caddy and Docker checks passed.
- Size: `entry edit.html: 57746 / 60000 PASS`, `entry upload.html: 3761 / 8000 PASS`. I rebuilt 0ad8540's web bundle in the scratchpad for comparison. `main-*.js` is 53,624 B gzip before and after, so the viewing page is unchanged (AC 11). The basemap chunk is 621 B gzip, lazy, and reached only from `start.svelte`. `main` does not reference it.
- `go test -race` on the F-36 tests in lifecycle, preview, httpapi, publish, privatedir and cmd: all passed.
- `go test -fuzz FuzzCheckMap -fuzztime 60s`: 84,519 execs, no failure.
- Go mutants (scratch copy): 10 of 12 killed. These were killed: map used without a key match, PUT without the stale-key check, StoreMap creating `private/{id}`, map bytes not counted, trailing data accepted, no single-flight slot, frame computed from the transform's input, served bytes equal to the stored map, other maps not removed, route not drawn. Two survived: removing the re-lookup under the lock (QA-36-r1-2) and not drawing the attribution (QA-36-r1-3).
- Frontend mutants: 5 of 5 killed. These were: `await job` removed, the job also running for drafts, `map.remove()` removed, the dark style, and no 10 s timer.
- Probes: 50 concurrent `Area.Frame` calls waiting in Parse add 65.3 MB of heap with `ride-12h.fit` (1.3 MB). The corpusgen all-caps file is 17,360,175 B. `CheckMap`+`EncodeMap` of a 5,958-byte 16-bit Adam7 upload allocate 16.1 MB.
- `TestNoRequestMetadataStored`: 300 isolated runs passed, and it passed in `make ci`. This flake is pre-existing. The test and the upload path were last changed in db91e62 (3 Nov), and no F-36 commit touches them. The test uses `httptest` and binds no port, so the "address already in use" text probably comes from another test in `cmd/workout-explorer`. `main_test.go:24` binds a busy port on purpose. I could not reproduce the failure.

## Findings

### Major
- **ID:** QA-36-r1-1
- **Severity / kind:** Major / defect (security, availability)
- **Route:** backend (architect to confirm the bound in MC-4 and the ADR-017 note)
- **Where:** `src/server/internal/lifecycle/previewmap.go:43-55`; `src/server/internal/publish/publish.go` `front()` (`Originals.Original` then `Parser.Parse`); `src/server/internal/httpapi/owner.go` `previewFrame`
- **Relates to:** MC-4, ADR-017 M(N), NFR Security; SH-12 ("publishing never fails because of the image")
- **Evidence:** The GET handler has no slot, limiter or lock. Each request runs `front()`, which reads the whole stored original with `io.ReadAll` and then waits for one of the two parse slots (`ingest.Parser.run` selects on the slot channel or `ctx.Done()`). The request context ends only when the client disconnects or the 30 s read deadline passes. Probe: 50 waiting calls hold +65.3 MB with a 1.3 MB original. With an original near the parse caps (the all-caps file is 17.4 MB), about 70 requests in flight reach the 1200m Compose cap. Keeping 70 requests in flight needs only about 3 requests per second, renewed every 30 s. Every other owner path that parses runs under `s.mu`, and uploads are capped at 4, so this exposure is new with F-36.
- **Consequence:** Anyone can upload a file, take its edit token and get the app OOM-killed for all users. A smaller flood is enough to keep both parse slots busy. Other users' uploads then wait or time out, and so do the builds behind PATCH, publish and PUT. One possible fix, if the architect agrees: bound GET the way PUT is bounded (a try-acquire that answers 503, which the job already treats as a fallback), and read the original only after a slot is free.

### Minor
- **ID:** QA-36-r1-2
- **Severity / kind:** Minor / risk (test)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/previewmap.go:107-109` (re-lookup under the lock)
- **Relates to:** MC-4, MC-8(a), SH-10
- **Evidence:** All tests still pass with these three lines removed (mutant M5). The line matters in two races. In the first, a PUT whose first lookup saw a draft finds the workout published once it has the lock, so it runs the build. Without the line, the published card stays the server-made one, and `current` stays true, so the job never retries. In the second, a PUT racing a hiding change gets `409`. If the line went, RG-5's `sameRecord` recheck would still block a leak. The interleaving itself is not tested: the end-to-end test's stale PUT already sees hiding at its first lookup.
- **Consequence:** A critical-lane race guard can regress without any test failing.

- **ID:** QA-36-r1-3
- **Severity / kind:** Minor / risk (test)
- **Route:** backend
- **Where:** `src/server/internal/preview/map.go` `RenderOnMap` (`d.DrawString(attribution)`)
- **Relates to:** AC 1, SH-12, INF-1 (licence attribution)
- **Evidence:** All tests still pass when the string is not drawn (mutant M10). The box is drawn, but nothing checks the text in it.
- **Consequence:** The required "© OpenStreetMap, OpenFreeMap" line can disappear without any test failing.

- **ID:** QA-36-r1-4
- **Severity / kind:** Minor / risk
- **Route:** architect
- **Where:** `src/server/internal/lifecycle/previewmap.go:83-90`
- **Relates to:** MC-4
- **Evidence:** The single map slot is shared by all workouts and is taken before the body is read. Reading can last up to the 30 s read deadline.
- **Consequence:** One slow or stalled PUT from any token holder sends every other workout's PUT a 503, and those workouts get the server-made card. Nothing worse happens, but the design does not say so. Record it as accepted, or take the slot after the body has been read.

- **ID:** QA-36-r1-5
- **Severity / kind:** Minor / defect (documentation)
- **Route:** architect
- **Where:** `architect/ADR-017-parse-isolation-and-limits.md:107` ("P ≤ 16 MB")
- **Relates to:** ADR-017 M(N), MC-5
- **Evidence:** `CheckMap`+`EncodeMap` alone allocate 16.1 MB (TotalAlloc, not peak) for a 16-bit interlaced upload. That is before the 4 MB body buffer and before the in-place build decodes the stored map again and quantizes it.
- **Consequence:** The recorded figure is optimistic. The open VPS `memory.peak` row in the RUNBOOK should include a 16-bit interlaced upload.

## Previous findings
None (round 1).

## Questions
None for the architect beyond the Routes above. For the user: see the manual checks under Coverage.

## Coverage
Privacy, traced in code and tests:
- The frame and the route pixels come only from the decoded public file (`front()` → transform → `FrameOf` and `RenderOnMap`).
- Every build compares keys (MC-3).
- These race orders are safe:
  - Hiding turned on while a PUT is in flight: frame recomputed under `s.mu`, then 409.
  - A stale key: 409.
  - `regenerate`: key check, plus RG-5.
  - A map stored before hiding: never used after the key changes. The served card is the F-23 card in the same PATCH.
- The token goes only in the Bearer header (`lib/owner` `request`). Paths carry only the 16-hex key. Logs carry the ID and a fixed reason, never the key (tested). Errors are path-free.

Acceptance criteria:

| AC | Code | Test |
| --- | --- | --- |
| 1 map card stored | `RenderOnMap`, PUT, publish build | `TestPreviewMapEndToEnd`, `TestRenderOnMap`; a real browser render is manual |
| 2 hiding, frame | `FrameOf` on the public file, key match | `TestFrameHidingOn`, `TestMapCardKeyBinding`, `TestRenderOnMapHidden`, end-to-end; frontend: no GeoJSON or `wx-`, light style, server camera |
| 3 fallbacks | `basemap.ts`, `basemap-job.ts` | basemap DOM and job tests, edit DOM test (no WebGL2, map error, PUT failure, 10 s) |
| 4 no positions, fully hidden | `FrameOf` ok=false | `TestPreviewMapNoFrame`, `TestFrame` (indoor). Fully hidden is covered only by composition: `TestHidingFullyHidden` drops the position columns. No direct test |
| 5 refusals | `CheckMap`, cap | `TestCheckMap`, `TestCheckMapBombHeader`, `TestPutPreviewMapRefusals`, `TestPreviewMapAccess`, fuzz |
| 6 never upload bytes | re-encode, paletted composite | `TestCheckMap` (text chunks), end-to-end `SECRET` check, chunk list |
| 7 token | `lookup` first | `TestPreviewMapAccess`, `TestPutPreviewMapRefusals` |
| 8 within 60 s | remove-first build; title redraw | end-to-end (same request) |
| 9 regenerate | key match in the build | `TestRegeneratePreviewMap` |
| 10 delete, expiry | `Files.Remove(private/{id})` | delete: end-to-end. Expiry uses the same `Delete` path, but no test has a map present at expiry |
| 11 no egress, bundle | no network in preview or publish | `forbidEgress`; size comparison above |
| 12 About | `about.html` | `about.test.ts` |
| 13 messengers | - | manual |

Frontend: light style only; the browser draws no route or text of ours; the map is removed on every path (tested). CSP: the Caddyfile already allows the tile host in `connect-src`, `blob:` in `img-src`, `worker-src 'self'` and `style-src 'self'`. The container uses a class, with no inline style (tested). I did not run a real browser.

Not run here (user checks):
1. AC 1 in a real browser: publish a ride, check the card and the alignment of the route with the roads on a few real routes. Local only.
2. AC 13: WhatsApp, Signal, Slack, Telegram and X; check that the attribution is legible.
3. Card size for a dense city route (the 300 KB fallback rate, ADR-041).
4. AC 3 in a browser: WebGL off, tiles blocked in devtools. The published page must open within 10 s and show no CSP violation in the console.
5. Offscreen rendering on iOS Safari and Android Chrome.
6. The MC-6 look review on three synthetic cards.
7. VPS `memory.peak` (QA-36-r1-5).
