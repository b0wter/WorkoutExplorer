# Technical design history

Superseded text of `spec/Workout Explorer — Technical Design.md`, newest last.

## 15 Nov 2026, F-59 (container image and install paths; ADR-052, ADR-053)

- **Hosting and operations, Server:** "One small VPS in an EU data centre (e.g. Hetzner) with at least 2 GB RAM, Debian. The Go server and Caddy run in one Docker Compose setup, started and stopped together, kept running by Docker restart policies (no systemd). The Go container's memory is capped from the measured worst case of 4 uploads at once (ADR-017 rev. 4, F-31). Settings (Conventions, Configuration) come from one env file read by Compose, the same for `serve` and `regenerate`"
- **Hosting and operations, TLS and static files:** "… Caddy serves `public/` directly, the static page shells (upload, generic, edit, confirmation, Impressum) from `{data}/site/`, where `serve` writes them filled at start (ADR-043), and proxies `/api/*` to Go. …" (assets were served from the web build)
- **Hosting and operations, Deploys:** "Pushing a version tag (`vX.Y.Z`) runs CI and, only when it is green, creates a draft GitHub Release with the Go binary and front-end bundle, a deploy-files archive and `SHA256SUMS`; the operator signs `SHA256SUMS` offline with minisign and publishes the release (ADR-050, F-52). Every deploy on the release layout runs on the server as `update.sh` … runs the archive's `deploy.sh` (no expiry; the deployed version is the tag; ADR-035), which copies the files into place, restarts the Go container (a few seconds of API downtime), then runs `workout-explorer regenerate` (step added by F-30) and, once it finishes, removes each old bundle that no kept workout refers to (Versioning). … Self-hosters can build the app image from the repository's multi-stage Dockerfile (Go server and web build) with `docker compose build`, or download a release; no image is published to a registry, CI pushes none and holds no server credentials. The repository is public (AGPL-3.0), so releases are public downloads. The operator's own deploy uses the release path (F-31 DP-2, DP-16; ADR-034, ADR-035; decision log 2 Nov 2026)"
- **Hosting and operations, Logs:** "Structured JSON logs, kept by Docker's logging driver; …"
- **Build and CI:** ended "Deploy (F-31) consumes `build/`."
- **Repository layout, `deploy/`:** "… `caddy.env` pins the Caddy version; … `Dockerfile`, its allow-list `Dockerfile.dockerignore` and `compose.build.yaml` for self-built images (F-31 DP-16); …"
- **Code structure, writer rule:** "Only the page-shell code started by `serve` writes under `site/` (ADR-043)."
- **Glossary, Data directory:** "`serve -data`: `workouts.db`, `private/`, `tmp/`, `public/` (Data model, Files on disk; ADR-022)."
