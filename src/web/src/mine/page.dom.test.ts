// @vitest-environment jsdom
// The My workouts page (F-53 AC 43 to 47, 50, 52 to 56; MW-4, MW-5, MW-10). Critical: stored text
// only as text, links only from checked parts, no request.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LIST_KEY } from '../lib/mine';
import { readWebFile } from '../../test-support/source.mjs';
import { relative, show, startMine } from '.';
import { en } from './texts';

const ID = 'AbCdEfGhIjKlMnOpQrStUv';
const TOKEN = 'Zz0123456789abcdefghij';
const H = 3600_000;
const NOW = Date.parse('2026-11-20T12:00:00Z');
const iso = (ms: number) => new Date(ms).toISOString();
const entry = (over: Record<string, unknown> = {}) => ({ id: ID, token: TOKEN, title: '', sport: 'cycling', date: '2026-11-19', created: iso(NOW - H), expires: iso(NOW + 30 * 24 * H), published: false, ...over });
const put = (...workouts: unknown[]) => localStorage.setItem(LIST_KEY, JSON.stringify({ v: 1, workouts }));

let host: HTMLElement;
const fetchSpy = vi.fn();
beforeEach(() => {
  vi.stubGlobal('fetch', fetchSpy);
  document.body.innerHTML = '<template id="wx-impressum"><a href="/impressum">Impressum</a></template><div id="wx-mine"></div>';
  host = document.getElementById('wx-mine')!;
});
afterEach(() => {
  expect(fetchSpy).not.toHaveBeenCalled(); // AC 47
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});
const start = () => show(host, () => localStorage, () => NOW);
const cells = () => [...host.querySelectorAll('tbody tr')].map((tr) => [...tr.children].map((td) => td.textContent));
const click = (sel: string) => host.querySelector<HTMLElement>(sel)!.click();

describe('the list', () => {
  it('columns, newest upload first, default and suggested titles, relative expiry; note and upload link', () => {
    put(
      entry({ id: 'A'.repeat(22), created: iso(NOW - 3 * H), title: 'Morning ride' }),
      entry({ published: true, expires: iso(NOW + 3 * 24 * H) }),
      entry({ id: 'B'.repeat(22), created: iso(NOW - 47 * H), title: '  ', sport: 'running', date: undefined }), // draft: 1 h left
    );
    start();
    expect(document.title).toBe('My workouts · Workout Explorer');
    expect([...host.querySelectorAll('th')].map((t) => t.textContent)).toEqual([en.colWorkout, en.colSport, en.colDate, en.colStatus, en.colExpires, en.colLinks, '']);
    expect(cells()).toEqual([
      ['Ride, 19 Nov 2026', 'Ride', '19 Nov 2026', 'Published', 'in 3 days', 'EditView', 'Forget'],
      ['Morning ride', 'Ride', '19 Nov 2026', 'Draft', 'in 45 hours', 'Edit', 'Forget'], // 48 h after upload
      ['Run', 'Run', '–', 'Draft', 'in 1 hour', 'Edit', 'Forget'],
    ]);
    const links = [...host.querySelectorAll<HTMLAnchorElement>('tbody a')];
    expect(links[0].getAttribute('href')).toBe('/edit/' + ID + '#' + TOKEN);
    expect(links[1].getAttribute('href')).toBe('/w/' + ID);
    for (const a of host.querySelectorAll('a')) {
      expect(a.hasAttribute('target')).toBe(false); // same tab (AC 43)
      expect(a.getAttribute('href')).toMatch(/^\/(edit\/|w\/|about$|impressum$|$)/);
    }
    expect(host.textContent).toContain(en.mineNote);
    expect(host.querySelector('main a[href="/"]')?.textContent).toBe('Upload a workout');
    expect(host.querySelector('footer a[href="/about"]')).not.toBeNull();
    expect(host.querySelector('footer a[href="/impressum"]')).not.toBeNull();
  });

  it('empty: the sentence, the upload link, no Clear list', () => {
    start();
    expect(host.querySelector('main')!.textContent).toContain('No workouts uploaded from this browser yet.');
    expect(host.querySelector('[data-act="clear"], table')).toBeNull();
    expect(host.querySelector('main a[href="/"]')).not.toBeNull();
  });

  it('drops expired entries when shown (AC 47)', () => {
    put(entry({ created: iso(NOW - 49 * H) }), entry({ id: 'B'.repeat(22) }));
    start();
    expect(cells()).toHaveLength(1);
    expect(JSON.parse(localStorage.getItem(LIST_KEY)!).workouts).toHaveLength(1);
  });

  it('relative expiry picks the unit', () => {
    expect([relative(10_000), relative(30 * 60_000), relative(5 * H), relative(47 * H), relative(49 * H)]).toEqual(['in 1 minute', 'in 30 minutes', 'in 5 hours', 'in 47 hours', 'in 2 days']);
  });
});

