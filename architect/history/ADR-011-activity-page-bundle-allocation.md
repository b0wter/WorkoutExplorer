# ADR-011 Activity page bundle allocation: history

History of [architect/ADR-011-activity-page-bundle-allocation.md](../ADR-011-activity-page-bundle-allocation.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed). Read it only when you need the history.

## Header date line as written (revision 7)

- **Date:** 11 Oct 2026 (revision 1, F-05 QA r1: 300 bytes of the reserve go to F-05, reserve 2,400; revision 2, 12 Oct 2026, F-05 QA r2 and the touch scroll lock: 200 more bytes go to F-05, reserve 2,200; revision 3, 12 Oct 2026, F-06 design: 300 bytes of the reserve go to F-06 for Vite's dynamic-import helper, F-06 share 1,800, reserve 1,900; revision 4, 12 Oct 2026, F-06 implementation: the helper measured 785 bytes, not 300; 300 more bytes go to F-06, share 2,100, reserve 1,600; the helper is charged once, to F-06; revision 5, 14 Oct 2026, F-06 QA r1: the r1 fixes may raise the F-06 share to at most 2,200 (reserve at least 1,500); the final number is recorded at F-06 QA r2. Close-out, F-06 QA r3: share 2,100 final, measured static 2,029–2,056, fallback unused, reserve 1,600; revision 6, 15 Oct 2026, F-07 design: F-07's page wiring gets its own share of 1,500 net, funded from the measured headroom, not from F-06's share or the reserve; the CI entry becomes the activity page. **Revision 7**, 15 Oct 2026, F-07 implementation stop: rev. 6 was wrong, because F-06's measurement (b) does not contain the data file reader, so its "headroom" did not exist. F-07's share becomes 225 measured (ceiling 300) and is taken from the reserve, which becomes 1,447. The absolute 45,000 trigger is retired in favour of the reserve trigger)

## Context, rev. 7 paragraphs on rev. 6's error

(rev. 7, replaces rev. 6's paragraph) **Measured ledger after F-07.** Rev. 6 read F-06's measurement (b) (42,557) as including the data file reader and found 2,843 bytes of headroom. It does not include the reader: (b) imports only `ChartStack`, `createCursorStore` and `RouteMap`, and none of the reader's `data file: …` strings are in that build. The engineer's measurements on 15 Oct 2026:

Against the plan, B0′ came in 103 below this table's projection plus the F-05 and F-06 reserve draws (42,831 + 500 + 2,100 = 45,431). The only unplanned cost is F-07's wiring (225), which no revision before rev. 6 had a line for. That is where the 153-byte gap against the old reserve of 1,600 comes from (60,000 − 45,553 = 14,447 versus 14,600 planned).

Ledger rows as written: | + F-07 page wiring, net of the retired placeholder → **E**, the CI entry | +254 → **45,582** (close-out, F-07 QA r2, after the body CSS moved into the bundle; QA measured 253 against its B0′ of 45,329; 225 → 45,553 before the move) | and | **Reserve** | **1,418** (close-out; 1,447 before the move) |

## Decision 4, share cells as written

| Feature | Share (bytes gzip) |
| --- | --- |
| F-06 static glue | **2,100, final** (rev. 5 close-out, F-06 QA r3: measured static 2,029–2,056; the 2,200 fallback was not used). (rev. 5: at most 2,200 if the QA r1 fixes needed it; QA r1 measured 2,038–2,065.) (rev. 4: the helper measured 785 bytes gzip with the lazy CSS list, F-06's own code about 1,281, total 2,066; F-06 design v1.2. Rev. 3: 1,800, which assumed 300 for the helper. Originally 1,500.) The helper is Vite's dynamic-import helper `__vitePreload`. The first `import()` of the entry brings it in; F-19, F-20 and F-28 reuse it and must **not** count it again in their shares |
| F-07 page wiring (rev. 7) | **254, final** (close-out, F-07 QA r2: E 45,582 − B0′ 45,328, including the body CSS moved from `index.html`; 225 before the move); ceiling 300 = E − B0′. B0′ is F-06's measurement (b) plus a temporary `loadDataFile` import, measured right before F-07. It covers the prototype page (query, load, error text, cursor store, the two components, User Timing marks, the `import()` call site of the lazy scrub recorder), less the retired F-01 placeholder (`App.svelte`, `lib/health`). It is taken **from the reserve**, not from F-06's 2,100. F-07's pixel-ratio fix (QA-06-r4-1) and the scrub recorder are lazy and cost 0 here. Above 300 in a QA round: stop and report. The close-out at F-07 QA records the final E. (Rev. 6 said 1,500 net of B0 from 2,843 "unallocated" bytes. That headroom was the data file reader, which (b) never contained) |
| Reserve (architect decides who gets it) | **1,418, final after F-07** (close-out, F-07 QA r2: 60,000 − E 45,582 − 13,000 planned). (rev. 7: 1,447 at E 45,553, before the body CSS move). (1,600, final after F-06; rev. 5 close-out; the 2,200 fallback for F-06 was unused, so nothing more was taken). (rev. 4: 300 more given to F-06, the helper being larger than estimated. Rev. 3: 1,900, 300 given to F-06 for the dynamic-import helper. Rev. 2: 2,200, 200 given to the F-05 QA r2 fixes and the scroll lock, F-05 design v1.4. Rev. 1: 2,400, 300 given to the F-05 QA r1 fixes, design v1.2. Originally 2,700) |

## Decision 5, the retired trigger

   - (Rev. 0–6 had an absolute trigger: "F-07's entry total exceeds 45,000". It was set when the plan put the entry after F-07 at about 44,300. The F-05 and F-06 reserve draws of rev. 1–4 (1,100 bytes) moved that point to about 45,400 without the number being updated. F-07 measured 45,553 and fired it. The review found no remedy needed: the overshoot over the moved point is F-07's 225, and the reserve still holds 1,447, above the 1,000 floor. The absolute trigger is retired because the reserve trigger says the same thing without going stale.)

## Consequences, first bullet as written

- The first view stays within budget with 1,418 bytes to spare (F-07 close-out; 1,600 at rev. 4), which keeps LCP under 2.5 s plausible on 4G.

## Revision 17 (25 Nov 2026, size review before F-74): moved out of the live ADR

The live ADR was reduced to the current state. Everything below was removed from it as written in revision 16 (last booked row: F-72, entry 61,577, reserve 423).

**Header date line:** 11 Oct 2026; revision 7 of 15 Oct 2026, with the F-07 QA r2 close-out; revision 8 of 24 Oct 2026, F-14 design: F-13 closed at 1,717 measured, its unused 3,283 bytes return to the reserve; revision 9 of 29 Oct 2026, F-17 QA r1: F-17 at 4,715 measured, 715 over its share accepted; revision 10 of 29 Oct 2026, F-19 QA r1: F-17 final 4,698, F-18 final 1,067, F-19 374 of 400 measured, reserve 3,640; revision 11 of 30 Oct 2026, F-20 QA r1 (QA-20-r1-4): F-20 +2 of its 100 ceiling, reserve 3,638; revision 12 of 30 Oct 2026, F-20 QA r2 (QA-20-r2-2): F-20 +7, reserve 3,633; revision 13 of 9 Nov 2026, F-39 and F-40 designs: shares 600 and 700 from the reserve, the How to use dialog lazy; revision 14 of 9 Nov 2026, F-42 design: reserve 955 after the F-40 share, below the review trigger; review result: F-42's browser code lazy, static ceiling 300; revision 15 of 11 Nov 2026: size review C1–C3 applied, reserve 3,493; revision 16 of 20 Nov 2026: F-54 over the line, remedy R-1 to R-3 and the conditional 62,000 budget (decision 6); outcome of 20 Nov 2026: entry 60,214 after R-1 to R-3, budget 62,000

**Context: F-05 projection**

| Part | Bytes gzip |
| --- | --- |
| F-01 base (Svelte core, placeholder) | 10,613 |
| `lib/datafile` (F-04) | 2,793 |
| uPlot + CSS | 22,359 |
| F-05 own code incl. Svelte block runtime | 7,066 |
| **Total** | **≈ 42,800** |
| **Remaining** | **≈ 17,200** |

**Context: measured ledger (rev. 7 to rev. 16)**

| Part | Bytes gzip |
| --- | --- |
| B0 = F-06 (b): Svelte core, F-01 placeholder, uPlot + CSS, F-05, F-06 static glue | 42,525 |
| + `lib/datafile` reader (`loadDataFile`) → B0′ | +2,803 → 45,328 |
| + F-07 page wiring, net of the retired placeholder → **E**, the CI entry | +254 → **45,582** |
| (rev. 8) + F-13 layout, client part → entry 47,299 | +1,717 |
| + F-14 units, pace, theme → entry 50,221 | +2,922 |
| (rev. 10) + F-17 range, axis toggle, splitter, prefs → entry 54,919 | +4,698 |
| + F-18 laps and splits → entry 55,986 | +1,067 |
| + F-19 page switch, owner `import()`, glue data seam, marks prop → entry 56,360 | +374 |
| (rev. 12) + F-20, the owner chunk's new hash in the entry → entry 56,367 | +7 |
| + F-25, F-26, F-28, F-33 and follow-ups, 5 Nov 2026 (QA-33-r1-1) → entry 57,399 | +1,032 |
| + F-36 to F-38, unbooked, 9 Nov 2026 → entry 57,746 | +347 |
| + F-39 How to use → entry 58,345 | +599 |
| + F-40 shareable range, with the F-39 QA r1 follow-ups → entry 58,855 | +510 |
| + F-41 summary tints (+133) and the F-40 QA r1 minors → entry 58,997 | +142 |
| + How to use on its own row (user 10 Nov 2026) → entry 59,026 | +29 |
| + Range panel actions as links (user 10 Nov 2026) → entry 59,017 | −9 |
| + F-39 range hint as an info box → entry 59,202 (two steps, +125 and +60) | +185 |
| + F-42 multisport sections, static part → entry 59,487 | +285 |
| + F-42 QA r1 minors → entry 59,510 | +23 |
| + F-42 leg names in every chart, cursor in `--fg` → entry 59,483 | −27 |
| + F-42 first leg's name kept when narrow → entry 59,485 | +2 |
| (rev. 15) − size review C1–C3 ([review](ADR-011-size-review-2026-11-11.md)) → entry 56,507 | −2,978 |
| + F-43 device line and F-44 pool cells → entry 56,655 | +148 |
| + F-46 laps and splits (LS-4 lazy `src/splits/`) → entry 56,351 (F-45 had left 56,661) | −304 |
| + F-48 strokes → entry 56,651 | +259 |
| + F-49 zones → entry 56,916 (115 over its 150 ceiling, accepted 13 Nov 2026); follow-up `cf493f6` → 56,939 | +265, +22 |
| + theme commit `659c6be` (unbooked) and F-51 comparison chart → entry 57,030 | +55, +36 |
| + F-50 best efforts → entry 57,176 | +142 |
| + F-58 summary dialog → entry 57,456 (51 over its 200 ceiling, reported) | +251 |
| + F-57 GPX download → entry 57,603 | +147 |
| + F-56 map pins → entry 57,858 (7 over, accepted) | +257 |
| + F-53 My workouts → entry 57,926 (4 over, reported) | +64 |
| (rev. 16) + F-54 translated interface → entry 60,214 (baseline 57,732, not 57,926; budget 62,000 per decision 6) | +2,482 |
| + F-61 satellite layer → entry 60,281 | +67 |
| + F-62 show my location → entry 60,302 | +21 |
| + F-63 (+102) and F-64 laps per leg (+65) → entry 60,469 | +167 |
| + F-65 owner-added laps → entry 60,521 | +52 |
| F-66 remove auto-laps → entry 60,463 (drop before F-66 unbooked) | 0 |
| + AP-8 laps footnote → entry 60,565 | +100 |
| + F-67 multisport leg picker → entry 60,963 | +398 |
| + GitHub link (footer, unbooked, other engineer) → entry 61,139 (61,031 at `654004a`) | +108 |
| + F-68 extra sensor streams → entry 61,367 | +228 |
| Owner chunk trim (published page lazy) → entry +13 (hash churn); main 25 Nov 2026 61,382 | +13 |
| + F-73 handover line in `restoreSplit` → 61,383 (from 61,377) | +6 |
| + F-72 swim distance in yards → entry 61,577 | +195 |

**Decision 4, share table (rev. 16)**: each closed feature's final share is the "+" figure of the ledger above. Notes kept from the cells:

- F-06 static glue 2,100 final; includes Vite's dynamic-import helper `__vitePreload` (785 gzip with the lazy CSS list, rev. 4). The first `import()` brought it in; later `import()` sites reuse it and count only their preload-list names.
- F-07 254 final (ceiling 300, from the reserve). F-13 1,717 final (planned 5,000). F-14 2,922 final (2,500 + 422 from the reserve). F-17 4,698 final (planned 4,000, overrun accepted rev. 9). F-18 1,067 final (planned 1,500). F-19 400 (374 measured). F-20 ceiling 100 (7 measured). F-33 300 planned, 240 measured. F-39 600 planned, 599 measured. F-40 700 planned, 510 measured. F-42 ceiling 300, 285 measured; lazy `src/sections/` 825 + 191 CSS, `lib/sections` 398. F-43 ceiling 100 and F-44 ceiling 150, together 148. F-48 ceiling 300, 259. F-49 ceiling 150, 265 (accepted). F-51 ceiling 80, 36; splits chunk 4,545 JS, 728 CSS, shared zone tokens 308. F-50 ceiling 150, 142; efforts chunk ≈ 2,610 + 369. F-58 ceiling 200, 251 (reported); summary-info 1,524, shared dialog shell 639 + 402. F-57 ceiling 150, 147; gpx 651. F-56 ceiling 250, 257 (accepted); pins 2,956 + 1,045. F-53 ceiling 60, 64 (reported); mine 1,433 + 310, `lib/mine` 885. F-54 target 800, ceiling 1,000, final 2,482 (decision 6). F-61 ceiling 100, 67; map-view 7,992 JS. F-62 ceiling 50, 21; map-view 8,572, location 1,120. F-64 ceiling 200, 65; splits 6,183. F-65 ceiling 50, 52 (noise, reported). F-66 ceiling 0, 0; owner chunk 18,907 → 19,701 after F-65 and the pin dialog. F-67 target 300, ceiling 400, 398; sections chunk 826 + 382. F-68 target 150, ceiling 250, 228; dynamics 1,002 + 211, texts in the `page` group. Owner chunk trim: `start.svelte` 20,040 → 19,424, lazy `published` 1,491. F-72 ceiling 200, 195; owner chunk 19,425 → 19,439; "the next feature that adds entry code needs a size review first".

**Decision 4, reserve cell (rev. 16), last values:** 1,786 after F-54 against 62,000; 1,719 after F-61; 1,698 after F-62; 1,531 after F-64; 1,479 after F-65; 1,537 after F-66; 1,435 after the AP-8 footnote; 1,037 after F-67; 633 after F-68; 618 on main 25 Nov 2026; 423 after F-72. Earlier values in the cell went back to 1,655 after F-39 (60,000 budget) and the rev. 14 review result "F-42 goes lazy".

**Decision 6, budget outcome as written:** if the entry after R-1 to R-3 is at most 59,000, the 60,000 line stays; above 59,000, the activity entry budget becomes 62,000 (ADR-002 CI line and TD Budget row) and the reserve is 62,000 minus the measured entry. Outcome 20 Nov 2026: entry 60,214 after R-1 to R-3, budget 62,000 (`LIMIT_BYTES`), F-54's share 2,482, reserve 1,786.

**Consequences, as written:** "The first view stays within budget with 3,633 bytes to spare at entry 56,367 (rev. 12)." "(rev. 9) F-17 overran its share by 715 … Raising the 60 KB budget is not needed now." "(rev. 4) F-19, F-20 and F-28 get the helper for free …" "(rev. 4) If the reserve falls below 1,000 before F-13 and F-17 are measured, apply the review trigger early." "(rev. 3) The map's lazy part … ceiling 5,000 … MapLibre as shipped, roughly 250 KB." "Cost: owner and report UI load on demand … F-13 and F-17 are the riskiest." "F-32 checks the final number; CI (ADR-002) fails the build at 60,000 regardless of shares."

**Decision 5 remedies as written (rev. 7):** move more code behind `import()`; drop the dead parts of the F-01 placeholder (done in F-07); replace uPlot with an own canvas renderer (ADR-009). Raising 60 KB goes to the user.

## Revision 11 (30 Oct 2026, F-20 QA r1, QA-20-r1-4): text replaced

- Ledger row | **Reserve** | **3,640** (rev. 10) |; decision 4 reserve cell "**3,640** (rev. 10: 60,000 − entry 56,360). F-20 and F-28 take their call-site shares from it in their designs"; Consequences "3,640 bytes to spare at entry 56,360 (rev. 10)"

## Revision 10 (29 Oct 2026, F-19 QA r1): text replaced

- Ledger rows: | (rev. 9) + F-17 range, axis toggle, splitter, prefs, measured at F-17 QA r1 → entry 54,936 | +4,715 |, | Planned share F-18 | 1,500 | and | **Reserve** | **3,564** (rev. 9) |
- Decision 4 cells: F-17 **4,715** (rev. 9, measured at F-17 QA r1: entry 54,936 − 50,221 ...; final at F-17 close; the r2 fixes may add at most 100); F-18 1,500; Reserve **3,564** (rev. 9: 60,000 − entry 54,936 − F-18 1,500)
- Consequences: "The first view stays within budget with 3,564 bytes to spare after F-18's share (rev. 9)"

## Revision 9 (29 Oct 2026, F-17 QA r1): text replaced

- Ledger rows: | Planned shares F-14 2,500, F-17 4,000, F-18 1,500 | 8,000 | and | **Reserve** | **4,701** (rev. 8) |
- Decision 4 cells: F-17 4,000; Reserve **4,279** (60,000 − entry 50,221 − F-17 4,000 − F-18 1,500)
- Consequences: "The first view stays within budget with 4,701 bytes to spare after the planned shares (rev. 8)"; "(rev. 8) F-13 came in at 1,717 of 5,000. F-17 (4,000) is now the riskiest share; its design measures early, with a temporary import as F-05 and F-06 did, and stops at its share."

## Revision 8 (24 Oct 2026, F-14 design): text replaced

- Ledger rows: | Planned shares F-13 5,000, F-14 2,500, F-17 4,000, F-18 1,500 | 13,000 | and | **Reserve** | **1,418** |
- Decision 4 cells: F-13 layout (client part) 5,000; F-14 2,500 (no overrun rule); Reserve **1,418, final after F-07** (close-out, F-07 QA r2: 60,000 − E 45,582 − 13,000 planned)
- Consequences: "The first view stays within budget with 1,418 bytes to spare (F-07 close-out)"; "(rev. 7) The margin is thin for the two riskiest shares, F-13 (5,000) and F-17 (4,000): together with the reserve they have 10,418. Their designs measure early, with a temporary import as F-05 and F-06 did, and stop at their share."
