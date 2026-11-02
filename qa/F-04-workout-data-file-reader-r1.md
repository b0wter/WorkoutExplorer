# QA F-04 Workout data file reader - round 1

- **Date:** 2026-10-11
- **Verdict:** not ready. One Major finding drives this:
  - **QA-04-r1-1:** five checks in the decoder have no test that fails when the check is removed. The reject tests named after them pass because a different check rejects the same input. Two of these checks are the misaligned-column case of AC 1 and the "no `RangeError`" guarantee.
  - The decoder itself is correct. Every crafted input I tried is rejected with the right rule, and it matched Go on every golden fixture and on the three local real files.
  - The fix is test-only and small. Apart from that, the open items are one Minor for the architect, one doc Minor, and the user's phone measurement (OQ-11).
- **Inputs:**
  - Feature: `features/F-04-workout-data-file-reader.md`, with the goal reworded for OQ-9.
  - Decisions: `features/README.md`.
  - Spec: AP-3 and AP-4 and NFR Reliability in the product specification; "Workout data file" and "Front end" in the technical design.
  - Architecture: `architect/F-04-workout-data-file-reader.md` v1.0, ADR-007, `architect/F-03-workout-data-file-writer.md` v1.3 (§1–§8, and the v1.3 §6 note on Go and F-04 verdicts), ADR-002, ADR-004 rev. 5, ADR-005, ADR-006 rev. 2, `architect/overview.md` v1.5, `architect/INDEX.md`.
  - Code: the uncommitted working tree on top of `a00d10e`:
    - `src/web/src/lib/datafile/` (`index`, `format`, `model`, `meta`, `decode`, `load`, `testing/build-file`, 4 test files);
    - `src/web/test-support/golden.{mjs,d.mts}`;
    - `src/web/bench/{decode.html,decode.ts,MEASUREMENTS.md}`;
    - `src/web/vite.config.ts`, `src/web/tsconfig.json`, `.gitignore`.
  - Go reference: `src/server/internal/datafile/{decode,fixture,quantise,datafile}.go` (unchanged since `a00d10e`).
  - Real files: three, local and git-ignored (`development` ×2, `wahoo_fitness`).

**Privacy note for this report.** It names manufacturers and pass/fail results only. It contains no coordinates, times, dates, places or file names from real files.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | **exit 0**. `npm ci` prints `EBADENGINE` because Node is 22.22.2 locally and `.nvmrc` says 24; this is expected. `svelte-check --fail-on-warnings`: `180 FILES 0 ERRORS 0 WARNINGS`. `go test ./...`: 8 of 8 `ok` (cached; no Go change in F-04). Vitest: `Test Files 6 passed (6)`, `Tests 265 passed (265)`. Bundle: `entry index.html: 10613 / 60000 bytes gzip (9559 brotli) PASS`, the same as F-03 r2 (10613), so the CI size line is unchanged |
| `vite build` manifest | Only `index.html` is an entry (`assets/index-B2EdSt_T.js`). No bench asset and no `.bin` in `build/web` |
| Bundle with the design's temporary import (scratch copy of `src/web`, `main.ts` + `import { decodeDataFile, loadDataFile } …; console.log(…)`) | `13406 / 60000 gzip (12094 brotli)`, against 10613 without it. **Δ = 2,793 bytes gzip**, which confirms the engineer's figure and is under the 4,000 ceiling |
| Vitest on a scratch copy (node_modules symlinked, `testdata` symlinked) | 265/265 before and after every probe. The scratch copy was restored after each mutation |
| Decode timing, Node 22 under Vitest, `ride-12h.bin`, 3 runs | cold 25.2 / 25.3 / 27.0 ms; warm mean 4.04 / 4.84 / 4.12 ms; warm median 3.50 / 3.71 / 3.85 ms. This is consistent with the recorded desktop row (cold 18.9, mean 3.54), which was measured without the Vitest transform |
| `git status --porcelain` before and after the round | identical (14 entries). Only this report and `qa/INDEX.md` were added or changed |

