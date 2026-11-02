# QA F-14 Units, pace or speed, and theme - round 1

- **Date:** 2026-10-25
- **Verdict:** ready. No Blocker or Major finding. Six Minor findings (two test or doc gaps on the pace chart, a unit factor outside `lib/format`, an unthemed map marker, a pre-existing cadence unit mismatch, the budget ledger) and one question for the user about the slowest pace for hikes, swims and rows.
- **Inputs:** `features/F-14-units-and-theme.md`; spec DS-1 to DS-4 and AP-2; TD Code structure (front-end dependency rules) and the "Units store" glossary entry; `architect/F-14-units-and-theme.md` v1.0 with the OQ-66 update of 25 Oct; ADR-027; ADR-011 rev. 8; decision log 24 and 25 Oct (OQ-66 to OQ-68). Code at `f99c008` (F-14 commits `aa27a1a`, `69d861d`), clean working tree. The user checked the build by hand on protoserve on 25 Oct and accepted the slowest pace values, the 0.3 m/s stop threshold and the dark colours.

## Build and test results

- `make ci`: exit 0. go vet clean; `npm ci` 0 vulnerabilities (EBADENGINE warning: node v22 against `>=24`, already there before F-14); svelte-check 284 files, 0 errors, 0 warnings; `go test ./...` all packages ok (cached); vitest 39 files, 1,043 tests passed; build ok.
- Size: activity entry **50,212 / 60,000 B gzip**, PASS (JS 47,512 + CSS 2,700); upload entry 2,272 / 8,000, PASS; upload entry has no storage API and no shared chunk, PASS. F-14 share as measured: 50,212 − 47,299 (entry at F-13 close, ADR-011 rev. 8) = **2,913 B**, 413 B over the 2,500 share. That is inside the 500 B the design lets F-14 take from the reserve, and the reserve stays at 4,288 (well above 1,000).
- Mutant (scratchpad copy, `src/` untouched): I reverted `69d861d`'s `MetricChart.svelte` change, so values are converted after decimation instead of before. All 1,043 tests still pass (see QA-14-r1-1).

## Findings

### Minor

- **ID:** QA-14-r1-1
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/components/charts/MetricChart.svelte:59`, `:106`
- **Relates to:** AC 5 ("a stop is a gap in the line"), DS-2, design OQ-66 update
- **Evidence:** Only `specs.test.ts` checks that `toDisplay` returns NaN for a stop. No test checks that the chart turns a stop into a break. Reverting the chart to "decimate raw speed, convert afterwards" (the code before `69d861d`) passes the whole suite. With that revert, a stop's 0 m/s becomes a bucket minimum, and `toDisplay(0)` = NaN reaches uPlot as a data value instead of `null`.
- **Consequence:** Nothing guards the user-decided gap behaviour at the place where it is drawn. A refactor of the conversion order would slip through CI. Add a ChartStack or MetricChart test: a run with a stop run longer than one bucket gives a `null` in the pace series and no NaN.

- **ID:** QA-14-r1-2
- **Severity / kind:** Minor / defect (dependency rule)
- **Route:** frontend
- **Where:** `src/web/src/lib/charts/specs.ts:76`
- **Relates to:** TD Code structure ("`lib/format` owns ... every conversion factor ... no other module holds one"), design D-1
- **Evidence:** `(v) => v * (metric ? 3.6 : 3600 / 1609.344)` repeats the speed factors of `lib/format` (`MPS_TO_MPH`, `M_PER_MILE`). Every other chart conversion goes through `toElevation`, `toTemperature` or `paceSeconds`.
- **Consequence:** There are now two homes for the speed conversion. F-17 and F-25 will copy whichever one they find first. Add a `toSpeed(v, units)` export to `lib/format`.

- **ID:** QA-14-r1-3
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.css:9-17`
- **Relates to:** design D-9 ("every colour of ... our map layers is a custom property ... no colour literal stays in components"), DS-4
- **Evidence:** The map cursor marker still uses `border: 2px solid #fff; background: #222`, under the comment "Colours are placeholders until F-14". It does not use `--cursor` or `--marker-stroke`. The hygiene colour test covers only `page.css`, so the lazy map CSS is not checked.
- **Consequence:** In dark mode the marker keeps its light-theme look, while the chart cursor line (`--cursor`) and the start and finish markers (`--marker-stroke`) switch. It is still visible because of the white ring, so the impact is cosmetic. Extend the colour-literal check to `map-view.css`.

- **ID:** QA-14-r1-4
- **Severity / kind:** Minor / defect (pre-existing, F-05)
- **Route:** frontend
- **Where:** `src/web/src/lib/charts/specs.ts:82`
- **Relates to:** AP-2 ("The sport decides ... the cadence unit"), DS-2
- **Evidence:** The cadence chart spec hardcodes `'rpm'`. The summary and laps use `cadenceUnit(sport)`, which gives `spm` for the pace sports and `/min` for multisport. On a run, the summary says "spm" while the cadence chart and the readout say "rpm". F-14 rewrote `chartSpecs` per sport but left this line alone.
- **Consequence:** The same page shows two units for one metric. One-line fix: `cadenceUnit(sport)`.

