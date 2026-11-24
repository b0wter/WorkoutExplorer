# ADR-047 Trim: history

History of [architect/ADR-047-trim-cut-of-the-normalised-workout.md](../ADR-047-trim-cut-of-the-normalised-workout.md). Read it only when you need the history.

## 24 Nov 2026: ADR-061 (CR-02)

- Item 3 read: "`GET` and `PATCH` parse `original.fit` under the O-7 lock and run the public transform's code …".
- Consequence read: "Cost: a parse per owner read of a trimmed workout, serialised with owner writes on the O-7 lock."
- Would choose differently read: "… or owners queueing on O-7". That trigger fired (CR-02). ADR-061 moved the view off O-7 instead of storing it.
