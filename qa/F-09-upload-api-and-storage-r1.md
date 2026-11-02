# QA F-09 Upload API and workout storage - round 1

- **Date:** 2026-10-16
- **Verdict:** **ready with conditions.** There is no Blocker or Major finding. Every one of the 13 acceptance criteria is traced to code and to a passing test. I mutated the key guarantees: the strip, the stored bytes, the first-parse release, the size cap, the rate limit, the in-flight cap and fail-closed stripping. Every one of those mutants is killed. The memory stop rule holds on my machine (A ≤ 247 MB, M(4) ≤ 677 MB). Conditions:
  1. **QA-09-r1-1 is fixed before a feature relies on "every row has its folder" (F-12, F-30).** A client that disconnects during the `INSERT` can leave a committed row whose private folder `lifecycle` has just deleted. The probe reproduced this.
  2. **The user's FIT-viewer check** (design row 15, F-32): run `parse -original` on one real file per brand and inspect the output in a FIT viewer. That step is the user's. My byte-level probe over the local real files passed (aggregates under Coverage).
- **Inputs:**
  - Feature: `features/F-09-upload-api-and-storage.md`, all 13 ACs and the Decisions of 16 Oct 2026.
  - Spec: Product Specification UP-1, UP-3, UP-7, UP-9, UP-10, SH-1, SH-2, SH-8, AC-1, AC-4, AP-10, and the Non-functional Security and Privacy lines.
  - Architecture: `architect/F-09-upload-api-and-storage.md` v1.3 (all sections, including both change lists and the stop conditions); ADR-017 rev. 3; ADR-019 rev. 1; ADR-020; ADR-021 rev. 2; ADR-022 rev. 1; ADR-023 rev. 2; `architect/overview.md` v1.18 (I-7, I-8, configuration rule, logging row, dependency rules).
  - F-08 follow-ups: `qa/F-08-fit-validation-and-recovery-r5.md`; `architect/F-08-fit-validation-and-recovery.md` v1.6 (change log 1.6, corpus row, section "Changes for the backend engineer (v1.6)").
  - State: base `8ab9085` plus the uncommitted working tree (41 modified, 28 untracked entries). The tree was the same at the end of the review.
  - QA changed nothing outside `qa/`. Mutants and probes ran through `go test -overlay`, with the replacement files in the scratchpad.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0 (21.5 s). gofmt clean, `go vet` clean, every Go package `ok`. The front-end build passed. Bundle 45,582 / 60,000 B gzip, PASS |
| `go test -count=1 -race ./...` (in `src/server`) | exit 0, all 12 packages `ok` (2 m 23 s; `internal/ingest` 139 s) |
| `make corpus` | `corpus: 50 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown` (44 synthetic, 6 local real). `swim-bike-distance-fall.fit` ok, bin 24,580 B |
| `CGO_ENABLED=0 go build ./cmd/workout-explorer` | ok. `build/workout-explorer` is 18,577,298 B, which matches ADR-019 rev. 1's 18.6 MB |
| `go test ./internal/ingest -run '^$' -bench AcceptAtCap -benchtime 5x -count 3 -benchmem` | 2.366 s, 2.430 s and 2.469 s per op, within 4.5 s. 885.8 MB/op; 467,691 to 467,862 allocs/op |
| A: `/usr/bin/time -v build/workout-explorer parse -original <scratch> <scratch>/cap-500k.fit`, all-caps twin (17,360,175 B from `corpusgen -cap-file`), 3 runs, `GOGC` and `GOMEMLIMIT` unset | Max RSS 243,776 / 242,760 / 246,152 kB, within 400 MB. Wall clock 3.2 to 3.4 s. Exit 0 |
| M(4): `serve -data <fresh scratch dir>` with the default settings; 4 × `curl -F file=@cap-500k.fit` in parallel; SIGINT; 3 runs | every answer `201` (12 of 12). Max RSS 652,360 / 619,628 / 676,956 kB, within the 1,000 MB F-31 trigger |
| `go mod tidy -diff` | **not tidy**: `modernc.org/sqlite` is a direct import but is listed `// indirect` (QA-09-r1-6) |
| `TestAcceptReleasesFirstParse -v`, 3 runs | excess −18,624 / −400 / −18,432 B, limit 2,348,407 B (see QA-09-r1-3) |

