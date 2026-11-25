// The theme script in the heads of index.html and edit.html (DS-4, user 3 Nov 2026): applies the
// stored choice before the first paint. Allowed by its hash in the CSP (ADR-032 item 3), so its
// bytes must be the same in both pages and match the hash in deploy/Caddyfile.
import { describe, expect, it } from 'vitest';
import { readWebFile } from '../../test-support/source.mjs';
import { PREF_KEYS } from '../lib/prefs';
import { LANDSCAPE_QUERY } from './splitter';

const inline = (html: string): string[] => [...html.matchAll(/<script>([^<]*)<\/script>/g)].map((m) => m[1]);
const [script] = inline(readWebFile('index.html'));

interface Run {
  dataset: Record<string, string>;
  fetched: string[];
  timers: (() => void)[];
  win: { wxLang?: unknown };
  root: Record<string, string>;
}
/** Runs the script against stand-ins: storage, root element, the held meta's attributes, browser languages. */
function runAll(getItem: (k: string) => string | null, languages: readonly string[] | undefined = ['en'], meta: Record<string, string> | null = null, matchMedia: ((q: string) => { matches: boolean }) | null = () => ({ matches: false })): Run {
  const root: Record<string, string> = {};
  const html = { dataset: {} as Record<string, string>, style: { setProperty: (k: string, v: string) => void (root[k] = v) } };
  const r: Run = { dataset: html.dataset, fetched: [], timers: [], win: {}, root };
  const doc = { documentElement: html, querySelector: (q: string) => (q === 'meta[data-hold]' && meta ? { getAttribute: (a: string) => meta[a] ?? null } : null) };
  const nav = { languages, language: 'en-GB' };
  const fetch = (u: string) => (r.fetched.push(u), Promise.resolve('response'));
  const setTimeout = (f: () => void, ms: number) => {
    expect(ms).toBe(500);
    r.timers.push(f);
  };
  new Function('localStorage', 'document', 'navigator', 'window', 'fetch', 'setTimeout', 'matchMedia', script)({ getItem }, doc, nav, r.win, fetch, setTimeout, matchMedia);
  return r;
}
function attrs(getItem: (k: string) => string | null): Record<string, string> {
  const d = runAll(getItem).dataset;
  delete d.lang;
  return d;
}
const run = (getItem: (k: string) => string | null): string | undefined => attrs(getItem).theme;
const scriptSrc = (csp: string): string[] => / script-src ([^;]*);/.exec(csp)?.[1].split(' ') ?? [];

