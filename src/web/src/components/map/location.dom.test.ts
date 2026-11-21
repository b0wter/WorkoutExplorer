// @vitest-environment jsdom
// F-62 "Show my location" through the lazy map (LC-1 to LC-10): buttons, the watch, dot and circle,
// follow mode, lifecycle, errors and the position's lack of exits. MapLibre is the recording fake;
// the Geolocation API a fake per test. Synthetic positions only.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createCursorStore } from '../../lib/cursor';
import { positioned } from '../../lib/route/testing/fixtures';
import { mountRouteMap, type RouteMapHandle } from './map-view';
import { recorder, resetRecorder, type FakeMap, type FakeMarker } from './testing/maplibre-fake';

vi.mock('maplibre-gl', async () => await import('./testing/maplibre-fake'));
vi.mock('maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url', () => ({ default: '/assets/maplibre-gl-worker-test.js' }));
vi.mock('maplibre-gl/dist/maplibre-gl.css', () => ({}));

const LON = [8.0, 8.01, 8.02];
const LAT = [47.0, 47.0, 47.0];

type Ok = (p: { coords: Partial<GeolocationCoordinates> }) => void;
type Err = (e: { code: number }) => void;

let geo: { watchPosition: ReturnType<typeof vi.fn>; clearWatch: ReturnType<typeof vi.fn>; getCurrentPosition: ReturnType<typeof vi.fn> };
let ok: Ok;
let err: Err;
let fetchMock: ReturnType<typeof vi.fn>;
let handle: RouteMapHandle | null;
let container: HTMLDivElement;
let map: FakeMap;
let spies: ReturnType<typeof vi.spyOn>[];

const tick = (): Promise<void> => new Promise((r) => setTimeout(r, 0));
const fix = (lng: number, lat: number, accuracy = 20): void => ok({ coords: { longitude: lng, latitude: lat, accuracy } });
const toggle = (): HTMLButtonElement => container.querySelector('.wx-loc-group')!.children[0] as HTMLButtonElement;
const follow = (): HTMLButtonElement => container.querySelector('.wx-loc-group')!.children[1] as HTMLButtonElement;
const locMarkers = (): FakeMarker[] => recorder.markers.filter((m) => /wx-loc/.test(m.options.element.className));
const shownMarkers = (): string[] => locMarkers().filter((m) => m.options.element.isConnected).map((m) => m.options.element.className);
const setVisibility = (v: 'hidden' | 'visible'): void => {
  Object.defineProperty(document, 'visibilityState', { value: v, configurable: true });
  document.dispatchEvent(new Event('visibilitychange'));
};

async function mount({ locate }: { locate?: boolean } = { locate: true }): Promise<void> {
  const data = positioned(LAT, LON);
  container = document.createElement('div');
  document.body.appendChild(container);
  handle = await mountRouteMap({
    container,
    data,
    cursor: createCursorStore(data.sampleCount, (f) => f()),
    whenQuiet: () => Promise.resolve(),
    locate,
    signal: new AbortController().signal,
  });
  map = recorder.maps[recorder.maps.length - 1];
  map.zoom = 12;
}

/** The first tap: the chunk loads, then the tap runs. */
async function turnOn(): Promise<void> {
  toggle().click();
  await vi.waitFor(() => expect(toggle().getAttribute('aria-pressed')).toBe('true'));
}

beforeEach(() => {
  resetRecorder();
  let id = 0;
  geo = {
    watchPosition: vi.fn((a: Ok, b: Err) => {
      ok = a;
      err = b;
      return ++id;
    }),
    clearWatch: vi.fn(),
    getCurrentPosition: vi.fn(),
  };
  Object.defineProperty(navigator, 'geolocation', { value: geo, configurable: true });
  Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true });
  Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
  fetchMock = vi.fn(() => new Promise(() => {})); // the style request; never answers
  vi.stubGlobal('fetch', fetchMock);
  spies = [
    ...(['log', 'info', 'warn', 'error', 'debug'] as const).map((k) => vi.spyOn(console, k)),
    vi.spyOn(Storage.prototype, 'setItem'),
    vi.spyOn(history, 'pushState'),
    vi.spyOn(history, 'replaceState'),
  ];
});

