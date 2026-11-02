# F-10 Storage limit and status endpoint

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** UP-8 (server); Technical design: Hosting and operations (Storage limit)
- **Depends on:** F-09

## Goal
Cap total storage with a server setting, refuse uploads when full, and tell the upload page before the sharer picks a file.

## Scope
- Config setting for the total size of `/srv/workouts`.
- Running total, recomputed at startup and updated on every write and delete.
- When the cap is reached, `POST /api/workouts` returns `503` with "Uploads are paused because storage is full. Please try again later."
- `GET /api/status` reports `uploads_open`.
- Publishing, edits, regeneration and deletes are never refused by the cap (assumption, see README).

## Out of scope
- The notice on the upload page (F-11).

## Acceptance criteria
- [ ] With the cap set just above current usage, the next upload that reaches it succeeds or is refused as documented, and every upload after that gets `503` with the exact message above.
- [ ] `GET /api/status` returns `uploads_open: false` while full and `true` again after a delete or expiry frees space, without a restart.
- [ ] After a restart the running total matches the actual size on disk.
- [ ] Publishing an already uploaded draft still works while uploads are paused.
