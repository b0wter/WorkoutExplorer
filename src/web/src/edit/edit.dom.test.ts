// @vitest-environment jsdom
// The edit page (F-19 design E-1 to E-10; AC 1 to 8). Critical lane: the edit token is in the
// fragment and only ever leaves as the Authorization header (ADR-020, ADR-025). ChartStack and
// RouteMap are the page tests' recording stubs; fetch serves synthetic golden data files.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import type { Settings } from '../lib/owner';
import { resetStubLog, stubLog } from '../page/testing/stub-log';
import { startEdit } from './start.svelte';

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

const NOW = Date.parse('2026-10-29T12:00:00Z');
const FIT = new Uint8Array([14, 16, 0x98, 0, 0xff, 0x2e, 0x46, 0x49, 0x54]);
const EDIT_LINK = () => window.location.origin + '/edit/' + ID + '#' + TOKEN;
const VIEW_LINK = () => window.location.origin + '/w/' + ID;
const fresh = (over: Partial<Settings> = {}): Settings => ({
  title: '',
  description: '',
  hide_ends: false,
  allow_download: false,
  visibility: 'draft',
  lifetime_days: 30,
  created_at: '2026-10-29T11:00:00Z',
  expires_at: '2026-11-28T11:00:00Z',
  data_end_s: null,
  view_url: '/w/' + ID,
  hide_ends_ranges: RANGES,
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
  vi.stubGlobal('requestAnimationFrame', () => 1);
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
      expect(k + (store.getItem(k) ?? '')).not.toContain(TOKEN);
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
function toggle(name: string): void {
  field(name).checked = !field(name).checked;
  field(name).dispatchEvent(new Event('change', { bubbles: true }));
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

  it('AC 4: hiding is off on a new upload; download hidden until F-24; visibility is "Draft" with no control', async () => {
    await open();
    expect(field('hide_ends').checked).toBe(false);
    expect(field('allow_download')).toBeNull();
    expect(field('visibility')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain('Draft');
  });
});

describe('AC 2, E-6: saving', () => {
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

  it('400: the server message, the form keeps the input, the header the stored text', async () => {
    onPatch = () => Response.json({ error: 'invalid_settings', message: 'The title is too long.' }, { status: 400 });
    await open();
    const before = $('h1')?.textContent;
    type('title', 'Mine');
    await save();
    expect($('.edit-status')?.textContent).toBe('The title is too long.');
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

  it('F-24 (phase 3): the download toggle is hidden and a save keeps the stored value', async () => {
    settings = fresh({ allow_download: true });
    await open();
    expect(field('allow_download')).toBeNull();
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
    settings = fresh({ data_end_s: 1800, hide_ends_ranges: [] });
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

describe('E-9, AC 8: to-be-hidden marks', () => {
  it('the map gets the stored ranges while the checkbox is on, saved or not', async () => {
    await open();
    const map = stubLog.routeMap[0];
    expect(map.hidden).toEqual([]);
    toggle('hide_ends');
    flushSync();
    expect(map.hidden).toEqual(RANGES);
    toggle('hide_ends');
    flushSync();
    expect(map.hidden).toEqual([]);
    toggle('hide_ends');
    await save();
    expect(patches()).toEqual([{ hide_ends: true }]);
    expect(map.hidden).toEqual(RANGES);
    expect(stubLog.routeMap).toHaveLength(1); // never remounted
  });

  it('a fully hidden route marks the whole track', async () => {
    settings = fresh({ hide_ends: true, hide_ends_ranges: [[0, N - 1]] });
    await open();
    expect(stubLog.routeMap[0].hidden).toEqual([[0, N - 1]]);
  });

  it('indoor: one column, no map, the toggle has no effect and no error', async () => {
    draft = () => new Response(golden('indoor-no-gps').slice(0), { status: 200 });
    settings = fresh({ hide_ends_ranges: [] });
    await open();
    expect($('.page')?.classList.contains('has-map')).toBe(false);
    expect($('#map-slot')).toBeNull();
    expect($('.hint')?.textContent).toBe('No location data in this workout.');
    toggle('hide_ends');
    await save();
    expect(patches()).toEqual([{ hide_ends: true }]);
    expect(stubLog.routeMap).toEqual([]);
    expect(consoleOut.filter((a) => a.some((x) => x instanceof Error))).toEqual([]);
    expectNoTokenOutsideTheFragment();
  });
});

const FIELDS = ['title', 'description', 'hide_ends', 'lifetime'];
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
    toggle('hide_ends');
    type('title', 'Loop');
    await click('publish');
    expect(methods()).toEqual(['PATCH ', 'POST /publish']);
    expect(patches()).toEqual([{ title: 'Loop', hide_ends: true }]);
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
    toggle('hide_ends');
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
    toggle('hide_ends');
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
    toggle('hide_ends');
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
      // F-26 (ADR-038 item 3): the fullscreen map's one history entry, with no URL argument.
      const c = file === '../components/map/map-view.ts' ? code(text).replace("history.pushState(null, '')", '') : code(text);
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
