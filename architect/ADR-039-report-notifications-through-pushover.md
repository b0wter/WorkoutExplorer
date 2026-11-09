# ADR-039 Report notifications through Pushover, opt-in, with egress only when configured

- **Status:** accepted (3 Nov 2026), with the F-27 design v0.1; rev. 1 (9 Nov 2026, F-37: item 2 and the first Consequence rewritten for INF-4 as changed, accepted with the F-37 design)
- **Date:** 3 Nov 2026; rev. 1 9 Nov 2026
- **History:** [history/ADR-039-report-notifications-through-pushover.md](history/ADR-039-report-notifications-through-pushover.md)
- **Features:** F-27 (introduces it), F-31 (Compose override, env file), F-29 (privacy policy), F-37 (links, [ADR-042](ADR-042-report-delete-keys.md))
- **Supersedes:** none. Narrows [ADR-034](ADR-034-compose-bridge-networks-with-mounted-binaries.md) item 1 ("`app` has no route out") to "unless notifications are configured" (ADR-034 rev. 4). An exception to the spec's "no external services", operator-only and opt-in (user decision 3 Nov 2026; decision log entry by the main session)
- **Requirements:** INF-2, NFR Privacy, TD invariant I-7; user decision of 3 Nov 2026 (notifications in scope)

## Context

The operator wants to learn of a report without checking the table. Pushover's API is one HTTPS `POST` to `https://api.pushover.net/1/messages.json` with an application token and a user key. Today `app` sits only on the internal `backend` network and cannot reach the internet; that isolation limits what a compromised `serve` could send out (originals hold coordinates). Pushover is a third party outside the EU, and its messages travel through Apple's or Google's push services.

## Decision

1. **Settings, read by `serve` only:** `WORKOUT_EXPLORER_PUSHOVER_TOKEN` and `WORKOUT_EXPLORER_PUSHOVER_USER`. **Both unset or empty: no notifications (default).** One set without the other, or a value that is not 30 ASCII letters and digits (Pushover's format): `serve` exits 2 with a message naming the variable, never its value (TD Configuration rule). `server started` logs `notifications` as `true` or `false` only. No other subcommand reads them.
2. **(rev. 1) Content, fixed:** title `Workout Explorer`; `html=1`; message `New report #<reports.id>: <Reason label>`, plus `(+N more)` when reports were held back (item 4), then the delete link of INF-6 as an `<a href>` anchor when a key was issued for this notification ([F-37](F-37-report-notification-links.md) DK-3, DK-10); `url` the workout's absolute viewing link, `url_title` `Open workout`. **Never** the title, the reporter's message, or anything about the request. No `priority`.
3. **Delivery, after commit and never in the request path:** the report is stored and answered first; the notification goes into a small bounded in-memory queue read by one sender goroutine. A full queue drops it and counts it into the next message's `(+N more)`. One request per notification, **10 s timeout, no retry**. The HTTP client has no proxy from the environment, follows no redirect and verifies TLS with the image's CA bundle. The secrets go only in the form body, never in the URL. A failure is one WARN line `notification failed` with the HTTP status or a fixed class (`timeout`, `network`), never the error text, the body or a secret. Pending notifications are not drained on shutdown.
4. **Global cap: at most 10 sent per rolling hour per process.** Reports beyond it are not sent on their own; their count rides on the next sent message. Only stored reports notify (F-27 R-4's unstored ones do not). Only a sent notification gets a delete key (rev. 1, ADR-042 item 3).
5. **Egress only when configured:** a Compose override `deploy/compose.notify.yaml`, selected with `COMPOSE_FILE` like `compose.build.yaml`, attaches `app` alone to a third bridge network `egress` (IPv4, fixed subnet `10.250.2.0/29`, not internal, no published port). Without the override `app` keeps no route out. Inbound is unchanged: `app` publishes no port, Go still trusts only Caddy's `backend` address. `serve` also answers on its `egress` address, which only the host reaches, as it reaches `backend` (Docker Engine 28 and later, from Docker's repository as the runbook requires, drops routed traffic to unpublished container ports).
6. **No callback abstraction.** `reports` calls one notifier port (the codebase's port pattern); its only adapter is Pushover's. Other services stay a later idea.

## Consequences

- (rev. 1) Pushover and the phone's push service learn the reported workout's viewing link, which is its access, and carry a single-use delete link (user decision 9 Nov 2026, INF-4, INF-6; About page). Both are limited to reported workouts and their lifetime; the delete key is not printed on the lock screen (HTML stripped there, F-37 DK-10). Turning notifications off revokes every delete link at the next start (ADR-042 item 6).
- Item 4 bounds the operator's phone and the Pushover quota: at most 240 a day, about 7,440 a month, under Pushover's free 10,000 per application. The per-client limit (60 per hour) alone does not: many addresses times 60 is unbounded.
- With the override, a compromised `serve` can reach any host on the internet, not only Pushover (Docker cannot filter by name). This is the price of opting in; an egress proxy with an allow-list was rejected as a third service for one request type.
- Setting the secrets without the override gives only `network` failures in the log; the runbook names both steps together. The env file now holds a secret (F-31 DP-6, DP-14: `0600`, never committed).
- A notification can be lost (Pushover down, queue full, restart). The table is the record; notifications are a hint.

## Would choose differently if

- Notifications to other services are wanted: then a generic callback, and a new ADR for its content and egress.
- Pushover changes its token format or endpoint: item 1's check and the URL are constants.
- Reports per hour ever exceed the cap in practice: raise it within the quota, or send a daily summary.
