<script lang="ts">
  // Stacked charts on one shared time axis with the readout above them
  // (F-05 design; AP-4, AP-6). Public component of components/charts.
  //
  // The page creates the cursor store (one per WorkoutData) and passes it in;
  // a new data prop rebuilds the stack, and the page passes a new store with it.
  // Charts and readout never talk to each other: they share the cursor store.
  import { onMount, untrack } from 'svelte';
  import type { WorkoutData } from '../../lib/datafile';
  import type { CursorStore } from '../../lib/cursor';
  import type { SelectionStore } from '../../lib/selection';
  import { distanceAxis, presentCharts, timeAxis, type AxisKind, type HintMode } from '../../lib/charts';
  import type { Units } from '../../lib/format';
  import CursorReadout from './CursorReadout.svelte';
  import MetricChart from './MetricChart.svelte';
  import { DEFAULT_CHART_HEIGHT, TITLE_ROW_HEIGHT, Y_AXIS_SIZE, chartAreaHeight, plotWidth } from './uplot-options';

  interface Props {
    data: WorkoutData;
    cursor: CursorStore;
    /** The range selection (F-17 R-1), one per WorkoutData like the cursor; absent: no selection. */
    selection?: SelectionStore;
    /** Plot area height of every chart in CSS px; default DEFAULT_CHART_HEIGHT.
     *  Read once per `data` value (D-15): changing it without a new `data` has no effect. */
    chartHeight?: number;
    /** The viewer's unit system and the colour scheme (F-14 D-10, D-11): a change recreates the uPlot
     *  instances and the readout (new axes, colours, field functions); the root, the chart area's
     *  reserved height and the cursor store stay. */
    units?: Units;
    scheme?: 'light' | 'dark';
    /** Called once, in the first animation frame after every chart has its first data (F-06 loads MapLibre from it). */
    onready?: () => void;
    /** The x axis to start on (AP-5; read once). Without distance the charts use time anyway. */
    initialAxis?: AxisKind;
    /** Called when the viewer picks an axis with the toggle (the page remembers it). */
    onaxis?: (axis: AxisKind) => void;
    /** The range hint (F-39 HU-3); default `plain`. The page owns it: × only reports through onhint. */
    hint?: HintMode;
    onhint?: () => void;
  }

  let { data, cursor, selection, chartHeight, units = 'metric', scheme = 'light', onready, initialAxis, onaxis, hint = 'plain', onhint }: Props = $props();

  // A wiring bug, not a data problem: one cursor store per WorkoutData (ADR-008).
  // Reads only the two sampleCount values: never cursor.get() or subscribe.
  function checkWiring(d: WorkoutData, c: CursorStore): void {
    if (c.sampleCount !== d.sampleCount) {
      throw new Error(`ChartStack: cursor store for ${c.sampleCount} samples, data has ${d.sampleCount}`);
    }
  }
  $effect.pre(() => checkWiring(data, cursor));

  const charts = $derived(presentCharts(data, units));
  // The x axis (F-17 R-7, R-8): ChartStack's own state, from initialAxis (time by default); the page
  // remembers the viewer's choice through onaxis. Both axes are built once per data value; a switch
  // rebuilds the uPlot instances (F-14 D-11's path) and leaves the stores alone, so the cursor and
  // the range stay.
  let axisKind = $state<AxisKind>(untrack(() => initialAxis ?? 'time'));
  function pickAxis(kind: AxisKind): void {
    axisKind = kind;
    onaxis?.(kind);
  }
  const timeX = $derived(timeAxis(data.time));
  const distanceX = $derived(data.metrics.distance ? distanceAxis(data.metrics.distance) : null);
  const axis = $derived(axisKind === 'distance' && distanceX ? distanceX : timeX);
  // chartHeight is read once per data value (D-15): untracked, so only a new data re-reads it.
  const plotHeight = $derived.by(() => {
    void data;
    return untrack(() => chartHeight ?? DEFAULT_CHART_HEIGHT);
  });
  let stack: HTMLDivElement;
  /** The chart area of the current data value (a new element per {#key data}). */
  let area = $state<HTMLDivElement | null>(null);
  /** Content width in CSS px; 0 until the first ResizeObserver callback. */
  let width = $state(0);
  /** True once the first width measurement has been processed (even a width of 0). */
  let measured = $state(false);
  const drawable = $derived(plotWidth(width) >= 1);

  // onready bookkeeping (plain variables: not rendered).
  let readyFired = false;
  let readyData: WorkoutData | null = null;
  let readyCount = 0;

  function fireReady(): void {
    if (readyFired) return;
    readyFired = true;
    requestAnimationFrame(() => onready?.());
  }

  function chartHasData(forData: WorkoutData, total: number): void {
    if (readyData !== forData) {
      readyData = forData;
      readyCount = 0;
    }
    readyCount++;
    if (readyCount >= total) fireReady();
  }

  onMount(() => {
    let frame = 0;
    let pending = 0;
    const observer = new ResizeObserver((entries) => {
      pending = Math.floor(entries[entries.length - 1].contentRect.width);
      if (frame === 0) {
        frame = requestAnimationFrame(() => {
          frame = 0;
          width = pending;
          measured = true;
        });
      }
    });
    // Layout custom properties, one write (no style= attribute, v1.3: Svelte's style runtime
    // costs about 790 bytes gzip). The root has no height (D-16, v1.4).
    stack.style.cssText = `--y-axis-width:${Y_AXIS_SIZE}px;--title-row-height:${TITLE_ROW_HEIGHT}px`;
    observer.observe(stack);
    return () => {
      observer.disconnect();
      if (frame !== 0) cancelAnimationFrame(frame);
    };
  });

  // The chart area reserves its height before the first paint and again for every new data
  // value, also while the charts cannot draw yet (D-16, v1.4; QA-05-r2-2).
  $effect(() => {
    const el = area;
    const h = chartAreaHeight(charts.length, plotHeight);
    if (el) el.style.cssText = 'height:' + h + 'px';
  });

  // onready fires exactly once, in the first frame after the first width measurement:
  // right away when no chart will draw (time only, or too narrow), else when every chart
  // has done its first layout (MetricChart reports even with 0 buckets).
  $effect(() => {
    if (measured && (charts.length === 0 || !drawable)) fireReady();
  });
