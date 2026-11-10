// @vitest-environment jsdom
// The viewing page's glue (F-13 design D-8, D-12, D-13; ADR-012 rev. 2, ADR-016, ADR-026). ChartStack
// and RouteMap are replaced by test-only stubs that record their props and creation in one call
// log (./testing); the scrub recorder is mocked. fetch returns the synthetic golden ride-4h.bin.
// The markup is a committed sample rendered by the edit page's components, so the slots are the
// template's. Synthetic data only.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import type { RecorderOptions } from '../measure/recorder';
import { loadDataFile, type DataFileResult } from '../lib/datafile';
import { dataFileUrl, startPage } from './start';
import { renderSample, type Fixture } from './testing/fixture';
import { resetStubLog, stubLog } from './testing/stub-log';

vi.mock('../components/charts/ChartStack.svelte', async () => await import('./testing/ChartStackStub.svelte'));
vi.mock('../components/map/RouteMap.svelte', async () => await import('./testing/RouteMapStub.svelte'));

const rec = vi.hoisted(() => ({
  /** How often the mocked module was loaded (its factory runs on the first import only). */
  imports: 0,
  options: [] as RecorderOptions[],
}));

vi.mock('../measure/recorder', async () => {
  rec.imports++;
  const { stubLog: log } = await import('./testing/stub-log');
  return {
    createRecorder: (o: RecorderOptions) => {
      log.calls.push('createRecorder');
      rec.options.push(o);
      return {
        chartsReady: () => log.calls.push('chartsReady'),
        mapReady: () => log.calls.push('mapReady'),
      };
    },
  };
});

const golden = loadGolden().find((g) => g.name === 'ride-4h');
if (!golden) throw new Error('no golden ride-4h');
const RIDE = golden.bin;
const N = new DataView(RIDE).getUint32(6, true); // F-03 §1: sample count after magic and version

let fetchMock: ReturnType<typeof vi.fn>;
let respond: () => Response;
let page: Fixture | null = null;

beforeEach(() => {
  resetStubLog();
  rec.options = [];
  respond = () => new Response(RIDE.slice(0), { status: 200 });
  fetchMock = vi.fn(async () => respond());
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('requestAnimationFrame', () => 1);
  vi.spyOn(console, 'warn').mockImplementation(() => {});
  // A fixed clock before the samples' expiry (2026-10-30T09:15Z): the expiry text must not depend on today.
  vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-24T09:15:00Z'));
  performance.clearMarks();
});

afterEach(async () => {
  // A range hint still collapsing (start.ts, 200 ms) must not land in the next test.
  if (document.documentElement.dataset.hint === 'gone') await new Promise((r) => setTimeout(r, 250));
  delete document.documentElement.dataset.hint;
  page?.remove();
  page = null;
  window.history.replaceState(null, '', '/');
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear(); // remembered viewer choices (lib/prefs) must not leak between tests
});

/** Renders the sample, runs the page glue at the given query and lets the load finish. */
async function open(sample = 'ride-30min', search = ''): Promise<void> {
  window.history.replaceState(null, '', '/w/x' + search);
  page = renderSample(sample);
  await startPage(document);
  await settle();
}

async function settle(): Promise<void> {
  for (let k = 0; k < 10; k++) {
    await new Promise((r) => setTimeout(r, 0));
    flushSync();
  }
}

const slot = (id: string): HTMLElement | null => document.getElementById(id);

describe('which file (D-12)', () => {
  it('fetches the data file exactly as the page names it, once; nothing is built from the location', async () => {
    await open('ride-30min', '?data=other');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe(page?.model.Page.DataURL);
    expect(fetchMock.mock.calls[0][0]).toMatch(/^\/w\/ride-30min\/data\.[0-9a-f]{16}\.bin$/);
  });

  it('dataFileUrl reads the preload link; a page without one has none', () => {
    expect(dataFileUrl(document)).toBeNull();
    page = renderSample('ride-30min');
    expect(dataFileUrl(document)).toBe(page.model.Page.DataURL);
  });

  it('no data file link: the error text, nothing fetched or mounted', async () => {
    window.history.replaceState(null, '', '/w/x');
    page = renderSample('ride-30min');
    document.getElementById('wx-data')?.remove();
    await startPage(document);
    expect(slot('map-slot')?.textContent).toBe('The workout data could not be loaded.');
    expect(fetchMock).not.toHaveBeenCalled();
    expect(stubLog.calls).toEqual([]);
  });
});

