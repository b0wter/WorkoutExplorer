# Continue here (state of 24 Nov 2026)

Follow the lean workflow in `CLAUDE.md`. Keep reports to the user short; ask when in doubt.

## Current

- **Every planned feature is built.** No row of `features/README.md` is `not started`, in architecture or in implementation. Last built: F-63, F-64, F-65 (owner-added laps), F-66 (remove auto-laps), each QA "ready with conditions".
- **Last release: v0.11.0** (24 Nov 2026): F-53 to F-58, F-61 to F-66, the pin dialog and the edit page UI fixes; schema v12, data file v7. Nothing unreleased on `main` yet.
- **Before the next release:** the Caddy minor check and the GitHub changelog (`CLAUDE.md`, Releases); the user's open hand checks in the Waiting on column.
- **Open minors** (small lane): QA-65-r2-1 (test: focus to the next × after a removal). The rest are listed in Waiting on.
- **Open UI questions** (not decided, nothing blocked): `architect/UI-edit-page-add-controls.md`, "Open".

## Reference

- Repo public (AGPL-3.0) at github.com/b0wter/WorkoutExplorer, branch `main`. CI runs on `v*` tags and manual runs. Deploy: `ssh deploy@host /srv/workout-explorer/update.sh vX.Y.Z` (F-52); releases are CI drafts that the user signs with `deploy/sign-release.sh vX.Y.Z` (secret key on the user's machine only).
- The user hosts on a 2 GB VPS.
- Dev server (only while the user tests): `make dev-server DEV_ADDR=127.0.0.1:18081 DEV_BASE_URL=http://192.168.178.98:18080`; Caddy (`make caddy` first) `WX_SITE=http://:18080 WX_API=127.0.0.1:18081 WX_DATA=$PWD/build/data build/tools/caddy-2.11/caddy run --config deploy/Caddyfile --adapter caddyfile`; after `make build`: restart Go, then `WORKOUT_EXPLORER_BASE_URL=http://192.168.178.98:18080 build/workout-explorer regenerate -data build/data -web build/web`.
- Sample pages on protoserve (`/w/<name>`) come from `src/server/testdata/page/*.json`, not from the data file names.

## Reminders

- Real FIT files are never committed. Before every commit, check that nothing under `corpus/real/`, `expected/real/` or `build/`, and no `.br`/`.zip`/`.png`/`.fit` file, is staged.
- `make ci` runs `npm ci`; a running Vite dev server then serves outdated deps. Restart Vite with `--force` after any agent ran `make ci`.
- The Go server reads the page template at start: after rebuilding `build/web`, restart it and regenerate.
- F-24 (viewer download) discarded 14 Nov 2026; unfinished work parked on the local, unpushed branch `wip/f-24-export` (332e041). Drop it or rebase it if it ever returns.