Not run: `go test -race` (no Go change in this feature), and no browser. No browser is installed on this machine, so the bench page was never executed in one; see Coverage.

## Findings

### Major

- **ID:** QA-04-r1-1
- **Severity / kind:** Major / defect (tests that pass for the wrong reason)
- **Route:** frontend
- **Where:**
  - Checks: `src/web/src/lib/datafile/decode.ts:115` (first stream is `time_offset`), `:128` (column order), `:135` (`byte_offset % 4`), `:137` (overlap with the metadata or the previous column), `src/web/src/lib/datafile/meta.ts:158` (`fatal: true`, `ignoreBOM: true`).
  - Tests: `decode.test.ts:150` (alignment, "without a RangeError"), `:474` (invalid UTF-8), `:475` (BOM), `:514` (first stream not `time_offset`), `:522` (out of order), `:557` (not a multiple of 4), `:568` (below `align4(14 + L)`), `:570` (overlapping column).
- **Relates to:** AC 1 (4-byte alignment) and AC 3 (clear error state). Also the F-04 design table "Rejects (AC 3), one test each" and "Alignment … rule 6, no `RangeError`", and ADR-007 item 4.
- **Evidence:** I ran 53 mutations of the decoder on a scratch copy. 45 were killed. The 8 survivors split into two groups.
  - **Equivalent (no finding):**
    - The `off < 0 || off > length` guard. Its cases are rejected anyway by the overlap check and the `N × size` check (the same argument as F-03 r2 for Go).
    - `align4` on the first "previous end". Given `% 4`, no aligned offset lies between `14 + L` and `align4(14 + L)`.
    - The `null` part of `field()`. A `null` value fails every type check after it.
    - The top-level `try`/`catch`. No input reaches it, which is exactly what the robustness tests assert.
  - **Not equivalent, and no test notices:**

    | Check removed (mutant) | What the named test feeds | Why it still passes | Input that tells them apart (probe) | Real decoder | Mutant |
    | --- | --- | --- | --- | --- | --- |
    | `off % 4 !== 0` | `time_offset: 1001`; heart rate `+2` | Out of bounds (`N × size`), so rule 6 anyway. A `Uint8Array` never raises a `RangeError`, so `:150` cannot fail the way its title says | `time_offset` at the correct offset `+ 2`, file 2 bytes longer, so it is in bounds with no trailing bytes | `corrupt` rule 6, "not 4-aligned" | **`internal`**: "start offset of Int32Array should be a multiple of 4" |
    | `off < end` (overlap) | power at time `+ 16`; time at 12 | The columns stay at their built positions, so the trailing-bytes check fires (rule 6) | power at time `+ 12` (8 bytes overlap), file cut to end at that column | `corrupt` rule 6, overlap | **accepted**, `power = 8,0,1,0,1` (time bytes read as power) |
    | `streams[0] !== 'time_offset'` | `[power, time]` | The order check fires (rule 5) | `[latitude, longitude]` with no `time_offset` at all | `corrupt` rule 5 | **accepted** with `time.length = 0`, `sampleCount = 5`. This breaks the "every array has length N" invariant that F-05 and F-17 index by |
    | `k < order`, weakened to `k < order − 1` | `[time, power, heart_rate]` (not adjacent in §2) | Still out of order | `[time, cadence, heart_rate]` (adjacent) | rule 5 | rule 6 instead of 5 (overlap) |
    | `fatal: true` → `false` | bytes `{ 0xFF }` | `JSON.parse` fails on U+FFFD | `0xFF` inside the `sport` string of a valid file | rule 4, "not valid UTF-8" | **accepted** (sport contains U+FFFD) |
    | `ignoreBOM: true` → `false` | BOM + `{}` | `{}` misses `sport` (rule 4) | BOM + the complete valid metadata, same L | rule 4, "not valid JSON" | **accepted** (Go rejects it, so verdict parity breaks) |

  - The robustness tests (1,000 inputs) did not catch any of these mutants either.
