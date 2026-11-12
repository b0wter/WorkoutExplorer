# QA F-47 Trim start and end - round 1

- **Date:** 2026-11-12
- **Verdict:** not ready - the code keeps the trimmed part out of every public file in every probe I ran, but two of the privacy mutants that the design requires to fail a test survive. Removing the clearing of a cut session's device totals publishes the whole session's totals, trimmed part included, and no test notices (QA-47-r1-1).
- **Inputs:** `features/F-47-trim.md` (all ACs), decision log OQ-169 to OQ-179 in `features/README.md`; spec UP-6, AP-2, AP-3, SH-4, SH-9, SH-10 (and SH-12, AP-19 as touched); `architect/F-47-trim.md` v0.1, ADR-047, ADR-017 (memory), ADR-030, F-15 P-7/P-15, F-30 RG-4/RG-5. Commits 0e1f655 (backend) and 0368ad0 (frontend); working tree clean apart from `features/README.md` (not reviewed).

## Build and test results

- `make ci`: exit 0. All Go packages ok, vitest 68 files / 1,754 tests passed, size check: viewing entry `index.html` 56,392 / 60,000 B gzip (the engineer reports +29 B, within the 100 B ceiling), caddy-check and docker-check ok.
- `go test -race -count=1 ./internal/{workout,publish,lifecycle,httpapi,store}`: all ok. `go vet ./...`: clean.
- Probes ran against a copy of `src/server` in the scratchpad. Nothing under `src/` was changed.
  - **Leak sweep (real parser):** 43 synthetic corpus files (ride-12h left out for time), each with four windows (end, start, both, one sample off each end) and hiding off and on: 344 published builds. For every build, every valid value of every stream in the public data file equals the untrimmed file's value at the same sample in the window. Offsets, distance and gain are rebased by a constant (±0.1 m from quantisation), and moving time starts at 0. Start time equals the start plus offset[a]. Elapsed time equals the window. Summary maxima stay within the window maxima, and the distance stays within the window distance. Calories, timer time and lengths are absent. Every session and lap span lies inside the window, and the number of laps equals the number of laps that overlap the window. With hiding on, no public position lies within 300 m of the first or last remaining recorded fix before the route leaves that circle. The two hits were at 300.0 m, quantisation at the boundary, and the unquantised positions were confirmed. The owner `trimmed` summary bytes and sport equal the public metadata, the owner `suggested_title` equals `og:title`, and no owner `hide_ends_ranges` index is public. Rebuilding gives the same names. After the trim is removed, the data file is byte-equal to the draft's and no old file remains.
  - **Metadata by hand:** checked ride-30min (cut on both ends), triathlon trimmed into T1 (multisport, "Morning multisport near …") and into the run ("running", "Morning run near …"), pool-swim (`pool_length_m` 25 kept, `lengths` gone, pool laps kept), run-30min-cut-60 (truncated), brick-run-bike and chained-ride-run. Cut sessions and cut laps carry only values computed from the window.
  - **AC 42:** gps-dropout.fit trimmed to its 299 s gap, hiding off and on. The page has `Route.NoLocation` ("No location data in this workout."), not the entirely-hidden hint.
  - **End to end through `newApp` (real store and area), PATCH:** 59 s gives 400 "At least 1 minute of the workout must remain." and 60 s gives 200. Null ends work, and the year-0 and far-future bounds store NULL. `.000Z` and `+02:00` are accepted. `.5Z`, lowercase `z`, no zone and `""` get 400. A duplicate key gets 400. A repeat stores the same row. A change to the end alone is checked against the stored start. Start after end gets 400. Widening on a published workout replaced all four file names. The logs contain no trim instants.
  - **Migration:** a v5 database made by the pre-F-47 binary (`git archive 68f20f9`), with an upload and a publish, opens under the new binary as v6. The old row reads `trim_*` null. A trim PATCH works (row offsets 120/1200, public files rebuilt). The CHECK constraint rejects start ≥ end.
  - **Memory (ADR-017, TR-10), real parser, `build/cap-500k.fit`, max RSS of the test binary:** untrimmed publish 269 to 276 MB, end-trimmed 274 MB, start-trimmed 277 MB. The trim PATCH sequence (resolve, build, owner view) reached 287 to 288 MB in 5.2 to 5.3 s. All are within noise of the untrimmed build and below 400 MB.
  - **Mutants (design list plus extras):** 16 of 19 killed. M4a (a cut session's device totals kept) and M4c (the summary built with the device totals of uncut sessions) survive (QA-47-r1-1). M15 (timer events after the window not clipped) survives, but it is equivalent: timer events are not encoded, and moving time is computed over the window.

## Findings

### Major

- **ID:** QA-47-r1-1
- **Severity / kind:** Major / risk (test gap in the privacy core)
- **Route:** backend
- **Where:** `src/server/internal/workout/trim.go:95-97` (cut session totals cleared) and `:169-173` (session totals left out of the summary). The tests are in `src/server/internal/workout/trim_test.go` and `src/server/internal/publish/trim_test.go`.
- **Relates to:** AC 33 ("a test searches for them"), AC 34, AC 39; TR-5; design Verification, Mutants ("device session or cut-lap totals kept" must fail a test)
- **Evidence:** I deleted `x.Totals = Totals{}` in the session loop and the whole suite still passes. With that mutant, ride-30min trimmed to 900 s publishes `sessions[0].totals` = `{"elapsed_time":1799, "distance":13461.18, "calories":299, …}`: the device totals of the whole recording, trimmed part included. Replacing `bare.Sessions[i].Totals = Totals{}` also passes, and the trimmed triathlon summary then shows `"calories":507, "timer_time":3057`, against AC 34. The cause is that the public leak test (`trimTrack`) and the leg test have no sessions with device totals, and the leak test searches only the heart-rate, latitude and altitude streams. It does not search the metadata (session and lap totals, summary) or the page for the drive's values.
- **Consequence:** The code is correct today: I found no leak in 344 builds. But the one guarantee of this feature is unguarded on the metadata side. Any later refactor of `Trim` or `summarise` that keeps session totals would publish the trimmed part's distance, speed, heart rate and calories with a green CI.

### Minor

- **ID:** QA-47-r1-2
- **Severity / kind:** Minor / risk (test gap)
- **Route:** backend
- **Where:** `src/server/internal/publish/trim_test.go`, `src/server/internal/publish/regenerate_test.go`
- **Relates to:** AC 42, AC 47
- **Evidence:** No automated test covers a trim that leaves no positions (no-location hint, not "entirely hidden"). No test covers `regenerate` keeping the trim columns. Both behave correctly: AC 42 by my probe above, AC 47 by reading the code (`UpdateDerived` writes no settings columns, and RG-5 `sameRecord` compares the trim fields by reflection).
- **Consequence:** A regression in either would not be caught.

- **ID:** QA-47-r1-3
- **Severity / kind:** Minor / risk
- **Route:** backend (or architect, to record the choice)
- **Where:** `src/server/internal/httpapi/owner.go:112-115` (`buildDeadline`), `:249` (GET with `ownerTimeout`), `src/server/internal/lifecycle/owner.go` `View`
- **Relates to:** TR-10 ("widens P-15's PATCH write deadline to cover both"), P-15
- **Evidence:** A trim PATCH on a published workout now runs three parses under the O-7 lock: resolve, build, then view. The 80 s deadline still budgets two. A GET of a trimmed workout now waits for the O-7 lock and parses, still under the 30 s `ownerTimeout`. Locally at the caps the PATCH took 5.3 s, so this is far from the limit in practice. The worst case is three 10 s parse limits behind another such PATCH holding the lock.
- **Consequence:** On a slow VPS under load, a saved trim can answer with a cut connection, or the edit page of a trimmed workout can fail to load, while the change itself is committed. Either widen the deadlines or record in the design that they are sufficient.

- **ID:** QA-47-r1-4
- **Severity / kind:** Minor / defect (wording)
- **Route:** backend
- **Where:** `src/server/internal/httpapi/owner.go:364-366`
- **Relates to:** F-12 error messages, TR-3
- **Evidence:** `{"trim_start_at":12345}` gets 400 "The title and description must be text, hide_ends and allow_download true or false, and lifetime_days a whole number." The message does not mention the trim fields. `msgBadTrim` exists and fits.
- **Consequence:** A misleading error for an API client. The slider never sends a number.

- **ID:** QA-47-r1-5
- **Severity / kind:** Minor / preference (report accuracy)
- **Route:** backend
- **Where:** `src/server/internal/workout/trim.go:148` (`Derived{}`) and `legs.go:72`
- **Relates to:** TR-5 ("Descent is recomputed or dropped, the engineer's choice")
- **Evidence:** The engineer reported that descent is dropped after a trim. It is dropped only when the summary comes from `Derived`. For truncated or session-less workouts, the trailing-leg path recomputes descent from the window: run-30min-cut-60 trimmed to its first half gives `descent` 9.867. Both values come from the window, and AP-2 does not show descent.
- **Consequence:** No privacy effect. The behaviour is inconsistent and differs from the report. Make it one rule and note it in the design.

## Engineer deviations

- **Backend.** I accept dropping descent, with the wording fixed under r1-5. I accept keeping the truncated flag on an end trim: it only reaches the owner's `data_end_s` on the edit page, which shows the full recording, and nothing public. I accept dropping the sport hints before the trim start: they only decide the sport of samples that no session covers, and no value of the trimmed part is published. The three parses on a trim PATCH are covered under r1-3.
- **Frontend.** I accept building the strip in the edit glue with `summaryItems`: it formats only (ADR-026), and a stored trim with `trimmed` null shows no values (tested). I accept `EditSeam.onlook`, but it is fragile: it redefines `look.decorate` with `Object.defineProperty`, which a later change to `Look` could silently break, although tests cover leg lines and trim shading together. I accept hiding the slider under 60 s (no trim is possible there). The +29 B is within the ceiling. Trimmed parts in the same grey as hidden marks on the map: UP-6 only asks for "shaded", and both mean "not public"; see Questions.

## Previous findings

None (first round).

## Questions

1. **Map shading:** TR-9 asked for the trimmed parts in their own map source below the hidden marks. The build greys them the same as the hidden marks, so the owner cannot tell a hidden section from a trimmed one on the map (the charts show only the trim). Suggestion: accept, since both are non-public and the slider label shows the trim times. Evidence that would change this: an owner confusing the two in the user's manual check.

## Coverage

- **Traced to code and a passing test:** AC 31, 32, 34 (summary only; see r1-1 for sessions), 35, 36, 37, 38, 39, 40, 41, 43, 44 (remove-first, MC-8(b) job), 45 (by construction: the trim path never writes under `private/`), 46 (pool and indoor tested; truncated by my sweep).
- **Verified by probe only:** AC 33 metadata side (r1-1), AC 42 and AC 47 (r1-2), owner view equals public across 344 builds, migration on an existing database, memory and timing at the caps.
- **Not run:** AC 48 LCP and 60 fps on `ride-12h` (browser), the live slider in a browser, the preview basemap replacement in a real browser, the real-device corpus (not needed: the synthetic sweep covers the shapes). I did not decode preview PNG pixels separately: the PNG is rendered from the decoded public data file (`publish.go:439`), and the existing fixture checks it is that render.
