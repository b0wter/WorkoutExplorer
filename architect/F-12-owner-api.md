# F-12 Owner API: architecture

- **Version:** 1.1 (30 Oct 2026: private state removed, O-6, O-11; built in F-20 B-1)
- **Status:** accepted (23 Oct 2026)
- **History:** [history/F-12-owner-api.md](history/F-12-owner-api.md)
- **Date:** 23 Oct 2026
- **Feature:** [`features/F-12-owner-api.md`](../features/F-12-owner-api.md)
- **Requirements:** AC-4, AC-5, UP-4 (server), UP-7 (lifetime change), UP-9, SH-2, SH-8, SH-10 (through the F-15 hook), NFR Security (originals never public); TD Publish and view flow (Publish 4, 6), TD Security and privacy (IDs and tokens, Draft workouts, Originals)
- **Builds on:** [F-09 design](F-09-upload-api-and-storage.md) (Schema, hand-off row F-12), [F-10 design](F-10-storage-limit-and-status.md) S-6, S-8, [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md), [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) rev. 1, [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) items 5, 9, 10, [ADR-022](ADR-022-private-workout-files-staging-and-recovery.md) rev. 2; new: [ADR-025](ADR-025-owner-api-access-and-not-found-response.md)
- **Owner:** backend

No new module, setting or migration: schema v1 already has every settings column and its `CHECK`s.

## Endpoints (published language for F-19, F-20)

Every endpoint takes `Authorization: Bearer <token>` and no query string. On any failure of the token check, it answers with ADR-025's single not-found response.

| Method and path | Success | Other answers |
| --- | --- | --- |
| `GET /api/workouts/{public_id}` | `200`, settings object | `404`, `500` |
| `PATCH /api/workouts/{public_id}` | `200`, settings object as stored after the change | `400 invalid_settings`, `404`, `500` |
| `GET /api/workouts/{public_id}/draft.bin` | `200`, the stored `draft.bin` | `404`, `500` |
| `GET /api/workouts/{public_id}/original.fit` | `200`, the stored `original.fit` as an attachment | `404`, `500` |
| `DELETE /api/workouts/{public_id}` | `204`, no body | `404`, `500` |

