# QA F-13 Activity page layout and template - round 2

- **Date:** 2026-10-24
- **Verdict:** ready with conditions. No Blocker or Major finding. Conditions: (1) AC 7 (Lighthouse, both presets) has not been run and still waits for the user's Chromium machine (OQ-64); (2) AC 11, AP-17 and AP-18 as written contradict the layout the user chose, so AC 11 cannot be ticked as worded until the feature file and spec are updated (r2-1).
- **Inputs:** follow-up diff `git diff 29197cb HEAD -- src architect/F-13-activity-page-layout.md architect/ADR-026-activity-page-template-adopted-not-hydrated.md` (HEAD `b4f84b4`); `architect/F-13-activity-page-layout.md` v1.1 (D-10, D-17 to D-20 with their 24 Oct 2026 notes, OQ-65); ADR-026 rev. 1 (items 4, 5); `features/F-13-activity-page-layout.md` (scope, AC 7, 8, 11, 12); spec AP-17, AP-18, NFR Compatibility; decision log of 24 Oct 2026 in `features/README.md`. The user's later choices recorded as notes in D-10, D-17, D-18 and D-19 (header and footer in the right column, `100lvh` flush sticky map, 1:1 columns, attribution at the top; `35lvh` / 125 px pinned map, collapsed attribution, "Pin map" / "Unpin map", readout card) are taken as given and are not findings. The user's manual check of 24 Oct 2026 (iPhone 13 mini and iPad, Safari and Orion) is taken as the manual verification of AC 11 and AC 12.

## Build and test results

- `make ci`: exit 0. `go vet ./...` clean. `svelte-check`: 279 files, 0 errors, 0 warnings. `go test ./...`: all packages ok (cached; `tools/protoserve` includes the new `has-map` and "no toggle in server markup" checks). `vitest run`: 36 files, 970 tests passed (959 in r1). Bundle: `entry index.html: 47299 / 60000 bytes gzip PASS`; F-13's share is 47,299 − 45,582 = 1,717 bytes (905 in r1), within its 5,000 (ADR-011). Upload entry 2,272 / 8,000 PASS.
- `build/web/index.html`: no `<style>` and no `style=` (ADR-026 item 5).
- protoserve on 8095 (HEAD): `/w/ride-12h` has `class="page has-map"`, `/w/indoor-ride` has `class="page"`; neither page contains `map-pin` or `<button`. `/w/ride-12h` is 7,123 bytes as served and 1,830 bytes after `gzip -9` of that body, far below 15,000 (AC 1). Headers unchanged: `Referrer-Policy: no-referrer`, `nosniff`, `no-store`.
- No headless browser exists on this machine (no Chromium, Firefox or Playwright), so CSS layout behaviour below is argued from the CSS and the CSS specifications, not measured. Findings that rest on that are marked.

## Findings

No Blocker, no Major.

### Minor

- **ID:** QA-13-r2-1
- **Severity / kind:** Minor / defect (requirements drift)
- **Route:** main session, then senior-requirements-engineer (spec and feature file; no code change)
- **Where:** `spec/Workout Explorer — Product Specification.md:158-159` (AP-17, AP-18); `features/F-13-activity-page-layout.md:22-23` (scope) and AC 11; `features/README.md:199` (decision log)
- **Relates to:** AP-17, AP-18, AC 11, AC 12; CLAUDE.md "one home per fact" (product decisions go in the decision log)
- **Evidence:** AP-17 says "The header spans the full width on top, the footer at the bottom"; F-13 scope and AC 11 say "header across the top"; AP-18 and the F-13 scope say "The toggle's label avoids the word 'pin'". The code puts header and footer in the right column (`page.css` two-column block, `index.html`) and labels the button "Pin map" / "Unpin map" (`enhance.ts:63-64`). These choices are recorded only as notes in the architecture document; the decision log entry of 24 Oct 2026 does not mention them.
- **Consequence:** The code is right by the user's choice, but AC 11 as worded fails, and the next agent reading AP-18 will "fix" the label back. Update AP-17, AP-18, the F-13 scope and AC 11, and add a decision log line.

