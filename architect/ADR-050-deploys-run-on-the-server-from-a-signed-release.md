# ADR-050 Deploys run on the server from a signed release; a deploy-files archive replaces the checkout

- **Status:** accepted (15 Nov 2026), with the F-52 design v0.1
- **Date:** 15 Nov 2026
- **Features:** F-52 (introduces it), F-31 (deploy, release job), F-30 (regenerate step, unchanged)
- **Supersedes:** [ADR-035](ADR-035-deploy-run-by-the-operator-no-server-credentials-in-ci.md) item 3 (the operator's command from a clean checkout, copying over SSH) and, in item 1, "published" at creation. ADR-035 items 2, 4 and 5 stand
- **Depends on:** [ADR-051](ADR-051-updater-as-host-script-verified-by-minisign.md) (how the signature is checked), [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) (no rollback)
- **Requirements:** INF-8, NFR Security; F-52 OQ-211, OQ-212, OQ-215

## Context

`deploy.sh` takes `compose.yaml`, `Caddyfile`, `caddy.env` and its own steps from a git checkout at the tag and checks the release against that checkout (`COMMIT` = HEAD). An updater on the server has no trusted checkout: a `git fetch` trusts GitHub, which INF-8 must not. Locking against a concurrent manual deploy and clearing the stopped state also need one place on the server, which the remote mode (many separate `ssh` calls) cannot hold.

## Decision

1. **Every release carries a deploy-files archive** (`workout-explorer-<tag>-deploy.tar.gz`), built from an allow-list of the files the release layout installs or runs, listed with the binary archive in `SHA256SUMS`, which the operator signs offline (F-52 AU-3 to AU-5).
2. **CI creates the release as a draft;** the operator's signing step publishes it. No unsigned release is public.
3. **Every deploy on the release layout runs on the server, through `update.sh`:** with a tag by hand (the operator over SSH), or without one on the nightly timer. It verifies the signed manifest, then runs the `deploy.sh` from the verified archive under one lock. `deploy.sh` keeps its steps and exit codes (F-31 DP-10 steps 2 to 5, F-30), loses its remote mode and its checkout, `git` and `gh` checks, and is no longer an entry point.
4. **The commit check moves to signing:** the operator checks `COMMIT` against the local tag before signing; afterwards the signature stands for it.

## Consequences

- One deploy path for manual and automatic deploys; no drift between them.
- The operator's machine needs `minisign`, `gh` and the checkout only to sign; deploying needs only SSH. The linux/amd64 requirement for the operator's machine goes away (Caddy is validated on the server).
- Release users (RUNBOOK 6a) get `regenerate` and the bundle clean-up they lacked, but need `minisign` and a pinned key even without automatic updates.
- Self-built clones are unaffected (OQ-215); they never used `deploy.sh`.
- A release with a broken `deploy.sh` is still signed and runs; `make ci` and the operator's review are the guard, as before.

## Reversibility

Cheap. The remote mode can return as a thin wrapper that copies the verified release and calls `update.sh` over one SSH session.

## Would choose differently if

- Hosters need to deploy without network access from the server: then the operator's machine downloads and verifies, and `update.sh` accepts a local release directory.
- Releases gain assets per platform: the manifest rule (names carry the tag) holds; the required set grows.