`500` uses the existing `server_error` row (UP-10's sentence). `invalid_settings` carries a short plain sentence for each rule. The engineer writes the sentences, and they are reviewed together with F-19, whose controls should never let these errors happen. Other methods get the mux's `405`, the same for every ID.

**Settings object.** It holds only what `draft.bin` does not. Sport, start time, time zone and summary come from the data file (F-03 §5).

| Field | Type | In `PATCH` | Rule |
| --- | --- | --- | --- |
| `title` | string | yes | Leading and trailing white space is trimmed. At most 100 code points after trimming. No control characters (Unicode Cc) |
| `description` | string | yes | At most 1,000 code points. No control characters except line feed |
| `hide_ends` | bool | yes | Stored. Takes effect at publish or regeneration (F-16) |
| `allow_download` | bool | yes | The viewer export only (F-24). It never affects the owner's original |
| `visibility` | `draft` \| `unlisted` | no | O-6 |
| `lifetime_days` | integer | yes, 1 to 30 | O-5. In `GET`: (`expires_at` − `created_at`) / 86,400, always whole |
| `created_at`, `expires_at` | RFC 3339 UTC, whole seconds | no | The edit page shows the resulting date for a chosen lifetime (F-19 AC 3) |
| `data_end_s` | integer or `null` | no | The UP-3 notice's time (F-09 hand-off) |
| `view_url` | string | no | `lifecycle.ViewPath`. Only `lifecycle` builds links (ADR-020 item 10) |
| `hide_ends_ranges` | array of `[first, last]` | no | What hiding would remove, computed at upload ([ADR-030](ADR-030-to-be-hidden-ranges-computed-at-upload.md)); `[]` without positions |

A `PATCH` body with an unknown field, a read-only field, a `null`, a wrong type, or anything that is not one JSON object gets `400`. An empty object `{}` is valid and changes nothing.

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| O-1 | **Handle and paths.** Paths as in the table above, with `public_id` as the handle. F-15's publish is `POST /api/workouts/{public_id}/publish`. | The internal ID never reaches a browser (ADR-020 item 1). File names in the paths match the glossary and the files on disk |
| O-2 | **Access** is ADR-025: one lookup-and-check in `lifecycle` for every owner operation, one `404` body, expired rows count as not found, any query string is refused, no rate limit, nothing logged for refusals. `httpapi` only extracts the header and maps the result | One code path makes "same status, body and timing" true by construction. F-15 reuses it |
| O-3 | **Partial update (`PATCH`).** Only the fields sent are validated and changed, all or nothing. The rule (validation plus the new record) is a pure function in `lifecycle`. The store applies it as a read-modify-write in one `BEGIN IMMEDIATE` transaction. If the row is gone by the write, the answer is not found | Two tabs that save different fields do not overwrite each other. The immediate transaction also covers writers in other processes (F-27's CLI, F-30). Pure rules are tested without a database (TD Dependency rules) |
| O-4 | **Plain text.** Title and description are stored and returned exactly as accepted: `<script>` stays `<script>` (AC 3). They are never HTML-escaped on input. Every renderer escapes on output (F-13 template, F-15, F-19). "Character" means a Unicode code point (Go runes, JS `[...s].length`). An HTML `maxlength` counts UTF-16 units, which is never looser, so F-19 may use either | ADR-019 item 6: the same meaning of "character" in the API and the browser. Escaping at output is the only place that knows the context |
| O-5 | **Lifetime** is set as whole days from upload (accepted review assumption 18): `expires_at = created_at + lifetime_days × 86,400 s`. It is refused unless 1 ≤ days ≤ 30 and the result is after now. Shortening is allowed. Upload 30 days → accepted, 31 → refused, a result in the past → refused (AC 2) | Whole days make the 30-day bound exact against the schema `CHECK`. A day count cannot drift through time zones or clocks in the browser. `now` comes from `Deps.Now`, so tests are exact |
| O-6 | **Visibility is read-only.** Only F-15's publish changes it, draft → unlisted, and nothing changes it back (SH-2, 30 Oct 2026). A `PATCH` with `visibility` is refused like any read-only field (`400`). `published_at` keeps the first publish time | SH-2, UP-4 ("publishing makes it unlisted", cannot be undone). The schema `CHECK` (draft ⇔ no `published_at`) stays true |
| O-7 | **Owner writes are serialised.** One service-wide mutex in `lifecycle` covers, for each write: check the token, change the row, run the F-15 hook. Writes are `PATCH`, delete, and later publish, F-21's expiry delete and phase 2 pins. Everything after the first write runs with `context.WithoutCancel` (F-09 hand-off, D-18). Reads take no lock | Without the lock, a delete can fall between a `PATCH`'s commit and its hook, and the hook would then write public files for a deleted workout. Two hooks could also finish in the opposite order to their rows. Checking the token under the lock means a second delete answers `404` (AC 5). ADR-006 already lets only one brotli run happen at a time, so one lock costs nothing at friends-only volume |
| O-8 | **File responses** (`draft.bin`, `original.fit`) are streamed from the open file, never read whole into memory. `PrivateFiles` gains a read operation, so `privatedir` stays the only code that builds private paths (ADR-022 item 7). Headers: `application/octet-stream`, `Content-Length`, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`. No compression in Go. A file missing after the token check makes the server look at the row again: no row is `404` (a delete won the race), a row is `500` (files lost outside the process, ADR-022 Consequences) | A 20 MB download per request would enter the server memory model M(N) (ADR-017). The bytes go out as stored (AC 4). `draft.bin` is fetched once or twice per edit session, so on-the-fly compression is left to Caddy (F-22). An open file can still be read after a delete removes it, so a download that has already started finishes |
| O-9 | **Original download.** The bytes are the stored `original.fit` exactly (AC 4): the stripped file of UP-9, with no transformation. Only the owner can download it, whatever `allow_download` says. Viewers get `export.{hash}.fit` from `public/` (F-24), never this file. `Content-Disposition: attachment` with an ASCII file name (OQ-59) | SH-8, NFR Security. `allow_download` belongs to F-24's viewer export |
| O-10 | **Delete** calls `lifecycle.Service.Delete`, unchanged: the row first, then the folders (ADR-022 item 6). Freed bytes are released only by the call that removed the row (F-10 S-6). Any error is `500`: either the row is still there, or the folder stays until the next start removes it. One INFO line `workout deleted` with `id` and `by` (`owner` here; F-21 and F-27 add their own values) | I-4, AC-4. The log line lets the operator see deletes without seeing any user data |
| O-11 | **F-15 hook.** `lifecycle` defines one port, "published workout changed", which receives the record before and after the change. It is nil until F-15 implements it, and nil means nothing to do. It is called under the O-7 lock after every successful `PATCH` of a published workout, also when nothing changed. If it fails, the answer is `500` and the row stays changed. Saving again calls the hook again | F-15 decides which files a change affects (title → page; `hide_ends` → data file; lifetime → page, F-21 AC 4). Calling it every time gives the owner a way to retry, and keeps F-15 free of a change log |
| O-12 | **Bounded requests.** Every owner handler sets its own read and write deadlines (`http.ResponseController`, as ADR-021 item 5; the server has none). The write deadline is long enough for a 20 MB download on a slow phone. The `PATCH` body is read through a small fixed cap, a few KiB above the largest valid body with JSON escapes. The engineer picks the values | F-01 left the server-wide timeouts unset on purpose. A slow client must not hold a connection without limit |
| O-13 | **Logging.** No line for reads, updates or refusals. Server failures are ERROR with `id` (internal) and a stage. A line never contains the `public_id`, the token or its hash, the title, the description, or request metadata | TD Conventions (Logging), I-5, I-7, ADR-020 item 2 |

## What F-12 verifies now (AC 5, AC 6)

The `reports` table (F-27), `public/` (F-15) and Caddy (F-22, F-31) do not exist yet.

- **AC 5.** One end-to-end test through `newApp`: upload, delete, then check that the row is gone, `private/{id}` is gone, the storage total dropped by the folder's bytes, and every owner endpoint answers not found, including a second `DELETE`. The test keeps its list of places that hold workout data in one spot. F-15 adds the public folder to that list, and F-27 adds `reports`. F-21 AC 5 later turns the list into the complete check.
- **AC 6, Go side.** After an upload, a test requests guessed paths through `newApp`'s handler: the file names under `/`, `/w/{public_id}/`, `/private/{id}/`, `/api/workouts/{public_id}/` without a token or with the token in the query, and with the internal ID instead of the `public_id`, plus encoded traversal forms. No answer is `2xx` with either file's bytes. The existing mode checks (`private/` 0700, files 0600) stay. F-12 creates nothing under `public/`.
- **AC 6, Caddy side.** This is F-22's AC "not reachable through Caddy under any path", checked again in F-31. Once F-15 writes `public/`, its test walks `public/{public_id}/` and asserts that no file there has the bytes of either private file (OQ-60).

## Hand-offs

| To | What |
| --- | --- |
| F-15 | Publish at `POST /api/workouts/{public_id}/publish`: ADR-025 check, under the O-7 lock. Implement the O-11 port. It must fail toward less public content: when a change hides more (`hide_ends` on, download off), old public files must never stay served because a step failed. Remove first, then write. Regeneration must be idempotent, so that a re-save converges. Errors from the hook and from `Delete` must not carry a `public_id` or a `public/{public_id}` path, because `ownerFail` logs `err.Error()` (O-13; wrap path errors, QA-12-r1-3). Extend `Delete` and `Recover` with `public/{public_id}` (F-09, F-10 hand-offs). Add the public folder to the AC 5 test and the `public/` byte check to AC 6 |
| F-15, F-27, F-30 | The O-7 lock is per process. A delete by `workout-explorer delete` (F-27), or a `regenerate` run (F-30), in another process can race a hook in `serve`. Constraint: a public folder never outlives its row by more than SH-10's 60 s. For example, a hook checks after writing that the row still exists, and removes the folder if it does not. F-27 adds `reports` to `DeleteWorkout`'s transaction (ADR-022 item 6) |
| F-19, F-20 | The contract above. Count code points, or use `maxlength`. Show `message` for `400` and `500`. A `404` from any owner endpoint shows the generic page. The Share button uses `view_url`. Download the original with `fetch` and a Blob, because the token header rules out a plain link (ADR-025 "Would choose differently if") |
| F-21 | Expiry deletes through `Delete` under the O-7 lock, with `by` set to `expiry`. The owner API already treats an expired row as not found |
| F-22, F-31 | Caddy proxies `/api/*`. No `file_server` root above `public/`. `Referrer-Policy: no-referrer` on every response (Go does not repeat it). Never `log_credentials`. Caddy's `encode` may compress `draft.bin`. Access logs, if enabled, contain the `public_id` of owner paths, never the token. Proxy timeouts at or above O-12's write deadline |
| Requirements | TD Code structure, Lifecycle row: add the owner operations and the hook port. TD Glossary: "Not-found response" (ADR-025). F-12 AC 5 and AC 6, depending on OQ-60 |

## Open questions

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-59 | File name of the owner's original download | Decided 23 Oct 2026: see the decision log in `features/README.md` | — |
| OQ-60 | AC 5 and AC 6 scope | Decided 23 Oct 2026: see the decision log in `features/README.md` | — |
