# ADR-005 Workout data file format, version 1: history

History of [architect/ADR-005-workout-data-file-format-v1.md](../ADR-005-workout-data-file-format-v1.md), moved out of the live ADR on 22 Oct 2026 (restructuring, no decision changed).

## Header date line as written (revision 1)

- **Date:** 10 Oct 2026 (revision 1, 22 Oct 2026, F-09 v1.10, OQ-55 decided by the user: a workout kept after a failed rebuild stays on its previous data file and bundle until a later run rebuilds it, and an old bundle is deleted only once no kept workout refers to it; each bundle still reads exactly one version (item 9). No format change)
