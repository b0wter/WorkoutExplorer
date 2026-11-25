# ADR-011 Activity page bundle: allocation of the entry budget and what loads lazily

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 16 of 20 Nov 2026 (budget 62,000, decision 6); **revision 17 of 25 Nov 2026: size review before F-74 (decision 7), reserve policy (decision 5), ledger reduced to the current state**
- **History:** [architect/history/ADR-011-activity-page-bundle-allocation.md](history/ADR-011-activity-page-bundle-allocation.md) (revisions 1 to 16, every closed share and reserve value); [size review of 11 Nov 2026](history/ADR-011-size-review-2026-11-11.md) (attribution of the entry, candidates C1 to C11)
- **Features:** F-05 (triggers it); every feature that adds code to the activity entry is bound by it, F-74 next; F-32 checks it
- **Supersedes:** none (works within [ADR-002](ADR-002-bundle-size-budget-measurement.md))
- **Requirements:** TD Front end, Budget (activity entry 62,000 gzip since decision 6) and Load order; NFR Performance (LCP under 2.5 s)

## Context

ADR-002 counts an entry chunk, its static imports and their CSS, gzip -9; dynamic `import()` chunks are not counted. The activity entry is one `main` chunk and one stylesheet, shared by `index.html`, `edit.html` and `my-workouts.html` (`vite.config.ts`, `codeSplitting`), checked against `LIMIT_BYTES` 62,000 in `src/web/scripts/check-bundle-size.mjs`.

What the entry is made of was last attributed on 11 Nov 2026 (review link above): uPlot about 38 %, Svelte runtime about 23 %, own code and CSS the rest. The growth since then (about 5,200) is own code: F-54's i18n core, the feature glue and every lazy chunk's names in the preload list.

## Ledger (current state)

| Item | Bytes gzip |
| --- | --- |
| Entry on main, 25 Nov 2026, after F-75 (T-1 to T-3: 61,886 → 59,100) | **59,100** |
| Budget | 62,000 |
| **Reserve** | **2,900** |
| Owner chunk `start.svelte` (lazy; F-19 ceiling 20,000) | 19,439 (F-72) |

Shares booked since revision 17 (decision 4):

| Feature | Target / ceiling | Measured | Entry after |
| --- | --- | --- | --- |
| F-74 robust chart axis (AX-5), from the reserve, user 25 Nov 2026 | 100 / 150 | 148 (QA measurement) | 61,886 |
| F-75 T-1 range panel and `lib/range` lazy (`page/range-panel.ts`) | est. 1,200 to 1,500 | −1,457 | 60,429 |
| F-75 T-2 map-only glue (`page/map-glue.ts`, re-exported by `map-view`) and its CSS in the map chunk | est. 700 to 900 | −958 | 59,471 |
| F-75 T-3 route-colouring tokens in `map-view.css` | est. 200 to 250 | −371 | 59,100 |

T-4 not needed (T-1 to T-3 saved 2,786, above the 1,500 condition). The estimate was met; the reserve is above the 1,000 trigger (decision 5).

## Decision

1. **Static (counted): only what every viewer needs before the map.** The data file reader (F-04), the charts and cursor (F-05), the page glue and enhancers of the server markup (F-13, F-14, F-18), the i18n core (F-54), and the small static part of each lazy feature (its condition and `import()` site). Exceptions decided in decision 7 (built in F-75): the range panel (F-17) and the map-only glue leave the entry.
2. **Behind `import()` (not counted):** the map and everything that only works with it (MapLibre, `map-view`, route colouring, fullscreen, location); owner UI (`edit/start.svelte`, the published page); report form; How to use and the summary dialog with their shared dialog shell; multisport sections; splits, lengths and comparison chart; zones; best efforts; pins; GPX; dynamics; My workouts; the language loader, the prefix reader, the header language pass and the language files; the scrub recorder. **Rule:** code that serves only some workouts, only the owner, or only runs after a viewer's action belongs here. Prefer joining an existing lazy chunk to a new `import()` site: every site adds its hashed file names to the entry's preload list (about 15 to 30 gzip per file).
3. **Separate entries, each with its own ceiling** (`ENTRY_LIMITS`): upload (ADR-024), delete link, Impressum, About and generic pages. They must not import `uplot` or `lib/charts`, and share no chunk with the activity entry.
4. **Shares.** A feature that adds entry code states a target and a ceiling in its design. The engineer measures the entry before and after and books the figure in the table above; follow-ups count against the feature. Above the ceiling: stop and report.
5. **Review trigger and reserve policy (rev. 17).**
   - The architect reviews this ADR when the reserve falls below **1,000**, or a feature cannot fit its ceiling in the reserve.
   - **While the reserve is below 1,000, a new feature's entry share is net zero:** the feature (or a trim of decision 7 landed before it) removes at least as many entry bytes as it adds. Taking from a reserve below 1,000 needs the user's acceptance for that feature (as for F-74).
   - Remedies, in this order: move code behind `import()` or into an existing lazy chunk; trim CSS and build output (C1 to C3 done, 11 Nov 2026); structural changes (Svelte out of the entry, an own chart renderer), each with its own ADR.
   - The budget stays **62,000**. Raising it goes to the user and is not recommended while the remedies of decision 7 are open and LCP has never been measured on a phone (F-32 U-2).
