// @vitest-environment jsdom
// RouteMap's static glue (F-06 design, Tests; ADR-012, ADR-013; AC 1, AC 5). The lazy
// adapter './map-view' is mocked (vi.mock): this file checks when it is reached, with what,
// and what happens around it. The touch-quiet gate is the real one, driven by a manual
// requestAnimationFrame. Synthetic data only.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { createCursorStore, type CursorStore } from '../../lib/cursor';
import type { WorkoutData } from '../../lib/datafile';
import { golden, hidePositions, positioned } from '../../lib/route/testing/fixtures';
import type { MountOptions, RouteMapHandle } from './map-view';
import RouteMap from './RouteMap.svelte';
import { routeMapHarness, type RouteMapHarness, type RouteMapTestProps } from './route-map-props.test-helper.svelte';

const mv = vi.hoisted(() => ({
  calls: [] as MountOptions[],
  handles: [] as { destroy: ReturnType<typeof vi.fn>; setHidden: ReturnType<typeof vi.fn> }[],
  /** resolve: mount resolves at once; reject: rejects; defer: resolves when release() is called. */
  mode: 'resolve' as 'resolve' | 'reject' | 'defer',
  release: null as null | (() => void),
}));

vi.mock('./map-view', () => ({
  mountRouteMap: (o: MountOptions): Promise<RouteMapHandle> => {
    mv.calls.push(o);
    const handle = { destroy: vi.fn(), setScheme: vi.fn(), setHidden: vi.fn() };
    mv.handles.push(handle);
    if (mv.mode === 'reject') return Promise.reject(new Error('GPUInitializationError (test)'));
    if (mv.mode === 'defer') {
      return new Promise((resolve) => {
        mv.release = () => resolve(handle);
      });
    }
    return Promise.resolve(handle);
  },
}));

// ---- environment -------------------------------------------------------------------

const frames: FrameRequestCallback[] = [];
beforeEach(() => {
  window.requestAnimationFrame = (f: FrameRequestCallback): number => {
    frames.push(f);
    return frames.length;
  };
  mv.calls = [];
  mv.handles = [];
  mv.mode = 'resolve';
  mv.release = null;
});

interface Mounted {
  target: HTMLDivElement;
  app: ReturnType<typeof mount>;
  h: RouteMapHarness;
  ready: () => number;
}
let current: Mounted | null = null;

afterEach(() => {
  if (current) {
    unmount(current.app);
    flushSync();
    current.target.remove();
    current = null;
  }
  frames.length = 0;
  vi.restoreAllMocks();
});

const store = (d: WorkoutData): CursorStore => createCursorStore(d.sampleCount, () => {});

function setup(data: WorkoutData, extra: Partial<RouteMapTestProps> = {}): Mounted {
  let ready = 0;
  const h = routeMapHarness({ data, cursor: store(data), onmapready: () => ready++, ...extra });
  const target = document.createElement('div');
  document.body.appendChild(target);
  const app = mount(RouteMap, { target, props: h.props });
  flushSync();
  current = { target, app, h, ready: () => ready };
  return current;
}

/** Runs animation frames and timers until the async start-up chain has had its chance. */
async function run(): Promise<void> {
  for (let k = 0; k < 5; k++) {
    for (const f of frames.splice(0)) f(0);
    await new Promise((r) => setTimeout(r, 0));
  }
}

const ride = () => golden('ride-4h');
const container = (m: Mounted) => m.target.querySelector<HTMLDivElement>('.route-map');

// ---- tests ---------------------------------------------------------------------------

describe('RouteMap: presence (AP-3, AC 5)', () => {
  it('no position stream: the exact hint, and the map is never requested, also after load', async () => {
    const m = setup(golden('indoor-no-gps'));
    expect(m.target.textContent?.trim()).toBe('No location data in this workout.');
    expect(m.target.querySelector('p.map-hint')).not.toBeNull();
    expect(container(m)).toBeNull();
    m.h.set({ load: true });
    flushSync();
    await run();
    expect(mv.calls).toHaveLength(0);
  });

  it('every position hidden: nothing rendered, never requested (OQ-15 default)', async () => {
    const m = setup(hidePositions(ride(), () => true));
    expect(m.target.textContent?.trim()).toBe('');
    expect(m.target.querySelector('div, p')).toBeNull();
    m.h.set({ load: true });
    flushSync();
    await run();
    expect(mv.calls).toHaveLength(0);
  });
});

