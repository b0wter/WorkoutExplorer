# F-52 auto-update design: history

## v0.2 → v0.3 (16 Nov 2026), QA-52-r1-2

- AU-5 signing checks were: hashes, `COMMIT` in the deploy archive equals the local tag's commit, names carry the tag. Now also image repository pinned and deploy archive rebuilt from the local tag and compared file by file; P-9 added.
- Attack rows were: "GitHub account, repository or CI compromised; assets swapped" (refused without a signature, which held only after signing) and "CI builds malicious bytes from the tagged commit: not caught; `COMMIT` catches only a wrong commit. Residual".

## v0.1 (15 Nov 2026) → v0.2 (16 Nov 2026), revision on top of the built F-59

Replaced rows of v0.1 (the release-tarball layout). Rows not listed here carried over unchanged in substance.

- **AU-3 (v0.1):** release assets `…-linux-amd64.tar.gz`, `…-deploy.tar.gz` (new), `SHA256SUMS`, `SHA256SUMS.minisig`; allow-list `deploy.sh`, `update.sh`, `clean-assets.sh`, `fetch-caddy.sh`, `caddy.env`, `compose.yaml`, `compose.notify.yaml`, `Caddyfile`, the two systemd units, templates. "Caddy stays covered: its pinned checksums live in the signed `caddy.env`."
- **AU-5 (v0.1):** "… publishes the release and marks it latest." Nothing on the image's `latest` tag (no image then).
- **AU-7 (v0.1) step 4:** "the archives are fetched" (both).
- **AU-8 (v0.1):** "A manual run never alerts (the hoster is watching)."
- **AU-13 (v0.1):** two system units for every host; no Podman variant.
- **AU-16 (v0.1):** prerequisites included `sha512sum`; rootless Podman (Quadlet) hosts out of scope ("program files are root-owned and `deploy.sh` does not drive them").
- **Open questions P-1 to P-6 (v0.1):** decided by the user on 15 Nov 2026 (decision log of `features/README.md`); P-6 closed by the F-59 decision (Podman covered).
- **Status notes:** "To be revised after F-59" in `architect/INDEX.md` and the TD Deploys row; removed with v0.2.
