// @vitest-environment jsdom
// Wiring of the lazy map adapter (F-06 design, Tests; ADR-012 rev. 3, ADR-013, ADR-015).
// MapLibre is replaced by the recording fake in ./testing/maplibre-fake.ts (it needs WebGL2);
// every decision map-view makes lives in lib/route and map-style.ts with their own Node
// tests. This file checks call order, options, listener options, cleanup, and the
// cursor-to-marker and pointer-to-cursor paths. Synthetic data only.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createCursorStore, type CursorStore } from '../../lib/cursor';
import { createSelectionStore } from '../../lib/selection';
import type { WorkoutData } from '../../lib/datafile';
import { buildRoute, mercatorX } from '../../lib/route';
import { positioned } from '../../lib/route/testing/fixtures';
import { COLOUR_SOURCE, HIDDEN_SOURCE, MAX_MAP_PIXEL_RATIO, OPENFREEMAP_DARK_STYLE_URL, OPENFREEMAP_STYLE_URL, RANGE_SOURCE, ROUTE_SOURCE, mergeStyle } from './map-style';
import { RANGE_SETTLE_MS, mountRouteMap, type RouteMapHandle } from './map-view';
import { createUnitsStore } from '../../lib/units';
import { recorder, resetRecorder, type FakeMap, type FakeMarker } from './testing/maplibre-fake';

vi.mock('maplibre-gl', async () => await import('./testing/maplibre-fake'));
vi.mock('maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url', () => ({ default: '/assets/maplibre-gl-worker-test.js' }));
vi.mock('maplibre-gl/dist/maplibre-gl.css', () => ({}));

// ---- data: a west–east line, 0.01° apart (≈ 233 CSS px at zoom 14), samples 3–4 without position

const N = Number.NaN;
const LON = [8.0, 8.01, 8.02, N, N, 8.05, 8.06, 8.07];
const LAT = [47.0, 47.0, 47.0, N, N, 47.0, 47.0, 47.0];
const ZOOM = 14;
/** Degrees of longitude per CSS px at ZOOM (Mercator x = (lon + 180) / 360). */
const LON_PER_PX = 360 / (512 * 2 ** ZOOM);

// ---- harness ------------------------------------------------------------------------

interface Deferred {
  promise: Promise<void>;
  resolve: () => void;
}
function deferred(): Deferred {
  let resolve: () => void = () => {};
  const promise = new Promise<void>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

interface Setup {
  data: WorkoutData;
  cursor: CursorStore;
  flush: () => void;
  container: HTMLDivElement;
  quiet: Deferred[];
  whenQuiet: () => Promise<void>;
  abort: AbortController;
  fetchMock: ReturnType<typeof vi.fn>;
  styleResponse: Deferred & { json: unknown; fail: boolean };
}

let handles: RouteMapHandle[] = [];

function setup(data: WorkoutData = positioned(LAT, LON)): Setup {
  const queue: (() => void)[] = [];
  const cursor = createCursorStore(data.sampleCount, (f) => queue.push(f));
  const flush = () => {
    for (const f of queue.splice(0)) f();
  };
  const container = document.createElement('div');
  document.body.appendChild(container);
  const quiet: Deferred[] = [];
  const whenQuiet = () => {
    const d = deferred();
    quiet.push(d);
    return d.promise;
  };
  const styleResponse = Object.assign(deferred(), {
    json: { version: 8, sources: { openmaptiles: { type: 'vector', url: 'https://tiles.openfreemap.org/planet' } }, layers: [] } as unknown,
    fail: false,
  });
  const fetchMock = vi.fn(async (_url: string, init: RequestInit) => {
    await styleResponse.promise;
    if (init.signal?.aborted) throw new DOMException('aborted', 'AbortError');
    if (styleResponse.fail) throw new TypeError('network');
    return { ok: true, json: async () => styleResponse.json };
  });
  vi.stubGlobal('fetch', fetchMock);
  return { data, cursor, flush, container, quiet, whenQuiet, abort: new AbortController(), fetchMock, styleResponse };
}

/** Lets pending promise callbacks run. */
async function settle(): Promise<void> {
  for (let k = 0; k < 10; k++) await Promise.resolve();
}

/** Resolves the two quiet moments before new Map (step 5, and step 6a of v1.4). */
async function quietUntilMap(s: Setup): Promise<void> {
  await settle();
  s.quiet[0].resolve();
  await settle();
  s.quiet[1].resolve();
}

/** Mounts and resolves the whenQuiet calls before new Map: the map exists when this returns. */
async function mounted(s: Setup, styleUrl?: string): Promise<{ handle: RouteMapHandle; map: FakeMap; marker: FakeMarker }> {
  const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, styleUrl, signal: s.abort.signal });
  await quietUntilMap(s);
  const handle = await p;
  handles.push(handle);
  const map = recorder.maps[recorder.maps.length - 1];
  map.zoom = ZOOM;
  return { handle, map, marker: recorder.markers[recorder.markers.length - 1] };
}

function pointer(target: Element, type: string, o: Partial<PointerEventInit> = {}): void {
  target.dispatchEvent(
    new PointerEvent(type, { bubbles: true, pointerType: 'mouse', pointerId: 1, isPrimary: true, buttons: 0, ...aimed, ...o }),
  );
}

/** jsdom has TouchEvent but no Touch, so `touches` cannot be filled: a plain Event with a
 *  `touches` property stands in (ADR-012 rev. 2). The handler reads only that and `cancelable`. */
function touchmove(target: Element, touches: number, cancelable = true): Event {
  const ev = new Event('touchmove', { bubbles: true, cancelable });
  Object.defineProperty(ev, 'touches', { value: { length: touches } });
  target.dispatchEvent(ev);
  return ev;
}

/** The fake's linear view (v1.3): x = 0 at longitude LNG0, 1 CSS px = LON_PER_PX; y = 0 at 47°. */
const LNG0 = 7.99;
/** Where the inner container sits in the viewport (jsdom's own rect is all zeros). */
let offset = { left: 0, top: 0 };
/** clientX/clientY the next pointer() uses unless given. */
let aimed = { clientX: 0, clientY: 0 };

/** Map x in CSS px of sample i under the fake's linear view. */
const xOf = (i: number): number => (LON[i] - LNG0) / LON_PER_PX;

/** Aims the next pointer() `px` CSS px east of sample i, in client coordinates. */
function aim(i: number, px = 0): void {
  aimed = { clientX: offset.left + xOf(i) + px, clientY: offset.top };
}

/** Stubs the inner container's rectangle at (left, top). */
function placeInner(map: FakeMap, left: number, top: number): ReturnType<typeof vi.spyOn> {
  offset = { left, top };
  const inner = map.options.container as HTMLElement;
  return vi.spyOn(inner, 'getBoundingClientRect').mockReturnValue(new DOMRect(left, top, 600, 320));
}