- **Consequence:**
  - The current code is right, but the test suite does not protect it. A refactor that drops or misorders one of these checks passes CI.
  - The worst outcomes:
    - an `internal` error for a misaligned column, which is the exact bug class the alignment test claims to rule out;
    - overlapping columns decoded as silent garbage;
    - a `WorkoutData` whose `time` is shorter than `sampleCount`, which later features will index out of range.
  - AC 1's "tests cover 4-byte alignment" holds for the positive side (all four `(14 + L) % 4` residues decode). The negative side, the case that matters for robustness, is not asserted.
  - **What the fix needs:** one test per probe input above, each asserting `code`, `rule` and preferably the check named in `message`. Each must fail when its check is removed. The builder may need an option for a file length or `rawMetadata` with hand-set offsets.
  - **What would change my mind:** a reason why one of these checks is not part of the contract. The design lists all of them.

### Minor

- **ID:** QA-04-r1-2
- **Severity / kind:** Minor / risk (decision)
- **Route:** architect
- **Where:**
  - `architect/ADR-007-decoded-workout-data-in-the-browser.md` ("Would choose differently if": "warm decode **median** above 50 ms");
  - `architect/F-04-workout-data-file-reader.md` "Decode timing" ("warm **mean** above 50 ms on the phone");
  - `src/web/bench/MEASUREMENTS.md` (trigger stated as warm mean).
- **Relates to:** AC 5, ADR-007 item 5, NFR Performance.
- **Evidence:**
  - The viewer page decodes the file **once** per load, so the long task a user sees is the cold run. The trigger measures warm runs.
  - On the desktop the cold run is 5–7× the warm one: 18.9 ms against 3.54 ms in the recorded row, and 25–27 ms against 3.5–3.9 ms (median) in my three runs.
  - On a phone 4–6× slower than this i7-4770T, the cold run would be about 75–160 ms while the warm mean stays under 50 ms. The Worker trigger would then not fire although the page has a long task of over 50 ms. That figure is extrapolated, not measured.
  - The two documents also disagree on the statistic: median in ADR-007, mean in the design.
- **Consequence:** The phone measurement for OQ-11 can be read as "no action" while the user-facing decode on page load is a long task.
- **What would settle it:**
  - The architect states which number triggers the Worker (cold, warm, or both) and with which statistic, in ADR-007 and the F-04 design alike.
  - Or the phone row shows the cold run under 50 ms, which makes the question moot.

- **ID:** QA-04-r1-3
- **Severity / kind:** Minor / doc
- **Route:** architect (wording to forward to senior-requirements-engineer)
- **Where:** `features/F-04-workout-data-file-reader.md` (goal: "missing samples become gaps (NaN)"; scope: "Turn sentinels into gaps"), `architect/INDEX.md` and the F-04 design "Open questions" (OQ-9 still listed as open).
- **Relates to:** OQ-9; overview v1.5 glossary (Gap vs Missing sample).
- **Evidence:**
  - The reworded goal says "missing samples become gaps (NaN)". The overview glossary, updated for F-04, says a gap is a time gap of 5 s or more (`WorkoutData.gaps`) and a missing sample is `NaN`. It lists "missing sample" under "Not" for Gap.
  - The new wording uses the conflation the design was written to undo.
  - The goal has been reworded, but OQ-9 is still listed as open in `architect/INDEX.md` and in the design.
- **Consequence:** F-05 and F-17 authors read "gap" in the feature file and may treat `NaN` as a break (or `gaps` as missing values). This is cheap to fix now.
- **What would settle it:**
  - The goal says "missing samples become `NaN`" and the scope says "Turn sentinels into missing samples (`NaN`)".
  - OQ-9 is marked closed with a pointer to the reworded goal.

## Previous findings

None: this is round 1 of F-04.

**F-03 OQ-8 (F-03 AC 6): can be closed.** I checked `golden.test.ts` against the fixture schema in `src/server/internal/datafile/fixture.go`.

