# QA F-70 Report notifications through other services - round 2

- **Date:** 2026-11-24
- **Verdict:** ready with conditions. The Major QA-70-r1-1 is fixed: shutdown now waits for the in-flight send and its revoke, and a mutant without the wait fails the new test. Minors r1-2 to r1-5 are fixed. The F70-Q3 disclosure meets the new AC. Three new Minors and the user's hand checks are listed below.
- **Inputs:** `qa/F-70-report-notification-services-r1.md`; `features/F-70-report-notification-services.md` (new Scope "ntfy and push providers" and new AC 8); `architect/F-70-report-notification-services.md` v0.2; `architect/INDEX.md` F70-Q3; spec INF-1 and NFR Privacy. Diff `6adc76e..a71b977` (c09cd38, 94ef52b, a0d1e21 and their merges). Working tree clean.

## Build and test results

- `make ci`: exit 0. go vet clean; `go test ./...` all ok (`cmd/workout-explorer` 58.9 s, `internal/notify` 0.22 s); svelte-check 0 errors; vitest 93 files / 2,416 tests passed; bundle and language-file budgets pass (`lang-static-*` 4,221 to 4,460 / 5,120 B gzip); Docker image built; `update_test.py` **57 OK** (56 in round 1, plus the bracket test); `sign_test.py` 12 OK.
- `go test -race -count=5 -run TestShutdownRevokesInFlight ./cmd/workout-explorer/`: ok. `go test -race -count=1 ./internal/notify/ ./internal/reports/` and `-run 'Notify|Ntfy|Callback|Shutdown|About|Settings|Started' ./cmd/workout-explorer/`: ok.
- Probes (scratchpad only; test files added with `go test -overlay`, no change to `src/` or `deploy/`):
  - **Mutant, wiring.go without `<-notifyDone`:** `TestShutdownRevokesInFlight` fails ("key left valid after shutdown"). The test kills the defect it was written for.
  - **Shutdown timing, ntfy endpoint that hangs, a second report queued:** `closeApp` returns in 0.36 to 0.44 ms, and no key is left (3 of 3 runs). **The same with the database write-locked by a second connection:** `closeApp` returns in 5.009 s (the 5 s revoke deadline holds), "delete key not revoked", and the key is left. That is NS-7's accepted failure class, logged with the ID only.
  - **Mutant, `update.sh` without `-g`:** `test_notify_url_brackets_are_not_globbed` fails. The test kills it.
  - **Go `CheckURL` vs `update.sh notify_url_ok`:** 12,041 cases (2 × 6,000 generated: IPv6 with 0 to 10 groups of 0 to 5 digits, `::`/`:::`, IPv4 tails, fc/fd/fe/fb prefixes, v4 with octets 256 and `01`, `%` in path and query, ports 0/1/8080/65536; plus 41 hand-picked: `[::01]`, `[0::1]`, `[::ffff:7f00:1]`, `[fd00:1:2:3::4:5:6:7]`, `[fd00:1:2:3:4:5:6:7::]`, `%25`, `%e2%82%ac`, `:00443`, `100.64.0.1`, `169.254.1.1`, `[fe80::1]`, and others). **677 differences, all in the documented class**: https with a bracketed IPv6 host, Go rejects and the shell accepts (shape-only). There are **0** differences for http and **0** for `%` escapes, and no case where the shell accepts plain http that Go refuses.
  - **About page rendered from the real built `about.html`** for ntfy, callback, Pushover and off: "Firebase" appears only with ntfy.

## Findings

### Minor

- **ID:** QA-70-r2-1
- **Severity / kind:** Minor / defect (log noise)
- **Route:** backend
- **Where:** `src/server/internal/notify/notify.go` `Run` (the `select` between `ctx.Done()` and `n.queue`)
- **Relates to:** NS-7, NS-8, ADR-039 item 3 ("pending notifications are dropped at shutdown")
- **Evidence:** Now that `closeApp` waits for `Run`, `Run` keeps running after the cancel. When both cases are ready, Go's `select` picks one at random, so `Run` can take queued items after the cancel. In the timing probe with a second report queued, 2 of 6 runs logged `"delete key not issued","id":2` and `"notification failed","kind":"ntfy","class":"network"` for a report that was never attempted. No key is issued: `issue` fails on the cancelled context. The drain is bounded by `queueSize` (16) and fast.
- **Consequence:** A restart while reports are queued writes WARN lines claiming those notifications failed. The operator reads this as an endpoint fault. The doc comment "pending ones are dropped" is not what happens. Fix direction: return at the top of `handle` (or in `Run`) once `ctx.Err() != nil`.

- **ID:** QA-70-r2-2
- **Severity / kind:** Minor / defect (docs)
- **Route:** architect
- **Where:** `architect/F-70-report-notification-services.md` section "Open", row F70-Q3
- **Relates to:** F70-Q3 (decided 24 Nov 2026), CLAUDE.md "live documents hold the current state only"
- **Evidence:** NS-5 in the same doc and `architect/INDEX.md` record F70-Q3 as decided. The "Open" table still asks "Send it?" with owner "user".
- **Consequence:** The doc says one thing in NS-5 and the opposite in "Open", the same kind of contradiction r1-5 removed for F70-Q1 and Q2.