beforeEach(() => {
  resetRecorder();
  recorder.view = { lng0: LNG0, lat0: 47, lngPerPx: LON_PER_PX, latPerPx: LON_PER_PX };
  offset = { left: 0, top: 0 };
  aimed = { clientX: 0, clientY: 0 };
});

afterEach(() => {
  for (const h of handles) h.destroy();
  handles = [];
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

// ---- tests ---------------------------------------------------------------------------

describe('mountRouteMap: start-up', () => {
  it('awaits whenQuiet twice before new Map (before and after the route model, v1.4); setWorkerUrl comes first', async () => {
    const s = setup();
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await settle();
    expect(s.quiet).toHaveLength(1);
    expect(recorder.log).toEqual([]);
    s.quiet[0].resolve();
    await settle();
    expect(s.quiet).toHaveLength(2); // step 6a: the route model and new Map never share a task
    expect(recorder.log).toEqual([]);
    expect(performance.getEntriesByName('wx-map:route-model', 'measure')).toHaveLength(1);
    s.quiet[1].resolve();
    handles.push(await p);
    expect(recorder.log.slice(0, 2)).toEqual(['setWorkerUrl', 'new Map']);
    expect(recorder.workerUrl).toBe('/assets/maplibre-gl-worker-test.js');
  });

  it('Map options: container inside ours, base style, bounds, cooperative gestures, no rotation or pitch', async () => {
    const s = setup();
    const { map } = await mounted(s);
    const o = map.options;
    const inner = o.container as HTMLElement;
    expect(inner.parentElement).toBe(s.container);
    expect(inner.className).toBe('wx-map');
    expect(o.bounds).toEqual(buildRoute(s.data)?.bounds);
    expect(o.fitBoundsOptions).toEqual({ padding: 32, maxZoom: 16 });
    expect(o.center).toBeUndefined();
    expect(o).toMatchObject({
      cooperativeGestures: true,
      doubleClickZoom: false,
      dragRotate: false,
      pitchWithRotate: false,
      touchPitch: false,
      maxPitch: 0,
      attributionControl: { compact: true },
      maplibreLogo: false,
      renderWorldCopies: false,
    });
    expect(Object.keys((o.style as { sources: object }).sources)).toContain(ROUTE_SOURCE);
    expect(map.rotationDisabled).toBe(true);
    expect(map.tapDragZoomDisabled).toBe(true); // one-finger tap-and-drag zoom off (v1.2)
    expect(map.controls).toHaveLength(1);
    expect(map.controls[0][1]).toBe('top-right');
    expect((map.controls[0][0] as { options: unknown }).options).toEqual({ showCompass: false });
  });

  it('a single positioned sample: center and zoom 15, no bounds, no route source', async () => {
    const s = setup(positioned([N, 47, N], [N, 8, N]));
    const { map } = await mounted(s);
    expect(map.options.bounds).toBeUndefined();
    expect(map.options.center).toEqual([8, 47]);
    expect(map.options.zoom).toBe(15);
    expect(Object.keys((map.options.style as { sources: object }).sources)).not.toContain(ROUTE_SOURCE);
  });

  it('fetches the style without referrer and credentials; default URL liberty', async () => {
    const s = setup();
    await mounted(s);
    expect(s.fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = s.fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(OPENFREEMAP_STYLE_URL);
    expect(init.referrerPolicy).toBe('no-referrer');
    expect(init.credentials).toBe('omit');
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  it('uses the given styleUrl', async () => {
    const s = setup();
    await mounted(s, 'https://tiles.invalid/styles/liberty');
    expect(s.fetchMock.mock.calls[0][0]).toBe('https://tiles.invalid/styles/liberty');
  });

  it('the dark scheme at mount fetches the dark style; a scheme change repaints our layers and swaps the style, no remount (F-14 D-12)', async () => {
    const s = setup();
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, scheme: 'dark', signal: s.abort.signal });
    await quietUntilMap(s);
    const handle = await p;
    handles.push(handle);
    expect(s.fetchMock.mock.calls[0][0]).toBe(OPENFREEMAP_DARK_STYLE_URL);
    const map = recorder.maps[recorder.maps.length - 1];
    s.styleResponse.resolve();
    map.fire('load');
    await settle();
    s.quiet[2].resolve();
    await settle();
    expect(map.styles).toHaveLength(1);
    handle.setScheme('dark'); // same scheme: nothing
    expect(map.paints).toHaveLength(0);
    s.styleResponse.json = { ...(s.styleResponse.json as object) }; // the other style: a new object
    handle.setScheme('light');
    await settle();
    expect(map.paints.map((x) => x[0])).toContain('wx-route-line');
    expect(map.paints.find((x) => x[0] === 'wx-route-line')?.[2]).toBe('#c2187a'); // no theme in jsdom: the light fallback
    expect(s.fetchMock.mock.calls[1][0]).toBe(OPENFREEMAP_STYLE_URL);
    await settle();
    s.quiet[3].resolve();
    await settle();
    expect(map.styles).toHaveLength(2);
    expect(recorder.maps).toHaveLength(1);
    expect(map.fits).toHaveLength(0);
  });

  it('setStyle only after load and the style JSON, after whenQuiet, with transformStyle = mergeStyle', async () => {
    const s = setup();
    const { map } = await mounted(s);
    s.styleResponse.resolve();
    await settle();
    expect(map.styles).toHaveLength(0); // no load yet
    map.fire('load');
    await settle();
    expect(s.quiet).toHaveLength(3);
    expect(map.styles).toHaveLength(0); // waits for a quiet moment
    s.quiet[2].resolve();
    await settle();
    expect(map.styles).toHaveLength(1);
    expect(map.styles[0][0]).toBe(s.styleResponse.json);
    // v1.4: no diff attempt from the base style to the remote one.
    expect(map.styles[0][1]).toEqual({ diff: false, transformStyle: mergeStyle });
    map.fire('load');
    await settle();
    expect(map.styles).toHaveLength(1); // applied once
  });

  it('load first, the style later: applied once both are there', async () => {
    const s = setup();
    const { map } = await mounted(s);
    map.fire('load');
    await settle();
    expect(s.quiet).toHaveLength(2);
    s.styleResponse.resolve();
    await settle();
    s.quiet[2].resolve();
    await settle();
    expect(map.styles).toHaveLength(1);
  });

  it('a failed or invalid style fetch leaves the base style; nothing thrown', async () => {
    for (const bad of ['network', 'invalid'] as const) {
      const s = setup();
      if (bad === 'network') s.styleResponse.fail = true;
      else s.styleResponse.json = { version: 8, layers: 'nope' };
      const { map } = await mounted(s);
      s.styleResponse.resolve();
      map.fire('load');
      await settle();
      expect(s.quiet).toHaveLength(2);
      expect(map.styles).toHaveLength(0);
    }
  });

  it('swallows map error events (one listener)', async () => {
    const s = setup();
    const { map } = await mounted(s);
    expect(map.listeners.get('error')).toHaveLength(1);
    expect(() => map.fire('error')).not.toThrow();
  });

  it('a throwing Map constructor (no WebGL2): rejects, removes its container, aborts the style fetch', async () => {
    const s = setup();
    recorder.constructorError = new Error('GPUInitializationError (test)');
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await quietUntilMap(s);
    await expect(p).rejects.toThrow('GPUInitializationError (test)');
    expect(s.container.children).toHaveLength(0);
    const init = s.fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.signal?.aborted).toBe(true);
  });

  it('aborted before the quiet moment: no map, the style fetch aborted', async () => {
    const s = setup();
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await settle();
    s.abort.abort();
    s.quiet[0].resolve();
    const handle = await p;
    expect(recorder.log).not.toContain('new Map');
    expect((s.fetchMock.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);
    expect(() => handle.destroy()).not.toThrow();
  });

  it('aborted between the route model and new Map (step 6a): no map, the style fetch aborted', async () => {
    const s = setup();
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await settle();
    s.quiet[0].resolve();
    await settle();
    s.abort.abort();
    s.quiet[1].resolve();
    const handle = await p;
    expect(recorder.log).not.toContain('new Map');
    expect(s.container.children).toHaveLength(0);
    expect((s.fetchMock.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);
    expect(() => handle.destroy()).not.toThrow();
  });

  it('v1.4 options: pixelRatio = min(devicePixelRatio, 2); validateStyle only in development', async () => {
    for (const [dpr, expected] of [
      [3, 2],
      [2, 2],
      [1.5, 1.5],
      [1, 1],
    ] as const) {
      vi.stubGlobal('devicePixelRatio', dpr);
      const s = setup();
      const { map } = await mounted(s);
      expect(map.options.pixelRatio).toBe(expected);
      expect(map.options.validateStyle).toBe(import.meta.env.DEV);
    }
    expect(MAX_MAP_PIXEL_RATIO).toBe(2);
    // Vitest runs in development mode: validation stays on here, as on the dev page.
    expect(import.meta.env.DEV).toBe(true);
  });

  it('User Timing (v1.4): new-map and route-model measures; load mark; set-style measure and idle mark after the remote style', async () => {
    performance.clearMarks();
    performance.clearMeasures();
    const names = (type: 'mark' | 'measure') => performance.getEntriesByType(type).map((e) => e.name);
    const s = setup();
    const { map } = await mounted(s);
    expect(names('measure')).toEqual(['wx-map:route-model', 'wx-map:new-map']);
    expect(names('mark')).toEqual([]);
    map.fire('idle'); // an idle of the base style does not count
    map.fire('load');
    expect(names('mark')).toEqual(['wx-map:load']);
    s.styleResponse.resolve();
    await settle();
    s.quiet[2].resolve();
    await settle();
    expect(map.styles).toHaveLength(1);
    expect(names('measure')).toEqual(['wx-map:route-model', 'wx-map:new-map', 'wx-map:set-style']);
    expect(names('mark')).toEqual(['wx-map:load']);
    map.fire('idle');
    map.fire('idle'); // only the first idle after the remote style
    expect(names('mark')).toEqual(['wx-map:load', 'wx-map:idle']);
  });
});

describe('cursor marker (AC 2, AC 6)', () => {
  it('one Marker with our element, hidden at first, anchored at the centre, subpixel', async () => {
    const s = setup();
    const { marker, map } = await mounted(s);
    expect(recorder.markers).toHaveLength(1);
    expect(marker.map).toBe(map);
    expect(marker.options.anchor).toBe('center');
    expect(marker.options.subpixelPositioning).toBe(true);
    expect(marker.options.element.className).toBe('wx-cursor-marker');
    expect(marker.options.element.style.visibility).toBe('hidden');
  });

  it('follows the cursor to positioned samples; hidden for a sample without position and for null', async () => {
    const s = setup();
    const { marker } = await mounted(s);
    const el = marker.options.element;
    const before = marker.positions.length;

    s.cursor.set(2);
    s.flush();
    expect(marker.positions.slice(before)).toEqual([[LON[2], LAT[2]]]);
    expect(el.style.visibility).toBe('visible');

    s.cursor.set(3); // GPS dropout
    s.flush();
    expect(marker.positions.length).toBe(before + 1); // no setLngLat
    expect(el.style.visibility).toBe('hidden');

    s.cursor.set(6);
    s.flush();
    expect(marker.positions[marker.positions.length - 1]).toEqual([LON[6], LAT[6]]);
    expect(el.style.visibility).toBe('visible');

    s.cursor.set(null);
    s.flush();
    expect(marker.positions.length).toBe(before + 2);
    expect(el.style.visibility).toBe('hidden');
  });

  it('a late map gets the current cursor at once', async () => {
    const s = setup();
    s.cursor.set(5);
    s.flush();
    const { marker } = await mounted(s);
    expect(marker.positions[marker.positions.length - 1]).toEqual([LON[5], LAT[5]]);
    expect(marker.options.element.style.visibility).toBe('visible');
  });
});

describe('route interaction (AC 2, ADR-015)', () => {
  it('mouse hover within 12 px of a sample sets the cursor; farther or with a button pressed does not', async () => {
    const s = setup();
    const { map } = await mounted(s);
    aim(6, 11);
    pointer(map.canvas, 'pointermove');
    expect(s.cursor.get()).toBe(6);

    aim(1, 13);
    pointer(map.canvas, 'pointermove');
    expect(s.cursor.get()).toBe(6);

    aim(1, 0);
    pointer(map.canvas, 'pointermove', { buttons: 1 }); // dragging the map
    expect(s.cursor.get()).toBe(6);

    pointer(map.canvas, 'pointerleave');
    expect(s.cursor.get()).toBe(6);
  });

  it('a pointer whose target is not the canvas never hits the route (map controls, the containers)', async () => {
    const s = setup();
    const { map } = await mounted(s);
    const control = document.createElement('button'); // e.g. the + button over the route
    (map.options.container as HTMLElement).appendChild(control);
    aim(1, 0); // exactly on sample 1
    for (const target of [control, map.canvasContainer, map.options.container as HTMLElement, s.container]) {
      pointer(target, 'pointermove');
      pointer(target, 'pointerdown', { pointerType: 'touch', pointerId: 2 });
      expect(touchmove(target, 1).defaultPrevented).toBe(false); // no scrub, no lock
      pointer(target, 'pointerup', { pointerType: 'touch', pointerId: 2 });
    }
    expect(s.cursor.get()).toBeNull();
    expect(recorder.log).not.toContain('unproject');
    pointer(map.canvas, 'pointermove'); // the same aim on the canvas hits
    expect(s.cursor.get()).toBe(1);
  });

  it('touch down on the route sets the cursor and locks: the touchmove is prevented and hidden from MapLibre', async () => {
    const s = setup();
    const { map } = await mounted(s);
    const reached: Event[] = [];
    map.canvasContainer.addEventListener('touchmove', (e) => reached.push(e));
    const touch = { pointerType: 'touch', pointerId: 7 };

    aim(2, 15); // within 20 px
    pointer(map.canvas, 'pointerdown', touch);
    expect(s.cursor.get()).toBe(2);
    const ev = touchmove(map.canvas, 1);
    expect(ev.defaultPrevented).toBe(true);
    expect(reached).toHaveLength(0);

    aim(5, 40); // within 48 px during a scrub
    pointer(map.canvas, 'pointermove', touch);
    expect(s.cursor.get()).toBe(5);

    expect(touchmove(map.canvas, 2).defaultPrevented).toBe(false); // two touches: MapLibre's
    expect(reached).toHaveLength(1);

    pointer(map.canvas, 'pointerup', touch);
    expect(s.cursor.get()).toBe(5); // the cursor stays
    expect(touchmove(map.canvas, 1).defaultPrevented).toBe(false);
    expect(reached).toHaveLength(2);
  });

  it('touch down off the route: not prevented, reaches MapLibre, cursor unchanged', async () => {
    const s = setup();
    const { map } = await mounted(s);
    const reached: Event[] = [];
    map.canvasContainer.addEventListener('touchmove', (e) => reached.push(e));
    aim(2, 25); // beyond 20 px
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 8 });
    expect(s.cursor.get()).toBeNull();
    expect(touchmove(map.canvas, 1).defaultPrevented).toBe(false);
    expect(reached).toHaveLength(1);
  });

  it('pointercancel restores the cursor from before the touch', async () => {
    const s = setup();
    const { map } = await mounted(s);
    s.cursor.set(1);
    const touch = { pointerType: 'touch', pointerId: 9 };
    aim(6, 0);
    pointer(map.canvas, 'pointerdown', touch);
    expect(s.cursor.get()).toBe(6);
    pointer(map.canvas, 'pointercancel', touch);
    expect(s.cursor.get()).toBe(1);
  });

  it('a non-cancelable touchmove during a scrub: no preventDefault, still hidden from MapLibre', async () => {
    const s = setup();
    const { map } = await mounted(s);
    const reached: Event[] = [];
    map.canvasContainer.addEventListener('touchmove', (e) => reached.push(e));
    aim(2, 0);
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 3 });
    const ev = new Event('touchmove', { bubbles: true, cancelable: false });
    Object.defineProperty(ev, 'touches', { value: { length: 1 } });
    const spy = vi.spyOn(ev, 'preventDefault');
    map.canvas.dispatchEvent(ev);
    expect(spy).not.toHaveBeenCalled();
    expect(reached).toHaveLength(0);
  });

  it('caches the container rectangle: read on pointerenter and pointerdown, again after scroll or resize', async () => {
    const s = setup();
    const { map } = await mounted(s);
    const inner = map.options.container as HTMLElement;
    const rect = vi.spyOn(inner, 'getBoundingClientRect').mockReturnValue(new DOMRect(100, 50, 600, 320));
    pointer(map.canvas, 'pointerenter');
    expect(rect).toHaveBeenCalledTimes(1);
    pointer(map.canvas, 'pointermove');
    pointer(map.canvas, 'pointermove');
    expect(rect).toHaveBeenCalledTimes(1);
    window.dispatchEvent(new Event('scroll'));
    pointer(map.canvas, 'pointermove');
    expect(rect).toHaveBeenCalledTimes(2);
    map.fire('resize');
    pointer(map.canvas, 'pointermove');
    expect(rect).toHaveBeenCalledTimes(3);
    pointer(map.canvas, 'pointerdown');
    expect(rect).toHaveBeenCalledTimes(4);
  });
});

