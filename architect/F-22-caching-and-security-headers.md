# F-22 Caching, fast removal and security headers: architecture

- **Version:** 1.0
- **Status:** accepted (31 Oct 2026). Critical lane: CSP and referrer (I-6, the edit token on owner pages), SH-10 removal, originals never public. OQ-88 to OQ-90 resolved 31 Oct 2026 (decision log)
- **Feature:** [`features/F-22-caching-and-security-headers.md`](../features/F-22-caching-and-security-headers.md)
- **Requirements:** SH-10, AC-6, NFR Privacy (no tracking cookies), NFR Security (originals never public); TD invariant I-6, Hosting and operations (TLS and static files), Security and privacy (Headers)
- **Builds on:** [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md) item 6 (map sources), [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) item 7, [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md), [F-15](F-15-publish-pipeline-and-static-page.md) P-5, P-13, [F-21](F-21-lifetime-and-expiry.md) X-6, [F-20](F-20-publish-share-delete-controls.md) C-5, C-6; new: [ADR-032](ADR-032-response-headers-in-caddy.md)
- **Owner:** backend only (`deploy/Caddyfile`, the Caddy check, the pinned Caddy in CI). **No frontend change** unless OQ-88 is accepted (then frontend first, backend adds the hash). **Bundle:** 0 bytes on the activity entry; ADR-011 rev. 12 unchanged

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| H-1 | **Caddy sets every header in this document, on every response of the site**, from one snippet in `deploy/Caddyfile`: pages, assets, data files, the generic page at any status, Caddy's own `404`s and error responses, and `/api/*`. On `/api/*` Caddy adds the H-2 set but never touches Go's `Cache-Control` (`no-store`). Caddy has **no server-side cache** (no cache module); `file_server` reads the disk on every request (ADR-032 item 1) | Caddy serves `public/` while Go is down. One place to read the policy. What is served follows the disk, so SH-10 is decided by what is on disk |
| H-2 | **Security set, every response.** `Content-Security-Policy: default-src 'self'; script-src 'self'; worker-src 'self'; connect-src 'self' https://tiles.openfreemap.org; img-src 'self' data: blob: https://tiles.openfreemap.org; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`. `Referrer-Policy: no-referrer`. `X-Content-Type-Options: nosniff`. `Strict-Transport-Security: max-age=31536000` (no `includeSubDomains`, no `preload`; browsers ignore it over plain HTTP, so local runs are unaffected). **Not sent:** `Permissions-Policy` (H-5), `X-Frame-Options` (`frame-ancestors` covers it), COOP/CORP, a CSP report endpoint (it would store data). The map sources are ADR-014's table; "fonts only from the tile host" (TD) means the glyph files, fetched under `connect-src`; pages use system fonts, so `font-src` falls back to `'self'` | ADR-014 items 3, 6: the worker asset takes CSP and referrer policy from its own response, so "every response" covers it by construction. HSTS is one-way for a year per browser; accepted, Caddy always serves HTTPS in production |
| H-3 | **One policy for every page** (activity, edit, published, upload, generic, About). The upload and About pages allow the tile host they never use | One string, no per-route drift. The edit page shows the map, so owner pages need the map sources anyway |
| H-4 | **No inline sources.** No `'unsafe-inline'`, `'unsafe-eval'` or nonces (static files cannot carry a per-response nonce). Styles only from files or through the CSSOM (`el.style`, `setProperty`); a `style` attribute in markup, a Svelte template or via `setAttribute` is blocked (F-13 D-10, ADR-026 item 5 already require this). An inline script is allowed only by hash, under ADR-032 item 3. **The stored-split jump fix (decision log 29 Oct, F-13 D-10) is not built in F-22** unless OQ-88 says so | Keeps the policy free of exceptions; the hash rule exists so the later fix is one frozen script and one hash |
| H-5 | **Owner features under the CSP** (F-20 C-5, C-6). The original download is a navigation of a temporary `<a download href="blob:…">`, which CSP does not govern: no `blob:` source is added for it. The Clipboard API is gated by secure context and Permissions-Policy, not by CSP; F-22 sends no `Permissions-Policy`, so nothing denies `clipboard-write`. If the H-12 browser check shows a block in one engine, add the narrowest source and record it here | No speculative sources. The fallbacks (select by hand) already exist |
| H-6 | **Caching by file name, not by route** (TD Hosting). See the table below. A name under `/w/` that is not content-hashed (`index.html`, anything unknown) is `no-cache` wherever it is served. "Content-hashed" means P-5's pattern: `.{16 lowercase hex}.` in the name (`data.{hash}.bin`, `.bin.br`, F-23's preview, F-24's export). Everything under `/assets/` is Vite-hashed; an unhashed file there is a defect. ETag and Last-Modified from `file_server` stay, so a revalidated deleted page gets the generic `404` | Unknown names fail toward revalidation. ADR-032 item 4 for `private` |
| H-7 | **Compression.** Data files: the `.br` sidecar (ADR-006) through `precompressed br`, answered **`200`** to a request without `Range` (QA-15-r1-3: Caddy 2.10.2 answers `206`; its internal `Range` trick, fixed after 2.10.2, Caddy PR #7042). The fix is a **pinned Caddy version** that passes the H-12 check; one home for the pin in the repository, shared by the check and F-31's image. Everything else: `encode zstd gzip` on the fly (HTML, JS, CSS, JSON, `draft.bin`, and the original download `original.fit` as `application/octet-stream`). No pre-compressed bundle in the MVP (the build writes none; ADR-011 measures gzip) | A `206` to a preload may not be reused from the HTTP cache (double download). Would choose differently if no pinned release passes: drop `precompressed` for `/w/` and let `encode` compress the `.bin` (the `.br` stays written for later) |
| H-8 | **`X-Robots-Tag: noindex, nofollow, noarchive` on every response of the site** (OQ-89; F-19, F-20 hand-offs; ADR-020 updated). F-23 keeps the robots meta and `robots.txt` | The owner pages already carry the robots meta; the header is one line in a handle F-22 rewrites anyway |
| H-9 | **No cookies.** Neither Caddy nor Go sets one; nothing to configure. The check fails on any `Set-Cookie` | NFR Privacy |
| H-10 | **Never reachable** (F-12 AC 6, Caddy side). Caddy's only roots are `{data}/public` (only inside the `/w/` handles, names limited by P-13's patterns, dot files hidden) and the web build. Constraint for F-31: the web build and the data directory are disjoint, and in Docker the Caddy container mounts **only `{data}/public` (read-only) and the web build**, never the data directory, so `private/`, `tmp/`, the database and `public.lock` are unreachable by construction | A route bug can then expose nothing private |
| H-11 | **SH-10 chain, nothing added by F-22.** Delete, expiry or hiding on → files gone from disk (F-21 X-6: ≤ about 45 s at T = 10 s; hiding on is remove-first before the save returns, P-7) → Caddy reads the disk per request → the page is `no-cache`, data files `private`. Accepted exceptions, already decided: Go down (OQ-87), T above about 25 s (X-6), and a viewer's already open tab, back/forward cache or own HTTP cache (content already shown to that viewer) | Makes the 60 s a chain of recorded bounds, not a Caddy property |
| H-12 | **The check.** Automated, in `make ci`: the pinned real Caddy with `deploy/Caddyfile` in front of a real `serve` over a scratch data directory, synthetic files only. It asserts the H-2 set on every route class (including `404`s and errors), the H-6 table, H-7 (`200`, `Content-Encoding: br`, `Vary: Accept-Encoding`, no `Content-Range`), H-8, H-9, H-10 with the F-12 AC 6 guessed-path list sent through Caddy, and SH-10: after delete, expiry (clock or row seam, the engineer's choice) and hiding on, the old page and old data URL answer the generic page or `404` within 60 s of the trigger. The Caddy binary is downloaded with a checksum and never committed (OQ-70); CI installs it like a toolchain (ADR-001). Without it the check fails, not skips (OQ-90). **"No request reaches Go"** (AC 2) is checked with `serve` stopped: page, data file and assets load through Caddy, and the page makes no `/api/` request. Go has no access log (F-01), so the AC's "Go access logs" wording goes to requirements | Header strings asserted exactly, so a loosened policy fails the build |

**H-6 cache table**

| Response | `Cache-Control` |
| --- | --- |
| `/w/{public_id}`, `/w/{public_id}/index.html`, any non-hashed name under `/w/` | `no-cache` |
| `/w/{public_id}/` content-hashed name (data `.bin`, `.br`, preview, export) | `private, max-age=31536000, immutable` |
| `/assets/*` | `public, max-age=31536000, immutable` |
| `/`, `/about`, the generic page at any status | `no-cache` |
| `/edit/*`, `/published/*` | `no-store` (ADR-020 item 7) |
| `/api/*` | Go's (`no-store`); Caddy does not set it |

## Verification (critical lane: QA at effort high)

| What | Check |
| --- | --- |
| H-1 to H-12, AC 1, 3, 5 to 7 | The H-12 automated check |
| AC 4, H-2, H-4, H-5 | Browser, through Caddy, in Chrome, Firefox and Safari (phone too): no CSP violation in the console on the activity page (map, dark style, splitter, range, units), the edit page (draft and published states), the published page, upload, generic and About (once F-29 lands). Download original and every Copy button work. A scratch page in a scratch data directory with one inline and one cross-origin script shows both blocked. Tiles load; the worker's tile requests carry no `Referer`, and the worker response carries the CSP (ADR-014 item 6) |
| QA-15-r1-3 | Network log: the data file is fetched once per load (the preload is reused); on reload it comes from the HTTP cache and the page revalidates (`304`) |
| Mutants | Each must fail the check: `'unsafe-inline'` added; the CSP or `Referrer-Policy` missing on the worker asset or a `404`; `public` on a data file; `immutable` on `index.html`; Caddy's `Cache-Control` replacing Go's on `/api/*`; a root at `{data}`; the 2.10.2 binary |

## Hand-offs

| To | What |
| --- | --- |
| F-23 | `X-Robots-Tag` is F-22's (OQ-89). The preview name must match P-5's hash pattern to get H-6's caching |
| F-24 | The export name must match P-5's hash pattern |
| F-25, F-26, F-28, F-29 | QA runs the H-12 browser check through Caddy; no `style` attributes, no inline script or style. F-29: `/about` is `no-cache` and gets the H-2 set with no change |
| F-31 | The H-7 pin is the image's Caddy version. H-10's mounts. TLS on, so H-2's HSTS takes effect. Proxy timeouts on `/api/*` at or above both 120 s + 6·T (ADR-021) and the 10 min download deadline (F-12 O-12). Never `log_credentials` (ADR-020 item 9) |
| Requirements | TD Hosting (TLS and static files): workout files `private`, assets `public`, both immutable, link H-6. TD Security and privacy (Headers): link H-2. TD Open technical questions: headers and caching decided here, the rest stays F-31's. F-22 AC 2: "checked with the Go process stopped" instead of "Go access logs". F-22 AC 1: `private` on data files |

## Open questions (product; the user answers by number)

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-88 (resolved 31 Oct: not now) | Fix the layout jump with a remembered map split now, with an inline `<head>` script allowed by its CSP hash (decision log 29 Oct, F-13 D-10)? It makes F-22 frontend plus backend and revises ADR-026 item 5 | **Not now.** Keep the accepted jump; ADR-032 item 3 keeps the later fix to one frozen script and one hash | Only the H-4 exception; F-22 builds without it |
| OQ-89 (resolved 31 Oct: yes, in F-22) | Send `X-Robots-Tag: noindex` on every response, including the upload and About pages, so the site is not listed in search engines at all? | **Yes,** in F-22 as one site-wide line; F-23 keeps the robots meta and `robots.txt`. Fits a friends-only site; cost: the site cannot be found by search | H-8's scope and F-23's header item |
| OQ-90 (resolved 31 Oct: accepted, fail not skip) | The H-12 check needs a downloaded Caddy binary, so a local `make ci` fails without it. Accept? | **Yes,** fail, not skip (tests stay strict); one download, the same as OQ-70. Alternative: run the check only in CI and QA | H-12's `make ci` wiring only |
