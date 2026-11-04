# F-30 Regenerate command

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** SH-10 (old files deleted), UP-7 (workout ends early when the new version rejects its file), AC-4 (delete routine), NFR Reliability, NFR Operations; Technical design: Workout data file (Versioning)
- **Depends on:** F-21, F-23, F-31 (the export file waits for F-24, phase 3)

## Goal
`workout-explorer regenerate` rebuilds every live workout from its original, so only one data format and one bundle are ever live, except for workouts kept after a failed rebuild, which keep their previous files and bundle until a later run rebuilds them.

## Scope
- Rebuild every live workout, drafts included, from `original.fit` and its database settings: draft data file, public data file, `index.html`, preview image, export file, as applicable to its visibility and settings.
- New content-hashed names; old files deleted afterwards (SH-10).
- No long-term storage (OQ-53): a workout that regenerates differently, including shorter (e.g. a new FIT library version stops reading earlier), is kept as regenerated. A workout is deleted with everything stored for it, through the one delete routine (F-12), as on expiry (AC-4, UP-7), only when the new version rejects its file. On a server-side failure, an interrupted run or a time limit, the workout is kept unchanged for the next run (decided 21 Oct 2026). Each deletion is logged with the device model, never coordinates. Failures never stop the deploy: the command finishes the run and reports how many workouts were deleted and how many were kept. A kept workout serves its previous files until a later run rebuilds it; an old bundle is removed only once no kept workout refers to it (OQ-55, decided 22 Oct 2026).
- Uses the same parse time limit as the server (`WORKOUT_EXPLORER_PARSE_TIMEOUT`, read from the same settings; F-31).
- Idempotent; skips workouts that expire during the run.
- Deploy step (added to F-31's deploy): after the Go container restarts, the deploy runs `workout-explorer regenerate` with the same env file as `serve`, then removes each old bundle that no kept workout refers to, and reports how many workouts were deleted and how many were kept (OQ-55).

## Out of scope
- The rest of the deploy (F-31).

## Acceptance criteria
- [ ] After a format version bump, running the command makes every live workout serve the new version; no file of the old version remains on disk, except the files and bundle of workouts kept after a failed rebuild.
- [ ] An old bundle that a kept workout still refers to is not removed, and that workout's page keeps working; once a later run rebuilds it, the old bundle is removed.
- [ ] Running it twice in a row leaves every page working.
- [ ] A workout whose original is made unparseable (test) is deleted: row, reports, `private/{id}/` and `public/{public_id}/` are gone, and its viewing link shows the generic "expired or not available" page within 60 s (SH-10). The log line names the device model and contains no coordinates. The other workouts are regenerated and the run completes without stopping the deploy.
- [ ] A workout whose original the new version reads shorter (test: a parser change that stops earlier) is kept and serves the shorter workout.
- [ ] A workout whose rebuild fails for a server-side reason (test: a disk write fails), or hits the time limit, is kept, is not counted as deleted, and the next run rebuilds it. If the first write fails, it is kept unchanged (row and files); if a later write fails, it may stay partly on the new version until the next run finishes it (decision log, 4 Nov 2026).
- [ ] A run stopped part-way leaves the unprocessed workouts unchanged; the next run rebuilds them.
- [ ] When regeneration fails for a workout, the deploy still completes. If the new version rejects the file, the workout is deleted; otherwise it is kept and its old bundle stays. The deploy output reports how many workouts were deleted and how many were kept.
- [ ] Hiding, visibility, lifetime, download and title settings are preserved exactly.
- [ ] A run over 100 workouts finishes within seconds on the target server class.

## Open points for the design
None. Both earlier points (owner edits a kept workout; kept draft on the edit page) are settled by `architect/F-30-regenerate-command.md` RG-12.