- **Every field is asserted:**
  - `fixture_version`: must be 1, otherwise the test fails with "fixture shape changed".
  - `format_version`: header bytes 4–5.
  - `byte_length`: `bin.byteLength`.
  - `sample_count`: `sampleCount`.
  - `metadata_length`: header bytes 10–13.
  - `streams[].name`: order and length, through `toEqual` on the full list.
  - `streams[].byte_offset`: read by the test from the file's own descriptors. The decoder does not expose offsets, so this checks the fixture against the file, not against the decoder; that is the right check here.
  - `valid`, `missing`, `first`, `last`, `min`, `max`.
  - `digest`: SHA-256 over `k\n` / `-\n`, first 16 hex characters, exactly as `Fixture` computes it.
  - On top: every value on the test's own §2 grid (`|v/s − k| < 1e-6`), and `values[firstValid] === first × s` (bit-identical multiply).
- **All 18 synthetic fixtures** run through it; the loader fails on unpaired files, foreign entries or an empty folder.
- **The golden tests catch decoder bugs.** These mutants were each killed, by between 16 and 28 failing tests: no delta accumulation, divide instead of multiply, raw scale dropped, `int8` read as `uint8`, raw sentinel ignored.
- **Real files, verified manually.**
  - I produced Go fixtures for the three local real data files (`build/corpus/real/*.bin`), with `go test -overlay` adding a scratch test that calls `Decode` and `Fixture`. No file under `src/` was changed.
  - I then ran `golden.test.ts` unchanged against them, in the scratch copy with the loader pointed at a scratch folder.
  - Result: 15 of 15 per-file tests pass (AC 2 plus the four derived rejects × 3 files). All three files decode `ok`: `development` ×2 (cycling, running) and `wahoo_fitness` (cycling).
  - The 5 failures in that run were the synthetic-name checks, which are expected in a folder without the synthetic files. The scratch copies were deleted afterwards.

## Questions

1. **Real files for AC 2 ("every corpus file").**
   - The design says the operator "may run the same test locally against `build/corpus/real/` output". But no fixture for real files exists anywhere, and `golden.mjs` hard-codes the synthetic folder, so this needs a QA-side overlay as I did above.
   - The question for the architect: is a manual QA run per release enough, or should there be a local-only path, for example a `GOLDEN_DIR` override plus a Go test that writes fixtures to `build/corpus/real/`?
   - Answer that settles it: "manual QA run is enough". Then I will repeat it in every F-04-affecting round.
2. **Bench page, never run in a browser.**
   - I verified on the Vite dev server that `/bench/decode.html` and `decode.ts` are served, that `decode.ts` imports the `ride-12h.bin` URL, and that the URL returns 200.
   - Nothing has executed the page itself.
   - Answer that settles it: the operator's phone run for OQ-11 also confirms the page works; no separate action is needed.

## Coverage

**Acceptance criteria**

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 Alignment, delta, sentinels per type, absent streams, scale | `decode.ts` `checkLayout`, `decodeDelta`, `decodeRaw`; `format.ts` `STREAM_SPECS` | `decode.test.ts` alignment (16 cases, all residues), delta (6 including int32 overflow), sentinels (every type, plus the values next to them), scale (bit-identical, spelling `1e-06` accepted, `1e-05` rule 5), absent streams (all 127 subsets) | **Traced, except** the negative alignment case (QA-04-r1-1) |
| 2 Matches the F-03 fixture for every corpus file within resolution | `decode.ts` | `golden.test.ts`, 18 synthetic fixtures, exact stored integers | Traced, passing. Real files verified manually (see Previous findings) |
| 3 Wrong magic or unknown version → clear error, no exception | `decode.ts:36-44`, result type, top-level `catch`; `load.ts` | 75 reject cases plus 3 order and message tests, `load.test.ts` (7), robustness (1,000 inputs, never `internal`), derived rejects on every golden | Traced and passing for magic and version. The other rejects are weakened by QA-04-r1-1 |
| 4 Indoor file: no position, no error | `decode.ts:98-103` | `golden.test.ts` AC 4 on `indoor-no-gps`, `indoor-no-distance-no-gps` and `pool-swim` | Traced, passing |
| 5 12-hour decode on a mid-range phone measured and recorded | `bench/` | Not a test (by design) | Desktop row recorded (Node 22 V8, not a browser), and my reruns agree. **Phone row pending: user condition (OQ-11)** |

