# QA F-09 Upload API and workout storage - round 4

- **Date:** 2026-10-22
- **Verdict:** **ready.** No Blocker or Major finding is open. All 14 acceptance criteria are traced to code and to passing tests. For AC 14 I also verified the end-to-end path by hand (see Coverage). All three round-3 findings are fixed. I reproduced every claim of the engineer's round-8 hand-off, except the sweep's running time. Five Minor findings remain as follow-ups. The one that matters is QA-09-r4-1: the closed list of deliberate refusals misses a case. An 83-byte synthetic file makes `Accept` return `internal`, and `FuzzAccept` fails on it. Nothing is stored or leaked. The client gets the same sentence. The only effects are the kind and the log level, so it does not block the feature. Under D-29 it goes the normal route: architect, then backend.
- **Inputs:**
  - Feature: `features/F-09-upload-api-and-storage.md` (14 ACs; Decisions of 16 and 21 Oct 2026).
  - Spec: UP-2, UP-3, UP-9, UP-10 and NFR Security/Privacy, as cited by the feature.
  - Architecture:
    - `architect/F-09-upload-api-and-storage.md` v1.9: D-26 to D-29, Strip "Deliberate refusals", Tests, the round-7 review and the round-8 list;
    - ADR-023 rev. 8 (items 3a, 6, 8, 8a, Consequences), ADR-018 rev. 5, ADR-004 rev. 10, ADR-019 to ADR-022.
    - During the review the architect moved the design to v1.10, ADR-023 to rev. 9 and ADR-004 to rev. 11 (the user's 22 Oct 2026 decision about real-device files). I checked findings QA-09-r4-1 and -r4-4 against those versions too.
  - Previous rounds: `qa/F-09-upload-api-and-storage-r1.md` to `-r3.md`.
  - State: base `8ab9085` plus the uncommitted working tree.
    - The SHA-256 of every modified and untracked file under `src/` was the same at the start and at the end of the review, and `git status` for `src/` did not change.
    - Since my round-3 snapshot, production code changed only in `internal/ingest` (`accept.go`, `errors.go`, `ingest.go`, `log.go`, `strip.go`, `strip_table.go`, new `frame.go`) and in `tools/corpusgen`.
    - `lifecycle`, `store` and `privatedir` are byte-identical to round 3. `httpapi` and `cmd` changed only in `upload_test.go`.
  - QA changed nothing outside `qa/`:
    - Probes and mutants ran in copies of `src/server` without `corpus/real/`, under `<scratchpad>/acf38496-ea04-454f-9beb-6c089f0f46a3/scratchpad/qa-r4/`.
    - The real-file aggregate ran as a `go test -overlay` file from the same scratch folder.
  - Not in scope, as instructed: the "files from real devices are always accepted" wording (being removed by requirements and architect), and crafted-input refusals on the closed list (U1, U2).

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0 (21.6 s). gofmt and `go vet` clean, `svelte-check` 0/0, all Go packages `ok` (cached), web 806 tests passed, bundle 45,582 / 60,000 B gzip PASS |
| `go test -count=1 ./...` (`src/server`) | exit 0, all 12 packages `ok` (76.7 s; `ingest` 75.7 s). Includes `TestCorpus`/`TestStripCorpus` on the 6 local real files |
| `gofmt -l .`; `go mod tidy -diff` | no output; exit 0 |
| `make corpus` | `corpus: 52 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` (46 synthetic, 6 local real) |
| `go test ./internal/ingest -run '^FuzzAccept$' -v` (seeds only) | ok, 54 subtests: 46 synthetic seeds and 8 committed inputs (including `empty-complete-seq`) |
| `TestFramingSweep -v` (scratch copy, 3 runs) | PASS each time. Part 1: probe 158 B, 40,290 mutants (1,274 without spans), 209 refused as U1, 0 failed, 5.9 / 7.1 / 8.4 s. Part 2: lead 57 B, 29,424 mutants (reduced value set), 209 U1, 0 U2, 0 failed, **31.5 / 35.6 / 37.3 s** (budget 20 s; engineer 22 s; load average about 2 on 8 threads). See QA-09-r4-2 |
| `FuzzAccept`, own run: fresh `-test.fuzzcachedir`, `-fuzztime 5m -fuzzminimizetime 5s`, scratch copy | **PASS**. 1,944,156 execs, 965 new interesting (1,018 in the corpus). 1 of 103 status lines shows 0 execs/s. No input written. I did not run the 30-minute run (instruction); the engineer's log is checked below |
| `go test -count=1 -race` on `cmd/...`, `httpapi`, `lifecycle`, `store`, `privatedir` | all `ok` |
| `go test -count=1 -race ./internal/ingest -run '<round-8 tests>\|FuzzAccept\|TestStripCorpus\|…'` (5 runs, 2 of them while the mutant runs loaded the machine) | 4 × ok. **1 × FAIL** under load: `accept_test.go:478: integrity: Accept(stored): ingest: limit (time): time limit` (QA-09-r4-3). It did not recur in 4 more race runs |
| `go test ./internal/ingest -run '^$' -bench 'AcceptAtCap\|AcceptDefinitions' -benchtime 3x -count 2 -benchmem` | `AcceptAtCap` 2.34 / 2.33 s, 885.8 MB/op (unchanged since r3). `AcceptDefinitions/fields=1` 0.83 / 0.83 s, **283.2 MB/op** (r3: 0.71 s, 176.5 MB/op). `fields=255` 0.47 / 0.46 s, 274.5 MB/op. All within 4.5 s |
| Round-8 tests, `-v` | `TestAcceptEmptySequence`, `TestStripEmptyHandBuilt`, `TestRemovedTimestampDependency` (7 cases), `TestCheckStoredBytes`, `TestAcceptRemovedTimestamp` (a)–(c), `TestAcceptFailsClosed`: all PASS. `TestAcceptDataSizeSweep` 3.7 s; `TestBlankFill` 1.4 s |
| Local real files (overlay probe; counts only, ADR-004 item 7) | 6 of 6 accepted, 0 refused as U1, 0 as U2, 0 other. 50 fields blanked, 1 message removed, 0 truncated, the same as rounds 1–3. The SHA-256 of the 6 stored outputs equals the engineer's round-7 snapshot in the scratch folder |

### Mutation probes

Each mutant ran as a scratch copy, and I ran the whole `ingest` package against it. Every mutant was killed.

| Mutant | Killed by |
| --- | --- |
| m12 `bigEndian` = `arch == 1` | `TestFramingSweep` (unchanged probe: "record 4: global number differs"), `TestBlankFill`, `TestStripHandBuilt` (vi), (vii), `FuzzAccept/crasher-05bdce6d7833d6a3` |
| m13 definition iff bit 6 set | `TestFramingSweep` (unchanged probe, record 8), `TestStripHandBuilt` (compressed removed message), `TestRemovedTimestampDependency` (6 cases), `TestAcceptRemovedTimestamp` (a), (b) |
| m14 developer field sizes left out | `TestFramingSweep` (record 14, data length), `TestStripCorpus`, `TestCorpus`, `TestAcceptDataSizeSweep`, `TestAcceptLogs`, `FuzzAccept` (2 seeds) |
| m15 local type `& 0x07` | `TestFramingSweep` only: part 1 and part 2 each 381 mutants, the first at offset 14, value 0x48 ("local type differs") |
| m16 `emptyComplete` always false | `TestFramingSweep` (part 1 and part 2 each 209, offset 19, value 0x2F), `TestAcceptEmptySequence`, `TestStripEmptyHandBuilt`, `FuzzAccept/empty-complete-seq` |
| m16b (own) only `strip`'s U1 pre-loop removed; `stripSeq`'s guard kept | `TestStripEmptyHandBuilt` only (`stop` is called). Enough: the guard in `stripSeq` still stops any output, so the only observable change is that `stop` runs first |
| m17 `errEmptySequence` → `internal` | `TestFramingSweep` part 2 (209, offset 19, value 0x2F: "internal: strip: empty complete sequence: sequence at byte 57"), `TestAcceptEmptySequence`, `FuzzAccept/empty-complete-seq` |
| m18 U2 classifier always true | `TestAcceptFailsClosed` ("a different workout", "a record byte altered") |
| m19 (own) U2 flag not reset per span | `TestRemovedTimestampDependency/removed in span 0, compressed in span 1` |

## Findings

### Minor

- **ID:** QA-09-r4-1
- **Severity / kind:** Minor / defect, in the architecture: the closed list is incomplete. The code implements ADR-023 item 8a exactly.
- **Route:** architect first: extend U2, or add a U3, in ADR-023 item 8a and audit row X1. Then backend: classifier, test, and the input committed as a seed.
- **Where:**
  - `architect/ADR-023-…md` item 8a, row U2 ("a data message that item 5 removes has a field 253 of size above 0"), and Consequences (rev. 5: "the case covers any removed message whose definition has field 253");
  - the design's audit row X1;
  - `src/server/internal/ingest/strip.go:229-259` (`removedTimestampDependency`; line 249 uses only `hasTimestamp(rec.def)`).
- **Relates to:** D-26, D-28, D-29 (P3: "`KindInternal` whatever its cause … fails the target"); ADR-018 rev. 5 (`internal` always means a defect); the feature's acceptance rule of 21 Oct 2026.
- **Evidence:**
  - **Root cause.** In the pinned decoder, a compressed-timestamp header also moves the decoder's clock (`decoder.go:641-644`: `d.timestamp += …; d.lastTimeOffset = timeOffset`). Audit X1 cites those very lines, but U2 counts only field 253. So when a removed message (`user_profile`, 29, `ant_rx`, `ant_tx`, 149) has a compressed header and a kept compressed `record` follows, removing it moves that record. The reference then differs, the classifier returns false, and the upload is refused as `internal`.
  - **Probe** (`qa4_compressed_removed_test.go` in the scratch copy):
    - Five records with normal timestamps.
    - An `ant_tx` (or a `user_profile`) definition without field 253, and its data message with a compressed header 20 s later.
    - A `record` definition without field 253, and its data message with a compressed header another 20 s later.
    - `Parse` accepts it. `Accept` returns `ingest: internal: strip: the stripped file is a different workout`, and `errors.Is(err, errRemovedTimestamp)` is false. Both message types behave the same.
  - **`FuzzAccept` fails on it.** I added the 83-byte input as a seed in the scratch copy:

    ```
    --- FAIL: FuzzAccept/qa4-compressed-removed
        accept_test.go:472: internal refusal: ingest: internal: strip: the stripped file is a different workout
    ```

    The input is kept at `…/scratchpad/qa-r4/qa4-compressed-removed` (Go's fuzz format) and `….fit` (raw bytes). It is hand-built in test code, so it is synthetic.
  - **End to end** (`newApp` probe): `422`, `{"error":"internal","message":"This file could not be read."}`, an ERROR `fit rejected` line, 0 rows, `private/` and `tmp/` empty. For comparison, `empty-complete-seq` gives `422 unreadable` and WARN.
  - **Neither deterministic gate covers it.** The sweep probe has no kept compressed-timestamp message in the workout. The 30-minute fuzz run did not reach it either.
- **Consequence:**
  - Privacy is unaffected: the upload fails closed.
  - The sharer sees the same sentence.
  - But the closed list does not match what the decoder does. A crafted 83-byte file produces an ERROR line on demand, up to 60 per hour per IP, and a later F-31 alert on ERROR lines would page the operator for nothing. `internal` stops meaning "a defect on our side".
  - D-29's property does not hold for the code as built.
- **Evidence that would close it:**
  - ADR-023 item 8a says that a removed data message moves the clock through field 253 **or through a compressed-timestamp header**. The classifier then adds `|| rec.header&proto.MesgCompressedHeaderMask != 0` for removed messages.
  - `TestRemovedTimestampDependency` gets the case "removed compressed message without 253, then compressed kept → true".
  - `TestAcceptRemovedTimestamp` gets the variant through `Accept` → `unreadable`.
  - The input is committed under `testdata/fuzz/FuzzAccept/`.
  - The existing case "the compressed message itself removed → false" stays false.

- **ID:** QA-09-r4-2
- **Severity / kind:** Minor / risk (test cost)
- **Route:** architect
- **Where:**
  - design `TestFramingSweep` row ("part 2 ≤ 20 s … If part 2 is slower, it uses the reduced value set");
  - `src/server/internal/ingest/frame_test.go:424-534`.
- **Relates to:** D-29; round-8 row 7; engineer deviation 1.
- **Evidence:**
  - Part 2 already uses the reduced value set and still takes 31.5 to 37.3 s in my three runs. The engineer measured about 22 s.
  - The design has no second fallback.
  - `TestFramingSweep` alone is now about 40 s of the 76 s that `ingest` takes in `make ci` (round 3: 24.6 s for all packages).
  - It is not run under `-race` in CI. Under `-race` it would be several minutes.
- **Consequence:** a budget that the code does not meet and that no one enforces. CI time grows with every probe extension.
- **Evidence that would close it:** the design states the budget the sweep actually needs, or a cheaper part 2 (for example, parallel subtests, or the reduced set for definition payload bytes too), with the measured figure.

- **ID:** QA-09-r4-3
- **Severity / kind:** Minor / risk (a false finding in the fuzz oracle)
- **Route:** backend (the architect confirms the rule)
- **Where:**
  - `src/server/internal/ingest/accept_test.go:136-146` (`checkIdempotent`), called from `FuzzAccept` at `:478`;
  - the design's `FuzzAccept` row (P3 allows a load-dependent `limit`/`time` for the upload, but says nothing for the idempotence call).
- **Relates to:** D-29 P3; round-8 row 10 ("no failure of any class").
- **Evidence:**
  - One of my `-race` runs failed while the machine was loaded by 9 parallel mutant runs: `accept_test.go:478: integrity: Accept(stored): ingest: limit (time): time limit`.
  - `FuzzAccept`'s parser has `Timeout: 1 s`. The target accepts a `limit`/`time` from the first `Accept`, but `checkIdempotent`'s second `Accept` treats the same load-dependent result as an **integrity** failure.
  - The sweep re-runs such results (`frame_test.go:379-385`); `FuzzAccept` does not.
  - It did not recur in 4 more race runs.
- **Consequence:** a long fuzz run on a busy machine can stop with a false "integrity" finding, which by the convergence rule starts a fix round.
- **Evidence that would close it:** `checkIdempotent`, used from `FuzzAccept`, retries once on `canceled` or `limit`/`time` and fails only if that repeats, like part 2 of the sweep.

- **ID:** QA-09-r4-4
- **Severity / kind:** Minor / traceability
- **Route:** architect (AC table), backend (end-to-end case)
- **Where:**
  - `architect/F-09-upload-api-and-storage.md` (still v1.10): the header ("13 ACs") and section "Acceptance criteria to verification" (rows 1–13 only);
  - `src/server/cmd/workout-explorer/upload_test.go` (`TestUploadRejectedLeavesNothing`).
- **Relates to:** AC 14.
- **Evidence:**
  - The feature has 14 ACs, but the design maps 13. AC 14 is mentioned only inside the AC 7 row, as "outside AC 7's stored cases".
  - AC 14 is covered by composition:
    - `TestAcceptEmptySequence`: `unreadable`, the sentence, no strip hook, a zero `Accepted`;
    - `TestParseErrorMapping`: `422`;
    - `TestUploadFailures`: a parser error means no `Create` and no insert.
  - No end-to-end test uploads the empty-sequence file through `newApp` and checks "no row or files", which `TestUploadRejectedLeavesNothing` does for AC 11. I verified it by hand (Coverage).
- **Consequence:** a reader of the design cannot see what verifies AC 14, and a regression in the wiring would be caught only by composition.
- **Evidence that would close it:**
  - an AC 14 row in the design's table;
  - the `empty-complete-seq` bytes added to `TestUploadRejectedLeavesNothing` (or a sibling), expecting `422`, `unreadable`, "This file could not be read.", 0 rows, `private/` and `tmp/` empty, and one WARN line.

- **ID:** QA-09-r4-5
- **Severity / kind:** Minor / documentation
- **Route:** backend
- **Where:**
  - `src/server/internal/ingest/accept.go:20-23`: the `Accept` doc comment still says every failure is the upload's rejection "or Kind internal". It does not name `unreadable`, which the design's public surface (v1.9) adds.
  - `src/server/internal/ingest/frame.go:14-15`: "refuses only where Parse refuses too (D-24)". D-24 struck that sentence in v1.9.
  - `src/server/testdata/corpus/MEASUREMENTS.md` round 4: still holds `AcceptDefinitions/fields=1` at 176.5 MB/op. Since the round-7 iterator it is 283.2 MB/op and 0.83 s.
- **Relates to:** D-24, D-26; the design's `BenchmarkAcceptDefinitions` row ("ns/op and B/op go into `MEASUREMENTS.md`"); engineer deviation 12.
- **Consequence:** the comments contradict the normative rule they cite. The recorded B/op is 60 % below the current figure.

## Engineer's round-8 claims and deviations

| Claim or deviation | Result |
| --- | --- |
| Rows 1–10 built | Confirmed by reading and by the tests above. `httpapi`, `lifecycle` and `log.go` have no production change in round 8 (row 1). `TestAcceptFailsClosed` is byte-identical to round 3 (row 4) |
| `make ci` green; `make corpus` 52 ok, 0 diff | Reproduced |
| 6/6 real files accepted, stored bytes unchanged from round 7 | Reproduced: 6/6, and the hashes equal the engineer's round-7 snapshot. The aggregates equal rounds 1–3 |
| Sweep part 1 40,290 / 0; part 2 29,424 / 0, about 22 s | Counts reproduced. Time not reproduced: 31 to 37 s here (QA-09-r4-2) |
| m12–m18 each fail | Reproduced (table above), plus m16b and m19 |
| 30-minute `FuzzAccept`, 23.7M execs, no failure | The log in `…/scratchpad/r8/fuzz.log` confirms it: 23,710,465 execs, PASS, 1 of 603 status lines at 0 execs/s. The baseline was 960 cached entries, not a fresh cache (allowed for the engineer). My fresh 5-minute run is green. It did not find QA-09-r4-1 |
| 1. Part 2 over budget, reduced value set | **Accepted** as reported. The budget question goes to the architect (QA-09-r4-2) |
| 2. `canceled` before U1 in `Parser.strip` | **Accepted.** This is the order in the design ("a canceled context first, then U1") and in ADR-018 rev. 5 item 2. `TestAcceptEmptySequence` checks it |
| 3. U2 skips `emptyComplete` spans | **Accepted, but the skip is dead code.** Such a span has `records` = 0, so the loop does not run, and `strip` refuses U1 before U2 can be reached. Preference: remove it or say so in the comment |
| 4. Two U1 tests (hand-built span with `records` = 0; probe mutant) | **Accepted.** `TestStripEmptyHandBuilt` is design case (viii) as its own function. It also proves that `stop` is not called, which is the only test that kills m16b |
| 5., 6. Extra `crossCheckFile` calls in the sweep | **Accepted:** stronger than the design (part 2 also cross-checks the upload) |
| 7. `sweepTB.Failed()` = any recorded message | **Accepted:** the correct meaning for a double whose failures are values |
| 8. A panic is classed `robustness` by matching the cause text "panic" | **Accepted.** `ingest.go:70` builds `errors.New("panic")` on every recovery, so the text is the only handle. Preference: an unexported sentinel would survive a rewording |
| 9. New helpers, `accept_v19_test.go` | Accepted |
| 10. `TestAcceptRemovedTimestamp` rewritten; `checkStored` gets the header-only skip | **Accepted.** (a)–(c) are as in the design. The skip is row 5's `checkStoredBytes`, and `TestCheckStoredBytes` covers both directions |
| 11. No golden changed | Confirmed: `make corpus` 0 diff; against round 3 only `ride-30min-size-short` is new (round 5) |
| 12. `MEASUREMENTS.md` and corpus README not updated | **Accepted for round 8:** no round-8 row asks for them, and the README's `identifying-data` row is fixed. The stale benchmark figure is QA-09-r4-5 |
| Not listed: part 2 retries `canceled` and `limit`/`time` up to twice; the design says once | Accepted (preference) |

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-09-r3-1 (header data size ends inside a record → `internal`) | **fixed** | D-21: `stripSeq` frames complete spans by `records`, with the loop-end check (`strip.go:46-125`). `ride-30min-size-short.fit` is in the corpus: `TestUploadFraming` stores it byte-identical to `ride-30min.fit` with `data_end_s` NULL. `TestAcceptDataSizeSweep` is green in 3.7 s. `crasher-d0ededf3c3e842f3` (SHA-256 equal to my round-3 input) and `overrun-k5` are committed seeds and pass |
| QA-09-r3-2 (fuzz run spent most of its time minimizing) | **fixed** | `-fuzzminimizetime 5s` is in ADR-004 item 9, the design and `accept_test.go:431`. My 5-minute run: 1 of 103 status lines at 0 execs/s (round 3: 93 of 111) |
| QA-09-r3-3 (corpus README row for `identifying-data.fit`) | **fixed** | `README.md:73` lists `ant_channel_id`, `ant_rx`/`ant_tx`, the segment device ID, leader IDs and strings, the three account keys and both tank sensors |
| r1–r3 condition: the user's FIT-viewer check per brand | **open** (user, F-32) | Not something QA can run. It is no longer a condition of this verdict: F-32's launch sweep owns it (design F-32 hand-off) |

## Questions

1. **For the architect (QA-09-r4-1): is a removed data message with a compressed-timestamp header part of U2, or a new cause U3?** Either answer settles it, once item 8a, audit X1 and the classifier say so and a deterministic case plus the committed seed back it. Before deciding, check whether any other decoder state that a removed message can change is still unlisted. I found none: the developer-data state comes from 206 and 207, which are never removed, and the accumulator is off.
2. **For the architect (QA-09-r4-2): what does part 2 of the sweep get?** A budget that matches the measurement, or a cheaper part 2. Either answer is fine, as long as it is written down.

## Coverage

**Acceptance criteria → code → tests (all passing in this round):**

| AC | Code | Tests |
| --- | --- | --- |
| 1 | `lifecycle.Upload`, `NewDraft`, `NewIdentity`, `store.InsertWorkout` (unchanged since r3) | `TestUploadEndToEnd`, `TestNewDraft`, `TestUploadCreated` |
| 2 | `EditLink`; no token in any log call | `TestUploadEndToEnd`, `TestUploadCreated`, `TestUploadLogKeys`, `TestUploadLogs` |
| 3 | `upload.go` steps 5, 6 | `TestUploadSizeCap` (a), (a2), (b), (c) |
| 4 | `clientaddr.go`, `ratelimit.go` | `TestRateLimit`, `TestClientPrefix`, `TestRateTableFull`, `TestLimiterMemory` |
| 5 | schema; limiter keys only | `TestNoRequestMetadataStored`, `TestSchema`, `TestUploadLogs` |
| 6 | `strip_table.go`, `frame.go`, `strip.go`, `accept.go` | `TestStripIdentifyingData` (16 blanked, 5 removed), `TestUploadIdentifyingData`, `TestStripTable`, `TestBlankFill`, `TestAcceptLogs`, `TestStripCorpus`; the sweep and `FuzzAccept` privacy checks (`checkStoredBytes`) |
| 7 | `stripSeq` by `records` for both statuses, the loop-end check, the unclosed form | `TestUploadFraming` (6 files, including `bad-basetype` and `size-short`), `TestStripFrames`, `TestStripRejectedRecord`, `TestStripOverlap`, `TestWalkRecords`, `TestAcceptDataSizeSweep`, `TestStripCorpus`, `TestFramingSweep`, `FuzzAccept` seeds |
| 8 | `upload.go` slots, `settings.go`, `wiring.go` | `TestInFlight`, `TestSettings`, `TestServeInvalidSetting`, `TestSettingsReachTheHandler` |
| 9 | `upload.go` steps 4–7 | `TestUploadShape`, `TestUploadUnreadable` |
| 10 | `lifecycle` step 6, `privatedir`, `Recover` | `TestUploadServerError`, `TestServerErrorMapping`, `TestUploadFailures`, `TestCreateFails`, `TestUploadInsertNotCancelled`, `TestUploadCancelDuringInsert`, `TestRecover`, `TestServeRecovers` |
| 11 | `httpapi.fail` (422) | `TestUploadRejectedLeavesNothing`, `TestParseErrorMapping`, `TestAcceptRejections` |
| 12 | no `public/`; Go serves no `/w/` | `TestUploadEndToEnd`, `TestNoViewPath`; the generic page is F-15's |
| 13 | `base62`, `NewIdentity` | `TestUploadPublicIDs`, `TestBase62` |
| 14 | `emptyComplete`, the U1 check in `strip` and `stripSeq`, the mapping in `Parser.strip`, `KindUnreadable` | `TestAcceptEmptySequence`, `TestStripEmptyHandBuilt`, `TestParseErrorMapping` (`unreadable` → 422 and the sentence), `TestUploadFailures`, `FuzzAccept/empty-complete-seq`, `TestFramingSweep` (209 U1 per part). **Manually verified end to end:** a `newApp` probe in the scratch copy uploaded the committed `empty-complete-seq` bytes and got `422`, `{"error":"unreadable","message":"This file could not be read."}`, 0 rows, `private/` and `tmp/` empty, one WARN `fit rejected` line (QA-09-r4-4 asks for this as a test) |

**Checked and found correct:**
- `KindUnreadable` has the sentence "This file could not be read.". Its log line is WARN without `panic` or `frames`. `RejectionJSON` is unchanged.
- `strip` checks U1 for every span before it writes a byte or calls `stop`. `stripSeq` guards with the same predicate.
- The order inside `Accept` is `canceled`, U1, strip, reparse `limit`/`time`, then U2 or `internal`. It matches the design and ADR-018 rev. 5.
- U2 is reached only after a reference mismatch. It is per span, reads only through `frame.go`, and is not called on the success path.
- `FuzzAccept` implements P1–P3 with a class prefix on every failure (`classTB`, `storedClass`). An unlisted `unreadable` cause fails as "internal refusal" (`accept_test.go:463-467`).
- The `sweepTB` overrides cover every failure method; a skip counts as a failure.
- `TestCorpus` and `TestStripCorpus` are strict and name the kind (ADR-004 rev. 10). For a real file the message holds the kind and an offset-only cause, no figure.
- **Git hygiene:**
  - No real-derived file is untracked and unignored.
  - The new seed `empty-complete-seq` is built from test code (`sweepLead` + `emptySeqProbe`), so it is synthetic.
  - No real-file figure is in `MEASUREMENTS.md`, the corpus README or this report.

**Not run, or partly run:**
- QA's own 30-minute `FuzzAccept` run (design v1.8/v1.9): not run, by instruction. A fresh 5-minute run stands in, as a sample.
- The full `ingest` package under `-race` (sweep included): not run. The round-8 tests ran under `-race` (see QA-09-r4-3).
- The server runs M(4)/M(16) and A on the cap twin: not repeated. `lifecycle`, `httpapi` and `store` are unchanged, and `AcceptAtCap` allocates as in round 3.
- The FIT-viewer inspection (user, F-32), and third-party FIT readers (none available here).