describe('RouteMap: scheme (F-14 D-12)', () => {
  it('the scheme is passed at mount; a change reaches the same map without a remount', async () => {
    const m = setup(ride(), { load: true, scheme: 'dark' });
    await run();
    expect(mv.calls).toHaveLength(1);
    expect(mv.calls[0].scheme).toBe('dark');
    const setScheme = (mv.handles[0] as unknown as { setScheme: ReturnType<typeof vi.fn> }).setScheme;
    m.h.set({ scheme: 'light' });
    flushSync();
    expect(setScheme).toHaveBeenLastCalledWith('light');
    expect(mv.calls).toHaveLength(1);
  });
});

describe('RouteMap: to-be-hidden marks (F-19 E-9)', () => {
  it('the ranges reach the map after mount and on every change, without a remount; none is []', async () => {
    const m = setup(ride(), { load: true, hidden: [[0, 9]] });
    await run();
    const setHidden = mv.handles[0].setHidden;
    expect(setHidden).toHaveBeenLastCalledWith([[0, 9]]);
    m.h.set({ hidden: undefined });
    flushSync();
    expect(setHidden).toHaveBeenLastCalledWith([]);
    m.h.set({ hidden: [[3, 4]] });
    flushSync();
    expect(setHidden).toHaveBeenLastCalledWith([[3, 4]]);
    expect(mv.calls).toHaveLength(1);
  });

  it('the viewing page (no prop) marks nothing', async () => {
    setup(ride(), { load: true });
    await run();
    expect(mv.handles[0].setHidden).toHaveBeenLastCalledWith([]);
  });
});

