# QA F-52 Optional automatic updates for hosters - round 2

- **Date:** 2026-11-16
- **Verdict:** ready with conditions. All six r1 findings are fixed and their tests kill the mutants that matter. No Blocker or Major finding is open. Conditions: (1) the real-host checks at the end of this report, starting with the first real signing, which is the only test of whether CI's deploy archive and the operator's rebuild match (QA-52-r2-3); (2) the user's decision on P-9. Four Minor findings: the most important one is that `sign-release.sh`'s `sha256sum -c`, the check that keeps r1-2 closed, has no test (QA-52-r2-1).
- **Inputs:** `qa/F-52-auto-update-r1.md` (QA-52-r1-1 to r1-6); `architect/F-52-auto-update.md` v0.3 (AU-5, notes on AU-8, AU-14, AU-16, P-9); ADR-050 item 4. Reviewed the diff 69eb27d..664800c (commits 55cadfe design v0.3, 664800c fixes) on a clean tree, plus the code it touches.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Go: every package ok. Vitest: 1,927 tests passed. `docker-check` ok. `update-check`: `update_test.py` 36 tests OK, `sign_test.py` 6 tests OK (real minisign 0.12) |
| `update_test.py` and `sign_test.py` with `WX_TEST_SHELL=` Debian trixie `dash` | 36 OK, 6 OK. The new trap tests also pass under dash (`deploy.sh` itself still runs under the host's `sh`, bash here) |
| Mutants of `update.sh` and `deploy.sh` (scratchpad clone, both harnesses) | **11 of 11 killed.** r1 survivors: m14 (every `.minisig` failure treated as a refusal), m17 (no `9>&-`), m19 (no size check after the download), m20 (no `--max-filesize`). New: all three traps removed, only the TERM trap removed, `applying=yes` removed, `stop_` removed from `killed`, the switch-off terminal line removed; `deploy.sh` with the `site` check put back, and without the updater-unit copy |
| Mutants of `sign-release.sh` | **7 killed, 8 survived.** Killed: owner pattern instead of `b0wter`; listing compare removed; `diff -r` removed; all of (c) replaced by `true`; tarball `COMMIT` check removed; worktree clean-up removed from the trap. Survived: `sha256sum -c` removed; post-sign `minisign -V` removed; draft check removed; version-order check (P-4) removed; asset-set check removed; names check removed; `umask 022` removed; key-inside-repository check removed. Asset set and names are equivalent here (`sha256sum -c` fails on a missing listed file, and `update.sh` checks the names). `umask` cannot fail in the harness (r2-3). The other four are real gaps (QA-52-r2-1) |
| Probe: draft whose `SHA256SUMS` names another hash for the deploy archive than the served asset (the archive to be swapped in after publication) | real script: refused ("a checksum does not match"). With `sha256sum -c` removed: **signed and published, exit 0**, and no test fails |
| Probe: `make deploy-archive` in fresh clones under `umask` 022, 002 and 077, listings compared as `sign-release.sh` does | all 33 lines differ (modes); details in r2-3 |
| `sign_test.py` with `TMPDIR` in the scratchpad | one `tmp.*` folder with the draft's three assets is left behind after the run (r2-2) |
| The diff checked for anything outside the findings | 14 files, all belonging to r1-1 to r1-6 or their docs and tests. No change to `.github/`, `deploy/minisign.pub`, the key or the units. No new network call except the test doubles' local ones and the `curl` line BAREMETAL.md needed for `SHA256SUMS.minisig`. No check weakened beyond those the findings asked for: the `site` check (r1-1), the `PLACEHOLDER` check (obsolete: the real key is committed, and the post-sign `minisign -V` against `deploy/minisign.pub` still catches a wrong key), and the deploy archive's `COMMIT` comparison, which the rebuild now covers |

Not run: Podman, a real `docker compose`, any GitHub or ghcr.io request, a real CI run, the units on a real host.

## Findings

### Minor

- **ID:** QA-52-r2-1
- **Severity / kind:** Minor / risk (tests)
- **Route:** backend
- **Where:** `deploy/test/sign_test.py`; `deploy/sign-release.sh:27`, `:31`, `:43`, `:65`
- **Relates to:** AU-5 (a), P-4, QA-52-r1-2, feature goal ("a forged or tampered release is never installed")
- **Evidence:** four checks can be deleted without a red test (mutants above). The worst is `sha256sum -c` (`:43`). Without it, the probe draft whose `SHA256SUMS` names hash H' for the deploy archive while serving the good archive passes (c), because the served archive equals the rebuild. The operator signs H', and anyone with release write access then uploads the archive with hash H' (`gh release upload --clobber`). Every host verifies the signature, finds H', and runs that archive's `deploy.sh`. That is r1-2 again, by a second route. Also untested: the post-sign `minisign -V` against `deploy/minisign.pub` (`:65`; without it a release signed with the wrong key is published and every automatic host refuses it, stops and alerts), the draft check (`:27`) and the version-order check (P-4, `:31`; the fake `gh` always answers "draft, nothing published").
- **Consequence:** the code is correct today. A regression in the checks that close r1-2 would ship with a green `make ci`. Four small cases close it: checksum mismatch, a key that does not match `deploy/minisign.pub`, not a draft, a higher version already published.

- **ID:** QA-52-r2-2
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `deploy/sign-release.sh:35` (`trap '... worktree remove ...; rm -rf "$tmp"' EXIT` under `set -e`)
- **Relates to:** AU-5
- **Evidence:** under `set -e`, a failing command inside the EXIT trap ends the trap, in bash and dash alike (probe: `sh -c 'set -eu; trap "git -C / worktree remove --force /x >/dev/null 2>&1; echo RM" EXIT; exit 1'` prints nothing and exits 128). Before (c) creates the worktree, `git worktree remove` fails, so `rm -rf "$tmp"` never runs. This happens on every refusal in (a) or (b) and when `gh release download` fails: the draft's assets stay in `/tmp/tmp.*` and the exit status becomes 128. Reproduced: the harness leaves one such folder (from `test_refuses_foreign_image_owner`). A second, smaller gap: on a Debian or Ubuntu operator machine (`/bin/sh` = dash), Ctrl-C at the password prompt, the abort RUNBOOK.md recommends, does not run the EXIT trap at all, so the worktree registration and `$tmp` are left (probe: dash skips the EXIT trap on SIGINT, bash runs it).
- **Consequence:** stale folders of public release files in `/tmp` and stale `git worktree` entries on the operator's machine. Refusals are still non-zero, so nothing is signed by mistake. Fix: `[ ! -d "$tmp/wt" ] || git ... worktree remove ...` (or `|| true`) and `trap 'exit 130' INT` (likewise HUP and TERM) so the EXIT trap runs.

- **ID:** QA-52-r2-3
- **Severity / kind:** Minor / risk (engineer's deviation `umask 022`)
- **Route:** backend
- **Where:** `Makefile:142-151` (`deploy-archive`), `deploy/sign-release.sh:17`, `.github/workflows/ci.yml:16`, `:44`
- **Relates to:** AU-5 (c), ADR-050 item 4
- **Evidence:** the deploy archive's modes come from the build machine's umask twice: through `git checkout` (files 644 or 664) and through `mkdir -p`, `cp` and the `COMMIT` redirect. Probe: archives built in fresh clones under umask 022, 002 and 077 differ in all 33 listing lines. `sign-release.sh` pins its side to 022, which is right for the operator's machine. The comment "(CI: 022)" is not verified anywhere: no run of the workflow has been inspected, and the workflow sets no umask. The GitHub runner is a systemd service, and systemd's default `UMask=` is 0022, so 022 is likely, but I could not confirm it from the repository or public sources. The harness builds the draft and the rebuild on the same machine, so it cannot catch a mismatch.
- **Consequence:** fails safe. If CI's umask is not 022, `sign-release.sh` refuses every draft ("differs ... (members, types, modes or bytes)"), with no hint at the cause. Nothing wrong is ever signed. Cheap fix that removes the dependency on both sides: normalise in the archive, e.g. `tar ... --mode='u=rwX,go=rX'` in `deploy-archive`; then `umask 022` is no longer needed. Otherwise the first real signing settles it.

- **ID:** QA-52-r2-4
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/RUNBOOK.md` "Cut a release", review list, third point
- **Relates to:** AU-5, P-9
- **Evidence:** "the `release` job's log shows the digest it pushed". By the workflow, it does not: `skopeo copy --digestfile pushed` writes the digest to a file, and the job prints it only on a mismatch. The digest does appear in the `ci` job's log, in make's echoed `sed` line of `make deploy-archive APP_IMAGE=...`. The comparison is also weak: both values come from CI, so it checks CI against itself. Of the three review points, only the package page ("no image version you did not expect") looks from outside the CI run.
- **Consequence:** the operator looks for a line that is not there, or skips the step. Point at the `ci` job's `make deploy-archive` line, or make the release job `echo "pushed $repo:$tag@$digest"`, and say plainly that this step only catches inconsistency, not a malicious CI (P-9).

## Engineer's deviations

| # | Judgement |
| --- | --- |
| New reason class `killed` | Accepted. It is a fixed word (AU-14 note, RUNBOOK list). A signal before `deploy.sh` starts changes nothing, so "none, killed" with exit 128 + n is honest. After it starts, `failed deploy` with the stopped state and the alert is the conservative choice. Tested for HUP, INT and TERM before, and TERM during, under bash and dash. Edge, not a finding: a signal sent only to `update.sh` (not the group) waits for `deploy.sh` to finish and then reports `failed` even if it succeeded. That errs towards an alert |
| Terminal line on a failed manual run with the switch off, no alert | Accepted. It is what r1-4 asked for, recorded as a note on AU-8, asserted by `test_manual_failure_with_switch_off_sets_state_quietly`, and paired with the "`cat update.stopped` first" step in both guides |
| `deploy.sh` copies the updater units of both paths to `{deploy dir}/updater/` | Accepted. AU-13 holds: the installed units are never changed by a release; only the copies the hoster installs from are refreshed. Writes go by temp name and rename, inside the deploy directory the Compose unit already may write. On Compose, root now installs a unit file the deploy user can write. That costs no boundary, because that user is in the docker group (root-equivalent, accepted in AU-13), and the earlier source `/tmp/wx-$v/d` belonged to the same user. On Podman they are the service user's own user units. Covered by `test_real_deploy_sh_*` (mutant d-updater killed) |
| `umask 022` in `sign-release.sh` | Correct for the operator's side. CI's side is unverified: QA-52-r2-3 |
| Design notes added by the engineer to AU-8, AU-14, AU-16 | Accepted, small-lane style. Preference: they sit in the "Reason" column after the existing text without a separator; fold them into the decision column next time the design is touched |

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-52-r1-1 Blocker | **fixed** | `deploy.sh:31-34` no longer looks into the data directory. The real `deploy.sh` now runs under `update.sh` in the harness on the Compose layout (data folder `chmod 0`, stronger than the guides' 0750) with a fake `docker`, and on the Podman layout with a fake control script. Both apply, regenerate, and install `update.sh`, the Caddyfile and the unit copies. Putting the check back (d-site) fails both tests. The control script's host-side `$data` is used only inside `podman unshare` or the container |
| QA-52-r1-2 Major | **fixed** | AU-5 (a) to (d) and the attack table now cover the window before signing. ADR-050 item 4 is revised with history. P-9 states the residual (image bytes, tarball binary). `sign-release.sh` pins `ghcr.io/b0wter/workout-explorer:<tag>@sha256:<64 hex>`, rebuilds the archive from a detached worktree of the local tag's commit with that reference, and compares the listings (modes, owners, sizes, names; dates dropped) before it unpacks the draft and `diff -r`s the trees. The rebuild writes one reference into both `compose.yaml` and the Podman `Image=`, so a Podman mismatch is caught. My r1 probe (swapped `deploy.sh`, `ghcr.io/attacker`) is now refused, as is each case in `sign_test.py`. Test gap: r2-1 |
| QA-52-r1-3 Minor | **fixed** | m14, m17, m19 and m20 are all killed by the new tests: 5xx and dropped `.minisig`; an announced 64 MiB download stopped early; a curl without `--max-filesize`; a lock held by a child `deploy.sh` left running |
| QA-52-r1-4 Minor | **fixed** | terminal line (above), DOCKER.md and PODMAN.md "First `cat ... update.stopped`", RUNBOOK |
| QA-52-r1-5 Minor | **fixed** | HUP, INT and TERM traps, `applying` flag, RUNBOOK line; tests and mutants above |
| QA-52-r1-6 Minor | **fixed** | (a) RUNBOOK: "Leave `deploy/minisign.pub` as it is"; regeneration only after a loss, with `-f`. (b) The timer is installed from `{deploy dir}/updater/`, filled by the first install and refreshed by every deploy. (c) BAREMETAL.md downloads `SHA256SUMS.minisig` and runs `minisign -V -P` and the name check before `sha256sum -c` |

## Questions

1. **User (product, via the main session):** P-9 is still open in the design and missing from the decision log. Does the operator's signature vouch for CI's image and tarball binary (the suggested "accept the residual"), or is a local image check wanted? "Accept" settles it. The code needs no change either way for this round.

## Coverage

This round covered the diff and the six routed findings. AC 4, 11 and 17 were blocked by r1-1 and AC 9 (signing side) was open through r1-2. All four now trace to code and passing harness tests: the real `deploy.sh` on both layouts, and `sign_test.py`. Every other AC row of r1 is unchanged by this diff and was not re-verified.

**Still to do on a real host:**
- The first signed release end to end: CI draft, `sign-release.sh` accepting CI's archive (this settles r2-3), publication, and `latest.yml` moving only upwards.
- The DNS TXT record matching `deploy/minisign.pub`.
- The Compose units installed as root from `/srv/workout-explorer/updater/system` with a real `deploy` user: `journalctl` lines, writes outside the deploy directory failing, `ss -ltn` before and after, no secret in `ps` during an alert, and a run killed by `systemctl stop` logging `failed deploy` or `killed`.
- A refused Caddy patch leaving the stack and the tag unchanged.
- Rootless Podman throughout: user timer, `sudo -u workouts update.sh`, no root process, the pause process not holding the lock.
- One nightly run over a full interval with the switch off and with it on.
