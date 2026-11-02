# QA F-09 Upload API and workout storage - round 2

- **Date:** 2026-10-16
- **Verdict:** **ready with conditions.** There is no Blocker or Major finding. All six round-1 findings are fixed, and each fix is proven by a mutant that now fails. Q1 is built as the user decided, including OQ-52 ("blank"). All 13 acceptance criteria, including the extended AC 6, are traced to code and to passing tests. A is at most 270 MB, within the 400 MB limit. Conditions:
  1. **QA-09-r2-1 is decided by the architect and closed before F-12 (owner download) and F-30 (regenerate) are built.** In a sequence that the decoder ended at a record it rejects, the strip keeps every record after that point. The stored original can then hold far more than the workout the sharer sees (in my probe, 600 of 600 records stored, 60 shown). The library's decoder stops at the same record, so `FuzzAccept`'s stored-file check fails. Every fuzz run hits this within about a minute. UP-9 still holds for those bytes, and the upload did not have to be refused under the rules as written.
  2. **The user's FIT-viewer check** (r1 condition 2, design row 15, F-32) is still open, and only the user can do it.
- **Inputs:**
  - Feature: `features/F-09-upload-api-and-storage.md` (revised scope and AC 6; Decisions including QA-09-r1 Q1 and OQ-52); `features/README.md` decisions; F-29 and F-31 changes.
  - Spec: Product Specification UP-9 (5 parts, revised), INF-1, SH-8, NFR Privacy, Security and Operations; Technical Design Publish 2 and Security and privacy (Originals).
  - Architecture:
    - `architect/F-09-upload-api-and-storage.md` v1.4 (D-18, D-19, Memory with L, Tests, round-3 list and stop conditions);
    - ADR-023 rev. 3, ADR-022 rev. 2, ADR-021 rev. 3, ADR-017 rev. 4, ADR-003 rev. 4;
    - `architect/overview.md` v1.19 (I-8);
    - `architect/INDEX.md`.
  - Previous round: `qa/F-09-upload-api-and-storage-r1.md`.
  - State: base `8ab9085` plus the uncommitted working tree. `git status` and the SHA-256 of every modified and untracked file were the same at the start and at the end of the review.
  - QA changed nothing outside `qa/`:
    - Mutants and probes ran through `go test -overlay`, with their files under `<scratchpad>/acf38496-ea04-454f-9beb-6c089f0f46a3/scratchpad/qa/`.
    - Fuzzing ran on a copy of `src/server` in the same scratch directory, without the real corpus, so no failing input was written into `src/`.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0 (21.6 s). gofmt clean, `go vet` clean, every Go package `ok`. The Go results were cached, so I also ran `go test -count=1 ./...` below. The front-end build passed. Bundle 45,582 / 60,000 B gzip, PASS |
| `go test -count=1 ./...` (in `src/server`) | exit 0, all 12 packages `ok` (21.0 s) |
| `go test -count=1 -race` on `ingest`, `lifecycle`, `store`, `privatedir`, `httpapi`, `cmd/workout-explorer`, `datafile`, `workout`, `tools/corpusgen` | exit 0, all 9 `ok` (2 m 19 s; `ingest` 137 s) |
| `make corpus` | `corpus: 50 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` (44 synthetic, 6 local real). `identifying-data.fit` ok, bin 18,352 B |
| `go mod tidy -diff` | no output, exit 0 (QA-09-r1-6 fixed) |
| `go test ./internal/ingest -run '^$' -bench AcceptAtCap -benchtime 3x -count 2 -benchmem` | 2.438 s and 2.516 s per op, within 4.5 s. 885.8 MB/op |
| A: `/usr/bin/time -v build/workout-explorer parse -original <scratch> <scratch>/cap-500k.fit`, all-caps twin from `corpusgen -cap-file` (17,360,175 B, identical to `build/cap-500k.fit`), `GOGC` and `GOMEMLIMIT` unset | Max RSS 263,156 / 260,940 / 270,272 kB (3 runs, while the fuzzer was running beside them, wall clock 7.2 to 7.6 s). A fourth run without other load: 259,960 kB, 3.38 s. All runs within 400 MB, exit 0 |
| `TestLimiterMemory -v`, 3 runs | L = 29,541,752 / 29,530,504 / 29,530,616 B, limit 40,000,000 |
| `TestAcceptReleasesFirstParse -v`, 3 runs | excess 1,283,552 / 1,283,984 / 1,289,080 B, limit 2,348,407 (stripped size 1,299,831 + 1 MiB) |
| `TestUploadCancelDuringInsert`: `-race -count=3`; `-count=5` | ok; ok |
| `FuzzAccept`, 90 s, test as committed (scratch copy) | **FAIL after 61 s**, a third input of the class the engineer reported: `decoding the stored file: … MesgNumInvalid(27681): fields[0].BaseType: invalid(5): invalid basetype` (QA-09-r2-1) |
| `FuzzAccept`, 300 s. My scratch copy changes `checkStored` in two ways. It accepts a decoder stop with "invalid basetype" in the unclosed form. It also checks the blank and remove tables with the strip's own framer over **every** stored byte | PASS: 198,200 execs, 77 corpus entries. With my three crashers as seeds, all pass. No input left a blank-table field valid or a removed message in the stored bytes |
| Server run M(4) | not repeated. Design round-3 row 16 says it need not be, and no change touches the parse. r1 measured 620 to 677 MB |

