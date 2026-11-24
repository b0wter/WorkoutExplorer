# QA F-67 Multisport leg picker - round 1

- **Date:** 2026-11-24
- **Verdict:** ready with conditions - no Blocker or Major; four Minors and the manual browser/device checks below
- **Inputs:** `features/F-67-multisport-leg-picker.md` (AC, OQ-315 to OQ-325), `architect/F-67-multisport-leg-picker.md` (LP-1 to LP-12), ADR-011 item 4; commit 2131806 (`git diff 80cb450 2131806`, 25 files)

## Build and test results
- `make ci` (foreground): exit 0. svelte-check 430 files, 0 errors, 0 warnings; vitest 93 files, 2,401 tests passed; all Go packages ok; caddy-check, docker-check, update-check ok.
- Size check: viewing entry `index.html` **60,963 / 62,000** gzip (same for edit and my-workouts); 60,565 before = **+398**, inside the F-67 ceiling of 400 (98 over the 300 target). Owner chunk `start.svelte` **19,946** gzip (engineer's before figure 19,943; +3, hash churn; under 20,000). Before figures taken from ADR-011, not rebuilt.
- Go flakiness: `cd src/server && go test -count=3 ./...` (foreground), all 23 packages ok, plus the `make ci` run: **4 of 4 passes, failure not reproduced**. F-67 changes no Go code. No finding raised; the engineer should record the failing test name and output if it recurs (likely candidates by run time and load sensitivity: `cmd/workout-explorer`, `internal/ingest`, `internal/publish`).

## Findings

### Minor

- **ID:** QA-67-r1-1
- **Severity / kind:** Minor / defect
- **Route:** frontend (architect for the LP-9 wording)
- **Where:** `src/web/src/sections/index.ts:609-615`, `src/web/src/components/map/map-view.ts` (`fit`, `fitTo`, resize handler)
- **Relates to:** LP-8, LP-9, OQ-317, OQ-319, F-13 D-20
- **Evidence:** a pick sets `fitTo` to the leg box and `moved = false`. LP-8's automatic return (a range outside the leg) sets the view to `null` and the select to "Whole workout" but, as LP-9 prescribes, does not call the map. `fitTo` stays the leg box and `moved` stays false, so the next map `resize` (rotation, splitter drag, pin/unpin, window resize) jumps the camera to the old leg while the picker says "Whole workout". Same if the viewer picks a leg and an LP-8 return happens before the map is ready: the queued `fit(leg)` runs on map-ready. Read from code; not reproduced in a browser.
- **Consequence:** the camera moves to a leg nobody shows any more, with no viewer action. Fix in lazy code: on the automatic return, reset the refit target to the route without moving (e.g. a target-only handle call), or have `fit` not survive a return. LP-9's "until the viewer moves" needs a line on this.

- **ID:** QA-67-r1-2
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/components/charts/CursorReadout.svelte:23-24`
- **Relates to:** AC 2, LP-6, LP-10
- **Evidence:** the readout's `sport` starts `undefined`, so a freshly keyed readout reads the workout sport (`multisport` → "Speed km/h") until the first cursor sample. In a Run or Swim leg view the readout says "Speed · km/h  –" above a "Pace · /km" chart until the viewer hovers.
- **Consequence:** label disagrees with the chart in every leg view before the first hover. Seed from the view's sport (costs entry bytes; the reserve to the F-67 ceiling is 2, so measure and book it) or accept and note.

- **ID:** QA-67-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/sections/sections.dom.test.ts`, `src/web/src/components/charts/ChartStack.dom.test.ts`
- **Relates to:** AC 4
- **Evidence:** no test switches language (`onLanguage` → `fill`) and checks that the selected index and `look.view` survive with translated option texts, and no test switches units in a leg view and checks the zoom and the pace unit (/mi). Both hold by reading the code (`fill` keeps `selectedIndex`; units do not touch `Look.view`).
- **Consequence:** AC 4 is traced to code only; a later change to `fill` (e.g. rebuilding `picks`) would go unnoticed.

- **ID:** QA-67-r1-4
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:** `architect/F-67-multisport-leg-picker.md` LP-6
- **Relates to:** LP-6
- **Evidence:** LP-6 says "the other five specs stay the workout's (cadence keeps /min)"; the code uses `presentCharts(data, units, view.sport)` and `readoutFields(…, sport)` for all specs. Only the cadence unit label differs (`cadenceUnit(sport)`, identity conversion), so values are unchanged and the result matches the F-64 tables (OQ-300). I accept the deviation, but the live doc contradicts the code.
- **Consequence:** the next reader of LP-6 is misled. One-line update.

## Engineer's deviations
- **All chart specs follow the leg's sport:** accepted (QA-67-r1-4 for the doc). Only the cadence unit label changes; no value conversion.
- **Whole view through the leg code path:** accepted. `subarray(0, N)` is a view (no copy); `gaps` filter is one allocation per build; `[at[0], at[N-1]]` equals `xScale`'s min/max on both axes (`lib/charts/axis.ts`), except a multi-sample workout with zero time span, which now widens to +1 instead of a zero range (better). The test "Whole workout restores exactly" compares the decimated data of every chart before and after.
- **"Speed km/h" with "–" in transitions:** accepted; AC 3 asks only for "–".
- **Map callbacks as a list:** accepted. Without a route, each pick pushes a never-run callback (bounded by clicks; negligible).
- **Pick resets the "moved" flag:** accepted as "fits as the initial fit" (OQ-317), but it is what makes QA-67-r1-1 visible.
- **`sections` added to the `page` text group:** accepted; same pattern as `splits`, `zones`, `pins`; costs two keys in each `lang-page-*.json`, not entry bytes.
- **Drop-down, no picker without distance:** accepted by the user.

## Previous findings
None (round 1).

## Questions
None that block. If the operator's leg-switch measurement exceeds 100 ms, the design's fallback (keep uPlot instances) costs entry bytes the ceiling no longer has (2 left): that would need an ADR-011 booking first.

## Coverage
| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 Picker entries, no picker for single sport / two cycling sessions | `sections/index.ts` `pickable`, `startSections`; `ordered` gate in `start.ts` | sections.dom "LP-1", "AC 1, LP-2" (Swim · Ride · Run; numbering; no-toolbar case) | traced (the label is "Ride", F-42's name, not "Bike") |
| 2 Zoom on time and distance, pace units, restore exactly | `MetricChart` subarrays and scale, `ChartStack` key and LP-5 | ChartStack.dom F-67 block (run/swim/bike titles, x range, y fit, distance axis, LP-5, restore) | traced; mi/100 yd not tested in a leg view |
| 3 Readout per leg, "–" in T1/T2, fullscreen matches | `CursorReadout`, `order.ts` `sportIn`; strip mounts the same component | ChartStack.dom "AC 3" | traced; fullscreen by shared component only |
| 4 Units/language keep the leg | `Look.view`, `fill` | none | code only (QA-67-r1-3) |
| 5 Cursor and range in a leg; OQ-316/319 | `indexAt` clamp, LP-8 listener; RangePanel per section | ChartStack.dom "LP-7", sections.dom "LP-8", RangePanel.dom | traced; long-press in a leg not separately tested |
| 6 Map, laps; hidden start/end | `map-view` `fit` (NaN positions skipped), tables untouched | map-view.dom "LP-9" | traced; privacy holds: the fit reads the decoded public positions only. See QA-67-r1-1 |
| 7 Reload / Back | no storage, no history write | sections.dom (view null at start) | traced by code |
| 8 Themes, 360 px, layouts, keyboard | native `<select>`, `sections.css` tokens | none | **manual** |
| 9 No JS | no server or markup change | - | traced (diff touches no server/no-JS file) |
| 10 Budgets, LCP, layout jump, perf, corpus | entry 60,963, toolbar row reserved by `has-distance` | size check PASS; corpus suite in vitest/Go | size traced; LCP/perf **manual** |
| 11 Texts in four languages | `sections/texts.ts`, de/es/fr json | i18n key-parity checks in vitest | traced |

**Manual browser/device checks still required (conditions):**
1. Leg switch ≤ 100 ms on a mid-range phone (User Timing or ADR-016 recorder around the rebuild), Swim, Bike, Run and back.
2. Scrubbing at 60 fps in a leg view on the longest multisport corpus file (ADR-016 recorder).
3. 360 px portrait: toolbar with toggle + picker, no horizontal page scroll in all four languages ("Entrenamiento completo"); landscape and desktop two-column; pinned map.
4. Light and dark theme of the select (native popup colours).
5. Keyboard: Tab to the select, open and choose with Enter/Space/arrows; screen reader announces "Leg".
6. Map fit on a pick, refit on "Whole workout", no move for a pool swim leg; hidden start/end workout: fit shows nothing beyond the public route.
7. LCP unchanged and no chart jump when the picker appears.
8. Edit page shows the same picker.

Not run: browser, device, LCP, the frame-time recorder.
