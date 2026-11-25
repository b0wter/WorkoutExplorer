# ADR-011 Activity page bundle: allocation of the 60,000 bytes and what loads lazily

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 7 of 15 Oct 2026, with the F-07 QA r2 close-out; revision 8 of 24 Oct 2026, F-14 design: F-13 closed at 1,717 measured, its unused 3,283 bytes return to the reserve; revision 9 of 29 Oct 2026, F-17 QA r1: F-17 at 4,715 measured, 715 over its share accepted; revision 10 of 29 Oct 2026, F-19 QA r1: F-17 final 4,698, F-18 final 1,067, F-19 374 of 400 measured, reserve 3,640; revision 11 of 30 Oct 2026, F-20 QA r1 (QA-20-r1-4): F-20 +2 of its 100 ceiling, reserve 3,638; revision 12 of 30 Oct 2026, F-20 QA r2 (QA-20-r2-2): F-20 +7, reserve 3,633; revision 13 of 9 Nov 2026, F-39 and F-40 designs: shares 600 and 700 from the reserve, the How to use dialog lazy; revision 14 of 9 Nov 2026, F-42 design: reserve 955 after the F-40 share, below the review trigger; review result: F-42's browser code lazy, static ceiling 300; revision 15 of 11 Nov 2026: size review C1–C3 applied, reserve 3,493; revision 16 of 20 Nov 2026: F-54 over the line, remedy R-1 to R-3 and the conditional 62,000 budget (decision 6); outcome of 20 Nov 2026: entry 60,214 after R-1 to R-3, budget 62,000
- **History:** [architect/history/ADR-011-activity-page-bundle-allocation.md](history/ADR-011-activity-page-bundle-allocation.md) (revision list, earlier shares and reserves, the retired 45,000 trigger)
- **Features:** F-05 (triggers it); F-06, F-07, F-13, F-14, F-17, F-18, F-19, F-20, F-25, F-26, F-28, F-33, F-39, F-40, F-42, F-43, F-44, F-46, F-48, F-49, F-50, F-51, F-53, F-54, F-56, F-57, F-58, F-61, F-62, F-64, F-65, F-66, F-67 (bound by it); F-32 (checks it)
- **Supersedes:** none (works within [ADR-002](ADR-002-bundle-size-budget-measurement.md), which stays unchanged)
- **Requirements:** TD Front end, Budget ("App bundle, Svelte + uPlot + app code: 60 KB") and Load order; NFR Performance (LCP under 2.5 s)

## Context

ADR-002 counts the entry chunk plus its static imports plus CSS, gzip -9, 60,000 bytes per entry, and excludes dynamic `import()` chunks. The F-05 measurement (F-05 design v1.1) gives this projection for the activity page after F-07:

| Part | Bytes gzip |
| --- | --- |
| F-01 base (Svelte core, placeholder) | 10,613 |
| `lib/datafile` (F-04) | 2,793 |
| uPlot + CSS | 22,359 |
| F-05 own code incl. Svelte block runtime | 7,066 |
| **Total** | **≈ 42,800** |
| **Remaining** | **≈ 17,200** |

