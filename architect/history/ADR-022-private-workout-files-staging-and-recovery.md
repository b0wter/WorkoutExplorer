# ADR-022 Private workout files, staging and recovery: history

History of [architect/ADR-022-private-workout-files-staging-and-recovery.md](../ADR-022-private-workout-files-staging-and-recovery.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 2)

- **Date:** 16 Oct 2026 (revision 1, 16 Oct 2026, UP-9: `original.fit` is the stripped file from `ingest.Accept` (ADR-023), never the upload's bytes; item 1). Revision 2, 16 Oct 2026, F-09 v1.4 (QA-09-r1-1, QA-09-r1-5): the insert cannot be cancelled by the client, and a folder is removed after a failed insert only once the row is known to be absent (item 4); recovery also deletes a row whose folder is missing (item 5); unknown entries are counted in the INFO line, with no WARN (item 5)
