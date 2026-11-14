# F-52 Optional automatic updates for hosters

- **Status:** not started (open questions OQ-210 to OQ-217)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** backend
- **Lane:** critical (code that installs and runs new code on the server; a new signing key; the update path is an attack path)
- **Requirements:** INF-8 (new), INF-4 (alerts), non-goals and NFR Privacy (second opt-in third party: the release host), NFR Security, UP-7 and AC-4 (a failed rebuild may delete workouts, as on a manual deploy, OQ-53); technical design: Hosting and operations (Deploys), Configuration, Security and privacy
- **Depends on:** F-31 (deploy, release layout), F-30 (regenerate in the deploy), F-37 (Pushover notifier)

## Goal
A hoster who opts in gets patch releases installed without doing anything, and is told when something newer needs a manual step or when an update failed. A forged or tampered release is never installed.

## Scope
- **Opt-in.** Nothing is installed or running by default. The hoster installs the updater by hand, following the runbook, and it does nothing until it is turned on.
- **Polling only.** A schedule on the host (interval: OQ-213) fetches the release list over HTTPS. No inbound port or endpoint, no webhook, no new route in Caddy or Go.
- **Authenticity.** Releases are signed with a key kept outside GitHub (never in CI, repository secrets or release assets). The server verifies the signature against a public key the hoster pinned at install, before anything is unpacked or run. The signature covers everything that gets installed or run: binary, web bundle, and the deploy files the update uses (`deploy.sh`, compose files, Caddyfile, the updater itself), plus the version (OQ-210, OQ-211). The release's `SHA256SUMS` is only an integrity check and is never enough on its own.
- **Which versions.** It applies automatically only a release with the same major and minor version as the running one and a higher patch version (v1.4.0 to v1.4.2, never v1.4.2 to v1.4.0). A newer minor or major version is only reported, once per version. Pre-releases, tags not in `vX.Y.Z` form, older versions and the running version itself are ignored.
- **Apply.** It runs the same steps as a manual deploy (`deploy.sh` on the server itself, or an equivalent the architect decides), so the health wait, `regenerate` and the clean-up of old bundles happen as usual.
- **Failure.** If an update fails (deploy exit 1 refused or not healthy, 3 regenerate not finished, 4 clean-up failed), or if a release fails signature checking, the updater sends an alert through the existing Pushover notifier when it is set up (INF-4), otherwise to the log. It then records a stopped state: no further automatic update runs until the hoster deploys by hand successfully, which clears the state. Reports about newer minor or major versions keep coming while stopped (OQ-214).
- **Least privilege.** The updater runs as the user that runs `deploy.sh` (the owner of the deploy directory, in the docker group), never as root and without sudo. It writes only where a manual deploy writes, plus its own state file. The app container gets no internet access for this (it keeps none unless notifications are on); the release download happens outside the app container.
- **One at a time.** An automatic update and a manual deploy never run at the same time.
- **Logging.** Each run logs one line: versions seen, action taken (none, applied, reported, refused, failed), exit status. No workout or visitor data.
- **About page (INF-1):** the third-party list says, only when the updater is on, that the server contacts the release host to download updates and sends it nothing but ordinary requests (OQ-216).
- **Release process:** the operator signs each release locally after CI has built it and uploads the signature as a release asset; the runbook says how (OQ-212).
- **Runbook:** install, turn on, turn off, pin and change the public key, read the state, clear a stopped state by a manual deploy, uninstall.
- Technical design updates (architect): Deploys, Configuration, Security and privacy (signing key, updater rights), third-party list.

## Out of scope
- Automatic minor or major updates. Rollback to an earlier version (ADR-019: no rollback). Updating Docker, the OS or the host's reverse proxy. Updating self-hosts that build their image from a clone (OQ-215). Any inbound trigger. Update notifications to services other than Pushover. Key rotation beyond "pin a new key by hand" (OQ-217).

