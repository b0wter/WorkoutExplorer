# QA F-46 Laps and splits: lap triggers and split presets - round 1

- **Date:** 2026-11-12
- **Verdict:** ready. No Blocker or Major. Three minors, all test gaps; behaviour checked by probe.
- **Inputs:** `features/F-46-laps-and-splits.md` (AC 1 to 14), spec AP-7, AP-8, AP-19, AP-20, DS-1; `architect/F-46-laps-and-splits.md` (LS-1 to LS-9), ADR-046, ADR-011, ADR-026; decision log 11–12 Nov 2026 (OQ-164 to OQ-166), OQ-167 (cycling list for e-biking), OQ-168 ("Splits" heading). Commit `d6b461a`. Working tree: only `features/README.md` modified, no code.

## Build and test results

- `make ci`: exit 0.
  - `go test -count=1 ./...`: all packages ok (ingest 78.5 s, publish 36.9 s, workout 6.4 s).
  - svelte-check: 0 errors, 0 warnings. Vitest: 68 files, 1,739 tests passed.
  - `npm run size`: `entry index.html: 56351 / 60000 bytes gzip PASS`, the same for `edit.html`. Splits chunk `splits-*.js` 1,602 gzip, `splits-*.css` 226 gzip. These match ADR-011.
- `build/web/index.html` and `edit.html` each load exactly one `<script type="module">` (`main-*.js`). The only preload is the data file. The entry contains no `Half marathon` or `split-select` strings.
- Template probe: a scratch test injected into `internal/publish` with `go test -overlay`; no source file was changed. It rendered `index.html` for ride-no-laps, ride-lap-triggers, pool-swim-33m and triathlon with every lap set to `time`. Results:
  - ride-no-laps: `<section class="laps" hidden>`, `<h2>Splits</h2>`, and no table.
  - ride-lap-triggers: `Trigger` is the last column, with cells Button, Distance, Position, Stop and an empty cell.
  - triathlon: separator `colspan="9"` (8 columns plus Trigger). No separator row has a trigger cell.
  - pool-swim-33m: no trigger column.
- Not run: AC 13 frame rate and LCP. This needs the operator's phone trace.

## Findings

- **ID:** QA-46-r1-1
- **Severity / kind:** Minor / test gap (risk)
- **Route:** backend
- **Where:** `src/server/internal/publish/laps_test.go` (`TestLapsMarkup`), `src/web/src/page/hygiene.test.ts`
- **Relates to:** AC 5 (no laps, JavaScript off: no section), LS-3
- **Evidence:** No test contains `hidden` for the template. A grep for `hidden{{end}}`, `.None}}hidden` and `class="laps" hidden` across the Go and TS tests finds nothing. `TestLapsMarkup` checks only `<h2>Splits</h2>` and the absence of `<table`. The Svelte test checks `hidden` only for `LapsTable.svelte`, not for the Go template.
- **Consequence:** If `{{if .None}}hidden{{end}}` were removed from `src/web/index.html`, a no-JS page would show an empty "Splits" heading and CI would stay green. Behaviour is correct today (see the probe).

- **ID:** QA-46-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/page/page_test.go` samples, `src/web/src/lib/page/samples.ts`
- **Relates to:** LS-2 ("a multisport sample"), AC 3
- **Evidence:** I decoded the golden metadata. Every multisport golden (triathlon, brick-run-bike) has `trigger: ""` on every lap, so no shared sample has a trigger column together with separator rows. The span is `len(Columns)` on both sides, so it is correct by construction. My probe confirms `colspan="9"`.
- **Consequence:** The Go and TS builders are not pinned to agree on a multisport table with a trigger column. A future change to how the span is computed would not be caught.

- **ID:** QA-46-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/page/laps.dom.test.ts` ("AC 9, AC 38")
- **Relates to:** AC 10 (link holds no split distance), AP-19
- **Evidence:** The test asserts `localStorage.length === 0`. It does not check `sessionStorage` or `location.hash` after a distance change. By inspection the chunk writes neither: `src/splits/index.ts` touches no storage, `lib/rangelink` is unchanged, and the selection store is not written on a change.
- **Consequence:** Low. A later regression that writes the choice to the fragment or to session storage would not be caught.

