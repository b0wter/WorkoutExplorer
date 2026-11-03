# QA F-28 Report form - round 1

- **Date:** 2026-11-03
- **Verdict:** ready with conditions. No Blocker or Major. Condition: the browser checks in the design's Verification table (AC 1 on a phone and a desktop, keyboard and send button, Esc and back, no CSP violation, one real report and one `429` through Caddy). jsdom stubs `showModal`, so AC 1 is traced to code only.
- **Inputs:** `features/F-28-report-form.md`, INF-2, DS-3; `architect/F-28-report-form.md` (RF-1 to RF-10); `architect/F-27-reports-api-and-operator-delete.md` (R-5, Responses); decision log 3 Nov 2026 (OQ-103, OQ-104); `src/server/internal/httpapi/report.go`, `src/server/internal/reports/reports.go`, `deploy/Caddyfile`; commit 6094082 (`git show HEAD`), clean tree.

## Build and test results

- `make ci`: exit 0 (2m55s). Go: all packages `ok`. Vitest: 55 files, 1430 tests passed. `npm run size`: entry `index.html` 57,159 / 60,000 B gzip PASS; upload entry 2,272 / 8,000 PASS. `caddy-check` (Caddy 2.11.7): ok. `docker-check`: ok.
- Static delta (RF-2, ceiling 300): built HEAD~1's `src/web` in the scratchpad: entry 56,902 B gzip, so **+257 B**. Confirmed.
- Lazy share (RF-3, ceiling 3,000): `form-*.js` 1,192 + `form-*.css` 450 + shared `text-*.js` 104 = **1,746 B gzip**. Confirmed.
- `build/web/index.html` has no preload of `form-*` or `text-*`; `main-*.js` contains no `/api/` string.
- Validation probe (scratchpad, not committed): 12 edge-case messages run through the client path (cut to 1,000 UTF-16 units as `maxlength` does, then `cleanText(…, true)`, then the exact `JSON.stringify` of `form.ts`), and the bodies fed to the server's `parseReport` and `reports.Check` via a `go test -overlay` file. Cases: 1,000 ASCII, 500 emoji, `'a'` + 500 emoji (cut splits a surrogate pair), every Cc character x20, 1,000 lone surrogates, quotes and backslashes, CR/CRLF, U+0085, U+2028/2029/200B/FEFF, whitespace only, bidi controls, empty. **All 12 accepted**; largest body 6,068 B (lone surrogates), well under the 16 KiB limit.

## Findings

### Minor

- **ID:** QA-28-r1-1
- **Severity / kind:** Minor / risk (accessibility)
- **Route:** frontend
- **Where:** `src/web/src/report/form.ts:62-75`
- **Relates to:** AC 2 (confirmation), RF-7
- **Evidence:** read. While sending, the focused send button is disabled; on `204` its form is hidden and the confirmation `div[role=status]` is unhidden with its text already in place. Focus is not moved anywhere (not to Close, not to the confirmation). Unhiding a live region whose text was already present is announced inconsistently by screen readers; a region must be in the accessibility tree before its content changes. The error path (`role=alert`, text set) is fine. Not reproducible in jsdom; needs a screen-reader check.
- **Consequence:** a keyboard or screen-reader user may get no feedback that the report was sent, and focus lands on the dialog body. Cheapest fix: focus the Close button after showing the confirmation (and the send button again after an error).

