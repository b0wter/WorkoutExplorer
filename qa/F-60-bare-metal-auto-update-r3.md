# QA F-60 Automatic updates for bare-metal installs - round 3

- **Date:** 2026-11-18
- **Verdict:** ready with conditions. QA-60-r2-1 is closed: root no longer takes a version from anything `deploy` writes, and a layout check runs before any `sudo install` in sections 2, 6 and 7. `update.sh` refuses a pre-layout release on bare metal, with a test that kills its mutants. r2-2 to r2-4 are fixed. Three new minors. Condition, as in r1 and r2: the real Debian 13 host run listed under Coverage before the first bare-metal release.
- **Inputs:** diff 0266a0e..bf45396 on a clean tree: `architect/F-60-bare-metal-auto-update.md` (BM-10, BM-11), `deploy/BAREMETAL.md`, `deploy/PODMAN.md`, `deploy/RUNBOOK.md`, `deploy/update.sh`, `deploy/test/update_test.py`. Routed findings: `qa/F-60-bare-metal-auto-update-r2.md` (QA-60-r2-1 Major, r2-2 to r2-4). Also read: `git show v0.9.1:deploy/systemd/workout-explorer.service`.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Go: every package ok. Vitest: 1,926 passed (74 files). Entry edit.html 57,205/60,000. `caddy-check` ok on caddy:2.11 = v2.11.9. `docker-check` ok. `update_test.py` 50 tests OK, `sign_test.py` 12 tests OK |
| Mutants of the new guard in `update.sh` (scratchpad copies, `WX_TEST_UPDATE=… update_test.py -k bare`) | **7 of 8 killed** by `test_bare_metal_refuses_release_without_etc_layout`: guard removed; `bare = no` inverted; guard moved after the tarball fetch (killed by the no-`linux-amd64`-request assertion); grep weakened to any `EnvironmentFile=`; `stop_` (writes stopped state) instead of `done_`; exit 1 instead of 2; reason `archive` instead of `config`. **Survived:** `grep -q` without `-x`. Equivalent for every signed release: none has the line commented or prefixed |
| The documented layout check against real units | v0.9.1's `workout-explorer.service`: stops. Current `deploy/systemd/workout-explorer.service`: passes. A release whose archive has no such file also stops (grep exits 2) |
| Probe: section 6's block pasted into a nested shell (as after `sudo -i`), with a pre-layout unit | Inner shell prints "release too old for this guide: stop" and exits; the next pasted line runs in the outer shell with `w` empty (QA-60-r3-1) |

No container run this round: the diff adds no unit, rule or privilege; r2's container probe (v0.9.1's unit plus a planted symlink leaks root's secrets) is the attack that the layout check now refuses before installation. Not run: `dpkg -s` on Debian (engineer did not either), `sudoedit` on a real host, a real exit-5 cycle.

## Is r2-1 closed? The `/api/health` question

Yes. A compromised `deploy` controls the binary and the Caddyfile, so the version `/api/health` reports may lie. That does not matter, because the check carries no security weight:

- The version root installs comes from the Releases page (GitHub plus the pinned signature), not from `deploy`.
- The `grep -qx` layout check runs on the hand-verified copy in `"$w"` before any `sudo install`. It refuses every release whose app unit reads `.env` from `/srv` (v0.9.1 and older, probe above), whatever the running version claims.
- A lie that reports a higher version only makes the hoster stop. `deploy` can already stop updates, so this gives it nothing new. A lie that reports a lower version lets the hoster go on with the newest genuine release, which is the safe result.

So the layout check and "version from the Releases page" are the guard. The `/api/health` compare is only a reminder to the hoster (see r3-2).

The `update.sh` refusal is sound as a second line. The check runs in the *installed* `update.sh` after signature and archive checks and before the tarball fetch. A missing file fails closed. It exits 2 with no stopped state, no change and no `systemctl` call (all asserted). On bare metal, exit 5 therefore never names a pre-layout release. It does not defend against a compromised `deploy`, which can run anything as itself anyway. It protects an honest hoster who rolls back too far, and it keeps the exit-5 alert from pointing at old units.

## Findings

### Minor

- **ID:** QA-60-r3-1
- **Severity / kind:** Minor / risk (docs)
- **Route:** backend
- **Where:** `deploy/BAREMETAL.md:49`, `:112` (`|| { echo "release too old for this guide: stop"; exit 1; }`), with `:109` ("As root, download and check that release")
- **Relates to:** BM-10, QA-60-r2-1
- **Evidence:** The stop is `exit 1` in a pasted block. Section 6 says "As root", so a hoster may run `sudo -i` first. Probe: there the `exit` leaves only the inner shell, and the rest of the pasted block runs in the outer one. With `w` unset, every `sudo install "$w"/…` fails on `/systemd/…`, which is harmless. If the outer shell still has a `w` from an earlier download, root installs from that folder unchecked. In a plain SSH login the `exit` ends the session, which is safe but surprising.
- **Consequence:** The control works only when the reader's shell setup is the expected one. Cheap fix: make the install lines depend on the check (`grep -qx … && { sudo install …; }`, or `if … then … fi` around the block) and say "your own login" instead of "As root", as section 2 does.