**Requirements.**

- AP-4 (only present streams): there is no all-`NaN` array for an absent stream, and the key order is `METRIC_ORDER`. Both are tested, and a mutant that reverses the order is killed.
- AP-3: `position` is absent when there is no GPS, and present as a pair otherwise; the pairing is checked by rule 5 and rule 7.
- NFR Reliability: the decoder never throws. Its results come only from `failure()` or `ok`, the `try`/`catch` is a safety net that nothing reaches, and `loadDataFile` never rejects (tested with a broken `Response`).
- Privacy:
  - The module never logs.
  - Messages carry no sample values and no metadata values; there is one test for this.
  - `loadDataFile` drops the exception text so a URL cannot leak; the edit-token header is the caller's business.

**Architecture checked and correct.**

- Module boundary: `lib/datafile` imports nothing outside itself. Nothing in `src/` imports `testing/`, `test-support/` or `bench/`. `index.ts` exports exactly the design's public surface.
- §6 order matches Go `Decode` step by step, rule 7 included. I compared it against `decode.go:15-133`.
- Delta sums use plain JS numbers. The int32-wrap mutant is killed.
- Gaps are computed in the time loop, with `>=` 5 s. The `>` and off-by-one mutants are killed.
- The output does not keep the input buffer alive; this is tested.
- No new dependencies.

**Vite `fs.allow`, probed on the dev server.**

- Served: the synthetic `.bin`/`.json` files and everything under `src/web`.
- **403** for `src/server/go.mod`, `build/corpus/real/*.bin`, the repo-root `.gitignore`, `testdata/corpus/synthetic/*.fit`, a `../..` traversal into `testdata/corpus/expected/real`, and `/etc/passwd`.
- Encoded traversals and `/../server/go.mod` return the SPA `index.html`, not the file.
- The scope is as designed.

**`.gitignore`, checked with `git check-ignore -v --no-index`.**

- Ignored: `X.BIN`, `a.Bin.BR`, `src/web/x.bin`, `src/web/bench/ride.bin`, `synthetic/X.BIN`, `synthetic/x.Bin`, `synthetic/x.br` and `synthetic/sub/x.bin`.
- Not ignored: `synthetic/ride-12h.bin`.
- No golden file shows as deleted.
- `.json` files in `synthetic/` remain addable, which they must be for the fixtures. `golden.mjs` throws on an unpaired `.json`, so a stray fixture cannot sit there unnoticed. Nothing derived from real files is in the working tree.

**Engineer deviations, judged.**

- **Messages leave out values that Go includes** (for example Go's `unknown stream %q`). Accepted: the F-04 design forbids metadata values in messages. The one value that stays is the byte offset in the overlap message, which the design allows.
- **`align4` and `StreamSpec`/`StreamName` types exported from `format.ts`.** Accepted: they are internal to the folder and not re-exported by `index.ts`.
- **`failure()` lives in `meta.ts` and is imported by `load.ts`.** Preference only: a small shared `result.ts` would read better. Not a finding.
- **Robustness tests beyond the design.** Welcome, but they do not substitute for the targeted tests in QA-04-r1-1: none of those mutants was caught by them.

**Not traced or not run.**

- No browser run of the bench page.
- No phone run (OQ-11).
- OQ-10 (user-facing error text) stays open with the user and does not block F-04.

**User conditions (for a later ready verdict).**

- The phone row in `bench/MEASUREMENTS.md`, measured with the bench page (OQ-11). See QA-04-r1-2 for which number to read.
- CI runs on Node 24 (`.nvmrc`), while every local run here was Node 22.