describe('load failure (AC 9, D-13, OQ-10)', () => {
  it('404: the exact text, no technical detail, neither island mounted, the server markup stays', async () => {
    respond = () => new Response('not found\n', { status: 404 });
    const summaryBefore = (): string => document.querySelector('.summary')?.innerHTML ?? '';
    window.history.replaceState(null, '', '/w/x');
    page = renderSample('ride-30min');
    const before = summaryBefore();
    await startPage(document);
    await settle();
    expect(slot('map-slot')?.textContent).toBe('The workout data could not be loaded.');
    expect(document.body.textContent).not.toMatch(/404|HTTP|data file:/);
    expect(stubLog.calls).toEqual([]);
    expect(summaryBefore()).toBe(before);
    expect(document.querySelector('h1')).not.toBeNull();
    expect(console.warn).toHaveBeenCalledWith('data file: HTTP status 404');
  });

  it('a different format version (bytes 4 and 5 = 3): the text ends in "Reload the page."', async () => {
    respond = () => {
      const b = RIDE.slice(0);
      new DataView(b).setUint16(4, 3, true);
      return new Response(b, { status: 200 });
    };
    await open();
    expect(slot('map-slot')?.textContent).toBe('The workout data could not be loaded. Reload the page.');
    expect(slot('charts-slot')?.textContent).toBe('The workout data could not be loaded. Reload the page.');
    expect(slot('charts-slot')?.classList.contains('load-failed')).toBe(true);
    expect(stubLog.calls).toEqual([]);
  });

  it('a workout without a map shows the text in the chart area', async () => {
    respond = () => new Response('', { status: 500 });
    await open('indoor-ride');
    expect(slot('map-slot')).toBeNull();
    expect(slot('charts-slot')?.textContent).toBe('The workout data could not be loaded.');
  });

  it('F-28 RF-2: Report works while the data file hangs and after it failed', async () => {
    let fail: (r: Response) => void = () => {};
    respond = (() => new Promise<Response>((r) => (fail = r))) as unknown as () => Response;
    window.history.replaceState(null, '', '/w/x');
    page = renderSample('ride-30min');
    document.querySelector('a[data-copy-link]')!.setAttribute('href', '/w/AbCdEfGhIjKlMnOpQrStUv');
    const report = document.querySelector<HTMLButtonElement>('footer button[data-report]')!;
    expect(report.hidden).toBe(true);
    const started = startPage(document);
    expect(report.hidden).toBe(false);
    fail(new Response('', { status: 500 }));
    await started;
    expect(slot('map-slot')?.textContent).toBe('The workout data could not be loaded.');
    expect(report.hidden).toBe(false);
  });
});

describe('adoption, not hydration (D-8, ADR-026 item 4)', () => {
  it('Svelte mounts only into the empty slots; the server markup is the same nodes with the same text', async () => {
    window.history.replaceState(null, '', '/w/x');
    page = renderSample('ride-30min');
    const nodes = [...document.querySelectorAll('header, .summary, .notice, .laps, footer')];
    const html = nodes.map((n) => n.outerHTML);
    await startPage(document);
    await settle();
    // Only the expiry text changes (enhancer), the units and theme selects and the laps switch arrive in their
    // reserved slots, nothing else (the laps module's 'live' class comes with the taps, after onready).
    const after = [...document.querySelectorAll('header, .summary, .notice, .laps, footer')];
    expect(after).toEqual(nodes);
    const strip = (h: string): string =>
      h.replace(/Expires in [^<]+/, 'X').replace(/<select.*?<\/select>/g, '').replace(/<span class="laps-switch">.*?<\/span>/, '').replace('class="laps live"', 'class="laps"');
    expect(after.map((n) => strip(n.outerHTML))).toEqual(html.map((h) => h.replace(/Expires around [^<]+/, 'X')));
  });
});

