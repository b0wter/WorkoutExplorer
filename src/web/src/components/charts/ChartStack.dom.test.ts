// @vitest-environment jsdom
// Component smoke tests for components/charts (ADR-012; F-05 design v1.2, Tests,
// "components/charts/*.dom.test.ts"). They check wiring, ordering, lifecycle,
// event-to-store paths and DOM writes; never pixels, layout or timing.
//
// Real Svelte, real uPlot. Stubs, all installed before uPlot loads:
// - requestAnimationFrame / cancelAnimationFrame: manual frames;
// - ResizeObserver: jsdom has none; the test drives widths;
// - matchMedia: jsdom has none; uPlot queries the device pixel ratio at module load;
// - canvas getContext and Path2D: jsdom has no 2D canvas, and uPlot builds its
//   paths with Path2D. Both are no-ops; nothing here inspects drawing.
// uPlot is wrapped (vi.mock below), not replaced: the wrapper constructs the real
// uPlot and only records each instance and its setSize / setData / destroy calls.
// Synthetic data only (ADR-004 item 7).

import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../../lib/datafile';
import { createCursorStore, nearestIndex, type CursorStore } from '../../lib/cursor';
import { createSelectionStore, type SelectionStore } from '../../lib/selection';
import { LONG_PRESS_MS, distanceAxis } from '../../lib/charts';
import { bucketCount, decimate, presentCharts, readoutFields } from '../../lib/charts';
import ChartStack from './ChartStack.svelte';
import { chartAreaHeight, plotWidth, xRange } from './uplot-options';
import { swapHarness } from './swap-props.test-helper.svelte';
import { lookHarness } from './look-props.test-helper.svelte';

interface UPlotLike {
  readonly over: HTMLDivElement;
  readonly bbox: { width: number };
  valToPos(v: number, scale: string): number;
  posToVal(v: number, scale: string): number;
  setSize(s: { width: number; height: number }): void;
  setData(d: unknown, reset?: boolean): void;
  destroy(): void;
}
interface Recorded {
  readonly u: UPlotLike;
  readonly log: string[];
  readonly data: { x: number[]; y: (number | null)[]; bboxWidth: number }[];
  destroyed: boolean;
}

