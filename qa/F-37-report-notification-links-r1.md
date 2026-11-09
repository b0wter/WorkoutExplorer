# QA F-37 Report notification links - round 1

- **Date:** 2026-11-09
- **Verdict:** ready with conditions. No Blocker or Major finding. The condition is the user's phone check (AC 41, OQ-125). That check confirms the `#fragment` survives the Pushover app's anchor and that the lock screen shows no key.
- **Inputs:** `features/F-37-report-notification-links.md`; `architect/F-37-report-notification-links.md` v0.1 (DK-1 to DK-14), ADR-042, ADR-039 rev. 1, ADR-025, ADR-020; spec INF-1, INF-3, INF-4, INF-6, AC-4, SH-10, NFR Privacy and Security; TD Hosting (Reports), Security, Logging, Data model. Commits bb47219 (backend) and 36be73d (frontend), clean tree.

## Build and test results

- `make ci`: exit 0. Lint and vet are clean. Go reports `ok` for all 19 packages. Vitest: 62 files and 1,586 tests passed. Build succeeded. Size: `report-delete.html` 1,610 / 4,000 B gzip PASS, and its JS and CSS form their own chunk. The `index`/`edit` entries are unchanged at 57,746 / 60,000. The upload entry PASSes. `caddy-check` ran against Caddy 2.11.7 (`TestCaddy*` ok, strict `/report-delete` branch against the built page). `docker-check` ok.
- `go test -race -count=3` for the F-37 tests (reports, httpapi, pushover, store, cmd `ReportDelete*`): ok.
- Migration probe (go `-overlay`, not committed): I built a v4 database with an unlisted workout and 3 reports and opened it with `store.Open`. Result: `user_version` 5, the 3 reports kept with `delete_key_hash` NULL, the partial unique index present, a duplicate hash refused, and the cascade removed the reports with the workout. PASS.
- Go mutants (overlay, 8): 6 killed outright (query-string check, workout-alive clause, 30-day clause, `ClearDeleteKeys` at start, `html=1`, key-format check). 2 survived a single run: the in-process redeem mutex (killed at `-count=30`, see QA-37-r1-3) and the rate value 30→3000 (QA-37-r1-3).

## Findings

### Minor

- **ID:** QA-37-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend (or architect: record it as accepted)
- **Where:** `src/server/internal/lifecycle/owner.go:383`, `src/server/internal/reports/reports.go` (`RedeemDeleteKey`)
- **Relates to:** DK-8, architecture Verification focus ("a confirm racing the CLI delete: exactly one `204` and one operator log line")
- **Evidence:** Code read, not executed. `DeleteByOperator` logs `workout deleted` and returns nil whenever `Delete` returns nil. `Delete` returns nil when the row is already gone, because both folder removals are idempotent (`publish.removeFolder` and `privatedir.Remove` ignore a missing folder). So if the confirm consumes the key and the CLI then deletes, or the CLI shows the row, the phone deletes, and the operator types `y`, both processes log an operator line and both report success.
- **Consequence:** One workout gets two `by=operator` lines. No data stays behind, no key leaks, and storage is not released twice (it is guarded by `deleted`). The verification focus as written is not met. **Judgement:** acceptable. The same behaviour has existed for CLI vs CLI since F-27. Fix it cheaply in the small lane (log only when the row was deleted), or record it as a known deviation in DK-8.

- **ID:** QA-37-r1-2
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/report-delete/page.ts:85`
- **Relates to:** DK-8 ("a delete error after the consume answers `500` … the operator uses `workout-explorer delete`"), INF-6
- **Evidence:** A POST `500` sets the page to `ready` with "Something went wrong. Please try again later.". The key is already spent, so the retry gets the 404 sentence "…already used, has expired, or the workout is gone."
- **Consequence:** After a failed delete the workout can still be online, for example when the row delete itself failed. The operator is first told to retry and then told the link is spent or the workout is gone, so they may believe it is deleted. A POST `500` should say that the link is used up and that the workout may still exist, and point to the CLI. It is rare (a database or file error), but this path deletes data.

- **ID:** QA-37-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/reportdelete_test.go:311`, `src/server/internal/httpapi/reportdelete_test.go:150`
- **Relates to:** AC 33, AC 38, DK-8, DK-9
- **Evidence:** I removed `s.mu.Lock` and its `defer Unlock` in `RedeemDeleteKey`. `TestReportDeleteConcurrent` passed one run and failed under `-count=30` (codes `[204 204 404…]`, 3 delete lines), so it catches the mutant only probabilistically. The rate-limit test loops to `reportDeletePerHour`, so changing 30 to 3000 survives.
- **Consequence:** The single-204 guarantee for two keys of one workout depends on the mutex, and CI catches its loss only by chance. DK-9's value is not pinned. Fix: force the interleaving, for example with a store fake that blocks after the first consume, and assert the literal 30.