describe('hostile values (AC 53, 54)', () => {
  it('markup and control characters stay text; foreign or script links are not built', () => {
    const title = '<img src=x onerror=alert(1)><script>alert(2)</script>\u0000‮"\'';
    put(
      entry({ title, id: 'javascript:alert(1)', published: true }),
      entry({ id: 'B'.repeat(22), token: 'https://evil.example/', sport: '<b>x</b>', date: 'javascript:1', created: iso(NOW - 2 * H), edit: 'javascript:alert(1)', url: '//evil.example' }),
    );
    start();
    expect(host.querySelector('img, script, b')).toBeNull();
    expect(cells()[0]).toEqual([title, 'Ride', '19 Nov 2026', 'Published', 'in 30 days', '', 'Forget']);
    expect(cells()[1].slice(0, 3)).toEqual(['<b>x</b>', '<b>x</b>', '–']);
    expect(host.querySelectorAll('tbody a')).toHaveLength(0);
    expect(host.innerHTML).not.toMatch(/javascript:|evil\.example/);
  });
});

describe('Forget and Clear list (AC 50)', () => {
  it('Forget removes one entry at once', () => {
    put(entry(), entry({ id: 'B'.repeat(22), created: iso(NOW) }));
    start();
    click('[data-act="forget:1"]');
    expect(JSON.parse(localStorage.getItem(LIST_KEY)!).workouts.map((e: { id: string }) => e.id)).toEqual(['B'.repeat(22)]);
    expect(cells()).toHaveLength(1);
  });

  it('Clear list asks once; Cancel keeps, confirm removes the key', () => {
    put(entry());
    start();
    click('[data-act="clear"]');
    expect(host.querySelector('.mine-confirm')!.textContent).toContain(
      'Remove all workouts from this list? They stay online, but without their edit links you cannot change or delete them.',
    );
    click('[data-act="cancel"]');
    expect(cells()).toHaveLength(1);
    click('[data-act="clear"]');
    click('[data-act="confirm"]');
    expect(localStorage.getItem(LIST_KEY)).toBeNull();
    expect(host.textContent).toContain(en.mineEmpty);
  });
});

describe('storage unavailable (AC 52)', () => {
  it.each([
    ['unreadable', () => localStorage.setItem(LIST_KEY, '{"v":1,"workouts":'), () => localStorage],
    ['full', () => vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('full', 'QuotaExceededError'); }), () => localStorage],
    ['blocked', () => {}, () => { throw new DOMException('blocked', 'SecurityError'); }],
    ['missing', () => {}, () => null],
  ])('%s: the sentence, no error, the unreadable value untouched', (_, prep, store) => {
    prep();
    const before = localStorage.getItem(LIST_KEY);
    show(host, store as () => Storage, () => NOW);
    expect(host.querySelector('main')!.textContent).toContain('The list is not available in this browser.');
    expect(host.querySelector('table')).toBeNull();
    expect(localStorage.getItem(LIST_KEY)).toBe(before);
  });
});

describe('startMine', () => {
  it('renders once its stylesheet has loaded or failed', () => {
    startMine(host, () => localStorage, () => NOW);
    expect(host.querySelector('h1')).toBeNull();
    const link = document.head.querySelector<HTMLLinkElement>('link[rel="stylesheet"]')!;
    link.dispatchEvent(new Event('error'));
    expect(host.querySelector('h1')?.textContent).toBe('My workouts');
    link.remove();
  });
});

describe('the shell (MW-1, AC 56)', () => {
  const html = readWebFile('my-workouts.html');
  it('noscript line, theme script as edit.html, no Open Graph card, marker', () => {
    expect(html).toMatch(/<div id="wx-mine"><noscript><p>[^<]*needs JavaScript[^<]*<\/p><\/noscript><\/div>/);
    const inline = (s: string) => [...s.matchAll(/<script>([^<]*)<\/script>/g)].map((m) => m[1]);
    expect(inline(html)).toEqual(inline(readWebFile('edit.html')));
    expect(html).not.toMatch(/og:|twitter:/);
    expect(html).toContain('<meta name="referrer" content="no-referrer" />');
    expect(html).toContain('<meta name="robots" content="noindex, nofollow, noarchive" />');
    expect(html.match(/<!--wx:impressum-->/g)).toHaveLength(1);
  });
});
