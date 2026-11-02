# ADR-022 Private workout files: data directory layout, staged creation, row last, recovery at startup

- **Status:** accepted
- **Date:** 16 Oct 2026; revision 2 of the same day
- **History:** [architect/history/ADR-022-private-workout-files-staging-and-recovery.md](history/ADR-022-private-workout-files-staging-and-recovery.md)
- **Features:** F-09 (introduces it), F-10 (usage total), F-12 (reads and deletes), F-15 (public folder beside it), F-21 (expiry delete), F-27 (operator delete), F-30 (regenerate rewrites `draft.bin`), F-31 (permissions, paths)
- **Supersedes:** none
- **Requirements:** TD Data model (Files on disk: `private/{id}/original.fit`, `private/{id}/draft.bin`), TD Security and privacy (originals never served publicly), NFR Security (originals never reachable by a public URL), NFR Privacy (no data outlives its workout), UP-7, AC-4, TD invariants I-3 and I-4, F-09 scope ("write files to a temp path and rename", "failure at any step leaves no row and no files behind")

## Context

An upload creates two files and one row. A file system and a database cannot commit together, and the process can die between any two steps. Whatever the order, a crash can leave one without the other. One of the two leftovers has to be the designated garbage that a sweep removes, and it must be the one nobody can reach.

## Decision

1. **Data directory** (`serve -data <dir>`, `/srv/workouts` in production):

   ```
   <data>/
     workouts.db (+ -wal, -shm)     ADR-019; mode 0600
     private/                       mode 0700; written only by internal/privatedir
       {id}/                        mode 0700; id = internal ID (ADR-020)
         original.fit               mode 0600; the upload with identifying data removed (UP-9, ADR-023): Accepted.Original
         draft.bin                  mode 0600; datafile.Encode of the parsed workout
     tmp/                           mode 0700; staging, emptied at startup
       {id}/                        one upload being written
     public/                        created and written only by internal/publish (F-15, I-3)
   ```

   The upload's own bytes are never written anywhere, not even to `tmp/`: only the stripped bytes are staged.

   `private/` and `tmp/` are `0700`, so even a Caddy misconfiguration cannot read originals (Caddy runs as another user); F-31 keeps `public/` readable by Caddy and nothing else.

2. **Draft data file: `draft.bin` only, no `.br`.** It is fetched by its owner through Go (F-12), not by every viewer through Caddy, so pre-compressing it would cost a brotli run (ADR-006's mutex) on every upload for one or two downloads. F-12 may compress the response on the fly.
3. **Creation is staged:** `tmp/{id}/` is created, each file is written and `fsync`ed, the folder is renamed to `private/{id}` in one `rename(2)` (same file system), and `private/` is `fsync`ed. Readers never see a half-written folder. If any step fails, `tmp/{id}` is removed before the error is returned.
4. **The row is inserted last**, after the folder is in place, in one transaction. If the insert fails, `private/{id}` is removed. So the only state a crash can leave is **a folder without a row**: unreachable, because every lookup starts from the row (by `public_id`), and never served.

   (rev. 2) **Never remove a folder whose row may exist.** A database driver can report an error for a statement that has committed: `modernc.org/sqlite` returns `ctx.Err()` when the context ends after an autocommit `INSERT` has finished (QA-09-r1-1). If the folder is removed then, the result is a row without files, which item 5 did not sweep. So:
   - Once the folder is in place, the client can no longer cancel the operation. The insert, and the check below, run with `context.WithoutCancel(ctx)`. The request context is checked once, before the files are written (F-09 design, Upload pipeline step 4). After that, a client that leaves gets a stored draft that expires (OQ-45). No timeout is added to the insert: any context that can end brings the same race back. Lock waits are bounded by `busy_timeout`.
   - If the insert still returns an error, `lifecycle` asks the repository whether the row exists (`WorkoutExists`, same non-cancellable context). The row exists: the upload succeeded and the error is ignored. The row is absent: remove the folder, as before. The check fails: the folder is **left in place** for item 5 to settle at the next start.
5. **Recovery at startup**, before the listener opens (`lifecycle.Service.Recover`):
   1. Remove everything in `tmp/`.
   2. Remove every `private/{name}` whose name is a valid internal ID and has no row.
   3. (rev. 2) Delete every row whose `private/{id}` folder does not exist. No code path of ours leaves this state (folder first, row last; row deleted first, folders after). It means the files were lost outside the process: a folder removed by hand, a disk fault, or a bug like QA-09-r1-1. Such a workout cannot be served, downloaded or regenerated. Deleting the row turns a broken link into a "not found" one, and it lets F-15's sweep remove the public folder that is left behind.
   4. Entries in `private/` that are not a valid internal ID are left alone and counted (they were not made by us).

   One INFO line `recovery` with counts only: `staging_removed`, `orphans_removed`, `rows_removed`, `unknown_entries` (rev. 2: there is no separate WARN line for unknown entries).
6. **Deletion order for later features (F-12, F-21, F-27), so the same sweep covers them:** delete the row first (with its reports, in one transaction), then the folders. A crash in between leaves a folder without a row, which step 5 removes at the next start. F-15 extends step 5 to `public/{public_id}` folders without a row.
7. **Only `internal/privatedir` creates, renames or removes anything under `private/` and `tmp/`.** It validates every ID against the internal-ID pattern before building a path.

## Consequences

- AC "failure at any step leaves no row and no files" holds for every error path in the process, and for a crash after the next start's recovery. Between a crash and the next start an orphan folder can exist on disk; nothing can reach it.
- (rev. 2) Every reader that goes from a row to its files (F-12, F-15, F-30) can rely on the folder existing after startup. In a running process the only exception is files lost outside the process; readers treat a missing file as a server error, and the next start repairs the state.
- (rev. 2) Item 5.3 deletes data, and that cannot be undone. It deletes only rows whose files are already gone, so nothing that could still be served is lost. If `IDs` fails, `Recover` fails and `serve` does not start (as before), so a listing error cannot delete rows.
- (rev. 2) The repository port gains `WorkoutExists(ctx, id)` and `DeleteWorkout(ctx, id)`. F-12, F-21 and F-27 reuse `DeleteWorkout` for "row first" (F-27 adds its reports to the same transaction).
- `fsync` costs a few milliseconds per upload; the upload rate is tiny.
- `-data` must be one file system (rename). F-31 mounts nothing below it.
- F-10's usage total is computed over `<data>` at startup after recovery, so orphans never count.
- (F-10 S-6) `PrivateFiles.Remove` now returns the freed bytes (the sizes of the files it held), and `Repo.DeleteWorkout` returns whether this call removed the row; `Delete` releases the bytes only then (QA-10-r1-1).

## Would choose differently if

- A storage backend without atomic rename is introduced (TD scaling path: object storage). Then the row gets a `state` column (`staging`, `ready`) and the order flips; that is a new ADR.
- Startup recovery becomes slow because of the number of folders (far beyond friends-only volume): run it in the background before the expiry job's first run, with uploads unaffected.
