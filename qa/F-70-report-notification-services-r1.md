# QA F-70 Report notifications through other services - round 1

- **Date:** 2026-11-24
- **Verdict:** not ready - one Major: if the server shuts down while a send is in flight, its delete key stays valid. The revoke runs after the database has closed (QA-70-r1-1). Everything else meets the acceptance criteria. Minors and one Question are listed below.
- **Inputs:** `features/F-70-report-notification-services.md` (AC, F70-Q1, F70-Q2), `architect/F-70-report-notification-services.md` (NS-1 to NS-13), ADR-039, ADR-042, F-37 DK-3/DK-5, F-52 AU-11, TD Settings. Code at main 6adc76e (implementation e8ee12d). Working tree clean.

## Build and test results

- `make ci`: exit 0. go vet clean; `go test -count=1 ./...` all ok (`internal/notify` 0.22 s, `cmd/workout-explorer` 57 s); svelte-check 0 errors; vitest 93 files / 2,416 tests passed; Docker image built; `update_test.py` **56 tests OK**; `sign_test.py` 12 OK.
- `go test -race -count=1 ./internal/notify/ ./internal/reports/ ./internal/about/`: ok. `go test -race -run 'Notify|Report|Settings|Started|About|Expiry' ./cmd/workout-explorer/`: ok.
- Probes (scratchpad only, no change to `src/`), using `go test -overlay` to add test files to `cmd/workout-explorer`:
  - NS-13 state file: empty, short (47 bytes), long (49 bytes), one bit flipped in the MAC, one bit flipped in the salt: each revokes. Changing between Pushover and ntfy revokes. Moving one character across the URL/token boundary gives a different MAC. A directory in place of the file stops the start with `notify state: rename …: file exists` (fails closed; the message holds the path only, no secret).
  - Shutdown during a send: reproduced QA-70-r1-1, three runs out of three.
  - Comparison of Go `CheckURL` and `update.sh notify_url_ok`: 4,928 generated cases plus 41 hand-picked ones. Results under QA-70-r1-3.
  - curl URL globbing through `-K -` against a local Python fake: QA-70-r1-2.

## Findings

### Major

- **ID:** QA-70-r1-1
- **Severity / kind:** Major / defect
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/wiring.go:261-264` (`closeApp`: `stopNotify()` then `db.Close()` straight away; it does not wait for `notifier.Run`); `src/server/internal/notify/notify.go:157-161`
- **Relates to:** AC 5 ("timing out … no delete key issued for that report"), NS-7 ("own deadline, so it also runs at shutdown"), INF-6
- **Evidence:** Overlay test: ntfy pointed at a local endpoint that hangs, then a report; the request reaches the endpoint (key issued); then `closeApp()`. Log: `{"level":"WARN","msg":"delete key not revoked","id":1}`. `reports.delete_key_hash` still holds the hash after shutdown, in 3 of 3 runs. A restart with the same settings keeps it, by design (NS-13). `TestFailureClassesRevoke` ("also with an already cancelled context (shutdown)") passes only because it uses a fake `RevokeFunc`; no test runs this path against the real database.
- **Consequence:** A delete link whose delivery was never confirmed stays valid for up to 30 days. This happens on every restart (nightly auto-update, deploy, `docker stop`) while a send is in flight; a slow endpoint stretches that window to 10 s. The behaviour contradicts the rule the design writes down and the test claims to cover. Fix direction: `closeApp` waits for `Run` to return (and its revoke, at most 5 s) before `db.Close`, and a test exercises this through `newApp`.

### Minor

- **ID:** QA-70-r1-2
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `deploy/update.sh:170-174` (curl without `-g`/`--globoff`)
- **Relates to:** NS-11 ("lets the shell updater pass the URL to curl without escaping"), NS-3 (allows `[` and `]`)
- **Evidence:** I passed the URL to `curl -K -` as `alert()` does, against a local fake. `http://127.0.0.1:18997/cb?a[1-3]=x` led to **three** POSTs (`/cb?a1=x`, `/cb?a2=x`, `/cb?a3=x`). `…/cb/a[b]` made curl exit 3 (`bad range in URL`); the curl error text holds the whole URL and is discarded only because of `2>/dev/null`. Go and the shell both accept both URLs.
- **Consequence:** With a valid setting the updater can send several alerts, or none, while the server sends correctly. A URL with `[1-99999]` would send a burst of requests. Fix: add `-g`.

