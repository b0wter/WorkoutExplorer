# ADR-019 SQLite store: pure-Go driver, one connection, schema v1, forward-only migrations

- **Status:** accepted; revision 2 (private state removed, item 8) accepted with the F-20 design v0.2; revision 3 (deleted rows scrubbed, items 3 and 9) accepted with the F-21 design v0.2 (QA-21-r1-1, user decision); revision 4 (`reports` with `ON DELETE CASCADE`, items 6, 9, 10) accepted with the F-27 design v0.1
- **Date:** 16 Oct 2026; revision 1 of the same day; revision 2 of 30 Oct 2026; revision 3 of 31 Oct 2026; revision 4 of 3 Nov 2026
- **History:** [architect/history/ADR-019-sqlite-store-schema-and-migrations.md](history/ADR-019-sqlite-store-schema-and-migrations.md)
- **Features:** F-09 (introduces it), F-10, F-12, F-15, F-21, F-27 (`reports` table), F-30, F-31
- **Supersedes:** none. Settles the former open question "SQLite driver, schema and migrations" (architect/history/overview.md)
- **Requirements:** TD Tech stack (SQLite, WAL mode), TD Data model (`workouts` table), NFR Privacy (store as little as possible), NFR Security (hashed tokens), F-09 AC 1 and AC 5; TD Conventions (Binary: `CGO_ENABLED=0`) and TD Code structure (Dependency rules: standard library first)

## Context

The TD fixes SQLite in WAL mode and the `workouts` columns, but not the driver, the column types, how the schema changes over time, or where the file lives. The binary is static and built without cgo (TD Conventions, Binary), which rules out `mattn/go-sqlite3`. There are no backups (TD Principles), so a migration that goes wrong cannot be restored; it has to be right or refuse to run.

Pure-Go drivers checked on 16 Oct 2026:

| Driver | How | Licence | Notes |
| --- | --- | --- | --- |
| `modernc.org/sqlite` v1.60.1 (29 Sep 2026) | SQLite C source translated to Go; embeds SQLite 3.53.4 | BSD-3-Clause (SQLite public domain) | `database/sql` driver `"sqlite"`; pragmas through `_pragma` in the DSN; linux/amd64 and linux/arm64 supported; widely used |
| `github.com/ncruces/go-sqlite3` | SQLite compiled to Wasm, run by `wazero` | MIT | Second runtime (Wasm) inside the binary; its own API besides `database/sql` |

## Decision

