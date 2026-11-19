# Continue here (state of 18 Nov 2026)

Follow the lean workflow in `CLAUDE.md`. Keep reports to the user short; ask when in doubt.

## Current (19 Nov 2026): v0.10.0 drafted (user signs); F-58 and F-55 built, QA ready with conditions (browser checks); next: F-57

- **F-59** container image and install paths (INF-9; ADR-052, ADR-053, ADR-034 rev. 5): app image on ghcr.io (CI release job), Caddy from the official image by minor tag (patches float, no capability, start guard on every pull route), web assets served from `{data}/site/assets`, docs split into `deploy/RUNBOOK.md` (shared), `DOCKER.md`, `PODMAN.md`, `BAREMETAL.md`. QA r1 ready with conditions, minors fixed. Waiting on real-host checks (Waiting on column).
- **F-52** automatic updates (INF-8; ADR-050, ADR-051, ADR-054; design v0.3): `update.sh` (manual `update.sh vX.Y.Z` and nightly timer, Compose and rootless Podman), `sign-release.sh` (checks the draft against the local tag, signs, publishes), CI makes draft releases, `latest.yml` moves `latest` on publication. QA r2 ready with conditions (`qa/F-52-auto-update-r2.md`), minors fixed (3d3295a); P-9 accepted 18 Nov.
- **Next:** v0.10.0 is a CI draft (F-60, elevation-gain overlay); the user runs `deploy/sign-release.sh v0.10.0`. Since then on main, unreleased: F-58 (summary dialog) and F-55 (hiding sliders, schema v9, irreversible migration). Both wait on the user's browser checks (Waiting on); F-55 also on a local migration + regenerate run before release. All product questions up to F-54 are decided (OQ-268 to OQ-281, 19 Nov). Build order: F-57 (design next), F-56, F-53, F-54, each architect → engineer(s) → QA without asking the user unless a P-point matters.
- Release signing key: created by the user 16 Nov; public key in `deploy/minisign.pub` and DNS TXT `_wx-release-key.workouts.plugman.de` (checked). The secret key stays on the user's machine.
- After F-52: F-60 bare-metal auto-update (decided, AC to write), then F-53, F-58, F-54, F-57, F-55, F-56.
- v0.8.0 (15 Nov): F-47 to F-51 and more; data file format version 6.
- v0.7.0 (12 Nov): F-45, F-46. v0.6.0 (11 Nov): F-42 to F-44, uPlot trim.
- Build order and backlog (Planned, Idea, Fix, Discarded): `features/README.md`.
- Minors to batch (small lane): F-50 QA r1-1 to r1-3; TD/roadmap still lists items that are now ideas.
- User checks still open: Waiting on column of `features/README.md`.
- Dev server: `make dev-server DEV_ADDR=127.0.0.1:18081 DEV_BASE_URL=http://192.168.178.98:18080`; Caddy (`make caddy` first) `WX_SITE=http://:18080 WX_API=127.0.0.1:18081 WX_DATA=$PWD/build/data build/tools/caddy-2.11/caddy run --config deploy/Caddyfile --adapter caddyfile`; after `make build`: restart Go, then `WORKOUT_EXPLORER_BASE_URL=http://192.168.178.98:18080 build/workout-explorer regenerate -data build/data -web build/web`. Only while the user tests.

Repo is public (AGPL-3.0) at github.com/b0wter/WorkoutExplorer, branch `main`; history restarted on 2 Nov 2026 (commit hashes cited in older docs no longer exist). CI runs on `v*` tags and manual runs only; releases v0.1.0, v0.2.0 (4 Nov 2026, up to F-29), v0.3.0 (8 Nov 2026: F-30, F-33, F-25 colouring rework, map controls) v0.4.0 (9 Nov 2026: F-35 to F-38, F-34) v0.5.0 (10 Nov 2026: F-39 to F-41), v0.6.0 (11 Nov 2026: F-42 to F-44, uPlot trim) v0.7.0 (12 Nov 2026: F-45, F-46) and v0.8.0 (15 Nov 2026: F-47 to F-51) exist. From v0.9.0 deploy with `ssh deploy@host /srv/workout-explorer/update.sh vX.Y.Z` (F-52). `make ci` runs Go tests uncached (`-count=1`, about 2 min 46 s).

Done 3 Nov 2026: F-25 route colouring (ADR-037; coloured selection with outline, route faded) and F-26 fullscreen map (CSS overlay, ADR-038; OQ-100 accepted: bars stay visible), both checked by the user. Theme drop-down System/Light/Dark (DS-4, inline head script allowed by CSP hash); header controls on their own line on narrow screens; F-19 download toggle hidden until F-24.

F-27 reports API, operator `delete` command and Pushover notifications (INF-4, ADR-039, opt-in via `compose.notify.yaml` + two env secrets): QA r2 ready. Reports kept at most 30 days and never outlive their workout; 11th report per workout answered 204, not stored; notification = report number + reason only; no auto-hide. Waiting on the user's VPS checks (no egress without override, real Pushover message, delete prompt via exec, Caddy 404 after delete, report query with `sudo setpriv`).

F-28 report form (native dialog, lazy; no button without JS, OQ-103/104 accepted): QA r1 ready. Waiting on: one 429 check (dialog on phone and desktop, wording, no CSP error and a real report checked by the user 4 Nov).

