# QA F-22 Caching, fast removal and security headers - round 1

- **Date:** 2026-10-31
- **Verdict:** ready with conditions. No Blocker or Major. The condition is the browser check, which no automated test covers: AC 4 (CSP blocks inline and cross-origin script, tiles still load), H-5 (blob download and Copy buttons under the CSP), and the network-log part of AC 2 and QA-15-r1-3.
- **Inputs:** `features/F-22-caching-and-security-headers.md`; `architect/F-22-caching-and-security-headers.md` v0.1 (H-1 to H-12); ADR-032, ADR-014, ADR-020 item 7 and 9, ADR-028; F-15 P-5, P-13; F-21 X-6; product spec SH-10; TD Hosting and operations, Security and privacy (Headers), invariants I-6 and I-7; decision log 31 Oct (OQ-88 to OQ-90). Commits 9794638, aa44d9e and 3f5852d (working tree clean at 3f5852d). Critical lane, effort high.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0, 38.4 s. gofmt, `go vet -tags caddycheck`: clean. svelte-check: 0 errors, 0 warnings. Go: all 14 packages ok. vitest: 51 files, 1,331 tests passed. Size: upload 2,272 / 8,000 B gzip, edit and index 56,366 / 60,000 B gzip. `caddy-check`: `ok workout-explorer/cmd/workout-explorer 2.070s` with the pinned v2.11.7. (npm EBADENGINE warning: local node 22 vs `>=24`, pre-existing.) |
| `make corpus` | 55 ok, 0 diff, 0 fail, 0 new, 0 stale |
| `go test -race -tags caddycheck -count=3 -run '^TestCaddy$' ./cmd/workout-explorer` | ok, 15.7 s (three runs, no flakes in the SH-10 polling) |
| `go test -race -count=1 ./cmd/workout-explorer` | ok, 27.0 s |
| Mutants (scratch copy of `deploy/Caddyfile`, `TestCaddy` per mutant) | All 7 from the design's verification table were killed: `'unsafe-inline'`; the security set missing on the worker asset; the set missing on 404s (`handle_errors`); `public` on the data file; `immutable` on `index.html`; Caddy's `Cache-Control` replacing Go's on `/api/*`; a catch-all root at `{data}` (it serves `private/…/original.fit`); the 2.10.2 binary (Fedora's `/usr/bin/caddy`: `data br: status 206, want 200`). 8 of my own were killed too: `defer` removed (duplicate `nosniff`), `X-Robots-Tag noindex` only, `precompressed` removed, `no-cache` on owner pages, `max-age=120` on the page, HSTS removed, `browse` on `/w/{id}/`, and a corrected re-run of the worker mutant. 3 survived, all equivalent: `/w/` file root at `{data}` with `/public/` in the rewrite (it serves the same files); `browse` on the `@file` server (the regex never routes a folder there); `hide .*` removed (see QA-22-r1-1) |
| Probes | See the findings, plus: API error paths, other methods, Range, gzip-only client, path variants (Coverage) |

## Findings

### Minor

- **ID:** QA-22-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/Caddyfile:88` (`hide .*`)
- **Relates to:** F-22 scope ("serves `public/`"), F-15 P-13, F-31 hand-off (data path)
- **Evidence:** Caddy compares a `hide` pattern that has no `/` with every component of the absolute file path, not just the requested name. I ran the pinned Caddy with `WX_DATA` under a folder named `.dotdata`. `/w/{id}` returned 200 (the `@view` handler has no `hide`), but `/w/{id}/data.{hash}.bin` and `/w/{id}/index.html` both returned 404. The same files under a plain folder returned 200. The mutant that removes `hide .*` survives: the `@file` regex already refuses a name that starts with a dot, so the line protects nothing and only adds this trap.
- **Consequence:** if F-31 puts the data directory under any dot folder (for example `~/.local/share/…`), every published page loads but fetches no data file: empty charts and map, and the API still reports success. Fix: delete `hide .*`, since the regex already refuses dot names. Keep the `dot file` assertion in `TestCaddy`.

- **ID:** QA-22-r1-2
- **Severity / kind:** Minor / defect (documents)
- **Route:** architect (TD text through the requirements engineer)
- **Where:** `spec/Workout Explorer — Technical Design.md:270` and `:423`; `architect/ADR-032-response-headers-in-caddy.md:3`; `architect/F-22-caching-and-security-headers.md:4`; `architect/INDEX.md` (F-22 row); `architect/F-20-publish-share-delete-controls.md` C-1 (`deploy/Caddyfile:68`); `deploy/Caddyfile:2`
- **Relates to:** OQ-89, H-8, "one home per fact"
- **Evidence:** TD:270 still calls the Caddyfile "minimal: published pages, generic page, /api proxy (F-15 P-13)". TD:423 still says Caddy adds `X-Robots-Tag` "to every response under `/w/` and `public/`". Four lines later, TD:427 and the decision log (OQ-89) say every response, and that is what the code does. ADR-032 is still `proposed`, the F-22 design and its INDEX row are still `draft` v0.1, F-20 C-1 cites a Caddyfile line number that has moved (the owner handle is now near line 100), and the Caddyfile header still says "the unit is F-31's", although F-31 now uses Docker. H-7 lists `draft.bin` as compressed on the fly but not the original download, which the code also compresses (accepted below, but not written down).
- **Consequence:** the TD contradicts itself on the scope of `X-Robots-Tag`, and a reader of TD:270 would think headers and caching are still open.

- **ID:** QA-22-r1-3
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/fetch-caddy.sh:9`, `:14`
- **Relates to:** H-7, H-12 (binary downloaded with a checksum), OQ-70
- **Evidence:** I put a fake `curl` on `PATH` that writes a tampered tarball. The script stopped at `sha512sum -c` with `FAILED` and exit 1, so the tarball check works. The no-op branch is weaker: a 2-line shell script at the destination that prints `v2.11.7 h1:tampered` was accepted with exit 0 and never hashed. The non-amd64 message ("put Caddy v… at $dest yourself") invites a hand-placed binary that is then treated as the pin.
- **Consequence:** CI is unaffected, because a fresh checkout always downloads and verifies. On a developer machine, any binary at `build/tools/caddy-2.11.7/caddy` that reports the version passes `make caddy-check` as "the pinned Caddy". Fix (small lane): also pin the SHA-512 of the extracted `caddy` binary in `caddy.env`, and check it in the no-op branch.

