# F-60 Automatic updates for bare-metal installs

- **Status:** decided (15 Nov 2026; OQ-262, OQ-263 decided); acceptance criteria to write after F-52's revised design
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** backend
- **Lane:** critical (code that installs and runs new code on the server, outside any container)
- **Requirements:** INF-8 (extended to bare metal), INF-9 (bare-metal path), INF-4 (alerts); NFR Privacy, NFR Security
- **Depends on:** F-59 (bare-metal path), F-52 (updater, signed manifest, stopped state). Built after F-52.

## Goal
A bare-metal hoster who opts in gets new releases installed like a Docker or Podman hoster does under F-52, with the same guarantees: only signed releases, alert and stop on failure, no more rights than a manual update needs.

## Scope sketch
- The F-52 updater, extended to the bare-metal layout of F-59: same opt-in, schedule, signature check against the pinned key, version rule, stopped state, alerts, one-at-a-time lock, logging and About-page line (F-52 scope; not repeated here).
- Applies the release tarball (and Caddy, if its version changed) with the same steps as the manual bare-metal update in the runbook: replace the files, restart the services, wait for health, `regenerate`, clean-up.
- The F-59 invariants hold afterwards: units, IP filtering, non-root users and memory cap unchanged by an update unless the signed release changes them.

## Out of scope
- Everything F-52 excludes (rollback, OS or proxy updates, inbound triggers).

## Decisions (user, 15 Nov 2026)
- **OQ-262 Rights:** the updater runs as a dedicated deploy user that owns the program directory and may only restart the two units, through a polkit rule or one narrow sudoers line for `systemctl restart` of exactly those units; never as root. This is a bare-metal-only exception to F-52's "never calls sudo".
- **OQ-263 Unit files:** part of the signed release, installed only when they changed, followed by a daemon reload; if the deploy user lacks the rights for a change, the update stops with an alert (the F-52 stopped state).