## Previous findings
None (round 1).

## Questions
None blocking. The answers to the focus points:
1. **"33.5 m is not 33⅓" depends only on server rounding.** `lib/splits` `poolOf` treats `pool_length_m === 33` (or `_yd`) as 100/3. `workout/device.go` rounds half away from zero, so 33.5 becomes 34. Both ends are pinned: `device_test.go` "33.5 m rounds half away from zero" → 34, and `splits.test.ts` `pool(34, 33.5)` → 3 lengths, "102 m".
2. **Yard edge (not a defect).** FIT stores the length in centimetres, so a 33.5 yd pool is stored as 30.63 m. That converts back to 33.497 yd, is shown as 33 yd and is treated as 33⅓. This follows the spec ("shows as 33 yd").
3. **Pool boundary tolerance.** The 1 % tolerance was checked on synthetic pool files only. No real pool file was checked in this round.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 manual, distance and position laps, in order | `ingest/listener.go` `lapTrigger`, both `lapsGroup` | corpus `ride-lap-triggers` golden; `ActivityPage.dom.test.ts` LS-2 |
| 2 trigger mapping, unknown is an empty cell | `lapTrigger`; `TRIGGERS` in `build.ts`; `words` in `page.go` | `TestLapTrigger` (all 9 values, invalid, 99); `page.test.ts` LS-2 (incl. `toString`); `TestTriggerColumn` |
| 3 heading, no column without a known trigger, none on splits or separators | both builders | as AC 2; splits pass no trigger (`laps.dom.test.ts`); separators by probe, see r1-2 |
| 4 JavaScript off and edit page | template plus `page.go`; `LapsTable.svelte` | view-model golden `testdata/page/ride-lap-triggers.json` shared by Go and TS; my template probe |
| 5 no laps: splits only; no distance: no section; JavaScript off: no section | template `hidden`, LS-3; `startSplits` reveal | `laps.dom.test.ts` LS-3; `TestNoLapsModule`; `ActivityPage.dom.test.ts`; template `hidden` by probe only (r1-1) |
| 6 list and default per sport, multisport one list, e-biking (OQ-167) | `lib/splits` `LISTS` | `splits.test.ts` AC 34 |
| 7 long presets hidden, default always offered, 5/5/0.4, 0.8 km | `splitChoice`, `splitBounds` | `splits.test.ts` AC 35 |
| 8 pool rule (25, 32.5/33/33.33, 33.5, 30, 50, 25 yd, no shared length) | `poolOf`, `poolPreset`, `poolDistance`, `presetBounds` | `splits.test.ts` LS-6; `laps.dom.test.ts` pool (golden pool-swim-33m, imperial, "Distance (m)") |
| 9 range kept on a change | `build()` leaves the selection alone | `laps.dom.test.ts` AC 37 (spies on `set`/`clear`) |
| 10 default on reload, nothing stored, not in the link | chunk memory only | `laps.dom.test.ts` AC 9/38 (localStorage only, r1-3) |
| 11 drop-down only in the splits view | `show()` | `laps.dom.test.ts` |
| 12 imperial: metric list, 0.62 mi, choice kept | `lapsGroup` units | `laps.dom.test.ts` AC 40 |
| 13 not in the entry; 60 fps; LCP | lazy `import('../splits')` in `start.ts` | size check PASS, one module script; frame rate and LCP not run (manual) |
| 14 every corpus file at v4 | datafile v4, regenerated goldens | `go test ./...` corpus and datafile tests pass |

The Go and TS view models agree through the shared page samples (ADR-026). The static units enhancer rebuilds the laps table with `lapsGroup(meta.sport, meta.laps, units)`; `meta.laps` carries the trigger, so the column count and the trigger text match the server markup. No CSS rule overrides `hidden` on `section.laps`. `check-bundle-size.mjs` now fails when a page loads more than one module script (with a test).
