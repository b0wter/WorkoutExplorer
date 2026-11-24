# QA F-68 Extra sensor streams - round 1

- **Date:** 2026-11-24
- **Verdict:** ready with conditions. No Blocker or Major. The privacy boundary holds: in every probe, no developer-field name, unit, app ID or value reached any public output, draft or log. The conditions are the hand checks below: 60 fps and LCP with ten charts (no 12-hour file with the new charts exists yet, QA-68-r1-4), 360 px, the two themes and four languages, and F68-Q1 on a real Stryd file before release.
- **Inputs:** `features/F-68-extra-sensor-streams.md` (AC 1 to 8), `architect/F-68-extra-sensor-streams.md` (XS-1 to XS-11, the QA list), ADR-005, ADR-011 (ledger), ADR-017 (the F-68 note), ADR-023, the TD rows for "FIT parsing" and "Workout data file", and `architect/INDEX.md` F68-Q1 to Q3. Code at main `8141d26` (backend merge `e2a2a0f`, frontend merge `759ff77`). The working tree was clean.

## Build and test results

- `make ci` on main: **exit 2**. `go vet` and svelte-check are clean, and every Go package passes except `internal/ingest` `TestCorpus/real/development-watch7-5-{run,triathlon}`: the local, git-ignored golden reports for the real files are stale (QA-68-r1-3). The other targets ran with `make -k test-web build size caddy-check docker-check update-check`, **exit 0**:
  - vitest: 95 files, 2,495 tests passed. Docker image and `docker-check` ok. `update_test.py`: 57 OK. `sign_test.py`: 12 OK.
  - Size: viewing entry **61,367 / 62,000** gzip. I rebuilt `15cd39e` in the scratchpad: 61,137. The delta is **+230 B**, ceiling 250, and the ledger says +228. Lazy `dynamics` chunk: 1,002 JS + 211 CSS gzip. Owner `start.svelte`: 20,046.
- `make corpus`: 74 ok, 2 DIFF (the same two real files). A structural diff without values shows that only `/streams/{vertical_oscillation,ground_contact_time,stride_length,vertical_ratio}` were added. Both real files are far under the 500 KB `.br` budget.
- `go test -race -count=1`: the F-68 tests in `ingest`, `workout` and `publish` (plus `Regenerate`), and all of `internal/page`: ok.
- Fuzzing, on a scratch copy without `real/` (the seeds include the 8 new developer-field files): `FuzzParse` 150 s (291k execs) and `FuzzAccept` 120 s (260k execs): PASS.
- End-to-end probe with the built binary (scratch data dir, own port). Uploaded `developer-hostile.fit` three ways (no hiding, 300/300 m, 2000/2000 m = fully hidden), plus `stryd-power`, `stryd-and-native-power`, `stryd-chained`, `run-dynamics` and `run-dynamics` with 2000/2000 m hidden. Each was published, then I downloaded `draft.bin` and `original.fit`. I swept every file under `public/` (raw and brotli-decoded), every `draft.bin` and the server log for 17 name, unit and app-ID markers, the injected semicircle and degree positions, and the injected uint16 series: **0 hits**. As a control, the sweep finds 32 hits in the stored original. The original downloads equal the uploads byte for byte. Published stream lists: hostile = base streams only (no power). Stryd = power present. Both = native values. Chained = power. Dynamics = the four columns.
- Regenerate probe: I built the pre-F-68 binary (`39dee4d`) in the scratchpad and published three workouts as version 7. Then I ran `regenerate` with the current binary: rebuilt=3, every data file is version 8, `run-dynamics` gained the four columns (`charts-9 cells-11`), `stryd-power` gained power, the hostile file is still clean (sweep 0 hits), and the file names equal those of a direct v8 build.
- Out-of-range and two-app probes: I wrote three FIT files with a scratch generator (muktihari/fit) and ran them through `parse` and `parse -datafile`. Results under QA-68-r1-1 and r1-2. A corrupted developer base type, and a file cut inside the first developer record, were refused as `no_data`. Neither the rejection report nor stderr contained a marker.

## Findings

### Minor

