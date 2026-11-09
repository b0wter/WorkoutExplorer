// @vitest-environment jsdom
// The preview basemap (F-36 MC-7; AC 2, 37) with the recording MapLibre fake (ADR-012 rev. 3):
// liberty only (no GeoJSON, no wx- source or layer) whatever the theme, exactly the server's
// camera, 1200 × 630 at pixel ratio 1, and the map removed on every path. Synthetic frame only.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { recorder, resetRecorder, type FakeMap } from './testing/maplibre-fake';
import { OPENFREEMAP_STYLE_URL } from './map-style';
import { renderBasemap } from './basemap';

vi.mock('maplibre-gl', async () => await import('./testing/maplibre-fake'));

const FRAME = { lon: 11.5, lat: 48.1, zoom: 12.25 };
const LIBERTY = {
  version: 8,
  sources: { openmaptiles: { type: 'vector', url: 'https://tiles.openfreemap.org/planet' } },
  layers: [{ id: 'background', type: 'background' }],
  terrain: { source: 'openmaptiles' },
};
const PNG = new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });
let fetches: [string, RequestInit][];

beforeEach(() => {
  resetRecorder();
  fetches = [];
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit) => (fetches.push([url, init]), Response.json(LIBERTY))));
  document.documentElement.dataset.theme = 'dark';
});
afterEach(() => {
  vi.unstubAllGlobals();
  delete document.documentElement.dataset.theme;
  document.body.replaceChildren();
});

/** Waits until the fake map exists. */
async function mapped(): Promise<FakeMap> {
  for (let k = 0; k < 20 && recorder.maps.length === 0; k++) await new Promise((r) => setTimeout(r, 0));
  return recorder.maps[0];
}
const blobOn = (m: FakeMap, b: Blob | null) => ((m.canvas as { toBlob: unknown }).toBlob = (cb: (b: Blob | null) => void, type: string) => cb(type === 'image/png' ? b : null));

describe('renderBasemap', () => {
  it('liberty alone at the server frame, offscreen 1200 × 630, PNG after the first idle', async () => {
    const signal = new AbortController().signal;
    const out = renderBasemap(FRAME, signal);
    const m = await mapped();
    expect(fetches).toEqual([[OPENFREEMAP_STYLE_URL, { credentials: 'omit', referrerPolicy: 'no-referrer', signal }]]);
    const o = m.options as Record<string, unknown> & { style: { sources: object; layers: { id: string }[]; terrain?: unknown; projection: unknown } };
    expect(o).toMatchObject({ center: [11.5, 48.1], zoom: 12.25, bearing: 0, pitch: 0, interactive: false, attributionControl: false, fadeDuration: 0, pixelRatio: 1 });
    expect(o.canvasContextAttributes).toEqual({ preserveDrawingBuffer: true });
    expect(o.style.sources).toEqual(LIBERTY.sources); // no GeoJSON, no wx- source
    expect(o.style.layers.map((l) => l.id)).toEqual(['background']);
    expect(o.style.terrain).toBeUndefined();
    expect(o.style.projection).toEqual({ type: 'mercator' });
    expect((o.container as HTMLElement).className).toBe('wx-basemap');
    expect((o.container as HTMLElement).getAttribute('style')).toBeNull(); // CSP: a class, no inline style
    expect(m.controls).toEqual([]);
    blobOn(m, PNG);
    m.fire('idle');
    expect(await out).toBe(PNG);
    expect(m.removed).toBe(true);
    expect(document.querySelector('.wx-basemap')).toBeNull();
  });

  it.each([
    ['a map error (tile, style, glyph, sprite)', (m: FakeMap) => m.fire('error', { error: new Error('https://tiles.openfreemap.org/x') })],
    ['toBlob gives nothing', (m: FakeMap) => (blobOn(m, null), m.fire('idle'))],
    [
      'a tainted canvas',
      (m: FakeMap) => {
        (m.canvas as { toBlob: unknown }).toBlob = () => {
          throw new DOMException('tainted', 'SecurityError');
        };
        m.fire('idle');
      },
    ],
  ])('%s rejects and removes the map', async (_name, act) => {
    const out = renderBasemap(FRAME, new AbortController().signal);
    const m = await mapped();
    act(m);
    await expect(out).rejects.toBeDefined();
    expect(m.removed).toBe(true);
    expect(document.querySelector('.wx-basemap')).toBeNull();
  });

  it('an abort rejects and removes the map', async () => {
    const c = new AbortController();
    const out = renderBasemap(FRAME, c.signal);
    const m = await mapped();
    c.abort();
    await expect(out).rejects.toBeDefined();
    expect(m.removed).toBe(true);
  });

  it('no WebGL2 (the constructor throws) rejects; nothing left behind', async () => {
    recorder.constructorError = new Error('GPUInitializationError (test)');
    await expect(renderBasemap(FRAME, new AbortController().signal)).rejects.toThrow('GPUInitializationError');
    expect(document.querySelector('.wx-basemap')).toBeNull();
  });

  it('a failed or invalid style rejects before any map', async () => {
    for (const r of [() => new Response('', { status: 503 }), () => Response.json({ version: 7 })]) {
      vi.stubGlobal('fetch', vi.fn(async () => r()));
      await expect(renderBasemap(FRAME, new AbortController().signal)).rejects.toThrow();
    }
    expect(recorder.maps).toEqual([]);
  });
});

describe('module boundary (MC-7, ADR-013 note; AC 45)', () => {
  const sources = import.meta.glob<string>(['/src/**/*.ts', '/src/**/*.svelte', '!/src/**/*.test.ts'], { query: '?raw', import: 'default', eager: true });

  it('only src/edit/basemap-job.ts reaches basemap, and only through import()', () => {
    const users = Object.entries(sources).filter(([, text]) => /map\/basemap['"]/.test(text));
    expect(users.map(([f]) => f)).toEqual(['/src/edit/basemap-job.ts']);
    expect(users[0][1]).not.toMatch(/^import [^t].*map\/basemap['"]/m); // `import type` only, statically
  });

  it('only map-view.ts and basemap.ts import maplibre-gl as a value', () => {
    const value = Object.entries(sources).filter(([, text]) => /^import (?!type)[^;]*from 'maplibre-gl';/m.test(text));
    expect(value.map(([f]) => f).sort()).toEqual(['/src/components/map/basemap.ts', '/src/components/map/map-view.ts']);
  });
});