1. **Driver: `modernc.org/sqlite`, pinned at v1.60.1** (exact version in `go.mod`, `go.sum` committed). Only `internal/store` imports it in production code (TD Code structure, Dependency rules). Rev. 1: `_test.go` files in `cmd/workout-explorer` may also import it, only to inspect the database file in end-to-end tests. No other test package does. Upgrade rule: upgrade `modernc.org/sqlite` alone and let it choose its `modernc.org/libc` version; never bump `modernc.org/libc` on its own (the two are released as a matched pair).
2. **Database file:** `<data>/workouts.db` (with its `-wal` and `-shm` files). `store.Open` creates the file with mode `0600` before the driver opens it, so the WAL and shared-memory files get the same mode.
3. **Connection settings, constants only, set through the DSN on every connection:** `journal_mode(WAL)`, `synchronous(NORMAL)`, `foreign_keys(ON)`, `busy_timeout(5000)`, `secure_delete(ON)` (rev. 3, item 9), and `_txlock=immediate`. The path is escaped into the DSN; no part of a pragma comes from input. `Open` reads back `PRAGMA journal_mode` and fails unless it is `wal`.
4. **One connection:** `db.SetMaxOpenConns(1)`. Every statement is serialised in the process. At this volume (friends, at most a few uploads a minute) this is never the bottleneck, and it removes `SQLITE_BUSY` between our own connections. The `regenerate` process (F-30) is a second process; `busy_timeout` covers it.
5. **Forward-only migrations, numbered SQL files embedded in the binary.** `internal/store/migrations/NNNN_<slug>.sql`, numbered from `0001` without holes. The schema version is `PRAGMA user_version`. `Open` applies each file above the current version in its own `BEGIN IMMEDIATE` transaction, which also sets `user_version` to the file's number, then commits. A database whose `user_version` is above the highest embedded number makes `Open` fail ("database schema N is newer than this binary (M)") and `serve` exit 1. No migration runs and no table is created. The connection pragmas of item 3 run first, so the file's header may still change (for example, the WAL switch). That is harmless, because every schema version uses WAL. There are no down migrations. A file, once released, is never edited; a change is a new file.
6. **Schema v1 (`0001_workouts.sql`)** is the TD's `workouts` table, with these choices (exact SQL in the F-09 design, section Schema):
   - `STRICT` table; times as `INTEGER` Unix seconds (UTC); booleans as `INTEGER` 0/1 with a `CHECK`; the token hash as a 32-byte `BLOB`.
   - `id` is the internal ID (ADR-020), `TEXT PRIMARY KEY`; `public_id` is `UNIQUE`.
   - TD `time_zone` is stored as `time_zone_offset_s` (the workout's fixed UTC offset, F-02 decision 13; `NULL` when unknown). `start_time` is `NULL` when the file has no valid time.
   - **Added:** `data_end_s` (`NULL` unless the file was truncated): the UP-3 notice's time, which F-08 asks F-09 to persist for F-19. The data file format stays unchanged (ADR-005).
   - **Deferred:** `trim_start_s`, `trim_end_s` (UP-6, phase 2) are added by a later migration when trim is built. `privacy_zones`, `pins` (phase 2) likewise. (rev. 4) `reports` is schema v4 (F-27 design R-2).
   - Invariants as `CHECK` constraints: `visibility` in (`draft`, `unlisted`) (since item 8); `published_at IS NULL` exactly when `visibility = 'draft'` (SH-2); `created_at < expires_at <= created_at + 30 days` (UP-7); `json_valid(summary_json)`; fixed lengths of `id`, `public_id` and the hash.
   - No column for anything about the request: no IP address, user agent, file name or upload duration (NFR Privacy, F-09 AC 5). `TestSchema` pins the exact column list.
   - Title and description lengths are validated by F-12 in Go, not by a `CHECK`, because "character" must mean the same thing in the API and the browser.
7. **Index:** `workouts_expires_at` on `expires_at`, for the expiry job (F-21).
8. **(rev. 2) No private state** (SH-2, 30 Oct 2026). The next migration narrows `visibility` to (`draft`, `unlisted`). SQLite cannot change a `CHECK` in place, so it rebuilds `workouts` (new table with every column and `CHECK` of schema v2, copy, drop, rename, index again; the new-table-plus-copy pattern below) and **does not copy `private` rows**. Their `private/{id}/` folders go at the next start (ADR-022 recovery, a folder without a row); they have no public folder (F-15 P-7). A private row is never turned into `unlisted`, which would publish what its owner hid. Workouts are short-lived and carry no compatibility promise, so dropping them is accepted.
9. **(rev. 3) A deleted row leaves no bytes in the database files** (AC-4 "nothing remains", NFR Privacy; QA-21-r1-1). `secure_delete(ON)` (item 3) overwrites freed content in the main file. Because WAL frames still hold the old page, `PRAGMA wal_checkpoint(TRUNCATE)` runs after every expiry pass that deleted a row and after every owner delete (F-21 X-4, F-12 O-10, F-27 `delete`). A checkpoint that reports busy or fails is logged, and the next expiry pass checkpoints again even if it deleted nothing. (rev. 4) `serve`'s expiry pass checkpoints on **every** pass, so a delete by another process (F-27 `delete`, F-30) leaves the WAL within one pass (F-27 R-10). Checked by the F-21 X-9 byte scan of `workouts.db`, `-wal` and `-shm`. **Accepted limit** (user, 31 Oct 2026): copies below SQLite (file system journal, SSD remapping, backups) are not covered.
10. **(rev. 4) Child tables reference `workouts (id)` with `ON DELETE CASCADE`** (`reports`, F-27 R-2; later `pins`, `privacy_zones`), so the single delete path (I-4) removes them in the row's own statement. **Constraint on later migrations:** with `foreign_keys(ON)`, dropping `workouts` (the rebuild pattern of item 8) deletes every child row, and `PRAGMA foreign_keys` cannot be switched off inside the migration's transaction. A migration that rebuilds `workouts` must rebuild its child tables in the same file (copy aside, rebuild, copy back), or state that it drops them.

## Consequences

- One more third-party module tree (`modernc.org/sqlite` and its `modernc.org/*` dependencies). It is pure Go, so the build stays `CGO_ENABLED=0`. The binary grows by several MB. Measured in F-09 (`CGO_ENABLED=0`, linux/amd64): 11.5 MB before, 18.6 MB after (+7.05 MB). Accepted: it is one static file copied per deploy, with no runtime cost beyond the code pages it uses. The trigger below stays. The QA dependency check gains `modernc.org/...`.
- **One-way door: migrations are forward-only, and there are no backups.** A deploy that runs a new migration cannot be rolled back by deploying the old binary: the old binary refuses to start (item 5) instead of running on a schema it does not know. F-31's deploy script must state this; rolling back means deploying a fixed new binary. Every migration is reviewed as a one-way change.
- `summary_json`, `sport`, `start_time` and `time_zone_offset_s` duplicate values in `draft.bin`, as the TD intends ("for lists and preview images"). They are refreshed together with the files by F-30.
- SQLite `CHECK`s back the domain rules up; the rules themselves live in `lifecycle` (pure Go) and are tested there.

## Would choose differently if

- The binary size or build time of `modernc.org/sqlite` becomes a problem on the deploy path (F-31): try `ncruces/go-sqlite3` behind the same `store` package.
- Several app processes ever write at the same time (TD fallback: PostgreSQL), or `SQLITE_BUSY` shows up in logs between `serve` and `regenerate`: raise `busy_timeout` first, then reconsider the single connection.
- A migration needs data the old schema cannot express without loss, or a changed `CHECK`: write it as a new table plus copy in one migration file, never as an edit of an old file.
