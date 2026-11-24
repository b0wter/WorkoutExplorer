# F-21 Lifetime and automatic expiry: history

History of [architect/F-21-lifetime-and-expiry.md](../F-21-lifetime-and-expiry.md).

## 24 Nov 2026: ADR-061 (CR-02)

- **X-3** began: "Every owner write runs the lookup under the O-7 lock, and the expiry pass deletes under the same lock, so no request can revive a due row (lengthen its lifetime, publish it). Every owner update also re-checks `Live(now)` …".
- **X-4** steps (2) and (3): "each under the O-7 lock" and "the P-8 public sweep under the O-7 lock".
- **X-6** read: "Deadline → files gone ≤ 30 s (period) + one owner build waiting on O-7 (≈ T + one compression, T = `WORKOUT_EXPLORER_PARSE_TIMEOUT`) + one other process's write phase waiting on the public lock (ADR-031). At the default T = 10 s this is about 45 s. With T above about 25 s, a save of a slow file that overlaps a deadline can exceed 60 s. Accepted and recorded; the operator setting is the lever." CR-02 showed it assumed one build on O-7; any number could queue.

## v0.1 to v0.2 (31 Oct 2026, QA-21-r1)

v0.1 status: "draft (31 Oct 2026)". Builds on: "ADR-019 rev. 2". Changed rows as they read in v0.1:

- **X-3:** "Every owner write runs the lookup under the O-7 lock, and the expiry pass deletes under the same lock, so no request can revive a due row (lengthen its lifetime, publish it). A clock stepped backwards can delay a delete; it cannot resurrect content already removed. Accepted." Added: the `Live` re-check inside the update (QA-21-r1-2).
- **X-4:** steps (1) to (3) only; nothing said about step 3 when step 1 fails. Added: the sweep runs anyway (QA-21-r1-4), step 4 checkpoint (QA-21-r1-1).
- **X-5:** ended "The pass never stops the timer on error." Added: a failing startup pass is logged and `serve` starts anyway (engineer's deviation, QA-21-r1-5).
- **X-9:** ended "F-27 (`reports`) and later pins must add theirs." Added: the byte scan and its accepted limit (QA-21-r1-1).
- **Verification X-9 row:** "Adding an unclassified table or data-directory entry fails the check". Mutants row extended.
- **Hand-off F-27:** "`workout-explorer delete` goes through `Delete`, so it takes the ADR-031 lock. Add `reports` to the X-9 check".