6. **No English text twice in the entry (rev. 16, F-54).** The entry carries English only for strings its own code builds.

   | # | Change | Rule |
   | --- | --- | --- |
   | R-1 | **Markup English is read from the page, not shipped.** No key that a `data-t` in a template or shell carries is in the code's English tables | The applier records each keyed element's served text or attribute before its first rewrite and restores it for `en`. LG-8's completeness check reads the templates and shells as the English source for these keys |
   | R-2 | **A lazy group's English lives in its lazy chunk** | LG-3's text group = entry or lazy chunk, English included. The entry's table holds only entry-group strings |
   | R-3 | **API code texts (LG-6) are not in the activity entry** | They go in the lazy chunk(s) that send API requests; the upload entry keeps its own |

   Not allowed as a remedy: shortened or numeric keys, minified English, or dropping the switch back to English without reloading data.
7. **Size review of 25 Nov 2026 (before F-74): trim T-1 to T-3 after F-74, before the next feature that adds entry code.** Estimates are from the 11 Nov 2026 attribution and F-25's measurement, not re-measured; the engineer measures each step. "Budget" = the bytes leave the counted entry but most viewers still load them after first paint; "real" = viewers who do not need them never download them. *Note (QA-75-r1-3): with T-2 the splitter handle appears only once the map is ready; a failed map (no WebGL, MapLibre error) leaves a two-column page without a splitter. Accepted (F-13 D-17).*

   | # | Change | Est. saving | Kind | Cost and risk |
   | --- | --- | --- | --- | --- |
   | T-1 | **Range panel lazy.** `RangePanel` and `lib/range` (with the selection copy routine) load by `import()` started in the charts' `onready`, where the panel is mounted today; the slot stays hidden until a range exists. `lib/range` becomes a chunk shared with `src/splits/` and the owner chunk | 1,200 to 1,500 | budget (off the first-paint path) | The panel mounts one chunk later after first paint; a range link (F-40) or a lap tap before the chunk arrives shows the panel when it lands, no layout shift (slot reserved, F-17 R-9). Low to medium: the edit page and the range-link path need their DOM tests kept green. Changes decision 1 for F-17 |
   | T-2 | **Map-only glue into the `map-view` chunk:** pin toggle (`addPinToggle`, F-13 D-19), fullscreen readout strip (`mountStrip`, F-33), splitter drag (`addSplitter`; `restoreSplit` and the layout queries stay static), with their CSS (`.map-pin*`, `.layout.pinned`, `.wx-strip`, `#map-slot .readout`, `.splitter`) moved to `map-view.css` | 700 to 900 | real for workouts without a route, budget otherwise | Toggle and strip are created at map ready already: no change. The splitter handle appears when the map chunk has loaded instead of at mount: accepted, the map area is empty until then. Low |
   | T-3 | **Route-colouring tokens into `map-view.css`:** `--seq-*`, `--div-*`, `--range-outline`, `--route-fade` with the same three theme selectors; only the map reads them | 200 to 250 | real for workouts without a route | The map style already has fallbacks; the map-theme test must read them from the map stylesheet. Very low. Changes F-25 C-5 (palette in the static theme block) |
   | T-4 | **Smaller preload list:** keep CSS and `map-style` (MapLibre) in `__vitePreload`'s dependency lists, drop the other JS names (`build.modulePreload.resolveDependencies`) | 200 to 350, grows with each new lazy chunk | real | Click-loaded chunks with a JS dependency (summary dialog, How to use, splits) load one round trip later. Medium: CSS must still load before the module runs. **Only if** T-1 to T-3 together save less than 1,500 |

   Not now: Svelte out of the entry (C6, 12,000 to 14,000 real) and an own chart renderer (C7, 16,000 to 19,000 real) are weeks of work with axis and DOM-test regressions; choose one when a planned feature needs more than about 2,000 entry bytes, or when a phone LCP measurement fails. Still rejected from 11 Nov 2026: merging the dark-theme blocks (59 at most), lazy copy routines (Safari's clipboard activation), lazy hint and selects (layout shift).

   Expected result after T-1 to T-3: reserve about 2,200 to 2,800 (from 112 after F-74), back above the trigger. Lane: one frontend task, normal lane without an architect step (this decision is its design), QA at medium for T-1's timing.

## Consequences

- F-74 can go ahead within 150 from the reserve; after it, no feature adds entry code until T-1 to T-3 are built (decision 5).
- The range panel, the pin toggle, the strip and the splitter each appear one chunk later than today. All of them already appear after the charts' first paint or at map ready, so nothing on the first paint changes.
- Every new lazy chunk still costs entry bytes through the preload list; designs count them in their share.
- Cost: owner, report and other lazy UI need a loading state and appear about one round trip later. CI (ADR-002) fails the build at 62,000 regardless of shares.

## Would choose differently if

- F-31 makes brotli guaranteed and ADR-002 is superseded to measure brotli (about 10 % smaller). Then the shares are recomputed, not the decisions on lazy loading.
- A phone LCP measurement (F-32 U-2) shows LCP well under 2.5 s with margin. Then the user may raise the budget; the lazy-loading rules stay.
