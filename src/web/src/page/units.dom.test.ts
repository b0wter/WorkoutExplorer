// @vitest-environment jsdom
// Units on the activity page (F-14 design D-3 to D-8, D-10; ADR-027): the default from the browser
// language, the in-place text rewrite, the drop-down, the scheme listener. ChartStack and RouteMap are
// the test stubs (./testing); fetch returns the synthetic golden file the sample was built from.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile } from '../lib/datafile';
import { createUnitsStore } from '../lib/units';
import { enhanceUnits, addThemeSelect, addUnitsSelect } from './enhance';
import { startPage } from './start';
import { renderSample, type Fixture } from './testing/fixture';
import { resetStubLog, stubLog } from './testing/stub-log';

vi.mock('../components/charts/ChartStack.svelte', async () => await import('./testing/ChartStackStub.svelte'));
vi.mock('../components/map/RouteMap.svelte', async () => await import('./testing/RouteMapStub.svelte'));

const bin = (name: string): ArrayBuffer => {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error('no golden ' + name);
  return g.bin;
};
const meta = (name: string) => {
  const r = decodeDataFile(bin(name));
  if (!r.ok) throw new Error('decode');
  return r.data.meta;
};

let page: Fixture | null = null;
let fetchMock: ReturnType<typeof vi.fn>;
let matches = false;
const mqListeners: ((e: { matches: boolean }) => void)[] = [];

function setLanguages(languages: string[]): void {
  vi.spyOn(window.navigator, 'languages', 'get').mockReturnValue(languages);
}

beforeEach(() => {
  resetStubLog();
  matches = false;
  mqListeners.length = 0;
  fetchMock = vi.fn(async () => new Response(bin('ride-30min').slice(0), { status: 200 }));
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('requestAnimationFrame', () => 1);
  vi.stubGlobal('matchMedia', () => ({
    get matches() {
      return matches;
    },
    addEventListener: (_: string, f: (e: { matches: boolean }) => void) => mqListeners.push(f),
  }));
  vi.spyOn(console, 'warn').mockImplementation(() => {});
});

afterEach(() => {
  page?.remove();
  page = null;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

async function open(sample = 'ride-30min'): Promise<void> {
  window.history.replaceState(null, '', '/w/x');
  page = renderSample(sample);
  await startPage(document);
  for (let k = 0; k < 10; k++) {
    await new Promise((r) => setTimeout(r, 0));
    flushSync();
  }
}

const metric = (key: string): string => document.querySelector(`.metric[data-k="${key}"] .v`)?.textContent ?? '';
const select = (): HTMLSelectElement | null => document.querySelector('#units-slot select');

describe('default and switch (AC 1, 2, 3, 9)', () => {
  it('en-US opens in miles, feet, mph and degrees F; the drop-down shows Imperial', async () => {
    setLanguages(['en-US']);
    await open();
    expect(metric('distance')).toBe('8.36 mi');
    expect(metric('ascent')).toMatch(/ft$/);
    expect(metric('speed')).toMatch(/mph/);
    expect(metric('temperature')).toMatch(/°F/);
    expect(select()?.value).toBe('imperial');
    expect([...document.querySelectorAll('.laps th')].map((th) => th.textContent)).toContain('Distance (mi)');
    expect(stubLog.chartStack[0].units).toBe('imperial');
  });

  it.each(['de-DE', 'en-GB', 'en'])('%s opens in kilometres and degrees C', async (lang) => {
    setLanguages([lang]);
    await open();
    expect(metric('distance')).toBe('13.46 km');
    expect(metric('temperature')).toMatch(/°C/);
    expect(select()?.value).toBe('metric');
    expect(stubLog.chartStack[0].units).toBe('metric');
  });

  it('switching updates summary, laps and the chart props at once; no request or cookie, one storage entry', async () => {
    setLanguages(['de-DE']);
    await open();
    const nodes = [...document.querySelectorAll('.summary *, .laps *')];
    const sel = select();
    if (!sel) throw new Error('no select');
    sel.value = 'imperial';
    sel.dispatchEvent(new Event('change'));
    flushSync();
    expect(metric('distance')).toBe('8.36 mi');
    expect(stubLog.chartStack[0].units).toBe('imperial');
    expect([...document.querySelectorAll('.summary *, .laps *')]).toEqual(nodes);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(document.cookie).toBe('');
    expect(localStorage.length).toBe(1);
    expect(localStorage.getItem('wx.units')).toBe('imperial');
    expect(sessionStorage.length).toBe(0);
    expect(JSON.stringify(fetchMock.mock.calls)).not.toContain('imperial');
    sel.value = 'metric';
    sel.dispatchEvent(new Event('change'));
    expect(metric('distance')).toBe('13.46 km');
  });

  it('a failed data file: metric text, no drop-down', async () => {
    setLanguages(['en-US']);
    fetchMock.mockImplementation(async () => new Response('x', { status: 404 }));
    await open();
    expect(select()).toBeNull();
    expect(metric('distance')).toBe('13.46 km');
  });
});

describe('remembered on the device (DS-1, AP-5; decision log 29 Oct 2026)', () => {
  it('without a stored choice: the language default, and nothing is written by opening', async () => {
    setLanguages(['en-US']);
    await open();
    expect(select()?.value).toBe('imperial');
    expect(localStorage.length).toBe(0);
  });

  it.each([
    ['de-DE', 'imperial', '8.36 mi'],
    ['en-US', 'metric', '13.46 km'],
  ])('%s with a stored %s: the stored choice wins', async (lang, stored, distance) => {
    setLanguages([lang]);
    localStorage.setItem('wx.units', stored);
    await open();
    expect(metric('distance')).toBe(distance);
    expect(select()?.value).toBe(stored);
    expect(stubLog.chartStack[0].units).toBe(stored);
  });

  it('a stored unknown value is ignored', async () => {
    setLanguages(['de-DE']);
    localStorage.setItem('wx.units', 'furlongs');
    localStorage.setItem('wx.axis', 'pace');
    await open();
    expect(select()?.value).toBe('metric');
    expect(stubLog.chartStack[0].initialAxis).toBe('time');
  });

  it('throwing storage: the default, no message, and the drop-down still switches', async () => {
    setLanguages(['de-DE']);
    const error = vi.spyOn(console, 'error').mockImplementation(() => {});
    vi.spyOn(window, 'localStorage', 'get').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError');
    });
    await open();
    expect(select()?.value).toBe('metric');
    const sel = select()!;
    sel.value = 'imperial';
    sel.dispatchEvent(new Event('change'));
    expect(metric('distance')).toBe('8.36 mi');
    stubLog.chartStack[0].onaxis?.('distance');
    expect(error).not.toHaveBeenCalled();
    expect(console.warn).not.toHaveBeenCalled();
  });

  it('the axis: the stored one is passed in, a toggle choice is stored', async () => {
    setLanguages(['de-DE']);
    localStorage.setItem('wx.axis', 'distance');
    await open();
    expect(stubLog.chartStack[0].initialAxis).toBe('distance');
    stubLog.chartStack[0].onaxis?.('time');
    expect(localStorage.getItem('wx.axis')).toBe('time');
    expect(localStorage.length).toBe(1);
  });
});

