# F-17 Range selection and x-axis toggle: history

History of [architect/F-17-range-selection-and-axis-toggle.md](../F-17-range-selection-and-axis-toggle.md). Read it only when you need the history.

## 0.3 (29 Oct 2026, accepted; F-17 QA r1)

- QA-17-r1-1: ADR-029 rev. 1 anchors `M[0] = 0`; B-1's invariants run on every corpus file. AC 9 unchanged.
- QA-17-r1-2: the stored split is restored before the first `await`; the residual shift is recorded in F-13 D-10.
- QA-17-r1-3: Budget "Ledger" row as written: "Entry 50,220 (F-16 QA r1). At the share it becomes 54,220, leaving 1,500 for F-18 and a reserve of 4,280". ADR-011 rev. 9.
- ADR-029 item 1 as written: "`M[i]` is R-7's moving time, R-13.M included, within `[0, Offset[i]]`. It never decreases, and `M[N−1]` equals the derived moving time."

## 0.2 (26 Oct 2026, accepted)

User decisions of 26 Oct 2026 on OQ-72 to OQ-74:

- OQ-72: in the one-column layout the range panel sticks to the bottom of the screen while a range exists; in two columns it stays in place (R-9).
- OQ-73: as suggested, only a tap on a chart outside the shaded part clears; map taps, other taps and scrolls never do (gesture table unchanged).
- OQ-74, **changed from the suggestion**: the panel is hidden on every device until a range exists, with no reserved height. The jump of content below the charts when it appears is accepted. A one-line hint under the charts says how to select (R-9).

Superseded text of v0.1:

- R-9: "Whenever the page has charts, the slot has a fixed reserved height per breakpoint. This replaces F-13's 'zero height until filled'. [...] With no range, the values show '–' and a one-line hint shows how to select [...]. The clear control is in the panel. Its space is reserved, and it is visible only while a range exists." Reason given: a panel that appears on the first range would move the laps under the finger when F-18 rows select a range, and would break CLS 0 when in view at load.
- Requirements hand-off: "F-13 hand-off: the range slot is reserved whenever there are charts" (dropped; F-13's "zero height until filled" stands).
- Budget remedy: "with the hint as static text" (the hint is now in the charts module).
- Open questions OQ-72 to OQ-74 with their suggestions (now in `history/INDEX.md`).

ADR-029 accepted with this version; ADR-005 item 9 notes version 2.

## 0.1 (26 Oct 2026, draft)

First design (commit 62b2853).
