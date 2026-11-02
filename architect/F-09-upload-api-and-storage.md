# F-09 Upload API and workout storage: architecture

- **Version:** 1.13 (documentation only, 30 Oct 2026: no private state, ADR-019 rev. 2; no first-open marker, ADR-020 rev. 2)
- **Status:** accepted. No open questions (section Open questions). F-09 passed QA round 4 (ready); its five r4 follow-ups are routed separately. The parse time limit as a server setting (D-30, the user's decision of 22 Oct 2026) is a change for the backend engineer.
- **Date:** 16 Oct 2026; v1.11 of 22 Oct 2026
- **History:** [architect/history/F-09-upload-api-and-storage.md](history/F-09-upload-api-and-storage.md) (change log, superseded text, decided questions, engineer round lists 1 to 8 and the v1.11 work list)
- **Feature:** [`features/F-09-upload-api-and-storage.md`](../features/F-09-upload-api-and-storage.md) (revised 16 Oct 2026: UP-9 with the whole user profile removed, UP-10, 13 ACs including the framing AC 7)
- **Depends on:** TD (system-wide picture), [F-03 design](F-03-workout-data-file-writer.md) v1.4, [F-08 design](F-08-fit-validation-and-recovery.md) v1.7, [ADR-003](ADR-003-fit-decoding-library.md) rev. 6, [ADR-004](ADR-004-parser-corpus-and-golden-reports.md) rev. 11, [ADR-005](ADR-005-workout-data-file-format-v1.md) rev. 1, [ADR-006](ADR-006-brotli-pre-compression.md), [ADR-017](ADR-017-parse-isolation-and-limits.md) rev. 7, [ADR-018](ADR-018-parse-outcomes-messages-and-log.md) rev. 6; new: [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) rev. 1, [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md), [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) rev. 4, [ADR-022](ADR-022-private-workout-files-staging-and-recovery.md) rev. 2, [ADR-023](ADR-023-identifying-data-stripped-by-a-byte-level-rewriter.md) rev. 9
- **Requirements:** UP-1 (server), UP-2 (messages through), UP-3 (persist the data end), UP-7 (default lifetime 7 days, at most 30), UP-9 (identifying data removed, framing repaired), UP-10 (upload messages), AP-10 (zones not from the user profile), AC-1, SH-1, SH-2 (draft state), NFR Security (128-bit IDs, hashed tokens, 20,000,000 bytes, 60 uploads per hour per IP with refused uploads counted and IPv6 per /64, uploads processed at once, parse limits), NFR Privacy (store as little as possible; no device identifiers or user profile from the file); TD Publish and view flow (Publish 1–3), TD Data model, TD Security and privacy (IDs and tokens, Uploads, Originals, Draft workouts)
- **Owner:** backend (senior-backend-engineer)

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | SQLite through the pure-Go `modernc.org/sqlite` v1.60.1, WAL, one connection, `<data>/workouts.db`; forward-only numbered migrations in `PRAGMA user_version` | ADR-019; section Schema |
| D-2 | Schema v1 is the TD's `workouts` table, `STRICT`, Unix seconds, `CHECK`s for the invariants; **adds** `data_end_s` (UP-3), stores TD `time_zone` as `time_zone_offset_s`; trim columns wait for phase 2 | ADR-019; section Schema |
| D-3 | Internal `id` = 128-bit random, 32 hex; `public_id` and edit token = 128-bit random, base62 in `math/big` digit order, fixed 22 characters; hash = SHA-256 of the token's ASCII | ADR-020 |
| D-4 | Viewing link `/w/{public_id}`; edit link `/edit/{public_id}#{token}`; both root-relative in the response; the token goes to the API later only as `Authorization: Bearer` (F-12) | ADR-020; section Response |
| D-5 | Upload cap 20,000,000 bytes of file content; multipart read as a stream into memory, never `ParseMultipartForm`; refused by `Content-Length` before any byte is read | ADR-021; section `POST /api/workouts` |
| D-6 | 60 per rolling hour per client address (IPv6 per /64); every attempt past the rate step counts; client from the last `X-Forwarded-For` entry only when the peer is loopback; limiter keys are seeded `maphash` values, never addresses | ADR-021; section Client address and rate limit |
| D-7 | At most N uploads in flight per process, N from `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` (default 4, 1–16, invalid → `serve` exits 2); read deadline 120 s, write deadline 120 s + 6·T per request, T the parse time limit (180 s at the default, D-30) | ADR-021 rev. 1, rev. 4; section Wiring |
| D-8 | Files staged in `tmp/{id}/`, `fsync`ed, renamed to `private/{id}/`; the row is inserted last; a folder without a row is garbage, swept at startup | ADR-022 |
| D-9 | `draft.bin` only, no `.br` | ADR-022 item 2 |
| D-10 | New packages `internal/lifecycle` (domain and application service), `internal/store` (SQLite adapter), `internal/privatedir` (private files adapter); `httpapi` gains the upload endpoint and the limiter; `ingest` gains `Accept` and the strip | section Packages |
| D-11 | `serve -data <dir>` is required; `make dev-server` passes `build/data` | section Wiring |
| D-12 | The upload request carries only the file. Title, lifetime, hiding and download are set on the draft's edit page (F-12, F-19), which meets UP-7's and SH-8's "at upload" (**confirmed by the user**) | section `POST /api/workouts` |
| D-13 | Invariant I-7: no request metadata (address, user agent, file name) is stored or logged | TD Architecture (Invariants); ADR-021 item 11 |
| D-14 | (v1.1, UP-9) `ingest.Accept` = parse the upload (F-08 outcome and message) → strip → reparse the stripped bytes → verify the same workout (golden-report digest). `draft.bin` and the row come from the reparse; `original.fit` is the stripped bytes. One parse slot, one log line | ADR-023 item 1; section Strip |
| D-15 | The strip is our own byte-level rewriter in `ingest`, not the library's encoder: blanks serial numbers, ANT device numbers and ANT IDs in place with the invalid value of the type the decoder reads (D-23) (transmission types kept), drops data messages 3 (`user_profile`), 29, 80, 81 and 149 (D-19), rewrites data size and CRCs, writes a truncated sequence in the unclosed form, drops trailing bytes | ADR-023 items 2 to 7 |
| D-16 | A framing disagreement (always `errRecordCount`, D-20, D-21), a rejected reparse or a different workout refuses the upload as `internal` ("This file could not be read."); the unstripped upload is never stored or logged. New invariant I-8. Two exceptions: a reparse time limit is the upload's `limit`/`time` (D-22), and the deliberate refusals U1 and U2 are `unreadable` (D-26) | ADR-023 item 8; TD Architecture (Invariants) |
| D-17 | Memory. During the strip and the reparse, nothing reachable points into the first parse: `run` clears its workout, and the outcome keeps log facts as values, not the listener. No GC setting in code. Server worst case M(N) ≤ 2·A + (N − 2)·B, plus L (the rate table at its cap, ≤ 40 MB). `GOMEMLIMIT`, if used, is set only by F-31's unit | ADR-017 rev. 3, rev. 4; ADR-021 rev. 3; section Memory |
| D-18 | (v1.4) Once the folder is in place, the client cannot cancel the upload: insert and existence check run with `context.WithoutCancel(ctx)`. A folder is removed after a failed insert only when `WorkoutExists` says the row is absent. `Recover` deletes rows without a folder | ADR-022 rev. 2 items 4, 5; section Upload pipeline |
| D-19 | (v1.4, UP-9 literal) The blank table adds 82/2, 148/5, 151/9, 151/10, 319/0, 323/0; (v1.5, OQ-52 decided "blank") and 148/4, 151/4, 151/8; the remove list is {3, 29, 80, 81, 149}. `TestStripTable` classifies every identifier-like field of the pinned profile as blanked or kept (`keptIdentifiers`) | ADR-023 rev. 4 items 4, 4a, 5 |
| D-20 | (v1.5, QA-09-r2-1, user: cut) A truncated sequence is stored up to the last record the decoder accepted, counted per sequence by a record counter on both decoder hooks (`seqSpan.records`). Nothing after it is kept: not a rejected record, not an incomplete one, not the bytes behind them. A framed count that differs from the decoder's fails closed (`errRecordCount`) | ADR-023 rev. 4 items 2, 6, 6a, 8; ADR-003 rev. 5 |
| D-21 | (v1.6, QA-09-r3-1: accept) A **complete** sequence is framed by `records` too: the first `records` records from the data start, read up to the end of the upload, because the decoder reads the last record in full even when the header's data size ends inside it. Fail closed unless the framed records end where the decoder's loop ends. Data size = the records written, fresh CRCs. Spans may overlap in a crafted file; each is written from its own bytes | ADR-023 rev. 5 items 2, 6, 7, 8 |
| D-22 | (v1.6) The reparse applies every ADR-017 limit except `MaxBytes`: the stored file can be longer than the upload, within ADR-023 rev. 5 item 7's bound. A reparse that hits the time limit is reported as the upload's `limit`/`time`, not as `internal` | ADR-017 rev. 6; ADR-018 rev. 4 |
| D-23 | (v1.7) A blanked field is filled by its **read type**: the profile's base type and array flag (the decoder ignores the declared base type of a known field), else the declared one. Whole elements get the read type's invalid value; trailing bytes and fields shorter than the type get 0x00. A non-z field declared shorter than its type reads as 0 (no invalid encoding exists) and is accepted. (v1.8) A field declared with size 0 is dropped by the decoder and reads as **absent**, for every type. It is not a short field | ADR-023 rev. 7 item 4; section Strip |
| D-24 | (v1.8) **The strip reads every byte exactly as the pinned decoder does.** Where the decoder accepts something unusual, the strip mirrors it. `errRecordCount` always means a divergence, a defect. A file `Parse` accepts is refused only through D-26. The framing audit table is the normative mapping, one row per decision, with the decoder's `file:line`. A framing change adds or edits its row in the same change. Architecture byte: little-endian only when it is 0, big-endian otherwise | ADR-023 rev. 7 items 3, 3a; section Strip, "Framing audit" |
| D-25 | (v1.8) **Anti-drift.** Every framing decision lives in one pure iterator in `ingest/frame.go`, which uses the library's own constants and `proto.LocalMesgNum`. `stripSeq` and the U2 classifier (D-28) are its only production callers. A test-only cross-check (`crossCheckFraming`) decodes each span with the library and compares it with the iterator, record by record: kind, header, local type, global number, byte order, field and developer field lists, data length, and for every blank field the offset, size, read type and byte order. It runs in `FuzzAccept`, `TestCorpus`, `TestStripCorpus`, `TestAcceptDataSizeSweep`, `TestBlankFill` and the new deterministic `TestFramingSweep` | ADR-023 rev. 7 item 3b; section Tests |
| D-26 | (v1.9, user: acceptance rule; v1.10, narrowed by the user on 22 Oct 2026) **Deliberate refusals.** No file is promised acceptance, from a real device or not: what errors a device may write cannot be known in advance. The corpus and the local real files stay strict in the tests (D-29) as a regression check, not as a product promise. A file that `Parse` accepts may be refused by `Accept` only for a cause on ADR-023 item 8a's closed list. The kind is the new `unreadable` ("This file could not be read.", WARN, `422`), never `internal`, which now always means a defect. Not `not_fit`, because its sentence ("This file is not a FIT file.") would be false and a kind has one sentence (ADR-018 rev. 5). Privacy is unchanged: refuse when unsure, never store on a guess | ADR-018 rev. 5; ADR-023 rev. 8 item 8a; section Strip, "Deliberate refusals" |
| D-27 | (v1.9) **U1, empty complete sequence**: status complete, `records` = 0. It has no faithful stored form (data size 0 means unclosed). `strip` refuses it with `errEmptySequence` before it writes any byte. `Accept` maps that to `unreadable`. `walk`, `Parse`, `Integrity` and the reference do not change | ADR-023 rev. 8 items 6, 8a |
| D-28 | (v1.9, OQ-54 closed by the user: refuse) **U2, removed timestamp.** It is classified only after a reference mismatch, by `removedTimestampDependency`. If that returns true the refusal is `unreadable` (`errRemovedTimestamp`); otherwise it stays `internal`. The set of refused files does not change | ADR-023 rev. 8 item 8a |
| D-29 | (v1.9) **Test properties under the acceptance rule.** Corpus files (synthetic and local real) and `TestAcceptDataSizeSweep`: strict, any refusal fails. This strictness is a regression check: the built code accepts each of these inputs, so a refusal is a change in our behaviour that must be explained. It is not a promise that files from real devices are accepted. `FuzzAccept` and `TestFramingSweep`: accepted with a faithful stored file, or refused as U1 or U2; `internal`, `errRecordCount` and an unlisted `unreadable` cause fail. Convergence: privacy and divergence findings stop and escalate by class; the other classes go by the normal route | section Tests; section Fuzz findings, convergence and mutants |
| D-30 | (v1.11, user: the time limit is configurable) **Parse time limit as a server setting.** `WORKOUT_EXPLORER_PARSE_TIMEOUT`, whole seconds, default 10 (from `ingest.DefaultLimits.Timeout`), 5 to 60. Invalid → exit 2 before anything is opened. Read in `settings.go` by `serve`, `parse` and `regenerate` (F-30), and passed as `ingest.Limits{Timeout: t}`; every other limit stays fixed. The upload write deadline is 120 s + 6·T. Memory, slots, the in-flight cap, `Retry-After`, and ingest's message and log line are unchanged; `server started` gains `parse_timeout_s` | ADR-017 rev. 7; ADR-021 rev. 4; section Wiring |

## Boundary

| In F-09 | Not in F-09 (owner) |
| --- | --- |
| `POST /api/workouts`, size cap, rate limit, in-flight cap and its setting | Storage cap and `GET /api/status` (F-10): a gate slot is reserved first in the check order |
| `workouts` table, migrations, store adapter | `reports` table (F-27); trim, zones, pins (phase 2) |
| Stripping identifying data before storing (UP-9), `ingest.Accept` | Telling users what is removed (F-29); serving the stored file to its owner (F-12, F-20) |
| Creating `private/{id}/original.fit` and `draft.bin`; startup recovery of `tmp/`, orphan folders and (v1.4) rows without a folder | Reading `draft.bin` and `original.fit`, settings, delete (F-12); expiry (F-21); anything under `public/` (F-15) |
| Generating `id`, `public_id`, token; formatting both links | Checking the token (F-12); the edit page at `/edit/…` (F-19, F-20); Caddy routes (F-15 minimal, F-22, F-31) |
| Persisting `Integrity.DataEnd` as `data_end_s` | Showing the UP-3 notice (F-19) |
| The viewing link of a draft is not served by Go and nothing exists under `public/` | The generic page itself (F-15) |

## Packages and files

```
src/server/
├── go.mod, go.sum                        + modernc.org/sqlite v1.60.1 (ADR-019)
├── cmd/workout-explorer/
│   ├── main.go                           serve: -data flag, the environment setting; open store, privatedir, parser, lifecycle; Recover; handler
│   ├── wiring.go                         newApp(dataDir, settings, logger) (handler, close func, error): the one wiring, used by serve and tests
│   ├── settings.go                       maxConcurrentUploads(getenv func(string) string) (int, error): the only environment read; (v1.11) parseTimeout, readSettings, newParser
│   ├── parse.go                          + -original <path>: writes Accept's stripped file (exit 3 on rejection, as today)
│   └── upload_test.go, settings_test.go  end-to-end and setting tests (section Tests); upload_test.go may import modernc.org/sqlite to read the database (ADR-019 rev. 1)
├── internal/ingest/                      (v1.1)
│   ├── accept.go                         Accepted, StripCounts, (*Parser).Accept: parse, reference, strip, reparse, verify (ADR-023 item 1)
│   ├── frame.go                          (v1.8, D-25) pure framing iterator: record kind, local type, definitions, data length, blank spots; one function per audit row, each citing the decoder line (ADR-023 rev. 7 items 3, 3a)
│   ├── strip.go                          rewrite per sequence over frame.go's records (ADR-023 items 2, 5 to 7)
│   ├── frame_test.go                     (v1.8) crossCheckFraming, TestFramingSweep, the framing probe
│   ├── strip_table.go                    fields blanked, messages removed, identifiers kept (v1.4), invalid-value patterns (ADR-023 items 4, 4a, 5)
│   ├── sequence.go                       walk also records each sequence's span, status and (v1.5) accepted record count for the strip
│   └── strip_test.go, accept_test.go     section Tests
├── internal/lifecycle/                   NEW. Workout record, identity, links, defaults; Upload and Recover. No SQL, no os
│   ├── lifecycle.go                      Service, Deps, New, Upload, Recover, Created
│   ├── record.go                         Record, Visibility, NewDraft (pure), DefaultLifetime, MaxLifetime
│   ├── identity.go                       Identity, NewIdentity, HashToken, base62, ViewPath, EditLink
│   ├── ports.go                          Parser, Repository, PrivateFiles, File, file names
│   └── *_test.go
├── internal/store/                       NEW. The only production importer of modernc.org/sqlite
│   ├── store.go                          Open, Close, InsertWorkout, WorkoutExists, DeleteWorkout (v1.4), WorkoutIDs
│   ├── migrate.go                        embedded migrations, user_version
│   ├── migrations/0001_workouts.sql
│   └── *_test.go
├── internal/privatedir/                  NEW. The only writer under <data>/private and <data>/tmp
│   ├── privatedir.go                     Open, Create, Remove, IDs, ClearStaging
│   └── privatedir_test.go
├── internal/httpapi/
│   ├── httpapi.go                        NewHandler(Config); health unchanged
│   ├── upload.go                         POST /api/workouts
│   ├── clientaddr.go                     client address and limiter key
│   ├── ratelimit.go                      sliding-log limiter (reused by F-27)
│   ├── respond.go                        JSON success and error bodies, codes and messages table
│   └── *_test.go
├── internal/datafile/
│   └── encode.go                         + SummaryJSON(workout.Workout) ([]byte, error): exactly the bytes of the metadata's "summary" value; no format change
├── tools/corpusgen/                      + identifyingData() (section Corpus); F-08 v1.6 lap override
├── internal/ingest/testdata/fuzz/FuzzAccept/  (v1.5) the three QA-09-r2-1 inputs, (v1.6) the two QA-09-r3-1 inputs (Go's fuzz format; synthetic, ADR-004 rev. 9)
└── testdata/corpus/                      + synthetic/identifying-data.fit, (v1.5) synthetic/ride-30min-bad-basetype.fit, (v1.6) synthetic/ride-30min-size-short.fit, their goldens; README manifest rows
```

Dependencies (TD Code structure, Dependency rules): `cmd → httpapi, lifecycle, store, privatedir, ingest`; `httpapi → lifecycle, ingest` (the `*ingest.Error` type and kinds only, never `Parse` or `Accept`); `lifecycle → workout, ingest` (the `Accepted` and `Error` types; the parser arrives as the `Parser` interface), `datafile`; `store → lifecycle, modernc.org/sqlite`; `privatedir → lifecycle` (the `File` type and the ID check). `lifecycle` imports neither `database/sql` nor `os`. Only `ingest` reads or writes FIT bytes.

**Third-party module (TD Code structure, Dependency rules):** `modernc.org/sqlite`. The standard library has no SQLite; the binary must stay `CGO_ENABLED=0`; it is the widely used pure-Go driver (ADR-019). The engineer reports the binary size before and after. The strip adds no module: it uses `muktihari/fit`'s `mesgnum`, `fieldnum` and `kit/hash/crc16` (ADR-003 rev. 4).

## Data directory

ADR-022 item 1 is the layout. `serve` creates `<data>` (mode 0750) if missing, then `privatedir.Open` creates `private/` and `tmp/` (0700). Nothing in F-09 creates `public/`. Only stripped bytes are ever written (ADR-022 rev. 1).

## Schema

`internal/store/migrations/0001_workouts.sql`, exactly:

```sql
CREATE TABLE workouts (
    id                 TEXT    PRIMARY KEY CHECK (length(id) = 32),
    public_id          TEXT    NOT NULL UNIQUE CHECK (length(public_id) = 22),
    edit_token_hash    BLOB    NOT NULL CHECK (length(edit_token_hash) = 32),
    title              TEXT    NOT NULL DEFAULT '',
    description        TEXT    NOT NULL DEFAULT '',
    sport              TEXT    NOT NULL,
    start_time         INTEGER,
    time_zone_offset_s INTEGER,
    visibility         TEXT    NOT NULL CHECK (visibility IN ('draft', 'unlisted', 'private')),
    hide_ends          INTEGER NOT NULL DEFAULT 0 CHECK (hide_ends IN (0, 1)),
    allow_download     INTEGER NOT NULL DEFAULT 0 CHECK (allow_download IN (0, 1)),
    summary_json       TEXT    NOT NULL CHECK (json_valid(summary_json)),
    data_end_s         INTEGER CHECK (data_end_s >= 0),
    created_at         INTEGER NOT NULL,
    published_at       INTEGER,
    expires_at         INTEGER NOT NULL,
    CHECK ((visibility = 'draft') = (published_at IS NULL)),
    CHECK (expires_at > created_at AND expires_at <= created_at + 2592000)
) STRICT;

CREATE INDEX workouts_expires_at ON workouts (expires_at);
```

(v1.13) A later migration narrows `visibility` to `draft` and `unlisted` and drops `private` rows ([ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) rev. 2 item 8). `0001` itself is never edited.

| Column | Value written by F-09 | Source |
| --- | --- | --- |
| `id`, `public_id`, `edit_token_hash` | `NewIdentity` | ADR-020 |
| `title`, `description` | `''` (UP-4: empty title shows sport and date, F-13) | — |
| `sport` | `Workout.Sport` (`''` if unknown) | F-08 R-9 |
| `start_time` | `Workout.StartTime` as Unix seconds; `NULL` if zero | F-02 R-1 |
| `time_zone_offset_s` | `TimeZone.OffsetSeconds` if `Known`, else `NULL` | F-02 decision 13 |
| `visibility` | `draft` | SH-2 |
| `hide_ends`, `allow_download` | 0, 0 | SH-4, SH-8 defaults |
| `summary_json` | `datafile.SummaryJSON(w)`: same shape and units as the data file's `summary` (F-03 §5.2) | TD Data model |
| `data_end_s` | `Integrity.DataEnd` if `Integrity.Truncated`, else `NULL` | UP-3, F-08 hand-off |
| `created_at` | request time, UTC, truncated to the second | — |
| `published_at` | `NULL` | — |
| `expires_at` | `created_at` + 604,800 (7 days) | UP-7 |

`w` is always `Accepted.Workout`, the reparse of the stripped file (D-14).

`PRAGMA table_info` shows `id` as not-null although the SQL above does not say `NOT NULL`: in a `STRICT` table a `PRIMARY KEY` column cannot be `NULL`. `TestSchema` expects not-null for `id` (v1.3).

## Identifiers and links

ADR-020 is normative. Summary for implementation:

| Value | Bytes | Text | Where it appears |
| --- | --- | --- | --- |
| `id` | 16 from `crypto/rand` | 32 lowercase hex | folder name, row, log lines |
| `public_id` | 16 from `crypto/rand` | base62 of the big-endian 128-bit value, digits as `big.Int.Text(62)` (`0-9a-zA-Z`), left-padded with `0` to 22 | row, both links, `public/` folder (F-15); never logged |
| edit token | 16 from `crypto/rand` | as `public_id` | only the edit link's fragment in the one upload response |
| `edit_token_hash` | 32 | SHA-256 of the token's 22 ASCII bytes | row only |

Links: `ViewPath(p) = "/w/" + p`; `EditLink(p, t) = "/edit/" + p + "#" + t`.

## `POST /api/workouts`

**Request.** `Content-Type: multipart/form-data; boundary=…`. Exactly one part, form name `file`, holding the FIT bytes. The part's file name and content type are ignored and never stored or logged. No other part is accepted (D-12).

**Order of checks** (ADR-021 item 3):

| # | Step | On failure |
| --- | --- | --- |
| 0 | (F-10) storage gate | F-10 |
| 1 | Try to take one of N in-flight slots (`Config.MaxConcurrentUploads`); released when the handler returns | `503 busy`, `Retry-After: 30` |
| 2 | Rate limit for the client key; counts this attempt if allowed | `429 rate_limited`, `Retry-After` |
| 3 | Read deadline now + 120 s, write deadline now + 120 s + 6 × `Config.ParseTimeout` (v1.11; 180 s at the default 10 s, ADR-021 rev. 4) (`http.ResponseController`; ignore `http.ErrNotSupported`) | — |
| 4 | Media type `multipart/form-data` with a boundary (`mime.ParseMediaType`) | `400 bad_request` |
| 5 | `r.ContentLength > maxBodyBytes` (`MaxUploadBytes + 16 KiB`) | `413 too_large`, no body byte read |
| 6 | `r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)`; first part has form name `file`; read it through `io.LimitReader(part, MaxUploadBytes+1)` into a buffer (pre-size from `Content-Length` when known, capped at `MaxUploadBytes`) | more than `MaxUploadBytes` read, or `*http.MaxBytesError`: `413 too_large`; any other read or multipart error, including a body cut off mid-file: `400 bad_request` |
| 7 | `NextPart()` returns `io.EOF` | `400 bad_request` |
| 8 | `Uploads.Upload(r.Context(), data)` (section Upload pipeline) | mapped below |
| 9 | `201` with the two links | — |

A 0-byte file passes steps 4 to 7 and is refused by the parser as `not_fit`.

**Response, success:** `201 Created`, `Content-Type: application/json`, `Cache-Control: no-store`.

```json
{"view_url":"/w/4kQ0dTqv1Zr8mN2xYbL7Ce","edit_url":"/edit/4kQ0dTqv1Zr8mN2xYbL7Ce#P9sTa0Wm3Hq5Lx2Kd8Rf1V"}
```

(illustrative values). No other key: the token appears only in the fragment of `edit_url` (AC 2).

**Response, failure:** same headers, body `{"error":"<code>","message":"<sentence>"}`. Messages for refusals not about the file's content are UP-10's, exactly:

| Situation | Status | `error` | `message` |
| --- | --- | --- | --- |
| Storage full (F-10) | 503 | `storage_full` | UP-8's text (F-10) |
| N uploads in flight | 503 + `Retry-After: 30` | `busy` | "Too many uploads right now. Please try again in a minute." |
| Rate limit | 429 + `Retry-After` | `rate_limited` | "Too many uploads. Please try again later." |
| Not multipart, no boundary, wrong or extra part, malformed body, body cut off; request context ended before the files were written | 400 | `bad_request` | "The upload could not be read. Please try again." |
| Over the cap | 413 | `too_large` | "This file is larger than 20 MB." |
| `ingest` `not_fit`, `unsupported_type`, `no_data`, `limit`, `internal` (including strip failures, D-16), (v1.9) `unreadable` (D-26) | 422 | the `Kind` | `ingErr.Message()` unchanged (ADR-018) |
| `ingest` `canceled` | 400 | `canceled` | `ingErr.Message()` (the client is usually gone) |
| Encode, file or database failure, ID collision | 500 | `server_error` | "Something went wrong on the server. Please try again later." |

The table lives in `respond.go` as one map from code to status and message; ingest's messages are never copied into it. `GET`, `PUT` and others on `/api/workouts` get the mux's `405`.

## Client address and rate limit

ADR-021 items 6 to 8 are normative.

```go
// clientaddr.go
func clientPrefix(r *http.Request) netip.Prefix // /32 for IPv4, /64 for IPv6, from the rules in ADR-021 item 6
func limiterKey(seed maphash.Seed, p netip.Prefix) uint64

// ratelimit.go
type limiter struct { /* mu, now func() time.Time, epoch time.Time, limit int, window time.Duration, maxKeys int, hits map[uint64]*keyLog, ... */ }
type keyLog struct { times []uint32; refused bool } // v1.4: times = whole seconds since epoch, rounded up; cap == limit, allocated once
func newLimiter(limit int, window time.Duration, maxKeys int, now func() time.Time) *limiter
// allow counts the attempt if it is allowed. retryAfter is set when refused;
// firstRefusal is true for the first refusal of a key since its last allowed attempt.
func (l *limiter) allow(key uint64) (ok bool, retryAfter time.Duration, firstRefusal bool)
```

Constants (`httpapi`): `MaxUploadBytes = 20_000_000`, `maxBodyBytes = MaxUploadBytes + 16<<10`, `uploadRateLimit = 60`, `uploadRateWindow = time.Hour`, `maxRateKeys = 100_000`, `uploadReadTimeout = 120 * time.Second`, `uploadWriteFactor = 6` (the write deadline is `uploadReadTimeout + uploadWriteFactor × Config.ParseTimeout`, computed at step 3). The in-flight cap is `Config.MaxConcurrentUploads`, and the parse time limit is `Config.ParseTimeout`; neither is a constant (section Wiring).

(v1.4, ADR-021 rev. 3 item 8) Representation, exactly:
- `epoch = now()` in `newLimiter`.
- `stamp(t) = uint32(ceil((t − epoch) / 1 s))`.
- An entry `s` is live while `now < epoch + (s · 1 s) + window`.
- `retryAfter = epoch + times[0] · 1 s + window − now` (`httpapi` rounds it up to whole seconds, as now).
- A new key gets `times: make([]uint32, 0, l.limit)`. Expired entries are dropped with `n := copy(k.times, k.times[i:]); k.times = k.times[:n]`. Never `k.times = k.times[i:]`.
- The sweep's expiry test uses the same rule.

Behaviour is unchanged for clocks on whole seconds since `epoch` (every fake-clock test). A real attempt counts for at most 1 s longer than the hour.

## Strip (UP-9)

ADR-023 is normative: where it runs, the framing, the field table, the invalid values, the frame rewrite and the failure rule. The public surface:

```go
// package ingest
type StripCounts struct{ BlankedFields, RemovedMessages int } // BlankedFields counts only fields whose bytes changed (v1.3): a field already invalid is not counted
type Accepted struct {
    Workout  workout.Workout // parsed from Original (the reparse)
    Original []byte          // the upload with UP-9 data removed; what is stored as original.fit
    Stripped StripCounts
}
// Accept parses upload, strips it, reparses the result and verifies the workout is unchanged.
// Every failure is an *Error: the upload's own rejection, or Kind internal with a "strip:" cause,
// (v1.9) or Kind unreadable with a "strip:" cause for a deliberate refusal (D-26).
func (p *Parser) Accept(ctx context.Context, upload []byte) (Accepted, error)
```

`walk` gains a per-sequence record `seqSpan{pos, headerSize, dataEnd int; status complete|truncated|skipped}` (status rules in ADR-023 item 2: a sequence that `walk` continues past is complete; one that ends the walk because of a patched header, a decode error or missing data is truncated; `skipped` is orthogonal and kept as a flag).

(v1.5, D-20) **`seqSpan.records`** is the number of records the decoder accepted in the sequence's last decoder call. Definitions and data messages both count, including data messages the strip removes.
- `sequence.go` holds the counter: `type recordCounter struct{ n int }` with `OnMesg(proto.Message)` and `OnMesgDef(proto.MessageDefinition)`, each doing `c.n++`.
- `decodeSequence` creates one per call, passes `WithMesgListener(l, c)` and `WithMesgDefListener(c)`, and returns `(records int, err error)`.
- After a header-CRC retry, the retry's count is the one stored.
- `stripSeq` uses it as ADR-023 rev. 4 item 6 says:
  - **truncated**: write the first `records` records and stop. If one of them cannot be framed, return `errRecordCount`.
  - **complete**: framed by `records` as well (v1.6, below).
- `errRecordCount` is the strip's framing sentinel. Its message names the sequence's byte offset and both counts.

(v1.6, D-21, ADR-023 rev. 5 item 6) **Complete spans are framed by `records` as well.** `sequence.go` does not change: `dataEnd` of a complete span stays the end the header declares (`pos + headerSize + DataSize`).
- Write the first `records` records from `pos + headerSize`. The framing may read up to `len(data)`, not only up to `dataEnd`. Reason: the decoder loops `for cur < DataSize` and reads each record in full (`decoder.go:1010-1016`), so the last record it accepts may end after `dataEnd`. If one of the `records` records cannot be framed within the upload, return `errRecordCount`.
- Let E be the end of the last written record (`pos + headerSize` when `records` is 0). **Loop-end check:** the framing must stop where the decoder's loop stopped. Both of these must hold, else `errRecordCount`:
  - (a) if `records` > 0, the last record starts before `dataEnd`;
  - (b) E ≥ `dataEnd`, or the record that starts at E runs past the end of the upload. That is a definition whose bytes do not fit, or a data message of a known local type whose length does not fit. A data message without a definition at E fails (b): the decoder would have rejected it, and the walk would have said truncated.
- Write the data size as the length of the records written (removed messages excluded), a fresh header CRC for a 14-byte header, and a fresh file CRC, as before. For every corpus file today E equals `dataEnd`, so the output does not change.
- **Overlap.** After an overrun, `walk` looks for the next sequence at `dataEnd + 2`, which can lie before E. That happens only in a crafted file whose next header sits inside the overrun record. `stripSeq` copies each span from `data` on its own, so the overlapping bytes are written twice, once per sequence, in the role each sequence's decoder read them in. The reparse then reads exactly what the first parse read. ADR-023 rev. 5 item 7 bounds the growth.
- **`errStripFrame` is removed.** It has no case left: every framing disagreement, in either status, is `errRecordCount`, and its text names the sequence's offset and what disagreed (the count, or where the loop ends). Tests that expected the `record framing` prefix now expect `errRecordCount`.
- `Parser.strip` wraps a strip error with `%w`, not `%v` (today `accept.go:44`), so `errors.Is(err, errRecordCount)` holds through `*Error.Unwrap`. The text of the cause does not change. (v1.3) `strip(data []byte, spans []seqSpan, stop func() error) ([]byte, StripCounts, error)` is the one loop over the spans, used by `Accept` and by the tests. It calls `stop` before each sequence and returns its error unchanged. `Accept` passes `ctx.Err` and maps that error to `canceled`; tests pass `nil` (never stop). `strip` imports no `context`. There is no second loop in `accept.go`.

**Memory (D-17, ADR-017 rev. 3).** In `run`, after the first parse: `ref := reference(w)`, then `w = workout.Workout{}`, then `strip` with `ref`. The strip step receives no workout. `outcome` has no `*listener` field. `parse` fills `outcome` with value copies of the log facts (`fileType`, `device`, `garminProduct`, `recordMessages`) before it returns, on every path, and `errorOf` reads them from there. During the strip and the reparse, the only first-parse data still live is `o.spans`, `o.integ`, `o.dataEnd` and the 32-byte reference.

**Reparse limits (v1.6, D-22, ADR-017 rev. 6).**
- The reparse skips the `MaxBytes` check and applies every other limit unchanged. The stripped file can be longer than the upload: 2 bytes for a sequence whose CRC was missing, plus the overlap above. Before v1.6, an upload close to `MaxBytes` with a missing CRC was accepted by `Parse` and refused by `Accept`. How the reparse skips the check is the engineer's choice, for example a field of the reparse's `outcome`. The first parse still checks `MaxBytes` on the upload.
- A reparse rejected with `KindLimit` and limit `time` returns the upload's error `p.errorOf(o, KindLimit, limitTime, cause)`. It is logged at WARN as `fit rejected` with `limit` `time` (ADR-018). A time limit depends on load, so a second decode can hit it when the first did not. Reporting it as `internal` would claim the file is unreadable. Every other reparse rejection stays `internal` (`reparse rejected (<kind>)`).

What is taken out (ADR-023 rev. 4): serial numbers, ANT device numbers and ANT IDs in any message, segment device IDs, the leaders' activity IDs and (v1.5, OQ-52) the segment account keys are blanked (item 4); every `user_profile` (3), saved-location (29), `ant_rx` (80), `ant_tx` (81) and `segment_leaderboard_entry` (149) data message is removed whole, developer fields on it included (item 5). Besides these, only the frame changes of item 6 (OQ-48, accepted) and (v1.5) the cut of item 6a: in a truncated sequence nothing after the last record the decoder accepted. (v1.6) In a complete sequence the data size is rewritten to cover exactly the records the decoder read, also when the header's data size ended inside the last one.

**Blank fill (v1.7, D-23, ADR-023 rev. 6 item 4).** Normative rule and decoder behaviour in the ADR. In code:
- `strip_table.go` builds, once at package init, the read type of every `blankTable` entry from `factory.StandardFactory().CreateField(mesg, field)`. That is the factory the parser's decoder uses: `sequence.go` passes no `WithFactory`. If `ingest` ever passes one, the strip must use the same instance (say so in a comment at both places).
- `readType(global uint16, field, declared uint8) basetype.BaseType` returns that table's type. For a field the profile does not know, it returns the declared base type, as the decoder does. It is unreachable for the pinned table, but it is kept so that the fill and the decoder cannot disagree after a profile change.
- `fillInvalid(v []byte, t basetype.BaseType, big bool)` takes the read type, not the declared one. It writes the read type's invalid value for each whole element. The bytes after the last whole element, a field shorter than one element, and a string are written as 0x00. There is no "not a multiple → all 0xFF" branch and no `!ok` branch: the read type is always a valid base type.
- `blankFields` passes `readType(d.global, f.num, f.baseType)`. Nothing else in `strip.go` changes, and `BlankedFields` still counts fields whose bytes changed.
- Every corpus file declares its blank fields with the profile's own type at a multiple of its size, so no stored corpus file changes.

**Framing audit (v1.8, D-24, ADR-023 rev. 7 item 3a).** This table lists every decision the walk, the framer and the fill make from upload bytes. Each row is compared with `muktihari/fit` v0.28.4 at source level: `decoder/decoder.go` unless another file is named, and `strip.go`/`sequence.go` as of round 6. It is normative. A change to any of these decisions edits its row in the same change, and so does a library upgrade that moves a cited line.

| # | Decision | Decoder rule | Strip / walk rule (round 6) | Verdict |
| --- | --- | --- | --- | --- |
| H1 | Header size | byte 0 is 12 or 14, else `ErrNotFITFile` (459-463) | `looksLikeFIT` (`sequence.go:27-29`); the strip copies `headerSize` from the span | same |
| H2 | `.FIT` signature | bytes 8-11 (477) | `looksLikeFIT` | same |
| H3 | Protocol and profile version | read (483-484), never checked | copied, never read | same: neither interprets them |
| H4 | Data size | little-endian `uint32` at bytes 4-7 (485); 0 gives `ErrNotFITFile` (489-491) | `parseHeader` little-endian (`sequence.go:31-33`); 0 means unclosed, decoded with the bytes present (F-08 R-c, `sequence.go:160-168`); written little-endian (`strip.go:128`) | same. The unclosed case is F-08's documented recovery, which `Parse` applies too |
| H5 | Header CRC (14-byte header) | over bytes 0-11; checked only with checksums on and a stored CRC ≠ 0 (465-466, 493-507) | walk retries without checks (`sequence.go:85-90`); strip writes a fresh CRC over bytes 0-11 (`strip.go:129-133`), or 0 in the unclosed form | same |
| H6 | File CRC coverage | 12-byte header: header and records. 14-byte: the CRC is reset after the header (499-502), so records only; compared after the last record (934-946) | over header and records for both sizes (`strip.go:134-136`) | **same result**: a CRC-16 over a block followed by that block's correct CRC, little-endian, is 0, which is the reset state. The strip writes the header CRC first. Keep it, with a comment that states this identity. `decodesToEnd` (checksums on) proves it on every stored file |
| S1 | Chained-sequence boundaries | the decoder is called once per sequence on `data[pos:]` | walk: next header at `pos + size + DataSize + 2` (`sequence.go:132`); the strip handles exactly the walk's spans | same by construction: `Parse` and the reparse use the same walk |
| S2 | Where a sequence's records end | `for cur < DataSize`, every record read in full (1009-1021, `readN` 948-958) | first `records` records, then the loop-end check (`strip.go:64-118`) | same (D-21) |
| S3 | Region of a truncated span | the records it accepted lie before `min(len, declared end)` | `region = data[…:dataEnd]` (`strip.go:49-53`) | same. If it were not, the strip would fail closed (`errRecordCount`); it cannot leak |
| R1 | Definition or data | definition iff `h & 0xC0 == 0x40` (530-531) | bit 7 first, then bit 6 (`strip.go:73-79`, `runsPast` 148-158) | same |
| R2 | Local type, compressed header | `(h & 0x60) >> 5` (627-630; `proto/proto.go:275-276`) | `(h >> 5) & 3` | same |
| R3 | Local type, normal header | `h & 0x0F` (543, 632); bits 4 and 5 of a data header are ignored | `h & 0x0F` | same |
| R4 | Developer-data flag | `h & 0x20`, read on definition headers only (583) | `b[0] & 0x20` in `readDefinition` (`strip.go:184`) | same |
| R5 | Missing definition | slot `Header == 0` gives `ErrMesgDefMissing` (633-635); slots reset per decoder (240-242) | `defs` nil gives `errRecordCount`; `defs` per `stripSeq` | same: unreachable within `records`, otherwise fails closed |
| D1 | Reserved byte | stored (545), never used | ignored | same |
| D2 | **Architecture byte** | **0 is little-endian, every other value big-endian** (550-554; field padding 727-731; developer padding 902-906; `proto/value_unmarshal.go`, `arch == LittleEndian` throughout) | `big := arch != proto.LittleEndian` (`frame.go`, since round 7) | **same.** It decides the global number (D3), so removal and blank selection, and the fill's byte order. Mirror the decoder; do not refuse, because `Parse` accepts such a file |
| D3 | Global number | 2 bytes in D2's order (550-554) | `strip.go:168-172` | follows D2: same |
| D4 | Field count, field definitions | count, then 3 bytes each: number, size, base type (556-571) | `strip.go:174-183` | same |
| D5 | Invalid declared base type | definition rejected, not counted (572-575) | not checked; never framed past `records`; in `runsPast` unreachable, because a decode error makes the span truncated | same through the count (D-20) |
| D6 | Field size 0 | dropped from the definition, no bytes read, the field is absent (576-579) | kept with size 0: adds 0 bytes; the fill writes nothing | same lengths and outcome. The ADR's "reads as 0" was wrong (ADR-023 rev. 7 item 4) |
| D7 | The same field number twice in one definition | both read and appended (700-753) | both blanked (`strip.go:209-221`) | same |
| V1 | Developer field definitions | count, then 3 bytes each: number, size, data index; size 0 dropped (583-611) | sizes summed (`strip.go:184-197`) | same |
| V2 | Field-description lookup | by (data index, field number) over the `field_description` messages read so far. Without a description the bytes are read and skipped; an invalid `fit_base_type_id` is an error (853-895) | developer bytes never interpreted; 206 and 207 never blanked or removed | same: lengths do not depend on the lookup, a rejection is an uncounted record, and the reparse sees the same descriptions |
| L1 | Data message length | 1 + the sizes of the active definition's fields and developer fields (703, 857) | `1 + dataSize` (`strip.go:76-77`, 91-93) | same |
| L2 | Local type redefined | slot overwritten (543-548) | `defs[h & 0x0F] = d` (`strip.go:84`) | same |
| U1 | Unknown global number | fields read with the declared types (709-721) | neither blanked nor removed unless on a list (29 is) | same |
| U2 | Unknown field number in a blank-table message | declared type | not blanked: only listed numbers are | same |
| F1 | Read type of a blank field | factory base type and array flag for a known field, the declared type otherwise (709-721) | `readTypes` from `factory.StandardFactory()` (`strip_table.go:57-76`) | same (D-23). The base type is enough: every whole element gets the invalid value and the rest gets 0x00, so the array flag cannot change the outcome |
| F2 | Padding of a short field | little-endian: bytes at the front; big-endian: at the back (724-732) | filled with zeros, so the side does not matter | same |
| X1 | Decoder state that carries across records | active timestamp from any field 253 (641-645, 742-746); first `file_id` (661-663); developer index bitmap and field descriptions (666-674); accumulator off with `WithNoComponentExpansion` | removal drops field-253 values; no blank-table field is 253; 206 and 207 never change | same, except the known OQ-54 case (U2 also covers a removed message with a compressed-timestamp header, QA-09-r4-1) |
| X2 | Checksum option | gates only the CRC writes and comparisons (465, 493, 503, 941, 954); the record stream is the same with or without it | — | used by the cross-check: it decodes with `WithIgnoreChecksum()` and sees the walk's records |
| S4 | (v1.9) Complete span with no record | the first record starts before `DataSize` and runs past the end of the bytes: EOF, nothing counted (1009-1021, `readN` 948-958); the walk calls it complete, CRC missing (`sequence.go:108`, `117-118`) | `strip` refuses it before writing (`errEmptySequence`, D-27) | **same reading.** It is not a divergence: it has no stored form, so it is a deliberate refusal (U1) |

**Result:** every row is the same rule. One row (H6) is the same only through a CRC identity and carries a comment. (The audit found one divergence, D2, fixed in round 7; history.)

**Framing iterator (v1.8, D-25, ADR-023 rev. 7 item 3b).** `frame.go` holds every row above that reads record bytes (R1-R5, D1-D7, V1, L1, L2, F1), and nothing else. It does no I/O and no output.
- One small function per decision. Each one's comment cites the decoder line of its audit row. Use the library's constants (`proto.MesgDefinitionMask`, `proto.MesgCompressedHeaderMask`, `proto.DevDataMask`, `proto.LittleEndian`) and `proto.LocalMesgNum(h)` for the local type, so that the expression is the decoder's own wherever the library exports it.
- `stripDef` gains `arch byte` (as read) and `blank []blankSpot{offset, size int; readType basetype.BaseType; big bool}`. (v1.9) Developer fields get their own element type, `stripDevField{num, size, devIndex uint8; offset int}`. The developer data index is not kept in a field named `baseType`. `blank` is computed once per definition from `blankTable` and `readType`, so `blankFields` no longer searches the table for each data message.
- `nextRecord(b []byte, defs *[16]*stripDef) (framedRecord, frameStatus)`, where `framedRecord{length int; header byte; definition bool; local uint8; def *stripDef}` and `frameStatus` is `framed`, `noDefinition` or `doesNotFit`. `stripSeq` and `runsPast` both use it; `runsPast` becomes `status == doesNotFit`. Output, removal, blanking and counts stay in `strip.go`.
- Exact names are the engineer's choice. The rule is: one function per decision, the iterator is the only place that reads a record byte, and `stripSeq` and the U2 classifier (D-28) are its only production callers.

**Deliberate refusals (v1.9, D-26 to D-28, ADR-023 rev. 8 item 8a).** ADR-023 item 8a is normative: the rule, the closed list and the reasons. In code:

```go
// errors.go: new kind (ADR-018 rev. 5)
KindUnreadable Kind = "unreadable" // message "This file could not be read."; logged as `fit rejected` at WARN

// strip.go
var errEmptySequence    = errors.New("empty complete sequence")                         // U1
var errRemovedTimestamp = errors.New("removed timestamp before a compressed-timestamp record") // U2 (OQ-54)
func emptyComplete(sp seqSpan) bool { return sp.status == seqComplete && sp.records == 0 }
func removedTimestampDependency(data []byte, spans []seqSpan) bool
```

- **U1 in `strip`.** Before the loop, and before the first `stop` call, `strip` checks every span with `emptyComplete`. On the first hit it returns `nil` output and `fmt.Errorf("%w: sequence at byte %d", errEmptySequence, sp.pos)`. `stripSeq` returns the same error if it is ever handed such a span, so a direct call cannot write the ambiguous 16 bytes. Both use the one predicate.
- **U2 in `Parser.strip`** (`accept.go`). This step runs only when the reference differs. `removedTimestampDependency` walks, per span, the first `records` records with `nextRecord` and `install`, as `stripSeq` does. It returns true if a data message that is kept and has a compressed-timestamp header (`h & proto.MesgCompressedHeaderMask != 0`) comes after a removed data message whose definition has field 253 with size above 0, or that has a compressed-timestamp header itself (QA-09-r4-1). Both must be in the **same** span, because each sequence gets a fresh decoder, which carries no timestamp over (audit X1). It is pure, reads only through `frame.go`, and is not called on the success path.
- **Mapping in `Parser.strip`:**

  | Situation | Result |
  | --- | --- |
  | `strip` returns an error wrapping `errEmptySequence` | `p.errorOf(o, KindUnreadable, "", fmt.Errorf("strip: %w", err))`. The `stripHook` is not called and there is no reparse |
  | Reference mismatch and `removedTimestampDependency` true | `p.errorOf(o, KindUnreadable, "", fmt.Errorf("strip: %w", errRemovedTimestamp))` |
  | Reference mismatch otherwise, `errRecordCount`, a rejected reparse other than `limit`/`time` | `internal`, as before |

- **Order inside `Accept`:** a canceled context first, then U1 (before the strip), then the strip, then a reparse `limit`/`time`, then U2 or `internal`.
- The log line follows ADR-018 rev. 5: `fit rejected`, WARN, `kind` `unreadable`, cause with an offset or the U2 text only.
- `log.go:61-64` already picks ERROR for `internal` alone and WARN for every other rejection, and `log.go:102` adds `panic` and `frames` for `internal` alone. So `unreadable` needs no change in `log.go`; `TestAcceptEmptySequence` proves it (one WARN line without `panic` or `frames`).
- `httpapi` and `lifecycle` do not change: every ingest kind except `canceled` is already `422` with `Message()`.

(v1.4) `strip_table.go` holds three tables: `blankTable`, `removeTable` and `keptIdentifiers` (ADR-023 item 4a; message number, field number, reason). `keptIdentifiers` is read only by `TestStripTable`. It is where an identifier is recorded as deliberately kept.

What stays the same in the stored file: every byte not covered by ADR-023 items 4 to 6a. In particular ANT transmission types (OQ-46), `sport.name`, `bike_profile.name`, `device_info.product_name`, `file_id.product_name`, developer data IDs, field descriptions and the developer field bytes of kept messages, every definition message (including those of removed messages), and the order of all kept messages.

## Upload pipeline (`lifecycle.Service.Upload`)

```go
type Parser interface {
    Accept(ctx context.Context, upload []byte) (ingest.Accepted, error) // *ingest.Parser satisfies it
}
type Repository interface {
    InsertWorkout(ctx context.Context, r Record) error
    WorkoutExists(ctx context.Context, id string) (bool, error) // v1.4: SELECT 1 … WHERE id = ?
    DeleteWorkout(ctx context.Context, id string) error          // v1.4: nil if absent; F-12/F-21/F-27 reuse it
    WorkoutIDs(ctx context.Context) (map[string]struct{}, error)
}
type PrivateFiles interface {
    Create(id string, files []File) error // fails if private/{id} exists; staged, fsynced, renamed; cleans its own staging on error
    Remove(id string) error               // private/{id}; nil if absent
    IDs() (ids []string, unknown int, err error)
    ClearStaging() (removed int, err error)
}
type File struct{ Name string; Data []byte }
const (OriginalName = "original.fit"; DraftName = "draft.bin")

type Deps struct {
    Parser Parser
    Repo   Repository
    Files  PrivateFiles
    Logger *slog.Logger
    Now    func() time.Time // nil: time.Now
    Rand   io.Reader        // nil: crypto/rand.Reader
}
func New(d Deps) *Service
type Created struct{ PublicID, EditToken string }
func (s *Service) Upload(ctx context.Context, upload []byte) (Created, error)
func (s *Service) Recover(ctx context.Context) error
```

| Step | Action | On error |
| --- | --- | --- |
| 1 | `acc, err := Parser.Accept(ctx, upload)`; then `w, original, counts := acc.Workout, acc.Original, acc.Stripped`; `acc` is not used again, and `w` is not used after step 3 (v1.3: so only the data file and the bytes are live during the file writes, ADR-017 rev. 3 item 4) | return `err` unchanged (`errors.As` finds `*ingest.Error`); nothing written; ingest has logged it (ADR-018) |
| 2 | `bin, err := datafile.Encode(w)`; `sum, err := datafile.SummaryJSON(w)` | log `upload failed` `stage=encode`; return a server error |
| 3 | `id := NewIdentity(Rand)`; `rec := NewDraft(w, sum, id, Now())` (pure) | `stage=identity` (reader failure) |
| 4 | If `ctx.Err() != nil`, stop and return `ctx.Err()`; `httpapi` answers `400 bad_request` (the client is gone) and logs nothing | nothing written |
| 5 | `Files.Create(id.ID, []File{{OriginalName, original}, {DraftName, bin}})` | `stage=files`; nothing left (Create cleaned up) |
| 6 | (v1.4, D-18) `sctx := context.WithoutCancel(ctx)`; `Repo.InsertWorkout(sctx, rec)`. From here on the request context is never consulted | Insert error → `ok, xerr := Repo.WorkoutExists(sctx, id.ID)`: <br>• `xerr == nil && ok`: the row is committed. Continue with step 7 as a success; the insert error is dropped and not logged. <br>• `xerr == nil && !ok`: `Files.Remove(id.ID)`, then `stage=store` with the insert error; if `Remove` fails, also `stage=cleanup` (startup recovery removes the folder). <br>• `xerr != nil`: **do not remove the folder**; log `stage=store` with the insert error and `stage=cleanup` with `xerr`. Startup recovery settles it either way. <br>Both error cases return `*ServerError{Stage: "store"}` |
| 7 | log `upload accepted`; return `Created{PublicID, EditToken}` | — |

`upload` (the unstripped bytes) is never passed to `Files` or the logger; after step 1 only `original` (the stripped bytes) is used. Errors that are not parser rejections are returned as `*ServerError{Stage, Err}` (v1.3, accepted). `httpapi` maps any error that is neither an `*ingest.Error` nor a context error to `500`.

`NewDraft` is pure and holds the defaults: `Visibility = draft`, `HideEnds = AllowDownload = false`, `PublishedAt = nil`, `CreatedAt = now.UTC().Truncate(time.Second)`, `ExpiresAt = CreatedAt + DefaultLifetime` (7 × 24 h); `MaxLifetime` (30 × 24 h) is exported for F-12.

A client that disconnects after step 4 never receives its links. Its draft is stored and stays until it expires (accepted by the user, OQ-45). `httpapi` answers `201` to a gone client, which is harmless.

Why step 6 cannot use the request context (QA-09-r1-1): `modernc.org/sqlite` returns `ctx.Err()` for a statement whose context ended before the driver's deferred check, even when the autocommit `INSERT` has already committed. Removing the folder on that error leaves a row with no files. `WithoutCancel` removes the cause, and `WorkoutExists` makes the cleanup safe against any other misreported error. No timeout is added (ADR-022 rev. 2 item 4).

`Recover` (ADR-022 rev. 2 item 5): `ClearStaging`; `IDs`; `WorkoutIDs`; `Remove` every folder ID without a row; (v1.4) `DeleteWorkout` every row ID without a folder; one INFO line `recovery` with `staging_removed`, `orphans_removed`, `rows_removed`, `unknown_entries`. Errors from `Remove` and `DeleteWorkout` are joined and returned after the line, as now. `Recover` runs with the startup context, not a request context.

**`internal/store`:** `Open(ctx, path string) (*DB, error)` (ADR-019 items 2 to 5); `(*DB).InsertWorkout`, `(*DB).WorkoutExists`, `(*DB).DeleteWorkout`, `(*DB).WorkoutIDs`, `(*DB).Close`. Times convert to Unix seconds at this edge; `nil` pointers to `NULL`. (v1.4) `Open` returns the error of closing the file it created (`store: %w`), instead of dropping it.

**`internal/privatedir`:** `Open(dataDir string) (*Dir, error)`; methods as `PrivateFiles`. Every ID is checked against `^[0-9a-f]{32}$` and every file name against `OriginalName`/`DraftName` before a path is built (Go 1.26 `os.Root` may be used for the folder operations).

## Wiring

**Flags and settings of `serve`:**

| Setting | Source | Default | Invalid |
| --- | --- | --- | --- |
| listen address | `-addr` | `127.0.0.1:8080` | empty → exit 2 (F-01) |
| data directory | `-data` | none, required | missing or empty → exit 2, `serve: -data must not be empty` |
| uploads processed at once | environment `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` | 4 (unset or empty) | not `^[1-9][0-9]*$` or above 16 → exit 2, `serve: WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS must be a whole number from 1 to 16` |
| (v1.11) parse time limit, whole seconds | environment `WORKOUT_EXPLORER_PARSE_TIMEOUT` | `ingest.DefaultLimits.Timeout`, 10 s (unset or empty) | not `^[1-9][0-9]*$`, more than 2 digits, or outside 5 to 60 → exit 2, `serve: WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60` |

`settings.go` reads the variables through an injected `getenv` (tests use a map; `main` passes `os.Getenv`). Nothing else in the binary reads the environment (TD Conventions, Configuration). Settings are read before the database is opened, so an invalid value changes nothing on disk. The `server started` log line gains `max_concurrent_uploads` and (v1.11) `parse_timeout_s`.

(v1.11, D-30, ADR-017 rev. 7) **The parse time limit.**
- `parseTimeout(getenv) (time.Duration, error)`: unset or empty → `ingest.DefaultLimits.Timeout`; otherwise the same strict whole-number check as `maxConcurrentUploads` (one shared helper), then 5 to 60, times `time.Second`. `cmd` does not repeat the number 10.
- `readSettings(getenv) (settings, error)` reads both variables, the in-flight cap first, and returns the first error. `serve` calls it once.
- `newParser(timeout time.Duration, logger *slog.Logger) *ingest.Parser` returns `ingest.NewParser(ingest.Limits{Timeout: timeout}, logger)`. It is the only `ingest.NewParser` call in `cmd`. `newApp` uses it with `s.parseTimeout`; zero, as in the older tests, means the default.
- `parse` reads `parseTimeout(os.Getenv)` before it opens the input. An invalid value prints `parse: WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60` and exits 2. It then builds its parser with `newParser(t, nil)`. The read size stays `ingest.DefaultLimits.MaxBytes + 1`. Its usage text names the variable in one line.
- F-30's `regenerate` uses `parseTimeout` and `newParser` the same way (hand-off).
- `httpapi.Config.ParseTimeout` gets `parser.Limits().Timeout`, the parser's effective value, so the write deadline and the parser can never disagree.

Order: settings → logger → `newApp` (create `<data>`, `store.Open`, `privatedir.Open`, `newParser(s.parseTimeout, logger)`, `lifecycle.New`, `Recover`, `httpapi.NewHandler`) → listen → serve → shutdown → `close` (database). Any `newApp` error is logged and exits 1, before "server started".

```go
// httpapi
type Uploader interface {
    Upload(ctx context.Context, upload []byte) (lifecycle.Created, error)
}
type Config struct {
    Version              string
    Uploads              Uploader
    Logger               *slog.Logger
    MaxConcurrentUploads int              // validated by cmd; NewHandler panics on < 1 when Uploads is set (a wiring bug)
    ParseTimeout         time.Duration    // (v1.11) the parser's effective time limit; write deadline = 120 s + 6 × it; NewHandler panics on <= 0 when Uploads is set (a wiring bug)
    Now                  func() time.Time // limiter clock; nil: time.Now
}
func NewHandler(cfg Config) http.Handler
```

(v1.3, accepted) If `Uploads` is nil, `NewHandler` registers no upload route. That is the health-only handler of the older tests. `serve` always sets `Uploads`; `TestUploadEndToEnd` proves the route exists through `newApp`.

**Test seams (v1.3, accepted).** They are nil or at their default in production, set only by `_test.go` files, and never read from flags or the environment:
- `ingest.Parser.stripHook`: called with the stripped bytes before the reparse.
- The unexported `settings.wrapUploader` in `cmd/workout-explorer`: wraps the service for `TestSettingsReachTheHandler`. `maxConcurrentUploads` never sets it.
- `uploadReadTimeout` is a package variable instead of a constant. A test that changes it restores it with `t.Cleanup` and does not call `t.Parallel`. The write deadline is computed from `uploadReadTimeout` and `Config.ParseTimeout` at step 3, so shortening `uploadReadTimeout` shortens both.
- (v1.11) The unexported `settings.onParser func(*ingest.Parser)` in `cmd/workout-explorer`: `newApp` calls it with the parser it built, for `TestSettingsReachTheParser`. Nil in production; `readSettings` never sets it.

`Makefile` `dev-server`: adds `-data '$(CURDIR)/build/data'` (git-ignored). The environment variable is not set by the Makefile; the default applies.

`parse -original <path>` (diagnostic, like `-datafile`): runs `Accept` instead of `Parse` and writes `Accepted.Original` to `<path>`; with `-datafile` the data file comes from `Accepted.Workout`. On a rejection it writes nothing and exits 3. Output from a real file stays local (ADR-004 item 9).

## Memory (v1.3, D-17)

ADR-017 rev. 3 is normative. The terms below are what QA and F-31 check:

| Term | Meaning | Rule or source |
| --- | --- | --- |
| A | Max RSS of `parse -original` on the all-caps twin (one `Accept` as a whole process) | ≤ 400 MB on each of 3 runs, `GOGC` and `GOMEMLIMIT` unset (stop rule) |
| B | Memory of one request after its `Accept`: upload, stripped copy, reparsed workout, data file and encoder | From the server run, `(M(4) − 2·A) / 2` at most; recorded |
| M(N) | Max RSS of `serve` with N uploads of the all-caps twin sent at once and N in-flight slots | Worst case ≤ 2·A + (N − 2)·B; M(4) and M(16) recorded in `MEASUREMENTS.md` |
| L | (v1.4) Heap of the upload limiter at `maxRateKeys` keys × `uploadRateLimit` attempts | ≤ 40,000,000 bytes (`TestLimiterMemory`); not part of the server run |

At the defaults (N = 4, 2 parse slots) the worst case is **2·A + 2·B + L, estimated at 0.85 to 1.05 GB plus at most 40 MB**. The estimate uses synthetic figures (ADR-017 rev. 3 item 4), and the recorded M(4) replaces the first part. The server run does not fill the rate table, so L is always added on top of M(4) (ADR-017 rev. 4). OQ-51 is decided: a VPS with at least 2 GB, N = 4, and F-31 sets `MemoryMax` from M(4) + L. If M(4) is above 1,000 MB, that is not an F-09 stop. It goes to F-31, and the gate in ADR-017 rev. 3 ("Would choose differently if") becomes the remedy. A request that waits for a parse slot holds only its upload (≤ 20 MB).

(v1.11, D-30, ADR-017 rev. 7 item 5) **The parse time limit T does not enter the model.**
- A, B, L and M(N) are bounded by the budgets and by N, not by time. A longer T keeps the same memory alive for longer, and only for files that run into the deadline. N still bounds how many requests exist at once.
- The server run and `MemoryMax` are measured and set as before, with T unset.
- What T changes is how long a slot is held: up to 2·T per `Accept` for a crafted file. One client address can take a share of T / 60 s of the two parse slots, which is why 60 s is the bound.

## Corpus

New synthetic file, generated by `corpusgen` (`identifyingData()` in `tools/corpusgen/f09.go`), committed with its golden report and golden data file like every synthetic file (ADR-004). Every value is invented; none comes from a real file.

| File | Built as | Golden expectation |
| --- | --- | --- |
| `identifying-data.fit` | One activity sequence: `file_id` (`serial_number` 3,735,928,559); right after it (v1.3, as real writers order them) `developer_data_id` and a `field_description` (`field_name` "marker_dev_field"); `device_info` for the creator (`device_index` 0, `serial_number` 2,882,400,001), a heart-rate strap (`device_index` 1, `serial_number` 2,882,400,002, `ant_device_number` 54,321, `ant_transmission_type` 1) and a power meter (`device_index` 2, `serial_number` 2,882,400,003, `ant_device_number` 12,345, `ant_transmission_type` 5); `user_profile` (`friendly_name` "Marker Person", `gender` female, `age` 41, `height` 1.71 m, `weight` 63.5 kg, `resting_heart_rate` 47, `default_max_heart_rate` 187, `global_id` the 6 bytes of "MkrGid"); `hrm_profile` (`hrm_ant_id` 54,321, `hrm_ant_id_trans_type` 1); a message 29 (`name` "Marker Home Place", a position, a timestamp before the first record); a `segment_leaderboard_entry` (`name` "Marker Rival Person", `activity_id` 424,242); a `sport` message (`sport` cycling, `name` "Marker Sport Name"); (v1.3, feature AC 6) one `zones_target` (maximum and threshold heart rate, threshold power) and one `time_in_zone` (zone boundaries, resting, maximum and threshold heart rate, threshold power), all values invented. (v1.4, QA-09-r1 Q1, feature AC 6) Also in the preamble: an `ant_channel_id` (`channel_number` 0, `device_type` 120, `device_number` 23,456, `transmission_type` 1, `device_index` 1); an `ant_rx` and an `ant_tx` (`timestamp` at the start, `mesg_id` 0x4E and 0x4F, `mesg_data` = channel 0 followed by the 8 ASCII bytes "MkrAntRx" and "MkrAntTx"); a `segment_id` (`name` "Marker Segment", `uuid` "marker-segment-uuid", `sport` cycling, `user_profile_primary_key` 2,882,400,005, `device_id` 2,882,400,004); a `segment_file` (`file_uuid` "marker-segment-uuid", `user_profile_primary_key` 2,882,400,010, `leader_type` [overall, personal_best], `leader_group_primary_key` [2,882,400,006, 2,882,400,007], `leader_activity_id` [2,882,400,008, 2,882,400,009], `leader_activity_id_string` ["MarkerLeaderOne", "MarkerLeaderTwo"]); a `tank_update` (`timestamp` at 60 s, `sensor` 2,882,400,011, `pressure` 200 bar) and a `tank_summary` (`timestamp` at the end, `sensor` 2,882,400,011, start 200 bar, end 50 bar, 10 L). Each binary marker must occur in the upload only at the fields that carry it. If a chosen 2-byte value also occurs elsewhere in the generated file, the engineer picks another invented value and names it in the hand-off. Then a 10-minute ride, with the developer field in every record at 1 Hz (positions, distance, speed, heart rate), timer start and stop, one lap, one session, `activity`. Normal record headers (no compressed timestamps). If the library's encoder cannot write message 29, which is not in its profile, `corpusgen` writes that definition and data message as bytes and recomputes the CRC | Accepted, `integrity` clean, 1 session, sport `cycling`. The report shows none of the markers (the parser reads none of those fields). Its `Accept` result is pinned by `TestStripIdentifyingData` |

Manifest row: "synthetic, no personal data; markers for the UP-9 strip".

(v1.5, D-20) A second new synthetic file, made from `ride-30min.fit` by byte operations in `corpusgen`, like the other broken files (ADR-004 item 8):

| File | Built as | Golden expectation |
| --- | --- | --- |
| `ride-30min-bad-basetype.fit` | `ride-30min.fit` with one 9-byte definition message inserted before the first `record` data message at or after 60 % of the record messages: header `0x40 \| L`, reserved 0, architecture 0, global number 0xFF00, 1 field (number 0, size 1, base type 0x30). L is a local type that no message after the insert point uses, so the old framing could still read everything behind it; the engineer names L in the hand-off. Every byte after the insert is unchanged. Data size, header CRC and file CRC are computed afresh, so the file is closed with correct CRCs | Accepted. `integrity.truncated` true, `crc_failed` false, `data_end` = the offset of the last record before the insert; as many samples as record messages before the insert (the golden is reviewed by hand for this). The stored original (`TestStripCorpus`, `TestUploadFraming`) is the upload's bytes up to the inserted definition, exclusive, in the unclosed form (data size 0, header CRC 0, no file CRC) |

Manifest row: "synthetic; F-08 R-a: the decoder rejects a definition (invalid base type) with records after it; F-09 cut (ADR-023 item 6a)".

(v1.6, D-21) A third new synthetic file, made from `ride-30min.fit` by byte operations in `corpusgen`:

| File | Built as | Golden expectation |
| --- | --- | --- |
| `ride-30min-size-short.fit` | `ride-30min.fit` with the header data size lowered by 5 and the header CRC computed afresh. Every other byte is unchanged, the file CRC included. `corpusgen` asserts that the new data size ends inside the last record, not on a record boundary. If 5 does not land inside it, pick the smallest k ≥ 5 that does and name it in the hand-off | Accepted. Report equal to `ride-30min.fit`'s except `integrity.ignored_bytes` = 5 (or k): `truncated` false, `crc_failed` false. The decoder reads the whole last record and then the unchanged file CRC. With a 14-byte header and a correct header CRC, that CRC does not depend on the header's data size. If `ride-30min.fit` has a 12-byte header, `crc_failed` is true instead; the hand-off says which. The stored original (`TestStripCorpus`, `TestUploadFraming`) is byte-identical to `ride-30min.fit` |
| Manifest row | "synthetic; header data size ends inside the last record; the decoder reads that record in full (ADR-003 rev. 6); F-09 stores the records read with the data size repaired (ADR-023 rev. 5 item 6)" | |

F-08's `ignored_bytes` counts from the end the header declares, so here it counts bytes the decoder did read: the tail of the last record and the CRC. The stored file keeps them as part of that record. That is F-08's figure and stays as it is.

## Logging

TD Conventions (Logging), I-5, I-7, I-8. Besides ingest's own line (ADR-018 rev. 2: one per `Accept`), the upload path writes only these lines and keys:

| `msg` | Level | Keys | Written by |
| --- | --- | --- | --- |
| `upload accepted` | INFO | `id`, `bytes` (the upload's size, as in ingest's line; v1.3), `truncated`, `blanked_fields`, `removed_messages`, `duration_ms` | lifecycle |
| `upload failed` | ERROR | `stage` (`encode`, `identity`, `files`, `store`, `cleanup`), `id` (once known), `error` | lifecycle |
| `upload failed` | WARN | `stage` (`deadline`), `error` | httpapi, step 3, when `SetReadDeadline` or `SetWriteDeadline` fails with anything but `http.ErrNotSupported` (v1.4, QA-09-r1-6). WARN, because the usual cause is a connection the client has already closed. The handler continues; a broken connection then fails at step 6. The standard library's error carries only the local address. |
| `upload refused` | INFO | `reason` (`busy`, `rate_limited`, `bad_request`, `too_large`, `storage_full` (F-10)), `status`; `rate_limited` only for a key's first refusal | httpapi |
| `rate table full` | WARN | `keys` | httpapi, at most once per sweep; a full table is swept at most once a minute (v1.3, ADR-021 rev. 2 item 8) |
| `recovery` | INFO | `staging_removed`, `orphans_removed`, `rows_removed` (v1.4), `unknown_entries`. The only line `Recover` writes; no WARN for unknown entries (ADR-022 rev. 2) | lifecycle |
| `storage` | INFO | `used_bytes`, `limit_bytes`, `uploads_open` (F-10 S-4; written at startup after recovery) | cmd |
| `server started` | INFO | + `max_concurrent_uploads`, (v1.11) `parse_timeout_s` (integer seconds, the parser's effective value) | cmd |
| `shutdown incomplete` | ERROR | `error` | cmd (F-01's line). v1.4: also written when the close function from `newApp` fails (the database close); `serve` then exits 1 |

Never: the client address or its prefix, `X-Forwarded-For`, user agent, file name, `public_id`, token, token hash, or any value the strip blanks or removes. Parser rejections (422) are not logged a second time.

## Tests

**`internal/ingest`** (v1.1)

| Test | Must show |
| --- | --- |
| `TestStripIdentifyingData` (AC 6) | `Accept` on `identifying-data.fit`. The stored bytes, decoded with the library's full decoder in the test, have: `serial_number` invalid in `file_id` and in all three `device_info`; `ant_device_number` invalid in both sensors; `hrm_ant_id` invalid; `ant_transmission_type` 1 and 5 and `hrm_ant_id_trans_type` 1 unchanged (OQ-46); no `user_profile`, no message 29 and no `segment_leaderboard_entry` data message, while their definition messages are still there; `sport.name` "Marker Sport Name", the developer data ID, field description and every developer value unchanged; the `zones_target` and `time_in_zone` data messages present and byte for byte unchanged (v1.3, OQ-50, feature AC 6); every other message equal field by field and in the same order. (v1.4) Also invalid: `ant_channel_id.device_number`, `segment_id.device_id`, `segment_file.leader_activity_id` (every element) and `leader_activity_id_string` (every element), both tank `sensor` fields. Unchanged: `ant_channel_id.transmission_type`, `device_type` and `device_index`, both `uuid` and `file_uuid` values and `segment_id.name`. Gone: the `ant_rx` and `ant_tx` data messages, with their definitions still present. (v1.5, OQ-52 decided "blank", as built in round 3) Also invalid: the three `*primary_key` fields, which are blanked markers. `StripCounts` = {16 blanked fields (1 + 3 serial numbers, 2 ANT device numbers, 1 `hrm_ant_id`, plus `device_number`, `device_id`, the two leader fields, the two `sensor` fields and the three account keys), 5 removed messages}. New markers: `device_number` 23,456 as 2-byte LE; `device_id`, both leader activity IDs and the tank sensor as 4-byte LE; "MkrAntRx", "MkrAntTx" and both leader strings as text. The kept identifiers (`uuid` text) are checked as present and unchanged. `Parse(stored)` is accepted and its report equals `Parse(upload)`'s byte for byte. Markers (v1.3): each name and the `global_id` as text; each serial number and the leaderboard `activity_id` as 4-byte little-endian; each ANT device number as 2-byte little-endian (the fields are 16-bit). The test first checks that every binary marker is in the upload, then that none is in the stored bytes. Decimal text forms are checked for absence only (they are not in a FIT file) |
| `TestAcceptReleasesFirstParse` (v1.3, D-17) | On `synthetic/ride-12h.fit`, with `DefaultLimits`, in a test that does not call `t.Parallel`. One `Accept` call to warm up. Then `runtime.GC()` and the baseline `HeapAlloc`, with the upload already in memory. Then `Accept` with a `stripHook` that calls `runtime.GC()` and reads `HeapAlloc`. (v1.4, QA-09-r1-3) `runtime.KeepAlive(upload)` follows the measured `Accept` call, so the upload stays live through the hook as it does on the server (`httpapi` holds `buf`). Without it the GC frees the upload while the stripped copy of the same size is live, and the test tolerates about 2.3 MB of retention instead of 1 MiB. Rule: the heap at the hook minus the baseline is at most `len(stripped) + 1 MiB`. The hand-off states the excess this test measured on the round-1 code, which must fail it (the first workout and recording of this file are several MB) |
| `TestParserLimits` (v1.11, ADR-017 rev. 7) | `NewParser(Limits{}, nil).Limits()` equals `DefaultLimits`. `NewParser(Limits{Timeout: 37 * time.Second}, nil).Limits()` equals `DefaultLimits` with `Timeout` 37 s. Changing a field of the returned value does not change the parser's next `Limits()` |
| `TestStripFrames` (AC 7) | Built from `identifying-data.fit` in the test: cut at 60 %, with the test asserting that the cut falls inside a record (else it moves the cut by one byte) → stored sequence ends at the last complete record, in the unclosed form (data size 0, header CRC 0, no file CRC), report equal including `truncated` and `data_end`; data size 0 upload → same; wrong file CRC and wrong header CRC → stored with correct CRCs, report equal except `crc_failed`; 64 trailing zero bytes → dropped; the file chained twice → both sequences stripped, data sizes and CRCs right; followed by a `settings` sequence holding a `user_profile` → that sequence kept with only its `file_id`, data size and CRC rewritten, still counted as skipped; a 12-byte header variant → no header CRC written. (v1.3) Two variants run on hand-built bytes, not through `Accept`. `TestStripHandBuilt`: a big-endian `device_info` definition is recognised and blanked; the fill itself is `TestBlankFill`, below. Reason: every field in the blank table has an invalid value that reads the same in either byte order (0 for the `z` types and strings; all 0xFF for `uint32`, v1.4), so a big-endian case through `Accept` could not fail. (v1.5, D-20) **Rejected developer field**, built in the test from `identifying-data.fit`. Before the first `record` at or after 60 % of the records, insert three things: a `field_description` data message for developer data index 0 with a new field number N and `fit_base_type_id` 0x30, using a copy of the file's `field_description` definition on a free local type; a `record` definition on another free local type that carries developer field N (size 1); and one data message of that definition. Then the rest of the file, closed with correct size and CRCs. The decoder accepts the description and the definition and rejects the data message. Expected: the stored sequence ends right before that data message, in the unclosed form, and the inserted description and definition are kept. The file passes `decodesToEnd`, and its report equals the upload's except `crc_failed`. **Count disagreement** (`TestStripHandBuilt`): a hand-built complete span of N records with `records` = N + 1, and a truncated span with `records` larger than the records that can be framed, each give `errRecordCount` and no output; with `records` = N, the output is unchanged. (v1.6, D-21) **Complete spans framed by count** (`TestStripHandBuilt`, hand-built spans; compare output bytes, not only the error): (i) `dataEnd` inside the last record, `records` = N: written in full, data size = N records, fresh CRCs; (ii) `records` = N − 1, `dataEnd` after the end of record N − 1, and record N fits in the bytes: `errRecordCount` (loop-end check b); (iii) record N starts before `dataEnd` and runs past the end of the bytes, `records` = N − 1: the N − 1 records written closed, no error; (iv) `records` = N but `dataEnd` at or before the start of record N: `errRecordCount` (check a); (v) the existing "unreadable complete sequence" case now expects `errRecordCount`, not the `record framing` prefix |
| `TestStripCorpus` (AC 6, AC 7) | For every accepted synthetic corpus file, `Accept` succeeds, and the stored bytes equal the upload byte for byte, except for listed files and differences: `chained-with-settings.fit` (its settings sequence loses the `user_profile` data message; that sequence's data size and CRCs); `identifying-data.fit` (`TestStripIdentifyingData`); `*-bad-crc`, `*-bad-header-crc`, `*-no-crc` (CRC bytes only); `*-trailing-zeros` (trailing bytes removed); every `*-cut-*` and `*-size-0` file (header data size and CRC fields, the dropped incomplete last record, no file CRC); (v1.5) `ride-30min-bad-basetype.fit` (header data size and CRC fields, the inserted definition and every byte after it, no file CRC: the stored file equals the upload's first bytes up to the inserted definition, exclusive, with the header in the unclosed form). Any other difference fails. (v1.3) `*-cut-*` files are compared record by record up to the last complete record. `chained-with-settings.fit` has its own check: the activity sequence is byte-identical, and the settings sequence equals the upload's without the `user_profile` data message. (v1.6) `ride-30min-size-short.fit`: the stored file equals `ride-30min.fit` byte for byte (header data size and CRC repaired; nothing dropped) |
| `TestStripTable` | (v1.4, replaces the rule) Enumerates every field of every message of the pinned profile (the library's factory) except those on the remove list. It applies ADR-023 item 4a (rev. 3; v1.5: rev. 4 tables): a field is identifier-like by name, by a subfield's name, or by profile type `ant_channel_id`. It fails if an identifier-like field is in neither `blankTable` nor `keptIdentifiers`, or in both. It fails if an entry of `blankTable` or `keptIdentifiers` is not in the profile or not identifier-like, so every rule line is needed by some entry. It fails if the blank table names a field ending in `trans_type` or `transmission_type` (OQ-46) or a message on the remove list. The blank table equals ADR-023 rev. 4 item 4 exactly (20 fields), with profile names. `keptIdentifiers` equals rev. 4 item 4a's table exactly (17 fields). (v1.5) Comments and failure messages cite "ADR-023 rev. 4 item 4" and "item 4a", without "(with OQ-52)" or "less the three rows". The remove list is exactly {3, 29, 80, 81, 149}, and 3, 80, 81 and 149 equal the library's `mesgnum` constants for `user_profile`, `ant_rx`, `ant_tx` and `segment_leaderboard_entry`. Mutants that must fail it: each of the six rule lines of item 4a removed, one at a time, with the table entries left in; `device_number` dropped from the blank table; an entry dropped from `keptIdentifiers`. (v1.7, D-23) Also: every `blankTable` entry's read type equals the factory's `BaseType` for that field and is not the unknown field; `readType` of a field number the profile lacks returns the declared base type. Mutant: one read-type entry changed to another base type |
| `TestBlankFill` (v1.7, D-23, ADR-023 rev. 6 item 4; replaces `TestFillInvalid`) | Deterministic, in CI, hand-built bytes through `strip` (not `Accept`), read back with the library's decoder (`decoder.New` with default options: the standard factory, checksums on). **Matrix:** every `blankTable` field (20) × every valid declared base type (the 17 base-type bytes of the library's `basetype` package, endian bit included) × declared sizes {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 16, 17} × (v1.8) **architecture byte ∈ {0x00, 0x01, 0x30, 0xFF}**, with the global number and the field bytes written in the order the decoder reads them (big-endian for every value but 0). That gives 20 × 17 × 13 × 4 = 17,680 cases. Each case is one complete sequence: a 14-byte header, a definition of the field's message with two fields, the blank field and a neighbour (a field number the profile lacks for that message, `byte`, size 2; asserted unknown through the factory), and one data message whose blank-field bytes are all 0x5A and whose neighbour is 0x5A 0x5A, with correct size and CRCs and `seqSpan` built by the test (complete, `records` = 2). Per case: (1) the output decodes without error, and the data message has the field; at declared size 0 the field is **absent** (the decoder drops it, `decoder.go:576-579`). (2) If the case is a **short field** (declared size s with 0 < s < n, n the read type's size, read type neither a z type nor `string`), every element of the value is 0; otherwise `!Value.Valid(field.BaseType)`. (3) No 0x5A byte is left in the blank field's bytes, and every other byte equals the input, except the file CRC. (4) For a fixed field, size and byte order, the blank field's output bytes are the same for all 17 declared types. (5) `strip` on the output returns the same bytes, with `BlankedFields` 0. (6) `BlankedFields` is 1 when the size is above 0, else 0. (7) `crossCheckFraming` on every input. **Pinned bytes** (literal expectations, little-endian unless noted): `file_id.serial_number` declared `uint8`, size 1 → `00` (the fuzz input's shape); declared `uint32`, size 4 → `00 00 00 00`; declared `uint32z`, size 5 → `00 00 00 00 00`; `device_info.ant_device_number` declared `uint32`, size 4, big-endian → `00 00 00 00`; `segment_id.device_id` declared `uint8`, size 1 → `00`, which reads as 0 (short field); `segment_id.device_id` declared `uint16`, size 4, big-endian → `FF FF FF FF`; `segment_file.leader_activity_id` size 10 → `FF FF FF FF FF FF FF FF 00 00`; `segment_file.leader_activity_id_string` declared `uint8`, size 5 → five `00`; `tank_update.sensor` declared `sint32`, size 4 → `00 00 00 00`; (v1.8) `device_info.serial_number` declared `uint32z`, size 4, architecture 0x30 → `00 00 00 00` and `BlankedFields` 1. Budget: at most 2 s without `-race`; the hand-off gives the case count. Mutants that must fail it: (m8) the round-5 fill (declared type); (m9) the trailing bytes after the last whole element left as uploaded; (m10) a z-type field shorter than its type filled with 0xFF; (m11) the read type used only when the declared size is not a multiple of the profile type's size; (m12) `big := arch == 1` (the round-6 rule), which must fail the architecture-byte cases and the 0x30 pinned case |
| `TestAcceptFailsClosed` | (v1.3) An undefined local type in a complete sequence cannot reach the strip: the library ends the decode there, so the walk marks the sequence truncated. The framing error is therefore tested on a hand-built complete span (`TestStripHandBuilt`: error, no output). At the `Accept` level, `stripHook` drives three cases: one altered record byte, a damaged workout and a non-FIT result. Between them they must cover both the reference mismatch and the rejected reparse. Each gives `internal` with a cause starting `strip:`, exactly one ERROR line without a field value, and a zero `Accepted` |
| `TestAcceptRejections` | Every rejected corpus file gives the same `*Error` (kind, message) from `Accept` as from `Parse`, and one log line |
| `TestAcceptLogs` | One line per `Accept` (none for a clean upload); a truncated upload logs `truncated: true` from the first parse; the reparse writes nothing; the position scan of `TestParseLogs` and a scan for the markers of `identifying-data.fit` find nothing |
| `FuzzAccept` | Seeds from `synthetic/` only. **The property (v1.9, D-29).** For any input:<br>(P1) no panic, and the call returns within 2 × (`Timeout` + 2 s);<br>(P2) `crossCheckFile(upload)`: for an input that `looksLikeFIT`, `crossCheckFraming` over the walk's spans (v1.8). A difference fails the target even when `Accept` succeeded and nothing leaked;<br>(P3) the result is exactly one of these:<br>• **accepted**: `checkStored` (below), `decodesToEnd`, `checkIdempotent` (`Accept(stored)` succeeds and its `Original` equals `stored` byte for byte, F-30 idempotence) and `crossCheckFile(stored)`;<br>• **the upload's own rejection** (`not_fit`, `unsupported_type`, `no_data`, `limit`), or `canceled`, or a reparse `limit`/`time`;<br>• **a deliberate refusal**: `KindUnreadable`, and the error wraps `errEmptySequence` or `errRemovedTimestamp` (`errors.Is`). An `unreadable` with any other cause fails the target.<br>Anything else fails the target: `KindInternal` whatever its cause, an error that wraps `errRecordCount`, or an error that is not an `*Error`.<br>Each failure has a **class**, which the hand-off names and with which each failure message starts (`privacy:`, `divergence:`, `integrity:`, `internal refusal:`, `robustness:`): **privacy** (a removed message or a valid blank field in the stored bytes); **divergence** (`crossCheckFraming` fails, or `errRecordCount`); **integrity** (a reference mismatch in `checkStored`, `decodesToEnd`, idempotence); **internal refusal** (`KindInternal`, an unlisted `unreadable` cause); **robustness** (a panic, the time bound).<br>**`checkStored`** is QA's oracle (QA-09-r2-1) and is not relaxed: `Parse(stored)` gives the same reference as `Parse(upload)`, and decoding the stored bytes with the library's decoder finds no valid value in a blank-table field and no data message whose number is in `removeTable` (read from the tables, not literal numbers). It has exactly three narrow rules beyond that. (v1.7, D-23) `storedListener` also implements `OnMesgDef` and is passed to the decoder with `WithMesgDefListener`. It records each (message, field) of `blankTable` that a definition declares with a size below its read type's size, where the read type is neither a z type nor `string` (a short field, ADR-023 rev. 6 item 4). In `OnMesg`, such a field counts as blank when it is absent, or when it was recorded and every element of its value is 0. (v1.8) The record is kept **per local type**: each definition replaces the record of its local type, and `OnMesg` looks up `proto.LocalMesgNum(m.Header)`, so 0 is accepted only for a data message whose own definition declares the field short. Reason: a short field has no invalid encoding, and `TestBlankFill` pins its bytes, so the exception cannot hide a strip defect in any other case. (v1.9) The decode loop is `checkStoredBytes(tb, stored)`, which decodes each stored sequence with `storedListener` and skips an unclosed sequence with no bytes after its header, as `decodesToEnd` does; `checkStored` = reference comparison + `checkStoredBytes`. Reason: that sequence is the stored form of a truncated span with `records` = 0 (ADR-023 item 6), and an empty sequence cannot hide a leak.<br>**Regression inputs**, all synthetic, committed under `internal/ingest/testdata/fuzz/FuzzAccept/` with their file names and bytes unchanged; `go test` runs them in CI: the three QA-09-r2-1 inputs (ADR-004 rev. 8); the two QA-09-r3-1 inputs, `crasher-d0ededf3c3e842f3` and `overrun-k5` (ADR-004 rev. 9); `crasher-c4b42a9acb8f4b92` (engineer round 5); `crasher-05bdce6d7833d6a3` (engineer round 6: architecture 0x30 on a `user_profile` definition); `empty-complete-seq` (v1.9: `sweepLead` followed by `framingProbe()` with byte 19 set to 0x2F, hand-built from test code).<br>**The fuzz run** (QA-09-r3-2), wherever it is written down (this design, ADR-004 item 9, the test comment): `go test ./internal/ingest -run '^$' -fuzz '^FuzzAccept$' -fuzztime 30m -fuzzminimizetime 5s`, on a scratch copy of `src/server` without `testdata/corpus/real/`. Without `-fuzzminimizetime`, Go spends up to 60 s minimizing each large input. Acceptance: section Fuzz findings, convergence and mutants |
| `crossCheckFraming(tb, data, spans)` (v1.8, D-25; test helper in `frame_test.go`) | For each span, decode the sequence as `decodeSequence` does: `data[pos:]`, with the header patched when its data size is 0. Use `WithIgnoreChecksum()`, `WithBroadcastOnly()`, `WithNoComponentExpansion()` and one recorder on both hooks (audit X2: the record stream is the walk's). The decoder's error is ignored. Then: (1) the recorder's count equals `sp.records`. (2) Frame the same span with `frame.go`'s iterator for `sp.records` records, as `stripSeq` does, and compare each record i: definition flag, header byte, local type = `proto.LocalMesgNum(header)`. For a definition: global = `MesgNum`; big = `Architecture != proto.LittleEndian`; the iterator's fields of size > 0 equal `FieldDefinitions` (number, size, base type) in order, and likewise the developer fields. For a data message: global = `m.Num`; length = 1 + the sizes of the decoder's active definition for that local type; `removeTable[global]` equals `removeTable[m.Num]`. Each `blankSpot` matches a decoder field of that number at that position, with `BaseType` = `readType`, offset = the sizes before it in the decoder's definition, and big as above. Every field of `blankTable[m.Num]` with size > 0 has a spot. A failure names the span offset, the record index and the attribute that differs, never a value. It does not use `Parse`, the listener or `checkStored` |
| `TestFramingSweep` (v1.8, D-25; deterministic, in CI) | **Framing probe**, built in the test: one complete sequence with a 14-byte header and correct CRCs, at most 400 bytes. It holds: `file_id` (architecture 0); `device_info` (architecture 1) with `serial_number` and `ant_device_number`; `segment_id` (architecture 0x30) with `device_id`; a `user_profile` definition with one normal and one compressed-timestamp data message; `developer_data_id`, `field_description`, and a definition with one developer field plus its data message; a definition with a size-0 field; one local type defined twice; a `record` with a `timestamp`. Every blank-field byte is 0x5A. **Sweep:** for every byte offset of the probe (file header and CRC included) and every value 0-255 except the original, change that one byte and nothing else (no CRC repair). Take `walk`'s spans; skip a mutant without spans. **Reduced value set** (used only when a part is over its budget, below): every value for record-header and definition bytes, and the values {0x00, 0x01, 0x02, 0x0F, 0x10, 0x20, 0x30, 0x40, 0x7F, 0x80, 0xFE, 0xFF, original ± 1} for the file-header, payload and CRC bytes. **The property (v1.9, D-29)** has two parts, both over every mutant of the same probe:<br>**Part 1, strip level** (the probe alone). The mutant has spans, so the checks run: `crossCheckFraming` passes. Then `strip` either (a) returns output that passes `checkStoredBytes`, `decodesToEnd` and `crossCheckFile(output)`, or (b) returns an error that wraps `errEmptySequence`, with nil output. Any other error fails, `errRecordCount` included.<br>**Part 2, `Accept` level.** The mutant is chained behind a fixed lead sequence, `sweepLead`, built in the test: 14-byte header, `file_id` (type activity), three `record`s with `timestamp` 1 s apart and `heart_rate`, closed with correct CRCs, at most 120 bytes. `Parse(sweepLead)` is asserted accepted once. For each mutant, `Parse(lead + mutant)`. If it rejects, `Accept` returns the same kind. If it accepts, `Accept` is either accepted, passing `checkStored`, `decodesToEnd`, `checkIdempotent` and `crossCheckFile(stored)`, or `KindUnreadable` wrapping `errEmptySequence` or `errRemovedTimestamp`. Anything else fails, `internal` above all. A `canceled` or `limit`/`time` result is re-run once and fails if it repeats. The parser uses `DefaultLimits` and a nil logger.<br>**`sweepTB`** overrides every failure method of `testing.TB`. `Error`, `Errorf` and `Fail` record. `Fatal`, `Fatalf` and `FailNow` record and panic with `sweepStop`. `Skip`, `Skipf` and `SkipNow` panic with a distinct value that counts as a failure ("a check skipped"). A panic of any other value is re-raised. Nothing a check calls reaches the real `*testing.T` except `Log`, `Logf`, `Helper`, `Name` and `Cleanup`.<br>**Report** (hand-off, not asserted): mutants per part, how many each part refused as U1 and as U2, failures, and the time per part. Budget: part 1 ≤ 10 s, part 2 ≤ 20 s without `-race`. If part 2 is slower, it uses the reduced value set above for file-header, payload and CRC bytes, and the hand-off says so.<br>**Mutants that must fail it** (applied in a scratch copy): (m12) `big := arch == 1`; (m13) a header with bits 7 and 6 set read as a definition; (m14) developer field sizes left out of the data length; (m15) normal local type `h & 0x07`; **(m16)** the U1 check removed from `strip`, which must fail part 1 at offset 19 and part 2; **(m17)** `errEmptySequence` mapped to `internal` in `accept.go`, which must fail part 2 |
| `decodesToEnd` (v1.5, test helper in `accept_test.go`) | For each sequence of a stored file, read with the library's decoder only. A closed sequence (data size > 0) must decode with checksums on and return nil. An unclosed one (data size 0) has its data size set to the bytes present (header CRC 0 for a 14-byte header) and 2 placeholder bytes appended, and must decode with `WithIgnoreChecksum()` and return nil. That proves that every byte up to the end is a record the decoder accepts. An unclosed sequence with no bytes after its header is accepted without decoding |
| `TestWalkRecords` (v1.5, D-20) | `seqSpan.records` equals the number of records the strip's framing reads in the same span for: `ride-30min.fit` (complete); `ride-30min-bad-header-crc.fit` (the retry's count, not twice the count); `ride-30min-cut-60.fit` (the records before the incomplete one); `chained-with-settings.fit` (each sequence separately); `ride-30min-bad-basetype.fit` (the records before the inserted definition). (v1.6) `ride-30min-size-short.fit`: complete, and `records` equals `ride-30min.fit`'s, so the record that crosses the header's data size is counted |
| `TestAcceptDataSizeSweep` (v1.6, D-21; QA's `qa3_overrun_test.go` made into a test) | Deterministic, in CI. Inputs, all from `synthetic/`: `ride-30min.fit`, `identifying-data.fit`, and `chained-ride-ride.fit` with the first sequence's data size changed. For each, the header data size of the changed sequence is moved by k for every k from −300 to +300 except 0. Each k is run in two variants: the header CRC left as it is (now wrong) and computed afresh. Rule per variant: if `Parse` rejects, `Accept` returns the same `Kind`. If `Parse` accepts, `Accept` accepts, and the stored bytes pass `checkStored`, `decodesToEnd` and idempotence (`Accept(stored).Original` equals `stored`). `Parse(stored)`'s report then equals `Parse(variant)`'s except `crc_failed` and `ignored_bytes`. Budget: at most 15 s without `-race` on the development machine. If it is slower, keep every k within the last two records and the CRC of the changed sequence plus every 10th k elsewhere, and state the number of variants in the hand-off. On the round-4 `strip.go` (overlay) it must fail; the hand-off gives the number of failing variants per file |
| `TestStripOverlap` (v1.6, D-21) | Built by hand in the test, through `Accept`. Sequence 1 is a 14-byte header, a `file_id` (`type` activity), and a definition of a manufacturer-range message (global 0xFF00) with one `byte` field of size S. Its one data message carries, at payload offset p, a complete second sequence: a `file_id` (`type` activity) and enough `record` messages with timestamps to be accepted. Sequence 1's header data size is chosen so that `dataEnd + 2` falls on that inner header; the overrun record and 2 CRC bytes follow. Assert that `Parse` accepts with `sequences` = 2. Then: `Accept` accepts; `stored` = sequence 1 closed (data size covering the whole overrun record, fresh CRCs) followed by sequence 2 closed, so `stored` is longer than the upload; `Parse(stored)`'s report equals `Parse(upload)`'s except `crc_failed` and `ignored_bytes`; `decodesToEnd`; idempotence. On the round-4 `strip.go` it fails (`internal`) |
| `TestAcceptReparseLimits` (v1.6, D-22) | (a) `ride-30min-no-crc.fit` with a parser of `Limits{MaxBytes: len(upload)}`: `Accept` accepts, and the stored file is `len(upload) + 2` bytes. (b) A parser whose `hook` sleeps past `Timeout` only during the reparse (switched on from `stripHook`): `Accept` returns `KindLimit` with `Limit` `time`, one WARN `fit rejected` line with `limit` `time`, and a zero `Accepted`. With the round-4 code, (a) gives `internal` (`reparse rejected (limit)`) and (b) gives `internal` |
| `TestAcceptRemovedTimestamp` (v1.6, OQ-54: pins the open case as it behaves today) | Built by hand: an activity sequence whose records use normal headers, then an `ant_tx` data message (removed by item 5) whose `timestamp` is at least 32 s after the last record's, then a `record` with a compressed-timestamp header. The decoder computes a compressed timestamp from the last timestamp it read (`decoder.go:641-644`, `742-745`), so without the `ant_tx` the record lands at least 32 s earlier. `Parse` accepts. **(v1.9, OQ-54 closed: refuse; D-28)** Expected: `KindUnreadable`, "This file could not be read.", `errors.Is(err, errRemovedTimestamp)`, one `fit rejected` line at WARN with `kind` `unreadable`, and a zero `Accepted`. Two more variants of the same file: (b) the `ant_tx`'s definition without field 253: `Accept` accepts, which shows the classifier does not refuse more than before; (c) the `record` after it with a normal header and a `timestamp`: `Accept` accepts |
| `TestRemovedTimestampDependency` (v1.9, D-28) | Pure, hand-built spans, no `Accept`. True for: a removed message (`ant_tx`) whose definition has field 253 (size 4), then a kept compressed-timestamp `record`, in one span. False for: the same with field 253 of size 0; a removed message without field 253; only normal headers after the removed message; the compressed record before the removed message; the removed message in span 0 and the compressed record in span 1; the compressed message itself removed and nothing kept after it. `crossCheckFraming` runs on each input |
| `TestAcceptEmptySequence` (v1.9, D-27; U1) | Built by hand: `sweepLead`, then a second 14-byte-header sequence whose header CRC is correct and whose data size D is at most the bytes left after the header. Its first record is a definition with field count 0xFF, so the definition runs past the end of the upload. Assert: `Parse` accepts, with `sequences` 2 and `crc_failed` true; `walk`'s second span is complete with `records` 0; `strip` returns `nil` and an error wrapping `errEmptySequence` whose text names the second sequence's offset; `Accept` returns `KindUnreadable`, "This file could not be read.", one WARN `fit rejected` line with `kind` `unreadable` and without `panic` or `frames`, and a zero `Accepted`; a `stripHook` set by the test is never called. On the round-7 code the `Accept` assertion fails with `internal` ("the stripped file is a different workout"), and the hand-off quotes it. `TestStripHandBuilt` (viii): a hand-built complete span with `records` = 0 gives `errEmptySequence` both through `strip` and through `stripSeq` called directly, and no output |
| `TestStripRejectedRecord` (v1.5, D-20; QA's probe `qa_tail_test.go` made into a test) | Built in the test from `identifying-data.fit`. After 10 % of its record messages, insert a 9-byte definition with base type 0x30 on local type 15. Then insert a copy of the creator's `device_info` definition and data message on local type 14, which carries the `serial_number` marker. Then the rest, closed with correct size and CRCs. `Accept` succeeds with `truncated` true. Assert: (a) the stored records are exactly the upload's records before the inserted definition, without the removed data messages. Compare by framing both files (record kind, local type, global number and length), not by a count copied from a run; (b) `decodesToEnd`; (c) the serial-number marker of the copied `device_info` is absent from the stored bytes; (d) `Accept(stored).Original` equals `stored`; (e) `checkStored`. On the round-3 code, (a) and (b) fail. The hand-off states how |
| `BenchmarkAcceptDefinitions` (v1.5, ADR-017 rev. 5) | Two in-memory uploads of 20,000,000 bytes, never committed, each one closed sequence: a `file_id`, one `record`, then definitions to the end. In one, every definition has 1 field; in the other, every definition has 255 fields. Each must take at most 4.5 s per `Accept` on the development machine. ns/op and B/op go into `MEASUREMENTS.md` |
| `BenchmarkAcceptAtCap` | ADR-017 rev. 2 item 4 and rev. 3: ≤ 4.5 s per call. Recorded in `MEASUREMENTS.md` with A, the RSS of `parse -original` on the all-caps twin: ≤ 400 MB on each of 3 runs, with `GOGC` and `GOMEMLIMIT` unset (stop rule). B/op is recorded, not ruled |
| Server run (v1.3, not a Go test; D-17) | `make build-server`, then `/usr/bin/time -v build/workout-explorer serve -data <fresh dir under build/> -addr 127.0.0.1:<port>` with the default N, and N uploads of `build/cap-500k.fit` sent at once (`curl -F file=@…`, one per upload, in parallel); every answer `201`; stop `serve` with SIGINT. 3 runs, each with a fresh data directory, for N = 4 (default) and N = 16 (`WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS=16`, 16 uploads). Record M(4) and M(16) in `MEASUREMENTS.md`. No stop rule; above 1,000 MB for M(4) goes to F-31 and OQ-51 (section Memory) |
| `TestCorpus` (extended) | For every accepted file, synthetic and local real: `Accept` succeeds and the stored bytes pass the decoder check of `FuzzAccept` (ADR-004 item 9). (v1.5) Also `decodesToEnd`, and `Accept(stored).Original` equals the stored bytes (ADR-004 rev. 8). No figure is asserted. (v1.9, ADR-004 rev. 10) Strict: a refusal of any kind fails, `unreadable` included, and the failure names the kind. `TestStripCorpus` likewise |

**`internal/lifecycle`** (fakes for every port)

| Test | Must show |
| --- | --- |
| `TestBase62` | All-zero bytes → 22 × `0`; value 1 → 21 × `0` + `1`; 62 → `…10`; all-`0xFF` → 22 characters; for 10,000 seeded random values the result equals `new(big.Int).SetBytes(b).Text(62)` left-padded with `0` to 22 (if the implementation is that expression, the test compares against a hand-written division loop instead) |
| `TestNewIdentity` | From a fixed reader: exact `id`, `public_id`, token; `id` matches `^[0-9a-f]{32}$`, the others `^[0-9A-Za-z]{22}$`; hash = `sha256.Sum256([]byte(token))` and differs from the token bytes; a short reader → error |
| `TestUploadPublicIDs` (AC 13) | 10,000 `Upload` calls with the real `crypto/rand`, a fake parser returning a small fixed `Accepted`, an in-memory repository and files: 10,000 distinct `public_id`s, all `^[0-9A-Za-z]{22}$`; also distinct `id`s and token hashes |
| `TestNewDraft` | Defaults above; `ExpiresAt − CreatedAt` = exactly 604,800 s; `CreatedAt` UTC and whole seconds; `DataEnd` only when truncated; `StartTime` nil for a zero time; time zone nil when unknown |
| `TestUploadStoresStripped` | The fake parser returns an `Original` different from the upload: `Create` receives `Original`, never the upload; the upload bytes appear in no log line |
| `TestUploadFailures` (AC 10, AC 11) | Parser error: returned with `errors.As` to `*ingest.Error` intact, `Create` and `InsertWorkout` never called. Encode error (a lap with `End < Start`): no `Create`, no insert. `Create` error: no insert. (v1.4) Insert error and `WorkoutExists` false: `Remove(id)` called with the created ID, `stage=store` line, `*ServerError`. Insert error and `WorkoutExists` true: `Remove` not called, `upload accepted` logged, `Created` returned with a nil error. Insert error and `WorkoutExists` error: `Remove` not called, a `stage=store` and a `stage=cleanup` line, `*ServerError`. Context cancelled before step 5: nothing called after `Accept` |
| `TestUploadInsertNotCancelled` (v1.4, D-18; deterministic) | The fake repository's `InsertWorkout` first calls the test's `cancel` for the request context, then records `ctx.Err()` of the context it was given, then stores the row. Its `WorkoutExists` records `ctx.Err()` the same way. Must show: both recorded errors are nil; `Upload` returns `Created` and a nil error; `Remove` is never called; the folder and the row both exist. On the round-1 code, `ctx.Err()` is `context.Canceled` (the hand-off says so) |
| `TestRecover` | Staging cleared; a folder without a row removed; a folder with a row kept; (v1.4) a row without a folder deleted through `DeleteWorkout`, a row with a folder kept; unknown entries counted, not removed; exactly one log line, `recovery` at INFO, with the four counts (no WARN) |
| `TestUploadLogKeys` | Every line from `Upload` uses only the keys in section Logging |

**`internal/store`** (temp directory)

| Test | Must show |
| --- | --- |
| `TestOpen` | Fresh file: `user_version` 1, `journal_mode` `wal`, file mode `0600`; opening again applies nothing |
| `TestSchema` (AC 5) | `PRAGMA table_info(workouts)` equals the 16 columns of section Schema in order, with type and not-null flags (`id` not-null, see section Schema); no other table except SQLite's own |
| `TestNewerSchemaRefused` | `user_version` 99 → `Open` fails with the "newer than this binary" error; afterwards `user_version` is still 99 and no table was created (v1.3: not byte equality, since the WAL pragma may touch the header, ADR-019 rev. 1 item 5) |
| `TestMigrationFiles` | Embedded names are `0001` … `N` with no gap |
| `TestInsertWorkout` | Round trip of every column through a raw `SELECT`, `NULL`s included; duplicate `public_id` fails; each `CHECK` rejects a violating row (bad visibility, draft with `published_at`, unlisted without it, 30 days + 1 s, short hash, invalid JSON) |
| `TestWorkoutIDs` | Returns exactly the inserted IDs |
| `TestWorkoutExists` (v1.4) | true for an inserted ID, false for an unknown one |
| `TestDeleteWorkout` (v1.4) | The row is gone and other rows are untouched; an absent ID returns nil |
**`internal/privatedir`**

| Test | Must show |
| --- | --- |
| `TestCreate` | Both files with their bytes; modes `0700` (folders) and `0600` (files); `tmp/` empty afterwards |
| `TestCreateFails` | A rename that cannot succeed (an existing non-empty `private/{id}`, since `rename(2)` replaces an empty folder; or `private/` made read-only, skipped as root) → error, no `tmp/{id}`, the existing folder unchanged |
| `TestNames` | IDs that are not 32 lowercase hex (`../x`, uppercase, 31 characters) and file names other than the two constants are refused before any file system call |
| `TestRemoveIDsStaging` | `Remove` of an absent ID is nil; `IDs` returns only valid IDs and counts the rest; `ClearStaging` empties `tmp/` |

**`internal/httpapi`** (fake `Uploader`, fake clock)

| Test | Must show |
| --- | --- |
| `TestUploadCreated` | `201`, exactly the two keys, `Content-Type` and `Cache-Control: no-store`; `view_url` = `/w/` + public ID; `edit_url` = `/edit/` + public ID + `#` + token; neither has a `?` |
| `TestUploadSizeCap` (AC 3) | Body read through a counting reader, handler called directly (`ServeHTTP` with `httptest.NewRequest`). (v1.3) (a) A file large enough that `Content-Length` exceeds `maxBodyBytes` (more than `MaxUploadBytes` + 16 KiB of body): `413`, "This file is larger than 20 MB.", **0 body bytes read**, uploader not called. (a2) A 20,000,001-byte file with `Content-Length` set: its `Content-Length` (about 20,000,200) is below `maxBodyBytes`, so the part is read. `413`, the same message, the part read through the limit reader stops at `MaxUploadBytes` + 1 file bytes, uploader not called. (b) The same and a 30,000,000-byte file with `ContentLength = -1`: `413`, bytes read ≤ `maxBodyBytes` + 64 KiB (far below the body size), uploader not called. (c) Exactly 20,000,000 bytes: the uploader receives all 20,000,000 |
| `TestUploadShape` (AC 9) | Wrong media type, no boundary, first part named otherwise, a second part, broken multipart, and a body that ends mid-file (`io.ErrUnexpectedEOF`): `400`, "The upload could not be read. Please try again.", uploader not called |
| `TestRateLimit` (AC 4) | Peer `127.0.0.1`, `X-Forwarded-For: 203.0.113.7`: 60 requests reach the uploader; the 61st gets `429`, "Too many uploads. Please try again later.", a `Retry-After`; a request from `198.51.100.9` still gets `201`. One hour after the first request, the first address is allowed again (rolling window). Refused attempts do not count. Attempts the uploader rejects (an `*ingest.Error`) and `413` refusals count. `busy` refusals do not. `2001:db8:1:2::a` and `2001:db8:1:2::b` share one limit; `2001:db8:1:3::a` has its own |
| `TestClientPrefix` | Non-loopback peer with `X-Forwarded-For` → the peer; loopback with `1.1.1.1, 2.2.2.2` → `2.2.2.2`; unparsable entry → the peer; two IPv6 addresses in one /64 → one key, in different /64s → different keys; `::ffff:192.0.2.1` and `192.0.2.1` → one key |
| `TestRateTableFull` | (v1.4, QA-09-r1-2: every clock step stays under 10 minutes since the last sweep, so the periodic sweep can never stand in for the full-table sweep) Limiter created at T with `maxKeys` 2, fake clock on whole seconds. Both keys allowed at T. (a) Window 1 h: a new key at T+30 s is refused, one WARN; at T+59 s refused, no further WARN; at T+61 s refused, **exactly one more** WARN (2 in total), which shows the full-table sweep ran. (b) A fresh limiter with window 30 s, filled the same way: a new key at T+59 s is refused (no sweep yet, under a minute since T); at T+61 s it is **allowed**, because the full-table sweep dropped the expired keys. The mutant "full-table sweep removed" must fail both (a) and (b) |
| `TestLimiterMemory` (v1.4, ADR-021 rev. 3) | `newLimiter(uploadRateLimit, uploadRateWindow, maxRateKeys, fake clock)`. `runtime.GC()`, baseline `HeapAlloc`. Then `maxRateKeys` distinct keys × `uploadRateLimit` allowed attempts each, through `allow`, with the clock advancing 1 s per round of keys. Then `runtime.GC()`. Rule: `HeapAlloc` − baseline ≤ 40,000,000 bytes. The hand-off states the measured value. No `t.Parallel`. The mutant `[]time.Time` instead of `[]uint32` must fail it. The copy-down rule (no forward reslicing) is checked by code review: in this fill nothing expires |
| `TestInFlight` (AC 8) | `MaxConcurrentUploads` 4: four uploads blocked in the fake uploader, the fifth gets `503`, "Too many uploads right now. Please try again in a minute.", `Retry-After: 30`, and does not count toward the rate limit; after one is released, the next gets `201`. The same with 2: the third is refused |
| `TestParseErrorMapping` (AC 11) | One `*ingest.Error` per kind → the status and code of the table, `message` = `Message()`. (v1.9) The kinds include `unreadable` → `422`, `unreadable`, "This file could not be read." |
| `TestServerErrorMapping` (AC 10) | Any non-`ingest` error from the uploader → `500`, "Something went wrong on the server. Please try again later." |
| `TestHealthDuringUpload` (F-08 hand-off) | On an `httptest.Server`: while an upload is held inside the fake uploader (standing in for a hostile parse; the parser's own isolation is F-08's `TestHostileDoesNotBlockOthers`), and while all N are held, `GET /api/health` answers `200` within 1 s |
| `TestCapsOrder` | `MaxUploadBytes ≤ ingest.DefaultLimits.MaxBytes` |
| `TestUploadDeadlines` (v1.11, ADR-021 rev. 4) | A test `ResponseWriter` that records the times passed to `SetReadDeadline` and `SetWriteDeadline` (the wrapper pattern of `TestUploadLogs`), with a fake uploader. `ParseTimeout` 10 s: read = start + 120 s, write = start + 180 s. `ParseTimeout` 60 s: read = start + 120 s, write = start + 480 s. "start" is taken around the request, with a tolerance of 1 s. `NewHandler` with `Uploads` set and `ParseTimeout` 0 panics |
| `TestNoViewPath` (AC 12, Go side) | `GET /w/<public id>` on the handler → `404`; Go never serves the view path (I-1) |
| `TestUploadLogs` | Keys only from section Logging; no line contains the address strings of the requests. (v1.4, QA-09-r1-6) Also reaches the `stage=deadline` line: a test `ResponseWriter` wrapping `httptest.ResponseRecorder` whose `SetReadDeadline` and `SetWriteDeadline` return a non-`ErrNotSupported` error gives two `upload failed` lines at WARN with only `stage` and `error`, and the upload still completes |

**`cmd/workout-explorer`** (end to end through `newApp` in `t.TempDir()`: real parser, store, files; corpus files from `testdata/corpus/synthetic/`)

| Test | Must show |
| --- | --- |
| `TestUploadEndToEnd` (AC 1, AC 2, AC 12) | `ride-30min.fit` → `201`. The row: `visibility` `draft`, `published_at` `NULL`, `expires_at − created_at` = 604,800, `hide_ends` 0, `allow_download` 0, `data_end_s` `NULL`, `edit_token_hash` = SHA-256 of the fragment and not equal to the token's bytes. `private/{id}/original.fit` byte-identical to the upload (this file has nothing to strip and correct CRCs); `draft.bin` equal to `datafile.Encode` of a fresh parse of the stored file; `summary_json` equal to the data file's `summary`. `public/` does not exist. The token string occurs nowhere: not in `view_url`, not in the captured log, not in any byte of any file under the data directory (database, WAL, shared memory, private files) |
| `TestUploadIdentifyingData` (AC 6) | `identifying-data.fit` → `201`. `original.fit` equals `Accept(upload).Original`. (v1.3) Every marker of `TestStripIdentifyingData` is absent from `original.fit`. Every marker of 4 bytes or more (names, `global_id`, 4-byte serial numbers and `activity_id`, every decimal text form) is absent from every file under the data directory (database, WAL, shared memory, `original.fit`, `draft.bin`), and every marker of 5 characters or more is absent from the captured log. The 2-byte ANT forms are **not** searched outside the FIT file. (v1.4) The markers include the new ones of `TestStripIdentifyingData`: `device_number` 23,456 as 2 bytes in `original.fit` only; the 4-byte IDs and the text markers ("MkrAntRx", "MkrAntTx", the leader strings) everywhere. The database holds random IDs and a random hash, so a 2-byte pattern there is chance: 12,345 in little-endian is the ASCII text "90", which a random 32-hex internal ID contains about one time in nine |
| `TestUploadFraming` (AC 7) | Files with nothing to strip, so every byte difference is framing. `ride-30min-trailing-zeros.fit` and `ride-30min-bad-crc.fit` → `201`, `original.fit` byte-identical to `ride-30min.fit`. `ride-30min-cut-60.fit` and `ride-30min-cut-60-size-0.fit` → `201`, `original.fit` is the size-0 file up to the end of its last complete record (shorter than the upload: the incomplete record is gone), parses as truncated, `data_end_s` = the upload's `Integrity.DataEnd`. (v1.5, D-20) `ride-30min-bad-basetype.fit` → `201`, `original.fit` is the upload up to the inserted definition, exclusive, in the unclosed form, parses as truncated, `data_end_s` = the upload's `Integrity.DataEnd`. (v1.6, D-21) `ride-30min-size-short.fit` → `201`, `original.fit` byte-identical to `ride-30min.fit`, `data_end_s` `NULL`. For all six, `Parse(original.fit)` gives the upload's report except `crc_failed` and `ignored_bytes` |
| `TestUploadRejectedLeavesNothing` (AC 11) | `not-fit-png.fit`, `course.fit`, `ride-30min-header-only.fit` → `422`, `message` exactly ingest's (`course.fit`: "Unsupported FIT file type."); afterwards 0 rows, `private/` and `tmp/` empty, exactly one parser log line each |
| `TestUploadUnreadable` (AC 9) | A malformed multipart body and a body cut off mid-file (a reader that fails after half the file) → `400`, UP-10's text; 0 rows, `private/` and `tmp/` empty |
| `TestUploadServerError` (AC 10) | `private/` made read-only (skipped as root) → `500`, UP-10's text; 0 rows, `tmp/` empty |
| `TestNoRequestMetadataStored` (AC 5) | Uploads with peer `127.0.0.1` and `X-Forwarded-For` `203.0.113.77`, and `2001:db8:1234:5678::abcd` (text and its 16 bytes), a marker user agent and a marker multipart file name: none of these strings or bytes occur in any file under the data directory or in the captured log. (IPv4 is checked as text only; 4 random bytes could occur in a FIT file by chance.) With `TestSchema` |
| `TestSettings` (AC 8) | `maxConcurrentUploads`: unset and empty → 4; `"2"` → 2; `"16"` → 16; `"0"`, `"17"`, `"-1"`, `"02"`, `" 2"`, `"2 "`, `"abc"`, `"1e3"` → error |
| `TestParseTimeoutSetting` (v1.11, D-30) | `parseTimeout`: unset and empty → `ingest.DefaultLimits.Timeout`; `"5"` → 5 s; `"10"` → 10 s; `"37"` → 37 s; `"60"` → 60 s. Errors, each with exactly the message of section Wiring: `"4"`, `"61"`, `"0"`, `"-5"`, `"+10"`, `"010"`, `" 10"`, `"10 "`, `"10s"`, `"1m"`, `"10.0"`, `"1e1"`, `"abc"`, `"100"`, `"99999999999999999999"`, `"٣"` |
| `TestDefaultParseTimeoutInRange` (v1.11) | `ingest.DefaultLimits.Timeout` is a whole number of seconds from 5 to 60, so the default is a value the setting itself accepts |
| `TestReadSettings` (v1.11) | One map with both variables (`"3"`, `"37"`) → `settings{maxConcurrentUploads: 3, parseTimeout: 37 s}`; empty map → 4 and 10 s; an invalid value in either → that variable's error |
| `TestServeInvalidParseTimeout` (v1.11) | `serve` with `WORKOUT_EXPLORER_PARSE_TIMEOUT=61` (`t.Setenv`) exits 2 with `serve: WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60` and creates no data directory |
| `TestSettingsReachTheParser` (v1.11) | `newApp` in `t.TempDir()` with `settings{maxConcurrentUploads: 4, parseTimeout: 37 s, onParser: capture}`: the captured parser's `Limits()` equals `ingest.DefaultLimits` with `Timeout` 37 s, and nothing else changed. With `parseTimeout` 0: equals `ingest.DefaultLimits` |
| `TestParseCommandTimeout` (v1.11) | `run(["parse", ride-30min.fit])` with `WORKOUT_EXPLORER_PARSE_TIMEOUT` `"4"` exits 2 with `parse: WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60` and writes nothing to stdout; with `"60"` it exits 0 and the report equals the golden |
| `TestServeInvalidSetting` (AC 8; v1.3 split) | `serve` with an invalid value exits 2 with the message and creates no database |
| `TestSettingsReachTheHandler` (AC 8; v1.3 split) | `newApp` with 2, the service wrapped by `settings.wrapUploader` in a blocking uploader: the third concurrent upload is refused with `503` |
| `TestServeData` | `serve` without `-data` or with `-data ''` → exit 2 |
| `TestServeRecovers` | A pre-existing orphan `private/<valid id>/` and a `tmp/x/` are gone after `newApp`; a folder with a row stays; (v1.4) a row whose folder was removed by the test is gone from the database |
| `TestUploadCancelDuringInsert` (v1.4, QA-09-r1-1; reproduces the race reliably with the real driver) | `store.Open` and `privatedir.Open` in `t.TempDir()`. A stub `lifecycle.Parser` returns one `Accepted`, computed once with the real parser from `ride-30min.fit`. A repository wrapper embeds `*store.DB` and overrides `InsertWorkout`: it starts a goroutine that sleeps a seeded random 0 to 100 µs and then calls this upload's `cancel`, and it calls the real `InsertWorkout`. 200 uploads, each with its own cancellable context. Must show: every `Upload` returns a nil error, and afterwards `WorkoutIDs()` and `IDs()` are the same set of 200 IDs. It must fail on the round-1 code in each of 3 runs (QA's probe lost the folder in about 18 % of uploads at this delay range). If it does not, raise the count to 1,000 before reporting. The hand-off states the failures seen on the round-1 code. Runtime under 10 s without `-race` |

## Acceptance criteria to verification

Numbers follow the feature file (14 ACs; revised 16 Oct 2026 with the framing criterion as AC 7).

| AC | Verified by |
| --- | --- |
| 1 Valid ride → both links; row `draft`, `expires_at` exactly 7 days after `created_at`, `edit_token_hash` not the token | `TestUploadEndToEnd`; `TestNewDraft`; `TestUploadCreated` |
| 2 Token only in the edit link's fragment; never in a query string, log line or the viewing link | `TestUploadEndToEnd` (log, files, `view_url`); `TestUploadCreated`; `TestUploadLogKeys`, `TestUploadLogs`; ADR-020 items 7 and 9 for later requests (F-12) |
| 3 20,000,000 bytes read in full; 20,000,001 refused with "This file is larger than 20 MB.", reading stops early | `TestUploadSizeCap` (a), (a2), (b), (c). "Stops reading early" is shown by (a) (no body byte read) and (b) (bounded far below the body) |
| 4 61st upload in an hour refused with "Too many uploads. Please try again later."; another IP still uploads; refused uploads count; IPv6 per /64 | `TestRateLimit`; `TestClientPrefix` |
| 5 No request metadata in the database or on disk | `TestNoRequestMetadataStored`; `TestSchema`; `TestUploadLogs` (logs go to journald, on disk) |
| 6 No identifying data stored: serial numbers, ANT device numbers and ANT IDs blank (v1.4: including `ant_channel_id.device_number`, `segment_id.device_id`, `segment_file.leader_activity_id`; v1.5: and the three segment account keys, OQ-52), transmission types unchanged; `user_profile`, `location`, leaderboard entry, `ant_rx` and `ant_tx` gone; sport name and developer field unchanged; the stored file passes F-08, parses to the same workout data and differs only in these points and its CRCs; nothing removed in the database, `draft.bin` or logs | `TestStripIdentifyingData`; `TestUploadIdentifyingData`; `TestAcceptLogs`; `TestStripCorpus` (only these differences); `TestStripTable` |
| 7 Framing only: trailing non-FIT bytes and an incomplete last record of a truncated file are dropped; (v1.5, QA-09-r2-1) a file whose reading stops early at a damaged record is stored ending at that record, nothing after it kept; CRCs correct, or for a truncated file marked as unfinished without CRCs (OQ-49, decided); same workout data; UP-3 notice kept | `TestUploadFraming` (incl. `ride-30min-bad-basetype.fit`; v1.6 `ride-30min-size-short.fit`); `TestStripFrames` (incl. the rejected developer field); `TestStripRejectedRecord`; `TestWalkRecords`; `TestStripCorpus`; (v1.6) `TestAcceptDataSizeSweep`, `TestStripOverlap`, `TestStripHandBuilt` count cases; `FuzzAccept` with its QA-09-r2-1 and QA-09-r3-1 seeds; (v1.8) `TestFramingSweep` and `crossCheckFraming` in every strip test; (v1.9) `TestAcceptEmptySequence`, `TestAcceptRemovedTimestamp`, `TestRemovedTimestampDependency` for the deliberate refusals, which are outside AC 7's "stored" cases and answer with the F-08-style sentence "This file could not be read." (D-26) |
| 8 Fifth upload refused with "Too many uploads right now. Please try again in a minute." at the default; the next is accepted once one finishes; another value takes effect after a restart | `TestInFlight`; `TestSettings`, `TestServeInvalidSetting`, `TestSettingsReachTheHandler` |
| 9 Malformed multipart, body cut off mid-file → "The upload could not be read. Please try again.", no row or files | `TestUploadShape`; `TestUploadUnreadable` |
| 10 Server-side failure → "Something went wrong on the server. Please try again later.", no row or files | `TestUploadServerError`; `TestServerErrorMapping`; `TestUploadFailures`; `TestCreateFails`; (v1.4, a client leaving during the insert) `TestUploadInsertNotCancelled`, `TestUploadCancelDuringInsert`, `TestRecover`, `TestServeRecovers` |
| 11 File rejected by F-08 → its message, no row or files | `TestUploadRejectedLeavesNothing`; `TestParseErrorMapping`; `TestAcceptRejections`; `TestUploadFailures` |
| 12 Fresh draft's viewing link shows no route, title or numbers | Nothing exists under `public/` (`TestUploadEndToEnd`), Go does not serve `/w/` (`TestNoViewPath`); the generic page itself is F-15's AC 1 and AC 6 |
| 13 10,000 `public_id`s unique, ≤ 22 base62 characters | `TestUploadPublicIDs`; `TestBase62` |
| 14 Crafted chained file with an empty complete sequence → "This file could not be read.", no row or files | `TestUploadEmptySequence` (end to end, QA-09-r4-4); `TestAcceptEmptySequence`; `TestStripEmptyHandBuilt`; `TestParseErrorMapping`; `FuzzAccept/empty-complete-seq` |
| (v1.11, no feature AC yet; requirements hand-off) Parse time limit is a server setting, default 10 s, 5 to 60 s, invalid stops the subcommand | `TestParseTimeoutSetting`, `TestDefaultParseTimeoutInRange`, `TestReadSettings`, `TestServeInvalidParseTimeout`, `TestSettingsReachTheParser`, `TestParseCommandTimeout`, `TestParserLimits`, `TestUploadDeadlines`, the extended `server started` test |

AC 5 is read as **request metadata** (confirmed by the user). Identifying data inside the file is AC 6's subject.

AC 7 for a truncated file (OQ-49, decided by the user on 16 Oct 2026; the feature file now says so): the stored sequence is written in FIT's unclosed form (data size 0, header CRC 0 meaning "not computed", no file CRC; ADR-023 item 6), so no CRC in it is wrong, but none is computed either. Closing it with a data size and CRCs would make the reparse see a complete file: the UP-3 notice and `data_end_s` would be lost, the verify step would refuse the upload, and F-30 could not rebuild the notice from `original.fit`.

## Hand-offs

| To | What |
| --- | --- |
| F-10 | Gate at step 0 of the check order, before the in-flight slot; refusals do not count toward the rate limit. Usage total computed after `Recover` at startup; a successful `Upload` adds `len(acc.Original) + len(draft)`. The storage cap is a server setting too: use the same mechanism as `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` (an environment variable read in `settings.go`, validated, exit 2 on an invalid value) and add it to the settings table in TD Conventions (Configuration) |
| F-11 | `POST /api/workouts` with `FormData` field `file`. Check `file.size > 20_000_000` in the browser. Show `message` as text for any non-201. A network error during a large upload may be the server closing after `413`. On success navigate to `edit_url` made absolute with `location.origin`, with no marker (v1.13, ADR-020 rev. 2) |
| F-12 | Look up by `public_id` (`^[0-9A-Za-z]{22}$`, else not found), token only from `Authorization: Bearer`, compare `HashToken(token)` with `subtle.ConstantTimeCompare`. `data_end_s` is the UP-3 value. `original.fit` is the stripped file (UP-9): serve it as stored. (v1.5) Every stored sequence reads to its end with the pinned decoder; a truncated one is in the unclosed form and ends at the last record the parser read (ADR-023 rev. 4 item 6a). `draft.bin` has no `.br`; compress on the fly if wanted. Delete: row (and reports) first, through `Repository.DeleteWorkout`, then folders (ADR-022 item 6). (v1.4) After startup every row has its folder (ADR-022 rev. 2); a file missing at run time is a `500`. Any write that must survive a client leaving runs with `context.WithoutCancel` once its first irreversible step has begun (D-18). `MaxLifetime` from `lifecycle` |
| F-15 | Folder `public/{public_id}`; only `publish` creates `public/`; extend `Recover` to remove `public/{public_id}` folders without a row. (v1.4) Run that sweep after the rows-without-folder step, so a public folder whose row recovery just deleted goes in the same start |
| F-19, F-20 | The edit page is served for `/edit/{public_id}` and `/published/{public_id}` (F-20 C-1); it reads the token from `location.hash` and sends it only as `Authorization: Bearer` |
| F-21, F-27 | Delete through the F-12 routine (row first, then folders). (v1.4) F-27's report limiter states its own memory bound (its cap × its per-key bound) and adds it to M(N) (ADR-017 rev. 4) |
| F-22, F-31 | Caddy: `reverse_proxy` to Go for `/api/*` (default `X-Forwarded-For` handling; do not configure `trusted_proxies` so clients cannot pass their own); never `log_credentials`; `/edit/*` → the edit page with `no-store`; nothing under `<data>` except `public/` readable by Caddy. Systemd unit: `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` via `Environment=` when not 4. (v1.3, v1.4) Machine and memory (OQ-51, decided: VPS ≥ 2 GB RAM, N = 4): `MemoryMax` ≥ the measured M(4) + L (40 MB, the rate table at its cap, which the server run does not fill), plus 2·A for a `regenerate` process if it can run during a deploy (ADR-017 rev. 4). The runbook records M(4) and the chosen cap (F-31 AC). If used, `GOMEMLIMIT` goes in the unit, below `MemoryMax`; the binary never sets it (TD Conventions, Configuration). Rollback note for forward-only migrations (ADR-019). (v1.11, ADR-017 rev. 7) **Parse time limit:**
- The operator documentation lists `WORKOUT_EXPLORER_PARSE_TIMEOUT`: whole seconds, default 10, range 5 to 60, and an invalid value stops the service at start with exit 2 and the message of section Wiring.
- The runbook gives ADR-017 rev. 7 item 6's procedure for choosing the value: `parse` on the synthetic all-caps twin, 4 × the median wall time, unset when that is at most 10 s, and stop at more than 60 s. It records the measured time and the chosen value.
- Both variables go in one `EnvironmentFile=` that the unit and the deploy script's `regenerate` call both read, so `serve` and `regenerate` never run with different limits.
- Every Caddy timeout on `/api/*` stays at or above the upload write deadline, 120 s + 6·T (ADR-021 rev. 4).
- Memory sizing does not change. |
| F-29 | (v1.9, OQ-53 follow-up 2, decided by the user on 21 Oct 2026) The About page carries this sentence, exactly: **"Workouts are not archived: after an app update, a damaged file may show less than before or be removed."** It belongs with "how long it is kept". The feature file and INF-1 still have to name it (requirements engineer). The list of what is removed is ADR-023 rev. 4 item 4's table and item 5, in plain words: device and sensor serial numbers and ANT+ IDs wherever they appear, the device ID of a segment, the activity IDs of a segment's leaders and (v1.5, OQ-52) the account keys of a segment's creator and of its leaders' groups (blanked); the whole user profile, including heart-rate settings; saved locations; segment leaderboard entries; raw ANT radio messages (removed) (v1.4). Kept and worth saying (OQ-50, decided): heart-rate and power zone messages (`zones_target`, `time_in_zone`), user-chosen names and developer fields stay as uploaded |
| Phase 2 (AP-10) | The stored original has no `user_profile`; zones come from the file's zone messages or the sharer, never from profile heart-rate settings |
| F-30 | Run `Accept` (not `Parse`) on each stored original; if `Accepted.Original` differs from the stored bytes (a widened strip table), replace `original.fit` the way `privatedir.Create` writes files (temp and rename). Rewrite `draft.bin` the same way; update `sport`, `start_time`, `time_zone_offset_s`, `summary_json`, `data_end_s`. Do not run `Recover` from `regenerate`; `serve` clears `tmp/` only at its own start. (v1.5, ADR-023 rev. 4 item 6a)
- With the pinned library, `Accept` on a stored original returns the same bytes. A later library that accepts more cannot make a workout grow.
- **(v1.6, OQ-53 decided by the user) The app does not store workouts long term.** If a later library reads a stored original differently, the regenerated result is accepted as it is, shorter included: write `original.fit` (now cut further), `draft.bin` and the row from that `Accept`, the same way as any other regenerated workout. No run is stopped and no exit code is raised to protect stored workouts. A workout whose stored original `Accept` refuses is deleted through the one delete routine (F-12), as on expiry (F-30 feature, Scope), but only when the file itself is rejected (the follow-up 1 bullet below).
- F-30 may still report how many workouts changed their truncation state or got shorter (compare the regenerated `Integrity.Truncated` and `DataEnd` with the row's `data_end_s`). That is information for the deploy report only. F-30's design picks the keys and the level.
- **(v1.9, OQ-53 follow-up 1, decided by the user on 21 Oct 2026) Delete only when the file itself is rejected.** The rule works on `Accept`'s error for the stored original, read with `errors.As`:
  - **Delete** (through the one delete routine, F-12, as on expiry; the deletion is logged with the device model) on an `*ingest.Error` with kind `not_fit`, `unsupported_type`, `no_data`, `unreadable` (D-26), `internal`, or `limit` with a limit other than `time`. Each of these depends only on the file's bytes and the code's version. `internal` is a recovered panic or a strip failure on this file, which is the user's "internal from the file".
  - **Keep unchanged** (row and files), to be retried on the next run, on an `*ingest.Error` with kind `canceled` or `limit`/`time` (load, or an interrupted run), and on any error that is not an `*ingest.Error` (disk, database, data-file encoding, page or image writing: server side).
  - F-30 counts kept workouts in its deploy report, next to deleted ones.
  - **(v1.10, OQ-55 decided by the user on 22 Oct 2026, as proposed) Kept workouts and old bundles.** A kept workout serves its previous files (data file, page, preview image, download file, under their old content-hashed names) until a later run rebuilds it. **An old app bundle is deleted only once no kept workout refers to it.** Until then more than one format version and bundle can be live, but each bundle still reads exactly one format version, so no reader for an old version is ever written (ADR-005 rev. 1 item 9). The rule is computed from the kept workouts of the run, not from a bundle age. The deploy report counts kept workouts. Where the deletion runs (inside `regenerate` or in the deploy step right after it, as the TD's Deploys row puts it) is F-30's and F-31's choice; the rule is the same. TD Versioning has been reworded by requirements.
  - **For F-30's design to settle (not a user question):** a kept workout shown through the edit page rather than a page of its own (a draft) is read by the **current** bundle. After a format version change its old `draft.bin` would fail the current strict reader until the next run. F-30's design states what such a workout shows meanwhile (for example F-04's load-error text) or how it avoids the case.
- (v1.6) A stored original can be longer than `MaxBytes`, though only for a crafted upload (ADR-023 rev. 5 item 7). F-30 reads stored originals with `MaxBytes` raised by that bound, so that `Accept` cannot refuse one of our own files for its size. `ingest` gets that entry point when F-30 is built.
- (v1.11, ADR-017 rev. 7 items 2 and 5) **The same parse time limit as `serve`.**
  - `regenerate` reads `WORKOUT_EXPLORER_PARSE_TIMEOUT` with `parseTimeout` from `settings.go`, and exits 2 with `regenerate: …` on an invalid value before it opens the database.
  - It builds its parser from that value together with the raised `MaxBytes` of the bullet above, through the same helper in `cmd` (extend `newParser` when F-30 adds that allowance).
  - It adds no per-workout deadline of its own.
  - A `limit`/`time` keeps the workout (follow-up 1), so a lower T between runs can never delete a workout. |
| F-32 | The launch sweep checks a real file of each brand with `parse -original` and a FIT viewer: the UP-9 fields blank, the UP-9 messages gone, nothing else changed but the framing (results by manufacturer in the QA report only) |

## Fuzz findings, convergence and mutants

Standing rules for every change to the strip, the framing or their tests. They were set in engineer rounds 6 to 8; the round lists are in history.

- **Fuzz acceptance.** After a strip change is green in CI: one run of the `FuzzAccept` command of section Tests (`-fuzztime 30m -fuzzminimizetime 5s`, scratch copy of `src/server` without `testdata/corpus/real/`), green for the full 30 minutes with no failure of any class. The hand-off gives execs and "new interesting" at about 10, 20 and 30 minutes, the corpus size, and how many status lines show 0 execs/s. No new input is written into `src/`. QA runs its own 30-minute run with a fresh `-test.fuzzcachedir` in its next round, so the two runs explore independently.
- **Why 30 minutes, and what counts as evidence.** At about 350 execs/s a 5-minute run is about 100,000 executions, and three rounds in a row found a new input inside that budget. For the framing class the evidence is the deterministic `TestFramingSweep` in CI, together with `crossCheckFraming` on every corpus file, seed and fuzz input. The fuzz run is the backstop.
- **Classes** (`FuzzAccept` row): privacy, divergence, integrity, internal refusal, robustness. A deliberate refusal (U1, U2) is not a finding.
- **Privacy and divergence: stop.** The engineer reports to the architect, and nothing is point-fixed. The next round first adds a deterministic check that fails on the unfixed code. For divergence that is an audit row plus an extension of `crossCheckFraming` or `TestFramingSweep`. For privacy it is a hand-built case or a committed seed, plus an extension of `TestStripTable`, `TestBlankFill` or the sweep. Only then is the code fixed. **Escalation by class:** if a new input of the same class (privacy or divergence) turns up after two fix rounds, it goes to the user, and ADR-023's "Would choose differently if" (decoder-driven rewrite) is put to them.
- **Integrity, internal refusal, robustness: normal route.** The engineer stops the run, attaches the input (synthetic) and its class. The architect decides per finding:
  - fix the code, so the input is accepted with a faithful stored file; or
  - if the input is crafted and has no faithful stored form, add a cause to ADR-023 item 8a, with a precise detector and a deterministic test.

  These classes do not escalate by class. CLAUDE.md's rule still applies to every finding: the **same** finding surviving two fix rounds goes to the user.
- **Mutants and overlays.** Mutants and older-code overlays are applied as temporary edits in a scratch copy of `src/server`, never in `src/`. The hand-off gives one line per mutant: the file, what changed, and the tests that failed. It confirms that `git status` shows nothing under `src/` from mutants.
- **Budgets.** A time budget in section Tests is a hand-off figure unless the row says "asserted": wall-clock assertions in unit tests are flaky on shared CI.
- **Real files.** No real-file figure goes into `architect/`, `MEASUREMENTS.md` or the corpus README; hand-offs give counts only.

## Accepted engineer choices

Choices the engineer made where this design left room, reviewed and accepted in rounds 5 to 8. They are current.

| Where | Choice |
| --- | --- |
| `TestAcceptDataSizeSweep` | Uses the fallback k set: every k from minus the last two records' length to +4, plus every 10th k elsewhere, which covers the last two records and the file CRC. The sub-tests share one `Parser` whose slot count is raised to `runtime.NumCPU()`, a test seam in the same package; the parse path is unchanged |
| `TestStripHandBuilt` (ii), (iv) | (ii) puts `dataEnd` strictly after the end of record N − 1, inside record N. (iv) checks both a `dataEnd` at and one before the start of record N |
| `TestAcceptReparseLimits` (a) | Also asserts that `MaxBytes = len(upload) − 1` refuses the upload: the first parse still checks `MaxBytes` on the upload |
| `corpusgen` | Prints nothing on success. Facts about a corpus file (for example k, the last record's length and the header size of `ride-30min-size-short.fit`) go in its README manifest row |
| `TestStripTable` | The read-type mutant changes every entry in turn |
| `TestBlankFill` | The neighbour field is the highest field number below 251 that the factory does not know, `byte`, size 2, asserted unknown |
| `stripDef` | Keeps size-0 fields as read. Offsets stay right, blank spots skip them (audit D6, D-23) and the cross-check drops them as the decoder does |
| `rec.install(&defs)` | A separate step, so `nextRecord` has no side effects and L2 is one visible call. Every walk over `records` records calls it after each framed definition (`stripSeq`, the cross-check, the U2 classifier). `runsPast` only peeks and never installs |
| `frameEnd` status | Not a decoder decision, so no audit row. Inside the first `records` records it is a count disagreement like any status other than `framed`. In `runsPast` it is false, and for a complete span it cannot be reached there: the region runs to the end of the upload, which lies at or after the declared end. Its comment says so |
| `crossCheckFraming` | Also compares the architecture byte and byte order on data messages (the definition in force), so it catches an `install` defect (L2). It requires a blank spot for every blank-table field with size above 0 |

## Open questions

None for F-09. Decided questions are in the decision log of `features/README.md` and in `INDEX.md`; the F-09 decision lists are in history.
