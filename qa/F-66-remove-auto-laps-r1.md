# QA F-66 Remove auto-laps - round 1

- **Date:** 2026-11-23
- **Verdict:** not ready. One Major: on a workout without advancing distance and without pool lengths, removing every auto-lap removes the whole laps module, and with it the "Restore auto-laps" line. After that the owner cannot undo the setting on the edit page (QA-66-r1-1, breaks OQ-306 and AC 3). Everything else holds: the Go rule, the order, the API, storage, the in-place rebuild, privacy and the budgets.
- **Inputs:** `features/F-66-remove-auto-laps.md` (AC, OQ-306 to OQ-314), `architect/F-66-remove-auto-laps.md` (RA-1 to RA-10, "What QA must check"), ADR-060 addendum, spec AP-8 and INF-1, `architect/F-12`, `F-15` P-7 and `F-29` PT-2 diffs. Reviewed commit f19c993 (backend b853970 + frontend) against main 92958c2, in a separate checkout. The base was built for the size comparison.

## Build and test results

- `make ci` (f19c993, foreground): **exit 0**, 4 min 7 s. gofmt and vet clean. Every Go package `ok`. vitest **2,366 passed**. Docker and Caddy checks OK.
- Size (`npm run size`):
  - Activity entry **60,461 / 62,000** gzip PASS. The base 92958c2 gives 60,463. `main-*.js` raw is identical (144,503) and has no `laps-auto` string. The only counted growth is the owner stylesheet `start-*.css` (+32 gzip, which the script counts for index.html), so viewers load no new JS.
  - Owner chunk `start.svelte-*.js` **18,864 gzip** (base 18,095, so +769, against the 600 estimate). That leaves 1,136 under the 20,000 ceiling.
- Probes (throwaway, removed afterwards):
  - Go: `RemoveAutoLaps`, then Encode and Decode, over every synthetic corpus file. All encode and decode. Files that change: single sport goes to 0 laps (OQ-307), `chained-ride-ride` 4 → 2, `chained-ride-run` 3 → 2 (leg boundaries kept), `ride-lap-triggers` 5 → 3, pool files → 0 (lengths stay).
  - Local Apple Watch file: 13 device laps → 0 after removal, so splits only. The parser reports their trigger as `distance`, not missing; the rule covers both.
  - vitest probe for QA-66-r1-1 (below).

## Findings

### Major

- **ID:** QA-66-r1-1
- **Severity / kind:** Major / defect
- **Route:** frontend (architect if the line needs a new home)
- **Where:** `src/web/src/components/page/LapsTable.svelte:57`: the line is inside `{#if laps}`. `src/web/src/lib/page/view.ts:86`: `pageLaps` returns `null` with no laps, no advancing distance and no lengths. `src/web/src/edit/laps.ts` `auto()`.
- **Relates to:** OQ-306 (reversible), AC 3, RA-9, AP-8 ("without distance there is no laps and splits section")
- **Evidence:** vitest probe on golden `indoor-no-distance-no-gps` with two device laps (`time`, `stop`):
  - `removableBoundaries` = 1, and `pageLaps` with the laps is not null, so "Remove auto-laps (1)" shows.
  - `removeAutoLaps` gives `[]`, and `pageLaps` with `[]` is `null`. So `LapsTable` renders nothing, including the "Restore auto-laps" button.
  - The same happens when the edit page opens with the setting stored on.
  - The corpus has no such file, so no test catches it.
- **Consequence:** A sharer with a no-distance workout (treadmill without a foot pod, strength or indoor sessions with time auto-laps) who taps the line loses the laps table. They also lose the only way back: after Save, the setting is stuck on unless someone calls the API by hand. Two possible fixes:
  - Render the line outside `{#if laps}` on the edit page.
  - Offer the action only when the result can still show a module.

  Either is fine, but say which one.

### Minor

- **ID:** QA-66-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** backend + frontend
- **Where:** `src/server/internal/datafile/autolaps_test.go` (`autoLapsSpecs`), `src/server/testdata/datafile/auto-laps.json`, `src/server/cmd/workout-explorer/autolaps_test.go`
- **Relates to:** RA-3, RA-5, "Go and browser agree", privacy row of "What QA must check"
- **Evidence:** In the Go/TS sample, `triathlon` merges nothing (one lap per leg), and `pool-swim` and `pool-swim-mixed` merge to `[]`. So the sample never compares a merged lap with a stroke, nor a merge inside a multisport leg, nor pool or multisport with added boundaries. Only `ride-lap-triggers` checks merged values. `chained-ride-ride` / `chained-ride-run` (merges inside legs) are not used. The end-to-end privacy check runs only at radius 300 without a trim; the design asks for 0, 300 and 2,000, with and without a trim. Each side does unit-test the stroke and leg rules.
- **Consequence:** A Go/TS drift in the stroke or leg rule would pass CI. The privacy risk is low, because removal changes only `laps` and stream equality holds by construction.

