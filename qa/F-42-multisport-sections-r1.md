# QA F-42 Multisport sections - round 1

- **Date:** 2026-11-10
- **Verdict:** ready. No Blocker or Major. Four Minors, two of them about manual-only coverage (AC 7 end to end, AC 10 performance)
- **Inputs:** `features/F-42-multisport-sections.md`; `architect/F-42-multisport-sections.md` (MS-1 to MS-9); ADR-011 rev. 14; ADR-026; spec AP-3, AP-4, AP-5, AP-7, AP-8, AP-12, AP-15, SH-4. Commits 759ac6e (backend) and 33efd4a (frontend, HEAD), clean tree

## Build and test results

- `make ci`: exit 0. svelte-check 343 files, 0 errors, 0 warnings; `go vet`; `go test -count=1 ./...` all ok; vitest 67 files, **1,683 passed**; build; docker-check ok.
- Size: `entry index.html: 59487 / 60000 bytes gzip PASS`, same for `edit.html`. Matches ADR-011 (+285 of the 300 ceiling, reserve 513). Lazy: `sections-*.js` 0.83 + 0.39 KB gzip, `sections-*.css` 0.19 KB.

## Findings

### Minor

- **ID:** QA-42-r1-1
- **Severity / kind:** Minor / risk (coverage)
- **Route:** frontend
- **Where:** `src/web/src/lib/route/markers.test.ts` (F-42 MS-8 case)
- **Relates to:** AC 7, SH-4, MS-8
- **Evidence:** The only test of "no leg marker without a position" is a 5-sample synthetic array. No test publishes the triathlon with start/end hiding and runs `routeMarkers` over the public data file. The rule holds by construction: the marker list reads only `data.position`, and hidden samples are NaN in the public file (earlier features).
- **Consequence:** If the hidden-part nulling changed, nothing F-42-specific would catch a leg marker inside a hidden end. Low risk. Hand check below.

- **ID:** QA-42-r1-2
- **Severity / kind:** Minor / defect (cosmetic)
- **Route:** frontend
- **Where:** `src/web/src/page/page.css:607-617`
- **Relates to:** AP-8, MS-6
- **Evidence:** `tbody tr:nth-child(even)` still counts separator rows. On `triathlon` every lap row lands at an even position, so all five are striped. On `triathlon-leg-without-laps` (Swim, #1, T1, #2, Ride, #3, T2, Run, #4), laps 1 to 3 are striped and lap 4 is not.
- **Consequence:** Striping no longer alternates per lap. Visual only.

- **ID:** QA-42-r1-3
- **Severity / kind:** Minor / risk (accessibility)
- **Route:** backend + frontend
- **Where:** `src/web/index.html:57`, `src/web/src/components/page/LapsTable.svelte:25`
- **Relates to:** AP-8, MS-6 (deviation reported by the backend: no `scope`)
- **Evidence:** `<th colspan=N>` sits in `tbody` with no `scope`. Screen readers fall back to heuristics, which may read the leg name as a header for the cells below it, or skip it. One option: `scope="rowgroup"` with one `tbody` per leg.
- **Consequence:** Unclear screen-reader output on the laps table of multisport files only.

- **ID:** QA-42-r1-4
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:547`, `src/web/src/page/page.css:572`
- **Relates to:** MS-9 (deviations: load failure swallowed; pointer cursor before the chunk)
- **Evidence:** `import('../sections').then(..., () => {})` handles a failed load with no `console.warn`, although `enhanceLaps` does warn when rows do not match. `.laps.live tbody tr` gives separator rows a pointer cursor while their range is NaN. If the chunk never loads, that state is permanent.
- **Consequence:** A failed chunk cannot be told apart from a working page in the console. The separator rows look tappable but do nothing. Accepted by MS-9 ("without taps"), but a one-line warn would help.

## Previous findings

None (round 1).

## Questions

None. The reported deviations are acceptable:
- Separator groups carry their own rows, so lap cells appear twice in the view model. The HTML prints only `Sections`.
- `isTop` is a separate prop.
- Label sizes.
- No line where the first leg has no samples. This follows MS-2.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 lines and names, time and distance | `sections/index.ts` `decorator`, `MetricChart.svelte` layout seam | `sections.dom.test.ts` (4 lines, names in the top chart only, a too-wide name dropped with its line kept). `ChartStack.dom.test.ts` (seam called on arrival, width change and axis switch, not on cursor or range flush). Real fit on the distance axis: hand check |
| 2 five separators, JS on/off, edit page, units | `page/sections.go`, `index.html`, `LapsTable.svelte`, `enhance.ts` `:not(.laps-section)` | `publish/laps_test.go` TestSectionRows (published HTML, triathlon 5 rows, colspan, each directly before its lap). `laps.dom.test.ts` (Svelte table, imperial rewrite, no mismatch warning). Samples pin Go = TS |
| 3 separator tap selects the leg | `laps.ts` NaN pairs filled by `legs`, `sectionRanges` | `laps.dom.test.ts` (no selection before the chunk, then [start, next start] / N−1, marked, lap rows still mapped past separators). `page.dom.test.ts` (chunk after onready on the real glue) |
| 4 crossing lap under its start leg | `lapSections` / `sectionOf` | sample `triathlon-lap-crossing` (a rule based on the lap's end would fail it), TestSections and the mirrored TS table |
| 5 leg marker replaces lap marker | `markers.ts` | `markers.test.ts` |
| 6 map markers, fullscreen, none without position | `map-view.ts`, `map-style.ts` `wx-leg-label` | `markers.test.ts`, `map-style.test.ts`. Fullscreen: hand check |
| 7 hidden start/end | the marker reads only public positions | Synthetic only (QA-42-r1-1). Hand check |
| 8 two-ride and single-session files unchanged | MS-1 in Go, TS and `start.ts` | sample `two-rides` (no sections), `page.dom.test.ts` (no chunk, no rows), all other samples `Sections: []` |
| 9 truncated tail | MS-2 last section to N−1 | sample `triathlon-cut` (Swim…T2, lap 4 under T2) |
| 10 unchanged elsewhere, 60 fps, size, LCP | Seam runs on layout only; data file format untouched | Seam test as above. Entry measured. 60 fps on `ride-12h` and LCP **not run**: hand check |

Checked and correct:
- Go and TS models agree: stable sort by `start`, the first section starts at 0, T numbering, and the "last section whose start ≤ lap start" rule. Both tables are identical, and the samples carry the contract.
- `lib/route` still imports only `lib/datafile`.
- The chunk is `import()`ed only when the sessions hold more than one sport, after `onready`.
- No new public data: names come from sessions already in the file.
- Names are HTML-escaped by `html/template` and Svelte.
- Empty sections (b ≤ a) are not selectable.
- `legs` does nothing when the row count does not match.
- The `.on` mark overrides the separator's `background: none` by specificity.

Not run: a real browser (layout, fit of the names, MapLibre labels), the performance trace, Lighthouse.
