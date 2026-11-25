# QA F-76 Taller charts - round 1

- **Date:** 2026-11-25
- **Verdict:** ready with conditions. There are no Blocker or Major findings. Conditions: the ADR-016 60 fps record and the LCP check at the tall size on ride-12h were not run (AC 7, AC 10), and the landscape cap and pinned map were not hand-checked on a real phone.
- **Inputs:** `features/F-76-taller-charts.md`; spec DS-8; `architect/F-76-taller-charts.md` TC-1 to TC-12; ADR-011 ledger (rev. 17 bookings); F-73 RS-2/RS-6 (head script, hash rotation); F-13 D-10 note. Reviewed main at 97a9a1e (merge of f361407 and 239c556), fast-forwarded in worktree `agent-a0361cf5ba3987dd7`.

## Build and test results

- `make ci`: exit 0. Vitest 98 files and 2,563 tests passed. Activity entry 59,438 / 62,000 gzip on index, edit and my-workouts. The ledger books +322 over 59,116 (ceiling 400, reserve 2,562). I did not re-measure the baseline myself. The Caddy, docker and update checks passed.
- `make ui-test` (all 7 projects, build/ui-test, free ports): 409 passed, 94 skipped and 1 flaky. The flaky test was `[iPad] dialogs.pw.mjs:6` (2.0 min, "Target page … closed"; it passed on retry). Every F-76 test passed in every project that runs it.
- `make ui-test PROJECT=iPad UI_ARGS="legs.pw.mjs:42 --repeat-each=60 --retries=0"` (4 workers): 4 of 60 failed and **4 core dumps** were written to `src/web/e2e/`. With `--workers=1` the same run passed 60 of 60 (host load was lower by then). The core dumps are deleted.
- Head script: the inline script in all six shells (index, edit, generic, upload, my-workouts, about) hashes to `sha256-mb97ihup…Q2w=`. That is the first hash after `'self'` in `deploy/Caddyfile` and `caddy_test.go`. The F-73 hash `QnlpnggjN…` is second, and the older three are kept.

## Findings

### Minor

- **ID:** QA-76-r1-1
- **Severity / kind:** Minor / defect (missing evidence)
- **Route:** frontend
- **Where:** none (no record exists)
- **Relates to:** AC 7 (60 fps on ride-12h at tall), AC 10 (LCP unchanged), the architecture's "What tests must prove" (ADR-016 record at the tall size)
- **Evidence:** The engineer reports that neither was measured. I could not find an ADR-016 record for 200 px.
- **Consequence:** Two ACs are open. The risk is low: the scrub path is DOM transforms, and the height-only change is one `setSize` per chart. Still, a 200 px canvas is 67 % more pixels per redraw, and nobody has measured it.

- **ID:** QA-76-r1-2
- **Severity / kind:** Minor / test weakness
- **Route:** frontend
- **Where:** `src/web/e2e/f76-chart-size.pw.mjs:192` (no-jump test), `:81` (blocked storage)
- **Relates to:** AC 5 (cached and uncached load), AC 4 ("no console error")
- **Evidence:**
  - The no-jump test does one load only. There is no second, cached load.
  - CLS is asserted on Chromium only (`__clsOk`). WebKit and the phone projects check only the position of `section.laps`.
  - The blocked-storage test filters out every error matching `/localStorage|blocked/`. A real bundle error about blocked storage would be filtered out too. The pattern is copied from F-73.
- **Consequence:** A cached-load shift, or an uncaught storage error from the bundle, would pass the suite.

- **ID:** QA-76-r1-3
- **Severity / kind:** Minor / risk (not reproduced)
- **Route:** frontend
- **Where:** `src/web/src/components/charts/ChartStack.svelte:121-141` (`readPlot` runs only from the width ResizeObserver, at mount and after a toggle)
- **Relates to:** TC-5, D-10
- **Evidence:** The landscape cap depends on `svh`/`vh`. A viewport change in height only that crosses the cap leaves the stack's box unchanged, so no ResizeObserver callback fires and `--plot` is not re-read. The CSS reservation then changes while the canvases keep the old height until the next width change. One example: a desktop window ≤ 1023 px wide whose height is dragged below about 300 px. On phones, rotation always changes the width, and `svh` does not move with the browser bar, so phones are not affected. I did not probe this in a browser.
- **Consequence:** In a rare desktop resize, charts overlap or leave a gap until the next width change. TC-5 accepted this as "rotation always changes width". Write it down, or also re-read `--plot` on `resize`.

