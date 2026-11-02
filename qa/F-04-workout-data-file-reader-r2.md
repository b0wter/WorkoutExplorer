# QA F-04 Workout data file reader - round 2

- **Date:** 2026-10-11
- **Verdict:** ready with conditions.
  - All three r1 findings are fixed, and both r1 questions are settled in design v1.1. Every one of the six targeted tests now kills its mutant.
  - One new Minor (QA-04-r2-1): no test sits exactly on the overlap boundary. It is test-only and does not block.
  - The remaining condition is the user's: the phone measurement (OQ-11). It is also the first run of the bench page in a browser.
- **Inputs:**
  - Feature: `features/F-04-workout-data-file-reader.md` (goal, scope and AC 1 reworded), `features/F-05-stacked-charts-with-shared-cursor.md` (scope reworded), `features/README.md`.
  - Architecture: `architect/F-04-workout-data-file-reader.md` v1.1, ADR-007 rev. 1, `architect/INDEX.md` (OQ-8, OQ-9, Q1 and Q2 marked closed), `architect/overview.md` v1.5.
  - Previous round: `qa/F-04-workout-data-file-reader-r1.md`.
  - Code: the uncommitted working tree on top of `a00d10e`. I compared it file by file (`diff -r`) against my r1 snapshot of `src/web`. Only three things changed:
    - `src/lib/datafile/decode.test.ts`: the `layoutOf` helper, the alignment test rewritten, and a new describe block "distinguishing rejects", 5 tests;
    - `bench/decode.ts` and `bench/decode.html`;
    - `bench/MEASUREMENTS.md`.
  - **No production code changed** (`decode.ts`, `meta.ts`, `load.ts`, `format.ts` and `model.ts` are byte-identical to r1).
  - Real files: three, local and git-ignored (`development` ×2, `wahoo_fitness`).

**Privacy note for this report.** It names manufacturers and pass/fail results only. It contains no coordinates, times, dates, places or file names from real files.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | **exit 0**. `svelte-check`: `180 FILES 0 ERRORS 0 WARNINGS`. `go test ./...`: 8 of 8 `ok` (cached; no Go change). Vitest: `Test Files 6 passed (6)`, `Tests 270 passed (270)`, against 265 in r1. Bundle `10613 / 60000 bytes gzip (9559 brotli) PASS`, unchanged |
| `vite build` manifest | One entry (`index.html` → `assets/index-B2EdSt_T.js`). No bench asset in `build/web` |
| Mutation run (the same 53 mutants as r1, on a scratch copy identical to the tree) | **48 killed, 5 survived**. All 6 mutants named in QA-04-r1-1 are killed. Of the survivors, 4 are equivalent and 1 is new, see QA-04-r2-1 |
| Manual real-file golden run (design v1.1, Q1) | Go fixtures for `build/corpus/real/*.bin` via `go test -overlay` with a scratch test file. Then the unchanged `golden.test.ts` in the scratch copy, with the loader pointed at a scratch folder. **15 of 15 per-file tests pass** (AC 2 plus the four derived rejects, × 3 files): `development` ×2, `wahoo_fitness` `ok`. The 5 failures are the expected synthetic-name checks. The scratch fixtures and copies were deleted |
| Vite dev server | `/bench/decode.html` (with the `reset` button) and `/bench/decode.ts` return 200; `decode.ts` imports `ride-12h.bin?import&url`. The server was stopped afterwards |
| `git status --porcelain` before and after the round | identical. Only this report and `qa/INDEX.md` were added or changed |

Not run: there is no browser on this machine, so neither the bench page nor its storage logic was executed. I reviewed that logic by reading, below.

## Findings

### Minor

- **ID:** QA-04-r2-1
- **Severity / kind:** Minor / defect (test boundary)
- **Route:** frontend
- **Where:**
  - Check: `src/web/src/lib/datafile/decode.ts:137` (`if (off < end)`).
  - Test: `decode.test.ts`, "overlap: a column starting inside the previous one", which overlaps by 8 bytes.
- **Relates to:** AC 3, F-03 §6 rule 6, and the r1-1 fix.
- **Evidence:**
  - Mutant `off < end − 4` survives all 270 tests.
  - Probe input: time at `t` (20 bytes), power at `t + 16` (4 bytes overlap, the smallest aligned overlap), file cut to end at that column.
  - Real decoder: `corrupt` rule 6, "overlaps".
  - Mutant: **accepted**, with `power = 1,0,1,2,3`, read from time bytes.
- **Consequence:** An off-by-4 regression in the overlap bound passes CI and lets a crafted file decode into garbage. Fix: move the existing overlap test to `t + 16` / `t + 26`, or add that case beside it.
- **Correction to r1.** My r1 report said "8 survivors". The correct count was 12, and I did not classify this boundary mutant there. It survived in r1 as well; it is not a regression.

## Previous findings