### Mutation probes (`go test -overlay`, nothing in `src/` changed)

| Guarantee | Mutant | Killed by |
| --- | --- | --- |
| First parse released (D-17) | `w = workout.Workout{}` removed in `run` | `TestAcceptReleasesFirstParse` (excess 3,358,480 > 2,348,407) |
| | `outcome` keeps `*listener` again | `TestAcceptReleasesFirstParse` (excess 4,245,344) |
| Blanking (UP-9 (1)) | `blankFields` not applied | `TestStripIdentifyingData`; `TestUploadIdentifyingData` (markers in `original.fit`) |
| Removal (UP-9 (2) to (4)) | `removeTable` check off | `TestStripIdentifyingData`; `TestUploadIdentifyingData` |
| Transmission types kept (OQ-46) | `ant_transmission_type` added to the blank table | `TestStripIdentifyingData`, `TestStripTable` |
| Only stripped bytes stored (I-8) | `Files.Create` gets `upload` | `TestUploadStoresStripped`; `TestUploadIdentifyingData` |
| Fail closed (ADR-023 item 8) | reference check removed | `TestAcceptFailsClosed` (2 of 3 hooks) |
| Unclosed form for truncated files (OQ-49) | truncated sequence written closed | `TestStripFrames`, `TestStripCorpus` |
| Size cap | `Content-Length` check removed | `TestUploadSizeCap` (a) |
| | `>` → `>=`; `> Max+1` | (c); (a2) and (b) |
| Rate limit | 60 → 61 | `TestRateLimit` (3 subtests) |
| | limiter skipped | `TestRateLimit` |
| | IPv6 /64 → /128 | `TestRateLimit`, `TestClientPrefix` |
| | first `X-Forwarded-For` entry; header trusted from any peer | `TestClientPrefix` |
| | full-table one-minute sweep removed | **survives** (QA-09-r1-2) |
| In-flight cap | N + 1 slots | `TestInFlight` |
| | busy attempts counted by the limiter | `TestInFlight` |
| No leftovers | staging not removed on error | `TestCreateFails`, `TestUploadServerError` |
| | folder not removed after an insert error | `TestUploadFailures` |
| | context not checked before the writes | `TestUploadFailures` |
| Log keys (I-7) | extra key on `upload refused` | `TestUploadLogs` |

Expected survivors that are not findings:
- `MaxBytesReader` removed alone, and `LimitReader` removed alone, each survive, because the other one plus the `buf.Len()` check covers every tested size. A probe with a 30 MB preamble before the part gets `400` after reading 4,096 bytes with either guard, because Go's multipart reader refuses it itself.
- `fsync` removed survives, because no test can observe it without crash injection.

## Findings

### Minor

- **ID:** QA-09-r1-1
- **Severity / kind:** Minor / defect
- **Route:** backend (fix); architect (design step 6 and ADR-022 item 4 say nothing about it)
- **Where:**
  - `src/server/internal/lifecycle/lifecycle.go:85-89`: `InsertWorkout(ctx, rec)` with the request context; on error the folder is removed.
  - `src/server/internal/store/store.go:84`: `ExecContext(ctx, …)`.
- **Relates to:** feature scope "Failure at any step leaves no row and no files behind"; ADR-022 item 4 ("the only state a crash can leave is a folder without a row"); AC 9, AC 10.
- **Evidence:**
  - `modernc.org/sqlite` v1.60.1 `stmt.exec` (`stmt.go:109-111`) replaces the result with `ctx.Err()` whenever the context ended before its deferred check. This includes a cancellation that arrives after the autocommit `INSERT` has finished.
  - Probe 1, `internal/store`: 20,000 inserts, each with a context cancelled 0 to 200 µs later. In 1,280 of them `InsertWorkout` returned `store: insert workout: context canceled` although the row was committed.
  - Probe 2, end to end with the real `store`, `privatedir` and `lifecycle`: a repository wrapper cancels the request context 0 to 100 µs into the insert. Of 2,000 uploads, 1,111 rows were committed and **366 of them have no `private/{id}` folder**. `lifecycle` logged `upload failed stage=store` and removed the folder.
