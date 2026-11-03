# F-27 Reports API and operator delete command

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** INF-2 (server), INF-3, INF-4, NFR Privacy (no data outlives its workout; Pushover the only server-side third party, opt-in); Technical design: Data model (`reports`), Hosting and operations (Reports), Configuration (Pushover settings); design: `architect/F-27-reports-api-and-operator-delete.md`, ADR-039
- **Depends on:** F-15

## Goal
Anyone can report a published workout; the operator reviews reports with `sqlite3` and deletes a workout without its edit link.

## Scope
- `reports` table as in the technical design.
- `POST /api/reports`: public ID, reason (spam, illegal content, someone else's personal data, other), optional message up to 1,000 characters.
- Accepted only for published (unlisted) workouts; others get the same not-found response.
- Rate limit per IP in memory (value assumed, see README). No IP address or contact details stored.
- Reports deleted with their workout (every delete path, design R-2), and at the latest 30 days after filing (design R-3).
- At most 10 stored reports per workout; further ones are answered `204` and not stored (OQ-101).
- CLI subcommand `workout-explorer delete -data <dir> <public_id>` running the normal delete, for drafts and published workouts.
- Pushover notifications (INF-4, ADR-039): off unless `WORKOUT_EXPLORER_PUSHOVER_TOKEN` and `WORKOUT_EXPLORER_PUSHOVER_USER` are both set; message is the report number and reason only; sent after the report is stored, never delaying or failing it; at most 10 per rolling hour, the rest counted into the next message. Compose override for the app's outbound access.
- Short operator note in the repository: how to list reports with `sqlite3`.

## Out of scope
- Report form UI (F-28). Notifications to services other than Pushover (later idea). Auto-hiding reported workouts (decided 3 Nov 2026: not planned). Admin web page (not planned).

## Acceptance criteria
- [ ] A valid report is stored with reason, message and time only; the table has no IP or contact column.
- [ ] A message of 1,001 characters, or an unknown reason, is refused.
- [ ] A report for a draft or unknown workout returns not found.
- [ ] Exceeding the rate limit from one IP is refused; nothing about the IP is persisted.
- [ ] Reports are deleted with their workout by every delete (owner, operator, expiry); F-12's delete test (AC 5) checks that none is left.
- [ ] A report filed 30 days ago or earlier is gone after one expiry pass; a younger one stays. No report ever outlives its workout.
- [ ] An 11th report on one workout is answered `204` and not stored.
- [ ] `workout-explorer delete -data <dir> <public_id>` removes the workout, its reports and files exactly like an owner delete; an unknown ID gives a clear error and non-zero exit status; a draft can be deleted too.
- [ ] With neither Pushover setting set, no notification is sent and `app` has no route out; with only one set, or a value that is not 30 ASCII letters and digits, `serve` exits 2 naming the setting, never its value.
- [ ] With both set, each stored report sends one notification whose message is `New report #<id>: <reason>` only: no link, public ID, title, reporter's message or request data. Refused or unstored reports send none.
- [ ] A slow or failing Pushover neither delays nor changes the report's response; a failure logs one WARN line without the error text, body or any secret.
- [ ] At most 10 notifications per rolling hour; later reports are counted into the next message as `(+N more)`.
