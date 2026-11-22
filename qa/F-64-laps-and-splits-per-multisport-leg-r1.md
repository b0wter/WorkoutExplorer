# QA F-64 Laps and splits per multisport leg - round 1

- **Date:** 2026-11-22
- **Verdict:** ready with conditions - no Blocker or Major. Conditions: the manual checks of the architecture's Verification table (corpus triathlon in both views, both themes, phone and desktop, German; `ride-12h` scrubbing at 60 fps; LCP and summary timing), which were not run here.
- **Inputs:** `features/F-64-laps-and-splits-per-multisport-leg.md` (OQ-296 to OQ-300, 16 ACs), `architect/F-64-laps-and-splits-per-multisport-leg.md` (LL-1 to LL-8), F-64 notes in `architect/F-42-multisport-sections.md`, `F-46-laps-and-splits.md`, `F-51-split-comparison-chart.md`, ADR-011 item 4. Branch `worktree-agent-ab8142c9e442d3d97`: backend 4bba6b6, merge of main d2e87cd, frontend a504075, reviewed against main a0250ac.

## Build and test results
- `make ci` in the worktree: exit 0 (no load flakes this run). `go vet` clean, gofmt clean, svelte-check 0 errors / 0 warnings. Go: all packages `ok`. Vitest: 90 files, 2,313 tests passed.
- Size: `entry index.html: 60469 / 62000 bytes gzip PASS` (edit.html the same). Built in the scratchpad from `git archive`: main a0250ac **60,404**, merge d2e87cd **60,396**, HEAD **60,469**. F-64 in all: **+65 gzip** (backend −8, frontend +73), ceiling 200.
- **Single sport unchanged:** rendered all 26 page samples with the old (a0250ac) and new `index.html` template through `html/template` (same call as protoserve's `renderPage`): the 18 single-sport samples and `two-rides` are **byte-identical**; only the five `triathlon*` samples differ, plus the new `run-ride-run`. The single-sport sample JSON differs only in the key `Sections: []` → `Legs: []`.
- **Probes** (copy of the worktree in the scratchpad; `src/` untouched):
  - P1, AC 12 (trimmed, untested by the engineer): Go publish test trimming the corpus `triathlon.fit` at the middle of the ride (sample 1560) through the real `workout.Trim`. Trimmed sessions: cycling 0..900, transition 900..960, running 960..2160. Server page: three blocks Ride, T1, Run, no Swim; Ride table starts with the trimmed lap. Same data file in jsdom: laps view Ride, T1, Run; splits view `Ride` (5 km preselected, 2 splits), `T1 · 1:00`, `Run` (1 km, 4 splits); the first ride split selects 0..669 (starts at the trim point).
  - P2, language switch in the splits view (untested by the engineer): triathlon with a run without distance, ride set to 10 km, a ride split tapped, then `use('de', …)` with the real `de.json` and `header()`. Splits headings `Schwimmen`, `T1 · 1:00`, `Radfahren`, `T2 · 1:00`, `Laufen`; laps-view headings translated; `Keine Splits`; drop-down aria-label `Split-Distanz`, ride keeps 10 km; columns German, decimal comma (`10,00`); chart caption `10 km-Splits`; selection kept (1995..2460); after the rebuild a ride row still maps to its split (660).
  - P3: units changed after the splits view was built: laps tables still rewritten per leg (`Pace (/100 yd)`, `Speed (mph)`, `Pace (/mi)`), no warning.
  - P4, AC 16: every synthetic golden through Laps, Splits, Lengths, every option of every leg drop-down, then imperial: no warning, no error, no throw. Multisport goldens: brick-run-bike (Run, Ride), chained-ride-run, ride-transition-ride, swim-bike-distance-fall, triathlon, triathlon-cut-90.
  - P5: a leg drop-down change redraws that leg's chart (10 rows, 10 bars, caption `400 m splits`).
- **Mutants** (16; scratchpad copy, repo test suites for the touched modules): 10 killed, 6 survived.
  - killed: Go `lapLegs` without `max` (TestSections); Go `lapLegs` with a fixed sport (page samples); tap index by `rowIndex` instead of DOM position (M1); a click on a heading's drop-down selects (M3); no seam call after a leg change (M16); whole-workout advance instead of the leg's (M20); unmark every split row on a change (M21); chart for transition laps (M24); `partialLast`/`splitBounds` measured from `d[0]` instead of the window start.
  - **survived** (M12, M17, M23, M15 shown correct by probes P2, P3, P5; M4, M5 correct by reading; tests missing): see QA-64-r1-1.

## Findings

### Minor
- **ID:** QA-64-r1-1
- **Severity / kind:** Minor / risk (test gaps)
- **Route:** frontend
- **Where:** `src/web/src/page/header.ts:30`; `src/web/src/page/enhance.ts:49`, `:54`; `src/web/src/splits/index.ts:210`, `:226`
- **Relates to:** LL-3 (units and language per block), LL-6 (pool rule per session), LL-7 (chart per table), AC 5, AC 6, AC 14
- **Evidence:** these six mutants pass the repo tests:
  - M12 `header.ts` selector `.laps-legs h3` (splits headings included: once the splits view exists the counts differ and the laps-view leg names stop following the language); M17 selector matching nothing. No test checks that the laps view's leg headings follow the language at all.
  - M23 `enhance.ts` blocks selector without `:not(.splits)` (once the splits view exists the units pass warns and leaves the laps tables in the old units); M4 the `k !== meta.laps.length` guard removed.
  - M15 `drawOne(leg.drawn!)` removed from the leg drop-down handler (chart keeps the old preset's bars and caption).
  - M5 pool rule for every swim leg instead of only a `lap_swimming` session: the AC 6 test's open-water case has no pool length in the summary, so it cannot tell.
- **Consequence:** the code is right today (P2, P3, P5), but each of these can regress silently. Suggested tests: a German pass after the splits view is built (laps headings translated, no warning); units after the splits view is built (laps tables rewritten); bars and caption after a leg drop-down change; an open-water swim leg in a file whose summary holds a pool length.

- **ID:** QA-64-r1-2
- **Severity / kind:** Minor / defect (documentation)
- **Route:** frontend (small lane)
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md:96`
- **Relates to:** ADR-011 item 4, F-64 Cost
- **Evidence:** the row says "60,302 → 60,396 before it is F-63 and the F-64 backend commit, so F-64 in all ≤ +167". Measured: main a0250ac (F-63 included) 60,404, merge d2e87cd 60,396, HEAD 60,469. F-63 is +102 and not F-64's. F-64's backend commit is −8, its frontend +73.
- **Consequence:** the ledger books 102 bytes of F-63 against F-64. Record F-64 as **+65** (60,404 → 60,469).

## Deviations reported by the engineer
1. Split tables carry the `splits` class: same as the single-sport splits table, and required so the units pass and the header skip the bundle's tables. Accepted.
2. Multisport pool Lengths view keeps no chart: before F-64 a multisport workout had no chart, so this is "Lengths view unchanged" (LL-8); LL-7 adds charts to leg tables only. Accepted.
3. "No splits" without drop-down or chart: this is LL-5 as written, not a deviation.
4. One line of a single-sport chart test changed: the line asserts the triathlon (multisport), whose rule OQ-299 changed; no single-sport assertion was touched. Accepted.
5. `lib/range` test imports `splitBounds` from `lib/splits`: follows from LL-4's move. Accepted.

## Previous findings
None (round 1).

## Questions
1. **Truncated multisport tail.** `triathlon-cut-90` has no run session (the file ends in the run), so by the F-42 rule the run's samples belong to the last leg, T2. The splits view now shows `T2 · 14:59` with no splits, where before F-64 that distance was in the one splits table. The feature's Reliability constraint keeps the F-42 rule, so this is as specified. Settled by: the user accepting it (suggested; truncated files are an edge case), or a rule that a transition window with advancing distance gets splits.
2. **Transition names after a trim** (F-42, not F-64): trimming away the swim and T1 renames the old T2 to `T1` (P1). Settled by: confirming numbering follows the trimmed file (suggested), as it does today.

## Coverage
- AC 1 (one table per leg, JS on/off, edit page): `TestLegBlocks`, samples, `laps.dom.test` "one table per leg"; template diff read. Traced.
- AC 2 (Swim, Ride, Run; T1/T2 headings with duration; no split crosses a leg): `laps.dom.test` splits-per-leg tests; windowed `splitBounds` tests. Traced.
- AC 3 (lap crossing a leg): `lapCrossing` test, Go/TS `TestSections`, sample. Traced.
- AC 4 (partial per leg, sums, numbers restart): traced (sum within 5 m per row).
- AC 5 (own drop-down, defaults, rebuild one table, keep range): traced; chart redraw only by P5 (QA-64-r1-1).
- AC 6 (pool rule per leg): traced; negative case weak (QA-64-r1-1).
- AC 7 (run, ride, run): `run-ride-run` sample (Go and TS) and dom test. Traced.
- AC 8 (taps on rows and headings, second tap clears): traced.
- AC 9 (speed/pace per leg, trigger per table): traced (`TestLegBlocks` swim pace, dom test imperial headings, `triathlon-triggers` sample).
- AC 10 (defaults on reload, no storage, nothing in the range link): storage asserted in the dom test; state lives only in chunk memory (read). Traced.
- AC 11 (ride, ride unchanged): byte-identical HTML, dom test on `chained-ride-ride`. Traced.
- AC 12 (trimmed): no repo test; verified by probe P1 with the real trim. Traced manually.
- AC 13 (chart per leg table, none for transitions): traced.
- AC 14 (texts in four languages, follow the drop-down): `noSplits` in de/es/fr; behaviour by probe P2 only (QA-64-r1-1).
- AC 15 (bundle, 60 fps, LCP): entry +65 measured, splits code lazy (splits chunk per ADR row). 60 fps and LCP **not run** (manual condition).
- AC 16 (every corpus file renders): probe P4 over all synthetic goldens; `make corpus` with real files not run.
- Privacy: nothing new stored, sent or logged; hidden start and end keep five leg blocks (`TestSectionsHidden`). No new endpoint.
- Not run: real-device hand checks, `make corpus`, `-race` (no new Go concurrency).
