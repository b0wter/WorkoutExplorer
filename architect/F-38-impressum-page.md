# F-38 Impressum page: architecture

- **Version:** 0.1
- **Status:** accepted, built (9 Nov 2026). Normal lane. User decisions of 9 Nov 2026 built in: OQ-120 to OQ-124 as suggested; the text loads only after a click on "Show Impressum"; the text address is rate-limited per address and answers only its own page
- **Feature:** [`features/F-38-impressum-page.md`](../features/F-38-impressum-page.md); "AC n" is the n-th acceptance criterion there
- **Requirements:** INF-7, INF-1, DS-3, DS-4, NFR Privacy; TD I-1, I-7, Configuration, Hosting, Security and privacy (Indexing)
- **Builds on:** [F-29](F-29-about-page-and-attributions.md) AB-1, AB-4, AB-8; [F-22](F-22-caching-and-security-headers.md) H-2, H-6, H-8, H-10, H-12; [F-23](F-23-preview-card-and-indexing.md) PC-11; [F-15](F-15-publish-pipeline-and-static-page.md) P-10, P-12, P-13; [F-30](F-30-regenerate-command.md); [F-37](F-37-report-notification-links.md) DK-9, DK-11; [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) items 6 to 8, 11; [ADR-024](ADR-024-upload-page-plain-typescript-entry.md); [ADR-025](ADR-025-owner-api-access-and-not-found-response.md) item 7; [ADR-032](ADR-032-response-headers-in-caddy.md). New: [ADR-043](ADR-043-page-shells-filled-at-start.md)
- **Owner:** both, backend first (setting, shells, text address, Caddy, runbook), then frontend (`impressum.html`, footer markers, edit-page footers)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| IM-1 | **Setting `WORKOUT_EXPLORER_IMPRESSUM_FILE`**, absolute path, empty = unset. Content checked and rendered by the operator file's parser, `about.ParseOperator`, unchanged (AB-8: three rules, every character escaped, 16 KiB, UTF-8, no control characters but tab and line ends, one non-blank line). **Unlike AB-8, set but missing, a directory or unreadable → exit 2 naming the setting**, never a placeholder (OQ-122). Read once by `serve` and by `regenerate` with the same check, so the two never disagree. Host file next to the operator file, `/srv/workout-explorer/operator/impressum.txt`, in the existing read-only mount (F-29 hand-off to F-31). Repository: only `deploy/impressum.example.txt`, headings with bracketed prompts, no real data and no claim about what the law requires | One parser, one format the hoster already knows. A placeholder Impressum on a live site would be wrong |
| IM-2 | **Page `/impressum`:** frontend template `src/web/impressum.html`, a Vite entry in plain TypeScript (ADR-024 pattern: no Svelte, no chunk shared with the activity entry; ceiling 2,000 bytes gzip under ADR-002). Plain theme CSS, system light or dark, no theme script, no drop-down (as About, AB-1). Metas `robots noindex, nofollow, noarchive`, `referrer no-referrer`, `color-scheme`; no Open Graph or Twitter tags. Content: the heading, a "Show Impressum" button (`hidden` in the HTML, shown by the script), an empty container, and `<noscript>` "JavaScript is needed to show the Impressum." **Never any Impressum text.** Served by Caddy from `{data}/site/impressum.html`, which `serve` writes at start only when IM-1 is set and removes otherwise (ADR-043), so unset gives Caddy's `404` (AC 1). `Cache-Control: no-cache` (as `/about`); `/impressum.html` stays `404` | AC 3, AC 4, AC 11. No JavaScript, no button that does nothing |
| IM-3 | **Loading:** only on the button's click, one same-origin `fetch` of `/api/impressum` with the IM-5 header. `200` → the body goes into the container and the button goes away; any other status or a network error → "The Impressum could not be loaded. Reload the page." (OQ-124). No load before the click, no retry loop | User decision 9 Nov 2026. A crawler that renders the page but clicks nothing sees no text |
| IM-4 | **Text address `GET /api/impressum`, answered by Go** from bytes rendered once at start. Body: exactly `ParseOperator`'s output (escaped `<h2>`, `<p>`, `<ul>`), `text/html; charset=utf-8`, `no-store`. The route is registered only when IM-1 is set; unset → Go's unknown-route `404`. Caddy unchanged (`/api/*` proxy; H-2 and `X-Robots-Tag` from the site-wide set) | Caddy has no rate limit without a plugin. **HTML fragment, not JSON plus a TypeScript parser:** one parser, the same escaped output the About page already serves; the CSP blocks script and inline handlers on top. Stricter caching than `/about` |
| IM-5 | **"Own page only" check, first, before the rate limit:** the request carries `Wx-Impressum: 1` exactly, and `Sec-Fetch-Site`, when present, is `same-origin`. Otherwise `http.NotFound`, byte-identical to an unregistered route. Not `Origin`: browsers send none on a same-origin `GET`. **Honest scope:** it stops HTML-only scrapers (they never see the address), naive scripts that `GET` the address from `robots.txt` or the bundle, and other sites reading it through a visitor's browser (a custom header needs a CORS preflight, which Go never answers, ADR-025 item 7). It does **not** stop anyone who sends the two headers (`curl -H`), headless browsers that click, or people copying the text. A speed bump, not access control | User decision 9 Nov 2026. A refusal that looks like "no Impressum" tells a naive script nothing. Browsers without `Sec-Fetch-*` (before Safari 16.4) still pass |
| IM-6 | **Rate limit:** an own instance of the ADR-021 limiter type (client rule items 6 to 8, IPv4 address or IPv6 /64, hashed keys in memory only), **30 per rolling hour per key**, a constant, counted only for requests that pass IM-5; table cap **1,000 keys** (≤ 0.3 MB, F-31 DP-8 unchanged), fail closed. Refusal `429` with `Retry-After`, as DK-9. **No log line** for any request or refusal; nothing stored (I-7) | A reader needs one request per visit. **Cost:** 1,000 addresses within an hour hide the text from new readers for up to an hour; accepted for a friends' site |
| IM-7 | **Footer link: one marker `<!--wx:impressum-->`, one constant `<a href="/impressum">Impressum</a>`**, defined once on each side. Set → the marker becomes the link; unset → nothing. Every page source with a footer carries the marker once; a source without it makes the loader exit 1 (as P-10). Where it is filled: table below. Caddy's bodyless answers (catch-all `404`, `handle_errors`, `/about` `502`) have no footer, as today | One mechanism for every page type; no view-model or contract change (ADR-026) |
| IM-8 | **`robots.txt`:** Caddy answers `/robots.txt` itself (inline `respond`, always, OQ-121), `text/plain; charset=utf-8`, `no-cache`, exactly: `User-agent: *` / `Disallow: /impressum` / `Disallow: /api/impressum`. Nothing else; `/w/` is never disallowed (F-23 PC-11's reasons stand). Naming the address is harmless: the page's script names it too, and IM-5 does not rely on secrecy | One home, the Caddyfile; no build step, no Go route |
| IM-9 | **Headers and privacy:** H-2's set, CSP included, unchanged; `X-Robots-Tag: noindex, nofollow, noarchive` on both addresses already (H-8). No cookie, no third party, no per-viewer log, nothing per workout | AC 7, AC 11 |

**Where the footer link is filled (IM-7)**

| Page | Filled by | Follows a setting change after |
| --- | --- | --- |
| Upload `/`, generic page (`/w/` misses), `/report-delete`, `/impressum`, the edit shell for `/edit/*` and `/published/*` | `serve` at start, written to `{data}/site/` (ADR-043) | restart of `app` |
| About | `serve` at start, in memory (AB-4) | restart of `app` |
| Published workout page `/w/{id}` | The `index.html` source, filled before it is parsed as the template, by `serve` and `regenerate` (P-10) | the next `regenerate` (OQ-120); an owner change rebuilds its own page at once |
| Footers the edit page draws in the browser (page footer, published view, not-found view) | The owner chunk copies the link from its own shell (frontend chooses the element); no extra request | restart of `app` |

## Verification focus

- AC 3: a marker string from the file in no response but `/api/impressum`, over every page type through Caddy.
- IM-5, IM-6: without the header, with a cross-site `Sec-Fetch-Site`, unset: the same bytes as an unknown `/api/` path; the 31st request in an hour is `429`.
- AC 9, AC 10: every IM-1 failure exits 2 in both `serve` and `regenerate` before listening or touching the data directory.
- AC 8: every page type with and without the setting, after restart and `regenerate`, through `make caddy-check`.

## Hand-offs

| To | What |
| --- | --- |
| F-31 (backend engineer) | `{data}/site/` created like `public/` (same owner and mode) and mounted read-only into Caddy in `compose.yaml` and `compose.build.yaml`. `example.env`: commented `WORKOUT_EXPLORER_IMPRESSUM_FILE` line. Runbook: write the file from the example, restart `app`, run `regenerate`; `/impressum` is hard to crawl, not secret. `make caddy-check`: new routes, `robots.txt` body, shell routes from `site/` |
| Requirements | F-23 AC "`/robots.txt` answers `404`" and its scope line are replaced by IM-8 |

## Open questions

None. The legal adequacy of a click-to-load, JavaScript-only Impressum is the hoster's call (feature Out of scope).
