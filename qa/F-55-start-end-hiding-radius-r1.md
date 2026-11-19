# QA F-55 Selectable radius for start and end hiding - round 1

- **Date:** 2026-11-19
- **Verdict:** ready with conditions. No Blocker or Major findings. No radius or hiding fact reaches a public file, the page text or the viewing-page dialog: I checked 17 sources × 2 trims × 11 radius pairs through the real Go build and jsdom. Raising a radius on either end is remove-first; lowering is in place. A 300/300 build is byte-identical to the pre-F-55 hiding-on build. Of 31 mutants, 28 are killed, 2 are equivalent and 1 survives as a test gap (r1-1). Condition: the hand checks below. No real browser ran, and the map shading was only exercised through stubs.
- **Inputs:** `features/F-55-start-end-hiding-radius.md` (AC; OQ-246, 248, 249, 268); `architect/F-55-start-end-hiding-radius.md` v0.1 (HR-1 to HR-10); ADR-056; F-16 H-1 to H-9; F-47 TR-6, TR-8, TR-9; F-58 SE-4; spec SH-4, SH-9, SH-10, SH-12, UP-5, UP-6. Code: backend 83f7aff, frontend ffa8048 (diff `474d9b5..HEAD`), working tree clean.

## Build and test results

- `make ci` at ffa8048: **exit 0**. gofmt, `go vet` and svelte-check clean; `go test ./...` all ok; vitest 75 files, **1952 passed**. Entry size: `index.html` 57,464 / 60,000 B gzip PASS. Built at 474d9b5 the entry was 57,469, so F-55 changes it by **−5 B** (target 0, ceiling 50). The caddy-check, docker-check and update-check targets passed.
- `go test -race -count=1` on `internal/workout`, `lifecycle`, `store`, `httpapi` and `cmd/workout-explorer`: all ok. `internal/publish` was not run under `-race` (about 7 min last time; its only change is one call and one switch case).
- **Repeat of F-58 AC 39 with radii (scratch copy, repo untouched).** I published through the real `Area` build. Sources: 14 corpus files (ride-30min, run-30min, triathlon, gps-dropout, pause-hours, timer-pause, no-distance-with-gps, chained-ride-ride, brick-run-bike, indoor-no-gps, pool-swim, ride-4h, smart-recording, rolling-hills) plus three generated tracks: a 500 m walk, a 2 km walk, and a track with jumps at both ends. Each was published untrimmed and trimmed at both ends, at 11 radius pairs: 0/0, 300/300, 0/500, 2000/0, 50/0, 0/50, 500/500, 2000/2000, 1000/0, 0/1000 and 150/1950.
  - Go side: the metadata (minus stream descriptors) is identical across all pairs. Pairs that give the same cut produce **byte-identical public folders** (`.bin`, `.br`, `index.html`, PNG); for example, the jumps track has 11 pairs but only 6 distinct cuts. Between any two pairs that both have positions, or both are fully hidden, `index.html` is byte-identical apart from the hashed file names. No page contains "N m", "radius", "hide_" or "hiding".
  - jsdom: each published `index.html` was loaded with its `data.bin` through `startPage`, and ⓘ was clicked. The button markup and the dialog `outerHTML` are **identical across all 11 pairs** for every source and trim, in metric and imperial (748 comparisons), including the fully hidden walks. As a control, the trimmed and untrimmed dialogs differ, and the fully hidden imperial dialog contains no radius text.
- **AC 41 against the pre-F-55 code.** The same 17 sources, untrimmed and trimmed, were published with 474d9b5 (`HideEnds` false or true) and with HEAD (0/0 or 300/300). `diff -r` of the 272 public files: **identical**.
- **AC 38 through the area (scratch test):** raising a radius from 0 (0/0→0/50, 0/0→2000/0), raising one end while lowering the other (300/300→350/0, 300/300→0/2000) and lowering one end (300/300→300/250, 300/300→0/0). The first four go remove-first (steps start "remove index.html", "remove folder"), the last two go in place. No old data file is left behind.
- **Mutants** (scratch copy, one at a time, against the relevant packages or the web suites):
  - Killed: Window start/end swapped; Window index off by one; pass radius off by one; a real 0 m circle (k from 0, no entry-0 override); entry 0 set to the first or last positioned sample; publicTransform radii swapped; HidesMore with the end check off, with a lowered end counting as a raise, or with any change counting as hiding more; migration mapping hiding on to 0/0 or 300/0; `-1` shifted by `a`; trimmed steps not shifted (`Shift(first*0)`); suggestion skipping nothing when fully hidden or when the steps are null; ApplyPatch without the range check; UPDATE with start/end columns swapped; PATCH accepting null; scan accepting another step count.
  - Frontend, killed: shading from the saved radii; step index +1; start/end swapped in the shading; the F-58 dialog from the slider values; fully hidden only on `-1`; end section starting at `to`; ft/mi switch at 0.2 mi; basemap job not run after a radius save; hiding marks dropped when trim marks exist.
  - Equivalent: `k := 0` with the entry-0 override kept; HidesMore `>=` (equal values never reach the switch).
  - Survived: r1-1.