describe('wiring (F-06 "Wiring")', () => {
  it('both stubs get the same cursor for N samples; RouteMap load is false until ChartStack calls onready', async () => {
    await open();
    expect(stubLog.calls).toEqual(['RouteMap', 'ChartStack']);
    const [map] = stubLog.routeMap;
    const [charts] = stubLog.chartStack;
    expect(charts.cursor).toBe(map.cursor);
    expect(charts.data).toBe(map.data);
    expect(charts.cursor.sampleCount).toBe(N);
    expect(map.load).toBe(false);

    charts.onready?.();
    flushSync();
    expect(map.load).toBe(true);
    expect(stubLog.calls).toEqual(['RouteMap', 'ChartStack']); // no remount
  });

  it('lap rows are tappable only once the range panel is mounted (L-7, QA-18-r1-3)', async () => {
    const g = loadGolden().find((x) => x.name === 'ride-30min');
    if (!g) throw new Error('no golden ride-30min');
    respond = () => new Response(g.bin.slice(0), { status: 200 }); // rows match the data file (L-2)
    await open();
    const [charts] = stubLog.chartStack;
    const row = document.querySelector<HTMLElement>('section.laps tbody tr');
    row?.click();
    expect(charts.selection?.start()).toBe(-1);
    expect(document.querySelector('section.laps')?.classList.contains('live')).toBe(false);
    charts.onready?.();
    flushSync();
    expect(document.getElementById('range-slot')?.childElementCount).toBeGreaterThan(0);
    row?.click();
    expect(charts.selection?.start()).toBe(0);
  });

  it('a workout without location data mounts no map, only the charts', async () => {
    await open('indoor-ride');
    expect(stubLog.calls).toEqual(['ChartStack']);
    expect(document.querySelector('.hint')?.textContent).toBe('No location data in this workout.');
  });

  it('(7) without measure: the recorder module is never imported', async () => {
    // Runs before the ?measure test in this file: the mock factory counts only the first import.
    await open();
    stubLog.chartStack[0].onready?.();
    stubLog.routeMap[0].onmapready?.();
    flushSync();
    expect(rec.imports).toBe(0);
    expect(stubLog.calls).toEqual(['RouteMap', 'ChartStack']);
  });

  it('(6) ?measure: createRecorder once, with hasRoute true, before ChartStack; chartsReady after onready, mapReady after onmapready', async () => {
    await open('ride-30min', '?measure');
    expect(rec.imports).toBe(1);
    expect(rec.options).toHaveLength(1);
    const o = rec.options[0];
    expect(o.hasRoute).toBe(true);
    expect(o.name).toMatch(/^data\.[0-9a-f]{16}\.bin$/);
    expect(o.cursor).toBe(stubLog.chartStack[0].cursor);
    expect(o.data).toBe(stubLog.chartStack[0].data);
    expect(stubLog.calls).toEqual(['createRecorder', 'RouteMap', 'ChartStack']);

    stubLog.chartStack[0].onready?.();
    expect(stubLog.calls.slice(3)).toEqual(['chartsReady']);
    stubLog.routeMap[0].onmapready?.();
    expect(stubLog.calls.slice(3)).toEqual(['chartsReady', 'mapReady']);
  });

  it('(8) User Timing marks: load-start, data-ready, charts-ready, map-ready', async () => {
    await open();
    const has = (name: string) => performance.getEntriesByName(name, 'mark').length;
    expect(has('wx-page:load-start')).toBe(1);
    expect(has('wx-page:data-ready')).toBe(1);
    expect(has('wx-page:charts-ready')).toBe(0);
    stubLog.chartStack[0].onready?.();
    expect(has('wx-page:charts-ready')).toBe(1);
    stubLog.routeMap[0].onmapready?.();
    expect(has('wx-page:map-ready')).toBe(1);
  });
});

