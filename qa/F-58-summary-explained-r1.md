# QA F-58 How the summary is computed - round 1

- **Date:** 2026-11-19
- **Verdict:** ready with conditions. No Blocker or Major. The privacy check (AC 39) holds when the real Go publish output runs through the real viewing glue, and every privacy mutant is killed. Conditions: the hand checks below. jsdom has no `matchMedia` or layout, so no test runs the desktop placement, the phone dialog or the themes. Six Minor findings, mostly layout of the ⓘ corner and test gaps around which end was trimmed.
- **Inputs:** `features/F-58-summary-explained.md` (AC, OQ-251 to OQ-260); `architect/F-58-summary-explained.md` v0.1 (SE-1 to SE-7, P-11 decided); F-16 H-1, H-5, H-6, H-8; F-47 TR-3, TR-8, TR-9; ADR-011 item 4; spec AP-24. Diff `d392756..0b19c3b` (commits 13fbc31 backend B-1, 0b19c3b frontend), working tree clean at 0b19c3b.

## Build and test results

- `make ci` at 0b19c3b: **exit 0**. gofmt and `go vet` clean; svelte-check 0 errors, 0 warnings; `go test -count=1 ./...` all packages ok; vitest 75 files, **1942 passed**; size `entry index.html: 57456 / 60000 bytes gzip PASS` (+251 against the recorded 57,205 baseline, as booked in ADR-011); caddy-check (caddy 2.11.9), docker-check and update-check passed.
- Lazy chunks (not counted): `summary-info` 1,524 B gzip JS (budget about 3,000), shared `dialog` 639 JS + 402 CSS, `howto` 907 → 684.
- **AC 39 through Go (scratch copy of `src/`, nothing in the repo changed).** I added a scratch test in `internal/publish` that publishes through the real `Area` build (template, transform, encode). It covers 14 synthetic corpus files: ride-30min, triathlon, pool-swim, pool-swim-25yd, zones-wahoo-ride, indoor-no-gps, run-30min, brick-run-bike, no-distance-with-gps (distance computed from GPS), gps-dropout, chained-ride-ride, ride-4h, timer-pause and pause-hours. It also covers two generated walks: 500 m, which is fully hidden with hiding on, and 2 km, which has both ends hidden. Each source was published with hiding off and on, each untrimmed and trimmed at both ends. A scratch jsdom test then loaded each published `index.html` with its `data.bin` through `startPage`, clicked ⓘ and saved the dialog's `outerHTML` and the button's markup, in metric and imperial.
  - Result: the dialog HTML and the button markup are **byte-identical** between hiding off and on for all 16 sources × 2 trims × 2 unit systems (64 comparisons). That includes the fully hidden walk (no position columns, notice instead of the map) and partial hiding. The trimmed variants differ from the untrimmed ones, so the check can see a difference.
