# QA Test-suite audit - round 1

- **Date:** 2026-11-27
- **Verdict:** ready with conditions. The suite has not grown into heavy overlap. Two flaky tests make `make ci` unreliable, and they matter more than any deletion: `TestNotifyChangeRevokes` (Go, failed 4 of 10 runs) and the colouring golden test (Vitest, timed out once under load).
- **Inputs:** `src/web/src/**/*.test.ts`, `src/web/e2e/*.pw.mjs` with its README and BACKLOG, `src/server/**/*_test.go`, Makefile. Tree at main 556f41a.

## Build and test results

- `go test -count=1 ./...` (2 m 42 s wall): **FAIL**. `cmd/workout-explorer` `TestNotifyChangeRevokes` failed at `notify_test.go:230` "same settings revoked: []". Run alone it failed 2 of 3, then 2 of 6. Every other package passes.
- `npx vitest run` (1 m 42 s wall): **2,566 tests, 1 failed**. `colouring.test.ts:245` "ride-12h ... (a few ms each)" hit the 5 s timeout. Run alone, the file passes and the test takes 1,849 ms.
- Go time per test (ingest, publish, preview): `TestFramingSweep` 74 s, `TestAddedLapsPrivacyCorpus` 27 s, preview `TestCorpus` 26 s, `TestHidingRadii` 21 s, `TestHidingCorpus` 20 s, `TestAcceptDataSizeSweep` 19 s.
- Vitest time per file: `edit.dom.test.ts` 66 s (169 tests), `page.dom` 21 s, `ChartStack.dom` 20 s, `zip` 14 s, `laps.dom` 12 s.
- `make ui-test` and the Python tests were not run.

The suite is larger than the brief said: Vitest has 2,566 cases, not about 1,490.

## Findings

