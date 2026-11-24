# F-70 Report notifications through other services

- **Status:** planned (user, 24 Nov 2026); listed last in the build order; open questions OQ-338 to OQ-345 open
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** backend (settings, sender, About page block, deploy docs)
- **Lane:** critical: each notification carries a single-use delete link (INF-6), a credential, to a new destination; new secrets in the hoster's configuration
- **Requirements:** INF-4, INF-6; INF-1, INF-2, INF-8, INF-9; NFR Privacy (third parties), NFR Security
- **Depends on:** F-27 (reports, Pushover), F-37 (links, delete keys, ADR-042), F-29 (About), F-59 and F-60 (install paths), F-52 (updater alerts)

## Goal
A hoster who does not use Pushover gets report notifications through a service they choose, such as a self-hosted or public ntfy server or their own HTTP endpoint. Operator-only and opt-in: viewers and sharers still reach no external service.

## What exists
- Pushover only (ADR-039): two settings, `WORKOUT_EXPLORER_PUSHOVER_TOKEN` and `_USER`, both or neither, otherwise exit 2 (technical design, Settings). Adapter `src/server/internal/pushover`: one try, 10 s timeout, one WARN line on failure, at most 10 messages per hour (the rest folded into the next as "(+3 more)", without delete link).
- Message: report number, reason in words, viewing link to open, delete link (INF-4, INF-6); never the title or the reporter's message. A delete key is issued only when the notification is sent; a start without Pushover revokes every key (ADR-042).
- The app has no route out unless the hoster adds one (`deploy/compose.notify.yaml`, Podman drop-in, bare-metal `notify.conf`); `deploy/example.env` and RUNBOOK "Notifications" describe Pushover.
- The updater (F-52, F-60) alerts a failed update through the same Pushover settings.
- The About page names Pushover and what it learns (INF-1).

## Scope
- **Services** (OQ-338): Pushover stays; added: ntfy (any server, topic URL, optional access token) and a generic HTTP callback (HTTPS POST of a fixed JSON body, OQ-345). One service at a time (OQ-339).
- **Content:** the same for every service: report number, reason in words, viewing link, delete link, "(+N more)" folding; never the title, the reporter's message or anything about the reporter (INF-4). Same 10-per-hour limit and same delete-key rules (INF-6).
- **Configuration** (OQ-340, OQ-342): environment settings only, read once at start with strict formats; a wrong or incomplete combination exits 2 naming the setting, never its value. Off by default.
- **Failure handling** (OQ-341): a report is stored and confirmed whether or not its notification is sent (INF-4).
- **Updater alerts** (OQ-343) and **revocation** of delete links when the service changes (OQ-344).
- **Disclosure:** the About page names the chosen service (or "your own server" for a callback) and what it learns (INF-1); RUNBOOK, `example.env` and the three install paths explain each service and the route out.

## Out of scope
- Email (no email, settled), SMS, chat services with their own APIs (Matrix, Telegram, Slack) except through the callback.
- Several services at once (unless OQ-339 changes it); any notification to sharers or viewers; any inbound endpoint.
- Notifications for anything but new reports and failed updates.

## Constraints
- **No external service for users:** nothing a viewer or sharer does contacts the chosen service except through the stored report; the browser never does.
- **Secrets:** tokens, and ntfy topic URLs (on a public server the topic name is the secret), never appear in logs, error messages, the About page or `ps`; the env file stays mode 0600.
- **Egress:** the same route-out overrides as today; a service on the same host or LAN must be reachable without opening the app to the internet if the design can manage it (the design decides how).
- **Reliability:** a slow, failing or redirecting endpoint never delays the report response or blocks other reports; no retry storm.

## Acceptance criteria
- [ ] With ntfy set, a new report arrives as an ntfy message with number, reason, a click action to the viewing link and the delete link as an action or in the body (lock screen shows no key, as F-37 AC 41); title and reporter message never appear.
- [ ] With a callback set, the endpoint receives one HTTPS POST per report with the fixed JSON body (OQ-345) and the configured token; nothing else.
- [ ] Pushover works unchanged; existing `.env` files need no edit.
- [ ] Two services set, an invalid URL, `http://` where not allowed (OQ-340) or a token without a URL: exit 2 naming the setting, value never printed.
- [ ] Endpoint down, timing out (10 s), answering 4xx/5xx or redirecting: report stored and confirmed, one WARN line without secret, no delete key issued for that report.
- [ ] Rate limit: the 11th report within an hour is folded into the next message as today, on every service.
- [ ] Delete links revoked per OQ-344; updater alerts per OQ-343.
- [ ] About page names the service; RUNBOOK, `example.env`, DOCKER, PODMAN and BAREMETAL docs cover it; tests with a local fake endpoint per service, none contacting a real service in CI.

## Open questions
- [ ] **OQ-338 Which services.** Suggest: ntfy and a generic HTTPS callback, besides Pushover. Gotify, Matrix and others go through the callback or a small bridge the hoster runs. Alternative: also Gotify natively. Affects Scope, AC 1, AC 2.
- [ ] **OQ-339 One or several at once.** Suggest: exactly one service; setting two exits 2. Alternative: several, each gets the same message and the same delete link. Affects INF-6 (one link reaching more places), AC 4.
- [ ] **OQ-340 Settings and URLs.** Suggest: `WORKOUT_EXPLORER_NOTIFY_URL` (ntfy topic URL or callback URL) with `WORKOUT_EXPLORER_NOTIFY_KIND` (`ntfy` or `callback`) and an optional `WORKOUT_EXPLORER_NOTIFY_TOKEN` (sent as a bearer token); HTTPS only, except `http://` to a loopback or private address (self-hosted on the same host or LAN). Pushover keeps its two settings. Alternative: HTTPS only, no exception. Affects technical design (Settings), AC 4.
- [ ] **OQ-341 Failure handling.** Suggest: as Pushover today: one try, 10 s timeout, no redirects followed, one WARN line, no retry, no queue. Alternative: one retry after a minute. Affects INF-4, AC 5.
- [ ] **OQ-342 Secret handling.** Suggest: the whole URL and the token are treated as secrets (logs show only the kind and the host); no secret file support beyond the env file. Alternative: also a `_FILE` variant for container secrets. Affects RUNBOOK, AC 4, AC 5.
- [ ] **OQ-343 Updater alerts.** Suggest: the updater (F-52, F-60) uses the same chosen service for its failure alert. Alternative: updater alerts stay Pushover-only. Affects INF-8, F-52, F-60.
- [ ] **OQ-344 Revoking delete links on a change.** Suggest: as today, every key is revoked at a start with notifications off; RUNBOOK tells the hoster to turn notifications off for one start when switching service or after a leaked topic or token. Alternative: also revoke automatically when the service or URL changed (needs a stored fingerprint of the destination). Affects INF-6, ADR-042, AC 7.
- [ ] **OQ-345 Callback body.** Suggest: a fixed JSON object `{"report": 12, "reason": "Personal data", "more": 0, "view_url": "…", "delete_url": "…"}`, `delete_url` absent when no key was issued; no templating. Alternative: a hoster-set template. Affects AC 2.
