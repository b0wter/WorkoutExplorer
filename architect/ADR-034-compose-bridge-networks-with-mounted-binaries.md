# ADR-034 Container topology: two bridge networks, two non-root containers (our app image and the official Caddy image)

- **Status:** accepted (1 Nov 2026), rev. 3 with the F-31 design v0.4; rev. 4 with ADR-039; rev. 5 with F-59 (file name kept for links)
- **Date:** 31 Oct 2026; rev. 1 of 1 Nov 2026 (bridge networks instead of the host network); rev. 2 of 1 Nov 2026 (behind the operator's reverse proxy, user decision); rev. 3 of 1 Nov 2026 (self-built image, user decision); rev. 4 of 3 Nov 2026 (opt-in egress for report notifications, ADR-039); rev. 5 of 15 Nov 2026 (published app image and official Caddy image, ADR-052, F-59)
- **History:** [history/ADR-034-compose-bridge-networks-with-mounted-binaries.md](history/ADR-034-compose-bridge-networks-with-mounted-binaries.md)
- **Features:** F-31 (introduces it); F-27, F-30 (subcommands through `docker compose exec app`); F-59 (rev. 5; the same topology in rootless Podman, its systemd form on bare metal: F-59 IP-13 to IP-15)
- **Supersedes:** "Go binary under systemd" (TD before 31 Oct 2026) and every systemd reference listed in the F-31 design ("Earlier text this design replaces"). **Superseded in part:** item 4 by [ADR-052](ADR-052-release-image-built-once-pushed-by-the-release-job.md) (15 Nov 2026)
- **Depends on:** [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) rev. 6 item 6, [ADR-031](ADR-031-cross-process-lock-on-the-public-area.md), [ADR-022](ADR-022-private-workout-files-staging-and-recovery.md), [ADR-053](ADR-053-web-assets-served-from-the-data-directory.md), F-22 H-7, H-10
- **Requirements:** decision log 31 Oct 2026 (Docker Compose), 1 Nov 2026 (no host network), 1 Nov 2026, later (deployment mode), 15 Nov 2026 (container image); INF-9, NFR Privacy, NFR Security

## Context

Go and Caddy run as two containers with restart policies, without host networking, behind the operator's existing reverse proxy, which does TLS (user decisions). Compose's defaults would break three recorded decisions:

1. On a bridge network, every peer of Caddy and of Go is a container or gateway address, so trusting `X-Forwarded-For` from "loopback" or from any peer would give every visitor one rate-limit key, or let anyone choose theirs.
2. (rev. 5) The two images are not ours to shape alike: our app image (ADR-052) and the official Caddy image, which runs as root by default, writes state into its own directories and ships a binary with a file capability.
3. Named volumes and Docker-created host paths are root-owned and live under `/var/lib/docker`; ADR-022 and ADR-031 need `/srv/workouts` on one local file system with known modes.

## Decision

1. **Two bridge networks.** `edge` carries Caddy only, IPv4, fixed subnet, and publishes one plain HTTP port on a host address from the env file, default `127.0.0.1` (never public). `backend` is internal (no route out) and carries Caddy and `app`, with Caddy at a fixed address. `app` publishes no port. No `depends_on`. (rev. 4) Only when report notifications are configured, the override `compose.notify.yaml` adds a third network `egress` (not internal) to `app` alone, for outbound HTTPS to Pushover (ADR-039 item 5); inbound is unchanged.
2. **A chain of exactly two trusted hops.** Caddy trusts the forwarded client address only from the operator's proxy (`WX_TRUSTED_PROXIES`, default the `edge` gateway, the source of a host-local proxy) and hands Go the resolved client as the only `X-Forwarded-For` entry. Go trusts exactly Caddy's `backend` address (`WORKOUT_EXPLORER_TRUSTED_PROXY`), which the Compose file sets on both sides.
3. **No Docker daemon settings required.** Clients never reach a published port directly, so the userland proxy and ip6tables do not affect client addresses.
4. **(rev. 5) Images:** `app` runs our published image, pinned by digest in the release's `compose.yaml`; `caddy` runs the official Caddy image on a pinned minor tag; see [ADR-052](ADR-052-release-image-built-once-pushed-by-the-release-job.md). Self-hosters build the app image locally with the override `compose.build.yaml`.
5. **Bind mounts of host directories at the same path inside**, never single files, never created by Compose. `app` mounts the data directory and the operator folder (read-only). Caddy mounts `public/` and `site/` from the data directory (the web assets live in `site/assets/`, ADR-053) and the `conf/` folder with the Caddyfile, all read-only; a `tmpfs` covers the image's state directories, so no anonymous volume exists.
6. **Two non-root UIDs.** Caddy gets `app`'s GID as a supplementary group, so it can read `public/` and `site/` and nothing else. Both run with a read-only root, all capabilities dropped and `no-new-privileges`. (rev. 5) Caddy too runs with an empty bounding set: the official binary's file capability has no effective bit since 2.11.8, and a `cap_add` would leave it in CapPrm under Docker. Every Caddy pull is guarded, because a patch with the effective bit back would not exec (F-59 IP-7, user decision 16 Nov 2026).

## Consequences

- Go has no outbound route unless notifications are configured (rev. 4, ADR-039), and nothing outside `backend` reaches port 8080, except the host over `egress` when the notify override is on (ADR-039). Caddy's port is reachable from the host only.
- The rate limit is per real client as long as the operator's proxy appends the client's address and `WX_TRUSTED_PROXIES` names only that proxy (runbook, F-31 Verification). Every host process shares the proxy's source address and is trusted with it; the host is the operator's.
- No TLS keys, certificates or ACME account in our setup; TLS is the operator's proxy's.
- (rev. 5) A deploy is an image pull and a recreate of `app`; Caddy is recreated only when a new patch of its minor arrived or its definition changed (brief page outage, accepted). There is still no rollback (ADR-019).
- (rev. 5) A Caddy patch whose binary needs its file capability again is refused by the pull guard (F-59 IP-7); Caddy then stays on its previous patch until our release changes the setup.

## Reversibility

Cheap. Only the Compose files, the Podman units, the Caddyfile and the runbook change.

## Would choose differently if

- Standalone mode comes (Caddy with TLS on 80/443, decision log 1 Nov 2026, later): `edge` publishes 80/443 with IPv6, Caddy trusts no proxy, and the userland proxy off plus ip6tables on become runbook duties again, because clients then reach a published port directly.
- Go and Caddy move to separate hosts (TD scaling path): set Go's trusted address to the Caddy host's.
- The operator's proxy is on another host: bind `WX_LISTEN` to a private interface only that host reaches, and trust its address; not supported until asked.
- A Caddy patch restores the effective bit for good: add `NET_BIND_SERVICE` to Caddy's bounding set (process still holds none on Podman; CapPrm 0x400 on Docker).