- **ID:** QA-37-r1-4
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/caddy_test.go:335`
- **Relates to:** AC 37, DK-11
- **Evidence:** The `shortcut:` branch accepts `/report-delete` → 404 when `report-delete.html` is missing from the web build. The page now exists.
- **Consequence:** A build that drops the entry still passes `caddy-check`. Remove the fallback.

- **ID:** QA-37-r1-5
- **Severity / kind:** Minor / docs
- **Route:** architect
- **Where:** `architect/ADR-042-report-delete-keys.md:3`, `architect/F-37-report-notification-links.md` (Status)
- **Evidence:** ADR-042 is still "proposed" and the design is still "draft", although both are built and shipped in the code.
- **Consequence:** The record does not match what is built. Mark both accepted, and add QA-37-r1-1 if it is accepted.

## Checked and correct

- **Key.** 16 bytes from `crypto/rand` encoded as 22 base62 characters (`lifecycle.NewKey`). Only the SHA-256 is stored, in `reports.delete_key_hash` (CHECK length 32, partial unique index). The key reaches the server only as `Authorization: Bearer`, and there is no route with a key in the path or query.
- **Never in the clear.** The e2e byte scan of `workouts.db`/`-wal`/`-shm`, all log lines and all API responses finds neither the key nor its hash nor `report-delete#`. The store binds the hash as a parameter, and error lines carry no key.
- **Issued only when sent.** The key is issued after the hourly cap admits the message and right before the request (`TestCap`: no key for held or folded reports). A failed issue sends the message without a link and logs a WARN with the ID only.
- **Pushover payload.** Exactly 7 fields: token, user, title, message, `html=1`, `url` = `/w/{public_id}`, `url_title`. Label and link are HTML-escaped. The reporter's message, title, description, IP, internal ID and other public IDs are absent (fake-Pushover test with markers). All four labels are exercised.
- **Lookup.** A `SELECT` only, so repeated lookups leave hashes and the public folder intact. It returns exactly the 7 DK-7 fields, and it returns no public ID, internal ID, description or message. A key never shows or deletes another workout, and no public ID is accepted in the request.
- **Single use.** One `UPDATE … RETURNING` consumes the key. 16 concurrent consumes over two connections yield exactly one winner. In-process, the service mutex makes the second key of a deleted workout invalid.
- **Invalid answer.** Absent, short, long, non-base62, unknown, used, other key of a deleted workout, 30-day bound, workout at its deadline and owner-deleted all get the identical 404 body with `no-store`, for GET and POST, and none of these refusals is logged. On timing, only the malformed-format check skips the database, and that reveals nothing secret.
- **Rate limit.** Checked first. 429 with `Retry-After`, one log line without the address, and lookup and confirm counted together.
- **Pushover off.** Start clears all keys (logged count), and reports create none. Keys stay revoked after Pushover is turned back on.
- **Deletion.** Uses `DeleteByOperator` unchanged. The F-12 list is empty afterwards (`dataLeft`), with one `by=operator` line of 5 keys and no key. The public folder is removed at once (SH-10).
- **Confirmation page.** No POST without a click, and no second POST while one is in flight (button disabled and guarded). A malformed fragment sends no request. Values are set via `textContent`. The page uses `redirect:'error'`, `referrerPolicy:'no-referrer'`, no inline script (CSP unchanged), robots/referrer metas, no og/twitter tags and a `<noscript>` note. Caddy serves `/report-delete` with `no-store` and the site-wide `X-Robots-Tag`/`Referrer-Policy`/CSP, and `/report-delete.html` and `/report-delete/` answer 404. No access log (`TestCaddyLogsNoRequests`).
- **About text** matches what Pushover receives and the INF-1 wording. It is still shown only when Pushover is on.

## Previous findings

None (round 1).

## Questions

- **Q1 (user, AC 41 / OQ-125).** On the iPhone, check three things. First, the lock-screen and banner text shows "Delete this workout" and not the URL. Second, tapping the anchor in the Pushover app opens `/report-delete#…` with the key. If not, the page shows the invalid message without making a request. Third, "Open workout" opens `/w/{id}`. If the fragment is lost, DK-10 falls back to a plain URL line, which puts the key on the lock screen and needs an About text update.
- **Q2 (user, wording).** The About text says Pushover "never learns … the workout's title". That is true of the payload, but the viewing link it receives opens the public page, title and public route included. Do you want the text to say that Pushover can open the reported workout? Settled by your wording choice. INF-1 itself is satisfied as written.

## Coverage

All 15 ACs are traced to code and tests. AC 27 to AC 40 have automated tests (e2e `cmd/workout-explorer/reportdelete_test.go`, `httpapi/reportdelete_test.go`, `reports`, `store`, `pushover`, `caddy_test.go`, `report-delete/page.dom.test.ts`). AC 39 is met by the architect's equivalent (Bearer header as the CSRF barrier, DK-6). AC 41 is a user check (Q1). Not run: the cross-process confirm-vs-CLI race (QA-37-r1-1 is from code reading), a real Pushover delivery, fuzzing (the key parser is a fixed regex plus a hash lookup, so fuzzing adds little). I did not touch the dev stack on 18080/18081.