afterEach(() => {
  handle?.destroy();
  handle = null;
  container.remove();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('F-62 location controls (LC-1, LC-2)', () => {
  it('without the viewing-page flag (edit page, bench): no location control', async () => {
    await mount({});
    expect(container.querySelector('.wx-loc-group')).toBeNull();
  });

  it('viewing page: a top-right group with the toggle, the follow button hidden; nothing asks for the position before the tap (AC 41, 42)', async () => {
    await mount();
    const group = container.querySelector('.wx-loc-group')!;
    expect(group.classList).toContain('maplibregl-ctrl-group');
    expect(map.controls.at(-1)![1]).toBe('top-right');
    expect([toggle().title, toggle().getAttribute('aria-label'), toggle().getAttribute('aria-pressed')]).toEqual(['Show my location', 'Show my location', 'false']);
    expect([follow().title, follow().hidden]).toEqual(['Follow my location', true]);
    await tick();
    expect(geo.watchPosition).not.toHaveBeenCalled();
    expect(geo.getCurrentPosition).not.toHaveBeenCalled();
  });
});

describe('F-62 position, follow mode and turning off (LC-3 to LC-6)', () => {
  it('tap: one high-accuracy watch; the first fix draws dot and circle, centres and follows (AC 43)', async () => {
    await mount();
    await turnOn();
    expect(geo.watchPosition).toHaveBeenCalledTimes(1);
    expect(geo.watchPosition.mock.calls[0][2]).toMatchObject({ enableHighAccuracy: true });
    expect(follow().hidden).toBe(true); // no fix yet
    expect(map.jumps).toHaveLength(0);
    fix(8.5, 47.5, 30);
    expect(shownMarkers()).toEqual(['wx-loc-ring', 'wx-loc-dot']); // circle below the dot
    for (const m of locMarkers()) expect(m.positions.at(-1)).toEqual([8.5, 47.5]);
    expect(map.jumps).toEqual([{ center: { lng: 8.5, lat: 47.5 }, zoom: 16 }]);
    expect([follow().hidden, follow().getAttribute('aria-pressed')]).toEqual([false, 'true']);
    // 2 · 30 m at zoom 16 and 47.5°: 2 · 30 · 2^16 / (40075016.686 / 512 · cos 47.5°)
    const ring = locMarkers()[0].options.element;
    const want = (60 * 2 ** 16) / ((40075016.686 / 512) * Math.cos((47.5 * Math.PI) / 180));
    expect(parseFloat(ring.style.width)).toBeCloseTo(want, 3);
    // while following, the map stays centred on the moving position
    fix(8.6, 47.6);
    expect(map.jumps.at(-1)).toEqual({ center: { lng: 8.6, lat: 47.6 }, zoom: 16 });
  });

  it('the circle follows the zoom', async () => {
    await mount();
    await turnOn();
    fix(8, 47, 50);
    const ring = locMarkers()[0].options.element;
    const w = parseFloat(ring.style.width);
    map.zoom = 17;
    map.fire('zoom');
    expect(parseFloat(ring.style.width)).toBeCloseTo(w * 2, 3);
  });

  it('a viewer move stops follow mode, the dot keeps updating; our own moves and a resize do not (AC 44)', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    map.fire('movestart'); // our jumpTo, a resize: no originalEvent
    map.fire('resize');
    fix(8.1, 47.1);
    expect(map.jumps).toHaveLength(2);
    map.fire('zoomstart', { originalEvent: new WheelEvent('wheel') });
    expect(follow().getAttribute('aria-pressed')).toBe('false');
    fix(8.2, 47.2);
    expect(map.jumps).toHaveLength(2);
    expect(locMarkers()[1].positions.at(-1)).toEqual([8.2, 47.2]);
    map.fire('movestart', { originalEvent: new MouseEvent('mousedown') });
    fix(8.3, 47.3);
    expect(map.jumps).toHaveLength(2);
  });

  it('follow re-centres and resumes follow mode, every time it is tapped (AC 45)', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    map.fire('movestart', { originalEvent: new MouseEvent('mousedown') });
    map.zoom = 18;
    follow().click();
    await tick();
    expect(map.jumps.at(-1)).toEqual({ center: { lng: 8, lat: 47 }, zoom: 18 }); // the viewer's closer zoom stays
    expect(follow().getAttribute('aria-pressed')).toBe('true');
    follow().click();
    await tick();
    expect(map.jumps).toHaveLength(3);
    fix(8.1, 47.1);
    expect(map.jumps).toHaveLength(4);
  });

  it('a second tap stops the watch, removes dot, circle and follow button, leaves the map (AC 46)', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    toggle().click();
    await tick();
    expect(geo.clearWatch).toHaveBeenCalledWith(1);
    expect(shownMarkers()).toEqual([]);
    expect([toggle().getAttribute('aria-pressed'), follow().hidden]).toEqual(['false', true]);
    expect(map.jumps).toHaveLength(1);
    ok({ coords: { longitude: 9, latitude: 48, accuracy: 5 } }); // a late callback
    expect(shownMarkers()).toEqual([]);
    expect(map.jumps).toHaveLength(1);
    // on again: a new watch, centred again
    toggle().click();
    await tick();
    fix(8, 47);
    expect(geo.watchPosition).toHaveBeenCalledTimes(2);
    expect(map.jumps).toHaveLength(2);
  });

  it('the first centring ends the refit, so a resize never fits the route over the position (LC-5)', async () => {
    await mount();
    map.fire('resize');
    expect(map.fits).toHaveLength(1); // D-20 still refits before
    await turnOn();
    fix(8, 47);
    map.fire('resize');
    expect(map.fits).toHaveLength(1);
  });

  it('two quick taps while the chunk loads: off again, at most one watch (LC-7)', async () => {
    await mount();
    toggle().click();
    toggle().click();
    await vi.waitFor(() => expect(geo.watchPosition).toHaveBeenCalledTimes(1));
    await tick();
    expect(geo.clearWatch).toHaveBeenCalledWith(1);
    expect(toggle().getAttribute('aria-pressed')).toBe('false');
  });
});

