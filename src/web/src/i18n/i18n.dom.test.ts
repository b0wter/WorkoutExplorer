// @vitest-environment jsdom
// F-54 LG-8 (OQ-228): `make ci` fails when a text is missing in German, Spanish or French, a
// language file has a key English lacks, a translation's placeholders differ from English (this also
// holds AC 43: no translation adds a value to the F-58 example), a count text lacks `one` or `other`,
// a listed API code lacks a text, or a template or Svelte component carries interface text outside
// the tables. Also: every table is in a group, each language file stays within 5 KB gzip.
import { describe, expect, it } from 'vitest';
import { ROOT as WEB_ROOT, readWebFile } from '../../test-support/source.mjs';
import { LANG_FILE_LIMIT, gzipSize, langFiles, readTranslations } from '../../scripts/lang-files.mjs';
import type { Text } from '../lib/i18n';
import { en as errorTexts } from '../lib/errors/texts';
import { GROUPS, MARKUP, PAGES, TRANSLATED } from './groups';
import { CODE_TABLES, markupKeys, markupTables } from './sources';

const SOURCES: Record<string, Record<string, Text>> = { ...CODE_TABLES, ...markupTables(readWebFile) };
const LANGS = Object.fromEntries(TRANSLATED.map((l) => [l, JSON.parse(readWebFile(`src/i18n/${l}.json`)) as Record<string, Record<string, Text>>]));
const placeholders = (t: Text): string[] => [...new Set((typeof t === 'string' ? [t] : Object.values(t)).flatMap((s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1])))].sort();

describe('tables', () => {
  it('every key is unique across tables (language files are flat)', () => {
    const seen = new Map<string, string>();
    for (const [table, keys] of Object.entries(SOURCES))
      for (const k of Object.keys(keys)) {
        expect(seen.get(k), `${table}.${k}`).toBeUndefined();
        seen.set(k, table);
      }
  });

  it('a markup key that a code table holds has the same English (index.html and the edit components)', () => {
    const coded = Object.assign({}, ...Object.values(CODE_TABLES)) as Record<string, Text>;
    for (const files of Object.values(MARKUP))
      for (const f of files) for (const [k, v] of Object.entries(markupKeys(readWebFile(f)))) if (k in coded) expect(v, `${f} ${k}`).toBe(coded[k]);
  });

  it('every table is in a group, every group names known tables, every page names known groups', () => {
    const grouped = new Set(Object.values(GROUPS).flat());
    expect([...grouped].filter((t) => !(t in SOURCES))).toEqual([]);
    expect(Object.keys(SOURCES).filter((t) => !grouped.has(t))).toEqual([]);
    for (const p of Object.values(PAGES)) for (const g of p.groups) expect(GROUPS[g], g).toBeDefined();
  });

  it('count texts have `one` and `other`, and only CLDR categories', () => {
    for (const [table, keys] of Object.entries(SOURCES))
      for (const [k, v] of Object.entries(keys)) {
        if (typeof v === 'string') continue;
        expect(Object.keys(v), `${table}.${k}`).toEqual(expect.arrayContaining(['one', 'other']));
        for (const l of TRANSLATED) {
          const t = LANGS[l][table]?.[k];
          expect(typeof t === 'object' && t !== null ? Object.keys(t) : [], `${l} ${table}.${k}`).toEqual(expect.arrayContaining(['one', 'other']));
          for (const c of Object.keys(t as object)) expect(['zero', 'one', 'two', 'few', 'many', 'other'], `${l} ${table}.${k}.${c}`).toContain(c);
        }
      }
  });
});

describe.each(TRANSLATED)('%s', (l) => {
  const file = LANGS[l];

  it('has a non-empty text for every English key, of the same kind', () => {
    for (const [table, keys] of Object.entries(SOURCES))
      for (const [k, v] of Object.entries(keys)) {
        const t = file[table]?.[k];
        expect(typeof t, `${table}.${k}`).toBe(typeof v);
        for (const s of typeof t === 'string' ? [t] : Object.values(t ?? {})) expect(s.trim(), `${table}.${k}`).not.toBe('');
      }
  });

  it('has no table or key English lacks', () => {
    for (const [table, keys] of Object.entries(file)) {
      expect(SOURCES[table], table).toBeDefined();
      for (const k of Object.keys(keys)) expect(SOURCES[table]?.[k], `${table}.${k}`).toBeDefined();
    }
  });

  it('has the placeholders of English, no more and no fewer (AC 43: the F-58 example adds no value)', () => {
    for (const [table, keys] of Object.entries(SOURCES))
      for (const [k, v] of Object.entries(keys)) if (file[table]?.[k] !== undefined) expect(placeholders(file[table][k]), `${table}.${k}`).toEqual(placeholders(v));
  });

  it('holds plain text only: no markup', () => {
    for (const [table, keys] of Object.entries(file))
      for (const [k, v] of Object.entries(keys)) for (const s of typeof v === 'string' ? [v] : Object.values(v)) expect(s, `${table}.${k}`).not.toMatch(/<[a-z!/]/i);
  });
});

describe('language files (LG-3)', () => {
  const { files, urls } = langFiles(GROUPS, readTranslations(WEB_ROOT, TRANSLATED));
  it('one per group and language, each within 5 KB gzip, content-hashed, flat', () => {
    expect(Object.keys(files)).toHaveLength(Object.keys(GROUPS).length * TRANSLATED.length);
    for (const [name, text] of Object.entries(files)) {
      expect(name).toMatch(/^assets\/lang-[a-z]+-(de|es|fr)-[\w-]{8}\.json$/);
      expect(gzipSize(text), name).toBeLessThanOrEqual(LANG_FILE_LIMIT);
      for (const v of Object.values(JSON.parse(text))) expect(typeof v === 'string' || typeof v === 'object').toBe(true);
    }
    expect(Object.keys(urls)).toEqual(Object.keys(GROUPS));
  });
});

