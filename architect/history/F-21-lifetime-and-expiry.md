# F-21 Lifetime and automatic expiry: history

History of [architect/F-21-lifetime-and-expiry.md](../F-21-lifetime-and-expiry.md).

## v0.1 to v0.2 (31 Oct 2026, QA-21-r1)

v0.1 status: "draft (31 Oct 2026)". Builds on: "ADR-019 rev. 2". Changed rows as they read in v0.1:

- **X-3:** "Every owner write runs the lookup under the O-7 lock, and the expiry pass deletes under the same lock, so no request can revive a due row (lengthen its lifetime, publish it). A clock stepped backwards can delay a delete; it cannot resurrect content already removed. Accepted." Added: the `Live` re-check inside the update (QA-21-r1-2).
- **X-4:** steps (1) to (3) only; nothing said about step 3 when step 1 fails. Added: the sweep runs anyway (QA-21-r1-4), step 4 checkpoint (QA-21-r1-1).
- **X-5:** ended "The pass never stops the timer on error." Added: a failing startup pass is logged and `serve` starts anyway (engineer's deviation, QA-21-r1-5).
- **X-9:** ended "F-27 (`reports`) and later pins must add theirs." Added: the byte scan and its accepted limit (QA-21-r1-1).
- **Verification X-9 row:** "Adding an unclassified table or data-directory entry fails the check". Mutants row extended.
- **Hand-off F-27:** "`workout-explorer delete` goes through `Delete`, so it takes the ADR-031 lock. Add `reports` to the X-9 check".
