# F-04 Workout data file reader: history

History of [architect/F-04-workout-data-file-reader.md](../F-04-workout-data-file-reader.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Change log

| Version | Change |
| --- | --- |
| 1.1 | QA round 1 ([qa/F-04-workout-data-file-reader-r1.md](../../qa/F-04-workout-data-file-reader-r1.md)). r1-2: the Worker trigger is the **median cold decode over 5 page loads** (ADR-007 rev. 1); the bench page keeps cold runs across reloads and shows their median. r1-3: OQ-9 closed; wording for "missing sample" vs "gap" given to requirements. OQ-8 closed. Q1: real-file AC 2 check stays a manual QA run, with a trigger to automate. Q2: the operator's phone run also counts as the bench page's browser check |
| 1.0 | First version |

## Open questions as they stood in v1.1

None block implementation.

| ID | Question | Blocks | Owner |
| --- | --- | --- | --- |
| ~~OQ-9~~ | **Closed (v1.1).** Requirements agreed with ADR-007; the feature goal now says raw columns are "copied at most once". The remaining "gap" wording in the goal, scope and AC 1 is corrected as below (QA-04-r1-3) | — | — |
| OQ-10 | What the viewer sees when the data file cannot be loaded or decoded. The spec is silent. Proposal: the summary and laps (already in the HTML) stay; the charts area shows "The workout data could not be loaded." with no technical detail; `unsupported-version` adds "Reload the page." | F-07 / F-13 wording only, not F-04 | user (product), via F-07 |
| OQ-11 | **Closed 2026-10-12** (see `INDEX.md`): iPhone 13 mini, Orion (WebKit), cold median 31 ms, `ok: true`, 10 streams; under 50 ms, so no Worker. Was: AC 5 needs the operator's phone; the same run is the bench page's browser check (QA r1 Q2) | Nothing | user (operator) |

OQ-10 was decided by the user on 15 Oct 2026 as proposed (decision log of `features/README.md`).

**Feature file wording for requirements (QA-04-r1-3, v1.1; applied by requirements).** Glossary: a *missing sample* is `NaN`, a *gap* is two samples 5 s or more apart. Replace:

| Where | Current | Replace with |
| --- | --- | --- |
| Goal | "… copied at most once, so missing samples become gaps (NaN)." | "… copied at most once, and missing samples become `NaN`." |
| Scope, bullet 3 | "Turn sentinels into gaps (values the charts and map treat as missing, never as numbers)." | "Turn sentinels into missing samples (`NaN`): values the charts and map treat as missing, never as numbers." |
| Scope, new bullet after it | — | "Report time gaps (two consecutive samples 5 s or more apart) as sample indices; the values themselves are not changed." (describes D-4, already built) |
| AC 1 | "(e.g. 255 in a uint8 heart-rate column becomes a gap)" | "(e.g. 255 in a uint8 heart-rate column becomes a missing sample, `NaN`)" |
