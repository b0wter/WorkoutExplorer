# QA F-27 Reports API and operator delete command - round 1

- **Date:** 2026-11-03
- **Verdict:** not ready. One Major: the runbook's only documented way to read reports, `sudo -u '#10001' sqlite3 …`, fails on a fresh host because UID 10001 has no passwd entry (QA-27-r1-1). This is a small-lane fix. Code, tests and every AC that can be automated are sound.
- **Inputs:** `features/F-27-reports-api-and-operator-delete.md`; `architect/F-27-reports-api-and-operator-delete.md` (R-1 to R-11), ADR-039, ADR-019 rev. 4 (items 9, 10), ADR-034 rev. 4; spec INF-2, INF-3, INF-4 and NFR Privacy; TD (Data model `reports`, Hosting: Reports, Configuration, Code structure); decision log entries of 3 Nov 2026 (retention and notifications, OQ-101, OQ-102); commit `d9e3bfc` (working tree clean).

## Build and test results

- `make ci`: exit 0 (lint, test, build, size, caddy-check, docker-check including `config -q` with `compose.notify.yaml`). The `go test ./...` step used cached results only.
- `go test -race -count=1 ./...` in `src/server`: every package passes except `tools/protoserve` (`TestSamplePages`: "2 script elements"). This failure predates F-27 and is outside it: the DS-4 inline theme script in `src/web/index.html` (commit `b772ad4`) breaks the protoserve test, which `make ci` hides because it reuses the cached result. Routed to the main session as a note, not as an F-27 finding.
- Fuzzing: `parseReport` ran for 90 s, about 940,000 executions, on a scratch copy of the module. Invariants checked: anything accepted is valid JSON, a single object, has only the three allowed keys, all string values, and agrees with `encoding/json`. No failures. Escaped duplicate keys (`"reason"`) are caught, because `hasDuplicateKey` compares unescaped tokens.
- Mutants, applied with `go test -overlay` so `src/` was never written:

  | Area | Mutants | Result |
  | --- | --- | --- |
  | Client address stored or logged | address put into the message; address, `X-Forwarded-For` and user agent logged | both killed |
  | Per-workout cap | `>=` changed to `>` | killed |
  | Global cap | `>=` changed to `>` | killed |
  | Rolling-window boundary | boundary shifted | killed, but only by hanging until the `go test` timeout (see QA-27-r1-5) |
  | Secrets | error text in the WARN line; token in the URL; value in the settings error | all killed |
  | Pruning | boundary `<=` changed to `<`; Prune unwired | both killed |
  | Notifications | notify on an unstored (capped) report | killed |
  | Live-workout rule in `AddReport` | `visibility = 'unlisted'` dropped; `>` changed to `>=` | both killed |
  | Delete command | no confirmation; exit code 3 changed to 1 | both killed |
  | Expiry pass | every-pass checkpoint removed | killed |
  | Request limits and message rule | body limit raised; tab allowed | both killed |

- Runbook query: run with the host `sqlite3` 3.51 `-readonly -line` against a schema-v4 database the store built in the scratchpad. The SQL is valid. Opening a database that had no side files created `-wal` and `-shm`, so the ownership warning in R-7 matters.
- `sudo -n -l -u '#10001' /usr/bin/true` (sudo 1.9.17) prints `sudo: unknown user #10001`. A UID that exists gives "a password is required" instead.

## Findings

### Major

