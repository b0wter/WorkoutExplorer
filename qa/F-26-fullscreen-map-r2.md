# QA F-26 Fullscreen map - round 2

- **Date:** 2026-11-03
- **Verdict:** ready with conditions. All three round-1 findings are fixed in code and no new finding is open. The condition: nobody has run it in a browser yet (none is installed here). One desktop check and the phone checks from round 1 are still needed (see Coverage).
- **Inputs:** `qa/F-26-fullscreen-map-r1.md`; diff of commit 16b473c (`git show HEAD`, working tree clean); `architect/F-26-fullscreen-map.md`, ADR-038; NFR Compatibility in the product spec; the engineer's probe `/tmp/claude-1000/-home-b0wter-syncthing-work-workout-explorer/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/f26-stacking-fixed.html`.

## Build and test results

- `make ci`: exit 0 (lint, test, build, size, caddy-check, docker-check ok).
- svelte-check: clean with `--fail-on-warnings`. vitest: **54 files, 1,404 tests passed** (the 2 new tests are included).
- Entry budget: unchanged. `main-*.js` is 135.90 kB raw / 53.72 kB gzip, the same as round 1.
- Lazy cost with `gzip -9`, against the round-1 baseline (HEAD~2):
  - map-view JS: 286,602 to 287,560 B (+958).
  - map-view CSS: 10,796 to 10,849 B (+53).
  - Total **+1,011 B** of the 1,500 B cap, which matches the engineer's figure.
- The built CSS still contains `html.wx-fs :has(.maplibregl-pseudo-fullscreen){z-index:2147483647}`; the build did not drop or rewrite the rule.
- No headless browser is available (`chromium`, `google-chrome` and `firefox` are not installed, and there is no Playwright cache). The probe page was read but not rendered.

## Findings

None.

## Previous findings

- **QA-26-r1-1 (Major): fixed in code, not confirmed in a browser.**
  - `html.wx-fs :has(.maplibregl-pseudo-fullscreen)` matches every ancestor of the map container.
  - In both two-column layouts the map slot is positioned (`sticky` on desktop, `fixed` in short landscape), so the `z-index` applies. The slot is then painted in the root stacking context at the maximum layer, above the charts' cursor line and range shade, which are at layer 0. Earlier tree order no longer matters.
  - On static ancestors that are not flex or grid items, `z-index` has no effect, so the rule does no harm there.
  - One-column pinned: `.layout.pinned .map-slot` (specificity 0,3,0) beats the new rule (0,2,1) and keeps `z-index: 3`. This still works: no other `z-index` in `src/web/src` is above 2 (`page.css` 327, 505, 655 and `CursorReadout.svelte` 59).
  - `:has()` is supported by the last two versions of every browser the NFR Compatibility row lists.
- **QA-26-r1-2 (Minor): fixed.**
  - `scrollbar-gutter` is gone.
  - On entry, the inline `padding-right` is set to `innerWidth - clientWidth`. This is 0 with overlay scrollbars or with no scrollbar, so a page without a scrollbar no longer resizes.
  - The fixed overlay is sized from the viewport, so the root's padding does not narrow it and no strip shows.
  - The measurement is taken after MapLibre has made the container `fixed`. That does not change the page height, because `.map-slot` has an explicit height in every layout.
  - No other code sets `padding-right` on the root, so resetting it to `''` on exit loses nothing.
  - The test sets up a 15 px scrollbar and checks the padding is set on entry and cleared on exit.
- **QA-26-r1-3 (Minor): fixed.**
  - A capture listener on the button stops the click while `backPending` is set. Capture listeners run before target-phase bubble listeners in current browsers, so MapLibre's toggle never runs.
  - `backPending` is set only on an exit that calls `history.back()`. A one-time `popstate` listener clears it.
  - While fullscreen is on, `backPending` is always false, so Esc, back and destroy (all through `press()`) are never blocked.
  - The new test fails without the guard: the third click would call `pushState` a second time.

## Regression checks

- **Exit paths:** the button, Esc, `popstate` and destroy keep their order. `popped` still skips `back()` on the `popstate` path.
  - In the shared fixture, the `back` mock now fires `popstate` at once. This does not weaken the earlier assertions, because `onPop` is removed before `back()` runs, and the counts of `back()` and `pushState` are still asserted.
- **Token rule (UP-9 / AC-1):** the only history writes in `src/web/src` are `history.pushState(null, '')` (map-view.ts:357) and `history.back()` (372). The E-2 and hygiene scans pass in `make ci`.
- **Scrub path:** the cursor listener, marker move, capture lock and double-tap disable are not touched by the diff.

## Questions

None.

## Coverage

**Conditions (manual, operator):**
- Desktop (≥1024 px): enter fullscreen, scrub the route, select a range, and check that no line or band shows over the map. This closes r1-1 for good. With classic scrollbars, check that the map has no strip on the right and that the page has not moved on exit.
- The phone items from round 1: AC 1-4 on iPhone Safari and Android Chrome, AC 4 rotation (including r1-1 in landscape), FS-6 gestures, ADR-016 gate 0 in fullscreen on `ride-12h`.

**Not checked:**
- The style-matching cost of a rule whose rightmost part is `:has()`, on the scrub path. I judged it negligible: inline transform changes do not trigger `:has` invalidation, and the gate 0 re-run above covers it.
- Any real browser.
- `-race` and `make corpus`: there is no Go change.