describe('F-62 lifecycle (LC-9, AC 48)', () => {
  it('hidden: the watch is cleared, dot and follow mode kept; visible: a new watch without a tap', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    setVisibility('hidden');
    expect(geo.clearWatch).toHaveBeenCalledWith(1);
    expect(shownMarkers()).toHaveLength(2);
    setVisibility('visible');
    expect(geo.watchPosition).toHaveBeenCalledTimes(2);
    fix(8.1, 47.1);
    expect(map.jumps).toHaveLength(2); // still following
  });

  it('hidden while off: no watch on return', async () => {
    await mount();
    await turnOn();
    toggle().click();
    await tick();
    setVisibility('hidden');
    setVisibility('visible');
    expect(geo.watchPosition).toHaveBeenCalledTimes(1);
  });

  it('pagehide: fully off', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    window.dispatchEvent(new Event('pagehide'));
    expect(geo.clearWatch).toHaveBeenCalledWith(1);
    expect(shownMarkers()).toEqual([]);
    expect(toggle().getAttribute('aria-pressed')).toBe('false');
  });

  it('map teardown: the watch is cleared and no listener is left', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    handle!.destroy();
    expect(geo.clearWatch).toHaveBeenCalledWith(1);
    window.dispatchEvent(new Event('pagehide'));
    setVisibility('hidden');
    setVisibility('visible');
    expect(geo.watchPosition).toHaveBeenCalledTimes(1);
  });
});

describe('F-62 errors (LC-8, AC 49)', () => {
  const note = (): HTMLElement | null => container.querySelector('.wx-loc-note');

  it('denied before the first fix: one message (role status, our text), off, map unchanged', async () => {
    await mount();
    await turnOn();
    err({ code: 1 });
    expect([note()?.textContent, note()?.getAttribute('role')]).toEqual(['Location access is not allowed.', 'status']);
    expect(geo.clearWatch).toHaveBeenCalledWith(1);
    expect(toggle().getAttribute('aria-pressed')).toBe('false');
    expect([map.jumps.length, shownMarkers().length]).toEqual([0, 0]);
  });

  for (const code of [2, 3])
    it(`error ${code} before the first fix: off with the message`, async () => {
      await mount();
      await turnOn();
      err({ code });
      expect(note()?.textContent).toBe('Your location is not available.');
      expect(toggle().getAttribute('aria-pressed')).toBe('false');
    });

  it('after the first fix: unavailable keeps the dot and the watch, the message once; a denial turns off', async () => {
    await mount();
    await turnOn();
    fix(8, 47);
    err({ code: 3 });
    expect(note()?.textContent).toBe('Your location is not available.');
    note()!.remove();
    err({ code: 2 });
    expect(note()).toBeNull();
    expect(shownMarkers()).toHaveLength(2);
    expect(geo.clearWatch).not.toHaveBeenCalled();
    err({ code: 1 });
    expect(note()?.textContent).toBe('Location access is not allowed.');
    expect(shownMarkers()).toEqual([]);
  });

  it('the message clears itself', async () => {
    vi.useFakeTimers();
    try {
      await mount();
      toggle().click();
      await vi.waitFor(() => expect(geo.watchPosition).toHaveBeenCalled());
      err({ code: 2 });
      expect(note()).not.toBeNull();
      vi.advanceTimersByTime(5000);
      expect(note()).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('insecure context or no API: the message, no watch, off', async () => {
    Object.defineProperty(window, 'isSecureContext', { value: false, configurable: true });
    await mount();
    toggle().click();
    await vi.waitFor(() => expect(note()?.textContent).toBe('Your location is not available.'));
    expect(geo.watchPosition).not.toHaveBeenCalled();
    expect(toggle().getAttribute('aria-pressed')).toBe('false');
  });
});

describe('F-62 the position has no exit (LC-10, AC 50)', () => {
  it('with location on and moving: no request but the style, no storage, cookie, history or URL change, no console output', async () => {
    const href = location.href;
    await mount();
    await turnOn();
    for (let k = 0; k < 5; k++) fix(8 + k / 100, 47 + k / 100);
    map.fire('movestart', { originalEvent: new MouseEvent('mousedown') });
    follow().click();
    setVisibility('hidden');
    setVisibility('visible');
    err({ code: 3 });
    toggle().click();
    await tick();
    expect(fetchMock.mock.calls.map((c) => String(c[0]))).toEqual([expect.stringMatching(/^https:\/\/tiles\.openfreemap\.org\//)]);
    for (const s of spies) expect(s).not.toHaveBeenCalled();
    expect(document.cookie).toBe('');
    expect(location.href).toBe(href);
  });
});