- **ID:** QA-70-r2-3
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/settings_test.go:84` (`"Google (Firebase) and Apple"` checked with `Contains` on the whole template); `TestAboutNotifyBlocks` uses the fixture web dir, not `about.html`
- **Relates to:** AC 8 ("with Pushover, a callback or notifications off, it does not")
- **Evidence:** No test checks that the sentence lies inside `<!--wx:ntfy-->…<!--/wx:ntfy-->`. If the `<p data-t="ab_ntfyPush">` moved outside the block, every test would still pass. It is correct today: I rendered the real page per setting.
- **Consequence:** The negative half of the AC is protected only by review.

## Previous findings

| ID | Status | How verified |
| --- | --- | --- |
| QA-70-r1-1 (Major) | **fixed** | `wiring.go`: `notifyDone` is closed by the `Run` goroutine, or right away when the notifier is nil; `closeApp` waits after `wg.Wait()` and before `db.Close()`. Repeat calls are safe (`once` plus a closed channel). `TestShutdownRevokesInFlight` goes through `newApp`, the real database and a hanging endpoint; it fails on the mutant and passes 5 times under `-race`. Worst-case stop: `srv.Shutdown` grace 10 s (the notifier is not cancelled until `closeApp`), then the expiry pass, then at most 5 s of revoke plus a fast queue drain, about 15 s. That is below `stop_grace_period: 30s` (compose), `StopTimeout=30` (Podman) and `TimeoutStopSec=30s` (systemd). A hanging endpoint does not add its 10 s client timeout, because `closeApp` cancels the request. The `privatedir.Usage` error path calls `stopNotify()` without waiting, but at that point the listener is not open, so nothing can be queued (not a defect). Residual: QA-70-r2-1. |
| QA-70-r1-2 | **fixed** | `curl -q -g`; new `test_notify_url_brackets_are_not_globbed` (one request, literal path) kills the mutant. |
| QA-70-r1-3 | **fixed** | Shared `%XX` rule in Go (`badPercent`) and the shell; Go needs the canonical `[::1]` and rejects IPv6 with an IPv4 part on http; the shell checks fc00::/7 by group count. 17 new TSV cases. Comparison above: only the documented https shape-only class remains (recorded in NS-3). |
| QA-70-r1-4 | **fixed** | DOCKER.md and PODMAN.md name `"notify"` / `notify_host`; no `"notifications":true` left in `deploy/` or `src/`. |
| QA-70-r1-5 | **fixed** | Header v0.2 "accepted", NS-12/NS-13 state automatic revocation, F70-Q1/Q2 removed from "Open", history file added. New residual about F70-Q3: QA-70-r2-2. |
| QA-70-r1-6 (Question) | **closed by user decision F70-Q3** | Disclosure checked against the new AC 8: `ab_ntfyPush` sits inside the ntfy block only (real page rendered per setting); `TestNtfyRequest` asserts no `Firebase` header; RUNBOOK covers the forwarding, `upstream-base-url` poll request, and the leaked-topic step; spec INF-1/NFR Privacy and the feature decision log are updated. |

Translation check (`ab_ntfyPush`), all correct and faithful to the English:
- de "Der ntfy-Server kann diese Benachrichtigungen, auch mit dem Ansichts- und dem Löschlink, an Google (Firebase) und Apple weitergeben, damit sie auf Telefonen zugestellt werden." Natural.
- es "…puede pasar estos avisos, incluidos el enlace de visualización y el de eliminación, a Google (Firebase) y Apple para entregarlos en los teléfonos." Grammatical (agreement of "incluidos" is right). "transmitir" would read a little more formally than "pasar"; not a defect.
- fr "…peut transmettre ces notifications, y compris le lien de consultation et le lien de suppression, à Google (Firebase) et à Apple pour les remettre aux téléphones." Correct. "pour qu'elles soient distribuées sur les téléphones" would be more idiomatic; not a defect.
The terms match the sibling sentences (Ansichtslink, enlace de visualización, lien de consultation). All three ship in the built `lang-static-*` files.

RUNBOOK: the Scope asks it to say that `Firebase: no` "is not offered as an option". It says the request has no such header, but not explicitly that no setting exists. Acceptable; one clause would remove any doubt.

## Questions

None.

## Coverage

- AC 5 (shutdown case): now traced to `wiring.go` `closeApp` and `TestShutdownRevokesInFlight`, which I mutation-checked.
- AC 7 updater alert: bracket URLs are covered by `test_notify_url_brackets_are_not_globbed`; validator agreement is covered by the TSV and my 12,041-case comparison.
- AC 8 (new): About sentence traced to `about.html` ntfy block plus de/es/fr; template test (positive only, QA-70-r2-3); no-`Firebase`-header test; real page probe per setting; RUNBOOK read.
- ACs 1 to 4 and 6: the diff does not touch them; not re-verified (closed in round 1).
- Not verified: ntfy's FCM/APNs behaviour against its documentation or a real server (no external services in this round). The RUNBOOK's "public topic" and `upstream-base-url` poll-request statements match my understanding of ntfy but are unverified here.
- **User hand checks (conditions):** (1) one real ntfy delivery to your phone: the title, body and **Delete this workout** button, with no link visible on the lock screen (AC 1, F70-V1 for iOS); (2) the About page with ntfy set, in en/de/es/fr: the new sentence shows between the ntfy paragraph and "The ntfy server is:", and not with Pushover or with notifications off; (3) one forced updater failure on a host with ntfy set, to confirm the alert arrives once.