### Mutation probes (`go test -overlay`, nothing in `src/` changed)

| Guarantee | Mutant | Killed by |
| --- | --- | --- |
| D-18 insert not cancellable | Round-1 step 6: insert with `ctx`, remove the folder on any error | `TestUploadCancelDuringInsert` (3 of 3 runs, 154 to 165 failed uploads out of 200, 49 to 65 rows without a folder, with the fuzzer running beside it), `TestUploadInsertNotCancelled`, `TestUploadFailures` |
| | Insert with `ctx`, existence check with `sctx` | `TestUploadCancelDuringInsert` (3 of 3: 166 to 192 rows, and as many folders, instead of 200), `TestUploadInsertNotCancelled` |
| | Existence check with `ctx` | `TestUploadInsertNotCancelled` |
| | No existence check (always remove) | `TestUploadFailures`, `TestUploadInsertNotCancelled` |
| One-minute full-table sweep (r1-2) | `fullSweepEvery` branch removed | `TestRateTableFull` (a): 1 WARN instead of 2; (b): refused instead of allowed |
| L ≤ 40 MB (r1-4) | `[]time.Time` per key | `TestLimiterMemory` (159,005,128 B) |
| First parse released (r1-3) | `w = workout.Workout{}` removed | `TestAcceptReleasesFirstParse` (4,660,080 B) |
| | `outcome` keeps `*listener` | `TestAcceptReleasesFirstParse` (5,547,536 B) |
| UP-9 literal (Q1) | `ant_channel_id.device_number` dropped from `blankTable` | `TestStripTable`, `TestStripIdentifyingData`, `TestUploadIdentifyingData` |
| | Round-2 tables (all v1.4 rows and 80, 81 removed) | the same three |
| | OQ-52 keys not blanked | the same three |
| | `ant_rx` dropped from `removeTable` | the same three |
| Identifier rule (item 4a), as a source mutant of the test | rule line "ends with device_number" removed from `idRules` | `TestStripTable` |
| | rule line "profile type ant_channel_id" removed | `TestStripTable` |

Expected survivors that are not findings:
- **Forward reslicing** (`k.times = k.times[i:]`) survives `TestLimiterMemory` (29.46 MB). The design says the copy-down rule is checked by code review. `ratelimit.go:94-97` does the copy-down.
- **Removing the pre-allocated capacity** survives at 31.1 MB, still under the 40 MB bound.

## Findings

### Minor

