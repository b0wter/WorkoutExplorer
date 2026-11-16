# QA F-52 Optional automatic updates for hosters - round 1

- **Date:** 2026-11-16
- **Verdict:** not ready. (1) Blocker: the real `deploy.sh` refuses on every documented host, because the deploy user cannot see `{data}/site`. Every `update.sh` run, by hand or nightly, ends `failed`, sets the stopped state and alerts (QA-52-r1-1). (2) Major: `sign-release.sh` signs whatever the GitHub draft holds. A draft whose `deploy.sh` was swapped, or whose image belongs to someone else, gets signed and published (QA-52-r1-2).
- **Inputs:** `features/F-52-auto-update.md` (17 AC, decisions OQ-210 to OQ-217, P-1 to P-8); `architect/F-52-auto-update.md` v0.2 (AU-1 to AU-17, attack paths); ADR-050, ADR-051, ADR-052 item 2, ADR-054; decision log of 14 to 16 Nov 2026 in `features/README.md`; spec INF-8 and NFR Privacy; TD Deploys, Configuration, Security and privacy (Updates). Reviewed the diff of commit 46a46dc against 6e72023; the working tree was clean.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (local; `/bin/sh` here is bash) | exit 0. Go: every package ok. Vitest: 74 files, 1,927 tests. Size checks pass. `docker-check: ok`, which includes `sh -n` of `update.sh` and the deploy-archive allow-list. `update-check`: `sh -n` of both scripts; 28 harness tests OK with the real minisign 0.12 |
| `go vet ./...` | clean |
| Harness with `WX_TEST_SHELL=bash` | 28 OK |
| Harness under Debian trixie's `dash`, copied out of `debian:trixie` (`/bin/sh` on the target hosts) | 28 OK |
| 24 mutants of `update.sh` against the harness, under dash (scripts in the scratchpad) | **17 killed, 7 survived.** Equivalent (another check or a tool default already covers them): m06 without the `..` check (GNU tar refuses the member anyway), m07 without the `find` check (the type check covers it), m12 without `--proto-redir` (`--proto =https` already limits redirects). Real gaps: m14, m17, m19, m20 (QA-52-r1-3) |
| `docker compose up/exec`, `docker run`, `docker image inspect` under bwrap: root file system read-only, HOME an empty read-only folder, only the deploy folder writable, private `/tmp` | all work. The Compose unit's confinement (`ProtectSystem=strict`, `ProtectHome=read-only`, `ReadWritePaths=` the deploy folder) does not block the Docker CLI. `pull` was not run (no external calls) |
| `systemd-analyze verify` (system units) and `systemd-analyze --user verify` (user units) | clean |
| `git log --all -p` and `.github/` searched for secret-key material and signing steps | no secret key, no `minisign -S`, no signing secret. `deploy/minisign.pub` decodes to an Ed25519 key with ID 9D68BFAEF14635D1 (matches its comment) and passes `update.sh`'s key format check |
| `sign-release.sh` with a fake `gh`, a throwaway key and a scratch repository tagged v9.9.9 (QA-52-r1-2) | it signed and published a draft whose `deploy.sh` is not in the repository and whose `compose.yaml` names `ghcr.io/attacker/...`: exit 0 |
| Data folder as documented (`0750`, owner 10001), in `debian:trixie` as an ordinary user (QA-52-r1-1) | `[ -d /srv/workouts/site ]` is false although the folder exists |

Not run here: Podman (not installed), any GitHub, ghcr.io or DNS request, a real `docker compose pull`, the systemd units installed and run as a real deploy user, `latest.yml`.

## Findings

### Blocker

- **ID:** QA-52-r1-1
- **Severity / kind:** Blocker / defect
- **Route:** backend
- **Where:** `deploy/deploy.sh:31-33`; layouts in `deploy/DOCKER.md:58`, `deploy/PODMAN.md:58`, `deploy/RUNBOOK.md` (operator: `install -d -o 10001 -g 10001 -m 0750 /srv/workouts ...`)
- **Relates to:** AC 4, AC 5, AC 11, AC 17; AU-16; INF-8
- **Evidence:** `deploy.sh` checks `[ -d "${d:-/srv/workouts}/site" ]` as the deploy user. Every guide creates `/srv/workouts` with mode `0750` and an owner the deploy user does not match: 10001 on Compose, the mapped UID on Podman. The deploy user is not in that group either. `stat` needs search permission on every parent folder, so the test is false. Reproduced in `debian:trixie` with an ordinary user: `site-NOT-visible`, although `ls -ld` as root shows the folder. The F-59 remote mode had the same check, but it never ran on a real host (F-59 r1 did not run `deploy.sh` end to end). F-52 now sends every deploy on both container paths through it, and on Podman for the first time. The harness cannot catch it: its `deploy.sh` is a test double, and no test runs the real `deploy.sh` under `update.sh`, not even with a stubbed `docker`.
- **Consequence:** On any host set up by the guides, `update.sh vX.Y.Z` exits 1 with "site is missing". It writes `update.stopped`, and with the switch on it sends a Pushover alert. Automatic updates never apply anything. Clearing the state needs a successful manual deploy, which cannot succeed either. A hoster who follows the runbook from scratch does not get a working updater (AC 17).