- **ID:** QA-22-r1-4
- **Severity / kind:** Minor / risk (test)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/caddy_test.go:219`, `:394`
- **Relates to:** AC 2, H-12
- **Evidence:** The scan skips the body of every chunk whose name starts with `start`. Today that is exactly the owner chunk `start.svelte-*.js`, which `main` imports only when `#wx-edit` is present, and the test asserts the published page has no `wx-edit`. I replayed the scan on `build/web`: it reaches main, map-view, the map worker, recorder and the CSS files, so the coverage is right today. However, the exclusion depends on a file name Vite derives from a Svelte file name, and the API check is a search for the substring `/api/`.
- **Consequence:** if the owner chunk is renamed, or a view chunk happens to start with `start`, coverage changes silently. A URL built without the literal `/api/` is not seen. Fix: take the exclusion from `build/web/.vite/manifest.json` (the dynamic import of the edit entry by its source path), not from a name prefix. The browser network log stays the real check (condition of this round).

## Previous findings

Round 1; no earlier F-22 findings.
- **QA-15-r1-3** (Caddy answered `.br` with `206`): fixed by the 2.11.7 pin. `TestCaddy` asserts `200` with no `Content-Range`, and the 2.10.2 mutant fails it. Still open: the browser half (the preload is reused, the data file is fetched once).

## Engineer's deviations: judgement

| Deviation | Judgement |
| --- | --- |
| `X-Robots-Tag: noindex, nofollow, noarchive` | Not a deviation. It is the string in feature AC 7, H-8 and the decision log (OQ-89). Accepted |
| Check runs `serve`'s setup in-process | `newApp` plus `runServer` on a real TCP port is `serve`'s wiring (`main.go:109`, `:142`). Only settings from the environment, `IdleTimeout` and `ErrorLog` differ, and none of them affects headers or removal. It is needed for the expiry timer seam (100 ms) and for stopping Go in AC 2. Accepted |
| Missing `/w/{id}/{name}` → empty `404`, not the generic page | AC 3 allows "generic page or 404". P-13 requires the generic page only for `/w/{id}`, and a data URL is never a navigation. The empty 404 carries the full H-2 set and `no-cache` (tested). Accepted |
| `application/octet-stream` compressed on the fly, including the original download | Nothing breaks: the front end reads no `Content-Length` on downloads (only upload progress, which is request-side). There is no BREACH-type exposure: no response body repeats a stable secret next to attacker input, and the upload response's token is new per request. The cost is CPU for at most 20 MB once per click. A gzip-only client gets the data file gzip-encoded from the identity `.bin` (probe: 200, `gzip`, `private` …). Accepted. Record it in H-7 (QA-22-r1-2) |
| AC 2 "no `/api` request" via a static scan with a name-prefix shortcut | Acceptable as the automated proxy, with the shortcut comment present. See QA-22-r1-4. Not a substitute for the network log |
| `caddy-check` downloads the pin itself, linux/amd64 only | Matches OQ-90 (fail, not skip); CI is `ubuntu-24.04` amd64, and the shortcut comment is present. See QA-22-r1-3 for the no-op branch. Accepted |
| HSTS over plain HTTP locally | Correct: RFC 6797 §8.1 says browsers ignore STS received over insecure transport, and H-2 already says so. Production HTTPS is F-31's. Accepted |