describe('destroy', () => {
  it('unsubscribes, removes every listener with the same capture flag, removes the map, aborts the style fetch', async () => {
    type Call = { target: EventTarget; type: string; fn: unknown; capture: boolean; passive: boolean | undefined };
    const added: Call[] = [];
    const removed: Call[] = [];
    const record = (list: Call[], target: EventTarget, type: string, fn: unknown, opts?: boolean | AddEventListenerOptions) =>
      list.push({
        target,
        type,
        fn,
        capture: typeof opts === 'boolean' ? opts : !!opts?.capture,
        passive: typeof opts === 'object' ? opts.passive : undefined,
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
    // jsdom defines addEventListener on the window object itself.
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

    const s = setup();
    let active = 0;
    const subscribe = s.cursor.subscribe.bind(s.cursor);
    (s.cursor as { subscribe: CursorStore['subscribe'] }).subscribe = (l) => {
      active++;
      const off = subscribe(l);
      return () => {
        active--;
        off();
      };
    };
    // The signal may be Node's EventTarget rather than jsdom's: spied on directly.
    const signalAdd = vi.spyOn(s.abort.signal, 'addEventListener');
    const signalRemove = vi.spyOn(s.abort.signal, 'removeEventListener');
    const { handle, map, marker } = await mounted(s);
    handles = [];
    expect(active).toBe(1);
    expect(signalAdd).toHaveBeenCalledTimes(1);
    expect(signalAdd.mock.calls[0][0]).toBe('abort');

    const ours = added.filter((c) => c.target === s.container || c.target === window);
    expect(ours.map((c) => c.type).sort()).toEqual(
      ['pointercancel', 'pointerdown', 'pointerenter', 'pointerleave', 'pointermove', 'pointerup', 'scroll', 'touchmove'].sort(),
    );
    const onContainer = ours.filter((c) => c.target === s.container);
    expect(onContainer.every((c) => c.capture)).toBe(true);
    expect(onContainer.filter((c) => c.type !== 'touchmove').every((c) => c.passive === true)).toBe(true);
    expect(onContainer.find((c) => c.type === 'touchmove')?.passive).toBe(false);
    const scroll = ours.find((c) => c.type === 'scroll');
    expect(scroll?.capture && scroll.passive).toBe(true);

    handle.destroy();
    for (const a of ours) {
      const match = removed.some((r) => r.target === a.target && r.type === a.type && r.fn === a.fn && r.capture === a.capture);
      expect(match, `${a.type} removed with capture ${a.capture}`).toBe(true);
    }
    expect(signalRemove).toHaveBeenCalledWith('abort', signalAdd.mock.calls[0][1]);
    expect(active).toBe(0);
    expect(map.removed).toBe(true);
    expect(marker.removed).toBe(true);
    expect(s.container.children).toHaveLength(0);
    expect((s.fetchMock.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);

    // A later flush does not touch the marker; pointer events do nothing.
    const count = marker.positions.length;
    s.cursor.set(1);
    s.flush();
    expect(marker.positions.length).toBe(count);
    aim(6, 0);
    pointer(s.container, 'pointermove');
    expect(s.cursor.get()).toBe(1);
    expect(() => handle.destroy()).not.toThrow();

    // A style that arrives after destroy is not applied.
    map.fire('load');
    s.styleResponse.resolve();
    await settle();
    expect(map.styles).toHaveLength(0);
  });
});

describe('hit radius maths', () => {
  it('the test aims in CSS px at the fake zoom (sanity check of LON_PER_PX)', () => {
    const unit = 1 / (512 * 2 ** ZOOM);
    expect(mercatorX(LON[1] + 10 * LON_PER_PX) - mercatorX(LON[1])).toBeCloseTo(10 * unit, 18);
  });
});

describe('hit-test geometry (v1.3, QA-06-r1-3)', () => {
  it('passes [clientX − left, clientY − top] to unproject and selects the sample the view places there', async () => {
    const s = setup();
    const { map } = await mounted(s);
    placeInner(map, 120, 70);
    pointer(map.canvas, 'pointerenter');
    aim(5, 3);
    aimed.clientY += 4;
    pointer(map.canvas, 'pointermove');
    expect(recorder.points[recorder.points.length - 1]).toEqual([aimed.clientX - 120, aimed.clientY - 70]);
    expect(recorder.points[recorder.points.length - 1][0]).toBeCloseTo(xOf(5) + 3, 9);
    expect(recorder.points[recorder.points.length - 1][1]).toBe(4);
    expect(s.cursor.get()).toBe(5);
  });

  it('after a scroll and a moved rectangle, the next pointerenter or pointerdown uses the new one', async () => {
    const s = setup();
    const { map } = await mounted(s);
    placeInner(map, 120, 70);
    pointer(map.canvas, 'pointerenter');
    aim(1);
    pointer(map.canvas, 'pointermove');
    expect(s.cursor.get()).toBe(1);

    // pointerenter after the move
    window.dispatchEvent(new Event('scroll'));
    placeInner(map, 300, 10);
    aim(6);
    pointer(map.canvas, 'pointerenter');
    pointer(map.canvas, 'pointermove');
    expect(recorder.points[recorder.points.length - 1]).toEqual([aimed.clientX - 300, 0]);
    expect(s.cursor.get()).toBe(6);

    // pointerdown after the move (a touch, which hits on down)
    window.dispatchEvent(new Event('scroll'));
    placeInner(map, 40, 500);
    aim(2);
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 4 });
    expect(recorder.points[recorder.points.length - 1]).toEqual([aimed.clientX - 40, 0]);
    expect(s.cursor.get()).toBe(2);
  });

  it('radius per kind: a sample 30 px away is missed by a primary touch down, also one replacing a stale scrub', async () => {
    const s = setup();
    const { map } = await mounted(s);
    aim(5, 30);
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 1 });
    expect(s.cursor.get()).toBeNull();
    expect(touchmove(map.canvas, 1).defaultPrevented).toBe(false);

    // A scrub on sample 2 whose up is never seen, then a new primary touch 30 px from sample 5.
    aim(2);
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 2 });
    expect(s.cursor.get()).toBe(2);
    aim(5, 30);
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 3 });
    expect(s.cursor.get()).toBe(2); // 30 px > 20 px start radius
    expect(touchmove(map.canvas, 1).defaultPrevented).toBe(false); // the stale scrub ended
  });

  it('radius per kind: a move of the active scrub hits a sample 30 px away (48 px)', async () => {
    const s = setup();
    const { map } = await mounted(s);
    aim(2);
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 7 });
    expect(s.cursor.get()).toBe(2);
    aim(5, 30);
    pointer(map.canvas, 'pointermove', { pointerType: 'touch', pointerId: 7 });
    expect(s.cursor.get()).toBe(5);
  });
});

