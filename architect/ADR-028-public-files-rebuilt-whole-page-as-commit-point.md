# ADR-028 Public files: row first, a whole rebuild from the original on every change, `index.html` as the commit point, remove first when a change hides more

- **Status:** accepted (25 Oct 2026), with the F-15 design v1.0; revision 1 (30 Oct 2026, private state removed: items 3 and 4) accepted with the F-20 design v0.2
- **History:** [history/ADR-028-public-files-rebuilt-whole-page-as-commit-point.md](history/ADR-028-public-files-rebuilt-whole-page-as-commit-point.md)
- **Date:** 25 Oct 2026
- **Features:** F-15 (introduces it); F-16, F-21, F-23, F-24 (plug in), F-30 (reuses the build), F-22, F-31 (serving)
- **Note (9 Nov 2026, F-36, ADR-041):** item 2's build also reads the stored preview basemap, if any, and uses it only when its key matches the frame of the current public route. A basemap upload changes no row field and rebuilds in place
- **Supersedes:** none. Applies [ADR-022](ADR-022-private-workout-files-staging-and-recovery.md) items 5 and 6 to `public/`; implements the F-12 hand-off ("fail toward less public content, remove first, then write, idempotent")
- **Depends on:** [ADR-005](ADR-005-workout-data-file-format-v1.md) (deterministic encoding), [ADR-006](ADR-006-brotli-pre-compression.md), [ADR-025](ADR-025-owner-api-access-and-not-found-response.md), [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md)
- **Requirements:** SH-2, SH-10, AC-5, UP-4, NFR Privacy; TD invariants I-3, I-4

## Context

1. Caddy serves `public/` on its own. It keeps serving while Go is down, so whatever is on disk after a crash is public until something removes it.
2. A viewer can load the page during a change. The page must never point at a missing or half-written file (F-15 AC 8).
3. A change that hides more (hiding on, download off, later privacy zones) must never leave the older, more public files served because a step failed (SH-10, F-12 hand-off).
4. F-30 rebuilds every workout from its original on every deploy, and a kept workout may still serve old files and an old bundle (ADR-005 item 9). A partial rewrite of such a workout (a new page over an old data file) would mix versions.
5. Encoding is deterministic (F-03): the same original and settings give the same bytes, so the same content-hashed names.

## Decision

1. **Row first, files after.** Publish and every settings change commit the row first; the files follow under the F-12 O-7 lock. A failure after the row commits leaves less public on disk than the row allows (the generic page), never more. The owner retries (publish again, or any save: F-12 O-11), and the retry converges.
2. **Every change is a whole public build, in memory, from `original.fit`:** parse, the public transform (hiding; F-16 cuts start and end positions, [F-16 design](F-16-start-and-end-hiding.md)), then the files (data file and `.br`, later preview and export), then the page from the written data file's metadata. Only files whose content-hashed name is not on disk yet are written (and only those are compressed). `index.html` is always written. A title edit therefore writes one file, and a hiding change writes the new data file. The same build serves publish, edits and F-30.
3. **`index.html` is the commit point.** New referenced files are written first (temporary name in the same folder, `fsync`, `rename`), then `index.html` by `rename` over the old one, then the folder is `fsync`ed. After that, every other entry in the folder is removed. A stop after any step leaves the old complete page or the new one.
4. **Remove first when a change could hide more.** A pure rule compares the records before and after. The change is written in place only if every field that differs is on a closed allow-list of changes that cannot remove public content (F-15 design P-7). Any other difference, including a field added later and never classified, removes `index.html` first, then the rest of the folder, and then builds as for a first publish.
5. **Reconciliation.** At startup and on F-21's schedule (F-21 design X-5; under [ADR-031](ADR-031-cross-process-lock-on-the-public-area.md)'s public lock), every entry under `public/` that is not the folder of a live unlisted row is removed. In a kept folder, every entry that `index.html` does not reference is removed. "Referenced" is a byte match on the page for `/` plus the file name (the next character is not a file-name character or `/`), not an HTML parse; `X.br` is referenced when `X` is; anything not linked that way is deleted, and a kept folder without `index.html` is removed whole. After each build, `lifecycle` reads the row again and removes the folder if the row is gone or no longer unlisted (other processes: F-27, F-30).

## Consequences

- Viewers see either a complete page or the generic page. During a remove-first build, for up to about a second, they see the generic page.
- A failed first publish leaves an unlisted row with no page. Nothing rebuilds it at startup. The owner's next save or the next deploy does.
- **Cost:** every save parses the original and encodes it again (one parse slot, ADR-017). For normal files that takes tens to hundreds of milliseconds, and it holds the O-7 lock while it runs. In return, nothing reads public files back, and the code has one path instead of a page-only path, a data path and F-30's path. A kept workout moves to the current version at its owner's next save, which is the F-30 open point's suggestion.
- Files a stopped build leaves behind (unreferenced old files, or a folder without `index.html` after a stopped remove-first build) are removed by the item 5 sweep, so they are served at most until the next sweep.
- A stored original that the running binary rejects at publish is a server error. Only F-30 deletes on rejection.

## Reversibility

Cheap. Public files are rebuilt on every deploy and live at most 30 days. No one-way door.

## Would choose differently if

- A title save of synthetic `ride-12h` takes more than 1 s on the reference machine. Then add a page-only path that reads the current data file (when exactly one exists, at this binary's version) and falls back to the whole build.
- Storage moves to an object store without atomic rename (TD scaling path). Then a manifest object becomes the commit point. That needs a new ADR.
- A CDN goes in front of `public/`. Then every remove and every `index.html` rewrite also purges, so SH-10's 60 s still holds (TD scaling path).