## Questions

None that block. For F-31 (not F-22): keep the data directory path free of dot components, or fix QA-22-r1-1 first.

## Coverage

**Acceptance criteria**

| AC | Code | Test / evidence | Status |
| --- | --- | --- | --- |
| 1 Data `private`, immutable, 1 year; `index.html` `no-cache`; `.br` with `Content-Encoding: br` | `deploy/Caddyfile` `@file`, `@view` | `TestCaddy/routes`, `/data file` (exact strings, `Vary`, no `Content-Range`, bytes equal to the `.br` on disk) | traced, passing |
| 2 Published page works with Go stopped, no `/api/` request | static handlers only on the view path | `TestCaddy/Go stopped`: page, data and every reached chunk are 200 with Go down; `/api/health` is 502 with the full set | automated part passing; **network log: hand check** |
| 3 Old page and data gone ≤ 60 s after delete, expiry, hiding on | disk-per-request serving, `no-cache` | `TestCaddy/SH-10`: all three; expiry with a 100 ms timer, the production bound is F-21 X-6 (H-11) | traced, passing |
| 4 CSP blocks inline and cross-origin script; tiles load | `(security)` snippet | exact-string assertion and mutant only; static scan below | **not verified: hand check** |
| 5 No `Set-Cookie` | none set | `check()` on every response it inspects | traced, passing |
| 6 `Referrer-Policy` on page, edit, published, JS asset, data, `/api/*` | `(security)`, `handle_errors` | `TestCaddy/routes`, `/data file`; worker asset included | traced, passing |
| 7 `X-Robots-Tag` on every response class | `(security)` | `TestCaddy` (upload, page, data, edit, published, assets, `/api/*`, 404s); About does not exist yet (F-29) and falls into the tested 404 | traced, passing |
| 8 `original.fit`, `draft.bin` not reachable | `@file` regex, roots only `{data}/public` and the web build | `TestCaddy/never reachable`: F-12 AC 6 list plus database, lock and traversal variants, GET and HEAD; catch-all-root mutant killed | traced, passing |

**CSP, static evidence (no browser here).** The four built HTML files have no inline script, no `style=` attribute, no `on*=` handler and no `<base>`. Go's page package adds none. The JS bundles create no `<style>`, call no `setAttribute('style', …)` and have no `style="` in Svelte templates. Svelte writes `style.cssText` (CSSOM, not governed by `style-src`). MapLibre gets a same-origin `/assets/maplibre-gl-worker-*.js` through `setWorkerUrl`, so its `blob:` worker path (cross-origin only) is not taken and `worker-src 'self'` is enough. The worker's `globalThis.eval` sits only in the RTL-plugin loader, which the app never calls. The only external host the code requests is `tiles.openfreemap.org` (the `liberty` and `dark` styles); the other URLs in the bundles are message or documentation strings. I did not verify offline that the remote style JSON references only that host (glyphs, sprites, raster).

**Hand check (the condition of this round), through Caddy, not `protoserve`.** Chrome, Firefox and Safari, plus a phone:
- No CSP violation in the console on the activity page (map light and dark, splitter, range, units), the edit page (draft and published), the published page, upload and the generic page.
- A scratch page with one inline and one cross-origin script shows both blocked.
- Tiles, glyphs and sprites load. The worker's tile requests carry no `Referer`, and the worker response carries the CSP.
- Download original and every Copy button work.
- With Go stopped, the published page makes no `/api/` request.
- The data file is fetched once per load, and a reload revalidates the page (`304`).

**Also checked and correct.** No cache module; `admin off`. `defer` replaces Go's `nosniff` instead of doubling it. Caddy logs nothing for a 502 with an IP, user agent or `public_id` at the default level (I-7). `handle_errors` resets `Cache-Control` after a `@hashed` miss. `/upload.html`, `/edit.html`, `/generic.html`, `/index.html` and `/.vite/manifest.json` all return 404. `/W/` and `//w/` behave. Range on the data file returns a correct 206 (only the full request has to be 200). POST and OPTIONS on static routes return 405 with the full set. Go's own mux 404 and 405 under `/api/` (unrouted paths) carry no `Cache-Control`. That is harmless, because there are no validators and so no heuristic caching, and it predates F-22. `fetch-caddy.sh` checks the tarball before extracting.

**Not run:** any browser; the official checksum list for v2.11.7 (no external call made: the pinned SHA-512 is trusted as recorded); F-31 deployment (TLS, mounts).