(rev. 7) **Measured ledger after F-07** (the engineer's measurements on 15 Oct 2026; close-out at F-07 QA r2). F-06's measurement (b) does not contain the data file reader: it imports only `ChartStack`, `createCursorStore` and `RouteMap`.

| Part | Bytes gzip |
| --- | --- |
| B0 = F-06 (b): Svelte core, F-01 placeholder, uPlot + CSS, F-05, F-06 static glue | 42,525 |
| + `lib/datafile` reader (`loadDataFile`) → B0′ | +2,803 → 45,328 (this table's 2,793 was right) |
| + F-07 page wiring, net of the retired placeholder → **E**, the CI entry | +254 → **45,582** (close-out, F-07 QA r2, with the body CSS in the bundle; QA measured 253 against its B0′ of 45,329) |
| (rev. 8) + F-13 layout, client part, measured at close → entry 47,299 (CI line 47.3 KB) | +1,717 |
| + F-14 units, pace, theme, measured at close → entry 50,221 | +2,922 |
| (rev. 10) + F-17 range, axis toggle, splitter, prefs, final at F-17 QA r2 → entry 54,919 | +4,698 |
| + F-18 laps and splits, measured at F-18 QA r1 → entry 55,986 | +1,067 |
| + F-19 page switch, owner `import()`, glue data seam, marks prop, measured at F-19 QA r1 → entry 56,360 | +374 |
| (rev. 12) + F-20, the owner chunk's new hash in the entry, measured at F-20 QA r2 → entry 56,367 | +7 |
| + F-25, F-26, F-28, F-33 and follow-ups, measured 5 Nov 2026 (QA-33-r1-1) → entry 57,399 | +1,032 |
| + F-36 to F-38, unbooked, measured 9 Nov 2026 before F-39 → entry 57,746 | +347 |
| + F-39 How to use, measured 9 Nov 2026 → entry 58,345 | +599 |
| + F-40 shareable range, with the F-39 QA r1 follow-ups, measured 10 Nov 2026 → entry 58,855 | +510 |
| + F-41 summary tints (+133) and the F-40 QA r1 minors, measured 10 Nov 2026 → entry 58,997 | +142 |
| + F-39 How to use moved to its own row under the header (user 10 Nov 2026), measured 10 Nov 2026 → entry 59,026 | +29 |
| + Range panel actions on their own row as links (user 10 Nov 2026), measured 10 Nov 2026 → entry 59,017 | −9 |
| + F-39 range hint as an info box under the charts (user 10 Nov 2026), measured 10 Nov 2026 → entry 59,142 | +125 |
| + F-39 range hint collapses during the visit, animated (user 10 Nov 2026), measured 10 Nov 2026 → entry 59,202 | +60 |
| + F-42 multisport sections, static part (sessions check, `import()` call site, chart decoration seam, separator rows in the laps and units enhancers, CSS), measured 10 Nov 2026 → entry 59,487 | +285 |
| + F-42 QA r1 minors (one tbody per leg, stripe selector, chunk-failure warn, separator pointer class), measured 10 Nov 2026 → entry 59,510 | +23 |
| + F-42 leg names in every chart (no `isTop`), cursor in `--fg` (user 10 Nov 2026), measured 10 Nov 2026 → entry 59,483 | −27 |
| + F-42 first leg's name kept when narrow (lazy chunk only; chunk hashes in the entry), measured 10 Nov 2026 → entry 59,485 | +2 |
| (rev. 15) − size review C1–C3 (user 11 Nov 2026, [review](history/ADR-011-size-review-2026-11-11.md)): unused uPlot statics blanked at build (`wx-uplot-trim` in `vite.config.ts`, ADR-009), uPlot CSS cut to the used rules (`components/charts/uplot.css`), Svelte `discloseVersion: false`; measured 11 Nov 2026 → entry 56,507 | −2,978 |
| + F-43 device line and F-44 pool cells (reader keys, two `summaryItems` cells, `.device` CSS; the edit page's line is in the owner chunk), measured 11 Nov 2026 → entry 56,655 | +148 |
| + F-46 laps and splits (LS-4: switch, splits table and drop-down leave for the lazy `src/splits/`, 1,602 + 226 CSS gzip; trigger column and the LS-8 seam join), measured 12 Nov 2026 → entry 56,351 (F-45 had left it at 56,661) | −304 |
| + AP-8 laps footnote (starred Time/Pace labels, hint under device-lap tables), measured 24 Nov 2026 → entry 60,565 of 62,000 | +100 |
| + F-73 handover line in `restoreSplit` (RS-3), measured 25 Nov 2026 → entry 61,383 (from 61,377) | +6 |
| **Reserve** | **3,649** (60,000 − 56,351) |
| **Total** | **60,000** |

Six MVP features still add code that viewers would load on the activity page, and four more add owner or optional UI. Without a plan, the budget is spent first-come, first-served, and the last feature hits the limit. Raising 60 KB is a change to the TD budget, a product decision for the user, not an architecture call.

## Decision

1. **Static (counted) on the activity page: only what every viewer needs before the map.** That is the charts and cursor (F-05) and the data file reader (F-04). It also covers F-06's small static glue (the decision whether to load the map, the `import()`, the "No location data" hint), the client part of the layout (F-13), units and theme (F-14), range selection and stats (F-17), and laps and splits interaction (F-18). The TD already renders the summary strip and laps table in the HTML, so F-13 and F-18 add behaviour, not markup.
2. **Behind dynamic `import()` (not counted, ADR-002 item 1):**
   - the map and everything that only works with it: MapLibre, the F-06 map module, route colouring (F-25), fullscreen map (F-26);
   - owner-only UI, loaded only when the page has an edit token: the settings panel (F-19), the publish, share and delete controls (F-20);
   - the report form (F-28), loaded when the Report link is first opened;
   - (rev. 13) the How to use dialog with all its texts (F-39, `src/howto/`), loaded when its button is first used;
   - (rev. 14) multisport sections (F-42, `src/sections/`): section lines, names and separator-row ranges, loaded after the charts' first paint and only for a workout whose sessions have two or more sports. Code that serves only some workouts belongs here, not in the entry.
3. **Separate entries, each with its own 60,000:** the upload page (F-11) and the About page (F-29), if they need JavaScript at all. They must not import `uplot` or `lib/charts`.
4. **Planning shares of the remaining ≈ 17,200** (stop triggers, like F-05's ceiling; each feature design restates its share and measures it):

   | Feature | Share (bytes gzip) |
   | --- | --- |
   | F-06 static glue | **2,100, final** (F-06 QA r3: measured static 2,029–2,056). It includes Vite's dynamic-import helper `__vitePreload`, measured at 785 bytes gzip with the lazy CSS list (rev. 4). The first `import()` of the entry brings it in; F-19, F-20 and F-28 reuse it and must **not** count it again in their shares |
   | F-07 page wiring (rev. 7) | **254, final** (close-out, F-07 QA r2: E 45,582 − B0′ 45,328, including the body CSS moved from `index.html`); ceiling 300 = E − B0′. B0′ is F-06's measurement (b) plus a temporary `loadDataFile` import, measured right before F-07. It covers the prototype page (query, load, error text, cursor store, the two components, User Timing marks, the `import()` call site of the lazy scrub recorder), less the retired F-01 placeholder (`App.svelte`, `lib/health`). It is taken **from the reserve**, not from F-06's 2,100. F-07's pixel-ratio fix (QA-06-r4-1) and the scrub recorder are lazy and cost 0 here. Above 300 in a QA round: stop and report. The close-out at F-07 QA records the final E |
   | F-13 layout (client part) | **1,717, final** (rev. 8: measured at F-13 close, v1.1 included; planned 5,000, the rest returned to the reserve) |
   | F-14 units, pace, theme | **2,922, final** (measured at close, 25 Oct 2026: entry 50,221 − 47,299; planned 2,500 plus 422 of the 500 from the reserve) |
   | F-17 range selection, axis toggle, stats | **4,698, final** (rev. 10: F-17 QA r2 entry 54,919 − 50,221, with the splitter, `lib/prefs` and readout follow-ups; planned 4,000, the overrun taken from the reserve and accepted in rev. 9) |
   | F-18 laps and splits interaction | **1,067, final** (rev. 10: F-18 QA r1 entry 55,986 − 54,919; planned 1,500, the rest returned to the reserve) |
   | F-19 edit page switch (owner UI lazy) | **400**; measured 374 at F-19 QA r1 (entry 56,360 − 55,986). Fixes after r1 stay within the 400 |
   | F-20 publish, share, delete, published page (owner UI lazy) | **0 planned, ceiling 100**; measured +7 at F-20 QA r2 (entry 56,367). The published page (F-20 v0.2) is in the lazy chunk and adds nothing static |
   | F-33 fullscreen readout strip (static glue and CSS) | **300 planned; measured 229** (4 Nov 2026: entry 57,388 − 57,159 on `dbe0450`); 240 with the QA-33-r1-2 `[hidden]` rule (entry 57,399) |
   | (rev. 13) F-39 How to use: button enhancer, `import()` call site, hint mode and ×, two prefs entries, CSS | **600 planned; measured 599** (9 Nov 2026: entry 58,345 − 57,746) |
   | (rev. 13) F-40 shareable range: `lib/rangelink`, read and apply, `replaceState`, panel button | **700 planned; measured 510** (10 Nov 2026: entry 58,855 − 58,345, including the F-39 QA r1 follow-ups) |
   | (rev. 14) F-42 multisport sections: the sessions check, the `import()` call site, a chart decoration seam, separator rows in the laps and units enhancers, CSS | **300 ceiling; measured 285** (10 Nov 2026: entry 59,487 − 59,202, F-42 frontend); the rest is lazy (`src/sections/`, 825 + 191 CSS gzip, and the shared `lib/sections` chunk, 398) |
   | (11 Nov 2026) F-43 device line: `lib/datafile`'s check of `device`, the line's CSS; the line builder stays in the owner chunk | **100 ceiling** (F-43 DI-9); measured with F-44 (one format bump): together **148** of 250 (11 Nov 2026, entry 56,655 − 56,507; backend 143, `.device` CSS 5) |
   | (11 Nov 2026) F-44 pool cells: two cells in `summaryItems`, two reader keys | **150 ceiling** (F-44 PS-6); measured with F-43, see above |
   | (13 Nov 2026) F-48 strokes: reader checks of the version 5 keys, stroke words, Stroke column and cell, the extended `import()` condition; the Lengths view is lazy in `src/splits/` | **300 ceiling** (F-48 PL-9); measured **+259 gzip** (entry 56,651 / 60,000) |
   | (13 Nov 2026) F-49 zones: the reader's `zones` check, the slot, the `import()` condition; section and `lib/zones` lazy in `src/zones/` | **150 ceiling** (F-49 ZN-9); measured **+265 gzip** (entry 56,916 / 60,000); the overrun of 115 was accepted by the user on 13 Nov 2026. The F-49 follow-up of 14 Nov 2026 (`cf493f6`: device basis next to the bounds, edit placeholders, how-to line, About page facts) measured **+22** (entry 56,939) |
   | (14 Nov 2026) F-51 comparison chart: the LS-8 seam's mark notification, preload-map growth from the shared zone-colour stylesheet; chart and `lib/compare` lazy in `src/splits/` | **80 ceiling** (F-51 Cost); measured **+34 gzip** (14 Nov 2026: entry 57,028 − 56,994; the baseline includes +55 from the theme commit `659c6be`, unbooked); **57,030** after the SC-11 percentile cap (+2 from chunk names in the preload map, no entry code change), F-51 total +36. Lazy, not counted: `src/splits/` chunk 2,095 → 4,545 JS and 235 → 728 CSS gzip, plus the shared zone-token stylesheet 308 gzip (moved from the zones chunk's CSS, 489 → 232) |
   | (14 Nov 2026) F-50 best efforts and power curve: the slot, the `import()` call site and condition, preload-map entries; section, curve and `lib/efforts` lazy in `src/efforts/` | **150 ceiling** (F-50 Cost); measured **+142 gzip** (14 Nov 2026: entry 57,176 − 57,034; the baseline was 4 above the recorded 57,030, unbooked; ±4 between builds from chunk names in the preload map). Lazy, not counted: `src/efforts/` chunk about 2,610 JS and 369 CSS gzip |
   | (19 Nov 2026) F-58 summary dialog: the ⓘ enhancer and a shared lazy-click helper, the `import()` call site, the viewing facts, preload-map entries, the strip's corner CSS; dialog and shared shell lazy | **200 ceiling** (F-58 Cost); measured **+251 gzip** (19 Nov 2026: entry 57,456 − 57,205; the baseline was 29 above the recorded 57,176, unbooked), **51 over the ceiling, reported**. Lazy, not counted: `src/summary-info/` 1,524 JS gzip; the shared shell `src/dialog/` 639 JS and 402 CSS (How to use's CSS moved in); How to use 907 → 684 JS |
   | (20 Nov 2026) F-57 GPX download: the button enhancer, the `import()` call site with the closed inputs (GX-2), the row CSS (backend's reserve rule included) | **150 ceiling** (F-57 Cost); measured **+147 gzip** (20 Nov 2026: entry 57,603 − 57,456). Lazy, not counted: `src/gpx/` with `lib/gpx` 651 JS gzip |
   | (20 Nov 2026) F-56 map pins: slot reserve, the `import()` call site, the map seam hand-over, the keyed chart decorators, the two-column order CSS | **250 ceiling** (F-56 Cost); measured **+257 gzip** (20 Nov 2026: entry 57,858 − 57,601 at `9ec6db3`, before F-56's backend commit, whose view-model change cost +8), **7 over the ceiling, accepted from the reserve** as for F-58 (QA-56-r1-2). Lazy, not counted: `src/pins/` with `lib/pins` 2,956 JS + 1,045 CSS gzip |
   | (20 Nov 2026) F-53 My workouts: the `#wx-mine` branch and its `import()` call site, three preload-map names (the list page's chunk, the shared `lib/mine` chunk, the owner chunk's new dependency); the stylesheet is loaded by URL from the lazy chunk to keep a fourth name out | **60 ceiling** (F-53 Cost); measured **+64 gzip** (20 Nov 2026: entry 57,926 − 57,862; 57,923 to 57,926 between builds from chunk hashes in the preload map), **4 over, reported**. Lazy, not counted: `src/mine/` 1,433 JS + 310 CSS gzip, `lib/mine` 885 JS gzip; owner chunk 12,438 → 12,679 |
   | (20 Nov 2026) F-54 translated interface: i18n core, language drop-down, markup applier, data-file prefix reader, `Intl` paths, hold release; de/es/fr files lazy (F-54 LG-3) | **800 target, ceiling 1,000** (F-54 Cost); English literals moved into tables net zero. Lazy, not counted: language files, at most 5 KB gzip each. (rev. 16) First build `c91b574` measured **+2,482** (entry 60,214, over the 60,000 line). Remedy per decision 6. **Final: +2,482** (20 Nov 2026: entry 60,214 before and after R-1 to R-3 against the measured baseline 57,732 at `4cbe231`); the build already kept lazy groups' English and the API code texts in lazy chunks (R-2, R-3) and read markup English from the page (R-1); the one key in both a template and an entry table, `noLocation`, stays for RouteMap's own hint |
   | (20 Nov 2026) F-61 satellite layer: the `wx.map` accessor in `lib/prefs` and handing the choice to the map; toggle, style composition, Esri constants and texts in the lazy map chunk (F-61 SL-1) | **100 ceiling** (F-61 Cost); measured **+67 gzip** (20 Nov 2026: entry 60,281 − 60,214). Lazy, not counted: map-view chunk +364 JS (7,628 → 7,992) and +128 CSS gzip |
   | (21 Nov 2026) F-62 show my location: the viewing-page flag through the map props | **50 ceiling** (F-62 Cost); measured **+21 gzip** (after the F-62 QA r1 minors: entry 60,302 − 60,281; QA r1 measured 60,293, the rest is chunk-hash churn). Lazy, not counted: map-view chunk 7,992 → 8,572 JS, 892 → 1,050 CSS; location chunk 1,120 |
   | (22 Nov 2026) F-64 laps and splits per multisport leg: per-block units and language rewrite, taps by DOM position over leg headings and rows, leg heading CSS (separator CSS gone); `splitBounds` moved to the lazy `lib/splits` | **200 ceiling** (F-64 Cost); measured **+65 gzip** (entry 60,404 at main `a0250ac`, F-63 included → 60,469; backend −8, frontend +73; F-63's +102 is not F-64's, QA-64-r1-2). Lazy, not counted: splits chunk 5,141 → 6,183 JS, 835 → 864 CSS; sections CSS 239 → 261 |
   | (22 Nov 2026) F-65 owner-added laps: the trigger code `added` and its word; `enhanceUnits` no longer runs on the edit page (its laps and strip are built in the owner chunk), which pays for the owner chunk's new exports | **50 ceiling** (F-65 Cost); measured **+52 gzip** (entry 60,469 → 60,521; 60,511 to 60,528 across builds of the same code, chunk-hash churn), within noise of the ceiling, reported. Lazy, not counted: owner chunk 13,888 → 16,745 JS, 1,480 → 1,666 CSS; pins chunk 2,101 → 2,135; map-view 8,563 → 8,604 |
   | (23 Nov 2026) F-66 remove auto-laps: nothing in the entry; removal mirror, line and toggle in the owner chunk only (F-66 RA-9, RA-10) | **0 ceiling** (F-66 Cost); measured **0** (entry 60,463 at F-66 QA r2, equal to the base 92958c2; the `{:else if auto}` branch in the shared `LapsTable` is unreachable on the viewing page). Lazy, not counted: owner chunk `start.svelte` 18,095 → **18,907** gzip (estimate 600, measured +812), 1,093 under F-19's 20,000 ceiling; 19,412 after the F-65 added-laps table (23 Nov 2026, +505); **19,701** after the pin dialog (23 Nov 2026, +289): the baseline for the next owner feature. The shared `src/dialog/` stays its own chunk (735 gzip), not counted in `start.svelte` |
   | (24 Nov 2026) F-67 multisport leg picker: view state on `Look`, the chart zoom hook (key, leg subarrays and range, pointer clamp, clip class), readout and range panel per leg from a small `lib/sections` ordering module (never the index), map and selection handed to the sections chunk; picker, numbering, texts and the range listener lazy in `src/sections/`, the map fit in the map chunk (F-67 LP-10, Cost) | **300 target, ceiling 400** (F-67 Cost); measured **+398 gzip** (24 Nov 2026: entry 60,565 → 60,963; owner chunk `start.svelte` 19,943 → 19,946, hash churn). Lazy, not counted: sections chunk 414 → 826 JS, CSS 261 → 382; shared `lib/sections` chunk 398 → 381 |
   | (24 Nov 2026) GitHub link: footer on the viewing and edit pages, line under the upload form (unbooked, other engineer) | measured **+108 gzip** (entry 61,031 at `654004a` → 61,139 at `15cd39e`); owner chunk `start.svelte` 20,044 |
   | (24 Nov 2026) F-68 extra sensor streams: dynamics names in the reader's metric order, the `lib/charts` `dynamics` seam (spec list and range cells installed by the lazy chunk), the conditional `import()`, the panel's one hook; chart specs, texts, colour tokens, the range means and the cells in the lazy `src/dynamics/` (F-68 XS-9) | **150 target, ceiling 250** (F-68 Cost); measured **+228 gzip** (24 Nov 2026: entry 61,367 − 61,139 at main `15cd39e`, which holds the backend's reader rows and the footer link; 61,031 at `654004a` before the footer; 60,963 above is F-67's close). Most of it is the `import()` preload entry (two hashed file names) and the per-chart CSS the server's reserved layout needs (`.charts-7` to `-10`, `.cells-9` to `-12`, `--r8`). Lazy, not counted: dynamics chunk 1,002 JS and 211 CSS gzip; the texts joined the `page` language group (no own group: a group needs a head `<meta>` and a `loadGroup` call, about 30 gzip in the entry, over the ceiling); owner chunk `start.svelte` 20,044 → 20,046 (the reserved-layout count in `lib/page/view.ts`) |
   | (25 Nov 2026) Owner chunk trim: the published page (`startPublished`, `PublishedView`) moved to the lazy `src/edit/published.ts` chunk, loaded only for an edit link of a published workout; no behaviour change | owner chunk `start.svelte` 20,040 → **19,424** gzip (576 under the 20,000 ceiling); lazy `published` chunk 1,491 gzip; entry +13 (preload-map name, hash churn) |
   | Reserve (architect decides who gets it) | **1,655** after F-39 (9 Nov 2026: 60,000 − entry 58,345). The baseline before F-39 was 57,746, not 57,399: F-36 to F-38 took 347 unbooked, so the reserve was 2,254, not 2,601. **1,145 after F-40's measured 510** (planned 955 after its 700, below the review trigger, decision 5). (rev. 14) **845 after F-42's ceiling**; measured **513** after F-42's 285 (10 Nov 2026, entry 59,487; F-41 and the 10 Nov page changes took 347 in between, table above); **490** after the F-42 QA r1 minors (+23, entry 59,510). **517** after the leg-name and cursor-colour change (−27, entry 59,483); **515** after the first-leg name fix (+2, entry 59,485). (rev. 15) **3,493** after size review C1–C3 (−2,978, entry 56,507); **3,243** after the F-43 and F-44 ceilings (11 Nov 2026); **3,345** measured after both (+148, entry 56,655). **3,349** after F-48 (+259, entry 56,651; F-46 had left 56,351 and F-47 56,392). **3,199** after the F-49 ceiling (13 Nov 2026); **3,084** measured after F-49 (+265, entry 56,916). **3,061** after the F-49 follow-up `cf493f6` (+22, entry 56,939); **2,981** after the F-51 ceiling (14 Nov 2026). **2,970** measured after F-51 (entry 57,030: `659c6be` +55, F-51 +36). **2,820** after the F-50 ceiling (14 Nov 2026); **2,824** measured after F-50 (entry 57,176). **2,544** measured after F-58 (entry 57,456). **2,397** measured after F-57 (entry 57,603). **2,142** measured after F-56 (entry 57,858; 57,601 before it, so F-56's cost is 257). **2,074** measured after F-53 (20 Nov 2026: entry 57,926 − 57,862 at `15b92d8`, **+64**, 4 over its 60 ceiling, reported). Review result: F-42 goes lazy (item 2); the next feature that adds static code applies a remedy of decision 5 before it is built. (rev. 16) **1,786** after F-54 against the **62,000** budget (decision 6: entry 60,214, above 59,000). **1,686** after the F-61 ceiling (20 Nov 2026); **1,719** measured after F-61 (entry 60,281); **1,698** measured after F-62 (entry 60,302); **1,531** measured after F-64 (entry 60,469); **1,479** measured after F-65 (entry 60,521); **1,537** measured after F-66 (entry 60,463; the drop from 60,521 came before F-66, unbooked); **1,435** measured after the AP-8 laps footnote (entry 60,565); **1,035** after the F-67 ceiling (24 Nov 2026); **1,037** measured after F-67 (entry 60,963) **633** measured after F-68 (entry 61,367) |

5. **Review trigger (rev. 7):** the reserve falls **below 1,000**, or a feature cannot fit its share plus the reserve. The architect then revisits this ADR before that feature is implemented.
   - From F-07 on, the CI line measures the activity page itself, because the prototype page replaced the F-01 placeholder as `index.html`'s entry. The ledger is therefore read from the CI line plus the open shares.
   - There is no absolute entry-size trigger: the reserve trigger says the same thing without going stale.

   Remedies in this order:
   - move more code behind `import()`;
   - drop the dead parts of the F-01 placeholder (done in F-07);
   - replace uPlot with an own canvas renderer (ADR-009, "Would choose differently if").
   
   Raising 60 KB goes to the user.

6. **(rev. 16, 20 Nov 2026) F-54 remedy: no English text twice in the entry.** The entry carries English only for strings its own code builds. The engineer applies all three, then measures:

   | # | Change | Rule |
   | --- | --- | --- |
   | R-1 | **Markup English is read from the page, not shipped.** Remove every key that a `data-t` (text or attribute) in a template or shell carries from the code's English tables | The applier records each keyed element's served text or attribute value before its first rewrite and restores that record for `en`. The served page is English (LG-4), so the record is the English source. LG-8's completeness check keeps reading the templates and shells as the English source for these keys. Same core in the upload and static entries |
   | R-2 | **A lazy group's English lives in its lazy chunk.** Owner UI, report form, How to use, map (MapLibre `locale` included), sections, splits, zones, efforts, pins, GPX, summary dialog, My workouts: each chunk imports its own English table | LG-3's text group = entry or lazy chunk, applied to English too. The entry's table holds only entry-group strings |
   | R-3 | **API code texts (LG-6) are not in the activity entry.** They go in the lazy chunk(s) that send API requests (one shared lazy module is fine); the upload entry keeps its own | The viewing page sends no API request that can show a server message |

   Not allowed as a remedy: shortened or numeric keys, minified English, or dropping the at-once switch back to English without reloading data (F-54 Acceptance criteria).

   **Budget outcome** (user decision 20 Nov 2026): if the entry after R-1 to R-3 is **at most 59,000** (reserve at or above the review trigger), the 60,000 line stays and F-54's share is final as measured. If it is **above 59,000**, the activity entry budget becomes **62,000 bytes gzip** (ADR-002 CI line and TD Budget row; the requirements engineer updates the TD) and the reserve is 62,000 minus the measured entry. Either way the engineer reports the entry before and after R-1 to R-3 and the F-54 share against the measured baseline (60,214 − 2,482 = 57,732, not the recorded 57,926; the difference is unbooked).

   **Outcome (20 Nov 2026):** entry 60,214 after R-1 to R-3, so the activity entry budget is **62,000** (`LIMIT_BYTES` in `src/web/scripts/check-bundle-size.mjs`); F-54's share is 2,482, the reserve 1,786.

## Consequences

- The first view stays within budget with 3,633 bytes to spare at entry 56,367 (rev. 12), which keeps LCP under 2.5 s plausible on 4G.
- (rev. 9) F-17 overran its share by 715, mostly through follow-ups the design did not plan (splitter, stored preferences). Follow-ups to a feature count against its share and are reported in the QA round that measures them. Raising the 60 KB budget is not needed now. It stays a user decision, raised only if a remedy of decision 5 cannot cover a shortfall.
- (rev. 4) F-19, F-20 and F-28 get the helper for free. Their lazy chunks only add entries to the helper's dependency map, a few bytes each, which they count. Their designs measure the static delta of their `import()` call site against this.
- (rev. 4) If the reserve falls below 1,000 before F-13 and F-17 are measured, apply the review trigger (decision 5) early.
- (rev. 3) The map's lazy part (MapLibre JS, CSS and worker, and F-06's own lazy code, ceiling 5,000) is outside the 60,000 and reported by F-06; MapLibre has no limit of its own beyond the TD's "as shipped, roughly 250 KB".
- **Cost:** owner and report UI load on demand, so they need a loading state and appear about one round trip later. Feature designs F-19, F-20 and F-28 must design for that. The shares are estimates; F-13 and F-17 are the riskiest.
- F-32 checks the final number; CI (ADR-002) fails the build at 60,000 regardless of shares.

## Would choose differently if

- F-31 makes brotli guaranteed and ADR-002 is superseded to measure brotli (about 10–20 % smaller). Then the shares are recomputed, not the decisions on lazy loading.
- F-07's phone measurement shows LCP well under 2.5 s with margin. Then the user may decide to raise the TD budget; the lazy-loading rules stay.
