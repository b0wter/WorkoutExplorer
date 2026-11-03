# QA F-26 Fullscreen map - round 1

- **Date:** 2026-11-03
- **Verdict:** not ready - QA-26-r1-1: in both two-column layouts (desktop, phone in landscape) the hidden charts' cursor line and range shade most likely paint over the fullscreen map, because the map slot's stacking context traps the overlay's `z-index`. Not reproduced in a browser (none available here). A one-minute desktop check confirms or closes it.
- **Inputs:** `features/F-26-fullscreen-map.md`; AP-15, AP-18, NFR Accessibility and Compatibility in the product spec; `architect/F-26-fullscreen-map.md` v0.1; ADR-038; decision log OQ-99. Reviewed commit c33de53 (working tree clean). MapLibre 6.14.0 `src/ui/control/fullscreen_control.ts` read for the real control's behaviour.

## Build and test results

- `make ci`: exit 0 (lint, test, build, size, caddy-check, docker-check ok).
- `make lint test size`: svelte-check clean; all Go packages ok; vitest **54 files, 1,402 tests passed**.
- Entry budget: `index.html` and `edit.html` at 56,903 / 60,000 B gzip, PASS.
- Static cost: `main-*.css` byte-identical to HEAD~1; `main-*.js` same size (135,903 B), differs only in the map chunk's hash. **0 B**, as FS-2 requires.
- Lazy cost: rebuilt HEAD~1 from `git archive` in the scratchpad. With `gzip -9`, map-view JS 286,602 to 287,479 B (+877), map-view CSS 10,796 to 10,847 B (+51): **+928 B**, under the 1,500 B cap. The engineer reported +937 B with the project's own gzip level. Both are within the cap.
- No browser or phone run, by me or by the engineer.

## Findings

### Major

- **ID:** QA-26-r1-1
- **Severity / kind:** Major / defect by CSS stacking rules (not reproduced: no browser available)
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.css:100-112`; `src/web/src/page/page.css` (`.has-map .map-slot` is `position: sticky` on desktop and `position: fixed` on short landscape, both with `z-index: auto`); `src/web/src/components/charts/MetricChart.svelte:90-125` (inline `visibility: visible` on `.wx-cursor-line` and `.wx-range-shade`)
- **Relates to:** AC 1 (no data field visible), AC 4 (rotating to landscape), FS-3, FS-4
- **Evidence:**
  - `position: sticky` and `position: fixed` always create a stacking context. The `z-index: 99999` of `.maplibregl-pseudo-fullscreen` therefore only counts inside `.map-slot`. In the root context, `.map-slot` is painted at layer auto/0, in tree order.
  - `#map-slot` comes before `#charts-slot` in `src/web/index.html:47-50`.
  - The chart cursor line (absolute, `will-change: transform`) and the range shade (absolute) are in the same layer, later in tree order. They set inline `visibility: visible`, which overrides `html.wx-fs body { visibility: hidden }`.
  - Result: they paint above the overlay. In fullscreen, a route scrub shows one vertical line per chart over the map, and a selected range shows translucent bands.
  - The one-column layout is not affected: unpinned, `.map-slot` is `relative`/auto and creates no context; pinned, it has `z-index: 3`.
  - The engineer's note ("paint under the opaque overlay") holds only for the one-column layout.
  - Minimal repro: open `/tmp/claude-1000/-home-b0wter-syncthing-work-workout-explorer/02f8d273-8be4-4162-b9c6-83a1f26c2a9e/scratchpad/f26-stacking.html` in any browser. A red line and a blue band over the green box confirm the defect.
- **Consequence:** On desktop, and on a phone rotated to landscape (AC 4), chart artefacts move over the fullscreen map while the viewer follows the route. The jsdom tests cannot catch this, because jsdom does not paint.
- **Settles it:** On desktop (≥1024 px), enter fullscreen, scrub the route, select a range. If no line or band shows, close this finding.

### Minor