- **Consequence:**
  - A client that drops its connection during the insert window leaves a row with no files.
  - `Recover` sweeps only folders without rows, so the row stays until expiry (F-21 is not built yet).
  - Every later reader that goes from row to files will meet a missing `original.fit` or `draft.bin`: F-12, F-15 publish, and F-30 `regenerate`, which iterates every row.
  - The operator also sees an ERROR line for what was a client disconnect.
  - The window is a few milliseconds per upload, so this is rare. But the code reproducibly breaks the invariant the design is built on, and the fix is small: for example, insert with `context.WithoutCancel(ctx)` after the step-4 check, or check whether the row exists before removing the folder.

- **ID:** QA-09-r1-2
- **Severity / kind:** Minor / test (unreported deviation)
- **Route:** backend
- **Where:** `src/server/internal/httpapi/upload_test.go:480-516` (`TestRateTableFull`); `ratelimit.go:54-56`
- **Relates to:** design section Tests, `TestRateTableFull` ("once a minute has passed (fake clock), one more sweep and at most one more WARN"); ADR-021 rev. 2 item 8
- **Evidence:**
  - The test advances the clock by **2 hours**, which also triggers the 10-minute periodic sweep. It never advances by more than 1 minute and less than 10.
  - Mutant: the full-table sweep at `ratelimit.go:55` removed → `TestRateTableFull` passes.
  - The hand-off did not mention the change.
- **Consequence:** the rev. 2 behaviour (a full table is swept again after 1 minute, not 10) has no test that can fail. Use 61 s with expired keys, or a clock advanced past the window but under 10 minutes since the last sweep, and assert at most one more WARN line.

- **ID:** QA-09-r1-3
- **Severity / kind:** Minor / test (passes partly for the wrong reason)
- **Route:** backend
- **Where:** `src/server/internal/ingest/accept_test.go:230-255` (`TestAcceptReleasesFirstParse`)
- **Relates to:** D-17; ADR-017 rev. 3 item 3; design rule "heap at the hook minus the baseline is at most `len(stripped) + 1 MiB`, with the upload already in memory"
- **Evidence:**
  - `upload` is not used after the second `Accept`. Inside `Accept` the strip is its last use. So the GC frees the 1,299,831-byte upload by the time the hook runs, while the stripped copy of the same size is live. They cancel out: the measured excess is −18 KB to 0, not the +1.3 MB of the stripped copy.
  - With `runtime.KeepAlive(upload)` added after the call (overlay), the excess is **+1,283,232 B**, as the design expects.
  - As built, the test tolerates up to about 2.35 MB of first-parse retention instead of 1 MiB. Both round-1 retention paths are still caught (3.36 MB and 4.25 MB), but by margins of only 1.0 MB and 1.9 MB.
- **Consequence:** a partial regression between 1 MiB and about 2.3 MB passes, for example keeping one stream or the lap and session slices of the first workout. Add `runtime.KeepAlive(upload)` after the measured call. The server path keeps the upload alive (`httpapi` holds `buf`), so the test should too.

- **ID:** QA-09-r1-4
- **Severity / kind:** Minor / risk
- **Route:** architect (memory model, F-31 hand-off)
- **Where:**
  - `src/server/internal/httpapi/ratelimit.go:11-27`: per key, a `[]time.Time` of up to 60 entries, each 24 bytes.
  - `upload.go:29`: `maxRateKeys = 100_000`.