## Findings

### Minor

- **ID:** QA-55-r1-1
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner.go:491` (`hiddenSkip`), `src/server/internal/lifecycle/suggest_test.go`
- **Relates to:** HR-7, UP-5, AC 39 (suggested title's town)
- **Evidence:** I changed `{0, from - 1}` to `{0, from - 2}`, so the last hidden start sample is no longer skipped. `go test ./internal/lifecycle ./cmd/workout-explorer` still passes. The test's towns are kilometres apart, so a one-sample shift never changes the town.
- **Consequence:** Nothing today. If the boundary regresses, the owner's suggestion could name a town from a hidden fix. The impact is limited: the suggestion is shown to the owner only, and the public page computes its town from the public file. A track whose last hidden fix and first public fix lie on either side of a town boundary would pin it.

- **ID:** QA-55-r1-2
- **Severity / kind:** Minor / documentation (deviations not recorded)
- **Route:** architect
- **Where:** `architect/F-55-start-end-hiding-radius.md` HR-7; ADR-056 item 2
- **Relates to:** HR-7, ADR-056
- **Evidence:** HR-7 says that with a radius above 0 and `hiding_steps` NULL the suggestion is `""`. The code returns the town-less suggestion ("Morning workout"), as for an indoor file (`suggest_test.go`, "no steps"). Separately, `store.scanRecord` reads a `hiding_steps` value of another length as NULL. Neither is written down.
- **Consequence:** A later reader may "fix" the code back to `""`, or wonder why a corrupt or old column silently shows no shading. Both deviations are acceptable (judged below); each needs one line in HR-7 and ADR-056 item 2.

- **ID:** QA-55-r1-3
- **Severity / kind:** Minor / risk (display; carried over from F-47 TR-9, wider with F-55)
- **Route:** architect
- **Where:** `src/web/src/edit/start.svelte.ts:346-350` (`hidden()`), steps from the **saved** trim (ADR-056 item 3)
- **Relates to:** HR-8, OQ-249, F-47 TR-9
- **Evidence (reading, not run in a browser):** take a saved trim starting at sample `a` and a start radius of 300 m, then drag the trim's start handle back towards 0 without saving. The hiding shade stays `[0, steps.start[6] − 1]`, which is measured from `a`. After Save, the server measures 300 m from the new first position. Everything between that point and the old boundary becomes public, but the page shaded it as hidden until the save. Dragging inward errs the other way (it shows less hidden than will be), which is safe.
- **Consequence:** For a few seconds the owner can believe that a stretch near the old trim point stays hidden when it will be published. TR-9 accepted "follows the saved trim" when hiding had one fixed radius. With up to 2 km per end the stretch can be long. The public build stays correct; only the preview before saving is misleading.

## Deviations judged

| Deviation | Judgement |
| --- | --- |
| Steps of another length read as not computed | Accept. It fails toward no shading and no town, and `regenerate` rewrites the column. Record it (r1-2) |
| Town-less suggestion instead of `""` before `regenerate` | Accept. It meets HR-7's aim ("fails toward no town"), matches the indoor case, and is tested. Record it (r1-2) |
| JSON encoding of `hiding_steps` | Accept. ADR-056 leaves the encoding to the engineer; the column is checked with `json_valid`, and the length is checked on read (mutant killed) |
| `formatRadius` in `lib/format/dates.ts` | Accept. It keeps the formatter out of the entry (`dates` is a lazy chunk; built `index.html` loads one module script). The file name now undersells its contents (preference) |
| "2000 m" without thousands separator | Accept. No formatter in `lib/format` groups digits; this is consistent |
| Imperial F-58 dialog "0.19 mi" for 300 m | Accept. It follows OQ-268 (0.1 mi = 160.9 m, so miles from 200 m), and the summary-info test pins it |
| Hiding shading follows the saved trim while an unsaved trim moves | This is F-47 TR-9 as written, not a new deviation. The residual risk is r1-3 |
| Overlapping trim and hiding shading drawn darker | Accept (preference). It reveals nothing beyond the two ranges, which are shown anyway |

## Previous findings

None (round 1).

## Questions

1. **r1-3, architect:** with an unsaved trim that differs from the saved one, should the edit page (a) keep today's behaviour and record the effect in HR-8, (b) hide the hiding shade until the trim is saved, or (c) show a one-line note ("Hidden sections update after saving the trim")? Suggestion: (c), or (a) if the user accepts it. Showing me that owners never move a trim and a radius in the same edit would settle it as (a).

## Coverage

| AC | Code | Test / check |
| --- | --- | --- |
| 1 Two sliders, 0–2,000/50, new upload 0, no browser storage | `EditPanel.svelte:174-181` (inputs at 176, 179), `NewDraft` | `edit.dom.test.ts` "sliders: 0 to 2,000 m…" (localStorage keys), `hiding_steps_test.go` (0/0 after upload) |
| 2 Live label, imperial, shading before Save, public only on Save | `EditPanel.svelte` `hideText`, `formatRadius`, `start.svelte.ts` `onhide`/`hidden()` | `format.test.ts`, `edit.dom.test.ts` (frame throttle, imperial, steps lookup), mutants F1–F3, F7. **Hand check 1, 2** |
| 3 PATCH refusals store nothing | `httpapi/owner.go:364`, `lifecycle/owner.go:111` | `cmd/owner_test.go:207` (row unchanged), `httpapi/owner_test.go`, `lifecycle/owner_test.go`; mutants G14, G16 |
| 4 0/50/300/2000 every combination, no hidden position public | `workout.Hide`, `publicTransform` | `TestHidingRadii`, `TestHideRadiiProperty` (300 seeds × 41 steps vs oracle), `TestHidingStepsCorpus`; scratch 17×2×11 |
| 5 With trim, measured from remaining ends | `front` → `trimmed` → `encode` | `TestTrimKeepsTheTrimmedPartOutOfEveryPublicFile` (4 pairs), scratch trimmed dumps |
| 6 Summary identical at every radius | transform touches lat/lon only | `checkHiddenAt` metadata compare; scratch: metadata equal across 11 pairs × 34 runs |
| 7 Fully hidden notice, no warning; indoor notice | `Hide` fully flag, `ActivityPage.svelte:32` | `TestHidingRadii`, `TestHideRadiiFully`; scratch walks; no edit-page warning exists (grep) |
| 8 Raise either (from 0 too) remove-first; lower in place | `HidesMore` `publish.go:322-329` | `TestHidesMore` (9 new cases), `TestHidingOnAfterPublish`; scratch per-end area probe; mutants G5–G7 |
| 9 Preview, town, markers, range link follow saved radii | by construction through the transform (HR-6); `TrimmedView` town via `encode` | `fx.read` checks preview = render of public file; `suggest_test.go`; mutants G12, G13. Range links and markers by construction (public data only) |
| 10 No radius in public files, page text, requests; edit dialog shows both | HR-6; `radii()` saved values | `TestHidingRadiiNotPublic`, scratch byte-identity + 748 dialog comparisons, `edit.dom.test.ts` F-58 block; mutant F4 |
| 11 Migration on → 300/300, off → 0/0; pages unchanged after regenerate | `0009_hiding_radii.sql`, `UpdateDerived` | `TestMigrateRadii`, `TestRegenerateFillsHidingSteps`; scratch diff against 474d9b5 (272 files identical); mutants G9, G9b |
| 12 Delete/expiry leaves no radius | columns on the row (HR-10) | by construction (row delete, `secure_delete`); `hiding_steps_test.go` checks logs hold no radius, steps or token |

Not run: a real browser (no headless browser installed; none downloaded), `-race` on `internal/publish`, fuzzing beyond the 300-seed property test, and a live `serve` → `regenerate` on a real v8 data directory (the migration was checked on a v8 schema built from migrations 1–8 and through the publish comparison above).

## What the user should check by hand

1. On a published GPS workout, drag Start and End. The label should change while dragging, and the map grey and the chart shading should follow smoothly. After Save, the public page should show the cut; before Save it should not change.
2. Switch units to imperial on the edit page without reloading. The slider labels should switch (160 ft, 490 ft, 0.31 mi, 1.24 mi), and ⓘ should show the saved radii in miles.
3. On a phone, the two sliders should be usable with the thumb, and nothing should shift around the summary.
4. With a trim stored and hiding on, move the trim outward without saving and look at the shading (r1-3). Also check how the darker overlap looks.
5. Open an indoor workout. The sliders should be present, with no shading and no error.
6. On a local copy of real data (not the VPS): migrate and run `regenerate`. Workouts with hiding on should show 300/300, the others 0/0, and the public file names should be unchanged.
