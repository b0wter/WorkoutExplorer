// @vitest-environment jsdom
// The edit page (F-19 design E-1 to E-10; AC 1 to 8). Critical lane: the edit token is in the
// fragment and only ever leaves as the Authorization header (ADR-020, ADR-025). ChartStack and
// RouteMap are the page tests' recording stubs; fetch serves synthetic golden data files.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { loadGolden } from '../../../test-support/golden.mjs';
import type { Settings } from '../../lib/owner';
import { decodeDataFile } from '../../lib/datafile';
import { removableBoundaries, removeAutoLaps } from '../../lib/addedlaps';
import { createRangeStats } from '../../lib/range';
import { formatElapsed } from '../../lib/format';
import { en as editTexts } from '../texts';
import { en as errorTexts } from '../../lib/errors/texts';

import { resetStubLog, stubLog } from '../../page/testing/stub-log';
import { startEdit } from '../start.svelte';
import editHtml from '../../../edit.html?raw';
import { LIST_KEY } from '../../lib/mine';

vi.mock('../../components/charts/ChartStack.svelte', async () => await import('../../page/testing/ChartStackStub.svelte'));
vi.mock('../../components/map/RouteMap.svelte', async () => await import('../../page/testing/RouteMapStub.svelte'));
// F-36: the offscreen basemap needs WebGL2; its own test runs it on the MapLibre fake.
export const renders: [unknown, AbortSignal][] = [];

vi.mock('../../components/map/basemap', () => ({
  renderBasemap: (frame: unknown, signal: AbortSignal) => (renders.push([frame, signal]), s.render()),
}));

export const ID = 'AbCdEfGhIjKlMnOpQrStUv';
export const TOKEN = 'Zz0123456789abcdefghij';
export const golden = (name: string): ArrayBuffer => {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error('no golden ' + name);
  return g.bin;
};
export const RIDE = golden('ride-4h');
export const N = new DataView(RIDE).getUint32(6, true);
export const RANGES: [number, number][] = [
  [0, 41],
  [N - 30, N - 1],
];
/** ADR-056's table: at 300 m (k = 6) the start keeps 42 on and the end N − 31 back, i.e. RANGES. */
export const STEPS = { start: Array.from({ length: 41 }, (_, k) => 7 * k), end: Array.from({ length: 41 }, (_, k) => N - 1 - 5 * k) };
export const ON = { hide_start_m: 300, hide_end_m: 300 };

export const NOW = Date.parse('2026-10-29T12:00:00Z');
export const FIT = new Uint8Array([14, 16, 0x98, 0, 0xff, 0x2e, 0x46, 0x49, 0x54]);
export const EDIT_LINK = () => window.location.origin + '/edit/' + ID + '#' + TOKEN;
export const VIEW_LINK = () => window.location.origin + '/w/' + ID;
export const fresh = (over: Partial<Settings> = {}): Settings => ({
  title: '',
  description: '',
  hide_start_m: 0,
  hide_end_m: 0,
  visibility: 'draft',
  lifetime_days: 30,
  created_at: '2026-10-29T11:00:00Z',
  expires_at: '2026-11-28T11:00:00Z',
  data_end_s: null,
  view_url: '/w/' + ID,
  hiding_steps: STEPS,
  suggested_title: '',
  trim_start_at: null,
  trim_end_at: null,
  trimmed: null,
  zones_max_heart_rate: null,
  zones_threshold_power: null,
  pins: [],
  added_lap_boundaries: [],
  remove_auto_laps: false,
  ...over,
});

export interface Call {
  url: string;
  init: RequestInit;
}











/** requestAnimationFrame's queue; nothing runs unless a test runs it. */


/** Navigations through lib/owner: 'assign <url>', 'replace <url>'. */

/** The pages' pageshow listeners: every test shares one window, so each is removed after its test. */
export const shows: EventListenerOrEventListenerObject[] = [];

