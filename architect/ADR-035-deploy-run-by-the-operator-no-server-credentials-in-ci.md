# ADR-035 Deploys are run by the operator from a GitHub Release; CI holds no server credentials

- **Status:** accepted (31 Oct 2026), with the F-31 design v0.1; rev. 2 with v0.5
- **Date:** 31 Oct 2026; rev. 2 of 2 Nov 2026 (GitHub Releases on version tags replace run artefacts)
- **History:** [history/ADR-035-deploy-run-by-the-operator-no-server-credentials-in-ci.md](history/ADR-035-deploy-run-by-the-operator-no-server-credentials-in-ci.md)
- **Features:** F-31 (introduces it); F-30 (adds the `regenerate` step to the same script)
- **Supersedes:** none. Settles the TD open question "deploy script" together with F-31 DP-10, DP-18
- **Depends on:** [ADR-001](ADR-001-repository-layout-and-build-entry-point.md) items 4 and 5 (CI runs `make -k ci`; deploy consumes `build/`), [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) (forward-only migrations)
- **Requirements:** F-31 AC 1, AC 2; NFR Security, NFR Privacy; decision log 2 Nov 2026

## Context

"CI builds, a script copies" leaves open who runs the copy. If GitHub Actions deploys, GitHub holds an SSH key that can replace the server's binary. That key is a path to every upload and edit-token hash on the server. Deploys are rare, and there is one operator. The user chose GitHub Releases on version tags as the deploy source (no expiry, the version is the tag); the repository and its releases are public (2 Nov 2026).

## Decision

1. **A `vX.Y.Z` tag produces one GitHub Release from the bytes `make -k ci` tested in that run** (F-31 DP-18). A separate job, gated on the green CI job, is the only holder of `contents: write` and runs no repository code. CI runs only on `v*` tags and manual runs (no branch-push runs; 2 Nov 2026); branch runs produce nothing to deploy. A release is created once and never overwritten.
2. **CI never connects to the server.** No server credential exists in GitHub.
3. **The operator runs one command** with a release tag, from a clean checkout at that tag. It downloads the release, verifies its checksum and commit, copies it over the operator's own SSH access, and waits for `/api/health` to report the tag (F-31 DP-10).
4. **The Dockerfile is not on this path** (F-31 DP-2, DP-16). `make ci` builds it as a check; CI uploads no image and pushes to no registry.
5. **No automatic rollback.** A failed health wait stops the script with a non-zero exit and the app's logs. The fix is a newer version (ADR-019). Deploying an older release is possible only while no migration separates the two.

## Consequences

- A leaked GitHub token can change code and create releases, but cannot deploy them. The `SHA256SUMS` asset guards against a broken download, not against someone with write access to the repository: both live in the same place.
- The repository and its releases are public (2 Nov 2026): `deploy.sh` and self-hosters (RUNBOOK 6a) download them without a login.
- A deploy needs the operator's machine, `gh` access to the repository and a pushed tag. There is no deploy on merge.
- Release assets hold build output only. CI uses synthetic files.
- Self-hosters use no release and no deploy script; they build the image on their own server (F-31 DP-16). This ADR does not apply to them.

## Reversibility

Cheap. Adding a CI deploy job later is one workflow job plus a server key. That key is a security decision of its own and needs an ADR revision.

## Would choose differently if

- More than one person deploys, or deploys become frequent. Then use a CI job with a key restricted to the deploy command (`command=` in `authorized_keys`), in a revision.
- The repository becomes public and the releases should not be: move assets to a private store, or delete releases after deploy.
