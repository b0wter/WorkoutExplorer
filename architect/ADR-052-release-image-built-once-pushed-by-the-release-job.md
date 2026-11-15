# ADR-052 One app image per release, built and tested once in CI, pushed by the release job; the release's deploy files pin it by digest; Caddy from the official image on a pinned minor

- **Status:** accepted (15 Nov 2026), with the F-59 design v0.1
- **Date:** 15 Nov 2026
- **Features:** F-59 (introduces it); F-52 (builds on it, to be revised), F-31, F-60
- **Supersedes:** [ADR-034](ADR-034-compose-bridge-networks-with-mounted-binaries.md) item 4 and Context item 2 (rev. 4 text, now in history); [ADR-035](ADR-035-deploy-run-by-the-operator-no-server-credentials-in-ci.md) item 4 ("CI uploads no image"); the decision "nothing published" (1 Nov 2026), reversed by the user on 15 Nov 2026
- **Depends on:** ADR-035 items 1, 2, 5; [ADR-053](ADR-053-web-assets-served-from-the-data-directory.md)
- **Requirements:** INF-9, INF-1, INF-8, NFR Security

## Context

The user made a published image the install path for Docker and Podman, the operator's deploy included, with tags `vX.Y.Z` and `latest`, Caddy from the official image, and no image signing (F-52's signed manifest is to cover our digest). CI gains write access to a public registry, so the question is which bytes get pushed, by whom, and where the digest is recorded so a signature can cover it.

## Decision

1. **Build once.** A `vX.Y.Z` tag run builds the image into an OCI archive inside `make ci`, runs every image check on that archive, and hands it to the release job. Nothing is rebuilt after the tests.
2. **Push by the release job only.** The job that creates the release is the only holder of `packages: write`; it checks out nothing and runs no repository code. It pushes the archive unchanged (digest preserved), refuses a version tag that already exists with another digest, and moves `latest` only to a higher version. A version tag is never overwritten.
3. **The release's deploy files pin our image by digest.** CI writes the digest as a literal into the release's `compose.yaml` (and the Podman app unit) inside the deploy archive listed in `SHA256SUMS`. That file is the digest's one home; image and deploy files of a release can only be used together.
4. **Caddy is the official image on a pinned minor tag** (`docker.io/library/caddy:<minor>`), patch versions floating (user decision 15 Nov 2026). Our image never contains Caddy; ghcr.io carries none.
5. **One plain linux/amd64 image manifest:** no index, no attestations, no signature (out of scope).

## Consequences

- AC "pushed digest = tested digest" holds by construction; a whole re-run fails instead of replacing an image.
- F-52 gets "pull by the signed digest" for free: the signed archive's `compose.yaml` names it.
- The signature covers Caddy's tag, not its bytes; a Caddy patch reaches hosters without a release of ours and without our tests (accepted).
- A leaked `GITHUB_TOKEN` of the release job could push a new version tag; a hoster pinned to a release's `compose.yaml` does not pull it. Until F-52 signs releases, `SHA256SUMS` is integrity only, as before (ADR-035).
- Hosters pull from ghcr.io and Docker Hub: host actions, no About-page line (feature scope).

## Reversibility

Cheap for the push mechanics and the Caddy tag form. One-way in practice: published version tags and digests stay in hosters' files; withdrawing a version means a newer release, not a deletion.

## Would choose differently if

- Builds become reproducible: then the release job could rebuild and compare instead of carrying the archive.
- A Caddy patch breaks pages in production: then pin Caddy by digest again and accept a release per patch.
- Image signing becomes a requirement independent of F-52: cosign keyless adds an online third party; the minisign manifest stays preferred.