- **ID:** QA-13-r2-2
- **Severity / kind:** Minor / defect (not measured: no browser here; by arithmetic)
- **Route:** frontend
- **Where:** `src/web/src/page/page.css:252-259`
- **Relates to:** AP-18 ("While pinned, the map is smaller than in the page"), D-19 ("a smaller fixed height per orientation"), AC 12
- **Evidence:** The unpinned map is 320 px; the pinned map is `35lvh` with no upper bound. One column applies to every viewport below 1024 px wide. On an iPad in portrait (820 × 1180 CSS px) `35lvh` is about 390 to 410 px, so pinning makes the map larger. The same holds for any one-column viewport taller than about 915 px (desktop windows narrowed below 1024 px, foldables, large tablets in portrait). On the iPhone 13 mini it is about 260 px, which is why the manual check passed.
- **Consequence:** On tablets in portrait the "smaller while pinned" promise is broken and the charts get less room than intended. `min(320px, 35lvh)` keeps the user's 35 % on phones and caps it elsewhere.

- **ID:** QA-13-r2-3
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/page/page.css:252-253`; `src/web/src/page/hygiene.test.ts:132`
- **Relates to:** D-19 note ("`35lvh` with a `35vh` fallback")
- **Evidence:** `--pin-h: 35vh; --pin-h: 35lvh;` is not a fallback. Custom properties accept any token sequence, so the second declaration always wins the cascade; in a browser without `lvh`, `height: var(--pin-h)` and `top: var(--pin-h)` become invalid at computed-value time and fall back to `auto`, so the pinned map box would collapse to its content and the readout would stop sticking. The fallback that does work is the one on the real property (`height: 100vh; height: 100lvh;` at line 300-301). The hygiene test asserts the two-line pattern, so it checks the text, not the behaviour. Today this has no effect: every browser in NFR Compatibility (last two versions) supports `lvh`.
- **Consequence:** Dead code that a later reader will trust, and a test that cannot catch a real fallback failure. Either drop the `35vh` line and the claim, or put the fallback on `height` and `top` themselves.

- **ID:** QA-13-r2-4
- **Severity / kind:** Minor / risk (read from code; not reproduced in a browser)
- **Route:** frontend; the user confirms the rule
- **Where:** `src/web/src/components/map/map-view.ts:373-382`
- **Relates to:** D-20 (attribution collapsed on phones); ADR-014 (OpenFreeMap, OSM data); OSMF Attribution Guidelines
- **Evidence:** The attribution is collapsed on every `idle` until the viewer taps it. The credits only appear after the remote style is merged (MapLibre 6.14 `_updateAttributions` then `_updateCompact` opens them), and the next `idle` follows as soon as that style's tiles have rendered. So on a phone the credits are visible only for the tile load time, typically well under the five seconds the OSMF guidelines name for automatic collapse ("after five seconds", or on interaction, and attribution "should not require individuals to interact with the map").
- **Consequence:** The phone layout may fall short of the OSM attribution guidelines. Collapsing five seconds after the credits first appear, or on the first map interaction, would meet both the user's choice and the guideline at the same cost.

- **ID:** QA-13-r2-5
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.dom.test.ts:970-983`; `src/web/src/lib/page/build.ts:135`
- **Relates to:** D-18, D-20, AC 11, AC 12 ("one with no visible route")
- **Evidence:** (a) The refit test runs with jsdom's `clientWidth` 0, so only the 32 px padding branch of `fitPadding()` is tested; the two-column 60 px bottom padding is not. (b) The phone attribution collapse (`matchMedia('(max-width: 1023.98px)')`, `idle`, the tapped flag) has no test. (c) No sample or builder test covers a `no-visible-route` workout, so "one column at every width, no toggle" for a fully hidden route rests only on `build.ts:135` mapping `Map` to `''` for anything but `route`; a change there would pass CI.
- **Consequence:** Regressions in these three paths would pass `make ci`.

## Previous findings

