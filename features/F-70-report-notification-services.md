# F-70 Report notifications through other services

- **Status:** decided (24 Nov 2026)
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
- **Services** (OQ-338): Pushover stays; added: ntfy (any server, topic URL, optional access token) and a generic HTTP callback. Gotify, Matrix and others go through the callback or a small bridge the hoster runs. Exactly one service at a time; setting two exits 2 (OQ-339).
- **Callback body** (OQ-345): one HTTPS POST per message with a fixed JSON object `{"report": 12, "reason": "Personal data", "more": 0, "view_url": "…", "delete_url": "…"}`, `delete_url` absent when no key was issued; no templating.
- **Content:** the same for every service: report number, reason in words, viewing link, delete link, "(+N more)" folding; never the title, the reporter's message or anything about the reporter (INF-4). Same 10-per-hour limit and same delete-key rules (INF-6).
- **Configuration** (OQ-340): environment settings only, read once at start with strict formats; off by default. `WORKOUT_EXPLORER_NOTIFY_KIND` (`ntfy` or `callback`), `WORKOUT_EXPLORER_NOTIFY_URL` (ntfy topic URL or callback URL) and optional `WORKOUT_EXPLORER_NOTIFY_TOKEN` (sent as a bearer token); Pushover keeps its two settings. HTTPS only, except `http://` to a literal loopback or private IP address (no host names, not 100.64.0.0/10; F70-Q2, user, 24 Nov 2026). A wrong or incomplete combination exits 2 naming the setting, never its value.
- **Secrets** (OQ-342): the whole URL and the token are secrets; logs show only the kind and the host. No `_FILE` variant; the env file only.
- **Failure handling** (OQ-341): as Pushover today: one try, 10 s timeout, no redirects followed, one WARN line, no retry, no queue. A failed send revokes that report's delete key, Pushover included (replaces F-37 DK-3's "a failed send keeps the key"; acknowledged by the user, 24 Nov 2026). A report is stored and confirmed whether or not its notification is sent (INF-4).
- **Updater alerts** (OQ-343): the updater (F-52, F-60) sends its failure alert through the same chosen service.
- **Revocation** (OQ-344, changed by F70-Q1, user, 24 Nov 2026): every delete key is revoked at a start with notifications off (ADR-042), and automatically at the first start after the service, URL or token changes (Pushover settings included). The server keeps only a salted hash of the settings in a small file in the data directory; the first start after the upgrade revokes the delete links still out, once. RUNBOOK tells the hoster that a switch or a changed topic or token revokes open delete links, and that after a leak they change the topic or token.
- **Disclosure:** the About page names the chosen service (or "your own server" for a callback) and what it learns (INF-1); RUNBOOK, `example.env` and the three install paths explain each service and the route out.

## Out of scope
- Email (no email, settled), SMS, chat services with their own APIs (Matrix, Telegram, Slack) except through the callback.
- Several services at once; Gotify or other services natively; a `_FILE` variant for secrets; retries or a queue; any notification to sharers or viewers; any inbound endpoint.
- Notifications for anything but new reports and failed updates.

## Constraints
- **No external service for users:** nothing a viewer or sharer does contacts the chosen service except through the stored report; the browser never does.
- **Secrets:** tokens, and ntfy topic URLs (on a public server the topic name is the secret), never appear in logs, error messages, the About page or `ps`; the env file stays mode 0600.
- **Egress:** the same route-out overrides as today; a service on the same host or LAN must be reachable without opening the app to the internet if the design can manage it (the design decides how).
- **Reliability:** a slow, failing or redirecting endpoint never delays the report response or blocks other reports; no retry storm.

## Acceptance criteria
- [ ] With ntfy set, a new report arrives as an ntfy message with number, reason, a click action to the viewing link and the delete link as an action or in the body (lock screen shows no key, as F-37 AC 41); title and reporter message never appear.
- [ ] With a callback set, the endpoint receives one HTTPS POST per report with the fixed JSON body (Scope "Callback body") and the configured token; nothing else.
- [ ] Pushover works unchanged; existing `.env` files need no edit.
- [ ] Two services set, an invalid URL, `http://` to a public address or a token without a URL: exit 2 naming the setting, value never printed.
- [ ] Endpoint down, timing out (10 s), answering 4xx/5xx or redirecting: report stored and confirmed, one WARN line without secret, no delete key issued for that report.
- [ ] Rate limit: the 11th report within an hour is folded into the next message as today, on every service.
- [ ] A start with notifications off revokes every delete key; so does the first start after the service, URL, token or a Pushover setting changed, and the first start after the upgrade; a restart with unchanged settings keeps them; the data directory holds no setting in plain text; a failed update alerts through the chosen service.
- [ ] About page names the service; RUNBOOK, `example.env`, DOCKER, PODMAN and BAREMETAL docs cover it; tests with a local fake endpoint per service, none contacting a real service in CI.

## Open questions
- [x] **OQ-338 Which services.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Services".
- [x] **OQ-339 One or several at once.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Services".
- [x] **OQ-340 Settings and URLs.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Configuration".
- [x] **OQ-341 Failure handling.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Failure handling".
- [x] **OQ-342 Secret handling.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Secrets".
- [x] **OQ-343 Updater alerts.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Updater alerts".
- [x] **OQ-344 Revoking delete links on a change.** Decided (user, 24 Nov 2026) as suggested, then changed the same day (F70-Q1): revoked automatically on a change; now in Scope "Revocation".
- [x] **OQ-345 Callback body.** Decided (user, 24 Nov 2026) as suggested; now in Scope "Callback body".
