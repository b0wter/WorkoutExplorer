# F-18 Laps and splits table: history

History of [architect/F-18-laps-and-splits-table.md](../F-18-laps-and-splits-table.md). Read only when you need it.

## Change log

- **0.2 (29 Oct 2026):** OQ-75 to OQ-80 decided by the user as suggested. L-7 (tapped row kept in place) is decided, not proposed. L-1 does not refill transition-lap speed, cadence or power. New L-8: laps module without device laps (B-1 scope gains the view-model change and a sample). L-3 and L-7 note OQ-79 and OQ-80. Status accepted.
- **0.1 (29 Oct 2026):** first draft (commit 65da757).

## Decided questions

| ID | Question | Decision (user, 29 Oct 2026) |
| --- | --- | --- |
| OQ-75 | Native scroll anchoring cannot keep a tapped row in place when the panel's slot is on screen (and maybe not at all in Safari). Build L-7? | Yes, in all layouts |
| OQ-76 | Remember the laps/splits choice in `lib/prefs`? | No: memory only, every page starts on laps |
| OQ-77 | A file with distance but no device laps has no laps module (F-13 D-5), so no splits either | The server renders the module when there are laps or distance; without laps it says "No laps recorded." and the switch offers splits |
| OQ-78 | A transition lap has its speed, cadence and power removed (F-08 R-13); read literally, AP-8 computes its speed | Not refilled: "–", as in the summary; splits that cross a transition count it as a range does |
| OQ-79 | After a tap on a device lap, the panel's sample values may differ from the row's device values | Accepted, no note (as OQ-24) |
| OQ-80 | On the phone, a lap tap while reading the laps leaves the panel above the screen | Accepted (AP-7 placement) |
