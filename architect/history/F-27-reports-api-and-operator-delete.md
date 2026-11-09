# F-27 design history

## 9 Nov 2026 (F-37): replaced text

R-11 decision cell, as accepted on 3 Nov 2026:

> **Notifications through Pushover, opt-in** (ADR-039): settings `WORKOUT_EXPLORER_PUSHOVER_TOKEN` and `_USER` (both unset: off, the default; one only or a malformed value: exit 2); message `New report #<id>: <reason>`, never the link, `public_id`, title, reporter's text or request data; sent after commit from a bounded queue by one goroutine, 10 s timeout, no retry, so a report never fails or waits on Pushover; at most 10 per rolling hour per process, the rest counted into the next message; egress for `app` only through the override `compose.notify.yaml`

R-11 reason cell: "… The link is the workout's access and stays off third parties".

F-29 hand-off, last sentence: "If the operator enables notifications, Pushover learns only that a report with a given reason arrived."

Reason: INF-4 changed on 9 Nov 2026 (F-37, ADR-039 rev. 1, ADR-042).