- **ID:** QA-09-r2-1
- **Severity / kind:** Minor / defect (a design gap, so the test's check and the code disagree)
- **Route:** architect (decide where a decoder-ended truncated sequence stops, and what `FuzzAccept` must check), then backend (implement; fix `checkStored`; commit the crashers as seeds if the design adopts Go's `testdata/fuzz` convention)
- **Where:**
  - `src/server/internal/ingest/strip.go:56-63` and `121-153`. `readDefinition` frames any definition by its sizes and never checks base types.
  - `src/server/internal/ingest/sequence.go:118`. A sequence ended by a decoder error (not EOF) gets `seqTruncated` with `dataEnd` = the end of its data, not the position where the decoder stopped.
  - `src/server/internal/ingest/accept_test.go:64`. `checkStored` accepts only EOF for the unclosed form.
  - `architect/ADR-023-…md` items 2 ("so it cannot disagree with the parser about boundaries"), 6 ("framed as far as they go") and 8.
- **Relates to:** UP-9 ("the upload with only these changes, plus repaired framing"); AC 6 and AC 7 ("same workout data"); ADR-023 items 2, 6 and 8; I-8; design `FuzzAccept`; F-30 hand-off; SH-8 owner download.
- **Evidence:**
  - **Root cause.**
    - The decoder of `muktihari/fit` v0.28.4 rejects some records that the strip's framer can still read:
      - a definition with an invalid base type (`decoder.go:572-575`; base type 5 without the endian bit counts as invalid too);
      - a data message whose developer field points to a field description with an invalid `fit_base_type_id` (`decoder.go:891-893`).
    - F-08's walk then marks the sequence truncated and gives the workout up to that record.
    - The strip walks the same sequence with its own framer. It accepts the rejected definition (only the sizes matter to it) and copies every record that follows, up to the end of the data region. Only a last record that is incomplete, or that cannot be framed at all, is dropped.
    - The stored file is written in the unclosed form. It holds the record the decoder rejects, followed by the rest of the sequence.
  - **The engineer's two crashers** (copied to `…/scratchpad/qa/crashers/`; probe `…/scratchpad/qa/probes/qa_crash_test.go`):
    - Both are one sequence whose header says it is 808 MB long. The workout has 1 record, and the 2nd definition (local 1, global 12336, base type 0x30) is invalid.
    - The strip keeps that definition and drops only the incomplete record after it.
    - The library's decoder fails at the same byte position (111 and 186) on the stored bytes, and also on the upload patched to its own length. So the strip does not create the bad record; it keeps it.
    - My own 90 s run found a third input of the same class (base type 5).
  - **How much can be kept** (probe `…/scratchpad/qa/probes/qa_tail_test.go`):
    - Input: `identifying-data.fit` with a 9-byte definition with an invalid base type inserted after 10 % of the records, followed by a copy of a `device_info` holding the serial-number marker, then closed with the right data size and CRCs.
    - `Accept` succeeds: 60 samples, truncated, `data_end` 59 s.
    - The stored file holds **all 600 record messages**, in 19,776 B against 19,892 B uploaded.
  - **UP-9 still holds for the kept tail.**
    - In that probe the marker is gone from the stored bytes (17 fields blanked, including the copied `device_info`).
    - My 300 s fuzz with a framer-based table check over every stored byte found no valid blank-table field and no removed message.
    - The 6 local real files pass the same check.
  - **F-30 regeneration works today.** `Accept` on the stored original returns identical bytes and the same workout: for the probe file, for all 6 local real files, and for the crashers.
- **Answers to the three questions:**
  - **Root cause:** as above. The strip and the decoder disagree about where a sequence that the decoder ended at a rejected record stops. ADR-023 item 2 says they cannot disagree. That holds for sequence boundaries, but not for the stop point inside a truncated sequence.
  - **Can a stored original fail F-30 or the owner's download?**
    - F-30 with the pinned library: no, regeneration is idempotent (above).
    - F-30 after a library upgrade that accepts what v0.28.4 rejects (it already only logs, and does not fail on, size-0 fields): the regenerated workout would grow. In the probe it would grow from 60 to 600 samples, with positions the sharer never reviewed, on workouts that may already be published.
    - Owner's download: any reader built on this library stops at the same record as it would on the upload. So the download is no worse than the upload for that reader, but it is not a clean file. I could not check third-party viewers (F-32, user).
  - **Should the upload have been refused?** Not under the rules as written:
    - ADR-023 item 8 fails closed only on a *complete* sequence that cannot be framed, a rejected reparse or a reference mismatch.
    - I-8 is not broken: the tail is stripped like everything else.
    - Refusing would also reject files F-08 recovers (UP-3).
    - What is missing is a decision on whether bytes the parser never read may be stored at all. The verify step (item 1.5) proves only the decoded prefix.
- **Consequence:**
  - `FuzzAccept`, a test the design names, fails within about a minute on every run. The fuzz target is useless for finding new defects until this is settled. CI runs only the seeds, so CI stays green.
  - Stored originals can hold up to the whole rest of a 20 MB file beyond what the sharer was shown. This goes against "store as little as possible" and against what the UP-3 notice tells the sharer.
  - It is a regeneration hazard for F-30 after a library upgrade.
- **Options and the evidence that would close it:**
  - (a) End a decoder-ended sequence at the record the decoder rejected. For example, `readDefinition` could reject a definition whose base type fails the library's own `basetype.BaseType.Valid()`, so the two cannot drift. The developer-field case also needs the field descriptions, or a record count from the walk.
    - Evidence: the three crashers and the QA tail probe stored up to the rejected record, decoded by the library to EOF; `FuzzAccept` unchanged and green for 5 minutes.
  - (b) Keep the bytes, record the decision in ADR-023 items 2 and 6, and change `checkStored`: accept a decoder stop at a record the library rejects in the unclosed form, and check the tables with the strip's framer over every stored byte (as my scratch oracle does).
    - Evidence: the ADR text, the crashers committed as seeds, and a green 5-minute fuzz run.
  - Either way, F-30's design should say what happens when a library upgrade changes the decoded prefix.

- **ID:** QA-09-r2-2
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:**
  - `architect/F-09-upload-api-and-storage.md:4` (status: "OQ-52 … is open"), `:551` (Open questions) and `:536` (F-29 hand-off without the account keys);
  - `architect/ADR-023-…md:81-84` (the three keys "pending OQ-52" in `keptIdentifiers`; item 4's table lacks them);
  - `architect/INDEX.md:66` (OQ-52 under Open questions);
  - `architect/overview.md:35` (I-8 does not name the segment account keys).
- **Relates to:** OQ-52 (decided "blank" by the user, 16 Oct 2026); D-19; feature AC 6; spec UP-9 (4).
- **Evidence:**
  - The feature file, the spec, `features/README.md` and the code treat OQ-52 as decided:
    - `strip_table.go:32-37` blanks 148/4, 151/4 and 151/8;
    - `strip_test.go:870-916` `wantBlank` has them, and `wantKept` lacks them;
    - `TestStripIdentifyingData` expects 16 blanked fields.
  - The design's round-3 row 3 allowed this conditionally.
  - But `TestStripTable` reports a blank-table entry outside the list as "not in ADR-023 item 4 (with OQ-52)". It asserts a table the ADR does not contain.
- **Consequence:**
  - The normative document (ADR-023 is "normative" per the design) disagrees with the code and the test that enforces it.
  - F-29's text, and a future profile-upgrade review, would start from the wrong list.
  - Needed: ADR-023 rev. 4 (item 4 gains the three keys, item 4a loses the pending rows), OQ-52 moved to Resolved, I-8 and the F-29 hand-off updated.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-09-r1-1 (cancelled insert leaves a row without its folder) | **fixed** | `lifecycle.go:87-104`: `context.WithoutCancel` for the insert and `WorkoutExists`, the three branches as in design step 6. `store.WorkoutExists` and `DeleteWorkout` added. `Recover` deletes rows without a folder (`lifecycle.go:166-175`) and logs `rows_removed`. ADR-022 rev. 2 items 4 and 5 document it. All four mutants are killed (table above). `TestUploadCancelDuringInsert` is green with `-race` (3 runs) and without it (5 runs) |
| QA-09-r1-2 (`TestRateTableFull` could not fail) | **fixed** | `upload_test.go:484-523` as in the design: clock steps under 10 minutes; (a) WARN count 1, 1, 2; (b) refused at 59 s, allowed at 61 s. The full-sweep mutant fails both |
| QA-09-r1-3 (`TestAcceptReleasesFirstParse` margin) | **fixed** | `runtime.KeepAlive(upload)` at `accept_test.go:251`. Excess 1.28 MB, as expected. Both retention mutants fail, at 4.66 MB and 5.55 MB |
| QA-09-r1-4 (limiter memory) | **fixed** | `uint32` seconds since `epoch`, rounded up, one array of capacity 60 per key, copy-down (`ratelimit.go`). L = 29.5 MB, within 40 MB; the `[]time.Time` mutant gives 159 MB and fails. L is in ADR-021 rev. 3, ADR-017 rev. 4, design section Memory, the F-31 hand-off and OQ-51. The limit (60), window (1 h), key cap (100,000) and fail-closed rule are unchanged; `TestRateLimit` and `TestInFlight` are green |
| QA-09-r1-5 (ADR-022 WARN contradiction) | **fixed** | ADR-022 rev. 2 item 5 says there is no WARN. `Recover` writes one INFO line; `TestRecover` asserts exactly one line |
| QA-09-r1-6 (go.mod, `stage=deadline` line, dropped Close errors) | **fixed** | `go mod tidy -diff` is empty. The deadline line is a WARN in the design's Logging table, and `TestUploadLogs` reaches it through `deadlineFailWriter` (2 lines, upload still `201`). `store.Open` returns the `f.Close` error. `serve` calls `closeApp` explicitly on every return after `newApp` and exits 1 with `shutdown incomplete` (`main.go:107-139`) |
| QA-09-r1 Q1 (UP-9 literal) | **closed in code and tests; documentation partly stale (QA-09-r2-2)** | `blankTable` adds 82/2, 148/5, 151/9, 151/10, 319/0, 323/0 and (OQ-52) 148/4, 151/4, 151/8. `removeTable` = {3, 29, 80, 81, 149}. `TestStripTable` applies item 4a to the whole pinned profile in both directions. The new messages are in `identifying-data.fit`, each binary marker at its carrying fields only (`markerFields`). 16 blanked fields, 5 removed messages; all markers absent from `original.fit`, the database, `draft.bin` and the log |
| r1 condition 2 (FIT-viewer check per brand) | **open** (user) | Not something QA can run |

Engineer's round-3 claims, checked:

| Claim | Result |
| --- | --- |
| Deviation: extra `TestRateTableFullHandler` | Confirmed (`upload_test.go:527-558`), the round-1 handler-level test kept beside the rewritten unit test. Harmless; it still uses a 2-hour step, but the rewritten test covers the one-minute rule |
| Deviation: in-test mutants through `tableProblems` | Confirmed. Each of the 6 rule lines, `device_number`, each `keptIdentifiers` entry, a transmission type and a removed message is tried. I also ran two of the rule-line mutants and four table mutants as **source** mutants: all killed. Accept |
| Deviation: `TestUploadInsertNotCancelled` fake stores the row and then errors | Confirmed (`lifecycle_test.go:515-536`). It is stronger than the design's version, because the existence check then actually runs and its context is checked. Accept |
| Deviation: `TestLimiterMemory` key spread | Confirmed (keys × 0x9E3779B97F4A7C15). Distinct keys as the design asks. Accept |
| Deviation: extra `markerFields()` check | Confirmed (`strip_test.go:206-220`). It is the design's "each binary marker occurs only at its carrying fields", made executable. Accept |
| Deviation: "Round 3" section in `MEASUREMENTS.md` | Confirmed. Synthetic figures only: 1.28 MB, 4.6 MB, 5.6 MB, L 29.5 MB and 159 MB for the old representation. No real-file figure |
| Deviation: `cleanup` and `store` log order | Confirmed. In the "row absent" branch `cleanup` is logged before `store`; in the "check failed" branch `store` comes first. The design does not fix an order, and both are tested. Accept |
| Mutant: insert race fails `TestUploadCancelDuringInsert` 100 to 132 of 200 | Reproduced qualitatively: 154 to 165 failed uploads out of 200 in my 3 runs, with the fuzzer running beside them |
| Mutant: full-sweep mutant fails `TestRateTableFull` | Reproduced, (a) and (b) |
| `TestLimiterMemory` about 29.5 MB; the `[]time.Time` mutant fails | Reproduced: 29.53 to 29.54 MB; 159.0 MB |
| `TestAcceptReleasesFirstParse` about 1.28 MB; both retention mutants fail | Reproduced: 1.284 to 1.289 MB; 4.66 and 5.55 MB |
| Fuzz finding reported, not fixed | Reproduced, and a third input found. Root cause and classification in QA-09-r2-1 |
| No unreported test deviation | I compared every test the v1.4 design names with the code (`ingest` 13, `lifecycle` 9, `store` 8, `privatedir` 4, `httpapi` 16, `cmd` 13). All are present and assert what the design says. The deviations are the ones listed above, plus extra tests (`TestStripStop`, `TestAcceptKeepsOneSlot`, `TestUploadDeadlines`, `TestNewHandlerPanicsWithoutSlots`, `TestOpenPathNeedsEscaping`) |

## Questions

1. **For the architect (and the user, if it reads as a product choice): may `original.fit` keep records that F-08's decoder never read?** (QA-09-r2-1)
   - UP-9 says the stored file is "the upload with only these changes"; that argues for keeping them.
   - "Store as little as possible", the UP-3 notice and a clean file for F-30 and SH-8 argue for cutting at the rejected record.
   - Either "cut at the rejected record" or "keep, and check the tail with the strip's framer" settles it.
2. **For the architect: when the pinned library is upgraded, what does F-30 do if the decoded prefix of a stored original changes?** For example, it could refuse to replace the row and log, or re-run the sharer's review. This settles the regeneration hazard in QA-09-r2-1, whichever option is chosen there.

## Coverage

**Acceptance criteria → code → test (all tests passing):**

| AC | Code | Tests |
| --- | --- | --- |
| 1 | `lifecycle.Upload`, `NewDraft`, `NewIdentity`, `store.InsertWorkout` | `TestUploadEndToEnd`, `TestNewDraft`, `TestUploadCreated` |
| 2 | `EditLink`; no token in any log call | `TestUploadEndToEnd`, `TestUploadCreated`, `TestUploadLogKeys`, `TestUploadLogs` |
| 3 | `upload.go` steps 5 and 6 | `TestUploadSizeCap` (a), (a2), (b), (c) |
| 4 | `clientaddr.go`, `ratelimit.go` (v1.4 representation) | `TestRateLimit`, `TestClientPrefix`, `TestRateTableFull`, `TestLimiterMemory` |
| 5 | schema; no request metadata used outside the limiter key | `TestNoRequestMetadataStored`, `TestSchema`, `TestUploadLogs` |
| 6 (extended) | `strip_table.go` (v1.4 and OQ-52), `strip.go`, `accept.go`; `identifying-data.fit` with every message the AC lists | `TestStripIdentifyingData` (16 blanked, 5 removed; transmission types, `device_type`, `device_index`, `uuid`, `file_uuid`, segment name, sport name, developer field and both zone messages unchanged; markers only at their fields), `TestUploadIdentifyingData`, `TestAcceptLogs`, `TestStripCorpus`, `TestStripTable`; mutants killed |
| 7 | `stripSeq` (complete and unclosed forms) | `TestUploadFraming`, `TestStripFrames`, `TestStripCorpus`. AC 7 holds for "incomplete last record". A record the decoder rejects is not covered by any rule (QA-09-r2-1) |
| 8 | `upload.go` slots, `settings.go`, `wiring.go` | `TestInFlight`, `TestSettings`, `TestServeInvalidSetting`, `TestSettingsReachTheHandler` |
| 9 | steps 4 to 7 | `TestUploadShape`, `TestUploadUnreadable` |
| 10 | `lifecycle` step 6 (D-18), `privatedir.Create`, `Recover` | `TestUploadServerError`, `TestServerErrorMapping`, `TestUploadFailures`, `TestCreateFails`, `TestUploadInsertNotCancelled`, `TestUploadCancelDuringInsert`, `TestRecover`, `TestServeRecovers` |
| 11 | `httpapi.fail` (422) | `TestUploadRejectedLeavesNothing`, `TestParseErrorMapping`, `TestAcceptRejections` |
| 12 | no `public/` from F-09; Go serves no `/w/` | `TestUploadEndToEnd`, `TestNoViewPath`. The generic page itself is F-15's |
| 13 | `base62`, `NewIdentity` | `TestUploadPublicIDs`, `TestBase62` |

**Checked and found correct:**
- The limiter's `stamp`, `expiry` and `retryAfter` follow the design's formulas. The sweep uses the same expiry rule.
- `Recover` deletes rows only after `IDs` has succeeded, and counts unknown entries without touching them.
- `store.WorkoutExists` returns an error, not "absent", when the database fails (tested on a closed database).
- The `closeApp` exit code: `shutdown incomplete` and exit 1.
- The F-29 feature and the spec INF-1 list the account keys and the raw ANT messages.
- The F-31 feature names 2 GB, N = 4 and `MemoryMax`. It says "plus headroom" rather than naming L and 2·A, but the F-09 hand-off to F-31 names both.
- Git hygiene:
  - Nothing under `corpus/real/`, `expected/real/` or `build/`, and no `.br`, is untracked-and-unignored.
  - `internal/ingest/testdata/fuzz/` does not exist in the tree. The engineer did not leave the crashers there, and my fuzz runs wrote only to the scratch copy.
  - No real-file figure is in `MEASUREMENTS.md`, the corpus README, `architect/` or the feature files.

**Local real files (aggregates only, ADR-004 item 7):** 6 files from 3 manufacturers.
- `Accept` succeeds on all 6, and `TestCorpus` passes its decoder check on them.
- In total 50 fields were blanked and 1 message was removed. That is unchanged from r1: none of the v1.4 or OQ-52 fields or messages occur in them.
- My framer-based table check over every stored byte passes.
- `Accept` on each stored original returns identical bytes (F-30 idempotence).

**Not run, or partly run:**
- M(4) and M(16) server runs (not required this round; r1 figures stand).
- The FIT-viewer inspection (condition 2, user).
- Third-party FIT readers on the QA-09-r2-1 stored files: none is available here, and no external tool or service was used.