- **ID:** QA-70-r1-3
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/notify/notify.go:279-308`, `deploy/update.sh:126-142`, `deploy/test/notify-url-cases.tsv`
- **Relates to:** NS-11 ("the same accepted set as Go"), NS-3
- **Evidence:** The two validators disagree beyond the shared TSV:
  - Go rejects and the shell accepts: invalid percent escapes in the path (`https://ntfy.example.org/t%zz`, `…/t%`, `…/t%2`), which `url.Parse` refuses; invalid IPv6 literals (`https://[:::]/t`, `http://[fc00:::]/t`, `http://[fd00:1.2.3.4]/t`), which is the documented shape-only deviation.
  - Go accepts and the shell rejects: non-canonical loopback forms `http://[0:0:0:0:0:0:0:1]/t` and `http://[::0:1]/t`. The server sends; the updater logs "settings invalid; no alert".
  - No case lets the shell send plain http to a public address.
- **Consequence:** With a valid server setting the updater can send no alert, without anyone noticing. Fix: add these cases to the TSV and align the two validators (in both, reject a `%` not followed by two hex digits; in Go, require the canonical `::1`, or accept these forms in the shell).

- **ID:** QA-70-r1-4
- **Severity / kind:** Minor / defect (docs)
- **Route:** backend
- **Where:** `deploy/DOCKER.md:87`, `deploy/PODMAN.md:146`
- **Relates to:** AC 8 (docs cover it), NS-8
- **Evidence:** Both pages tell the hoster to check that `server started` shows `"notifications":true`. That field no longer exists (`main.go` `startedFields` now logs `notify` and `notify_host`).
- **Consequence:** A hoster following the install check sees no match and assumes notifications are off.

- **ID:** QA-70-r1-5
- **Severity / kind:** Minor / defect (docs)
- **Route:** architect
- **Where:** `architect/F-70-report-notification-services.md` header, NS-12, NS-13, "Open" table
- **Relates to:** F70-Q1, F70-Q2 (decided), CLAUDE.md "live documents hold the current state only"
- **Evidence:** The doc still says "NS-13 is proposed and built only if the user accepts F70-Q1". NS-12 says "a change of service, URL or token alone revokes nothing; RUNBOOK tells the hoster to start once with notifications off". F70-Q1 and F70-Q2 are listed as open. The code, the feature, ADR-042 and the RUNBOOK all implement automatic revocation.
- **Consequence:** The live design contradicts the code on a privacy rule, so a later reader may "fix" the code back to match the doc.

### Question

- **ID:** QA-70-r1-6
- **Severity / kind:** Question (Major if confirmed) / risk
- **Route:** architect
- **Where:** NS-5 ("No … `Firebase` header"), `notify.go:185-191`, `about.html` ntfy block, RUNBOOK "ntfy"
- **Relates to:** INF-1, INF-6, NFR Privacy (third parties)
- **Evidence:** I believe ntfy.sh forwards messages for topics that anyone can read to Google Firebase Cloud Messaging, including the click and action URLs, unless the request sends `Firebase: no`. I have not verified this in this round, because I contacted no external service or its documentation. NS-5 drops the header on purpose but gives no reason. Neither the About page nor the RUNBOOK names Google as a recipient.
- **Consequence if true:** For the most likely setup (a public topic on ntfy.sh), every delete link and viewing link also reaches Google, and nothing discloses it. Settled by: the architect checks ntfy's documented FCM behaviour, then either sends `Firebase: no` (cost: possibly slower delivery to the Play-store Android app) or names FCM on the About page and in the RUNBOOK.

