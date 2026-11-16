# ADR-050 history

## Item 4, revised 16 Nov 2026 (F-52 v0.3, QA-52-r1-2)

4. **The commit check moves to signing:** the operator checks `COMMIT` against the local tag before signing; afterwards the signature stands for it.

## 15 Nov 2026 text, revised 16 Nov 2026 (F-52 v0.2 on top of F-59)

- Item 1: "**Every release carries a deploy-files archive** (`workout-explorer-<tag>-deploy.tar.gz`), built from an allow-list of the files the release layout installs or runs, listed with the binary archive in `SHA256SUMS`, which the operator signs offline (F-52 AU-3 to AU-5)." The archive itself is now ADR-052's (F-59 IP-5).
- Item 3: "Every deploy on the release layout runs on the server, through `update.sh` … `deploy.sh` keeps its steps and exit codes (F-31 DP-10 steps 2 to 5, F-30), loses its remote mode and its checkout, `git` and `gh` checks."
- Consequences: "The linux/amd64 requirement for the operator's machine goes away (Caddy is validated on the server)" (already true since F-59 IP-10); "Release users (`deploy/DOCKER.md`) get `regenerate` and the bundle clean-up they lacked".