### Major

- **ID:** QA-52-r1-2
- **Severity / kind:** Major / defect (gap in the threat model and in the signing script)
- **Route:** architect (AU-5 and the attack-path table), then backend (`sign-release.sh`)
- **Where:** `deploy/sign-release.sh:136-144`; `architect/F-52-auto-update.md` attack paths, rows "GitHub account, repository or CI compromised; assets swapped" and "CI builds malicious bytes"
- **Relates to:** feature goal ("A forged or tampered release is never installed"), AC 9, OQ-212, ADR-050 item 4
- **Evidence:** Before signing, the script checks only these things: the draft's assets match the draft's own `SHA256SUMS`; the names carry the tag; `COMMIT` equals the tag's commit; and some line of `compose.yaml` matches `ghcr.io/[a-z0-9._-]+/workout-explorer:<tag>@sha256:...`. `COMMIT` is a text file that CI writes, so anyone who can change the draft can write the right value. Nothing compares `deploy.sh`, `update.sh`, `workout-explorer-ctl`, the Podman units or the Caddyfile with the tagged source. The image owner is not pinned to `b0wter`, and the Podman app unit's `Image=` is not checked at all. Probe: a draft with an invented `deploy.sh` and `image: ghcr.io/attacker/workout-explorer:v9.9.9@sha256:0…` was signed and published, exit 0. The attack table says swapped assets are refused for lack of a signature. That holds only for swaps after signing. A swap in the window between the CI draft and the operator's signature gets signed. "CI builds malicious bytes" is recorded as a residual risk, but the deploy files are not built; they are copied from the repository with one line changed, so they can be checked locally. The only thing that cannot be checked locally is the image.
- **Consequence:** Anyone with write access to the repository's releases (a stolen GitHub session or token, or a compromised workflow) can get a host-side script signed. Every automatic host then runs it as the docker-group user (root-equivalent) or as the Podman service user. The offline key no longer protects against GitHub for the files that matter most. The fix is cheap: rebuild the deploy archive from the tag (`make deploy-archive APP_IMAGE=<the image named in the draft's compose.yaml>`), then compare every member byte for byte with the draft's. Pin the image owner, and check that the Podman `Image=` matches `compose.yaml`. Optionally check that ghcr.io's `vX.Y.Z` resolves to that digest. The image bytes stay a residual risk, and the attack table should say so for what it is.

### Minor

- **ID:** QA-52-r1-3
- **Severity / kind:** Minor / risk (tests)
- **Route:** backend
- **Where:** `deploy/test/update_test.py`; `deploy/update.sh:132-136`, `:84`, `:158`
- **Relates to:** AU-7, AU-8, AU-10; AC 16
- **Evidence:** Four mutants survive. m14: every failure to fetch `.minisig` treated as a refusal (not only 404). No test serves a 5xx or a timeout for the signature, so a GitHub hiccup that sets the stopped state and alerts would go unnoticed (AC 16). m19 and m20: the size cap after the download and `--max-filesize` each removed; there is no test with an oversized download. m17: `9>&-` removed, so `deploy.sh` and its children inherit the lock. On Podman the rootless pause process outlives the run and would hold `deploy.lock` for good, so every later run would be `busy`. No test checks that the lock goes with the process.
- **Consequence:** The code is correct today. These four properties can regress without a red test.

- **ID:** QA-52-r1-4
- **Severity / kind:** Minor / risk (engineer's deviation 3)
- **Route:** backend (docs)
- **Where:** `deploy/update.sh:89-95`; `deploy/DOCKER.md:146`, `deploy/PODMAN.md:168` ("Then turn it on")
- **Relates to:** P-3, AU-8, AU-9
- **Evidence:** A failed manual run with the switch off writes `update.stopped` and says nothing about it (`test_manual_failure_with_switch_off_sets_state_quietly`). If the hoster turns the switch on later, every night logs `reason=stopped` and sends no alert. The "turn it on" steps do not say to check for the file first.
- **Consequence:** Automatic updates stay off without any notice. One line in the "turn on" steps fixes it ("`cat update.stopped`: if it exists, deploy by hand first"). Changing the code instead is the architect's call.

- **ID:** QA-52-r1-5
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/update.sh` (no `trap`); units `TimeoutStartSec=1h`
- **Relates to:** AU-8, AU-14; AC 10
- **Evidence:** I read this in the code and did not run it. A run killed by systemd's timeout or by an SSH hang-up ends with no log line, no stopped state and no alert. If it dies after `up -d`, the next night `/api/health` already reports the new tag, the run logs `current`, and a `regenerate` that never finished goes unnoticed.
- **Consequence:** A rare, silent, half-done update. A `trap` on TERM, HUP and INT that calls `stop_ failed deploy <n>` while `deploy.sh` runs would close it.

