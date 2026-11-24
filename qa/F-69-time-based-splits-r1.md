# QA F-69 Time-based splits - round 1

- **Date:** 2026-11-24
- **Verdict:** ready with conditions - no Blocker or Major; AC 8 and AC 9 (360 px, themes, languages on screen, 60 fps on `ride-12h`, corpus render) need the manual browser checks below
- **Inputs:** `features/F-69-time-based-splits.md`; `architect/F-69-time-based-splits.md` (TS-1 to TS-9, F69-Q1); F-46 LS-5 to LS-9, F-64 LL-4 to LL-7, F-51, ADR-029, ADR-011; AP-7, AP-8, AP-19, AP-20, AP-23, DS-6. Code at main 1a278fd (implementation a42b684)

## Build and test results

- `make ci`: exit 0 (lint, Go and front-end tests, build, size, caddy-check, docker-check, update tests 50 OK, sign tests 12 OK).
- `npm run size`: entry `index.html` / `edit.html` / `my-workouts.html` **60,955 / 62,000 gzip, PASS - unchanged** (target 0 bytes met). Splits chunk `splits-*.js` 6,411 gzip (lazy, not counted).
- Probe (scratch copy of HEAD, not committed): a DOM test on `ride-30min` that chooses "5 min" after a row tap. Passed: range kept on the preset change; row count equals `presetBounds` over `M`; each row's time cell equals `formatElapsed(M[b] − M[a])`; tapping row 2 selects `[bound 1, bound 2]` and `createRangeStats` moving time over that range equals the cell; an imperial switch keeps "5 min" (inside the `<optgroup>`) and the row count.

## Findings

### Minor

- **ID:** QA-69-r1-1
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/page/laps.dom.test.ts` (no test chooses a time preset)
- **Relates to:** AC 3, AC 7, TS-4, TS-5, TS-8
- **Evidence:** the only DOM changes list the options. Nothing committed checks that a time split's time cell shows moving time (`src/web/src/splits/index.ts:80`), that a row tap selects the row's range, that the range is kept, or that units/language keep a time choice. The scratch probe above shows the behaviour is correct today.
- **Consequence:** a regression in the `p.time ? m : d` series switch (`splits/index.ts:268, 277, 300, 304`) or in the moving/elapsed cell would pass CI. Add the probe as a test.

- **ID:** QA-69-r1-2
- **Severity / kind:** Minor / undeclared deviation (preference)
- **Route:** frontend (or architect note)
- **Where:** `src/web/src/lib/splits/index.ts:84`
- **Relates to:** TS-9, DS-6
- **Evidence:** the labels "N min" are built in code (`m + ' min'`), not taken from the text files as TS-9 says. "min" is correct in all four languages, so nothing is visibly wrong.
- **Consequence:** none now. A fifth language or a different abbreviation would need a code change. Either move the label into the texts or note in TS-9 that it is language-neutral.

## Previous findings

None (round 1).

## Questions

None blocking. F69-Q1 was built as suggested ("Time" header showing moving time for time splits). If the user objects, that is a one-text change.

## Coverage

| AC | Code | Test / check |
| --- | --- | --- |
| 1 (50 min: 1-30 min, 5 x 10:00, opens on 1 km) | `lib/splits/index.ts:84` offer rule, `splitBounds` over `M` | `splits.test.ts` "offer", "50 min ... 47 min"; DOM default unchanged (`laps.dom.test.ts:227-229`) |
| 2 (47 min: 4 x 10:00 + 7:00 partial) | `splitBounds`, `partialLast` over `M` | `splits.test.ts` (rows and `partial`) |
| 3 (stop inside row 2; boundary at the stop's start puts it in the next row; panel elapsed ~30:00 / moving 10:00) | boundaries: `splitBounds`; panel: row tap -> LS-8 seam -> `RangePanel.svelte` shows `elapsed` (`rangeTime`) and `moving` (`rangeMoving`) from the same `createRangeStats` the cell uses (`splits/index.ts:77-80`), so cell = panel moving time by construction | unit test (row 2 spans 1800 samples, 600 s moving; stop at boundary -> next row); panel path by reading and the probe; **no committed DOM test** (QA-69-r1-1) |
| 4 (gap across a multiple) | `splitBounds` `while` loop (one row, no empty rows) | `splits.test.ts` gap across two multiples; smart-recording spacing |
| 5 (triathlon legs; clock from the leg start; transitions in no row) | `splits/index.ts:235-236` per-leg `movingOf(M, a, b)`, window `[a, b]` | windowed unit test (leg from sample 1000, rows sum to window); DOM: open-water leg of `triathlon` golden has 1/5/10 min (`laps.dom.test.ts:717`) |
| 6 (pool swim / pool leg: no time group; no distance: no section) | `pool ?` and `movingOf(..., lapSwim)`; section gated on distance as before | unit tests; DOM lap-swim leg options (`laps.dom.test.ts:716`); AC 4 test "without distance" unchanged |
| 7 (row tap, one bar per row, range kept, not in `#range=`) | unchanged LS-8 seam; chart slots from `totals`; preset only in chunk memory | probe (row tap, range kept); chart and fragment by reading only (no change in that code) |
| 8 (360 px, themes, four languages) | native `<optgroup>`; texts in de/es/fr | **manual** |
| 9 (entry unchanged; 60 fps on 12 h with 1-min splits; every corpus file renders) | - | entry 60,955 unchanged (measured); fps and corpus render **manual** |

Declared deviations checked: the two edited assertions in `laps.dom.test.ts` only append the time options and the optgroup label; the distance behaviour they check is unchanged (accepted). The `lapSwim` flag from `sessions[].sub_sport` matches TS-3 ("or its window's session is lap swimming"). The "texts in the `splits` group instead of `page`" deviation is not one: `i18n/groups.ts:18` puts the `splits` text files in the `page` group, as TS-9 wants.

Not run: browser checks (AC 8, AC 9 fps, corpus render on screen), the German UI on screen.

Manual checks for the user (protoserve):
1. 360 px portrait: the drop-down with the "Time" group fits; no horizontal page scroll; light and dark.
2. `ride-12h`, 1 min: about 720 rows, no chart, scrubbing smooth (60 fps).
3. A ride with a stop: choose 10 min, tap the row with the stop; the panel shows elapsed > moving and moving = the row's time.
4. Corpus triathlon: bike and run drop-downs have "Time", the pool-swim leg has none; T1/T2 unchanged.
5. German, Spanish, French: group heading Zeit / Tiempo / Temps; changing the language keeps the chosen preset.