describe('no hit test the reducer would discard (v1.3, QA-06-r1-4)', () => {
  it('a mouse move with a button pressed, a touch move without a scrub, ups and leaves do not unproject', async () => {
    const s = setup();
    const { map } = await mounted(s);
    aim(1);
    pointer(map.canvas, 'pointermove', { buttons: 1 }); // dragging the map
    pointer(map.canvas, 'pointerdown'); // mouse down
    pointer(map.canvas, 'pointerup');
    pointer(map.canvas, 'pointerleave');
    pointer(map.canvas, 'pointermove', { pointerType: 'touch', pointerId: 9 }); // a scroll over the map
    pointer(map.canvas, 'pointerdown', { pointerType: 'touch', pointerId: 10, isPrimary: false });
    expect(recorder.log).not.toContain('unproject');
    expect(s.cursor.get()).toBeNull();
    pointer(map.canvas, 'pointermove'); // a hover does
    expect(recorder.points).toHaveLength(1);
    expect(s.cursor.get()).toBe(1);
  });
});

describe('partial mount (v1.3, QA-06-r1-1)', () => {
  type Call = { target: EventTarget; type: string; fn: unknown; capture: boolean };
  /** Records add/removeEventListener on every EventTarget and on window (jsdom defines window's own). */
  function trackListeners(): { added: Call[]; removed: Call[] } {
    const added: Call[] = [];
    const removed: Call[] = [];
    const rec = (list: Call[], target: EventTarget, type: string, fn: unknown, opts?: boolean | EventListenerOptions) =>
      list.push({ target, type, fn, capture: typeof opts === 'boolean' ? opts : !!opts?.capture });
    const add = EventTarget.prototype.addEventListener;
    const remove = EventTarget.prototype.removeEventListener;
    vi.spyOn(EventTarget.prototype, 'addEventListener').mockImplementation(function (
      this: EventTarget,
      type: string,
      fn: EventListenerOrEventListenerObject | null,
      opts?: boolean | AddEventListenerOptions,
    ) {
      rec(added, this, type, fn, opts);
      add.call(this, type, fn, opts);
    });
    vi.spyOn(EventTarget.prototype, 'removeEventListener').mockImplementation(function (
      this: EventTarget,
      type: string,
      fn: EventListenerOrEventListenerObject | null,
      opts?: boolean | EventListenerOptions,
    ) {
      rec(removed, this, type, fn, opts);
      remove.call(this, type, fn, opts);
    });
    const winAdd = window.addEventListener.bind(window);
    const winRemove = window.removeEventListener.bind(window);
    vi.spyOn(window, 'addEventListener').mockImplementation(
      (type: string, fn: EventListenerOrEventListenerObject, opts?: boolean | AddEventListenerOptions) => {
        rec(added, window, type, fn, opts);
        winAdd(type, fn, opts);
      },
    );
    vi.spyOn(window, 'removeEventListener').mockImplementation(
      (type: string, fn: EventListenerOrEventListenerObject, opts?: boolean | EventListenerOptions) => {
        rec(removed, window, type, fn, opts);
        winRemove(type, fn, opts);
      },
    );
    return { added, removed };
  }

  const cases: { name: string; arrange: (s: Setup) => void; listenersAdded: boolean }[] = [
    { name: 'Marker.addTo throws', arrange: () => (recorder.throwIn = 'Marker.addTo'), listenersAdded: false },
    {
      name: 'cursor.subscribe throws',
      arrange: (s) => {
        (s.cursor as { subscribe: CursorStore['subscribe'] }).subscribe = () => {
          throw new Error('subscribe (test)');
        };
      },
      listenersAdded: false,
    },
    // After the input listeners and the cursor subscription exist.
    { name: "map.on('resize') throws", arrange: () => (recorder.throwIn = 'Map.on:resize'), listenersAdded: true },
  ];

  for (const c of cases) {
    it(`${c.name}: rejects; map removed once, fetch aborted, no listener or subscription left`, async () => {
      const { added, removed } = trackListeners();
      const s = setup();
      let active = 0;
      const subscribe = s.cursor.subscribe.bind(s.cursor);
      (s.cursor as { subscribe: CursorStore['subscribe'] }).subscribe = (l) => {
        active++;
        const off = subscribe(l);
        return () => {
          active--;
          off();
        };
      };
      c.arrange(s);
      const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
      await quietUntilMap(s);
      await expect(p).rejects.toThrow(/test/);

      const map = recorder.maps[0];
      expect(map.removeCount).toBe(1);
      expect((s.fetchMock.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);
      expect(active).toBe(0);
      expect(s.container.children).toHaveLength(0);
      const ours = added.filter((a) => a.target === s.container || a.target === window);
      expect(ours.length > 0).toBe(c.listenersAdded);
      for (const a of ours) {
        const match = removed.some((r) => r.target === a.target && r.type === a.type && r.fn === a.fn && r.capture === a.capture);
        expect(match, `${a.type} removed with capture ${a.capture}`).toBe(true);
      }
      // Nothing reacts any more.
      aim(1);
      pointer(s.container, 'pointermove');
      expect(s.cursor.get()).toBeNull();
    });
  }
});

describe('pixel ratio follows devicePixelRatio (F-07 D-8, QA-06-r4-1)', () => {
  /** A fake MediaQueryList: records its query and its change listeners (ADR-012 allows a
   *  matchMedia stub; jsdom has no matchMedia). */
  interface FakeQuery {
    readonly media: string;
    readonly listeners: Set<EventListener>;
    addEventListener(type: string, fn: EventListener): void;
    removeEventListener(type: string, fn: EventListener): void;
  }
  let queries: FakeQuery[] = [];
  const original = Object.getOwnPropertyDescriptor(window, 'devicePixelRatio');

  function setDpr(value: number): void {
    Object.defineProperty(window, 'devicePixelRatio', { value, configurable: true, writable: true });
  }
  function stubMatchMedia(): void {
    vi.stubGlobal('matchMedia', (media: string): FakeQuery => {
      const listeners = new Set<EventListener>();
      const q: FakeQuery = {
        media,
        listeners,
        addEventListener: (type, fn) => {
          if (type === 'change') listeners.add(fn);
        },
        removeEventListener: (type, fn) => {
          if (type === 'change') listeners.delete(fn);
        },
      };
      queries.push(q);
      return q;
    });
  }
  /** Fires `change` on every list that has listeners (a real list fires when its match flips). */
  function fireChange(): void {
    for (const q of [...queries]) for (const fn of [...q.listeners]) fn(new Event('change'));
  }
  /** Queries that still have a listener. */
  const armed = (): string[] => queries.filter((q) => q.listeners.size > 0).map((q) => `${q.media} ×${q.listeners.size}`);

  beforeEach(() => {
    queries = [];
  });
  afterEach(() => {
    if (original) Object.defineProperty(window, 'devicePixelRatio', original);
    else delete (window as { devicePixelRatio?: number }).devicePixelRatio;
  });

  it('re-arms on every change and calls setPixelRatio only when the capped value changes; destroy removes the listener', async () => {
    setDpr(1.5);
    stubMatchMedia();
    const s = setup();
    const { handle, map } = await mounted(s);
    expect(map.options.pixelRatio).toBe(1.5);
    expect(armed()).toEqual(['(resolution: 1.5dppx) ×1']);

    setDpr(2.25);
    fireChange();
    expect(map.pixelRatios).toEqual([2]);
    expect(armed()).toEqual(['(resolution: 2.25dppx) ×1']);

    setDpr(3);
    fireChange();
    expect(map.pixelRatios).toEqual([2]); // still capped at 2: no call
    expect(armed()).toEqual(['(resolution: 3dppx) ×1']);

    setDpr(1);
    fireChange();
    expect(map.pixelRatios).toEqual([2, 1]);
    expect(armed()).toEqual(['(resolution: 1dppx) ×1']);

    handle.destroy();
    expect(armed()).toEqual([]);
    expect(recorder.log.indexOf('map.remove')).toBeGreaterThan(-1);
    // A later change reaches nothing: dispatch to every list ever created.
    for (const q of queries) q.listeners.forEach((fn) => fn(new Event('change')));
    setDpr(2);
    fireChange();
    expect(map.pixelRatios).toEqual([2, 1]);
  });

  it('the listener is removed before map.remove() in teardown', async () => {
    setDpr(2);
    stubMatchMedia();
    const s = setup();
    const { handle } = await mounted(s);
    const q = queries[0];
    const remove = vi.spyOn(q, 'removeEventListener');
    let removedBeforeMap = false;
    const fakeMap = recorder.maps[0];
    const mapRemove = fakeMap.remove.bind(fakeMap);
    fakeMap.remove = () => {
      removedBeforeMap = remove.mock.calls.length === 1;
      mapRemove();
    };
    handle.destroy();
    expect(removedBeforeMap).toBe(true);
  });

  it('matchMedia undefined: the mount succeeds with the capped ratio', async () => {
    setDpr(3);
    vi.stubGlobal('matchMedia', undefined);
    const s = setup();
    const { map } = await mounted(s);
    expect(map.options.pixelRatio).toBe(2);
    expect(recorder.markers).toHaveLength(1);
  });

  it('partial mount (Marker.addTo throws): rejects and no listener is left', async () => {
    setDpr(1.5);
    stubMatchMedia();
    recorder.throwIn = 'Marker.addTo';
    const s = setup();
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await quietUntilMap(s);
    await expect(p).rejects.toThrow(/test/);
    expect(queries).toHaveLength(1);
    expect(armed()).toEqual([]);
  });

  it('refits the route on resize until the viewer moves the camera (F-13 D-20)', async () => {
    const s = setup();
    const { map } = await mounted(s);
    map.fire('resize');
    expect(map.fits).toHaveLength(1);
    expect(map.fits[0][1]).toEqual({ padding: 32, maxZoom: 16, animate: false });
    map.fire('movestart'); // programmatic: no originalEvent
    map.fire('resize');
    expect(map.fits).toHaveLength(2);
    map.fire('zoomstart', { originalEvent: new Event('wheel') });
    map.fire('resize');
    expect(map.fits).toHaveLength(2);
  });
});

describe('fit padding in two columns (F-13 D-18)', () => {
  it('60 px at the bottom from 1024 px wide, 32 px all round below', async () => {
    vi.spyOn(document.documentElement, 'clientWidth', 'get').mockReturnValue(1280);
    const s = setup();
    const { map } = await mounted(s);
    const pad = { top: 32, right: 32, bottom: 60, left: 32 };
    expect(map.options.fitBoundsOptions).toEqual({ padding: pad, maxZoom: 16 });
    map.fire('resize');
    expect(map.fits[0][1]).toEqual({ padding: pad, maxZoom: 16, animate: false });
  });
});

describe('phone attribution (F-13 D-20, QA-13-r2-4)', () => {
  /** A phone-width matchMedia whose only job is to match the max-width query. */
  async function phone(): Promise<{ map: FakeMap; attrib: HTMLElement }> {
    vi.stubGlobal('matchMedia', (media: string) => ({
      media,
      matches: media.includes('max-width'),
      addEventListener: () => {},
      removeEventListener: () => {},
    }));
    const { map } = await mounted(setup());
    const attrib = (map.options.container as HTMLElement).querySelector<HTMLElement>('.maplibregl-ctrl-attrib')!;
    return { map, attrib };
  }
  const open = (a: HTMLElement): boolean => a.classList.contains('maplibregl-compact-show');

  afterEach(() => vi.useRealTimers());

  it('stays open for 5 s after the text first appears, then collapses', async () => {
    vi.useFakeTimers();
    const { map, attrib } = await phone();
    map.fire('idle'); // no credits yet: the clock has not started
    vi.advanceTimersByTime(10000);
    expect(open(attrib)).toBe(true);
    attrib.textContent = '© OpenFreeMap';
    map.fire('idle');
    vi.advanceTimersByTime(4999);
    expect(open(attrib)).toBe(true);
    vi.advanceTimersByTime(1);
    expect(open(attrib)).toBe(false);
    expect(attrib.hasAttribute('open')).toBe(false);
  });

  it('collapses at the first interaction with the map, before the 5 s', async () => {
    vi.useFakeTimers();
    const { map, attrib } = await phone();
    attrib.textContent = '© OpenFreeMap';
    map.fire('idle');
    vi.advanceTimersByTime(1000);
    (map.options.container as HTMLElement).dispatchEvent(new Event('pointerdown', { bubbles: true }));
    expect(open(attrib)).toBe(false);
  });

  it('a tap on (i) is the viewer\'s choice: no later collapse', async () => {
    vi.useFakeTimers();
    const { map, attrib } = await phone();
    attrib.textContent = '© OpenFreeMap';
    map.fire('idle');
    attrib.dispatchEvent(new Event('pointerdown', { bubbles: true }));
    vi.advanceTimersByTime(20000);
    expect(open(attrib)).toBe(true);
  });

  it('(F-17) without a selection store the map never sets range data', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('matchMedia', (media: string) => ({ media, matches: false, addEventListener: () => {}, removeEventListener: () => {} }));
    const { map } = await mounted(setup());
    expect(map.sourceData).toEqual([]);
  });

  it('not on desktop: no timer, stays as MapLibre leaves it', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('matchMedia', (media: string) => ({ media, matches: false, addEventListener: () => {}, removeEventListener: () => {} }));
    const s = setup();
    const { map } = await mounted(s);
    const before = vi.getTimerCount();
    (map.options.container as HTMLElement).querySelector('.maplibregl-ctrl-attrib')!.textContent = '© OpenFreeMap';
    map.fire('idle');
    expect(vi.getTimerCount()).toBe(before);
  });
});

