// @vitest-environment jsdom
// The upload page's wiring (F-11 design U-5 to U-10; ADR-012). upload.html is loaded as it is;
// fetch, XMLHttpRequest and navigation are fakes. Synthetic data only.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import HTML from '../../upload.html?raw';
import { start, type Env } from './page';

const sources = import.meta.glob(['./*.ts', '../lib/upload/*.ts', '!./*.test.ts', '!../lib/upload/*.test.ts'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

const ORIGIN = 'http://localhost';
const EDIT = '/edit/4kQ0dTqv1Zr8mN2xYbL7Ce#P9sTa0Wm3Hq5Lx2Kd8Rf1V';

class FakeXhr {
  static all: FakeXhr[] = [];
  upload: { onprogress?: (e: unknown) => void; onload?: () => void } = {};
  onload?: () => void;
  onerror?: () => void;
  onabort?: () => void;
  method = '';
  url = '';
  body: FormData | null = null;
  status = 0;
  responseText = '';
  constructor() {
    FakeXhr.all.push(this);
  }
  open(m: string, u: string) {
    this.method = m;
    this.url = u;
  }
  send(b: FormData) {
    this.body = b;
  }
  respond(status: number, body: unknown) {
    this.upload.onload?.();
    this.status = status;
    this.responseText = typeof body === 'string' ? body : JSON.stringify(body);
    this.onload?.();
  }
}

let statusReply: () => Promise<Response>;
let navigated: string[];
let fetchCalls: string[];
let win: Window & typeof globalThis;

const flush = () => new Promise<void>((r) => setTimeout(r, 0));
const $ = (id: string) => document.getElementById(id)!;
const state = () => $('page').dataset.state;
const input = () => $('file') as HTMLInputElement;

function file(size: number, name = 'ride.fit'): File {
  const f = new File([], name);
  Object.defineProperty(f, 'size', { value: size });
  return f;
}
function pickFile(f: File) {
  Object.defineProperty(input(), 'files', { value: [f], configurable: true });
  input().dispatchEvent(new Event('change'));
}
function dropFiles(...files: File[]): Event {
  const e = new Event('drop', { bubbles: true, cancelable: true });
  Object.defineProperty(e, 'dataTransfer', { value: { files } });
  document.dispatchEvent(e);
  return e;
}
const reply = (status: number, body: unknown) => () =>
  Promise.resolve(new Response(typeof body === 'string' ? body : JSON.stringify(body), { status }));

let stop: (() => void) | undefined;

async function boot(env: Partial<Env> = {}) {
  stop = start(document, win, {
    fetch: (u) => {
      fetchCalls.push(u);
      return statusReply();
    },
    xhr: () => new FakeXhr() as unknown as XMLHttpRequest,
    navigate: (u) => navigated.push(u),
    origin: ORIGIN,
    statusWaitMs: 50,
    ...env,
  });
  await flush();
}

beforeEach(() => {
  document.documentElement.innerHTML = HTML.replace(/<script[\s\S]*?<\/script>/, '');
  FakeXhr.all = [];
  navigated = [];
  fetchCalls = [];
  statusReply = reply(200, { uploads_open: true });
  win = window as unknown as Window & typeof globalThis;
});
afterEach(() => {
  stop?.();
  stop = undefined;
  vi.restoreAllMocks();
});

describe('page text (U-3)', () => {
  it('has the texts, the About link and the referrer policy in the markup, no inline script or style', () => {
    expect(document.querySelector('h1')!.textContent).toBe('Workout Explorer');
    expect(document.body.textContent).toContain('Upload a FIT file to get a page with map, charts and laps that you can share.');
    expect(document.body.textContent).toContain('Drop a FIT file here');
    expect(document.querySelector('label.button')!.textContent).toBe('Choose a file');
    expect(document.querySelector('footer a')!.getAttribute('href')).toBe('/about');
    expect(document.querySelector('footer a')!.textContent).toBe('About');
    expect(document.querySelector('meta[name=referrer]')!.getAttribute('content')).toBe('no-referrer');
    expect(HTML).not.toMatch(/<script(?![^>]*\bsrc=)/);
    expect(HTML).not.toMatch(/<style|\sstyle=/);
    expect(input().hasAttribute('accept')).toBe(false);
  });
});

describe('status check (AC 4, U-5)', () => {
  it('shows no drop zone until the answer is in', async () => {
    statusReply = () => new Promise(() => {});
    await boot({ statusWaitMs: 1000 });
    expect(state()).toBe('checking');
    expect(input().disabled).toBe(true);
  });
  it('uploads_open true offers the drop zone', async () => {
    await boot();
    expect(state()).toBe('ready');
    expect(input().disabled).toBe(false);
    expect(fetchCalls).toEqual(['/api/status']);
  });
  it('uploads_open false shows the notice and no drop zone', async () => {
    statusReply = reply(200, { uploads_open: false });
    await boot();
    expect(state()).toBe('paused');
    expect($('notice').textContent).toBe('Uploads are paused because storage is full. Please try again later.');
    expect(input().disabled).toBe(true);
    pickFile(file(10));
    dropFiles(file(10));
    expect(FakeXhr.all).toHaveLength(0);
  });
  it.each([
    ['error status', reply(500, 'boom')],
    ['malformed body', reply(200, 'nope')],
    ['network error', () => Promise.reject(new TypeError('offline'))],
  ])('fails open on %s, with no message', async (_n, r) => {
    statusReply = r;
    await boot();
    expect(state()).toBe('ready');
    expect($('message').textContent).toBe('');
  });
  it('fails open when the answer is late', async () => {
    // the answer never comes; the abort must come from the timer, so this fetch honours the signal
    await boot({
      statusWaitMs: 10,
      fetch: (_u, init) =>
        new Promise((_res, rej) => init!.signal!.addEventListener('abort', () => rej(new DOMException('x', 'AbortError')))),
    });
    await new Promise((r) => setTimeout(r, 30));
    expect(state()).toBe('ready');
  });
  it('a back-forward-cache restore resets the page and checks again (AC 1)', async () => {
    await boot();
    pickFile(file(10));
    FakeXhr.all[0].respond(201, { edit_url: EDIT });
    expect(state()).toBe('processing');
    statusReply = reply(200, { uploads_open: false });
    const e = new Event('pageshow');
    Object.defineProperty(e, 'persisted', { value: true });
    win.dispatchEvent(e);
    await flush();
    expect(fetchCalls).toHaveLength(2);
    expect(state()).toBe('paused');
  });
});

describe('picking and the size rule (AC 2, AC 6)', () => {
  beforeEach(() => boot());
  it('refuses 25 MB in the browser with the message and sends no request', () => {
    pickFile(file(25_000_000));
    expect(FakeXhr.all).toHaveLength(0);
    expect($('message').textContent).toBe('This file is larger than 20 MB.');
    expect(state()).toBe('ready');
    dropFiles(file(25_000_000));
    expect(FakeXhr.all).toHaveLength(0);
  });
  it('sends exactly 20,000,000 bytes and refuses 20,000,001', () => {
    pickFile(file(20_000_001));
    expect(FakeXhr.all).toHaveLength(0);
    pickFile(file(20_000_000));
    expect(FakeXhr.all).toHaveLength(1);
    expect($('message').textContent).toBe('');
  });
  it('sends one multipart request with the field "file" to the relative URL, no content type set by hand', () => {
    const f = file(10);
    pickFile(f);
    const x = FakeXhr.all[0];
    expect([x.method, x.url]).toEqual(['POST', '/api/workouts']);
    expect(x.body!.get('file')).toBeInstanceOf(File);
    expect(Object.keys(x)).not.toContain('headers');
  });
  it('an empty file is sent; the server refuses it', () => {
    pickFile(file(0));
    expect(FakeXhr.all).toHaveLength(1);
  });
  it('clears the input so the same file can be picked again', () => {
    pickFile(file(25_000_000));
    expect(input().value).toBe('');
  });
});

describe('drop (U-6, AC 7 of F-11 scope: first file only, disabled while running)', () => {
  beforeEach(() => boot());
  it('prevents the browser from opening a dropped file, also while busy', () => {
    expect(dropFiles(file(10)).defaultPrevented).toBe(true);
    expect(dropFiles(file(10)).defaultPrevented).toBe(true);
    const over = new Event('dragover', { cancelable: true });
    document.dispatchEvent(over);
    expect(over.defaultPrevented).toBe(true);
  });
  it('uploads only the first of several dropped files', () => {
    dropFiles(file(10, 'a.fit'), file(10, 'b.fit'));
    expect(FakeXhr.all).toHaveLength(1);
    expect(($('message')).textContent).toBe('');
  });
  it('while an upload runs, the picker is disabled and a second drop or pick starts nothing', () => {
    dropFiles(file(10));
    expect(state()).toBe('uploading');
    expect(input().disabled).toBe(true);
    dropFiles(file(10));
    pickFile(file(10));
    expect(FakeXhr.all).toHaveLength(1);
    FakeXhr.all[0].upload.onload?.();
    expect(state()).toBe('processing');
    expect(input().disabled).toBe(true);
    dropFiles(file(10));
    expect(FakeXhr.all).toHaveLength(1);
  });
  it('after an error both picker and drop work again', () => {
    dropFiles(file(10));
    FakeXhr.all[0].respond(422, { error: 'not_fit', message: 'This file is not a FIT file.' });
    expect(input().disabled).toBe(false);
    dropFiles(file(10));
    expect(FakeXhr.all).toHaveLength(2);
  });
});

describe('progress (U-7)', () => {
  beforeEach(() => boot());
  it('shows the percentage while sending, then the processing state', () => {
    pickFile(file(10));
    expect($('progress').textContent).toBe('Uploading… 0 %');
    FakeXhr.all[0].upload.onprogress!({ lengthComputable: true, loaded: 42, total: 100 });
    expect($('progress').textContent).toBe('Uploading… 42 %');
    FakeXhr.all[0].upload.onload!();
    expect($('progress').textContent).toBe('Reading the workout…');
  });
});

describe('responses (AC 1, AC 3, AC 5, U-8, U-9)', () => {
  beforeEach(() => boot());
  it('201 opens the absolute edit link, no marker', () => {
    pickFile(file(10));
    FakeXhr.all[0].respond(201, { view_url: '/w/x', edit_url: EDIT });
    expect(navigated).toEqual([`${ORIGIN}${EDIT}`]);
  });
  it('201 with a foreign link navigates nowhere and shows the server error', () => {
    pickFile(file(10));
    FakeXhr.all[0].respond(201, { edit_url: `https://evil.example${EDIT}` });
    expect(navigated).toEqual([]);
    expect($('message').textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(state()).toBe('ready');
  });
  it.each([
    [422, 'not_fit', 'This file is not a FIT file.'],
    [422, 'unsupported_type', 'Unsupported FIT file type.'],
    [429, 'rate_limited', 'Too many uploads. Please try again later.'],
    [503, 'busy', 'Too many uploads right now. Please try again in a minute.'],
  ])('%i %s: the server message is shown and a new pick works', (status, error, message) => {
    pickFile(file(10));
    FakeXhr.all[0].respond(status, { error, message });
    expect($('message').textContent).toBe(message);
    expect(state()).toBe('ready');
    expect(input().disabled).toBe(false);
    pickFile(file(10));
    expect(FakeXhr.all).toHaveLength(2);
    expect($('message').textContent).toBe('');
  });
  it('503 storage_full switches to the paused notice', () => {
    pickFile(file(10));
    FakeXhr.all[0].respond(503, { error: 'storage_full', message: 'Uploads are paused because storage is full. Please try again later.' });
    expect(state()).toBe('paused');
    expect($('notice').textContent).toContain('Uploads are paused');
    expect(input().disabled).toBe(true);
  });
  it('a proxy error page shows the server error; a lost connection shows "could not be read"', () => {
    pickFile(file(10));
    FakeXhr.all[0].respond(502, '<html>Bad gateway</html>');
    expect($('message').textContent).toBe('Something went wrong on the server. Please try again later.');
    pickFile(file(10));
    FakeXhr.all[1].onerror!();
    expect($('message').textContent).toBe('The upload could not be read. Please try again.');
    expect(state()).toBe('ready');
  });
});

describe('privacy (AC 8, U-10)', () => {
  it('sets no cookie and writes no browser storage over an upload', async () => {
    const set = vi.spyOn(Storage.prototype, 'setItem');
    await boot();
    expect(document.cookie).toBe('');
    pickFile(file(10));
    FakeXhr.all[0].respond(201, { edit_url: EDIT });
    expect(document.cookie).toBe('');
    expect(set).not.toHaveBeenCalled();
    expect(localStorage.length + sessionStorage.length).toBe(0);
  });
  it('the sources use no cookie or storage API', () => {
    const forbidden = /document\.cookie|localStorage|sessionStorage|indexedDB|\bcaches\b|serviceWorker|cookieStore/;
    expect(Object.keys(sources).length).toBeGreaterThanOrEqual(3);
    for (const [f, text] of Object.entries(sources)) expect(text, f).not.toMatch(forbidden);
  });
});
