# QA F-60 Automatic updates for bare-metal installs - round 1

- **Date:** 2026-11-18
- **Verdict:** not ready. (1) The deploy user can read any root-only file in `KEY=VALUE` form: it owns the directory that holds root's `.env`, so it can replace `.env` with a symlink, and systemd (PID 1, as root) follows it into the environment of a binary the deploy user controls. The user can trigger this itself through the polkit rule (QA-60-r1-1, Blocker). (2) BAREMETAL.md has root install unit files from a fixed, predictable `/tmp/wx-$v` folder. Any local user, the deploy user included, can create that folder in advance and swap the checked files before `sudo install` runs. The exit-5 stop tells the deploy user which tag root will download next (QA-60-r1-2, Major). Everything else holds: polkit, the tarball check, stop-before-change and the code paths.
- **Inputs:** `features/F-60-bare-metal-auto-update.md` (AC 25 to AC 35 as the design numbers them: AC 25 is the file's first criterion); `architect/F-60-bare-metal-auto-update.md` v0.1 (BM-1 to BM-12, T-1 to T-3, P-10); ADR-055; F-52 (`features/F-52-auto-update.md`, `architect/F-52-auto-update.md`), ADR-050, ADR-051; F-59 IP-13, IP-16. Reviewed: commit deb6349 (diff f1d728e..deb6349) on a clean tree: `deploy/update.sh`, `deploy/deploy.sh`, `deploy/systemd/50-workout-explorer.rules`, `deploy/systemd/workout-explorer-regenerate.service`, `deploy/BAREMETAL.md`, `deploy/RUNBOOK.md`, `deploy/updater/system/*`, `deploy/test/update_test.py`, `Makefile`.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Go: every package ok. Vitest: 1,926 passed. Entry edit.html 57,205/60,000. `caddy-check` ok on caddy:2.11 = v2.11.9. `docker-check` ok. `update-check`: `update_test.py` 44 tests OK, `sign_test.py` 12 tests OK |
| `update_test.py` with Debian trixie `dash` as `sh` on PATH (so `update.sh` and `deploy.sh` both run under dash) | 44 tests OK |
| Mutants of the bare-metal code in `update.sh` and `deploy.sh` (scratchpad copies, tests `-k bare -k path -k container`) | **22 of 29 killed.** Killed: each of slice, `notify.conf`, run wrapper, pin and Caddy-minor compares removed; validate without `env -i`, result ignored, or run on the installed Caddyfile; binary mode; Caddy always restarted; `web.old` kept; both regenerate-status mappings; fixed health port in `deploy.sh` and in `update.sh`; exit 5 not mapped to `root`; tarball hash, members and required members not checked; container paths fetching the tarball; Compose and bare metal both accepted; `update.sh` not replaced. Survived: (a) `*.path` dropped from the compare; (b) restore of `web.old` after a run killed mid-swap removed; (c) Caddy not started when it is inactive (the fake `is-active` always succeeds); (d) empty pin accepted; (e) `web/` modes not set; (f) leftovers only partly removed; (g) `deploy.sh`'s own tarball check removed. (e) to (g) are equivalent here (`update.sh` checks first; the harness umask; `cp` and `mv` overwrite). (a) to (c) are real gaps (QA-60-r1-7) |
| Container probe, Debian 13 `debian:trixie`, systemd 257.13, polkitd 126, the shipped rule (throwaway, removed) | As `deploy` with `NoNewPrivileges`: `restart` of the app and Caddy units and `start` of the regenerate unit are allowed. Denied: `restart` of regenerate, `start` and `stop` of the app, `reload-or-restart`, `try-restart`, `kill`, restart or start of another unit, `daemon-reload`, `set-property`, `reset-failed`, `enable`, `--job-mode=isolate`, `systemd-run` (named `workout-explorer-regenerate.service` or not, `--scope`), raw D-Bus `StartTransientUnit` (UnitExists), `EnqueueUnitJob`, `SetUnitProperties`, `RestartUnit` with `restart-dependencies`, `sudo` ("no new privileges"). Also allowed: `restart --job-mode=ignore-dependencies` (harmless). The same results under the timer's sandbox (`ProtectSystem=strict`, `PrivateTmp`, `ProtectHome=read-only`). **T-1 confirmed** |
| T-2 in the same container | A failed oneshot keeps `ExecMainCode=1 ExecMainStatus=4` after `start` returns: confirmed. **But after a refused start the old run's values are still there** (QA-60-r1-3) |
| T-3: `caddy validate` of `deploy/Caddyfile` with caddy:2.11 (v2.11.9) as UID 65534, read-only root, `env -i` with the unit's six `WX_*` values and an empty throwaway home | "Valid configuration", exit 0; nothing written to the home; no admin socket, no data directory. **Confirmed** |
| `systemctl show -p Environment` with a drop-in moving `WX_SITE` to `:9099`, parsed by the scripts' `unitenv` | `WX_SITE=http://:9099`, `WX_BIND=127.0.0.1`, `WX_TRUSTED_PROXIES=127.0.0.1 ::1` (quoted value): correct |
| `EnvironmentFile=` probe (QA-60-r1-1) and `/tmp/wx-$v` probe (QA-60-r1-2) | Both reproduce; details in the findings |
| Real release tarball shape: `.github/workflows/ci.yml:47` (`tar -C build -czf … workout-explorer web COMMIT`) and today's `build/web` names against the member rule | Flat names, as the tests use; every `build/web` name passes the rule; no symlinks; 25 MB binary + 2.9 MB web, far below the 200 MB cap |

Not run: a real Debian 13 VM with the real binary and Caddy, a real release from GitHub, the nightly timer, Pushover, `apt`.

## Findings

### Blocker

- **ID:** QA-60-r1-1
- **Severity / kind:** Blocker / defect (design)
- **Route:** architect (BM-2, ADR-055: where `.env` lives relative to the deploy-owned tree), then backend (units, `workout-explorer-run`, BAREMETAL.md sections 2 and 7)
- **Where:** `deploy/BAREMETAL.md:27` and `:152` (deploy owns `/srv/workout-explorer`), `:51` (`.env` inside it); `deploy/systemd/workout-explorer.service:13`, `deploy/systemd/workout-explorer-regenerate.service:15`, `deploy/systemd/workout-explorer-run:27` (`EnvironmentFile=/srv/workout-explorer/.env`)
- **Relates to:** AC 31 ("No updater process runs as root"), OQ-262, ADR-055 Consequences ("app-equivalent, never root-equivalent"), BM-2, the design's attack-path row "Deploy user compromised"
- **Evidence:** BM-2 accepts that the deploy user "can rename root's files inside it, `.env` included", and treats that as no more than app-equivalence. That reasoning misses one thing: PID 1 reads `.env` as root. Probe in the Debian 13 container: a directory owned by `deploy` holding a `root:deploy 0640 .env`; `/root/secret.env` 0600 holding `API_KEY=topsecret`, `DB_PASSWORD = spaced-secret` and `PrivateKey = wgkey123` (WireGuard style). As `deploy`: `mv .env .env.orig && ln -s /root/secret.env .env`. A unit with `User=nobody`, `ProtectHome=yes` and `EnvironmentFile=` on that path then started with all three values in its environment (`env` in the journal). On a real install that process is `/srv/workout-explorer/bin/workout-explorer`, which the deploy user replaces at will, and the deploy user starts it itself (`restart workout-explorer.service` or `start workout-explorer-regenerate.service`, both allowed). The replaced binary can hand the values back over localhost or through `{data}/public` served by Caddy. Root's `sudo workout-explorer-run …` reads the same path.
- **Consequence:** A compromised deploy user can read every root-only file that systemd's parser accepts: other services' `.env` files, WireGuard keys, `~root/.aws/credentials`-style `key = value` files. The app user cannot read any of these, so this is more than the design's stated limit, and it needs no action from root. F-59 did not have this (the program directory was root's); F-60 introduces it. Possible fixes, for the architect to choose: `.env` (and `operator/`) outside any directory the deploy user owns, for example a root-owned `/srv/workout-explorer` with deploy-owned subdirectories for program files, `update.sh`, the lock and the state; or `.env` under `/etc` on this path. Any fix moves paths in the unit files, `update.sh` (`$dir/.env`) and BAREMETAL.md section 7. Evidence that would change my mind: a systemd option that makes `EnvironmentFile=` refuse symlinks or files not owned by root (I know of none in 257).

