# F-13 Activity page layout and template: history

History of [architect/F-13-activity-page-layout.md](../F-13-activity-page-layout.md).

## 1.1, 24 Oct 2026: two columns on desktop, pinnable map on the phone

User decision of 24 Oct 2026 (decision log, `features/README.md`). Changes:

- D-10 rewritten: no viewport units in any height (was: none in reserved heights); layouts moved to D-17; user-caused movement (pinning) allowed.
- D-15: the privacy notice sits beside the map in two columns.
- New: D-17 (two layouts, breakpoint 1024 px, one DOM order, CSS grid placement, right column ≥ 608 px), D-18 (desktop map sticky), D-19 (phone pin toggle, bundle-only), D-20 (map size follows its box, MapLibre's own resize, refit until moved, no remount).
- Budgets, verification, hand-offs (F-19, F-26, requirements) and OQ-65 added.

### D-10 as written in 1.0

> **No layout shift.** Markup and CSS reserve every box the client fills before the bundle runs: the map box when presence is `route`; the chart area by F-05's `chartAreaHeight(n, chartHeight)`, chosen by a class from the chart count; and the readout's first-render height at each breakpoint. No `style` attributes (CSP). No `vh` or `dvh` in reserved heights: phone browser bars change them. System fonts only. One column at every width, with a maximum width on desktop; phone portrait, phone landscape and desktop differ only in heights and spacing. With JavaScript off, reserved boxes stay empty.

### D-15 placement as written in 1.0

> placed directly under the summary, above the map.

## 1.0, 24 Oct 2026

First design. OQ-61 to OQ-64 decided the same day.