describe('theme head script', () => {
  it('is the same single script in both pages, with no template action', () => {
    expect(inline(readWebFile('index.html'))).toEqual([script]);
    expect(inline(readWebFile('edit.html'))).toEqual([script]);
    expect(inline(readWebFile('my-workouts.html'))).toEqual([script]); // F-53 MW-1
    // F-54 LG-2: the upload, About and generic pages hold for the language too.
    for (const f of ['upload.html', 'about.html', 'generic.html']) expect(inline(readWebFile(f)), f).toEqual([script]);
    expect(script).not.toMatch(/\{\{|\/\/|\/\*/); // html/template would rewrite actions or comments
  });

  it('its sha256 is in the CSP that Caddy sends, and in the caddy check', async () => {
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(script));
    const hash = `'sha256-${btoa(String.fromCharCode(...new Uint8Array(digest)))}'`;
    // F-39 HU-2: the previous script's hash may follow until no published page carries it.
    for (const file of ['../../deploy/Caddyfile', '../server/cmd/workout-explorer/caddy_test.go']) {
      const src = scriptSrc(readWebFile(file));
      expect(src.slice(0, 2)).toEqual(["'self'", hash]);
      expect(src).toContain("'sha256-z5SjeUWaAaA20n9xjEAw3BJBRbo/eoPKtoQHO3aO8bY='"); // F-73 RS-6: older hashes stay
      expect(src).toContain("'sha256-O55BZSok+26ER+X6MzVA5nes+xUDGDoeXwJKDKzdalw='");
    }
  });

  it('follows the viewport meta in every page (F-73 RS-5)', () => {
    for (const f of ['index.html', 'edit.html', 'upload.html', 'about.html', 'generic.html', 'my-workouts.html']) {
      const html = readWebFile(f);
      expect(html.indexOf('<meta name="viewport"'), f).toBeGreaterThan(-1);
      expect(html.indexOf('<meta name="viewport"'), f).toBeLessThan(html.indexOf('<script>'));
    }
  });

  // F-73 RS-1, RS-2: the stored split before the first paint (the bundle comparison is in splitter.dom.test.ts).
  describe('split', () => {
    const wide = (v: string | null) => (k: string) => (k === PREF_KEYS.wide ? v : k === PREF_KEYS.landscape ? '35' : null);
    const split = (getItem: (k: string) => string | null, landscape = false) => runAll(getItem, ['en'], null, () => ({ matches: landscape })).root['--map-w'];

    it('reads the layout key by the splitter query, from the prefs keys', () => {
      expect(script).toContain(LANDSCAPE_QUERY);
      expect(script).toContain(PREF_KEYS.wide);
      expect(script).toContain(PREF_KEYS.landscape);
      expect(split(wide('60'))).toBe('60%');
      expect(split(wide('60'), true)).toBe('35%');
      const asked: string[] = [];
      runAll(() => null, ['en'], null, (q) => (asked.push(q), { matches: false }));
      expect(asked).toEqual([LANDSCAPE_QUERY]);
    });

    it('applies only 30 to 70 in the prefs format', () => {
      for (const v of ['30', '60', '70', '45.5', '60.00']) expect(split(wide(v)), v).toBe(v + '%');
      for (const v of ['20', '29.99', '70.01', '80', 'abc', '50%', '1e2', '', null]) expect(split(wide(v)), String(v)).toBeUndefined();
    });

    it('is silent when storage or matchMedia fails, and theme, hint and language still apply', () => {
      const keys: Record<string, string> = { 'wx.theme': 'dark', 'wx.hint.done': '1', 'wx.lang': 'de', [PREF_KEYS.wide]: '60' };
      const r = runAll((k) => keys[k] ?? null, ['en'], null, null);
      expect(r.root).toEqual({});
      expect(r.dataset).toEqual({ theme: 'dark', hint: 'done', lang: 'de' });
      const throwing = runAll(() => {
        throw new Error('SecurityError');
      });
      expect(throwing.root).toEqual({});
    });
  });

  it('sets data-theme only for a stored light or dark, from the prefs key', () => {
    const store = (v: string | null) => (k: string) => (k === PREF_KEYS.theme ? v : null);
    expect(run(store('dark'))).toBe('dark');
    expect(run(store('light'))).toBe('light');
    for (const v of [null, 'system', 'Dark', 'dark ', '']) expect(run(store(v))).toBeUndefined();
  });

  it('sets data-hint="done" only for a stored "1", from the prefs key (F-39 HU-2)', () => {
    const store = (theme: string | null, hint: string | null) => (k: string) => (k === PREF_KEYS.theme ? theme : k === PREF_KEYS.hint ? hint : null);
    expect(attrs(store(null, '1'))).toEqual({ hint: 'done' });
    expect(attrs(store('dark', '1'))).toEqual({ theme: 'dark', hint: 'done' });
    for (const v of [null, '0', 'done', '1 ', '']) expect(attrs(store('light', v))).toEqual({ theme: 'light' });
  });

  it('is silent when storage throws', () => {
    expect(
      run(() => {
        throw new Error('SecurityError');
      }),
    ).toBeUndefined();
  });

  // F-54 LG-1, LG-2, AC 35, 38.
  describe('language', () => {
    const none = () => null;
    const stored = (v: string) => (k: string) => (k === 'wx.lang' ? v : null);
    const throwing = () => {
      throw new Error('SecurityError');
    };
    const files = { 'data-de': '/assets/lang-page-de-x.json', 'data-es': '/assets/lang-page-es-x.json', 'data-fr': '/assets/lang-page-fr-x.json' };

    it('takes the first browser language whose primary subtag is one of the four, else English', () => {
      expect(runAll(none, ['de-AT', 'en']).dataset.lang).toBe('de');
      expect(runAll(none, ['it', 'fr']).dataset.lang).toBe('fr');
      expect(runAll(none, ['it']).dataset.lang).toBe('en');
      expect(runAll(none, ['en-US', 'de']).dataset.lang).toBe('en');
      expect(runAll(none, ['ES-mx']).dataset.lang).toBe('es');
      expect(runAll(none, []).dataset.lang).toBe('en');
      expect(runAll(none, undefined).dataset.lang).toBe('en'); // navigator.language 'en-GB'
    });

    it('a stored valid choice wins; anything else reads as nothing stored', () => {
      expect(runAll(stored('fr'), ['de']).dataset.lang).toBe('fr');
      expect(runAll(stored('en'), ['de']).dataset.lang).toBe('en');
      for (const v of ['FR', 'it', 'de ', '']) expect(runAll(stored(v), ['es']).dataset.lang).toBe('es');
    });

    it('storage that throws skips only the stored step (AC 38)', () => {
      expect(runAll(throwing, ['de-CH']).dataset.lang).toBe('de');
    });

    it('holds and requests the one file of the held group for a language other than English, released after 500 ms', () => {
      const r = runAll(none, ['es'], files);
      expect(r.dataset.hold).toBe('');
      expect(r.fetched).toEqual(['/assets/lang-page-es-x.json']);
      expect(r.win.wxLang).toBeInstanceOf(Promise);
      expect(r.timers).toHaveLength(1);
      r.timers[0]();
      expect('hold' in r.dataset).toBe(false);
    });

    it('never holds or fetches for English or a page without a held group', () => {
      for (const r of [runAll(none, ['en'], files), runAll(none, ['de'], null)]) {
        expect(r.dataset.hold).toBeUndefined();
        expect(r.fetched).toEqual([]);
        expect(r.timers).toEqual([]);
      }
    });
  });
});
