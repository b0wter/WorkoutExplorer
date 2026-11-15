# QA F-59 Container image and install paths - round 1

- **Date:** 2026-11-15
- **Verdict:** ready with conditions. No Blocker or Major. What is left is the real-host checks that cannot run here: the first tag release on GitHub and ghcr.io, rootless Podman, bare metal on Debian 13, and the migration rehearsal (list under Coverage). Four Minor findings.
- **Inputs:** `features/F-59-container-image-and-install-paths.md` (AC 1 to 15); `architect/F-59-container-image-and-install-paths.md` v0.1; ADR-052, ADR-053, ADR-034 rev. 5; decision log entries of 15 Nov 2026 (`features/README.md`); TD Release supply chain; F-30 RG-8 to RG-10. Commit ca85f04 compared with 871f67e. Working tree clean.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Go: 24 packages ok. Vitest: 74 files, 1,927 tests. Entry 57,208 of 60,000 B gzip. `caddy-check` ran against `docker.io/library/caddy:2.11` = v2.11.9 and passed. `docker-check: ok (sha256:9bab7078…)`: one plain OCI manifest, image file list within IP-1, labels present, user `10001:10001`, a copyright file for every base package, base package list equal to `sources.json`, deploy archive only allow-listed files |
| `go vet ./...`; `go test -race -count=1 ./internal/assets/ ./cmd/workout-explorer/` | clean; both ok (172 s) |
| Mutants on a scratch copy of `src/` (without the real corpus): the 3 mutants the design asks for, plus 6 more | **9 run, 9 killed.** Killed: Clean drops the current list; Copy removes (`RemoveAll` of `site/assets`); first list rewritten on every start; no "not on current" filter (QA-30-r1-1); regenerate cleans with `used = nil`; clean-up before the incomplete-run check; exit 0 instead of 4 on a failed clean-up; `serve` without the copy; `regenerate` without the copy. A first batch was invalid (the cmd tests panic without `src/web`). I added `src/web` and ran it again; every mutant compiles |
| Caddy file capability, `caddy:2.11` (v2.11.9) | `getcap /usr/bin/caddy` = `cap_net_bind_service=p` (no effective bit). Hardened with no `cap_add`: CapPrm, CapEff and CapBnd are 0, and it binds 8088. With `--cap-add NET_BIND_SERVICE`: **CapPrm 0x400**, CapBnd 0x400. So the engineer's deviation 1 is confirmed on Docker 29.9.3 |
| Compose stack in the scratchpad: the tested archive (`docker load`), `deploy/compose.yaml` with only the image line changed, project `wxqa`, listen on 127.0.0.1:18089, then torn down | Both containers: CapPrm, CapEff and CapBnd are 0; read-only root; `no-new-privileges`; no anonymous volume. Caddy's environment has only the 5 `WX_*` values (no `.env` key). Caddy sees only `public/`, `site/` and `conf/`: `cat workouts.db` fails. `serve` copied 70 files into `site/assets/` (0644, dir 0750) and wrote 1 list. `/assets/<name>` 200 from there; `/bundles/` 404. From `app`'s network namespace, `wget 1.1.1.1` gives "Network unreachable" and DNS gives SERVFAIL. The only new listener is 127.0.0.1:18089. N to N+1, simulated: a kept list named by a page in `public/` stayed whole (entry and lazy chunk); an unnamed list went (`files_removed=1 bundles_removed=1`, exit 0). After a `caddy reload` and an `app` restart, the lists were unchanged |
| The runbook's section 8 `caddy validate` line, run literally (no `WX_DATA`) | "Valid configuration" |

Not run: anything on GitHub or ghcr.io (no push, tag or publish), Podman, skopeo, apt Caddy, systemd units, `deploy.sh` end to end.

## Findings

### Minor

- **ID:** QA-59-r1-1
- **Severity / kind:** Minor / risk
- **Route:** architect (record the decision), backend (guards)
- **Where:** `deploy/compose.yaml:105`, `deploy/podman/workout-explorer-caddy.container:26-28`, `deploy/podman/workout-explorer-ctl:53`, `deploy/RUNBOOK.md:342` ("Between releases, `docker compose pull && docker compose up -d`"), `deploy/PODMAN.md:145`, `deploy/deploy.sh:58-61`
- **Relates to:** AC 6, AC 8, ADR-034 rev. 5 item 6, IP-7, OQ-265 (decision log 15 Nov 2026)
- **Evidence:** I accept deviation 1. It is the only setup that meets AC 8: the probe above shows that IP-7's `cap_add` leaves `NET_BIND_SERVICE` in CapPrm under Docker. IP-7's claim "CapPrm and CapEff 0" is therefore false, and so is the runbook's own check in section 7. The deviation is not free, though. Because the patch floats, Caddy now starts only while the official binary's file capability has no effective bit. It had one until 2.11.7, the engineer says. If a patch brings it back, the kernel refuses to exec the binary with an empty bounding set. `deploy.sh` and `ctl install` test for this before changing anything. Four update routes do not: RUNBOOK section 8's "between releases" line, PODMAN section 9's `ctl pull && ctl restart`, a Podman reboot after `ctl pull` (Quadlet runs the tag again), and F-52 later. Also, `deploy.sh` and `ctl install` pull the tag before they validate, so a refused patch stays the local `caddy:2.11`. The next `docker compose up -d` (for example after a `.env` edit) recreates Caddy on it. ADR-034 item 6, its Consequence and its "would choose differently" line, IP-7, the IP-15 table and the OQ-265 decision-log entry still describe the bounding capability.
- **Consequence:** Today, none. If a patch restores the effective bit, Caddy goes into a restart loop and every page and the API are down on any host that took one of the unguarded routes. The documents describe a setup the code does not use.

