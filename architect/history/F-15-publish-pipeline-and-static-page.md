# F-15 Publish pipeline and static page: history

History of [architect/F-15-publish-pipeline-and-static-page.md](../F-15-publish-pipeline-and-static-page.md). Read it only when you need the history.

## 24 Nov 2026: ADR-061 (CR-02)

- F-21 hand-off read: "Run P-8's public sweep every minute with the expiry deletes, under the O-7 lock".
- P-8 ended: "The same sweep runs under the O-7 lock when F-21 schedules it".

## Version 1.1 (30 Oct 2026, F-20 design v0.2): private state removed (SH-2)

- P-7 allow-list read "`visibility` from draft or private to unlisted", and the decision continued: "When the visibility after the change is private, the folder is only removed (`index.html` first)." Why: "A change to private takes effect within the request (SH-10)".
- P-8 sweep list read "folders with no row, draft, private and expired rows".
- Verification, AC 1, 5, 6 read: "private removes the folder; switching back to unlisted restores the same names."
