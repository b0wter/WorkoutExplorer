# F-76 Taller charts

- **Status:** planned (25 Nov 2026), built right after F-75; OQ-359 to OQ-367 decided 25 Nov 2026 (user, as suggested)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (frontend: toggle, CSS and the head script; backend: the CSP hash in `deploy/Caddyfile` and its check; frontend first)
- **Lane:** normal
- **Requirements:** DS-8; AP-4, AP-6, AP-7, AP-17, AP-18, AP-19, DS-1, DS-6, INF-1; NFR Performance (entry budget, LCP, 60 fps)
- **Depends on:** F-05, F-14, F-22 (CSP), F-54, F-73 (head script), F-74, F-75 (entry reserve, ADR-011)

## Problem
Every chart's plot area is a fixed 120 px on every screen (`DEFAULT_CHART_HEIGHT`), so on small phones speed, heart rate or power are hard to read and hard to scrub. Viewers have no way to make the charts larger.

## Scope
- **Toggle (DS-8):** "Taller charts" in the charts' header row, next to the time/distance toggle, on the viewing and edit pages, in every layout; pressed while tall. No drag handle.
- **Sizes:** normal 120 px plot area (default, today) and tall 200 px, for every stacked line chart (AP-4) at once. On short landscape screens (AP-17) tall is capped so one chart (title row and plot) fits the visible height of the content column.
- **Not affected:** comparison chart (AP-23), power curve, zones; cursor, range selection, robust axis (AP-26) and scrubbing work as at the normal size; the pinned map (AP-18) stays as it is.
- **Remembered** in browser storage like the units (DS-1); only a choice made with the toggle is stored; blocked storage or an unreadable value gives normal without a message, the toggle still works for that page. Never sent to the server, not in the range link (AP-19).
- **No layout jump on load:** the existing inline head script (F-73) also reads the stored size and sets a class on the page before the first paint, so the CSS reserves the tall chart space; the same validation as the bundle (unreadable or blocked gives normal). Read only: `lib/prefs` stays the only writer. New CSP hash; the previous one stays until no served page carries it (ADR-032 item 3). The script text stays the same on every page (one hash).
- **Texts:** toggle label in en, de, es, fr (DS-6); About page "In your browser" paragraph lists the setting (INF-1), in all four languages.
- **Entry bundle:** the control sits in the chart stack, which is in the viewing entry. Its bytes come from the reserve F-75 frees; if the reserve is still below 1,000 bytes after F-75, the entry share must be net zero (ADR-011 decision 5).

## Out of scope
- Hiding or reordering individual charts; per-chart sizes; more than two sizes; pinch-to-zoom on the value axis.

## Acceptance criteria
- [ ] Fresh browser: charts at 120 px plot height, toggle not pressed, on the viewing and edit pages.
- [ ] Tapping the toggle makes every stacked chart's plot area 200 px, pressed state shown; tapping again returns to 120 px. Comparison chart, power curve and zones keep their size.
- [ ] The choice is remembered: reload, another workout and the edit page open at the chosen size. Nothing about it in any request to the server (network log) or in the range link.
- [ ] Blocked storage (Safari private, site data blocked) or an unreadable stored value: normal size, no console error; the toggle still works for that page.
- [ ] No layout jump: with tall remembered, a cached and an uncached load show no layout shift from the charts (CLS contribution 0 in a performance trace); the laps table's position does not change after the charts appear.
- [ ] Short landscape phone (e.g. iPhone 13 mini landscape, 812 × 375): tall charts are capped so one chart's title row and plot fit the visible height.
- [ ] Cursor scrubbing, range selection by drag and long-press, and the cursor readout work at both sizes, mouse and touch; 60 fps scrubbing on ride-12h at the tall size (phone reference, NFR).
- [ ] Toggle label and About text present in en, de, es, fr; the label fits at 360 px width in every language.
- [ ] Both themes; the leg view (F-67) keeps the chosen size.
- [ ] Entry bundle: measured before and after; within ADR-011 (reserve or net zero as above); `make ci` passes the 62,000 check. LCP unchanged.
- [ ] No CSP violation on any page (viewing, edit, upload, About, generic); the Caddy check passes with the new hash; the old hash still listed. JavaScript off: normal size, page unchanged.
- [ ] **UI test** `src/web/e2e/f76-chart-size.pw.mjs` (Playwright, synthetic workouts only) covers: toggle on and off (plot height 120 → 200 → 120); remembered after reload and on the edit page; blocked storage gives 120 without error; sizes on iPhone (portrait 375 × 812 and landscape 812 × 375, cap checked), iPad (portrait 820 × 1180) and desktop (1440 × 900); no layout jump on load with tall stored (position of the element below the charts equal at first chart paint and after load, and no layout-shift entry from the charts).

## Decisions (user, 25 Nov 2026, as suggested)
- [x] **OQ-359 Control.** One small "Taller charts" toggle in the chart stack's header row, next to the time/distance toggle; no drag handle.
- [x] **OQ-360 Sizes.** Two: normal (120 px plot, today) and tall (200 px plot).
- [x] **OQ-361 Where offered.** Every layout; on short landscape screens a tall chart is capped so one chart still fits the screen height.
- [x] **OQ-362 Remembered.** In this browser like the units (DS-1), listed in the About page's "In your browser" paragraph (INF-1); never sent to the server, not in the range link.
- [x] **OQ-363 First paint.** Replaced by OQ-367.
- [x] **OQ-364 Edit page.** The same control and setting.
- [x] **OQ-365 Pinned map in portrait (AP-18).** Allowed, no automatic change; the viewer can unpin the map.
- [x] **OQ-366 Requirement.** DS-8 "Chart size", written 25 Nov 2026.
- [x] **OQ-367 No jump on load.** The F-73 head script also reads the stored size and sets a class before the first paint, so the CSS reserves the tall space; new CSP hash; no entry bytes. Replaces OQ-363.
