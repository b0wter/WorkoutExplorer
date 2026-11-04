# QA F-33 Cursor readout in the fullscreen map - round 1

- **Date:** 2026-11-04
- **Verdict:** ready with conditions. No Blocker or Major. Conditions: the operator's device check of AC 1 to AC 5 (both phones and desktop, portrait and landscape) and the reference-phone scrub gate (F-26 "Gate"), both owned by the user per the design's verification notes. Two Minor findings.
- **Inputs:** `features/F-33-fullscreen-cursor-readout.md`; `architect/F-33-fullscreen-cursor-readout.md` v0.1 (CR-1 to CR-9); `architect/F-26-fullscreen-map.md` FS-3/FS-4; ADR-011, ADR-015, ADR-038; spec AP-21, AP-15, AP-6. Commit `bb594a2` (diff against `dbe0450`), clean tree.

## Build and test results

- `make ci`: exit 0. Go: all packages `ok`. Vitest: 58 files, 1,442 tests passed. Build, size, caddy-check, docker-check ok.
- Size (`check-bundle-size.mjs`): `entry index.html` and `edit.html` 57,388 / 60,000 gzip PASS.
- Baseline: `git archive dbe0450` into the scratchpad, built with the same `node_modules`: 57,159. F-33 costs **229 bytes gzip**, matching the ADR-011 row (planned 300).
- Not run: a real browser. No Chromium/Playwright is installed here and installing needs a download, so the layout and input checks are by code reading plus the engineer's headless figures in `bench/MEASUREMENTS.md`.

## Findings

### Minor

- **ID:** QA-33-r1-1
- **Severity / kind:** Minor / defect (document)
- **Route:** architect
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md`, item 4, Reserve row; header "Features"
- **Relates to:** ADR-011 item 4 and 5 (reserve and review trigger)
- **Evidence:** the row says 3,633 (60,000 − 56,367 at rev. 12). Measured entry before F-33 is 57,159, after it is 57,388. So the real reserve is **2,612**. 792 bytes were spent between F-20 r2 and `dbe0450` and never booked to a row (F-25, F-26, F-28 call sites or follow-ups). The F-33 row is also not in the header's "Features" list. The engineer's claim is confirmed.
- **Consequence:** the ledger overstates the headroom by 1,021 bytes. It is still above the 1,000 trigger, but F-32 and later shares would be planned against a wrong figure.

- **ID:** QA-33-r1-2
- **Severity / kind:** Minor / risk (tests)
- **Route:** frontend
- **Where:** `src/web/src/page/strip.ts:30-32`, `src/web/src/components/map/map-view.css:20-25`, `src/web/src/page/strip.dom.test.ts`
- **Relates to:** AC 3, CR-4
- **Evidence:** jsdom has no `ResizeObserver`, so `mountStrip` skips it and no test reaches `--wx-strip` or the corner rule. The CSS test checks `display` in and out of fullscreen. Nothing checks that the strip is still hidden in fullscreen without a cursor: that relies on the UA `[hidden]` rule because `.wx-strip` sets no `display`. AC 3 is only covered by the engineer's headless layout run.
- **Consequence:** two regressions would pass CI unnoticed: a `display:` added to `.wx-strip` (the strip would show "–" cells without a cursor in fullscreen), or a broken observer (the bottom controls would sit under the strip on phone portrait). Fix: a stub `ResizeObserver` test that asserts `--wx-strip` on the container, plus a computed-style check of `[hidden]` under `html.wx-fs`.

## Previous findings

None (first round).

## Questions

None.

## Coverage

| AC | Code | Test / verification |
| --- | --- | --- |
| 1 Strip in fullscreen, same values | `strip.ts` (second `CursorReadout`, same store and units store), `start.ts:147-148` (mount into `.route-map` on map-ready), `page.css` (same tracks) | `strip.dom.test.ts` (same text as the page readout, also after a units change); `page.dom.test.ts` (mounted only after `onmapready`, none without a route); engineer headless run (values equal). Device check: operator |
| 2 No cursor, no strip | `strip.ts` subscribe toggles `hidden` on the transition only | `strip.dom.test.ts` (hidden until set, hidden again on null). No code sets the cursor back to null (grep), so the strip appears once and then stays (OQ-13) |
| 3 Controls visible and usable | `map-view.css` lifts the bottom corners by `--wx-strip`; the two-column media rule (same specificity, later) still moves them to the top on desktop and in short landscape; MapLibre corners have `z-index: 2`, above the strip's `sticky z-index: 1` | No automated test (QA-33-r1-2). Engineer headless layout: no overlap at 320×568, 568×320, 844×390, desktop. Device check: operator |
| 4 No range panel in fullscreen | `#range-slot` is outside `.route-map`, hidden by FS-4 `body { visibility: hidden }` | By construction; no F-33 test. Device check with a range selected: operator |
| 5 Leaving fullscreen, F-26 ACs | `html:not(.wx-fs) .wx-strip { display: none }`; page readout untouched | `strip.dom.test.ts` CSS test; F-26 tests in `map-view.dom.test.ts` pass |

Also checked and found correct:
- **Input:** `pointer-events: none` on `.wx-strip`. No descendant sets it back (grep). Hover and touch go to the canvas, so ADR-015 is unchanged (CR-7).
- **Per-frame work:** the strip listener only compares a boolean. The readout writes `textContent` (D-10). The `ResizeObserver` fires only on a size change, and the fixed tracks keep the height the same while scrubbing. The engineer's recorder figures are in `MEASUREMENTS.md`; the reference-phone gate is the operator's.
- **Accessibility:** outside fullscreen the strip is `display: none`; inside, the page readout is `visibility: hidden`. So exactly one readout is in the tree. No `aria-live`, nothing focusable.
- **Units remount:** the old instance unsubscribes from the cursor on unmount (`onMount` return). The units store does not call on subscribe, so there is no double mount at start.
- **Lifetime:** `data` is fixed for the page, so `{#key data}` never re-keys the map and `onmapready` runs once. The never-released subscriptions do not leak.
- **Safe area:** the viewport meta has no `viewport-fit=cover`, so the strip cannot sit under a notch in landscape. This settles the engineer's open point at code level.
- **Edit page:** `ActivityPage.svelte` renders the same `#map-slot`, so the glue and the CSS apply to it.
- **Privacy:** nothing new is loaded, stored or logged.

Not run: any browser check (hover, touch, notch, edit page, range-then-fullscreen), the reference-phone gate.
