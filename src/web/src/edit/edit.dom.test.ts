// @vitest-environment jsdom
// The edit page (F-19 design E-1 to E-10; AC 1 to 8). Critical lane: the edit token is in the
// fragment and only ever leaves as the Authorization header (ADR-020, ADR-025). ChartStack and
// RouteMap are the page tests' recording stubs; fetch serves synthetic golden data files.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import type { Settings } from '../lib/owner';
import { decodeDataFile } from '../lib/datafile';
import { formatElapsed } from '../lib/format';
import { en as editTexts } from './texts';
import { en as errorTexts } from '../lib/errors/texts';

const PIN_REFUSED = editTexts.pinRefused;
import { resetStubLog, stubLog } from '../page/testing/stub-log';
import { startEdit } from './start.svelte';
import editHtml from '../../edit.html?raw';
import { LIST_KEY } from '../lib/mine';

vi.mock('../components/charts/ChartStack.svelte', async () => await import('../page/testing/ChartStackStub.svelte'));
vi.mock('../components/map/RouteMap.svelte', async () => await import('../page/testing/RouteMapStub.svelte'));
// F-36: the offscreen basemap needs WebGL2; its own test runs it on the MapLibre fake.
const renders: [unknown, AbortSignal][] = [];
let render: () => Promise<Blob>;
vi.mock('../components/map/basemap', () => ({
  renderBasemap: (frame: unknown, signal: AbortSignal) => (renders.push([frame, signal]), render()),
}));

const ID = 'AbCdEfGhIjKlMnOpQrStUv';
const TOKEN = 'Zz0123456789abcdefghij';
const golden = (name: string): ArrayBuffer => {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error('no golden ' + name);
  return g.bin;
};
const RIDE = golden('ride-4h');
const N = new DataView(RIDE).getUint32(6, true);
const RANGES: [number, number][] = [
  [0, 41],
  [N - 30, N - 1],
];
/** ADR-056's table: at 300 m (k = 6) the start keeps 42 on and the end N − 31 back, i.e. RANGES. */
const STEPS = { start: Array.from({ length: 41 }, (_, k) => 7 * k), end: Array.from({ length: 41 }, (_, k) => N - 1 - 5 * k) };
const ON = { hide_start_m: 300, hide_end_m: 300 };

const NOW = Date.parse('2026-10-29T12:00:00Z');
const FIT = new Uint8Array([14, 16, 0x98, 0, 0xff, 0x2e, 0x46, 0x49, 0x54]);
const EDIT_LINK = () => window.location.origin + '/edit/' + ID + '#' + TOKEN;
const VIEW_LINK = () => window.location.origin + '/w/' + ID;
const fresh = (over: Partial<Settings> = {}): Settings => ({
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
  ...over,
});

interface Call {
  url: string;
  init: RequestInit;
}
let calls: Call[];
let settings: Settings;
let draft: () => Response;
let onGet: () => Response;
let onPatch: (body: Record<string, unknown>) => Response;
let onPublish: () => Response;
let onDelete: () => Response;
let original: () => Response;
let onFrame: () => Response;
let onPut: () => Response;
let consoleOut: unknown[][];
/** requestAnimationFrame's queue; nothing runs unless a test runs it. */
let frames: FrameRequestCallback[];
let host: HTMLElement;
/** Navigations through lib/owner: 'assign <url>', 'replace <url>'. */
let navs: string[];
/** The pages' pageshow listeners: every test shares one window, so each is removed after its test. */
const shows: EventListenerOrEventListenerObject[] = [];

beforeEach(() => {
  resetStubLog();
  calls = [];
  navs = [];
  settings = fresh();
  draft = () => new Response(RIDE.slice(0), { status: 200 });
  onGet = () => Response.json(settings);
  onPatch = (body) => {
    settings = { ...settings, ...body } as Settings;
    return Response.json(settings);
  };
  onPublish = () => Response.json((settings = { ...settings, visibility: 'unlisted' }));
  onDelete = () => new Response(null, { status: 204 });
  onFrame = () => Response.json({ frame: null });
  onPut = () => new Response(null, { status: 204 });
  renders.length = 0;
  render = async () => new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });
  original = () => new Response(FIT, { headers: { 'Content-Disposition': 'attachment; filename="workout-2026-10-29.fit"' } });
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    calls.push({ url, init });
    if (url.endsWith('/draft.bin')) return draft();
    if (url.endsWith('/original.fit')) return original();
    if (url.endsWith('/publish')) return onPublish();
    if (url.endsWith('/preview-map')) return onFrame();
    if (url.includes('/preview-map/')) return onPut();
    if (init.method === 'DELETE') return onDelete();
    return init.method === 'PATCH' ? onPatch(JSON.parse(init.body as string)) : onGet();
  }));
  frames = [];
  vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback) => frames.push(f));
  const add = window.addEventListener.bind(window);
  vi.spyOn(window, 'addEventListener').mockImplementation((type: string, fn: EventListenerOrEventListenerObject, o?: boolean | AddEventListenerOptions) => {
    if (type === 'pageshow') shows.push(fn);
    add(type, fn, o);
  });
  vi.spyOn(Date, 'now').mockReturnValue(NOW);
  consoleOut = [];
  for (const k of ['log', 'info', 'warn', 'error', 'debug'] as const) {
    vi.spyOn(console, k).mockImplementation((...args: unknown[]) => void consoleOut.push(args));
  }
  host = document.createElement('div');
  host.id = 'wx-edit';
  document.body.replaceChildren(host);
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

async function settle(): Promise<void> {
  for (let k = 0; k < 10; k++) {
    await new Promise((r) => setTimeout(r, 0));
    flushSync();
  }
}

async function open(hash = '#' + TOKEN, path = '/edit/' + ID): Promise<void> {
  window.history.replaceState(null, '', path + hash);
  const l = window.location;
  await startEdit(host, window, {
    origin: l.origin,
    pathname: l.pathname,
    hash: l.hash,
    assign: (url: string | URL) => void navs.push('assign ' + url),
    replace: (url: string | URL) => void navs.push('replace ' + url),
  });
  await settle();
}
const openPublished = (hash = '#' + TOKEN): Promise<void> => open(hash, '/published/' + ID);
const PUBLISHED_PAGE = () => 'assign ' + window.location.origin + '/published/' + ID + '#' + TOKEN;

/** The E-2 list: nowhere in the document (text, attributes, input and textarea values), title,
 *  storage, cookies, history state or console. The one exception (F-20 C-5): on the published
 *  page, the edit link as exactly one text node, the only child of a translate="no" element. */