- **ID:** QA-52-r1-6
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/RUNBOOK.md` "Release signing key" (`rm deploy/minisign.pub  # the placeholder`); `deploy/DOCKER.md:136`, `deploy/PODMAN.md:165`; `deploy/BAREMETAL.md:33-34`
- **Relates to:** AC 17, OQ-217
- **Evidence:** (a) The real key is now committed, but the "once, and after a loss" step still deletes "the placeholder". Following it after a key loss is right; following it as written otherwise deletes the live key. (b) The timer is installed "from the deploy archive you checked in section 2" (`/tmp/wx-$v/d`). A hoster who turns updates on weeks later no longer has that folder, and the guides do not say to download and check again. (c) Bare metal still checks only `SHA256SUMS`, although every release now carries `SHA256SUMS.minisig`. The feature's own rule is that checksums alone are never proof. F-60 owns bare metal, but the two-line `minisign -V` step costs nothing now.
- **Consequence:** Small traps for the operator and for hosters. (c) leaves bare-metal installs, done as root, trusting GitHub.

## Engineer's deviations

| # | Judgement |
| --- | --- |
| 1 `reason=-` on applied; a latest that is not vX.Y.Z logs `latest=- reason=current` | Accepted. Not echoing an unverified tag keeps the log free of injected text |
| 2 Exit codes 0 / 1 / 2 / deploy's own | Accepted. `deploy.sh` under `set -e` can in theory exit 2, which reads like "config", but the log line (`action=failed reason=deploy`) tells them apart |
| 3 A refused manual run also sets the stopped state; terminal line and alert only with the switch on | Accepted as P-3 reads; see QA-52-r1-4 |
| 4 `WX_UPDATE_RELEASES`, `WX_UPDATE_PUSHOVER`, `DEPLOY_DIR` | Accepted. None of them can get anything unsigned installed: the signature is always checked against the key in `$DEPLOY_DIR/.env`. Setting any of them requires control of the deploy user's environment. The units set none; `sudo -u` resets the environment; sshd's default `AcceptEnv` does not pass them. Whoever controls that environment can already run code as that user and read `.env`, so redirecting the alert exposes nothing new. `DEPLOY_DIR` does change which `.env` (and so which key) is read, under the same precondition |
| 5 `deploy.sh` refuses without `WX_DEPLOY_FROM_UPDATE=1` | Accepted as a guard against mistakes. It is not a security boundary, and the runbook does not claim it is |
| 6 `FixedRandomDelay`, 03:00 | Accepted. Needs systemd 247 or newer (Debian 12 has 252). `Persistent=false`, so no catch-up, as AU-13 says |
| 7 `_wx-release-key.<domain>` | Accepted. I did not query the record (no external calls) |
| 8 Separate `latest.yml` with `skopeo inspect` | Accepted. `permissions: {}` plus `packages: write`, no checkout, the tag passed through `env` (no expression injection), only upwards, shares the concurrency group `release` |
| 9 Python harness in `deploy/test/` | Accepted. It is outside the archive allow-list. CI installs minisign. It also passes under dash (above) |
| 10 `sign-release.sh` checks the tarball's `COMMIT` and refuses back-ports | Accepted. P-4 logic read and correct. Not enough on its own: QA-52-r1-2 |
| No signature check in BAREMETAL.md | QA-52-r1-6 (c) |

## Previous findings

None (round 1).

## Questions

1. **Architect:** Is the window between the CI draft and the operator's signature inside the threat model? If yes, QA-52-r1-2 needs AU-5 to require the byte-for-byte comparison of the deploy files. If it is deliberately accepted, the attack-path table must say so, and the runbook's "never sign a release you have not reviewed" must say what to review. Either answer settles it.
2. **Backend:** For QA-52-r1-1, drop the `site` check from `deploy.sh`, or check it in a way the deploy user can run (Compose: through the daemon; Podman: `podman unshare`)? Either works. The check matters only for Caddy's bind mount, which already fails visibly.

