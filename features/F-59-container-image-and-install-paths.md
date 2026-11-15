# F-59 Container image and install paths

- **Status:** ready for architecture (decided 15 Nov 2026; OQ-261 decided the same day)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** backend
- **Lane:** critical (release supply chain: CI gets write access to a public registry; the privacy invariants of ADR-034 must hold on three paths instead of one)
- **Requirements:** INF-9 (new); INF-4 (egress only with notifications), INF-1 (licences of the base image our image ships), INF-8 (F-52 builds on the image); NFR Privacy (no third party, EU hosting), NFR Security (upload limit per real client), NFR Operations (memory cap); UP-7, AC-4 (the migration loses no workout); technical design: Hosting and operations (Server, Deploys), Build and CI
- **Depends on:** F-31 (deploy, Compose setup, Dockerfile), F-30 (`regenerate` and the bundle clean-up), F-37 (notify egress), F-38 (page shells in `site/`, ADR-043). **Before:** F-52 (its design is revised after F-59)

## Goal
A hoster installs and updates Workout Explorer from a published image with Docker Compose or rootless Podman, or without containers on bare metal, and gets the same privacy and security guarantees on each path. The operator's own server moves to the image without losing anything.

## Scope
- **Image per release.** CI builds one image per `vX.Y.Z` tag from `deploy/Dockerfile` (today's self-build Dockerfile becomes the release build) and pushes it to the GitHub Container Registry, public, anonymous pull. linux/amd64 only. Tags: `vX.Y.Z`, and `latest` for hosters who want the newest release; `latest` moves only to a newer version. The F-52 updater never uses `latest`: it pulls by the digest in the signed manifest. Pre-releases and other tags get no image and do not move `latest`.
- **Contents:** only the app: the Go binary and the web build, on the pinned base image. No Caddy, no Caddyfile, nothing from a data directory, no settings, no operator file, no Impressum, no key or token.
- **Caddy from the official image.** On the container paths Caddy runs from the official Caddy image, pinned by digest in `compose.yaml` (and the Quadlet unit), at the version in `deploy/caddy.env`; a `make ci` check keeps the two in step. The Caddyfile comes from the deploy files, not from an image. Caddy cannot read `web/` inside our image, so the web assets reach it through the data directory (see Old pages keep their assets).
- **Same hardening as today** (ADR-034 items 5, 6), for the app container and the official Caddy container alike: two non-root UIDs, read-only root, no capabilities, `no-new-privileges`, host directories bind-mounted, never created by the container runtime. How the Caddy image is run that way is the architect's.
- **Supply chain.** The pushed image is the one the green CI run built and tested; it is pushed only when CI is green, by the job that also creates the release, which alone holds write access to the package and runs no repository code (as ADR-035 item 1). A version tag is pushed once and never overwritten. The release lists the image's digest in a file covered by `SHA256SUMS`, so F-52's signed manifest (OQ-211) covers the image. No image signing in F-59.
- **Licences.** Publishing the image distributes the base image's files: the About page's attribution list (INF-1) names the base image with its licences, licence texts it requires ship in the image, and the image's labels name the source repository and the tag it was built from (AGPL-3.0 source offer). Caddy is not shipped by us (official image, or fetched by the hoster on bare metal); its existing attribution entry stays.
- **Old pages keep their assets.** A page published under an older release keeps loading all its scripts and styles after a deploy of a newer one, until `regenerate` has rebuilt it; the clean-up after a successful `regenerate` then removes only assets no served page names (F-30 RG-10). Today this works because `web/` on the host is copied over additively; in an image `web/` is replaced whole. On the container paths the assets must reach Caddy through the data directory anyway (official Caddy image). How it is solved is the architect's (one idea: `serve` copies its assets additively into the data directory at start, like the page shells in `site/`, ADR-043, and the clean-up works there).
- **Three install paths, each a runbook section:**
  - **Docker Compose** with the published image (`deploy/RUNBOOK.md`); replaces the release-tarball layout (RUNBOOK 6a and the operator's deploy, section 13). The update steps include `regenerate` and the clean-up; the operator's `deploy.sh` deploys the image of a tag.
  - **Rootless Podman** with systemd Quadlet units using the image (`deploy/PODMAN.md` rewritten).
  - **Bare metal:** the Go binary and Caddy as two systemd services on the host, from the release tarball (still published for this path, without Caddy). Caddy is installed at the version pinned in `caddy.env` with the checksum check of `fetch-caddy.sh`, not from a distribution package (OQ-261). Update by hand, steps in the runbook (automatic updates later: F-60).
  - Building the image locally from a clone (`compose.build.yaml`) stays possible, as a short runbook note, not a separate path; F-52 does not cover it (OQ-215).
- **Invariants on every path** (ADR-034):
  - The app has no outbound network access unless report notifications are on (INF-4, Compose `compose.notify.yaml` or its equivalent); then only the app gets it. Bare metal: systemd IP filtering or an equivalent the architect picks.
  - Caddy's port is bound to a host-local address only; the app's port is reachable by Caddy and the host only, never from outside.
  - The same trust chain: Caddy trusts the forwarded address only from the hoster's proxy, the app only from Caddy, so the upload limit (UP-10, NFR Security) counts real clients.
  - The app's memory is capped at the measured worst case (NFR Operations); both services restart after a crash or reboot, a stopped service stays stopped.
  - Caddy can read only what it serves (`public/`, `site/` and the web assets), not the database, stored FIT files or anything else in the data directory; nothing writes outside the data directory.
  - Logs stay as today: no visitor addresses (TD Conventions, Logging).
  - The subcommands (`regenerate`, `delete`, reading reports) work on every path, as documented.
- **Migration.** Existing installs on the release-tarball layout (the operator's VPS, RUNBOOK 6a) and the Podman setup of today move to the image with a runbook procedure. The data directory stays in place; every workout, draft, report, edit link, preview image and published page survives with its links and expiry unchanged (workouts may still end at a later format upgrade, UP-7, but never because of this move). A short outage of pages and API during the move is accepted.
- **Pulling the image** is a host action, like downloading a release today; the app process still contacts nobody. No About-page line for it (the updater's line stays F-52's, OQ-216).
- Technical design updates (architect): Hosting and operations (Server: three paths; Deploys: image, digest in the release; "no image is published" removed), Build and CI (registry push); ADR-034 item 4 and its Context item 2 reversed, ADR-035 item 4 changed (user decision 15 Nov 2026); F-29 attribution list gains the base image.

## Out of scope
- Other architectures (arm64, multi-arch). Image signing with cosign or similar (the digest in F-52's signed manifest is enough). Automatic updates (F-52 for Docker and Podman; bare metal: F-60, planned after F-52). Kubernetes, Swarm, other registries. Standalone mode with TLS in Caddy. Hosters' own reverse proxy setup beyond today's runbook section.

## Acceptance criteria
- [ ] Pushing a `vX.Y.Z` tag with green CI publishes exactly one image `ghcr.io/…:vX.Y.Z` (linux/amd64), moves `latest` to it, and creates a release whose checksummed files include the image's digest; a red run publishes neither. A pre-release or other tag publishes no image and leaves `latest` unchanged; pushing an older version than the newest never moves `latest`.
- [ ] The digest pushed equals the digest of the image tested in that CI run; re-running the job for an existing version does not change the image behind the tag.
- [ ] Only the release job has write access to the package; no other job and no repository script can push.
- [ ] The image can be pulled without a login. It contains the Go binary and the web build, and no Caddy, Caddyfile, settings, data, operator file, Impressum, key or token (checked by listing its file system).
- [ ] The About page lists the base image with its licences; the image carries the required licence texts and labels naming the repository and the tag.
- [ ] On the container paths, Caddy runs from the official image pinned by digest at the `caddy.env` version, with the Caddyfile from the deploy files; `make ci` fails if the digest's version and `caddy.env` differ. On bare metal, the installed Caddy matches `caddy.env` and its checksum.
- [ ] On each path (Compose, rootless Podman, bare metal), following the runbook on a fresh Debian VPS gives a working site: upload, publish, viewing page, About page and `/api/health` reporting the tag.
- [ ] On each path: both processes, Caddy included, run as non-root users without capabilities (Compose and Podman: read-only root, `no-new-privileges`, for both containers; bare metal: the systemd equivalent); the app cannot open an outbound connection with notifications off, and can reach Pushover with them on; `ss -ltn` shows Caddy only on the configured host-local address and nothing from the app on a public address.
- [ ] On each path, through the hoster's proxy, external IPv4 and IPv6 visitors count under their own address or /64 for the upload limit; a forwarded address from any other source is ignored (as F-31 AC 5).
- [ ] On each path, Caddy cannot read the database, the stored FIT files or anything in the data directory beyond `public/`, `site/` and the web assets.
- [ ] On each path, after a crash, an out-of-memory kill or a reboot both services run again; the app's memory cap equals the Compose value.
- [ ] On each path, `regenerate`, `delete` and the read-only report check work as the runbook says.
- [ ] Deploying release N+1 over N: a page published under N, opened after the deploy and before `regenerate`, loads every script and style (no 404 in the network log); after `regenerate` and the clean-up, assets no served page names are gone and assets of kept workouts (`kept` > 0) stay.
- [ ] Migration rehearsal on a copy of a tarball-layout install with drafts, published workouts (with and without hidden ends), reports and preview images: afterwards every workout, link, expiry and report is unchanged, published pages and edit links work, and `regenerate` reports `deleted=0` for files the old version read.
- [ ] The Podman guide and the bare-metal section no longer use the tarball layout of `web/` with bind-mounted binaries (Podman) and need no Docker (bare metal).

## Decisions (user, 15 Nov 2026)
- Docker and rootless Podman are first-class install paths on a published ghcr.io image built by CI per tag; it replaces the release-tarball install, the operator's deploy included. Reverses ADR-034 item 4 and "nothing published".
- Bare metal (Go and Caddy as systemd services, from the release tarball) stays supported.
- Tags `vX.Y.Z` and `latest`; the F-52 updater never uses `latest` and pulls by the signed digest.
- Our image holds only the app (Go binary, web build); Caddy runs from the official Caddy image pinned by digest at the `caddy.env` version; ghcr.io carries no Caddy.
- **OQ-261 Caddy on bare metal:** installed at the pinned version with the checksum check of `fetch-caddy.sh`, not a distribution package; the release tarball does not include it.
- F-59 is built before F-52. Automatic updates cover Docker and Podman first (F-52); bare metal is updated by hand until F-60 (planned, after F-52).