- **Privacy mutants** (frontend: the repo's `summary-info.dom.test.ts` and `edit.dom.test.ts` F-58 blocks; backend: `go test ./internal/publish ./internal/page ./internal/httpapi`):
  - Killed: a topic depending on whether position data exists (A); on the first sample's position (B); on the title (C); on the route hints (D); on `#map-slot` (E); owner facts passed on the viewing page (F); the hiding line conditioned on `has-map` (G); the trim string taken from the route markup (H); the edit radius ignoring the settings (I); the transition count depending on `has-map` (K). Go: `Trim` depending on `routeHidden` (killed by `TestHidingFullyHidden`) or on `HideEnds` (killed by `TestHidingCorpus`).
  - Survived (not privacy, see r1-4 and r1-5): in Go, the publish `Trim` line removed (G3) and start/end swapped (G4). In the frontend, start/end swapped in the edit glue (L) and in `TRIM_ENDS` (M), and the edit model's `Trim` not refreshed after a save (J). All three frontend survivors also pass the full web suite. The scratch copy has two failures it already had before any mutant, both from missing `deploy/` paths.
- **Deviation probe:** I made the chunk import `summaryItems` from `lib/page` directly and built. Vite then splits out a `format-*.js` chunk, and `index.html` loads it as an extra module script. Passing `summaryItems` in as an argument is justified.
- No headless browser is installed here (no chromium, firefox or playwright). I downloaded none, so the placement and layout findings come from reading the CSS.

## Findings

### Minor

- **ID:** QA-58-r1-1
- **Severity / kind:** Minor / defect (not rendered in a browser)
- **Route:** frontend
- **Where:** `src/web/src/page/page.css:365-373` against `page.css:741-745` and `page.css:687-690`
- **Relates to:** SE-5 (corner reserved), AC 36
- **Evidence:** `.summary { padding-right: 24px }` has specificity (0,1,0). In the two-column map layouts, `.has-map .layout > * { padding: 0 16px 0 24px }` (desktop ≥1024 px) and `{ padding: 0 12px 0 16px }` (short landscape phone) have specificity (0,2,0) and win. The strip's right padding is therefore 16 px or 12 px, while the absolutely placed button is 24 px wide at `right: 0`.
- **Consequence:** on the main desktop layout (a workout with a map), the ⓘ overlaps the top-right metric card by 8 px; on a landscape phone with a map, by 12 px. The reservation only works in the one-column layouts.

- **ID:** QA-58-r1-2
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/page/page.css:372`
- **Relates to:** SE-5, F-13 strip layout (AP-2); the question in the brief
- **Evidence:** the strip is `repeat(auto-fill, minmax(140px, 1fr))` with an 8 px gap, so it has floor((W + 8) / 148) columns, where W = viewport − 32 in the one-column layout. Taking 24 px from W drops one column at viewport widths 320–343 (2 → 1), 468–491 (3 → 2), 616–639 (4 → 3), 764–787 (5 → 4; includes 768 px portrait tablets) and 912–935 (6 → 5). This is the same with and without JavaScript, so there is no layout shift (D-10 holds).
- **Consequence:** at 320 px the strip becomes a single column about twice as tall, which pushes the map and charts down. Reserving the corner without taking a full column strip, for example with a negative offset or room in the first row only, would avoid it. How is the engineer's choice.

- **ID:** QA-58-r1-3
- **Severity / kind:** Minor / defect (owner page only)
- **Route:** frontend
- **Where:** `src/web/src/summary-info/index.ts:82-84` with `src/web/src/edit/start.svelte.ts:165-166`
- **Relates to:** SE-4, P-11, F-47 TR-8 and TR-9 (a trim is stored but `trimmed` is `null` after a failed read: the strip shows no values)
- **Evidence:** a scratch probe of `openSummaryInfo` with owner facts, an empty strip summary, both ends trimmed and a full untrimmed summary prints: `This workout: trimmed (now starts 06:10, ends 06:40); dropped: distance, moving time, elapsed time, calories, avg speed, elevation gain.`
- **Consequence:** the owner is told that every total was dropped by the trim. In fact the server's trimmed values are just unavailable. With an empty strip, the example should name neither recomputed nor dropped totals.

- **ID:** QA-58-r1-4
- **Severity / kind:** Minor / risk (test gap)
- **Route:** backend
- **Where:** `src/server/internal/publish/publish.go:431`; `page.TrimOf` (`src/server/internal/page/page.go`)
- **Relates to:** SE-2, AC 38 (the trim example on the viewing page)
- **Evidence:** mutants G3 (the `Trim:` line removed from the publish settings) and G4 (start and end swapped) pass `go test ./internal/publish ./internal/page ./internal/httpapi`. The only Go coverage is the `ride-trimmed` sample, which calls `Build` with `Trim: "both"` directly. No test publishes a trimmed record and checks `data-trim`, and nothing checks `TrimOf` with `start` or `end`.
- **Consequence:** a refactor can silently drop the trim line from every viewing page, or name the wrong end. CI stays green.

- **ID:** QA-58-r1-5
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/edit/start.svelte.ts:154` and `:203` (`Trim: next.Trim`); `src/web/src/summary-info/index.ts` `TRIM_ENDS`
- **Relates to:** SE-2, SE-4, AC 38, AC 40
- **Evidence:** these mutants pass the full web suite: L (the edit glue swaps `start` and `end`), M (`TRIM_ENDS` swaps "at the start" and "at the end"), and J (the edit model's `Trim` is not refreshed after a save). The tests only use `both` and `''`, and the edit test saves a hiding change, never a trim change.
- **Consequence:** after a trim save without reload, the edit page could show a stale or missing trim line, or "at the start" for an end trim, with no test catching it.

- **ID:** QA-58-r1-6
- **Severity / kind:** Minor / doc
- **Route:** architect
- **Where:** `architect/F-58-summary-explained.md` SE-4 ("The published page (`/published/`, same edit seam) counts as the edit page")
- **Relates to:** SE-4; F-20 C-5
- **Evidence:** `startEdit` returns `startPublished` for `/published/` (`src/web/src/edit/start.svelte.ts:132`). That page shows settings and links only, with no strip, so there is no place for an ⓘ. The engineer's deviation ("no ⓘ on `/published/`") is correct.
- **Consequence:** the design states a surface that does not exist; a later reader may "fix" it. One line to correct in the small lane.

### Engineer's deviations (judged)

1. `summaryItems` passed into the lazy chunk: **accepted**. The probe confirms that a direct import splits `lib/format` into a separate chunk the entry has to load.
2. Edit-page radius in m or ft per the units setting: **accepted**, DS-1 requires it (300 m → 984 ft, tested).
3. Load-failure text reused from How to use: **accepted**. One line after the strip, never a second (tested).
4. "This workout:" example only where there is a real value: acceptable, but it departs from the wording of AC 38. See Question 1.
5. No ⓘ on `/published/`: **accepted**; the design text needs fixing (r1-6).
6. Entry +251 B (51 over the 200 B ceiling): measured 57,456 / 60,000, as booked in ADR-011; accepted by the main session from the reserve (2,544 left).

## Previous findings

None (round 1).

## Questions

1. **AC 38 wording** (requirements): AC 38 says the dialog shows "each topic's general line and this workout's example". As built, five topics have no example: Elapsed time without transitions, Range panel and laps, Zones, Pool when the strip has no length value, and Hiding on the viewing page (no example there by design, SE-3). Suggestion: accept, and change the AC to "and, where the workout has a value, this workout's example". Settled by a yes from the user.
2. **Multisport without transition sessions** (architect, SE-3): brick-run-bike (1:01:00 elapsed, 1:00:00 moving) and chained-ride-ride (24:58 elapsed, 19:58 moving) get only "including pauses". The feature scope says "in multisport, transitions and gaps", but SE-3 ties the sentence to `transition` sessions. Suggestion: add "and gaps between legs" whenever there is more than one session. The session count is public metadata and does not depend on hiding, so AC 39 is unaffected. Settled by a one-line SE-3 decision.

## What the user should check by hand

1. Desktop ≥1024 px, workout with a map: whether the ⓘ sits on the top-right card's corner (r1-1). Open it: the dialog should cover the strip, right-aligned with it and inside the viewport. Scroll down and open again: it should be clamped to 8 px from the top.
2. Phone in portrait (360 to 430 px): the dialog should be centred and full width, and close with Close, Esc or the back gesture, and a tap outside. The URL and history should not change. Also check that the ⓘ (24 px wide, 16 px glyph) is easy to hit.
3. Widths 320 px and 768 px portrait: the strip's column count (r1-2).
4. Both themes: the ⓘ colour and the muted example lines.
5. With the production CSP (Caddy, `style-src 'self'`; protoserve sends none): the desktop placement uses `element.style.cssText`. CSSOM changes should be allowed, but check the console for CSP violations.
6. Edit page: save a new trim, then open ⓘ without reloading. The trim line should name the right end (r1-5).

## Coverage

- **AC 36** (button, none without JS): template `hidden` and revealed after decode (`page/enhance.ts`, `page/start.ts:186-189`); `hygiene.test.ts`, the `summary-info` test asserts hidden → shown. Traced.
- **AC 37** (style, placement, closing, history): shared shell `src/dialog/index.ts`; close and backdrop tested in `howto.dom.test.ts`; `showModal` stubbed. Placement code (`dialog/index.ts:27`) never runs in jsdom (no `matchMedia`): **manual only** (hand checks 1, 2).
- **AC 37 topic list** (OQ-254): ride, triathlon, pool and zones tested; the trim line is absent when untrimmed (edit test). Traced.
- **AC 38** (general line plus example; trimmed says recomputed): tested for "both" only (r1-4, r1-5); see Question 1.
- **AC 39** (hiding): by construction (SE-1 facts, `start.ts:188-189`), the repo jsdom check, my Go-published 64-way comparison and the mutants above. Several radii are not possible until F-55; the hand-off is recorded. Traced and verified.
- **AC 40** (owner radii, trim ends, recomputed and dropped): `edit.dom.test.ts` F-58 block, `summary-info` owner tests. Traced; the defect in r1-3 is on the failure path.
- **AC 41** (lazy, budget): size check PASS, chunks listed above. LCP not measured; nothing new on the first-paint path apart from markup and CSS.
- **AC 42** (themes, units): units tested (imperial text); themes via tokens only, **manual** (hand check 4).
- Not run: a real browser, a Lighthouse or LCP measurement, `-race` (the Go change is one string field).
