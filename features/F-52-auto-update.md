# F-52 Optional automatic updates for hosters

- **Status:** ready for architecture (after F-50); OQ-210 to OQ-217 decided 14 Nov 2026. Built after F-59; design revised for the published image (`architect/F-52-auto-update.md` v0.2, 16 Nov 2026), P-7 and P-8 decided 16 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** backend
- **Lane:** critical (code that installs and runs new code on the server; a new signing key; the update path is an attack path)
- **Requirements:** INF-8 (new), INF-4 (alerts), non-goals and NFR Privacy (second opt-in third party: the release host), NFR Security, UP-7 and AC-4 (a failed rebuild may delete workouts, as on a manual deploy, OQ-53); technical design: Hosting and operations (Deploys), Configuration, Security and privacy
- **Depends on:** F-59 (published image, install paths), F-31 (deploy, release layout), F-30 (regenerate in the deploy), F-37 (Pushover notifier)

## Goal
A hoster who opts in gets every new release installed without doing anything, and is told when an update failed. A forged or tampered release is never installed.

## Scope
- **Opt-in.** Nothing is installed or running by default. The hoster installs the updater by hand, following the runbook, and it does nothing until it is turned on.
- **Polling only.** A schedule on the host (interval: OQ-213) fetches the release list over HTTPS. No inbound port or endpoint, no webhook, no new route in Caddy or Go.
- **Authenticity.** Releases are signed with a key kept outside GitHub (never in CI, repository secrets or release assets). The server verifies the signature against a public key the hoster pinned at install, before anything is unpacked or run. The signature covers everything that gets installed or run: binary, web bundle, and the deploy files the update uses (`deploy.sh`, compose files, Caddyfile, the updater itself), plus the version (OQ-210, OQ-211). The release's `SHA256SUMS` is only an integrity check and is never enough on its own.
- **Which versions.** It installs the latest signed release whenever it is newer than the running one, whatever changed (patch, minor or major; OQ-214). Pre-releases, tags not in `vX.Y.Z` form, older versions and the running version itself are ignored.
- **Apply.** It runs the same steps as a manual deploy: the verified release's `deploy.sh`, which on Compose follows the F-59 steps and on rootless Podman uses `workout-explorer-ctl`. Before anything changes, the pulled Caddy patch must pass the Caddy guard against the new Caddyfile; a refused Caddy patch counts as a failed update. Then the health wait, `regenerate` and the clean-up of old bundles happen as usual.
- **Failure.** If an update fails (deploy exit 1 refused, not healthy or Caddy patch refused, 3 regenerate not finished, 4 clean-up failed), or if a release fails signature checking, the updater sends an alert through the existing Pushover notifier when it is set up (INF-4), otherwise to the log. It then records a stopped state: no further automatic update runs until the hoster deploys by hand successfully, which clears the state. While stopped, it sends no further alerts for the same failure.
- **Least privilege.** Never as root, never with sudo. Docker Compose: the updater runs as the user that runs `deploy.sh` (the owner of the deploy directory, in the docker group). Rootless Podman: the service user owns the deploy files, units and `workout-explorer-ctl` and runs the updater from a user timer (P-7). It writes only where a manual deploy writes, plus its own state file. The app container gets no internet access for this (it keeps none unless notifications are on); the release download happens outside the app container.
- **One at a time.** An automatic update and a manual deploy never run at the same time.
- **Logging.** Each run logs one line: versions seen, action taken (none, applied, refused, failed), exit status. No workout or visitor data.
- **About page (INF-1):** the third-party list says, only when the updater is on, that the server contacts the release host to download updates and sends it nothing but ordinary requests (OQ-216).
- **Release process:** the operator signs each release locally after CI has built it and uploads the signature as a release asset; the runbook says how (OQ-212).
- **Runbook:** install, turn on, turn off, pin and change the public key, read the state, clear a stopped state by a manual deploy, uninstall.
- Technical design updates (architect): Deploys, Configuration, Security and privacy (signing key, updater rights), third-party list.

## Out of scope
- Rollback to an earlier version (ADR-019: no rollback). Updating Docker, the OS or the host's reverse proxy. Updating self-hosts that build their image from a clone (OQ-215). Bare-metal installs (F-59): planned as F-60, after F-52. Any inbound trigger. Update notifications to services other than Pushover. Key rotation beyond "pin a new key by hand" (OQ-217). Notify-only or same-minor modes.

