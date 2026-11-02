# QA F-09 Upload API and workout storage - round 3

- **Date:** 2026-10-21
- **Verdict:** **ready with conditions.** There is no Blocker or Major finding. Both round-2 findings are fixed, with the evidence round 2 asked for:
  - the crashers and the kept-records probe now decode to their end;
  - `FuzzAccept`, with `checkStored` unchanged, ran green for 5 min 31 s in my own run;
  - ADR-023 rev. 4 and `TestStripTable` match exactly.

  All 13 acceptance criteria are traced to code and passing tests, including AC 7 with the new cut file. Conditions:
  1. **QA-09-r3-1 must be decided by the architect, then fixed.** This defect has been in the code since round 1. A complete FIT sequence whose header data size ends inside a record is accepted by `Parse` (CRC failure plus ignored bytes), but `Accept` refuses it as `internal` ("This file could not be read."). `FuzzAccept` fails on such an input. It found one within 67 s once its minimization time was shortened (QA-09-r3-2). With the default settings, both the engineer's 5-minute run and mine spent about 80 % of the time minimizing, so they did not find it. A 5-minute run with `-fuzzminimizetime 5s` must then be green.
  2. **The user's FIT-viewer check** (r1 condition 2; F-32 launch sweep) is still open. Only the user can do it.
