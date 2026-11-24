# F-12 Owner API: history

History of [architect/F-12-owner-api.md](../F-12-owner-api.md). Read it only when you need the history.

## 24 Nov 2026: ADR-061 (CR-02)

- O-7 read: "**Owner writes are serialised.** One service-wide mutex in `lifecycle` covers, for each write: check the token, change the row, run the F-15 hook. Writes are `PATCH`, delete, and later publish, F-21's expiry delete and phase 2 pins. Everything after the first write runs with `context.WithoutCancel` (F-09 hand-off, D-18). Reads take no lock". Why: "Without the lock, a delete can fall between a `PATCH`'s commit and its hook, and the hook would then write public files for a deleted workout. Two hooks could also finish in the opposite order to their rows. Checking the token under the lock means a second delete answers `404` (AC 5). ADR-006 already lets only one brotli run happen at a time, so one lock costs nothing at friends-only volume".
- F-21 hand-off read: "Expiry deletes through `Delete` under the O-7 lock".

## Version 1.1 (30 Oct 2026, F-20 design v0.2): private state removed (SH-2)

- Settings table, `visibility`: type "`draft` | `unlisted` | `private`", in `PATCH` "`unlisted` or `private`, only once published".
- O-6 read: "A draft accepts no `visibility` at all: publishing is F-15's endpoint, and private exists only after publishing (assumption 19). Published: `unlisted` ↔ `private`, and the same value is a no-op. `draft` is always refused."
- O-11: the hook ran after a `PATCH` of a published workout "(`unlisted` or `private`)"; its list included "`private` → remove the folder".
- F-15 hand-off: "when a change hides more (`private`, `hide_ends` on)".
