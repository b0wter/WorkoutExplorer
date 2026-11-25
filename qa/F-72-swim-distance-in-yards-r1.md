# QA F-72 Swim distance in yards - round 1

- **Date:** 2026-11-25
- **Verdict:** ready with conditions. No Blocker or Major finding. The code matches YD-1 to YD-8 and the rule is tested. Five call sites have no test that would fail if they lost the sport (QA-72-r1-1). AC 6 and AC 8 (and parts of AC 1, 5, 7 and 9) are browser behaviour, so they stay open until the hand checks below are done.
- **Inputs:** `features/F-72-swim-distance-in-yards.md`, `architect/F-72-swim-distance-in-yards.md` (YD-1 to YD-8, F72-Q1 to Q4), ADR-011 item 4, ADR-027, F-67 LP-3/LP-10, F-64 LL-3; DS-1, DS-2, DS-6, AP-2/5/6/7/8/20/21, PIN-8. Branch `f-72-swim-yards` at daa25b7, diff against `main` (6ec25c0), 23 files.

## Build and test results

- `make ci` (lint, test, build, size, caddy-check, docker-check, update-check) ran in the foreground: **exit 0**. Vitest: 97 files, 2,533 tests passed. Every Go package `ok`. update_test 57 OK, sign_test 12 OK.
- Size, measured by building both trees with the project's `check-bundle-size.mjs`:
  - On `main`, built from a `git archive` in the scratchpad: entry JS 57,104 gzip, entry 61,382 / 62,000, owner chunk `start.svelte` 19,425.
  - On the branch: entry JS 57,299, entry **61,577 / 62,000** (**+195**, under the 200 ceiling), owner chunk **19,439** (**+14**, under the ceiling of 30). CSS is unchanged.
  - The ADR-011 row matches these figures. The reserve left is 423.
- Metric output: no page sample and no Go file changed. The only edited existing tests are three imperial pool-swim expectations, which changed from `mi` to `yd` on purpose (`page.test.ts:305`, `units.dom.test.ts:350, 361`). No metric expectation was edited.
- Corpus probe. A temporary vitest file, since removed, decoded every synthetic golden data file and every local `build/corpus` data file: 131 files, 130 decoded. The one that failed is `build/corpus/real/garmin_edge_explorer.bin`, a stale build artefact from 15 Oct in an old format, unrelated to F-72. For each decoded file the probe checked, in imperial:
  - the unit of the summary distance;
  - the distance header of `pageLaps`, for each multisport leg as well;
  - the readout's distance unit;
  - the axis label.

  Every check expected `yd` for swims and `mi` otherwise. In metric it also checked that the summary says `km` and that no laps table contains `yd)`. Result: **0 mismatches**. The files covered 8 swims, 7 multisport files with a swim leg, 6 other multisport files, and runs, rides and training sessions.
- Mutants. Each was applied in turn, the full vitest suite was run, and the original was restored. `git status` is clean afterwards.

| Mutant | Result |
| --- | --- |
| A `distanceUnit` gives yd for metric swimming | killed (4) |
| B `distanceMetres` gives yards for metric swimming | killed (4) |
| C `distanceUnit` gives yd for every imperial sport | killed (30) |
| D `distanceMetres` gives yards for every imperial sport | killed (17) |
| E `MetricChart` calls `xScale(ax, units)` without the sport (axis) | **survives** |
| F viewing pins list (`pins/index.ts`) without the sport | **survives** |
| G edit pins list (`edit/pins.ts`) without the sport | **survives** |
| H added-laps table (`edit/addedtable.ts`) without the sport | **survives** |
| I `CursorReadout` subscribe passes the cursor's section sport as the distance sport | **survives** |
| J `RangePanel` effect does not track the view | killed (1) |
| K strip does not set `look.onview` | killed (1) |
| L `CursorReadout` initial fields without `dist` | killed (1) |

## Findings

### Minor

- **ID:** QA-72-r1-1
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:**
  - `src/web/src/components/charts/MetricChart.svelte:85`
  - `src/web/src/pins/index.ts:124-125`
  - `src/web/src/edit/pins.ts:88`
  - `src/web/src/edit/addedtable.ts:45`
  - `src/web/src/components/charts/CursorReadout.svelte:40`
