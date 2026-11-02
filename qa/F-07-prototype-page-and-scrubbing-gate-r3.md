# QA F-07 Prototype page and scrubbing gate - round 3

- **Date:** 2026-10-15
- **Verdict:** **ready.**
  - Gate 0 passes under ADR-016's rules on the reference phone. Every AC is traced. `make ci` passes on a clean checkout. No Blocker or Major finding is open.
  - Three Minor follow-ups remain. None of them affects the verdict.
- **Inputs:**
  - The feature file, the F-07 design v1.3, ADR-016 (decision 5 accepted), ADR-011 rev. 7 and ADR-004 rev. 3 item 7.
  - The spec and TD changes in `74380cb` and `9ccd744`, `features/README.md` at HEAD, and the r1 and r2 reports.
  - State: HEAD `e478e7a`. It differs from the operators' run commit `9ccd744` only in `.claude/agents/`. Also reviewed: the uncommitted `src/web/bench/MEASUREMENTS.md`. I ignored the uncommitted status row in `features/README.md`.

## Build and test results

| Check | Result |
| --- | --- |
| `make ci` in the working tree | **Fails** in `go test`: `TestCorpus/real/<PP8 file>`, "no golden report … run make corpus-update". The cause is the operator's local real file. It is not caused by committed code. See QA-07-r3-3. |
| `make ci` in a clean clone of HEAD (scratchpad) | **Exit 0.** svelte-check: 256 files, 0 errors, 0 warnings. Go: 9/9 ok. Vitest: **29 files, 706 passed** (717 − 11 from removing `App.svelte` and `lib/health*` in `59b3c5b`). Size: `entry index.html: 45582 / 60000 bytes gzip (40912 brotli) PASS`. Lazy chunks: recorder 2,434 gzip and `map-view-DUwfvfLK.js` 284,135 gzip, both unchanged from r2. |
| `go test -race -count=1 ./...` (clean clone) | 9/9 ok |
| r2-1 check: mutant M30 (`fingers = 0` removed from the click handler, clean clone) | **Killed.** The new test "two runs in a row … (QA-07-r2-1)" fails, with 1 failed and 33 passed out of 34. Restored, 34 pass. I re-ran no other mutants, because the code has not changed since `e4cbb8d` apart from the dead-file removal. |
| Commit hygiene | Only `features/README.md` and `MEASUREMENTS.md` are modified. Nothing is untracked. `build/`, `corpus/real/*` and `expected/real/*` are ignored (`git check-ignore`). No `.fit`, `.bin` or `.br` is tracked outside the two synthetic folders. Only `.gitkeep` is tracked in `real/`. The PP8 file's name appears in no tracked file. |

## Gate 0 against ADR-016

- **Decision 5 conditions:** recorded on every phone row. iPhone 13 mini, iOS 18.7, Orion, Low Power Mode off, portrait, synthetic `ride-12h`, map in view, production build at `9ccd744`, over `-tls`.
- **Decision 3 validity:** all six gate runs meet every rule.
  - Median interval: 17 ms (< 25).
  - No-finger frames: 0.
  - Changed share: 93.50–100.00 % (≥ 75).
  - Map in view: yes.
  - The two earlier HTTP runs (247 and 51 no-finger frames) are correctly `Counts: NO`.
  - Desktop route run 6 (72.33 % changed) is correctly `Counts: NO`.
- **Decision 2 pass rule:** every gate run passes.
  - Share ≤ 20 ms: 99.50–99.83 % (≥ 99).
  - Flush p95: 1 ms (≤ 2).
  - Flush max: 1–3 ms (≤ 8).
  - The transcribed rows are internally consistent with their `Counts` and `PASS` columns.
- **Decision 4:** met. Chart runs 1–3 and route runs 1–3 are counted, pass and are judged smooth ("pretty much instant"). No counted run failed, so the repeat clause was not used.
- **Verdict PASS is correct.**

### PP8, real long file (information only; aggregates only, per design v1.3 PP8 and ADR-004 rev. 3)

