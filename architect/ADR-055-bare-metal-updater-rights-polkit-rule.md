# ADR-055 Bare-metal updater rights: one polkit rule for three unit actions; root-only files never installed by the updater

- **Status:** accepted (18 Nov 2026), with the F-60 design v0.1
- **Date:** 18 Nov 2026
- **Features:** F-60 (introduces it), F-59 (bare-metal program files change owner)
- **Supersedes:** F-59 IP-13 "Program files root-owned" for the program directory (`.env` and `operator/` stay root's); F-52's "never calls sudo" stays true (no sudo either)
- **Depends on:** [ADR-050](ADR-050-deploys-run-on-the-server-from-a-signed-release.md), [ADR-054](ADR-054-rootless-podman-deploy-files-owned-by-the-service-user.md) (same rule: root never runs a file the updater's user can write)
- **Requirements:** INF-8 ("no more rights than a manual deploy needs"), INF-9, NFR Security; OQ-262, OQ-263, OQ-267

## Context

On bare metal the app and Caddy are system services. An update must replace program files, restart both units and run `regenerate` as the app user. The updater must never run as root (OQ-262), yet restarting a system unit and running a command as another user are privileged. OQ-262 allows one narrow grant through polkit or one sudoers line; OQ-267 adds `regenerate`. OQ-263 allows the updater to install changed unit files only "if it has the right".

## Decision

1. **One polkit rule** (in the deploy archive, installed once by root under `/etc/polkit-1/rules.d/`) grants the user `deploy`, for action `org.freedesktop.systemd1.manage-units`, exactly: verb `restart` on `workout-explorer.service` and `workout-explorer-caddy.service`, verb `start` on `workout-explorer-regenerate.service`. Everything else falls through to polkit's default (denied for a non-interactive caller).
2. **`regenerate` runs as a fixed oneshot unit** with the app's user, `.env`, slice and sandbox and a fixed command line. The deploy user cannot pass arguments, so `delete` and `parse` stay out of reach.
3. **The deploy user never gets the right to install root-only files** (unit files, drop-ins, `workout-explorer-run`, the apt pin, the rule itself). A release that changes one stops the update before anything changes (exit 5, reason `root`); root installs them from a release it verified by hand.

## Rejected

- **One sudoers line** (`deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart …`). `sudo` is setuid and fails under `NoNewPrivileges`, which the shared timer unit (F-52 AU-13) sets; dropping it widens every run. It also needs a second line for `regenerate` with a fixed argument list, and the privileged process would be one we start rather than PID 1.
- **Write access to `/etc/systemd/system` or `/usr/local/sbin`** for the deploy user, or a `daemon-reload` right: a unit file names its own `User=`, so this is root.
- **A `systemd-run` right for `regenerate`:** polkit cannot restrict the command line of a transient unit.

## Consequences

- Needs the `polkitd` package (not on every minimal Debian 13 host); one more root daemon.
- No `NoNewPrivileges` exception; the Compose and bare-metal timers share one unit file.
- Every release that touches a unit, the run wrapper or the Caddy pin needs a root step on bare metal; releases should batch such changes and name them in the notes.
- The deploy user is app-equivalent (it owns the binary the app unit runs), never root-equivalent.

## Reversibility

Cheap: the rule is one file; a sudoers line can replace it if the timer drops `NoNewPrivileges`.

## Would choose differently if

- A supported distribution ships no polkit with JavaScript rules (or without systemd's `unit` and `verb` details): then the sudoers line, with `NoNewPrivileges` off for that path's timer only.
- Root steps on bare metal become frequent enough to annoy hosters: then a root-owned, signed-file installer run by a separate root timer, which is a new trust boundary and needs its own ADR.
