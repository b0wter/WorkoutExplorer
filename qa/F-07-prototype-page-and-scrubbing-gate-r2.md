# QA F-07 Prototype page and scrubbing gate - round 2

- **Date:** 2026-10-15
- **Verdict:** **ready with conditions.**
  - All five r1 findings are resolved, and no Blocker or Major finding is open.
  - The scrub recorder can now be trusted for the operator's phone gate run.
  - The conditions are not engineering work:
    - the operator's runs (AC 1, 2, 3, and the manual part of 7);
    - the user's answers (AC 8, and OQ-27 for a final Gate 0 verdict);
    - the ADR-011 close-out with the new E.
- **Inputs:**
  - Design and decisions: F-07 design v1.2, ADR-016 (decision 3 amended), ADR-011 rev. 7, ADR-014 rev. 2.
  - The r1 report.
  - State: the uncommitted working tree on `f037688`, untracked files included.

## Build and test results

| Check | Result |
| --- | --- |
| `make ci` | **Exit 0.** svelte-check: 259 files, 0 errors, 0 warnings. Go: 9/9 ok. Vitest: **30 files, 716 passed** (matches the engineer). Size line: `entry index.html: 45582 / 60000 bytes gzip (40912 brotli) PASS`. |
| `go test -race -count=1 ./tools/protoserve` | ok |
| `make prototype-data` | Unchanged: ride-12h br 266,150; ride-4h br 106,951; budget PASS. |
| protoserve, live on `127.0.0.1:18090` (built binary, stopped with SIGINT, port free afterwards) | `br` → `200 266150`; no `br` → `200 1176640`; `/.vite/manifest.json` → 404; `POST /` → 405. The log now reads `HEAD /nope 404 - 0 ae=no-br`. |
| Bundle | **E = 45,582**. **E − B0′ = 253** against my B0′ of 45,329, or 254 against the engineer's 45,328. Both are ≤ 300 and E ≤ 46,000. The entry CSS went from 1,094 to 1,124 gzip because the body rule moved there; the entry JS is unchanged at 44,458. **M1 = 284,135**, the same `map-view-DUwfvfLK.js` as r1, so the growth is still +141. The recorder is **2,434** gzip. Step 5: `(resolution: ` count in the entry is 0, and the grep finds `map-view`. Step 6: no `modulepreload`; the recorder is only a dynamic import of `index.html`. The built `index.html` has no `<style>`, and `body{…}` is in `index-*.css`. |
| Commit hygiene | Untracked or modified: only source, docs and QA files. Nothing is under `build/`, and there is no `.bin`, `.br`, `.json` or `.fit`. No tracked `.bin`, `.br` or `.fit` exists outside `synthetic/`. |

### Recorder mutants, re-run against the r2 code (33 measure tests)

- **13 of 14 killed.**
  - The four r1 survivors are now killed except M8:
    - M1, charts and map buffers swapped (2 tests fail);
    - M9, listener M a no-op (2 fail);
    - M12, `flushed` never reset (1 fails).
  - New mutants, all killed:
    - the old `input === 'touch'` condition put back (1 fails);
    - no-finger frames never counted (2 fail);
    - `scrubStats` ignoring `touch` (4 fail);
    - total measured from `tM` (2 fail).
  - The r1 mutants re-run and still killed: `≤ 20` made strict, `mapInView`, `startedOn`, the window length, listener removal.
- **M8 (`tM = tA` in listener A) is equivalent. I confirm the engineer's claim.**
  - The store calls listeners in subscription order and isolates throws (`store.ts:57-62`), so M runs after A in every flush.
  - `recording` cannot change between A and M, because it toggles only in a `setTimeout` (start) and in the recorder's own frame callback (finish). Neither runs inside a flush.
  - M is subscribed before Record can be pressed.
  - So whenever Z writes a sample, M has just overwritten `tM`.
- **One new survivor, M30:** removing `fingers = 0` from the click handler.
  - The code is correct, but no test covers it. See QA-07-r2-1.

## Findings

- **ID:** QA-07-r2-1
- **Severity / kind:** Minor / test gap (risk)
- **Route:** frontend
- **Where:** `src/web/src/measure/recorder.ts` (click handler, `fingers = 0`); `recorder.dom.test.ts` "touch validity"
- **Relates to:** ADR-016 decision 3; QA-07-r1-1
- **Evidence:**
  - Without the reset, mutant M30 passes all 33 tests.
  - In the real procedure (PP2 and PP3, three runs in a row), a finger still down when run *n* ends leaves `fingers = 1`, because the listeners are removed at `finish`.
  - Without the reset, a late finger in run *n + 1* would again go uncounted. That is r1-1, coming back on the second and third runs.
