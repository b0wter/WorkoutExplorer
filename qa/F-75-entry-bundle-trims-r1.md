# QA F-75 Entry bundle trims - round 1

- **Date:** 2026-11-25
- **Verdict:** ready with conditions. No Blocker or Major. The `legs.pw.mjs:42` failures come from a race in the test that was already there at baseline f2c4574. F-75 is not the cause. Conditions: LCP and 60 fps on ride-12h, plus the AC 2 checks in a real browser (pinned map on a phone, both themes), were not measured.
- **Inputs:** `features/F-75-entry-bundle-trims.md`; ADR-011 (decision 7, ledger); `architect/F-17-…` and `architect/F-25-…` deviation lines. Reviewed branch `worktree-agent-a570f06ed2693d86d` (079280d, af9042d, 8aac01e, 46a0430) merged onto main b72fa72. Baseline is f2c4574.

## Build and test results

- `make ci`: exit 0. vitest 98 files, 2,557 tests passed. Entry `index.html` and `edit.html` are **59,100 / 62,000** gzip, PASS (reserve 2,900, matching the ledger). Lazy: `range-panel` 1,614 + `range` 686 + css 305 gzip; `map-view` 9,531 js + 1,888 css.
- `make ui-test` (F-75): exit 1. 301 passed, 59 skipped, 3 flaky (webkit `f71-compare:28`, webkit and iPad `legs:42`), 1 failed (iPhone `legs:42`, both tries). All 4 F-75 e2e tests (`f75-lazy.pw.mjs`) passed.
- `legs.pw.mjs` "F-67: the picker…", `--project=webkit,iPhone,iPad --repeat-each=15 --retries=0`, own build and ports:

  | Build | Run 1 failed | Run 2 failed | Total |
  | --- | --- | --- | --- |
  | baseline f2c4574 | 20/45 | 24/45 | 44/90 (49 %) |
  | F-75 | 25/45 | 29/45 | 54/90 (60 %) |

- No core dumps found.

## Findings

### Minor

- **ID:** QA-75-r1-1
- **Severity / kind:** Minor / defect (test)
- **Route:** frontend
- **Where:** `src/web/e2e/legs.pw.mjs:44`
- **Relates to:** F-67 LP-1, LP-2 (UI suite)
- **Evidence:** `texts()` calls `evaluateAll`, which does not wait. The picker comes from the lazy `sections` chunk loaded after the charts are ready, but `open()` only waits for the charts. Every failure receives `[]`. The test fails about 49 % of the time at baseline and about 60 % with F-75, on WebKit only. The extra `range-panel` chunk requested at the same `onready` may slow the sections chunk a little. That difference (z ≈ 1.5 over 90 runs each) is within noise. The earlier green baseline run was luck.
- **Consequence:** The suite is red at random on WebKit. This is not a product defect: the picker coming late is accepted lazy behaviour. Fix: wait first, for example with `await expect(page.locator('.wx-leg-pick option')).toHaveText([...])`. `f71-compare.pw.mjs:28` (flaky on webkit) is worth checking for the same pattern.

- **ID:** QA-75-r1-2
- **Severity / kind:** Minor / defect (test)
- **Route:** frontend
- **Where:** `src/web/src/page/page.dom.test.ts` "lap rows are tappable only once the range panel is mounted (L-7, QA-18-r1-3)"
- **Relates to:** F-75 AC 2 ("a … lap tap before the range chunk arrives shows the panel when it lands")
- **Evidence:** Lap rows now go live in `ready()`, before the panel chunk lands, so the test name no longer describes the code. The test also waits for the panel before it taps, so it never covers a tap before the chunk lands. The e2e test `f75-lazy.pw.mjs:6` cannot control whether the chunk has already landed. The range-link tests do cover a selection set before the chunk lands, so this is a gap in test coverage, not a known bug.
- **Consequence:** A regression where an early lap tap is lost would go unnoticed. Fix: rename the test, and tap right after `onready()` before awaiting, then wait for the visible panel.

- **ID:** QA-75-r1-3
- **Severity / kind:** Minor / risk (behaviour change, not written down)
- **Route:** frontend (one line in ADR-011 decision 7 or F-13 D-17)
- **Where:** `src/web/src/page/start.ts:245-251`
- **Relates to:** AP-17, F-13 D-17, ADR-011 decision 7 T-2
- **Evidence:** `addSplitter` used to run whenever a route was mounted. It now runs only from `onmapready`. If the map never becomes ready (no WebGL, MapLibre error), a two-column page has no splitter at all. ADR-011 accepts the handle appearing "about one round trip later", but not that it is missing for good.
- **Consequence:** On a failed map, the user cannot resize the empty or error map column. Accept it and write it down, or keep it.

- **ID:** QA-75-r1-4
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:246`
- **Evidence:** `void import('../components/map/map-view').then(...)` has no rejection handler. Every other lazy import in the file has one (`console.warn`). The module is already loaded at that point, so in practice it cannot fail.
- **Consequence:** Inconsistent with the rest of the file. An unhandled rejection is only theoretical.

## Previous findings
First round.

## Questions
None for the user.

## Coverage
- **AC 1 (reserve ≥ 1,000):** traced and passing. 59,100 measured by `make ci`, ledger booked in ADR-011, T-4 correctly skipped.
- **AC 2 (behaviour unchanged):**
  - Range selection, the range link set before the chunk lands, and the panel shown when it lands: `page.dom.test.ts` range-panel and range-link suites, plus e2e `f75-lazy` drag and lap tap. Passed.
  - Order inside `mountRangePanel` (subscribe sets `hidden` before `mount`): no flash of an empty panel. Before the chunk lands, the slot is `:empty`, with zero padding and no height (e2e asserts height 0).
  - Dynamics cells: `dynamics` is awaited before the charts mount, so `dynamics.range` exists when the panel mounts.
  - Edit page: `edit/laps.ts` and `splits` import `lib/range` themselves (shared `range` chunk), and pins only set the selection. No dependency on the panel's timing.
  - Pinned map, strip and splitter: CSS moved verbatim into `map-view.css` with the same media queries, including `.map-pin-group` hidden in both two-column queries. Unit tests and e2e splitter drag passed. The `.splitter` and pin are absolutely positioned or inside MapLibre's control stack, so no layout shift. `restoreSplit` stays in the entry (F-73 unaffected).
  - Route-colouring tokens: the 69 token lines are identical in value and order to baseline. They use the same three theme selectors as `page.css`, so specificity behaves as before for system dark with `data-theme='light'` and for system light with `data-theme='dark'`. No other consumer than `map-style.ts`, which keeps its fallbacks.
  - Dark and light rendering not checked in a browser beyond the e2e `layout` dark-theme tests, which passed.
- **AC 3 (no-route workout loads no map code or tokens):** e2e `f75-lazy.pw.mjs:112` passed on chromium and webkit (Firefox skipped, no WebGL).
- **AC 4 (LCP, 60 fps on ride-12h):** not run.
- **Not run:** a manual browser pass on a real phone (pinned map) and the LCP and scrub benchmarks.