- **ID:** QA-68-r1-1
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/ingest/devfields.go:142` (`validWatts`), `:217` (`fillStryd`)
- **Relates to:** XS-3 ("treated as native power (F-03 W-4 clamp)"), XS-6, AP-2
- **Evidence:** Native power is uint16 (0 to 65,534). Stryd values can be sint16, sint32 or float, and they enter `RawRecord.Power` without being clamped to that range. The data file clamps its column, but the summary and lap totals use the raw values. Probe results: a float64 Stryd field with a single value of 1e12 gives `summary.max_power 1000000000000` and `avg_power 1666666916.25`, also in `laps[].totals` of the public data file. A sint16 field alternating −30000 and 250 gives `avg_power -14875`.
- **Consequence:** For a crafted file, the summary strip and the laps table show nonsense (negative or 10^12 W) that disagrees with the clamped chart. There is no privacy impact, and no real device does this. The fix is to clamp to native power's range in `validWatts`, or to drop values outside it.

- **ID:** QA-68-r1-2
- **Severity / kind:** Minor / deviation
- **Route:** backend
- **Where:** `src/server/internal/ingest/devfields.go:119-133` (`power`: `if s.bad[k] { continue }`)
- **Relates to:** XS-4 ("a second qualifying pair (two apps) is ignored")
- **Evidence:** Probe with two qualifying `Power`/`Watts` uint16 pairs: app 0 writes 200 W, app 1 writes 900 W, and from 300 s on app 0 writes an array-shaped value (a new local definition). Result: power is 200 for 0 to 299 s, then 900. The source switched to the second app in the middle of the file, and the mean of 549 W mixes both apps.
- **Consequence:** One power chart holds two apps' values. This is exotic (a definition that changes size), but it contradicts XS-4's "first pair is the source". Either the source stays locked once it is chosen (a bad pair yields nothing and the second is never used), or the architecture doc records the fallback.

- **ID:** QA-68-r1-3
- **Severity / kind:** Minor / process
- **Route:** backend (operator, on the machine that holds `real/`)
- **Where:** `src/server/testdata/corpus/expected/real/development-watch7-5-{run,triathlon}.json` (git-ignored)
- **Relates to:** AC 7 ("every corpus file renders"), ADR-004
- **Evidence:** `make ci` fails in `TestCorpus/real/...`. Engineer worktrees have no `real/` files, so their runs passed. Structural diff: only the four new stream entries were added (values not quoted).
- **Consequence:** `make ci` is red on the operator's checkout until someone runs `make corpus-update` and reviews the result. The diff above is already the review: the new reports contain only the four new streams.

- **ID:** QA-68-r1-4
- **Severity / kind:** Minor / gap
- **Route:** frontend (corpus file: backend)
- **Where:** `src/web/bench/MEASUREMENTS.md` (no F-68 rows); `src/server/tools/corpusgen/f68.go` (no 12-hour run)
- **Relates to:** AC 7 ("LCP unchanged within noise; 60 fps scrubbing on the 12-hour file"), F-68 Cost ("ADR-016 gate on a 12-hour synthetic run with every stream (ten charts)")
- **Evidence:** No synthetic 12-hour file carries the dynamics. `ride-12h` has six charts at most. No scrub or LCP measurement for F-68 was recorded.
- **Consequence:** The design's own gate has not run, and the operator cannot run the hand check without such a file. The risk is low (earlier records show 1 to 2 ms flush p95 with six charts), but the AC has no evidence. Fix: add a 12-hour run with every stream to corpusgen, then record one scrub run under ADR-016.

- **ID:** QA-68-r1-5
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/dynamics/range.ts:44`; `src/web/src/components/range/RangePanel.svelte` `<style>`
- **Relates to:** AC 6 (360 px), AP-7
- **Evidence:** The panel's `min-width: 0; overflow: hidden; text-overflow: ellipsis` rules are Svelte-scoped (built CSS: `.cell.svelte-a4yg7d,.label.svelte-a4yg7d,.val.svelte-a4yg7d`). The cells that `rangeCells` creates with `createElement` lack the scope class, so they get only the global `white-space: nowrap`.
- **Consequence:** Where a dynamics cell's text is wider than its 64 px+ track, it overflows into the next cell instead of being cut with an ellipsis. Not seen in a browser; check it by hand at 360 px with DE/ES/FR.

- **ID:** QA-68-r1-6
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/server/internal/page/page.go:522` (`chartMetrics`); `src/server/testdata/page/` and `src/web/src/lib/page/samples.ts` (no sample with dynamics)
- **Relates to:** XS-9 (no layout shift), F-13 D-3 (Go/TS view-model contract)
- **Evidence:** No committed page sample has the four streams, so neither builder's `charts-7` to `-10` or `cells-9` to `-12` path is compared. In my end-to-end probe the server wrote `charts-9 cells-11 has-distance` for `run-dynamics`, which matches the client's count. That is correct today, but no test pins it.
- **Consequence:** If someone later edits one chart list (Go `chartMetrics` or TS `view.ts`/`presentCharts`), nothing catches the drift. The result would be a layout shift on running pages. Fix: add a `run-dynamics` sample.

- **ID:** QA-68-r1-7
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `architect/ADR-017-parse-isolation-and-limits.md` (F-68 note); `src/server/testdata/corpus/MEASUREMENTS.md` "Extra sensor streams"
- **Relates to:** XS-11, the ADR-017 400 MB rule
- **Evidence:** The note compares 337 MB at **440,000** records with F-68 fields against 289 to 322 MB at **500,000** records without them, and concludes that F-68 adds "15 to 45 MB". At equal record counts the increase is larger: scaled linearly, 500,000 F-68 records come to about 380 MB. Such a file fits in `MaxBytes` with compressed-timestamp headers (about 41 B/record, roughly 19.6 MiB). This was not measured.
- **Consequence:** The 400 MB rule probably still holds, with about 5 % margin rather than the stated 15 %. The note understates the cost. Fix: measure a 500,000-record twin (compressed timestamps) or correct the note.

- **ID:** QA-68-r1-8
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/dynamics/range.ts:50`
- **Relates to:** F-05 D-10 (the selection listener writes text only)
- **Evidence:** Every selection change rebuilds `dynamicsSpecs(current)`: 4 objects, about 12 closures and 8 `t()` lookups. It also runs `rows.find` for each spec. This happens on every frame of a range drag.
- **Consequence:** Small allocations on every frame of the drag hot path. Fix: cache the specs per units and language change.

