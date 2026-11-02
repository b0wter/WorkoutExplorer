# ADR-037 Route colouring drawn as colour classes: one MultiLineString per class in its own source

- **Status:** accepted (2 Nov 2026), with the F-25 design v0.1
- **Date:** 2 Nov 2026
- **Features:** F-25 (introduces it), F-26 (fullscreen keeps it)
- **Supersedes:** none. Adds an overlay source next to [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md) item 8's `wx-route`; `wx-route` is unchanged
- **Depends on:** [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md) items 4, 7, 8; [ADR-008](ADR-008-shared-cursor-store.md); [ADR-016](ADR-016-scrubbing-smoothness-measurement-and-gate.md)
- **Requirements:** AP-9, AP-3, NFR Performance (60 fps on the 12-hour file)

## Context

AP-9 colours the route by a metric. The route is drawn from up to 43,200 positioned samples (the 12-hour file), split into recorded segments and dashed bridges (F-06 D-7). Bridges and hidden sections must stay as they are (F-25 AC 6, AC 7), and scrubbing must stay at 60 fps. MapLibre GL JS 6 offers three ways to colour a line:

| Option | Why not, or why |
| --- | --- |
| `line-gradient` over `line-progress` | Needs `lineMetrics: true`. Its expression **cannot use feature properties** ("data-driven styling: not supported yet", MapLibre style spec, checked 2 Nov 2026), so every feature of the layer gets the same gradient over its own length. With dropouts the recorded part is many lines, each needing its own stops, so it would need one layer per line, and up to 43,200 stops each |
| One feature per edge, continuous `interpolate` on a value property | Up to 43,200 GeoJSON features: a large `setData` (worker tiling, memory) on every metric switch, and a legend that cannot name the colour of a given value exactly |
| **One `MultiLineString` per colour class** | At most one feature per class; geometry is the recorded edges only; the colour comes from a `match` on the class number |

## Decision

1. Each sample's value maps to one of a fixed small number of **colour classes** (F-25: 7), or to none (missing).
2. A separate GeoJSON source `wx-colour` holds **at most one `MultiLineString` per class**. Consecutive recorded edges of the same class are merged into one line. Edges without a class are left out, so the plain `wx-route-line` underneath shows through in the route colour.
3. One layer `wx-colour-line` draws it, with `line-color` chosen by the class property. It lies directly above `wx-route-line` and below every other `wx-` layer (hidden marks, range, markers).
4. Plain mode is the same source with no features. The source exists from the base style on, like `wx-range`.
5. The colouring changes only through one `setData` on a viewer's selection, never on a cursor or range update.

## Consequences

- Bridges and hidden sections cannot be coloured: they are never in the source.
- Missing values, stops of the pace sports and transitions fall back to the plain route colour without a special layer (AP-9).
- A metric switch costs one `setData` of the route's recorded vertices. Scrubbing costs nothing new: the cursor marker is still a DOM marker (ADR-013 item 4).
- A units switch only relabels the legend, because the classes are defined in a quantity that unit conversions scale linearly (F-25 C-4). A scheme switch only repaints.
- **Cost:** the route shows visible steps instead of a smooth gradient, and the legend has a fixed number of swatches.

## Reversibility

Cheap. Lazy map code only, no data format, nothing stored.

## Would choose differently if

- The operator finds 7 steps too coarse on screen: first raise the class count (up to about 12, still one feature per class), before considering per-edge features.
- A MapLibre release supports feature properties in `line-gradient`: then a smooth gradient per recorded segment becomes possible in one layer, and this ADR is revisited.