| ID | Status | Evidence |
| --- | --- | --- |
| QA-04-r1-1 (Major, untested checks) | **fixed** | Each mutant from r1 is now killed: `% 4` (1 test), overlap (1), first stream `time_offset` (1), weakened order (1), fatal UTF-8 (1), BOM (1). The tests use the r1 probe inputs. Each one checks that its input is built as intended (`layoutOf`), and asserts `code`, `rule` and the message of the check. The BOM test also proves that the same bytes without the BOM are accepted, so it cannot pass for the wrong reason. The misleading "RangeError" test is replaced by an in-bounds `int32` test that asserts "not 4-aligned". Boundary remainder: QA-04-r2-1 |
| QA-04-r1-2 (Minor, trigger on warm runs) | **fixed** | ADR-007 rev. 1, design v1.1 "Decode timing" and `MEASUREMENTS.md` all say the same thing: cold decode, median of 5 page loads, above 50 ms on the phone, with warm values for information only. No "warm mean/median" trigger remains in the architecture or the bench. Bench logic reviewed below |
| QA-04-r1-3 (Minor, gap vs missing sample) | **fixed** | The F-04 goal says "missing samples become `NaN`". The scope separates "missing samples (`NaN`)" from a new bullet on time gaps reported as indices, and AC 1 says "missing sample, `NaN`". F-05 scope: "Missing samples (`NaN`) and gaps (5 s or more between samples) drawn as breaks". No other feature file uses "gap" for a sentinel (grep). OQ-9 is closed in the design and in `architect/INDEX.md` |
| r1 Q1 (real files for AC 2) | **settled** | Design v1.1 and INDEX: a manual QA run in every round that changes `lib/datafile`, the format or the writer. Done this round, see the table above |
| r1 Q2 (bench never run in a browser) | **settled** | The phone run counts if it shows `ok: true`, N 43,200 and 10 streams. This is written in the design and in step 5 of the `MEASUREMENTS.md` procedure |
| F-03 OQ-8 | **closed** | Recorded closed in `architect/INDEX.md` on the basis of r1. `golden.test.ts` is unchanged and still passes on all 18 synthetic fixtures and on the 3 local real files |

### Bench logic (r1-2), reviewed by reading

- **One cold run per load.**
  - The Run listener is registered with `{ once: true }`, and the button stays disabled with the text "Reload the page for the next cold run".
  - The timed call is the first `decodeDataFile` in the page: nothing calls it at module load or after the fetch.
  - Correct.
- **Storage failures.**
  - `readCold`, `appendCold` and `clearCold` each catch exceptions.
  - Non-array or non-number content is filtered out; JSON cannot hold `NaN` or `Infinity`, so nothing else slips through.
  - When storage is unavailable the page still shows this load's cold value, but it never shows a median ("stored cold ms (0)"). The operator would then have to take the median of 5 by hand. That is acceptable for a one-off dev page and not a finding.
- **Median.**
  - It sorts a copy, takes the middle value for odd lengths and averages the two middle values for even lengths.
  - It is applied to `slice(-5)` only once 5 or more values are stored, so it is always the odd case.
  - Min and max come from the same 5 values.
  - Correct.
- **Minor observation, no finding.** A failed decode would also store its cold time. Decoding the same file is deterministic, and step 5 requires `ok: true`, so this cannot skew a valid run.
- **Desktop row.**
  - 5 Node processes standing in for 5 page loads give a cold median of 20.2 ms. My r1 runs under Vitest gave 25–27 ms, which includes the transform overhead.
  - The row states that it is not a browser.
  - Acceptable as a reference.

## Questions

None.

## Coverage

| AC | Status |
| --- | --- |
| 1 Alignment, delta, sentinels per type, absent streams, scale | Traced and passing. Alignment is now asserted on both sides: all four `(14 + L) % 4` residues decode, and an in-bounds misaligned `int32` column is rejected by the alignment check itself |
| 2 Fixture match for every corpus file | Traced and passing: 18 synthetic fixtures in CI. The 3 local real files were verified manually this round |
| 3 Wrong magic or version → clear error, no exception | Traced and passing: 75 reject cases, 5 distinguishing rejects, `load.test.ts`, robustness. Boundary gap: QA-04-r2-1 (Minor) |
| 4 Indoor file: no position, no error | Traced and passing (`indoor-no-gps`, `indoor-no-distance-no-gps`, `pool-swim`) |
| 5 Phone decode measured and recorded | Desktop reference row recorded. **Phone row pending: user condition (OQ-11)**. It also serves as the bench page's browser check |

- **Equivalent survivors, the same as r1:**
  - the `off < 0 || off > length` guard, covered by the overlap and `N × size` checks;
  - `align4` on the first "previous end", covered by `% 4`;
  - the `N × size` check, covered by the trailing-bytes and overlap checks before any view is created;
  - the `null` branch of `field()`, covered by the type checks;
  - the top-level `try`/`catch`, which no input reaches.
- **Unchanged since r1 and not re-reviewed beyond the diff:**
  - the production module;
  - the Vite `fs.allow` scope (`vite.config.ts` is unchanged);
  - `.gitignore` (unchanged);
  - the bundle delta.
- **Conditions:**
  - OQ-11: the phone run on the bench page (user). Read the cold median of 5 loads against 50 ms, and check `ok: true`, N 43,200 and 10 streams.
  - OQ-10 (the error text the viewer sees) stays open with the user and does not block F-04.
  - CI uses Node 24 (`.nvmrc`); every local run here was Node 22.
