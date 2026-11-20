// @vitest-environment jsdom
// The i18n core and loader (F-54 LG-1 to LG-7): formatter, lookup, the first pass under the hold,
// a switch, lazy groups, the markup applier.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { format, lang, loadGroup, onLanguage, pageLang, resetI18n, texts } from '.';
import { applyMarkup, setLang, startLang } from './load';

const files: Record<string, unknown> = {
  '/de/page.json': { a: 'A-de', n: { one: '{n} Pin', other: '{n} Pins' }, bad: 5, k: 'Schlüssel {x}' },
  '/fr/page.json': { a: 'A-fr' },
  '/de/help.json': { h: 'Hilfe' },
};
let fetched: string[];
const html = document.documentElement;

beforeEach(() => {
  resetI18n();
  fetched = [];
  vi.stubGlobal('fetch', (u: string) => {
    fetched.push(u);
    return Promise.resolve(u in files ? Response.json(files[u]) : new Response('x', { status: 404 }));
  });
  document.head.innerHTML =
    '<meta name="wx-lang-page" data-hold data-de="/de/page.json" data-es="/es/page.json" data-fr="/fr/page.json"><meta name="wx-lang-help" data-de="/de/help.json" data-es="/es/help.json" data-fr="/fr/help.json">';
  delete html.dataset.hold;
  html.dataset.lang = 'de';
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  delete (window as { wxLang?: unknown }).wxLang;
});

const t = texts({ a: 'A', n: { one: '{n} pin', other: '{n} pins' }, bad: 'Bad', k: 'Key {x}', h: 'Help' });

describe('format and lookup', () => {
  it('fills placeholders as text, keeps unknown ones, picks the plural category', () => {
    expect(format('a {x} {y}', { x: '<b>' })).toBe('a <b> {y}');
    expect(t('n', { n: 1 })).toBe('1 pin');
    expect(t('n', { n: 2 })).toBe('2 pins');
    expect(format({ one: 'one', other: 'other' })).toBe('other');
  });

  it('pageLang reads <html data-lang>; anything else is English', () => {
    expect(pageLang()).toBe('de');
    html.dataset.lang = 'it';
    expect(pageLang()).toBe('en');
  });
});

describe('first pass (LG-2)', () => {
  it('applies the file while the hold is set, then lifts it; reuses the head script request', async () => {
    html.dataset.hold = '';
    (window as { wxLang?: Promise<Response> }).wxLang = Promise.resolve(Response.json(files['/de/page.json']));
    const apply = vi.fn();
    expect(await startLang(['page'], document, Promise.resolve(), apply)).toBe(true);
    expect(fetched).toEqual([]); // no second download
    expect(apply).toHaveBeenCalledOnce();
    expect(lang()).toBe('de');
    expect(html.lang).toBe('de');
    expect('hold' in html.dataset).toBe(false);
    expect(t('a')).toBe('A-de');
    expect(t('n', { n: 3 })).toBe('3 Pins');
    expect(t('bad')).toBe('Bad'); // not a text: English
    expect(t('h')).toBe('Help'); // not in the file: English
  });

  it('a file arriving after the hold was lifted (500 ms) is not applied: this view stays English', async () => {
    html.dataset.hold = '';
    let ready!: () => void;
    const late = new Promise<void>((r) => (ready = r));
    const apply = vi.fn();
    const pass = startLang(['page'], document, late, apply);
    delete html.dataset.hold; // the head script's timer
    ready();
    expect(await pass).toBe(false);
    expect(apply).not.toHaveBeenCalled();
    expect(lang()).toBe('en');
    expect(t('a')).toBe('A');
  });

  it('a failed file leaves the page English, without an error (AC 56)', async () => {
    html.dataset.lang = 'es';
    html.dataset.hold = '';
    expect(await startLang(['page'], document, undefined, () => {})).toBe(true);
    expect(fetched).toEqual(['/es/page.json']);
    expect(t('a')).toBe('A');
  });

  it('English: no request, no hold', async () => {
    html.dataset.lang = 'en';
    expect(await startLang(['page'])).toBe(false);
    expect(fetched).toEqual([]);
  });

  it('a page without a hold applies within 500 ms only', async () => {
    vi.useFakeTimers();
    let ready!: () => void;
    const pass = startLang(['page'], document, new Promise<void>((r) => (ready = r)));
    await vi.advanceTimersByTimeAsync(501);
    ready();
    expect(await pass).toBe(false);
    expect(lang()).toBe('en');
  });
});

describe('switch and lazy groups (LG-1, LG-3)', () => {
  it('a switch loads every group in use, then tells the listeners; English needs no file', async () => {
    html.dataset.lang = 'en';
    await startLang(['page']);
    await loadGroup('help');
    expect(fetched).toEqual([]);
    const heard = vi.fn(() => lang());
    onLanguage(heard);
    await setLang('de');
    expect(fetched.sort()).toEqual(['/de/help.json', '/de/page.json']);
    expect(heard).toHaveReturnedWith('de');
    expect([t('a'), t('h')]).toEqual(['A-de', 'Hilfe']);
    await setLang('en');
    expect([t('a'), t('h')]).toEqual(['A', 'Help']);
  });

  it('a lazy group loads in the current language and joins', async () => {
    html.dataset.hold = '';
    await startLang(['page'], document);
    await loadGroup('help');
    expect(t('h')).toBe('Hilfe');
  });

  it('the later of two quick switches wins', async () => {
    html.dataset.lang = 'en';
    await startLang(['page']);
    const a = setLang('de');
    const b = setLang('fr');
    await Promise.all([a, b]);
    expect(lang()).toBe('fr');
    expect(t('a')).toBe('A-fr');
  });
});

describe('applyMarkup (LG-4)', () => {
  it('rewrites text and attributes, keeps placed children, goes back to the markup English; never parses HTML', async () => {
    files['/de/page.json'] = { p: '{link}: <b>fett</b>', l: 'Meine', ti: 'Titel' };
    document.body.innerHTML = '<p data-t="p"><a href="/x" data-p="link" data-t="l">Mine</a>: the list</p><button title="Title" data-t-title="ti">x</button>';
    html.dataset.hold = '';
    await startLang(['page']);
    applyMarkup(document);
    const p = document.querySelector('p')!;
    expect(p.textContent).toBe('Meine: <b>fett</b>');
    expect(p.querySelector('b')).toBeNull();
    expect(p.querySelector('a')!.getAttribute('href')).toBe('/x');
    expect(document.querySelector('button')!.title).toBe('Titel');
    await setLang('en');
    applyMarkup(document);
    expect(p.textContent).toBe('Mine: the list');
    expect(document.querySelector('button')!.title).toBe('Title');
  });
});