- **Inputs:**
  - Feature: `features/F-09-upload-api-and-storage.md`: UP-9 and AC 7 revised, Decision "QA-09-r2-1 (option a, cut)".
  - Spec: Product Specification UP-9 (revised: "When reading stops early at a damaged record …"), UP-3, SH-8, INF-1.
  - Architecture:
    - `architect/F-09-upload-api-and-storage.md` v1.5 (D-20, sections Strip, Corpus, Tests, the round-4 list and its stop conditions, the F-30 hand-off);
    - ADR-023 rev. 4 (items 2, 4, 4a, 6, 6a, 8; Consequences);
    - ADR-003 rev. 5, ADR-017 rev. 5, ADR-018 rev. 3, ADR-004 rev. 8;
    - `architect/F-08-fit-validation-and-recovery.md` v1.7;
    - `architect/overview.md` v1.20 (I-8);
    - `architect/INDEX.md`.
  - Previous rounds: `qa/F-09-upload-api-and-storage-r1.md`, `-r2.md`.
  - State: base `8ab9085` plus the uncommitted working tree. `git status` and the SHA-256 of every modified and untracked file were the same at the start and at the end of the review.
    - Against my round-2 end snapshot, round 4 changed code only in `internal/ingest` (`sequence.go`, `strip.go`, `strip_table.go` comments, `accept.go`, tests), in `cmd/workout-explorer/upload_test.go` (`TestUploadFraming`) and in `tools/corpusgen` (`f08.go`, `f09.go`).
    - It added the new corpus file with its goldens and the three fuzz seeds.
    - `lifecycle`, `store`, `privatedir` and `httpapi` are byte-identical to round 2, so the round-2 checks there still hold.
  - QA changed nothing outside `qa/`:
    - Probes and mutants ran in a copy of `src/server` without `corpus/real/`, under `<scratchpad>/acf38496-ea04-454f-9beb-6c089f0f46a3/scratchpad/qa3/copy/`, or through `go test -overlay`.
    - The real-file aggregate ran as an overlay test file (`…/scratchpad/qa3/realprobe/`).
  - Not in scope: OQ-53 and the F-30 and F-31 changes for it (the user's decision, with follow-ups pending).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0 (21.1 s). gofmt clean, `go vet` clean, `svelte-check` 0 errors and 0 warnings, Go and web tests `ok`, build passed. Bundle 45,582 / 60,000 B gzip, PASS. The Go results were cached, so I also ran the line below |
| `go test -count=1 ./...` (in `src/server`) | exit 0, all 12 packages `ok` (24.6 s). This includes `TestCorpus` on the 6 local real files, now with `decodesToEnd` and idempotence |
| `gofmt -l .`; `go mod tidy -diff` | no output; exit 0 |
| `go test -count=1 -race` on `ingest`, `lifecycle`, `store`, `privatedir`, `httpapi`, `cmd/workout-explorer`, `datafile`, `workout`, `tools/corpusgen` | exit 0, all 9 `ok` (`ingest` 156 s) |
| `make corpus` | `corpus: 51 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` (45 synthetic, 6 local real). `ride-30min-bad-basetype.fit` ok, bin 31,576 B. `identifying-data.fit` bin 18,352 B, `swim-bike-distance-fall.fit` bin 24,580 B, both unchanged since round 2 |
| `go test ./internal/ingest -run '^FuzzAccept$'` (seeds only, as in CI) | ok (48 seeds: 45 synthetic files and 3 committed inputs) |
| **`FuzzAccept`, own run**, scratch copy, fresh `-test.fuzzcachedir`, default settings, `-fuzztime 330s` | **PASS**. 5 min 31 s, 113,821 execs, 53 new interesting inputs (101 in the corpus), no failing input written. 93 of 111 status lines show 0 execs/s (QA-09-r3-2) |
| `FuzzAccept`, the same target, `-fuzztime 300s -fuzzminimizetime 5s` | **FAIL after 67 s**: `internal error: ingest: internal: strip: record framing: record past the end of its data at byte 11037` (QA-09-r3-1). The input is kept at `…/scratchpad/qa3/crasher-d0ededf3c3e842f3` |
| `go test ./internal/ingest -run '^$' -bench 'AcceptAtCap\|AcceptDefinitions' -benchtime 3x -count 2 -benchmem` | `AcceptAtCap`: 2.324 s and 2.399 s per op, 885.8 MB/op. `AcceptDefinitions/fields=1`: 0.707 s and 0.722 s, 176.5 MB/op. `fields=255`: 0.487 s and 0.451 s, 273.2 MB/op. All within 4.5 s |
| A: `/usr/bin/time -v build/workout-explorer parse -original <scratch> <scratch>/cap-500k.fit`, the twin from `corpusgen -cap-file` (SHA-256 identical to `build/cap-500k.fit` and to round 2's), `GOGC` and `GOMEMLIMIT` unset | Max RSS 252,892 / 244,044 / 270,988 kB, wall clock 3.16 s each, exit 0. Within 400 MB. The output equals the input (nothing to strip) |
| Local real files (overlay probe; aggregates only, ADR-004 item 7) | 6 of 6 accepted, 50 fields blanked, 1 message removed (as in r1 and r2), 0 truncated. All 6 pass `decodesToEnd`, and `Accept` on each stored original returns the same bytes |

### Mutation probes

All of these ran in the scratch copy with `go test -overlay`. For `ingest` I ran the whole package and listed every failing test; for `cmd` I ran `TestUploadFraming`, `TestUploadIdentifyingData` and `TestUploadEndToEnd`.

| Mutant | Killed by |
| --- | --- |
| **Real round-3 restore**: the round-3 `strip.go` from my round-2 scratch copy, with a stub that only declares `errRecordCount` | `FuzzAccept` (the 3 committed inputs and the `ride-30min-bad-basetype.fit` seed), `TestStripRejectedRecord` ("(a) stored 648 records, want the 97 before the rejected definition"; (b) `decodesToEnd`: "invalid basetype"), `TestStripFrames/rejected_developer_field` ("stored 649 records, want the 400"; `decodesToEnd`), `TestStripHandBuilt/count_disagreement`, `TestStripCorpus`, `TestCorpus` and `TestUploadFraming` on the new file |
| m1 (the engineer's overlay: truncated spans ignore `records`) | exactly the same set as the real restore, so the overlay is faithful |
| m2 (`records − 1`) | `TestStripCorpus`, `TestCorpus` and `TestUploadFraming` on every `*-cut-*` file and on the new file; `TestAcceptLogs`; 6 seeds of `FuzzAccept`; `TestStripFrames`; `TestStripHandBuilt` |
| m3 (count check for complete spans removed) | **only** `TestStripHandBuilt/count_disagreement_fails_closed` (judged in Previous findings) |
| m4 (own): truncated spans write `records + 1` records, with no final count check | the same set as m2, plus `TestStripRejectedRecord` and the developer-field variant |
| `TestStripTable` source mutants (re-run after the comment-only change): `ant_channel_id.device_number` dropped from `blankTable`; `camera_file_uuid` dropped from `keptIdentifiers`; OQ-52 key 148/4 dropped; rule line "contains primary_key" removed; rule line "profile type ant_channel_id" removed; 81 dropped from `removeTable` | All 6 fail `TestStripTable`. The three table mutants of blanked or removed data also fail `TestStripIdentifyingData` |

## Findings

### Minor

- **ID:** QA-09-r3-1
- **Severity / kind:** Minor / defect. It has been in the code since round 1 and is not a regression; round 2 missed it too.
- **Route:** architect first, to decide where a complete sequence ends when the decoder has read past its header data size. Then backend: implement the decision, and commit the input as a `FuzzAccept` seed under ADR-004 rev. 8.
- **Where:**
  - `src/server/internal/ingest/sequence.go:108` and `117-118`. The sequence is `seqComplete` with `dataEnd = pos + headerSize + DataSize`, but the decoder's `records` count includes a last record that runs past `DataSize`.
  - `src/server/internal/ingest/strip.go:94-99`. The framing stops at `dataEnd` and returns `errStripFrame`.
  - `architect/ADR-023-…md` item 2 ("so it cannot disagree with the parser about boundaries") and item 8.
  - `src/server/internal/ingest/accept_test.go:370-372`. `FuzzAccept` treats any `internal` result as a failure.
- **Relates to:**
  - UP-9 ("plus repaired framing"); OQ-48; F-08 R-b and R-e (UP-3 recoveries);
  - AC 7 ("a valid file with trailing non-FIT bytes … stored without those bytes"), because F-08 reports exactly this file as complete with `ignored_bytes`;
  - AC 11 by analogy: a file F-08 accepts must not be refused;
  - the design's `FuzzAccept` row ("an error that wraps `errRecordCount` fails the target", and any `internal` result fails it too);
  - ADR-023 rev. 4 items 2 and 8.
- **Evidence:**
  - **Root cause.**
    - `decodeMessagesWithContext`, which `DecodeWithContext` uses (`muktihari/fit` v0.28.4, `decoder.go:1009-1016`), loops `for d.cur < DataSize` and reads every record in full. So a record that starts before `DataSize` and ends after it is accepted, and the hook counts it.
    - The decoder then reads the next 2 bytes as the file CRC. If they match (a 14-byte header's CRC covers only the data), or do not match, or are missing, the walk calls the sequence `complete` either way. The bytes after `pos + headerSize + DataSize + 2` become `ignored_bytes`.
    - The strip's region ends at `DataSize`, so the last record "runs past the end of its data". The result is `errStripFrame` and then `Accept` returns `internal`.
    - The decoder and the strip agree on every record length. This is the one place where they disagree on where the sequence ends.
  - **Probe `…/scratchpad/qa3/copy/server/internal/ingest/qa3_overrun_test.go`.**
    - Input: `ride-30min.fit` and `identifying-data.fit` with the header data size changed by k = −300 … +300 (k ≠ 0), each with and without a recomputed header CRC.
    - `Parse` accepts every variant.
    - `Accept` returns `internal` for 586 of 1,200 variants of `ride-30min.fit` and for 590 of 1,200 of `identifying-data.fit`. Those are the decreases that do not land on a record boundary. Increases are handled correctly, as truncated files.
    - Example, data size 5 bytes short: `Parse` gives `truncated false, crc_failed false, ignored_bytes 5`. `Accept` gives `strip: record framing: record past the end of its data at byte 55147`.
    - The same probe on `ride-30min-bad-basetype.fit` gives 0 failures, because the cut happens before the end.
  - **`FuzzAccept` fails on it.**
    - With that 5-byte variant added as a seed in the scratch copy: `--- FAIL: FuzzAccept/qa3-overrun-k5 … internal error`.
    - The fuzzer found the same class on its own in 67 s with `-fuzzminimizetime 5s`. That input (15,031 B) is a complete sequence that `Parse` accepts, with 1,717 samples, `crc_failed` and 3,989 ignored bytes. The walk reports 1,830 records and `Accept` returns `internal`.
  - **Provenance.** Both inputs are derived only from synthetic files. In a scan of 16-byte windows, none matches a real file without also matching a synthetic one.
  - **Privacy.** UP-9 is not affected. The upload fails closed, and nothing is stored or logged beyond ADR-018's ERROR line, which holds offsets only.
- **Consequence:**
  - A file whose header data size ends inside a record is refused with "This file could not be read." and an ERROR log line. F-08 accepts the same file as recoverable.
  - How often real uploads look like this is unknown. A writer whose data size is checkpointed at a record boundary is not affected. A miscounted or damaged size field is.
  - `FuzzAccept`, the oracle the design names for AC 7, fails within about a minute when it fuzzes effectively. Until this is settled, it cannot find new defects in that time (the same situation as round 2's QA-09-r2-1).
- **Evidence that would close it:**
  - ADR-023 items 2 and 8 state the rule. One option is to frame a complete sequence by `records`, as a truncated one is, and rewrite the data size to cover them. That fits I-8's "holds only records the parser read". The other is to refuse such files deliberately, with the reason written down and `FuzzAccept` expecting it.
  - A test with the 5-byte-short variant.
  - The 67 s input committed as a seed.
  - A 5-minute `FuzzAccept` run with `-fuzzminimizetime 5s`, green.

- **ID:** QA-09-r3-2
- **Severity / kind:** Minor / risk (test evidence)
- **Route:** architect (the design's round-4 row 5 and ADR-004 fix the run command), then backend (the "Manual run" comment at `accept_test.go:345`)
- **Where:**
  - `architect/F-09-upload-api-and-storage.md`, round-4 row 5 (`-fuzz '^FuzzAccept$' -fuzztime 5m`);
  - `src/server/internal/ingest/accept_test.go:345`.
- **Relates to:** design `FuzzAccept`; the round-2 evidence request ("green for 5 minutes"); the engineer's claim that the run was green.
- **Evidence:**
  - **What happens in the default run.**
    - The 45 synthetic seeds are 112 B to about 330 KB.
    - In the run with `GODEBUG=fuzzdebug=1` (`…/scratchpad/qa3/fuzzdbg.log`), 11 inputs were queued for minimization within the first second. They were 18 to 332 KB.
    - Each minimization ran into Go's default limit of 60 s (queued at 11:46:42, done at 11:47:42), and all 8 workers were busy with it.
    - Throughput collapses during those minutes. In my default 5.5-minute run, 93 of 111 status lines show 0 execs/s, and the stalls end at about every full minute (1m3s, 2m3s, 3m3s, 4m3s, 5m3s). The engineer's log (`…/scratchpad/fuzz.log`) has the same pattern, with 89,601 execs in total.
  - **So the 5-minute runs were real but weak.**
    - They were not hung. The final count is about 90,000 to 114,000 execs.
    - But most of the wall clock went to shrinking large inputs, not to exploring.
    - With `-fuzzminimizetime 5s` the stall lines dropped to 3 of 20, and the run found QA-09-r3-1 in 67 s.
  - The round-2 runs (mine included) were affected in the same way.
- **Consequence:** "5 minutes green" overstates the assurance, and a 5-minute gate can miss a failure class that an effective run finds in one minute.
- **Evidence that would close it:** the fuzz command in the design, ADR-004 item 9 and the test comment names a short minimization limit (for example `-fuzzminimizetime 5s`), and the acceptance run uses it.

- **ID:** QA-09-r3-3
- **Severity / kind:** Minor / documentation
- **Route:** backend
- **Where:** `src/server/testdata/corpus/README.md:72`, the manifest row for `synthetic/identifying-data.fit`.
- **Relates to:** ADR-004 (manifest); design section Corpus (v1.4 and v1.5 contents); AC 6.
- **Evidence:**
  - The row lists only the v1.2 contents (serial numbers, ANT device numbers, `user_profile`, `hrm_profile`, message 29, the leaderboard entry, the kept messages).
  - It does not list the v1.4 and v1.5 markers the file now carries and AC 6 depends on: `ant_channel_id`, `ant_rx`, `ant_tx`, `segment_id` and `segment_file` with their device ID, leader IDs and account keys, and the two tank messages.
  - Round 2 missed this. Round 4 touched the README only to add the new file's row.
- **Consequence:** the manifest no longer says what the file is for. A later reader, or F-29 and F-32 work that relies on it, would think the segment, tank and raw-ANT cases are not covered.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-09-r2-1 (records after a decoder-rejected record were kept) | **fixed** | See the four parts below the table |
| QA-09-r2-2 (OQ-52 bookkeeping) | **fixed** | See the parts below the table |
| r2 Q1 (keep or cut) | **answered**: cut (user) | Feature Decisions; ADR-023 rev. 4; D-20 |
| r2 Q2 (F-30 when the decoded prefix moves) | **answered in the design** | ADR-023 rev. 4 Consequences and the F-30 hand-off rule. OQ-53 for the user is not in scope here |
| r1 and r2 condition (FIT-viewer check per brand) | **open** (user) | Not something QA can run (F-32) |

**QA-09-r2-1, fixed:**
- **Code:**
  - `recordCounter` is registered on both hooks, fresh for each decoder call (`sequence.go:147-183`).
  - `seqSpan.records` takes the last call's count (`sequence.go:83-89`, `113`).
  - `stripSeq` writes exactly `records` records for a truncated span. For a complete span it requires exactly `records`. Any other count gives `errRecordCount` (`strip.go:58-121`).
  - `accept.go:44` wraps the error with `%w`.
  - In the library, `OnMesgDef` and `OnMesg` are called only after a record has been read and accepted in full (`decoder.go:613-620` and `692-694`), as ADR-003 rev. 5 says.
- **Probes:**
  - The 3 round-2 crashers: `Accept` stores 90 B, 132 B and 85,214 B. Each passes `checkStored`, `decodesToEnd` and idempotence.
  - The kept-records probe (`qa_tail_test.go`): now 60 records stored for 60 shown (round 2: 600 for 60). The serial-number marker of the copied `device_info` is not in the stored bytes. It passes `checkStored`, `decodesToEnd` and idempotence.
- **Fuzz:**
  - `FuzzAccept` with `checkStored` unchanged (the function is identical to round 3's, without my round-2 relaxations): 5 min 31 s green with the default settings (113,821 execs).
  - The failure found with a short minimization time belongs to a different class (QA-09-r3-1). It is not this finding: the input is refused, and no unread data is stored.
- **Commits:** the seeds are byte for byte my round-2 crashers (SHA-256 equal), under the original names.

**QA-09-r2-2, fixed:**
- ADR-023 rev. 4 item 4 has 20 fields in 12 messages and item 4a has 17 fields in 14 messages, with the 6 rule lines. I compared them row by row with `wantBlank`, `wantKept`, `idRules`, `blankTable` and `keptIdentifiers`: they are equal.
- OQ-52 is under Resolved in `INDEX.md`.
- I-8 (overview v1.20) names the segment account keys. So do the F-29 hand-off and the design's status line and Open questions.
- The comments and failure messages cite "rev. 4 item 4" and "item 4a".
- The 6 `TestStripTable` source mutants are killed.

**m3 (only `TestStripHandBuilt` kills it): enough.**
- I compared the library's record framing (`decoder.go:518-700`) with `readDefinition` and `stripSeq`:
  - header kinds and local-type masks;
  - size-0 fields and size-0 developer fields (both count no bytes);
  - developer field sizes;
  - compressed headers.

  Every record length is computed the same way.
- So with the pinned decoder, no input can make a complete span's count differ while the framing still ends exactly at the end of the region. The only boundary disagreement I found, QA-09-r3-1, hits `errStripFrame` first.
- The check guards against a later library version. A hand-built span is the only possible test, and ADR-003 rev. 5's upgrade check covers the library side.
- Small gap: the case "with `records` = N, the output is unchanged" (design, `TestStripHandBuilt`) checks only that there is no error, not the bytes (`strip_test.go:794-796`). `TestStripCorpus` compares bytes on every unmodified synthetic file, so this is accepted.

**Engineer's round-4 claims, checked:**

| Claim | Result |
| --- | --- |
| Deviation: existing hand-built spans gained `records` values | Confirmed (`strip_test.go:721`, `733`, `744`, `755`, `759`, `813-814`). Needed, because a complete span must now carry its count. Accept |
| Deviation: an extra truncated N-of-M case | Confirmed (`strip_test.go:798-802`: truncated, `records` = 2 of 3, 28 bytes). Stronger than the design. Accept |
| Deviation: wider `TestWalkRecords` file list | Confirmed: 9 files instead of the design's 5 (adds `cut-60-size-0`, `chained-ride-ride`, `triathlon`, `identifying-data`). Accept |
| Deviation: the shape of the developer-field variant | Confirmed. The copied `field_description` changes field number 0 to 1 and the base type to 0x30. The inserted `record` definition carries only developer field 1 (size 1) and no regular field. The test asserts that the upload is truncated and that the last three stored records are exactly the inserted description and definition. It fails on the round-3 restore. Accept |
| Deviation: approximate flood sizes | Confirmed. `definitionFlood` fills to at most 20,000,000 bytes and stops within one definition; `MaxBytes` is checked. Accept |
| Deviation: m1 as an overlay, not a real round-3 restore | I ran a real restore. Its set of failing tests is identical to m1's, so the overlay is faithful. Accept |
| Deviation: `TestStripTable` mutants not re-run after the comment-only change | Re-run as 6 source mutants, all killed. Accept |
| Mutants m1, m2, m3 as reported | Reproduced (table above) |
| Fuzz: 5 min, 89,601 execs, green; the rate fell to 0 execs/s | The log confirms the figures. The zero-rate stretches are minimization of large inputs, not a hang (QA-09-r3-2). The run was real but weak |
| Benchmarks: `Accept` at the caps 2.32 to 2.35 s; floods 0.44 to 0.71 s | Reproduced: 2.32 and 2.40 s; 0.45 to 0.72 s |
| A: 237 to 270 MB | Reproduced: 244 to 271 MB |
| Goldens: only `ride-30min-bad-basetype` is new this round | Confirmed by hash. The 8 files of `identifying-data` and `swim-bike-distance-fall` (`.fit`, golden report, `.bin`, data-file golden) have the same SHA-256 as at the end of round 2. The `swim-bike-distance-fall` change against `HEAD` is F-08 v1.6 (r5-2, the lap carries the session's distance). The new file inserts `4f 00 00 00 ff 01 00 01 30` (L = 15) at byte 32,961; every byte after it is unchanged. Its golden has `truncated` true, `crc_failed` false, `data_end` 1,079 and 1,080 samples (60 % of 1,800), as the design says |
| No unreported test deviation | I compared every test the v1.5 design names (`decodesToEnd`, `TestWalkRecords`, `TestStripRejectedRecord` (a) to (e), the `TestStripFrames` developer variant, the `TestStripHandBuilt` count cases, `TestStripCorpus` and `TestUploadFraming` for the new file, `FuzzAccept` (a) to (c) plus seeds, `TestCorpus`, `BenchmarkAcceptDefinitions`) with the code. One deviation was not listed: the `records = N` "output unchanged" check (above); accepted. Extra: `corpusAccept` also asserts that `Accept` rejects what `Parse` rejects |

## Questions

1. **For the architect (QA-09-r3-1): when the decoder reads past the header data size of a complete sequence, where does the stored sequence end?**
   - Option one: store the `records` records the decoder read, with the data size and CRCs rewritten. That is a framing repair under OQ-48, and it keeps I-8's "only records the parser read".
   - Option two: refuse the upload on purpose.
   - Either answer settles it once it is written into ADR-023 items 2 and 8 and backed by a test.
   - If refusal is chosen, it goes against F-08's own acceptance of the file (UP-3). Then the user should confirm, because it is a product choice.

## Coverage

**Acceptance criteria → code → test (all tests passing):**

| AC | Code | Tests |
| --- | --- | --- |
| 1 | `lifecycle.Upload`, `NewDraft`, `NewIdentity`, `store.InsertWorkout` | `TestUploadEndToEnd`, `TestNewDraft`, `TestUploadCreated` |
| 2 | `EditLink`; no token in any log call | `TestUploadEndToEnd`, `TestUploadCreated`, `TestUploadLogKeys`, `TestUploadLogs` |
| 3 | `upload.go` steps 5 and 6 | `TestUploadSizeCap` (a), (a2), (b), (c) |
| 4 | `clientaddr.go`, `ratelimit.go` | `TestRateLimit`, `TestClientPrefix`, `TestRateTableFull`, `TestLimiterMemory` |
| 5 | schema; no request metadata used outside the limiter key | `TestNoRequestMetadataStored`, `TestSchema`, `TestUploadLogs` |
| 6 | `strip_table.go` (ADR-023 rev. 4), `strip.go`, `accept.go`; `identifying-data.fit` | `TestStripIdentifyingData`, `TestUploadIdentifyingData`, `TestAcceptLogs`, `TestStripCorpus`, `TestStripTable` (and its mutants) |
| 7 | `stripSeq` with `seqSpan.records` (complete form, unclosed form, cut at the rejected record) | `TestUploadFraming` (incl. `ride-30min-bad-basetype.fit`: `original.fit` is exactly the upload up to byte 32,961, unclosed, `data_end_s` = the upload's), `TestStripFrames` (incl. the rejected developer field), `TestStripRejectedRecord`, `TestWalkRecords`, `TestStripCorpus`, `FuzzAccept` seeds. A header data size that ends inside a record is not handled (QA-09-r3-1) |
| 8 | `upload.go` slots, `settings.go`, `wiring.go` | `TestInFlight`, `TestSettings`, `TestServeInvalidSetting`, `TestSettingsReachTheHandler` |
| 9 | steps 4 to 7 | `TestUploadShape`, `TestUploadUnreadable` |
| 10 | `lifecycle` step 6 (D-18), `privatedir.Create`, `Recover` | `TestUploadServerError`, `TestServerErrorMapping`, `TestUploadFailures`, `TestCreateFails`, `TestUploadInsertNotCancelled`, `TestUploadCancelDuringInsert`, `TestRecover`, `TestServeRecovers` |
| 11 | `httpapi.fail` (422) | `TestUploadRejectedLeavesNothing`, `TestParseErrorMapping`, `TestAcceptRejections` |
| 12 | no `public/` from F-09; Go serves no `/w/` | `TestUploadEndToEnd`, `TestNoViewPath`. The generic page itself is F-15's |
| 13 | `base62`, `NewIdentity` | `TestUploadPublicIDs`, `TestBase62` |

**Checked and found correct:**
- The record counter changes nothing the listener sees, and the decoder options are exactly ADR-003 rev. 5's.
- `errRecordCount` is distinct from `errStripFrame`, its text holds only the offset and the two counts, and it unwraps through `*Error`.
- `decodesToEnd` handles closed sequences, unclosed ones and header-only ones as the design says.
- `checkStored` is unchanged.
- `TestCorpus` runs `decodesToEnd` and idempotence on synthetic files in CI and on the real files locally.
- The F-12 and F-30 hand-offs say that every stored sequence reads to its end.
- **Git hygiene:**
  - Nothing under `corpus/real/`, `expected/real/` or `build/`, and no `.br`, is untracked and unignored.
  - The only untracked `.fit` and `.bin` files are under `synthetic/`. `TestSyntheticCorpusCurrent` passes, so they are what `corpusgen` writes.
  - The three committed fuzz inputs are byte-identical to QA's round-2 crashers, which came from runs seeded only from `synthetic/`. None of their 16-byte windows matches a real file without also matching a synthetic one.
  - No real-file figure is in `MEASUREMENTS.md` (Round 4 has synthetic figures only), the corpus README, the round-4 changes in `architect/`, or the feature files.

**Not run, or partly run:**
- The M(4) and M(16) server runs and `TestLimiterMemory`: not required this round, and the code they measure (`lifecycle`, `httpapi`) is byte-identical to round 2.
- A 5-minute `FuzzAccept` run with a short minimization time ending green: not possible until QA-09-r3-1 is settled. My effective run stopped at 67 s on that failure.
- The FIT-viewer inspection (user, F-32).
- Third-party FIT readers: none is available here.
