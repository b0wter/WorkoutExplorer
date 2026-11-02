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
