# F-31 Server, deploy and monitoring

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** NFR Privacy (EU hosting, no backups), NFR Operations (incl. 2 GB RAM and memory cap), NFR Security (uploads at once); Technical design: Hosting and operations
- **Depends on:** F-22

## Goal
The app runs on one EU VPS, deploys with one script, and restarts itself after a crash or reboot.

## Scope
- One small VPS in an EU data centre with at least 2 GB RAM, Debian (OQ-51). The Go server and Caddy run in one Docker Compose setup, started and stopped together, kept running by Docker restart policies (no systemd units). The Caddy version comes from `deploy/caddy.env` (decision log 31 Oct 2026).
- The Go container's memory is capped by a Compose memory limit, set from the measured worst case with 4 uploads processed at once (QA measured 620–861 MB in F-09) plus headroom; the concurrent-upload default stays 4.
- Server settings: storage cap, cap on uploads processed at once (default 4, F-09), parse time limit (default 10 s, F-09), uploads per hour per IPv4 address and per IPv6 /64 prefix (`WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4` / `_IPV6`, default 60 each, range 1–240; decision log 1 Nov 2026), Caddy's trusted proxy address (the operator's reverse proxy), base URL, data paths, all in one env file read by Compose (F-30 runs `regenerate` with the same file). The site name comes from that file and lives only in `WORKOUT_EXPLORER_BASE_URL` (required by the Go server since F-23, used for `og:image`); Caddy has no site address and answers on its port for any host (F-31 design DP-6).
- First deployment is for the operator's own testing (e.g. messenger preview checks, F-23 AC 8 and 9); links go to friends only after F-29 (launch condition, decision log 31 Oct 2026).
- Runs behind the operator's existing reverse proxy on the same server (host process or container; another machine is not supported for now), which does TLS and forwards to Caddy on a plain HTTP port. Caddy keeps the ADR-032 headers, caching and `.br` files and serves published pages while Go is down; it trusts the forwarded client address only from the operator's proxy, and Go trusts only Caddy (decision log 1 Nov 2026, later).
- Every Caddy timeout on `/api/*` stays at or above the upload write deadline, 120 s + 6·T, T the parse time limit (ADR-021 rev. 4).
- Deploy: pushing a version tag (`vX.Y.Z`) runs CI and, only when green, creates a GitHub Release with the binary and web build attached; `deploy.sh` deploys a given release tag (no expiry; the deployed version is the tag), copies them and restarts the Go container. The repository is public (AGPL-3.0), so releases are public downloads. CI holds no server credentials (decision log 2 Nov 2026). Without `regenerate` (added by F-30), a deploy keeps old bundles, and old workouts may break after a format change (accepted: no compatibility promise).
- A multi-stage Dockerfile (Go server and web build) lets self-hosters build the app image themselves for the Compose setup (`docker compose build`); they may instead download a release. The operator's own deploy uses the release path above (F-31 design DP-2, DP-16: `deploy/Dockerfile`, `deploy/compose.build.yaml`).
- Structured JSON logs, kept by Docker's logging driver.
- The runbook (`deploy/RUNBOOK.md`, linked from the repository's `README.md`) holds the Docker and proxy operator notes: how to point the operator's reverse proxy at Caddy and set the trusted proxy address. Docker's `"userland-proxy"` and `ip6tables` daemon settings are not needed behind the operator's proxy (F-31 design DP-1d).
- No backups configured. Server config and deploy scripts in the repository.
- Written runbook for rebuilding on a fresh server (binary, Caddy config, empty database).

## Out of scope
- CDN and object storage (scaling path). Analytics (none at first).
- The `regenerate` deploy step (F-30).
- Standalone mode (Caddy with automatic TLS on ports 80/443, ACME); may come later.
- Publishing images: no image registry, no public image, no CI push to the server.
- Uptime monitoring: the operator sets up their own monitor later (decision log 1 Nov 2026).

## Acceptance criteria
- [ ] Pushing a `vX.Y.Z` tag creates a GitHub Release with the binary and web build only when CI is green; a red run creates none.
- [ ] Deploying a given release tag to the server completes with one command; API downtime is only the restart (a few seconds); published pages stay reachable throughout, except for a brief outage when the deploy recreates Caddy (version, service definition or settings change; accepted).
- [ ] After a crash, an out-of-memory kill or a reboot, the Go server and Caddy run again without the operator; a stopped service stays stopped.
- [ ] Caddy listens on a plain HTTP port only (no ACME, nothing on 80/443); a request through the operator's reverse proxy gets the same pages and headers as before.
- [ ] The upload limits are two server settings, default 60 per hour each: IPv4 per address, IPv6 per /64 prefix. Through the operator's proxy, external IPv4 and IPv6 visitors are counted under their own address or prefix; a forwarded address from any other source is ignored.
- [ ] On a clean checkout, `docker compose build` builds the app image from the repository's Dockerfile (Go server and web bundle) with no CI artefact, no release and no registry, and `docker compose up` with it serves the same pages and API as the operator's deploy.
- [ ] Rebuild rehearsal: following the runbook on a fresh VPS gives a working site in under an hour.
- [ ] The VPS has at least 2 GB RAM, and the Compose file caps the Go container's memory at or above the measured worst case for 4 uploads at once; the runbook records the measured value and the chosen cap.
- [ ] The runbook records the measured `parse` time of the all-caps synthetic file (ADR-017 item 7) and the chosen parse time limit.
- [ ] With 4 uploads at the parse limits processed at once, the Go container stays below its memory limit and is not killed; a fifth gets the "try again in a minute" message.
- [ ] No backup job, snapshot schedule or off-server copy of `/srv/workouts` exists.