- **ID:** QA-76-r1-4
- **Severity / kind:** Minor / test issue (judgement on the open iPad finding)
- **Route:** frontend (suite)
- **Where:** `src/web/e2e/legs.pw.mjs:42`
- **Relates to:** F-67 LP-3/LP-6; not F-76
- **Evidence:**
  - With 4 workers, the run failed 4 of 60 and wrote exactly 4 core dumps from `/ms-playwright/webkit-2104/minibrowser-wpe/bin/WPEWebProcess`. All four have `si_signo 5` (SIGTRAP, WebKit's `CRASH()`/release-assert path) and are about 200 MB resident.
  - The trace of a failure shows the page stalled for minutes: `selectOption` started at 25 s, resolved at 51 s and completed at 114 s. The test timed out twice, and then "page … closed".
  - No JavaScript `pageerror` was recorded.
  - "Titles without canvases" is the frame from the moment the web process died during the leg remount.
  - With `--workers=1` the same run passed 60 of 60.
  - The full-suite iPad dialogs flake has the same 2.0 min signature.
- **Consequence:** This is a crash of Playwright's WPE WebKit build under parallel load. It is not an app draw bug, because the app code throws nothing. A residual product risk would be settled by switching legs on a real iPad in Safari. Also, the test title says "shows only its charts", but the test asserts only that the first chart is visible, so it cannot check that claim. Suggested fix: rename the test or assert the chart set. Optionally run the iPad project with fewer workers. Keep `retries: 1`.

## Previous findings

First round. QA-75-r1-1 (wait for the picker) is in `legs.pw.mjs:44`. It passed in every run above.

## Questions

1. QA-76-r1-3: is "height-only resize across the landscape cap is not followed" accepted? If yes, add one line to TC-5. If no, add a `resize` listener.
2. QA-76-r1-4: is a real-iPad hand check of the leg switch wanted before closing the F-67 flake as a test-environment issue? Suggested answer: yes, once, with the next device check.

## Coverage

- **Traced to code and a passing test:**
  - AC 1, AC 2 and AC 3: e2e toggle, comparison chart, remember, edit page, no request contains the key, URL clean.
  - AC 4: e2e blocked storage and the `Tall` value; head-script stand-ins in `theme-script.test.ts`.
  - AC 5: e2e position plus Chromium CLS; see r1-2.
  - AC 6: e2e 812×375 and 812×260 caps; the cap lives only inside `LANDSCAPE_QUERY` (hygiene test); `vh` comes before `svh`.
  - AC 8: label fit at 360 px in four languages on Chromium and WebKit; About text in en/de/es/fr.
  - AC 9: the leg view keeps the size (e2e); themes through CSS tokens.
  - AC 11: CSP checked on viewing, edit, upload and About in e2e. The generic page is covered by the identical-hash check. JavaScript off gives 120 px.
  - AC 12: `f76-chart-size.pw.mjs`.
  - TC-6: the DOM test proves one `setSize` per chart, no new instance, and that selection and cursor survive a toggle.
- **Design mutants, killed by existing assertions (read, not executed):**
  - statement order: theme-script test
  - `--plot` in `{#key}`: the "no new instance" assertion
  - cap outside the query: hygiene test
  - F-73 hash dropped: theme-script test
- **Deviations accepted:**
  - `svh` in the cap. F-13 D-10 is annotated. The reason D-10 gives (browser bars move the unit) does not apply to `svh`, and the hygiene test still forbids it everywhere else.
  - `--n` per `.charts-N`, `onsize` wired in start.ts.
  - The viewport matrix runs on Chromium and WebKit only.
- **Not run:**
  - 60 fps and LCP at tall on ride-12h (r1-1).
  - Real-phone check of the pinned map in portrait at tall. My arithmetic for the iPhone 13 mini with bars, 35lvh map and a 3-row readout leaves about 270 px, which is at least 224 px, so one chart fits.
  - The baseline entry size was not re-measured.