export const s = {} as {
  calls: Call[];
  settings: Settings;
  draft: () => Response;
  onGet: () => Response;
  onPatch: (body: Record<string, unknown>) => Response;
  onPublish: () => Response;
  onDelete: () => Response;
  original: () => Response;
  onFrame: () => Response;
  onPut: () => Response;
  consoleOut: unknown[][];
  frames: FrameRequestCallback[];
  host: HTMLElement;
  navs: string[];
  render: () => Promise<Blob>;
};
beforeEach(() => {
  resetStubLog();
  s.calls = [];
  s.navs = [];
  s.settings = fresh();
  s.draft = () => new Response(RIDE.slice(0), { status: 200 });
  s.onGet = () => Response.json(s.settings);
  s.onPatch = (body) => {
    s.settings = { ...s.settings, ...body } as Settings;
    return Response.json(s.settings);
  };
  s.onPublish = () => Response.json((s.settings = { ...s.settings, visibility: 'unlisted' }));
  s.onDelete = () => new Response(null, { status: 204 });
  s.onFrame = () => Response.json({ frame: null });
  s.onPut = () => new Response(null, { status: 204 });
  renders.length = 0;
  s.render = async () => new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });
  s.original = () => new Response(FIT, { headers: { 'Content-Disposition': 'attachment; filename="workout-2026-10-29.fit"' } });
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    s.calls.push({ url, init });
    if (url.endsWith('/draft.bin')) return s.draft();
    if (url.endsWith('/original.fit')) return s.original();
    if (url.endsWith('/publish')) return s.onPublish();
    if (url.endsWith('/preview-map')) return s.onFrame();
    if (url.includes('/preview-map/')) return s.onPut();
    if (init.method === 'DELETE') return s.onDelete();
    return init.method === 'PATCH' ? s.onPatch(JSON.parse(init.body as string)) : s.onGet();
  }));
  s.frames = [];
  vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback) => s.frames.push(f));
  const add = window.addEventListener.bind(window);
  vi.spyOn(window, 'addEventListener').mockImplementation((type: string, fn: EventListenerOrEventListenerObject, o?: boolean | AddEventListenerOptions) => {
    if (type === 'pageshow') shows.push(fn);
    add(type, fn, o);
  });
  vi.spyOn(Date, 'now').mockReturnValue(NOW);
  s.consoleOut = [];
  for (const k of ['log', 'info', 'warn', 'error', 'debug'] as const) {
    vi.spyOn(console, k).mockImplementation((...args: unknown[]) => void s.consoleOut.push(args));
  }
  s.host = document.createElement('div');
  s.host.id = 'wx-edit';
  document.body.replaceChildren(s.host);
  document.title = 'Workout Explorer';
});

afterEach(() => {
  for (const fn of shows.splice(0)) window.removeEventListener('pageshow', fn);
  window.history.replaceState(null, '', '/');
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
  sessionStorage.clear();
  document.body.replaceChildren();
});

export async function settle(): Promise<void> {
  for (let k = 0; k < 10; k++) {
    await new Promise((r) => setTimeout(r, 0));
    flushSync();
  }
}

export async function open(hash = '#' + TOKEN, path = '/edit/' + ID): Promise<void> {
  window.history.replaceState(null, '', path + hash);
  const l = window.location;
  await startEdit(s.host, window, {
    origin: l.origin,
    pathname: l.pathname,
    hash: l.hash,
    assign: (url: string | URL) => void s.navs.push('assign ' + url),
    replace: (url: string | URL) => void s.navs.push('replace ' + url),
  });
  await settle();
}
export const openPublished = (hash = '#' + TOKEN): Promise<void> => open(hash, '/published/' + ID);
export const PUBLISHED_PAGE = () => 'assign ' + window.location.origin + '/published/' + ID + '#' + TOKEN;

/** The E-2 list: nowhere in the document (text, attributes, input and textarea values), title,
 *  storage, cookies, history state or console. The one exception (F-20 C-5): on the published
 *  page, the edit link as exactly one text node, the only child of a translate="no" element. */