describe('axis and split (F-17 AC 12, 14, 15; QA-17-r1-2, QA-17-r1-4)', () => {
  it('the stored split is set before the data load resolves', async () => {
    setLanguages(['de-DE']);
    localStorage.setItem('wx.split.wide', '35');
    let resolve: (r: Response) => void = () => {};
    fetchMock.mockImplementation(() => new Promise<Response>((r) => (resolve = r)));
    window.history.replaceState(null, '', '/w/x');
    page = renderSample('ride-30min');
    const started = startPage(document);
    const layout = document.querySelector<HTMLElement>('.layout');
    expect(layout?.style.getPropertyValue('--map-w')).toBe('35.00%');
    expect(document.querySelector('.splitter')).toBeNull();
    resolve(new Response(bin('ride-30min').slice(0), { status: 200 }));
    await started;
    expect(document.querySelector('#map-slot .splitter')).not.toBeNull();
    expect(layout?.style.getPropertyValue('--map-w')).toBe('35.00%');
  });

  it('no cookie and no request carries the axis or the split', async () => {
    setLanguages(['de-DE']);
    HTMLElement.prototype.setPointerCapture = () => {};
    await open();
    stubLog.chartStack[0].onaxis?.('distance');
    const handle = document.querySelector<HTMLElement>('#map-slot .splitter');
    if (!handle) throw new Error('no splitter');
    const at = (type: string, clientX: number): boolean =>
      handle.dispatchEvent(new PointerEvent(type, { bubbles: true, cancelable: true, clientX, pointerId: 1, button: 0 }));
    at('pointerdown', 0);
    at('pointermove', 1000);
    at('pointerup', 1000);
    expect(localStorage.getItem('wx.axis')).toBe('distance');
    expect(localStorage.getItem('wx.split.wide')).not.toBeNull();
    expect(document.cookie).toBe('');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const sent = JSON.stringify(fetchMock.mock.calls);
    expect(sent).not.toContain('distance');
    expect(sent).not.toContain(localStorage.getItem('wx.split.wide'));
  });

  it('an indoor workout in between leaves the stored axis unchanged', async () => {
    setLanguages(['de-DE']);
    localStorage.setItem('wx.axis', 'distance');
    fetchMock.mockImplementation(async () => new Response(bin('indoor-no-distance-no-gps').slice(0), { status: 200 }));
    await open('indoor-unknown-zone');
    expect(stubLog.chartStack[0].initialAxis).toBe('distance');
    expect(localStorage.getItem('wx.axis')).toBe('distance');
    expect(localStorage.length).toBe(1);
    page?.remove();
    resetStubLog();
    fetchMock.mockImplementation(async () => new Response(bin('ride-30min').slice(0), { status: 200 }));
    await open();
    expect(stubLog.chartStack[0].initialAxis).toBe('distance');
  });
});

