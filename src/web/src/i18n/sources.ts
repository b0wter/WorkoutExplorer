// The English sources of the language files (F-54 LG-3, LG-8), for the completeness tests and the
// build's group check: every `texts.ts` table by its path, and the markup tables from `data-t`.
// Test-only (import.meta.glob of every table); never imported by the bundle.
import type { Table } from '../lib/i18n';
import { MARKUP } from './groups';

const modules = import.meta.glob(['../**/*texts.ts', '!../**/*.test.ts'], { eager: true }) as Record<string, { en?: Table }>;

/** Code tables: `src/<path>/texts.ts` is table `<path>`; `src/<path>/<name>-texts.ts` is `<path>/<name>-texts`. */
export const CODE_TABLES: Record<string, Table> = Object.fromEntries(
  Object.entries(modules)
    .filter(([, m]) => m.en)
    .map(([file, m]) => [file.replace(/^\.\.\//, '').replace(/\/texts\.ts$/, '').replace(/\.ts$/, ''), m.en!]),
);

/** The `data-t` keys of a markup file with their English: the element's text (child elements with
 *  `data-p` as `{name}`), and `data-t-<attr>` keys with the attribute's value. */
export function markupKeys(html: string): Record<string, string> {
  const doc = new DOMParser().parseFromString(html.replace(/\{\{[^}]*\}\}/g, ''), 'text/html');
  const out: Record<string, string> = {};
  for (const el of doc.querySelectorAll<HTMLElement>('[data-t]')) {
    out[el.dataset.t!] = [...el.childNodes].map((n) => (n.nodeType === 1 ? `{${(n as HTMLElement).dataset.p}}` : n.nodeType === 3 ? n.textContent : '')).join('');
  }
  for (const a of ['title', 'aria-label', 'placeholder', 'alt'])
    for (const el of doc.querySelectorAll(`[data-t-${a}]`)) out[el.getAttribute(`data-t-${a}`)!] = el.getAttribute(a) ?? '';
  return out;
}

/** Markup tables: the keys no code table holds, by MARKUP's table name. */
export function markupTables(read: (file: string) => string): Record<string, Record<string, string>> {
  const coded = new Set(Object.values(CODE_TABLES).flatMap((t) => Object.keys(t)));
  return Object.fromEntries(
    Object.entries(MARKUP).map(([name, files]) => [
      name,
      Object.fromEntries(files.flatMap((f) => Object.entries(markupKeys(read(f)))).filter(([k]) => !coded.has(k))),
    ]),
  );
}