describe('range panel (F-17 R-9, AC 4, AC 5)', () => {
  const rangeSlot = (): HTMLElement => slot('range-slot') as HTMLElement;
  const values = (): string[] => [...rangeSlot().querySelectorAll('.value')].map((e) => e.textContent ?? '');
  const labels = (): string[] => [...rangeSlot().querySelectorAll('.label')].map((e) => e.textContent ?? '');

  beforeEach(() => {
    // Store flushes run at once here: the panel's transitions are under test, not the frame timing.
    vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback) => {
      f(0);
      return 1;
    });
  });

  it('charts and map share one selection store for N samples', async () => {
    await open();
    const sel = stubLog.chartStack[0].selection;
    expect(sel).toBeDefined();
    expect(stubLog.routeMap[0].selection).toBe(sel);
    expect(sel?.sampleCount).toBe(N);
  });

  it('mounted only after the charts are ready; hidden until a range exists, nothing reserved', async () => {
    await open();
    expect(rangeSlot().childElementCount).toBe(0);
    expect(rangeSlot().hidden).toBe(false); // the template's empty, zero-height slot
    stubLog.chartStack[0].onready?.();
    flushSync();
    expect(rangeSlot().querySelector('.range-panel')).not.toBeNull();
    expect(rangeSlot().hidden).toBe(true);
  });

  it('a range shows the panel with its values; the clear control clears it and hides the panel', async () => {
    await open();
    stubLog.chartStack[0].onready?.();
    flushSync();
    const sel = stubLog.chartStack[0].selection;
    if (!sel) throw new Error('no selection');
    sel.set(100, 3000);
    flushSync();
    expect(rangeSlot().hidden).toBe(false);
    expect(labels().slice(0, 4)).toEqual(['Distance', 'Time', 'Moving', 'Gain']);
    expect(values()[0]).toMatch(/^\d+\.\d\d$/);
    expect(values()[1]).toMatch(/^\d+:\d\d(:\d\d)?$/);
    expect(rangeSlot().textContent).toContain('mi'); // jsdom's language is en-US: imperial (F-14 D-4)
    (rangeSlot().querySelector('.range-panel .actions button.clear') as HTMLButtonElement).click();
    flushSync();
    expect([sel.start(), sel.end()]).toEqual([-1, -1]);
    expect(rangeSlot().hidden).toBe(true);
  });

  it('a units change rewrites the panel without a new range', async () => {
    await open();
    stubLog.chartStack[0].onready?.();
    flushSync();
    const sel = stubLog.chartStack[0].selection;
    sel?.set(100, 3000);
    flushSync();
    const mi = Number(values()[0]);
    const select = document.querySelector('#units-slot select') as HTMLSelectElement;
    expect(select.value).toBe('imperial');
    select.value = 'metric';
    select.dispatchEvent(new Event('change'));
    flushSync();
    expect(rangeSlot().textContent).toContain('km');
    expect(Number(values()[0])).toBeCloseTo(mi * 1.609344, 1);
  });

  it('no panel on a load failure', async () => {
    respond = () => new Response('x', { status: 404 });
    await open();
    expect(rangeSlot().querySelector('.range-panel')).toBeNull();
  });
});