describe('RouteMap: loading (AC 1)', () => {
  it('reserves the height before load; not requested while load is false; once after it turns true', async () => {
    const m = setup(ride());
    expect(container(m)?.style.height).toBe('320px');
    await run();
    expect(mv.calls).toHaveLength(0);

    m.h.set({ load: true });
    flushSync();
    await run();
    expect(mv.calls).toHaveLength(1);
    const o = mv.calls[0];
    expect(o.container).toBe(container(m));
    expect(o.data).toBe(m.h.props.data);
    expect(o.cursor).toBe(m.h.props.cursor);
    expect(o.styleUrl).toBeUndefined();
    expect(o.signal.aborted).toBe(false);
    expect(typeof o.whenQuiet).toBe('function');
    expect(m.ready()).toBe(1);

    // load back to false and true again: no unload, no second mount.
    m.h.set({ load: false });
    flushSync();
    await run();
    m.h.set({ load: true });
    flushSync();
    await run();
    expect(mv.calls).toHaveLength(1);
    expect(mv.handles[0].destroy).not.toHaveBeenCalled();
    expect(m.ready()).toBe(1);
  });

  it('uses the height prop and passes styleUrl', async () => {
    const m = setup(ride(), { height: 200, styleUrl: 'https://tiles.invalid/styles/liberty', load: true });
    expect(container(m)?.style.height).toBe('200px');
    await run();
    expect(mv.calls[0].styleUrl).toBe('https://tiles.invalid/styles/liberty');
  });

  it('waits for a touch-quiet moment (no finger on the screen) before requesting the map', async () => {
    // A plain Event with `touches` stands in for TouchEvent (jsdom has no Touch; ADR-012 rev. 2).
    const touch = (type: string, touches: number) => {
      const ev = new Event(type);
      Object.defineProperty(ev, 'touches', { value: { length: touches } });
      window.dispatchEvent(ev);
    };
    const m = setup(ride());
    touch('touchstart', 1);
    m.h.set({ load: true });
    flushSync();
    await run();
    expect(mv.calls).toHaveLength(0);
    touch('touchend', 0);
    await new Promise((r) => setTimeout(r, 260));
    await run();
    expect(mv.calls).toHaveLength(1);
  });

  it('a mount rejection shows the failure text and throws nothing from the component', async () => {
    // Every microtask still runs (Svelte uses them too); only what throws is captured.
    const thrown: unknown[] = [];
    const original = globalThis.queueMicrotask.bind(globalThis);
    vi.spyOn(globalThis, 'queueMicrotask').mockImplementation((cb) =>
      original(() => {
        try {
          cb();
        } catch (error) {
          thrown.push(error);
        }
      }),
    );
    mv.mode = 'reject';
    const m = setup(ride(), { load: true });
    await run();
    expect(container(m)?.textContent).toBe('The map could not be loaded.');
    expect(container(m)?.children).toHaveLength(0);
    expect(m.ready()).toBe(0);
    // The error goes to the console through a microtask, not to the page.
    expect(thrown).toHaveLength(1);
    expect(String(thrown[0])).toContain('GPUInitializationError (test)');
  });

  it('unmount before the mount resolves: destroy once, onmapready never', async () => {
    mv.mode = 'defer';
    const m = setup(ride(), { load: true });
    await run();
    expect(mv.calls).toHaveLength(1);
    const signal = mv.calls[0].signal;
    unmount(m.app);
    flushSync();
    current = null;
    expect(signal.aborted).toBe(true);
    mv.release?.();
    await run();
    expect(mv.handles[0].destroy).toHaveBeenCalledTimes(1);
    expect(m.ready()).toBe(0);
  });

  it('unmount after the mount: destroy once', async () => {
    const m = setup(ride(), { load: true });
    await run();
    expect(m.ready()).toBe(1);
    unmount(m.app);
    flushSync();
    current = null;
    await run();
    expect(mv.handles[0].destroy).toHaveBeenCalledTimes(1);
  });

  it('a new data value: the old map destroyed, a new one mounted', async () => {
    const m = setup(ride(), { load: true });
    await run();
    const first = container(m);
    const other = golden('gps-dropout');
    m.h.set({ data: other, cursor: store(other) });
    flushSync();
    expect(mv.handles[0].destroy).toHaveBeenCalledTimes(1);
    expect(mv.calls[0].signal.aborted).toBe(true);
    await run();
    expect(mv.calls).toHaveLength(2);
    expect(mv.calls[1].data).toBe(other);
    expect(mv.calls[1].container).toBe(container(m));
    expect(container(m)).not.toBe(first);
    expect(m.ready()).toBe(2);

    // To a workout without positions: the map goes, the hint comes.
    const indoor = golden('indoor-no-gps');
    m.h.set({ data: indoor, cursor: store(indoor) });
    flushSync();
    expect(mv.handles[1].destroy).toHaveBeenCalledTimes(1);
    expect(container(m)).toBeNull();
    expect(m.target.textContent?.trim()).toBe('No location data in this workout.');
  });

  it('throws on a cursor store for another sample count (wiring check)', () => {
    const data = ride();
    const target = document.createElement('div');
    expect(() => {
      mount(RouteMap, { target, props: { data, cursor: createCursorStore(5, () => {}) } });
      flushSync();
    }).toThrow(/cursor store for 5 samples/);
  });

  it('a single positioned sample still gets a map container', () => {
    const m = setup(positioned([Number.NaN, 47, Number.NaN], [Number.NaN, 8, Number.NaN]));
    expect(container(m)).not.toBeNull();
  });
});

describe('RouteMap: a failed chunk', () => {
  it('an import() rejection shows the failure text and throws nothing from the component', async () => {
    const thrown: unknown[] = [];
    const original = globalThis.queueMicrotask.bind(globalThis);
    vi.spyOn(globalThis, 'queueMicrotask').mockImplementation((cb) =>
      original(() => {
        try {
          cb();
        } catch (error) {
          thrown.push(error);
        }
      }),
    );
    // Fresh modules, so this RouteMap's import('./map-view') sees a factory that fails like a
    // chunk that cannot be fetched. Svelte is imported fresh too (one runtime per module graph).
    vi.resetModules();
    vi.doMock('./map-view', () => {
      throw new Error('Failed to fetch dynamically imported module (test)');
    });
    const svelte = await import('svelte');
    const { default: FreshRouteMap } = await import('./RouteMap.svelte');
    const data = ride();
    const target = document.createElement('div');
    document.body.appendChild(target);
    const app = svelte.mount(FreshRouteMap, { target, props: { data, cursor: store(data), load: true } });
    svelte.flushSync();
    try {
      await run();
      expect(target.querySelector('.route-map')?.textContent).toBe('The map could not be loaded.');
      expect(mv.calls).toHaveLength(0);
      expect(thrown).toHaveLength(1);
    } finally {
      svelte.unmount(app);
      svelte.flushSync();
      target.remove();
      vi.doUnmock('./map-view');
    }
  });
});