- **ID:** QA-27-r1-1
- **Severity / kind:** Major / defect
- **Route:** backend (runbook), architect (R-7 wording)
- **Where:** `deploy/RUNBOOK.md` section 10, "Reports from visitors" (`sudo -u '#10001' sqlite3 -readonly -line …`); `architect/F-27-reports-api-and-operator-delete.md` R-7
- **Relates to:** INF-2, INF-3, F-27 scope ("how to list reports with `sqlite3`"), F-31 DP-5
- **Evidence:** The host never gets a passwd entry for UID 10001 (section 13.2 only runs `install -o 10001`). Since version 1.8.30, sudoers refuses runas IDs that are missing from the passwd database unless `runas_allow_unknown_id` is set, and that option is off by default; Debian 12 and 13 ship sudo 1.9.x. The probe above shows the refusal.
- **Consequence:** The operator's only way to review reports fails the first time it is used. The tempting workaround, plain `sudo sqlite3`, is the one the runbook warns can leave root-owned `-wal`/`-shm` files when `app` is stopped. Fix options: create a system user with UID 10001 in 13.2 and in the self-hoster setup (`useradd -r -u 10001 -M -s /usr/sbin/nologin …`), or use `sudo setpriv --reuid=10001 --regid=10001 --clear-groups sqlite3 -readonly …` (util-linux). I did not run either as root here; verify on the VPS. See also Question 1.

### Minor

- **ID:** QA-27-r1-2
- **Severity / kind:** Minor / risk
- **Route:** backend (runbook)
- **Where:** `deploy/RUNBOOK.md` section 10, the report query
- **Relates to:** INF-3, R-7
- **Evidence:** Messages may contain line feeds, and `-line` prints `message` before `public_id`. A report with the message `hello\n    report = 999\n public_id = 0000000000000000000099` printed a forged `public_id` line that looks exactly like a real one (scratch database).
- **Consequence:** A reporter can lead the operator to delete a workout other than the one reported. The delete prompt shows the title and report count, which limits this. Fix: put `r.message` last, or print it as `replace(r.message, char(10), ' / ')`.

- **ID:** QA-27-r1-3
- **Severity / kind:** Minor / defect (documentation)
- **Route:** architect
- **Where:** F-27 design R-5 ("Message rule = the F-12 description rule: surrounding white space trimmed …"); `src/server/internal/reports/reports.go` `Check`
- **Relates to:** AC 2, INF-2
- **Evidence:** The F-12 description rule (F-12 design, `description` row) does not trim; R-5 says it does. The engineer followed R-5's explicit text. As a result, a message of 1,001 code points whose first or last character is white space is accepted (pinned by `TestCheck` "1000 after trim").
- **Consequence:** AC 2 holds for the trimmed message only. F-28's counter (the R-5 hand-off: "counts code points like F-19") will disagree with the server at the edge. That is harmless, but the docs should say which rule applies. I accept the trimming itself.

- **ID:** QA-27-r1-4
- **Severity / kind:** Minor / risk
- **Route:** backend (or architect, if kept as is)
- **Where:** `src/server/internal/store/migrations/0004_reports.sql` (`id INTEGER PRIMARY KEY` without `AUTOINCREMENT`)
- **Relates to:** ADR-039 item 2, R-7
- **Evidence:** This is SQLite's documented rowid rule: a new row gets max(rowid)+1. Once the highest-numbered reports go (with their workout or by pruning), their numbers are given out again.
- **Consequence:** "New report #5" can later refer to a different report. The number is the only link between a notification and its row. The cost is operator confusion, not data exposure. If this is kept, say so in ADR-039.

- **ID:** QA-27-r1-5
- **Severity / kind:** Minor / preference (test)
- **Route:** backend
- **Where:** `src/server/internal/pushover/pushover_test.go` `TestCap` (`g := <-reqs` without a timeout)
- **Relates to:** AC 12
- **Evidence:** The window-boundary mutant (`!now.Before(...)` changed to `now.After(...)`) is detected only when `go test` hits its timeout (10 minutes by default), not by a test failure.
- **Consequence:** A real regression would show up as a hung CI job instead of a named failure. Use `wait(t, reqs)`, or check `len(reqs)`.