- **ID:** QA-68-r1-9
- **Severity / kind:** Minor / documentation
- **Route:** architect
- **Where:** `architect/F-68-extra-sensor-streams.md` XS-9
- **Relates to:** XS-9; CLAUDE.md "one home per fact"
- **Evidence:** XS-9 still says the chunk starts "from the streamed metadata prefix", has its "own text group" and costs "owner chunk 0 bytes". As built (`src/web/src/page/start.ts:240`), the chunk loads after the full decode, the texts are in the `page` group, and the owner chunk grows by 2 B. Only the ADR-011 ledger row records this.
- **Consequence:** The live design doc contradicts the code. Each deviation is defensible (the reserved layout prevents a shift, and one round trip of about 1 KB delays the charts only), but the doc should say so in one line.

## Previous findings

None. This is the first round.

## Questions

1. **F68-Q2 and F68-Q3 were decided by the main session, not the user** (`architect/INDEX.md:183-184`). Q2 puts Stryd power into the summary strip, while the feature's Scope "Where values appear" says "not in the summary strip". Q3 means a brick shows no run power. Both are product scope. This is settled if the user confirms both (or the decision log in `features/README.md` records it).
2. **F68-Q1 (release blocker):** are the Stryd strings `Power` and `Watts`? This is settled when the operator runs `workout-explorer parse` on one real Stryd file and reports whether power appears (none of the 6 local real files carries Stryd power).

## Coverage

| AC | Code | Test / evidence | Status |
| --- | --- | --- | --- |
| 1 Charts per present stream, below temperature, readout (+fullscreen) | `ingest/listener.go` `dynamic`, `datafile` v8 rows, `lib/charts/specs.ts` `presentCharts`/`readoutFields`, `src/dynamics/specs.ts` | `TestRunDynamicsStreams`, `dynamics.test.ts`, `page.dom.test.ts` (chunk loaded before the charts; failure warns), e2e `charts-9` | traced. Fullscreen readout: hand check |
| 2 Stryd power → chart, curve, zones; native wins | `devfields.go`, `fillStryd` | `TestStrydSources`, `TestStrydPublishes`, `TestStrydNativeWins`, e2e | traced (QA-68-r1-1, r1-2) |
| 3 Non-allowlisted fields publish nothing | `devfields.go` (XS-3/XS-7) | `TestDeveloperHostilePublishesNothing` (data.bin raw and decoded, .br, HTML, PNG, draft, log; hidden and not), `TestQualifies`, e2e sweep incl. full hiding and regenerate | traced; independently reproduced |
| 4 Range panel averages; laps and summary unchanged | `src/dynamics/range.ts`, `RangePanel.svelte` | `range.dom.test.ts` (mean, imperial, destroy) | traced (QA-68-r1-5, r1-8) |
| 5 Trim and transitions remove values | `workout/transition.go` `ruleDrop` | `TestDynamicsDroppedAtTransitions`, `TestDynamicsTrim`, the corpus invariants (real triathlon included) | traced |
| 6 Units, themes, 360 px, landscape, languages | `specs.ts` (in, ft), `dynamics.css` tokens, `i18n/{de,es,fr}.json` (8 keys each) | `dynamics.test.ts` imperial; translations read | units traced; themes, layouts and languages: hand check |
| 7 Budgets, LCP, 60 fps, corpus, fuzz | ledger, `TestSizeBudget4hRun` (148,884 `.br`) | size check run; fuzz run; corpus run | budgets and fuzz traced; LCP and 60 fps **not measured** (QA-68-r1-4); real corpus red locally (r1-3) |
| 8 UP-9 unchanged, About unchanged | strip untouched | `TestStoredOriginalKeepsDeveloperFields`, e2e original download byte-equal | traced |

Not run: browser checks (scrub, LCP, 360 px, themes, languages, fullscreen readout, edit page) and a real Stryd file (F68-Q1).

**Hand checks for the user:** (a) open a running workout with the dynamics on a phone at 360 px portrait and in landscape: four charts below temperature, no jump when the charts appear, readout and fullscreen readout labels fit, range panel cells fit in DE, ES and FR. (b) Toggle the theme: the chart colours are readable in light and dark. (c) Switch to imperial: in and ft. (d) Once QA-68-r1-4 provides a file: 60 fps scrub on the phone. (e) F68-Q1 on a real Stryd file. (f) After reviewing, run `make corpus-update` locally for the real files.
