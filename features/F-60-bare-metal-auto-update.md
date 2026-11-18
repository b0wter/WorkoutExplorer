# F-60 Automatic updates for bare-metal installs

- **Status:** ready for architecture (decided 15 Nov 2026; OQ-262, OQ-263 decided; acceptance criteria written and OQ-267 decided 18 Nov 2026)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** backend
- **Lane:** critical (code that installs and runs new code on the server, outside any container)
- **Requirements:** INF-8 (extended to bare metal), INF-9 (bare-metal path), INF-4 (alerts), INF-1 (About-page line); NFR Privacy, NFR Security
- **Depends on:** F-59 (bare-metal path, `deploy/BAREMETAL.md`), F-52 (`update.sh`, signed manifest, stopped state, lock, alerts; `architect/F-52-auto-update.md`). Built after F-52.

## Goal
A bare-metal hoster who opts in gets new releases installed like a Docker or Podman hoster does under F-52, with the same guarantees: only signed releases, alert and stop on failure, no more rights than a manual update needs.

## Scope
- F-52's `update.sh` on the bare-metal layout of F-59: same opt-in switch, nightly timer, manual `update.sh vX.Y.Z`, pinned-key check of the signed `SHA256SUMS`, version rule, stopped state, lock, Pushover alert from the host, log line, killed-run handling and About-page line (F-52 scope; not repeated here).
- **Every bare-metal update goes through `update.sh`**, by hand or by timer (as P-1 for the container paths), so the lock and the stopped state cover manual updates too. The hand steps of BAREMETAL.md section 6 remain only for what needs root (below).
- **Apply:** the same steps as BAREMETAL.md section 6, from the verified deploy archive and the verified release tarball: check the new Caddyfile with the installed Caddy, replace binary, `web/`, Caddyfile and the updater, restart the two units, wait for health, `regenerate` with its clean-up.
- **Root-only changes stop the update** (OQ-263): unit files and drop-ins, `workout-explorer-run` and the Caddy apt pin `caddy.pref` are installed by the updater only if the deploy user has the right; otherwise the update stops before anything changes, with an alert, and the hoster installs them by hand.
- **Caddy is not updated by the updater.** It comes from Caddy's apt repository: patches arrive with the host's package updates (`workout-explorer-caddy-upgrade.path` restarts it); a new minor comes as a changed `caddy.pref` in a release, which is a root-only change.
- The F-59 invariants hold after every update.

## Out of scope
- Everything F-52 excludes (rollback, OS, package or proxy updates, inbound triggers). Installing Caddy packages, running `apt`.

## Acceptance criteria
- [ ] Every acceptance criterion of F-52 passes on a bare-metal install, except the ones on root and sudo, on the app container and on Caddy image patches, which the criteria below replace. "Running version" and the health wait use `/api/health` at Caddy's host-local address (default `127.0.0.1:8088`).
- [ ] `update.sh vX.Y.Z` run as the deploy user is the manual update on bare metal: it takes the same lock as the timer, clears the stopped state on success and sets it on failure, as on the container paths.
- [ ] An applied update installs the binary and `web/` from the release tarball only after the tarball's SHA-256 matched the verified manifest and its members passed the same checks as the deploy archive; a tarball that does not match is refused (alert, stopped state) and nothing is unpacked.
- [ ] After an applied update: `/api/health` reports the new tag, `regenerate` has run, the log line says `applied`, and a page published under the old release loads every script and style before `regenerate` (as F-59's N to N+1 criterion).
- [ ] A release whose Caddyfile the installed Caddy rejects fails before anything changes (exit 1, alert, stopped state): binary, `web/`, Caddyfile and both running units are as before. A release with a changed, valid Caddyfile is served with it afterwards.
- [ ] A release whose unit files, drop-ins (`notify.conf`, when installed), `workout-explorer-run` or `caddy.pref` differ from the installed ones: if the deploy user may install them, they are installed and systemd is reloaded; otherwise the update stops before anything changes, the alert and the stopped state name the reason, and nothing is replaced. After the hoster installs those files by hand, `update.sh vX.Y.Z` applies the release and clears the state.
- [ ] No updater process runs as root. The deploy user's only privileged actions are restarting exactly `workout-explorer.service` and `workout-explorer-caddy.service` (through one polkit rule or one sudoers line) and running `regenerate` as the app user. As the deploy user, restarting any other unit, any other `sudo` command, and writing to `/etc`, `/usr/local/sbin` or the data directory fail.
- [ ] The updater writes only to the program directory (default `/srv/workout-explorer`), its stopped-state file and its lock.
- [ ] After an applied update, the F-59 bare-metal checks still pass: both services non-root without capabilities, the app without outbound access unless `notify.conf` is installed, `ss -ltn` unchanged, Caddy unable to read the database or stored FIT files, the memory cap unchanged. The updater's own downloads and its Pushover alert work while the app's outbound block is on.
- [ ] The updater never runs `apt` and never changes a Caddy package; a Caddy patch installed by apt between runs does not affect the next update.
- [ ] BAREMETAL.md covers, as the container guides do for F-52: creating the deploy user and handing it the program directory, the restart rule, pinning the key, installing the timer, turning updates on and off, the manual update with `update.sh vX.Y.Z`, the root-only steps when an update stops for them, moving an existing hand-updated install over without losing any workout, draft, report or link, and uninstalling (rule, timer and user removed). A hoster following it on a fresh Debian 13 host gets a working updater.

## Decisions (user, 15 Nov 2026)
- **OQ-262 Rights:** the updater runs as a dedicated deploy user that owns the program directory and may only restart the two units, through a polkit rule or one narrow sudoers line for `systemctl restart` of exactly those units; never as root. This is a bare-metal-only exception to F-52's "never calls sudo".
- **OQ-263 Unit files:** part of the signed release, installed only when they changed, followed by a daemon reload; if the deploy user lacks the rights for a change, the update stops with an alert (the F-52 stopped state).

## Assumed (18 Nov 2026, to confirm)
- Caddy is never updated by the updater (apt repository since 15 Nov 2026); a changed `caddy.pref` is a root-only change like a unit file (OQ-263). Replaces the sketch's "and Caddy, if its version changed".
- `workout-explorer-run` and installed drop-ins count as unit files under OQ-263.
- Manual bare-metal updates go through `update.sh vX.Y.Z` (P-1 extended), so a manual update clears the stopped state.

## Open questions
- **OQ-267 Regenerate rights** (user, 18 Nov 2026): the same rule also lets the deploy user start `regenerate` as the app user through a fixed unit; never `delete`, `parse` or any other command.
