# QA F-23 Preview card, Open Graph and indexing - round 2

- **Date:** 2026-10-31
- **Verdict:** not ready. One Major: the card and `og:description` show pace or speed when the workout has no distance. SH-5 and PC-3 forbid this, and `TestCard` asserts the wrong behaviour. The fix is one condition.
- **Inputs:** `features/F-23-preview-card-and-indexing.md` (AC as of c24ec64); `architect/F-23-preview-card-and-indexing.md` PC-3, PC-4, PC-6; ADR-033; product spec SH-5 (c24ec64); decision log 31 Oct (OQ-91). Diff reviewed: c24ec64 (spec, feature, design) and a92821d (`internal/page`, `internal/preview`, tests). Working tree clean at a92821d. Follow-up round, diff only.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. Go: all packages ok. vitest: 51 files, 1,331 passed. Size: index and edit 56,366 / 60,000 B gzip, upload 2,272 / 8,000 B. `caddy-check` (2.11.7): ok, 2.32 s |
| `go test -count=1 ./internal/page ./internal/preview ./cmd/workout-explorer` | ok (0.04 s, 9.4 s, 6.5 s), uncached |
| `WX_CARDS=… go test -run TestSampleCards ./internal/preview` | ok; six v2 cards written and inspected |
| Probe (scratchpad overlay, `go test -overlay`): page `Build` with `Summary.Distance = ""` for ride, run and pool swim | see QA-23-r2-1 |
| Probe: rendered `ride-12h` and a GPS run relabelled `swimming` (open water) | layout ok: the longest values (`304.02 km`, `11:59:59`, `0:30 /100 m`) stay left of the route box (x 560) |

## Findings

### Major

- **ID:** QA-23-r2-1
- **Severity / kind:** Major / defect
- **Route:** backend
- **Where:** `src/server/internal/page/page.go:178` (`add("speed", cardSpeedSports[vm.Header.SportIcon])`); `src/server/internal/page/page_test.go` case "distance 0"
- **Relates to:** SH-5 ("Distance, pace or speed only when the workout has a distance"), PC-3 ("pace per 100 m for swims (only with distance)"), AC 2 ("no distance, pace, speed … value")
- **Evidence:** the speed item is gated only on the sport, not on distance. Probe output with the summary distance removed:
  `ride-30min: "Moving time 29:59 · Elevation gain 104 m · Avg speed 26.9 km/h · Avg heart rate 142 bpm"`,
  `run-30min: "… Avg pace 5:03 /km …"`, `pool-swim: "Elapsed time 20:00 · Avg pace 2:00 /100 m · Avg heart rate 132 bpm"`.
  The "distance 0" case in `TestCard` expects `"Moving time 29:59 · Elevation gain 104 m · Avg speed 26.9 km/h · Avg heart rate 142 bpm"`, so the test asserts the behaviour the spec forbids. AC 2 passes only because the `indoor-no-distance-no-gps` fixture has no average speed.
- **Consequence:** a file with an average speed but no distance, or a distance of 0, shows a pace or speed on the card and in `og:description` (for example a trainer ride with virtual speed and no distance). Fix: gate `speed` on the same distance condition as `distance`, and change the "distance 0" expectation. Add a no-distance case with an average speed.

### Minor

- **ID:** QA-23-r2-2
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/internal/preview/preview.go:299` (`icon`); `preview_test.go`
- **Relates to:** AC 3 (sport icons)
- **Evidence:** no test checks which icon a sport draws. Only the `TestSampleCards` output covers it, and the user reviewed that by eye. The `cardSpeedSports` side is tested (the rowing case).
- **Consequence:** a swapped or dropped `case` in `icon` would pass `make ci`. The user's review covers AC 3 for now; a later change would not be caught. A cheap check: the icon square's pixels differ between the five icon keys.

- **ID:** QA-23-r2-3
- **Severity / kind:** Minor / defect (docs)
- **Route:** architect
- **Where:** `architect/F-23-preview-card-and-indexing.md:20` (PC-6)
- **Relates to:** PC-3, PC-6
- **Evidence:** the PC-6 example `"Moving time 1:23:45 · Distance 42.3 km · Avg heart rate 142 bpm"` shows the v1 order. The code now puts distance first and adds gain and pace/speed (`publish_test.go:116`).
- **Consequence:** stale example. Update it when the document is next touched.

## Previous findings

| ID | Status |
| --- | --- |
| QA-23-r1-1 (ADR-033 compression, size, x/text, x/sys) | fixed in 99866ce: ADR-033 item 1 says default compression; consequences give about 1.2 MB and list x/text and x/sys |
| QA-23-r1-2 (description save keeping the preview untested) | fixed in 99866ce (`publish_test.go`); passes in `make ci` |
| QA-23-r1-3 (`TestServeWithoutBaseURL` hangs) | fixed in 99866ce (`settings_test.go`); passes in `make ci` |

## Questions

None.

## Coverage

- **AC 1 (ride: icon, name, distance, moving time, km/h, avg HR, gain):** `TestCard` "ride", `TestPublishEndToEnd` (og:description); the image was checked by eye (`ride-hr-gain`).
- **AC 2 (indoor without HR):** `TestCard` "indoor without heart rate". It is weakened by QA-23-r2-1: the fixture has no average speed.
- **AC 3 (run pace, swim /100 m, walk/hike pace, generic icon):** the value side is covered by `TestCard` (run, pool swim, rowing). There is no walking or hiking case in `TestCard`; it shares `paceSports` with running, so the risk is low. The icon side was checked only by eye (QA-23-r2-2).
- **AC 4 (cut at 40 plus "…"):** `TestName` (from r1). `Render` now passes the narrower width (minus the icon); `Name` takes the width as a parameter, and the "cut until it fits" branch is tested.
- **Gain rules:** `TestCard` "gain rounds to 0" and "no altitude recorded". Max-only fallbacks are excluded for speed and HR (`HasPrefix "Max "`; "max heart rate only" case).
- **Privacy:** the diff adds only values that are already in the public page summary (gain, pace/speed) plus a fixed icon. It adds no coordinates, no token and no new field to the tags. Tag printing and escaping are unchanged since r1.
- **Not run:** `-race` (the diff is pure, single-goroutine code); `make corpus` (the diff does not touch ingest or the data file); the messenger checks (AC 8/9, still waiting on a deployed instance).
