# F-33 Cursor readout in the fullscreen map: architecture

- **Version:** 0.1
- **Status:** accepted (4 Nov 2026). Normal lane. No open questions
- **Feature:** [`features/F-33-fullscreen-cursor-readout.md`](../features/F-33-fullscreen-cursor-readout.md)
- **Requirements:** AP-21, AP-15, AP-6
- **Builds on:** [F-26](F-26-fullscreen-map.md) FS-3, FS-4, FS-5, FS-9; [F-05](F-05-stacked-charts-with-shared-cursor.md) D-10, D-17; ADR-008, ADR-011, ADR-015, ADR-038. Changes FS-3: the readout is no longer on F-26's hidden list, as a second instance (CR-1)
- **Owner:** frontend only

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| CR-1 | **A second `CursorReadout` instance (the "strip"), not the page's readout moved or restyled.** Same component, same `data`, same cursor store, same units as the charts' readout, so the values are the same by construction (AC 1). It follows units changes the way `ChartStack` does (a new instance per units value, since the component fixes its fields at mount). The page's readout stays where it is, hidden with the page (FS-4) | Moving or fixing the page's readout over the overlay fights its sticky card layout, the stacking contexts lifted by QA-26-r1-1 and FS-4's "no trapping ancestor" rule for a second subtree. Two instances cost one more `textContent` loop per frame, on a box that is `display: none` outside fullscreen |
| CR-2 | **Mounted by the page glue (`src/page`), into `div.route-map`** (the pseudo-fullscreen element), from the existing `onmapready` callback. `components/map` does not import `components/charts` (TD: map and charts meet only in the cursor store), and `map-view` stays unaware of it. No map, failed map or no route: no strip, as no fullscreen (FS-9). Viewing and edit pages alike (same glue); `bench/map.html` has none | Keeps the module boundary; the container is the only box visible in fullscreen |
| CR-3 | **Shown only in fullscreen and only with a cursor.** Outside fullscreen: `display: none`, keyed on the `html.wx-fs` class F-26 sets. Without a cursor (`null`): hidden; the glue toggles it on the null/non-null transition only (the range slot's pattern), never per frame. Since the cursor stays after the finger lifts (OQ-13), the strip appears once and stays | AC 2, AC 5 |
| CR-4 | **Bottom of the container, opaque, above the map, never covering a control.** Full width, theme background (`--bg`, `--fg`). While the strip is shown, the bottom control corners (attribution, F-25 colour control and legend; at the bottom on phone portrait, at the top in the two-column media query of `map-view.css`) sit above it. The strip's height reaches that CSS as one custom property on the container, set by CSS from the known row count or by one `ResizeObserver`, never per cursor frame. Top corners (zoom, exit) are untouched | AC 3 |
| CR-5 | **Fixed tracks, bounded height.** As the page's readout (F-13 D-10), the strip lays its cells on fixed grid tracks, so its row count depends on width and field set only, never on the digits: no layout shift while scrubbing. Compact cells. On a 320 px wide portrait phone and in phone landscape it takes at most a third of the map's height | Scrub smoothness; small phones |
| CR-6 | **The route under the strip is accepted.** No camera padding, no refit when the strip appears (FS-5 stays: no camera rule of our own; a padding change mid-scrub would move the route under the finger). The viewer pans to see the covered band | Cost of CR-4. Would change if the operator check shows the fitted route regularly under the strip: then add bottom padding to the fit only while unmoved (FS-5 amendment) |
| CR-7 | **No input of its own.** `pointer-events: none`: a hover or a one-finger scrub that crosses the strip keeps driving the map (ADR-015 unchanged); the strip only reads the cursor store. No focusable element, no shortcut | AC 1 on touch and hover |
| CR-8 | **Accessibility.** No `aria-live` (F-05: it would be announced up to 60 times a second). Outside fullscreen `display: none` removes it from the accessibility tree, so a screen reader never meets two readouts; in fullscreen the page's readout is hidden by `visibility`, so only the strip is read. Text contrast from the theme variables, both schemes | NFR Accessibility |
| CR-9 | **Range panel.** Nothing to build: it stays outside the container and hidden by FS-4 (AC 4) | — |

## Cost

| What | Decision |
| --- | --- |
| Static | The glue and the strip's CSS are static (the component already is). Share at most **300 bytes gzip** from the ADR-011 reserve; the engineer records the measured figure as F-33's row in ADR-011 item 4. Above: stop and report |
| Lazy | 0 beyond the corner offset rule in `map-view.css` |
| Scrub | One more subscriber writing `textContent` (F-05 D-10 path, no Svelte reactivity). ADR-016 gate 0, route part, re-run in fullscreen with the strip shown, as F-26's gate; recorded in `bench/MEASUREMENTS.md` |

## Verification notes

Automated (jsdom): strip mounted in the map container only after map-ready, hidden without a cursor and outside fullscreen, same text as the page's readout for a cursor value and after a units change. Manual (operator, both phones and desktop, portrait and landscape): AC 1 to AC 5, no control covered, no shift while scrubbing.