- **Relates to:** ADR-021 item 8; ADR-017 rev. 3 item 4 (M(N)); F-31 `MemoryMax`; OQ-51
- **Evidence:**
  - Probe: a limiter filled to 100,000 keys × 60 attempts holds **184.7 MB** of heap.
  - Filling it takes 6 million requests in an hour from 100,000 distinct prefixes: about 1,700 per second.
  - IPv6 counts per /64, so one /48 already provides 65,536 prefixes.
  - Each such request costs one in-flight slot only briefly, because a wrong content type is refused at step 4 after it has been counted.
- **Consequence:**
  - M(N) ≤ 2·A + (N − 2)·B leaves out up to about 185 MB, about a quarter of the measured M(4).
  - F-31 sizing from M(4) alone would be short by that amount on a 1 to 2 GB VPS.
  - Either add the limiter's bound to the model and to the F-31 hand-off, or shrink it, for example with `int32` seconds since process start and a cap smaller than 100,000.

- **ID:** QA-09-r1-5
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:** `architect/ADR-022-private-workout-files-staging-and-recovery.md` item 5; F-09 design section Logging; `lifecycle.go:149`
- **Relates to:** ADR-022 item 5
- **Evidence:**
  - ADR-022 says entries that are not internal IDs are "logged once as a WARN", and in the next sentence "Logged as one INFO line with counts only".
  - The F-09 design lists only the INFO `recovery` line with `unknown_entries`, and the code does that.
- **Consequence:** the ADR contradicts itself and the design. Remove the WARN sentence, or add the WARN line to the design.

- **ID:** QA-09-r1-6
- **Severity / kind:** Minor / preference (hygiene)
- **Route:** backend
- **Where:** `src/server/go.mod:21`; `internal/httpapi/upload.go:100-105`; `internal/store/store.go:31`; `cmd/workout-explorer/main.go` (`defer closeApp()`)
- **Relates to:** overview section 5 module rule; design section Logging
- **Evidence:**
  - `go mod tidy -diff` moves `modernc.org/sqlite v1.60.1` from `// indirect` to the direct `require` block.
  - `httpapi` writes `upload failed` with `stage=deadline`. That line and stage are not in the design's Logging table: there `upload failed` belongs to `lifecycle`, with five stages. The error carries only the local address (`net.conn.SetReadDeadline` puts `laddr` in the `OpError`), so this is not an I-7 leak.
  - `f.Close()` after creating the database file and the deferred `closeApp()` drop their errors.
- **Consequence:**
  - `go.mod` misstates a direct dependency.
  - The upload path has one log line outside the allow-list that `TestUploadLogs` cannot see, because no test reaches it.
  - A failed close at shutdown goes unnoticed.

## Previous findings

| Finding | Status | Evidence |
| --- | --- | --- |
| QA-08-r5-1 (test comment, `f08_v14_test.go:134-135`) | **fixed** | The comment now says that holding before the fill turns the 50 into 100 and gives `[100 100 125 150 175 200]`. That is the H3 mutant's output recorded in r5 |
| QA-08-r5-2 (lap total in `swim-bike-distance-fall.fit`, F-08 v1.6) | **fixed** | `corpusgen` `lapMesg` applies the session's `sessionDist` override to a lap with exactly the session's span, `avg_speed` = override / timer. Golden: `laps[k].totals` = `sessions[k].totals` for distance (599, 2,946) and avg speed (1.000, 4.918). Only that file's report golden changed (lap keys). Its data-file golden changed in `byte_length`, `metadata_length` and the stream `byte_offset`s (+4 B), as the design's round-2 allowance says. `make corpus` shows 0 diff. The field was not renamed; the design allowed that |

Engineer claims, checked:

| Claim | Result |
| --- | --- |
| Round 1: the 14 deviations accepted in v1.3 are now as designed | Confirmed, each one: <br>• `TestUploadSizeCap` (a)/(a2) <br>• `TestAcceptFailsClosed` through `stripHook` (2 hooks hit the reference mismatch, 1 the rejected reparse; the mutant shows both paths are covered) <br>• `TestStripHandBuilt` and `TestFillInvalid` <br>• 2-byte ANT markers <br>• `TestStripCorpus` special cases <br>• `id` not-null <br>• `TestNewerSchemaRefused` <br>• the three test seams (all nil or default in production; the timeout variables are restored, no `t.Parallel`) <br>• the four settings tests <br>• derived offsets in the swim-bike data-file golden <br>• the `identifying-data.fit` order and zone messages <br>• the one-minute sweep in code (test gap: QA-09-r1-2) <br>• `ServerError`, `bytes` = upload size, `NewHandler` without `Uploads`, `BlankedFields` counting changed fields, the `storage_full` comment, the escaped DSN (`TestOpenPathNeedsEscaping`) |
| Round 1 stop: RSS 413 to 467 MB | Explained in ADR-017 rev. 3. Both retention paths are gone; each one, re-added as a mutant, fails the new test |
| Round 2: first parse released before the strip, `outcome.l` removed | Confirmed (`ingest.go:84-96`, `log.go:22-45`) |
| `TestAcceptReleasesFirstParse` fails on the round-1 state | Confirmed for each retention path separately. Its margin is weaker than designed (QA-09-r1-3) |
| One `strip(data, spans, stop)` loop | Confirmed: `strip.go:226-245`; `accept.go` has no loop; `TestStripStop` |
| 2-byte markers searched only in `original.fit` | Confirmed (`cmd/workout-explorer/upload_test.go:314-322`). Markers of 4 bytes or more are searched in every data file, and markers of 5 or more characters in the log |
| A 253 to 262 MB, `Accept` 2.46 to 2.49 s | Reproduced as 242,760 to 246,152 kB and 2.37 to 2.47 s |
| M(4) 707 to 861 MB, M(16) 1,120 to 1,237 MB | M(4) reproduced lower: 619,628 to 676,956 kB, all 201. M(16) not run (Coverage) |
| `make ci`, `make corpus` green (50 ok) | Confirmed |
| `take()` called twice; `reference(w)` in `run` | Both harmless. `take` copies the same values again and is needed for a panic inside the walk; `reference` in `run` is what D-17 prescribes |

## Questions