## Acceptance criteria
- [ ] On a fresh install that follows the runbook without the updater steps, no updater is installed, no timer runs, and no request to the release host is made (checked over a full check interval).
- [ ] Installed but turned off, the updater makes no request to the release host.
- [ ] No new listening socket appears on the host (`ss -ltn` before and after install), and no new route answers in Caddy or Go.
- [ ] A release with a valid signature, same major and minor version and a higher patch version is applied: afterwards `/api/health` reports the new version, `regenerate` has run and the log line says "applied".
- [ ] A release with a higher minor or major version is not applied; one notification (Pushover when set up, else the log) names the version, and the next runs do not repeat it for the same version.
- [ ] An older version, the running version, a pre-release and a tag not in `vX.Y.Z` form are never applied.
- [ ] A release with no signature, a signature by another key, a signature that does not match any covered file, or a valid signature for a different version (a replayed old signature) is refused before anything is unpacked or run; an alert is sent and the stopped state is set.
- [ ] A release whose `SHA256SUMS` matches its files but has no valid signature is refused (checksums alone are not proof).
- [ ] A changed `deploy.sh`, compose file, Caddyfile or updater in the release that is not covered by the signature is never run.
- [ ] A deploy that ends with exit 1, 3 or 4 sends one alert through Pushover when set up (else the log) naming the version and the exit status, and sets the stopped state; while stopped, no automatic update is applied, even for a newer valid patch.
- [ ] A successful manual deploy clears the stopped state; the next run applies updates again.
- [ ] The updater process never runs as root and never calls sudo; it writes only to the deploy directory, the data directory as a manual deploy does, and its own state file.
- [ ] The app container still has no outbound route unless notifications are on (`compose.notify.yaml`); the release download does not go through it.
- [ ] An automatic update started while a manual deploy runs (or the other way round) waits or skips; the two never overlap.
- [ ] The signing key's private part appears nowhere in the repository, CI configuration, CI logs or release assets.
- [ ] Without network access, or when the release host answers with an error, the run logs it and changes nothing; this is not a failure that sets the stopped state.
- [ ] The runbook covers install, turning on and off, pinning the public key, clearing a stopped state and uninstalling; a hoster following it from scratch gets a working updater.

## Open questions
- [ ] **OQ-210 Signing tool and key.** Suggested: minisign (or signify), one Ed25519 key held offline by the operator; the hoster pins the public key in the updater's settings at install. Its fingerprint is published on GitHub and in a second place GitHub does not control (e.g. the operator's own instance's About page). Affects the release process and the runbook.
- [ ] **OQ-211 What is signed.** Suggested: sign one manifest (`SHA256SUMS` listing every release file, including a new deploy-files archive), with the tag inside the signed text, so the version cannot be swapped. Affects the release assets and CI.
- [ ] **OQ-212 Who signs, when.** Suggested: the operator signs locally after CI has built the release and uploads `SHA256SUMS.minisig`; an unsigned release is simply never auto-applied. Affects the release routine.
- [ ] **OQ-213 Check interval and time.** Suggested: once a day at a random minute in a night hour the hoster sets, via a systemd timer (or cron) as the deploy user. Affects the runbook and the brief downtime of a deploy.
- [ ] **OQ-214 Patches only while pre-1.0?** With versions v0.x, each feature release bumps the minor version, so only fixes (v0.8.0 to v0.8.1) are ever applied automatically. Suggested: accept; the same-minor rule stays as stated. Affects how useful the feature is before 1.0.
- [ ] **OQ-215 Which installs it supports.** Suggested: only the release layout (runbook section 6a and the operator's deploy), not self-hosts that build from a clone. `deploy.sh` today needs a git checkout at the tag and `gh`; the architect decides whether the updater uses a signed deploy-files archive instead. Affects the runbook and `deploy.sh`.
- [ ] **OQ-216 About page and privacy wording.** Suggested: when the updater is on, the About page's third-party list adds one line ("This server downloads software updates from GitHub; no workout or visitor data is sent"). Needs a setting the app can read. Affects INF-1 and the operator file.
- [ ] **OQ-217 Lost or compromised signing key.** Suggested: no automatic rotation; the operator announces a new public key and hosters pin it by hand; until then updates are refused and alerted. Affects the runbook.
- [ ] **Docker group (accepted risk, confirm).** `deploy.sh` needs the docker group, which is effectively root on the host; the updater inherits exactly that and nothing more. Suggested: accept; rootless Podman hosts (PODMAN.md) keep their lower rights. Affects the security section of the runbook.
