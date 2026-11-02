# ADR-019 SQLite store, schema and migrations: history

History of [architect/ADR-019-sqlite-store-schema-and-migrations.md](../ADR-019-sqlite-store-schema-and-migrations.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 1)

- **Date:** 16 Oct 2026 (revision 1, 16 Oct 2026, F-09 round 1: the import rule covers production code (tests in `cmd/workout-explorer` may read the database file through the driver); the binary size is measured; on a refused newer schema the file can still be touched by the connection pragmas)

## Revision 2 (30 Oct 2026, F-20 design v0.2): private state removed

Item 6, invariants, read: "`visibility` in (`draft`, `unlisted`, `private`); `published_at IS NULL` exactly when `visibility = 'draft'` (SH-2: private means published, then hidden)". Item 8 added (migration narrowing `visibility`, private rows dropped).

## Revision 3 (31 Oct 2026, F-21 design v0.2, QA-21-r1-1): deleted rows scrubbed

Item 3's pragma list read: "`journal_mode(WAL)`, `synchronous(NORMAL)`, `foreign_keys(ON)`, `busy_timeout(5000)`, and `_txlock=immediate`". Item 9 added (`secure_delete`, checkpoint after deletes).