- **ID:** QA-28-r1-2
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/components/edit/fields.ts:25`
- **Relates to:** RF-4 ("one shared function, not a copy")
- **Evidence:** `export { cleanText };` is used only by `fields.test.ts:4`; no production code imports `cleanText` from `fields`.
- **Consequence:** a re-export kept for a test, so two import paths for one function. The test should import from `lib/text` and the re-export go.

## Engineer's deviations

1. **Go test changes:** accepted. `protoserve` and `hygiene.test.ts` now require exactly one `<button`, and the exact Report markup, so a pin toggle in the server markup still fails both. The `caddy_test.go` exemption adds `src/report/` beside `src/edit/`: exempt chunks are neither scanned nor followed, so the shared `text-*.js` (reached only through exempt chunks) is not scanned; that is the same shape as the edit precedent and the chunk is 104 B of regex. The property it guards (the page load names no API) still holds: the entry and every eagerly reached chunk are scanned, and a static import of `src/report` would be inlined into `main-*.js` and fail the scan. "Loaded only on click" is asserted by `enhance.dom.test.ts` ("the chunk loads only on click").
2. **`cleanText` moved to `src/lib/text`:** accepted (keeps the edit panel's rules out of the report chunk). Cost: one extra 104 B request on first Report click. See QA-28-r1-2 for the leftover re-export.
3. **Counter in UTF-16 units:** this is RF-4 itself, not a deviation. Probe confirms it is never looser than the server.
4. **Reset on close:** matches RF-7 (form cleared on `204`, confirmation swapped back on close by any means: Close, Esc, back). If the viewer closes the dialog while a send is in flight and it succeeds, the next open shows the confirmation first; acceptable.
5. **Dialog width:** matches RF-10 (`calc(100% - 16px)` below 1,024 px, 480 px above; textarea 16 px). Theme variables are on `:root`, so the dialog appended to `body` gets them.
6. **No `interactive-widget` meta:** agreed; adding it would change the whole page's viewport (RF-10 says no layout change). Whether the send button stays reachable with the keyboard open is part of the manual condition.
7. **Sizes:** confirmed above (+257 B static, 1,746 B lazy).

## Previous findings

None (round 1).

## Questions

None.

## Coverage

| Check | Code | Test / evidence |
| --- | --- | --- |
| AC 1 opens without leaving the page | `enhance.ts` `enhanceReport`, `form.ts` `openReport` (`showModal`) | button unhidden, chunk on click (`enhance.dom.test.ts`); works with data file hanging and failed (`page.dom.test.ts`). **Real `showModal`, phone and desktop: manual, not run** |
| AC 2 reason required; reason only gives confirmation | `required` radio, guard at `form.ts:61` | "no reason: nothing is sent", "a reason and no message: exactly the three keys, then the confirmation" |
| AC 3 stops at 1,000 | `maxlength="1000"`, counter | form test (maxLength, counter in UTF-16), probe above |
| AC 4 no contact fields | `form.ts` HTML | field list is exactly 4 radios + 1 textarea |
| AC 5 plain error, text kept | `post`, `form.ts:66-70` | 10 cases: 400/404/429/500 with message, 500 non-string message, 400 `null`, 502 non-JSON, 503 JSON, 200, network failure; reason and text kept; retry succeeds |
| Body exactly `public_id`, `reason`, `message` | `form.ts:85-89` | `toEqual` on the whole `RequestInit` (no credentials, referrer or extra key) |
| `public_id` from view path, not `location` | copy link `href` last segment, `^[0-9A-Za-z]{22}$` (same rule as `lifecycle.ValidPublicID`) | 4 invalid hrefs and missing copy link keep it hidden; no `location` in `enhanceReport` or `src/report` (grep) |
| Nothing stored or logged | no storage, cookie or console call in `src/report`, `lib/text` (grep) | test checks storage, cookie, `console.log/warn` |
| Hidden on edit and owner pages | `ActivityPage` `report=false` (`edit/start.svelte.ts:185`); `PublishedView.svelte:65` footer has About only | `edit.dom.test.ts:214`; published page: by markup only, no test asserts the absence (structurally impossible today) |
| No JavaScript: no Report control (OQ-103) | `hidden` in `index.html` and `PageFooter.svelte` | `hygiene.test.ts`, `protoserve` samples test |
| CSP | no inline style or script; CSS via `<link>` from `'self'`; `connect-src 'self'` | **browser console check: manual, not run** |

Not run: any real browser, screen reader, or the live Caddy/Go stack beyond `caddy-check`.