describe('How to use and the range hint (F-39 HU-1 to HU-6; AC 2, AC 6, AC 7)', () => {
  beforeEach(() => {
    vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback) => {
      f(0);
      return 1;
    });
  });
  afterEach(() => {
    delete document.documentElement.dataset.hint;
  });
  const hint = (): string | undefined => stubLog.chartStack[0].hint;
  // User 10 Nov 2026: the box fades and its space shrinks (data-hint 'gone', 200 ms), then it goes.
  const collapsed = async (): Promise<void> => {
    expect([hint(), document.documentElement.dataset.hint]).toEqual(['once', 'gone']);
    await new Promise((r) => setTimeout(r, 250));
    flushSync();
    expect([hint(), document.documentElement.dataset.hint]).toEqual(['none', 'done']);
  };

  it('fresh: the hint with its ×; the first range writes the flag once and removes the box and its space', async () => {
    await open();
    expect(hint()).toBe('once');
    stubLog.chartStack[0].selection?.set(10, 20);
    flushSync();
    await collapsed();
    expect(localStorage.getItem('wx.hint.done')).toBe('1');
  });

  it('the × does the same', async () => {
    await open();
    stubLog.chartStack[0].onhint?.();
    flushSync();
    await collapsed();
    expect(localStorage.getItem('wx.hint.done')).toBe('1');
  });

  it('data-hint="done" from the head script: no line, and a range writes nothing', async () => {
    document.documentElement.dataset.hint = 'done';
    await open();
    expect(hint()).toBe('none');
    stubLog.chartStack[0].selection?.set(10, 20);
    flushSync();
    expect(localStorage.length).toBe(0);
  });

  it('the button is emphasised until opened on this device; storage is untouched by loading', async () => {
    page = renderSample('ride-30min');
    const row = () => document.querySelector('.head')!.insertAdjacentHTML('afterend', '<div class="howto-row"><button type="button" class="howto" data-howto>How to use this page</button></div>');
    row();
    await startPage(document);
    expect(document.querySelector('.howto')?.className).toBe('howto ready fresh');
    expect(localStorage.length).toBe(0);
    page.remove();
    localStorage.setItem('wx.howto.opened', '1');
    page = renderSample('ride-30min');
    row();
    await startPage(document);
    expect(document.querySelector('.howto')?.className).toBe('howto ready');
  });

  it('storage that throws: the button emphasised and the hint shown, no error', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('SecurityError');
    });
    await open();
    expect(hint()).toBe('once');
    stubLog.chartStack[0].onhint?.();
    flushSync();
    await collapsed();
  });
});