- **ID:** QA-59-r1-2
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `.github/workflows/ci.yml:8-10` and `:109-120`
- **Relates to:** AC 1 ("pushing an older version than the newest never moves `latest`"), IP-4 step 4
- **Evidence:** The concurrency group is `ci-${{ github.ref }}`, one per tag. Two tags pushed close together (say v1.3.0, then v1.2.1 as a fix) run their `release` jobs in parallel. Step 4 reads `latest`'s label and moves the tag later, with no lock between the two. If the job for v1.2.1 reads before the v1.3.0 job moves `latest`, it moves `latest` back to v1.2.1. I found this by reading the workflow; I did not reproduce it (it needs the registry).
- **Consequence:** `latest` points at an older release until the next release. Hosters are told not to use `latest` in Compose (OQ-264), so the effect is small. Fix: a fixed concurrency group on the `release` job, without cancel (`group: release`, `cancel-in-progress: false`).

- **ID:** QA-59-r1-3
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/internal/assets/assets.go:105-149`
- **Relates to:** INF-9 "a page published under an older release keeps all its scripts and styles", AC 13, AC 14, IP-17
- **Evidence:** `Clean` keeps only names that are on a kept list. A file in `site/assets/` that a served page names, but that no list has, is removed. This happens when the hand copy of the old `web/assets/` (RUNBOOK section 15 step 3, PODMAN section 9 step 1) is done after the first start of the image: `bundles/` already exists, so no `pre-f59` list is made. A kept workout (`kept` > 0) then loses its assets at the first clean-up. The runbook order prevents this, but nothing enforces it. Read from the code; the rule is plain there.
- **Consequence:** Kept pages lose their scripts and styles until a later run rebuilds them. One line would make this impossible, because a name a served page uses is never wrong to keep: also keep every name in `used`.

- **ID:** QA-59-r1-4
- **Severity / kind:** Minor / documents
- **Route:** architect (F-30, ADRs), requirements engineer (F-31 feature file), main session (`CONTINUE.md`)
- **Where:** `architect/F-30-regenerate-command.md` RG-8 (report "the RG-9 asset names"; exit codes 0/1/2, no 4); `architect/ADR-053-…md:18` (`pre-f30`; the code uses `pre-f59`, deviation 3); `architect/ADR-050-…md:12` (`caddy.env` from the checkout); `features/F-31-server-deploy-and-monitoring.md:12` ("The Caddy version comes from `deploy/caddy.env`"); `CONTINUE.md:35`, `:49`. Also, the F-59 design's AC numbers are off from IP-11 onward: IP-11's "AC 13" is AC 15, IP-16's "AC 11" is AC 12 and IP-17's "AC 12" is AC 14, although the design says "AC n is the n-th acceptance criterion"
- **Relates to:** deviations 2, 3, 11; CLAUDE.md "one home per fact"
- **Evidence:** grep for `caddy.env|fetch-caddy|clean-assets|asset=|pre-f30` across live documents. These can stay as they are: the decision-log history entries (`features/README.md:394`, `:770`, superseded by `:772`), and F-52 AU-1 and AU-3, which F-59's "What the F-52 revision must change" already lists.
- **Consequence:** Live documents describe the removed `asset=` report, a list name the code does not use, and the removed Caddy pin.

## Engineer's deviations

| # | Judgement |
| --- | --- |
| 1 Caddy with no capability | Accepted: verified, and stricter than IP-7, whose premise is wrong under Docker. Follow-up in r1-1 |
| 2 `files_removed=` / `bundles_removed=`, exit 4 | Accepted. Tests cover it (`TestRegenerateAssets`, mutants). F-30 RG-8 is stale (r1-4) |
| 3 list key = first 8 bytes of SHA-256; `pre-f59` | Accepted (64 bits; same build gives the same key, `TestCopy`). ADR-053 says `pre-f30` (r1-4) |
| 4 `docker save` archive, uncompressed layers, one manifest | Accepted. The archive's manifest is the digest written into `compose.yaml` and checked again after the push. Still to see on the first release: ghcr.io takes uncompressed layers with `--preserve-digests` |
| 5 placeholder `:unreleased` | Accepted. `deploy-archive` writes the value exactly once in both files, and `ctl install` refuses the placeholder |
| 6 Podman notify drop-in, `Pull=missing`, patches via `ctl pull` / `install` | Accepted. `ctl pull` is not guarded (r1-1) |
| 7 bare-metal admin socket, `workout-explorer-run` | Accepted. The wrapper repeats the main sandbox lines but not all of them (no `ProtectKernel*`, `SystemCallFilter`, `ProtectProc`). Acceptable for an operator-run command |
| 8 `deploy.sh` refuses `WX_WEB`; no forced restart | Accepted |
| 9 About heading renamed | Accepted; no test or document names the old heading |
| 11 stale mentions | Partly done; the rest is in r1-4 |

## Previous findings

None (round 1).

## Questions

1. **For the user / architect (r1-1):** Keep Caddy with an empty bounding set, as the code does, and guard every pull route? Or go back to IP-7's `cap_add` and accept CapPrm `0x400` on Docker, which is robust to a patch that restores the effective bit but fails AC 8's literal check? Suggestion: keep the code and add the guard to `ctl pull` and the runbook's "between releases" line. Either answer settles it.
2. **For the architect:** On bare metal, `IPAddressAllow=localhost` lets the app reach any loopback service, for example a local DNS stub or forward proxy, with notifications off. Is that inside AC 8's "cannot open an outbound connection"? A one-line note in IP-15 would settle it. Debian 13 does not run a stub resolver by default.

## Coverage

| AC | Traced to | Verified here |
| --- | --- | --- |
| 1 release per `vX.Y.Z`, `latest` | `ci.yml` (tag regex, `needs: ci`, steps 1 to 4) | read only. r1-2 |
| 2 pushed = tested digest; re-run never overwrites | `docker-check` on the archive, `--preserve-digests`, `--digestfile` compared, step (1) | read only. Fail-safe: trailing newlines are stripped before hashing, which can only refuse, never accept |
| 3 only `release` can push | top-level `contents: read`; `packages: write` only in `release`, which has no checkout and pins its one action by SHA | read; I could not check offline that the SHA is v7.0.0. The package setting is an operator step (RUNBOOK 13.3) |
| 4 anonymous pull, file list | `docker-check` | the file list, yes. Anonymous pull: real |
| 5 About page, licence texts, labels | `sources.json`, `checkBase`, `docker-check` | yes (`make ci`) |
| 6 Caddy by minor, no override; apt on bare metal | compose, Quadlet, `caddy.pref`, minor check in `docker-check` | yes (files); pulling a newer patch: real |
| 7 fresh VPS per path | runbook | real, all three paths |
| 8 non-root, no capability, egress, listeners | units, compose | **Compose: yes** (probe). Podman and bare metal: real |
| 9 per-client upload limit | unchanged trust chain; bare metal IP-13 | real |
| 10 Caddy's view of the data | mounts; `TemporaryFileSystem` + binds | **Compose: yes.** Podman and bare metal: real |
| 11 restart after crash, OOM, reboot; memory cap | `restart`, `Restart=always`, slice 1200M | read; real |
| 12 subcommands | ctl, `workout-explorer-run`, runbook | Compose `regenerate` exec: yes; others real |
| 13 N to N+1 assets | `assets`, `TestCopy`, `TestClean`, `TestRegenerateAssets`, mutants | yes (tests + Compose probe); the browser network log: real |
| 14 migration rehearsal | RUNBOOK 15, PODMAN 9, `deploy.sh` `WX_WEB` refusal | the documents, yes; the rehearsal: real (r1-3 concerns the step order) |
| 15 no tarball layout in the Podman guide; no Docker on bare metal | PODMAN.md rewritten; RUNBOOK 14 | yes (read) |

**Real-host checks that remain (the conditions):**
1. **First tag on GitHub.**
   - Is `skopeo` on `ubuntu-24.04`?
   - Does ghcr.io answer an unknown package with text that matches `manifest unknown|name unknown|not found`? If it does not, the first release fails red.
   - ghcr.io takes uncompressed layers.
   - The pushed digest equals the one in the deploy archive.
   - Make the package public with write access for this repository's Actions only, then pull anonymously.
   - Re-running the whole workflow is refused.
   - An older tag leaves `latest` alone.
2. **Rootless Podman 5.4 on Debian 13.**
   - Quadlet takes `GroupAdd=`, `Tmpfs=…,U` and the `.container.d` drop-in.
   - CapPrm and CapEff are 0 for both containers.
   - The upload limit is per client with `WX_TRUSTED_PROXIES=10.250.0.2` under pasta.
   - The memory cap applies.
   - No egress with notifications off; Pushover reachable with them on.
3. **Bare metal on Debian 13.**
   - The apt `/usr/bin/caddy` has no `+ep` file capability. If it has one, the unit with an empty bounding set does not start.
   - The path unit restarts Caddy on a real `apt` upgrade.
   - `IPAddressDeny` blocks the app with notifications off and `notify.conf` lifts it.
   - Caddy's unit cannot read `workouts.db` or `.env`.
   - `workout-explorer-run regenerate` and `delete` work.
4. **AC 7, 9, 11 and 13 on every path,** and the AC 14 migration rehearsal on a copy of the operator's VPS and of a Podman setup.

Probe side effects: I pulled `alpine:3` for the data-directory probes and left it. The scratch stack, its networks, the loaded `workout-explorer:ci` and the scratch data are all removed.
