# ADR-043 Page shells filled by `serve` at start and served by Caddy from `{data}/site/`

- **Status:** accepted (9 Nov 2026), with the F-38 design v0.1
- **Date:** 9 Nov 2026
- **Features:** F-38 (introduces it); touches F-11 (upload page), F-15 P-12, P-13 (generic page, Caddy), F-19 E-1 (edit page), F-37 DK-11 (confirmation page), F-31 (mounts)
- **Supersedes:** none. Widens [F-22](F-22-caching-and-security-headers.md) H-10 and [ADR-034](ADR-034-compose-bridge-networks-with-mounted-binaries.md) item 5 by one Caddy mount
- **Requirements:** INF-7, INF-1, I-1

## Context

INF-7 puts an "Impressum" footer link on every page, but only when the hoster has set an Impressum. Upload, generic, edit and confirmation pages are static files of the release build, identical for every deployment, served by Caddy straight from the web build. The generic page is in the view path (expired or unknown `/w/` links), so it must not reach Go (I-1) and must work while Go is down.

## Decision

1. **The static pages are shells with comment markers.** `serve` reads them from `-web` once at start, replaces each per-deployment marker with fixed bytes (today only F-38's footer link, or nothing), and writes them to `{data}/site/<name>.html` (temporary file and rename, `0644`) before it listens. A shell without its marker, or a failed write, makes `serve` exit 1. Shells for features that are off (the Impressum page when unset) are removed, so Caddy answers `404`.
2. **One writer:** only the page-shell package writes under `{data}/site/`. Nothing in it is per workout or per request; it is public content.
3. **Caddy serves those routes from `{data}/site/`**, mounted read-only like `public/`; assets stay in the web build. The data directory as a whole is still never mounted into Caddy.
4. **Fill rule as F-29 AB-4:** markers become constant bytes; nothing from a request, nothing interpreted.

## Consequences

- I-1 holds: the generic page is a file and survives Go being down.
- A setting change reaches the shells at the next start of `app`. A deploy's new shells appear when `deploy.sh` restarts `app`; until then the old shells point at old assets, which stay until after `regenerate` (F-31 DP-10).
- On a fresh host the shell routes answer `404` until `serve` has started once.
- Dev servers that serve the raw build (`vite dev`, `protoserve`) show no link: the marker stays a comment.
- One more host directory, created like `public/`, and one more read-only mount in both Compose files.

## Rejected

- **Caddy `templates`:** a second reader of a setting with no validation; AB-4's reasons.
- **Go serves the shells:** the generic page is in the view path (I-1); the upload page would `502` while Go is down.
- **Two build variants picked by a Caddy environment variable:** two settings that must agree.
- **Shells under `public/`:** I-3 and ADR-028's sweep are about workouts only.
- **Link always present:** the user wants no link when unset (OQ-120).

## Would choose differently if

- Shells need per-request content: then Go serves the pages outside the view path and only the generic page stays a file.