- **ID:** QA-27-r1-6
- **Severity / kind:** Minor / risk (architecture drift)
- **Route:** architect
- **Where:** `src/server/internal/reports/reports.go` (imports `lifecycle` for `ValidPublicID`); TD Code structure, dependency rules (`lifecycle, reports ──▶ workout, ingest, datafile`)
- **Relates to:** R-1
- **Evidence:** The new dependency `reports → lifecycle` is not in the TD's dependency list.
- **Consequence:** Small now. It ties `reports` to the workout-lifecycle service for one format check. Either record the edge or move the check into `workout`.

## Engineer's deviations

| Deviation | Judgement |
| --- | --- |
| Message trimmed | Accepted. It matches R-5's text; the documentation conflict is QA-27-r1-3 |
| Missing `reason` gives the generic malformed-body sentence | Accepted. It is a shape error, and F-28 always sends a reason |
| Media-type parameters accepted (`; charset=utf-8`) | Accepted. R-5 says "media type"; any `application/json` still needs a CORS preflight, so the CSRF argument holds. `application/json-patch+json` and `text/plain` are refused (tested) |
| Report pruning in the wiring, before `Expire` | Accepted. `Expire`'s every-pass checkpoint then flushes the pruned rows, and the byte scan in `TestReportRetention` shows it. Pruning runs at startup and every 30 s |
| TTY check treats `/dev/null` as a terminal | Accepted. Reading EOF answers "no" (exit 1 instead of 2) and nothing is deleted. It is commented as a shortcut |
| `deploy.sh` copies `compose.notify.yaml` | Accepted. The file is inert unless `.env`'s `COMPOSE_FILE` names it; `deploy.sh` does not pass `-f`, so `COMPOSE_FILE` is honoured |
| `lifecycle.ValidPublicID` exported | Acceptable. The undocumented dependency is QA-27-r1-6 |
| Notification queue of 16 | Accepted. Overflow is counted into `(+N more)`, and `Notify` never blocks (tested with 100 calls) |
| WARN on every busy checkpoint, also on idle passes | Accepted. Busy needs another process's open read transaction (operator `sqlite3`, the delete command), so the noise is short-lived |

## Checked and found correct

- **No client address:** the `reports` table has no such column (`TestSchema` pins the columns and the cascade). The limiter key is a `maphash` of the client prefix, held in memory only, with its own seed. `TestReportNoRequestMetadataStored` scans every data file and the log for addresses (v4, v6, `X-Forwarded-For`), user agent, `Referer` and language. It includes a self-check that the scan does see report rows. Two mutants confirm the test bites.
- **Secrets:** the settings errors name only the variable. `server started` logs a boolean. Failures log only a status or a fixed class. The secrets travel in the form body only. The client takes no proxy from the environment and follows no redirect. Neither secret appears in a data file. The delete command reads no setting (`TestDeleteReadsNoSetting`). Nothing formats the settings struct.
- **Reports never outlive their workout:** `ON DELETE CASCADE` with `foreign_keys(ON)` on every connection, including the delete command's. Owner delete, expiry, operator delete while `serve` runs, and recovery's row removal each leave no row and no bytes in `workouts.db`/`-wal` (`TestReportsGoWithTheirWorkout`, `dataLeft` extended). The 30-day ceiling is tested at the exact boundary.
- **11th report:** answered `204`, with no row, no log line and no notification.
- **Trust boundary:** the order is rate limit → deadlines → content type → 16 KiB `MaxBytesReader` → strict JSON → reason → message → lookup. A malformed ID returns not found without a query. The 404 body is byte-identical to the owner API's. Every request past the rate step counts. One `429` is logged per key, without the address. Message control characters are refused (C0, C1, DEL), so the operator's terminal cannot be driven by them. The delete prompt prints the title with `%q`.
- **Pushover cannot fail or delay a report:** `Notify` is a non-blocking send after the commit. The write uses `context.WithoutCancel`. With a hanging fake, 10 reports each answer in under 1 s (`TestReportNotification`). The global cap is 10 per rolling hour per process; the rest ride on the next message.
- **Delete command:** exit codes 0/1/2/3 as in R-9. It opens an existing database only and refuses a different schema version. It finds drafts and rows past their deadline. It deletes through `lifecycle.Service.Delete` under the ADR-031 flock, then checkpoints and logs `by: operator`. A repeat gives 3. Being stopped before or at the prompt changes nothing.
- **Migration:** `0004` only creates `reports` and its index; existing rows are unchanged (`TestMigrateV1` and `TestMigrateNoPrivate` at v4). ADR-019 item 10 records the cascade constraint for later rebuilds of `workouts`.
- **Compose:** without the override, `app` is on `backend` (internal) only. The override adds `egress` to `app` alone and publishes no port. The `env_file` reaches `app` only, not Caddy.