The diff does not touch the code of any r1 finding, so all three remain as they were (not fixed, not re-verified beyond that):

- QA-13-r1-1 (load failure leaves the reserved chart area blank): not fixed; `showLoadError` and `.charts.load-failed` unchanged. In two columns the map box is now one window tall, so the text in the map slot is the visible one; the blank right-column area remains.
- QA-13-r1-2 (`&nbsp;` before the unit in the template vs SummaryStrip): not fixed; `index.html` still has `&nbsp;`.
- QA-13-r1-3 (protoserve `gzipSize` discards errors): not fixed; only `samples_test.go` changed in protoserve.

## Questions

1. **Attribution on phones (r2-4), for the user:** may the credits collapse on the first `idle`, or should they stay visible for five seconds or until the first interaction? Answer "five seconds or first interaction" settles r2-4 as a small-lane fix; "first idle is fine" closes it as accepted.
2. **Pinned height on tablets (r2-2), for the user:** should the pinned map be capped at the unpinned 320 px? "Yes" settles r2-2 with `min()`; "no, 35 % everywhere" means AP-18's "smaller than in the page" must be reworded.

## Coverage

| Item | Code | Test | Status |
| --- | --- | --- | --- |
| AC 7 (as reworded: both presets; a pin tap is not a shift) | reserved sizes in `page.css`; readout card padding added to `.charts` height; toggle is an absolutely positioned overlay inserted after map ready | `hygiene.test` (reserved readout padding, viewport units only in the two allowed rules) | open: Lighthouse not run (OQ-64) |
| AC 8 (indoor: hint, one column at every width) | `{{if .Route.Map}} has-map` in `index.html`; two-column rules only under `.has-map` | `samples_test.go` (class iff `Route.Map`), `ActivityPage.dom.test` (indoor has no `has-map`), `page.dom.test` (no toggle on indoor) | traced |
| AC 11 (two columns from 1024 px, map sticky, one column below and without a route, JS off) | `page.css` two-column block (grid, `minmax(608px, 1fr)`, map spans rows 1-7, sticky, `100lvh`); CSS only, class from the template | `hygiene.test` layout structure; `samples_test.go`; user's manual check (iPad) | traced; wording conflict r2-1; no-visible-route untested r2-5 |
| AC 12 (pin on phone, ≤ 40 %, to the end of the laps, marker follows, not stored, no toggle ≥ 1024 / indoor / hidden route) | `enhance.ts addPinToggle`, called in `onmapready` (`start.ts`); `page.css` phone block; `.map-pin { display: none }` ≥ 1024 px; `RouteMap` `fill` (D-20); refit until moved (`map-view.ts`) | `page.dom.test` pin toggle (after map ready only, class and `aria-pressed`, no remount, no storage, none on indoor or failed load); `hygiene.test` (35 %, 125 px ≤ 40 %); `map-view.dom.test` refit; user's manual check (iPhone 13 mini, both orientations) | traced; r2-2 for tablets |
| ADR-026 rev. 1 items 4, 5 | toggle not in the template; layouts by media query and template class | `hygiene.test` (no `map-pin`, `<button`, `aria-pressed`, `pinned` in the template) | traced |

Checked and found correct: the grid has seven rows for at most seven right-column items, so the map spans every row and stays sticky to the end of the footer; the spanning map adds its height only to the flexible last row; `.layout.pinned` rules are inert at ≥ 1024 px; the toggle is created once per slot (guarded) and is a sibling of the map container, so `map-view`'s capture listeners never see it; the refit ignores programmatic camera changes (no `originalEvent`) and stops after a viewer move, matching D-20; ChartStack measures its own content box, so the new column padding does not widen the charts; the readout's row count is set by CSS columns and `nowrap` cells, so the narrower right column cannot add rows.

Not run: Lighthouse (AC 7); any real-browser layout measurement (no browser here). The `package-lock.json` change in the diff is a dev-only `undici` bump (8.11.2 to 8.12.3) with no runtime effect on the page; it is not explained in the diff.
