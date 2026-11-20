// The My workouts page (F-53 MW-4 to MW-10). LAZY: reached only through main.ts's import() on
// my-workouts.html (#wx-mine). Sends no request (AC 47). CRITICAL (MW-5): every stored string
// reaches the DOM as text only (textContent), and the only attribute built from storage is an
// href from lib/mine's checked, root-relative paths; same tab, no target.
// The stylesheet by URL, not as a side-effect import: that would add one more file name to the
// activity entry's preload map, over its 60-byte ceiling (F-53 Cost). Same origin (CSP style-src 'self').
import css from './mine.css?url';
import { clear, forget, prune, type Row, type StoreGetter } from '../lib/mine';
import { formatDate, sportName } from '../lib/format/dates';
import { MISSING_TEXT } from '../lib/format';
import { impressum } from '../lib/page/impressum';

/** MW-9: every interface text of the page, in one place (F-54 swaps it and LANG). */
export const TEXT = {
  title: 'My workouts',
  note: 'Edit links are saved in this browser. Anyone using this browser can open them.',
  empty: 'No workouts uploaded from this browser yet.',
  unavailable: 'The list is not available in this browser.',
  heads: ['Workout', 'Sport', 'Date', 'Status', 'Expires', 'Links', ''],
  draft: 'Draft',
  published: 'Published',
  edit: 'Edit',
  view: 'View',
  forget: 'Forget',
  clear: 'Clear list',
  ask: 'Remove all workouts from this list? They stay online, but without their edit links you cannot change or delete them.',
  confirm: 'Remove all',
  cancel: 'Cancel',
  upload: 'Upload a workout',
  about: 'About',
};
const LANG = 'en';

/** 'in 3 days' (MW-9): minutes below an hour, hours below two days, else days. */
export function relative(ms: number, rtf = new Intl.RelativeTimeFormat(LANG)): string {
  const min = ms / 60_000;
  if (min < 60) return rtf.format(Math.max(1, Math.round(min)), 'minute');
  if (min < 48 * 60) return rtf.format(Math.round(min / 60), 'hour');
  return rtf.format(Math.round(min / 1440), 'day');
}

const day = (date: string): string => formatDate(date + 'T00:00:00Z', 0);

/** UP-5: the stored title (owner's or suggested), else the default title built here. */
export const rowTitle = (r: Row): string => (r.title?.trim() ? r.title : sportName(r.sport ?? '') + (r.date ? ', ' + day(r.date) : ''));

/** Renders once the stylesheet has loaded or failed, so the table never shows unstyled. */
export function startMine(host: HTMLElement, store?: StoreGetter, now?: () => number): void {
  const link = host.ownerDocument.createElement('link');
  link.rel = 'stylesheet';
  link.href = css;
  link.onload = link.onerror = () => show(host, store, now);
  host.ownerDocument.head.append(link);
}

export function show(host: HTMLElement, store: StoreGetter = () => window.localStorage, now: () => number = Date.now): void {
  const doc = host.ownerDocument;
  doc.title = TEXT.title + ' · Workout Explorer';
  const el = <K extends keyof HTMLElementTagNameMap>(tag: K, text = '', parent?: HTMLElement): HTMLElementTagNameMap[K] => {
    const e = doc.createElement(tag);
    e.textContent = text;
    parent?.append(e);
    return e;
  };
  const button = (text: string, act: string, parent: HTMLElement): void => {
    const b = el('button', text, parent);
    b.type = 'button';
    b.className = 'mine-button';
    b.dataset.act = act;
  };
  const link = (text: string, href: string, parent: HTMLElement): void => {
    el('a', text, parent).href = href;
  };
  const main = el('main');
  main.className = 'mine';
  /** The rows on screen, by their row index (Forget). */
  let rows: Row[] = [];

  /** `cleared`: the key was just removed; shown empty without writing it back (OQ-279). */
  const render = (mode?: 'asking' | 'cleared'): void => {
    main.replaceChildren();
    el('h1', TEXT.title, main);
    el('p', TEXT.note, main);
    const got = mode === 'cleared' ? [] : prune(store, now());
    rows = got ?? [];
    if (!got) el('p', TEXT.unavailable, main);
    else if (got.length === 0) el('p', TEXT.empty, main);
    else {
      const box = el('div', '', main);
      box.className = 'mine-box';
      const table = el('table', '', box);
      const head = el('tr', '', el('thead', '', table));
      for (const h of TEXT.heads) el('th', h, head);
      const body = el('tbody', '', table);
      const t = now();
      got.forEach((r, i) => {
        const tr = el('tr', '', body);
        el('td', rowTitle(r), tr);
        el('td', r.sport === undefined ? MISSING_TEXT : sportName(r.sport), tr);
        el('td', r.date ? day(r.date) : MISSING_TEXT, tr);
        el('td', r.published ? TEXT.published : TEXT.draft, tr);
        el('td', relative(r.expiresMs - t), tr);
        const links = el('td', '', tr);
        links.className = 'mine-links';
        if (r.edit) link(TEXT.edit, r.edit, links);
        if (r.view) link(TEXT.view, r.view, links);
        button(TEXT.forget, 'forget:' + i, el('td', '', tr));
      });
      const p = el('p', '', main);
      if (mode !== 'asking') button(TEXT.clear, 'clear', p);
      else {
        p.className = 'mine-confirm';
        el('span', TEXT.ask, p);
        el('br', '', p);
        button(TEXT.confirm, 'confirm', p);
        p.append(' ');
        button(TEXT.cancel, 'cancel', p);
      }
    }
    link(TEXT.upload, '/', el('p', '', main));
  };
  host.replaceChildren(main);
  render();

  main.addEventListener('click', (e) => {
    const act = (e.target as Element).closest<HTMLElement>('[data-act]')?.dataset.act;
    if (!act) return;
    if (act.startsWith('forget:')) {
      const r = rows[Number(act.slice(7))];
      if (r) forget(store, r.raw);
      render();
    } else if (act === 'clear') render('asking');
    else if (act === 'confirm') render(clear(store) ? 'cleared' : undefined); else if (act === 'cancel') render();
  });

  const foot = el('footer', '', host);
  foot.className = 'mine-foot';
  link(TEXT.about, '/about', foot);
  impressum(foot);
}