- **ID:** QA-66-r1-3
- **Severity / kind:** Minor / docs
- **Route:** architect (ADR-011), requirements (TD)
- **Where:** `architect/ADR-011-*` has no F-66 row (F-65 added one). `spec/Workout Explorer — Technical Design.md` has no `remove_auto_laps` column and no "Auto-lap" glossary entry.
- **Relates to:** F-66 Cost and Hand-offs
- **Evidence:** grep finds 0 hits in the TD and no F-66 in ADR-011.
- **Consequence:** The next owner feature has no recorded owner-chunk baseline (18,864). The data model in the TD is out of date.

- **ID:** QA-66-r1-4
- **Severity / kind:** Minor / defect (pre-existing since F-65, repeated here)
- **Route:** backend
- **Where:** `src/server/internal/datafile/autolaps_test.go:16,76` (and `addedlaps_test.go:18,107`), `Makefile` `corpus-update`
- **Evidence:** The comment and the failure message both say "run make corpus-update", but that target runs only `TestCorpus` and `TestDataFileCorpus` with `-update`. It never rewrites `auto-laps.json` or `added-laps.json`.
- **Consequence:** Following the error message does not fix the failure.

## Previous findings

None (round 1).

## Questions

1. **Migration v12 reversible?** Note only. `ALTER TABLE … ADD COLUMN`, forward-only per ADR-019. A v11 binary refuses a v12 database (`OpenExisting`, tested). A downgrade means restoring a backup, or losing workouts, which is accepted. Nothing to settle.
2. For QA-66-r1-1: should the action be offered at all on a workout whose laps module would disappear? That is, should the line live outside the module on the edit page? Settled by a one-line decision in RA-9.

## Coverage

- **Checked and correct:**
  - Removal rule (Go `workout/autolaps.go`): auto triggers `distance/time/position/""`. A run plus the next lap merge: first start, latest end, last trigger, RA-5 stroke, `completeTotals` (so the transition rule applies). Unmerged laps are untouched. OQ-307 gives empty laps. Nothing removable returns the input unchanged. A truncated tail stays. Gaps fall inside the merged lap.
  - The TS mirror (`lib/addedlaps`) matches line by line, and the parity sample passes.
  - Order removal → AddLaps → trim → hiding in `publish.front`. `CheckEdits` judges against the laps after removal on the resulting record (e2e: Add lap on a removed boundary gives 200 while on and 400 when switched off in the same PATCH).
  - Hiding: streams are byte-equal on and off, and there is no position in the hidden ends (e2e at radius 300). A cut lap has its totals cleared and recomputed by `Trim`.
  - In-place rebuild: `HidesMore` allow-list, tested both directions.
  - 400 on `null`, `1`, `"true"` (httpapi and e2e). Wrong token gives 404.
  - Column: default 0, CHECK refuses 2, round trip, `UpdateDerived` (regenerate) never writes it. Deleted with the row.
  - Restore drop rule: page `held` and tolerance, and the Save sends `added_lap_boundaries: []` (dom test). The server skips the stored entry in the build.
  - Splits fallback: AL-8 rule, and the splits chunk restarts after each rebuild.
  - Leg boundaries kept (Go unit, corpus probe). Stroke rule (unit tests on both sides).
  - The count includes only boundaries strictly inside the live trim (unit and dom test).
  - About sentence and edit texts in EN/DE/ES/FR. The setting is only in the owner chunk.
- **AC traced:**
  - AC 1 (Go, dom, real file).
  - AC 2 (unit).
  - AC 3: works except QA-66-r1-1.
  - AC 4 (`removableBoundaries` = 0 gives no line).
  - AC 5 (e2e + dom).
  - AC 6: Go only, see r1-2.
  - AC 7 (radius 300 only).
  - AC 8 (unit).
  - AC 9.
  - AC 10 (by row deletion; no new test).
  - AC 11: texts only.
  - AC 12: entry size and corpus Go probe.
- **Not run or hand checks left (conditions):**
  - Both themes, phone and desktop, touch and mouse.
  - Screen reader focus after the toggle.
  - LCP.
  - ride-12h 60 fps with the setting on.
  - Server-rendered page HTML with the setting on, JavaScript off. It comes from the decoded metadata, so it follows by construction, but it was not asserted.
