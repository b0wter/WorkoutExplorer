# F-76 Taller charts on small screens

- **Status:** noted (25 Nov 2026), not to be built now; no build-order position; open questions OQ-359 to OQ-366
- **Category:** Planned, unscheduled (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend (assumed)
- **Lane:** normal (assumed)
- **Requirements:** AP-4, AP-6, AP-7, AP-17, AP-18; DS-1 (remembered like the units), INF-1; NFR Performance (entry budget, LCP, 60 fps). A new display-setting ID (DS-8) once the questions are decided (OQ-366)
- **Depends on:** F-05, F-74, F-75 (entry reserve, ADR-011)

## Problem
On small phones the stacked charts are small: every chart's plot area is a fixed 120 px tall on every screen (`DEFAULT_CHART_HEIGHT`), so details in speed, heart rate or power are hard to read and hard to scrub. Viewers have no way to make them larger.

## Rough scope
- The viewer can switch the charts to a taller size; the default stays as today.
- Applies to the stacked line charts (AP-4) only, all of them at once; not the comparison chart (AP-23), power curve or zones.
- Cursor, range selection, robust axis (AP-26) and scrubbing behave as today at either size.
- **Entry bundle:** the viewing entry's reserve is about 110 bytes and below the 1,000-byte trigger, so the feature's entry share must be net zero (ADR-011 decision 5): the control and its text load lazily, or other entry bytes are removed in the same feature. An exception needs the user's acceptance.

## Out of scope
- Hiding or reordering individual charts; per-chart sizes; pinch-to-zoom on the value axis.

## Open questions
- [ ] **OQ-359 Control.** How does the viewer choose? Suggestion: one small toggle in the chart stack's header row ("Taller charts"), next to the time/distance toggle; no drag handle.
- [ ] **OQ-360 Sizes.** How many sizes and how tall? Suggestion: two, normal (120 px plot, today) and tall (200 px plot).
- [ ] **OQ-361 Where offered.** Every layout, or only small screens? Suggestion: every layout (one code path); on short landscape screens a tall chart is capped so one chart still fits the screen height.
- [ ] **OQ-362 Remembered.** Suggestion: remembered in this browser like the units (DS-1), listed in the About page's "In your browser" paragraph (INF-1); never sent to the server, not in the range link.
- [ ] **OQ-363 First paint.** A remembered tall size must not make the page jump. Suggestion: read the setting when the charts mount (they are drawn after the summary), no head-script change and no new CSP hash; accept that a viewer who switches sees the stack grow once.
- [ ] **OQ-364 Edit page.** Suggestion: yes, the same control and setting (same layout as the viewing page, AP-17).
- [ ] **OQ-365 Pinned map in portrait (AP-18).** With the map pinned at 35 % of the screen, tall charts leave little room. Suggestion: allowed, no automatic change; the viewer can unpin the map.
- [ ] **OQ-366 Requirement.** Suggestion: a new DS-8 "Chart size" in Display settings, written when the feature is scheduled; translations in de, es, fr (DS-6).
