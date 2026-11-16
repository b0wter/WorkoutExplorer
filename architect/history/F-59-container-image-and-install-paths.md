# F-59 design history

## "What the F-52 revision must change" (15 Nov 2026), done in F-52 v0.2 on 16 Nov 2026

1. **AU-3:** the deploy archive exists already (IP-5); add `update.sh` and the updater units to its allow-list; drop `clean-assets.sh`, `fetch-caddy.sh`, `caddy.env`. "Caddy stays covered by the checksums in the signed `caddy.env`" is no longer true: the signature covers Caddy's tag name only (user decision).
2. **AU-2, AU-7, "pulls by the signed digest":** satisfied by running the verified archive's `deploy.sh`, whose `compose.yaml` names our image by digest; no separate digest asset or check in `update.sh`.
3. **deploy.sh steps** are IP-10's: image pull instead of file copy, Caddy recreated when a patch arrived, clean-up inside `regenerate`. Exit codes unchanged.
4. **AU-5 drafts vs `latest`:** IP-4 moves `latest` at release creation; with draft releases it would point at an unsigned release's image. Move `latest` in a job on release publication (holding `packages: write`), or accept it (the updater never uses `latest`).
5. **AU-16 prerequisites:** no `sha512sum`; Docker and Compose pull from ghcr.io and Docker Hub.
6. **P-6 / AU-16 Podman:** the Quadlet units are now shipped files (IP-11); decide whether `update.sh` drives Podman hosts (unit directory root-owned).
7. **ADR-050 item 1** ("every release carries a deploy-files archive") is now ADR-052's; ADR-050 keeps signing and the single entry point.
8. **Caddy guard:** `update.sh` pulls Caddy only through IP-7's guard (as `deploy.sh` does), so a patch that needs a capability again is refused before anything changes.
9. **F-60:** units and the apt pin are in the archive; Caddy updates come through apt, never through our updater.

Outcome: 1–3, 5, 7–9 in F-52 AU-3, AU-16, AU-17 and ADR-050; 4 in F-52 AU-5 and ADR-052 item 2; 6 in F-52 AU-13 and ADR-054.
