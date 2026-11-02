# F-15 Publish pipeline and static page: architecture

- **Version:** 1.1 (30 Oct 2026: private state removed, P-7, P-8; built in F-20 B-1)
- **Status:** accepted (25 Oct 2026). Critical lane. OQ-69 and OQ-70 decided by the user (25 Oct 2026)
- **History:** [history/F-15-publish-pipeline-and-static-page.md](history/F-15-publish-pipeline-and-static-page.md)
- **Date:** 25 Oct 2026
- **Feature:** [`features/F-15-publish-pipeline-and-static-page.md`](../features/F-15-publish-pipeline-and-static-page.md)
- **Requirements:** SH-1, SH-2, SH-10, UP-4, AC-5, DS-4, NFR Privacy; TD Publish and view flow (Publish 5 and 6, View), Data model (Files on disk), Security and privacy; invariants I-1, I-3, I-4, I-5
- **Builds on:** [F-12](F-12-owner-api.md) O-6, O-7, O-11, O-12, O-13 and its hand-offs; [F-13](F-13-activity-page-layout.md) D-1 to D-3, D-7 and its hand-off; [F-14](F-14-units-and-theme.md) D-9; [F-10](F-10-storage-limit-and-status.md) S-6, S-8; [F-09](F-09-upload-api-and-storage.md) hand-off; ADR-005, ADR-006, ADR-019, ADR-020, ADR-022, ADR-025, ADR-026, ADR-027; new: [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md)
- **Owner:** backend. The generic page (P-12) is a file in `src/web`; the main session decides who writes it

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| P-1 | **Publish endpoint.** `POST /api/workouts/{public_id}/publish`, with the ADR-025 check and no request body read. As in `DeleteByOwner`, strangers are refused before they wait on the O-7 lock, and the check runs again under the lock. A pure rule in `lifecycle`, next to `ApplyPatch`: visibility becomes `unlisted` from any state, and `published_at` is set only while it is null. Then the F-12 hook runs with the records before and after. Answers: `200` with the F-12 settings object, the not-found response, or `500 server_error`. Publishing an unlisted workout is a no-op on the row and rebuilds its files. One INFO line `workout published` with `id`, for draft → unlisted only | UP-4 ("publishing makes it unlisted"). Idempotent, so a failed publish can be repeated. One check and one lock, as every owner write (ADR-025 item 4, O-7) |
| P-2 | **Row first, files after; rebuild everything; `index.html` is the commit point; remove first when a change could hide more.** ADR-028 items 1 to 4 | Caddy serves `public/` while Go is down. Viewers never see a broken page. SH-10 |
| P-3 | **Modules.** `internal/publish` implements `lifecycle.ChangeHook` and a new `lifecycle` port for the public area: remove one workout's folder (returning the freed bytes) and list the folder names. It is the only code that creates, writes or removes anything under `public/` (I-3). It defines its own small ports: the original's bytes (implemented by `privatedir`, which stays the only builder of private paths) and `Parse` (the process's single `*ingest.Parser`, so its slots, budgets and time limit apply). A new pure package `internal/page` holds the Go page view-model builder and metric formatting. It imports only `datafile` and does no I/O. `publish` may import `lifecycle` (`Record`, ports), `page`, `ingest`, `workout`, `datafile`, `precompress`. `lifecycle` imports none of them. `cmd` wires them | TD dependency rules: decision logic is pure and I/O sits at the edges. Adapters import the package that defines the port. `page` is reused by F-23 (`og:description`) |
| P-4 | **The public build** (one workout, in memory): read `original.fit` → `Parse` → **public transform** (identity in F-15; F-16 hiding; phase 2 trim and zones) → **artefacts** (F-15: the data file and its `.br`; F-23 preview; F-24 export, only if allowed) → decode the data file just encoded → view model → execute the template. Artefacts and the view model receive only the transform's output, never the parsed workout before it. The build is deterministic | One place where hiding happens before anything public exists (I-3). The view model comes from the metadata as written (F-13 hand-off, ADR-026 item 3). F-30 calls the same build |
| P-5 | **Names.** `data.{hash}.bin` and `data.{hash}.bin.br`. `{hash}` is the first 16 lowercase hex digits of SHA-256 over the `.bin` bytes, the rule the samples use. `DataURL` is `/w/{public_id}/data.{hash}.bin`. `ViewPath` comes from `lifecycle.ViewPath`. Brotli runs only for a name not yet on disk | Matches `test-support/samples.mjs` `fileHash`. A title edit costs no compression (ADR-006 item 6) |
| P-6 | **Writing in place.** Every content-hashed file that is not yet in the folder is written under a unique dot-prefixed temporary name (`os.CreateTemp`) in the same folder, `fsync`ed and renamed. Then `index.html` the same way. Then the folder is `fsync`ed. Then every other entry is removed. Afterwards, `lifecycle` reads the row again. If it is gone, expired or not unlisted, `lifecycle` removes the folder through the port. Modes: folders `0750`, files `0640`. F-31 sets the group so Caddy can read them | ADR-028 item 3. `fsync` before `rename`, so a power loss never leaves an empty file under a final name. The second read covers F-12's cross-process race (F-27, F-30) |
| P-7 | **Hides-more rule** (pure, in `publish`). A change is written in place only if every field that differs between the two records is on this allow-list: `title`, `description`, `expires_at`, `published_at`, `visibility` draft → unlisted, `hide_ends` on → off, `allow_download` off → on. Any other difference, including a field the rule does not know, counts as hiding more. The build then removes `index.html` first, then the rest of the folder, and then writes as for a first publish. No difference at all: rebuild in place | Fails toward less public content by default, so F-16, F-24 and phase 2 cannot forget to classify a new setting |
| P-8 | **Delete and recovery.** `Delete`: the row, then `public/{public_id}`, then `private/{id}`; a failing public step does not skip the private one, and the errors are joined. The freed bytes of both are released only by the call that removed the row (F-10 S-6). `Recover`, after the step that removes rows without a folder (F-09 hand-off): removes every entry under `public/` that is not the folder of a live unlisted row. That includes folders with no row, draft and expired rows, names that are not a `public_id`, and, inside a live folder, every entry `index.html` does not reference (temporary files included), or the whole folder when it has no `index.html` (ADR-028 item 5). The `recovery` line gains a count, `public_removed`. The same sweep runs under the O-7 lock when F-21 schedules it | I-4. ADR-022 item 6 order. Anything left under `public/` is served, so an unknown entry is removed, not left alone as ADR-022 does in `private/` |
| P-9 | **Storage.** Every byte written or removed under `public/`, temporary files included, goes through `Storage.Add` and `Storage.Release`. These never refuse | F-10 S-8, review decision 22 |
| P-10 | **Template location.** `serve -web <dir>`, required like `-data` (`make dev-server` passes `build/web`). `<dir>/index.html` is read once at start and parsed with `missingkey=error`. If it is missing or does not parse, `serve` exits 1 before it listens. Go never serves it | Settles the TD open question. F-13 hand-off: Go builds and tests stay independent of the web build. A default path could silently break publishing |
| P-11 | **Go view model.** A struct with exactly the samples' shape. Its inputs mirror `lib/page`'s settings: title, description, `expires_at`, `public_id` and the data file name, plus the decoded metadata. It must reproduce every committed sample in `src/server/testdata/page/` as equal JSON values. **Rounding as JavaScript, not Go:** fixed decimals as `toFixed` (an exact binary tie rounds away from zero), whole numbers as `Math.round` (ties toward +∞). Never `strconv`'s ties-to-even or `math.Round`'s behaviour for negatives. Constructed tie values are tested in both languages, because no sample may contain a tie. Metric only (ADR-027). Fixed English month and sport names. Start time in the workout's offset, or UTC with the label when unknown. Expiry as in F-13 D-7 | Samples are the contract (ADR-026 item 2). Ties are rare in real data, so only a deliberate test catches them |
| P-12 | **Generic page.** One static file, `src/web/generic.html`, a third Vite entry built to `build/web/generic.html`. It has no script, no Open Graph tags, no workout data and no link to a workout. Title "Workout Explorer". Metas for `color-scheme light dark`, `referrer no-referrer` and `robots noindex, nofollow, noarchive`. The sentence "This workout has expired or is not available." and a footer link to About. Its stylesheet carries the upload page's theme block as one shared CSS source for the two non-activity entries, never a chunk shared with the activity entry (ADR-024). A static check, like the existing hygiene test, asserts no `<script>`, the meta and the dark block | DS-4 without JavaScript (F-14 D-9). One file, so the response is byte-identical for every ID by construction (AC 6) |
| P-13 | **Minimal Caddy config:** `deploy/Caddyfile`, the file that F-22 and F-31 extend. Paths come from environment placeholders. `/w/{id}` with `{id}` exactly 22 base62 characters → `public/{id}/index.html` if that **file** exists, else the generic page with status `404`. `/w/{id}/{name}` → the file, `precompressed br`, dot files never served. `/assets/*` and `/` (upload page) come from `build/web`. `/api/*` is proxied to Go. Never served: `build/web/index.html`, `private/`, `tmp/`, any root above `public/` | A folder without `index.html` (a first publish in progress, a remove-first build) must already show the generic page. `404` means "does not exist" (SH-2), and crawlers make no card from it. Headers and caching are F-22's |
| P-14 | **Errors and logs.** No error that the hook, the port or `Delete` returns contains a `public_id` or a path under `public/`, because `ownerFail` logs `err.Error()`. Path errors are reduced to the operation, the file's role (page, data file) and the system error. A stored original that `Parse` rejects at publish is a `500`, never a delete (F-30 owns deletes on rejection) | O-13, QA-12-r1-3, TD Logging |
| P-15 | **Deadlines.** The write deadlines of the publish and `PATCH` handlers (O-12) cover the parse time limit T, one compression and the writes. The engineer picks the values | A whole build runs inside the request (ADR-028 item 2) |

## Build notes

- Port: `PublicFiles.Remove(public_id)` and `Sweep(keep)`; the keep set comes from `Repository.LivePublicIDs`. `Delete(ctx, id, public_id)`: F-21 and F-27 pass the row's `public_id`.
- Bytes of a folder removed by the post-build re-read (P-6) are not released until restart (overcounted, so uploads close early).
- Write deadline of publish and `PATCH`: `2 × (30 s + T)`.
- A measuring error on a removed file counts it as 0 bytes and logs a warning without the path.
- Caddy answers the `.br` with `206` when no `Range` was sent (QA-15-r1-3, for F-22).

## Verification

Synthetic files only (ADR-004). Real files stay on the operator's machine; QA reports from them carry no figures that identify the workout.

| AC | Check |
| --- | --- |
| 1, 5, 6 | Go, end to end through `newApp`: no `public/{public_id}/` before publish; after publish, `index.html` and both data files. Generic page: P-13's rule is "no `index.html`", so the Go tests assert on the files. Through Caddy: manual (OQ-70) |
| 2 | The written `index.html` contains every summary value and lap cell of the view model, and the robots meta |
| 3 | A title `PATCH` changes `index.html`, and the data file names stay the same |
| 4 | A build with a changed public transform (a test seam) writes a new name, and the old `.bin` and `.br` are gone |
| 7 | P-12's static check. Manual: light and dark mode in a browser with JavaScript off |
| 8 | Fault injection at every write, rename and remove step, in place and remove-first. After each stop, the folder holds the old complete page, the new complete page, or no `index.html` |
| 9 | F-12's AC 5 list of places that hold workout data gains `public/{public_id}/` |
| 10 | Walk `public/` after a publish and after a build, comparing bytes with both private files: as decided in OQ-69 |
| Samples | `internal/page` reproduces all nine samples. The same transform `ride-no-hr` uses is applied in the Go test. Constructed ties (P-11) |
| Privacy | An injected file-system failure in a hook, the port and `Delete`: the logged line contains neither the `public_id` nor a public path |
| F-10 AC 4 | Publishing while uploads are paused succeeds and is counted |
| Measure | Publish and title save of `ride-12h` on the reference machine, recorded in `testdata/corpus/MEASUREMENTS.md` (ADR-028 trigger: 1 s) |

## Hand-offs

| To | What |
| --- | --- |
| F-16 | Hiding is the public transform (P-4). Turning `hide_ends` on is already remove-first (P-7). Route presence comes from the published file, so a fully hidden route has no position columns (ADR-005 item 6). OQ-15 is decided there |
| F-21 | Run P-8's public sweep every minute with the expiry deletes, under the O-7 lock: the SH-10 bound for every failure path. A lifetime change already rewrites the page (F-21 AC 4) |
| F-23 | The preview is an artefact (P-4). Add the Open Graph group to the samples and to `internal/page`, plus the base URL setting (ADR-020 item 8). The preview file must be linked from `index.html` by a plain path ending in its file name (ADR-028 item 5), or the sweep deletes it |
| F-24 | The export is an artefact, written only when allowed. Turning download off is already remove-first (P-7). The export file must be linked from `index.html` by a plain path ending in its file name (ADR-028 item 5), not only built in JavaScript, or the sweep deletes it |
| F-22, F-31 | Extend `deploy/Caddyfile`. Set the group on `public/` (P-6). Put `-web` in the unit. Measure `MemoryMax` with one publish (a parse slot plus one compression) running alongside N uploads |
| F-30 | Call the public build per workout. An owner save of a kept published workout rebuilds it at the current version (ADR-028). `draft.bin` of kept workouts stays F-30's |
| F-19, F-20 | Publish returns the settings object. After a `500`, visibility may already be unlisted with no page: show the error and let the owner publish again |
| Requirements (TD) | Applied to the [technical design](<../spec/Workout Explorer — Technical Design.md>) (Code structure, Conventions, Files on disk, Open technical questions, Glossary). F-12 AC 6 and F-15 AC 10 depend on OQ-69 |

## Open questions

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-69 | AC 10 asks that no public file has the bytes of `draft.bin`. With hiding off, the public data file **is** `draft.bin`, byte for byte. Reword AC 10? | **Decided (user, 25 Oct 2026):** no file under `public/` has the bytes of `original.fit` (F-15 AC 10); with hiding on, no public file contains hidden data (F-16 AC 1) | — |
| OQ-70 | AC 1, 5 and 6 through a real Caddy need the `caddy` binary. Is it available? | **Decided (user, 25 Oct 2026):** QA runs a locally downloaded release binary (never committed) once with `deploy/Caddyfile`; automated Caddy checks stay in F-22 | — |
