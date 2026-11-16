# ADR-050 Deploys run on the server from a signed release; a deploy-files archive replaces the checkout

- **Status:** accepted (15 Nov 2026); items 1 and 3 revised 16 Nov 2026 with the F-52 design v0.2 (image layout, F-59), item 4 with v0.3 (QA-52-r1-2); old text in [history](history/ADR-050-deploys-run-on-the-server-from-a-signed-release.md)
- **Date:** 15 Nov 2026
- **Features:** F-52 (introduces it), F-31 (deploy, release job), F-30 (regenerate step, unchanged)
- **Supersedes:** [ADR-035](ADR-035-deploy-run-by-the-operator-no-server-credentials-in-ci.md) item 3 (the operator's command from a clean checkout, copying over SSH) and, in item 1, "published" at creation. ADR-035 items 2 and 5 stand (item 4 is superseded by ADR-052)
- **Depends on:** [ADR-051](ADR-051-updater-as-host-script-verified-by-minisign.md) (how the signature is checked), [ADR-052](ADR-052-release-image-built-once-pushed-by-the-release-job.md) (image and deploy archive), [ADR-054](ADR-054-rootless-podman-deploy-files-owned-by-the-service-user.md) (Podman ownership), [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) (no rollback)
- **Requirements:** INF-8, NFR Security; F-52 OQ-211, OQ-212, OQ-215

## Context

`deploy.sh` took `compose.yaml`, `Caddyfile`, `caddy.env` and its own steps from a git checkout at the tag (since F-59 from the release's deploy archive, and `caddy.env` is gone, IP-19) and checks the release against that checkout (`COMMIT` = HEAD). An updater on the server has no trusted checkout: a `git fetch` trusts GitHub, which INF-8 must not. Locking against a concurrent manual deploy and clearing the stopped state also need one place on the server, which the remote mode (many separate `ssh` calls) cannot hold.

## Decision

1. **The operator signs `SHA256SUMS` offline;** it lists the deploy archive (ADR-052, F-59 IP-5, whose `compose.yaml` and Podman app unit name our image by digest) and the bare-metal tarball (F-52 AU-3 to AU-5).
2. **CI creates the release as a draft;** the operator's signing step publishes it. No unsigned release is public.
3. **Every deploy on Docker Compose and rootless Podman runs on the server, through `update.sh`:** with a tag by hand (as the path's deploy user), or without one on the nightly timer. It verifies the signed manifest, then runs the `deploy.sh` from the verified archive under one lock; `deploy.sh` drives both paths (F-52 AU-16). Exit codes 0, 1, 3, 4 unchanged; no remote mode, no checkout, `git` or `gh`; not an entry point.
4. **The release check moves to signing:** before signing, the operator's script rebuilds the deploy archive from the local tag and requires the draft's to match it file by file, and pins our image repository (F-52 AU-5); afterwards the signature stands for it. The image bytes behind the digest stay CI's (F-52 P-9).

## Consequences

- One deploy path for manual and automatic deploys; no drift between them.
- The operator's machine needs `minisign`, `gh` and the checkout only to sign; deploying needs only SSH.
- Compose and Podman hosters need `minisign` and a pinned key even without automatic updates (P-1).
- Self-built clones and bare metal are unaffected (OQ-215; F-60).
- A release with a broken `deploy.sh` is still signed and runs; `make ci` and the operator's review are the guard, as before.

## Reversibility

Cheap. The remote mode can return as a thin wrapper that copies the verified release and calls `update.sh` over one SSH session.

## Would choose differently if

- Hosters need to deploy without network access from the server: then the operator's machine downloads and verifies, and `update.sh` accepts a local release directory.
- Releases gain assets per platform: the manifest rule (names carry the tag) holds; the required set grows.