### Major

- **ID:** QA-60-r1-2
- **Severity / kind:** Major / defect (docs)
- **Route:** backend (BAREMETAL.md); architect for a one-line note on BM-10
- **Where:** `deploy/BAREMETAL.md:37` (`mkdir -p /tmp/wx-$v && cd /tmp/wx-$v`), then root's `install` lines at `:17`, `:43`-`:46`, `:75`, `:104`-`:108` (exit-5 steps), `:120` (timer), `:147`/`:154` (migration)
- **Relates to:** BM-10 ("root installs and runs no file the deploy user can write"), ADR-055 item 3, AC 30, AC 35 ("a hoster following it … gets a working updater")
- **Evidence:** Container probe: `deploy` runs `mkdir /tmp/wx-v1.5.0 && chmod 0777`. The hoster's `mkdir -p`, download, check and unpack all succeed silently in that folder. `deploy`, as the folder's owner, then renames `systemd/` aside and puts in its own `systemd/workout-explorer.service` with `User=root`. That is the file root's documented `sudo install … systemd/*.service /etc/systemd/system/` would copy. The window is the minutes between the hoster's `minisign`/`sha256sum` checks and `sudo install`. The exit-5 path makes it targeted: the deploy user's own run writes the tag into `update.stopped`, and the next step tells root to download exactly that release into `/tmp/wx-$v`. On Debian 13, `/tmp` is a shared tmpfs; manual `update.sh` runs use it too (BM-12).
- **Consequence:** A compromised deploy user (or any local user) gets root on the next root step. This undoes ADR-055 item 3 through the documentation rather than the code. The fix is cheap: `w=$(mktemp -d) && cd "$w"` and `"$w"` in every later reference, plus "keep this shell open, or download and check again". DOCKER.md and PODMAN.md use the same pattern; there the stakes are lower (Docker's deploy user is root-equivalent anyway), but fix them when they are next touched.

### Minor

- **ID:** QA-60-r1-3
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `deploy/deploy.sh:186`-`:190`
- **Relates to:** BM-4, T-2, AC 28 (exit codes)
- **Evidence:** Container probe: the regenerate unit exits 4 once. Then the rule is removed and `start` is refused ("Interactive authentication required"). `systemctl show` still prints `ExecMainCode=1 ExecMainStatus=4`, so `deploy.sh` would report exit 4, "live and regenerated, but removing old bundles failed", for a regenerate that never ran. The test's "refused start leaves 0 0" case covers only a unit that has never run.
- **Consequence:** A wrong message and the wrong recovery advice ("run it again later" instead of "regenerate did not run"). Reaching it takes a rule that grants the restarts but not the regenerate start, so it is rare. Fix: compare `InvocationID` (or `ExecMainStartTimestampMonotonic`) before and after `start`; unchanged means rc 1.

- **ID:** QA-60-r1-4
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/BAREMETAL.md:103`-`:111`
- **Relates to:** AC 30, AC 33, BM-6 step 4
- **Evidence:** After root installs changed units and runs `daemon-reload`, `update.sh` restarts the app always and Caddy only for a changed Caddyfile (or when Caddy is inactive). Nothing restarts `workout-explorer-caddy-upgrade.path`. A release that changes only Caddy's unit (sandbox, environment) or the path unit leaves the running processes on the old definition until the next reboot or Caddy package patch.
- **Consequence:** "Applied" while part of the release is not in effect. Add `sudo systemctl try-restart workout-explorer-caddy.service workout-explorer-caddy-upgrade.path` after the `daemon-reload` in that block.

- **ID:** QA-60-r1-5
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/BAREMETAL.md:12`-`:13`, `:26`, `:178`, `:188`, `:191`-`:200`
- **Relates to:** AC 35 (removal "without trace … checked by listing"; "a hoster following it on a fresh Debian 13 host")
- **Evidence:** (a) Section 1 runs `sudo gpg --dearmor` but installs no `gnupg`. The `debian:trixie` image has no `gpg`; whether a fresh Debian 13 server has it depends on the installer tasks (real host). (b) The removal purges `polkitd minisign sqlite3`, but not `debian-keyring` or `apt-transport-https` (nor `gnupg`), which section 1 installed. (c) The final check lists names, not owners. A file left by one of the three deleted UIDs (for example a SIGKILLed manual run's `/tmp/tmp.*`, which the `EXIT` path never removes) passes it. Add `sudo find / -xdev \( -nouser -o -nogroup \) -print` to the check. `getent group caddy` is missing from the Caddy line. (d) Section 2's `useradd … deploy` fails if a `deploy` user already exists (a common name; it may be in `sudo`, against BM-2), and the hoster may carry on. The removal then `userdel`s that pre-existing user. Say "the name must be new; check `id deploy` first".
- **Consequence:** AC 35's "no package or file left" is not quite met, and a step may fail on a minimal host.

- **ID:** QA-60-r1-6
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/deploy.sh:82`
- **Relates to:** BM-6 (1), engineer deviation "validate env limited to referenced WX_* keys"
- **Evidence:** For every `{$WX_…}` in the Caddyfile the script passes `K=$(unitenv K)`. That value is set but empty when the unit does not set K. Caddy treats set-but-empty differently from unset: with `bind {$WX_BIND:127.0.0.9}`, `caddy adapt` gives `127.0.0.9:8088` when the variable is unset and `{}` when it is `WX_BIND=`. Today the unit sets all six referenced keys, so there is no effect.
- **Consequence:** A future Caddyfile key with a default, not set in the unit, is validated against a different config than Caddy runs. Pass the key only when `unitenv` finds it.

- **ID:** QA-60-r1-7
- **Severity / kind:** Minor / risk (tests)
- **Route:** backend
- **Where:** `deploy/test/update_test.py` (bare-metal tests)
- **Relates to:** BM-5, BM-6 step 2, BM-11
- **Evidence:** Surviving mutants: (a) `*.path` dropped from the compare (no case changes the path unit); (b) the `web.old` restore after a run killed between the two renames removed (the leftovers case has `web.new` and the staged binary, never `web.old` without `web/`); (c) Caddy not started when inactive (the fake `systemctl is-active` always succeeds). (c) is the fresh-install path of BM-11 and section 3.
- **Consequence:** Each of these can regress without a failing test. Add one case each.

## Previous findings

None (round 1).

## Questions

1. **For the architect (QA-60-r1-1):** where does root's `.env` live on bare metal so that no deploy-writable path is opened by PID 1? Settled by a BM-2/ADR-055 revision naming the layout. Also check `BindReadOnlyPaths=/srv/workout-explorer/conf`: PID 1 follows a deploy-planted symlink there too. I found no permission bypass (the Caddy user still needs the file permissions), but the new layout should keep it out of reach anyway.
2. **For the architect:** is a run killed between `mv web web.old` and `mv web.new web` acceptable as is? It leaves no `web/` until the next `update.sh` run, so an app crash or reboot in between does not start. The stopped state and the alert are set. Settled by one sentence in BM-6, or by restoring `web.old` in `update.sh`'s kill trap.

## Coverage

| AC | Code | Test / check | Status |
| --- | --- | --- | --- |
| 25 F-52 ACs on bare metal; health at Caddy's address from the unit | `update.sh:62`-`:80`, `deploy.sh:28`-`:40`, `:107`-`:112` | `test_path_detection_needs_exactly_one_marker` (drop-in port, two markers); shared F-52 tests; container `unitenv` check | traced |
| 26 manual `update.sh` takes the lock, clears or sets the state | shared F-52 code | `test_bare_metal_root_only_change_…` (stopped, then cleared by the manual run) | traced |
| 27 tarball hash and member checks, nothing unpacked on mismatch | `update.sh:173`-`:199` | `test_bare_metal_tarball_refused`; mutants 23 to 25 killed | traced |
| 28 health new tag, regenerate, `applied`, old pages keep assets | `deploy.sh:162`-`:210` | `test_bare_metal_applies` (fakes); old-page assets by construction (IP-8) | traced; old pages need a real host |
| 29 rejected Caddyfile changes nothing; changed valid one served | `deploy.sh:73`-`:84`, `:103` | `…caddyfile_rejected…`, `…changed_caddyfile…`; T-3 | traced |
| 30 root-only differences stop with exit 5; after root installs, applied | `deploy.sh:59`-`:71`, `update.sh:205` | `…root_only_change…` (6 cases) | traced; `.path` untested (r1-7); restart gap r1-4 |
| 31 no root process; only the three actions; other actions, sudo, writes to `/etc`, `/usr/local/sbin`, data fail | rule, timer unit | container polkit probe (above) | traced for systemd and sudo; **fails in substance: r1-1**; writes and `ps` on a real host |
| 32 writes only to the program directory, state, lock | timer `ReadWritePaths`; manual runs also use `/tmp` (BM-12) | reading | traced; real host |
| 33 F-59 checks after an update; downloads and alert with the outbound block on | BM-8 by construction; timer has no IP filter | reading | real host |
| 34 no `apt`; a Caddy patch between runs does not matter | `grep apt` finds only the pin compare; minor compare only | test layout 2.11.7 vs pin `2.11.*` | traced |
| 35 BAREMETAL.md coverage, turn-off for good (P-10), full removal | `deploy/BAREMETAL.md` sections 1 to 8 | read against a fresh Debian 13 | all topics present; Caddy-shared-with-proxy and data-loss warnings present; proxy entry named, not changed. **r1-2**, r1-4, r1-5 |

Engineer's deviations, judged:
- `WX_HOST_ROOT`: accepted. `sudo` resets the environment and the timer's environment is root's. A deploy user who sets it only fools its own checks, which it could skip anyway by editing `update.sh`.
- Health-address parsing in both scripts: accepted. Each copy is pinned by a test (mutants 19 and 27 killed). Fold them together when either is touched.
- Caddy restarted when inactive: accepted. The fresh install (BM-11) needs it. Untested (r1-7 c).
- Polkit rule not compared: as designed (BM-5).
- Validate environment limited to the referenced `WX_*` keys: accepted, with r1-6.
- No hand download of the tarball: accepted. Root never installs program files (BM-11).

Also checked and found correct: verification order (signature → names → deploy archive → tarball, each hashed against the signed manifest, member-checked, unpacked into an empty folder in the 0700 work folder, nothing fetched again); no path traversal, symlink, hard-link or absolute member gets through `unpack` (type and name rules before extraction, `find` after). The stop-before-change paths (exit 5, rejected Caddyfile, refused tarball) write nothing in the program directory and call no changing `systemctl` (harness asserts both). A run killed during staging leaves the running install untouched, and the next run removes the leftovers. Swap by rename on one file system; `update.sh` is replaced by rename, so the running copy is unaffected. The regenerate unit has a fixed command line, the app's user, slice and sandbox, and no caller arguments. Exit codes and the `root` reason appear in RUNBOOK and the alert text. ADR-050, F-59 IP-13 and IP-16 notes are present.

**Needs a real Debian 13 host:** the fresh install of sections 1 to 3 end to end (`gpg`, the Cloudsmith key under Debian 13's apt and `sqv`, `polkitd` from a minimal install); the first `update.sh` starting both units; a page published under the old release loading before `regenerate`; the nightly timer with the switch off and on; the Pushover alert with the app's outbound block on; `ps` showing no root process of ours during a run; writes to `/etc`, `/usr/local/sbin` and `/srv/workouts` failing as `deploy`; that `sudo -u deploy` uses a pty (`use_pty`, the default from sudo 1.9.14), since the deploy user controls `update.sh`; the migration rehearsal (section 7) on a hand-updated install with workouts, drafts, reports and links; the full removal followed by its listing (plus `find -nouser`). Repeat r1-1's probe once the layout changes.
