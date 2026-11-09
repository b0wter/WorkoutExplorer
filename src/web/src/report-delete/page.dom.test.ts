// @vitest-environment jsdom
// The delete link's confirmation page (F-37 DK-6, DK-7, DK-11, DK-12; AC 30, 31, 35).
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { keyOf, start } from './page';

const KEY = 'AbCdEfGhIjKlMnOpQrStUv';
const INVALID = 'This delete link is no longer valid: it was already used, has expired, or the workout is gone.';
const LOOKUP = {
  title: 'Morning ride',
  start_date: 'Sun, 8 Nov 2026',
  report_id: 7,
  reason: 'Personal data',
  filed_at: 1_793_000_000,
  expires_at: 1_793_500_000,
  report_count: 2,
};
const json = (status: number, body: unknown): Response =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

let replies: (() => Promise<Response>)[];
let fetchFn: ReturnType<typeof vi.fn<(input: string, init?: RequestInit) => Promise<Response>>>;

beforeEach(() => {
  document.body.innerHTML = `<main id="page"><dl id="facts"></dl><p id="message"></p><button id="delete"></button></main>`;
  replies = [];
  fetchFn = vi.fn((_input: string, _init?: RequestInit) => replies.shift()!());
});
afterEach(() => {
  document.body.innerHTML = '';
});

const $ = (id: string): HTMLElement => document.getElementById(id)!;
const state = (): string | undefined => $('page').dataset.state;
const settle = async (): Promise<void> => {
  for (let k = 0; k < 5; k++) await new Promise((r) => setTimeout(r, 0));
};

describe('keyOf', () => {
  it('reads the fragment up to the first &', () => {
    expect(keyOf('#' + KEY)).toBe(KEY);
    expect(keyOf('#' + KEY + '&x=1')).toBe(KEY);
  });
  it('refuses malformed keys', () => {
    for (const h of ['', '#', '#' + KEY.slice(1), '#' + KEY + 'x', '#' + KEY.slice(1) + '-', '#' + KEY.slice(1) + '%'])
      expect(keyOf(h)).toBeNull();
  });
});

describe('confirmation page', () => {
  it('shows the invalid message without any request for a malformed fragment', async () => {
    await start(document, '#short', fetchFn);
    expect(fetchFn).not.toHaveBeenCalled();
    expect(state()).toBe('failed');
    expect($('message').textContent).toBe(INVALID);
  });

  it('looks up with the key in the header only and shows every fact; opening never posts', async () => {
    replies.push(async () => json(200, LOOKUP));
    await start(document, '#' + KEY, fetchFn);
    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/report-delete');
    expect(init).toMatchObject({
      method: 'GET',
      headers: { Authorization: 'Bearer ' + KEY },
      credentials: 'same-origin',
      redirect: 'error',
      referrerPolicy: 'no-referrer',
    });
    expect(init.body).toBeUndefined();
    expect(state()).toBe('ready');
    const text = $('facts').textContent!;
    for (const s of ['Morning ride', 'Sun, 8 Nov 2026', '#7: Personal data', '2']) expect(text).toContain(s);
    expect($('facts').querySelectorAll('dd')).toHaveLength(6);
    expect(text).toContain(new Date(LOOKUP.filed_at * 1000).getFullYear().toString());
  });

  it('shows a 404 message as is', async () => {
    replies.push(async () => json(404, { error: 'not_found', message: INVALID }));
    await start(document, '#' + KEY, fetchFn);
    expect(state()).toBe('failed');
    expect($('message').textContent).toBe(INVALID);
  });

  it('deletes only on a click, once, and says so', async () => {
    replies.push(async () => json(200, LOOKUP));
    await start(document, '#' + KEY, fetchFn);
    let finish!: (r: Response) => void;
    replies.push(() => new Promise((r) => (finish = r)));
    $('delete').click();
    $('delete').click(); // disabled while the request runs: no second POST
    expect(fetchFn).toHaveBeenCalledTimes(2);
    const init = fetchFn.mock.calls[1][1] as RequestInit;
    expect(init).toMatchObject({ method: 'POST', headers: { Authorization: 'Bearer ' + KEY }, redirect: 'error' });
    expect(init.body).toBeUndefined();
    expect(($('delete') as HTMLButtonElement).disabled).toBe(true);
    finish(new Response(null, { status: 204 }));
    await settle();
    expect(state()).toBe('deleted');
    expect($('message').textContent).toBe('The workout has been deleted.');
    expect($('facts').childElementCount).toBe(0);
  });

  it('a spent link on confirm shows the server sentence; a 429 lets the operator try again', async () => {
    replies.push(async () => json(200, LOOKUP));
    await start(document, '#' + KEY, fetchFn);
    replies.push(async () => json(429, { error: 'rate_limited', message: 'Too many requests. Please try again later.' }));
    $('delete').click();
    await settle();
    expect(state()).toBe('ready');
    expect($('message').textContent).toBe('Too many requests. Please try again later.');
    replies.push(async () => json(404, { error: 'not_found', message: INVALID }));
    $('delete').click();
    await settle();
    expect(state()).toBe('failed');
    expect($('message').textContent).toBe(INVALID);
  });

  it('a network failure or a bad body never shows facts', async () => {
    replies.push(async () => {
      throw new TypeError('offline');
    });
    await start(document, '#' + KEY, fetchFn);
    expect(state()).toBe('failed');
    replies.push(async () => json(200, { title: 'x' }));
    await start(document, '#' + KEY, fetchFn);
    expect(state()).toBe('failed');
    expect($('facts').childElementCount).toBe(0);
  });
});