- **Consequence:** none today. The protection depends on one untested line.
- **Wanted:** a test with two runs in a row: the first ends with a finger down, and the second has a late finger. The second must report `no finger down in …`.

Trivial doc leftovers, no finding:
- `Prototype.svelte:7` still cites "ADR-011 rev. 6".
- Design v1.2 line 60, in the layout tree, still says "inline body style" for `index.html`.

## Previous findings

| r1 finding | Status | How verified |
| --- | --- | --- |
| QA-07-r1-1 Major: a late finger is not counted | **Fixed** | Design F-4 step 4 and ADR-016 decision 3 are amended. `recorder.ts:161` counts every frame with `fingers === 0`, and the reason is `no finger down in <z> frames`. Three new tests cover a late finger (143 frames, `Counts: NO`), a finger held the whole run (`Counts: yes`) and a mouse run (`–`). Mutants that bring the old condition back or stop counting are killed. |
| QA-07-r1-2 Minor: charts/map split and changed share untested | **Fixed** | `performance.now` is stubbed, and the tests assert `total 4.00 · charts 3.00 · map 1.00` exactly, plus a 49.92 % changed-share run. Mutants M1, M9 and M12 are killed; M8 is equivalent (above). |
| QA-07-r1-3 Minor: stale `MEASUREMENTS.md` template | **Fixed** | It cites design v1.2. The Bundle table is `B0 \| B0′ \| E \| E − B0′ \| recorder gzip \| M0 \| M1`. |
| QA-07-r1-4 Minor: HEAD 404 logged 10 bytes; 404/405 headers untested | **Fixed** | Live: `HEAD /nope 404 - 0`. `TestHandlerLog` has the HEAD 404 line. The r1 survivors G13 and G14 are now killed, and so is removing the HEAD branch. A HEAD request cannot reach 405, because HEAD is allowed. |
| QA-07-r1-5 Minor: inline `<style>` vs ADR-014 CSP | **Fixed** | `index.html` has no `<style>`. `:global(body)` in `Prototype.svelte` ends up in the entry CSS, at a cost of +30 gzip inside the 300 ceiling. |

## Is the recorder fit for the phone gate?

**Yes.**
- Every ADR-016 validity rule is enforced and has a test that can fail: the 30 Hz cap, a finger in every frame including late starts, a ≥ 75 % changed share, and the map in view.
- So are the pass rule and the charts/map split.
- The MutationObserver test still shows no DOM write during the window.

The only caveat is QA-07-r2-1. It is not a defect today, so it does not block the run.

## Conditions and pending items

- **Operator:**
  - PD1 to PD8 and PP1 to PP8, including the gate (PP2 ×3, PP3 ×3) and PD6 for AC 7.
  - Results go into `bench/MEASUREMENTS.md`. QA reviews them in the next round.
- **User:**
  - OQ-27 must be answered before the Gate 0 verdict is final.
  - OQ-22 to OQ-28 are needed for AC 8.
  - OQ-10 decides the shipped default text.
  - OQ-29 is optional.
- **Architect, close-out:** ADR-011 rev. 7 still records E 45,553 and a reserve of 1,447. With E = 45,582 the reserve is **1,418**, which is within the "at least 1,372" bound and above the 1,000 trigger. Record the final E at close-out.
- **Pending deletion (user):** `App.svelte`, `lib/health.ts` and `lib/health.test.ts` are still present. Only they import each other, and they are not in the build. README lines 28 and 30 are now correct.

## Coverage

| AC | Status |
| --- | --- |
| 1 12-hour file loads on the phone | Code and wiring tests pass. **Operator:** PP1, PD1, PD2 |
| 2 Chart touch scrub, 60 fps, smooth (gate) | Recorder verified (above). **Operator:** PP2 ×3 |
| 3 Route scrub at the same smoothness | As AC 2. **Operator:** PP3 ×3, PD5 |
| 4 `ride-4h` `.br` < 500 KB | Verified: 106,951, budget PASS, `TestSizeBudget4h` in CI |
| 5 Bundle < 60 KB | Verified: 45,582 / 60,000 |
| 6 Bottleneck identified if the gate fails | The split is now asserted by tests. Applies only if the gate fails |
| 7 Map sharp after zoom or monitor move | Code and tests verified in r1 and unchanged (same `map-view` chunk). **Operator:** PD6 |
| 8 Metric definitions handed to requirements | **User:** OQ-22 to OQ-28 |

**Not run:** the operator's checks, a real browser, and real FIT files.
