# QA F-47 Trim start and end - round 2

- **Date:** 2026-11-12
- **Verdict:** ready - the Major r1-1 is closed: both surviving privacy mutants (M4a, M4c) now fail a test. One Minor remains: the AC 47 test guards a fake repository, not the real store (QA-47-r2-1).
- **Inputs:** `qa/F-47-trim-r1.md` (QA-47-r1-1 to r1-5); commit 5afc5ff (diff against 2e13737 only: `architect/F-47-trim.md` TR-5 and TR-10, `httpapi/owner.go`, `httpapi/owner_test.go`, `lifecycle/regenerate_test.go`, `publish/trim_test.go`); `architect/F-47-trim.md` TR-4, TR-5, TR-10; F-15 P-15; ADR-017. Working tree clean.

## Build and test results

- `make ci`: exit 0. All Go packages ok, vitest 68 files / 1,754 tests passed, `entry index.html: 56392 / 60000 bytes gzip` PASS (unchanged from r1), docker-check ok.
- `go vet ./...`: clean. `go test -race -count=1 ./internal/httpapi ./internal/lifecycle`: ok.
- Mutants, on copies of `src/server` in the scratchpad (with `src/web` linked in for the page template); `go test -count=1 ./internal/publish ./internal/workout ./internal/lifecycle`. The unmutated copy passes.
  - **M4a** (`x.Totals = Totals{}` for a cut session removed, `trim.go:95-97`): `TestTrimKeepsDeviceTotalsOfTheTrimmedPartOut` fails, "end inside the run" and "start inside the run": "the cut run's device total 5071 / 999999 is public". Killed.
  - **M4c** (`bare.Sessions[i].Totals = Totals{}` removed, `trim.go:172`): the same test fails, "end at the run's end" and "start at the run's start": summary has `"calories":5071`, `"timer_time":600`, `"distance":999999`. Killed.
  - **New, M-RG** (`UpdateDerived` SQL also sets `trim_start_at = NULL, trim_end_at = NULL`, `store/store.go:378`): `./internal/store` and `./internal/lifecycle` both pass. Survives (QA-47-r2-1).

## Findings

### Minor

- **ID:** QA-47-r2-1
- **Severity / kind:** Minor / risk (test gap; r1-2 partially fixed)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/regenerate_test.go:316-331` (`TestRegenerateKeepsTheTrim`), `src/server/internal/store/store_test.go:526` (`TestUpdateDerived`)
- **Relates to:** AC 47, TR-10 ("RG-4 never writes settings columns")
- **Evidence:** The new test checks the trim on `r.rows["w1"]` after a regenerate, but that row is written by `fakeRepo.UpdateDerived`, a hand copy of the real column list. Its "row" half tests the fake. `TestUpdateDerived` in the real store keeps the settings columns via `reflect.DeepEqual`, but its record has no trim set, so nil equals nil. The M-RG mutant above (the real SQL clears both trim columns) passes the whole suite. The "hook" half of the new test is meaningful: the build gets the trim.
- **Consequence:** A later change to `UpdateDerived` that clears the trim passes CI. The next build after a regenerate (for example after a parser upgrade) then publishes the whole recording, trimmed part included. Fix: set `TrimStartAt` and `TrimEndAt` on the record in `TestUpdateDerived`. The existing `DeepEqual` then catches it.

## Previous findings

- **QA-47-r1-1 (Major): fixed.** `TestTrimKeepsDeviceTotalsOfTheTrimmedPartOut` (`publish/trim_test.go:167`) builds run plus drive sessions with marker totals, cut and uncut, at both ends. It searches the metadata of the decoded data file and the page, and checks the summary for `calories`, `timer_time` and the marker distance. M4a and M4c are killed (above). **Fixture realism:** the fixture is synthetic, with two sessions, three device totals each and no laps per session. That is enough for what the test has to guard. Clearing totals is all or nothing (`Totals{}`), so one marker per session catches it. Cut-lap totals were already killed in r1 (M-list 16/19). The r1 real-parser sweep over the corpus triathlon, brick and chained files found no leak. A corpus triathlon would add parse time without adding a case this one misses. Accepted.
- **QA-47-r1-2 (Minor): partially fixed.** AC 42: `TestTrimLeavingNoPositions` asserts the no-location hint and not the hidden notice, hiding off and on. Fixed. AC 47: see QA-47-r2-1.
- **QA-47-r1-3 (Minor): fixed.** `buildDeadline` = 3 × (30 s + parse limit) is used by PATCH, publish and now the owner GET (`owner.go:114-115`, `:244`). TR-10 was rewritten to match. **Sanity on the 2 GB VPS:** it is only a write deadline on the connection (`begin`, `owner.go:211`). The read deadline stays 30 s, and the work is still bounded by the O-7 lock and the parser slots. The longer deadline therefore adds no memory and no concurrency (ADR-017 M(N) unchanged). It is 120 s at the default 10 s parse limit and 270 s at the 60 s maximum. The only cost is that an owner GET of an untrimmed workout also gets 120 s instead of 30 s for a small response. Acceptable.
- **QA-47-r1-4 (Minor): fixed.** A number as a trim time now gets `msgBadTrim`. Tested in `TestPatchBody` (`owner_test.go:219`).
- **QA-47-r1-5 (Minor): fixed.** TR-5 now records the actual rule: descent is dropped, except that the trailing leg of a truncated or session-less workout recomputes it from the window.

## Questions

None. The map shading (r1 question) was decided by the user: trimmed and hidden parts share the same grey.

## Coverage

- Re-verified in this round: AC 33, 34, 39 (metadata side, by mutants), AC 42 (test), AC 47 (hook half by test; store half by reading only, see r2-1), TR-10 deadlines.
- Unchanged since r1 and not re-run: the leak sweep, migration, memory at the caps, the frontend (no frontend change in the diff), and AC 48 in a browser.