## Previous findings

None (round 1).

## Questions

1. QA-70-r1-6: does ntfy.sh forward public-topic messages, including action URLs, to FCM without `Firebase: no`? A yes or no with a reference settles it.
2. QA-70-r1-3: which side should give way on non-canonical IPv6 loopback forms? Suggestion: in both, allow `http` only to the canonical `[::1]`.

## Coverage

| AC | Code | Test | Result |
| --- | --- | --- | --- |
| 1 ntfy message, delete link only in the action | `notify.go` request (Ntfy) | `TestNtfyRequest`, `TestNtfyEndToEnd` (body has no key or report text; Title/Click/Actions/Authorization; no Markdown/Priority/Tags/Cache/Firebase) | pass |
| 2 callback fixed JSON plus token | `notify.go` request (Callback) | `TestCallbackRequest` (exact bytes, key order, `delete_url` absent), `TestCallbackEndToEnd` | pass |
| 3 Pushover unchanged | `notify.go` default branch | `TestRequest` (same form as before), `report_test.go`; deviation "any 2xx" accepted | pass |
| 4 exit 2, setting named, value never printed | `settings.go notifyService`, `CheckURL` fixed text | `TestNotifySettings` (incl. `serve` exit 2, no data dir created) | pass; error text is fixed by construction |
| 5 down / timeout / 4xx-5xx / redirect: stored, one WARN, no key | `notify.go send/handle` | `TestDeliveryAndRevocation` (3 kinds × 9 statuses, log byte scan), `TestFailureClassesRevoke`, `TestNtfyEndToEnd` redirect | pass while running; **fails at shutdown (QA-70-r1-1)** |
| 6 11th report folded on every service | shared `handle` | `TestSharedClient` | pass |
| 7 revocation (off, change, upgrade, unchanged keeps), no plain settings in data dir, updater alert | `notifystate.go`, `wiring.go` | `TestNotifyChangeRevokes`, QA overlay probe (corrupt/short/long/flipped state file), `update_test.py` ntfy/callback/redirect/invalid | pass |
| 8 About names the service, docs, local fakes only | `about.go`, `about.html`, deploy docs | `TestAboutNotifyBlocks` (ntfy host only, callback no host), about.test.ts | pass; docs stale (QA-70-r1-4) |

Checked and found correct:
- No URL, path, token, delete link, response body or Go error text in the WARN line (byte scan in tests, and a review of every `Warn`/`Error` call). Exit-2 messages are fixed text. `notify_host` is the only part of the URL that is logged.
- No panic path: `Host` cannot slice out of range (`rest+"/"`).
- Redirects are never followed (`ErrUseLastResponse`) in Go, and curl runs without `-L`.
- Response body read is capped at 4 KiB; no proxy is taken from the environment; one shared client.
- `update.sh`: URL and token reach curl only on stdin (`-K -`) and are checked in tests through recorded argv. `-q` ignores `.curlrc`; `--noproxy '*'`; `--proto` is pinned; stderr goes to `/dev/null`. `envval` reads a single line, so no newline can inject into the curl config, and `"` and `\` are excluded.
- `WX_UPDATE_NOTIFY_DRYRUN` comes only from the process environment. No unit sets it, and `.env` is read as data, never sourced. It can only suppress an alert, never redirect one, so it cannot be abused beyond what root can already do.
- Token with several `=` matches RFC 7235 token68 (correct). Rejecting `_` in host names (stricter host syntax) is acceptable.
- NS-13: the keys are cleared before the state file is written (temp file plus rename, 0600), so a crash in between clears again. The MAC fields are NUL-separated. `notifyChanged` is called only by `serve`.
- The de/es/fr About texts are accurate and match the English.

Not run or not traced: real ntfy apps (F70-V1, action buttons on iOS/web), a real Pushover or ntfy delivery, the updater on a real host, a manual look at the About page in each language.
