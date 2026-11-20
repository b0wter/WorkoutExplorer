// The My workouts list in browser storage (F-53 MW-2 to MW-7, ADR-058). CRITICAL: the list holds
// edit tokens. One localStorage key, never sent anywhere. Pure apart from the storage passed in;
// imports nothing of ours. Imported only by src/edit (the writers) and src/mine (the list page);
// never by the viewing page (mine.test.ts checks the import graph, check-upload-entry.mjs the build).
// Every access is guarded: a throwing getter, getItem or setItem is contained. Each write reads
// storage afresh and writes at once (MW-3). Links are rebuilt only from checked parts (ADR-058 item 2).

/** The part of `Storage` used here (localStorage in the browser, a map in tests). */
export interface ListStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}
export type StoreGetter = () => ListStore | null | undefined;

export const LIST_KEY = 'wx.workouts';

/** What the owner pages write (MW-2); `sport` and `date` absent keep the stored ones. */
export interface Entry {
  id: string;
  token: string;
  title: string;
  sport?: string;
  date?: string;
  created: string;
  expires: string;
  published: boolean;
}

/** One row of the list page, every field checked on its own (MW-5): absent when invalid. */
export interface Row {
  /** The stored entry as JSON: Forget removes exactly this one. */
  raw: string;
  title?: string;
  sport?: string;
  /** 'YYYY-MM-DD'. */
  date?: string;
  published: boolean;
  /** Effective expiry in ms (MW-4); NaN when unparseable (counts as expired). */
  expiresMs: number;
  createdMs: number;
  /** Root-relative, built from the checked ID and token; absent when either fails. */
  edit?: string;
  view?: string;
}

/** ADR-020's 22-character base62 format of public IDs and edit tokens. */
export const ID = /^[0-9A-Za-z]{22}$/;
const DATE = /^\d{4}-\d{2}-\d{2}$/;
const DRAFT_MS = 48 * 3600 * 1000;

type Obj = Record<string, unknown>;
const isObj = (v: unknown): v is Obj => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string | undefined => (typeof v === 'string' ? v : undefined);

/** The raw entries; [] with no key; null when unavailable or unreadable (not version 1 JSON of the
 *  shape). Entries that are not objects are dropped (MW-5). */
function read(store: StoreGetter): Obj[] | null {
  let text: string | null;
  try {
    const s = store();
    if (!s) return null;
    text = s.getItem(LIST_KEY);
  } catch {
    return null;
  }
  if (text === null) return [];
  try {
    const v: unknown = JSON.parse(text);
    if (!isObj(v) || v.v !== 1 || !Array.isArray(v.workouts)) return null;
    return v.workouts.filter(isObj);
  } catch {
    return null;
  }
}

/** True when written. */
function write(store: StoreGetter, workouts: readonly Obj[]): boolean {
  try {
    const s = store();
    if (!s) return false;
    s.setItem(LIST_KEY, JSON.stringify({ v: 1, workouts }));
    return true;
  } catch {
    return false;
  }
}

/** MW-4: published → `expires`; a draft → the earlier of `expires` and `created` + 48 h. NaN when unparseable. */
export function effectiveExpiry(created: unknown, expires: unknown, published: unknown): number {
  const c = typeof created === 'string' ? Date.parse(created) : NaN;
  const e = typeof expires === 'string' ? Date.parse(expires) : NaN;
  if (Number.isNaN(c) || Number.isNaN(e)) return NaN;
  return published === true ? e : Math.min(e, c + DRAFT_MS);
}

/** The checked view of one stored entry (MW-5). */
export function toRow(e: Obj): Row {
  const id = str(e.id);
  const token = str(e.token);
  const idOk = id !== undefined && ID.test(id);
  const date = str(e.date);
  const published = e.published === true;
  return {
    raw: JSON.stringify(e),
    title: str(e.title),
    sport: str(e.sport),
    date: date !== undefined && DATE.test(date) && !Number.isNaN(Date.parse(date)) ? date : undefined,
    published,
    expiresMs: effectiveExpiry(e.created, e.expires, e.published),
    createdMs: typeof e.created === 'string' ? Date.parse(e.created) : NaN,
    edit: idOk && token !== undefined && ID.test(token) ? '/edit/' + id + '#' + token : undefined,
    view: idOk && published ? '/w/' + id : undefined,
  };
}

/** Edit page (MW-3): adds or replaces the entry by `id`; an unreadable value is replaced by a fresh
 *  list (ADR-058 item 4). Never throws. */
export function upsert(store: StoreGetter, entry: Entry, now = Date.now()): void {
  const list = read(store) ?? [];
  const old = list.find((e) => e.id === entry.id);
  const next: Obj = { ...entry, sport: entry.sport ?? old?.sport, date: entry.date ?? old?.date };
  // Other entries past their expiry go on every write (About, INF-1); unparseable ones wait for prune.
  const live = list.filter((e) => e.id !== entry.id && !(effectiveExpiry(e.created, e.expires, e.published) <= now));
  write(store, [...live, next]);
}

/** Published page (MW-3): updates these fields of an existing entry, never adds one. */
export function update(store: StoreGetter, id: string, fields: Pick<Entry, 'title' | 'created' | 'expires' | 'published'>): void {
  const list = read(store);
  if (!list?.some((e) => e.id === id)) return;
  write(store, list.map((e) => (e.id === id ? { ...e, ...fields } : e)));
}

/** Not found or deleted (MW-3, ADR-058 item 5, P-12): removes the entry only when its stored token
 *  equals the address's. */
export function remove(store: StoreGetter, id: string, token: string): void {
  const list = read(store);
  if (!list?.some((e) => e.id === id && e.token === token)) return;
  write(store, list.filter((e) => !(e.id === id && e.token === token)));
}

/** List page (MW-4, MW-6): drops entries past their effective expiry at `now`, writes the result
 *  back and returns the rows newest `created` first; null when the list is not available (the
 *  value is unreadable, which is never rewritten, or the write-back fails). */
export function prune(store: StoreGetter, now: number): Row[] | null {
  const list = read(store);
  if (!list) return null;
  // NaN > now is false: unparseable counts as expired (MW-5).
  const kept = list.filter((e) => effectiveExpiry(e.created, e.expires, e.published) > now);
  if (!write(store, kept)) return null;
  return kept.map(toRow).sort((a, b) => b.createdMs - a.createdMs);
}

/** Forget (MW-4): removes the one entry whose JSON is `raw`, at once. True when written. */
export function forget(store: StoreGetter, raw: string): boolean {
  const list = read(store);
  return list !== null && write(store, list.filter((e) => JSON.stringify(e) !== raw));
}

/** Clear list (MW-4, OQ-279): removes the key. True when done. */
export function clear(store: StoreGetter): boolean {
  try {
    const s = store();
    if (!s) return false;
    s.removeItem(LIST_KEY);
    return true;
  } catch {
    return false;
  }
}
