<script lang="ts">
  // One metric chart: one uPlot instance (renderer only), our pointer handling
  // and our cursor line (F-05 design v1.4, "Charts", "Interaction"; ADR-009, ADR-010 rev. 2).
  //
  // Hot path (D-10): moveLine runs on every cursor flush. It writes one
  // style.transform (and visibility when crossing null), reads no layout,
  // redraws no canvas and touches no Svelte state. The transform string is the
  // only allocation; the DOM needs a string.
  import { onMount } from 'svelte';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import type { WorkoutData } from '../../lib/datafile';
  import { nearestIndex, type CursorIndex, type CursorStore } from '../../lib/cursor';
  import type { SelectionStore } from '../../lib/selection';
  import type { Units } from '../../lib/format';
  import {
    IDLE,
    LONG_PRESS_MS,
    blocksScroll,
    holdsTouch,
    bucketCount,
    chartValues,
    decimate,
    step,
    type ChartAxis,
    type ChartSpec,
    type GestureInput,
    type GestureInputKind,
    type GestureState,
  } from '../../lib/charts';
  import { buildOptions, outerHeight, xScale, type ChartDecorator } from './uplot-options';

  interface Props {
    spec: ChartSpec;
    data: WorkoutData;
    cursor: CursorStore;
    /** The x axis (time or distance), built once per data value by ChartStack. */
    axis: ChartAxis;
    /** Absent: no range selection (bench pages). */
    selection?: SelectionStore;
    units?: Units;
    /** Full chart width in CSS px (the stack's content width). */
    width: number;
    /** Plot area height in CSS px. */
    height: number;
    isBottom: boolean;
    isTop?: boolean;
    /** See ChartDecorator; called from layout only. */
    decorate?: ChartDecorator;
    /** Called once, after the first layout: after the first setData, or with no data when the width gives 0 buckets. */
    onfirstdata?: () => void;
  }

  let { spec, data, cursor, axis, selection, units = 'metric', width, height, isBottom, isTop, decorate, onfirstdata }: Props = $props();

  let host: HTMLDivElement;
  let layout: ((w: number, f?: ChartDecorator) => void) | null = null;

  type MutableInput = { -readonly [K in keyof GestureInput]: GestureInput[K] };

  onMount(() => {
    // Props are fixed for the life of this instance (ChartStack keys on data).
    const s = spec;
    const d = data;
    const c = cursor;
    const plotHeight = height;
    const bottom = isBottom;
    const firstData = onfirstdata;
    const ax = axis;
    const sel = selection;
    const xs = ax.x;
    const at = ax.at;
    const keys = ax.keys;
    const keyIndex = ax.index;
    // presentCharts only gives specs of present streams.
    // Converted once, so a pace stop (NaN) is a gap before decimation and the min/max view stays honest.
    const values = chartValues(s, d.metrics[s.metric] as Float64Array);
    const scale = xScale(ax, units);
    const xMin = scale.min;
    const xMax = scale.max;

    // Canvas colours come from the theme's custom properties (F-14 D-10); the spec's colour and
    // light literals are the fallback where no stylesheet is present (bench pages, jsdom).
    const css = getComputedStyle(host);
    const prop = (name: string, fallback: string): string => css.getPropertyValue(name).trim() || fallback;
    const ink = { line: prop('--c-' + s.metric, s.colour), axis: prop('--muted', '#555'), grid: prop('--grid', 'rgba(0,0,0,0.1)') };
    const u = new uPlot(buildOptions(s, d, { width, height: plotHeight, isBottom: bottom }, ink, scale), undefined, host);
    const over = u.over;

    // Cursor line: one absolutely positioned div in the plot area.
    const line = document.createElement('div');
    line.className = 'wx-cursor-line';
    line.style.visibility = 'hidden';
    over.appendChild(line);
    let lineVisible = false;

    // Range shading (R-6): one element, a 1 px box scaled to the range's width; hidden for none.
    const shade = document.createElement('div');
    shade.className = 'wx-range-shade';
    shade.style.visibility = 'hidden';
    over.insertBefore(shade, line);
    let shadeA = -1;
    let shadeB = -1;
    const placeShade = (a: number, b: number): void => {
      shadeA = a;
      shadeB = b;
      if (a < 0) {
        shade.style.visibility = 'hidden';
        return;
      }
      const left = u.valToPos(at[a], 'x');
      const w = u.valToPos(at[b], 'x') - left;
      shade.style.transform = 'translateX(' + left + 'px) scaleX(' + (w > 1 ? w : 1) + ')';
      shade.style.visibility = 'visible';
    };

    const moveLine = (i: CursorIndex): void => {
      // No x (a sample without distance): no line (R-7).
      if (i === null || xs[i] !== xs[i]) {
        if (lineVisible) {
          line.style.visibility = 'hidden';
          lineVisible = false;
        }
        return;
      }
      line.style.transform = 'translateX(' + u.valToPos(xs[i], 'x') + 'px)';
      if (!lineVisible) {
        line.style.visibility = 'visible';
        lineVisible = true;
      }
    };

    // Cached plot rectangle (CSS px, viewport). null = stale.
    let rect: DOMRect | null = null;
    let lastWidth = -1;
    let firstLayoutDone = false;

    layout = (w: number, f?: ChartDecorator): void => {
      if (w !== lastWidth) {
        lastWidth = w;
        u.batch(() => {
          u.setSize({ width: w, height: outerHeight(plotHeight, bottom) });
          // setSize computes u.bbox synchronously (canvas px); never read from the DOM.
          const buckets = bucketCount(u.bbox.width / uPlot.pxRatio, uPlot.pxRatio);
          if (buckets < 1) return;
          const dec = decimate(xs, values, d.gaps, xMin, xMax, buckets);
          u.setData([dec.x, dec.y]);
        });
        rect = null;
        moveLine(c.get());
        placeShade(shadeA, shadeB);
        if (!firstLayoutDone) {
          // onready must never wait for a chart that will not draw (v1.1).
          firstLayoutDone = true;
          firstData?.();
        }
      }
      f?.(over, (i) => u.valToPos(at[i], 'x'), !!isTop);
    };
    layout(width);

    // Pointer input → gesture reducer → cursor.set. One reused input object.
    let state: GestureState = IDLE;
    const input: MutableInput = {
      kind: 'move',
      pointerType: '',
      pointerId: 0,
      isPrimary: true,
      index: -1,
      current: null,
      x: 0,
      y: 0,
      rangeA: -1,
      rangeB: -1,
    };
    // The sample nearest the pointer's x: earliest on a tie, never one without an x (R-7).
    const indexAt = (clientX: number): number => {
      if (rect === null) rect = over.getBoundingClientRect();
      const k = nearestIndex(keys, u.posToVal(clientX - rect.left, 'x'));
      return k < 0 || keyIndex === null ? k : keyIndex[k];
    };
    let pressTimer: ReturnType<typeof setTimeout> | undefined;
    const stopTimer = (): void => {
      clearTimeout(pressTimer);
      pressTimer = undefined;
    };
    const run = (kind: GestureInputKind, pointerType: string, pointerId: number, isPrimary: boolean, index: number, x: number, y: number): void => {
      input.kind = kind;
      input.pointerType = pointerType;
      input.pointerId = pointerId;
      input.isPrimary = isPrimary;
      input.index = index;
      input.current = c.get();
      input.x = x;
      input.y = y;
      input.rangeA = sel ? sel.start() : -1;
      input.rangeB = sel ? sel.end() : -1;
      const r = step(state, input);
      state = r.state;
      if (r.action.kind === 'set') c.set(r.action.index);
      const range = r.range;
      if (range && sel) {
        if (range.a < 0) sel.clear();
        else sel.set(range.a, range.b);
      }
    };
    const handle = (kind: GestureInputKind, e: PointerEvent): void => {
      const index = kind === 'down' || kind === 'move' || kind === 'up' ? indexAt(e.clientX) : -1;
      run(kind, e.pointerType, e.pointerId, e.isPrimary, index, e.clientX, e.clientY);
    };

    const onEnter = (): void => {
      rect = over.getBoundingClientRect();
    };
    const onDown = (e: PointerEvent): void => {
      // Mouse: the main button only; the other buttons keep their browser meaning.
      if (e.pointerType !== 'touch' && e.button !== 0) return;
      rect = over.getBoundingClientRect();
      handle('down', e);
      stopTimer();
      if (!sel || !e.isPrimary) return;
      if (e.pointerType === 'touch') {
        const id = e.pointerId;
        pressTimer = setTimeout(() => {
          pressTimer = undefined;
          run('longpress', 'touch', id, true, -1, 0, 0);
        }, LONG_PRESS_MS);
      } else {
        // A drag keeps reaching this chart when it leaves the plot.
        try {
          over.setPointerCapture?.(e.pointerId);
        } catch {
          // not an active pointer (synthetic events)
        }
      }
    };
    const onMove = (e: PointerEvent): void => handle('move', e);
    const onUp = (e: PointerEvent): void => {
      stopTimer();
      handle('up', e);
    };
    const onCancel = (e: PointerEvent): void => {
      stopTimer();
      handle('cancel', e);
    };
    // No touch callout or context menu while a touch is held or selecting (R-5).
    const onMenu = (e: Event): void => {
      if (holdsTouch(state)) e.preventDefault();
    };
    const onLeave = (e: PointerEvent): void => handle('leave', e);
    const onScroll = (): void => {
      rect = null;
    };
    // The scroll lock (D-18): while one finger scrubs, the page does not scroll. The only
    // non-passive listener. Like the flush: no allocation, no layout read, no set.
    const onTouchMove = (e: TouchEvent): void => {
      if (e.cancelable && blocksScroll(state, e.touches.length)) e.preventDefault();
    };

    const passive: AddEventListenerOptions = { passive: true };
    const capturePassive: AddEventListenerOptions = { passive: true, capture: true };
    const blocking: AddEventListenerOptions = { passive: false };
    over.addEventListener('pointerenter', onEnter, passive);
    over.addEventListener('pointerdown', onDown, passive);
    over.addEventListener('pointermove', onMove, passive);
    over.addEventListener('pointerup', onUp, passive);
    over.addEventListener('pointercancel', onCancel, passive);
    over.addEventListener('pointerleave', onLeave, passive);
    over.addEventListener('contextmenu', onMenu);
    // Added here, at mount, never during a touch: iOS WebKit decides at touch start whether
    // a blocking listener exists (WebKit bug 184251).
    over.addEventListener('touchmove', onTouchMove, blocking);
    // Capture: scrolling of any ancestor moves the plot, not only the window's.
    window.addEventListener('scroll', onScroll, capturePassive);

    const unsubscribe = c.subscribe(moveLine);
    const unsubscribeRange = sel ? sel.subscribe(placeShade) : null;

    return () => {
      unsubscribe();
      unsubscribeRange?.();
      stopTimer();
      over.removeEventListener('contextmenu', onMenu);
      over.removeEventListener('pointerenter', onEnter, passive);
      over.removeEventListener('pointerdown', onDown, passive);
      over.removeEventListener('pointermove', onMove, passive);
      over.removeEventListener('pointerup', onUp, passive);
      over.removeEventListener('pointercancel', onCancel, passive);
      over.removeEventListener('pointerleave', onLeave, passive);
      over.removeEventListener('touchmove', onTouchMove, blocking);
      window.removeEventListener('scroll', onScroll, capturePassive);
      layout = null;
      u.destroy();
    };
  });

  // Width changes (resize, rotation; AC 6): ChartStack coalesces them to one
  // animation frame. Not on the scrubbing path.
  // The decoration too (F-42 MS-7): on these and when it arrives.
  $effect(() => {
    const w = width;
    const f = decorate;
    if (layout) layout(w, f);
  });
</script>

<div class="metric-chart">
  <div class="chart-title">{spec.label} · {spec.unit}</div>
  <div class="chart-host" bind:this={host}></div>
</div>

<style>
  .chart-title {
    height: var(--title-row-height);
    line-height: var(--title-row-height);
    font-size: 12px;
    color: var(--muted, #555);
    padding-left: var(--y-axis-width);
    white-space: nowrap;
    overflow: hidden;
  }
  .chart-host :global(.u-over) {
    touch-action: pan-y pinch-zoom;
    user-select: none;
    -webkit-user-select: none;
    -webkit-touch-callout: none;
  }
  .chart-host :global(.wx-range-shade) {
    position: absolute;
    top: 0;
    left: 0;
    width: 1px;
    height: 100%;
    background: var(--range-shade, rgba(31, 111, 197, 0.16));
    transform-origin: 0 0;
    pointer-events: none;
  }
  .chart-host :global(.wx-cursor-line) {
    position: absolute;
    top: 0;
    left: 0;
    width: 1px;
    height: 100%;
    background: var(--cursor, #222);
    pointer-events: none;
    will-change: transform;
  }
</style>
