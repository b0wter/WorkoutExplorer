# ADR-053 Web assets copied by the app into `{data}/site/assets/`; the bundle clean-up runs in `regenerate`

- **Status:** accepted (15 Nov 2026), with the F-59 design v0.1
- **Date:** 15 Nov 2026
- **Features:** F-59 (introduces it); F-30 (moves its clean-up), F-38 / ADR-043 (`site/`), F-31 (Caddy mounts)
- **Supersedes:** F-30 RG-10's actor and place (`deploy.sh` with `clean-assets.sh` and `bundles/` in the deploy directory) and F-31 DP-10's additive copy of `web/assets/`; ADR-043 item 3 "assets stay in the web build". RG-10's keep rule itself is unchanged
- **Depends on:** [ADR-043](ADR-043-page-shells-filled-at-start.md), F-30 RG-9, RG-10, ADR-022 (data directory on one local file system)
- **Requirements:** INF-9 ("a page published under an older release keeps all its scripts and styles"), SH-10, I-1

## Context

Published pages name content-hashed assets of the release they were built with until `regenerate` rebuilds them; lazy chunks are named only inside JavaScript. Today `deploy.sh` copies `web/assets/` additively on the host and removes old bundles after `regenerate`. In an image `web/` is replaced whole, and the official Caddy container cannot read our image at all. The same must hold on Compose, Podman and bare metal.

## Decision

1. **Caddy serves `/assets/*` from `{data}/site/assets/`** on every path; it never reads the web build. No new mount: `site/` is already Caddy's.
2. **`serve` (at start, before the shells and before listening) and `regenerate` (at start) copy the web build's assets there, additively:** identical files stay, anything else is written by temporary name and rename; a copy never removes anything.
3. **One new package is the only writer of `site/assets/` and `{data}/bundles/`.** `shell` stays the only writer of the shell files in `site/`. Each copy records the current bundle's file names as a list keyed by its content. On a data directory without `bundles/`, the files already in `site/assets/` first become one list (as `pre-f30`).
4. **`regenerate` performs the clean-up** after a run that processed every workout, with RG-10's rule (keep the current list and every list naming an asset a served page names; remove other files, then empty lists), and reports a failed clean-up distinctly.
5. **No lock.** A copy adds only the running version's names; a clean-up never removes the current list; the RG-9 scan can only over-keep.

## Consequences

- Older pages keep their assets on every path, self-built images included (OQ-96's accepted loss ends).
- The deploy user needs no write access to anything Caddy serves; `deploy.sh` and the Podman and bare-metal updates lose their clean-up step.
- Asset files count toward the storage limit (a few MB per kept release).
- A manual `regenerate` (after an Impressum change) also cleans up; harmless, same rule.
- Migration: copying an old install's `web/assets/` into `site/assets/` once is enough; no list conversion.

## Rejected

- **A separate `{data}/assets/` directory:** one more host directory and mount on three paths for no gain.
- **A shared volume filled from the image by an init container:** no counterpart on bare metal, and Compose-only semantics for "additive".
- **Clean-up in `deploy.sh` as today:** the deploy user would need write access inside the data directory.

## Would choose differently if

- Assets move to a CDN or object store (TD scaling path): then the copy targets that store and the clean-up follows.
