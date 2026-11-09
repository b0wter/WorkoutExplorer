# ADR-039 history

## Rev. 1 (9 Nov 2026, F-37): replaced text

Item 2, as accepted on 3 Nov 2026:

> 2. **Content, fixed:** title `Workout Explorer`, message `New report #<reports.id>: <reason>`, plus `(+N more)` when reports were held back (item 4). **Never** the `public_id`, a link, the title, the reporter's message, or anything about the request. No `url`, `priority` or `html` parameters.

First Consequences bullet, as accepted on 3 Nov 2026:

> - The link is deliberately absent: an unlisted workout's link is its access, and sending it to Pushover and the push services would share the workout with them. The operator finds the report by its number with the F-27 R-7 query, and deletes over SSH anyway (R-8), so a link would save little.

Reason for the change: INF-4 changed on 9 Nov 2026 (user decision, F-37): the notification opens the workout's viewing link and carries a single-use delete link (INF-6); the reason is shown in words.
