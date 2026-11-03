# QA F-27 Reports API and operator delete command - round 2

- **Date:** 2026-11-03
- **Verdict:** ready with conditions. All six round-1 findings are fixed, and the round-1 Major (QA-27-r1-1) is closed by a probe as UID 10001. No Blocker or Major is open. The conditions are the manual Compose checks from round 1, which have not been run: no route out without the override, a real Pushover message, `exec` prompting on a TTY, the Caddy `404` after a delete, and the runbook query on the VPS.
- **Inputs:** QA r1 report; commit `cb50e71` (diff since `d9e3bfc`); F-27 design R-5, R-7 and test plan; ADR-039 item 5; ADR-034 rev. 4; TD Code structure; `deploy/RUNBOOK.md` section 10 and the requirements tables; Docker Engine 28.0 release notes.

## Build and test results

- `make ci`: exit 0, with 1,404 vitest tests passing and docker-check ok. The Go step used cached results.
- `go test -count=1 -race ./...` in `src/server`: all 17 packages pass, including `tools/protoserve`, whose test now expects the module script and the DS-4 theme script.
- Mutants, applied with `go test -overlay` so `src/` was never written:

  | Mutant | Result |
  | --- | --- |
  | `AUTOINCREMENT` removed from `0004_reports.sql` | killed: `TestReports` "number after deletes 1, want 6" |
  | Cap window boundary `!now.Before(…)` changed to `now.After(…)` | killed in 5 s: `TestCap` "no request". In r1 it was caught only by the 10-minute timeout |

- Runbook query probe. Inside a user namespace with `--map-auto` and a private tmpfs, the store's four migrations built a database owned `10001:10001` with mode 0600 in a 0700 directory. The runbook command then ran verbatim as `setpriv --reuid=10001 --regid=10001 --clear-groups sqlite3 -readonly -line …` (util-linux 2.41.6, sqlite3 3.51.2). It works. It created `-wal` and `-shm` files, both owned by 10001. A message containing `hello\npublic_id = BBBB…\n\n   report = 99\nit's` printed as one line, `message = 'hello\npublic_id = …\n\n   report = 99\nit''s'`, so it can no longer forge a line or a whole record. A root `sqlite3 -readonly` run (namespace root, so `geteuid()==0`) also left the side files owned by 10001, which confirms Question 1 of round 1. After report 2 was deleted, the next report got number 3, so numbers are not reused.
- `TestUploadIdentifyingData` (flake triage): failed 1 time in 30 under `-race`. With an overlay that prints the log on failure, 2 failures came from about 3,000 runs. The log lines were:
  - `"time":"…15:11:53.543213838+01:00"`: contains `54321`
  - `"time":"…15:12:30.234567464+01:00"`: contains `23456`

  Both hits are in the nanosecond part of slog's timestamp. No log field carries device data. The other variable fields are the random hex `id` and `duration_ms`, and the count fields are fixed. This is a **flake, not a leak** (QA-27-r2-2).

## Findings

### Minor

- **ID:** QA-27-r2-1
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect (ADR-034), backend (runbook requirements)
- **Where:** `architect/ADR-039-report-notifications-through-pushover.md` item 5; `architect/ADR-034-*.md` line 30 ("nothing outside `backend` reaches port 8080"); `deploy/RUNBOOK.md:36` (requirements: "Docker Engine … Compose 2.24 or newer")
- **Relates to:** ADR-039 item 5, ADR-034, round-1 Question 2
- **Evidence:** The Docker 28.0 release notes confirm "Direct routed access to container ports that are not exposed using `-p`/`--publish` is now blocked". They also confirm that the host can still reach them. ADR-039 now relies on Engine 28 and says the runbook requires it. The runbook does not: the general requirements table asks only for Compose 2.24, which runs on older Engines. Only the operator's VPS line (385) says "from Docker's repository", which gives a current Engine in practice. ADR-034's consequence line, which is unchanged, is now false: `serve` answers on its `egress` address.
- **Consequence:** A self-hoster on an older Engine, such as Debian's own `docker.io` package, who enables notifications lets L2 neighbours reach `serve:8080` without passing Caddy. The exposure is small. Go treats such a peer as the client, so the rate limits still apply, and the API is public anyway. However, Caddy's headers and limits are skipped, and the docs claim otherwise. Fix: add "Docker Engine 28 or newer" to the runbook's requirements and correct ADR-034's line. Alternatively, bind `serve` to its `backend` address.

- **ID:** QA-27-r2-2
- **Severity / kind:** Minor / defect (test, flaky CI)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/upload_test.go:339` (`len(m) >= 5 && strings.Contains(a.logs.String(), string(m))`)
- **Relates to:** AC 6 of the upload feature (not F-27). Found while triaging the engineer's report.
- **Evidence:** See the flake triage above. The 5-digit markers `54321`, `12345` and `23456` are matched against the raw log text, and that text contains three 9-digit nanosecond timestamps. The expected failure rate is about 5 × 10⁻⁴ per run.
- **Consequence:** CI fails at random, and a flaky privacy test teaches people to rerun privacy failures. Fix: decode each JSON log line and search the values with `time` excluded, or check the decimal markers against the fields other than `time` only.

## Previous findings

| ID | Status | How verified |
| --- | --- | --- |
| QA-27-r1-1 (Major) | fixed | Runbook and R-7 now use `setpriv`. Ran verbatim as UID 10001 without a passwd entry (probe above). Not yet run on the VPS (condition) |
| QA-27-r1-2 | fixed | The message comes last, quoted, with line feeds shown as `\n`. The forged-record probe prints one line. The title, the only other user text, cannot hold control characters (F-12) |
| QA-27-r1-3 | fixed | R-5 now says "trimmed, then the F-12 description rule; F-12 itself does not trim" |
| QA-27-r1-4 | fixed | `AUTOINCREMENT` in `0004` (no release tag contains `d9e3bfc`, so the change to an existing migration is safe). Pinned by `TestReports`, mutant killed |
| QA-27-r1-5 | fixed | `TestCap` uses `wait`. The boundary mutant fails in 5 s |
| QA-27-r1-6 | fixed | TD line `reports ──▶ lifecycle for ValidPublicID only (F-27)` |
| r1 note: protoserve | fixed | Passes uncached under `-race` |

Round-1 questions: Q1 is answered by the engineer and confirmed by the probe: SQLite chowns side files it creates as root. One nit: R-7 cites this as "tested, F-27 QA r1", but round 1 only asked the question and this round tested it. Q2 is answered by the ADR-039 sentence; what remains is QA-27-r2-1.

## Questions

None.

## Coverage

This round covered only the diff and the routed findings. The AC coverage table from round 1 still holds. Not run (manual, Compose on the VPS): egress with and without the override, a real Pushover message, `exec` on a TTY, the Caddy `404` after a delete, and the runbook query on Debian with real `sudo`. The probe ran in a user namespace and not under `sudo`. No real FIT file was read.
