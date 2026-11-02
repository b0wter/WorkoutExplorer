# ADR-012 Component smoke tests in jsdom, in CI

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 3 of 12 Oct 2026
- **History:** [architect/history/ADR-012-component-smoke-tests-in-jsdom.md](history/ADR-012-component-smoke-tests-in-jsdom.md)
- **Features:** F-05 (introduces it); every front-end feature with component glue (F-06, F-07, F-11, F-13, F-14, F-17, F-18, F-19, F-20, F-28)
- **Supersedes:** the sentence "no jsdom (F-01 tests are Node-only)" in the [F-01 design](F-01-project-skeleton-and-ci.md), section Front end, for `src/components/` only. F-01 D-6 (Vitest in Node) stays the default.
- **Requirements:** NFR Reliability; CLAUDE.md delivery workflow (QA runs the build and tests)

## Context

F-01 kept tests Node-only so that `lib/` stays free of DOM and the dev dependencies stay small. That works for pure logic. F-05 then put about 400 lines of component glue in `components/charts`, with no test in CI:
- the `onready` timing;
- the resize order;
- the pointer-position cache;
- the wiring check;
- pointer handling;
- cleanup on unmount.

QA r1 (QA-05-r1-5) found it covered only by manual checks. The engineer wrote a 49-check jsdom smoke test in the scratchpad, which would be lost. F-06, F-13 and F-17 will add more glue of the same kind, all of it easy to break silently.

There are three options:
- keep manual checks only;
- commit the jsdom script outside CI;
- run it in CI.

A script outside CI rots at the first refactor, and manual checks need the operator's browser (OQ-12) for things a DOM emulation can check.

## Decision

1. **`jsdom` becomes a dev dependency** (caret range, locked in `package-lock.json` like every other dev dependency). It is the only one added; there is no `@testing-library` and no Playwright.
2. **Component smoke tests** are named `*.dom.test.ts`, live next to their component under `src/components/`, and start with `// @vitest-environment jsdom`. The Vitest default stays `node`; `lib/` tests never use jsdom.
3. **They run in `make ci`** through the existing `npm test`. A failure fails CI.
4. **Scope:**
   - What they check: wiring, ordering, lifecycle, event-to-store paths and DOM writes.
   - What they never check: pixels, layout, real touch behaviour or timing, all of which jsdom cannot represent. Those stay manual (dev page, OQ-12).
   - Stubs allowed: the canvas (`getContext` and `Path2D`, which jsdom lacks and uPlot needs), `ResizeObserver`, `requestAnimationFrame` and `matchMedia`, and library internals only where jsdom cannot run them, with a comment saying why.
   - Setup (rev. 1): `vite.config.ts` adds `resolve.conditions: ['browser']` only under Vitest (`process.env.VITEST`), Svelte's documented setup, so components load Svelte's client build; `vite build` is unaffected. Test helpers that read files resolve paths with `path.resolve(import.meta.dirname, …)`, because jsdom's global `URL` breaks `node:fs` with file URLs.
   - (rev. 2) Synthetic `touchmove` events may be dispatched to check *wiring*: listener options, and whether `preventDefault` was called. If jsdom lacks `Touch` or `TouchEvent`, a plain `Event` with a defined `touches` property stands in, with a comment. This checks our handler, not the browser's scroll behaviour.
   - (rev. 2) A test may use a test-only wrapper component (a `.svelte` file next to the test, or a `*.svelte.ts` helper) to change props in place. Only tests import it.
   - (rev. 3) **A library that cannot run in jsdom at all** (MapLibre GL JS v6 needs WebGL2 and throws without it) may be replaced with `vi.mock` by a **recording fake**, under three conditions:
     - the fake implements only the members the adapter calls;
     - it holds no logic beyond recording calls and emitting the events the test triggers (for example `load`, `resize`);
     - every decision the adapter makes is in a pure module with its own Node tests (`lib/route`, `map-style.ts`), so the fake test checks wiring only: call order, options, listener options, cleanup, and the cursor-to-marker and pointer-to-cursor paths.

     The fake lives in the test file or in a `testing/` folder next to the adapter, and a comment names the library version whose API it mirrors. Imports Vite resolves specially (`?worker&url`) are mocked in the same test.
5. Synthetic data only (ADR-004 item 7), as everywhere.

## Consequences

- Component glue regressions fail CI instead of waiting for a manual browser run.
- **Cost:** one dev dependency (several MB in `node_modules`, a few seconds of CI). Stubs can drift from real browser behaviour, so a green smoke test is not a browser check. The manual checks (OQ-12) stay required for the ACs that need a real screen.
- No effect on the bundle (dev dependency only).

## Would choose differently if

- jsdom can no longer run uPlot or Svelte 5 components without stubbing most of them. Then the smoke tests are deleted rather than kept as stubs testing stubs, and a real-browser runner is proposed in a superseding ADR.
- (rev. 3) The MapLibre fake grows logic of its own (projection maths, event sequencing beyond what a test triggers), or the adapter's wiring bugs keep reaching the operator's manual checks despite a green fake test. Then drop the fake test and propose a real-browser runner for the map in a superseding ADR.