describe('range highlight (F-17 R-10, AC 3, AC 4)', () => {
  async function withRange(a = -1, b = -1) {
    const s = setup();
    const queue: (() => void)[] = [];
    const selection = createSelectionStore(s.data.sampleCount, (f) => queue.push(f));
    if (a >= 0) selection.set(a, b);
    const flushRange = () => queue.splice(0).forEach((f) => f());
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, selection, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await quietUntilMap(s);
    const handle = await p;
    handles.push(handle);
    const map = recorder.maps[recorder.maps.length - 1];
    return { s, selection, flushRange, map, handle };
  }
  const style = (map: FakeMap) => map.options.style as { sources: Record<string, { data: { features: { properties: { kind: string }; geometry: { coordinates: number[][][] } }[] } }>; layers: { id: string }[] };

  it('the base style has an empty wx-range source, its layers above the route and below the markers', async () => {
    const { map } = await withRange();
    const st = style(map);
    expect(st.sources[RANGE_SOURCE].data.features).toEqual([]);
    const ids = st.layers.map((l) => l.id);
    expect(ids.indexOf('wx-range-line')).toBeGreaterThan(ids.indexOf('wx-route-line'));
    expect(ids.indexOf('wx-range-line')).toBeLessThan(ids.indexOf('wx-lap-circle'));
  });

  it('a range at mount is in the base style; positions only, a dropout as a bridge', async () => {
    const { map } = await withRange(1, 6);
    const f = style(map).sources[RANGE_SOURCE].data.features;
    expect(f.map((x) => x.properties.kind)).toEqual(['recorded', 'bridge']);
    expect(f[0].geometry.coordinates).toEqual([[[8.01, 47], [8.02, 47]], [[8.05, 47], [8.06, 47]]]);
    expect(f[1].geometry.coordinates).toEqual([[[8.02, 47], [8.05, 47]]]);
  });

  it('one setData 150 ms after the range stops changing, never per frame; a clear empties it', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const { selection, flushRange, map } = await withRange();
    for (const b of [2, 5, 7]) {
      selection.set(0, b);
      flushRange();
      vi.advanceTimersByTime(RANGE_SETTLE_MS - 10);
    }
    expect(map.sourceData).toEqual([]);
    vi.advanceTimersByTime(10);
    expect(map.sourceData).toHaveLength(1);
    const [id, data] = map.sourceData[0] as [string, { features: { geometry: { coordinates: number[][][] } }[] }];
    expect(id).toBe(RANGE_SOURCE);
    const lons = data.features.flatMap((x) => x.geometry.coordinates.flat().map((c) => c[0]));
    expect(lons).not.toContain(Number.NaN);
    expect(new Set(lons)).toEqual(new Set([8.0, 8.01, 8.02, 8.05, 8.06, 8.07]));
    selection.clear();
    flushRange();
    vi.advanceTimersByTime(RANGE_SETTLE_MS);
    expect(map.sourceData.at(-1)).toEqual([RANGE_SOURCE, { type: 'FeatureCollection', features: [] }]);
  });

  it('a style swap re-applies the latest range on style.load; destroy cancels a pending update', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const { selection, flushRange, map, handle } = await withRange();
    selection.set(0, 2);
    flushRange();
    vi.advanceTimersByTime(RANGE_SETTLE_MS);
    map.fire('style.load');
    const range = () => map.sourceData.filter(([id]) => id === RANGE_SOURCE);
    expect(range()).toHaveLength(2);
    expect(range()[1]).toEqual(range()[0]);
    selection.set(0, 7);
    flushRange();
    handle.destroy();
    vi.advanceTimersByTime(RANGE_SETTLE_MS);
    expect(range()).toHaveLength(2);
  });

  it('the map never writes the selection (OQ-73): a tap on the route keeps it', async () => {
    const { selection, map } = await withRange(1, 2);
    const canvas = map.getCanvas();
    aim(6);
    pointer(canvas, 'pointerdown', { pointerType: 'touch', pointerId: 3 });
    pointer(canvas, 'pointerup', { pointerType: 'touch', pointerId: 3 });
    expect([selection.start(), selection.end()]).toEqual([1, 2]);
  });
});

