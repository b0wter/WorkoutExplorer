# QA F-16 Start and end hiding - round 1

- **Date:** 2026-10-26
- **Verdict:** ready. No Blocker or Major finding. All 10 ACs are traced to code and a passing test. Two Minor findings and one question for the user.
- **Inputs:** `features/F-16-start-and-end-hiding.md` (current ACs 1 to 10), `architect/F-16-start-and-end-hiding.md` v0.2 (H-1 to H-9, FE-1, Verification), ADR-028, ADR-005 items 6 and 7, ADR-026 items 2 and 3, spec UP-9, SH-4, SH-9, SH-10, AP-3, AP-17, TD Security and privacy (Hidden sections), I-3. Commits 7f5fff4 (FE-1) and 5053fe6 (backend), working tree clean at 9605fba. Critical lane.

## Build and test results

- `make ci`: exit 0. gofmt and vet clean, Go tests ok, vitest 40 files / 1,077 tests passed, entry index.html 50,220 / 60,000 B gzip PASS (unchanged from F-15 r2), upload entry 2,270 / 8,000 B PASS.
- `go test -count=1 -race ./...` (src/server): every package ok.
- F-16 tests, `-count=1 -v`: `TestHideEnds*` (9 tests, including the 2,000-seed property test) and `FuzzHideEnds` seeds pass; `TestHidingCorpus`, `TestHidingFullyHidden`, `TestHidingOnAfterPublish` pass. `TestKill` "remove first" and `TestSweepAfterStoppedRemoveFirst` now run with the real cut and pass.
- `make corpus`: 52 ok, 0 diff, 0 fail, 0 new, 0 stale; real: 6 files (local only).
- `FuzzHideEnds -fuzztime 120s`, run on a copy of the tree in the scratchpad: 212,863 execs, no failure.
- Mutants, applied to the scratchpad copy, run against `./internal/workout ./internal/publish ./internal/page ./cmd/workout-explorer` (plus `./tools/protoserve` for the page mutants):

