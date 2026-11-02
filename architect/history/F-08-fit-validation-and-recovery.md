# F-08 FIT validation, truncated-file recovery and parse limits: history

History of [architect/F-08-fit-validation-and-recovery.md](../F-08-fit-validation-and-recovery.md). The live document holds the current state only. This file holds, verbatim, what was moved out of it on 22 Oct 2026 (restructuring, no decision changed): the change log, the header's status history, superseded and struck-through text, the engineer change lists v1.1 to v1.6 and the resolved questions. Read it only when you need the history.

## Header as of v1.7 (before the restructuring)

- **Version:** 1.7 (see Change log)
- **Status:** accepted. Rule R-13 (transitions) and all its details were decided by the user on 15 Oct 2026 (OQ-38, OQ-38a to OQ-38d, OQ-39). Rule R-14 (the distance stream never falls) is built and passed QA round 4. v1.5 (one synthetic corpus file and test fixes) is built and passed QA round 5. v1.6 changes one fixture's lap totals; the backend engineer builds it in F-09's first round (section [Changes for the backend engineer (v1.6)](#changes-for-the-backend-engineer-v16))

## Change log

| Version | Change |
| --- | --- |
| 1.7 | F-09 v1.5 (QA-09-r2-1): the walk counts the records the decoder accepted in each sequence, through a record counter on both decoder hooks (section Sequence walk, Decoder options; ADR-003 rev. 5). It is used only by the UP-9 strip, which cuts a truncated sequence at that count (ADR-023 rev. 4). The F-08 rules, the reports and the goldens do not change. The new synthetic file `ride-30min-bad-basetype.fit` (F-09 design, section Corpus) pins R-a for a definition with an invalid base type. Built in F-09 round 4 |
| 1.6 | QA round 5 ([qa/F-08-fit-validation-and-recovery-r5.md](../../qa/F-08-fit-validation-and-recovery-r5.md), verdict ready). **QA-08-r5-2, decided by the user (16 Oct 2026):** in `swim-bike-distance-fall.fit` each lap gets the same distance override as its session, so the bike lap's total equals the bike session's total (2,946 m) instead of last − first record (2,995 m). No writer produces a one-lap session whose lap is longer than the session, and a later lap-table check would otherwise inherit that as a golden. Corpus row updated; R-14 and every other rule are unchanged. The code change is built in F-09's first round (section [Changes for the backend engineer (v1.6)](#changes-for-the-backend-engineer-v16)). QA-08-r5-1 (a test comment) is a backend fix with no design change |
| 1.5 | QA round 4 ([qa/F-08-fit-validation-and-recovery-r4.md](../../qa/F-08-fit-validation-and-recovery-r4.md), verdict ready). **QA-08-r4-2:** status line updated (R-14 is built); real-file counts removed from the R-14 Problem paragraph, the v1.4 change-log row and `INDEX.md`. **Q1:** the CI distance invariant was vacuous (no synthetic file falls), so a new synthetic file `swim-bike-distance-fall.fit` pins R-14 end to end through `Parse` (a near-zero dropout, a fall inside the swim and a downward correction at the leg boundary). **QA-08-r4-1 (backend):** R-14's place after `fillGaps` is now stated with a worked example and pinned by a test; the record-leg case asserts an exact value. Section [Changes for the backend engineer (v1.4 follow-up)](#changes-for-the-backend-engineer-v14-follow-up) |
| 1.4 | QA round 3 ([qa/F-08-fit-validation-and-recovery-r3.md](../../qa/F-08-fit-validation-and-recovery-r3.md), verdict ready). **OQ-39 accepted by the user** (15 Oct 2026, "damaged files are a bonus not a must-have"): the behaviour as built stands, moved to Resolved questions; no code change. **QA-08-r3-2 / Q2 → new rule R-14, the distance stream never falls** (D-25): QA found that the device distance of a local real triathlon file falls at the T1 to bike boundary and drops to near 0 in several runs inside the open-water swim (counts in the QA report only). That is writer data (R-6a kept it as recorded), but it folds the readout, the distance axis, range distances, km splits and gradient. After R-3, the device distance stream becomes its running maximum over valid values; missing samples stay missing. Applied on the server, like R-13, so every consumer sees one non-decreasing stream; F-02 R-6a is marked in v1.11. No synthetic golden changes (every generated distance is already non-decreasing within a sequence); real goldens with a falling distance change in the distance stream and the values derived from it. `TestCorpus` gains the invariant "distance never falls"; the real-file check's point 3 is restated. The F-17/F-18 hand-off is widened from "a small decrease" (section Hand-offs). Section [Changes for the backend engineer (v1.4)](#changes-for-the-backend-engineer-v14) lists the change and its proof |
| 1.3 | User decision OQ-38, an engineer report and OQ-37. **OQ-38 → new rule R-13, transition legs:** at every sample covered by a session with FIT sport `transition`, and in the totals of those sessions and of laps that start in one, only position, altitude, heart rate, distance and temperature are kept; speed (so pace), cadence, power and every future stream become missing samples or absent totals. Applied in `workout.Normalise` after R-3 and before R-4, so every later rule, the golden report and the data file see the same values (D-21, D-22). Only a session makes a transition, never a sport hint or a fallback (D-23, user answer 9). The user's answers to the details (D-24): distance counts toward the total and the axis, transition time counts as moving, temperature is kept, summary speed, cadence and power exclude transitions while heart rate includes them, ranges and laps count transition distance and time, route colouring by speed, power or gradient is neutral in transitions (F-25), and `transition` counts as a sport (D-17 unchanged). New synthetic file `ride-transition-ride.fit` has every stream in its transition, with values that would win the maxima if the mask failed, and no timer events (proves R-13.M and the new "Multisport" AC 9); `triathlon.fit` (which already had records in its transitions, without every stream) keeps its bytes and loses only its transition speeds. Feature ACs: transition AC 8 and Multisport AC 9 are new; the limits and log ACs move to 10 and 11. **Engineer report (D-20):** a complete chained file whose later sequence has no session has one leg, so v1.2's R-10.E did not apply and the summary showed the session's device elapsed time. R-10.E is restated as one rule: the summary elapsed time is the wall time unless one session is the whole workout in one FIT sequence (D-18 revised). No golden changes for it. **OQ-37:** resolved by the user's OQ-35 answer; AP-2 now says "from the workout start". **OQ-38a to OQ-38d** resolved by the user's answers (section Resolved questions). **OQ-39 (new, open, blocks nothing):** a file cut inside a transition keeps the tail's speed, cadence and power (answer 9 applied literally); recommendation: accept. Section [Changes for the backend engineer (v1.3)](#changes-for-the-backend-engineer-v13) lists every change with its proof and the goldens allowed to change |
| 1.2 | User answers and QA round 2 ([qa/F-08-fit-validation-and-recovery-r2.md](../../qa/F-08-fit-validation-and-recovery-r2.md), verdict ready). **OQ-35 → wall time:** R-10.E is now `Derived.ElapsedTime`, the time from the workout start to the last record, transitions and gaps included (D-18). The time axis and the cursor already count elapsed time from the workout start across every leg; verified and stated in section [Time axis across legs](#time-axis-across-legs-v12), no change (D-19). **OQ-36 confirmed:** D-17 stands as built. **QA-08-r2-1 and Q2:** R-12 restarts only if the first distance of a later sequence is below half the previous one (midpoint rule); no unconditional offset (D-14 revised). **Q1:** a complete later sequence without a session gets no record leg; accepted and pinned by a test (D-20). **QA-08-r2-3:** F-02 v1.9 marks its boundary sentence on `multisport`. QA-08-r2-2 is a test gap for the engineer. Section [Changes for the backend engineer (v1.2)](#changes-for-the-backend-engineer-v12) lists every change with its proof |
| 1.1 | QA round 1 ([qa/F-08-fit-validation-and-recovery-r1.md](../../qa/F-08-fit-validation-and-recovery-r1.md)). **QA-08-r1-1:** sport hints from the `sport` message and `lap.sport`; one sport timeline replaces F-02's R-5 membership rule and is used by R-5 and R-7 (D-12). **QA-08-r1-2:** the record leg, the part of a truncated workout that no surviving session describes; R-10 combines over legs, not sessions (D-13). **QA-08-r1-3:** new R-12, distance continues across FIT sequences (D-14). **QA-08-r1-4:** only the first sequence's `file_id` can reject the file or name the device (D-15). **QA-08-r1-5:** normalisation stays outside the deadline, but has a complexity bound and is measured with every budget at its cap; new budget `sport_messages` (D-16, ADR-017 rev. 1). **Q2:** `multisport` only when the workout's sports differ (D-17, OQ-36). **Q1:** `ElapsedTime` of 2+ legs isolated as rule R-10.E, pending the user (OQ-35). New corpus files: `run-30min`, `run-30min-cut-60`, `triathlon-cut-90`, `chained-ride-ride`, `chained-ride-ride-cut-75`. Section [Changes for the backend engineer (v1.1)](#changes-for-the-backend-engineer-v11) lists every change with the test that proves it |
| 1.0 | First version |

## Superseded and struck-through text removed from the live document

Each item is the original text; the live document states the current form.

- **D-14 (v1.2 wording):** | D-14 | (v1.1) R-12: a later FIT sequence whose distance restarts is offset by the previous sequence's last distance, so the distance stream is cumulative over the whole workout. **v1.2:** "restarts" = its first valid distance is below **half** the previous last distance (midpoint rule; v1.1: any value below it) | section R-12 |
- **R-12 rule, restart line:** `    if prev exists and first exists and first < prev / 2: base = prev      (v1.2; v1.1: first < prev)`
- **R-12 case table, continuing-count row:** | Later sequence continues the count (`first ≥ prev / 2`), including a small drop at the boundary (QA-08-r2-1: 2,994.99 after 2,995) | Unchanged by R-12; the small drop is then held by R-14 (v1.4; v1.2: it stayed, as inside a sequence) |
- **R-13, Real-file check, point (3):** (3) the distance stream does not fall at any transition (v1.4: failed on device data, QA-08-r3-2; replaced by section R-14, Real-file check);
- **Hand-offs, F-17, F-18 (v1.1) row, struck sentence:** ~~Within one sequence, and at a sequence boundary where the writer carried its count on (v1.2), a device may still write a small decrease (R-6a); the browser must not assume strict monotonicity.~~ (v1.4: superseded by the F-17, F-18 (v1.4) row: the stream never decreases, of any size.)
- **R-14, Real-file check, end of point (c):** (c) no real golden changed outside the keys allowed in section [Changes for the backend engineer (v1.4)](#changes-for-the-backend-engineer-v14).
- **Tests, R-12 row, last sentence:** **v1.4:** the value cases call `continueDistance` directly, not `Normalise`, so R-14 does not hide R-12's output (v1.4 engineer row 6)

## Changes for the backend engineer (v1.6)

**Starting point.** v1.5 passed QA round 5. Built as the first rows of F-09's round 1 ([F-09 design](../F-09-upload-api-and-storage.md), section Changes for the backend engineer), and committed separately from the F-09 code if the main session wants a clean history.

| # | Finding | Change | Proven by |
| --- | --- | --- | --- |
| 1 | QA-08-r5-2 (user decision) | `tools/corpusgen/main.go`, `lapMesg`: a lap whose span equals a session span that has a `sessionDist` override gets the same override, applied exactly as `sessionMesg` applies it (`total_distance` = the override, `avg_speed` = override / timer when the timer is above 0). Only `swimBikeDistanceFall` sets `sessionDist`, so every other generated file stays byte-identical. Rename the field (for example `distOverride`) and its comment if the name no longer fits; the behaviour is what matters | `TestSyntheticCorpusCurrent`; the regenerated golden matches the corpus row (v1.6): lap and session distance and average speed equal, per leg |
| 2 | QA-08-r5-1 | `internal/workout/f08_v14_test.go:134-135`: the comment states the H3 mutant's output, `[100 100 125 150 175 200]` | Review |
| 3 | Corpus | `make corpus-gen`, `make corpus-update`, then review the diff | `make corpus`: every file ok, 0 diff; `make ci` |

**Goldens that may change (row 3), and only in these keys:** `synthetic/swim-bike-distance-fall.fit` (its lap messages); `expected/synthetic/swim-bike-distance-fall.json`: `laps[0].totals` and `laps[1].totals`, keys `distance` and `avg_speed` only (the swim lap's distance stays 599); `testdata/datafile/synthetic/swim-bike-distance-fall.bin`/`.json`: the same lap keys in data-file form. `summary`, `derived`, `sessions`, every stream and every other file stay byte-identical.

**Stop and report instead of deviating if:** any other `.fit`, report or data file changes; a key outside the list above changes; or the bike lap's distance is not 2,946.

## Changes for the backend engineer (v1.4 follow-up)

**Starting point.** v1.4 is committed (`bd50240`) and passed QA round 4. These are the round's Minor items (design v1.5). Work through the rows in order.

| # | Finding | Change | Proven by |
| --- | --- | --- | --- |
| 1 | QA-08-r4-1 (order) | No code change: `holdDistance` stays right after `fillGaps` (section R-14, Order against R-3). Add the R-14 order case: records 100, 50, 200 at 0, 1, 5 s through `Normalise` → exactly `[100 100 100 125 162.5 200]` | The test; mutant H3 (hold before `fillGaps`) is killed |
| 2 | QA-08-r4-1 (record leg) | The truncated record-leg case asserts the exact summary distance (`== 400`: session 300 plus leg 100), not `>= 300` | The test fails with `holdDistance` removed |
| 3 | QA r4 Q1 | `corpusgen`: new generator `swimBikeDistanceFall()` in `tools/corpusgen/f08.go`, registered like the others, as in its corpus row (section Corpus, v1.5). It needs explicit per-record distances; add that as an option used only by this file, so every other generated file stays byte-identical. README manifest row: "synthetic, no personal data" | `TestSyntheticCorpusCurrent`; the new golden report and data file match the row (review them by hand) |
| 4 | Corpus | `make corpus-update`, then review the diff | `make corpus`: 43 synthetic files ok (42 + 1) plus the local real files, 0 diff; `make ci` |

**Goldens that may change:** only the new file's report and data file pair. Every other `.fit`, report and data file stays byte-identical.

**Stop and report instead of deviating if:** any other synthetic file, golden or data file changes; the new golden contradicts its corpus row (in particular a distance other than 199 at offsets 200–219 or 599 at 600–609, or `derived.distance` ≠ `summary.distance`); or removing `holdDistance` does not make `TestCorpus` fail on the new file.

## Changes for the backend engineer (v1.4)

**Starting point.** v1.3 is committed (`27ea5af`) and passed QA round 3. The round's backend findings (QA-08-r3-1, r3-3, r3-4) are worked on separately; do them first, or keep the two change sets apart, as the main session decides. Then QA checks both in the next round. Work through these rows in order; the rule is in section R-14.

| # | Finding | Change | Proven by |
| --- | --- | --- | --- |
| 1 | QA-08-r3-2 (D-25) | `distance.go`: a function for R-14 (for example `holdDistance(s *Samples)`) over `s.Distance`, as in section R-14. `normalise.go`: call it right after `fillGaps`, before the sport timeline and `maskTransitions`; update the `Normalise` comment to "R-1 to R-14". Comment names R-14, D-25 and QA-08-r3-2 | R-14 cases (section Tests, `internal/workout`) |
| 2 | R-14 invariant | `ingest` `checkInvariants`: the distance invariant (section Tests, `TestCorpus`) | Passes on every synthetic file in `make ci`; the operator's local run, row 5 |
| 3 | R-14 through `Parse` | `TestChained` midpoint case: the step check becomes "no negative step" (section Tests). This test is also part of QA-08-r3-1 | The test itself |
| 4 | Corpus | `make corpus-update`, then review the diff | `make corpus`: every synthetic file ok, **0 synthetic diff**; `make ci` |
| 5 | Real files | Operator: `make corpus-update` and `make corpus` locally, review the real golden diffs, then section R-14, Real-file check (a) to (c), reported by manufacturer only and into the QA report, never into `architect/` | The operator's result in the hand-off |
| 6 | Engineer stop (v1.4): `TestContinueDistance` runs its R-12 cases through `Normalise`, so 5 cases now see R-14's held values | **Option (a):** the R-12 cases call `continueDistance` directly (the records in arrival order, before R-2), and keep their R-12 expected values unchanged. Do **not** rewrite them to held values: R-12 is unchanged, and its test must isolate it. The held result through `Normalise` is already covered by the R-14 row (section Tests), including the "continuing count 0.01 m below `prev`" case; keep the purity case (input `Recording` unchanged) through `Normalise` | `TestContinueDistance` green with its pre-v1.4 expected values; R-14 cases green |

No `MEASUREMENTS.md` row is needed: R-14 is one O(n) pass, well inside D-16.

**Goldens that may change (row 4), and only in these keys:**

| File | Allowed to change |
| --- | --- |
| Every synthetic report and data file | **Nothing.** `corpusgen` integrates every device distance from a non-negative speed, so each sequence is already non-decreasing, and R-12 continues restarted sequences |
| Local real reports and data files whose device distance falls anywhere | `streams.distance`; `derived.distance` and `derived.avg_speed`; `derived.moving_time` and `summary.moving_time` only when `moving_time_source` is `speed` and the file has no speed stream; `summary.distance` and `summary.avg_speed` only where they come from step 2 or include a record leg; the same values in data-file form |
| Other local real files | Nothing |

**Stop and report instead of deviating if:** any synthetic golden or data file changes; a real golden changes outside the allowed keys; a unit test outside the R-14 and `TestChained` rows above has to change its expected values (name it and say why).

## Changes for the backend engineer (v1.3)

**Starting point.** The v1.2 changes are built in the working tree but **not committed and not yet through QA** (the last commit is v1.1, QA r2). v1.3 builds on it: row 1 replaces v1.2's `legsElapsed` (`summary.go:58`) and edits the tests v1.2 added for it, and row 8's allowed golden diff is measured against the v1.2 goldens (the triathlon summaries already hold the v1.2 wall times 3,719 and 3,359). Keep the v1.2 working-tree changes (code, tests and the six goldens of v1.2 row 7); do not reset them. Before row 1, run `make ci` and `make corpus` once to confirm v1.2 is green; if not, stop and report. QA round 3 then covers v1.2 and v1.3 together.

Work through these rows in order. The rules are in the sections named; do not re-derive them from this table.

| # | Finding | Change | Proven by |
| --- | --- | --- | --- |
| 1 | Engineer report on D-20 (D-18 revised) | `sequences.go`: `recordSequences(rec Recording) int` (section Summary, R-10.E). `summary.go`: `legsElapsed` is replaced by R-10.E as restated: `summarise` gets `singleRecording` (computed in `Normalise`: one session, no record leg, `recordSequences(rec) <= 1`) and applies the rule for every leg count; step 2 unchanged. Comment names D-18, D-20, OQ-35. Tests: v1.2's `TestLegsElapsed` (`internal/workout/f08_v11_test.go`) moves to the new function (its "returned pointer is not Derived's" case stays, against the new code), and the "not asserted here" comment in `TestRecordLegCompleteChained` is replaced by the assertion | `TestRecordLegCompleteChained` asserts summary elapsed time 109 (today 9); the R-10.E v1.3 cases and `recordSequences` cases (section Tests, `internal/workout`). **No golden changes** (every synthetic file with records from 2 sequences already has 2+ legs) |
| 2 | OQ-38 (D-21, D-23) | `sports.go`: `inTransition(t)`, true only when rule 1 (the precomputed `cover`) gives a session with sport `transition`; hints and fallbacks never. New `transition.go`: the allow-list (section R-13: speed, cadence, power dropped; temperature kept) as one table over streams and `Totals` fields, and `maskTransitions(w *Workout, tl *sportTimeline)` for samples, transition sessions and transition laps | R-13 tests (section Tests), including allow-list completeness and the "which sessions" cases (hint, rule 3, sub-sport only → not masked) |
| 3 | OQ-38 (D-22) | `normalise.go`: build `tl` right after `fillGaps`, call `maskTransitions`, then `settlePresence`; R-5 onward unchanged and use the same `tl` | R-13 tests: filled sample masked, stream with values only in a transition becomes `nil`; existing `TestSampleCount`, sport timeline equivalence test unchanged |
| 4 | OQ-38b answered (R-13.M) | `moving.go`: the transition intervals `(Offset[i−1], Offset[i]]` for every `i ≥ 1` with `inTransition(Offset[i])` and `Δt < 5` always count as moving. Speed branch: such a sample adds `Δt` without the threshold. Timer branch: moving time = the length of the union of the timer-running intervals and the transition intervals (sort and merge; O(n + e log e)). Comment names R-13.M and the user's answer 2 | R-13.M cases. No existing golden changes from this row (the timer runs through both triathlon transitions) |
| 5 | Fixture (section Corpus, `ride-transition-ride.fit` row) | `corpusgen`: new generator `rideTransitionRide()` in `tools/corpusgen/f08.go`, registered like the other files; transition constants as in the row (the route advances at 1.5 m/s also at the 12 m/s sample). README manifest row: "synthetic, no personal data". `triathlon()` stays unchanged: adding streams to it would change its length (the generator omits missing fields) and move the cut of `triathlon-cut-90` | `TestSyntheticCorpusCurrent`; its new golden report and data file match the row's expectations (review them by hand), AC 8 and AC 9 |
| 6 | Real-file check (section R-13) | `ingest` `checkInvariants`: the transition invariant (section Tests, `TestCorpus`) | Passes on all synthetic files in `make ci`; operator's local run, row 9 |
| 7 | D-23 through truncation | `TestTruncationPrefix`: `ride-transition-ride.fit` as a fourth base; prefix equality except speed, cadence and power in a transition whose session the cut lost, which keep their recorded values; coverage check for a cut ending inside the transition (section Tests) | The test itself; it fails if no cut ends inside the transition |
| 8 | Corpus | `make corpus-update`, then review the diff | `make corpus`: 42 synthetic files ok (41 + 1) plus the local real files, 0 diff; `make ci` |
| 9 | Real files | Operator: `make corpus-update` and `make corpus` locally, then the five points of section R-13, Real-file check, reported by manufacturer only and into the QA report, never into `architect/` | Operator's result in the hand-off |
| 10 | D-16 | Re-run `BenchmarkParseAtCap` (shape unchanged) and add a `MEASUREMENTS.md` row; R-13 adds one timeline lookup per sample | Row present, `Normalise` within the ADR-017 stop rule |

**Goldens that may change (row 8), and only in these keys:**

| File | Allowed to change |
| --- | --- |
| `synthetic/ride-transition-ride.fit`, its golden report and its data file pair | New |
| `expected/synthetic/triathlon.json`, `expected/synthetic/triathlon-cut-90.json` | The `speed` stream entry (missing at T1 and T2 samples); `avg_speed` and `max_speed` removed from the totals of the two transition sessions and the two transition laps. Nothing else |
| `testdata/datafile/synthetic/triathlon.bin`/`.json`, `…/triathlon-cut-90.bin`/`.json` | The same changes in data file form |

Every `.fit` file other than the new one stays byte-identical. In the two triathlon reports, `summary`, `derived`, `sport`, `integrity`, `samples`, every other stream entry and the swim, bike and run session and lap totals stay byte-identical.

**Stop and report instead of deviating if:** any other golden (synthetic report or data file) changes; a key outside the allowed list changes in the two triathlon reports; rows 1 or 4 change an existing golden; the new file's report contradicts its corpus row (in particular a speed, cadence or power maximum or average that includes a transition constant, a maximum temperature other than 30, `derived.moving_time` other than 1,259, or a sport other than `multisport`); `TestTruncationPrefix` finds no cut ending inside a transition; a real golden changes for a file without a transition session; or `Normalise` at the caps breaks the ADR-017 stop rule.

## Changes for the backend engineer (v1.2)

The v1.1 changes are built and passed QA round 2. Work through these rows in order. The rules are in the sections named; do not re-derive them from this table.

| # | Finding | Change | Proven by |
| --- | --- | --- | --- |
| 1 | OQ-35 (D-18) | `legsElapsed` (`summary.go`) returns a copy of `d.ElapsedTime` instead of the sum of the legs. The comment says: R-10.E, wall time from the workout start to the last record, OQ-35 answered by the user on 15 Oct 2026. Remove "pending" and "in force until the user answers" | `TestLegsElapsed` rewritten to the R-10.E cases in section Tests (`internal/workout`). Goldens: the six reports in section Summary, R-10.E change in `summary.elapsed_time` only, each to its `derived.elapsed_time` (3,660 / 2,998 / 1,498 / 1,199 / 3,719 / 3,359); their golden data files under `testdata/datafile/synthetic/` change in the summary only |
| 2 | OQ-36 (D-17), QA-08-r2-3 (code part) | No behaviour change. `workoutSport` comment (`timezone.go`): D-17 confirmed by the user on 15 Oct 2026. Remove "the user is asked to confirm" and the sentence about "the only function to change" | Review only; `TestWorkoutSport` and the `chained-ride-ride` golden unchanged |
| 3 | QA-08-r2-1, QA r2 Q2 (D-14 v1.2) | `continueDistance` (`sequences.go`): the restart test becomes `first < prev/2`, and the comment explains the midpoint rule. Nothing else in R-12 changes | `TestContinueDistance` v1.2 cases (section Tests, R-12); `TestChained` v1.2 case through `Parse` (QA's probe shape, which gives 8,984.99 m today). `chained-ride-run`, `chained-ride-ride` and `chained-ride-ride-cut-75` restart at 0, so their distances do not change |
| 4 | QA r2 Q1 (D-20) | No code change. A test pins the accepted behaviour | "Record leg, complete chained" case (section Tests, `internal/workout`) |
| 5 | QA-08-r2-2 | `TestParseLogs`: add the `samples` limit case (section Tests, `TestParseLogs`). `TestLimitsAtTheCap`: delete the unused `skipOverTest` field | `TestParseLogs` passes with the new case; `go vet` clean |
| 6 | QA-08-r2-3 (doc part) | None for the engineer: F-02 v1.9 marks its `multisport` boundary sentence (architect) | — |
| 7 | Corpus | `make corpus-update`, then review the diff. Only rows 1's six reports and six data files may change, and only in `summary.elapsed_time`. No synthetic `.fit` changes, and `corpusgen` is untouched. The operator runs `make corpus` on the real files: they have no file with 2+ legs (QA r2), so their goldens must not change | `make corpus`: 45 ok, 0 diff; `TestSyntheticCorpusCurrent`; `make ci` |

Stop and report instead of deviating if: any golden key other than `summary.elapsed_time` changes; any of the six new values differs from its `derived.elapsed_time`; or a real golden changes.

## Changes for the backend engineer (v1.1)

Work through the QA round 1 findings in this order. Each row says what to change and which test or fixture proves it. Rules are in the sections named; do not re-derive them from this table.

| # | Finding | Change | Proven by |
| --- | --- | --- | --- |
| 1 | QA-08-r1-4 | `listener.fileID`: decide by the sequence number passed to `beginSequence(n)`, not by `seenFileID`. Only sequence 1 sets `FileType`/`Device` and can reject; sequence n ≥ 2 only sets `skip` (section Listener) | `TestChained` v1.1 cases (fail today) |
| 2 | QA-08-r1-3 | `ingest`: `beginSequence(n ≥ 2)` appends `len(rec.Records)` to `rec.SequenceStarts`. `workout`: `Recording.SequenceStarts`; R-12 in `sequences.go`, applied to `buildTimeAxis`'s copy of the records before sorting, never to the caller's slice | R-12 unit tests; `TestChained` (`SequenceStarts`); `chained-ride-ride.fit` golden; `chained-ride-run.json` golden changes in `derived.distance`, `derived.avg_speed` and the distance stream only (review the diff) |
| 3 | QA-08-r1-1 | `ingest`: read `sport` messages into `rec.Sports` (pending until the next record of the same sequence) and `lap.sport` into `RawLap.Sport`; budget `MaxSportMessages` (`limits.go`, name `sport_messages`). `workout`: `sports.go` with hints, `sportAt` per the rule table, computed in one sweep; `convertCadence` and `movingTime` use it; `sessionAt`'s per-sample scan is removed | `TestListenerSport`; sport timeline tests incl. the equivalence test; `TestTruncationPrefix` on `run-30min` and `triathlon` (fails today); `run-30min-cut-60.fit` golden (`sport: running`, cadence about 171); `TestLimits` sport-message flood |
| 4 | Q2 / OQ-36 | `workoutSport` becomes R-9 sport over `S` (section Workout sports `S`). The mixed test in `summarise` uses the same `S` | R-9 tests; `chained-ride-ride.fit` golden (`sport: cycling`); `triathlon.fit` golden unchanged |
| 5 | QA-08-r1-2 | `legs.go`: legs and the record leg (step 0); `summarise` combines over legs; R-10.E in its own function, commented "pending OQ-35" | Record-leg tests; R-10.E test; `triathlon-cut-90.fit` and `chained-ride-ride-cut-75.fit` goldens; `ride-30min-cut-60.json` unchanged |
| 6 | QA-08-r1-5 | All-caps file in `corpusgen -cap-file` and `BenchmarkParseAtCap`; decode and `Normalise` timed separately; new `MEASUREMENTS.md` row (development machine, Go version as `go version` prints it). Stop rule unchanged | `MEASUREMENTS.md` row ≤ 2 s and ≤ 400 MB max RSS |
| 7 | Corpus | `corpusgen`: `sportHints` flag; `run-30min`, `run-30min-cut-60`, `triathlon-cut-90`, `chained-ride-ride`, `chained-ride-ride-cut-75`; README manifest rows ("synthetic, no personal data" / "derived from synthetic `<base>`"); `make corpus-update`, then review every changed golden: only `chained-ride-run` (R-12) and the new files may differ; `triathlon` must be byte-identical in its report | `make corpus`: 41 synthetic files (36 + 5), 0 diff; `TestSyntheticCorpusCurrent` |
| 8 | QA-08-r1-6, QA-08-r1-7 | No design change. Fix as QA describes: `TestSampleCount` over every corpus file, `TestLimitsAtTheCap` for every budget it sets, `TestParseLogs` for the time limit, every budget and the panic line's position scan, `stdout.Write` error checked, "provisional" comment removed, toolchain version in `MEASUREMENTS.md` corrected | QA round 2 |
| 9 | Real files | Operator runs `make corpus-update` and `make corpus` locally (ADR-004 item 7). Expected: all 4 real files ok; their sport unchanged (complete files: sessions win), their goldens unchanged unless a file has samples outside its sessions and a hint (report the count by manufacturer only) | Operator's result in the hand-off |

Stop and report instead of deviating if: a real or synthetic complete file changes sport or summary through the hints; the all-caps file does not fit in `MaxBytes` even without timer events; or the equivalence test cannot be met with the sweep.

## Resolved questions

v1.4:

| ID | Question | Decision | Blocks | Owner |
| --- | --- | --- | --- | --- |
| OQ-39 | (v1.3, answer 9, D-23) A file cut **inside** a transition loses that transition's session. Its tail is then covered by no session, so rule 1 cannot name it a transition, and its speed, cadence and power stay, although its `sport` hint says `transition` (section R-13, Interactions, sport timeline row (b)). Accept this edge, or let a `transition` hint mask the uncovered tail of a truncated file? | **Accepted by the user**, 15 Oct 2026: "accept" ("damaged files are a bonus not a must-have"). Behaviour as built: answer 9 applied literally, only a session makes a transition; `TestTruncationPrefix` pins it. Trigger to revisit: a real truncated file whose transition tail shows values the user calls wrong | Nothing; no code change | user |

v1.3:

| ID | Question | Decision | Blocks | Owner |
| --- | --- | --- | --- | --- |
| OQ-37 | Anchor of the multi-leg elapsed time: the first record (AP-2's old wording) or the workout start (AP-5)? | **The workout start**, decided by the user's OQ-35 answer ("elapsed time should be tracked across the entire workout"); AP-2 now says "from the workout start to the last record". As built (D-18) | Nothing | user (via OQ-35), requirements |
| OQ-38 | Transition legs with records | **Keep only position, heart rate and elevation** (user, 15 Oct 2026; AP-4, AP-12): R-13. The details were answered too (OQ-38a to OQ-38d below; answers 1 to 9 in section R-13): distance and temperature are kept as well, and only a session with sport `transition` is a transition | Nothing | user |
| OQ-38a | Distance in transitions: kept in the distance stream, the distance axis and the total distance? | **Kept everywhere** (answer 1, "distance matters"): allow-list, R-10 sums include transition legs | Nothing | user |
| OQ-38b | Moving time in transitions: does a transition count as moving? | **Yes, the whole transition** (answer 2, "triathlon counts the total time as result time"): R-13.M in both moving-time sources | Nothing | user |
| OQ-38c | Temperature in transitions: kept or dropped? | **Kept** (answer 3): `Temperature`, `AvgTemperature`, `MaxTemperature` on the allow-list | Nothing | user |
| OQ-38d | Ranges and km splits that span a transition (F-17, F-18): special treatment? | **None** (answer 5): distance and time include the transition; speed, cadence and power have no transition samples, so averages over present samples leave them out by themselves; moving time counts the transition as moving (hand-offs F-17, F-18) | Nothing in F-08 | user |

OQ-35 and OQ-36 were answered by the user on 15 Oct 2026 (v1.2):

| ID | Question | Decision | Blocks | Owner |
| --- | --- | --- | --- | --- |
| OQ-35 | AP-2 "elapsed time" of a workout with 2+ legs: the sum of the legs' elapsed times, or the wall time from the start to the last record? | **Wall time** (D-18). User: "I swam for 1 hour then rode a bike for 1 hour, what elapsed time should be shown when hovering over the 30 minute mark of the ride? 90 minutes, elapsed time should be tracked across the entire workout". The time axis already does this (D-19) | Nothing | user |
| OQ-36 | Should a file with 2+ sessions of one sport show its sport or "Multisport"? | **Its sport**: D-17 confirmed as built; AP-12 reworded by requirements | Nothing | user |

The next five were accepted as recommended by the user on 15 Oct 2026 ("accept"):

| ID | Question | Decision (accepted) | Blocks | Owner |
| --- | --- | --- | --- | --- |
| OQ-30 | Wording of the messages the spec does not fix (decision 20) | "This file is not a FIT file." / "This file contains no readable workout data." / "This file is too large or too complex to read." / "This file could not be read." / "The upload was interrupted." "Unsupported FIT file type." is fixed by UP-2 | Nothing; wording changes the rejection reports only | user |
| OQ-31 | Longest workout accepted: 500,000 samples (about 138 h at 1 Hz, or a smart-recording file of the same duration) | Applied (ADR-017) | Nothing; a change is an ADR-017 revision, bounded by the memory stop rule | user |
| OQ-32 | Multi-session summary: average speed and cadence hidden when sports differ; average heart rate, power and temperature over the whole workout from the records | Applied (R-10 revised) | Nothing; F-13 shows what is present | user |
| OQ-33 | A file with only a CRC failure (data complete) shows no notice | Applied: no notice, logged only | Nothing; F-19 would need a second notice text if the user wants one | user |
| OQ-34 | A truncated file usually loses its `activity` message, so its start time shows in UTC, labelled (decision 13); there is no other offline source for the local time zone | Applied | Nothing | user (information) |
