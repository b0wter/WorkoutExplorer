// @vitest-environment jsdom
// The Impressum page (F-38 IM-2, IM-3; AC 4, AC 7, AC 8).
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { start } from './page';
import html from '../../impressum.html?raw';

const FAILED = 'The Impressum could not be loaded. Reload the page.';
const FRAGMENT = '<h2>Kontakt</h2><p>&lt;b&gt;x&lt;/b&gt;</p><ul><li>a</li></ul>';

let replies: (() => Promise<Response>)[];
let fetchFn: ReturnType<typeof vi.fn<(input: string, init?: RequestInit) => Promise<Response>>>;

beforeEach(() => {
  document.documentElement.innerHTML = html.replace(/^<!doctype html>/i, '');
  replies = [];
  fetchFn = vi.fn((_input: string, _init?: RequestInit) => replies.shift()!());
});

const $ = (id: string): HTMLElement | null => document.getElementById(id);
const button = (): HTMLButtonElement => $('show') as HTMLButtonElement;
const settle = async (): Promise<void> => {
  for (let k = 0; k < 5; k++) await new Promise((r) => setTimeout(r, 0));
};

describe('impressum.html', () => {
  it('has the heading, a hidden button, the noscript note, noindex and no card tags', () => {
    expect(document.querySelector('h1')!.textContent).toBe('Impressum / Legal notice');
    expect(button().hidden).toBe(true);
    expect(html).toContain('<noscript><p>JavaScript is needed to show the Impressum.</p></noscript>');
    expect(html).toContain('<meta name="robots" content="noindex, nofollow, noarchive" />');
    expect(html).not.toMatch(/og:|twitter:/);
    expect(html.match(/<!--wx:impressum-->/g)).toHaveLength(1);
    expect(html).not.toMatch(/<script(?![^>]*\ssrc=)/); // CSP: no inline script
  });
});

describe('Show Impressum', () => {
  it('shows the button and fetches nothing on load', () => {
    start(document, fetchFn);
    expect(button().hidden).toBe(false);
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it('fetches once with the exact options and replaces the button with the text', async () => {
    replies.push(async () => new Response(FRAGMENT, { status: 200 }));
    start(document, fetchFn);
    button().click();
    expect(button().disabled).toBe(true);
    await settle();
    expect(fetchFn).toHaveBeenCalledTimes(1);
    expect(fetchFn).toHaveBeenCalledWith('/api/impressum', {
      headers: { 'Wx-Impressum': '1' },
      mode: 'same-origin',
      credentials: 'same-origin',
      referrerPolicy: 'no-referrer',
      cache: 'no-store',
      redirect: 'error',
    });
    expect($('show')).toBeNull();
    expect($('text')!.innerHTML).toBe(FRAGMENT);
    expect($('text')!.querySelector('b')).toBeNull();
    expect($('message')!.textContent).toBe('');
  });

  it('sends one request for a double click', async () => {
    replies.push(async () => new Response(FRAGMENT, { status: 200 }));
    start(document, fetchFn);
    button().click();
    button().click();
    await settle();
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });

  for (const [name, reply] of [
    ['a 429', async () => new Response('', { status: 429 })],
    ['a 404', async () => new Response('404 page not found', { status: 404 })],
    ['a network error', async () => Promise.reject(new TypeError('Failed to fetch'))],
  ] as const) {
    it(`shows the failure message and the button again after ${name}, and retries on the next click`, async () => {
      replies.push(reply, async () => new Response(FRAGMENT, { status: 200 }));
      start(document, fetchFn);
      button().click();
      await settle();
      expect($('message')!.textContent).toBe(FAILED);
      expect(button().hidden).toBe(false);
      expect(button().disabled).toBe(false);
      expect($('text')!.innerHTML).toBe('');
      button().click();
      await settle();
      expect(fetchFn).toHaveBeenCalledTimes(2);
      expect($('text')!.innerHTML).toBe(FRAGMENT);
      expect($('message')!.textContent).toBe('');
    });
  }
});