| Mutant | Result |
| --- | --- |
| `>=` for `>` (the design's "`<` for `≤`") | killed (`TestHideEndsExactly300`) |
| circles centred on sample 0 / n-1 | killed (LateFix, EndSide, property) |
| end side dropped | killed |
| start side dropped | killed |
| H-3 dropped (inserted samples count as boundary) | killed (BoundaryAfterFilledGap, property) |
| `inserted` not recorded in `fillGaps` | killed |
| along-track distance instead of the circle | killed (JitterAtStart, property, corpus) |
| every in-circle sample hidden, not only until the first exit | killed (MidRouteReturn, property) |
| one more hidden sample public at the start / at the end | killed |
| fully hidden keeps all-sentinel position columns | killed |
| `from > to` check dropped | killed |
| transform skipped (identity) | killed (corpus, fully hidden, on-after-publish, e2e) |
| hiding applied with `hide_ends` off | killed |
| off → on written in place (P-7) | killed (`TestHidingOnAfterPublish`, `TestHidesMore`, `TestRemoveFirst`, sweep test) |
| `Hidden` set for an indoor workout | killed |
| `RouteHidden` not passed to the page / ignored by `page.Build` | killed |
| TS `build.ts` never sets `Hidden` | killed (page.test.ts, 2 tests) |

  Two further mutants survived and are equivalent: `+0` on a longitude, and removing `Hidden` from the template's load probe (the branch body is a literal, nothing in it can fail to execute).
- Real files (read only, nothing written under `corpus/real/` or `expected/real/`; probe test in the scratchpad copy, deleted after the run): 6 parsed, 6 with positions; the full publish-level check `checkHidden` (AC 1 to 4, H-6 page equality) passes on all 6, all partly hidden, none fully hidden. 0 of 6 have a first or last fix more than 300 m from each of its five neighbours (see Q1).

## Findings

### Minor

- **ID:** QA-16-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/internal/workout/hiding.go:26`, `src/server/internal/workout/workout.go:94-97`, `src/server/internal/workout/helpers_test.go:97` (`checkInvariants`)
- **Relates to:** H-3, H-4
- **Evidence:** `recorded` indexes `s.inserted[i]` for every `i < len(s.Offset)` and trusts that the two have the same length. Nothing checks it: `checkInvariants` checks every stream's length against `Offset` but not `inserted`, and `TestTransitionAllowListComplete` now skips the field by name. Today no rule after `fillGaps` changes the number of samples (read: `holdDistance`, `maskTransitions`, `settlePresence`, `convertCadence`, `computeDistance`), so it holds.
- **Consequence:** the privacy boundary depends on an unexported field that later sample-removing code has to remember to slice as well. UP-6 trim (phase 2), the first rule that removes samples, could miss it. A shorter `inserted` panics in the public build. A shifted one moves the H-3 boundary silently, so an inserted sample next to a hidden fix could become public. Fix: add `len(inserted) ∈ {0, len(Offset)}` to `checkInvariants` (the property test already calls it), or make `HideEnds` treat a length mismatch as "fully hidden".

- **ID:** QA-16-r1-2
- **Severity / kind:** Minor / risk (test)
- **Route:** backend
- **Where:** `src/server/internal/publish/hiding_test.go`, `circleBounds` and its use in `checkHidden`
- **Relates to:** AC 1, H-3, design Verification row 1 ("none is an inserted sample next to a hidden fix")
- **Evidence:** at publish level, `circleBounds` takes the first *positioned* sample (inserted ones included) more than 300 m out as the bound, so `TestHidingCorpus` and the real-file run do not check H-3. The workout-level oracle (`expectCut`, from raw record offsets) does check it, and the H-3 mutants are killed there. The gap is only that the end-to-end check over the corpus is weaker than the verification table says.
- **Consequence:** low. If a later change made publish build from something other than `workout.HideEnds` on a fresh parse (for example a cached or decoded workout without `inserted`), the publish-level tests would still pass. Fix: let `circleBounds` skip samples whose offset is not a recorded one (as `expectCut` does), or state in the test that H-3 is covered only in `workout`.

No finding on: hidden positions in `data.*.bin`, `.br` or `index.html` (by construction and by test: the encoded file equals `Encode(HideEnds(parse))`, every non-position column is bit-identical to hiding off, the metadata bytes are equal for partial hiding, and the page equals the hiding-off page except for the data file name). `datafile.Meta` has no coordinate field and no position-derived value. Interpolated samples at the boundary are hidden (H-3). Distance, altitude, speed and timing stay public by spec (SH-9, design "Not hidden, by the spec"). Not reported.

## Previous findings

None (first round). Not re-reported, as instructed: the TD Security and privacy wording, the ADR-028 "identity until F-16" line and the AC numbering of the design's verification table, all being fixed in parallel. Note: TD line 406 (Hidden sections) already describes the circle rule. F-15 r2-1 (the sweep against a cross-process data file) is unchanged and stays with F-21/F-30.

## Questions

1. **User: a bad first (or last) fix.** SH-4 centres the circle on "the first position". If a device or phone app records one stale or coarse first fix (last known place, cell-tower location) more than 300 m from the real start, the real start is outside that circle and stays public, with no warning. Evidence: 0 of 6 real files show it, and it is untested because the spec defines the rule this way. Suggestion: accept for the MVP as the spec reads. Revisit if a real phone recording shows it. A fix later would be to centre the circles on a robust start (for example the median of the first N fixes), or to hide until the track is more than 300 m from *every* one of the first N fixes.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 No hidden data in any public file | `workout.HideEnds`, `publish.publicTransform`, `build` | `TestHidingCorpus` (synthetic, at least 10 partly hidden + indoor), `TestHidingFullyHidden`, `TestHideEnds*` oracle + property + fuzz; real files 6/6 |
| 2 HR, speed, altitude kept | H-1, only Lat/Lon replaced | `checkHidden` bit-identical non-position columns; `checkCut` DeepEqual of everything but positions |
| 3 Summary distance and time identical | Summary/laps computed before the transform | `checkHidden` metadata equality |
| 4 Hiding off = full track | `publicTransform` returns w when off | `checkHidden` `off.bin == Encode(parse)`; M15 killed |
| 5 500 m walk fully hidden, notice, one column | `HideEnds` fullyHidden, `page.Build` `Route.Hidden`, `index.html:42`, no `has-map` | `TestHideEndsFullyHidden`, `TestHidingFullyHidden` (notice, no map slot, no `has-map`), `route-hidden` sample in page, protoserve and vitest tests. "Does not break": the client path is the indoor one (no `#map-slot`, no position columns) |
| 6 5 km on a 400 m track | as 5 | both fully hidden tests |
| 7 2 min jitter within 50 m | circle rule | `TestHideEndsJitterAtStart` (exact oracle), along-track mutant killed |
| 8 Indoor with hiding on: indoor hint | `HideEnds` returns false without positions | `TestHideEndsOffByDefaultIsIdentity`, `TestHidingFullyHidden` "indoor"; M8 killed |
| 9 Late fix | centre on the first recorded fix | `TestHideEndsLateFix`; M2 killed |
| 10 On after publish deletes the old data file | `HidesMore` HideEnds off → on | `TestHidingOnAfterPublish`, `TestPublishEndToEnd`, `TestKill` "remove first", `TestSweepAfterStoppedRemoveFirst` |

Also checked and correct: H-4 (one pure function in `workout`, only caller `publish`, nothing stored, cached or logged; the only log line in `publish` is the measure warning, no log in `HideEnds`); H-8 (`publish` never touches `draft.bin`; `RouteHidden` set only in `publish.build`); H-9 (on → off in place and restores the full bytes; a rebuild with hiding on gives the same bytes); FE-1 (TS and Go set at most one of Map, NoLocation, Hidden; Svelte mirrors the template).

Not run: a browser check of the fully hidden page (the markup and CSS are the indoor case, already checked in F-13). F-23 preview and F-24 export do not exist yet (their ACs re-check AC 1).