- **Relates to:** AC 1 (distance axis), AC 5 (pins lists), F-65 added-laps table, AC 7 and YD-2 (the distance unit never follows the cursor's section), architecture Verification ("through every builder", "in both readouts")
- **Evidence:** mutants E to I survive the full suite. `yards.test.ts` tests `xScale` and `readoutFields` directly, but nothing checks that the chart, the two pins lists, the added-laps table or the readout's per-section rebuild pass the scope sport. I read the five call sites and they are correct today.
- **Consequence:** a later refactor could drop the sport at any of these sites and the tests would stay green. Mutant I is the worst case. In the whole view of a triathlon, scrubbing over the swim would write yard values under a `mi` label, because the unit text comes from the markup while the value comes from the rebuilt fields. Fix: one DOM test per surface, for example the triathlon whole view scrubbed into the swim (distance still in miles), plus a pool swim's pins line and its added-laps row in imperial.

- **ID:** QA-72-r1-2
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/lib/format/index.ts:62, 74`
- **Relates to:** YD-1 ("one rule")
- **Evidence:** the swimming test is written twice, once in `distanceMetres` and once in `distanceUnit`. Mutants A to D show that the tests catch the two drifting apart.
- **Consequence:** low. `distanceUnit` could be derived from `distanceMetres`, which would make one rule in fact. Fix it if it costs no entry bytes; otherwise leave it.

## Previous findings

None (round 1).

## Questions

1. **Edit page, workout trimmed to the swim only (architect).** The edit page builds its summary from the trimmed sport (`edit/start.svelte.ts:302, 422`). If a triathlon is trimmed down to its swim leg, the server's trimmed sport would be `swimming`. The summary would then read yards while the charts, readout, range panel and pins (`data.meta.sport` = `multisport`) read miles. The published page is consistent, because its data file carries the new sport. I could not check whether the server sets the trimmed sport to `swimming` in this case. This is a question, not a defect. To settle it, either accept the mixed units on the draft page or state which sport the edit page's scope uses.

## Coverage

- **AC 1:** traced. The summary, laps column, range panel and readout are covered by `yards.test.ts`, `RangePanel.yards.dom.test.ts` and `strip.dom.test.ts`. The axis is tested at `xScale` but not through the chart (r1-1). Pace per 100 yd and the pool cell in `m` are tested. Pool splits keep the pool unit through the unchanged `splitsTable` path (`splits/index.ts:94-101`), and splits tests pass.
- **AC 2:** traced. `formatDistance(66 × 22.86)` gives "1,650". The pool cell keeps its unit in both systems (`units.dom.test.ts`). The metric value "1.51 km" comes from the unchanged formatter.
- **AC 3:** traced by reading the code. Non-pool splits go through `lapsGroup(sport)`, and `formatDistance(100)` gives "109". There is no splits-table test with a swim (it is covered by the rule tests).
- **AC 4:** traced. Run and ride stay in miles (tests and probe).
- **AC 5:** traced in code only (r1-1). Needs a hand check.
- **AC 6:** code path traced: the units store and language change go through the existing triggers, `ChartStack` key, `RangePanel` effect and strip remount. Needs a hand check.
- **AC 7:** traced. The tests cover the triathlon summary, leg tables, readout fields, range panel (whole view, swim leg, bike leg) and strip.
- **AC 8:** formatting is tested in en, de, es and fr at 999, 1,000, 1,650 and 10,000 yd. The 360 px layout needs a hand check.
- **AC 9:** no server, template or sample changed, and no metric expectation changed. The entry stays within ADR-011 (measured above). The corpus probe found 0 mismatches. 60 fps scrubbing needs a hand check (nothing new runs per frame: the readout's distance field is fixed per instance).
- **Not run:** browser checks, fuzz, `go test -race` (no Go change).

### Hand checks (browser)

1. Imperial page of a 25 m pool swim:
   - the summary shows "1,640 yd";
   - the laps column header is "Distance (yd)";
   - the distance axis shows whole-yard ticks;
   - the readout and the range panel show yd;
   - the pool cell shows "25 m";
   - the splits show "100 m" with a distance of "100".
2. A 25 yd pool swim reads "1,650 yd" on an imperial page and "1.51 km" on a metric page.
3. Open-water swim with a map:
   - the splits keep the label "100 m" and show "109 yd" in the distance column;
   - the fullscreen readout strip shows yd.
4. Imperial swim with pins: both pins lists (viewing page and edit page) show yd. The edit page's added-laps table shows yd.
5. Switch Metric and Imperial on a swim. Every surface above is rewritten without a reload, and the choice is still there after a reload.
6. Triathlon on an imperial page:
   - the whole view (axis, readout, range panel) is in mi, including while scrubbing over the swim;
   - the swim leg view (axis, readout, range panel, fullscreen strip) is in yd;
   - the bike and run leg views are in mi;
   - switching between legs and back to the whole view rewrites the range panel and the strip.
7. de, fr and es pages show "1.650 yd", "1 650 yd" and "1650 yd". At 360 px portrait, "1,650 yd" fits in the summary and the readout.
8. JavaScript off: the page is metric. Scrubbing the longest swim and the corpus triathlon holds 60 fps.