- **ID:** QA-14-r1-5
- **Severity / kind:** Minor / doc
- **Route:** architect
- **Where:** `architect/F-14-units-and-theme.md`, D-11 and the last line under Checks
- **Relates to:** CLAUDE.md "Live documents hold the current state only"; OQ-66 (25 Oct)
- **Evidence:** D-11 still says "samples slower than a fixed slowest pace ..., stops included, clipped to it so a stop does not flatten the line". The 25 Oct change is appended as an "OQ-66 update ... supersedes the stops-clipped wording in D-11" line instead of rewriting D-11. D-11 also does not record the stop threshold (`STOP_SPEED` 0.3 m/s) as a decision.
- **Consequence:** A reader of D-11 gets the superseded rule. Rewrite D-11 to the current rule (stops < 0.3 m/s are gaps and show "–"; only moving samples clip) and move the old wording to `architect/history/`.

- **ID:** QA-14-r1-6
- **Severity / kind:** Minor / doc
- **Route:** architect
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md` (ledger and decision 4 table), `architect/INDEX.md` ADR-011 row
- **Relates to:** ADR-011 rev. 8, F-14 design Budget ("the engineer reports it")
- **Evidence:** The measured F-14 share is 2,913 B (entry 50,212). ADR-011 still shows F-14 as planned 2,500 and the reserve as 4,701. Neither the ADR nor the design records the 413 B taken from the reserve.
- **Consequence:** F-17, the riskiest share at 4,000 B, would start its budget from a stale reserve. Close F-14 in the ledger: final 2,913, reserve 4,288.

## Previous findings

None (first round).

## Questions

1. **For the user: slowest pace per sport.** Did the 25 Oct hand check include a hike, a swim and a row, or only runs? 15:00/km is 4 km/h, so much of a steep hike clips to the floor of the chart. 3:00/100 m clips many recreational swimmers. 3:00/500 m clips easy rowing. `slowestPace` has one value per pace unit, so walking and hiking share running's 15:00/km. *Settles it:* "yes, accepted for all five sports" closes this. New values per sport are a small-lane change in `lib/format`.

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 en-US imperial; de-DE, en-GB metric | `lib/units` `defaultUnits`, `page/start.ts` | `units.test.ts` (tag table), `units.dom.test.ts` (en-US opens in mi, ft, mph, °F) | traced, pass |
| 2 switch updates everything at once, no request | `enhanceUnits`, `addUnitsSelect`, `createLook`, ChartStack `{#key units + scheme}` | `units.dom.test.ts` (no fetch, no cookie or storage), `ChartStack.dom.test.ts` (recreation keeps root, height, cursor) | traced, pass; range panel does not exist yet (F-17 inherits it) |
| 3 reload returns to default | memory-only store, default read at start | `hygiene.test.ts` (no storage, cookie or location API) | traced by construction |
| 4 pace or speed per sport | `lib/format` `paceBasis`, `isPaceSport` | `format.test.ts`, `page.test.ts`, `specs.test.ts` (incl. `inline_skating` → mph) | traced, pass |
| 5 pace chart faster up, readout pace, stop gap and "–", slow clip | `specs.ts`, `uplot-options.ts` (`dir: -1`), `MetricChart.svelte` | `specs.test.ts`, `uplot-options.test.ts`; chart-level gap untested (QA-14-r1-1) | traced; user hand check 25 Oct |
| 6 swim distance km or mi | `formatDistance`, `paceBasis` | `page.test.ts` (OQ-67), `units.dom.test.ts` (pool-swim) | traced, pass |
| 7 OS dark mode switches page, charts and map live | `page.css` theme block, `createLook`, `map-view` `setScheme`, `recolour` | `units.dom.test.ts`, `map-view.dom.test.ts`, `map-theme.dom.test.ts` | traced; user hand check 25 Oct (marker colour: QA-14-r1-3) |
| 8 upload page theme, also without JS | `upload.css`, `upload.html` meta | `hygiene.test.ts` | traced, pass |
| 9 no cookie or storage after switching | `lib/units` | `units.dom.test.ts`, `hygiene.test.ts` | traced, pass |

Also checked and correct: the swap is text only and restores the metric markup byte for byte, and a mismatched markup is left alone and logged. The select appears only after a successful decode, in a slot of reserved size. Map style changes are ordered correctly (a stale style response is dropped, the latest wins, our layers are repainted before the merge), with no remount and no refit. The dark style stays on the one tile host, without credentials or referrer. Decimation of pace on display values keeps per-bucket min and max, because the pace conversion is monotonic.

Not run or not verified: real-browser layout shift on the imperial swap (D-7). `.v` has no `white-space: nowrap`, while D-7 says value lines never wrap; that waits for the F-13 Lighthouse run in `en-US` (OQ-64). Phone dark-mode switch: covered only by the user's hand check. `make corpus` was not run (no server change).
