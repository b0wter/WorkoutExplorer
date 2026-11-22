# QA F-65 Owner-added laps - round 2

- **Date:** 2026-11-22
- **Verdict:** ready with conditions. No Blocker or Major finding is open. QA-65-r1-1 is closed by the architect's ceiling decision on main (`f1f5f3c`), and every r1 Minor is fixed. One new Minor test gap (QA-65-r2-1). Conditions: the hand checks listed under Coverage, which r1 also left open.
- **Inputs:** QA r1 (`36dc108`); diff `36dc108..9070e3c` on branch `worktree-agent-aca7cc8627b50d23a`: `dba49c4` (frontend, r1-2 to r1-6) and `9070e3c` (Go, r1-2 and r1-7); main `f1f5f3c` (F-19 v0.4 owner chunk ceiling 20,000; F-20, F-56 and F-65 docs point to it). The branch merges with main cleanly (`git merge-tree`).

## Build and test results

- `make -k ci` in the worktree: **exit 0**. gofmt, `go vet`, svelte-check (0 errors, 0 warnings), Go tests all `ok`, vitest **91 files, 2,339 tests passed** (r1: 2,335), build, size, caddy-check (2.11.9), docker-check and update-check passed.
- Size: activity entry **60,497 / 62,000** gzip PASS. The diff adds nothing to the entry; r1 measured 60,521, and the difference is chunk-hash churn. Owner chunk `start.svelte-*.js` **16,902 gzip** in the size script's output (44,857 raw; r1: 16,745), so the r1 fixes cost +157. This is below the new 20,000 ceiling.
- Go mutants, each in a scratchpad copy and run over `publish`, `lifecycle`, `httpapi` and `cmd`. **All 4 killed:**
  - The dropped `onEnd` check now compiles as `OnBoundary = k == lo || k == hi; _ = onEnd`. Killed by `TestCheckEditsLapBoundaryKinds`.
  - Both window-end checks dropped: killed by the same test.
  - Only the `k == hi` check dropped: killed by the same test.
  - The deletion of a sent stored entry that the trim removes skipped (`case !c.Kept && stored && false`): killed by `TestAddedLapsSentStoredDeletedByTrim`.
  - In the window-end run, `TestTrimMemoryStopRule` also failed (trimmed 12 MB against untrimmed 11 MB). That run overlapped with `make ci`, so it is the known memory-sampler load flake and not caused by the mutant. The test passed in `make ci`.
- TS mutants, each over `src/edit`, `src/lib/addedlaps` and `src/pins`. **9 of 10 killed:**
  - Killed: the window-end filter dropped; the `focusout` close dropped; `v.map((m) => m.laps())` removed; the splits restart removed; `seams.taps()` removed; the × name back to `i + 1`; the cap back to `set.length`; `button.focus()` after a choice removed; `refocus` never set.
  - **Survived (1):** after a ×, focus never goes to "the × now at its place" (`xs[...]` replaced by `undefined`). See QA-65-r2-1.

## Findings

### Minor

- **ID:** QA-65-r2-1
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/edit/start.svelte.ts` `relaps()` refocus line (`xs[Math.min(refocus, xs.length - 1)] ?? … ?? '.plus-button'`)
- **Relates to:** QA-65-r1-4, AC 9 and AC 13 (keyboard)
- **Evidence:** The only test for this path removes the single added lap, so the heading is the expected target either way. The mutant that never focuses a neighbouring × survives. The fallback to "+" when the laps heading sits in a hidden block is not exercised either.
- **Consequence:** A regression that sends keyboard users to the heading after every ×, instead of to the next ×, would go unnoticed. Add one case with two added laps: remove the first, and focus is on the remaining ×. Small lane.

## Previous findings

| ID | Status | How verified |
| --- | --- | --- |
| QA-65-r1-1 (Major, owner chunk above 15,000) | **fixed** (decision) | F-19 v0.4 on main raises the ceiling to 20,000 with a reason: owner-only, and a split would add a request for about 3 KB. It also sets the next step: above 20,000, split rather than raise again. F-20 and F-56 now point to F-19 instead of repeating the figure, and history is recorded. The reason holds, because viewers never load this chunk. The chunk is now 16,902, which leaves 3,098 of headroom. The F-65 Cost row and ADR-011 still give 16,745; the next measurement updates them. |
| QA-65-r1-2 (Minor, test gaps) | **fixed** | All 7 named mutants (3 Go, 4 TS) are now killed, see above. The Go device-end case runs at the real port on a track where the device lap end, the trim start and the trim end are three different samples. |
| QA-65-r1-3 (× name on multisport) | **fixed** | `rows()` numbers rows through `lapsBySection`, the same grouping `lib/page/view.ts` uses for the tables. Its output keeps input order and every lap, so the flattened numbers line up with `data.meta.laps`. The triathlon test asserts "lap 1" on the first Bike lap, and the `i + 1` mutant dies. |
| QA-65-r1-4 (focus after a choice or ×) | **fixed** in code and jsdom; real browser not checked | After a choice, focus returns to "+". After a ×, it goes to the × now at that place, else to the laps heading (`tabindex=-1`), else to "+". Mutants are killed, except the neighbour case (QA-65-r2-1). |
| QA-65-r1-5 (cap counts trimmed boundaries) | **fixed** | `set.filter(kept).length`. A test with 100 stored boundaries and a live trim checks it; the mutant dies. Server parity: the server counts the sent set, and the browser leaves trimmed boundaries out of it (AL-8). |
| QA-65-r1-6 (ADR-011 reserve) | **fixed** | The Reserve row ends with "**1,479** measured after F-65 (entry 60,521)". |
| QA-65-r1-7 (`CheckPins` only used by tests) | **fixed** | The method is removed from `publish/laps.go`. A test helper `checkPins` in `pins_test.go` calls `CheckEdits`. |

## Questions

None.

## Coverage

- r1's AC table stands. This round changed nothing outside the diff: AC 5's real-port gap and AC 10's edit-page wiring gap are now covered by tests, and the rest is unchanged.
- **Not run (conditions, carried from r1):** real-browser checks of keyboard focus after a choice and after a ×, themes, phone and desktop, touch, a screen reader (AC 13); LCP; the ADR-016 60 fps scrub on `ride-12h` with 100 added boundaries (AC 14).
