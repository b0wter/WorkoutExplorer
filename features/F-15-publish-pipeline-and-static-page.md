# F-15 Publish pipeline and static page

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** SH-1, SH-2, DS-4 (generic page theme), UP-4 (publish step), AC-5 (edits reach the live page), SH-10 (old files deleted on regeneration), NFR Privacy (only unlisted workouts under `public/`); Technical design: Publish and view flow (Publish 5–6, View), Data model (Files on disk), Security and privacy
- **Depends on:** F-12, F-13, F-14

## Goal
Publishing writes the static files Caddy serves to viewers; later edits regenerate only what they affect; drafts never appear under `public/`.

## Scope
- `POST /api/workouts/{public_id}/publish` (token header, the same check and not-found response as the owner API, ADR-025): visibility becomes unlisted, `published_at` set.
- Writes `public/{public_id}/data.{hash}.bin` and `.br`, and `index.html` from the F-13 template with summary and laps in the markup, `<meta name="robots" content="noindex, nofollow, noarchive">`, preload links for bundle and data file.
- A hook where hiding (F-16), preview image (F-23) and export file (F-24) plug into the same pipeline.
- Later edits regenerate only affected files: title or description → `index.html` only; settings that change the streams → data file and everything derived from it. New content-hashed names; `index.html` rewritten atomically to point at them; old files deleted afterwards.
- No unpublish (SH-2, 30 Oct 2026): a published workout keeps its public folder until delete or expiry. The built unlisted ↔ private path is removed (removal of built behaviour; architect hand-off).
- Generic page served for every `/w/{public_id}` that has no public folder (draft, deleted, expired, unknown): no title, no numbers, no route, no Open Graph data. Text: "This workout has expired or is not available." It gets the same CSS-only light and dark theme as the upload page (DS-4, F-14).
- Writes are atomic: a viewer never sees a page pointing at a missing or half-written data file.
- The delete routine (F-12) also removes `public/{public_id}/`.
- A minimal Caddy config serving `public/` and the generic-page fallback, enough to test locally; headers and caching are finalised in F-22.

## Out of scope
- Caddy caching headers and the 60 s check (F-22), hiding (F-16), preview and OG tags (F-23), export file (F-24), expiry (F-21).

## Acceptance criteria
- [ ] Publishing a draft makes `/w/{public_id}` show the full activity page; before publishing it shows the generic page.
- [ ] The published page's summary and laps are in the HTML source; the page carries the `noindex, nofollow, noarchive` meta tag.
- [ ] Changing only the title rewrites `index.html` and leaves the data file name unchanged.
- [ ] A change that rewrites the data file produces a new hashed name, and the old data file no longer exists on disk.
- [ ] ~~Switching to private removes `public/{public_id}/` entirely; switching back restores the page.~~ Removed 30 Oct 2026 (no private state, SH-2).
- [ ] A draft shows exactly the same generic page as an unknown ID (byte-identical response).
- [ ] The generic page follows the system light or dark setting without JavaScript (DS-4).
- [ ] Killing the process mid-publish (test) leaves either the old or the new complete page, never a broken one.
- [ ] After delete, `public/{public_id}/` is gone (added to F-12's delete test, AC 5).
- [ ] No file under `public/` has the bytes of `original.fit`, checked after publish and after a regeneration. (With hiding off, the public data file may equal `draft.bin`; no hidden data in public files is F-16's AC 1.)

AC 1, 5 and 6 through Caddy: QA runs a locally downloaded Caddy release binary (never committed) once with `deploy/Caddyfile`. Automated Caddy checks are F-22's.
