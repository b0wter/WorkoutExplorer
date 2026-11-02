# ADR-018 Parse outcomes: history

History of [architect/ADR-018-parse-outcomes-messages-and-log.md](../ADR-018-parse-outcomes-messages-and-log.md). The live ADR states every decision once, in its current form, and keeps the labels rev. 1 to rev. 6 that code cites. This file holds, verbatim, what was moved out of it on 22 Oct 2026 (restructuring, no decision changed). Read it only when you need the history.

## Header revision list (as of revision 6)

- **Date:** 15 Oct 2026 (revision 1, 15 Oct 2026, F-08 QA round 1: limit name `sport_messages` added (ADR-017 rev. 1); `unsupported_type` confirmed as "first `file_id` of the first sequence", which the F-08 v1.0 listener did not do (QA-08-r1-4, F-08 D-15))
- **Revision 2:** 16 Oct 2026, F-09: `Accept` writes the one log line for the whole upload; strip failures are `internal` with a `strip:` cause (ADR-023). See Revision 2
- **Revision 3:** 21 Oct 2026, F-09 v1.5: one more strip failure, a record count that differs from the decoder's (ADR-023 rev. 4 item 8). Item 2 of Revision 2 is amended in place. No new kind, message or log key
- **Revision 4:** 21 Oct 2026, F-09 v1.6 (QA-09-r3-1): every framing disagreement is the record-count failure (ADR-023 rev. 5 item 8); a reparse that hits the time limit is reported as the upload's `limit` with `limit` `time`, at WARN, not as `internal`. Item 2 of Revision 2 is amended in place. No new kind, message or log key
- **Revision 5:** 21 Oct 2026, F-09 v1.9 (the user's acceptance rule of 21 Oct 2026): new kind **`unreadable`**, produced only by `Accept`, for an upload that `Parse` accepts but that has no faithful stored form (ADR-023 rev. 8 item 8a). Message "This file could not be read.", level WARN. `internal` keeps its meaning: a defect on our side (a recovered panic or an unexplained strip failure). See Revision 5. No new log key
- **Revision 6:** 22 Oct 2026, F-09 v1.10 (the user's decision of 22 Oct 2026): text only. Revision 5's context no longer says that files from real devices must be accepted; that requirement is withdrawn (ADR-023 rev. 9 item 8a). No kind, message, precedence or log change

## Revision 2 item 2 as amended in place by revisions 3, 4 and 5

2. **Strip failures are `internal`** (rev. 5: except the deliberate refusals of ADR-023 rev. 8 item 8a, which are `unreadable`; Revision 5). A complete sequence the rewriter cannot frame, (rev. 3) a framed record count that differs from the decoder's (the cause names the sequence's byte offset and both counts), a rejected reparse or a reference mismatch gives `Kind: internal`, message "This file could not be read.", level ERROR, and a `cause` starting `strip:` that holds byte offsets, message numbers and the reparse's kind, never a field value. `panic` and `frames` are empty unless a panic occurred. (rev. 4) "Cannot frame" and "count differs" are now one failure, the record-count failure. Its cause names the sequence's offset and what disagreed: the count, or where the sequence's records end. **Exception:** a reparse rejected for the time limit gives the upload's `Kind: limit`, `Limit: time`, message "This file is too large or too complex to read.", level WARN (item 3's table), with the first parse's keys as for any `Accept` line.

## Revision 5 context as written (with the rev. 6 strike)

**Context.** On 21 Oct 2026 the user replaced the rule "upload accepts whatever `Parse` accepts". On malformed or crafted input, upload may refuse a file that `Parse` accepts, with the standard unreadable-file sentence "This file could not be read.". It must not use `internal`, which means a fault on the server's side. ~~Files from real devices must still be accepted.~~ (rev. 6: withdrawn by the user on 22 Oct 2026; no file is promised acceptance, ADR-023 rev. 9 item 8a.) None of the six kinds fits:

## Revision 5 item 1 as written (a second copy of the `unreadable` row of item 1)

1. New kind `unreadable`, added to item 1's table:

   | Kind | When | `Message()` |
   | --- | --- | --- |
   | `unreadable` | **`Accept` only.** The upload's own parse accepted it, but the strip has no faithful stored form for it. The causes form a closed list, ADR-023 rev. 8 item 8a: (U1) an empty complete sequence; (U2) a removed timestamp that a later compressed-timestamp record depends on (OQ-54) | "This file could not be read." (the user's wording, 21 Oct 2026; the same sentence as `internal`) |

Item 1's row as it stood: | `unreadable` | (rev. 5) `Accept` only, never `Parse`: the upload parses, but it has no faithful stored form (ADR-023 rev. 8 item 8a). See Revision 5 | "This file could not be read." |
