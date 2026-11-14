# QA F-51 Lap and split comparison chart - round 1

- **Date:** 2026-11-14
- **Verdict:** not ready - one slow lap (a standing rest lap in an interval run) flattens every other bar to 1-2 px in a pace sport, so the chart fails its goal on a common real workout (QA-51-r1-1)
- **Inputs:** `features/F-51-split-comparison-chart.md`, decision log OQ-196 to OQ-209 (`features/README.md`), spec AP-23, `architect/F-51-split-comparison-chart.md` (SC-1 to SC-10), ADR-011 item 4, `architect/F-14-units-and-theme.md` D-9; commits `0215492` (F-51) and `659c6be` (theme meta, small lane); working tree clean at `0215492`

## Build and test results
- `make ci`: exit 0. svelte-check 355 files, 0 errors, 0 warnings. `go test -count=1 ./...` all packages ok. Vitest 72 files, **1,889 passed**. Build ok; Caddy and Docker checks ok.
- Size: `entry edit.html: 57028 / 60000 bytes gzip PASS` (index.html the same entry). Matches ADR-011 (+34 for F-51 within the 80 ceiling, +55 for `659c6be` recorded as unbooked). Splits chunk 9.54 kB / 4.30 kB gzip (no before figure recorded in ADR-011 or the commit; the Cost table asks for one).
- Probe (scratchpad vitest against `lib/compare`, not committed): see QA-51-r1-1.

## Findings

### Major

- **ID:** QA-51-r1-1
- **Severity / kind:** Major / defect (design rule)
- **Route:** architect (SC-5 scale rule), then frontend
- **Where:** `src/web/src/lib/compare/index.ts:72-73`; SC-5 "half-range the larger of the furthest drawn bar end and the band edge"
- **Relates to:** AP-23 goal ("sees at a glance which laps ... were faster or slower"), AC 2, OQ-203
- **Evidence:** run, Laps view, 6 work laps at 4.5 m/s, 5 jog recoveries at 2.5 m/s, one standing rest lap at 0.3 m/s (pace, linear in 1/v). Result: reference 3.5 m/s; work bars 1.1 px, recoveries 1.9 px (both drawn at the 2 px minimum), the rest lap 51 px; top axis label empty (`top` = -0.36 m/s, formatted as ''), bottom label "55:33". Without the rest lap the same laps draw 51 px bars. Any lap slower than half the median in a pace sport pushes the top edge past infinite speed.
- **Consequence:** interval runs and swims with a rest or walk lap, the main use of the Laps view, show one long blue bar and a flat line; the top axis label disappears. Needs a clip rule (for example the half-range from the counted rows' spread, bars beyond it cut at the edge and marked, as the pace chart's 95th-percentile clip in F-14 D-11), or a product decision that slow laps without zero speed get no bar. Evidence that would change my mind: the user accepts the flattened chart for such workouts.

### Minor

- **ID:** QA-51-r1-2
- **Severity / kind:** Minor / defect (accessibility)
- **Route:** frontend
- **Where:** `src/web/src/splits/chart.ts:86`, `:161`
- **Relates to:** OQ-201, SC-9
- **Evidence:** read: an overlay change calls `draw`, which replaces the header and with it the focused `<select>`.
- **Consequence:** a keyboard user loses focus after the first change (arrow keys on a focused select fire `change` in Chromium), so stepping through the overlay options by keyboard does not work.

- **ID:** QA-51-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/page/laps.dom.test.ts` (comparison chart block)
- **Relates to:** AC 8 (trim, edit page), AC 9 (truncated file), AC 10 (LCP, summary timing, 60 fps on `ride-12h`)
- **Evidence:** no test for a trimmed workout's chart or the edit page's rows, none for a truncated corpus file; no performance measurement in the commit. All hold by construction (SC-3: the chart reads the page's `data`; `draw` is not subscribed to cursor or selection flushes, only the O(1) mark seam), which I verified by reading.
- **Consequence:** a later change to how the splits chunk receives `data` could break UP-6 on the edit page unnoticed.

- **ID:** QA-51-r1-4
- **Severity / kind:** Minor / documentation
- **Route:** frontend
- **Where:** ADR-011 item 4 F-51 row; F-51 Cost "Lazy"
- **Relates to:** F-51 Cost table
- **Evidence:** the splits chunk's gzip size before and after is not recorded (now 4.30 kB gzip).
- **Consequence:** lazy growth is untracked.

## Previous findings
None (first round).

## Questions
1. QA-51-r1-1: should a slow lap still get a full-length bar (clip at the edge with a marker), or should the scale ignore outliers beyond a bound? Suggestion: half-range from the counted bars excluding the slowest 10 % (or capped at ±50 % of the reference), longer bars cut at the edge.

## Coverage
- AC 1 (11 bars, partial left out, hand-computed median/best): `compare.test.ts` "10.4 km", `splits.test.ts` "partial last split", DOM test "1 km splits: 14 bars". Traced.
- AC 2 (classes, colours, pace up): `compare.test.ts` "classes and geometry", DOM "run: pace in the header"; CSS classes map to `--zone-red/blue/grey`. Traced.
- AC 3 (view and distance switch, lengths without rest): DOM "1 km splits", "pool swim lengths". Traced.
- AC 4 (bar tap = row tap, highlight): DOM "a bar tap selects as its row tap"; tap forwarded via `row.click()` through the one LS-8 listener (SC-7). Traced.
- AC 5 (overlay offer, None on load, dots): `compare.test.ts` overlay, DOM "overlay". Not in prefs or fragment (read). Traced.
- AC 6 (empty slot, <2 bars, multisport, >200): `compare.test.ts` slots, DOM "a lap without speed". Traced.
- AC 7 (units redraw, both themes): DOM "units redraw it"; zone tokens moved to `src/theme/zones.css`, imported by both chunks; selected bar outlined (not colour only). Theme not checked in a browser.
- AC 8, AC 9, AC 10: by construction only, see QA-51-r1-3. Entry size measured (57,028, PASS). No browser, LCP or fps run.
- CSP: SVG built by `innerHTML` with geometry attributes only, no `style` attributes; labels are row numbers. No `require-trusted-types-for` in the policy. OK.
- `659c6be`: meta `color-scheme` set from the stored theme on start and on each choice; `units.dom.test.ts` asserts all three states and the stored-dark load; F-14 D-9 noted. OK. Before the page script runs, the meta still says `light dark` (the hashed head script does not set it); acceptable for the toolbar strip, not reported.
- Not run: Lighthouse, browser check on iOS Safari, mutants.
