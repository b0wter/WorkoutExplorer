# F-12 Owner API: history

History of [architect/F-12-owner-api.md](../F-12-owner-api.md). Read it only when you need the history.

## Version 1.1 (30 Oct 2026, F-20 design v0.2): private state removed (SH-2)

- Settings table, `visibility`: type "`draft` | `unlisted` | `private`", in `PATCH` "`unlisted` or `private`, only once published".
- O-6 read: "A draft accepts no `visibility` at all: publishing is F-15's endpoint, and private exists only after publishing (assumption 19). Published: `unlisted` ↔ `private`, and the same value is a no-op. `draft` is always refused."
- O-11: the hook ran after a `PATCH` of a published workout "(`unlisted` or `private`)"; its list included "`private` → remove the folder".
- F-15 hand-off: "when a change hides more (`private`, `hide_ends` on)".