describe('to-be-hidden marks (F-19 E-9, ADR-030)', () => {
  type Lines = { features: { properties: { kind: string }; geometry: { coordinates: number[][][] } }[] };
  it('setHidden draws the positioned samples of each range; [] empties; style.load re-applies; destroy stops', async () => {
    const s = setup();
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, signal: s.abort.signal });
    await quietUntilMap(s);
    const handle = await p;
    handles.push(handle);
    const map = recorder.maps[recorder.maps.length - 1];
    const base = (map.options.style as { sources: Record<string, { data: Lines }> }).sources[HIDDEN_SOURCE];
    expect(base.data.features).toEqual([]);
    const last = (): [string, Lines] => map.sourceData[map.sourceData.length - 1] as [string, Lines];

    handle.setHidden([[0, 1], [5, 7]]);
    expect(last()[0]).toBe(HIDDEN_SOURCE);
    expect(last()[1].features.map((f) => f.properties.kind)).toEqual(['recorded']);
    expect(last()[1].features[0].geometry.coordinates).toEqual([[[8.0, 47], [8.01, 47]], [[8.05, 47], [8.06, 47], [8.07, 47]]]);

    handle.setHidden([[0, 7]]); // the whole route: the dropout is a bridge
    expect(last()[1].features.map((f) => f.properties.kind)).toEqual(['recorded', 'bridge']);

    handle.setHidden([]);
    expect(last()).toEqual([HIDDEN_SOURCE, { type: 'FeatureCollection', features: [] }]);
    handle.setHidden([[0, 1]]);
    const count = map.sourceData.length;
    map.fire('style.load'); // re-applies the hidden marks (and the plain colouring, F-25 C-7)
    expect(map.sourceData.length).toBe(count + 2);
    expect(map.sourceData.find(([id], k) => k >= count && id === HIDDEN_SOURCE)).toEqual(map.sourceData[count - 1]);

    handle.destroy();
    handle.setHidden([[0, 7]]);
    expect(map.sourceData.length).toBe(count + 2);
  });
});