## Acceptance criteria
- [ ] On a fresh install that follows the runbook without the updater steps, no updater is installed, no timer runs, and no request to the release host is made (checked over a full check interval).
- [ ] Installed but turned off, the updater makes no request to the release host.
- [ ] No new listening socket appears on the host (`ss -ltn` before and after install), and no new route answers in Caddy or Go.
- [ ] The latest release with a valid signature and a higher version is applied, whether patch, minor or major (e.g. v1.4.0 to v1.4.2, v1.5.0 and v2.0.0 each tested): afterwards `/api/health` reports the new version, `regenerate` has run and the log line says "applied".
- [ ] With several newer releases out, it installs the latest one directly.
- [ ] An older version, the running version, a pre-release and a tag not in `vX.Y.Z` form are never applied.
- [ ] A release with no signature, a signature by another key, a signature that does not match any covered file, or a valid signature for a different version (a replayed old signature) is refused before anything is unpacked or run; an alert is sent and the stopped state is set.
- [ ] A release whose `SHA256SUMS` matches its files but has no valid signature is refused (checksums alone are not proof).
- [ ] A changed `deploy.sh`, compose file, Caddyfile or updater in the release that is not covered by the signature is never run.
- [ ] A deploy that ends with exit 1, 3 or 4 sends one alert through Pushover when set up (else the log) naming the version and the exit status, and sets the stopped state; while stopped, no automatic update is applied, even for a newer valid patch.
- [ ] A successful manual deploy clears the stopped state; the next run applies updates again.
- [ ] The updater process never runs as root and never calls sudo; it writes only to the deploy directory, the data directory as a manual deploy does, and its own state file; on rootless Podman also the service user's unit directory and image storage.
- [ ] The app container still has no outbound route unless notifications are on (`compose.notify.yaml`); the release download does not go through it.
- [ ] An automatic update started while a manual deploy runs (or the other way round) waits or skips; the two never overlap.
- [ ] The signing key's private part appears nowhere in the repository, CI configuration, CI logs or release assets.
- [ ] Without network access, or when the release host answers with an error, the run logs it and changes nothing; this is not a failure that sets the stopped state.
- [ ] The runbook covers install, turning on and off, pinning the public key, clearing a stopped state and uninstalling; a hoster following it from scratch gets a working updater.

## Decisions (user, 14 Nov 2026)
- **OQ-210 Signing tool and key:** minisign (Ed25519), private key held offline by the operator; the hoster pins the public key in the updater's settings at install; its fingerprint is published on GitHub and in a second place GitHub does not control: a DNS TXT record on the operator's domain (decided 15 Nov 2026).
- **OQ-211 What is signed:** one manifest (`SHA256SUMS` listing every release file, including a deploy-files archive), with the tag inside the signed text.
- **OQ-212 Who signs:** the operator, locally, after CI has built the release; uploads `SHA256SUMS.minisig`. An unsigned release is never applied.
- **OQ-213 Interval:** once a day at a random minute in a night hour the hoster sets, via a systemd timer (or cron) as the deploy user.
- **OQ-214 Versions:** changed from the suggestion: the updater always installs the latest signed release, whatever its version (patch, minor or major); there is no same-minor rule and no notify-only case.
- **OQ-215 Installs:** only the release layout (runbook section 6a and the operator's deploy), not self-hosts that build from a clone; the architect decides whether a signed deploy-files archive replaces the git checkout `deploy.sh` needs today.
- **OQ-216 About page:** when the updater is on, the third-party list adds "This server downloads software updates from GitHub; no workout or visitor data is sent", from a setting the app reads.
- **OQ-217 Key loss:** no automatic rotation; the operator announces a new key, hosters pin it by hand; until then updates are refused and alerted.
- **Docker group:** accepted risk; the updater has exactly the rights of `deploy.sh` (docker group), nothing more; rootless Podman hosts keep their lower rights.
- **P-1 to P-5 (15 Nov 2026, architecture points):** every release deploy goes through `update.sh` and needs `minisign` and the pinned key (P-1); releases stay drafts until the operator signs them with a local script (P-2); a failed manual deploy also sets the stopped state, and the hoster must be told (P-3, how: open); no back-port releases (P-4); systemd timer only (P-5). P-6 (Podman) open.
- **15 Nov 2026, install paths (F-59):** releases become a published image (ghcr.io) built by CI; F-59 comes first. The updater covers Docker Compose and rootless Podman on the image; bare metal later (F-60). The signed manifest (OQ-211) must cover the image digest, which F-59 lists in the release. **P-6 closed:** Podman is a supported path through F-59 and covered by the revised design. **P-3 decided:** a failed manual deploy that stops automatic updates says so in a line in the terminal and sends a Pushover alert when set up. **Tags:** the image also gets `latest`, which moves when the operator publishes the signed release, not at CI time (P-8, 16 Nov 2026), and which the updater never uses; it pulls by the digest in the signed manifest. Caddy runs from the official image by its minor tag (`caddy:2.11`), patches floating: an update pulls the newest patch of that minor; the signature covers the tag name in the deploy files, not Caddy's bytes (trust in Docker's official-image pipeline accepted). **Bare metal (F-60):** may restart its two units through polkit or one narrow sudoers line, an exception to "never calls sudo" for that path only.
- **P-7, P-8 (16 Nov 2026, design v0.2):** rootless Podman: deploy files, units and `workout-explorer-ctl` belong to the service user, which runs the updater from a user timer and the control script without sudo; `latest` moves on publication of the signed release.
- **External service:** the release host as a second hoster-only, opt-in exception (spec non-goals, NFR Privacy) is accepted.