- **ID:** QA-AUD-r1-1
- **Severity / kind:** Major / defect (flaky test)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/notify_test.go:91-101`, `:217-230`; `internal/notify/notify.go:160-164`
- **Relates to:** F-70 NS-7, NS-13
- **Evidence:** The fake sends the body to its channel before it writes the status. `f.next(t)` returns, the test checks `hashes()` and calls `a.close()` while the notifier is still waiting for the response. The cancelled send counts as a failure, NS-7 revokes the key, and the reopened app finds none. 4 failures in 10 runs.
- **Consequence:** `make ci` fails at random. Developers learn to re-run red builds, which hides real failures in the delete-key (privacy) path. Fix: make the test wait for the notifier to finish, for example by waiting for the response or the log line before `close`.

- **ID:** QA-AUD-r1-2
- **Severity / kind:** Minor / defect (flaky, slow)
- **Route:** frontend
- **Where:** `src/web/src/lib/colouring/colouring.test.ts:245-255`
- **Evidence:** The test calls `expect` once per edge, for every metric on ride-12h: about 43k samples times each metric. It takes 1.8 s alone and timed out at 5 s in the full run.
- **Consequence:** Random red runs in CI. Fix: count the out-of-range classes and assert once per metric (`expect(bad).toBe(0)`). Coverage stays the same at a fraction of the time. The same per-element `for … expect` pattern appears 95 times in 20 files; only check it where the data is large.

- **ID:** QA-AUD-r1-3
- **Severity / kind:** Minor / duplicate
- **Route:** frontend
- **Where:** `src/web/e2e/security.pw.mjs:9-23` (7 tests) vs `src/server/cmd/workout-explorer/caddy_test.go:34-38` (`TestCaddy`, run by `make ci` through `caddy-check`)
- **Relates to:** F-22
- **Evidence:** Both send requests to the pinned Caddy and check the CSP, Referrer-Policy, X-Robots-Tag and nosniff headers. The Go test compares the exact header values and checks the CSP hashes against the built shells. The Playwright test checks a weaker set of substrings and runs only nightly.
- **Consequence:** About 15 lines with no extra protection. Delete them; keep F-23 (Open Graph) and F-36 (no third-party host), which only the browser can check. Risk: low.

- **ID:** QA-AUD-r1-4
- **Severity / kind:** Minor / weak (tests the implementation)
- **Route:** frontend
- **Where:** `src/web/src/page/hygiene.test.ts:45-61`
- **Evidence:** These tests compare literal markup strings from `LapsTable.svelte` and `ActivityPage.svelte` with `index.html`, including the exact props `<LapsTable laps={model.Laps} {remove} {auto} />`.
- **Consequence:** Renaming a prop breaks the test while behaviour stays the same. The behaviour it protects (no layout jump between the server markup and the mounted page) is checked by the "JS off" and CLS tests in `f73-split.pw.mjs:51` and `f76-chart-size.pw.mjs:193,215`, but only nightly. Keep it, but loosen it to the slot ids and their order. The CSP and `{@html}` source checks in the same file are architecture guards and stay.

## Overlap: jsdom vs Playwright (finding 1 of the request)

Overlap is shallow. Each Playwright test is a single happy path. The jsdom tests cover what the browser suite never reaches: 404, 409 and 500 answers, C-2 locking, teardown and listener removal, refused clipboard, blocked storage, bfcache. Pairs that check the same behaviour:

| Behaviour | jsdom | Playwright |
| --- | --- | --- |
| Suggested title as placeholder | `edit/edit.dom.test.ts:339` | `e2e/edit.pw.mjs:24` |
| Lifetime select and expiry text | `edit.dom.test.ts:452` | `edit.pw.mjs:35` |
| Share copies the viewing link | `edit.dom.test.ts:917` | `publish.pw.mjs:26` |
| Delete asks first, then the gone screen | `edit.dom.test.ts:1123,1137` | `publish.pw.mjs:69` |
| Pin dialog: Cancel, Esc, OK, Delete | `edit.dom.test.ts:2031,2095` | `pins-laps.pw.mjs:73` |
| Remove and Restore auto-laps | `edit.dom.test.ts:2302` | `pins-laps.pw.mjs:152` |
| Laps row tap selects, a second tap clears | `page/laps.dom.test.ts:109,130` | `laps.pw.mjs:115` |
| Split presets change the rows | `laps.dom.test.ts:218` | `laps.pw.mjs:35` |
| One table per leg | `laps.dom.test.ts:490` | `laps.pw.mjs:94` |
| One comparison bar per lap | `laps.dom.test.ts:769` | `f71-compare.pw.mjs:8` |
| Units switch is live, no request | `page/units.dom.test.ts:97` | `prefs.pw.mjs:16,107` |
| How-to hint, × dismisses | `page/page.dom.test.ts:538,547` | `help-share.pw.mjs:9` |
| Range link selects the range | `page.dom.test.ts:636` | `help-share.pw.mjs:42` |
| Satellite only after the toggle | `components/map/map-view.dom.test.ts:1613` | `map-layers.pw.mjs:42` |
| Fullscreen, Esc leaves | `map-view.dom.test.ts:1461,1480` | `map.pw.mjs:69` |
| Storage full shows the paused notice; fail open | `upload/page.dom.test.ts:168,188` | `upload.pw.mjs:26,34` |

Keep both sides. The jsdom tests run in `make ci` and catch the regression within minutes. The Playwright tests run in real browsers and have caught real cross-browser bugs (B-22, the iPad layout). Deleting the jsdom side would leave these paths unchecked between nightly runs. Deleting the Playwright side would save about 16 tests (roughly 100 runs across the 7 projects) and would lose the only real-browser evidence. Neither is worth it.

The Go handler and integration tests are layered, not duplicates. For example, `internal/httpapi/upload_test.go:670` `TestServerErrorMapping` checks how errors map to status codes, and `cmd/workout-explorer/upload_test.go:541` `TestUploadServerError` checks that a failed upload leaves no row or file behind. `hiding.pw.mjs` overlaps the Go hiding corpus tests, but it covers UP-9, so it stays.

I found no snapshot tests. Weak assertions are rare (`toBeTruthy`, `toBeDefined` and bare `not.toThrow` appear 21 times in total).

## Gaps the overlap hides

- **YD-8:** a pin or an added lap on the edit page reads mi instead of yd on a swim leg. Only `f72-open-water.pw.mjs:64,80` records it, as `test.fail`, and only nightly. `edit.dom.test.ts` and `pins.dom.test.ts` have no yards case for pins or added laps, so a fix has no fast test. This matches QA-72 r1-1.
- **Speed:** the Go sweeps and corpora take about 4.5 of the 8.7 CPU minutes. They guard hostile input and privacy (`TestFramingSweep`, `TestAcceptDataSizeSweep`, `TestAddedLapsPrivacyCorpus`, `TestHidingCorpus`), so keep them in `make ci`. No test uses `testing.Short()`. A `-short` gate would make local runs faster without weakening CI.

## Recommendation (ranked)

1. Fix QA-AUD-r1-1, the flaky notify test (backend, small lane). Nothing to delete. Risk: none.
2. Fix QA-AUD-r1-2, the colouring loop: about 5 lines, saves about 1.5 s per run and removes the timeout. Risk: none.
3. Delete the 7 F-22 header tests in `security.pw.mjs` (about 15 lines). Risk: low, because `TestCaddy` checks the same headers more strictly.
4. Add a jsdom test for YD-8 when the bug is fixed. Fast coverage gained.
5. Optional: loosen `hygiene.test.ts:45-61` to the slot ids. Split `edit.dom.test.ts` (66 s) into 3 or 4 files so Vitest can run them in parallel. This saves wall time and deletes nothing.
6. Do not delete jsdom `*.dom.test.ts` files or mutation-killer tests. The mutant-named tests (32 mentions, mostly in ingest, hiding and pins) guard hostile input or UP-9.

Expected gain if all of this is done: about 20 tests and 30 lines removed, `make ci` stable, Vitest wall time down about 30 % from the split. The suite is not oversized for its scope. Its real problems are the two flaky tests.

## Questions

1. Should Playwright run all 7 projects every night? The F-76 r1 run had 409 passed and 94 skipped. The pure table and units tests (laps, prefs, f71, f72-yards) could run on chromium plus one phone only, which cuts about 25 % of UI time. The risk is missing WebKit or Firefox layout bugs. Suggestion: keep all projects in the nightly run, since time there is cheap.

## Coverage

Checked: test titles of the 6 largest jsdom files against all 29 `*.pw.mjs` titles; upload tests in Go httpapi vs cmd; Caddy header tests vs e2e; source-regex tests; per-test timings in Go and Vitest. Not checked: every one of the 2,566 Vitest cases one by one, the 12 Python deploy tests, a `make ui-test` run, and whether each mutant-named test still kills its mutant.