- **ID:** QA-60-r3-2
- **Severity / kind:** Minor / defect (docs and design wording)
- **Route:** architect (one BM-10 sentence), then backend (BAREMETAL.md section 6)
- **Where:** `deploy/BAREMETAL.md:109` ("the newest release … If that version is older than the running one (`curl … /api/health`), stop"); `architect/F-60-bare-metal-auto-update.md:36` (BM-10 "stops if it is older than the running one")
- **Relates to:** BM-10, RUNBOOK.md "By hand" (`update.sh` installs "also an older one"), AC 30
- **Evidence:** As shown above, the `/api/health` version is served by `deploy`'s binary, and the layout check is what protects. Yet BM-10 lists the version compare as part of the rule, as if it were a guard. Section 6 also requires "the newest release". A hoster who rolls back on purpose (`update.sh v1.4.0` after a bad v1.5.0) to a release whose units differ gets exit 5. Section 6 then leads them to install v1.5.0's root files and run `update.sh` with v1.5.0. A rollback across a root-file change has no documented path, though RUNBOOK promises older releases. Only `deploy`-named versions are unsafe. A version the hoster chose that passes the layout check is safe.
- **Consequence:** A design line that overstates a check `deploy` can spoof, and no rollback path when the release is broken. Fix: BM-10 says "a version the hoster chooses (Releases page, never `deploy`'s state, log or alert), refused by the layout check if older than the layout". The health compare becomes an optional "older than running: only if you mean to roll back".

- **ID:** QA-60-r3-3
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/BAREMETAL.md:14` (`tee ~/wx-new-packages.txt`), `:200`, `:203`-`:212`
- **Relates to:** AC 35 (removal "without trace"), QA-60-r2-2
- **Evidence:** The r2-2 fix writes a note file to the hoster's home. Section 8 reads it in step 7, but neither removes it nor lists it in "Check that nothing is left".
- **Consequence:** One file the guide created survives the full removal. Fix: `rm ~/wx-new-packages.txt` after step 7. (Side note: `dpkg -s` exits 0 for a package left in the `config-files` state, so such a package is treated as already installed and kept. That is the safe direction; no change needed.)

## Previous findings

| ID | Status | How verified |
| --- | --- | --- |
| QA-60-r2-1 | **fixed** | (a) Exit 5: the version comes from the Releases page, with `update.stopped`, the log line and the alert named as untrusted. The layout check runs before the first `sudo install` (`BAREMETAL.md:109`, `:112`). (b) Migration: step 3 names the first release with the `/etc` layout and runs the same check (`:167`). Section 2 has it too (`:49`). BM-10 and BM-11 updated. `update.sh` refuses pre-layout releases on bare metal: test plus 7 of 8 mutants killed, 1 equivalent. Check run against the real v0.9.1 unit: stops. Wording issues in r3-1 and r3-2 (Minor) |
| QA-60-r2-2 | **fixed** | The `dpkg -s` loop runs before `apt install` and writes `~/wx-new-packages.txt`, which section 8 step 7 uses. Leftover note: r3-3 |
| QA-60-r2-3 | **fixed** | PODMAN.md uses `sudo install -o root -g workouts -m 0640 "$w"/d/example.env …` (replaces a link; `"$w"/d` is section 3's `mkdir d` inside `$w`), then `sudoedit`, and states the rule once. BM-10's Podman sentence corrected. Engineer's note accepted: later "set X in `.env`" lines do not repeat `sudoedit`, but the rule is stated where the file is created |
| QA-60-r2-4 | **fixed** | RUNBOOK.md now sends bare-metal readers to BAREMETAL.md section 2 for `operator/`, "not with the commands below", and gives the `/etc` path for `WORKOUT_EXPLORER_OPERATOR_FILE` |

r2 Question 1 (may root install a release older than the layout or the running one?) is answered in BM-10 and BM-11. The answer to "older than the running one" needs rewording (r3-2).

## Questions

None that block. r3-2 needs one architect sentence; if the user wants no bare-metal rollbacks across a root-file change, RUNBOOK.md should say so instead.

## Coverage

Diff-only round. r1's AC table and r2's changes still apply. Changes:

| AC | Change in this diff | Status |
| --- | --- | --- |
| 28 exit codes | bare metal: pre-layout release is exit 2, `config`, no stopped state | traced: test, mutants |
| 30 root-only differences | exit 5 no longer names a pre-layout release; root's version comes from the Releases page | traced: test (update.sh); docs read; r3-2 |
| 31 no root-equivalence | layout check before every documented root install from a release | **holds** (docs read, check run against real v0.9.1 and current units); r3-1 is a shell-setup caveat |
| 35 install, migration, removal | package note before `apt`; migration names the first layout release | read; r3-3; real host still needed |

**Condition, needs a real Debian 13 host (unchanged from r2):** fresh install, first `update.sh` start, nightly timer, Pushover with the outbound block, `ps` without a root process, writes to `/etc`, `/usr/local/sbin` and `/srv/workouts` failing as `deploy`, `sudo -u deploy` with `use_pty`, the section 7 migration on a hand-updated v0.9.x install with workouts, drafts, reports and links (with the first release that has the layout, which is not tagged yet), `sudoedit /etc/workout-explorer/.env`, the `dpkg -s` loop, a real exit-5 cycle with `apt install caddy` and `try-restart`, and the section 8 removal with its listing and `find -nouser`.