- **ID:** QA-26-r1-2
- **Severity / kind:** Minor / risk (not reproduced)
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.css:104-107` (`scrollbar-gutter: stable`)
- **Relates to:** FS-4 ("nothing changes size"), AC 1
- **Evidence:** The rule is read, not run.
  - With classic (non-overlay) scrollbars, the reserved gutter narrows the initial containing block. The `width: 100%` fixed overlay would then stop one scrollbar-width short of the right edge, and the page background would show in that strip.
  - On a desktop page that has no scrollbar (two-column page no taller than the window), `stable` adds a gutter on entry. That resizes the page under the overlay (chart ResizeObservers fire) and resizes it again on exit.
  - Phones use overlay scrollbars and are not affected.
- **Consequence:** A cosmetic strip and an avoidable resize on desktop only. Check both in the same desktop session as r1-1.

- **ID:** QA-26-r1-3
- **Severity / kind:** Minor / risk (accepted by the engineer, not documented)
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.ts:356-364`
- **Relates to:** FS-7 ("the history entry is removed exactly once"), ADR-038 item 3
- **Evidence:** An exit by button or Esc calls `history.back()`, which is asynchronous. If the viewer re-enters before that traversal runs, the new `pushState` happens first. The queued back then lands on the new entry's predecessor and fires `popstate` while the new listener is live, so fullscreen exits again at once. The engineer listed this in the hand-off notes; it is not in the design doc or ADR-038's consequences.
- **Consequence:** A rapid exit and re-enter (a double tap on the button) can bounce out of fullscreen. Nothing is lost and history stays consistent. Either document it in ADR-038's consequences or ignore re-entry until the pending `popstate` arrives.

## Previous findings

None (round 1).

## Questions

- None for the architect. OQ-100 stays open as designed. Nothing for it was built: no `requestFullscreen` or `webkitEnterFullscreen` anywhere in `src/web/src`, and the control runs with `pseudo: true`, so MapLibre never calls the API.

## Coverage

**Traced and passing (jsdom, `map-view.dom.test.ts` "fullscreen (F-26, ADR-038)" plus the start-up test):**

- **AC 1, 2 (mechanism):** the control is added with `{ container, pseudo: true }` at top-right after the zoom control, and on enter the root class and the overlay class are set. Route, markers, zoom, attribution and the F-25 colour control (`addControl`, bottom-left, OQ-99) all live inside `div.route-map`, so FS-3's visible set holds by construction. The pin toggle and splitter are siblings and get hidden. No ancestor of the map has `transform`, `filter`, `contain`, `will-change` or `backdrop-filter` (FS-4 constraint checked in `page.css`).
- **AC 3 exit paths:** each of the button, Esc and `popstate` exits.
  - One `pushState` per entry. The button and Esc paths each call `history.back()` once. `popstate` calls none, because the `popped` flag skips it.
  - The listeners are removed on exit, and a later Esc or `popstate` does nothing.
  - Destroy while fullscreen exits before `map.remove()`.
  - The scroll guard re-applies the offset only if it moved.
  - The fake mirrors MapLibre 6.14.0's `_togglePseudoFullScreen` order (class, events, resize), which I checked against the real source.
- **AC 5:** the control exists only inside a mounted map. The existing presence tests (no map, no control) still pass.
- **UP-9 / AC-1 (edit token):**
  - `history.pushState(null, '')` takes no URL argument, so the fragment is unchanged and no new URL enters history. The state is `null`. The test asserts `[null, '']`.
  - The E-2 token scan (`edit.dom.test.ts`) and the hygiene scan (`hygiene.test.ts`) remove only the first exact occurrence of that literal from `map-view.ts`, so a second or URL-bearing call would still fail them.
  - No other `history`, `popstate` or `hashchange` use exists in `src/web/src`.
  - `lib/owner` is untouched.
- **Scrub path:** the diff in `map-view.ts` adds only the import, the `exitFullscreen` hook in teardown and the fullscreen block. The cursor listener, marker move, ADR-015 capture lock and the double-tap disable are unchanged.
- **Budgets:** static +0, lazy +928 B (cap 1,500).

**Not traceable without a device (manual, operator):**

- AC 1, 2 on iPhone Safari and Android Chrome.
- AC 3 by back gesture on both phones, from a scrolled position, pinned and unpinned.
- AC 4 rotation (and r1-1 in landscape).
- FS-6 gestures in fullscreen.
- ADR-016 gate 0 re-run in fullscreen on `ride-12h`.
- iOS scroll lock via `overflow: hidden` on `html`.
- The engineer's note that Esc which closes an open native `<select>` may also exit fullscreen was not checked. Browsers usually consume Esc in an open select popup.

**Not run:** any real browser; `-race` (no Go change); `make corpus` (no backend change).