F-29 About page (4 Nov 2026): Go serves `/about`, filled at start from the template (app facts only) and the per-deployment operator file `WORKOUT_EXPLORER_OPERATOR_FILE` (example `deploy/operator.example.txt`; the user's own lives on the VPS in `/srv/workout-explorer/operator/`, never in the repo). Attribution list generated in every `make build` (Vite licence file + bundled dependency walk + Go build info + `tools/attributions/sources.json`, incl. FIT Protocol licence). Done, checked by the user 11 Nov. Still: write the operator file on the VPS (incl. what the proxy logs).

F-30 regenerate (5 Nov 2026, critical lane, QA r2 ready, done): `deploy.sh` runs `regenerate` after health, then removes old bundles only after exit 0 (exit 3 = regenerate did not finish, nothing removed; retry = same tag again). Memory overlap with serve during a deploy accepted by the user (ADR-017 rev. 8). Waiting on the user at the first F-30 deploy: time AC 10 (100 workouts) on the VPS; after each deploy check `OOMKilled` and `RestartCount` (runbook).

F-33 fullscreen cursor readout strip (AP-21, Must): done, checked by the user 5 Nov.

F-34 iOS Shortcut link on the About page (INF-5, Could, phase 2): done (setting `WORKOUT_EXPLORER_IOS_SHORTCUT_URL`). The user's Shortcut works against the existing API with Request Body = Form, one File field `file` (a bare File body gets 400); no server change needed for uploads.

The user hosts on a 2 GB VPS.

F-31 (Compose behind the operator's proxy, IPv4/IPv6 limits, deploy/RUNBOOK.md) QA r1 ready with conditions, minors fixed. The user deploys and runs the server checks listed in qa/F-31-server-deploy-and-monitoring-r1.md; then F-23 AC 8/9 messenger checks.


F-23 built, card v2 approved by the user, QA r3 ready. Waiting on the user: AC 3 loop-route check and AC 8/9 messenger checks (needs a deployed instance). F-21 done. F-22 built (Caddy headers, `caddy-check` in `make ci`); QA r1 ready with conditions, minors fixed (40169a3). Waiting on the user: F-22 browser check through Caddy (no CSP errors, tiles, Copy/download, published page with Go stopped makes no /api request) and F-20's network log. For the browser check use the pinned binary `build/tools/caddy-2.11.7/caddy`, not /usr/bin/caddy (2.10.2). The pinned binary hash in deploy/caddy.env was taken from the downloaded binary, not verified independently.

Dev setup for hand checks: Go `make dev-server DEV_ADDR=127.0.0.1:18081` (8080 is taken by another app), Caddy `WX_SITE=http://:18082 WX_API=127.0.0.1:18081 WX_DATA=$PWD/build/data WX_WEB=$PWD/build/web caddy run --config deploy/Caddyfile --adapter caddyfile`, user opens http://<your-lan-ip>:18082/.

Sample pages on protoserve (`/w/<name>`) come from `src/server/testdata/page/*.json`, not from the data file names: ride-30min, ride-12h, ride-no-hr, run-30min, triathlon, pool-swim, indoor-ride, indoor-unknown-zone, route-hidden, hostile-text. No page shows the no-distance data file.

## Open items for the user

Open user checks are in the Waiting on column of `features/README.md` (moved there 14 Nov 2026).

## Reminders

- F-24 (viewer download) discarded on 14 Nov 2026 (was deferred to phase 3 on 2 Nov 2026). Its design (architect/F-24-viewer-download.md, ADR-036) is done; unfinished B-1 work (internal/fitexport, publish/page/template/Caddyfile changes, no QA, `make ci` not run) is parked on the local, unpushed branch `wip/f-24-export` (332e041). Rebase it onto main when phase 3 starts, or drop it.

- Caddy: official image by minor tag (`compose.yaml`), patches float (F-59); 2.10.2 answered `.br` with 206.

- F-31: Go + Caddy in one Docker setup (decision 31 Oct). Its design must reword the systemd assumptions in F-31 (lines ~13–30), the TD (Hosting and operations, Logs) and the F-21 design hand-off (`Restart=always`).

- Dev: the Go server reads the page template at start. After rebuilding `build/web`, restart it, then re-save (or re-publish) workouts published before.

- `make ci` runs `npm ci`; a running Vite dev server then serves outdated deps (504, blank page). Restart Vite with `--force` after any agent ran `make ci`.

- F-30 `regenerate` must recompute ADR-030's hidden ranges and add `HiddenRanges` to `publish.HidesMore`'s allow-list (else every change triggers a remove-first rebuild).

- F-18 (laps and splits): check that the tapped lap row stays in place when the range panel appears above the laps (desktop jump accepted, 27 Oct). The F-17 design has F-18 hand-offs.
- F-21 and F-30 designs must settle QA-15-r2-1 (sweep vs a concurrent build: cross-process lock or a grace period). F-22: Caddy answers `.br` with 206 (QA-15-r1-3).
- F-23/F-24 artefacts must be linked from `index.html` by a plain path ending in the file name, or the public sweep deletes them (ADR-028 item 5).
- The frontend engineer runs on Opus at effort high from F-17 on (memory `frontend-model-per-feature`).
- Real FIT files are never committed. Before every commit, check that nothing under `corpus/real/`, `expected/real/` or `build/`, and no `.br`/`.zip`/`.png`/`.fit` file, is staged.
