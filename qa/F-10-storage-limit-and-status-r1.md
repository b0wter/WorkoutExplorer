# QA F-10 Storage limit and status endpoint - round 1

- **Date:** 2026-10-23
- **Verdict:** ready. All 4 ACs are traced to code and passing tests. AC 4 is partly deferred to F-15, and that is recorded in the F-10 design's Hand-offs. There are 3 Minor findings. The most important is r1-1: two deletes of the same workout at the same time release its bytes twice. It must be fixed before F-12 or F-21 call `Delete`.
- **Inputs:** `features/F-10-storage-limit-and-status.md`; UP-8; TD Hosting and operations (Storage limit) and Configuration; `architect/F-10-storage-limit-and-status.md` (S-1 to S-11); ADR-021, ADR-022; uncommitted working tree on `master` at `8fd4281` (the `src/server` diff plus the new files `storage.go`, `storage_test.go`, `status_test.go` and `cmd/.../storage_test.go`).

## Build and test results

- `make ci`: exit 0. Go tests, vet and the web build all passed. Bundle 45,582 / 60,000 bytes gzip, PASS.
- `go test -race -count=1 ./cmd/... ./internal/httpapi/... ./internal/lifecycle/... ./internal/privatedir/...`: all 4 packages `ok`.
- Probe (copy of `src/server` in the scratchpad, `qa-f10/server/cmd/workout-explorer/zz_probe_test.go`): one upload, then two `Service.Delete` calls on the same ID at the same time, 200 runs. The running total dropped below its value before the upload in 1 of 200 runs (57,496 to 0, clamped). A single delete gave 0 of 200.

## Findings

### Minor

- **ID:** QA-10-r1-1
- **Severity / kind:** Minor / defect (latent: `Delete` has no caller yet)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/lifecycle.go:155-167`, `src/server/internal/privatedir/privatedir.go` `Remove`
- **Relates to:** S-5/S-6 ("every uncertain case counts too much"), AC 2, I-4
- **Evidence:** The probe above. `DeleteWorkout` returns nil when the row is already gone. Both callers then run `Usage(folder)` before either one's `RemoveAll`, so both get the full size back and both call `Release`.
- **Consequence:** The total undercounts by one workout per race, until the next restart. That is the unsafe direction: it can let the disk go past the cap. Expiry (F-21) racing an owner delete (F-12) is the realistic trigger. Fix: release only when this call deleted the row (`RowsAffected`). An alternative is to rename the folder away first, so that only one remover measures it. Add a test with two concurrent deletes.

- **ID:** QA-10-r1-2
- **Severity / kind:** Minor / risk (undocumented behaviour)
- **Route:** architect
- **Where:** `src/server/internal/httpapi/upload.go` step 2 vs `fail` (`ErrStorageFull` branch)
- **Relates to:** S-9, ADR-021 item 3, UP-10
- **Evidence:** A refusal at step 0 does not count against the hourly limit. An upload refused by `Reserve` after its parse has already used a rate-limit attempt (step 2 counts every allowed attempt).
- **Consequence:** A sharer who loses the race at the cap is charged one of their hourly uploads for a file that was not stored. This is harmless at friends-only volume, but S-9 does not say it. Record it in S-9, or refund the attempt.

- **ID:** QA-10-r1-3
- **Severity / kind:** Minor / doc
- **Route:** architect
- **Where:** `architect/F-09-upload-api-and-storage.md:544` (log table), ADR-022 (port list)
- **Relates to:** deviations 6 and 8
- **Evidence:** The F-09 log table still lists `reason` as `busy`, `rate_limited`, `bad_request` or `too_large`, with no `storage_full`. The new INFO line `storage` (`used_bytes`, `limit_bytes`, `uploads_open`) is only in the F-10 doc. ADR-022 does not mention that `Remove` now returns the freed bytes.
- **Consequence:** The log reference is incomplete for the operator (F-31). Add both log lines to the log table and add a one-line note to ADR-022.

## Deviations (engineer's list), judged

1. `Delete` as the single path without a route: correct and required by I-4. See r1-1.
2. AC 4 handed to F-15: acceptable. It is recorded in the design's Hand-offs, and `TestStorageFullRefusesOnlyUploads` covers `Add` and `Delete` while the cap is reached.
3. Refusal after the parse: acceptable. It is documented in S-9, and the HTTP mapping is tested (`TestStorageFullFromUpload`). See r1-2 for the rate-limit side.
4. DB growth not tracked: acceptable. It is bounded (WAL checkpoint about 4 MB) and recorded in S-7. The probe shows about 33 KB of database and WAL growth per upload and delete, which is consistent with S-7.
5. Apparent size: acceptable (S-2).
6. and 8. Doc gaps: r1-3.
7. No `Retry-After`: acceptable. There is no known time, and the test asserts that the header is absent.
9. A minimum of 1 MB: acceptable. It is useful for tests.
10. No end-to-end test of two real uploads racing: acceptable. `TestStorageRace` covers `Reserve` under `-race`, and the handler mapping is covered separately.

## Previous findings

None (first round).

## Questions

- **Q1 (user, already in the `features/README.md` Waiting on column):** Is a default cap of 10 GB right? Settled by a yes, or by another number in the TD Configuration row. This does not block: the value is a setting.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 Cap just above usage: the next upload succeeds or is refused as documented, later uploads get 503 with the exact text | `Storage.Reserve`, `upload.go` step 0 and `fail`, `respond.go` `storage_full` | `TestStorageLimitEndToEnd`, `TestStorageGate`, `TestStorageFullFromUpload`, `TestStorage`, `TestStorageRace` |
| 2 Status false while full, true after a delete or expiry, no restart | `statusHandler`, `Service.Delete` and `Release` | `TestStatus`, `TestStorageLimitEndToEnd` (delete). Expiry is F-21, which uses the same `Delete` |
| 3 After a restart the total equals the size on disk | `newApp`: `Recover`, then `privatedir.Usage` | `TestStorageLimitEndToEnd` (twice, including crash leftovers), `TestUsageAndFreed` |
| 4 Publish works while paused | `Storage.Add` never refuses. Publish does not exist yet | `TestStorageFullRefusesOnlyUploads` (partial). Full test handed to F-15 |

What I checked and found correct:
- Only `POST /api/workouts` consults the gate. `/api/health` and `/api/status` answer while the cap is reached, and `Delete` and `Add` never refuse.
- Upload failure paths: `Create` failing, insert failing with the folder removed, insert failing with the folder left, and `WorkoutExists` failing all keep the total correct or high (see `TestUploadStorageFailures`).
- The total starts from 0 and the usage is measured after `Recover`, so crash leftovers never count.
- The setting is parsed strictly, and an invalid value exits with code 2 before `<data>` is created.
- The status body is exactly one boolean, with `no-store`, `HEAD` 200 and other methods 405.
- The step 0 refusal reads no body and does not count against the rate limit.

Not run: `make corpus` (no parser change).
