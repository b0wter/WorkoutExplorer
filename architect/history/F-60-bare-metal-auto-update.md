# F-60 bare-metal auto-update design: history

## v0.1 → v0.2 (18 Nov 2026), QA-60-r1-1 (Blocker), QA-60-r1-2 (Major)

Rows replaced. Rows not listed carried over unchanged in substance.

- **BM-2 (v0.1):** root keeps `.env` (`root:deploy`, `0640`) and `operator/` **inside the deploy-owned program directory**. "As directory owner it can rename root's files inside it, `.env` included. `.env`'s root ownership guards against a faulty release, not against the deploy user." Wrong: PID 1 reads `EnvironmentFile=` as root and follows a symlink the deploy user plants, so the deploy user could read any root-only `KEY=VALUE` file through a binary it controls (QA-60-r1-1).
- **BM-10 (v0.1):** "Root installs and runs no file the deploy user can write"; root's steps take files from a release verified by hand (BAREMETAL.md section 2), never from the program directory. Did not cover root reading through deploy-controlled paths, nor the shared, predictable `/tmp/wx-$v` work folder (QA-60-r1-2).
- **BM-11 (v0.1) migration:** `chown -R` of the program directory "except `.env` and `operator/`", which stayed in place.
- **Attack row "Deploy user compromised" (v0.1):** "... restarts our units, rewrites `.env`; ..."
