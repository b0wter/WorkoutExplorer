# QA F-04 Workout data file reader - round 3

- **Date:** 2026-10-11
- **Verdict:** ready with conditions.
  - QA-04-r2-1 is fixed, and there are no new findings.
  - The only open item is the user's: the phone measurement (OQ-11). Until it is recorded, AC 5 cannot be ticked.
- **Inputs:**
  - `qa/F-04-workout-data-file-reader-r2.md`.
  - The working tree on top of `a00d10e`, compared with `diff -r` against my r2 snapshot of `src/web`.
  - A search for any file under `architect/`, `features/`, `src/` or `.gitignore` that is newer than the r2 report.

## Build and test results

| Command | Result |
| --- | --- |
| Change since r2 | **Only `src/web/src/lib/datafile/decode.test.ts`**: one new test, "overlap boundary … exactly 4 bytes (rule 6, QA-04-r2-1)", 8 lines. No other file under `src/`, `architect/`, `features/` or `.gitignore` changed |
| `make ci` (exit code captured directly) | **exit 0**. `svelte-check`: 0 errors and 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest: `Tests 271 passed (271)`. Bundle `10613 / 60000 bytes gzip PASS`, unchanged |
| Mutant `off < end − 4` (scratch copy) | **killed**: 1 failed (the new boundary test), 270 passed |
| Mutant: overlap check removed | killed: 2 failed (the r2 overlap test and the new boundary test) |
| `git status --porcelain` before and after the round | identical. Only this report and `qa/INDEX.md` were added or changed |

## Findings

None.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-04-r2-1 (overlap boundary untested) | **fixed** | The new test uses the r2 probe input: time at `t`, power at `t + 16`, file ending at `t + 26`. It first checks that this layout was built as intended, then asserts `corrupt`, rule 6 and "overlaps". The `− 4` mutant now fails exactly this test |

The r1 findings and the r1 questions stay as recorded in r2: all fixed or settled.

## Coverage

- **AC 1–4:** traced and passing, as in r2. No production code has changed since r1.
- **AC 5:** the desktop reference row is recorded. **The phone row is pending: user condition (OQ-11).** It must show `ok: true`, N 43,200 and 10 streams, and the cold median of 5 loads is read against 50 ms. This run is also the bench page's first run in a browser.
- **Not repeated this round:**
  - the manual real-file golden run, because `lib/datafile`, the format and the writer did not change since r2;
  - the full mutation set;
  - the Vite and `.gitignore` checks, because those files did not change.
- **Other conditions:**
  - OQ-10 (the error text the viewer sees) stays open with the user and does not block F-04.
  - CI uses Node 24 (`.nvmrc`); every local run here was Node 22.
