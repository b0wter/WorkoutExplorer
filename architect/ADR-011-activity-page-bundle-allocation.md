# ADR-011 Activity page bundle: allocation of the 60,000 bytes and what loads lazily

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 7 of 15 Oct 2026, with the F-07 QA r2 close-out; revision 8 of 24 Oct 2026, F-14 design: F-13 closed at 1,717 measured, its unused 3,283 bytes return to the reserve; revision 9 of 29 Oct 2026, F-17 QA r1: F-17 at 4,715 measured, 715 over its share accepted; revision 10 of 29 Oct 2026, F-19 QA r1: F-17 final 4,698, F-18 final 1,067, F-19 374 of 400 measured, reserve 3,640; revision 11 of 30 Oct 2026, F-20 QA r1 (QA-20-r1-4): F-20 +2 of its 100 ceiling, reserve 3,638; revision 12 of 30 Oct 2026, F-20 QA r2 (QA-20-r2-2): F-20 +7, reserve 3,633; revision 13 of 9 Nov 2026, F-39 and F-40 designs: shares 600 and 700 from the reserve, the How to use dialog lazy; revision 14 of 9 Nov 2026, F-42 design: reserve 955 after the F-40 share, below the review trigger; review result: F-42's browser code lazy, static ceiling 300
- **History:** [architect/history/ADR-011-activity-page-bundle-allocation.md](history/ADR-011-activity-page-bundle-allocation.md) (revision list, earlier shares and reserves, the retired 45,000 trigger)
- **Features:** F-05 (triggers it); F-06, F-07, F-13, F-14, F-17, F-18, F-19, F-20, F-25, F-26, F-28, F-33, F-39, F-40, F-42 (bound by it); F-32 (checks it)
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
| **Reserve** | **983** (60,000 − 59,017) |
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
   | (rev. 14) F-42 multisport sections: the sessions check, the `import()` call site, a chart decoration seam, separator rows in the laps and units enhancers, CSS | **300 ceiling** (F-42 design MS-9, Cost); the rest is lazy. Measured and recorded here like F-39 |
   | Reserve (architect decides who gets it) | **1,655** after F-39 (9 Nov 2026: 60,000 − entry 58,345). The baseline before F-39 was 57,746, not 57,399: F-36 to F-38 took 347 unbooked, so the reserve was 2,254, not 2,601. **1,145 after F-40's measured 510** (planned 955 after its 700, below the review trigger, decision 5). (rev. 14) **845 after F-42's ceiling**. Review result: F-42 goes lazy (item 2); the next feature that adds static code applies a remedy of decision 5 before it is built |

5. **Review trigger (rev. 7):** the reserve falls **below 1,000**, or a feature cannot fit its share plus the reserve. The architect then revisits this ADR before that feature is implemented.
   - From F-07 on, the CI line measures the activity page itself, because the prototype page replaced the F-01 placeholder as `index.html`'s entry. The ledger is therefore read from the CI line plus the open shares.
   - There is no absolute entry-size trigger: the reserve trigger says the same thing without going stale.

   Remedies in this order:
   - move more code behind `import()`;
   - drop the dead parts of the F-01 placeholder (done in F-07);
   - replace uPlot with an own canvas renderer (ADR-009, "Would choose differently if").
   
   Raising 60 KB goes to the user.

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