describe('range link (F-40 SR-1 to SR-8; AP-19)', () => {
  let decoded: DataFileResult;
  let time: Float64Array;
  let writeText: ReturnType<typeof vi.fn>;
  beforeEach(async () => {
    vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback) => {
      f(0);
      return 1;
    });
    decoded = await loadDataFile(async () => new Response(RIDE.slice(0)));
    if (!decoded.ok) throw new Error('golden does not decode');
    time = decoded.data.time;
    writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
  });
  afterEach(() => {
    delete (navigator as { clipboard?: unknown }).clipboard;
    delete document.documentElement.dataset.hint;
  });
  const sel = () => stubLog.chartStack[0].selection!;
  const range = (): [number, number] => [sel().start(), sel().end()];
  const hint = (): string | undefined => stubLog.chartStack[0].hint;
  // User 10 Nov 2026: the box fades and its space shrinks (data-hint 'gone', 200 ms), then it goes.
  const collapsed = async (): Promise<void> => {
    expect([hint(), document.documentElement.dataset.hint]).toEqual(['once', 'gone']);
    await new Promise((r) => setTimeout(r, 250));
    flushSync();
    expect([hint(), document.documentElement.dataset.hint]).toEqual(['none', 'done']);
  };
  const rangeSlot = (): HTMLElement => slot('range-slot') as HTMLElement;
  const share = (): HTMLButtonElement | null => rangeSlot().querySelector('.range-panel .actions button.share');
  const frag = (a: number, b: number): string => `#range=${time[a]}-${time[b]}`;
  async function openAt(path: string, seam?: boolean): Promise<void> {
    window.history.replaceState({ fs: 1 }, '', path);
    page = renderSample('ride-30min');
    await startPage(document, window, seam && decoded.ok ? { data: decoded, hidden: [] } : undefined);
    await settle();
    stubLog.chartStack[0].onready?.();
    flushSync();
  }

  it('selects the link range before the charts mount; it is not the viewer’s: no hint done, the address stays', async () => {
    await openAt('/w/x?q=1' + frag(100, 3000));
    expect(range()).toEqual([100, 3000]);
    expect(hint()).toBe('once');
    expect(localStorage.getItem('wx.hint.done')).toBeNull();
    expect(window.location.hash).toBe(frag(100, 3000));
    expect(rangeSlot().hidden).toBe(false);
  });

  it('the viewer’s first change drops the fragment without a history entry, keeping query and state; the hint goes', async () => {
    await openAt('/w/x?q=1' + frag(100, 3000));
    const length = window.history.length;
    sel().set(200, 400);
    flushSync();
    expect([window.location.pathname, window.location.search, window.location.hash]).toEqual(['/w/x', '?q=1', '']);
    expect(window.history.length).toBe(length);
    expect(window.history.state).toEqual({ fs: 1 });
    await collapsed();
  });

  it('Clear also drops it, and does not count as a selection for the hint', async () => {
    await openAt('/w/x' + frag(100, 3000));
    (rangeSlot().querySelector('.range-panel .actions button.clear') as HTMLButtonElement).click();
    flushSync();
    expect(window.location.hash).toBe('');
    expect(hint()).toBe('once');
  });

  it.each(['#range=abc', '#range=5-1', '#range=7-7', '#range=99999999-1', '#range=1-5&x', '#x'])('ignores %s: no range, the page works; it stays until the viewer’s change', async (h) => {
    await openAt('/w/x' + h);
    expect(range()).toEqual([-1, -1]);
    expect(window.location.hash).toBe(h);
    expect(rangeSlot().hidden).toBe(true);
    sel().set(10, 20);
    flushSync();
    expect(window.location.hash).toBe('');
  });

  it('cuts a range past the data to the data', async () => {
    await openAt(`/w/x#range=0-9999999`);
    expect(range()).toEqual([0, N - 1]);
  });

  it('the map gets the link range: the same store as the charts (AC 1)', async () => {
    await openAt('/w/x' + frag(100, 3000));
    const map = stubLog.routeMap[0].selection!;
    expect(map).toBe(sel());
    expect([map.start(), map.end()]).toEqual([100, 3000]);
  });

  it('a link into a hidden start: cut to the first shown sample, or nothing when it is all hidden (AC 6)', async () => {
    // Start hiding: the published file's offsets begin later. Shift the first time delta by K s.
    const K = 600;
    const buf = RIDE.slice(0);
    const view = new DataView(buf);
    const meta = JSON.parse(new TextDecoder().decode(new Uint8Array(buf, 14, view.getUint32(10, true))));
    const off = meta.streams.find((s: { name: string }) => s.name === 'time_offset').byte_offset;
    view.setInt32(off, view.getInt32(off, true) + K, true);
    respond = () => new Response(buf.slice(0));
    await openAt(`/w/x#range=0-${time[500] + K}`);
    expect(range()).toEqual([0, 500]);
    page?.remove();
    resetStubLog();
    await openAt(`/w/x#range=0-${K - 1}`);
    expect(range()).toEqual([-1, -1]);
  });

  it('without a fragment, selecting and clearing leave the address alone', async () => {
    await openAt('/w/x');
    sel().set(10, 20);
    flushSync();
    sel().clear();
    flushSync();
    expect(window.location.href).toBe(window.location.origin + '/w/x');
  });

  it('the edit seam neither reads nor writes it, and has no button', async () => {
    await openAt('/edit/x' + frag(100, 3000), true);
    expect(range()).toEqual([-1, -1]);
    sel().set(10, 20);
    flushSync();
    expect(window.location.hash).toBe(frag(100, 3000));
    expect(rangeSlot().querySelector('.range-panel')).not.toBeNull();
    expect(share()).toBeNull();
  });

  it('the button copies the viewing link with the range; "Link copied" for 2 s; the copy round-trips; Copy link stays plain', async () => {
    await openAt('/w/x');
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    try {
      expect(share()).not.toBeNull(); // in the hidden panel: shows only with a range
      sel().set(123, 2345);
      flushSync();
      share()!.click();
      await Promise.resolve();
      await Promise.resolve();
      const view = window.location.origin + page!.model.Page.ViewPath;
      expect(writeText).toHaveBeenCalledWith(view + frag(123, 2345));
      expect(share()!.textContent).toBe('Link copied');
      vi.advanceTimersByTime(2000);
      expect(share()!.textContent).toBe('Copy link to this selection');
      (document.querySelector('a[data-copy-link]') as HTMLAnchorElement).click();
      await Promise.resolve();
      await Promise.resolve();
      expect(writeText).toHaveBeenLastCalledWith(view);
    } finally {
      vi.useRealTimers();
    }
    const copied = String(writeText.mock.calls[0][0]);
    page?.remove();
    resetStubLog();
    await openAt('/w/x' + copied.slice(copied.indexOf('#')));
    expect(range()).toEqual([123, 2345]);
  });

  it('without the Clipboard API the link shows in a selected field in the panel, removed when the range changes', async () => {
    delete (navigator as { clipboard?: unknown }).clipboard;
    await openAt('/w/x');
    sel().set(10, 20);
    flushSync();
    share()!.click();
    await settle();
    const field = rangeSlot().querySelector<HTMLInputElement>('.range-panel input.copy-url')!;
    expect(field.value).toBe(window.location.origin + page!.model.Page.ViewPath + frag(10, 20));
    expect([field.readOnly, document.activeElement === field]).toEqual([true, true]);
    expect(document.querySelector('header input.copy-url')).toBeNull();
    sel().set(10, 30);
    flushSync();
    expect(rangeSlot().querySelector('input.copy-url')).toBeNull();
  });
});