function expectNoTokenOutsideTheFragment(): void {
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
  expect(JSON.stringify(consoleOut.map((a) => a.map(String)))).not.toContain(TOKEN);
  for (const { url, init } of calls) {
    expect(url).toMatch(new RegExp('^/api/workouts/' + ID + '(/draft\\.bin|/publish|/original\\.fit|/preview-map(/[0-9a-f]{16})?)?$'));
    expect(init).toMatchObject({ cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer', redirect: 'error' });
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer ' + TOKEN);
    expect(String(init.body ?? '')).not.toContain(TOKEN);
  }
}

const $ = <T extends Element = HTMLElement>(sel: string): T | null => document.querySelector<T>(sel);
const field = (name: string) => $<HTMLFormElement>('form.edit-panel')!.elements.namedItem(name) as HTMLInputElement & HTMLSelectElement & HTMLTextAreaElement;
const GENERIC = 'This workout has expired or is not available.';

function type(name: string, value: string): void {
  field(name).value = value;
  field(name).dispatchEvent(new Event('input', { bubbles: true }));
  flushSync();
}
/** F-55: moves the hiding sliders as a drag does, then runs the frames that queued. */
function hideAt(startM: number, endM: number): void {
  const queued = frames.length;
  for (const [name, m] of [['hide_start_m', startM], ['hide_end_m', endM]] as const) {
    if (field(name).value === String(m)) continue;
    field(name).value = String(m);
    field(name).dispatchEvent(new Event('input', { bubbles: true }));
  }
  for (const f of frames.splice(queued)) f(0);
  flushSync();
}
async function save(): Promise<void> {
  $<HTMLFormElement>('form.edit-panel')!.dispatchEvent(new Event('submit', { cancelable: true }));
  await settle();
}
const patches = (): Record<string, unknown>[] => calls.filter((c) => c.init.method === 'PATCH').map((c) => JSON.parse(c.init.body as string));

describe('AC 1: a fresh upload, the token only in a header', () => {
  it('loads settings and draft.bin in parallel, renders the page with the panel first, leaves the address alone', async () => {
    await open('#' + TOKEN + '&x');
    expect(window.location.hash).toBe('#' + TOKEN + '&x'); // E-3: nothing rewrites it
    expect(calls.map((c) => c.url).sort()).toEqual(['/api/workouts/' + ID, '/api/workouts/' + ID + '/draft.bin']);
    expect($('main.published')).toBeNull();
    expect(window.location.pathname).toBe('/edit/' + ID);
    const layout = $('.page.has-map > .layout')!;
    expect(layout.firstElementChild?.id).toBe('edit-slot');
    expect(layout.firstElementChild?.firstElementChild?.matches('form.edit-panel')).toBe(true);
    expect(layout.children[1].matches('header.head')).toBe(true);
    expect($('h1')?.textContent).toMatch(/^Ride, /);
    expect(document.title).toBe($('h1')?.textContent);
    expect(field('title').placeholder).toBe($('h1')?.textContent);
    expect($('footer button[data-report]')).toBeNull();
    expect($<HTMLSelectElement>('header #theme-slot select')?.value).toBe('system'); // DS-4
    expect(stubLog.calls).toEqual(['RouteMap', 'ChartStack']);
    expectNoTokenOutsideTheFragment();
  });

  it("F-39 AC 9: today's hint, even after the viewer's hint is done, and no How to use button", async () => {
    document.documentElement.dataset.hint = 'done';
    try {
      await open();
      expect(stubLog.chartStack[0].hint).toBe('plain');
      expect($('[data-howto]')).toBeNull();
    } finally {
      delete document.documentElement.dataset.hint;
    }
  });

  it('F-57: no GPX button (GX-5); the viewer download is stated under the hiding sliders (OQ-271)', async () => {
    await open();
    expect($('[data-gpx]')).toBeNull();
    const field = $('input[name="hide_end_m"]')!.closest('.edit-field')!;
    expect(field.lastElementChild?.textContent).toBe('Viewers can download the visible route as a GPX file: the track and its public pins, without times, heart rate or any other data.');
  });

  it.each(['triathlon', 'indoor-no-gps'])('F-57 QA-57-r1-1: no GPX line for %s (multisport or no positions)', async (name) => {
    draft = () => new Response(golden(name).slice(0), { status: 200 });
    await open();
    expect($('input[name="hide_end_m"]')).not.toBeNull();
    expect($('.edit-panel')?.textContent).not.toContain('GPX');
  });

  it('the copy link copies the view path, never the address', async () => {
    const writeText = vi.fn(async () => {});
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    settings = fresh({ visibility: 'unlisted' });
    await open();
    expect($<HTMLAnchorElement>('header a[data-copy-link]')!.hidden).toBe(false);
    $('header a[data-copy-link]')!.click();
    await settle();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(window.location.origin + '/w/' + ID);
  });

  it('a draft has no copy link (no /w/ page yet); it follows the visibility the server returns', async () => {
    const writeText = vi.fn(async () => {});
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    await open();
    const link = () => $<HTMLAnchorElement>('header a[data-copy-link]')!;
    expect(link().hidden).toBe(true);
    onPatch = (body) => Response.json((settings = { ...settings, ...body, visibility: 'unlisted' } as Settings));
    type('title', 'a');
    await save();
    expect(link().hidden).toBe(false);
    link().click();
    await settle();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(window.location.origin + '/w/' + ID);
    onPatch = (body) => Response.json((settings = { ...settings, ...body, visibility: 'draft' } as Settings));
    type('title', 'b');
    await save();
    expect(link().hidden).toBe(true);
  });

  it('AC 4: hiding is off on a new upload; no download toggle (F-24 discarded); visibility is "Draft" with no control', async () => {
    await open();
    expect([field('hide_start_m').value, field('hide_end_m').value]).toEqual(['0', '0']);
    expect(field('allow_download')).toBeNull();
    expect(field('visibility')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain('Draft');
  });
});

describe('AC 2, E-6: saving', () => {
  it('F-45 ST-7: the suggestion is header, tab and placeholder while the title is empty; it follows the save response and is never sent', async () => {
    settings = fresh({ suggested_title: 'Morning ride near Köln' });
    onPatch = (body) => Response.json((settings = { ...settings, ...body, suggested_title: body.hide_start_m ? 'Morning ride near Bonn' : settings.suggested_title } as Settings));
    await open();
    expect($('h1')?.textContent).toBe('Morning ride near Köln');
    expect(document.title).toBe('Morning ride near Köln');
    expect(field('title').placeholder).toBe('Morning ride near Köln');
    expect(field('title').value).toBe('');
    hideAt(300, 300);
    flushSync();
    expect(field('title').placeholder).toBe('Morning ride near Köln'); // not from an unsaved toggle
    await save();
    expect(patches()).toEqual([{ ...ON }]);
    expect($('h1')?.textContent).toBe('Morning ride near Bonn');
    expect(document.title).toBe('Morning ride near Bonn');
    expect(field('title').placeholder).toBe('Morning ride near Bonn');
    type('title', 'Loop');
    await save();
    expect($('h1')?.textContent).toBe('Loop');
    type('title', '');
    await save();
    expect($('h1')?.textContent).toBe('Morning ride near Bonn');
    expect(field('title').value).toBe('');
    for (const p of patches()) expect(p).not.toHaveProperty('suggested_title');
  });

  it('sends only the changed field, and the header re-renders from the response', async () => {
    await open();
    type('title', 'Evening <b>loop</b>');
    expect($('.edit-status')?.textContent).toBe('');
    await save();
    expect(patches()).toEqual([{ title: 'Evening <b>loop</b>' }]);
    expect($('h1')?.textContent).toBe('Evening <b>loop</b>');
    expect($('h1 b')).toBeNull();
    expect(document.title).toBe('Evening <b>loop</b>');
    expect($('.edit-status')?.textContent).toBe('Saved');
    expect($('time[data-expires]')?.textContent).toBe('Expires in 29 days');
    type('description', 'x');
    expect($('.edit-status')?.textContent).toBe('');
    expectNoTokenOutsideTheFragment();
  });

  it('the panel refills from the stored settings (the server trims)', async () => {
    onPatch = (body) => {
      settings = { ...settings, ...body, title: String(body.title).trim() } as Settings;
      return Response.json(settings);
    };
    await open();
    type('title', '  Trimmed  ');
    await save();
    expect(field('title').value).toBe('Trimmed');
  });

  it('a pasted tab is never sent; the rest of the description stays (E-7)', async () => {
    await open();
    type('description', 'one\ttwo\nthree');
    await save();
    expect(patches()).toEqual([{ description: 'one two\nthree' }]);
    expect($('.desc')?.textContent).toBe('one two\nthree');
  });

  it('nothing changed: no request', async () => {
    await open();
    await save();
    expect(patches()).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('No changes');
  });

  it('one save at a time', async () => {
    let release: () => void = () => {};
    onPatch = (body) => {
      settings = { ...settings, ...body } as Settings;
      return new Promise<Response>((r) => (release = () => r(Response.json(settings)))) as unknown as Response;
    };
    await open();
    type('title', 'A');
    $<HTMLFormElement>('form.edit-panel')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect($<HTMLButtonElement>('.edit-save')?.disabled).toBe(true);
    $<HTMLFormElement>('form.edit-panel')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect(patches()).toHaveLength(1);
    release();
    await settle();
    expect($<HTMLButtonElement>('.edit-save')?.disabled).toBe(false);
  });

  it('400: the text of the error code (F-54 LG-6, P-14), the form keeps the input, the header the stored text', async () => {
    onPatch = () => Response.json({ error: 'invalid_settings', message: 'The title is too long.' }, { status: 400 });
    await open();
    const before = $('h1')?.textContent;
    type('title', 'Mine');
    await save();
    expect($('.edit-status')?.textContent).toBe(errorTexts.invalid_settings);
    expect(field('title').value).toBe('Mine');
    expect($('h1')?.textContent).toBe(before);
  });

  it('F-20 C-11: a published workout has no visibility control; no request carries visibility', async () => {
    settings = fresh({ visibility: 'unlisted' });
    await open();
    expect(field('visibility')).toBeNull();
    expect($('.edit-panel select:not([name="lifetime"])')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain('Unlisted');
    expect($('.edit-panel')?.textContent).not.toMatch(/private/i);
    type('title', 'x');
    await save();
    expect(patches()).toEqual([{ title: 'x' }]);
  });

});

describe('AC 3: lifetime', () => {
  it('offers 1 to 30 days from upload, only after now, and shows the resulting expiry', async () => {
    settings = fresh({ created_at: '2026-10-19T11:00:00Z', expires_at: '2026-11-18T11:00:00Z' });
    await open();
    const days = [...field('lifetime').options].map((o) => Number(o.value));
    expect(days).toEqual(Array.from({ length: 20 }, (_, i) => i + 11));
    expect(field('lifetime').value).toBe('30');
    expect($('.edit-expiry')?.textContent).toMatch(/^Expires 18 Nov 2026, \d\d:00$/);
    field('lifetime').value = '12';
    field('lifetime').dispatchEvent(new Event('change', { bubbles: true }));
    await settle();
    expect($('.edit-expiry')?.textContent).toMatch(/^Expires 31 Oct 2026, \d\d:00$/);
    await save();
    expect(patches()).toEqual([{ lifetime_days: 12 }]);
  });
});

describe('AC 5: the incomplete-file notice', () => {
  it('a truncated file shows the time of the last intact record', async () => {
    draft = () => new Response(golden('ride-30min-cut-60').slice(0), { status: 200 });
    settings = fresh({ data_end_s: 1800, hiding_steps: null });
    await open();
    expect($('.edit-notice')?.textContent).toBe('The file was incomplete; data after 00:30:00 is missing.');
  });

  it('a complete file shows none', async () => {
    await open();
    expect($('.edit-notice')).toBeNull();
  });
});

describe('AC 6, E-4: not found and errors', () => {
  it.each([
    ['#' + TOKEN.slice(0, 21) + 'x0', '/edit/' + ID],
    ['#' + TOKEN.slice(1), '/edit/' + ID],
    ['', '/edit/' + ID],
    ['#' + TOKEN, '/edit/' + ID.slice(2)],
  ])('a malformed link (%s on %s): the generic page and no request', async (hash, path) => {
    await open(hash, path);
    expect(calls).toEqual([]);
    expect(document.body.textContent).toContain(GENERIC);
    expect($('main.generic h1')?.textContent).toBe('Workout Explorer');
    expect($('footer a[href="/about"]')).not.toBeNull();
  });

  it.each(['settings', 'draft.bin'])('a 404 on %s: the generic page', async (which) => {
    if (which === 'settings') onGet = () => new Response('{}', { status: 404 });
    else draft = () => new Response('', { status: 404 });
    await open();
    expect(document.body.textContent).toContain(GENERIC);
    expect($('form.edit-panel')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it('a 404 on save: the generic page', async () => {
    onPatch = () => new Response('', { status: 404 });
    await open();
    type('title', 'x');
    await save();
    expect(document.body.textContent).toContain(GENERIC);
    expect(document.title).toBe('Workout Explorer');
  });

  it('a 500 on settings: the server message, never the generic page', async () => {
    onGet = () => Response.json({ error: 'server_error', message: 'Something went wrong on the server. Please try again later.' }, { status: 500 });
    await open();
    expect(document.body.textContent).toContain('Something went wrong on the server.');
    expect(document.body.textContent).not.toContain(GENERIC);
    expectNoTokenOutsideTheFragment();
  });

  it('a 500 on save: the message', async () => {
    onPatch = () => new Response('oops', { status: 500 });
    await open();
    type('title', 'x');
    await save();
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expectNoTokenOutsideTheFragment();
  });

  it('draft.bin unreadable: the panel still works, D-13 text, no islands', async () => {
    draft = () => new Response(new Uint8Array([1, 2, 3, 4]), { status: 200 });
    await open();
    expect($('.load-error')?.textContent).toBe('The workout data could not be loaded.');
    expect(stubLog.calls).toEqual([]);
    type('title', 'Still');
    await save();
    expect(patches()).toEqual([{ title: 'Still' }]);
    expectNoTokenOutsideTheFragment();
  });
});

describe('E-9, AC 8; F-55 HR-8: to-be-hidden marks from the sliders', () => {
  const shaded = (): number[][] => {
    const over = document.createElement('div');
    (stubLog.chartStack[0].decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
    return [...over.querySelectorAll<HTMLElement>('.wx-trim > div')].map((d) => [parseFloat(d.style.left), parseFloat(d.style.left) + parseFloat(d.style.width)]);
  };
  const label = (name: string) => field(name).closest('label')!.textContent!.trim();
  beforeEach(() => localStorage.setItem('wx.units', 'metric')); // jsdom's en-US would default to imperial

  it('map and charts follow the unsaved slider values through hiding_steps, once per frame; saved, they stay', async () => {
    await open();
    const map = stubLog.routeMap[0];
    expect(map.hidden).toEqual([]);
    expect(shaded()).toEqual([]);
    // A drag: the value is in the label at once, the shading waits for the frame.
    field('hide_start_m').value = '500';
    field('hide_start_m').dispatchEvent(new Event('input', { bubbles: true }));
    field('hide_start_m').value = '300';
    field('hide_start_m').dispatchEvent(new Event('input', { bubbles: true }));
    flushSync();
    expect(label('hide_start_m')).toBe('Start: hide 300 m');
    expect(frames).toHaveLength(1); // one frame for two moves
    expect(map.hidden).toEqual([]);
    frames.splice(0).forEach((f) => f(0));
    flushSync();
    expect(map.hidden).toEqual([RANGES[0]]); // start only: 300 m → steps.start[6]
    expect(shaded()).toEqual([RANGES[0]]);
    hideAt(0, 500);
    expect(map.hidden).toEqual([[STEPS.end[10] + 1, N - 1]]); // end only: 500 m → steps.end[10]
    expect(label('hide_start_m')).toBe('Start: not hidden');
    expect(label('hide_end_m')).toBe('End: hide 500 m');
    hideAt(300, 300);
    expect(map.hidden).toEqual(RANGES);
    expect(shaded()).toEqual(RANGES);
    await save();
    expect(patches()).toEqual([{ ...ON }]);
    expect(map.hidden).toEqual(RANGES);
    expect(stubLog.routeMap).toHaveLength(1); // never remounted
  });

  it('sliders: 0 to 2,000 m in 50 m steps, labelled, filled from the saved radii; only changed radii sent', async () => {
    settings = fresh({ hide_start_m: 2000, hide_end_m: 150 });
    await open();
    for (const name of ['hide_start_m', 'hide_end_m']) expect([field(name).type, field(name).min, field(name).max, field(name).step]).toEqual(['range', '0', '2000', '50']);
    expect([field('hide_start_m').value, field('hide_end_m').value]).toEqual(['2000', '150']);
    expect(label('hide_start_m')).toBe('Start: hide 2000 m');
    expect(field('hide_end_m').getAttribute('aria-valuetext')).toBe('End: hide 150 m');
    hideAt(2000, 0);
    await save();
    expect(patches()).toEqual([{ hide_end_m: 0 }]);
    expect(Object.keys(localStorage).filter((k) => k !== 'wx.units' && k !== LIST_KEY)).toEqual([]); // OQ-248; F-53's list
  });

  it('OQ-268: imperial labels; the value sent stays metric', async () => {
    localStorage.setItem('wx.units', 'imperial');
    await open();
    hideAt(50, 150);
    expect([label('hide_start_m'), label('hide_end_m')]).toEqual(['Start: hide 160 ft', 'End: hide 490 ft']);
    hideAt(500, 2000);
    expect([label('hide_start_m'), label('hide_end_m')]).toEqual(['Start: hide 0.31 mi', 'End: hide 1.24 mi']);
    await save();
    expect(patches()).toEqual([{ hide_start_m: 500, hide_end_m: 2000 }]);
  });

  it('hiding_steps null (not yet regenerated): nothing shaded, no error', async () => {
    settings = fresh({ hiding_steps: null });
    await open();
    hideAt(300, 300);
    expect(stubLog.routeMap[0].hidden).toEqual([]);
    expect(shaded()).toEqual([]);
    expect(consoleOut).toEqual([]);
  });

  it('a fully hidden route marks the whole track', async () => {
    settings = fresh({ ...ON, hiding_steps: { start: STEPS.start, end: STEPS.end.map(() => -1) } });
    await open();
    expect(stubLog.routeMap[0].hidden).toEqual([[0, N - 1]]);
  });

  it('indoor: one column, no map, the toggle has no effect and no error', async () => {
    draft = () => new Response(golden('indoor-no-gps').slice(0), { status: 200 });
    settings = fresh({ hiding_steps: null });
    await open();
    expect($('.page')?.classList.contains('has-map')).toBe(false);
    expect($('#map-slot')).toBeNull();
    expect($('.hint')?.textContent).toBe('No location data in this workout.');
    hideAt(300, 300);
    await save();
    expect(patches()).toEqual([{ ...ON }]);
    expect(stubLog.routeMap).toEqual([]);
    expect(consoleOut.filter((a) => a.some((x) => x instanceof Error))).toEqual([]);
    expectNoTokenOutsideTheFragment();
  });
});

const FIELDS = ['title', 'description', 'hide_start_m', 'hide_end_m', 'lifetime'];
const btn = (act: string) => $<HTMLButtonElement>(`[data-act="${act}"]`);
async function click(act: string): Promise<void> {
  btn(act)!.click();
  await settle();
}
const methods = (): string[] => calls.filter((c) => c.init.method).map((c) => c.init.method + ' ' + c.url.replace('/api/workouts/' + ID, ''));
const DRAFT_LINE = 'Unpublished drafts are deleted 48 hours after upload.';
const WARNING = 'Save this link. It is the only way to change or delete this workout, and it cannot be recovered.';
const SH10 = 'Copies that viewers have already downloaded, and previews that messengers have already made, cannot be recalled.';
const clipboard = (write: (t: string) => Promise<void> = async () => {}) => {
  const writeText = vi.fn(write);
  vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
  return writeText;
};

describe('F-20 C-10, C-11, AC 1, 2: a draft', () => {
  it('no link anywhere, no Share, no header Copy link; the draft line and SH-10 shown', async () => {
    await open();
    expect(btn('publish')).not.toBeNull();
    expect(btn('share')).toBeNull();
    expect(btn('links')).toBeNull();
    expect($('.edit-share, .edit-warn, main.published')).toBeNull();
    expect($<HTMLAnchorElement>('header a[data-copy-link]')!.hidden).toBe(true);
    expect(document.body.textContent).not.toContain('/w/' + ID);
    expect($('.edit-panel')?.textContent).toContain(DRAFT_LINE);
    expect($('.edit-panel')?.textContent).toContain(SH10);
    expectNoTokenOutsideTheFragment();
  });

  it('published: no draft line, no Publish; SH-10 still shown', async () => {
    settings = fresh({ visibility: 'unlisted' });
    await open();
    expect($('.edit-panel')?.textContent).not.toContain(DRAFT_LINE);
    expect(btn('publish')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain(SH10);
  });
});

describe('F-20 C-2, C-3, AC 3, 4: publish', () => {
  it('without changes: only the POST, then assign to the published page; busy until unload, nothing re-rendered', async () => {
    let release: () => void = () => {};
    onPublish = () =>
      new Promise<Response>((r) => (release = () => r(Response.json((settings = { ...settings, visibility: 'unlisted' }))))) as unknown as Response;
    await open();
    const map = stubLog.routeMap[0];
    await click('publish');
    expect(methods()).toEqual(['POST /publish']);
    expect(navs).toEqual([]); // never before the 200
    release();
    await settle();
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    expect(stubLog.routeMap).toEqual([map]);
    expect(btn('share')).toBeNull(); // not re-rendered as published
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    for (const b of [$<HTMLButtonElement>('.edit-save'), btn('publish'), btn('delete'), btn('download')]) expect(b?.disabled).toBe(true);
    expectNoTokenOutsideTheFragment();
  });

  it('OQ-85: unsaved changes are saved first (PATCH before POST), then published', async () => {
    await open();
    hideAt(300, 300);
    type('title', 'Loop');
    await click('publish');
    expect(methods()).toEqual(['PATCH ', 'POST /publish']);
    expect(patches()).toEqual([{ title: 'Loop', ...ON }]);
    expect(navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('OQ-85: a failed save stops the publish and stays', async () => {
    onPatch = () => Response.json({ message: 'The title is too long.' }, { status: 400 });
    await open();
    type('title', 'x');
    await click('publish');
    expect(methods()).toEqual(['PATCH ']);
    expect($('.edit-status')?.textContent).toBe('The title is too long.');
    expect(btn('publish')?.disabled).toBe(false);
    expect(navs).toEqual([]);
  });

  it('a 500 on publish: the message, Publish still offered, no navigation; a retry works', async () => {
    onPublish = () => Response.json({ message: 'Something went wrong on the server. Please try again later.' }, { status: 500 });
    await open();
    await click('publish');
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(btn('publish')?.disabled).toBe(false);
    expect(navs).toEqual([]);
    onPublish = () => Response.json((settings = { ...settings, visibility: 'unlisted' }));
    await click('publish');
    expect(navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('a 404 on publish: the generic page', async () => {
    onPublish = () => new Response('', { status: 404 });
    await open();
    await click('publish');
    expect(document.body.textContent).toContain(GENERIC);
    expect(document.title).toBe('Workout Explorer');
    expect(navs).toEqual([]);
  });

  it('C-2: one action at a time; Save, Publish, Delete and Download are disabled while one runs', async () => {
    let release: () => void = () => {};
    onPublish = () => new Promise<Response>((r) => (release = () => r(new Response('', { status: 500 })))) as unknown as Response;
    await open();
    btn('publish')!.click();
    await settle();
    for (const b of [$<HTMLButtonElement>('.edit-save'), btn('publish'), btn('delete'), btn('download')]) expect(b?.disabled).toBe(true);
    // The fields too: Publish read the form at the click, so a change now would be dropped (QA-20-r2-1).
    for (const f of FIELDS) expect(field(f).matches(':disabled'), f).toBe(true);
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    type('title', 'x');
    await save();
    btn('publish')!.dispatchEvent(new MouseEvent('click', { bubbles: true })); // a disabled button's click, forced
    await settle();
    expect(methods()).toEqual(['POST /publish']);
    release();
    await settle();
    expect($<HTMLButtonElement>('.edit-save')?.disabled).toBe(false);
    for (const f of FIELDS) expect(field(f).matches(':disabled'), f).toBe(false);
  });

  it('C-2: Show links and the edit link\'s Go there are disabled while an action runs (QA-20-r2-3)', async () => {
    let release: () => void = () => {};
    original = () => new Promise<Response>((r) => (release = () => r(new Response('', { status: 500 })))) as unknown as Response;
    settings = fresh({ visibility: 'unlisted' });
    await open();
    await click('share');
    await click('download');
    expect(btn('links')?.disabled).toBe(true);
    release();
    await settle();
    expect(btn('links')?.disabled).toBe(false);
    onDelete = () => new Promise<Response>((r) => (release = () => r(new Response('', { status: 500 })))) as unknown as Response;
    host = document.createElement('div');
    document.body.replaceChildren(host);
    await openPublished();
    await click('delete');
    await click('confirm');
    expect(btn('back')?.disabled).toBe(true);
    release();
    await settle();
    expect(btn('back')?.disabled).toBe(false);
  });
});

describe('F-36 MC-8, OQ-112: the preview basemap job', () => {
  const KEY = '0123456789abcdef';
  const FRAME = { lon: 11.5, lat: 48.1, zoom: 12.25 };
  const withFrame = (current = false) => (onFrame = () => Response.json({ frame: FRAME, key: KEY, current }));
  const frameGets = () => calls.filter((c) => !c.init.method && c.url.endsWith('/preview-map')).length;

  it('publish: GET the frame after the save, render it, PUT under its key, then the published page', async () => {
    withFrame();
    await open();
    hideAt(300, 300);
    await click('publish');
    expect(methods()).toEqual(['PATCH ', 'POST /publish', 'PUT /preview-map/' + KEY]);
    const order = calls.map((c) => c.url.replace('/api/workouts/' + ID, ''));
    expect(order.indexOf('/preview-map')).toBeGreaterThan(order.indexOf('')); // after the save's 200
    expect(renders.map(([f]) => f)).toEqual([FRAME]);
    const put = calls.find((c) => c.init.method === 'PUT')!;
    expect(put.init.body).toBeInstanceOf(Blob);
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    expectNoTokenOutsideTheFragment();
  });

  it('the published page opens only once the job ended', async () => {
    withFrame();
    let done: (b: Blob) => void = () => {};
    render = () => new Promise<Blob>((r) => (done = r));
    await open();
    await click('publish');
    expect(methods()).toEqual(['POST /publish']);
    expect(navs).toEqual([]);
    done(new Blob([new Uint8Array([1])]));
    await settle();
    expect(methods()).toEqual(['POST /publish', 'PUT /preview-map/' + KEY]);
    expect(navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('a render that never ends: the published page opens after 10 s, nothing sent', async () => {
    withFrame();
    render = () => new Promise<Blob>(() => {});
    await open();
    vi.useFakeTimers();
    try {
      btn('publish')!.click();
      await vi.advanceTimersByTimeAsync(9_999);
      expect(navs).toEqual([]);
      await vi.advanceTimersByTimeAsync(1);
      expect(navs).toEqual([PUBLISHED_PAGE()]);
      expect(renders[0][1].aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
    expect(methods()).toEqual(['POST /publish']);
  });

  it.each([
    ['no WebGL2', () => (render = () => Promise.reject(new Error('GPUInitializationError')))],
    ['a tile error', () => (render = () => Promise.reject(new Error('basemap: map error')))],
    ['the PUT refused', () => (onPut = () => Response.json({ error: 'invalid_image', message: 'x' }, { status: 400 }))],
    ['the PUT lost', () => (onPut = () => Promise.reject(new TypeError('network')) as unknown as Response)],
    ['the GET failed', () => (onFrame = () => new Response('', { status: 500 }))],
  ])('%s: publish succeeds, the published page opens, nothing shown', async (_name, fail) => {
    withFrame();
    fail();
    await open();
    await click('publish');
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    expect(consoleOut).toEqual([]);
  });

  it.each([
    ['no frame (indoor, fully hidden)', { frame: null }],
    ['a current map', { frame: FRAME, key: KEY, current: true }],
  ])('%s: nothing rendered, no PUT', async (_name, body) => {
    onFrame = () => Response.json(body);
    await open();
    await click('publish');
    expect(frameGets()).toBe(1);
    expect(renders).toEqual([]);
    expect(methods()).toEqual(['POST /publish']);
    expect(navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('a failed publish shows its message; the job does not open anything', async () => {
    withFrame();
    onPublish = () => Response.json({ message: 'Something broke.' }, { status: 500 });
    await open();
    await click('publish');
    await settle();
    expect(navs).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('Something broke.');
  });

  it('MC-8(b): a save that changes hiding on a published workout runs the job in the background', async () => {
    settings = fresh({ visibility: 'unlisted' });
    withFrame();
    await open();
    hideAt(300, 300);
    await save();
    expect($('.edit-status')?.textContent).toBe('Saved');
    expect(frameGets()).toBe(1);
    expect(methods()).toEqual(['PATCH ', 'PUT /preview-map/' + KEY]);
    expect(navs).toEqual([]);
    expectNoTokenOutsideTheFragment();
  });

  it('MC-8(b): no job for a title-only save', async () => {
    settings = fresh({ visibility: 'unlisted' });
    withFrame();
    await open();
    type('title', 'Loop');
    await save();
    expect(frameGets()).toBe(0);
    expect(renders).toEqual([]);
  });

  it('MC-8(b): no job for a save of a draft', async () => {
    withFrame();
    await open();
    hideAt(300, 300);
    await save();
    expect(frameGets()).toBe(0);
  });

  it('never on the published page', async () => {
    settings = fresh({ visibility: 'unlisted' });
    withFrame();
    await openPublished();
    await settle();
    expect(frameGets()).toBe(0);
  });
});

describe('F-20 C-4, OQ-86, AC 7, 8: Share on the edit page', () => {
  it('published: Share shows exactly origin + view_url; copy puts exactly that; Show links opens the published page', async () => {
    const writeText = clipboard();
    settings = fresh({ visibility: 'unlisted' });
    await open();
    expect($<HTMLAnchorElement>('header a[data-copy-link]')!.hidden).toBe(false);
    expect($('.edit-share')).toBeNull();
    await click('share');
    expect($('.edit-share .edit-link')?.textContent).toBe(VIEW_LINK());
    $<HTMLAnchorElement>('.edit-share a[data-copy-link]')!.click();
    await settle();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(VIEW_LINK());
    expectNoTokenOutsideTheFragment();
    await click('links');
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    await click('share');
    expect($('.edit-share')).toBeNull();
  });

  it('a refused clipboard on Share: D-14\'s read-only field with the viewing link only', async () => {
    clipboard(() => Promise.reject(new Error('denied')));
    settings = fresh({ visibility: 'unlisted' });
    await open();
    await click('share');
    $<HTMLAnchorElement>('.edit-share a[data-copy-link]')!.click();
    await settle();
    expect($<HTMLInputElement>('.edit-share input.copy-url')?.value).toBe(VIEW_LINK());
    expectNoTokenOutsideTheFragment();
  });
});

describe('F-20 C-2: back-forward cache, never a reload', () => {
  const pageshow = (persisted: boolean): void => {
    const e = new Event('pageshow');
    Object.defineProperty(e, 'persisted', { value: persisted });
    window.dispatchEvent(e);
  };
  const gets = () => calls.filter((c) => !c.init.method && c.url === '/api/workouts/' + ID).length;

  it('the published page: no reload, no request on a restore (an app resume on mobile WebKit)', async () => {
    settings = fresh({ visibility: 'unlisted' });
    await openPublished();
    const before = calls.length;
    pageshow(true);
    await settle();
    expect(navs).toEqual([]);
    expect(calls).toHaveLength(before);
    expect($('main.published')).not.toBeNull();
  });

  it('the edit page, still a draft: re-checks the settings on a restore only; unsaved values stay', async () => {
    await open();
    type('title', 'Typed, not saved');
    pageshow(false);
    await settle();
    expect(gets()).toBe(1);
    pageshow(true);
    await settle();
    expect(gets()).toBe(2);
    expect(navs).toEqual([]);
    expect(field('title').value).toBe('Typed, not saved');
    expect(btn('publish')).not.toBeNull();
  });

  it('the edit page, published meanwhile: shows as published, the form keeps the unsaved values', async () => {
    await open();
    type('title', 'Typed, not saved');
    settings = { ...settings, title: 'Stored elsewhere', visibility: 'unlisted' };
    pageshow(true);
    await settle();
    expect(navs).toEqual([]);
    expect(btn('publish')).toBeNull();
    expect(btn('share')).not.toBeNull();
    expect(field('title').value).toBe('Typed, not saved');
    expect(patches()).toEqual([]);
    expect(localStorage.getItem(LIST_KEY)).toContain('"published":true'); // QA-53-r1-3: the list follows
  });

  it('Back from the published page: the edit page shows as published and is no longer locked', async () => {
    await open();
    await click('publish');
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    pageshow(true);
    await settle();
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    expect(btn('publish')).toBeNull();
    expect($('.edit-status')?.textContent).toBe('');
    expect($<HTMLFieldSetElement>('form.edit-panel fieldset')!.disabled).toBe(false);
    expect($<HTMLButtonElement>('form.edit-panel button[type="submit"]')!.disabled).toBe(false);
    expectNoTokenOutsideTheFragment();
  });
});

describe('F-20 C-5, AC 5, 6: the published page', () => {
  beforeEach(() => {
    settings = fresh({ visibility: 'unlisted', title: 'Secret loop', description: 'Through the park' });
  });

  it('the settings only; both links, the warning, Go there in each box and Delete; no workout data', async () => {
    await openPublished();
    expect(calls.map((c) => c.url)).toEqual(['/api/workouts/' + ID]);
    expect(stubLog.calls).toEqual([]);
    expect(document.title).toBe('Workout Explorer');
    expect($('main.published h1')?.textContent).toBe('Workout published');
    expect($('main.published .edit-share .edit-link')?.textContent).toBe(VIEW_LINK());
    expect($('main.published [translate="no"]')?.textContent).toBe(EDIT_LINK());
    // Two boxes, blue then red: each with its header, its link and its Copy.
    const boxes = [...document.querySelectorAll('main.published .link-box')];
    expect(boxes.map((b) => b.querySelector('h2')?.textContent)).toEqual(['Viewing link', 'Edit link']);
    expect(boxes[0].classList.contains('view-box')).toBe(true);
    expect(boxes[0].querySelector('a[data-copy-link]')).not.toBeNull();
    const go = boxes[0].querySelector<HTMLAnchorElement>('a.go');
    expect([go?.textContent, go?.getAttribute('href')]).toEqual(['Go there', '/w/' + ID]);
    expect(boxes[1].classList.contains('edit-warn')).toBe(true);
    expect(boxes[1].querySelector('[translate="no"]')?.textContent).toBe(EDIT_LINK());
    expect(boxes[1].querySelector('[data-act="copy-edit"]')).not.toBeNull();
    expect(boxes[1].textContent).toContain(WARNING);
    expect($('main.published')?.textContent).toContain(WARNING);
    const back = boxes[1].querySelector('[data-act="back"]');
    expect([back?.tagName, back?.textContent]).toEqual(['BUTTON', 'Go there']);
    expect($('main.published')?.textContent).not.toContain('Back to edit');
    expect(btn('delete')).not.toBeNull();
    expect($('main.published .edit-delete-hint + .edit-row')?.previousElementSibling?.textContent).toBe('Uploaded accidentally?');
    expect($('footer a[href="/about"]')).not.toBeNull();
    const html = document.documentElement.outerHTML;
    for (const gone of ['Secret loop', 'Through the park']) expect(html).not.toContain(gone);
    expect($('.edit-panel, .summary, #map-slot, #charts-slot, table, header')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it('copy puts exactly each link on the clipboard', async () => {
    const writeText = clipboard();
    await openPublished();
    $<HTMLAnchorElement>('main.published a[data-copy-link]')!.click();
    await settle();
    await click('copy-edit');
    expect(writeText.mock.calls).toEqual([[VIEW_LINK()], [EDIT_LINK()]]);
    expect($('.edit-copied')?.textContent).toBe('Edit link copied.');
    expectNoTokenOutsideTheFragment();
  });

  it('a refused clipboard: select-by-hand text for the edit link, the viewing link in D-14\'s field', async () => {
    clipboard(() => Promise.reject(new Error('denied')));
    await openPublished();
    await click('copy-edit');
    expect($('.edit-copied')?.textContent).toBe('Copying did not work. Select the link above and copy it by hand.');
    $<HTMLAnchorElement>('main.published a[data-copy-link]')!.click();
    await settle();
    expect([...document.querySelectorAll<HTMLInputElement>('input')].map((i) => i.value)).toEqual([VIEW_LINK()]);
    expectNoTokenOutsideTheFragment();
  });

  it('the edit link\'s Go there: assign to the edit link', async () => {
    await openPublished();
    await click('back');
    expect(navs).toEqual(['assign ' + EDIT_LINK()]);
  });

  it('a reload shows it again', async () => {
    await openPublished();
    await openPublished();
    expect($('main.published [translate="no"]')?.textContent).toBe(EDIT_LINK());
  });

  it('a draft: replace to the edit page (Back does not bounce), nothing shown', async () => {
    settings = fresh();
    await openPublished();
    expect(navs).toEqual(['replace ' + EDIT_LINK()]);
    expect($('main.published')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it.each([
    ['#' + TOKEN.slice(0, 21) + 'x0', '/published/' + ID],
    ['', '/published/' + ID],
    ['#' + TOKEN, '/published/' + ID.slice(2)],
  ])('a malformed link (%s on %s): the generic page and no request', async (hash, path) => {
    await open(hash, path);
    expect(calls).toEqual([]);
    expect(document.body.textContent).toContain(GENERIC);
  });

  it('a 404: the generic page; a 500: the message', async () => {
    onGet = () => new Response('', { status: 404 });
    await openPublished();
    expect(document.body.textContent).toContain(GENERIC);
    expectNoTokenOutsideTheFragment();
    host = document.createElement('div');
    document.body.replaceChildren(host);
    onGet = () => new Response('', { status: 500 });
    await openPublished();
    expect(document.body.textContent).toContain('Something went wrong on the server.');
    expect(document.body.textContent).not.toContain(GENERIC);
    expect($('main.published')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });
});

describe.each([
  ['edit', () => open()],
  ['published', () => openPublished()],
])('F-20 C-7, C-8, AC 9: delete on the %s page', (_, go) => {
  beforeEach(() => {
    settings = fresh({ visibility: 'unlisted', title: 'Secret loop', description: 'Through the park' });
  });

  it('Delete only asks: the confirmation is announced and Cancel has the focus; Cancel sends nothing', async () => {
    await go();
    await click('delete');
    expect(methods()).toEqual([]);
    expect($('.edit-confirm')?.getAttribute('role')).toBe('alert');
    expect($('.edit-confirm')?.textContent).toContain('permanently');
    expect($('.edit-confirm')?.textContent).toContain('Both the viewing link and the edit link stop working');
    expect(document.activeElement).toBe(btn('cancel'));
    await click('cancel');
    expect($('.edit-confirm')).toBeNull();
    expect(methods()).toEqual([]);
    expect(btn('delete')).not.toBeNull();
  });

  it('confirm: DELETE, then the deleted screen with no workout data', async () => {
    await go();
    await click('delete');
    await click('confirm');
    expect(methods()).toEqual(['DELETE ']);
    expect(document.title).toBe('Workout Explorer');
    expect($('main.generic p')?.textContent).toBe('This workout has been deleted.');
    expect($<HTMLAnchorElement>('main.generic a')?.getAttribute('href')).toBe('/');
    expect($('footer a[href="/about"]')).not.toBeNull();
    const html = document.documentElement.outerHTML;
    for (const gone of [ID, TOKEN, 'Secret loop', 'Through the park', '/w/']) expect(html).not.toContain(gone);
    expect(document.body.textContent).not.toMatch(/\d/);
    expect($('.edit-panel, main.published, .summary, #map-slot, #charts-slot, table')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it('a 404 on delete: the generic page', async () => {
    onDelete = () => new Response('', { status: 404 });
    await go();
    await click('delete');
    await click('confirm');
    expect(document.body.textContent).toContain(GENERIC);
  });

  it('a 500 on delete: the message; the page stays and a retry is possible', async () => {
    onDelete = () => new Response('', { status: 500 });
    await go();
    await click('delete');
    await click('confirm');
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(document.body.textContent).not.toContain(GENERIC);
    expect(btn('confirm')?.disabled).toBe(false);
  });
});

describe('F-20 C-6, AC 10: download original', () => {
  const BLOB = 'blob:' + window.location.origin + '/0d1e';
  let anchors: HTMLAnchorElement[];
  let blobs: Blob[];
  beforeEach(() => {
    anchors = [];
    blobs = [];
    Object.assign(URL, {
      createObjectURL: vi.fn((b: Blob) => (blobs.push(b), BLOB)),
      revokeObjectURL: vi.fn(),
    });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      anchors.push(this.cloneNode() as HTMLAnchorElement);
      expect(this.isConnected).toBe(true);
    });
  });

  it('offered draft and published; the bytes as served, the server\'s name, exactly the blob: URL', async () => {
    for (const visibility of ['unlisted', 'draft'] as const) {
      settings = fresh({ visibility });
      await open();
      expect(btn('download'), visibility).not.toBeNull();
    }
    const timers = vi.spyOn(globalThis, 'setTimeout');
    await click('download');
    const revoke = timers.mock.calls.find((c) => c[1] === 40_000);
    expect(URL.revokeObjectURL).not.toHaveBeenCalled(); // not at once: Safari would drop the file
    (revoke![0] as () => void)();
    expect(calls.at(-1)?.url).toBe('/api/workouts/' + ID + '/original.fit');
    expect(anchors).toHaveLength(1);
    expect(anchors[0].getAttribute('href')).toBe(BLOB); // nothing added, so no token (QA-20-r1-2 b)
    expect(anchors[0].download).toBe('workout-2026-10-29.fit');
    expect(new Uint8Array(await blobs[0].arrayBuffer())).toEqual(FIT);
    expect($('a[download]')).toBeNull(); // removed again
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(BLOB);
    expectNoTokenOutsideTheFragment();
  });

  it('C-2: a running download disables Save, Publish and Delete (QA-20-r1-2 c)', async () => {
    let release: () => void = () => {};
    original = () => new Promise<Response>((r) => (release = () => r(new Response(FIT)))) as unknown as Response;
    await open();
    await click('download');
    for (const b of [$<HTMLButtonElement>('.edit-save'), btn('publish'), btn('delete'), btn('download')]) expect(b?.disabled).toBe(true);
    expect($('.edit-status')?.textContent).toBe('Downloading…');
    btn('publish')!.dispatchEvent(new MouseEvent('click', { bubbles: true })); // forced
    await settle();
    expect(methods()).toEqual([]);
    release();
    await settle();
    expect(btn('publish')?.disabled).toBe(false);
    expect(anchors[0].getAttribute('href')).toBe(BLOB);
  });

  it('no Content-Disposition: workout.fit', async () => {
    original = () => new Response(FIT);
    await open();
    await click('download');
    expect(anchors[0].download).toBe('workout.fit');
  });

  it('404: the generic page; 500: the message', async () => {
    original = () => new Response('', { status: 500 });
    await open();
    await click('download');
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(anchors).toEqual([]);
    original = () => new Response('', { status: 404 });
    await click('download');
    expect(document.body.textContent).toContain(GENERIC);
  });
});

describe('F-38 IM-7: the Impressum link in the browser-drawn footers', () => {
  const LINK = '<a href="/impressum">Impressum</a>';
  const withTemplate = (filled: string): void => {
    const t = document.createElement('template');
    t.id = 'wx-impressum';
    t.innerHTML = filled;
    document.body.prepend(t);
  };
  const links = (): string[] => [...document.querySelectorAll('footer a')].map((a) => a.getAttribute('href')!);

  for (const [name, filled, want] of [
    ['set', LINK, ['/about', '/impressum']],
    ['unset', '', ['/about']],
    ['the unfilled dev marker', '<!--wx:impressum-->', ['/about']],
  ] as const) {
    describe(name, () => {
      beforeEach(() => withTemplate(filled));

      it('the page footer', async () => {
        await open();
        expect(links()).toEqual(want);
        expect(document.querySelectorAll('footer')).toHaveLength(1);
      });

      it('the published view', async () => {
        settings = fresh({ visibility: 'unlisted' });
        await openPublished();
        expect($('main.published')).not.toBeNull();
        expect(links()).toEqual(want);
      });

      it('the not-found view, also when drawn twice (delete, then the deleted screen)', async () => {
        await open('');
        expect($('main.generic')).not.toBeNull();
        expect(links()).toEqual(want);
        settings = fresh({ visibility: 'unlisted' });
        host = document.createElement('div');
        document.body.append(host);
        await openPublished();
        await click('delete');
        await click('confirm');
        expect($('main.generic p')?.textContent).toBe('This workout has been deleted.');
        expect(links()).toEqual(want);
      });
    });
  }

  it('the edit.html template holds exactly the marker, before #wx-edit', () => {
    expect(editHtml).toContain('<template id="wx-impressum"><!--wx:impressum--></template>\n    <div id="wx-edit"></div>');
  });
});

describe('the token stays in lib/owner (E-2 source scan)', () => {
  const sources = import.meta.glob(['../**/*.ts', '../**/*.svelte', '../../*.html', '!../**/*.test.ts', '!../**/testing/**', '!../**/*.test-helper.*'], {
    query: '?raw',
    import: 'default',
    eager: true,
  }) as Record<string, string>;
  const code = (text: string): string => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1');

  it('only lib/owner reads the fragment, builds Authorization, changes history or navigates (F-20 C-1)', () => {
    expect(Object.keys(sources).length).toBeGreaterThan(40);
    for (const [file, text] of Object.entries(sources)) {
      if (file === '../lib/owner/index.ts') continue;
      // The upload page turns the edit link the API returns into the address it opens (F-11 U-9):
      // the token's way into the fragment, before any edit page code runs.
      if (file === '../lib/upload/rules.ts') continue;
      // F-37 DK-11: the delete link's page reads its own key and sends it as Bearer; it never touches history.
      if (file.startsWith('../report-delete/')) {
        expect(code(text), file).not.toMatch(/replaceState|pushState|location\.(assign|replace|href)|console\.|localStorage|sessionStorage|innerHTML/);
        continue;
      }
      // F-26 (ADR-038 item 3): the fullscreen map's one history entry, with no URL argument.
      let c = file === '../components/map/map-view.ts' ? code(text).replace("history.pushState(null, '')", '') : code(text);
      // F-40 SR-4, SR-5: the viewing page (no edit seam) reads its range link once and drops it
      // without a history entry; the edit seam reads nothing.
      if (file === '../page/start.ts') c = c.replace("edit ? '' : win.location.hash", '').replace("win.history.replaceState(win.history.state, '', win.location.pathname + win.location.search)", '');
      expect(c, file).not.toMatch(/\.hash\b|Authorization|Bearer|replaceState|pushState|location\.href/);
      // The upload page opens the edit link it checked (F-11 U-9).
      if (file !== '../upload/main.ts') expect(c, file).not.toMatch(/location\.(assign|replace)\b/);
    }
    const owner = sources['../lib/owner/index.ts'];
    expect(owner).toContain("Authorization: 'Bearer ' + token");
    expect(owner).toContain("redirect: 'error'");
    expect(code(owner)).not.toMatch(/console\.|localStorage|sessionStorage|indexedDB|cookie|caches\.|location\.href|innerHTML|textContent/);
    expect(owner).not.toMatch(/^import .* from '\./m); // imports nothing of ours
  });

  it('the edit page has the head of the template, no inline script but the theme script, no inline style, no data preload', () => {
    const edit = sources['../../edit.html'].replace(/<!--[\s\S]*?-->/g, '');
    expect(edit).toContain('<meta name="referrer" content="no-referrer"');
    expect(edit).toContain('<meta name="robots" content="noindex, nofollow, noarchive"');
    expect(edit).toContain('<script type="module" src="/src/main.ts"></script>');
    expect(edit).toContain('<div id="wx-edit"></div>');
    expect(edit).not.toMatch(/<style[\s>]|\sstyle\s*=|\son[a-z]+\s*=|preload/i);
    expect(edit.match(/<script(?![^>]*\ssrc=)[^>]*>/gi)).toEqual(['<script>']); // DS-4, theme-script.test.ts
    const sprite = (html: string): string => html.slice(html.indexOf('<svg class="sprite"'), html.indexOf('</defs></svg>'));
    expect(sprite(edit)).toBe(sprite(sources['../../index.html'])); // the copy of the template's icons
    expect(sprite(edit)).toContain('sport-generic');
  });
});

describe('F-47 TR-9: the trim slider (AC 31, 32, 41)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const T = decoded.data.time;
  const S = Date.parse(decoded.data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((S + T[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const drag = (name: 'trim_a' | 'trim_b', i: number): void => type(name, String(i));
  const handles = (): [number, number] => [Number(field('trim_a').value), Number(field('trim_b').value)];
  /** What the charts' decoration draws into a fresh overlay: [left, width] per shaded part. */
  const shaded = (): number[][] => {
    const over = document.createElement('div');
    (stubLog.chartStack[0].decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
    return [...over.querySelectorAll<HTMLElement>('.wx-trim > div')].map((d) => [parseFloat(d.style.left), parseFloat(d.style.width)]);
  };
  const keys = (): string[] => [...document.querySelectorAll('.summary > .metric')].map((m) => m.getAttribute('data-k')!);

  it('the handles shade the charts and grey the map live; Save sends sample instants, null at the ends', async () => {
    await open();
    expect(handles()).toEqual([0, N - 1]);
    expect(shaded()).toEqual([]);
    drag('trim_a', 100);
    drag('trim_b', N - 200);
    expect(shaded()).toEqual([
      [0, 100],
      [N - 200, 199],
    ]);
    expect(stubLog.routeMap[0].hidden).toEqual([
      [0, 100],
      [N - 200, N - 1],
    ]);
    expect($('.edit-panel')?.textContent).toContain(formatElapsed(T[100] - T[0]) + ' – ' + formatElapsed(T[N - 200] - T[0])); // elapsed from the recording start
    await save();
    expect(patches()).toEqual([{ trim_start_at: at(100), trim_end_at: at(N - 200) }]);
    expect(handles()).toEqual([100, N - 200]); // refilled from the response
    drag('trim_a', 0);
    drag('trim_b', N - 1);
    expect(stubLog.routeMap[0].hidden).toEqual([]);
    await save();
    expect(patches()[1]).toEqual({ trim_start_at: null, trim_end_at: null });
    expectNoTokenOutsideTheFragment();
  });

  it('with hiding on, the trim marks come first, then the stored hidden ranges', async () => {
    settings = fresh({ ...ON });
    await open();
    drag('trim_b', N - 500);
    expect(stubLog.routeMap[0].hidden).toEqual([[N - 500, N - 1], ...RANGES]);
  });

  it('QA-55-r1-3: an unsaved trim with hiding on says the shading waits for Save', async () => {
    settings = fresh({ ...ON, trim_start_at: at(100), trim_end_at: at(N - 300) });
    await open();
    const note = () => $('.edit-stale')?.textContent;
    expect(note()).toBeUndefined();
    drag('trim_a', 0);
    expect(note()).toBe('Save to update the hidden sections.');
    hideAt(0, 0);
    expect(note()).toBeUndefined(); // nothing hidden, nothing stale
    hideAt(300, 0);
    expect(note()).toBe('Save to update the hidden sections.');
    drag('trim_a', 100);
    expect(note()).toBeUndefined(); // back on the saved trim
  });

  it('at least 60 s remain (AC 41): the moved handle is held back', async () => {
    await open();
    drag('trim_a', 1000);
    drag('trim_b', 1000);
    const [a, b] = handles();
    expect(a).toBe(1000);
    expect(T[b] - T[a]).toBeGreaterThanOrEqual(60);
    expect(T[b - 1] - T[a]).toBeLessThan(60);
  });

  it('F-51 AC 8: with a stored trim the comparison chart shows the same full rows as the edit page table', async () => {
    settings = fresh({ trim_start_at: at(50), trim_end_at: at(N - 300), trimmed: { sport: 'cycling', summary: { distance: 12_345, elapsed_time: 600 } } });
    await open();
    stubLog.chartStack[0].onready?.(); // the charts' first paint starts the lazy chunks
    await vi.waitFor(() => expect($('section.laps .compare rect.bar')).not.toBeNull());
    const rows = [...host.querySelectorAll('section.laps .laps-scroll:not(.splits) tbody tr')];
    expect(rows.length).toBeGreaterThan(1);
    expect(host.querySelectorAll('section.laps .compare rect.bar')).toHaveLength(rows.length);
  });

  it('a stored trim: handles from it, only the summary trimmed, in the chosen units; header and laps stay full', async () => {
    const full = fresh();
    settings = full;
    await open();
    const head = $('h1')?.textContent;
    const laps = $('section.laps')?.textContent;
    const fullKeys = keys();
    document.body.replaceChildren(host);
    host.replaceChildren();
    resetStubLog();
    settings = fresh({ trim_start_at: at(50), trim_end_at: at(N - 300), trimmed: { sport: 'cycling', summary: { distance: 12_345, elapsed_time: 600 } } });
    await open();
    expect(handles()).toEqual([50, N - 300]);
    expect(stubLog.routeMap[0].hidden).toEqual([
      [0, 50],
      [N - 300, N - 1],
    ]);
    expect(keys()).toEqual(['distance', 'elapsed_time']);
    expect(fullKeys).not.toEqual(keys());
    expect($('h1')?.textContent).toBe(head);
    expect($('section.laps')?.textContent).toBe(laps);
    const units = $<HTMLSelectElement>('#units-slot select')!;
    for (const [u, n, unit] of [['imperial', '7.67', 'mi'], ['metric', '12.35', 'km'], ['imperial', '7.67', 'mi']]) {
      units.value = u;
      units.dispatchEvent(new Event('change', { bubbles: true }));
      flushSync();
      expect($('.summary [data-k="distance"] .n')?.textContent).toBe(n);
      expect($('.summary [data-k="distance"] .u')?.textContent).toBe(unit);
      expect($('section.laps thead')?.textContent).toContain('Distance (' + unit + ')'); // the laps still follow the units
    }
    expect(keys()).toEqual(['distance', 'elapsed_time']);
  });

  it('a stored trim whose trimmed values failed (null): no summary values, never the full totals', async () => {
    settings = fresh({ trim_end_at: at(N - 300), trimmed: null });
    await open();
    expect(keys()).toEqual([]);
  });

  it('the save response replaces the strip: trimmed values after a trim save, full ones after removing it', async () => {
    await open();
    const fullKeys = keys();
    onPatch = (body) => Response.json((settings = { ...settings, ...body, trimmed: body.trim_start_at ? { sport: 'cycling', summary: { elapsed_time: 900 } } : null } as Settings));
    drag('trim_a', 300);
    await save();
    expect(keys()).toEqual(['elapsed_time']);
    drag('trim_a', 0);
    await save();
    expect(keys()).toEqual(fullKeys);
  });

  it('multisport: the leg lines and the trim shading are both drawn', async () => {
    draft = () => new Response(golden('triathlon').slice(0), { status: 200 });
    settings = fresh({ hiding_steps: null });
    await open();
    const charts = stubLog.chartStack[0];
    charts.onready?.();
    await vi.waitFor(() => {
      const over = document.createElement('div');
      (charts.decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
      expect(over.querySelector('.wx-legs')).not.toBeNull();
    });
    drag('trim_a', 10);
    const over = document.createElement('div');
    (charts.decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
    expect([!!over.querySelector('.wx-legs'), over.querySelectorAll('.wx-trim > div').length]).toEqual([true, 1]);
  });

  it('indoor: the slider works without a map; no error', async () => {
    draft = () => new Response(golden('indoor-no-gps').slice(0), { status: 200 });
    settings = fresh({ hiding_steps: null });
    await open();
    drag('trim_a', 50);
    await save();
    expect(Object.keys(patches()[0])).toEqual(['trim_start_at']);
    expect(stubLog.routeMap).toEqual([]);
    expect(consoleOut.filter((a) => a.some((x) => x instanceof Error))).toEqual([]);
  });

  it('an undecodable draft: no slider, the stored trim goes back unchanged', async () => {
    draft = () => new Response(new Uint8Array([1, 2, 3]), { status: 200 });
    settings = fresh({ trim_start_at: '2026-10-29T10:00:00Z' });
    await open();
    expect(field('trim_a')).toBeNull();
    type('title', 'x');
    await save();
    expect(patches()).toEqual([{ title: 'x' }]);
  });

  it('MC-8(b): a trim save on a published workout runs the basemap job; not on a draft', async () => {
    onFrame = () => Response.json({ frame: { lon: 11.5, lat: 48.1, zoom: 12 }, key: '0123456789abcdef', current: false });
    const frameGets = () => calls.filter((c) => !c.init.method && c.url.endsWith('/preview-map')).length;
    await open();
    drag('trim_a', 100);
    await save();
    expect(frameGets()).toBe(0);
    document.body.replaceChildren(host);
    host.replaceChildren();
    resetStubLog();
    settings = fresh({ visibility: 'unlisted' });
    await open();
    drag('trim_b', N - 100);
    await save();
    expect(frameGets()).toBe(1);
  });
});

describe('F-49 ZN-4, ZN-7: zone fields and the Zones section (AC 6, 7, 9)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const T = decoded.data.time;
  const S = Date.parse(decoded.data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((S + T[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const zones = (): HTMLElement => $('#zones-slot')!;
  const bounds = (kind: string): string[] => [...zones().querySelectorAll(`.zone[data-k="${kind}"] .zone-r`)].map((e) => e.textContent!);
  const seconds = (kind: string): number =>
    [...zones().querySelectorAll(`.zone[data-k="${kind}"] .zone-t`)].map((e) => e.textContent!.split(':').reduce((s, p) => s * 60 + Number(p), 0)).reduce((x, y) => x + y, 0);
  const paint = async (): Promise<void> => {
    stubLog.chartStack[0].onready?.();
    await settle();
  };

  it('a file without zone values: no section until a value is saved; then the sharer\'s zones; cleared, none again', async () => {
    await open();
    await paint();
    expect([field('zones_max_heart_rate').value, field('zones_threshold_power').value]).toEqual(['', '']);
    expect(zones().hidden).toBe(true);
    type('zones_threshold_power', '250');
    await settle();
    expect(zones().hidden).toBe(true); // unsaved input changes nothing (E-6)
    await save();
    expect(patches()).toEqual([{ zones_threshold_power: 250 }]);
    await vi.waitFor(() => expect(bounds('power')).toHaveLength(7));
    expect(bounds('power')[5]).toBe('138–187 W'); // 137.5 → 138, 187.5 → 188; Z2, highest on top
    expect([...zones().querySelectorAll('.zones-src')].map((e) => e.textContent)).toEqual(['Estimated from threshold power 250 W']);
    expect(field('zones_threshold_power').value).toBe('250');
    type('zones_threshold_power', '');
    await save();
    expect(patches()[1]).toEqual({ zones_threshold_power: null });
    expect(zones().hidden).toBe(true);
  });

  it('a stored value fills its field and is the basis from the start', async () => {
    settings = fresh({ zones_max_heart_rate: 190 });
    await open();
    await paint();
    await vi.waitFor(() => expect(bounds('heart_rate')).toHaveLength(5));
    expect(field('zones_max_heart_rate').value).toBe('190');
    expect(bounds('heart_rate')[4]).toBe('≤ 113 bpm'); // highest zone on top
    expect([...zones().querySelectorAll('.zones-src')].map((e) => e.textContent)).toEqual(['Estimated from max heart rate 190 bpm']);
  });

  it('ZN-7, ZN-10, AC 2b: the file basis is the placeholder; empty keeps the device zones, a typed value (even the same) is the sharer\'s', async () => {
    draft = () => new Response(golden('zones-wahoo-ride').slice(0), { status: 200 });
    const captions = (): string[] => [...zones().querySelectorAll('.zones-src')].map((e) => e.textContent!);
    await open();
    await paint();
    // Power has a basis next to its list; heart rate only a list: no placeholder.
    expect([field('zones_max_heart_rate').placeholder, field('zones_threshold_power').placeholder]).toEqual(['', '250']);
    expect([field('zones_max_heart_rate').value, field('zones_threshold_power').value]).toEqual(['', '']);
    expect([...document.querySelectorAll('.edit-small')].map((e) => e.textContent)).toContain("For the heart-rate and power zones. Empty: the device's zones and values from the FIT file are used.");
    await vi.waitFor(() => expect(bounds('power')).toHaveLength(7));
    expect(captions()).toEqual(['Zones as recorded by the device', 'Zones as recorded by the device (threshold power 250 W)']);
    expect(bounds('power')[5]).toBe('139–188 W'); // the device's list
    type('title', 'x');
    await save();
    expect(patches()).toEqual([{ title: 'x' }]); // empty fields send nothing new: null stays null
    type('zones_threshold_power', '250');
    await save();
    expect(patches()[1]).toEqual({ zones_threshold_power: 250 });
    await vi.waitFor(() => expect(captions()[1]).toBe('Estimated from threshold power 250 W'));
    expect(bounds('power')[5]).toBe('138–187 W'); // the model replaced the whole kind
    type('zones_threshold_power', '');
    await save();
    expect(patches()[2]).toEqual({ zones_threshold_power: null });
    await vi.waitFor(() => expect(captions()[1]).toBe('Zones as recorded by the device (threshold power 250 W)'));
    expect(field('zones_threshold_power').placeholder).toBe('250');
  });

  it('OQ-195: a value the server refuses is not sent; its sentence shows until the next edit', async () => {
    await open();
    type('zones_max_heart_rate', '99');
    await save();
    expect(patches()).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('The max heart rate must be a whole number from 100 to 250 bpm.');
    type('zones_max_heart_rate', '250');
    expect($('.edit-status')?.textContent).toBe('');
    type('zones_threshold_power', '1000.5');
    $<HTMLButtonElement>('[data-act="publish"]')!.click();
    await settle();
    expect(calls.filter((c) => c.url.endsWith('/publish'))).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('The threshold power must be a whole number from 50 to 1,000 W.');
  });

  it('TR-9, AC 9: the zones count only the saved trim window; an unsaved trim changes nothing', async () => {
    settings = fresh({ zones_threshold_power: 250, trim_start_at: at(1000), trim_end_at: at(1600) });
    await open();
    await paint();
    await vi.waitFor(() => expect(bounds('power')).toHaveLength(7));
    const saved = seconds('power');
    expect(saved).toBeGreaterThan(0);
    expect(saved).toBeLessThanOrEqual(600);
    type('trim_b', String(N - 1));
    await settle();
    expect(seconds('power')).toBe(saved);
  });
});

describe('F-50 EF-1: the Best efforts section over the saved trim window (AC 5)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const T = decoded.data.time;
  const S = Date.parse(decoded.data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((S + T[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const names = (): string[] => [...document.querySelectorAll('#efforts-slot tbody td:first-child')].map((e) => e.textContent!);

  it('only the saved window counts; an unsaved trim changes nothing; a save that changes it recomputes', async () => {
    settings = fresh({ trim_start_at: at(1000), trim_end_at: at(1600) });
    await open();
    stubLog.chartStack[0].onready?.();
    await vi.waitFor(() => expect(names()).toEqual(['5 s', '1 min', '5 min']));
    const table = $('#efforts-slot table');
    type('title', 'x');
    await save();
    expect($('#efforts-slot table')).toBe(table); // same window: no recompute
    type('trim_b', String(N - 1));
    await settle();
    expect($('#efforts-slot table')).toBe(table);
    await save();
    await vi.waitFor(() => expect(names()).toEqual(['5 s', '1 min', '5 min', '20 min', '60 min']));
  });
});

describe('F-58 SE-4: the summary dialog on the edit page (AC 36, 40)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const { calories, ...rest } = decoded.data.meta.summary;
  const dialogText = async (): Promise<string> => {
    $<HTMLButtonElement>('button[data-summary-info]')!.click();
    await vi.waitFor(() => expect($<HTMLDialogElement>('dialog.summary-dlg')?.open).toBe(true));
    const d = $<HTMLDialogElement>('dialog.summary-dlg')!;
    d.close();
    return d.textContent!;
  };
  beforeEach(() => {
    HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
      this.open = true;
    };
    HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
      this.open = false;
    };
    localStorage.setItem('wx.units', 'metric');
  });

  it('radii, trim ends and the recomputed and dropped totals from the saved settings; rebuilt on every open', async () => {
    expect(calories).toBeGreaterThan(0);
    settings = fresh({ ...ON, trim_start_at: '2026-10-29T10:00:00Z', trim_end_at: '2026-10-29T11:00:00Z', trimmed: { sport: decoded.data.meta.sport, summary: rest } });
    await open();
    const button = $<HTMLButtonElement>('button[data-summary-info]')!;
    expect([button.classList.contains('ready'), button.dataset.trim]).toEqual([true, 'both']);
    let text = await dialogText();
    expect(text).toContain('This workout: start hidden within 300 m, end hidden within 300 m.');
    expect(text).toMatch(/This workout: trimmed \(now starts \d\d:\d\d, ends \d\d:\d\d\); recomputed: distance, [^;]*; dropped: calories\./);
    hideAt(0, 0);
    text = await dialogText();
    expect(text).toContain('This workout: start hidden within 300 m, end hidden within 300 m.'); // the saved radii, not the sliders
    await save();
    text = await dialogText();
    expect(text).toContain('This workout: start not hidden, end not hidden.');
  });

  it('names the trimmed end, and a trim save refreshes it without a reload (QA-58-r1-5)', async () => {
    settings = fresh({ trim_start_at: '2026-10-29T10:00:00Z' });
    await open();
    const button = $<HTMLButtonElement>('button[data-summary-info]')!;
    expect(button.dataset.trim).toBe('start');
    let text = await dialogText();
    expect(text).toMatch(/trimmed \(now starts \d\d:\d\d\)/);
    type('trim_a', '0');
    type('trim_b', String(N - 200));
    await save();
    expect(button.dataset.trim).toBe('end');
    text = await dialogText();
    expect(text).toMatch(/trimmed \(now ends \d\d:\d\d\)/);
    expect(text).not.toContain('starts');
  });

  it('untrimmed: no trim line, data-trim empty', async () => {
    await open();
    expect($<HTMLButtonElement>('button[data-summary-info]')!.dataset.trim).toBe('');
    expect(await dialogText()).not.toMatch(/Trim\.|recomputed/);
  });
});

describe('F-56 PN-12: pins on the edit page (AC 33 to 36, 41)', () => {
  const data = (() => {
    const r = decodeDataFile(RIDE);
    if (!r.ok) throw new Error('no ride');
    return r.data;
  })();
  const startS = Date.parse(data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((startS + data.time[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const pin = (i: number, title: string) => ({ type: 'danger' as const, title, note: '', start_at: at(i), end_at: null });
  const rowTexts = (): string[] => [...document.querySelectorAll('.pin-row')].map((r) => r.textContent!);
  const button = (text: string): HTMLButtonElement => [...document.querySelectorAll<HTMLButtonElement>('.pin-tools button, .wx-addpin button, .pin-form button')].find((b) => b.textContent === text)!;
  /** F-65 AL-9: "+" opened (when it can be), then the item's disabled state; "+" disabled counts as off. */
  const plusButton = (): HTMLButtonElement => document.querySelector<HTMLButtonElement>('.plus-button')!;
  const off = (text: string): boolean => {
    const plus = plusButton();
    if (plus.disabled) return true;
    if (plus.getAttribute('aria-expanded') !== 'true') plus.click();
    return button(text).getAttribute('aria-disabled') === 'true';
  };
  async function start(): Promise<void> {
    await open();
    stubLog.chartStack[0].onready?.();
    await vi.waitFor(() => expect(document.querySelector('.pin-tools')).not.toBeNull());
    await settle();
  }
  const pick = async (i: number): Promise<void> => {
    stubLog.chartStack[0].cursor.set(i);
    frames.splice(0).forEach((f) => f(0));
    await settle();
  };

  it('dimmed reasons; Add pin refuses a hidden sample, places on a visible one; Save sends the whole set, dimmed included', async () => {
    settings = fresh({ ...ON, pins: [pin(10, 'start'), pin(1000, 'mid'), pin(N - 5, 'end')] });
    await start();
    expect(rowTexts()).toEqual(['startHidden by the start radius', expect.stringMatching(/^mid\d/), 'endHidden by the end radius']);
    expect(off('Add pin')).toBe(true); // no cursor
    await pick(20); // in the hidden start radius: refused, nothing placed
    expect(off('Add pin')).toBe(true);
    button('Add pin').click();
    expect(rowTexts()).toHaveLength(3);
    await pick(2000);
    expect(off('Add pin')).toBe(false);
    button('Add pin').click();
    expect(rowTexts()).toHaveLength(4);
    const title = document.querySelector<HTMLInputElement>('.pin-form input:not([type])')!;
    expect(document.querySelector<HTMLElement>('.pin-form')!.hidden).toBe(false);
    title.value = '<b>new</b>';
    title.dispatchEvent(new Event('input', { bubbles: true }));
    expect(rowTexts()[2]).toMatch(/^<b>new<\/b>/);
    expect(document.querySelector('.pin-list b')).toBeNull();
    await save();
    expect(patches()).toEqual([{ pins: [pin(10, 'start'), pin(1000, 'mid'), { ...pin(2000, '<b>new</b>'), type: 'point_of_interest' }, pin(N - 5, 'end')] }]);
    await save();
    expect(patches()).toHaveLength(1); // unchanged: no pins sent, 'No changes'
  });

  it('a Save whose response lacks pins says how many were removed (AC 36)', async () => {
    settings = fresh({ pins: [pin(10, 'a'), pin(1000, 'b')] });
    onPatch = (body) => Response.json((settings = { ...settings, ...body, pins: [pin(1000, 'b')] } as Settings));
    await start();
    $<HTMLInputElement>('input[name="title"]')!.value = 'x';
    await save();
    expect(document.querySelector('.pin-tools .edit-status')?.textContent).toBe('1 pin in the trimmed part removed.');
    expect(rowTexts()).toHaveLength(1);
  });

  it('no visible route: no Add pin, stored pins listed dimmed (OQ-275)', async () => {
    settings = fresh({ hide_start_m: 2000, hiding_steps: { start: STEPS.start.map(() => -1), end: STEPS.end }, pins: [pin(1000, 'b')] });
    await start();
    await pick(1000);
    plusButton().click();
    expect((button('Add pin').parentElement as HTMLElement).hidden).toBe(true);
    expect((button('Add lap').parentElement as HTMLElement).hidden).toBe(false); // OQ-305: only Add lap
    expect(rowTexts()[0]).toMatch(/Hidden by the start/);
  });

  it('Add pin over a range makes a section; a new range moves its ends; a hidden end is refused and nothing changes (AC 34)', async () => {
    settings = fresh({ ...ON, pins: [] });
    await start();
    const sel = stubLog.chartStack[0].selection!;
    const range = async (a: number, b: number): Promise<void> => {
      sel.set(a, b);
      frames.splice(0).forEach((f) => f(0));
      await settle();
    };
    await range(2000, 3000);
    button('Add pin').click();
    expect(rowTexts()[0]).toMatch(/–/);
    await range(2500, 3500);
    expect(rowTexts()[0]).toMatch(/–/);
    await range(2600, N - 5); // the end is in the hidden end radius
    expect(document.querySelector('.pin-tools .edit-status')?.textContent).toBe(PIN_REFUSED);
    expect(off('Add pin')).toBe(true);
    await save();
    expect(patches()).toEqual([{ pins: [{ ...pin(2500, ''), type: 'point_of_interest', end_at: at(3500) }] }]); // still 2500 to 3500
  });

  it('PN-12, AL-9: in the two-column layout "+" with Add pin and its reason docks on the map slot, one column keeps them in the tools row', async () => {
    let matches = true;
    const listeners: ((e: { matches: boolean }) => void)[] = [];
    vi.stubGlobal('matchMedia', () => ({ get matches() { return matches; }, addEventListener: (_: string, f: (e: { matches: boolean }) => void) => listeners.push(f), removeEventListener() {} }));
    settings = fresh({ ...ON, pins: [] });
    await start();
    const add = plusButton();
    expect(add.closest('#map-slot .wx-addpin')).not.toBeNull();
    expect(button('Add pin').closest('.wx-plus')).toBe(add.closest('.wx-plus'));
    matches = false;
    listeners.forEach((f) => f({ matches }));
    expect(add.closest('.pin-tools')).not.toBeNull();
    expect(document.querySelector('.pin-tools')?.classList.contains('on-map')).toBe(false);
    // one column: no "+", both items open side by side; back in two columns the "+" returns closed
    expect([add.hidden, document.querySelector<HTMLElement>('.plus-menu')!.hidden]).toEqual([true, false]);
    expect([...document.querySelectorAll<HTMLElement>('.plus-item')].map((b) => [b.textContent, b.getAttribute('aria-label')])).toEqual([['+ Lap', 'Add lap'], ['+ Pin', 'Add pin']]);
    matches = true;
    listeners.forEach((f) => f({ matches }));
    expect([add.hidden, add.getAttribute('aria-expanded'), document.querySelector<HTMLElement>('.plus-menu')!.hidden]).toEqual([false, 'false', true]);
    expect([...document.querySelectorAll<HTMLElement>('.plus-item')].map((b) => [b.textContent, b.getAttribute('aria-label')])).toEqual([['Add lap', null], ['Add pin', null]]);
  });

  it('one column: each row is the short button and its reason on one line, the row keeps its place when empty, full texts for assistive tech', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
    settings = fresh({ ...ON, pins: [] });
    await start();
    const whys = [...document.querySelectorAll<HTMLElement>('.plus-row .edit-small')];
    expect(whys.map((n) => n.textContent)).toEqual([editTexts.lapHere, editTexts.pinHiddenSpot]); // in item order
    expect(whys.map((n) => n.title)).toEqual([editTexts.lapHere, editTexts.pinHiddenSpot]);
    const add = document.querySelector<HTMLButtonElement>('.plus-item')!;
    expect(add.getAttribute('aria-describedby')).toBe(whys[0].id);
    expect(add.getAttribute('aria-label')).toBe('Add lap');
    await pick(2000);
    expect(whys.map((n) => n.textContent)).toEqual(['', '']); // still in the DOM, the CSS keeps their height
    expect(whys.every((n) => n.isConnected)).toBe(true);
  });

  it('one column: the items are disabled, as "+" is, until a cursor or range is set', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
    settings = fresh({ ...ON, pins: [] });
    await start();
    expect([...document.querySelectorAll<HTMLButtonElement>('.plus-item')].map((b) => b.disabled)).toEqual([true, true]);
    await pick(2000);
    expect([...document.querySelectorAll<HTMLButtonElement>('.plus-item')].map((b) => b.disabled)).toEqual([false, false]);
  });

  const why = (): string => (button('Add pin').nextElementSibling as HTMLElement).textContent!.trim();

  it('the button and its visible reason follow radius and trim changes; a twin is refused; Move refuses a hidden spot (r3-2 to r3-4)', async () => {
    settings = fresh({ ...ON, pins: [] });
    await start();
    expect([off('Add pin'), why()]).toEqual([true, '']);
    await pick(20); // inside the 300 m start radius
    expect([off('Add pin'), why()]).toEqual([true, 'Hidden or trimmed']);
    hideAt(0, 0); // radius off: the same cursor is public
    await settle();
    expect([off('Add pin'), why()]).toEqual([false, '']);
    hideAt(300, 300);
    await settle();
    expect(off('Add pin')).toBe(true);
    await pick(2000);
    type('trim_a', '3000'); // the cursor is now in the trimmed part
    await settle();
    expect([off('Add pin'), why()]).toEqual([true, 'Hidden or trimmed']);
    type('trim_a', '0');
    await settle();
    expect(off('Add pin')).toBe(false);
    button('Add pin').click();
    expect(rowTexts()).toHaveLength(1);
    expect([off('Add pin'), why()]).toEqual([true, 'Already a pin here']); // selected pin: no twin
    button('Add pin').click();
    expect(rowTexts()).toHaveLength(1);
    await pick(30); // Move into the hidden start: disabled, and a click changes nothing
    expect(button('Move').disabled).toBe(true);
    button('Move').click();
    await save();
    expect(patches().at(-1)).toEqual({ pins: [{ ...pin(2000, ''), type: 'point_of_interest' }] });
  });

  it('Move puts a point pin at the cursor; disabled without a usable cursor', async () => {
    settings = fresh({ ...ON, pins: [pin(1000, 'mid')] });
    await start();
    document.querySelector<HTMLButtonElement>('.pin-row')!.click();
    await pick(1500);
    button('Move').click();
    await save();
    expect(patches()).toEqual([{ pins: [pin(1500, 'mid')] }]);
  });
});

describe('F-65 AL-8, AL-9: added laps on the edit page', () => {
  const data = (() => {
    const r = decodeDataFile(RIDE);
    if (!r.ok) throw new Error('no ride');
    return r.data;
  })();
  const startS = Date.parse(data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((startS + data.time[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const plus = (): HTMLButtonElement => document.querySelector<HTMLButtonElement>('.plus-button')!;
  const item = (text: string): HTMLButtonElement => [...document.querySelectorAll<HTMLButtonElement>('.plus-item')].find((b) => b.textContent === text)!;
  const reason = (text: string): string => (item(text).nextElementSibling as HTMLElement).textContent!;
  const rows = (): HTMLTableRowElement[] => [...document.querySelectorAll<HTMLTableRowElement>('section.laps .laps-scroll:not(.splits) tbody tr')];
  const lastCells = (): string[] => rows().map((r) => r.cells[r.cells.length - 2]?.textContent ?? '');
  async function start(): Promise<void> {
    await open();
    stubLog.chartStack[0].onready?.();
    await vi.waitFor(() => expect(document.querySelector('.pin-tools')).not.toBeNull());
    await settle();
  }
  const pick = async (i: number): Promise<void> => {
    stubLog.chartStack[0].cursor.set(i);
    frames.splice(0).forEach((f) => f(0));
    await settle();
  };
  const addLap = async (): Promise<void> => {
    if (plus().getAttribute('aria-expanded') !== 'true') plus().click();
    item('Add lap').click();
    await settle();
  };

  it('"+": disabled without a cursor; opens a list with Add lap and Add pin; Esc, outside and "+" again close it', async () => {
    await start();
    expect(plus().disabled).toBe(true);
    await pick(1000);
    expect([plus().disabled, plus().getAttribute('aria-expanded'), plus().getAttribute('aria-label')]).toEqual([false, 'false', 'Add a lap or pin']);
    plus().click();
    const menu = document.querySelector<HTMLElement>('.plus-menu')!;
    expect([menu.hidden, plus().getAttribute('aria-expanded')]).toEqual([false, 'true']);
    expect([...menu.querySelectorAll('.plus-item')].map((b) => b.textContent)).toEqual(['Add lap', 'Add pin']);
    expect(plus().nextElementSibling).toBe(menu); // Tab reaches the items after "+"
    item('Add lap').focus();
    item('Add lap').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect([menu.hidden, document.activeElement]).toEqual([true, plus()]);
    plus().click();
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }));
    expect(menu.hidden).toBe(true);
    plus().click();
    plus().click();
    expect(menu.hidden).toBe(true);
    plus().click();
    stubLog.chartStack[0].cursor.set(null);
    frames.splice(0).forEach((f) => f(0));
    await settle();
    expect([menu.hidden, plus().disabled]).toEqual([true, true]);
  });

  it('AL-9: a boundary within 2 s of a device lap end counts as there already (cursor and both range ends)', async () => {
    await start();
    const e = data.meta.laps[0].end_index - 1;
    const near = data.time.findIndex((x, i) => i > e && x - data.time[e] > 0 && x - data.time[e] <= 2);
    const far = data.time.findIndex((x, i) => i > e && x - data.time[e] > 2);
    const sel = stubLog.chartStack[0].selection!;
    const range = async (a: number, b: number): Promise<void> => {
      sel.set(a, b);
      frames.splice(0).forEach((f) => f(0));
      await settle();
    };
    await pick(near);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Already a lap here']);
    await range(e, near); // both ends at or near the same end
    expect(item('Add lap').getAttribute('aria-disabled')).toBe('true');
    await range(near, far); // the far end is new: only it is added
    expect(item('Add lap').getAttribute('aria-disabled')).toBe('false');
    item('Add lap').click();
    await settle();
    expect(rows()).toHaveLength(17);
    await save();
    expect(patches()).toEqual([{ added_lap_boundaries: [at(far)] }]);
  });

  it('Add lap splits the lap at the cursor: one more row, "Added" then the device trigger, × only there; Save sends the set once', async () => {
    await start();
    expect(rows()).toHaveLength(16);
    await pick(1000);
    await addLap();
    expect(rows()).toHaveLength(17);
    const x = [...document.querySelectorAll<HTMLButtonElement>('.lap-x-btn')];
    expect(x.map((b) => b.getAttribute('aria-label'))).toEqual(['Remove the added lap boundary at the end of lap 2']);
    expect(rows()[1].querySelector('.lap-x-btn')).toBe(x[0]);
    expect(rows()[1].cells[0].textContent).toBe('2');
    expect(rows()[1].textContent).toContain('Added');
    expect(rows()[2].textContent).not.toContain('Added');
    expect(data.meta.laps).toHaveLength(16); // the test's own decode is untouched
    // On the boundary now: nothing to add, with the reason.
    plus().click();
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Already a lap here']);
    expect(item('Add lap').getAttribute('aria-describedby')).toBe((item('Add lap').nextElementSibling as HTMLElement).id);
    await save();
    expect(patches()).toEqual([{ added_lap_boundaries: [at(1000)] }]);
    await save();
    expect(patches()).toHaveLength(1);
    // The × removes it, the row goes, and the next Save clears the set.
    x[0].click();
    await settle();
    expect(rows()).toHaveLength(16);
    expect(document.querySelector('.lap-x-btn')).toBeNull();
    await save();
    expect(patches().at(-1)).toEqual({ added_lap_boundaries: [] });
  });

  it('with a range: both ends (OQ-304); an end on a device boundary adds nothing', async () => {
    await start();
    const sel = stubLog.chartStack[0].selection!;
    sel.set(1000, 1200);
    frames.splice(0).forEach((f) => f(0));
    await settle();
    await addLap();
    sel.set(1100, 1800); // 1800 is a device lap end
    frames.splice(0).forEach((f) => f(0));
    await settle();
    await addLap();
    await save();
    expect(patches()).toEqual([{ added_lap_boundaries: [at(1000), at(1100), at(1200)] }]);
    expect(rows()).toHaveLength(19);
  });

  it('a stored boundary shows at once; a trimmed one is refused, dimmed "Removed on Save" and not sent; an invalid stored one is dropped', async () => {
    settings = fresh({ added_lap_boundaries: ['2000-01-01T00:00:00Z', at(500), at(5000)] });
    await start();
    expect(rows()).toHaveLength(18);
    type('trim_a', '3000');
    await settle();
    expect(rows()[0].classList.contains('dim')).toBe(true);
    expect(rows()[0].querySelector('.lap-x')?.textContent).toBe('×Removed on Save');
    await pick(2000);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'In the trimmed part']);
    await save();
    expect(patches()[0].added_lap_boundaries).toEqual([at(5000)]);
  });

  it('a lap boundary in a hidden start is allowed (no position check)', async () => {
    settings = fresh(ON);
    await start();
    await pick(20); // inside the 300 m start radius
    await addLap();
    await save();
    expect(patches()[0].added_lap_boundaries).toEqual([at(20)]);
  });

  it('at 100 added boundaries Add lap is off with its reason', async () => {
    settings = fresh({ added_lap_boundaries: Array.from({ length: 100 }, (_, k) => at(11 + 10 * k)) });
    await start();
    await pick(5003);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Max. 100 added laps']);
  });

  it('without device laps: the first boundary brings the laps view, removing it returns to splits only', async () => {
    const plain = golden('ride-no-laps');
    draft = () => new Response(plain.slice(0), { status: 200 });
    await start();
    const section = document.querySelector<HTMLElement>('section.laps')!;
    expect(section.querySelector('.laps-scroll:not(.splits)')).toBeNull();
    await pick(300);
    await addLap();
    await vi.waitFor(() => expect(rows()).toHaveLength(2));
    expect(lastCells()).toEqual(['Added', '']);
    await vi.waitFor(() => expect(document.querySelector<HTMLElement>('section.laps')?.hidden).toBe(false));
    document.querySelector<HTMLButtonElement>('.lap-x-btn')!.click();
    await settle();
    expect(rows()).toHaveLength(0);
  });

  it('QA-65-r1-2: the live trim end is a boundary; focus leaving the group closes the list; a choice returns focus to "+"', async () => {
    await start();
    type('trim_b', '5000');
    await settle();
    await pick(5000);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Already a lap here']);
    const menu = document.querySelector<HTMLElement>('.plus-menu')!;
    item('Add lap').dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: field('title') }));
    expect(menu.hidden).toBe(true);
    await pick(1000);
    plus().click();
    item('Add lap').focus();
    item('Add lap').click();
    expect([menu.hidden, document.activeElement]).toEqual([true, plus()]);
  });

  it('QA-65-r1-2: after a change the map lap markers, the splits chunk (its switch) and the row taps start again', async () => {
    await start();
    const laps = vi.fn();
    const handle = { setScheme() {}, setHidden() {}, marker: () => () => {}, marks() {}, laps, control() {}, sampleAt: () => -1, destroy() {} };
    (stubLog.routeMap[0] as unknown as { onmapready(h: typeof handle): void }).onmapready(handle);
    await settle();
    await pick(1000);
    await addLap();
    await vi.waitFor(() => expect(laps).toHaveBeenCalledTimes(1));
    const section = document.querySelector<HTMLElement>('section.laps')!;
    await vi.waitFor(() => expect(section.querySelector('#laps-slot')!.childElementCount).toBeGreaterThan(0)); // the switch
    expect(section.classList.contains('live')).toBe(true);
    rows()[1].click(); // the new first part: samples 900 to 1000
    frames.splice(0).forEach((f) => f(0));
    const sel = stubLog.chartStack[0].selection!;
    expect([sel.start(), sel.end()]).toEqual([900, 1000]);
    // ×: the module is rebuilt and focus lands on the laps heading (no other ×).
    document.querySelector<HTMLButtonElement>('.lap-x-btn')!.click();
    await settle();
    expect(laps).toHaveBeenCalledTimes(2);
    expect(document.activeElement).toBe(document.querySelector('section.laps h2'));
  });

  it('QA-65-r1-5: boundaries the live trim removes do not count towards the 100', async () => {
    settings = fresh({ added_lap_boundaries: Array.from({ length: 100 }, (_, k) => at(11 + 10 * k)) });
    await start();
    type('trim_a', '500');
    await settle();
    await pick(5003);
    plus().click();
    expect(item('Add lap').getAttribute('aria-disabled')).toBe('false');
  });

  it('QA-65-r1-3: on a multisport workout the × names the lap number its leg table shows', async () => {
    draft = () => new Response(golden('triathlon').slice(0), { status: 200 });
    await start();
    const tri = decodeDataFile(golden('triathlon'));
    if (!tri.ok) throw new Error('no triathlon');
    const bike = tri.data.meta.sessions.find((x) => x.sport === 'cycling')!;
    const firstBikeLap = tri.data.meta.laps.find((l) => l.start_index >= bike.start_index)!;
    await pick(firstBikeLap.start_index + 10);
    await addLap();
    const x = document.querySelector<HTMLButtonElement>('.lap-x-btn')!;
    expect(x.closest('tr')!.cells[0].textContent).toBe('1');
    expect(x.getAttribute('aria-label')).toBe('Remove the added lap boundary at the end of lap 1');
  });

  it('no visible route (OQ-305): "+" sits below the no-location hint and offers only Add lap', async () => {
    const indoor = golden('indoor-no-gps');
    draft = () => new Response(indoor.slice(0), { status: 200 });
    settings = fresh({ hiding_steps: null });
    await start();
    const hint = document.querySelector('p.hint')!;
    expect(hint.compareDocumentPosition(plus()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    await pick(100);
    plus().click();
    expect([...document.querySelectorAll<HTMLElement>('.plus-row')].map((r) => r.hidden)).toEqual([false, true]);
  });
});

describe('F-53 MW-2, MW-3: the My workouts entry (AC 42, 44, 46, 48, 49, 51, 52)', () => {
  const OTHER = 'Yy0123456789abcdefghij';
  const stored = () => JSON.parse(localStorage.getItem(LIST_KEY) ?? 'null') as { v: number; workouts: Record<string, unknown>[] } | null;
  const entry = () => stored()?.workouts.find((e) => e.id === ID);
  const meta = () => {
    const d = decodeDataFile(RIDE.slice(0));
    if (!d.ok) throw new Error('golden');
    return d.data.meta;
  };
  const put = (...workouts: Record<string, unknown>[]) => localStorage.setItem(LIST_KEY, JSON.stringify({ v: 1, workouts }));

  it('the first read adds one entry with the header sport and local date, Draft, the stored instants; nothing sent', async () => {
    settings = fresh({ suggested_title: 'Morning ride' });
    await open();
    const m = meta();
    const date = new Date(Date.parse(m.start_time) + (m.time_zone.known ? m.time_zone.offset_seconds : 0) * 1000).toISOString().slice(0, 10);
    expect(stored()).toEqual({
      v: 1,
      workouts: [{ id: ID, token: TOKEN, title: 'Morning ride', sport: m.sport, date, created: settings.created_at, expires: settings.expires_at, published: false }],
    });
    for (const { url, init } of calls) expect(url + String(init.body ?? '')).not.toMatch(/Morning ride|wx\.workouts/);
    expect(document.cookie).toBe('');
    expectNoTokenOutsideTheFragment();
  });

  it('the default title applies: title stored empty; an owner title wins', async () => {
    await open();
    expect(entry()?.title).toBe('');
    type('title', 'Loop');
    await save();
    expect(entry()?.title).toBe('Loop');
  });

  it('one entry per workout: reopening and saving replace it, others stay; an unreadable value is replaced', async () => {
    put({ id: 'B'.repeat(22), token: OTHER, title: 'Other' }, { id: ID, token: TOKEN, title: 'old', sport: 'rowing', date: '2020-01-01' });
    await open();
    expect(entry()).toMatchObject({ title: '', sport: meta().sport });
    type('title', 'New');
    await save();
    expect(stored()!.workouts.map((e) => e.id)).toEqual(['B'.repeat(22), ID]);
    expect(entry()).toMatchObject({ title: 'New', sport: meta().sport, expires: settings.expires_at });
    localStorage.setItem(LIST_KEY, '{"v":2}');
    await open();
    expect(stored()!.workouts.map((e) => e.id)).toEqual([ID]);
    put({ id: ID, token: OTHER, title: 'old' });
    await open();
    expect(entry()?.token).toBe(TOKEN); // QA-53-r1-3: the address's token wins
  });

  it('the draft unreadable: the stored sport and date stay', async () => {
    put({ id: ID, token: TOKEN, title: 'x', sport: 'rowing', date: '2020-01-01' });
    draft = () => new Response(new Uint8Array([1, 2, 3]), { status: 200 });
    await open();
    expect(entry()).toMatchObject({ sport: 'rowing', date: '2020-01-01', title: '' });
  });

  it('publish sets Published before the navigation; the published page updates title, status, expiry of an entry and never adds one', async () => {
    await open();
    await click('publish');
    expect(entry()?.published).toBe(true);
    localStorage.clear();
    settings = fresh({ visibility: 'unlisted', title: 'Pub', expires_at: '2026-11-05T11:00:00Z' });
    await openPublished();
    expect(stored()).toBeNull();
    put({ id: ID, token: TOKEN, title: 'old', sport: 'rowing', published: false });
    await openPublished();
    expect(entry()).toEqual({ id: ID, token: TOKEN, title: 'Pub', sport: 'rowing', created: settings.created_at, expires: '2026-11-05T11:00:00Z', published: true });
    expect(document.body.textContent).toContain('Edit links are saved in this browser. Anyone using this browser can open them.');
  });

  it.each([
    ['edit', () => open()],
    ['published', () => openPublished()],
  ])('delete on the %s page removes the entry', async (_, go) => {
    settings = fresh({ visibility: 'unlisted' });
    put({ id: ID, token: TOKEN, title: 'x' }, { id: 'B'.repeat(22), token: OTHER });
    await go();
    expect(entry()).toBeDefined();
    await click('delete');
    await click('confirm');
    expect(stored()!.workouts.map((e) => e.id)).toEqual(['B'.repeat(22)]);
  });

  it('P-12: not found removes the entry only when it holds the address token', async () => {
    onGet = () => new Response('{}', { status: 404 });
    put({ id: ID, token: OTHER, title: 'mine' });
    await open();
    expect(document.body.textContent).toContain(GENERIC);
    expect(entry()?.token).toBe(OTHER);
    put({ id: ID, token: TOKEN, title: 'mine' });
    await open();
    expect(stored()!.workouts).toEqual([]);
    put({ id: ID, token: TOKEN, title: 'mine' });
    await openPublished();
    expect(stored()!.workouts).toEqual([]);
  });

  it('not found on save also removes it', async () => {
    await open();
    onPatch = () => new Response('', { status: 404 });
    type('title', 'x');
    await save();
    expect(stored()!.workouts).toEqual([]);
  });

  it.each([
    ['a throwing getter', () => vi.spyOn(window, 'localStorage', 'get').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError'); })],
    ['a full storage', () => vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('full', 'QuotaExceededError'); })],
    ['a throwing getItem', () => vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError'); })],
  ])('AC 52, %s: edit, save, publish and delete still work, no message', async (_, block) => {
    block();
    await open();
    expect($('form.edit-panel')).not.toBeNull();
    type('title', 'x');
    await save();
    expect($('.edit-status')?.textContent).toBe('Saved');
    await click('publish');
    expect(navs).toEqual([PUBLISHED_PAGE()]);
    expect(consoleOut).toEqual([]);
  });
});
