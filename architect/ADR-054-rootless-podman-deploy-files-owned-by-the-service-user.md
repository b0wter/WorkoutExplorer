# ADR-054 Rootless Podman: deploy files and units belong to the service user; root never runs them

- **Status:** proposed (16 Nov 2026), with the F-52 design v0.2; accepted when the user confirms P-7
- **Date:** 16 Nov 2026
- **Features:** F-52 (introduces it), F-59 (changes IP-11's ownership)
- **Supersedes:** F-59 IP-11 "Root installs the units (root-owned unit directory, as today)" and PODMAN.md's `sudo workout-explorer-ctl` from `/usr/local/sbin`
- **Depends on:** [ADR-050](ADR-050-deploys-run-on-the-server-from-a-signed-release.md), [ADR-052](ADR-052-release-image-built-once-pushed-by-the-release-job.md)
- **Requirements:** INF-8, INF-9, NFR Security

## Context

Every release changes the Podman app unit (it names our image by digest, ADR-052) and may change the Caddyfile and the control script. F-59 keeps all three root-owned and the control script runs as root. An updater that must never run as root and never call sudo cannot install them. The root ownership protected little: the service user runs rootless Podman, so it can already start any container of its own with any network, unit files or not.

## Decision

1. **On rootless Podman hosts the deploy directory (`/srv/workout-explorer`, with `conf/` and `operator/`), the Quadlet units and `workout-explorer-ctl` belong to the service user.** Units live in the service user's own Quadlet directory, not under `/etc/containers/systemd/users/`; the notify drop-in sits beside them. The control script lives in the deploy directory and is installed by `deploy.sh` like every deploy file.
2. **Root never runs a file the service user can write.** `workout-explorer-ctl` refuses to run as root; hosters call it as the service user. Commands that today need root (reports as the mapped UID, logs) get a non-root form or a plain root command in the runbook that runs none of our files.
3. **`.env` stays root-owned, group the service user, `0640`**, as today.
4. One Podman layout for every host, updater on or off.

## Consequences

- The updater runs entirely as the unprivileged service user (F-52 AU-13); no step needs root after the one-time setup (user, subuid range, linger, data directory, `.env`).
- The service user can edit its units and the Caddyfile. A compromise of that user already controlled the containers and the data; nothing new is exposed. A compromise of the app container (mapped UID) reaches none of these files.
- Existing Podman setups move once (engineer's runbook step): units out of `/etc/containers/systemd/users/<uid>/`, `/usr/local/sbin/workout-explorer-ctl` removed, ownership changed.
- Engineer confirms on Podman 5.4 that units in the user directory are found and drop-ins apply, and that logs and reports work without root.

## Reversibility

Cheap: ownership and two paths. Going back means Podman hosts lose automatic updates.

## Would choose differently if

- A host must keep units immutable for its own policy: then that host stays on manual updates as root, outside F-52.