| Item | Value |
| --- | --- |
| Writer / sport | `development` / cycling |
| Size of the workout | 45,400 samples, 7 streams, 61 laps, about 16 h, about 301 km |
| `data.bin` | 1,058,444 B; brotli about 243.5 KB |
| Desktop Firefox: load | Body read + decode 4 ms. Data / charts / map ready: 116 / 159 / 297 ms |
| Desktop Firefox: scrub | One counted mouse run, **PASS**: 100 % ≤ 20 ms, max 17.08 ms, cursor changed 77.41 % |
| Phone: load | Body read + decode 26 ms. Data / charts / map ready: 347 / 441 / 556 ms |
| Phone: scrub | One run, **did not count**: 16 frames without a finger, cursor changed 69.50 %. As information: 99.33 % ≤ 20 ms, max 50 ms, flush max 1 ms |

The real file has 5 % more samples than `ride-12h`, but fewer streams. It behaves like the gate file, so the gate file is representative. Decode stays under the ADR-007 trigger of 50 ms.

## Findings

- **ID:** QA-07-r3-1
- **Severity / kind:** Minor / record gap (risk)
- **Route:** frontend (wording in `MEASUREMENTS.md`)
- **Where:** `src/web/bench/MEASUREMENTS.md`, F-07 section (uncommitted)
- **Relates to:** design v1.3 "What to record", PD1, PD5, PD6, PP1, PP2 and PP6; ADR-016 decision 3
- **Evidence:** the section follows the template's tables, but it departs from the procedure in these ways. Each departure is labelled honestly in the section ("not reported", "not pasted").
  - No raw Load or Scrub blocks; the values are transcribed.
  - The phone has 2 Load rows instead of PP1's 5.
  - Charts and map p95/max are "not reported" for the phone gate runs.
  - Transfer sizes come from Resource Timing, for `ride-12h.bin` only. There is no `protoserve` line for anything else, PP6's `ride-4h` line included.
  - Wording that is wrong:
    - **PD5 is marked "pass", but it has only 2 counted route runs, where PD4/PD5 ask for 3.**
    - **PD6 does not say whether the monitor move was checked.** The design asks for "monitor move not checked" if it was not.
    - The verdict row calls route run 6 "PASS by the rule". ADR-016 decision 3 says that a run that does not count is never a pass or a fail.
- **Consequence:**
  - None for Gate 0. Every value the ADR-016 rules need is present.
  - AC 4 is covered by `make prototype-data` (106,951 B) and `TestSizeBudget4h`.
  - AC 6 needs the split only on failure, and the flush max was ≤ 3 ms.
  - The ADR-007 information trigger (median of 5) cannot be evaluated as written, but both phone values (28 and 5 ms) and F-04's cold median of 31 ms are under 50 ms.
  - Later re-measurements (F-17, F-25, F-32) have no recorded charts/map baseline from the phone.
- **Wanted:** fix the wording only: PD5 "2 counted of 3 (information only)", and PD6 "monitor move checked" or "not checked". Also drop "PASS by the rule". No re-run.

- **ID:** QA-07-r3-2
- **Severity / kind:** Minor / doc inconsistency
- **Route:** architect
- **Where:** `architect/F-07-prototype-page-and-scrubbing-gate.md:643` (OQ-29 row); `architect/INDEX.md` (resolved OQ-29 row); `MEASUREMENTS.md` PP8 line, which points to that row
- **Relates to:** ADR-004 rev. 3 item 7; design PP8
- **Evidence:**
  - ADR-004 item 7 says sizes for real files are "not recorded in committed files such as `MEASUREMENTS.md`" and go into the QA report.
  - The design's own PP8 says "Record in the QA report".
  - The OQ-29 row records the real file's `data.bin` and brotli bytes, distance, duration and lap count in two committed architecture docs.
- **Consequence:**
  - The privacy impact is low: there is no name, date or coordinates.
  - But the accepted ADR and the design disagree on where real-file facts may live, and the next feature will copy whichever it reads first.
- **Wanted:** either trim the OQ-29 rows to "aggregates in QA F-07 r3", or amend ADR-004 item 7 to allow rounded aggregates in design docs.

