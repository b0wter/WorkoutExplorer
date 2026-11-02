# F-01 Project skeleton and CI: history

History of [architect/F-01-project-skeleton-and-ci.md](../F-01-project-skeleton-and-ci.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed).

## Change log

| Version | Change |
| --- | --- |
| 1.3 | QA round 2 follow-up ([qa/F-01-project-skeleton-and-ci-r2.md](../../qa/F-01-project-skeleton-and-ci-r2.md)). Empty address: the Makefile treats an empty `DEV_ADDR` as unset (same as Vite), and `serve` rejects an empty `-addr` with exit 2 (QA-01-r2-2). `main.go` may change: one validation line plus the `defaultAddr` comment, with the wording fixed here (QA-01-r2-1). |
| 1.2 | QA round 1 follow-up ([qa/F-01-project-skeleton-and-ci-r1.md](../../qa/F-01-project-skeleton-and-ci-r1.md)). D-4 revised: dev address set once through `DEV_ADDR` (QA-01-r1-5). Accepted deviations recorded: actions `@v7`, TypeScript held below 7, `check` fails on warnings (D-8) (QA-01-r1-6). README scope tightened (QA-01-r1-4). Test list extended with `main_test.go` and size-script `main` tests (QA-01-r1-1, r1-3). QA questions 1 and 2 settled (user, 10 Oct 2026): `cancel-in-progress` stays; no size line when there is no bundle. |
| 1.1 | CI host decided as GitHub Actions, workflow specified; OQ-1 and OQ-2 closed. |
| 1.0 | First version. |

## Open questions closed on 10 Oct 2026

| ID | Resolution |
| --- | --- |
| OQ-1 | CI host is GitHub Actions (user). Specified in section CI. |
| OQ-2 | Feature file fixed by the requirements engineer: Scope now points to ADR-001 for the layout and names GitHub Actions. No deviation from the feature file remains. |
| QA r1 Q1 | `cancel-in-progress: true` kept (user). Section CI, Result guarantee. |
| QA r1 Q2 | No size line when there is no bundle is acceptable (user). Section Acceptance criteria to verification, criterion 4. |
| QA r2 Q1 | Empty `DEV_ADDR` is the default in the Makefile and Vite; empty `serve -addr` exits 2. Sections Server (exit codes) and Local development address. |
