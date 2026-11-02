# QA F-05 Stacked charts with shared cursor - round 2

- **Date:** 2026-10-11
- **Verdict:** not ready.
  - All six r1 findings are fixed, and the r1 question about F-13 is settled by D-16.
  - **QA-05-r2-1 (Major, architect)** drives the verdict. The new fixed-height readout scrolls sideways instead of wrapping. On a phone, only about 3 of the 8 values are visible while scrubbing, and you cannot scroll the readout with the finger that is scrubbing. That breaks AP-6 ("shows all values at that point") on the form factor that AC 5 and F-07 target. A cheap alternative keeps "no jump" and shows every value.
  - Also new: one latent Minor defect, the stack height after `data` is swapped in place (the engineer's open note, reproduced; QA-05-r2-2), and one Minor test gap (4 surviving glue mutants; QA-05-r2-3).
- **Inputs:**
  - Design v1.3, compared against the v1.1 text I reviewed in r1 with `diff`: D-14 to D-16, layout constants, readout, tests and bundle.
  - ADR-008 rev. 1, ADR-011 rev. 1 and ADR-012 rev. 1.
  - The F-01 design diff, which narrows the "no jsdom" sentence to `src/components/`.
  - `architect/INDEX.md`.
  - The uncommitted working tree on top of `699a1d1`:
    - `src/web/src/lib/cursor/`;
    - `src/web/src/components/charts/`, including the new `ChartStack.dom.test.ts`;
    - `src/web/vite.config.ts`, `src/web/package.json`, `src/web/package-lock.json` and `src/web/test-support/golden.mjs`.
  - `qa/F-05-stacked-charts-with-shared-cursor-r1.md`.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (repo, exit code captured) | **exit 0**. `npm ci` printed an `EBADENGINE` warning (Node 22.22.2 against `>=24`); it existed before this round and does not fail the build. `svelte-check`: 202 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: `Test Files 14 passed (14)`, `Tests 463 passed (463)`. Bundle: `10613 / 60000 bytes gzip PASS`, unchanged |
| Bundle, reproduced in a scratch copy. The design's temporary `ChartStack` + `createCursorStore` import, then uPlot only | Base **10,613**. uPlot only **32,972**. F-05 import **40,237**, which is **+29,624 / 29,800**. Own code 40,237 − 32,972 = **7,265 / 7,400**. Matches the engineer's numbers exactly. CSS is a separate file |
| jsdom kept out of the production build | `jsdom` is a dev dependency only. The lockfile diff only *adds* jsdom and its dependencies (35 packages); no existing version changed. `vite build` with no temporary import gives the same 10,613 bytes. The built assets contain no `jsdom` or `vitest` string in any of the three builds. `resolve.conditions: ['browser']` applies only when `process.env.VITEST` is set |
| jsdom kept out of the Node tests | Only `ChartStack.dom.test.ts` has `// @vitest-environment jsdom`. The default is still `environment: 'node'`. I added a throwaway `src/lib` test in the scratch copy asserting that `window` and `document` are undefined; it passes, so Node tests do not see jsdom's globals |
| Mutation run (scratch copy, `mut2.py`): the 49 r1 mutants re-run, plus 31 new ones (store errors, component glue, `stackHeight`) | **80 run, 74 killed, 6 survived.** 2 survivors change nothing observable (`nearestIndex` `<`; resize coalescing, which Svelte batching hides). **4 are test gaps:** QA-05-r2-3 |
| Store mutants (r1 survivors and new error handling) | Killed: initial call with the delivered value; in-place `splice` (both r1 survivors); `try/catch` removed (in `store.test.ts` and in `ChartStack.dom.test.ts`); `report` not called; `break` after an error. Survived: `delivered = null` after an error (QA-05-r2-3 d) |
| Glue mutants against `ChartStack.dom.test.ts` | **21 of 25 killed.** Killed: rectangle not reset on resize; scroll listener a no-op; bucket count read before `setSize`; line not re-placed after a resize; `toDisplay` dropped; line not hidden on `null`; no unsubscribe; `pointerleave` not removed; no `destroy`; `Math.round` instead of `nearestIndex`; `current` not passed; `onfirstdata` dropped; `onready` guard, synchronous `onready` and no-chart `onready` mutants; `cssText` write dropped; observer not disconnected; wiring check dropped; `chartHeight` ignored; readout not reset on `null`. Survived: listed in QA-05-r2-3 and in the row above |
| In-place `data` swap probe (scratch `Wrap.svelte` holding `data` and `cursor` in `$state.raw`, plus a copy of the jsdom setup) | Mount ride-12h (6 charts), lay out at 800 px, then swap to no-distance-with-gps (3 charts) with a new store. Result: **`before 918px … after 918px`, 3 charts, expected `stackHeight(3) = 486px`**. The probe fails; see QA-05-r2-2 |
| `git status --porcelain`, before and after | Identical, apart from this report and `qa/INDEX.md`. All scratch files were removed from the scratch copy |

## Findings

### Blocker

None.

### Major

- **ID:** QA-05-r2-1
- **Severity / kind:** Major / defect: an architecture trade-off that breaks a Must requirement on phones
- **Route:** architect (layout rule in D-16 and "Readout"); then frontend
- **Where:**
  - `src/web/src/components/charts/CursorReadout.svelte:60-74`: `height: var(--readout-row-height)` (26 px), `overflow-x: auto`, `overflow-y: hidden`, cells `flex: none`, `nowrap`;
  - design v1.2/v1.3, "Readout": "the row scrolls horizontally rather than wrapping, so its height stays fixed";
  - D-16.
- **Relates to:** AP-6 ("shows all values at that point", Must); F-05 AC 2 ("updates the readout with all present values"); AC 5 and F-07, which make the phone the target device.
- **Evidence:**
  - ride-12h has 8 cells. I counted the characters of each cell's label, value, unit and the spaces between them: Time 13, Distance 18, Elevation 17, Speed 16, Heart rate 18, Power 12, Cadence 15, Temperature 18. That is 127 characters at 13 px, plus 7 gaps of 16 px and 64 px of padding. **The row needs about 1,000 px or more.**
  - On a 390 px phone, the bench page leaves 358 px for the stack. Minus 48 px left padding and 16 px right padding, the visible width is **about 294 px, enough for 2–3 cells** (Time, Distance, part of Elevation). Heart rate, power, cadence and temperature are off-screen.
  - Scrubbing uses the finger on the chart, so the readout cannot be scrolled at the same time.
  - On desktops with classic scrollbars (Linux, Windows), a window narrower than about 1,000 px gets a horizontal scrollbar of about 15 px inside the 26 px `border-box` row. That leaves about 11 px for an 18 px line, with `overflow-y: hidden`, so the scrollbar covers the values.
  - r1 wrapped the readout, so all values were visible. The change came with the r1-1 and D-16 fixes.
  - I did not see this in a browser. The width arithmetic leaves no room, even at 5.5 px per character, but the operator's phone check (P7) confirms it.
- **Consequence:** On a phone, the main use case (scrub with a finger and read heart rate or power at that point) does not show those values. The F-07 phone gate would run on this layout.
- **Alternative to consider:** keep the readout in normal flow (wrapping, auto height), and reserve a fixed height only for the *chart area* (a container below the readout sized `n × (title + chartHeight + 8) + 28`). The browser decides the readout height in the first layout, before the first paint, so nothing jumps. Only the chart area waits for the `ResizeObserver` frame, and its space is already reserved. This would also remove QA-05-r2-2 if the container sits inside `{#key data}`.
  - Svelte style attributes need not come back: the container height can be set in the same `cssText` way.
  - F-13 can still own the final layout.
- **What would change my mind:**
  - the operator's phone run shows all values visible while scrubbing; or
  - the user accepts, for phase 0, that phones show only the first values until F-13. That is a product decision on AP-6, so escalate it rather than decide it in code.

### Minor

- **ID:** QA-05-r2-2
- **Severity / kind:** Minor / defect (latent: no caller swaps `data` in place yet)
- **Route:** architect (D-16 wording: "in `onMount`" against the Lifecycle rule "a new `data` prop rebuilds the whole stack"), then frontend
- **Where:** `src/web/src/components/charts/ChartStack.svelte:53-56` (`rootStyle` is `$derived` from `data`) and `:98` (written once in `onMount`). The root `div` is outside `{#key data}` (`:114-115`).
- **Relates to:** D-16; design "Lifecycle"; the engineer's open note
- **Evidence:**
  - The probe above shows the stack stays at 918 px with 3 charts. The correct height is 486 px, so 432 px are left empty.
  - In the reverse direction (3 charts to 6), the charts overflow the fixed root by 432 px onto whatever follows, because the root has no `overflow` rule.
  - `chartHeight` *is* re-read for the new `data` (D-15, `plotHeight` is derived from `data`). The charts therefore change height, but the root does not.
- **Consequence:** F-07 or F-13 could reasonably load a second file into the same `ChartStack`, which the design describes as supported. The page then overlaps or leaves a gap.
- **Fix:** either write the `cssText` from an effect that tracks `rootStyle`, so it runs at mount and on every `data` change, at a cost of a few bytes; or change the Lifecycle text to "a new `data` requires a new `ChartStack` (the parent keys it)". My recommendation is the effect, because it keeps the Lifecycle promise. If the architect adopts the QA-05-r2-1 alternative with the container inside `{#key}`, this goes away.

- **ID:** QA-05-r2-3
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:**
  - `src/web/src/components/charts/ChartStack.dom.test.ts`;
  - `src/web/src/lib/cursor/store.test.ts`;
  - code under test: `MetricChart.svelte:130` and `:173`, `CursorReadout.svelte:42`, `store.ts:61-63`.
- **Relates to:** ADR-012 ("component glue regressions fail CI"); QA-05-r1-1 (no clipping); AC 2 (tap)
- **Evidence:** four mutants survive, and each is a plausible regression:
  - (a) **Touch tap.** `kind === 'up'` is dropped from the index mapping (`MetricChart.svelte:130`), so `pointerup` sends index −1 and a tap no longer places the cursor. The reducer test covers the reducer, but no jsdom test sends a touch `pointerup`.
  - (b) **Scroll listener cleanup.** The scroll listener is removed without `capture` (`:173`), so the browser keeps it. Each rebuild leaks one window listener per chart, and each one holds a destroyed uPlot. The cleanup test matches only target, type and function, not the capture flag.
  - (c) **r1-1 regression.** `style.minWidth` becomes `style.width` (`CursorReadout.svelte:42`), which brings clipping back. Nothing asserts the cell style.
  - (d) **Null after an error.** The store sets `delivered = null` after a listener error. A later `set(null)` is then not delivered, so the lines and readout stay on the old value.
- **Consequence:** the jsdom suite does not lock in three of the r1 behaviours it was added for. Each test is a few lines:
  - a touch down then up in the pointer test;
  - compare the `capture` option in the cleanup test, or count window `scroll` listeners after unmount;
  - assert `minWidth` is set and `width` is empty on a value cell;
  - an error flush followed by `set(null)`.

### Preference

None new.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-05-r1-1 (readout clipped long values) | **fixed** in code | Value cells use `min-width` in `ch`, `nowrap`, and have no `overflow: hidden`, so a long value widens its cell. The visual check is manual, and no test pins it (QA-05-r2-3 c). The new row layout causes QA-05-r2-1 |
| QA-05-r1-2 (store rules not pinned) | **fixed** | The three new tests kill both r1 survivors: initial call with the delivered value, and in-place `splice` |
| QA-05-r1-3 (a throwing listener freezes later ones) | **fixed** | ADR-008 rev. 1 and D-14: per-listener `try/catch` with an injectable `report`; the default rethrows in a microtask. The `try` removed, `report` dropped and `break` mutants are killed, in both the store test and the jsdom test. There is no allocation in the hot loop |
| QA-05-r1-4 (`chartHeight` silently ignored) | **fixed** | D-15 and the prop doc say "read once per `data` value". `plotHeight` is derived from `data` and reads `chartHeight` untracked. The `chartHeight` ignored mutant is killed |
| QA-05-r1-5 (no CI coverage of the glue) | **fixed** | ADR-012 rev. 1 adds `ChartStack.dom.test.ts`: 14 tests, real Svelte and real uPlot (wrapped, not replaced), in `make ci`. It covers every item the design lists, and 21 of 25 glue mutants are killed. The remaining gaps are QA-05-r2-3 |
| QA-05-r1-6 (`48px` literals) | **fixed** | No literal repeats a uPlot geometry value. `--y-axis-width`, `--title-row-height` and `--readout-row-height` come from the TypeScript constants through one `cssText` write. There are no `style=` or `style:` attributes in `components/charts` |
| r1 Question 1 (who reserves the stack height, F-13) | **settled** | D-16: `ChartStack` sets `stackHeight(n, chartHeight)` before the first paint, and F-13 reserves the same height in the server HTML. The `stackHeight` tests pass, and both `stackHeight` mutants are killed |
| r1 Question 2 (OQ-13) | open, user | Unchanged. To be confirmed in F-07 |

## Questions

1. **For the architect (QA-05-r2-1):** keep the fixed-height, sideways-scrolling readout, or reserve height only for the chart area and let the readout wrap? If the fixed row stays, the user has to accept the AP-6 trade-off on phones. **Settled by:** a revised D-16 and "Readout" section, or a recorded user decision.
2. **For the architect (QA-05-r2-2):** is `data` replaceable in place on one `ChartStack` (Lifecycle, as written today), or must the parent remount it? **Settled by:** one sentence in D-16 or Lifecycle, matching the code.

## Code review notes (changes since r1, checked and found correct)

**Store**
- `flush` wraps each listener in its own `try/catch` and allocates nothing.
- `subscribe`'s initial call is deliberately not wrapped, as the design says.
- `ReportError` is exported from `lib/cursor/index.ts`.

**ChartStack**
- `rootStyle` is written with one `cssText` assignment before `observer.observe`.
- `onMount` runs in the microtask after `mount()`, so the height is in place before the first paint. The jsdom test asserts it after `flushSync` with no chart mounted yet.

**`stackHeight`**
- The formula, readout + n × (16 + h + 8) + 28, matches what is drawn: title row, then uPlot `outerHeight`, with the x axis on the bottom chart only.
- uPlot's root adds no height when the legend is off. A browser check for gaps or overlap is in the list below.

**Tests**
- The jsdom test wraps the real uPlot and records `setSize`, `setData` and `destroy`. It recomputes the expected decimation independently and checks `bbox` width against `plotWidth`.
- It stubs only what ADR-012 allows, each with a comment.
- `golden.mjs` now uses `path.resolve(import.meta.dirname, …)`, as ADR-012 rev. 1 records.

**Module rules (re-checked)**
- `uplot` is imported as a value only in `MetricChart.svelte`.
- `lib/cursor` imports nothing.
- `lib/charts` imports only `../datafile` (type).
- No `$cursor`, and no rune fed by the cursor.
- jsdom appears only in `*.dom.test.ts`.

## Coverage

| AC | Status since r1 |
| --- | --- |
| 1 | The logic is traced and passing. The jsdom test now also checks chart count per file and the height per chart count. Visual alignment and the absence of gaps need the operator |
| 2 | The logic is traced and passing, with pointer to reducer to store to line and readout now in CI (jsdom). **Readout visibility on phones is QA-05-r2-1.** The touch tap glue is untested (QA-05-r2-3 a). Browser check needed |
| 3, 4 | Unchanged from r1: traced and passing. Visual check needed |
| 5 | Phone check needed (OQ-12) |
| 6 | The resize order, the re-placed line and coalescing are now in CI (jsdom). Desktop resize and phone rotation still need the operator |
| 7 | Desktop measurement still pending in `MEASUREMENTS.md` |

**Not run:**
- no real browser and no phone are available here (OQ-12);
- brotli was not looked at beyond the size script's column;
- no real (non-synthetic) files, per ADR-004 item 7.

## Manual check list for the operator (changes since r1)

The r1 list still applies, with these changes.

| # | Change | Check | AC |
| --- | --- | --- | --- |
| D4 | **replaced** | ride-12h at 1,200 px or wider: the readout is one row, the cells do not change width while scrubbing, and nothing below the stack moves. Then narrow the window to about 1,000 px: the row scrolls sideways. **Check that the horizontal scrollbar does not cover the values** (Linux and Windows have classic scrollbars), and note which values are visible without scrolling | 2 (QA-05-r2-1) |
| D13 | **new** | Load each file (3, 5 and 6 charts: no-distance-with-gps, brick-run-bike, ride-12h). When the charts appear there is **no page jump**: the spacer text below the stack stays where it was at first paint. There is **no gap and no overlap** between the bottom chart's time axis and the next element | 1, D-16 |
| D14 | **new** | Drag-resize with a 3-chart file and with ride-12h: the stack height stays the same (only the width changes), and nothing overlaps the content below | 6 |
| P7 | **replaced** | Portrait, ride-12h: the readout is one row that scrolls sideways. **While scrubbing with one finger, note how many values are visible** (expected: Time, Distance and part of Elevation). Swiping the readout itself scrolls it. Check that no scrollbar covers the text | 2 (QA-05-r2-1) |
| P10 | **new** | Phone: no page jump when the charts appear. After rotating portrait to landscape and back: no gap and no overlap below the stack | 6, D-16 |

Long values (100 h or more, 1,000 km or more) cannot be checked with the synthetic files. The fix is verified by CSS review only, which is why QA-05-r2-3 c asks for a test.
