# QA F-23 Preview card, Open Graph and indexing - round 3

- **Date:** 2026-10-31
- **Verdict:** ready
- **Inputs:** qa/F-23-preview-card-and-indexing-r2.md; diff of 05c99c4 (`page.go`, `page_test.go`, `preview_test.go`, `architect/F-23-preview-card-and-indexing.md`)

## Build and test results
`cd src/server && go test ./...`: all 15 packages ok.

## Findings
None.

## Previous findings

| ID | Status |
| --- | --- |
| QA-23-r2-1 (pace/speed without distance) | fixed: `page.go` gates `speed` on `hasDist`, the same condition as `distance`. The TestCard "distance 0" case (fixture with an average speed, distance 0) now expects no speed, so it covers the no-distance-with-speed case |
| QA-23-r2-2 (no icon-per-sport test) | fixed: `TestIconPerSport` checks that the five icon keys draw different pixels and that hiking shares the walking icon |
| QA-23-r2-3 (PC-6 example in v1 order) | fixed: example now in v2 order with gain and speed |

## Questions
None.

## Coverage
Only the r2 findings and the HEAD diff were reviewed. The r1 conditions (operator look, AC 3 visual check, AC 8 and 9 on a deployed instance) are unchanged. Not run: `make ci`, vitest, `-race`.
