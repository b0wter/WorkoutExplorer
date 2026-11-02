# F-05 Stacked charts with shared cursor: architecture

- **Version:** 1.4
- **Status:** accepted
- **Date:** 11 Oct 2026
- **History:** [architect/history/F-05-stacked-charts-with-shared-cursor.md](history/F-05-stacked-charts-with-shared-cursor.md) (revision list, closed questions)
- **Feature:** [`features/F-05-stacked-charts-with-shared-cursor.md`](../features/F-05-stacked-charts-with-shared-cursor.md)
- **Depends on:** TD (system-wide picture), [F-01 design](F-01-project-skeleton-and-ci.md) v1.3 (D-6 Vitest in Node, no jsdom; D-8 every `svelte-check` warning fails CI), [F-04 design](F-04-workout-data-file-reader.md) v1.1 (`WorkoutData`, `gaps`), [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-007](ADR-007-decoded-workout-data-in-the-browser.md) rev. 1, [ADR-008](ADR-008-shared-cursor-store.md), [ADR-009](ADR-009-uplot-as-renderer-only.md), [ADR-010](ADR-010-touch-gestures-on-charts.md), [ADR-011](ADR-011-activity-page-bundle-allocation.md), [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md)
- **Requirements:** AP-4, AP-6 (chart side), NFR Performance (60 fps scrubbing on a 12-hour workout); TD Front end (Implementation notes: one cursor store), TD Workout data file (Charts: min/max per pixel); review decisions 5 (time axis: elapsed time, pauses as breaks; tie → earliest sample) and 6 (touch gestures)
- **Owner:** frontend (senior-frontend-engineer)
- **Supersedes:** none

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | Three new modules: `lib/cursor` (pure store), `lib/charts` (pure decimation, chart specs, formatting, gesture reducer), `components/charts` (Svelte + uPlot). Only `components/charts` imports `uplot` | Module boundary; ADR-009 |
| D-2 | The cursor is a framework-free store holding a sample index or `null`; writers call `set`, readers `subscribe`; delivery coalesced to one flush per animation frame; subscribers get the current value on subscribe | Cursor store; ADR-008 |
| D-3 | uPlot only draws. One uPlot instance per chart, cursor, legend, select and sync off. We own pointer handling and draw the cursor line as one transformed `div` per chart | Charts; ADR-009 |
| D-4 | Charts in order elevation, speed, heart rate, power, cadence, temperature (`CHART_ORDER`), only for present streams with at least one value. Distance is never a chart | Which charts |
| D-5 | Min/max per pixel column, per *segment*; a break (`null`) between segments. Segments end at every missing sample and at every gap. Decimation reads `WorkoutData` and never changes it | Breaks and decimation |
| D-6 | The x scale range is fixed to `[time[0], time[N−1]]` on every chart, the y axis has a fixed width and the plot padding is fixed (never uPlot's auto padding), so all plots share the same pixel-to-time mapping | Charts |
| D-7 | Touch: `touch-action: pan-y pinch-zoom` on the plot area; the browser owns vertical scrolling and pinch. (v1.4) Touch down places the cursor under the finger at once; moves and up set it; `pointercancel` (the browser took a scroll or pinch) restores the cursor from before the touch. The flicker when a scroll starts on a chart is accepted (user, P3 option B). One drag need not continue onto another chart (user, P2) | Interaction; ADR-010 rev. 2 |
| D-18 | (v1.4) Scroll lock. Each touch is classified once, when it is 8 CSS px from its start: within 30° of horizontal is a *scrub*, anything else a *pan*. During a scrub, the chart's one non-passive `touchmove` listener calls `preventDefault()` (one touch, `cancelable` only), so the page does not scroll for the rest of that touch. Pans and pinches are never prevented | Interaction; ADR-010 rev. 2 |
| D-8 | The cursor stays where it is when the pointer leaves the chart (mouse and touch) | Interaction; OQ-13 |
| D-9 | Readout and cursor line read full-resolution arrays, never the decimated data | Readout |
| D-10 | Hot path (cursor flush): no allocations other than short strings (the readout's formatted values and one `transform` string per chart), no Svelte reactivity, no layout reads, no canvas redraw | Performance |
| D-11 | Dev page `bench/charts.html` for every browser check and the 60 fps measurement (auto-scrub + frame monitor); results in `bench/MEASUREMENTS.md` | Performance |
| D-12 | Bundle: nothing in the counted entry yet (the page that imports the charts is F-07). Measured now with a temporary import: ceiling **+30,000 bytes gzip** for uPlot, its CSS and F-05 code, of which F-05's own code (incl. Svelte's block runtime) **7,600** (v1.4, 200 more bytes from the reserve for the QA r2 fixes and the scroll lock; v1.2: 29,800 / 7,400, 300 bytes from the ADR-011 reserve for the QA r1 fixes; v1.1: 29,500 / 7,100; v1.0: 28,000 / 6,000). Allocation of the rest of the entry: ADR-011 | Bundle impact; ADR-011 |
| D-14 | (v1.2) A throwing cursor listener does not stop delivery to the others: the store catches per listener and reports the error asynchronously. Listeners still must not throw (review rule) | Cursor store; ADR-008 rev. 1 |
| D-15 | (v1.2) `chartHeight` and the chart set (hence the bottom chart) are fixed for one `data` value. A different height needs a new `{#key}` rebuild; F-13 decides if it needs live height changes | Charts |
| D-16 | (v1.4, revises v1.3) `ChartStack` reserves a fixed height only for the **chart area**, a container below the readout and inside `{#key data}`. The height is `chartAreaHeight(chartCount, chartHeight)`, written imperatively (`style.cssText`, never a `style=` attribute) before the first paint, and again for every new `data` value. Nothing jumps when the first resize frame draws, and an in-place `data` swap gets the right height (QA-05-r2-2). The root has no height. F-13 reserves the chart-area space before the bundle runs | Charts |
| D-17 | (v1.4) The readout is in normal flow, wraps between cells and has auto height, so every value is visible on a phone (AP-6, QA-05-r2-1). It is `position: sticky; top: 0` inside the stack, so it stays on screen while the lower charts are scrubbed. The browser lays it out before the first paint, so it causes no jump. It never scrolls sideways | Readout |
| D-13 | (v1.1) Y-axis tick steps never finer than the spec's display precision: `ChartSpec.decimals` drives the y `incrs` (1-2-5 steps from 10^−decimals). X ticks at least 70 CSS px apart (`space: 70`) | Charts |

## Boundary with neighbouring features

| Not in F-05 | F-05 provides instead | Owner |
| --- | --- | --- |
| Map marker and route hover | `CursorStore.subscribe` / `set`; `onready` callback for "load MapLibre after the charts" | F-06 |
| Prototype page, loading the real file, error text (OQ-10), phone gate | `ChartStack` component and `createCursorStore`; the dev page `bench/charts.html` | F-07 |
| Units, pace, cadence label by sport, dark theme | All labels, units, colours and formatters in one place: `lib/charts/specs.ts` and `format.ts` (metric placeholder values) | F-14 |
| Range selection, long-press, click-drag, distance axis | Gesture reducer with a reserved long-press state slot; `decimate` takes any strictly increasing x array | F-17 |
| Page layout, chart height, readout placement | `chartHeight` prop; `chartAreaHeight` for the server-side reservation; readout above the charts, wrapping and sticky (D-17), which F-13 may restyle (for example the sticky offset under a page header) | F-13 |

## Module boundary

```
src/web/
├── src/lib/cursor/                 pure TypeScript, no Svelte, no DOM, imports nothing
│   ├── index.ts                    public surface
│   ├── store.ts                    createCursorStore
│   ├── nearest.ts                  nearestIndex
│   └── *.test.ts
├── src/lib/charts/                 pure TypeScript, no Svelte, no DOM; imports only lib/datafile (index.ts)
│   ├── index.ts                    public surface
│   ├── specs.ts                    CHART_ORDER, ChartSpec, CHART_SPECS, presentCharts(data), readoutFields(data)
│   ├── decimate.ts                 decimate(), bucketCount()
│   ├── format.ts                   formatElapsed(), formatDistance(), per-metric value formatters (metric placeholders until F-14)
│   ├── gesture.ts                  gesture reducer (pointer input → cursor action, touch axis), blocksScroll, TOUCH_SLOP
│   └── *.test.ts
├── src/components/charts/          Svelte 5 + uPlot; the only place that imports 'uplot'
│   ├── ChartStack.svelte           public component: readout + one MetricChart per present stream
│   ├── MetricChart.svelte          one uPlot instance, pointer listeners, cursor line
│   ├── CursorReadout.svelte        values at the cursor
│   ├── uplot-options.ts            buildOptions(spec, data, { width, height (plot area), isBottom }): uPlot Options; type-only import of 'uplot'.
│   │                               May export further pure helpers (e.g. y incrs, chartAreaHeight) for the components and their tests; nothing outside components/charts imports them
│   └── uplot-options.test.ts       Node test (types only, no DOM)
└── bench/
    ├── charts.html, charts.ts      dev-only: browser checks and the 60 fps measurement (not a build entry)
    └── MEASUREMENTS.md             gains a section for F-05
```

**Dependency rules** (in TD Code structure, Dependency rules):

| Module | May import |
| --- | --- |
| `lib/cursor` | nothing |
| `lib/charts` | `lib/datafile` (its `index.ts`, types and constants only) |
| `components/charts` | `lib/cursor`, `lib/charts`, `lib/datafile` (each via `index.ts`), `svelte`, `uplot`, `uplot/dist/uPlot.min.css` |
| `bench/charts.ts` | the above, plus `lib/datafile`'s public surface; never imported by `src/` |

Nothing outside `components/charts` imports `uplot`. Inside it, only the `.svelte` files import the uPlot *value*; `uplot-options.ts` uses `import type`. Later features (F-06 map, F-17 selection) talk to the charts only through stores, never by importing `MetricChart` or reaching into uPlot.

## Cursor store (ADR-008)

`lib/cursor/index.ts` exports exactly:

```ts
export type CursorIndex = number | null;                 // a sample index into WorkoutData arrays, or no cursor
export type CursorListener = (index: CursorIndex) => void;
export type Schedule = (flush: () => void) => void;       // production: f => requestAnimationFrame(() => f())

export interface CursorStore {
  readonly sampleCount: number;
  /** Latest value passed to set (may be newer than the last one delivered). */
  get(): CursorIndex;
  /** index: integer with 0 ≤ index < sampleCount, or null. Same value as get(): no-op.
   *  Anything else throws RangeError (a caller bug). Never calls listeners synchronously. */
  set(index: CursorIndex): void;
  /** Calls listener once, synchronously, with get(); then once per flush in which the
   *  delivered value changed. Returns the unsubscribe function. */
  subscribe(listener: CursorListener): () => void;
}
export type ReportError = (error: unknown) => void;       // default: e => queueMicrotask(() => { throw e; })
export function createCursorStore(sampleCount: number, schedule: Schedule, report?: ReportError): CursorStore;

/** values strictly increasing, length ≥ 1. Index of the value nearest to target; tie → lower index;
 *  target below the first / above the last → 0 / length−1; non-finite target → −1. Binary search, no allocation. */
export function nearestIndex(values: Float64Array, target: number): number;
```

| Rule | Detail |
| --- | --- |
| Value | A sample index (TD Glossary "Cursor"), never a time or a pixel. `null` = no cursor (initial state) |
| Coalescing | `set` stores the value and, if no flush is pending, calls `schedule(flush)` once. `flush` compares the latest value with the last delivered one; if equal, it calls nobody; otherwise it delivers the latest value to every listener in subscription order |
| Listener list | Copy-on-write array: `subscribe`/unsubscribe replace the array, `flush` iterates the array it started with. A listener unsubscribed during a flush may still receive that flush; one subscribed during a flush gets only its synchronous initial call for that value |
| Listener errors (v1.2) | `flush` calls each listener inside its own `try`/`catch`. A throw is passed to `report` and delivery continues with the next listener; the value counts as delivered. The default `report` rethrows in a microtask, so the error still reaches the console and `window.onerror` unchanged. Tests inject a collecting `report`. The subscribe-time initial call is not wrapped: a throw there reaches the subscriber's caller directly. Listeners still must not throw (review rule); the catch keeps one faulty view (e.g. the map in F-06) from freezing the others |
| Writers vs readers | Only input handlers (chart pointer events now, map hover in F-06) call `set`. A listener never calls `set`: that would be a feedback loop between views. If one does anyway, the store stays correct (the value is delivered in the next frame), so this is a review rule, not a runtime check |
| Latency | Pointer events of a frame run before its `requestAnimationFrame` callbacks, so the cursor is normally drawn in the same frame as the input that set it, and never more than one frame later |
| Ownership | One store per `WorkoutData`, created by the page (F-07; bench page in F-05) and passed as a prop. `ChartStack` checks `cursor.sampleCount === data.sampleCount` in `$effect.pre` (before mount and whenever either prop changes) and throws otherwise (a wiring bug). The effect reads only `data.sampleCount` and `cursor.sampleCount`, never `cursor.get()` or `subscribe`, so it never runs on the hot path (v1.1) |
| Svelte | The store happens to satisfy Svelte's store contract. **Never use `$cursor` (or any `$state`/`$derived` fed by the cursor) in a component**: that would put Svelte reactivity on the hot path. QA greps `components/` for `$cursor` |
| F-06 | The map calls `cursor.subscribe(moveMarker)` when it has loaded (it gets the current index immediately, so a late subscriber is in sync) and `cursor.set(i)` on route hover. No change to F-05 code is needed |

## Which charts, in which order

`lib/charts/specs.ts`:

```ts
export type ChartMetric = 'altitude' | 'speed' | 'heart_rate' | 'power' | 'cadence' | 'temperature';
export const CHART_ORDER: readonly ChartMetric[] = ['altitude', 'speed', 'heart_rate', 'power', 'cadence', 'temperature'];
export interface ChartSpec {
  readonly metric: ChartMetric;
  readonly label: string;                 // 'Elevation', 'Speed', 'Heart rate', 'Power', 'Cadence', 'Temperature'
  readonly unit: string;                  // display unit: 'm', 'km/h', 'bpm', 'W', 'rpm', '°C'
  readonly decimals: number;              // display precision of format(); also the finest y tick step, 10^−decimals (D-13)
  readonly toDisplay: (v: number) => number; // §2 unit → display unit (speed × 3.6; others identity)
  readonly format: (v: number) => string; // display value → text; NaN → '–'
  readonly colour: string;                // CSS colour
}
export const CHART_SPECS: Readonly<Record<ChartMetric, ChartSpec>>;
/** Specs in CHART_ORDER for streams present in data with at least one non-NaN value. */
export function presentCharts(data: WorkoutData): readonly ChartSpec[];

export interface ReadoutField {
  readonly key: 'elapsed' | 'distance' | ChartMetric;
  readonly label: string;                 // 'Time', 'Distance', or the chart's label
  readonly unit: string;                  // '' for elapsed, 'km' for distance
  readonly text: (index: number) => string; // value at a sample index, formatted; '–' for a missing sample
}
/** Readout fields in order: elapsed, distance (if the stream is present), then presentCharts(data).
 *  Built once per WorkoutData; text() reads the full-resolution arrays (D-9). */
export function readoutFields(data: WorkoutData): readonly ReadoutField[];
```

| Rule | Detail |
| --- | --- |
| Order | `CHART_ORDER` (AP-4). It differs from `METRIC_ORDER` (cadence and power swap; distance is not charted). Do not derive one from the other |
| Shown | Key present in `data.metrics` **and** at least one value is not `NaN`. A present column that is entirely missing gets no chart (an empty chart says nothing) |
| Distance | Never a chart. Shown in the readout only. It becomes an x axis in F-17 |
| Display units | Metric placeholders until F-14: elevation m (0 decimals), speed km/h (1), heart rate bpm (0), power W (0), cadence rpm (0), temperature °C (0). F-14 replaces `CHART_SPECS` by a function of the unit system and sport; nothing else changes |
| Missing value | `format(NaN)` → `'–'` (U+2013). Never `'0'` |

## Breaks and decimation

**Vocabulary.** A *segment* is a maximal run of consecutive samples of one stream that are not missing and do not cross a gap. A *break* is where the line is not drawn: between two segments. Missing samples and gaps both cause breaks; nothing is ever drawn as zero (feature scope).

`lib/charts/decimate.ts`:

```ts
export interface Decimated { readonly x: number[]; readonly y: (number | null)[] }   // uPlot AlignedData parts
/** x strictly increasing (no NaN), y with NaN = missing, gaps = WorkoutData.gaps,
 *  [xMin, xMax] = the axis range, buckets ≥ 1. Pure; allocates only the result. */
export function decimate(x: Float64Array, y: Float64Array, gaps: Uint32Array,
                         xMin: number, xMax: number, buckets: number): Decimated;
/** Pixel columns of a plot: Math.round(plotWidthCss × devicePixelRatio); 0 if the plot has no width. */
export function bucketCount(plotWidthCss: number, devicePixelRatio: number): number;
```

**Algorithm** (one pass over i = 0 … N−1, a second index walks `gaps`):

1. `bucket(i) = min(B−1, floor((x[i] − xMin) / (xMax − xMin) × B))`; if `xMax === xMin`, bucket 0.
2. If i is the next gap index: flush the open run, then emit a break at `x = (x[i−1] + x[i]) / 2`.
3. If `y[i]` is `NaN`: flush the open run, then emit a break at `x = x[i]`. Go to the next i.
4. If a run is open and `bucket(i)` equals its bucket: update its min and max (strict `<`/`>`, so the first occurrence wins).
5. Otherwise flush the open run and open a new one at i.
6. **Flush** emits the run's points in index order: one point if min and max are the same sample, else two (`(x[minIdx], y[minIdx])`, `(x[maxIdx], y[maxIdx])` ordered by index).
7. A break is a point `(x, null)`. It is emitted only if at least one data point has been emitted and the last emitted point is not already a break. A trailing break is removed at the end.

| Property (each one is a test) | Why |
| --- | --- |
| `x` strictly increasing | uPlot requires sorted x; break x lies strictly between neighbours (gap ≥ 5 s; a missing sample's own time) |
| `y` never contains `NaN`; `null` only at breaks | uPlot draws `null` as a break with `spanGaps: false` (default); `NaN` behaviour is undefined |
| Per bucket and segment, the emitted min and max equal a brute-force min and max | Envelope is exact |
| Number of breaks = number of segments − 1 | Every missing run and every gap with values on both sides is visible |
| Length ≤ 2 × B + 3 × segments, and ≤ 2 × (valid samples) | Bounded by pixels, not by N: at most B + S − 1 runs of ≤ 2 points, plus S − 1 breaks. The second bound covers pathological alternating missing samples |
| All `NaN` → empty arrays | `presentCharts` hides that chart anyway |

**Where it runs.** On mount and on every width change, once per chart, on the main thread: 6 × 43,200 samples is a few milliseconds and never on the scrubbing path. `toDisplay` is applied to the decimated `y` (at most a few thousand values), so a unit switch in F-14 re-maps without re-decimating; a monotonic decreasing transform (pace) keeps the envelope.

**What the viewer sees.** Elapsed time on a linear axis (decision 5): a multi-hour pause is an empty stretch with no line (AC 4). A sensor dropout is a hole in that chart's line only (AC 3). A dropout shorter than one pixel column is a hole of at most one pixel; that is correct, not a bug. A segment that decimates to a single isolated point is not drawn (uPlot draws no line through one point, and series points are off); accepted, at most one pixel column wide.

**Do not** fill, interpolate, or connect across breaks; do not use uPlot's `spanGaps`; do not pass `WorkoutData` arrays to uPlot directly.

## Charts (ADR-009)

One uPlot instance per `ChartSpec`, created in `MetricChart.svelte`. Options built by `uplot-options.ts`:

| Option | Value | Why |
| --- | --- | --- |
| `cursor` | `{ show: false }` | uPlot binds no mouse listeners then; we own interaction (ADR-009) |
| `legend` | `{ show: false }` | Our readout replaces it |
| `select` | `{ show: false }` | F-17 owns selection |
| `scales.x` | `{ time: false, range: () => [time[0], time[N−1]] }`; if N = 1, `[time[0], time[0] + 1]` | Elapsed seconds; **identical range on every chart** (a stream that starts late must not shift its plot) |
| `padding` | `[8, 16, 0, 0]` (top, right, bottom, left, CSS px) on **every** chart | uPlot's default auto padding adds right padding only when a chart has a sized bottom axis (`uPlot.esm.js:3801-3813`, 1.6.32), so the bottom chart's plot would be narrower than the others. Fixed padding keeps D-6 |
| `scales.y` | auto range | Per chart |
| `series[1]` | `{ stroke: spec.colour, width: 1.5, points: { show: false }, spanGaps: false }` | Breaks at `null` |
| `axes[0]` (x) | Ticks at `incrs: [60, 300, 600, 900, 1800, 3600, 7200, 10800, 21600]` s, at least `space: 70` CSS px apart (v1.1: at uPlot's default 50, labels like `10:30:00` nearly touch), labels `formatElapsed` (`h:mm:ss`, or `m:ss` under 1 h). Bottom chart: `size: 28`, tick labels. Every other chart: `size: 0`, no ticks, empty-string labels, grid lines kept | Readable elapsed time; compact stack. The bottom chart is 28 px taller; its plot area is not |
| `axes[1]` (y) | `size: 48` (fixed, CSS px) on every chart, labels via `spec.format`; `incrs` = the 1-2-5 sequence from 10^−`spec.decimals` up to 10^6 (e.g. decimals 0: 1, 2, 5, 10, 20, 50, …; decimals 1: 0.1, 0.2, 0.5, 1, …), built once per spec (v1.1) | Same plot left edge in every chart, so one pointer x means the same time everywhere. A step finer than the display precision would give duplicate labels ("1", "1", "2") |
| Size | width = stack content width; plot area height = `chartHeight` (prop, default 120 CSS px) for every chart, read once per `data` value (D-15: changing it without a new `data` has no effect; this is documented on the prop). `buildOptions`' `height` parameter is this **plot-area** height; the builder adds the top padding (8) on every chart and the x-axis size (28) on the bottom chart, and `MetricChart` passes the same total to `setSize` | F-13 may change the height |
| Title | HTML row above each chart: `{label} · {unit}` | No uPlot axis label (saves width) |

**Layout constants (v1.2, v1.4).** The y-axis width (48), top padding (8), x-axis size (28) and title row height (16) are TypeScript constants in `uplot-options.ts`, each defined once. CSS that needs one reads a custom property (`--y-axis-width`, `--title-row-height`) that `ChartStack` writes once on its root element in `onMount`, with one `cssText` assignment. No CSS literal repeats a value that uPlot also uses. (v1.4) The readout has no fixed height any more: `READOUT_ROW_HEIGHT` and `--readout-row-height` are deleted.

**Chart-area height (D-16, v1.4; replaces `stackHeight`).** `uplot-options.ts` exports `chartAreaHeight(chartCount, chartHeight)`: per chart (title row + `chartHeight` + 8), plus 28 for the bottom chart; 0 for 0 charts. Markup of `ChartStack`:

```
div.chart-stack  (root: width only, observed by the ResizeObserver; no height)
└── {#key data}
    ├── CursorReadout                      normal flow, auto height, sticky (D-17)
    └── div.chart-area  bind:this={area}   height = chartAreaHeight(charts.length, plotHeight)
        └── {#if drawable} {#each charts} MetricChart {/each} {/if}
```

- `area` is a `$state` variable filled by `bind:this`. One `$effect` reads `area` and the derived height and, when `area` is set, assigns `area.style.cssText = 'height:' + h + 'px'`. The effect runs after the block's DOM exists and within Svelte's microtask flush, so the height is in place before the first paint. It runs again for every new `data` value, because `{#key}` creates a new `div.chart-area`. That fixes QA-05-r2-2 without a separate rule for in-place swaps.
- The chart area reserves its height also while `drawable` is false (as the stack did in v1.3), so nothing jumps when the width first allows drawing.
- **No `style=` or `style:` attributes in `components/charts`** (v1.3). Svelte's style runtime costs about 790 bytes gzip (measured), and F-05 does not need it; a later feature that needs it pays for it from its ADR-011 share. No `use:` action for this either; the effect is enough.
- **F-13** reserves the chart-area height in the server-rendered HTML, before the bundle runs, and may reuse the formula. The readout's height comes from normal flow; how F-13 renders it before the bundle runs is F-13's design.

**Cursor line.** One absolutely positioned `div` per chart, appended to `u.over`, full height, `will-change: transform`. Position: `translateX(u.valToPos(data.time[i], 'x'))` (CSS px). Hidden (`visibility: hidden`) when the index is `null`. The line is drawn in every chart, also where that chart's value is missing.

**Resize and rotation (AC 6).** `ChartStack` observes its own content width with one `ResizeObserver`. A width change is coalesced to one `requestAnimationFrame`, then each chart runs `u.setSize({ width, height })` (this computes `u.bbox` synchronously). It then calls `bucketCount(u.bbox.width / uPlot.pxRatio, uPlot.pxRatio)` and `decimate`, then `u.setData(...)`. Finally it re-places its cursor line from `cursor.get()` and invalidates its cached plot rectangle. The first mount runs the same sequence. The plot width is never read from the DOM. The chart area's height never changes on resize; the readout's may, when it re-wraps (v1.4). The observer then also fires for the root's height change; with an unchanged width, `layout` returns early, so nothing redraws. A `devicePixelRatio` change without a width change keeps the old bucket count until the next resize (accepted).

**Lifecycle.** `onMount` creates the instance, the listeners and the subscription; the cleanup unsubscribes, removes every listener with the same options it was added with (including `capture` and `passive`) and calls `u.destroy()`. A new `data` prop on the same `ChartStack` is supported: everything inside `{#key data}` (readout, chart area and its height, charts) is rebuilt; the page passes a new cursor store with it. The root element and its `ResizeObserver` stay.

**`onready`.** `ChartStack` calls its optional `onready()` prop exactly once, in the first animation frame after the first width measurement has been processed: after every chart has received its first `setData`, or right away if there are no charts or the width gives 0 buckets (v1.1). F-06/F-07 start the MapLibre `import()` from it (F-06 AC 1); the map must never wait for charts that will not draw.

## Interaction (ADR-010, review decision 6)

**Listeners (v1.4).** On each chart's `u.over`:

| Listener | Options | Why |
| --- | --- | --- |
| `pointerenter`, `pointerdown`, `pointermove`, `pointerup`, `pointercancel`, `pointerleave` | `{ passive: true }` | All cursor input; never call `preventDefault` |
| `touchmove` | `{ passive: false }` | The scroll lock (D-18). The only non-passive listener and the only `touch*` listener |
| `scroll` on `window` | `{ passive: true, capture: true }` | Marks the cached rect stale |

All are added once in `onMount`, before any touch can start, and removed in the cleanup with the same options. **Never add the `touchmove` listener on `pointerdown` or `touchstart`:** iOS WebKit decides at touch start whether a blocking listener exists, so one added during the touch does not stop scrolling ([WebKit bug 184251](https://bugs.webkit.org/show_bug.cgi?id=184251)). No `mouse*` listeners and no `on:`/`onpointer…`/`ontouch…` attributes in markup, so there are no a11y warnings under F-01 D-8. CSS on `u.over`: `touch-action: pan-y pinch-zoom; user-select: none; -webkit-user-select: none` (unchanged).

**Pointer x → index:** `x = e.clientX − rect.left` with a cached `rect` of `u.over` (refreshed on `pointerenter`, `pointerdown`, after a resize, and marked stale by the `scroll` listener); `t = u.posToVal(x, 'x')`; `i = nearestIndex(data.time, t)`; ignore `i === −1`. On a tie the earlier sample wins (decision 5).

**Gesture reducer** (`lib/charts/gesture.ts`, pure): `step(state, input) → { state, action }`, where `input = { kind: 'down' | 'move' | 'up' | 'cancel' | 'leave', pointerType, pointerId, isPrimary, index, current, x, y }` (`current` = `cursor.get()`; v1.4: `x`, `y` = `clientX`, `clientY` in CSS px) and `action = { kind: 'none' } | { kind: 'set', index }`. Inputs from non-primary pointers are ignored. State is `idle` or (v1.4) `touch { pointerId, before, x0, y0, axis }` with `axis: 'open' | 'scrub' | 'pan'`; F-17 adds a `pressing`/`selecting` state for long-press without changing the existing rows.

| Pointer | Input | Action | Next state |
| --- | --- | --- | --- |
| mouse or pen | down | set(index) | idle |
| mouse or pen | move (with or without buttons) | set(index) | idle |
| mouse or pen | up, cancel, leave | none (cursor stays, D-8) | idle |
| touch | down | set(index) (v1.4, P3 option B: the cursor jumps under the finger) | touch { pointerId, before: current, x0: x, y0: y, axis: 'open' } |
| touch | move, same pointerId | set(index) | touch; if `axis` is `'open'` and the slop is reached, `axis` = `classify(x − x0, y − y0)` (below) |
| touch | up, same pointerId | set(index) (a tap places the cursor) | idle |
| touch | cancel, same pointerId (the browser took the gesture: scroll or pinch) | set(before) (undo down and moves; with `before` `null`, set(null)) | idle |
| touch | leave (fires after up or cancel) | none | unchanged |
| touch | down, primary, other pointerId, while in `touch` (v1.1) | set(index) (v1.4) | touch { new pointerId, before: current, x0, y0, axis: 'open' } |
| touch | any other input with another pointerId | none | unchanged |

Every touch move sets the cursor, whatever the axis. A touch classified `pan` that the browser does not scroll (Chrome treats 30–45° as horizontal, see below) therefore still scrubs, only without the lock.

**Direction rule (D-18).** Exported from `gesture.ts`:

```ts
export const TOUCH_SLOP = 8;   // CSS px from the touch-down point
/** The axis is decided on the first touch move with dx² + dy² ≥ TOUCH_SLOP², and never changes afterwards.
 *  scrub: within 30° of horizontal (3·dy² ≤ dx², boundary inclusive); pan: everything else. */
/** True only while one finger is scrubbing: state is touch with axis 'scrub', and touchCount === 1. */
export function blocksScroll(state: GestureState, touchCount: number): boolean;
```

The `touchmove` handler in `MetricChart` is the whole lock. It allocates nothing, reads no layout and never calls `set`:

```ts
const onTouchMove = (e: TouchEvent): void => {
  if (e.cancelable && blocksScroll(state, e.touches.length)) e.preventDefault();
};
```

| Question | Answer |
| --- | --- |
| Why 8 px | Small enough that a horizontal start is classified before the browser starts a scroll of its own, large enough to ignore finger jitter on touch down. Assumed, not measured: P11 and P12 check it |
| Why 30°, not 45° | It biases ambiguous diagonals toward scrolling. A missed lock costs a retry, and `pointercancel` restores the cursor. A wrong lock traps the page under the finger, which is worse on a phone, where charts fill the screen |
| Which `touchmove` can still be cancelled | One with `cancelable === true`. The browser clears the flag once it has started scrolling: Chrome after its own slop if the direction is allowed, WebKit once its pan has begun. Before that point every `touchmove` can be cancelled, not only the first, and preventing each one keeps the browser from starting a scroll later. We call `preventDefault` on **every** `touchmove` of a scrub, not once. If the flag is false, we do nothing: the browser scrolls, sends `pointercancel`, and the cursor is restored. Checking the flag also avoids Chrome's "Ignored attempt to cancel a touchmove event with cancelable=false" warning |
| Event order | Chrome and WebKit dispatch a touch's pointer events before its compatibility touch events, so the `pointermove` that decides the axis has already run when the `touchmove` for the same movement arrives. If an engine reversed the order, the lock would start one event later, still before a horizontal start can turn into a scroll. Assumption; P11 checks it |
| Android Chrome | Chrome checks `touch-action` once, when the touch leaves its slop. If the direction is not allowed (`pan-y` with a horizontal start), it does not scroll for the rest of that touch. The lock is redundant there but harmless |
| iOS WebKit | The operator's run (P2) shows WebKit starting a scroll in the middle of a horizontal drag once it drifts vertically, and sending `pointercancel`. The lock stops exactly that |
| Pinch (P5) | `blocksScroll` needs exactly one touch. When a second finger lands, `touches.length` is 2 and nothing is prevented, so the browser may pinch-zoom; it sends `pointercancel` for the primary pointer, and the cursor is restored. Non-primary pointers stay ignored by the reducer |
| Passive-listener rule | v1.0 had "all listeners passive". v1.4 makes one exception, `touchmove` on `u.over`. **Cost:** for a touch that starts on a chart, the browser now waits for the main thread before it scrolls (Chrome: the first `touchmove`; WebKit: synchronous dispatch). The handler does two comparisons, so it only stalls when the main thread is busy (decode, MapLibre start-up in F-06). The F-06 and F-07 phone checks watch for a sticky first scroll on charts. Scrolls that start outside the charts are unaffected |
| P2 (user decision) | One drag need not continue onto another chart. With the lock, a scrub that drifts onto a neighbouring chart is no longer taken over as a scroll. Touch events and the implicit pointer capture stay with the start chart, whose x mapping equals every other chart's (D-6), so P2 will probably pass now. If it does not, no further work |

The v1.1 row: a pointer is primary only when no other touch is active, so a primary `down` with a new ID proves the old touch ended without us seeing its `up` or `cancel` (lost capture, element re-created). It replaces the stale touch instead of ignoring the new one, so a chart can never get stuck ignoring touches, or stuck blocking scrolls (the new touch starts with axis `open`). Implicit capture makes this rare; the rule makes it impossible.

**Why this meets decision 6 and the user's touch decisions (v1.4).**
- A vertical swipe that starts on a chart: `touch-action: pan-y` lets the browser scroll it and send `pointercancel`, and the classification gives `pan`, so nothing is prevented (AC 5, P1). The cursor jumped under the finger on touch down and returns on `pointercancel` (P3 option B).
- A horizontal drag: the browser does not pan it, `pointermove` keeps scrubbing, and from 8 px on the lock keeps any later vertical drift from turning into a scroll (user decision 2).
- A pinch: never prevented (P5).
- Desktop: hover moves the cursor. Click-drag is reserved for F-17 and moves the cursor until then.

## Readout

`CursorReadout.svelte`, above the charts. One cell per `readoutFields(data)` entry, in its order: elapsed time (`formatElapsed(data.time[i])`), distance (`formatDistance`: km, 2 decimals) if `distance` is present, then one cell per present chart in `CHART_ORDER`: `{label} {format(toDisplay(data.metrics[m][i]))} {unit}`. Values come from the full-resolution arrays at index i (D-9). With a `null` cursor every value cell shows `–`. The component decides nothing about which values exist; `readoutFields` does, and it is unit-tested.

No `aria-live` on the readout: it would be announced up to 60 times per second. Accessible readout and keyboard scrubbing are not in AP-6. If they are wanted, they belong in F-13.

The component renders its cells once (Svelte markup), keeps `bind:this` references to the value text nodes, and its cursor listener assigns `textContent` directly. Value cells have a `min-width` in `ch` sized for the common case (elapsed `10:00:00` → 8ch, distance `999.99` → 6ch, metric values to their usual digits), `white-space: nowrap`, no `overflow: hidden` and no fixed `width`. They use `font-variant-numeric: tabular-nums`. Common values never change a cell's width. Rare long values (≥ 100 h, ≥ 1,000 km) widen their cell instead of being clipped (v1.2, QA-05-r1-1).

**Layout (v1.4, D-17; replaces the v1.3 fixed-height, sideways-scrolling row).**

| Property | Value | Why |
| --- | --- | --- |
| Flow | `display: flex; flex-wrap: wrap; column-gap: 16px; row-gap: 2px`; cells `flex: none; white-space: nowrap` | Wraps between cells, never inside one. On a 375 px phone, ride-12h's 8 cells take about 4 lines (about 80 px), and all 8 values are visible (AP-6) |
| Height | Auto: no `height`, no `overflow-x`/`overflow-y` | The browser lays it out before the first paint, so it causes no jump. No scrollbar can cover values |
| Position | `position: sticky; top: 0; z-index: 1`, opaque background (`#fff` until F-14's theme) | On a phone the stack is taller than the screen. Without this, scrubbing the lower charts would push the readout off-screen and fail AP-6 in practice. It sticks only inside the stack (its parent), so it never covers content below the stack |
| Containment | `contain: layout` (kept) | Value text changes do not lay out the rest of the page |
| Padding | `0 16px 0 var(--y-axis-width)` (unchanged) | Aligns with the plots; F-13 may change it |

The readout's height changes only when its line count changes: on a width change (resize or rotation, which redraws anyway), or when a rare long value widens a cell during scrubbing. The second case may move the charts by one line; accepted, because it needs ≥ 100 h or ≥ 1,000 km. F-13 owns the final readout layout.

## Performance (NFR: 60 fps on a 12-hour workout)

**Hot path** = everything a cursor flush runs: the store's `flush`, every chart's `moveLine`, the readout listener (and F-06's marker later). Rules (D-10), checked by QA in review:

- No `new`, array or object literals, closures, spread, or `Array` methods that allocate. The only allocations allowed are short strings: the readout's formatted values (at most 8 per flush) and one `style.transform` string per chart (at most 6), i.e. at most 14 short strings per frame. Those die young and cost nothing measurable; the 60 fps measurement is the check. Formatters (`Intl.NumberFormat` if used) are created once, at module load.
- No Svelte reactivity: no `$state` writes, no `$cursor`.
- No layout reads (`getBoundingClientRect`, `offsetWidth`, `clientX`-to-rect conversion uses the cached rect).
- No canvas redraw: no `setData`, `setSize`, `redraw`, `setCursor` or `setScale` on the hot path.
- DOM writes only: one `style.transform` (and `visibility` when crossing `null`) per chart, `textContent` per readout cell.
- (v1.4) The `touchmove` handler is on the input path, not the flush, but it blocks scrolling on the main thread, so it follows the same rules: no allocation, no layout read, no `set`, nothing but `cancelable`, `blocksScroll` and `preventDefault`.

**Measurement (AC 7, desktop).** On `bench/charts.html` (see Dev page):

| Item | Value |
| --- | --- |
| File | `ride-12h.bin` (43,200 samples, all six charts) |
| Window | Desktop browser, page width ≥ 1,200 CSS px, DevTools closed for the auto-scrub run |
| Auto-scrub | Button "Auto-scrub 10 s": dispatches a synthetic `pointermove` (`pointerType: 'mouse'`, `isPrimary: true`, `bubbles: true`) on the middle chart's `u.over` every 8 ms, sweeping the plot left → right → left at constant speed for 10 s |
| Frame monitor | A `requestAnimationFrame` loop running only during the window, writing frame intervals into a preallocated `Float64Array` (no allocation per frame) |
| Flush time | The bench subscribes one listener to the cursor **before** mounting `ChartStack` and one **after**; listeners run in subscription order, so the difference of their `performance.now()` is the time spent in the charts and the readout. Stored in a preallocated array |
| Shown | Frames, interval median, p95, max; share of intervals ≤ 20 ms; flush median, p95, max; N; charts; `devicePixelRatio`; plot width; `navigator.userAgent` |
| **Pass** | ≥ 99 % of frame intervals ≤ 20 ms, **and** flush p95 ≤ 2 ms and max ≤ 8 ms |
| Confirmation | One manual 10 s hover scrub recorded in the DevTools Performance panel: no long task (> 50 ms) and no dropped frames in the Frames track during the scrub |
| Information only | The same auto-scrub with DevTools CPU throttling 4× (a rough phone proxy; the phone run is F-07) |
| Record | New section "F-05 scrubbing" in `bench/MEASUREMENTS.md`: Date, Commit, Device, OS, Browser and version, File, Charts, Plot width, DPR, Throttling, Frames, Interval median/p95/max ms, Share ≤ 20 ms, Flush median/p95/max ms, Pass |

20 ms (not 16.7) absorbs timer jitter at 60 Hz; on a faster display the same bound still means "at least 60 fps". Vitest asserts no timings.

## Dev page `bench/charts.html`

Dev-only, like F-04's bench page: served by `npm run dev` at `/bench/charts.html`, not a build entry (the manifest still lists only `index.html`), never imported by `src/`. It replaces the prototype page for F-05's browser checks; F-07 builds the real one.

- A file selector over synthetic golden files (`?url` imports, `server.fs.allow` already covers the folder): `ride-12h`, `ride-4h`, `indoor-no-gps` (no altitude), `no-distance-with-gps` (no speed, no power), `brick-run-bike` (power missing for the run part), `pause-hours` (multi-hour gap), and **"ride-4h with heart-rate dropout"**: `ride-4h` decoded, then a copy of `heart_rate` with `NaN` at indices 3,600–4,199, built in the page (synthetic, no file).
- Mounts `ChartStack` with a cursor store from `createCursorStore(N, f => requestAnimationFrame(() => f()))`.
- A "second subscriber" line showing the delivered index, standing in for F-06's map.
- The auto-scrub, frame monitor and flush timing above.
- Add `"bench/**/*.svelte"` to `tsconfig.json` `include` only if the page uses a Svelte file; `charts.ts` with `mount(ChartStack, …)` is enough.

## Bundle impact (ADR-002)

- **The counted entry does not change in F-05**: `main.ts` still mounts the F-01 placeholder; the prototype page that imports `ChartStack` is F-07. The CI line stays `10613 / 60000`; QA checks it.
- **Measured now**, as in F-04: build once with a temporary `import ChartStack from './components/charts/ChartStack.svelte'; import { createCursorStore } from './lib/cursor'; console.log(ChartStack, createCursorStore);` in `main.ts`, and once with only `import uPlot from 'uplot'; import 'uplot/dist/uPlot.min.css'; console.log(uPlot);`. Report both `entry index.html` gzip totals against 10,613, then revert.
- **Ceilings (v1.4):** whole F-05 import ≤ **+30,000 bytes gzip** over 10,613; F-05's own code (first number minus the uPlot-only number) ≤ **7,600**. Above either: stop and report. Before the v1.4 changes, QA r2 measured +29,624 / 7,265. The extra 200 bytes come from the ADR-011 reserve (2,400 → 2,200, ADR-011 rev. 2). They pay for the axis classification and `blocksScroll` in the reducer, the `touchmove` listener, the chart-area `$effect` and the sticky readout CSS. The deleted readout height, overflow rules and custom property pay some of it back. (v1.2: +29,800 / 7,400, 300 bytes from the reserve for the QA r1 fixes.) Fixes in later QA rounds of F-05 come from the same reserve, by a new design version, never by silently exceeding.
- **Why raised (v1.1).** v1.0 had +28,000 / 6,000. Measured: uPlot + CSS +22,359, all of F-05 +29,425, own code 7,066 after two trims (~950 bytes). The v1.0 estimate left out Svelte's block runtime (`{#each}`, `{#if}`, `{#key}`, `bind:this`, about 3 KB), which the F-01 placeholder never loaded. That runtime is a one-time cost per entry: F-13 (layout) and F-18 (tables) need the same blocks, so writing F-05 imperatively (no Svelte blocks) would save it only until F-13 and would make the readout and stack harder to change. Shaving our own code cannot recover 1,066 bytes without hurting clarity. The 60,000 limit (ADR-002) is unchanged; only F-05's own stop trigger moves.
- **After F-05 (v1.1 note):** measured 29,430 / 29,500 and 7,071 / 7,100 before QA r1. These ceilings are F-05's own stop trigger and close when F-05 is done. Later changes to `lib/cursor`, `lib/charts` or `components/charts` (F-06, F-07, F-14, F-17) count against the changing feature's ADR-011 share, measured as the entry total before and after.
- **Projection after F-07:** 10,613 + 2,793 (`lib/datafile`, F-04 r1) + 29,425 ≈ **42,800** of 60,000, leaving about **17,200** (v1.2: up to about 43,200 and 16,800 after the QA r1 fixes). How that remainder is shared, and what must load lazily, is decided in [ADR-011](ADR-011-activity-page-bundle-allocation.md).
- No new runtime dependency: `uplot` 1.6.32 is already in `package.json` (F-01). v1.2: one new dev dependency, `jsdom`, for component smoke tests only (ADR-012); no Playwright. Dev dependencies never enter the bundle.

## Tests

All Vitest, synthetic data only. `lib/` and `uplot-options.test.ts` run in Node (F-01 D-6); `*.dom.test.ts` files under `components/` run in jsdom (ADR-012). Two pieces of setup this needs (v1.3):
- `vite.config.ts` sets `resolve.conditions: ['browser']` only when `process.env.VITEST` is set, so Svelte resolves to its client build under Vitest (Svelte's documented setup). `vite build` is unaffected; QA checks the CI size line is unchanged.
- `test-support/golden.mjs` builds file paths with `path.resolve(import.meta.dirname, …)`, not `new URL(…, import.meta.url)`, because jsdom's global `URL` breaks `node:fs`.

Data comes from: arrays built in the test (seeded PRNG where random), `lib/datafile/testing/build-file.ts`, or committed synthetic golden files through `test-support/golden.mjs`.

| File | Must cover |
| --- | --- |
| `lib/cursor/store.test.ts` | Subscribe calls the listener synchronously with `null` initially and with the current value later. `set` calls no listener synchronously; many `set`s before a flush cause exactly one `schedule` call and one delivery of the latest value; setting the current value schedules nothing; set 5 then back to 4 (delivered 4) within a frame → no delivery; `null` delivered; `RangeError` for −1, N, 1.5, `NaN`, `Infinity`; nothing thrown for 0 and N−1; unsubscribe stops delivery; unsubscribing another listener during a flush does not skip later listeners of that flush; subscribing during a flush gives one initial call and no second call for the same value; delivery order = subscription order; `get` returns the latest value before the flush. Scheduler is a manual queue in the test. **v1.2 (QA-05-r1-2), each written so it fails against the naive implementation named in brackets:**
- set 3, flush, set 7, then subscribe before the next flush: the initial call gets 7, not 3 [initial call with the last *delivered* value];
- listeners A, B, C; A unsubscribes itself during a flush: B and C still get that value [`splice` on the live array];
- A unsubscribes B during a flush: C still gets it [same].

**Listener errors (v1.2, D-14):** A throws, B and C still get the value in that flush, and the injected `report` gets A's error once; the next flush goes to A again; no `report` call when nobody throws. **(v1.4, QA-05-r2-3 d)** A throws on the delivery of 5; then `set(null)` and flush: A and B both receive `null` [resetting `delivered` to `null` after an error] |
| `lib/cursor/nearest.test.ts` | Exact hit; between two values (closer wins); exact midpoint → lower index; below first → 0; above last → N−1; N = 1; `NaN`, `±Infinity` → −1; values with a large gap (`[0, 1, 2, 10000, 10001]`: target 5000 → 2, 5001 (the midpoint, a tie) → 2, 5002 → 3); 1,000 seeded random targets on a 43,200-element strictly increasing array equal a linear-search reference |
| `lib/charts/decimate.test.ts` | Every property in the table under "Breaks and decimation", on seeded random data (random NaN runs, random gaps, B in {1, 7, 640, 2,400}, N in {1, 2, 3, 1,000, 43,200}). Hand cases: NaN run in the middle → exactly one `null` at the first missing sample's time; gap → `null` at the midpoint; NaN directly at a gap → one break; leading and trailing NaN → no leading or trailing break; all NaN → empty; a stream starting late keeps `xMin`/`xMax` from the arguments (first emitted x > xMin is fine); the last sample lands in bucket B−1; B > N; `y` never contains `NaN` and never a `0` where the input was `NaN`. Golden `pause-hours`: for each charted stream, at least one `null` lies inside the multi-hour gap. `bucketCount`: rounding, 0 width → 0 |
| `lib/charts/specs.test.ts` | `CHART_ORDER` literal; `presentCharts`: all streams → six specs in order; without power → five, no power; without altitude → no elevation; distance present → never a spec; a present all-`NaN` stream → no spec; time only → empty. Golden files: `ride-12h` → 6; `indoor-no-gps` → no altitude; `no-distance-with-gps` → no speed, no power. `readoutFields`: order elapsed, distance, then `presentCharts`; no distance field without `distance`; `text(i)` equals the formatted full-resolution value at i (not a decimated one); a `NaN` sample → `–`, never `0`; on `ride-12h` six chart fields plus elapsed and distance |
| `lib/charts/format.test.ts` | `formatElapsed`: 0 → `0:00`, 59 → `0:59`, 3,599 → `59:59`, 3,600 → `1:00:00`, 43,199 → `11:59:59`, 360,000 → `100:00:00`. `formatDistance`: 0 → `0.00`, 12,345.6 m → `12.35`, `NaN` → `–`. Each spec: `toDisplay` (speed 10 m/s → 36 km/h), `format` decimals equal `spec.decimals`, `format(NaN)` → `–`, never `0` for `NaN` |
| `lib/charts/gesture.test.ts` | Every row of the gesture table, including: touch down then cancel restores `before` (also `null`); (v1.4) touch down sets the cursor to the index under the finger and records `before` = current, `x0`, `y0`, axis `'open'`; touch down, move, move, up → sets on down, on each move and on up; a second touch pointer is ignored throughout; mouse leave and touch leave do nothing; non-primary inputs ignored; (v1.1) in `touch` state, a primary `down` with a new pointerId gives state `touch` with the new ID, `before` = current and axis `'open'`, sets the cursor (v1.4), and later moves of the new ID set the cursor while moves of the old ID are ignored. **Direction (v1.4, D-18):** moves with dx² + dy² < 64 keep axis `'open'`; the first move at (8, 0) → `'scrub'`, (0, 8) → `'pan'`, (−8, 0) → `'scrub'`; the 30° boundary (dy = dx / √3 is not exactly representable): (8, 4.61) → `'scrub'`, (8, 4.62) → `'pan'`, and the same with negative dx and dy; once `'scrub'`, a later move straight down stays `'scrub'`; once `'pan'`, a later horizontal move stays `'pan'`; mouse moves never create an axis. `blocksScroll`: true only for touch + `'scrub'` + touchCount 1; false for `idle`, `'open'`, `'pan'`, and for touchCount 0 or 2 |
| `components/charts/uplot-options.test.ts` | `uplot-options.ts` imports only *types* from `uplot` (`import type`), so it runs in Node. For every spec of `ride-12h`, and with `isBottom` true and false: `cursor.show`, `legend.show`, `select.show` false; `padding` equals `[8, 16, 0, 0]`; y axis `size` 48; x `range()` returns `[time[0], time[N−1]]` for every chart, including a stream that starts late; `[t, t + 1]` for N = 1; series `spanGaps` false and `points.show` false; x axis `size` 28 only on the bottom chart, else 0. (v1.1) x axis `space` 70; y `incrs` is strictly increasing, its smallest step equals 10^−`spec.decimals` (1 for decimals 0, 0.1 for speed), and every step is a 1-2-5 multiple; the total uPlot height equals `height` + 8 (top padding) on every chart, plus 28 on the bottom chart, so the bottom chart is exactly 28 px taller (v1.1, corrected). (v1.4, replaces the `stackHeight` checks) `chartAreaHeight(0, h)` is 0; `chartAreaHeight(n, h)` equals n × (title row + h + 8) + 28; it equals the sum of the per-chart `outerHeight` values plus n title rows for n = 1 and n = 6 |
| `components/charts/*.dom.test.ts` (v1.2, ADR-012) | The engineer's jsdom smoke test, committed and run in `make ci`. Prefer real uPlot; the test may stub canvas `getContext` and `Path2D` (one canvas stub; jsdom lacks `Path2D` and uPlot needs it), `ResizeObserver`, `requestAnimationFrame` and `matchMedia`, and stub uPlot only where jsdom cannot run it (say which, in a comment). At least these checks:
- the wiring check throws on a sample-count mismatch;
- (v1.4) the chart area's `style.height` equals `chartAreaHeight(n, chartHeight)` before the first resize frame, and the root has no `style.height`;
- (v1.4, QA-05-r2-2) **in-place swap:** mount ride-12h (6 charts), run the resize frame, then replace `data` (and the store) on the same `ChartStack` with no-distance-with-gps (3 charts), and back. After each swap the chart area's height is `chartAreaHeight` of the new chart count. The swap needs a test-only wrapper that holds `data` and `cursor` in `$state.raw`: a `.svelte` file next to the test, or a `*.svelte.ts` helper. It is never imported by non-test code;
- `onready` fires exactly once, also with no charts and with width 0;
- resize order: `setSize` before `setData`, with the bucket count from `bbox`;
- pointer events go through the gesture reducer into `cursor.set`;
- (v1.4, QA-05-r2-3 a) **touch tap:** a touch `pointerdown` at one x and a `pointerup` at a *different* x (same pointerId) leave the cursor at the up position's index (with down setting the cursor, an up at the same x would not catch a lost `up` mapping);
- (v1.4, D-18) **scroll lock wiring:** a `touchmove` listener is registered on `u.over` at mount with `passive: false`. A cancelable `touchmove` with one touch is `defaultPrevented` only after touch pointer events for a horizontal move of ≥ 8 px. It is not prevented after a vertical move, before the slop is reached, with two touches, after `pointerup`, or for mouse input. On a non-cancelable `touchmove`, `preventDefault` is not called (spy). jsdom may lack `Touch`/`TouchEvent`; then dispatch an `Event('touchmove', { cancelable })` with a `touches` property defined on it, and say so in a comment;
- the cursor line transform and visibility after a flush;
- readout text, including `–`;
- (v1.4, QA-05-r2-3 c) every value cell has the `style.minWidth` from the design ("Readout") and an empty `style.width`;
- a throwing extra subscriber does not freeze the lines;
- the position cache is refreshed on `pointerenter` and after resize;
- cleanup on unmount: no listeners left, `destroy` called, unsubscribed. (v1.4, QA-05-r2-3 b) Each `removeEventListener` must match an `addEventListener` call by target, type, listener **and capture flag**: normalise a boolean or an options object to its `capture` value. This applies to the window `scroll` listener (capture) and the new `touchmove`.

No pixel, layout or timing assertions |

**Not covered by automated tests** (jsdom has no layout, canvas or real touch; verified on the dev page, manually, and by QA review):

| What | How it is verified |
| --- | --- |
| uPlot draws the lines, breaks are visible, plots are aligned (AC 1, 3, 4) | Dev page: ride-12h, indoor-no-gps, no-distance-with-gps, HR-dropout variant, pause-hours; the cursor line sits at the same x in every chart |
| Hover moves the cursor in every chart and the readout (AC 2) | Dev page, desktop; the "second subscriber" line follows |
| Vertical swipe on a chart scrolls the page; horizontal drag scrubs (AC 5); the scroll lock and pinch (D-18); all readout values visible while scrubbing (D-17) | Phone, dev page (same access as F-04's bench, F-04 design "Phone access"), with the v1.4 phone checks below. Desktop DevTools touch emulation is a smoke test only |
| Resize and rotation redraw at the new width (AC 6) | Desktop window resize; phone rotation |
| 60 fps (AC 7) | Measurement above |
| Hot-path rules (D-10) | Code review of `MetricChart`'s and `CursorReadout`'s listeners and the `touchmove` handler; `grep -rn '\$cursor' src/web/src/components` is empty |

**Manual checks, v1.4 (operator; they replace or add to the QA r2 list).** Phone: iPhone 13 mini, Orion (iOS WebKit), portrait unless stated. If an Android phone with Chrome is at hand, repeat P1, P3, P11 and P12 there; otherwise Android stays unchecked, and that is recorded.

| # | Check | Pass | Covers |
| --- | --- | --- | --- |
| P1 | Vertical swipe starting on a chart | The page scrolls | AC 5 |
| P3 | Touch down on a chart and hold | The cursor jumps under the finger at once. Then a vertical swipe from a chart: the page scrolls and the cursor returns to where it was before (a brief flicker is fine) | D-7, user P3 B |
| P5 | Pinch on a chart. Also: start a scrub, then put a second finger down and pinch | The page zooms, or at least nothing gets stuck: after lifting both fingers, a vertical swipe on a chart scrolls | D-18 pinch |
| P7 (replaced) | ride-12h, scrub any chart | All 8 readout values (Time, Distance, 6 metrics) are visible on several lines, with no sideways scrolling and nothing clipped. Scroll down to the temperature chart and scrub it: the readout stays pinned at the top of the screen and updates | AP-6, D-17 |
| P10 | Load ride-12h; rotate portrait → landscape → portrait | No page jump when the charts appear. After each rotation the charts redraw at the new width, the readout re-wraps, and there is no gap or overlap below the stack | AC 6, D-16 |
| P11 (new) | Start a clearly horizontal drag on a chart, then move the finger up and down while still dragging | The page does not scroll for the rest of that touch, and the cursor follows the finger's x. Lift, then swipe vertically from a chart: the page scrolls | D-18, user decision 2 |
| P12 (new) | Drags from a chart at roughly 20° and roughly 45° from horizontal (by eye) | 20°: scrubs and locks as in P11. 45°: the page scrolls | D-18 30° rule |
| P13 (new) | Tap a chart without moving | The cursor lands where the finger touched, and stays after lifting | AC 2, QA-05-r2-3 a |
| P2 (information only) | A horizontal drag that drifts onto the next chart | Record whether the cursor keeps following. Not required (user decision) | ADR-010 rev. 2 |

Desktop: QA r2's D13 and D14 still apply. **D4 (replaced):** ride-12h at 1,200 px or wider shows the readout on one line, and the cells keep their width while scrubbing. Narrowing the window to about 600 px makes the readout wrap onto more lines, with no horizontal scrollbar and nothing clipped.

## Acceptance criteria to verification

| AC | Verified by |
| --- | --- |
| 1 Six charts aligned; no power chart without power; no elevation chart without altitude | `specs.test.ts`; alignment inputs in `uplot-options.test.ts`; dev page visual check with ride-12h, no-distance-with-gps, indoor-no-gps (no synthetic *running* file without power exists; chart choice does not depend on sport, so the no-power file stands in) |
| 2 Hover/touch-drag moves the cursor in every chart and updates the readout | `store.test.ts`, `gesture.test.ts`; dev page desktop and phone |
| 3 Heart-rate dropout is a break, not zero | `decimate.test.ts`; dev page "ride-4h with heart-rate dropout" |
| 4 Multi-hour pause is a break on the time axis | `decimate.test.ts` (golden `pause-hours`); dev page |
| 5 Vertical swipe on a chart scrolls on a phone | Phone run on the dev page (OQ-12) |
| 6 Resize and rotation redraw at the new width | Desktop resize; phone rotation (OQ-12) |
| 7 12-hour scrub at 60 fps on desktop | Dev page measurement, row in `bench/MEASUREMENTS.md` (OQ-12) |

## Open questions

None blocks implementation.

| ID | Question | Blocks | Owner |
| --- | --- | --- | --- |
| OQ-12 | The engineer's machine has no browser (F-04 `MEASUREMENTS.md`). ACs 1–7 need a browser for their visual part, AC 5 and AC 6 a phone, AC 7 a desktop browser. Proposal: the operator runs the dev page on a desktop browser and on their phone once the engineer hands off, and records AC 7 in `MEASUREMENTS.md`; QA reviews the record | Ticking ACs 1–7 (visual parts), not implementation | user (operator) |