# ADR-032 Response headers set in Caddy: one site-wide policy, no inline sources, workout files cached only by the viewer's browser

- **Status:** accepted (31 Oct 2026), with the F-22 design v1.0
- **Date:** 31 Oct 2026
- **Features:** F-22 (introduces it); F-23, F-24 (file names), F-25, F-26, F-28, F-29 (pages under the policy), F-31 (Caddy image)
- **Supersedes:** none. Extends [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md) item 6 (map sources) to the whole policy
- **Depends on:** [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md) item 5 (no inline CSS or script), [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md), ADR-005 item 9 and OQ-55 (kept workouts on old bundles)
- **Requirements:** SH-10, NFR Privacy, TD invariant I-6, TD Security and privacy (Headers)

## Context

Caddy serves `public/` and the web build as static files, also while Go is down. Static files cannot carry a per-response CSP nonce, and a `<meta>` CSP can only tighten a header policy, never loosen it. Kept workouts (OQ-55) may serve an older page next to the current bundle, so one site-wide header must fit pages built by different template versions. Shared caches between viewer and server (a TLS-intercepting proxy, a future CDN) cannot be purged by us.

## Decision

1. **Caddy owns the headers**, for every response of the site, including errors and `/api/*`; Go sets only its own API `Cache-Control` and content headers, which Caddy leaves alone. No cache in Caddy.
2. **One CSP for every page**, the string in F-22 H-2, with no `'unsafe-inline'`, `'unsafe-eval'` or nonce.
3. **Inline script only by hash, and only for byte-constant text**: no template value inside, so every page of one template version carries the same bytes. The policy lists one `sha256-` per distinct script text in use. A changed script adds a hash; the old hash leaves the policy only when no served page carries the old text (in practice, together with the old bundle's removal, OQ-55). No inline styles at all (`'unsafe-hashes'` is not used).
4. **Cache by file name.** Content-hashed workout files: `private, max-age=31536000, immutable`. Bundle assets (no workout data): `public, max-age=31536000, immutable`. Pages: `no-cache`. Owner pages: `no-store`. A name not recognised as content-hashed is `no-cache`.

## Consequences

- A loosened policy or a missing header fails one automated check (F-22 H-12), not a review.
- Every front-end change must run clean under the CSP; violations show only through Caddy, not under Vite dev or `protoserve`.
- `private` means no shared cache ever holds workout content, so SH-10 rests on our disk alone. **Cost:** a CDN in front of `public/` (TD scaling path) cannot cache the data files without a revision.

## Reversibility

Cheap: headers change with the Caddyfile on the next deploy. HSTS (F-22 H-2) is the one part browsers keep for a year.

## Would choose differently if

- A CDN goes in front of `public/`: then `public` with purging on every remove and `index.html` rewrite (ADR-028), in a revision.
- SH-7 (iframe embed, phase 2) ships: then `frame-ancestors` for `/w/` only, in a revision.
- A library we need sets inline styles through markup or `setAttribute`: refuse the library or patch it, before considering `'unsafe-inline'` for styles.
