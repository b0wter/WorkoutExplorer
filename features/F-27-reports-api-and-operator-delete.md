# F-27 Reports API and operator delete command

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** INF-2 (server), INF-3, NFR Privacy (no data outlives its workout); Technical design: Data model (`reports`), Hosting and operations (Reports)
- **Depends on:** F-15

## Goal
Anyone can report a published workout; the operator reviews reports with `sqlite3` and deletes a workout without its edit link.

## Scope
- `reports` table as in the technical design.
- `POST /api/reports`: public ID, reason (spam, illegal content, someone else's personal data, other), optional message up to 1,000 characters.
- Accepted only for published (unlisted) workouts; others get the same not-found response.
- Rate limit per IP in memory (value assumed, see README). No IP address or contact details stored.
- Reports deleted with their workout: F-27 adds them to the F-12 delete routine.
- CLI subcommand `workout-explorer delete <public_id>` running the normal delete.
- Short operator note in the repository: how to list reports with `sqlite3`.

## Out of scope
- Report form UI (F-28). Push notifications (Later). Admin web page (not planned).

## Acceptance criteria
- [ ] A valid report is stored with reason, message and time only; the table has no IP or contact column.
- [ ] A message of 1,001 characters, or an unknown reason, is refused.
- [ ] A report for a draft or unknown workout returns not found.
- [ ] Exceeding the rate limit from one IP is refused; nothing about the IP is persisted.
- [ ] Reports are deleted with their workout by every delete (owner, operator, expiry); F-12's delete test (AC 5) checks that none is left.
- [ ] `workout-explorer delete <public_id>` removes the workout, its reports and files exactly like an owner delete; an unknown ID gives a clear error and non-zero exit status.