describe('route colouring control (F-25 C-6, C-7, C-8)', () => {
  type Colour = { features: { properties: { c: number }; geometry: { coordinates: number[][][] } }[] };
  /** LAT/LON with speed (5 → 10 m/s), heart rate and no power; samples 3–4 have no position. */
  function coloured(sport = 'cycling'): WorkoutData {
    const d = positioned(LAT, LON);
    return {
      ...d,
      metrics: { speed: Float64Array.from([5, 6, 7, 8, 8, 9, 10, 10]), heart_rate: Float64Array.from([100, 110, 120, N, N, 150, 160, 170]) },
      meta: { ...d.meta, sport },
    };
  }
  async function withControl(data: WorkoutData, units = createUnitsStore('metric')) {
    const s = setup(data);
    const p = mountRouteMap({ container: s.container, data: s.data, cursor: s.cursor, whenQuiet: s.whenQuiet, units, signal: s.abort.signal });
    await quietUntilMap(s);
    const handle = await p;
    handles.push(handle);
    const map = recorder.maps[recorder.maps.length - 1];
    const entry = map.controls.find(([, pos]) => pos === 'bottom-left');
    const box = entry ? (entry[0] as { onAdd(m: unknown): HTMLElement }).onAdd(map) : null;
    return { s, handle, map, box, units };
  }
  const pick = (box: HTMLElement, value: string): void => {
    const select = box.querySelector('select')!;
    select.value = value;
    select.dispatchEvent(new Event('change'));
  };

  it('offers plain route first, then only the metrics the workout has; nothing is classed before a choice', async () => {
    const { map, box } = await withControl(coloured());
    expect(Array.from(box!.querySelectorAll('option'), (o) => [o.value, o.textContent])).toEqual([
      ['', 'Plain route'],
      ['speed', 'Speed'],
      ['heart_rate', 'Heart rate'],
    ]);
    expect(box!.querySelector<HTMLElement>('.wx-legend')!.hidden).toBe(true);
    expect(map.sourceData.filter(([id]) => id === COLOUR_SOURCE)).toEqual([]);
  });

  it('no offered metric: no control', async () => {
    const { box } = await withControl(positioned(LAT, LON));
    expect(box).toBeNull();
  });

  it('a choice sets wx-colour once, recorded edges only; the legend follows the units; plain empties it', async () => {
    const { map, box, units } = await withControl(coloured());
    pick(box!, 'speed');
    const writes = map.sourceData.filter(([id]) => id === COLOUR_SOURCE);
    expect(writes).toHaveLength(1);
    const data = writes[0][1] as Colour;
    const edges = data.features.flatMap((f) => f.geometry.coordinates).reduce((n, line) => n + line.length - 1, 0);
    expect(edges).toBe(4); // 0–1, 1–2, 5–6, 6–7: never the dropout bridge 2–5
    const legendBox = box!.querySelector<HTMLElement>('.wx-legend')!;
    expect(legendBox.hidden).toBe(false);
    expect(legendBox.querySelectorAll('.wx-swatches span')).toHaveLength(7);
    expect(legendBox.querySelector('.wx-l7')!.textContent).toMatch(/ km\/h$/);
    units.set('imperial');
    expect(legendBox.querySelector('.wx-l7')!.textContent).toMatch(/ mph$/);
    expect(map.sourceData.filter(([id]) => id === COLOUR_SOURCE)).toHaveLength(1); // a units switch only relabels
    pick(box!, '');
    expect(map.sourceData[map.sourceData.length - 1]).toEqual([COLOUR_SOURCE, { type: 'FeatureCollection', features: [] }]);
    expect(legendBox.hidden).toBe(true);
  });

  it('a run labels speed as pace, with a pace legend', async () => {
    const { box } = await withControl(coloured('running'));
    expect(box!.querySelector('option[value="speed"]')!.textContent).toBe('Pace');
    pick(box!, 'speed');
    expect(box!.querySelector('.wx-l7')!.textContent).toMatch(/^\d+:\d\d \/km$/);
  });

  it('the cursor never touches wx-colour; style.load re-applies the latest colouring; the scheme rebuilds the class colours', async () => {
    const { s, handle, map, box } = await withControl(coloured());
    pick(box!, 'heart_rate');
    const latest = map.sourceData[map.sourceData.length - 1];
    const count = map.sourceData.length;
    s.cursor.set(1);
    s.flush();
    expect(map.sourceData).toHaveLength(count);
    map.fire('style.load');
    expect(map.sourceData.filter(([id], k) => k >= count && id === COLOUR_SOURCE)).toEqual([latest]);
    handle.setScheme('dark');
    expect(map.paints.some(([id, prop]) => id === 'wx-colour-line' && prop === 'line-color')).toBe(true);
  });
});