</script>

<!-- tabindex -1: the ×'s focus lands here when it removes itself (QA-39-r1-2). -->
<div class="chart-stack" tabindex="-1" bind:this={stack}>
  {#key data}
   {#if data.metrics.distance && charts.length > 0}
    <!-- The row is reserved by the page with any distance stream (has-distance); the control only
         when the distance advances (R-8). -->
    <div class="toolbar">
      {#if distanceX}
        <span class="axis-toggle">
          <button type="button" aria-pressed={axis.kind === 'time'} onclick={() => pickAxis('time')}>Time</button><button
            type="button" aria-pressed={axis.kind === 'distance'} onclick={() => pickAxis('distance')}>Distance</button>
        </span>
      {/if}
    </div>
   {/if}
   {#key units + scheme + axis.kind}
    <CursorReadout {data} {cursor} {units} />
    <div class="chart-area" bind:this={area}>
      {#if drawable}
        {#each charts as spec, k (spec.metric)}
          <MetricChart
            {spec}
            {data}
            {cursor}
            {axis}
            {selection}
            {units}
            {width}
            height={plotHeight}
            isBottom={k === charts.length - 1}
            onfirstdata={() => chartHasData(data, charts.length)}
          />
        {/each}
      {/if}
    </div>
   {/key}
   {#if charts.length > 0 && hint !== 'none'}
    <!-- How to select (R-12): the wording follows the primary pointer, in CSS. -->
    <p class="select-hint">{#if hint !== 'done'}<span class="fine">Drag across a chart to select a range.</span><span class="coarse">Touch and hold a chart, then drag to select a range.</span>{/if}{#if hint === 'once'}<button
      type="button" class="hint-x" aria-label="Dismiss hint" onclick={() => {
        stack.focus();
        onhint?.();
      }}>×</button>{/if}</p>
   {/if}
  {/key}
</div>

<style>
  .chart-stack {
    width: 100%;
    min-width: 0;
  }
  /* Fixed heights: the page reserves them (page.css --bar, --hint). */
  .toolbar {
    height: 32px;
    display: flex;
    align-items: center;
  }
  .axis-toggle {
    display: inline-flex;
    border: 1px solid var(--line, #c4c8cc);
    border-radius: 6px;
    overflow: hidden;
  }
  .axis-toggle button {
    height: 26px;
    padding: 0 10px;
    border: 0;
    background: var(--bg, #fff);
    color: var(--fg, #111);
    font: inherit;
    font-size: 13px;
  }
  .axis-toggle button[aria-pressed='true'] {
    background: var(--accent, #1f6fc5);
    color: var(--on-accent, #fff);
  }
  .select-hint {
    height: 20px;
    margin: 4px 0 0;
    font-size: 12px;
    line-height: 20px;
    color: var(--muted, #555);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* F-39 HU-5: the text ellipsises, the × never; its 24 px hit area takes the 4 px gap above. */
  .select-hint:has(button) {
    display: flex;
    overflow: visible;
  }
  .select-hint:has(button) span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .hint-x {
    flex: none;
    width: 24px;
    height: 24px;
    margin-top: -4px;
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    font-size: 16px;
    line-height: 24px;
  }
  .coarse {
    display: none;
  }
  @media (pointer: coarse) {
    .fine {
      display: none;
    }
    .coarse {
      display: inline;
    }
  }
</style>