describe('colour scheme (AC 7, D-10)', () => {
  it('follows prefers-color-scheme live and passes it to charts', async () => {
    setLanguages(['de-DE']);
    matches = true;
    await open();
    expect(stubLog.chartStack[0].scheme).toBe('dark');
    for (const f of mqListeners) f({ matches: false });
    flushSync();
    expect(stubLog.chartStack[0].scheme).toBe('light');
  });
});

describe('theme drop-down (DS-4, user 3 Nov 2026)', () => {
  const theme = (): HTMLSelectElement => document.querySelector('#theme-slot select')!;
  const choose = (v: string): void => {
    theme().value = v;
    theme().dispatchEvent(new Event('change'));
    flushSync();
  };
  afterEach(() => delete document.documentElement.dataset.theme);

  it('System by default; a forced theme wins over the system and is remembered; System forgets it', async () => {
    setLanguages(['de-DE']);
    await open();
    expect([...theme().options].map((o) => [o.value, o.text])).toEqual([['system', 'System'], ['light', 'Light'], ['dark', 'Dark']]);
    expect(theme().value).toBe('system');
    expect(stubLog.chartStack[0].scheme).toBe('light');
    choose('dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('wx.theme')).toBe('dark');
    expect(stubLog.chartStack[0].scheme).toBe('dark');
    for (const f of mqListeners) f({ matches: false }); // the system says light: ignored
    flushSync();
    expect(stubLog.chartStack[0].scheme).toBe('dark');
    matches = true;
    for (const f of mqListeners) f({ matches: true });
    choose('light');
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(stubLog.chartStack[0].scheme).toBe('light');
    choose('system');
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
    expect(localStorage.getItem('wx.theme')).toBeNull();
    expect(stubLog.chartStack[0].scheme).toBe('dark');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('opens with the remembered choice', async () => {
    localStorage.setItem('wx.theme', 'dark');
    await open();
    expect(theme().value).toBe('dark');
    expect(stubLog.chartStack[0].scheme).toBe('dark');
  });

  it('is there even when the data file fails to load', async () => {
    fetchMock.mockImplementation(async () => new Response('', { status: 500 }));
    await open();
    expect(theme().value).toBe('system');
    expect(select()).toBeNull();
  });

  it('fills the empty slot once; without the slot nothing happens', () => {
    page = renderSample('ride-30min');
    expect(addThemeSelect(document.body, 'light', () => {})?.value).toBe('light');
    expect(addThemeSelect(document.body, 'light', () => {})).toBeNull();
    document.getElementById('theme-slot')?.remove();
    expect(addThemeSelect(document.body, 'light', () => {})).toBeNull();
  });
});

describe('enhanceUnits (D-2, D-5)', () => {
  it('restores the metric markup byte for byte and keeps every node', () => {
    page = renderSample('ride-30min');
    const root = document.body;
    const before = root.innerHTML;
    const nodes = [...root.querySelectorAll('*')];
    const store = createUnitsStore('imperial');
    enhanceUnits(root, meta('ride-30min'), store);
    expect(root.innerHTML).not.toBe(before);
    expect([...root.querySelectorAll('*')]).toEqual(nodes);
    store.set('metric');
    expect(root.innerHTML).toBe(before);
  });

  it('a markup that does not match is left untouched and logged', () => {
    page = renderSample('ride-30min');
    const before = document.body.innerHTML;
    enhanceUnits(document.body, meta('triathlon'), createUnitsStore('imperial'));
    expect(document.body.innerHTML).toBe(before);
    expect(console.warn).toHaveBeenCalled();
  });

  it('works on a run (pace) and a swim', () => {
    page = renderSample('run-30min');
    enhanceUnits(document.body, meta('run-30min'), createUnitsStore('imperial'));
    expect(document.querySelector('.metric[data-k="speed"] .u')?.textContent).toBe('/mi');
    page.remove();
    page = renderSample('pool-swim');
    enhanceUnits(document.body, meta('pool-swim'), createUnitsStore('imperial'));
    expect(document.querySelector('.metric[data-k="speed"] .u')?.textContent).toBe('/100 yd');
    expect(document.querySelector('.metric[data-k="distance"] .u')?.textContent).toBe('mi');
  });
});

describe('addUnitsSelect (D-8)', () => {
  it('fills the empty slot once; without the slot nothing happens', () => {
    page = renderSample('ride-30min');
    const store = createUnitsStore('metric');
    expect(select()).toBeNull();
    expect(addUnitsSelect(document.body, store)).not.toBeNull();
    expect(addUnitsSelect(document.body, store)).toBeNull();
    expect(document.querySelectorAll('#units-slot select').length).toBe(1);
    expect([...select()!.options].map((o) => o.value)).toEqual(['metric', 'imperial']);
    document.getElementById('units-slot')?.remove();
    expect(addUnitsSelect(document.body, store)).toBeNull();
  });
});