1. **For the architect, then the user: does UP-9 (1) cover ANT device numbers outside `device_info` and the profile messages?**
   - The spec says "every serial number and every ANT+ device number or ANT ID … of every sensor listed in the file". The feature file narrows it to "(`device_info` and profile messages)".
   - The pinned profile also has:
     - `ant_channel_id` (82), field 2 `device_number`;
     - `ant_rx`/`ant_tx` (80, 81), which carry raw ANT payloads;
     - `segment_id` (148), field 5 `device_id`;
     - `segment_file` (151), field 9 `leader_activity_id` (the leaders' activity IDs, akin to the removed message 149).
   - None is blanked or removed. `TestStripTable`'s name rule (`serial_number`, `ant_device_number`, `*_ant_id`) cannot catch `device_number` or `device_id`.
   - None of these messages appears in any of the 6 local real files.
   - Either answer settles it:
     - "The feature file's narrowing stands" (then record it in ADR-023 Consequences);
     - or "blank 82/2 (and decide 80, 81, 148/5, 151/9)", with `device_number` added to the `TestStripTable` rule.

## Coverage

**Acceptance criteria → code → test (all tests passing):**

| AC | Code | Tests |
| --- | --- | --- |
| 1 | `lifecycle.Upload`, `NewDraft`, `NewIdentity`, `store.InsertWorkout`, `httpapi` step 9 | `TestUploadEndToEnd`, `TestNewDraft`, `TestUploadCreated` |
| 2 | `EditLink`; no token in any log call (grep of all log calls) | `TestUploadEndToEnd` (log, every data file, `view_url`), `TestUploadCreated`, `TestUploadLogKeys`, `TestUploadLogs` |
| 3 | `upload.go:114-143` | `TestUploadSizeCap` (a), (a2), (b), (c); mutants killed |
| 4 | `clientaddr.go`, `ratelimit.go`, `upload.go:86-95` | `TestRateLimit`, `TestClientPrefix`; mutants killed |
| 5 | schema without request columns; no `RemoteAddr`, user agent or file name used outside the limiter key (grep) | `TestNoRequestMetadataStored`, `TestSchema`, `TestUploadLogs` |
| 6 | `strip.go`, `strip_table.go`, `accept.go` | `TestStripIdentifyingData`, `TestUploadIdentifyingData`, `TestAcceptLogs`, `TestStripCorpus`, `TestStripTable`; mutants killed |
| 7 | `stripSeq` (complete and truncated forms), `walk` spans | `TestUploadFraming`, `TestStripFrames`, `TestStripCorpus`; unclosed-form mutant killed |
| 8 | `upload.go:76-84`, `settings.go`, `wiring.go` | `TestInFlight`, `TestSettings`, `TestServeInvalidSetting`, `TestSettingsReachTheHandler` |
| 9 | `readFailure`, steps 4 to 7 | `TestUploadShape`, `TestUploadUnreadable` |
| 10 | `lifecycle.fail`, `privatedir.Create` cleanup, `httpapi.fail` | `TestUploadServerError` (not root, so not skipped), `TestServerErrorMapping`, `TestUploadFailures`, `TestCreateFails`. A client disconnect during the insert breaks "no row" (QA-09-r1-1) |
| 11 | `httpapi.fail` (422 with `Message()`) | `TestUploadRejectedLeavesNothing`, `TestParseErrorMapping`, `TestAcceptRejections` |
| 12 | no `public/` from F-09; Go serves no `/w/` | `TestUploadEndToEnd`, `TestNoViewPath`. The generic page itself is F-15's AC 1 and AC 6, as the design says |
| 13 | `base62`, `NewIdentity` | `TestUploadPublicIDs` (10,000 with the real `crypto/rand`), `TestBase62` |

**Spec and architecture checked and found correct:**
- UP-10 texts are exact (`respond.go`).
- Tokens are 128 bits from `crypto/rand`; only their SHA-256 is stored; the token appears only in the fragment.
- `Cache-Control: no-store` on every upload response.
- Data directory modes: data 0750; `private/` and `tmp/` 0700; files 0600; the database 0600.
- Staging, `fsync` and rename; the row is inserted last; recovery runs before listening.
- Package imports match overview section 5. `lifecycle` imports neither `database/sql` nor `os`. `modernc.org/sqlite` is imported only by `internal/store` and the `cmd` end-to-end test (ADR-019 rev. 1). The only environment read is `settings.go`. No `SetGCPercent`, `SetMemoryLimit` or `GOMEMLIMIT` in code.
- Git hygiene: nothing under `corpus/real/`, `expected/real/` or `build/`, and no `.br`, is untracked-and-unignored. The committed `identifying-data` goldens and `draft.bin` golden hold none of the removed markers. No real-file figure is in `MEASUREMENTS.md`, the corpus README or the architecture docs.

**Local real files (aggregates only, ADR-004 item 7):** 6 files from 3 manufacturers.
- `Accept` succeeds on all of them, and the `checkStored` decoder check passes (in `TestCorpus`).
- In total 50 fields were blanked and 1 message was removed.
- My probe compared the uploads and the stored files record by record: no record differs outside the blank table and no record is missing or added.

**Not run, or partly run:**
- `FuzzAccept` beyond its seeds. A fuzz run would write failing inputs into `src/server/internal/ingest/testdata/fuzz/`, inside `src/`.
- M(16). The scratch filesystem's quota was almost exhausted by a 4.2 GB file belonging to another session, and 16 stored uploads at the caps need about 0.5 GB. M(16) has no stop rule, and the engineer's figure stands as information for F-31.
- The FIT-viewer inspection of `parse -original` output per brand (condition 2, user).
- `go mod tidy -diff` downloaded test-only modules into the Go module cache. That was a network call to the module proxy; no repository file changed.
