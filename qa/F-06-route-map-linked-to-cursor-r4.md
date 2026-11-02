# QA F-06 Route map linked to the cursor - round 4

- **Date:** 2026-10-14
- **Verdict:** **ready.**
  - r3-2, r3-3 and r3-4 (a–d) are resolved.
  - The new validation test can fail.
  - The production bundle is byte-identical to r3.
  - There is one new Minor finding, QA-06-r4-1: the user's decision to move r3-1 to F-07 is not written down anywhere.
- **Inputs:**
  - Design and decisions: F-06 design v1.5, ADR-013 rev. 3, ADR-011 rev. 5 close-out, overview v1.9, `architect/INDEX.md`.
  - Requirements: spec AP-3, TD Front end step 4, `features/F-06`, `features/F-25`, `features/README.md`.
  - Code and records: `src/web/package.json` and the lockfile, `map-style.validate.test.ts` (new), `map-style.test.ts`, `route.test.ts`, `bench/map.ts`, `bench/MEASUREMENTS.md`.
  - State reviewed: the uncommitted working tree on `422cb80`.

## Build and test results

| Check | Result |
| --- | --- |
| `make ci` | **Exit 0.** svelte-check found 246 files, 0 errors, 0 warnings. `go test ./...`: 8 of 8 ok. Vitest: **640 passed**, matching the engineer's report. Bundle: `10613 / 60000 PASS`. |
| Production bundle (scratch build (b) = ChartStack + cursor store + RouteMap) | Same file hashes as the r3 build: `index-DcR67U25.js` and `map-view-Dq7ngfhM.js` (283,994 gzip). Entry 42,557, so the static share stays at **2,029** (the engineer measured 2,056) and own lazy code at **3,868**. The CI entry chunk `index-B2EdSt_T.js` is unchanged. The only `src/web/src` changes are tests. |
| Can the new test fail? (scratch mutants of `map-style.ts`, run against `map-style.validate.test.ts` alone) | **4 of 6 killed:** a malformed filter expression, an invalid `line-cap` enum, `text-size: '10px'`, and `mergeStyle` dropping the `wx-` sources. The test also asserts that a deliberately broken layer fails validation. **2 survived, both limits of `validateStyleMin`:** a negative dash value, and a base style without `glyphs` that has a symbol layer. The latter is still killed by `map-style.test.ts` (glyphs assertion). |
| Dependency | `@maplibre/maplibre-gl-style-spec` is pinned to exactly `26.4.4` as a devDependency. The lockfile resolves 26.4.4, the same version `maplibre-gl` 6.14.0 depends on (`^26.4.4`). It was already installed, so the lockfile change is one line. |
| Commit hygiene | Untracked: only `map-style.validate.test.ts`. Nothing is staged. `continue.md` is gone. Real FIT files and their expected JSON are present but ignored (`git check-ignore` confirms). No tracked `.br` file, no `.bin` outside `testdata/datafile/synthetic/`, and no liberty JSON. |
| Working tree | Unchanged by QA apart from this report and its INDEX row. Probes ran in the scratchpad. |

## Findings

- **ID:** QA-06-r4-1
- **Severity / kind:** Minor / doc gap
- **Route:** architect
- **Where:** F-06 design v1.5 (change log, Map options `pixelRatio` row); `features/F-07-prototype-page-and-scrubbing-gate.md`
- **Relates to:** QA-06-r3-1, the user decision to move it to F-07
- **Evidence:** I searched `architect/` and `features/` for `pixelRatio`, `pixel ratio`, `setPixelRatio` and `r3-1`. Nothing records that the frozen-at-mount pixel ratio is a known limitation owned by F-07. The design's `pixelRatio` row still lists only the cap.
- **Consequence:** F-07 will be designed without this item, so the deferred defect can be lost: on desktop the map stays blurry after browser zoom or after moving the window to another screen. The fix is one line in the F-06 design's limitations or `pixelRatio` row ("frozen at mount; F-07 re-caps on DPR change"), plus a note in the F-07 feature file or design.

## Previous findings (r3)

| r3 finding | Status | Evidence |
| --- | --- | --- |
| r3-1: pixel ratio fixed at mount | **Deferred to F-07** by the user | Not recorded in the docs, see QA-06-r4-1 |
| r3-2: no style-spec validation in CI | **Fixed** | `map-style.validate.test.ts` covers:<br>- `baseStyle` for ride-12h, gps-dropout (recorded and bridge), pause-hours (time-gap bridge), M = 1, and a route with no lap markers;<br>- `mergeStyle` with a synthetic remote style (vector, raster, raster-dem, globe projection, terrain), applied twice and with `previous` undefined;<br>- a self-check that a broken layer fails.<br>The test can fail (see above). ADR-013 rev. 3 decision 7 and design v1.5 make it the rule for F-14, F-17, F-25 and F-26. |
| r3-3: stale trigger wording | **Fixed** | `MEASUREMENTS.md` and the `bench/map.ts` report line now both say "300 ms (OQ-21 accepted)". |
| r3-4 (a): dropout vs gap terminology | **Fixed** | GPS dropout and time gap are separate terms in AP-3, TD step 4, the F-06 scope and AC 4 (now one dropout file and one time-gap file), the F-25 AC, the README decision and the overview v1.9 glossary. All say both causes are drawn as a dashed bridge. AC 4's time-gap case has explicit tests: `route.test.ts` asserts pause-hours' bridge comes from the gap with adjacent positioned ends, and `map-style.test.ts` checks one bridge per file, drawn only by the dashed layer. |
| r3-4 (b): ADR-011 close-out | **Fixed** | F-06 share **2,100, final**; reserve **1,600, final**; the 2,200 fallback was unused. |
| r3-4 (c): MP1 cases in the design | **Fixed** | Design v1.5 MP1 has (a) swipes, (b) held drag-scroll and (c) resting finger, all "passed 14 Oct 2026". |
| r3-4 (d): "one GeoJSON line" | **Fixed** | ADR-013 rev. 3 decision 8 records one `wx-route` source with recorded and bridge features as a refinement of the TD. TD step 4, the F-06 scope and design D-7 use the same wording. |
| r2 question: who pays for F-07's page wiring | **Open, not blocking** | For F-07's design. The static headroom is 44–71 bytes. |

## Coverage

**Acceptance criteria.** Unchanged from r3, all traced to code with passing tests and recorded operator checks. AC 4 now has automated cases for both causes: the gps-dropout golden file (GPS dropout) and the pause-hours golden file (time gap).

**Docs.** I compared the spec, TD, the F-06 and F-25 feature files, the README, design v1.5, ADR-011, ADR-013, overview v1.9, the INDEX and `MEASUREMENTS.md`. They agree, apart from QA-06-r4-1. One small leftover: the `MEASUREMENTS.md` header still cites "design v1.4 … ADR-013 rev. 2" as the procedure source. That is harmless, because both still define the procedure and the trigger.

**Not run.** No browser checks; none were needed for this round.