describe('API error codes (LG-6, P-14)', () => {
  const list = JSON.parse(readWebFile('../server/testdata/api/error-codes.json')) as { codes: Record<string, { general?: true; messages?: string[] }> };
  it('every listed code has a text, and no text is for a code the API does not send', () => {
    expect(Object.keys(errorTexts).sort()).toEqual(Object.keys(list.codes).sort());
  });
  it("a code with one fixed server sentence shows that sentence in English", () => {
    for (const [code, c] of Object.entries(list.codes)) if (c.messages?.length === 1) expect(errorTexts[code as keyof typeof errorTexts], code).toBe(c.messages[0]);
  });
});

/** Text a reader sees without a key: text nodes with letters outside `data-t`, and keyless title, aria-label, placeholder, alt. */
function unkeyed(html: string): string[] {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  for (const e of doc.querySelectorAll('script, style, noscript, title, template, svg')) e.remove();
  const out: string[] = [];
  const walk = (n: Node, keyed: boolean): void => {
    if (n.nodeType === 3 && !keyed && /\p{L}/u.test(n.textContent ?? '')) out.push(n.textContent!.trim());
    if (n.nodeType !== 1) return;
    const el = n as HTMLElement;
    for (const a of ['title', 'aria-label', 'placeholder', 'alt']) if (/\p{L}/u.test(el.getAttribute(a) ?? '') && !el.hasAttribute('data-t-' + a)) out.push(`${a}="${el.getAttribute(a)}"`);
    for (const c of el.childNodes) walk(c, keyed || el.hasAttribute('data-t'));
  };
  walk(doc.body, false);
  return out;
}

/** Allowed without a key, with the reason. */
const ALLOWED: Record<string, string> = {
  'Workout Explorer': 'the product name, the same in every language',
  '{{.Header.Title}}': "the title is the owner's or the default title, rewritten by the language pass (LG-4, LG-10)",
  '{{.Header.SportName}} · {{.Header.Start}}': 'view-model text, rewritten by the language pass (LG-4)',
  '{{.Header.Description}}': "owner-written text, never translated (LG-10)",
  '{{.Header.ExpiresText}}': 'view-model text, rewritten by enhanceExpiry',
  'max {{.Max}}': 'view-model text, rewritten by enhanceUnits (LG-4)',
};

/** International unit symbols, the same in the four languages (see lib/format/texts). */
const SYMBOLS: Record<string, string> = { W: 'watts' };

describe('no interface text outside the tables (LG-8)', () => {
  it.each(['index.html', 'edit.html', 'my-workouts.html', 'upload.html', 'about.html', 'generic.html'])('%s', (f) => {
    const html = readWebFile(f).replace(/\{\{(if|else|end|range|with)[^}]*\}\}/g, '');
    // Template actions whose value is view-model text or data stay; anything else needs a key.
    const left = unkeyed(html).filter((t) => !(t in ALLOWED) && !/^(\{\{[^}]*\}\}\s*)+$/.test(t));
    expect(left).toEqual([]);
  });

  const svelte = import.meta.glob(['../**/*.svelte', '!../**/testing/**', '!../**/*.test-helper.*'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
  it.each(Object.keys(svelte))('%s', (f) => {
    const markup = svelte[f].replace(/<script[\s\S]*?<\/script>/g, '').replace(/<style[\s\S]*?<\/style>/g, '').replace(/<!--[\s\S]*?-->/g, '');
    let bare = markup.replace(/<svg[\s\S]*?<\/svg>/g, '').replace(/&\w+;/g, ' ');
    for (let before = ''; before !== bare; ) [before, bare] = [bare, bare.replace(/\{[^{}]*\}/g, ' ')];
    const texts = bare
      .split(/<[^>]*>/)
      .map((s) => s.trim())
      .filter((s) => /\p{L}/u.test(s) && !(s in SYMBOLS));
    const attrs = [...markup.matchAll(/\s(title|aria-label|placeholder|alt)="([^"{]*\p{L}[^"]*)"/gu)].map((m) => m[0].trim());
    expect([...texts, ...attrs]).toEqual([]);
  });
});

// Files that assign HTML: constants, SVG, markup shells or fetched files, never a text. A new file here
// (or a text passed to one of them through a variable) is a decision, so it fails until listed.
// Gap (F-62): English text that TypeScript builds with textContent or setAttribute is not scanned.
const HTML_ASSIGNERS = ['edit/start.svelte.ts', 'efforts/index.ts', 'splits/chart.ts', 'impressum/page.ts', 'page/enhance.ts', 'components/map/map-view.ts', 'report/form.ts', 'lib/i18n/index.ts'];

describe('texts never reach HTML parsing (LG-7)', () => {
  const code = import.meta.glob(['../**/*.ts', '../**/*.svelte', '!../**/*.test.ts', '!../**/testing/**'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
  it('only the listed files assign HTML', () => {
    const files = Object.entries(code).filter(([, s]) => /innerHTML|outerHTML|insertAdjacentHTML|\{@html/.test(s)).map(([f]) => f.replace('../', ''));
    expect(files.filter((f) => !HTML_ASSIGNERS.includes(f))).toEqual([]);
  });
  it('no innerHTML, outerHTML, insertAdjacentHTML or {@html} is given a text from a table', () => {
    for (const [f, s] of Object.entries(code))
      for (const line of s.split('\n')) expect(line, f).not.toMatch(/(innerHTML|outerHTML|insertAdjacentHTML\([^,]*,)\s*[=+]?[^;]*\bt[a-z]?\(|\{@html[^}]*\bt[a-z]?\(/);
  });
});
