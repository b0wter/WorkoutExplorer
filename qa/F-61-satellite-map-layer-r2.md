# QA F-61 Satellite map layer - round 2

- **Date:** 2026-11-20
- **Verdict:** ready with conditions - no finding open. QA-61-r1-1 is fixed and the Esri check is recorded below. Conditions: the real-browser and on-phone hand checks from round 1 (AC 3, 6, 8, 9, 10, 12, 14) are still open.
- **Inputs:** `qa/F-61-satellite-map-layer-r1.md`. Diff `7f9aa21..3d64cf3` (F-61 part: `page/page.dom.test.ts`, `page/testing/stub-log.ts`). Branch `f-54-frontend`, clean tree.

## Build and test results
- `make ci` at `3d64cf3`: **exit 0**. Vitest 86 files, 2,228 passed. Entry 60,281 / 62,000.
- Mutant S7 (`page/start.ts:286` hands `undefined` instead of `prefs` to the map): **killed** (2 tests, including the new `map layer choice (F-61 AC 4)`).

## Findings
None.

## Previous findings

| ID | Status |
| --- | --- |
| QA-61-r1-1 `start.ts` prefs pass-through untested | **fixed.** The new test stores `wx.map=satellite`, opens the page, reads the layer back through the props the map receives, and checks that `setMapLayer('map')` reaches storage. S7 is killed |

## Questions
- r1 Q1 (Esri check, SL-7 "Verification"): **answered by the engineer, as reported to me, not re-run** (no live calls from QA). `curl -I` with an `Origin` header on a `World_Imagery` tile returned 200, `Access-Control-Allow-Origin: *`, no redirect and no key. That meets SL-7. The note is not yet in `architect/F-61-satellite-map-layer.md`. Adding a dated one-liner there (small lane) would make it findable.
- r1 Q2 (CSP site-wide): unchanged. It is read as met by ADR-014 rev. 4.

## Coverage
Only the diff and r1-1. Round 1's table stands. Hand checks still open: browser network log with Map and with Satellite (no Esri with Map, no Referer, no cookie); legibility in both themes on the phone; attribution after switching both ways; imagery blocked or offline; control order with the pin toggle and in fullscreen; 60 fps with Satellite on the 12-hour ride.