const env = vi.hoisted(() => {
  // Manual animation frames.
  const callbacks = new Map<number, FrameRequestCallback>();
  let nextId = 1;
  window.requestAnimationFrame = (f: FrameRequestCallback): number => {
    callbacks.set(nextId, f);
    return nextId++;
  };
  window.cancelAnimationFrame = (id: number): void => {
    callbacks.delete(id);
  };
  const frame = (): void => {
    const batch = [...callbacks.values()];
    callbacks.clear();
    for (const f of batch) f(performance.now());
  };

  // Manual ResizeObserver.
  const observers: { cb: ResizeObserverCallback; disconnected: boolean }[] = [];
  class FakeResizeObserver {
    private readonly entry: { cb: ResizeObserverCallback; disconnected: boolean };
    constructor(cb: ResizeObserverCallback) {
      this.entry = { cb, disconnected: false };
      observers.push(this.entry);
    }
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {
      this.entry.disconnected = true;
    }
  }
  window.ResizeObserver = FakeResizeObserver as unknown as typeof ResizeObserver;
  const resize = (width: number): void => {
    for (const o of observers) {
      if (!o.disconnected) o.cb([{ contentRect: { width } } as ResizeObserverEntry], {} as ResizeObserver);
    }
  };

  window.matchMedia = ((media: string) => ({
    matches: false,
    media,
    onchange: null,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;

  // No-op 2D context: every method is a no-op, measureText gives a width, properties store.
  const store: Record<string | symbol, unknown> = {};
  const ctx = new Proxy(store, {
    get(target, key) {
      if (key in target) return target[key];
      if (key === 'measureText') return () => ({ width: 20 });
      return () => {};
    },
    set(target, key, value) {
      target[key] = value;
      return true;
    },
  });
  HTMLCanvasElement.prototype.getContext = (() => ctx) as unknown as HTMLCanvasElement['getContext'];
  class NoopPath2D {
    moveTo(): void {}
    lineTo(): void {}
    rect(): void {}
    arc(): void {}
    closePath(): void {}
    bezierCurveTo(): void {}
    addPath(): void {}
  }
  (globalThis as unknown as { Path2D: unknown }).Path2D = NoopPath2D;

  const charts: Recorded[] = [];
  return { frame, resize, observers, charts };
});

vi.mock('uplot', async (importOriginal) => {
  type Ctor = (new (opts: unknown, data: unknown, target: unknown) => UPlotLike) & { pxRatio: number };
  const real = ((await importOriginal()) as { default: Ctor }).default;
  function RecordedUPlot(opts: unknown, data: unknown, target: unknown): UPlotLike {
    const u = new real(opts, data, target);
    const rec: Recorded = { u, log: [], data: [], destroyed: false };
    const setSize = u.setSize;
    const setData = u.setData;
    const destroy = u.destroy;
    const mutable = u as { -readonly [K in keyof UPlotLike]: UPlotLike[K] };
    mutable.setSize = (s) => {
      rec.log.push('setSize');
      setSize(s);
    };
    mutable.setData = (d, reset) => {
      rec.log.push('setData');
      const [x, y] = d as [number[], (number | null)[]];
      rec.data.push({ x, y, bboxWidth: u.bbox.width });
      setData(d, reset);
    };
    mutable.destroy = () => {
      rec.destroyed = true;
      destroy();
    };
    env.charts.push(rec);
    return u;
  }
  Object.defineProperty(RecordedUPlot, 'pxRatio', { get: () => real.pxRatio });
  return { default: RecordedUPlot };
});

// ---- data ----------------------------------------------------------------------

const decoded = new Map<string, WorkoutData>();
for (const g of loadGolden()) {
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(`${g.name}: ${r.error.message}`);
  decoded.set(g.name, r.data);
}
function golden(name: string): WorkoutData {
  const d = decoded.get(name);
  if (!d) throw new Error(`no golden ${name}`);
  return d;
}
function heartRateDropout(d: WorkoutData): WorkoutData {
  const hr = Float64Array.from(d.metrics.heart_rate ?? []);
  hr.fill(Number.NaN, 3600, 4200);
  return { ...d, metrics: { ...d.metrics, heart_rate: hr } };
}
function timeOnly(n: number): WorkoutData {
  const base = golden('ride-4h');
  const time = new Float64Array(n);
  for (let i = 0; i < n; i++) time[i] = i;
  return { ...base, sampleCount: n, time, gaps: new Uint32Array(0), position: undefined, metrics: {} };
}

// ---- mounting --------------------------------------------------------------------

interface Mounted {
  data: WorkoutData;
  cursor: CursorStore;
  target: HTMLDivElement;
  reported: unknown[];
  ready: () => number;
  /** Subscriptions currently active on the store. */
  active: () => number;
  app: ReturnType<typeof mount>;
}

let current: Mounted | null = null;

function setup(
  data: WorkoutData,
  options: {
    chartHeight?: number;
    before?: (c: CursorStore) => void;
    cursor?: CursorStore;
    selection?: SelectionStore;
    initialAxis?: 'time' | 'distance';
    onaxis?: (a: 'time' | 'distance') => void;
    hint?: 'plain' | 'once' | 'done' | 'none';
    onhint?: () => void;
  } = {},
): Mounted {
  const reported: unknown[] = [];
  const cursor =
    options.cursor ?? createCursorStore(data.sampleCount, (f) => requestAnimationFrame(() => f()), (e) => reported.push(e));
  // Count active subscriptions (cleanup check).
  let active = 0;
  const subscribe = cursor.subscribe.bind(cursor);
  (cursor as { subscribe: CursorStore['subscribe'] }).subscribe = (l) => {
    active++;
    const off = subscribe(l);
    let done = false;
    return () => {
      if (!done) {
        done = true;
        active--;
      }
      off();
    };
  };
  options.before?.(cursor);
  const target = document.createElement('div');
  document.body.appendChild(target);
  let ready = 0;
  const props: Record<string, unknown> = { data, cursor, onready: () => ready++ };
  if (options.chartHeight !== undefined) props.chartHeight = options.chartHeight;
  if (options.selection) props.selection = options.selection;
  if (options.initialAxis) props.initialAxis = options.initialAxis;
  if (options.onaxis) props.onaxis = options.onaxis;
  if (options.hint) props.hint = options.hint;
  if (options.onhint) props.onhint = options.onhint;
  const app = mount(ChartStack, { target, props: props as never });
  flushSync();
  current = { data, cursor, target, reported, ready: () => ready, active: () => active, app };
  return current;
}

/** One ResizeObserver report, its animation frame, and Svelte's effects. */
function layout(width: number): void {
  env.resize(width);
  env.frame();
  flushSync();
}

const overs = (m: Mounted) => [...m.target.querySelectorAll<HTMLDivElement>('.u-over')];
const lines = (m: Mounted) => [...m.target.querySelectorAll<HTMLDivElement>('.wx-cursor-line')];
const readout = (m: Mounted) => [...m.target.querySelectorAll<HTMLSpanElement>('.readout .value')].map((e) => e.textContent);

function pointer(el: Element, type: string, clientX: number, o: Partial<PointerEventInit> = {}): void {
  el.dispatchEvent(
    new PointerEvent(type, { bubbles: true, clientX, clientY: 10, pointerType: 'mouse', pointerId: 1, isPrimary: true, ...o }),
  );
}

afterEach(() => {
  if (current) {
    unmount(current.app);
    flushSync();
    current.target.remove();
    current = null;
  }
  env.frame();
  env.charts.length = 0;
  env.observers.length = 0;
  vi.restoreAllMocks();
});

// ---- tests -------------------------------------------------------------------------

describe('ChartStack in jsdom', () => {
  it('throws on a cursor store for another sample count (wiring check)', () => {
    const data = golden('ride-4h');
    const target = document.createElement('div');
    expect(() => {
      mount(ChartStack, { target, props: { data, cursor: createCursorStore(5, () => {}) } });
      flushSync();
    }).toThrow(/cursor store for 5 samples/);
  });

  const areaOf = (m: Mounted) => m.target.querySelector<HTMLDivElement>('.chart-area');
  const rootOf = (m: Mounted) => m.target.querySelector<HTMLDivElement>('.chart-stack');

  it('sets the chart-area height and the root custom properties at mount, before the first resize frame', () => {
    const m = setup(golden('ride-12h'));
    expect(areaOf(m)?.style.height).toBe(`${chartAreaHeight(6, 120)}px`);
    expect(rootOf(m)?.style.height).toBe('');
    expect(rootOf(m)?.style.getPropertyValue('--y-axis-width')).toBe('48px');
    expect(rootOf(m)?.style.getPropertyValue('--title-row-height')).toBe('16px');
    expect(overs(m)).toHaveLength(0);
    expect(env.charts).toHaveLength(0);
  });

  it('uses chartHeight for the chart-area height and the charts', () => {
    const m = setup(golden('no-distance-with-gps'), { chartHeight: 80 });
    expect(areaOf(m)?.style.height).toBe(`${chartAreaHeight(3, 80)}px`);
    layout(800);
    expect(overs(m).map((o) => o.style.height)).toEqual(['80px', '80px', '80px']);
    expect(areaOf(m)?.style.height).toBe(`${chartAreaHeight(3, 80)}px`);
  });

  it('gives a workout without charts a chart area of height 0', () => {
    const m = setup(timeOnly(10));
    expect(areaOf(m)?.style.height).toBe('0px');
    expect(rootOf(m)?.style.height).toBe('');
  });

  it('swapping data in place rebuilds the charts and rewrites the chart-area height (6 → 3 → 6, QA-05-r2-2)', () => {
    const schedule = (f: () => void) => {
      requestAnimationFrame(() => f());
    };
    const ride = golden('ride-12h');
    const small = golden('no-distance-with-gps');
    const target = document.createElement('div');
    document.body.appendChild(target);
    const h = swapHarness(ride, createCursorStore(ride.sampleCount, schedule));
    const app = mount(ChartStack, { target, props: h.props });
    flushSync();
    const area = () => target.querySelector<HTMLDivElement>('.chart-area');
    const charts = () => target.querySelectorAll('.u-over').length;
    try {
      layout(900);
      expect(charts()).toBe(6);
      expect(area()?.style.height).toBe(`${chartAreaHeight(6, 120)}px`);

      h.swap(small, createCursorStore(small.sampleCount, schedule));
      flushSync();
      expect(charts()).toBe(3);
      expect(area()?.style.height).toBe(`${chartAreaHeight(3, 120)}px`);
      expect(target.querySelectorAll('.readout .value')).toHaveLength(readoutFields(small).length);

      h.swap(ride, createCursorStore(ride.sampleCount, schedule));
      flushSync();
      expect(charts()).toBe(6);
      expect(area()?.style.height).toBe(`${chartAreaHeight(6, 120)}px`);
      expect(target.querySelector<HTMLDivElement>('.chart-stack')?.style.height).toBe('');
    } finally {
      unmount(app);
      flushSync();
      target.remove();
    }
  });

  describe('onready', () => {
    it('fires exactly once, in the frame after every chart has its first data', () => {
      const m = setup(golden('ride-12h'));
      layout(1200);
      expect(overs(m)).toHaveLength(6);
      expect(m.ready()).toBe(0);
      env.frame();
      expect(m.ready()).toBe(1);
      layout(700);
      env.frame();
      layout(1300);
      env.frame();
      expect(m.ready()).toBe(1);
    });

    it('fires once with no charts', () => {
      const m = setup(timeOnly(10));
      layout(800);
      env.frame();
      expect(overs(m)).toHaveLength(0);
      expect(m.ready()).toBe(1);
    });

    it('fires once after a first width of 0, and not again when charts appear later', () => {
      const m = setup(golden('ride-4h'));
      layout(0);
      env.frame();
      expect(overs(m)).toHaveLength(0);
      expect(m.ready()).toBe(1);
      layout(800);
      env.frame();
      expect(overs(m)).toHaveLength(6);
      expect(m.ready()).toBe(1);
    });
  });

  it('resize order: setSize before setData, buckets from bbox, decimated full-resolution data', () => {
    const m = setup(golden('ride-12h'));
    layout(1200);
    const [xMin, xMax] = xRange(m.data.time);
    const specs = presentCharts(m.data);
    expect(env.charts).toHaveLength(6);
    const check = (width: number, round: number) => {
      env.charts.forEach((rec, k) => {
        expect(rec.log.slice(round * 2, round * 2 + 2)).toEqual(['setSize', 'setData']);
        const d = rec.data[round];
        expect(d.bboxWidth).toBe(plotWidth(width)); // pxRatio 1 in jsdom
        const spec = specs[k];
        const values = m.data.metrics[spec.metric] as Float64Array;
        const expected = decimate(m.data.time, values, m.data.gaps, xMin, xMax, bucketCount(d.bboxWidth, 1));
        expect(d.x).toEqual(expected.x);
        expect(d.y).toEqual(expected.y.map((v) => (v === null ? null : spec.toDisplay(v))));
        expect(d.x.length).toBeLessThan(m.data.sampleCount);
      });
    };
    check(1200, 0);
    layout(600);
    check(600, 1);
    // Coalesced: two reports in one frame give one layout at the last width.
    env.resize(500);
    env.resize(520);
    env.frame();
    flushSync();
    check(520, 2);
    expect(env.charts.every((r) => r.log.length === 6)).toBe(true);
  });

  it('pointer events go through the gesture reducer into cursor.set', () => {
    const m = setup(golden('ride-12h'));
    layout(1200);
    const o = overs(m);
    const u = env.charts[2].u;
    // jsdom rects are all zero, so clientX is the plot-relative x.
    pointer(o[2], 'pointermove', 568);
    const expected = nearestIndex(m.data.time, u.posToVal(568, 'x'));
    expect(m.cursor.get()).toBe(expected);

    // Touch: down places the cursor under the finger (P3 option B), move scrubs, cancel restores.
    const touch = { pointerType: 'touch', pointerId: 5 };
    pointer(o[0], 'pointerdown', 100, touch);
    expect(m.cursor.get()).toBe(nearestIndex(m.data.time, u.posToVal(100, 'x')));
    pointer(o[0], 'pointermove', 200, touch);
    expect(m.cursor.get()).toBe(nearestIndex(m.data.time, u.posToVal(200, 'x')));
    pointer(o[0], 'pointercancel', 200, touch);
    expect(m.cursor.get()).toBe(expected);

    // Non-primary pointers are ignored; mouse leave keeps the cursor.
    pointer(o[1], 'pointermove', 900, { isPrimary: false });
    pointer(o[1], 'pointerleave', 2000);
    expect(m.cursor.get()).toBe(expected);

    // Outside the plot: clamped to the ends.
    pointer(o[1], 'pointermove', -500);
    expect(m.cursor.get()).toBe(0);
    pointer(o[1], 'pointermove', 99999);
    expect(m.cursor.get()).toBe(m.data.sampleCount - 1);
  });

  it('places every cursor line at the cursor after a flush, hides them for null, re-places after resize', () => {
    const m = setup(golden('brick-run-bike'));
    layout(800);
    expect(lines(m)).toHaveLength(5);
    expect(lines(m).every((l) => l.style.visibility === 'hidden')).toBe(true);

    m.cursor.set(1234);
    expect(lines(m).every((l) => l.style.visibility === 'hidden')).toBe(true); // not before the frame
    env.frame();
    const x = env.charts[0].u.valToPos(m.data.time[1234], 'x');
    for (const l of lines(m)) {
      expect(l.style.transform).toBe(`translateX(${x}px)`);
      expect(l.style.visibility).toBe('visible');
    }

    layout(500);
    const x2 = env.charts[0].u.valToPos(m.data.time[1234], 'x');
    expect(x2).not.toBe(x);
    for (const l of lines(m)) expect(l.style.transform).toBe(`translateX(${x2}px)`);

    m.cursor.set(null);
    env.frame();
    expect(lines(m).every((l) => l.style.visibility === 'hidden')).toBe(true);
  });

  it('readout shows every present value at the cursor, – for missing samples and for no cursor', () => {
    const data = heartRateDropout(golden('ride-4h'));
    const m = setup(data);
    layout(800);
    const fields = readoutFields(data);
    expect(readout(m)).toEqual(fields.map(() => '–'));

    m.cursor.set(100);
    env.frame();
    expect(readout(m)).toEqual(fields.map((f) => f.text(100)));
    expect(readout(m).every((t) => t !== '–')).toBe(true);

    m.cursor.set(3900); // inside the heart-rate dropout
    env.frame();
    const hr = fields.findIndex((f) => f.key === 'heart_rate');
    expect(readout(m)[hr]).toBe('–');
    expect(readout(m).filter((t) => t === '–')).toHaveLength(1);

    m.cursor.set(null);
    env.frame();
    expect(readout(m)).toEqual(fields.map(() => '–'));
  });

  it('a throwing extra subscriber does not freeze the lines or the readout (D-14)', () => {
    const boom = new Error('map not ready');
    const m = setup(golden('ride-4h'), {
      before: (c) => {
        c.subscribe((i) => {
          if (i !== null) throw boom;
        });
      },
    });
    layout(800);
    m.cursor.set(500);
    env.frame();
    const x = env.charts[0].u.valToPos(m.data.time[500], 'x');
    expect(lines(m).every((l) => l.style.transform === `translateX(${x}px)`)).toBe(true);
    expect(readout(m)[0]).toBe(readoutFields(m.data)[0].text(500));
    expect(m.reported).toEqual([boom]);
  });

  it('refreshes the cached plot rectangle on pointerenter, pointerdown, scroll and resize only', () => {
    const m = setup(golden('ride-4h'));
    layout(800);
    const over = overs(m)[1];
    const u = env.charts[1].u;
    // Spy on this element's rect to observe the cache; jsdom's own rect is all zeros.
    const rect = vi.spyOn(over, 'getBoundingClientRect').mockReturnValue(new DOMRect(100, 0, plotWidth(800), 120));

    pointer(over, 'pointerenter', 150);
    expect(rect).toHaveBeenCalledTimes(1);
    pointer(over, 'pointermove', 400);
    pointer(over, 'pointermove', 450);
    expect(rect).toHaveBeenCalledTimes(1);
    expect(m.cursor.get()).toBe(nearestIndex(m.data.time, u.posToVal(350, 'x')));

    window.dispatchEvent(new Event('scroll'));
    expect(rect).toHaveBeenCalledTimes(1); // marked stale, read lazily
    pointer(over, 'pointermove', 460);
    expect(rect).toHaveBeenCalledTimes(2);

    layout(700);
    pointer(over, 'pointermove', 470);
    expect(rect).toHaveBeenCalledTimes(3);
    pointer(over, 'pointermove', 480);
    expect(rect).toHaveBeenCalledTimes(3);

    pointer(over, 'pointerdown', 480);
    expect(rect).toHaveBeenCalledTimes(4);
  });

  it('touch tap: down at one x, up at another leaves the cursor at the up index (QA-05-r2-3 a)', () => {
    const m = setup(golden('ride-4h'));
    layout(800);
    const over = overs(m)[0];
    const u = env.charts[0].u;
    const touch = { pointerType: 'touch', pointerId: 3 };
    pointer(over, 'pointerdown', 100, touch);
    expect(m.cursor.get()).toBe(nearestIndex(m.data.time, u.posToVal(100, 'x')));
    pointer(over, 'pointerup', 500, touch);
    expect(m.cursor.get()).toBe(nearestIndex(m.data.time, u.posToVal(500, 'x')));
    expect(m.cursor.get()).not.toBe(nearestIndex(m.data.time, u.posToVal(100, 'x')));
  });

  it('readout values are plain left-aligned text: no inline width or min-width (user 29 Oct 2026)', () => {
    const m = setup(golden('ride-12h'));
    const cells = [...m.target.querySelectorAll<HTMLSpanElement>('.readout .value')];
    expect(cells).toHaveLength(readoutFields(m.data).length);
    for (const cell of cells) expect(cell.getAttribute('style')).toBeNull();
  });

  describe('scroll lock (D-18)', () => {
    // jsdom has TouchEvent but no Touch, so a TouchEvent's `touches` cannot be filled.
    // The handler reads only `cancelable` and `touches.length`: a plain Event with a
    // `touches` property stands in for it.
    function touchmove(el: Element, touches: number, cancelable = true): Event {
      const ev = new Event('touchmove', { bubbles: true, cancelable });
      Object.defineProperty(ev, 'touches', { value: { length: touches } });
      el.dispatchEvent(ev);
      return ev;
    }

    function start() {
      const m = setup(golden('ride-4h'));
      layout(800);
      return { m, over: overs(m)[1] };
    }
    const t = { pointerType: 'touch', pointerId: 4 };

    it('prevents scrolling only after a horizontal touch move of at least 8 px, with one touch', () => {
      const { over } = start();
      pointer(over, 'pointerdown', 100, { ...t, clientY: 50 });
      expect(touchmove(over, 1).defaultPrevented).toBe(false); // open
      pointer(over, 'pointermove', 105, { ...t, clientY: 51 });
      expect(touchmove(over, 1).defaultPrevented).toBe(false); // inside the slop
      pointer(over, 'pointermove', 108, { ...t, clientY: 52 });
      expect(touchmove(over, 1).defaultPrevented).toBe(true); // scrub
      pointer(over, 'pointermove', 110, { ...t, clientY: 200 });
      expect(touchmove(over, 1).defaultPrevented).toBe(true); // still scrub after vertical drift
      expect(touchmove(over, 2).defaultPrevented).toBe(false); // two touches: pinch allowed
      pointer(over, 'pointerup', 110, { ...t, clientY: 200 });
      expect(touchmove(over, 1).defaultPrevented).toBe(false); // after up
    });

    it('never prevents after a vertical move', () => {
      const { over } = start();
      pointer(over, 'pointerdown', 100, { ...t, clientY: 50 });
      pointer(over, 'pointermove', 101, { ...t, clientY: 60 });
      expect(touchmove(over, 1).defaultPrevented).toBe(false);
      pointer(over, 'pointermove', 300, { ...t, clientY: 60 });
      expect(touchmove(over, 1).defaultPrevented).toBe(false); // pan stays pan
    });

    it('never prevents for mouse input', () => {
      const { over } = start();
      pointer(over, 'pointerdown', 100, { clientY: 50 });
      pointer(over, 'pointermove', 200, { clientY: 50 });
      expect(touchmove(over, 1).defaultPrevented).toBe(false);
    });

    it('does not call preventDefault on a non-cancelable touchmove', () => {
      const { over } = start();
      pointer(over, 'pointerdown', 100, { ...t, clientY: 50 });
      pointer(over, 'pointermove', 120, { ...t, clientY: 50 });
      const ev = new Event('touchmove', { bubbles: true, cancelable: false });
      Object.defineProperty(ev, 'touches', { value: { length: 1 } });
      const spy = vi.spyOn(ev, 'preventDefault');
      over.dispatchEvent(ev);
      expect(spy).not.toHaveBeenCalled();
      expect(touchmove(over, 1).defaultPrevented).toBe(true); // the same touch, cancelable
    });
  });

  it('cleans up on unmount: listeners removed, uPlot destroyed, unsubscribed, observer disconnected', () => {
    type Call = { target: EventTarget; type: string; fn: unknown; capture: boolean; passive: boolean | undefined };
    const added: Call[] = [];
    const removed: Call[] = [];
    const record = (list: Call[], target: EventTarget, type: string, fn: unknown, opts?: boolean | EventListenerOptions) =>
      list.push({
        target,
        type,
        fn,
        capture: typeof opts === 'boolean' ? opts : !!opts?.capture,
        passive: typeof opts === 'object' ? (opts as AddEventListenerOptions).passive : undefined,
      });
    const add = EventTarget.prototype.addEventListener;
    const remove = EventTarget.prototype.removeEventListener;
    vi.spyOn(EventTarget.prototype, 'addEventListener').mockImplementation(function (
      this: EventTarget,
      type: string,
      fn: EventListenerOrEventListenerObject | null,
      opts?: boolean | AddEventListenerOptions,
    ) {
      record(added, this, type, fn, opts);
      add.call(this, type, fn, opts);
    });
    vi.spyOn(EventTarget.prototype, 'removeEventListener').mockImplementation(function (
      this: EventTarget,
      type: string,
      fn: EventListenerOrEventListenerObject | null,
      opts?: boolean | EventListenerOptions,
    ) {
      record(removed, this, type, fn, opts);
      remove.call(this, type, fn, opts);
    });
    // jsdom defines addEventListener on the window object itself, not via EventTarget.prototype.
    const winAdd = window.addEventListener.bind(window);
    const winRemove = window.removeEventListener.bind(window);
    vi.spyOn(window, 'addEventListener').mockImplementation(
      (type: string, fn: EventListenerOrEventListenerObject, opts?: boolean | AddEventListenerOptions) => {
        record(added, window, type, fn, opts);
        winAdd(type, fn, opts);
      },
    );
    vi.spyOn(window, 'removeEventListener').mockImplementation(
      (type: string, fn: EventListenerOrEventListenerObject, opts?: boolean | EventListenerOptions) => {
        record(removed, window, type, fn, opts);
        winRemove(type, fn, opts);
      },
    );

    const m = setup(golden('ride-4h'));
    layout(800);
    const o = overs(m);
    const ours = (c: Call) =>
      (c.target instanceof HTMLElement && c.target.classList.contains('u-over')) || (c.target === window && c.type === 'scroll');
    const ourAdds = added.filter(ours);
    // Per chart: six pointer listeners, one contextmenu (F-17 R-5) and one touchmove on u.over,
    // one scroll listener on window.
    expect(ourAdds.length).toBe(6 * 9);
    expect(ourAdds.filter((c) => c.type === 'contextmenu')).toHaveLength(6);
    const touchmoves = ourAdds.filter((c) => c.type === 'touchmove');
    expect(touchmoves).toHaveLength(6);
    expect(touchmoves.every((c) => c.passive === false && !c.capture)).toBe(true);
    const scrolls = ourAdds.filter((c) => c.type === 'scroll');
    expect(scrolls.every((c) => c.capture && c.passive === true)).toBe(true);
    expect(ourAdds.filter((c) => c.type.startsWith('pointer')).every((c) => c.passive === true)).toBe(true);
    expect(m.active()).toBe(1 + 6); // readout + six charts

    pointer(o[0], 'pointermove', 300);
    const before = m.cursor.get();
    unmount(m.app);
    flushSync();
    current = null;
    m.target.remove();

    // Each add has a remove with the same target, type, listener and capture flag (QA-05-r2-3 b).
    for (const a of ourAdds) {
      const match = removed.some((r) => r.target === a.target && r.type === a.type && r.fn === a.fn && r.capture === a.capture);
      expect(match, `${a.type} removed with capture ${a.capture}`).toBe(true);
    }
    expect(env.charts.every((r) => r.destroyed)).toBe(true);
    expect(m.active()).toBe(0);
    expect(env.observers.every((ob) => ob.disconnected)).toBe(true);
    pointer(o[0], 'pointermove', 600);
    expect(m.cursor.get()).toBe(before);
  });
});


describe('units and scheme (F-14 D-10, D-11)', () => {
  it('a change recreates the uPlot instances and the readout, keeps the root, the area height and the cursor', () => {
    const data = golden('ride-4h');
    const cursor = createCursorStore(data.sampleCount, (f) => f());
    const h = lookHarness(data, cursor);
    const target = document.createElement('div');
    document.body.appendChild(target);
    const app = mount(ChartStack, { target, props: h.props });
    flushSync();
    current = { data, cursor, target, reported: [], ready: () => 0, active: () => 0, app };
    layout(900);
    cursor.set(100);
    const root = target.querySelector('.chart-stack');
    const before = env.charts.length;
    expect(before).toBeGreaterThan(0);
    const height = target.querySelector<HTMLElement>('.chart-area')?.style.height;
    expect(target.querySelector('.readout')?.textContent).toContain('km');
    h.set('imperial', 'dark');
    flushSync();
    layout(900);
    expect(target.querySelector('.chart-stack')).toBe(root);
    expect(env.charts.slice(0, before).every((r) => r.destroyed)).toBe(true);
    expect(env.charts.length).toBe(2 * before);
    expect(target.querySelectorAll('.u-over').length).toBe(before);
    expect(target.querySelector<HTMLElement>('.chart-area')?.style.height).toBe(height);
    expect(target.querySelector('.readout')?.textContent).toContain('mi');
    expect(target.querySelector('.readout')?.textContent).toContain('ft');
    expect(cursor.get()).toBe(100);
    expect(target.querySelectorAll('.readout .value')[0].textContent).not.toBe('–'); // the cursor value, not a reset
  });
});

describe('F-17 range selection and x-axis toggle', () => {
  const withSelection = (data: WorkoutData) => {
    const selection = createSelectionStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
    const m = setup(data, { selection });
    layout(900);
    return { m, selection };
  };
  const shades = (m: Mounted) => [...m.target.querySelectorAll<HTMLDivElement>('.wx-range-shade')];
  const toggle = (m: Mounted) => [...m.target.querySelectorAll<HTMLButtonElement>('.axis-toggle button')];
  const t = { pointerType: 'touch', pointerId: 5 };

  afterEach(() => vi.useRealTimers());

  it('AC 1: no toolbar without distance; a ride has the Time / Distance toggle, time first', () => {
    const indoor = setup(golden('indoor-no-distance-no-gps'));
    layout(900);
    expect(indoor.target.querySelector('.toolbar')).toBeNull();
    unmount(indoor.app);
    indoor.target.remove();
    const { m } = withSelection(golden('ride-4h'));
    expect(toggle(m).map((b) => [b.textContent, b.getAttribute('aria-pressed')])).toEqual([
      ['Time', 'true'],
      ['Distance', 'false'],
    ]);
  });

  it('AP-5: opens on initialAxis; only a toggle click reports the choice', () => {
    const picked: string[] = [];
    const m = setup(golden('ride-4h'), { initialAxis: 'distance', onaxis: (a) => picked.push(a) });
    layout(900);
    expect(toggle(m).map((b) => b.getAttribute('aria-pressed'))).toEqual(['false', 'true']);
    expect(picked).toEqual([]);
    toggle(m)[0].click();
    flushSync();
    expect(toggle(m)[0].getAttribute('aria-pressed')).toBe('true');
    expect(picked).toEqual(['time']);
  });

  it('AP-5: without distance, initialAxis distance draws on time and reports nothing', () => {
    const picked: string[] = [];
    const m = setup(golden('indoor-no-distance-no-gps'), { initialAxis: 'distance', onaxis: (a) => picked.push(a) });
    layout(900);
    expect(m.target.querySelector('.toolbar')).toBeNull();
    expect(env.charts.length).toBeGreaterThan(0);
    expect(picked).toEqual([]);
  });

  it('a distance that never advances keeps the reserved toolbar row but shows no toggle (R-8)', () => {
    const d = golden('ride-4h');
    const m = setup({ ...d, metrics: { ...d.metrics, distance: new Float64Array(d.sampleCount).fill(5) } });
    layout(900);
    expect(m.target.querySelector('.toolbar')).not.toBeNull();
    expect(toggle(m)).toEqual([]);
  });

  it('AC 1: switching redraws every chart on the distance axis; cursor and range stay', () => {
    const data = golden('ride-4h');
    const { m, selection } = withSelection(data);
    m.cursor.set(1234);
    selection.set(1000, 2000);
    env.frame();
    const before = env.charts.length;
    toggle(m)[1].click();
    flushSync();
    expect(env.charts.slice(0, before).every((r) => r.destroyed)).toBe(true);
    expect(env.charts.length).toBe(2 * before);
    expect(m.cursor.get()).toBe(1234);
    expect([selection.start(), selection.end()]).toEqual([1000, 2000]);
    const ax = distanceAxis(data.metrics.distance as Float64Array);
    if (!ax) throw new Error('no distance axis');
    const fresh = env.charts.slice(before);
    for (const r of fresh) {
      const x = r.data.at(-1)?.x ?? [];
      expect(x[0]).toBeGreaterThanOrEqual(ax.min);
      expect(x.at(-1)).toBeLessThanOrEqual(ax.max);
    }
    // The pointer maps to the sample nearest its distance.
    const o = overs(m);
    pointer(o[0], 'pointermove', 300);
    const k = nearestIndex(ax.keys, fresh[0].u.posToVal(300, 'x'));
    expect(m.cursor.get()).toBe((ax.index as Uint32Array)[k]);
    // And back.
    toggle(m)[0].click();
    flushSync();
    expect(toggle(m)[0].getAttribute('aria-pressed')).toBe('true');
    expect(m.cursor.get()).toBe((ax.index as Uint32Array)[k]);
  });

  it('shading follows the store on every chart, after a flush; clear hides it', () => {
    const data = golden('ride-4h');
    const { m, selection } = withSelection(data);
    expect(shades(m).length).toBe(overs(m).length);
    expect(shades(m).every((s) => s.style.visibility === 'hidden')).toBe(true);
    selection.set(100, 500);
    env.frame();
    const u = env.charts[0].u;
    const left = u.valToPos(data.time[100], 'x');
    const width = u.valToPos(data.time[500], 'x') - left;
    expect(shades(m)[0].style.transform).toBe(`translateX(${left}px) scaleX(${width})`);
    expect(shades(m).every((s) => s.style.visibility === 'visible')).toBe(true);
    selection.clear();
    env.frame();
    expect(shades(m).every((s) => s.style.visibility === 'hidden')).toBe(true);
  });

  it('AC 2: a mouse drag selects from the press sample to the pointer sample, live', () => {
    const data = golden('ride-4h');
    const { m, selection } = withSelection(data);
    const o = overs(m)[1];
    const u = env.charts[1].u;
    const at = (x: number) => nearestIndex(data.time, u.posToVal(x, 'x'));
    pointer(o, 'pointerdown', 100);
    pointer(o, 'pointermove', 102);
    expect(selection.start()).toBe(-1); // inside the slop
    pointer(o, 'pointermove', 300);
    expect([selection.start(), selection.end()]).toEqual([at(100), at(300)]);
    pointer(o, 'pointermove', 50);
    pointer(o, 'pointerup', 50);
    expect([selection.start(), selection.end()]).toEqual([at(50), at(100)]);
    expect(m.cursor.get()).toBe(at(50));
  });

  it('AC 4: a click inside the range keeps it, a click outside clears it', () => {
    const { m, selection } = withSelection(golden('ride-4h'));
    const o = overs(m)[0];
    const u = env.charts[0].u;
    const at = (x: number) => nearestIndex(golden('ride-4h').time, u.posToVal(x, 'x'));
    selection.set(at(200), at(400));
    pointer(o, 'pointerdown', 300);
    pointer(o, 'pointerup', 300);
    expect(selection.start()).toBe(at(200));
    pointer(o, 'pointerdown', 600);
    expect(selection.start()).toBe(at(200)); // never on down
    pointer(o, 'pointerup', 600);
    expect(selection.start()).toBe(-1);
  });

  it('touch: long press, then drag selects and blocks scrolling; no context menu while held', () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const data = golden('ride-4h');
    const { m, selection } = withSelection(data);
    const o = overs(m)[2];
    const u = env.charts[2].u;
    const at = (x: number) => nearestIndex(data.time, u.posToVal(x, 'x'));
    pointer(o, 'pointerdown', 200, { ...t, clientY: 50 });
    const menu = new Event('contextmenu', { cancelable: true });
    o.dispatchEvent(menu);
    expect(menu.defaultPrevented).toBe(true);
    vi.advanceTimersByTime(LONG_PRESS_MS - 1);
    pointer(o, 'pointermove', 400, { ...t, clientY: 50 });
    pointer(o, 'pointercancel', 400, t);
    expect(selection.start()).toBe(-1); // moved past the slop before the long press: a scrub
    pointer(o, 'pointerdown', 200, { ...t, clientY: 50 });
    vi.advanceTimersByTime(LONG_PRESS_MS);
    const ev = new Event('touchmove', { bubbles: true, cancelable: true });
    Object.defineProperty(ev, 'touches', { value: { length: 1 } });
    o.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true); // before any movement
    pointer(o, 'pointermove', 420, { ...t, clientY: 120 });
    pointer(o, 'pointerup', 420, { ...t, clientY: 120 });
    expect([selection.start(), selection.end()]).toEqual([at(200), at(420)]);
    const after = new Event('contextmenu', { cancelable: true });
    o.dispatchEvent(after);
    expect(after.defaultPrevented).toBe(false);
  });

  it('AC 9: scrubbing with a range keeps the range and moves the cursor', () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const data = golden('ride-4h');
    const { m, selection } = withSelection(data);
    selection.set(10, 20);
    const o = overs(m)[0];
    pointer(o, 'pointerdown', 300, t);
    pointer(o, 'pointermove', 320, t);
    vi.advanceTimersByTime(LONG_PRESS_MS * 2);
    pointer(o, 'pointermove', 500, t);
    pointer(o, 'pointerup', 500, t);
    expect([selection.start(), selection.end()]).toEqual([10, 20]);
    expect(m.cursor.get()).toBe(nearestIndex(data.time, env.charts[0].u.posToVal(500, 'x')));
  });

  it('R-12: the selection hint is under the charts, both wordings, only with charts', () => {
    const { m } = withSelection(golden('ride-4h'));
    const hint = m.target.querySelector('.select-hint');
    expect(hint?.previousElementSibling?.classList.contains('chart-area')).toBe(true);
    expect(hint?.querySelector('.fine')?.textContent).toBe('Drag across a chart to select a range.');
    expect(hint?.querySelector('.coarse')?.textContent).toBe('Touch and hold a chart, then drag to select a range.');
    unmount(m.app);
    m.target.remove();
    const none = setup(timeOnly(50));
    layout(900);
    expect(none.target.querySelector('.select-hint')).toBeNull();
  });

  it('F-39 HU-3, HU-5: plain has no ×; once adds "Dismiss hint", which only reports; done keeps an empty line; none has no line', () => {
    const ride = golden('ride-4h');
    const hintOf = (hint?: 'plain' | 'once' | 'done' | 'none', onhint?: () => void): Element | null => {
      if (current) {
        unmount(current.app);
        current.target.remove();
      }
      const m = setup(ride, { hint, onhint });
      layout(900);
      return m.target.querySelector('.select-hint');
    };
    expect(hintOf()?.querySelector('button')).toBeNull();
    expect(hintOf('plain')?.querySelector('button')).toBeNull();
    const onhint = vi.fn();
    const once = hintOf('once', onhint)!;
    const x = once.querySelector('button')!;
    expect([x.getAttribute('aria-label'), x.textContent, once.lastElementChild === x]).toEqual(['Dismiss hint', '×', true]);
    x.focus();
    x.click();
    expect(onhint).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(once.closest('.chart-stack')); // QA-39-r1-2: not body when the × goes
    const done = hintOf('done')!;
    expect([done.childElementCount, done.textContent]).toEqual([0, '']);
    expect(hintOf('none')).toBeNull();
  });
});
