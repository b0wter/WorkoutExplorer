# ADR-042 Report delete keys: issued at send, stored as a hash on the report, redeemed by one consuming statement

- **Status:** accepted (9 Nov 2026), with the F-37 design v0.1
- **Date:** 9 Nov 2026
- **Features:** F-37 (introduces it), F-27 (reports, operator delete), F-29 (About text)
- **Supersedes:** none. Changes [ADR-039](ADR-039-report-notifications-through-pushover.md) item 2 (rev. 1); a second credential beside the edit token ([ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md))
- **Requirements:** INF-6, INF-4, AC-4, SH-10, NFR Security (keys 128-bit, stored hashed), NFR Privacy (no data outlives its workout)

## Context

INF-6 lets the operator delete a reported workout from the Pushover notification through a single-use link. That link is a credential that deletes data, it passes through Pushover and the phone's push service, and it must not outlive its report or workout. The notifier only sends some notifications (hourly cap, bounded queue, no retry: ADR-039 items 3, 4).

## Decision

1. **Format:** 128 random bits, 22 base62 characters, SHA-256 of the text stored, the edit token's rules (ADR-020 items 3, 4). Sent to the server only as `Authorization: Bearer`, carried in the link's fragment.
2. **Storage:** a nullable, partially unique column `reports.delete_key_hash`, not a table. A report has at most one key, and the key's life is the report's: the `ON DELETE CASCADE` from `workouts` and the 30-day prune remove it with no extra rule.
3. **Issued at send, not at filing:** only when the notifier has decided to send a report's notification. A report without a sent notification has no key.
4. **Validity:** hash present, report younger than 30 days, workout row present and not past `expires_at`.
5. **Redeemed by one consuming statement** (`UPDATE … SET delete_key_hash = NULL … RETURNING`) before the unchanged operator delete. Single use does not depend on the delete succeeding or on a process lock.
6. **Revocation:** `serve` without Pushover settings clears every key at start.

## Consequences

- No key exists that could not have reached the operator, and none outlives its report, without any cleanup job.
- A database failure between the consume and the delete spends the key; the operator falls back to `workout-explorer delete`.
- Turning notifications off revokes every link at the next start; turning them on again issues keys only for new notifications.
- Anyone with the operator's Pushover account or unlocked phone can delete a reported workout within its lifetime (OQ-119, accepted; About page).
- A forward-only schema change (ADR-019 item 5): once deployed, rolling back means a newer binary.

## Would choose differently if

- A report needs more than one key, or keys for other actions (dismiss, hide): then a `report_keys` table with a purpose column.
- Notifications go to more than the operator's own device: then a shorter lifetime and a second factor, not just a link.