describe('fullscreen readout strip (F-33 CR-2)', () => {
  it('mounted into the map box only after onmapready; none without a map', async () => {
    await open('ride-30min');
    const box = document.createElement('div'); // the stub renders no div.route-map
    box.className = 'route-map';
    slot('map-slot')?.append(box);
    expect(document.querySelector('.wx-strip')).toBeNull();
    stubLog.routeMap[0].onmapready?.();
    expect(box.querySelector('.wx-strip .readout')).not.toBeNull();
    page?.remove();
    resetStubLog();
    await open('indoor-ride');
    expect(document.querySelector('.wx-strip')).toBeNull();
  });
});

describe('pin toggle (D-19, AP-18)', () => {
  const buttons = (): NodeListOf<Element> => document.querySelectorAll('#map-slot .map-pin');

  it('appears only after onmapready, off, as a sibling inside #map-slot; fills the map instead of a height', async () => {
    await open('ride-30min');
    expect(buttons()).toHaveLength(0);
    expect(stubLog.routeMap[0].fill).toBe(true);
    expect(stubLog.routeMap[0].height).toBeUndefined();
    stubLog.routeMap[0].onmapready?.();
    expect(buttons()).toHaveLength(1);
    expect(buttons()[0].getAttribute('aria-pressed')).toBe('false');
    expect(document.querySelector('.layout')?.classList.contains('pinned')).toBe(false);
  });

  it('flips the class and aria-pressed; no remount, nothing stored', async () => {
    expect(localStorage.length).toBe(0);
    await open('ride-30min');
    stubLog.routeMap[0].onmapready?.();
    const b = buttons()[0] as HTMLButtonElement;
    b.click();
    expect(document.querySelector('.layout')?.classList.contains('pinned')).toBe(true);
    expect(b.getAttribute('aria-pressed')).toBe('true');
    expect(b.getAttribute('aria-label')).toBe('Pin map');
    b.click();
    expect(document.querySelector('.layout')?.classList.contains('pinned')).toBe(false);
    expect(b.getAttribute('aria-pressed')).toBe('false');
    expect(stubLog.routeMap).toHaveLength(1);
    expect(window.location.hash).toBe('');
    expect(localStorage.length + sessionStorage.length).toBe(0);
  });

  it('never exists without a map (indoor workout) or on a failed load', async () => {
    await open('indoor-ride');
    expect(document.querySelector('.map-pin')).toBeNull();
    page?.remove();
    respond = () => new Response('x', { status: 404 });
    await open('ride-30min');
    expect(document.querySelector('.map-pin')).toBeNull();
  });
});
