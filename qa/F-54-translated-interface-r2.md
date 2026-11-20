# QA F-54 Translated interface: German, Spanish, French - round 2

- **Date:** 2026-11-20
- **Verdict:** ready with conditions - QA-54-r1-1 (Major) is fixed and its mutants are killed. No Blocker or Major is open. Conditions: the hand checks from round 1 are still open (network view, throttled German first visit, LCP, phone portrait, on-screen review OQ-225).
- **Inputs:** `qa/F-54-translated-interface-r1.md`. Diff `7f9aa21..3d64cf3` (branch `f-54-frontend`, clean tree). Follow-up round: only the diff and the r1 findings were reviewed.

## Build and test results
- `make ci` at `3d64cf3`: **exit 0**. Vitest 86 files, **2,228 tests passed**. Go ok. Activity entry 60,281 / 62,000 (unchanged). Upload entry 5,452 / 8,000 (+115). Static entry 824 / 2,000. Largest language file `lang-static-de` 4,060 / 5,120. (My first `make ci` run hit a 5 s timeout in `colouring.test.ts` ride-12h because my mutant runs were using the CPU at the same time. Run on its own, it passes.)
- Mutants, run on a copy in the scratchpad, against `src/page src/upload src/i18n src/lib/upload src/about src/static`. The baseline copy has one failure caused by the copy itself (the Caddyfile path in `theme-script.test.ts`). The counts below exclude it.

| # | Mutant | Result |
| --- | --- | --- |
| U1 | `rules.ts`: drop the upload's own texts (`serverText(v)` again), the r1-1 defect | killed (5 tests: rules and upload page) |
| U2 | `rateLimited` reads "Too many requests…" | killed (2) |
| U3 | `busyNow` reads "The server is busy…" | killed (2) |
| M4b | `const label = t('paused'); notice.innerHTML = label` in `upload/page.ts` | killed (new allowlist test `only the listed files assign HTML`) |
| M4c | `el.innerHTML = value` in `lib/i18n/load.ts` | killed (allowlist) |
| R1 | upload page: `onLanguage(draw)` removed (no redraw on a switch) | **survived** (QA-54-r2-1) |
| R2 | About `<title>` without `data-t="ab_title"` | killed (4: the language files have a key that English lacks) |

## Findings

### Minor

- **ID:** QA-54-r2-1
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/upload/page.ts:50` (`const offLang = onLanguage(draw)`), `src/web/src/upload/page.dom.test.ts`
- **Relates to:** AC "A choice applies at once", r1-3 fix
- **Evidence:** mutant R1 removes the subscription and all tests still pass. No test shows a message, switches the language and checks the line again.
- **Consequence:** the r1-3 fix for the upload page can regress without any test failing.

- **ID:** QA-54-r2-2 (the part of QA-54-r1-3 still open)
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:44-56` (`showLoadError` writes the text once), `:205`, `:231`
- **Relates to:** AC "A choice applies at once"
- **Evidence:** read, not probed. The load-error line is drawn once with `t()` at the moment of failure. A later switch does not redraw it. The same applies to the first load if the data file fails (for example a fast 404) before the language file arrives: the error stays English while `applyMarkup` translates the rest of the page.
- **Consequence:** this happens only on a failed load, a rare page with one sentence. The page can mix two languages, but nothing is lost or exposed. It stays Minor.

- **ID:** QA-54-r2-3 (the part of QA-54-r1-4 still open)
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/i18n/i18n.dom.test.ts:163` (`HTML_ASSIGNERS`)
- **Evidence:** the allowlist closes the variable form in any new file (M4b, M4c killed). A text passed through a variable inside one of the 8 listed files still passes. Text that TypeScript builds with `textContent` or `setAttribute` is still not scanned. Both gaps are recorded in a code comment ("Gap (F-62)") but not in LG-7 or LG-8.
- **Consequence:** as in r1. This matters for F-62.

## Previous findings

| ID | Status |
| --- | --- |
| QA-54-r1-1 (Major) UP-10 sentences | **fixed.** `rate_limited`, `busy` and `too_large` map to the upload page's own texts (`rules.ts:70-72`). The rules and page tests now send "server words" and assert the UP-10 sentence. de/es/fr have `rateLimited` and `busyNow`. U1 to U3 killed |
| QA-54-r1-2 About `<title>` | **fixed.** `data-t="ab_title"` is set, and `applyMarkup` covers `<head>` (it runs on the whole `document`). de "Info", es "Acerca de", fr "À propos". The `vite.config.ts` `<title` match still requires exactly one title. R2 killed |
| QA-54-r1-3 messages not redrawn | **partly fixed.** Upload notice, progress and message are now functions that are redrawn on `onLanguage`, and `offLang` is called on teardown. The recomputed outcome is pure, so a redraw is safe. The redraw is untested (r2-1). The viewing page's load error is still open (r2-2) |
| QA-54-r1-4 innerHTML guard per line | **partly fixed.** File allowlist added (M4b killed). Residual: r2-3 |
| QA-54-r1-5 no hold on edit / published / My workouts | **fixed.** Recorded in LG-2 "As built" |

## Questions
None.

## Coverage
Only the diff and the r1 findings. Round 1's coverage table stands for everything else. Hand checks still open: network view per page; throttled German first visit; LCP in German on the reference phone; phone portrait in the longest language; on-screen review (OQ-225); cursor and range kept across a switch; About tab title in each language (browser).