## Coverage

| AC | Code | Test or check | Status |
| --- | --- | --- | --- |
| 1 Fresh install, no updater, no request | nothing installed by default; units only from the archive | read; real host needed for "a full interval" | traced, real host open |
| 2 Installed and off: no request | `update.sh:68` | `test_switch_off_makes_no_request` | pass |
| 3 No new socket or route | no listener in `update.sh` or `deploy.sh`; no Go route added (diff) | read; `ss -ltn` on a real host open | traced |
| 4 Newest signed release applied, patch/minor/major | `update.sh:110-162` | `test_applies_patch_minor_major` (fake `deploy.sh`) | **fails on a real host: QA-52-r1-1** |
| 5 Several newer: installs the latest | redirect plus compare | `test_several_newer_installs_latest_directly` | pass (harness) |
| 6 Older, same, pre-release, not vX.Y.Z: never | `semver`, `newer()` | `test_never_applies_...`, `test_numeric_comparison`; mutants m11, m21, m23 killed | pass |
| 7 No / foreign / mismatched / replayed signature refused, alert, stop | `update.sh:132-143` | five refusal tests; m01 to m03, m13 killed | pass |
| 8 SHA256SUMS without a valid signature refused | `:137` | `test_refuses_checksums_without_valid_signature` | pass |
| 9 Unsigned deploy files never run | hash against the verified manifest, member checks | `test_refuses_file_not_matching...`, `..._unsafe_archive_members`; m04, m05, m08 killed | pass on the host side; **signing side: QA-52-r1-2** |
| 10 Deploy exit 1/3/4: one alert, stop; no apply while stopped | `stop_`, `:78` | `test_failed_deploy_alerts_once_and_stops`; m09, m16, m24 killed. A refused Caddy patch was not run (needs a real pull) | pass (harness) |
| 11 Manual success clears the state | `:161` | `test_manual_success_clears_stopped_state`; m15 killed | pass (harness); blocked on a real host by r1-1 |
| 12 Never root, no sudo, writes only where allowed | `:50`; ctl refuses root; units `User=`, `ProtectSystem=strict`, `ReadWritePaths` | read; bwrap probe of the Docker CLI | traced; real-host `ps` and write checks open, Podman not run |
| 13 `app` has no route out; download outside it | host-side `curl` only | read | traced |
| 14 Manual and automatic never overlap | `flock` `:71-76` | `test_lock_busy`, `test_manual_and_scheduled_never_overlap`; m10 killed; m17 survives (r1-3) | pass |
| 15 No private key in the repository, CI, logs or assets | `sign-release.sh` uploads only the `.minisig` | history and `.github/` search | pass for the repository and CI configuration; CI logs and assets of a real release not seen |
| 16 Network or HTTP error: log, no change, no stop | `fetch`, `:115-145` | `test_network_errors_change_nothing`; m14 survives (r1-3) | pass |
| 17 Runbook from scratch gives a working updater | RUNBOOK "Updates", DOCKER §8, PODMAN §9 | read | **fails: QA-52-r1-1**; docs gaps in r1-4 and r1-6 |

Also checked and found correct: verification order (signature, then names, then the archive fetched once, hashed, member-checked, unpacked into an empty 0700 folder and run from there; nothing fetched again; no TOCTOU); replay and mix-and-match through the tag in every name; downgrade only by hand; Pushover secrets go to `curl` through the shell's built-in `printf` on stdin (harness argv check; dash's `printf` is built in); `.env` is read as data, never sourced; the log line uses fixed reason classes; `curl -q` comes first everywhere; health is fetched with `--noproxy '*'`; the About block shows only when the switch is on (Go and Vitest tests); the release job now creates a draft; TD Deploys, Configuration and Security are updated.

**Still to do on a real host (after QA-52-r1-1 is fixed):** the first signed release end to end (draft, `sign-release.sh`, publication, `latest.yml` moving only upwards); the DNS TXT record matching `deploy/minisign.pub`; the Compose units installed as root with a real `deploy` user (`journalctl`, writes outside `/srv/workout-explorer` failing, `ss -ltn` before and after, `ps` showing no secret while an alert is sent); a refused Caddy patch leaving stack and tag unchanged; rootless Podman throughout (user timer, `sudo -u workouts update.sh`, no root process, the pause process not holding the lock); one real nightly run over a full interval with the switch off and with it on.
