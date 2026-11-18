# QA F-60 Automatic updates for bare-metal installs - round 2

- **Date:** 2026-11-18
- **Verdict:** not ready. The v0.2 layout closes r1-1 for the current files, but two documented root steps can still install the units of an older signed release, and those units read `.env` from the deploy-owned directory. The exit-5 step takes the tag from the deploy user's own output. Section 7 says "the running one", which is pre-F-60 on every host that needs migrating. Either way, released v0.9.1 brings r1-1 back (probe reproduces it, QA-60-r2-1, Major). Every r1 finding is fixed; three new minors.
- **Inputs:** `architect/F-60-bare-metal-auto-update.md` v0.2 (commit 3405b1e: BM-2, BM-6, BM-10, BM-11), ADR-055 (Consequences revised), F-59 IP-13 note; `qa/F-60-bare-metal-auto-update-r1.md` (QA-60-r1-1 to r1-7, Questions 1 and 2). Diff c60590c..ca536e0 on a clean tree: `deploy/update.sh`, `deploy/deploy.sh`, `deploy/systemd/{workout-explorer.service,workout-explorer-regenerate.service,workout-explorer-run}`, `deploy/example.env`, `deploy/BAREMETAL.md`, `deploy/DOCKER.md`, `deploy/PODMAN.md`, `deploy/RUNBOOK.md`, `deploy/test/update_test.py`. Also read: the v0.9.1 tag's `deploy/systemd/` and `Makefile` (`DEPLOY_ALLOW`).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Go: every package ok. Vitest: 1,926 passed (74 files). Entry edit.html 57,205/60,000. `caddy-check` ok on caddy:2.11 = v2.11.9. `docker-check` ok. `update_test.py` 49 tests OK, `sign_test.py` 12 tests OK |
| `update_test.py` with Debian trixie `dash` as `sh` on PATH | 49 tests OK |
| Mutants of the diff (scratchpad copies of `deploy/`, `update_test.py -k bare`, 11 tests) | **10 of 11 killed:** InvocationID compare removed; validate-env filter removed; quoted keys (`"WX_TRUSTED_PROXIES=…"`) dropped from it; `.env`, `operator` and symlinked `.env` in the program directory not refused (3 mutants); bare path reading `$dir/.env`; `web.old` restore removed; `*.path` dropped from the compare; inactive Caddy not started. Survived: `deploy.sh`'s `WX_WEB` grep run on the bare path. Equivalent: `.env` is absent there, so `grep` exits 2 and the negation passes |
| Container probe, Debian 13 `debian:trixie`, systemd 257.13, polkitd, the shipped units, `workout-explorer-run` and rule, installed with BAREMETAL.md section 2's commands (throwaway container and image, removed) | **r1-1 on the v0.2 layout: clean.** As `deploy`: `.env` and `operator` symlinks to `/root/secret.env` in `/srv/workout-explorer`; `mv`, `ln -sf` and append on `/etc/workout-explorer/.env` all "Permission denied"; `restart workout-explorer.service` and `start workout-explorer-regenerate.service` (allowed) and root's `workout-explorer-run regenerate` each started the deploy-written binary with only the two `.env` values, none of the secrets |
| Same container, v0.9.1's `workout-explorer.service` installed by root (as the exit-5 steps would for a v0.9.1 tag) | `EnvironmentFile=/srv/workout-explorer/.env`. `deploy` plants the symlink and restarts: `API_KEY=topsecret`, `DB_PASSWORD=spaced-secret`, `PrivateKey=wgkey123` in the binary's environment (QA-60-r2-1) |
| r1-3 in the container, with the shipped `deploy.sh` logic | Unit exits 4 as `deploy`: rc 4. Start refused by polkit (as `workout-explorer`) with `ExecMainStatus=4` still recorded: InvocationID unchanged, rc 1. **Fixed** |
| r1-4 in the container | `try-restart workout-explorer-caddy.service workout-explorer-caddy-upgrade.path`: path unit gets a new InvocationID; the inactive Caddy stays inactive. **Fixed** |
| `chown -R deploy:deploy` over a tree with symlinks to `/root/secret.env` and `/root` (section 7 step 4) | Targets stay `root`; only the links change owner. Correct |
| `chown root:deploy` and `chmod 0640` on a path in a deploy-owned directory where a symlink was planted (PODMAN.md's `.env` step, same pattern) | The link's target became `root:deploy 0640` (QA-60-r2-3). `install -m 0640 src dest` over a symlink replaces the link and leaves the target untouched |

Not run: a real Debian 13 VM with the real binary and Caddy, a real release from GitHub, the nightly timer, Pushover, `apt`, `sudoedit`, the migration of section 7 on a real hand-updated install, and the removal of section 8.

## Findings

### Major

- **ID:** QA-60-r2-1
- **Severity / kind:** Major / defect (design and docs)
- **Route:** architect (BM-10, BM-11: which release root may take root-only files from, and from whom the version comes), then backend (BAREMETAL.md sections 6 and 7)
- **Where:** `deploy/BAREMETAL.md:105` ("`v` set to the tag `update.sh` named") and `:108`-`:113`; `deploy/BAREMETAL.md:162` ("the running one or a newer one") and `:164`; `architect/F-60-bare-metal-auto-update.md:36` (BM-10 "Root steps"), `:37` (BM-11 "(4) `update.sh` with the running or a newer tag")
- **Relates to:** BM-10 (root never follows a path the deploy user can change), QA-60-r1-1, ADR-055 Consequences, AC 30, AC 31, AC 35
- **Evidence:** Released and signed v0.9.1 ships `systemd/workout-explorer.service` and `systemd/workout-explorer-run` with `EnvironmentFile=/srv/workout-explorer/.env` (`git show v0.9.1:deploy/systemd/…`; `DEPLOY_ALLOW` at v0.9.1 includes `systemd/`). Two root steps can install those files on a v0.2 host:
  (a) **Exit 5.** The tag root downloads comes from `update.stopped`, the log line or the alert. All three are written by `update.sh`, which the deploy user owns (`update.sh:112`). A compromised deploy user writes `tag=v0.9.1 … reason=root`. The hoster follows section 6: the signature, the tag count and the checksum all pass, because v0.9.1 is a genuine release. Root then installs its units and runs `daemon-reload`. Container probe: with v0.9.1's app unit installed, `deploy` plants `/srv/workout-explorer/.env -> /root/secret.env` and restarts the app (allowed). All three root-only secrets are in the environment of the deploy-written binary. This is r1-1 again, through a documented root step. The signature proves where the files come from, not that they are the right version.
  (b) **Migration, section 7.** "The running one or a newer one": a host that needs this section was hand-updated before F-60, so the running release is v0.9.x. Step 3 installs v0.9.x's units and `update.sh`. Step 2 has already moved `.env` to `/etc`, so the next app restart fails ("Do steps 2 to 4 in one go" does not help). Step 4 hands the directory to `deploy`, which reopens r1-1's exposure. Step 5 fails: v0.9.1's `update.sh` has no bare-metal path, so it exits 2. BM-11 says the same "running or a newer tag".
- **Consequence:** (a) a compromised deploy user gets any root-only `KEY=VALUE` file, the r1-1 Blocker's effect, with one documented step by root; (b) every real migration that follows the text as written ends with the app unable to restart, a failed `update.sh`, and units that read `.env` from a directory `deploy` owns. Cheap fixes: root takes the version from the Releases page, never from the deploy user's state, log or alert, and stops if that version is older than the running one (`/api/health`). Before any `sudo install`, a mechanical check refuses a pre-v0.2 release, for example `grep -qx 'EnvironmentFile=/etc/workout-explorer/.env' "$w"/systemd/workout-explorer.service || echo "too old for this guide: stop"`. Section 7 and BM-11 name the first release with this layout instead of "the running one". Evidence that would change my mind: v0.9.0 and v0.9.1 were never signed with the key hosters pin. Commit f050bac ("v0.9.1 install and first signing done") says they were.

### Minor

- **ID:** QA-60-r2-2
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/BAREMETAL.md:12` and `:17`
- **Relates to:** AC 35 (removal "without trace"), QA-60-r1-5 (b)
- **Evidence:** "Note which of these packages were not installed before (`dpkg -l <name>`)" comes after the `apt install -y` block that installs them. A hoster who works top to bottom cannot know afterwards which were new. Section 8 step 7 then says "Not sure: keep it".
- **Consequence:** Packages this guide installed stay after the full removal. Fix: put the `dpkg -l …` note (or `dpkg -l … > ~/wx-before.txt`) before the `apt install` line.

- **ID:** QA-60-r2-3
- **Severity / kind:** Minor / risk (docs; BM-10's container paragraph)
- **Route:** backend (PODMAN.md); architect one-line correction to BM-10's "Rootless Podman: … no root process reads it"
- **Where:** `deploy/PODMAN.md:82` (`sudo chown root:workouts /srv/workout-explorer/.env && sudo chmod 0640 …`), with `:57` (`/srv/workout-explorer` owned by `workouts`); `architect/F-60-bare-metal-auto-update.md:36`
- **Relates to:** BM-10, ADR-054
- **Evidence:** Root runs `chown`/`chmod` on a path inside the service user's directory. Both follow a symlink. Probe: the planted link's target became `root:<user> 0640`. On a fresh install `workouts` has run nothing but `install` and `cp` of checked files by then, so it is not exploitable in practice. BM-10 is still wrong to say no root process touches the file on Podman. The same holds for any later `sudo <editor> /srv/workout-explorer/.env`.
- **Consequence:** A later edit of these lines, or a hoster who repeats them after the service user has run code, gives that user read access to any root file. Fix: `sudo install -o root -g workouts -m 0640 d/example.env /srv/workout-explorer/.env`, which replaces a link instead of following it (probe), then `sudoedit`, which refuses symlinks by default.

- **ID:** QA-60-r2-4
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/RUNBOOK.md:34`-`:38` (engineer's deviation "operator commands left with a bare-metal note")
- **Relates to:** BM-2 (`operator/` `root:root 0755`), BAREMETAL.md section 2
- **Evidence:** "bare metal: `/etc/workout-explorer/operator` … use that path below" leads to `sudo install -d -o "$USER" -m 0755 /etc/workout-explorer/operator` and an unprivileged `cp`. That gives root's folder to the hoster's login, against BM-2 and section 2's `sudo install -m 0644 … && sudoedit`. Not a BM-10 breach, because the login is not `deploy`.
- **Consequence:** Two contradictory instructions for the same folder. Fix: "bare metal: BAREMETAL.md section 2 instead of these commands".

## Previous findings

| ID | Status | How verified |
| --- | --- | --- |
| QA-60-r1-1 | **fixed** for this release's files (r2-1 is a new path to the same effect) | Container probe on the v0.2 layout: clean (above). `update.sh` refuses `.env`/`operator` (also as a symlink) in the program directory; test plus 3 mutants killed. `test_bare_metal_units_read_root_files_from_etc` pins every `EnvironmentFile=`/`LoadCredential` in `deploy/systemd/` away from `/srv/` |
| QA-60-r1-2 | **fixed** | No `/tmp/wx-` left in any guide (grep). BAREMETAL.md, DOCKER.md and PODMAN.md use `w=$(mktemp -d)`, quote `"$w"` in every later reference, and say to download again in a new shell. PODMAN's `chmod 0755 "$w"` does not reopen a swap window: `mktemp` creates the folder 0700 before the `chmod`; `$w` and `d/` belong to the login, so others can read but not write; `/tmp`'s sticky bit stops anyone else renaming `$w`; and the reader is `sudo -u workouts`, not root |
| QA-60-r1-3 | **fixed** | InvocationID compare; container probe (refused start after an exit-4 run gives rc 1); test cases `refused`; mutant killed |
| QA-60-r1-4 | **fixed** | `try-restart` line in the exit-5 block; container probe |
| QA-60-r1-5 | **fixed**, except the ordering in r2-2 | (a) `gnupg` added; (b) removal lists the packages, keeps `debian-archive-keyring`; (c) `find / -xdev \( -nouser -o -nogroup \)` and `getent group caddy` added; (d) `id deploy` must fail, the removal deletes only a user the guide created |
| QA-60-r1-6 | **fixed** | Only keys present in the unit's environment are passed, plain or quoted; test; 2 mutants killed |
| QA-60-r1-7 | **fixed** | Three new cases (path unit edited, `web.old` restore, inactive Caddy); the three r1 surviving mutants are now killed |

r1 Questions: (1) answered by BM-2 and BM-10. `/etc/workout-explorer/`; Caddy's `conf/` mount is allowed with a stated reason (the Caddy user opens it, file permissions apply, the deploy user already chooses Caddy's config). Accepted. (2) answered in BM-6: accepted, and the next run restores `web.old` first (tested).

## Questions

1. **For the architect (r2-1):** may root's steps on bare metal ever install root-only files from a release older than the running one, or older than the first release with the `/etc/workout-explorer` layout? Settled by one BM-10/BM-11 sentence ("never; the version comes from the Releases page, not from the deploy user") and the check in BAREMETAL.md.

## Coverage

Diff-only round. The AC table of r1 still applies; changes since then:

| AC | Change in this diff | Status |
| --- | --- | --- |
| 28 exit codes | refused regenerate start now rc 1 (r1-3) | traced: test, mutant, container |
| 29 validate | env keeps unset keys unset (r1-6) | traced: test, mutants |
| 30 root-only differences | `.path` compared (test); root steps restart Caddy and the path unit | traced; **r2-1** (which release root installs) |
| 31 no root-equivalence | `.env`/`operator` under `/etc`; BM-10 rule | **holds for this release's files** (container probe); **fails through old signed units, r2-1** |
| 35 BAREMETAL.md install, migration, removal | `/etc/workout-explorer`, mktemp, migration order, removal fixes | read; **r2-1 (b)**, r2-2; real host still needed |

Engineer's deviations, judged:
- RUNBOOK.md operator commands left with a bare-metal note: not accepted as written (r2-4, Minor).
- Migration steps 2 to 4 in one go: accepted. A crash restart in between fails until step 3 installs the new units, and step 5's restart recovers it. Undermined by r2-1 (b) when the release is pre-F-60.
- The engineer's own Debian 13 repeat of the r1-1 probe matches mine.

BM-10 swept across the root and `sudo` steps of BAREMETAL.md, DOCKER.md, PODMAN.md and RUNBOOK.md:
- **BAREMETAL.md:** every `sudo install` takes its files from `"$w"`. Section 2 hands `/srv/workout-explorer` to `deploy` last. Root reads deploy state only through `sudo -u deploy cat`. `chown -R` and `rm -rf` do not follow links (probe). `.env` and `operator/` are edited with `sudoedit` under root's `/etc`. `workout-explorer-run` and the regenerate unit hand the deploy-chosen binary only to the app user, which BM-10 allows. Open: r2-1.
- **DOCKER.md:** the deploy user is root-equivalent through the Docker group, so this path has no BM-10 issue.
- **PODMAN.md:** r2-3.
- **RUNBOOK.md:** `cat …/update.stopped` runs as the login, not root. Open: r2-4.

**Needs a real Debian 13 host:** everything r1 listed (fresh install, first `update.sh` start, old-page assets, nightly timer, Pushover with the outbound block, `ps` without a root process, writes to `/etc`, `/usr/local/sbin` and `/srv/workouts` failing as `deploy`, `sudo -u deploy` with `use_pty`). Also: the section 7 migration with a release that has the v0.2 layout, on a hand-updated v0.9.x install with workouts, drafts, reports and links; `sudoedit /etc/workout-explorer/.env`; a real exit-5 cycle including `apt install caddy` and the `try-restart`; and the section 8 removal followed by its listing and `find -nouser`.
