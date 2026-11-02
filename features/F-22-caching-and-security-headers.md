# F-22 Caching, fast removal and security headers

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** SH-10, NFR Privacy (no advertising or tracking cookies), NFR Security (originals never public); Technical design: Hosting and operations (TLS and static files), Security and privacy (Headers)
- **Depends on:** F-16, F-21

## Goal
Caddy serves viewers fast from `public/`, never keeps removed content longer than 60 s, and sends strict security headers.

## Scope
- Caddy config in the repository: serves `public/` for `/w/{public_id}`, proxies `/api/*` to Go, automatic TLS, brotli and zstd, pre-compressed `.br` files.
- Content-hashed files `immutable` for a year; `index.html` `no-cache`.
- Generic page fallback for missing workout folders (F-15).
- Strict Content Security Policy: scripts only from the site, tiles and fonts only from the tile host.
- `Referrer-Policy: no-referrer` on every response of the site, including JavaScript assets (ADR-014 item 3).
- No cookies set by any response.
- `X-Robots-Tag: noindex, nofollow, noarchive` on every response of the site (decision log 31 Oct 2026, OQ-89).
- Automated check for SH-10: after delete, expiry and turning on hiding, the old `index.html` and old data files stop being served within 60 s.

## Out of scope
- Robots meta and `robots.txt` (F-23). CDN (scaling path, not MVP).

## Acceptance criteria
- [ ] Response headers: `data.{hash}.bin` has `Cache-Control: private`, immutable, one year; `index.html` has `no-cache`; `.br` served with `Content-Encoding: br` to browsers that accept it.
- [ ] No request when viewing a published page reaches the Go process (checked with the Go process stopped: page, data file and assets still load, and the page makes no `/api/` request).
- [ ] After each of delete, expiry and hiding turned on, a request to the old page and old data URL returns the generic page or 404 within 60 s.
- [ ] CSP blocks an inline test script and a script from another origin; tiles from OpenFreeMap still load.
- [ ] No `Set-Cookie` header on any page or API response.
- [ ] `Referrer-Policy: no-referrer` is present on a workout page, the edit page, the published page (AC-6), a JS asset, `data.{hash}.bin` and an `/api/*` response.
- [ ] Every response (upload page, About, workout page and data file, edit and published pages, assets, `/api/*`, `404`) carries `X-Robots-Tag: noindex, nofollow, noarchive`.
- [ ] `original.fit` and `draft.bin` are not reachable through Caddy under any path.