## Previous findings

None (round 1).

## Questions

1. **Architect, R-7 ("never as root"):** when the process runs as root, SQLite's unix VFS changes the owner of new `-wal`/`-shm` files to the database file's owner. If that holds for Debian's `sqlite3`, plain `sudo sqlite3 -readonly …` is safe, and QA-27-r1-1 shrinks to a wording fix. To settle it: on the VPS with `app` stopped, run `sudo sqlite3 -readonly /srv/workouts/workouts.db 'select 1'`, then `ls -ln /srv/workouts/workouts.db*`.
2. **Architect, ADR-039 item 5 / ADR-034 ("inbound is unchanged", "nothing outside `backend` reaches port 8080"):** `serve` listens on `:8080`, so with the override it also answers on its `egress` address. The host owns that bridge's gateway. On Docker Engine before 28, a neighbour on the provider's L2 network may be able to route to unpublished container addresses directly. To settle it: on the VPS with the override, run `curl -m3 http://<app egress IP>:8080/api/health` from the host, and the same against the `backend` address without the override. If only the egress address answers, either bind `serve` to its `backend` address or correct the ADR text and require Engine 28 or later.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 Stored with reason, message and time only; no IP or contact column | `store/reports.go`, `0004_reports.sql` | `TestSchema`, `TestReports`, `TestReportEndToEnd`, `TestReportNoRequestMetadataStored` |
| 2 1,001 characters or unknown reason refused | `reports.Check` | `TestCheck`, `TestReportValidation` (trim caveat: QA-27-r1-3) |
| 3 Draft or unknown workout gives not found | `AddReport`, `File` | `TestReportEndToEnd`, `TestDeadlineBoundary`, `TestReportNotFound` |
| 4 Rate limit; nothing about the IP persisted | `httpapi/report.go` | `TestReportRateLimit`, `TestReportNoRequestMetadataStored` |
| 5 Deleted by every delete path | cascade | `TestReportsGoWithTheirWorkout` (4 paths), `dataLeft` |
| 6 30-day ceiling | `Prune`, `DeleteReportsFiledBy` | `TestReportRetention`, `TestReports`, `TestPrune` |
| 7 11th report: `204`, not stored | `AddReport` cap | `TestReportEndToEnd`, `TestReportNotification` |
| 8 Delete command | `delete.go`, `DeleteByOperator`, `OpenExisting` | `TestDeleteCommand`, `TestOpenExisting`, operator subtest |
| 9 Settings matrix; no route out | `pushoverKeys`, compose | `TestPushoverSettings`; route out: manual, not run |
| 10 Message content; none for refused or unstored reports | `pushover.Message`, `File` | `TestRequest`, `TestReportNotification`, `TestFile` |
| 11 Slow or failing Pushover | `Notify`, `send` | `TestReportNotification`, `TestNotifyNeverBlocks`, `TestFailures` |
| 12 10 per hour, `(+N more)` | `handle` | `TestCap`, `TestNotifyNeverBlocks` |

**Not run (manual, Compose on the VPS):** egress with and without the override; a real Pushover message; `exec` prompting on a TTY; the Caddy `404` after a delete; the runbook query as UID 10001 on Debian. No real FIT file was read; the fixtures are corpusgen-synthetic.
