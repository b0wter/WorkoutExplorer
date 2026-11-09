# ADR-034 Docker Compose on two bridge networks, with bind-mounted binaries on one pinned base image

- **Status:** accepted (1 Nov 2026), rev. 3 with the F-31 design v0.4; rev. 4 with ADR-039
- **Date:** 31 Oct 2026; rev. 1 of 1 Nov 2026 (bridge networks instead of the host network); rev. 2 of 1 Nov 2026 (behind the operator's reverse proxy, user decision); rev. 3 of 1 Nov 2026 (self-built image for self-hosters, user decision); rev. 4 of 3 Nov 2026 (opt-in egress for report notifications, ADR-039)
- **History:** [history/ADR-034-compose-bridge-networks-with-mounted-binaries.md](history/ADR-034-compose-bridge-networks-with-mounted-binaries.md)
- **Features:** F-31 (introduces it); F-27, F-30 (subcommands through `docker compose exec app`)
- **Supersedes:** "Go binary under systemd" (TD before 31 Oct 2026) and every systemd reference listed in the F-31 design ("Earlier text this design replaces")
- **Depends on:** [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) rev. 6 item 6, [ADR-031](ADR-031-cross-process-lock-on-the-public-area.md), [ADR-022](ADR-022-private-workout-files-staging-and-recovery.md), F-22 H-7, H-10
- **Requirements:** decision log 31 Oct 2026 (Docker Compose), 1 Nov 2026 (no host network), 1 Nov 2026, later (deployment mode), 1 Nov 2026, Dockerfile; NFR Privacy, NFR Security

## Context

Go and Caddy run in one Docker Compose setup with restart policies, without host networking, behind the operator's existing reverse proxy, which does TLS (user decisions). Compose's defaults would break three recorded decisions:

1. On a bridge network, every peer of Caddy and of Go is a container or gateway address, so trusting `X-Forwarded-For` from "loopback" or from any peer would give every visitor one rate-limit key, or let anyone choose theirs.
2. An image per release needs a build on the VPS or a way to ship it, puts the Caddy version in a second place next to `deploy/caddy.env`, and replaces `web/` whole, while published pages still name older assets. Self-hosters, who use no release, still need a way to build what runs (user decision: a Dockerfile, nothing published).
3. Named volumes and Docker-created host paths are root-owned and live under `/var/lib/docker`; ADR-022 and ADR-031 need `/srv/workouts` on one local file system with known modes.

## Decision

1. **Two bridge networks.** `edge` carries Caddy only, IPv4, fixed subnet, and publishes one plain HTTP port on a host address from the env file, default `127.0.0.1` (never public). `backend` is internal (no route out) and carries Caddy and `app`, with Caddy at a fixed address. `app` publishes no port. No `depends_on`. (rev. 4) Only when report notifications are configured, the override `compose.notify.yaml` adds a third network `egress` (not internal) to `app` alone, for outbound HTTPS to Pushover (ADR-039 item 5); inbound is unchanged.
2. **A chain of exactly two trusted hops.** Caddy trusts the forwarded client address only from the operator's proxy (`WX_TRUSTED_PROXIES`, default the `edge` gateway, the source of a host-local proxy) and hands Go the resolved client as the only `X-Forwarded-For` entry. Go trusts exactly Caddy's `backend` address (`WORKOUT_EXPLORER_TRUSTED_PROXY`), which the Compose file sets on both sides.
3. **No Docker daemon settings required.** Clients never reach a published port directly, so the userland proxy and ip6tables do not affect client addresses.
4. **The operator runs no image of ours; self-hosters build one, nobody publishes one.** On the operator's server both services run one third-party base image (distroless static, Debian), pinned by digest; the Go binary and `web/` (GitHub Release, ADR-035) and Caddy (`deploy/fetch-caddy.sh`, `caddy.env` checksums) are bind-mounted read-only. Self-hosters build one local image from `deploy/Dockerfile` on the same base digest, holding the same files at the same paths, and select it with the override `deploy/compose.build.yaml`; `compose.yaml` stays the one service definition for both. No registry, no CI push; `make ci` builds the image (F-31 DP-16, DP-17).
5. **Bind mounts of host directories at the same path inside**, never single files, never created by Compose. Caddy mounts only `public/` from the data directory; its own state is `tmpfs`. *(Note 9 Nov 2026, F-38: also `site/`, read-only, the page shells, ADR-043.)*
6. **Two non-root UIDs.** Caddy gets `app`'s GID as a supplementary group, so it can read `public/` and nothing else. Both run with a read-only root, no capabilities and `no-new-privileges`.

## Consequences

- Go has no outbound route unless notifications are configured (rev. 4, ADR-039), and nothing outside `backend` reaches port 8080, except the host over `egress` when the notify override is on (ADR-039). Caddy's port is reachable from the host only.
- The rate limit is per real client as long as the operator's proxy appends the client's address and `WX_TRUSTED_PROXIES` names only that proxy (runbook, F-31 Verification). Every host process shares the proxy's source address and is trusted with it; the host is the operator's.
- No TLS keys, certificates or ACME account in our setup; TLS is the operator's proxy's.
- A deploy is a file copy and a container restart. There are no image tags to roll back to (ADR-019 forbids rollback anyway).
- One more pull from a third-party registry, the base image, pinned by digest. A digest bump is a reviewed change, in `compose.yaml` and the Dockerfile together (`make ci` fails if they differ).
- Two ways to get the files into the containers; `make ci` builds both. A self-hoster's upgrade recreates Caddy and replaces `web/` whole: a brief page outage, and older published pages lose their assets until `regenerate` (F-30).

## Reversibility

Cheap. Only the Compose files, the Dockerfile, the Caddyfile and the runbook change. Moving the operator to the image later is a deploy script change plus a way to keep old assets.

## Would choose differently if

- Standalone mode comes (Caddy with TLS on 80/443, decision log 1 Nov 2026, later): `edge` publishes 80/443 with IPv6, Caddy trusts no proxy, and the userland proxy off plus ip6tables on become runbook duties again, because clients then reach a published port directly.
- Go and Caddy move to separate hosts (TD scaling path): build and push images, and set Go's trusted address to the Caddy host's.
- Keeping two paths working costs more than it saves (for example the DP-17 check is often red for path-only reasons), or bundles become content-addressed in a data-directory store that outlives the image: then the operator moves to the self-built image too.
- The operator's proxy is on another host: bind `WX_LISTEN` to a private interface only that host reaches, and trust its address; not supported until asked.
