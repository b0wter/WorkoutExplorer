# F-25 Route colouring by metric: history

History of [architect/F-25-route-colouring.md](../F-25-route-colouring.md). Read it only when you need the history.

## 0.2 (7 Nov 2026, accepted; user decisions of 7 Nov 2026 after testing heart rate colouring on real rides)

Feedback: a fast steady stretch on the flat and a maxed-out climb got the same colour. Causes: equal-width classes between p5 and p95 spend classes on rarely visited values, and a single-hue palette makes neighbouring classes hard to tell apart. Changed: C-1 and C-6 (class counts, legend), C-4 (quantile edges), C-5 (three-hue palette), C-8 and Budget (figures), Verification rows AC 3 and C-3; new C-10 (smoothing kept).

Superseded text of v0.1:

- C-1: "Each sample's value maps to one of **7 classes** or none."
- C-4: "Speed or pace, heart rate, power: **workout-relative**. Bounds are the 5th and 95th percentile of the drawn edges' values (so hidden sections and stops do not move them); values outside clip to the end classes; 7 equal-width classes between the bounds. Equal bounds give one class, never a division by zero. Bounds may be rounded outward to the legend's precision. [...] Gradient: **fixed** class edges −8, −4, −1, +1, +4, +8 % (starting values), not percentiles."
- C-5: "One sequential palette for these, one diverging palette for gradient (descent, flat in the middle, climb). Both are 7 custom properties each in the theme block, light and dark (F-14 D-9) [...]. Constraints: sequential lightness is monotonic; no class is confusable with `--route` (missing) or `--range` (drawn over the colouring) in either theme; the order stays readable with red–green colour-vision deficiency." Values shipped: a single-hue yellow → brown sequential palette and a BrBG-like diverging one.
- C-6 legend: "the 7 swatches, the bound labels (gradient: its class edges) with the unit".
- Budget, static: "The units prop wiring plus C-5's 28 palette properties in the static theme block: 253 bytes gzip measured (QA-25-r1-1), from the reserve".
- Verification AC 3, automated: "Classes are monotonic in the value; clipping at the bounds; legend labels in both unit systems".
