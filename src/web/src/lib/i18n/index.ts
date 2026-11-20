// The i18n core (F-54 LG-1 to LG-7, ADR-059): language store and message formatter; the loader and
// the markup applier are ./load, which the activity page imports only for a language other than
// English (bundle budget). Framework-free, kept per entry (LG-5: the upload and static entries get
// their own copy, `wx-entry-copy` in vite.config.ts). One instance per page: a page has one entry.
// Only the head script decides the language; this reads <html data-lang>. Nothing from a language
// file reaches innerHTML: every text is set as text.

export type Lang = 'en' | 'de' | 'es' | 'fr';
export const LANGS: readonly Lang[] = ['en', 'de', 'es', 'fr'];
/** The drop-down names each language in itself (LG-11); not in the language files. */
export const LANG_NAMES = ['English', 'Deutsch', 'Español', 'Français'];

/** A text, or a count text: CLDR plural categories, `one` and `other` required (LG-7). */
export type Text = string | Readonly<Record<string, string>>;
export type Table = Readonly<Record<string, Text>>;
export type Params = Readonly<Record<string, string | number>>;

let current: Lang = 'en';
/** The reactive hook: the activity page reads the language through a Svelte state (map-props). */
let source = (): Lang => current;
let sink = (l: Lang): void => void l;
/** The current language's texts of every loaded group, flat (keys are unique across tables). */
let dict: Record<string, unknown> = {};
let listeners: (() => void)[] = [];

export const lang = (): Lang => source();
/** Installs a reactive store: `write` gets every new language before the listeners run. */
export function langSource(read: () => Lang, write: (l: Lang) => void): void {
  source = read;
  sink = write;
  write(current);
}

/** Called after every language change (not on subscribe); returns the unsubscribe function. */
export function onLanguage(listener: () => void): () => void {
  listeners = [...listeners, listener];
  return () => (listeners = listeners.filter((l) => l !== listener));
}

/** `{name}` filled as text; a count text picks its category for `n` (missing category: `other`). */
export function format(text: Text, p?: Params): string {
  if (typeof text != 'string') text = text[new Intl.PluralRules(current).select(+(p?.n ?? 0))] ?? text.other;
  return p ? text.replace(/\{(\w+)\}/g, (m, k: string) => (k in p ? String(p[k]) : m)) : text;
}

const valid = (v: unknown): v is Text => typeof v == 'string' || (!!v && typeof v == 'object' && Object.values(v).every((s) => typeof s == 'string'));

/** The lookup for one English table: the loaded translation, else English. */
export function texts<T extends Table>(en: T): (key: keyof T & string, p?: Params) => string {
  return (key, p) => {
    source(); // a Svelte template that calls this re-renders on a change
    const v = dict[key];
    return format(valid(v) ? v : en[key], p);
  };
}

/** The page view's language, as the head script decided it (LG-1). */
export function pageLang(doc: Document = document): Lang {
  const l = doc.documentElement.dataset.lang as Lang;
  return LANGS.includes(l) ? l : 'en';
}

/** Every group this page view uses (entry and loaded lazy chunks): a switch loads them all. */
export const groups: string[] = [];

/** Takes `l` with its texts: the reactive hook, <html lang>, then every listener. */
export function use(l: Lang, texts: Record<string, unknown>, doc: Document): void {
  dict = texts;
  current = l;
  doc.documentElement.lang = l;
  sink(l);
  for (const f of listeners) f();
}

/** Merges a lazy group's texts when they are still for the current language. */
export function merge(l: Lang, texts: Record<string, unknown>): void {
  if (l == current) Object.assign(dict, texts);
}

/** The text of a markup key in the current language, if a file gave one. */
export const markupText = (key: string): string | undefined => (typeof dict[key] == 'string' ? (dict[key] as string) : undefined);

/** A lazy chunk's group, requested together with its import() (LG-3); English needs no file and no loader. Never rejects. */
export function loadGroup(group: string, doc: Document = document): Promise<void> {
  if (groups.includes(group)) return Promise.resolve();
  groups.push(group);
  const l = current;
  return l == 'en' ? Promise.resolve() : import('./load').then((m) => m.fetchGroup(group, l, doc).then((t) => merge(l, t)), () => {});
}

/** Test seam: back to a fresh page. */
export function resetI18n(): void {
  current = 'en';
  source = () => current;
  sink = () => {};
  dict = {};
  groups.length = 0;
  listeners = [];
}