- **ID:** QA-07-r3-3
- **Severity / kind:** Minor / procedure gap
- **Route:** architect (PP8 text). The operator then runs `make corpus-update` locally.
- **Where:** design PP8; `src/server/internal/ingest/corpus_test.go:51`
- **Relates to:** ADR-004 rev. 3 item 7 ("a missing report is `NEW` and fails until `make corpus-update` is run there"); ADR-004 naming `<brand>-<model>-<what>.fit`
- **Evidence:**
  - PP8 tells the operator to put the real file in `corpus/real/`. It does not say to run `make corpus-update`.
  - `make ci` in the working tree therefore fails on `TestCorpus` (output above).
  - The file name also does not follow ADR-004's naming, and it contains a date. That date would flow into the ignored golden name and into the `real-<file>` URL path in `protoserve` log lines.
- **Consequence:**
  - The operator's local `make ci` is red until someone acts, and a red local run hides real regressions.
  - CI and clean checkouts are unaffected.
- **Wanted:**
  - PP8 should add "rename per ADR-004, then `make corpus-update` and review".
  - Alternatively, keep PP8 files outside the corpus, since `REAL=` takes any path.

**Doc leftovers, no finding:**
- Design line 615 still says "measured 45,553" for AC 5; the close-out value is 45,582.
- `load-error.ts:2` still says "OQ-10 is open with the user"; it was closed on 15 Oct.

**Information:** the phone's encoded size for `ride-12h.bin` (266,312) is 162 B above the served brotli body (266,150; protoserve compresses deterministically at start-up). This is probably WebKit counting HTTP/2 framing over `-tls`. It is not verifiable here and does not matter for the verdict.

## Previous findings

| r2 finding / condition | Status | How verified |
| --- | --- | --- |
| QA-07-r2-1 Minor: `fingers = 0` reset untested | **Fixed** | `recorder.dom.test.ts:395`, two runs in a row. Mutant M30 is now killed (above). |
| r2 doc leftovers (`Prototype.svelte:7` rev. 6; design line 60 "inline body style") | **Fixed** | Both now read rev. 7 and no inline style |
| Condition: operator runs | **Met** | `MEASUREMENTS.md`; Gate 0 PASS (above) |
| Condition: OQ-27 and OQ-10, OQ-22 to OQ-29 | **Met** | ADR-016 status and decision 5; design v1.3; `architect/INDEX.md`; spec AP-2, AP-5 to AP-8, SH-5 and NFR reference; "Metric definitions" ticked; F-13, F-17, F-18, F-23 and F-32 updated; README decisions. OQ-23, OQ-25 and the OQ-28 columns are marked provisional consistently in all of them. |
| Condition: ADR-011 close-out | **Met** | Rev. 7: E 45,582, F-07 254 final, reserve 1,418. Matches `make size`. |
| Pending deletion of `App.svelte` and `lib/health*` | **Done** | `59b3c5b`; −11 tests; E unchanged |

## Questions

1. **Phone OS version.**
   - The phone rows record "iOS 18.7", but Orion's user agent says `Version/27.0`. On iOS, every browser uses the system WebKit.
   - If the phone is in fact on a newer iOS, ADR-016 decision 5 needs a text update. A re-run is not needed.
   - Settled by: Settings → General → About on the phone (user).

## Coverage

| AC | Status |
| --- | --- |
| 1 12-hour file loads on the phone | PP1 (2 loads recorded), PD2; `Prototype.dom.test.ts` |
| 2 Chart touch scrub, 60 fps, smooth (**gate**) | PP2 ×3 counted PASS and smooth, under ADR-016 |
| 3 Route scrub, same smoothness | PP3 ×3 counted PASS and smooth (phone); desktop route 2 counted PASS |
| 4 `ride-4h` `.br` < 500 KB | 106,951; `TestSizeBudget4h` in CI |
| 5 Bundle < 60 KB | 45,582 / 60,000 (`make size`, clean clone) |
| 6 Bottleneck if the gate fails | Not triggered (gate passed); the split is asserted by tests (r2) |
| 7 Map sharp after zoom or monitor move | PD6 pass (zoom; monitor move not stated, r3-1); `map-view` tests (r1, chunk unchanged) |
| 8 Metric definitions handed to requirements | Done (`74380cb`, `9ccd744`); see Previous findings |

**Not run by QA:** a real browser or phone. Gate values are the operator's transcription; the raw blocks were not pasted.
