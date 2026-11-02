# ADR-018 Parse outcomes: rejection kinds, user messages and the parser log line

- **Status:** accepted (message wording other than UP-2's own string accepted as recommended by the user, 15 Oct 2026, OQ-30)
- **Revision:** 6 (22 Oct 2026). First version 15 Oct 2026. Labels "rev. N" below are the stable IDs code cites; each marks a decision made in that revision, stated here in its current form
- **History:** [architect/history/ADR-018-parse-outcomes-messages-and-log.md](history/ADR-018-parse-outcomes-messages-and-log.md)
- **Features:** F-08 (introduces it), F-09 (maps kinds to HTTP responses), F-11 (shows the message), F-30 (logs regeneration failures through it), F-32
- **Supersedes:** none. Extends TD invariant I-5 with a test-enforced allow-list for `ingest`
- **Requirements:** UP-2 ("Unsupported FIT file type."; plain error messages), UP-3, NFR Operations ("Parser errors are logged with the device model (no location data)"), TD Conventions (Logging), TD invariant I-5; `features/README.md` question 20 (error texts chosen in F-08/F-09)

## Context

Three consumers need the parse outcome. F-09 turns it into an HTTP response, and F-11 shows the sharer a plain sentence. The operator reads the log to support new devices (NFR Operations). F-30 logs regeneration failures "with the device model, never coordinates" (TD Versioning). If every consumer interprets library errors itself, the messages drift and a library error string may end up in front of a user. The log is the one place where workout data could leak by accident (I-5).

## Decision

1. **Every failure of `(*ingest.Parser).Parse` is an `*ingest.Error`** with one `Kind`. Its `Message()` is the user-facing sentence, fixed per kind. No other error leaves `Parse`.

   | Kind | When | `Message()` |
   | --- | --- | --- |
   | `not_fit` | Input shorter than a FIT header, first byte not 12 or 14, or bytes 8–11 not `.FIT` (ingest's own header check, before the decoder runs) | "This file is not a FIT file." (accepted, OQ-30) |
   | `unsupported_type` | The first `file_id` of the first FIT sequence has a valid `type` other than `activity` (course, workout, settings, monitoring, …). A `file_id` in a later sequence never gives this kind, even when the first sequence has none (rev. 1) | "Unsupported FIT file type." (**exact, UP-2**) |
   | `no_data` | A FIT header, but no `record` with a valid timestamp could be read, or the samples come to zero | "This file contains no readable workout data." (accepted, OQ-30) |
   | `limit` | A budget or the time limit of ADR-017 was exceeded; `Error.Limit` names it (`bytes`, `time`, `records`, `samples`, `laps`, `sessions`, `timer_events`, `field_descriptions`, `sequences`, `sport_messages` (rev. 1)) | "This file is too large or too complex to read." (accepted, OQ-30) |
   | `internal` | A recovered panic (ADR-017 item 6) | "This file could not be read." (accepted, OQ-30) |
   | `canceled` | The caller's `ctx` ended (client gone, or no parse slot in time) | "The upload was interrupted." (rarely seen) |
   | `unreadable` | (rev. 5) **`Accept` only, never `Parse`.** The upload's own parse accepted it, but the strip has no faithful stored form for it. The causes form a closed list, ADR-023 item 8a: (U1) an empty complete sequence; (U2) a removed timestamp that a later compressed-timestamp record depends on (OQ-54). See section `unreadable` (rev. 5) | "This file could not be read." (the user's wording, 21 Oct 2026; the same sentence as `internal`) |

   Precedence when more than one applies: `canceled`, `limit` (`bytes` first), `internal`, `not_fit`, `unsupported_type`, `no_data`. A course file with records is therefore `unsupported_type`, not accepted. A monitoring file, which has no records, gets the same answer.

2. **Recovery is not an error.** A truncated file, a CRC failure, trailing bytes or a skipped non-activity sequence still returns a workout. Its `Integrity` (F-08 design) says what happened, and the UP-3 notice is built from it by F-19.

3. **`ingest` writes the parser log line, through the `*slog.Logger` given to `NewParser`.** Exactly one line per `Parse` that ends in a rejection or a recovery. None for a clean success, and none from the CLI (`nil` logger). Callers may add their own attributes with `logger.With` (for example a random request or workout ID), never IP addresses, tokens or file names.

   | Outcome | `msg` | Level |
   | --- | --- | --- |
   | Rejected: `not_fit`, `unsupported_type`, `no_data`, `limit` | `fit rejected` | WARN |
   | `internal` | `fit rejected` | ERROR |
   | `canceled` | `fit canceled` | INFO |
   | Accepted with any of: truncated, CRC failed, ignored bytes, skipped sequences | `fit recovered` | INFO |

4. **Allow-list of keys.** `ingest` writes only these keys (besides slog's `time`, `level`, `msg`), always in this order and always present unless marked "if":

   | Key | Value |
   | --- | --- |
   | `outcome` | `rejected`, `recovered`, `canceled` |
   | `kind` | The `Kind` (if rejected or canceled) |
   | `limit` | The limit name (if `kind` is `limit`) |
   | `file_type` | `file_id.type` name, `""` if unknown |
   | `manufacturer` | `file_id.manufacturer` name, `""` if unknown |
   | `product` | Product name from the FIT profile when the manufacturer is `garmin`, `dynastream`, `dynastream_oem` or `tacx` (`typedef.GarminProduct`, `unknown_<n>` if not in the profile), else `""` |
   | `product_id` | `file_id.product`, 0 if absent |
   | `product_name` | `file_id.product_name`, at most 64 bytes, cut at a rune boundary |
   | `bytes` | Input length |
   | `sequences`, `skipped_sequences`, `ignored_bytes` | From `Integrity` |
   | `truncated`, `crc_failed` | From `Integrity` |
   | `data_end_s` | Seconds from the workout start to the last intact record (if truncated and accepted) |
   | `records` | `record` messages read |
   | `duration_ms` | Wall time of the parse, slot wait excluded |
   | `cause` | The error chain text (if rejected): our wrapping plus the decoder's message. v0.28.4 decoder errors hold byte positions, CRC values, message and field numbers and base types only, never field values. Our own wrapping never formats a field value |
   | `panic`, `frames` | The panic value as text, and up to 32 stack frames as `function file:line` (if `internal`). No argument words: Go's default traceback prints raw argument words, which can be the bits of a coordinate |

   Never logged: positions in any form, start time, time zone, timestamps from the file, totals, stream values, serial number, file name.

5. **Enforced by a test, not only by review.** `TestParseLogs` (F-08 design, AC 9) runs every rejection and recovery path, checks the key allow-list and the "one line per outcome" rule, and scans every line for the positions of the input files (semicircle integers, and degrees written with 4 to 7 decimals) and for the substrings `lat`, `lon` and `position`.

6. **One encoder for the rejection report.** `ingest.RejectionJSON(*Error)` is the only producer of the golden rejection report used by the corpus (ADR-004 rev. 6). It contains `kind`, `message`, `limit` (if any) and `file_type` (if known).

### `Accept` (rev. 2, rev. 3, rev. 4)

1. **One line per `Accept`, as per `Parse`.** It describes the upload, not the stripped file: `outcome`, `integrity` keys, `records` and `bytes` come from the first parse. The reparse writes no line. A clean upload with nothing recovered writes none (the upload line of F-09 does).
2. **Strip failures are `internal`**, except the deliberate refusals of ADR-023 item 8a, which are `unreadable` (rev. 5). A record-count failure (rev. 3, rev. 4: every framing disagreement, its cause names the sequence's byte offset and what disagreed: the count, or where the sequence's records end), a rejected reparse or a reference mismatch gives `Kind: internal`, message "This file could not be read.", level ERROR, and a `cause` starting `strip:` that holds byte offsets, message numbers and the reparse's kind, never a field value. `panic` and `frames` are empty unless a panic occurred. **Exception (rev. 4):** a reparse rejected for the time limit gives the upload's `Kind: limit`, `Limit: time`, message "This file is too large or too complex to read.", level WARN (item 3's table), with the first parse's keys as for any `Accept` line.
3. **Allow-list unchanged.** The counts of blanked fields and removed messages are logged by F-09's `upload accepted` line, not by `ingest`.

### `unreadable` (rev. 5, rev. 6)

**Context.** On 21 Oct 2026 the user replaced the rule "upload accepts whatever `Parse` accepts". On malformed or crafted input, upload may refuse a file that `Parse` accepts, with the standard unreadable-file sentence "This file could not be read.". It must not use `internal`, which means a fault on the server's side. No file is promised acceptance, from a real device or not (rev. 6, the user's decision of 22 Oct 2026; ADR-023 item 8a). None of the six other kinds fits:

| Candidate | Why not |
| --- | --- |
| `not_fit` | Its fixed sentence is "This file is not a FIT file.". That is false here: the file is FIT, and its first sequence parses. Giving `not_fit` a second sentence would break item 1's "one fixed message per kind" |
| `internal` | Excluded by the user. It is logged at ERROR, and F-09's fuzz and sweep treat it as a defect. A deliberate refusal under it could hide real defects |
| `no_data`, `limit` | Their sentences describe other causes |

**Decision.**
1. New kind `unreadable`: the row in item 1's table.
2. **Precedence.** `unreadable` comes only after the first parse has accepted the upload, so it never competes with `not_fit`, `unsupported_type`, `no_data` or a first-parse `limit`. Inside `Accept`: `canceled` first, then a reparse `limit`/`time`, then `unreadable`, then `internal`.
3. **Log.** `fit rejected` at **WARN** (item 3's table row for rejections), with `kind` `unreadable` and a `cause` starting `strip:` that names the cause and a byte offset, never a field value. The keys are the first parse's keys, as for any `Accept` line (rev. 2 item 1).
4. **`internal` is narrowed.** It means a recovered panic, or a strip failure that item 8a does not explain: a record-count failure, a rejected reparse, or a reference mismatch outside U2. F-09's tests treat every `internal` as a finding.
5. `RejectionJSON` is unchanged. No corpus golden can hold `unreadable`, because the corpus runs `Parse`, never `Accept`. F-09 maps the kind to `422` through its generic rule for ingest kinds, with no code change in `httpapi`.

**Consequences.** F-11 shows the sentence as it shows any other. F-30 treats `unreadable` as "the file is rejected" and deletes the workout (F-09 v1.9, F-30 hand-off). A new cause of `unreadable` needs a new row in ADR-023 item 8a. It is never added to the code alone.

## Consequences

- F-09 needs no knowledge of FIT or of library errors: `errors.As(err, &ingErr)`, then status and `ingErr.Message()`. F-09 picks the status codes.
- The user-facing wording lives in one table. Changing it changes the corpus rejection reports, so the change is visible in review (ADR-004).
- The log tells the operator the device (make, Garmin model or product name) and where the decoder stopped, which is what NFR Operations asks for. It does not include the file, so a rejected upload cannot be reproduced from the log alone; nothing of a rejected upload is stored (NFR Privacy, F-09).
- A library upgrade whose error texts start to include field values would break the scan test on a corpus file with positions. The upgrade hand-off checks this (ADR-003 item 1).

## Would choose differently if

- The operator cannot diagnose new devices from these lines: add fields from `file_id` or `device_info` (never positions or times) in a revision, never the file itself.
- The front end needs to react per kind (for example, a help link for `unsupported_type`): F-09 returns `kind` in the API response as well; the kinds are already stable names.
