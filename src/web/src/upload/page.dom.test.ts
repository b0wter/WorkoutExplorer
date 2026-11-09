// @vitest-environment jsdom
// The upload page's wiring (F-11 design U-5 to U-10; ADR-012). upload.html is loaded as it is;
// fetch, XMLHttpRequest and navigation are fakes. Synthetic data only.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import HTML from '../../upload.html?raw';
import { start, type Env } from './page';
import { syntheticFits, zip, type Entry } from '../../test-support/zip.mjs';

const sources = import.meta.glob(['./*.ts', '../lib/upload/*.ts', '../lib/zip/*.ts', '!./*.test.ts', '!../lib/*/*.test.ts'], {
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
// Both wait until the page has read the file's first bytes (F-35 Z-3).
async function pickFile(f: File) {
  Object.defineProperty(input(), 'files', { value: [f], configurable: true });
  input().dispatchEvent(new Event('change'));
  await flush();
}
async function dropFiles(...files: File[]): Promise<Event> {
  const e = new Event('drop', { bubbles: true, cancelable: true });
  Object.defineProperty(e, 'dataTransfer', { value: { files } });
  document.dispatchEvent(e);
  await flush();
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
    expect(document.body.textContent).toContain('Upload a FIT file, or a ZIP archive holding one, to get a page with map, charts and laps that you can share.');
    expect(document.body.textContent).toContain('Drop a FIT file or ZIP archive here');
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
    await pickFile(file(10));
    await dropFiles(file(10));
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
    await pickFile(file(10));
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
  it('refuses 25 MB in the browser with the message and sends no request', async () => {
    await pickFile(file(25_000_000));
    expect(FakeXhr.all).toHaveLength(0);
    expect($('message').textContent).toBe('This file is larger than 20 MB.');
    expect(state()).toBe('ready');
    await dropFiles(file(25_000_000));
    expect(FakeXhr.all).toHaveLength(0);
  });
  it('sends exactly 20,000,000 bytes and refuses 20,000,001', async () => {
    await pickFile(file(20_000_001));
    expect(FakeXhr.all).toHaveLength(0);
    await pickFile(file(20_000_000));
    expect(FakeXhr.all).toHaveLength(1);
    expect($('message').textContent).toBe('');
  });
  it('sends one multipart request with the field "file" to the relative URL, no content type set by hand', async () => {
    const f = file(10);
    await pickFile(f);
    const x = FakeXhr.all[0];
    expect([x.method, x.url]).toEqual(['POST', '/api/workouts']);
    expect(x.body!.get('file')).toBeInstanceOf(File);
    expect(Object.keys(x)).not.toContain('headers');
  });
  it('an empty file is sent; the server refuses it', async () => {
    await pickFile(file(0));
    expect(FakeXhr.all).toHaveLength(1);
  });
  it('clears the input so the same file can be picked again', async () => {
    await pickFile(file(25_000_000));
    expect(input().value).toBe('');
  });
});

describe('drop (U-6, AC 7 of F-11 scope: first file only, disabled while running)', () => {
  beforeEach(() => boot());
  it('prevents the browser from opening a dropped file, also while busy', async () => {
    expect((await dropFiles(file(10))).defaultPrevented).toBe(true);
    expect((await dropFiles(file(10))).defaultPrevented).toBe(true);
    const over = new Event('dragover', { cancelable: true });
    document.dispatchEvent(over);
    expect(over.defaultPrevented).toBe(true);
  });
  it('uploads only the first of several dropped files', async () => {
    await dropFiles(file(10, 'a.fit'), file(10, 'b.fit'));
    expect(FakeXhr.all).toHaveLength(1);
    expect(($('message')).textContent).toBe('');
  });
  it('while an upload runs, the picker is disabled and a second drop or pick starts nothing', async () => {
    await dropFiles(file(10));
    expect(state()).toBe('uploading');
    expect(input().disabled).toBe(true);
    await dropFiles(file(10));
    await pickFile(file(10));
    expect(FakeXhr.all).toHaveLength(1);
    FakeXhr.all[0].upload.onload?.();
    expect(state()).toBe('processing');
    expect(input().disabled).toBe(true);
    await dropFiles(file(10));
    expect(FakeXhr.all).toHaveLength(1);
  });
  it('after an error both picker and drop work again', async () => {
    await dropFiles(file(10));
    FakeXhr.all[0].respond(422, { error: 'not_fit', message: 'This file is not a FIT file.' });
    expect(input().disabled).toBe(false);
    await dropFiles(file(10));
    expect(FakeXhr.all).toHaveLength(2);
  });
});

describe('progress (U-7)', () => {
  beforeEach(() => boot());
  it('shows the percentage while sending, then the processing state', async () => {
    await pickFile(file(10));
    expect($('progress').textContent).toBe('Uploading… 0 %');
    FakeXhr.all[0].upload.onprogress!({ lengthComputable: true, loaded: 42, total: 100 });
    expect($('progress').textContent).toBe('Uploading… 42 %');
    FakeXhr.all[0].upload.onload!();
    expect($('progress').textContent).toBe('Reading the workout…');
  });
});

describe('responses (AC 1, AC 3, AC 5, U-8, U-9)', () => {
  beforeEach(() => boot());
  it('201 opens the absolute edit link, no marker', async () => {
    await pickFile(file(10));
    FakeXhr.all[0].respond(201, { view_url: '/w/x', edit_url: EDIT });
    expect(navigated).toEqual([`${ORIGIN}${EDIT}`]);
  });
  it('201 with a foreign link navigates nowhere and shows the server error', async () => {
    await pickFile(file(10));
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
  ])('%i %s: the server message is shown and a new pick works', async (status, error, message) => {
    await pickFile(file(10));
    FakeXhr.all[0].respond(status, { error, message });
    expect($('message').textContent).toBe(message);
    expect(state()).toBe('ready');
    expect(input().disabled).toBe(false);
    await pickFile(file(10));
    expect(FakeXhr.all).toHaveLength(2);
    expect($('message').textContent).toBe('');
  });
  it('503 storage_full switches to the paused notice', async () => {
    await pickFile(file(10));
    FakeXhr.all[0].respond(503, { error: 'storage_full', message: 'Uploads are paused because storage is full. Please try again later.' });
    expect(state()).toBe('paused');
    expect($('notice').textContent).toContain('Uploads are paused');
    expect(input().disabled).toBe(true);
  });
  it('a proxy error page shows the server error; a lost connection shows "could not be read"', async () => {
    await pickFile(file(10));
    FakeXhr.all[0].respond(502, '<html>Bad gateway</html>');
    expect($('message').textContent).toBe('Something went wrong on the server. Please try again later.');
    await pickFile(file(10));
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
    await pickFile(file(10));
    FakeXhr.all[0].respond(201, { edit_url: EDIT });
    expect(document.cookie).toBe('');
    expect(set).not.toHaveBeenCalled();
    expect(localStorage.length + sessionStorage.length).toBe(0);
  });
  it('the sources use no cookie or storage API', () => {
    const forbidden = /document\.cookie|localStorage|sessionStorage|indexedDB|\bcaches\b|serviceWorker|cookieStore/;
    expect(Object.keys(sources).length).toBeGreaterThanOrEqual(4);
    for (const [f, text] of Object.entries(sources)) expect(text, f).not.toMatch(forbidden);
  });
});

describe('zip archives (F-35)', () => {
  const FIT = syntheticFits()[0].bytes;
  const archive = (entries: Entry[], name = 'export.zip') => new File([zip(entries).bytes], name);
  const sent = () => FakeXhr.all[0].body!.get('file') as File;
  const bytesOf = async (f: File) => new Uint8Array(await f.arrayBuffer());
  const until = async (ok: () => boolean) => {
    for (const t = Date.now(); !ok() && Date.now() - t < 8000; ) await flush();
  };
  const Real = globalThis.DecompressionStream;
  beforeEach(() => boot());
  afterEach(() => {
    globalThis.DecompressionStream = Real;
  });

  it('a FIT file is sent as the picked File itself', async () => {
    const f = new File([FIT], 'ride.fit');
    await pickFile(f);
    expect(sent()).toBe(f);
  });
  it.each(['export.zip', 'workout.fit', 'ride'])('an archive named %s is opened; only the FIT bytes go out, as workout.fit', async (name) => {
    await pickFile(archive([{ name: 'ride.fit', data: FIT, descriptor: 'sig' }, { name: 'notes.txt' }], name));
    await until(() => FakeXhr.all.length > 0);
    expect(FakeXhr.all).toHaveLength(1);
    expect(sent().name).toBe('workout.fit');
    expect(await bytesOf(sent())).toEqual(FIT);
    expect(state()).toBe('uploading');
  });
  it('a FIT entry of exactly 20,000,000 bytes is sent', async () => {
    await pickFile(archive([{ name: 'ride.fit', data: new Uint8Array(20_000_000).fill(1) }]));
    await until(() => FakeXhr.all.length > 0);
    expect(sent().size).toBe(20_000_000);
  }, 15_000);
  it.each([
    ['no root FIT file', [{ name: 'sub/ride.fit' }], 'This archive contains no FIT file.'],
    ['an empty archive', [], 'This archive contains no FIT file.'],
    ['two root FIT files', [{ name: 'a.fit' }, { name: 'b.fit' }], 'This archive contains more than one FIT file. Please upload one at a time.'],
    ['an encrypted FIT entry', [{ name: 'ride.fit', flags: 1 }], 'This archive could not be opened. Please unzip it and upload the FIT file.'],
    ['a FIT entry that is a zip', [{ name: 'ride.fit', data: zip([{ name: 'x.fit' }]).bytes }], 'This archive could not be opened. Please unzip it and upload the FIT file.'],
    ['a FIT entry over the limit', [{ name: 'ride.fit', data: new Uint8Array(20_000_001) }], 'The FIT file in this archive is larger than 20 MB.'],
  ] as [string, Entry[], string][])('%s: message, no request, usable again', async (_n, entries, message) => {
    await pickFile(archive(entries));
    await until(() => state() === 'ready');
    expect($('message').textContent).toBe(message);
    expect(FakeXhr.all).toHaveLength(0);
    expect(input().disabled).toBe(false);
    await pickFile(new File([FIT], 'ride.fit'));
    expect(FakeXhr.all).toHaveLength(1);
  });
  it('the first part of a split archive is refused in the browser', async () => {
    const b = zip([{ name: 'ride.fit', data: FIT }]).bytes;
    new DataView(b.buffer).setUint32(0, 0x08074b50, true);
    await pickFile(new File([b], 'ride.z01'));
    await until(() => state() === 'ready');
    expect($('message').textContent).toBe('This archive could not be opened. Please unzip it and upload the FIT file.');
    expect(FakeXhr.all).toHaveLength(0);
  });
  it('an archive over 20,000,000 bytes is refused before it is read', async () => {
    const f = archive([{ name: 'ride.fit', data: FIT }]);
    Object.defineProperty(f, 'size', { value: 20_000_001 });
    const read = vi.spyOn(f, 'slice');
    await pickFile(f);
    expect($('message').textContent).toBe('This file is larger than 20 MB.');
    expect(read).not.toHaveBeenCalled();
  });
  it('without DecompressionStream a zip gets the browser message and a FIT file still uploads', async () => {
    // @ts-expect-error removing the global, as an old browser
    delete globalThis.DecompressionStream;
    await pickFile(archive([{ name: 'ride.fit', data: FIT, method: 0 }]));
    expect($('message').textContent).toBe('This browser cannot open ZIP archives. Please unzip it and upload the FIT file.');
    expect(FakeXhr.all).toHaveLength(0);
    await pickFile(new File([FIT], 'ride.fit'));
    expect(FakeXhr.all).toHaveLength(1);
  });
  it('busy from the first byte read through extraction and upload', async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    const f = archive([{ name: 'ride.fit', data: FIT }]);
    const full = f.arrayBuffer.bind(f);
    vi.spyOn(f, 'arrayBuffer').mockImplementation(async () => (await gate, full()));
    Object.defineProperty(input(), 'files', { value: [f], configurable: true });
    input().dispatchEvent(new Event('change'));
    expect(state()).toBe('opening');
    expect(input().disabled).toBe(true);
    await flush();
    expect($('progress').textContent).toBe('Opening the archive…');
    await dropFiles(new File([FIT], 'other.fit'));
    expect(FakeXhr.all).toHaveLength(0);
    release();
    await until(() => FakeXhr.all.length > 0);
    expect(state()).toBe('uploading');
    expect(input().disabled).toBe(true);
  });
  it('a file that cannot be read shows "could not be read"', async () => {
    const f = new File([FIT], 'ride.fit');
    vi.spyOn(f, 'slice').mockImplementation(() => ({ arrayBuffer: () => Promise.reject(new DOMException('gone', 'NotReadableError')) }) as unknown as Blob);
    await pickFile(f);
    expect($('message').textContent).toBe('The upload could not be read. Please try again.');
    expect(state()).toBe('ready');
  });
  it('a back-forward-cache restore drops a running extraction', async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    const f = archive([{ name: 'ride.fit', data: FIT }]);
    const full = f.arrayBuffer.bind(f);
    vi.spyOn(f, 'arrayBuffer').mockImplementation(async () => (await gate, full()));
    await pickFile(f);
    const e = new Event('pageshow');
    Object.defineProperty(e, 'persisted', { value: true });
    win.dispatchEvent(e);
    await flush();
    release();
    for (let i = 0; i < 20; i++) await flush();
    expect(FakeXhr.all).toHaveLength(0);
    expect(state()).toBe('ready');
    expect($('message').textContent).toBe('');
  });
  it('nothing from an archive is stored', async () => {
    const set = vi.spyOn(Storage.prototype, 'setItem');
    await pickFile(archive([{ name: 'ride.fit', data: FIT }]));
    await until(() => FakeXhr.all.length > 0);
    expect(set).not.toHaveBeenCalled();
    expect(document.cookie).toBe('');
  });
});
