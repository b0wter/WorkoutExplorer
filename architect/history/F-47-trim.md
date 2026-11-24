# F-47 Trim start and end: history

History of [architect/F-47-trim.md](../F-47-trim.md). Read it only when you need the history.

## 24 Nov 2026: ADR-061 (CR-02)

- **TR-8** read: "With a trim stored, `GET` and `PATCH` compute, under the O-7 lock from one parse of `original.fit`: …".
- **TR-10** read: "The owner-view parse runs under the O-7 lock, which already admits one owner-side build at a time and takes no extra original while waiting. Parses still go through the parser's slots, so ADR-017's M(N) is unchanged." CR-02: unthrottled owner `GET`s queued on O-7 ahead of expiry and deletes.