export function expectNoTokenOutsideTheFragment(): void {
  const html = document.documentElement.outerHTML;
  const walk = document.createTreeWalker(document.documentElement, NodeFilter.SHOW_TEXT);
  const holders: Text[] = [];
  for (let n = walk.nextNode(); n; n = walk.nextNode()) if ((n as Text).data.includes(TOKEN)) holders.push(n as Text);
  if (document.querySelector('main.published')) {
    expect(holders).toHaveLength(1);
    expect(holders[0].data).toBe(EDIT_LINK());
    expect(holders[0].parentElement?.getAttribute('translate')).toBe('no');
    expect(holders[0].parentElement?.childNodes).toHaveLength(1); // selects as one unit
    expect(holders[0].parentElement?.closest('a, button, input, textarea, label')).toBeNull();
    expect(html.split(TOKEN)).toHaveLength(2); // once: that text node, never an attribute
  } else {
    expect(holders).toEqual([]);
    expect(html).not.toContain(TOKEN);
  }
  for (const el of document.querySelectorAll('*')) {
    for (const a of el.attributes) expect(a.value).not.toContain(TOKEN);
  }
  // A value set as a property is not in outerHTML (QA-20-r1-2 a).
  for (const el of document.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>('input, textarea')) expect(el.value).not.toContain(TOKEN);
  expect(document.title).not.toContain(TOKEN);
  expect(document.cookie).not.toContain(TOKEN);
  for (const store of [localStorage, sessionStorage]) {
    for (let i = 0; i < store.length; i++) {
      const k = store.key(i) ?? '';
      // F-53 (ADR-058): the one exception, the My workouts entry's `token` field, never a link.
      const v = store === localStorage && k === LIST_KEY ? JSON.stringify(JSON.parse(store.getItem(k)!), (key, x) => (key === 'token' ? '' : x)) : store.getItem(k);
      expect(k + (v ?? '')).not.toContain(TOKEN);
    }
  }
  expect(JSON.stringify(window.history.state) ?? '').not.toContain(TOKEN);
  expect(JSON.stringify(s.consoleOut.map((a) => a.map(String)))).not.toContain(TOKEN);
  for (const { url, init } of s.calls) {
    expect(url).toMatch(new RegExp('^/api/workouts/' + ID + '(/draft\\.bin|/publish|/original\\.fit|/preview-map(/[0-9a-f]{16})?)?$'));
    expect(init).toMatchObject({ cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer', redirect: 'error' });
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer ' + TOKEN);
    expect(String(init.body ?? '')).not.toContain(TOKEN);
  }
}

export const $ = <T extends Element = HTMLElement>(sel: string): T | null => document.querySelector<T>(sel);
export const field = (name: string) => $<HTMLFormElement>('form.edit-panel')!.elements.namedItem(name) as HTMLInputElement & HTMLSelectElement & HTMLTextAreaElement;
export const GENERIC = 'This workout has expired or is not available.';

export function type(name: string, value: string): void {
  field(name).value = value;
  field(name).dispatchEvent(new Event('input', { bubbles: true }));
  flushSync();
}
/** F-55: moves the hiding sliders as a drag does, then runs the s.frames that queued. */
export function hideAt(startM: number, endM: number): void {
  const queued = s.frames.length;
  for (const [name, m] of [['hide_start_m', startM], ['hide_end_m', endM]] as const) {
    if (field(name).value === String(m)) continue;
    field(name).value = String(m);
    field(name).dispatchEvent(new Event('input', { bubbles: true }));
  }
  for (const f of s.frames.splice(queued)) f(0);
  flushSync();
}
export async function save(): Promise<void> {
  $<HTMLFormElement>('form.edit-panel')!.dispatchEvent(new Event('submit', { cancelable: true }));
  await settle();
}
export const patches = (): Record<string, unknown>[] => s.calls.filter((c) => c.init.method === 'PATCH').map((c) => JSON.parse(c.init.body as string));

/** The map is ready: its control stacks are #map-slot here, so a "+" control is in the document (no real MapLibre). */
export const controlPos: string[] = [];
export const mapReady = (): void => {
  const handle = { setScheme() {}, setHidden() {}, marker: (el: HTMLElement) => (document.getElementById('map-slot')!.append(el), () => el.remove()), marks() {}, laps() {}, control: (el: HTMLElement, pos?: string) => (controlPos.push(pos ?? ''), document.getElementById('map-slot')!.append(el)), sampleAt: () => -1, destroy() {} };
  (stubLog.routeMap[0] as unknown as { onmapready(h: typeof handle): void } | undefined)?.onmapready(handle);
};


export const FIELDS = ['title', 'description', 'hide_start_m', 'hide_end_m', 'lifetime'];
export const btn = (act: string) => $<HTMLButtonElement>(`[data-act="${act}"]`);
export async function click(act: string): Promise<void> {
  btn(act)!.click();
  await settle();
}
export const methods = (): string[] => s.calls.filter((c) => c.init.method).map((c) => c.init.method + ' ' + c.url.replace('/api/workouts/' + ID, ''));
export const DRAFT_LINE = 'Unpublished drafts are deleted 48 hours after upload.';
export const WARNING = 'Save this link. It is the only way to change or delete this workout, and it cannot be recovered.';
export const SH10 = 'Copies that viewers have already downloaded, and previews that messengers have already made, cannot be recalled.';
export const clipboard = (write: (t: string) => Promise<void> = async () => {}) => {
  const writeText = vi.fn(write);
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
  return writeText;
};

